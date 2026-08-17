# Task 5.2 — Build and verify the desktop module

**Phase:** 5 — Verification and documentation  
**Status:** Complete  
**Priority:** High  
**Risk:** Low

## Goal

Confirm the native client compiles and its essential import, inspect, and generate flows remain usable and visually consistent.

## Plan coverage

- Build and test the desktop module.
- Verify the desktop app retains the original app's style.

## Dependencies

- All Phase 4 tasks

## Files

- `desktop/build.gradle.kts`
- `desktop/settings.gradle.kts`
- `desktop/src/main/kotlin/io/miniorca/desktop/*.kt`

## Implementation steps

1. Resolve the pinned Kotlin and Compose plugins and dependencies.
2. Compile all production Kotlin sources.
3. Run the desktop test task.
4. Launch against a configured local daemon.
5. Import a representative project and verify analysis/status feedback.
6. Inspect project metrics and a selected text file.
7. Submit an atomic generation request and verify preview-only behavior.
8. Check the charcoal panels, blue accent, borders, typography, and three-pane proportions.

## Acceptance criteria

- The Gradle test/build task succeeds on Java 21.
- The UI remains responsive during daemon calls.
- Import, file selection, metadata, and generation preview work end to end.
- Generated code is never automatically written.
- No Gradle or build cache output is tracked.

## Verification

```bash
gradle -p desktop test
gradle -p desktop run
git status --short
```
