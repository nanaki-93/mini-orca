# Task 3.3 — Guarantee safe one-file output handling

**Phase:** 3 — Atomic code generation  
**Status:** Complete  
**Priority:** Critical  
**Risk:** High

## Goal

Ensure preview generation never writes automatically and any legacy workflow write remains confined to one validated target file.

## Plan coverage

- Generated code may impact only one file.
- Generated output cannot target another file or escape the project.

## Dependencies

- Task 1.2
- Task 1.3
- Task 3.2

## Files

- `internal/orchestrator/orchestrator.go`
- `internal/orchestrator/orchestrator_test.go`
- `internal/project/path.go`
- `internal/tools/executor.go`
- `internal/tools/formatter.go`

## Implementation steps

1. Keep chat/desktop generation response-only; do not write the model result from the API handler.
2. Validate legacy `// target:` comments through `ResolvePathForWrite` instead of `filepath.Join` alone.
3. Reject absolute, parent-traversal, sibling-prefix, and symlink-parent output paths.
4. Return target-resolution errors before creating temporary or output files.
5. Atomically write only the resolved target when the legacy workflow is explicitly used.
6. Format the resolved target file through the file-specific formatter rather than a project-wide formatter.
7. Test malicious generated target comments.

## Acceptance criteria

- API generation changes no file on disk.
- `// target: ../outside.go` is rejected.
- A safe project-relative target resolves correctly.
- Formatting one target cannot rewrite unrelated files.

## Verification

```bash
go test ./internal/orchestrator -run TargetOutsideProject
go test ./internal/project -run ResolveFile
go test ./internal/tools
```
