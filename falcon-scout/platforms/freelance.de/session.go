package freelancede

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"hblabs.co/falcon/common/helpers"
	fljwt "hblabs.co/falcon/common/jwt"
)

// Session holds the active session cookies for freelance.de and
// knows how to refresh them via a form login.
// All exported methods are safe for concurrent use.
type Session struct {
	mu            sync.RWMutex
	freelance     string
	freelanceUser string
	sso           string
	token         *fljwt.DecodedToken
}

var (
	sessionOnce sync.Once
	session     *Session
)

// getSession returns the process-wide singleton, initialised once from the
// environment variables FREELANCE_DE_COOKIE, FREELANCE_DE_USER_COOKIE and FREELANCE_DE_SSO_COOKIE.
func getSession() *Session {
	sessionOnce.Do(func() {
		session = &Session{
			// freelance:     os.Getenv("FREELANCE_DE_COOKIE"),
			// freelanceUser: os.Getenv("FREELANCE_DE_USER_COOKIE"),
			// sso:           os.Getenv("FREELANCE_DE_SSO_COOKIE"),
			freelance:     "",
			freelanceUser: "",
			sso:           "",
		}
	})
	return session
}

// Cookies returns a fresh slice of http.Cookie values for injection into a
// chromedp browser session.
func (s *Session) Cookies() []*http.Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cookiesLocked()
}

// Login performs a form-based login against freelance.de using the credentials
// in FREELANCE_DE_EMAIL and FREELANCE_DE_PASSWORD, then updates the in-memory cookies.
// Only one goroutine executes the login at a time; concurrent callers block
// until the login completes and then share the refreshed cookies.
func (s *Session) Login() error {
	getLogger().Info("starting login")

	// skip login if session cookies are already present
	if s.sso != "" {
		getLogger().Info("session already active, skipping login")
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	values, err := helpers.ReadEnvs("FREELANCE_DE_EMAIL", "FREELANCE_DE_PASSWORD")
	if err != nil {
		return err
	}

	email, password := values[0], values[1]
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("could not create cookie jar: %w", err)
	}

	client := &http.Client{Jar: jar}

	resp, err := client.PostForm(baseUrl+"/login.php", url.Values{
		"username": {email},
		"password": {password},
	})
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("login request failed: %s", resp.Status)
	}

	base, _ := url.Parse(baseUrl)
	for _, c := range jar.Cookies(base) {
		switch c.Name {
		case "freelance":
			s.freelance = c.Value
		case "freelance_user":
			s.freelanceUser = c.Value
		case "sso":
			s.sso = c.Value
		}
	}

	getLogger().Info("session refreshed via login")
	return nil
}

// AccessToken returns a valid JWT access token, fetching a new one if the
// cached token is missing or expired. Safe for concurrent use.
func (s *Session) AccessToken(ctx context.Context) (string, error) {
	s.mu.RLock()
	if s.token.IsValid() {
		t := s.token.Raw
		s.mu.RUnlock()
		return t, nil
	}
	s.mu.RUnlock()

	return s.refreshToken(ctx)
}

// refreshToken fetches a new access token from the API using the session cookies.
func (s *Session) refreshToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check under write lock in case another goroutine already refreshed.
	if s.token.IsValid() {
		return s.token.Raw, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseUrl+tokenEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	for _, c := range s.cookiesLocked() {
		req.AddCookie(c)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrTokenUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %s", resp.Status)
	}

	// The endpoint returns a bare JSON string, e.g. "eyJ0eXAi..."
	var raw string
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	token, err := parseAccessToken(raw)
	if err != nil {
		return "", fmt.Errorf("parse access token: %w", err)
	}

	s.token = token
	getLogger().Infof("access token refreshed — new token expires at %s", token.ExpiresAt.Format(time.RFC3339))
	return token.Raw, nil
}

// cookiesLocked returns the session cookies. Must be called with mu held.
func (s *Session) cookiesLocked() []*http.Cookie {
	exp := time.Now().Add(24 * time.Hour)
	const cookieConsent = `{stamp:%27X3FpYfJsMbPL2VJXTlqRTRyp6Gf0tdwARehcIswJkPUbE/25fYs//w==%27%2Cnecessary:true%2Cpreferences:true%2Cstatistics:true%2Cmarketing:true%2Cmethod:%27explicit%27%2Cver:1%2Cutc:1775140390867%2Cregion:%27de%27}`
	cookies := []*http.Cookie{
		{Name: "freelance_user", Value: s.freelanceUser, Path: "/", HttpOnly: true, Secure: true, Expires: exp},
		{Name: "sso", Value: s.sso, Path: "/", HttpOnly: true, Secure: true, Expires: exp},
		{Name: "CookieConsent", Value: cookieConsent, Path: "/", Secure: true, Expires: exp},
	}
	if s.freelance != "" {
		cookies = append(cookies, &http.Cookie{Name: "freelance", Value: s.freelance, Path: "/", HttpOnly: true, Secure: true, Expires: exp})
	}
	return cookies
}
