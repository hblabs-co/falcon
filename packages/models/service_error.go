package models

import (
	"time"

	"hblabs.co/falcon/packages/constants"
)

// ErrorPriority indicates the severity/escalation level of a service error.
type ErrorPriority string

const (
	ErrorPriorityLow      ErrorPriority = "low"
	ErrorPriorityMedium   ErrorPriority = "medium"
	ErrorPriorityHigh     ErrorPriority = "high"
	ErrorPriorityCritical ErrorPriority = "critical"
)

// ServiceError is the unified error document written to the "errors" collection by any service.
// Service-specific fields use omitempty so they are absent from MongoDB when not applicable.
type ServiceError struct {
	// Identity
	ID string `json:"id" bson:"id"`

	// Common fields — always present
	ServiceName string    `json:"service_name" bson:"service_name"`
	ErrorName   string    `json:"error_name"   bson:"error_name"`
	Error       string    `json:"error"        bson:"error"`
	StackTrace  string    `json:"stack_trace"  bson:"stack_trace"`
	OccurredAt  time.Time `json:"occurred_at"  bson:"occurred_at"`

	// Aggregation — only populated for categorical errors (Candidate == nil at record
	// time). OccurredAt keeps its existing meaning (first time recorded); LastSeenAt
	// is updated on every subsequent occurrence and OccurrenceCount tracks how many
	// times the same category has been observed. For per-item errors these stay zero.
	LastSeenAt      time.Time `json:"last_seen_at,omitempty"     bson:"last_seen_at,omitempty"`
	OccurrenceCount int       `json:"occurrence_count,omitempty" bson:"occurrence_count,omitempty"`

	// Retry / escalation
	Priority   ErrorPriority `json:"priority"    bson:"priority"`
	RetryCount int           `json:"retry_count" bson:"retry_count"`
	Resolved   bool          `json:"resolved"    bson:"resolved"`

	// Project context — set by normalizer, scout, dispatch, match-engine
	ProjectID         string `json:"project_id,omitempty"          bson:"project_id,omitempty"`
	Platform          string `json:"platform,omitempty"            bson:"platform,omitempty"`
	PlatformUpdatedAt string `json:"platform_updated_at,omitempty" bson:"platform_updated_at,omitempty"`

	// CV context — set by normalizer when processing CVs
	CVID   string `json:"cv_id,omitempty"   bson:"cv_id,omitempty"`
	UserID string `json:"user_id,omitempty" bson:"user_id,omitempty"`

	// LLM errors — set by normalizer and match-engine
	RawLLMContent string `json:"raw_llm_content,omitempty" bson:"raw_llm_content,omitempty"`

	// Scrape errors — set by scout
	PlatformID string `json:"platform_id,omitempty" bson:"platform_id,omitempty"`
	URL        string `json:"url,omitempty"         bson:"url,omitempty"`
	HTML       string `json:"html,omitempty"        bson:"html,omitempty"`
	// Candidate data for retry — stores the full candidate so the retry worker
	// can reconstruct and reprocess it. Platform-agnostic (any serializable struct).
	Candidate any `json:"candidate,omitempty" bson:"candidate,omitempty"`
}

func (e *ServiceError) IsNormalizerError() bool { return e.ServiceName == constants.ServiceNormalizer }
func (e *ServiceError) IsScoutError() bool      { return e.ServiceName == constants.ServiceScout }
func (e *ServiceError) IsAPIError() bool        { return e.ServiceName == constants.ServiceAPI }
func (e *ServiceError) IsAuthError() bool       { return e.ServiceName == constants.ServiceAuth }
func (e *ServiceError) IsDispatchError() bool   { return e.ServiceName == constants.ServiceDispatch }
func (e *ServiceError) IsMatchEngineError() bool {
	return e.ServiceName == constants.ServiceMatchEngine
}
func (e *ServiceError) IsSignalError() bool  { return e.ServiceName == constants.ServiceSignal }
func (e *ServiceError) IsStorageError() bool { return e.ServiceName == constants.ServiceStorage }
