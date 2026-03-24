# VexGo

**English | [中文](docs/README_zh_cn.md)**

This is a blog CMS built on React, Go, Gin, JWT, and SQLite, which implements features such as user registration and article management.

## Quick Start

Select the corresponding system and architecture on the release page to download.

### Linux

```bash
./vexgo-linux-amd64
```

### Docker

```bash
sudo docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/weimm16/vexgo:latest
```

### ❄️Nix

You can try VexGo instantly without installing:

```bash
nix run github:weimm16/vexgo
```

### ❄️NixOS Flake

Add the following to your `inputs` in `flake.nix`:

```nix
# flake.nix
inputs = {
  vexgo = {
    url = "github:weimm16/vexgo";
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
# Options: "sqlite", "mysql", "postgres"
db_type: "sqlite"

# When db_type is "mysql", configure the following parameters
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

| Variable               | Default   | Description                                                                                                                                                                                                                                                  |
| ---------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ADDR`                 | `0.0.0.0` | Server listen address                                                                                                                                                                                                                                        |
| `PORT`                 | `3001`    | Server listen port                                                                                                                                                                                                                                           |
| `DATA`                 | `./data`  | Data directory path                                                                                                                                                                                                                                          |
| `JWT_SECRET`           | —         | JWT secret key (required for production)                                                                                                                                                                                                                     |
| `BEHIND_REVERSE_PROXY` | `false`   | Set to `true` if the server is behind a reverse proxy (nginx, Cloudflare, etc.). This enables proper handling of `X-Forwarded-*` headers.                                                                                                                    |
| `TRUSTED_PROXIES`      | —         | Comma-separated list of trusted proxy IPs/CIDRs. Only used when `BEHIND_REVERSE_PROXY=true`. If empty, defaults to common private networks (127.0.0.1, ::1, 192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12). Example: `TRUSTED_PROXIES="192.168.1.100, 10.0.0.1"` |

#### Database

| Variable      | Default   | Description                                     |
| ------------- | --------- | ----------------------------------------------- |
| `DB_TYPE`     | `sqlite`  | Database type: `sqlite`, `mysql`, or `postgres` |
| `DB_HOST`     | —         | Database host (required for mysql/postgres)     |
| `DB_PORT`     | —         | Database port (required for mysql/postgres)     |
| `DB_USER`     | —         | Database username (required for mysql/postgres) |
| `DB_PASSWORD` | —         | Database password (required for mysql/postgres) |
| `DB_NAME`     | —         | Database name (required for mysql/postgres)     |
| `DB_SSL_MODE` | `disable` | SSL mode for postgres                           |

#### SSO / Single Sign-On

VexGo supports GitHub, Google, and any OpenID Connect (OIDC) compatible provider (Keycloak, Authentik, Authelia, Okta, Casdoor, etc.).

**General**

| Variable            | Default | Description                                                                                                                                                            |
| ------------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `BASE_URL`          | —       | Public base URL of your instance, e.g. `https://vexgo.example.com`. Required when running behind a reverse proxy so that OAuth2 redirect URIs are generated correctly. |
| `ALLOW_LOCAL_LOGIN` | `true`  | Set to `false` to disable password login and enforce SSO-only access.                                                                                                  |

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
  ghcr.io/antipeth/vexgo:latest
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
  ghcr.io/weimm16/vexgo:latest
```

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
go run main.go -c ../examples/config-postgres.yml
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
go run main.go -c ../examples/config-mysql.yml
```

## Development

### Requirements

- Linux/MacOS
- go
- nodejs
- pnpm

### Backend Structure and Architecture

The backend is organized into the following folders with clear separation of concerns:

#### **cmd/** — Application Entry Point and Configuration Parsing

- **Files**: `server.go`
- **Purpose**: Parses command-line arguments and loads configuration from config files, environment variables, and command-line flags
- **Key Functions**:
  - `ParseFlags()`: Parses command-line arguments and config files
  - `Config` struct: Holds all configuration values
- **Import Rules**:
  - ✅ **Can import**: Standard library, external packages (gin, gorm, etc.)
  - ✅ **Can be imported by**: `main.go`, other initialization modules
  - ❌ **Cannot import**: Other backend modules (to avoid circular imports and keep it as a pure configuration parser)

#### **config/** — Configuration Initialization (Pure Setup Module)

- **Files**:
  - `jwt.go`: JWT secret initialization and validation
  - `s3.go`: S3-compatible storage configuration (AWS S3, MinIO, etc.)
  - `sso.go`: SSO provider configuration (GitHub, Google, OIDC)
- **Purpose**: Initializes and manages configuration objects. This is a pure setup module with no business logic.
- **Key Functions**:
  - `config.Init(jwtSecret)`: Initializes JWT configuration
  - `config.LoadFromConfig(cfg)`: Loads SSO configuration from config file
  - `S3Config.GetURL()`: Generates S3 object URLs
- **Import Rules**:
  - ✅ **Can import**: Standard library, external packages only (NO backend modules)
  - ✅ **Can be imported by**: `main.go`, `handler`, `middleware`, `utils`
  - ❌ **Cannot import**: `handler`, `middleware`, `model`, `utils`, `public` (maintains isolation)
  - **Reason**: `config` is a pure initialization module that must not depend on application logic to prevent circular imports

#### **handler/** — HTTP Request Handlers and API Endpoints

- **Files**:
  - `api.go`: Main API route registration
  - `auth.go`: Authentication endpoints (login, logout, password reset)
  - `register.go`: User registration endpoints
  - `post.go`: Blog post CRUD operations
  - `comment.go`: Comment management
  - `comment_moderation.go`: Comment moderation and approval
  - `post_moderation.go`: Post moderation workflows
  - `like.go`: Post/comment like functionality
  - `user_management.go`: Admin user management
  - `upload.go`: File upload handling
  - `s3.go`: S3 upload and storage integration
  - `sso.go`: OAuth2/OIDC login handlers
  - `verification.go`: Email verification endpoints
  - `home.go`: Homepage data endpoints
  - `config.go`: Configuration management endpoints
  - `db.go`: Database initialization and connection management
- **Purpose**: Implements all HTTP endpoint handlers with business logic
- **Import Rules**:
  - ✅ **Can import**: `config`, `model`, `middleware`, `utils`, standard library, external packages
  - ✅ **Can be imported by**: `main.go`, other handler files
  - ❌ **Cannot import**: `cmd`, `public`

#### **middleware/** — HTTP Middleware Functions

- **Files**:
  - `auth.go`: JWT token verification and authentication
  - `permission.go`: Role-based authorization and permission checking
- **Purpose**: Provides reusable middleware for request/response processing
- **Key Functions**:
  - `JWTAuth()`: Validates JWT tokens from Authorization header
  - `OptionalJWTAuth()`: Optional authentication (doesn't fail if no token)
  - `SetDB()`: Sets database connection for permission checking
- **Import Rules**:
  - ✅ **Can import**: `config`, `model`, standard library, external packages
  - ✅ **Can be imported by**: `main.go`, `handler`
  - ❌ **Cannot import**: `cmd`, `handler`, `utils`, `public`

#### **model/** — Data Models and Database Schemas

- **Files**:
  - `post.go`: Post, User, Tag, Like, Comment data models
  - `roles.go`: User role definitions and permissions
  - `sso_binding.go`: OAuth provider account bindings
  - `config.go`: Database configuration models (SMTPConfig, GeneralSettings, etc.)
- **Purpose**: Defines all data structures and GORM database models
- **Import Rules**:
  - ✅ **Can import**: Standard library only (no external dependencies except for GORM struct tags)
  - ✅ **Can be imported by**: `handler`, `middleware`, `utils`, `config`, other model files
  - ❌ **Cannot import**: `cmd`, `config`, `handler`, `middleware`, `public`, `utils`
  - **Reason**: Model is at the core of the dependency graph; it must not import application logic

#### **utils/** — Utility Functions and Helpers

- **Files**:
  - `mailer.go`: Email sending functionality (SMTP client)
- **Purpose**: Provides reusable utility functions for common tasks
- **Import Rules**:
  - ✅ **Can import**: `model`, `config`, standard library, external packages
  - ✅ **Can be imported by**: `handler`, `middleware`
  - ❌ **Cannot import**: `cmd`, `public`

#### **public/** — Embedded Static Resources

- **Files**:
  - `public.go`: Embedded frontend build output management
- **Purpose**: Serves the built frontend application as embedded assets
- **Functions**:
  - `GetStaticFS()`: Returns filesystem for static assets
  - `GetIndexHTML()`: Returns the main index.html file
- **Import Rules**:
  - ✅ **Can import**: Standard library only
  - ✅ **Can be imported by**: `main.go`
  - ❌ **Cannot import**: Any backend modules

### Import Conventions and Rules

The project uses Go modules with the module name `vexgo`. All imports follow the pattern `vexgo/backend/{folder}`.

**Example imports**:

```go
import (
    "vexgo/backend/config"    // From config package
    "vexgo/backend/model"     // From model package
    "vexgo/backend/handler"   // From handler package
    "vexgo/backend/middleware" // From middleware package
    "vexgo/backend/utils"     // From utils package
)
```

**Dependency Graph** (arrows show "can import"):

```
main.go
  ↓
cmd → config → (pure setup, no backend imports)
  ↓
main.go → handler → config, model, middleware, utils
        → middleware → config, model
        → config
```

**Critical Rules**:

1. `config/` package must NOT import any other backend modules (prevents circular dependencies)
2. `model/` package must NOT import application logic modules (keeps data structures clean)
3. Never import from `cmd/` outside of `main.go` (command-line parsing is initialization-only)

### Steps

```bash
git clone https://github.com/weimm16/vexgo.git
cd vexgo/frontend
pnpm install
pnpm run build
cd ../backend
go run main.go
```

Then, visit http://127.0.0.1:3001
