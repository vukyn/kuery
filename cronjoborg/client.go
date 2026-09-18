package cronjoborg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the public API endpoint.
	DefaultBaseURL = "https://api.cron-job.org"

	// defaultTimeout is applied to the internally created http.Client when the
	// caller supplies neither an HTTPClient nor a Timeout.
	defaultTimeout = 30 * time.Second

	// maxErrorBodyBytes caps how much of a failed response body is kept in an
	// APIError, so a server error page cannot end up whole in a log line.
	maxErrorBodyBytes = 1024

	pathJobs    = "/jobs"
	pathFolders = "/folders"
)

// Config configures a Client. APIKey is required; everything else has a default.
type Config struct {
	// APIKey is the cron-job.org API key, generated in the console under
	// Settings. It is sent as "Authorization: Bearer <APIKey>" on every call.
	// Treat it as a password: load it from the environment or a secret store,
	// never from a repository file.
	APIKey string
	// BaseURL overrides the API endpoint. Empty means DefaultBaseURL; a test
	// points it at an httptest server.
	BaseURL string
	// HTTPClient overrides the underlying client. When nil a client is created
	// with Timeout (or defaultTimeout).
	HTTPClient *http.Client
	// Timeout is applied to the internally created client when HTTPClient is
	// nil. Ignored when HTTPClient is set.
	Timeout time.Duration
}

// Client is a typed cron-job.org API client. It is safe for concurrent use.
//
// The client never retries. The API answers 429 for a rate limit AND for an
// exhausted daily quota, and only the body tells them apart, so a retry policy
// belongs to the caller that knows which of the two it can afford to wait out.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New validates the config and returns a ready Client.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("cronjoborg: APIKey is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(cfg.APIKey),
		httpClient: httpClient,
	}, nil
}

// doJSON sends method to path with an optional JSON body and decodes the
// response into out. body is marshalled when non-nil; out may be nil when the
// response is not needed (several methods answer with an empty object).
// Non-2xx responses become an *APIError.
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cronjoborg: encode request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("cronjoborg: build request: %w", err)
	}
	// The API ignores a request payload whose Content-Type is not exactly
	// application/json, so a missing header here loses the body silently.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cronjoborg: do request: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse(resp, out)
}

// decodeResponse maps a non-2xx status to an *APIError and otherwise decodes
// the body into out.
func decodeResponse(resp *http.Response, out any) error {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cronjoborg: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    truncate(strings.TrimSpace(string(raw)), maxErrorBodyBytes),
			RetryAfter: resp.Header.Get("Retry-After"),
		}
	}

	if out == nil {
		return nil
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("cronjoborg: empty response body")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("cronjoborg: decode response: %w", err)
	}
	return nil
}

// truncate shortens s to at most limit bytes, marking that it was cut. It cuts
// on a byte boundary, so the result is reported as truncated rather than
// presented as the whole body.
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "… (truncated)"
}
