package base

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	// ErrorCode is a STABLE, machine-readable name for the failure, such as
	// "PLACE_NOT_FOUND". Message is prose for a developer and may be reworded
	// at any time; ErrorCode is the part a client is allowed to branch on —
	// to pick a translated sentence, to decide whether to retry, or to focus
	// the field at fault.
	//
	// ⚠️ omitempty is load-bearing. Every service already on this envelope
	// keeps byte-identical JSON until it starts setting a code, so adopting
	// this is per-endpoint and never a breaking change.
	//
	// It is deliberately absent from 5xx responses: the code would name the
	// subsystem that failed, which is exactly the internal detail the generic
	// 5xx body exists to withhold.
	ErrorCode string `json:"error_code,omitempty"`
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
