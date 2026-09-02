package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
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
import androidx.compose.ui.text.font.FontWeight
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

internal fun projectSummaryPresentation(overview: ProjectOverview?, project: ProjectAnalysis?): ProjectSummaryPresentation {
    val metrics = overview?.metrics
    val value = metrics ?: ProjectMetrics(
        type = project?.type.orEmpty(),
        buildFile = project?.buildFile.orEmpty(),
        fileCount = project?.fileCount ?: 0,
        sourceFileCount = project?.sourceFileCount ?: 0,
        totalLines = project?.totalLines ?: 0,
        languages = project?.languages.orEmpty(),
    )
    val analysis = overview?.analysis
    val analysisStatus = (analysis?.status ?: project?.aiStatus).orEmpty().lowercase().ifBlank { "missing" }
    val analysisMessage = when (analysisStatus) {
        "fresh" -> "Fresh model interpretation is available. It is advisory and separate from indexed facts."
        "stale" -> "Model interpretation is out of date. Refresh analysis before relying on it."
        "failed" -> analysis?.failure?.ifBlank { "Model analysis failed. Deterministic facts remain available." }
            ?: "Model analysis failed. Deterministic facts remain available."
        "running" -> "Model analysis is running; deterministic facts remain available."
        else -> "No current model interpretation is available. Deterministic facts remain available."
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
        coverage = "${coverage?.fresh ?: 0} fresh · ${coverage?.stale ?: 0} stale · ${coverage?.missing ?: 0} missing · ${coverage?.failed ?: 0} failed · ${coverage?.running ?: 0} running",
        findings = "${findingCounts?.verified ?: 0} verified · ${findingCounts?.aiSuggestions ?: 0} AI suggestions",
    )
}

/** Scan-friendly project overview. Indexed facts never depend on model analysis being present. */
@Composable
internal fun ProjectSummaryPane(overview: ProjectOverview?, project: ProjectAnalysis?, onWorkspace: (Workspace) -> Unit) {
    val presentation = projectSummaryPresentation(overview, project)
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("PROJECT SUMMARY", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(12.dp))

        FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
            SectionLabel("DETERMINISTIC PROJECT FACTS")
            if (!presentation.hasProject) {
                Text("Import a project to view its indexed facts.", color = SecondaryText, fontSize = 13.sp, modifier = Modifier.padding(top = 6.dp))
            } else {
                Column(Modifier.padding(top = 6.dp)) {
                    Text("${presentation.projectType} · ${presentation.buildMetadata}", color = PrimaryText, fontSize = 13.sp)
                    if (presentation.languages.isNotBlank()) Text(presentation.languages, color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
                }
            }
        }

        Spacer(Modifier.height(10.dp))
        FocusFlowPanel(Modifier.fillMaxWidth()) {
            SectionLabel("MODEL INTERPRETATION · ADVISORY")
            StatusBadge(presentation.analysisStatus, Modifier.padding(top = 7.dp))
            Text(presentation.analysisMessage, color = if (presentation.analysisStatus == "failed") Error else if (presentation.analysisStatus == "stale") Warning else SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
            if (presentation.analysisStatus == "fresh" || presentation.analysisStatus == "stale") {
                val analysis = overview?.analysis
                ProjectInterpretation(analysis)
            }
        }

        Spacer(Modifier.height(10.dp))
        FocusFlowPanel(Modifier.fillMaxWidth()) {
            SectionLabel("WORKSPACE COVERAGE")
            Text("File analysis: ${presentation.coverage}", color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
            Text("Findings: ${presentation.findings}", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            FocusFlowButton(onClick = { onWorkspace(Workspace.Analysis) }, tone = ActionTone.Navigation, modifier = Modifier.padding(top = 9.dp)) { Text("Open Analysis") }
            FocusFlowButton(onClick = { onWorkspace(Workspace.Bugs) }, tone = ActionTone.Navigation, modifier = Modifier.padding(top = 6.dp)) { Text("Open Bugs") }
        }
    }
}

@Composable
private fun ProjectInterpretation(analysis: StructuredProjectAnalysis?) {
    if (analysis == null) return
    Text(analysis.purpose.ifBlank { "No purpose returned." }, color = PrimaryText, fontSize = 13.sp, modifier = Modifier.padding(top = 8.dp))
    if (analysis.architecture.isNotBlank()) Text("Architecture: ${analysis.architecture}", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
    projectList("Components", analysis.components)
    projectList("Entry points", analysis.entryPoints)
    projectList("Flows", analysis.flows)
    projectList("Risks (AI suggestions)", analysis.risks.map { "${it.severity.uppercase()} · ${it.summary}" })
    projectList("Next steps", analysis.nextSteps)
}

@Composable
private fun projectList(label: String, values: List<String>) {
    if (values.isEmpty()) return
    Text(label, color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 8.dp))
    values.forEach { Text("• $it", color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 2.dp)) }
}
