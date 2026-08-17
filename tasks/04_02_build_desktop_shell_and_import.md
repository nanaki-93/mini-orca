# Task 4.2 — Build the desktop shell and project import flow

**Phase:** 4 — Kotlin Compose Desktop client  
**Status:** Complete  
**Priority:** Critical  
**Risk:** Medium

## Goal

Build the one-window application shell and let users select and analyze a project without blocking the UI.

## Plan coverage

- Preserve the original charcoal, blue-accent, three-pane IDE style.
- Provide project import and summary.
- Keep state and networking simple.

## Dependencies

- Task 4.1

## Files

- `desktop/src/main/kotlin/io/miniorca/desktop/Main.kt`

## Implementation steps

1. Create one resizable Compose `Window` and a dark Material theme matching the web IDE palette.
2. Build header, left explorer, center content, right generator, and footer status regions.
3. Add a native directory chooser to the header's Import project action.
4. Run blocking HTTP calls on `Dispatchers.IO` through one remembered coroutine scope.
5. Store only necessary UI state with `remember`: project, selected file, filters, request fields, output, busy state, and status/error.
6. Show busy progress and keep import disabled during the analysis request.
7. Display the analysis summary and status after a successful import.

## Acceptance criteria

- The application launches as one window with the intended three-pane layout.
- Import opens a directory chooser and calls the daemon asynchronously.
- Successful import shows project identity and AI analysis status.
- Failures appear in the status bar without crashing or freezing the window.

## Verification

```bash
gradle -p desktop test
gradle -p desktop run
```
