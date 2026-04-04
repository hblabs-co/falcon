package freelancede

import (
	gojwt "github.com/golang-jwt/jwt/v5"
	fljwt "hblabs.co/falcon/common/jwt"
)

var jwtParser = fljwt.NewParser()

// freelanceClaims implements jwt.Claims for freelance.de tokens.
// It does not embed RegisteredClaims because freelance.de uses a numeric sub
// field which would fail json.Unmarshal into the string Subject field.
// We only extract the fields we actually need: exp and rights.
type freelanceClaims struct {
	Exp    *gojwt.NumericDate `json:"exp"`
	Rights any                `json:"rights"`
}

func (c *freelanceClaims) GetExpirationTime() (*gojwt.NumericDate, error) { return c.Exp, nil }
func (c *freelanceClaims) GetIssuedAt() (*gojwt.NumericDate, error)       { return nil, nil }
func (c *freelanceClaims) GetNotBefore() (*gojwt.NumericDate, error)      { return nil, nil }
func (c *freelanceClaims) GetIssuer() (string, error)                     { return "", nil }
func (c *freelanceClaims) GetSubject() (string, error)                    { return "", nil }
func (c *freelanceClaims) GetAudience() (gojwt.ClaimStrings, error)       { return nil, nil }

// parseAccessToken decodes and validates a raw JWT from freelance.de.
// It checks both the standard exp claim and the platform-specific rights field.
func parseAccessToken(raw string) (*fljwt.DecodedToken, error) {
	var claims freelanceClaims
	token, err := jwtParser.ParseUnverified(raw, &claims)
	if err != nil {
		return nil, err
	}

	if claims.Rights == nil {
		return nil, ErrNullRights
	}

	return token, nil
}
