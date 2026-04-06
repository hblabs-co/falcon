package models

import (
	"time"

	"hblabs.co/falcon/common/constants"
)

// ServiceError is the shared document written to the "errors" collection by any service.
type ServiceError struct {
	ServiceName       string    `json:"service_name"        bson:"service_name"`
	ProjectID         string    `json:"project_id"          bson:"project_id"`
	Platform          string    `json:"platform"            bson:"platform"`
	PlatformUpdatedAt string    `json:"platform_updated_at" bson:"platform_updated_at"`
	Error             string    `json:"error"               bson:"error"`
	RawLLMContent     string    `json:"raw_llm_content,omitempty" bson:"raw_llm_content,omitempty"`
	StackTrace        string    `json:"stack_trace"         bson:"stack_trace"`
	OccurredAt        time.Time `json:"occurred_at"         bson:"occurred_at"`
}

func (e *ServiceError) IsNormalizerError() bool  { return e.ServiceName == constants.ServiceNormalizer }
func (e *ServiceError) IsScoutError() bool        { return e.ServiceName == constants.ServiceScout }
func (e *ServiceError) IsAPIError() bool          { return e.ServiceName == constants.ServiceAPI }
func (e *ServiceError) IsAuthError() bool         { return e.ServiceName == constants.ServiceAuth }
func (e *ServiceError) IsDispatchError() bool     { return e.ServiceName == constants.ServiceDispatch }
func (e *ServiceError) IsMatchEngineError() bool  { return e.ServiceName == constants.ServiceMatchEngine }
func (e *ServiceError) IsSignalError() bool       { return e.ServiceName == constants.ServiceSignal }
func (e *ServiceError) IsStorageError() bool      { return e.ServiceName == constants.ServiceStorage }
