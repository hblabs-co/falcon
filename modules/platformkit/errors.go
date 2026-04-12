package platformkit

import (
	"context"
	"errors"
	"fmt"
)

// ============================================================================
// Error name constants
// ============================================================================
//
// Stable identifiers used as ServiceError.ErrorName when the service persists
// runner-reported errors. They are shared across all platforms because the same
// failure modes (gone, server error, unauthorized) apply to any HTTP scraper.

const (
	// ErrNameScrapeInspectFailed — generic catch-all for inspect failures.
	ErrNameScrapeInspectFailed = "scrape_inspect_failed"
	// ErrNameScrapeServerError — 5xx response from the platform; should be retried.
	ErrNameScrapeServerError = "scrape_server_error"
	// ErrNameScrapeGone — 410 Gone; the project was permanently removed; do not retry.
	ErrNameScrapeGone = "scrape_gone"
	// ErrNameScrapeUnauthorized — 401/403 from the platform; usually requires re-auth.
	ErrNameScrapeUnauthorized = "scrape_unauthorized"
	// ErrNameScrapeListingEmpty — listing page returned 0 candidates; usually means
	// markup drift on the source platform. Categorical (system-level) error.
	ErrNameScrapeListingEmpty = "scrape_listing_empty"
)

// ============================================================================
// Sentinel errors
// ============================================================================

var (
	// ErrGone is returned when the platform responds with 410 — the resource was
	// permanently removed. Callers should skip the candidate and not retry.
	ErrGone = errors.New("project gone (410)")

	// ErrUnauthorized is returned when the platform responds with 401 or 403 —
	// the request was rejected due to missing/expired credentials. For platforms
	// that require login this signals that the session must be refreshed.
	ErrUnauthorized = errors.New("unauthorized (401/403)")
)

// ErrServerError represents a 5xx HTTP response from the platform. These are
// transient and should be retried with backoff. The status code and URL are
// preserved for diagnostics.
type ErrServerError struct {
	StatusCode int
	URL        string
}

func (e *ErrServerError) Error() string {
	return fmt.Sprintf("server error %d for %s", e.StatusCode, e.URL)
}

// IsServerError reports whether err is (or wraps) an ErrServerError.
func IsServerError(err error) bool {
	var se *ErrServerError
	return errors.As(err, &se)
}

// ErrEmptyListing represents a categorical failure where a listing page that
// is expected to contain candidates returned zero parseable items.
//
// CardsSeen tracks how many raw card elements the outer selector matched
// BEFORE parseJobCard tried to extract data from each one. This lets the
// caller distinguish two very different situations:
//
//   - CardsSeen > 0, 0 parsed: definite internal markup drift — the card
//     containers exist but their inner structure changed (href missing,
//     URL format different, etc.). Strongest signal.
//
//   - CardsSeen == 0: ambiguous — the outer selector itself may have broken,
//     or the platform is genuinely empty, or a challenge/block page was served.
//     Check the HTML size for a secondary clue (tiny = blocked, large = selector broken).
type ErrEmptyListing struct {
	Page      int
	HTML      string
	CardsSeen int
}

func (e *ErrEmptyListing) Error() string {
	if e.CardsSeen > 0 {
		return fmt.Sprintf("listing page %d found %d job cards but none could be parsed — internal markup changed", e.Page, e.CardsSeen)
	}
	return fmt.Sprintf("listing page %d returned no job cards — possible outer selector drift or blocking", e.Page)
}

// IsEmptyListing reports whether err is (or wraps) an ErrEmptyListing.
func IsEmptyListing(err error) bool {
	var el *ErrEmptyListing
	return errors.As(err, &el)
}

// AsEmptyListing extracts an *ErrEmptyListing from err so callers can read the
// HTML snapshot. Returns nil if err is not an empty-listing error.
func AsEmptyListing(err error) *ErrEmptyListing {
	var el *ErrEmptyListing
	if errors.As(err, &el) {
		return el
	}
	return nil
}

// IsGone reports whether err is (or wraps) ErrGone.
func IsGone(err error) bool { return errors.Is(err, ErrGone) }

// IsUnauthorized reports whether err is (or wraps) ErrUnauthorized.
func IsUnauthorized(err error) bool { return errors.Is(err, ErrUnauthorized) }

// ============================================================================
// HTTP status mapping
// ============================================================================

// ErrorFromStatus maps an HTTP status code to a platformkit sentinel/typed error.
// Returns nil for status codes that aren't an error (1xx/2xx/3xx) and a generic
// wrapped error for codes that don't fit a known category.
//
// Use this from a colly OnError callback:
//
//	c.OnError(func(r *colly.Response, err error) {
//	    scrapeErr = platformkit.ErrorFromStatus(r.StatusCode, r.Request.URL.String(), err)
//	})
func ErrorFromStatus(statusCode int, url string, cause error) error {
	switch {
	case statusCode == 410:
		return ErrGone
	case statusCode == 401, statusCode == 403:
		return ErrUnauthorized
	case statusCode >= 500:
		return &ErrServerError{StatusCode: statusCode, URL: url}
	case statusCode >= 400:
		return fmt.Errorf("HTTP %d: %w", statusCode, cause)
	}
	return nil
}

// ============================================================================
// Classification — for callers reporting errors via ErrFn
// ============================================================================

// ClassifyError inspects err and returns the appropriate ErrorName, priority,
// and any CallOptions that should accompany the recording (notably Categorical
// for system-wide failures). Centralizes the mapping so every platform reports
// the same shape for the same kind of failure.
//
// Returned priority strings ("low" | "medium" | "high" | "critical") match the
// values used by ServiceError.Priority and the WarnFn priority field.
//
// Categorical opts are returned for failures that affect a shared resource:
//   - ErrEmptyListing → markup of the listing page is broken
//   - ErrUnauthorized → session/auth dead at the connection layer
//
// Per-item failures (5xx, generic inspect errors) get no opts; the runner can
// still pass extra opts on its own if it knows something the classifier doesn't.
func ClassifyError(err error) (name, priority string, opts []CallOption) {
	switch {
	case IsGone(err):
		return ErrNameScrapeGone, "low", nil
	case IsUnauthorized(err):
		return ErrNameScrapeUnauthorized, "high", []CallOption{Categorical()}
	case IsServerError(err):
		return ErrNameScrapeServerError, "medium", nil
	case IsEmptyListing(err):
		return ErrNameScrapeListingEmpty, "critical", []CallOption{Categorical()}
	default:
		return ErrNameScrapeInspectFailed, "high", nil
	}
}

// ============================================================================
// Callback type
// ============================================================================

// ErrFn is a callback injected by the service to record an error.
// The runner calls it after classifying the failure with ClassifyError so the
// same error category produces the same persisted shape across platforms.
//
// Parameters:
//   - name:      stable identifier (e.g. "scrape_inspect_failed") — typically from ClassifyError
//   - message:   human-readable description (typically err.Error())
//   - priority:  "low" | "medium" | "high" | "critical" — typically from ClassifyError
//   - html:      HTML snapshot at the moment of failure (pass "" if not available)
//   - candidate: opaque payload (the candidate that failed, for retry reconstruction)
//   - opts:      functional options (e.g. Categorical()) that change how the
//                service persists the record. Spread the slice returned by
//                ClassifyError to inherit its category-aware defaults.
type ErrFn func(ctx context.Context, name, message, priority, html string, candidate any, opts ...CallOption) error
