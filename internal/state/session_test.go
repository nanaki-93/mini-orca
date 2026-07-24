package state

import (
	"os"
	"testing"
)

func TestNewSession(t *testing.T) {
	session := NewSession("test-1", "Test Session", "A test session", "/tmp/test")

	if session.ID != "test-1" {
		t.Errorf("expected ID 'test-1', got '%s'", session.ID)
	}
	if session.Name != "Test Session" {
		t.Errorf("expected name 'Test Session', got '%s'", session.Name)
	}
	if session.Phase != PhasePlanning {
		t.Errorf("expected phase '%s', got '%s'", PhasePlanning, session.Phase)
	}
	if session.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if len(session.History) != 0 {
		t.Errorf("expected empty history, got %d events", len(session.History))
	}
}

func TestSessionAddEvent(t *testing.T) {
	session := NewSession("test-1", "Test Session", "A test session", "/tmp/test")
	event := NewEvent("evt-1", "phase_change", PhasePlanning, "Session started")

	session.AddEvent(event)

	if len(session.History) != 1 {
		t.Errorf("expected 1 event, got %d", len(session.History))
	}
	if session.History[0].Message != "Session started" {
		t.Errorf("expected message 'Session started', got '%s'", session.History[0].Message)
	}
	if session.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero after adding event")
	}
}

func TestSessionValidateSession(t *testing.T) {
	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{
			name:    "valid session",
			session: NewSession("test-1", "Test", "Desc", "/tmp"),
			wantErr: false,
		},
		{
			name:    "empty ID",
			session: &Session{Name: "Test", Description: "Desc", Phase: PhasePlanning},
			wantErr: true,
		},
		{
			name:    "empty name",
			session: &Session{ID: "test-1", Description: "Desc", Phase: PhasePlanning},
			wantErr: true,
		},
		{
			name:    "empty phase",
			session: &Session{ID: "test-1", Name: "Test", Description: "Desc"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.ValidateSession()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSession() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPhaseFlow(t *testing.T) {
	tests := []struct {
		from Phase
		to   Phase
		want bool
	}{
		{PhasePlanning, PhasePlanningReview, true},
		{PhasePlanningReview, PhaseCoding, true},
		{PhaseCoding, PhaseTesting, true},
		{PhaseTesting, PhaseReview, true},
		{PhaseReview, PhaseHumanReview, true},
		{PhaseHumanReview, PhaseComplete, true},
		{PhaseHumanReview, PhaseCancelled, true},
		{PhasePlanning, PhaseCoding, false},       // Invalid direct transition
		{PhasePlanning, PhaseComplete, false},      // Invalid skip
		{PhaseComplete, PhasePlanning, false},      // Terminal state
		{PhaseCancelled, PhasePlanning, false},     // Terminal state
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			session := NewSession("test-1", "Test", "Desc", "/tmp")
			session.Phase = tt.from
			got := session.CanTransitionTo(tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%s) = %v, want %v", tt.to, got, tt.want)
			}
		})
	}
}

func TestSessionTransitionTo(t *testing.T) {
	session := NewSession("test-1", "Test", "Desc", "/tmp")
	session.Phase = PhasePlanning

	err := session.TransitionTo(PhasePlanningReview)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if session.Phase != PhasePlanningReview {
		t.Errorf("expected phase '%s', got '%s'", PhasePlanningReview, session.Phase)
	}
	if len(session.History) != 1 {
		t.Errorf("expected 1 history event, got %d", len(session.History))
	}

	// Try invalid transition
	err = session.TransitionTo(PhaseComplete)
	if err == nil {
		t.Error("expected error for invalid transition, got nil")
	}
}

func TestPlanAddAtomicUnit(t *testing.T) {
	plan := NewPlan("plan-1", "session-1", "Test Plan")
	unit := AtomicUnit{
		ID:   "au-001",
		Name: "Test Function",
		Type: "function",
	}

	plan.AddAtomicUnit(unit)

	if len(plan.AtomicUnits) != 1 {
		t.Errorf("expected 1 atomic unit, got %d", len(plan.AtomicUnits))
	}
	if plan.AtomicUnits[0].ID != "au-001" {
		t.Errorf("expected unit ID 'au-001', got '%s'", plan.AtomicUnits[0].ID)
	}
}

func TestPlanGetPendingUnits(t *testing.T) {
	plan := NewPlan("plan-1", "session-1", "Test Plan")
	plan.AddAtomicUnit(AtomicUnit{ID: "au-001", Status: UnitStatusPending})
	plan.AddAtomicUnit(AtomicUnit{ID: "au-002", Status: UnitStatusComplete})
	plan.AddAtomicUnit(AtomicUnit{ID: "au-003", Status: UnitStatusInProgress})

	pending := plan.GetPendingUnits()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending units, got %d", len(pending))
	}
}

func TestPlanGetCompletedUnits(t *testing.T) {
	plan := NewPlan("plan-1", "session-1", "Test Plan")
	plan.AddAtomicUnit(AtomicUnit{ID: "au-001", Status: UnitStatusPending})
	plan.AddAtomicUnit(AtomicUnit{ID: "au-002", Status: UnitStatusComplete})
	plan.AddAtomicUnit(AtomicUnit{ID: "au-003", Status: UnitStatusComplete})

	completed := plan.GetCompletedUnits()
	if len(completed) != 2 {
		t.Errorf("expected 2 completed units, got %d", len(completed))
	}
}

func TestPhaseIsTerminal(t *testing.T) {
	tests := []struct {
		phase  Phase
		isTerm bool
	}{
		{PhasePlanning, false},
		{PhaseCoding, false},
		{PhaseComplete, true},
		{PhaseCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := tt.phase.IsTerminal(); got != tt.isTerm {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.isTerm)
			}
		})
	}
}

func TestPhaseRequiresHumanApproval(t *testing.T) {
	tests := []struct {
		phase         Phase
		requiresHuman bool
	}{
		{PhasePlanningReview, true},
		{PhaseHumanReview, true},
		{PhasePlanning, false},
		{PhaseCoding, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := tt.phase.RequiresHumanApproval(); got != tt.requiresHuman {
				t.Errorf("RequiresHumanApproval() = %v, want %v", got, tt.requiresHuman)
			}
		})
	}
}

func TestStoreCreateAndGetSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := NewSession("test-1", "Test Session", "A test session", "/tmp/test")

	// Create session
	err = store.CreateSession(session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Get session
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("expected ID '%s', got '%s'", session.ID, retrieved.ID)
	}
	if retrieved.Name != session.Name {
		t.Errorf("expected name '%s', got '%s'", session.Name, retrieved.Name)
	}
}

func TestStoreUpdateSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := NewSession("test-1", "Test Session", "A test session", "/tmp/test")
	err = store.CreateSession(session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Update session
	session.Name = "Updated Session"
	err = store.UpdateSession(session)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Verify update
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if retrieved.Name != "Updated Session" {
		t.Errorf("expected name 'Updated Session', got '%s'", retrieved.Name)
	}
}

func TestStoreDeleteSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := NewSession("test-1", "Test Session", "A test session", "/tmp/test")
	err = store.CreateSession(session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Delete session
	err = store.DeleteSession(session.ID)
	if err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	// Verify deletion
	_, err = store.GetSession(session.ID)
	if err == nil {
		t.Error("expected error after deletion, got nil")
	}
}

func TestStoreListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		session := NewSession(string(rune('a'+i)), "Session "+string(rune('a'+i)), "Desc", "/tmp")
		err := store.CreateSession(session)
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestStoreSaveAndLoadPlan(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session := NewSession("test-1", "Test Session", "A test session", "/tmp/test")
	err = store.CreateSession(session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	plan := NewPlan("plan-1", "test-1", "Test Plan")
	plan.AddAtomicUnit(AtomicUnit{ID: "au-001", Name: "Test Function", Type: "function"})

	err = store.SavePlan(plan)
	if err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	loadedPlan, err := store.LoadPlan("test-1")
	if err != nil {
		t.Fatalf("failed to load plan: %v", err)
	}

	if loadedPlan.ID != "plan-1" {
		t.Errorf("expected plan ID 'plan-1', got '%s'", loadedPlan.ID)
	}
	if len(loadedPlan.AtomicUnits) != 1 {
		t.Errorf("expected 1 atomic unit, got %d", len(loadedPlan.AtomicUnits))
	}
}

func TestStoreNonExistentSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	_, err = store.GetSession("non-existent")
	if err == nil {
		t.Error("expected error for non-existent session, got nil")
	}
}

// TestPhaseFlowCompleteness verifies all defined phases have valid transitions
func TestPhaseFlowCompleteness(t *testing.T) {
	allPhases := []Phase{
		PhasePlanning,
		PhasePlanningReview,
		PhaseCoding,
		PhaseTesting,
		PhaseReview,
		PhaseHumanReview,
		PhaseComplete,
		PhaseCancelled,
	}

	for _, phase := range allPhases {
		_, ok := PhaseFlow[phase]
		if !ok {
			t.Errorf("phase %s not defined in PhaseFlow", phase)
		}
	}
}

// BenchmarkSessionCreation benchmarks creating a new session
func BenchmarkSessionCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewSession("test", "Test", "Test", "/tmp")
	}
}

// BenchmarkAddEvent benchmarks adding an event to a session
func BenchmarkAddEvent(b *testing.B) {
	session := NewSession("test", "Test", "Test", "/tmp")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.AddEvent(NewEvent("", "test", PhasePlanning, "test"))
	}
}

// TestMain adds cleanup for test files
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Cleanup
	os.Exit(code)
}
