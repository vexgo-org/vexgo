# Architecture

> This page covers how VexGo is designed: the backend layout, roles and permissions, the moderation pipeline, theming, and SSO. For instructions you can follow, see the guides.

## Overview

VexGo is a self-hosted blog CMS with two main parts:

- A Go backend (`backend/`): an HTTP API built with Gin and GORM, serving the admin panel, the public site, and the REST API. It can run against SQLite, PostgreSQL, or MySQL.
- A React frontend (`frontend/`): the admin console, a TypeScript + Vite + Tailwind CSS SPA that talks to the API. Its build output is embedded into the backend binary and served under `/admin/...`; legacy top-level URLs (such as `/login`) 301-redirect there with the query string preserved, so old bookmarks keep working. Public pages are server-side rendered from themes instead.

A theme system lets the backend server-side-render public pages with uploaded themes, so visitors don't need JavaScript to read content.

## Backend layout

The backend follows a domain-oriented layout under `backend/internal`, with a composition root for bootstrapping:

```text
backend/
  cmd/vexgo/main.go         # entry point: resolves config via cli, delegates to app package
  internal/
    api/                     # wire types shared by the REST surface (responses, error bodies)
    app/                     # composition root: wires all dependencies together
    auth/                    # registration, login, JWT, profile, password reset, email verification
    cache/                   # cache backends: in-process memory + Valkey (valkey-io/valkey-go)
    captcha/                 # sliding-puzzle captcha generation and verification
    cli/                     # cobra command line: flags, help, version, .env loading
    comment/                 # comments and AI-powered moderation
    config/                  # layered config resolution via viper, JWT, S3, SSO setup
    database/                # connection, auto-migration, seeding
    home/                    # site statistics
    mailer/                  # SMTP mail building and sending
    notification/            # in-app notifications
    middleware/              # JWT auth, role-based permissions, request logging
    model/                   # GORM data models + shared seams (Notifier, FileRemover)
    page/                    # custom pages served at /:slug
    post/                    # post CRUD, categories, tags, likes
    public/                  # embedded frontend, themes, SSR renderer, static routes
    router/                  # route registration (composes every domain)
    secrets/                 # AES-256-GCM encryption of secrets stored in the database
    settings/                # admin configuration (SMTP, AI, general, theme)
    sso/                     # GitHub / Google / OIDC login
    upload/                  # file upload (local disk or S3)
    user/                    # user management, roles, creator applications
```

### Layered architecture (per domain)

Each domain package follows a three-layer pattern:

```text
handler.go    → HTTP request parsing, response rendering (calls service)
service.go    → business logic, cross-domain orchestration (calls repository)
repository.go → persistence interface + GORM implementation (calls database)
```

- Handlers never touch GORM directly; they delegate to the service.
- Services are database-agnostic behind a `Repository` interface, so they can be tested without a live database. The domain tests run them against in-memory SQLite, and the seams are faked.
- Repositories encapsulate all SQL/GORM queries, including batch operations that prevent N+1.

### Shared seams (`model/interfaces.go`)

The `model` package holds two cross-domain seams as small interfaces; other domains declare their own where they are consumed:

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
    Delete(ctx context.Context, url string) error
}
```

The implementations are wired in `internal/app`, so no domain imports another domain's concrete type:

- `notification` implements `Notifier`; `post`, `comment`, and `user` consume it.
- `upload` implements `FileRemover`; `post`, `user`, and `auth` consume it for file cleanup.
- `captcha` implements the `CaptchaChecker` seam that `auth` declares; `settings` declares its own `SecretCipher` and `post`/`home` their own `ReadCache` the same way.
- Email is an exception: `mailer.Service` is injected as a concrete type into `auth` and `settings`. It is send-only; verification tokens, password resets, and accounts stay in the domain repositories.

### Cache backends (`internal/cache/`)

`internal/cache` is a leaf package that imports no other backend module. It provides one `Cache` interface (`Get`/`Set`/`Delete`/`GetDel`/`Incr`) with two implementations: an in-process memory backend and a Valkey (Redis-compatible) backend built on `valkey-io/valkey-go`. All keys are namespaced with a `vexgo:` prefix so the server can be shared with unrelated applications.

Following the consumer-declared seam convention, no domain imports `cache`. Instead:

- `middleware` declares `CounterStore`, the atomic increment behind the distributed fixed-window rate limiter
- `sso` declares `StateStore`, one-time OAuth state via `Set`/`GetDel`
- `post` and `home` declare `ReadCache`, the read-through decorators for the public read paths

`cache.Cache` satisfies all of these structurally, and the composition root injects the concrete backend (memory, or a valkey connection dialed and PINGed at startup). Runtime store errors fail **open** in the rate limiter (availability over abuse protection) and fail **closed** in the SSO state check (CSRF protection stays intact).

### Composition root (`internal/app/`)

The `internal/app/app.go` package is the composition root, also called the wiring layer. It:

1. Receives the configuration resolved by `cli.Execute()`: cobra parses the flags, and viper layers flags > config file > environment variables > defaults.
2. Ensures the JWT secret exists (generating a random development fallback) and applies the frontend URL default; the SSO struct is derived during config resolution.
3. Opens the database and runs migrations/seeding.
4. Builds the at-rest cipher from `settings_encryption_key` (a warning is logged when unset, meaning secrets stay in plaintext) and runs `database.MigrateSecretsAtRest` to encrypt still-plaintext secrets in place (idempotent).
5. Creates storage (local or S3).
6. Instantiates every domain's dependencies and wires them into the router.

`cmd/vexgo/main.go` is a thin entry point that calls `cli.Execute()`, then `app.New(cfg)` and `app.Run()`.

### Dependency rules

The package layout keeps the dependency graph acyclic:

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
    model/         ← data models + shared seams, imported by every domain
    config/        ← configuration parsing, imported by cli, app, database, sso, upload
    secrets/       ← AES-256-GCM cipher for secrets at rest; domains consume it
                     through their own SecretCipher interfaces, wired by app
    cache/         ← memory + Valkey backends; domains consume it through their
                     own narrow seams (CounterStore, StateStore, ReadCache)

Shared layer:
    middleware/     ← JWT auth, role permissions, request logging (imports model only)

Cross-domain edges:
    auth/          ← used by comment, post, sso (for privacy filtering)
    notification/  ← implements model.Notifier, used by comment, post, user as notification seam
    upload/        ← implements model.FileRemover, used by user, auth, post for file cleanup
    captcha/       ← implements auth.CaptchaChecker, the captcha-check seam auth declares
    public/        ← used by settings as the theme-renderer seam
    mailer/        ← injected as a concrete *mailer.Service into auth and settings (send-only)
```

### Configuration management

Configuration resolves through a priority chain:

```text
command line flags  →  config file (YAML)  →  environment variables  →  defaults
     (highest)                                                (lowest)
```

The command line is defined with **cobra** (`internal/cli`), and the sources are layered with **viper** (`internal/config`). One consequence of the ordering: an explicit `false` in the config file overrides a `true` from the environment, because environment variables only fill keys the config file leaves unset.

The `config.Config` struct holds all runtime values. There are no global variables; the JWT secret, SSO config, and frontend URL are fields on `Config`, not package-level vars.

### context.Context propagation

All service and repository methods accept `context.Context` as their first parameter. Handlers pass `c.Request.Context()` from the Gin request context, which gives:

- Request-scoped cancellation and timeouts
- Distributed tracing propagation
- GORM `WithContext()` for query cancellation

## Users, roles, and permissions

Authentication is JWT-based. Each user has exactly one role, and permissions are checked against the role in the database on every request.

| Role          | Can do                                                                         |
| ------------- | ------------------------------------------------------------------------------ |
| `super_admin` | Everything. Bypasses all permission checks. Cannot be modified by other users. |
| `admin`       | Moderate content, manage users and settings, approve creator applications      |
| `author`      | Publish posts directly                                                         |
| `contributor` | Apply for a role upgrade (creator application)                                 |
| `guest`       | Newly registered users, with limited access                                    |

Privilege checks are cumulative:

- Is admin = `admin` or `super_admin`
- Is author = `author` + admin roles
- Is contributor = `contributor` + higher roles

### Creator applications

New users register as `guest`. They can submit a creator application (with a reason) to request an upgrade. Admins review the queue and approve or reject each application; approving moves the user up a role tier.

## Content moderation

VexGo has two moderation pipelines, one for posts and one for comments. Both revolve around a `status` field:

- **Posts**: `draft` → `pending` → `published` / `rejected` (rejected posts can be resubmitted)
- **Comments**: `published`, `pending`, `rejected`

### Post moderation

When an author publishes a post, it can go straight to `published` (if the author has publishing rights) or to `pending` for admin review. Admins approve or reject it, optionally attaching a rejection reason.

### Comment moderation

Comment moderation is driven by three independent switches, configurable from the admin panel. All default to off, which publishes new comments immediately:

- Keyword filter: comments containing a blocked keyword are rejected outright. The LLM is not called.
- LLM review: the configured LLM (OpenAI-compatible API) reviews each comment against a prompt. A reject verdict rejects the comment, and an approve verdict publishes it only when manual review is off. Any LLM failure (network, timeout, non-200, non-JSON reply) holds the comment as `pending`, because the pipeline is fail-closed: a broken or fooled model can at worst send comments to the review queue, never publish junk.
- Manual review: every comment that was not published or rejected waits in the queue for an admin decision ("manual final review").

The moderation configuration (switches, prompt, keywords, model) lives in the database and is managed via the admin panel or the `/moderation` API endpoints. Installations upgrading from the single "enable AI moderation" toggle are migrated to the new switches automatically at startup.

## Theme system

Public pages are rendered server-side: the renderer in `internal/public` executes Go templates from the active theme and returns finished HTML, so visitors do not need JavaScript to read content. The embedded default theme is always available, and admins can upload additional themes as ZIP archives from the admin panel. The active theme is a database value and can be switched at runtime without a restart.

A theme is a directory of templates plus optional translations, seed pages and static assets:

```text
my-theme/
├── vexgo-theme.json   # metadata (id, name, version, ...)
├── index.html         # home page template
├── post.html          # post detail template
├── page.html          # generic custom-page template
├── user.html          # profile template
├── 404.html           # optional not-found template
├── <slug>.html        # optional dedicated template for one custom page
├── i18n/<lang>.json   # optional translation dictionaries
├── seed/<slug>.md     # optional default pages
└── assets/            # static files served at /theme-assets/*
```

Uploaded themes are extracted to `data/theme/<id>/`. All root-level templates are parsed into one set (so `{{define}}` fragments are shared across files), rendered with `html/template` auto-escaping, and Markdown bodies are rendered by goldmark in safe mode. Parsed templates are cached per theme and re-read when the theme's files change.

[Theming](/concepts/theming) explains the full pipeline: template resolution and its fallback chains, the language priority order, caching, seed pages, and the theme trust boundary. Field-level details live in the [Theme Templates reference](/reference/theme-templates).

## SSO

Login can be delegated to external identity providers:

- GitHub and Google OAuth
- Any OpenID Connect (OIDC) provider (Keycloak, Authentik, Authelia, Okta, Casdoor, ...)

SSO flows use the authorization-code grant with a popup window; the result is written to `localStorage` under `sso_callback_result` and the opener page picks it up via the `storage` event. When `allow_local_login` is `false`, password login is disabled entirely and SSO is the only way in.

The callback URLs are:

| Provider | Callback URL                                  |
| -------- | --------------------------------------------- |
| GitHub   | `https://your-domain/api/sso/github/callback` |
| Google   | `https://your-domain/api/sso/google/callback` |
| OIDC     | `https://your-domain/api/sso/oidc/callback`   |

`BASE_URL` must point to your public instance URL so these redirects are generated correctly.

## Storage

- Uploads go to the local data directory by default, or to any S3-compatible object storage (AWS S3, MinIO, Garage, ...) when S3 is enabled.
- Metadata (users, posts, comments, settings) lives in the database: SQLite by default, PostgreSQL/MySQL for production.

## Notifications

In-app notifications are stored per user. Events such as comments, likes, replies, post reviews, and role changes create notifications in the recipient's inbox, exposed through the `/notifications` API.

The notification system uses a seam interface, `model.Notifier`. Domain packages call `notifier.CreateNotification()` without importing the notification package; the composition root injects the concrete implementation at startup.

## Database

### Connection

The `database.Open()` function supports three backends:

| Backend    | Default | Production-ready   |
| ---------- | ------- | ------------------ |
| SQLite     | Yes     | For small installs |
| MySQL      | No      | Yes                |
| PostgreSQL | No      | Yes                |

The database type is determined by the `db_type` config field or `DB_TYPE` environment variable. When connecting to MySQL, the server will automatically create the database if it doesn't exist.

### Migrations and seeding

`database.AutoMigrate()` creates or updates the schema for all models. `database.Seed()` inserts default records (admin user, SMTP/general/AI/theme settings, default category) if they do not already exist.

## Request flow

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

### Performance: batch queries

For list endpoints that display per-post like and comment counts, the post domain uses batch queries instead of N+1:

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

This reduces the query count from `3 × N` to 3 queries regardless of page size.

## Testing

Each domain package has its own `_test.go` files. The test infrastructure:

- Uses in-memory SQLite (`glebarez/sqlite`) for fast, isolated database tests.
- Fakes cross-domain dependencies (`fakeNotifier`, `fakeFiles`) to keep tests decoupled from other domains.
- Each test creates a fresh database with `AutoMigrate()` and seeds only the data it needs.

To run the full test suite:

```bash
cd backend && go test ./...
```

To check coverage:

```bash
cd backend && go test -cover ./internal/post/... ./internal/user/... ./internal/comment/... ./internal/notification/...
```

## Related reading

- [Theming](/concepts/theming): how the server-side rendering pipeline and theme system work
- [Configuration Reference](/reference/configuration): every flag, variable, and config key
- [API Reference](api.html): the REST endpoints exposed by this architecture
- [Configuration Guide](/guides/configuration): practical setup recipes
