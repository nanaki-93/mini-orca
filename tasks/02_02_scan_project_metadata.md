# Task 2.2 — Scan projects and collect metadata

**Phase:** 2 — Project analysis backend  
**Status:** Complete  
**Priority:** High  
**Risk:** Medium

## Goal

Create a deterministic project inventory and the factual project/file information consumed by analysis and UIs.

## Plan coverage

- Collect project type, language/file/line counts, build file, and complete file inventory.
- Provide selected-file information.

## Dependencies

- Task 1.2

## Files

- `internal/project/analysis.go`
- `internal/project/project_test.go`
- `internal/tools/project_detection.go`

## Implementation steps

1. Walk the canonical project tree while skipping generated/dependency directories such as `.git`, `node_modules`, `vendor`, `build`, and `target`.
2. Cap the accepted inventory to prevent unbounded resource consumption.
3. Sort all relative file paths for deterministic output.
4. Detect languages from file extensions and count files and text lines.
5. Reuse project detection to capture project type and build file.
6. Implement selected-file metadata: name, relative path, extension, language, size, line count, modification time, binary flag, and text content.
7. Apply the safe read-size and binary rules from Task 1.2.

## Acceptance criteria

- Scanning the same tree twice returns the same ordered inventory.
- Counts and language summaries match fixture contents.
- Ignored build/dependency folders do not inflate metadata.
- Selected-file metadata never escapes the project root.

## Verification

```bash
go test ./internal/project -run 'TestAnalyzer|TestGetFileInfo'
```
