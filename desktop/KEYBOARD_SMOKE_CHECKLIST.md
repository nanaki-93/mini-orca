# Desktop keyboard smoke checklist

Start Mini-Orca with no project open, then use a Go project for the remaining steps.

1. Confirm the landing state contains only product identity, **Open project**, and contextual progress or retry feedback. Press `Cmd/Ctrl+O`, cancel the chooser, and confirm the landing state is unchanged. Repeat with a failed open if available; confirm **Open project** remains available for retry. Before a project opens, verify `Cmd/Ctrl+1`–`4`, `Cmd/Ctrl+P`, `Cmd/Ctrl+Shift+O`, `Cmd/Ctrl+K`, and `Cmd/Ctrl+Tab` have no project action.
2. Press `Cmd/Ctrl+O` and import the fixture. Confirm the full workspace replaces the landing state with no remaining landing content.
3. Press `Cmd/Ctrl+1` through `Cmd/Ctrl+4`; Summary, Analysis, Bugs, and Editor each become active. `Cmd/Ctrl+Tab` cycles through the same four destinations.
4. From Summary, Analysis, or Bugs, press `Cmd/Ctrl+P` and choose an indexed file. Confirm Editor becomes active with that exact path, then use `Cmd/Ctrl+Shift+O` to choose a symbol. Confirm the source remains selectable and read-only.
5. On a wide window, confirm Summary, Analysis, and Bugs have no Explorer or Context pane while Editor retains both. At exactly 1000dp, confirm Editor's wide panes remain stable. Below 1000dp, confirm labeled Files and Context drawers appear only in Editor; leave Editor with a drawer open and confirm it closes without losing the selected file.
6. In Analysis, confirm project coverage and current/last-run totals are visible. Confirm only failures with path, attempts, and sanitized error text appear under Analysis Errors, with a textual no-errors state when applicable. Check that the file/retry limits share one compact row and Pause/Cancel or Resume/Cancel wrap without clipping on a narrow window.
7. In Bugs, use Tab to reach the search field, Filters disclosure, advanced filters, and finding actions. Confirm only search is initially visible, active filters are named in text, and `Cmd/Ctrl+Shift+F` returns to Bugs.
8. In Editor, use `Cmd/Ctrl+K` to open the action route, select or create one target, Tab to the chat composer, and press `Cmd/Ctrl+Enter` to send.
9. Tab to the declaration and imports fields, edit only the draft, then press `Cmd/Ctrl+Shift+V` to validate and `Cmd/Ctrl+Shift+C` to run focused checks.
10. Use the Editor stage bar to move from Draft to Verify only after validation, then Continue to Apply only after current focused checks. Confirm the concise visible current-stage marker and understandable disabled-stage text. Confirm the final action names the selected symbol and file; Apply has no generic confirmation dialog.
11. During an open dialog or active request, press Escape and confirm only that dialog/request closes or cancels. With no active dialog/request, Escape leaves the current workspace and source unchanged.
12. After Apply and Undo, confirm the selected source and brief refresh and that old draft/check evidence is gone.

13. At supported text scaling, verify long relative paths, symbol names, diagnostics, command output, compact buttons, and fields remain readable through scrolling or ellipsis without horizontal clipping. Confirm primary, navigation, positive, attention, destructive, and neutral actions retain text labels and readable state in addition to their color.
