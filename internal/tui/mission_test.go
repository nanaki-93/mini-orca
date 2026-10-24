package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

const missionUIID = "mission-abcdefghijklmnopqrst"

func missionTestState() *State {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}, Path: "/repo", Name: "Mission fixture"}
	s.Index = &ProjectIndex{ProjectIdentity: s.Project.ProjectIdentity, Files: []IndexedFile{{Path: "main.go", ContentHash: "sha256:" + strings.Repeat("a", 64), LineCount: 2}}}
	s.Connected = true
	limits := client.AgentLimits{ModelCalls: 8, ToolCalls: 16, InputTokens: 1 << 20, OutputTokens: 8 << 20, ElapsedSeconds: 120, ConcurrentCalls: 1, Repairs: 1}
	settings := &client.AgentSettings{ProjectIdentity: s.Project.ProjectIdentity, Version: 1, Configuration: "configuration", State: "configured", Limits: limits, Ceilings: limits, Models: []client.Model{{Scope: "function", Model: "fixture", ProviderOrigin: "https://provider.example", RemoteProvider: true, Capabilities: client.ModelCapabilities{Completion: true, Tools: true}}}}
	s.Mission = &missionView{Parent: s.Project.ProjectIdentity, Scope: client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, Path: s.Project.Path, Epoch: "initial", Catalog: &client.MissionList{ProjectIdentity: s.Project.ProjectIdentity, Missions: []client.MissionSummary{}}, Settings: settings, Index: s.Index}
	s.ExecuteCommand(":mission-new Extract helper")
	s.ExecuteCommand(":mission-edit files new.go; main.go")
	return s
}

func missionTestPlan(s *State) client.MissionPlan {
	plan := cloneMissionPlan(*s.Mission.Draft)
	limits := client.AgentLimits{ModelCalls: 2, ToolCalls: 4, InputTokens: 4096, OutputTokens: 2048, ElapsedSeconds: 30, ConcurrentCalls: 1}
	plan.Children = []client.MissionChild{
		{ID: "helper", Title: "Extract helper", Approach: "Add a pure helper", AcceptanceCriteria: []string{"Returns expected value"}, Dependencies: []string{}, WritePaths: []string{"new.go"}, ReadPaths: []string{}, Limits: limits},
		{ID: "use", Title: "Use helper", Approach: "Call the helper from Run", AcceptanceCriteria: []string{"Run preserves behavior"}, Dependencies: []string{"helper"}, WritePaths: []string{"main.go"}, ReadPaths: []string{"new.go"}, Limits: limits},
	}
	plan.IntegrationOrder = []string{"helper", "use"}
	return plan
}

func missionTestDetail(s *State, plan client.MissionPlan, revision int64) *client.AgentMission {
	return &client.AgentMission{CatalogRevision: revision, SourceStatus: "current", Mission: client.MissionRecord{ProjectIdentity: s.Mission.Scope.ProjectIdentity, ID: missionUIID, PlanHash: strings.Repeat("b", 64), Plan: plan, Reviewed: []string{}}}
}

func missionTestPlanning(s *State, plan client.MissionPlan) *client.MissionPlanningResult {
	return &client.MissionPlanningResult{AgentMission: *missionTestDetail(s, plan, s.Mission.Catalog.CatalogRevision+1), InputHash: strings.Repeat("c", 64), PromptVersion: "mission-planner-v1", PlanningLimits: s.Mission.PlanningLimits, Usage: client.AgentUsage{ModelCalls: 1, InputTokens: 100, OutputTokens: 50}}
}

func missionTestPending(t *testing.T, ctx context.Context, s *State, cl *Client) (*BubbleModel, tea.Cmd) {
	t.Helper()
	if len(s.PendingActions) != 1 {
		t.Fatalf("expected one mission request, got %d: %s", len(s.PendingActions), s.Error)
	}
	m := NewBubbleModel(ctx, s, cl, 80, 24)
	cmd, ok := m.prepareAsyncActionCmd("mission-test", s.PendingActions[0])
	s.PendingActions = nil
	if !ok || cmd == nil {
		t.Fatalf("mission action was not prepared: %s", s.Error)
	}
	return m, cmd
}

func completeMissionTestAction(t *testing.T, s *State, cl *Client) {
	t.Helper()
	m, command := missionTestPending(t, t.Context(), s, cl)
	m.Update(command())
	if s.Error != "" || s.Mission.Pending != nil {
		t.Fatalf("mission action did not complete: %s", s.Error)
	}
}

func missionReadFixture(t *testing.T, w http.ResponseWriter, r *http.Request, settings *client.AgentSettings, index *ProjectIndex, catalog *client.MissionList) bool {
	t.Helper()
	if r.Method != http.MethodGet {
		return false
	}
	var value any
	switch r.URL.Path {
	case "/api/projects/current/agent-settings":
		value = settings
	case "/api/projects/current/index":
		value = index
	case "/api/projects/current/missions":
		value = catalog
	default:
		return false
	}
	if r.URL.Query().Get("project_id") != settings.ProjectID || r.URL.Query().Get("project_revision") != settings.ProjectRevision {
		t.Error("mission read lost its current checkout identity")
	}
	_ = json.NewEncoder(w).Encode(value)
	return true
}

func TestMissionTUIPlansReviewsApprovesAndInvalidatesEditedApproval(t *testing.T) {
	s := missionTestState()
	settings, index, catalog := *s.Mission.Settings, *s.Index, *s.Mission.Catalog
	plan := missionTestPlan(s)
	planning := missionTestPlanning(s, plan)
	stored := planning.AgentMission
	var writes atomic.Int32
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if missionReadFixture(t, w, r, &settings, &index, &catalog) {
			return
		}
		writes.Add(1)
		if r.URL.Path == "/api/projects/current/missions/plan" {
			var request client.MissionPlanningRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.ConfirmRemoteProvider || request.ProjectIdentity != settings.ProjectIdentity || request.CatalogRevision != 0 || !slices.Equal(request.Sources, plan.Sources) || request.PlanningLimits != planning.PlanningLimits {
				t.Error("planner lost confirmed input", err)
			}
			_ = json.NewEncoder(w).Encode(planning)
			return
		}
		if r.URL.Path != "/api/projects/current/missions/"+missionUIID || r.Method != "POST" {
			t.Error("mission UI dispatched an unrelated action", r.Method, r.URL.Path)
			http.Error(w, "unexpected route", 500)
			return
		}
		var request client.MissionMutation
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.PlanHash != stored.Mission.PlanHash || request.CatalogRevision != stored.CatalogRevision {
			t.Error("review lost exact plan authority", err)
		}
		stored.CatalogRevision++
		switch request.Action {
		case "review":
			stored.Mission.Reviewed = append(stored.Mission.Reviewed, request.ChildID)
		case "approve":
			stored.Mission.Approved = true
		case "revise":
			stored.Mission.Plan = *request.Plan
			stored.Mission.PlanHash = strings.Repeat("d", 64)
			stored.Mission.Approved, stored.Mission.Reviewed = false, []string{}
		default:
			t.Error("unexpected mission mutation", request.Action)
		}
		_ = json.NewEncoder(w).Encode(stored)
	})
	s.HandleKey(KeyEvent{Key: "g"})
	for _, token := range []string{"/repo", "revision", "https://provider.example", "new.go", "sha256:", "Planning budget", "Future mission allocation"} {
		if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, token) {
			t.Fatal("planning disclosure missing", token)
		}
	}
	if len(s.PendingActions) != 0 {
		t.Fatal("planning dispatched before confirmation")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	s.confirmMissionPlanning()
	if len(s.PendingActions) != 1 {
		t.Fatal("duplicate mission submission")
	}
	s.Error = ""
	completeMissionTestAction(t, s, cl)
	if s.Mission.Saved == nil || s.Mission.Saved.Mission.Approved || len(s.Mission.Saved.Mission.Reviewed) != 0 {
		t.Fatal("model proposal became approved")
	}
	s.confirmMissionReview(true)
	if s.Confirmation != nil || s.Error == "" {
		t.Fatal("mission approved before child reviews")
	}
	s.Error = ""
	for _, id := range []string{"helper", "use"} {
		s.selectMissionChild(id)
		s.HandleKey(KeyEvent{Key: "r"})
		if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, stored.Mission.PlanHash) || !strings.Contains(s.Confirmation.Message, "Acceptance:") {
			t.Fatal("review did not disclose concrete child and exact hash")
		}
		s.HandleKey(KeyEvent{Key: "Enter"})
		completeMissionTestAction(t, s, cl)
	}
	s.HandleKey(KeyEvent{Key: "a"})
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "does not execute") {
		t.Fatal("approval omitted its planning-only meaning")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	completeMissionTestAction(t, s, cl)
	if !s.Mission.Saved.Mission.Approved || writes.Load() != 4 {
		t.Fatal("review sequence did not produce one exact approval", writes.Load())
	}
	s.ExecuteCommand(":mission-edit approach Preserve errors explicitly")
	s.confirmMissionReview(true)
	if s.Error == "" || len(s.PendingActions) != 0 || !strings.Contains(s.missionStateText(), "EDITED LOCALLY") {
		t.Fatal("local edit reused prior approval")
	}
	s.Error = ""
	s.HandleKey(KeyEvent{Key: "s"})
	completeMissionTestAction(t, s, cl)
	if s.Mission.Saved.Mission.Approved || len(s.Mission.Saved.Mission.Reviewed) != 0 || writes.Load() != 5 {
		t.Fatal("revision retained review authority")
	}
}

func TestMissionTUIRejectsChangedConfirmationAndQueuedInputs(t *testing.T) {
	for _, queued := range []bool{false, true} {
		for _, changed := range []string{"goal", "budget", "model", "project", "catalog"} {
			t.Run(fmt.Sprintf("%t-%s", queued, changed), func(t *testing.T) {
				s := missionTestState()
				s.confirmMissionPlanning()
				if queued {
					s.HandleKey(KeyEvent{Key: "Enter"})
				}
				switch changed {
				case "goal":
					s.Mission.Draft.Goal = "Replaced draft"
				case "budget":
					s.Mission.PlanningLimits.ModelCalls--
				case "model":
					s.Mission.Settings.Models[0].ProviderOrigin = "https://changed.example"
				case "project":
					s.Project.ProjectRevision = "changed"
				case "catalog":
					s.Mission.Catalog.CatalogRevision++
				}
				if queued {
					if s.prepareMissionAction(s.PendingActions[0].Mission) {
						t.Fatal("stale queued input dispatched")
					}
				} else {
					s.HandleKey(KeyEvent{Key: "Enter"})
					if len(s.PendingActions) != 0 {
						t.Fatal("changed confirmation dispatched")
					}
				}
			})
		}
	}
}

func TestMissionTUIRechecksProviderBeforeHTTPPlanning(t *testing.T) {
	s := missionTestState()
	settings := *s.Mission.Settings
	settings.Models = slices.Clone(settings.Models)
	settings.Models[0].ProviderOrigin = "https://replaced.example"
	var posts atomic.Int32
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts.Add(1)
		}
		_ = json.NewEncoder(w).Encode(settings)
	})
	s.confirmMissionPlanning()
	s.HandleKey(KeyEvent{Key: "Enter"})
	m, command := missionTestPending(t, t.Context(), s, cl)
	m.Update(command())
	if posts.Load() != 0 || s.Mission.Saved != nil || s.Mission.Draft.Goal != "Extract helper" || !strings.Contains(s.Error, "changed after confirmation") {
		t.Fatal("changed provider dispatched or lost draft", s.Error)
	}
}

func TestMissionTUICancellationAndScopeReplacementOwnTransport(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			s := missionTestState()
			settings, index, catalog := *s.Mission.Settings, *s.Index, *s.Mission.Catalog
			started := make(chan struct{})
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if missionReadFixture(t, w, r, &settings, &index, &catalog) {
					return
				}
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
			})
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			s.confirmMissionPlanning()
			s.HandleKey(KeyEvent{Key: "Enter"})
			original := *s.Mission.Pending
			late := missionTestPlanning(s, missionTestPlan(s))
			m, command := missionTestPending(t, ctx, s, cl)
			done := make(chan tea.Msg, 1)
			go func() { done <- command() }()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("mission transport never started")
			}
			if replace {
				s.Project.ProjectRevision = "replacement"
				m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			} else {
				m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
			}
			select {
			case message := <-done:
				m.Update(message)
			case <-ctx.Done():
				t.Fatal("canceled mission transport leaked")
			}
			s.applyMissionResult(missionResult{Action: original, Planning: late, Detail: &late.AgentMission})
			if replace {
				if s.Mission != nil {
					t.Fatal("changed project retained private mission state")
				}
			} else if s.Mission.Draft.Goal != "Extract helper" || s.Mission.Saved != nil || s.Mission.Pending != nil || !strings.Contains(s.Mission.Failure, "canceled") {
				t.Fatal("late result replaced a canceled draft")
			}
		})
	}
}

func TestMissionTUILocalEditingAndNavigationArePassive(t *testing.T) {
	s := missionTestState()
	for _, command := range []string{":mission-child helper", ":mission-edit title Pure helper", ":mission-edit approach Return a constant", ":mission-edit criteria Returns one; Preserves error", ":mission-edit writes new.go", ":mission-edit reads main.go", ":mission-edit depends", ":mission-edit order helper", ":mission-budget child model_calls 2", ":mission-budget planning repairs 0"} {
		s.ExecuteCommand(command)
		if s.Error != "" || len(s.PendingActions) != 0 {
			t.Fatal("local editor dispatched or failed", command, s.Error)
		}
	}
	if s.missionChild().Limits.ModelCalls != 2 || len(s.missionChild().AcceptanceCriteria) != 2 {
		t.Fatal("local child editing lost content")
	}
	s.ExecuteCommand(":mission-budget planning repairs 1")
	if s.Error == "" {
		t.Fatal("planning repair ceiling was relaxed")
	}
	s.Error = ""
	for _, key := range []string{"ArrowLeft", "ArrowRight", "PageDown", "PageUp", "Tab"} {
		s.HandleKey(KeyEvent{Key: key})
	}
	if len(s.PendingActions) != 0 || s.Confirmation != nil || s.Page == PagePlan {
		t.Fatal("navigation dispatched or trapped Tab")
	}
	s.ExecuteCommand(":mission-remove helper")
	if len(s.Mission.Draft.Children) != 0 || len(s.Mission.Draft.IntegrationOrder) != 0 {
		t.Fatal("local child removal failed")
	}
}

func TestMissionTUIViewsAndConfirmationsFitEveryPalette(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
			s := missionTestState()
			s.Palette = palette
			plan := missionTestPlan(s)
			plan.Children[0].Approach = "unsafe\x1b[2J\n" + strings.Repeat("Detailed approach ", 60) + "end-of-child"
			s.Mission.Draft, s.Mission.ChildID = &plan, "helper"
			var visited strings.Builder
			for section := range 4 {
				s.DetailSection, s.DetailScroll = section, 0
				for range 45 {
					frame := s.Render(size[0], size[1])
					assertFrameSize(t, frame, size[0], size[1])
					visited.WriteString(frame)
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
			}
			for _, token := range []string{"MISSION PLAN", "end-of-child", "Captured inventory:", "Planning run:", "Future mission:"} {
				if !strings.Contains(visited.String(), token) {
					t.Fatal("mission content inaccessible", token, palette, size)
				}
			}
			if strings.Contains(visited.String(), "\x1b[2J") || len(s.PendingActions) != 0 {
				t.Fatal("mission view emitted controls or dispatched")
			}
			s.confirmMissionPlanning()
			for range 20 {
				frame := s.Render(size[0], size[1])
				assertFrameSize(t, frame, size[0], size[1])
				if !strings.Contains(frame, "Confirm") {
					t.Fatal("scrolling hid the confirmation action", palette, size)
				}
				s.HandleKey(KeyEvent{Key: "PageDown"})
			}
			s.HandleKey(KeyEvent{Key: "Escape"})
			if s.Confirmation != nil || len(s.PendingActions) != 0 {
				t.Fatal("canceling confirmation dispatched")
			}
		}
	}
}

func TestMissionTUIReadsSelectedWorktreeAndClearsReplacedScope(t *testing.T) {
	s := missionTestState()
	settings := *s.Mission.Settings
	settings.ProjectIdentity = ProjectIdentity{ProjectID: "task-project", ProjectRevision: "task-revision"}
	index := *s.Index
	index.ProjectIdentity = settings.ProjectIdentity
	s.Change = &ChangeSession{ProjectIdentity: settings.ProjectIdentity, WorktreePath: "/worktrees/task"}
	plan := missionTestPlan(s)
	detail := missionTestDetail(s, plan, 1)
	detail.Mission.ProjectIdentity = settings.ProjectIdentity
	catalog := client.MissionList{ProjectIdentity: settings.ProjectIdentity, CatalogRevision: 1, Missions: []client.MissionSummary{{ID: missionUIID, PlanHash: detail.Mission.PlanHash, ProjectRevision: "task-revision", Goal: plan.Goal, ChildCount: 2, SourceStatus: "unchecked"}}}
	var posts atomic.Int32
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			posts.Add(1)
		}
		if r.URL.Query().Get("worktree_path") != "/worktrees/task" {
			t.Error("mission request lost worktree ownership")
		}
		if missionReadFixture(t, w, r, &settings, &index, &catalog) {
			return
		}
		if r.URL.Path != "/api/projects/current/missions/"+missionUIID {
			t.Error("unexpected detail route", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(detail)
	})
	s.ExecuteCommand(":mission-scope task")
	completeMissionTestAction(t, s, cl)
	s.HandleKey(KeyEvent{Key: "Enter"})
	completeMissionTestAction(t, s, cl)
	if posts.Load() != 0 || s.Mission.Saved.Mission.ProjectIdentity != settings.ProjectIdentity || s.Mission.Scope.WorktreePath != "/worktrees/task" {
		t.Fatal("passive worktree mission history crossed scope or dispatched")
	}
	s.Change.WorktreePath = "/worktrees/replacement"
	m := NewBubbleModel(t.Context(), s, cl, 80, 24)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if s.Mission != nil {
		t.Fatal("changed task checkout retained private mission state")
	}
}

func TestMissionTUIDoesNotReuseOrdinaryTaskGenerationControls(t *testing.T) {
	s := missionTestState()
	s.openTaskPlan(&ChangeSeed{Title: "Single task", Paths: []string{"main.go"}, AcceptanceCriteria: []string{"Preserve behavior"}, Plan: &ChangePlanInput{Problem: "Single task", Approach: "Small change"}})
	original := s.PlanDraft
	s.ExecuteCommand(":mission-draft")
	s.ExecuteCommand(":generate")
	if s.Error == "" || len(s.PendingActions) != 0 || s.PlanDraft != original {
		t.Fatal("mission view reused single-task generation")
	}
	s.Error = ""
	s.ExecuteCommand(":plan")
	if s.PlanView != "" || s.PlanDraft != original || s.PlanDraft.Seed.Title != "Single task" {
		t.Fatal("ordinary task draft was lost")
	}
	s.HandleKey(KeyEvent{Key: "e"})
	if !s.CmdActive || s.CmdInput != "approach Small change" {
		t.Fatal("ordinary task editor changed behavior")
	}
}

func TestMissionTUIRefreshPreservesDraftAndSelectedRecord(t *testing.T) {
	s := missionTestState()
	plan := missionTestPlan(s)
	s.Mission.Draft = &plan
	s.Mission.Saved = missionTestDetail(s, plan, 2)
	s.updateMissionSummary(s.Mission.Saved)
	other := s.Mission.Catalog.Missions[0]
	other.ID = "mission-anotherabcdefghijkl"
	s.Mission.Catalog.Missions = append(s.Mission.Catalog.Missions, other)
	s.Mission.Selection = 1
	s.editMissionField("goal", "Retain local changes")
	s.queueMissionAction(missionAction{Operation: "read"})
	action := *s.Mission.Pending
	catalog := *s.Mission.Catalog
	catalog.Missions = []client.MissionSummary{other, catalog.Missions[0]}
	s.applyMissionResult(missionResult{Action: action, Catalog: &catalog, Settings: s.Mission.Settings, Index: s.Mission.Index})
	if s.Mission.Selection != 0 || s.Mission.Draft.Goal != "Retain local changes" {
		t.Fatal("refresh replaced the local draft or selected identity")
	}
	s.Mission.Saved.Stale, s.Mission.Saved.SourceStatus = true, "changed"
	s.confirmMissionReview(true)
	if s.Confirmation != nil || s.Error == "" {
		t.Fatal("stale mission remained approvable")
	}
	s.Error = ""
	s.Mission.History = true
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		s.Palette = palette
		assertFrameSize(t, s.Render(52, 16), 52, 16)
	}
}
