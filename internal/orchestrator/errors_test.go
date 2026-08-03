package orchestrator

import (
	"errors"
	"testing"
)

func TestOrchestratorError(t *testing.T) {
	cause := errors.New("cause")
	err := &OrchestratorError{
		Phase:   "coding",
		Message: "failed",
		Cause:   cause,
	}

	expected := "orchestrator error in phase coding: failed"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}

	if !errors.Is(err, cause) {
		t.Error("expected cause to be unwrappable")
	}
}
