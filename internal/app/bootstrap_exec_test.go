package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestExecuteBootstrapToolchainRunsMiseTrustInstallAndLefthook(t *testing.T) {
	binDir := t.TempDir()
	logFile := filepath.Join(binDir, "bootstrap.log")
	fakeMise := filepath.Join(binDir, "mise")
	miseScript := "#!/bin/sh\necho \"mise $@ in $(pwd)\" >> \"" + logFile + "\"\necho \"mise $@ ok\"\n"
	if err := os.WriteFile(fakeMise, []byte(miseScript), 0755); err != nil {
		t.Fatal(err)
	}
	fakeLefthook := filepath.Join(binDir, "lefthook")
	lefthookScript := "#!/bin/sh\necho \"lefthook $@ in $(pwd)\" >> \"" + logFile + "\"\necho \"lefthook $@ ok\"\n"
	if err := os.WriteFile(fakeLefthook, []byte(lefthookScript), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mise.toml"), []byte("[tools]\ngo = \"1.22\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte("pre-commit:\n  parallel: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "bootstrap-fixture", Path: root, Type: "go"}); err != nil {
		t.Fatal(err)
	}
	service, err := New(scopedTestConfig("http://127.0.0.1:1"), manager)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.ExecuteBootstrapToolchain(context.Background(), analysis.ProjectRevision, false); err == nil {
		t.Fatal("expected ExecuteBootstrapToolchain to require explicit confirmation or trust")
	}

	res, err := service.ExecuteBootstrapToolchain(context.Background(), analysis.ProjectRevision, true)
	if err != nil {
		t.Fatalf("ExecuteBootstrapToolchain: %v", err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("expected 2 bootstrap steps, got %+v", res.Steps)
	}
	if res.Steps[0].Command != "mise trust && mise install" || res.Steps[0].State != "passed" {
		t.Fatalf("expected mise trust && mise install to pass, got %+v", res.Steps[0])
	}
	if res.Steps[1].Command != "lefthook install" || res.Steps[1].State != "passed" {
		t.Fatalf("expected lefthook install to pass, got %+v", res.Steps[1])
	}
	logged, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logged)
	if !strings.Contains(logText, "mise trust in ") || !strings.Contains(logText, "mise install in ") || !strings.Contains(logText, "lefthook install in ") {
		t.Fatalf("expected mise trust, mise install, and lefthook install in log, got:\n%s", logText)
	}
}
