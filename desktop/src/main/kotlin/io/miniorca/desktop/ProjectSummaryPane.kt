package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Button
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Project facts are rendered independently from optional model interpretation. */
@Composable
internal fun ProjectSummaryPane(overview: ProjectOverview?, project: ProjectAnalysis?, onWorkspace: (Workspace) -> Unit) {
    val metrics = overview?.metrics
    val analysis = overview?.analysis
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("PROJECT SUMMARY", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(12.dp))
        SummaryHeading("DETERMINISTIC PROJECT FACTS")
        if (metrics == null && project == null) {
            Text("Import a project to view its indexed facts.", color = SecondaryText, fontSize = 13.sp)
        } else {
            val value = metrics ?: ProjectMetrics(project?.type.orEmpty(), project?.buildFile.orEmpty(), project?.fileCount ?: 0, project?.sourceFileCount ?: 0, project?.totalLines ?: 0)
            Text("${value.type.ifBlank { "Unknown" }} · ${value.buildFile.ifBlank { "No build metadata" }}", color = PrimaryText, fontSize = 13.sp)
            Text("${value.fileCount} files · ${value.sourceFileCount} source files · ${value.totalLines} lines", color = SecondaryText, fontSize = 12.sp)
            if (value.languages.isNotEmpty()) Text(value.languages.entries.sortedBy { it.key }.joinToString(" · ") { "${it.key}: ${it.value}" }, color = SecondaryText, fontSize = 12.sp)
            Text("Revision: ${overview?.projectRevision ?: project?.projectRevision.orEmpty()}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 6.dp))
        }
        Spacer(Modifier.height(16.dp))
        SummaryHeading("MODEL INTERPRETATION")
        when (analysis?.status?.lowercase()) {
            "fresh" -> ProjectInterpretation(analysis)
            "stale" -> { Text("Stale model interpretation — refresh analysis after this revision.", color = Warning, fontSize = 13.sp); ProjectInterpretation(analysis) }
            "failed" -> Text(analysis.failure.ifBlank { "Model analysis failed. Deterministic facts remain available." }, color = Error, fontSize = 13.sp)
            else -> Text("No current model interpretation is available. Deterministic facts remain available.", color = SecondaryText, fontSize = 13.sp)
        }
        Spacer(Modifier.height(16.dp))
        SummaryHeading("WORKSPACE COVERAGE")
        val coverage = overview?.analysisCoverage
        Text("Analysis: ${coverage?.fresh ?: 0} fresh · ${coverage?.stale ?: 0} stale · ${coverage?.missing ?: 0} missing", color = SecondaryText, fontSize = 12.sp)
        Text("Findings: ${overview?.findingCounts?.verified ?: 0} verified · ${overview?.findingCounts?.aiSuggestions ?: 0} AI suggestions", color = SecondaryText, fontSize = 12.sp)
        Button(onClick = { onWorkspace(Workspace.Analysis) }, modifier = Modifier.padding(top = 8.dp)) { Text("Open Analysis") }
        Button(onClick = { onWorkspace(Workspace.Bugs) }, modifier = Modifier.padding(top = 6.dp)) { Text("Open Bugs") }
    }
}

@Composable private fun SummaryHeading(value: String) = Text(value, color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(bottom = 6.dp))

@Composable private fun ProjectInterpretation(analysis: StructuredProjectAnalysis) {
    Text(analysis.purpose.ifBlank { "No purpose returned." }, color = PrimaryText, fontSize = 13.sp)
    if (analysis.architecture.isNotBlank()) Text("Architecture: ${analysis.architecture}", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
    projectList("Components", analysis.components)
    projectList("Entry points", analysis.entryPoints)
    projectList("Flows", analysis.flows)
    projectList("Risks (model suggestions)", analysis.risks.map { "${it.severity.uppercase()} · ${it.summary}" })
    projectList("Next steps", analysis.nextSteps)
}

@Composable private fun projectList(label: String, values: List<String>) {
    if (values.isEmpty()) return
    Text(label, color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 8.dp))
    values.forEach { Text("• $it", color = PrimaryText, fontSize = 12.sp) }
}
