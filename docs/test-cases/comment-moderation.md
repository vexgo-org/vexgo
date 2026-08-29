# Comment Moderation — Test Cases

Test cases for the comment moderation decision pipeline (explicit manual /
keyword / LLM review modes with fail-closed fallback). Backend tests live in
`backend/internal/comment/service_test.go`, `llm_test.go`, `handler_test.go`
and `backend/internal/database/moderation_migration_test.go`.

Switch semantics ("manual final review"): keyword filter and LLM review act
only as automated rejection filters; they can never auto-approve past an
enabled manual gate. Any LLM failure results in `pending`, never publish.

## Decision pipeline (unit, `comment/service_test.go`)

| ID | Scenario | Expected |
| --- | --- | --- |
| TC-CMOD-001 | All switches off (fresh install) | Comment `published` immediately, no moderation calls, no moderation reason, notifications fire |
| TC-CMOD-002 | Manual review only | Comment `pending`, no notifications |
| TC-CMOD-003 | Keyword filter only, content hits a keyword (case-insensitive) | `rejected` with reason persisted |
| TC-CMOD-004 | Keyword filter only, no hit | `published` |
| TC-CMOD-005 | Keyword + LLM both on, keyword hit | `rejected`, **zero** HTTP requests reach the fake LLM server (short-circuit) |
| TC-CMOD-006 | Keyword + LLM on, no hit, LLM approves, manual off | `published` |
| TC-CMOD-007 | LLM only, approve verdict, manual off | `published` |
| TC-CMOD-008 | LLM only, approve verdict, manual on | `pending` (LLM can never publish past the manual gate) |
| TC-CMOD-009 | LLM reject verdict | `rejected` with the model's reason persisted |
| TC-CMOD-010 | LLM returns HTTP 500, manual off | `pending` (fail-closed) |
| TC-CMOD-011 | LLM times out (server sleeps past the client timeout), manual off | `pending` (fail-closed) |
| TC-CMOD-012 | LLM returns malformed / non-JSON body, manual off | `pending` (fail-closed) |
| TC-CMOD-013 | LLM request shape | `Authorization: Bearer <ApiKey>` header, configured `model`, and `{{content}}` substitution in the prompt body asserted on the fake server |
| TC-CMOD-014 | Notification timing | pending/rejected → no notifications; published → post author, and parent author for replies; notification failure non-fatal |
| TC-CMOD-022 | Manual review pending comment approved/rejected via `SetStatus` | Works as before (queue endpoints) |

## LLM client (unit, `comment/llm_test.go`)

| ID | Scenario | Expected |
| --- | --- | --- |
| TC-CMOD-015 | Config validation: LLM on without stored/provided ApiKey or ApiEndpoint | Rejected with 400 (`ErrLLMConfigIncomplete`) |
| TC-CMOD-016 | Config test endpoint with reachable fake LLM | `POST /api/moderation/comments/config/test` returns success with model response |
| TC-CMOD-017 | Config test endpoint with incomplete config | 400 |
| TC-CMOD-018 | Config test endpoint with unreachable endpoint | 5xx error surfaced to admin |

## Config handling (unit, `comment/service_test.go`)

| ID | Scenario | Expected |
| --- | --- | --- |
| TC-CMOD-019 | ApiKey masking on GET/PUT responses and empty-means-keep on update | Preserved from previous behavior |
| TC-CMOD-020 | ApiKey encrypted at rest when a cipher is configured | Preserved (TC-ENC-015..018 still pass) |

## Migration (unit, `database/moderation_migration_test.go`)

| ID | Scenario | Expected |
| --- | --- | --- |
| TC-CMOD-021 | Old row `Enabled=false, AutoApprove=true` (old default) | All new switches off (publish immediately) |
| TC-CMOD-021 | Old row `Enabled=false, AutoApprove=false` | `ManualReviewEnabled=true` |
| TC-CMOD-021 | Old row `Enabled=true` (any AutoApprove) | `KeywordFilterEnabled=true`, `LLMReviewEnabled=true` |
| TC-CMOD-021 | Re-run migration after a successful run | No-op (idempotent); admin switch changes made after migration survive restarts |

## Handler / integration (`comment/handler_test.go`)

| ID | Scenario | Expected |
| --- | --- | --- |
| TC-CMOD-023 | Create comment (keyword reject) → appears in `/api/moderation/comments/pending`? No — in rejected list with `moderationReason`; pending list unaffected | Moderation reason returned by moderation list API |
| TC-CMOD-024 | Config round-trip `GET`/`PUT /api/moderation/comments/config` | New switches persisted and returned, ApiKey masked |
| TC-CMOD-025 | `POST /comments` returns `requiresModeration=true` when final status is `pending` | Commenter-facing behavior unchanged |
| TC-CMOD-026 | Moderation config endpoints reject non-admin roles | 401/403 as before |

## Frontend (manual, `pnpm run build` is the type gate)

| ID | Scenario | Expected |
| --- | --- | --- |
| TC-CMOD-027 | Admin config form renders three switches (keyword / LLM / manual), keeps keyword list, LLM API section, prompt editor, test button | No stale `enabled` / auto-approve / score-threshold UI |
| TC-CMOD-028 | Moderation queue rejected tab shows the moderation reason | Reasons rendered |
| TC-CMOD-029 | Commenter submits comment that lands in pending | "pending moderation" hint shown (`requiresModeration`) |
