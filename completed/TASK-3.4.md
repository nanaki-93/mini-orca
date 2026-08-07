# Task 3.4: Simplify Config - Remove SkillsConfig

## Phase
Phase 3: Remove Skills System

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `internal/config/config.go` to remove `SkillsConfig` struct and simplify skills handling.

## CLEANUP_PLAN Verification
- **Plan says**: "MODIFY: `internal/config/config.go` - Remove SkillsConfig"
- **Plan says**: "Skills become simple config strings in config.yaml (already supported)"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `internal/config/config.go` |

## Changes Required
- Remove `SkillsConfig` struct
- Remove `Skills` field from `Config` struct
- Remove `applyDefaults()` skills initialization
- Keep `AgentConfig.Skills` as simple `[]string`

## Impact
- **Medium**: Config structure changes
- All config loading must be updated

## Dependencies
- **Before**: Task 3.3 (skills types deleted)
- **After**: Proceed to Task 3.5

## Risk
🟡 **Medium** - config structure changes

## Checklist
- [ ] Remove `SkillsConfig` struct definition
- [ ] Remove `Skills` field from `Config` struct
- [ ] Remove skills initialization from `applyDefaults()`
- [ ] Keep `AgentConfig.Skills` as `[]string`
- [ ] Ensure config loading still works
