// Package push owns the push-notification template framework, mirror
// of the email package. YAML-driven so adding a new template is a
// single edit to push/templates.yaml — no Go code changes per
// template.
//
// Render(name, lang) returns a Payload (title + subtitle + body +
// category + sound) the APNs sender can stamp directly onto a
// notification. Variable substitution inside translation strings is
// out of scope for v1 — every translation is a plain string. If a
// future template needs interpolation we can promote each field to
// a text/template at compile time without breaking the public API.
package push

import (
	_ "embed"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed templates.yaml
var templatesYAML string

// templateDef is the in-memory shape of one entry under
// `templates:` in templates.yaml.
type templateDef struct {
	// Name is the human-readable label (e.g. "CV upload reminder").
	// Distinct from the YAML key, which is the stable id used in
	// code. Surfaced via List() so an admin UI can render a friendly
	// picker without duplicating the mapping anywhere.
	Name         string                       `yaml:"name"`
	Translations map[string]map[string]string `yaml:"translations"`
	Category     string                       `yaml:"category"`
	Sound        string                       `yaml:"sound"`
}

type config struct {
	Templates map[string]templateDef `yaml:"templates"`
}

var cfg config

// TemplateMeta is the public-facing summary of a single push template,
// safe to serialise to JSON for an admin endpoint that wants to list
// what's available. Mirrors email.TemplateMeta — same shape across
// channels so a single picker can list both.
type TemplateMeta struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Category  string   `json:"category,omitempty"`
	Languages []string `json:"languages"`
}

// List returns every push template's metadata, sorted by id so the
// output is stable across calls. Use from HTTP handlers that want to
// expose the catalogue (e.g. /admin/signal/test-push picker).
func List() []TemplateMeta {
	out := make([]TemplateMeta, 0, len(cfg.Templates))
	for id, def := range cfg.Templates {
		langs := make([]string, 0, len(def.Translations))
		for lang := range def.Translations {
			langs = append(langs, lang)
		}
		out = append(out, TemplateMeta{
			ID:        id,
			Name:      def.Name,
			Category:  def.Category,
			Languages: langs,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for i := range out {
		sort.Strings(out[i].Languages)
	}
	return out
}

// Has reports whether a template id exists. Kept as a cheap precheck
// for endpoints that want to fail fast on a bad ?template_id= before
// publishing the NATS event.
func Has(name string) bool {
	_, ok := cfg.Templates[name]
	return ok
}

func init() {
	if err := yaml.Unmarshal([]byte(templatesYAML), &cfg); err != nil {
		panic(fmt.Sprintf("parse push/templates.yaml: %v", err))
	}
}
