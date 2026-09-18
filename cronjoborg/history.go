package cronjoborg

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// HistoryItem is one execution of a job.
//
// Headers and Body are populated only by GetHistoryItem, and only for a job
// created with SaveResponses set — the list endpoint always reports them as
// nil, which is a missing detail rather than an empty response.
type HistoryItem struct {
	JobID         int64            `json:"jobId"`
	Identifier    string           `json:"identifier"`
	Date          int64            `json:"date"`        // unix seconds, actual execution
	DatePlanned   int64            `json:"datePlanned"` // unix seconds, ideal execution
	Jitter        int              `json:"jitter"`      // milliseconds between planned and actual
	URL           string           `json:"url"`         // the URL at the time of execution
	Duration      int              `json:"duration"`    // milliseconds
	Status        JobStatus        `json:"status"`
	StatusText    string           `json:"statusText"`
	HTTPStatus    int              `json:"httpStatus"`
	Headers       *string          `json:"headers"` // nil when unavailable
	Body          *string          `json:"body"`    // nil when unavailable
	Stats         HistoryItemStats `json:"stats"`
	SslCertExpiry int64            `json:"sslCertExpiry"` // unix seconds, HTTPS executions only
}

// HistoryItemStats is the timing breakdown of one execution, in MICROSECONDS —
// unlike HistoryItem.Duration and Jitter, which are milliseconds.
type HistoryItemStats struct {
	NameLookup    int `json:"nameLookup"`
	Connect       int `json:"connect"`
	AppConnect    int `json:"appConnect"` // 0 when the request did not use TLS
	PreTransfer   int `json:"preTransfer"`
	StartTransfer int `json:"startTransfer"`
	Total         int `json:"total"`
}

// History is the execution history of a job plus the scheduler's next
// predictions.
type History struct {
	// Items are the most recent executions, newest first.
	Items []HistoryItem
	// Predictions are Unix timestamps (seconds) of the next executions the
	// scheduler predicts, up to 3.
	Predictions []int64
}

// historyResponse is the raw shape of GET /jobs/{jobId}/history.
type historyResponse struct {
	History     []HistoryItem `json:"history"`
	Predictions []int64       `json:"predictions"`
}

// historyItemResponse is the raw shape of GET /jobs/{jobId}/history/{identifier}.
type historyItemResponse struct {
	JobHistoryDetails HistoryItem `json:"jobHistoryDetails"`
}

// GetHistory returns the last execution history items of a job and the
// predicted next executions. Headers and Body are not populated here — use
// GetHistoryItem for one item's response detail.
//
// Rate limit: 5 requests per second.
func (c *Client) GetHistory(ctx context.Context, jobID int64) (*History, error) {
	if jobID <= 0 {
		return nil, errors.New("cronjoborg: jobID is required")
	}

	var response historyResponse
	if err := c.doJSON(ctx, http.MethodGet, jobPath(jobID)+"/history", nil, &response); err != nil {
		return nil, err
	}
	return &History{Items: response.History, Predictions: response.Predictions}, nil
}

// GetHistoryItem returns one execution in full, including the response headers
// and body when the job saves responses.
//
// Rate limit: 5 requests per second.
func (c *Client) GetHistoryItem(ctx context.Context, jobID int64, identifier string) (*HistoryItem, error) {
	if jobID <= 0 {
		return nil, errors.New("cronjoborg: jobID is required")
	}
	if identifier == "" {
		return nil, errors.New("cronjoborg: identifier is required")
	}

	var response historyItemResponse
	path := jobPath(jobID) + "/history/" + url.PathEscape(identifier)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return &response.JobHistoryDetails, nil
}
