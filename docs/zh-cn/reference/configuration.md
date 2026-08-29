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

| 参数                  | 默认值    | 说明                                  |
| --------------------- | --------- | ------------------------------------- |
| `--config, -c <file>` | —         | YAML 配置文件路径                     |
| `--addr, -a <addr>`   | `0.0.0.0` | 监听地址                              |
| `--port, -p <port>`   | `3001`    | 监听端口                              |
| `--data, -d <dir>`    | `./data`  | 数据目录（SQLite 数据库和上传的媒体） |
| `--version, -V`       | —         | 打印版本并退出                        |
| `--help, -h`          | —         | 打印帮助并退出                        |

## 环境变量

### 服务器

| 变量                      | 默认值    | 说明                                                                                                                                       |
| ------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `ADDR`                    | `0.0.0.0` | 服务器监听地址                                                                                                                             |
| `PORT`                    | `3001`    | 服务器监听端口                                                                                                                             |
| `DATA_DIR`                | `./data`  | 数据目录路径                                                                                                                               |
| `JWT_SECRET`              | —         | JWT 签名密钥（**生产环境必填**）                                                                                                           |
| `SETTINGS_ENCRYPTION_KEY` | —         | 用于在数据库中对静态敏感信息（SMTP 密码、AI 与评论审核 API 密钥）做 AES-256-GCM 加密的口令。留空时这些信息以明文存储（启动时会记录警告）。 |
| `LOG_LEVEL`               | `info`    | 日志级别：`debug`、`info`、`warn`、`error`                                                                                                 |
| `BASE_URL`                | —         | 实例公网地址，如 `https://vexgo.example.com`。用于生成 OAuth 回调与邮件链接（邮箱验证、密码重置、换绑邮箱）。反向代理后必须设置。          |
| `FRONTEND_URL`            | —         | 构建面向用户的链接时使用的前端地址。未设置时回退到 `http://localhost:5173`（Vite 开发服务器）。                                            |
| `BEHIND_REVERSE_PROXY`    | `false`   | 位于反向代理之后时设为 `true`，以解析 `X-Forwarded-*` 请求头                                                                               |
| `TRUSTED_PROXIES`         | —         | 逗号分隔的可信代理 IP/CIDR。仅在 `BEHIND_REVERSE_PROXY=true` 时生效。留空 = 默认私有网段。                                                 |

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

| 变量                | 默认值 | 说明                                         |
| ------------------- | ------ | -------------------------------------------- |
| `ALLOW_LOCAL_LOGIN` | `true` | 设为 `false` 禁用密码登录，强制仅 SSO 登录。 |

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

配置文件使用相同设置的小写 YAML 键。仓库中的示例配置文件为 `examples/config.yml`，通过 `-c examples/config.yml` 加载。

### 服务器

| YAML 键                   | 默认值    | 说明                                                                                         |
| ------------------------- | --------- | -------------------------------------------------------------------------------------------- |
| `addr`                    | `0.0.0.0` | 监听地址                                                                                     |
| `port`                    | `3001`    | 监听端口                                                                                     |
| `data_dir`                | `./data`  | 数据目录路径                                                                                 |
| `jwt_secret`              | —         | JWT 签名密钥（**生产环境必填**）                                                             |
| `settings_encryption_key` | —         | 加密静态敏感信息（SMTP 密码、AI 与评论审核 API 密钥）的口令。留空 = 明文存储并输出启动警告。 |
| `log_level`               | `info`    | `debug`、`info`、`warn`、`error`                                                             |
| `base_url`                | —         | 实例公网地址，如 `https://vexgo.example.com`                                                 |
| `frontend_url`            | —         | 面向用户链接的前端地址                                                                       |
| `behind_reverse_proxy`    | `false`   | 为 `true` 时解析 `X-Forwarded-*` 请求头                                                      |
| `trusted_proxies`         | `[]`      | 可信代理 IP/CIDR 列表                                                                        |

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

| 设置                 | 环境变量                  | 配置文件键                | CLI 参数     |
| -------------------- | ------------------------- | ------------------------- | ------------ |
| 监听地址             | `ADDR`                    | `addr`                    | `--addr, -a` |
| 监听端口             | `PORT`                    | `port`                    | `--port, -p` |
| 数据目录             | `DATA_DIR`                | `data_dir`                | `--data, -d` |
| JWT 密钥             | `JWT_SECRET`              | `jwt_secret`              | —            |
| 静态敏感信息加密口令 | `SETTINGS_ENCRYPTION_KEY` | `settings_encryption_key` | —            |
| 日志级别             | `LOG_LEVEL`               | `log_level`               | —            |
| 公网地址             | `BASE_URL`                | `base_url`                | —            |
| 前端地址             | `FRONTEND_URL`            | `frontend_url`            | —            |
| 反向代理             | `BEHIND_REVERSE_PROXY`    | `behind_reverse_proxy`    | —            |
| 可信代理             | `TRUSTED_PROXIES`         | `trusted_proxies`         | —            |
| 数据库类型           | `DB_TYPE`                 | `db_type`                 | —            |
| 数据库主机           | `DB_HOST`                 | `db_host`                 | —            |
| 数据库端口           | `DB_PORT`                 | `db_port`                 | —            |
| 数据库用户           | `DB_USER`                 | `db_user`                 | —            |
| 数据库密码           | `DB_PASSWORD`             | `db_password`             | —            |
| 数据库名             | `DB_NAME`                 | `db_name`                 | —            |
| SSL 模式             | `DB_SSL_MODE`             | `db_ssl_mode`             | —            |
| GitHub Client ID     | `GITHUB_CLIENT_ID`        | `github_client_id`        | —            |
| GitHub Secret        | `GITHUB_CLIENT_SECRET`    | `github_client_secret`    | —            |
| Google Client ID     | `GOOGLE_CLIENT_ID`        | `google_client_id`        | —            |
| Google Secret        | `GOOGLE_CLIENT_SECRET`    | `google_client_secret`    | —            |
| OIDC 启用            | `OIDC_ENABLED`            | `oidc_enabled`            | —            |
| OIDC Issuer          | `OIDC_ISSUER_URL`         | `oidc_issuer_url`         | —            |
| OIDC Client ID       | `OIDC_CLIENT_ID`          | `oidc_client_id`          | —            |
| OIDC Secret          | `OIDC_CLIENT_SECRET`      | `oidc_client_secret`      | —            |
| OIDC 自动跳转        | `OIDC_AUTO_REDIRECT`      | `oidc_auto_redirect`      | —            |
| OIDC 邮箱验证        | `OIDC_VERIFY_EMAIL`       | `oidc_verify_email`       | —            |
| S3 启用              | `S3_ENABLED`              | `s3_enabled`              | —            |
| S3 端点              | `S3_ENDPOINT`             | `s3_endpoint`             | —            |
| S3 区域              | `S3_REGION`               | `s3_region`               | —            |
| S3 桶                | `S3_BUCKET`               | `s3_bucket`               | —            |
| S3 Access Key        | `S3_ACCESS_KEY`           | `s3_access_key`           | —            |
| S3 Secret Key        | `S3_SECRET_KEY`           | `s3_secret_key`           | —            |

只有 `addr`、`port` 和 `data` 提供命令行参数，其余设置均通过环境变量或配置文件键配置。

> **注意：** `base_url`（或 `BASE_URL`）是通用服务器配置，并非 SSO 专属：它同时用于生成邮件中的链接（邮箱验证、密码重置、换绑邮箱）。未设置时这些链接会退回使用请求来源，任何能直连服务器的攻击者都可利用 Host 头注入篡改链接。

## 静态敏感信息加密

SMTP 密码以及 AI / 评论审核 API 密钥存储在数据库中。由于数据库转储是常见的备份与迁移载体，VexGo 支持使用 `SETTINGS_ENCRYPTION_KEY` 对这些敏感信息做静态加密（AES-256-GCM；加密密钥由配置的口令经 scrypt 派生）。

行为说明：

- **设置了密钥：** 保存的敏感信息会带上 `enc:v1:` 标记存储；启动时会把存量明文一次性就地加密（幂等——已加密的值不会重复处理）。读取时透明解密。
- **未设置密钥：** 敏感信息与从前一样以明文存储，启动时会记录一条显著的警告。
- **密钥错误或已轮换：** 解密失败的敏感信息按"未设置"处理，并记录一条指明该配置项的错误日志——服务器不会崩溃，但对应功能（SMTP 发信、AI 调用、AI 评论审核）需要管理员在后台重新保存。

> **注意：** 密钥丢失意味着所有已加密的敏感信息都需要在后台重新录入，请将其与数据库一起备份。无论是否加密，API 响应都会对这些敏感信息脱敏，更新时留空表示"保留现有值"。
