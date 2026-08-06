# Task 2.2: Split internal/api/session.go

## Goal
Split 426-line file into 2 files: types + handler.

## Files to CREATE
- `internal/api/session_types.go`
- `internal/api/session_handler.go`

## Files to DELETE
- `internal/api/session.go`

---

## File 1: session_types.go (~150 lines)

**Move here:**
- `SessionCreateRequest` struct
- `SessionResponse` struct
- `SessionListResponse` struct
- `SessionSummary` struct
- `SessionStore` struct
- `NewSessionStore()`
- `CreateSession()`
- `GetSession()`
- `UpdateSessionStatus()`
- `ListSessions()`
- `copySession()`

```go
package api

type SessionCreateRequest struct { ... }
type SessionResponse struct { ... }
type SessionListResponse struct { ... }
type SessionSummary struct { ... }

type SessionStore struct { ... }
func NewSessionStore() *SessionStore { ... }
func (s *SessionStore) CreateSession(...) (*state.Session, error) { ... }
func (s *SessionStore) GetSession(id string) (*state.Session, error) { ... }
func (s *SessionStore) UpdateSessionStatus(id string, status state.SessionStatus, phase state.Phase) error { ... }
func (s *SessionStore) ListSessions() []*state.Session { ... }
func copySession(s *state.Session) *state.Session { ... }
```

---

## File 2: session_handler.go (~280 lines)

**Move here:**
- `SessionHandler` struct
- `NewSessionHandler()`
- `CreateSession()` handler
- `GetSessionStatus()` handler
- `StartSession()` handler
- `PauseSession()` handler
- `ResumeSession()` handler
- `StopSession()` handler
- `ListSessions()` handler
- `ExtractSessionID()` (if defined here)

```go
package api

type SessionHandler struct { ... }
func NewSessionHandler(...) *SessionHandler { ... }
func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) { ... }
func (h *SessionHandler) GetSessionStatus(w http.ResponseWriter, r *http.Request) { ... }
func (h *SessionHandler) StartSession(w http.ResponseWriter, r *http.Request) { ... }
func (h *SessionHandler) PauseSession(w http.ResponseWriter, r *http.Request) { ... }
func (h *SessionHandler) ResumeSession(w http.ResponseWriter, r *http.Request) { ... }
func (h *SessionHandler) StopSession(w http.ResponseWriter, r *http.Request) { ... }
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) { ... }
```

---

## Implementation Steps
1. Create `session_types.go` with all struct definitions and Store methods
2. Create `session_handler.go` with all handler methods
3. Delete `session.go`
4. Run `go build ./internal/api/...`

## Verification
- `go build ./internal/api/...` succeeds
- `go test ./internal/api/...` passes
- `session_types.go` < 200 lines
- `session_handler.go` < 300 lines
