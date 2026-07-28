# Task 3.1.2 — Define ToolExecutor Interface

## Milestone
Milestone 3: Language-Agnostic Tool Executor

## Description
Define the ToolExecutor interface and ExecResult struct.

## Checklist
- [ ] Define `ToolExecutor` interface with:
  - `Execute(cmd string, args []string, timeout time.Duration) (*ExecResult, error)`
  - `FormatCode(path string) error`
  - `WriteFile(path string, content string) error`
  - `ReadFile(path string) (string, error)`
  - `AppendToFile(path string, content string) error`
- [ ] Define `ExecResult` struct:
  - `ExitCode int`
  - `Stdout string`
  - `Stderr string`
  - `Duration time.Duration`

## Dependencies
- Task 3.1.1

## Deliverables
- `internal/tools/executor.go` with interface and types

## Status
- [ ] Not Started
- [ ] In Progress
- [x] Done
