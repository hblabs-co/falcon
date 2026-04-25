// falcon-designer — local-only static design canvas.
//
// Layout:
//
//	designs/<slug>/index.html     entry point of one design
//	designs/<slug>/meta.json      optional title/description/tags
//	ui/dashboard.html             "/" — index lists every design
//
// Run:    go run .
// Open:   http://localhost:8083 (and the LAN URL printed at startup)
//
// Iteration loop: edit any file under designs/ or the SCREENSHOTS_DIR
// and the connected browser auto-reloads via SSE. Adding a new design
// is a `mkdir designs/<slug> && touch designs/<slug>/index.html` away.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/fswatch"
	"hblabs.co/falcon/packages/ownhttp"
	"hblabs.co/falcon/packages/system"
)

// designsDir holds one subfolder per design, each self-contained
// (HTML + assets + meta.json). The server enumerates this on every
// dashboard hit so adding/removing a design needs zero restarts.
const designsDir = "designs"

// uiDir holds dashboard.html and any future cross-design UI chrome.
// Kept separate from designs/ so the dashboard isn't itself listed
// as a design.
const uiDir = "ui"

// reloadScriptTag is the one-liner injected into every HTML response.
// The actual JS lives at /__reload.js so it's not duplicated per
// design and stays editable as a single source of truth.
const reloadScriptTag = `<script src="/__reload.js"></script>`

// reloadScript is served at /__reload.js. Self-contained so each
// design's HTML doesn't need to ship its own copy.
const reloadScript = `(function () {
  var es = new EventSource('/__reload');
  es.onmessage = function () { console.log('[hot-reload] reloading…'); location.reload(); };
  es.onerror   = function () { /* server restart — let browser auto-reconnect */ };
})();`

// design is one entry rendered on the dashboard. Slug is the URL
// path segment ("/<slug>/"); everything else is metadata for display.
type design struct {
	Slug        string   `json:"-"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func main() {
	port := environment.ParseInt("PORT", 8083)
	screenshotsDir := environment.ReadOptional("SCREENSHOTS_DIR", "../assets/ios-screenshots")
	debounceWindow := environment.ParseDuration("DEBOUNCE_WINDOW", "100ms")

	ctx := system.Boot(constants.ServiceDesigner,
		system.WithPort(port),
		system.WithoutStorage(), // local-only dev tool, no Mongo
		system.WithBannerExtras(
			"watching:",
			"    - "+designsDir,
			"    - "+uiDir,
			"    - "+screenshotsDir,
		),
	)

	hub := ownhttp.NewHub()

	// Hot-reload watcher. Same shape as falcon-nest's: shared
	// `packages/fswatch` helper handles recursive AddTree, debounce,
	// Chmod-noise filtering, and per-burst onChange. We just provide
	// the trees and the broadcast callback.
	watcher, err := fswatch.New(debounceWindow)
	if err != nil {
		log.Fatalf("watcher: %v", err)
	}
	defer watcher.Close()
	for _, root := range []string{designsDir, uiDir, screenshotsDir} {
		if err := watcher.AddTree(root); err != nil {
			log.Printf("warn: watch %s: %v (continuing)", root, err)
		}
	}
	go watcher.Run(ctx, func(path string) {
		log.Printf("[hot-reload] change detected → broadcasting reload (%s)", path)
		hub.Broadcast()
	})

	mux := http.NewServeMux()
	ownhttp.RegisterVanillaHealthz(mux, constants.ServiceDesigner)

	// Hot-reload SSE + the JS that subscribes to it.
	mux.HandleFunc("/__reload", hub.ServeSSE("reload"))
	mux.HandleFunc("/__reload.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write([]byte(reloadScript))
	})

	// Shared screenshot tree. Mounted at /assets/ so designs reference
	// `assets/<lang>/<id>.jpg` without knowing the filesystem layout.
	mux.Handle("/assets/", http.StripPrefix("/assets/",
		http.FileServer(http.Dir(screenshotsDir))))

	// Dashboard at exactly "/" — re-enumerates designs on every hit so
	// dropping a new folder shows up without restart.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		serveDashboard(w, r)
	})

	// One mount per design. The discovery happens once at boot — adding
	// a new design folder requires a restart so the route gets wired.
	// Acceptable trade for a 5-line discovery loop.
	for _, d := range scanDesigns() {
		prefix := "/" + d.Slug + "/"
		root := filepath.Join(designsDir, d.Slug)
		mux.Handle(prefix, designHandler(prefix, root))
		log.Printf("mounted /%s/ → %s", d.Slug, root)
	}

	srv := ownhttp.NewServerModule(constants.ServiceDesigner, fmt.Sprintf(":%d", port), mux)
	if err := system.RunForever(ctx, srv); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// scanDesigns walks `designs/` and returns one entry per subfolder
// that contains BOTH an index.html AND a meta.json. Designs missing
// either are skipped with a loud, actionable log line so the
// operator knows exactly what to fix. meta.json is required on
// purpose — the dashboard needs a title + description per card, and
// defaulting silently to the slug name hides typos and untitled
// drafts from the person browsing the dashboard.
func scanDesigns() []design {
	entries, err := os.ReadDir(designsDir)
	if err != nil {
		log.Printf("warn: read %s: %v", designsDir, err)
		return nil
	}
	var out []design
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		root := filepath.Join(designsDir, slug)

		if _, err := os.Stat(filepath.Join(root, "index.html")); err != nil {
			log.Printf("[designs] skipping %q: missing index.html — drop an HTML file there or remove the folder", slug)
			continue
		}

		metaPath := filepath.Join(root, "meta.json")
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			log.Printf("[designs] skipping %q: %s is required but missing. Create it with {\"title\":\"…\",\"description\":\"…\",\"tags\":[…]}", slug, metaPath)
			continue
		}
		var d design
		if err := json.Unmarshal(raw, &d); err != nil {
			log.Printf("[designs] skipping %q: %s has invalid JSON — %v", slug, metaPath, err)
			continue
		}
		if d.Title == "" || d.Description == "" {
			log.Printf("[designs] skipping %q: %s is missing `title` or `description` (both required)", slug, metaPath)
			continue
		}
		d.Slug = slug
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// serveDashboard renders ui/dashboard.html with the live list of
// designs. Re-parses on every request — fast (one file, ~few KB)
// and means edits to dashboard.html show up without restart.
func serveDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(filepath.Join(uiDir, "dashboard.html"))
	if err != nil {
		http.Error(w, "dashboard template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{"Designs": scanDesigns()}); err != nil {
		http.Error(w, "render dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	body := injectReloadScript(buf.Bytes())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	_, _ = w.Write(body)
}

// designHandler serves a single design's folder as a static tree
// rooted at `prefix`. HTML responses get the reload-script tag
// injected; everything else passes through.
func designHandler(prefix, root string) http.Handler {
	fs := http.StripPrefix(prefix, http.FileServer(http.Dir(root)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !looksLikeHTMLRequest(r.URL.Path) {
			fs.ServeHTTP(w, r)
			return
		}
		rec := &captureWriter{header: http.Header{}, body: &bytes.Buffer{}, status: 200}
		fs.ServeHTTP(rec, r)

		body := injectReloadScript(rec.body.Bytes())
		for k, v := range rec.header {
			if strings.EqualFold(k, "Content-Length") {
				continue
			}
			w.Header()[k] = v
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.WriteHeader(rec.status)
		_, _ = w.Write(body)
	})
}

// injectReloadScript splices the SSE client right before </body>, or
// appends it if no </body> tag is present.
func injectReloadScript(body []byte) []byte {
	if bytes.Contains(body, []byte("</body>")) {
		return bytes.Replace(body, []byte("</body>"),
			[]byte(reloadScriptTag+"\n</body>"), 1)
	}
	return append(body, []byte(reloadScriptTag)...)
}

func looksLikeHTMLRequest(path string) bool {
	if strings.HasSuffix(path, "/") || path == "" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".html" || ext == ".htm" || ext == ""
}

// captureWriter buffers the response body so designHandler can splice
// the reload tag into HTML before flushing.
type captureWriter struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func (c *captureWriter) Header() http.Header         { return c.header }
func (c *captureWriter) Write(b []byte) (int, error) { return c.body.Write(b) }
func (c *captureWriter) WriteHeader(status int)      { c.status = status }

var _ http.ResponseWriter = (*captureWriter)(nil)
