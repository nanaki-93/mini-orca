# Task 5.2: Update Project Handler

## Goal
The project handler mostly stays the same — it handles opening existing projects. Minor updates may be needed for the new session creation flow.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/api/handlers/project.go`

## Detailed Steps

### Step 1: Review ProjectCreateRequest
No changes needed. The project creation still requires a path and optionally a name/type.

### Step 2: Review ProjectHandler Methods
All methods stay the same:
- `ListProjects` — unchanged
- `CreateProject` — unchanged
- `ListFiles` — unchanged
- `GetFileContent` — unchanged

### Step 3: Verify ProjectStore
No changes needed. Project storage is independent of the session workflow.

### Step 4: Verify Helper Functions
All helper functions stay the same:
- `detectProjectTypeFromDir` — unchanged
- `countFiles` — unchanged
- `scanDirectory` — unchanged
- `detectContentType` — unchanged
- `resolveProjectPath` — unchanged

## Verification
- No changes needed in project.go
- `go build ./internal/api/handlers/` — compiles (may compile due to other changes in this phase)
- Project API remains functional for existing project management
