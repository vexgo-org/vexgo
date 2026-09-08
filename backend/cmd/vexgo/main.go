// Command vexgo is the VexGo server entry point. It resolves configuration
// via the cli package, wires the application through the app package, and
// serves HTTP until shutdown.
//
// The swaggo annotation block below is read by `swag init --v3.1` (see
// justfile `generate`) to generate docs/swagger.json. The generated
// spec is consumed by `orval` to produce the typed axios client under
// frontend/src/api/generated/.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/vexgo-org/vexgo/backend/internal/app"
	"github.com/vexgo-org/vexgo/backend/internal/cli"
)

// Version is the build version, overridable at build time via ldflags:
//
//	go build -ldflags "-X main.Version=1.2.3"
var Version = "dev"

//	@title			VexGo API
//	@version		1.0.0
//	@description	Self-hosted blog CMS HTTP API.
//	@termsOfService	https://github.com/vexgo-org/vexgo

//	@contact.name	GitHub Issues
//	@contact.url	https://github.com/vexgo-org/vexgo/issues

//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT bearer token issued by /api/auth/login

// @tag.name			auth
// @tag.description	Authentication, registration, profile, email
// @tag.name			users
// @tag.description	User administration and creator application
// @tag.name			posts
// @tag.description	Post CRUD, drafts, likes
// @tag.name			comments
// @tag.description	Comments and moderation queue
// @tag.name			categories
// @tag.description	Post categories
// @tag.name			tags
// @tag.description	Post tags
// @tag.name			notifications
// @tag.description	User notifications
// @tag.name			uploads
// @tag.description	File uploads
// @tag.name			stats
// @tag.description	Home-page statistics
// @tag.name			captcha
// @tag.description	Sliding-puzzle captcha
// @tag.name			sso
// @tag.description	Single sign-on provider list
// @tag.name			config
// @tag.description	Site configuration (admin only)
func main() {
	cfg, err := cli.Execute(Version, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "vexgo: error: %v\n", err)
		os.Exit(2)
	}
	if cfg == nil {
		// Help or version information was printed; nothing to run.
		return
	}

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialize application", "err", err)
		os.Exit(1)
	}

	if err := application.Run(); err != nil {
		slog.Error("failed to start server", "err", err)
		os.Exit(1)
	}
}
