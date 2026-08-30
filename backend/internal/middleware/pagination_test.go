package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParsePagination_ClampsUnsafeValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name         string
		query        string
		defaultLimit int
		wantPage     int
		wantLimit    int
	}{
		{"defaults", "", 10, 1, 10},
		{"valid", "page=3&limit=25", 10, 3, 25},
		{"zero limit falls back", "page=1&limit=0", 10, 1, 10},
		{"negative limit falls back", "limit=-1", 10, 1, 10},
		{"zero page becomes 1", "page=0", 10, 1, 10},
		{"negative page becomes 1", "page=-5", 10, 1, 10},
		{"non-numeric falls back", "page=abc&limit=xyz", 10, 1, 10},
		{"limit capped at max", "limit=10000", 10, 1, 100},
		{"small default respected", "limit=0", 5, 1, 5},
		{"invalid default becomes standard", "limit=0", -3, 1, 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)

			page, limit := ParsePagination(c, tc.defaultLimit)
			if page != tc.wantPage || limit != tc.wantLimit {
				t.Errorf("ParsePagination(%q) = (%d, %d), want (%d, %d)",
					tc.query, page, limit, tc.wantPage, tc.wantLimit)
			}
		})
	}
}
