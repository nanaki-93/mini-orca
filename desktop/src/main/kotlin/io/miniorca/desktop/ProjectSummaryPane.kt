package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class ProjectSummaryPresentation(
    val hasProject: Boolean,
    val projectType: String,
    val buildMetadata: String,
    val languages: String,
    val analysisStatus: String,
    val analysisMessage: String,
    val coverage: String,
    val findings: String,
)

internal fun projectSummaryPresentation(
    overview: ProjectOverview?,
    project: ProjectAnalysis?
): ProjectSummaryPresentation {
  val metrics = overview?.metrics
  val value =
      metrics
          ?: ProjectMetrics(
              type = project?.type.orEmpty(),
              buildFile = project?.buildFile.orEmpty(),
              fileCount = project?.fileCount ?: 0,
              sourceFileCount = project?.sourceFileCount ?: 0,
              totalLines = project?.totalLines ?: 0,
              languages = project?.languages.orEmpty(),
          )
  val analysis = overview?.analysis
  val analysisStatus =
      (analysis?.status ?: project?.aiStatus).orEmpty().lowercase().ifBlank { "missing" }
  val analysisMessage =
      when (analysisStatus) {
        "fresh" ->
            "Fresh model interpretation is available. It is advisory and separate from indexed facts."
        "stale" -> "Model interpretation is out of date. Refresh analysis before relying on it."
        "failed" ->
            analysis?.failure?.ifBlank {
              "Model analysis failed. Deterministic facts remain available."
            } ?: "Model analysis failed. Deterministic facts remain available."
        "running" -> "Model analysis is running; deterministic facts remain available."
        else ->
            "No current model interpretation is available. Deterministic facts remain available."
      }
  val coverage = overview?.analysisCoverage
  val findingCounts = overview?.findingCounts
  return ProjectSummaryPresentation(
      hasProject = overview != null || project != null,
      projectType = value.type.ifBlank { "Unknown project type" },
      buildMetadata = value.buildFile.ifBlank { "No build metadata" },
      languages = value.languages.keys.sorted().joinToString(" · "),
      analysisStatus = analysisStatus,
      analysisMessage = analysisMessage,
      coverage =
          "${coverage?.fresh ?: 0} fresh · ${coverage?.stale ?: 0} stale · ${coverage?.missing ?: 0} missing · ${coverage?.failed ?: 0} failed · ${coverage?.running ?: 0} running",
      findings =
          "${findingCounts?.verified ?: 0} verified · ${findingCounts?.aiSuggestions ?: 0} AI suggestions",
  )
}

/** Scan-friendly project overview. Indexed facts never depend on model analysis being present. */
@Composable
internal fun ProjectSummaryPane(
    overview: ProjectOverview?,
    project: ProjectAnalysis?,
    onWorkspace: (Workspace) -> Unit
) {
  val presentation = projectSummaryPresentation(overview, project)
  Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(12.dp)) {
    WorkspacePaneHeader("Summary")
    Spacer(Modifier.height(8.dp))
    if (!presentation.hasProject) {
      SystemStateMessage(
          "No project selected",
          "Import a project to view its indexed facts.",
          modifier = Modifier.fillMaxWidth(),
      )
    } else {
      MiniOrcaPanel(
          Modifier.fillMaxWidth(), contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Project facts")
            CompactKeyValueRows(
                listOf(
                    "Type" to presentation.projectType,
                    "Build" to presentation.buildMetadata,
                    "Languages" to presentation.languages.ifBlank { "Not detected" },
                ),
                modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
            )
          }

      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(),
          raised = presentation.analysisStatus in setOf("fresh", "stale", "failed"),
          contentPadding = PaddingValues(MiniOrcaSpacing.standard),
      ) {
        SectionLabel("Interpretation · advisory")
        StatusBadge(presentation.analysisStatus, Modifier.padding(top = MiniOrcaSpacing.standard))
        Text(
            presentation.analysisMessage,
            color =
                if (presentation.analysisStatus == "failed") Error
                else if (presentation.analysisStatus == "stale") Warning else SecondaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
        if (presentation.analysisStatus == "fresh" || presentation.analysisStatus == "stale") {
          ProjectInterpretation(overview?.analysis)
        }
      }

      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(), contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Workspace coverage")
            CompactKeyValueRows(
                listOf(
                    "File analysis" to presentation.coverage, "Findings" to presentation.findings),
                modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
            )
            ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard)) {
              MiniOrcaButton(
                  onClick = { onWorkspace(Workspace.Analysis) },
                  tone = ActionTone.Navigation,
                  density = ButtonDensity.Toolbar) {
                    Text("Open Analysis", fontSize = 11.sp)
                  }
              MiniOrcaButton(
                  onClick = { onWorkspace(Workspace.Bugs) },
                  tone = ActionTone.Navigation,
                  density = ButtonDensity.Toolbar) {
                    Text("Open Bugs", fontSize = 11.sp)
                  }
            }
          }
    }
  }
}

@Composable
private fun ProjectInterpretation(analysis: StructuredProjectAnalysis?) {
  if (analysis == null) return
  Text(
      analysis.purpose.ifBlank { "No purpose returned." },
      color = PrimaryText,
      fontSize = 12.sp,
      modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
  if (analysis.architecture.isNotBlank())
      Text(
          "Architecture: ${analysis.architecture}",
          color = SecondaryText,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
  projectList("Components", analysis.components)
  projectList("Entry points", analysis.entryPoints)
  projectList("Flows", analysis.flows)
  projectList(
      "Risks (AI suggestions)", analysis.risks.map { "${it.severity.uppercase()} · ${it.summary}" })
  projectList("Next steps", analysis.nextSteps)
  EngineeringInsightPanel(
      analysis.engineeringInsight,
      stale = analysis.status.equals("stale", ignoreCase = true),
      scopeLabel = "Project")
}

@Composable
private fun projectList(label: String, values: List<String>) {
  if (values.isEmpty()) return
  Text(
      label,
      color = SecondaryText,
      fontSize = 11.sp,
      modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
  values.forEach {
    Text(
        "• $it",
        color = PrimaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
  }
}
