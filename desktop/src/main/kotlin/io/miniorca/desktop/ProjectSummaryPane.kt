package io.miniorca.desktop

import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
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
    val coverageFresh: Int?,
    val coverageTotal: Int?,
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
  val currentRun =
      run?.takeIf {
        it.identity.projectId == project?.projectId &&
            it.identity.projectRevision == project.projectRevision
      }
  val findings = overview?.findingCounts
  val outdated =
      if (hasCoverage) (coverage?.stale ?: 0) > 0
      else
          normalizedStatus == "stale" ||
              (coverage?.stale ?: 0) > 0 ||
              (run != null && AnalysisResultPageState(AnalysisResultType.Bugs, project, run).stale)
  return ProjectSummaryPresentation(
      hasProject = hasProject,
      projectName = project?.name?.takeIf { it.isNotBlank() } ?: "Project",
      projectType = metrics?.type?.ifBlank { "Unknown project type" } ?: "Unavailable",
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
                  summaryAnalysisMessage(normalizedStatus, analysis?.failure.orEmpty()),
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
      interpretationMessage = summaryAnalysisMessage(normalizedStatus, analysis?.failure.orEmpty()),
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
      coverageFresh = if (hasCoverage) coverage?.fresh else null,
      coverageTotal = if (hasCoverage) coverage?.total else null,
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

@OptIn(ExperimentalLayoutApi::class)
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
  val fontScale = LocalDensity.current.fontScale
  val identity =
      (project?.projectId ?: overview?.projectId) to
          (project?.projectRevision ?: overview?.projectRevision)
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    val pageWidth = maxWidth - 48.dp
    val sideBySide = pageWidth / fontScale >= 760.dp
    val architecture = presentation.details.firstOrNull { it.title == "Architecture" }
    val modules = presentation.details.firstOrNull { it.title == "Packages / modules" }
    val flows = presentation.details.firstOrNull { it.title == "Flows" }
    val insight =
        presentation.engineeringInsight?.let(::engineeringInsightPieces)?.takeIf { it.isNotEmpty() }
    val ownerIdentity = listOf(identity.first, identity.second)
    val sectionsAvailable = buildList {
      add("Coverage")
      if (architecture != null) add("Architecture")
      if (modules != null) add("Packages / modules")
      if (flows != null) add("Flows")
      if (insight != null) add("Engineering insight")
    }
    var selected by remember(identity) { mutableStateOf("Coverage") }
    if (selected !in sectionsAvailable) selected = "Coverage"
    val pageScroll = remember(identity) { ScrollState(0) }
    LaunchedEffect(pageScroll, selected) { pageScroll.scrollTo(0) }
    if (!presentation.hasProject) {
      SystemStateMessage(
          "No project selected", "", modifier = Modifier.fillMaxWidth().padding(24.dp))
    } else {
      Column(Modifier.fillMaxSize().verticalScroll(pageScroll).testTag("summary-page-scroll")) {
        val currentRun = currentProjectRun(run, project)
        SummaryProjectHeader(presentation)
        SummaryIntroduction(presentation)
        if (currentRun?.showsProgressOnSummary() == true) {
          AnalysisRunStrip(
              AnalysisWorkspacePaneState(
                  project,
                  (analysisState ?: ProjectAnalysisRunState(run = run, sections = sections)).copy(
                      run = currentRun)),
              analysisActions,
              AnalysisRunStripScope.Summary,
              Modifier.fillMaxWidth()
                  .padding(horizontal = 24.dp)
                  .testTag("summary-analysis-run-strip"))
        }
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)) {
              if (sideBySide) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                  Column(
                      Modifier.width(290.dp)
                          .testTag("summary-outline-panel")
                          .background(Panel, RoundedCornerShape(14.dp))
                          .padding(16.dp),
                      verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(
                            "Summary",
                            color = ResultAccent,
                            style = IdeTypography.workspaceHeading,
                            modifier = Modifier.semantics { heading() }.padding(bottom = 4.dp))
                        SummaryOutlineRow(
                            "Coverage",
                            coverageOutlineValue(presentation),
                            selected == "Coverage") {
                              selected = "Coverage"
                            }
                        presentation.issueMetrics.forEach { metric ->
                          SummaryOutlineRow(metric.label, metric.value?.toString() ?: "—", false) {
                            openResults(metric.type.workspace)
                          }
                        }
                        sectionsAvailable.drop(1).forEach { label ->
                          SummaryOutlineRow(
                              label,
                              if (label == "Packages / modules")
                                  modules?.values?.size?.toString().orEmpty()
                              else if (label == "Flows") flows?.values?.size?.toString().orEmpty()
                              else "›",
                              selected == label) {
                                selected = label
                              }
                        }
                      }
                  Column(
                      Modifier.weight(1f)
                          .padding(bottom = 20.dp)
                          .testTag("summary-detail-panel")
                          .background(Panel, RoundedCornerShape(14.dp))
                          .padding(16.dp),
                      verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        SummarySelectedDetail(
                            selected,
                            presentation,
                            architecture,
                            modules,
                            flows,
                            insight,
                            ownerIdentity,
                            openResults) {
                              selected = "Architecture"
                            }
                      }
                }
              } else {
                val twoOutlineColumns = (pageWidth - 32.dp) / fontScale >= 600.dp
                Column(
                    Modifier.fillMaxWidth()
                        .testTag("summary-outline-panel")
                        .background(Panel, RoundedCornerShape(14.dp))
                        .padding(16.dp)) {
                      Text(
                          "Summary",
                          color = ResultAccent,
                          style = IdeTypography.workspaceHeading,
                          modifier = Modifier.semantics { heading() }.padding(bottom = 4.dp))
                      FlowRow(
                          Modifier.fillMaxWidth(),
                          horizontalArrangement = Arrangement.spacedBy(4.dp),
                          verticalArrangement = Arrangement.spacedBy(4.dp),
                          maxItemsInEachRow = if (twoOutlineColumns) 2 else 1) {
                            SummaryOutlineRow(
                                "Coverage",
                                coverageOutlineValue(presentation),
                                selected == "Coverage",
                                Modifier.weight(1f)) {
                                  selected = "Coverage"
                                }
                            presentation.issueMetrics.forEach { metric ->
                              SummaryOutlineRow(
                                  metric.label,
                                  metric.value?.toString() ?: "—",
                                  false,
                                  Modifier.weight(1f)) {
                                    openResults(metric.type.workspace)
                                  }
                            }
                            sectionsAvailable.drop(1).forEach { label ->
                              SummaryOutlineRow(
                                  label,
                                  if (label == "Packages / modules")
                                      modules?.values?.size?.toString().orEmpty()
                                  else if (label == "Flows")
                                      flows?.values?.size?.toString().orEmpty()
                                  else "›",
                                  selected == label,
                                  Modifier.weight(1f)) {
                                    selected = label
                                  }
                            }
                          }
                    }
                Column(
                    Modifier.fillMaxWidth()
                        .padding(bottom = 20.dp)
                        .testTag("summary-detail-panel")
                        .background(Panel, RoundedCornerShape(14.dp))
                        .padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)) {
                      SummarySelectedDetail(
                          selected,
                          presentation,
                          architecture,
                          modules,
                          flows,
                          insight,
                          ownerIdentity,
                          openResults) {
                            selected = "Architecture"
                          }
                    }
              }
            }
      }
    }
  }
}

@Composable
private fun SummaryOutlineRow(
    label: String,
    value: String,
    selected: Boolean,
    modifier: Modifier = Modifier,
    onClick: () -> Unit
) {
  MiniOrcaButton(
      onClick = onClick,
      modifier =
          modifier.fillMaxWidth().testTag("summary-index-$label").semantics {
            this.selected = selected
          },
      accessibleName =
          if (label in listOf("Bugs", "Performance", "Security")) "Open $label results, $value"
          else "Select $label summary, $value",
      selected = selected,
      tone = if (selected) ActionTone.Navigation else ActionTone.Neutral) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
          Text(label, style = IdeTypography.workspaceMetadata)
          if (value.isNotBlank())
              Text(value, style = IdeTypography.workspaceMetadata, color = SecondaryText)
        }
      }
}

private fun coverageOutlineValue(presentation: ProjectSummaryPresentation) =
    if (presentation.coverageFresh == null || presentation.coverageTotal == null) "—"
    else "${presentation.coverageFresh} / ${presentation.coverageTotal}"

@Composable
private fun SummarySelectedDetail(
    selected: String,
    presentation: ProjectSummaryPresentation,
    architecture: ProjectSummaryDetail?,
    modules: ProjectSummaryDetail?,
    flows: ProjectSummaryDetail?,
    insight: List<EngineeringInsightPiece>?,
    ownerIdentity: List<String?>,
    openResults: (Workspace) -> Unit,
    selectArchitecture: () -> Unit,
) {
  when (selected) {
    "Coverage" -> {
      SummaryCoverage(presentation) { openResults(Workspace.Analysis) }
      Text("Finding categories", color = ResultAccent, style = IdeTypography.workspaceHeading)
      SummaryCategories(
          presentation.issueMetrics,
          openResults,
          architecture != null,
          architecture?.values?.firstOrNull().orEmpty(),
          selectArchitecture)
      Text(
          "Overall findings · " +
              presentation.findingMetrics.joinToString(" · ") {
                "${it.value ?: "—"} ${if (it.label == "AI suggestions") it.label else it.label.lowercase()}"
              },
          color = SecondaryText,
          style = IdeTypography.workspaceMetadata,
          modifier = Modifier.testTag("summary-findings-provenance"))
    }
    "Architecture" ->
        architecture?.let {
          WorkspaceSection(modifier = Modifier.fillMaxWidth().testTag("summary-architecture")) {
            MermaidDiagram(
                it.values.single(),
                "Architecture",
                title = "Architecture",
                ownerIdentity = ownerIdentity + "architecture")
          }
        }
    "Packages / modules" ->
        modules?.let {
          WorkspaceSection(modifier = Modifier.fillMaxWidth().testTag("summary-modules")) {
            Text(
                "Packages / modules",
                color = ResultAccent,
                style = IdeTypography.workspaceHeading,
                modifier = Modifier.semantics { heading() })
            SummaryModules(it.values)
          }
        }
    "Flows" -> flows?.let { SummaryFlows(it, ownerIdentity, Modifier.fillMaxWidth()) }
    "Engineering insight" ->
        insight?.let {
          SummaryEngineeringInsight(
              it,
              presentation.interpretationStatus == "stale",
              ownerIdentity,
              Modifier.fillMaxWidth())
        }
  }
}

@Composable
private fun SummaryProjectHeader(presentation: ProjectSummaryPresentation) {
  BoxWithConstraints(Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 12.dp)) {
    val stackHeader = maxWidth / LocalDensity.current.fontScale < 640.dp
    if (stackHeader) {
      Column(
          Modifier.fillMaxWidth().testTag("summary-project-header"),
          verticalArrangement = Arrangement.spacedBy(4.dp)) {
            ProjectSummaryHeaderName(presentation.projectName)
            ProjectSummaryHeaderMetadata(presentation)
          }
    } else {
      Row(
          Modifier.fillMaxWidth().testTag("summary-project-header"),
          horizontalArrangement = Arrangement.spacedBy(16.dp),
          verticalAlignment = Alignment.CenterVertically) {
            ProjectSummaryHeaderName(presentation.projectName, Modifier.weight(1f))
            ProjectSummaryHeaderMetadata(presentation, Modifier.weight(1f))
          }
    }
  }
  Box(
      Modifier.fillMaxWidth()
          .padding(horizontal = 24.dp)
          .height(1.dp)
          .background(SecondaryText.copy(alpha = 0.35f))
          .testTag("summary-header-separator"))
}

@Composable
private fun ProjectSummaryHeaderName(name: String, modifier: Modifier = Modifier) {
  Text(
      name,
      color = ResultAccent,
      fontSize = 24.sp,
      lineHeight = 30.sp,
      fontWeight = FontWeight.SemiBold,
      modifier = modifier.semantics { heading() })
}

@Composable
private fun ProjectSummaryHeaderMetadata(
    presentation: ProjectSummaryPresentation,
    modifier: Modifier = Modifier,
) {
  val metadata =
      listOf(presentation.projectType, presentation.buildMetadata)
          .filter(String::isNotBlank)
          .joinToString(" · ")
  if (metadata.isNotBlank()) {
    Text(
        metadata,
        color = SecondaryText,
        style = IdeTypography.workspaceMetadata,
        modifier = modifier)
  }
}

@Composable
private fun SummaryIntroduction(presentation: ProjectSummaryPresentation) {
  WorkspaceSection(modifier = Modifier.testTag("summary-introduction")) {
    presentation.purpose?.let {
      ModelResultContent(it, preview = true, style = IdeTypography.workspaceBody)
    }
    if (presentation.interpretationStatus != "fresh") {
      Text(
          presentation.interpretationMessage,
          color = if (presentation.interpretationStatus == "failed") Error else SecondaryText,
          style = IdeTypography.workspaceMetadata,
          modifier = Modifier.testTag("summary-interpretation-status"))
    } else if (presentation.purpose == null) {
      Text("Project description unavailable", color = SecondaryText, style = IdeTypography.body)
    }
    val facts =
        listOf(presentation.projectType, presentation.buildMetadata) +
            presentation.projectMetrics.map { metric ->
              "${metric.value?.let { "%,d".format(java.util.Locale.ROOT, it) } ?: "—"} ${if (metric.label == "Total lines") "lines" else metric.label.lowercase()}"
            } +
            presentation.languages.split(" · ").filter {
              it.isNotBlank() && !it.equals(presentation.projectType, ignoreCase = true)
            }
    Text(facts.joinToString(" · "), color = SecondaryText, style = IdeTypography.workspaceMetadata)
    SummaryAnalysisStatus(presentation)
    presentation.analysisMessage
        .lines()
        .filter { it.startsWith("Analysis run") }
        .forEach { message ->
          Text(
              message,
              color = if (message.contains("failed")) Error else SecondaryText,
              style = IdeTypography.workspaceMetadata)
        }
  }
}

@Composable
private fun SummaryCategories(
    metrics: List<SummaryIssueMetric>,
    openResults: (Workspace) -> Unit,
    architectureAvailable: Boolean,
    architecturePreview: String,
    selectArchitecture: () -> Unit
) {
  BoxWithConstraints(Modifier.fillMaxWidth()) {
    val columns = if ((maxWidth - 32.dp) / LocalDensity.current.fontScale >= 600.dp) 2 else 1
    FlowRow(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
        maxItemsInEachRow = columns) {
          metrics.forEach { metric ->
            SummaryIssue(metric, { openResults(metric.type.workspace) }, Modifier.weight(1f))
          }
          MiniOrcaButton(
              onClick = selectArchitecture,
              modifier = Modifier.weight(1f).fillMaxWidth().testTag("summary-architecture-preview"),
              enabled = architectureAvailable,
              accessibleName = "Select Architecture summary from preview",
              tone = ActionTone.Neutral) {
                Column {
                  Text("Architecture", color = PrimaryText, style = IdeTypography.workspaceHeading)
                  Text(
                      if (architectureAvailable) architecturePreview
                      else "Architecture unavailable",
                      color = SecondaryText,
                      style = IdeTypography.compactBody,
                      maxLines = 3,
                      overflow = TextOverflow.Ellipsis)
                }
              }
        }
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
