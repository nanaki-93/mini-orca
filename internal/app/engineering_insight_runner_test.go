package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	calls    atomic.Int32
	reply    string
	response *llm.ChatResponse
	err      error
	prompt   string
	schema   llm.JSONSchema
}

func (c *fakeEngineeringInsightClient) ChatWithJSONSchema(_ context.Context, messages []llm.ChatMessage, schema llm.JSONSchema) (*llm.ChatResponse, error) {
	c.calls.Add(1)
	if len(messages) != 1 || !strings.Contains(messages[0].Content, "TARGET_SOURCE") {
		return nil, errors.New("runner did not use the selected-file production prompt")
	}
	c.prompt = messages[0].Content
	c.schema = schema
	if c.err != nil {
		return nil, c.err
	}
	if c.response != nil {
		return c.response, nil
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
	productionSchema, err := FileAnalysisResponseSchema().Identity()
	if err != nil {
		t.Fatal(err)
	}
	runnerSchema, err := client.schema.Identity()
	if err != nil || runnerSchema != productionSchema {
		t.Fatalf("runner schema identity = %q, %v; want production %q", runnerSchema, err, productionSchema)
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

func TestEngineeringInsightRunnerParsesFinalContentWithoutReasoningPrefix(t *testing.T) {
	const reasoningContent = "private reasoning_content"
	const reasoning = "private reasoning"
	client := &fakeEngineeringInsightClient{response: &llm.ChatResponse{Model: "model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", Content: runnerValidResponse(), ReasoningContent: reasoningContent, Reasoning: reasoning}, FinishReason: "stop"}}, Usage: llm.ChatUsage{CompletionTokens: 7}}}
	cfg := runnerTestConfig(t, "reasoning-with-final", EngineeringInsightCollectRunMode, 3, client)
	receipt, handoff, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, attempt := range receipt.Attempts {
		if attempt.Outcome != "completed" || !attempt.UsableSummary || !attempt.EmittedResponse {
			t.Fatalf("final JSON was not parsed independently of reasoning: %+v", attempt)
		}
	}
	for _, response := range handoff.Responses {
		if !strings.Contains(response, reasoningContent) || !strings.Contains(response, reasoning) || !strings.Contains(response, runnerValidResponse()) {
			t.Fatalf("private scoring handoff omitted emitted material: %q", response)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(cfg.Root, runnerRelativeDirectory, cfg.RunID+".json"))
	if err != nil || strings.Contains(string(manifest), reasoningContent) || strings.Contains(string(manifest), reasoning) {
		t.Fatalf("source-free receipt retained reasoning: %s, %v", manifest, err)
	}
	handoff.Discard()
}

func TestEngineeringInsightRunnerRetainsReasoningOnlyLengthResponseForPrivateScoring(t *testing.T) {
	const reasoning = "private length-limited reasoning"
	client := &fakeEngineeringInsightClient{response: &llm.ChatResponse{Model: "model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", ReasoningContent: reasoning}, FinishReason: "length"}}, Usage: llm.ChatUsage{CompletionTokens: 31}}}
	cfg := runnerTestConfig(t, "reasoning-only-length", EngineeringInsightCollectRunMode, 3, client)
	receipt, handoff, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls.Load() != 3 || receipt.Consumption.Requests != 3 || receipt.Consumption.OutputTokens != 93 {
		t.Fatalf("consumption did not retain reserved attempts and usage: calls=%d receipt=%+v", client.calls.Load(), receipt.Consumption)
	}
	for _, attempt := range receipt.Attempts {
		if attempt.Outcome != "truncated" || attempt.FinishReason != "length" || attempt.OutputTokens != 31 || !attempt.EmittedResponse || attempt.UsableSummary {
			t.Fatalf("reasoning-only response lost failure metadata: %+v", attempt)
		}
	}
	schedule := make([]EngineeringInsightExpectedAttempt, 0, len(cfg.Cases))
	for _, item := range cfg.Cases {
		schedule = append(schedule, item.Expected)
	}
	if _, err := ValidateEngineeringInsightEvaluationReceipt(receipt, EngineeringInsightEvaluationExpectation{CandidateID: cfg.CandidateID, Provider: cfg.Provider, Model: cfg.Model, PromptVersion: cfg.PromptVersion, CorpusID: cfg.CorpusID, CorpusDigest: cfg.CorpusDigest, BaseRevision: cfg.BaseRevision, MaxRequests: 6, MaxOutputTokens: qualificationOutputTokenCap, AttemptTimeoutSeconds: qualificationAttemptTimeoutSeconds, Schedule: schedule}); err != nil {
		t.Fatalf("receipt rejected reasoning-only digest evidence: %v", err)
	}
	for _, response := range handoff.Responses {
		if response != reasoning {
			t.Fatalf("private scoring material = %q, want reasoning", response)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(cfg.Root, runnerRelativeDirectory, cfg.RunID+".json"))
	if err != nil || strings.Contains(string(manifest), reasoning) {
		t.Fatalf("source-free receipt retained private reasoning: %s, %v", manifest, err)
	}
	reloaded, err := LoadEngineeringInsightScoringHandoff(cfg.Root, cfg.RunID)
	if err != nil || len(reloaded.Responses) != 3 {
		t.Fatalf("reasoning-only private handoff did not reload: %d, %v", len(reloaded.Responses), err)
	}
	reloaded.Discard()
	handoff.Discard()
}

func TestEngineeringInsightRunnerRetainsReasoningOnlyStoppedResponseForPrivateScoring(t *testing.T) {
	const reasoning = "private stopped reasoning"
	client := &fakeEngineeringInsightClient{response: &llm.ChatResponse{Model: "model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", Reasoning: reasoning}, FinishReason: "stop"}}, Usage: llm.ChatUsage{CompletionTokens: 29}}}
	cfg := runnerTestConfig(t, "reasoning-only-stop", EngineeringInsightCollectRunMode, 3, client)
	receipt, handoff, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls.Load() != 3 || receipt.Consumption.Requests != 3 || receipt.Consumption.OutputTokens != 87 {
		t.Fatalf("stopped reasoning-only response retried or lost consumption: calls=%d receipt=%+v", client.calls.Load(), receipt.Consumption)
	}
	digest := sha256.Sum256([]byte(reasoning))
	wantDigest := hex.EncodeToString(digest[:])
	for _, attempt := range receipt.Attempts {
		if attempt.Outcome != "malformed" || attempt.FinishReason != "stop" || attempt.OutputTokens != 29 || !attempt.EmittedResponse || attempt.ResponseDigest != wantDigest || attempt.UsableSummary {
			t.Fatalf("stopped reasoning-only response lost metadata: %+v", attempt)
		}
	}
	schedule := make([]EngineeringInsightExpectedAttempt, 0, len(cfg.Cases))
	for _, item := range cfg.Cases {
		schedule = append(schedule, item.Expected)
	}
	if _, err := ValidateEngineeringInsightEvaluationReceipt(receipt, EngineeringInsightEvaluationExpectation{CandidateID: cfg.CandidateID, Provider: cfg.Provider, Model: cfg.Model, PromptVersion: cfg.PromptVersion, CorpusID: cfg.CorpusID, CorpusDigest: cfg.CorpusDigest, BaseRevision: cfg.BaseRevision, MaxRequests: 6, MaxOutputTokens: qualificationOutputTokenCap, AttemptTimeoutSeconds: qualificationAttemptTimeoutSeconds, Schedule: schedule}); err != nil {
		t.Fatalf("receipt rejected stopped reasoning-only digest evidence: %v", err)
	}
	for _, response := range handoff.Responses {
		if response != reasoning {
			t.Fatalf("private scoring material = %q, want reasoning", response)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(cfg.Root, runnerRelativeDirectory, cfg.RunID+".json"))
	if err != nil || strings.Contains(string(manifest), reasoning) {
		t.Fatalf("source-free receipt retained private reasoning: %s, %v", manifest, err)
	}
	handoff.Discard()
}

func TestEngineeringInsightRunnerFingerprintBindsOptionalSamplingControls(t *testing.T) {
	cfg := runnerTestConfig(t, "sampling-fingerprint", EngineeringInsightCollectRunMode, 3, &fakeEngineeringInsightClient{reply: runnerValidResponse()})
	baseline := runnerFingerprint(cfg)
	topP := float32(0)
	cfg.Profile.TopP = &topP
	withTopP := runnerFingerprint(cfg)
	if baseline == withTopP {
		t.Fatal("omitted and explicit zero top_p have the same fingerprint")
	}
	topP = 0.95
	changed := runnerFingerprint(cfg)
	if withTopP == changed {
		t.Fatal("top_p value change did not change fingerprint")
	}
	cloneValue := float32(0.95)
	cfg.Profile.TopP = &cloneValue
	if changed == baseline || runnerFingerprint(cfg) != changed {
		t.Fatal("sampling fingerprint did not deterministically bind the configured value")
	}
}

func TestEngineeringInsightRunnerFingerprintBindsResponseSchemaIdentity(t *testing.T) {
	cfg := runnerTestConfig(t, "schema-fingerprint", EngineeringInsightCollectRunMode, 3, &fakeEngineeringInsightClient{reply: runnerValidResponse()})
	identity, err := FileAnalysisResponseSchema().Identity()
	if err != nil {
		t.Fatal(err)
	}
	if runnerFingerprintWithSchemaIdentity(cfg, identity) == runnerFingerprintWithSchemaIdentity(cfg, identity+"-changed") {
		t.Fatal("response schema identity did not change the resumable-run fingerprint")
	}
}

func TestEngineeringInsightRunnerTerminatesAfterPermanentStructuredRequestRejection(t *testing.T) {
	for _, test := range []struct {
		name                               string
		config                             func(*fakeEngineeringInsightClient) EngineeringInsightRunnerConfig
		wantDevelopment, wantQualification int
	}{
		{name: "development", config: func(client *fakeEngineeringInsightClient) EngineeringInsightRunnerConfig {
			return runnerTestConfig(t, "permanent-development", EngineeringInsightDevelopmentRunMode, 3, client)
		}, wantDevelopment: 1},
		{name: "qualification", config: func(client *fakeEngineeringInsightClient) EngineeringInsightRunnerConfig {
			return qualificationRunnerTestConfig(t, "permanent-qualification", client)
		}, wantQualification: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeEngineeringInsightClient{err: llm.ErrStructuredRequestRejected}
			cfg := test.config(client)
			receipt, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
			if !errors.Is(err, ErrEngineeringInsightRunTerminated) || client.calls.Load() != 1 || receipt.Consumption.Requests != 1 || len(receipt.Attempts) != 1 || receipt.Attempts[0].Outcome != "failed" || receipt.Attempts[0].FinishReason != "error" {
				t.Fatalf("permanent rejection result: receipt=%+v calls=%d err=%v", receipt, client.calls.Load(), err)
			}
			directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
			campaign, err := loadEvaluationCampaign(filepath.Join(directory, "campaign.json"))
			if err != nil || campaign.DevelopmentRequests != test.wantDevelopment || campaign.QualificationRequests != test.wantQualification {
				t.Fatalf("campaign after permanent rejection: %+v, %v", campaign, err)
			}
			manifest, exists, err := loadRunManifest(filepath.Join(directory, cfg.RunID+".json"))
			if err != nil || !exists || !manifest.Finished || manifest.TerminalReason != permanentRequestRejectionReason {
				t.Fatalf("terminal manifest: %+v exists=%t err=%v", manifest, exists, err)
			}
			if exported, err := LoadEngineeringInsightEvaluationReceipt(cfg.Root, cfg.RunID); err != nil || exported.Consumption.Requests != 1 {
				t.Fatalf("terminal receipt export: %+v, %v", exported, err)
			}
			if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); !errors.Is(err, ErrEngineeringInsightRunTerminated) || client.calls.Load() != 1 {
				t.Fatalf("terminal run resumed dispatch: calls=%d err=%v", client.calls.Load(), err)
			}
			campaign, err = loadEvaluationCampaign(filepath.Join(directory, "campaign.json"))
			if err != nil || campaign.DevelopmentRequests != test.wantDevelopment || campaign.QualificationRequests != test.wantQualification {
				t.Fatalf("terminal resume changed campaign: %+v, %v", campaign, err)
			}
		})
	}
}

func TestEngineeringInsightRunnerContinuesAfterMalformedModelContent(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: "not JSON"}
	cfg := runnerTestConfig(t, "malformed-content", EngineeringInsightCollectRunMode, 3, client)
	receipt, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil || client.calls.Load() != 3 || len(receipt.Attempts) != 3 {
		t.Fatalf("malformed content collection: receipt=%+v calls=%d err=%v", receipt, client.calls.Load(), err)
	}
	for _, attempt := range receipt.Attempts {
		if attempt.Outcome != "malformed" {
			t.Fatalf("malformed content stopped or changed outcome: %+v", attempt)
		}
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

func TestEngineeringInsightRunnerAppliesBoundQwenRecoveryGrantOnlyAfterTwelveRequests(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	first := runnerTestConfig(t, "development-one", EngineeringInsightCollectRunMode, 3, client)
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := runnerTestConfig(t, "development-two", EngineeringInsightDevelopmentRunMode, 3, client)
	second.Root = first.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := GrantEngineeringInsightDevelopmentBudget(first.Root, "extension-one", 6); err != nil {
		t.Fatal(err)
	}
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(first.Root, recoveryDevelopmentAuthorizationID, 6, recoveryDevelopmentCandidateID, recoveryDevelopmentModel); err == nil {
		t.Fatal("second recovery grant was accepted before twelve requests")
	}
	for _, runID := range []string{"development-three", "development-four"} {
		cfg := runnerTestConfig(t, runID, EngineeringInsightDevelopmentRunMode, 3, client)
		cfg.Root = first.Root
		if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err != nil {
			t.Fatalf("complete first recovery %q: %v", runID, err)
		}
	}
	if client.calls.Load() != 12 {
		t.Fatalf("first two development grants dispatched %d requests, want 12", client.calls.Load())
	}
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(first.Root, recoveryDevelopmentAuthorizationID, 6, "wrong-candidate", recoveryDevelopmentModel); err == nil {
		t.Fatal("recovery grant accepted the wrong candidate")
	}
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(first.Root, recoveryDevelopmentAuthorizationID, 6, recoveryDevelopmentCandidateID, "wrong-model"); err == nil {
		t.Fatal("recovery grant accepted the wrong model")
	}
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(first.Root, recoveryDevelopmentAuthorizationID, 6, recoveryDevelopmentCandidateID, recoveryDevelopmentModel); err != nil {
		t.Fatal(err)
	}
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(first.Root, recoveryDevelopmentAuthorizationID, 6, recoveryDevelopmentCandidateID, recoveryDevelopmentModel); err == nil {
		t.Fatal("recovery grant replay was accepted")
	}

	wrongCandidate := recoveryRunnerTestConfig(t, "recovery-wrong-candidate", client)
	wrongCandidate.Root = first.Root
	wrongCandidate.CandidateID = "another-candidate"
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), wrongCandidate); err == nil || client.calls.Load() != 12 {
		t.Fatalf("wrong recovery candidate dispatched: %v, calls=%d", err, client.calls.Load())
	}
	wrongModel := recoveryRunnerTestConfig(t, "recovery-wrong-model", client)
	wrongModel.Root = first.Root
	wrongModel.Profile.Model = "another-model"
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), wrongModel); err == nil || client.calls.Load() != 12 {
		t.Fatalf("wrong recovery model dispatched: %v, calls=%d", err, client.calls.Load())
	}
	wrongDestination := recoveryRunnerTestConfig(t, "recovery-wrong-destination", client)
	wrongDestination.Root = first.Root
	wrongDestination.Profile.APIBaseURL = "https://provider.example/v1"
	wrongDestination.ConfirmRemoteProvider = true
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), wrongDestination); err == nil || client.calls.Load() != 12 {
		t.Fatalf("remote recovery destination dispatched: %v, calls=%d", err, client.calls.Load())
	}
	for _, runID := range []string{"recovery-one", "recovery-two"} {
		cfg := recoveryRunnerTestConfig(t, runID, client)
		cfg.Root = first.Root
		if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); err != nil {
			t.Fatalf("bound recovery %q: %v", runID, err)
		}
	}
	final := recoveryRunnerTestConfig(t, "recovery-exhausted", client)
	final.Root = first.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), final); err == nil || client.calls.Load() != 18 {
		t.Fatalf("recovery cap was not exhausted: %v, calls=%d", err, client.calls.Load())
	}
	campaign, err := loadEvaluationCampaign(filepath.Join(first.Root, runnerRelativeDirectory, "campaign.json"))
	if err != nil || campaign.DevelopmentRequests != 18 || campaign.QualificationRequests != 0 {
		t.Fatalf("recovery campaign counters: %+v, %v", campaign, err)
	}
}

func TestEngineeringInsightRunnerRejectsRecoveryAuthorizationAsFirstGrant(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: defaultDevelopmentRequestCap}); err != nil {
		t.Fatal(err)
	}
	if err := GrantEngineeringInsightDevelopmentBudget(root, recoveryDevelopmentAuthorizationID, developmentGrantRequestCount); err == nil {
		t.Fatal("unbound recovery authorization was accepted as first grant")
	}
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(root, recoveryDevelopmentAuthorizationID, developmentGrantRequestCount, recoveryDevelopmentCandidateID, recoveryDevelopmentModel); err == nil {
		t.Fatal("bound recovery authorization was accepted as first grant")
	}
	if _, exists, err := loadDevelopmentGrantLedger(directory); err != nil || exists {
		t.Fatalf("recovery authorization created a ledger: exists=%t err=%v", exists, err)
	}
}

func TestEngineeringInsightRunnerAppliesStructuredOutputGrantOnlyAtRequestsTwentyFourThroughTwentyNine(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := v12RunnerTestConfig(t, "v12-one", client)
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	ledger := engineeringInsightDevelopmentGrantLedger{Grants: []engineeringInsightDevelopmentGrant{
		{AuthorizationID: "extension-one", Requests: 6},
		{AuthorizationID: recoveryDevelopmentAuthorizationID, Requests: 6, CandidateID: recoveryDevelopmentCandidateID, Model: recoveryDevelopmentModel},
	}}
	if err := writeRunnerJSON(developmentGrantLedgerPath(directory), ledger); err != nil {
		t.Fatal(err)
	}
	if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: recoveryDevelopmentRequestCap}); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []engineeringInsightDevelopmentGrantIdentity{
		{CandidateID: "wrong-candidate", Model: v11DevelopmentModel, PromptVersion: v11DevelopmentPromptVersion},
		{CandidateID: v11DevelopmentCandidateID, Model: "wrong-model", PromptVersion: v11DevelopmentPromptVersion},
		{CandidateID: v11DevelopmentCandidateID, Model: v11DevelopmentModel, PromptVersion: "wrong-prompt"},
	} {
		if err := GrantEngineeringInsightV11DevelopmentBudget(cfg.Root, v11DevelopmentAuthorizationID, 6, identity.CandidateID, identity.Model, identity.PromptVersion); err == nil {
			t.Fatal("v11 grant accepted the wrong identity")
		}
	}
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(cfg.Root, v11DevelopmentAuthorizationID, 6, v11DevelopmentCandidateID, v11DevelopmentModel); err == nil {
		t.Fatal("unbound v11 grant was accepted")
	}
	if err := GrantEngineeringInsightV11DevelopmentBudget(cfg.Root, v11DevelopmentAuthorizationID, 6, v11DevelopmentCandidateID, v11DevelopmentModel, v11DevelopmentPromptVersion); err != nil {
		t.Fatal(err)
	}
	if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: finalDevelopmentRequestCap}); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []engineeringInsightDevelopmentGrantIdentity{
		{CandidateID: "wrong-candidate", Model: v12DevelopmentModel, PromptVersion: v12DevelopmentPromptVersion},
		{CandidateID: v12DevelopmentCandidateID, Model: "wrong-model", PromptVersion: v12DevelopmentPromptVersion},
		{CandidateID: v12DevelopmentCandidateID, Model: v12DevelopmentModel, PromptVersion: "wrong-prompt"},
	} {
		if err := GrantEngineeringInsightV12DevelopmentBudget(cfg.Root, v12DevelopmentAuthorizationID, 6, identity.CandidateID, identity.Model, identity.PromptVersion); err == nil {
			t.Fatal("structured-output grant accepted the wrong identity")
		}
	}
	if err := GrantEngineeringInsightV12DevelopmentBudget(cfg.Root, v12DevelopmentAuthorizationID, 6, v12DevelopmentCandidateID, v12DevelopmentModel, v12DevelopmentPromptVersion); err != nil {
		t.Fatal(err)
	}
	if err := GrantEngineeringInsightV12DevelopmentBudget(cfg.Root, "extra-grant", 6, v12DevelopmentCandidateID, v12DevelopmentModel, v12DevelopmentPromptVersion); err == nil {
		t.Fatal("fifth development grant was accepted")
	}

	wrongCandidate := v12RunnerTestConfig(t, "v12-wrong-candidate", client)
	wrongCandidate.Root = cfg.Root
	wrongCandidate.CandidateID = "another-candidate"
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), wrongCandidate); err == nil || client.calls.Load() != 0 {
		t.Fatalf("wrong v11 candidate dispatched: %v, calls=%d", err, client.calls.Load())
	}
	wrongModel := v12RunnerTestConfig(t, "v12-wrong-model", client)
	wrongModel.Root = cfg.Root
	wrongModel.Model = "another-model"
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), wrongModel); err == nil || client.calls.Load() != 0 {
		t.Fatalf("wrong v11 model dispatched: %v, calls=%d", err, client.calls.Load())
	}
	wrongProfile := v12RunnerTestConfig(t, "v12-wrong-profile", client)
	wrongProfile.Root = cfg.Root
	wrongProfile.Profile.Model = "another-model"
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), wrongProfile); err == nil || client.calls.Load() != 0 {
		t.Fatalf("wrong v11 profile model dispatched: %v, calls=%d", err, client.calls.Load())
	}
	wrongDestination := v12RunnerTestConfig(t, "v12-wrong-destination", client)
	wrongDestination.Root = cfg.Root
	wrongDestination.Profile.APIBaseURL = "https://provider.example/v1"
	wrongDestination.ConfirmRemoteProvider = true
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), wrongDestination); err == nil || client.calls.Load() != 0 {
		t.Fatalf("remote v11 destination dispatched: %v, calls=%d", err, client.calls.Load())
	}
	for _, runID := range []string{"v12-one", "v12-two"} {
		valid := v12RunnerTestConfig(t, runID, client)
		valid.Root = cfg.Root
		if _, _, err := RunEngineeringInsightEvaluation(context.Background(), valid); err != nil {
			t.Fatalf("v12 run %q: %v", runID, err)
		}
	}
	if client.calls.Load() != 6 {
		t.Fatalf("structured-output grant dispatched %d requests, want 6", client.calls.Load())
	}
	exhausted := v12RunnerTestConfig(t, "v12-exhausted", client)
	exhausted.Root = cfg.Root
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), exhausted); err == nil || client.calls.Load() != 6 {
		t.Fatalf("structured-output cap was not exhausted: %v, calls=%d", err, client.calls.Load())
	}
	campaign, err := loadEvaluationCampaign(filepath.Join(directory, "campaign.json"))
	if err != nil || campaign.DevelopmentRequests != structuredDevelopmentRequestCap || campaign.QualificationRequests != 0 {
		t.Fatalf("structured-output campaign counters: %+v, %v", campaign, err)
	}
}

func TestEngineeringInsightRunnerPreservesPersistedDevelopmentCampaignCaps(t *testing.T) {
	tests := []struct {
		name       string
		ledger     string
		consumed   int
		wantCap    int
		wantGrants int
		recovery   bool
		v11        bool
	}{
		{name: "original six-request campaign", consumed: 6, wantCap: 6},
		{name: "legacy twelve-request ledger", ledger: `{"grants":[{"authorization_id":"extension-one","requests":6}]}`, consumed: 12, wantCap: 12, wantGrants: 1},
		{name: "bound eighteen-request ledger", ledger: `{"grants":[{"authorization_id":"extension-one","requests":6},{"authorization_id":"qual05-qwen38-recovery-1","requests":6,"candidate_id":"qwen38-v10-recovery-1","model":"qwen/qwen3.8-27b"}]}`, consumed: 12, wantCap: 18, wantGrants: 2, recovery: true},
		{name: "v11 ledger retains recovery binding through request seventeen", ledger: `{"grants":[{"authorization_id":"extension-one","requests":6},{"authorization_id":"qual05-qwen38-recovery-1","requests":6,"candidate_id":"qwen38-v10-recovery-1","model":"qwen/qwen3.8-27b"},{"authorization_id":"qual05-qwen38-schema-1","requests":6,"candidate_id":"qwen38-v11-schema-1","model":"qwen/qwen3.8-27b","prompt_version":"file-analysis-v11"}]}`, consumed: 12, wantCap: 24, wantGrants: 3, recovery: true},
		{name: "bound twenty-four-request ledger", ledger: `{"grants":[{"authorization_id":"extension-one","requests":6},{"authorization_id":"qual05-qwen38-recovery-1","requests":6,"candidate_id":"qwen38-v10-recovery-1","model":"qwen/qwen3.8-27b"},{"authorization_id":"qual05-qwen38-schema-1","requests":6,"candidate_id":"qwen38-v11-schema-1","model":"qwen/qwen3.8-27b","prompt_version":"file-analysis-v11"}]}`, consumed: 18, wantCap: 24, wantGrants: 3, v11: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: test.consumed}); err != nil {
				t.Fatal(err)
			}
			if test.ledger != "" {
				if err := os.WriteFile(developmentGrantLedgerPath(directory), []byte(test.ledger), 0600); err != nil {
					t.Fatal(err)
				}
			}
			loaded, exists, err := loadDevelopmentGrantLedger(directory)
			if err != nil || exists != (test.ledger != "") || len(loaded.Grants) != test.wantGrants {
				t.Fatalf("load persisted ledger: %+v, %t, %v", loaded, exists, err)
			}
			campaign, err := loadEvaluationCampaign(filepath.Join(directory, "campaign.json"))
			if err != nil || campaign.DevelopmentRequests != test.consumed || campaign.QualificationRequests != 0 {
				t.Fatalf("load persisted campaign: %+v, %v", campaign, err)
			}
			cfg := runnerTestConfig(t, "persisted-cap", EngineeringInsightDevelopmentRunMode, 3, &fakeEngineeringInsightClient{})
			if test.recovery {
				cfg = recoveryRunnerTestConfig(t, "persisted-cap", &fakeEngineeringInsightClient{})
			}
			if test.v11 {
				cfg = v11RunnerTestConfig(t, "persisted-cap", &fakeEngineeringInsightClient{})
			}
			cap, err := developmentRequestCap(directory, test.consumed, cfg)
			if err != nil || cap != test.wantCap {
				t.Fatalf("development cap = %d, want %d: %v", cap, test.wantCap, err)
			}
		})
	}
}

func TestEngineeringInsightRunnerRejectsUnboundRecoveryAuthorizationInPersistedLedger(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(developmentGrantLedgerPath(directory), []byte(`{"grants":[{"authorization_id":"qual05-qwen38-recovery-1","requests":6}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadDevelopmentGrantLedger(directory); err == nil {
		t.Fatal("persisted recovery authorization was accepted as an unbound first grant")
	}
}

func TestEngineeringInsightRunnerDoesNotReplayInterruptedBoundRecovery(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := recoveryRunnerTestConfig(t, "interrupted-recovery", client)
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	ledger := engineeringInsightDevelopmentGrantLedger{Grants: []engineeringInsightDevelopmentGrant{
		{AuthorizationID: "extension-one", Requests: 6},
		{AuthorizationID: recoveryDevelopmentAuthorizationID, Requests: 6, CandidateID: recoveryDevelopmentCandidateID, Model: recoveryDevelopmentModel},
	}}
	if err := writeRunnerJSON(developmentGrantLedgerPath(directory), ledger); err != nil {
		t.Fatal(err)
	}
	manifest := engineeringInsightRunManifest{Fingerprint: runnerFingerprint(cfg), Receipt: runnerReceipt(cfg)}
	for _, item := range cfg.Cases {
		manifest.Receipt.Attempts = append(manifest.Receipt.Attempts, unknownReservedAttempt(item.Expected, cfg.CandidateID))
	}
	manifest.Receipt.Consumption.Requests = len(cfg.Cases)
	if err := writeRunnerJSON(filepath.Join(directory, cfg.RunID+".json"), manifest); err != nil {
		t.Fatal(err)
	}
	if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: 15}); err != nil {
		t.Fatal(err)
	}
	receipt, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg)
	if err != nil || client.calls.Load() != 0 || receipt.Attempts[0].Outcome != "unknown" {
		t.Fatalf("interrupted bound recovery replayed: receipt=%+v calls=%d err=%v", receipt, client.calls.Load(), err)
	}
	campaign, err := loadEvaluationCampaign(filepath.Join(directory, "campaign.json"))
	if err != nil || campaign.DevelopmentRequests != 15 || campaign.QualificationRequests != 0 {
		t.Fatalf("interrupted recovery campaign changed: %+v, %v", campaign, err)
	}
}

func TestEngineeringInsightRunnerLocksBoundRecoveryGrantAndDispatchTogether(t *testing.T) {
	client := &fakeEngineeringInsightClient{reply: runnerValidResponse()}
	cfg := recoveryRunnerTestConfig(t, "concurrent-recovery", client)
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	ledger := engineeringInsightDevelopmentGrantLedger{Grants: []engineeringInsightDevelopmentGrant{{AuthorizationID: "extension-one", Requests: 6}}}
	if err := writeRunnerJSON(developmentGrantLedgerPath(directory), ledger); err != nil {
		t.Fatal(err)
	}
	if err := writeRunnerJSON(filepath.Join(directory, "campaign.json"), engineeringInsightCampaign{DevelopmentRequests: 12}); err != nil {
		t.Fatal(err)
	}
	lock, err := lockRunnerFile(filepath.Join(directory, "campaign.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := GrantEngineeringInsightRecoveryDevelopmentBudget(cfg.Root, recoveryDevelopmentAuthorizationID, 6, recoveryDevelopmentCandidateID, recoveryDevelopmentModel); !errors.Is(err, ErrEngineeringInsightRunLocked) {
		t.Fatalf("concurrent recovery grant = %v", err)
	}
	if _, _, err := RunEngineeringInsightEvaluation(context.Background(), cfg); !errors.Is(err, ErrEngineeringInsightRunLocked) || client.calls.Load() != 0 {
		t.Fatalf("concurrent recovery dispatch = %v, calls=%d", err, client.calls.Load())
	}
}

func TestEngineeringInsightRunnerRejectsCorruptDevelopmentGrantLedgerBeforeDispatch(t *testing.T) {
	for name, ledger := range map[string]string{
		"unknown field":    `{"grants":[{"authorization_id":"extension-one","requests":6}],"unexpected":true}`,
		"duplicate field":  `{"grants":[],"grants":[{"authorization_id":"extension-one","requests":6}]}`,
		"empty grants":     `{"grants":[]}`,
		"wrong requests":   `{"grants":[{"authorization_id":"extension-one","requests":5}]}`,
		"v11 out of order": `{"grants":[{"authorization_id":"extension-one","requests":6},{"authorization_id":"qual05-qwen38-schema-1","requests":6,"candidate_id":"qwen38-v11-schema-1","model":"qwen/qwen3.8-27b","prompt_version":"file-analysis-v11"}]}`,
		"unbound v11":      `{"grants":[{"authorization_id":"extension-one","requests":6},{"authorization_id":"qual05-qwen38-recovery-1","requests":6,"candidate_id":"qwen38-v10-recovery-1","model":"qwen/qwen3.8-27b"},{"authorization_id":"qual05-qwen38-schema-1","requests":6,"candidate_id":"qwen38-v11-schema-1","model":"qwen/qwen3.8-27b"}]}`,
		"unbound v12":      `{"grants":[{"authorization_id":"extension-one","requests":6},{"authorization_id":"qual05-qwen38-recovery-1","requests":6,"candidate_id":"qwen38-v10-recovery-1","model":"qwen/qwen3.8-27b"},{"authorization_id":"qual05-qwen38-schema-1","requests":6,"candidate_id":"qwen38-v11-schema-1","model":"qwen/qwen3.8-27b","prompt_version":"file-analysis-v11"},{"authorization_id":"authqual05-qwen38-structured-1","requests":6,"candidate_id":"wrong-candidate","model":"qwen/qwen3.8-27b","prompt_version":"file-analysis-v12"}]}`,
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

func recoveryRunnerTestConfig(t *testing.T, runID string, client EngineeringInsightRunnerClient) EngineeringInsightRunnerConfig {
	cfg := runnerTestConfig(t, runID, EngineeringInsightDevelopmentRunMode, 3, client)
	cfg.CandidateID = recoveryDevelopmentCandidateID
	cfg.Model = recoveryDevelopmentModel
	cfg.Profile.Model = recoveryDevelopmentModel
	return cfg
}

func v11RunnerTestConfig(t *testing.T, runID string, client EngineeringInsightRunnerClient) EngineeringInsightRunnerConfig {
	cfg := runnerTestConfig(t, runID, EngineeringInsightDevelopmentRunMode, 3, client)
	cfg.CandidateID = v11DevelopmentCandidateID
	cfg.Model = v11DevelopmentModel
	cfg.Profile.Model = v11DevelopmentModel
	cfg.PromptVersion = v11DevelopmentPromptVersion
	return cfg
}

func v12RunnerTestConfig(t *testing.T, runID string, client EngineeringInsightRunnerClient) EngineeringInsightRunnerConfig {
	cfg := runnerTestConfig(t, runID, EngineeringInsightDevelopmentRunMode, 3, client)
	cfg.CandidateID = v12DevelopmentCandidateID
	cfg.Model = v12DevelopmentModel
	cfg.Profile.Model = v12DevelopmentModel
	cfg.PromptVersion = v12DevelopmentPromptVersion
	return cfg
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
