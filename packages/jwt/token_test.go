package jwt

import (
	"testing"
	"time"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		name  string
		token *DecodedToken
		want  bool
	}{
		{
			name:  "nil token",
			token: nil,
			want:  false,
		},
		{
			name:  "expired token",
			token: &DecodedToken{ExpiresAt: time.Now().Add(-time.Hour)},
			want:  false,
		},
		{
			name:  "expires exactly now",
			token: &DecodedToken{ExpiresAt: time.Now()},
			want:  false,
		},
		{
			name:  "expires within buffer (30s remaining)",
			token: &DecodedToken{ExpiresAt: time.Now().Add(30 * time.Second)},
			want:  false,
		},
		{
			name:  "expires at buffer boundary (exactly 1 min remaining)",
			token: &DecodedToken{ExpiresAt: time.Now().Add(expiryBuffer)},
			want:  false,
		},
		{
			name:  "expires just past buffer (61s remaining)",
			token: &DecodedToken{ExpiresAt: time.Now().Add(expiryBuffer + time.Second)},
			want:  true,
		},
		{
			name:  "valid token with plenty of time",
			token: &DecodedToken{ExpiresAt: time.Now().Add(time.Hour)},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
