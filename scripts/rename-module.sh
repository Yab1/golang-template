#!/usr/bin/env bash
# Rename the Go module path across the repo (go.mod + imports + golangci local-prefix).
# Usage:
#   ./scripts/rename-module.sh github.com/acme/myapp
#   make rename MODULE=github.com/acme/myapp
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

NEW_MODULE="${1:-}"
if [[ -z "$NEW_MODULE" ]]; then
  echo "usage: $0 <new-module-path>" >&2
  echo "example: $0 github.com/acme/myapp" >&2
  exit 1
fi

OLD_MODULE="$(go list -m)"
if [[ -z "$OLD_MODULE" ]]; then
  echo "could not read current module from go.mod" >&2
  exit 1
fi

if [[ "$OLD_MODULE" == "$NEW_MODULE" ]]; then
  echo "already using module path: $OLD_MODULE"
  exit 0
fi

echo "renaming module: $OLD_MODULE -> $NEW_MODULE"

go mod edit -module "$NEW_MODULE"

# Rewrite import paths in Go sources and swagger-generated docs.
while IFS= read -r -d '' f; do
  sed -i "s|${OLD_MODULE}|${NEW_MODULE}|g" "$f"
done < <(find . -type f \( -name '*.go' -o -name '*.md' \) \
  ! -path './.git/*' ! -path './tmp/*' ! -path './temp/*' ! -path './bin/*' -print0)

if [[ -f .golangci.yml ]]; then
  sed -i "s|${OLD_MODULE}|${NEW_MODULE}|g" .golangci.yml
fi

echo "done. next: go mod tidy && make gen-docs"
