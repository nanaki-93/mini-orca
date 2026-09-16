package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.util.Locale

@Composable
internal fun PerformanceWorkspacePane(
    state: PerformanceWorkspacePaneState,
    actions: PerformanceWorkspaceActions
) {
  var benchmarksExpanded by remember { mutableStateOf(false) }
  var measurementDetailsExpanded by remember { mutableStateOf(false) }
  val results = performanceResults(state.page)
  val semantic = state.page.semantic
  val benchmarkStatus =
      performanceBenchmarkStatusPresentation(
          state.benchmarkComparison,
          state.expectedBenchmarkIdentity,
          state.selectedBenchmark,
          state.benchmarkRunning)
  AnalysisResultsPane(
      page = state.page,
      rows = results.map { it.row() } + semantic.map(::semanticResultRow),
      browser = state.browser,
      facetLabel = "Impact",
      openAnalysis = actions.openAnalysis,
      tools = {
        IdeDisclosureHeader(
            "Benchmark evidence",
            benchmarksExpanded,
            { benchmarksExpanded = !benchmarksExpanded },
            stateLabel = benchmarkStatus.stateLabel)
        if (benchmarksExpanded)
            Column(
                Modifier.fillMaxWidth()
                    .heightIn(max = 260.dp)
                    .verticalScroll(rememberScrollState())) {
                  Text(
                      benchmarkStatus.summary,
                      color = SecondaryText,
                      style = IdeTypography.compactBody,
                      modifier = Modifier.padding(top = 4.dp))
                  PerformanceBenchmarkControls(
                      state.benchmarkCatalog,
                      state.selectedBenchmark,
                      state.expectedBenchmarkIdentity != null,
                      state.benchmarkRunning,
                      actions)
                  state.benchmarkComparison?.let { comparison ->
                    IdeDisclosureHeader(
                        "Measurement details",
                        measurementDetailsExpanded,
                        { measurementDetailsExpanded = !measurementDetailsExpanded },
                        stateLabel = benchmarkStatus.stateLabel)
                    if (measurementDetailsExpanded)
                        PerformanceBenchmarkEvidence(
                            comparison, state.expectedBenchmarkIdentity, state.selectedBenchmark)
                  }
                }
      }) { key ->
        val result = results.firstOrNull { it.row().key == key }
        if (result != null) PerformanceFindingDetails(result, state.index, actions)
        else
            semantic
                .firstOrNull { semanticResultRow(it).key == key }
                ?.let { FindingDetailsRegion(it, actions.semanticActions) }
      }
}

internal data class PerformanceResult(
    val report: PerformanceFileReport,
    val finding: PerformanceFinding,
    val stale: Boolean
) {
  fun row() =
      ResultRowPresentation(
          "performance:${report.path}:${finding.id}",
          finding.title.ifBlank { "Untitled opportunity" },
          "${report.path}:${finding.startLine}",
          finding.observedPattern,
          finding.potentialImpact.ifBlank { "Unknown impact" },
          "Model suggestion",
          if (stale) "Stale" else "")
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
            PerformanceResult(report, it, page.stale || report.status == "stale")
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

internal fun performanceCanPrepare(result: PerformanceResult, index: ProjectIndex?): Boolean {
  if (index == null ||
      result.stale ||
      result.report.projectId != index.projectId ||
      result.report.projectRevision != index.projectRevision ||
      result.report.status !in setOf("completed", "partial"))
      return false
  val file =
      index.files.firstOrNull {
        it.path == result.report.path && it.contentHash == result.report.contentHash
      } ?: return false
  val declaration =
      file.symbols.singleOrNull {
        it.name == result.finding.symbol && it.atomicTarget && it.confidence == "exact"
      } ?: return false
  return file.language == "Go" &&
      result.finding.startLine in declaration.startLine..declaration.endLine
}

@Composable
private fun PerformanceFindingDetails(
    result: PerformanceResult,
    index: ProjectIndex?,
    actions: PerformanceWorkspaceActions
) {
  val finding = result.finding
  var technical by remember(result.row().key) { mutableStateOf(false) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
    ResultDetailHeader(result.row())
    Text(
        "Unmeasured recommendation. Benchmark the affected workload before claiming an improvement.",
        color = SecondaryText,
        style = IdeTypography.compactBody)
    ResultEvidenceSection("Observed pattern", finding.observedPattern)
    ResultEvidenceSection("Recommendation", finding.recommendation)
    ResponsiveActionGroup(Modifier.fillMaxWidth()) {
      MiniOrcaButton(
          onClick = { actions.prepareOptimization(result.report.path, finding) },
          enabled = performanceCanPrepare(result, index),
          tone = ActionTone.Primary) {
            Text("Prepare fix")
          }
    }
    if (!performanceCanPrepare(result, index))
        Text(
            if (result.stale) "Analyze again to prepare a fix from current source."
            else "Fix preparation requires a matching indexed Go declaration.",
            color = SecondaryText,
            style = IdeTypography.compactBody)
    IdeDisclosureHeader(
        "Workload, trade-offs and verification", technical, { technical = !technical })
    if (technical) {
      Text(
          "Potential improvement; benchmark the affected workload to verify its impact.",
          color = SecondaryText,
          style = IdeTypography.compactBody)
      ResultEvidenceSection("When it matters", finding.workloadConditions)
      ResultEvidenceSection("Trade-offs", finding.tradeoff)
      ResultEvidenceSection("Verification plan", finding.verificationPlan)
      Text(
          "${result.report.profile} · ${result.report.model} · ${result.report.providerOrigin}",
          color = SecondaryText,
          style = IdeTypography.compactBody)
      if (result.report.warning.isNotBlank()) ModelResultContent(result.report.warning)
      EngineeringInsightPanel(
          finding.engineeringInsight,
          stale = result.stale,
          scopeLabel = "Selected performance opportunity")
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
) {
  val presentation = performanceBenchmarkPresentation(comparison, expectedIdentity, expectedChoice)
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = "Benchmark evidence",
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
              "Measured trade-offs",
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
)

/**
 * Keeps the benchmark status visible even before an explicit local comparison exists. A benchmark
 * comparison applies to its candidate identity and never validates a separate model suggestion.
 */
internal fun performanceBenchmarkStatusPresentation(
    comparison: GoBenchmarkComparison?,
    expectedIdentity: GoBenchmarkComparisonIdentity?,
    expectedChoice: GoBenchmarkChoice?,
    running: Boolean,
): PerformanceBenchmarkStatusPresentation =
    when {
      running ->
          PerformanceBenchmarkStatusPresentation(
              "Running · explicit local execution",
              "The selected benchmark is running in isolated copies for the current candidate.")
      comparison != null ->
          performanceBenchmarkPresentation(comparison, expectedIdentity, expectedChoice).let {
            PerformanceBenchmarkStatusPresentation(
                it.stateLabel,
                "Benchmark evidence is candidate-specific and does not measure this model suggestion.")
          }
      else ->
          PerformanceBenchmarkStatusPresentation(
              "Not measured · explicit local execution",
              "No benchmark evidence is available for the current candidate. Listing is read-only; running a benchmark requires explicit local execution.")
    }

/** Listing a catalog is read-only; only the explicitly labeled run action can execute code. */
@Composable
private fun PerformanceBenchmarkControls(
    catalog: GoBenchmarkCatalog?,
    selected: GoBenchmarkChoice?,
    candidateAvailable: Boolean,
    running: Boolean,
    actions: PerformanceWorkspaceActions,
) {
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = "Benchmark comparison",
        icon = DesktopIcon.Performance,
        stateLabel =
            when {
              running -> "Running in isolated copies"
              catalog == null && candidateAvailable -> "List existing benchmarks"
              catalog == null -> "Validate a current candidate first"
              !catalog.available -> "Not available"
              selected == null -> "Select one benchmark"
              catalog.trusted -> "Ready to run"
              else -> "Local execution needs trust"
            },
        stateTint =
            if (catalog?.available == false || !candidateAvailable) Warning else SecondaryText,
    )
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      if (catalog == null) {
        Text(
            "Listing compatible benchmarks is read-only and does not execute project code.",
            color = SecondaryText,
            fontSize = 11.sp,
            lineHeight = 16.sp)
        MiniOrcaButton(
            onClick = actions.loadBenchmarks,
            enabled = candidateAvailable && !running,
            tone = ActionTone.Neutral,
            modifier = Modifier.padding(top = 6.dp)) {
              Text("List compatible benchmarks", fontSize = 11.sp)
            }
        return@Column
      }
      if (!catalog.available) {
        Text(
            catalog.reason.ifBlank { "No compatible benchmark is available for this candidate." },
            color = Warning,
            fontSize = 11.sp,
            lineHeight = 16.sp)
        return@Column
      }
      Text(
          "Select one existing benchmark. The daemon-built argv below is the only command this action can run.",
          color = SecondaryText,
          fontSize = 11.sp,
          lineHeight = 16.sp)
      catalog.benchmarks.forEach { choice ->
        ChromeButton(
            onClick = { actions.selectBenchmark(choice) },
            selected = choice == selected,
            modifier = Modifier.fillMaxWidth().padding(top = 4.dp)) {
              Text(choice.name, fontSize = 11.sp, modifier = Modifier.weight(1f))
            }
      }
      selected?.let { choice ->
        SelectionContainer {
          Text(
              choice.command.joinToString(" "),
              color = SecondaryText,
              fontSize = 10.sp,
              lineHeight = 16.sp,
              modifier = Modifier.padding(top = 6.dp))
        }
        MiniOrcaButton(
            onClick = actions.runBenchmark,
            enabled = candidateAvailable && !running,
            tone = ActionTone.Primary,
            modifier = Modifier.padding(top = 6.dp)) {
              Text(
                  if (running) "Comparing benchmark…"
                  else if (catalog.trusted) "Run selected benchmark"
                  else "Trust and run selected benchmark",
                  fontSize = 11.sp)
            }
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
    val benchmarkRunning: Boolean = false,
    val browser: ResultBrowserState = newResultBrowserState(page),
)

internal data class PerformanceWorkspaceActions(
    val prepareOptimization: (String, PerformanceFinding) -> Unit,
    val openAnalysis: () -> Unit,
    val semanticActions: FindingActions,
    val loadBenchmarks: () -> Unit = {},
    val selectBenchmark: (GoBenchmarkChoice) -> Unit = {},
    val runBenchmark: () -> Unit = {},
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
      else -> "Not measured · missing"
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
