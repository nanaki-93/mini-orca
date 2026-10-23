package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func declarationRequest(t *testing.T, service *Service) DeclarationExplanationRequest {
	t.Helper()
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	return DeclarationExplanationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run"}
}

func repositoryToolCall(w io.Writer, name, args string) {
	fmt.Fprintf(w, `{"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"read-1","type":"function","function":{"name":%q,"arguments":%q}}]}}]}`, name, args)
}

func TestDeclarationToolsReturnScopeRequestWithoutLeakingSource(t *testing.T) {
	for _, args := range []string{`{"path":"main.go"}`, `{"path":"helper.go"}`, `{"path":"main.go","start_line":1,"end_line":100}`} {
		t.Run(args, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				data, _ := io.ReadAll(r.Body)
				if strings.Contains(string(data), "helper secret") {
					t.Error("outside-scope source sent")
				}
				if calls == 1 {
					repositoryToolCall(w, "read_snippet", args)
					return
				}
				if args != `{"path":"main.go"}` && !strings.Contains(string(data), "scope_required") {
					t.Errorf("scope request missing: %s", data)
				}
				json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validDeclarationExplanation}}}})
			}))
			defer server.Close()
			service, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
			before, _ := os.ReadFile(filepath.Join(root, "main.go"))
			if _, err := service.ExplainDeclaration(t.Context(), declarationRequest(t, service)); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(filepath.Join(root, "main.go"))
			if calls != 2 || string(before) != string(after) || len(service.changeAuthority) != 0 {
				t.Fatalf("unexpected mutation or calls=%d", calls)
			}
		})
	}
}

func TestDeclarationToolsStopBeforeNextDispatchWhenIdentityChanges(t *testing.T) {
	for _, name := range []string{"main.go", "AGENTS.md"} {
		t.Run(name, func(t *testing.T) {
			root := ""
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if err := os.WriteFile(filepath.Join(root, name), []byte("changed instructions or source"), 0600); err != nil {
					t.Error(err)
				}
				repositoryToolCall(w, "read_snippet", `{"path":"main.go"}`)
			}))
			defer server.Close()
			service, dir := newSemanticAnalysisService(t, server.URL, 2)
			root = dir
			_, err := service.ExplainDeclaration(t.Context(), declarationRequest(t, service))
			if !errors.Is(err, agentruntime.ErrPolicyDenied) || calls != 1 {
				t.Fatalf("changed authority dispatched again: %d %v", calls, err)
			}
		})
	}
}

func TestAgentChecksRemainSourceFreeProposalEvidence(t *testing.T) {
	service, root, proposal := artifactChangeFixture(t)
	request := declarationRequest(t, service)
	prepared, err := service.prepareDeclarationExplanation(t.Context(), request, service.runtimes.function)
	if err != nil {
		t.Fatal(err)
	}
	scope := project.AgentScope{Path: "main.go", Hash: request.BaseFileHash, StartLine: prepared.symbol.StartLine, EndLine: prepared.symbol.EndLine}
	read := func() project.AgentEvidenceResult {
		t.Helper()
		result, err := service.agentCheckEvidence(context.Background(), root, request, scope, prepared.policy)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if result := read(); result.Status != "unavailable" {
		t.Fatalf("unrun checks treated as evidence: %+v", result)
	}
	checked, err := service.CheckChange(t.Context(), proposal.ID, ChangeCheckRequest{ChangeIdentity: changeIdentity(proposal)})
	if err != nil {
		t.Fatal(err)
	}
	checked.Checks[0].Output = "private diagnostic source"
	if err := storeChangeSession(root, checked); err != nil {
		t.Fatal(err)
	}
	result := read()
	if result.Status != "ok" || len(result.Evidence) == 0 {
		t.Fatalf("missing checked proposal: %+v", result)
	}
	for _, item := range result.Evidence {
		if strings.Contains(item.Text, "private diagnostic") || !strings.Contains(item.Text, checked.Hash) {
			t.Fatalf("unbound/source-bearing check: %+v", item)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "unindexed.txt"), []byte("new workspace content"), 0600); err != nil {
		t.Fatal(err)
	}
	if result := read(); result.Status != "stale" || len(result.Evidence) != 0 {
		t.Fatalf("stale checks published: %+v", result)
	}
	service.changesMu.Lock()
	clear(service.changeAuthority)
	service.changesMu.Unlock()
	if result := read(); result.Status != "unavailable" {
		t.Fatal("passive history granted authority")
	}
}
