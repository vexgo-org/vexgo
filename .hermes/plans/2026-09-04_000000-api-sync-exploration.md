# API Synchronization: Frontend ↔ Backend — Exploration

> **Status:** Exploration. No code changes. The `refactor/tygo-refactor` branch
> has already shipped the core decision (codegen from Go source). This document
> surveys the remaining axes of variation so we can decide what to refine,
> harden, or replace.

## Goal

Keep the JSON contract between the Go backend (Gin handlers) and the
React/TypeScript frontend (axios clients) consistent, with minimal manual
maintenance and fast feedback when shapes drift.

## Current State (as of `refactor/tygo-refactor`)

The repo already has a working pipeline. Reading the actual files confirms:

- **`backend/internal/api/*.go`** is the single source of truth for HTTP
  envelopes (response types + request DTOs). 9 files covering
  `auth`, `captcha`, `comment`, `common`, `home`, `notification`, `post`,
  `settings`, `upload`, `user`. ~530 lines total.
- **`backend/internal/model/*.go`** holds the GORM/domain models.
- **`tygo.yaml`** maps both packages → TypeScript. Configured with `union`
  enums, `time.Time` → `string /* RFC3339 */`, and explicit `model.X` type
  remappings so `api.ts` can import from `model.ts`.
- **`just api-types`** runs `tygo generate` to produce
  `frontend/src/types/generated/{api,model}.ts` (~920 lines combined).
- **`frontend/src/types/index.ts`** re-exports both files and adds
  hand-written types (`ApiResponse<T>`, `AIModel`) for shapes that have no
  Go counterpart.
- Handlers render typed envelopes instead of `gin.H{...}` maps.
- `frontend/src/lib/api.ts` already imports the generated response types
  end-to-end (e.g. `LoginResponse`, `PostsResponse`, `UploadResponse`).

### What's working well

- No hand-maintained TypeScript types for backend shapes.
- JSON tags on Go structs are the spec — same field names in both languages.
- Pointers (`*string`, `*bool`) translate cleanly to optional fields.
- The `binding:"required"` constraint is preserved as `required` semantics,
  though tygo doesn't emit them as TypeScript required-modifiers (no
  equivalent in plain TS without Zod — see Risks below).

### Known friction points (visible from a read-through, not exhaustive)

1. **`any` with `tstype` directives.** `CreatePostRequest.Category` and
   `UpdatePostRequest.Category` are declared `any` in Go so the binder
   accepts `number | string`, with `tstype:"number | string"` hinting tygo.
   This works but is a leak — the type lives in two places.
2. **No compile-time link from handler to envelope.** Nothing prevents a
   future handler from calling `c.JSON(200, gin.H{"user": ...})` again.
   The lint pipeline (`gopls check`) won't catch it.
3. **Drift detection is commit-time, not run-time.** A stale generated file
   produces no warning until someone forgets to run `just api-types` and
   ships a broken frontend.
4. **OpenAPI is not derived.** We don't have a single spec document that
   describes the whole API surface; reviewers rely on reading the Go
   structs.
5. **Request DTOs vs response types are conflated in one package.** The
   `api` package mixes `LoginRequest` and `LoginResponse`. That mirrors
   what most frameworks do, but it means a tygo regeneration touches both
   surfaces whenever either moves.
6. **Field-name case mix.** Generated TS uses snake_case (`email_verified`,
   `created_at`) which exactly matches the JSON wire format. The rest of
   the frontend uses camelCase. A naming policy would clean this up, but
   it's out of scope for this exploration.

## The Space of Solutions

Approaches ranked by how much of the problem they solve for a project of
this size.

### 1. Status quo: tygo from Go (already adopted)

```
Go struct + json tag  ──tygo──▶  TS interface
```

- **Pros:** Zero runtime cost, no extra services, deterministic, fast,
  already wired in.
- **Cons:** Drift detection is "remember to run `just api-types`", no
  runtime validation, request DTOs use `any` for polymorphic fields.
- **Hardening options that don't change the model:**
  - Add a `gopls`/CI check that greps handlers for `c.JSON(... gin.H{`
    and fails the build.
  - Add a `git diff` check in CI: fail if `backend/internal/{api,model}/`
    changed but `frontend/src/types/generated/` didn't.
  - Replace `any` + `tstype` with a real Go type (e.g. a `CategoryRef`
    struct with custom `UnmarshalJSON`) and let tygo derive it natively.
  - Wire `just api-types` into a pre-commit hook.

### 2. OpenAPI-first (spec as source of truth)

```
openapi.yaml  ──▶  Go server stubs (oapi-codegen / kin-openapi)
            ──▶  TS client + types (openapi-typescript / orval)
```

- **Pros:** Single spec document; tooling for both directions is mature;
  client SDKs can be generated for any language; `spectral` can lint
  the spec.
- **Cons:** Hand-authored OpenAPI drifts from handlers unless we generate
  the spec *from* Go (`oapi-codegen` reverse mode, or `swag`,
  `kin-openapi` reflectors). For a Gin codebase, the most common pattern
  is annotations (`swag init`) — which is its own drift problem.
- **Fit for this repo:** Moderate. We get a real spec for free, but we'd
  need to add spec annotations to every handler, or adopt a router-level
  reflector. `swag` comments are notoriously noisy.

### 3. GraphQL / gRPC

- **Pros:** Strong schema contract; generated types in both directions;
  client gets exactly the fields it asks for.
- **Cons:** Throwing out the entire REST surface. Not justified by the
  stated problem (sync drift) when tygo already solves 80% of it.
- **Fit:** None. Out of scope.

### 4. Runtime validation (Zod / go-playground/validator echo)

- **Pros:** Catches contract violations at the boundary, not at the
  type-check step. Zod schemas compose well with `tsc -b`.
- **Cons:** Doubles the type definitions (interface + schema). Has to be
  written by hand unless codegen'd from a spec.
- **Fit:** Useful *as a complement* to tygo, not as a replacement.
  Specifically: derive Zod schemas from tygo output (or vice versa) so
  both stay in lockstep.

### 5. Schema-diff CI job

A standalone test that:
1. Starts the backend, hits every `/api/*` route, records the response
   shape (and request schema from the OpenAPI/tygo source).
2. Compares the live shape against the generated TypeScript.
3. Fails CI on any diff.

- **Pros:** Catches what static analysis misses (e.g. a handler returning
  the wrong envelope type at runtime).
- **Cons:** Needs a running backend in CI; flaky if responses are
  non-deterministic (timestamps, etc.); the harness itself needs
  maintenance.
- **Fit:** Reasonable belt-and-suspenders, especially before a 1.0.

### 6. Shared IDL (Protobuf / FlatBuffers)

- **Pros:** Strongest contract guarantees. Generated code for any
  language.
- **Cons:** Overkill for a JSON REST API. Forces a content-type break and
  hurts browser/debuggability.
- **Fit:** None.

## Recommendation

**Stay with tygo (Option 1) and harden it.** The cost of adopting
OpenAPI-first is high and the benefit is incremental — we already have
type-correctness at compile time. The remaining gaps are operational, not
structural.

Concrete follow-ups, in priority order:

| # | Action | Effort | Why |
|---|---|---|---|
| 1 | CI guard: `backend/internal/{api,model}` changed ⇒ regenerated types committed | S | Prevents the most common drift. |
| 2 | CI guard: forbid `c.JSON(... gin.H{` in handlers | S | Closes the "render an ad-hoc map" loophole. |
| 3 | Replace `Category any` with a proper Go type that has a `tstype` literal already declared (or with custom `UnmarshalJSON`) | S | Removes the one hand-maintained TS type. |
| 4 | Document the workflow in `AGENTS.md` ("run `just api-types` before committing API changes") | XS | Cheap, durable. |
| 5 | Optional: add a runtime smoke test that hits each route against the generated TS types (Option 5) | M | Only if Steps 1-4 still leave us uncomfortable. |
| 6 | Optional: derive Zod schemas from generated types for boundary validation | M | Only if we start trusting runtime more than compile time. |

## Open Questions

- Do we want a public OpenAPI doc (for third-party integrators, Swagger
  UI)? If yes, deriving it from the same Go structs (via a tygo-adjacent
  tool, or `kin-openapi` reflection) is the cheapest path. If no, skip.
- Is the `any` category field a one-off, or will more polymorphic
  fields appear? If more, design a typed `JSONRaw` or `OneOf[T]` helper.
- Do we want to lint the generated TS file in CI? Currently it's marked
  DO NOT EDIT and excluded from prettier formatting — fine, but worth
  confirming intentional.

## Files Likely to Change (when we act on the recommendation)

- `tygo.yaml` — possibly tighten type mappings or add a `frontend` output
  post-processor for camelCase.
- `justfile` — add `api-types-check` recipe that runs `tygo generate` into
  a temp dir and diffs against the committed files.
- A new CI workflow file (location depends on the project's CI system —
  not currently visible in this exploration).
- `backend/internal/api/post.go` — replace the two `any` category fields
  with a real type.
- `AGENTS.md` — workflow note.
- `.golangci.yml` (if it exists) or a new lint rule to flag `gin.H{`
  in handlers.

## Verification (any future PR touching this area)

- `just format && just lint && just test && just build` all pass.
- `just api-types` is a no-op on a clean checkout.
- CI step (once added) blocks PRs that change the source without
  regenerating.

## Notes

- This exploration was read-only. Nothing under `backend/`, `frontend/`,
  or `tygo.yaml` was modified.
- The `refactor/tygo-refactor` branch already implements most of what
  Option 1 calls for; the remaining work is hardening + one or two
  cleanup items.
