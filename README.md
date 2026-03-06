# VexGo
This is a blog built on React, Go, Gin, JWT, and SQLite, which implements features such as user registration and article management.

## How to run

### Requirements
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
go run main.go
```

If not set, the application will generate a random secret at startup for development purposes. Never use the default development secret in production.