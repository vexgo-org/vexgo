// Wire types for the home/stats domain. Swag reads the JSON
// tags to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package home

// StatsAggregate describes the per-bucket counters returned by
// GET /api/stats. Each field is the count of rows visible to
// the caller; anonymous callers see a reduced view (no
// pending posts, etc.) per the service-layer policy.
type StatsAggregate struct {
	Posts      int64 `json:"posts" example:"42"`
	Users      int64 `json:"users" example:"120"`
	Comments   int64 `json:"comments" example:"318"`
	Categories int64 `json:"categories" example:"7"`
	Tags       int64 `json:"tags" example:"23"`
}

// StatsResponse is the body of GET /api/stats. The shape
// mirrors the original gin.H{ "stats": { ... } } so the
// frontend can keep treating it as a single object.
type StatsResponse struct {
	Stats StatsAggregate `json:"stats"`
}
