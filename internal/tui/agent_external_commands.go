package tui

import (
	"strings"
	"time"
)

func (s *State) execExternalCommand(command, arg string) bool {
	switch command {
	case "external-scope":
		s.selectExternalScope(strings.TrimSpace(arg))
	case "external-refresh":
		s.refreshExternalWork()
	case "external-cancel":
		s.cancelExternalRequest()
	case "external-inspect":
		s.inspectExternalReceipt()
	case "api-approve":
		s.confirmAPIAction("approve", "", "")
	case "api-reconcile":
		outcome, observation, _ := strings.Cut(strings.TrimSpace(arg), " ")
		s.confirmAPIAction("reconcile", outcome, strings.TrimSpace(observation))
	case "research":
		question, only := externalQuestionArgument(arg)
		s.captureExternalWork(externalDraft{Mode: "research", Question: question, QuestionOnly: only})
	case "delegate":
		s.startExternalDelegation(arg)
	case "report-open":
		s.openSelectedResearchReport()
	default:
		return s.execRemoteControlCommand(command, arg)
	}
	return true
}

func (s *State) execRemoteControlCommand(command, arg string) bool {
	switch command {
	case "remote-refresh", "remote-reconnect", "remote-cancel", "remote-input", "remote-reconcile":
		s.confirmRemoteControl(strings.TrimPrefix(command, "remote-"), strings.TrimSpace(arg))
	default:
		return false
	}
	return true
}

func externalQuestionArgument(arg string) (string, bool) {
	text := strings.TrimSpace(arg)
	if strings.HasPrefix(text, "--question-only ") {
		return strings.TrimSpace(strings.TrimPrefix(text, "--question-only ")), true
	}
	return text, false
}

func (s *State) startExternalDelegation(arg string) {
	parts := strings.SplitN(strings.TrimSpace(arg), " ", 4)
	if len(parts) != 4 {
		s.Error = "Use :delegate <remote-id> <analysis|review> <skill> [--question-only] <question>."
		return
	}
	question, only := externalQuestionArgument(parts[3])
	d := externalDraft{Mode: "delegate", Kind: parts[1], Skill: parts[2], Question: question, QuestionOnly: only}
	d.Remote.Config.ID = parts[0]
	s.captureExternalWork(d)
}

func (s *State) handleExternalKey(key string) bool {
	switch key {
	case "r":
		s.refreshExternalWork()
	case "x":
		s.cancelExternalRequest()
	case "a":
		s.confirmAPIAction("approve", "", "")
	case "n":
		s.openCommandInput("research ")
	case "d":
		s.openCommandInput("delegate ")
	case "o":
		s.openCommandInput("external-scope ")
	case "i":
		if item := s.selectedRemoteForControl("input"); item != nil {
			s.openCommandInput("remote-input ")
		}
	case "u":
		s.openExternalReconciliation()
	default:
		return false
	}
	return true
}

func (s *State) selectedRemoteForControl(operation string) *externalKey {
	key, err := s.selectedExternal(externalRemote)
	if err != nil {
		s.Error = err.Error()
		return nil
	}
	item := s.remoteReceipt(key.ID)
	if item == nil || !remoteControlAllowed(*item, operation, time.Now()) {
		s.Error = "Inspect a current receipt that permits this control first."
		return nil
	}
	return &key
}

func (s *State) openExternalReconciliation() {
	key := s.selectedExternalKey()
	if key.Kind == externalRemote {
		if s.selectedRemoteForControl("reconcile") != nil {
			s.openCommandInput("remote-reconcile ")
		}
		return
	}
	if _, err := s.selectedExternal(externalAPI); err != nil {
		s.Error = err.Error()
		return
	}
	item := s.apiReceipt(key.ID)
	if item == nil || !item.CanReconcile {
		s.Error = "Only an uncertain API receipt permits reconciliation."
		return
	}
	s.openCommandInput("api-reconcile ")
}

func (s *State) inspectExternalReceipt() {
	key := s.selectedExternalKey()
	if _, err := s.selectedExternal(key.Kind); err != nil {
		s.Error = err.Error()
		return
	}
	s.DetailScroll = 0
	switch key.Kind {
	case externalAPI:
		item := s.apiReceipt(key.ID)
		if item == nil {
			s.Error = "API receipt unavailable."
			return
		}
		s.AgentExternal.Reviewed = externalReview{ID: item.ID, Version: item.Version, Hash: item.RequestHash}
		s.CmdOutput = "Review the exact request and hash with PgUp/PgDn. Use :api-approve for a separate dispatch confirmation."
	case externalRemote:
		s.queueExternalAction(agentExternalAction{Operation: "remote-detail", Key: key})
	case externalResearch:
		s.openSelectedResearchReport()
	}
}

func (s *State) openSelectedResearchReport() {
	key, err := s.selectedExternal(externalResearch)
	if err != nil {
		s.Error = err.Error()
		return
	}
	s.queueExternalAction(agentExternalAction{Operation: "report", Key: key})
}
