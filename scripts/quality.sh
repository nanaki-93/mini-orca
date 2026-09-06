#!/usr/bin/env sh

set -u

staticcheck_version=v0.7.0
tools_version=v0.40.0
gocyclo_version=v0.6.0
jscpd_version=4.0.5
forced_failure=${MINI_ORCA_QUALITY_FAIL_STAGE:-}
failed_stages=""

run_stage() {
  stage=$1
  shift

  printf '\n==> %s\n' "$stage"
  if [ "$forced_failure" = "$stage" ]; then
    printf 'FAIL %s (forced diagnostic failure)\n' "$stage" >&2
    failed_stages="${failed_stages}${failed_stages:+, }${stage}"
    return
  fi

  if "$@"; then
    printf 'PASS %s\n' "$stage"
    return
  fi

  printf 'FAIL %s\n' "$stage" >&2
  failed_stages="${failed_stages}${failed_stages:+, }${stage}"
}

run_go_quality() {
  # go run and npx use their user-level module/package caches. No quality tool is
  # added to go.mod, the daemon image, or this worktree.
  run_stage "Go static analysis" go run honnef.co/go/tools/cmd/staticcheck@"$staticcheck_version" ./...
  run_stage "Go reachability" go run golang.org/x/tools/cmd/deadcode@"$tools_version" ./cmd/daemon
  run_stage "Go complexity" find cmd internal -name '*.go' -type f ! -name '*_test.go' -exec go run github.com/fzipp/gocyclo/cmd/gocyclo@"$gocyclo_version" -over 15 '{}' +
  run_stage "Go clone detection" npx --yes jscpd@"$jscpd_version" \
    --min-tokens 70 \
    --min-lines 8 \
    --threshold 1 \
    --format go \
    --ignore '**/*_test.go,**/testdata/**,build/**,desktop/**' \
    internal cmd
}

run_desktop_quality() {
  run_stage "Desktop formatting and static analysis" ./scripts/desktop-gradle.sh spotlessCheck detekt
}

case "${1:-all}" in
  all)
    run_go_quality
    run_desktop_quality
    ;;
  --go-only)
    run_go_quality
    ;;
  --desktop-only)
    run_desktop_quality
    ;;
  --help|-h)
    printf '%s\n' "Usage: $0 [--go-only|--desktop-only]"
    printf '%s\n' "Set MINI_ORCA_QUALITY_FAIL_STAGE to a stage name to exercise aggregate failure reporting."
    exit 0
    ;;
  *)
    printf 'Unknown quality lane: %s\n' "$1" >&2
    exit 2
    ;;
esac

if [ -n "$failed_stages" ]; then
  printf '\nQuality failed: %s\n' "$failed_stages" >&2
  exit 1
fi

printf '\nQuality passed.\n'
