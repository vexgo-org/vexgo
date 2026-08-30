package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultPaginationLimit is used when the limit parameter is absent or invalid.
	DefaultPaginationLimit = 10
	// MaxPaginationLimit caps the page size so a client cannot request an
	// unbounded dump of a table.
	MaxPaginationLimit = 100
)

// ParsePagination reads the "page" and "limit" query parameters and clamps
// them to safe ranges: page < 1 becomes 1, limit < 1 becomes defaultLimit and
// limit above 100 is capped. Handlers must not use raw query values in
// Offset/Limit calls — a negative limit makes GORM drop the LIMIT clause
// entirely, and a zero limit panics on the subsequent totalPages division.
func ParsePagination(c *gin.Context, defaultLimit int) (page, limit int) {
	if defaultLimit < 1 {
		defaultLimit = DefaultPaginationLimit
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit < 1 {
		limit = defaultLimit
	}
	if limit > MaxPaginationLimit {
		limit = MaxPaginationLimit
	}
	return page, limit
}
