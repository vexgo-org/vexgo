package api

// Pagination describes a paged result.
type Pagination struct {
	Total      int64 `json:"total" example:"42"`
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"10"`
	TotalPages int64 `json:"totalPages" example:"5"`
}

func ToPagination(total int64, page, limit int) Pagination {
	totalPages := (total + int64(limit) - 1) / int64(limit)
	if totalPages == 0 {
		totalPages = 1
	}

	return Pagination{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}
}
