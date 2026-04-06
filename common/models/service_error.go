package models

import (
	"time"

	"hblabs.co/falcon/common/constants"
)

// ServiceError is the unified error document written to the "errors" collection by any service.
// Service-specific fields use omitempty so they are absent from MongoDB when not applicable.
type ServiceError struct {
	// Common fields — always present
	ServiceName string    `json:"service_name" bson:"service_name"`
	ErrorName   string    `json:"error_name"   bson:"error_name"`
	Error       string    `json:"error"        bson:"error"`
	StackTrace  string    `json:"stack_trace"  bson:"stack_trace"`
	OccurredAt  time.Time `json:"occurred_at"  bson:"occurred_at"`

	// Project context — set by normalizer, scout, dispatch, match-engine
	ProjectID         string `json:"project_id,omitempty"          bson:"project_id,omitempty"`
	Platform          string `json:"platform,omitempty"            bson:"platform,omitempty"`
	PlatformUpdatedAt string `json:"platform_updated_at,omitempty" bson:"platform_updated_at,omitempty"`

	// LLM errors — set by normalizer and match-engine
	RawLLMContent string `json:"raw_llm_content,omitempty" bson:"raw_llm_content,omitempty"`

	// Scrape errors — set by scout
	PlatformID string `json:"platform_id,omitempty" bson:"platform_id,omitempty"`
	URL        string `json:"url,omitempty"         bson:"url,omitempty"`
	HTML       string `json:"html,omitempty"        bson:"html,omitempty"`
}

func (e *ServiceError) IsNormalizerError() bool  { return e.ServiceName == constants.ServiceNormalizer }
func (e *ServiceError) IsScoutError() bool        { return e.ServiceName == constants.ServiceScout }
func (e *ServiceError) IsAPIError() bool          { return e.ServiceName == constants.ServiceAPI }
func (e *ServiceError) IsAuthError() bool         { return e.ServiceName == constants.ServiceAuth }
func (e *ServiceError) IsDispatchError() bool     { return e.ServiceName == constants.ServiceDispatch }
func (e *ServiceError) IsMatchEngineError() bool  { return e.ServiceName == constants.ServiceMatchEngine }
func (e *ServiceError) IsSignalError() bool       { return e.ServiceName == constants.ServiceSignal }
func (e *ServiceError) IsStorageError() bool      { return e.ServiceName == constants.ServiceStorage }
