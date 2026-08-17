# Task 4.3 — Add project explorer, viewer, and metadata

**Phase:** 4 — Kotlin Compose Desktop client  
**Status:** Complete  
**Priority:** Critical  
**Risk:** Medium

## Goal

Make project-wide facts and useful information about the selected file visible in the desktop app.

## Plan coverage

- Provide project summary and file explorer/viewer.
- Show selected-file details.

## Dependencies

- Task 4.2
- Task 2.5

## Files

- `desktop/src/main/kotlin/io/miniorca/desktop/Main.kt`
- `desktop/src/main/kotlin/io/miniorca/desktop/ApiClient.kt`
- `desktop/src/main/kotlin/io/miniorca/desktop/Models.kt`

## Implementation steps

1. Render the imported analysis file inventory in a lazy, filterable left pane.
2. Track the selected relative file path and visually highlight it.
3. Fetch file metadata/content through the daemon instead of bypassing backend safety rules.
4. Show project type, total files, source files, total lines, and AI status in compact metric cards.
5. Show selected-file language, size, and line count.
6. Render selected text or the project analysis in a scrollable, selectable center pane.
7. Handle binary, oversized, and failed file requests through the shared status/error state.

## Acceptance criteria

- Every inventoried file is searchable and selectable.
- Selecting a valid text file shows content and metadata.
- Project summary remains visible alongside selected-file information.
- No direct desktop filesystem read bypasses the daemon's project-boundary checks.

## Verification

```bash
gradle -p desktop test
```

Manual check: import a mixed-language project, filter its file list, and inspect at least one source file.
