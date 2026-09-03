#!/usr/bin/env sh

set -eu

staticcheck_version=v0.7.0
tools_version=v0.40.0
gocyclo_version=v0.6.0
jscpd_version=4.0.5

# go run and npx use their user-level module/package caches. No quality tool is
# added to go.mod, the daemon image, or this worktree.
go run honnef.co/go/tools/cmd/staticcheck@"$staticcheck_version" ./...
go run golang.org/x/tools/cmd/deadcode@"$tools_version" ./cmd/daemon

production_files=$(find cmd internal -name '*.go' -type f ! -name '*_test.go' | sort)
# shellcheck disable=SC2086
go run github.com/fzipp/gocyclo/cmd/gocyclo@"$gocyclo_version" -over 15 $production_files

npx --yes jscpd@"$jscpd_version" \
  --min-tokens 70 \
  --min-lines 8 \
  --threshold 1 \
  --format go \
  --ignore '**/*_test.go,**/testdata/**,build/**,desktop/**' \
  internal cmd
