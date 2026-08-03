# Task 4.1: Update Session API

## Goal
Update the session API for the new single-feature workflow.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/session.go`

## Detailed Steps

### Step 1: Update SessionCreateRequest
**Replace:**
```go
type SessionCreateRequest struct {
	Goal          string `json:"goal"`
	ProjectPath   string `json:"project_path"`
	ProjectType   string `json:"project_type,omitempty"`
}
```
**With:**
```go
type SessionCreateRequest struct {
	FeatureRequest string `json:"feature_request"`
	ProjectPath    string `json:"project_path"`
	ProjectType    string `json:"project_type,omitempty"`
}
```

### Step 2: Update SessionResponse
**Replace `Goal` field with `FeatureRequest` field.**

### Step 3: Update SessionSummary
**Replace `Goal` field with `FeatureRequest` field.**

### Step 4: Update SessionStore.CreateSession()
```go
func (s *SessionStore) CreateSession(featureRequest, projectPath, projectType string) (*state.Session, error) {
	if featureRequest == "" {
		return nil, fmt.Errorf("session: feature_request is required")
	}
	if projectPath == "" {
		return nil, fmt.Errorf("session: project_path is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	session := &state.Session{
		ID: sessionID, Goal: featureRequest, ProjectPath: projectPath,
		ProjectType: projectType, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		CurrentPhase: state.PhaseCoding, Status: state.SessionStatusPending,
		History: nil, TestResults: nil, ReviewReports: nil, Error: "",
	}
	s.sessions[sessionID] = session
	return session, nil
}
```

### Step 5: Update SessionHandler.CreateSession()
```go
func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req SessionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}
	if req.FeatureRequest == "" {
		WriteAppError(w, apperrors.BadRequest("feature_request is required", "A feature request must be provided.", nil))
		return
	}
	if req.ProjectPath == "" {
		WriteAppError(w, apperrors.BadRequest("project_path is required", "A project path must be provided.", nil))
		return
	}
	session, err := h.sessionStore.CreateSession(req.FeatureRequest, req.ProjectPath, req.ProjectType)
	if err != nil {
		WriteAppError(w, apperrors.BadRequest("session creation failed", "Failed to create new session: "+err.Error(), err))
		return
	}
	WriteJSON(w, http.StatusCreated, SessionResponse{
		ID: session.ID, FeatureRequest: session.Goal, ProjectPath: session.ProjectPath,
		ProjectType: session.ProjectType, CurrentPhase: string(state.PhaseCoding),
		Status: string(session.Status), CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
	})
}
```

### Step 6: Update GetSessionStatus and ListSessions handlers
Replace `Goal` with `FeatureRequest` in responses.

### Step 7: Update StartSession handler
Ensure it sets phase to `PhaseCoding` (not `PhasePlanning`).

## Verification
- `go build ./internal/api/` — compiles
- `SessionCreateRequest` uses `FeatureRequest` field
- Sessions start at `PhaseCoding`
