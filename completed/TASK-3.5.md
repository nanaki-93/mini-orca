# Task 3.5: Update Agent Registry - Remove Skills Dependency

## Phase
Phase 3: Remove Skills System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/agent/registry.go` to remove skills registry dependency. Agents will receive skills as `[]string` from config.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/agent/registry.go` - Remove skills from agent creation"
- **Plan says**: "Agents read skills directly from config, no registry needed"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/agent/registry.go` |

## Changes Required
- Remove `skillsRegistry` parameter from `InitAgentRegistry`
- Agents receive skills as `[]string` from config
- Remove skills registry initialization

## Impact
- **Medium**: Agent creation
- All agent initialization must be updated

## Dependencies
- **Before**: Task 3.4 (config simplified)
- **After**: Phase 3 complete → Proceed to Phase 4

## Risk
🟡 **Medium** - agent creation

## Checklist
- [ ] Remove `skillsRegistry` parameter from `InitAgentRegistry`
- [ ] Update agent creation to use `[]string` skills from config
- [ ] Remove skills registry initialization from `main.go`
- [ ] Ensure agents still receive their skill lists
