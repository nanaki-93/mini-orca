# 02 — Create one configured application service

## Goal

Replace per-request agent construction with one injected service that applies the real runtime configuration.

## Depends on

Task 01.

## Implementation

- Introduce an application/service package owning LLM access, generation, project state, retry policy, and task profiles.
- Construct it once in `cmd/daemon/main.go` and inject it into HTTP handlers.
- Apply configured coder skills, model override, temperature, timeout, and retry settings to the actual generation call.
- Remove startup-only agent instances that merely log configuration.
- Mark or remove overlapping unused orchestration code after migrating callers.
- Add an effective-model/profile query used by the desktop client.

## Acceptance criteria

- A generation request demonstrably uses configured coder skills and selected model profile.
- No handler creates an unconfigured coder directly.
- Configuration has one authoritative runtime owner.

## Verification

- Use a fake LLM server to assert model and prompt profile.
- Run `go test ./...` and `go vet ./...`.
