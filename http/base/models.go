package base

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Pagination struct {
	Page      int    `json:"page" query:"page"`
	Size      int    `json:"size" query:"size"`
	SortBy    string `json:"sort_by" query:"sort_by"`
	SortOrder string `json:"sort_order" query:"sort_order"`
	CountOnly bool   `json:"count_only" query:"count_only"`
	// Count, when set by the client (query `count=true`), asks the list endpoint
	// to also compute the total record count (an extra COUNT query). When false,
	// repositories skip the count to avoid the cost and leave Total at 0.
	Count bool `json:"count" query:"count"`
	// Total is the total number of matching records across all pages, set by the
	// repository/usecase on a list response (only when Count is requested) so
	// clients can compute page counts. Response-only — ignored as a query param.
	Total int `json:"total" query:"-"`
	// GetAll    bool   `json:"get_all" query:"get_all"`
}

func (p *Pagination) GetOffset() int {
	return (p.Page - 1) * p.Size
}

func (p *Pagination) GetLimit() int {
	return p.Size
}
