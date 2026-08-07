# Task 4.7: Simplify Template Engine Functions

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/api/handlers/htmx_templates.go` to remove unused template functions.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/api/handlers/htmx_templates.go` - Remove unused template funcs"
- Part of cleaning up unused code
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/api/handlers/htmx_templates.go` |

## Functions to Remove
- `getStats`, `logTypeClass`, `logStatusClass`
- `typeBadgeClass`, `phaseState`, `phaseClass`, `phaseLabelClass`
- `phaseConnectorClass`, `phaseProgress`, `currentPhaseDotClass`
- `priorityClass`, `agentIconClass`, `agentIcon`, `humanName`
- `categoryBadgeClass`, `scoreColor`, `totalIssues`, `countBySeverity`
- `filterBySeverity`, `isSkillAssigned`, `phaseBorderClass`
- `phaseDotClass`, `phaseProgressClass`, `phaseTextClass`
- `coverageColor`, `md5`

## Functions to Keep
- `fileExtension`
- `fileIcon`
- `codeLines`
- `lower`
- `title`
- `dict`

## Impact
- **Medium**: Template rendering
- Must ensure all remaining template functions are still registered

## Dependencies
- **Before**: Task 4.6 (IDE template simplified)
- **After**: Phase 4 complete → Proceed to Phase 5

## Risk
🟡 **Medium** - template rendering

## Checklist
- [ ] Remove all listed unused template functions
- [ ] Keep only the 6 listed functions
- [ ] Remove function registrations from template engine
- [ ] Ensure remaining templates compile and render
