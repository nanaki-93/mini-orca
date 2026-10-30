package app

import (
	"fmt"
	"os"
	"testing"
)

// Fixture consent must never read or modify the developer's saved decisions.
func TestMain(m *testing.M) {
	if os.Getenv(fakePiEnv) == "1" {
		os.Exit(fakePi())
	}
	dir, err := os.MkdirTemp("", "mini-orca-trust-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("MINI_ORCA_TRUST_DIR", dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}
