# Architecture

> **Explanation** — this page explains how VexGo is designed: the backend layout, how roles and permissions work, the moderation pipeline, theming, and SSO. It's background knowledge — read it to understand VexGo, not to accomplish a specific task.

## Overview

VexGo is a self-hosted blog CMS with two main parts:

- **A Go backend** (`backend/`) — an HTTP API built with Gin and GORM, serving the admin panel, the public site, and the REST API. It can run against SQLite, PostgreSQL, or MySQL.
- **A React frontend** (`frontend/`) — a TypeScript + Vite + Tailwind CSS SPA that talks to the API. Its build output is embedded into the backend binary.

A **theme system** lets the backend server-side-render public pages with uploaded themes, so visitors don't need JavaScript to read content.

## Backend Layout

The backend follows a domain-oriented layout under `backend/internal` with a composition root for bootstrapping:

```text
backend/
  cmd/vexgo/main.go         # entry point: resolves config via cli, delegates to app package
  internal/
    app/                     # composition root: wires all dependencies together
    auth/                    # registration, login, JWT, profile, password reset
    cache/                   # cache backends: in-process memory + Valkey (valkey-io/valkey-go)
    cli/                     # cobra command line: flags, help, version, .env loading
    comment/                 # comments and AI-powered moderation
    config/                  # layered config resolution via viper, JWT, S3, SSO setup
    database/                # connection, auto-migration, seeding
    home/                    # site statistics
    mailer/                  # SMTP mail building and sending
    notification/            # in-app notifications
    middleware/              # JWT auth, role-based permissions, request logging
    model/                   # GORM data models + shared interfaces (Notifier, FileRemover, Mailer)
    post/                    # post CRUD, categories, tags, likes
    public/                  # embedded frontend, themes, SSR renderer, static routes
    router/                  # route registration (composes every domain)
    secrets/                 # AES-256-GCM encryption of secrets stored in the database
    settings/                # admin configuration (SMTP, AI, general, theme)
    sso/                     # GitHub / Google / OIDC login
    upload/                  # file upload (local disk or S3)
    user/                    # user management, roles, creator applications
    verification/            # email verification and sliding-puzzle captcha
```

### Layered Architecture (per domain)

Each domain package follows a consistent three-layer pattern:

```text
handler.go    → HTTP request parsing, response rendering (calls service)
service.go    → business logic, cross-domain orchestration (calls repository)
repository.go → persistence interface + GORM implementation (calls database)
```

This separation ensures that:

- **Handlers** never touch GORM directly — they delegate to the service.
- **Services** are database-agnostic behind a `Repository` interface, making them unit-testable with fakes.
- **Repositories** encapsulate all SQL/GORM queries, including batch operations for N+1 prevention.

### Shared Interfaces (`model/interfaces.go`)

Cross-domain seams are defined in the `model` package as small interfaces:

```go
// NotificationInput groups the notification fields passed to CreateNotification.
type NotificationInput struct {
    UserID        uint
    Type          NotificationType
    Title         string
    Content       string
    RelatedID     string
    RelatedType   NotificationRelatedType
    RelatedPostID *uint
}

// Notifier is the seam for creating notifications; implemented by the notification domain.
type Notifier interface {
    CreateNotification(ctx context.Context, input NotificationInput) error
}

// FileRemover deletes a stored file by its public URL; implemented by upload.Storage.
type FileRemover interface {
    Delete(url string) error
}

// Mailer is the seam for sending transactional email and managing email
// verification / password-reset tokens; implemented by the mailer domain.
type Mailer interface {
    IsEmailEnabled() (bool, error)
    GenerateVerificationToken(userID uint) (string, error)
    SendVerificationEmail(toEmail, toName, verificationLink string) error
    VerifyEmail(token string) error
    GenerateEmailChangeToken(userID uint, newEmail string) (string, error)
    SendEmailChangeEmail(toEmail, toName, newEmail, verificationLink string) error
    GeneratePasswordResetToken(userID uint) (string, error)
    SendPasswordResetEmail(toEmail, toName, resetLink string) error
    ConfirmEmailChange(token string) error
}
```

These interfaces allow domains like `post`, `comment`, and `user` to trigger notifications and file cleanup — and `auth`/`verification` to send email — without importing the concrete implementations, keeping the dependency graph acyclic.

### Cache Backends (`internal/cache/`)

`internal/cache` is a **leaf package** (it imports no other backend module) providing one `Cache` interface — `Get`/`Set`/`Delete`/`GetDel`/`Incr` — with two implementations: an in-process memory backend and a Valkey (Redis-compatible) backend built on `valkey-io/valkey-go`. All keys are namespaced with a `vexgo:` prefix so the server can be shared with unrelated applications.

Following the consumer-declared seam convention, no domain imports `cache`. Instead:

- `middleware` declares `CounterStore` — the atomic increment behind the distributed fixed-window rate limiter
- `sso` declares `StateStore` — one-time OAuth state via `Set`/`GetDel`
- `post` and `home` declare `ReadCache` — the read-through decorators for the public read paths

`cache.Cache` satisfies all of these structurally, and the composition root injects the concrete backend (memory, or a valkey connection dialed and PINGed at startup). Runtime store errors fail **open** in the rate limiter (availability over abuse protection) and fail **closed** in the SSO state check (CSRF protection stays intact).

### Composition Root (`internal/app/`)

The `internal/app/app.go` package is the composition root (also called the "wiring" layer). It:

1. Receives the configuration resolved by `cli.Execute()` — cobra parses the flags, viper layers flags > config file > environment variables > defaults.
2. Ensures the JWT secret exists (generating a random development fallback) and applies the frontend URL default; the SSO struct is derived during config resolution.
3. Opens the database and runs migrations/seeding.
4. Builds the at-rest cipher from `settings_encryption_key` (a warning is logged when unset, meaning secrets stay in plaintext) and runs `database.MigrateSecretsAtRest` to encrypt still-plaintext secrets in place (idempotent).
5. Creates storage (local or S3).
6. Instantiates every domain's dependencies and wires them into the router.

`cmd/vexgo/main.go` is a thin entry point that calls `cli.Execute()`, then `app.New(cfg)` and `app.Run()`.

### Dependency Rules

The package layout keeps the dependency graph **acyclic**:

```text
cmd/vexgo/main.go
    ├─→ internal/cli          ← cobra command tree, binds flags to viper
    │       └─→ config        ← layered resolution (flags > file > env > defaults)
    └─→ internal/app          ← composition root, imports everything
            ├─→ config         ← resolved Config type (no domain imports)
            ├─→ database       ← Open/AutoMigrate/Seed (imports config, model)
            ├─→ router         ← route registration (imports all domain handlers)
            └─→ internal/*     ← domain packages

Leaf packages (no internal imports):
    model/         ← data models + shared interfaces, imported by every domain
    config/        ← configuration parsing, imported by cli, app, auth, database, sso, upload
    secrets/       ← AES-256-GCM cipher for secrets at rest; domains consume it
                     through their own SecretCipher interfaces, wired by app

Shared layer:
    middleware/     ← JWT auth, role permissions, request logging (imports model only)

Cross-domain edges:
    auth/          ← used by comment, post, sso (for privacy filtering)
    mailer/        ← implements model.Mailer, used by auth and verification for email
    notification/  ← implements model.Notifier, used by comment, post, user as notification seam
    upload/        ← implements model.FileRemover, used by user, auth, post for file cleanup
    verification/  ← used by auth as the captcha-check seam
    public/        ← used by settings as the theme-renderer seam
```

### Configuration Management

Configuration is loaded through a three-layer priority chain:

```text
command line flags  →  config file (YAML)  →  environment variables  →  defaults
     (highest)                                                (lowest)
```

The command line is defined with **cobra** (`internal/cli`), and the sources are layered with **viper** (`internal/config`). One consequence of the ordering: an explicit `false` in the config file overrides a `true` from the environment — environment variables only fill keys the config file leaves unset.

The `config.Config` struct holds all runtime values. There are **no global variables** — the JWT secret, SSO config, and frontend URL are fields on `Config`, not package-level vars.

### context.Context Propagation

All service and repository methods accept `context.Context` as their first parameter. Handlers pass `c.Request.Context()` from the Gin request context, enabling:

- Request-scoped cancellation and timeouts
- Distributed tracing propagation
- GORM `WithContext()` for query cancellation

## Users, Roles, and Permissions

Authentication is JWT-based. Each user has exactly one role; permissions are checked against the role in the database on every request.

| Role          | Can do                                                                         |
| ------------- | ------------------------------------------------------------------------------ |
| `super_admin` | Everything. Bypasses all permission checks. Cannot be modified by other users. |
| `admin`       | Moderate content, manage users and settings, approve creator applications      |
| `author`      | Publish posts directly                                                         |
| `contributor` | Apply for a role upgrade (creator application)                                 |
| `guest`       | Newly registered users — limited access                                        |

Privilege checks are cumulative:

- **Is admin** = `admin` or `super_admin`
- **Is author** = `author` + admin roles
- **Is contributor** = `contributor` + higher roles

### Creator Applications

New users register as `guest`. They can submit a **creator application** (with a reason) to request an upgrade. Admins review the queue and approve or reject each application; approving moves the user up a role tier.

## Content Moderation

VexGo has two moderation pipelines — one for posts, one for comments. Both revolve around a `status` field:

- **Posts**: `draft` → `pending` → `published` / `rejected` (rejected posts can be resubmitted)
- **Comments**: `published`, `pending`, `rejected`

### Post moderation

When an author publishes a post, it can go straight to `published` (if the author has publishing rights) or to `pending` for admin review. Admins approve or reject it, optionally attaching a rejection reason.

### Comment moderation

Comment moderation is driven by three independent switches, configurable from the admin panel (all default to off, which publishes new comments immediately):

- **Keyword filter** — comments containing a blocked keyword are rejected outright; the LLM is not called
- **LLM review** — the configured LLM (OpenAI-compatible API) reviews each comment against a prompt. A reject verdict rejects the comment; an approve verdict is published only when manual review is off. Any LLM failure (network, timeout, non-200, non-JSON reply) holds the comment as `pending` — the pipeline is fail-closed, so a broken or fooled model can at worst send comments to the review queue, never publish junk
- **Manual review** — every comment that was not published or rejected waits in the queue for an admin decision ("manual final review")

The moderation configuration (switches, prompt, keywords, model) lives in the database and is managed via the admin panel or the `/moderation` API endpoints. Installations upgrading from the single "enable AI moderation" toggle are migrated to the new switches automatically at startup.

## Theme System

Public pages are rendered server-side. The embedded **default theme** is always available; admins can upload additional themes as ZIP archives from the admin panel.

A theme contains:

```text
theme.zip
└── theme-id/
    ├── vexgo-theme.json   # metadata (id, name, author, version, ...)
    ├── preview.png        # optional preview image
    └── dist/              # built frontend assets (index.html, JS, CSS)
```

Installed themes are extracted to `data/theme/<id>/` and served by the renderer. The active theme is stored in the database and can be switched at runtime without restarting the server.

## SSO

Login can be delegated to external identity providers:

- **GitHub** and **Google** OAuth
- Any **OpenID Connect (OIDC)** provider (Keycloak, Authentik, Authelia, Okta, Casdoor, ...)

SSO flows use the authorization-code grant with a popup window; the result is written to `localStorage` under `sso_callback_result` and the opener page picks it up via the `storage` event. When `allow_local_login` is `false`, password login is disabled entirely and SSO is the only way in.

The callback URLs are:

| Provider | Callback URL                                  |
| -------- | --------------------------------------------- |
| GitHub   | `https://your-domain/api/sso/github/callback` |
| Google   | `https://your-domain/api/sso/google/callback` |
| OIDC     | `https://your-domain/api/sso/oidc/callback`   |

`BASE_URL` must point to your public instance URL so these redirects are generated correctly.

## Storage

- **Uploads** go to the local data directory by default, or to any **S3-compatible object storage** (AWS S3, MinIO, Garage, ...) when S3 is enabled.
- **Metadata** (users, posts, comments, settings) lives in the database — SQLite by default, PostgreSQL/MySQL for production.

## Notifications

In-app notifications are stored per user. Events such as comments, likes, replies, post reviews, and role changes create notifications in the recipient's inbox, exposed through the `/notifications` API.

The notification system uses a **seam interface** (`model.Notifier`) — domain packages call `notifier.CreateNotification()` without importing the notification package. The concrete implementation is injected at startup by the composition root.

## Database

### Connection

The `database.Open()` function supports three backends:

| Backend    | Default | Production-ready   |
| ---------- | ------- | ------------------ |
| SQLite     | Yes     | For small installs |
| MySQL      | No      | Yes                |
| PostgreSQL | No      | Yes                |

The database type is determined by the `db_type` config field or `DB_TYPE` environment variable. When connecting to MySQL, the server will automatically create the database if it doesn't exist.

### Migrations and Seeding

`database.AutoMigrate()` creates or updates the schema for all models. `database.Seed()` inserts default records (admin user, SMTP/general/AI/theme settings, default category) if they don't already exist.

## Request Flow

A typical request looks like this:

```text
Browser/API client
      │  HTTP
      ▼
Gin router (internal/router)
      │
      ▼
Middleware chain: logger → optional JWT auth → role permission check
      │
      ▼
Domain handler (e.g. internal/post/handler.go)
      │  passes c.Request.Context()
      ▼
Domain service (e.g. internal/post/service.go)
      │  calls repository methods
      ▼
Repository (e.g. internal/post/repository.go)
      │  GORM queries with .WithContext(ctx)
      ▼
JSON response (or SSR-rendered HTML for theme pages)
```

The JWT middleware validates the token and sets the user in the Gin context via `middleware.CurrentUser(c)` / `middleware.CurrentUserID(c)` helpers. The permission middleware checks the database role against the endpoint's required roles. `super_admin` always passes.

### Performance: Batch Queries

For list endpoints that display per-post like/comment counts, the post domain uses **batch queries** instead of N+1:

```text
// Before (N+1): 3 queries per post
for _, post := range posts {
    repo.CountLikes(ctx, post.ID)       // query 1
    repo.CountComments(ctx, post.ID)    // query 2
    repo.FindLike(ctx, post.ID, userID) // query 3
}

// After (batch): 3 queries total
likesCounts    := repo.BatchCountLikesByPostIDs(ctx, postIDs)       // 1 query with GROUP BY
commentsCounts := repo.BatchCountCommentsByPostIDs(ctx, postIDs)    // 1 query with GROUP BY
likedPosts     := repo.BatchFindLikedPostIDs(ctx, postIDs, userID)  // 1 query with IN
```

This reduces the query count from `3 × N` to exactly **3 queries** regardless of page size.

## Testing

Each domain package has its own `_test.go` files. The test infrastructure:

- Uses **in-memory SQLite** (`glebarez/sqlite`) for fast, isolated database tests.
- Fakes cross-domain dependencies (`fakeNotifier`, `fakeFiles`) to avoid coupling tests to other domains.
- Each test creates a fresh database with `AutoMigrate()` and seeds only the data it needs.

To run the full test suite:

```bash
cd backend && go test ./...
```

To check coverage:

```bash
cd backend && go test -cover ./internal/post/... ./internal/user/... ./internal/comment/... ./internal/notification/...
```

## Related Reading

- [Configuration Reference](/reference/configuration) — every flag, variable, and config key
- [API Reference](/reference/api) — the REST endpoints exposed by this architecture
- [Configuration Guide](/guides/configuration) — practical setup recipes
