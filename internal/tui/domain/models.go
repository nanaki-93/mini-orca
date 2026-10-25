package domain

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

// ContainsGuideline reports whether existing AGENTS.md text already contains a rule bullet.
func ContainsGuideline(markdown, rule string) bool {
	normRule := normalizeFindingText(rule)
	if normRule == "" {
		return false
	}
	for _, line := range strings.Split(markdown, "\n") {
		if normalizeFindingText(line) == normRule {
			return true
		}
	}
	return false
}

// InsertGuidelineSection appends a bullet rule under a named Markdown heading in AGENTS.md.
func InsertGuidelineSection(markdown, heading, bullet string) string {
	trimmedBullet := strings.TrimSpace(bullet)
	if !strings.HasPrefix(trimmedBullet, "- ") {
		trimmedBullet = "- " + trimmedBullet
	}
	if ContainsGuideline(markdown, trimmedBullet) {
		return markdown
	}
	headerLine := "## " + strings.TrimSpace(heading)
	lines := strings.Split(strings.TrimRight(markdown, "\n"), "\n")
	for idx, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), headerLine) {
			out := append([]string{}, lines[:idx+1]...)
			out = append(out, trimmedBullet)
			out = append(out, lines[idx+1:]...)
			return strings.Join(out, "\n") + "\n"
		}
	}
	base := strings.TrimSpace(markdown)
	if base == "" {
		base = "# Project instructions"
	}
	return fmt.Sprintf("%s\n\n%s\n\n%s\n", base, headerLine, trimmedBullet)
}

// BootstrapPillar represents a repository governance pillar in the Governance & Rules hub.
type BootstrapPillar struct {
	ID           string
	Title        string
	Ready        bool
	DetectedPath string
	Description  string
}

// EvaluateBootstrapChecklist inspects the project index and instructions to check 4 core governance pillars.
func EvaluateBootstrapChecklist(index *client.ProjectIndex, instructions *client.InstructionPreview) []BootstrapPillar {
	files := make(map[string]bool)
	if index != nil {
		for _, f := range index.Files {
			files[f.Path] = true
		}
	}
	hasAgents := (instructions != nil && instructions.Exists) || files["AGENTS.md"]
	hasEntry := files["Makefile"] || files["justfile"]
	hasGates := files["scripts/validate.sh"] || files["scripts/quality.sh"] || files[".github/workflows/validate.yml"]
	hasHooks := files["lefthook.yml"] || files[".pre-commit-config.yaml"]

	return []BootstrapPillar{
		{ID: "agents-md", Title: "AGENTS.md Instructions", Ready: hasAgents, DetectedPath: pickPath(files, "AGENTS.md"), Description: "Repository boundaries, architecture ownership, and verification rules"},
		{ID: "entrypoints", Title: "Standardized Entrypoints", Ready: hasEntry, DetectedPath: pickPath(files, "justfile", "Makefile"), Description: "Reproducible build, test, and check targets"},
		{ID: "quality-gates", Title: "Automated Quality Gates", Ready: hasGates, DetectedPath: pickPath(files, "scripts/validate.sh", "scripts/quality.sh"), Description: "Static analysis, reachability, complexity, and race checks"},
		{ID: "git-hooks", Title: "Pre-Commit Guardrails", Ready: hasHooks, DetectedPath: pickPath(files, "lefthook.yml", ".pre-commit-config.yaml"), Description: "Fast pre-commit formatting, whitespace, and secret checks"},
	}
}

func pickPath(files map[string]bool, candidates ...string) string {
	for _, c := range candidates {
		if files[c] {
			return c
		}
	}
	return ""
}

// CanonicalModelScopes returns the three primary LLM routing scopes in Mini-Orca.
func CanonicalModelScopes() []string {
	return []string{"analyze", "bug", "code"}
}

// ScopeRoleDescription returns the purpose of a model scope.
func ScopeRoleDescription(scope string) string {
	switch scope {
	case "analyze":
		return "Architecture and Security AI; workflow roles are assigned in Roles & Budgets"
	case "bug":
		return "Semantic bug detection; workflow roles are assigned in Roles & Budgets"
	case "code":
		return "Scoped tasks and investigation; workflow roles are assigned in Roles & Budgets"
	default:
		return "Scoped LLM Runtime"
	}
}

// DefaultModelPresets returns the built-in 3-scope model configuration presets.
func DefaultModelPresets() []client.ModelPreset {
	return []client.ModelPreset{
		{ID: "local-default", Name: "Local Studio Default", Analyze: "local", Bug: "local", Code: "local"},
		{ID: "hybrid-precision", Name: "Hybrid Precision", Analyze: "local", Bug: "remote-pro", Code: "local"},
		{ID: "deep-audit", Name: "Deep Security & Bug Audit", Analyze: "remote-pro", Bug: "remote-pro", Code: "remote-fast"},
	}
}

// MergeModelPresets combines built-in presets with user-saved custom presets.
func MergeModelPresets(custom []client.ModelPreset) []client.ModelPreset {
	out := DefaultModelPresets()
	out = append(out, custom...)
	return out
}

// IsBuiltinModelPreset reports whether the given preset ID is a built-in default preset.
func IsBuiltinModelPreset(id string) bool {
	for _, p := range DefaultModelPresets() {
		if p.ID == id {
			return true
		}
	}
	return false
}

// SlugifyPresetID normalizes a preset name into a stable lowercase identifier.
func SlugifyPresetID(raw string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// BuildCustomModelPreset constructs a ModelPreset from the current 3-scope model routing.
func BuildCustomModelPreset(name string, scopeModels map[string]string, existingCount int) client.ModelPreset {
	analyze := AssignedScopeModelID(scopeModels, "analyze")
	bug := AssignedScopeModelID(scopeModels, "bug")
	code := AssignedScopeModelID(scopeModels, "code")
	cleanName := strings.TrimSpace(name)
	id := SlugifyPresetID(cleanName)
	if cleanName == "" {
		cleanName = fmt.Sprintf("Custom Profile %d (%s/%s)", existingCount+1, analyze, bug)
		id = fmt.Sprintf("custom-%d", existingCount+1)
	} else if id == "" || IsBuiltinModelPreset(id) {
		id = fmt.Sprintf("custom-%s-%d", client.FirstNonEmpty(id, "preset"), existingCount+1)
	}
	return client.ModelPreset{
		ID:      id,
		Name:    cleanName,
		Analyze: analyze,
		Bug:     bug,
		Code:    code,
	}
}

// AssignedScopeModelID returns the user-assigned model/profile ID for a scope.
func AssignedScopeModelID(scopeModels map[string]string, scope string) string {
	if scopeModels != nil && scopeModels[scope] != "" {
		return scopeModels[scope]
	}
	return scope
}

// AvailableModelIDs returns all selectable model IDs across configured scopes and discovered CLI catalogs.
func AvailableModelIDs(avail *client.AvailableModels) []string {
	ids := append([]string(nil), CanonicalModelScopes()...)
	if avail != nil {
		for _, ch := range avail.Models {
			if ch.ID != "" && !ContainsString(ids, ch.ID) {
				ids = append(ids, ch.ID)
			}
		}
	}
	for _, fallback := range []string{"local", "remote-pro", "remote-fast"} {
		if !ContainsString(ids, fallback) {
			ids = append(ids, fallback)
		}
	}
	return ids
}

// EffectiveScopeModel resolves the full runtime Model metadata for the given scope.
func EffectiveScopeModel(scopeModels map[string]string, avail *client.AvailableModels, models *client.ModelCatalog, scope string) client.Model {
	assigned := AssignedScopeModelID(scopeModels, scope)
	if avail != nil {
		for _, ch := range avail.Models {
			if ch.ID == assigned {
				m := ch.Model
				m.Scope = scope
				m.Profile = ch.ID
				if m.Model == "" {
					m.Model = ch.Name
				}
				return m
			}
		}
	}
	if models != nil {
		if m, ok := models.Scopes[assigned]; ok {
			m.Scope = scope
			m.Profile = client.FirstNonEmpty(m.Profile, assigned)
			return m
		}
		if m, ok := models.Scopes[scope]; ok {
			m.Scope = scope
			m.Profile = client.FirstNonEmpty(assigned, m.Profile, scope)
			return m
		}
	}
	remote := strings.Contains(assigned, "remote") || strings.HasPrefix(assigned, "agy:")
	origin := "http://127.0.0.1:11434"
	if remote {
		origin = "remote-provider"
	}
	return client.Model{
		Scope:            scope,
		Profile:          assigned,
		Model:            assigned,
		ProviderOrigin:   origin,
		RemoteProvider:   remote,
		ReasoningEffort:  "medium",
		ContextMaxTokens: 32768,
		Timeout:          "2m0s",
		MaxRetries:       2,
	}
}

// FormatModelParams formats a concise runtime parameter string for a Model.
func FormatModelParams(m client.Model) string {
	effort := client.FirstNonEmpty(m.ReasoningEffort, "default")
	ctxTok := "default"
	if m.ContextMaxTokens > 0 {
		ctxTok = fmt.Sprintf("%dk", max(1, m.ContextMaxTokens/1024))
	}
	maxTok := "auto"
	if m.MaxTokens != nil && *m.MaxTokens > 0 {
		maxTok = fmt.Sprintf("%d", *m.MaxTokens)
	}
	temp := "default"
	if m.Temperature != nil {
		temp = fmt.Sprintf("%.2f", *m.Temperature)
	}
	timeout := client.FirstNonEmpty(m.Timeout, "2m0s")
	return fmt.Sprintf("effort:%s · ctx:%s · max_tokens:%s · temp:%s · timeout:%s · retries:%d",
		effort, ctxTok, maxTok, temp, timeout, m.MaxRetries)
}

// ResolveProjectDocs returns the project's structured handbook or synthesizes a fallback from Overview.
func ResolveProjectDocs(overview *client.Overview, project *client.Project) client.ProjectDocsReport {
	if overview != nil && overview.Docs != nil {
		return *overview.Docs
	}
	purpose := "Local repository workspace managed by Mini-Orca."
	topology := "Layered application repository"
	var comps []client.ProjectDocsComponent
	if overview != nil && overview.Analysis.Purpose != "" {
		purpose = overview.Analysis.Purpose
	} else if project != nil && project.Summary != "" {
		purpose = project.Summary
	}
	if overview != nil {
		if overview.Analysis.Architecture != "" {
			topology = overview.Analysis.Architecture
		}
		for _, c := range overview.Analysis.Components {
			comps = append(comps, client.ProjectDocsComponent{Name: c, Layer: "Core", Summary: c})
		}
	}
	return client.ProjectDocsReport{
		Status: "synthesized",
		Context: client.ProjectDocsContext{
			Purpose:  purpose,
			Topology: topology,
		},
		Components: comps,
	}
}

// RenderArchitectureASCII formats an ASCII box-and-arrow architecture diagram for terminal display.
func RenderArchitectureASCII(docs client.ProjectDocsReport) []string {
	if len(docs.Components) == 0 {
		return []string{
			"╭──────────────────────────────────────────────────────────╮",
			"│  ◆ No architecture layers synthesized yet                │",
			"│  ▸ Press [r] or run :analyze to map repository topology  │",
			"╰──────────────────────────────────────────────────────────╯",
		}
	}
	var lines []string
	for idx, comp := range docs.Components {
		layer := comp.Layer
		if layer == "" {
			layer = "Layer"
		}
		label := fmt.Sprintf("[%s] %s", strings.ToUpper(layer), comp.Name)
		if comp.Path != "" {
			label += " (" + comp.Path + ")"
		}
		lines = append(lines, "╭── ◆ "+label)
		if comp.Summary != "" {
			lines = append(lines, "│   "+comp.Summary)
		}
		if len(comp.Dependencies) > 0 {
			lines = append(lines, "│   deps ──▶ "+strings.Join(comp.Dependencies, ", "))
		}
		lines = append(lines, "╰──────────────────────────────────────────────────────────")
		if idx < len(docs.Components)-1 {
			lines = append(lines, "          │", "          ▼")
		}
	}
	return lines
}

// DefaultAnalysisRunConfig returns the default daemon limits and flags for project analysis runs.
func DefaultAnalysisRunConfig() client.AnalysisRunConfig {
	return client.AnalysisRunConfig{
		BatchFiles:          64,
		BudgetSeconds:       300,
		MaxAttemptsPerStage: 2,
		IncludeFeatures:     true,
		Refresh:             false,
		ClearResults:        false,
	}
}

// NormalizeAnalysisRunConfig clamps limits to daemon bounds and ensures valid flag combinations.
func NormalizeAnalysisRunConfig(cfg client.AnalysisRunConfig) client.AnalysisRunConfig {
	if cfg.BatchFiles <= 0 {
		cfg.BatchFiles = 64
	} else if cfg.BatchFiles > 500 {
		cfg.BatchFiles = 500
	}
	if cfg.BudgetSeconds <= 0 {
		cfg.BudgetSeconds = 300
	} else if cfg.BudgetSeconds > 3600 {
		cfg.BudgetSeconds = 3600
	}
	if cfg.MaxAttemptsPerStage <= 0 {
		cfg.MaxAttemptsPerStage = 2
	} else if cfg.MaxAttemptsPerStage > 4 {
		cfg.MaxAttemptsPerStage = 4
	}
	if cfg.ClearResults {
		cfg.Refresh = true
	}
	return cfg
}

// StepAnalysisBatchFiles cycles through canonical batch file sizes (16..500).
func StepAnalysisBatchFiles(cur int, forward bool) int {
	return stepIntChoices([]int{16, 32, 64, 128, 256, 500}, cur, forward)
}

// StepAnalysisBudgetSeconds cycles through canonical analysis time budgets (60s..3600s).
func StepAnalysisBudgetSeconds(cur int, forward bool) int {
	return stepIntChoices([]int{60, 120, 300, 600, 900, 1800, 3600}, cur, forward)
}

// StepAnalysisMaxAttempts cycles through canonical stage retry limits (1..4).
func StepAnalysisMaxAttempts(cur int, forward bool) int {
	return stepIntChoices([]int{1, 2, 3, 4}, cur, forward)
}

func stepIntChoices(choices []int, cur int, forward bool) int {
	pos := 0
	found := false
	for i, v := range choices {
		if v >= cur {
			pos = i
			found = true
			break
		}
	}
	if !found {
		pos = len(choices) - 1
	}
	if forward {
		return choices[(pos+1)%len(choices)]
	}
	return choices[(pos-1+len(choices))%len(choices)]
}

// AnalysisConfigRowSpec describes a single interactive row in the Analysis Run Configuration panel.
type AnalysisConfigRowSpec struct {
	Key         string
	Label       string
	Value       string
	Badge       string
	Description string
}

// BuildAnalysisConfigRows builds the 5 interactive configuration rows for PageAnalysisRun.
func BuildAnalysisConfigRows(cfg client.AnalysisRunConfig) []AnalysisConfigRowSpec {
	norm := NormalizeAnalysisRunConfig(cfg)
	featBadge := "ENABLED"
	featVal := "Synthesize feature proposals"
	if !norm.IncludeFeatures {
		featBadge = "SKIPPED"
		featVal = "Skip feature generation stage"
	}
	cacheBadge := "INCREMENTAL"
	cacheVal := "Reuse cached unchanged files"
	if norm.ClearResults {
		cacheBadge = "CLEAN"
		cacheVal = "Force refresh & clear prior results"
	} else if norm.Refresh {
		cacheBadge = "REFRESH"
		cacheVal = "Bypass cache & re-evaluate all files"
	}
	return []AnalysisConfigRowSpec{
		{
			Key:         "[b/B]",
			Label:       "Batch Files Limit",
			Value:       fmt.Sprintf("%d files / batch", norm.BatchFiles),
			Badge:       fmt.Sprintf("%d FILES", norm.BatchFiles),
			Description: "Max repository files dispatched per analysis batch (1..500, :batch <n>)",
		},
		{
			Key:         "[t/T]",
			Label:       "Execution Budget",
			Value:       fmt.Sprintf("%ds (%dm %ds)", norm.BudgetSeconds, norm.BudgetSeconds/60, norm.BudgetSeconds%60),
			Badge:       fmt.Sprintf("%ds", norm.BudgetSeconds),
			Description: "Wall-clock execution time allowance before pausing run (1..3600s, :budget <s>)",
		},
		{
			Key:         "[a/A]",
			Label:       "Max Stage Attempts",
			Value:       fmt.Sprintf("%d attempts / stage", norm.MaxAttemptsPerStage),
			Badge:       fmt.Sprintf("%d TRIES", norm.MaxAttemptsPerStage),
			Description: "Maximum LLM retry attempts per file analysis stage (1..4, :attempts <n>)",
		},
		{
			Key:         "[f]",
			Label:       "Feature Discovery",
			Value:       featVal,
			Badge:       featBadge,
			Description: "Toggle goal-aligned feature & capability synthesis during project analysis",
		},
		{
			Key:         "[c]",
			Label:       "Cache & Refresh Policy",
			Value:       cacheVal,
			Badge:       cacheBadge,
			Description: "Cycle Incremental (cached) -> Refresh (bypass cache) -> Clean Reanalyze (clear findings)",
		},
	}
}
