package auth

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/clientmeta"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/ownhttp"
	"hblabs.co/falcon/packages/system"
)

// magicTokenTTL is how long a freshly-issued magic-link stays
// valid. Short on purpose: phishing windows shrink as fast as we
// can keep the UX tolerable, and 15 min covers "click it now or
// re-request" without forcing users to wait for a third party.
const magicTokenTTL = 15 * time.Minute

// rawTokenLength is the character length of the magic-link nonce
// produced by gonanoid. 32 chars (URL-safe alphabet, ~190 bits of
// entropy) is well above the threshold where brute-force becomes
// infeasible even with the full 15-min TTL window.
const rawTokenLength = 32

// maxRawTokenLength caps what /auth/verify accepts in `?token=`.
// Issued tokens are exactly rawTokenLength chars; doubling that
// gives slack for URL-encoding artifacts without letting a client
// hand us 10 MB of garbage to SHA-256 (reports.md rec #7).
const maxRawTokenLength = 64

// RequestMagicLink handles `POST /auth/magic` — the entry point
// of the auth flow. Body is JSON: `{ email, device_id, platform }`.
// Side-effects (in order): records an auth_intent row, checks
// auth_blocks, applies the per-email throttle, then creates a
// single-use magic token and publishes `signal.magic_link` so
// falcon-signal delivers the email.
func RequestMagicLink(c *gin.Context) {
	// device_id is required and capped at 128 chars. Required so
	// `revokeDeviceJWTs` in /auth/verify has a real anchor to scope
	// the "one session per device" invariant — without it, a Test=
	// false token has nothing to revoke against (reports.md B20).
	// Capped because auth_intents is append-only without TTL —
	// garbage device_ids would accumulate in Mongo forever
	// (reports.md rec #7). Real iOS IDFV/vendor-id strings are 36
	// chars (UUID), so 128 is generous slack.
	var body struct {
		Email    string `json:"email"     binding:"required,email"`
		DeviceID string `json:"device_id" binding:"required,max=128"`
		Platform string `json:"platform"  binding:"required,oneof=ios"`
	}
	if !system.BindJSONOrAbort(c, &body) {
		return
	}
	body.Email = datasource.NormalizeEmail(body.Email)

	ctx := c.Request.Context()
	log := logrus.WithFields(logrus.Fields{"email": body.Email})

	// 1. Audit log — runs first so every attempt leaves a trace,
	//    including blocked and throttled ones. Fail-closed: the
	//    throttle in step 3 counts these rows, so if the write fails
	//    the count is wrong and an attacker bursts through (B14).
	client := clientmeta.Capture(c.GetHeader, c.ClientIP())
	if err := recordIntent(ctx, body.Email, body.DeviceID, body.Platform, client); err != nil {
		log.Errorf(logPrefix+" record intent: %v", err)
		system.RespondInternal(c)
		return
	}

	// 2. auth_blocks check — short-circuit before doing any work.
	//    Generic 403 with no detail; we don't tell a bot which
	//    emails are blocked. Fail-open on Mongo error.
	if block, err := ActiveBlock(ctx, body.Email); err != nil {
		log.Warnf(logPrefix+" auth_blocks lookup failed (fail-open): %v", err)
	} else if block != nil {
		log.Infof(logPrefix+" magic link blocked — reason=%s scope=%s", block.Reason, block.Scope)
		c.JSON(http.StatusForbidden, gin.H{"error": "request rejected"})
		return
	}

	// 3. Throttle — count recent intents (just-inserted row included).
	//    Above threshold → silent success response. Admins are NOT
	//    exempt: a previous bypass let `email=admin@…` evade the cap
	//    and burn Mailjet quota (reports.md sec.2). Operators that
	//    need a fresh link use falcon-admin's test-link endpoint.
	//    Fail-closed on Count error so a Mongo blip doesn't disable
	//    the throttle.
	since := time.Now().Add(-time.Hour)
	recent, err := recentIntentsCount(ctx, body.Email, since)
	if err != nil {
		log.Errorf(logPrefix+" recent intents count: %v", err)
		system.RespondInternal(c)
		return
	}
	if recent > MaxIntentsPerHour {
		log.Warnf(logPrefix+" throttle: %s pidió %d links en última hora — skipping send", body.Email, recent)
		c.JSON(http.StatusAccepted, gin.H{"message": "magic link sent"})
		return
	}

	rawToken, err := mintMagicToken(ctx, body.Email, body.DeviceID, body.Platform, log)
	if err != nil {
		system.RespondInternal(c, "could not create magic link")
		return
	}

	if err := publishMagicEvent(ctx, c, body.Email, rawToken); err != nil {
		log.Errorf(logPrefix+" publish signal.magic_link: %v", err)
		system.RespondInternal(c, "could not send magic link")
		return
	}

	log.Infof(logPrefix+" magic link sent to %s", body.Email)
	c.JSON(http.StatusAccepted, gin.H{"message": "magic link sent"})
}

// mintMagicToken generates the random token, persists the row in
// MongoAuthTokensCollection, and returns the raw token (which becomes
// the URL the user clicks). Token.Validate() is NOT called here:
// the gin binding tags on `handleMagic.body` (`oneof=ios` for
// Platform, `required` for DeviceID) already enforce the same
// rules at the boundary, so a second pass would be dead code
// (reports.md B11). falcon-admin's test-link path keeps calling
// Validate because it constructs the doc directly without binding.
func mintMagicToken(ctx context.Context, email, deviceID, platform string, log *logrus.Entry) (string, error) {
	rawToken := gonanoid.Must(rawTokenLength)
	now := time.Now()
	doc := models.Token{
		ID:        gonanoid.Must(),
		Type:      models.TokenTypeMagicLink,
		Email:     email,
		DeviceID:  deviceID,
		Platform:  platform,
		TokenHash: tokenHash(rawToken),
		ExpiresAt: now.Add(magicTokenTTL),
		Used:      false,
		CreatedAt: now,
	}

	if err := system.GetStorage().Set(ctx, constants.MongoAuthTokensCollection,
		bson.M{"id": doc.ID}, doc,
	); err != nil {
		log.Errorf(logPrefix+" save magic token: %v", err)
		return "", err
	}
	return rawToken, nil
}

// publishMagicEvent fires `signal.magic_link` so falcon-signal
// delivers the email. The deeplink is built via BuildMagicURL so
// the prod handler and the falcon-admin test-link path stay in
// sync (reports.md B5, N2).
func publishMagicEvent(ctx context.Context, c *gin.Context, email, rawToken string) error {
	evt := models.MagicLinkRequestedEvent{
		Email:     email,
		MagicLink: BuildMagicURL(rawToken),
		Platform:  ownhttp.DetectPlatform(c.GetHeader("User-Agent")),
	}
	return system.Publish(ctx, constants.SubjectSignalMagicLink, evt)
}

// BuildMagicURL constructs the deeplink the user clicks from the
// magic-link email or the admin test-link response. Single source
// of truth for (a) the APP_SCHEME env-var lookup and (b) the
// QueryEscape of the raw token — every caller that hands a magic
// URL to a client MUST go through this so the two stay symmetric
// (reports.md B5, N2).
func BuildMagicURL(rawToken string) string {
	return environment.ReadOptional("APP_SCHEME", "falcon") + "://auth?token=" + url.QueryEscape(rawToken)
}
