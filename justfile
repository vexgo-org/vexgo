format:
    # Run formatter.
    gofumpt -w backend/**/*.go
    prettier --write "**/*.{js,jsx,ts,tsx,html,md}"

lint:
    # Run linter.
    golangci-lint run
    prettier --check "**/*.{js,jsx,ts,tsx,html,md}"
    diffs="$(gofumpt -d .)"; test -z "$diffs" || { echo "$diffs"; exit 1; }
    eslint -c frontend/eslint.config.js frontend/
