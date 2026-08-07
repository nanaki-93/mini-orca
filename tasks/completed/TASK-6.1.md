# Task 6.1: Create Chat Types

## Phase
Phase 6: Add Chat API

## Status
⬜ Pending | 🔄 In Progress | ✅ Complete

## Goal
Create `internal/api/handlers/chat_types.go` with chat request/response type definitions.

## CLEANUP_PLAN Verification
- **Plan says**: "New Files to CREATE: `internal/api/handlers/chat_types.go`"
- **Plan says**: "New Endpoints to ADD: `POST /api/chat/message`, `GET /api/chat/history`"
- ✅ This task directly fulfills the plan requirement

## Files
| Action | File |
|--------|------|
| CREATE | `internal/api/handlers/chat_types.go` |

## Types to Create

```go
type ChatRequest struct {
    Message    string `json:"message"`
    FilePath   string `json:"file_path,omitempty"`    // optional: file being edited
    LineNumber int    `json:"line_number,omitempty"`  // optional: cursor position
}

type ChatResponse struct {
    Role      string    `json:"role"`       // "assistant" or "system"
    Content   string    `json:"content"`
    Phase     string    `json:"phase"`      // "coding", "testing", "review"
    Timestamp time.Time `json:"timestamp"`
}
```

## Impact
- **Low**: New types, no breaking changes

## Dependencies
- **Before**: Start here (Phase 6 start)
- **After**: Proceed to Task 6.2

## Risk
🟢 **Low** - new types

## Checklist
- [x] Create `internal/api/handlers/chat_types.go`
- [x] Define `ChatRequest` struct with Message, FilePath, LineNumber
- [x] Define `ChatResponse` struct with Role, Content, Phase, Timestamp
- [x] Ensure types compile
