# Task 1.4 — Fix detection, history, and web interaction defects

**Phase:** 1 — Correctness and security  
**Status:** Complete  
**Priority:** High  
**Risk:** Medium

## Goal

Correct project-type/build-file detection and eliminate chat-history and duplicate-submission defects in the original web IDE.

## Plan coverage

- Fix Kotlin/Gradle and `pyproject.toml` project detection.
- Fix chat-history role persistence.
- Add regression tests for found defects.

## Dependencies

- Task 1.1
- Task 1.2
- Task 1.3

## Files

- `internal/tools/project_detection.go`
- `internal/tools/executor_test.go`
- `internal/api/handlers/chat_handler.go`
- `internal/api/handlers/chat_types.go`
- `internal/api/static/js/chat.js`
- `internal/api/templates/ide.html`

## Implementation steps

1. Detect Kotlin projects from Kotlin source roots or Kotlin Gradle plugins.
2. Report `build.gradle.kts` when it is the actual build file and preserve Java detection for Java-only Gradle projects.
3. Report `pyproject.toml` when Python has no `requirements.txt` or uses pyproject configuration.
4. Persist submitted chat prompts with role `user`, matching the browser history renderer.
5. Remove competing HTMX submission attributes from the textarea and keep one JavaScript submission path.
6. Keep keyboard and button submission behavior equivalent.
7. Add detection and chat regressions to the focused test suites.

## Acceptance criteria

- Kotlin, Java, and Python fixtures return the correct project type and build filename.
- Reloaded user messages render as user messages.
- Pressing Enter sends exactly one request.
- Shift+Enter continues to add a newline.

## Verification

```bash
go test ./internal/tools ./internal/api/handlers
go test -race ./internal/tools
```
