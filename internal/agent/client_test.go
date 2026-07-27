package agent

import (
	"testing"

	"github.com/nanaki-93/mini-orca/internal/model"
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

func TestClient_Generate_NoRouter(t *testing.T) {
	client := NewClient(nil)
	_, err := client.Generate("planning", []model.ChatMessage{{Role: "user", Content: "test"}})
	if err == nil {
		t.Fatal("expected error for nil router")
	}
}

func TestClient_Generate_EmptyPhase(t *testing.T) {
	router := model.NewRouter()
	client := NewClient(router)
	_, err := client.Generate("", []model.ChatMessage{{Role: "user", Content: "test"}})
	if err == nil {
		t.Fatal("expected error for empty phase")
	}
}

func TestClient_Generate_EmptyMessages(t *testing.T) {
	router := model.NewRouter()
	client := NewClient(router)
	_, err := client.Generate("planning", nil)
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

func TestClient_ListModels_NoRouter(t *testing.T) {
	client := NewClient(nil)
	_, err := client.ListModels()
	if err == nil {
		t.Fatal("expected error for nil router")
	}
}
