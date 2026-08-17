# Task 3.4 — Update the web IDE for symbol-scoped generation

**Phase:** 3 — Atomic code generation  
**Status:** Complete  
**Priority:** Medium  
**Risk:** Low

## Goal

Keep the original HTMX web IDE compatible with the atomic generation contract.

## Plan coverage

- Require a selected file and named function/class from every UI.
- Preserve the original application's simple dark style.

## Dependencies

- Task 3.1
- Task 3.2

## Files

- `internal/api/templates/ide.html`
- `internal/api/static/js/chat.js`

## Implementation steps

1. Add a compact required target-symbol input above the existing chat textarea.
2. Keep the selected file as the target file through `setFileContext`.
3. Block submission with a clear message when no file or symbol is selected.
4. Include `target_symbol` in the fetch JSON body.
5. Disable the target input while a request is running.
6. Preserve Enter-to-send, Shift+Enter newline, loading, error, and history behavior.
7. Avoid adding new screens or frontend frameworks.

## Acceptance criteria

- The web IDE cannot submit atomic generation without file and symbol.
- A valid request matches the backend JSON contract.
- Only one request is emitted per send action.
- The new control matches existing colors, spacing, and typography.

## Verification

```bash
go test ./internal/api/handlers
go vet ./...
```
