package api

// Stats aggregates the site-wide counters shown on the admin home page.
type Stats struct {
	Posts      int64 `json:"posts"`
	Users      int64 `json:"users"`
	Comments   int64 `json:"comments"`
	Categories int64 `json:"categories"`
	Tags       int64 `json:"tags"`
}

// StatsResponse is the body of GET /api/stats.
type StatsResponse struct {
	Stats Stats `json:"stats"`
}
