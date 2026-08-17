package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/agent"
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

// ChatHandler manages chat message and history endpoints.
type ChatHandler struct {
	mu        sync.Mutex
	history   []ChatResponse
	llmClient *llm.Client
	manager   *project.Manager
}

// NewChatHandler creates a new ChatHandler instance.
func NewChatHandler(llmClient *llm.Client, manager *project.Manager) *ChatHandler {
	return &ChatHandler{
		history:   make([]ChatResponse, 0),
		llmClient: llmClient,
		manager:   manager,
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

	if req.FilePath == "" {
		api.WriteError(w, http.StatusBadRequest, "a file must be open to send a message")
		return
	}
	if strings.TrimSpace(req.TargetSymbol) == "" {
		api.WriteError(w, http.StatusBadRequest, "target_symbol is required (function or class name)")
		return
	}
	if len(req.TargetSymbol) > 200 || strings.ContainsAny(req.TargetSymbol, "\r\n") {
		api.WriteError(w, http.StatusBadRequest, "target_symbol is invalid")
		return
	}
	projectRoot := h.manager.Root()
	fileInfo, err := project.GetFileInfo(projectRoot, req.FilePath)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if fileInfo.Binary {
		api.WriteError(w, http.StatusBadRequest, "target file must be a text source file")
		return
	}

	// Store user message
	h.mu.Lock()
	historyEntry := ChatResponse{
		Role:      "user",
		Content:   req.Message,
		Phase:     "coding",
		Timestamp: time.Now(),
	}
	h.history = append(h.history, historyEntry)
	h.mu.Unlock()

	projectContext, err := project.NewContextBuilder().Build(projectRoot, req.FilePath)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("build project context: %v", err))
		return
	}

	// Run the pipeline
	_, executor := tools.InitToolExecutorWithPath(projectRoot)
	orchestrator := agent.NewOrchestrator(h.llmClient, executor)
	result, err := orchestrator.RunCoderForSymbol(req.Message, projectContext, req.FilePath, req.TargetSymbol)
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
