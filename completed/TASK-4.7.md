# Task 4.7: Simplify Template Engine Functions

## Phase
Phase 4: Simplify UI Components

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Completed
- **Date**: 2025-08-07
- **Removed 26 unused template functions** from funcMap:
  - Activity log: `getStats`, `logTypeClass`, `logStatusClass`, `typeBadgeClass`
  - Phase helpers: `phaseState`, `phaseClass`, `phaseLabelClass`, `phaseConnectorClass`, `phaseProgress`, `currentPhaseDotClass`
  - Priority/agent: `priorityClass`, `agentIconClass`, `agentIcon`, `humanName`
  - Badge/score: `categoryBadgeClass`, `scoreColor`, `totalIssues`, `countBySeverity`
  - Issue helpers: `filterBySeverity`, `isSkillAssigned`
  - Phase colors: `phaseBorderClass`, `phaseDotClass`, `phaseProgressClass`, `phaseTextClass`, `coverageColor`
  - Hash: `md5` (replaced with path-based IDs)
- **Deleted 2 files**: `htmx_template_funcs.go`, `htmx_template_colors.go`
- **Cleaned up** `htmx_helpers.go`: removed `md5Hash`, `humanName`, unused imports
- **Updated templates** to inline CSS class logic:
  - `activity-entry.html`: replaced `logTypeClass`, `logStatusClass`, `typeBadgeClass`
  - `activity-log.html`: replaced `getStats` with inline counting
  - `phase-node.html`: replaced `phaseClass`, `phaseLabelClass`, `phaseConnectorClass`, `phaseProgress`
  - `phase-tracker.html`: replaced `phaseState`, `currentPhaseDotClass`
  - `file-tree-item.html`: replaced `md5` with path-based IDs
  - `header.html`: replaced `phaseBorderClass`, `phaseDotClass`, `phaseTextClass`, `phaseProgressClass`
- **Kept 6 template functions**: `fileExtension`, `fileIcon`, `codeLines`, `lower`, `title`, `dict`
- **Build verified: ✅ passes**
- **No dangling references to removed functions remain**

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
- [x] Remove all listed unused template functions
- [x] Keep only the 6 listed functions
- [x] Remove function registrations from template engine
- [x] Ensure remaining templates compile and render
