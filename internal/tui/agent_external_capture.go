package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type externalDraft struct {
	Mode, Question, Kind, Skill string
	QuestionOnly                bool
	Paths                       []string
	Unshared                    []string
	PlanFingerprint             string
	Sources                     []client.ExternalSource
	Model                       Model
	Remote                      client.AgentConnection
}

func externalFingerprint(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("capture external work identity: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func externalTextInput(text string) error {
	if strings.TrimSpace(text) == "" || len(text) > 2048 || !utf8.ValidString(text) {
		return fmt.Errorf("provide nonempty text up to 2048 UTF-8 bytes")
	}
	for _, char := range text {
		if unicode.IsControl(char) && char != '\n' && char != '\t' {
			return fmt.Errorf("input contains unsupported control characters")
		}
	}
	return nil
}

func (s *State) externalReady(kind string) error {
	if !s.externalCurrent() {
		return fmt.Errorf("load External Work for the current checkout first")
	}
	v := s.AgentExternal
	if !v.Loaded[kind] || v.Failures[kind] != "" {
		return fmt.Errorf("refresh local external-work history before acting")
	}
	return nil
}

func (s *State) captureExternalWork(d externalDraft) {
	if err := s.prepareExternalDraft(&d); err != nil {
		s.Error = err.Error()
		return
	}
	s.Navigate(PageExternalWork)
	s.queueExternalAction(agentExternalAction{Operation: "capture", Key: externalKey{Kind: externalDraftGroup(d)}, Draft: &d})
	s.CmdOutput = "Reading the local index to capture source hashes. External sharing requires a separate confirmation."
}

func externalDraftGroup(d externalDraft) string {
	if d.Mode == "research" {
		return externalResearch
	}
	return externalRemote
}

func (s *State) prepareExternalDraft(d *externalDraft) error {
	if err := s.externalReady(externalDraftGroup(*d)); err != nil {
		return err
	}
	if s.AgentExternal.Pending != "" {
		return fmt.Errorf("finish or cancel the pending external request first")
	}
	if err := externalTextInput(d.Question); err != nil {
		return err
	}
	if err := s.captureExternalPlanScope(d); err != nil {
		return err
	}
	if d.Mode == "research" {
		model, err := s.externalResearchModel()
		if err != nil {
			return err
		}
		d.Model = model
		return nil
	}
	if d.Mode != "delegate" || d.Kind != "analysis" && d.Kind != "review" {
		return fmt.Errorf("remote delegation requires explicit analysis or review intent")
	}
	remote, err := s.externalRemoteConnection(d.Remote.Config.ID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(remote.Inventory.Skills, func(skill client.ConnectionSkill) bool { return skill.ID == d.Skill }) {
		return fmt.Errorf("select an advertised, allowlisted remote skill from Connections")
	}
	data, err := json.Marshal(remote)
	if err != nil {
		return fmt.Errorf("capture remote card: %w", err)
	}
	return json.Unmarshal(data, &d.Remote)
}

func (s *State) captureExternalPlanScope(d *externalDraft) error {
	if d.QuestionOnly {
		return nil
	}
	plan, v := s.PlanDraft, s.AgentExternal
	if plan == nil || plan.Seed.Plan == nil || plan.Source != v.Parent || plan.Seed.WorktreePath != v.Scope.WorktreePath {
		return fmt.Errorf("open a plan in this checkout, or choose --question-only explicitly")
	}
	d.Paths = append(slices.Clone(plan.Seed.Paths), plan.Seed.Plan.ReadPaths...)
	if len(d.Paths) == 0 || len(d.Paths) > 8 {
		return fmt.Errorf("external work requires one to eight selected plan paths, or --question-only")
	}
	d.PlanFingerprint = planDraftFingerprint(plan)
	return nil
}

func (s *State) externalResearchModel() (Model, error) {
	model := s.workflowBoundModel("code")
	view := s.AgentExternal.Research
	if view == nil || view.State != "available" || model.Model != view.Model || model.ProviderOrigin == "" || !model.Capabilities.Grounding {
		return Model{}, fmt.Errorf("research requires available Google Search and current Code model metadata with grounding support")
	}
	// Pointer-valued generation parameters are part of the captured binding;
	// later metadata edits must not mutate the confirmation's snapshot.
	if model.Temperature != nil {
		value := *model.Temperature
		model.Temperature = &value
	}
	if model.MaxTokens != nil {
		value := *model.MaxTokens
		model.MaxTokens = &value
	}
	return model, nil
}

func (s *State) externalRemoteConnection(id string) (*client.AgentConnection, error) {
	v := s.AgentConnections
	if !s.connectionsCurrent() || v.Scope != s.AgentExternal.Scope || v.Pending != "" || v.Failures[client.ConnectionRemote].State != "" {
		return nil, fmt.Errorf("load and discover remote connections in the same checkout first")
	}
	for _, row := range s.connectionRows() {
		if row.Key.Kind != client.ConnectionRemote || row.Key.ID != id {
			continue
		}
		if row.State != "ready" || row.Data.Inventory == nil || row.Data.Inventory.Fingerprint == "" {
			return nil, fmt.Errorf("remote card grant is unavailable or expired; discover it again")
		}
		return &row.Data, nil
	}
	return nil, fmt.Errorf("remote connection is not configured")
}

func (s *State) externalDraftCurrent(d *externalDraft) error {
	if d == nil {
		return fmt.Errorf("captured external input is unavailable")
	}
	if err := s.externalReady(externalDraftGroup(*d)); err != nil {
		return err
	}
	if !d.QuestionOnly && planDraftFingerprint(s.PlanDraft) != d.PlanFingerprint {
		return fmt.Errorf("plan changed; capture and confirm its current sources again")
	}
	var current, captured any
	if d.Mode == "research" {
		model, err := s.externalResearchModel()
		if err != nil {
			return err
		}
		current, captured = model, d.Model
	} else {
		remote, err := s.externalRemoteConnection(d.Remote.Config.ID)
		if err != nil {
			return err
		}
		current, captured = remote, d.Remote
	}
	a, err := externalFingerprint(current)
	if err != nil {
		return err
	}
	b, err := externalFingerprint(captured)
	if err != nil {
		return err
	}
	if a != b {
		return fmt.Errorf("provider or remote grant changed; capture and confirm the current destination again")
	}
	return nil
}

func externalCapturedSources(paths []string, index *ProjectIndex) ([]client.ExternalSource, []string, error) {
	if index == nil {
		return nil, nil, fmt.Errorf("source index unavailable")
	}
	var result []client.ExternalSource
	var unshared []string
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			return nil, nil, fmt.Errorf("external scope repeats a source path")
		}
		seen[path] = true
		found := false
		for _, file := range index.Files {
			if file.Path == path && file.ContentHash != "" {
				result = append(result, client.ExternalSource{Path: path, Hash: file.ContentHash})
				found = true
				break
			}
		}
		if !found {
			unshared = append(unshared, path)
		}
	}
	if len(paths) > 0 && len(result) == 0 {
		return nil, nil, fmt.Errorf("the plan has no current indexed sources; select existing paths or --question-only explicitly")
	}
	return result, unshared, nil
}

func (s *State) confirmCapturedExternalWork(r agentExternalResult) {
	d := r.Action.Draft
	if err := s.externalDraftCurrent(d); err != nil {
		s.Error = err.Error()
		return
	}
	if r.Index == nil || r.Index.ProjectIdentity != r.Action.Scope.ProjectIdentity {
		s.Error = "Captured index belongs to another checkout revision."
		return
	}
	sources, unshared, err := externalCapturedSources(d.Paths, r.Index)
	if err != nil {
		s.Error = err.Error()
		return
	}
	captured := *d
	captured.Sources = sources
	captured.Unshared = unshared
	message := externalDraftConsent(captured, s.AgentExternal.Path)
	s.Confirmation = &Confirmation{Title: "Confirm external work", AcceptLabel: "Send once", Message: message, OnConfirm: func(state *State) {
		if !state.externalCurrent() || state.AgentExternal.Scope != r.Action.Scope || state.AgentExternal.Parent != r.Action.Parent {
			state.Error = "Checkout changed. Capture the current scope again."
			return
		}
		if err := state.externalDraftCurrent(&captured); err != nil {
			state.Error = err.Error()
			return
		}
		state.queueExternalAction(agentExternalAction{Operation: captured.Mode, Key: externalKey{Kind: externalDraftGroup(captured)}, Draft: &captured})
	}}
}

func externalDraftConsent(d externalDraft, path string) string {
	message := "Checkout: " + theme.SanitizeCodeText(path) + "\nQuestion: " + theme.SanitizeCodeText(d.Question)
	if d.Mode == "research" {
		message += "\nSend to " + settingsText(d.Model.Model+" at "+d.Model.ProviderOrigin) + " with Google Search. Confirm provider use, search and sharing the captured question and source excerpts."
	} else {
		message += "\nDelegate " + d.Kind + " to " + settingsText(d.Remote.Config.Endpoint) + " using skill " + settingsText(d.Skill) + ". Confirm sharing the captured question and source excerpts."
		if d.Remote.Inventory.Streaming {
			message += " Stream the advertised remote task response."
		}
	}
	if len(d.Sources) == 0 {
		message += "\nQuestion only: no project source files are shared."
	}
	for _, source := range d.Sources {
		message += "\n" + theme.SanitizeCodeText(source.Path) + " · " + source.Hash
	}
	for _, path := range d.Unshared {
		message += "\nNot shared (no indexed source): " + theme.SanitizeCodeText(path)
	}
	return message + "\nResults are advisory. Ambient memory, skills and history are excluded. No project source writes or execution are authorized."
}

func externalRemoteUse(remote client.AgentConnection, sharing bool) client.IntegrationUse {
	return client.IntegrationUse{ID: remote.Config.ID, Grant: remote.Grant, SchemaFingerprint: remote.Inventory.Fingerprint, ConfirmSourceSharing: sharing}
}

func (s *State) externalAdmission(action agentExternalAction) error {
	switch action.Operation {
	case "capture", "research", "delegate":
		return s.externalDraftCurrent(action.Draft)
	case "api-control", "remote-control":
		current, err := s.externalControlIdentity(action)
		if err != nil {
			return err
		}
		if current != action.Expected {
			return fmt.Errorf("receipt, destination or grant changed; review the current action again")
		}
	}
	return nil
}
