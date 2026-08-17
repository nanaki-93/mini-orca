# Task 1.2 — Enforce canonical project path boundaries

**Phase:** 1 — Correctness and security  
**Status:** Complete  
**Priority:** Critical  
**Risk:** High

## Goal

Guarantee that project reads, directory expansion, metadata requests, and generated output paths cannot escape the selected project.

## Plan coverage

- Replace unsafe path-prefix checks with canonical project-boundary validation.
- Reject attempts to read outside the active project.
- Guard file viewing against binary and oversized content.

## Dependencies

- None.

## Files

- `internal/project/path.go`
- `internal/project/analysis.go`
- `internal/project/project_test.go`
- `internal/api/handlers/htmx_filetree.go`
- `internal/api/handlers/htmx_helpers.go`
- `internal/api/templates/components/file-tree-item.html`

## Implementation steps

1. Canonicalize imported roots using absolute paths and `filepath.EvalSymlinks`.
2. Resolve requested files with `filepath.Rel`; reject absolute paths, `..` traversal, sibling-prefix tricks, and symlink escapes.
3. Add a separate safe resolver for output files that may not exist yet while validating their existing parent directory.
4. Route HTMX folder/file access through the canonical resolvers.
5. Generate collision-resistant DOM identifiers from complete relative paths.
6. Escape dynamically generated path attributes and HTMX JSON values.
7. Reject displayed files larger than 1 MiB and identify binary/invalid UTF-8 files without returning their contents.

## Acceptance criteria

- `/tmp/app2` is not treated as a child of `/tmp/app`.
- `../`, absolute paths, and symlinks outside the root are rejected.
- A safe non-existent output filename inside an existing project directory is accepted.
- Binary or oversized files are never inserted into HTML or generation context as visible content.
- Repeated folder names do not create duplicate DOM IDs.

## Verification

```bash
go test ./internal/project ./internal/api/handlers
go test -race ./internal/project
```
