# VexGo

**[English](README.md) | 中文**

这是一个基于 React、Go、Gin、JWT 和 SQLite 构建的博客内容管理系统，实现了用户注册、文章管理等功能。

## 快速开始

在发布页面选择对应的系统和架构进行下载。

### Linux

```bash
./vexgo-linux-amd64
```

### Docker

```bash
sudo docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/antipeth/vexgo:latest
```

然后访问 http://127.0.0.1:3001

**默认超级管理员账号**：admin@example.com  
**默认超级管理员密码**：password  

您可以在个人资料页面修改账号密码。

## 配置

配置优先级：**命令行参数 > 配置文件 > 环境变量 > 默认值**

### 使用配置文件

以下是示例配置文件：

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
# Database configuration
db_type: "sqlite"  # Options: "sqlite", "mysql", or "postgres"
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
```

然后运行以下命令：

```bash
./vexgo-linux-amd64 -c /the/path/to/config.yml
```

### 使用环境变量

您也可以通过环境变量配置应用程序。

#### Server

| 变量          | 默认值      | 说明                  |
|---------------|-------------|-----------------------|
| ADDR          | 0.0.0.0     | 服务监听地址          |
| PORT          | 3001        | 服务监听端口          |
| DATA          | ./data      | 数据目录路径          |
| JWT_SECRET    | —           | JWT 签名密钥（生产环境必填） |

#### Database

| 变量           | 默认值   | 说明                              |
|----------------|----------|-----------------------------------|
| DB_TYPE        | sqlite   | 数据库类型：sqlite、mysql、postgres |
| DB_HOST        | —        | 数据库主机（mysql/postgres 必填） |
| DB_PORT        | —        | 数据库端口（mysql/postgres 必填） |
| DB_USER        | —        | 数据库用户名（mysql/postgres 必填）|
| DB_PASSWORD    | —        | 数据库密码（mysql/postgres 必填） |
| DB_NAME        | —        | 数据库名称（mysql/postgres 必填） |
| DB_SSL_MODE    | disable  | Postgres SSL 模式                 |

## SSO / 单点登录

VexGo 支持 GitHub、Google 以及任何兼容 OpenID Connect (OIDC) 的提供商（Keycloak、Authentik、Authelia、Okta、Casdoor 等）。

### 通用配置

| 变量               | 默认值 | 说明 |
|--------------------|--------|------|
| BASE_URL           | —      | 实例的公网访问地址（如 https://vexgo.example.com）。使用反向代理时必须填写，用于正确生成 OAuth2 回调地址。 |
| ALLOW_LOCAL_LOGIN  | true   | 设置为 false 可禁用密码登录，强制仅使用 SSO 登录。 |

### GitHub

| 变量                  | 说明 |
|-----------------------|------|
| GITHUB_CLIENT_ID      | GitHub OAuth App Client ID |
| GITHUB_CLIENT_SECRET  | GitHub OAuth App Client Secret |

在 https://github.com/settings/developers 注册 OAuth App，回调地址设置为 `https://your-domain/api/sso/github/callback`。

### Google

| 变量                  | 说明 |
|-----------------------|------|
| GOOGLE_CLIENT_ID      | Google OAuth 2.0 Client ID |
| GOOGLE_CLIENT_SECRET  | Google OAuth 2.0 Client Secret |

在 https://console.developers.google.com 创建凭证，回调地址设置为 `https://your-domain/api/sso/google/callback`。

### OIDC

| 变量                  | 默认值 | 说明 |
|-----------------------|--------|------|
| OIDC_ENABLED          | false  | 设置为 true 启用 OIDC 登录 |
| OIDC_ISSUER_URL       | —      | OIDC 提供商的 Issuer URL，例如 https://auth.example.com/realms/myrealm VexGo 会自动通过 `<issuer>/.well-known/openid-configuration` 发现端点。 |
| OIDC_CLIENT_ID        | —      | 提供商颁发的 Client ID |
| OIDC_CLIENT_SECRET    | —      | 提供商颁发的 Client Secret |

**高级选项：**

| 变量                  | 默认值             | 说明 |
|-----------------------|--------------------|------|
| OIDC_SCOPES           | openid profile email | 空格分隔的作用域。如需组权限可添加 `groups`。 |
| OIDC_EMAIL_CLAIM      | email              | 用户邮箱对应的 Claim 名称 |
| OIDC_NAME_CLAIM       | name               | 用户显示名称对应的 Claim 名称 |
| OIDC_GROUP_CLAIM      | groups             | 用户组对应的 Claim 名称 |
| OIDC_ALLOWED_GROUPS   | —                  | 允许登录的组列表（逗号分隔）。留空表示允许所有用户。 |
| OIDC_AUTO_REDIRECT    | false              | 登录页自动跳转到 OIDC 提供商，跳过密码表单。 |
| OIDC_VERIFY_EMAIL     | false              | 要求 token 中 `email_verified=true` 才允许登录。 |
| OIDC_AUTH_URL         | —                  | 手动指定授权端点（仅在自动发现不可用时使用） |
| OIDC_TOKEN_URL        | —                  | 手动指定 Token 端点 |
| OIDC_USERINFO_URL     | —                  | 手动指定 UserInfo 端点（id_token 缺少必要信息时备用） |

OIDC 客户端回调地址：`https://your-domain/api/sso/oidc/callback`。

**提示**：要查找 Issuer URL，可打开 `<provider-base-url>/.well-known/openid-configuration` 并查看 `issuer` 字段。

#### 示例：Docker + OIDC

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

#### 示例：环境变量方式

```bash
export BASE_URL=https://vexgo.example.com
export OIDC_ENABLED=true
export OIDC_ISSUER_URL=https://auth.example.com/realms/myrealm
export OIDC_CLIENT_ID=your-client-id
export OIDC_CLIENT_SECRET=your-client-secret
./vexgo-linux-amd64
```

## 数据库

### Postgres（推荐版本：Postgres 18）

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
go run main.go -c ../examples/config-postgres.yml
```

### MySQL（推荐版本：MySQL 8）

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
go run main.go -c ../examples/config-mysql.yml
```

## 开发环境

### 环境要求

* Linux / macOS
* Go
* Node.js
* pnpm

### 构建步骤

```bash
git clone https://github.com/weimm16/vexgo.git
cd vexgo/frontend
pnpm install
pnpm run build
cd ../backend
go run main.go
```

然后访问 http://127.0.0.1:3001
