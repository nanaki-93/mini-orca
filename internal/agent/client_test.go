package agent

import (
	"context"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestNewClient(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewClient(client)

	if agent == nil {
		t.Fatal("expected non-nil client")
	}
	if agent.llmClient == nil {
		t.Error("expected client to hold the provided LLM client")
	}
}

func TestClient_Name(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewClient(client)
	agent.name = "test-agent"

	if got := agent.Name(); got != "test-agent" {
		t.Errorf("expected test-agent, got %s", got)
	}
}

func TestClient_Execute_NoClient(t *testing.T) {
	agent := NewClient(nil)
	agent.name = "test-agent"
	_, err := agent.Execute(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for nil LLM client")
	}
}

func TestClient_Execute_EmptyInput(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewClient(client)
	agent.name = "test-agent"
	agent.phase = "coding"
	_, err := agent.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestClient_GetSkills_Default(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewClient(client)
	agent.name = "test-agent"
	agentSkills := agent.GetSkills()
	if agentSkills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(agentSkills) != 0 {
		t.Errorf("expected empty skills, got %v", agentSkills)
	}
}

func TestClient_SetSkills(t *testing.T) {
	client := llm.NewClient("http://localhost:1234", "test-key", "test-model", 0.7, 4096)
	agent := NewClient(client)
	agent.name = "test-agent"
	agent.SetSkills([]string{"skill1", "skill2"})

	agentSkills := agent.GetSkills()
	if len(agentSkills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(agentSkills))
	}
	if agentSkills[0] != "skill1" || agentSkills[1] != "skill2" {
		t.Errorf("expected [skill1, skill2], got %v", agentSkills)
	}
}

func TestClient_ListModels_NoClient(t *testing.T) {
	agent := NewClient(nil)
	agent.name = "test-agent"
	_, err := agent.ListModels()
	if err == nil {
		t.Fatal("expected error for nil LLM client")
	}
}
