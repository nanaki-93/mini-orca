// Package state provides session and plan management for Mini-Orca.
//
// The state package manages the persistent state of Mini-Orca sessions,
// including the workflow phase, execution plan, and activity history.
//
// # Session
//
// A session represents a single Mini-Orca run. It tracks:
//   - ID: Unique identifier (UUID)
//   - Name: Human-readable name
//   - Description: Project description
//   - Phase: Current workflow phase
//   - Plan: The execution plan with atomic units
//   - History: Timestamped activity log
//   - CreatedAt/UpdatedAt: Lifecycle timestamps
//
// # Plan
//
// A plan breaks down the project into atomic units of work:
//   - ID: Unique identifier
//   - SessionID: Associated session
//   - Status: pending, in_progress, complete
//   - AtomicUnits: List of functions/structs/classes to implement
//
// Each atomic unit has:
//   - ID, Name, Type (function/struct/class)
//   - Description, File, LineRange
//   - Status, TestCode, ReviewComments
//
// # Phase Flow
//
// Sessions follow a strict phase transition flow:
//
//	planning → planning_review → coding → testing → review → human_review → complete
//	                                                                  ↓
//	                                                              cancelled
//
// Invalid transitions are rejected (e.g., skipping from planning to coding).
// Terminal phases (complete, cancelled) cannot transition further.
//
// # Persistence
//
// State is persisted to JSON files in a base directory:
//
//	<base_dir>/
//	  <session_id>/
//	    session.json    # Session state
//	    plan.json        # Execution plan
//
// # Example
//
//	store, _ := state.NewStore(".mini-orca/state")
//	session := state.NewSession("", "My Project", "Description", "/path/to/project")
//	store.CreateSession(session)
//
//	plan := state.NewPlan("plan-1", session.ID, "Implementation Plan")
//	plan.AddAtomicUnit(state.AtomicUnit{ID: "au-001", Name: "UserAuth", Type: "function"})
//	store.SavePlan(plan)
package state
