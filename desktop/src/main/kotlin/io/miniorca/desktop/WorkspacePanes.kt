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
import androidx.compose.material.TextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

fun shouldPollAnalyzeAll(job: AnalyzeAllJob?): Boolean = job?.status?.lowercase() in setOf("running", "pausing", "canceling")
fun findingCanPrepareFix(finding: UnifiedFinding): Boolean = finding.freshness.lowercase() == "fresh" && finding.location.path.isNotBlank()

@Composable
internal fun AnalysisWorkspacePane(job: AnalyzeAllJob?, coverage: AnalysisCoverage?, onStart: () -> Unit, onPause: () -> Unit, onResume: () -> Unit, onCancel: () -> Unit, onOpen: (String) -> Unit) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("PROJECT ANALYSIS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        Text("${coverage?.fresh ?: 0} fresh · ${coverage?.stale ?: 0} stale · ${coverage?.missing ?: 0} missing · ${coverage?.running ?: 0} running · ${coverage?.failed ?: 0} failed", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
        Spacer(Modifier.height(10.dp))
        when (job?.status?.lowercase()) {
            "running" -> { Text("Analyze-all running", color = PrimaryText); Button(onClick = onPause) { Text("Pause") }; Button(onClick = onCancel, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel") } }
            "paused" -> { Text("Analyze-all paused; completed results remain visible.", color = SecondaryText); Button(onClick = onResume) { Text("Resume") }; Button(onClick = onCancel, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel") } }
            "canceled" -> { Text("Analyze-all canceled; start a new explicit job to continue.", color = SecondaryText); Button(onClick = onStart) { Text("Start Analyze-all") } }
            else -> { Text(if (job == null) "No Analyze-all job. Import and reindex never start one automatically." else "Analyze-all ${job.status}.", color = SecondaryText); Button(onClick = onStart) { Text("Start Analyze-all") } }
        }
        job?.files.orEmpty().forEach { file ->
            Button(onClick = { onOpen(file.path) }, modifier = Modifier.padding(top = 6.dp)) { Text("${file.path} · ${file.status} · attempt ${file.attempts}") }
            if (file.error.isNotBlank()) Text(file.error, color = Error, fontSize = 11.sp)
        }
    }
}

@Composable
internal fun BugsWorkspacePane(findings: List<UnifiedFinding>, scan: GoScanReport?, onOpen: (UnifiedFinding) -> Unit, onPrepare: (UnifiedFinding) -> Unit, onStartScan: () -> Unit, onCancelScan: () -> Unit) {
    var query by remember { mutableStateOf("") }
    var source by remember { mutableStateOf("all") }
    val visible = findings.filter { (query.isBlank() || listOf(it.title, it.message, it.location.path, it.location.symbol).any { value -> value.contains(query, true) }) && (source == "all" || it.confidence == source) }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("PROJECT BUGS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        Text("Verified/tool-reported issues and AI suggestions are separate confidence classes.", color = SecondaryText, fontSize = 12.sp)
        TextField(query, { query = it }, label = { Text("Search findings") }, modifier = Modifier.padding(top = 10.dp))
        Button(onClick = { source = if (source == "all") "tool_reported" else if (source == "tool_reported") "suggested" else "all" }, modifier = Modifier.padding(top = 6.dp)) { Text("Source: $source") }
        if (scan?.status?.lowercase() == "running") Button(onClick = onCancelScan, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel verified scan") }
        else Button(onClick = onStartScan, modifier = Modifier.padding(top = 6.dp)) { Text("Run verified scan") }
        FindingSection("VERIFIED / TOOL-REPORTED", visible.filter { it.confidence == "tool_reported" }, onOpen, onPrepare)
        FindingSection("AI SUGGESTIONS", visible.filter { it.confidence == "suggested" }, onOpen, onPrepare)
    }
}

@Composable private fun FindingSection(title: String, findings: List<UnifiedFinding>, onOpen: (UnifiedFinding) -> Unit, onPrepare: (UnifiedFinding) -> Unit) {
    Text(title, color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 16.dp))
    if (findings.isEmpty()) Text("None", color = SecondaryText, fontSize = 12.sp)
    findings.forEach { finding ->
        Text("${finding.severity.uppercase()} · ${finding.title}", color = PrimaryText, fontSize = 13.sp, modifier = Modifier.padding(top = 8.dp))
        Text("${finding.location.path}:${finding.location.startLine} · ${finding.status} · ${finding.freshness}", color = SecondaryText, fontSize = 11.sp)
        Text(finding.message, color = SecondaryText, fontSize = 12.sp)
        Button(onClick = { onOpen(finding) }, enabled = finding.location.path.isNotBlank(), modifier = Modifier.padding(top = 4.dp)) { Text("Open in Editor") }
        Button(onClick = { onPrepare(finding) }, enabled = findingCanPrepareFix(finding), modifier = Modifier.padding(start = 6.dp)) { Text("Prepare fix") }
    }
}
