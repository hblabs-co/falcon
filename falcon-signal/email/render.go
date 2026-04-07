package email

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// renderData is passed to every HTML template.
type renderData struct {
	T      map[string]template.HTML // translated strings (HTML-safe)
	Asset  map[string]template.URL // asset data URIs (trusted for src attributes)
	Shared map[string]string       // shared config values
	Var    map[string]any          // per-email variables (strings or template.URL for links)
}

// T returns the translated strings for a template and language.
// Falls back to "en" if the language is not found.
func T(templateName, lang string) map[string]string {
	if def, ok := cfg.Templates[templateName]; ok {
		if t, ok := def.Translations[lang]; ok {
			return t
		}
		if t, ok := def.Translations["en"]; ok {
			return t
		}
	}
	return map[string]string{}
}

// Render builds the HTML for a template with the given language and variables.
func Render(templateName, lang string, vars map[string]string) (string, error) {
	tpl, ok := compiledTemplates[templateName]
	if !ok {
		return "", fmt.Errorf("email template %q not found", templateName)
	}

	strs := T(templateName, lang)
	htmlStrings := make(map[string]template.HTML, len(strs))
	for k, v := range strs {
		htmlStrings[k] = template.HTML(v)
	}

	trustedAssets := make(map[string]template.URL, len(assetDataURIs))
	for k, v := range assetDataURIs {
		trustedAssets[k] = template.URL(v)
	}

	typedVars := make(map[string]any, len(vars))
	for k, v := range vars {
		if strings.Contains(k, "link") || strings.Contains(k, "url") || strings.HasPrefix(v, "http") || strings.HasPrefix(v, "falcon://") {
			typedVars[k] = template.URL(v)
		} else {
			typedVars[k] = v
		}
	}

	data := renderData{
		T:      htmlStrings,
		Asset:  trustedAssets,
		Shared: cfg.Shared,
		Var:    typedVars,
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render %q: %w", templateName, err)
	}
	return buf.String(), nil
}
