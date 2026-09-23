package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

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
  // The list owns the viewport for one project revision; count and run updates retain its position.
  val sectionsList = remember(identity) { LazyListState() }
  val scrollScope = rememberCoroutineScope()
  BoxWithConstraints(Modifier.fillMaxSize().background(EditorCanvas)) {
    val contentWidth = maxWidth - workspacePageHorizontalGutter(maxWidth) * 2
    val sideBySide = contentWidth / fontScale >= 900.dp
    val compactIndex = contentWidth / fontScale < 1100.dp
    val architecture = presentation.details.firstOrNull { it.title == "Architecture" }
    val modules = presentation.details.firstOrNull { it.title == "Packages / modules" }
    val flows = presentation.details.firstOrNull { it.title == "Flows" }
    val insight =
        presentation.engineeringInsight?.let(::engineeringInsightPieces)?.takeIf { it.isNotEmpty() }
    val ownerIdentity = listOf(identity.first, identity.second)
    val sectionEntries =
        if (presentation.hasProject)
            buildList {
              add("Project" to "introduction")
              add("Coverage" to "coverage")
              add("Findings" to "categories")
              if (architecture != null) add("Architecture" to "architecture")
              if (modules != null) add("Packages / modules" to "modules")
              if (insight != null) add("Engineering insight" to "insight")
              if (flows != null) add("Flows" to "flows")
            }
        else emptyList()
    // Use the same key order to construct the list and resolve navigation, including the run strip.
    val itemKeys =
        (if (presentation.hasProject &&
            currentProjectRun(run, project)?.showsProgressOnSummary() == true)
            listOf("run")
        else emptyList()) + sectionEntries.map { it.second }
    val selectedSection = remember(identity) { mutableStateOf("introduction") }
    val requestedSection = remember(identity) { mutableStateOf<String?>(null) }
    LaunchedEffect(identity, sectionsList, itemKeys) {
      snapshotFlow {
            val visible = sectionsList.layoutInfo.visibleItemsInfo
            val requested =
                requestedSection.value?.takeIf { key ->
                  visible.any { item -> itemKeys.getOrNull(item.index) == key }
                }
            requested to sectionsList.firstVisibleItemIndex
          }
          .collect { (requested, position) ->
            if (requested == null) requestedSection.value = null
            selectedSection.value =
                requested
                    ?: when (val key = itemKeys.getOrNull(position)) {
                      "run" -> "introduction"
                      null -> sectionEntries.firstOrNull()?.second.orEmpty()
                      else -> key
                    }
          }
    }
    val indexEntries: @Composable () -> Unit = {
      sectionEntries.forEach { (label, key) ->
        SummaryIndexEntry(label, selectedSection.value == key) {
          scrollScope.launch {
            requestedSection.value = key
            sectionsList.scrollToItem(itemKeys.indexOf(key))
          }
        }
      }
    }
    Column(Modifier.fillMaxSize()) {
      if (!sideBySide) {
        FlowRow(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)) {
              indexEntries()
            }
      }
      Row(Modifier.weight(1f).fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        if (sideBySide) {
          Column(
              Modifier.width(if (compactIndex) 148.dp else 180.dp),
              verticalArrangement = Arrangement.spacedBy(4.dp)) {
                indexEntries()
              }
        }
        LazyColumn(
            Modifier.weight(1f).fillMaxHeight(),
            state = sectionsList,
            contentPadding =
                workspacePagePadding(this@BoxWithConstraints.maxWidth, vertical = 20.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)) {
              if (!presentation.hasProject) {
                item(key = "empty") {
                  SystemStateMessage("No project selected", "", modifier = Modifier.fillMaxWidth())
                }
              } else {
                val currentRun = currentProjectRun(run, project)
                val stripState =
                    analysisState ?: ProjectAnalysisRunState(run = run, sections = sections)
                if ("run" in itemKeys)
                    item(key = "run") {
                      AnalysisRunStrip(
                          AnalysisWorkspacePaneState(project, stripState.copy(run = currentRun)),
                          analysisActions,
                          AnalysisRunStripScope.Summary,
                          Modifier.testTag("summary-analysis-run-strip"))
                    }
                item(key = "introduction") { SummaryIntroduction(presentation) }
                item(key = "coverage") {
                  SummaryCoverage(presentation) { openResults(Workspace.Analysis) }
                }
                item(key = "categories") {
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
                architecture?.let { detail ->
                  item(key = "architecture") {
                    WorkspaceSection(
                        modifier = Modifier.fillMaxWidth().testTag("summary-architecture")) {
                          MermaidDiagram(
                              detail.values.single(),
                              "Architecture",
                              title = "Architecture",
                              ownerIdentity = ownerIdentity + "architecture")
                        }
                  }
                }
                modules?.let { detail ->
                  item(key = "modules") {
                    WorkspaceSection(
                        modifier = Modifier.fillMaxWidth().testTag("summary-modules")) {
                          Text(
                              "Packages / modules",
                              color = ResultAccent,
                              style = IdeTypography.workspaceHeading,
                              modifier = Modifier.semantics { heading() })
                          SummaryModules(detail.values)
                        }
                  }
                }
                insight?.let { pieces ->
                  item(key = "insight") {
                    SummaryEngineeringInsight(
                        pieces,
                        presentation.interpretationStatus == "stale",
                        ownerIdentity,
                        Modifier.fillMaxWidth())
                  }
                }
                flows?.let { detail ->
                  item(key = "flows") {
                    SummaryFlows(detail, ownerIdentity, Modifier.fillMaxWidth())
                  }
                }
              }
            }
      }
    }
  }
}

@Composable
private fun SummaryIndexEntry(label: String, selected: Boolean, onClick: () -> Unit) {
  MiniOrcaButton(
      onClick = onClick,
      modifier =
          Modifier.testTag("summary-index-$label").semantics {
            contentDescription = "Go to $label summary"
            this.selected = selected
          },
      selected = selected,
      tone = if (selected) ActionTone.Navigation else ActionTone.Neutral) {
        Text(label, style = IdeTypography.workspaceMetadata)
      }
}

@Composable
private fun SummaryIntroduction(presentation: ProjectSummaryPresentation) {
  WorkspaceSection(modifier = Modifier.testTag("summary-introduction")) {
    Text(
        presentation.projectName,
        color = PrimaryText,
        fontSize = 24.sp,
        lineHeight = 30.sp,
        fontWeight = FontWeight.SemiBold,
        modifier = Modifier.semantics { heading() })
    presentation.purpose?.let {
      ModelResultContent(it, preview = false, style = IdeTypography.workspaceBody)
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
  }
}

@Composable
private fun SummaryCategories(metrics: List<SummaryIssueMetric>, openResults: (Workspace) -> Unit) {
  Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
    metrics.forEach { metric -> SummaryIssue(metric, { openResults(metric.type.workspace) }) }
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
