package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Constraints
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

internal data class ProjectSummaryPresentation(
    val hasProject: Boolean,
    val projectName: String,
    val projectType: String,
    val buildMetadata: String,
    val languages: String,
    val analysisStatus: String,
    val summaryStatus: String,
    val analysisMessage: String,
    val interpretationStatus: String,
    val interpretationMessage: String,
    val outdated: Boolean,
    val purpose: String?,
    val projectMetrics: List<ProjectSummaryMetric>,
    val findingMetrics: List<ProjectSummaryMetric>,
    val coverageMetrics: List<ProjectSummaryMetric>,
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
  val selection =
      fileSelection?.takeIf {
        it.projectId == (project?.projectId ?: currentOverview?.projectId) &&
            it.projectRevision == (project?.projectRevision ?: currentOverview?.projectRevision)
      }
  val coverage = selection?.let(::analysisSelectionCoverage) ?: currentOverview?.analysisCoverage
  val hasCoverage = selection != null || coverage != null && coverage != AnalysisCoverage()
  val currentRun =
      run?.takeIf {
        it.identity.projectId == project?.projectId &&
            it.identity.projectRevision == project.projectRevision
      }
  val findings = currentOverview?.findingCounts
  val outdated =
      if (hasCoverage) (coverage?.stale ?: 0) > 0
      else
          normalizedStatus == "stale" ||
              (coverage?.stale ?: 0) > 0 ||
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
      analysisMessage =
          listOfNotNull(
                  if (hasCoverage)
                      "Selected files: ${analysisStatusLabel(analysisCoverageStatus(requireNotNull(coverage)))}"
                  else null,
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
          listOf(
                  ProjectSummaryMetric("Up to date", coverage?.fresh, SummaryMetricTone.Ready),
                  ProjectSummaryMetric("Outdated", coverage?.stale, SummaryMetricTone.Stale),
                  ProjectSummaryMetric(
                      "Not analyzed", coverage?.missing, SummaryMetricTone.Missing),
                  ProjectSummaryMetric("Running", coverage?.running, SummaryMetricTone.Running),
                  ProjectSummaryMetric("Failed", coverage?.failed, SummaryMetricTone.Failed),
                  ProjectSummaryMetric("Incomplete", coverage?.partial, SummaryMetricTone.Stale),
                  ProjectSummaryMetric(
                      "Unavailable",
                      coverage?.let {
                        it.unavailable +
                            (it.total.toLong() -
                                    it.fresh -
                                    it.stale -
                                    it.missing -
                                    it.running -
                                    it.failed -
                                    it.partial -
                                    it.unavailable)
                                .coerceAtLeast(0L)
                                .toInt()
                      },
                      SummaryMetricTone.Failed),
              )
              .filter { it.value != 0 },
      issueMetrics =
          summaryIssueMetrics(
              project, currentRun, if (currentRun != null) sections else emptyMap()),
      details = if (interpretationAvailable) projectSummaryDetails(analysis) else emptyList(),
      engineeringInsight = analysis?.engineeringInsight.takeIf { interpretationAvailable },
  )
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
    openResults: (Workspace) -> Unit,
    run: AnalysisRun? = overview?.analysisRun,
    sections: Map<AnalysisResultKey, AnalysisSectionState> = emptyMap(),
    fileSelection: AnalysisFileSelection? = null,
    analysisState: ProjectAnalysisRunState? = null,
    analysisActions: AnalysisWorkspaceActions? = null,
) {
  val presentation = projectSummaryPresentation(overview, project, run, sections, fileSelection)
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = workspacePagePadding(vertical = 20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
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
        if (currentRun?.showsProgressOnSummary() == true)
            item {
              AnalysisRunStrip(
                  runPaneState,
                  analysisActions,
                  AnalysisRunStripScope.Summary,
                  Modifier.testTag("summary-analysis-run-strip"))
            }
        item {
          SummaryIntroduction(
              presentation,
              runPaneState,
              analysisActions,
              showActionFeedback = currentRun?.showsProgressOnSummary() != true)
        }
        item { SummaryCoverage(presentation) { openResults(Workspace.Analysis) } }
        item {
          Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            SummaryCategories(presentation.issueMetrics, openResults)
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
        item {
          SummaryLowerComposition(
              presentation,
              listOf(
                  project?.projectId ?: overview?.projectId,
                  project?.projectRevision ?: overview?.projectRevision))
        }
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
            presentation.languages.split(" · ").filter {
              it.isNotBlank() && !it.equals(presentation.projectType, ignoreCase = true)
            }
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

// The narrative's right column needs more space than a category card (diagrams and prose).
internal fun summaryNarrativesStacked(width: Dp, fontScale: Float): Boolean =
    width < 16.dp + 740.dp * fontScale

@Composable
private fun SummaryLowerComposition(
    presentation: ProjectSummaryPresentation,
    ownerIdentity: List<String?>,
) {
  val architecture = presentation.details.firstOrNull { it.title == "Architecture" }
  val modules = presentation.details.firstOrNull { it.title == "Packages / modules" }
  val flows = presentation.details.firstOrNull { it.title == "Flows" }
  val insight =
      presentation.engineeringInsight?.let(::engineeringInsightPieces)?.takeIf { it.isNotEmpty() }
  val hasLeft = architecture != null || modules != null
  val hasRight = insight != null || flows != null
  if (!hasLeft && !hasRight) return

  Layout(
      content = {
        if (hasLeft) SummaryArchitectureModules(architecture, modules, ownerIdentity)
        if (hasRight)
            SummaryInsightFlows(
                insight, flows, presentation.interpretationStatus == "stale", ownerIdentity)
      },
      modifier = Modifier.fillMaxWidth().testTag("summary-lower-composition"),
  ) { measurables, constraints ->
    val width = constraints.maxWidth
    val gap = 16.dp.roundToPx()
    val stacked = !hasLeft || !hasRight || summaryNarrativesStacked(width.toDp(), fontScale)
    val stackedGap = if (stacked && hasLeft && hasRight) gap else 0
    val available = (width - gap).coerceAtLeast(0)
    val leftWidth = if (stacked) width else (available * 0.62f).toInt()
    val rightWidth = if (stacked) width else available - leftWidth
    val left = if (hasLeft) measurables[0].measure(Constraints.fixedWidth(leftWidth)) else null
    val right =
        if (hasRight) measurables[if (hasLeft) 1 else 0].measure(Constraints.fixedWidth(rightWidth))
        else null
    val height =
        if (stacked) (left?.height ?: 0) + stackedGap + (right?.height ?: 0)
        else maxOf(left?.height ?: 0, right?.height ?: 0)
    layout(width, height) {
      left?.placeRelative(0, 0)
      right?.placeRelative(
          if (stacked) 0 else leftWidth + gap, if (stacked) (left?.height ?: 0) + stackedGap else 0)
    }
  }
}

@Composable
private fun SummaryArchitectureModules(
    architecture: ProjectSummaryDetail?,
    modules: ProjectSummaryDetail?,
    ownerIdentity: List<String?>,
    modifier: Modifier = Modifier,
) {
  WorkspaceSection(modifier = modifier.testTag("summary-lower-left")) {
    architecture?.let {
      Column(Modifier.testTag("summary-architecture")) {
        MermaidDiagram(
            it.values.single(),
            "Architecture",
            title = "Architecture",
            ownerIdentity = ownerIdentity + "architecture")
      }
    }
    if (architecture != null && modules != null) IdeHorizontalSeparator()
    modules?.let {
      Column(Modifier.testTag("summary-modules")) {
        Text("Packages / modules", color = ResultAccent, style = IdeTypography.workspaceHeading)
        SummaryModules(it.values)
      }
    }
  }
}

@Composable
private fun SummaryInsightFlows(
    insight: List<EngineeringInsightPiece>?,
    flows: ProjectSummaryDetail?,
    stale: Boolean,
    ownerIdentity: List<String?>,
    modifier: Modifier = Modifier,
) {
  Column(
      modifier.testTag("summary-lower-right"), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        insight?.let { SummaryEngineeringInsight(it, stale, ownerIdentity) }
        flows?.let { SummaryFlows(it, ownerIdentity) }
      }
}

@Composable
private fun SummaryFlows(
    flows: ProjectSummaryDetail,
    ownerIdentity: List<String?>,
    modifier: Modifier = Modifier,
) {
  WorkspaceSection(modifier = modifier.testTag("summary-flows")) {
    flows.values.forEachIndexed { index, value ->
      if (index > 0) IdeHorizontalSeparator()
      MermaidDiagram(
          value,
          "Flow ${index + 1}",
          title = if (flows.values.size == 1) "Flows" else "Flow ${index + 1}",
          ownerIdentity = ownerIdentity + "flow-$index")
    }
  }
}

@Composable
private fun SummaryEngineeringInsight(
    pieces: List<EngineeringInsightPiece>,
    stale: Boolean,
    ownerIdentity: List<String?>,
    modifier: Modifier = Modifier,
) {
  WorkspaceSection("Engineering insight", modifier.testTag("summary-insight")) {
    SummaryEngineeringInsightPanel(
        pieces = pieces,
        stale = stale,
        ownerIdentity = ownerIdentity + pieces,
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
