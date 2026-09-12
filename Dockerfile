# Phase 1: Building the front ends (admin SPA + standalone default theme)
FROM oven/bun:1.3.13-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package.json frontend/bun.lock frontend/bunfig.toml ./
RUN bun install --frozen-lockfile

COPY frontend/ ./
RUN bun run build
# output: /app/backend/internal/public/dist

# The default theme lives in its own repository
# (https://github.com/vexgo-org/vexgo-default-theme); build it from source
# and copy its dist output to the path the backend embeds.
ARG DEFAULT_THEME_REF=main
WORKDIR /tmp/vexgo-default-theme
RUN apk add --no-cache git && \
    git clone --depth 1 --branch "${DEFAULT_THEME_REF}" https://github.com/vexgo-org/vexgo-default-theme.git . && \
    bun install --frozen-lockfile && \
    bun run build
# output: /tmp/vexgo-default-theme/dist

# Phase 2: Compiling the backend
FROM golang:1.26-alpine AS backend-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY backend/ ./backend/
COPY --from=frontend-builder /app/backend/internal/public/dist ./backend/internal/public/dist
COPY --from=frontend-builder /tmp/vexgo-default-theme/dist ./backend/internal/public/default-theme

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
