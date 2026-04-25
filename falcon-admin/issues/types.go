// Package issues exposes the unified errors+warnings admin
// surface — one list across both collections, with filters,
// resolve actions, and (later) live updates via NATS. Mounted
// under the admin bearer-protected route group via Mount().
package issues

import "time"

// Type discriminator used in the UI list and on resolve routes.
const (
	TypeError   = "error"
	TypeWarning = "warning"
)

// issueView is the unified row the UI consumes. Errors and
// warnings have different shapes server-side (different name
// fields, different context columns), so we project both into
// this view-only struct. Anything optional rides through with
// omitempty so the JSON stays compact for warnings that don't
// carry, e.g., a project_id.
type issueView struct {
	Type       string    `json:"type"` // "error" | "warning"
	ID         string    `json:"id"`
	Service    string    `json:"service,omitempty"`
	Name       string    `json:"name,omitempty"`     // error_name / warning_name
	Message    string    `json:"message,omitempty"`  // error / message
	Priority   string    `json:"priority,omitempty"`
	Resolved   bool      `json:"resolved"`
	OccurredAt time.Time `json:"occurred_at"`

	// Aggregation (errors only — warnings don't aggregate today).
	LastSeenAt      time.Time `json:"last_seen_at,omitempty"`
	OccurrenceCount int       `json:"occurrence_count,omitempty"`

	// Source / context — populated when present on the underlying doc.
	Platform   string `json:"platform,omitempty"`
	PlatformID string `json:"platform_id,omitempty"`
	URL        string `json:"url,omitempty"`
	ProjectID  string `json:"project_id,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	CVID       string `json:"cv_id,omitempty"`
	RetryCount int    `json:"retry_count,omitempty"`

	// Heavy fields — only included on the per-issue detail
	// endpoint, not on list rows, so the list payload stays light.
	StackTrace    string `json:"stack_trace,omitempty"`
	HTML          string `json:"html,omitempty"`
	RawLLMContent string `json:"raw_llm_content,omitempty"`
	Candidate     any    `json:"candidate,omitempty"`
}
