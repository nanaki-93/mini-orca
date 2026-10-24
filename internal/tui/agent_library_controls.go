package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func (s *State) libraryRecordFingerprint(key libraryKey) (string, error) {
	if key.Kind == libraryArtifacts {
		item := s.libraryArtifact(key.ID)
		if item == nil {
			return "", fmt.Errorf("select a current artifact version")
		}
		return externalFingerprint(item)
	}
	item := s.libraryMemory(key.ID)
	if item == nil {
		return "", fmt.Errorf("select a current memory record")
	}
	var detail *client.AgentMemory
	if s.AgentLibrary.Memory != nil && s.AgentLibrary.Memory.ID == key.ID {
		detail = s.AgentLibrary.Memory
	}
	return externalFingerprint(struct {
		Record *client.AgentMemory
		Detail *client.AgentMemory
	}{item, detail})
}

func (s *State) libraryAdmission(action agentLibraryAction) error {
	if action.Expected != "" {
		current, err := s.libraryRecordFingerprint(action.Key)
		if err != nil || current != action.Expected || s.AgentLibrary.Failures[action.Key.Kind] != "" {
			return fmt.Errorf("library record changed; inspect and confirm its current version again")
		}
	}
	if action.Draft != nil {
		return s.libraryDraftCurrent(action.Draft)
	}
	return nil
}

func (s *State) inspectLibraryRecord() {
	key := s.selectedLibraryKey()
	if err := s.libraryReady(key.Kind); err != nil {
		s.Error = err.Error()
		return
	}
	fingerprint, err := s.libraryRecordFingerprint(key)
	if err != nil {
		s.Error = err.Error()
		return
	}
	operation := "artifact-detail"
	if key.Kind == libraryMemory {
		operation = "memory-detail"
	}
	s.DetailScroll = 0
	s.queueLibraryAction(agentLibraryAction{Operation: operation, Key: key, Expected: fingerprint})
}

func (s *State) prepareArtifactControl(action string) (agentLibraryAction, string, error) {
	key := s.selectedLibraryKey()
	if key.Kind != libraryArtifacts {
		return agentLibraryAction{}, "", fmt.Errorf("select an artifact in Code / Artifacts")
	}
	if err := s.libraryReady(key.Kind); err != nil {
		return agentLibraryAction{}, "", err
	}
	item := s.libraryArtifact(key.ID)
	if item == nil || action == "delete" && !item.CanDelete {
		return agentLibraryAction{}, "", fmt.Errorf("select an available version; protected evidence and unfinished run references cannot be deleted")
	}
	fingerprint, err := s.libraryRecordFingerprint(key)
	request := client.ArtifactControl{ProjectIdentity: s.AgentLibrary.Scope.ProjectIdentity, Action: action, Version: item.Version, Hash: item.Hash}
	message := fmt.Sprintf("%s artifact %s, version %d.\nHash: %s\nOwner: %s / %s / %s\nRecorded revision: %s (%s)\nCheckout: %s\nCurrent revision: %s", action, item.Name, item.Version, item.Hash, item.Owner.App, item.Owner.User, item.Owner.ID, item.ProjectRevision, item.RevisionStatus, s.AgentLibrary.Path, request.ProjectRevision)
	if action == "export" {
		message += "\nCreate a new private file under .mini-orca/exports/artifacts. The receipt will show the actual path and hash."
	} else {
		message += "\nRemove only this retained version. Existing exported copies remain separate. This does not edit project source or undo a proposal."
	}
	return agentLibraryAction{Operation: "artifact-control", Key: key, Expected: fingerprint, Artifact: request}, externalSafeText(message), err
}

func (s *State) confirmArtifactControl(action string) {
	request, message, err := s.prepareArtifactControl(action)
	if err != nil {
		s.Error = err.Error()
		return
	}
	parent, scope, path := s.AgentLibrary.Parent, s.AgentLibrary.Scope, s.AgentLibrary.Path
	s.Confirmation = &Confirmation{Title: "Confirm artifact " + action, Message: message, AcceptLabel: action, Destructive: action == "delete", OnConfirm: func(state *State) {
		current, _, err := state.prepareArtifactControl(action)
		if err != nil || !state.checkoutScopeCurrent(parent, scope, path) || state.AgentLibrary.Scope != scope || current.Key != request.Key || current.Expected != request.Expected {
			state.Error = "Artifact or checkout changed. Review the current version again."
			return
		}
		state.queueLibraryAction(request)
	}}
}

func (s *State) selectedMemoryDetail() (*client.AgentMemory, error) {
	key := s.selectedLibraryKey()
	if key.Kind != libraryMemory {
		return nil, fmt.Errorf("select a record in Settings / Memory")
	}
	if err := s.libraryReady(libraryMemory); err != nil {
		return nil, err
	}
	item, detail := s.libraryMemory(key.ID), s.AgentLibrary.Memory
	if item == nil || detail == nil || detail.ID != key.ID || detail.Version != item.Version {
		return nil, fmt.Errorf("press Enter to inspect the exact memory version first")
	}
	return detail, nil
}

func (s *State) prepareMemoryControl(action string) (agentLibraryAction, string, error) {
	item, err := s.selectedMemoryDetail()
	if err != nil {
		return agentLibraryAction{}, "", err
	}
	if err := libraryMemoryControlAllowed(*item, action); err != nil {
		return agentLibraryAction{}, "", err
	}
	key := libraryKey{libraryMemory, item.ID}
	fingerprint, err := s.libraryRecordFingerprint(key)
	wireAction := "forget"
	if action == "accept" {
		wireAction = "accept"
	}
	request := client.MemoryMutation{ProjectIdentity: s.AgentLibrary.Scope.ProjectIdentity, Action: wireAction, Version: item.Version}
	message := fmt.Sprintf("%s memory %s, version %d (%s).\nCheckout: %s\nCurrent revision: %s\nKind: %s\nExpires: %s\nOrigin: %s\n\n%s", action, item.Title, item.Version, item.State, s.AgentLibrary.Path, request.ProjectRevision, item.Kind, item.ExpiresAt, item.Provenance.Origin, item.Text)
	message += librarySourceMessage(item.Sources)
	if action == "accept" {
		message += "\nAccept as advisory context for future runs within this source scope until expiry. This grants no execution or source-change permission."
	} else {
		message += "\nRemove this advice from future memory retrieval. Already produced invocation history and previously sent context remain separate."
	}
	return agentLibraryAction{Operation: "memory-control", Key: key, Expected: fingerprint, Memory: request}, externalSafeText(message), err
}

func libraryMemoryControlAllowed(item client.AgentMemory, action string) error {
	switch action {
	case "accept":
		expires, err := time.Parse(time.RFC3339Nano, item.ExpiresAt)
		if err != nil || !expires.After(time.Now()) || item.State != "candidate" || item.Stale || item.Expired || item.SourceStatus != "current" {
			return fmt.Errorf("acceptance requires current, unexpired candidate advice; edit stale advice to recapture its sources")
		}
	case "reject":
		if item.State != "candidate" {
			return fmt.Errorf("reject applies to candidates; use explicit forget for accepted advice")
		}
	case "forget":
		if item.State != "accepted" {
			return fmt.Errorf("forget applies to accepted advice; use reject for a candidate")
		}
	default:
		return fmt.Errorf("unknown memory action")
	}
	return nil
}

func (s *State) confirmMemoryControl(action string) {
	request, message, err := s.prepareMemoryControl(action)
	if err != nil {
		s.Error = err.Error()
		return
	}
	parent, scope, path := s.AgentLibrary.Parent, s.AgentLibrary.Scope, s.AgentLibrary.Path
	s.Confirmation = &Confirmation{Title: "Confirm memory " + action, Message: message, AcceptLabel: action, Destructive: action != "accept", OnConfirm: func(state *State) {
		current, _, err := state.prepareMemoryControl(action)
		if err != nil || !state.checkoutScopeCurrent(parent, scope, path) || state.AgentLibrary.Scope != scope || current.Key != request.Key || current.Expected != request.Expected || current.Memory != request.Memory {
			state.Error = "Memory, source status or checkout changed. Inspect the current version again."
			return
		}
		state.queueLibraryAction(request)
	}}
}

func (s *State) openLibrarySource(argument string) {
	item, err := s.selectedMemoryDetail()
	if err != nil {
		s.Error = err.Error()
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(argument))
	if err != nil || n < 1 || n > len(item.Sources) {
		s.Error = "Use :memory-source <number> from the selected record's source list."
		return
	}
	key := libraryKey{libraryMemory, item.ID}
	fingerprint, err := s.libraryRecordFingerprint(key)
	if err != nil {
		s.Error = err.Error()
		return
	}
	s.queueLibraryAction(agentLibraryAction{Operation: "source", Key: key, Expected: fingerprint, Source: item.Sources[n-1]})
}

func (s *State) openLibraryReport() {
	key := s.selectedLibraryKey()
	if err := s.libraryReady(libraryArtifacts); err != nil {
		s.Error = err.Error()
		return
	}
	detail := s.AgentLibrary.Artifact
	if key.Kind != libraryArtifacts || detail == nil || detail.ID != key.ID || detail.PreviewKind != "research_report" {
		s.Error = "Inspect a retained research artifact first."
		return
	}
	fingerprint, err := s.libraryRecordFingerprint(key)
	if err != nil {
		s.Error = err.Error()
		return
	}
	s.queueLibraryAction(agentLibraryAction{Operation: "report", Key: key, Expected: fingerprint, ReportID: detail.ReportID})
}
