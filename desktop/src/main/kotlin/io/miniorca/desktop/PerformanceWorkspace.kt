package io.miniorca.desktop

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.ClipEntry
import androidx.compose.ui.platform.LocalClipboard
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.awt.datatransfer.StringSelection
import java.util.Locale
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

@Composable
internal fun PerformanceWorkspacePane(
    state: PerformanceWorkspacePaneState,
    actions: PerformanceWorkspaceActions
) {
  val results = performanceResults(state.page)
  val semantic = state.page.semantic
  val currentBenchmarkChoice =
      state.selectedBenchmark.takeIf { state.benchmarkEligibility.canCompare }
  val benchmarkStatus =
      performanceBenchmarkStatusPresentation(
          state.benchmarkComparison,
          state.expectedBenchmarkIdentity,
          currentBenchmarkChoice,
          state.benchmarkDiscovery,
          state.benchmarkAdmission,
          state.benchmarkEligibility,
          state.benchmarkCatalog,
          state.benchmarkLatestOutcome)
  AnalysisResultsPane(
      page = state.page,
      rows = results.map { it.row() } + semantic.map(::semanticResultRow),
      browser = state.browser,
      facetLabel = "Impact",
      openAnalysis = actions.openAnalysis,
      retryResults = actions.retryResults,
      tools = {
        val selected = state.browser.selectedKey
        val selectedResult = results.firstOrNull { it.row().key == selected }
        val selectedSemantic = semantic.firstOrNull { semanticResultRow(it).key == selected }
        var benchmarksExpanded by
            remember(state.browser.identity, selected, selectedResult, selectedSemantic) {
              mutableStateOf(false)
            }
        var measurementDetailsExpanded by
            remember(state.browser.identity, selected, selectedResult, selectedSemantic) {
              mutableStateOf(false)
            }
        val latestResponse =
            state.benchmarkLatestOutcome?.response?.takeUnless { it == state.benchmarkComparison }
        var responseDetailsExpanded by remember(latestResponse) { mutableStateOf(false) }
        IdeDisclosureHeader(
            "Explore benchmark evidence",
            benchmarksExpanded,
            { benchmarksExpanded = !benchmarksExpanded },
            stateLabel = benchmarkStatus.stateLabel)
        SelectionContainer {
          Text(
              benchmarkStatus.summary,
              color = SecondaryText,
              style = IdeTypography.compactBody,
              modifier = Modifier.padding(top = 4.dp))
        }
        // Use the bounded result overview's scroll owner so long catalogs and required admission
        // text do not compete with a second fixed-height viewport.
        if (benchmarksExpanded)
            Column(Modifier.fillMaxWidth().testTag("benchmark-discovery-content")) {
              PerformanceBenchmarkControls(
                  state.benchmarkCatalog,
                  state.selectedBenchmark,
                  state.benchmarkEligibility,
                  state.benchmarkDiscovery,
                  state.benchmarkAdmission,
                  actions)
              latestResponse?.let { response ->
                IdeDisclosureHeader(
                    "Latest response details",
                    responseDetailsExpanded,
                    { responseDetailsExpanded = !responseDetailsExpanded })
                if (responseDetailsExpanded) {
                  Column(Modifier.fillMaxWidth().testTag("benchmark-latest-response")) {
                    IdePaneHeader("Latest comparison response", icon = DesktopIcon.Performance)
                    RecordedBenchmarkRows(performanceBenchmarkResponseRows(response))
                    PerformanceBenchmarkRecordedDetails(
                        response,
                        copyLabel = "Copy displayed response details",
                        copyTag = "benchmark-copy-response") { conditions, samples ->
                          performanceBenchmarkResponseCopyText(response, conditions, samples)
                        }
                  }
                }
              }
              state.benchmarkComparison?.let { comparison ->
                val presentation =
                    performanceBenchmarkPresentation(
                        comparison,
                        state.expectedBenchmarkIdentity,
                        currentBenchmarkChoice,
                        benchmarkStatus.priorEvidence)
                if (benchmarkStatus.priorEvidence) {
                  SelectionContainer {
                    Text(
                        presentation.summary,
                        color = Warning,
                        style = IdeTypography.compactBody,
                        modifier = Modifier.padding(vertical = 4.dp))
                  }
                }
                IdeDisclosureHeader(
                    if (benchmarkStatus.priorEvidence) "Prior measurement details"
                    else "Measurement details",
                    measurementDetailsExpanded,
                    { measurementDetailsExpanded = !measurementDetailsExpanded },
                    stateLabel = benchmarkStatus.stateLabel)
                if (measurementDetailsExpanded)
                    PerformanceBenchmarkEvidence(
                        comparison,
                        state.expectedBenchmarkIdentity,
                        currentBenchmarkChoice,
                        benchmarkStatus.priorEvidence)
              }
            }
      }) { key ->
        val result = results.firstOrNull { it.row().key == key }
        if (result != null) PerformanceFindingDetails(result, state.index, actions)
        else
            semantic
                .firstOrNull { semanticResultRow(it).key == key }
                ?.let {
                  FindingDetailsRegion(
                      it,
                      actions.semanticActions,
                      findingPreparationDecision(it, state.page.project, semantic, state.index),
                      sourceAvailable = findingNavigationTarget(it, state.index) != null)
                }
      }
}

internal data class PerformanceResult(
    val report: PerformanceFileReport,
    val finding: PerformanceFinding,
    val stale: Boolean,
    val page: AnalysisResultPageState,
) {
  fun row() =
      ResultRowPresentation(
          "performance:${report.path}:${finding.id}",
          finding.title.ifBlank { "Untitled opportunity" },
          buildString {
            append(report.path.ifBlank { "Path not supplied" })
            if (finding.startLine > 0) append(":${finding.startLine}")
            else append(" · Source line not supplied")
            append(" · ").append(finding.symbol.ifBlank { "Symbol not supplied" })
          },
          finding.observedPattern,
          finding.potentialImpact.ifBlank { "Unknown impact" },
          "Model suggestion",
          listOfNotNull(
                  report.status.takeIf { it != "completed" }?.replaceFirstChar(Char::uppercase),
                  "Stale".takeIf { stale && report.status != "stale" })
              .joinToString(" · "))
}

internal fun performanceSelectionGuard(
    page: AnalysisResultPageState,
    browser: ResultBrowserState,
    result: PerformanceResult,
): () -> Boolean {
  val generation = browser.selectionGeneration
  return {
    browser.selectionGeneration == generation &&
        resultBrowserSelection(
            browser.selectedKey,
            filteredResultRows(
                performanceResults(page).map { it.row() } + page.semantic.map(::semanticResultRow),
                browser.filter,
                browser.query),
            browser.explicitTarget) == result.row().key
  }
}

internal fun performanceResults(page: AnalysisResultPageState): List<PerformanceResult> =
    page.results
        ?.performance
        .orEmpty()
        .filter {
          it.projectId == page.project?.projectId &&
              it.projectRevision == page.run?.identity?.projectRevision
        }
        .flatMap { report ->
          report.findings.map {
            PerformanceResult(report, it, page.stale || report.status == "stale", page)
          }
        }
        .sortedWith(
            compareByDescending<PerformanceResult> {
                  performanceImpactOrder(it.finding.potentialImpact)
                }
                .thenByDescending { performanceConfidenceOrder(it.finding.confidence) }
                .thenBy { it.report.path }
                .thenBy { it.finding.startLine }
                .thenBy { it.finding.id })

internal sealed interface PerformancePreparationDecision {
  data class Eligible(
      val projectId: String,
      val projectRevision: String,
      val path: String,
      val contentHash: String,
      val declaration: SymbolInfo,
  ) : PerformancePreparationDecision

  data class Blocked(val reason: String) : PerformancePreparationDecision
}

/** Indexed evidence is only a preflight; preparation must recheck the loaded file and symbols. */
internal fun performancePreparationDecision(
    result: PerformanceResult,
    index: ProjectIndex?,
): PerformancePreparationDecision {
  fun blocked(reason: String) = PerformancePreparationDecision.Blocked(reason)
  val page = result.page
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
      return blocked(
          "Load the current project index and Performance results before preparing a fix.")
  if (result.report.projectId != project.projectId ||
      result.report.projectRevision != project.projectRevision ||
      page.results?.performance?.count { report ->
        report == result.report && report.findings.count { it == result.finding } == 1
      } != 1)
      return blocked(
          "This opportunity is missing or ambiguous in the current Performance results. Refresh the analysis.")
  if (result.stale || page.stale)
      return blocked("Analyze again to prepare a fix from current source.")
  if (result.report.status !in setOf("completed", "partial"))
      return blocked("Wait for a completed or partial Performance report before preparing a fix.")
  if (result.report.path.isBlank())
      return blocked("The report must identify an indexed target file path.")
  val files = index.files.filter { it.path == result.report.path }
  if (files.size != 1)
      return blocked("The target file is missing or ambiguous in the project index.")
  val file = files.single()
  if (result.report.contentHash.isBlank() ||
      file.contentHash.isBlank() ||
      result.report.contentHash != file.contentHash)
      return blocked(
          "The report's file hash is missing or no longer matches the indexed source. Reanalyze the file.")
  if (result.finding.symbol.isBlank())
      return blocked("The opportunity must name one indexed declaration.")
  val declarations = file.symbols.filter { it.name == result.finding.symbol }
  if (declarations.size != 1)
      return blocked("The opportunity must identify one unambiguous indexed declaration.")
  val declaration = declarations.single()
  if (result.finding.startLine <= 0 ||
      result.finding.startLine !in declaration.startLine..declaration.endLine)
      return blocked(
          "The reported source line must fall within the indexed declaration. Reanalyze the file.")
  val eligibility =
      symbolEditEligibility(
          ProjectFileInfo(
              file.path,
              file.contentHash,
              file.path.substringAfterLast('/'),
              language = file.language,
              sizeBytes = file.sizeBytes,
              lineCount = file.lineCount,
              modifiedAt = file.modifiedAt,
              binary = file.binary),
          file.symbols,
          declaration)
  if (!eligibility.eligible) return blocked(eligibility.blockedReason)
  return PerformancePreparationDecision.Eligible(
      project.projectId, project.projectRevision, file.path, file.contentHash, declaration)
}

internal fun loadedPerformancePreparationDecision(
    target: PerformancePreparationDecision.Eligible,
    finding: PerformanceFinding,
    file: ProjectFileInfo,
    symbols: List<SymbolInfo>,
): PerformancePreparationDecision {
  if (file.path != target.path || file.contentHash != target.contentHash)
      return PerformancePreparationDecision.Blocked(
          "Loaded source no longer matches the indexed file hash. Reanalyze the file.")
  val matches = symbols.filter { it.name == target.declaration.name }
  if (matches.size != 1 ||
      matches.single() != target.declaration ||
      finding.startLine !in matches.single().startLine..matches.single().endLine)
      return PerformancePreparationDecision.Blocked(
          "Loaded source no longer contains the exact indexed declaration and anchor. Reanalyze the file.")
  val eligibility = symbolEditEligibility(file, symbols, matches.single())
  if (!eligibility.eligible)
      return PerformancePreparationDecision.Blocked(eligibility.blockedReason)
  return target
}

internal fun performancePreparationRequest(finding: PerformanceFinding): String = buildString {
  append("Optimize ")
      .append(finding.symbol)
      .append(" without changing behavior. Keep the change within this declaration.")
  listOf(
          "Observed pattern" to finding.observedPattern,
          "Recommendation" to finding.recommendation,
          "Workload conditions" to finding.workloadConditions,
          "Trade-offs" to finding.tradeoff,
          "Verification plan" to finding.verificationPlan,
      )
      .forEach { (label, value) ->
        if (value.isNotBlank()) append("\n").append(label).append(": ").append(value)
      }
}

@Composable
private fun PerformanceField(label: String, value: String) {
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
    Text(label, color = SecondaryText, style = IdeTypography.workspaceMetadata)
    ModelResultContent(
        value.ifBlank { "Not supplied." }, preview = false, style = IdeTypography.workspaceBody)
  }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun PerformanceFindingDetails(
    result: PerformanceResult,
    index: ProjectIndex?,
    actions: PerformanceWorkspaceActions
) {
  val finding = result.finding
  val preparation = performancePreparationDecision(result, index)
  var technical by remember(result.row().key) { mutableStateOf(false) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
    ResultDetailHeader(result.row())
    IdeLabelBadge("Unmeasured", Warning, icon = DesktopIcon.Performance)
    PerformanceField("Report status", result.report.status.ifBlank { "Not supplied." })
    PerformanceField(
        "Freshness", if (result.stale) "Stale · saved evidence" else "Current for this analysis")
    PerformanceField("Observed pattern", finding.observedPattern)
    PerformanceField("Potential impact", finding.potentialImpact)
    PerformanceField("Model confidence", finding.confidence)
    PerformanceField("Recommendation", finding.recommendation)
    FlowRow(
        horizontalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.compact),
        verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.compact)) {
          MiniOrcaButton(onClick = { actions.openSource(result) }, tone = ActionTone.Neutral) {
            Text("Open source")
          }
          MiniOrcaButton(
              onClick = { actions.prepareOptimization(result) },
              enabled = preparation is PerformancePreparationDecision.Eligible,
              tone = ActionTone.Primary) {
                Text("Prepare fix")
              }
        }
    if (preparation is PerformancePreparationDecision.Blocked)
        Text(preparation.reason, color = SecondaryText, style = IdeTypography.compactBody)
    if (result.stale || result.report.status == "partial")
        Text(
            if (result.stale) "Outdated · analyze again"
            else "Partial report · incomplete evidence",
            color = Warning,
            style = IdeTypography.compactBody)
    PerformanceField("Report warning", result.report.warning)
    PerformanceField("Workload conditions", finding.workloadConditions)
    PerformanceField("Trade-offs", finding.tradeoff)
    PerformanceField("Verification plan", finding.verificationPlan)
    if (finding.engineeringInsight == null ||
        engineeringInsightPieces(finding.engineeringInsight).isEmpty())
        PerformanceField("Engineering insight", "Not supplied.")
    else
        EngineeringInsightPanel(
            finding.engineeringInsight,
            stale = result.stale,
            scopeLabel = "Selected performance opportunity")
    IdeDisclosureHeader("Report metadata", technical, { technical = !technical })
    if (technical) {
      val report = result.report
      listOf(
              "Finding category" to finding.category,
              "Finding ID" to finding.id,
              "Profile" to report.profile,
              "Model" to report.model,
              "Provider origin" to report.providerOrigin,
              "Scope" to report.scope,
              "Reasoning effort" to report.reasoningEffort,
              "Generated at" to report.generatedAt,
              "Schema version" to report.schemaVersion,
              "Prompt version" to report.promptVersion,
              "Context policy version" to report.contextPolicyVersion,
              "Project ID" to report.projectId,
              "Project revision" to report.projectRevision,
              "Content hash" to report.contentHash)
          .forEach { (label, value) -> PerformanceField(label, value) }
    }
  }
}

/**
 * Displays already captured PERF-02 evidence. It has no benchmark action or process side effect.
 */
@Composable
private fun PerformanceBenchmarkEvidence(
    comparison: GoBenchmarkComparison,
    expectedIdentity: GoBenchmarkComparisonIdentity?,
    expectedChoice: GoBenchmarkChoice?,
    priorEvidence: Boolean,
) {
  val presentation =
      performanceBenchmarkPresentation(comparison, expectedIdentity, expectedChoice, priorEvidence)
  val historical = priorEvidence || presentation.isStale
  Column(Modifier.fillMaxWidth().testTag("benchmark-measurement-evidence")) {
    IdePaneHeader(
        title =
            if (priorEvidence || presentation.isStale) "Prior benchmark evidence"
            else "Benchmark evidence",
        icon = DesktopIcon.Performance,
        stateLabel = presentation.stateLabel,
        stateTint =
            when {
              presentation.isStale || presentation.inconclusive -> Warning
              presentation.isMeasured -> Success
              else -> SecondaryText
            },
    )
    SelectionContainer {
      Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
        Text(
            presentation.summary,
            color = if (presentation.inconclusive || presentation.isStale) Warning else PrimaryText,
            fontSize = 12.sp,
            lineHeight = 18.sp,
            modifier = Modifier.padding(top = 6.dp))
        if (presentation.insights.isNotEmpty()) {
          Text(
              if (priorEvidence || presentation.isStale) "Prior measured trade-offs"
              else "Measured trade-offs",
              color = SecondaryText,
              fontSize = 10.sp,
              modifier = Modifier.padding(top = 8.dp))
          presentation.insights.forEach { insight ->
            Text(
                insight,
                color = SecondaryText,
                fontSize = 11.sp,
                lineHeight = 16.sp,
                modifier = Modifier.padding(top = 2.dp))
          }
        }
        PerformanceBenchmarkMedians(
            presentation.metrics, historical = priorEvidence || presentation.isStale)
        CompactKeyValueRows(
            presentation.rows.filter { row ->
              presentation.metrics.none { it.label == row.first } && row.first != "Samples"
            })
        Text(
            presentation.conditions,
            color = SecondaryText,
            fontSize = 11.sp,
            lineHeight = 16.sp,
            modifier = Modifier.padding(top = 6.dp))
      }
    }
    PerformanceBenchmarkRecordedDetails(comparison) { conditions, samples ->
      performanceBenchmarkCopyText(comparison, presentation, historical, conditions, samples)
    }
  }
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun PerformanceBenchmarkRecordedDetails(
    comparison: GoBenchmarkComparison,
    copyLabel: String = "Copy displayed benchmark evidence",
    copyTag: String = "benchmark-copy-evidence",
    copyText: (conditionsExpanded: Boolean, samplesExpanded: Boolean) -> String,
) {
  var conditionsExpanded by remember(comparison) { mutableStateOf(false) }
  var samplesExpanded by remember(comparison) { mutableStateOf(false) }
  val clipboard = LocalClipboard.current
  val copyScope = rememberCoroutineScope()
  var copyFeedback by remember(comparison) { mutableStateOf<String?>(null) }
  Column(Modifier.fillMaxWidth()) {
    IdeDisclosureHeader(
        "Recorded conditions & identity",
        conditionsExpanded,
        { conditionsExpanded = !conditionsExpanded })
    if (conditionsExpanded) RecordedBenchmarkRows(performanceBenchmarkRecordedRows(comparison))
    IdeDisclosureHeader(
        "Returned sample details", samplesExpanded, { samplesExpanded = !samplesExpanded })
    if (samplesExpanded) RecordedBenchmarkRows(performanceBenchmarkSampleRows(comparison))
    ChromeButton(
        onClick = {
          val payload = copyText(conditionsExpanded, samplesExpanded)
          copyFeedback = null
          copyScope.launch {
            try {
              clipboard.setClipEntry(ClipEntry(StringSelection(payload)))
              copyFeedback = "Displayed benchmark evidence copied."
            } catch (cancelled: CancellationException) {
              throw cancelled
            } catch (exception: Exception) {
              copyFeedback =
                  "Could not copy benchmark evidence: ${exception.message ?: "Clipboard unavailable"}"
            }
          }
        },
        accessibleName = copyLabel,
        tooltip = null,
        modifier = Modifier.testTag(copyTag)) {
          Text(copyLabel, style = IdeTypography.compactBody)
        }
    copyFeedback?.let { Text(it, color = SecondaryText, style = IdeTypography.compactBody) }
  }
}

@Composable
private fun RecordedBenchmarkRows(rows: List<Pair<String, String>>) {
  SelectionContainer {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
      rows.forEach { (label, value) ->
        Text("$label: $value", color = SecondaryText, style = IdeTypography.compactBody)
      }
    }
  }
}

@Composable
private fun PerformanceBenchmarkMedians(
    metrics: List<BenchmarkMetricRow>,
    historical: Boolean,
) {
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    // Budget for side medians and coverage text, not just the short numeric values. Use the
    // actual pane width in dp so density and larger text do not leave cramped columns.
    val stacked = maxWidth < 640.dp * LocalDensity.current.fontScale
    Column(
        Modifier.fillMaxWidth()
            .testTag(if (stacked) "benchmark-medians-stacked" else "benchmark-medians-columns")) {
          if (!stacked) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
              Text(
                  "Metric",
                  Modifier.weight(0.8f),
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
              Text(
                  "Baseline median",
                  Modifier.weight(1.5f),
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
              Text(
                  "Candidate median",
                  Modifier.weight(1.5f),
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
              Text(
                  "Change / availability",
                  Modifier.weight(1.3f),
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
            }
          }
          metrics.firstOrNull()?.let { first ->
            for ((label, side) in
                listOf("Baseline" to first.base, "Candidate" to first.candidate)) {
              Text(
                  "$label samples: ${side.sampleCount ?: "not returned"}",
                  color = SecondaryText,
                  style = IdeTypography.compactBody,
                  modifier = Modifier.padding(top = 4.dp))
            }
          }
          metrics.forEach { metric ->
            val label =
                when (metric.label) {
                  "ns/op" -> "Time (ns/op)"
                  "B/op" -> "Bytes (B/op)"
                  else -> "Allocations (allocs/op)"
                }
            val rowModifier =
                Modifier.fillMaxWidth()
                    .padding(vertical = 6.dp)
                    .testTag("benchmark-metric-${metric.label}")
            if (stacked) {
              Column(rowModifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text(label, color = PrimaryText, style = IdeTypography.compactBody)
                BenchmarkMedianSide(metric.label, "Baseline", metric.base, labeled = true)
                BenchmarkMedianSide(metric.label, "Candidate", metric.candidate, labeled = true)
                Text(
                    "Change / availability",
                    color = SecondaryText,
                    style = IdeTypography.compactBody)
                Text(
                    metric.changeLabel(historical),
                    color = if (metric.isComplete) SecondaryText else Warning,
                    style = IdeTypography.compactBody)
              }
            } else {
              Row(rowModifier, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    label,
                    Modifier.weight(0.8f),
                    color = PrimaryText,
                    style = IdeTypography.compactBody)
                BenchmarkMedianSide(
                    metric.label, "Baseline", metric.base, modifier = Modifier.weight(1.5f))
                BenchmarkMedianSide(
                    metric.label, "Candidate", metric.candidate, modifier = Modifier.weight(1.5f))
                Text(
                    metric.changeLabel(historical),
                    Modifier.weight(1.3f),
                    color = if (metric.isComplete) SecondaryText else Warning,
                    style = IdeTypography.compactBody)
              }
            }
          }
        }
  }
}

@Composable
private fun BenchmarkMedianSide(
    unit: String,
    label: String,
    side: BenchmarkMetricSide,
    modifier: Modifier = Modifier,
    labeled: Boolean = false,
) {
  Column(modifier.semantics { contentDescription = "$unit, $label median: ${side.display()}" }) {
    if (labeled) Text("$label median", color = SecondaryText, style = IdeTypography.compactBody)
    Text(side.medianLabel(unit), color = PrimaryText, style = IdeTypography.compactBody)
    Text(side.availabilityLabel(), color = SecondaryText, style = IdeTypography.compactBody)
  }
}

internal data class PerformanceBenchmarkStatusPresentation(
    val stateLabel: String,
    val summary: String,
    val priorEvidence: Boolean = false,
)

/**
 * Keeps the benchmark status visible even before an explicit local comparison exists. A benchmark
 * comparison applies to its candidate identity and never validates a separate model suggestion.
 */
internal fun performanceBenchmarkStatusPresentation(
    comparison: GoBenchmarkComparison?,
    expectedIdentity: GoBenchmarkComparisonIdentity?,
    expectedChoice: GoBenchmarkChoice?,
    discovery: BenchmarkDiscoveryOutcome = BenchmarkDiscoveryOutcome.NotRequested,
    admission: BenchmarkAdmissionOutcome = BenchmarkAdmissionOutcome.Idle,
    eligibility: BenchmarkEligibility? = null,
    catalog: GoBenchmarkCatalog? = null,
    latestOutcome: BenchmarkComparisonOutcome? = null,
): PerformanceBenchmarkStatusPresentation {
  fun current(label: String, summary: String, prior: Boolean = comparison != null) =
      PerformanceBenchmarkStatusPresentation(label, summary, priorEvidence = prior)
  return when {
    admission == BenchmarkAdmissionOutcome.Admitting ->
        current("Admitting · execution trust", "Checking execution trust")
    admission == BenchmarkAdmissionOutcome.Running ->
        current("Running · explicit local execution", "Running in temporary copies")
    discovery == BenchmarkDiscoveryOutcome.Loading ->
        current("Listing · read-only discovery", "Loading compatible benchmarks…")
    admission is BenchmarkAdmissionOutcome.Failed ->
        current("Benchmark admission failed", admission.message)
    admission == BenchmarkAdmissionOutcome.Stopped ->
        current("Benchmark admission stopped", "Stopped · previous evidence retained")
    latestOutcome != null &&
        (latestOutcome.status != BenchmarkComparisonStatus.Completed ||
            latestOutcome.response != comparison) ->
        current(
            benchmarkOutcomeLabel(latestOutcome),
            latestOutcome.response.reason.ifBlank {
              when (latestOutcome.status) {
                BenchmarkComparisonStatus.Completed ->
                    "The comparison completed without new measurements."
                BenchmarkComparisonStatus.Canceled -> "The daemon canceled the comparison."
                BenchmarkComparisonStatus.Failed -> "The daemon reported a comparison failure."
                BenchmarkComparisonStatus.Unavailable ->
                    "The comparison is unavailable. Refresh compatible benchmarks before trying again."
                BenchmarkComparisonStatus.Unsupported ->
                    "The daemon returned unsupported status ${latestOutcome.response.status.ifBlank { "(not recorded)" }}; no successful comparison is confirmed."
              }
            },
            prior = comparison != null && latestOutcome.response != comparison)
    discovery is BenchmarkDiscoveryOutcome.Failed ->
        current("Benchmark lookup failed", discovery.message)
    discovery is BenchmarkDiscoveryOutcome.Unavailable ->
        current(
            "Discovery unavailable",
            discovery.reason.ifBlank { "No compatible benchmark · refresh catalog" })
    discovery == BenchmarkDiscoveryOutcome.Invalidated ->
        current("Discovery invalidated", "Catalog outdated · refresh benchmarks")
    discovery == BenchmarkDiscoveryOutcome.Loaded && catalog?.benchmarks?.isEmpty() == true ->
        current("No compatible benchmarks", "No compatible benchmarks")
    discovery == BenchmarkDiscoveryOutcome.Loaded && eligibility?.canCompare == false ->
        current("Comparison blocked", eligibility.comparisonBlockedReason!!)
    comparison != null ->
        performanceBenchmarkPresentation(
                comparison,
                expectedIdentity,
                expectedChoice.takeIf { eligibility?.canCompare == true })
            .let {
              PerformanceBenchmarkStatusPresentation(
                  it.stateLabel,
                  (listOf(it.summary) + it.insights + "Candidate benchmark · suggestion unmeasured")
                      .joinToString("\n"),
                  priorEvidence = it.isStale)
            }
    else ->
        PerformanceBenchmarkStatusPresentation(
            "Not measured · explicit local execution", "Not benchmarked")
  }
}

/** Listing a catalog is read-only; only the explicitly labeled run action can execute code. */
@Composable
private fun PerformanceBenchmarkControls(
    catalog: GoBenchmarkCatalog?,
    selected: GoBenchmarkChoice?,
    eligibility: BenchmarkEligibility,
    discovery: BenchmarkDiscoveryOutcome,
    admission: BenchmarkAdmissionOutcome,
    actions: PerformanceWorkspaceActions,
) {
  val active =
      admission == BenchmarkAdmissionOutcome.Admitting ||
          admission == BenchmarkAdmissionOutcome.Running
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = "Benchmark comparison",
        icon = DesktopIcon.Performance,
        stateLabel =
            when {
              discovery == BenchmarkDiscoveryOutcome.Loading -> "Read-only lookup in progress"
              admission == BenchmarkAdmissionOutcome.Admitting -> "Checking execution trust"
              admission == BenchmarkAdmissionOutcome.Running -> "Running in isolated copies"
              eligibility.candidate is BenchmarkCandidateDecision.Blocked -> "Candidate unavailable"
              discovery != BenchmarkDiscoveryOutcome.Loaded || catalog == null ->
                  "Read-only discovery"
              !catalog.available -> "Not available"
              catalog.benchmarks.isEmpty() -> "No compatible benchmarks"
              selected == null -> "Select one benchmark"
              !eligibility.canCompare -> "Comparison blocked"
              catalog.trusted -> "Ready to run"
              else -> "Local execution needs trust"
            },
        stateTint =
            if (catalog?.available == false || !eligibility.canDiscover) Warning else SecondaryText,
    )
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      Text(
          if (eligibility.candidate is BenchmarkCandidateDecision.Ready)
              "Current validated candidate."
          else "A current validated candidate is required.",
          color = SecondaryText,
          style = IdeTypography.compactBody)
      eligibility.discoveryBlockedReason?.let {
        SelectionContainer { Text(it, color = Warning, fontSize = 11.sp, lineHeight = 16.sp) }
      }
      Text(
          "Refresh clears the benchmark selection",
          color = SecondaryText,
          fontSize = 11.sp,
          lineHeight = 16.sp)
      MiniOrcaButton(
          onClick = actions.loadBenchmarks,
          enabled = eligibility.canDiscover && !active,
          tone = ActionTone.Neutral,
          modifier =
              Modifier.padding(top = 6.dp).testTag("benchmark-discovery").semantics {
                eligibility.discoveryBlockedReason?.let { stateDescription = it }
              }) {
            Text(
                if (discovery == BenchmarkDiscoveryOutcome.NotRequested)
                    "List compatible benchmarks"
                else "Refresh compatible benchmarks",
                fontSize = 11.sp)
          }
      if (discovery != BenchmarkDiscoveryOutcome.Loaded || catalog == null) return@Column
      if (!catalog.available) {
        Text(
            catalog.reason.ifBlank { "No compatible benchmark is available for this candidate." },
            color = Warning,
            fontSize = 11.sp,
            lineHeight = 16.sp)
        return@Column
      }
      if (catalog.benchmarks.isEmpty()) {
        Text("No compatible benchmarks", color = Warning, style = IdeTypography.compactBody)
        return@Column
      }
      eligibility.comparisonBlockedReason?.let {
        SelectionContainer { Text(it, color = Warning, fontSize = 11.sp, lineHeight = 16.sp) }
      }
      Text("Select a benchmark", color = SecondaryText, fontSize = 11.sp, lineHeight = 16.sp)
      catalog.benchmarks.forEachIndexed { index, choice ->
        ChromeButton(
            onClick = { actions.selectBenchmark(choice) },
            selected = choice == selected,
            role = Role.RadioButton,
            accessibleName = "Select benchmark ${choice.name.ifBlank { "Unnamed benchmark" }}",
            tooltip = null,
            modifier =
                Modifier.fillMaxWidth()
                    .padding(top = 4.dp)
                    .testTag("benchmark-choice-$index")
                    .semantics {
                      this.selected = choice == selected
                      stateDescription = if (choice == selected) "Selected" else "Not selected"
                    }) {
              Text(
                  "${if (choice == selected) "Selected" else "Select"} · ${choice.name.ifBlank { "Unnamed benchmark" }}",
                  fontSize = 11.sp,
                  modifier = Modifier.weight(1f))
            }
      }
      selected?.let { choice ->
        PerformanceBenchmarkAdmissionDisclosure(choice, eligibility.candidate, catalog.trusted)
        MiniOrcaButton(
            onClick = actions.runBenchmark,
            enabled = eligibility.canCompare && !active,
            tone = ActionTone.Primary,
            modifier =
                Modifier.padding(top = 6.dp).testTag("benchmark-run").semantics {
                  eligibility.comparisonBlockedReason?.let { stateDescription = it }
                }) {
              Text(
                  if (admission == BenchmarkAdmissionOutcome.Admitting) "Checking execution trust…"
                  else if (admission == BenchmarkAdmissionOutcome.Running) "Comparing benchmark…"
                  else if (catalog.trusted) "Run selected benchmark"
                  else "Trust and run selected benchmark",
                  fontSize = 11.sp)
            }
      }
    }
  }
}

/** Values are taken from the current validated candidate, not retained measurement evidence. */
internal fun performanceBenchmarkAdmissionRows(
    choice: GoBenchmarkChoice,
    candidate: BenchmarkCandidateDecision,
): List<Pair<String, String>> = buildList {
  add("Selected benchmark" to choice.name)
  (candidate as? BenchmarkCandidateDecision.Ready)?.draft?.let { draft ->
    add("Project ID" to draft.projectId)
    add("Project revision" to draft.projectRevision)
    add("Target path" to draft.targetPath)
    add("Validated draft revision" to draft.revision.toString())
    add(
        "Package working directory" to
            draft.targetPath.substringBeforeLast('/', "").ifEmpty { "." })
  }
  add("Opaque scope guard (identity metadata)" to choice.scope)
}

/** JSON quoting keeps empty arguments, whitespace and control characters unambiguous. */
internal fun performanceBenchmarkArgv(command: List<String>): String =
    command
        .mapIndexed { index, argument -> "argv[$index] = ${Json.encodeToString(argument)}" }
        .joinToString("\n")

@OptIn(ExperimentalComposeUiApi::class)
@Composable
private fun PerformanceBenchmarkAdmissionDisclosure(
    choice: GoBenchmarkChoice,
    candidate: BenchmarkCandidateDecision,
    trusted: Boolean,
) {
  Column(
      Modifier.fillMaxWidth().padding(top = 6.dp),
      verticalArrangement = Arrangement.spacedBy(4.dp)) {
        SelectionContainer {
          Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            performanceBenchmarkAdmissionRows(choice, candidate).forEach { (label, value) ->
              Text("$label: $value", color = SecondaryText, style = IdeTypography.compactBody)
            }
            Text(
                "Directory relative to project root",
                color = SecondaryText,
                style = IdeTypography.compactBody)
          }
        }
        val clipboard = LocalClipboard.current
        val copyScope = rememberCoroutineScope()
        var copyFeedback by remember(choice) { mutableStateOf<String?>(null) }
        var copyFocused by remember { mutableStateOf(false) }
        val copyReveal = remember { BringIntoViewRequester() }
        LaunchedEffect(copyFocused, copyFeedback) {
          if (copyFocused) {
            withFrameNanos {}
            copyReveal.bringIntoView()
          }
        }
        ChromeButton(
            onClick = {
              copyFeedback = null
              copyScope.launch {
                try {
                  clipboard.setClipEntry(
                      ClipEntry(StringSelection(performanceBenchmarkArgv(choice.command))))
                  copyFeedback = "Selected benchmark argv copied."
                } catch (cancelled: CancellationException) {
                  throw cancelled
                } catch (exception: Exception) {
                  copyFeedback =
                      "Could not copy selected benchmark argv: ${exception.message ?: "Clipboard unavailable"}"
                }
              }
            },
            accessibleName = "Copy selected benchmark argv",
            tooltip = null,
            modifier =
                Modifier.testTag("benchmark-copy-argv")
                    .bringIntoViewRequester(copyReveal)
                    .onFocusChanged { copyFocused = it.isFocused }) {
              Text("Copy argv", style = IdeTypography.compactBody)
            }
        copyFeedback?.let { Text(it, color = SecondaryText, style = IdeTypography.compactBody) }
        var commandFocused by remember { mutableStateOf(false) }
        val commandReveal = remember { BringIntoViewRequester() }
        // Reveal the same bounds used by selection focus, after layout has incorporated
        // selection/copy feedback. A child requester can compete with the automatic focus reveal.
        LaunchedEffect(commandFocused, copyFeedback) {
          if (commandFocused) {
            withFrameNanos {}
            commandReveal.bringIntoView()
          }
        }
        // Keep this passive focus target scoped to argv, not the entire consent disclosure.
        SelectionContainer(
            Modifier.fillMaxWidth()
                .testTag("benchmark-argv")
                .bringIntoViewRequester(commandReveal)
                .onFocusChanged { commandFocused = it.hasFocus }
                .border(if (commandFocused) 2.dp else 0.dp, FocusAccent, MiniOrcaShapes.control)
                .semantics {
                  contentDescription = "Selected benchmark argv"
                  stateDescription = "Read-only"
                }) {
              Column {
                Text(
                    "Daemon-returned argv (read-only)",
                    color = PrimaryText,
                    style = IdeTypography.compactBody)
                performanceBenchmarkArgv(choice.command)
                    .ifEmpty { "No argv returned; execution is blocked." }
                    .lines()
                    .forEach { argument ->
                      Text(argument, color = SecondaryText, fontSize = 10.sp, lineHeight = 16.sp)
                    }
              }
            }
        SelectionContainer {
          Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text("Arguments · no shell", color = SecondaryText, style = IdeTypography.compactBody)
            listOf(
                    "Executes project code · file and network access",
                    "Temporary copies · not sandboxed")
                .forEach { Text(it, color = Warning, style = IdeTypography.compactBody) }
            Text(
                if (trusted) "Execution trusted · current project revision"
                else "Requires execution trust for this revision",
                color = SecondaryText,
                style = IdeTypography.compactBody)
            listOf(
                    "Trust scope: go test ./... · includes other Go tests",
                    "Trust duration: current project revision · daemon session",
                    "Runs the selected benchmark command")
                .forEach { Text(it, color = Warning, style = IdeTypography.compactBody) }
          }
        }
      }
}

internal data class PerformanceWorkspacePaneState(
    val page: AnalysisResultPageState,
    val index: ProjectIndex?,
    val benchmarkComparison: GoBenchmarkComparison? = null,
    val expectedBenchmarkIdentity: GoBenchmarkComparisonIdentity? = null,
    val benchmarkCatalog: GoBenchmarkCatalog? = null,
    val selectedBenchmark: GoBenchmarkChoice? = null,
    val benchmarkDiscovery: BenchmarkDiscoveryOutcome = BenchmarkDiscoveryOutcome.NotRequested,
    val benchmarkAdmission: BenchmarkAdmissionOutcome = BenchmarkAdmissionOutcome.Idle,
    val benchmarkLatestOutcome: BenchmarkComparisonOutcome? = null,
    val benchmarkEligibility: BenchmarkEligibility = benchmarkEligibility(DesktopState()),
    val browser: ResultBrowserState = newResultBrowserState(page),
)

internal data class PerformanceWorkspaceActions(
    val prepareOptimization: (PerformanceResult) -> Unit,
    val openAnalysis: () -> Unit,
    val semanticActions: FindingActions,
    val openSource: (PerformanceResult) -> Unit,
    val loadBenchmarks: () -> Unit = {},
    val selectBenchmark: (GoBenchmarkChoice) -> Unit = {},
    val runBenchmark: () -> Unit = {},
    val retryResults: (() -> Unit)? = null,
)

internal data class GoBenchmarkComparisonIdentity(
    val draftId: String,
    val draftRevision: Long,
    val draftHash: String,
    val projectId: String,
    val projectRevision: String,
    val baseFileHash: String,
    val targetPath: String,
)

internal fun goBenchmarkComparisonIdentity(
    draft: DeclarationDraft?,
): GoBenchmarkComparisonIdentity? =
    draft
        ?.takeIf {
          it.id.isNotBlank() &&
              it.revision > 0 &&
              it.hash.isNotBlank() &&
              it.projectId.isNotBlank() &&
              it.projectRevision.isNotBlank() &&
              it.baseFileHash.isNotBlank() &&
              it.targetPath.isNotBlank()
        }
        ?.let {
          GoBenchmarkComparisonIdentity(
              it.id,
              it.revision,
              it.hash,
              it.projectId,
              it.projectRevision,
              it.baseFileHash,
              it.targetPath,
          )
        }

internal data class PerformanceBenchmarkPresentation(
    val stateLabel: String,
    val rows: List<Pair<String, String>>,
    val conditions: String,
    val summary: String,
    val insights: List<String>,
    val isMeasured: Boolean,
    val isStale: Boolean,
    val inconclusive: Boolean,
    val metrics: List<BenchmarkMetricRow>,
)

/**
 * Converts a single selected benchmark comparison into labels. The presentation deliberately makes
 * only benchmark-scoped observations; it never claims that the project is faster.
 */
internal fun performanceBenchmarkPresentation(
    comparison: GoBenchmarkComparison,
    expectedIdentity: GoBenchmarkComparisonIdentity? = null,
    expectedChoice: GoBenchmarkChoice? = null,
    priorEvidence: Boolean = false,
): PerformanceBenchmarkPresentation {
  val assessment =
      benchmarkFreshnessPresentation(
          comparison,
          benchmarkMeasurementPresentation(comparison),
          expectedIdentity,
          expectedChoice)
  return if (priorEvidence)
      assessment.copy(
          stateLabel = "Prior evidence · ${assessment.stateLabel}",
          rows = benchmarkHistoricalRows(assessment),
          summary = "Prior comparison · ${assessment.summary}",
          insights = assessment.insights.map { "Prior observation: $it" },
          isMeasured = false)
  else assessment
}

private fun benchmarkFreshnessPresentation(
    comparison: GoBenchmarkComparison,
    assessment: PerformanceBenchmarkPresentation,
    expectedIdentity: GoBenchmarkComparisonIdentity?,
    expectedChoice: GoBenchmarkChoice?,
): PerformanceBenchmarkPresentation {
  val stale =
      when {
        expectedIdentity == null -> "Stale · no current candidate" to "No current draft"
        comparison.identityOrNull() != expectedIdentity ->
            "Stale · candidate identity changed" to "Draft or source changed"
        expectedChoice == null ->
            "Stale · no current benchmark selection" to "Select a compatible benchmark"
        comparison.benchmark != expectedChoice.name ||
            comparison.scope != expectedChoice.scope ||
            comparison.command != expectedChoice.command ->
            "Stale · selected benchmark changed" to "Benchmark selection changed"
        else -> null
      } ?: return assessment
  return assessment.copy(
      stateLabel = stale.first,
      rows = benchmarkHistoricalRows(assessment),
      summary = "${stale.second} · prior: ${assessment.summary}",
      insights = assessment.insights.map { "Historical observation: $it" },
      isMeasured = false,
      isStale = true,
      inconclusive = true)
}

private fun benchmarkHistoricalRows(
    assessment: PerformanceBenchmarkPresentation,
): List<Pair<String, String>> =
    assessment.rows.map { row ->
      val metric = assessment.metrics.firstOrNull { it.label == row.first }
      if (metric == null) row else row.first to metric.display(historical = true)
    }

private fun benchmarkMeasurementPresentation(
    comparison: GoBenchmarkComparison,
): PerformanceBenchmarkPresentation {
  val metrics = benchmarkMetricRows(comparison)
  val presentation =
      PerformanceBenchmarkPresentation(
          stateLabel = "Inconclusive · incomplete measurement evidence",
          rows =
              benchmarkIdentityRows(comparison) +
                  ("Samples" to
                      "${benchmarkSampleCount(comparison.base)} base · ${benchmarkSampleCount(comparison.candidate)} candidate") +
                  metrics.map { it.label to it.display() } +
                  metrics.flatMap { it.invalidRows() }.distinct(),
          conditions = benchmarkConditions(comparison),
          summary = "Incomplete · requires 5 valid samples per side",
          insights = emptyList(),
          isMeasured = false,
          isStale = false,
          inconclusive = true,
          metrics = metrics)
  if (comparison.status != "completed")
      return presentation.copy(
          stateLabel = benchmarkTerminalLabel(comparison),
          summary = comparison.reason.ifBlank { "Comparison unavailable" },
          inconclusive = comparison.status != "unavailable")
  if (metrics.any {
    it.base.availability == BenchmarkMetricAvailability.Invalid ||
        it.candidate.availability == BenchmarkMetricAvailability.Invalid
  })
      return presentation.copy(
          stateLabel = "Inconclusive · invalid samples",
          summary = "Invalid samples · valid-only medians")
  if (comparison.benchmark.isBlank() || !metrics.first().isComplete) return presentation

  val incompleteMemoryMetrics = metrics.drop(1).filter { !it.isComplete }
  if (incompleteMemoryMetrics.isNotEmpty())
      return presentation.copy(
          stateLabel = "Inconclusive · incomplete memory evidence",
          summary = "Incomplete ${incompleteMemoryMetrics.joinToString { it.label }}",
          insights = listOf("Memory evidence incomplete"))

  val variableMetrics = metrics.filter { it.variability > benchmarkVariabilityLimit }
  val cpu = metrics.first()
  val memoryRegressions = metrics.drop(1).filter { it.candidate.median!! > it.base.median!! }
  val memoryImprovements = metrics.drop(1).filter { it.candidate.median!! < it.base.median!! }
  val opposingMemorySignals = memoryRegressions.isNotEmpty() && memoryImprovements.isNotEmpty()
  val cpuImproved = cpu.candidate.median!! < cpu.base.median!!
  val insights = buildList {
    if (opposingMemorySignals) add("Memory metrics disagree")
    else if (cpuImproved && memoryRegressions.isNotEmpty())
        add(
            "CPU median is lower, while ${memoryRegressions.joinToString { it.label }} increased; " +
                "this is a trade-off, not an unconditional win.")
    else if (cpuImproved) add("Lower CPU median · ${comparison.benchmark} only")
    if (variableMetrics.isNotEmpty())
        add(
            "${variableMetrics.joinToString { it.label }} varies by more than " +
                "${formatPercent(benchmarkVariabilityLimit)} across samples.")
  }
  val inconclusive = variableMetrics.isNotEmpty() || opposingMemorySignals
  val summary =
      when {
        opposingMemorySignals ->
            "Inconclusive: ${memoryRegressions.joinToString { it.label }} increased while " +
                "${memoryImprovements.joinToString { it.label }} decreased."
        inconclusive -> "Inconclusive · variable ${variableMetrics.joinToString { it.label }}"
        cpuImproved && memoryRegressions.isNotEmpty() ->
            "Lower CPU · higher memory · selected benchmark"
        cpuImproved -> "Lower CPU · selected benchmark"
        else -> "No CPU improvement · selected benchmark"
      }
  return presentation.copy(
      stateLabel =
          when {
            opposingMemorySignals -> "Inconclusive · opposing memory signals"
            inconclusive -> "Inconclusive · noisy samples"
            else -> "Measured · selected benchmark"
          },
      rows = presentation.rows + ("Variability" to variabilityLabel(variableMetrics)),
      summary = summary,
      insights = insights,
      isMeasured = !inconclusive,
      inconclusive = inconclusive,
  )
}

private const val benchmarkVariabilityLimit = 0.10
private const val benchmarkRequiredSamples = 5

internal enum class BenchmarkMetricAvailability {
  MissingMeasurement,
  EmptySamples,
  Unavailable,
  Partial,
  Invalid,
  Complete,
}

internal data class BenchmarkMetricSide(
    val availability: BenchmarkMetricAvailability,
    val sampleCount: Int?,
    val validValues: List<Double>,
    val invalidSamples: List<IndexedValue<GoBenchmarkSample>>,
) {
  val median: Double?
    get() = benchmarkMedian(validValues)

  fun medianLabel(unit: String): String =
      median?.let { "${formatMetric(it)} $unit" } ?: "Unavailable ($unit)"

  fun availabilityLabel(): String {
    val coverage =
        when (availability) {
          BenchmarkMetricAvailability.MissingMeasurement -> "measurement not returned"
          BenchmarkMetricAvailability.EmptySamples -> "empty samples"
          BenchmarkMetricAvailability.Unavailable -> "unavailable"
          BenchmarkMetricAvailability.Partial -> "partial"
          BenchmarkMetricAvailability.Invalid -> "invalid samples"
          BenchmarkMetricAvailability.Complete -> "complete"
        }
    val count =
        sampleCount
            ?.let {
              "; ${validValues.size}/$it valid observations; $benchmarkRequiredSamples required"
            }
            .orEmpty()
    return "$coverage$count"
  }

  fun display(): String = "${median?.let(::formatMetric) ?: "unavailable"} (${availabilityLabel()})"
}

internal data class BenchmarkMetricRow(
    val label: String,
    val base: BenchmarkMetricSide,
    val candidate: BenchmarkMetricSide,
) {
  val isComplete: Boolean
    get() =
        base.availability == BenchmarkMetricAvailability.Complete &&
            candidate.availability == BenchmarkMetricAvailability.Complete

  val variability: Double
    get() =
        maxOf(
            relativeRange(base.validValues, base.median!!),
            relativeRange(candidate.validValues, candidate.median!!))

  fun changeLabel(historical: Boolean = false): String {
    val label = if (historical) "Historical change" else "Observed change"
    return if (isComplete) "$label: ${metricChangeLabel(base.median!!, candidate.median!!)}"
    else "$label unavailable: incomplete evidence"
  }

  fun display(historical: Boolean = false): String =
      "Baseline median: ${base.display()} · Candidate median: ${candidate.display()} · ${changeLabel(historical)}"

  fun invalidRows(): List<Pair<String, String>> = buildList {
    for ((side, evidence) in listOf("Baseline" to base, "Candidate" to candidate)) {
      evidence.invalidSamples.forEach { (index, sample) ->
        add(
            "$side invalid sample ${index + 1}" to
                "iterations=${sample.iterations}; ns/op=${sample.nanosecondsPerOperation}; B/op=${sample.bytesPerOperation ?: "unavailable"}; allocs/op=${sample.allocationsPerOperation ?: "unavailable"}")
      }
    }
  }
}

private fun GoBenchmarkComparison.identityOrNull(): GoBenchmarkComparisonIdentity? =
    GoBenchmarkComparisonIdentity(
            draftId,
            draftRevision,
            draftHash,
            projectId,
            projectRevision,
            baseFileHash,
            targetPath,
        )
        .takeIf {
          it.draftId.isNotBlank() &&
              it.draftRevision > 0 &&
              it.draftHash.isNotBlank() &&
              it.projectId.isNotBlank() &&
              it.projectRevision.isNotBlank() &&
              it.baseFileHash.isNotBlank() &&
              it.targetPath.isNotBlank()
        }

private fun benchmarkIdentityRows(comparison: GoBenchmarkComparison): List<Pair<String, String>> =
    buildList {
      comparison.benchmark.takeIf(String::isNotBlank)?.let { add("Workload" to it) }
      comparison.targetPath.takeIf(String::isNotBlank)?.let { add("Target" to it) }
      comparison.draftId.takeIf(String::isNotBlank)?.let {
        add("Candidate" to "${it} rev ${comparison.draftRevision} · ${comparison.draftHash}")
      }
      comparison.projectRevision.takeIf(String::isNotBlank)?.let { add("Base source" to it) }
      comparison.baseFileHash.takeIf(String::isNotBlank)?.let { add("Base file" to it) }
    }

private fun recordedBenchmarkFlag(command: List<String>, flag: String): String {
  val index = command.indexOfFirst { it == flag || it.startsWith("$flag=") }
  if (index < 0) return "not recorded"
  val argument = command[index]
  if (argument.startsWith("$flag=")) return argument.substringAfter('=').ifBlank { "not recorded" }
  val next = command.getOrNull(index + 1)?.takeUnless { it.startsWith('-') }
  return if (flag == "-benchmem") next?.takeIf { it == "true" || it == "false" } ?: "true"
  else next?.ifBlank { "not recorded" } ?: "not recorded"
}

internal fun performanceBenchmarkRecordedRows(
    comparison: GoBenchmarkComparison
): List<Pair<String, String>> {
  fun recorded(value: String) = value.ifBlank { "not recorded" }
  return listOf(
      "Benchmark" to recorded(comparison.benchmark),
      "Target path" to recorded(comparison.targetPath),
      "Project ID" to recorded(comparison.projectId),
      "Project revision" to recorded(comparison.projectRevision),
      "Draft ID" to recorded(comparison.draftId),
      "Draft revision" to
          comparison.draftRevision.takeIf { it > 0 }?.toString().orEmpty().let(::recorded),
      "Draft hash" to recorded(comparison.draftHash),
      "Base file hash" to recorded(comparison.baseFileHash),
      "Opaque scope guard" to recorded(comparison.scope),
      "Recorded -count" to recordedBenchmarkFlag(comparison.command, "-count"),
      "Recorded -benchtime" to recordedBenchmarkFlag(comparison.command, "-benchtime"),
      "Recorded -benchmem" to recordedBenchmarkFlag(comparison.command, "-benchmem"),
      "Recorded comparison argv (read-only)" to
          performanceBenchmarkArgv(comparison.command).ifEmpty { "not recorded" })
}

internal fun performanceBenchmarkSampleRows(
    comparison: GoBenchmarkComparison
): List<Pair<String, String>> = buildList {
  for ((side, measurement) in
      listOf("Baseline" to comparison.base, "Candidate" to comparison.candidate)) {
    add(
        "$side returned samples" to
            (measurement?.samples?.size?.toString() ?: "unavailable · measurement not returned"))
    measurement?.samples?.forEachIndexed { index, sample ->
      add(
          "$side sample ${index + 1}" to
              "iterations=${sample.iterations}; ns/op=${sample.nanosecondsPerOperation}; B/op=${sample.bytesPerOperation ?: "unavailable"}; allocs/op=${sample.allocationsPerOperation ?: "unavailable"}")
    }
  }
}

private fun performanceBenchmarkResponseRows(
    response: GoBenchmarkComparison
): List<Pair<String, String>> =
    listOf(
        "Daemon status" to response.status.ifBlank { "not recorded" },
        "Daemon reason" to response.reason.ifBlank { "not recorded" })

internal fun performanceBenchmarkResponseCopyText(
    response: GoBenchmarkComparison,
    conditionsExpanded: Boolean,
    samplesExpanded: Boolean,
): String =
    buildList {
          add("Latest comparison response")
          performanceBenchmarkResponseRows(response).forEach { (label, value) ->
            add("$label: $value")
          }
          if (conditionsExpanded) {
            add("Recorded conditions & identity")
            performanceBenchmarkRecordedRows(response).forEach { (label, value) ->
              add("$label: $value")
            }
          }
          if (samplesExpanded) {
            add("Returned sample details")
            performanceBenchmarkSampleRows(response).forEach { (label, value) ->
              add("$label: $value")
            }
          }
        }
        .joinToString("\n")

internal fun performanceBenchmarkCopyText(
    comparison: GoBenchmarkComparison,
    presentation: PerformanceBenchmarkPresentation,
    historical: Boolean,
    conditionsExpanded: Boolean,
    samplesExpanded: Boolean,
): String =
    buildList {
          add(if (historical) "Prior benchmark evidence" else "Benchmark evidence")
          add(presentation.stateLabel)
          add(presentation.summary)
          if (presentation.insights.isNotEmpty()) {
            add(if (historical) "Prior measured trade-offs" else "Measured trade-offs")
            addAll(presentation.insights)
          }
          add("Metric | Baseline median | Candidate median | Change / availability")
          presentation.metrics.firstOrNull()?.let {
            add("Baseline samples: ${it.base.sampleCount ?: "not returned"}")
            add("Candidate samples: ${it.candidate.sampleCount ?: "not returned"}")
          }
          presentation.metrics.forEach {
            val label =
                when (it.label) {
                  "ns/op" -> "Time (ns/op)"
                  "B/op" -> "Bytes (B/op)"
                  else -> "Allocations (allocs/op)"
                }
            add(
                "$label | ${it.base.medianLabel(it.label)} · ${it.base.availabilityLabel()} | ${it.candidate.medianLabel(it.label)} · ${it.candidate.availabilityLabel()} | ${it.changeLabel(historical)}")
          }
          presentation.rows
              .filter { row ->
                presentation.metrics.none { it.label == row.first } && row.first != "Samples"
              }
              .forEach { (label, value) -> add("$label: $value") }
          add(presentation.conditions)
          if (conditionsExpanded) {
            add("Recorded conditions & identity")
            performanceBenchmarkRecordedRows(comparison).forEach { (label, value) ->
              add("$label: $value")
            }
          }
          if (samplesExpanded) {
            add("Returned sample details")
            performanceBenchmarkSampleRows(comparison).forEach { (label, value) ->
              add("$label: $value")
            }
          }
        }
        .joinToString("\n")

private fun benchmarkConditions(comparison: GoBenchmarkComparison): String =
    listOf("-count", "-benchtime", "-benchmem").joinToString(" · ") {
      "Recorded $it: ${recordedBenchmarkFlag(comparison.command, it)}"
    }

private fun benchmarkTerminalLabel(comparison: GoBenchmarkComparison): String =
    when (comparison.status) {
      "unavailable" -> "Not measured · unavailable"
      "canceled" -> "Not measured · canceled"
      "failed" -> "Not measured · failed"
      else -> "Not measured · unsupported status"
    }

private fun benchmarkOutcomeLabel(outcome: BenchmarkComparisonOutcome): String =
    when (outcome.status) {
      BenchmarkComparisonStatus.Completed -> "Completed · no new measurements"
      BenchmarkComparisonStatus.Canceled -> "Comparison canceled · daemon"
      BenchmarkComparisonStatus.Failed -> "Comparison failed · daemon"
      BenchmarkComparisonStatus.Unavailable -> "Comparison unavailable · daemon"
      BenchmarkComparisonStatus.Unsupported -> "Comparison unsupported · daemon status"
    }

private fun benchmarkSampleCount(measurement: GoBenchmarkMeasurement?): String =
    measurement?.samples?.size?.toString() ?: "not returned"

private fun benchmarkMetricRows(comparison: GoBenchmarkComparison): List<BenchmarkMetricRow> {
  fun row(label: String, metric: (GoBenchmarkSample) -> Double?) =
      BenchmarkMetricRow(
          label,
          benchmarkMetricSide(comparison.base, metric),
          benchmarkMetricSide(comparison.candidate, metric))
  return listOf(
      row("ns/op") { it.nanosecondsPerOperation },
      row("B/op") { it.bytesPerOperation?.toDouble() },
      row("allocs/op") { it.allocationsPerOperation?.toDouble() })
}

private fun benchmarkMetricSide(
    measurement: GoBenchmarkMeasurement?,
    metric: (GoBenchmarkSample) -> Double?,
): BenchmarkMetricSide {
  if (measurement == null)
      return BenchmarkMetricSide(
          BenchmarkMetricAvailability.MissingMeasurement, null, emptyList(), emptyList())
  val validValues = mutableListOf<Double>()
  val invalidSamples = mutableListOf<IndexedValue<GoBenchmarkSample>>()
  measurement.samples.withIndex().forEach { observation ->
    val sample = observation.value
    val value = metric(sample)
    if (sample.iterations <= 0 ||
        !sample.nanosecondsPerOperation.isFinite() ||
        sample.nanosecondsPerOperation <= 0 ||
        (value != null && (!value.isFinite() || value < 0)))
        invalidSamples.add(observation)
    else if (value != null) validValues.add(value)
  }
  val availability =
      when {
        measurement.samples.isEmpty() -> BenchmarkMetricAvailability.EmptySamples
        invalidSamples.isNotEmpty() -> BenchmarkMetricAvailability.Invalid
        validValues.isEmpty() -> BenchmarkMetricAvailability.Unavailable
        measurement.samples.size != benchmarkRequiredSamples ||
            validValues.size != benchmarkRequiredSamples -> BenchmarkMetricAvailability.Partial
        else -> BenchmarkMetricAvailability.Complete
      }
  return BenchmarkMetricSide(availability, measurement.samples.size, validValues, invalidSamples)
}

private fun benchmarkMedian(values: List<Double>): Double? {
  if (values.isEmpty()) return null
  val sorted = values.sorted()
  val middle = sorted.size / 2
  return if (sorted.size % 2 == 1) sorted[middle]
  else sorted[middle - 1] + (sorted[middle] - sorted[middle - 1]) / 2
}

private fun relativeRange(values: List<Double>, median: Double): Double =
    when {
      median > 0 -> (values.max() - values.min()) / median
      values.all { it == 0.0 } -> 0.0
      else -> Double.POSITIVE_INFINITY
    }

private fun variabilityLabel(variableMetrics: List<BenchmarkMetricRow>): String =
    if (variableMetrics.isEmpty()) "within 10% range"
    else "high: ${variableMetrics.joinToString { it.label }}"

private fun formatMetric(value: Double): String =
    if (value == value.toLong().toDouble()) value.toLong().toString()
    else String.format(Locale.ROOT, "%.2f", value)

private fun metricChangeLabel(base: Double, candidate: Double): String =
    when {
      base == 0.0 && candidate == 0.0 -> "no change from zero"
      base == 0.0 -> "from zero to ${formatMetric(candidate)}"
      else -> formatPercent((candidate - base) / base)
    }

private fun formatPercent(value: Double): String =
    String.format(Locale.ROOT, "%+.1f%%", value * 100)

private fun performanceImpactOrder(value: String): Int =
    when (value) {
      "high" -> 4
      "medium" -> 3
      "low" -> 2
      else -> 1
    }

private fun performanceConfidenceOrder(value: String): Int =
    when (value) {
      "high" -> 3
      "medium" -> 2
      else -> 1
    }
