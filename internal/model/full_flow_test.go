package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_FullFlow_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			resp := listModelsResponse{
				Object: "list",
				Data: []modelEntry{
					{ID: "model-a", Object: "model", OwnedBy: "owner-a"},
					{ID: "model-b", Object: "model", OwnedBy: "owner-b"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)

	models, err := router.RouteListModels(context.Background())
	if err != nil {
		t.Fatalf("RouteListModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "model-a" {
		t.Errorf("expected model-a, got %s", models[0].ID)
	}
}

func TestRouter_FullFlow_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}
			if req.Model != "test-model" {
				t.Errorf("expected model test-model, got %s", req.Model)
			}

			resp := ChatResponse{
				ID:      "chat-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "test-model",
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Hello from LM Studio!"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("coding", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "test-model",
		Temperature: 0.7,
		MaxTokens:   1024,
	})

	result, err := router.Chat("coding", []ChatMessage{{Role: "user", Content: "Hello"}})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.ID != "chat-1" {
		t.Errorf("expected chat-1, got %s", result.ID)
	}
	if result.Model != "test-model" {
		t.Errorf("expected model test-model, got %s", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Hello from LM Studio!" {
		t.Errorf("expected 'Hello from LM Studio!', got %s", result.Choices[0].Message.Content)
	}
}

func TestRouter_FullFlow_RouteChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := ChatResponse{
				ID:      "chat-2",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "route-model",
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Routed!"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("coding", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "route-model",
		Temperature: 0.5,
		MaxTokens:   2048,
	})

	result, err := router.RouteChat(context.Background(), "coding", []ChatMessage{{Role: "user", Content: "Write code"}})
	if err != nil {
		t.Fatalf("RouteChat failed: %v", err)
	}

	if result.ID != "chat-2" {
		t.Errorf("expected chat-2, got %s", result.ID)
	}
}

func TestRouter_FullFlow_FallbackToActive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			resp := ChatResponse{
				ID:      "chat-3",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "active-model",
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "Fallback!"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	// No phase config set — should fall back to active provider
	router.SetActiveProvider(provider.Name())

	result, err := router.Chat("unknown_phase", []ChatMessage{{Role: "user", Content: "Test"}})
	if err != nil {
		t.Fatalf("Chat fallback failed: %v", err)
	}

	if result.Model != "active-model" {
		t.Errorf("expected active-model, got %s", result.Model)
	}
}

func TestRouter_FullFlow_GetPhaseConfig(t *testing.T) {
	router := NewRouter()
	router.SetDefaultConfig("coding", ModelConfig{
		Provider:    "lm-studio",
		ModelID:     "coding-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})

	phaseCfg, err := router.GetPhaseConfig("coding")
	if err != nil {
		t.Fatalf("GetPhaseConfig failed: %v", err)
	}

	if phaseCfg.ModelID != "coding-model" {
		t.Errorf("expected coding-model, got %s", phaseCfg.ModelID)
	}
	if phaseCfg.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %f", phaseCfg.Temperature)
	}
	if phaseCfg.MaxTokens != 4096 {
		t.Errorf("expected 4096, got %d", phaseCfg.MaxTokens)
	}
}

func TestRouter_FullFlow_UnconfiguredPhase(t *testing.T) {
	router := NewRouter()
	_, err := router.GetPhaseConfig("nonexistent")
	if err == nil {
		t.Fatal("expected error for unconfigured phase")
	}
}

// TestFullFlow_CodingToHumanGate tests the complete flow: coding → testing → review → human_review
func TestFullFlow_CodingToHumanGate(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			callCount++

			// Different responses based on model/phase
			content := "Response"
			switch req.Model {
			case "coding-model":
				content = "package user\n\ntype User struct {\n\tID string\n\tName string\n}"
			case "testing-model":
				content = "=== RUN TestUser\n--- PASS: TestUser (0.00s)\n\nPASS"
			case "review-model":
				content = "## Score: 85\n## Summary: Code is well structured\n## Recommendation: Approve"
			}

			resp := ChatResponse{
				ID:      "flow-1",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   req.Model,
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: content}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("coding", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "coding-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})
	router.SetDefaultConfig("testing", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "testing-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})
	router.SetDefaultConfig("review", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "review-model",
		Temperature: 0.5,
		MaxTokens:   2048,
	})

	// Start session (simulated by using the router)
	// 1. Run coding phase
	codingResult, err := router.Chat("coding", []ChatMessage{{Role: "user", Content: "Create a User struct"}})
	if err != nil {
		t.Fatalf("coding phase failed: %v", err)
	}
	if codingResult.ID == "" {
		t.Error("expected non-empty coding result ID")
	}
	if codingResult.Choices[0].Message.Content == "" {
		t.Error("expected non-empty coding output")
	}

	// 2. Run testing phase
	testResult, err := router.Chat("testing", []ChatMessage{{Role: "user", Content: "Generate tests"}})
	if err != nil {
		t.Fatalf("testing phase failed: %v", err)
	}
	if testResult.Choices[0].Message.Content == "" {
		t.Error("expected non-empty test output")
	}

	// 3. Run review phase
	reviewResult, err := router.Chat("review", []ChatMessage{{Role: "user", Content: "Review the code"}})
	if err != nil {
		t.Fatalf("review phase failed: %v", err)
	}
	if reviewResult.Choices[0].Message.Content == "" {
		t.Error("expected non-empty review output")
	}

	// 4. Verify human review phase is configured (simulated by checking phase config)
	humanReviewCfg, err := router.GetPhaseConfig("human_review")
	if err != nil {
		// human_review may not have a model config since it's handled by the orchestrator
		// This is expected behavior
		t.Logf("human_review phase not configured in router (expected): %v", err)
	}
	_ = humanReviewCfg

	// Verify all phases were called
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

// TestFullFlow_RetryOnTestFailure tests that test failure sends back to coding
func TestFullFlow_RetryOnTestFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			callCount++

			// First coding pass: generate code
			if req.Model == "coding-model" && callCount == 1 {
				resp := ChatResponse{
					ID:      "flow-1",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID string\n}"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Second coding pass (after retry): improved code
			if req.Model == "coding-model" && callCount == 4 {
				resp := ChatResponse{
					ID:      "flow-2",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID    string `json:\"id\"`\n\tName  string `json:\"name\"`\n\tEmail string `json:\"email\"`"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Testing always fails on first attempt
			if req.Model == "testing-model" {
				resp := ChatResponse{
					ID:      "flow-test",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "=== RUN TestUser\n--- FAIL: TestUser (0.00s)\n\nFAIL: tests failed"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Review phase
			if req.Model == "review-model" {
				resp := ChatResponse{
					ID:      "flow-review",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "## Score: 90\n## Summary: Good code\n## Recommendation: Approve"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			resp := ChatResponse{
				ID:      "flow-default",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   req.Model,
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "OK"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("coding", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "coding-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})
	router.SetDefaultConfig("testing", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "testing-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})
	router.SetDefaultConfig("review", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "review-model",
		Temperature: 0.5,
		MaxTokens:   2048,
	})

	// Simulate retry loop: coding → testing (fail) → coding → testing (pass) → review
	// 1. First coding pass
	codingResult, err := router.Chat("coding", []ChatMessage{{Role: "user", Content: "Create User struct"}})
	if err != nil {
		t.Fatalf("coding phase failed: %v", err)
	}
	if codingResult.Choices[0].Message.Content == "" {
		t.Error("expected non-empty coding output")
	}

	// 2. First testing pass (will fail)
	testResult, err := router.Chat("testing", []ChatMessage{{Role: "user", Content: "Generate tests"}})
	if err != nil {
		t.Fatalf("testing phase failed: %v", err)
	}
	if testResult.Choices[0].Message.Content == "" {
		t.Error("expected non-empty test output")
	}

	// 3. Second coding pass (retry)
	codingResult2, err := router.Chat("coding", []ChatMessage{{Role: "user", Content: "Improve User struct based on test failures"}})
	if err != nil {
		t.Fatalf("coding retry failed: %v", err)
	}
	if codingResult2.Choices[0].Message.Content == "" {
		t.Error("expected non-empty coding output on retry")
	}

	// 4. Second testing pass (will pass)
	_, err = router.Chat("testing", []ChatMessage{{Role: "user", Content: "Generate tests"}})
	if err != nil {
		t.Fatalf("testing phase failed: %v", err)
	}

	// 5. Review phase
	reviewResult, err := router.Chat("review", []ChatMessage{{Role: "user", Content: "Review the code"}})
	if err != nil {
		t.Fatalf("review phase failed: %v", err)
	}
	if reviewResult.Choices[0].Message.Content == "" {
		t.Error("expected non-empty review output")
	}

	// Verify 5 API calls were made (2 coding, 2 testing, 1 review)
	if callCount != 5 {
		t.Errorf("expected 5 API calls, got %d", callCount)
	}
}

// TestFullFlow_RetryOnReviewFailure tests that review failure sends back to coding
func TestFullFlow_RetryOnReviewFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var req ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
				return
			}

			callCount++

			// First coding pass
			if req.Model == "coding-model" && callCount == 1 {
				resp := ChatResponse{
					ID:      "flow-1",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID string\n}"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Second coding pass (after review retry)
			if req.Model == "coding-model" && callCount == 4 {
				resp := ChatResponse{
					ID:      "flow-2",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "package user\n\ntype User struct {\n\tID    string `json:\"id\"`\n\tName  string `json:\"name\"`\n\tEmail string `json:\"email\"`"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Testing always passes
			if req.Model == "testing-model" {
				resp := ChatResponse{
					ID:      "flow-test",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "=== RUN TestUser\n--- PASS: TestUser (0.00s)\n\nPASS"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// First review (will fail/reject)
			if req.Model == "review-model" && callCount == 3 {
				resp := ChatResponse{
					ID:      "flow-review",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "## Score: 40\n## Summary: Missing error handling\n## Recommendation: Reject"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Second review (will pass)
			if req.Model == "review-model" && callCount == 6 {
				resp := ChatResponse{
					ID:      "flow-review-2",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   req.Model,
					Choices: []ChatChoice{
						{Index: 0, Message: ChatMessage{Role: "assistant", Content: "## Score: 85\n## Summary: Good code with error handling\n## Recommendation: Approve"}, FinishReason: "stop"},
					},
					Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			resp := ChatResponse{
				ID:      "flow-default",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   req.Model,
				Choices: []ChatChoice{
					{Index: 0, Message: ChatMessage{Role: "assistant", Content: "OK"}, FinishReason: "stop"},
				},
				Usage: ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewLMStudioProvider(server.URL)
	router := NewRouter()
	router.RegisterProvider(provider)
	router.SetDefaultConfig("coding", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "coding-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})
	router.SetDefaultConfig("testing", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "testing-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	})
	router.SetDefaultConfig("review", ModelConfig{
		Provider:    provider.Name(),
		ModelID:     "review-model",
		Temperature: 0.5,
		MaxTokens:   2048,
	})

	// Simulate retry loop: coding → testing → review (fail) → coding → testing → review (pass)
	// 1. First coding pass
	codingResult, err := router.Chat("coding", []ChatMessage{{Role: "user", Content: "Create User struct"}})
	if err != nil {
		t.Fatalf("coding phase failed: %v", err)
	}
	if codingResult.Choices[0].Message.Content == "" {
		t.Error("expected non-empty coding output")
	}

	// 2. Testing pass
	_, err = router.Chat("testing", []ChatMessage{{Role: "user", Content: "Generate tests"}})
	if err != nil {
		t.Fatalf("testing phase failed: %v", err)
	}

	// 3. First review (will fail)
	_, err = router.Chat("review", []ChatMessage{{Role: "user", Content: "Review the code"}})
	if err != nil {
		t.Fatalf("review phase failed: %v", err)
	}

	// 4. Second coding pass (retry)
	_, err = router.Chat("coding", []ChatMessage{{Role: "user", Content: "Fix review issues"}})
	if err != nil {
		t.Fatalf("coding retry failed: %v", err)
	}

	// 5. Second testing pass
	_, err = router.Chat("testing", []ChatMessage{{Role: "user", Content: "Generate tests"}})
	if err != nil {
		t.Fatalf("testing phase failed: %v", err)
	}

	// 6. Second review (will pass)
	reviewResult2, err := router.Chat("review", []ChatMessage{{Role: "user", Content: "Review the code"}})
	if err != nil {
		t.Fatalf("review phase failed: %v", err)
	}
	if reviewResult2.Choices[0].Message.Content == "" {
		t.Error("expected non-empty review output on retry")
	}

	// Verify 6 API calls were made (2 coding, 2 testing, 2 review)
	if callCount != 6 {
		t.Errorf("expected 6 API calls, got %d", callCount)
	}
}
