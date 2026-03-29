# Installation Guide for Linux

This guide provides detailed instructions for installing VexGo on Linux systems using various methods.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Method 1: Binary Installation](#method-1-binary-installation)
- [Method 2: Docker Installation](#method-2-docker-installation)
- [Method 3: Docker Compose Installation](#method-3-docker-compose-installation)
- [Method 4: Nix Installation](#method-4-nix-installation)
- [Method 5: NixOS Flake Installation](#method-5-nixos-flake-installation)
- [Method 6: Building from Source](#method-6-building-from-source)
- [Post-Installation](#post-installation)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before installing VexGo, ensure your system meets the following requirements:

- **Operating System**: Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+, Fedora 35+, Arch Linux, or other modern distributions)
- **Memory**: Minimum 512MB RAM (1GB recommended)
- **Disk Space**: Minimum 100MB for application, additional space for data storage
- **Network**: Internet connection for downloading dependencies and OAuth providers

### Optional Dependencies

- **Docker**: For container-based installation (Method 2 & 3)
- **Docker Compose**: For multi-container setups (Method 3)
- **Nix**: For Nix-based installation (Method 4)
- **Go 1.21+**: For building from source (Method 6)
- **Node.js 18+ & pnpm**: For building from source (Method 6)

## Method 1: Binary Installation

This is the simplest method for most users. Download the pre-compiled binary and run it directly.

### Step 1: Download the Binary

Visit the [VexGo Releases page](https://github.com/vexgo-org/vexgo/releases) and download the appropriate binary for your system architecture.

For most modern Linux systems (64-bit x86):

```bash
# Download the latest release automatically
curl -L $(curl -s https://api.github.com/repos/vexgo-org/vexgo/releases/latest | grep browser_download_url | grep linux-amd64 | cut -d '"' -f 4) -o vexgo

# Make executable
chmod +x vexgo
```

For ARM-based systems (Raspberry Pi, ARM servers):

```bash
curl -L $(curl -s https://api.github.com/repos/vexgo-org/vexgo/releases/latest | grep browser_download_url | grep linux-arm64 | cut -d '"' -f 4) -o vexgo
chmod +x vexgo
```

Other supported architectures may be available - check releases page for complete list.

### Step 2: Create a Data Directory

```bash
mkdir -p ./data
```

### Step 3: Run VexGo

```bash
./vexgo
```

VexGo will start on `http://0.0.0.0:3001` by default.

### Step 4: (Optional) Run with Custom Parameters

You can customize the configuration with command-line arguments:

```bash
# Run with custom port and data directory
./vexgo --port 8080 --data /path/to/data

# Run with custom address
./vexgo --addr 127.0.0.1

# Get help with available options
./vexgo --help
```

### Step 5: (Optional) Install as a System Service

To run VexGo as a background service that starts automatically:

Create a systemd service file:

```bash
sudo nano /etc/systemd/system/vexgo.service
```

Add the following content:

```ini
[Unit]
Description=VexGo Blog CMS
After=network.target

[Service]
Type=simple
User=vexgo
Group=vexgo
WorkingDirectory=/opt/vexgo
ExecStart=/opt/vexgo/vexgo-linux-amd64
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Create a dedicated user and setup directories:

```bash
sudo useradd -r -s /bin/false vexgo
sudo mkdir -p /opt/vexgo /var/lib/vexgo
sudo chown -R vexgo:vexgo /opt/vexgo /var/lib/vexgo
sudo mv vexgo /opt/vexgo/
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable vexgo
sudo systemctl start vexgo
sudo systemctl status vexgo
```

## Method 2: Docker Installation

Docker provides a containerized environment that isolates VexGo from your host system.

### Step 1: Install Docker

If you don't have Docker installed:

```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Fedora/CentOS
sudo dnf install docker
sudo systemctl start docker
sudo systemctl enable docker

# Arch Linux
sudo pacman -S docker
sudo systemctl start docker
sudo systemctl enable docker
```

### Step 2: Pull and Run VexGo

```bash
# Pull the latest image
sudo docker pull ghcr.io/vexgo-org/vexgo:latest

# Run the container
sudo docker run -d \
  --name vexgo \
  -p 3001:3001 \
  -v ./data:/app/data \
  --restart unless-stopped \
  ghcr.io/vexgo-org/vexgo:latest
```

### Step 3: Verify the Installation

```bash
# Check container status
sudo docker ps

# View logs
sudo docker logs vexgo
```

### Step 4: (Optional) Run with Custom Configuration

```bash
sudo docker run -d \
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

### Step 5: (Optional) Docker Management Commands

```bash
# Stop the container
sudo docker stop vexgo

# Start the container
sudo docker start vexgo

# Restart the container
sudo docker restart vexgo

# Remove the container
sudo docker rm -f vexgo

# Update to the latest version
sudo docker pull ghcr.io/vexgo-org/vexgo:latest
sudo docker stop vexgo
sudo docker rm vexgo
sudo docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data --restart unless-stopped ghcr.io/vexgo-org/vexgo:latest
```

## Method 3: Docker Compose Installation

Docker Compose is ideal for running VexGo with additional services like PostgreSQL or MySQL.

### Step 1: Install Docker Compose

```bash
# Linux
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Or using pip
pip install docker-compose
```

### Step 2: Create a docker-compose.yml File

Create a file named `docker-compose.yml`:

```yaml
version: '3.8'

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

  # Optional: MySQL alternative
  # mysql:
  #   image: mysql:8.0
  #   container_name: vexgo-mysql
  #   environment:
  #     - MYSQL_ROOT_PASSWORD=root_password
  #     - MYSQL_DATABASE=vexgo_db
  #     - MYSQL_USER=vexgo
  #     - MYSQL_PASSWORD=vexgo_password
  #   volumes:
  #     - ./mysql:/var/lib/mysql
  #   restart: unless-stopped

volumes:
  postgres:
  mysql:
```

### Step 3: Start the Services

```bash
# Create necessary directories
mkdir -p data postgres mysql

# Start all services
sudo docker-compose up -d

# View logs
sudo docker-compose logs -f vexgo

# Check status
sudo docker-compose ps
```

### Step 4: Docker Compose Management Commands

```bash
# Stop all services
sudo docker-compose stop

# Start all services
sudo docker-compose start

# Restart all services
sudo docker-compose restart

# Stop and remove all containers
sudo docker-compose down

# Stop and remove all containers and volumes
sudo docker-compose down -v

# Rebuild and restart
sudo docker-compose up -d --build

# View logs for a specific service
sudo docker-compose logs -f postgres
```

## Method 4: Nix Installation

Nix allows you to try VexGo instantly without permanent installation.

### Step 1: Install Nix

```bash
# Install Nix package manager
curl -L https://nixos.org/nix/install | sh

# Source Nix
source ~/.nix-profile/etc/profile.d/nix.sh
```

### Step 2: Run VexGo with Nix

```bash
# Run VexGo directly from GitHub
nix run github:vexgo-org/vexgo
```

### Step 3: (Optional) Install Permanently

```bash
# Add to your Nix profile
nix profile install github:vexgo-org/vexgo

# Run from anywhere
vexgo
```

### Step 4: (Optional) Use with Configuration

```bash
# Run with custom config
nix run github:vexgo-org/vexgo -- -c /path/to/config.yml

# Run with command-line arguments
nix run github:vexgo-org/vexgo -- --port 8080 --addr 0.0.0.0
```

## Method 5: NixOS Flake Installation

For NixOS users, you can integrate VexGo as a system service using flakes.

### Step 1: Enable Flakes

Add to your `/etc/nixos/configuration.nix`:

```nix
{ config, pkgs, ... }:
{
  nix.settings.experimental-features = [ "nix-command" "flakes" ];
}
```

Rebuild your system:

```bash
sudo nixos-rebuild switch
```

### Step 2: Update Your Flake

Add VexGo to your `flake.nix`:

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

### Step 3: Create VexGo Configuration

Create `vexgo.nix`:

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
      
      # Database configuration
      db_type = "sqlite";
      # db_type = "postgres";
      # db_host = "127.0.0.1";
      # db_port = 5432;
      # db_user = "vexgo";
      # db_password = "password";
      # db_name = "vexgo";
    };
  };
  
  # Optional: Configure PostgreSQL
  services.postgresql = {
    enable = true;
    ensureDatabases = [ "vexgo" ];
    ensureUsers = [
      {
        name = "vexgo";
        ensureDBOwnership = true;
      }
    ];
  };
  
  # Open firewall port
  networking.firewall.allowedTCPPorts = [ 3001 ];
}
```

### Step 4: Rebuild Your System

```bash
# Update flake inputs
sudo nix flake update

# Rebuild system
sudo nixos-rebuild switch --flake .#myhost

# Check service status
sudo systemctl status vexgo
```

### Step 5: Service Management

```bash
# Start service
sudo systemctl start vexgo

# Stop service
sudo systemctl stop vexgo

# Restart service
sudo systemctl restart vexgo

# Enable service at boot
sudo systemctl enable vexgo

# View logs
sudo journalctl -u vexgo -f
```

## Method 6: Building from Source

Build VexGo from source if you need to customize the code or use the latest development version.

### Step 1: Install Build Dependencies

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y golang git nodejs npm

# Install pnpm
npm install -g pnpm

# Fedora/CentOS
sudo dnf install -y golang git nodejs npm
npm install -g pnpm

# Arch Linux
sudo pacman -S go git nodejs npm pnpm
```

### Step 2: Clone the Repository

```bash
git clone https://github.com/vexgo-org/vexgo.git
cd vexgo
```

### Step 3: Build the Frontend

```bash
cd frontend
pnpm install
pnpm run build
cd ..
```

### Step 4: Build the Backend

```bash
cd backend
go mod download
go build -o vexgo ./cmd/server
cd ..
```

### Step 5: Run VexGo

```bash
./vexgo
```

### Step 6: (Optional) Create a Production Build

```bash
# Build with version information and optimizations
VERSION=$(git describe --tags --always)
CGO_ENABLED=0 go build \
  -ldflags="-s -w -X main.Version=${VERSION}" \
  -o vexgo \
  ./backend

# Make executable
chmod +x vexgo
```

### Step 7: (Optional) Install System-wide

```bash
# Copy binary to system path
sudo cp vexgo /usr/local/bin/vexgo

# Create data directory
sudo mkdir -p /var/lib/vexgo
sudo chown $USER:$USER /var/lib/vexgo

# Create systemd service
sudo nano /etc/systemd/system/vexgo.service
```

Add the following:

```ini
[Unit]
Description=VexGo Blog CMS
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=/var/lib/vexgo
ExecStart=/usr/local/bin/vexgo
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable vexgo
sudo systemctl start vexgo
```

## Post-Installation

### Access VexGo

After installation, open your web browser and navigate to:

http://localhost:3001

Or if running on a remote server:

http://your-server-ip:3001


### Default Credentials

The default super admin account:

- **Email**: `admin@example.com`
- **Password**: `password`

**⚠️ Important**: Change the default password immediately after first login!

### Change Default Password

1. Log in with the default credentials
2. Navigate to your profile page
3. Change your password
4. Save the changes

### Configure Reverse Proxy (Optional)

For production use, it's recommended to use a reverse proxy like Nginx.

#### Nginx Configuration

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

Enable SSL with Let's Encrypt:

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
```

### Configure Firewall

```bash
# UFW (Ubuntu/Debian)
sudo ufw allow 3001/tcp
sudo ufw enable

# firewalld (Fedora/CentOS)
sudo firewall-cmd --permanent --add-port=3001/tcp
sudo firewall-cmd --reload

# iptables
sudo iptables -A INPUT -p tcp --dport 3001 -j ACCEPT
```

## Troubleshooting

### Port Already in Use

If port 3001 is already in use:

```bash
# Find the process using port 3001
sudo lsof -i :3001

# Kill the process
sudo kill -9 <PID>

# Or use a different port
./vexgo-linux-amd64 --port 8080
```

### Permission Denied

If you encounter permission errors:

```bash
# Fix file permissions
chmod +x vexgo-linux-amd64

# Fix directory permissions
sudo chown -R $USER:$USER ./data
```

### Database Connection Issues

For PostgreSQL/MySQL connection problems:

```bash
# Check if database is running
sudo docker ps | grep postgres

# Test database connection
psql -h localhost -U vexgo -d vexgo_db

# Check database logs
sudo docker logs vexgo-postgres
```

### Docker Container Won't Start

```bash
# Check container logs
sudo docker logs vexgo

# Inspect container
sudo docker inspect vexgo

# Remove and recreate
sudo docker rm -f vexgo
sudo docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/vexgo-org/vexgo:latest
```

### Systemd Service Issues

```bash
# Check service status
sudo systemctl status vexgo

# View service logs
sudo journalctl -u vexgo -n 50

# Restart service
sudo systemctl restart vexgo

# Check service configuration
sudo systemctl cat vexgo
```

### Performance Issues

If VexGo is running slowly:

1. **Check system resources**:
   ```bash
   htop
   df -h
   ```

2. **Optimize database**:
   ```bash
   # For PostgreSQL
   sudo docker exec -it vexgo-postgres psql -U vexgo -d vexgo_db -c "VACUUM ANALYZE;"
   ```

3. **Increase log level** for debugging:
   ```bash
   ./vexgo --log-level debug
   ```

### Getting Help

If you encounter issues not covered here:

- Check the [GitHub Issues](https://github.com/vexgo-org/vexgo/issues)
- Review the [Documentation](https://github.com/vexgo-org/vexgo/tree/main/docs)
- Join the [Discussions](https://github.com/vexgo-org/vexgo/discussions)

## Additional Resources

- [Configuration Guide](configuration.md) - Detailed configuration options
- [User Manual](user-manual.md) - Complete usage instructions
- [API Documentation](api.md) - REST API reference
- [Development Guide](development.md) - Contributing to VexGo

---

**Note**: This documentation is maintained with the latest version of VexGo. For version-specific instructions, please refer to the release notes.
- Review the [main README](https://github.com/vexgo-org/vexgo)
- Join the community discussions

## Additional Resources

- [Main Documentation](README.md)
- [API Documentation](api.md)
- [Configuration Guide](config.md)
- [GitHub Repository](https://github.com/vexgo-org/vexgo)
- [Docker Hub](https://hub.docker.com/r/vexgo-org/vexgo)

## License

VexGo is released under the MIT License. See [LICENSE](LICENSE) for details.