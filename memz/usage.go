package memz

import (
	"context"
	"net/http"
)

// Usage mirrors memz's usage Response: the authenticated client's request count
// and accrued cost. LastUpdated is the server-formatted timestamp string.
type Usage struct {
	ID          string  `json:"id"`
	ClientID    string  `json:"client_id"`
	Requests    int     `json:"requests"`
	RequestCost float64 `json:"request_cost"`
	HourlyCost  float64 `json:"hourly_cost"`
	LastUpdated string  `json:"last_updated"`
}

// GetUsage returns the current usage and cost for the authenticated client.
func (c *Client) GetUsage(ctx context.Context) (*Usage, error) {
	var result Usage
	if err := c.doJSON(ctx, http.MethodGet, pathUsages, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
