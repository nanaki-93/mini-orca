package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryJournalErrorsNameTheFileAndRemedy(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	path := filepath.Join(root, ".mini-orca", "changes", "last-mutation.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	for content, want := range map[string]string{
		"broken":        "is unreadable",
		`{"version":7}`: "written by a newer Mini-Orca",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := service.ChangeRecovery()
		if err == nil || !strings.Contains(err.Error(), ".mini-orca/changes/last-mutation.json") || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: %v", content, err)
		}
		if kept, _ := os.ReadFile(path); string(kept) != content {
			t.Fatal("recovery journal was moved or changed")
		}
	}
}

func TestNewerTaskFilesAreSkippedNotSetAside(t *testing.T) {
	server := changeProvider(t, func() string { return workflowImplementation })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	session := prepareChangeFixture(t, service, "new.go")
	path := filepath.Join(root, ".mini-orca", "changes", session.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	newer := strings.Replace(string(data), `"schema_version": 1`, `"schema_version": 2`, 1)
	if newer == string(data) {
		t.Fatal("fixture schema version not found")
	}
	if err := os.WriteFile(path, []byte(newer), 0o600); err != nil {
		t.Fatal(err)
	}
	history, err := service.ChangeHistory(context.Background())
	if err != nil || len(history) != 0 {
		t.Fatalf("newer task listed or failed the list: %+v %v", history, err)
	}
	report, err := service.Quarantine(context.Background())
	if err != nil || len(report.Files) != 0 || len(report.Skipped) != 1 || !strings.Contains(report.Skipped[0].Remedy, "Upgrade Mini-Orca") {
		t.Fatalf("report: %+v %v", report, err)
	}
	if kept, _ := os.ReadFile(path); string(kept) != newer {
		t.Fatal("newer task file was moved")
	}
}
