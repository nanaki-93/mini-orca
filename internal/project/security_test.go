package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSecurityFindingsIsStrictAnchoredAndAllowsEmpty(t *testing.T) {
	indexed := securityIndexedFile()
	source := securitySource()
	findings, err := ParseSecurityFindings(validSecurityFindingsJSON(), indexed, source)
	if err != nil || len(findings) != 1 || findings[0].ID == "" {
		t.Fatalf("valid findings = %+v, %v", findings, err)
	}
	empty, err := ParseSecurityFindings(`{"findings":[]}`, indexed, source)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty findings = %+v, %v", empty, err)
	}
	for _, test := range []struct {
		name   string
		output string
	}{
		{name: "unknown field", output: strings.Replace(validSecurityFindingsJSON(), `"title":"TLS verification disabled"`, `"title":"TLS verification disabled","unknown":true`, 1)},
		{name: "other file", output: strings.Replace(validSecurityFindingsJSON(), `"path":"main.go"`, `"path":"other.go"`, 1)},
		{name: "outside symbol", output: strings.Replace(validSecurityFindingsJSON(), `"start_line":4,"end_line":4`, `"start_line":1,"end_line":1`, 1)},
		{name: "path only anchor", output: strings.Replace(validSecurityFindingsJSON(), `"start_line":4,"end_line":4,"symbol":"Run"`, `"symbol":"Run"`, 1)},
		{name: "unknown symbol", output: strings.Replace(validSecurityFindingsJSON(), `"symbol":"Run"`, `"symbol":"Missing"`, 1)},
		{name: "deterministic evidence in model response", output: strings.Replace(validSecurityFindingsJSON(), `"evidence_kind":"model_suspicion"`, `"evidence_kind":"rule_match"`, 1)},
		{name: "verified reproduction in model response", output: strings.Replace(validSecurityFindingsJSON(), `"evidence_kind":"model_suspicion"`, `"evidence_kind":"verified_reproduction"`, 1)},
		{name: "overlong raw title", output: strings.Replace(validSecurityFindingsJSON(), `"title":"TLS verification disabled"`, `"title":"`+strings.Repeat("x", maxSecurityTextBytes+1)+`"`, 1)},
		{name: "missing findings", output: `{}`},
		{name: "null findings", output: `{"findings":null}`},
		{name: "trailing data", output: validSecurityFindingsJSON() + `{}`},
		{name: "invalid cwe", output: strings.Replace(validSecurityFindingsJSON(), `"verification_idea":"Exercise the request against an invalid certificate."`, `"verification_idea":"Exercise the request against an invalid certificate.","cwe":"CWE-0"`, 1)},
		{name: "unsafe reference", output: strings.Replace(validSecurityFindingsJSON(), `"verification_idea":"Exercise the request against an invalid certificate."`, `"verification_idea":"Exercise the request against an invalid certificate.","reference":"http://example.com"`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseSecurityFindings(test.output, indexed, source); err == nil {
				t.Fatal("invalid security output was accepted")
			}
		})
	}
}

func TestParseSecurityFindingsRejectsAnyInvalidNonemptyFinding(t *testing.T) {
	valid := strings.TrimSuffix(strings.TrimPrefix(validSecurityFindingsJSON(), `{"findings":[`), `]}`)
	invalid := strings.Replace(valid, `"title":"TLS verification disabled"`, `"title":""`, 1)
	if _, err := ParseSecurityFindings(`{"findings":[`+valid+","+invalid+`]}`, securityIndexedFile(), securitySource()); err == nil {
		t.Fatal("mixed valid and invalid findings became a partial decode")
	}
}

func TestParseSecurityFindingsRejectsEveryEmptyRequiredProseField(t *testing.T) {
	for _, field := range []string{"title", "observed_condition", "preconditions_or_unknowns", "remediation", "verification_idea"} {
		t.Run(field, func(t *testing.T) {
			output := strings.Replace(validSecurityFindingsJSON(), `"`+field+`":"`+securityRequiredFieldValue(field)+`"`, `"`+field+`":""`, 1)
			if _, err := ParseSecurityFindings(output, securityIndexedFile(), securitySource()); err == nil {
				t.Fatal("empty required prose was accepted")
			}
		})
	}
}

func TestParseSecurityFindingsRejectsDuplicateIdentity(t *testing.T) {
	valid := strings.TrimSuffix(strings.TrimPrefix(validSecurityFindingsJSON(), `{"findings":[`), `]}`)
	if _, err := ParseSecurityFindings(`{"findings":[`+valid+","+valid+`]}`, securityIndexedFile(), securitySource()); err == nil {
		t.Fatal("duplicate finding identity was accepted")
	}
}

func TestParseSecurityFindingsBindsSourceHash(t *testing.T) {
	differentSameLineSource := strings.Replace(securitySource(), "true", "false", 1)
	if _, err := ParseSecurityFindings(validSecurityFindingsJSON(), securityIndexedFile(), differentSameLineSource); err == nil {
		t.Fatal("same-line-count source with a different hash was accepted")
	}
}

func TestSecurityFindingIdentityExcludesModelWordingAndInsight(t *testing.T) {
	findings, err := ParseSecurityFindings(validSecurityFindingsJSON(), securityIndexedFile(), securitySource())
	if err != nil {
		t.Fatal(err)
	}
	changed := findings[0]
	changed.Title = "Different model title"
	changed.ObservedCondition = "Different model explanation."
	changed.EngineeringInsight = &EngineeringInsight{Mechanism: "Different", WhyItMattersHere: "Still advisory"}
	if SecurityFindingID(changed) != findings[0].ID {
		t.Fatal("finding identity changed with model wording or insight")
	}
}

func TestSecurityRedactionCoversCommonCredentialAssignments(t *testing.T) {
	for _, test := range []struct {
		name   string
		input  string
		secret string
	}{
		{name: "quoted JSON key", input: `{"api_key": "json-secret"}`, secret: "json-secret"},
		{name: "provider token", input: `github_token=provider-secret`, secret: "provider-secret"},
		{name: "provider API token", input: `"openai_api_token":"provider-api-secret"`, secret: "provider-api-secret"},
		{name: "prefixed secret access key", input: `AWS_SECRET_ACCESS_KEY='access-secret'`, secret: "access-secret"},
		{name: "authorization equals", input: `Authorization=Bearer equals-secret`, secret: "equals-secret"},
		{name: "proxy authorization equals", input: `Proxy_Authorization = Basic proxy-secret`, secret: "proxy-secret"},
		{name: "escaped quoted value", input: `{"client_secret":"prefix\"quoted-secret"}`, secret: "quoted-secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			redacted := redactSecurityText(test.input)
			if strings.Contains(redacted, test.secret) || !strings.Contains(redacted, "[redacted]") {
				t.Fatalf("redacted text = %q", redacted)
			}
			if again := redactSecurityText(redacted); again != redacted {
				t.Fatalf("redaction is not idempotent: %q then %q", redacted, again)
			}
		})
	}
	for _, safe := range []string{
		"Token bucket behavior limits requests.",
		"token_count=2048",
		"Authorization policy requires review.",
		"Keep secret handling inside the provider boundary.",
	} {
		if redacted := redactSecurityText(safe); redacted != safe {
			t.Fatalf("safe prose changed from %q to %q", safe, redacted)
		}
	}
}

func TestSecurityCachePersistsSourceFreeFreshnessBoundReports(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(securitySource()), 0600); err != nil {
		t.Fatal(err)
	}
	cache, err := NewSecurityReportCache(root)
	if err != nil {
		t.Fatal(err)
	}
	input := securityReportInput()
	input.ContentHash = contentHash([]byte(securitySource()))
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	input.ContextPolicyVersion = policy.Version()
	input.ProviderOrigin = "github_token=provider-secret"
	report := securityReport(input, SecurityStatusPartial)
	report.Reason = `"api_key": "json-secret"; Authorization=Bearer equals-secret`
	report.ProviderOrigin = "github_token=provider-secret"
	report.Findings[0].ObservedCondition = "openai_api_token=provider-api-secret"
	report.Findings[0].Reference = "https://example.com/docs?client_secret=reference-secret"
	report.Findings[0].EngineeringInsight = &EngineeringInsight{Mechanism: "password='insight secret'", WhyItMattersHere: "The persisted report remains safe to inspect."}
	if err := cache.Store(report, securityIndexedFile()); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(cache.cachePath(input.Path, input.Source))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"json-secret", "equals-secret", "provider-secret", "provider-api-secret", "reference-secret", "insight secret"} {
		if strings.Contains(string(stored), secret) {
			t.Fatalf("security cache retained %q: %s", secret, stored)
		}
	}
	if strings.Contains(string(stored), securitySource()) || filepath.Base(cache.cachePath(input.Path, input.Source)) == input.Path {
		t.Fatalf("security cache retained source or a raw path: %s", stored)
	}
	loaded, err := cache.Load(input)
	if err != nil || loaded == nil || loaded.Status != SecurityStatusPartial || loaded.Findings[0].Triage != SecurityTriageOpen || loaded.Findings[0].VerificationState != SecurityVerificationUnverified {
		t.Fatalf("loaded security report = %+v, %v", loaded, err)
	}
	if loaded.Findings[0].EngineeringInsight == nil || strings.Contains(loaded.Findings[0].EngineeringInsight.Mechanism, "insight secret") {
		t.Fatalf("loaded insight was not redacted = %+v", loaded.Findings[0].EngineeringInsight)
	}
	loaded.Findings[0].EngineeringInsight = &EngineeringInsight{Mechanism: "mutated", WhyItMattersHere: "mutated"}
	again, err := cache.Load(input)
	if err != nil || again.Findings[0].EngineeringInsight == nil || again.Findings[0].EngineeringInsight.Mechanism == "mutated" {
		t.Fatalf("cache did not clone findings = %+v, %v", again, err)
	}
	deterministicInput := input
	deterministicInput.Source = SecuritySourceDeterministic
	deterministicInput.RuleSetVersion = "go-rules-v1"
	deterministicInput.Model = ""
	deterministicInput.ConfiguredModel = ""
	deterministicInput.Profile = ""
	deterministicInput.Scope = ""
	deterministicInput.ProviderOrigin = ""
	deterministicInput.ReasoningEffort = ""
	deterministicInput.PromptVersion = ""
	deterministic := securityReport(deterministicInput, SecurityStatusCompleted)
	if err := cache.Store(deterministic, securityIndexedFile()); err != nil {
		t.Fatal(err)
	}
	ai, err := cache.Load(input)
	if err != nil || ai == nil || ai.Status != SecurityStatusPartial {
		t.Fatalf("AI report was replaced by deterministic report = %+v, %v", ai, err)
	}
	deterministicLoaded, err := cache.Load(deterministicInput)
	if err != nil || deterministicLoaded == nil || deterministicLoaded.Status != SecurityStatusCompleted {
		t.Fatalf("deterministic report = %+v, %v", deterministicLoaded, err)
	}
	changedRuleSet := deterministicInput
	changedRuleSet.RuleSetVersion = "go-rules-v2"
	if stale, err := cache.Load(changedRuleSet); err != nil || stale == nil || stale.Status != SecurityStatusStale {
		t.Fatalf("rule-set stale report = %+v, %v", stale, err)
	}
	changedProfile := input
	changedProfile.Profile = "other"
	if stale, err := cache.Load(changedProfile); err != nil || stale == nil || stale.Status != SecurityStatusStale {
		t.Fatalf("AI provenance stale report = %+v, %v", stale, err)
	}
	changed := input
	changed.ContentHash = "sha256:changed"
	stale, err := cache.Load(changed)
	if err != nil || stale == nil || stale.Status != SecurityStatusStale {
		t.Fatalf("stale report = %+v, %v", stale, err)
	}
}

func TestSecurityStoreRejectsNonCanonicalProvenanceAndAnchors(t *testing.T) {
	root := t.TempDir()
	cache, err := NewSecurityReportCache(root)
	if err != nil {
		t.Fatal(err)
	}
	indexed := securityIndexedFile()
	for _, test := range []struct {
		name   string
		mutate func(*SecurityFileReport)
	}{
		{name: "wrong id", mutate: func(report *SecurityFileReport) { report.Findings[0].ID = "security:wrong" }},
		{name: "padded enum", mutate: func(report *SecurityFileReport) { report.Findings[0].Severity = " high" }},
		{name: "overlong prose", mutate: func(report *SecurityFileReport) {
			report.Findings[0].Title = strings.Repeat("x", maxSecurityTextBytes+1)
		}},
		{name: "out of range anchor", mutate: func(report *SecurityFileReport) { report.Findings[0].Anchor.EndLine = 99 }},
		{name: "unknown symbol", mutate: func(report *SecurityFileReport) { report.Findings[0].Anchor.Symbol = "Missing" }},
		{name: "wrong indexed file", mutate: func(report *SecurityFileReport) { report.Path = "other.go" }},
		{name: "wrong indexed hash", mutate: func(report *SecurityFileReport) { report.ContentHash = "sha256:other" }},
		{name: "secret in identity", mutate: func(report *SecurityFileReport) { report.ProjectID = `"api_key":"identity-secret"` }},
		{name: "incomplete AI provenance", mutate: func(report *SecurityFileReport) { report.ConfiguredModel = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := securityReport(securityReportInput(), SecurityStatusCompleted)
			test.mutate(&report)
			if err := cache.Store(report, indexed); err == nil {
				t.Fatal("invalid report was stored")
			}
		})
	}
	deterministic := securityReport(deterministicSecurityInput(), SecurityStatusCompleted)
	deterministic.Model = "unexpected"
	if err := cache.Store(deterministic, indexed); err == nil {
		t.Fatal("deterministic report accepted AI provenance")
	}
	aiRuleMatch := securityReport(securityReportInput(), SecurityStatusCompleted)
	aiRuleMatch.Findings[0].EvidenceKind = "rule_match"
	aiRuleMatch.Findings[0].ID = SecurityFindingID(aiRuleMatch.Findings[0])
	if err := cache.Store(aiRuleMatch, indexed); err == nil {
		t.Fatal("AI report accepted deterministic rule evidence")
	}
	deterministicSuspicion := securityReport(deterministicSecurityInput(), SecurityStatusCompleted)
	deterministicSuspicion.Findings[0].EvidenceKind = "model_suspicion"
	deterministicSuspicion.Findings[0].ID = SecurityFindingID(deterministicSuspicion.Findings[0])
	if err := cache.Store(deterministicSuspicion, indexed); err == nil {
		t.Fatal("deterministic report accepted model evidence")
	}
	longCWE := securityReport(securityReportInput(), SecurityStatusCompleted)
	longCWE.Findings[0].CWE = "CWE-" + strings.Repeat("1", 40)
	if err := cache.Store(longCWE, indexed); err == nil {
		t.Fatal("overlong CWE was accepted")
	}
	empty := securityReport(securityReportInput(), SecurityStatusCompletedEmpty)
	empty.Findings = nil
	if err := cache.Store(empty, indexed); err == nil {
		t.Fatal("nil findings state was accepted")
	}
}

func TestSecurityCacheRecoversCorruptionAndValidatesLifecycleStates(t *testing.T) {
	root := t.TempDir()
	cache, err := NewSecurityReportCache(root)
	if err != nil {
		t.Fatal(err)
	}
	input := securityReportInput()
	if err := os.MkdirAll(filepath.Dir(cache.cachePath(input.Path, input.Source)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache.cachePath(input.Path, input.Source), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := cache.Load(input)
	if err != nil || loaded != nil {
		t.Fatalf("corrupt report = %+v, %v", loaded, err)
	}
	corrupt, err := filepath.Glob(cache.cachePath(input.Path, input.Source) + ".corrupt-*")
	if err != nil || len(corrupt) != 1 {
		t.Fatalf("corrupt backup = %v, %v", corrupt, err)
	}

	for _, status := range []string{SecurityStatusNotRun, SecurityStatusRunning, SecurityStatusCompleted, SecurityStatusCompletedEmpty, SecurityStatusPartial, SecurityStatusStale, SecurityStatusFailed, SecurityStatusCanceled} {
		report := securityReport(input, status)
		switch status {
		case SecurityStatusNotRun, SecurityStatusRunning, SecurityStatusCompletedEmpty, SecurityStatusStale:
			report.Findings = []SecurityFinding{}
		case SecurityStatusPartial:
			report.Reason = "Deterministic coverage is incomplete."
		case SecurityStatusFailed, SecurityStatusCanceled:
			report.Findings = []SecurityFinding{}
			report.Reason = "The selected review did not finish."
		}
		if err := cache.Store(report, securityIndexedFile()); err != nil {
			t.Fatalf("store %s: %v", status, err)
		}
	}
	invalid := securityReport(input, SecurityStatusPartial)
	invalid.Findings = []SecurityFinding{}
	invalid.Reason = ""
	if err := cache.Store(invalid, securityIndexedFile()); err == nil {
		t.Fatal("partial report without findings and reason was accepted")
	}
}

func TestSecurityTriageAndVerificationAreIndependent(t *testing.T) {
	report := securityReport(securityReportInput(), SecurityStatusPartial)
	report.Reason = "Some producer coverage was unavailable."
	report.Findings[0].Triage = SecurityTriageDismissed
	report.Findings[0].VerificationState = SecurityVerificationVerified
	if !validStoredSecurityReport(report, nil) {
		t.Fatal("independent triage and verification states were rejected")
	}
	report.Findings[0].VerificationState = SecurityTriageDismissed
	if validStoredSecurityReport(report, nil) {
		t.Fatal("triage value was accepted as verification state")
	}
}

func securitySource() string {
	return "package main\n\nfunc Run() {\n\ttransport.InsecureSkipVerify = true\n}\n"
}

func securityIndexedFile() IndexFile {
	return IndexFile{Path: "main.go", ContentHash: contentHash([]byte(securitySource())), LineCount: 6, Symbols: []SymbolInfo{{Name: "Run", StartLine: 3, EndLine: 5}}}
}

func validSecurityFindingsJSON() string {
	return `{"findings":[{"rule":"go.tls.insecure-skip-verify","category":"transport-security","title":"TLS verification disabled","source_anchor":{"path":"main.go","start_line":4,"end_line":4,"symbol":"Run"},"severity":"high","confidence":"high","evidence_kind":"model_suspicion","observed_condition":"The transport disables certificate verification.","preconditions_or_unknowns":"Reachability from an external request is unknown.","remediation":"Keep certificate verification enabled.","verification_idea":"Exercise the request against an invalid certificate."}]}`
}

func securityRequiredFieldValue(field string) string {
	return map[string]string{
		"title":                     "TLS verification disabled",
		"observed_condition":        "The transport disables certificate verification.",
		"preconditions_or_unknowns": "Reachability from an external request is unknown.",
		"remediation":               "Keep certificate verification enabled.",
		"verification_idea":         "Exercise the request against an invalid certificate.",
	}[field]
}

func securityReportInput() SecurityReportInput {
	return SecurityReportInput{ProjectID: "sha256:project", ProjectRevision: "sha256:revision", Path: "main.go", ContentHash: contentHash([]byte(securitySource())), Source: SecuritySourceAI, Model: "model", ConfiguredModel: "model", Profile: "analyze", Scope: "analyze", ProviderOrigin: "https://provider.example", ReasoningEffort: "high", PromptVersion: SecurityPromptVersion, ContextPolicyVersion: "policy-v1"}
}

func deterministicSecurityInput() SecurityReportInput {
	input := securityReportInput()
	input.Source = SecuritySourceDeterministic
	input.RuleSetVersion = "go-rules-v1"
	input.Model = ""
	input.ConfiguredModel = ""
	input.Profile = ""
	input.Scope = ""
	input.ProviderOrigin = ""
	input.ReasoningEffort = ""
	input.PromptVersion = ""
	return input
}

func securityReport(input SecurityReportInput, status string) SecurityFileReport {
	findings, err := ParseSecurityFindings(validSecurityFindingsJSON(), securityIndexedFile(), securitySource())
	if err != nil {
		panic(err)
	}
	if input.Source == SecuritySourceDeterministic {
		findings[0].EvidenceKind = "rule_match"
		findings[0].ID = SecurityFindingID(findings[0])
	}
	return SecurityFileReport{SchemaVersion: securityReportSchemaVersion, ProjectID: input.ProjectID, ProjectRevision: input.ProjectRevision, Path: input.Path, ContentHash: input.ContentHash, Source: input.Source, RuleSetVersion: input.RuleSetVersion, Status: status, Findings: findings, Model: input.Model, ConfiguredModel: input.ConfiguredModel, Profile: input.Profile, Scope: input.Scope, ProviderOrigin: input.ProviderOrigin, ReasoningEffort: input.ReasoningEffort, PromptVersion: input.PromptVersion, ContextPolicyVersion: input.ContextPolicyVersion, GeneratedAt: time.Now().UTC()}
}
