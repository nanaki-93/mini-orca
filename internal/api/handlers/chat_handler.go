package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/agent"
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

// ChatHandler manages chat message and history endpoints.
type ChatHandler struct {
	mu           sync.Mutex
	history      []ChatResponse
	orchestrator *agent.Orchestrator
	projectPath  string
}

// NewChatHandler creates a new ChatHandler instance.
func NewChatHandler(llmClient *llm.Client, executor tools.ToolExecutor, projectPath string) *ChatHandler {
	return &ChatHandler{
		history:      make([]ChatResponse, 0),
		orchestrator: agent.NewOrchestrator(llmClient, executor),
		projectPath:  projectPath,
	}
}

// SendMessage handles POST /api/chat/message
// Sends a message to the chat, triggers the pipeline, and returns the response.
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Message == "" {
		api.WriteError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Store user message
	h.mu.Lock()
	historyEntry := ChatResponse{
		Role:      "system",
		Content:   req.Message,
		Phase:     "coding",
		Timestamp: time.Now(),
	}
	h.history = append(h.history, historyEntry)
	h.mu.Unlock()

	// Build project context
	projectContext := ""
	if h.projectPath != "" {
		goModPath := fmt.Sprintf("%s/go.mod", h.projectPath)
		if data, err := os.ReadFile(goModPath); err == nil {
			projectContext = string(data)
		}
	}

	// Run the pipeline
	result, err := h.orchestrator.RunCoderFromPrompt(req.Message, projectContext)
	if err != nil {
		h.mu.Lock()
		h.history = append(h.history, ChatResponse{
			Role:      "assistant",
			Content:   fmt.Sprintf("Error: %v", err),
			Phase:     "error",
			Timestamp: time.Now(),
		})
		h.mu.Unlock()
		api.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Store assistant response
	h.mu.Lock()
	h.history = append(h.history, ChatResponse{
		Role:      "assistant",
		Content:   result.Output,
		Phase:     "coding",
		Timestamp: time.Now(),
	})
	h.mu.Unlock()

	api.WriteJSON(w, http.StatusOK, result)
}

// GetHistory handles GET /api/chat/history
// Returns the conversation history.
func (h *ChatHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.mu.Lock()
	historyCopy := make([]ChatResponse, len(h.history))
	copy(historyCopy, h.history)
	h.mu.Unlock()

	api.WriteJSON(w, http.StatusOK, historyCopy)
}
