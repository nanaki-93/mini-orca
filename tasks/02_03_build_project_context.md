# Task 2.3 — Build bounded project-wide context

**Phase:** 2 — Project analysis backend  
**Status:** Complete  
**Priority:** Critical  
**Risk:** Medium

## Goal

Give analysis and generation a global view of the project without constructing an unbounded prompt.

## Plan coverage

- Build context from analysis, complete inventory, build metadata, and bounded source snippets.
- Always prioritize the selected file.

## Dependencies

- Task 2.2

## Files

- `internal/project/context.go`
- `internal/project/project_test.go`

## Implementation steps

1. Include a complete, sorted relative file inventory.
2. Prioritize the selected file, `.mini-orca/analysis.md`, README, and language build manifests.
3. Include bounded snippets from recognized source, configuration, and documentation files.
4. Give the target file a larger content allowance than non-target files.
5. Enforce a total context byte budget while retaining the full inventory.
6. Skip binary/unreadable content and excluded generated directories.
7. Validate the target with canonical project-boundary rules before reading it.

## Acceptance criteria

- Context always names every inventoried file.
- Target-file content appears before ordinary source snippets.
- Representative source and build metadata are present when within budget.
- Context generation rejects a target outside the project.
- Large projects do not create unbounded source-content prompts.

## Verification

```bash
go test ./internal/project -run TestAnalyzerWritesAnalysisAndContextIncludesInventory
```
