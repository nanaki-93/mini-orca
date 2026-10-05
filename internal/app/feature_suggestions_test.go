package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const validFeatureResponse = `{"suggestions":[{"title":"Add cancellation-aware work","benefit":"Let callers stop work early.","evidence":"The project exposes a Run entry point.","paths":["main.go"],"effort":"small","acceptance_criteria":["Canceled work returns promptly."]}]}`

func TestFeatureGenerationGetsIndependentExtendedDeadline(t *testing.T) {
	server := changeProvider(t, func() string { return validFeatureResponse })
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	for _, timeout := range []time.Duration{time.Millisecond, 20 * time.Minute} {
		runtime := s.runtimes.analyze
		runtime.effective.Timeout = timeout.String()
		var remaining time.Duration
		report, err := s.generateFeatures(context.Background(), featureRequestFor(t, s, "Improve cancellation."), runtime, nil, featureGenerationAuthority{
			BeforeAttempt: func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("feature request has no bounded deadline")
				}
				remaining = time.Until(deadline)
				return nil
			},
			Publish: func(write func() error) error { return write() },
		})
		if err != nil || len(report.Suggestions) != 1 || remaining < max(10*time.Minute, timeout)-time.Second {
			t.Fatalf("feature request deadline=%s report=%+v err=%v", remaining, report, err)
		}
	}
}

func featureRequestFor(t *testing.T, service *Service, goals string) FeatureRequest {
	t.Helper()
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	report, err := service.Features(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return FeatureRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, ExpectedHash: report.Hash, Goals: goals}
}

func TestFeatureGenerationGoalsAndSavedStatus(t *testing.T) {
	var calls atomic.Int32
	server := changeProvider(t, func() string { calls.Add(1); return validFeatureResponse })
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	request := featureRequestFor(t, service, "Improve cancellation.")
	goals, err := service.SaveFeatureGoals(request)
	if err != nil || goals.Goals != request.Goals || calls.Load() != 0 {
		t.Fatalf("passive goals: %+v, %v", goals, err)
	}
	request.ExpectedHash = goals.Hash
	report, err := service.GenerateFeatures(context.Background(), request)
	if err != nil || report.Status != "ready" || len(report.Suggestions) != 1 {
		t.Fatalf("generated features: %+v, %v", report, err)
	}
	request.ExpectedHash = report.Hash
	saved, err := service.UpdateFeatureStatus(report.Suggestions[0].ID, FeatureStatusRequest{FeatureRequest: request, Status: "saved"})
	if err != nil || saved.Suggestions[0].Status != "saved" {
		t.Fatalf("save feature: %+v, %v", saved, err)
	}
	request.ExpectedHash = saved.Hash
	refreshed, err := service.GenerateFeatures(context.Background(), request)
	if err != nil || refreshed.Suggestions[0].Status != "saved" {
		t.Fatalf("regeneration lost triage: %+v, %v", refreshed, err)
	}
	passive, err := service.Features(context.Background())
	if err != nil || passive.Freshness != "current" || calls.Load() != 2 {
		t.Fatalf("feature reads dispatch: %+v, %v", passive, err)
	}
	request.ExpectedHash = passive.Hash
	request.Goals = "Different project direction."
	changed, err := service.SaveFeatureGoals(request)
	if err != nil || changed.Freshness != "stale" {
		t.Fatalf("changed goals reused evidence: %+v, %v", changed, err)
	}
}

func TestFeatureMalformedOutputPreservesPreviousSuggestions(t *testing.T) {
	var output atomic.Value
	output.Store(validFeatureResponse)
	server := changeProvider(t, func() string { return output.Load().(string) })
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	good, err := service.GenerateFeatures(context.Background(), featureRequestFor(t, service, "Improve the project."))
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{`{}`, strings.Replace(validFeatureResponse, "main.go", "invented.go", 1), strings.Replace(validFeatureResponse, "small", "instant", 1), strings.Replace(validFeatureResponse, `"benefit":`, `"status":"verified","benefit":`, 1)} {
		output.Store(invalid)
		request := featureRequestFor(t, service, "Improve the project.")
		if _, err := service.GenerateFeatures(context.Background(), request); err == nil {
			t.Fatalf("accepted output %s", invalid)
		}
		retained, err := service.Features(context.Background())
		if err != nil || retained.Status != "failed" || retained.Failure != "Feature search failed. Try again." || retained.Suggestions[0].ID != good.Suggestions[0].ID {
			t.Fatalf("failure lost previous ideas: %+v, %v", retained, err)
		}
	}
}

func TestFeatureFreshnessEmptyAndRemoteConsent(t *testing.T) {
	server := changeProvider(t, func() string { return `{"suggestions":[]}` })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	report, err := service.GenerateFeatures(context.Background(), featureRequestFor(t, service, "Keep the project small."))
	if err != nil || report.Status != "ready" || len(report.Suggestions) != 0 {
		t.Fatalf("empty success: %+v, %v", report, err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("New project guidance."), 0644); err != nil {
		t.Fatal(err)
	}
	stale, err := service.Features(context.Background())
	if err != nil || stale.Freshness != "stale" {
		t.Fatalf("instruction change not stale: %+v, %v", stale, err)
	}
	remote, _ := newSemanticAnalysisService(t, "https://provider.invalid", 0)
	if _, err := remote.GenerateFeatures(context.Background(), featureRequestFor(t, remote, "Improve.")); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("remote consent: %v", err)
	}
}
