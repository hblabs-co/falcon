package system

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BindJSONOrAbort decodes the request body into target. On failure
// it writes a 400 response with the validation error message and
// returns false. **The caller MUST `return` after a false result**
// — gin doesn't short-circuit a handler just because we wrote a
// response. Pattern:
//
//	var body struct{ ... }
//	if !system.BindJSONOrAbort(c, &body) { return }
//
// Replaces the boilerplate `if err := c.ShouldBindJSON(&body); err
// != nil { c.JSON(400, gin.H{"error": err.Error()}); return }`
// that was repeated across every handler.
func BindJSONOrAbort(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}

// RequireParam reads the URL path parameter `name` and writes a
// 400 response if empty. Returns the value and a `ok` flag.
// **The caller MUST `return` after a false result**.
//
//	id, ok := system.RequireParam(c, "id")
//	if !ok {
//	    return
//	}
//
// Replaces the boilerplate `id := c.Param("id"); if id == "" {
// c.JSON(400, ...); return }` that was repeated across every
// delete-by-id handler. Generic on the param name so it covers
// `:id`, `:cv_id`, `:project_id`, etc.
func RequireParam(c *gin.Context, name string) (string, bool) {
	v := c.Param(name)
	if v == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " is required"})
		return "", false
	}
	return v, true
}

// RespondInternal writes a 500 response. With no extra args the
// body is `{"error": "internal error"}` — the safe default that
// leaks nothing to the client. Pass a single string to use a
// caller-specific message ("failed to queue scrape", "could not
// send magic link", etc) when the UX benefits from a more
// concrete hint while the actual error stays in the server logs.
//
// Like BindJSONOrAbort, **the caller MUST `return` after this**.
//
//	// generic 500
//	if err := doStuff(); err != nil {
//	    log.Errorf("doStuff: %v", err)
//	    system.RespondInternal(c)
//	    return
//	}
//
//	// caller-specific message
//	if err := publishEvent(...); err != nil {
//	    log.Errorf("publish: %v", err)
//	    system.RespondInternal(c, "failed to queue request")
//	    return
//	}
func RespondInternal(c *gin.Context, msg ...string) {
	message := "internal error"
	if len(msg) > 0 && msg[0] != "" {
		message = msg[0]
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": message})
}
