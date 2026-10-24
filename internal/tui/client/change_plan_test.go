package client

import (
	"context"
	"encoding/json"
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

func TestPlanInvestigationRouteAndEvidenceContract(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request PlanInvestigationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Sources[0].Hash != "hash" || !request.Sources[0].Exists || request.Sources[1].Exists || request.Plan.Citations[0].Quote != "package main" {
			t.Errorf("lost captured identity/evidence: %+v %v", request, err)
		}
		if len(request.Integrations) != 1 || request.Integrations[0] != (IntegrationUse{ID: "docs", Grant: "grant", SchemaFingerprint: "schema", ConfirmSourceSharing: true}) {
			t.Errorf("lost explicit integration grant: %+v", request.Integrations)
		}
		_ = json.NewEncoder(w).Encode(PlanInvestigation{ProjectIdentity: request.ProjectIdentity, Version: "v1", InputHash: "input", Evidence: request.Plan.Citations, Assumptions: request.Plan.Assumptions})
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	path := "/api/projects/current/plans/investigate"
	request := PlanInvestigationRequest{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}, Paths: []string{"main.go", "main_test.go"}, Sources: []PlanInvestigationSource{{Path: "main.go", Hash: "hash", Exists: true}, {Path: "main_test.go"}}, Plan: ChangePlanInput{Problem: "Coverage", Approach: "Inspect Run", Citations: []PlanEvidence{{Path: "main.go", Hash: "hash", StartLine: 1, EndLine: 1, Quote: "package main", Observation: "Package"}}, Assumptions: []string{"Preserve API"}}}
	var result PlanInvestigation
	request.Integrations = []IntegrationUse{{ID: "docs", Grant: "grant", SchemaFingerprint: "schema", ConfirmSourceSharing: true}}
	if err := cl.DoJSON(t.Context(), "investigate", http.MethodPost, path, request, &result); err != nil || len(result.Evidence) != 1 || len(result.Assumptions) != 1 {
		t.Fatalf("investigation round trip: %+v %v", result, err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		if _, err := cl.Request(t.Context(), method, method, path, ""); err == nil {
			t.Fatalf("unsupported method %s reached daemon", method)
		}
	}
	if calls != 1 {
		t.Fatalf("unexpected dispatch: %d", calls)
	}
}
