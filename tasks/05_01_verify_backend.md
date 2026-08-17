# Task 5.1 — Add and run backend regression coverage

**Phase:** 5 — Verification and documentation  
**Status:** Complete  
**Priority:** Critical  
**Risk:** Low

## Goal

Prove that the new backend behavior and every bug fix remain correct under unit, race, vet, format, and build checks.

## Plan coverage

- Add regression tests for every found bug.
- Run Go unit, race, vet, and formatting checks.

## Dependencies

- All Phase 1 tasks
- All Phase 2 tasks
- All Phase 3 tasks

## Files

- `internal/project/project_test.go`
- `internal/agent/orchestrator_test.go`
- `internal/orchestrator/orchestrator_test.go`
- `internal/tools/executor_test.go`
- `internal/tools/shell_executor_test.go`

## Implementation steps

1. Test sibling-prefix traversal, `..` traversal, symlink escape, and safe output resolution.
2. Test analysis persistence, metadata counts, inventory, context content, binary handling, and size limits.
3. Capture atomic coder prompts and assert file, symbol, scope prohibition, and complete-file output contract.
4. Test generated target-comment traversal.
5. Test safe-shell prefix lookalikes and command-start failures.
6. Test executor project working directory and nil project information.
7. Test Kotlin/Java/Gradle and Python build-file detection.
8. Run format, vet, build, unit, and race checks across the module.

## Acceptance criteria

- `go test -race ./...` passes.
- `go vet ./...` passes.
- `gofmt` produces no changes.
- The daemon builds successfully.
- Each security/correctness fix has a focused regression assertion.

## Verification

```bash
go fmt ./...
go vet ./...
go test -race ./...
go build -o /tmp/mini-orca-daemon ./cmd/daemon
git diff --check
```
