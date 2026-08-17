# Task 4.4 — Add the desktop atomic generation panel

**Phase:** 4 — Kotlin Compose Desktop client  
**Status:** Complete  
**Priority:** Critical  
**Risk:** Medium

## Goal

Provide a focused generation panel that can request one function/class in the selected file and preview the result safely.

## Plan coverage

- Provide a symbol-scoped generation panel.
- Keep generation limited to one file while using project-wide context.

## Dependencies

- Task 4.2
- Task 3.2

## Files

- `desktop/src/main/kotlin/io/miniorca/desktop/Main.kt`
- `desktop/src/main/kotlin/io/miniorca/desktop/ApiClient.kt`

## Implementation steps

1. Show the selected target file read-only in the right pane.
2. Add inputs for exact function/class name and precise behavioral request.
3. Enable Generate only when target file, symbol, and prompt are present and no request is running.
4. Submit the atomic JSON contract asynchronously.
5. Display the generated one-file result in the central selectable preview.
6. Explicitly state that preview generation never applies changes automatically.
7. Keep errors, busy state, and phase information in the existing status bar.

## Acceptance criteria

- Generation is impossible without selecting a file and entering a symbol and prompt.
- A request contains exactly one file and one symbol.
- The returned code appears as a preview and no project file is modified.
- The panel matches the original application's compact dark IDE style.

## Verification

```bash
gradle -p desktop test
```

Manual check: generate a named method, verify the preview, and confirm the target file on disk is unchanged.
