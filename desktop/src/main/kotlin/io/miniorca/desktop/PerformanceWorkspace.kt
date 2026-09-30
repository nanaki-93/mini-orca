package io.miniorca.desktop

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
  val benchmarkStatus =
      performanceBenchmarkStatusPresentation(
          state.benchmarkComparison,
          state.expectedBenchmarkIdentity,
          state.selectedBenchmark,
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
        IdeDisclosureHeader(
            "Explore benchmark evidence",
            benchmarksExpanded,
            { benchmarksExpanded = !benchmarksExpanded },
            stateLabel = benchmarkStatus.stateLabel)
        // Use the bounded result overview's scroll owner so long catalogs and required admission
        // text do not compete with a second fixed-height viewport.
        if (benchmarksExpanded)
            Column(Modifier.fillMaxWidth().testTag("benchmark-discovery-content")) {
              SelectionContainer {
                Text(
                    benchmarkStatus.summary,
                    color = SecondaryText,
                    style = IdeTypography.compactBody,
                    modifier = Modifier.padding(top = 4.dp))
              }
              PerformanceBenchmarkControls(
                  state.benchmarkCatalog,
                  state.selectedBenchmark,
                  state.benchmarkEligibility,
                  state.benchmarkDiscovery,
                  state.benchmarkAdmission,
                  actions)
              state.benchmarkComparison?.let { comparison ->
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
                        state.selectedBenchmark,
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
    Text(
        "Unmeasured recommendation. Benchmark the affected workload before claiming an improvement.",
        color = SecondaryText,
        style = IdeTypography.compactBody)
    PerformanceField("Report status", result.report.status.ifBlank { "Not supplied." })
    PerformanceField(
        "Freshness", if (result.stale) "Stale · saved evidence" else "Current for this analysis")
    PerformanceField("Observed pattern", finding.observedPattern)
    PerformanceField("Potential impact · qualitative, not a measured gain", finding.potentialImpact)
    PerformanceField(
        "Model confidence · not a measurement or speedup probability", finding.confidence)
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
            if (result.stale) "Saved evidence may not match current source. Analyze again."
            else "Partial report; some evidence may be missing.",
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
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = if (priorEvidence) "Prior benchmark evidence" else "Benchmark evidence",
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
        CompactKeyValueRows(presentation.rows)
        Text(
            presentation.conditions,
            color = SecondaryText,
            fontSize = 11.sp,
            lineHeight = 16.sp,
            modifier = Modifier.padding(top = 6.dp))
        Text(
            presentation.summary,
            color = if (presentation.inconclusive || presentation.isStale) Warning else PrimaryText,
            fontSize = 12.sp,
            lineHeight = 18.sp,
            modifier = Modifier.padding(top = 6.dp))
        if (presentation.insights.isNotEmpty()) {
          Text(
              if (priorEvidence) "Prior measured trade-offs" else "Measured trade-offs",
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
      }
    }
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
        current(
            "Admitting · execution trust", "Checking execution trust for the selected benchmark.")
    admission == BenchmarkAdmissionOutcome.Running ->
        current(
            "Running · explicit local execution",
            "The selected benchmark is running in isolated copies for the current candidate.")
    discovery == BenchmarkDiscoveryOutcome.Loading ->
        current(
            "Listing · read-only discovery",
            "Looking up compatible benchmarks. No project code is executed by discovery.")
    admission is BenchmarkAdmissionOutcome.Failed ->
        current("Benchmark admission failed", admission.message)
    admission == BenchmarkAdmissionOutcome.Stopped ->
        current(
            "Benchmark admission stopped",
            "The local benchmark operation stopped. Prior evidence does not confirm this operation completed.")
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
            discovery.reason.ifBlank {
              "No compatible benchmark is available for this candidate. Refresh the catalog to check again."
            })
    discovery == BenchmarkDiscoveryOutcome.Invalidated ->
        current(
            "Discovery invalidated",
            "Benchmark catalog and selection are no longer current. Validate the candidate if needed, then refresh compatible benchmarks.")
    discovery == BenchmarkDiscoveryOutcome.Loaded && catalog?.benchmarks?.isEmpty() == true ->
        current(
            "No compatible benchmarks",
            "No compatible benchmarks were found for this candidate. Refresh to check again; no benchmark is selected.")
    discovery == BenchmarkDiscoveryOutcome.Loaded && eligibility?.canCompare == false ->
        current("Comparison blocked", eligibility.comparisonBlockedReason!!)
    comparison != null ->
        performanceBenchmarkPresentation(comparison, expectedIdentity, expectedChoice).let {
          PerformanceBenchmarkStatusPresentation(
              it.stateLabel,
              "Benchmark evidence is candidate-specific and does not measure this model suggestion.",
              priorEvidence = it.isStale)
        }
    else ->
        PerformanceBenchmarkStatusPresentation(
            "Not measured · explicit local execution",
            "No benchmark evidence is available for the current candidate. Listing is read-only; running a benchmark requires explicit local execution.")
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
          "Listing compatible benchmarks is read-only and does not execute project code. Refresh clears the selection; select again after reviewing the returned catalog.",
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
        Text(
            "No compatible benchmarks were found for this candidate. Refresh to check again; no benchmark is selected.",
            color = Warning,
            style = IdeTypography.compactBody)
        return@Column
      }
      eligibility.comparisonBlockedReason?.let {
        SelectionContainer { Text(it, color = Warning, fontSize = 11.sp, lineHeight = 16.sp) }
      }
      Text(
          "Select one existing benchmark. The daemon-built argv below is the only command this action can run.",
          color = SecondaryText,
          fontSize = 11.sp,
          lineHeight = 16.sp)
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
                "The working directory is relative to the project root; . means project root. The scope guard is opaque identity metadata, not a directory.",
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
            Text(
                "Each JSON-quoted entry is one argument, not a shell command. No shell parsing or editable command is used.",
                color = SecondaryText,
                style = IdeTypography.compactBody)
            listOf(
                    "Running benchmarks executes imported project code.",
                    "Execution may have external effects, including file and network access.",
                    "Baseline and candidate use copied workspaces; these are not a security sandbox.")
                .forEach { Text(it, color = Warning, style = IdeTypography.compactBody) }
            Text(
                if (trusted)
                    "The catalog reports session execution trust for this project/revision."
                else
                    "Trust and run grants session execution trust for this project/revision, then runs the selected benchmark.",
                color = SecondaryText,
                style = IdeTypography.compactBody)
            listOf(
                    "Trust contract (separate from selected argv): go test ./...",
                    "This is broader than benchmark-only permission.",
                    "Trust lasts for this project revision in the daemon session.",
                    "Granting trust does not execute “go test ./...”.",
                    "The combined action requests the selected benchmark separately.")
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

internal fun benchmarkEvidenceIdentity(
    review: DraftReviewState,
): GoBenchmarkComparisonIdentity? =
    review.draft
        ?.takeIf {
          review.editor?.status == DraftEditorStatus.Valid && it.validation?.applicable == true
        }
        ?.let(::goBenchmarkComparisonIdentity)

internal data class PerformanceBenchmarkPresentation(
    val stateLabel: String,
    val rows: List<Pair<String, String>>,
    val conditions: String,
    val summary: String,
    val insights: List<String>,
    val isMeasured: Boolean,
    val isStale: Boolean,
    val inconclusive: Boolean,
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
  val assessment = benchmarkMeasurementPresentation(comparison, expectedIdentity, expectedChoice)
  return if (priorEvidence)
      assessment.copy(
          stateLabel = "Prior evidence · ${assessment.stateLabel}",
          summary =
              "Prior comparison only; it does not confirm the latest attempt. Recorded assessment: ${assessment.summary}",
          insights = assessment.insights.map { "Prior observation: $it" },
          isMeasured = false)
  else assessment
}

private fun benchmarkMeasurementPresentation(
    comparison: GoBenchmarkComparison,
    expectedIdentity: GoBenchmarkComparisonIdentity?,
    expectedChoice: GoBenchmarkChoice?,
): PerformanceBenchmarkPresentation {
  val identity = comparison.identityOrNull()
  val rows = benchmarkIdentityRows(comparison)
  val conditions = benchmarkConditions(comparison)
  if (expectedIdentity == null)
      return PerformanceBenchmarkPresentation(
          stateLabel = "Stale · no current candidate",
          rows = rows,
          conditions = conditions,
          summary = "This comparison is retained, but no current draft identity can verify it.",
          insights = emptyList(),
          isMeasured = false,
          isStale = true,
          inconclusive = true,
      )
  if (identity != expectedIdentity)
      return PerformanceBenchmarkPresentation(
          stateLabel = "Stale · candidate identity changed",
          rows = rows,
          conditions = conditions,
          summary =
              "This comparison is for a different draft or source revision and is not usable.",
          insights = emptyList(),
          isMeasured = false,
          isStale = true,
          inconclusive = true,
      )
  if (expectedChoice != null &&
      (comparison.benchmark != expectedChoice.name || comparison.scope != expectedChoice.scope))
      return PerformanceBenchmarkPresentation(
          stateLabel = "Stale · selected benchmark changed",
          rows = rows,
          conditions = conditions,
          summary = "This comparison is for a different benchmark selection and is not usable.",
          insights = emptyList(),
          isMeasured = false,
          isStale = true,
          inconclusive = true,
      )
  if (comparison.status != "completed")
      return PerformanceBenchmarkPresentation(
          stateLabel = benchmarkTerminalLabel(comparison),
          rows = rows,
          conditions = conditions,
          summary = comparison.reason.ifBlank { "No benchmark measurements are available." },
          insights = emptyList(),
          isMeasured = false,
          isStale = false,
          inconclusive = comparison.status != "unavailable",
      )

  val base = comparison.base?.samples.orEmpty()
  val candidate = comparison.candidate?.samples.orEmpty()
  if (comparison.benchmark.isBlank() ||
      !validBenchmarkSamples(base) ||
      !validBenchmarkSamples(candidate))
      return PerformanceBenchmarkPresentation(
          stateLabel = "Inconclusive · incomplete measurement evidence",
          rows = rows + ("Samples" to "${base.size} base · ${candidate.size} candidate"),
          conditions = conditions,
          summary = "The selected benchmark did not return a complete comparable measurement.",
          insights = emptyList(),
          isMeasured = false,
          isStale = false,
          inconclusive = true,
      )

  val optionalMetrics = benchmarkOptionalMetricCoverage(base, candidate)
  val incompleteMemoryMetrics = optionalMetrics.filter { it.isIncomplete }
  if (incompleteMemoryMetrics.isNotEmpty())
      return PerformanceBenchmarkPresentation(
          stateLabel = "Inconclusive · incomplete memory evidence",
          rows =
              rows +
                  ("Samples" to "${base.size} base · ${candidate.size} candidate") +
                  listOf(
                          benchmarkMetricRow(
                              "ns/op",
                              base.map { it.nanosecondsPerOperation },
                              candidate.map { it.nanosecondsPerOperation }))
                      .map { it.label to it.display() } +
                  optionalMetrics.map { it.presentationRow() },
          conditions = conditions,
          summary =
              "Inconclusive: ${incompleteMemoryMetrics.joinToString { it.label }} is missing " +
                  "from some samples or one side of the comparison.",
          insights =
              listOf(
                  "CPU measurements cannot establish a performance win until memory evidence is complete."),
          isMeasured = false,
          isStale = false,
          inconclusive = true,
      )

  val metrics = benchmarkMetricRows(base, candidate, optionalMetrics)
  val variableMetrics = metrics.filter { it.variability > benchmarkVariabilityLimit }
  val cpu = metrics.first { it.label == "ns/op" }
  val memoryRegressions = metrics.filter { it.label != "ns/op" && it.candidate > it.base }
  val memoryImprovements = metrics.filter { it.label != "ns/op" && it.candidate < it.base }
  val opposingMemorySignals = memoryRegressions.isNotEmpty() && memoryImprovements.isNotEmpty()
  val cpuImproved = cpu.candidate < cpu.base
  val insights = buildList {
    if (opposingMemorySignals)
        add(
            "Memory metrics disagree; CPU measurements cannot establish a performance win until the trade-off is understood.")
    else if (cpuImproved && memoryRegressions.isNotEmpty())
        add(
            "CPU median is lower, while ${memoryRegressions.joinToString { it.label }} increased; " +
                "this is a trade-off, not an unconditional win.")
    else if (cpuImproved)
        add(
            "CPU median is lower for ${comparison.benchmark}; this does not establish project-wide performance.")
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
        inconclusive ->
            "Inconclusive: ${variableMetrics.joinToString { it.label }} is too variable across the selected samples."
        cpuImproved && memoryRegressions.isNotEmpty() ->
            "CPU is lower for the selected benchmark, with higher memory use."
        cpuImproved -> "CPU is lower for the selected benchmark."
        else -> "The candidate did not lower CPU median for the selected benchmark."
      }
  return PerformanceBenchmarkPresentation(
      stateLabel =
          when {
            opposingMemorySignals -> "Inconclusive · opposing memory signals"
            inconclusive -> "Inconclusive · noisy samples"
            else -> "Measured · selected benchmark"
          },
      rows =
          rows +
              ("Samples" to "${base.size} base · ${candidate.size} candidate") +
              metrics.map { it.label to it.display() } +
              ("Variability" to variabilityLabel(variableMetrics)),
      conditions = conditions,
      summary = summary,
      insights = insights,
      isMeasured = !inconclusive,
      isStale = false,
      inconclusive = inconclusive,
  )
}

private const val benchmarkVariabilityLimit = 0.10

private data class BenchmarkMetricRow(
    val label: String,
    val base: Double,
    val candidate: Double,
    val variability: Double,
) {
  fun display(): String =
      "${formatMetric(base)} → ${formatMetric(candidate)} · ${metricChangeLabel(base, candidate)}"
}

private data class OptionalBenchmarkMetricCoverage(
    val label: String,
    val base: List<Double>,
    val candidate: List<Double>,
    val baseSampleCount: Int,
    val candidateSampleCount: Int,
) {
  val isComplete: Boolean
    get() = base.size == baseSampleCount && candidate.size == candidateSampleCount

  val isIncomplete: Boolean
    get() = !isComplete

  fun incompleteCoverageLabel(): String =
      "incomplete: ${base.size} base · ${candidate.size} candidate"

  fun presentationRow(): Pair<String, String> =
      if (isComplete) benchmarkMetricRow(label, base, candidate).let { it.label to it.display() }
      else label to incompleteCoverageLabel()
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

private fun benchmarkConditions(comparison: GoBenchmarkComparison): String {
  val duration =
      comparison.command
          .zipWithNext()
          .firstOrNull { (argument, _) -> argument == "-benchtime" }
          ?.second
          ?: comparison.command
              .firstOrNull { it.startsWith("-benchtime=") }
              ?.removePrefix("-benchtime=")
  val conditions = buildList {
    duration?.let { add("Target duration per sample: $it") }
    if (comparison.command.contains("-benchmem")) add("benchmem enabled")
    if (comparison.command.isNotEmpty()) add("fixed argv")
  }
  return if (conditions.isEmpty()) "Test conditions were not recorded."
  else conditions.joinToString(" · ")
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

private fun validBenchmarkSamples(samples: List<GoBenchmarkSample>): Boolean =
    samples.size == 5 &&
        samples.all {
          it.iterations > 0 &&
              it.nanosecondsPerOperation.isFinite() &&
              it.nanosecondsPerOperation > 0 &&
              (it.bytesPerOperation == null || it.bytesPerOperation >= 0) &&
              (it.allocationsPerOperation == null || it.allocationsPerOperation >= 0)
        }

private fun benchmarkMetricRows(
    base: List<GoBenchmarkSample>,
    candidate: List<GoBenchmarkSample>,
    optionalMetrics: List<OptionalBenchmarkMetricCoverage>,
): List<BenchmarkMetricRow> = buildList {
  add(
      benchmarkMetricRow(
          "ns/op",
          base.map { it.nanosecondsPerOperation },
          candidate.map { it.nanosecondsPerOperation }))
  optionalMetrics
      .filter { !it.isIncomplete && it.base.isNotEmpty() && it.candidate.isNotEmpty() }
      .forEach { add(benchmarkMetricRow(it.label, it.base, it.candidate)) }
}

private fun benchmarkOptionalMetricCoverage(
    base: List<GoBenchmarkSample>,
    candidate: List<GoBenchmarkSample>,
): List<OptionalBenchmarkMetricCoverage> =
    listOf(
        optionalBenchmarkMetricCoverage("B/op", base, candidate) { it.bytesPerOperation },
        optionalBenchmarkMetricCoverage("allocs/op", base, candidate) {
          it.allocationsPerOperation
        },
    )

private fun optionalBenchmarkMetricCoverage(
    label: String,
    base: List<GoBenchmarkSample>,
    candidate: List<GoBenchmarkSample>,
    metric: (GoBenchmarkSample) -> Long?,
): OptionalBenchmarkMetricCoverage =
    OptionalBenchmarkMetricCoverage(
        label = label,
        base = base.mapNotNull(metric).map(Long::toDouble),
        candidate = candidate.mapNotNull(metric).map(Long::toDouble),
        baseSampleCount = base.size,
        candidateSampleCount = candidate.size,
    )

private fun benchmarkMetricRow(
    label: String,
    base: List<Double>,
    candidate: List<Double>,
): BenchmarkMetricRow {
  val baseMedian = base.sorted()[base.size / 2]
  val candidateMedian = candidate.sorted()[candidate.size / 2]
  return BenchmarkMetricRow(
      label,
      baseMedian,
      candidateMedian,
      maxOf(relativeRange(base, baseMedian), relativeRange(candidate, candidateMedian)),
  )
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
