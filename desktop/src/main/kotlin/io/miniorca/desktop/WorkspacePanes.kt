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
import androidx.compose.material.Checkbox
import androidx.compose.material.Text
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
            Spacer(Modifier.height(12.dp))
            FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
                SectionLabel("PROJECT COVERAGE")
                Text(
                    "Total: ${presentation.coverage.total} · Fresh: ${presentation.coverage.fresh} · Stale: ${presentation.coverage.stale}",
                    color = PrimaryText,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.padding(top = 7.dp),
                )
                Text(
                    "Missing: ${presentation.coverage.missing} · Running: ${presentation.coverage.running} · Failed: ${presentation.coverage.failed}",
                    color = SecondaryText,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
            Spacer(Modifier.height(10.dp))
            FocusFlowPanel(Modifier.fillMaxWidth()) {
                SectionLabel("CURRENT / LAST RUN")
                Text(presentation.run.statusLabel, color = if (presentation.run.statusLabel == "Failed") Error else PrimaryText, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 7.dp))
                Text(presentation.run.statusDetail, color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
                Text(
                    "Candidates: ${presentation.run.candidates} · Completed: ${presentation.run.completed} · Failed: ${presentation.run.failed} · Running: ${presentation.run.running} · Remaining: ${presentation.run.remaining}",
                    color = SecondaryText,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = 4.dp),
                )
                job?.let {
                    Text("Limits: ${presentation.run.maxFiles} files · ${presentation.run.maxRetries} retries per file", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
                }
            }
            Spacer(Modifier.height(10.dp))
            FocusFlowPanel(Modifier.fillMaxWidth()) {
                SectionLabel("ANALYZE-ALL CONTROLS")
                when (presentation.run.statusLabel) {
                    "Running" -> {
                        ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = 8.dp)) {
                            FocusFlowButton(onClick = onPause, tone = ActionTone.Attention) { Text("Pause") }
                            FocusFlowButton(onClick = onCancel, tone = ActionTone.Destructive) { Text("Cancel") }
                        }
                    }
                    "Pausing", "Canceling" -> {
                        Text(presentation.controls, color = Warning, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
                        FocusFlowButton(onClick = onCancel, enabled = presentation.run.statusLabel == "Pausing", tone = ActionTone.Destructive, modifier = Modifier.padding(top = 8.dp)) { Text("Cancel") }
                    }
                    "Paused" -> {
                        RemoteProviderConfirmation(remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed)
                        ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = 8.dp)) {
                            FocusFlowButton(onClick = { onResume(remoteProviderConfirmed) }, enabled = !remoteProvider || remoteProviderConfirmed, tone = ActionTone.Primary) { Text("Resume") }
                            FocusFlowButton(onClick = onCancel, tone = ActionTone.Destructive) { Text("Cancel") }
                        }
                    }
                    else -> {
                        AnalyzeAllStartControls(maxFiles, { maxFiles = it }, maxRetries, { maxRetries = it }, remoteProvider, remoteProviderConfirmed, onRemoteProviderConfirmed, options, onStart)
                    }
                }
            }
            Spacer(Modifier.height(10.dp))
            SectionLabel("ANALYSIS ERRORS")
        }
        if (presentation.failures.isEmpty()) {
            item { Text(presentation.noErrorsMessage, color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp)) }
        } else {
            items(presentation.failures, key = { it.path }) { failure ->
                FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 7.dp)) {
                    Text(failure.path, color = PrimaryText, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
                    Text("Attempt ${failure.attempts}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
                    Text(failure.error, color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
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
    ResponsiveFieldPair(
        modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
        first = { modifier -> CompactSingleLineField(maxFiles, onMaxFiles, label = { Text("File limit (1–500)") }, modifier = modifier) },
        second = { modifier -> CompactSingleLineField(maxRetries, onMaxRetries, label = { Text("Retry limit (0–3)") }, modifier = modifier) },
    )
    RemoteProviderConfirmation(remoteProvider, remoteConfirmed, onRemoteConfirmed)
    FocusFlowButton(onClick = { onStart(options) }, enabled = !remoteProvider || remoteConfirmed, tone = ActionTone.Primary, modifier = Modifier.padding(top = 6.dp)) { Text("Start Analyze-all") }
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
    var showFilters by remember { mutableStateOf(false) }
    val filters = BugsFilters(query, source, severity, freshness, lifecycle)
    val activeFilters = activeBugsFilters(filters)
    val visible = filterFindings(findings, filters)
    val progress = verifiedScanProgress(scan)
    val grouped = FindingClassification.entries.associateWith { classification -> visible.filter { classifyFinding(it) == classification } }
    LazyColumn(Modifier.fillMaxSize().padding(18.dp)) {
        item {
            Text("PROJECT BUGS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(12.dp))
            FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
                SectionLabel("SEARCH AND FILTER")
                CompactSingleLineField(query, { query = it }, label = { Text("Search findings") }, modifier = Modifier.fillMaxWidth().padding(top = 7.dp))
                FocusFlowButton(onClick = { showFilters = !showFilters }, tone = ActionTone.Neutral, selected = showFilters, modifier = Modifier.padding(top = 6.dp)) { Text(if (showFilters) "Hide filters" else "Filters") }
                if (activeFilters.isNotEmpty()) Text("Filters active: ${activeFilters.joinToString(" · ")}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 5.dp))
                if (showFilters) {
                    ResponsiveFieldPair(
                        modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
                        first = { modifier -> CompactSingleLineField(source, { source = it }, label = { Text("Source") }, modifier = modifier) },
                        second = { modifier -> CompactSingleLineField(severity, { severity = it }, label = { Text("Severity") }, modifier = modifier) },
                    )
                    ResponsiveFieldPair(
                        modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
                        first = { modifier -> CompactSingleLineField(freshness, { freshness = it }, label = { Text("Freshness") }, modifier = modifier) },
                        second = { modifier -> CompactSingleLineField(lifecycle, { lifecycle = it }, label = { Text("Lifecycle") }, modifier = modifier) },
                    )
                }
            }
            Spacer(Modifier.height(10.dp))
            FocusFlowPanel(Modifier.fillMaxWidth()) {
                SectionLabel("VERIFIED SCAN")
                Text(progress.summary, color = if (progress.warnings.isNotEmpty()) Warning else SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 7.dp))
                progress.warnings.forEach { warning -> Text("Warning: $warning", color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp)) }
                if (progress.canCancel) {
                    FocusFlowButton(onClick = onCancelScan, enabled = scan?.status?.lowercase() == "running", tone = ActionTone.Destructive, modifier = Modifier.padding(top = 8.dp)) { Text(if (scan?.status?.lowercase() == "canceling") "Canceling…" else "Cancel verified scan") }
                } else {
                    FocusFlowButton(onClick = onStartScan, tone = ActionTone.Primary, modifier = Modifier.padding(top = 8.dp)) { Text("Run verified scan") }
                }
            }
            Spacer(Modifier.height(10.dp))
            SectionLabel("FINDINGS")
        }
        FindingClassification.entries.forEach { classification ->
            val section = grouped.getValue(classification)
            item {
                SectionLabel(classification.sectionLabel, Modifier.padding(top = 9.dp))
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
        ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = 7.dp)) {
            FocusFlowButton(onClick = { onOpen(finding) }, enabled = finding.location.path.isNotBlank(), tone = ActionTone.Navigation) { Text("Open in Editor") }
            FocusFlowButton(onClick = { onPrepare(finding) }, enabled = findingCanPrepareFix(finding), tone = ActionTone.Navigation) { Text("Prepare fix") }
            findingLifecycleActions(finding).forEach { action ->
                FocusFlowButton(onClick = { onTriage(finding, action) }, tone = if (action.status == "dismissed") ActionTone.Destructive else ActionTone.Neutral) { Text(action.label) }
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
