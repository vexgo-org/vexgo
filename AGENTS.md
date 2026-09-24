# AGENTS.md

VexGo is a self-hosted blog CMS: a Go backend (Gin + GORM) serving the JSON API, server-side-rendered public pages, and an embedded admin SPA (React + TypeScript, Vite, Tailwind, shadcn/ui). Module path: `github.com/vexgo-org/vexgo`.

`CONTRIBUTING.md` owns the development environment, API codegen, and issue/PR/commit conventions. This file is what you need before changing code.

## Where things live

- `backend/internal/<domain>/`: one domain per package, always the same three layers: `handler.go` (HTTP parsing and response rendering; never touches GORM), `service.go` (business logic; depends only on the domain `Repository` interface), `repository.go` (the `Repository` interface and its GORM implementation; every query, including batch queries that prevent N+1). The API wire types live beside them in `types.go`: request and response structs carrying the JSON tags that `swag` and `orval` consume, declared once; the handler binds them and can hand a request straight to the service instead of copying it into a second struct.
- `backend/internal/router`: `RegisterAPIRoutes` mounts every domain's routes under `/api` (optional-JWT middleware applies at the group level) from the aggregate `router.Deps`, the struct that collects each domain's own `Deps`. `router_test.go` locks the complete method+path surface, so adding, removing, or renaming a route means updating that list deliberately.
- `backend/internal/app`: composition root that wires every domain. `backend/cmd/vexgo/main.go` only resolves config via `cli.Execute` and calls `app.New(cfg)`.
- Leaf packages that import no other backend package: `config`, `model`, `secrets`, `cache`. `model` holds the GORM models and the cross-domain seams `Notifier`/`FileRemover` (`model/interfaces.go`); it must not import application logic.
- `backend/internal/public`: SSR engine, theme serving, embedded assets. `backend/internal/settings`: site settings and theme management.
- `frontend/`: the admin SPA. `bun run build` writes it to `backend/internal/public/dist`, which is gitignored and embedded into the backend binary, so the backend serves frontend changes only after a rebuild.

## Public pages and themes

- Public routes render Go templates from the active theme: `/`, `/post/:slug`, `/posts/:slug`, `/user/:id`, custom pages at `/:slug`, assets under `/theme-assets/`. Every non-public route lives under `/admin/...`; legacy top-level URLs 301-redirect there with the query string intact, so emailed `?token=...` links keep working. Engine: `theme_render.go` + `theme_handlers.go`.
- A theme is a directory: `vexgo-theme.json` manifest, `index.html`/`post.html`/`page.html`/`user.html`/`404.html`, optional `i18n/`, `seed/`, `assets/`. The built-in theme is built from the standalone `vexgo-org/vexgo-default-theme` repo by `just build-theme`; uploaded themes are extracted to `data/theme/<id>/`.
- Template data: `.Site`, `.Posts`, `.Post`, `.User`, `.Pagination`, `.Query`, plus `.PopularPosts`/`.PopularTags` on the home page; helpers `date`, `truncate`, `userURL`, `categoryURL`. Markdown renders through goldmark in safe mode (GFM).
- Comments are rendered by a vanilla-JS widget: `<div id="vexgo-comments" data-post-id="{{.Post.ID}}"></div>` + `<script src="/theme-assets/comments.js" defer></script>`. It uses the public comment API with the admin SPA's localStorage session.
- Trap: `go()` expressions inside HTML attributes must not contain double quotes. React escapes them to `&quot;`, which breaks template parsing. Use the helper funcs instead.
- Full reference: `docs/concepts/theming.md`, `docs/guides/theme-development.md`, `docs/reference/theme-templates.md`.

## Commands

Requirements: Go, bun, and the tools in the root `justfile` (`nix develop` provides them). Install with `go mod download` and `cd frontend && bun install`. Configuration layers server flags > config file (`vexgo server -c`, see `examples/`) > environment (`.env.example`) > defaults; runtime data lives in `./data`.

```bash
just format           # gofumpt -w -extra . && prettier --write ... && go tool swag fmt backend/
just lint             # golangci-lint, deadcode, prettier, gofumpt, oxlint, gopls, swag fmt, OpenAPI freshness
just test             # go test -v ./...
just server           # start the server
just build            # admin SPA + default theme + backend
just build-frontend   # admin SPA only
just build-theme      # fetch and build the standalone default theme
just generate         # swag -> docs/swagger.json -> orval -> frontend client
```

- Run `just format` and `just lint` after any change, `just test` for behavior changes, and `just build` before handoff. If `just` is unavailable, run the corresponding recipe from the `justfile` directly.
- `just lint` fails on a stale `docs/swagger.json` or unformatted swag annotations. Never hand-edit `docs/swagger.json` or `frontend/src/api/generated/`; change the handler types/annotations and run `just generate`.
- Start with `vexgo server`; bare `vexgo` prints help. Server flags are `--config/-c`, `--addr/-a`, `--port/-p`, and `--data/-d`; version is root-only (`vexgo --version` or `-V`). `just run *args` passes CLI arguments (e.g. `just run server -c examples/config.yml` or `just run --version`); bare `just run` prints help.
- Frontend dev loop: `bun run dev` in `frontend/` (API base from `VITE_API_URL`, default `http://localhost:3001/api`).

## Testing

- Tests are required for new user-visible behavior, bug fixes, and boundary conditions (empty input, invalid input, permission boundaries).
- Backend tests use the standard `testing` package. Service tests usually run against in-memory SQLite through the domain's `newTestDB`/`newTestService` helpers; injected seams are faked where a DB-backed check is impractical (`model.Notifier`, `model.FileRemover`, `mailer.SetMailCaptureHook`), while `page` uses a fake `Repository`.
- There is no frontend test framework: `bun run build` (`tsc -b` + vite) is the typecheck gate, and behavior changes are verified in the dev server.

## Code style

- Go: `gofumpt -extra` + `golangci-lint`; import through the full module path; thread `context.Context` through every layer (handlers pass `c.Request.Context()`); propagate errors with context instead of swallowing them; pass dependencies through `Deps` structs. The only global mutable state is the test seam `mailer.SetMailCaptureHook`, which tests restore with `t.Cleanup`.
- Config keys live in `keyDefaults` (`internal/config/config.go`) with a matching `mapstructure` tag; viper layers flags > file > env > defaults, so never re-introduce per-source parsing.
- Cross-domain calls go through consumer-declared interfaces where practical (`notification` implements `model.Notifier`, `upload` implements `model.FileRemover`). `mailer.Service` is an exception: injected as the concrete `*mailer.Service` into `auth` and `settings`, send-only, with account/token persistence in the domain repositories.
- `cache` holds the memory and Valkey backends behind one `Cache` interface; consumers declare narrow seams (`middleware.CounterStore`, `sso.StateStore`) and `internal/app` picks the backend from `cache_enabled` and `valkey_enabled`/`valkey_url`.
- `secrets` encrypts DB-stored secrets at rest with AES-256-GCM (SMTP password, AI/comment-moderation keys): consumers declare `SecretCipher`, `internal/app` builds it from `settings_encryption_key` (unset → plaintext), and `database.MigrateSecretsAtRest` encrypts existing plaintext at startup.
- `cli` defines the cobra command line, binds flags to viper, and imports only `config`.
- TypeScript/React: prettier + oxlint (`frontend/.oxlintrc.json`; `typescript/no-explicit-any` is an error); `@` maps to `frontend/src/`; shared types in `frontend/src/types/`, logic in `frontend/src/lib/`, i18n in `frontend/src/locales/` (update both `en-US.ts` and `zh-CN.ts`), UI components follow the shadcn/ui + Tailwind conventions in `frontend/src/components/ui/`.
- English for all code, documentation, and comments.

## Security

- Roles, highest first: `super_admin`, `admin`, `author`, `contributor`, `guest` (`model/user.go`).
- Middleware enforces authorization: never bypass it in handlers or services, and keep the permission checks on uploaded resources.
- Validate uploaded files and never trust client-provided filenames.
- Nothing under `data/`, `backend/internal/public/dist/`, `backend/internal/public/default-theme/`, or `.env` is committed.
