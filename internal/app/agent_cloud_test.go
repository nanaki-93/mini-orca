package app

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

type fakeCloud struct {
	mu                              sync.Mutex
	data                            []byte
	copies, checks, deletes, closed int
	fail                            bool
	started                         chan struct{}
}

func (f *fakeCloud) Copy(ctx context.Context, r agentstate.CloudCopy, data []byte) error {
	f.mu.Lock()
	f.data = slices.Clone(data)
	f.copies++
	fail, started := f.fail, f.started
	f.mu.Unlock()
	if started != nil {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	if fail {
		return errors.New("PRIVATE_REMOTE_PAYLOAD token=secret")
	}
	return nil
}
func (f *fakeCloud) Inspect(ctx context.Context, r agentstate.CloudCopy) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	if f.fail {
		return false, errors.New("private provider failure")
	}
	return cloudHash(f.data) == r.Hash, nil
}
func (f *fakeCloud) Delete(context.Context, agentstate.CloudCopy) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes++
	if f.fail {
		return errors.New("private deletion failure")
	}
	f.data = nil
	return nil
}
func (f *fakeCloud) Close() error { f.mu.Lock(); defer f.mu.Unlock(); f.closed++; return nil }
func cloudTestTarget() config.CloudTarget {
	return config.CloudTarget{ID: "archive", Service: "gcs_artifact", Project: "123456789012", Region: "us-central1", Resource: "mini-orca-archives", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "CLOUD_KEY", RetentionDays: 30}
}

func cloudAppFixture(t *testing.T) (*Service, *agentstate.Store, CloudSelection, *fakeCloud) {
	t.Helper()
	s, store, q, write := libraryFixture(t)
	record, err := store.SaveArtifact(t.Context(), write)
	if err != nil {
		t.Fatal(err)
	}
	s.integrations.config.Cloud = []config.CloudTarget{cloudTestTarget()}
	fake := &fakeCloud{}
	s.openCloud = func(context.Context, config.CloudTarget) (agentruntime.CloudServices, error) { return fake, nil }
	return s, store, CloudSelection{ProjectID: q.ProjectID, ProjectRevision: q.ProjectRevision, TargetID: "archive", Kind: "artifact", SourceID: artifactLibraryID(*record)}, fake
}
func cloudCopyRequest(p *CloudPreview) CloudControl {
	return CloudControl{ProjectID: p.ProjectID, ProjectRevision: p.ProjectRevision, Action: "copy", ID: p.ID, Preview: p, Confirm: true, ConfirmSourceSharing: true}
}

func TestCloudCopiesPreviewConfirmationAndExactRecovery(t *testing.T) {
	s, store, selection, remote := cloudAppFixture(t)
	p, err := s.PreviewCloudCopy(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	if remote.copies+remote.checks != 0 {
		t.Fatal("preview connected")
	}
	body, _ := json.Marshal(p)
	if strings.Contains(string(body), "PRIVATE_") {
		t.Fatal("preview contains private content")
	}
	r := cloudCopyRequest(p)
	r.ConfirmSourceSharing = false
	if _, err := s.ControlCloudCopy(t.Context(), r); err == nil {
		t.Fatal("source sharing not required")
	}
	r.ConfirmSourceSharing = true
	r.Preview.Hash = strings.Repeat("0", 64)
	if _, err := s.ControlCloudCopy(t.Context(), r); !errors.Is(err, agentstate.ErrCloudConflict) {
		t.Fatal("changed preview accepted", err)
	}
	p, err = s.PreviewCloudCopy(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	r = cloudCopyRequest(p)
	copy, err := s.ControlCloudCopy(t.Context(), r)
	if err != nil || copy.State != "completed" || remote.copies != 1 || remote.closed != 1 {
		t.Fatal(copy, err)
	}
	if _, err := s.ControlCloudCopy(t.Context(), r); !errors.Is(err, agentstate.ErrCloudConflict) || remote.copies != 1 {
		t.Fatal("upload replayed", err)
	}
	for _, action := range []string{"check", "delete"} {
		copy, err = s.ControlCloudCopy(t.Context(), CloudControl{ProjectID: selection.ProjectID, ProjectRevision: selection.ProjectRevision, Action: action, ID: copy.ID, Version: copy.Version, Confirm: true})
		if err != nil {
			t.Fatal(err)
		}
	}
	if copy.State != "deleted" || remote.deletes != 1 {
		t.Fatal(copy)
	}
	if rows, err := store.ArtifactInventory(t.Context()); err != nil || len(rows) != 1 {
		t.Fatal("local original removed", err)
	}
}

func TestCloudUnknownOutcomesSurviveCancellationAndDeletionFailure(t *testing.T) {
	s, store, selection, remote := cloudAppFixture(t)
	p, err := s.PreviewCloudCopy(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	remote.fail = true
	r, err := s.ControlCloudCopy(t.Context(), cloudCopyRequest(p))
	if err != nil || r.State != "unknown" {
		t.Fatal(r, err)
	}
	body, _ := json.Marshal(r)
	if strings.Contains(string(body), "PRIVATE_") || strings.Contains(string(body), "secret") {
		t.Fatal("diagnostic leak")
	}
	remote.fail = false
	r, err = s.ControlCloudCopy(t.Context(), CloudControl{ProjectID: selection.ProjectID, ProjectRevision: selection.ProjectRevision, ID: r.ID, Version: r.Version, Action: "check", Confirm: true})
	if err != nil || r.State != "completed" || remote.copies != 1 {
		t.Fatal("lost receipt was replayed or not reconciled", r, err)
	}
	remote.fail = true
	r, err = s.ControlCloudCopy(t.Context(), CloudControl{ProjectID: selection.ProjectID, ProjectRevision: selection.ProjectRevision, ID: r.ID, Version: r.Version, Action: "delete", Confirm: true})
	if err != nil || r.State != "unknown" {
		t.Fatal("failed delete claimed success", r, err)
	}
	remote.fail = false
	remote.started = make(chan struct{})
	p, err = s.PreviewCloudCopy(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		r, err := s.ControlCloudCopy(ctx, cloudCopyRequest(p))
		if err == nil && r.State != "unknown" {
			err = errors.New("cancellation lost uncertainty")
		}
		done <- err
	}()
	<-remote.started
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	rows, err := store.CloudCopies(t.Context(), selection.ProjectID)
	if err != nil || len(rows) != 2 || rows[1].State != "unknown" {
		t.Fatal("canceled receipt missing", rows, err)
	}
}
