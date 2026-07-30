package orchestrator

import (
	"testing"
	"time"
)

func TestHumanGate_Approve(t *testing.T) {
	gate := NewHumanGate("s1", PhasePlanningReview)

	go func() {
		time.Sleep(10 * time.Millisecond)
		gate.Respond("approve", "looks good")
	}()

	res, err := gate.RequestApproval("some output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gate.IsApproved() != true {
		t.Error("expected gate to be approved")
	}
	if gate.Feedback() != "looks good" {
		t.Errorf("expected feedback 'looks good', got %q", gate.Feedback())
	}
	if res != nil {
		t.Error("expected nil response for approve")
	}
}

func TestHumanGate_Reject(t *testing.T) {
	gate := NewHumanGate("s1", PhasePlanningReview)

	go func() {
		time.Sleep(10 * time.Millisecond)
		gate.Respond("reject", "fix this")
	}()

	res, err := gate.RequestApproval("some output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gate.IsApproved() != false {
		t.Error("expected gate NOT to be approved")
	}
	if res.Action != "reject" {
		t.Errorf("expected action 'reject', got %q", res.Action)
	}
}

func TestHumanGate_InvalidAction(t *testing.T) {
	gate := NewHumanGate("s1", PhasePlanningReview)
	err := gate.Respond("invalid", "")
	if err == nil {
		t.Error("expected error for invalid action")
	}
}
