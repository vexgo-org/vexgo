# Local Development

> **Tutorial + How-to** — this section takes you from zero to a running local stack, then serves as a lookup manual for daily backend and frontend work.

By the end of the tutorial part you will have the backend API, the admin SPA, and the default theme running locally, with one test post published. The How-to part covers the loops you will use every day.

Audience: new contributors and experienced Go/React developers. No prior VexGo knowledge is required, but basic Go and TypeScript familiarity helps in the How-to part.

Scope: environment setup, backend loop, frontend loop, test/lint gates, API codegen, theme iteration, troubleshooting. Production topics (reverse proxy, HTTPS, systemd, S3/SSO full setup) live in [Configuration](/guides/configuration) and [Deployment](/guides/deployment).

Contents:

- [Backend Development](/local-development/backend) — run the Go server, config layers, domain layout, add an endpoint.
- [Frontend Development](/local-development/frontend) — run the admin SPA with HMR, API base URL, embedded build.
- [Common Workflow](/local-development/workflow) — format/lint/test gates, swag + orval codegen, theme iteration, troubleshooting.

## Prerequisites

| Tool | Version | Notes                                         |
| ---- | ------- | --------------------------------------------- |
| Go   | 1.26+   | Backend, `go test`, `swag` via `go tool`      |
| bun  | 1.3     | Frontend deps, dev server, builds, `orval`    |
| just | latest  | Recommended; every recipe below assumes it    |
| git  | any     | Clone source and the standalone default theme |

The Nix flake provides all of the above via `nix develop` (direnv activates it automatically through the checked-in `.envrc`).

Default local addresses and credentials:

| Item                  | Value                                                       |
| --------------------- | ----------------------------------------------------------- |
| Backend + public site | `http://127.0.0.1:3001`                                     |
| Admin SPA             | `http://127.0.0.1:3001/admin/`                              |
| Super admin           | `admin@example.com` / `password` (change after first login) |

## Tutorial

### Step 1: Clone and install dependencies

```bash
git clone https://github.com/vexgo-org/vexgo.git
cd vexgo

# Install backend dependencies
go mod download

# Install the remaining dependencies
cd frontend
bun install
cd ..
```

> **What just happened?** You fetched the Go module dependencies and the admin SPA dependencies. Nothing is built yet.

### Step 2: Build the embedded frontends once

There are two frontends to build: the admin SPA and the default theme.

1. Admin SPA

```bash
bun run --cwd frontend build
```

2. Default theme

```bash
# Run somewhere outside the vexgo checkout
git clone https://github.com/vexgo-org/vexgo-default-theme.git
cd vexgo-default-theme
bun install
bun run build

mkdir -p path/to/vexgo/backend/internal/public/default-theme/
cp -r dist/* path/to/vexgo/backend/internal/public/default-theme/
```

Or copy everything under `vexgo-default-theme/dist/` into `vexgo/backend/internal/public/default-theme/` by hand.

If `just` is available (e.g. on Linux), you can use it as a shortcut:

```bash
just build-frontend
just build-theme
```

`just build-frontend` runs `bun run --cwd frontend build` and writes the admin SPA to `backend/internal/public/dist/` (gitignored). `just build-theme` clones `vexgo-org/vexgo-default-theme` at a pinned ref and builds it into `backend/internal/public/default-theme/` (gitignored). Both outputs are embedded into the Go binary with `go:embed`, so the backend cannot start from a clean checkout until they exist (`just server` runs `ensure-dist` for the same reason).

Skip this step on later runs unless you cleaned those directories.

### Step 3: Start the backend

```bash
go run backend/cmd/vexgo/main.go server
```

Or with `just`:

```bash
just server
```

It is `go run backend/cmd/vexgo/main.go server` plus the `ensure-dist` guard. Open `http://127.0.0.1:3001`. You should see the default theme home page backed by a fresh SQLite database in `./data/`.

Useful variants:

```bash
go run backend/cmd/vexgo/main.go server -c examples/config.yml   # load a config file
go run backend/cmd/vexgo/main.go server --port 8080 --data /tmp/vexgo-data
go run backend/cmd/vexgo/main.go --version                       # root-only version flag
```

Bare `vexgo` prints help only; `vexgo server --help` lists `--config/-c`, `--addr/-a`, `--port/-p`, `--data/-d`.

### Step 4: Log in and change the password

1. Open `http://127.0.0.1:3001/admin/login`.
2. Log in with `admin@example.com` / `password`.
3. Open your profile and change the password immediately.

Legacy top-level URLs such as `/login` 301-redirect to their `/admin/` equivalent with the query string preserved.

### Step 5: Publish a test post and verify

1. Open `http://127.0.0.1:3001/admin/write`.
2. Create a post titled `Hello, VexGo!` and publish it.
3. Check that it appears on `http://127.0.0.1:3001`.

Success means: public site renders, admin login works, and a new post flows from the SPA through the API into SQLite and back to the theme.

## Daily loop cheat sheet

| I want to…                            | Run                                       | Details                                                            |
| ------------------------------------- | ----------------------------------------- | ------------------------------------------------------------------ |
| Run backend + embedded SPA/theme      | `just server`                             | [Backend](/local-development/backend)                              |
| Run SPA dev server with HMR           | `bun run dev` in `frontend/`              | [Frontend](/local-development/frontend)                            |
| Iterate on a theme without rebuilding | `just theme ../vexgo-default-theme/dist/` | [Workflow](/local-development/workflow#iterate-on-a-theme)         |
| Format, lint, test before a PR        | `just format && just lint && just test`   | [Workflow](/local-development/workflow#pass-the-gates-before-a-pr) |
| Regenerate the typed API client       | `just generate`                           | [Workflow](/local-development/workflow#regenerate-the-api-client)  |

Config resolution order is flags > config file > environment (`.env`, see `.env.example`) > defaults; runtime data lives in `./data/`. Full key list: [Configuration](/guides/configuration).

## What's next?

- Backend internals: [Backend Development](/local-development/backend).
- SPA internals: [Frontend Development](/local-development/frontend).
- Gates, codegen, themes, fixes: [Common Workflow](/local-development/workflow).
- System overview: [Architecture](/concepts/architecture).
- Every endpoint: [API Reference](api.html).
