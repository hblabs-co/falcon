// falcon-nest — local dev portal.
//
// Single page that lists every Falcon service (HTTP + NATS-only),
// every infra component (Mongo, NATS, Qdrant, MinIO, Ollama), and a
// cheat sheet of port-forward commands you keep forgetting (Studio
// 3T → remote Mongo, Lens → realtime, etc.).
//
// Not for production. Runs locally on :8080 and hot-reloads itself
// when static/* or config.yaml changes.
//
// TODO (tracked but not implemented):
//
//  1. Auth — SSO / passkey / master-key for the production deploy.
//     Locally we don't need it; in prod the portal would expose
//     whatever ports/creds the operator can see, which is sensitive.
//  2. Kubernetes-aware config source — in prod, read manifests via
//     informer instead of the local config.yaml so URLs/ports stay
//     in sync with the actual cluster without manual edits.
//  3. Plugin framework — each section reads a meta.json that
//     describes how to render it (badge / card / table / chart).
//     Today config.yaml is a single flat catalogue; the plugin pass
//     would let external tools contribute sections without code
//     changes.
package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/user"
	"time"

	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/fswatch"
	"hblabs.co/falcon/packages/ownhttp"
	"hblabs.co/falcon/packages/system"
)

const staticDir = "static"

func main() {
	system.LoadEnvs()

	port := environment.ParseInt("PORT", 8080)
	debounceWindow := environment.ParseDuration("DEBOUNCE_WINDOW", "100ms")
	statusInterval := environment.ParseDuration("STATUS_INTERVAL", "10s")

	// config.yaml is the catalogue (services + infra + port-forwards).
	// Loaded once at startup; a background watcher reloads on edits.
	cfg, err := newConfigStore(configFile)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// assetVer is the cache-busting token appended to every static
	// asset URL via `?v={{.V}}`. Bumped on hot-reload events so the
	// next HTML render emits a fresh URL and Chrome re-fetches even
	// with the immutable Cache-Control header on /static/*.
	assetVer := ownhttp.NewAssetVersion()
	// Two hubs on purpose: status snapshots fan out every poll tick,
	// hot-reload broadcasts only on file changes. Sharing one hub used
	// to make every status tick also trigger location.reload() on the
	// SSE client, refreshing the page every 10 s.
	statusHub := ownhttp.NewHub()
	reloadHub := ownhttp.NewHub()

	hr, err := fswatch.NewHotReloader(debounceWindow, reloadHub)
	if err != nil {
		log.Fatalf("hotreload: %v", err)
	}
	defer hr.Close()
	if err := hr.WatchTree(staticDir); err != nil {
		log.Printf("warn: watch %s: %v (continuing)", staticDir, err)
	}

	// Watch config.yaml independently from static/*. Keeping them on
	// separate watchers means a template edit doesn't re-parse YAML,
	// and a YAML edit doesn't fire on every Chmod event in static/.
	// On every successful reload we poke the hot-reload hub so every
	// connected browser refreshes and sees the new catalogue.
	ctx := context.Background()
	go func() {
		if err := cfg.watch(ctx, configFile, debounceWindow); err != nil {
			log.Printf("config watch: %v", err)
		}
	}()
	go func() {
		ch := cfg.subscribe()
		for range ch {
			assetVer.Bump()
			hr.Broadcast()
		}
	}()

	// Hot-reload also bumps the asset version on every file change
	// in static/ — same purpose as above (next HTML render emits
	// new ?v= query params), just for a different change source.
	hr.OnReload(assetVer.Bump)

	// Background status poller — every STATUS_INTERVAL it probes the
	// current component set (HTTP probe or TCP dial) and writes into
	// `store`. Passing cfg.components as a closure means a reloaded
	// config takes effect on the very next tick without plumbing
	// additional notifications through.
	store := newStatusStore()
	go runStatusPoller(ctx, store, statusHub, statusInterval, cfg.components)

	mux := http.NewServeMux()
	hr.Mount(mux)
	hr.Run(ctx)
	ownhttp.RegisterVanillaHealthz(mux, constants.ServiceNest)
	registerConfigAPI(mux, configFile)

	// Live status feed over WebSocket. Chose WS over SSE here because
	// this is the one endpoint intended to survive a k8s ingress later;
	// hot-reload (SSE) is dev-only and never ships. Payload is the full
	// {name: status} snapshot on every tick — tiny, easy to diff client-
	// side, and idempotent if the socket reconnects.
	mux.HandleFunc("/ws/status", statusHub.ServeWS(func() any {
		return map[string]any{"statuses": store.snapshot()}
	}))

	// Static files — favicon, logo, CSS — under /static/. Cached
	// aggressively (one week, immutable); cache busting is handled
	// at the URL level via `?v={{.V}}` query params bumped on every
	// reload, same trick as falcon-landing.
	mux.Handle("/static/", ownhttp.CacheImmutable(http.StripPrefix("/static/",
		http.FileServer(http.Dir(staticDir)))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		serveDashboard(w, hr, store, cfg, assetVer)
	})

	startCfg := cfg.snapshot()
	system.PrintStartupBannerAndPort(constants.ServiceNest, port,
		"sections:",
		fmt.Sprintf("    - %d Falcon services", len(startCfg.Services)),
		fmt.Sprintf("    - %d infra components", len(startCfg.Infra)),
		fmt.Sprintf("    - %d port-forward reminders", len(startCfg.PortForwards)),
	)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// serveDashboard renders static/index.html with the live data sets
// pulled from the config store (so a hot-reloaded YAML edit shows up
// on the next page load) and the live status snapshot.
//
// The `status` template func is wired here (not on a shared template
// instance) so it always reads the current `store` — the poller
// goroutine updates it asynchronously.
func serveDashboard(w http.ResponseWriter, hr *fswatch.HotReload, store *statusStore, cfg *configStore, assetVer *ownhttp.AssetVersion) {
	tmpl, err := template.New("dashboard").
		Funcs(template.FuncMap{
			"status": func(name string) string { return string(store.get(name)) },
		}).
		ParseFiles(staticDir + "/index.html")
	if err != nil {
		http.Error(w, "template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hostname, _ := os.Hostname()
	currentUser := "?"
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}

	now := time.Now()
	snap := cfg.snapshot()
	data := map[string]any{
		"Apps":         snap.Services,
		"Infra":        snap.Infra,
		"PortForwards": snap.PortForwards,
		"Env":          environment.ReadOptional("FALCON_ENV", "dev"),
		"Host":         hostname,
		"User":         currentUser,
		"StartedAt":    now.Format("2006-01-02 15:04"),
		// Year is rendered into the © footer. Pulled server-side
		// (instead of hardcoded) so the page stays accurate at the
		// turn of the year without anyone touching the template.
		"Year": now.Year(),
		// V is appended as `?v=…` to every static asset URL — see
		// ownhttp.AssetVersion for the dev/prod cache-busting story.
		"V": assetVer.Current(),
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "index.html", data); err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
		return
	}
	body := hr.InjectScript(buf.Bytes())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	// HTML must always reflect the current ?v= token so Chrome picks
	// up new asset URLs on the next paint after a reload.
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(body)
}
