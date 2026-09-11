package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	SecurityPromptVersion               = "security-file-v1"
	SecurityMaxSourceBytes              = 64 * 1024
	maxSecurityOutputBytes              = 64 * 1024
	maxSecurityFindings                 = 5
	maxSecurityTextBytes                = 2048
	maxSecurityReasonBytes              = 1024
	minSecurityEchoWordCharacters       = 4
	minSecurityEchoSingleWordCharacters = 8

	SecurityStatusNotRun         = "not_run"
	SecurityStatusRunning        = "running"
	SecurityStatusCompleted      = "completed"
	SecurityStatusCompletedEmpty = "completed_empty"
	SecurityStatusPartial        = "partial"
	SecurityStatusStale          = "stale"
	SecurityStatusFailed         = "failed"
	SecurityStatusCanceled       = "canceled"

	SecurityTriageOpen      = "open"
	SecurityTriageDismissed = "dismissed"
	SecurityTriageAccepted  = "accepted"

	SecurityVerificationUnverified      = "unverified"
	SecurityVerificationVerified        = "verified"
	SecurityVerificationNotReproducible = "not_reproducible"

	SecuritySourceDeterministic = "deterministic"
	SecuritySourceAI            = FindingSourceAI
)

const securityReportSchemaVersion = "1"

var (
	securityKeyValueSecret = regexp.MustCompile(`(?i)(^|[^a-z0-9_])["']?(?:[a-z0-9]+[._-])*(?:secret[_-]?access[_-]?key|access[_-]?key(?:[_-]?id)?|api[_-]?(?:key|token)|access[_-]?token|auth[_-]?token|client[_-]?secret|private[_-]?key|credentials?|password|passwd|secret|token|pwd)["']?\s*[:=]\s*(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s,;}"']+)`)
	securityAuthorization  = regexp.MustCompile(`(?im)(^|[^a-z0-9_])["']?(?:proxy[-_])?authorization["']?\s*[:=]\s*(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\r\n;}"']*)`)
	securityCWE            = regexp.MustCompile(`^CWE-[1-9][0-9]*$`)
)

type SecuritySourceAnchor struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Symbol    string `json:"symbol,omitempty"`
}

// SecurityFinding keeps the user's triage separate from verification evidence.
type SecurityFinding struct {
	ID                 string               `json:"id"`
	Rule               string               `json:"rule"`
	Category           string               `json:"category"`
	Title              string               `json:"title"`
	Anchor             SecuritySourceAnchor `json:"source_anchor"`
	Severity           string               `json:"severity"`
	Confidence         string               `json:"confidence"`
	EvidenceKind       string               `json:"evidence_kind"`
	ObservedCondition  string               `json:"observed_condition"`
	Preconditions      string               `json:"preconditions_or_unknowns"`
	Remediation        string               `json:"remediation"`
	VerificationIdea   string               `json:"verification_idea"`
	CWE                string               `json:"cwe,omitempty"`
	Reference          string               `json:"reference,omitempty"`
	Triage             string               `json:"triage"`
	VerificationState  string               `json:"verification_state"`
	EngineeringInsight *EngineeringInsight  `json:"engineering_insight,omitempty"`
}

// SecurityFileReport is owned by either deterministic rules or an AI review.
// The source-specific provenance makes those evidence types unambiguous.
type SecurityFileReport struct {
	SchemaVersion        string            `json:"schema_version"`
	ProjectID            string            `json:"project_id"`
	ProjectRevision      string            `json:"project_revision"`
	Path                 string            `json:"path"`
	ContentHash          string            `json:"content_hash"`
	Status               string            `json:"status"`
	Source               string            `json:"source"`
	RuleSetVersion       string            `json:"rule_set_version,omitempty"`
	Findings             []SecurityFinding `json:"findings"`
	Reason               string            `json:"reason,omitempty"`
	Model                string            `json:"model,omitempty"`
	ConfiguredModel      string            `json:"configured_model,omitempty"`
	Profile              string            `json:"profile,omitempty"`
	Scope                string            `json:"scope,omitempty"`
	ProviderOrigin       string            `json:"provider_origin,omitempty"`
	ReasoningEffort      string            `json:"reasoning_effort,omitempty"`
	PromptVersion        string            `json:"prompt_version,omitempty"`
	ContextPolicyVersion string            `json:"context_policy_version"`
	GeneratedAt          time.Time         `json:"generated_at"`
}

type SecurityReportInput struct {
	ProjectID, ProjectRevision, Path, ContentHash, Source, RuleSetVersion string
	Model, ConfiguredModel, Profile, Scope, ProviderOrigin                string
	ReasoningEffort, PromptVersion, ContextPolicyVersion                  string
}

type securityWire struct {
	Findings []securityFindingWire `json:"findings"`
}
type securityFindingWire struct {
	Rule              string               `json:"rule"`
	Category          string               `json:"category"`
	Title             string               `json:"title"`
	Anchor            SecuritySourceAnchor `json:"source_anchor"`
	Severity          string               `json:"severity"`
	Confidence        string               `json:"confidence"`
	EvidenceKind      string               `json:"evidence_kind"`
	ObservedCondition string               `json:"observed_condition"`
	Preconditions     string               `json:"preconditions_or_unknowns"`
	Remediation       string               `json:"remediation"`
	VerificationIdea  string               `json:"verification_idea"`
	CWE               string               `json:"cwe"`
	Reference         string               `json:"reference"`
	Insight           json.RawMessage      `json:"engineering_insight"`
}

// ParseSecurityFindings accepts one bounded JSON object. Findings must be an
// explicit array; invalid non-empty results are rejected as a whole.
func ParseSecurityFindings(output string, indexed IndexFile, source string) ([]SecurityFinding, error) {
	if !validSecurityReviewInput(output, indexed, source) {
		return nil, fmt.Errorf("security review input is invalid or too large")
	}
	wire, err := decodeSecurityWire(output)
	if err != nil {
		return nil, err
	}
	if wire.Findings == nil || len(wire.Findings) > maxSecurityFindings || securitySourceLineCount(source) != indexed.LineCount {
		return nil, fmt.Errorf("security review findings or indexed facts are invalid")
	}
	return securityFindingsFromWire(wire.Findings, indexed)
}

func securitySourceLineCount(source string) int {
	if source == "" {
		return 0
	}
	lines := strings.Count(source, "\n")
	if !strings.HasSuffix(source, "\n") {
		lines++
	}
	return lines
}

func validSecurityReviewInput(output string, indexed IndexFile, source string) bool {
	return len(output) > 0 &&
		len(output) <= maxSecurityOutputBytes &&
		validSecurityIndex(indexed) &&
		len(source) <= SecurityMaxSourceBytes &&
		contentHash([]byte(source)) == indexed.ContentHash
}

func decodeSecurityWire(output string) (securityWire, error) {
	var wire securityWire
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return securityWire{}, fmt.Errorf("parse security review JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return securityWire{}, fmt.Errorf("security review JSON must contain one object")
	}
	return wire, nil
}

func securityFindingsFromWire(items []securityFindingWire, indexed IndexFile) ([]SecurityFinding, error) {
	findings := make([]SecurityFinding, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if !validSecurityWireLengths(item) {
			return nil, fmt.Errorf("security review contains an overlong finding")
		}
		finding := SecurityFinding{Rule: item.Rule, Category: item.Category, Title: redactSecurityText(item.Title), Anchor: item.Anchor, Severity: item.Severity, Confidence: item.Confidence, EvidenceKind: item.EvidenceKind, ObservedCondition: redactSecurityText(item.ObservedCondition), Preconditions: redactSecurityText(item.Preconditions), Remediation: redactSecurityText(item.Remediation), VerificationIdea: redactSecurityText(item.VerificationIdea), CWE: item.CWE, Reference: redactSecurityText(item.Reference), Triage: SecurityTriageOpen, VerificationState: SecurityVerificationUnverified}
		finding.EngineeringInsight, _ = ParseOptionalEngineeringInsight(item.Insight)
		finding.EngineeringInsight = redactSecurityInsight(finding.EngineeringInsight)
		if !validSecurityFindingContent(finding) || finding.EvidenceKind != "model_suspicion" || !validSecurityAnchor(finding.Anchor, indexed) {
			return nil, fmt.Errorf("security review contains an invalid finding")
		}
		finding.ID = SecurityFindingID(finding)
		if _, duplicate := seen[finding.ID]; duplicate {
			return nil, fmt.Errorf("security review contains duplicate findings")
		}
		seen[finding.ID] = struct{}{}
		findings = append(findings, finding)
	}
	return findings, nil
}

// SecurityFindingID excludes model wording, insights, CWE, and references.
func SecurityFindingID(f SecurityFinding) string {
	parts := []string{f.Rule, f.Category, f.Anchor.Path, fmt.Sprint(f.Anchor.StartLine), fmt.Sprint(f.Anchor.EndLine), f.Anchor.Symbol, f.EvidenceKind}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "security:" + hex.EncodeToString(sum[:])
}

func validSecurityWireLengths(item securityFindingWire) bool {
	values := []struct {
		value string
		limit int
	}{{item.Rule, 128}, {item.Category, 128}, {item.Title, maxSecurityTextBytes}, {item.ObservedCondition, maxSecurityTextBytes}, {item.Preconditions, maxSecurityTextBytes}, {item.Remediation, maxSecurityTextBytes}, {item.VerificationIdea, maxSecurityTextBytes}, {item.CWE, 32}, {item.Reference, 512}, {item.Anchor.Path, 1024}, {item.Anchor.Symbol, 256}}
	for _, value := range values {
		if len(value.value) > value.limit {
			return false
		}
	}
	return true
}

func validSecurityIndex(indexed IndexFile) bool {
	path, err := normalizedIndexedPath(indexed.Path)
	return err == nil && path == indexed.Path && indexed.ContentHash != "" && indexed.LineCount >= 0
}

func validSecurityFindingContent(f SecurityFinding) bool {
	return validSecurityIdentifier(f.Rule, 128) && validSecurityIdentifier(f.Category, 128) && validRequiredSecurityText(f.Title, maxSecurityTextBytes) && validRequiredSecurityText(f.ObservedCondition, maxSecurityTextBytes) && validRequiredSecurityText(f.Preconditions, maxSecurityTextBytes) && validRequiredSecurityText(f.Remediation, maxSecurityTextBytes) && validRequiredSecurityText(f.VerificationIdea, maxSecurityTextBytes) && validSecurityCWE(f.CWE) && validSecurityReference(f.Reference) && validSecurityTriage(f.Triage) && validSecurityVerification(f.VerificationState) && oneOf(f.Severity, "critical", "high", "medium", "low", "info") && oneOf(f.Confidence, "high", "medium", "low") && oneOf(f.EvidenceKind, "rule_match", "model_suspicion") && validStoredSecurityInsight(f.EngineeringInsight)
}

func validSecurityAnchor(anchor SecuritySourceAnchor, indexed IndexFile) bool {
	return validSecurityAnchorShape(anchor) && anchor.Path == indexed.Path && anchor.EndLine <= indexed.LineCount && (anchor.Symbol == "" || containsSecuritySymbol(indexed.Symbols, anchor))
}
func validSecurityAnchorShape(anchor SecuritySourceAnchor) bool {
	path, err := normalizedIndexedPath(anchor.Path)
	return err == nil && path == anchor.Path && anchor.StartLine >= 1 && anchor.EndLine >= anchor.StartLine && len(anchor.Symbol) <= 256 && anchor.Symbol == strings.TrimSpace(anchor.Symbol)
}
func containsSecuritySymbol(symbols []SymbolInfo, anchor SecuritySourceAnchor) bool {
	for _, symbol := range symbols {
		if symbol.Name == anchor.Symbol && anchor.StartLine >= symbol.StartLine && anchor.EndLine <= symbol.EndLine {
			return true
		}
	}
	return false
}
func validSecurityIdentifier(value string, limit int) bool {
	return value != "" && value == strings.ToLower(value) && validSecurityText(value, limit)
}
func validSecurityText(value string, limit int) bool {
	return value == strings.TrimSpace(value) && len(value) <= limit && redactSecurityText(value) == value
}
func validRequiredSecurityText(value string, limit int) bool {
	return value != "" && validSecurityText(value, limit)
}
func validSecurityCWE(value string) bool {
	return value == "" || len(value) <= 32 && securityCWE.MatchString(value)
}
func validSecurityReference(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return validSecurityText(value, 512) && err == nil && parsed.IsAbs() && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}
func redactSecurityText(value string) string {
	value = securityAuthorization.ReplaceAllString(value, "${1}Authorization: [redacted]")
	return securityKeyValueSecret.ReplaceAllString(value, "${1}[redacted]")
}
func redactSecurityInsight(insight *EngineeringInsight) *EngineeringInsight {
	if insight == nil {
		return nil
	}
	copy := *insight
	copy.Mechanism, copy.WhyItMattersHere = redactSecurityText(copy.Mechanism), redactSecurityText(copy.WhyItMattersHere)
	copy.TradeoffOrFailureMode, copy.TransferableLesson = redactSecurityText(copy.TradeoffOrFailureMode), redactSecurityText(copy.TransferableLesson)
	return &copy
}
func validStoredSecurityInsight(insight *EngineeringInsight) bool {
	if insight == nil {
		return true
	}
	redacted := redactSecurityInsight(insight)
	if redacted == nil || *redacted != *insight {
		return false
	}
	canonical := *insight
	return ValidEngineeringInsight(&canonical) && canonical == *insight
}

type SecurityReportCache struct {
	root string
}

var securityCacheMu sync.Mutex

func NewSecurityReportCache(root string) (*SecurityReportCache, error) {
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	return &SecurityReportCache{root: canonical}, nil
}

// Store requires current indexed facts. It only redacts prose/provenance, and
// otherwise rejects malformed states, IDs, fields, anchors, and identities.
func (c *SecurityReportCache) Store(report SecurityFileReport, indexed IndexFile) error {
	return c.store(report, indexed, nil)
}

// StoreAuthorized validates and serializes a report under the shared cache
// lock. Authorization runs after the temporary file is durable and immediately
// before its atomic rename, so rejected results never become visible.
func (c *SecurityReportCache) StoreAuthorized(report SecurityFileReport, indexed IndexFile, authorize func() error) error {
	if authorize == nil {
		return c.Store(report, indexed)
	}
	return c.store(report, indexed, authorize)
}

func (c *SecurityReportCache) store(report SecurityFileReport, indexed IndexFile, authorize func() error) error {
	securityCacheMu.Lock()
	defer securityCacheMu.Unlock()
	report = cloneSecurityFileReportValue(report)
	redactSecurityReport(&report)
	if !validStoredSecurityReport(report, &indexed) {
		return fmt.Errorf("security report is invalid")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode security report: %w", err)
	}
	if authorize == nil {
		return storage.WriteFile(c.cachePath(report.Path, report.Source), data, 0600)
	}
	return storage.WriteFileAuthorized(c.cachePath(report.Path, report.Source), data, 0600, authorize)
}
func (c *SecurityReportCache) Load(input SecurityReportInput) (*SecurityFileReport, error) {
	securityCacheMu.Lock()
	defer securityCacheMu.Unlock()
	redactSecurityInput(&input)
	if err := validSecurityInput(input); err != nil {
		return nil, err
	}
	path := c.cachePath(input.Path, input.Source)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read security cache: %w", err)
	}
	var report SecurityFileReport
	if err := decodeStoredSecurityReport(data, &report); err != nil || !validStoredSecurityReport(report, nil) {
		if recoverErr := storage.RecoverCorrupt(path); recoverErr != nil {
			return nil, recoverErr
		}
		return nil, nil
	}
	if !securityReportMatchesInput(report, input) {
		report.Status = SecurityStatusStale
		return cloneSecurityFileReport(&report), nil
	}
	policy, err := NewContextPolicy(c.root)
	if err != nil || policy.Version() != report.ContextPolicyVersion || !policy.Decide(report.Path).Include {
		report.Status = SecurityStatusStale
		return cloneSecurityFileReport(&report), nil
	}
	current, err := GetFileInfo(c.root, report.Path)
	if err != nil || current.SizeBytes > SecurityMaxSourceBytes || current.ContentHash != report.ContentHash {
		report.Status = SecurityStatusStale
	}
	return cloneSecurityFileReport(&report), nil
}

func (c *SecurityReportCache) cachePath(path, source string) string {
	key := sha256.Sum256([]byte(source + "\x00" + path))
	return filepath.Join(c.root, ".mini-orca", "security", "files", hex.EncodeToString(key[:])+".json")
}

func LoadSecurityFileReport(root string, input SecurityReportInput) (*SecurityFileReport, error) {
	cache, err := NewSecurityReportCache(root)
	if err != nil {
		return nil, err
	}
	return cache.Load(input)
}

func decodeStoredSecurityReport(data []byte, report *SecurityFileReport) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(report); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("security report must contain one object")
	}
	return nil
}

func validSecurityInput(input SecurityReportInput) error {
	if !validSecurityInputIdentity(input) {
		return fmt.Errorf("security report identity is incomplete")
	}
	switch input.Source {
	case SecuritySourceAI:
		if !validAISecurityInput(input) {
			return fmt.Errorf("AI security report provenance is incomplete")
		}
	case SecuritySourceDeterministic:
		if !validDeterministicSecurityInput(input) {
			return fmt.Errorf("deterministic security report provenance is invalid")
		}
	default:
		return fmt.Errorf("security report source is invalid")
	}
	return nil
}

func validSecurityInputIdentity(input SecurityReportInput) bool {
	path, err := normalizedIndexedPath(input.Path)
	return err == nil &&
		path == input.Path &&
		validRequiredSecurityProvenance(input.ProjectID, 128) &&
		validRequiredSecurityProvenance(input.ProjectRevision, 128) &&
		validRequiredSecurityProvenance(input.ContentHash, 128) &&
		validRequiredSecurityProvenance(input.ContextPolicyVersion, 128)
}

func validAISecurityInput(input SecurityReportInput) bool {
	if input.RuleSetVersion != "" {
		return false
	}
	fields := []struct {
		value string
		limit int
	}{
		{input.Model, 512},
		{input.ConfiguredModel, 512},
		{input.Profile, 128},
		{input.Scope, 128},
		{input.ProviderOrigin, 512},
		{input.ReasoningEffort, 128},
		{input.PromptVersion, 128},
	}
	for _, field := range fields {
		if !validRequiredSecurityProvenance(field.value, field.limit) {
			return false
		}
	}
	return true
}

func validDeterministicSecurityInput(input SecurityReportInput) bool {
	return validRequiredSecurityProvenance(input.RuleSetVersion, 128) &&
		input.Model == "" &&
		input.ConfiguredModel == "" &&
		input.Profile == "" &&
		input.Scope == "" &&
		input.ProviderOrigin == "" &&
		input.ReasoningEffort == "" &&
		input.PromptVersion == ""
}
func validRequiredSecurityProvenance(value string, limit int) bool {
	return value != "" && validSecurityText(value, limit)
}
func redactSecurityInput(input *SecurityReportInput) {
	input.Model, input.ConfiguredModel = redactSecurityText(input.Model), redactSecurityText(input.ConfiguredModel)
	input.Profile, input.Scope = redactSecurityText(input.Profile), redactSecurityText(input.Scope)
	input.ProviderOrigin, input.ReasoningEffort = redactSecurityText(input.ProviderOrigin), redactSecurityText(input.ReasoningEffort)
	input.PromptVersion, input.ContextPolicyVersion = redactSecurityText(input.PromptVersion), redactSecurityText(input.ContextPolicyVersion)
}
func redactSecurityReport(report *SecurityFileReport) {
	report.Reason, report.Model, report.ConfiguredModel = redactSecurityText(report.Reason), redactSecurityText(report.Model), redactSecurityText(report.ConfiguredModel)
	report.Profile, report.Scope, report.ProviderOrigin = redactSecurityText(report.Profile), redactSecurityText(report.Scope), redactSecurityText(report.ProviderOrigin)
	report.ReasoningEffort, report.PromptVersion, report.ContextPolicyVersion = redactSecurityText(report.ReasoningEffort), redactSecurityText(report.PromptVersion), redactSecurityText(report.ContextPolicyVersion)
	for index := range report.Findings {
		f := &report.Findings[index]
		f.Title, f.ObservedCondition, f.Preconditions = redactSecurityText(f.Title), redactSecurityText(f.ObservedCondition), redactSecurityText(f.Preconditions)
		f.Remediation, f.VerificationIdea, f.Reference = redactSecurityText(f.Remediation), redactSecurityText(f.VerificationIdea), redactSecurityText(f.Reference)
		f.EngineeringInsight = redactSecurityInsight(f.EngineeringInsight)
	}
}

// SanitizeSecurityFileReport returns a source-free report with every prose and
// provenance field redacted using the same rules enforced by cache storage.
func SanitizeSecurityFileReport(report SecurityFileReport) SecurityFileReport {
	report = cloneSecurityFileReportValue(report)
	redactSecurityReport(&report)
	return report
}

// ValidateSecurityFileReportSourceFree rejects report prose that repeats a
// meaningful source-line token sequence. Redaction removes credentials; this
// guard keeps formatting variations of source out of durable and returned
// review evidence.
func ValidateSecurityFileReportSourceFree(report SecurityFileReport, source string) error {
	reportSignatures := make([]string, 0, len(report.Findings)*9+9)
	for _, text := range securityReportText(report) {
		if signature, _, _ := securityEchoSignature(text); signature != "" {
			reportSignatures = append(reportSignatures, signature)
		}
	}
	for _, line := range strings.Split(source, "\n") {
		signature, words, wordCharacters := securityEchoSignature(line)
		if !meaningfulSecurityEcho(words, wordCharacters) {
			continue
		}
		for _, reportSignature := range reportSignatures {
			if strings.Contains(reportSignature, signature) {
				return fmt.Errorf("security report repeats source text")
			}
		}
	}
	return nil
}

func securityEchoSignature(value string) (string, int, int) {
	words := make([]string, 0, 8)
	var word strings.Builder
	wordCharacters := 0
	flush := func() {
		if word.Len() == 0 {
			return
		}
		words = append(words, word.String())
		word.Reset()
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' {
			word.WriteRune(character)
			wordCharacters++
			continue
		}
		flush()
	}
	flush()
	if len(words) == 0 {
		return "", 0, 0
	}
	return "\x00" + strings.Join(words, "\x00") + "\x00", len(words), wordCharacters
}

func meaningfulSecurityEcho(words, wordCharacters int) bool {
	return wordCharacters >= minSecurityEchoWordCharacters &&
		(words > 1 || wordCharacters >= minSecurityEchoSingleWordCharacters)
}

func securityReportText(report SecurityFileReport) []string {
	texts := []string{report.Reason, report.Model, report.ConfiguredModel, report.Profile, report.Scope, report.ProviderOrigin, report.ReasoningEffort, report.PromptVersion, report.ContextPolicyVersion}
	for _, finding := range report.Findings {
		texts = append(texts, finding.Rule, finding.Category, finding.Title, finding.ObservedCondition, finding.Preconditions, finding.Remediation, finding.VerificationIdea, finding.CWE, finding.Reference)
		if finding.EngineeringInsight != nil {
			texts = append(texts, finding.EngineeringInsight.Mechanism, finding.EngineeringInsight.WhyItMattersHere, finding.EngineeringInsight.TradeoffOrFailureMode, finding.EngineeringInsight.TransferableLesson)
		}
	}
	return texts
}

func validStoredSecurityReport(report SecurityFileReport, indexed *IndexFile) bool {
	input := SecurityReportInput{ProjectID: report.ProjectID, ProjectRevision: report.ProjectRevision, Path: report.Path, ContentHash: report.ContentHash, Source: report.Source, RuleSetVersion: report.RuleSetVersion, Model: report.Model, ConfiguredModel: report.ConfiguredModel, Profile: report.Profile, Scope: report.Scope, ProviderOrigin: report.ProviderOrigin, ReasoningEffort: report.ReasoningEffort, PromptVersion: report.PromptVersion, ContextPolicyVersion: report.ContextPolicyVersion}
	if !validSecurityReportHeader(report, input, indexed) || !validSecurityReportLifecycle(report) {
		return false
	}
	return validSecurityReportFindings(report, indexed)
}

func validSecurityReportHeader(report SecurityFileReport, input SecurityReportInput, indexed *IndexFile) bool {
	return report.SchemaVersion == securityReportSchemaVersion &&
		validSecurityInput(input) == nil &&
		report.Findings != nil &&
		!report.GeneratedAt.IsZero() &&
		validSecurityStatus(report.Status) &&
		validSecurityText(report.Reason, maxSecurityReasonBytes) &&
		len(report.Findings) <= maxSecurityFindings &&
		(indexed == nil || validSecurityIndex(*indexed) && report.Path == indexed.Path && report.ContentHash == indexed.ContentHash)
}

func validSecurityReportLifecycle(report SecurityFileReport) bool {
	switch report.Status {
	case SecurityStatusNotRun, SecurityStatusRunning:
		return len(report.Findings) == 0
	case SecurityStatusCompletedEmpty:
		return len(report.Findings) == 0 && report.Reason == ""
	case SecurityStatusCompleted:
		return len(report.Findings) > 0 && report.Reason == ""
	case SecurityStatusPartial:
		return len(report.Findings) > 0 && report.Reason != ""
	case SecurityStatusFailed, SecurityStatusCanceled:
		return len(report.Findings) == 0 && report.Reason != ""
	case SecurityStatusStale:
		return true
	default:
		return false
	}
}

func validSecurityReportFindings(report SecurityFileReport, indexed *IndexFile) bool {
	seen := make(map[string]struct{}, len(report.Findings))
	for _, finding := range report.Findings {
		if finding.Anchor.Path != report.Path || !validStoredSecurityFinding(finding, indexed) {
			return false
		}
		if !validSecurityEvidenceSource(report.Source, finding.EvidenceKind) {
			return false
		}
		if _, duplicate := seen[finding.ID]; duplicate {
			return false
		}
		seen[finding.ID] = struct{}{}
	}
	return true
}

func validSecurityEvidenceSource(source, evidenceKind string) bool {
	return source == SecuritySourceAI && evidenceKind == "model_suspicion" ||
		source == SecuritySourceDeterministic && evidenceKind == "rule_match"
}
func validStoredSecurityFinding(f SecurityFinding, indexed *IndexFile) bool {
	return f.ID != "" && f.ID == SecurityFindingID(f) && validSecurityFindingContent(f) && validSecurityAnchorShape(f.Anchor) && (indexed == nil || validSecurityAnchor(f.Anchor, *indexed))
}
func validSecurityStatus(value string) bool {
	return oneOf(value, SecurityStatusNotRun, SecurityStatusRunning, SecurityStatusCompleted, SecurityStatusCompletedEmpty, SecurityStatusPartial, SecurityStatusStale, SecurityStatusFailed, SecurityStatusCanceled)
}
func validSecurityTriage(value string) bool {
	return oneOf(value, SecurityTriageOpen, SecurityTriageDismissed, SecurityTriageAccepted)
}
func validSecurityVerification(value string) bool {
	return oneOf(value, SecurityVerificationUnverified, SecurityVerificationVerified, SecurityVerificationNotReproducible)
}
func securityReportMatchesInput(report SecurityFileReport, input SecurityReportInput) bool {
	return report.ProjectID == input.ProjectID && report.ProjectRevision == input.ProjectRevision && report.Path == input.Path && report.ContentHash == input.ContentHash && report.Source == input.Source && report.RuleSetVersion == input.RuleSetVersion && report.Model == input.Model && report.ConfiguredModel == input.ConfiguredModel && report.Profile == input.Profile && report.Scope == input.Scope && report.ProviderOrigin == input.ProviderOrigin && report.ReasoningEffort == input.ReasoningEffort && report.PromptVersion == input.PromptVersion && report.ContextPolicyVersion == input.ContextPolicyVersion
}
func cloneSecurityFileReportValue(source SecurityFileReport) SecurityFileReport {
	return *cloneSecurityFileReport(&source)
}
func cloneSecurityFileReport(source *SecurityFileReport) *SecurityFileReport {
	if source == nil {
		return nil
	}
	clone := *source
	if source.Findings != nil {
		clone.Findings = make([]SecurityFinding, len(source.Findings))
		copy(clone.Findings, source.Findings)
	}
	for index := range clone.Findings {
		clone.Findings[index].EngineeringInsight = CloneEngineeringInsight(source.Findings[index].EngineeringInsight)
	}
	return &clone
}
