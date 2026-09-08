// Wire types for the sso domain. Swag reads the JSON tags
// to populate the OpenAPI spec; orval turns the generated
// schemas into TypeScript interfaces.
package sso

// SSOProvidersResponse is the body of GET /api/sso/providers.
// The list contains the slugs of every enabled provider
// (e.g. ["github", "google"]); allowLocalLogin tells the
// frontend whether to render the email/password form.
type SSOProvidersResponse struct {
	Providers       []string `json:"providers" example:"github,google"`
	AllowLocalLogin bool     `json:"allow_local_login" example:"true"`
}
