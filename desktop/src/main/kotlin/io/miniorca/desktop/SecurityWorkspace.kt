package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.selection.SelectionContainer
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
          buildString {
            append(finding.anchor.path.ifBlank { "Path not supplied" })
            if (finding.anchor.startLine > 0) append(":${finding.anchor.startLine}")
          },
          finding.observedCondition,
          finding.severity,
          securityEvidencePresentation(this).label,
          listOfNotNull(
                  securityEvidencePresentation(this).label,
                  report.status
                      .takeIf { it in setOf("failed", "partial", "unavailable") }
                      ?.replaceFirstChar(Char::uppercase),
                  "Stale".takeIf { stale },
                  finding.triage.takeIf { it.isNotBlank() && it !in setOf("open", "untriaged") },
                  finding.verificationState
                      .takeIf { it.isNotBlank() && it != "unverified" }
                      ?.let { "Supplied verification: $it" })
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

// The finding-only presenter entry point is retained until typed intents are wired. It cannot
// choose between equally valued findings; workspace preparation itself uses the captured result.
internal fun securityFindingIsCurrent(finding: SecurityFinding, state: DesktopState): Boolean {
  val page = state.analysisResultPage("security")
  val matches = securityResults(page).filter { it.finding == finding }
  return matches.size == 1 &&
      securityPreparationDecision(matches.single(), state.index, page) is
          SecurityPreparationDecision.Eligible
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

internal sealed interface SecurityPreparationDecision {
  data class Eligible(
      val projectId: String,
      val projectRevision: String,
      val path: String,
      val contentHash: String,
      val declaration: SymbolInfo,
      val anchor: SecuritySourceAnchor,
  ) : SecurityPreparationDecision

  data class Blocked(val reason: String) : SecurityPreparationDecision
}

/** Indexed evidence is a preflight, not authorization to publish a prepared request. */
internal fun securityPreparationDecision(
    result: SecurityResult,
    index: ProjectIndex?,
    page: AnalysisResultPageState = result.page,
): SecurityPreparationDecision {
  fun blocked(reason: String) = SecurityPreparationDecision.Blocked(reason)
  val project = page.project
  val run = page.run
  if (project == null ||
      run == null ||
      index == null ||
      project.projectId.isBlank() ||
      project.projectRevision.isBlank() ||
      run.identity.projectId != project.projectId ||
      run.identity.projectRevision != project.projectRevision ||
      index.projectId != project.projectId ||
      index.projectRevision != project.projectRevision)
      return blocked("Load the current project index and Security results before preparing a fix.")
  if (!securityResultIsLoaded(result, page))
      return blocked(
          "This finding is missing or ambiguous in the current Security results. Refresh the analysis.")
  if (result.stale || page.stale)
      return blocked("Analyze again to prepare a fix from current source.")
  if (result.report.status !in setOf("completed", "partial"))
      return blocked("Wait for a completed or partial Security report before preparing a fix.")
  val anchor = result.finding.anchor
  if (result.report.path.isBlank() || anchor.path != result.report.path)
      return blocked("The finding must identify the report's indexed target file path.")
  val files = index.files.filter { it.path == anchor.path }
  if (files.isEmpty()) return blocked("The target file is missing from the project index.")
  if (files.size != 1) return blocked("The target file path is duplicated in the project index.")
  val indexed = files.single()
  if (result.report.contentHash.isBlank() || indexed.contentHash.isBlank())
      return blocked("The report or indexed file hash is missing. Reanalyze the file.")
  if (result.report.contentHash != indexed.contentHash)
      return blocked(
          "The report's file hash no longer matches the indexed source. Reanalyze the file.")
  if (anchor.startLine < 1 ||
      anchor.endLine < anchor.startLine ||
      anchor.endLine > indexed.lineCount)
      return blocked("The reported source range must be positive and within the indexed file.")
  if (anchor.symbol.isBlank()) return blocked("The finding must name one indexed declaration.")
  val declarations = indexed.symbols.filter { it.name == anchor.symbol }
  if (declarations.isEmpty())
      return blocked("The named declaration is missing from the indexed file.")
  if (declarations.size != 1)
      return blocked("The named declaration is ambiguous in the indexed file.")
  val declaration = declarations.single()
  if (declaration.startLine < 1 ||
      declaration.endLine < declaration.startLine ||
      declaration.endLine > indexed.lineCount ||
      !securityAnchorWithinDeclaration(anchor, indexed, declaration))
      return blocked("The reported source range must fall within one valid indexed declaration.")
  val eligibility =
      symbolEditEligibility(
          ProjectFileInfo(
              indexed.path,
              indexed.contentHash,
              indexed.path.substringAfterLast('/'),
              language = indexed.language,
              sizeBytes = indexed.sizeBytes,
              lineCount = indexed.lineCount,
              modifiedAt = indexed.modifiedAt,
              binary = indexed.binary),
          indexed.symbols,
          declaration)
  if (!eligibility.eligible) return blocked(eligibility.blockedReason)
  return SecurityPreparationDecision.Eligible(
      project.projectId,
      project.projectRevision,
      indexed.path,
      indexed.contentHash,
      declaration,
      anchor)
}

/** Revalidate both returned source and symbols against the captured indexed target. */
internal fun loadedSecurityPreparationDecision(
    target: SecurityPreparationDecision.Eligible,
    file: ProjectFileInfo,
    response: SymbolsResponse,
): SecurityPreparationDecision {
  if (file.path != target.path ||
      file.contentHash.isBlank() ||
      file.contentHash != target.contentHash)
      return SecurityPreparationDecision.Blocked(
          "Loaded source no longer matches the indexed file path and hash. Reanalyze the file.")
  if (response.projectId != target.projectId ||
      response.projectRevision != target.projectRevision ||
      response.path != target.path)
      return SecurityPreparationDecision.Blocked(
          "Loaded symbols belong to a different project, revision or file. Reanalyze the file.")
  val symbols = response.symbols
  val matches = symbols.filter { it.name == target.declaration.name }
  if (matches.size != 1 ||
      matches.single() != target.declaration ||
      target.anchor.path != file.path ||
      target.anchor.startLine < 1 ||
      target.anchor.endLine < target.anchor.startLine ||
      target.anchor.endLine > file.lineCount ||
      target.declaration.startLine < 1 ||
      target.declaration.endLine > file.lineCount ||
      target.anchor.startLine < target.declaration.startLine ||
      target.anchor.endLine > target.declaration.endLine)
      return SecurityPreparationDecision.Blocked(
          "Loaded source no longer contains the exact indexed declaration and anchor. Reanalyze the file.")
  val eligibility = symbolEditEligibility(file, symbols, matches.single())
  if (!eligibility.eligible) return SecurityPreparationDecision.Blocked(eligibility.blockedReason)
  return target
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
      retryResults = actions.retryResults,
      tools = { SecurityReportAvailability(state.page) }) { key ->
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

// Report failures must be visible even when the affected producer supplied no finding row.
@Composable
private fun SecurityReportAvailability(page: AnalysisResultPageState) {
  SelectionContainer {
    Text(
        "Security findings describe analyzed evidence, not proof of safety. No findings does not mean the project is secure; unavailable evidence and incomplete coverage remain unknown.",
        color = Warning,
        style = IdeTypography.compactBody)
  }
  page.results?.security.orEmpty().forEach { report ->
    if (report.status in setOf("failed", "partial", "unavailable") || report.reason.isNotBlank()) {
      SelectionContainer {
        Text(
            "${report.source.ifBlank { "Unknown source" }} · ${report.path.ifBlank { "Path not supplied" }} · ${analysisResultStatusLabel(report.status) ?: report.status.ifBlank { "Status unavailable" }}: ${report.reason.ifBlank { "No report reason supplied." }}",
            color = Warning,
            style = IdeTypography.compactBody)
      }
    }
  }
  if (page.progress?.status in setOf("failed", "partial", "unavailable") &&
      page.run?.reason?.isNotBlank() == true)
      SelectionContainer {
        Text("Analysis · ${page.run.reason}", color = Warning, style = IdeTypography.compactBody)
      }
}

@Composable
private fun SecurityDetailField(label: String, value: String) {
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
    Text(label, color = SecondaryText, style = IdeTypography.workspaceMetadata)
    ModelResultContent(
        value.ifBlank { "Not supplied." }, preview = false, style = IdeTypography.workspaceBody)
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
  val preparation = securityPreparationDecision(result, index)
  var technical by remember(result.row().key) { mutableStateOf(false) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
    ResultDetailHeader(result.row())
    IdeLabelBadge(evidence.label, Information)
    SelectionContainer {
      Text(evidence.warning, color = Warning, style = IdeTypography.compactBody)
    }
    SecurityDetailField("Report status", result.report.status)
    SecurityDetailField(
        "Freshness", if (result.stale) "Stale · saved evidence" else "Current for this analysis")
    SecurityDetailField(
        "Reported range",
        listOfNotNull(
                finding.anchor.startLine.takeIf { it > 0 }?.toString(),
                finding.anchor.endLine.takeIf { it > 0 }?.toString())
            .joinToString("–"))
    SecurityDetailField("Symbol", finding.anchor.symbol)
    SecurityDetailField("Observed condition", finding.observedCondition)
    SecurityDetailField("Remediation", finding.remediation)
    SecurityDetailField("Preconditions / unknowns", finding.preconditions)
    SecurityDetailField("Safe verification idea · not performed", finding.verificationIdea)
    SecurityDetailField("Rule", finding.rule)
    SecurityDetailField("Category", finding.category)
    SecurityDetailField("Supplied confidence · not verification", finding.confidence)
    SecurityDetailField("CWE", finding.cwe)
    SecurityDetailField("Reference · plain text, not a link", finding.reference)
    SecurityDetailField("Triage state", finding.triage)
    SecurityDetailField(
        "Supplied verification state · not independently verified", finding.verificationState)
    if (finding.engineeringInsight == null ||
        engineeringInsightPieces(finding.engineeringInsight).isEmpty())
        SecurityDetailField("Engineering insight", "")
    else {
      Text("Engineering insight", color = SecondaryText, style = IdeTypography.workspaceMetadata)
      engineeringInsightPieces(finding.engineeringInsight).forEach { piece ->
        SecurityDetailField(piece.label, piece.content)
      }
    }
    if (result.report.reason.isNotBlank())
        SelectionContainer {
          Text(result.report.reason, color = Warning, style = IdeTypography.compactBody)
        }
    MiniOrcaButton(
        onClick = { actions.prepareFix(finding) },
        enabled = preparation is SecurityPreparationDecision.Eligible,
        tone = ActionTone.Primary) {
          Text("Prepare fix")
        }
    if (preparation is SecurityPreparationDecision.Blocked)
        SelectionContainer {
          Text(preparation.reason, color = SecondaryText, style = IdeTypography.compactBody)
        }
    IdeDisclosureHeader("Report metadata", technical, { technical = !technical })
    if (technical) {
      val report = result.report
      listOf(
              "Report source" to report.source,
              "Evidence kind" to finding.evidenceKind,
              "Report path" to report.path,
              "Scope" to report.scope,
              "Content hash" to report.contentHash,
              "Ruleset version" to report.ruleSetVersion,
              "Profile" to report.profile,
              "Model" to report.model,
              "Configured model" to report.configuredModel,
              "Provider origin" to report.providerOrigin,
              "Reasoning effort" to report.reasoningEffort,
              "Generated at" to report.generatedAt,
              "Schema version" to report.schemaVersion,
              "Prompt version" to report.promptVersion,
              "Context policy version" to report.contextPolicyVersion,
              "Project ID" to report.projectId,
              "Project revision" to report.projectRevision,
              "Finding ID" to finding.id)
          .forEach { (label, value) -> SecurityDetailField(label, value) }
    }
  }
}
