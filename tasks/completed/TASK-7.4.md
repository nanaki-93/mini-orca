# Task 7.4: Add Review Modal

## Phase
Phase 7: Update UI for Chat

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Create `internal/api/templates/components/review-modal.html` for user review actions.

## CLEANUP_PLAN Verification
- **Plan says**: "Show result to user (edit/accept/refuse)"
- Part of the simplified workflow: coder → tester → reviewer → user approves/edits/refuses
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| CREATE | `internal/api/templates/components/review-modal.html` |

## Content Required
- Shows generated code diff
- Shows test results
- Shows review report
- Buttons: Accept, Edit (with feedback), Refuse

## Impact
- **Medium**: New modal
- Must integrate with human gate from Phase 5

## Dependencies
- **Before**: Task 7.3 (chat JavaScript created)
- **After**: Phase 7 complete → Proceed to Phase 8

## Risk
🟡 **Medium** - new modal

## Checklist
- [x] Create `review-modal.html` component
- [x] Add code diff display
- [x] Add test results display
- [x] Add review report display
- [x] Add Accept, Edit (with feedback), Refuse buttons
- [x] Ensure modal integrates with human gate workflow
