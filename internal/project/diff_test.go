package project

import (
	"math/rand"
	"strings"
	"testing"
)

func TestUnifiedDiffKeepsUnchangedCodeBetweenSeparatedEdits(t *testing.T) {
	middle := strings.Repeat("unchanged code\n", 1500)
	before := "header\nold first\n" + middle + "old last\nfooter\n"
	after := "header\nnew first\n" + middle + "new last\nfooter\n"
	diff := BuildUnifiedDiff("file.go", before, after)
	added, removed := 0, 0
	for _, line := range diff.Lines {
		if line.Text == "unchanged code" && line.Kind != "context" {
			t.Fatal("unchanged code displayed as an edit")
		}
		switch line.Kind {
		case "added":
			added++
		case "removed":
			removed++
		}
	}
	if added != 2 || removed != 2 {
		t.Fatalf("diff inflated two edits: +%d -%d", added, removed)
	}
	assertDiffSources(t, diff, before, after)
}

func TestUnifiedDiffReconstructsBothSourcesAndLineNumbers(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	source := func() string {
		lines := []string{}
		for n := rng.Intn(20); n > 0; n-- {
			lines = append(lines, []string{"", "repeated", "one", "two", "\tthree", "  three"}[rng.Intn(6)])
		}
		return strings.Join(lines, "\n")
	}
	for i := 0; i < 300; i++ {
		before, after := source(), source()
		assertDiffSources(t, BuildUnifiedDiff("file", before, after), before, after)
	}
	for _, pair := range [][2]string{{"", "new\n"}, {"old\n", ""}, {"same\n", "same\n"}, {"end", "end\n"}} {
		assertDiffSources(t, BuildUnifiedDiff("file", pair[0], pair[1]), pair[0], pair[1])
	}
}

func assertDiffSources(t *testing.T, diff UnifiedDiff, before, after string) {
	t.Helper()
	old, next := []string{}, []string{}
	for _, line := range diff.Lines {
		if line.Kind != "added" {
			old = append(old, line.Text)
			if line.OldLine != len(old) {
				t.Fatalf("incorrect original line: %+v", line)
			}
		}
		if line.Kind != "removed" {
			next = append(next, line.Text)
			if line.NewLine != len(next) {
				t.Fatalf("incorrect proposed line: %+v", line)
			}
		}
	}
	if strings.Join(old, "\n") != before || strings.Join(next, "\n") != after {
		t.Fatalf("diff does not describe the source bytes: %#v", diff)
	}
}
