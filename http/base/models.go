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
	// GetAll    bool   `json:"get_all" query:"get_all"`
}

func (p *Pagination) GetOffset() int {
	return (p.Page - 1) * p.Size
}

func (p *Pagination) GetLimit() int {
	return p.Size
}
