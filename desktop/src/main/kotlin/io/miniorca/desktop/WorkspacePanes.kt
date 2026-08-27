package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
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
import androidx.compose.ui.Alignment
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
    val presentation = analyzeAllPresentation(job, coverage)
    LazyColumn(Modifier.fillMaxSize().padding(18.dp)) {
        item {
            Text("PROJECT ANALYSIS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
            Text("Analyze-all is explicit, revision-bound, and never starts during import or reindex.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(12.dp))
            FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
                SectionLabel("ANALYSIS COVERAGE")
                Text(presentation.statusLabel, color = PrimaryText, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 7.dp))
                Text(presentation.statusDetail, color = if (presentation.statusLabel == "Failed") Error else SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            }
            Spacer(Modifier.height(10.dp))
            FocusFlowPanel(Modifier.fillMaxWidth()) {
                SectionLabel("ANALYZE-ALL CONTROLS")
                when (job?.status?.lowercase()) {
                    "running" -> {
                        Text("Processing ${job.files.count { it.status.lowercase() in setOf("completed", "fresh", "success") }} of ${job.files.size} listed files.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
                        Button(onClick = onPause, modifier = Modifier.padding(top = 8.dp)) { Text("Pause") }
                        Button(onClick = onCancel, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel") }
                    }
                    "pausing", "canceling" -> {
                        Text(presentation.controls, color = Warning, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
                        Button(onClick = onCancel, enabled = job.status.lowercase() == "pausing", modifier = Modifier.padding(top = 8.dp)) { Text("Cancel") }
                    }
                    "paused" -> {
                        Text("Completed results remain visible while paused.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
                        RemoteProviderConfirmation(remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed)
                        Button(onClick = { onResume(remoteProviderConfirmed) }, enabled = !remoteProvider || remoteProviderConfirmed, modifier = Modifier.padding(top = 8.dp)) { Text("Resume") }
                        Button(onClick = onCancel, modifier = Modifier.padding(top = 6.dp)) { Text("Cancel") }
                    }
                    else -> {
                        Text(presentation.statusDetail.substringAfter(". "), color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
                        AnalyzeAllStartControls(maxFiles, { maxFiles = it }, maxRetries, { maxRetries = it }, remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed, options, onStart)
                    }
                }
                job?.let { Text("Bounded to ${it.maxFiles} files and ${it.maxRetries} retries per file.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 8.dp)) }
            }
            Spacer(Modifier.height(10.dp))
            SectionLabel("FILE RESULTS")
        }
        if (job?.files.isNullOrEmpty()) {
            item { Text("No file results yet. Start Analyze-all explicitly to populate this list.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp)) }
        } else {
            items(job!!.files, key = { it.path }) { file ->
                FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 7.dp)) {
                    Button(onClick = { onOpen(file.path) }) { Text("Open ${file.path}") }
                    Text("${file.status} · attempt ${file.attempts}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
                    if (file.error.isNotBlank()) Text(file.error, color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
                }
            }
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
    RemoteProviderConfirmation(remoteProvider, remoteConfirmed, onRemoteConfirmed)
    Button(onClick = { onStart(options) }, enabled = !remoteProvider || remoteConfirmed, modifier = Modifier.padding(top = 6.dp)) { Text("Start Analyze-all") }
}

@Composable
internal fun RemoteProviderConfirmation(remoteProvider: Boolean, confirmed: Boolean, onConfirmed: (Boolean) -> Unit) {
    Text(
        contextDestinationLabel(remoteProvider),
        color = if (remoteProvider) Warning else SecondaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = 7.dp),
    )
    if (remoteProvider) androidx.compose.foundation.layout.Row(modifier = Modifier.padding(top = 2.dp)) {
        Checkbox(checked = confirmed, onCheckedChange = onConfirmed)
        Text("Confirm remote destination", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 12.dp))
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
    val grouped = FindingClassification.entries.associateWith { classification -> visible.filter { classifyFinding(it) == classification } }
    LazyColumn(Modifier.fillMaxSize().padding(18.dp)) {
        item {
            Text("PROJECT BUGS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
            Text("Verified/tool-reported issues and AI suggestions remain separate, located, and revision-aware.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(12.dp))
            FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
                SectionLabel("SEARCH AND FILTER")
                TextField(query, { query = it }, label = { Text("Search findings") }, modifier = Modifier.fillMaxWidth().padding(top = 7.dp))
                Row(Modifier.fillMaxWidth().padding(top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                    TextField(source, { source = it }, label = { Text("Source") }, modifier = Modifier.weight(1f))
                    Spacer(Modifier.padding(horizontal = 3.dp))
                    TextField(severity, { severity = it }, label = { Text("Severity") }, modifier = Modifier.weight(1f))
                }
                Row(Modifier.fillMaxWidth().padding(top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                    TextField(freshness, { freshness = it }, label = { Text("Freshness") }, modifier = Modifier.weight(1f))
                    Spacer(Modifier.padding(horizontal = 3.dp))
                    TextField(lifecycle, { lifecycle = it }, label = { Text("Lifecycle") }, modifier = Modifier.weight(1f))
                }
            }
            Spacer(Modifier.height(10.dp))
            FocusFlowPanel(Modifier.fillMaxWidth()) {
                SectionLabel("VERIFIED SCAN")
                Text(progress.summary, color = if (progress.warnings.isNotEmpty()) Warning else SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
                progress.warnings.forEach { warning -> Text("Warning: $warning", color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp)) }
                if (progress.canCancel) {
                    Button(onClick = onCancelScan, enabled = scan?.status?.lowercase() == "running", modifier = Modifier.padding(top = 8.dp)) { Text(if (scan?.status?.lowercase() == "canceling") "Canceling…" else "Cancel verified scan") }
                } else {
                    Button(onClick = onStartScan, modifier = Modifier.padding(top = 8.dp)) { Text("Run verified scan") }
                }
            }
            Spacer(Modifier.height(10.dp))
            SectionLabel("FINDINGS · ${visible.size} MATCHING")
        }
        FindingClassification.entries.forEach { classification ->
            val section = grouped.getValue(classification)
            item {
                SectionLabel(classification.sectionLabel, Modifier.padding(top = 9.dp))
                Text(classification.description, color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 2.dp))
                if (section.isEmpty()) Text("No matching findings.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 5.dp))
            }
            items(section, key = { finding -> "${classification.name}:${finding.id}:${finding.location.path}:${finding.location.startLine}" }) { finding ->
                FindingCard(finding, onOpen, onPrepare, onTriage)
            }
        }
    }
}

@Composable
private fun FindingCard(
    finding: UnifiedFinding,
    onOpen: (UnifiedFinding) -> Unit,
    onPrepare: (UnifiedFinding) -> Unit,
    onTriage: (UnifiedFinding, FindingLifecycleAction) -> Unit,
) {
    FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 7.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text("${finding.severity.ifBlank { "unknown" }.uppercase()} · ${finding.title.ifBlank { "Untitled finding" }}", color = PrimaryText, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
            StatusBadge(finding.freshness.ifBlank { "missing" })
        }
        Text(findingProvenanceLabel(finding), color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 5.dp))
        Text("Location: ${findingLocationLabel(finding)}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 3.dp))
        Text("Status: ${findingStatusLabel(finding)}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 3.dp))
        Text(finding.message.ifBlank { "No message supplied." }, color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
        if (finding.evidence.isNotBlank()) Text("Evidence: ${finding.evidence}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
        Row(Modifier.fillMaxWidth().padding(top = 7.dp), verticalAlignment = Alignment.CenterVertically) {
            Button(onClick = { onOpen(finding) }, enabled = finding.location.path.isNotBlank()) { Text("Open in Editor") }
            Spacer(Modifier.padding(horizontal = 3.dp))
            Button(onClick = { onPrepare(finding) }, enabled = findingCanPrepareFix(finding)) { Text("Prepare fix") }
            findingLifecycleActions(finding).forEach { action ->
                Spacer(Modifier.padding(horizontal = 3.dp))
                Button(onClick = { onTriage(finding, action) }) { Text(action.label) }
            }
        }
    }
}

internal fun findingLocationLabel(finding: UnifiedFinding): String {
    val location = finding.location
    if (location.path.isBlank()) return "project-wide"
    val line = if (location.startLine > 0) ":${location.startLine}" else ""
    val symbol = location.symbol.takeIf { it.isNotBlank() }?.let { " · $it" }.orEmpty()
    return "${location.path}$line$symbol"
}
