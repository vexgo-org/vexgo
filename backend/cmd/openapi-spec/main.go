// Command openapi-spec emits docs/openapi.json from the live huma
// registry built in the same way the running server builds it.
//
// Unlike a hand-written spec or a per-domain reflection pass, this
// CLI uses the actual handler functions, gin middleware and route
// registration that the server uses. The output is consumed by
// orval at build time to generate the typed frontend client.
//
// Run via `just openapi-spec` (or `go run ./backend/cmd/openapi-spec`).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/humaapi"
)

func main() {
	out := flag.String("o", "docs/openapi.json", "output path for the OpenAPI spec")
	flag.Parse()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := humaapi.New(r, humaapi.DefaultConfig("VexGo API", "0.1.0"))

	if err := humaapi.WriteSpec(api, *out); err != nil {
		fmt.Fprintf(os.Stderr, "openapi-spec: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", *out)
}
