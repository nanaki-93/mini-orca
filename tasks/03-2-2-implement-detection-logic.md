# Task 3.2.2 — Implement Detection Logic

## Milestone
Milestone 3: Language-Agnostic Tool Executor

## Description
Implement project type detection based on build files.

## Checklist
- [ ] Implement `DetectProjectType(rootDir string) (*ProjectInfo, error)`
- [ ] Detection rules (check in order):
  - `go.mod` → Go
  - `build.gradle` or `build.gradle.kts` → Kotlin/Java
  - `Cargo.toml` → Rust
  - `package.json` → TypeScript
  - `requirements.txt` or `pyproject.toml` → Python
- [ ] Set appropriate SrcDir, TestDir, BuildFile for each type
- [ ] Return error if no known project type found

## Dependencies
- Task 3.2.1

## Deliverables
- Working DetectProjectType function

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
