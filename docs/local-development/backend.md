# Backend Development

> **How-to** — run, configure, and extend the Go backend (Gin + GORM). Read the [Local Development](/local-development/quick-develop) tutorial first so the embedded SPA and theme already exist.

## Run the server

```bash
just server                                   # standard local run
just run server -c examples/config.yml        # with a config file
just run server --addr 127.0.0.1 --port 3001 --data ./data
go run backend/cmd/vexgo/main.go server       # same, without just
```

`backend/cmd/vexgo/main.go` is thin: it resolves config through `cli.Execute` and calls `app.New(cfg)`. All wiring lives in `backend/internal/app` (the composition root that picks the cache backend, builds the cipher, and injects each domain's `Deps`).

Configuration layers, highest first: CLI flags > config file (`vexgo server -c`, see `examples/`) > environment (`.env.example`) > defaults in `internal/config/config.go` (`keyDefaults` + `mapstructure` tags; never re-introduce per-source parsing). Runtime data (SQLite file, uploads) lives in `./data/` by default.

`vexgo dev` behaves like `vexgo server` plus a dev-only `--theme-dir` flag; see [Common Workflow](/local-development/workflow#iterate-on-a-theme).

## Where code lives

Each domain under `backend/internal/<domain>/` follows the same three layers:

| File            | Role                                         | Rules                                                                                      |
| --------------- | -------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `handler.go`    | HTTP parsing and response rendering          | Never touches GORM; binds request types from `types.go`                                    |
| `service.go`    | Business logic                               | Depends only on the domain `Repository` interface                                          |
| `repository.go` | `Repository` interface + GORM implementation | Every query lives here, including batch queries that prevent N+1                           |
| `types.go`      | API wire types with JSON tags                | Declared once for `swag` + `orval`; the handler can hand a request straight to the service |

Supporting pieces:

- `backend/internal/router` — `RegisterAPIRoutes` mounts every domain under `/api` (optional-JWT middleware at the group level) from the aggregate `router.Deps`. `router_test.go` locks the method+path surface: adding, removing, or renaming a route means updating that list deliberately.
- `backend/internal/app` — wires every domain; `backend/cmd/vexgo/main.go` only resolves config and calls it.
- Leaf packages importing no other backend package: `config`, `model`, `secrets`, `cache`. `model` holds GORM models plus the `Notifier`/`FileRemover` seams (`model/interfaces.go`); it must not import application logic.
- `backend/internal/public` — SSR engine, theme serving, embedded assets. `backend/internal/settings` — site settings and theme management.
- Cross-domain calls go through consumer-declared interfaces (`notification` implements `model.Notifier`, `upload` implements `model.FileRemover`). `mailer.Service` is the deliberate exception: injected as concrete `*mailer.Service` into `auth` and `settings`.
- Thread `context.Context` through every layer; handlers pass `c.Request.Context()`. Pass dependencies through `Deps` structs, never globals (the only global mutable state is the test seam `mailer.SetMailCaptureHook`, restored with `t.Cleanup`).

## How-to: add or change an endpoint

1. Declare request/response structs in the domain `types.go` with JSON tags.
2. Annotate the handler with `swag` tags (`@Summary`, `@Param`, `@Success`, `@Failure`, `@Router`); the general API block lives in `backend/cmd/vexgo/main.go`.
3. Implement parsing in `handler.go`, logic in `service.go`, queries in `repository.go`.
4. Register the route from the domain handler and update `backend/internal/router/router_test.go` so the locked route surface matches.
5. Run `just generate` (or its equivalent, see `justfile`) to refresh `docs/swagger.json` and `frontend/src/api/generated/`; never hand-edit either. Then `just format && just lint && just test`.

File uploads use `@Accept multipart/form-data` with `@Param <name> formData file true "<desc>"`. Keep annotations `swag`-formatted (`just format` runs `go tool swag fmt backend/`).

## How-to: test the backend

```bash
go test ./backend/internal/post/...   # one domain
go test ./...                         # everything (just test adds ensure-dist first)
```

Service tests usually run against in-memory SQLite through the domain's `newTestDB`/`newTestService` helpers; injected seams are faked where a DB-backed check is impractical (`model.Notifier`, `model.FileRemover`, `mailer.SetMailCaptureHook`). `page` uses a fake `Repository`. Tests are required for new user-visible behavior, bug fixes, and boundary conditions (empty/invalid input, permission boundaries).

## Troubleshooting

| Symptom                                            | Likely cause and fix                                                                                                                       |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `go run` fails on missing `dist` / `default-theme` | Clean checkout without embedded builds. Run `just build-frontend && just build-theme` (or let `just server` run `ensure-dist`).            |
| Config value ignored                               | A higher layer wins. Check flags > `-c` file > env > defaults; confirm the key exists in `keyDefaults` with a matching `mapstructure` tag. |
| New route 404s in the SPA                          | `@Router` path/method disagrees with the registered route, or `router_test.go` was not updated. Align all three, then `just generate`.     |
| `docs/swagger.json is stale` in CI                 | Backend annotations changed without regenerating. Run `just generate` and commit the result.                                               |

Next: [Frontend Development](/local-development/frontend) for the SPA loop, [Common Workflow](/local-development/workflow) for gates and codegen.
