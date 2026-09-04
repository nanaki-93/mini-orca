package io.miniorca.desktop

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun AnalysisWorkspacePane(
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions
) {
  var maxFiles by remember { mutableStateOf(defaultAnalyzeAllFileLimit.toString()) }
  var maxRetries by remember { mutableStateOf(defaultAnalyzeAllRetryLimit.toString()) }
  val options =
      AnalyzeAllRunOptions(
              maxFiles = maxFiles.toIntOrNull() ?: defaultAnalyzeAllFileLimit,
              maxRetries = maxRetries.toIntOrNull() ?: defaultAnalyzeAllRetryLimit,
              confirmRemoteProvider = state.remoteProviderConfirmed,
          )
          .bounded()
  val presentation = analyzeAllPresentation(state.job, state.coverage)
  LazyColumn(Modifier.fillMaxSize().padding(12.dp)) {
    item {
      WorkspacePaneHeader("Analysis")
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(), contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Coverage")
            CompactKeyValueRows(
                listOf(
                    "Total" to presentation.coverage.total.toString(),
                    "Fresh / stale" to
                        "${presentation.coverage.fresh} / ${presentation.coverage.stale}",
                    "Missing / running" to
                        "${presentation.coverage.missing} / ${presentation.coverage.running}",
                    "Failed" to presentation.coverage.failed.toString(),
                ),
                modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
            )
          }
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(),
          raised = presentation.run.statusLabel in setOf("Running", "Paused", "Failed"),
          contentPadding = PaddingValues(MiniOrcaSpacing.standard),
      ) {
        SectionLabel("Current run")
        Text(
            presentation.run.statusLabel,
            color = if (presentation.run.statusLabel == "Failed") Error else PrimaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
        CompactKeyValueRows(
            listOf(
                "Progress" to
                    "${presentation.run.completed} complete · ${presentation.run.running} running · ${presentation.run.remaining} remaining",
                "Failures" to presentation.run.failed.toString(),
                "Limits" to
                    "${presentation.run.maxFiles} files · ${presentation.run.maxRetries} retries per file",
            ),
            modifier = Modifier.padding(top = MiniOrcaSpacing.compact),
        )
        Text(
            presentation.run.statusDetail,
            color = SecondaryText,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
      }
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(), contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Run controls")
            when (presentation.run.statusLabel) {
              "Running" -> {
                ResponsiveActionGroup(
                    Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard)) {
                      MiniOrcaButton(
                          onClick = actions.pause,
                          tone = ActionTone.Attention,
                          density = ButtonDensity.Toolbar) {
                            Text("Pause", fontSize = 11.sp)
                          }
                      MiniOrcaButton(
                          onClick = actions.cancel,
                          tone = ActionTone.Destructive,
                          density = ButtonDensity.Toolbar) {
                            Text("Cancel", fontSize = 11.sp)
                          }
                    }
              }
              "Pausing",
              "Canceling" -> {
                Text(
                    presentation.controls,
                    color = Warning,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
                MiniOrcaButton(
                    onClick = actions.cancel,
                    enabled = presentation.run.statusLabel == "Pausing",
                    tone = ActionTone.Destructive,
                    density = ButtonDensity.Toolbar,
                    modifier = Modifier.padding(top = MiniOrcaSpacing.standard)) {
                      Text("Cancel", fontSize = 11.sp)
                    }
              }
              "Paused" -> {
                RemoteProviderConfirmation(
                    ModelScope.Bug,
                    state.model,
                    state.remoteProviderConfirmed,
                    actions.confirmRemoteProvider)
                ResponsiveActionGroup(
                    Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard)) {
                      MiniOrcaButton(
                          onClick = { actions.resume(state.remoteProviderConfirmed) },
                          enabled = !state.model.remoteProvider || state.remoteProviderConfirmed,
                          tone = ActionTone.Primary,
                          density = ButtonDensity.Toolbar) {
                            Text("Resume", fontSize = 11.sp)
                          }
                      MiniOrcaButton(
                          onClick = actions.cancel,
                          tone = ActionTone.Destructive,
                          density = ButtonDensity.Toolbar) {
                            Text("Cancel", fontSize = 11.sp)
                          }
                    }
              }
              else -> {
                AnalyzeAllStartControls(
                    state =
                        AnalyzeAllStartControlsState(
                            maxFiles,
                            maxRetries,
                            state.model,
                            state.remoteProviderConfirmed,
                            options),
                    actions =
                        AnalyzeAllStartActions(
                            updateMaxFiles = { maxFiles = it },
                            updateMaxRetries = { maxRetries = it },
                            confirmRemoteProvider = actions.confirmRemoteProvider,
                            start = actions.start,
                        ),
                )
              }
            }
          }
      Spacer(Modifier.height(8.dp))
      SectionLabel("Analysis errors")
    }
    if (presentation.failures.isEmpty()) {
      item {
        SystemStateMessage(
            "No analysis errors",
            presentation.noErrorsMessage,
            modifier = Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard),
        )
      }
    } else {
      items(presentation.failures, key = { it.path }) { failure -> AnalysisFailureRow(failure) }
    }
  }
}

@Composable
private fun AnalysisFailureRow(failure: AnalysisFailurePresentation) {
  MiniOrcaPanel(
      Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.compact),
      raised = true,
      contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
        CompactKeyValueRows(
            listOf("File" to failure.path, "Attempt" to failure.attempts.toString()),
        )
        Text(
            failure.error,
            color = Error,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.compact),
        )
      }
}

@Composable
private fun AnalyzeAllStartControls(
    state: AnalyzeAllStartControlsState,
    actions: AnalyzeAllStartActions
) {
  ResponsiveFieldPair(
      modifier = Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard),
      first = { modifier ->
        CompactSingleLineField(
            state.maxFiles,
            actions.updateMaxFiles,
            label = { Text("File limit (1–500)") },
            modifier = modifier)
      },
      second = { modifier ->
        CompactSingleLineField(
            state.maxRetries,
            actions.updateMaxRetries,
            label = { Text("Retry limit (0–3)") },
            modifier = modifier)
      },
  )
  RemoteProviderConfirmation(
      ModelScope.Bug, state.model, state.remoteConfirmed, actions.confirmRemoteProvider)
  MiniOrcaButton(
      onClick = { actions.start(state.options) },
      enabled = !state.model.remoteProvider || state.remoteConfirmed,
      tone = ActionTone.Primary,
      density = ButtonDensity.Toolbar,
      modifier = Modifier.padding(top = MiniOrcaSpacing.standard)) {
        Text("Start Analyze-all", fontSize = 11.sp)
      }
}

/** Project-wide analysis data displayed by the Analysis workspace. */
internal data class AnalysisWorkspacePaneState(
    val job: AnalyzeAllJob?,
    val coverage: AnalysisCoverage?,
    val model: ScopedModel,
    val remoteProviderConfirmed: Boolean,
)

/** Analyze-all workflow intents, deliberately separate from file-scoped editing. */
internal data class AnalysisWorkspaceActions(
    val confirmRemoteProvider: (Boolean) -> Unit,
    val start: (AnalyzeAllRunOptions) -> Unit,
    val pause: () -> Unit,
    val resume: (Boolean) -> Unit,
    val cancel: () -> Unit,
)

private data class AnalyzeAllStartControlsState(
    val maxFiles: String,
    val maxRetries: String,
    val model: ScopedModel,
    val remoteConfirmed: Boolean,
    val options: AnalyzeAllRunOptions,
)

private data class AnalyzeAllStartActions(
    val updateMaxFiles: (String) -> Unit,
    val updateMaxRetries: (String) -> Unit,
    val confirmRemoteProvider: (Boolean) -> Unit,
    val start: (AnalyzeAllRunOptions) -> Unit,
)

@Composable
internal fun RemoteProviderConfirmation(
    scope: ModelScope,
    model: ScopedModel,
    confirmed: Boolean,
    onConfirmed: (Boolean) -> Unit
) {
  Text(
      modelDestinationLabel(scope, model),
      color = if (model.remoteProvider) Warning else SecondaryText,
      fontSize = 11.sp,
      modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
  )
  if (model.remoteProvider)
      androidx.compose.foundation.layout.Row(
          modifier = Modifier.padding(top = MiniOrcaSpacing.compact)) {
            Checkbox(checked = confirmed, onCheckedChange = onConfirmed)
            Text(
                "Confirm remote destination",
                color = SecondaryText,
                fontSize = 11.sp,
                modifier = Modifier.padding(top = 12.dp))
          }
}

@Composable
internal fun BugsWorkspacePane(state: BugsWorkspacePaneState, actions: BugsWorkspaceActions) {
  val filters = rememberFindingsFilterState()
  val presentation = findingsPresentation(state.findings, filters.filters, state.loading)
  val progress = verifiedScanProgress(state.scan)
  var selectedFindingKey by remember { mutableStateOf<String?>(null) }
  val selectedFinding = visibleFindingByDisplayKey(presentation, selectedFindingKey)
  LazyColumn(Modifier.fillMaxSize().padding(12.dp)) {
    item {
      WorkspacePaneHeader("Bugs")
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(), contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Search and filter")
            FindingsFilterControls(
                filters,
                presentation,
                modifier = Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard))
          }
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(),
          raised = progress.warnings.isNotEmpty(),
          contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Verified scan")
            Text(
                progress.summary,
                color = if (progress.warnings.isNotEmpty()) Warning else SecondaryText,
                fontSize = 11.sp,
                modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
            progress.warnings.forEach { warning ->
              Text(
                  "Warning: $warning",
                  color = Error,
                  fontSize = 11.sp,
                  modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
            }
            if (progress.canCancel) {
              MiniOrcaButton(
                  onClick = actions.cancelScan,
                  enabled = state.scan?.status?.lowercase() == "running",
                  tone = ActionTone.Destructive,
                  density = ButtonDensity.Toolbar,
                  modifier = Modifier.padding(top = MiniOrcaSpacing.standard)) {
                    Text(
                        if (state.scan?.status?.lowercase() == "canceling") "Canceling…"
                        else "Cancel scan",
                        fontSize = 11.sp)
                  }
            } else {
              MiniOrcaButton(
                  onClick = actions.startScan,
                  tone = ActionTone.Primary,
                  density = ButtonDensity.Toolbar,
                  modifier = Modifier.padding(top = MiniOrcaSpacing.standard)) {
                    Text("Run verified scan", fontSize = 11.sp)
                  }
            }
          }
      selectedFinding?.let { finding ->
        Spacer(Modifier.height(8.dp))
        FindingDetailsRegion(finding) { selectedFindingKey = null }
      }
      Spacer(Modifier.height(8.dp))
      SectionLabel("Findings")
    }
    if (presentation.priorityGroups.isEmpty()) {
      item {
        SystemStateMessage(
            "No findings",
            presentation.emptyMessage,
            modifier = Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard),
        )
      }
    } else {
      presentation.priorityGroups.forEach { group ->
        item { SectionLabel(group.priority.sectionLabel, Modifier.padding(top = 8.dp)) }
        items(
            group.findings,
            key = { finding -> "${group.priority.name}:${findingDisplayKey(finding)}" }) { finding
              ->
              CompactProblemRow(
                  finding,
                  actions.findingActions,
                  onShowDetails = { selectedFindingKey = findingDisplayKey(finding) })
            }
      }
    }
  }
}

@Composable
private fun FindingDetailsRegion(finding: UnifiedFinding, onDismiss: () -> Unit) {
  MiniOrcaPanel(
      Modifier.fillMaxWidth(),
      raised = true,
      contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
        Row(Modifier.fillMaxWidth()) {
          SectionLabel("Finding details", Modifier.weight(1f))
          MiniOrcaButton(
              onClick = onDismiss, tone = ActionTone.Neutral, density = ButtonDensity.Toolbar) {
                Text("Clear", fontSize = 11.sp)
              }
        }
        Text(
            finding.title.ifBlank { "Untitled finding" },
            color = PrimaryText,
            fontSize = 13.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
        CompactKeyValueRows(
            listOf(
                "Severity" to finding.severity.ifBlank { "unknown" },
                "Provenance" to findingProvenanceLabel(finding),
                "Location" to findingLocationLabel(finding),
                "Status" to findingStatusLabel(finding),
            ),
            modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
        )
        Text(
            finding.message.ifBlank { "No message supplied." },
            color = PrimaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
        if (finding.evidence.isNotBlank())
            Text(
                "Evidence: ${finding.evidence}",
                color = SecondaryText,
                fontSize = 11.sp,
                modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
        EngineeringInsightPanel(
            finding.engineeringInsight,
            stale = finding.freshness.equals("stale", ignoreCase = true),
            scopeLabel = "Selected finding")
        finding.taskSpec?.let { task ->
          CompactKeyValueRows(
              listOf("Fix task" to "${task.targetSymbol} · ${task.targetSignature}"),
              modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
          )
          Text(
              "Acceptance: ${task.acceptanceCriteria.joinToString(" · ")}",
              color = SecondaryText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
          if (task.nonGoals.isNotEmpty())
              Text(
                  "Non-goals: ${task.nonGoals.joinToString(" · ")}",
                  color = SecondaryText,
                  fontSize = 11.sp,
                  modifier = Modifier.padding(top = MiniOrcaSpacing.compact))
        }
      }
}

/** Read-only Bugs workspace inputs from the current project snapshot. */
internal data class BugsWorkspacePaneState(
    val findings: List<UnifiedFinding>,
    val scan: GoScanReport?,
    val loading: Boolean,
)

/** Finding navigation, task preparation, triage, and scan intents. */
internal data class BugsWorkspaceActions(
    val findingActions: FindingActions,
    val startScan: () -> Unit,
    val cancelScan: () -> Unit,
)
