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
        item { PerformanceFindingDetails(finding, presentation.pathFor(finding), actions) }
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
    actions: PerformanceWorkspaceActions
) {
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = "Selected opportunity",
        icon = DesktopIcon.Performance,
        stateLabel = "${finding.potentialImpact} · $path:${finding.startLine}",
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
        finding.engineeringInsight, scopeLabel = "Selected performance opportunity")
  }
}

internal data class PerformanceWorkspacePaneState(
    val job: PerformanceJob?,
    val report: PerformanceReport?,
    val context: PerformanceQueuePreview?,
    val model: ScopedModel,
    val remoteProviderConfirmed: Boolean,
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

internal class PerformanceReviewPresentation
private constructor(
    val report: PerformanceReport?,
) {
  val hasReport: Boolean
    get() = report != null

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
