# Task 6.1: Update Unit Tests

## Goal
Update all unit tests to work with the new simplified workflow. Remove plan-based tests and add prompt-based tests.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/orchestrator/orchestrator_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/orchestrator/phase_router_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/state/store_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/orchestrator_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/coder_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/reviewer_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/tester_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/prompts/coder_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/prompts/reviewer_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/prompts/tester_test.go`

## Detailed Steps

### Step 1: Update orchestrator/orchestrator_test.go
**Remove all tests that reference `Plan`, `PlanUnit`, or plan-related methods.**

**Add new tests for:**
```go
func TestStartSession(t *testing.T) {
	// Test that StartSession initializes a session correctly
	// without creating a plan
}

func TestRunCoding(t *testing.T) {
	// Test that runCoding calls coder agent with user prompt
	// and writes code to a file
}

func TestRunTesting(t *testing.T) {
	// Test that runTesting runs tests on generated code
	// and calls tester agent
}

func TestRunReview(t *testing.T) {
	// Test that runReview calls reviewer agent with code and prompt
}

func TestRunHumanReview(t *testing.T) {
	// Test that runHumanReview creates a human gate
	// and waits for approval
}

func TestTransitionToCompleted(t *testing.T) {
	// Test transition to completed state
}
```

### Step 2: Update orchestrator/phase_router_test.go
**Remove tests for planning phase transitions.**

**Update remaining tests:**
```go
func TestValidTransitions(t *testing.T) {
	// Test: coding → testing (valid)
	// Test: testing → coding (valid, retry)
	// Test: testing → review (valid)
	// Test: review → coding (valid, retry)
	// Test: review → human_review (valid)
	// Test: human_review → coding (valid, edit)
	// Test: human_review → completed (valid)
	// Test: coding → review (invalid)
}

func TestValidateTransition(t *testing.T) {
	// Same transitions as above
}

func TestNextPhase(t *testing.T) {
	// Test NextPhase for each valid current phase
}
```

### Step 3: Update state/store_test.go
**Remove all tests for:**
- `SavePlan`
- `GetPlan`
- `GetPendingUnits`
- `UpdateUnitStatus`
- `GetPlanBySession`
- `SaveUnitCode`

**Update tests for:**
- `SaveSession` — still valid
- `GetSession` — still valid
- `SavePhaseHistory` — still valid
- `SaveTestResult` — still valid
- `SaveReviewReport` — still valid

### Step 4: Update agent/orchestrator_test.go
**Remove tests for old `RunCoder(unit PlanUnit)` method.**

**Add tests for:**
```go
func TestRunCoderFromPrompt(t *testing.T) {
	// Test with empty prompt (error)
	// Test with valid prompt
}

func TestRunReviewer(t *testing.T) {
	// Test with empty code (error)
	// Test with empty prompt (error)
	// Test with valid inputs
}
```

### Step 5: Update agent/coder_test.go
**Remove tests that use `PlanUnit` struct.**

**Add tests for new input format.**

### Step 6: Update agent/reviewer_test.go
**Update tests to use `userPrompt` instead of `plan`.**

### Step 7: Update agent/tester_test.go
**No changes needed** — tester still works with code + test results.

### Step 8: Update prompts/coder_test.go
**Remove tests for `BuildCoderPrompt(unit, existingCode, skills)`.**

**Add tests for:**
```go
func TestBuildCoderPromptFromRequest(t *testing.T) {
	// Test with empty prompt (error)
	// Test with valid prompt only
	// Test with prompt and project context
	// Test with prompt and target file
}
```

### Step 9: Update prompts/reviewer_test.go
**Remove tests for `BuildReviewerPrompt(code, plan, skills)`.**

**Add tests for:**
```go
func TestBuildReviewerPromptFromRequest(t *testing.T) {
	// Test with empty code (error)
	// Test with empty user request (error)
	// Test with valid inputs
}
```

### Step 10: Update prompts/tester_test.go
**No changes needed** — tester prompt still works with code + test results.

## Verification
- `go test ./internal/orchestrator/` — all tests pass
- `go test ./internal/state/` — all tests pass
- `go test ./internal/agent/` — all tests pass
- `go test ./internal/agent/prompts/` — all tests pass
- No references to `Plan`, `PlanUnit`, `PlanStatus`, `UnitStatus` in test files
