package reminders

import (
	"testing"
	"time"

	"hblabs.co/falcon/packages/models"
)

// TestNextReminderAction walks the cadence decision tree across
// every meaningful (timeSinceJoin, lastAt-gap, stopped) combination.
// Pure function under test — no Mongo / NATS / APNs needed.
//
// Reference cadence (see runtime.go):
//
//	T < 1d                → SKIP (grace)
//	1d ≤ T < 7d           → daily, gap 24h
//	7d ≤ T < 30d          → weekly, gap 7d
//	T ≥ 30d               → STOP
func TestNextReminderAction(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		joinedAgo   time.Duration
		lastAgo     time.Duration // 0 = never sent (zero LastAt)
		count       int
		stopped     bool
		want        decision
		description string
	}{
		// ── Stopped short-circuits everything ─────────────────────
		{
			name: "stopped short-circuits even mid-cycle",
			joinedAgo: 3 * 24 * time.Hour, lastAgo: 25 * time.Hour, stopped: true,
			want: decisionSkip,
		},

		// ── Grace period ──────────────────────────────────────────
		{
			name:      "fresh signup, 1 hour in → skip",
			joinedAgo: 1 * time.Hour,
			want:      decisionSkip, description: "grace period (T<1d)",
		},
		{
			name:      "23 hours in → still grace",
			joinedAgo: 23 * time.Hour,
			want:      decisionSkip, description: "grace period (T<1d)",
		},
		{
			name:      "exactly 24h, never sent → first send",
			joinedAgo: 24 * time.Hour,
			want:      decisionSend, description: "first send right after grace",
		},

		// ── Daily phase (week 1) ──────────────────────────────────
		{
			name:      "day 2, never sent → send",
			joinedAgo: 2 * 24 * time.Hour,
			want:      decisionSend,
		},
		{
			name:      "day 2, sent 12h ago → skip (under 24h gap)",
			joinedAgo: 2 * 24 * time.Hour, lastAgo: 12 * time.Hour, count: 1,
			want: decisionSkip,
		},
		{
			name:      "day 2, sent 23h ago → skip (still under gap)",
			joinedAgo: 2 * 24 * time.Hour, lastAgo: 23 * time.Hour, count: 1,
			want: decisionSkip,
		},
		{
			name:      "day 2, sent 25h ago → send (gap satisfied)",
			joinedAgo: 2 * 24 * time.Hour, lastAgo: 25 * time.Hour, count: 1,
			want: decisionSend,
		},
		{
			name:      "day 6, sent 24h+ ago → send (still daily phase)",
			joinedAgo: 6 * 24 * time.Hour, lastAgo: 25 * time.Hour, count: 5,
			want: decisionSend,
		},

		// ── Weekly phase (days 7–29) ──────────────────────────────
		{
			name:      "day 7, sent 24h ago → skip (now weekly gap)",
			joinedAgo: 7 * 24 * time.Hour, lastAgo: 24 * time.Hour, count: 6,
			want: decisionSkip,
		},
		{
			name:      "day 7, sent 6 days ago → skip (under 7d gap)",
			joinedAgo: 7 * 24 * time.Hour, lastAgo: 6 * 24 * time.Hour, count: 6,
			want: decisionSkip,
		},
		{
			name:      "day 14, sent 7d+ ago → send",
			joinedAgo: 14 * 24 * time.Hour, lastAgo: 7*24*time.Hour + time.Hour, count: 7,
			want: decisionSend,
		},
		{
			name:      "day 21, sent 8d ago → send",
			joinedAgo: 21 * 24 * time.Hour, lastAgo: 8 * 24 * time.Hour, count: 8,
			want: decisionSend,
		},
		{
			name:      "day 28, sent 7d ago exactly → send",
			joinedAgo: 28 * 24 * time.Hour, lastAgo: 7 * 24 * time.Hour, count: 9,
			want: decisionSend,
		},

		// ── Terminal window ───────────────────────────────────────
		{
			name:      "day 30, never sent → stop",
			joinedAgo: 30 * 24 * time.Hour,
			want:      decisionStop,
		},
		{
			name:      "day 31, sent 1d ago → stop (terminal beats cadence)",
			joinedAgo: 31 * 24 * time.Hour, lastAgo: 24 * time.Hour, count: 10,
			want: decisionStop,
		},
		{
			name:      "day 90, never sent → stop",
			joinedAgo: 90 * 24 * time.Hour,
			want:      decisionStop,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rem := models.UserReminder{
				Kind:    models.UserReminderKindCVUpload,
				Count:   tc.count,
				Stopped: tc.stopped,
			}
			if tc.lastAgo > 0 {
				rem.LastAt = now.Add(-tc.lastAgo)
				if rem.FirstAt.IsZero() {
					rem.FirstAt = rem.LastAt
				}
			}
			joinedAt := now.Add(-tc.joinedAgo)

			got := nextReminderAction(now, joinedAt, rem)
			if got != tc.want {
				t.Errorf("nextReminderAction(joinedAgo=%s, lastAgo=%s, stopped=%v) = %v, want %v — %s",
					tc.joinedAgo, tc.lastAgo, tc.stopped, got, tc.want, tc.description)
			}
		})
	}
}

// TestWithinSendingWindow confirms the Berlin sending gate. Builds
// the test moments in UTC and lets withinSendingWindow translate to
// Berlin internally — same code path as production.
func TestWithinSendingWindow(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("Europe/Berlin not available in test env (tzdata missing): %v", err)
	}

	tests := []struct {
		name string
		when time.Time
		want bool
	}{
		{name: "07:59 Berlin → outside (before window)", when: time.Date(2026, 5, 1, 7, 59, 0, 0, berlin), want: false},
		{name: "08:00 Berlin → inside (start)", when: time.Date(2026, 5, 1, 8, 0, 0, 0, berlin), want: true},
		{name: "12:00 Berlin → inside", when: time.Date(2026, 5, 1, 12, 0, 0, 0, berlin), want: true},
		{name: "19:59 Berlin → inside (last minute)", when: time.Date(2026, 5, 1, 19, 59, 0, 0, berlin), want: true},
		{name: "20:00 Berlin → outside (strict <)", when: time.Date(2026, 5, 1, 20, 0, 0, 0, berlin), want: false},
		{name: "23:30 Berlin → outside", when: time.Date(2026, 5, 1, 23, 30, 0, 0, berlin), want: false},
		{name: "03:00 Berlin → outside (early morning)", when: time.Date(2026, 5, 1, 3, 0, 0, 0, berlin), want: false},
		// DST sanity — January 12:00 Berlin is CET (UTC+1); July 12:00 Berlin
		// is CEST (UTC+2). Both should test as inside the window because
		// withinSendingWindow evaluates against berlin.Hour() not UTC.
		{name: "winter noon Berlin (CET) → inside", when: time.Date(2026, 1, 15, 12, 0, 0, 0, berlin), want: true},
		{name: "summer noon Berlin (CEST) → inside", when: time.Date(2026, 7, 15, 12, 0, 0, 0, berlin), want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinSendingWindow(tc.when); got != tc.want {
				t.Errorf("withinSendingWindow(%s) = %v, want %v", tc.when, got, tc.want)
			}
		})
	}
}
