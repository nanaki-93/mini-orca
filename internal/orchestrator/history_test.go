package orchestrator

import (
	"testing"
	"time"
)

func TestHistoryTracker(t *testing.T) {
	tracker := NewHistoryTracker("sess-1")

	t.Run("LogPhaseTransition", func(t *testing.T) {
		tracker.LogPhaseTransition(PhaseCoding, PhaseTesting)
		history := tracker.GetHistory()
		if len(history) != 1 {
			t.Errorf("expected 1 event, got %d", len(history))
		}
		if history[0].EventType != "phase_transition" {
			t.Errorf("expected phase_transition, got %s", history[0].EventType)
		}
	})

	t.Run("LogLLMCall", func(t *testing.T) {
		tracker.LogLLMCall("coder", "input", "output", 100*time.Millisecond, nil)
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
		err := tracker.PersistToStore()
		if err != nil {
			t.Fatalf("PersistToStore failed: %v", err)
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
