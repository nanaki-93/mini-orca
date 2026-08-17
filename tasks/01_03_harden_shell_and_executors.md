# Task 1.3 — Harden shell and project executors

**Phase:** 1 — Correctness and security  
**Status:** Complete  
**Priority:** Critical  
**Risk:** High

## Goal

Prevent command allowlist bypasses and make all commands execute in the imported project's directory without nil-result panics.

## Plan coverage

- Require exact safe-shell executable matches.
- Fix executor defects discovered during the audit.

## Dependencies

- None.

## Files

- `internal/tools/shell.go`
- `internal/tools/executor.go`
- `internal/tools/go_executor.go`
- `internal/tools/java_executor.go`
- `internal/tools/kotlin_executor.go`
- `internal/tools/python_executor.go`
- `internal/tools/rust_executor.go`
- `internal/tools/typescript_executor.go`
- `internal/tools/shell_executor_test.go`
- `internal/tools/executor_test.go`

## Implementation steps

1. Match the `exec.Command` executable against the allowlist exactly; keep subcommands in the argument list.
2. Reject lookalike commands such as `goevil` even when `go` is allowed.
3. Add an optional working directory to shell executors and assign `cmd.Dir` before execution.
4. Construct generic and language-specific executors with `ProjectInfo.RootDir`.
5. Handle command-start failures where no `ShellResult` exists instead of dereferencing nil.
6. Return a generic executor when `NewExecutor` receives nil project information.
7. Delegate file formatting to file-specific formatters so formatting one generated file does not rewrite the complete project.

## Acceptance criteria

- Only explicitly allowed executable names run.
- A missing executable returns an error without panic.
- `pwd` from a project executor equals the configured project root.
- Go, Kotlin, Java, Rust, TypeScript, and Python executors inherit the project root.
- Formatting a generated target affects only the requested file.

## Verification

```bash
go test ./internal/tools
go test -race ./internal/tools
go vet ./internal/tools
```
