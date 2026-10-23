package app

import (
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestWorkflowModelTimeoutHonorsConfigurationWithoutChangingOtherRequests(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, seconds := range []int{0, 900, 1800} {
		cfg := config.Default()
		cfg.Timeouts.WorkflowModelSeconds = seconds
		s, err := New(cfg, manager)
		if err != nil {
			t.Fatal(err)
		}
		want := time.Duration(max(900, seconds)) * time.Second
		for stage, profile := range []string{"bug", "function", "analyze"} {
			if got := s.workflowStageRuntime(stage, profile).effective.Timeout; got != want.String() {
				t.Fatalf("stage %d timeout = %s, want %s", stage, got, want)
			}
		}
		if s.runtimes.function.effective.Timeout != "5m0s" {
			t.Fatal("workflow changed standalone generation timeout")
		}
		s.runtimes.bug.effective.Timeout = "45m0s"
		if got := s.workflowStageRuntime(0, "bug").effective.Timeout; got != "45m0s" {
			t.Fatalf("longer selected timeout shortened to %s", got)
		}
		s.runtimes.function.effective.Timeout = "50m0s"
		if got := s.workflowStageRuntime(2, "analyze").effective.Timeout; got != "50m0s" {
			t.Fatalf("longer generation timeout shortened to %s", got)
		}
	}
}

func TestWorkflowTotalDeadlineAccommodatesStagesChecksAndRepairs(t *testing.T) {
	s := &Service{workflowModelTimeout: 15 * time.Minute, focusedCheckTimeout: time.Minute}
	models := ChangeWorkflowModels{Create: "bug", Test: "function", Review: "analyze"}
	full := s.changeWorkflowTimeout(models, 0, false)
	resumed := s.changeWorkflowTimeout(models, 1, false)
	if full <= 45*time.Minute || resumed <= 30*time.Minute || resumed >= full {
		t.Fatalf("remaining models/checks do not fit: full=%s, test retry=%s", full, resumed)
	}
	withRepairs := s.changeWorkflowTimeout(models, 1, true)
	if withRepairs < resumed+2*(45*time.Minute+5*s.focusedCheckTimeout) {
		t.Fatalf("repair attempts would be cut short: %s", withRepairs)
	}
	s.focusedCheckTimeout = 10 * time.Minute
	if got := s.changeWorkflowTimeout(models, 0, false); got < 45*time.Minute+10*s.focusedCheckTimeout {
		t.Fatalf("probe and candidate checks would be cut short: %s", got)
	}
}
