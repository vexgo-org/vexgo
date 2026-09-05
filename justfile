set shell := ["bash", "-O", "globstar", "-c"]

format:
    # Run formatter.
    gofumpt -w -extra .
    prettier --write "**/*.{js,jsx,ts,tsx,html,md}" "frontend/*.json" "./*.{json,yml,yaml}"

lint:
    # Run linter.
    golangci-lint run
    output=$(deadcode -test ./...); test -z "$output" || { echo "$output"; exit 1;}
    prettier --check "**/*.{js,jsx,ts,tsx,html,md}" "frontend/*.json" "./*.{json,yml,yaml}"
    diffs="$(gofumpt -d .)"; test -z "$diffs" || { echo "$diffs"; exit 1; }
    oxlint --deny-warnings -c frontend/.oxlintrc.json frontend/
    output=$(gopls check -severity=hint ./**/*.go); test -z "$output" || { echo "$output"; exit 1;}

test:
    # Run tests.
    go test -v ./...

run:
    # Run VexGo.
    just ensure-dist
    go run backend/cmd/vexgo/main.go

build:
    # Build VexGo.
    just build-frontend
    just build-backend

build-frontend:
    # Build frontend.
    pnpm --dir frontend run build

build-backend:
    # Build backend.
    just ensure-dist
    go build backend/cmd/vexgo/main.go

@ensure-dist:
    # Ensure `backend/internal/public/dist` directory exists.
    test -d backend/internal/public/dist || just build-frontend

sync-openapi:
    # Regenerate docs/openapi.json from the live huma registry.
    # Run this after any handler signature change so the
    # committed spec matches the code.
    go run ./backend/cmd/openapi-spec

sync-frontend-api:
    # Regenerate the orval typed client from docs/openapi.json.
    # Run this after `sync-openapi` so the types line up.
    pnpm --dir frontend exec orval --config orval.config.ts

check-openapi-fresh:
    #!/usr/bin/env bash
    # CI guard: fail if docs/openapi.json is out of date with
    # the current huma registry, or if the generated client
    # is out of date with the spec. Catches drift between the
    # backend and the typed frontend.
    set -euo pipefail
    tmp=$(mktemp)
    trap "rm -f $tmp" EXIT
    go run ./backend/cmd/openapi-spec -o "$tmp"
    if ! diff -q docs/openapi.json "$tmp" >/dev/null 2>&1; then
        echo "docs/openapi.json is stale. Run: just sync-openapi"
        diff docs/openapi.json "$tmp" | head -50
        exit 1
    fi
    echo "openapi.json: fresh"
