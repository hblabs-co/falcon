package auth

// HMAC-signed unsubscribe tokens + bulk lookup over `auth_optouts`.
// Lives in the auth package because `auth_optouts` is part of the
// auth subsystem (see notes/AUTH.md), and the HMAC token gates the
// public `/unsubscribe` endpoint defined in unsubscribe.go.
//
// Why HMAC, not JWT: a one-purpose signed token has a smaller blast
// radius than a session token. If a footer URL leaks, the attacker
// can only opt out the named email, not access user data. See
// AUTH.md "Por qué HMAC es la solución correcta".

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// purposeReminders is the constant tag baked into every reminders-
// scope token so that a token leaked here CANNOT be replayed against
// a future opt-out flow with a different scope (e.g. "delete
// account"). New scopes get new purposes; same secret is fine across
// them because the HMAC input differs.
const purposeReminders = "optout:reminders"

// envHMACSecret is the env var that holds the signing key. Single
// secret across services — must be identical in falcon-api and
// falcon-signal or tokens won't validate.
const envHMACSecret = "OPTOUT_HMAC_SECRET"

// envPublicURL is the public origin used to build absolute
// unsubscribe URLs. When unset, RemindersURL falls back to a sane
// default so dev environments don't crash.
const envPublicURL = "LANDING_PUBLIC_URL"

// SignReminders returns the URL-safe HMAC token that pairs with
// `email` for the conversion-reminders opt-out scope. Returns an
// error only if the secret env var is missing — callers can decide
// whether to fail or fall back to "no unsub link in this email".
func SignReminders(email string) (string, error) {
	secret, err := environment.Read(envHMACSecret)
	if err != nil {
		return "", fmt.Errorf("missing %s — required for opt-out token signing: %w", envHMACSecret, err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(datasource.NormalizeEmail(email) + "|" + purposeReminders))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// ValidateReminders returns true iff `token` is the correct HMAC
// for `email` under the reminders scope. Constant-time compare to
// avoid timing oracles. Any error reading the secret returns false
// (fail-closed at validation time).
func ValidateReminders(email, token string) bool {
	// SignReminders applies NormalizeEmail internally — passing
	// the raw caller value is fine; both sides converge on the
	// same canonical form before HMACing.
	expected, err := SignReminders(email)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(token))
}

// RemindersURL builds the full unsubscribe link for embedding in
// email footers. Format:
//
//	https://<LANDING_PUBLIC_URL>/unsubscribe?email=<email>&token=<hmac>
//
// Returns an error if the secret is missing — caller decides
// whether to skip the unsubscribe footer or fail the send.
func RemindersURL(email string) (string, error) {
	token, err := SignReminders(email)
	if err != nil {
		return "", err
	}
	base := environment.ReadOptional(envPublicURL, "https://falcon.hblabs.co")
	q := url.Values{}
	q.Set("email", email)
	q.Set("token", token)
	return base + "/unsubscribe?" + q.Encode(), nil
}

// OptOutReminders handles `POST /reminders/opt-out` — the
// authenticated counterpart to the public `/unsubscribe` page.
// Mounted under the JWT-protected `/me` group in falcon-api so
// the effective URL is `POST /me/reminders/opt-out`. Reads the
// user_id from the gin context (populated by the JWT middleware),
// resolves the email, and writes the same `auth_optouts` row that
// the HMAC flow writes — single mechanism for both authenticated
// users and pure intents.
//
// Silences conversion reminders (cv_upload, login_after_cv, future
// intent reminders) for the authenticated user. The opt-out lives
// on email — not on user_id — because the same mechanism must
// cover pure intents (no `users` doc yet) when the future
// unsubscribe-link flow exists. See AUTH.md.
func OptOutReminders(c *gin.Context) {
	uidAny, _ := c.Get("user_id")
	uid, _ := uidAny.(string)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	ctx := c.Request.Context()

	// Resolve the email from the user_id — the JWT carries the id,
	// not the email, and the opt-out lookup at reminder time is
	// keyed by email so we need that here.
	var u models.User
	if err := system.GetStorage().GetByField(ctx, constants.MongoUsersCollection, "id", uid, &u); err != nil {
		logrus.Errorf(logPrefix+" reminders opt-out: load user %s: %v", uid, err)
		system.RespondInternal(c)
		return
	}

	doc := models.AuthOptOut{
		ID:         gonanoid.Must(),
		Email:      u.Email,
		Kind:       models.AuthOptOutKindConversionReminders,
		OptedOutAt: time.Now(),
		Source:     models.AuthOptOutSourceAuthenticated,
		UserID:     uid,
	}

	// Set with filter on (email, kind) is the upsert that matches
	// the unique compound index — re-running opt-out is a no-op
	// rather than an error.
	if err := system.GetStorage().Set(ctx,
		constants.MongoAuthOptOutsCollection,
		bson.M{"email": u.Email, "kind": models.AuthOptOutKindConversionReminders},
		doc,
	); err != nil {
		logrus.Errorf(logPrefix+" reminders opt-out: write %s: %v", u.Email, err)
		system.RespondInternal(c, "failed to opt out")
		return
	}

	logrus.Infof(logPrefix+" reminders opt-out: %s opted out of conversion reminders", u.Email)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ActiveOptOutsByEmail bulk-fetches every auth_optouts row whose
// email is in the candidate page and matches the given kind, and
// returns a set keyed by email. Bulk version of "is this email
// opted out of this kind?" for callers that work over a page of
// candidates (reminder loops, future admin dashboards).
//
// auth_optouts rows have no expires_at concept — presence in the
// collection IS the opt-out, until the admin explicitly removes
// the row. So the set is just "found rows by email".
//
// Caller policy on Mongo error: this function returns the error
// as-is. Reminder loops wrap this with fail-open + warn (silencing
// legitimately opted-out users on a transient blip is worse than
// briefly noop'ing the gate). Other callers may want strict
// failure — the choice stays with them.
func ActiveOptOutsByEmail(ctx context.Context, emails []string, kind models.AuthOptOutKind) (map[string]bool, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	var rows []models.AuthOptOut
	if err := system.GetStorage().GetMany(ctx,
		constants.MongoAuthOptOutsCollection,
		bson.M{
			"email": bson.M{"$in": emails},
			"kind":  kind,
		},
		&rows,
	); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(rows))
	for _, r := range rows {
		set[r.Email] = true
	}
	return set, nil
}
