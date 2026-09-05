package api

// Stats aggregates the site-wide counters shown on the admin home page.
type Stats struct {
	Posts      int64 `json:"posts" doc:"Number of published posts"`
	Users      int64 `json:"users" doc:"Number of registered users"`
	Comments   int64 `json:"comments" doc:"Number of comments"`
	Categories int64 `json:"categories" doc:"Number of categories"`
	Tags       int64 `json:"tags" doc:"Number of tags"`
}

// StatsResponse is the body of GET /api/stats.
type StatsResponse struct {
	Stats Stats `json:"stats" doc:"Site-wide counters"`
}
