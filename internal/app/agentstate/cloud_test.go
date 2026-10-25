package agentstate

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func cloudFixture() CloudCopy {
	now := time.Now().UTC()
	return CloudCopy{ID: "mo-" + strings.Repeat("a", 32), ProjectID: "project", ProjectRevision: "revision", Target: config.CloudTarget{ID: "archive", Service: "gcs_artifact", Project: "123456789012", Region: "us-central1", Resource: "mini-orca-archives", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "CLOUD_KEY", RetentionDays: 30}, Configuration: strings.Repeat("b", 64), Kind: "artifact", SourceID: "source", SourceVersion: 1, SourceRevision: "revision", Hash: strings.Repeat("c", 64), Size: 24, State: "copying", CreatedAt: now, CleanupAt: now.Add(30 * 24 * time.Hour), LeaseUntil: now.Add(90 * time.Second)}
}

func TestCloudReceiptsCASIdentityAndRestart(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := s.CloudCopies(t.Context(), "project"); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	if _, err := os.Stat(root + "/.mini-orca"); !os.IsNotExist(err) {
		t.Fatal("passive read created metadata")
	}
	r := cloudFixture()
	saved, err := s.SaveCloudCopy(t.Context(), r, 0, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCloudCopy(t.Context(), r, 0, func() error { return nil }); !errors.Is(err, ErrCloudConflict) {
		t.Fatal("upload receipt replay", err)
	}
	other, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := other.CloudCopies(t.Context(), "project")
	if err != nil || len(rows) != 1 || rows[0].State != "copying" {
		t.Fatal("restart lost uncertainty", rows, err)
	}
	changed := saved
	changed.State = "completed"
	changed.LeaseUntil = time.Time{}
	changed.Target.Region = "europe-west1"
	if _, err := s.SaveCloudCopy(t.Context(), changed, saved.Version, func() error { return nil }); !errors.Is(err, ErrCloudConflict) {
		t.Fatal("retargeted receipt", err)
	}
	saved.State = "unknown"
	saved.LeaseUntil = time.Time{}
	saved, err = s.SaveCloudCopy(t.Context(), saved, saved.Version, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCloudCopy(t.Context(), saved, 1, func() error { return nil }); !errors.Is(err, ErrCloudConflict) {
		t.Fatal("stale version", err)
	}
	path, err := s.path("cloud.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private permissions", err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CloudCopies(t.Context(), "project"); !errors.Is(err, ErrCorrupt) {
		t.Fatal("corrupt receipts accepted", err)
	}
}

func TestCloudReceiptFailuresPreservePriorEvidence(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := cloudFixture()
	if _, err := s.SaveCloudCopy(t.Context(), r, 0, func() error { return errors.New("scope replaced") }); err == nil {
		t.Fatal("scope rejection ignored")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.SaveCloudCopy(ctx, r, 0, func() error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	r, err = s.SaveCloudCopy(t.Context(), r, 0, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	s.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk unavailable") }
	r.State = "completed"
	r.LeaseUntil = time.Time{}
	if _, err := s.SaveCloudCopy(t.Context(), r, r.Version, func() error { return nil }); !errors.Is(err, ErrPersistence) {
		t.Fatal("write failure swallowed", err)
	}
	rows, err := s.CloudCopies(t.Context(), "project")
	if err != nil || rows[0].State != "copying" {
		t.Fatal("uncertain original receipt lost", rows, err)
	}
}
