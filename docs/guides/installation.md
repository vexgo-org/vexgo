# Installation

> **How-to** — this guide shows you how to install VexGo on your own machine or server. Pick the method that fits your environment.

## Prerequisites

- **Operating system**: Linux, macOS, Windows, FreeBSD or any system with Docker
- **Memory**: 512 MB minimum, 1 GB recommended
- **Disk**: at least 100 MB for the application, plus space for your data

| Method                                                  | Requires                  | Best for                              |
| ------------------------------------------------------- | ------------------------- | ------------------------------------- |
| [Binary](#method-1-binary-installation)                 | A downloaded executable   | Quick local runs, VPS deployment      |
| [Docker](#method-2-docker-installation)                 | Docker                    | Single-container deployment           |
| [Docker Compose](#method-3-docker-compose-installation) | Docker + Docker Compose   | Multi-service setups (DB, etc.)       |
| [Nix](#method-4-nix-installation)                       | Nix package manager       | Trying instantly, reproducible setups |
| [NixOS Flake](#method-5-nixos-flake-installation)       | NixOS with flakes enabled | NixOS systems                         |
| [From Source](#method-6-building-from-source)           | Go 1.25+, Node.js, pnpm   | Development, custom builds            |

---

## Method 1: Binary Installation

The simplest method — download a pre-compiled binary and run it.

### Step 1: Download the Binary

Visit the [Releases page](https://github.com/vexgo-org/vexgo/releases) and download the binary for your system and architecture.

For most 64-bit Linux systems:

```bash
curl -L $(curl -s https://api.github.com/repos/vexgo-org/vexgo/releases/latest | grep browser_download_url | grep linux-amd64 | cut -d '"' -f 4) -o vexgo
chmod +x vexgo
```

For ARM systems (Raspberry Pi, ARM servers):

```bash
curl -L $(curl -s https://api.github.com/repos/vexgo-org/vexgo/releases/latest | grep browser_download_url | grep linux-arm64 | cut -d '"' -f 4) -o vexgo
chmod +x vexgo
```

### Step 2: Create a Data Directory

```bash
mkdir -p ./data
```

### Step 3: Run VexGo

```bash
./vexgo
```

VexGo starts on `http://0.0.0.0:3001` by default.

### Step 4: Run with Custom Options

```bash
# Custom port and data directory
./vexgo --port 8080 --data /path/to/data

# Custom listen address
./vexgo --addr 127.0.0.1

# Load a config file
./vexgo -c /path/to/config.yml

# See all available options
./vexgo --help
```

### Step 5: Run as a systemd Service (Optional)

Create `/etc/systemd/system/vexgo.service`:

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

Create a dedicated user and enable the service:

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

## Method 2: Docker Installation

### Step 1: Pull and Run VexGo

```bash
docker pull ghcr.io/vexgo-org/vexgo:latest

docker run -d \
  --name vexgo \
  -p 3001:3001 \
  -v ./data:/app/data \
  --restart unless-stopped \
  ghcr.io/vexgo-org/vexgo:latest
```

### Step 2: Verify

```bash
docker ps                 # container status
docker logs vexgo         # container logs
```

### Step 3: Run with Custom Configuration

```bash
docker run -d \
  --name vexgo \
  -p 3001:3001 \
  -v ./data:/app/data \
  -v ./config.yml:/app/config.yml:ro \
  -e ADDR=0.0.0.0 \
  -e PORT=3001 \
  -e JWT_SECRET=your-secret-key-change-this-in-production \
  -e SETTINGS_ENCRYPTION_KEY=your-very-long-random-secret-here-change-this-in-production \
  --restart unless-stopped \
  ghcr.io/vexgo-org/vexgo:latest
```

### Common Docker Commands

```bash
docker stop vexgo
docker start vexgo
docker restart vexgo
docker rm -f vexgo

# Update to the latest version
docker pull ghcr.io/vexgo-org/vexgo:latest
docker stop vexgo && docker rm vexgo
docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data --restart unless-stopped ghcr.io/vexgo-org/vexgo:latest
```

---

## Method 3: Docker Compose Installation

Docker Compose is ideal when VexGo runs alongside PostgreSQL or MySQL.

### Step 1: Create `docker-compose.yml`

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
      - SETTINGS_ENCRYPTION_KEY=your-very-long-random-secret-here-change-this-in-production
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

### Step 2: Start the Services

```bash
mkdir -p data postgres
docker compose up -d

docker compose logs -f vexgo   # view logs
docker compose ps              # check status
```

### Common Compose Commands

```bash
docker compose stop
docker compose start
docker compose restart
docker compose down            # stop and remove containers
docker compose down -v         # also remove volumes (deletes data!)
```

---

## Method 4: Nix Installation

### Step 1: Install Nix

```bash
curl -L https://nixos.org/nix/install | sh
source ~/.nix-profile/etc/profile.d/nix.sh
```

### Step 2: Run VexGo Directly

```bash
# Run without installing (fetches from GitHub)
nix run github:vexgo-org/vexgo
```

### Step 3: Install Permanently

```bash
nix profile install github:vexgo-org/vexgo
vexgo
```

### Step 4: Run with Custom Options

```bash
nix run github:vexgo-org/vexgo -- -c /path/to/config.yml
nix run github:vexgo-org/vexgo -- --port 8080 --addr 0.0.0.0
```

---

## Method 5: NixOS Flake Installation

### Step 1: Enable Flakes

Add to `/etc/nixos/configuration.nix`:

```nix
{ config, pkgs, ... }:
{
  nix.settings.experimental-features = [ "nix-command" "flakes" ];
}
```

Rebuild:

```bash
sudo nixos-rebuild switch
```

### Step 2: Add VexGo to Your Flake

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

### Step 3: Create `vexgo.nix`

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
      settings_encryption_key = "your-very-long-random-secret-here-change-this-in-production";
      log_level = "info";
    };
  };

  # Optional: use PostgreSQL instead of SQLite
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

### Step 4: Rebuild and Manage

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

## Method 6: Building from Source

Use this when you want the latest development version or need to customize the code.

### Step 1: Install Build Dependencies

- **Go 1.25+**
- **Node.js** and **pnpm 10**
- Optional (recommended): `just`, `gofumpt`, `golangci-lint`, `prettier`, `oxlint` — a Nix dev shell with all of them is available via `nix develop`

### Step 2: Clone and Build

```bash
git clone https://github.com/vexgo-org/vexgo.git
cd vexgo

# Build the frontend (output is embedded into the backend binary)
cd frontend
pnpm install
pnpm run build
cd ..

# Build the backend
cd backend
go mod download
go build -o vexgo ./cmd/vexgo
cd ..
```

### Step 3: Run

```bash
./backend/vexgo
```

### Development Commands

```sh
just format            # gofumpt -w -extra . && prettier --write
just lint              # golangci-lint + prettier --check + gofumpt check + oxlint
go build -v ./...      # build the backend
go test -v ./...       # run backend tests
```

The frontend build output is written to `backend/internal/public/dist` and embedded into the backend binary, so rebuild the frontend after changing it.

---

## After Installation

### Access the Site

Open `http://localhost:3001` (or `http://your-server-ip:3001` on a remote server).

### Default Credentials

| Field    | Value               |
| -------- | ------------------- |
| Email    | `admin@example.com` |
| Password | `password`          |

> **⚠️ Important:** change the default password immediately after your first login — see the [Quick Start](/getting-started) tutorial.

### Verify It Works

1. Log in with the default account.
2. Create a test post and publish it.
3. Check that it appears on the home page.

### Next Steps

- [Configuration Guide](/guides/configuration) — set up a database, SSO, S3, and email
- [Production Deployment](/guides/deployment) — reverse proxy, HTTPS, systemd
- [Troubleshooting](/guides/deployment#troubleshooting) — common problems and fixes
