# Task 3.1 — Require target file and symbol in generation requests

**Phase:** 3 — Atomic code generation  
**Status:** Complete  
**Priority:** Critical  
**Risk:** High

## Goal

Make every code-generation request identify one existing text file and one exact function or class.

## Plan coverage

- Require `file_path` and `target_symbol` in code-generation requests.
- Reject generation unless a file and function/class are selected.

## Dependencies

- Task 2.1
- Task 2.5

## Files

- `internal/api/handlers/chat_types.go`
- `internal/api/handlers/chat_handler.go`
- `internal/api/static/js/chat.js`
- `desktop/src/main/kotlin/io/miniorca/desktop/Models.kt`
- `desktop/src/main/kotlin/io/miniorca/desktop/ApiClient.kt`

## Implementation steps

1. Add required `file_path` and `target_symbol` JSON fields to the request type.
2. Reject blank messages, target files, and target symbols with HTTP 400.
3. Bound the target-symbol length and reject embedded newlines.
4. Resolve the target file under the active project and require a readable text file.
5. Ensure the web and desktop clients send the exact same JSON contract.
6. Keep `line_number` optional for future cursor context without using it as the target boundary.

## Acceptance criteria

- Missing message, file, or symbol is rejected before any LLM request.
- Binary, oversized, outside-project, and directory targets are rejected.
- A valid relative text file and symbol proceed to prompt generation.
- Both clients send `message`, `file_path`, and `target_symbol`.

## Verification

```bash
go test ./internal/api/handlers ./internal/project
gradle -p desktop test
```
