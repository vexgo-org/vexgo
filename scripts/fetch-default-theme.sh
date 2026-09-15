#!/usr/bin/env bash
#
# Fetches and builds the standalone default theme
# (https://github.com/vexgo-org/vexgo-default-theme) into
# backend/internal/public/default-theme/, which the backend embeds via
# go:embed. The theme source used to live in frontend-public/; it now lives
# in its own repository, so every environment that compiles or typechecks the
# backend (CI, Docker, local builds) must run this first.
#
# Usage:
#   scripts/fetch-default-theme.sh [--force]
#
# Without --force the script is a no-op when the target already contains a
# built theme (used by `just ensure-dist`). With --force it always rebuilds.
#
# Environment overrides:
#   DEFAULT_THEME_REPO  theme git URL (default: the GitHub repo above)
#   DEFAULT_THEME_REF   branch/tag/commit to build (default: main)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="$REPO_ROOT/backend/internal/public/default-theme"
THEME_REPO="${DEFAULT_THEME_REPO:-https://github.com/vexgo-org/vexgo-default-theme.git}"
THEME_REF="${DEFAULT_THEME_REF:-main}"

FORCE=0
if [[ "${1:-}" == "--force" ]]; then
	FORCE=1
fi

if [[ "$FORCE" -eq 0 && -f "$TARGET/index.html" ]]; then
	echo "default theme already present at backend/internal/public/default-theme, skipping (use --force to rebuild)"
	exit 0
fi

if ! command -v bun >/dev/null 2>&1; then
	echo "error: bun is required to build the default theme" >&2
	exit 1
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "cloning default theme ($THEME_REPO @ $THEME_REF)..."
git clone --depth 1 --branch "$THEME_REF" "$THEME_REPO" "$WORK_DIR/theme" >&2

echo "building default theme..."
(
	cd "$WORK_DIR/theme"
	bun install --frozen-lockfile >&2
	bun run build >&2
)

rm -rf "$TARGET"
mkdir -p "$TARGET"
cp -r "$WORK_DIR/theme/dist/." "$TARGET/"

echo "default theme written to backend/internal/public/default-theme"
