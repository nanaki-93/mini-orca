# Task 2.5: Update File Tree Handler - Direct Filesystem Access

## Phase
Phase 2: Remove Project System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/api/handlers/htmx_filetree.go` to remove project store dependency and walk filesystem directly.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/api/handlers/htmx_filetree.go` - Direct file tree from filesystem"
- **Plan says**: "File tree reads directly from the project directory on disk"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/api/handlers/htmx_filetree.go` |

## Changes Required
- Remove project store dependency
- Walk filesystem directly from configured project path
- Use `os.ReadDir` or `filepath.Walk` for file tree generation

## Impact
- **Medium**: File tree is core UI feature
- Must handle large directories efficiently

## Dependencies
- **Before**: Task 2.4 (render handler updated)
- **After**: Phase 2 complete → Proceed to Phase 3

## Risk
🟡 **Medium** - file tree is core UI feature

## Checklist
- [ ] Remove project store dependency from file tree handler
- [ ] Implement direct filesystem walk
- [ ] Use config path for project root
- [ ] Test file tree rendering with various directory structures
- [ ] Ensure performance is acceptable for large projects
