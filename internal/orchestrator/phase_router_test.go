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
		// Valid transitions
		{PhaseCoding, PhaseTesting, false},
		{PhaseTesting, PhaseCoding, false}, // retry
		{PhaseTesting, PhaseReview, false},
		{PhaseReview, PhaseCoding, false}, // retry
		{PhaseReview, PhaseHumanReview, false},
		{PhaseHumanReview, PhaseCoding, false}, // edit
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
	tests := []struct {
		current   Phase
		wantCount int
		wantFirst Phase
	}{
		{PhaseCoding, 1, PhaseTesting},
		{PhaseTesting, 2, PhaseCoding},
		{PhaseReview, 2, PhaseCoding},
		{PhaseHumanReview, 2, PhaseCoding},
	}

	for _, tt := range tests {
		t.Run(string(tt.current), func(t *testing.T) {
			transitions := GetValidTransitions(tt.current)
			if len(transitions) != tt.wantCount {
				t.Errorf("expected %d transitions from %s, got %d", tt.wantCount, tt.current, len(transitions))
			}
			if tt.wantCount > 0 && transitions[0] != tt.wantFirst {
				t.Errorf("expected first transition %s, got %s", tt.wantFirst, transitions[0])
			}
		})
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

	t.Run("ValidateTransition", func(t *testing.T) {
		err := router.ValidateTransition(PhaseCoding, PhaseTesting)
		if err != nil {
			t.Errorf("expected no error for valid transition, got %v", err)
		}

		err = router.ValidateTransition(PhaseCoding, PhaseReview)
		if err == nil {
			t.Error("expected error for invalid transition")
		}
	})

	t.Run("NextPhaseFromTesting", func(t *testing.T) {
		next, err := router.NextPhase(PhaseTesting)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if next != PhaseCoding {
			t.Errorf("expected %s, got %s", PhaseCoding, next)
		}
	})

	t.Run("NextPhaseFromReview", func(t *testing.T) {
		next, err := router.NextPhase(PhaseReview)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if next != PhaseCoding {
			t.Errorf("expected %s, got %s", PhaseCoding, next)
		}
	})

	t.Run("NextPhaseFromHumanReview", func(t *testing.T) {
		next, err := router.NextPhase(PhaseHumanReview)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if next != PhaseCoding {
			t.Errorf("expected %s, got %s", PhaseCoding, next)
		}
	})
}
