# Mini-Orca standardized repository entrypoint

set shell := ["sh", "-cu"]

default:
    @just --list

# Install dependencies and configure local git hooks
setup:
    go mod download
    npm --prefix desktop/frontend install
    @if command -v lefthook >/dev/null 2>&1; then lefthook install; fi

# Run the local development daemon
dev:
    make dev

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
