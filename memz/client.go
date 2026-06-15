package memz

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
	// defaultTimeout is applied to the internally created http.Client when the
	// caller supplies neither an HTTPClient nor a Timeout.
	defaultTimeout = 60 * time.Second

	// authHeader carries the raw API key on every call. memz reads the key
	// directly from the Authorization header (no "Bearer" prefix, no
	// X-API-Key), SHA-256-hashes it, and derives the client id from the match.
	authHeader = "Authorization"

	// Route fragments — mirror memz's constants. Every authenticated route
	// mounts under /api/v1 (server.go: apiV1 := app.Group("/api/v1"), then a
	// protected sub-group with an empty prefix carries the api-key middleware).
	pathCaches = "/api/v1/caches"
	pathUsages = "/api/v1/usages"
)

// Config configures a Client. BaseURL and APIKey are required.
type Config struct {
	// BaseURL is the memz origin, e.g. "http://127.0.0.1:8084". Use a
	// 127.0.0.1:<port> address for server-to-server calls, not a *.local
	// hostname (mDNS adds a ~5s resolver stall).
	BaseURL string
	// APIKey is the raw API key, sent verbatim on the Authorization header of
	// every call.
	APIKey string
	// HTTPClient overrides the underlying client. When nil a client is created
	// with Timeout (or defaultTimeout).
	HTTPClient *http.Client
	// Timeout is applied to the internally created client when HTTPClient is
	// nil. Ignored when HTTPClient is set.
	Timeout time.Duration
}

// Client is a typed memz API client. It is safe for concurrent use.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New validates the config and returns a ready Client.
func New(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("memz: BaseURL is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("memz: APIKey is required")
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
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
	}, nil
}

// envelope is the kuery http/base.Response shape the server wraps every JSON
// response in.
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// doJSON sends an HTTP request with the given method and (optional) JSON body to
// path, attaches the API key on the Authorization header, and decodes the
// enveloped data into out. body is marshalled when non-nil; out may be nil when
// no response data is expected. Non-2xx responses are mapped to the typed
// errors.
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("memz: encode request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("memz: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(authHeader, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("memz: do request: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse(resp, out)
}

// decodeResponse reads resp, returning a typed error for non-2xx statuses and
// unwrapping envelope.data into out for success.
func decodeResponse(resp *http.Response, out any) error {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("memz: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classify(resp.StatusCode, raw)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("memz: decode envelope: %w", err)
	}
	// The server may also signal an application error via a non-2xx code inside
	// a 200 envelope; treat any non-success envelope code as an error too.
	if env.Code != 0 && (env.Code < 200 || env.Code >= 300) {
		return &APIError{StatusCode: resp.StatusCode, Code: env.Code, Message: env.Message}
	}
	if out == nil {
		return nil
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return errors.New("memz: empty response data")
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("memz: decode data: %w", err)
	}
	return nil
}

// classify maps a non-2xx HTTP response to an *APIError, pulling the envelope's
// code/message when the body is the standard envelope (some paths, e.g. the
// 401 funnel, return a non-enveloped body, in which case Message falls back to
// the raw body text).
func classify(statusCode int, raw []byte) error {
	apiErr := &APIError{StatusCode: statusCode}
	var env envelope
	if err := json.Unmarshal(raw, &env); err == nil && (env.Code != 0 || env.Message != "") {
		apiErr.Code = env.Code
		apiErr.Message = env.Message
	} else {
		apiErr.Message = strings.TrimSpace(string(raw))
	}
	return apiErr
}
