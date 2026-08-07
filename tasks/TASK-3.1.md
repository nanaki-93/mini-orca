# Task 3.1: Delete Skills Directory

## Phase
Phase 3: Remove Skills System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Delete `internal/agent/skills/` directory entirely to remove skills registry, library, and utilities.

## CLEANUP_PLAN Verification
- **Plan says**: "DELETE: `internal/agent/skills/` - Entire skills directory"
- **Plan says**: Files to delete: `library.go`, `registry.go`, `registry_test.go`, `skills.go`, `utils.go`, `utils_test.go`
- **Plan says**: "Skills become simple config strings in config.yaml (already supported)"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| DELETE | `internal/agent/skills/library.go` |
| DELETE | `internal/agent/skills/registry.go` |
| DELETE | `internal/agent/skills/registry_test.go` |
| DELETE | `internal/agent/skills/skills.go` |
| DELETE | `internal/agent/skills/utils.go` |
| DELETE | `internal/agent/skills/utils_test.go` |

## Impact
- **Medium**: Agents reference skills
- Agents will receive skills as `[]string` from config instead

## Dependencies
- **Before**: Start here (Phase 3 start)
- **After**: Proceed to Task 3.2

## Risk
🟡 **Medium** - agents reference skills

## Checklist
- [ ] Delete entire `internal/agent/skills/` directory
- [ ] Remove any imports of `internal/agent/skills`
- [ ] Ensure no dangling references remain
