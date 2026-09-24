# Common Workflow

> The shared gates, API codegen, theme iteration, and troubleshooting checklist for VexGo local development. Assumes the [Local Development](/local-development/quick-develop) tutorial already runs.

## Pass the gates before a PR

```bash
just format   # gofumpt -w -extra . && prettier --write ... && go tool swag fmt backend/
just lint     # golangci-lint, deadcode, prettier --check, gofumpt check, oxlint, gopls, swag fmt, OpenAPI freshness
just test     # ensure-dist + go test ./...
just build    # admin SPA + default theme + backend
```

Minimum before opening a PR (see `CONTRIBUTING.md`):

```bash
just format
just lint
bun run build   # frontend typecheck gate (tsc -b) + bundle
go build ./...
go test ./...
```

`just lint` fails on stale `docs/swagger.json` or unformatted `swag` annotations. If `just` is unavailable, run the corresponding recipe from the `justfile` directly.

## Regenerate the API client

The typed frontend client is generated, not handwritten:

`swag` annotations on backend handlers → `docs/swagger.json` → `orval` → `frontend/src/api/generated/`

```bash
just generate            # regenerate docs/swagger.json, then the TypeScript client
just check-openapi-fresh # CI guard: fails if docs/swagger.json is stale
```

Rules:

- Change backend types/annotations, then regenerate; never hand-edit `docs/swagger.json` or `frontend/src/api/generated/`.
- File uploads use `@Accept multipart/form-data` with `@Param <name> formData file true "<desc>"`.
- If you add, remove, or rename a route, update `backend/internal/router/router_test.go` and keep the `@Router` path/method identical to the registered route, otherwise the generated client calls a URL that 404s.
- In frontend code, call the API via `getVexGoAPI()` from `@/api/generated/endpoints`.

## Iterate on a theme

`vexgo dev` starts the server like `vexgo server` and adds a dev-only `--theme-dir` flag that overrides the active theme with a local directory:

```bash
just theme ../vexgo-default-theme/dist/
# = go run backend/cmd/vexgo/main.go dev --theme-dir ../vexgo-default-theme/dist/
```

Use it to iterate on a theme without rebuilding the embedded default theme or uploading anything. `server` rejects `--theme-dir` (see `backend/internal/cli/cli_test.go`). Full theme authoring lives in [Theme Development](/local-development/theme-development) and [Theming](/concepts/theming).

Template parsing trap: `go()` expressions inside HTML attributes must not contain double quotes. React escapes them to `&quot;`, which breaks template parsing. Use the helper funcs (`date`, `truncate`, `userURL`, `categoryURL`) instead.

## Troubleshooting

| Symptom                                     | Fix                                                                                                    |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Backend cannot start on a clean checkout    | Run `just build-frontend && just build-theme`; both outputs are gitignored but required by `go:embed`. |
| Frontend change invisible via `just server` | Rebuild: `just build-frontend`, then reload `/admin/`.                                                 |
| Port already in use                         | Change `--port` (e.g. `just run server --port 8080`) and match `VITE_API_URL` in the SPA dev server.   |
| `docs/swagger.json is stale`                | Run `just generate` and commit the result.                                                             |
| `Swag annotations are not formatted`        | Run `just format` (`go tool swag fmt backend/`).                                                       |
| Prettier/oxlint/gofumpt failures            | Run `just format` first, then `just lint` to see the remaining real issues.                            |

## References

- [Installation](/guides/installation): every install method, including building from source.
- [Configuration](/guides/configuration): config file, env keys, databases.
- [Deployment](/guides/deployment): reverse proxy, HTTPS, systemd, production setup.
- [Theme Development](/local-development/theme-development), [Theming](/concepts/theming), [Theme Templates](/reference/theme-templates).
- [Architecture](/concepts/architecture), [API Reference](api.html).
- `CONTRIBUTING.md`: workflow, issue/PR/commit conventions, Definition of Done.
