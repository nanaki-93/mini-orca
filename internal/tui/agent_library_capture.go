package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type libraryDraft struct {
	Key                             libraryKey
	Mutation                        client.MemoryMutation
	ContextKind, ContextFingerprint string
	PlanPaths                       []string
	Unshared                        []string
	DisplayText                     string
}

func parseLibraryDraft(argument string) (*client.MemoryDraft, error) {
	header, text, found := strings.Cut(argument, "|")
	parts := strings.SplitN(strings.TrimSpace(header), " ", 3)
	if !found || len(parts) != 3 {
		return nil, fmt.Errorf("use <decision|lesson> <1–365 days> <title> | <advice text>")
	}
	days, err := strconv.Atoi(parts[1])
	draft := &client.MemoryDraft{Kind: parts[0], RetentionDays: days, Title: strings.TrimSpace(parts[2]), Text: strings.TrimSpace(text)}
	if err != nil || days < 1 || days > 365 || !slices.Contains([]string{"decision", "lesson"}, draft.Kind) {
		return nil, fmt.Errorf("manual memory requires decision or lesson and 1–365 retention days")
	}
	if err := validateLibraryDraftText(draft); err != nil {
		return nil, err
	}
	return draft, nil
}

func validateLibraryDraftText(draft *client.MemoryDraft) error {
	if err := externalTextInput(draft.Title); err != nil || len(draft.Title) > 120 || strings.IndexFunc(draft.Title, unicode.IsControl) >= 0 {
		return fmt.Errorf("provide a single-line title of 1–120 UTF-8 bytes")
	}
	if draft.SessionID == "" {
		return externalTextInput(draft.Text)
	}
	return nil
}

func (s *State) captureLibraryAdvice(action, argument string) {
	if err := s.libraryReady(libraryMemory); err != nil {
		s.Error = err.Error()
		return
	}
	template, err := parseLibraryDraft(argument)
	if err != nil {
		s.Error = err.Error()
		return
	}
	d := &libraryDraft{Key: libraryKey{Kind: libraryMemory}, Mutation: client.MemoryMutation{ProjectIdentity: s.AgentLibrary.Scope.ProjectIdentity, Action: action, Draft: template}, DisplayText: template.Text}
	if action == "edit" {
		item, err := s.selectedMemoryDetail()
		if err != nil {
			s.Error = err.Error()
			return
		}
		d.Key.ID, d.Mutation.Version, d.ContextKind = item.ID, item.Version, "edit"
		template.Sources = slices.Clone(item.Sources)
	} else if err := s.libraryAdviceSources(d); err != nil {
		s.Error = err.Error()
		return
	}
	s.startLibraryCapture(d)
}

func (s *State) libraryAdviceSources(d *libraryDraft) error {
	v, plan := s.AgentLibrary, s.PlanDraft
	if plan != nil && plan.Seed.Plan != nil && plan.Source == v.Parent && plan.Seed.WorktreePath == v.Scope.WorktreePath {
		d.ContextKind = "plan"
		d.PlanPaths = append(slices.Clone(plan.Seed.Paths), plan.Seed.Plan.ReadPaths...)
		if len(d.PlanPaths) < 1 || len(d.PlanPaths) > 8 {
			return fmt.Errorf("select one to eight plan paths before retaining advice")
		}
		for _, path := range d.PlanPaths {
			d.Mutation.Draft.Sources = append(d.Mutation.Draft.Sources, client.MemorySource{Path: path})
		}
		return nil
	}
	if v.Scope.WorktreePath != "" || s.File == nil || s.File.Path == "" {
		return fmt.Errorf("open a plan in this checkout, or open a main-checkout file before retaining advice")
	}
	d.ContextKind = "file"
	d.Mutation.Draft.Sources = []client.MemorySource{{Path: s.File.Path}}
	return nil
}

func (s *State) libraryContextFingerprint(d *libraryDraft) (string, error) {
	switch d.ContextKind {
	case "plan":
		if s.PlanDraft != nil && s.PlanDraft.Source == s.AgentLibrary.Parent && s.PlanDraft.Seed.WorktreePath == s.AgentLibrary.Scope.WorktreePath {
			return planDraftFingerprint(s.PlanDraft), nil
		}
	case "file":
		if s.File != nil && s.AgentLibrary.Scope.WorktreePath == "" {
			return externalFingerprint(struct{ Path, Hash string }{s.File.Path, s.File.ContentHash})
		}
	case "edit":
		return s.libraryRecordFingerprint(d.Key)
	case "proposal":
		if s.Change != nil && s.Change.ProjectIdentity == s.AgentLibrary.Scope.ProjectIdentity && s.Change.WorktreePath == s.AgentLibrary.Scope.WorktreePath {
			return externalFingerprint(struct {
				ID, Hash string
				Revision int
				Checks   []Check
				Workflow *ChangeWorkflow
			}{s.Change.ID, s.Change.Hash, s.Change.Revision, s.Change.Checks, s.Change.Workflow})
		}
	}
	return "", fmt.Errorf("captured advice source or proposal is no longer selected")
}

func (s *State) libraryDraftCurrent(d *libraryDraft) error {
	if d == nil || !s.libraryCurrent() || d.Mutation.ProjectIdentity != s.AgentLibrary.Scope.ProjectIdentity || s.AgentLibrary.Failures[libraryMemory] != "" {
		return fmt.Errorf("captured advice belongs to another checkout")
	}
	current, err := s.libraryContextFingerprint(d)
	if err != nil || current != d.ContextFingerprint {
		return fmt.Errorf("advice source, record or proposal changed; capture and review it again")
	}
	return nil
}

func (s *State) startLibraryCapture(d *libraryDraft) {
	fingerprint, err := s.libraryContextFingerprint(d)
	if err != nil {
		s.Error = err.Error()
		return
	}
	d.ContextFingerprint = fingerprint
	s.Navigate(PageMemory)
	s.queueLibraryAction(agentLibraryAction{Operation: "capture", Key: d.Key, Draft: d})
	s.CmdOutput = "Reading current source hashes. Retaining advice requires a separate confirmation."
}

func (s *State) finishLibraryCapture(r agentLibraryResult) {
	d := r.Action.Draft
	if s.EffectivePage() != PageMemory || d == nil || r.Index == nil || r.Index.ProjectIdentity != r.Action.Scope.ProjectIdentity {
		s.Error = "Advice capture is no longer active. Capture the current selection again."
		return
	}
	if err := captureLibrarySources(d, r.Index); err != nil {
		s.Error = err.Error()
		return
	}
	message := fmt.Sprintf("%s candidate advice: %s\nKind: %s\nRetention: %d days\nCheckout: %s\nCurrent revision: %s\n\n%s", d.Mutation.Action, d.Mutation.Draft.Title, d.Mutation.Draft.Kind, d.Mutation.Draft.RetentionDays, s.AgentLibrary.Path, d.Mutation.ProjectRevision, d.DisplayText)
	message += librarySourceMessage(d.Mutation.Draft.Sources)
	if len(d.Unshared) > 0 {
		message += "\nUncaptured planned paths (no indexed text): " + strings.Join(d.Unshared, ", ")
	}
	if d.Mutation.Draft.Identity != nil {
		message += "\nHost-derived receipt for proposal: " + d.Mutation.Draft.Identity.Hash
	}
	message += "\nStore as a candidate. Editing resets acceptance; accepting this advice remains a separate action."
	parent, scope, path := s.AgentLibrary.Parent, s.AgentLibrary.Scope, s.AgentLibrary.Path
	s.Confirmation = &Confirmation{Title: "Retain candidate memory", Message: externalSafeText(message), AcceptLabel: "Retain candidate", OnConfirm: func(state *State) {
		if !state.checkoutScopeCurrent(parent, scope, path) || !state.libraryCurrent() || state.AgentLibrary.Scope != scope {
			state.Error = "Library checkout changed. Capture the current scope again."
			return
		}
		if err := state.libraryDraftCurrent(d); err != nil {
			state.Error = err.Error()
			return
		}
		state.queueLibraryAction(agentLibraryAction{Operation: "memory-control", Key: d.Key, Memory: d.Mutation, Draft: d})
	}}
}

func captureLibrarySources(d *libraryDraft, index *ProjectIndex) error {
	files := map[string]IndexedFile{}
	for _, file := range index.Files {
		files[file.Path] = file
	}
	var captured []client.MemorySource
	seen := map[string]bool{}
	for _, source := range d.Mutation.Draft.Sources {
		if seen[source.Path] {
			continue
		}
		seen[source.Path] = true
		file, ok := files[source.Path]
		if !ok || file.Binary || file.ContentHash == "" {
			if d.ContextKind != "plan" {
				return fmt.Errorf("source %s has no current indexed text", source.Path)
			}
			d.Unshared = append(d.Unshared, source.Path)
			continue
		}
		if d.ContextKind == "proposal" && source.Hash != file.ContentHash || source.EndLine > file.LineCount {
			return fmt.Errorf("source %s no longer matches the retained proposal or line range", source.Path)
		}
		source.Hash = file.ContentHash
		if err := source.Validate(); err != nil {
			return fmt.Errorf("capture %s: %w", source.Path, err)
		}
		captured = append(captured, source)
	}
	if len(captured) < 1 || len(captured) > 64 {
		return fmt.Errorf("memory requires one to 64 captured current sources")
	}
	d.Mutation.Draft.Sources = captured
	return nil
}

func librarySourceMessage(sources []client.MemorySource) string {
	var message strings.Builder
	for _, source := range sources {
		span := "whole file"
		if source.StartLine > 0 {
			span = fmt.Sprintf("lines %d–%d", source.StartLine, source.EndLine)
		}
		fmt.Fprintf(&message, "\nSource: %s (%s)\nHash: %s", source.Path, span, source.Hash)
	}
	return message.String()
}

func (s *State) retainLibraryProposal(argument string) {
	if err := s.libraryReady(libraryMemory); err != nil {
		s.Error = err.Error()
		return
	}
	draft, text, err := s.libraryProposalDraft(argument)
	if err != nil {
		s.Error = err.Error()
		return
	}
	s.startLibraryCapture(&libraryDraft{Key: libraryKey{Kind: libraryMemory}, ContextKind: "proposal", DisplayText: text, Mutation: client.MemoryMutation{ProjectIdentity: s.AgentLibrary.Scope.ProjectIdentity, Action: "create", Draft: draft}})
}

func (s *State) libraryProposalDraft(argument string) (*client.MemoryDraft, string, error) {
	chg := s.Change
	if chg == nil || chg.ProjectIdentity != s.AgentLibrary.Scope.ProjectIdentity || chg.WorktreePath != s.AgentLibrary.Scope.WorktreePath || chg.ID == "" || chg.Hash == "" {
		return nil, "", fmt.Errorf("select the exact proposal in this library checkout first")
	}
	parts := strings.SplitN(strings.TrimSpace(argument), " ", 3)
	if len(parts) != 3 {
		return nil, "", fmt.Errorf("use :memory-retain outcome|lesson:<number> <days> <title>")
	}
	days, err := strconv.Atoi(parts[1])
	if err != nil || days < 1 || days > 365 {
		return nil, "", fmt.Errorf("retention must be 1–365 days")
	}
	draft := &client.MemoryDraft{RetentionDays: days, Title: strings.TrimSpace(parts[2]), SessionID: chg.ID, Identity: &client.LibraryProposalIdentity{ProjectIdentity: chg.ProjectIdentity, Revision: int64(chg.Revision), Hash: chg.Hash}}
	for _, target := range chg.Targets {
		if target.Exists {
			draft.Sources = append(draft.Sources, client.MemorySource{Path: target.Path, Hash: target.Hash})
		}
	}
	if err := validateLibraryDraftText(draft); err != nil {
		return nil, "", err
	}
	text, err := libraryProposalText(chg, parts[0], draft)
	return draft, text, err
}

func libraryProposalText(chg *ChangeSession, selection string, draft *client.MemoryDraft) (string, error) {
	if selection == "outcome" {
		draft.Kind = "verified_outcome"
		var passed []string
		for _, check := range chg.Checks {
			if check.State == "passed" {
				passed = append(passed, check.Name)
			}
		}
		if len(passed) == 0 {
			return "", fmt.Errorf("the selected proposal has no passing check receipts")
		}
		return "Retain the host's check outcome for this exact proposal. Passing checks: " + strings.Join(passed, ", ") + ". The daemon rechecks current check authority and derives the saved text.", nil
	}
	n, err := strconv.Atoi(strings.TrimPrefix(selection, "lesson:"))
	if !strings.HasPrefix(selection, "lesson:") || err != nil || chg.Workflow == nil || n < 1 || n > len(chg.Workflow.HarnessLessons) {
		return "", fmt.Errorf("select outcome or lesson:<number> from the current proposal")
	}
	index := n - 1
	draft.Kind, draft.LessonIndex = "lesson", &index
	return chg.Workflow.HarnessLessons[index], nil
}
