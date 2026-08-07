package orchestrator

import (
	"testing"
	"time"
)

func TestHumanGate_Approve(t *testing.T) {
	gate := NewHumanGate("s1", PhaseHumanReview)

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

func TestHumanGate_Edit(t *testing.T) {
	gate := NewHumanGate("s1", PhaseHumanReview)

	go func() {
		time.Sleep(10 * time.Millisecond)
		gate.Respond("edit", "fix the typo")
	}()

	res, err := gate.RequestApproval("some output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gate.IsApproved() != false {
		t.Error("expected gate NOT to be approved")
	}
	if res.Action != "edit" {
		t.Errorf("expected action 'edit', got %q", res.Action)
	}
	if res.Feedback != "fix the typo" {
		t.Errorf("expected feedback 'fix the typo', got %q", res.Feedback)
	}
}

func TestHumanGate_Refuse(t *testing.T) {
	gate := NewHumanGate("s1", PhaseHumanReview)

	go func() {
		time.Sleep(10 * time.Millisecond)
		gate.Respond("refuse", "not what I asked for")
	}()

	res, err := gate.RequestApproval("some output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gate.IsApproved() != false {
		t.Error("expected gate NOT to be approved")
	}
	if res.Action != "refuse" {
		t.Errorf("expected action 'refuse', got %q", res.Action)
	}
	if res.Feedback != "not what I asked for" {
		t.Errorf("expected feedback 'not what I asked for', got %q", res.Feedback)
	}
}

func TestHumanGate_InvalidAction(t *testing.T) {
	gate := NewHumanGate("s1", PhaseHumanReview)
	err := gate.Respond("invalid", "")
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestHumanGate_AlreadyApproved(t *testing.T) {
	gate := NewHumanGate("s1", PhaseHumanReview)

	go func() {
		time.Sleep(10 * time.Millisecond)
		gate.Respond("approve", "ok")
	}()

	// First request should succeed
	_, err := gate.RequestApproval("output1")
	if err != nil {
		t.Fatalf("unexpected error on first request: %v", err)
	}

	// Second request should fail (already approved)
	_, err = gate.RequestApproval("output2")
	if err == nil {
		t.Error("expected error for second approval request")
	}
}

func TestHumanGate_SessionID(t *testing.T) {
	gate := NewHumanGate("test-session", PhaseHumanReview)
	if gate.SessionID() != "test-session" {
		t.Errorf("expected session ID 'test-session', got %q", gate.SessionID())
	}
}

func TestHumanGate_Phase(t *testing.T) {
	gate := NewHumanGate("s1", PhaseHumanReview)
	if gate.Phase() != PhaseHumanReview {
		t.Errorf("expected phase %s, got %s", PhaseHumanReview, gate.Phase())
	}
}
