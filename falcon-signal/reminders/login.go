package reminders

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/models"
)

// Login-after-CV reminder loop. Targets users who uploaded a CV
// but have no registered iOS device token — they either never
// opened the app after upload, signed out, or uninstalled. Without
// a token we can't push, so this loop only sends EMAIL. Same
// cadence and Berlin window as the cv-reminder.
//
// Defensive recheck reads users.last_logged_in_at — set on every
// /auth/verify, backfilled at boot by ensureUserLastLogin. Tokens
// are NOT a reliable signal because tokens.expires_at has a TTL
// index; a user who logged in last year (JWT TTL'd out) would
// look "never logged in" if we counted JWT rows.

// crossKindGap is the minimum gap between any cv-reminder and a
// follow-up login-reminder to the same user. Without it, a user
// who uploads their CV right after getting a cv-reminder could
// receive a login-reminder on the very next hourly tick (johannes
// case: cv-reminder 09:09, login-reminder 11:09 same morning).
const crossKindGap = 6 * time.Hour

var loginGuard = &tickGuard{label: "[login-reminder]"}

// RunLoginLoop is started as a goroutine by the signal service.
// Wakes every LOGIN_REMINDER_INTERVAL (default 1h).
func (s *Service) RunLoginLoop(ctx context.Context) {
	(&loop{
		label:    "login-reminder",
		interval: environment.ParseDuration("LOGIN_REMINDER_INTERVAL", "1h"),
		guard:    loginGuard,
		process:  s.processLoginQueue,
	}).run(ctx)
}

// processLoginQueue is the per-tick body. Selects users with a CV
// (cv_uploaded=true), joined ≤30d ago. Asks the runtime to bulk-
// load BOTH login_after_cv (own kind, for cadence) and cv_upload
// (cross-kind, for the cooldown gap).
func (s *Service) processLoginQueue(ctx context.Context) {
	s.processQueue(ctx, queueSpec{
		label:            "[login-reminder]",
		fixedActionLabel: "already_active",
		filter:           bson.M{"cv_uploaded": true},
		prefetchKinds: []models.UserReminderKind{
			models.UserReminderKindLoginAfterCV,
			models.UserReminderKindCVUpload,
		},
		processOne: s.processLoginUser,
	})
}

// processLoginUser handles one user. Cadence runs against pre-
// loaded reminder state from the page cache. The cross-kind
// cooldown also reads from the cache (cv_upload kind was bulk-
// loaded alongside login_after_cv).
func (s *Service) processLoginUser(ctx context.Context, u models.User, now time.Time, page *pageCache) action {
	rem := page.reminder(u.ID, models.UserReminderKindLoginAfterCV)
	log, act, send := s.decideFromCadence(ctx, "[login-reminder]", models.UserReminderKindLoginAfterCV, u, now, rem)
	if !send {
		return act
	}

	// Defensive recheck: has this user EVER completed the magic-
	// link → JWT exchange? Read straight off users.last_logged_in_at
	// which is on the User doc we already loaded — free check.
	if !u.LastLoggedInAt.IsZero() {
		log.Info("[login-reminder] user has logged in at least once — skipping")
		return actionFixedFlag
	}

	// Cross-kind cooldown: if cv-reminder fired recently to this
	// same user, give them breathing room before the login nudge.
	// Mutually exclusive in steady state (cv-reminder filter is
	// cv_uploaded=false, login-reminder is cv_uploaded=true), but
	// a user who uploads in between two ticks crosses both filters
	// within an hour. cv_upload reminder state was bulk-loaded by
	// the runtime when this page was built — no extra round-trip.
	cvRem := page.reminder(u.ID, models.UserReminderKindCVUpload)
	if !cvRem.LastAt.IsZero() && now.Sub(cvRem.LastAt) < crossKindGap {
		log.Infof("[login-reminder] cv-reminder sent %s ago — skipping (cross-kind cooldown %s)",
			now.Sub(cvRem.LastAt).Round(time.Minute), crossKindGap)
		return actionSkipped
	}

	lang := s.lang.ResolveUserLanguage(u.Email, "ios")
	s.sendLogin(ctx, u, lang, log)
	s.upsertReminder(ctx, u.ID, models.UserReminderKindLoginAfterCV, rem, now, false)
	return actionSent
}

// sendLogin dispatches the login_reminder email. Email-only by
// design — if the user had a device token we wouldn't be here.
// app_link is wired to the bare falcon:// scheme so iOS just
// opens the app cold; once a deeplink for "go to login screen"
// exists it can replace this constant.
func (s *Service) sendLogin(ctx context.Context, u models.User, lang string, log *logrus.Entry) {
	_ = ctx // reserved — mailjet client uses its own context internally
	vars := map[string]string{
		"app_link":        cvUploadDeeplink, // bare falcon:// — no specific screen yet
		"unsubscribe_url": unsubscribeURLOrEmpty(u.Email, log),
	}
	if err := s.mail.Send(u.Email, "login_reminder", lang, vars); err != nil {
		log.Errorf("[login-reminder] email FAILED — user_id=%s email=%s err=%v", u.ID, u.Email, err)
		return
	}
	log.Infof("[login-reminder] reminder delivered — user_id=%s email=%s lang=%s", u.ID, u.Email, lang)
}
