// Package state provides persistence and retrieval of orchestrator state.
package state

import "time"

// Phase represents a phase in the orchestrator flow.
type Phase string

const (
	PhaseCoding      Phase = "coding"
	PhaseTesting     Phase = "testing"
	PhaseReview      Phase = "review"
	PhaseHumanReview Phase = "human_review"
)

// SessionStatus represents the current status of a session.
type SessionStatus string

const (
	SessionStatusPending   SessionStatus = "pending"
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// PhaseStatus represents the status of a phase execution.
type PhaseStatus string

const (
	PhaseStatusPending    PhaseStatus = "pending"
	PhaseStatusInProgress PhaseStatus = "in_progress"
	PhaseStatusCompleted  PhaseStatus = "completed"
	PhaseStatusFailed     PhaseStatus = "failed"
	PhaseStatusSkipped    PhaseStatus = "skipped"
)

// PhaseHistory records the history of a single phase execution.
type PhaseHistory struct {
	// ID is the unique identifier for the phase history entry.
	ID string
	// SessionID is the ID of the session this phase belongs to.
	SessionID string
	// Phase is the phase that was executed.
	Phase string
	// StartedAt is when the phase execution started.
	StartedAt time.Time
	// CompletedAt is when the phase execution completed.
	CompletedAt time.Time
	// Status is the result status of the phase execution.
	Status PhaseStatus
	// Output contains the output of the phase execution.
	Output string
	// Error contains any error that occurred during the phase execution.
	Error string
	// RetryCount is the number of times this phase was retried.
	RetryCount int
}

// TestResult represents the result of a test execution for a plan unit.
type TestResult struct {
	// ID is the unique identifier for the test result.
	ID string
	// SessionID is the ID of the session this test result belongs to.
	SessionID string
	// UnitID is the ID of the unit this test result is for.
	UnitID string
	// TestOutput is the raw output from the test execution.
	TestOutput string
	// Passed indicates whether the tests passed.
	Passed bool
	// Failures are the list of test failures.
	Failures []string
	// Suggestions are the suggestions from the tester agent.
	Suggestions []string
	// Coverage is the code coverage percentage.
	Coverage string
	// Summary is a summary of the test results.
	Summary string
	// ExecutedAt is when the tests were executed.
	ExecutedAt time.Time
}

// ReviewReportEntry represents the result of a code review for a plan unit.
type ReviewReportEntry struct {
	// ID is the unique identifier for the review report.
	ID string
	// SessionID is the ID of the session this review report belongs to.
	SessionID string
	// UnitID is the ID of the unit this review report is for.
	UnitID string
	// Issues are the issues found during review.
	Issues []string
	// Suggestions are the suggestions from the reviewer agent.
	Suggestions []string
	// Score is the review score (0-100).
	Score int
	// Recommendation is the reviewer's recommendation.
	Recommendation string
	// Summary is a summary of the review.
	Summary string
	// ReviewedAt is when the review was performed.
	ReviewedAt time.Time
}

// Session represents a single execution session within the pipeline.
type Session struct {
	// ID is the unique identifier for the session (UUID).
	ID string
	// Goal is the high-level goal of the session.
	Goal string
	// ProjectPath is the path to the project directory.
	ProjectPath string
	// ProjectType is the type of the project (e.g., "go", "python").
	ProjectType string
	// CreatedAt is when the session was created.
	CreatedAt time.Time
	// UpdatedAt is when the session was last updated.
	UpdatedAt time.Time
	// CurrentPhase is the current phase of the session.
	CurrentPhase Phase
	// Status is the current status of the session.
	Status SessionStatus
	// History contains the execution history of each phase.
	History []PhaseHistory
	// TestResults contains the test results for each unit.
	TestResults []TestResult
	// ReviewReports contains the review reports for each unit.
	ReviewReports []ReviewReportEntry
	// Error contains any error that occurred during the session.
	Error string
}
