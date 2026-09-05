# Can we use OpenAPI? — Feasibility Analysis

> **For Hermes:** Read-only exploration. No source files were modified.
> Companion to `2026-09-04_000000-api-sync-exploration.md`.

## Direct Answer

**Yes, technically.** OpenAPI fits Gin + React without changing either
language. But for *this codebase, right now*, the cost outweighs the
benefit. The recommendation is still "stay with tygo and harden it." The
case for OpenAPI only flips if one of three things changes (see
"When OpenAPI wins" at the bottom).

## What we have today

Read from the repo:

- ~111 Gin route registrations across 14 route files
  (`backend/internal/post/routes.go` is the largest at 25 routes).
- Existing source of truth for *types*: `backend/internal/{api,model}/`.
  Already feeds tygo to produce `frontend/src/types/generated/*.ts`.
- Existing source of truth for *routes*: scattered across `routes.go`
  files per domain — there is **no single map of the URL space**.
- No OpenAPI / Swagger dependency in `go.mod`.
- No CI step regenerating types (the build workflow
  `.github/workflows/build-and-test.yml` doesn't run `tygo generate`).
- Frontend has axios + plain TS, no Zod, no `openapi-typescript`, no
  runtime validators.

## What "use OpenAPI" actually means in this repo

There are two distinct patterns, with very different cost/benefit.

### Pattern A — Hand-written `openapi.yaml` is the source of truth

```yaml
components:
  schemas:
    LoginRequest:
      type: object
      required: [email, password]
      properties:
        email: {type: string, format: email}
        password: {type: string, minLength: 6}
paths:
  /api/auth/login:
    post:
      requestBody:
        content:
          application/json:
            schema: {$ref: "#/components/schemas/LoginRequest"}
      responses:
        "200":
          content:
            application/json:
              schema: {$ref: "#/components/schemas/LoginResponse"}
```

Generated artifacts:
- **Backend:** `oapi-codegen` produces route stubs + typed request
  structs. *Handlers stay hand-written*, the boilerplate is generated.
- **Frontend:** `openapi-typescript` or `orval` produces types + (with
  `orval`) a typed axios client.

- **Pros:** Single spec document. SDK for any future client language for
  free. `spectral` for spec linting. Swagger UI as documentation.
- **Cons:** 111 routes + ~50 schemas hand-written twice (once in Go for
  handlers, once in YAML for the spec). The spec *will* drift from the
  handlers — exactly the same problem we have today, just moved to a
  different file.
- **Verdict:** Strict regression for this codebase. We replace one
  drift surface (TS types) with two (Go handlers vs YAML spec, plus the
  YAML spec itself).

### Pattern B — OpenAPI is *generated from* the Go source

The spec is derived; handlers stay authoritative. Three viable tools:

| Tool | Mechanism | Notes |
|---|---|---|
| `swag` (`swaggo/swag`) | Comments above each handler | Most popular; very noisy comments; needs `swag init` on every change. |
| `kin-openapi` + custom walker | Reflect over `*gin.Engine` routes at startup | No annotations, but you must teach it about your request/response types via struct tags + a small adapter. |
| `oapi-codegen` reverse mode | Start from hand-written spec, but… | Same as Pattern A for the spec. |

Pattern B is the only honest fit for a Gin codebase, because Gin has
no built-in schema awareness. In practice it means:

1. Adding a `// @Summary … @Param … @Success 200 {object} api.LoginResponse`
   comment block to all ~111 routes, OR
2. Writing a small reflection layer in `internal/app` that walks the
   `*gin.Engine` after routes are registered, inspects
   `*api.LoginResponse` via reflection, and emits the spec.

The second approach works but only covers *types* — paths, methods,
middleware-gated permissions, and query/path parameters all have to be
added to the reflection walker by hand. There is no real "free lunch"
for Gin like there is for `chi`+`oapi-codegen-middleware` or for code-
first frameworks (FastAPI, NestJS).

### Cost of Pattern B in this codebase

To reach parity with what tygo already gives us:

| Work | Estimate |
|---|---|
| Annotate 111 routes with operation summaries + params | 1–2 days, easy to get wrong, easy to forget |
| OR build a Gin route walker + reflect on response types | 3–5 days, fragile to middleware ordering |
| Add `oapi-codegen` / `kin-openapi` for emitting spec | 0.5 day |
| Add `openapi-typescript` or `orval` to frontend | 0.5 day |
| Replace 261 lines of hand-written axios typing in `frontend/src/lib/api.ts` with generated client | 1 day |
| Migrate `frontend/src/lib/{userApi,moderationApi}.ts` to generated clients | 0.5 day |
| Add CI step: regenerate, fail on diff | 0.5 day |
| Document, fix naming/casing, retrain muscle memory | 1 day |
| **Total** | **~5–8 working days** to get the same coverage we have now |

That buys us:
- A spec document (Swagger UI, third-party integrations).
- Slightly better request validation (OpenAPI request schemas → Go
  validation via `oapi-codegen` server stubs).
- A typed client SDK generator for *future* non-TS clients.

What it does **not** buy us, given the current shape:
- Faster drift detection (we can already add that to tygo for ~1 hour).
- Better runtime safety (Zod or similar still needed).
- Fewer files (we add the spec, the codegen output, and the generated
  client, replacing ~150 lines of hand-written TS).

## When OpenAPI actually wins

Pattern A or B becomes worth it if any of these become true:

1. **Third-party API consumers.** If you publish a public API and want
   SDKs in other languages. *Currently no evidence of this; the project
   is a self-hosted blog CMS with a single frontend.*
2. **Public documentation site.** Swagger UI / Redoc as a deliverable
   for operators who want to integrate with the CMS.
3. **Request-side validation richness.** OpenAPI 3.1 has better
   validation semantics than `gin` binding tags (oneOf/anyOf, format
   keywords, JSON Schema). Useful if request DTOs are growing
   polymorphic (`any` + `tstype` in `CreatePostRequest.Category` is a
   hint of this direction).
4. **Switching off Gin.** If the team ever moves to a router that has
   first-class schema support (Echo with `oapi-codegen`, chi, Fiber
   with `swag`), the cost of Pattern B drops dramatically.
5. **More than one frontend.** Mobile app, public widget, partner
   integration — generated typed clients start paying back.

If **none of those apply** and you just want to keep the TS types in
lockstep with the Go structs, tygo is doing that already.

## A middle path (worth considering, not committing to)

Keep tygo as the type generator. *Add* an OpenAPI spec derived from the
same Go structs (Pattern B via `kin-openapi` reflection or `swag`
annotations), and use it **only** for:

- A `/api/docs` Swagger UI route (operator convenience).
- A `spectral` lint step in CI (catches malformed docs).

Don't use the spec to drive the Go code or the TS client. The
generated TS continues to come from tygo; OpenAPI is an output, not a
source. This gets you 80% of the OpenAPI benefit at maybe 1 day of
work, with zero risk to the working pipeline.

## Open questions worth answering before any commitment

- Is there a near-term plan to expose the API to third parties?
- How painful is the `any` category field in practice — are more
  polymorphic fields coming?
- Is there appetite to add `swag` annotations to every handler, or is
  the team allergic to that style of comment noise?
- Would the team accept a "spec is derived, never hand-edited" rule, or
  is OpenAPI being hand-edited part of the appeal?

## TL;DR

- **Use OpenAPI? Not yet.** The current pipeline (tygo from Go) already
  solves the stated problem. Adding OpenAPI without a consumer beyond
  the React app is mostly cost.
- **If you want a doc surface**, do the middle path: tygo keeps
  driving TS, OpenAPI is generated for Swagger UI only.
- **If you plan to expose the API publicly or add a second client**,
  revisit and choose Pattern A (hand-written spec) or B (annotated
  reflection) at that point.

## Files referenced

- `backend/internal/post/routes.go` — example route registration (25 routes).
- `frontend/src/lib/api.ts` — 349 lines of hand-typed axios client.
- `frontend/src/types/generated/api.ts` — 596 lines of tygo output.
- `.github/workflows/build-and-test.yml` — current CI; no type-regen step.
- `go.mod` — no OpenAPI / Swagger dependency.
- `frontend/package.json` — no `openapi-typescript`, no `orval`, no Zod.
