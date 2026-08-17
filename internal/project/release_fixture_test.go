package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseFixtureRepresentsPolicyAndParserAcceptanceCases(t *testing.T) {
	root := filepath.Join("testdata", "release-fixture")
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".env", "private.pem", "config.yaml"} {
		if decision := policy.Decide(path); decision.Include {
			t.Fatalf("release fixture secret %s was included: %+v", path, decision)
		}
	}
	for _, path := range []string{"main.go", "main_test.go", "malformed.go.fixture"} {
		if decision := policy.Decide(path); !decision.Include {
			t.Fatalf("release fixture source %s was excluded: %+v", path, decision)
		}
	}
	mainSource, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	_, symbols, diagnostics := extractGoFacts("main.go", mainSource)
	if len(symbols) != 2 || len(diagnostics) != 0 {
		t.Fatalf("main fixture facts = symbols:%+v diagnostics:%+v", symbols, diagnostics)
	}
	brokenSource, err := os.ReadFile(filepath.Join(root, "malformed.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, diagnostics = extractGoFacts("malformed.go", brokenSource)
	if len(diagnostics) == 0 {
		t.Fatal("expected malformed fixture diagnostics")
	}
}
