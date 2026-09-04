package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal enum class SummaryMetricTone {
  Fact,
  Verified,
  Suggestion,
  Fresh,
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
    val analysisMessage: String,
    val purpose: String?,
    val projectMetrics: List<ProjectSummaryMetric>,
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
      projectName = project?.name?.takeIf { it.isNotBlank() } ?: "Project summary",
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
              ProjectSummaryMetric(
                  "Verified findings", findings?.verified, SummaryMetricTone.Verified),
              ProjectSummaryMetric(
                  "AI suggestions", findings?.aiSuggestions, SummaryMetricTone.Suggestion),
          ),
      coverageMetrics =
          listOf(
              ProjectSummaryMetric("Fresh", coverage?.fresh, SummaryMetricTone.Fresh),
              ProjectSummaryMetric("Stale", coverage?.stale, SummaryMetricTone.Stale),
              ProjectSummaryMetric("Missing", coverage?.missing, SummaryMetricTone.Missing),
              ProjectSummaryMetric("Running", coverage?.running, SummaryMetricTone.Running),
              ProjectSummaryMetric("Failed", coverage?.failed, SummaryMetricTone.Failed),
          ),
      details = if (interpretationAvailable) projectSummaryDetails(analysis) else emptyList(),
      engineeringInsight = analysis?.engineeringInsight.takeIf { interpretationAvailable },
  )
}

internal fun summaryMetricColumnCount(availableWidth: Dp): Int =
    when {
      availableWidth >= 1040.dp -> 5
      availableWidth >= 780.dp -> 4
      else -> 2
    }

private fun summaryAnalysisMessage(status: String, failure: String): String =
    when (status) {
      "fresh" -> "AI interpretation is advisory and separate from indexed facts."
      "stale" ->
          "AI interpretation is stale; source may have changed. Indexed facts remain current."
      "failed" ->
          failure.ifBlank { "AI interpretation failed. Deterministic facts remain available." }
      "running" -> "AI interpretation is running. Deterministic facts remain available."
      else -> "No current AI interpretation is available. Deterministic facts remain available."
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
          ?.let { ProjectSummaryDetail("Components", it) },
      analysis.entryPoints
          .takeIf { it.isNotEmpty() }
          ?.let { ProjectSummaryDetail("Entry points", it) },
      analysis.flows.takeIf { it.isNotEmpty() }?.let { ProjectSummaryDetail("Flows", it) },
      analysis.risks
          .takeIf { it.isNotEmpty() }
          ?.map { risk -> "${risk.severity.ifBlank { "unknown" }.uppercase()} · ${risk.summary}" }
          ?.let { ProjectSummaryDetail("Risks · AI suggestions", it) },
      analysis.nextSteps.takeIf { it.isNotEmpty() }?.let { ProjectSummaryDetail("Next steps", it) },
  )
}

/** Scan-friendly project overview. Indexed facts never depend on model analysis being present. */
@Composable
internal fun ProjectSummaryPane(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
    onWorkspace: (Workspace) -> Unit,
) {
  val presentation = projectSummaryPresentation(overview, project)
  var purposeExpanded by remember(presentation.purpose) { mutableStateOf(false) }
  var detailsExpanded by remember(presentation.details) { mutableStateOf(false) }
  BoxWithConstraints(Modifier.fillMaxSize()) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = workspacePagePadding(maxWidth, vertical = 24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
      item { SummaryHeader(presentation) }
      if (!presentation.hasProject) {
        item {
          SystemStateMessage(
              "No project selected",
              "Import a project to view indexed facts and any available interpretation.",
              modifier = Modifier.fillMaxWidth())
        }
      } else {
        item { SummaryMetricStrip(presentation.projectMetrics) }
        item { SummaryCoverage(presentation.analysisStatus, presentation.coverageMetrics) }
        item {
          SummaryInterpretation(
              presentation = presentation,
              purposeExpanded = purposeExpanded,
              onPurposeExpanded = { purposeExpanded = !purposeExpanded },
              detailsExpanded = detailsExpanded,
              onDetailsExpanded = { detailsExpanded = !detailsExpanded },
          )
        }
        item { SummaryNavigationLinks(onWorkspace) }
      }
    }
  }
}

@Composable
private fun SummaryHeader(presentation: ProjectSummaryPresentation) {
  Row(verticalAlignment = Alignment.CenterVertically) {
    DesktopLineIcon(DesktopIcon.Summary, "Summary", tint = SelectionText, iconSize = 26.dp)
    Spacer(Modifier.width(12.dp))
    Column(Modifier.weight(1f)) {
      Text(
          presentation.projectName,
          color = PrimaryText,
          fontSize = 22.sp,
          fontWeight = FontWeight.SemiBold,
          maxLines = 1,
          overflow = TextOverflow.Ellipsis)
      Text(
          listOf(
                  presentation.projectType,
                  presentation.languages.takeIf { it.isNotBlank() } ?: "Languages unavailable",
                  presentation.buildMetadata)
              .joinToString(" · "),
          color = SecondaryText,
          fontSize = 12.sp,
          maxLines = 2,
          overflow = TextOverflow.Ellipsis,
          modifier = Modifier.padding(top = 4.dp))
    }
  }
}

@Composable
private fun SummaryMetricStrip(metrics: List<ProjectSummaryMetric>) {
  MiniOrcaPanel(Modifier.fillMaxWidth(), contentPadding = PaddingValues(16.dp)) {
    SectionLabel("Project facts")
    Spacer(Modifier.height(16.dp))
    SummaryMetricGrid(metrics)
  }
}

@Composable
private fun SummaryCoverage(status: String, metrics: List<ProjectSummaryMetric>) {
  MiniOrcaPanel(Modifier.fillMaxWidth(), contentPadding = PaddingValues(16.dp)) {
    Row(verticalAlignment = Alignment.CenterVertically) {
      SectionLabel("Analysis coverage")
      Spacer(Modifier.weight(1f))
      StatusBadge(status)
    }
    Spacer(Modifier.height(16.dp))
    SummaryMetricGrid(metrics)
  }
}

@Composable
private fun SummaryMetricGrid(metrics: List<ProjectSummaryMetric>) {
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    val columns = summaryMetricColumnCount(maxWidth)
    Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
      metrics.chunked(columns).forEach { row ->
        Row(Modifier.fillMaxWidth()) {
          row.forEachIndexed { index, metric ->
            if (index > 0) Box(Modifier.height(48.dp).width(1.dp).background(Border))
            SummaryMetric(
                metric = metric,
                modifier = Modifier.weight(1f).padding(start = if (index == 0) 0.dp else 16.dp))
          }
        }
      }
    }
  }
}

@Composable
private fun SummaryMetric(metric: ProjectSummaryMetric, modifier: Modifier = Modifier) {
  Column(modifier) {
    Text(
        metric.value?.toString() ?: "—",
        color = if (metric.value == null) FaintText else summaryMetricTint(metric.tone),
        fontSize = 24.sp,
        fontWeight = FontWeight.Medium)
    Text(
        metric.label,
        color = SecondaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = 4.dp))
  }
}

private fun summaryMetricTint(tone: SummaryMetricTone): Color =
    when (tone) {
      SummaryMetricTone.Fact,
      SummaryMetricTone.Missing -> PrimaryText
      SummaryMetricTone.Verified,
      SummaryMetricTone.Fresh -> Success
      SummaryMetricTone.Suggestion,
      SummaryMetricTone.Running -> SelectionText
      SummaryMetricTone.Stale -> Warning
      SummaryMetricTone.Failed -> Error
    }

@Composable
private fun SummaryInterpretation(
    presentation: ProjectSummaryPresentation,
    purposeExpanded: Boolean,
    onPurposeExpanded: () -> Unit,
    detailsExpanded: Boolean,
    onDetailsExpanded: () -> Unit,
) {
  MiniOrcaPanel(
      Modifier.fillMaxWidth(),
      raised = presentation.analysisStatus in setOf("stale", "failed"),
      contentPadding = PaddingValues(16.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
          SectionLabel("AI interpretation")
          Spacer(Modifier.weight(1f))
          StatusBadge(presentation.analysisStatus)
        }
        if (presentation.analysisStatus != "fresh" || presentation.purpose == null)
            Text(
                presentation.analysisMessage,
                color = summaryStatusTint(presentation.analysisStatus),
                fontSize = 12.sp,
                lineHeight = 18.sp,
                modifier = Modifier.padding(top = 12.dp))
        presentation.purpose?.let { purpose ->
          Text(
              purpose,
              color = PrimaryText,
              fontSize = 13.sp,
              lineHeight = 20.sp,
              maxLines = if (purposeExpanded) Int.MAX_VALUE else 3,
              overflow = if (purposeExpanded) TextOverflow.Clip else TextOverflow.Ellipsis,
              modifier = Modifier.padding(top = 12.dp))
          ChromeButton(
              onClick = onPurposeExpanded,
              modifier = Modifier.padding(top = 8.dp),
              contentPadding = PaddingValues(horizontal = 8.dp, vertical = 4.dp)) {
                Text(if (purposeExpanded) "Show less" else "Show full purpose", fontSize = 11.sp)
              }
        }
        if (presentation.details.isNotEmpty()) {
          IdeDisclosureHeader(
              title = "Interpretation details",
              expanded = detailsExpanded,
              onToggle = onDetailsExpanded,
              stateLabel = "${presentation.details.size} sections",
              modifier = Modifier.padding(top = 12.dp))
          if (detailsExpanded) SummaryDetails(presentation.details)
        }
        EngineeringInsightPanel(
            presentation.engineeringInsight,
            stale = presentation.analysisStatus == "stale",
            scopeLabel = "Project")
      }
}

private fun summaryStatusTint(status: String): Color =
    when (status) {
      "failed" -> Error
      "stale" -> Warning
      else -> SecondaryText
    }

@Composable
private fun SummaryDetails(details: List<ProjectSummaryDetail>) {
  SelectionContainer {
    Column(Modifier.padding(top = 12.dp)) {
      details.forEach { detail ->
        Text(detail.title, color = SecondaryText, fontSize = 11.sp)
        detail.values.forEach { value ->
          Text(
              value,
              color = PrimaryText,
              fontSize = 12.sp,
              lineHeight = 18.sp,
              modifier = Modifier.padding(top = 4.dp))
        }
        Spacer(Modifier.height(12.dp))
      }
    }
  }
}

@Composable
private fun SummaryNavigationLinks(onWorkspace: (Workspace) -> Unit) {
  Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
    ChromeButton(onClick = { onWorkspace(Workspace.Analysis) }) {
      DesktopLineIcon(DesktopIcon.Analysis, "Open Analysis", iconSize = 16.dp)
      Spacer(Modifier.width(6.dp))
      Text("Analysis", fontSize = 11.sp)
    }
    ChromeButton(onClick = { onWorkspace(Workspace.Bugs) }) {
      DesktopLineIcon(DesktopIcon.Problems, "Open Bugs", iconSize = 16.dp)
      Spacer(Modifier.width(6.dp))
      Text("Bugs", fontSize = 11.sp)
    }
  }
}
