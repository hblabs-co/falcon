package signal

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/signal/email"
	"hblabs.co/falcon/signal/push"
)

// logTemplates dumps every email + push template's id and friendly
// name on startup. Called once from Module.Register so it runs after
// ConfigLogger but before the first NATS message is processed —
// catches typos in templates.yaml at boot rather than on the first
// send. Cheap (a few iterations over an in-memory map).
func logTemplates() {
	emails := email.List()
	logrus.Infof("[signal] loaded %d email template(s):", len(emails))
	for _, t := range emails {
		logrus.Infof("[signal]   email/%-20s — %s  (langs: %v)", t.ID, t.Name, t.Languages)
	}

	pushes := push.List()
	logrus.Infof("[signal] loaded %d push template(s):", len(pushes))
	for _, t := range pushes {
		logrus.Infof("[signal]   push/%-21s — %s  (langs: %v)", t.ID, t.Name, t.Languages)
	}
}
