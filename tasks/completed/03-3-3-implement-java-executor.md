# Task 3.3.3 — Implement Java Executor

## Milestone
Milestone 3: Language-Agnostic Tool Executor

## Description
Create the Java-specific executor.

## Checklist
- [ ] Create `internal/tools/java_executor.go`
- [ ] Implement `FormatCode()` → runs `./gradlew spotlessApply`
- [ ] Implement `RunTests()` → runs `./gradlew test`
- [ ] Implement `Build()` → runs `./gradlew build`
- [ ] Implement `Lint()` → runs `./gradlew spotlessCheck`

## Dependencies
- Task 3.1.2, Task 3.2.1

## Deliverables
- `internal/tools/java_executor.go`

## Status
- [ ] Not Started
- [ ] In Progress
- [x] Done
