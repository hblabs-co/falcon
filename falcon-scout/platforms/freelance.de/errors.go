package freelancede

import (
	"errors"
	"fmt"
)

var (
	// ErrNullRights is returned when the JWT rights field is null,
	// indicating an invalid or unauthenticated session.
	ErrNullRights = errors.New("JWT rights field is null — token is invalid or session expired")

	// ErrTokenUnauthorized is returned when the access token request returns 401,
	// indicating the session cookies are expired and a re-login is required.
	ErrTokenUnauthorized = errors.New("token request returned 401 — session cookies may be expired, re-login required")

	// ErrCandidatesUnauthorized is returned when the project candidates request
	// returns 401, indicating the access token was rejected.
	ErrCandidatesUnauthorized = errors.New("project candidates request returned 401 — token rejected")

	// ErrSessionExpired is returned by an inspector when the current session
	// cookies are no longer valid (e.g. the target site demands re-authentication).
	ErrSessionExpired = errors.New("session expired or unauthenticated")

	// ErrGone is returned when the platform returns 410 — the project was permanently removed.
	ErrGone = errors.New("project gone (410)")
)

// ErrServerError represents a 5xx HTTP response from the platform.
// These are retried with a longer interval since the project page
// may take up to an hour to become available.
type ErrServerError struct {
	StatusCode int
	URL        string
}

func (e *ErrServerError) Error() string {
	return fmt.Sprintf("server error %d for %s", e.StatusCode, e.URL)
}

// IsServerError checks if an error is a 5xx server error.
func IsServerError(err error) bool {
	var se *ErrServerError
	return errors.As(err, &se)
}
