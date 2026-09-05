// Package humaapi wires huma onto the existing Gin router so the
// REST API is described by a single OpenAPI 3.1 spec generated from
// Go handler signatures.
//
// The router still uses Gin; huma is mounted via the gin adapter
// (`humagin`) so the existing gin middleware (auth, rate limit,
// captcha) keeps running unchanged. huma only takes over the JSON
// REST surface; static, SSR, and HTML-emitting routes (SSO callback)
// continue to register directly with gin.
//
// The spec is not served at runtime (per project requirements: no
// document generation). The CLI in backend/cmd/openapi-spec
// instantiates the same registry and writes docs/openapi.json, which
// orval consumes at build time.
package humaapi

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"

	"github.com/gin-gonic/gin"
)

// DefaultConfig returns a huma.Config with reasonable defaults for the
// VexGo API: title + version, JSON-only content types, and a single
// /api server entry. Callers can mutate the returned config before
// passing it to New.
func DefaultConfig(title, version string) huma.Config {
	c := huma.DefaultConfig(title, version)
	c.Servers = []*huma.Server{{URL: "/api"}}
	return c
}

// New constructs a huma.API mounted on the /api prefix of the given
// gin engine. Operations registered with the returned API land under
// /api in the URL space, while non-REST routes (static files, SSR,
// SSO HTML callback) keep going through gin.
func New(r *gin.Engine, cfg huma.Config) huma.API {
	g := r.Group("/api")
	return humagin.NewWithGroup(r, g, cfg)
}

// WriteSpec serializes the given API's spec as JSON to the given
// path. Used by the backend/cmd/openapi-spec CLI to produce
// docs/openapi.json for orval to consume.
func WriteSpec(api huma.API, path string) error {
	spec := api.OpenAPI()
	if spec == nil {
		return fmt.Errorf("huma API has no OpenAPI spec")
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}
	return nil
}
