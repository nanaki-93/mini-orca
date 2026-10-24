package agentstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func missionRevalidationStoreFixture(t *testing.T) (*Store, MissionExecution, MissionBase, MissionProposalEvidence) {
	t.Helper()
	store, record := preparedWaveFixture(t, true)
	wave, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(record), "owner", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range wave.Wave.Children {
		if err := store.RecordMissionChild(t.Context(), missionExecutionIdentity(wave), wave.Wave.ID, id, waveChildProgress(id, "awaiting_review"), allowMemoryWrite); err != nil {
			t.Fatal(err)
		}
	}
	identity := missionExecutionIdentity(wave)
	identity.Revision = 0
	record, err = store.FinishMissionWave(t.Context(), identity, wave.Wave.ID, false, "", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	child := record.Children[0]
	evidence := MissionProposalEvidence{Key: ArtifactKey{Session: Key{App: "mini-orca-missions", User: "local", ID: record.ID}, Name: child.ID + "-proposal.json"}, Version: 1, Hash: strings.Repeat("c", 64)}
	input := MissionIntegrationInput{ChildID: child.ID, Source: MissionBase{Head: strings.Repeat("b", 40), Branch: child.Branch}, SessionID: child.SessionID, WorkflowID: child.WorkflowID, ProposalHash: child.ProposalHash, ProposalRevision: child.ProposalRevision, Evidence: evidence}
	record, err = store.BeginMissionIntegration(t.Context(), missionExecutionIdentity(record), "owner", input, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	record, err = store.FinishMissionIntegration(t.Context(), missionExecutionIdentity(record), record.PendingIntegration().ID, MissionIntegrationOutcome{State: "merged", After: missionCheckpointFixture(record)}, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Key.Name = record.Children[1].ID + "-proposal.json"
	return store, record, MissionBase{Head: record.Children[1].BaseHead, Branch: record.Children[1].Branch}, evidence
}

func TestMissionRevalidationStoreReservesOnceAndRetainsEveryCheckout(t *testing.T) {
	store, ready, source, evidence := missionRevalidationStoreFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 6)
	for range 6 {
		wg.Go(func() {
			_, err := store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(ready), "owner", "consume", source, evidence, allowMemoryWrite)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrMissionConflict) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatal("reservation was not exclusive", wins)
	}
	pending, err := store.MissionExecution(t.Context(), ready.Mission.ProjectID, ready.Mission.ID)
	if err != nil || pending.PendingRevalidation() == nil || pending.State != "revalidating" || len(pending.ReadyChildren()) != 0 {
		t.Fatal(pending, err)
	}
	if _, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(pending), "other", allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("pending revalidation admitted a wave", err)
	}
	entry := pending.PendingRevalidation()
	progress := MissionChildProgress{ProjectID: "fresh-project", ProjectRevision: "fresh-revision", SessionID: "fresh-session", ProposalHash: "fresh-proposal", ProposalRevision: 1}
	identity := missionExecutionIdentity(pending)
	identity.Revision = 0
	if err := store.RecordMissionRevalidationCandidate(t.Context(), identity, entry.ID, progress, allowMemoryWrite); err != nil {
		t.Fatal(err)
	}
	finished, err := store.FinishMissionRevalidation(t.Context(), identity, entry.ID, "awaiting_review", "", false, allowMemoryWrite)
	if err != nil || finished.Children[1].SessionID != progress.SessionID || finished.Children[1].WorkflowID != "" || finished.Children[1].Generation != 1 || finished.Children[1].BaseHead != ready.Current.Base.Head || !reflect.DeepEqual(finished.Mission, ready.Mission) || finished.ElapsedMillis < ready.ElapsedMillis {
		t.Fatal("candidate did not preserve immutable allocation and lineage", finished, err)
	}
	for _, child := range []MissionExecutionChild{entry.Previous, entry.Candidate} {
		if _, err := store.MissionWorktree(t.Context(), ready.Mission.ProjectID, "", child.Branch); err != nil {
			t.Fatal("lost historical checkout ownership", err)
		}
	}
	if _, err := store.FinishMissionRevalidation(t.Context(), identity, entry.ID, "awaiting_review", "", false, allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("replayed completion", err)
	}
	if _, err := store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(finished), "owner", "consume", source, evidence, allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("revalidated an already current candidate", err)
	}
}

func TestMissionRevalidationStoreBoundsFailuresAndUnknownOwnerCharge(t *testing.T) {
	store, record, source, evidence := missionRevalidationStoreFixture(t)
	original := record.Children[1]
	for generation := 1; generation <= MaxMissionRevalidations; generation++ {
		var err error
		record, err = store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(record), "owner", original.ID, source, evidence, allowMemoryWrite)
		if err != nil {
			t.Fatal(err)
		}
		entry := record.PendingRevalidation()
		if entry.Candidate.Generation != generation {
			t.Fatal("reused a generation", entry)
		}
		record, err = store.FinishMissionRevalidation(t.Context(), missionExecutionIdentity(record), entry.ID, "failed", "checks failed", false, allowMemoryWrite)
		if err != nil || !sameMissionCandidate(record.Children[1], original) {
			t.Fatal("failure replaced the previous proposal", record, err)
		}
	}
	if _, err := store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(record), "owner", original.ID, source, evidence, allowMemoryWrite); err == nil {
		t.Fatal("unbounded revalidation attempts")
	}
	store, record, source, evidence = missionRevalidationStoreFixture(t)
	pending, err := store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(record), "owner", "consume", source, evidence, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := store.CancelMissionExecution(t.Context(), missionExecutionIdentity(pending), allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	entry := canceled.PendingRevalidation()
	identity := missionExecutionIdentity(canceled)
	if _, err := store.FinishMissionRevalidation(t.Context(), identity, entry.ID, "awaiting_review", "", false, allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("cancellation published verification", err)
	}
	finished, err := store.FinishMissionRevalidation(t.Context(), identity, entry.ID, "interrupted", "owner stopped", true, allowMemoryWrite)
	if err != nil || finished.State != "canceled" || finished.ElapsedMillis != record.ElapsedMillis+entry.AllowanceMillis {
		t.Fatal("recovery minted elapsed time", finished, err)
	}
}

func TestMissionRevalidationStoreRejectsCorruptionAndPreservesFailedWrites(t *testing.T) {
	store, record, source, evidence := missionRevalidationStoreFixture(t)
	path, _ := store.path("mission-executions.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	denied := errors.New("authorization changed")
	if _, err := store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(record), "owner", "consume", source, evidence, func() error { return denied }); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed reservation altered the catalog", err)
	}
	pending, err := store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(record), "owner", "consume", source, evidence, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range []func(*MissionExecution){
		func(r *MissionExecution) { r.Revalidations[0].Candidate.Generation++ },
		func(r *MissionExecution) { r.Revalidations[0].Candidate.Limits.ModelCalls++ },
		func(r *MissionExecution) { r.Revalidations[0].Evidence.Hash = "bad" },
		func(r *MissionExecution) { r.Revalidations[0].Before.Base.Head = strings.Repeat("f", 40) },
		func(r *MissionExecution) { r.Revalidations[0].Candidate.WorkflowID = "stale-model-authority" },
		func(r *MissionExecution) { r.Revalidations[0].Previous.ID = "foreign-child" },
	} {
		data, _ := json.Marshal(pending)
		var invalid MissionExecution
		if err := json.Unmarshal(data, &invalid); err != nil {
			t.Fatal(err)
		}
		corrupt(&invalid)
		if !errors.Is(invalid.validate(), ErrCorrupt) {
			t.Fatal("accepted corrupt candidate lineage", invalid)
		}
	}
}

func TestMissionRevalidationStoreDiskFailuresPreserveReservationAndCandidate(t *testing.T) {
	for _, phase := range []string{"begin", "candidate", "finish"} {
		t.Run(phase, func(t *testing.T) {
			store, record, source, evidence := missionRevalidationStoreFixture(t)
			var err error
			if phase != "begin" {
				record, err = store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(record), "owner", "consume", source, evidence, allowMemoryWrite)
				if err != nil {
					t.Fatal(err)
				}
			}
			path, _ := store.path("mission-executions.json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk full") }
			switch phase {
			case "begin":
				_, err = store.BeginMissionRevalidation(t.Context(), missionExecutionIdentity(record), "owner", "consume", source, evidence, allowMemoryWrite)
			case "candidate":
				progress := MissionChildProgress{ProjectID: "new-project", ProjectRevision: "new-revision", SessionID: "new-session", ProposalHash: "new-hash", ProposalRevision: 1}
				err = store.RecordMissionRevalidationCandidate(t.Context(), missionExecutionIdentity(record), record.PendingRevalidation().ID, progress, allowMemoryWrite)
			case "finish":
				_, err = store.FinishMissionRevalidation(t.Context(), missionExecutionIdentity(record), record.PendingRevalidation().ID, "interrupted", "stopped", false, allowMemoryWrite)
			}
			if !errors.Is(err, ErrPersistence) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("failed write lost retained evidence", err)
			}
		})
	}
}

func TestMissionRevalidationCatalogMigratesV3WithoutDispatchOrAllocationReset(t *testing.T) {
	store, record, _, _ := missionRevalidationStoreFixture(t)
	path, _ := store.path("mission-executions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := bytes.Replace(data, []byte(`"version":4`), []byte(`"version":3`), 1)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.MissionExecution(t.Context(), record.Mission.ProjectID, record.Mission.ID)
	if err != nil || !reflect.DeepEqual(loaded, record) {
		t.Fatal("v3 upgrade changed retained integration evidence", loaded, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, legacy) {
		t.Fatal("passive read rewrote catalog", err)
	}
	if _, err := store.CancelMissionExecution(t.Context(), missionExecutionIdentity(loaded), allowMemoryWrite); err != nil {
		t.Fatal(err)
	}
	after, err = os.ReadFile(path)
	if err != nil || !bytes.Contains(after, []byte(`"version":4`)) {
		t.Fatal("explicit mutation did not upgrade catalog", err)
	}
}
