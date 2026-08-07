package state

import (
	"sync"
	"testing"
)

func TestNewStore(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.GetCurrentPhase() != "idle" {
		t.Errorf("expected default phase 'idle', got %q", s.GetCurrentPhase())
	}
	if s.GetStatus() != "pending" {
		t.Errorf("expected default status 'pending', got %q", s.GetStatus())
	}
	if s.GetError() != "" {
		t.Errorf("expected empty error, got %q", s.GetError())
	}
}

func TestInitStateStore(t *testing.T) {
	s := InitStateStore()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestUpdatePhaseStatus(t *testing.T) {
	s := NewStore()
	s.UpdatePhaseStatus("coding", "in_progress")

	if s.GetCurrentPhase() != "coding" {
		t.Errorf("expected phase 'coding', got %q", s.GetCurrentPhase())
	}
	if s.GetStatus() != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", s.GetStatus())
	}
}

func TestSetError(t *testing.T) {
	s := NewStore()
	s.SetError("something went wrong")

	if s.GetError() != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got %q", s.GetError())
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup

	// Writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			phase := "phase"
			if n%2 == 0 {
				phase = "coding"
			} else {
				phase = "testing"
			}
			s.UpdatePhaseStatus(phase, "status")
			s.SetError("error")
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.GetCurrentPhase()
			_ = s.GetStatus()
			_ = s.GetError()
		}()
	}

	wg.Wait()
	// If we reach here without deadlock or race, the test passes
}
