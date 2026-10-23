package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitMutationReceiptPreservesCompletionAndPartialCleanup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/projects/current/git/merge" {
			t.Errorf("unexpected mutation request: %s %s", r.Method, r.URL)
		}
		_, _ = w.Write([]byte(`{"state":"merged","removed_worktree":"/repo/fix","completed":[{"project_id":"p","kind":"security","id":"issue","path":"main.go","source":"ai","commit_oid":"fix-commit"}],"warnings":["branch retained: ref changed"]}`))
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	var receipt GitMutationResult
	err = cl.DoJSON(context.Background(), "merge", http.MethodPost, "/api/projects/current/git/merge", map[string]any{"confirm": true}, &receipt)
	if err != nil || receipt.State != "merged" || receipt.RemovedWorktree != "/repo/fix" || receipt.RemovedBranch != "" || len(receipt.Completed) != 1 || receipt.Completed[0].CommitOID != "fix-commit" || len(receipt.Warnings) != 1 {
		t.Fatalf("lost partial mutation receipt: %+v %v", receipt, err)
	}
}
