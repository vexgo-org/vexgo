# Configuration

> **How-to** — this guide shows you how to configure VexGo: where settings come from, how to connect a real database, enable SSO, use S3 storage, and set up email.

## How VexGo Loads Configuration

VexGo reads settings from four sources. When the same setting appears in more than one source, this priority order applies:

```
command-line arguments  >  config file  >  environment variables  >  defaults
```

| Source                 | Example                  | Used for                          |
| ---------------------- | ------------------------ | --------------------------------- |
| Command-line arguments | `./vexgo --port 8080`    | One-off overrides                 |
| Config file (`-c`)     | `./vexgo -c config.yml`  | Everything (recommended)          |
| Environment variables  | `PORT=8080 ./vexgo`      | Containers (Docker, systemd)      |
| Defaults               | `3001`, `./data`, `info` | Fallback when nothing else is set |

**CLI flags:** `-c <file>` (config file), `--addr`, `--port`, `--data`. Run `./vexgo --help` to see them all.

> **Tip:** secrets (like `JWT_SECRET` or database passwords) can go in either the config file or environment variables — pick what fits your deployment. Never commit real secrets to a repository.

## Using a Config File

Copy the [example config](https://github.com/vexgo-org/vexgo/blob/main/examples/config.yml) and adapt it:

```bash
cp examples/config.yml config.yml
```

Edit the values, then start VexGo with:

```bash
./vexgo -c config.yml
```

Example config file (abridged):

```yaml
# Server
addr: "0.0.0.0"
port: 3001
data: "./data"
jwt_secret: "your-secret-key-change-this-in-production"
log_level: "info"

# Behind a reverse proxy? Set this so X-Forwarded-* headers are honored.
behind_reverse_proxy: false
trusted_proxies: []

# Database
db_type: "sqlite" # sqlite | mysql | postgres

# SSO
github_client_id: ""
google_client_id: ""
oidc_enabled: false

# S3 storage
s3_enabled: false
s3_endpoint: ""
s3_bucket: "my-bucket"
```

For a full annotated config file, see [examples/config.yml](https://github.com/vexgo-org/vexgo/blob/main/examples/config.yml). Example files for PostgreSQL and MySQL live next to it (`examples/config-postgres.yml`, `examples/config-mysql.yml`).

> **Note:** an explicit value in the config file — including `false` — overrides an environment variable. For example, `s3_enabled: false` in the file wins over `S3_ENABLED=true` in the environment.

## Using Environment Variables

Every setting can also be provided as an environment variable. This is the natural fit for Docker and systemd deployments.

| Variable               | Default   | Description                                     |
| ---------------------- | --------- | ----------------------------------------------- |
| `ADDR`                 | `0.0.0.0` | Listen address                                  |
| `PORT`                 | `3001`    | Listen port                                     |
| `DATA_DIR`             | `./data`  | Data directory (SQLite DB and media)            |
| `JWT_SECRET`           | —         | JWT signing secret (**required in production**) |
| `LOG_LEVEL`            | `info`    | `debug`, `info`, `warn`, `error`                |
| `BEHIND_REVERSE_PROXY` | `false`   | Honor `X-Forwarded-*` headers when `true`       |
| `TRUSTED_PROXIES`      | —         | Comma-separated trusted proxy IPs/CIDRs         |

### Database

| Variable      | Default   | Description                                                         |
| ------------- | --------- | ------------------------------------------------------------------- |
| `DB_TYPE`     | `sqlite`  | `sqlite`, `mysql`, or `postgres`                                    |
| `DB_HOST`     | —         | Host (required for mysql/postgres)                                  |
| `DB_PORT`     | —         | Port (required for mysql/postgres)                                  |
| `DB_USER`     | —         | Username (required for mysql/postgres)                              |
| `DB_PASSWORD` | —         | Password (required for mysql/postgres)                              |
| `DB_NAME`     | —         | Database name (required for mysql/postgres)                         |
| `DB_SSL_MODE` | `disable` | Postgres SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |

### SSO / Single Sign-On

| Variable                                                 | Default                | Description                                                                                                                                         |
| -------------------------------------------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `BASE_URL`                                               | —                      | Public base URL of your instance, e.g. `https://vexgo.example.com`. Required behind a reverse proxy so OAuth redirect URIs are generated correctly. |
| `ALLOW_LOCAL_LOGIN`                                      | `true`                 | Set to `false` to force SSO-only login                                                                                                              |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET`              | —                      | GitHub OAuth App credentials                                                                                                                        |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`              | —                      | Google OAuth 2.0 credentials                                                                                                                        |
| `OIDC_ENABLED`                                           | `false`                | Enable generic OIDC login                                                                                                                           |
| `OIDC_ISSUER_URL`                                        | —                      | OIDC provider issuer URL (endpoints are auto-discovered via `/.well-known/openid-configuration`)                                                    |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET`                  | —                      | OIDC client credentials                                                                                                                             |
| `OIDC_SCOPES`                                            | `openid profile email` | Space-separated OAuth scopes                                                                                                                        |
| `OIDC_EMAIL_CLAIM`                                       | `email`                | Claim name for the user's email                                                                                                                     |
| `OIDC_NAME_CLAIM`                                        | `name`                 | Claim name for the display name                                                                                                                     |
| `OIDC_GROUP_CLAIM`                                       | `groups`               | Claim name for group membership                                                                                                                     |
| `OIDC_ALLOWED_GROUPS`                                    | —                      | Comma-separated groups allowed to log in; empty = allow all                                                                                         |
| `OIDC_AUTO_REDIRECT`                                     | `false`                | Skip the login page and redirect straight to the provider                                                                                           |
| `OIDC_VERIFY_EMAIL`                                      | `false`                | Require `email_verified=true` in the token                                                                                                          |
| `OIDC_AUTH_URL` / `OIDC_TOKEN_URL` / `OIDC_USERINFO_URL` | —                      | Manual endpoint overrides (only if discovery is unavailable)                                                                                        |

### S3 / Object Storage

| Variable                          | Default | Description                                                              |
| --------------------------------- | ------- | ------------------------------------------------------------------------ |
| `S3_ENABLED`                      | `false` | Store uploads in S3 instead of the local data dir                        |
| `S3_ENDPOINT`                     | —       | S3 endpoint URL; empty = standard AWS S3                                 |
| `S3_REGION`                       | —       | Region (required for AWS S3)                                             |
| `S3_BUCKET`                       | —       | Bucket name                                                              |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | —       | Credentials                                                              |
| `S3_FORCE_PATH`                   | `false` | Use path-style URLs (required for MinIO and most S3-compatible services) |
| `S3_CUSTOM_DOMAIN`                | —       | Custom domain for public file URLs (e.g. a CDN)                          |
| `S3_DISABLE_BUCKET_IN_CUSTOM_URL` | `false` | Don't include the bucket name in custom-domain URLs                      |

> When `S3_ENABLED=true`, uploaded media is stored in the configured bucket instead of the local `data` directory.

---

## Setting Up a Database

VexGo uses **SQLite out of the box** — no setup needed. For production or multi-user workloads, switch to **PostgreSQL** or **MySQL**.

### PostgreSQL (recommended for production)

Recommended version: PostgreSQL 18.

Start a PostgreSQL instance:

```bash
docker run -d --name postgres \
  -e POSTGRES_PASSWORD=test \
  -p 5432:5432 \
  -v ./postgres:/var/lib/postgresql/data \
  docker.io/library/postgres:18-alpine
```

Create the database and user:

```bash
psql -U postgres
postgres=# CREATE USER vexgo_user WITH PASSWORD 'password';
postgres=# CREATE DATABASE vexgo_db OWNER vexgo_user ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C' TEMPLATE template0;
```

Run VexGo with the PostgreSQL config:

```bash
go run ./backend/cmd/vexgo -c examples/config-postgres.yml
```

Or with environment variables:

```bash
export DB_TYPE=postgres
export DB_HOST=127.0.0.1
export DB_PORT=5432
export DB_USER=vexgo_user
export DB_PASSWORD=password
export DB_NAME=vexgo_db
./vexgo
```

### MySQL

Recommended version: MySQL 8.

```bash
docker run -d --name mysql \
  -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=test \
  -v ./mysql:/var/lib/mysql \
  docker.io/library/mysql:8
```

Create the database and user:

```bash
mysql -p
mysql> CREATE DATABASE vexgo_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
mysql> CREATE USER 'vexgo_user'@'%' IDENTIFIED BY 'password';
mysql> GRANT ALL ON vexgo_db.* TO 'vexgo_user'@'%';
mysql> FLUSH PRIVILEGES;
```

Run VexGo with the MySQL config:

```bash
cd backend
go run ./cmd/vexgo -c ../examples/config-mysql.yml
```

> On the first start, VexGo runs database migrations automatically — there's nothing to import manually.

---

## Setting Up SSO

VexGo supports login via **GitHub**, **Google**, and any **OpenID Connect (OIDC)** provider (Keycloak, Authentik, Authelia, Okta, Casdoor, etc.).

> Set `BASE_URL` to your public instance URL (e.g. `https://vexgo.example.com`) so that OAuth callback URLs are generated correctly, especially behind a reverse proxy.

### GitHub

1. Register an OAuth App at <https://github.com/settings/developers>.
2. Set the callback URL to `https://your-domain/api/sso/github/callback`.
3. Set `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` (or the `github_client_id` / `github_client_secret` config keys).

### Google

1. Create credentials at <https://console.developers.google.com>.
2. Set the callback URL to `https://your-domain/api/sso/google/callback`.
3. Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.

### OIDC (any provider)

1. Create a client in your OIDC provider with the callback URL `https://your-domain/api/sso/oidc/callback`.
2. Find your **issuer URL**: open `<provider-base-url>/.well-known/openid-configuration` and look for the `issuer` field.
3. Enable OIDC:

```yaml
oidc_enabled: true
oidc_issuer_url: "https://auth.example.com/realms/myrealm"
oidc_client_id: "your-client-id"
oidc_client_secret: "your-client-secret"
```

To restrict login to specific groups, set `oidc_allowed_groups` (e.g. `admins,developers`) and make sure the group claim is included in the token (add `groups` to `oidc_scopes` if needed).

### Enforce SSO-Only

Set `allow_local_login: false` to disable password login entirely — users can only sign in through SSO providers.

---

## Setting Up Email (SMTP)

SMTP is configured **at runtime from the admin panel**, not in the config file:

1. Log in as an admin and open **Settings → SMTP**.
2. Enter your SMTP host, port, credentials, and the `from` address/name.
3. Click **Send test email** to verify the setup.

Email is used for:

- Email verification after registration
- Password reset links
- Email change confirmation

---

## Enabling S3 Storage

S3 works with any S3-compatible service — AWS S3, MinIO, Garage, etc.

### Example: Docker with MinIO

```bash
docker run -d --name vexgo \
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

> **MinIO/Wasabi:** set `S3_FORCE_PATH=true` — most S3-compatible services require path-style URLs.

## What's Configurable at Runtime?

Some settings are managed from the **admin panel** and stored in the database (no restart needed):

- **General settings** — site name, description, registration toggle, captcha, guest viewing, items per page
- **Comment moderation** — enable/disable, prompt, block keywords, score thresholds
- **Active theme** — switch between installed themes
- **SMTP** — see above

See the [API Reference](/reference/api) for the corresponding endpoints, and the [Configuration Reference](/reference/configuration) for the complete list of flags, variables, and config keys.
