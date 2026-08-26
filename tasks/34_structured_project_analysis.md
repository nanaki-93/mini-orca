# 34 — Add structured project analysis

## Status

Pending

## Goal

Make project architecture analysis a versioned structured contract that project workspaces can render safely.

## Depends on

Tasks 32 and 33.

## Implementation

- Define a bounded project-analysis report with purpose, architecture, components, entry points, data/control flows, risks, next steps, status, model/profile, prompt version, and generated time.
- Change the import analysis prompt to return exactly one strict JSON object and reject prose, unknown fields, invalid limits, and empty required values.
- Persist the structured report atomically with its project revision and validity inputs.
- Generate `.mini-orca/analysis.md` only as a human-readable projection of the structured report and deterministic inventory.
- Preserve usable deterministic project facts when model analysis is unavailable or fails.

## Acceptance criteria

- API clients never need to parse Markdown to render project analysis sections.
- Malformed or oversized model responses produce a truthful failed/unavailable status without losing deterministic facts.
- A revision or prompt/schema change makes prior interpretation stale rather than silently current.

## Verification

- Add focused tests in `internal/project` and `internal/app` for parsing, persistence, projection, failure, and invalidation.
- Run `go test ./internal/project ./internal/app`, `make vet`, and `git diff --check`.
