set shell := ["bash", "-O", "globstar", "-c"]

format:
    # Run formatter.
    gofumpt -w -extra .
    prettier --write "**/*.{js,jsx,ts,tsx,html,md}" "frontend/*.json" "./*.{json,yml,yaml}"
    go tool swag fmt backend/

lint:
    # Run linter.
    golangci-lint run
    output=$(deadcode -test ./...); test -z "$output" || { echo "$output"; exit 1;}
    prettier --check "**/*.{js,jsx,ts,tsx,html,md}" "frontend/*.json" "./*.{json,yml,yaml}"
    diffs="$(gofumpt -d .)"; test -z "$diffs" || { echo "$diffs"; exit 1; }
    oxlint -c frontend/.oxlintrc.json frontend/
    output=$(gopls check -severity=hint ./**/*.go); test -z "$output" || { echo "$output"; exit 1;}
    just check-swag-fmt
    just check-openapi-fresh

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
    # the swaggo annotations in the backend.
    set -euo pipefail

    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT

    cd backend
    swag_output=$(
        go tool swag init \
            -g cmd/vexgo/main.go \
            --dir . \
            --v3.1 \
            -o "$tmp" \
            --ot json 2>&1 >/dev/null
    )
    if [[ -n "$swag_output" ]]; then
        echo "$swag_output" >&2
        exit 1
    fi

    if ! diff -q ../docs/swagger.json "$tmp/swagger.json" >/dev/null 2>&1; then
        echo "docs/swagger.json is stale. Run: just generate"
        diff ../docs/swagger.json "$tmp/swagger.json" | head -80
        exit 1
    fi

check-swag-fmt:
    #!/usr/bin/env bash
    set -euo pipefail

    tmp=$(mktemp -d)
    trap 'git worktree remove --force "$tmp" 2>/dev/null || true' EXIT
    git worktree add --detach "$tmp" HEAD >/dev/null
    (
        cd "$tmp/backend"
        go tool swag fmt
    )

    if ! git -C "$tmp" diff --quiet -- '*.go'; then
        echo "Swag annotations are not formatted."
        echo "Run: just format"
        echo
        git -C "$tmp" diff -- '*.go'
        exit 1
    fi
