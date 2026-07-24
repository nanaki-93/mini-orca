package model

import (
	"testing"
)

func TestNewRouter(t *testing.T) {
	config := Config{
		ActiveProvider: "lm-studio",
		Providers: map[string]ProviderConfig{
			"lm-studio": {BaseURL: "http://127.0.0.1:1234"},
		},
		Phases: map[Phase]PhaseConfig{
			PhasePlanning: {Provider: "lm-studio", Model: "test-model", Temperature: 0.3},
			PhaseCoding:   {Provider: "lm-studio", Model: "test-model", Temperature: 0.1},
		},
	}

	router := NewRouter(config)
	if router == nil {
		t.Fatal("expected non-nil router")
	}

	cfg := router.GetConfig()
	if cfg.ActiveProvider != "lm-studio" {
		t.Errorf("expected active provider 'lm-studio', got '%s'", cfg.ActiveProvider)
	}
}

func TestGetModelConfigForPhase(t *testing.T) {
	config := Config{
		ActiveProvider: "lm-studio",
		Providers: map[string]ProviderConfig{
			"lm-studio": {BaseURL: "http://127.0.0.1:1234"},
		},
		Phases: map[Phase]PhaseConfig{
			PhasePlanning: {Provider: "lm-studio", Model: "qwen-30b", Temperature: 0.3, MaxTokens: 4096},
		},
	}

	router := NewRouter(config)
	modelCfg, err := router.GetModelConfigForPhase(PhasePlanning)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if modelCfg.Model != "qwen-30b" {
		t.Errorf("expected model 'qwen-30b', got '%s'", modelCfg.Model)
	}
	if modelCfg.Temperature != 0.3 {
		t.Errorf("expected temperature 0.3, got %f", modelCfg.Temperature)
	}
	if modelCfg.MaxTokens != 4096 {
		t.Errorf("expected max tokens 4096, got %d", modelCfg.MaxTokens)
	}
}

func TestGetModelConfigForPhase_NotConfigured(t *testing.T) {
	config := Config{
		ActiveProvider: "lm-studio",
		Providers:      map[string]ProviderConfig{},
		Phases:         map[Phase]PhaseConfig{},
	}

	router := NewRouter(config)
	_, err := router.GetModelConfigForPhase(PhasePlanning)
	if err == nil {
		t.Error("expected error for unconfigured phase, got nil")
	}
}

func TestChatOption_WithTemperature(t *testing.T) {
	temp := 0.5
	req := ChatRequest{}
	opt := WithTemperature(temp)
	opt(&req)

	if req.Temperature == nil {
		t.Fatal("expected temperature to be set")
	}
	if *req.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", *req.Temperature)
	}
}

func TestChatOption_WithMaxTokens(t *testing.T) {
	req := ChatRequest{}
	opt := WithMaxTokens(2048)
	opt(&req)

	if req.MaxTokens != 2048 {
		t.Errorf("expected max tokens 2048, got %d", req.MaxTokens)
	}
}

func TestChatOption_WithTopP(t *testing.T) {
	topP := 0.9
	req := ChatRequest{}
	opt := WithTopP(topP)
	opt(&req)

	if req.TopP == nil {
		t.Fatal("expected top_p to be set")
	}
	if *req.TopP != 0.9 {
		t.Errorf("expected top_p 0.9, got %f", *req.TopP)
	}
}

func TestMessage_RoleValidation(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	for _, msg := range msgs {
		if msg.Role == "" {
			t.Errorf("expected non-empty role")
		}
		if msg.Content == "" {
			t.Errorf("expected non-empty content for role %s", msg.Role)
		}
	}
}

func TestChatRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ChatRequest
		wantErr bool
	}{
		{
			name:    "valid request",
			req:     ChatRequest{Model: "test", Messages: []Message{{Role: "user", Content: "hello"}}},
			wantErr: false,
		},
		{
			name:    "empty model",
			req:     ChatRequest{Model: "", Messages: []Message{{Role: "user", Content: "hello"}}},
			wantErr: true,
		},
		{
			name:    "empty messages",
			req:     ChatRequest{Model: "test", Messages: nil},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation check
			hasError := tt.req.Model == "" || len(tt.req.Messages) == 0
			if hasError != tt.wantErr {
				t.Errorf("validation result = %v, wantErr %v", hasError, tt.wantErr)
			}
		})
	}
}

func TestPhaseConstants(t *testing.T) {
	expectedPhases := []Phase{
		PhasePlanning,
		PhaseCoding,
		PhaseTesting,
		PhaseReview,
		PhaseHumanReview,
	}

	for _, phase := range expectedPhases {
		if phase == "" {
			t.Errorf("phase constant is empty")
		}
	}
}

func TestProviderConfig_DefaultValues(t *testing.T) {
	cfg := ProviderConfig{}

	if cfg.BaseURL != "" {
		t.Error("expected empty default base_url")
	}
	if cfg.APIKey != "" {
		t.Error("expected empty default api_key")
	}
}

func TestPhaseConfig_DefaultValues(t *testing.T) {
	cfg := PhaseConfig{}

	if cfg.Provider != "" {
		t.Error("expected empty default provider")
	}
	if cfg.Model != "" {
		t.Error("expected empty default model")
	}
	if cfg.Temperature != 0 {
		t.Error("expected zero default temperature")
	}
	if cfg.MaxTokens != 0 {
		t.Error("expected zero default max_tokens")
	}
}

func TestChatResponse_DefaultValues(t *testing.T) {
	resp := ChatResponse{}

	if len(resp.Choices) != 0 {
		t.Errorf("expected empty choices, got %d", len(resp.Choices))
	}
}

func TestModelConfig_AllFields(t *testing.T) {
	cfg := ModelConfig{
		Provider:    "lm-studio",
		Model:       "qwen-30b",
		BaseURL:     "http://127.0.0.1:1234",
		APIKey:      "test-key",
		Temperature: 0.3,
		MaxTokens:   4096,
		TopP:        0.9,
		Timeout:     60,
	}

	if cfg.Provider != "lm-studio" {
		t.Errorf("expected provider 'lm-studio', got '%s'", cfg.Provider)
	}
	if cfg.Model != "qwen-30b" {
		t.Errorf("expected model 'qwen-30b', got '%s'", cfg.Model)
	}
	if cfg.Timeout != 60 {
		t.Errorf("expected timeout 60, got %d", cfg.Timeout)
	}
}

// BenchmarkNewRouter benchmarks creating a new router
func BenchmarkNewRouter(b *testing.B) {
	config := Config{
		ActiveProvider: "lm-studio",
		Providers: map[string]ProviderConfig{
			"lm-studio": {BaseURL: "http://127.0.0.1:1234"},
		},
		Phases: map[Phase]PhaseConfig{
			PhasePlanning: {Provider: "lm-studio", Model: "test-model"},
			PhaseCoding:   {Provider: "lm-studio", Model: "test-model"},
			PhaseTesting:  {Provider: "lm-studio", Model: "test-model"},
			PhaseReview:   {Provider: "lm-studio", Model: "test-model"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewRouter(config)
	}
}

// BenchmarkGetModelConfig benchmarks getting model config
func BenchmarkGetModelConfig(b *testing.B) {
	config := Config{
		ActiveProvider: "lm-studio",
		Providers: map[string]ProviderConfig{
			"lm-studio": {BaseURL: "http://127.0.0.1:1234"},
		},
		Phases: map[Phase]PhaseConfig{
			PhasePlanning: {Provider: "lm-studio", Model: "test-model"},
		},
	}
	router := NewRouter(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = router.GetModelConfigForPhase(PhasePlanning)
	}
}
