# Task 5.3 — Update documentation and version information

**Phase:** 5 — Verification and documentation  
**Status:** Complete  
**Priority:** Medium  
**Risk:** Low

## Goal

Document the new workflow, APIs, desktop launch process, safety behavior, and consistent application version.

## Plan coverage

- Update README/API documentation with the new workflow and launch commands.
- Complete final verification and handoff.

## Dependencies

- Task 5.1
- Task 5.2

## Files

- `README.md`
- `API.md`
- `Plan.md`
- `desktop/README.md`
- `internal/version/version.go`
- `internal/api/handlers/htmx_render.go`
- `internal/api/templates/ide.html`
- `tasks/*.md`

## Implementation steps

1. Add AI project analysis, atomic generation, and Compose Desktop to the feature list.
2. Document daemon and desktop launch commands.
3. Document the import workflow and `.mini-orca/analysis.md` output.
4. Replace the old chat body with `message`, `file_path`, and `target_symbol`.
5. Document all project endpoints and file safety limits.
6. Keep the application, web footer, README, and desktop package version aligned.
7. Record completed verification in `Plan.md`.
8. Maintain this task index and dependency graph.

## Acceptance criteria

- A new contributor can start the daemon and desktop client from the README.
- API consumers can construct import, file-info, and atomic-generation requests from `API.md`.
- Version output is consistent across health/status, web UI, README, and desktop packaging.
- `Plan.md` and task statuses accurately reflect verified implementation state.

## Verification

```bash
rg -n "projects/import|target_symbol|analysis.md|gradle -p desktop" README.md API.md desktop/README.md
go test ./...
git diff --check
```
