# Deployment

> **How-to** — this guide walks you through running VexGo in production: hardening, reverse proxies, HTTPS, service management, upgrades, and troubleshooting.

## Production Checklist

Before exposing VexGo to the internet:

- [ ] Change the default super admin password
- [ ] Set a strong `JWT_SECRET` (e.g. `openssl rand -base64 32`)
- [ ] Set `SETTINGS_ENCRYPTION_KEY` (e.g. `openssl rand -base64 32`) so SMTP password and AI/comment-moderation API keys are encrypted at rest
- [ ] Put VexGo behind a reverse proxy with HTTPS
- [ ] Set `BASE_URL` to your public URL (required for SSO callbacks)
- [ ] Set `behind_reverse_proxy: true` and configure `trusted_proxies` (or set `BEHIND_REVERSE_PROXY=true` / `TRUSTED_PROXIES=...`)
- [ ] Use a real database (PostgreSQL) instead of the default SQLite for production workloads
- [ ] Running more than one instance behind a load balancer? Set `VALKEY_ENABLED=true` (plus `VALKEY_URL`) so rate-limiting and OAuth state are shared — see [Scaling to Multiple Instances](#scaling-to-multiple-instances)
- [ ] Back up the data directory and database on a schedule

---

## Reverse Proxy

VexGo serves HTTP on port `3001` by default. In production you normally put a reverse proxy (Nginx, Caddy, Cloudflare, etc.) in front of it to handle TLS, caching, and real client IPs.

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

Caddy obtains and renews HTTPS certificates automatically.

### Tell VexGo About the Proxy

Set these so VexGo trusts proxy headers (real client IPs) and generates correct URLs:

```yaml
behind_reverse_proxy: true
trusted_proxies: ["192.168.1.100"] # or CIDRs, e.g. "10.0.0.0/8"
```

If `trusted_proxies` is empty, VexGo defaults to common private networks (`127.0.0.1`, `::1`, `192.168.0.0/16`, `10.0.0.0/8`, `172.16.0.0/12`).

Also set your public base URL (used for SSO redirects) via the environment variable:

```bash
BASE_URL=https://your-domain.com ./vexgo
```

or in your Docker/systemd environment.

---

## HTTPS with Let's Encrypt

With Nginx and Certbot:

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
```

Certbot configures TLS, sets up automatic renewal, and redirects HTTP to HTTPS.

---

## Running as a Service (systemd)

A systemd unit keeps VexGo running across reboots and crashes. See the [Installation guide](/guides/installation#step-5-run-as-a-systemd-service-optional) for the full unit file. Summary:

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

## Firewall

Open only the ports VexGo needs:

```bash
# UFW (Ubuntu/Debian)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 3001/tcp   # only if VexGo is directly exposed
sudo ufw enable

# firewalld (Fedora/CentOS)
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --reload
```

> **Tip:** if VexGo is behind a reverse proxy on the same machine, only ports 80/443 need to be open — the proxy talks to VexGo on localhost.

---

## Scaling to Multiple Instances

VexGo is a single binary. By default the content cache, rate-limiting budgets and OAuth login state live **in the process** — which is correct for one instance, but two or more instances behind a load balancer must share that state through a Valkey (Redis-compatible) server:

```yaml
cache_enabled: true # content cache; false = public reads always hit the database
valkey_enabled: true # required for multi-instance deployments
valkey_url: "valkey://valkey.internal:6379"
```

Every instance must point at the same Valkey server. Unlike `cache_enabled`, which needs no external server and falls back to the in-process memory cache, enabling `valkey_enabled` makes Valkey a hard dependency: VexGo verifies the connection with a PING at startup and refuses to start when the server is unreachable (fail-fast), so a broken URL surfaces immediately instead of at request time.

Operational notes:

- **Keep the server private**: loopback or a trusted network, a password in the URL, TLS via `rediss://` where appropriate. Everything in it is treated as trusted and served to users.
- **Configure eviction**: set a `maxmemory` limit with `allkeys-lru` — rate-limit keys rotate per client IP and cannot be bounded application-side.
- **Placement matters**: colocate Valkey with the app (same host, unix socket or host networking) or keep the network path low-RTT. For cheap reads (a single post), the cache roundtrip must be cheaper than the database query it replaces, or the cache is a net loss.
- The content cache stays on the in-process memory backend when `valkey_enabled: false` — see the [Configuration Guide](/guides/configuration#content-cache--valkey) for the full behavior matrix.

---

## Backups

VexGo's data lives in two places:

1. **The data directory** — SQLite database file and uploaded media (or media in S3 if enabled)
2. **The database** — for PostgreSQL/MySQL deployments

### Back up the data directory

```bash
tar -czf vexgo-data-$(date +%F).tar.gz /var/lib/vexgo
```

### Back up PostgreSQL

```bash
pg_dump -U vexgo_user vexgo_db > vexgo-db-$(date +%F).sql
```

Restore with `psql -U vexgo_user vexgo_db < backup.sql`.

Schedule backups with cron or systemd timers, and store them off-machine (e.g. an S3 bucket or another server).

> If `SETTINGS_ENCRYPTION_KEY` is set, back it up together with the database: a
> backup restored without the key cannot decrypt the stored SMTP password and
> API keys (the server keeps running; an admin must re-enter the secrets in the UI).

---

## Upgrading

> **Before upgrading:** back up your data directory and database. Restart after upgrading so database migrations run.

### Docker

```bash
docker pull ghcr.io/vexgo-org/vexgo:latest
docker stop vexgo && docker rm vexgo
docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data --restart unless-stopped ghcr.io/vexgo-org/vexgo:latest
```

### Binary

1. Download the new binary from the [Releases page](https://github.com/vexgo-org/vexgo/releases).
2. Stop the service: `sudo systemctl stop vexgo`
3. Replace the binary: `sudo mv vexgo /opt/vexgo/vexgo`
4. Start it again: `sudo systemctl start vexgo`

### Nix

```bash
nix flake update
sudo nixos-rebuild switch --flake .#myhost
```

---

## Troubleshooting

### Port 3001 is already in use

```bash
sudo lsof -i :3001          # find the process
sudo kill -9 <PID>          # or use a different port:
./vexgo --port 8080
```

### Permission denied

```bash
chmod +x vexgo
sudo chown -R $USER:$USER ./data
```

### Database connection issues

```bash
docker ps | grep postgres                        # is the DB running?
psql -h localhost -U vexgo -d vexgo_db           # test the connection
docker logs vexgo-postgres                        # DB logs
```

### Docker container won't start

```bash
docker logs vexgo
docker inspect vexgo
docker rm -f vexgo && docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/vexgo-org/vexgo:latest
```

### systemd service issues

```bash
sudo systemctl status vexgo
sudo journalctl -u vexgo -n 50
sudo systemctl restart vexgo
sudo systemctl cat vexgo
```

### Getting help

- [GitHub Issues](https://github.com/vexgo-org/vexgo/issues)
- [Discussions](https://github.com/vexgo-org/vexgo/discussions)
