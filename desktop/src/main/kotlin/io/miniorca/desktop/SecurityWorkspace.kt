package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
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
    val index: ProjectIndex?,
    val browser: ResultBrowserState = newResultBrowserState(page),
)

internal data class SecurityWorkspaceActions(
    val prepareFix: (SecurityFinding) -> Unit,
    val openAnalysis: () -> Unit,
    val semanticActions: FindingActions,
    val retryResults: (() -> Unit)? = null,
)

internal data class SecurityResult(
    val report: SecurityFileReport,
    val finding: SecurityFinding,
    val stale: Boolean,
    val page: AnalysisResultPageState,
) {
  // Keep the report identity in the row key so two producers (or two report versions) cannot
  // select one another. Duplicate IDs within the same report still collide and are rejected.
  val rowKey: String
    get() =
        "security:" +
            listOf(
                    report.source,
                    report.path,
                    report.contentHash,
                    report.ruleSetVersion,
                    report.model,
                    report.profile,
                    report.generatedAt,
                    finding.id)
                .joinToString(":") { "${it.length}:$it" }

  fun row() =
      ResultRowPresentation(
          rowKey,
          finding.title.ifBlank { "Untitled security finding" },
          "${finding.anchor.path}:${finding.anchor.startLine}",
          finding.observedCondition,
          finding.severity,
          "",
          listOfNotNull(
                  "Stale".takeIf { stale },
                  finding.triage.takeIf { it.isNotBlank() && it !in setOf("open", "untriaged") },
                  finding.verificationState.takeIf { it.isNotBlank() && it != "unverified" })
              .joinToString(" · "))
}

/** The UI only names evidence when the report and finding retain their contract pairing. */
internal enum class SecurityEvidencePresentation {
  ModelHypothesis,
  SourceRule,
  Unavailable,
}

internal fun securityEvidencePresentation(result: SecurityResult): SecurityEvidencePresentation =
    when {
      result.report.source == "ai" && result.finding.evidenceKind == "model_suspicion" ->
          SecurityEvidencePresentation.ModelHypothesis
      result.report.source == "deterministic" && result.finding.evidenceKind == "rule_match" ->
          SecurityEvidencePresentation.SourceRule
      else -> SecurityEvidencePresentation.Unavailable
    }

internal val SecurityEvidencePresentation.label: String
  get() =
      when (this) {
        SecurityEvidencePresentation.ModelHypothesis -> "Model hypothesis"
        SecurityEvidencePresentation.SourceRule -> "Source rule"
        SecurityEvidencePresentation.Unavailable -> "Evidence type unavailable"
      }

internal val SecurityEvidencePresentation.warning: String
  get() =
      when (this) {
        SecurityEvidencePresentation.ModelHypothesis ->
            "Unverified model hypothesis. Validate the preconditions and source evidence before remediation."
        SecurityEvidencePresentation.SourceRule ->
            "A source rule match identifies a pattern; it does not confirm a vulnerability."
        SecurityEvidencePresentation.Unavailable ->
            "Evidence type was unavailable. Do not treat this finding as verified."
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
          report.findings.map {
            SecurityResult(report, it, page.stale || report.status == "stale", page)
          }
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

// The saved result page, not an independently stored file scan, owns workspace findings.
// Report identity uses the same fields as Summary targets; equal finding values are not owners.
internal fun securityResultIsLoaded(
    result: SecurityResult,
    page: AnalysisResultPageState
): Boolean {
  if (result.page.results !== page.results ||
      result.page.project != page.project ||
      result.page.run?.identity != page.run?.identity ||
      result.report.projectId != page.project?.projectId ||
      result.report.projectRevision != page.run?.identity?.projectRevision)
      return false
  val owner = result.report
  val reports =
      page.results?.security.orEmpty().filter {
        it.source == owner.source &&
            it.path == owner.path &&
            it.contentHash == owner.contentHash &&
            it.ruleSetVersion == owner.ruleSetVersion &&
            it.model == owner.model &&
            it.profile == owner.profile &&
            it.generatedAt == owner.generatedAt
      }
  return reports.size == 1 &&
      reports.single() == owner &&
      owner.findings.count { it.id == result.finding.id } == 1 &&
      owner.findings.single { it.id == result.finding.id } == result.finding
}

internal fun securityFindingIsCurrent(finding: SecurityFinding, state: DesktopState): Boolean {
  val page = state.analysisResultPage("security")
  val matches = securityResults(page).filter { it.finding == finding }
  val loaded =
      matches.size == 1 &&
          securityResultIsLoaded(matches.single(), page) &&
          !matches.single().stale &&
          securityReportMatchesIndex(matches.single().report, state.index)
  return loaded && securityFindingNavigationTarget(finding, state.index) != null
}

// Explicit scans have a separate action entry point; this must never be a fallback for a
// workspace result whose saved details are missing.
internal fun securityExplicitScanFindingIsCurrent(
    finding: SecurityFinding,
    state: DesktopState
): Boolean =
    securityExplicitScanContains(finding, state) &&
        securityFindingNavigationTarget(finding, state.index) != null

private fun securityExplicitScanContains(finding: SecurityFinding, state: DesktopState): Boolean =
    state.security.sourceReport?.let { report ->
      report.source == "deterministic" &&
          report.projectId == state.project?.projectId &&
          report.findings.count { it.id == finding.id } == 1 &&
          report.findings.single { it.id == finding.id } == finding &&
          securityReportMatchesIndex(report, state.index)
    } == true

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
      browser = state.browser,
      openAnalysis = actions.openAnalysis,
      retryResults = actions.retryResults) { key ->
        val matches = results.filter { it.rowKey == key }
        val semanticMatches = semantic.filter { semanticResultRow(it).key == key }
        when {
          matches.size + semanticMatches.size > 1 ->
              Text(
                  "Multiple results share this finding identity; no action is available.",
                  color = Warning,
                  style = IdeTypography.compactBody)
          matches.size == 1 -> SecurityFindingDetails(matches.single(), state.index, actions)
          semanticMatches.size == 1 ->
              semanticMatches.single().let {
                FindingDetailsRegion(
                    it,
                    actions.semanticActions,
                    findingPreparationDecision(it, state.page.project, semantic, state.index))
              }
        }
      }
}

@Composable
private fun SecurityFindingDetails(
    result: SecurityResult,
    index: ProjectIndex?,
    actions: SecurityWorkspaceActions
) {
  val finding = result.finding
  val evidence = securityEvidencePresentation(result)
  val canPrepare =
      securityResultIsLoaded(result, result.page) &&
          !result.stale &&
          securityReportMatchesIndex(result.report, index) &&
          securityFindingCanPrepareFix(finding, index)
  var technical by remember(result.row().key) { mutableStateOf(false) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
    ResultDetailHeader(result.row())
    IdeLabelBadge(evidence.label, Information)
    Text(evidence.warning, color = Warning, style = IdeTypography.compactBody)
    ResultEvidenceSection("Observed condition", finding.observedCondition)
    ResultEvidenceSection("Remediation", finding.remediation)
    if (result.report.reason.isNotBlank())
        Text(result.report.reason, color = Warning, style = IdeTypography.compactBody)
    Row(horizontalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.compact)) {
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
    IdeDisclosureHeader("Evidence and safe verification", technical, { technical = !technical })
    if (technical) {
      Text(
          when (evidence) {
            SecurityEvidencePresentation.ModelHypothesis ->
                "Model-provided hypothesis; verify the preconditions and source evidence."
            SecurityEvidencePresentation.SourceRule ->
                "Matched source rule: ${finding.rule}. Verify the preconditions before remediation."
            SecurityEvidencePresentation.Unavailable ->
                "Evidence provenance is unavailable; verify the preconditions before remediation."
          },
          color = SecondaryText,
          style = IdeTypography.compactBody)
      ResultEvidenceSection(
          "Preconditions / unknowns", finding.preconditions.ifBlank { "Not provided" })
      ResultEvidenceSection(
          "Safe verification idea", finding.verificationIdea.ifBlank { "Not provided" })
      Text(
          "${result.report.source} · ${result.report.ruleSetVersion} · ${result.report.profile} · ${result.report.model}",
          color = SecondaryText,
          style = IdeTypography.compactBody)
    }
  }
}
