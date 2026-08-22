# Configuration Reference

> **Reference** — a complete dictionary of every way to configure VexGo: command-line flags, environment variables, and config-file keys. For recipes and examples, see the [Configuration Guide](/guides/configuration).

## Configuration Sources and Priority

```
command-line arguments  >  config file  >  environment variables  >  defaults
```

- **Command-line arguments** — passed to the binary directly
- **Config file** — YAML, loaded with `-c <path>`
- **Environment variables** — read at startup
- **Defaults** — built into the binary

An explicit value in a higher-priority source overrides lower sources. Note that an explicit `false` in the config file overrides a `true` from an environment variable (the file is applied after the environment).

## Command-Line Flags

Run `./vexgo --help` for the authoritative list.

| Flag        | Default   | Description                                   |
| ----------- | --------- | --------------------------------------------- |
| `-c <file>` | —         | Path to a YAML configuration file             |
| `--addr`    | `0.0.0.0` | Listen address                                |
| `--port`    | `3001`    | Listen port                                   |
| `--data`    | `./data`  | Data directory (SQLite DB and uploaded media) |

## Environment Variables

### Server

| Variable               | Default   | Description                                                                                                            |
| ---------------------- | --------- | ---------------------------------------------------------------------------------------------------------------------- |
| `ADDR`                 | `0.0.0.0` | Server listen address                                                                                                  |
| `PORT`                 | `3001`    | Server listen port                                                                                                     |
| `DATA_DIR`             | `./data`  | Data directory path                                                                                                    |
| `JWT_SECRET`           | —         | JWT secret key (**required in production**)                                                                            |
| `LOG_LEVEL`            | `info`    | Logging level: `debug`, `info`, `warn`, `error`                                                                        |
| `BEHIND_REVERSE_PROXY` | `false`   | Set to `true` when behind a reverse proxy so `X-Forwarded-*` headers are honored                                       |
| `TRUSTED_PROXIES`      | —         | Comma-separated trusted proxy IPs/CIDRs. Only used when `BEHIND_REVERSE_PROXY=true`. Empty = default private networks. |

### Database

| Variable      | Default   | Description                                                         |
| ------------- | --------- | ------------------------------------------------------------------- |
| `DB_TYPE`     | `sqlite`  | Database type: `sqlite`, `mysql`, or `postgres`                     |
| `DB_HOST`     | —         | Database host (required for mysql/postgres)                         |
| `DB_PORT`     | —         | Database port (required for mysql/postgres)                         |
| `DB_USER`     | —         | Database username (required for mysql/postgres)                     |
| `DB_PASSWORD` | —         | Database password (required for mysql/postgres)                     |
| `DB_NAME`     | —         | Database name (required for mysql/postgres)                         |
| `DB_SSL_MODE` | `disable` | Postgres SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |

### SSO / Single Sign-On

**General**

| Variable            | Default | Description                                                                                                                     |
| ------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `BASE_URL`          | —       | Public base URL of the instance, e.g. `https://vexgo.example.com`. Required behind a reverse proxy for correct OAuth redirects. |
| `ALLOW_LOCAL_LOGIN` | `true`  | Set to `false` to disable password login and enforce SSO-only access.                                                           |

**GitHub**

| Variable               | Description                    |
| ---------------------- | ------------------------------ |
| `GITHUB_CLIENT_ID`     | GitHub OAuth App Client ID     |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App Client Secret |

**Google**

| Variable               | Description                    |
| ---------------------- | ------------------------------ |
| `GOOGLE_CLIENT_ID`     | Google OAuth 2.0 Client ID     |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 Client Secret |

**OIDC**

| Variable              | Default                | Description                                                                       |
| --------------------- | ---------------------- | --------------------------------------------------------------------------------- |
| `OIDC_ENABLED`        | `false`                | Enable OIDC login                                                                 |
| `OIDC_ISSUER_URL`     | —                      | Issuer URL; endpoints are auto-discovered via `/.well-known/openid-configuration` |
| `OIDC_CLIENT_ID`      | —                      | Client ID provided by the provider                                                |
| `OIDC_CLIENT_SECRET`  | —                      | Client secret provided by the provider                                            |
| `OIDC_SCOPES`         | `openid profile email` | Space-separated scopes; add `groups` if needed for group claims                   |
| `OIDC_EMAIL_CLAIM`    | `email`                | Claim name for the user's email                                                   |
| `OIDC_NAME_CLAIM`     | `name`                 | Claim name for the user's display name                                            |
| `OIDC_GROUP_CLAIM`    | `groups`               | Claim name for group membership                                                   |
| `OIDC_ALLOWED_GROUPS` | —                      | Comma-separated groups allowed to log in; empty = allow all users                 |
| `OIDC_AUTO_REDIRECT`  | `false`                | Skip the login page and redirect to the provider automatically                    |
| `OIDC_VERIFY_EMAIL`   | `false`                | Require `email_verified=true` in the token before allowing login                  |
| `OIDC_AUTH_URL`       | —                      | Manual override for the authorization endpoint (only if discovery is unavailable) |
| `OIDC_TOKEN_URL`      | —                      | Manual override for the token endpoint                                            |
| `OIDC_USERINFO_URL`   | —                      | Manual override for the userinfo endpoint (optional fallback)                     |

### S3 / Object Storage

| Variable                          | Default | Description                                                                                    |
| --------------------------------- | ------- | ---------------------------------------------------------------------------------------------- |
| `S3_ENABLED`                      | `false` | Store uploads in S3 instead of the local data directory                                        |
| `S3_ENDPOINT`                     | —       | S3 endpoint URL, e.g. `https://minio.example.com`. Empty = standard AWS S3.                    |
| `S3_REGION`                       | —       | AWS region, e.g. `us-east-1`. Required for AWS S3; any value works for S3-compatible services. |
| `S3_BUCKET`                       | —       | Bucket name                                                                                    |
| `S3_ACCESS_KEY`                   | —       | Access key ID                                                                                  |
| `S3_SECRET_KEY`                   | —       | Secret access key                                                                              |
| `S3_FORCE_PATH`                   | `false` | Use path-style URLs (required for MinIO and most S3-compatible services)                       |
| `S3_CUSTOM_DOMAIN`                | —       | Custom domain for public file URLs, e.g. `cdn.example.com` (e.g. a CDN in front of the bucket) |
| `S3_DISABLE_BUCKET_IN_CUSTOM_URL` | `false` | Don't include the bucket name in custom-domain URLs (default: bucket is included)              |

## Config File Keys

The config file uses the same settings with lowercase YAML keys. The default YAML config file is loaded from `examples/config.yml` in the repository.

### Server

| YAML key               | Default   | Description                                 |
| ---------------------- | --------- | ------------------------------------------- |
| `addr`                 | `0.0.0.0` | Listen address                              |
| `port`                 | `3001`    | Listen port                                 |
| `data`                 | `./data`  | Data directory path                         |
| `jwt_secret`           | —         | JWT secret key (**required in production**) |
| `log_level`            | `info`    | `debug`, `info`, `warn`, `error`            |
| `behind_reverse_proxy` | `false`   | Honor `X-Forwarded-*` headers when `true`   |
| `trusted_proxies`      | `[]`      | List of trusted proxy IPs/CIDRs             |

### Database

| YAML key      | Default   | Description                                 |
| ------------- | --------- | ------------------------------------------- |
| `db_type`     | `sqlite`  | `sqlite`, `mysql`, or `postgres`            |
| `db_host`     | —         | Host (required for mysql/postgres)          |
| `db_port`     | —         | Port (required for mysql/postgres)          |
| `db_user`     | —         | Username (required for mysql/postgres)      |
| `db_password` | —         | Password (required for mysql/postgres)      |
| `db_name`     | —         | Database name (required for mysql/postgres) |
| `db_ssl_mode` | `disable` | Postgres SSL mode                           |

### SSO

| YAML key               | Default                | Description                                     |
| ---------------------- | ---------------------- | ----------------------------------------------- |
| `github_client_id`     | —                      | GitHub OAuth Client ID                          |
| `github_client_secret` | —                      | GitHub OAuth Client Secret                      |
| `google_client_id`     | —                      | Google OAuth Client ID                          |
| `google_client_secret` | —                      | Google OAuth Client Secret                      |
| `oidc_enabled`         | `false`                | Enable OIDC login                               |
| `oidc_issuer_url`      | —                      | OIDC issuer URL                                 |
| `oidc_client_id`       | —                      | OIDC Client ID                                  |
| `oidc_client_secret`   | —                      | OIDC Client Secret                              |
| `oidc_auth_url`        | —                      | Manual authorization endpoint override          |
| `oidc_token_url`       | —                      | Manual token endpoint override                  |
| `oidc_userinfo_url`    | —                      | Manual userinfo endpoint override               |
| `oidc_scopes`          | `openid profile email` | OIDC scopes (space-separated)                   |
| `oidc_email_claim`     | `email`                | Email claim name                                |
| `oidc_name_claim`      | `name`                 | Display-name claim name                         |
| `oidc_group_claim`     | `groups`               | Group claim name                                |
| `oidc_allowed_groups`  | —                      | Comma-separated allowed groups; empty = all     |
| `oidc_auto_redirect`   | `false`                | Auto-redirect to the provider on the login page |
| `oidc_verify_email`    | `false`                | Require `email_verified=true`                   |
| `allow_local_login`    | `true`                 | Allow password login (set `false` for SSO-only) |

### S3

| YAML key                          | Default | Description                         |
| --------------------------------- | ------- | ----------------------------------- |
| `s3_enabled`                      | `false` | Use S3 for uploads                  |
| `s3_endpoint`                     | —       | S3 endpoint URL; empty = AWS        |
| `s3_region`                       | —       | Region                              |
| `s3_bucket`                       | —       | Bucket name                         |
| `s3_access_key`                   | —       | Access key                          |
| `s3_secret_key`                   | —       | Secret key                          |
| `s3_force_path`                   | `false` | Path-style URLs (MinIO etc.)        |
| `s3_custom_domain`                | —       | Custom domain for public URLs       |
| `s3_disable_bucket_in_custom_url` | `false` | Omit bucket from custom-domain URLs |

## Environment ↔ Config File ↔ Flag Cross-Reference

| Setting          | Environment variable   | Config file key        | CLI flag |
| ---------------- | ---------------------- | ---------------------- | -------- |
| Listen address   | `ADDR`                 | `addr`                 | `--addr` |
| Listen port      | `PORT`                 | `port`                 | `--port` |
| Data directory   | `DATA_DIR`             | `data`                 | `--data` |
| JWT secret       | `JWT_SECRET`           | `jwt_secret`           | —        |
| Log level        | `LOG_LEVEL`            | `log_level`            | —        |
| Reverse proxy    | `BEHIND_REVERSE_PROXY` | `behind_reverse_proxy` | —        |
| Trusted proxies  | `TRUSTED_PROXIES`      | `trusted_proxies`      | —        |
| DB type          | `DB_TYPE`              | `db_type`              | —        |
| DB host          | `DB_HOST`              | `db_host`              | —        |
| DB port          | `DB_PORT`              | `db_port`              | —        |
| DB user          | `DB_USER`              | `db_user`              | —        |
| DB password      | `DB_PASSWORD`          | `db_password`          | —        |
| DB name          | `DB_NAME`              | `db_name`              | —        |
| DB SSL mode      | `DB_SSL_MODE`          | `db_ssl_mode`          | —        |
| GitHub client ID | `GITHUB_CLIENT_ID`     | `github_client_id`     | —        |
| GitHub secret    | `GITHUB_CLIENT_SECRET` | `github_client_secret` | —        |
| Google client ID | `GOOGLE_CLIENT_ID`     | `google_client_id`     | —        |
| Google secret    | `GOOGLE_CLIENT_SECRET` | `google_client_secret` | —        |
| OIDC enabled     | `OIDC_ENABLED`         | `oidc_enabled`         | —        |
| OIDC issuer      | `OIDC_ISSUER_URL`      | `oidc_issuer_url`      | —        |
| OIDC client ID   | `OIDC_CLIENT_ID`       | `oidc_client_id`       | —        |
| OIDC secret      | `OIDC_CLIENT_SECRET`   | `oidc_client_secret`   | —        |
| S3 enabled       | `S3_ENABLED`           | `s3_enabled`           | —        |
| S3 endpoint      | `S3_ENDPOINT`          | `s3_endpoint`          | —        |
| S3 region        | `S3_REGION`            | `s3_region`            | —        |
| S3 bucket        | `S3_BUCKET`            | `s3_bucket`            | —        |
| S3 access key    | `S3_ACCESS_KEY`        | `s3_access_key`        | —        |
| S3 secret key    | `S3_SECRET_KEY`        | `s3_secret_key`        | —        |

> **Note:** `base_url` (or `BASE_URL`) is read directly from the environment by the SSO package rather than the main config struct — set it via `BASE_URL` in production.
