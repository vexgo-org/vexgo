# Contributing to VexGo

Thank you for contributing to VexGo.

This document covers the development workflow, coding standards, issue conventions, pull request requirements, and commit message format for this repository, so changes stay easy to review and safe to release.

## Table of contents

- [Motivation](#motivation)
- [Development environment](#development-environment)
- [Project layout](#project-layout)
- [Local workflow](#local-workflow)
- [Code style](#code-style)
- [Testing](#testing)
- [Issue guidelines](#issue-guidelines)
- [Pull request guidelines](#pull-request-guidelines)
- [Commit message convention](#commit-message-convention)
- [Review expectations](#review-expectations)
- [Scope control](#scope-control)
- [Definition of done](#definition-of-done)

## Motivation

VexGo should remain easy to understand, maintain, and evolve.

Contributors are expected to:

- keep changes small and focused
- preserve clear module boundaries between backend and frontend
- prefer explicit types and clear error handling
- avoid unrelated refactors
- include tests for behavior changes
- use consistent issue, pull request, and commit naming

The repository expects:

- issues describe work clearly
- pull requests describe the implementation clearly
- commits remain traceable through Conventional Commits

## Development environment

### Requirements

- Go 1.26+
- bun 1.3
- golangci-lint (v2), gofumpt, prettier, oxlint, and `just` (recommended, used by the `justfile`)

The Nix flake provides a ready-made development shell with all tools (`go`, `gofumpt`, `golangci-lint`, `just`, `oxlint`, `bun`, `prettier`):

```bash
nix develop
```

If you use direnv, the checked-in `.envrc` (`use flake`) activates the shell automatically. A `devbox.json` with the same core tools (`go`, `bun`) is also available.

> The Nix package builds the frontend with `bun install --frozen-lockfile` at build time (matching the Docker and CI build), so the build needs network access. Allow it with `sandbox = false` on NixOS, or use the default (non-sandboxed) build on other systems.

### Typical commands

```bash
just format            # gofumpt -w -extra . && prettier --write "**/*.{js,jsx,ts,tsx,html,md}"
just lint              # golangci-lint, prettier --check, gofumpt diff check, oxlint

go build -v ./...      # build the backend
go test -v ./...       # run backend tests

cd frontend
bun install
bun run dev            # frontend dev server with HMR
bun run build          # typecheck (tsc -b) + vite build + copy theme manifest
bun run lint           # oxlint
```

A contribution is expected to pass at least:

```bash
just format
just lint
bun run build # frontend
go build -v ./...
go test -v ./...
```

If your change affects runtime behavior, also verify it by running the server and exercising the relevant path manually.

### Running VexGo locally

Build the frontends once, then start the backend:

```bash
# Install the admin SPA dependencies and build it, then fetch and build the
# standalone default theme (outputs are embedded into the backend binary)
cd frontend && bun install && cd ..
just build-frontend
just build-theme
just server
```

`just server` starts the server. `just run *args` passes arguments to the CLI, for example `just run server -c examples/config.yml` or `just run --version`; `just run` without arguments prints help. For the binary, use `vexgo server` to start, `vexgo server --help` for server flags (`--config/-c`, `--addr/-a`, `--port/-p`, `--data/-d`), and `vexgo --version` for the root-only version flag.

`vexgo dev` is the development entry point: it starts the server like `vexgo server` and adds a `--theme-dir` flag that overrides the active theme with a local directory, so a developer can iterate on a theme without rebuilding the embedded default theme. For example, `vexgo dev --theme-dir ../vexgo-default-theme/dist/` renders public pages from the standalone theme checkout. The `justfile` wraps this as `just theme <dir>`.

`just theme <dir>` starts the server with a theme directory, e.g. `just theme ../vexgo-default-theme/dist/`.

Then visit http://127.0.0.1:3001. The default super admin account is `admin@example.com` with password `password`; change it on your profile page.

### API codegen (swag + orval)

The typed frontend API client is generated, not handwritten. The pipeline is:

`swag` annotations on backend handlers → `docs/swagger.json` → `orval` → `frontend/src/api/generated/`

```bash
just generate            # regenerate docs/swagger.json, then the TypeScript client
just check-openapi-fresh # CI guard: fails if docs/swagger.json is stale
```

Rules:

- Declare request/response shapes as Go types with JSON tags plus swag annotations (`@Summary`, `@Param`, `@Success`, `@Failure`, `@Router`) on the handler. The general API block lives in `backend/cmd/vexgo/main.go`.
- Never hand-edit `docs/swagger.json` or anything under `frontend/src/api/generated/`; change the backend annotations/types and re-run `just generate`.
- File uploads use `@Accept multipart/form-data` with `@Param <name> formData file true "<desc>"`. (swag v2 is pinned past `v2.0.0-rc5` in `go.mod` because rc5 cannot emit a correct multipart file schema.)
- Keep annotation formatting clean: `just format` runs `go tool swag fmt backend/`; CI enforces it via `just check-swag-fmt`.
- If you add, remove, or rename a route, also update the route surface locked by `backend/internal/router/router_test.go`, and make sure the `@Router` path/method matches the registered route, otherwise the generated client calls a URL that 404s.
- In frontend code, call the API via `getVexGoAPI()` from `@/api/generated/endpoints`. Requests go through the shared axios instance in `frontend/src/api/customAxios.ts` (attaches the token, redirects to `/admin/login` on non-auth 401s).

## Project layout

```text
backend/
  cmd/vexgo/main.go        # application entry point (thin: calls app.New / app.Run)
  internal/
    api/                   # wire types shared by the REST surface
    app/                   # composition root: wires storage, DB, and every domain
    auth/                  # authentication, JWT, email verification
    cache/                 # cache backends: in-process memory + Valkey
    captcha/               # sliding-puzzle captcha
    cli/                   # cobra command line
    comment/               # comments and moderation
    config/                # flag, env, and config-file parsing
    database/              # DB connection, migrations, seeding
    home/                  # homepage data endpoints
    mailer/                # email sending (SMTP)
    middleware/            # JWT auth and permission middleware
    model/                 # GORM data models + shared seams (Notifier, FileRemover)
    notification/          # notifications
    page/                  # custom pages served at /:slug
    post/                  # blog post CRUD
    public/                # embedded frontend assets and SSR renderer
    router/                # route registration
    secrets/               # AES-256-GCM encryption of secrets at rest
    settings/              # site settings endpoints
    sso/                   # OAuth2 / OIDC login
    upload/                # file upload (local disk or S3)
    user/                  # user management
frontend/
  src/
    components/            # React components
    pages/                 # route pages
    hooks/                 # custom hooks
    lib/                   # shared utilities
    locales/               # i18n strings
    types/                 # shared TypeScript types
  package.json
  vite.config.ts
scripts/                   # helper scripts (e.g. API test examples)
examples/                  # example configuration files
docs/                      # user documentation (docsify site, English + zh-cn)
docs/test-cases/           # manual and unit test-case catalogues per feature
nix/                       # Nix packaging
.github/workflows/         # CI pipelines
justfile                   # format / lint tasks
```

General expectations:

- backend production code lives under `backend/internal/...`
- frontend code lives under `frontend/src/...`
- Go tests live next to the code they test (`*_test.go`)
- avoid placing unrelated experiments or scratch files in the repository
- each domain package follows the three-layer pattern `handler.go → service.go → repository.go` (HTTP adapter, business logic, persistence); keep GORM queries inside `repository.go`
- thread `context.Context` through service and repository methods (handlers pass `c.Request.Context()`)

If a module becomes too broad, split it by responsibility rather than growing a single file indefinitely.

## Local workflow

Recommended local workflow:

1. Create or pick an issue
2. Use a focused branch
3. Make the smallest effective change
4. Add or update tests
5. Run formatting, linting, type checking, and tests
6. Open a pull request with a clear description

A good contribution should be:

- easy to review in one sitting
- limited to one clear goal
- supported by tests where behavior changes
- free of unrelated cleanup

## Code style

### General

Follow these principles:

- prefer small, composable functions
- keep module boundaries explicit
- model domain states with meaningful types
- keep public APIs minimal
- avoid unnecessary indirection

### Go

- format with `gofumpt` (`-extra` enabled) and lint with `golangci-lint` (errcheck, govet, ineffassign, staticcheck, unused)
- propagate errors explicitly and add context where it helps; do not swallow errors
- respect the `backend/internal` package boundaries and keep the dependency graph clean (for example, `model` must not import application logic, and `config` must stay a pure setup module)
- use the `vexgo` module path in imports (`github.com/vexgo-org/vexgo/backend/internal/...`)
- pass dependencies explicitly (see `router.Deps` and the `Deps` structs in each domain) instead of relying on global mutable state
- keep services database-agnostic: depend on the domain `Repository` interface, put GORM queries in `repository.go`, and use the shared seams in `model/interfaces.go` (`Notifier`, `FileRemover`, `Mailer`) for cross-domain calls

### TypeScript / React

- format with prettier and lint with oxlint
- follow the existing component conventions in `frontend/src/components` (Tailwind + shadcn/ui style)
- keep shared types in `frontend/src/types` and reusable logic in `frontend/src/lib`
- avoid `any`; prefer explicit types at module boundaries and let TypeScript infer the rest

### Naming

- use clear and stable names
- keep module names aligned with responsibility
- use English for code, documentation, and comments
- avoid abbreviations unless they are widely understood

## Testing

Tests are required for:

- new user-visible behavior
- bug fixes
- boundary conditions
- regressions that could reappear

Backend tests use Go's standard `testing` package and run with `go test -v ./...`. For frontend changes, run `bun run build` (which includes the `tsc -b` typecheck) and verify behavior in the dev server.

Testing expectations:

- cover both success and failure paths where relevant
- verify boundary conditions, not only happy paths
- for regressions, add a test that reproduces the previously broken case

Feature-level catalogues of the cases behind those tests live in `docs/test-cases/` (for example `docs/test-cases/comment-moderation.md` for the moderation pipeline). They are English-only and reference the Go test files that implement each case; add a row when you add a case worth tracking.

Examples of cases that should be verified:

- empty input
- invalid input
- permission boundaries (guest / user / admin)
- interaction between state transitions
- previously broken regressions

At minimum, run:

```bash
go test -v ./...
```

Before opening a pull request, also run:

```bash
just lint
bun run build # frontend
go build -v ./...
go test -v ./...
```

## Issue guidelines

Issues should describe one clear unit of work.

### Issue title format

Use the following format:

```text
<type>(<scope>): <summary>
```

Examples:

```text
feat(upload): support drag-and-drop upload in the editor
fix(auth): prevent session token from expiring mid-write
refactor(post): split post service from handler logic
docs(contributing): define issue and PR conventions
```

### Allowed issue types

Use one of the following types:

- `build`
- `chore`
- `ci`
- `docs`
- `feat`
- `fix`
- `perf`
- `refactor`
- `style`
- `test`

### Scope rules

The `scope` should refer to a stable functional area, such as:

- `auth`
- `post`
- `comment`
- `upload`
- `sso`
- `settings`
- `middleware`
- `database`
- `mailer`
- `notification`
- `router`
- `config`
- `frontend`
- `theme`
- `deps`
- `docs`
- `ci`

Do not use unstable or overly specific scope values such as temporary implementation details, ticket IDs, or pixel-level descriptions.

### Summary rules

The `summary` should:

- use imperative mood
- express one core intent
- stay concise
- avoid implementation detail

Good:

- `feat(upload): support drag-and-drop upload in the editor`

Bad:

- `upload changes`
- `feat: add something`
- `fix(upload): move the button to the top-right and make it 32px and update CSS and cleanup state logic`

### Issue content expectations

A good issue should include:

- motivation or problem statement
- current behavior or limitation
- expected behavior
- scope
- acceptance criteria
- constraints or risks, if relevant

For bugs, include:

- steps to reproduce
- actual behavior
- expected behavior
- impact

For features, include:

- why the feature is needed
- what should change
- what is explicitly out of scope
- how completion will be verified

## Pull request guidelines

Pull requests should remain tightly scoped and easy to review.

### PR title format

Pull request titles should generally follow the same format as issue titles:

```text
<type>(<scope>): <summary>
```

If the pull request resolves a single issue, prefer using the same title for traceability.

### PR description should include

- motivation
- implementation summary
- key invariants
- edge cases
- verification steps
- linked issue(s)

Suggested PR checklist:

```md
- [ ] The change is scoped to one clear objective
- [ ] Code is formatted (`just format`)
- [ ] `just lint` passes
- [ ] `bun run build` passes
- [ ] `go build -v ./...` passes
- [ ] `go test -v ./...` passes
- [ ] Tests were added or updated where needed
- [ ] No unrelated refactor is included
- [ ] Public API changes are explicitly called out
```

### PR size guidance

Prefer small to medium pull requests.

A pull request should not combine:

- feature work and refactor work
- bug fixes and broad cleanup
- behavioral change and unrelated renaming

If cleanup is necessary to enable the main change, keep it minimal and explain it clearly.

## Commit message convention

This repository uses Conventional Commits.

Format:

```text
<type>(<scope>): <summary>
```

Examples:

```text
feat(upload): support drag-and-drop upload in the editor
fix(auth): prevent session token from expiring mid-write
refactor(post): split post service from handler logic
docs(contributing): define issue and PR conventions
```

### Allowed commit types

- `build`
- `chore`
- `ci`
- `docs`
- `feat`
- `fix`
- `perf`
- `refactor`
- `style`
- `test`

### Commit rules

- use English only
- keep the first line concise
- use imperative mood
- make each commit represent one meaningful change
- avoid mixed-purpose commits
- do not hide behavior changes inside formatting-only or refactor-only commits

If a change is breaking, explicitly describe it in the commit body using a `BREAKING CHANGE:` footer.

Example:

```text
feat(api): replace legacy theme config shape

BREAKING CHANGE: theme config now requires an explicit mode field.
```

## Review expectations

Reviewers will primarily look for:

- correctness
- API clarity
- maintainability
- test coverage
- boundary handling
- unnecessary coupling
- scope discipline

Common review concerns include:

- swallowed or unchecked errors
- weak type modeling (e.g. `any` in TypeScript, or stringly-typed states in Go)
- hidden breaking changes
- missing regression tests
- unrelated edits in the same change

## Scope control

Do not include unrelated changes in a contribution.

Avoid:

- repository-wide formatting sweeps unrelated to the task
- renaming modules without a clear reason
- speculative abstractions
- dependency additions without explicit justification
- broad refactors hidden inside a feature or fix

If a larger redesign is truly necessary, open a dedicated issue first.

## Definition of done

A contribution is considered ready when:

- the issue is clearly defined
- the implementation is scoped correctly
- formatting, linting, checks, and tests pass
- behavior changes are verified
- the pull request description is complete
- the title follows the required convention
- no unrelated changes are included
