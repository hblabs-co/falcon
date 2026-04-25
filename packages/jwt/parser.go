package jwt

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Parser wraps golang-jwt's parser and exposes convenience methods for
// working with JWTs. Each platform inspector creates its own instance and
// passes its own claims struct to ParseUnverified.
type Parser struct {
	inner *jwt.Parser
}

// NewParser creates a Parser with default options. concurrent safe
func NewParser() *Parser {
	return &Parser{inner: jwt.NewParser()}
}

// ParseUnverified decodes raw into claims without verifying the signature.
// It extracts and returns the expiration time from the standard exp claim.
// claims must implement jwt.Claims — typically a struct that embeds
// jwt.RegisteredClaims plus any platform-specific fields.
//
// Signature verification is intentionally skipped: we consume third-party
// tokens and do not have access to the signing key.
func (p *Parser) ParseUnverified(raw string, claims jwt.Claims) (*DecodedToken, error) {
	_, _, err := p.inner.ParseUnverified(raw, claims)
	if err != nil {
		return nil, fmt.Errorf("parse JWT: %w", err)
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return nil, fmt.Errorf("JWT missing or invalid exp claim")
	}

	return &DecodedToken{
		Raw:       raw,
		ExpiresAt: exp.Time,
	}, nil
}
