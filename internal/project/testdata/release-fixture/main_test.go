package fixture

import "testing"

func TestRun(t *testing.T) {
	if got := Run("orca"); got != "hello orca" {
		t.Fatalf("Run() = %q", got)
	}
}

func TestWorker(t *testing.T) {
	if got := (Worker{Name: "orca"}).Name; got != "orca" {
		t.Fatalf("Worker.Name = %q", got)
	}
}
