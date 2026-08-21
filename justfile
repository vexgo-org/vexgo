set shell := ["bash", "-O", "globstar", "-c"]

format:
    # Run formatter.
    gofumpt -w -extra .
    prettier --write "**/*.{js,jsx,ts,tsx,html,md}"

lint:
    # Run linter.
    golangci-lint run
    prettier --check "**/*.{js,jsx,ts,tsx,html,md}"
    diffs="$(gofumpt -d .)"; test -z "$diffs" || { echo "$diffs"; exit 1; }
    oxlint -c frontend/.oxlintrc.json frontend/
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
