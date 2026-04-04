package freelancede

import "errors"

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
	// Callers can assert it with errors.Is and trigger a login + retry.
	ErrSessionExpired = errors.New("session expired or unauthenticated")
)
