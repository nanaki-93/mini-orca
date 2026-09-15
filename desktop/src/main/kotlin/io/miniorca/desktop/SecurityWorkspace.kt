package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

internal data class SecurityWorkspacePaneState(
    val page: AnalysisResultPageState,
    val index: ProjectIndex?
)

internal data class SecurityWorkspaceActions(
    val prepareFix: (SecurityFinding) -> Unit,
    val openAnalysis: () -> Unit,
    val semanticActions: FindingActions,
    val openResults: (Workspace) -> Unit = {},
)

internal data class SecurityResult(
    val report: SecurityFileReport,
    val finding: SecurityFinding,
    val stale: Boolean
) {
  fun row() =
      ResultRowPresentation(
          "security:${report.source}:${report.path}:${finding.id}",
          finding.title.ifBlank { "Untitled security finding" },
          "${finding.anchor.path}:${finding.anchor.startLine}",
          finding.observedCondition,
          finding.severity,
          if (finding.evidenceKind == "model_suspicion") "" else "Rule · ${finding.rule}",
          listOfNotNull(
                  "Stale".takeIf { stale },
                  finding.triage.takeIf { it.isNotBlank() && it !in setOf("open", "untriaged") },
                  finding.verificationState.takeIf { it.isNotBlank() && it != "unverified" })
              .joinToString(" · "))
}

internal fun securityResults(page: AnalysisResultPageState): List<SecurityResult> =
    page.results
        ?.security
        .orEmpty()
        .filter {
          it.projectId == page.project?.projectId &&
              it.projectRevision == page.run?.identity?.projectRevision
        }
        .flatMap { report ->
          report.findings
              .filter { it.anchor.path == report.path }
              .map { SecurityResult(report, it, page.stale || report.status == "stale") }
        }
        .sortedWith(
            compareBy<SecurityResult> {
                  listOf("critical", "high", "medium", "low").indexOf(it.finding.severity).let {
                      order ->
                    if (order < 0) 4 else order
                  }
                }
                .thenBy { it.report.path }
                .thenBy { it.finding.anchor.startLine }
                .thenBy { it.finding.id })

internal fun securityReportMatchesIndex(report: SecurityFileReport, index: ProjectIndex?): Boolean =
    index != null &&
        report.projectId == index.projectId &&
        report.projectRevision == index.projectRevision &&
        report.status in setOf("completed", "completed_empty", "partial") &&
        index.files.any { it.path == report.path && it.contentHash == report.contentHash }

internal fun securityFindingIsCurrent(finding: SecurityFinding, state: DesktopState): Boolean {
  val page = state.analysisResultPage("security")
  val current =
      securityResults(page).any {
        it.finding == finding && !it.stale && securityReportMatchesIndex(it.report, state.index)
      }
  // Explicit source scans retain their own report identity, independently of a project run.
  val scanned =
      listOfNotNull(state.security.sourceReport, state.security.aiReport).any {
        it.projectId == state.project?.projectId &&
            securityReportMatchesIndex(it, state.index) &&
            finding in it.findings
      }
  return (current || scanned) && securityFindingNavigationTarget(finding, state.index) != null
}

internal fun securityFindingNavigationTarget(
    finding: SecurityFinding,
    index: ProjectIndex?,
): EditorNavigationTarget? {
  val path = finding.anchor.path
  val file = index?.files?.firstOrNull { it.path == path } ?: return null
  if (!securityAnchorIsValid(finding.anchor, file)) return null
  return EditorNavigationTarget(path, finding.anchor.symbol, finding.anchor.startLine)
}

internal fun securityAnchorIsValid(anchor: SecuritySourceAnchor, file: IndexedFile): Boolean =
    anchor.path == file.path &&
        anchor.startLine >= 1 &&
        anchor.endLine >= anchor.startLine &&
        anchor.endLine <= file.lineCount &&
        (anchor.symbol.isBlank() ||
            file.symbols.any { symbol ->
              symbol.name == anchor.symbol &&
                  anchor.startLine >= symbol.startLine &&
                  anchor.endLine <= symbol.endLine
            })

internal fun securityAnchorWithinDeclaration(
    anchor: SecuritySourceAnchor,
    file: IndexedFile,
    declaration: SymbolInfo,
): Boolean =
    securityAnchorIsValid(anchor, file) &&
        declaration.startLine >= 1 &&
        declaration.endLine >= declaration.startLine &&
        anchor.startLine >= declaration.startLine &&
        anchor.endLine <= declaration.endLine

internal fun securityFindingCanPrepareFix(finding: SecurityFinding, index: ProjectIndex?): Boolean {
  val file = index?.files?.firstOrNull { it.path == finding.anchor.path } ?: return false
  val declaration =
      file.symbols.singleOrNull {
        it.name == finding.anchor.symbol && it.atomicTarget && it.confidence == "exact"
      } ?: return false
  return file.language == "Go" && securityAnchorWithinDeclaration(finding.anchor, file, declaration)
}

@Composable
internal fun SecurityWorkspacePane(
    state: SecurityWorkspacePaneState,
    actions: SecurityWorkspaceActions
) {
  val results = securityResults(state.page)
  val semantic = state.page.semantic
  AnalysisResultsPane(
      page = state.page,
      rows = results.map { it.row() } + semantic.map(::semanticResultRow),
      openAnalysis = actions.openAnalysis,
      emptyMessage = "No security findings reported in the analyzed scope.",
      openResults = actions.openResults) { key ->
        val result = results.firstOrNull { it.row().key == key }
        if (result != null) SecurityFindingDetails(result, state.index, actions)
        else
            semantic
                .firstOrNull { semanticResultRow(it).key == key }
                ?.let { FindingDetailsRegion(it, actions.semanticActions) }
      }
}

@Composable
private fun SecurityFindingDetails(
    result: SecurityResult,
    index: ProjectIndex?,
    actions: SecurityWorkspaceActions
) {
  val finding = result.finding
  val canPrepare =
      !result.stale &&
          securityReportMatchesIndex(result.report, index) &&
          securityFindingCanPrepareFix(finding, index)
  var technical by remember(result.row().key) { mutableStateOf(false) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
    ResultRowContent(result.row().copy(summary = ""))
    Text("Observed condition", style = IdeTypography.resultLabel, color = PrimaryText)
    ModelResultContent(finding.observedCondition)
    Text("Remediation", style = IdeTypography.resultLabel, color = PrimaryText)
    ModelResultContent(finding.remediation)
    ResponsiveActionGroup(Modifier.fillMaxWidth()) {
      MiniOrcaButton(
          onClick = { actions.prepareFix(finding) },
          enabled = canPrepare,
          tone = ActionTone.Primary) {
            Text("Prepare fix")
          }
    }
    if (!canPrepare)
        Text(
            if (result.stale) "Analyze again to prepare a fix from current source."
            else "Fix preparation requires a matching indexed Go declaration.",
            color = SecondaryText,
            style = IdeTypography.compactBody)
    if (result.report.reason.isNotBlank())
        Text(result.report.reason, color = Warning, style = IdeTypography.compactBody)
    IdeDisclosureHeader("Evidence and safe verification", technical, { technical = !technical })
    if (technical) {
      Text(
          if (finding.evidenceKind == "model_suspicion")
              "Model hypothesis; verify the preconditions and source evidence."
          else "Matched source rule: ${finding.rule}. Verify the preconditions before remediation.",
          color = SecondaryText,
          style = IdeTypography.compactBody)
      Text("Preconditions / unknowns", style = IdeTypography.resultLabel, color = PrimaryText)
      ModelResultContent(finding.preconditions.ifBlank { "Not provided" })
      Text("Safe verification idea", style = IdeTypography.resultLabel, color = PrimaryText)
      ModelResultContent(finding.verificationIdea.ifBlank { "Not provided" })
      Text(
          "${result.report.source} · ${result.report.ruleSetVersion} · ${result.report.profile} · ${result.report.model}",
          color = SecondaryText,
          style = IdeTypography.compactBody)
    }
  }
}
