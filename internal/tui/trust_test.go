package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrustPersistenceFailureAndProjectSwitchDoNotClaimSuccess(t *testing.T) {
	for _, changed := range []bool{false, true} {
		s := NewState()
		s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			http.Error(w, "cannot save execution trust settings", http.StatusBadRequest)
		}))
		cl, err := NewClient(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		s.ExecuteCommand("trust")
		if changed {
			s.Project.ProjectID = "another"
			s.WorkspaceTrust = false
		}
		s.FlushActions(context.Background(), cl)
		cl.Close()
		srv.Close()
		if s.Error == "" || s.WorkspaceTrust || strings.Contains(s.CmdOutput, "trust saved") || (changed && calls != 0) {
			t.Fatalf("failed/cross-project consent claimed success: changed=%t calls=%d error=%s output=%s", changed, calls, s.Error, s.CmdOutput)
		}
	}
}

func TestTrustGrantAndRevokeUseCapturedDecisions(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
	var decisions []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Confirm bool }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		decisions = append(decisions, body.Confirm)
		_ = json.NewEncoder(w).Encode(ExecutionTrust{Trusted: body.Confirm})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.ExecuteCommand("trust")
	s.ExecuteCommand("trust")
	s.FlushActions(context.Background(), cl)
	if s.Error != "" || len(decisions) != 2 || !decisions[0] || decisions[1] || s.WorkspaceTrust {
		t.Fatalf("queued decisions were replaced by later state: %v %s", decisions, s.Error)
	}
}
