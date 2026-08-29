# AGENTS.md

## Project Overview

VexGo is a self-hosted blog CMS. The repository is a full-stack app:

- **Backend**: Go (Gin, GORM) serving an HTTP API and SSR-rendered pages. Supports SQLite (default), MySQL, and PostgreSQL.
- **Frontend**: React + TypeScript SPA built with Vite and Tailwind CSS, using shadcn/ui-style components.
- Go module path: `github.com/vexgo-org/vexgo`

Key architectural fact: **the frontend build output is written to `backend/internal/public/dist` and embedded into the backend binary**. After changing frontend code you must rebuild the frontend (`pnpm run build` in `frontend/`) for the backend to serve it. The dev server (`pnpm run dev`) proxies/points at the backend API at `http://localhost:3001/api` (configurable via `VITE_API_URL`).

## Setup

Requirements: Go, Node.js + pnpm, and project tools. A Nix development shell is available via `nix develop`.

Install dependencies:

```bash
go mod download
cd frontend && pnpm install
```

Configuration priority: command-line flags > config file (`-c path/to/config.yml`, see `examples/`) > environment variables (see `.env.example`) > defaults. Data (SQLite DB, uploads) lives under `./data` by default.

## Development Workflow

The `justfile` at the repo root defines the common tasks:

```bash
just format           # gofumpt -w -extra . && prettier --write "**/*.{js,jsx,ts,tsx,html,md}"
just lint             # golangci-lint + prettier --check + gofumpt diff check + oxlint + gopls check
just test             # go test -v ./...
just run              # ensure dist exists, then go run backend/cmd/vexgo/main.go
just build            # build frontend, then build backend
just build-frontend   # pnpm --dir frontend run build
just build-backend    # ensure dist exists, then go build backend/cmd/vexgo/main.go
```

## Testing Instructions

- Backend tests use Go testing package.
- Add tests for new behavior, bugs, and permission boundaries.
- Services should be tested through Repository interfaces with fakes.
- Services are database-agnostic behind a `Repository` interface — unit-test them with fakes (existing `service_test.go` files follow this pattern).
- The frontend has no unit test framework; the `pnpm run build` step (which runs `tsc -b`) is the type check gate. Verify behavior changes in the dev server.
- Tests are required for new user-visible behavior, bug fixes, and boundary conditions (empty input, invalid input, permission boundaries: guest / user / admin).

## Code Style

### Go (backend)

- Format with `gofumpt -extra`; lint with `golangci-lint`.
- Each domain package under `backend/internal/` follows the three-layer pattern:
  - `handler.go` — HTTP request parsing and response rendering; never touches GORM.
  - `service.go` — business logic; depends only on the domain `Repository` interface.
  - `repository.go` — persistence interface + GORM implementation; all SQL/GORM queries live here (including batch operations to prevent N+1).
- Thread `context.Context` through all layers; handlers pass `c.Request.Context()`.
- Propagate errors explicitly with context; never swallow errors.
- Pass dependencies explicitly via `Deps` structs (see `router.Deps`) — no global mutable state, except the single sanctioned test seam `mailer.SetMailCaptureHook` (see below).
- Imports use the full module path: `github.com/vexgo-org/vexgo/backend/internal/<package>`.
- Package dependency rules (acyclic graph):
  - `config/`, `model/`, and `secrets/` are leaf packages — import no other backend module.
  - `model` holds GORM models plus cross-domain seams (`Notifier`, `FileRemover` in `model/interfaces.go`); it must not import application logic.
  - `config` is a pure setup module.
  - `secrets` provides AES-256-GCM encryption at rest for DB-stored secrets (SMTP password, AI/comment-moderation API keys). Consuming domains declare their own narrow `SecretCipher` interface; the composition root builds `secrets.Cipher` from `settings_encryption_key` (nil when unset → plaintext fallback) and `database.MigrateSecretsAtRest` encrypts plaintext values in place at startup.
  - `cli/` defines the cobra command line (flags, help, version, `.env` loading) and binds the flags to viper; it imports only `config`.
  - Config keys live in `keyDefaults` (`internal/config/config.go`) with a matching `mapstructure` tag on `Config`; viper layers flags > config file > environment > defaults, so do not re-introduce per-source parsing.
  - Cross-domain calls go through consumer-declared interfaces where practical: `notification` implements `model.Notifier`, `upload` implements `model.FileRemover`; the consuming domain owns any interface it defines (e.g. auth's `CaptchaChecker`), with the composition root (`internal/app`) wiring concrete implementations.
  - `mailer.Service` is the exception to interface-based injection: it is injected as the concrete type (`*mailer.Service`) into `auth` and `settings`. It is send-only (SMTP side effect plus token-free rendering); account/token persistence lives in domain repositories, not in mailer. For tests, `mailer.SetMailCaptureHook` captures rendered emails instead of dialing SMTP — this package-level hook is the one sanctioned piece of mutable global state; tests must restore it with `t.Cleanup`.
- Entry point `backend/cmd/vexgo/main.go` is thin (resolves configuration via `cli.Execute`, then calls `app.New(cfg)`); `internal/app` is the composition root that wires everything.
- The backend serves both the JSON API (`/api/...`) and server-side-rendered public pages (themes managed by `internal/public` and `internal/settings`).

### TypeScript / React (frontend)

- Format with `prettier`, lint with `oxlint` (config: `frontend/.oxlintrc.json`; `typescript/no-explicit-any` is an error).
- Follow the existing shadcn/ui + Tailwind component conventions in `frontend/src/components/ui/`.
- Path alias `@` maps to `frontend/src/`.
- Shared types go in `frontend/src/types/`, reusable logic in `frontend/src/lib/`, i18n strings in `frontend/src/locales/` (both `en-US.ts` and `zh-CN.ts` must be updated when adding UI strings).
- Avoid `any`; prefer explicit types at module boundaries.

### Naming and language

- Use English for all code, documentation, and comments.
- Use clear, stable names; avoid abbreviations unless widely understood.

## Security Rules

- Auth roles are `user`, `admin`, and `super_admin`.
- Permission checks are enforced by middleware.
- Never bypass authorization checks in handlers/services.
- Validate uploaded files and never trust client-provided filenames.
- Preserve permission checks for uploaded resources.

## Validation Commands

Run validation after implementation is complete, not after every individual edit. All validation commands should pass before considering the task complete.

```bash
just format
just lint
just test
just build
```

## Additional Notes

If the `just` command is unavailable, read the corresponding recipes from the root `justfile` and execute the underlying commands directly.
