package medioa

import (
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

	// apiKeyHeader carries the public-API key on every write call.
	apiKeyHeader = "X-API-Key"

	// Route fragments — mirror medioa2's constants. The public groups mount
	// under /api/v1.
	pathUpload       = "/api/v1/public/storage/upload"
	pathUploadStage  = "/api/v1/public/storage/upload/stage"
	pathUploadCommit = "/api/v1/public/storage/upload/commit"
	pathPublicObject = "/api/v1/public/objects/"
)

// Config configures a Client. BaseURL and APIKey are required.
type Config struct {
	// BaseURL is the medioa2 origin, e.g. "http://127.0.0.1:8082". Use a
	// 127.0.0.1:<port> address for server-to-server calls, not a *.local
	// hostname (mDNS adds a ~5s resolver stall).
	BaseURL string
	// APIKey is the public-API key (an "mk_..." value), sent as X-API-Key on
	// every write call.
	APIKey string
	// HTTPClient overrides the underlying client. When nil a client is created
	// with Timeout (or defaultTimeout).
	HTTPClient *http.Client
	// Timeout is applied to the internally created client when HTTPClient is
	// nil. Ignored when HTTPClient is set.
	Timeout time.Duration
}

// Client is a typed medioa2 public-API client. It is safe for concurrent use.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New validates the config and returns a ready Client.
func New(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("medioa: BaseURL is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("medioa: APIKey is required")
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

// PublicURL builds the anonymous, token-gated read URL for an uploaded object:
// {baseURL}/api/v1/public/objects/{token}. The server responds with a 302 to a
// short-lived presigned URL. It is a pure builder — no network call — so it can
// be used without a Client (e.g. when only persisting the URL).
func PublicURL(baseURL, token string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + pathPublicObject + token
}

// envelope is the kuery http/base.Response shape the server wraps every JSON
// response in.
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// doMultipart sends a POST with the given multipart body to path, attaches the
// API key, and decodes the enveloped data into out. Non-2xx responses are
// mapped to the typed errors.
func (c *Client) doMultipart(ctx context.Context, path, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("medioa: build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set(apiKeyHeader, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("medioa: do request: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse(resp, out)
}

// decodeResponse reads resp, returning a typed error for non-2xx statuses and
// unwrapping envelope.data into out for success.
func decodeResponse(resp *http.Response, out any) error {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("medioa: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classify(resp.StatusCode, raw)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("medioa: decode envelope: %w", err)
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
		return errors.New("medioa: empty response data")
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("medioa: decode data: %w", err)
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
