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
