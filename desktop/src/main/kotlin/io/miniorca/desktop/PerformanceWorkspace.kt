package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.selection.SelectionContainer
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
  var category by remember { mutableStateOf("") }
  var impact by remember { mutableStateOf("") }
  var path by remember { mutableStateOf("") }
  var selectedID by remember { mutableStateOf("") }
  var reviewOptionsExpanded by remember { mutableStateOf(true) }
  var filtersExpanded by remember { mutableStateOf(true) }
  val job = state.job
  val presentation = performanceReviewPresentation(job, state.report)
  val report = presentation.report
  val toolbarActions =
      performanceToolbarActions(
          job = job,
          hasPreviewContext = state.context != null,
          model = state.model,
          remoteProviderConfirmed = state.remoteProviderConfirmed,
      )
  val findings = presentation.findings(category, impact, path)
  val selected = findings.firstOrNull { it.id == selectedID }
  fun requestToolbarAction(action: PerformanceToolbarAction) {
    when (action) {
      PerformanceToolbarAction.Preview -> actions.preview()
      PerformanceToolbarAction.Start ->
          state.context?.let { actions.start(it, state.remoteProviderConfirmed) }
      PerformanceToolbarAction.Pause -> actions.pause()
      PerformanceToolbarAction.Resume -> actions.resume(state.remoteProviderConfirmed)
      PerformanceToolbarAction.Cancel -> actions.cancel()
    }
  }
  BoxWithConstraints(Modifier.fillMaxSize()) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = workspacePagePadding(maxWidth, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
      item { PerformanceReviewHeader(job, report, toolbarActions, ::requestToolbarAction) }
      item {
        PerformanceReviewScope(
            state = state,
            report = report,
            expanded = reviewOptionsExpanded,
            onToggle = { reviewOptionsExpanded = !reviewOptionsExpanded },
            onRemoteProviderConfirmed = actions.confirmRemoteProvider,
        )
      }
      if (state.expectedBenchmarkIdentity != null || state.benchmarkCatalog != null) {
        item {
          PerformanceBenchmarkControls(
              catalog = state.benchmarkCatalog,
              selected = state.selectedBenchmark,
              candidateAvailable = state.expectedBenchmarkIdentity != null,
              running = state.benchmarkRunning,
              actions = actions,
          )
        }
      }
      state.benchmarkComparison?.let { comparison ->
        item {
          PerformanceBenchmarkEvidence(
              comparison = comparison,
              expectedIdentity = state.expectedBenchmarkIdentity,
              expectedChoice = state.selectedBenchmark,
          )
        }
      }
      item {
        IdeDisclosureHeader(
            title = "Filters",
            expanded = filtersExpanded,
            onToggle = { filtersExpanded = !filtersExpanded },
            stateLabel = "Filter opportunities",
        )
        if (filtersExpanded) {
          Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
            ResponsiveFieldPair(
                modifier = Modifier.fillMaxWidth(),
                first = { modifier ->
                  CompactSingleLineField(category, { category = it }, "Category", modifier)
                },
                second = { modifier ->
                  CompactSingleLineField(impact, { impact = it }, "Potential impact", modifier)
                })
            CompactSingleLineField(
                path, { path = it }, "Path", Modifier.fillMaxWidth().padding(top = 6.dp))
          }
        }
      }
      selected?.let { finding ->
        item {
          PerformanceFindingDetails(
              finding, presentation.pathFor(finding), presentation.isStale, actions)
        }
      }
      item {
        SectionLabel("Opportunities")
        IdeHorizontalSeparator(Modifier.padding(top = 4.dp))
      }
      if (!presentation.hasReport)
          item {
            SystemStateMessage(
                "No performance review",
                "Preview limits, then start a bounded source review.",
                modifier = Modifier.fillMaxWidth())
          }
      else if (findings.isEmpty())
          item {
            SystemStateMessage(
                "No opportunities in reviewed files",
                "Coverage and skipped files are shown above.",
                modifier = Modifier.fillMaxWidth())
          }
      else
          items(findings, key = { it.id }) { finding ->
            ChromeButton(
                onClick = { selectedID = finding.id },
                selected = finding.id == selectedID,
                modifier = Modifier.fillMaxWidth()) {
                  Text(
                      "${finding.category.uppercase()} · ${finding.potentialImpact} · ${presentation.pathFor(finding)}:${finding.startLine} · ${finding.title}",
                      fontSize = 11.sp,
                      modifier = Modifier.weight(1f))
                }
          }
    }
  }
}

@Composable
private fun PerformanceReviewHeader(
    job: PerformanceJob?,
    report: PerformanceReport?,
    toolbarActions: List<PerformanceToolbarActionPresentation>,
    onToolbarAction: (PerformanceToolbarAction) -> Unit,
) {
  IdePaneHeader(
      title = "Performance",
      icon = DesktopIcon.Performance,
      stateLabel = performanceStatusLabel(job, report),
      stateTint = performanceStatusTint(job, report),
      actions = {
        toolbarActions.forEach { toolbarAction ->
          MiniOrcaButton(
              onClick = { onToolbarAction(toolbarAction.action) },
              enabled = toolbarAction.enabled,
              tone = performanceToolbarActionTone(toolbarAction.action),
              density = ButtonDensity.Toolbar) {
                Text(performanceToolbarActionLabel(toolbarAction.action), fontSize = 11.sp)
              }
        }
      },
  )
  Text(
      "Source-based review · Not measured",
      color = Warning,
      fontSize = 11.sp,
      modifier = Modifier.padding(start = 8.dp, top = 4.dp, end = 8.dp))
}

@Composable
private fun PerformanceReviewScope(
    state: PerformanceWorkspacePaneState,
    report: PerformanceReport?,
    expanded: Boolean,
    onToggle: () -> Unit,
    onRemoteProviderConfirmed: (Boolean) -> Unit,
) {
  val context = state.context
  val coverage = performanceCoveragePresentation(state.job, report)
  IdeDisclosureHeader(
      title = "Review scope",
      expanded = expanded,
      onToggle = onToggle,
      stateLabel =
          context?.let { "${it.files.size} selected files" } ?: "Preview limits to inspect scope",
      stateTint = if (context == null) SecondaryText else SelectionText,
  )
  if (expanded) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      context?.let { preview ->
        CompactKeyValueRows(
            listOf(
                "Selected" to "${preview.files.size} files",
                "Eligible limits" to
                    "${preview.excluded} excluded · ${preview.oversized} oversized · ${preview.outsideLimit} outside limit",
            ))
      }
          ?: Text(
              "No previewed scope is available yet. Preview limits before starting a review.",
              color = SecondaryText,
              fontSize = 11.sp,
              lineHeight = 16.sp)
      if (state.job?.status == "running")
          Text(
              modelDestinationLabel(ModelScope.Analyze, state.model),
              color = if (state.model.remoteProvider) Warning else SecondaryText,
              fontSize = 11.sp,
              lineHeight = 16.sp,
              modifier = Modifier.padding(top = 8.dp))
      else
          RemoteProviderConfirmation(
              ModelScope.Analyze,
              state.model,
              state.remoteProviderConfirmed,
              onRemoteProviderConfirmed)
      IdeHorizontalSeparator(Modifier.padding(top = 8.dp))
      SectionLabel("Review coverage", Modifier.padding(top = 8.dp))
      Text(
          coverage.stateLabel,
          color =
              if (coverage.stateLabel == "Unknown" || coverage.stateLabel.startsWith("Partial"))
                  Warning
              else SecondaryText,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = 4.dp))
      if (coverage.rows.isNotEmpty())
          CompactKeyValueRows(coverage.rows, Modifier.padding(top = 4.dp))
    }
  }
}

@Composable
private fun PerformanceFindingDetails(
    finding: PerformanceFinding,
    path: String,
    stale: Boolean,
    actions: PerformanceWorkspaceActions
) {
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = "Selected opportunity",
        icon = DesktopIcon.Performance,
        stateLabel = "${finding.potentialImpact} · Not measured · $path:${finding.startLine}",
        stateTint = performanceFindingTint(finding.potentialImpact),
        actions = {
          MiniOrcaButton(
              { actions.openInEditor(path, finding) },
              tone = ActionTone.Neutral,
              density = ButtonDensity.Toolbar) {
                Text("Open in Editor", fontSize = 11.sp)
              }
          MiniOrcaButton(
              { actions.prepareOptimization(path, finding) },
              enabled = finding.symbol.isNotBlank(),
              tone = ActionTone.Primary,
              density = ButtonDensity.Toolbar) {
                Text("Prepare optimization", fontSize = 11.sp)
              }
        },
    )
    SelectionContainer {
      Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
        CompactKeyValueRows(
            listOf("Location" to "$path:${finding.startLine}", "Confidence" to finding.confidence))
        Text(
            "Observed pattern: ${finding.observedPattern}",
            color = PrimaryText,
            fontSize = 12.sp,
            lineHeight = 18.sp,
            modifier = Modifier.padding(top = 6.dp))
        Text(
            "When it matters: ${finding.workloadConditions}\nRecommendation: ${finding.recommendation}\nTrade-off: ${finding.tradeoff}\nVerify: ${finding.verificationPlan}",
            color = SecondaryText,
            fontSize = 11.sp,
            lineHeight = 16.sp,
            modifier = Modifier.padding(top = 6.dp))
      }
    }
    EngineeringInsightPanel(
        finding.engineeringInsight, stale = stale, scopeLabel = "Selected performance opportunity")
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
    val job: PerformanceJob?,
    val report: PerformanceReport?,
    val context: PerformanceQueuePreview?,
    val model: ScopedModel,
    val remoteProviderConfirmed: Boolean,
    val benchmarkComparison: GoBenchmarkComparison? = null,
    val expectedBenchmarkIdentity: GoBenchmarkComparisonIdentity? = null,
    val benchmarkCatalog: GoBenchmarkCatalog? = null,
    val selectedBenchmark: GoBenchmarkChoice? = null,
    val benchmarkRunning: Boolean = false,
)

internal data class PerformanceWorkspaceActions(
    val confirmRemoteProvider: (Boolean) -> Unit,
    val preview: () -> Unit,
    val start: (PerformanceQueuePreview, Boolean) -> Unit,
    val pause: () -> Unit,
    val resume: (Boolean) -> Unit,
    val cancel: () -> Unit,
    val openInEditor: (String, PerformanceFinding) -> Unit,
    val prepareOptimization: (String, PerformanceFinding) -> Unit,
    val loadBenchmarks: () -> Unit = {},
    val selectBenchmark: (GoBenchmarkChoice) -> Unit = {},
    val runBenchmark: () -> Unit = {},
)

internal enum class PerformanceToolbarAction {
  Preview,
  Start,
  Pause,
  Resume,
  Cancel,
}

internal data class PerformanceToolbarActionPresentation(
    val action: PerformanceToolbarAction,
    val enabled: Boolean,
)

internal data class PerformanceCoveragePresentation(
    val stateLabel: String,
    val rows: List<Pair<String, String>>,
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

internal class PerformanceReviewPresentation
private constructor(
    val report: PerformanceReport?,
) {
  val hasReport: Boolean
    get() = report != null

  val isStale: Boolean
    get() = report?.status.equals("stale", ignoreCase = true)

  fun findings(category: String, impact: String, path: String): List<PerformanceFinding> =
      report
          ?.findings
          .orEmpty()
          .filter {
            (category.isBlank() || it.category.equals(category, true)) &&
                (impact.isBlank() || it.potentialImpact.equals(impact, true)) &&
                (path.isBlank() || pathFor(it).contains(path, true))
          }
          .sortedWith(
              compareByDescending<PerformanceFinding> { performanceImpactOrder(it.potentialImpact) }
                  .thenByDescending { performanceConfidenceOrder(it.confidence) }
                  .thenBy { pathFor(it) }
                  .thenBy { it.startLine }
                  .thenBy { it.id })

  fun pathFor(finding: PerformanceFinding): String = report?.paths?.get(finding.id).orEmpty()

  companion object {
    fun forJob(job: PerformanceJob?, report: PerformanceReport?): PerformanceReviewPresentation =
        PerformanceReviewPresentation(
            report?.takeIf { candidate ->
              job?.let { performanceReportMatchesJob(candidate, it) } == true
            })
  }
}

internal fun performanceReviewPresentation(
    job: PerformanceJob?,
    report: PerformanceReport?,
): PerformanceReviewPresentation = PerformanceReviewPresentation.forJob(job, report)

internal fun performanceCoveragePresentation(
    job: PerformanceJob?,
    report: PerformanceReport?,
): PerformanceCoveragePresentation {
  if (job == null) return PerformanceCoveragePresentation("Unknown", emptyList())

  val effectiveReport = performanceReviewPresentation(job, report).report
  val status = effectiveReport?.status?.ifBlank { job.status } ?: job.status
  val counts =
      effectiveReport?.counts?.takeIf { it.isNotEmpty() }
          ?: job.files.groupingBy { it.status }.eachCount()
  val completed = counts["completed"].orZero()
  val cached = counts["cached"].orZero()
  val stale = counts["stale"].orZero()
  val skipped = counts["skipped"].orZero()
  val failed = counts["failed"].orZero()
  val pending = counts["pending"].orZero()
  val running = counts["running"].orZero()
  val remaining = pending + running
  val partial =
      remaining > 0 ||
          skipped > 0 ||
          failed > 0 ||
          stale > 0 ||
          status in setOf("running", "paused", "canceled", "stale")
  val budgetLimited =
      job.runBudget > 0 &&
          (job.elapsed >= job.runBudget || (status == "completed" && remaining > 0))
  val coverageState =
      when {
        status == "canceled" -> "Partial · canceled review retains completed files"
        status == "stale" || stale > 0 -> "Partial · source or policy changed"
        partial -> "Partial"
        else -> "Complete"
      }
  val rows = buildList {
    add("Reviewed" to "$completed completed · $cached cached")
    if (stale > 0) add("Stale" to stale.toString())
    add("Skipped" to skipped.toString())
    add("Failed" to failed.toString())
    add("Remaining" to "$pending pending · $running running")
    if (job.runBudget > 0) {
      val budget = "${job.elapsed / 1_000_000_000}s of ${job.runBudget / 1_000_000_000}s"
      add("Budget" to if (budgetLimited) "$budget · budget-limited" else budget)
    }
  }
  return PerformanceCoveragePresentation(coverageState, rows)
}

private fun Int?.orZero(): Int = this ?: 0

internal fun performanceToolbarActions(
    job: PerformanceJob?,
    hasPreviewContext: Boolean,
    model: ScopedModel,
    remoteProviderConfirmed: Boolean,
): List<PerformanceToolbarActionPresentation> {
  val providerConfirmed = !model.remoteProvider || remoteProviderConfirmed
  return when (job?.status) {
    "running" ->
        listOf(
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Pause, true),
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Cancel, true),
        )
    "paused" ->
        listOf(
            PerformanceToolbarActionPresentation(
                PerformanceToolbarAction.Resume, providerConfirmed),
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Cancel, true),
        )
    else ->
        listOf(
            PerformanceToolbarActionPresentation(PerformanceToolbarAction.Preview, true),
            PerformanceToolbarActionPresentation(
                PerformanceToolbarAction.Start, hasPreviewContext && providerConfirmed),
        )
  }
}

internal fun performanceToolbarActionLabel(action: PerformanceToolbarAction): String =
    when (action) {
      PerformanceToolbarAction.Preview -> "Preview limits"
      PerformanceToolbarAction.Start -> "Analyze performance"
      PerformanceToolbarAction.Pause -> "Pause"
      PerformanceToolbarAction.Resume -> "Resume"
      PerformanceToolbarAction.Cancel -> "Cancel"
    }

private fun performanceToolbarActionTone(action: PerformanceToolbarAction) =
    when (action) {
      PerformanceToolbarAction.Preview -> ActionTone.Neutral
      PerformanceToolbarAction.Start,
      PerformanceToolbarAction.Resume -> ActionTone.Primary
      PerformanceToolbarAction.Pause -> ActionTone.Attention
      PerformanceToolbarAction.Cancel -> ActionTone.Destructive
    }

private fun performanceStatusTint(job: PerformanceJob?, report: PerformanceReport?) =
    when (performanceReviewStatus(job, report)) {
      "running" -> SelectionText
      "paused",
      "stale" -> Warning
      "completed" -> Success
      else -> SecondaryText
    }

private fun performanceFindingTint(impact: String) =
    when (impact.lowercase()) {
      "high" -> Error
      "medium" -> Warning
      else -> SecondaryText
    }

internal fun performanceStatusLabel(
    job: PerformanceJob?,
    report: PerformanceReport? = null
): String =
    when (performanceReviewStatus(job, report)) {
      "running" -> "Running · ${(job?.elapsed ?: 0) / 1_000_000_000}s budget used"
      "paused" -> "Paused · resume explicitly"
      "canceled" -> "Canceled · completed reviews remain available"
      "stale" -> "Stale · source or policy changed"
      "completed" -> "Completed · source-based queue"
      else -> "No review yet"
    }

private fun performanceReviewStatus(job: PerformanceJob?, report: PerformanceReport?): String? =
    performanceReviewPresentation(job, report).report?.status?.ifBlank { job?.status.orEmpty() }
        ?: job?.status

private fun performanceReportMatchesJob(report: PerformanceReport, job: PerformanceJob): Boolean =
    job.projectId.isNotBlank() &&
        job.projectRevision.isNotBlank() &&
        job.queueId.isNotBlank() &&
        report.projectId == job.projectId &&
        report.projectRevision == job.projectRevision &&
        report.queueId == job.queueId

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
