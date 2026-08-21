# Contributing to VexGo

Thank you for contributing to VexGo.

This document defines the expected development workflow, coding standards, issue conventions, pull request requirements, and commit message format for this repository. The goal is to keep changes easy to review, easy to trace, and safe to release.

## Table of Contents

- [Motivation](#motivation)
- [Development Environment](#development-environment)
- [Project Layout](#project-layout)
- [Local Workflow](#local-workflow)
- [Code Style](#code-style)
- [Testing](#testing)
- [Issue Guidelines](#issue-guidelines)
- [Pull Request Guidelines](#pull-request-guidelines)
- [Commit Message Convention](#commit-message-convention)
- [Review Expectations](#review-expectations)
- [Scope Control](#scope-control)
- [Definition of Done](#definition-of-done)

## Motivation

VexGo should remain easy to understand, maintain, and evolve.

Contributors are expected to:

- keep changes small and focused
- preserve clear module boundaries between backend and frontend
- prefer explicit types and clear error handling
- avoid unrelated refactors
- include tests for behavior changes
- use consistent issue, pull request, and commit naming

The repository follows a lightweight engineering discipline:

- issues describe work clearly
- pull requests describe the implementation clearly
- commits remain traceable through Conventional Commits

## Development Environment

### Requirements

- Go 1.25+
- Node.js and pnpm 11
- golangci-lint (v2), gofumpt, prettier, oxlint, and `just` (recommended, used by the `justfile`)

The Nix flake provides a ready-made development shell with all tools (`go`, `gofumpt`, `golangci-lint`, `just`, `nodejs`, `oxlint`, `pnpm`, `prettier`):

```bash
nix develop
```

If you use direnv, the checked-in `.envrc` (`use flake`) activates the shell automatically. A `devbox.json` with the same core tools (`go`, `nodejs`, `pnpm`) is also available.

### Typical commands

```bash
just format            # gofumpt -w -extra . && prettier --write "**/*.{js,jsx,ts,tsx,html,md}"
just lint              # golangci-lint, prettier --check, gofumpt diff check, oxlint

go build -v ./...      # build the backend
go test -v ./...       # run backend tests

cd frontend
pnpm install
pnpm run dev           # frontend dev server with HMR
pnpm run build         # typecheck (tsc -b) + vite build + copy theme manifest
pnpm run lint          # oxlint
```

A contribution is expected to pass at least:

```bash
just format
just lint
pnpm run build # frontend
go build -v ./...
go test -v ./...
```

If your change affects runtime behavior, also verify it by running the server and exercising the relevant path manually.

### Running VexGo locally

Build the frontend once, then start the backend:

```bash
cd frontend
pnpm install
pnpm run build
cd ../backend
go run ./cmd/vexgo
```

Then visit http://127.0.0.1:3001. The default super admin account is `admin@example.com` with password `password` — change it on your profile page.

## Project Layout

```text
backend/
  cmd/vexgo/main.go        # application entry point (thin: calls app.New / app.Run)
  internal/
    app/                   # composition root: wires storage, DB, and every domain
    auth/                  # authentication and JWT
    comment/               # comments and moderation
    config/                # flag, env, and config-file parsing
    database/              # DB connection, migrations, seeding
    home/                  # homepage data endpoints
    mailer/                # email sending (SMTP)
    message/               # notifications
    middleware/            # JWT auth and permission middleware
    model/                 # GORM data models + shared seams (Notifier, FileRemover, Mailer)
    post/                  # blog post CRUD
    public/                # embedded frontend assets and SSR renderer
    router/                # route registration
    settings/              # site settings endpoints
    sso/                   # OAuth2 / OIDC login
    upload/                # file upload (local disk or S3)
    user/                  # user management
    verification/          # email verification
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

## Local Workflow

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

## Code Style

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

Backend tests use Go's standard `testing` package and run with `go test -v ./...`. For frontend changes, run `pnpm run build` (which includes the `tsc -b` typecheck) and verify behavior in the dev server.

Testing expectations:

- cover both success and failure paths where relevant
- verify boundary conditions, not only happy paths
- for regressions, add a test that reproduces the previously broken case

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
pnpm run build # frontend
go build -v ./...
go test -v ./...
```

## Issue Guidelines

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
- `message`
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

## Pull Request Guidelines

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
- [ ] `pnpm run build` passes
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

## Commit Message Convention

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

## Review Expectations

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

## Scope Control

Do not include unrelated changes in a contribution.

Avoid:

- repository-wide formatting sweeps unrelated to the task
- renaming modules without a clear reason
- speculative abstractions
- dependency additions without explicit justification
- broad refactors hidden inside a feature or fix

If a larger redesign is truly necessary, open a dedicated issue first.

## Definition of Done

A contribution is considered ready when:

- the issue is clearly defined
- the implementation is scoped correctly
- formatting, linting, checks, and tests pass
- behavior changes are verified
- the pull request description is complete
- the title follows the required convention
- no unrelated changes are included

Thank you for helping keep VexGo maintainable and consistent.
