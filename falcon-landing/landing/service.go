package landing

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"hblabs.co/falcon/packages/ownhttp"
)

// Embedded assets: template, per-language locale JSON, and the raw
// (base64-text) falcon logo. Bundling everything into the binary means
// the pod ships as a single self-contained artefact — no ConfigMap,
// no volume mounts, no "did I forget to kubectl apply the HTML" footgun.
// Source of truth for the logo is still falcon-signal/email/assets/
// falcon_logo.txt; the Dockerfile copies the freshest one into
// falcon-landing/static/ at build time.
//
//go:embed templates/*.tmpl
var tmplFS embed.FS

//go:embed locales/*.json
var localesFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed static/falcon_logo.txt
var logoText []byte

// Supported gets imported where the order matters (precedence in picking
// a default). English stays first as the safe fallback.
var supported = []string{"en", "de", "es"}

const (
	defaultLang   = "en"
	langCookie    = "falcon_lang"
	cookieMaxAge  = 60 * 60 * 24 * 365 // 1 year — lang preference doesn't need to expire often.
	contentTypeHT = "text/html; charset=utf-8"
	// canonicalHost is the fully-qualified origin used in robots.txt
	// and sitemap.xml. Kept as a constant because search crawlers need
	// absolute URLs — relying on r.Host would let an attacker with a
	// DNS rebind trick our sitemap into pointing at their domain.
	canonicalHost = "https://falcon.hblabs.co"
	// appStoreURL is the single source of truth for the App Store
	// listing. Served via the branded /ios redirect so assets (badges,
	// locale strings, socials) link through one hop we control — if
	// the listing ever moves, we flip this constant instead of chasing
	// down every asset.
	appStoreURL = "https://apps.apple.com/app/falcon-f%C3%BCr-freelancer/id6763169883"
)

var (
	// locales[lang] holds the fully parsed translation tree for that
	// language. Map[string]any lets Go templates traverse nested keys
	// naturally via `.T.about.title`.
	locales = map[string]map[string]any{}

	// Pre-parsed template reused for every request. html/template is
	// safe for concurrent Execute() calls.
	tmpl *template.Template

	// Decoded PNG served at /favicon.png. Kept in memory — the logo is
	// ~10KB, not worth re-decoding per request.
	logoPNG []byte

	// assetVer is the cache-buster appended to every /static/* URL
	// in the rendered HTML. Initialised at process start, never
	// bumped — landing has no hot-reload, so a redeploy (= new
	// process = new token) is the natural busting cadence.
	assetVer = ownhttp.NewAssetVersion()
)

// Init loads locales + template + logo. Called once at process start.
// Returning an error (instead of panicking) lets main log+exit cleanly,
// which reads better in container logs than a stack trace.
func Init() error {
	// Locales ────────────────────────────────────────────────────
	entries, err := fs.ReadDir(localesFS, "locales")
	if err != nil {
		return fmt.Errorf("read locales dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		raw, err := fs.ReadFile(localesFS, "locales/"+e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		locales[lang] = m
	}
	for _, lang := range supported {
		if _, ok := locales[lang]; !ok {
			return fmt.Errorf("missing locale file for %q — expected locales/%s.json", lang, lang)
		}
	}

	// Template ───────────────────────────────────────────────────
	tmpl, err = template.ParseFS(tmplFS, "templates/*.tmpl")
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	// Logo — decode once ─────────────────────────────────────────
	trimmed := bytes.TrimSpace(logoText)
	decoded, err := base64.StdEncoding.DecodeString(string(trimmed))
	if err != nil {
		return fmt.Errorf("decode logo: %w", err)
	}
	logoPNG = decoded

	// Screenshots — enumerate once at startup from the embed FS.
	// This way the template just ranges over the list; adding or
	// removing a screenshot means dropping a file in static/screens/
	// and rebuilding. No template edit needed.
	if err := loadScreenshots(); err != nil {
		return fmt.Errorf("load screenshots: %w", err)
	}

	return nil
}

// screenshotFiles holds the sorted list of filenames in static/screens/
// (top-level only — thumbnails live in a subfolder). Populated once by
// loadScreenshots() and read by the template.
var screenshotFiles []string

func loadScreenshots() error {
	entries, err := fs.ReadDir(staticFS, "static/screens")
	if err != nil {
		// Missing folder is fine — gallery just renders empty.
		screenshotFiles = nil
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") {
			screenshotFiles = append(screenshotFiles, name)
		}
	}
	return nil
}

// Handler wires routes:
//   - /healthz            → 200 for k8s probes
//   - /favicon.png        → decoded logo bytes
//   - /{en,de,es}/...     → render template in that language
//   - everything else     → pick lang + 302 to /{lang}/
//
// We intentionally do NOT restrict HTTP methods to GET — any client that
// shows up (including ones that POST by mistake, or preflight OPTIONS
// sniffs) gets redirected / served the same content. Landing pages
// punish no one.
func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/favicon.png", serveLogo)
	mux.HandleFunc("/favicon.ico", serveLogo) // browsers often probe .ico first.

	mux.HandleFunc("/robots.txt", serveRobots)
	mux.HandleFunc("/sitemap.xml", serveSitemap)

	// Branded shortlink to the App Store listing — used by the
	// download badge, social posts, etc. 302 (not 301) so we can
	// retarget later without cached redirects locking visitors in.
	mux.HandleFunc("/ios", serveAppStoreRedirect)
	mux.HandleFunc("/ios/", serveAppStoreRedirect)

	// /static/* — photos, logos, anything else embedded. strip the
	// "static/" prefix since the embed FS keeps the whole tree
	// (so an asset at embed path `static/hbarcos.jpeg` should be
	// reachable at /static/hbarcos.jpeg without the prefix duplicated).
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", ownhttp.CacheImmutable(http.FileServer(http.FS(staticSub)))))

	mux.HandleFunc("/", root)

	// Basic access log so k8s stdout gives a useful trail. Shared
	// middleware keeps the log shape identical to falcon-api's gin
	// logger, so ops can `grep path=/healthz` across every service
	// and get the same field layout.
	return ownhttp.LoggingMiddleware(mux)
}

// root is the catch-all that either (a) renders the page in the path's
// language, (b) redirects to /{lang}/ picked from cookie → header →
// default. Kept in one handler so the routing story is "all paths land
// here", easy to reason about.
func root(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	first := path
	if i := strings.IndexByte(path, '/'); i >= 0 {
		first = path[:i]
	}

	// Already on /{lang}/ — render directly, set cookie so the next
	// raw visit (e.g. typing the domain without a lang prefix) lands
	// in the right place.
	if contains(supported, first) {
		setLangCookie(w, first)
		render(w, first)
		return
	}

	// No lang in URL → pick one, 302 to /{lang}/. Using 302 (not 301)
	// because the "right" language for a given user can change over
	// time — a cached 301 would lock them in.
	lang := pickLang(r)
	setLangCookie(w, lang)
	http.Redirect(w, r, "/"+lang+"/", http.StatusFound)
}

// pickLang runs the precedence chain:
//
//  1. cookie          → returning visitor keeps their choice
//  2. Accept-Language → first-time browser matches OS locale
//  3. defaultLang     → safe English fallback
//
// URL-path language is handled in the caller; this function only
// answers "when there's no language in the URL, which one do we want?"
func pickLang(r *http.Request) string {
	// Cookie.
	if c, err := r.Cookie(langCookie); err == nil && contains(supported, c.Value) {
		return c.Value
	}

	// Accept-Language header. Standard lib doesn't parse it for us, but
	// we only care about the primary tag of the first preference — good
	// enough for 99% of real traffic.
	if raw := r.Header.Get("Accept-Language"); raw != "" {
		for _, tag := range strings.Split(raw, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			// Drop the quality value (`;q=0.8`) and any region subtag.
			if i := strings.IndexAny(tag, ";-_"); i >= 0 {
				tag = tag[:i]
			}
			tag = strings.ToLower(tag)
			if contains(supported, tag) {
				return tag
			}
		}
	}

	return defaultLang
}

func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     langCookie,
		Value:    lang,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: false, // JS on the page can read it to stay in sync.
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
}

func render(w http.ResponseWriter, lang string) {
	w.Header().Set("Content-Type", contentTypeHT)
	// Tell crawlers + browsers the page body changes with cookie +
	// Accept-Language, so shared caches don't serve an English copy
	// to a German user who hit the same URL.
	w.Header().Set("Vary", "Cookie, Accept-Language")
	// Short cache: content is mostly static, but we do want a lang
	// change via cookie to show up quickly.
	w.Header().Set("Cache-Control", "public, max-age=60")

	data := map[string]any{
		"Lang":    lang,
		"T":       locales[lang],
		"Now":     time.Now().Year(),
		"Screens": screenshotFiles,
		"V":       assetVer.Current(),
	}
	if err := tmpl.ExecuteTemplate(w, "index.html.tmpl", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func serveLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	_, _ = w.Write(logoPNG)
}

// serveAppStoreRedirect forwards /ios to the App Store listing. Apple
// auto-routes to the visitor's regional storefront based on their
// device locale, so a single target URL is enough for all languages.
func serveAppStoreRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, appStoreURL, http.StatusFound)
}

// serveRobots answers crawler probes. Everything on this landing is
// meant to be indexed, so we allow all user agents and point them to
// the sitemap so they discover every language variant.
//
// Social-preview crawlers (Facebook, Twitter, LinkedIn, Slack, etc.)
// get explicit per-UA allow blocks on top of the wildcard. Some of
// these bots read robots.txt with a strict parser that ignores
// `User-agent: *` if they also see their own name — so we spell it
// out to remove any ambiguity when they fetch og:* tags.
func serveRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	var b strings.Builder
	b.WriteString("User-agent: *\nAllow: /\n\n")
	// Named link-preview bots — duplicate allow so strict parsers can't
	// misread the wildcard rule.
	for _, ua := range []string{
		"facebookexternalhit",
		"facebookcatalog",
		"meta-externalagent",
		"Twitterbot",
		"LinkedInBot",
		"Slackbot",
		"Slackbot-LinkExpanding",
		"Discordbot",
		"TelegramBot",
		"WhatsApp",
		"Applebot",
	} {
		b.WriteString("User-agent: " + ua + "\nAllow: /\n\n")
	}
	b.WriteString("Sitemap: " + canonicalHost + "/sitemap.xml\n")
	_, _ = w.Write([]byte(b.String()))
}

// serveSitemap emits an XML sitemap with all three language variants
// plus hreflang alternates, so Google serves the right URL to users
// based on their browser locale. Priority stays at 1.0 on English
// (the fallback) and 0.9 on the others — we do want all three
// indexed, but English is the "primary" for link-share URLs that
// arrive without a path prefix.
func serveSitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")

	// One <url> per language. The <xhtml:link> elements tell Google
	// "these URLs are translations of each other" — helps avoid the
	// duplicate-content penalty and routes users to the right variant.
	for _, lang := range supported {
		priority := "0.9"
		if lang == defaultLang {
			priority = "1.0"
		}
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>" + canonicalHost + "/" + lang + "/</loc>\n")
		for _, alt := range supported {
			b.WriteString(`    <xhtml:link rel="alternate" hreflang="` + alt + `" href="` + canonicalHost + "/" + alt + `/"/>` + "\n")
		}
		b.WriteString(`    <xhtml:link rel="alternate" hreflang="x-default" href="` + canonicalHost + "/" + defaultLang + `/"/>` + "\n")
		b.WriteString("    <changefreq>monthly</changefreq>\n")
		b.WriteString("    <priority>" + priority + "</priority>\n")
		b.WriteString("  </url>\n")
	}

	b.WriteString(`</urlset>` + "\n")
	_, _ = w.Write([]byte(b.String()))
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
