// Package orchestrator manages the 5-phase workflow of Mini-Orca.
//
// The orchestrator coordinates the four agents (Planner, Coder, Tester,
// Reviewer) through a state machine that enforces a strict workflow:
//
//	Planning → Planning Review → Coding → Testing → Review → Human Review → Complete
//
// # Workflow
//
//  1. Planning: Planner analyzes requirements and creates an execution plan
//     with atomic units (functions, structs, classes)
//
//  2. Planning Review: Human reviews and approves the plan
//
//  3. Coding Loop (repeat until all units complete):
//     a. Coding: Coder implements one atomic unit
//     b. Testing: Tester writes and runs tests
//     c. Review: Reviewer checks code quality
//
//  4. Human Review: Human reviews all changes before completion
//
// # Human Gates
//
// The orchestrator uses a human gate mechanism to pause execution
// at key decision points:
//
//	ApprovePhase(phase)  → Continue to next phase
//	RejectPhase(phase)   → Return to previous phase
//
// # Retry Logic
//
// Agent executions are wrapped with retry logic:
//   - maxRetries: 3 (configurable)
//   - retryDelay: 2s * attempt (exponential backoff)
//   - Only retryable errors trigger retries
//
// # Standalone Function Writer
//
// The orchestrator also supports a standalone mode for writing
// individual functions without the full workflow:
//
//	insertionManager := orchestrator.NewInsertionManager(orchestrator)
//	insertionManager.WriteStandaloneFunction(ctx, request)
//
// # Example
//
//	orch := orchestrator.NewOrchestrator(session, router, agents, ...)
//	go func() {
//	    if err := orch.Start(ctx); err != nil {
//	        log.Fatal(err)
//	    }
//	}()
//
//	// Later: approve or reject
//	orch.ApprovePhase(state.PhasePlanningReview, "Plan looks good")
package orchestrator
