package push

import "fmt"

// Payload is what Render returns: the strings that go straight onto
// an apns2 payload (title / subtitle / body) plus optional category +
// sound. Empty Subtitle is a valid state — apns2.Payload skips it
// when not set.
type Payload struct {
	Title    string
	Subtitle string
	Body     string
	Category string
	Sound    string
}

// Render resolves a templateID + language to a fully populated Payload.
// Falls back to "en" when the requested language isn't defined for
// this template, mirroring email.T behaviour. Returns an error when
// the templateID itself is unknown so callers fail loudly instead of
// silently sending a blank push.
func Render(templateID, lang string) (Payload, error) {
	def, ok := cfg.Templates[templateID]
	if !ok {
		return Payload{}, fmt.Errorf("push template %q not found", templateID)
	}

	strs := def.Translations[lang]
	if strs == nil {
		strs = def.Translations["en"]
	}
	if strs == nil {
		return Payload{}, fmt.Errorf("push template %q has no translations", templateID)
	}

	return Payload{
		Title:    strs["title"],
		Subtitle: strs["subtitle"],
		Body:     strs["body"],
		Category: def.Category,
		Sound:    def.Sound,
	}, nil
}
