package app

import (
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestDraftIdentityValuesRejectPartialValuesAndCompareTheFullTarget(t *testing.T) {
	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "project", call: func() error { _, err := newProjectSnapshot("", "revision"); return err }},
		{name: "file", call: func() error { _, err := newSelectedFile("main.go", ""); return err }},
		{name: "target", call: func() error { _, err := newTaskTarget("project", "revision", "main.go", "hash", "", "Run"); return err }},
		{name: "draft", call: func() error { _, err := newDraftRevisionIdentity("draft", 0, "hash"); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("expected identity validation error")
			}
		})
	}

	draft := Draft{ID: "draft", ProjectID: "project", ProjectRevision: "revision", BaseFileHash: "base", TargetPath: "main.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", Revision: 2, Hash: "draft-hash"}
	target, err := taskTargetForDraft(draft)
	if err != nil || !target.matchesDraft(draft) {
		t.Fatalf("target = %+v, err = %v", target, err)
	}
	for _, changed := range []Draft{
		func() Draft { copy := draft; copy.ProjectRevision = "later"; return copy }(),
		func() Draft { copy := draft; copy.BaseFileHash = "other"; return copy }(),
		func() Draft { copy := draft; copy.TargetPath = "other.go"; return copy }(),
		func() Draft { copy := draft; copy.TargetSymbol = "Other"; return copy }(),
	} {
		if target.matchesDraft(changed) {
			t.Fatalf("target unexpectedly matched %+v", changed)
		}
	}

	identity, err := newApplyIdentity(ApplyRequest{ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, BaseFileHash: draft.BaseFileHash, DraftID: draft.ID, DraftRevision: draft.Revision, DraftHash: draft.Hash, Confirm: true})
	if err != nil || !identity.matchesDraft(draft) {
		t.Fatalf("apply identity = %+v, err = %v", identity, err)
	}
	for _, changed := range []Draft{
		func() Draft { copy := draft; copy.ProjectID = "other-project"; return copy }(),
		func() Draft { copy := draft; copy.BaseFileHash = "other-hash"; return copy }(),
		func() Draft { copy := draft; copy.Revision++; return copy }(),
		func() Draft { copy := draft; copy.Hash = "other-draft-hash"; return copy }(),
	} {
		if identity.matchesDraft(changed) {
			t.Fatalf("apply identity unexpectedly matched %+v", changed)
		}
	}
}
