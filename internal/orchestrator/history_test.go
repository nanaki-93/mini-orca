package orchestrator

import (
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/internal/state"
)

func TestHistoryTracker(t *testing.T) {
	store := state.NewStore(nil)
	tracker := NewHistoryTracker("sess-1", store)

	t.Run("LogPhaseTransition", func(t *testing.T) {
		tracker.LogPhaseTransition(PhasePlanning, PhasePlanningReview)
		history := tracker.GetHistory()
		if len(history) != 1 {
			t.Errorf("expected 1 event, got %d", len(history))
		}
		if history[0].EventType != "phase_transition" {
			t.Errorf("expected phase_transition, got %s", history[0].EventType)
		}
	})

	t.Run("LogLLMCall", func(t *testing.T) {
		tracker.LogLLMCall("planner", "input", "output", 100*time.Millisecond, nil)
		history := tracker.GetHistory()
		if len(history) != 2 {
			t.Errorf("expected 2 events, got %d", len(history))
		}
	})

	t.Run("LogFileOperation", func(t *testing.T) {
		tracker.LogFileOperation("write", "test.go", nil)
		history := tracker.GetHistory()
		if len(history) != 3 {
			t.Errorf("expected 3 events, got %d", len(history))
		}
	})

	t.Run("PersistToStore", func(t *testing.T) {
		// Mock session in store for PersistToStore to work (it updates session history)
		store.SaveSession(&state.Session{ID: "sess-1"})

		err := tracker.PersistToStore()
		if err != nil {
			t.Fatalf("PersistToStore failed: %v", err)
		}

		sessionHistory, _ := store.GetSessionHistory("sess-1")
		if len(sessionHistory) != 3 {
			t.Errorf("expected 3 history entries in store, got %d", len(sessionHistory))
		}
	})
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello..." {
		t.Errorf("expected hello..., got %s", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("expected hello world, got %s", truncateString(s, 20))
	}
}

func TestHistoryTracker_MapEventStatus(t *testing.T) {
	h := &HistoryTracker{}

	tests := []struct {
		event HistoryEvent
		want  state.PhaseStatus
	}{
		{HistoryEvent{Error: ""}, state.PhaseStatusCompleted},
		{HistoryEvent{Error: "err"}, state.PhaseStatusFailed},
		{HistoryEvent{EventType: "phase_transition"}, state.PhaseStatusCompleted},
	}

	for _, tt := range tests {
		got := h.mapEventStatus(tt.event)
		if got != tt.want {
			t.Errorf("mapEventStatus(%v) = %v, want %v", tt.event, got, tt.want)
		}
	}
}
