# VexGo

**English | [中文](README_zh-cn.md)**

[![Go Version](https://img.shields.io/github/go-mod/go-version/vexgo-org/vexgo)](https://go.dev/)
[![License](https://img.shields.io/github/license/vexgo-org/vexgo)](./LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/vexgo-org/vexgo/build-and-test.yml?branch=main)](https://github.com/vexgo-org/vexgo/actions)
[![Release](https://img.shields.io/github/v/release/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/releases)

## VexGo - Modern Blog CMS

VexGo is a lightweight, self-hosted blog content management system designed for developers and writers who value simplicity, performance, and control. Built with modern technologies, it provides a complete blogging platform with user management, rich content creation, and extensibility.

### ✨ Key Features

- **🖥️ Modern Web Interface**: React-based admin panel for content management
- **🚀 High Performance**: Built with Go and Gin for fast, efficient processing
- **🔐 Secure Authentication**: JWT-based user system with role-based permissions (user / admin / super_admin)
- **📝 Rich Content**: Markdown editor, categories, tags, drafts, likes, and comments
- **🛡️ Configurable Comment Moderation**: Independent manual-review, keyword-filter, and LLM-review switches with fail-closed LLM fallback
- **🖼️ Media Management**: Built-in file storage with S3-compatible support
- **🎨 Theme System**: Server-side-rendered themes, switchable and uploadable from the admin panel
- **🔔 Notifications**: In-app notification inbox for likes, comments, and other events
- **🔑 SSO**: Login with GitHub, Google, or any OpenID Connect provider
- **🌐 Self-Hosted**: Complete control over your data and deployment

### 🛠️ Technology Stack

- **Backend**: Go, Gin, GORM, SQLite/PostgreSQL/MySQL
- **Frontend**: React, TypeScript, Vite, Tailwind CSS
- **Authentication**: JWT, OAuth (GitHub, Google, OIDC)
- **Storage**: Local filesystem or S3-compatible services
- **Email**: SMTP integration

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [SSO / Single Sign-On](#sso--single-sign-on)
- [Database](#database)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Quick Start

Select the corresponding system and architecture on the release page to download.

### Linux

```bash
./vexgo-linux-amd64
```

### Docker

```bash
sudo docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/vexgo-org/vexgo:latest
```

### ❄️Nix

You can try VexGo instantly without installing:

```bash
nix run github:vexgo-org/vexgo
```

### ❄️NixOS Flake

Add the following to your `inputs` in `flake.nix`:

```nix
# flake.nix
inputs = {
  vexgo = {
    url = "github:vexgo-org/vexgo";
    inputs.nixpkgs.follows = "nixpkgs";
  };
};
```

Then import the module in your `nixosSystem` modules:

```nix
# flake.nix
outputs = { self, nixpkgs, vexgo, ... } @ inputs:
  nixpkgs.lib.nixosSystem {
    specialArgs = {
      inherit inputs;
    };
    modules = [
      inputs.vexgo.nixosModules.default
      ./vexgo.nix
    ];
  };
```

Create `vexgo.nix` with your configuration:

```nix
# vexgo.nix
{ inputs,... }: {
  nixpkgs.overlays = [inputs.vexgo.overlays.default];
  services.vexgo = {
    enable = true;
    settings = {
      addr = "0.0.0.0";
      port = 3001;
    };
  };
}
```

Then rebuild your system:

```bash
sudo nixos-rebuild switch --flake .#your-host
```

### After Installation

Then, visit http://127.0.0.1:3001

The Default super admin account: `admin@example.com`
The Default super admin password: `password`

You can change your account password on your profile page.

## Configuration

Configuration priority: command-line arguments > configuration files > environment variables > default values

### Use config file

Here is example config file:

```yaml
# Server listen address
addr: "0.0.0.0"

# Server listen port
port: 3001

# Data directory (for storing SQLite database and uploaded media files)
data: "./data"

# JWT secret key for signing tokens
# IMPORTANT: Generate a secure random string for production!
# You can generate one with: openssl rand -base64 32
jwt_secret: "your-secret-key-change-this-in-production"

# Passphrase used to encrypt secrets at rest in the database (SMTP password, AI and comment-moderation API keys) with AES-256-GCM.
# IMPORTANT: Generate a secure random string for production!
# You can generate one with: openssl rand -base64 32
# When empty, these secrets are stored in plaintext (a warning is logged at startup).
# Existing plaintext values are encrypted in place on the first start with a key set.
settings_encryption_key: ""

# Logging level: "debug", "info", "warn", "error"
log_level: "info"

# Public base URL of the instance (e.g., "https://vexgo.example.com")
# Used to build OAuth/SSO callback URLs and emailed links
# (email verification, password reset, email change).
# Required behind a reverse proxy; if empty these links fall back to the
# request Host header, which is vulnerable to host-header poisoning.
base_url: ""

# Whether the server is behind a reverse proxy (e.g., nginx, Cloudflare)
# Set to true if you're using a reverse proxy that sets X-Forwarded-* headers
behind_reverse_proxy: false

# List of trusted proxy IPs/CIDRs (comma-separated)
# Only used when behind_reverse_proxy=true
# If empty, defaults to common private networks: 127.0.0.1, ::1, 192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12
# Examples:
#   - Single proxy: ["192.168.1.100"]
#   - Multiple: ["192.168.1.100", "10.0.0.1"]
#   - CIDR notation: ["192.168.1.0/24"]
trusted_proxies: []

# ==================== Database Configuration ====================

# Database type
# Options: "sqlite", "mysql", "postgres", "mariadb"
db_type: "sqlite"

# When db_type is "mysql" or "mariadb", configure the following parameters
# db_host: "127.0.0.1"
# db_port: 3306
# db_user: "your_username"
# db_password: "your_password"
# db_name: "vexgo"

# When db_type is "postgres", configure the following parameters
# db_host: "127.0.0.1"
# db_port: 5432
# db_user: "your_username"
# db_password: "your_password"
# db_name: "vexgo"
# db_ssl_mode: "disable"  # Options: "disable", "require", "verify-ca", "verify-full"

# ==================== Content Cache & Valkey ====================

# Content cache for public read paths (post lists, post by slug, popular,
# latest, home stats). Enabled (default) reads are served through a cache:
# in-process memory unless valkey is enabled, in which case the configured
# valkey server is used. Disabled, every read goes to the database directly.
cache_enabled: false

# Enable Valkey (Redis-compatible) for the content cache above and for
# shared state (rate limiting, OAuth login state) so multiple instances can
# run behind a load balancer. Disabled (default) everything stays in memory.
valkey_enabled: false

# Valkey connection URL (required when valkey_enabled is true).
# A trailing path ("/1") or a "db" query parameter selects a logical
# database; both default to database 0.
# valkey_url: "valkey://127.0.0.1:6379"

# ==================== SSO Configuration ====================

# -------------------- GitHub OAuth --------------------
# GitHub OAuth App credentials (https://github.com/settings/developers)
# Leave empty to disable GitHub login
github_client_id: ""
github_client_secret: ""

# -------------------- Google OAuth --------------------
# Google OAuth 2.0 credentials (https://console.cloud.google.com/apis/credentials)
# Leave empty to disable Google login
google_client_id: ""
google_client_secret: ""

# -------------------- OIDC (OpenID Connect) --------------------
# Generic OIDC provider support (Keycloak, Authentik, Okta, Authelia, etc.)
# Enable OIDC login
oidc_enabled: false

# OIDC discovery URL (required when enabled)
# The server will fetch the OIDC configuration from: {issuer_url}/.well-known/openid-configuration
# Example: "https://auth.example.com/realms/myrealm"
oidc_issuer_url: ""

# OIDC client credentials (required when enabled)
oidc_client_id: ""
oidc_client_secret: ""

# Manual endpoint override (only needed when OIDC discovery is unavailable)
# If provided, these override the discovery endpoints
oidc_auth_url: "" # Authorization endpoint
oidc_token_url: "" # Token endpoint
oidc_userinfo_url: "" # UserInfo endpoint (optional fallback)

# OIDC scopes (space-separated, default: "openid profile email")
# Add extra scopes if your provider requires them, e.g., "openid profile email groups"
oidc_scopes: "openid profile email"

# OIDC claim names (defaults are standard OIDC claims)
oidc_email_claim: "email" # Claim name for user's email
oidc_name_claim: "name" # Claim name for user's display name
oidc_group_claim: "groups" # Claim name for user's groups (for access control)

# OIDC access control (optional)
# Comma-separated list of groups allowed to log in
# Leave empty to allow all authenticated users
oidc_allowed_groups: ""

# OIDC user experience options
oidc_auto_redirect: false # If true, skip login page and redirect to OIDC provider automatically
oidc_verify_email: false # If true, require email_verified=true in the ID token

# -------------------- Global Options --------------------
# Set to false to enforce SSO-only (disable password login)
allow_local_login: true

# ==================== S3 Storage Configuration ====================

# Enable S3-compatible storage for file uploads
# Uploaded media files will be stored in the configured bucket instead of the local data directory.
s3_enabled: false

# S3 endpoint URL (leave empty for standard AWS S3)
s3_endpoint: ""

# AWS region (required for AWS S3; can be any value for S3-compatible services)
s3_region: "us-east-1"

# S3 bucket name
s3_bucket: "my-bucket"

# S3 access key ID
s3_access_key: ""

# S3 secret access key
s3_secret_key: ""

# Force path-style URLs (required for MinIO, Wasabi, and some S3-compatible services)
s3_force_path: false

# Optional custom domain for public file URLs (e.g., CDN domain like "cdn.example.com")
# Leave empty to use default S3 endpoints
s3_custom_domain: ""

# Disable including bucket in custom domain URLs (default: false, meaning include bucket by default)
s3_disable_bucket_in_custom_url: false
```

Then, Run the following command:

```bash
./vexgo-linux-amd64 -c /the/path/to/config.yml
```

### Use environment variables

You can also configure the application using environment variables.

#### Server

| Variable                  | Default   | Description                                                                                                                                                                                                                                                  |
| ------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ADDR`                    | `0.0.0.0` | Server listen address                                                                                                                                                                                                                                        |
| `PORT`                    | `3001`    | Server listen port                                                                                                                                                                                                                                           |
| `DATA_DIR`                | `./data`  | Data directory path                                                                                                                                                                                                                                          |
| `JWT_SECRET`              | —         | JWT secret key (required for production)                                                                                                                                                                                                                     |
| `SETTINGS_ENCRYPTION_KEY` | —         | Passphrase used to encrypt secrets at rest in the database (SMTP password, AI and comment-moderation API keys) with AES-256-GCM. When empty, these secrets are stored in plaintext (a warning is logged at startup).                                         |
| `LOG_LEVEL`               | `info`    | Logging level: `debug`, `info`, `warn`, `error`                                                                                                                                                                                                              |
| `BASE_URL`                | —         | Public base URL of the instance, e.g. `https://vexgo.example.com`. Used to build OAuth callback URLs and emailed links (verification, password reset, email change). Required when running behind a reverse proxy.                                           |
| `BEHIND_REVERSE_PROXY`    | `false`   | Set to `true` if the server is behind a reverse proxy (nginx, Cloudflare, etc.). This enables proper handling of `X-Forwarded-*` headers.                                                                                                                    |
| `TRUSTED_PROXIES`         | —         | Comma-separated list of trusted proxy IPs/CIDRs. Only used when `BEHIND_REVERSE_PROXY=true`. If empty, defaults to common private networks (127.0.0.1, ::1, 192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12). Example: `TRUSTED_PROXIES="192.168.1.100, 10.0.0.1"` |

#### Database

| Variable      | Default   | Description                                                |
| ------------- | --------- | ---------------------------------------------------------- |
| `DB_TYPE`     | `sqlite`  | Database type: `sqlite`, `mysql`, `postgres`, or `mariadb` |
| `DB_HOST`     | —         | Database host (required for mysql/postgres)                |
| `DB_PORT`     | —         | Database port (required for mysql/postgres)                |
| `DB_USER`     | —         | Database username (required for mysql/postgres)            |
| `DB_PASSWORD` | —         | Database password (required for mysql/postgres)            |
| `DB_NAME`     | —         | Database name (required for mysql/postgres)                |
| `DB_SSL_MODE` | `disable` | SSL mode for postgres                                      |

#### SSO / Single Sign-On

VexGo supports GitHub, Google, and any OpenID Connect (OIDC) compatible provider (Keycloak, Authentik, Authelia, Okta, Casdoor, etc.).

**General**

| Variable            | Default | Description                                                           |
| ------------------- | ------- | --------------------------------------------------------------------- |
| `ALLOW_LOCAL_LOGIN` | `true`  | Set to `false` to disable password login and enforce SSO-only access. |

> **Note:** `BASE_URL` is a general server setting rather than an SSO-specific one; see [Server](#server). It builds both OAuth callback URLs and emailed links (verification, password reset, email change), so setting it is strongly recommended whenever mail delivery or SSO is used.

**GitHub**

| Variable               | Description                    |
| ---------------------- | ------------------------------ |
| `GITHUB_CLIENT_ID`     | GitHub OAuth App Client ID     |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App Client Secret |

Register your OAuth App at https://github.com/settings/developers. Set the callback URL to `https://your-domain/api/sso/github/callback`.

**Google**

| Variable               | Description                    |
| ---------------------- | ------------------------------ |
| `GOOGLE_CLIENT_ID`     | Google OAuth 2.0 Client ID     |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 Client Secret |

Create credentials at https://console.developers.google.com. Set the callback URL to `https://your-domain/api/sso/google/callback`.

**OIDC**

| Variable             | Default | Description                                                                                                                                                           |
| -------------------- | ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `OIDC_ENABLED`       | `false` | Set to `true` to enable OIDC login                                                                                                                                    |
| `OIDC_ISSUER_URL`    | —       | Issuer URL of your OIDC provider, e.g. `https://auth.example.com/realms/myrealm`. VexGo will auto-discover endpoints via `<issuer>/.well-known/openid-configuration`. |
| `OIDC_CLIENT_ID`     | —       | Client ID provided by your OIDC provider                                                                                                                              |
| `OIDC_CLIENT_SECRET` | —       | Client Secret provided by your OIDC provider                                                                                                                          |

Advanced options:

| Variable              | Default                | Description                                                                                              |
| --------------------- | ---------------------- | -------------------------------------------------------------------------------------------------------- |
| `OIDC_SCOPES`         | `openid profile email` | Space-separated scopes. Add `groups` if your provider requires it for group claims.                      |
| `OIDC_EMAIL_CLAIM`    | `email`                | Claim name for the user's email                                                                          |
| `OIDC_NAME_CLAIM`     | `name`                 | Claim name for the user's display name                                                                   |
| `OIDC_GROUP_CLAIM`    | `groups`               | Claim name for group membership                                                                          |
| `OIDC_ALLOWED_GROUPS` | —                      | Comma-separated list of groups allowed to log in, e.g. `admins,developers`. Empty = allow all users.     |
| `OIDC_AUTO_REDIRECT`  | `false`                | Automatically redirect to the OIDC provider on the login page, skipping the password form.               |
| `OIDC_VERIFY_EMAIL`   | `false`                | Require `email_verified=true` in the token before allowing login.                                        |
| `OIDC_AUTH_URL`       | —                      | Manual override for the authorization endpoint (only needed if OIDC discovery is unavailable).           |
| `OIDC_TOKEN_URL`      | —                      | Manual override for the token endpoint.                                                                  |
| `OIDC_USERINFO_URL`   | —                      | Manual override for the userinfo endpoint (optional fallback when the `id_token` lacks required claims). |

Register your OIDC client with the callback URL: `https://your-domain/api/sso/oidc/callback`.

> **Tip:** To find your issuer URL, open `<provider-base-url>/.well-known/openid-configuration` and look for the `issuer` field.

**Example: Docker with OIDC**

```bash
sudo docker run -d --name vexgo \
  -p 3001:3001 \
  -v ./data:/app/data \
  -e BASE_URL=https://vexgo.example.com \
  -e OIDC_ENABLED=true \
  -e OIDC_ISSUER_URL=https://auth.example.com/realms/myrealm \
  -e OIDC_CLIENT_ID=your-client-id \
  -e OIDC_CLIENT_SECRET=your-client-secret \
  ghcr.io/vexgo-org/vexgo:latest
```

**Example: environment variables**

```bash
export BASE_URL=https://vexgo.example.com
export OIDC_ENABLED=true
export OIDC_ISSUER_URL=https://auth.example.com/realms/myrealm
export OIDC_CLIENT_ID=your-client-id
export OIDC_CLIENT_SECRET=your-client-secret
./vexgo-linux-amd64
```

#### S3 / Object Storage

> **Note:** When `S3_ENABLED=true`, uploaded media files will be stored in the configured bucket instead of the local `data` directory.

VexGo supports any S3-compatible object storage (AWS S3, MinIO, Garage, etc.).

| Variable                          | Default | Description                                                                                                             |
| --------------------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------- |
| `S3_ENABLED`                      | `false` | Set to `true` to enable S3 storage                                                                                      |
| `S3_ENDPOINT`                     | —       | S3 endpoint URL, e.g. `https://minio.example.com`. Leave empty for standard AWS S3.                                     |
| `S3_REGION`                       | —       | AWS region, e.g. `us-east-1`. Required for AWS S3; can be any value for S3-compatible services.                         |
| `S3_BUCKET`                       | —       | Target bucket name                                                                                                      |
| `S3_ACCESS_KEY`                   | —       | Access key ID                                                                                                           |
| `S3_SECRET_KEY`                   | —       | Secret access key                                                                                                       |
| `S3_FORCE_PATH`                   | `false` | Set to `true` to use path-style URLs (required for MinIO and most S3-compatible services)                               |
| `S3_CUSTOM_DOMAIN`                | —       | Custom domain for generating public file URLs, e.g. `cdn.example.com`. Useful when using a CDN in front of your bucket. |
| `S3_DISABLE_BUCKET_IN_CUSTOM_URL` | `false` | Set to `true` to disable including bucket in custom domain URLs (default: include bucket by default)                    |

**Example: Docker with MinIO**

```bash
sudo docker run -d --name vexgo \
  -p 3001:3001 \
  -v ./data:/app/data \
  -e S3_ENABLED=true \
  -e S3_ENDPOINT=https://minio.example.com \
  -e S3_REGION=us-east-1 \
  -e S3_BUCKET=vexgo \
  -e S3_ACCESS_KEY=your-access-key \
  -e S3_SECRET_KEY=your-secret-key \
  -e S3_FORCE_PATH=true \
  -e S3_DISABLE_BUCKET_IN_CUSTOM_URL=false \
  ghcr.io/vexgo-org/vexgo:latest
```

#### Content Cache & Valkey

| Variable         | Default | Description                                                                                                                                                          |
| ---------------- | ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CACHE_ENABLED`  | `false` | Serve public read paths (post lists, post by slug, popular, latest, home stats) through the content cache. `false` = every read goes to the database.                |
| `VALKEY_ENABLED` | `false` | Store cacheable state in Valkey (Redis-compatible): the content cache (when `CACHE_ENABLED=true`) plus rate limiting and OAuth login state, shared across instances. |
| `VALKEY_URL`     | —       | Valkey connection URL, e.g. `valkey://127.0.0.1:6379` (required when `VALKEY_ENABLED=true`; `redis://` and `rediss://` for TLS are also accepted)                    |

> **Notes:** with `VALKEY_ENABLED=false` the content cache runs on in-process memory and rate limiting/OAuth state are per-process — single-instance only. Running multiple instances behind a load balancer requires `VALKEY_ENABLED=true`. With `VALKEY_ENABLED=true` the server must be reachable at startup (fail-fast) and should be kept private, with a `maxmemory` limit and `allkeys-lru` eviction configured.

## Database

### Postgres

Recommend Version: Postgres 18

To use postgres. First, you run a postgres instance.

```bash
sudo docker run -d --name postgres -e POSTGRES_PASSWORD=test -p 5432:5432 -v ./postgres:/var/lib/postgresql/data docker.io/library/postgres:18-alpine
```

Then, enter postgres shell.

```bash
psql -U postgres
postgres=# CREATE USER vexgo_user WITH PASSWORD 'password';
postgres=# CREATE DATABASE vexgo_db OWNER vexgo_user ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C' TEMPLATE template0;
```

Run backend with this command:

```bash
go run ./backend/cmd/vexgo -c examples/config-postgres.yml
```

### Mysql

Recommend Version: Mysql 8

To use mysql. First, you run a mysql instance.

```bash
sudo docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=test -v ./mysql:/var/lib/mysql docker.io/library/mysql:8
```

Then, enter mysql shell.

```bash
mysql -p
mysql> CREATE DATABASE vexgo_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
mysql> CREATE USER 'vexgo_user'@'%' IDENTIFIED BY 'password';
mysql> GRANT ALL ON vexgo_db.* TO 'vexgo_user'@'%';
mysql> FLUSH PRIVILEGES;
```

Run backend with this command:

```bash
cd backend
go run ./cmd/vexgo -c ../examples/config-mysql.yml
```

## Development

### Requirements

- Linux/macOS
- Go 1.25+
- Node.js and pnpm 10
- `just`, `gofumpt`, `golangci-lint`, `prettier`, `oxlint` (recommended; a Nix dev shell with all of them is available via `nix develop`)

### Common commands

```sh
just format            # gofumpt -w -extra . && prettier --write
just lint              # golangci-lint + prettier --check + gofumpt check + oxlint
go build -v ./...      # build the backend
go test -v ./...       # run backend tests

cd frontend
pnpm install
pnpm run dev           # frontend dev server with HMR
pnpm run build         # typecheck (tsc -b) + vite build + copy manifest
pnpm run lint          # oxlint
```

The frontend build output is written to `backend/internal/public/dist` and embedded into the backend binary, so rebuild the frontend after changing it.

### Run locally

```bash
git clone https://github.com/vexgo-org/vexgo.git
cd vexgo
cd frontend
pnpm install
pnpm run build
cd ../backend
go run ./cmd/vexgo
```

Then visit http://127.0.0.1:3001. The default super admin account is `admin@example.com` / `password` — change it on your profile page.

### Backend structure

The backend follows a domain-oriented layout under `backend/internal` with a composition root for bootstrapping:

```text
backend/
  cmd/vexgo/main.go  # entry point: resolves config via cli, delegates to app.New()
  internal/
    app/             # composition root: wires storage, DB, and every domain
    auth/            # registration, login, JWT, profile, password reset
    cli/             # cobra command line: flags, help, version, .env loading
    comment/         # comments and AI-powered moderation
    config/          # layered config resolution via viper (flags > file > env > defaults), JWT, S3, SSO setup (pure setup, no backend imports)
    database/        # connection, auto-migration, seeding
    home/            # site statistics
    mailer/          # SMTP mail building and sending
    notification/    # in-app notifications
    middleware/      # JWT auth, role-based permissions, request logging
    model/           # GORM data models + shared seams (Notifier, FileRemover, Mailer)
    post/            # post CRUD, categories, tags, likes
    public/          # embedded frontend, themes, SSR renderer, static routes
    router/          # route registration (composes every domain)
    settings/        # admin configuration (SMTP, AI, general, theme)
    sso/             # GitHub / Google / OIDC login
    upload/          # file upload (local disk or S3)
    user/            # user management, roles, creator applications
    verification/    # email verification and sliding-puzzle captcha
```

Each domain package follows a consistent three-layer pattern:

```text
handler.go    → HTTP request parsing, response rendering (calls service)
service.go    → business logic, cross-domain orchestration (calls repository)
repository.go → persistence interface + GORM implementation (calls database)
```

Handlers never touch GORM, services are database-agnostic behind a `Repository` interface (unit-testable with fakes), and repositories encapsulate all SQL/GORM queries including batch operations for N+1 prevention. `context.Context` is propagated through all three layers.

Imports use the module path `github.com/vexgo-org/vexgo/backend/internal/<package>`, for example:

```go
import (
    "github.com/vexgo-org/vexgo/backend/internal/model"
    "github.com/vexgo-org/vexgo/backend/internal/post"
    "github.com/vexgo-org/vexgo/backend/internal/router"
)
```

### Dependency facts

- **Leaf packages** — `config/` and `model/` import no other backend module. `model` holds the GORM data models plus the cross-domain seams (`Notifier`, `FileRemover`, `Mailer`); `config` is imported by `app`, `auth`, `database`, `middleware`, `sso`, and `upload`.
- **Shared layer** — `middleware/` (JWT auth, role permissions, request logging) depends only on `config` and `model`.
- **Cross-domain edges** — `auth` is used by `comment`, `post`, and `sso`; `auth` itself depends on `verification`; `settings` depends on `public` (theme management) and `mailer` (SMTP); `database` depends on `config` and `model`. Domains consume each other through the seams in `model`: `notification` implements `Notifier`, `upload` implements `FileRemover`, `mailer` implements `Mailer`. The dependency graph is acyclic.
- **Wiring** — `backend/cmd/vexgo/main.go` is the thin entry point: it parses flags and calls `app.New(cfg)` / `app.Run()`. The `internal/app` package is the composition root — it opens the database, creates storage and the `public.Renderer`, and wires every domain together by calling `router.RegisterAPIRoutes(r, router.Deps{...})` (defined in `internal/router`).

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for coding standards, testing requirements, and the issue, pull request, and commit conventions.

### License

VexGo is licensed under the [GNU Affero General Public License v3.0](LICENSE).
