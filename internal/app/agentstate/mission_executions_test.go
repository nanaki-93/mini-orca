package agentstate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func approvedMissionFixture(t *testing.T, store *Store) MissionSnapshot {
	t.Helper()
	saved := saveMissionFixture(t, store)
	for _, child := range append(saved.Mission.Plan.IntegrationOrder, "") {
		var err error
		saved, err = store.ReviewMission(t.Context(), saved.Mission.ProjectID, saved.Mission.ID, saved.CatalogRevision, saved.Mission.PlanHash, child, allowMemoryWrite)
		if err != nil {
			t.Fatal(err)
		}
	}
	return saved
}

func preparedWaveFixture(t *testing.T, parallel bool) (*Store, MissionExecution) {
	t.Helper()
	store := memoryStore(t)
	mission := missionFixture()
	if parallel {
		mission.Plan.Children[1].Dependencies, mission.Plan.Children[1].ReadPaths = []string{}, []string{}
		mission.Plan.Limits.ConcurrentCalls = 2
	}
	saved, err := store.SaveMission(t.Context(), mission, 0, "", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range append(saved.Mission.Plan.IntegrationOrder, "") {
		saved, err = store.ReviewMission(t.Context(), saved.Mission.ProjectID, saved.Mission.ID, saved.CatalogRevision, saved.Mission.PlanHash, child, allowMemoryWrite)
		if err != nil {
			t.Fatal(err)
		}
	}
	record, err := store.PrepareMissionExecution(t.Context(), missionExecutionPrepare(saved), allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	return store, record
}

func waveChildProgress(child string, state string) MissionChildProgress {
	return MissionChildProgress{State: state, ProjectID: "child-project-" + child, ProjectRevision: "revision", SessionID: "session-" + child, WorkflowID: "workflow-" + child, ProposalHash: "proposal-" + child, ProposalRevision: 2}
}

func TestMissionWaveReservesOnceAndJoinsConcurrentReceipts(t *testing.T) {
	store, prepared := preparedWaveFixture(t, true)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	start := make(chan struct{})
	for range 8 {
		wg.Go(func() {
			<-start
			_, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(prepared), "owner", allowMemoryWrite)
			results <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrMissionConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("duplicate wave allocation", success)
	}
	wave, err := store.MissionExecution(t.Context(), prepared.Mission.ProjectID, prepared.Mission.ID)
	if err != nil || len(wave.Wave.Children) != 2 || wave.State != "running" || len(wave.ReadyChildren()) != 0 {
		t.Fatal(wave, err)
	}
	for _, child := range wave.Children {
		if child.State != "reserved" || child.Directory != wave.ID+"-"+child.ID || child.Branch != "codex/"+child.Directory || child.BaseHead != prepared.Base.Head || child.Limits != prepared.Children[0].Limits {
			t.Fatal("missing durable effect ownership", child)
		}
		wg.Go(func() {
			if err := store.RecordMissionChild(t.Context(), missionExecutionIdentity(wave), wave.Wave.ID, child.ID, waveChildProgress(child.ID, "awaiting_review"), allowMemoryWrite); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	identity := missionExecutionIdentity(wave)
	identity.Revision = 0
	finished, err := store.FinishMissionWave(t.Context(), identity, wave.Wave.ID, false, "", allowMemoryWrite)
	if err != nil || finished.State != "awaiting_review" || finished.Wave != nil || finished.ElapsedMillis < 1 || len(finished.ReadyChildren()) != 0 {
		t.Fatal(finished, err)
	}
	if _, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(finished), "owner", allowMemoryWrite); err == nil {
		t.Fatal("completed allocation reused")
	}
	if err := store.RecordMissionChild(t.Context(), identity, wave.Wave.ID, "create", waveChildProgress("create", "failed"), allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("late receipt accepted", err)
	}
}

func TestMissionWaveDependenciesAndTimeCannotBeReset(t *testing.T) {
	store, prepared := preparedWaveFixture(t, false)
	if !reflect.DeepEqual(prepared.ReadyChildren(), []string{"create"}) {
		t.Fatal(prepared.ReadyChildren())
	}
	wave, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(prepared), "owner", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	identity := missionExecutionIdentity(wave)
	if _, err := store.FinishMissionWave(t.Context(), identity, wave.Wave.ID, true, "restart", allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("unexpired owner stolen", err)
	}
	if err := store.RecordMissionChild(t.Context(), identity, "foreign", "create", waveChildProgress("create", "running"), allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal(err)
	}
	if err := store.RecordMissionChild(t.Context(), identity, wave.Wave.ID, "consume", waveChildProgress("consume", "running"), allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("dependent dispatched", err)
	}
	err = store.mutateMissionExecutions(t.Context(), allowMemoryWrite, func(c *missionExecutions, now time.Time) error {
		r := c.Records[prepared.Mission.ID]
		r.Wave.StartedAt = now.Add(-time.Duration(r.Wave.AllowanceMillis+1) * time.Millisecond)
		c.Records[prepared.Mission.ID] = r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	finished, err := store.RecoverMissionWave(t.Context(), identity, wave.Wave.ID, true, nil, allowMemoryWrite)
	if err != nil || finished.ElapsedMillis != wave.Wave.AllowanceMillis || finished.Children[0].State != "interrupted" || finished.Children[1].State != "pending" || len(finished.ReadyChildren()) != 0 {
		t.Fatal(finished, err)
	}
	if _, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(finished), "new-owner", allowMemoryWrite); err == nil {
		t.Fatal("restart reset budget")
	}
}

func TestMissionWaveStopPreservesCompletedEvidenceAndRevokesNewWorkers(t *testing.T) {
	for _, action := range []string{"pause", "cancel"} {
		t.Run(action, func(t *testing.T) {
			store, prepared := preparedWaveFixture(t, true)
			wave, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(prepared), "owner", allowMemoryWrite)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.RecordMissionChild(t.Context(), missionExecutionIdentity(wave), wave.Wave.ID, "create", waveChildProgress("create", "rejected"), allowMemoryWrite); err != nil {
				t.Fatal(err)
			}
			current, err := store.MissionExecution(t.Context(), prepared.Mission.ProjectID, prepared.Mission.ID)
			if err != nil {
				t.Fatal(err)
			}
			stopped, err := store.stopMissionExecution(t.Context(), missionExecutionIdentity(current), action, allowMemoryWrite)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(stopped), "other", allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
				t.Fatal(err)
			}
			finished, err := store.FinishMissionWave(t.Context(), missionExecutionIdentity(stopped), wave.Wave.ID, false, "stopped", allowMemoryWrite)
			want := map[string]string{"pause": "paused", "cancel": "canceled"}[action]
			if err != nil || finished.State != want || finished.Children[0].State != "rejected" || finished.Children[1].State != "interrupted" || finished.ElapsedMillis < 1 {
				t.Fatal(finished, err)
			}
		})
	}
}

func TestMissionWaveWriteFailuresAndCorruptionPreserveReservations(t *testing.T) {
	for _, phase := range []string{"begin", "progress", "finish", "recover"} {
		t.Run(phase, func(t *testing.T) {
			store, record := preparedWaveFixture(t, false)
			if phase != "begin" {
				var err error
				record, err = store.BeginMissionWave(t.Context(), missionExecutionIdentity(record), "owner", allowMemoryWrite)
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
			identity := missionExecutionIdentity(record)
			switch phase {
			case "begin":
				_, err = store.BeginMissionWave(t.Context(), identity, "owner", allowMemoryWrite)
			case "progress":
				err = store.RecordMissionChild(t.Context(), identity, record.Wave.ID, "create", waveChildProgress("create", "running"), allowMemoryWrite)
			case "finish":
				_, err = store.FinishMissionWave(t.Context(), identity, record.Wave.ID, false, "", allowMemoryWrite)
			case "recover":
				_, err = store.RecoverMissionWave(t.Context(), identity, record.Wave.ID, false, map[string]MissionChildProgress{"create": waveChildProgress("create", "awaiting_review")}, allowMemoryWrite)
			}
			if !errors.Is(err, ErrPersistence) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("failure replaced evidence", err)
			}
		})
	}
	store, prepared := preparedWaveFixture(t, true)
	wave, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(prepared), "owner", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*MissionExecution){
		"owner":          func(r *MissionExecution) { r.Wave.Owner = "../escape" },
		"unowned writer": func(r *MissionExecution) { r.Children[1].WaveID = "other"; r.Wave.Children = []string{"create"} },
		"no wave":        func(r *MissionExecution) { r.Wave = nil; r.State = "blocked" },
		"directory":      func(r *MissionExecution) { r.Children[0].Directory = "../outside" },
		"elapsed":        func(r *MissionExecution) { r.ElapsedMillis = 1 },
		"repeated child": func(r *MissionExecution) { r.Wave.Children = []string{"create", "create"} },
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(wave)
			if err != nil {
				t.Fatal(err)
			}
			var invalid MissionExecution
			if err := json.Unmarshal(data, &invalid); err != nil {
				t.Fatal(err)
			}
			mutate(&invalid)
			if !errors.Is(invalid.validate(), ErrCorrupt) {
				t.Fatal("incoherent wave accepted")
			}
		})
	}
}

func TestMissionExecutionVersionOneUpgradesWithoutResetOrReadWrites(t *testing.T) {
	store, prepared := preparedWaveFixture(t, false)
	path, _ := store.path("mission-executions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := bytes.Replace(data, []byte(`"version":2`), []byte(`"version":1`), 1)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.MissionExecution(t.Context(), prepared.Mission.ProjectID, prepared.Mission.ID)
	if err != nil || !reflect.DeepEqual(loaded, prepared) {
		t.Fatal(loaded, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(legacy, after) {
		t.Fatal("read rewrote legacy receipt", err)
	}
	if _, err := store.BeginMissionWave(t.Context(), missionExecutionIdentity(loaded), "owner", allowMemoryWrite); err != nil {
		t.Fatal(err)
	}
	after, err = os.ReadFile(path)
	if err != nil || !bytes.Contains(after, []byte(`"version":2`)) {
		t.Fatal("wave did not persist upgraded receipt", err)
	}
}

func missionExecutionPrepare(saved MissionSnapshot) MissionExecutionPrepare {
	return MissionExecutionPrepare{ProjectID: saved.Mission.ProjectID, MissionID: saved.Mission.ID, CatalogRevision: saved.CatalogRevision, PlanHash: saved.Mission.PlanHash, Base: MissionBase{Head: strings.Repeat("a", 40), Branch: "main"}}
}

func missionExecutionIdentity(record MissionExecution) MissionExecutionIdentity {
	return MissionExecutionIdentity{ProjectID: record.Mission.ProjectID, MissionID: record.Mission.ID, ExecutionID: record.ID, Revision: record.Revision, PlanHash: record.Mission.PlanHash}
}

func TestMissionExecutionPreservesFrozenAllocationsAcrossRevisionCancellationAndRestart(t *testing.T) {
	store := memoryStore(t)
	saved := approvedMissionFixture(t, store)
	request := missionExecutionPrepare(saved)
	record, err := store.PrepareMissionExecution(t.Context(), request, allowMemoryWrite)
	if err != nil || record.State != "prepared" || record.Revision != 1 || !reflect.DeepEqual(record.Mission, saved.Mission) {
		t.Fatal(record, err)
	}
	for i, child := range record.Children {
		if child.ID != saved.Mission.Plan.Children[i].ID || child.State != "pending" || child.Limits != saved.Mission.Plan.Children[i].Limits {
			t.Fatal("execution changed the approved child allocation", child)
		}
	}
	path, err := store.path("mission-executions.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("execution is not private", info, err)
	}
	reopened, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.MissionExecution(t.Context(), request.ProjectID, request.MissionID)
	if err != nil || !reflect.DeepEqual(loaded, record) {
		t.Fatal("restart lost the frozen receipt", loaded, err)
	}
	if _, err := reopened.PrepareMissionExecution(t.Context(), request, allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("restart reset allocations", err)
	}
	revision := saved.Mission
	revision.Plan.Goal = "A separately edited goal"
	revised, err := store.SaveMission(t.Context(), revision, saved.CatalogRevision, saved.Mission.PlanHash, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := reopened.CancelMissionExecution(t.Context(), missionExecutionIdentity(record), allowMemoryWrite)
	if err != nil || canceled.State != "canceled" || canceled.Revision != 2 || !reflect.DeepEqual(canceled.Mission, record.Mission) || !reflect.DeepEqual(canceled.Children, record.Children) {
		t.Fatal("cancel changed frozen evidence", canceled, err)
	}
	if _, err := store.CancelMissionExecution(t.Context(), missionExecutionIdentity(record), allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("cancellation replay accepted", err)
	}
	if _, err := store.PrepareMissionExecution(t.Context(), missionExecutionPrepare(revised), allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("edited/canceled plan reset execution", err)
	}
	if _, err := store.MissionExecution(t.Context(), "another-project", request.MissionID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign execution disclosed", err)
	}
}

func TestMissionExecutionConcurrentPreparationCreatesExactlyOneAllocation(t *testing.T) {
	store := memoryStore(t)
	request := missionExecutionPrepare(approvedMissionFixture(t, store))
	var wg sync.WaitGroup
	start, results := make(chan struct{}), make(chan error, 8)
	for range 8 {
		other, err := Open(store.root)
		if err != nil {
			t.Fatal(err)
		}
		wg.Go(func() {
			<-start
			_, err := other.PrepareMissionExecution(t.Context(), request, allowMemoryWrite)
			results <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrMissionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 7 {
		t.Fatal("concurrent requests issued more than one budget", success, conflicts)
	}
}

func TestMissionExecutionRejectsUnapprovedOrChangedPreparation(t *testing.T) {
	for name, mutate := range map[string]func(*MissionExecutionPrepare){
		"catalog": func(r *MissionExecutionPrepare) { r.CatalogRevision-- },
		"hash":    func(r *MissionExecutionPrepare) { r.PlanHash = strings.Repeat("b", 64) },
		"project": func(r *MissionExecutionPrepare) { r.ProjectID = "foreign" },
		"mission": func(r *MissionExecutionPrepare) { r.MissionID = "unknown" },
		"head":    func(r *MissionExecutionPrepare) { r.Base.Head = "not-a-commit" },
		"branch":  func(r *MissionExecutionPrepare) { r.Base.Branch = "\x1b[2J" },
	} {
		t.Run(name, func(t *testing.T) {
			store := memoryStore(t)
			request := missionExecutionPrepare(approvedMissionFixture(t, store))
			mutate(&request)
			if _, err := store.PrepareMissionExecution(t.Context(), request, allowMemoryWrite); err == nil {
				t.Fatal("invalid preparation accepted")
			}
			path, _ := store.path("mission-executions.json")
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("rejected preparation wrote evidence", err)
			}
		})
	}
	store := memoryStore(t)
	request := missionExecutionPrepare(saveMissionFixture(t, store))
	if _, err := store.PrepareMissionExecution(t.Context(), request, allowMemoryWrite); err == nil {
		t.Fatal("unapproved mission prepared")
	}
}

func TestMissionExecutionFailedOrCanceledPublicationPreservesEvidence(t *testing.T) {
	for _, phase := range []string{"prepare", "cancel"} {
		for _, failure := range []string{"write", "authorization", "canceled at rename", "deadline in authorization"} {
			t.Run(phase+"/"+failure, func(t *testing.T) {
				store := memoryStore(t)
				request := missionExecutionPrepare(approvedMissionFixture(t, store))
				var record MissionExecution
				if phase == "cancel" {
					var err error
					record, err = store.PrepareMissionExecution(t.Context(), request, allowMemoryWrite)
					if err != nil {
						t.Fatal(err)
					}
				}
				path, _ := store.path("mission-executions.json")
				before, readErr := os.ReadFile(path)
				if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
					t.Fatal(readErr)
				}
				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(nil)
				authorize := allowMemoryWrite
				switch failure {
				case "write":
					store.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("fixture disk full") }
				case "authorization":
					authorize = func() error { return errors.New("changed source") }
				case "canceled at rename":
					write := store.write
					store.write = func(path string, data []byte, mode os.FileMode, check func() error) error {
						cancel(context.Canceled)
						return write(path, data, mode, check)
					}
				case "deadline in authorization":
					calls := 0
					authorize = func() error {
						calls++
						if calls == 2 {
							cancel(context.DeadlineExceeded)
						}
						return nil
					}
				}
				var err error
				if phase == "prepare" {
					_, err = store.PrepareMissionExecution(ctx, request, authorize)
				} else {
					_, err = store.CancelMissionExecution(ctx, missionExecutionIdentity(record), authorize)
				}
				if err == nil || failure == "write" && !errors.Is(err, ErrPersistence) || context.Cause(ctx) != nil && !errors.Is(err, context.Cause(ctx)) {
					t.Fatal("publication failure lost its meaning", err)
				}
				after, err := os.ReadFile(path)
				if !bytes.Equal(before, after) || errors.Is(readErr, os.ErrNotExist) != errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed publication changed evidence", err)
				}
			})
		}
	}
}

func TestMissionExecutionRejectsCorruptionAndPreservesBytes(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"version":            func(v map[string]any) { v["version"] = 99 },
		"workspace":          func(v map[string]any) { v["workspace"] = "foreign" },
		"unknown":            func(v map[string]any) { v["unknown"] = true },
		"missing records":    func(v map[string]any) { v["records"] = nil },
		"missing allocation": func(v map[string]any) { executionJSONRecord(v)["children"] = []any{} },
		"approval":           func(v map[string]any) { executionJSONRecord(v)["mission"].(map[string]any)["approved"] = false },
		"version replay":     func(v map[string]any) { executionJSONRecord(v)["execution_revision"] = 0 },
		"forged allocation": func(v map[string]any) {
			executionJSONRecord(v)["children"].([]any)[0].(map[string]any)["limits"].(map[string]any)["model_calls"] = 100
		},
		"forged running": func(v map[string]any) { executionJSONRecord(v)["state"] = "running" },
	} {
		t.Run(name, func(t *testing.T) {
			store := memoryStore(t)
			request := missionExecutionPrepare(approvedMissionFixture(t, store))
			record, err := store.PrepareMissionExecution(t.Context(), request, allowMemoryWrite)
			if err != nil {
				t.Fatal(err)
			}
			path, _ := store.path("mission-executions.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			mutate(decoded)
			data, err = json.Marshal(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.MissionExecution(t.Context(), request.ProjectID, request.MissionID); !errors.Is(err, ErrCorrupt) {
				t.Fatal(err)
			}
			if _, err := store.CancelMissionExecution(t.Context(), missionExecutionIdentity(record), allowMemoryWrite); !errors.Is(err, ErrCorrupt) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatal("corruption overwritten", err)
			}
		})
	}
}

func executionJSONRecord(value map[string]any) map[string]any {
	return value["records"].(map[string]any)["mission"].(map[string]any)
}

func TestMissionExecutionReadIsPassiveAndBoundsPrivateStorage(t *testing.T) {
	store := memoryStore(t)
	if _, err := store.MissionExecution(t.Context(), "project", "mission"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	path, _ := store.path("mission-executions.json")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("passive read wrote metadata", err)
	}
	request := missionExecutionPrepare(approvedMissionFixture(t, store))
	record, err := store.PrepareMissionExecution(t.Context(), request, allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"permission", "oversized", "duplicate keys"} {
		t.Run(invalid, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Chmod(path, 0600); err != nil {
					t.Error(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Error(err)
				}
			}()
			switch invalid {
			case "permission":
				err = os.Chmod(path, 0644)
			case "oversized":
				err = os.WriteFile(path, bytes.Repeat([]byte(" "), maxMissionCatalog+1), 0600)
			case "duplicate keys":
				err = os.WriteFile(path, bytes.Replace(data, []byte(`"version":`), []byte(`"version":2,"version":`), 1), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.MissionExecution(t.Context(), request.ProjectID, request.MissionID); !errors.Is(err, ErrCorrupt) {
				t.Fatal(err)
			}
		})
	}
	for name, mutate := range map[string]func(*MissionExecutionIdentity){
		"execution": func(r *MissionExecutionIdentity) { r.ExecutionID = "different" },
		"revision":  func(r *MissionExecutionIdentity) { r.Revision++ },
		"hash":      func(r *MissionExecutionIdentity) { r.PlanHash = "different" },
	} {
		t.Run(name, func(t *testing.T) {
			identity := missionExecutionIdentity(record)
			mutate(&identity)
			if _, err := store.CancelMissionExecution(t.Context(), identity, allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
				t.Fatal(err)
			}
		})
	}
}
