package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type fakeEngineeringInsightClient struct {
	calls  atomic.Int32
	reply  string
	err    error
	prompt string
}

func (c *fakeEngineeringInsightClient) Chat(_ context.Context, messages []llm.ChatMessage) (*llm.ChatResponse, error) {
	c.calls.Add(1)
	if len(messages) != 1 || !strings.Contains(messages[0].Content, "TARGET_SOURCE") {
		return nil, errors.New("runner did not use the selected-file production prompt")
	}
	c.prompt = messages[0].Content
	if c.err != nil {
		return nil, c.err
	}
	return &llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", Content: c.reply}, FinishReason: "stop"}}, Usage: llm.ChatUsage{CompletionTokens: 7}}, nil
}

func runnerPromptContextManifest(t *testing.T, prompt string) project.ContextManifest {
	t.Helper()
	const prefix = "CONTEXT_MANIFEST:\n"
	start := strings.Index(prompt, prefix)
	end := strings.Index(prompt, "\n\nTARGET_FACTS:")
	if start < 0 || end < start+len(prefix) {
		t.Fatal("runner prompt has no context manifest")
	}
	var manifest project.ContextManifest
	if err := json.Unmarshal([]byte(prompt[start+len(prefix):end]), &manifest); err != nil {
		t.Fatalf("decode runner context manifest: %v", err)
	}
	return manifest
}

func TestEngineeringInsightRunnerPersistsSourceFreeSixRequestDevelopmentCampaign(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := runnerTestConfig(t, "development-one", EngineeringInsightCollectRunMode, 3, client)
	receipt, handoff, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls.Load() != 3 || receipt.Consumption.Requests != 3 || len(handoff.Responses) != 3 {
		t.Fatalf("calls=%d receipt=%+v handoff=%d", client.calls.Load(), receipt.Consumption, len(handoff.Responses))
	}
	promptManifest := runnerPromptContextManifest(t, client.prompt)
	wantManifest := bindContextManifestModel(project.ContextManifest{ByteLimit: maxSemanticAnalysisBytes, TokenLimit: maxSemanticAnalysisBytes / 4}, EffectiveModel{Scope: string(cfg.Profile.Scope), Model: cfg.Profile.Model, ProviderOrigin: providerOrigin(cfg.Profile.APIBaseURL), RemoteProvider: !isLoopbackURL(cfg.Profile.APIBaseURL), ContextMaxTokens: cfg.Profile.ContextMaxTokens})
	if promptManifest.Scope != wantManifest.Scope || promptManifest.Model != wantManifest.Model || promptManifest.ProviderOrigin != wantManifest.ProviderOrigin || promptManifest.RemoteProvider != wantManifest.RemoteProvider || promptManifest.ByteLimit != wantManifest.ByteLimit || promptManifest.TokenLimit != wantManifest.TokenLimit {
		t.Fatalf("runner prompt manifest = %+v, want production binding %+v", promptManifest, wantManifest)
	}
	manifest, err := os.ReadFile(filepath.Join(cfg.Root, runnerRelativeDirectory, cfg.RunID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"fixture-secret", "provider-secret", "model reply is private"} {
		if strings.Contains(string(manifest), forbidden) {
			t.Fatalf("manifest retained private data %q: %s", forbidden, manifest)
		}
	}
	reloaded, err := LoadEngineeringInsightScoringHandoff(cfg.Root, cfg.RunID)
	if err != nil || len(reloaded.Responses) != 3 {
		t.Fatalf("reload private scoring handoff: %d, %v", len(reloaded.Responses), err)
	}
	if err := os.RemoveAll(reloaded.privateDir); err != nil {
		t.Fatal(err)
	}
	handoff.Discard()
	if len(handoff.Responses) != 0 {
		t.Fatal("private scoring handoff was not discarded")
	}
	second := runnerTestConfig(t, "development-two", EngineeringInsightDevelopmentRunMode, 3, client)
	second.Root = cfg.Root
	if _, _, err = RunEngineeringInsightEvaluation(context.Background(), second); err != nil || client.calls.Load() != 6 {
		t.Fatalf("second reviewed development run failed: %v, calls=%d", err, client.calls.Load())
	}
	third := runnerTestConfig(t, "development-three", EngineeringInsightDevelopmentRunMode, 3, client)
	third.Root = cfg.Root
	_, _, err = RunEngineeringInsightEvaluation(context.Background(), third)
	if err == nil || !strings.Contains(err.Error(), "budget") || client.calls.Load() != 6 {
		t.Fatalf("aggregate campaign budget did not block a new run: %v, calls=%d", err, client.calls.Load())
	}
}

func TestEngineeringInsightRunnerRequiresExplicitDevelopmentBudgetGrantForSecondSixRequests(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	first := runnerTestConfig(t, "development-one", EngineeringInsightCollectRunMode, 3, client)
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := runnerTestConfig(t, "development-two", EngineeringInsightDevelopmentRunMode, 3, client)
	second.Root = first.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), second); err != nil || client.calls.Load() != 6 {
		t.Fatalf("original development budget = %v, calls=%d", err, client.calls.Load())
	}
	third := runnerTestConfig(t, "development-three", EngineeringInsightDevelopmentRunMode, 3, client)
	third.Root = first.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), third); err == nil || client.calls.Load() != 6 {
		t.Fatalf("ungranted extension dispatched: %v, calls=%d", err, client.calls.Load())
	}
	if err := GrantEngineeringInsightDevelopmentBudget(first.Root, "extension-one", 6); err != nil {
		t.Fatal(err)
	}
	if err := GrantEngineeringInsightDevelopmentBudget(first.Root, "extension-one", 6); err == nil {
		t.Fatal("duplicate development budget grant was accepted")
	}
	campaign, err := loadEvaluationCampaign(filepath.Join(first.Root, runnerRelativeDirectory, "campaign.json"))
	if err != nil || campaign.DevelopmentRequests != 6 || campaign.QualificationRequests != 0 {
		t.Fatalf("grant changed campaign counters: %+v, %v", campaign, err)
	}
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), third); err != nil || client.calls.Load() != 9 {
		t.Fatalf("first granted run = %v, calls=%d", err, client.calls.Load())
	}
	fourth := runnerTestConfig(t, "development-four", EngineeringInsightDevelopmentRunMode, 3, client)
	fourth.Root = first.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), fourth); err != nil || client.calls.Load() != 12 {
		t.Fatalf("second granted run = %v, calls=%d", err, client.calls.Load())
	}
	fifth := runnerTestConfig(t, "development-five", EngineeringInsightDevelopmentRunMode, 3, client)
	fifth.Root = first.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), fifth); err == nil || client.calls.Load() != 12 {
		t.Fatalf("development requests exceeded twelve: %v, calls=%d", err, client.calls.Load())
	}
	campaign, err = loadEvaluationCampaign(filepath.Join(first.Root, runnerRelativeDirectory, "campaign.json"))
	if err != nil || campaign.DevelopmentRequests != 12 || campaign.QualificationRequests != 0 {
		t.Fatalf("extended campaign counters: %+v, %v", campaign, err)
	}
}

func TestEngineeringInsightRunnerRejectsCorruptDevelopmentGrantLedgerBeforeDispatch(t *testing.T) {
	for name, ledger := range map[string]string{
		"unknown field":   `{"grants":[{"authorization_id":"extension-one","requests":6}],"unexpected":true}`,
		"duplicate field": `{"grants":[],"grants":[{"authorization_id":"extension-one","requests":6}]}`,
		"empty grants":    `{"grants":[]}`,
		"wrong requests":  `{"grants":[{"authorization_id":"extension-one","requests":5}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
			cfg := runnerTestConfig(t, "corrupt-grant", EngineeringInsightCollectRunMode, 3, client)
			directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: 6}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(developmentGrantLedgerPath(directory), []byte(ledger), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err == nil || client.calls.Load() != 0 {
				t.Fatalf("corrupt grant ledger dispatched: %v, calls=%d", err, client.calls.Load())
			}
			if err := GrantEngineeringInsightDevelopmentBudget(cfg.Root, "extension-two", 6); err == nil {
				t.Fatal("corrupt grant ledger accepted another grant")
			}
		})
	}
}

func TestEngineeringInsightRunnerStopsBeforeDispatchWhenReservationPersistenceFails(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := runnerTestConfig(t, "persistence", EngineeringInsightCollectRunMode, 3, client)
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(filepath.Join(directory, "campaign.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err == nil || client.calls.Load() != 0 {
		t.Fatalf("reservation persistence failure dispatched a request: %v, calls=%d", err, client.calls.Load())
	}
}

func TestEngineeringInsightRunnerRejectsCorruptCampaignAndChangedFixtureOnResume(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := runnerTestConfig(t, "corrupt", EngineeringInsightCollectRunMode, 3, client)
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "campaign.json"), []byte(`{"development_requests":-1,"qualification_requests":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err == nil || client.calls.Load() != 0 {
		t.Fatalf("corrupt campaign was accepted: %v calls=%d", err, client.calls.Load())
	}
	if err := os.Remove(filepath.Join(directory, "campaign.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Cases[0].Source += "// changed\n"
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err == nil || client.calls.Load() != 3 {
		t.Fatalf("changed fixture resumed a frozen run: %v calls=%d", err, client.calls.Load())
	}
}

func TestEngineeringInsightRunnerResumesReservedAttemptAsConsumedWithoutReplay(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := runnerTestConfig(t, "resume", EngineeringInsightCollectRunMode, 3, client)
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := engineeringInsightRunManifest{Fingerprint: runnerFingerprint(cfg), Receipt: runnerReceipt(cfg)}
	for _, item := range cfg.Cases {
		manifest.Receipt.Attempts = append(manifest.Receipt.Attempts, unknownReservedAttempt(item.Expected, cfg.CandidateID))
	}
	manifest.Receipt.Consumption.Requests = 3
	if err := writeRunnerJSON(filepath.Join(directory, cfg.RunID+".json"), manifest); err != nil {
		t.Fatal(err)
	}
	if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: 3}); err != nil {
		t.Fatal(err)
	}
	receipt, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil || client.calls.Load() != 0 || receipt.Attempts[0].Outcome != "unknown" {
		t.Fatalf("resume replayed unknown request: receipt=%+v calls=%d err=%v", receipt, client.calls.Load(), err)
	}
}

func TestEngineeringInsightRunnerRejectsConcurrentAndUnconfirmedRemoteRuns(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := runnerTestConfig(t, "locked", EngineeringInsightCollectRunMode, 3, client)
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	lock, err := lockRunnerFile(filepath.Join(directory, "campaign.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); !errors.Is(err, ErrEngineeringInsightRunLocked) || client.calls.Load() != 0 {
		t.Fatalf("concurrent run = %v, calls=%d", err, client.calls.Load())
	}
	lock.Close()
	cfg.Profile.APIBaseURL = "https://provider.example/v1"
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); !errors.Is(err, ErrEngineeringInsightRemoteDenied) || client.calls.Load() != 0 {
		t.Fatalf("remote run = %v, calls=%d", err, client.calls.Load())
	}
}

func TestEngineeringInsightRunnerRejectsPathLikeRunIDsBeforeStateAccess(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := runnerTestConfig(t, "safe-run", EngineeringInsightCollectRunMode, 3, client)
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	campaign := filepath.Join(directory, "campaign.json")
	campaignData, err := os.ReadFile(campaign)
	if err != nil {
		t.Fatal(err)
	}
	hostileFiles := []string{"...json", "..json"}
	for _, name := range hostileFiles {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(`{"finished":true}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, runID := range []string{".", "..", "a/b", "a.b"} {
		bad := cfg
		bad.RunID = runID
		if _, _, err := RunEngineeringInsightEvaluation(context.Background(), bad); err == nil {
			t.Fatalf("run ID %q accepted", runID)
		}
		if _, err := LoadEngineeringInsightScoringHandoff(cfg.Root, runID); err == nil {
			t.Fatalf("handoff accepted %q", runID)
		}
		if _, err := StoreEngineeringInsightScores(cfg.Root, runID, nil); err == nil {
			t.Fatalf("score accepted %q", runID)
		}
		if _, err := LoadEngineeringInsightEvaluationReceipt(cfg.Root, runID); err == nil {
			t.Fatalf("export accepted %q", runID)
		}
		(&EngineeringInsightScoringHandoff{RunID: runID, Responses: map[string]string{"private": "reply"}, privateDir: directory}).Discard()
		if data, err := os.ReadFile(campaign); err != nil || string(data) != string(campaignData) {
			t.Fatalf("campaign changed for %q: %v", runID, err)
		}
		if _, err := os.Stat(filepath.Join(directory, runID+".lock")); !os.IsNotExist(err) {
			t.Fatalf("invalid ID %q created a lock: %v", runID, err)
		}
	}
	for _, name := range hostileFiles {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("hostile manifest %q changed: %v", name, err)
		}
	}
	if client.calls.Load() != 3 {
		t.Fatalf("invalid IDs dispatched calls=%d", client.calls.Load())
	}
}

func TestEngineeringInsightRunnerStopsSecondQualificationRunAtAggregateCap(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := qualificationRunnerTestConfig(t, "qualification-one", client)
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if client.calls.Load() != 24 {
		t.Fatalf("qualification calls=%d", client.calls.Load())
	}
	next := qualificationRunnerTestConfig(t, "qualification-two", client)
	next.Root = cfg.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), next); err == nil || !strings.Contains(err.Error(), "budget") || client.calls.Load() != 24 {
		t.Fatalf("second qualification run bypassed cap: %v calls=%d", err, client.calls.Load())
	}
}

func TestEngineeringInsightRunnerStoresOnlyDigestBoundScores(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := runnerTestConfig(t, "scores", EngineeringInsightCollectRunMode, 3, client)
	receipt, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	id := expectedAttemptID(cfg.Cases[0].Expected)
	score := EngineeringInsightAttemptScore{ResponseDigest: receipt.Attempts[0].ResponseDigest, Correctness: 2, LocalRelevance: 2, TradeoffClarity: 2, UsefulVerification: 2}
	updated, err := StoreEngineeringInsightScores(cfg.Root, cfg.RunID, map[string]EngineeringInsightAttemptScore{id: score})
	if err != nil || updated.Attempts[0].Score == nil {
		t.Fatalf("store score = %+v, %v", updated, err)
	}
	score.ResponseDigest = strings.Repeat("0", 64)
	if _, err := StoreEngineeringInsightScores(cfg.Root, cfg.RunID, map[string]EngineeringInsightAttemptScore{id: score}); err == nil {
		t.Fatal("mismatched score digest was accepted")
	}
	data, err := os.ReadFile(filepath.Join(cfg.Root, runnerRelativeDirectory, cfg.RunID+".json"))
	if err != nil || !json.Valid(data) || strings.Contains(string(data), "model reply is private") {
		t.Fatalf("scored manifest is not source-free: %v %s", err, data)
	}
}

func TestEngineeringInsightRunnerRejectsIncompleteFileInsightsAcrossOptionalLocations(t *testing.T) {
	target := project.IndexFile{Path: "fixture.go", Language: "Go"}
	source := "package fixture\nfunc Run() {}\n"
	response := func(location, insight string) string {
		switch location {
		case "top-level":
			return `{"purpose":"Summarizes.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{},"engineering_insight":` + insight + `}`
		case "risk":
			return `{"purpose":"Summarizes.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"low","summary":"Conditional.","engineering_insight":` + insight + `}],"suggestions":[],"symbol_explanations":{}}`
		case "suggestion":
			return `{"purpose":"Summarizes.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[{"title":"Keep behavior","summary":"No change.","engineering_insight":` + insight + `}],"symbol_explanations":{}}`
		default:
			return ""
		}
	}

	for name, insight := range map[string]string{
		"missing trade-off":    `{"mechanism":"Run returns.","why_it_matters_here":"The local function returns.","transferable_lesson":"Call Run and expect its result."}`,
		"missing verification": `{"mechanism":"Run returns.","why_it_matters_here":"The local function returns.","tradeoff_or_failure_mode":"Changing it alters the returned value."}`,
	} {
		for _, location := range []string{"top-level", "risk", "suggestion"} {
			t.Run(name+"/"+location, func(t *testing.T) {
				content := response(location, insight)
				parsed, err := parseSemanticAnalysis(content, target, source)
				if err != nil {
					t.Fatal(err)
				}
				optional, degraded := evaluationOptionalState(content, target, source, parsed)
				if optional != "rejected" || !degraded {
					t.Fatalf("incomplete %s insight state = %q, degraded=%t", location, optional, degraded)
				}
			})
		}
	}

	client := &fakeEngineeringInsightClient{reply: response("top-level", `{"mechanism":"Run returns.","why_it_matters_here":"The local function returns.","transferable_lesson":"Call Run and expect its result."}`)}
	cfg := runnerTestConfig(t, "incomplete-control", EngineeringInsightCollectRunMode, 3, client)
	receipt, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	control := receipt.Attempts[2]
	if !control.UsableSummary || control.CompleteSummary || !control.OptionalSectionDegraded || control.OptionalInsight != "rejected" {
		t.Fatalf("control attempt treated incomplete insight as complete or omitted: %+v", control)
	}
	report := EngineeringInsightEvaluationReport{}
	recordScoreCounts(&report, cfg.Cases[2].Expected, control)
	if report.OmittedControls != 0 {
		t.Fatalf("incomplete control insight counted as intentional omission: %+v", report)
	}
}

func runnerTestConfig(t *testing.T, runID, mode string, count int, client EngineeringInsightRunnerClient) EngineeringInsightRunnerConfig {
	t.Helper()
	cases := make([]EngineeringInsightRunnerCase, 0, count)
	for index := 0; index < count; index++ {
		caseIndex := index % 3
		intent := "substantive"
		if caseIndex == 2 {
			intent = "control"
		}
		cases = append(cases, EngineeringInsightRunnerCase{Expected: EngineeringInsightExpectedAttempt{CaseName: "case-" + string(rune('a'+caseIndex)), Partition: "development", Intent: intent, Repetition: index/3 + 1, Attempt: 1}, Source: "package fixture\n\n// fixture-secret\nfunc Run() {}\n"})
	}
	return EngineeringInsightRunnerConfig{Root: t.TempDir(), RunID: runID, Mode: mode, CandidateID: "candidate", Provider: "configured", Model: "model", PromptVersion: semanticAnalysisPromptVersion, CorpusID: "corpus", CorpusDigest: strings.Repeat("a", 64), BaseRevision: "base", Profile: config.ModelProfile{Scope: config.BugModelScope, APIBaseURL: "http://127.0.0.1:9999", APIKey: "provider-secret", Model: "model", ContextMaxTokens: 32000}, Cases: cases, Client: client}
}

func qualificationRunnerTestConfig(t *testing.T, runID string, client EngineeringInsightRunnerClient) EngineeringInsightRunnerConfig {
	t.Helper()
	cases := make([]EngineeringInsightRunnerCase, 0, 24)
	for repetition := 1; repetition <= 2; repetition++ {
		for index := 0; index < 12; index++ {
			intent := "substantive"
			if index >= 8 {
				intent = "control"
			}
			cases = append(cases, EngineeringInsightRunnerCase{Expected: EngineeringInsightExpectedAttempt{CaseName: "qualification-" + string(rune('a'+index)), Partition: "qualification", Intent: intent, Repetition: repetition, Attempt: 1}, Source: "package fixture\nfunc Run() {}\n"})
		}
	}
	return EngineeringInsightRunnerConfig{Root: t.TempDir(), RunID: runID, Mode: EngineeringInsightQualificationRunMode, CandidateID: "candidate", Provider: "configured", Model: "model", PromptVersion: semanticAnalysisPromptVersion, CorpusID: "corpus", CorpusDigest: strings.Repeat("a", 64), BaseRevision: "base", Profile: config.ModelProfile{Scope: config.BugModelScope, APIBaseURL: "http://127.0.0.1:9999", Model: "model", ContextMaxTokens: 32000}, Cases: cases, Client: client}
}

func runnerValidResponse() string {
	return `{"purpose":"model reply is private","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}}`
}
