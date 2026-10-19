package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// DiffHunk represents a contiguous group of changed lines with surrounding context.
type DiffHunk struct {
	Index    int
	OldStart int
	OldEnd   int
	NewStart int
	NewEnd   int
	Lines    []DiffLine
}

// IsAddedLine reports whether a diff line kind represents an insertion.
func IsAddedLine(kind string) bool {
	return kind == "add" || kind == "added" || kind == "+"
}

// IsRemovedLine reports whether a diff line kind represents a deletion.
func IsRemovedLine(kind string) bool {
	return kind == "delete" || kind == "del" || kind == "removed" || kind == "-"
}

// SplitDiffIntoHunks groups diff lines into hunks with up to 2 lines of surrounding context.
func SplitDiffIntoHunks(lines []DiffLine) []DiffHunk {
	changed := changedLineIndices(lines)
	if len(changed) == 0 {
		return nil
	}
	ranges := mergeHunkRanges(changed, len(lines))
	out := make([]DiffHunk, 0, len(ranges))
	for i, r := range ranges {
		slice := lines[r[0] : r[1]+1]
		oldStart, oldEnd := lineBounds(slice, true)
		newStart, newEnd := lineBounds(slice, false)
		out = append(out, DiffHunk{
			Index:    i,
			OldStart: oldStart,
			OldEnd:   oldEnd,
			NewStart: newStart,
			NewEnd:   newEnd,
			Lines:    slice,
		})
	}
	return out
}

func changedLineIndices(lines []DiffLine) []int {
	var indices []int
	for idx, line := range lines {
		if IsAddedLine(line.Kind) || IsRemovedLine(line.Kind) {
			indices = append(indices, idx)
		}
	}
	return indices
}

func mergeHunkRanges(changed []int, total int) [][2]int {
	var ranges [][2]int
	for _, idx := range changed {
		start := max(0, idx-2)
		end := min(total-1, idx+2)
		if len(ranges) == 0 || start > ranges[len(ranges)-1][1]+1 {
			ranges = append(ranges, [2]int{start, end})
			continue
		}
		if end > ranges[len(ranges)-1][1] {
			ranges[len(ranges)-1][1] = end
		}
	}
	return ranges
}

func lineBounds(slice []DiffLine, old bool) (int, int) {
	first, last := 0, 0
	for _, line := range slice {
		num := line.NewLine
		if old {
			num = line.OldLine
		}
		if num <= 0 {
			continue
		}
		if first == 0 {
			first = num
		}
		last = num
	}
	if first == 0 {
		return 1, 1
	}
	return first, last
}

// RejectHunkFromDiff reconstructs the proposed file content with the specified hunk reverted.
func RejectHunkFromDiff(allLines []DiffLine, hunk DiffHunk) string {
	hunkRange := findHunkSliceBounds(allLines, hunk)
	var kept []string
	for idx, line := range allLines {
		inHunk := idx >= hunkRange[0] && idx <= hunkRange[1]
		if inHunk {
			if !IsAddedLine(line.Kind) {
				kept = append(kept, line.Text)
			}
		} else if !IsRemovedLine(line.Kind) {
			kept = append(kept, line.Text)
		}
	}
	return strings.Join(kept, "\n")
}

// RejectHunkLines returns an updated diff line slice with the specified hunk reverted to context.
func RejectHunkLines(allLines []DiffLine, hunk DiffHunk) []DiffLine {
	hunkRange := findHunkSliceBounds(allLines, hunk)
	out := make([]DiffLine, 0, len(allLines))
	for idx, line := range allLines {
		if idx >= hunkRange[0] && idx <= hunkRange[1] {
			if IsAddedLine(line.Kind) {
				continue
			}
			if IsRemovedLine(line.Kind) {
				line.Kind = "context"
				line.NewLine = line.OldLine
			}
		}
		out = append(out, line)
	}
	return out
}

func findHunkSliceBounds(allLines []DiffLine, hunk DiffHunk) [2]int {
	if len(hunk.Lines) == 0 || len(allLines) == 0 {
		return [2]int{-1, -1}
	}
	target := hunk.Lines[0]
	for i := 0; i+len(hunk.Lines) <= len(allLines); i++ {
		if allLines[i] == target {
			return [2]int{i, i + len(hunk.Lines) - 1}
		}
	}
	return [2]int{-1, -1}
}

// ClassifyDiffScope returns added, removed, and the precision scope badge for a change session.
func ClassifyDiffScope(changes []FileChange) (added int, removed int, badge string) {
	for _, fc := range changes {
		for _, line := range fc.Diff.Lines {
			if IsAddedLine(line.Kind) {
				added++
			} else if IsRemovedLine(line.Kind) {
				removed++
			}
		}
	}
	delta := added + removed
	switch {
	case delta == 0:
		return added, removed, "IDLE"
	case delta <= 60:
		return added, removed, "SURGICAL"
	case delta <= 200:
		return added, removed, "FOCUSED"
	default:
		return added, removed, "BROAD"
	}
}

var (
	whitespacePattern   = regexp.MustCompile(`\s+`)
	genericTitlePattern = regexp.MustCompile(`(?i)^(file|project) analysis suggestion$`)
	nonAlphaNumPattern  = regexp.MustCompile(`[^a-z0-9]+`)
	trailingPunct       = " \t\n\r.,;:!?…"
)

// FindingTitle normalizes a finding title, falling back to its summary when generic or empty.
func FindingTitle(title, summary string) string {
	norm := strings.TrimSpace(whitespacePattern.ReplaceAllString(title, " "))
	if norm == "" || genericTitlePattern.MatchString(norm) {
		norm = strings.TrimSpace(whitespacePattern.ReplaceAllString(summary, " "))
	}
	if norm == "" {
		return "Untitled finding"
	}
	return norm
}

// FindingName produces a concise 6-word / 60-character label for a finding.
func FindingName(title, summary string) string {
	content := FindingTitle(title, summary)
	words := strings.Split(content, " ")
	limitWords := words
	if len(words) > 6 {
		limitWords = words[:6]
	}
	name := strings.Join(limitWords, " ")
	maxRunes := 60
	if len(words) > 6 {
		maxRunes = 59
	}
	if utf8.RuneCountInString(name) > maxRunes {
		runes := []rune(name)
		name = string(runes[:59])
		if lastSpace := strings.LastIndex(name, " "); lastSpace > 0 {
			name = name[:lastSpace]
		}
	}
	if name == content {
		return name
	}
	return strings.TrimRight(name, trailingPunct) + "…"
}

// IssueRow represents a unified finding row in the TUI Overview and Agent Studio hubs.
type IssueRow struct {
	ID                 string
	Title              string
	Name               string
	Path               string
	Symbol             string
	Line               int
	Severity           string
	Category           string
	Status             string
	CommitOID          string
	AgentPath          string
	Cause              string
	Solution           string
	Target             FindingTarget
	AcceptanceCriteria []string
}

// IssueSummary aggregates open, in-progress, and resolved issue counts by category.
type IssueSummary struct {
	TotalOpen   int
	InProgress  int
	Done        int
	Bugs        int
	Security    int
	Performance int
	Features    int
}

func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

// SortIssuesBySeverity sorts issues in-place from highest severity to lowest.
func SortIssuesBySeverity(rows []IssueRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := severityRank(rows[i].Severity), severityRank(rows[j].Severity)
		if ri != rj {
			return ri < rj
		}
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].Line < rows[j].Line
	})
}

func normalizeFindingText(value string) string {
	return strings.TrimSpace(nonAlphaNumPattern.ReplaceAllString(strings.ToLower(value), " "))
}

func isActiveOrDoneStatus(status string) bool {
	return status == "in_progress" || status == "fixed" || status == "done"
}

func issueSpecificity(item IssueRow) int {
	score := 0
	if isActiveOrDoneStatus(item.Status) {
		score += 100
	}
	if item.Target.Kind == "security" || item.Target.Kind == "performance" {
		score += 20
	}
	if item.Line > 0 {
		score += 10
	}
	if item.Target.Source == "ai" {
		score += 2
	}
	return score
}

// DedupeIssues removes duplicate findings across semantic and dedicated analysis stages.
func DedupeIssues(items []IssueRow) []IssueRow {
	dedicatedPaths := collectDedicatedStagePaths(items)
	var out []IssueRow
	for _, item := range items {
		if shouldSkipUnanchoredSemantic(item, dedicatedPaths) {
			continue
		}
		dupIdx := findDuplicateIssueIndex(out, item)
		if dupIdx == -1 {
			out = append(out, item)
		} else if issueSpecificity(item) > issueSpecificity(out[dupIdx]) {
			out[dupIdx] = item
		}
	}
	return out
}

func collectDedicatedStagePaths(items []IssueRow) map[string]bool {
	paths := make(map[string]bool)
	for _, item := range items {
		if item.Target.Kind == "security" || item.Target.Kind == "performance" {
			cat := item.Category
			if cat == "" {
				cat = item.Target.Kind
			}
			paths[cat+":"+item.Path] = true
		}
	}
	return paths
}

func shouldSkipUnanchoredSemantic(item IssueRow, dedicatedPaths map[string]bool) bool {
	if item.Target.Kind != "semantic" || item.Line > 0 || isActiveOrDoneStatus(item.Status) {
		return false
	}
	return dedicatedPaths[item.Category+":"+item.Path]
}

func findDuplicateIssueIndex(existing []IssueRow, candidate IssueRow) int {
	normTitle := normalizeFindingText(candidate.Title)
	normName := normalizeFindingText(candidate.Name)
	for idx, cur := range existing {
		if cur.Category != candidate.Category || cur.Path != candidate.Path {
			continue
		}
		if isSameIssueIdentity(cur, candidate, normTitle, normName) {
			return idx
		}
	}
	return -1
}

func isSameIssueIdentity(cur, candidate IssueRow, normTitle, normName string) bool {
	curTitle := normalizeFindingText(cur.Title)
	curName := normalizeFindingText(cur.Name)
	sameTitle := (normTitle != "" && normTitle == curTitle) || (normName != "" && normName == curName)
	diffOrigin := cur.Target.Kind != candidate.Target.Kind || cur.Target.Source != candidate.Target.Source
	if sameTitle {
		return diffOrigin || cur.Severity == candidate.Severity
	}
	return candidate.Line > 0 && cur.Line == candidate.Line && diffOrigin
}

// ComputeIssueSummary aggregates category and lifecycle counts from unified issue rows.
func ComputeIssueSummary(issues []IssueRow, features *FeatureReport) IssueSummary {
	var summary IssueSummary
	for _, row := range issues {
		if row.Status == "done" {
			summary.Done++
			continue
		}
		summary.TotalOpen++
		if row.Status == "in_progress" {
			summary.InProgress++
		}
		switch row.Category {
		case "security":
			summary.Security++
		case "performance":
			summary.Performance++
		default:
			summary.Bugs++
		}
	}
	if features != nil {
		for _, idea := range features.Suggestions {
			if idea.Status != "dismissed" {
				summary.Features++
			}
		}
	}
	return summary
}

// AgentItem describes an active or completed 4-stage agent session in Agent Studio.
type AgentItem struct {
	ID             string
	Title          string
	Branch         string
	Path           string
	Stage          string
	StageNumber    int
	Status         string
	ScopeBadge     string
	AddedLines     int
	RemovedLines   int
	ChecksPassed   int
	ChecksTotal    int
	Mergeable      bool
	UndoAvailable  bool
	FindingTargets []FindingTarget
}

// CollectAgentItems derives Agent Studio items from the active change session and Git worktrees.
func CollectAgentItems(change *ChangeSession, receipt *ChangeMutation, gitWS *GitWorkspace) []AgentItem {
	var items []AgentItem
	if change != nil {
		items = append(items, buildAgentItemFromSession(change, receipt, gitWS))
	}
	if gitWS != nil {
		for _, wt := range gitWS.Worktrees {
			if wt.Current || hasAgentPath(items, wt.Path) {
				continue
			}
			items = append(items, buildAgentItemFromWorktree(wt))
		}
	}
	return items
}

func hasAgentPath(items []AgentItem, path string) bool {
	for _, item := range items {
		if item.Path == path && path != "" {
			return true
		}
	}
	return false
}

func buildAgentItemFromSession(change *ChangeSession, receipt *ChangeMutation, gitWS *GitWorkspace) AgentItem {
	added, removed, badge := ClassifyDiffScope(change.Changes)
	passed, total := countPassedChecks(change.Checks)
	stageNum, stageName := classifyAgentStage(change, passed, total)
	branch := "main"
	if change.Git != nil && change.Git.Branch != "" {
		branch = change.Git.Branch
	} else if gitWS != nil && gitWS.Branch != "" {
		branch = gitWS.Branch
	}
	undo := receipt != nil && receipt.UndoAvailable
	title := strings.TrimSpace(change.Title)
	if title == "" {
		title = "Guarded change session"
	}
	return AgentItem{
		ID:             change.ID,
		Title:          title,
		Branch:         branch,
		Path:           firstChangedPath(change),
		Stage:          stageName,
		StageNumber:    stageNum,
		Status:         effectiveAgentSessionStatus(change),
		ScopeBadge:     badge,
		AddedLines:     added,
		RemovedLines:   removed,
		ChecksPassed:   passed,
		ChecksTotal:    total,
		Mergeable:      false,
		UndoAvailable:  undo,
		FindingTargets: change.FindingTargets,
	}
}

func effectiveAgentSessionStatus(change *ChangeSession) string {
	if change == nil {
		return "idle"
	}
	if change.State == "applied" || change.State == "reverted" || change.State == "failed" {
		return change.State
	}
	if change.Workflow != nil && change.Workflow.Status != "" {
		switch change.Workflow.Status {
		case "awaiting_human_review":
			return "ready"
		case "running", "queued", "canceling", "failed", "canceled", "changes_requested":
			return change.Workflow.Status
		}
	}
	if change.State != "" {
		return change.State
	}
	return "working"
}

func buildAgentItemFromWorktree(wt GitWorktree) AgentItem {
	title := fmt.Sprintf("Worktree %s", wt.Branch)
	var targets []FindingTarget
	if len(wt.Sessions) > 0 && wt.Sessions[0].Title != "" {
		title = wt.Sessions[0].Title
		targets = wt.Sessions[0].FindingTargets
	} else if len(wt.Completed) > 0 && wt.Completed[0].Title != "" {
		title = wt.Completed[0].Title
		targets = wt.Completed[0].FindingTargets
	}
	stageName := "Human Review"
	status := "ready"
	if !wt.Mergeable {
		stageName = "Architect & Code"
		status = "working"
	}
	return AgentItem{
		ID:             wt.Path,
		Title:          title,
		Branch:         wt.Branch,
		Path:           wt.Path,
		Stage:          stageName,
		StageNumber:    4,
		Status:         status,
		ScopeBadge:     "WORKTREE",
		Mergeable:      wt.Mergeable,
		FindingTargets: targets,
	}
}

func countPassedChecks(checks []Check) (int, int) {
	passed := 0
	for _, c := range checks {
		if c.State == "passed" {
			passed++
		}
	}
	return passed, len(checks)
}

func classifyAgentStage(change *ChangeSession, passed, total int) (int, string) {
	if change.State == "applied" {
		return 4, "Applied"
	}
	if change.Workflow != nil {
		switch change.Workflow.Status {
		case "awaiting_human_review":
			return 4, "Human Review"
		case "changes_requested":
			return 3, "Critic Review"
		case "running", "queued", "failed":
			return activeWorkflowStage(change.Workflow.Stages)
		}
	}
	if total > 0 && passed == total {
		if change.Workflow != nil && change.Workflow.Review != nil {
			return 4, "Human Review"
		}
		return 3, "Critic Review"
	}
	if total > 0 {
		return 2, "Verify & Test"
	}
	return 1, "Architect & Code"
}

func activeWorkflowStage(stages []ChangeWorkflowStage) (int, string) {
	for idx, st := range stages {
		if st.Status == "running" || st.Status == "failed" {
			return min(4, idx+1), CanonicalStageLabel(idx, st.Name)
		}
	}
	for idx, st := range stages {
		if st.Status == "pending" || st.Status == "queued" {
			return min(4, idx+1), CanonicalStageLabel(idx, st.Name)
		}
	}
	return 1, "Architect & Code"
}

// CanonicalStageLabel maps daemon stage indices and names (create, test, review, human_review) to display labels.
func CanonicalStageLabel(idx int, rawName string) string {
	switch strings.ToLower(strings.TrimSpace(rawName)) {
	case "create", "architect & code", "architect":
		return "Architect & Code"
	case "test", "verify & test", "verify":
		return "Verify & Test"
	case "review", "critic review", "critic":
		return "Critic Review"
	case "human_review", "human review":
		return "Human Review"
	}
	labels := []string{"Architect & Code", "Verify & Test", "Critic Review", "Human Review"}
	if idx >= 0 && idx < len(labels) {
		return labels[idx]
	}
	return firstNonEmpty(rawName, "Architect & Code")
}

func firstChangedPath(change *ChangeSession) string {
	if len(change.Changes) > 0 {
		return change.Changes[0].Path
	}
	if len(change.Targets) > 0 {
		return change.Targets[0].Path
	}
	return ""
}

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
func EvaluateBootstrapChecklist(index *ProjectIndex, instructions *InstructionPreview) []BootstrapPillar {
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

// DefaultModelPresets returns the built-in 3-scope model configuration presets.
func DefaultModelPresets() []ModelPreset {
	return []ModelPreset{
		{ID: "local-default", Name: "Local Studio Default", Analyze: "local", Bug: "local", Function: "local"},
		{ID: "hybrid-precision", Name: "Hybrid Precision", Analyze: "local", Bug: "remote-pro", Function: "local"},
		{ID: "deep-audit", Name: "Deep Security & Bug Audit", Analyze: "remote-pro", Bug: "remote-pro", Function: "remote-fast"},
	}
}

// ResolveProjectDocs returns the project's structured handbook or synthesizes a fallback from Overview.
func ResolveProjectDocs(overview *Overview, project *Project) ProjectDocsReport {
	if overview != nil && overview.Docs != nil {
		return *overview.Docs
	}
	purpose := "Local repository workspace managed by Mini-Orca."
	topology := "Layered application repository"
	var comps []ProjectDocsComponent
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
			comps = append(comps, ProjectDocsComponent{Name: c, Layer: "Core", Summary: c})
		}
	}
	return ProjectDocsReport{
		Status: "synthesized",
		Context: ProjectDocsContext{
			Purpose:  purpose,
			Topology: topology,
		},
		Components: comps,
	}
}

// RenderArchitectureASCII formats an ASCII box-and-arrow architecture diagram for terminal display.
func RenderArchitectureASCII(docs ProjectDocsReport) []string {
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

var testNameSuffixes = []string{
	"_test.go", "_test.py", "test.java", "tests.java",
	"test.kt", "test.rs", "test.rb", "test.php", "test.cs",
}

// IsTestFilePath reports whether a project-relative path matches the daemon's Stage 2 test-path rules.
func IsTestFilePath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	base := strings.ToLower(filepath.Base(clean))
	if strings.HasPrefix(base, "test_") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	for _, suf := range testNameSuffixes {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	return hasTestDirSegment(clean)
}

func hasTestDirSegment(clean string) bool {
	for _, seg := range strings.Split(strings.ToLower(clean), "/") {
		switch seg {
		case "test", "tests", "__tests__", "spec", "specs":
			return true
		}
	}
	return false
}

// DeriveCompanionTestPath returns a canonical companion test file path for a source path.
func DeriveCompanionTestPath(path string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" || IsTestFilePath(clean) {
		return clean
	}
	dir := filepath.ToSlash(filepath.Dir(clean))
	base := filepath.Base(clean)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = base
	}
	testBase := companionTestBase(stem, ext)
	if dir == "." || dir == "" {
		return testBase
	}
	return dir + "/" + testBase
}

func companionTestBase(stem, ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return stem + "_test.go"
	case ".py":
		return "test_" + stem + ".py"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return stem + ".test" + ext
	default:
		return stem + "_test.go"
	}
}

// EnsureChangeTestPaths deduplicates captured paths, caps scope at 8 files, and guarantees at least one test path.
func EnsureChangeTestPaths(paths []string, index *ProjectIndex) []string {
	var clean []string
	for _, raw := range paths {
		p := filepath.ToSlash(strings.TrimSpace(raw))
		if p != "" && !containsString(clean, p) && len(clean) < 8 {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 && index != nil && len(index.Files) > 0 {
		clean = append(clean, index.Files[0].Path)
	}
	if len(clean) == 0 || hasAnyTestPath(clean) {
		return clean
	}
	testPath := DeriveCompanionTestPath(clean[0])
	if testPath == "" {
		return clean
	}
	if len(clean) >= 8 {
		clean[7] = testPath
		return clean
	}
	return append(clean, testPath)
}

func hasAnyTestPath(paths []string) bool {
	for _, p := range paths {
		if IsTestFilePath(p) {
			return true
		}
	}
	return false
}

// NormalizeFindingTargets validates and canonicalizes finding targets for OpenChangeSession.
func NormalizeFindingTargets(targets []FindingTarget, projectID string, allowedPaths []string) []FindingTarget {
	var out []FindingTarget
	for _, t := range targets {
		norm, ok := normalizeSingleTarget(t, projectID, allowedPaths)
		if ok {
			out = append(out, norm)
		}
	}
	return out
}

func normalizeSingleTarget(t FindingTarget, projectID string, allowedPaths []string) (FindingTarget, bool) {
	t.ID = strings.TrimSpace(t.ID)
	t.Path = filepath.ToSlash(strings.TrimSpace(t.Path))
	if t.ID == "" || t.Path == "" {
		return FindingTarget{}, false
	}
	if len(allowedPaths) > 0 && !containsString(allowedPaths, t.Path) {
		return FindingTarget{}, false
	}
	if t.ProjectID == "" {
		t.ProjectID = projectID
	}
	if t.ProjectID == "" {
		return FindingTarget{}, false
	}
	switch t.Kind {
	case "security":
		if t.Source != "ai" && t.Source != "deterministic" {
			t.Source = "deterministic"
		}
	case "performance":
		t.Source = ""
	default:
		t.Kind = "semantic"
		t.Source = ""
	}
	return t, true
}
