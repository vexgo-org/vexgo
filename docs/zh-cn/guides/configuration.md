# 配置

> **操作指南** —— 本指南介绍如何配置 VexGo：配置来源与优先级、连接真实数据库、启用 SSO、使用 S3 存储和设置邮件。

## VexGo 如何加载配置

VexGo 从四个来源读取配置。当同一设置出现在多个来源时，按以下优先级生效：

```
命令行参数  >  配置文件  >  环境变量  >  默认值
```

| 来源             | 示例                     | 适用场景                |
| ---------------- | ------------------------ | ----------------------- |
| 命令行参数       | `./vexgo --port 8080`    | 一次性覆盖              |
| 配置文件（`-c`） | `./vexgo -c config.yml`  | 所有配置（推荐）        |
| 环境变量         | `PORT=8080 ./vexgo`      | 容器（Docker、systemd） |
| 默认值           | `3001`、`./data`、`info` | 未设置任何值时的兜底    |

**CLI 参数：** `--config, -c <file>`、`--addr, -a`、`--port, -p`、`--data, -d`、`--version, -V`、`--help, -h`。运行 `./vexgo --help` 查看完整列表。

> **提示：** 密钥（如 `JWT_SECRET` 或数据库密码）既可在配置文件中，也可用环境变量提供——选择符合你部署方式的方案。切勿把真实密钥提交到仓库。

## 使用配置文件

复制[示例配置](https://github.com/vexgo-org/vexgo/blob/main/examples/config.yml)并进行修改：

```bash
cp examples/config.yml config.yml
```

编辑其中的值，然后启动 VexGo：

```bash
./vexgo -c config.yml
```

示例配置文件（节选）：

```yaml
# 服务器
addr: "0.0.0.0"
port: 3001
data_dir: "./data"
jwt_secret: "your-secret-key-change-this-in-production"
settings_encryption_key: "" # 对 SMTP 密码及 AI/评论审核 API 密钥做静态加密（留空 = 明文存储）
log_level: "info"

# 位于反向代理之后？设为 true 以正确解析 X-Forwarded-* 请求头
behind_reverse_proxy: false
trusted_proxies: []

# 数据库
db_type: "sqlite" # sqlite | mysql | postgres | mariadb

# SSO
github_client_id: ""
google_client_id: ""
oidc_enabled: false

# S3 存储
s3_enabled: false
s3_endpoint: ""
s3_bucket: "my-bucket"
```

完整注释版配置文件见 [examples/config.yml](https://github.com/vexgo-org/vexgo/blob/main/examples/config.yml)。PostgreSQL 和 MySQL 的示例文件位于同目录（`examples/config-postgres.yml`、`examples/config-mysql.yml`）。

> **注意：** 配置文件中的显式值——包括 `false`——会覆盖环境变量。例如，文件中的 `s3_enabled: false` 会优先于环境中的 `S3_ENABLED=true`。

## 使用环境变量

每个设置都可以通过环境变量提供，这在 Docker 和 systemd 部署中很自然。

| 变量                      | 默认值    | 说明                                                                                         |
| ------------------------- | --------- | -------------------------------------------------------------------------------------------- |
| `ADDR`                    | `0.0.0.0` | 监听地址                                                                                     |
| `PORT`                    | `3001`    | 监听端口                                                                                     |
| `DATA_DIR`                | `./data`  | 数据目录（SQLite 数据库和媒体文件）                                                          |
| `JWT_SECRET`              | —         | JWT 签名密钥（**生产环境必填**）                                                             |
| `SETTINGS_ENCRYPTION_KEY` | —         | 加密静态敏感信息（SMTP 密码、AI 与评论审核 API 密钥）的口令。留空 = 明文存储并输出启动警告。 |
| `LOG_LEVEL`               | `info`    | `debug`、`info`、`warn`、`error`                                                             |
| `BEHIND_REVERSE_PROXY`    | `false`   | 为 `true` 时解析 `X-Forwarded-*` 请求头                                                      |
| `TRUSTED_PROXIES`         | —         | 逗号分隔的可信代理 IP/CIDR                                                                   |

### 数据库

| 变量          | 默认值    | 说明                                                                |
| ------------- | --------- | ------------------------------------------------------------------- |
| `DB_TYPE`     | `sqlite`  | `sqlite`、`mysql`、`postgres` 或 `mariadb`                          |
| `DB_HOST`     | —         | 主机（mysql/postgres 必填）                                         |
| `DB_PORT`     | —         | 端口（mysql/postgres 必填）                                         |
| `DB_USER`     | —         | 用户名（mysql/postgres 必填）                                       |
| `DB_PASSWORD` | —         | 密码（mysql/postgres 必填）                                         |
| `DB_NAME`     | —         | 数据库名（mysql/postgres 必填）                                     |
| `DB_SSL_MODE` | `disable` | Postgres SSL 模式：`disable`、`require`、`verify-ca`、`verify-full` |

### SSO / 单点登录

| 变量                                                     | 默认值                 | 说明                                                                                                |
| -------------------------------------------------------- | ---------------------- | --------------------------------------------------------------------------------------------------- |
| `BASE_URL`                                               | —                      | 实例的公网地址，如 `https://vexgo.example.com`。位于反向代理后时需要，以便正确生成 OAuth 回调地址。 |
| `ALLOW_LOCAL_LOGIN`                                      | `true`                 | 设为 `false` 强制仅使用 SSO 登录                                                                    |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET`              | —                      | GitHub OAuth App 凭据                                                                               |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`              | —                      | Google OAuth 2.0 凭据                                                                               |
| `OIDC_ENABLED`                                           | `false`                | 启用通用 OIDC 登录                                                                                  |
| `OIDC_ISSUER_URL`                                        | —                      | OIDC 提供商 issuer URL（通过 `/.well-known/openid-configuration` 自动发现端点）                     |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET`                  | —                      | OIDC 客户端凭据                                                                                     |
| `OIDC_SCOPES`                                            | `openid profile email` | 空格分隔的 OAuth 范围                                                                               |
| `OIDC_EMAIL_CLAIM`                                       | `email`                | 用户邮箱的 claim 名                                                                                 |
| `OIDC_NAME_CLAIM`                                        | `name`                 | 显示名称的 claim 名                                                                                 |
| `OIDC_GROUP_CLAIM`                                       | `groups`               | 组成员身份的 claim 名                                                                               |
| `OIDC_ALLOWED_GROUPS`                                    | —                      | 允许登录的组（逗号分隔）；留空 = 允许所有人                                                         |
| `OIDC_AUTO_REDIRECT`                                     | `false`                | 跳过登录页，直接跳转到提供商                                                                        |
| `OIDC_VERIFY_EMAIL`                                      | `false`                | 要求令牌中 `email_verified=true`                                                                    |
| `OIDC_AUTH_URL` / `OIDC_TOKEN_URL` / `OIDC_USERINFO_URL` | —                      | 手动端点覆盖（仅在无法发现时使用）                                                                  |

### S3 / 对象存储

| 变量                              | 默认值  | 说明                                               |
| --------------------------------- | ------- | -------------------------------------------------- |
| `S3_ENABLED`                      | `false` | 将上传文件存到 S3 而非本地数据目录                 |
| `S3_ENDPOINT`                     | —       | S3 端点 URL；留空 = 标准 AWS S3                    |
| `S3_REGION`                       | —       | 区域（AWS S3 必填）                                |
| `S3_BUCKET`                       | —       | 存储桶名称                                         |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | —       | 访问凭据                                           |
| `S3_FORCE_PATH`                   | `false` | 使用路径风格 URL（MinIO 及大多数 S3 兼容服务必填） |
| `S3_CUSTOM_DOMAIN`                | —       | 公开文件 URL 的自定义域名（如 CDN）                |
| `S3_DISABLE_BUCKET_IN_CUSTOM_URL` | `false` | 自定义域名 URL 中不包含桶名                        |

> 当 `S3_ENABLED=true` 时，上传的媒体文件存储到配置的桶中，而非本地 `data` 目录。

---

## 配置数据库

VexGo **开箱即用 SQLite**——无需任何设置。生产环境或多用户场景下，建议切换为 **PostgreSQL** 或 **MySQL**。

### PostgreSQL（生产环境推荐）

推荐版本：PostgreSQL 18。

启动 PostgreSQL 实例：

```bash
docker run -d --name postgres \
  -e POSTGRES_PASSWORD=test \
  -p 5432:5432 \
  -v ./postgres:/var/lib/postgresql/data \
  docker.io/library/postgres:18-alpine
```

创建数据库和用户：

```bash
psql -U postgres
postgres=# CREATE USER vexgo_user WITH PASSWORD 'password';
postgres=# CREATE DATABASE vexgo_db OWNER vexgo_user ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C' TEMPLATE template0;
```

使用 PostgreSQL 配置运行 VexGo：

```bash
go run ./backend/cmd/vexgo -c examples/config-postgres.yml
```

或使用环境变量：

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

推荐版本：MySQL 8。

```bash
docker run -d --name mysql \
  -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=test \
  -v ./mysql:/var/lib/mysql \
  docker.io/library/mysql:8
```

创建数据库和用户：

```bash
mysql -p
mysql> CREATE DATABASE vexgo_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
mysql> CREATE USER 'vexgo_user'@'%' IDENTIFIED BY 'password';
mysql> GRANT ALL ON vexgo_db.* TO 'vexgo_user'@'%';
mysql> FLUSH PRIVILEGES;
```

使用 MySQL 配置运行 VexGo：

```bash
cd backend
go run ./cmd/vexgo -c ../examples/config-mysql.yml
```

> 首次启动时 VexGo 会自动执行数据库迁移——无需手动导入任何东西。

---

## 配置 SSO

VexGo 支持通过 **GitHub**、**Google** 以及任意 **OpenID Connect (OIDC)** 提供商（Keycloak、Authentik、Authelia、Okta、Casdoor 等）登录。

> 将 `BASE_URL` 设置为你的公网实例地址（如 `https://vexgo.example.com`），以便正确生成 OAuth 回调地址，尤其是在反向代理之后。

### GitHub

1. 在 <https://github.com/settings/developers> 注册 OAuth App。
2. 设置回调地址为 `https://your-domain/api/sso/github/callback`。
3. 设置 `GITHUB_CLIENT_ID` 和 `GITHUB_CLIENT_SECRET`（或配置文件中的 `github_client_id` / `github_client_secret`）。

### Google

1. 在 <https://console.developers.google.com> 创建凭据。
2. 设置回调地址为 `https://your-domain/api/sso/google/callback`。
3. 设置 `GOOGLE_CLIENT_ID` 和 `GOOGLE_CLIENT_SECRET`。

### OIDC（任意提供商）

1. 在 OIDC 提供商处创建客户端，回调地址为 `https://your-domain/api/sso/oidc/callback`。
2. 找到你的 **issuer URL**：打开 `<provider-base-url>/.well-known/openid-configuration`，查看 `issuer` 字段。
3. 启用 OIDC：

```yaml
oidc_enabled: true
oidc_issuer_url: "https://auth.example.com/realms/myrealm"
oidc_client_id: "your-client-id"
oidc_client_secret: "your-client-secret"
```

如需限制登录组，设置 `oidc_allowed_groups`（如 `admins,developers`），并确保令牌中包含组 claim（必要时在 `oidc_scopes` 中加入 `groups`）。

### 强制仅 SSO 登录

将 `allow_local_login` 设为 `false` 可完全禁用密码登录——用户只能通过 SSO 提供商登录。

---

## 配置邮件（SMTP）

SMTP 在**运行时通过管理面板**配置，而不是在配置文件中：

1. 以管理员身份登录，打开**设置 → SMTP**。
2. 输入 SMTP 主机、端口、凭据以及发件地址/名称。
3. 点击**发送测试邮件**验证配置。

邮件用于：

- 注册后的邮箱验证
- 密码重置链接
- 修改邮箱确认

---

## 启用 S3 存储

S3 兼容任何 S3 兼容服务——AWS S3、MinIO、Garage 等。

### 示例：Docker + MinIO

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

> **MinIO/Wasabi：** 请设置 `S3_FORCE_PATH=true`——大多数 S3 兼容服务需要路径风格 URL。

## 静态敏感信息加密

SMTP 密码以及 AI / 评论审核 API 密钥存储在数据库中。设置 `SETTINGS_ENCRYPTION_KEY`（或 `settings_encryption_key`）即可使用 AES-256-GCM 对其静态加密。设置密钥后，存量明文会在下次启动时一次性就地加密；未设置时，这些信息以明文存储并在启动时记录警告。完整行为（包括密钥错误时的处理）参见[配置参考](/zh-cn/reference/configuration#静态敏感信息加密)。

## 哪些配置在运行时管理？

以下设置在**管理面板**中管理，存储在数据库中（无需重启）：

- **通用设置** —— 站点名称、描述、注册开关、验证码、游客浏览、每页条数
- **评论审核** —— 人工审核、关键词过滤、大模型审核三个开关；提示词与拦截关键词
- **当前主题** —— 切换已安装的主题
- **SMTP** —— 见上文

对应的 API 端点见 [API 参考](/zh-cn/reference/api)，完整的参数、变量和配置键清单见[配置参考](/zh-cn/reference/configuration)。
