package state

import (
	"fmt"
	"time"
)

// Phase represents a phase in the Mini-Orca workflow.
type Phase string

const (
	PhasePlanning       Phase = "planning"
	PhasePlanningReview Phase = "planning_review"
	PhaseCoding         Phase = "coding"
	PhaseTesting        Phase = "testing"
	PhaseReview         Phase = "review"
	PhaseHumanReview    Phase = "human_review"
	PhaseComplete       Phase = "complete"
	PhaseCancelled      Phase = "cancelled"
)

// PhaseTransition represents a valid transition between phases.
type PhaseTransition struct {
	From Phase
	To   Phase
}

// PhaseFlow defines the valid phase transitions.
var PhaseFlow = map[Phase][]Phase{
	PhasePlanning:       {PhasePlanningReview},
	PhasePlanningReview: {PhaseCoding},
	PhaseCoding:         {PhaseTesting, PhasePlanningReview},
	PhaseTesting:        {PhaseReview, PhasePlanningReview},
	PhaseReview:         {PhaseHumanReview, PhasePlanningReview},
	PhaseHumanReview:    {PhasePlanningReview, PhaseComplete, PhaseCancelled},
	PhaseComplete:       {},
	PhaseCancelled:      {},
}

// IsTerminal returns true if the phase is a terminal state.
func (p Phase) IsTerminal() bool {
	return p == PhaseComplete || p == PhaseCancelled
}

// RequiresHumanApproval returns true if this phase requires human approval.
func (p Phase) RequiresHumanApproval() bool {
	return p == PhasePlanningReview || p == PhaseHumanReview
}

// Session represents a complete Mini-Orca session.
type Session struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ProjectDir  string    `json:"project_dir"`
	Phase       Phase     `json:"phase"`
	Plan        *Plan     `json:"plan,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	History     []Event   `json:"history"`
}

// NewSession creates a new session.
func NewSession(id, name, description, projectDir string) *Session {
	now := time.Now()
	return &Session{
		ID:          id,
		Name:        name,
		Description: description,
		ProjectDir:  projectDir,
		Phase:       PhasePlanning,
		Plan:        NewPlan(id, id, ""),
		CreatedAt:   now,
		UpdatedAt:   now,
		History:     []Event{},
	}
}

// AddEvent adds an event to the session history.
func (s *Session) AddEvent(event Event) {
	s.History = append(s.History, event)
	s.UpdatedAt = time.Now()
}

// Plan represents the development plan created during the planning phase.
type Plan struct {
	ID            string        `json:"id"`
	SessionID     string        `json:"session_id"`
	Description   string        `json:"description"`
	AtomicUnits   []AtomicUnit  `json:"atomic_units"`
	Status        string        `json:"status"` // "draft", "approved", "in_progress", "completed"
	CreatedAt     time.Time     `json:"created_at"`
	ApprovedAt    time.Time     `json:"approved_at,omitempty"`
}

// NewPlan creates a new plan.
func NewPlan(id, sessionID, description string) *Plan {
	return &Plan{
		ID:          id,
		SessionID:   sessionID,
		Description: description,
		AtomicUnits: []AtomicUnit{},
		Status:      "draft",
		CreatedAt:   time.Now(),
	}
}

// AddAtomicUnit adds an atomic unit to the plan.
func (p *Plan) AddAtomicUnit(unit AtomicUnit) {
	p.AtomicUnits = append(p.AtomicUnits, unit)
}

// GetPendingUnits returns atomic units that haven't been completed.
func (p *Plan) GetPendingUnits() []AtomicUnit {
	var pending []AtomicUnit
	for _, u := range p.AtomicUnits {
		if u.Status != UnitStatusComplete {
			pending = append(pending, u)
		}
	}
	return pending
}

// GetCompletedUnits returns atomic units that have been completed.
func (p *Plan) GetCompletedUnits() []AtomicUnit {
	var completed []AtomicUnit
	for _, u := range p.AtomicUnits {
		if u.Status == UnitStatusComplete {
			completed = append(completed, u)
		}
	}
	return completed
}

// AtomicUnit represents a single unit of work (function, struct, class).
type AtomicUnit struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"` // "function", "struct", "class", "interface"
	File        string         `json:"file"`
	Description string         `json:"description"`
	Dependencies []string       `json:"dependencies"`
	Priority    int            `json:"priority"`
	Status      string         `json:"status"` // "pending", "in_progress", "testing", "review", "complete", "rejected", "edited"
	Code        string         `json:"code,omitempty"`
	TestCode    string         `json:"test_code,omitempty"`
	Review      *ReviewReport  `json:"review,omitempty"`
	Errors      []string       `json:"errors,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// UnitStatus constants
const (
	UnitStatusPending    = "pending"
	UnitStatusInProgress = "in_progress"
	UnitStatusTesting    = "testing"
	UnitStatusReview     = "review"
	UnitStatusComplete   = "complete"
	UnitStatusRejected   = "rejected"
	UnitStatusEdited     = "edited"
)

// ReviewReport contains the results of a code review.
type ReviewReport struct {
	Rating       int      `json:"rating"`
	Strengths    []string `json:"strengths"`
	Issues       []string `json:"issues"`
	Recommendations []string `json:"recommendations"`
	Verdict      string   `json:"verdict"` // "approve", "reject", "requires_revision"
	Reviewer     string   `json:"reviewer"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

// Event represents an event in the session history.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "phase_change", "unit_start", "unit_complete", "human_approval", "error"
	Phase     Phase     `json:"phase"`
	UnitID    string    `json:"unit_id,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NewEvent creates a new event.
func NewEvent(id, eventType string, phase Phase, message string) Event {
	return Event{
		ID:        id,
		Type:      eventType,
		Phase:     phase,
		Message:   message,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
	}
}

// ValidateSession validates the session state.
func (s *Session) ValidateSession() error {
	if s.ID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	if s.Name == "" {
		return fmt.Errorf("session name cannot be empty")
	}
	if s.Phase == "" {
		return fmt.Errorf("session phase cannot be empty")
	}
	return nil
}

// GetCurrentPhase returns the current phase with validation.
func (s *Session) GetCurrentPhase() Phase {
	return s.Phase
}

// CanTransitionTo checks if a phase transition is valid.
func (s *Session) CanTransitionTo(to Phase) bool {
	validTransitions, ok := PhaseFlow[s.Phase]
	if !ok {
		return false
	}

	for _, valid := range validTransitions {
		if valid == to {
			return true
		}
	}
	return false
}

// TransitionTo attempts to transition to a new phase.
func (s *Session) TransitionTo(to Phase) error {
	if !s.CanTransitionTo(to) {
		return fmt.Errorf("invalid phase transition from %s to %s", s.Phase, to)
	}

	oldPhase := s.Phase
	s.Phase = to
	s.AddEvent(NewEvent("", "phase_change", to, fmt.Sprintf("Phase changed from %s to %s", oldPhase, to)))

	return nil
}
