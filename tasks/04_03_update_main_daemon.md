# Task 4.3: Update Main Daemon

## Goal
Simplify `cmd/daemon/main.go` to remove planning-related initialization and update the HTTP server setup for the new 4-phase flow.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/cmd/daemon/main.go`

## Detailed Steps

### Step 1: Simplify Phase Router Initialization
**Find the phase router creation in `startHTTPServer()` and simplify:**

**From:**
```go
// Create phase router for session lifecycle
currentSession := &state.Session{
	ID:           "default",
	CurrentPhase: state.PhaseCoding,
	Status:       state.SessionStatusPending,
}
phaseRouter := orchestrator.NewPhaseRouter(currentSession, nil)
```

**To:**
```go
// Create phase router for session lifecycle
currentSession := &state.Session{
	ID:           "default",
	CurrentPhase: state.PhaseCoding,
	Status:       state.SessionStatusPending,
}
phaseRouter := orchestrator.NewPhaseRouter(currentSession, nil)
```
*(No change needed here — it already starts at PhaseCoding)*

### Step 2: Update Session Start Handler Route
**Find the POST /api/sessions/ route and update:**

**From:**
```go
mux.HandleFunc("POST /api/sessions/", func(w http.ResponseWriter, r *http.Request) {
	parts := api.SplitPath(r.URL.Path)
	if len(parts) >= 5 {
		action := parts[4]
		switch action {
		case "start":
			sessionHandler.StartSession(w, r)
		case "pause":
			sessionHandler.PauseSession(w, r)
		case "resume":
			sessionHandler.ResumeSession(w, r)
		case "stop":
			sessionHandler.StopSession(w, r)
		case "gate":
			gateHandler.RespondToGate(w, r)
		}
	}
})
```

**To:**
```go
mux.HandleFunc("POST /api/sessions/", func(w http.ResponseWriter, r *http.Request) {
	parts := api.SplitPath(r.URL.Path)
	if len(parts) >= 5 {
		action := parts[4]
		switch action {
		case "start":
			sessionHandler.StartSession(w, r)
		case "pause":
			sessionHandler.PauseSession(w, r)
		case "resume":
			sessionHandler.ResumeSession(w, r)
		case "stop":
			sessionHandler.StopSession(w, r)
		case "gate":
			gateHandler.RespondToGate(w, r)
		}
	}
})
```
*(No change needed — the routing is the same, only the handler internals changed)*

### Step 3: Remove Planning-Related Log Messages
**In `main()`, remove or update any logging that references planning:**

Search for and update any mentions of "planning" in log messages. The startup logs should no longer reference planning phases.

### Step 4: Update Agent Initialization Log
**Update the agent registration logs to reflect the new flow:**

**From:**
```go
logging.Info("Agent registered", "name", coder.Name(), "description", coder.Name(), "skills", coder.GetSkills())
logging.Info("Agent registered", "name", tester.Name(), "description", tester.Description(), "skills", tester.GetSkills())
logging.Info("Agent registered", "name", reviewer.Name(), "description", reviewer.Description(), "skills", reviewer.GetSkills())
```

**To:**
```go
logging.Info("Agents initialized", "agents", []string{"coder", "tester", "reviewer"})
logging.Info("Workflow: coding → testing → review → human_gate")
```

### Step 5: Verify No Planning References
Search the entire file for "planning" and remove/update any references:
```bash
grep -n -i "planning" cmd/daemon/main.go
```

### Step 6: Verify Session Handler Dependencies
The session handler, gate handler, and project handler are still created the same way. No changes needed to their instantiation.

## Verification
- `go build ./cmd/daemon/` — compiles
- No references to `PhasePlanning` or `PhasePlanningReview` in main.go
- Startup log reflects new workflow
- All route handlers still wired correctly
