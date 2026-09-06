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

generate:
    # Codegen using swag and orval.
    go tool swag init -g backend/cmd/vexgo/main.go --dir . --v3.1 -o ./docs --ot json
    pnpm --dir frontend exec orval --config orval.config.ts

check-openapi-fresh:
    #!/usr/bin/env bash
    # CI guard: fail if docs/swagger.json is stale relative to
    # the swaggo annotations in the backend. Catches drift
    # between the code and the OpenAPI spec.
    set -euo pipefail
    tmp=$(mktemp)
    trap "rm -f $tmp" EXIT
    (cd backend && swag init -g cmd/vexgo/main.go --dir . --v3.1 -o "$tmp" --ot json >/dev/null)
    # swag only writes swagger.json to the output dir; rename the
    # tmp's swagger.json to be diffable.
    if ! diff -q docs/swagger.json "$tmp/swagger.json" >/dev/null 2>&1; then
        echo "docs/swagger.json is stale. Run: just sync-openapi"
        diff docs/swagger.json "$tmp/swagger.json" | head -80
        exit 1
    fi
    echo "swagger.json: fresh"
