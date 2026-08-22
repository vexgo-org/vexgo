# 配置参考

> **参考** —— 配置 VexGo 的所有方式的完整字典：命令行参数、环境变量和配置文件键。操作示例见[配置指南](/zh-cn/guides/configuration)。

## 配置来源与优先级

```
命令行参数  >  配置文件  >  环境变量  >  默认值
```

- **命令行参数** —— 直接传给二进制
- **配置文件** —— YAML，通过 `-c <path>` 加载
- **环境变量** —— 启动时读取
- **默认值** —— 内置在二进制中

高优先级来源中的显式值会覆盖低优先级来源。注意：配置文件中的显式 `false` 会覆盖环境变量中的 `true`（配置文件在环境变量之后应用）。

## 命令行参数

运行 `./vexgo --help` 获取权威列表。

| 参数        | 默认值    | 说明                                  |
| ----------- | --------- | ------------------------------------- |
| `-c <file>` | —         | YAML 配置文件路径                     |
| `--addr`    | `0.0.0.0` | 监听地址                              |
| `--port`    | `3001`    | 监听端口                              |
| `--data`    | `./data`  | 数据目录（SQLite 数据库和上传的媒体） |

## 环境变量

### 服务器

| 变量                   | 默认值    | 说明                                                                                       |
| ---------------------- | --------- | ------------------------------------------------------------------------------------------ |
| `ADDR`                 | `0.0.0.0` | 服务器监听地址                                                                             |
| `PORT`                 | `3001`    | 服务器监听端口                                                                             |
| `DATA_DIR`             | `./data`  | 数据目录路径                                                                               |
| `JWT_SECRET`           | —         | JWT 签名密钥（**生产环境必填**）                                                           |
| `LOG_LEVEL`            | `info`    | 日志级别：`debug`、`info`、`warn`、`error`                                                 |
| `BEHIND_REVERSE_PROXY` | `false`   | 位于反向代理之后时设为 `true`，以解析 `X-Forwarded-*` 请求头                               |
| `TRUSTED_PROXIES`      | —         | 逗号分隔的可信代理 IP/CIDR。仅在 `BEHIND_REVERSE_PROXY=true` 时生效。留空 = 默认私有网段。 |

### 数据库

| 变量          | 默认值    | 说明                                                                |
| ------------- | --------- | ------------------------------------------------------------------- |
| `DB_TYPE`     | `sqlite`  | 数据库类型：`sqlite`、`mysql` 或 `postgres`                         |
| `DB_HOST`     | —         | 数据库主机（mysql/postgres 必填）                                   |
| `DB_PORT`     | —         | 数据库端口（mysql/postgres 必填）                                   |
| `DB_USER`     | —         | 数据库用户名（mysql/postgres 必填）                                 |
| `DB_PASSWORD` | —         | 数据库密码（mysql/postgres 必填）                                   |
| `DB_NAME`     | —         | 数据库名（mysql/postgres 必填）                                     |
| `DB_SSL_MODE` | `disable` | Postgres SSL 模式：`disable`、`require`、`verify-ca`、`verify-full` |

### SSO / 单点登录

**通用**

| 变量                | 默认值 | 说明                                                                                          |
| ------------------- | ------ | --------------------------------------------------------------------------------------------- |
| `BASE_URL`          | —      | 实例的公网地址，如 `https://vexgo.example.com`。反向代理后必须设置，用于正确生成 OAuth 回调。 |
| `ALLOW_LOCAL_LOGIN` | `true` | 设为 `false` 禁用密码登录，强制仅 SSO 登录。                                                  |

**GitHub**

| 变量                   | 说明                           |
| ---------------------- | ------------------------------ |
| `GITHUB_CLIENT_ID`     | GitHub OAuth App Client ID     |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App Client Secret |

**Google**

| 变量                   | 说明                           |
| ---------------------- | ------------------------------ |
| `GOOGLE_CLIENT_ID`     | Google OAuth 2.0 Client ID     |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 Client Secret |

**OIDC**

| 变量                  | 默认值                 | 说明                                                              |
| --------------------- | ---------------------- | ----------------------------------------------------------------- |
| `OIDC_ENABLED`        | `false`                | 启用 OIDC 登录                                                    |
| `OIDC_ISSUER_URL`     | —                      | Issuer URL；通过 `/.well-known/openid-configuration` 自动发现端点 |
| `OIDC_CLIENT_ID`      | —                      | 提供商颁发的 Client ID                                            |
| `OIDC_CLIENT_SECRET`  | —                      | 提供商颁发的 Client Secret                                        |
| `OIDC_SCOPES`         | `openid profile email` | 空格分隔的范围；组 claim 需要时加 `groups`                        |
| `OIDC_EMAIL_CLAIM`    | `email`                | 用户邮箱的 claim 名                                               |
| `OIDC_NAME_CLAIM`     | `name`                 | 显示名称的 claim 名                                               |
| `OIDC_GROUP_CLAIM`    | `groups`               | 组成员身份的 claim 名                                             |
| `OIDC_ALLOWED_GROUPS` | —                      | 允许登录的组（逗号分隔）；留空 = 允许所有用户                     |
| `OIDC_AUTO_REDIRECT`  | `false`                | 跳过登录页，自动跳转到提供商                                      |
| `OIDC_VERIFY_EMAIL`   | `false`                | 登录前要求令牌中 `email_verified=true`                            |
| `OIDC_AUTH_URL`       | —                      | 授权端点手动覆盖（仅在无法发现时使用）                            |
| `OIDC_TOKEN_URL`      | —                      | 令牌端点手动覆盖                                                  |
| `OIDC_USERINFO_URL`   | —                      | userinfo 端点手动覆盖（可选回退）                                 |

### S3 / 对象存储

| 变量                              | 默认值  | 说明                                                              |
| --------------------------------- | ------- | ----------------------------------------------------------------- |
| `S3_ENABLED`                      | `false` | 将上传存到 S3 而非本地数据目录                                    |
| `S3_ENDPOINT`                     | —       | S3 端点 URL，如 `https://minio.example.com`。留空 = 标准 AWS S3。 |
| `S3_REGION`                       | —       | AWS 区域，如 `us-east-1`。AWS S3 必填；S3 兼容服务可填任意值。    |
| `S3_BUCKET`                       | —       | 存储桶名称                                                        |
| `S3_ACCESS_KEY`                   | —       | Access Key ID                                                     |
| `S3_SECRET_KEY`                   | —       | Secret Access Key                                                 |
| `S3_FORCE_PATH`                   | `false` | 使用路径风格 URL（MinIO 及大多数 S3 兼容服务必填）                |
| `S3_CUSTOM_DOMAIN`                | —       | 公开文件 URL 的自定义域名，如 `cdn.example.com`（例如桶前的 CDN） |
| `S3_DISABLE_BUCKET_IN_CUSTOM_URL` | `false` | 自定义域名 URL 中不包含桶名（默认包含）                           |

## 配置文件键

配置文件使用相同设置的小写 YAML 键。默认 YAML 配置文件为仓库中的 `examples/config.yml`。

### 服务器

| YAML 键                | 默认值    | 说明                                    |
| ---------------------- | --------- | --------------------------------------- |
| `addr`                 | `0.0.0.0` | 监听地址                                |
| `port`                 | `3001`    | 监听端口                                |
| `data`                 | `./data`  | 数据目录路径                            |
| `jwt_secret`           | —         | JWT 签名密钥（**生产环境必填**）        |
| `log_level`            | `info`    | `debug`、`info`、`warn`、`error`        |
| `behind_reverse_proxy` | `false`   | 为 `true` 时解析 `X-Forwarded-*` 请求头 |
| `trusted_proxies`      | `[]`      | 可信代理 IP/CIDR 列表                   |

### 数据库

| YAML 键       | 默认值    | 说明                            |
| ------------- | --------- | ------------------------------- |
| `db_type`     | `sqlite`  | `sqlite`、`mysql` 或 `postgres` |
| `db_host`     | —         | 主机（mysql/postgres 必填）     |
| `db_port`     | —         | 端口（mysql/postgres 必填）     |
| `db_user`     | —         | 用户名（mysql/postgres 必填）   |
| `db_password` | —         | 密码（mysql/postgres 必填）     |
| `db_name`     | —         | 数据库名（mysql/postgres 必填） |
| `db_ssl_mode` | `disable` | Postgres SSL 模式               |

### SSO

| YAML 键                | 默认值                 | 说明                                |
| ---------------------- | ---------------------- | ----------------------------------- |
| `github_client_id`     | —                      | GitHub OAuth Client ID              |
| `github_client_secret` | —                      | GitHub OAuth Client Secret          |
| `google_client_id`     | —                      | Google OAuth Client ID              |
| `google_client_secret` | —                      | Google OAuth Client Secret          |
| `oidc_enabled`         | `false`                | 启用 OIDC 登录                      |
| `oidc_issuer_url`      | —                      | OIDC issuer URL                     |
| `oidc_client_id`       | —                      | OIDC Client ID                      |
| `oidc_client_secret`   | —                      | OIDC Client Secret                  |
| `oidc_auth_url`        | —                      | 授权端点手动覆盖                    |
| `oidc_token_url`       | —                      | 令牌端点手动覆盖                    |
| `oidc_userinfo_url`    | —                      | userinfo 端点手动覆盖               |
| `oidc_scopes`          | `openid profile email` | OIDC 范围（空格分隔）               |
| `oidc_email_claim`     | `email`                | 邮箱 claim 名                       |
| `oidc_name_claim`      | `name`                 | 显示名称 claim 名                   |
| `oidc_group_claim`     | `groups`               | 组 claim 名                         |
| `oidc_allowed_groups`  | —                      | 允许的组（逗号分隔）；留空 = 全部   |
| `oidc_auto_redirect`   | `false`                | 登录页自动跳转到提供商              |
| `oidc_verify_email`    | `false`                | 要求 `email_verified=true`          |
| `allow_local_login`    | `true`                 | 允许密码登录（设为 `false` 仅 SSO） |

### S3

| YAML 键                           | 默认值  | 说明                      |
| --------------------------------- | ------- | ------------------------- |
| `s3_enabled`                      | `false` | 使用 S3 存储上传          |
| `s3_endpoint`                     | —       | S3 端点 URL；留空 = AWS   |
| `s3_region`                       | —       | 区域                      |
| `s3_bucket`                       | —       | 存储桶名称                |
| `s3_access_key`                   | —       | Access Key                |
| `s3_secret_key`                   | —       | Secret Key                |
| `s3_force_path`                   | `false` | 路径风格 URL（MinIO 等）  |
| `s3_custom_domain`                | —       | 公开 URL 的自定义域名     |
| `s3_disable_bucket_in_custom_url` | `false` | 自定义域名 URL 中省略桶名 |

## 环境变量 ↔ 配置文件 ↔ 命令行对照

| 设置             | 环境变量               | 配置文件键             | CLI 参数 |
| ---------------- | ---------------------- | ---------------------- | -------- |
| 监听地址         | `ADDR`                 | `addr`                 | `--addr` |
| 监听端口         | `PORT`                 | `port`                 | `--port` |
| 数据目录         | `DATA_DIR`             | `data`                 | `--data` |
| JWT 密钥         | `JWT_SECRET`           | `jwt_secret`           | —        |
| 日志级别         | `LOG_LEVEL`            | `log_level`            | —        |
| 反向代理         | `BEHIND_REVERSE_PROXY` | `behind_reverse_proxy` | —        |
| 可信代理         | `TRUSTED_PROXIES`      | `trusted_proxies`      | —        |
| 数据库类型       | `DB_TYPE`              | `db_type`              | —        |
| 数据库主机       | `DB_HOST`              | `db_host`              | —        |
| 数据库端口       | `DB_PORT`              | `db_port`              | —        |
| 数据库用户       | `DB_USER`              | `db_user`              | —        |
| 数据库密码       | `DB_PASSWORD`          | `db_password`          | —        |
| 数据库名         | `DB_NAME`              | `db_name`              | —        |
| SSL 模式         | `DB_SSL_MODE`          | `db_ssl_mode`          | —        |
| GitHub Client ID | `GITHUB_CLIENT_ID`     | `github_client_id`     | —        |
| GitHub Secret    | `GITHUB_CLIENT_SECRET` | `github_client_secret` | —        |
| Google Client ID | `GOOGLE_CLIENT_ID`     | `google_client_id`     | —        |
| Google Secret    | `GOOGLE_CLIENT_SECRET` | `google_client_secret` | —        |
| OIDC 启用        | `OIDC_ENABLED`         | `oidc_enabled`         | —        |
| OIDC Issuer      | `OIDC_ISSUER_URL`      | `oidc_issuer_url`      | —        |
| OIDC Client ID   | `OIDC_CLIENT_ID`       | `oidc_client_id`       | —        |
| OIDC Secret      | `OIDC_CLIENT_SECRET`   | `oidc_client_secret`   | —        |
| S3 启用          | `S3_ENABLED`           | `s3_enabled`           | —        |
| S3 端点          | `S3_ENDPOINT`          | `s3_endpoint`          | —        |
| S3 区域          | `S3_REGION`            | `s3_region`            | —        |
| S3 桶            | `S3_BUCKET`            | `s3_bucket`            | —        |
| S3 Access Key    | `S3_ACCESS_KEY`        | `s3_access_key`        | —        |
| S3 Secret Key    | `S3_SECRET_KEY`        | `s3_secret_key`        | —        |

> **注意：** `BASE_URL` 由 SSO 包直接从环境变量读取，而非主配置结构——生产环境请通过 `BASE_URL` 设置。
