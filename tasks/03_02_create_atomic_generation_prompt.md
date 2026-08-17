# Task 3.2 — Create the one-symbol generation prompt

**Phase:** 3 — Atomic code generation  
**Status:** Complete  
**Priority:** Critical  
**Risk:** High

## Goal

Instruct the coder to modify exactly one named function/class in one file while using global project context.

## Plan coverage

- Supply project-wide context.
- Tell the model to change only the named symbol and return one complete target file.

## Dependencies

- Task 2.3
- Task 3.1

## Files

- `internal/agent/orchestrator.go`
- `internal/agent/coder.go`
- `internal/agent/orchestrator_test.go`
- `internal/api/handlers/chat_handler.go`

## Implementation steps

1. Add `RunCoderForSymbol` alongside the legacy prompt entry point.
2. Validate non-empty user request, target file, and target symbol.
3. State the immutable file and symbol scope before the project context.
4. Explicitly forbid creating, renaming, or modifying any other file or unrelated symbol.
5. Require one code block containing the complete updated target-file content and no explanation or patch.
6. Use the language-neutral coder role so Kotlin, Java, Rust, TypeScript, Python, and Go projects are supported.
7. Construct the executor and context from the current project for every request.

## Acceptance criteria

- Captured LLM prompts contain the exact target file and symbol.
- Prompts contain complete project inventory/context.
- The output contract requests exactly one complete file.
- Empty target fields fail without calling the LLM.

## Verification

```bash
go test ./internal/agent -run RunCoderForSymbol
```
