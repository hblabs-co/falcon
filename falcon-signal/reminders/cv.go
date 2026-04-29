package reminders

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/signal/push"
)

// CV-upload reminder loop. A user that signed up via magic link
// but never uploaded a CV is invisible to the matching pipeline
// (no CV → no embeddings → no matches), so this loop nudges them
// via email + push on the cadence defined in runtime.go (1d grace
// → 1/day → 1/week → stop at 30d, ~10 envíos total).
//
// State per user lives in MongoUserRemindersCollection keyed by
// (user_id, kind=cv_upload). The flag `users.cv_uploaded` is the
// cheap primary filter — flipped to true by falcon-storage on a
// successful index, reconciled at boot by ensureUserCVFlag. The
// loop also does a defensive recheck against `cvs` per-user
// before sending: if the flag drifted from reality, fix it and
// skip the send.

const cvUploadDeeplink = "falcon://"

var cvGuard = &tickGuard{label: "[cv-reminder]"}

// RunCVLoop is started as a goroutine by the signal service.
// Wakes every CV_REMINDER_INTERVAL (default 1h).
func (s *Service) RunCVLoop(ctx context.Context) {
	(&loop{
		label:    "cv-reminder",
		interval: environment.ParseDuration("CV_REMINDER_INTERVAL", "1h"),
		guard:    cvGuard,
		process:  s.processCVQueue,
	}).run(ctx)
}

// processCVQueue is the per-tick body. Selects users with no CV,
// joined ≤30d ago. Asks the runtime to bulk-load this loop's
// reminder rows + the per-page set of users that DO have a usable
// CV (drives the defensive recheck without N Counts).
func (s *Service) processCVQueue(ctx context.Context) {
	s.processQueue(ctx, queueSpec{
		label:             "[cv-reminder]",
		fixedActionLabel:  "fixed_flag",
		filter:            bson.M{"cv_uploaded": false},
		prefetchKinds:     []models.UserReminderKind{models.UserReminderKindCVUpload},
		prefetchCVIndexed: true,
		processOne:        s.processCVUser,
	})
}

// processCVUser handles one user. Cadence runs against a reminder
// row pre-loaded from the page cache (no Get per user). The
// defensive recheck checks the cache's set of CV-indexed user_ids
// (no Count per user). The drift fix is still per-user because
// it's rare.
func (s *Service) processCVUser(ctx context.Context, u models.User, now time.Time, page *pageCache) action {
	rem := page.reminder(u.ID, models.UserReminderKindCVUpload)
	log, act, send := s.decideFromCadence(ctx, "[cv-reminder]", models.UserReminderKindCVUpload, u, now, rem)
	if !send {
		return act
	}

	// Defensive recheck via the page cache. Same semantics as the
	// old per-user Count (status indexed/normalizing/normalized) but
	// pre-computed for the whole page in buildPageCache.
	if page.hasCVIndexed(u.ID) {
		_, _ = system.GetStorage().UpdateOne(ctx,
			constants.MongoUsersCollection,
			bson.M{"id": u.ID},
			bson.M{"$set": bson.M{"cv_uploaded": true, "updated_at": now}},
		)
		log.Info("[cv-reminder] flag drift — user has indexed CV; fixed flag")
		return actionFixedFlag
	}
	log.Info("[cv-reminder] recheck OK — user has no usable CV, proceeding to send")

	lang := s.lang.ResolveUserLanguage(u.Email, "ios")
	s.sendCV(ctx, u, lang, log)
	s.upsertReminder(ctx, u.ID, models.UserReminderKindCVUpload, rem, now, false)
	return actionSent
}

// sendCV dispatches the email and the push for one user. Best-
// effort per channel: email failure doesn't block the push and
// vice versa. Errors are logged, not surfaced upward, because the
// cadence guard above is the rate-limiter.
func (s *Service) sendCV(ctx context.Context, u models.User, lang string, log *logrus.Entry) {
	emailVars := map[string]string{
		"upload_link":     cvUploadDeeplink,
		"unsubscribe_url": unsubscribeURLOrEmpty(u.Email, log),
	}
	if err := s.mail.Send(u.Email, "cv_reminder", lang, emailVars); err != nil {
		log.Errorf("[cv-reminder] email: %v", err)
	} else {
		log.Infof("[cv-reminder] email sent (lang=%s)", lang)
	}

	// Push — only if the user has registered an iOS device token.
	var tokens []models.IOSDeviceToken
	if err := system.GetStorage().GetAllByField(ctx,
		constants.MongoIOSDeviceTokensCollection, "user_id", u.ID, &tokens); err != nil {
		log.Warnf("[cv-reminder] fetch device tokens: %v", err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	payload, err := push.Render("cv_reminder", lang)
	if err != nil {
		log.Errorf("[cv-reminder] render push: %v", err)
		return
	}
	extras := map[string]string{
		"deeplink":    cvUploadDeeplink,
		"template_id": "cv_reminder",
	}
	var (
		stale []string
		sent  int
	)
	for _, dt := range tokens {
		if err := s.push.SendTemplated(ctx, dt.Token, payload, extras); err != nil {
			if s.push.IsStaleToken(err) {
				stale = append(stale, dt.Token)
				continue
			}
			log.Errorf("[cv-reminder] push FAILED — user_id=%s email=%s device=%s… err=%v",
				u.ID, u.Email, safePrefix(dt.Token, 8), err)
			continue
		}
		sent++
		log.Infof("[cv-reminder] push sent — user_id=%s email=%s lang=%s device=%s…",
			u.ID, u.Email, lang, safePrefix(dt.Token, 8))
	}

	if sent > 0 {
		log.Infof("[cv-reminder] reminder delivered — user_id=%s email=%s lang=%s push_devices=%d",
			u.ID, u.Email, lang, sent)
	}

	if len(stale) > 0 {
		if err := system.GetStorage().DeleteManyByFieldIn(ctx,
			constants.MongoIOSDeviceTokensCollection, "token", stale); err != nil {
			log.Errorf("[cv-reminder] delete stale tokens: %v", err)
		}
	}
}
