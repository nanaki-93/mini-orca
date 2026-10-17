# Mini-Orca standardized repository entrypoint

set shell := ["sh", "-cu"]

default:
    @just --list

# Install dependencies and configure local git hooks
setup:
    go mod download
    npm --prefix desktop/frontend install
    @if command -v lefthook >/dev/null 2>&1; then lefthook install; fi

# Run the local development daemon on :9090
dev:
    make dev

# Launch both the local daemon (if not already running on :9090) and the Wails desktop app
desktop:
    #!/usr/bin/env sh
    set -eu
    daemon_pid=""
    cleanup() {
        if [ -n "$daemon_pid" ] && kill -0 "$daemon_pid" 2>/dev/null; then
            kill "$daemon_pid" 2>/dev/null || true
        fi
    }
    trap cleanup EXIT INT TERM
    if ! lsof -iTCP:9090 -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "Starting Mini-Orca daemon on 127.0.0.1:9090..."
        go run ./cmd/daemon &
        daemon_pid=$!
        sleep 1
    else
        echo "Using existing Mini-Orca daemon on 127.0.0.1:9090..."
    fi
    ./scripts/desktop-web.sh run

# Build and launch only the Wails desktop client (assumes daemon is already running)
desktop-run:
    make desktop-run

# Package the macOS arm64 Wails desktop bundle (desktop/build/bin/Mini-Orca.app)
desktop-build:
    make desktop-build

# Run desktop frontend and native bridge tests
desktop-test:
    make desktop-test

# Preview the web UI in a browser
preview:
    make web-preview

# Run automated unit, race, and web UI tests
test:
    go test ./...
    make web-test

# Run a single Go test by regex pattern (e.g. just test-one TestChangeWorkflow)
test-one pattern:
    go test ./... -run "{{pattern}}" -v

# Run formatting, static analysis, complexity, clone, and test gates
check:
    make fmt-check
    make vet
    ./scripts/quality.sh --go-only
    make check

# Build the local daemon binary
build:
    make build
