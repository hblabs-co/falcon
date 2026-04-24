package signal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	environment "hblabs.co/falcon/common/environment"
)

// defaultAlertWindow is used when ADMIN_ALERT_WINDOW is unset or invalid.
const defaultAlertWindow = 2 * time.Minute

// bufferedAlert groups one or more identical alerts (same kind + name +
// platform) received within a single flush window. The original subject is
// kept for the notification; Count tracks how many times the same alert
// arrived during the window so the email/push can say "occurred N times".
type bufferedAlert struct {
	subject adminAlertSubject
	count   int
	firstAt time.Time
	lastAt  time.Time
}

// alertBuffer accumulates adminAlertSubject entries between flush cycles.
// Entries are deduplicated by (kind, name, platform) within each window:
// multiple arrivals of the same alert increment a counter instead of creating
// separate entries. This prevents email/push spam while preserving the full
// count for the notification message.
//
// Thread-safe: handleAdminAlert (NATS consumer goroutine) and the flush loop
// goroutine run concurrently.
type alertBuffer struct {
	mu    sync.Mutex
	byKey map[string]*bufferedAlert
	order []string // insertion order for deterministic flush
}

func newAlertBuffer() *alertBuffer {
	return &alertBuffer{
		byKey: make(map[string]*bufferedAlert),
	}
}

// Add pushes a subject into the buffer. If the same (kind, name, platform)
// already exists in the current window, only the counter and lastAt are
// updated — the original subject (from the first occurrence) is preserved.
func (b *alertBuffer) Add(subject adminAlertSubject) {
	key := fmt.Sprintf("%s:%s:%s", subject.Kind, subject.Name, subject.Platform)
	now := time.Now()

	b.mu.Lock()
	defer b.mu.Unlock()

	if entry, ok := b.byKey[key]; ok {
		entry.count++
		entry.lastAt = now
		return
	}

	b.byKey[key] = &bufferedAlert{
		subject: subject,
		count:   1,
		firstAt: now,
		lastAt:  now,
	}
	b.order = append(b.order, key)
}

// Flush returns all buffered entries in insertion order and resets the buffer.
// Returns nil if the buffer is empty.
func (b *alertBuffer) Flush() []*bufferedAlert {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.order) == 0 {
		return nil
	}

	result := make([]*bufferedAlert, 0, len(b.order))
	for _, key := range b.order {
		result = append(result, b.byKey[key])
	}

	b.byKey = make(map[string]*bufferedAlert)
	b.order = nil
	return result
}

// runAlertFlushLoop periodically flushes the buffer and delivers the
// accumulated alerts via the AdminNotifier. Runs until ctx is cancelled,
// then does a final flush to deliver any stragglers.
func runAlertFlushLoop(ctx context.Context, buf *alertBuffer, notifier *AdminNotifier) {
	window := readAlertWindow()
	logrus.Infof("[signal] alert flush loop started — window=%s", window)

	ticker := time.NewTicker(window)
	defer ticker.Stop()

	flush := func() {
		entries := buf.Flush()
		if len(entries) == 0 {
			return
		}
		logrus.Infof("[signal] flushing %d unique alert(s)", len(entries))
		for _, entry := range entries {
			subject := entry.subject
			if entry.count > 1 {
				subject.Message = fmt.Sprintf("[x%d in last %s] %s",
					entry.count,
					entry.lastAt.Sub(entry.firstAt).Round(time.Second),
					subject.Message,
				)
			}
			notifier.NotifyAll(ctx, subject)
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush() // final flush on shutdown
			return
		case <-ticker.C:
			flush()
		}
	}
}

// readAlertWindow parses ADMIN_ALERT_WINDOW from the environment. Expects a
// Go duration string (e.g. "2m", "90s", "5m"). Falls back to 2 minutes.
func readAlertWindow() time.Duration {
	v := environment.ReadOptional("ADMIN_ALERT_WINDOW", "2m")
	d, err := time.ParseDuration(v)
	if err != nil {
		logrus.Warnf("invalid ADMIN_ALERT_WINDOW %q — using %s default", v, defaultAlertWindow)
		return defaultAlertWindow
	}
	return d
}
