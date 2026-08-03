# Task 1.1: Remove Planning-Related Types from state/session.go

## Goal
Remove all planning-related data types from the state package.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/state/session.go`

## Detailed Steps

### Step 1: Remove Plan Status Constants
**Remove these constants entirely:**
```go
type PlanStatus string
const (
	PlanStatusDraft      PlanStatus = "draft"
	PlanStatusApproved   PlanStatus = "approved"
	PlanStatusRejected   PlanStatus = "rejected"
	PlanStatusInProgress PlanStatus = "in_progress"
	PlanStatusCompleted  PlanStatus = "completed"
)
```

### Step 2: Remove Unit Status Constants
**Remove these constants entirely:**
```go
type UnitStatus string
const (
	UnitStatusPending   UnitStatus = "pending"
	UnitStatusCoding    UnitStatus = "coding"
	UnitStatusTesting   UnitStatus = "testing"
	UnitStatusReview    UnitStatus = "review"
	UnitStatusCompleted UnitStatus = "completed"
	UnitStatusFailed    UnitStatus = "failed"
)
```

### Step 3: Remove PlanUnit Struct
**Remove the entire `PlanUnit` struct.**

### Step 4: Remove Plan Struct
**Remove the entire `Plan` struct.**

### Step 5: Remove AtomicUnit Struct
**Remove the entire `AtomicUnit` struct.**

### Step 6: Remove TestDefinition Struct
**Remove the entire `TestDefinition` struct.**

### Step 7: Remove ReviewComment Struct
**Remove the entire `ReviewComment` struct.**

### Step 8: Simplify Session Struct
**Remove these fields from `Session`:**
```go
Plan          *Plan          // REMOVE
AtomicUnits   []AtomicUnit   // REMOVE
```

**Keep these types UNTOUCHED:**
- `Phase` and all phase constants
- `SessionStatus` and all status constants
- `PhaseStatus` and all phase status constants
- `PhaseHistory`
- `TestResult`
- `ReviewReportEntry`

## Verification
- `go build ./internal/state/` — compiles
- No references to `Plan`, `PlanUnit`, `PlanStatus`, `UnitStatus`, `AtomicUnit` in session.go
