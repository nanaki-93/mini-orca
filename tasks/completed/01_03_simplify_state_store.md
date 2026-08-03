# Task 1.3: Simplify State Store

## Goal
Remove all plan-related methods from the state store.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/state/store.go`

## Detailed Steps

### Step 1: Remove these methods entirely:
- `SavePlan(plan *Plan) error`
- `GetPlan(planID string) (*Plan, error)`
- `GetPendingUnits(planID string) ([]PlanUnit, error)`
- `UpdateUnitStatus(unitID string, status UnitStatus) error`
- `GetPlanBySession(sessionID string) (*Plan, error)`
- `SaveUnitCode(sessionID string, unitID string, code string) error`
- `copyPlanData(plan *Plan) *Plan`

### Step 2: Update InitStateStore()
**Remove `AtomicUnits: nil` from session creation.**

### Step 3: Update copySessionData()
**Remove these blocks:**
```go
if session.Plan != nil {
	planCopy := s.copyPlanData(session.Plan)
	sessCopy.Plan = planCopy
}
if session.AtomicUnits != nil {
	unitsCopy := make([]AtomicUnit, len(session.AtomicUnits))
	copy(unitsCopy, session.AtomicUnits)
	sessCopy.AtomicUnits = unitsCopy
}
```

## Verification
- `go build ./internal/state/` — compiles
- No references to `Plan`, `PlanUnit`, `PlanStatus`, `UnitStatus`, `AtomicUnit` in store.go
- `InitStateStore()` creates a session without Plan or AtomicUnits
