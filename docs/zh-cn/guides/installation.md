# 安装

> **操作指南** —— 本指南介绍如何在你的机器或服务器上安装 VexGo。请根据你的环境选择合适的方式。

## 环境要求

- **操作系统**：Linux、MacOS、Windows、FreeBSD，或任何装有 Docker 的系统
- **内存**：最低 512 MB，推荐 1 GB
- **磁盘**：应用本身至少 100 MB，另需数据存储空间

| 方式                                         | 依赖                    | 适用场景               |
| -------------------------------------------- | ----------------------- | ---------------------- |
| [二进制包](#方式-1二进制包安装)              | 下载可执行文件          | 本地快速运行、VPS 部署 |
| [Docker](#方式-2docker-安装)                 | Docker                  | 单容器部署             |
| [Docker Compose](#方式-3docker-compose-安装) | Docker + Docker Compose | 多服务场景（含数据库） |
| [Nix](#方式-4nix-安装)                       | Nix 包管理器            | 即时试用、可复现配置   |
| [NixOS Flake](#方式-5nixos-flake-安装)       | 启用 flakes 的 NixOS    | NixOS 系统             |
| [源码编译](#方式-6源码编译)                  | Go 1.25+、Node.js、pnpm | 开发、定制构建         |

---

## 方式 1：二进制包安装

最简单的方式——下载预编译二进制并直接运行。

### 第 1 步：下载二进制

前往[发布页面](https://github.com/vexgo-org/vexgo/releases)，下载对应系统和架构的二进制。

大多数 64 位 Linux 系统：

```bash
curl -L $(curl -s https://api.github.com/repos/vexgo-org/vexgo/releases/latest | grep browser_download_url | grep linux-amd64 | cut -d '"' -f 4) -o vexgo
chmod +x vexgo
```

ARM 系统（树莓派、ARM 服务器）：

```bash
curl -L $(curl -s https://api.github.com/repos/vexgo-org/vexgo/releases/latest | grep browser_download_url | grep linux-arm64 | cut -d '"' -f 4) -o vexgo
chmod +x vexgo
```

### 第 2 步：创建数据目录

```bash
mkdir -p ./data
```

### 第 3 步：运行 VexGo

```bash
./vexgo
```

默认在 `http://0.0.0.0:3001` 启动。

### 第 4 步：自定义参数运行

```bash
# 自定义端口和数据目录
./vexgo --port 8080 --data /path/to/data

# 自定义监听地址
./vexgo --addr 127.0.0.1

# 加载配置文件
./vexgo -c /path/to/config.yml

# 查看所有可用参数
./vexgo --help
```

### 第 5 步：注册为 systemd 服务（可选）

创建 `/etc/systemd/system/vexgo.service`：

```ini
[Unit]
Description=VexGo Blog CMS
After=network.target

[Service]
Type=simple
User=vexgo
Group=vexgo
WorkingDirectory=/opt/vexgo
ExecStart=/opt/vexgo/vexgo
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

创建专用用户并启用服务：

```bash
sudo useradd -r -s /bin/false vexgo
sudo mkdir -p /opt/vexgo /var/lib/vexgo
sudo chown -R vexgo:vexgo /opt/vexgo /var/lib/vexgo
sudo mv vexgo /opt/vexgo/

sudo systemctl daemon-reload
sudo systemctl enable vexgo
sudo systemctl start vexgo
sudo systemctl status vexgo
```

---

## 方式 2：Docker 安装

### 第 1 步：拉取并运行 VexGo

```bash
docker pull ghcr.io/vexgo-org/vexgo:latest

docker run -d \
  --name vexgo \
  -p 3001:3001 \
  -v ./data:/app/data \
  --restart unless-stopped \
  ghcr.io/vexgo-org/vexgo:latest
```

### 第 2 步：验证

```bash
docker ps                 # 查看容器状态
docker logs vexgo         # 查看容器日志
```

### 第 3 步：自定义配置运行

```bash
docker run -d \
  --name vexgo \
  -p 3001:3001 \
  -v ./data:/app/data \
  -v ./config.yml:/app/config.yml:ro \
  -e ADDR=0.0.0.0 \
  -e PORT=3001 \
  -e JWT_SECRET=your-secret-key-change-this-in-production \
  --restart unless-stopped \
  ghcr.io/vexgo-org/vexgo:latest
```

### 常用 Docker 命令

```bash
docker stop vexgo
docker start vexgo
docker restart vexgo
docker rm -f vexgo

# 更新到最新版本
docker pull ghcr.io/vexgo-org/vexgo:latest
docker stop vexgo && docker rm vexgo
docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data --restart unless-stopped ghcr.io/vexgo-org/vexgo:latest
```

---

## 方式 3：Docker Compose 安装

当 VexGo 需要与 PostgreSQL 或 MySQL 一起运行时，Docker Compose 是理想选择。

### 第 1 步：创建 `docker-compose.yml`

```yaml
version: "3.8"

services:
  vexgo:
    image: ghcr.io/vexgo-org/vexgo:latest
    container_name: vexgo
    ports:
      - "3001:3001"
    volumes:
      - ./data:/app/data
    environment:
      - ADDR=0.0.0.0
      - PORT=3001
      - JWT_SECRET=your-secret-key-change-this-in-production
      - DB_TYPE=postgres
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=vexgo
      - DB_PASSWORD=vexgo_password
      - DB_NAME=vexgo_db
    depends_on:
      - postgres
    restart: unless-stopped

  postgres:
    image: postgres:18-alpine
    container_name: vexgo-postgres
    environment:
      - POSTGRES_USER=vexgo
      - POSTGRES_PASSWORD=vexgo_password
      - POSTGRES_DB=vexgo_db
    volumes:
      - ./postgres:/var/lib/postgresql/data
    restart: unless-stopped
```

### 第 2 步：启动服务

```bash
mkdir -p data postgres
docker compose up -d

docker compose logs -f vexgo   # 查看日志
docker compose ps              # 查看状态
```

### 常用 Compose 命令

```bash
docker compose stop
docker compose start
docker compose restart
docker compose down            # 停止并移除容器
docker compose down -v         # 同时删除数据卷（会删除数据！）
```

---

## 方式 4：Nix 安装

### 第 1 步：安装 Nix

```bash
curl -L https://nixos.org/nix/install | sh
source ~/.nix-profile/etc/profile.d/nix.sh
```

### 第 2 步：直接运行 VexGo

```bash
# 无需安装即可运行（从 GitHub 拉取）
nix run github:vexgo-org/vexgo
```

### 第 3 步：永久安装

```bash
nix profile install github:vexgo-org/vexgo
vexgo
```

### 第 4 步：自定义参数运行

```bash
nix run github:vexgo-org/vexgo -- -c /path/to/config.yml
nix run github:vexgo-org/vexgo -- --port 8080 --addr 0.0.0.0
```

---

## 方式 5：NixOS Flake 安装

### 第 1 步：启用 Flakes

在 `/etc/nixos/configuration.nix` 中添加：

```nix
{ config, pkgs, ... }:
{
  nix.settings.experimental-features = [ "nix-command" "flakes" ];
}
```

重建系统：

```bash
sudo nixos-rebuild switch
```

### 第 2 步：将 VexGo 加入 Flake

```nix
{
  description = "My NixOS Configuration";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    vexgo = {
      url = "github:vexgo-org/vexgo";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, vexgo, ... } @ inputs:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
        specialArgs = { inherit inputs; };
        modules = [
          inputs.vexgo.nixosModules.default
          ./configuration.nix
          ./vexgo.nix
        ];
      };
    };
}
```

### 第 3 步：创建 `vexgo.nix`

```nix
{ config, pkgs, inputs, ... }:
{
  nixpkgs.overlays = [ inputs.vexgo.overlays.default ];

  services.vexgo = {
    enable = true;
    settings = {
      addr = "0.0.0.0";
      port = 3001;
      data = "/var/lib/vexgo";
      jwt_secret = "your-secret-key-change-this-in-production";
      log_level = "info";
    };
  };

  # 可选：改用 PostgreSQL 而非 SQLite
  services.postgresql = {
    enable = true;
    ensureDatabases = [ "vexgo" ];
    ensureUsers = [
      { name = "vexgo"; ensureDBOwnership = true; }
    ];
  };

  networking.firewall.allowedTCPPorts = [ 3001 ];
}
```

### 第 4 步：重建与管理

```bash
sudo nix flake update
sudo nixos-rebuild switch --flake .#myhost
sudo systemctl status vexgo

sudo systemctl start vexgo
sudo systemctl stop vexgo
sudo systemctl restart vexgo
sudo journalctl -u vexgo -f
```

---

## 方式 6：源码编译

当你需要最新的开发版本或自定义代码时，请使用此方式。

### 第 1 步：安装构建依赖

- **Go 1.25+**
- **Node.js** 和 **pnpm 10**
- 可选（推荐）：`just`、`gofumpt`、`golangci-lint`、`prettier`、`oxlint` —— 可通过 `nix develop` 获得包含全部工具的 Nix 开发环境

### 第 2 步：克隆并构建

```bash
git clone https://github.com/vexgo-org/vexgo.git
cd vexgo

# 构建前端（产物嵌入后端二进制）
cd frontend
pnpm install
pnpm run build
cd ..

# 构建后端
cd backend
go mod download
go build -o vexgo ./cmd/vexgo
cd ..
```

### 第 3 步：运行

```bash
./backend/vexgo
```

### 开发常用命令

```sh
just format            # gofumpt -w -extra . && prettier --write
just lint              # golangci-lint + prettier --check + gofumpt check + oxlint
go build -v ./...      # 构建后端
go test -v ./...       # 运行后端测试
```

前端构建产物写入 `backend/internal/public/dist` 并嵌入后端二进制，因此修改前端后需要重新构建。

---

## 安装完成后

### 访问网站

打开 `http://localhost:3001`（远程服务器则为 `http://your-server-ip:3001`）。

### 默认账号

| 字段 | 值                  |
| ---- | ------------------- |
| 邮箱 | `admin@example.com` |
| 密码 | `password`          |

> **⚠️ 重要：** 首次登录后请立即修改默认密码——参见[快速开始](/zh-cn/getting-started)教程。

### 验证安装

1. 使用默认账号登录。
2. 创建一篇测试文章并发布。
3. 确认它出现在首页。

### 下一步

- [配置指南](/zh-cn/guides/configuration) —— 配置数据库、SSO、S3 和邮件
- [生产部署](/zh-cn/guides/deployment) —— 反向代理、HTTPS、systemd
- [故障排查](/zh-cn/guides/deployment#故障排查) —— 常见问题及解决方法
