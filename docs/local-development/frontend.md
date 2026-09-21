# Frontend Development

> **How-to** — run, change, and rebuild the admin SPA (React + TypeScript, Vite, Tailwind, shadcn/ui). Read the [Local Development](/local-development/quick-develop) tutorial first so `bun install` is done and the backend runs.

The SPA lives entirely under `/admin/`; public pages are server-rendered from themes, so the SPA must never claim the site root. `bun run build` writes the bundle to `backend/internal/public/dist/` (gitignored), which the Go binary embeds and serves. **The backend serves frontend changes only after a rebuild** — the Vite dev server does not proxy through the backend.

## Run the dev server

For this project, the recommended flow after changing the frontend is to rebuild the admin bundle and restart the backend.

```bash
bun run --cwd frontend build
go run backend/cmd/vexgo/main.go server
```

Alternatively, start the backend once and use the frontend dev server.

```bash
cd frontend
bun install
bun run dev
```

This starts Vite with HMR (default `http://localhost:5173`). Keep the backend running separately (`just server` in another terminal, default `http://127.0.0.1:3001`).

The API base comes from `VITE_API_URL` (see `frontend/.env.example`):

```bash
VITE_API_URL=http://localhost:3001/api
```

Resolution lives in `frontend/src/api/customAxios.ts`: `import.meta.env.VITE_API_URL || "/api"`. The shared axios instance attaches the `localStorage` token and redirects to `/admin/login` on non-auth 401s. If the dev SPA cannot reach the API, check `VITE_API_URL` first (wrong port, missing `/api` suffix, backend not running).

## Where code lives

| Path                     | Contents                                                                                            |
| ------------------------ | --------------------------------------------------------------------------------------------------- |
| `src/pages/`             | Route pages                                                                                         |
| `src/components/`        | Feature components; `src/components/ui/` holds shadcn/ui primitives                                 |
| `src/api/generated/`     | Generated API client — never edit by hand, run `just generate`                                      |
| `src/locales/`           | i18n strings; keep `en-US.ts` and `zh-CN.ts` in sync                                                |
| `src/lib/`, `src/types/` | Shared logic and shared types                                                                       |
| `vite.config.ts`         | `base: "/admin/"`, `outDir: ../backend/internal/public/dist`, `emptyOutDir: true`, `manifest: true` |

Conventions: prettier + oxlint (`frontend/.oxlintrc.json`; `typescript/no-explicit-any` is an error); `@` maps to `frontend/src/`; shared types in `src/types/`, logic in `src/lib/`; English for code and comments. Call the API via `getVexGoAPI()` from `@/api/generated/endpoints`.

## How-to: change the SPA and see it in production mode

```bash
cd frontend
bun run dev        # iterate with HMR against just server
bun run build      # tsc -b + vite build + copy theme manifest
```

From the repo root, `just build-frontend` runs the same build. Restarting the backend is unnecessary after a frontend rebuild in most cases — reload `/admin/` — but a stale `dist` (multiple same-named hashed chunks) is why `emptyOutDir: true` plus the manifest exists: never disable them.

## How-to: verify before a PR

```bash
cd frontend
bun run lint       # oxlint
bun run build      # typecheck gate (tsc -b) + production bundle
```

There is no frontend test framework: `bun run build` is the typecheck gate and behavior changes are verified in the dev server. Repository-wide gates (`just format`, `just lint`, `go test ./...`) are covered in [Common Workflow](/local-development/workflow#pass-the-gates-before-a-pr).

## Troubleshooting

| Symptom                                                   | Likely cause and fix                                                                                                                               |
| --------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| SPA shows login loop / 401s                               | Backend not running, `VITE_API_URL` wrong, or token expired. Confirm `http://localhost:3001/api` responds, fix `.env`, clear `localStorage` token. |
| Change visible in `bun run dev` but not via `just server` | Expected: the backend serves the embedded build. Run `bun run build` (or `just build-frontend`) and reload `/admin/`.                              |
| Vite build passes but backend serves old JS               | Stale files in `backend/internal/public/dist/`. Rebuild with `emptyOutDir: true` intact; do not hand-delete single chunks.                         |
| `any` / unused-import lint errors                         | `typescript/no-explicit-any` is an error. Type module boundaries explicitly and remove dead imports.                                               |
| Missing i18n string in one language                       | `en-US.ts` and `zh-CN.ts` drifted. Add the key to both.                                                                                            |

Next: [Backend Development](/local-development/backend) for the API side, [Common Workflow](/local-development/workflow) for codegen and gates.
