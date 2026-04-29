// Package reminders owns the cadence-driven email/push loops that
// nudge users through onboarding (cv_upload, login_after_cv, future
// magic_verify). Lives outside the signal package so the policy
// (when to send, to whom, on what schedule) is decoupled from the
// transport (mail client, APNs client) and from signal's own
// NATS-handler surface.
//
// Dependencies are injected via small interfaces (Mailer, Pusher,
// LangResolver) so the loops are easy to test and so adding a new
// kind doesn't drag in extra signal-internal coupling.
//
// IMPORTANT (single-replica only): the in-process timer + atomic
// guard pattern works for ONE pod. Two replicas would each fire
// every loop and double-send. Migrate to k8s CronJobs (see
// SCALING_AUDIT.md) before scaling falcon-signal.
package reminders

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/auth"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/signal/push"
)

// ── Injected dependencies ───────────────────────────────────────

// Mailer sends a templated email. Falcon-signal's *email.Client
// satisfies this implicitly.
type Mailer interface {
	Send(to, template, lang string, vars map[string]string) error
}

// Pusher delivers an APNs notification with a rendered payload +
// extras. Falcon-signal's *apnsClient satisfies this implicitly.
type Pusher interface {
	SendTemplated(ctx context.Context, deviceToken string, payload push.Payload, extra map[string]string) error
	IsStaleToken(err error) bool
}

// LangResolver picks the language for a user. signal.Service
// satisfies this via its ResolveUserLanguage method.
type LangResolver interface {
	ResolveUserLanguage(email, platform string) string
}

// Service is the entry point of this package. Construct once at
// boot, then call RunCVLoop and RunLoginLoop in goroutines.
type Service struct {
	mail Mailer
	push Pusher
	lang LangResolver
}

// New wires a Service from its dependencies.
func New(mail Mailer, pusher Pusher, lang LangResolver) *Service {
	return &Service{mail: mail, push: pusher, lang: lang}
}

// ── Cadence + window constants (shared across all kinds) ─────────
//
// Every UserReminderKind today follows the same shape: 1d grace,
// daily for week 1, weekly until 30d, then stop. If a future kind
// needs a different curve (e.g. magic_verify with a 20-min initial
// fire), parameterise nextReminderAction; for now keeping a single
// set of constants keeps the policy obvious.

const (
	gracePeriod = 24 * time.Hour
	dailyUntil  = 7 * 24 * time.Hour
	weeklyUntil = 30 * 24 * time.Hour
	dailyEvery  = 24 * time.Hour
	weeklyEvery = 7 * 24 * time.Hour

	// Sending window — local Berlin time. Outside this band the
	// loops are a no-op so we don't push or email someone in the
	// middle of the night. DST is handled automatically because we
	// evaluate against time.Now().In(berlin).Hour().
	sendStartHour = 8  // 08:00 Berlin
	sendEndHour   = 20 // strict <, so last delivery slot is 19:59
	timezone      = "Europe/Berlin"
)

// berlinLocation is loaded once at startup. If the timezone database
// is missing in the runtime image we fall back to UTC + a warning —
// alpine images need tzdata installed; if it isn't, the gate becomes
// "08:00–20:00 UTC" which is approximately right for Europe in
// summer and wrong by an hour in winter, but not catastrophic.
var berlinLocation = func() *time.Location {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		logrus.Warnf("[reminders] LoadLocation %s failed (%v) — falling back to UTC; install tzdata in the image", timezone, err)
		return time.UTC
	}
	return loc
}()

// withinSendingWindow reports whether `now` is inside the
// 08:00–20:00 Berlin window. Used as the per-tick gate so the loops
// can run hourly but only act during waking hours.
func withinSendingWindow(now time.Time) bool {
	h := now.In(berlinLocation).Hour()
	return h >= sendStartHour && h < sendEndHour
}

// ── Tunable knobs (shared via REMINDER_* env vars) ───────────────

type config struct {
	// PageSize — how many candidate users to load per Mongo round-
	// trip. Within a tick we walk pages until empty; choose this big
	// enough that the page query is cheap relative to the per-user
	// processing cost, but small enough that a single page fits
	// comfortably in memory (~1 KB per User doc → 100 = ~100 KB).
	PageSize int
	// BatchSize — sub-batches WITHIN a page. Pick this so
	// (BatchSize / BatchPause) stays under your slowest external
	// channel's rate cap. Mailjet's send API is the bottleneck
	// (~50 req/s/IP). Default 4 / 2s = 2/s is a 25× safety margin.
	BatchSize int
	// BatchPause — sleep between sub-batches. Honoured even if the
	// batch finished in milliseconds, to spread sends evenly.
	BatchPause time.Duration
}

// loadConfig pulls the three knobs once per tick. Reading every
// tick (instead of caching at boot) lets an operator bump the
// values via ConfigMap edit + pod restart without recompiling.
func loadConfig() config {
	return config{
		PageSize:   environment.ParseInt("REMINDER_PAGE_SIZE", 100),
		BatchSize:  environment.ParseInt("REMINDER_BATCH_SIZE", 4),
		BatchPause: environment.ParseDuration("REMINDER_BATCH_PAUSE", "2s"),
	}
}

// ── Re-entrancy guard ────────────────────────────────────────────

// tickGuard is the per-loop re-entrancy lock. Compare-and-swap
// from false → true at tick start; if the swap fails, the previous
// tick is still running and we skip cleanly.
type tickGuard struct {
	running atomic.Bool
	label   string // logging prefix, e.g. "[cv-reminder]"
}

func (g *tickGuard) tryAcquire() bool {
	if g.running.CompareAndSwap(false, true) {
		return true
	}
	logrus.Warnf("%s previous tick still running — skipping this scheduled tick", g.label)
	return false
}

func (g *tickGuard) release() { g.running.Store(false) }

// ── Action / decision enums ──────────────────────────────────────

// action is the per-user outcome bucket used by the batch
// accounting. Aggregated into the tick summary log.
type action int

const (
	actionSkipped action = iota
	actionSent
	actionFixedFlag
	actionStopped
)

// decision is what nextReminderAction returns. Pure-function
// output so the cadence rules stay testable without touching
// Mongo / NATS / APNs.
type decision int

const (
	decisionSkip decision = iota
	decisionStop
	decisionSend
)

// nextReminderAction is the cadence rule, isolated from any I/O
// so it can be tested with a table of (createdAt, last_at, count)
// scenarios. Order matters: Stopped first (cheap short-circuit),
// then terminal window, then grace, then per-stage gap.
func nextReminderAction(now, createdAt time.Time, rem models.UserReminder) decision {
	if rem.Stopped {
		return decisionSkip
	}
	timeSinceJoin := now.Sub(createdAt)

	if timeSinceJoin >= weeklyUntil {
		return decisionStop
	}
	if timeSinceJoin < gracePeriod {
		return decisionSkip
	}
	if !rem.LastAt.IsZero() {
		minGap := dailyEvery
		if timeSinceJoin >= dailyUntil {
			minGap = weeklyEvery
		}
		if now.Sub(rem.LastAt) < minGap {
			return decisionSkip
		}
	}
	return decisionSend
}

// ── Helpers shared across loops ─────────────────────────────────

// unsubscribeURLOrEmpty returns the HMAC-signed unsubscribe URL for
// the given email, or "" if the secret env var is missing. The
// templates render the URL directly into an <a href> — an empty
// href in HTML is harmless (the link is unclickable) and lets the
// reminder still go out without crashing the send.
func unsubscribeURLOrEmpty(email string, log *logrus.Entry) string {
	url, err := auth.RemindersURL(email)
	if err != nil {
		log.Warnf("[reminders] unsubscribe URL skipped — %v", err)
		return ""
	}
	return url
}

// loadBlockedEmails wraps `auth.ActiveBlocksByEmail` with the
// reminder-loop policy: fail-open + logrus.Warnf on Mongo error.
// The primitive lives in `packages/auth/blocks.go` next to the
// singular `ActiveBlock` so other consumers (future magic_verify
// loop, abuse dashboards) can reuse it; the policy stays here
// because it's reminder-specific (silencing banned users on a
// transient blip is worse than briefly noop'ing the gate).
func loadBlockedEmails(ctx context.Context, emails []string, now time.Time) map[string]bool {
	set, err := auth.ActiveBlocksByEmail(ctx, emails, now)
	if err != nil {
		logrus.Warnf("[reminders] load auth_blocks failed (fail-open, will send): %v", err)
		return nil
	}
	return set
}

// loadOptedOutEmails wraps `auth.ActiveOptOutsByEmail` with the
// reminder-loop policy: fail-open + logrus.Warnf on Mongo error.
// The primitive lives in `packages/auth/optout.go` next to the
// HMAC token helpers so other consumers (future magic_verify loop,
// admin opt-out dashboards) can reuse it; the policy stays here
// because it's reminder-specific (silencing legitimately opted-out
// users on a transient blip is worse than briefly noop'ing the
// gate).
func loadOptedOutEmails(ctx context.Context, kind models.AuthOptOutKind, emails []string) map[string]bool {
	set, err := auth.ActiveOptOutsByEmail(ctx, emails, kind)
	if err != nil {
		logrus.Warnf("[reminders] load opt-outs failed (fail-open, will send): %v", err)
		return nil
	}
	return set
}

// upsertReminder persists the reminder row after a send (or after
// a stop). Increments Count when sending, leaves it untouched when
// only flipping Stopped. Uses Set with the (user_id, kind) filter
// so the unique compound index covers it atomically.
func (s *Service) upsertReminder(ctx context.Context, userID string, kind models.UserReminderKind, prev models.UserReminder, now time.Time, stopped bool) {
	doc := models.UserReminder{
		ID:      prev.ID,
		UserID:  userID,
		Kind:    kind,
		Count:   prev.Count,
		FirstAt: prev.FirstAt,
		LastAt:  prev.LastAt,
		Stopped: stopped || prev.Stopped,
	}
	if doc.ID == "" {
		doc.ID = gonanoid.Must()
	}
	if !stopped {
		doc.Count++
		doc.LastAt = now
		if doc.FirstAt.IsZero() {
			doc.FirstAt = now
		}
	}

	if err := system.GetStorage().Set(ctx, constants.MongoUserRemindersCollection,
		bson.M{"user_id": userID, "kind": string(kind)}, doc); err != nil {
		logrus.Errorf("[reminder/%s] upsert for %s: %v", kind, userID, err)
	}
}

// safePrefix returns the first n runes of s, or all of s if it's
// shorter. Used in log lines that quote a device token without
// dumping the full secret.
func safePrefix(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// ── Loop runner ──────────────────────────────────────────────────

// loop is the ticker + guard + goroutine wrapper that every
// reminder kind shares.
type loop struct {
	label    string
	interval time.Duration
	guard    *tickGuard
	process  func(ctx context.Context)
}

// run blocks until ctx is done. Spawns each tick in a child
// goroutine so the parent ticker keeps firing on time even when a
// tick takes much longer than the interval. The first tick fires
// immediately so a fresh deploy does work right away.
func (l *loop) run(ctx context.Context) {
	logrus.Infof("[reminders] %s loop starting (tick every %s)", l.label, l.interval)

	t := time.NewTicker(l.interval)
	defer t.Stop()
	l.tryRunTick(ctx)
	for {
		select {
		case <-ctx.Done():
			logrus.Infof("[reminders] %s loop stopping", l.label)
			return
		case <-t.C:
			l.tryRunTick(ctx)
		}
	}
}

func (l *loop) tryRunTick(ctx context.Context) {
	if !l.guard.tryAcquire() {
		return
	}
	go func() {
		defer l.guard.release()
		l.process(ctx)
	}()
}

// ── Queue processor ──────────────────────────────────────────────

// pageCache holds bulk-loaded data for a single page of candidates.
// Built once after loadPage so per-user processors avoid N round-
// trips to Mongo.
//
// reminders[user_id][kind] = the loaded UserReminder for that
// pair. Missing entry = first-time reminder (zero-value rem).
//
// cvIndexedUsers is the set of user_ids in the page that have a
// CV in indexed/normalizing/normalized state. Populated only when
// the spec asks for it (cv-reminder kind only).
type pageCache struct {
	reminders      map[string]map[models.UserReminderKind]models.UserReminder
	cvIndexedUsers map[string]bool
}

// reminder returns the cached UserReminder for (userID, kind), or
// the zero value if none was loaded. nil-safe.
func (p *pageCache) reminder(userID string, kind models.UserReminderKind) models.UserReminder {
	if p == nil || p.reminders == nil {
		return models.UserReminder{}
	}
	return p.reminders[userID][kind]
}

// hasCVIndexed reports whether userID has a usable CV in the page
// cache. Only meaningful when the spec asked for cv prefetch.
func (p *pageCache) hasCVIndexed(userID string) bool {
	if p == nil {
		return false
	}
	return p.cvIndexedUsers[userID]
}

// buildPageCache loads the per-page bulk data declared by spec.
// Two queries max:
//   - user_reminders for (user_id $in [page], kind $in [spec.prefetchKinds])
//   - cvs distinct user_id for (user_id $in [page], status $in [indexed/normalizing/normalized])
//     — only if spec.prefetchCVIndexed.
//
// On any error returns (zero, err). The caller (loadPage) propagates
// the error so processBatched aborts the tick — better than
// continuing with partial data and re-sending to everyone as if
// they were first-timers.
func buildPageCache(ctx context.Context, userIDs []string, spec queueSpec) (pageCache, error) {
	cache := pageCache{
		reminders: make(map[string]map[models.UserReminderKind]models.UserReminder),
	}
	if len(userIDs) == 0 {
		return cache, nil
	}

	if len(spec.prefetchKinds) > 0 {
		kindStrs := make([]string, len(spec.prefetchKinds))
		for i, k := range spec.prefetchKinds {
			kindStrs[i] = string(k)
		}
		var rows []models.UserReminder
		if err := system.GetStorage().GetMany(ctx,
			constants.MongoUserRemindersCollection,
			bson.M{
				"user_id": bson.M{"$in": userIDs},
				"kind":    bson.M{"$in": kindStrs},
			},
			&rows,
		); err != nil {
			return cache, fmt.Errorf("bulk-load user_reminders: %w", err)
		}
		for _, r := range rows {
			if cache.reminders[r.UserID] == nil {
				cache.reminders[r.UserID] = make(map[models.UserReminderKind]models.UserReminder)
			}
			cache.reminders[r.UserID][r.Kind] = r
		}
	}

	if spec.prefetchCVIndexed {
		ids, err := system.GetStorage().Distinct(ctx,
			constants.MongoCVsCollection,
			"user_id",
			bson.M{
				"user_id": bson.M{"$in": userIDs},
				"status":  bson.M{"$in": models.CVStatusesUsableBSON()},
			},
		)
		if err != nil {
			return cache, fmt.Errorf("bulk-load cvs distinct: %w", err)
		}
		cache.cvIndexedUsers = make(map[string]bool, len(ids))
		for _, id := range ids {
			cache.cvIndexedUsers[id] = true
		}
	}

	return cache, nil
}

// queueSpec is the per-kind config for processQueue. Captures the
// candidate filter, the per-user processor, the labels for logging,
// and the bulk-load hints used to build the page cache.
type queueSpec struct {
	label            string
	fixedActionLabel string
	filter           bson.M
	// prefetchKinds lists the UserReminderKinds to bulk-load from
	// user_reminders for the page. Always include this loop's own
	// kind. Add cross-kinds when the per-user processor needs them
	// (login-reminder uses cv_upload to enforce the cross-kind gap).
	prefetchKinds []models.UserReminderKind
	// prefetchCVIndexed asks for the per-page set of user_ids that
	// have a usable CV (status indexed/normalizing/normalized).
	// Only cv-reminder needs this — to drive its defensive recheck
	// without an N-Count storm.
	prefetchCVIndexed bool
	// processOne handles a single user past the opt-out gate. Receives
	// the page cache so it can lookup reminder state and recheck data
	// without hitting Mongo per-candidate.
	processOne func(ctx context.Context, u models.User, now time.Time, page *pageCache) action
}

// processQueue is the per-tick body shared by every loop: gate the
// sending window, paginate candidates from `users`, capture the
// per-page opt-out set + bulk caches, and drive processBatched.
func (s *Service) processQueue(ctx context.Context, spec queueSpec) {
	now := time.Now()
	if !withinSendingWindow(now) {
		logrus.Debugf("%s outside Berlin sending window (08:00–20:00) — idle", spec.label)
		return
	}

	since := now.Add(-weeklyUntil)
	filter := bson.M{"created_at": bson.M{"$gte": since}}
	for k, v := range spec.filter {
		filter[k] = v
	}

	cfg := loadConfig()

	var optedOut map[string]bool
	var blocked map[string]bool
	var page pageCache

	loadPage := func(pageNum int) ([]models.User, error) {
		var users []models.User
		_, err := system.GetStorage().FindPage(ctx,
			constants.MongoUsersCollection,
			filter,
			"created_at", true, // newest first — fresh signups get the first ping while warm
			pageNum, cfg.PageSize, &users,
		)
		if err != nil {
			return nil, err
		}
		emails := make([]string, len(users))
		userIDs := make([]string, len(users))
		for i, u := range users {
			emails[i] = u.Email
			userIDs[i] = u.ID
		}
		optedOut = loadOptedOutEmails(ctx, models.AuthOptOutKindConversionReminders, emails)
		blocked = loadBlockedEmails(ctx, emails, now)
		page, err = buildPageCache(ctx, userIDs, spec)
		if err != nil {
			// Abort the tick rather than continue with stale or
			// missing reminder state — the next scheduled tick
			// retries from scratch.
			return nil, err
		}
		return users, nil
	}
	processOne := func(u models.User) action {
		if blocked[u.Email] {
			logrus.Debugf("%s %s blocked — skipping", spec.label, u.Email)
			return actionSkipped
		}
		if optedOut[u.Email] {
			logrus.Debugf("%s %s opted out — skipping", spec.label, u.Email)
			return actionSkipped
		}
		return spec.processOne(ctx, u, time.Now(), &page)
	}

	sent, skipped, fixed, stopped := s.processBatched(ctx, spec.label, cfg, loadPage, processOne)
	logrus.Infof("%s tick done — sent=%d skipped=%d %s=%d stopped=%d (page=%d batch=%d pause=%s)",
		spec.label, sent, skipped, spec.fixedActionLabel, fixed, stopped, cfg.PageSize, cfg.BatchSize, cfg.BatchPause)
}

// ── Per-user cadence helper ─────────────────────────────────────

// decideFromCadence is the front half of every per-user processor:
// run the cadence decision against a pre-loaded reminder row (from
// the page cache) and either short-circuit (skip / stop) or hand
// control back to the caller for kind-specific rechecks + send.
//
// Returns log + action + send. If send=false, the caller returns
// action immediately. If send=true, the caller proceeds with
// rechecks + delivery and at the end calls upsertReminder with the
// same `rem` it passed in.
//
// On decisionStop this helper persists Stopped=true and emits the
// terminal log line; the caller just returns actionStopped.
func (s *Service) decideFromCadence(
	ctx context.Context,
	label string,
	kind models.UserReminderKind,
	u models.User,
	now time.Time,
	rem models.UserReminder,
) (log *logrus.Entry, act action, send bool) {
	log = logrus.WithFields(logrus.Fields{"user_id": u.ID, "email": u.Email})

	switch nextReminderAction(now, u.CreatedAt, rem) {
	case decisionSkip:
		return log, actionSkipped, false
	case decisionStop:
		s.upsertReminder(ctx, u.ID, kind, rem, now, true)
		log.Infof("%s user past 30d window — stopped", label)
		return log, actionStopped, false
	}
	return log, actionSkipped, true
}

// ── Inner paginate + batch loop ─────────────────────────────────

// processBatched is the shared inner loop: paginate, sub-batch
// with pause, count actions. ctx is checked only BETWEEN batches —
// once a batch starts, we let it finish so no half-sent state.
func (s *Service) processBatched(
	ctx context.Context,
	label string,
	cfg config,
	loadPage func(page int) ([]models.User, error),
	processOne func(u models.User) action,
) (sent, skipped, fixedFlag, stopped int) {
	page := 1
	for {
		users, err := loadPage(page)
		if err != nil {
			logrus.Errorf("%s list candidates page %d: %v", label, page, err)
			return
		}
		if len(users) == 0 {
			return
		}
		logrus.Infof("%s processing page %d (%d users)", label, page, len(users))

		for i := 0; i < len(users); i += cfg.BatchSize {
			if ctx.Err() != nil {
				logrus.Infof("%s shutdown signalled mid-tick — stopping after current batch", label)
				return
			}
			end := i + cfg.BatchSize
			if end > len(users) {
				end = len(users)
			}
			for _, u := range users[i:end] {
				switch processOne(u) {
				case actionSent:
					sent++
				case actionSkipped:
					skipped++
				case actionFixedFlag:
					fixedFlag++
				case actionStopped:
					stopped++
				}
			}
			time.Sleep(cfg.BatchPause)
		}
		page++
	}
}
