// Package clientmeta captures HTTP request metadata that's useful
// for understanding the caller of any endpoint — auth intents
// today, future audit logs / abuse detection / analytics tomorrow.
//
// Design rule: capture, don't interpret. We snapshot the headers
// verbatim and let the consumer decide what to do with them. Only
// `IP` is computed (via gin's c.ClientIP() which already does the
// X-Real-IP / X-Forwarded-For dance) — the proxy headers are still
// kept raw so a downstream investigator can see the full chain.
//
// Cloudflare headers (CF-*) are present only when CF sits in front
// of the request; Sec-CH-UA-* only on browsers that opt into Client
// Hints. Both fields end up empty for native iOS calls — that's
// fine, omitempty keeps the BSON small.
package clientmeta

import "net"

// ClientMeta is the canonical bag of "what we know about the
// caller from this HTTP request". Embed it as a sub-document in
// any model that wants to record this metadata; reuse the Capture
// helper at the call site.
//
// Snake-case BSON tags match the rest of Falcon's models. JSON
// tags are identical for symmetry.
type ClientMeta struct {
	// IP is the best-effort real client IP, resolved by gin's
	// c.ClientIP() (which honours X-Real-IP and X-Forwarded-For).
	// Always present — for tests it's "::1".
	IP string `json:"ip,omitempty" bson:"ip,omitempty"`

	// UserAgent is the raw User-Agent header. Not parsed here —
	// downstream tooling (admin UI, analytics) does that.
	UserAgent string `json:"user_agent,omitempty" bson:"user_agent,omitempty"`

	// AcceptLanguage is the raw Accept-Language header. Useful for
	// localising emails before the user has any explicit locale
	// preference saved.
	AcceptLanguage string `json:"accept_language,omitempty" bson:"accept_language,omitempty"`

	// AppVersion is the X-App-Version custom header — populated
	// once the iOS / future web client starts sending it. See
	// IOS_QUESTIONS.md.
	AppVersion string `json:"app_version,omitempty" bson:"app_version,omitempty"`

	// Sec-CH-UA-* (Client Hints) — only browsers that opt into the
	// Client Hints standard send these. Native iOS via URLSession
	// does NOT, so expect empty for the iOS app today. Keep the
	// fields so when the web client lands they slot in.
	SecCHUA         string `json:"sec_ch_ua,omitempty" bson:"sec_ch_ua,omitempty"`
	SecCHUAPlatform string `json:"sec_ch_ua_platform,omitempty" bson:"sec_ch_ua_platform,omitempty"`

	// Proxy headers — the raw chain. ClientIP() already picks the
	// "right" IP from these, but keeping the raw values lets ops
	// see whether the request hit a proxy and which one.
	XRealIP       string `json:"x_real_ip,omitempty" bson:"x_real_ip,omitempty"`
	XForwardedFor string `json:"x_forwarded_for,omitempty" bson:"x_forwarded_for,omitempty"`

	// Cloudflare-specific headers — only present when CF sits in
	// front of the request. CFRay is the unique request ID (great
	// for support tickets that need to correlate with CF logs);
	// CFCountry is the ISO-3166-1 alpha-2 country code that CF
	// derives from the source IP (cheaper than running our own
	// GeoIP lookup); CFConnectingIP is CF's view of the real
	// client IP, useful as a sanity check against ClientIP().
	CFRay          string `json:"cf_ray,omitempty" bson:"cf_ray,omitempty"`
	CFCountry      string `json:"cf_country,omitempty" bson:"cf_country,omitempty"`
	CFConnectingIP string `json:"cf_connecting_ip,omitempty" bson:"cf_connecting_ip,omitempty"`
}

// Capture builds a ClientMeta from a generic header lookup
// function and a pre-resolved client IP. Framework-agnostic on
// purpose — packages/ doesn't depend on gin or any HTTP router.
// Callers adapt their framework: gin → clientmeta.Capture(c.GetHeader, c.ClientIP()).
//
// Pass a function (not a map or http.Header) so the implementation
// can stay lazy: gin's GetHeader is already cheap, but a caller
// who normalises their headers can plug their own normaliser in.
func Capture(getHeader func(string) string, clientIP string) ClientMeta {
	cfIP, _ := NormalizeIP(getHeader("CF-Connecting-IP"))
	ip, _ := NormalizeIP(clientIP)
	return ClientMeta{
		IP:              ip,
		UserAgent:       getHeader("User-Agent"),
		AcceptLanguage:  getHeader("Accept-Language"),
		AppVersion:      getHeader("X-App-Version"),
		SecCHUA:         getHeader("Sec-CH-UA"),
		SecCHUAPlatform: getHeader("Sec-CH-UA-Platform"),
		XRealIP:         getHeader("X-Real-IP"),
		XForwardedFor:   getHeader("X-Forwarded-For"),
		CFRay:           getHeader("CF-Ray"),
		CFCountry:       getHeader("CF-IPCountry"),
		CFConnectingIP:  cfIP,
	}
}

// NormalizeIP parses `ip` and returns its RFC 5952 canonical form
// plus an `ok` flag. If the input doesn't parse, the original
// string is returned unchanged with ok=false (audit captures
// shouldn't drop data; admin filters can use the flag to reject
// with 400). Same primitive on both sides of the boundary so the
// canonical form written to Mongo equals the canonical form an
// admin filter looks up — no false-empty results from `2001:DB8`
// vs `2001:db8` mismatches.
func NormalizeIP(ip string) (string, bool) {
	if ip == "" {
		return "", false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip, false
	}
	return parsed.String(), true
}

// LogFields returns the captured metadata as a map for log
// enrichment. Returns map[string]any (NOT logrus.Fields) so this
// package stays free of any logging-library dependency — modules
// can pass the result to logrus.WithFields, zap.Any, or whatever.
// Only non-empty fields are included so logs stay readable.
func (m ClientMeta) LogFields() map[string]any {
	out := make(map[string]any, 11)
	add := func(key, val string) {
		if val != "" {
			out[key] = val
		}
	}
	add("ip", m.IP)
	add("user_agent", m.UserAgent)
	add("accept_language", m.AcceptLanguage)
	add("app_version", m.AppVersion)
	add("sec_ch_ua", m.SecCHUA)
	add("sec_ch_ua_platform", m.SecCHUAPlatform)
	add("x_real_ip", m.XRealIP)
	add("x_forwarded_for", m.XForwardedFor)
	add("cf_ray", m.CFRay)
	add("cf_country", m.CFCountry)
	add("cf_connecting_ip", m.CFConnectingIP)
	return out
}
