package jwt

import "time"

// expiryBuffer is subtracted from the token's expiry when checking validity.
// A token with less than 1 minute remaining is treated as expired to avoid
// using a token that may expire mid-request.
const expiryBuffer = time.Minute

// DecodedToken holds a raw JWT string alongside its expiration time so the
// payload is only parsed once and callers can check validity cheaply.
type DecodedToken struct {
	Raw       string
	ExpiresAt time.Time
}

// IsValid reports whether the token is non-nil and has more than expiryBuffer
// time remaining before expiration.
func (t *DecodedToken) IsValid() bool {
	return t != nil && time.Now().Add(expiryBuffer).Before(t.ExpiresAt)
}
