package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestCurrentModelCatalogPreservesFunctionProjectionWithoutSecrets(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.New(&config.Config{
		LLM: config.LLMConfig{BaseURL: "http://localhost:1234", Model: "legacy", Temperature: 0.3, MaxTokens: 1024},
		ModelScopes: config.ModelScopesConfig{
			Analyze:  config.ModelProfileConfig{APIBaseURL: "https://analyze.example/v1", APIKey: "secret-value", Model: "analyze"},
			Bug:      config.ModelProfileConfig{APIBaseURL: "http://127.0.0.1:11434/v1", Model: "bug"},
			Function: config.ModelProfileConfig{APIBaseURL: "https://function.example/v1", APIKey: "secret-value", Model: "function"},
		},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	NewModelHandler(service).Current(response, httptest.NewRequest(http.MethodGet, "/api/models/current", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var catalog app.ModelCatalog
	if err := json.NewDecoder(response.Body).Decode(&catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Model != "function" || catalog.Scopes["analyze"].Model != "analyze" || catalog.Scopes["bug"].RemoteProvider || !catalog.Scopes["function"].RemoteProvider {
		t.Fatalf("catalog = %+v", catalog)
	}
	if strings.Contains(response.Body.String(), "secret-value") || strings.Contains(response.Body.String(), "/v1") {
		t.Fatalf("model response leaked secret configuration: %s", response.Body.String())
	}
}
