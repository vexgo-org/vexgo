# 部署

> **操作指南** —— 本指南介绍如何在生产环境运行 VexGo：安全加固、反向代理、HTTPS、服务管理、升级和故障排查。

## 生产环境检查清单

在将 VexGo 暴露到公网之前：

- [ ] 修改默认超级管理员密码
- [ ] 设置强 `JWT_SECRET`（例如 `openssl rand -base64 32`）
- [ ] 设置 `SETTINGS_ENCRYPTION_KEY`（例如 `openssl rand -base64 32`），使 SMTP 密码及 AI/评论审核 API 密钥静态加密
- [ ] 将 VexGo 放在带 HTTPS 的反向代理之后
- [ ] 将 `BASE_URL` 设置为公网地址（SSO 回调必需）
- [ ] 设置 `behind_reverse_proxy: true` 并配置 `trusted_proxies`（或设置 `BEHIND_REVERSE_PROXY=true` / `TRUSTED_PROXIES=...`）
- [ ] 生产环境使用真实数据库（PostgreSQL）而非默认 SQLite
- [ ] 定期备份数据目录和数据库

---

## 反向代理

VexGo 默认在 `3001` 端口提供 HTTP 服务。生产环境中通常在其前面放置反向代理（Nginx、Caddy、Cloudflare 等），负责 TLS、缓存和真实客户端 IP。

### Nginx

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Caddy

```caddy
your-domain.com {
    reverse_proxy 127.0.0.1:3001
}
```

Caddy 会自动申请并续期 HTTPS 证书。

### 告知 VexGo 代理的存在

设置以下项，让 VexGo 信任代理请求头（真实客户端 IP）并生成正确的 URL：

```yaml
behind_reverse_proxy: true
trusted_proxies: ["192.168.1.100"] # 或 CIDR，如 "10.0.0.0/8"
```

如果 `trusted_proxies` 为空，VexGo 默认使用常见私有网段（`127.0.0.1`、`::1`、`192.168.0.0/16`、`10.0.0.0/8`、`172.16.0.0/12`）。

同时通过环境变量设置公网地址（SSO 重定向用）：

```bash
BASE_URL=https://your-domain.com ./vexgo
```

或配置在 Docker/systemd 环境中。

---

## Let's Encrypt HTTPS

配合 Nginx 和 Certbot：

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
```

Certbot 会配置 TLS、设置自动续期，并将 HTTP 重定向到 HTTPS。

---

## 作为服务运行（systemd）

systemd 单元可让 VexGo 在重启和崩溃后自动恢复。完整单元文件见[安装指南](/zh-cn/guides/installation#第-5-步注册为-systemd-服务可选)。摘要：

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

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable vexgo
sudo systemctl start vexgo
sudo systemctl status vexgo
```

---

## 防火墙

只开放 VexGo 需要的端口：

```bash
# UFW（Ubuntu/Debian）
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 3001/tcp   # 仅在 VexGo 直接暴露时
sudo ufw enable

# firewalld（Fedora/CentOS）
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --reload
```

> **提示：** 如果 VexGo 与反向代理在同一台机器上，只需开放 80/443 端口——代理通过 localhost 与 VexGo 通信。

---

## 备份

VexGo 的数据存放在两处：

1. **数据目录** —— SQLite 数据库文件和上传的媒体（启用 S3 时媒体在 S3）
2. **数据库** —— 使用 PostgreSQL/MySQL 时

### 备份数据目录

```bash
tar -czf vexgo-data-$(date +%F).tar.gz /var/lib/vexgo
```

### 备份 PostgreSQL

```bash
pg_dump -U vexgo_user vexgo_db > vexgo-db-$(date +%F).sql
```

使用 `psql -U vexgo_user vexgo_db < backup.sql` 恢复。

用 cron 或 systemd 定时器定期备份，并存储到机器之外（如 S3 桶或其他服务器）。

> 如果设置了 `SETTINGS_ENCRYPTION_KEY`，请将其与数据库一起备份：恢复备份时若没有该密钥，将无法解密已存储的 SMTP 密码与 API 密钥（服务器仍可运行，但需要管理员在后台重新录入这些敏感信息）。

---

## 升级

> **升级前：** 备份数据目录和数据库。升级后重启，以便数据库迁移执行。

### Docker

```bash
docker pull ghcr.io/vexgo-org/vexgo:latest
docker stop vexgo && docker rm vexgo
docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data --restart unless-stopped ghcr.io/vexgo-org/vexgo:latest
```

### 二进制包

1. 从[发布页面](https://github.com/vexgo-org/vexgo/releases)下载新二进制。
2. 停止服务：`sudo systemctl stop vexgo`
3. 替换二进制：`sudo mv vexgo /opt/vexgo/vexgo`
4. 重新启动：`sudo systemctl start vexgo`

### Nix

```bash
nix flake update
sudo nixos-rebuild switch --flake .#myhost
```

---

## 故障排查

### 端口 3001 被占用

```bash
sudo lsof -i :3001          # 查找占用进程
sudo kill -9 <PID>          # 或使用其他端口：
./vexgo --port 8080
```

### 权限不足

```bash
chmod +x vexgo
sudo chown -R $USER:$USER ./data
```

### 数据库连接问题

```bash
docker ps | grep postgres                        # 数据库是否在运行？
psql -h localhost -U vexgo -d vexgo_db           # 测试连接
docker logs vexgo-postgres                        # 数据库日志
```

### Docker 容器无法启动

```bash
docker logs vexgo
docker inspect vexgo
docker rm -f vexgo && docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/vexgo-org/vexgo:latest
```

### systemd 服务问题

```bash
sudo systemctl status vexgo
sudo journalctl -u vexgo -n 50
sudo systemctl restart vexgo
sudo systemctl cat vexgo
```

### 获取帮助

- [GitHub Issues](https://github.com/vexgo-org/vexgo/issues)
- [Discussions](https://github.com/vexgo-org/vexgo/discussions)
