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
postgres=# CREATE USER vexgo_user WITH PASSWORD 'test';
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
mysql>CREATE USER 'vexgo_user'@'%' IDENTIFIED BY '你的密码';
mysql>GRANT ALL ON vexgo_db.* TO 'vexgo_user'@'%';
mysql>FLUSH PRIVILEGES;
```

Run backend with this command:
```bash
go run main.go -c ../examples/config-mysql.yml
```