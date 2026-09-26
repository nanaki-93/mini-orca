package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal enum class SummaryMetricTone {
  Fact,
  ToolReported,
  Suggestion,
  Ready,
  Stale,
  Missing,
  Running,
  Failed,
}

internal data class ProjectSummaryMetric(
    val label: String,
    val value: Int?,
    val tone: SummaryMetricTone,
)

internal data class ProjectSummaryDetail(
    val title: String,
    val values: List<String>,
)

internal data class SummaryCoverageOwner(
    val projectId: String,
    val projectRevision: String,
    val selectionId: String? = null,
)

internal sealed interface SummaryCoveragePaths {
  data class Selected(val rows: List<AnalysisFileStatus>) : SummaryCoveragePaths

  data object Unavailable : SummaryCoveragePaths
}

internal data class SummaryCoverageBucket(
    val id: AnalysisCoverageBucket,
    val count: Int,
    val paths: SummaryCoveragePaths,
)

internal sealed interface SummaryCoverageProjection {
  data class Known(
      val owner: SummaryCoverageOwner,
      val total: Int,
      val buckets: List<SummaryCoverageBucket>,
      val saved: AnalysisCoverage,
  ) : SummaryCoverageProjection

  data class Empty(val owner: SummaryCoverageOwner) : SummaryCoverageProjection

  data object Unavailable : SummaryCoverageProjection
}

/** Counts and inspectable paths are projected from the same confirmed owner, never from a run. */
internal fun summaryCoverageProjection(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
    fileSelection: AnalysisFileSelection?,
): SummaryCoverageProjection {
  val currentOverview =
      overview?.takeIf {
        project == null ||
            it.projectId == project.projectId && it.projectRevision == project.projectRevision
      }
  val projectId = project?.projectId ?: currentOverview?.projectId
  val revision = project?.projectRevision ?: currentOverview?.projectRevision
  val selection =
      fileSelection?.takeIf { it.projectId == projectId && it.projectRevision == revision }
  if (selection != null) {
    val owner =
        SummaryCoverageOwner(selection.projectId, selection.projectRevision, selection.selectionId)
    val rows = analysisSelectionCoverageRows(selection)
    if (rows.isEmpty()) return SummaryCoverageProjection.Empty(owner)
    val saved = analysisSelectionCoverage(selection)
    return SummaryCoverageProjection.Known(
        owner,
        saved.total,
        AnalysisCoverageBucket.entries.mapNotNull { id ->
          val matching = rows.filter { it.bucket == id }.map { it.saved }
          matching
              .takeIf { it.isNotEmpty() }
              ?.let { SummaryCoverageBucket(id, it.size, SummaryCoveragePaths.Selected(it)) }
        },
        saved)
  }
  val aggregate = currentOverview?.analysisCoverage ?: return SummaryCoverageProjection.Unavailable
  val counts =
      listOf(
          aggregate.fresh,
          aggregate.stale,
          aggregate.missing,
          aggregate.running,
          aggregate.failed,
          aggregate.partial,
          aggregate.unavailable)
  val accounted = counts.sumOf { it.toLong() }
  if (aggregate.total <= 0 || counts.any { it < 0 } || accounted > aggregate.total.toLong())
      return SummaryCoverageProjection.Unavailable
  val remainder = aggregate.total.toLong() - accounted
  // The residual fits in Int because the reported total is a nonnegative Int.
  val saved = aggregate.copy(unavailable = (aggregate.unavailable.toLong() + remainder).toInt())
  val bucketCounts =
      listOf(
          saved.fresh,
          saved.stale,
          saved.missing,
          saved.running,
          saved.failed,
          saved.partial,
          saved.unavailable)
  return SummaryCoverageProjection.Known(
      SummaryCoverageOwner(requireNotNull(projectId), requireNotNull(revision)),
      saved.total,
      AnalysisCoverageBucket.entries.zip(bucketCounts).mapNotNull { (id, count) ->
        count
            .takeIf { it > 0 }
            ?.let { SummaryCoverageBucket(id, it, SummaryCoveragePaths.Unavailable) }
      },
      saved)
}

internal sealed interface SummaryFileLedger {
  val selectionNotice: String?

  data class Selected(
      val owner: SummaryCoverageOwner,
      val totalSelected: Int,
      val rows: List<AnalysisFileStatus>,
      override val selectionNotice: String?,
  ) : SummaryFileLedger

  data class Empty(
      val owner: SummaryCoverageOwner,
      override val selectionNotice: String?,
  ) : SummaryFileLedger

  data class AggregateOnly(
      val totalReported: Int,
      override val selectionNotice: String?,
  ) : SummaryFileLedger

  data class Unavailable(override val selectionNotice: String?) : SummaryFileLedger
}

/** Ledger paths come only from the confirmed selection behind the saved coverage dial. */
internal fun summaryFileLedger(
    coverage: SummaryCoverageProjection,
    selectionNotice: String?,
): SummaryFileLedger =
    when (coverage) {
      is SummaryCoverageProjection.Known ->
          if (coverage.owner.selectionId == null)
              SummaryFileLedger.AggregateOnly(coverage.total, selectionNotice)
          else
              SummaryFileLedger.Selected(
                  coverage.owner,
                  coverage.total,
                  coverage.buckets
                      .flatMap { (it.paths as SummaryCoveragePaths.Selected).rows }
                      .sortedBy { it.file.path }
                      .take(3),
                  selectionNotice)
      is SummaryCoverageProjection.Empty -> SummaryFileLedger.Empty(coverage.owner, selectionNotice)
      SummaryCoverageProjection.Unavailable -> SummaryFileLedger.Unavailable(selectionNotice)
    }

internal data class ProjectSummaryPresentation(
    val hasProject: Boolean,
    val projectName: String,
    val projectType: String,
    val buildMetadata: String,
    val languages: String,
    val analysisStatus: String,
    val summaryStatus: String,
    val analysisMessage: String,
    val selectionNotice: String?,
    val selectionError: Boolean,
    val runMessage: String?,
    val interpretationStatus: String,
    val interpretationMessage: String,
    val outdated: Boolean,
    val purpose: String?,
    val projectMetrics: List<ProjectSummaryMetric>,
    val findingMetrics: List<ProjectSummaryMetric>,
    val coverageMetrics: List<ProjectSummaryMetric>,
    val coverage: SummaryCoverageProjection,
    val fileLedger: SummaryFileLedger,
    val issueMetrics: List<SummaryIssueMetric>,
    val details: List<ProjectSummaryDetail>,
    val engineeringInsight: EngineeringInsight?,
)

internal fun projectSummaryPresentation(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
    run: AnalysisRun? = overview?.analysisRun,
    sections: Map<AnalysisResultKey, AnalysisSectionState> = emptyMap(),
    fileSelection: AnalysisFileSelection? = null,
    selectionState: AnalysisSelectionState = AnalysisSelectionState(selection = fileSelection),
): ProjectSummaryPresentation {
  val currentOverview =
      overview?.takeIf {
        project == null ||
            (it.projectId == project.projectId && it.projectRevision == project.projectRevision)
      }
  val metrics =
      currentOverview?.metrics
          ?: project?.let {
            ProjectMetrics(
                type = it.type,
                buildFile = it.buildFile,
                fileCount = it.fileCount,
                sourceFileCount = it.sourceFileCount,
                totalLines = it.totalLines,
                languages = it.languages)
          }
  val hasProject = currentOverview != null || project != null
  val analysis = currentOverview?.analysis
  val analysisStatus =
      analysis?.status?.takeIf { it.isNotBlank() }
          ?: project?.aiStatus?.takeIf { it.isNotBlank() }
          ?: "missing"
  val normalizedStatus = analysisStatus.lowercase()
  val interpretationAvailable = normalizedStatus in setOf("fresh", "stale")
  val descriptionMessage =
      if (normalizedStatus == "fresh" && analysis?.purpose.isNullOrBlank())
          "Project description: unavailable · no purpose provided"
      else summaryAnalysisMessage(normalizedStatus, analysis?.failure.orEmpty())
  val currentSelectionState =
      selectionState.takeIf { state ->
        state.selection?.let {
          it.projectId == (project?.projectId ?: currentOverview?.projectId) &&
              it.projectRevision == (project?.projectRevision ?: currentOverview?.projectRevision)
        } != false
      } ?: AnalysisSelectionState()
  val coverageProjection =
      summaryCoverageProjection(currentOverview, project, currentSelectionState.selection)
  val selectionNotice = summarySelectionNotice(currentSelectionState, coverageProjection)
  val coverage =
      when (coverageProjection) {
        is SummaryCoverageProjection.Known -> coverageProjection.saved
        is SummaryCoverageProjection.Empty -> AnalysisCoverage()
        SummaryCoverageProjection.Unavailable -> null
      }
  val hasCoverage = coverage != null
  val currentRun =
      run?.takeIf {
        it.identity.projectId == project?.projectId &&
            it.identity.projectRevision == project.projectRevision
      }
  val findings = currentOverview?.findingCounts
  val outdated =
      if (coverage != null) coverage.stale > 0
      else
          normalizedStatus == "stale" ||
              (currentRun != null &&
                  AnalysisResultPageState(AnalysisResultType.Bugs, project, currentRun).stale)
  return ProjectSummaryPresentation(
      hasProject = hasProject,
      projectName = project?.name?.takeIf { it.isNotBlank() } ?: "Project",
      projectType = metrics?.type.orEmpty(),
      buildMetadata = metrics?.buildFile?.ifBlank { "No build metadata" } ?: "Unavailable",
      languages = metrics?.languages?.keys?.sorted()?.joinToString(" · ").orEmpty(),
      analysisStatus = normalizedStatus,
      summaryStatus =
          when {
            currentRun?.isActive() == true -> "running"
            currentRun?.status in setOf("paused", "interrupted", "canceled", "failed", "partial") ->
                requireNotNull(currentRun).status
            hasCoverage -> analysisCoverageStatus(requireNotNull(coverage))
            normalizedStatus == "failed" -> "failed"
            outdated -> "stale"
            normalizedStatus == "running" -> "running"
            else -> "unknown"
          },
      selectionNotice = selectionNotice,
      selectionError = currentSelectionState.error != null,
      runMessage =
          currentRun?.let {
            "${if (it.isActive()) "Current" else "Last"} analysis run: ${analysisStatusLabel(it.status)} · separate from saved coverage."
          },
      analysisMessage =
          listOfNotNull(
                  when (coverageProjection) {
                    is SummaryCoverageProjection.Known -> {
                      val source =
                          if (coverageProjection.owner.selectionId != null) "Selected files"
                          else "Saved aggregate coverage"
                      "$source: ${analysisStatusLabel(analysisCoverageStatus(coverageProjection.saved))}"
                    }
                    is SummaryCoverageProjection.Empty -> "Selected files: Excluded"
                    SummaryCoverageProjection.Unavailable -> null
                  },
                  descriptionMessage,
                  if (currentRun?.status == "failed" && normalizedStatus != "failed")
                      "Analysis run failed · ${currentRun.reason.ifBlank { "No failure details available" }}"
                  else null,
                  currentRun
                      ?.takeIf {
                        it.status in setOf("paused", "interrupted", "canceled", "partial") &&
                            it.reason.isNotBlank()
                      }
                      ?.let {
                        "Analysis run ${analysisStatusLabel(it.status).lowercase()} · ${it.reason}"
                      },
                  if (outdated && normalizedStatus != "stale")
                      "Some analysis results are outdated. Run analysis to update them."
                  else null)
              .joinToString("\n"),
      interpretationStatus = normalizedStatus,
      interpretationMessage = descriptionMessage,
      outdated = outdated,
      purpose = analysis?.purpose?.takeIf { interpretationAvailable && it.isNotBlank() },
      projectMetrics =
          listOf(
              ProjectSummaryMetric("Indexed files", metrics?.fileCount, SummaryMetricTone.Fact),
              ProjectSummaryMetric("Total lines", metrics?.totalLines, SummaryMetricTone.Fact),
          ),
      findingMetrics =
          listOf(
              ProjectSummaryMetric(
                  "Tool-reported issues", findings?.verified, SummaryMetricTone.ToolReported),
              ProjectSummaryMetric(
                  "AI suggestions", findings?.aiSuggestions, SummaryMetricTone.Suggestion),
          ),
      coverageMetrics =
          when (coverageProjection) {
            is SummaryCoverageProjection.Known ->
                coverageProjection.buckets.map { bucket ->
                  val (label, tone) =
                      when (bucket.id) {
                        AnalysisCoverageBucket.UpToDate -> "Up to date" to SummaryMetricTone.Ready
                        AnalysisCoverageBucket.Outdated -> "Outdated" to SummaryMetricTone.Stale
                        AnalysisCoverageBucket.NotAnalyzed ->
                            "Not analyzed" to SummaryMetricTone.Missing
                        AnalysisCoverageBucket.Running -> "Running" to SummaryMetricTone.Running
                        AnalysisCoverageBucket.Failed -> "Failed" to SummaryMetricTone.Failed
                        AnalysisCoverageBucket.Incomplete -> "Incomplete" to SummaryMetricTone.Stale
                        AnalysisCoverageBucket.Unavailable ->
                            "Unavailable" to SummaryMetricTone.Failed
                      }
                  ProjectSummaryMetric(label, bucket.count, tone)
                }
            is SummaryCoverageProjection.Empty -> emptyList()
            SummaryCoverageProjection.Unavailable ->
                listOf(
                    ProjectSummaryMetric("Up to date", null, SummaryMetricTone.Ready),
                    ProjectSummaryMetric("Outdated", null, SummaryMetricTone.Stale),
                    ProjectSummaryMetric("Not analyzed", null, SummaryMetricTone.Missing),
                    ProjectSummaryMetric("Running", null, SummaryMetricTone.Running),
                    ProjectSummaryMetric("Failed", null, SummaryMetricTone.Failed),
                    ProjectSummaryMetric("Incomplete", null, SummaryMetricTone.Stale),
                    ProjectSummaryMetric("Unavailable", null, SummaryMetricTone.Failed))
          },
      coverage = coverageProjection,
      fileLedger = summaryFileLedger(coverageProjection, selectionNotice),
      issueMetrics =
          summaryIssueMetrics(
              project, currentRun, if (currentRun != null) sections else emptyMap()),
      details = if (interpretationAvailable) projectSummaryDetails(analysis) else emptyList(),
      engineeringInsight = analysis?.engineeringInsight.takeIf { interpretationAvailable },
  )
}

private fun summarySelectionNotice(
    state: AnalysisSelectionState,
    coverage: SummaryCoverageProjection,
): String? {
  val confirmed =
      (coverage as? SummaryCoverageProjection.Known)?.owner?.selectionId != null ||
          coverage is SummaryCoverageProjection.Empty
  return when {
    state.error != null -> {
      val operation =
          when (state.failure) {
            AnalysisSelectionFailure.Read -> "load"
            AnalysisSelectionFailure.Save -> "save"
            null -> "update"
          }
      "File selection $operation failed · ${if (confirmed) "Showing last confirmed selection." else "No confirmed selection available."} ${state.error}"
    }
    state.saving ->
        if (confirmed)
            "Saving file selection · showing last confirmed selection until the save succeeds."
        else "Saving file selection · no confirmed selection available."
    state.loading ->
        if (confirmed) "Loading file selection · showing last confirmed selection."
        else "Loading file selection · no confirmed selection available."
    else -> null
  }
}

private fun summaryAnalysisMessage(status: String, failure: String): String =
    when (status) {
      "fresh" -> "Project description: current · AI-generated"
      "stale" -> "Project description: stale · source may have changed"
      "failed" ->
          "Project description: failed · ${failure.ifBlank { "No failure details available" }}"
      "running" -> "Project description: running"
      "missing" -> "Project description: unavailable"
      else -> "Project description: ${status.replace('_', ' ')}"
    }

private fun projectSummaryDetails(
    analysis: StructuredProjectAnalysis?,
): List<ProjectSummaryDetail> {
  if (analysis == null) return emptyList()
  return listOfNotNull(
      analysis.architecture
          .takeIf { it.isNotBlank() }
          ?.let { ProjectSummaryDetail("Architecture", listOf(it)) },
      analysis.components
          .takeIf { it.isNotEmpty() }
          ?.let { ProjectSummaryDetail("Packages / modules", it) },
      analysis.flows.takeIf { it.isNotEmpty() }?.let { ProjectSummaryDetail("Flows", it) },
  )
}

@Composable
internal fun ProjectSummaryPane(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
    selectWorkspace: (Workspace) -> Unit,
    run: AnalysisRun? = overview?.analysisRun,
    sections: Map<AnalysisResultKey, AnalysisSectionState> = emptyMap(),
    fileSelection: AnalysisFileSelection? = null,
    analysisState: ProjectAnalysisRunState? = null,
    analysisActions: AnalysisWorkspaceActions? = null,
    findingState: DesktopState =
        DesktopState(
            projectState = ProjectWorkspaceState(project = project),
            analysisRun = ProjectAnalysisRunState(run = run, sections = sections)),
    onFindingSelected: ((SummaryFindingTarget) -> Unit)? = null,
) {
  val findingPreview = summaryFindingPreview(findingState)
  val selectionState =
      analysisState?.fileSelection ?: AnalysisSelectionState(selection = fileSelection)
  val presentation =
      projectSummaryPresentation(
          overview, project, run, sections, selectionState.selection, selectionState)
  val ownerIdentity =
      listOf(
          project?.projectId ?: overview?.projectId,
          project?.projectRevision ?: overview?.projectRevision)
  val architecture =
      presentation.details.firstOrNull { it.title == "Architecture" }?.values?.single()
  val flows = presentation.details.firstOrNull { it.title == "Flows" }?.values.orEmpty()
  val pieces = presentation.engineeringInsight?.let(::engineeringInsightPieces).orEmpty()
  // Own local disclosure state above the lazy item so scrolling it away does not discard it.
  val architectureView = remember(ownerIdentity, architecture) { DiagramViewState() }
  val flowViews =
      flows.mapIndexed { index, value ->
        key(index) { remember(ownerIdentity, value) { DiagramViewState() } }
      }
  val insightExpansion = remember(ownerIdentity, pieces) { mutableStateOf(false) }
  // Inspection is local to the confirmed coverage owner, outside the lazy item lifecycle.
  val coverageOwner =
      when (val coverage = presentation.coverage) {
        is SummaryCoverageProjection.Known -> coverage.owner
        is SummaryCoverageProjection.Empty -> coverage.owner
        SummaryCoverageProjection.Unavailable -> null
      }
  val inspectedBucket = remember(coverageOwner) { mutableStateOf<AnalysisCoverageBucket?>(null) }
  val focusedLegend = remember(coverageOwner) { mutableStateOf<AnalysisCoverageBucket?>(null) }
  val availableBuckets =
      (presentation.coverage as? SummaryCoverageProjection.Known)?.buckets?.map { it.id }.orEmpty()
  LaunchedEffect(coverageOwner, availableBuckets) {
    if (inspectedBucket.value !in availableBuckets) inspectedBucket.value = null
  }
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    LazyColumn(
        Modifier.fillMaxSize().testTag("summary-scroll"),
        contentPadding = workspacePagePadding(vertical = 20.dp),
        verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section),
    ) {
      if (!presentation.hasProject) {
        item { SystemStateMessage("No project selected", "", modifier = Modifier.fillMaxWidth()) }
      } else {
        item {
          Text(
              "Summary",
              color = PrimaryText,
              style = IdeTypography.workspaceHeading,
              modifier = Modifier.testTag("summary-page-heading").semantics { heading() })
        }
        val currentRun = currentProjectRun(run, project)
        val stripState = analysisState ?: ProjectAnalysisRunState(run = run, sections = sections)
        val runPaneState = AnalysisWorkspacePaneState(project, stripState.copy(run = currentRun))
        item {
          SummaryIntroduction(
              presentation,
              runPaneState,
              analysisActions,
              showActionFeedback = currentRun?.showsProgressOnSummary() != true)
        }
        if (currentRun?.showsProgressOnSummary() == true)
            item {
              AnalysisRunStrip(
                  runPaneState,
                  analysisActions,
                  AnalysisRunStripScope.Summary,
                  Modifier.testTag("summary-analysis-run-strip"))
            }
        item {
          BoxWithConstraints(Modifier.fillMaxWidth().testTag("summary-coverage-results")) {
            val paired = !coverageResultsStacked(maxWidth, LocalDensity.current.fontScale)
            val coverage: @Composable (Modifier) -> Unit = { modifier ->
              androidx.compose.foundation.layout.Box(modifier.testTag("summary-coverage-column")) {
                SummaryCoverage(presentation, inspectedBucket, focusedLegend) {
                  selectWorkspace(Workspace.Analysis)
                }
              }
            }
            val results: @Composable (Modifier) -> Unit = { modifier ->
              Column(
                  modifier.testTag("summary-results"),
                  verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    SummaryCategories(presentation.issueMetrics, selectWorkspace)
                    SummaryFileEvidence(presentation.fileLedger) {
                      selectWorkspace(Workspace.Analysis)
                    }
                    Text(
                        "Overall findings · " +
                            presentation.findingMetrics.joinToString(" · ") {
                              "${it.value ?: "—"} ${if (it.label == "AI suggestions") it.label else it.label.lowercase()}"
                            },
                        color = SecondaryText,
                        style = IdeTypography.workspaceMetadata,
                        modifier = Modifier.testTag("summary-findings-provenance"))
                  }
            }
            if (paired) {
              Row(horizontalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section)) {
                coverage(Modifier.weight(2f))
                results(Modifier.weight(3f))
              }
            } else {
              Column(verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section)) {
                coverage(Modifier.fillMaxWidth())
                results(Modifier.fillMaxWidth())
              }
            }
          }
        }
        if (architecture != null || pieces.isNotEmpty()) {
          item {
            SummaryLowerComposition(presentation, ownerIdentity, architectureView, insightExpansion)
          }
        }
        item {
          SummaryModulesAndFindings(
              presentation.details.firstOrNull { it.title == "Packages / modules" },
              findingPreview,
              summaryFindingEmptyMessage(findingState, findingPreview),
              selectWorkspace,
              onFindingSelected)
        }
        if (flows.isNotEmpty()) {
          item {
            SummaryFlows(
                requireNotNull(presentation.details.firstOrNull { it.title == "Flows" }),
                ownerIdentity,
                flowViews)
          }
        }
        item { SummaryChangeLifecycle { selectWorkspace(Workspace.Editor) } }
      }
    }
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun SummaryIntroduction(
    presentation: ProjectSummaryPresentation,
    runState: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions?,
    showActionFeedback: Boolean,
) {
  val type = projectTypePresentation(presentation.projectType)
  WorkspaceSection(modifier = Modifier.testTag("summary-introduction")) {
    FlowRow(
        Modifier.fillMaxWidth().testTag("summary-project-identity"),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
        itemVerticalAlignment = Alignment.CenterVertically) {
          Text(
              presentation.projectName,
              color = PrimaryText,
              fontSize = 24.sp,
              lineHeight = 30.sp,
              fontWeight = FontWeight.SemiBold,
              modifier = Modifier.semantics { heading() })
          Text(
              type.description,
              color = type.tint,
              style = IdeTypography.workspaceMetadata,
              modifier = Modifier.testTag("summary-project-type"))
        }
    presentation.purpose?.let {
      ModelResultContent(it, preview = false, style = IdeTypography.workspaceBody)
    }
    Text(
        presentation.interpretationMessage,
        color = if (presentation.interpretationStatus == "failed") Error else SecondaryText,
        style = IdeTypography.workspaceMetadata,
        modifier = Modifier.testTag("summary-interpretation-status"))
    val facts =
        listOf(presentation.buildMetadata) +
            presentation.projectMetrics.map { metric ->
              "${metric.value?.let { "%,d".format(java.util.Locale.ROOT, it) } ?: "—"} ${if (metric.label == "Total lines") "lines" else metric.label.lowercase()}"
            } +
            presentation.languages.split(" · ").filter { it.isNotBlank() }
    Text(facts.joinToString(" · "), color = SecondaryText, style = IdeTypography.workspaceMetadata)
    if (actions != null &&
        AnalysisRunCommand.Start in projectRunPresentation(runState.analysis).commands) {
      MiniOrcaButton(
          onClick = { actions.start(defaultAnalysisRunLimits, false) },
          enabled = analysisRunActionEnabled(runState),
          tone = ActionTone.Primary,
          modifier = Modifier.testTag("summary-start-analysis")) {
            Text(AnalysisRunCommand.Start.label, style = IdeTypography.action)
          }
    }
    if (showActionFeedback) AnalysisActionFeedback(runState.analysis)
  }
}

// Reserve room for the coverage controls and three readable result cards in the paired row.
internal fun coverageResultsStacked(width: Dp, fontScale: Float): Boolean =
    width < 1080.dp * fontScale

// Two narrative panels need room for selectable prose and diagram controls.
internal fun narrativePanelsStacked(width: Dp, fontScale: Float): Boolean =
    width < 840.dp * fontScale

// Module descriptions and finding evidence each need a readable column.
internal fun summaryEvidencePanelsStacked(width: Dp, fontScale: Float): Boolean =
    width < 960.dp * fontScale

// Three category cards need room for their labels, counts and status at the current text scale.
internal fun categoryPanelsStacked(width: Dp, fontScale: Float, gap: Dp): Boolean =
    width < 200.dp * 3 * fontScale + gap * 2

@Composable
private fun SummaryCategories(metrics: List<SummaryIssueMetric>, openResults: (Workspace) -> Unit) {
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    if (categoryPanelsStacked(maxWidth, LocalDensity.current.fontScale, 12.dp)) {
      Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        metrics.forEach { metric ->
          SummaryIssue(metric, { openResults(metric.type.workspace) }, Modifier.fillMaxWidth())
        }
      }
    } else {
      Row(Modifier.height(IntrinsicSize.Max), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        metrics.forEach { metric ->
          SummaryIssue(
              metric, { openResults(metric.type.workspace) }, Modifier.weight(1f).fillMaxHeight())
        }
      }
    }
  }
}

@Composable
private fun SummaryFileEvidence(ledger: SummaryFileLedger, onOpenAnalysis: () -> Unit) {
  WorkspaceSection("File evidence", Modifier.testTag("summary-file-evidence")) {
    ledger.selectionNotice?.let {
      Text(
          it,
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata,
          modifier = Modifier.testTag("summary-file-selection-notice"))
    }
    when (ledger) {
      is SummaryFileLedger.Selected -> {
        Text(
            "Showing ${ledger.rows.size} of ${ledger.totalSelected} selected files · saved status",
            color = SecondaryText,
            style = IdeTypography.workspaceMetadata)
        SelectionContainer {
          Column(
              Modifier.fillMaxWidth().testTag("summary-file-paths"),
              verticalArrangement = Arrangement.spacedBy(8.dp)) {
                ledger.rows.forEach { row ->
                  Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(row.file.path, color = PrimaryText, style = IdeTypography.resultCode)
                    DiagnosticText(
                        "${row.status.label} · ${row.explanation}", color = SecondaryText)
                  }
                }
              }
        }
      }
      is SummaryFileLedger.Empty ->
          Text(
              "No files selected in the confirmed selection.",
              color = SecondaryText,
              style = IdeTypography.workspaceBody)
      is SummaryFileLedger.AggregateOnly ->
          Text(
              "File paths unavailable · ${ledger.totalReported} files in saved aggregate coverage. Load a confirmed selection to inspect file evidence.",
              color = SecondaryText,
              style = IdeTypography.workspaceBody)
      is SummaryFileLedger.Unavailable ->
          Text(
              "File evidence unavailable · no confirmed file selection or saved file paths.",
              color = SecondaryText,
              style = IdeTypography.workspaceBody)
    }
    MiniOrcaButton(
        onClick = onOpenAnalysis,
        tone = ActionTone.Navigation,
        modifier = Modifier.testTag("summary-all-files")) {
          Text("All files", style = IdeTypography.action)
        }
  }
}

@Composable
private fun SummaryLowerComposition(
    presentation: ProjectSummaryPresentation,
    ownerIdentity: List<String?>,
    architectureView: DiagramViewState,
    insightExpansion: MutableState<Boolean>,
) {
  val architecture = presentation.details.firstOrNull { it.title == "Architecture" }
  val insight =
      presentation.engineeringInsight?.let(::engineeringInsightPieces)?.takeIf { it.isNotEmpty() }
  Column(
      Modifier.fillMaxWidth().testTag("summary-lower-composition"),
      verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section)) {
        if (architecture != null || insight != null) {
          BoxWithConstraints(Modifier.fillMaxWidth().testTag("summary-narrative-row")) {
            val paired =
                architecture != null &&
                    insight != null &&
                    !narrativePanelsStacked(maxWidth, LocalDensity.current.fontScale)
            val architecturePanel: @Composable (Modifier) -> Unit = { modifier ->
              architecture?.let {
                WorkspaceSection(modifier = modifier.testTag("summary-architecture")) {
                  MermaidDiagram(
                      it.values.single(),
                      "Architecture",
                      title = "Architecture",
                      ownerIdentity = ownerIdentity + "architecture",
                      viewState = architectureView)
                }
              }
            }
            val insightPanel: @Composable (Modifier) -> Unit = { modifier ->
              insight?.let {
                SummaryEngineeringInsight(
                    it,
                    presentation.interpretationStatus == "stale",
                    ownerIdentity,
                    insightExpansion,
                    modifier)
              }
            }
            if (paired) {
              Row(horizontalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section)) {
                architecturePanel(Modifier.weight(1f))
                insightPanel(Modifier.weight(1f))
              }
            } else {
              Column(verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section)) {
                architecturePanel(Modifier.fillMaxWidth())
                insightPanel(Modifier.fillMaxWidth())
              }
            }
          }
        }
      }
}

@Composable
private fun SummaryModulesAndFindings(
    modules: ProjectSummaryDetail?,
    preview: SummaryFindingPreview,
    emptyMessage: String,
    openResults: (Workspace) -> Unit,
    onFindingSelected: ((SummaryFindingTarget) -> Unit)?,
) {
  BoxWithConstraints(Modifier.fillMaxWidth().testTag("summary-evidence-row")) {
    val modulePanel: @Composable (Modifier) -> Unit = { modifier ->
      modules?.let {
        WorkspaceSection(modifier = modifier.testTag("summary-modules")) {
          Text("Packages / modules", color = ResultAccent, style = IdeTypography.workspaceHeading)
          SummaryModules(it.values)
        }
      }
    }
    val findingPanel: @Composable (Modifier) -> Unit = { modifier ->
      SummarySelectedFindings(preview, emptyMessage, openResults, onFindingSelected, modifier)
    }
    if (modules != null &&
        !summaryEvidencePanelsStacked(maxWidth, LocalDensity.current.fontScale)) {
      Row(horizontalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section)) {
        modulePanel(Modifier.weight(1f))
        findingPanel(Modifier.weight(1f))
      }
    } else {
      Column(verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.section)) {
        modulePanel(Modifier.fillMaxWidth())
        findingPanel(Modifier.fillMaxWidth())
      }
    }
  }
}

@Composable
private fun SummarySelectedFindings(
    preview: SummaryFindingPreview,
    emptyMessage: String,
    openResults: (Workspace) -> Unit,
    onFindingSelected: ((SummaryFindingTarget) -> Unit)?,
    modifier: Modifier,
) {
  WorkspaceSection("Selected findings", modifier.testTag("summary-selected-findings")) {
    Text(
        "Showing ${preview.rows.size} of ${preview.loadedCount} loaded findings · not category totals",
        color = SecondaryText,
        style = IdeTypography.workspaceMetadata)
    if (preview.rows.isEmpty())
        Text(emptyMessage, color = SecondaryText, style = IdeTypography.workspaceBody)
    preview.categories.forEach { category ->
      Text(
          "${category.category.workspace.name} · ${category.status} · ${category.loadedCount} loaded" +
              (category.detail?.let { " · $it" } ?: ""),
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata)
    }
    preview.rows.forEachIndexed { index, row ->
      IdeActionSurface(
          onClick = { onFindingSelected?.invoke(row.target) },
          enabled = onFindingSelected != null,
          colors =
              IdeActionColors(
                  background = Panel,
                  hoveredBackground = ControlHover,
                  pressedBackground = SelectionSurface,
                  selectedBackground = SelectionSurface,
                  disabledBackground = Panel,
                  content = PrimaryText,
                  selectedContent = PrimaryText,
                  disabledContent = SecondaryText,
                  border = PaneSeparator),
          accessibleName =
              "Inspect ${row.title} in ${row.target.category.workspace.name} results at ${row.location}",
          shape = MiniOrcaShapes.interactiveCard,
          contentPadding = PaddingValues(horizontal = 12.dp, vertical = 10.dp),
          modifier = Modifier.fillMaxWidth().testTag("summary-finding-$index")) {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
              Text(row.title, color = PrimaryText, style = IdeTypography.workspaceBody)
              SelectionContainer {
                Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                  Text(
                      "${row.target.category.workspace.name} · ${row.severity.ifBlank { "Impact unavailable" }}",
                      color = SecondaryText,
                      style = IdeTypography.workspaceMetadata)
                  Text(
                      row.location,
                      color = PrimaryText,
                      style = IdeTypography.resultCode,
                      modifier = Modifier.fillMaxWidth().testTag("summary-finding-location-$index"))
                  Text(
                      "${row.origin} · ${row.materialState.ifBlank { "Material state unavailable" }}",
                      color = SecondaryText,
                      style = IdeTypography.workspaceMetadata)
                }
              }
            }
          }
    }
    AnalysisResultType.entries.forEach { type ->
      MiniOrcaButton(
          onClick = { openResults(type.workspace) },
          tone = ActionTone.Navigation,
          modifier = Modifier.testTag("summary-all-${type.category}")) {
            Text("All ${type.workspace.name} results", style = IdeTypography.action)
          }
    }
  }
}

@Composable
private fun SummaryChangeLifecycle(onOpenEditor: () -> Unit) {
  WorkspaceSection("Change lifecycle", Modifier.testTag("summary-change-lifecycle")) {
    Text(
        "Mini-Orca editing workflow (not a project Flow)",
        color = SecondaryText,
        style = IdeTypography.workspaceBody)
    Text(
        "Request → Draft → Validate → Checks → Review → Apply",
        color = PrimaryText,
        style = IdeTypography.workspaceMetadata,
        modifier = Modifier.testTag("summary-change-stages"))
    Text(
        "Edit only an isolated declaration/import draft. Apply is an explicit, guarded one-file source change; Undo is guarded and available only when the change is still eligible.",
        color = SecondaryText,
        style = IdeTypography.workspaceBody)
    MiniOrcaButton(
        onClick = onOpenEditor,
        tone = ActionTone.Navigation,
        modifier = Modifier.testTag("summary-open-editor")) {
          Text("Open Editor", style = IdeTypography.action)
        }
  }
}

@Composable
private fun SummaryFlows(
    flows: ProjectSummaryDetail,
    ownerIdentity: List<String?>,
    viewStates: List<DiagramViewState>,
    modifier: Modifier = Modifier,
) {
  WorkspaceSection(modifier = modifier.testTag("summary-flows")) {
    flows.values.forEachIndexed { index, value ->
      if (index > 0) IdeHorizontalSeparator()
      MermaidDiagram(
          value,
          "Flow ${index + 1}",
          title = if (flows.values.size == 1) "Flows" else "Flow ${index + 1}",
          ownerIdentity = ownerIdentity + "flow-$index",
          viewState = viewStates[index])
    }
  }
}

@Composable
private fun SummaryEngineeringInsight(
    pieces: List<EngineeringInsightPiece>,
    stale: Boolean,
    ownerIdentity: List<String?>,
    expansion: MutableState<Boolean>,
    modifier: Modifier = Modifier,
) {
  WorkspaceSection("Engineering insight", modifier.testTag("summary-insight")) {
    SummaryEngineeringInsightPanel(
        pieces = pieces,
        stale = stale,
        ownerIdentity = ownerIdentity + pieces,
        expansion = expansion,
    )
  }
}

internal fun summaryMetricTint(tone: SummaryMetricTone): Color =
    when (tone) {
      SummaryMetricTone.Fact -> SelectionText
      SummaryMetricTone.Missing -> SecondaryText
      SummaryMetricTone.ToolReported,
      SummaryMetricTone.Ready -> Success
      SummaryMetricTone.Suggestion,
      SummaryMetricTone.Running -> Information
      SummaryMetricTone.Stale -> Warning
      SummaryMetricTone.Failed -> Error
    }
