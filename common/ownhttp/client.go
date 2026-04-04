package ownhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Request holds the options for an HTTP call.
// Mirrors the browser fetch() init object: url goes in the call, everything else here.
type Request struct {
	Headers map[string]string // merged with Client default headers; call-level headers win
	Cookies []*http.Cookie    // added to the request as-is
	Body    any               // marshaled to JSON; Content-Type set automatically
	Result  any               // response body decoded into this if non-nil
}

// Client sends requests to BaseURL with default Headers merged into every call.
type Client struct {
	BaseURL string
	Headers map[string]string
	inner   *http.Client
}

// New returns a Client for baseURL. Pass default headers (e.g. Authorization) or nil.
func New(baseURL string, headers map[string]string) *Client {
	return &Client{BaseURL: baseURL, Headers: headers, inner: http.DefaultClient}
}

// Post sends a POST to BaseURL+path.
func (c *Client) Post(ctx context.Context, path string, req Request) error {
	return c.DoRequest(ctx, http.MethodPost, path, req)
}

// Put sends a PUT to BaseURL+path.
func (c *Client) Put(ctx context.Context, path string, req Request) error {
	return c.DoRequest(ctx, http.MethodPut, path, req)
}

// Status sends a GET to BaseURL+path and returns only the HTTP status code.
// Does not error on non-2xx — use this when the status itself is the signal.
func (c *Client) Status(ctx context.Context, path string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return 0, err
	}
	c.applyHeaders(req, nil)
	resp, err := c.inner.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// DoRequest executes an arbitrary HTTP method. Post and Put are convenience wrappers around it.
func (c *Client) DoRequest(ctx context.Context, method, path string, req Request) error {
	var body io.Reader
	if req.Body != nil {
		data, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(data)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	c.applyHeaders(httpReq, req.Headers)
	for _, cookie := range req.Cookies {
		httpReq.AddCookie(cookie)
	}

	resp, err := c.inner.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if req.Result != nil {
		return json.NewDecoder(resp.Body).Decode(req.Result)
	}
	return nil
}

// applyHeaders sets default client headers first, then call-level headers (which win on conflict).
func (c *Client) applyHeaders(req *http.Request, callHeaders map[string]string) {
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range callHeaders {
		req.Header.Set(k, v)
	}
}
