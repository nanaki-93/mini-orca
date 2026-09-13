package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
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
    val tint: Color = ResultAccent,
    val group: SummaryDetailGroup = SummaryDetailGroup.Overview,
)

internal enum class SummaryDetailGroup {
  Overview,
  Structure,
  Guidance,
}

internal data class ProjectSummaryPresentation(
    val hasProject: Boolean,
    val projectType: String,
    val buildMetadata: String,
    val languages: String,
    val analysisStatus: String,
    val analysisMessage: String,
    val purpose: String?,
    val projectMetrics: List<ProjectSummaryMetric>,
    val findingMetrics: List<ProjectSummaryMetric>,
    val coverageMetrics: List<ProjectSummaryMetric>,
    val details: List<ProjectSummaryDetail>,
    val engineeringInsight: EngineeringInsight?,
)

internal fun projectSummaryPresentation(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
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
  val coverage = overview?.analysisCoverage
  val findings = overview?.findingCounts
  return ProjectSummaryPresentation(
      hasProject = hasProject,
      projectType = metrics?.type?.ifBlank { "Unknown project type" } ?: "Unavailable",
      buildMetadata = metrics?.buildFile?.ifBlank { "No build metadata" } ?: "Unavailable",
      languages = metrics?.languages?.keys?.sorted()?.joinToString(" · ").orEmpty(),
      analysisStatus = normalizedStatus,
      analysisMessage = summaryAnalysisMessage(normalizedStatus, analysis?.failure.orEmpty()),
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
              )
              .filter { it.value != 0 },
      details = if (interpretationAvailable) projectSummaryDetails(analysis) else emptyList(),
      engineeringInsight = analysis?.engineeringInsight.takeIf { interpretationAvailable },
  )
}

internal fun summaryMetricColumnCount(availableWidth: Dp): Int =
    when {
      availableWidth >= 480.dp -> 5
      availableWidth >= 320.dp -> 3
      else -> 2
    }

private fun summaryAnalysisMessage(status: String, failure: String): String =
    when (status) {
      "fresh" -> "Project analysis: current · AI-generated"
      "stale" -> "Project analysis: stale · source may have changed"
      "failed" -> "Project analysis: failed · ${failure.ifBlank { "No failure details available" }}"
      "running" -> "Project analysis: running"
      "missing" -> "Project analysis: unavailable"
      else -> "Project analysis: ${status.replace('_', ' ')}"
    }

private fun projectSummaryDetails(
    analysis: StructuredProjectAnalysis?,
): List<ProjectSummaryDetail> {
  if (analysis == null) return emptyList()
  return listOfNotNull(
      analysis.architecture
          .takeIf { it.isNotBlank() }
          ?.let {
            ProjectSummaryDetail("Architecture", listOf(it), group = SummaryDetailGroup.Structure)
          },
      analysis.components
          .takeIf { it.isNotEmpty() }
          ?.let {
            ProjectSummaryDetail("Packages / modules", it, group = SummaryDetailGroup.Structure)
          },
      analysis.flows
          .takeIf { it.isNotEmpty() }
          ?.let { ProjectSummaryDetail("Flows", it, group = SummaryDetailGroup.Structure) },
      analysis.risks
          .takeIf { it.isNotEmpty() }
          ?.map { risk -> "${risk.severity.ifBlank { "unknown" }.uppercase()} · ${risk.summary}" }
          ?.let {
            ProjectSummaryDetail("Risks · AI suggestions", it, Warning, SummaryDetailGroup.Guidance)
          },
  )
}

/** Indexed facts and advisory interpretation share a dashboard, with distinct section labels. */
@Composable
internal fun ProjectSummaryPane(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
) {
  val presentation = projectSummaryPresentation(overview, project)
  val fontScale = LocalDensity.current.fontScale
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    val contentWidth = maxWidth - workspacePageHorizontalGutter(maxWidth) * 2
    val readableWidth = contentWidth / fontScale
    val detailColumns = if (readableWidth >= 720.dp) 2 else 1
    val details =
        listOfNotNull(presentation.purpose?.let { ProjectSummaryDetail("Purpose", listOf(it)) }) +
            presentation.details
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
        item { SummaryUnderstandingHeader(presentation) }
        item { SummaryOverviewMetrics(presentation, readableWidth >= 1120.dp) }
        details
            .groupBy { it.group }
            .forEach { (group, sections) ->
              val columns = if (group == SummaryDetailGroup.Structure) 1 else detailColumns
              items(sections.chunked(columns)) { row ->
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                  row.forEach { detail -> SummaryDetail(detail, Modifier.weight(1f)) }
                }
              }
            }
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
private fun SummaryOverviewMetrics(presentation: ProjectSummaryPresentation, singleRow: Boolean) {
  Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
    if (singleRow) {
      Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        SummaryProjectFacts(presentation, Modifier.weight(1f))
        SummaryFindings(presentation, Modifier.weight(1f))
        if (presentation.coverageMetrics.isNotEmpty()) {
          SummaryCoverage(presentation, Modifier.weight(2f))
        }
      }
    } else {
      ResponsiveFieldPair(
          minimumHorizontalWidth = 640.dp * LocalDensity.current.fontScale,
          first = { SummaryProjectFacts(presentation, it) },
          second = { SummaryFindings(presentation, it) })
      if (presentation.coverageMetrics.isNotEmpty()) SummaryCoverage(presentation)
    }
  }
}

@Composable
private fun SummaryProjectFacts(presentation: ProjectSummaryPresentation, modifier: Modifier) {
  SummarySection("Project facts", SelectionAccent, modifier) {
    SummaryMetricGrid(presentation.projectMetrics)
  }
}

@Composable
private fun SummaryFindings(presentation: ProjectSummaryPresentation, modifier: Modifier) {
  SummarySection("Findings", ResultAccent, modifier) {
    SummaryMetricGrid(presentation.findingMetrics)
  }
}

@Composable
private fun SummaryCoverage(
    presentation: ProjectSummaryPresentation,
    modifier: Modifier = Modifier
) {
  SummarySection(
      "Analysis coverage", statusBadgeStyle(presentation.analysisStatus).color, modifier) {
        SummaryMetricGrid(presentation.coverageMetrics)
      }
}

@Composable
private fun SummarySection(
    title: String,
    tint: Color,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
  Column(modifier.fillMaxWidth().background(Panel).border(1.dp, PaneSeparator)) {
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
private fun SummaryMetricGrid(metrics: List<ProjectSummaryMetric>) {
  val fontScale = LocalDensity.current.fontScale
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    val columns = summaryMetricColumnCount(maxWidth / fontScale).coerceAtMost(metrics.size)
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
      metrics.chunked(columns).forEach { row ->
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
          row.forEach { metric -> SummaryMetric(metric, Modifier.weight(1f)) }
          repeat(columns - row.size) { Spacer(Modifier.weight(1f)) }
        }
      }
    }
  }
}

@Composable
private fun SummaryMetric(metric: ProjectSummaryMetric, modifier: Modifier = Modifier) {
  val tint = if (metric.value == null) FaintText else summaryMetricTint(metric.tone)
  Column(modifier.background(blendOver(tint.copy(alpha = 0.08f), Panel))) {
    Box(Modifier.fillMaxWidth().height(4.dp).background(tint))
    Column(Modifier.padding(8.dp)) {
      Text(
          metric.value?.toString() ?: "—",
          color = tint,
          fontSize = 20.sp,
          lineHeight = 24.sp,
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
