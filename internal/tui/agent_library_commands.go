package tui

import "strings"

func (s *State) execLibraryCommand(command, arg string) bool {
	switch command {
	case "library-scope":
		s.selectLibraryScope(strings.TrimSpace(arg))
	case "library-refresh":
		s.refreshLibrary()
	case "library-cancel":
		s.cancelLibraryRequest()
	case "library-inspect":
		s.inspectLibraryRecord()
	case "library-hide":
		s.hideLibraryDisclosure()
	case "artifact-next", "artifact-first":
		s.pageLibraryArtifacts(command == "artifact-next")
	case "artifact-export", "artifact-delete":
		s.confirmArtifactControl(strings.TrimPrefix(command, "artifact-"))
	case "artifact-report":
		s.openLibraryReport()
	default:
		return s.execMemoryCommand(command, arg)
	}
	return true
}

func (s *State) execMemoryCommand(command, arg string) bool {
	switch command {
	case "memory-create", "memory-edit":
		s.captureLibraryAdvice(strings.TrimPrefix(command, "memory-"), arg)
	case "memory-accept", "memory-reject", "memory-forget":
		s.confirmMemoryControl(strings.TrimPrefix(command, "memory-"))
	case "memory-retain":
		s.retainLibraryProposal(arg)
	case "memory-source":
		s.openLibrarySource(arg)
	default:
		return false
	}
	return true
}

func (s *State) hideLibraryDisclosure() {
	if s.AgentLibrary != nil {
		s.AgentLibrary.Artifact, s.AgentLibrary.Memory, s.AgentLibrary.Source = nil, nil, nil
	}
	s.DetailScroll = 0
	s.CmdOutput = "Private library previews hidden."
}

func (s *State) handleLibraryKey(key string) bool {
	switch key {
	case "r":
		s.refreshLibrary()
	case "x":
		s.cancelLibraryRequest()
	case "o":
		s.openCommandInput("library-scope ")
	case "Escape":
		s.hideLibraryDisclosure()
	default:
		if s.EffectivePage() == PageArtifacts {
			return s.handleArtifactKey(key)
		}
		return s.handleMemoryKey(key)
	}
	return true
}

func (s *State) handleArtifactKey(key string) bool {
	switch key {
	case "e":
		s.confirmArtifactControl("export")
	case "d":
		s.confirmArtifactControl("delete")
	case "n":
		s.pageLibraryArtifacts(true)
	case "N":
		s.pageLibraryArtifacts(false)
	case "w":
		s.openLibraryReport()
	default:
		return false
	}
	return true
}

func (s *State) handleMemoryKey(key string) bool {
	switch key {
	case "n":
		s.openCommandInput("memory-create decision 30 Title | ")
	case "e":
		if item, err := s.selectedMemoryDetail(); err != nil {
			s.Error = err.Error()
		} else {
			s.openCommandInput("memory-edit decision 30 " + item.Title + " | ")
		}
	case "a":
		s.confirmMemoryControl("accept")
	case "d":
		s.confirmMemoryControl("reject")
	case "f":
		s.confirmMemoryControl("forget")
	case "s":
		s.openCommandInput("memory-source ")
	case "t":
		s.openCommandInput("memory-retain ")
	default:
		return false
	}
	return true
}

func (s *State) pageLibraryArtifacts(next bool) {
	if err := s.libraryReady(libraryArtifacts); err != nil {
		s.Error = err.Error()
		return
	}
	page := s.AgentLibrary.Artifacts
	action := agentLibraryAction{Operation: "artifact-page", Key: libraryKey{Kind: libraryArtifacts}}
	if next {
		if page == nil || page.NextCursor == "" {
			s.CmdOutput = "No next artifact page. Refresh to read the current catalog."
			return
		}
		action.Cursor, action.Snapshot = page.NextCursor, page.Snapshot
	}
	s.queueLibraryAction(action)
}
