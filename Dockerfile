# Phase 1: Building the front end
FROM node:25-alpine AS frontend-builder

RUN npm install -g pnpm

WORKDIR /app/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install

COPY frontend/ ./
RUN pnpm run build
# output: /app/backend/internal/public/dist

# Phase 2: Compiling the backend
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY backend/ ./backend/
COPY --from=frontend-builder /app/backend/internal/public/dist ./backend/internal/public/dist

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o vexgo ./backend/cmd/vexgo

# Phase 3: Final Mirror
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=backend-builder /app/vexgo .

# expose port
EXPOSE 3001

# Set environment
ENV ADDR=0.0.0.0
ENV PORT=3001
ENV DATA_DIR=/app/data

CMD ["./vexgo"]
