package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Constraints
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
    val tint: Color = ResultAccent,
)

internal data class ProjectSummaryPresentation(
    val hasProject: Boolean,
    val projectType: String,
    val buildMetadata: String,
    val languages: String,
    val analysisStatus: String,
    val summaryStatus: String,
    val analysisMessage: String,
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
  val metrics =
      overview?.metrics
          ?: project?.let {
            ProjectMetrics(
                type = it.type,
                buildFile = it.buildFile,
                fileCount = it.fileCount,
                sourceFileCount = it.sourceFileCount,
                totalLines = it.totalLines,
                languages = it.languages)
          }
  val hasProject = overview != null || project != null
  val analysis = overview?.analysis
  val analysisStatus =
      analysis?.status?.takeIf { it.isNotBlank() }
          ?: project?.aiStatus?.takeIf { it.isNotBlank() }
          ?: "missing"
  val normalizedStatus = analysisStatus.lowercase()
  val interpretationAvailable = normalizedStatus in setOf("fresh", "stale")
  val selection =
      fileSelection?.takeIf {
        it.projectId == (project?.projectId ?: overview?.projectId) &&
            it.projectRevision == (project?.projectRevision ?: overview?.projectRevision)
      }
  val coverage = selection?.let(::analysisSelectionCoverage) ?: overview?.analysisCoverage
  val hasCoverage = selection != null || coverage != null && coverage != AnalysisCoverage()
  val findings = overview?.findingCounts
  val outdated =
      if (hasCoverage) (coverage?.stale ?: 0) > 0
      else
          normalizedStatus == "stale" ||
              (coverage?.stale ?: 0) > 0 ||
              (run != null && AnalysisResultPageState(AnalysisResultType.Bugs, project, run).stale)
  return ProjectSummaryPresentation(
      hasProject = hasProject,
      projectType = metrics?.type?.ifBlank { "Unknown project type" } ?: "Unavailable",
      buildMetadata = metrics?.buildFile?.ifBlank { "No build metadata" } ?: "Unavailable",
      languages = metrics?.languages?.keys?.sorted()?.joinToString(" · ").orEmpty(),
      analysisStatus = normalizedStatus,
      summaryStatus =
          when {
            run?.isActive() == true -> "running"
            run?.status == "failed" -> "failed"
            hasCoverage -> analysisCoverageStatus(requireNotNull(coverage))
            normalizedStatus == "failed" -> "failed"
            outdated -> "stale"
            run?.status in setOf("completed", "completed_empty") -> "fresh"
            else -> normalizedStatus
          },
      analysisMessage =
          listOfNotNull(
                  if (hasCoverage)
                      "Selected files: ${analysisStatusLabel(analysisCoverageStatus(requireNotNull(coverage)))}"
                  else null,
                  summaryAnalysisMessage(normalizedStatus, analysis?.failure.orEmpty()),
                  if (run?.status == "failed" && normalizedStatus != "failed")
                      "Analysis run failed · ${run.reason.ifBlank { "No failure details available" }}"
                  else null,
                  if (outdated && normalizedStatus != "stale")
                      "Some analysis results are outdated. Run analysis to update them."
                  else null)
              .joinToString("\n"),
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
                  ProjectSummaryMetric("Ready", coverage?.fresh, SummaryMetricTone.Ready),
                  ProjectSummaryMetric("Stale", coverage?.stale, SummaryMetricTone.Stale),
                  ProjectSummaryMetric("Missing", coverage?.missing, SummaryMetricTone.Missing),
                  ProjectSummaryMetric("Running", coverage?.running, SummaryMetricTone.Running),
                  ProjectSummaryMetric("Failed", coverage?.failed, SummaryMetricTone.Failed),
                  ProjectSummaryMetric("Incomplete", coverage?.partial, SummaryMetricTone.Stale),
                  ProjectSummaryMetric(
                      "Unavailable", coverage?.unavailable, SummaryMetricTone.Failed),
              )
              .filter { it.value != 0 },
      issueMetrics = summaryIssueMetrics(project, run, sections),
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

/** Indexed facts and advisory interpretation share a dashboard, with distinct section labels. */
@Composable
internal fun ProjectSummaryPane(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
    openResults: (Workspace) -> Unit,
    run: AnalysisRun? = overview?.analysisRun,
    sections: Map<AnalysisResultKey, AnalysisSectionState> = emptyMap(),
    fileSelection: AnalysisFileSelection? = null,
) {
  val presentation = projectSummaryPresentation(overview, project, run, sections, fileSelection)
  val fontScale = LocalDensity.current.fontScale
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    val contentWidth = maxWidth - workspacePageHorizontalGutter(maxWidth) * 2
    val readableWidth = contentWidth / fontScale
    val detailColumns = if (readableWidth >= 720.dp) 2 else 1
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = workspacePagePadding(maxWidth, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
      if (!presentation.hasProject) {
        item {
          SystemStateMessage(
              "No project selected",
              "Import a project to view indexed facts and any available interpretation.",
              modifier = Modifier.fillMaxWidth())
        }
      } else {
        item {
          SummarySection("Purpose", ResultAccent) {
            presentation.purpose?.let { ModelResultContent(it, preview = false) }
                ?: Text("No purpose available.", color = SecondaryText, style = IdeTypography.body)
          }
        }
        item { SummaryOverviewMetrics(presentation, openResults) }
        items(presentation.details) { detail -> SummaryDetail(detail) }
        presentation.engineeringInsight?.let { insight ->
          val pieces = engineeringInsightPieces(insight)
          if (pieces.isNotEmpty()) {
            item {
              SummarySection("Engineering insight", ResultAccent) {
                pieces.chunked(detailColumns).forEach { row ->
                  Row(
                      Modifier.fillMaxWidth(),
                      horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        row.forEach { piece ->
                          Column(
                              Modifier.weight(1f),
                              verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                Text(
                                    piece.label,
                                    color = ResultAccent,
                                    style = IdeTypography.resultLabel)
                                ModelResultContent(piece.content, preview = false)
                              }
                        }
                      }
                }
              }
            }
          }
        }
      }
    }
  }
}

@Composable
private fun SummaryOverviewMetrics(
    presentation: ProjectSummaryPresentation,
    openResults: (Workspace) -> Unit,
) {
  val tint = summaryAnalysisTint(presentation.summaryStatus)
  AccentPanel(
      "Analysis summary",
      tint,
      Modifier.testTag("analysis-summary"),
      trailing = {
        androidx.compose.foundation.layout.Spacer(Modifier.weight(1f))
        SummaryAnalysisStatus(presentation)
      }) {
        SummaryMetricGrid(
            Modifier.fillMaxWidth().padding(start = 8.dp, end = 8.dp, bottom = 8.dp)) {
              (presentation.projectMetrics +
                      presentation.findingMetrics +
                      presentation.coverageMetrics)
                  .forEach { SummaryMetric(it) }
              presentation.issueMetrics.forEach { metric ->
                SummaryIssue(metric, onClick = { openResults(metric.type.workspace) })
              }
            }
      }
}

/** One measured cell size keeps all rows aligned, including wrapped labels and larger text. */
@Composable
private fun SummaryMetricGrid(modifier: Modifier, content: @Composable () -> Unit) {
  val fontScale = LocalDensity.current.fontScale
  Layout(content = content, modifier = modifier) { measurables, constraints ->
    val gap = 8.dp.roundToPx()
    val minimumWidth = (112.dp * fontScale).roundToPx()
    val maximumColumns =
        ((constraints.maxWidth + gap) / (minimumWidth + gap)).coerceIn(1, measurables.size)
    val rows = (measurables.size + maximumColumns - 1) / maximumColumns
    val columns = (measurables.size + rows - 1) / rows
    val cellWidth = (constraints.maxWidth - (columns - 1) * gap) / columns
    val cellHeight =
        measurables
            .maxOf { it.minIntrinsicHeight(cellWidth) }
            .coerceAtLeast((64.dp * fontScale).roundToPx())
    val cells = measurables.map { it.measure(Constraints.fixed(cellWidth, cellHeight)) }
    layout(constraints.maxWidth, rows * cellHeight + (rows - 1) * gap) {
      cells.forEachIndexed { index, cell ->
        cell.placeRelative(
            (index % columns) * (cellWidth + gap), (index / columns) * (cellHeight + gap))
      }
    }
  }
}

@Composable
private fun SummarySection(
    title: String,
    tint: Color,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
  Column(
      modifier
          .fillMaxWidth()
          .clip(MiniOrcaShapes.interactiveCard)
          .background(Panel)
          .border(1.dp, PaneSeparator, MiniOrcaShapes.interactiveCard)) {
        Text(
            title,
            color = tint,
            style = IdeTypography.section,
            modifier =
                Modifier.fillMaxWidth()
                    .background(blendOver(tint.copy(alpha = 0.08f), Panel))
                    .semantics { heading() }
                    .padding(8.dp))
        Column(
            Modifier.fillMaxWidth().padding(8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
            content = content)
      }
}

@Composable
private fun SummaryMetric(metric: ProjectSummaryMetric, modifier: Modifier = Modifier) {
  val tint = if (metric.value == null) FaintText else summaryMetricTint(metric.tone)
  Column(
      modifier
          .testTag("summary-metric-${metric.label}")
          .clip(MiniOrcaShapes.control)
          .background(blendOver(tint.copy(alpha = 0.08f), Panel))) {
        Box(Modifier.fillMaxWidth().height(4.dp).background(tint))
        Column(Modifier.padding(6.dp)) {
          Text(
              metric.value?.toString() ?: "—",
              color = tint,
              fontSize = 18.sp,
              lineHeight = 22.sp,
              fontWeight = FontWeight.SemiBold)
          Text(
              metric.label,
              color = tint,
              style = IdeTypography.resultLabel,
              modifier = Modifier.padding(top = 4.dp))
        }
      }
}

private fun summaryMetricTint(tone: SummaryMetricTone): Color =
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

@Composable
private fun SummaryDetail(detail: ProjectSummaryDetail, modifier: Modifier = Modifier) {
  SummarySection(detail.title, detail.tint, modifier) {
    when (detail.title) {
      "Architecture" -> MermaidDiagram(detail.values.single(), "Architecture")
      "Packages / modules" -> SummaryModules(detail.values)
      "Flows" ->
          detail.values.forEachIndexed { index, value ->
            if (index > 0) IdeHorizontalSeparator()
            MermaidDiagram(value, "Flow ${index + 1}")
          }
      else ->
          detail.values.forEachIndexed { index, value ->
            if (index > 0) IdeHorizontalSeparator()
            ModelResultContent(value, preview = false)
          }
    }
  }
}
