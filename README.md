# VexGo
This is a blog built on React, Go, Gin, JWT, and SQLite, which implements features such as user registration and article management.

## Quick Start

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

### Security Configuration

For production deployment, it's highly recommended to set the `JWT_SECRET` environment variable to a strong, random secret:

```bash
export JWT_SECRET="your-very-long-random-secret-here"
```

If not set, the application will generate a random secret at startup for development purposes. Never use the default development secret in production.


## Config File

Configuration priority: command-line arguments > configuration files > environment variables > default values

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

## Docker Deployment

### Using Docker Compose (Recommended)

The easiest way to deploy VexGo is using Docker Compose, which will build the application and run it with SQLite by default.

1. **Build and start the application:**

```bash
# Build and start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

2. **Configure environment variables:**

Copy `.env.example` to `.env` and modify as needed:

```bash
cp .env.example .env
```

Important: **Set a strong `JWT_SECRET` in production!**

3. **Access the application:**

Visit http://localhost:3001

4. **Data persistence:**

All data (SQLite database and uploaded files) are stored in the `./data` directory on the host machine.

### Using External Database with Docker Compose

If you want to use MySQL or PostgreSQL instead of SQLite:

1. **Uncomment the database service** in `docker-compose.yml` (either `mysql` or `postgres` section)
2. **Configure environment variables** in the `vexgo` service:
   - For MySQL: set `DB_TYPE=mysql` and ensure the database credentials match
   - For PostgreSQL: set `DB_TYPE=postgres` and ensure the database credentials match
3. **Update the database service** with your preferred root password
4. **Start all services:**

```bash
docker-compose up -d
```

The application will automatically connect to the database service via Docker network.

### Building Docker Image Manually

If you prefer to build the Docker image without Docker Compose:

```bash
# Build the image
docker build -t vexgo:latest .

# Run the container
docker run -d \
  --name vexgo \
  -p 3001:3001 \
  -e JWT_SECRET="your-secret-key" \
  -v $(pwd)/data:/app/data \
  vexgo:latest
```

### Docker Image Structure

The Dockerfile uses a multi-stage build:

1. **Frontend builder stage**: Uses Node.js to build the React frontend
2. **Backend builder stage**: Uses Go to compile the backend with embedded frontend
3. **Runtime stage**: Uses lightweight Alpine Linux with only necessary dependencies

This ensures the final image is small and contains only the compiled binary and runtime dependencies.

### Production Considerations

1. **JWT_SECRET**: Always set a strong, random secret in production
2. **Reverse proxy**: Consider using Nginx or Traefik as reverse proxy for SSL termination
3. **Backups**: Regularly backup the `data` directory (contains SQLite database and uploads)
4. **Updates**: To update, rebuild the image and restart the container:
   ```bash
   docker-compose pull  # if using base images updates
   docker-compose build
   docker-compose up -d
   ```