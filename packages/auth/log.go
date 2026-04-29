package auth

// logPrefix is the bracketed tag prepended to every log line
// emitted by this package. Centralising it means a future audit /
// log-routing rule (Loki label, fluentd tag, alerting filter)
// only has to know one string to capture every auth message.
//
// Usage convention:
//
//	logrus.Warnf(logPrefix+" foo failed: %v", err)
//	log.Errorf(logPrefix+" save token: %v", err)
//
// Don't add sub-prefixes (e.g. "[auth][unsubscribe]") — the
// message body carries the per-flow context.
const logPrefix = "[auth]"
