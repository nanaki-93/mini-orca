package io.miniorca.desktop

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
import androidx.compose.ui.text.font.FontWeight
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
  LazyColumn(Modifier.fillMaxSize().padding(18.dp)) {
    item {
      Text(
          "PROJECT ANALYSIS",
          color = PrimaryText,
          fontSize = 18.sp,
          fontWeight = FontWeight.SemiBold)
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
        Text(
            presentation.run.statusLabel,
            color = if (presentation.run.statusLabel == "Failed") Error else PrimaryText,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(top = 7.dp))
        Text(
            presentation.run.statusDetail,
            color = SecondaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = 4.dp))
        Text(
            "Candidates: ${presentation.run.candidates} · Completed: ${presentation.run.completed} · Failed: ${presentation.run.failed} · Running: ${presentation.run.running} · Remaining: ${presentation.run.remaining}",
            color = SecondaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = 4.dp),
        )
        state.job?.let {
          Text(
              "Limits: ${presentation.run.maxFiles} files · ${presentation.run.maxRetries} retries per file",
              color = SecondaryText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 4.dp))
        }
      }
      Spacer(Modifier.height(10.dp))
      FocusFlowPanel(Modifier.fillMaxWidth()) {
        SectionLabel("ANALYZE-ALL CONTROLS")
        when (presentation.run.statusLabel) {
          "Running" -> {
            ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = 8.dp)) {
              FocusFlowButton(onClick = actions.pause, tone = ActionTone.Attention) {
                Text("Pause")
              }
              FocusFlowButton(onClick = actions.cancel, tone = ActionTone.Destructive) {
                Text("Cancel")
              }
            }
          }
          "Pausing",
          "Canceling" -> {
            Text(
                presentation.controls,
                color = Warning,
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 7.dp))
            FocusFlowButton(
                onClick = actions.cancel,
                enabled = presentation.run.statusLabel == "Pausing",
                tone = ActionTone.Destructive,
                modifier = Modifier.padding(top = 8.dp)) {
                  Text("Cancel")
                }
          }
          "Paused" -> {
            RemoteProviderConfirmation(
                ModelScope.Bug,
                state.model,
                state.remoteProviderConfirmed,
                actions.confirmRemoteProvider)
            ResponsiveActionGroup(Modifier.fillMaxWidth().padding(top = 8.dp)) {
              FocusFlowButton(
                  onClick = { actions.resume(state.remoteProviderConfirmed) },
                  enabled = !state.model.remoteProvider || state.remoteProviderConfirmed,
                  tone = ActionTone.Primary) {
                    Text("Resume")
                  }
              FocusFlowButton(onClick = actions.cancel, tone = ActionTone.Destructive) {
                Text("Cancel")
              }
            }
          }
          else -> {
            AnalyzeAllStartControls(
                state =
                    AnalyzeAllStartControlsState(
                        maxFiles, maxRetries, state.model, state.remoteProviderConfirmed, options),
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
      Spacer(Modifier.height(10.dp))
      SectionLabel("ANALYSIS ERRORS")
    }
    if (presentation.failures.isEmpty()) {
      item {
        Text(
            presentation.noErrorsMessage,
            color = SecondaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = 7.dp))
      }
    } else {
      items(presentation.failures, key = { it.path }) { failure ->
        FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 7.dp)) {
          Text(
              failure.path, color = PrimaryText, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
          Text(
              "Attempt ${failure.attempts}",
              color = SecondaryText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 4.dp))
          Text(
              failure.error,
              color = Error,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 4.dp))
        }
      }
    }
  }
}

@Composable
private fun AnalyzeAllStartControls(
    state: AnalyzeAllStartControlsState,
    actions: AnalyzeAllStartActions
) {
  ResponsiveFieldPair(
      modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
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
  FocusFlowButton(
      onClick = { actions.start(state.options) },
      enabled = !state.model.remoteProvider || state.remoteConfirmed,
      tone = ActionTone.Primary,
      modifier = Modifier.padding(top = 6.dp)) {
        Text("Start Analyze-all")
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
      modifier = Modifier.padding(top = 7.dp),
  )
  if (model.remoteProvider)
      androidx.compose.foundation.layout.Row(modifier = Modifier.padding(top = 2.dp)) {
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
  LazyColumn(Modifier.fillMaxSize().padding(18.dp)) {
    item {
      Text("PROJECT BUGS", color = PrimaryText, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
      Spacer(Modifier.height(12.dp))
      FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
        SectionLabel("SEARCH AND FILTER")
        FindingsFilterControls(
            filters, presentation, modifier = Modifier.fillMaxWidth().padding(top = 7.dp))
      }
      Spacer(Modifier.height(10.dp))
      FocusFlowPanel(Modifier.fillMaxWidth()) {
        SectionLabel("VERIFIED SCAN")
        Text(
            progress.summary,
            color = if (progress.warnings.isNotEmpty()) Warning else SecondaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = 7.dp))
        progress.warnings.forEach { warning ->
          Text(
              "Warning: $warning",
              color = Error,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 4.dp))
        }
        if (progress.canCancel) {
          FocusFlowButton(
              onClick = actions.cancelScan,
              enabled = state.scan?.status?.lowercase() == "running",
              tone = ActionTone.Destructive,
              modifier = Modifier.padding(top = 8.dp)) {
                Text(
                    if (state.scan?.status?.lowercase() == "canceling") "Canceling…"
                    else "Cancel verified scan")
              }
        } else {
          FocusFlowButton(
              onClick = actions.startScan,
              tone = ActionTone.Primary,
              modifier = Modifier.padding(top = 8.dp)) {
                Text("Run verified scan")
              }
        }
      }
      Spacer(Modifier.height(10.dp))
      SectionLabel("FINDINGS")
    }
    if (presentation.priorityGroups.isEmpty()) {
      item {
        Text(
            presentation.emptyMessage,
            color = SecondaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = 9.dp))
      }
    } else {
      presentation.priorityGroups.forEach { group ->
        item { SectionLabel(group.priority.sectionLabel, Modifier.padding(top = 9.dp)) }
        items(
            group.findings,
            key = { finding ->
              "${group.priority.name}:${finding.id}:${finding.location.path}:${finding.location.startLine}"
            }) { finding ->
              DetailedFindingCard(finding, actions.findingActions)
            }
      }
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
