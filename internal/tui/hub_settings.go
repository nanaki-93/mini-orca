package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) renderAnalysisRunPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[2] AGENT STUDIO · LAST ANALYSIS RUN & CONFIG", "Section coverage, execution telemetry & run parameters")
	selected := s.ClampedIdx()
	if s.Run == nil {
		lines = append(lines, fmt.Sprintf("  %sNo analysis run recorded yet — press [r] or run :analyze to start.%s", th.muted, th.reset), "")
		return append(lines, s.renderAnalysisConfigSection(th, width, panelW, selected)...)
	}
	done, total := runAggregateCoverage(s.Run)
	telemetry := []string{
		fmt.Sprintf(
			"%sRun ID:%s %s%s%s · %sStatus:%s %s · %sElapsed:%s %ds · %sFiles:%s %d · %sCoverage:%s %s",
			th.muted, th.reset, th.bold+th.fg, s.Run.Identity.ID, th.reset,
			th.muted, th.reset, th.statusBadge(s.Run.Status),
			th.muted, th.reset, s.Run.ElapsedSeconds,
			th.muted, th.reset, s.Run.WindowFilesCompleted,
			th.muted, th.reset, th.renderProgressBar(done, total, 12),
		),
	}
	if strings.TrimSpace(s.Run.Reason) != "" {
		telemetry = append(telemetry, fmt.Sprintf("%sReason:%s %s%s%s",
			th.muted, th.reset, th.amber, s.Run.Reason, th.reset))
	}
	if s.isAnalysisPausedOrInterrupted() {
		telemetry = append(telemetry, fmt.Sprintf("%sAction:%s %sPress [r] or [Enter] to Resume remaining work%s · %s[n]%s Full Reanalyze",
			th.bold+th.cyan, th.reset, th.bold+th.green, th.reset, th.cyan, th.reset))
	} else if s.Run.Status == "failed" {
		telemetry = append(telemetry, fmt.Sprintf("%sAction:%s %sPress [R] to Retry failed stages%s · %s[n]%s Full Reanalyze",
			th.bold+th.cyan, th.reset, th.bold+th.amber, th.reset, th.cyan, th.reset))
	}
	lines = append(lines, th.renderPanelBox("ANALYSIS TELEMETRY", strings.ToUpper(s.Run.Status), panelW, telemetry)...)
	lines = append(lines, "")
	lines = append(lines, s.renderAnalysisSectionsTable(th, width, selected)...)
	lines = append(lines, "")
	return append(lines, s.renderAnalysisConfigSection(th, width, panelW, selected)...)
}

func (s *State) renderAnalysisSectionsTable(th ansiTheme, width, selected int) []string {
	lines := th.renderTableHeader("    SECTION         STATUS        PROGRESS       SUCCEEDED       FAILED", width)
	for idx, sec := range s.Run.Sections {
		cursor := th.renderCursorPrefix(idx == selected)
		bar := th.renderProgressBar(sec.Coverage.Succeeded, sec.Coverage.Total, 10)
		failStr := fmt.Sprintf("%sfailed:%d%s", th.muted, sec.Coverage.Failed, th.reset)
		if sec.Coverage.Failed > 0 {
			failStr = th.critBg + fmt.Sprintf(" failed:%d ", sec.Coverage.Failed) + th.reset
		}
		tStyle := rowTitleStyle(th, idx == selected)
		succCol := theme.PadVisibleRight(fmt.Sprintf("%ssucceeded:%d/%d%s", th.green, sec.Coverage.Succeeded, sec.Coverage.Total, th.reset), 14)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-14s%s  %s  %s   %s  %s",
			cursor, tStyle, strings.ToUpper(sec.Category), th.reset,
			th.padBadge(sec.Status, 12), bar,
			succCol, failStr,
		))
		if strings.TrimSpace(sec.Reason) != "" {
			lines = append(lines, fmt.Sprintf("      %s↳ %s%s", th.amber, sec.Reason, th.reset))
		}
		for _, diag := range collectSectionStageFailures(s.Run.Files, sec.Category) {
			lines = append(lines, fmt.Sprintf("      %s↳ %s%s", th.red, theme.TruncatePlain(diag, max(36, width-10)), th.reset))
		}
	}
	if s.Run.Features != nil {
		featIdx := len(s.Run.Sections)
		cursor := th.renderCursorPrefix(featIdx == selected)
		tStyle := rowTitleStyle(th, featIdx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-14s%s  %s  %sattempts:%d%s",
			cursor, tStyle, "FEATURES", th.reset,
			th.padBadge(s.Run.Features.Status, 12),
			th.muted, s.Run.Features.Attempts, th.reset,
		))
		if strings.TrimSpace(s.Run.Features.Reason) != "" {
			lines = append(lines, fmt.Sprintf("      %s↳ %s%s", th.amber, theme.TruncatePlain(s.Run.Features.Reason, max(36, width-10)), th.reset))
		}
	}
	return lines
}

func stageMatchesSection(stageName, category string) bool {
	s := strings.ToLower(strings.TrimSpace(stageName))
	c := strings.ToLower(strings.TrimSpace(category))
	return s == c || strings.HasPrefix(s, c+"_")
}

func collectSectionStageFailures(files []RunFile, category string) []string {
	var out []string
	for _, rf := range files {
		for _, st := range rf.Stages {
			if !stageMatchesSection(st.Stage, category) {
				continue
			}
			isComplete := st.Status == "completed" || st.Status == "completed_empty"
			if st.Status == "failed" || (!isComplete && strings.TrimSpace(st.Reason) != "") {
				reason := client.FirstNonEmpty(strings.TrimSpace(st.Reason), "stage execution failed")
				out = append(out, fmt.Sprintf("%s [%s] — %s (attempts:%d)", rf.Path, st.Stage, reason, st.Attempts))
				if len(out) >= 4 {
					return out
				}
			}
		}
	}
	return out
}

func (s *State) renderAnalysisConfigSection(th ansiTheme, width, panelW, selected int) []string {
	cfg := s.EffectiveAnalysisConfig()
	rows := domain.BuildAnalysisConfigRows(cfg)
	secCount := s.analysisRunSectionCount()
	lines := th.renderTableHeader("    KEY     ANALYSIS RUN PARAMETER    ACTIVE VALUE                      DESCRIPTION", width)
	for cIdx, row := range rows {
		rowIdx := secCount + cIdx
		cursor := th.renderCursorPrefix(rowIdx == selected)
		tStyle := rowTitleStyle(th, rowIdx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-7s%s %s%-24s%s  %s%-32s%s  %s%s%s",
			cursor, th.cyan, row.Key, th.reset,
			tStyle, row.Label, th.reset,
			th.bold+th.green, row.Value, th.reset,
			th.muted, row.Description, th.reset,
		))
	}
	codeMod := s.EffectiveScopeModel("bug")
	analyzeMod := s.EffectiveScopeModel("analyze")
	planLines := []string{
		fmt.Sprintf("%sDispatch Limits:%s %s%d files/batch%s · %sBudget:%s %s%ds%s · %sMax Attempts:%s %s%d/stage%s · %sFeatures:%s %s",
			th.muted, th.reset, th.bold+th.fg, cfg.BatchFiles, th.reset,
			th.muted, th.reset, th.bold+th.fg, cfg.BudgetSeconds, th.reset,
			th.muted, th.reset, th.bold+th.fg, cfg.MaxAttemptsPerStage, th.reset,
			th.muted, th.reset, th.statusBadge(rows[3].Badge)),
		fmt.Sprintf("%sStage Models:%s   %scode:%s%s · %sperf/sec/features:%s%s · %sCache:%s %s  %s([m] Model Routing · [x] Reset)%s",
			th.muted, th.reset,
			th.cyan, client.FirstNonEmpty(codeMod.Model, "bug"), th.reset,
			th.cyan, client.FirstNonEmpty(analyzeMod.Model, "analyze"), th.reset,
			th.muted, th.reset, th.statusBadge(rows[4].Badge),
			th.muted, th.reset),
	}
	lines = append(lines, "")
	return append(lines, th.renderPanelBox("ANALYSIS RUN CONFIGURATION & EXECUTION PLAN", "DISPATCH POLICY", panelW, planLines)...)
}

func (s *State) renderAnalysisFilesPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "[2] AGENT STUDIO · FILE SCOPE", "Captured path inclusion & per-file analysis stage diagnostics")
	if s.Index == nil || len(s.Index.Files) == 0 {
		return append(lines, fmt.Sprintf("  %sNo indexed files available.%s", th.muted, th.reset))
	}
	total := len(s.Index.Files)
	excl := len(s.ExcludedPaths)
	incl := max(0, total-excl)
	lines = append(lines, fmt.Sprintf("  %sScope Summary:%s %s%d Included%s · %s%d Excluded%s · %d Total Indexed Files",
		th.muted, th.reset, th.bold+th.green, incl, th.reset, th.amber, excl, th.reset, total), "")
	selected := s.ClampedIdx()
	pathCol := max(24, width-48)
	headerCols := fmt.Sprintf("    %-8s  %-*s  %s", "SCOPE", pathCol, "REPOSITORY PATH", "LANGUAGE / LINES")
	lines = append(lines, th.renderTableHeader(headerCols, width)...)
	winRows := s.adaptiveWindowRows(10, 14)
	if s.Run != nil && len(s.Run.Files) > 0 {
		winRows = s.adaptiveWindowRows(6, 10)
	}
	start, end := windowBounds(total, selected, winRows)
	for idx := start; idx < end; idx++ {
		f := s.Index.Files[idx]
		cursor := th.renderCursorPrefix(idx == selected)
		scopeState := "INCL"
		if domain.ContainsString(s.ExcludedPaths, f.Path) {
			scopeState = "EXCL"
		}
		runTag := s.fileRunStatusBadge(th, f.Path)
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s  %s%-*s%s  %s%s · %d lines%s%s",
			cursor, th.padBadge(scopeState, 8), tStyle, pathCol, theme.TruncatePlain(f.Path, pathCol), th.reset,
			th.muted, client.FirstNonEmpty(f.Language, "text"), f.LineCount, th.reset, runTag,
		))
	}
	lines = appendWindowFooter(th, lines, start, end, total)
	if selected >= 0 && selected < len(s.Index.Files) {
		lines = append(lines, s.renderSelectedFileStageInspector(th, adaptivePanelWidth(width), s.Index.Files[selected].Path)...)
	}
	return lines
}

func (s *State) findRunFile(path string) *RunFile {
	if s.Run == nil {
		return nil
	}
	for i := range s.Run.Files {
		if s.Run.Files[i].Path == path {
			return &s.Run.Files[i]
		}
	}
	return nil
}

func (s *State) fileRunStatusBadge(th ansiTheme, path string) string {
	rf := s.findRunFile(path)
	if rf == nil || len(rf.Stages) == 0 {
		return ""
	}
	status := "COMPLETED"
	for _, st := range rf.Stages {
		if st.Status == "failed" {
			status = "FAILED"
			break
		}
		if st.Status != "completed" && st.Status != "completed_empty" {
			status = st.Status
		}
	}
	return "  " + th.statusBadge(status)
}

func (s *State) renderSelectedFileStageInspector(th ansiTheme, panelW int, path string) []string {
	rf := s.findRunFile(path)
	if rf == nil || len(rf.Stages) == 0 {
		return nil
	}
	var body []string
	for _, st := range rf.Stages {
		findingsText := "0 issues (clean)"
		if st.Status == "failed" {
			findingsText = "stage failed"
		} else if st.FindingCount != nil && *st.FindingCount > 0 {
			findingsText = fmt.Sprintf("%d findings", *st.FindingCount)
		}
		row := fmt.Sprintf("%-22s  %s  %s%-18s%s  %sattempts:%d%s",
			strings.ToUpper(st.Stage), th.padBadge(st.Status, 18),
			th.fg, findingsText, th.reset,
			th.muted, st.Attempts, th.reset)
		body = append(body, row)
		if strings.TrimSpace(st.Reason) != "" {
			body = append(body, fmt.Sprintf("  %s↳ Error: %s%s", th.red, strings.TrimSpace(st.Reason), th.reset))
		}
	}
	return append([]string{""}, th.renderPanelBox("SELECTED FILE ANALYSIS STAGES", path, panelW, body)...)
}

func (s *State) renderGovernanceSubPage(th ansiTheme, width int) []string {
	section := s.RulesSection
	switch s.Page {
	case PageModels:
		section = "models"
	case PageBootstrap:
		section = "bootstrap"
	case PageLessons:
		section = "lessons"
	}
	switch section {
	case "models":
		return s.renderModelsPage(th, width)
	case "bootstrap":
		return s.renderBootstrapPage(th, width)
	case "lessons":
		return s.renderLessonsPage(th, width)
	default:
		return s.renderTrustPage(th, width)
	}
}

func (s *State) renderTrustPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	trustBadge := th.statusBadge("TRUSTED")
	if !s.WorkspaceTrust {
		trustBadge = th.statusBadge("UNTRUSTED")
	}
	lines := renderPageHeader(th, "PROJECT TRUST", "[Enter / :trust] Grant or revoke")
	body := []string{
		fmt.Sprintf("%sExecution:%s %s", th.bold+th.fg, th.reset, trustBadge),
		"Trust is remembered for this project path until you revoke it.",
		"Checks validate the current revision and run in copied workspaces.",
	}
	return append(lines, th.renderPanelBox("LOCAL EXECUTION", "", panelW, body)...)
}

func (s *State) renderModelsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "MODELS", "[↑/↓] Select · [←/→] Change model · [Enter] Apply preset")
	lines = append(lines, th.renderTableHeader("   SCOPE         MODEL                        ACCESS", width)...)
	selected := s.ClampedIdx()
	scopes := domain.CanonicalModelScopes()
	for idx, scope := range scopes {
		cursor := th.renderCursorPrefix(idx == selected)
		eff := s.EffectiveScopeModel(scope)
		assigned := s.AssignedScopeModelID(scope)
		modelLabel := client.FirstNonEmpty(eff.Model, assigned)
		if assigned != scope && assigned != eff.Model {
			modelLabel = fmt.Sprintf("%s (%s)", assigned, eff.Model)
		}
		boundary := "LOCAL"
		if eff.RemoteProvider {
			boundary = "REMOTE"
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-12s%s  %-26s  %s",
			cursor, tStyle, strings.ToUpper(scope), th.reset,
			theme.TruncatePlain(modelLabel, 26), th.padBadge(boundary, 9),
		))
	}
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("    PRESET                        STATE", width)...)
	for pIdx, p := range s.AllModelPresets() {
		rowIdx := len(scopes) + pIdx
		cursor := th.renderCursorPrefix(rowIdx == selected)
		active := "         "
		if p.ID == s.ActivePresetID {
			active = th.green + "(ACTIVE) " + th.reset
		} else if !domain.IsBuiltinModelPreset(p.ID) {
			active = th.cyan + "(CUSTOM) " + th.reset
		}
		tStyle := rowTitleStyle(th, rowIdx == selected)
		lines = append(lines, fmt.Sprintf(
			"  %s%s%-28s%s %s",
			cursor, tStyle, theme.TruncatePlain(p.Name, 28), th.reset, active,
		))
	}
	lines = append(lines, "")
	lines = append(lines, s.renderSelectedModelInspector(th, panelW)...)
	return lines
}

func (s *State) renderSelectedModelInspector(th ansiTheme, panelW int) []string {
	if idx := s.ClampedIdx() - len(domain.CanonicalModelScopes()); idx >= 0 {
		presets := s.AllModelPresets()
		if idx < len(presets) {
			return renderModelPresetInspector(th, panelW, presets[idx])
		}
	}
	selScope := s.selectedModelScope()
	eff := s.EffectiveScopeModel(selScope)
	assigned := s.AssignedScopeModelID(selScope)
	boundary := "LOCAL"
	if eff.RemoteProvider {
		boundary = "REMOTE"
	}
	choices := s.AvailableModelIDs()
	piStatus, agyStatus := "idle", "idle"
	if s.AvailableModels != nil {
		if s.AvailableModels.Pi.Status != "" {
			piStatus = s.AvailableModels.Pi.Status
		}
		if s.AvailableModels.Agy.Status != "" {
			agyStatus = s.AvailableModels.Agy.Status
		}
	}
	contentW := issuePanelContentWidth(panelW)
	body := renderDiagnosticRow(th, th.cyan, "Role:", th.fg, domain.ScopeRoleDescription(selScope), contentW)
	body = append(body, renderDiagnosticRow(th, th.cyan, "Assignment:", th.fg, assigned, contentW)...)
	body = append(body, renderDiagnosticRow(th, th.cyan, "Model:", th.fg, client.FirstNonEmpty(eff.Model, assigned), contentW)...)
	body = append(body, renderDiagnosticRow(th, th.cyan, "Provider:", th.fg, client.FirstNonEmpty(eff.ProviderOrigin, "unavailable"), contentW)...)
	body = append(body, renderDiagnosticRow(th, th.cyan, "Parameters:", th.fg, domain.FormatModelParams(eff), contentW)...)
	body = append(body, renderDiagnosticRow(th, th.cyan, "Discovery:", th.fg, fmt.Sprintf("Pi(%s) Agy(%s) · %d choices", piStatus, agyStatus, len(choices)), contentW)...)
	return th.renderPanelBox("MODEL DETAILS", strings.ToUpper(selScope)+" · "+boundary, panelW, body)
}

func renderModelPresetInspector(th ansiTheme, panelW int, preset ModelPreset) []string {
	contentW := issuePanelContentWidth(panelW)
	body := renderDiagnosticRow(th, th.cyan, "Preset:", th.fg, preset.Name, contentW)
	for _, role := range []struct{ label, model string }{{"Analyze:", preset.Analyze}, {"Bug:", preset.Bug}, {"Code:", preset.Function}} {
		body = append(body, renderDiagnosticRow(th, th.cyan, role.label, th.fg, role.model, contentW)...)
	}
	return th.renderPanelBox("PRESET DETAILS", "[Enter] Apply", panelW, body)
}

func (s *State) renderBootstrapPage(th ansiTheme, width int) []string {
	pillars := domain.EvaluateBootstrapChecklist(s.Index, s.Instructions)
	readyCount := countReadyPillars(pillars)
	lines := renderPageHeader(th, "[5] GOVERNANCE & RULES · BOOTSTRAP CHECKLIST", "Repository readiness & AGENTS.md pillars")
	lines = append(lines, fmt.Sprintf("  %sGovernance Readiness:%s %s %s(%d/%d pillars active)%s",
		th.muted, th.reset, th.renderProgressBar(readyCount, len(pillars), 14), th.bold+th.fg, readyCount, len(pillars), th.reset), "")
	lines = append(lines, th.renderTableHeader("    STATUS      GOVERNANCE PILLAR             DESCRIPTION", width)...)
	selected := s.ClampedIdx()
	for idx, pil := range pillars {
		cursor := th.renderCursorPrefix(idx == selected)
		status := "MISSING"
		if pil.Ready {
			status = "READY"
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%s %s%-28s%s %s— %s%s",
			cursor, th.padBadge(status, 10), tStyle, pil.Title, th.reset, th.muted, pil.Description, th.reset))
	}
	return lines
}

func (s *State) renderLessonsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	lines := renderPageHeader(th, "[5] GOVERNANCE & RULES · HARNESS² LESSONS", "Graduated engineering guardrails & AGENTS.md rules")
	lines = append(lines, th.renderTableHeader("    STATUS        HARNESS² GUARDRAIL LESSON", width)...)
	lessons := s.AllHarnessLessons()
	selected := s.ClampedIdx()
	for idx, rule := range lessons {
		cursor := th.renderCursorPrefix(idx == selected)
		status := "CANDIDATE"
		if s.IsLessonGraduated(rule) {
			status = "GRADUATED"
		}
		tStyle := rowTitleStyle(th, idx == selected)
		lines = append(lines, fmt.Sprintf("  %s%s %s%s%s", cursor, th.padBadge(status, 12), tStyle, rule, th.reset))
	}
	if selected < len(lessons) {
		selRule := lessons[selected]
		statusText := "Candidate guardrail — press [Enter/g] to stage into AGENTS.md"
		if s.IsLessonGraduated(selRule) {
			statusText = "Active in AGENTS.md draft — press [d] to inspect diff or [a/c] to apply & commit"
		}
		propLine := fmt.Sprintf("%sTarget:%s %sAGENTS.md%s · %sStatus:%s %s",
			th.muted, th.reset, th.bold+th.cyan, th.reset, th.muted, th.reset, statusText)
		if s.Change != nil && (s.Change.Kind == "instructions" || (len(s.Change.Changes) > 0 && s.Change.Changes[0].Path == "AGENTS.md")) {
			propLine = fmt.Sprintf("%sProposal:%s %s%s%s (%s) · %s[d]%s Diff · %s[a/c]%s Apply & Commit · %s[u]%s Undo",
				th.muted, th.reset, th.bold+th.fg, s.Change.Title, th.reset, th.statusBadge(s.Change.State),
				th.cyan, th.reset, th.cyan, th.reset, th.cyan, th.reset)
		}
		box := []string{
			fmt.Sprintf("%sRule:%s     %s%s%s", th.muted, th.reset, th.bold+th.fg, selRule, th.reset),
			propLine,
		}
		lines = append(lines, "")
		lines = append(lines, th.renderPanelBox("AGENTS.MD GUARDRAIL PREVIEW", "HARNESS² GOVERNANCE", panelW, box)...)
	}
	return lines
}

func countReadyPillars(pillars []BootstrapPillar) int {
	ready := 0
	for _, p := range pillars {
		if p.Ready {
			ready++
		}
	}
	return ready
}
