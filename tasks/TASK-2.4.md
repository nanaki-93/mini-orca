# Task 2.4: Update Render Handler - Get Project from Config

## Phase
Phase 2: Remove Project System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/api/handlers/htmx_render.go` to remove project store dependency and read project path from config or cwd directly.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/api/handlers/htmx_render.go` - Get project from config, not store"
- **Plan says**: "Project path comes from config.yaml (or cwd)"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/api/handlers/htmx_render.go` |

## Changes Required
- Remove `projectStore` field from `HTMXRenderHandler`
- Read project path from config or cwd directly
- File tree reads from filesystem directly (also handled in Task 2.5)

## Impact
- **Medium**: Affects main page rendering
- Must ensure project path is correctly obtained

## Dependencies
- **Before**: Task 2.3 (project files handler deleted)
- **After**: Proceed to Task 2.5

## Risk
🟡 **Medium** - affects main page rendering

## Checklist
- [ ] Remove `projectStore` field from `HTMXRenderHandler` struct
- [ ] Add config path reading to handler
- [ ] Update render logic to use config path
- [ ] Ensure main page renders correctly
