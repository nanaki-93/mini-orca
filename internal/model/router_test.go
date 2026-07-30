package model

import (
	"context"
	"testing"
)

type MockProvider struct {
	name string
}

func (m *MockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{Model: req.Model}, nil
}

func (m *MockProvider) ListModels(ctx context.Context) ([]Model, error) {
	return []Model{{ID: "model-1"}}, nil
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) IsStreamingSupported() bool {
	return false
}

func TestRouter(t *testing.T) {
	r := NewRouter()
	p1 := &MockProvider{name: "p1"}
	p2 := &MockProvider{name: "p2"}

	r.RegisterProvider(p1)
	r.RegisterProvider(p2)

	t.Run("RegisterProvider and GetProvider", func(t *testing.T) {
		p, err := r.GetProvider("p1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if p.Name() != "p1" {
			t.Errorf("expected p1, got %s", p.Name())
		}

		_, err = r.GetProvider("non-existent")
		if err == nil {
			t.Error("expected error for non-existent provider")
		}
	})

	t.Run("ActiveProvider", func(t *testing.T) {
		// First registered should be active
		p, err := r.GetActiveProvider()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if p.Name() != "p1" {
			t.Errorf("expected p1, got %s", p.Name())
		}

		err = r.SetActiveProvider("p2")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		p, _ = r.GetActiveProvider()
		if p.Name() != "p2" {
			t.Errorf("expected p2, got %s", p.Name())
		}
	})

	t.Run("Defaults", func(t *testing.T) {
		cfg := ModelConfig{Provider: "p1", ModelID: "m1", Temperature: 0.7}
		r.SetDefaultConfig("planning", cfg)

		got := r.GetDefaultConfig("planning")
		if got.ModelID != "m1" {
			t.Errorf("expected m1, got %s", got.ModelID)
		}

		got = r.GetDefaultConfig("non-existent")
		if got.ModelID != "" {
			t.Error("expected empty config for non-existent key")
		}
	})

	t.Run("RouteChat", func(t *testing.T) {
		r.SetActiveProvider("p2")
		r.SetDefaultConfig("coding", ModelConfig{Provider: "p1", ModelID: "m-coding"})

		// Should use phase-specific config
		resp, err := r.RouteChat(context.Background(), "coding", nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Model != "m-coding" {
			t.Errorf("expected m-coding, got %s", resp.Model)
		}

		// Should fallback to active provider (p2)
		resp, err = r.RouteChat(context.Background(), "unknown", nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Model != "" {
			t.Errorf("expected empty model ID for fallback, got %s", resp.Model)
		}
	})

	t.Run("GetPhaseConfig", func(t *testing.T) {
		r.SetDefaultConfig("testing", ModelConfig{Provider: "p1", ModelID: "m-test", Temperature: 0.5})
		pc, err := r.GetPhaseConfig("testing")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pc.ModelID != "m-test" {
			t.Errorf("expected m-test, got %s", pc.ModelID)
		}
		if pc.Temperature != 0.5 {
			t.Errorf("expected 0.5, got %f", pc.Temperature)
		}

		_, err = r.GetPhaseConfig("unconfigured")
		if err == nil {
			t.Error("expected error for unconfigured phase")
		}
	})

	t.Run("ListProviders", func(t *testing.T) {
		names := r.ListProviders()
		if len(names) != 2 {
			t.Errorf("expected 2 providers, got %d", len(names))
		}
	})

	t.Run("Chat method", func(t *testing.T) {
		r.SetDefaultConfig("review", ModelConfig{Provider: "p2", ModelID: "m-review"})
		resp, err := r.Chat("review", nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Model != "m-review" {
			t.Errorf("expected m-review, got %s", resp.Model)
		}
	})

	t.Run("RouteListModels", func(t *testing.T) {
		r.SetActiveProvider("p1")
		models, err := r.RouteListModels(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(models) != 1 || models[0].ID != "model-1" {
			t.Errorf("expected model-1, got %v", models)
		}
	})
}
