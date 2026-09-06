// Wire types shared by the REST surface. Swag reads the JSON
// tags to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package api

// ErrorResponse is the body of any non-2xx response across the
// REST surface. The frontend uses the `error` field for toast
// messages and to surface validation issues.
type ErrorResponse struct {
	// Error is a short, human-readable description of what went
	// wrong. The server never returns raw stack traces or DB
	// error strings here; the goal is to be informative without
	// leaking internals.
	Error string `json:"error" example:"Invalid request payload"`
}

// CodeErrorResponse is the body of any 4xx response that also
// returns a machine-readable error code (e.g. "slug_taken" or
// "duplicate_name") so the frontend can branch on it without
// parsing the human-readable string. The optional fields are
// only present for endpoints that need them.
type CodeErrorResponse struct {
	Error string `json:"error" example:"Slug is already taken"`
	Code  string `json:"code,omitempty" example:"slug_taken"`
}

// NotFoundWithIDResponse is the body of GET /api/posts/by-id/{id}
// on 404. The postId is echoed back so the frontend can confirm
// which id was looked up.
type NotFoundWithIDResponse struct {
	Error  string `json:"error" example:"Post does not exist"`
	PostID string `json:"postId" example:"42"`
}

// NotFoundWithSlugResponse is the body of GET /api/posts/{slug}
// on 404. The slug is echoed back for the same reason.
type NotFoundWithSlugResponse struct {
	Error string `json:"error" example:"Post does not exist"`
	Slug  string `json:"slug" example:"my-post"`
}
