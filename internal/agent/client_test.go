package agent

import (
	"context"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/model"
)

func TestNewClient(t *testing.T) {
	router := model.NewRouter()
	client := NewClient(router)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.router != router {
		t.Error("expected client to hold the provided router")
	}
}

func TestClient_Name(t *testing.T) {
	client := NewClient(nil)
	client.name = "test-agent"

	if got := client.Name(); got != "test-agent" {
		t.Errorf("expected test-agent, got %s", got)
	}
}

func TestClient_Execute_NoRouter(t *testing.T) {
	client := NewClient(nil)
	client.name = "test-agent"
	_, err := client.Execute(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for nil router")
	}
}

func TestClient_Execute_EmptyInput(t *testing.T) {
	router := model.NewRouter()
	client := NewClient(router)
	client.name = "test-agent"
	client.phase = model.PhasePlanning
	_, err := client.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestClient_GetSkills_Default(t *testing.T) {
	client := NewClient(nil)
	client.name = "test-agent"
	skills := client.GetSkills()
	if skills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(skills) != 0 {
		t.Errorf("expected empty skills, got %v", skills)
	}
}

func TestClient_SetSkills(t *testing.T) {
	client := NewClient(nil)
	client.name = "test-agent"
	client.SetSkills([]string{"skill1", "skill2"})

	skills := client.GetSkills()
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
	if skills[0] != "skill1" || skills[1] != "skill2" {
		t.Errorf("expected [skill1, skill2], got %v", skills)
	}
}

func TestClient_ListModels_NoRouter(t *testing.T) {
	client := NewClient(nil)
	client.name = "test-agent"
	_, err := client.ListModels()
	if err == nil {
		t.Fatal("expected error for nil router")
	}
}
