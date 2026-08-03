# Task 5.1: Update HTMX Render Handler

## Goal
Update the HTMX render handler to reflect the 4-phase flow. Remove all planning-related UI elements and update the phase tracker.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/handlers/htmx_render.go`

## Detailed Steps

### Step 1: Update PhaseRenderData Struct
**Remove these fields from `PhaseRenderData`:**
```go
// REMOVE:
PlanningSteps           []PhaseStep              `json:"planning_steps"`
CurrentStep             int                      `json:"current_step"`
Subtasks                []PhaseSubtask           `json:"subtasks"`
PlanningReviewPhase     bool                     `json:"planning_review_phase"`
PlanningReviewPhaseData *PlanningReviewPhaseData `json:"planning_review_phase_data,omitempty"`
```

**Add these fields:**
```go
// ADD:
FeatureRequest         string  `json:"feature_request"`
CurrentPhaseName       string  `json:"current_phase_name"`
TotalPhases            int     `json:"total_phases"`  // Changed from 6 to 4
GeneratedCode          string  `json:"generated_code,omitempty"`
TargetFile             string  `json:"target_file,omitempty"`
```

### Step 2: Update RenderPhase Handler
**Replace the entire `RenderPhase` method:**

**From:**
```go
func (h *HTMXRenderHandler) RenderPhase(w http.ResponseWriter, r *http.Request) {
	phase := extractPhaseFromPath(r.URL.Path)
	if phase == "" {
		api.WriteAppError(w, apperrors.BadRequest("phase is required", "A phase name must be provided.", nil))
		return
	}

	var data PhaseRenderData
	sessions := h.sessionStore.ListSessions()
	if len(sessions) > 0 {
		session := sessions[0]
		data.GoalDescription = session.Goal
		data.TotalPhases = 6

		data.CurrentStep = getPhaseStep(phase)
		data.PlanningSteps = []PhaseStep{
			{Name: "Analyze requirements", Description: "Understand project scope and goals"},
			{Name: "Break down tasks", Description: "Create atomic units of work"},
			{Name: "Generate execution plan", Description: "Order tasks and define dependencies"},
		}
		// ... rest of planning logic
	}
	// ...
}
```

**To:**
```go
func (h *HTMXRenderHandler) RenderPhase(w http.ResponseWriter, r *http.Request) {
	phase := extractPhaseFromPath(r.URL.Path)
	if phase == "" {
		api.WriteAppError(w, apperrors.BadRequest("phase is required", "A phase name must be provided.", nil))
		return
	}

	var data PhaseRenderData
	sessions := h.sessionStore.ListSessions()
	if len(sessions) > 0 {
		session := sessions[0]
		data.FeatureRequest = session.Goal
		data.TotalPhases = 4  // coding, testing, review, human_review

		// Set phase-specific data
		switch session.CurrentPhase {
		case state.PhaseCoding:
			data.CodingPhase = true
			data.CodingPhaseData = &CodingPhaseData{
				Status:        "running",
				CurrentFile:   "generating...",
				GeneratedCode: "",
			}
		case state.PhaseTesting:
			data.TestingPhase = true
			data.TestingPhaseData = &TestingPhaseData{
				Status: "running",
			}
		case state.PhaseReview:
			data.ReviewPhase = true
			data.ReviewPhaseData = &ReviewPhaseData{
				Status: "running",
			}
		case state.PhaseHumanReview:
			data.HumanReviewPhase = true
			data.HumanReviewPhaseData = &HumanReviewPhaseData{
				CurrentPhase: "human_review",
				Output:       "Review pending...",
			}
		}

		// Add session-specific data
		data.CurrentPhaseName = getCurrentPhaseName(session)
	}

	rendered, err := h.templateEngine.RenderPhasePartial(phase, data)
	if err != nil {
		api.WriteAppError(w, apperrors.Internal("template rendering failed", "Failed to render the phase component.", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}
```

### Step 3: Update getPhaseStep() Function
**Replace:**
```go
func getPhaseStep(phase string) int {
	switch lower(phase) {
	case "planning":
		return 0
	case "planning_review":
		return 1
	case "coding":
		return 2
	case "testing":
		return 3
	case "review":
		return 4
	case "human_review":
		return 5
	default:
		return 0
	}
}
```

**To:**
```go
func getPhaseStep(phase string) int {
	switch lower(phase) {
	case "coding":
		return 0
	case "testing":
		return 1
	case "review":
		return 2
	case "human_review":
		return 3
	default:
		return 0
	}
}
```

### Step 4: Update getCurrentPhaseInfo() Function
**Replace:**
```go
func getCurrentPhaseInfo(session *state.Session) (string, string) {
	switch session.CurrentPhase {
	case state.PhasePlanning:
		return "Planning", "in_progress"
	case state.PhasePlanningReview:
		return "Planning Review", "in_progress"
	case state.PhaseCoding:
		return "Coding", "in_progress"
	case state.PhaseTesting:
		return "Testing", "in_progress"
	case state.PhaseReview:
		return "Review", "in_progress"
	case state.PhaseHumanReview:
		return "Human Review", "in_progress"
	default:
		return "Unknown", "pending"
	}
}
```

**To:**
```go
func getCurrentPhaseInfo(session *state.Session) (string, string) {
	switch session.CurrentPhase {
	case state.PhaseCoding:
		return "Coding", "in_progress"
	case state.PhaseTesting:
		return "Testing", "in_progress"
	case state.PhaseReview:
		return "Review", "in_progress"
	case state.PhaseHumanReview:
		return "Human Review", "in_progress"
	default:
		return "Unknown", "pending"
	}
}
```

### Step 5: Add getCurrentPhaseName() Helper
**Add this new helper:**
```go
func getCurrentPhaseName(session *state.Session) string {
	switch session.CurrentPhase {
	case state.PhaseCoding:
		return "Coding"
	case state.PhaseTesting:
		return "Testing"
	case state.PhaseReview:
		return "Review"
	case state.PhaseHumanReview:
		return "Human Review"
	default:
		return "Unknown"
	}
}
```

### Step 6: Update RenderPhaseTracker Handler
**Replace the phase list:**

**From:**
```go
allPhases := []PhaseTrackerItem{
	{Name: "Planning", Status: "pending", MaxRetries: 3},
	{Name: "Planning Review", Status: "pending", MaxRetries: 3},
	{Name: "Coding", Status: "pending", MaxRetries: 3},
	{Name: "Testing", Status: "pending", MaxRetries: 3},
	{Name: "Review", Status: "pending", MaxRetries: 3},
	{Name: "Human Review", Status: "pending", MaxRetries: 3},
}
```

**To:**
```go
allPhases := []PhaseTrackerItem{
	{Name: "Coding", Status: "pending", MaxRetries: 3},
	{Name: "Testing", Status: "pending", MaxRetries: 3},
	{Name: "Review", Status: "pending", MaxRetries: 3},
	{Name: "Human Review", Status: "pending", MaxRetries: 3},
}
```

**Update the switch logic to match 4 phases:**
```go
for i := range allPhases {
	switch session.CurrentPhase {
	case state.PhaseCoding:
		if i == 0 {
			allPhases[i].Status = "in_progress"
		}
	case state.PhaseTesting:
		if i < 2 {
			allPhases[i].Status = "completed"
		}
		if i == 1 {
			allPhases[i].Status = "in_progress"
		}
	case state.PhaseReview:
		if i < 3 {
			allPhases[i].Status = "completed"
		}
		if i == 2 {
			allPhases[i].Status = "in_progress"
		}
	case state.PhaseHumanReview:
		if i < 4 {
			allPhases[i].Status = "completed"
		}
		if i == 3 {
			allPhases[i].Status = "in_progress"
		}
	}

	if session.Status == state.SessionStatusCompleted {
		for j := range allPhases {
			allPhases[j].Status = "completed"
		}
	}
}
```

### Step 7: Update RenderDashboard Handler
**Apply the same 4-phase changes to `RenderDashboard()` as in `RenderPhaseTracker()`.**

### Step 8: Update RenderMainPage Handler
**Update to reflect new session creation flow:**

```go
func (h *HTMXRenderHandler) RenderMainPage(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})

	sessions := h.sessionStore.ListSessions()
	if len(sessions) > 0 {
		session := sessions[0]
		data["SessionID"] = session.ID
		data["FeatureRequest"] = session.Goal
		data["SessionStatus"] = string(session.Status)

		currentPhaseName, currentPhaseStatus := getCurrentPhaseInfo(session)
		data["CurrentPhaseName"] = currentPhaseName
		data["CurrentPhaseStatus"] = currentPhaseStatus
		data["PhaseProgress"] = getPhaseProgress(session.Status)
	} else {
		data["SessionID"] = "no-active-session"
		data["CurrentPhaseName"] = "Idle"
		data["CurrentPhaseStatus"] = "pending"
		data["PhaseProgress"] = 0
	}

	// Project data
	projects := h.projectStore.ListProjects()
	if len(projects) > 0 {
		project := projects[0]
		data["ProjectName"] = project.Name
		data["ProjectPath"] = project.Path

		entries, err := h.projectStore.ListProjectFiles(project.ID, "")
		if err == nil {
			items := make([]FileSystemItem, 0, len(entries))
			for _, entry := range entries {
				items = append(items, FileSystemItem{
					Name:  entry.Name,
					Path:  entry.Path,
					IsDir: entry.IsDir,
					Size:  entry.Size,
				})
			}
			data["RootItems"] = items
		}
	} else {
		data["ProjectName"] = "No Project"
		data["ProjectPath"] = "Please open or create a project"
		data["RootItems"] = []FileSystemItem{}
	}

	data["CurrentPath"] = ""
	data["ModelName"] = "gpt-4o"
	data["ProviderName"] = "OpenAI"

	h.templateEngine.RenderMain(w, "ide", data)
}
```

## Verification
- `go build ./internal/api/handlers/` — compiles
- Phase tracker shows 4 phases (Coding, Testing, Review, Human Review)
- `getPhaseStep()` returns 0-3 for the 4 phases
- `getCurrentPhaseInfo()` handles only 4 phases
- Dashboard and main page updated
