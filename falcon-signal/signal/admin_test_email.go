package signal

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/signal/email"
)

// handleAdminTestEmail is triggered by GET /admin/signal/test-email.
// Looks the requested template up in falcon-signal/email/templates.yaml,
// renders it for the requested language (falls back to "en") with the
// supplied vars, and sends the resulting email to every address in
// ADMIN_EMAILS. Mirrors handleAdminTestPush — same idea, different
// channel — so the operator can iterate the email copy/HTML without
// waiting on the real flow that fires it.
func (s *Service) handleAdminTestEmail(data []byte) error {
	var event struct {
		TemplateID string            `json:"template_id"`
		Lang       string            `json:"lang"`
		Vars       map[string]string `json:"vars"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal admin_test_email: %w", err)
	}
	if event.TemplateID == "" {
		return fmt.Errorf("admin_test_email: empty template_id")
	}

	// Pre-flight check so we fail fast (and log a useful list) instead
	// of letting mail.Send error out per recipient. email.List() is
	// cheap and already public.
	known := false
	ids := make([]string, 0, len(email.List()))
	for _, t := range email.List() {
		ids = append(ids, t.ID)
		if t.ID == event.TemplateID {
			known = true
		}
	}
	if !known {
		return fmt.Errorf("admin_test_email: unknown template_id %q (available: %v)", event.TemplateID, ids)
	}

	lang := event.Lang
	if lang == "" {
		lang = "en"
	}
	if event.Vars == nil {
		event.Vars = map[string]string{}
	}

	log := logrus.WithFields(logrus.Fields{"template": event.TemplateID, "lang": lang})

	if s.admin == nil || s.admin.config.Empty() {
		log.Warn("admin_test_email: ADMIN_EMAILS not set — nothing to do")
		return nil
	}

	for _, adminEmail := range s.admin.config.List() {
		if err := s.mail.Send(adminEmail, event.TemplateID, lang, event.Vars); err != nil {
			log.Errorf("admin_test_email send to %s: %v", adminEmail, err)
			continue
		}
		log.Infof("admin_test_email sent to %s", adminEmail)
	}
	return nil
}
