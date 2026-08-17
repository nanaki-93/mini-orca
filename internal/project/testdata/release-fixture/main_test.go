package fixture

import "testing"

func TestRun(t *testing.T) {
	if got := Run("orca"); got != "hello orca" {
		t.Fatalf("Run() = %q", got)
	}
}
