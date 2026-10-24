package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

const missionExecutionUIID = "mission-run-abcdefghijklmnopqrstuvwx"

func missionExecutionTestState(task bool) *State {
	s := missionTestState()
	v := s.Mission
	if task {
		v.Scope = client.AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "task-project", ProjectRevision: "task-revision"}, WorktreePath: "/repo/.worktrees/owner"}
		v.Path = v.Scope.WorktreePath
		index := *s.Index
		index.ProjectIdentity = v.Scope.ProjectIdentity
		v.Index = &index
		v.Settings.ProjectIdentity = v.Scope.ProjectIdentity
		v.Catalog.ProjectIdentity = v.Scope.ProjectIdentity
		s.Change = &ChangeSession{ProjectIdentity: v.Scope.ProjectIdentity, ID: "owner-task", WorktreePath: v.Path}
		v.OwnerChange = s.Change
	}
	plan := missionTestPlan(s)
	v.Draft, v.Saved = &plan, missionTestDetail(s, plan, 4)
	v.Saved.Mission.Approved, v.Saved.Mission.Reviewed = true, []string{"helper", "use"}
	v.Saved.Mission.Instructions = map[string]string{"main.go": "instructions", "new.go": "instructions"}
	v.Catalog.CatalogRevision = 4
	v.Settings.Roles = client.AgentRoles{Creation: "function", Testing: "function", Review: "function"}
	v.Repository = &GitRepository{ProjectIdentity: v.Scope.ProjectIdentity, GitIdentity: client.GitIdentity{Head: strings.Repeat("a", 40), Branch: "main"}, Root: v.Path, Available: true, AtRoot: true}
	v.Trust = &ExecutionTrust{ProjectIdentity: v.Scope.ProjectIdentity, Trusted: true, Commands: [][]string{{"go", "test", "./..."}, {"go", "vet", "./..."}}}
	r := client.MissionExecution{ID: missionExecutionUIID, Revision: 1, CatalogRevision: 4, Mission: v.Saved.Mission, State: "prepared", Base: v.Repository.GitIdentity, Current: client.MissionCheckpoint{ProjectRevision: v.Scope.ProjectRevision, Base: v.Repository.GitIdentity, Sources: slices.Clone(plan.Sources), Instructions: v.Saved.Mission.Instructions}}
	for _, child := range plan.Children {
		r.Children = append(r.Children, client.MissionExecutionChild{ID: child.ID, State: "pending", Limits: child.Limits})
	}
	v.Execution = &client.AgentMissionExecution{Execution: r, SourceStatus: "current", OwnerState: "idle", ReadyChildren: []string{"helper"}}
	v.ExecutionLoaded, v.ExecutionOpen, v.ChildID = true, true, "helper"
	s.PlanView = "mission"
	s.Navigate(PagePlan)
	return s
}

func missionExecutionCandidate(s *State) *ChangeSession {
	v := s.Mission
	r := &v.Execution.Execution
	child := &r.Children[0]
	child.State, child.WaveID = "awaiting_review", "wave-abcdefghijklmnopqrstuvwx"
	child.Directory, child.Branch, child.BaseHead = r.ID+"-helper", "codex/"+r.ID+"-helper", r.Current.Base.Head
	child.ProjectID, child.ProjectRevision, child.SessionID, child.WorkflowID = "child-project", "child-revision", "child-session", "child-workflow"
	child.ProposalHash, child.ProposalRevision = strings.Repeat("c", 64), 3
	r.State, r.Revision, v.Execution.ReadyChildren = "awaiting_review", 4, nil
	proposal := &ChangeSession{ProjectIdentity: ProjectIdentity{ProjectID: child.ProjectID, ProjectRevision: child.ProjectRevision}, ID: child.SessionID, Revision: int(child.ProposalRevision), Hash: child.ProposalHash, Kind: "feature", Title: "Extract helper", State: "draft", Freshness: "current", Plan: &ChangePlan{Hash: "captured-plan"}, Workflow: &ChangeWorkflow{ID: child.WorkflowID, Status: "awaiting_human_review"}, MissionOrigin: &client.MissionChildOrigin{MissionID: r.Mission.ID, ExecutionID: r.ID, ChildID: child.ID, PlanHash: r.Mission.PlanHash}, WorktreePath: missionCandidatePath(v.Path, *child), Changes: []FileChange{{Path: "new.go", Content: "package fixture\n", Hash: "new-content", Diff: FileDiff{NewPath: "new.go", Lines: []DiffLine{{Kind: "add", NewLine: 1, Text: "package fixture"}}}}}, Checks: []Check{{Name: "test", Required: true, State: "passed"}}}
	v.Candidate, v.CandidateGit = proposal, &GitRepository{ProjectIdentity: proposal.ProjectIdentity, GitIdentity: client.GitIdentity{Head: child.BaseHead, Branch: child.Branch}, Root: proposal.WorktreePath, Available: true, AtRoot: true}
	return proposal
}

type missionHTTPFixture struct {
	owner     Project
	settings  client.AgentSettings
	index     ProjectIndex
	trust     ExecutionTrust
	repo      GitRepository
	detail    client.AgentMission
	execution *client.AgentMissionExecution
	candidate *ChangeSession
	childGit  *GitRepository
	writes    []string
	control   func(client.MissionExecutionRequest) *client.AgentMissionExecution
}

func missionHTTPFromState(s *State) *missionHTTPFixture {
	v := s.Mission
	f := &missionHTTPFixture{owner: Project{ProjectIdentity: v.Scope.ProjectIdentity, Path: v.Path}, settings: *v.Settings, index: *v.Index, trust: *v.Trust, repo: *v.Repository, detail: *v.Saved, candidate: v.Candidate, childGit: v.CandidateGit}
	data, _ := json.Marshal(v.Execution)
	_ = json.Unmarshal(data, &f.execution)
	return f
}

func (f *missionHTTPFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if r.Method == http.MethodPost {
		f.writes = append(f.writes, r.URL.Path)
	}
	var value any
	switch r.URL.Path {
	case "/api/projects/current":
		value = f.owner
	case "/api/projects/current/agent-settings":
		value = f.settings
	case "/api/projects/current/index":
		value = f.index
	case "/api/projects/current/git/repository":
		value = f.repo
		if f.candidate != nil && r.URL.Query().Get("worktree_path") == f.candidate.WorktreePath {
			value = f.childGit
		}
	case "/api/projects/current/execution-trust":
		if r.Method == http.MethodPost {
			var request struct {
				ProjectRevision string `json:"project_revision"`
				Confirm         bool   `json:"confirm"`
			}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil || !request.Confirm || request.ProjectRevision != f.owner.ProjectRevision {
				t.Error("trust lost exact check confirmation", err)
			}
			f.trust.Trusted = true
		}
		value = f.trust
	case "/api/projects/current/missions/" + missionUIID:
		value = f.detail
	case "/api/projects/current/missions/" + missionUIID + "/execution":
		if r.Method == http.MethodPost {
			var request client.MissionExecutionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if f.control == nil {
				t.Error("unexpected mission mutation")
				http.Error(w, "unexpected mutation", 400)
				return
			}
			value = f.control(request)
		} else {
			if f.execution == nil {
				http.NotFound(w, r)
				return
			}
			value = f.execution
		}
	default:
		if f.candidate == nil || !strings.HasPrefix(r.URL.Path, "/api/projects/current/changes/"+f.candidate.ID) {
			t.Error("unexpected route", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		value = f.proposalAction(t, r)
	}
	_ = json.NewEncoder(w).Encode(value)
}

func (f *missionHTTPFixture) proposalAction(t *testing.T, r *http.Request) any {
	t.Helper()
	p := f.candidate
	if r.URL.Query().Get("worktree_path") != p.WorktreePath {
		t.Error("proposal lost owned checkout")
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/review-file"):
		p.ReviewedFiles, p.FileReviewHash = []string{"new.go"}, p.Hash
	case strings.HasSuffix(r.URL.Path, "/review"):
		if !allProposalFilesReviewed(p) {
			t.Error("human Review bypassed per-file inspection")
		}
		p.ReviewedHash = p.Hash
	case strings.HasSuffix(r.URL.Path, "/apply"):
		if p.ReviewedHash != p.Hash {
			t.Error("Apply bypassed final Review")
		}
		p.State = "applied"
		p.Commit = &ChangeCommit{State: "committed", OID: strings.Repeat("b", 40), Branch: f.childGit.Branch}
		f.childGit.Head = p.Commit.OID
		return ChangeMutation{ProjectIdentity: p.ProjectIdentity, SessionID: p.ID, State: "applied", Hash: p.Hash, Commit: p.Commit}
	}
	return p
}

func completeMissionExecutionAction(t *testing.T, s *State, cl *Client) {
	t.Helper()
	m, command := missionTestPending(t, t.Context(), s, cl)
	m.nativeStreams = false
	m.Update(command())
	if s.Error != "" || s.Mission == nil || s.Mission.Pending != nil {
		t.Fatalf("mission action failed: %s", s.Error)
	}
}

func confirmMissionExecutionTest(t *testing.T, s *State, command string) {
	t.Helper()
	s.ExecuteCommand(":" + command)
	if s.Confirmation == nil {
		t.Fatalf("missing confirmation for %s: %s", command, s.Error)
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
}

func TestMissionExecutionTUIPreparesTrustsAndStartsOnlyConfirmedReadyChildren(t *testing.T) {
	s := missionExecutionTestState(false)
	prepared := s.Mission.Execution
	s.Mission.Execution, s.Mission.ExecutionLoaded, s.Mission.Trust.Trusted = nil, false, false
	f := missionHTTPFromState(s)
	f.control = func(request client.MissionExecutionRequest) *client.AgentMissionExecution {
		switch request.Action {
		case "prepare":
			if request.ConfirmExecution || len(request.ConfirmedProfiles) != 0 || request.CatalogRevision != 4 || request.Base == nil || *request.Base != f.repo.GitIdentity {
				t.Error("prepare acquired execution authority")
			}
			f.execution = prepared
		case "start":
			if !request.ConfirmExecution || !slices.Equal(request.ConfirmedProfiles, []string{"function"}) || request.Base != nil || request.ExecutionRevision != 1 {
				t.Error("start lost fixed allocation or provider consent")
			}
			f.execution.Execution.Revision++
			f.execution.Execution.State, f.execution.OwnerState = "running", "active"
			child := &f.execution.Execution.Children[0]
			child.State, child.Directory, child.Branch, child.BaseHead = "reserved", missionExecutionUIID+"-helper", "codex/"+missionExecutionUIID+"-helper", f.repo.Head
			f.execution.ReadyChildren = nil
		default:
			t.Error("unexpected control", request.Action)
		}
		return f.execution
	}
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) })
	s.ExecuteCommand(":mission-execution")
	completeMissionExecutionAction(t, s, cl)
	if len(f.writes) != 0 || s.Mission.Execution != nil {
		t.Fatal("inspection started execution")
	}
	confirmMissionExecutionTest(t, s, "mission-prepare")
	completeMissionExecutionAction(t, s, cl)
	s.ExecuteCommand(":mission-start")
	if s.Confirmation != nil || s.Error == "" {
		t.Fatal("untrusted checks admitted")
	}
	s.Error = ""
	confirmMissionExecutionTest(t, s, "mission-trust")
	completeMissionExecutionAction(t, s, cl)
	s.ExecuteCommand(":mission-start")
	for _, token := range []string{"https://provider.example", "Creation:", "Testing:", "Review:", "go test ./...", "main.go", "Fixed mission allocation:"} {
		if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, token) {
			t.Fatal("consent omitted", token)
		}
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	completeMissionExecutionAction(t, s, cl)
	if s.Mission.Execution.Execution.State != "running" || len(f.writes) != 3 {
		t.Fatal("confirmed journey did not start exactly once", f.writes)
	}
}

func (f *missionHTTPFixture) integrate(t *testing.T, request client.MissionExecutionRequest) *client.AgentMissionExecution {
	t.Helper()
	r := &f.execution.Execution
	input := request.Integration
	if request.Action != "integrate" || input == nil || input.Path != f.candidate.WorktreePath || input.ProposalHash != f.candidate.Hash || input.Source.Head != f.childGit.Head || request.Base == nil || *request.Base != r.Current.Base || !request.ConfirmExecution || len(request.ConfirmedProfiles) != 0 {
		t.Error("integration lost exact candidate admission", request)
		return nil
	}
	before := r.Current
	f.owner.ProjectRevision = "owner-integrated"
	f.index.ProjectRevision, f.repo.ProjectRevision, f.settings.ProjectRevision, f.trust.ProjectRevision = f.owner.ProjectRevision, f.owner.ProjectRevision, f.owner.ProjectRevision, f.owner.ProjectRevision
	f.repo.Head, f.trust.Trusted = input.Source.Head, false
	f.detail.Stale, f.detail.SourceStatus = true, "changed"
	r.Current.ProjectRevision, r.Current.Base = f.owner.ProjectRevision, f.repo.GitIdentity
	r.Current.Sources = slices.Clone(before.Sources)
	for i := range r.Current.Sources {
		if r.Current.Sources[i].Path == "new.go" {
			r.Current.Sources[i].Exists, r.Current.Sources[i].Hash = true, "new-content"
		}
	}
	f.index.Files = append(slices.Clone(f.index.Files), IndexedFile{Path: "new.go", ContentHash: "new-content"})
	after := r.Current
	entry := client.MissionIntegration{ID: "integration-abcdefghijklmnopqrstuvwx", ChildID: "helper", State: "merged", Cleanup: "removed", Source: input.Source, SessionID: input.SessionID, WorkflowID: input.WorkflowID, ProposalHash: input.ProposalHash, ProposalRevision: input.ProposalRevision, Before: before, After: &after}
	entry.Evidence.Key.Session = client.AgentSessionKey{App: "mini-orca-missions", User: "local", ID: r.ID}
	entry.Evidence.Key.Name, entry.Evidence.Hash, entry.Evidence.Version = "helper-proposal.json", strings.Repeat("e", 64), 1
	r.Integrations = append(r.Integrations, entry)
	r.Children[0].State, r.State, r.Revision = "integrated", "ready", r.Revision+1
	f.execution.ReadyChildren = []string{"use"}
	result := *f.execution
	result.Git = &client.GitMutationResult{State: "merged", RemovedWorktree: input.Path, RemovedBranch: input.Source.Branch, Index: &f.index, Repository: &f.repo}
	return &result
}

func TestMissionExecutionTUIReviewsAppliesAndIntegratesMainAndTaskOwners(t *testing.T) {
	for _, task := range []bool{false, true} {
		t.Run(map[bool]string{false: "main", true: "task"}[task], func(t *testing.T) {
			s := missionExecutionTestState(task)
			missionExecutionCandidate(s)
			parent := *s.Project
			f := missionHTTPFromState(s)
			f.control = func(request client.MissionExecutionRequest) *client.AgentMissionExecution {
				return f.integrate(t, request)
			}
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) })
			s.HandleKey(KeyEvent{Key: "Enter"})
			completeMissionExecutionAction(t, s, cl)
			if s.Page != PageEditor || !s.missionCurrent() || s.Change.MissionOrigin == nil || len(f.writes) != 0 {
				t.Fatal("proposal navigation changed owner or executed")
			}
			s.ExecuteCommand(":apply")
			if len(s.PendingActions) != 0 {
				t.Fatal("Apply skipped per-file review")
			}
			s.HandleKey(KeyEvent{Key: "w"})
			if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "review-file" {
				t.Fatal("review key did not select candidate file")
			}
			action := s.PendingActions[0]
			s.PendingActions = nil
			if err := s.dispatchReviewFile(t.Context(), cl, "file-review", action); err != nil {
				t.Fatal(err)
			}
			s.ExecuteCommand(":apply")
			if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "apply-commit" {
				t.Fatal("checked review did not enable Apply", s.CmdOutput)
			}
			action = s.PendingActions[0]
			s.PendingActions = nil
			if err := s.dispatchApplyCommit(t.Context(), cl, "apply", action); err != nil {
				t.Fatal(err)
			}
			if s.Page != PagePlan || !s.Mission.ExecutionOpen || !s.missionCurrent() {
				t.Fatal("commit lost mission owner")
			}
			completeMissionExecutionAction(t, s, cl)
			confirmMissionExecutionTest(t, s, "mission-integrate")
			completeMissionExecutionAction(t, s, cl)
			if s.Mission.Execution.Execution.Children[0].State != "integrated" || s.Mission.Scope.ProjectRevision != "owner-integrated" || s.Mission.Trust != nil {
				t.Fatal("integration did not advance owner or invalidate trust")
			}
			if task && (s.Project.ProjectIdentity != parent.ProjectIdentity || s.Index.ProjectIdentity != parent.ProjectIdentity || s.Change != s.Mission.OwnerChange) {
				t.Fatal("nested integration replaced main project or lost owner selection")
			}
			if !task && s.Project.ProjectRevision != "owner-integrated" {
				t.Fatal("main integration did not advance index")
			}
			for _, path := range f.writes {
				if strings.Contains(path, "/git/merge") {
					t.Fatal("mission used generic merge")
				}
			}
			s.ExecuteCommand(":mission-execution")
			completeMissionExecutionAction(t, s, cl)
			if !s.Mission.Saved.Stale || s.Mission.Execution.Stale || s.Mission.ChildID != "helper" {
				t.Fatal("frozen plan/current execution freshness conflated")
			}
		})
	}
}

func TestMissionExecutionTUIRejectsChangedConfirmationDispatchAndCandidate(t *testing.T) {
	for name, mutate := range map[string]func(*State){
		"scope":     func(s *State) { s.Project.ProjectRevision = "changed" },
		"plan":      func(s *State) { s.Mission.Draft.Goal = "changed" },
		"execution": func(s *State) { s.Mission.Execution.Execution.Revision++ },
		"provider":  func(s *State) { s.Mission.Settings.Models[0].ProviderOrigin = "https://other.example" },
		"selection": func(s *State) { s.Mission.ChildID = "use" },
	} {
		t.Run(name, func(t *testing.T) {
			s := missionExecutionTestState(false)
			s.ExecuteCommand(":mission-start")
			if s.Confirmation == nil {
				t.Fatal(s.Error)
			}
			mutate(s)
			s.HandleKey(KeyEvent{Key: "Enter"})
			if len(s.PendingActions) != 0 || s.Error == "" {
				t.Fatal("changed authority dispatched")
			}
		})
	}
	s := missionExecutionTestState(false)
	f := missionHTTPFromState(s)
	confirmMissionExecutionTest(t, s, "mission-start")
	f.settings.Models = slices.Clone(f.settings.Models)
	f.settings.Models[0].ProviderOrigin = "https://replaced.example"
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) })
	m, command := missionTestPending(t, t.Context(), s, cl)
	m.nativeStreams = false
	m.Update(command())
	if len(f.writes) != 0 || s.Error == "" {
		t.Fatal("changed transport provider dispatched")
	}
}

func TestMissionExecutionTUIRefreshRetainsDraftAndCanceledEffectRecovery(t *testing.T) {
	s := missionExecutionTestState(false)
	missionExecutionCandidate(s)
	f := missionHTTPFromState(s)
	original := missionDigest(s.Mission.Draft)
	s.Mission.ExecutionOpen = false
	s.Mission.Draft.Goal = "unsaved local goal"
	f.owner.ProjectRevision, f.index.ProjectRevision, f.settings.ProjectRevision, f.repo.ProjectRevision, f.trust.ProjectRevision = "advanced", "advanced", "advanced", "advanced", "advanced"
	f.execution.Stale, f.execution.SourceStatus = true, "changed"
	f.detail.Stale, f.detail.SourceStatus = true, "changed"
	s.Mission.Failure = "Request canceled; outcome unknown."
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) })
	s.ExecuteCommand(":mission-execution")
	completeMissionExecutionAction(t, s, cl)
	if s.Mission.Scope.ProjectRevision != "advanced" || !s.Mission.Execution.Stale || s.Mission.Draft.Goal != "unsaved local goal" || missionDigest(s.Mission.Draft) == original || len(f.writes) != 0 {
		t.Fatal("recovery replayed work or replaced local edits")
	}
	s.ExecuteCommand(":mission-start")
	if s.Confirmation != nil {
		t.Fatal("stale source adopted for model work")
	}
}

func TestMissionExecutionTUIRetainsOnlyOwnedNestedSelection(t *testing.T) {
	s := missionExecutionTestState(true)
	proposal := missionExecutionCandidate(s)
	s.Change = proposal
	s.reconcileMissionScope()
	if s.Mission == nil {
		t.Fatal("owned nested child cleared mission")
	}
	foreign := *proposal
	foreign.WorktreePath = filepath.Join("/other", "child")
	s.Change = &foreign
	s.reconcileMissionScope()
	if s.Mission != nil {
		t.Fatal("unrelated task retained mission authority")
	}
}

func TestMissionExecutionTUIViewsFitAllPalettesAndKeepConsentAccessible(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
			s := missionExecutionTestState(false)
			missionExecutionCandidate(s)
			s.Palette = palette
			s.Mission.Execution.Execution.Children[0].Reason = "inert\x1b[2J\n" + strings.Repeat("Retained detail ", 30) + "end-of-receipt"
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
			for _, token := range []string{"MISSION EXECUTION", "end-of-receipt", "Current sources:", "Elapsed charged:", "Unstarted limits:"} {
				if !strings.Contains(visited.String(), token) {
					t.Fatal("inaccessible execution detail", token, palette, size)
				}
			}
			if strings.Contains(visited.String(), "\x1b[2J") || len(s.PendingActions) != 0 {
				t.Fatal("render executed or emitted terminal controls")
			}
			s.Mission.Execution.Execution.Children[0].BaseHead = "older-base"
			s.confirmMissionExecution("revalidate")
			if s.Confirmation == nil {
				t.Fatal(s.Error)
			}
			for range 20 {
				frame := s.Render(size[0], size[1])
				assertFrameSize(t, frame, size[0], size[1])
				if !strings.Contains(frame, "Confirm") {
					t.Fatal("confirmation action hidden")
				}
				s.HandleKey(KeyEvent{Key: "PageDown"})
			}
			s.HandleKey(KeyEvent{Key: "Escape"})
			if len(s.PendingActions) != 0 {
				t.Fatal("cancel dispatched revalidation")
			}
		}
	}
}

func TestMissionExecutionTUITransportCancellationRejectsLateReceipt(t *testing.T) {
	s := missionExecutionTestState(false)
	confirmMissionExecutionTest(t, s, "mission-start")
	action := *s.Mission.Pending
	ctx, cancel := context.WithCancel(t.Context())
	s.Mission.Cancel = cancel
	s.ExecuteCommand(":mission-cancel")
	if ctx.Err() == nil || s.Mission.Pending != nil {
		t.Fatal("transport was not canceled")
	}
	result := *s.Mission.Execution
	result.Execution.Revision++
	result.Execution.State = "running"
	s.applyMissionResult(missionResult{Action: action, Execution: &result})
	if s.Mission.Execution.Execution.State != "prepared" || s.Mission.Failure == "" {
		t.Fatal("late receipt replaced uncertain canceled state")
	}
}

func TestMissionExecutionTUIRecoveryBindsRetainedEffectsWithoutRestartingWork(t *testing.T) {
	for _, operation := range []string{"reconcile", "reconcile_integration", "reconcile_revalidation", "cleanup_integration", "pause", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			s := missionExecutionTestState(false)
			missionExecutionCandidate(s)
			v := s.Mission
			child := v.Execution.Execution.Children[0]
			v.Execution.OwnerState = "recovery_required"
			v.Settings.State = "stale"
			v.Draft.Goal = "unsubmitted local edits"
			switch operation {
			case "reconcile", "pause":
				v.Execution.Execution.State = "running"
				v.Execution.Execution.Wave = &client.MissionWave{ID: "wave", Children: []string{child.ID}, AllowanceMillis: 30000}
			case "reconcile_integration", "cleanup_integration":
				state, cleanup := "pending", "retained"
				if operation == "cleanup_integration" {
					state, cleanup, v.Execution.OwnerState = "merged", "failed", "idle"
				}
				v.Execution.Execution.Integrations = []client.MissionIntegration{{ID: "integration-retained", ChildID: child.ID, SessionID: child.SessionID, State: state, Cleanup: cleanup, Source: client.GitIdentity{Head: "observed-commit", Branch: child.Branch}}}
			case "reconcile_revalidation":
				v.Execution.Execution.Revalidations = []client.MissionRevalidation{{ID: "revalidation-retained", State: "pending", Previous: child, Source: client.GitIdentity{Head: child.BaseHead, Branch: child.Branch}}}
			}
			s.confirmMissionExecution(operation)
			if s.Confirmation == nil {
				t.Fatal("recovery blocked by irrelevant provider/draft changes", s.Error)
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			request := v.Pending.Execution
			if request.ExecutionID != v.Execution.Execution.ID || request.ExecutionRevision != v.Execution.Execution.Revision || request.PlanHash != v.Execution.Execution.Mission.PlanHash || len(request.ConfirmedProfiles) != 0 || request.CatalogRevision != 0 {
				t.Fatal("recovery lost exact identity or acquired provider authority", request)
			}
			if operation == "reconcile_revalidation" && (request.Revalidation == nil || request.Revalidation.RevalidationID != "revalidation-retained" || request.Revalidation.Previous.SessionID != child.SessionID) {
				t.Fatal("revalidation recovery lost previous candidate")
			}
			if request.Integration != nil && (request.Integration.IntegrationID != "integration-retained" || request.Integration.Path != missionCandidatePath(v.Path, child) || request.Integration.Source.Head != "observed-commit") {
				t.Fatal("integration recovery lost retained Git identity")
			}
			if (operation == "pause" || operation == "cancel" || operation == "reconcile") && (request.Base != nil || request.ConfirmExecution) {
				t.Fatal("stop or wave recovery granted execution")
			}
		})
	}
	s := missionExecutionTestState(false)
	s.Mission.Execution.OwnerState = "waiting_for_owner"
	s.confirmMissionExecution("reconcile_revalidation")
	if s.Confirmation != nil || s.Error == "" {
		t.Fatal("active lease was stolen")
	}
}

func TestMissionExecutionTUIInspectingHistoricalCandidateCannotPromoteIt(t *testing.T) {
	s := missionExecutionTestState(true)
	original := missionExecutionCandidate(s)
	v := s.Mission
	previous := v.Execution.Execution.Children[0]
	fresh := previous
	fresh.Generation, fresh.WorkflowID, fresh.SessionID = 1, "", "fresh-session"
	fresh.Directory += "-r1"
	fresh.Branch += "-r1"
	entry := client.MissionRevalidation{ID: "revalidation-abcdefghijklmnopqrstuvwx", State: "awaiting_review", Previous: previous, Candidate: fresh, Before: v.Execution.Execution.Current, Source: client.GitIdentity{Head: previous.BaseHead, Branch: previous.Branch}, AllowanceMillis: 30000}
	entry.Evidence.Key.Session = client.AgentSessionKey{App: "mini-orca-missions", User: "local", ID: missionExecutionUIID}
	entry.Evidence.Key.Name, entry.Evidence.Hash, entry.Evidence.Version = "helper-proposal.json", strings.Repeat("e", 64), 1
	v.Execution.Execution.Revalidations = []client.MissionRevalidation{entry}
	v.Execution.Execution.Children[0] = fresh
	f := missionHTTPFromState(s)
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) })
	s.ExecuteCommand(":mission-candidate 0")
	completeMissionExecutionAction(t, s, cl)
	if s.Change.ID != original.ID || s.Mission.Execution.Execution.Children[0].SessionID != "fresh-session" || !s.missionCurrent() || len(f.writes) != 0 {
		t.Fatal("history inspection promoted old authority or lost owner")
	}
	s.confirmMissionExecution("integrate")
	if s.Confirmation != nil || s.Error == "" {
		t.Fatal("historical proposal acquired integration authority")
	}
	s.Error = ""
	s.ExecuteCommand(":mission-model-history")
	completeMissionExecutionAction(t, s, cl)
	if s.Page != PageAgents || !s.AgentDetailOpen || !strings.Contains(s.CmdOutput, "Historical") {
		t.Fatal("original model history not disclosed")
	}
	s.PendingActions = nil
	s.ExecuteCommand(":retry")
	if len(s.PendingActions) != 0 || !strings.Contains(s.CmdOutput, "cannot be restarted") {
		t.Fatal("historical model allocation restarted")
	}
}

func TestMissionExecutionTUIGitReceiptCannotCleanForeignCheckout(t *testing.T) {
	s := missionExecutionTestState(false)
	missionExecutionCandidate(s)
	before := s.Project.ProjectIdentity
	result := missionResult{Action: missionAction{Parent: before, Scope: s.Mission.Scope, Path: s.Mission.Path, Execution: client.MissionExecutionRequest{Integration: &client.MissionIntegrationRequest{Path: s.Mission.Candidate.WorktreePath, Source: s.Mission.CandidateGit.GitIdentity}}}, Execution: s.Mission.Execution}
	result.Execution.Git = &client.GitMutationResult{RemovedWorktree: "/foreign", Index: &ProjectIndex{ProjectIdentity: ProjectIdentity{ProjectID: before.ProjectID, ProjectRevision: "changed"}}, Repository: s.Mission.Repository}
	if err := s.applyMissionGitReceipt(result); err == nil || s.Project.ProjectIdentity != before {
		t.Fatal("invalid cleanup mutated owner before rejection")
	}
	s.Mission = nil
	s.GitWorkspace = &GitWorkspace{Worktrees: []GitWorktree{{Path: "/repo/.worktrees/mission-run-owned-child", GitIdentity: client.GitIdentity{Branch: "codex/mission-run-owned-child"}, Mergeable: true}}}
	s.Change = nil
	s.Navigate(PageGit)
	s.mergeSelectedWorktree()
	if len(s.PendingActions) != 0 || !strings.Contains(s.CmdOutput, "mission ownership") {
		t.Fatal("uninspected mission checkout used ordinary merge")
	}
}

func TestMissionExecutionTUIRechecksCandidateBeforeGitDispatch(t *testing.T) {
	for _, field := range []string{"proposal", "commit"} {
		t.Run(field, func(t *testing.T) {
			s := missionExecutionTestState(false)
			candidate := missionExecutionCandidate(s)
			candidate.State = "applied"
			candidate.Commit = &ChangeCommit{State: "committed", OID: strings.Repeat("b", 40), Branch: s.Mission.CandidateGit.Branch}
			s.Mission.CandidateGit.Head = candidate.Commit.OID
			f := missionHTTPFromState(s)
			proposal, repository := *f.candidate, *f.childGit
			f.candidate, f.childGit = &proposal, &repository
			confirmMissionExecutionTest(t, s, "mission-integrate")
			if field == "proposal" {
				f.candidate.Hash = "changed"
			} else {
				f.childGit.Head = "moved"
			}
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) })
			m, command := missionTestPending(t, t.Context(), s, cl)
			m.nativeStreams = false
			m.Update(command())
			if len(f.writes) != 0 || s.Error == "" {
				t.Fatal("changed candidate reached Git dispatch")
			}
		})
	}
}
