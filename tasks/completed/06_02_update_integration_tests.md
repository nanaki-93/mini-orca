# Task 6.2: Update Integration Tests

## Goal
Update integration tests to work with the new simplified workflow.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/integration_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/model/full_flow_test.go`
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/config/integration_test.go`

## Detailed Steps

### Step 1: Update agent/integration_test.go
**Remove any tests that create plans or use multi-unit workflows.**

**Add new integration test:**
```go
func TestFullFeatureWorkflow(t *testing.T) {
	// Skip if LLM not available
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// 1. Start session with feature request
	// 2. Run coding phase
	// 3. Run testing phase
	// 4. Run review phase
	// 5. Verify human gate can be created
}
```

### Step 2: Update model/full_flow_test.go
**This file needs a complete rewrite for the new flow.**

**New test structure:**
```go
func TestFullFlow_CodingToHumanGate(t *testing.T) {
	// Create mock router, executor, config
	// Create orchestrator
	// Start session
	// Run coding phase
	// Verify code was generated
	// Run testing phase
	// Verify test results saved
	// Run review phase
	// Verify review report saved
	// Run human review phase
	// Verify human gate created
}

func TestFullFlow_RetryOnTestFailure(t *testing.T) {
	// Test that test failure sends back to coding
}

func TestFullFlow_RetryOnReviewFailure(t *testing.T) {
	// Test that review failure sends back to coding
}
```

### Step 3: Update config/integration_test.go
**Review and update any config tests that reference planning phases.**

**Update phase-related config validation:**
```go
func TestConfig_Phases(t *testing.T) {
	// Verify config accepts the 4 active phases
	// Remove references to planning phases
}
```

## Verification
- `go test -tags=integration ./...` — integration tests pass (where LLM available)
- `go test ./...` — all unit tests pass
- No compilation errors
