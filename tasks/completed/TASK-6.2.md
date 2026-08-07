# Task 6.2: Create Chat Handler

## Phase
Phase 6: Add Chat API

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Create `internal/api/handlers/chat_handler.go` with chat message and history endpoints.

## CLEANUP_PLAN Verification
- **Plan says**: "New Files to CREATE: `internal/api/handlers/chat_handler.go`"
- **Plan says**: "New Endpoints: `POST /api/chat/message`, `GET /api/chat/history`"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| CREATE | `internal/api/handlers/chat_handler.go` |

## Endpoints to Create
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/chat/message` | Send message, trigger pipeline |
| GET | `/api/chat/history` | Get conversation history |

## Logic Required
1. Parse user request
2. Call orchestrator with request + file context
3. Stream responses back to client
4. Store chat history in memory

## Impact
- **Medium**: New feature
- Integrates with orchestrator (simplified in Phase 5)

## Dependencies
- **Before**: Task 6.1 (chat types created)
- **After**: Proceed to Task 6.3

## Risk
🟡 **Medium** - new feature

## Checklist
- [x] Create `internal/api/handlers/chat_handler.go`
- [x] Implement `SendMessage` handler for `POST /api/chat/message`
- [x] Implement `GetHistory` handler for `GET /api/chat/history`
- [x] Add in-memory chat history storage
- [x] Integrate with simplified orchestrator
- [x] Implement response streaming
