# VexGo

**[English](README.md) | 中文**

[![Go Version](https://img.shields.io/github/go-mod/go-version/vexgo-org/vexgo)](https://go.dev/)
[![License](https://img.shields.io/github/license/vexgo-org/vexgo)](./LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/vexgo-org/vexgo/build-and-test.yml?branch=main)](https://github.com/vexgo-org/vexgo/actions)
[![Release](https://img.shields.io/github/v/release/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/releases)

## VexGo - 现代化博客 CMS

VexGo 是一个轻量级的、自托管博客内容管理系统，专为重视简洁性、性能和控制权的开发者和作家而设计。采用现代技术构建，它提供了一个完整的博客平台，包括用户管理、丰富内容创作和可扩展性。

### ✨ 主要特性

- **🖥️ 现代化 Web 界面**：基于 React 的管理面板用于内容管理
- **🚀 高性能**：使用 Go 和 Gin 构建，实现快速高效的处理
- **🔐 安全认证**：基于 JWT 的用户系统，具有基于角色的权限（user / admin / super_admin）
- **📝 丰富内容**：Markdown 编辑器、分类、标签、草稿、点赞和评论
- **🛡️ AI 内容审核**：可配置提示词、关键词拦截和评分阈值的自动评论审核
- **🖼️ 媒体管理**：内置文件存储，支持 S3 兼容服务
- **🎨 主题系统**：服务端渲染主题，可在管理面板切换和上传
- **🔔 通知**：点赞、评论等事件的站内通知收件箱
- **🔑 SSO**：支持 GitHub、Google 及任意 OpenID Connect 提供商登录
- **🌐 自托管**：完全控制您的数据和部署

### 🛠️ 技术栈

- **后端**：Go, Gin, GORM, SQLite/PostgreSQL/MySQL
- **前端**：React, TypeScript, Vite, Tailwind CSS
- **认证**：JWT, OAuth (GitHub, Google, OIDC)
- **存储**：本地文件系统或 S3 兼容服务
- **邮件**：SMTP 集成

## 目录

- [快速开始](#快速开始)
- [配置](#配置)
- [SSO / 单点登录](#sso--单点登录)
- [数据库](#数据库)
- [开发环境](#开发环境)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

## 快速开始

在发布页面选择对应的系统和架构进行下载。

### Linux

```bash
./vexgo-linux-amd64
```

### Docker

```bash
sudo docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/vexgo-org/vexgo:latest
```

### ❄️Nix

无需安装即可立即试用 VexGo：

```bash
nix run github:vexgo-org/vexgo
```

### ❄️NixOS Flake

在你的 `flake.nix` 的 `inputs` 中添加：

```nix
# flake.nix
inputs = {
  vexgo = {
    url = "github:vexgo-org/vexgo";
    inputs.nixpkgs.follows = "nixpkgs";
  };
};
```

然后在 `nixosSystem` 的 modules 中导入模块：

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

创建 `vexgo.nix` 并写入配置：

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

然后重建系统：

```bash
sudo nixos-rebuild switch --flake .#your-host
```

### 安装之后

访问 http://127.0.0.1:3001

**默认超级管理员账号**：`admin@example.com`
**默认超级管理员密码**：`password`

您可以在个人资料页面修改账号密码。

## 配置

配置优先级：**命令行参数 > 配置文件 > 环境变量 > 默认值**

### 使用配置文件

以下是示例配置文件：

```yaml
# 服务器监听地址
addr: "0.0.0.0"

# 服务器监听端口
port: 3001

# 数据目录（用于存储 SQLite 数据库和上传的媒体文件）
data: "./data"

# JWT 密钥，用于签名 token
# 重要：生产环境必须使用安全的随机字符串！
# 可以使用以下命令生成：openssl rand -base64 32
jwt_secret: "your-secret-key-change-this-in-production"

# Passphrase used to encrypt secrets at rest in the database (SMTP password,
# AI and comment-moderation API keys) with AES-256-GCM.
# IMPORTANT: Generate a secure random string for production!
# You can generate one with: openssl rand -base64 32
# When empty, these secrets are stored in plaintext (a warning is logged at startup).
# Existing plaintext values are encrypted in place on the first start with a key set.
settings_encryption_key: ""

# 日志级别："debug", "info", "warn", "error"
log_level: "info"

# 实例的公网访问地址（如 "https://vexgo.example.com"）
# 用于生成 OAuth/SSO 回调地址和邮件中的链接
# （邮箱验证、密码重置、换绑邮箱）。
# 反向代理后必须设置；留空时这些链接会退回使用请求 Host 头，
# 存在 Host 头注入风险。
base_url: ""

# 服务是否位于反向代理之后（例如 nginx、Cloudflare）
# 如果使用了会设置 X-Forwarded-* 请求头的反向代理，请设为 true
behind_reverse_proxy: false

# 受信任的代理 IP/CIDR 列表（逗号分隔）
# 仅在 behind_reverse_proxy=true 时生效
# 如果留空，默认使用常见私有网络：127.0.0.1、::1、192.168.0.0/16、10.0.0.0/8、172.16.0.0/12
# 示例：
#   - 单个代理：["192.168.1.100"]
#   - 多个代理：["192.168.1.100", "10.0.0.1"]
#   - CIDR 表示法：["192.168.1.0/24"]
trusted_proxies: []

# ==================== 数据库配置 ====================

# 数据库类型
# 可选值: "sqlite", "mysql", "postgres", "mariadb"
db_type: "sqlite"

# 当 db_type 为 "mysql" 或 "mariadb" 时，配置以下参数
# db_host: "127.0.0.1"
# db_port: 3306
# db_user: "your_username"
# db_password: "your_password"
# db_name: "vexgo"

# 当 db_type 为 "postgres" 时，配置以下参数
# db_host: "127.0.0.1"
# db_port: 5432
# db_user: "your_username"
# db_password: "your_password"
# db_name: "vexgo"
# db_ssl_mode: "disable"  # 可选值: "disable", "require", "verify-ca", "verify-full"

# ==================== SSO 配置 ====================

# -------------------- GitHub OAuth --------------------
# GitHub OAuth 应用凭据 (https://github.com/settings/developers)
# 留空则禁用 GitHub 登录
github_client_id: ""
github_client_secret: ""

# -------------------- Google OAuth --------------------
# Google OAuth 2.0 凭据 (https://console.cloud.google.com/apis/credentials)
# 留空则禁用 Google 登录
google_client_id: ""
google_client_secret: ""

# -------------------- OIDC (OpenID Connect) --------------------
# 通用 OIDC 提供商支持 (Keycloak, Authentik, Okta, Authelia 等)
# 是否启用 OIDC 登录
oidc_enabled: false

# OIDC discovery URL（启用时必填）
# 服务器会从以下地址获取 OIDC 配置：{issuer_url}/.well-known/openid-configuration
# 示例: "https://auth.example.com/realms/myrealm"
oidc_issuer_url: ""

# OIDC 客户端凭据（启用时必填）
oidc_client_id: ""
oidc_client_secret: ""

# 手动端点覆盖（仅当 OIDC discovery 不可用时需要）
# 如果填写，将覆盖自动发现的端点
oidc_auth_url: "" # 授权端点
oidc_token_url: "" # Token 端点
oidc_userinfo_url: "" # UserInfo 端点（可选备用）

# OIDC scope（空格分隔，默认: "openid profile email"）
# 如果提供商需要，可以添加额外 scope，例如："openid profile email groups"
oidc_scopes: "openid profile email"

# OIDC claim 名称（默认是标准 OIDC claim）
oidc_email_claim: "email" # 用户邮箱的 claim 名称
oidc_name_claim: "name" # 用户显示名的 claim 名称
oidc_group_claim: "groups" # 用户组的 claim 名称（用于访问控制）

# OIDC 访问控制（可选）
# 允许登录的 group 列表，用逗号分隔
# 留空表示允许所有已认证用户
oidc_allowed_groups: ""

# OIDC 用户体验选项
oidc_auto_redirect: false # true 时跳过登录页，直接跳转到 OIDC 提供商
oidc_verify_email: false # true 时要求 ID token 中 email_verified=true

# -------------------- 全局选项 --------------------
# 设为 false 表示强制只允许 SSO 登录（禁用本地密码登录）
allow_local_login: true

# ==================== S3 存储配置 ====================

# 启用 S3 兼容存储用于文件上传
# 上传的媒体文件将存储到 bucket，而不是本地 data 目录
s3_enabled: false

# S3 endpoint URL（AWS S3 留空）
s3_endpoint: ""

# AWS region（AWS 必填；S3 兼容服务可随意填）
s3_region: "us-east-1"

# S3 bucket 名称
s3_bucket: "my-bucket"

# S3 access key
s3_access_key: ""

# S3 secret key
s3_secret_key: ""

# 强制使用 path-style URL（MinIO / Wasabi 等需要）
s3_force_path: false

# 可选自定义域名用于公开文件 URL （例如 CDN 域名: "cdn.example.com"）
# 留空则使用默认 S3 地址
s3_custom_domain: ""

# 禁用在自定义域名 URL 中包含存储桶（默认: false，意味着默认包含存储桶）
s3_disable_bucket_in_custom_url: false
```

然后运行以下命令：

```bash
./vexgo-linux-amd64 -c /the/path/to/config.yml
```

### 使用环境变量

您也可以通过环境变量配置应用程序。

#### Server

| 变量                      | 默认值    | 说明                                                                                                                                                                                                                           |
| ------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ADDR`                    | `0.0.0.0` | 服务监听地址                                                                                                                                                                                                                   |
| `PORT`                    | `3001`    | 服务监听端口                                                                                                                                                                                                                   |
| `DATA_DIR`                | `./data`  | 数据目录路径                                                                                                                                                                                                                   |
| `JWT_SECRET`              | —         | JWT 签名密钥（生产环境必填）                                                                                                                                                                                                   |
| `SETTINGS_ENCRYPTION_KEY` | —         | 用于在数据库中对静态敏感信息（SMTP 密码、AI 与评论审核 API 密钥）做 AES-256-GCM 加密的口令。留空时这些信息以明文存储（启动时会记录警告）。                                                                                     |
| `LOG_LEVEL`               | `info`    | 日志级别：`debug`、`info`、`warn`、`error`                                                                                                                                                                                     |
| `BASE_URL`                | —         | 实例的公网地址（如 `https://vexgo.example.com`）。用于生成 OAuth 回调地址和邮件中的链接（邮箱验证、密码重置、换绑邮箱）。反向代理后必须设置。                                                                                  |
| `BEHIND_REVERSE_PROXY`    | `false`   | 设置为 `true` 表示服务器位于反向代理（如 nginx、Cloudflare 等）之后。启用后才会正确处理 `X-Forwarded-*` 头部。                                                                                                                 |
| `TRUSTED_PROXIES`         | —         | 受信任的代理 IP/CIDR 列表（逗号分隔）。仅在 `BEHIND_REVERSE_PROXY=true` 时生效。如果留空，默认使用常见私有网络（127.0.0.1、::1、192.168.0.0/16、10.0.0.0/8、172.16.0.0/12）。示例：`TRUSTED_PROXIES="192.168.1.100, 10.0.0.1"` |

#### Database

| 变量          | 默认值    | 说明                                                 |
| ------------- | --------- | ---------------------------------------------------- |
| `DB_TYPE`     | `sqlite`  | 数据库类型：`sqlite`、`mysql`、`postgres`、`mariadb` |
| `DB_HOST`     | —         | 数据库主机（mysql/postgres 必填）                    |
| `DB_PORT`     | —         | 数据库端口（mysql/postgres 必填）                    |
| `DB_USER`     | —         | 数据库用户名（mysql/postgres 必填）                  |
| `DB_PASSWORD` | —         | 数据库密码（mysql/postgres 必填）                    |
| `DB_NAME`     | —         | 数据库名称（mysql/postgres 必填）                    |
| `DB_SSL_MODE` | `disable` | Postgres SSL 模式                                    |

#### SSO / 单点登录

VexGo 支持 GitHub、Google 以及任何兼容 OpenID Connect (OIDC) 的提供商（Keycloak、Authentik、Authelia、Okta、Casdoor 等）。

**通用配置**

| 变量                | 默认值 | 说明                                                 |
| ------------------- | ------ | ---------------------------------------------------- |
| `ALLOW_LOCAL_LOGIN` | `true` | 设置为 `false` 可禁用密码登录，强制仅使用 SSO 登录。 |

> **注意：** `BASE_URL` 是通用服务器配置，并非 SSO 专属，详见[服务器](#server)一节。它同时决定 OAuth 回调地址和邮件中的链接（邮箱验证、密码重置、换绑邮箱），启用邮件或 SSO 时建议务必设置。

**GitHub**

| 变量                   | 说明                           |
| ---------------------- | ------------------------------ |
| `GITHUB_CLIENT_ID`     | GitHub OAuth App Client ID     |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App Client Secret |

在 https://github.com/settings/developers 注册 OAuth App，回调地址设置为 `https://your-domain/api/sso/github/callback`。

**Google**

| 变量                   | 说明                           |
| ---------------------- | ------------------------------ |
| `GOOGLE_CLIENT_ID`     | Google OAuth 2.0 Client ID     |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 Client Secret |

在 https://console.developers.google.com 创建凭证，回调地址设置为 `https://your-domain/api/sso/google/callback`。

**OIDC**

| 变量                 | 默认值  | 说明                                                                                                                                              |
| -------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `OIDC_ENABLED`       | `false` | 设置为 `true` 启用 OIDC 登录                                                                                                                      |
| `OIDC_ISSUER_URL`    | —       | OIDC 提供商的 Issuer URL，例如 `https://auth.example.com/realms/myrealm`。VexGo 会自动通过 `<issuer>/.well-known/openid-configuration` 发现端点。 |
| `OIDC_CLIENT_ID`     | —       | 提供商颁发的 Client ID                                                                                                                            |
| `OIDC_CLIENT_SECRET` | —       | 提供商颁发的 Client Secret                                                                                                                        |

**高级选项：**

| 变量                  | 默认值                 | 说明                                                                           |
| --------------------- | ---------------------- | ------------------------------------------------------------------------------ |
| `OIDC_SCOPES`         | `openid profile email` | 空格分隔的作用域。如需组权限可添加 `groups`。                                  |
| `OIDC_EMAIL_CLAIM`    | `email`                | 用户邮箱对应的 Claim 名称                                                      |
| `OIDC_NAME_CLAIM`     | `name`                 | 用户显示名称对应的 Claim 名称                                                  |
| `OIDC_GROUP_CLAIM`    | `groups`               | 用户组对应的 Claim 名称                                                        |
| `OIDC_ALLOWED_GROUPS` | —                      | 允许登录的组列表（逗号分隔），例如 `admins,developers`。留空表示允许所有用户。 |
| `OIDC_AUTO_REDIRECT`  | `false`                | 登录页自动跳转到 OIDC 提供商，跳过密码表单。                                   |
| `OIDC_VERIFY_EMAIL`   | `false`                | 要求 token 中 `email_verified=true` 才允许登录。                               |
| `OIDC_AUTH_URL`       | —                      | 手动指定授权端点（仅在自动发现不可用时使用）                                   |
| `OIDC_TOKEN_URL`      | —                      | 手动指定 Token 端点                                                            |
| `OIDC_USERINFO_URL`   | —                      | 手动指定 UserInfo 端点（`id_token` 缺少必要信息时备用）                        |

OIDC 客户端回调地址：`https://your-domain/api/sso/oidc/callback`。

> **提示：** 要查找 Issuer URL，可打开 `<provider-base-url>/.well-known/openid-configuration` 并查看 `issuer` 字段。

**示例：Docker + OIDC**

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

**示例：环境变量方式**

```bash
export BASE_URL=https://vexgo.example.com
export OIDC_ENABLED=true
export OIDC_ISSUER_URL=https://auth.example.com/realms/myrealm
export OIDC_CLIENT_ID=your-client-id
export OIDC_CLIENT_SECRET=your-client-secret
./vexgo-linux-amd64
```

#### S3 / 对象存储

> **注意：** 当 `S3_ENABLED=true` 时，上传的媒体文件将存储到配置的存储桶中，而非本地 `data` 目录。

VexGo 支持任何兼容 S3 协议的对象存储服务（AWS S3、MinIO、Garage 等）。

| 变量                              | 默认值  | 描述                                                                                               |
| --------------------------------- | ------- | -------------------------------------------------------------------------------------------------- |
| `S3_ENABLED`                      | `false` | 设置为 `true` 以启用 S3 存储                                                                       |
| `S3_ENDPOINT`                     | —       | S3 端点 URL，例如 `https://minio.example.com`。使用标准 AWS S3 时留空即可。                        |
| `S3_REGION`                       | —       | AWS 区域，例如 `us-east-1`。使用 AWS S3 时必填；使用兼容 S3 的服务时可填任意值。                   |
| `S3_BUCKET`                       | —       | 目标存储桶名称                                                                                     |
| `S3_ACCESS_KEY`                   | —       | Access Key ID                                                                                      |
| `S3_SECRET_KEY`                   | —       | Secret Access Key                                                                                  |
| `S3_FORCE_PATH`                   | `false` | 设置为 `true` 以使用路径风格 URL（MinIO 及大多数兼容 S3 的服务需要开启）                           |
| `S3_CUSTOM_DOMAIN`                | —       | 生成文件公开访问 URL 时使用的自定义域名，例如 `cdn.example.com`。适用于在存储桶前接入 CDN 的场景。 |
| `S3_DISABLE_BUCKET_IN_CUSTOM_URL` | `false` | 设置为 `true` 以禁用在自定义域名 URL 中包含存储桶名（默认：包含存储桶，默认情况下）                |

**示例：Docker + MinIO**

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

## 数据库

### Postgres

推荐版本：Postgres 18

先启动 Postgres：

```bash
sudo docker run -d --name postgres -e POSTGRES_PASSWORD=test -p 5432:5432 -v ./postgres:/var/lib/postgresql/data docker.io/library/postgres:18-alpine
```

进入 Postgres 创建数据库和用户：

```bash
psql -U postgres
postgres=# CREATE USER vexgo_user WITH PASSWORD 'password';
postgres=# CREATE DATABASE vexgo_db OWNER vexgo_user ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C' TEMPLATE template0;
```

然后使用以下命令启动后端：

```bash
go run ./backend/cmd/vexgo -c examples/config-postgres.yml
```

### MySQL

推荐版本：MySQL 8

先启动 MySQL：

```bash
sudo docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=test -v ./mysql:/var/lib/mysql docker.io/library/mysql:8
```

进入 MySQL 创建数据库和用户：

```bash
mysql -p
mysql> CREATE DATABASE vexgo_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
mysql> CREATE USER 'vexgo_user'@'%' IDENTIFIED BY 'password';
mysql> GRANT ALL ON vexgo_db.* TO 'vexgo_user'@'%';
mysql> FLUSH PRIVILEGES;
```

然后使用以下命令启动后端：

```bash
cd backend
go run ./cmd/vexgo -c ../examples/config-mysql.yml
```

## 开发环境

### 环境要求

- Linux / macOS
- Go 1.25+
- Node.js 和 pnpm 10
- `just`、`gofumpt`、`golangci-lint`、`prettier`、`oxlint`（推荐；也可通过 `nix develop` 进入包含全部工具的 Nix 开发环境）

### 常用命令

```sh
just format            # gofumpt -w -extra . && prettier --write
just lint              # golangci-lint + prettier --check + gofumpt 检查 + oxlint
go build -v ./...      # 构建后端
go test -v ./...       # 运行后端测试

cd frontend
pnpm install
pnpm run dev           # 前端开发服务器（HMR）
pnpm run build         # 类型检查（tsc -b）+ vite 构建 + 拷贝 manifest
pnpm run lint          # oxlint
```

前端构建产物会输出到 `backend/internal/public/dist` 并嵌入后端二进制，修改前端后需要重新构建。

### 本地运行

```bash
git clone https://github.com/vexgo-org/vexgo.git
cd vexgo
cd frontend
pnpm install
pnpm run build
cd ../backend
go run ./cmd/vexgo
```

然后访问 http://127.0.0.1:3001。默认超级管理员账号：`admin@example.com` / `password`——请在个人资料页面修改密码。

### 后端结构

后端采用领域化布局，代码位于 `backend/internal` 下，并通过组合根（composition root）完成装配：

```text
backend/
  cmd/vexgo/main.go  # 入口：通过 cli 解析配置，委托给 app.New()
  internal/
    app/             # 组合根：装配存储、数据库与所有领域模块
    auth/            # 注册、登录、JWT、个人资料、密码重置
    cli/             # cobra 命令行：参数、帮助、版本、.env 加载
    comment/         # 评论与 AI 审核
    config/          # 基于 viper 的分层配置解析（参数 > 文件 > 环境变量 > 默认值），JWT、S3、SSO 初始化（纯配置，不依赖后端模块）
    database/        # 数据库连接、自动迁移、种子数据
    home/            # 站点统计
    mailer/          # SMTP 邮件构建与发送
    notification/    # 站内通知
    middleware/      # JWT 认证、角色权限、请求日志
    model/           # GORM 数据模型 + 共享接口（Notifier、FileRemover、Mailer）
    post/            # 文章 CRUD、分类、标签、点赞
    public/          # 内嵌前端、主题、SSR 渲染、静态路由
    router/          # 路由注册（组合所有领域模块）
    settings/        # 管理端配置（SMTP、AI、通用设置、主题）
    sso/             # GitHub / Google / OIDC 登录
    upload/          # 文件上传（本地磁盘或 S3）
    user/            # 用户管理、角色、创作者申请
    verification/    # 邮箱验证与滑块验证码
```

每个领域包内部遵循一致的三层结构：

```text
handler.go    → HTTP 请求解析、响应渲染（调用 service）
service.go    → 业务逻辑、跨域编排（调用 repository）
repository.go → 持久化接口 + GORM 实现（访问数据库）
```

handler 不直接接触 GORM；service 依赖 `Repository` 接口、与数据库解耦（可用 fake 做单元测试）；repository 封装全部 SQL/GORM 查询（含避免 N+1 的批量操作）。`context.Context` 贯穿三层。

导入使用模块路径 `github.com/vexgo-org/vexgo/backend/internal/<package>`，例如：

```go
import (
    "github.com/vexgo-org/vexgo/backend/internal/model"
    "github.com/vexgo-org/vexgo/backend/internal/post"
    "github.com/vexgo-org/vexgo/backend/internal/router"
)
```

### 依赖事实

- **叶子包** — `config/` 和 `model/` 不导入任何其他后端模块。`model` 除 GORM 数据模型外还持有跨域接口（`Notifier`、`FileRemover`、`Mailer`）；`config` 被 `app`、`auth`、`database`、`middleware`、`sso`、`upload` 引用。
- **共享层** — `middleware/`（JWT 认证、角色权限、请求日志）只依赖 `config` 和 `model`。
- **领域间依赖** — `auth` 被 `comment`、`post`、`sso` 引用；`auth` 自身依赖 `verification`；`settings` 依赖 `public`（主题管理）和 `mailer`（SMTP）；`database` 依赖 `config` 和 `model`。领域之间通过 `model` 中的接口协作：`notification` 实现 `Notifier`、`upload` 实现 `FileRemover`、`mailer` 实现 `Mailer`。依赖图无环。
- **接线** — `backend/cmd/vexgo/main.go` 是极简入口：解析参数后调用 `app.New(cfg)` / `app.Run()`。`internal/app` 是组合根——打开数据库、创建存储和 `public.Renderer`，然后通过调用 `router.RegisterAPIRoutes(r, router.Deps{...})`（定义于 `internal/router`）组装所有领域包。

### 贡献指南

请参阅 [CONTRIBUTING.md](CONTRIBUTING.md) 了解编码规范、测试要求以及 Issue、Pull Request 和提交信息规范。

### 许可证

VexGo 使用 [GNU Affero General Public License v3.0](LICENSE) 许可证。
