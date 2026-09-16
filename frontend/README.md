# VexGo admin console (SPA)

The admin console for VexGo: React + TypeScript, built with Vite and styled with Tailwind CSS and shadcn/ui-style primitives.

It is not a standalone site. `bun run build` writes the bundle to `backend/internal/public/dist/` (gitignored), which is embedded into the Go binary and served under `/admin/...` only. Public pages are server-side rendered from themes, so the SPA never claims the site root.

## Develop

```bash
bun install

# Vite dev server with HMR; talks to the backend API directly.
# The API base comes from VITE_API_URL (see .env.example), default http://localhost:3001/api
bun run dev

bun run lint     # oxlint
```

## Build

```bash
bun run build    # tsc -b && vite build && bun scripts/copy-manifest.mjs
```

From the repository root, `just build-frontend` runs the same thing. **The backend serves the embedded build**, so frontend changes are invisible until you rebuild — the dev server does not proxy through the backend.

## Layout

| Path                     | Contents                                                                |
| ------------------------ | ----------------------------------------------------------------------- |
| `src/pages/`             | Route pages                                                             |
| `src/components/`        | Feature components; `src/components/ui/` holds the shadcn/ui primitives |
| `src/api/generated/`     | Generated API client — never edit by hand, run `just generate`          |
| `src/locales/`           | i18n strings; keep `en-US.ts` and `zh-CN.ts` in sync                    |
| `src/lib/`, `src/types/` | Shared logic and shared types                                           |

Conventions (formatting, testing, commits) live in `CONTRIBUTING.md`; the repository-wide agent notes are in `AGENTS.md`.
