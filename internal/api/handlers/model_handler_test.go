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

func TestCurrentModelCatalogExposesOnlyScopedProfilesWithoutSecrets(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.New(&config.Config{
		ModelScopes: config.ModelScopesConfig{
			Analyze:  config.ModelProfileConfig{APIBaseURL: "https://analyze.example/v1", APIKey: "secret-value", Model: "analyze", ReasoningEffort: "high"},
			Bug:      config.ModelProfileConfig{APIBaseURL: "http://127.0.0.1:11434/v1", Model: "bug", ReasoningEffort: "low"},
			Function: config.ModelProfileConfig{APIBaseURL: "https://function.example/v1", APIKey: "secret-value", Model: "function", ReasoningEffort: "max"},
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
	if len(catalog.Scopes) != 3 || catalog.Scopes["analyze"].Model != "analyze" || catalog.Scopes["analyze"].ReasoningEffort != "high" || catalog.Scopes["bug"].ReasoningEffort != "low" || catalog.Scopes["function"].ReasoningEffort != "max" || catalog.Scopes["bug"].RemoteProvider || !catalog.Scopes["function"].RemoteProvider {
		t.Fatalf("catalog = %+v", catalog)
	}
	if strings.Contains(response.Body.String(), "secret-value") || strings.Contains(response.Body.String(), "/v1") {
		t.Fatalf("model response leaked secret configuration: %s", response.Body.String())
	}
}

func TestCLIModelCatalogRequiresConsentWithoutLaunchingOrExposingExecutablePaths(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.ModelScopes.Analyze = config.ModelProfileConfig{Provider: config.AgyProvider, CLIPath: "/not-installed/private-agy", Model: "agy-model"}
	cfg.ModelScopes.Function = config.ModelProfileConfig{Provider: config.PiProvider, CLIPath: "/not-installed/private-pi", Model: "pi-model"}
	service, err := app.New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	NewModelHandler(service).Current(response, httptest.NewRequest(http.MethodGet, "/api/models/current", nil))
	var catalog app.ModelCatalog
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &catalog) != nil {
		t.Fatalf("catalog = %s", response.Body.String())
	}
	for scope, provider := range map[string]string{"analyze": "agy", "function": "pi"} {
		model := catalog.Scopes[scope]
		if model.ProviderOrigin != "cli://"+provider || !model.RemoteProvider || model.Temperature != nil || model.MaxTokens != nil {
			t.Fatalf("CLI metadata = %+v", model)
		}
	}
	if strings.Contains(response.Body.String(), "private-") || catalog.Scopes["bug"].MaxTokens == nil {
		t.Fatalf("invalid metadata = %s", response.Body.String())
	}
}
