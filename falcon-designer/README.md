# falcon-designer

Local-only static design canvas for Falcon — a tiny Go server that
serves a dashboard of design projects (marketing pages, App Store
previews, email templates, brand mockups, whatever) and hot-reloads
the browser on any file change.

Not deployed to the cluster. It's a dev tool you run on your Mac.

---

## What it does

- **Dashboard at `/`** — scans `designs/` and shows a card per project
  with title, description, and tags pulled from each design's
  `meta.json`.
- **One mount per design** — `designs/<slug>/index.html` is served at
  `http://localhost:8083/<slug>/`. JSX/CSS/images under that folder
  are served as static assets relative to the design's own root.
- **Hot reload** — any save under `designs/`, `ui/`, or the shared
  screenshots tree triggers a browser refresh via Server-Sent Events.
  No bundler, no extension, no HMR runtime to install.
- **Shared asset mount** — `/assets/` maps to
  `../assets/ios-screenshots/` (configurable via `SCREENSHOTS_DIR`)
  so multiple designs can share the same iPhone captures without
  duplicating files.
- **Dark/light aware dashboard** — respects the OS preference via
  `prefers-color-scheme`.

## How to run it

```bash
cd falcon-designer
go run .
```

Output:

```
  ╭──────────────────────────────────────────────────╮
  │                 falcon-designer                  │
  ╰──────────────────────────────────────────────────╯
  ➜  Local:    http://localhost:8083
  ➜  Network:  http://192.168.1.x:8083
  watching:
      - designs
      - ui
      - ../assets/ios-screenshots
```

Open the local URL, pick a design from the dashboard, iterate.

### Env vars (`.env.example` ships a template)

| Var | Default | What it does |
|-----|---------|--------------|
| `PORT` | `8083` | HTTP listen port |
| `SCREENSHOTS_DIR` | `../assets/ios-screenshots` | Shared asset root mounted at `/assets/` |
| `DEBOUNCE_WINDOW` | `100ms` | Collapses editor save-bursts into one reload |

## How it was built

A single Go binary:

- `net/http` + `http.ServeMux` for routing (no Gin — the surface is
  tiny, vanilla is cleaner here).
- `fsnotify` for file watching (walked once at boot; per-subdir `Add`
  because macOS/Linux fsnotify isn't recursive).
- `common/ownhttp.Hub` — fan-out broadcaster shared with other Falcon
  services for SSE-style endpoints.
- `common/ownhttp.LanIPs` — lists every non-loopback IPv4 so the
  startup banner prints the URL reachable from phones on the same
  wifi.
- `html/template` for the dashboard, re-parsed on every request so
  tweaking `ui/dashboard.html` hot-reloads with no restart.

Hot reload itself is:

1. Browser connects to `/__reload` (SSE) via a 6-line script injected
   into every HTML response.
2. `fsnotify` watcher debounces file writes (editors often emit 3–5
   writes per `⌘S`) into a single `Broadcast()` call on the hub.
3. Each connected client receives a `data: reload` message and runs
   `location.reload()`.

## How to add a new design

The workflow is **design in [Claude](https://claude.ai/) (or whatever
tool produces static HTML/CSS/JS), drop the files into a subfolder,
restart the server**. No build step, no framework lock-in.

### 1. Generate the design with Claude

Two good options depending on the fidelity you want:

- **[Claude](https://claude.ai/)** (the regular chat) — prompt it to
  produce a self-contained HTML page with inline styles, a design
  system baked in, and any helper JS you need (plain `<script>` tags
  or `<script type="text/babel">` if you want in-browser JSX via
  Babel standalone — see the `appstore-preview` design for an
  example). Iterate in the artifact pane until you're happy, then
  copy the final HTML out.

- **[Claude Design](https://claude.ai/design)** — the newer Claude
  tool purpose-built for UI work. Better fit for visual iteration:
  lets you generate a design system (colors, typography, spacing
  scale) separately from the components, and export everything as
  static HTML/CSS you drop straight into `designs/<slug>/`. Use this
  when the project is UI-heavy (landing pages, mockups, marketing
  heroes) and you want reusable tokens across multiple designs.

Either way, the output lands the same: static files you copy into
`designs/<slug>/`. The Go server treats all of them identically —
it doesn't care whether the HTML came from Claude, Claude Design,
Figma export, or a hand-written file.

Expect to produce:
- An `index.html` with inline styles (or linking its own CSS).
- A design system / color palette baked into the HTML, or exported
  as a separate CSS file you include with `<link>`.
- Any helper JS / JSX.

### 2. Drop the files into `designs/<slug>/`

```bash
mkdir -p falcon-designer/designs/my-new-design
cp ~/Downloads/claude-output.html falcon-designer/designs/my-new-design/index.html
```

Slug conventions: lowercase kebab-case. The slug becomes the URL
path (`/my-new-design/`) so keep it URL-safe.

If the design has extra assets (logos, screenshots, fonts), put them
alongside `index.html`:

```
designs/my-new-design/
├── index.html
├── logo.svg
├── styles.css
└── assets/
    └── hero.jpg
```

Reference them with relative paths in the HTML (`<img src="logo.svg">`).

### 3. Add `meta.json` — REQUIRED

Every design needs a `meta.json` next to its `index.html`. The server
**will skip and loudly log any design without it** — no silent
fallbacks, because the dashboard needs real copy, not auto-generated
placeholder titles.

```json
{
  "title": "My New Design",
  "description": "Short blurb shown on the dashboard card — what this design is for.",
  "tags": ["marketing", "hero", "landing"]
}
```

Required fields:
- `title` — shown bold on the dashboard card.
- `description` — one-line subtitle under the title.

Optional:
- `tags` — lowercased keywords, rendered as small pills at the bottom
  of the card.

If you launch the server without `meta.json`, you'll see a line like:

```
[designs] skipping "my-new-design": /designs/my-new-design/meta.json is required but missing. Create it with {"title":"…","description":"…","tags":[…]}
```

Same thing if the JSON is malformed or `title`/`description` are
empty — the server tells you exactly what's wrong and which file.

### 4. Restart

```bash
# Ctrl-C the running server, then:
go run .
```

Mounts (`/my-new-design/`) are wired at boot, so a restart is
required to expose a new design. The restart takes <1s and the
dashboard card appears on the next page load.

Hot reload applies to edits **within** a design — adding a new one
needs the one-time restart.

## Shared assets (`/assets/`)

Screenshots and other cross-design assets live at
`../assets/ios-screenshots/` in the repo (configurable via
`SCREENSHOTS_DIR`). The server mounts this tree at `/assets/` so
designs reference `assets/en/2.jpg` without hard-coding the
filesystem path.

Drop a new PNG in `assets/ios-screenshots/en/` and every connected
browser reloads — useful when you're iterating on screenshots
captured from the simulator.

## Layout

```
falcon-designer/
├── main.go              # Go server — routing, watcher, SSE hub
├── go.mod / go.sum
├── .env / .env.example
├── designs/             # one subfolder per design
│   └── appstore-preview/
│       ├── index.html
│       ├── design-canvas.jsx
│       └── meta.json
└── ui/
    └── dashboard.html   # "/" template listing every design
```
