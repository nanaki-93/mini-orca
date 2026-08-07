# Task 2.3: Delete Project Files Handler

## Phase
Phase 2: Remove Project System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete `internal/api/handlers/project_files.go` to remove file content API endpoint.

## CLEANUP_PLAN Verification
- **Plan says**: "DELETE: `internal/api/handlers/project_files.go` - File listing via API"
- **Plan says**: "File tree reads directly from the project directory on disk"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| DELETE | `internal/api/handlers/project_files.go` |

## Endpoints to Remove
- `GET /api/projects/:id/files` (file content endpoint)

## Impact
- **Low**: File content now served via editor component
- File tree will read from filesystem directly (Task 2.5)

## Dependencies
- **Before**: Task 2.2 (project types deleted)
- **After**: Proceed to Task 2.4

## Risk
🟢 **Low** - file content now served via editor component

## Checklist
- [ ] Delete `internal/api/handlers/project_files.go`
- [ ] Remove route registrations from `main.go`
- [ ] Ensure no dangling references remain
