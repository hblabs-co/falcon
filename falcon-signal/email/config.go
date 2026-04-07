package email

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"
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
	Translations map[string]map[string]string `yaml:"translations"`
	HTML         string                       `yaml:"html"`
}

type config struct {
	Assets    map[string]assetDef    `yaml:"assets"`
	Shared    map[string]string      `yaml:"shared"`
	Templates map[string]templateDef `yaml:"templates"`
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
