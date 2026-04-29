package email

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed templates.yaml
var templatesYAML string

// --- YAML schema ---

type assetDef struct {
	File string `yaml:"file"`
	MIME string `yaml:"mime"`
}

type templateDef struct {
	// Name is the human-readable label for the template (e.g.
	// "Magic link login"). Distinct from the YAML key, which is the
	// stable id used in code. Exposed via List() so the admin UI /
	// HTTP layer can render a friendly picker without duplicating
	// the mapping anywhere.
	Name         string                       `yaml:"name"`
	Translations map[string]map[string]string `yaml:"translations"`
	HTML         string                       `yaml:"html"`
}

type config struct {
	Assets    map[string]assetDef    `yaml:"assets"`
	Shared    map[string]string      `yaml:"shared"`
	Templates map[string]templateDef `yaml:"templates"`
}

// TemplateMeta is the public-facing summary of a single template,
// safe to serialise to JSON for an admin endpoint that wants to list
// what's available. Doesn't include the HTML body or the translation
// strings — those are internal to render.
type TemplateMeta struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Languages []string `json:"languages"`
}

// List returns every email template's metadata, sorted by id so the
// output is stable across calls. Use from HTTP handlers that want to
// expose the catalogue (e.g. an admin "send test email" picker).
func List() []TemplateMeta {
	out := make([]TemplateMeta, 0, len(cfg.Templates))
	for id, def := range cfg.Templates {
		langs := make([]string, 0, len(def.Translations))
		for lang := range def.Translations {
			langs = append(langs, lang)
		}
		out = append(out, TemplateMeta{ID: id, Name: def.Name, Languages: langs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for i := range out {
		sort.Strings(out[i].Languages)
	}
	return out
}

// --- Package state ---

var cfg config
var compiledTemplates map[string]*template.Template
var assetDataURIs map[string]string

func packageDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func init() {
	if err := yaml.Unmarshal([]byte(templatesYAML), &cfg); err != nil {
		panic(fmt.Sprintf("parse templates.yaml: %v", err))
	}

	// Load assets from disk using paths defined in the YAML.
	dir := packageDir()
	assetDataURIs = make(map[string]string, len(cfg.Assets))
	for name, asset := range cfg.Assets {
		data, err := os.ReadFile(filepath.Join(dir, asset.File))
		if err != nil {
			panic(fmt.Sprintf("load asset %q (%s): %v", name, asset.File, err))
		}
		assetDataURIs[name] = "data:" + asset.MIME + ";base64," + strings.TrimSpace(string(data))
	}

	// Compile HTML templates.
	compiledTemplates = make(map[string]*template.Template, len(cfg.Templates))
	for name, def := range cfg.Templates {
		if def.HTML == "" {
			continue
		}
		tpl, err := template.New(name).Parse(def.HTML)
		if err != nil {
			panic(fmt.Sprintf("compile email template %q: %v", name, err))
		}
		compiledTemplates[name] = tpl
	}
}
