# Mini-Orca standardized repository entrypoint

set shell := ["sh", "-cu"]

default:
    @just --list

# Install Go dependencies and configure local git hooks
setup:
    go mod download
    @if command -v lefthook >/dev/null 2>&1; then lefthook install; fi

# Run the local development daemon on :9090
dev:
    make dev

# Launch both the local daemon (if not already running on :9090) and the Go TUI
tui:
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
    go run ./cmd/tui

# Launch only the Go TUI client (assumes daemon is already running)
tui-run:
    make tui-run

# Build the Go TUI binary
tui-build:
    make tui-build

# Run Go TUI workflow and renderer tests
tui-test:
    make tui-test

# Run automated unit and race tests
test:
    go test -race ./...

# Run a single Go test by regex pattern (e.g. just test-one TestChangeWorkflow)
test-one pattern:
    go test ./... -run "{{pattern}}" -v

# Run formatting, static analysis, complexity, clone, and test gates
check:
    make fmt-check
    make vet
    ./scripts/quality.sh --go-only
    make check

# Build the local daemon and TUI binaries
build:
    make build
