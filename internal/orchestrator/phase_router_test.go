package orchestrator

import (
	"testing"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		from    Phase
		to      Phase
		wantErr bool
	}{
		{PhasePlanning, PhasePlanningReview, false},
		{PhasePlanningReview, PhaseCoding, false},
		{PhaseCoding, PhaseTesting, false},
		{PhaseTesting, PhaseReview, false},
		{PhaseReview, PhaseHumanReview, false},
		{PhaseHumanReview, "completed", false},
		// Invalid transitions
		{PhasePlanning, PhaseCoding, true},
		{PhaseCoding, PhasePlanning, true},
		{PhaseReview, PhasePlanning, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetValidTransitions(t *testing.T) {
	transitions := GetValidTransitions(PhasePlanning)
	if len(transitions) != 1 || transitions[0] != PhasePlanningReview {
		t.Errorf("expected [PhasePlanningReview], got %v", transitions)
	}

	transitions = GetValidTransitions(PhaseTesting)
	if len(transitions) != 2 {
		t.Errorf("expected 2 transitions from Testing, got %d", len(transitions))
	}
}

func TestPhaseRouter(t *testing.T) {
	router := NewPhaseRouter(nil, nil)

	t.Run("NextPhase", func(t *testing.T) {
		next, err := router.NextPhase(PhasePlanning)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if next != PhasePlanningReview {
			t.Errorf("expected %s, got %s", PhasePlanningReview, next)
		}
	})

	t.Run("GetTransitionsForPhase", func(t *testing.T) {
		ts := router.GetTransitionsForPhase(PhasePlanning)
		if len(ts) != 1 || ts[0].To != PhasePlanningReview {
			t.Errorf("expected transition to PhasePlanningReview, got %v", ts)
		}
	})
}
