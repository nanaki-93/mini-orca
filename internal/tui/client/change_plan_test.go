package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlanAndFileReviewRoutesStayWriteOnlyAndGuarded(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	for _, action := range []string{"plan", "review-file"} {
		path := "/api/projects/current/changes/change-123/" + action
		if _, err := cl.Request(context.Background(), action, http.MethodPost, path, `{"project_id":"p","project_revision":"r","revision":1,"hash":"h"}`); err != nil {
			t.Fatal(err)
		}
		if _, err := cl.Request(context.Background(), action+"-read", http.MethodGet, path, ""); err == nil {
			t.Fatal("unsupported read method reached daemon")
		}
	}
	if requests != 2 {
		t.Fatalf("unexpected dispatch count: %d", requests)
	}
}
