package platformkit

import "context"

// Warning name constants — stable identifiers reused across platforms.
// Each platform that hits the same anomaly should use the same warning name
// so dashboards/alerts can aggregate occurrences from multiple sources.
const (
	// WarnReferenceIDNotFound — the platform's external reference ID could not be
	// extracted from the detail page. Usually means markup drift.
	WarnReferenceIDNotFound = "reference_id_not_found"
)

// WarnFn is a callback injected by the service to record a non-fatal anomaly.
// The runner calls it when it detects something worth persisting for later analysis
// (markup drift, missing optional fields, fallback paths taken) but that doesn't break
// processing of the current item.
//
// Parameters:
//   - name:      stable identifier for the warning (e.g. "reference_id_not_found")
//   - message:   human-readable description of what happened
//   - priority:  "low" | "medium" | "high" | "critical"
//   - html:      HTML snapshot for markup-drift warnings (pass "" if not relevant)
//   - candidate: opaque payload (the candidate that triggered the warning, or any other context)
type WarnFn func(ctx context.Context, name, message, priority, html string, candidate any) error
