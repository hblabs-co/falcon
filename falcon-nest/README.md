# falcon-nest

Local dev portal for the Falcon stack — a single page that lists every
service, every infra component (Mongo, NATS, Qdrant, MinIO, Ollama),
and the port-forward commands you keep forgetting. Open
`http://localhost:8080`, scan in two seconds.

Not deployed to the cluster (yet — see TODOs).

---

## What it shows

- **Falcon services** — every Go binary that ships under `falcon-*`.
  HTTP services link to their local URL; NATS-only consumers don't
  have one. Names + descriptions pulled from `common/constants/services.go`
  so a new service registered there only needs one card row in
  `portal.go`.
- **Infrastructure** — Mongo, NATS, Qdrant, MinIO (API + console),
  optional Ollama. Each card opens the local console / monitoring UI
  where one exists.
- **Port-forwards I always forget** — `kubectl port-forward` recipes
  for the recurring "I need Studio 3T against remote Mongo" /
  "I need to peek at the prod NATS monitor" tasks. Copy-paste the
  command, run, profit.
- **Hero meta** — env, host, user, started-at. Quick sanity check
  that you're looking at the right session.

## How it was built

- **Go server**: `net/http` + `html/template`. No framework, no Gin —
  the surface is one page.
- **Static assets**: `static/index.html` + `static/styles.css` +
  `static/falcon-logo.png` (copied from `assets/brand/falcon@1024.png`).
  Color tokens mirror `falcon-designer/ideas/ios-v2/DESIGN_SYSTEM.md`
  (brand 50–900 + neutrals + dark/light variants) so the portal feels
  like the rest of the iOS-v2 design language.
- **Hot reload**: `modules/hotreload/` — same SSE + fsnotify + debounce
  helper that powers `falcon-designer`. Saves to anything under
  `static/` reload the connected browser.
- **Port-forward, service, infra catalogues**: hard-coded in
  `portal.go` for now. Will become plugin-driven via per-section
  `meta.json` (TODO #3 below).

## Running it

```bash
cd falcon-nest
go run .
```

Default port is **8080**. Override via `PORT` in `.env` if the
production-ish port name clashes locally.

## Adding a new card

Edit `portal.go`:

```go
// In falconServices() / infraComponents() — same struct shape:
{
    Name:        constants.ServiceFooBar,   // or hard-coded for infra
    Kind:        "http",                    // "http" | "nats" | "infra" | "tool"
    KindLabel:   "HTTP",                    // text shown inside the badge
    URL:         "http://localhost:8085",   // optional — empty hides the link
    Description: "What this service does in one sentence.",
}
```

Restart the server — `mux` registration is at boot, so a new card
needs `Ctrl-C && go run .`. Browser hot-reloads the resulting HTML.

For port-forwards:

```go
// In portForwards():
{
    Label:   "What it's for",
    Command: "kubectl port-forward -n falcon svc/foo 1234:1234",
    Target:  "localhost:1234",
}
```

## Layout

```
falcon-nest/
├── main.go              # server, hot-reload, template render
├── portal.go            # hard-coded service/infra/port-forward catalogues
├── go.mod
├── .env / .env.example
├── README.md
└── static/
    ├── index.html       # one-page template (Go's html/template)
    ├── styles.css       # design-system tokens (brand 50–900 + neutrals)
    └── falcon-logo.png  # copy of assets/brand/falcon@1024.png
```

## TODOs

These are tracked-but-not-implemented and intentionally so — the
portal needs to earn its keep before each gets built.

### 1. Auth (prod only)

The portal exposes ports + commands + URLs that point at internal
infra. Local dev: zero auth, fine. Cluster deploy: needs gating.

Options worth exploring when this lands:

- **SSO** via Tailscale / Cloudflare Access / Auth0 — least code,
  works for the operator-only audience.
- **Passkey** with WebAuthn — modern, no shared secret, works with
  Touch ID on the dev's Mac.
- **Master key** — one env var, one shared header. Crude but ships
  in 30 minutes.

Dev path stays auth-free either way (no friction during iteration).

### 2. Config watchers

Today the catalogues are baked into `portal.go`. Real ports / URLs /
manifests change in two places:

- **Dev**: `.env` files in each service. Watch them, re-derive cards.
- **Prod**: Kubernetes manifests under `deployment/`. Watch via
  fsnotify in dev (operator edits locally) or a k8s informer in
  cluster (live state).

End state: cards reflect the source of truth automatically — change
`PORT=9000` in a service's `.env`, the portal updates.

### 3. Plugin framework

Each section becomes a self-contained plugin:

```
falcon-nest/plugins/
├── falcon-services/
│   ├── meta.json     # title, render-as: "cards"
│   └── source        # where to read the data from
├── infra/
│   └── meta.json
└── port-forwards/
    └── meta.json
```

Server enumerates `plugins/`, loads each, renders the section. Same
pattern as `falcon-designer` uses for `designs/`. Adding a new
section becomes a `mkdir` + `meta.json` + restart.
