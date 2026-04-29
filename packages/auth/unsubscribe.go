package auth

import (
	_ "embed"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// unsubscribeHTML is loaded at compile time from unsubscribe.html
// next to this file. Logo + favicon are referenced by absolute URL
// against the production landing (falcon.hblabs.co/favicon.png) —
// no asset embedded into this binary for them. If the landing is
// down the page still works, just without the icon.
//
//go:embed unsubscribe.html
var unsubscribeHTML string

var unsubscribeTpl = template.Must(template.New("unsub").Parse(unsubscribeHTML))

// Unsubscribe handles the public `GET /unsubscribe` link hit from
// the footer of every reminder email. GET so clicking in any
// client (Gmail, Outlook, Apple Mail) just works without forms or
// JS. The HMAC token in the query string is the authorisation —
// no JWT, no cookie. See `optout.go` in this package for the token
// primitives.
//
// Writes one row in auth_optouts with source=unsubscribe_link and
// user_id="" (the link works for both authenticated users and pure
// intents — we don't resolve the user_id here because the link
// must work without a JWT).
//
// Trade-off: antivirus / link prefetchers in some clients hit GET
// URLs proactively. Impact is small (one fewer reminder before the
// user notices) and matches what every major ESP does.
func Unsubscribe(c *gin.Context) {
	email := datasource.NormalizeEmail(c.Query("email"))
	token := c.Query("token")

	if email == "" || token == "" {
		renderUnsubscribePage(c, http.StatusBadRequest, false, "")
		return
	}
	if !ValidateReminders(email, token) {
		logrus.Warnf(logPrefix+" invalid token for %s", email)
		renderUnsubscribePage(c, http.StatusForbidden, false, "")
		return
	}

	ctx := c.Request.Context()
	doc := models.AuthOptOut{
		ID:         gonanoid.Must(),
		Email:      email,
		Kind:       models.AuthOptOutKindConversionReminders,
		OptedOutAt: time.Now(),
		Source:     models.AuthOptOutSourceUnsubscribeLink,
	}

	// Set with (email, kind) filter is the upsert that respects
	// the unique compound index — re-clicking the link is a no-op
	// instead of a duplicate-key error.
	if err := system.GetStorage().Set(ctx,
		constants.MongoAuthOptOutsCollection,
		bson.M{"email": email, "kind": models.AuthOptOutKindConversionReminders},
		doc,
	); err != nil {
		logrus.Errorf(logPrefix+" write %s: %v", email, err)
		renderUnsubscribePage(c, http.StatusInternalServerError, false, email)
		return
	}

	logrus.Infof(logPrefix+" %s opted out via unsubscribe link", email)
	renderUnsubscribePage(c, http.StatusOK, true, email)
}

func renderUnsubscribePage(c *gin.Context, status int, ok bool, email string) {
	title := "Unsubscribed"
	if !ok {
		title = "Unsubscribe failed"
	}
	c.Status(status)
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := unsubscribeTpl.Execute(c.Writer, struct {
		Title string
		OK    bool
		Email string
	}{Title: title, OK: ok, Email: email}); err != nil {
		// Status + headers already flushed; can't switch to 500.
		// Log so a silent blank body shows up in errors instead of
		// being invisible (reports.md B15).
		logrus.Errorf(logPrefix+" render unsubscribe page: %v", err)
	}
}
