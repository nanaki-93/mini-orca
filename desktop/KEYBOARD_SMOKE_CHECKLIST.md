# Desktop keyboard smoke checklist

Run this checklist with a Go project imported in Mini-Orca.

1. Press `Cmd/Ctrl+1` through `Cmd/Ctrl+4`; Summary, Analysis, Bugs, and Editor each become active. `Cmd/Ctrl+Tab` cycles through the same four destinations.
2. From Summary, Analysis, or Bugs, press `Cmd/Ctrl+P` and choose an indexed file. Confirm Editor becomes active with that exact path, then use `Cmd/Ctrl+Shift+O` to choose a symbol. Confirm the source remains selectable and read-only.
3. On a wide window, confirm Summary, Analysis, and Bugs have no Explorer or Context pane while Editor retains both. At exactly 1000dp, confirm Editor's wide panes remain stable. Below 1000dp, confirm labeled Files and Context drawers appear only in Editor; leave Editor with a drawer open and confirm it closes without losing the selected file.
4. In Analysis, confirm project coverage and current/last-run totals are visible. Confirm only failures with path, attempts, and sanitized error text appear under Analysis Errors, with a textual no-errors state when applicable.
5. In Bugs, use Tab to reach filters and finding actions. Press `Cmd/Ctrl+Shift+F` to return to Bugs.
6. In Editor, use `Cmd/Ctrl+K` to open the action route, select or create one target, Tab to the chat composer, and press `Cmd/Ctrl+Enter` to send.
7. Tab to the declaration and imports fields, edit only the draft, then press `Cmd/Ctrl+Shift+V` to validate and `Cmd/Ctrl+Shift+C` to run focused checks.
8. Use the Editor stage bar to move from Draft to Verify only after validation, then Continue to Apply only after current focused checks. Confirm the final action names the selected symbol and file; Apply has no generic confirmation dialog.
9. During an open dialog or active request, press Escape and confirm only that dialog/request closes or cancels. With no active dialog/request, Escape leaves the current workspace and source unchanged.
10. After Apply and Undo, confirm the selected source and brief refresh and that old draft/check evidence is gone.

11. At supported text scaling, verify long relative paths, symbol names, diagnostics, and command output remain readable through scrolling or ellipsis without horizontal clipping.
