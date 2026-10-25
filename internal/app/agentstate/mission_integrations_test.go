package agentstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

func missionIntegrationStoreFixture(t *testing.T) (*Store, MissionExecution, MissionIntegrationInput) {
	t.Helper()
	store, record := preparedWaveFixture(t, false)
	wave, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(record), "owner", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordMissionChild(t.Context(), missionExecutionIdentity(wave), wave.Wave.ID, "create", waveChildProgress("create", "awaiting_review"), allowMemoryWrite); err != nil {
		t.Fatal(err)
	}
	identity := missionExecutionIdentity(wave)
	identity.Revision = 0
	record, err = store.FinishMissionWave(t.Context(), identity, wave.Wave.ID, false, "", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	child := record.Children[0]
	input := MissionIntegrationInput{ChildID: child.ID, Source: MissionBase{Head: strings.Repeat("b", 40), Branch: child.Branch}, SessionID: child.SessionID, WorkflowID: child.WorkflowID, ProposalHash: child.ProposalHash, ProposalRevision: child.ProposalRevision, Evidence: MissionProposalEvidence{Key: ArtifactKey{Session: Key{App: "mini-orca-missions", User: "local", ID: record.ID}, Name: child.ID + "-proposal.json"}, Version: 1, Hash: strings.Repeat("c", 64)}}
	return store, record, input
}

func missionCheckpointFixture(record MissionExecution) *MissionCheckpoint {
	checkpoint := record.Current
	checkpoint.Sources = slices.Clone(checkpoint.Sources)
	checkpoint.Instructions = maps.Clone(checkpoint.Instructions)
	checkpoint.Base.Head, checkpoint.ProjectRevision = strings.Repeat("b", 40), "integrated-revision"
	checkpoint.Sources[0].Exists, checkpoint.Sources[0].Hash = true, "sha256:"+strings.Repeat("b", 64)
	return &checkpoint
}

func TestMissionIntegrationStoreSerializesIntentAndReleasesDependencyOnlyAfterReceipt(t *testing.T) {
	store, ready, input := missionIntegrationStoreFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			_, err := store.BeginMissionIntegration(t.Context(), missionExecutionIdentity(ready), "owner", input, allowMemoryWrite)
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
		t.Fatal("integration intent was not exclusive", wins)
	}
	pending, err := store.MissionExecution(t.Context(), ready.Mission.ProjectID, ready.Mission.ID)
	if err != nil || len(pending.ReadyChildren()) != 0 || pending.State != "integrating" {
		t.Fatal(pending, err)
	}
	if _, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(pending), "other", allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("integration released dependency early", err)
	}
	merged, err := store.FinishMissionIntegration(t.Context(), missionExecutionIdentity(pending), pending.PendingIntegration().ID, MissionIntegrationOutcome{State: "merged", After: missionCheckpointFixture(pending)}, allowMemoryWrite)
	if err != nil || merged.State != "ready" || !reflect.DeepEqual(merged.ReadyChildren(), []string{"consume"}) || !reflect.DeepEqual(merged.Mission, ready.Mission) || merged.Base != ready.Base {
		t.Fatal(merged, err)
	}
	if owned, err := store.MissionWorktree(t.Context(), merged.Mission.ProjectID, merged.Children[0].Directory, ""); err != nil || owned.ID != merged.ID {
		t.Fatal("lost worktree ownership", owned, err)
	}
	if owned, err := store.MissionWorktree(t.Context(), merged.Mission.ProjectID, "moved", merged.Children[0].Branch); err != nil || owned.ID != merged.ID {
		t.Fatal("moving a checkout lost its branch ownership", owned, err)
	}
	merged, err = store.RecordMissionCleanup(t.Context(), missionExecutionIdentity(merged), merged.Integrations[0].ID, "removed", "", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(merged), "next", allowMemoryWrite)
	if err != nil || next.Children[1].BaseHead != merged.Current.Base.Head || next.Children[1].Limits != ready.Children[1].Limits {
		t.Fatal(next, err)
	}
	if state := integratedMissionState(MissionExecution{
		Children: []MissionExecutionChild{
			{ID: "first", State: "awaiting_review"},
			{ID: "second", State: "pending"},
		},
	}); state != "awaiting_review" {
		t.Fatalf("expected awaiting_review when sibling is pending, got %q", state)
	}
}

func TestMissionIntegrationStoreFailureAndCancellationPreserveEvidence(t *testing.T) {
	for _, phase := range []string{"begin", "finish", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			store, record, input := missionIntegrationStoreFixture(t)
			var err error
			if phase != "begin" {
				record, err = store.BeginMissionIntegration(t.Context(), missionExecutionIdentity(record), "owner", input, allowMemoryWrite)
				if err != nil {
					t.Fatal(err)
				}
			}
			id := ""
			if record.PendingIntegration() != nil {
				id = record.PendingIntegration().ID
			}
			if phase == "cleanup" {
				record, err = store.FinishMissionIntegration(t.Context(), missionExecutionIdentity(record), id, MissionIntegrationOutcome{State: "merged", After: missionCheckpointFixture(record)}, allowMemoryWrite)
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
				_, err = store.BeginMissionIntegration(t.Context(), missionExecutionIdentity(record), "owner", input, allowMemoryWrite)
			case "finish":
				_, err = store.FinishMissionIntegration(t.Context(), missionExecutionIdentity(record), id, MissionIntegrationOutcome{State: "merged", After: missionCheckpointFixture(record)}, allowMemoryWrite)
			case "cleanup":
				_, err = store.RecordMissionCleanup(t.Context(), missionExecutionIdentity(record), id, "removed", "", allowMemoryWrite)
			}
			if !errors.Is(err, ErrPersistence) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("failed write changed retained evidence", err)
			}
		})
	}
	store, record, input := missionIntegrationStoreFixture(t)
	pending, err := store.BeginMissionIntegration(t.Context(), missionExecutionIdentity(record), "owner", input, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := store.CancelMissionExecution(t.Context(), missionExecutionIdentity(pending), allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	identity := missionExecutionIdentity(pending)
	identity.Revision = 0
	merged, err := store.FinishMissionIntegration(t.Context(), identity, pending.PendingIntegration().ID, MissionIntegrationOutcome{State: "merged", After: missionCheckpointFixture(pending)}, allowMemoryWrite)
	if err != nil || merged.State != "canceled" || merged.Revision != canceled.Revision+1 || len(merged.ReadyChildren()) != 0 || merged.Children[0].State != "integrated" {
		t.Fatal(merged, err)
	}
}

func TestMissionIntegrationStoreRejectsCorruptionAndUnapprovedCheckpoint(t *testing.T) {
	store, ready, input := missionIntegrationStoreFixture(t)
	for name, change := range map[string]func(*MissionIntegrationInput){
		"order":    func(v *MissionIntegrationInput) { v.ChildID = "consume" },
		"proposal": func(v *MissionIntegrationInput) { v.ProposalRevision++ },
		"archive":  func(v *MissionIntegrationInput) { v.Evidence.Key.Session.ID = "foreign" },
		"base":     func(v *MissionIntegrationInput) { v.Source.Head = ready.Base.Head },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := input
			change(&candidate)
			if _, err := store.BeginMissionIntegration(t.Context(), missionExecutionIdentity(ready), "owner", candidate, allowMemoryWrite); err == nil {
				t.Fatal("invalid integration admitted")
			}
		})
	}
	pending, err := store.BeginMissionIntegration(t.Context(), missionExecutionIdentity(ready), "owner", input, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	outside := missionCheckpointFixture(pending)
	outside.Sources[1].Hash = "sha256:" + strings.Repeat("d", 64)
	if _, err := store.FinishMissionIntegration(t.Context(), missionExecutionIdentity(pending), pending.PendingIntegration().ID, MissionIntegrationOutcome{State: "merged", After: outside}, allowMemoryWrite); !errors.Is(err, ErrCorrupt) {
		t.Fatal("unapproved source advanced checkpoint", err)
	}
	for name, change := range map[string]func(*MissionExecution){
		"missing child":   func(v *MissionExecution) { v.Integrations[0].ChildID = "foreign" },
		"missing archive": func(v *MissionExecution) { v.Integrations[0].Evidence.Hash = "" },
		"silent advance":  func(v *MissionExecution) { v.Current = *missionCheckpointFixture(*v) },
		"false success":   func(v *MissionExecution) { v.Children[0].State = "integrated" },
		"unsafe owner":    func(v *MissionExecution) { v.Integrations[0].Owner = "../foreign" },
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(pending)
			if err != nil {
				t.Fatal(err)
			}
			var invalid MissionExecution
			if err := json.Unmarshal(data, &invalid); err != nil {
				t.Fatal(err)
			}
			change(&invalid)
			if !errors.Is(invalid.validate(), ErrCorrupt) {
				t.Fatal("corrupt receipt accepted")
			}
		})
	}
}

func TestMissionIntegrationMigratesVersionTwoActiveAndRetainedChildrenWithoutWriting(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "retained", true: "active"}[active], func(t *testing.T) {
			store, record, _ := missionIntegrationStoreFixture(t)
			if active {
				store, record = preparedWaveFixture(t, false)
				var err error
				record, err = store.BeginMissionWave(t.Context(), missionExecutionIdentity(record), "owner", allowMemoryWrite)
				if err != nil {
					t.Fatal(err)
				}
			}
			path, _ := store.path("mission-executions.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var catalog map[string]json.RawMessage
			if err := json.Unmarshal(data, &catalog); err != nil {
				t.Fatal(err)
			}
			var records map[string]map[string]json.RawMessage
			if err := json.Unmarshal(catalog["records"], &records); err != nil {
				t.Fatal(err)
			}
			for _, entry := range records {
				delete(entry, "current")
				delete(entry, "integrations")
			}
			catalog["records"], err = json.Marshal(records)
			if err != nil {
				t.Fatal(err)
			}
			catalog["version"] = json.RawMessage(`2`)
			legacy, err := json.Marshal(catalog)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, legacy, 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := store.MissionExecution(t.Context(), record.Mission.ProjectID, record.Mission.ID)
			if err != nil || !reflect.DeepEqual(loaded, record) {
				t.Fatal("migration lost original allocation or child evidence", loaded, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, legacy) {
				t.Fatal("passive migration rewrote evidence", err)
			}
			if _, err := store.CancelMissionExecution(t.Context(), missionExecutionIdentity(loaded), allowMemoryWrite); err != nil {
				t.Fatal(err)
			}
			after, err = os.ReadFile(path)
			if err != nil || !bytes.Contains(after, []byte(`"version":4`)) {
				t.Fatal("explicit mutation did not persist upgraded schema", err)
			}
		})
	}
}
