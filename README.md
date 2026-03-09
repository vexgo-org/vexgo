# VexGo
This is a blog built on React, Go, Gin, JWT, and SQLite, which implements features such as user registration and article management.

## Quick Start

Select the corresponding system and architecture on the release page to download.

### Linux

```bash
./vexgo-linux-amd64
```

### Docker

```bash
sudo docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/antipeth/vexgo:latest
```

Then, visit http://127.0.0.1:3001

The Default super admin account: `admin@example.com`

The Default super admin password: `password`

You can change your account password on your profile page.

## Configuration

Configuration priority: command-line arguments > configuration files > environment variables > default values

### Use config file
Here is example config file:

```config.yml
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

Then, Run the following command:

```bash
./vexgo-linux-amd64 -c /the/path/to/config.yml
```
### Use environment

You can also configure the application using environment variables. The environment variable names correspond to the configuration keys in uppercase with underscores.

Available environment variables:

- `ADDR`: Server listen address (default: "0.0.0.0")
- `PORT`: Server listen port (default: 3001)
- `DATA`: Data directory path (default: "./data")
- `JWT_SECRET`: JWT secret key (required for production)
- `DB_TYPE`: Database type, options: "sqlite", "mysql", or "postgres" (default: "sqlite")
- `DB_HOST`: Database host (required for mysql/postgres)
- `DB_PORT`: Database port (required for mysql/postgres)
- `DB_USER`: Database username (required for mysql/postgres)
- `DB_PASSWORD`: Database password (required for mysql/postgres)
- `DB_NAME`: Database name (required for mysql/postgres)
- `DB_SSL_MODE`: SSL mode for postgres (default: "disable")

Example using environment variables:

```bash
export PORT=3001
export DB_TYPE=mysql
export DB_HOST=127.0.0.1
export DB_PORT=3306
export DB_USER=vexgo_user
export DB_PASSWORD=password
export DB_NAME=vexgo_db
./vexgo-linux-amd64
```

or

```bash
sudo docker run -d --name vexgo \
  -p 3001:3001 \
  -e PORT=3001 \
  -e DB_TYPE=sqlite \
  -v ./data:/app/data \ 
  ghcr.io/antipeth/vexgo:latest
```

## Database

### Postgres

Recommend Version: Postgres 18

To use postgres.

First, you run a postgres instance.

```bash
sudo docker run -d --name postgres -e POSTGRES_PASSWORD=test -p 5432:5432 -v ./postgres:/var/lib/postgresql/data docker.io/library/postgres:18-alpine
```

Then, enter postgres shell.

```bash
psql -U postgres
postgres=# CREATE USER vexgo_user WITH PASSWORD 'password';
postgres=# CREATE DATABASE vexgo_db OWNER vexgo_user ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C' TEMPLATE template0;
```

Run backend with this command:
```bash
go run main.go -c ../examples/config-postgres.yml
```

### Mysql

Recommend Version: Mysql 8

To use mysql.

First, you run a mysql instance.

```bash
sudo docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=test -v ./mysql:/var/lib/mysql docker.io/library/mysql:8
```

Then, enter mysql shell.

```bash
mysql -p
mysql>CREATE DATABASE vexgo_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
mysql>CREATE USER 'vexgo_user'@'%' IDENTIFIED BY 'password';
mysql>GRANT ALL ON vexgo_db.* TO 'vexgo_user'@'%';
mysql>FLUSH PRIVILEGES;
```

Run backend with this command:
```bash
go run main.go -c ../examples/config-mysql.yml
```

## Development

### Requirements
- Linux/MacOS
- go
- nodejs
- pnpm

### Steps
```
git clone https://github.com/weimm16/vexgo.git
cd vexgo/frontend
pnpm install
pnpm run build
cd ../backend
go run main.go
```
Then, visit http://127.0.0.1:3001

