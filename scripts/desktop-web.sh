#!/bin/sh
# Build and test the isolated desktop web module from any working directory.
set -eu

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
repo_root=$(CDPATH= cd "$script_dir/.." && pwd)
web_root="$repo_root/desktop/webapp"
export npm_config_cache="${MINI_ORCA_NPM_CACHE:-$repo_root/build/npm-cache}"

case "${1:-}" in
  test|build|run|ui) action=$1 ;;
  *) printf 'Usage: %s {test|build|run|ui}\n' "$0" >&2; exit 2 ;;
esac

command -v node >/dev/null || { printf 'Node.js 22+ is required.\n' >&2; exit 1; }
command -v npm >/dev/null || { printf 'npm is required.\n' >&2; exit 1; }
cd "$web_root/frontend"
npm ci --no-fund
npm run build

case "$action" in
  test)
    npm test
    cd "$web_root"
    unformatted=$(gofmt -l main.go internal cmd)
    if [ -n "$unformatted" ]; then printf 'Unformatted Go files:\n%s\n' "$unformatted" >&2; exit 1; fi
    go test -race ./internal/... ./cmd/...
    go vet ./internal/... ./cmd/...
    ;;
  ui) exec npm run preview ;;
  build|run)
    if [ "$(uname -s)" != Darwin ] || [ "$(uname -m)" != arm64 ]; then
      printf 'Native packaging is currently supported on macOS arm64.\n' >&2
      exit 1
    fi
    cd "$web_root"
    go run ./cmd/assets
    go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -s -skipbindings -platform darwin/arm64
    if [ "$action" = run ]; then
      exec "$web_root/build/bin/Mini-Orca.app/Contents/MacOS/mini-orca-desktop"
    fi
    ;;
esac
