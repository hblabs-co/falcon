package main

import "strings"

// component is one renderable card on the dashboard. Used for both
// Falcon Go services and infra (Mongo, NATS, etc.) — same shape.
// Loaded from config.yaml at startup (see config.go); field names
// are the yaml keys.
type component struct {
	Name        string `yaml:"name"`               // human / kebab name (the title on the card)
	Kind        string `yaml:"kind"`               // CSS class on the badge: "http" | "nats" | "infra" | "tool"
	KindLabel   string `yaml:"kindLabel"`          // label shown in the badge
	URL         string `yaml:"url,omitempty"`      // local URL — empty means "no clickable target"
	Description string `yaml:"description"`
	PortHint    string `yaml:"portHint,omitempty"` // shown in place of URL when no clickable URL exists

	// Icon is a Lucide icon name (https://lucide.dev/icons), rendered
	// in the card header via <i data-lucide="…"></i>. Falls back to
	// kindIcons[Kind] when empty.
	Icon string `yaml:"icon,omitempty"`

	// StatusURL is the endpoint the status poller hits with HTTP GET.
	// Optional — when empty and URL is HTTP, the poller defaults to
	// `<URL>/healthz`. Only set this when the component's health
	// endpoint lives on a different path (Qdrant /readyz, MinIO
	// /minio/health/live) or on the root (services without /healthz).
	StatusURL string `yaml:"statusURL,omitempty"`

	// StatusHost is "host:port" for daemons that don't expose HTTP
	// (Mongo's wire protocol, etc.). The poller does a TCP dial with
	// a 1s timeout. Empty when StatusURL is set.
	StatusHost string `yaml:"statusHost,omitempty"`

	// Tags are freeform labels shown as small chips under the card's
	// description. Use them to group services by surface (ios, web,
	// dev), layer (core, ai, storage) or behavior (background, one-
	// shot, optional).
	Tags []string `yaml:"tags,omitempty"`
}

// kindIcons maps the Kind classifier to a default Lucide icon. Used
// when a component doesn't override Icon. Names from
// https://lucide.dev/icons.
var kindIcons = map[string]string{
	"http":  "globe",
	"ws":    "radio",
	"nats":  "radio-tower",
	"infra": "database",
	"tool":  "terminal-square",
}

// EffectiveIcon returns the icon to render for this card: the
// component's own Icon if set, else the kind's default. Exposed as a
// method so the template can call `{{.EffectiveIcon}}` without
// reaching into the kindIcons map.
func (c component) EffectiveIcon() string {
	if c.Icon != "" {
		return c.Icon
	}
	if v, ok := kindIcons[c.Kind]; ok {
		return v
	}
	return "circle" // fallback that's always valid in Lucide
}

// EffectiveStatusURL returns the URL the status poller should probe
// for this component. Single rule:
//
//   - StatusURL set → use it verbatim.
//   - URL is HTTP + StatusURL empty → default to `<URL>/healthz`.
//   - URL is non-HTTP → empty (the poller falls back to StatusHost).
//
// Applies to every kind (http, ws, infra, tool). A component that
// doesn't actually expose /healthz should declare StatusURL
// explicitly (e.g. minio console points to its root; qdrant points
// to /readyz).
//
// Exported so the template can render a "open health endpoint"
// shortcut on HTTP cards.
func (c component) EffectiveStatusURL() string {
	if c.StatusURL != "" {
		return c.StatusURL
	}
	if !strings.HasPrefix(c.URL, "http://") && !strings.HasPrefix(c.URL, "https://") {
		return ""
	}
	if strings.HasSuffix(c.URL, "/healthz") {
		return c.URL
	}
	return strings.TrimRight(c.URL, "/") + "/healthz"
}

// portForward is a recurring port-forward command the operator never
// remembers (Studio 3T → remote Mongo, etc.). Loaded from config.yaml.
type portForward struct {
	Label   string `yaml:"label"`
	Command string `yaml:"command"`
	Target  string `yaml:"target"`
}
