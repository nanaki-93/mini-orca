# Task 6.3: Register Chat Routes in main.go

## Phase
Phase 6: Add Chat API

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Edit `cmd/daemon/main.go` to register chat handler and routes.

## CLEANUP_PLAN Verification
- **Plan says**: "New Endpoints to ADD: `POST /api/chat/message`, `GET /api/chat/history`"
- Part of adding new chat API
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| EDIT | `cmd/daemon/main.go` |

## Changes Required
### Add
- `chatHandler := handlers.NewChatHandler(...)`
- Route registrations:
  ```go
  mux.HandleFunc("POST /api/chat/message", chatHandler.SendMessage)
  mux.HandleFunc("GET /api/chat/history", chatHandler.GetHistory)
  ```

## Impact
- **Low**: Just registration
- Chat handler already created in Task 6.2

## Dependencies
- **Before**: Task 6.2 (chat handler created)
- **After**: Phase 6 complete → Proceed to Phase 7

## Risk
🟢 **Low** - just registration

## Checklist
- [ ] Create `chatHandler` instance
- [ ] Register `POST /api/chat/message` route
- [ ] Register `GET /api/chat/history` route
- [ ] Ensure routes are properly mounted
- [ ] Verify endpoints are accessible
