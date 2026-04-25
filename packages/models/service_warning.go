package models

import (
	"time"

	"hblabs.co/falcon/packages/constants"
)

// WarningPriority indicates the severity/escalation level of a service warning.
// Mirrors ErrorPriority — same scale, different domain (warnings are non-fatal
// signals that something looks off, while errors mean an operation actually failed).
type WarningPriority string

const (
	WarningPriorityLow      WarningPriority = "low"
	WarningPriorityMedium   WarningPriority = "medium"
	WarningPriorityHigh     WarningPriority = "high"
	WarningPriorityCritical WarningPriority = "critical"
)

// ServiceWarning is the unified warning document written to the "warnings" collection
// by any service. Warnings represent non-fatal anomalies (e.g. expected field missing,
// fallback used, deprecated input format) that don't break processing but may indicate
// markup drift, data quality issues, or future failures worth investigating.
//
// The Candidate field carries the full opaque payload that triggered the warning, so the
// service can persist whatever context the platform considers relevant without coupling
// the model to platform-specific fields.
type ServiceWarning struct {
	// Identity
	ID string `json:"id" bson:"id"`

	// Common fields — always present
	ServiceName string          `json:"service_name" bson:"service_name"`
	WarningName string          `json:"warning_name" bson:"warning_name"`
	Message     string          `json:"message"      bson:"message"`
	Priority    WarningPriority `json:"priority"     bson:"priority"`
	OccurredAt  time.Time       `json:"occurred_at"  bson:"occurred_at"`
	Resolved    bool            `json:"resolved"     bson:"resolved"`

	// Source — used for filtering/grouping
	Platform string `json:"platform,omitempty" bson:"platform,omitempty"`

	// HTML snapshot at the moment the warning was raised. Used for markup-drift
	// warnings so investigators can see exactly what the page looked like without
	// re-fetching (and risking missing the broken state).
	HTML string `json:"html,omitempty" bson:"html,omitempty"`

	// Opaque payload — the full candidate (or whatever struct) the platform attached.
	Candidate any `json:"candidate,omitempty" bson:"candidate,omitempty"`
}

func (w *ServiceWarning) IsNormalizerWarning() bool {
	return w.ServiceName == constants.ServiceNormalizer
}
func (w *ServiceWarning) IsScoutWarning() bool    { return w.ServiceName == constants.ServiceScout }
func (w *ServiceWarning) IsAPIWarning() bool      { return w.ServiceName == constants.ServiceAPI }
func (w *ServiceWarning) IsAuthWarning() bool     { return w.ServiceName == constants.ServiceAuth }
func (w *ServiceWarning) IsDispatchWarning() bool { return w.ServiceName == constants.ServiceDispatch }
func (w *ServiceWarning) IsMatchEngineWarning() bool {
	return w.ServiceName == constants.ServiceMatchEngine
}
func (w *ServiceWarning) IsSignalWarning() bool  { return w.ServiceName == constants.ServiceSignal }
func (w *ServiceWarning) IsStorageWarning() bool { return w.ServiceName == constants.ServiceStorage }
