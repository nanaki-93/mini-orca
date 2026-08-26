package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Button
import androidx.compose.material.Checkbox
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

@Composable
internal fun AnalysisWorkspacePane(
    job: AnalyzeAllJob?,
    coverage: AnalysisCoverage?,
    remoteProvider: Boolean,
    remoteProviderConfirmed: Boolean,
    onRemoteProviderConfirmed: (Boolean) -> Unit,
    onStart: (AnalyzeAllRunOptions) -> Unit,
    onPause: () -> Unit,
    onResume: (Boolean) -> Unit,
    onCancel: () -> Unit,
    onOpen: (String) -> Unit,
) {
    var maxFiles by remember { mutableStateOf(defaultAnalyzeAllFileLimit.toString()) }
    var maxRetries by remember { mutableStateOf(defaultAnalyzeAllRetryLimit.toString()) }
    val options = AnalyzeAllRunOptions(
        maxFiles = maxFiles.toIntOrNull() ?: defaultAnalyzeAllFileLimit,
        maxRetries = maxRetries.toIntOrNull() ?: defaultAnalyzeAllRetryLimit,
        confirmRemoteProvider = remoteProviderConfirmed,
    ).bounded()
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("PROJECT ANALYSIS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        Text("${coverage?.fresh ?: 0} fresh · ${coverage?.stale ?: 0} stale · ${coverage?.missing ?: 0} missing · ${coverage?.running ?: 0} running · ${coverage?.failed ?: 0} failed", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
        Spacer(Modifier.height(10.dp))
        when (job?.status?.lowercase()) {
            "running" -> { Text("Analyze-all running", color = PrimaryText); Button(onClick = onPause) { Text("Pause") }; Button(onClick = onCancel, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel") } }
            "paused" -> {
                Text("Analyze-all paused; completed results remain visible.", color = SecondaryText)
                if (remoteProvider) RemoteProviderConfirmation(remoteProviderConfirmed, onRemoteProviderConfirmed)
                Button(onClick = { onResume(remoteProviderConfirmed) }, enabled = !remoteProvider || remoteProviderConfirmed) { Text("Resume") }
                Button(onClick = onCancel, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel") }
            }
            "canceled" -> { Text("Analyze-all canceled; start a new explicit job to continue.", color = SecondaryText); AnalyzeAllStartControls(maxFiles, { maxFiles = it }, maxRetries, { maxRetries = it }, remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed, options, onStart) }
            else -> { Text(if (job == null) "No Analyze-all job (204 No Content). Import and reindex never start one automatically." else "Analyze-all ${job.status}; stale jobs cannot resume on a newer revision.", color = SecondaryText); AnalyzeAllStartControls(maxFiles, { maxFiles = it }, maxRetries, { maxRetries = it }, remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed, options, onStart) }
        }
        job?.let { Text("Bounded to ${it.maxFiles} files and ${it.maxRetries} retries per file.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 8.dp)) }
        job?.files.orEmpty().forEach { file ->
            Button(onClick = { onOpen(file.path) }, modifier = Modifier.padding(top = 6.dp)) { Text("${file.path} · ${file.status} · attempt ${file.attempts}") }
            if (file.error.isNotBlank()) Text(file.error, color = Error, fontSize = 11.sp)
        }
    }
}

@Composable
private fun AnalyzeAllStartControls(
    maxFiles: String,
    onMaxFiles: (String) -> Unit,
    maxRetries: String,
    onMaxRetries: (String) -> Unit,
    remoteProvider: Boolean,
    remoteConfirmed: Boolean,
    onRemoteConfirmed: (Boolean) -> Unit,
    options: AnalyzeAllRunOptions,
    onStart: (AnalyzeAllRunOptions) -> Unit,
) {
    TextField(maxFiles, onMaxFiles, label = { Text("File limit (1–500)") }, modifier = Modifier.padding(top = 6.dp))
    TextField(maxRetries, onMaxRetries, label = { Text("Retry limit (0–3)") }, modifier = Modifier.padding(top = 6.dp))
    if (remoteProvider) RemoteProviderConfirmation(remoteConfirmed, onRemoteConfirmed)
    Button(onClick = { onStart(options) }, enabled = !remoteProvider || remoteConfirmed, modifier = Modifier.padding(top = 6.dp)) { Text("Start Analyze-all") }
}

@Composable
internal fun RemoteProviderConfirmation(confirmed: Boolean, onConfirmed: (Boolean) -> Unit) {
    androidx.compose.foundation.layout.Row(modifier = Modifier.padding(top = 6.dp)) {
        Checkbox(checked = confirmed, onCheckedChange = onConfirmed)
        Text("Confirm if the configured provider is remote", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 12.dp))
    }
}

@Composable
internal fun BugsWorkspacePane(
    findings: List<UnifiedFinding>,
    scan: GoScanReport?,
    onOpen: (UnifiedFinding) -> Unit,
    onPrepare: (UnifiedFinding) -> Unit,
    onTriage: (UnifiedFinding, FindingLifecycleAction) -> Unit,
    onStartScan: () -> Unit,
    onCancelScan: () -> Unit,
) {
    var query by remember { mutableStateOf("") }
    var source by remember { mutableStateOf("") }
    var severity by remember { mutableStateOf("") }
    var freshness by remember { mutableStateOf("") }
    var lifecycle by remember { mutableStateOf("") }
    val visible = filterFindings(findings, BugsFilters(query, source, severity, freshness, lifecycle))
    val progress = verifiedScanProgress(scan)
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("PROJECT BUGS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        Text("Verified/tool-reported issues are isolated scan results. AI suggestions are model interpretation.", color = SecondaryText, fontSize = 12.sp)
        TextField(query, { query = it }, label = { Text("Search findings") }, modifier = Modifier.padding(top = 10.dp))
        TextField(source, { source = it }, label = { Text("Source filter (for example: vet, test, ai)") }, modifier = Modifier.padding(top = 6.dp))
        TextField(severity, { severity = it }, label = { Text("Severity filter") }, modifier = Modifier.padding(top = 6.dp))
        TextField(freshness, { freshness = it }, label = { Text("Freshness filter") }, modifier = Modifier.padding(top = 6.dp))
        TextField(lifecycle, { lifecycle = it }, label = { Text("Lifecycle filter") }, modifier = Modifier.padding(top = 6.dp))
        Text(progress.summary, color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 10.dp))
        progress.warnings.forEach { warning -> Text("Warning: $warning", color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp)) }
        if (progress.canCancel) Button(onClick = onCancelScan, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel verified scan") }
        else Button(onClick = onStartScan, modifier = Modifier.padding(top = 6.dp)) { Text("Run verified scan") }
        FindingSection(FindingClassification.Verified, visible.filter { classifyFinding(it) == FindingClassification.Verified }, onOpen, onPrepare, onTriage)
        FindingSection(FindingClassification.Suggested, visible.filter { classifyFinding(it) == FindingClassification.Suggested }, onOpen, onPrepare, onTriage)
        FindingSection(FindingClassification.Unclassified, visible.filter { classifyFinding(it) == FindingClassification.Unclassified }, onOpen, onPrepare, onTriage)
    }
}

@Composable private fun FindingSection(classification: FindingClassification, findings: List<UnifiedFinding>, onOpen: (UnifiedFinding) -> Unit, onPrepare: (UnifiedFinding) -> Unit, onTriage: (UnifiedFinding, FindingLifecycleAction) -> Unit) {
    Text(classification.sectionLabel, color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 16.dp))
    Text(classification.description, color = SecondaryText, fontSize = 11.sp)
    if (findings.isEmpty()) Text("None", color = SecondaryText, fontSize = 12.sp)
    findings.forEach { finding ->
        Text("${finding.severity.uppercase()} · ${finding.title}", color = PrimaryText, fontSize = 13.sp, modifier = Modifier.padding(top = 8.dp))
        Text("Provenance: ${finding.source.ifBlank { "unknown" }} · Confidence: ${finding.confidence.ifBlank { "unknown" }}", color = SecondaryText, fontSize = 11.sp)
        Text("Location: ${findingLocationLabel(finding)} · Status: ${finding.status.ifBlank { "unknown" }} · Freshness: ${finding.freshness.ifBlank { "unknown" }}", color = SecondaryText, fontSize = 11.sp)
        Text("Revision: ${finding.projectRevision.ifBlank { "unknown" }}", color = SecondaryText, fontSize = 11.sp)
        Text(finding.message, color = SecondaryText, fontSize = 12.sp)
        if (finding.evidence.isNotBlank()) Text("Evidence: ${finding.evidence}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 2.dp))
        Button(onClick = { onOpen(finding) }, enabled = finding.location.path.isNotBlank(), modifier = Modifier.padding(top = 4.dp)) { Text("Open in Editor") }
        Button(onClick = { onPrepare(finding) }, enabled = findingCanPrepareFix(finding), modifier = Modifier.padding(start = 6.dp)) { Text("Prepare fix") }
        findingLifecycleActions(finding).forEach { action ->
            Button(onClick = { onTriage(finding, action) }, modifier = Modifier.padding(start = 6.dp)) { Text(action.label) }
        }
    }
}

private fun findingLocationLabel(finding: UnifiedFinding): String {
    val location = finding.location
    if (location.path.isBlank()) return "project-wide"
    val line = if (location.startLine > 0) ":${location.startLine}" else ""
    val symbol = location.symbol.takeIf { it.isNotBlank() }?.let { " · $it" }.orEmpty()
    return "${location.path}$line$symbol"
}
