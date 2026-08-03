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
		{PhaseCoding, PhaseTesting, false},
		{PhaseTesting, PhaseCoding, false},
		{PhaseTesting, PhaseReview, false},
		{PhaseReview, PhaseCoding, false},
		{PhaseReview, PhaseHumanReview, false},
		{PhaseHumanReview, PhaseCoding, false},
		{PhaseHumanReview, "completed", false},
		// Invalid transitions
		{PhaseCoding, PhaseReview, true},
		{PhaseCoding, PhaseHumanReview, true},
		{PhaseTesting, PhaseHumanReview, true},
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
	transitions := GetValidTransitions(PhaseCoding)
	if len(transitions) != 1 || transitions[0] != PhaseTesting {
		t.Errorf("expected [PhaseTesting], got %v", transitions)
	}

	transitions = GetValidTransitions(PhaseTesting)
	if len(transitions) != 2 {
		t.Errorf("expected 2 transitions from Testing, got %d", len(transitions))
	}
}

func TestPhaseRouter(t *testing.T) {
	router := NewPhaseRouter(nil, nil)

	t.Run("NextPhase", func(t *testing.T) {
		next, err := router.NextPhase(PhaseCoding)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if next != PhaseTesting {
			t.Errorf("expected %s, got %s", PhaseTesting, next)
		}
	})

	t.Run("GetTransitionsForPhase", func(t *testing.T) {
		ts := router.GetTransitionsForPhase(PhaseCoding)
		if len(ts) != 1 || ts[0].To != PhaseTesting {
			t.Errorf("expected transition to PhaseTesting, got %v", ts)
		}
	})
}
