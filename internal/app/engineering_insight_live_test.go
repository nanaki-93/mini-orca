package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// TestEngineeringInsightLiveEvaluation is an opt-in local-provider observation run.
// It makes exactly eight single attempts, with no import request or retry, through
// the production prompt, client and parser. A passing test means collection
// completed; it does not certify model reliability or insight usefulness.
func TestEngineeringInsightLiveEvaluation(t *testing.T) {
	if os.Getenv("MINI_ORCA_LIVE_INSIGHT_EVAL") != "1" {
		t.Skip("explicit live evaluation only")
	}
	cfg, err := config.LoadFromYAML("../../config.yaml")
	if err != nil {
		t.Fatal("configuration unavailable")
	}
	profiles, err := config.ResolveModelProfiles(cfg)
	if err != nil {
		t.Fatal("invalid configuration")
	}
	profile := profiles.Bug
	if !isLoopbackURL(profile.APIBaseURL) {
		t.Fatal("evaluation limited to configured local provider")
	}
	profile.MaxTokens = 4096
	client := llm.NewClient(profile)
	type observation struct {
		Case                string  `json:"case"`
		Model               string  `json:"model"`
		Finish              string  `json:"finish"`
		Tokens              int     `json:"tokens"`
		Seconds             float64 `json:"seconds"`
		Valid               bool    `json:"valid"`
		ExplanationsOmitted bool    `json:"explanations_omitted"`
		PromptVersion       string  `json:"prompt_version"`
		MaxOutputTokens     int     `json:"max_output_tokens"`
		Failure             string  `json:"failure,omitempty"`
		InsightPresent      bool    `json:"insight_present"`
	}
	if err := os.MkdirAll("../../.mini-orca/autopilot", 0700); err != nil {
		t.Fatal(err)
	}
	records := []observation{}
	fixtures := loadEngineeringInsightFixtures(t)
	if len(fixtures) != 8 {
		t.Fatal("expected exactly eight cases")
	}
	for _, fixture := range fixtures {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(fixture.Source), 0600); err != nil {
			t.Fatal(err)
		}
		manager, err := project.NewManager(root)
		if err != nil {
			t.Fatal(err)
		}
		analysis := project.Analysis{Name: "evaluation", Type: "go", Path: root}
		if err := manager.Set(root, &analysis); err != nil {
			t.Fatal(err)
		}
		index, err := manager.Index()
		if err != nil {
			t.Fatal(err)
		}
		target, err := manager.IndexedFile("sample.go")
		if err != nil {
			t.Fatal(err)
		}
		prompt, err := semanticPrompt(fixture.Source, analysis, index, *target, semanticManifest(*target))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		start := time.Now()
		reply, callErr := client.Chat(ctx, []llm.ChatMessage{{Role: "user", Content: prompt}})
		cancel()
		r := observation{Case: fixture.Name, Model: profile.Model, Seconds: time.Since(start).Seconds(), PromptVersion: semanticAnalysisPromptVersion, MaxOutputTokens: profile.MaxTokens}
		if callErr != nil {
			r.Failure = "transport failure"
		} else {
			switch reply.Choices[0].FinishReason {
			case "stop", "length", "content_filter", "tool_calls", "function_call":
				r.Finish = reply.Choices[0].FinishReason
			default:
				r.Finish = "unknown"
			}
			r.Tokens = reply.Usage.CompletionTokens
			parsed, parseErr := parseSemanticAnalysis(reply.Choices[0].Message.Content, *target, fixture.Source)
			var wire semanticAnalysisWireResponse
			if decodeSemanticAnalysis(reply.Choices[0].Message.Content, &wire) == nil {
				_, explanationErr := normalizeSymbolExplanations(wire.SymbolExplanations, target.Symbols)
				r.ExplanationsOmitted = explanationErr != nil
			}
			r.Valid = parseErr == nil
			if parseErr != nil {
				r.Failure = "semantic contract rejected"
			} else {
				r.InsightPresent = parsed.EngineeringInsight != nil
			}
		}
		records = append(records, r)
		bytes, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("../../.mini-orca/autopilot/insight-live-evaluation.json", bytes, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("case=%s valid=%t finish=%s tokens=%d seconds=%.1f", r.Case, r.Valid, r.Finish, r.Tokens, r.Seconds)
	}
}
