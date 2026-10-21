package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

var (
	whitespacePattern   = regexp.MustCompile(`\s+`)
	genericTitlePattern = regexp.MustCompile(`(?i)^(file|project) analysis suggestion$`)
	nonAlphaNumPattern  = regexp.MustCompile(`[^a-z0-9]+`)
	commitCodePattern   = regexp.MustCompile(`(?i)\b(ORC-[0-9A-F]{8})\b`)
	trailingPunct       = " \t\n\r.,;:!?…"
)

// GenerateCommitCode produces a deterministic 8-hex-character commit code (ORC-XXXXXXXX)
// linking a guarded change commit to its resolved issue targets.
func GenerateCommitCode(sessionID, hash string, targets []client.FindingTarget) string {
	var parts []string
	if id := strings.TrimSpace(sessionID); id != "" {
		parts = append(parts, id)
	}
	if h := strings.TrimSpace(hash); h != "" {
		parts = append(parts, h)
	}
	for _, t := range targets {
		parts = append(parts, t.Kind+":"+t.ID+":"+t.Path)
	}
	seed := strings.Join(parts, "|")
	if seed == "" {
		seed = "mini-orca-commit"
	}
	sum := sha256.Sum256([]byte(seed))
	return "ORC-" + strings.ToUpper(hex.EncodeToString(sum[:4]))
}

// ExtractCommitCode extracts an ORC-XXXXXXXX commit code from a commit message if present.
func ExtractCommitCode(message string) string {
	if m := commitCodePattern.FindStringSubmatch(message); len(m) == 2 {
		return strings.ToUpper(m[1])
	}
	return ""
}

// FormatCommitMessageWithCode prefixes a commit message with [ORC-XXXXXXXX] unless already tagged.
func FormatCommitMessageWithCode(message, code string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		trimmed = "Apply verified Mini-Orca proposal"
	}
	normCode := strings.ToUpper(strings.TrimSpace(code))
	if normCode == "" || ExtractCommitCode(trimmed) != "" {
		return trimmed
	}
	return fmt.Sprintf("[%s] %s", normCode, trimmed)
}

// ResolveCommitCode extracts or deterministically derives the ORC-XXXXXXXX code for a commit.
func ResolveCommitCode(sessionID, hash string, targets []client.FindingTarget, commit *client.ChangeCommit) string {
	if commit != nil {
		if code := ExtractCommitCode(commit.Message); code != "" {
			return code
		}
		if commit.OID != "" && sessionID == "" && hash == "" && len(targets) == 0 {
			return GenerateCommitCode(commit.OID, "", nil)
		}
	}
	if sessionID == "" && hash == "" && len(targets) == 0 {
		return ""
	}
	return GenerateCommitCode(sessionID, hash, targets)
}

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
	Signature          string
	Line               int
	EndLine            int
	Severity           string
	Confidence         string
	Category           string
	Rule               string
	Source             string
	Freshness          string
	Status             string
	CommitOID          string
	CommitCode         string
	AgentPath          string
	Cause              string
	Evidence           string
	Solution           string
	Target             client.FindingTarget
	AcceptanceCriteria []string
	NonGoals           []string
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
func ComputeIssueSummary(issues []IssueRow, features *client.FeatureReport) IssueSummary {
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
	FindingTargets []client.FindingTarget
}

// IsCompletedOrInactiveSession reports whether a change session has already finished or been canceled.
func IsCompletedOrInactiveSession(chg *client.ChangeSession) bool {
	if chg == nil {
		return true
	}
	switch chg.State {
	case "applied", "reverted", "undone", "canceled":
		return true
	}
	if chg.Workflow != nil && chg.Workflow.Status == "canceled" {
		return true
	}
	return false
}

// BuildWorktreeSpec generates a deterministic, collision-free branch name and .worktrees/<slug> path for a new issue fix.
func BuildWorktreeSpec(seed *client.ChangeSeed, existing []client.GitWorktree) (branch, relPath string) {
	rawTitle := "issue-fix"
	if seed != nil && strings.TrimSpace(seed.Title) != "" {
		rawTitle = seed.Title
	}
	slug := strings.Trim(nonAlphaNumPattern.ReplaceAllString(strings.ToLower(rawTitle), "-"), "-")
	if len(slug) > 24 {
		slug = strings.Trim(slug[:24], "-")
	}
	if slug == "" {
		slug = "issue-fix"
	}
	var targets []client.FindingTarget
	msg := rawTitle
	if seed != nil {
		targets = seed.FindingTargets
		if seed.Message != "" {
			msg = seed.Message
		}
	}
	code := strings.ToLower(strings.TrimPrefix(GenerateCommitCode(rawTitle, msg, targets), "ORC-"))
	if len(code) > 6 {
		code = code[:6]
	}
	baseName := fmt.Sprintf("fix-%s-%s", slug, code)
	candidateBranch := "orca/" + baseName
	candidateRel := filepath.ToSlash(filepath.Join(".worktrees", baseName))
	for suffix := 1; worktreeSpecCollides(existing, candidateBranch, candidateRel); suffix++ {
		name := fmt.Sprintf("%s-%d", baseName, suffix+1)
		candidateBranch = "orca/" + name
		candidateRel = filepath.ToSlash(filepath.Join(".worktrees", name))
	}
	return candidateBranch, candidateRel
}

func worktreeSpecCollides(existing []client.GitWorktree, branch, relPath string) bool {
	base := filepath.Base(relPath)
	for _, wt := range existing {
		if wt.Branch == branch || filepath.Base(wt.Path) == base {
			return true
		}
	}
	return false
}

// CollectAllAgentItems derives Agent Studio items from all tracked change sessions and Git worktrees.
func CollectAllAgentItems(change *client.ChangeSession, activeChanges []*client.ChangeSession, receipt *client.ChangeMutation, gitWS *client.GitWorkspace) []AgentItem {
	items, seenIDs, seenWorktrees := collectSessionAgentItems(change, activeChanges, receipt, gitWS)
	if shouldAppendPrimaryChange(change, len(items), seenIDs) {
		if change.WorktreePath != "" {
			seenWorktrees[change.WorktreePath] = true
		}
		items = append(items, buildAgentItemFromSession(change, receipt, gitWS))
	}
	return appendWorktreeAgentItems(items, gitWS, seenWorktrees)
}

func collectSessionAgentItems(change *client.ChangeSession, activeChanges []*client.ChangeSession, receipt *client.ChangeMutation, gitWS *client.GitWorkspace) ([]AgentItem, map[string]bool, map[string]bool) {
	var items []AgentItem
	seenIDs := make(map[string]bool)
	seenWorktrees := make(map[string]bool)
	for _, chg := range activeChanges {
		if chg == nil {
			continue
		}
		cur := resolveCurrentSession(chg, change)
		if cur.ID != "" {
			seenIDs[cur.ID] = true
		}
		if cur.WorktreePath != "" {
			seenWorktrees[cur.WorktreePath] = true
		}
		if !IsCompletedOrInactiveSession(cur) {
			items = append(items, buildAgentItemFromSession(cur, receipt, gitWS))
		}
	}
	return items, seenIDs, seenWorktrees
}

func resolveCurrentSession(candidate, primary *client.ChangeSession) *client.ChangeSession {
	if primary != nil && primary.ID != "" && candidate.ID == primary.ID {
		return primary
	}
	return candidate
}

func shouldAppendPrimaryChange(change *client.ChangeSession, count int, seenIDs map[string]bool) bool {
	if change == nil || IsCompletedOrInactiveSession(change) {
		return false
	}
	return count == 0 || (change.ID != "" && !seenIDs[change.ID])
}

func appendWorktreeAgentItems(items []AgentItem, gitWS *client.GitWorkspace, seenWorktrees map[string]bool) []AgentItem {
	if gitWS == nil {
		return items
	}
	for _, wt := range gitWS.Worktrees {
		if wt.Current || seenWorktrees[wt.Path] || hasAgentPath(items, wt.Path) {
			continue
		}
		items = append(items, buildAgentItemFromWorktree(wt))
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

func buildAgentItemFromSession(change *client.ChangeSession, receipt *client.ChangeMutation, gitWS *client.GitWorkspace) AgentItem {
	added, removed, badge := ClassifyDiffScope(change.Changes)
	passed, total := CountPassedChecks(change.Checks)
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

func effectiveAgentSessionStatus(change *client.ChangeSession) string {
	if change == nil {
		return "idle"
	}
	if change.State == "applied" || change.State == "reverted" || change.State == "failed" || change.State == "canceled" {
		return change.State
	}
	if change.Freshness == "stale" {
		return "stale"
	}
	if change.Workflow != nil && change.Workflow.Status != "" {
		switch change.Workflow.Status {
		case "awaiting_human_review":
			return "ready"
		case "running", "queued", "canceling", "failed", "canceled", "changes_requested", "stale":
			return change.Workflow.Status
		}
	}
	if change.State != "" {
		return change.State
	}
	return "working"
}

func buildAgentItemFromWorktree(wt client.GitWorktree) AgentItem {
	title := fmt.Sprintf("Worktree %s", wt.Branch)
	var targets []client.FindingTarget
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

// CountPassedChecks returns the number of passed (or non-required skipped) checks and total checks.
func CountPassedChecks(checks []client.Check) (int, int) {
	passed := 0
	for _, c := range checks {
		if c.State == "passed" || (!c.Required && c.State == "skipped") {
			passed++
		}
	}
	return passed, len(checks)
}

func classifyAgentStage(change *client.ChangeSession, passed, total int) (int, string) {
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

func activeWorkflowStage(stages []client.ChangeWorkflowStage) (int, string) {
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
	return client.FirstNonEmpty(rawName, "Architect & Code")
}

func firstChangedPath(change *client.ChangeSession) string {
	if len(change.Changes) > 0 {
		return change.Changes[0].Path
	}
	if len(change.Targets) > 0 {
		return change.Targets[0].Path
	}
	return ""
}

// NormalizeFindingTargets validates and canonicalizes finding targets for OpenChangeSession.
func NormalizeFindingTargets(targets []client.FindingTarget, projectID string, allowedPaths []string) []client.FindingTarget {
	var out []client.FindingTarget
	seen := make(map[client.FindingTarget]bool, len(targets))
	for _, t := range targets {
		norm, ok := normalizeSingleTarget(t, projectID, allowedPaths)
		if ok && !seen[norm] && len(out) < 16 {
			seen[norm] = true
			out = append(out, norm)
		}
	}
	return out
}

func normalizeSingleTarget(t client.FindingTarget, projectID string, allowedPaths []string) (client.FindingTarget, bool) {
	t.ID = strings.TrimSpace(t.ID)
	t.Path = filepath.ToSlash(strings.TrimSpace(t.Path))
	if t.ID == "" || t.Path == "" {
		return client.FindingTarget{}, false
	}
	if len(allowedPaths) > 0 && !ContainsString(allowedPaths, t.Path) {
		return client.FindingTarget{}, false
	}
	if t.ProjectID == "" {
		t.ProjectID = projectID
	}
	if t.ProjectID == "" {
		return client.FindingTarget{}, false
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

func ContainsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
