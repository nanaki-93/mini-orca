# 36 — Add explicit isolated Go project scans

## Status

Pending

## Goal

Produce tool-reported parser, vet, and test findings without allowing scans to mutate the imported project.

## Depends on

Task 35.

## Implementation

- Reuse index diagnostics as parser findings with project-relative locations.
- Add an explicit Go scan service that copies the eligible project into an isolated workspace before invoking tools.
- Support separately visible parse, `go vet ./...`, and `go test ./...` phases with cancellation, deadlines, sanitized bounded output, and exit status.
- Parse useful project-relative file/line/rule information conservatively and retain unparsed output as bounded evidence.
- Persist source-free scan progress/results and never run vet or tests during import, reindex, file analysis, or generation.

## Acceptance criteria

- Starting a scan requires an explicit request for the current project revision.
- Scan processes cannot write to the imported project through their working directory.
- Cancellation stops the active command and completed phase results remain available.
- Tool findings are labeled `tool_reported`, never guaranteed-correct or AI-generated.

## Verification

- Add fixture tests for pass, parse failure, vet/test failure, cancellation, stale revision, output truncation, and source isolation.
- Run `go test ./internal/app ./internal/project -race`, `make vet`, and `git diff --check`.
