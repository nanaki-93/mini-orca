package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func writeInterruptedMissionFixture(t *testing.T, root string, record agentstate.MissionExecution) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*", "mission-executions.json"))
	if err != nil || len(paths) != 1 {
		t.Fatal(paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var catalog map[string]json.RawMessage
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	catalog["records"], err = json.Marshal(map[string]agentstate.MissionExecution{record.Mission.ID: record})
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMissionChildRestartReconcilesOwnedEvidenceWithoutDispatch(t *testing.T) {
	for _, evidence := range []string{"completed", "changed source", "missing worktree", "foreign origin"} {
		t.Run(evidence, func(t *testing.T) {
			var calls atomic.Int32
			service, root, prepared := missionSchedulerFixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); missionFixtureAnswer(t, w, r) }, 12)
			request := missionStartRequest(prepared)
			if _, err := service.TrustProjectExecution(t.Context(), request.ProjectRevision, true); err != nil {
				t.Fatal(err)
			}
			started, err := service.ControlMissionExecution(t.Context(), prepared.Execution.Mission.ID, request)
			if err != nil {
				t.Fatal(err)
			}
			finished := waitMissionFixture(t, service, prepared.Execution.Mission.ID)
			if finished.Execution.State != "awaiting_review" {
				t.Fatal(finished)
			}
			crash := finished.Execution
			crash.State, crash.Wave, crash.ElapsedMillis = "running", started.Execution.Wave, 0
			crash.Children[0].State = "running"
			writeInterruptedMissionFixture(t, root, crash)
			restarted, err := New(service.config, service.manager)
			if err != nil {
				t.Fatal(err)
			}
			query := settingsQuery(t, restarted)
			current, err := restarted.MissionExecution(t.Context(), query, crash.Mission.ID)
			if err != nil || current.OwnerState != "waiting_for_owner" {
				t.Fatal(current, err)
			}
			control := cancelExecutionRequest(current, query)
			control.Action = "reconcile"
			if _, err := restarted.ControlMissionExecution(t.Context(), crash.Mission.ID, control); err == nil {
				t.Fatal("restart stole a live lease")
			}
			crash.Wave.StartedAt = time.Now().Add(-time.Duration(crash.Wave.AllowanceMillis+1) * time.Millisecond)
			writeInterruptedMissionFixture(t, root, crash)
			child := crash.Children[0]
			path := filepath.Join(root, ".worktrees", child.Directory)
			switch evidence {
			case "changed source":
				err = os.WriteFile(filepath.Join(path, "main.go"), []byte("package main\n// changed after completion\n"), 0600)
			case "missing worktree":
				err = os.Rename(path, path+"-moved")
			case "foreign origin":
				var session *ChangeSession
				session, err = loadChangeSession(path, child.SessionID)
				if err == nil {
					session.MissionOrigin.ExecutionID = "different-execution"
					err = storeChangeSession(path, session)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err := restarted.ControlMissionExecution(t.Context(), crash.Mission.ID, control)
			if err != nil || result.Execution.Wave != nil || result.OwnerState != "idle" || result.Execution.ElapsedMillis != crash.Wave.AllowanceMillis {
				t.Fatal(result, err)
			}
			want := "interrupted"
			if evidence == "completed" {
				want = "awaiting_review"
			}
			if result.Execution.Children[0].State != want || result.Execution.Children[0].SessionID != child.SessionID || result.Execution.Children[1].State != "pending" || calls.Load() != 5 {
				t.Fatal("recovery lost evidence or dispatched", result, calls.Load())
			}
			if _, err := restarted.ControlMissionExecution(t.Context(), crash.Mission.ID, missionStartRequest(result)); err == nil {
				t.Fatal("recovery reset allocation")
			}
		})
	}
}

func TestMissionChildContextDoesNotReuseBroaderComposition(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	if err := os.WriteFile(filepath.Join(root, "sibling.go"), []byte("package main\nfunc UnselectedSibling() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	root = service.manager.Root()
	settings, err := config.ResolveAgentContext(config.AgentContext{LocalCache: true})
	if err != nil {
		t.Fatal(err)
	}
	service.agentRuntime.Context = settings
	session := openChangeFixture(t, service, "main.go", "new.go")
	ctx, release, err := service.agentRunContext(t.Context(), service.runtimes.function)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx = agentruntime.WithContextAuthority(ctx, func(ctx context.Context) error { return service.verifyChangeCurrent(ctx, root, session) })
	broad, err := composedChangeContext(ctx, root, session, false)
	if err != nil || !strings.Contains(broad.Text, "UnselectedSibling") {
		t.Fatal("fixture did not include sibling declaration", err)
	}
	session.MissionOrigin = &MissionChildOrigin{MissionID: "mission", ExecutionID: "execution", ChildID: "child", PlanHash: strings.Repeat("a", 64)}
	narrow, err := composedChangeContext(ctx, root, session, false)
	if err != nil || strings.Contains(narrow.Text, "UnselectedSibling") {
		t.Fatal("mission reused a broader context", err)
	}
	if err := requireMissionProviderAdmission(ctx, session); err == nil {
		t.Fatal("unowned mission proposal granted provider access")
	}
}
