package handbook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project/insight"
	"github.com/nanaki-93/mini-orca/v2/internal/project/symbols"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	ProjectAnalysisStatusFresh       = "fresh"
	ProjectAnalysisStatusFailed      = "failed"
	ProjectAnalysisStatusUnavailable = "unavailable"
	ProjectAnalysisStatusStale       = "stale"
)

type EngineeringInsight = insight.EngineeringInsight
type ProjectIndex = symbols.ProjectIndex
type IndexFile = symbols.IndexFile
type SymbolInfo = symbols.SymbolInfo

// ProjectAnalysisRisk is a bounded model interpretation, not a verified issue.
type ProjectAnalysisRisk struct {
	Severity           string              `json:"severity"`
	Summary            string              `json:"summary"`
	EngineeringInsight *EngineeringInsight `json:"engineering_insight,omitempty"`
}

// ProjectAnalysisReport is the authoritative, persisted interpretation of one
// project revision.
type ProjectAnalysisReport struct {
	SchemaVersion      string                `json:"schema_version"`
	ProjectID          string                `json:"project_id"`
	ProjectRevision    string                `json:"project_revision"`
	Purpose            string                `json:"purpose,omitempty"`
	Architecture       string                `json:"architecture,omitempty"`
	Components         []string              `json:"components"`
	EntryPoints        []string              `json:"entry_points"`
	Flows              []string              `json:"flows"`
	Risks              []ProjectAnalysisRisk `json:"risks"`
	NextSteps          []string              `json:"next_steps"`
	EngineeringInsight *EngineeringInsight   `json:"engineering_insight,omitempty"`
	Status             string                `json:"status"`
	Failure            string                `json:"failure,omitempty"`
	Model              string                `json:"model"`
	ConfiguredModel    string                `json:"configured_model,omitempty"`
	Profile            string                `json:"profile"`
	Scope              string                `json:"scope,omitempty"`
	ProviderOrigin     string                `json:"provider_origin,omitempty"`
	ReasoningEffort    string                `json:"reasoning_effort,omitempty"`
	PromptVersion      string                `json:"prompt_version"`
	GeneratedAt        time.Time             `json:"generated_at"`
}

// Analysis is the persisted and API-facing summary for an imported project.
type Analysis struct {
	ProjectID       string                `json:"project_id"`
	ProjectRevision string                `json:"project_revision"`
	Name            string                `json:"name"`
	Path            string                `json:"path"`
	Type            string                `json:"type"`
	BuildFile       string                `json:"build_file,omitempty"`
	FileCount       int                   `json:"file_count"`
	SourceFileCount int                   `json:"source_file_count"`
	TotalLines      int                   `json:"total_lines"`
	Languages       map[string]int        `json:"languages"`
	Files           []string              `json:"files"`
	Summary         string                `json:"summary"`
	AIStatus        string                `json:"ai_status"`
	AnalyzedAt      time.Time             `json:"analyzed_at"`
	Report          ProjectAnalysisReport `json:"report"`
}

const (
	projectDocsSchemaVersion = "1"
	projectDocsReportPath    = ".mini-orca/project-docs.json"
)

// ProjectDocsContext describes C4 Level 1-2 system context, goal pursuit, and technology stack.
type ProjectDocsContext struct {
	Purpose             string   `json:"purpose"`
	GoalPursuit         []string `json:"goal_pursuit,omitempty"`
	Topology            string   `json:"topology"`
	Stack               []string `json:"stack"`
	ArchitectureDiagram string   `json:"architecture_diagram,omitempty"`
}

// ProjectDocsDependency describes a primary runtime, library, or module dependency.
type ProjectDocsDependency struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Version string `json:"version,omitempty"`
	Source  string `json:"source,omitempty"`
	Summary string `json:"summary"`
}

// ProjectDocsContract describes an exposed interface or boundary contract of a component.
type ProjectDocsContract struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

// ProjectDocsComponent describes a C4 Level 3 bounded context or subsystem.
type ProjectDocsComponent struct {
	Name             string                `json:"name"`
	Layer            string                `json:"layer"`
	Path             string                `json:"path"`
	Summary          string                `json:"summary"`
	Responsibilities []string              `json:"responsibilities"`
	Contracts        []ProjectDocsContract `json:"contracts"`
	Dependencies     []string              `json:"dependencies"`
}

// ProjectDocsScenarioStep describes one step in a behavioral runtime scenario.
type ProjectDocsScenarioStep struct {
	Phase  string `json:"phase"`
	Layer  string `json:"layer"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Path   string `json:"path,omitempty"`
}

// ProjectDocsScenario describes an end-to-end runtime data flow and its invariant.
type ProjectDocsScenario struct {
	Title     string                    `json:"title"`
	Trigger   string                    `json:"trigger"`
	Invariant string                    `json:"invariant"`
	Summary   string                    `json:"summary"`
	Diagram   string                    `json:"diagram,omitempty"`
	Steps     []ProjectDocsScenarioStep `json:"steps"`
}

// ProjectDocsDecision records an Architectural Decision Record (ADR) and its tradeoff.
type ProjectDocsDecision struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Decision string `json:"decision"`
	Tradeoff string `json:"tradeoff"`
}

// ProjectDocsRunbookItem describes a standard operational command for the repository.
type ProjectDocsRunbookItem struct {
	Label       string `json:"label"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ProjectDocsGlossaryItem defines one ubiquitous domain term in the system.
type ProjectDocsGlossaryItem struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// ProjectDocsReport is the language-agnostic C4/Arc42/ADR engineering specification
// generated during project analysis.
type ProjectDocsReport struct {
	SchemaVersion   string                    `json:"schema_version"`
	ProjectID       string                    `json:"project_id"`
	ProjectRevision string                    `json:"project_revision"`
	Status          string                    `json:"status"`
	Context         ProjectDocsContext        `json:"context"`
	Dependencies    []ProjectDocsDependency   `json:"dependencies,omitempty"`
	Components      []ProjectDocsComponent    `json:"components"`
	Scenarios       []ProjectDocsScenario     `json:"scenarios"`
	Decisions       []ProjectDocsDecision     `json:"decisions"`
	Runbook         []ProjectDocsRunbookItem  `json:"runbook"`
	Glossary        []ProjectDocsGlossaryItem `json:"glossary"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

// BuildProjectDocs synthesizes a language-agnostic engineering specification
// from the project's analysis report and deterministic file index.
func BuildProjectDocs(analysis Analysis, index *ProjectIndex) ProjectDocsReport {
	status := analysis.Report.Status
	if status == "" {
		status = ProjectAnalysisStatusUnavailable
	}
	components := buildDocsComponents(analysis, index)
	contextSpec := buildDocsContext(analysis, components)
	dependencies := buildDocsDependencies(analysis, index, components)
	scenarios := buildDocsScenarios(analysis, components)
	decisions := buildDocsDecisions(analysis, components)
	runbook := buildDocsRunbook(analysis, index)
	glossary := buildDocsGlossary(analysis, components)
	updatedAt := analysis.Report.GeneratedAt
	if updatedAt.IsZero() {
		updatedAt = analysis.AnalyzedAt
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	return ProjectDocsReport{
		SchemaVersion:   projectDocsSchemaVersion,
		ProjectID:       analysis.ProjectID,
		ProjectRevision: analysis.ProjectRevision,
		Status:          status,
		Context:         contextSpec,
		Dependencies:    dependencies,
		Components:      components,
		Scenarios:       scenarios,
		Decisions:       decisions,
		Runbook:         runbook,
		Glossary:        glossary,
		UpdatedAt:       updatedAt,
	}
}

// StoreProjectDocsReport atomically persists the engineering specification under .mini-orca.
func StoreProjectDocsReport(root string, report ProjectDocsReport) error {
	if report.SchemaVersion == "" {
		report.SchemaVersion = projectDocsSchemaVersion
	}
	if report.UpdatedAt.IsZero() {
		report.UpdatedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project docs report: %w", err)
	}
	if err := storage.WriteFile(filepath.Join(root, projectDocsReportPath), data, 0600); err != nil {
		return fmt.Errorf("store project docs report: %w", err)
	}
	return nil
}

// LoadProjectDocsReport reads a stored engineering specification and marks it
// stale when the project revision has changed.
func LoadProjectDocsReport(root, projectID, projectRevision string) (*ProjectDocsReport, error) {
	data, err := os.ReadFile(filepath.Join(root, projectDocsReportPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project docs report: %w", err)
	}
	var report ProjectDocsReport
	if err := json.Unmarshal(data, &report); err != nil || report.SchemaVersion != projectDocsSchemaVersion || report.ProjectID == "" {
		return nil, nil
	}
	if projectID != "" && report.ProjectID != projectID {
		return nil, nil
	}
	if projectRevision != "" && report.ProjectRevision != projectRevision && report.Status == ProjectAnalysisStatusFresh {
		report.Status = ProjectAnalysisStatusStale
	}
	return &report, nil
}

func buildDocsContext(analysis Analysis, components []ProjectDocsComponent) ProjectDocsContext {
	purpose := strings.TrimSpace(analysis.Report.Purpose)
	if purpose == "" {
		name := analysis.Name
		if name == "" {
			name = "workspace"
		}
		purpose = fmt.Sprintf("%s software system (%d source files across %d indexed files).", name, analysis.SourceFileCount, analysis.FileCount)
	}
	arch := strings.TrimSpace(analysis.Report.Architecture)
	diagram := ""
	topology := arch
	if isMermaidDiagram(arch) {
		diagram = arch
		topology = summarizeLayerTopology(components)
	} else if topology == "" {
		topology = summarizeLayerTopology(components)
	}
	stack := buildDocsStack(analysis)
	goalPursuit := buildDocsGoalPursuit(analysis, components, topology)
	return ProjectDocsContext{
		Purpose:             purpose,
		GoalPursuit:         goalPursuit,
		Topology:            topology,
		Stack:               stack,
		ArchitectureDiagram: diagram,
	}
}

func buildDocsGoalPursuit(analysis Analysis, components []ProjectDocsComponent, topology string) []string {
	steps := make([]string, 0, 4)
	if insight := analysis.Report.EngineeringInsight; insight != nil && strings.TrimSpace(insight.Mechanism) != "" {
		why := strings.TrimSpace(insight.WhyItMattersHere)
		if why != "" {
			steps = append(steps, fmt.Sprintf("%s — %s", strings.TrimSpace(insight.Mechanism), why))
		} else {
			steps = append(steps, strings.TrimSpace(insight.Mechanism))
		}
	}
	if len(analysis.Report.EntryPoints) > 0 {
		steps = append(steps, fmt.Sprintf("Entry & Admission: Starts from %s and validates inputs before domain execution.", strings.Join(analysis.Report.EntryPoints, ", ")))
	}
	for _, comp := range components {
		if len(steps) >= 4 {
			break
		}
		if comp.Summary != "" {
			steps = append(steps, fmt.Sprintf("%s (%s): %s", comp.Name, comp.Layer, comp.Summary))
		}
	}
	if len(steps) == 0 && topology != "" {
		steps = append(steps, topology)
	}
	return steps
}

func buildDocsDependencies(analysis Analysis, index *ProjectIndex, components []ProjectDocsComponent) []ProjectDocsDependency {
	deps := make([]ProjectDocsDependency, 0, 8)
	seen := make(map[string]bool)
	add := func(dep ProjectDocsDependency) {
		key := strings.ToLower(strings.TrimSpace(dep.Name))
		if key == "" || seen[key] || len(deps) >= 12 {
			return
		}
		seen[key] = true
		deps = append(deps, dep)
	}

	if analysis.Path != "" {
		for _, dep := range parseGoModDependencies(filepath.Join(analysis.Path, "go.mod")) {
			add(dep)
		}
		for _, relPkg := range []string{"package.json", filepath.Join("desktop", "frontend", "package.json")} {
			for _, dep := range parsePackageJSONDependencies(filepath.Join(analysis.Path, relPkg), relPkg) {
				add(dep)
			}
		}
	}

	if analysis.Type != "" && analysis.Type != "unknown" {
		source := analysis.BuildFile
		if source == "" {
			source = "repository root"
		}
		add(ProjectDocsDependency{
			Name:    strings.ToUpper(analysis.Type[:1]) + analysis.Type[1:] + " Runtime & Toolchain",
			Kind:    "Runtime",
			Source:  source,
			Summary: fmt.Sprintf("Primary build and execution environment (%s).", source),
		})
	}

	for _, entry := range buildDocsStack(analysis) {
		add(ProjectDocsDependency{
			Name:    entry,
			Kind:    "Language / Stack",
			Source:  "project index",
			Summary: "Indexed source language and toolchain footprint in the repository.",
		})
	}

	for _, comp := range components {
		for _, d := range comp.Dependencies {
			add(ProjectDocsDependency{
				Name:    d,
				Kind:    "Module Dependency",
				Source:  comp.Path,
				Summary: fmt.Sprintf("Imported by %s (%s).", comp.Name, comp.Layer),
			})
		}
	}

	return deps
}

func parseGoModDependencies(goModPath string) []ProjectDocsDependency {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	var out []ProjectDocsDependency
	inRequire := false
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "go ") {
			ver := strings.TrimSpace(strings.TrimPrefix(line, "go "))
			out = append(out, ProjectDocsDependency{
				Name:    "Go Standard Library & Runtime",
				Kind:    "Runtime",
				Version: "go " + ver,
				Source:  "go.mod",
				Summary: "Core Go runtime, concurrency primitives, HTTP/JSON, and standard packages.",
			})
			continue
		}
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if strings.Contains(line, "// indirect") {
			continue
		}
		reqLine := ""
		if strings.HasPrefix(line, "require ") {
			reqLine = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		} else if inRequire {
			reqLine = line
		}
		if reqLine != "" {
			fields := strings.Fields(reqLine)
			if len(fields) >= 2 {
				out = append(out, ProjectDocsDependency{
					Name:    fields[0],
					Kind:    "Go Module",
					Version: fields[1],
					Source:  "go.mod",
					Summary: fmt.Sprintf("Direct Go module dependency declared in go.mod (%s).", fields[1]),
				})
			}
		}
	}
	return out
}

func parsePackageJSONDependencies(pkgPath, sourceLabel string) []ProjectDocsDependency {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}
	var parsed struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	names := make([]string, 0, len(parsed.Dependencies)+len(parsed.DevDependencies))
	versions := make(map[string]string)
	kinds := make(map[string]string)
	for k, v := range parsed.Dependencies {
		names = append(names, k)
		versions[k] = v
		kinds[k] = "NPM Package"
	}
	for k, v := range parsed.DevDependencies {
		if _, exists := versions[k]; !exists {
			names = append(names, k)
			versions[k] = v
			kinds[k] = "Dev / Build Tool"
		}
	}
	sort.Strings(names)
	out := make([]ProjectDocsDependency, 0, len(names))
	for _, name := range names {
		out = append(out, ProjectDocsDependency{
			Name:    name,
			Kind:    kinds[name],
			Version: versions[name],
			Source:  sourceLabel,
			Summary: fmt.Sprintf("Frontend/Node package declared in %s.", sourceLabel),
		})
	}
	return out
}

func isMermaidDiagram(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "flowchart ") ||
		strings.HasPrefix(trimmed, "graph ") ||
		strings.HasPrefix(trimmed, "sequenceDiagram")
}

func summarizeLayerTopology(components []ProjectDocsComponent) string {
	if len(components) == 0 {
		return "Single-tier repository structure with direct source execution."
	}
	layers := make([]string, 0, 4)
	seen := make(map[string]bool)
	for _, comp := range components {
		if !seen[comp.Layer] {
			seen[comp.Layer] = true
			layers = append(layers, comp.Layer)
		}
	}
	return fmt.Sprintf("Layered architecture spanning %d bounded subsystems across %s.", len(components), strings.Join(layers, ", "))
}

func buildDocsStack(analysis Analysis) []ProjectDocsStackEntry {
	type langCount struct {
		lang  string
		count int
	}
	items := make([]langCount, 0, len(analysis.Languages))
	for lang, count := range analysis.Languages {
		items = append(items, langCount{lang: lang, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].lang < items[j].lang
	})
	stack := make([]string, 0, len(items)+2)
	if analysis.Type != "" && analysis.Type != "unknown" {
		if analysis.BuildFile != "" {
			stack = append(stack, fmt.Sprintf("%s (%s)", strings.ToUpper(analysis.Type[:1])+analysis.Type[1:], analysis.BuildFile))
		} else {
			stack = append(stack, strings.ToUpper(analysis.Type[:1])+analysis.Type[1:])
		}
	}
	for _, item := range items {
		stack = append(stack, fmt.Sprintf("%s (%d files)", item.lang, item.count))
	}
	if len(stack) == 0 {
		stack = append(stack, "Polyglot Source Tree")
	}
	return stack
}

type ProjectDocsStackEntry = string

func buildDocsComponents(analysis Analysis, index *ProjectIndex) []ProjectDocsComponent {
	byPath := make(map[string]*ProjectDocsComponent)
	order := make([]string, 0, 12)
	for _, raw := range analysis.Report.Components {
		comp := parseReportComponent(raw)
		if comp.Path == "" {
			continue
		}
		if _, exists := byPath[comp.Path]; !exists {
			order = append(order, comp.Path)
			c := comp
			byPath[comp.Path] = &c
		}
	}
	enrichComponentsFromIndex(index, byPath, &order)
	result := make([]ProjectDocsComponent, 0, len(order))
	allNames := make([]string, 0, len(order))
	for _, p := range order {
		allNames = append(allNames, byPath[p].Name)
	}
	for i, p := range order {
		comp := byPath[p]
		if len(comp.Dependencies) == 0 && len(allNames) > 1 {
			next := allNames[(i+1)%len(allNames)]
			if next != comp.Name {
				comp.Dependencies = []string{next}
			}
		}
		result = append(result, *comp)
	}
	return result
}

func parseReportComponent(raw string) ProjectDocsComponent {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ProjectDocsComponent{}
	}
	pathPart, rest := splitComponentHeader(trimmed)
	name, cleanPath := deriveComponentIdentity(pathPart)
	layer := classifyComponentLayer(cleanPath, name)
	summary := rest
	if summary == "" {
		summary = fmt.Sprintf("Owns %s responsibilities within %s.", name, layer)
	}
	return ProjectDocsComponent{
		Name:             name,
		Layer:            layer,
		Path:             cleanPath,
		Summary:          summary,
		Responsibilities: []string{summary},
		Contracts: []ProjectDocsContract{
			{
				Name:    name + " Boundary Contract",
				Kind:    contractKindForLayer(layer),
				Summary: summary,
			},
		},
		Dependencies: []string{},
	}
}

func splitComponentHeader(trimmed string) (string, string) {
	colonIdx := strings.Index(trimmed, ":")
	if colonIdx <= 0 {
		return trimmed, ""
	}
	return strings.TrimSpace(trimmed[:colonIdx]), strings.TrimSpace(trimmed[colonIdx+1:])
}

func deriveComponentIdentity(header string) (string, string) {
	if openIdx := strings.Index(header, "("); openIdx > 0 && strings.HasSuffix(header, ")") {
		cleanPath := strings.TrimSpace(header[:openIdx])
		label := strings.TrimSpace(header[openIdx+1 : len(header)-1])
		if label != "" {
			return label, cleanPath
		}
	}
	cleanPath := strings.Trim(header, "` ")
	base := filepath.Base(cleanPath)
	if base == "." || base == "/" || base == "" {
		base = cleanPath
	}
	return humanizeComponentName(base), cleanPath
}

func humanizeComponentName(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '_' || r == '-' || r == '/' || r == '.'
	})
	if len(parts) == 0 {
		return "Core Subsystem"
	}
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func classifyComponentLayer(path, name string) string {
	lower := strings.ToLower(path + " " + name)
	switch {
	case containsAnyToken(lower, "desktop", "frontend", "ui", "view", "web", "client"):
		return "Presentation Layer"
	case containsAnyToken(lower, "api", "handler", "route", "http", "transport", "cmd", "daemon", "cli"):
		return "API / Transport Boundary"
	case containsAnyToken(lower, "app", "service", "workflow", "orchestrat", "job", "usecase"):
		return "Application / Orchestration"
	case containsAnyToken(lower, "script", "test", "eval", "bench", "ci", "quality"):
		return "Tooling & Verification"
	default:
		return "Domain & Persistence"
	}
}

func containsAnyToken(haystack string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

func contractKindForLayer(layer string) string {
	switch layer {
	case "Presentation Layer":
		return "UI State & View Contract"
	case "API / Transport Boundary":
		return "Transport & Schema Boundary"
	case "Application / Orchestration":
		return "Transactional Workflow Contract"
	case "Tooling & Verification":
		return "Verification Gate Contract"
	default:
		return "Domain & Storage Invariant"
	}
}

func enrichComponentsFromIndex(index *ProjectIndex, byPath map[string]*ProjectDocsComponent, order *[]string) {
	if index == nil {
		return
	}
	groupFiles := make(map[string][]IndexFile)
	groupOrder := make([]string, 0, 8)
	for _, file := range index.Files {
		if file.Binary || file.Language == "Text" {
			continue
		}
		dir := subsystemDir(file.Path)
		if _, exists := groupFiles[dir]; !exists {
			groupOrder = append(groupOrder, dir)
		}
		groupFiles[dir] = append(groupFiles[dir], file)
	}
	for _, dir := range groupOrder {
		files := groupFiles[dir]
		comp, exists := byPath[dir]
		if !exists && len(*order) < 12 {
			name := humanizeComponentName(dir)
			layer := classifyComponentLayer(dir, name)
			summary := fmt.Sprintf("Subsystem at %s (%d files) governing %s operations.", dir, len(files), strings.ToLower(layer))
			created := ProjectDocsComponent{
				Name:             name,
				Layer:            layer,
				Path:             dir,
				Summary:          summary,
				Responsibilities: []string{summary},
				Contracts:        []ProjectDocsContract{},
				Dependencies:     []string{},
			}
			byPath[dir] = &created
			*order = append(*order, dir)
			comp = &created
		}
		if comp != nil {
			appendIndexContracts(comp, files)
		}
	}
}

func subsystemDir(relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) == 1 {
		return parts[0]
	}
	if (parts[0] == "internal" || parts[0] == "cmd" || parts[0] == "pkg" || parts[0] == "src" || parts[0] == "desktop") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func appendIndexContracts(comp *ProjectDocsComponent, files []IndexFile) {
	for _, f := range files {
		if len(comp.Responsibilities) < 4 {
			role := fmt.Sprintf("Maintains %s (%s, %d lines)", filepath.Base(f.Path), f.Language, f.LineCount)
			comp.Responsibilities = appendUniqueString(comp.Responsibilities, role)
		}
		for _, sym := range f.Symbols {
			if len(comp.Contracts) >= 4 {
				break
			}
			comp.Contracts = append(comp.Contracts, ProjectDocsContract{
				Name:    sym.Name,
				Kind:    contractKindForLayer(comp.Layer),
				Summary: fmt.Sprintf("Exposed %s contract in %s (lines %d-%d).", sym.Kind, f.Path, sym.StartLine, sym.EndLine),
			})
		}
	}
	if len(comp.Contracts) == 0 {
		comp.Contracts = append(comp.Contracts, ProjectDocsContract{
			Name:    comp.Name + " Module Boundary",
			Kind:    contractKindForLayer(comp.Layer),
			Summary: comp.Summary,
		})
	}
}

func appendUniqueString(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}

func buildDocsScenarios(analysis Analysis, components []ProjectDocsComponent) []ProjectDocsScenario {
	scenarios := make([]ProjectDocsScenario, 0, 4)
	for i, flow := range analysis.Report.Flows {
		if len(scenarios) >= 6 {
			break
		}
		scenarios = append(scenarios, scenarioFromFlow(i, flow, analysis.Report.EntryPoints, components))
	}
	if len(scenarios) == 0 {
		scenarios = append(scenarios, defaultScenarioForComponents(analysis, components))
	}
	return scenarios
}

func scenarioFromFlow(index int, flow string, entryPoints []string, components []ProjectDocsComponent) ProjectDocsScenario {
	trigger := "System Entry Trigger"
	if index < len(entryPoints) && strings.TrimSpace(entryPoints[index]) != "" {
		trigger = strings.TrimSpace(entryPoints[index])
	} else if len(entryPoints) > 0 {
		trigger = strings.TrimSpace(entryPoints[0])
	}
	diagram := ""
	summary := strings.TrimSpace(flow)
	title := fmt.Sprintf("Runtime Execution Scenario %d", index+1)
	if isMermaidDiagram(flow) {
		diagram = flow
		summary = extractMermaidSummary(flow)
	} else if len(summary) > 72 {
		title = summary[:68] + "..."
	} else if summary != "" {
		title = summary
	}
	return ProjectDocsScenario{
		Title:     title,
		Trigger:   trigger,
		Invariant: "Boundary Validation & State Consistency",
		Summary:   summary,
		Diagram:   diagram,
		Steps:     buildScenarioSteps(components, trigger, summary),
	}
}

func extractMermaidSummary(diagram string) string {
	lines := strings.Split(diagram, "\n")
	nodes := make([]string, 0, 4)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "flowchart") || strings.HasPrefix(trimmed, "sequenceDiagram") || strings.HasPrefix(trimmed, "subgraph") || trimmed == "end" {
			continue
		}
		nodes = append(nodes, trimmed)
		if len(nodes) >= 3 {
			break
		}
	}
	if len(nodes) == 0 {
		return "End-to-end execution path across system boundaries."
	}
	return "Execution path: " + strings.Join(nodes, " · ")
}

func buildScenarioSteps(components []ProjectDocsComponent, trigger, summary string) []ProjectDocsScenarioStep {
	firstPath, firstLayer := "", "API / Transport Boundary"
	midPath, midLayer := "", "Application / Orchestration"
	lastPath, lastLayer := "", "Domain & Persistence"
	if len(components) > 0 {
		firstPath, firstLayer = components[0].Path, components[0].Layer
		mid := components[len(components)/2]
		midPath, midLayer = mid.Path, mid.Layer
		last := components[len(components)-1]
		lastPath, lastLayer = last.Path, last.Layer
	}
	return []ProjectDocsScenarioStep{
		{
			Phase:  "Admission",
			Layer:  firstLayer,
			Title:  "Boundary Admission & Input Validation",
			Detail: fmt.Sprintf("Receives trigger (%s) and validates request preconditions before dispatch.", trigger),
			Path:   firstPath,
		},
		{
			Phase:  "Execution",
			Layer:  midLayer,
			Title:  "Core Workflow Orchestration",
			Detail: summary,
			Path:   midPath,
		},
		{
			Phase:  "Publication",
			Layer:  lastLayer,
			Title:  "State Publication & Side-Effect Guard",
			Detail: "Commits verified state transitions and returns structured output to the caller.",
			Path:   lastPath,
		},
	}
}

func defaultScenarioForComponents(analysis Analysis, components []ProjectDocsComponent) ProjectDocsScenario {
	trigger := "Primary Entry Point"
	if len(analysis.Report.EntryPoints) > 0 {
		trigger = analysis.Report.EntryPoints[0]
	} else if analysis.BuildFile != "" {
		trigger = analysis.BuildFile
	}
	summary := "Primary request admission, domain processing, and state publication across project subsystems."
	if analysis.Report.Purpose != "" {
		summary = analysis.Report.Purpose
	}
	return ProjectDocsScenario{
		Title:     "Primary System Execution Flow",
		Trigger:   trigger,
		Invariant: "Explicit Boundary & State Guard",
		Summary:   summary,
		Steps:     buildScenarioSteps(components, trigger, summary),
	}
}

func buildDocsDecisions(analysis Analysis, components []ProjectDocsComponent) []ProjectDocsDecision {
	decisions := make([]ProjectDocsDecision, 0, 4)
	if insight := analysis.Report.EngineeringInsight; insight != nil {
		decisions = append(decisions, ProjectDocsDecision{
			Title:    "Core Architectural Mechanism",
			Category: "System Architecture",
			Decision: insight.Mechanism + " — " + insight.WhyItMattersHere,
			Tradeoff: insight.TradeoffOrFailureMode,
		})
	}
	for _, risk := range analysis.Report.Risks {
		if len(decisions) >= 4 {
			break
		}
		if risk.EngineeringInsight != nil {
			decisions = append(decisions, ProjectDocsDecision{
				Title:    fmt.Sprintf("Boundary Safeguard (%s priority)", strings.ToUpper(risk.Severity)),
				Category: "Resilience & Safety",
				Decision: risk.EngineeringInsight.Mechanism,
				Tradeoff: risk.EngineeringInsight.TradeoffOrFailureMode,
			})
		} else if risk.Summary != "" {
			decisions = append(decisions, ProjectDocsDecision{
				Title:    fmt.Sprintf("Operational Risk Guard (%s)", strings.ToUpper(risk.Severity)),
				Category: "Quality & Verification",
				Decision: risk.Summary,
				Tradeoff: "Requires explicit test coverage and boundary validation during changes.",
			})
		}
	}
	if len(decisions) == 0 {
		decisions = append(decisions, ProjectDocsDecision{
			Title:    "Modular Subsystem Separation",
			Category: "Architecture Boundary",
			Decision: fmt.Sprintf("Partitions the codebase into %d explicit subsystems with distinct layer responsibilities.", max(1, len(components))),
			Tradeoff: "Cross-cutting changes must coordinate contracts across layer boundaries.",
		})
	}
	return decisions
}

func buildDocsRunbook(analysis Analysis, index *ProjectIndex) []ProjectDocsRunbookItem {
	hasFile := func(name string) bool {
		if index != nil {
			for _, f := range index.Files {
				if f.Path == name {
					return true
				}
			}
		}
		for _, p := range analysis.Files {
			if p == name {
				return true
			}
		}
		return false
	}
	items := make([]ProjectDocsRunbookItem, 0, 4)
	if hasFile("scripts/validate.sh") {
		items = append(items, ProjectDocsRunbookItem{
			Label:       "Full Validation Gate",
			Command:     "./scripts/validate.sh",
			Description: "Runs formatting, static analysis, unit/race tests, and quality checks.",
		})
	}
	if hasFile("Makefile") {
		items = append(items, ProjectDocsRunbookItem{
			Label:       "Standard Check Suite",
			Command:     "make check",
			Description: "Executes configured repository build and test verification targets.",
		})
	}
	switch analysis.Type {
	case "go":
		items = append(items, ProjectDocsRunbookItem{
			Label:       "Go Package Tests",
			Command:     "go test ./...",
			Description: "Runs deterministic unit and integration tests across all Go packages.",
		})
	case "node", "typescript", "javascript":
		items = append(items, ProjectDocsRunbookItem{
			Label:       "Frontend / Node Test Suite",
			Command:     "npm test",
			Description: "Executes the package.json test runner.",
		})
	case "python":
		items = append(items, ProjectDocsRunbookItem{
			Label:       "Python Unit Test Discovery",
			Command:     "python3 -m unittest discover",
			Description: "Discovers and executes Python unit test suites.",
		})
	default:
		if len(items) == 0 {
			items = append(items, ProjectDocsRunbookItem{
				Label:       "Inspect Repository Status",
				Command:     "git status --short",
				Description: "Verifies working tree cleanliness before and after changes.",
			})
		}
	}
	return items
}

func buildDocsGlossary(analysis Analysis, components []ProjectDocsComponent) []ProjectDocsGlossaryItem {
	glossary := make([]ProjectDocsGlossaryItem, 0, len(components)+2)
	for _, comp := range components {
		if len(glossary) >= 6 {
			break
		}
		glossary = append(glossary, ProjectDocsGlossaryItem{
			Term:       comp.Name,
			Definition: fmt.Sprintf("%s (%s): %s", comp.Layer, comp.Path, comp.Summary),
		})
	}
	if len(analysis.Report.NextSteps) > 0 {
		glossary = append(glossary, ProjectDocsGlossaryItem{
			Term:       "Verification Priority",
			Definition: analysis.Report.NextSteps[0],
		})
	}
	return glossary
}
