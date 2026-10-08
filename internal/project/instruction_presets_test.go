package project

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestInstructionPresetsMatchIndexedScope(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{
		"api/server.go":     "package api\nimport \"net/http\"\nvar _ = http.MethodGet\n",
		"api/store.go":      "package api\nimport \"database/sql\"\nvar _ *sql.DB\n",
		"api/README.md":     "# API\n",
		"api-extra/main.py": "def run():\n  pass\n",
		"web/app.tsx":       "export function App() { return <button>Run</button>; }",
		"tools/check.sh":    "#!/bin/sh\nexit 0\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	index, err := BuildIndex(root, "project", "revision")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		target string
		want   []string
	}{
		{"AGENTS.md", []string{"go", "javascript", "ui", "python", "shell", "docs", "http", "sql"}},
		{"api/AGENTS.md", []string{"go", "docs", "http", "sql"}},
		{"web/AGENTS.md", []string{"javascript", "ui"}},
		{"new/AGENTS.md", nil},
	} {
		t.Run(test.target, func(t *testing.T) {
			presets, err := SuggestInstructionPresets(root, test.target, index.Files)
			if err != nil {
				t.Fatal(err)
			}
			var matched []string
			sections := map[string]bool{}
			general := map[string]bool{}
			for _, preset := range presets {
				if preset.Category == "" {
					t.Fatalf("choice has no AGENTS.md section: %+v", preset)
				}
				sections[preset.Category] = true
				if len(preset.Evidence) == 0 {
					general[preset.ID] = true
					continue
				}
				// The original stack choices remain available alongside their granular rules.
				if slices.Contains([]string{"go", "javascript", "ui", "python", "shell", "docs", "http", "sql"}, preset.ID) {
					matched = append(matched, preset.ID)
				}
				if preset.Reason == "" || preset.Content == "" {
					t.Fatalf("unexplained recommendation: %+v", preset)
				}
				for _, path := range preset.Evidence {
					if test.target != "AGENTS.md" && !strings.HasPrefix(path, filepath.Dir(test.target)+"/") {
						t.Fatalf("out-of-scope evidence %q", path)
					}
				}
			}
			slices.Sort(matched)
			want := slices.Clone(test.want)
			slices.Sort(want)
			if !reflect.DeepEqual(matched, want) || !general["focused"] || !general["tests"] || len(sections) < 6 {
				t.Fatalf("matched=%v general=%v; want %v", matched, general, test.want)
			}
			if _, err := os.Stat(filepath.Join(root, test.target)); !os.IsNotExist(err) {
				t.Fatalf("suggestions wrote instructions: %v", err)
			}
		})
	}
}

func TestInstructionPresetsFilterCurrentPolicyAndBoundEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".mini-orca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mini-orca/context-policy.json"), []byte(`{"exclude":["private/**"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "linked.py")
	if err := os.WriteFile(outside, []byte("pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.py")); err != nil {
		t.Fatal(err)
	}
	files := []IndexFile{
		{Path: "z.go", Language: "Go"}, {Path: "b.go", Language: "Go"},
		{Path: "c.go", Language: "Go"}, {Path: "a.go", Language: "Go"},
		{Path: "private/server.go", Language: "Go", Imports: []string{"net/http"}},
		{Path: "secret.py", Language: "Python"}, {Path: "linked.py", Language: "Python"},
		{Path: "binary.tsx", Language: "TypeScript", Binary: true},
	}
	presets, err := SuggestInstructionPresets(root, "AGENTS.md", files)
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, preset := range presets {
		if len(preset.Evidence) == 0 {
			continue
		}
		matched++
		if !strings.HasPrefix(preset.ID, "go") || !reflect.DeepEqual(preset.Evidence, []string{"a.go", "b.go", "c.go"}) {
			t.Fatalf("unexpected eligible evidence: %+v", preset)
		}
	}
	if matched < 2 {
		t.Fatal("expected multiple focused Go choices")
	}
	if files[0].Path != "z.go" {
		t.Fatal("suggestions reordered the captured index")
	}
	if _, err := SuggestInstructionPresets(root, "../AGENTS.md", files); err == nil {
		t.Fatal("unsafe scope accepted")
	}
	if err := os.WriteFile(filepath.Join(root, ".mini-orca/context-policy.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SuggestInstructionPresets(root, "AGENTS.md", files); err == nil {
		t.Fatal("policy failure hidden by generic suggestions")
	}
}

func TestInstructionPresetsIdentifyProjectTypesFromManifestsAndImports(t *testing.T) {
	for _, test := range []struct {
		name         string
		files        []IndexFile
		want, absent []string
	}{
		{"node", []IndexFile{{Path: "package.json", Language: "JSON"}}, []string{"javascript-tooling", "javascript-tests"}, []string{"go", "typescript-types", "react-state", "ui"}},
		{"typescript-react", []IndexFile{{Path: "tsconfig.json", Language: "JSON"}, {Path: "app.tsx", Language: "TypeScript", Imports: []string{"import React from 'react';"}}}, []string{"typescript-types", "react-state", "react-effects", "ui-forms"}, []string{"go", "python"}},
		{"python", []IndexFile{{Path: "pyproject.toml", Language: "Text"}}, []string{"python-environment", "python-errors", "python-tests"}, []string{"javascript", "go"}},
		{"jvm", []IndexFile{{Path: "build.gradle.kts", Language: "Kotlin"}}, []string{"gradle-build", "jvm-contracts", "jvm-tests"}, []string{"go", "ui"}},
		{"rust", []IndexFile{{Path: "Cargo.toml", Language: "Text"}}, []string{"rust-build", "rust-tests"}, []string{"go", "python"}},
		{"cli", []IndexFile{{Path: "main.go", Language: "Go", Imports: []string{"flag", "github.com/spf13/cobra"}}}, []string{"cli-contracts", "cli-streams", "cli-tests"}, []string{"http", "sql"}},
		{"desktop", []IndexFile{{Path: "wails.json", Language: "JSON"}}, []string{"desktop-boundaries", "desktop-lifecycle"}, []string{"http", "sql"}},
		{"delivery", []IndexFile{{Path: "Dockerfile", Language: "Text"}, {Path: ".github/workflows/check.yml", Language: "YAML"}}, []string{"container-build", "container-secrets", "ci-checks", "ci-permissions"}, []string{"python", "go"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			presets, err := SuggestInstructionPresets(t.TempDir(), "AGENTS.md", test.files)
			if err != nil {
				t.Fatal(err)
			}
			byID := map[string]InstructionPreset{}
			for _, preset := range presets {
				if _, duplicate := byID[preset.ID]; duplicate {
					t.Fatalf("duplicate choice %s", preset.ID)
				}
				byID[preset.ID] = preset
				seen := map[string]bool{}
				for _, path := range preset.Evidence {
					if seen[path] {
						t.Fatalf("duplicate evidence in %+v", preset)
					}
					seen[path] = true
				}
			}
			for _, id := range test.want {
				if preset := byID[id]; preset.Reason == "" || len(preset.Evidence) == 0 || preset.Category == "" {
					t.Fatalf("missing explained choice %s: %+v", id, preset)
				}
			}
			for _, id := range test.absent {
				if _, exists := byID[id]; exists {
					t.Fatalf("unrelated project choice %s", id)
				}
			}
		})
	}
}
