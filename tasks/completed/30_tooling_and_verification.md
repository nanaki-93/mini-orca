# 30 — Improve tooling and system-boundary verification

## Status

Complete

## Goal

Make the implementation repeatable and catch daemon, API, LLM, desktop, and concurrency regressions.

## Depends on

Tasks 03, 06, 11, 14, 19, 20, and 29.

## Implementation

- Update `../../Makefile`: create build directory for coverage; add `test-race`, `vet`, `check`, `desktop-test`, `desktop-build`, host `build`, and named Linux release build targets; complete `.PHONY`.
- Add Gradle wrapper and use it in docs/Makefile.
- Add `httptest` route/error-contract suite and fake OpenAI-compatible server tests for malformed, non-200, timeout, cancellation, empty, and oversized responses.
- Add cache expiration race regression for `ResponseCache.Get`, or remove it if only web-specific.
- Add desktop API/view-model tests and an automated or documented desktop smoke fixture.
- Verify formatting, `go vet`, race tests, coverage report creation, and desktop build from a clean checkout.

## Acceptance criteria

- `make check` executes the complete supported validation path.
- Critical API and LLM error paths have tests; desktop tests are no longer `NO-SOURCE`.

## Verification

- Run `make check`, clean-build the desktop distribution, and preserve test logs/artifacts.
