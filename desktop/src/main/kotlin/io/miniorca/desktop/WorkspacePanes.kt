package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private val workspaceWideGutterMinimum = 900.dp

internal fun workspacePageHorizontalGutter(availableWidth: Dp): Dp =
    if (availableWidth >= workspaceWideGutterMinimum) 24.dp else 16.dp

internal fun workspacePagePadding(availableWidth: Dp, vertical: Dp): PaddingValues =
    PaddingValues(horizontal = workspacePageHorizontalGutter(availableWidth), vertical = vertical)

internal fun analysisMetricColumnCount(availableWidth: Dp): Int =
    when {
      availableWidth >= 780.dp -> 6
      availableWidth >= 520.dp -> 3
      else -> 2
    }

@Composable
internal fun AnalysisWorkspacePane(
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions
) {
  var maxFiles by remember { mutableStateOf(defaultAnalyzeAllFileLimit.toString()) }
  var maxRetries by remember { mutableStateOf(defaultAnalyzeAllRetryLimit.toString()) }
  var advancedOptionsExpanded by remember { mutableStateOf(false) }
  var failuresExpanded by remember { mutableStateOf(false) }
  var actionRequestPending by
      remember(state.job?.status, state.job?.projectRevision) { mutableStateOf(false) }
  val options =
      AnalyzeAllRunOptions(
              maxFiles = maxFiles.toIntOrNull() ?: defaultAnalyzeAllFileLimit,
              maxRetries = maxRetries.toIntOrNull() ?: defaultAnalyzeAllRetryLimit,
              confirmRemoteProvider = state.remoteProviderConfirmed,
          )
          .bounded()
  val presentation = analyzeAllPresentation(state.job, state.coverage)
  val toolbarActions =
      analyzeAllToolbarActions(presentation.run, state.model, state.remoteProviderConfirmed)
  fun requestRunAction(action: AnalyzeAllToolbarAction) {
    actionRequestPending = true
    when (action) {
      AnalyzeAllToolbarAction.Start -> actions.start(options)
      AnalyzeAllToolbarAction.Pause -> actions.pause()
      AnalyzeAllToolbarAction.Resume -> actions.resume(state.remoteProviderConfirmed)
      AnalyzeAllToolbarAction.Cancel -> actions.cancel()
    }
  }
  BoxWithConstraints(Modifier.fillMaxSize()) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = workspacePagePadding(maxWidth, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
      item {
        AnalysisRunHeader(
            run = presentation.run,
            job = state.job,
            model = state.model,
            toolbarActions = toolbarActions,
            actionRequestPending = actionRequestPending,
            onRunAction = ::requestRunAction,
        )
      }
      item {
        AnalysisAdvancedOptions(
            maxFiles = maxFiles,
            maxRetries = maxRetries,
            model = state.model,
            remoteProviderConfirmed = state.remoteProviderConfirmed,
            requiresRemoteConfirmation =
                toolbarActions.any {
                  !it.enabled &&
                      (it.action == AnalyzeAllToolbarAction.Start ||
                          it.action == AnalyzeAllToolbarAction.Resume)
                },
            expanded = advancedOptionsExpanded,
            enabled = !actionRequestPending,
            onToggle = { advancedOptionsExpanded = !advancedOptionsExpanded },
            onMaxFilesChanged = { maxFiles = it },
            onMaxRetriesChanged = { maxRetries = it },
            onRemoteProviderConfirmed = actions.confirmRemoteProvider,
        )
      }
      item { AnalysisCoverageMetrics(presentation.coverage, available = state.coverage != null) }
      item {
        AnalysisErrorsHeader(
            count = presentation.failures.size,
            expanded = failuresExpanded,
            onToggle = { failuresExpanded = !failuresExpanded },
        )
      }
      if (failuresExpanded && presentation.failures.isEmpty()) {
        item {
          Row(verticalAlignment = Alignment.CenterVertically) {
            DesktopLineIcon(
                DesktopIcon.Check, "No analysis errors", tint = Success, iconSize = 16.dp)
            Spacer(Modifier.width(8.dp))
            Column {
              Text("No analysis errors", color = PrimaryText, fontSize = 12.sp)
              Text(
                  if (state.job == null) "Run failures appear here."
                  else presentation.noErrorsMessage,
                  color = SecondaryText,
                  fontSize = 11.sp,
                  modifier = Modifier.padding(top = 4.dp))
            }
          }
        }
      } else if (failuresExpanded) {
        items(presentation.failures, key = { it.path }) { failure -> AnalysisFailureRow(failure) }
      }
    }
  }
}

@Composable
private fun AnalysisRunHeader(
    run: AnalyzeAllRunPresentation,
    job: AnalyzeAllJob?,
    model: ScopedModel,
    toolbarActions: List<AnalyzeAllToolbarActionPresentation>,
    actionRequestPending: Boolean,
    onRunAction: (AnalyzeAllToolbarAction) -> Unit,
) {
  val statusTint = analysisRunStatusTint(run.statusLabel)
  IdePaneHeader(
      title = "Analysis",
      icon = DesktopIcon.Analysis,
      stateLabel = "${run.statusLabel} · ${run.controls}",
      stateTint = statusTint,
      actions = {
        toolbarActions.forEach { toolbarAction ->
          MiniOrcaButton(
              onClick = { onRunAction(toolbarAction.action) },
              enabled = toolbarAction.enabled && !actionRequestPending,
              tone = analysisRunActionTone(toolbarAction.action),
              density = ButtonDensity.Toolbar) {
                Text(analyzeAllToolbarActionLabel(toolbarAction.action), fontSize = 11.sp)
              }
        }
      },
  )
  Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
    Text(run.statusDetail, color = SecondaryText, fontSize = 12.sp, lineHeight = 18.sp)
    Text(
        if (run.candidates > 0) "${run.completed + run.failed} of ${run.candidates} files processed"
        else "No files queued",
        color = PrimaryText,
        fontSize = 13.sp,
        fontWeight = FontWeight.Medium,
        modifier = Modifier.padding(top = 8.dp))
    IdeProgressBar(
        progress = analysisRunProgress(run),
        modifier = Modifier.fillMaxWidth().padding(top = 6.dp).height(3.dp),
        color = statusTint,
        trackColor = StrongSurface)
    Text(
        "${run.completed} complete  ·  ${run.running} running  ·  ${run.remaining} remaining  ·  ${run.failed} failed",
        color = SecondaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = 6.dp))
    job?.files
        ?.firstOrNull { it.status.equals("running", ignoreCase = true) }
        ?.let { file ->
          Text(
              "Current: ${file.path}",
              color = SecondaryText,
              fontSize = 11.sp,
              fontFamily = FontFamily.Monospace,
              maxLines = 2,
              overflow = TextOverflow.Ellipsis,
              modifier = Modifier.padding(top = 8.dp))
        }
    if (job != null) {
      Text(
          "${run.maxFiles} file limit  ·  ${run.maxRetries} retries per file",
          color = FaintText,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = 8.dp))
      Text(
          if (model.model.isBlank()) "Bugs model is not configured."
          else modelDestinationLabel(ModelScope.Bug, model),
          color = SecondaryText,
          fontSize = 11.sp,
          lineHeight = 16.sp,
          modifier = Modifier.padding(top = 4.dp))
    }
  }
}

private fun analysisRunStatusTint(statusLabel: String) =
    when (statusLabel) {
      "Failed" -> Error
      "Completed" -> Success
      "Running" -> SelectionText
      "Paused",
      "Pausing",
      "Canceling",
      "Stale" -> Warning
      else -> SecondaryText
    }

private fun analysisRunActionTone(action: AnalyzeAllToolbarAction) =
    when (action) {
      AnalyzeAllToolbarAction.Start,
      AnalyzeAllToolbarAction.Resume -> ActionTone.Primary
      AnalyzeAllToolbarAction.Pause -> ActionTone.Attention
      AnalyzeAllToolbarAction.Cancel -> ActionTone.Destructive
    }

@Composable
private fun AnalysisCoverageMetrics(coverage: AnalysisCoveragePresentation, available: Boolean) {
  Column(Modifier.fillMaxWidth()) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
      SectionLabel("Coverage")
      Spacer(Modifier.weight(1f))
      Text(if (available) "Indexed files" else "Unavailable", color = FaintText, fontSize = 11.sp)
    }
    IdeHorizontalSeparator(Modifier.padding(top = 4.dp))
    if (!available) {
      Text(
          "Coverage is unavailable; zero is not inferred from missing data.",
          color = SecondaryText,
          fontSize = 11.sp,
          lineHeight = 16.sp,
          modifier = Modifier.padding(top = 8.dp))
    }
    val metrics =
        listOf(
            Triple("Total files", coverage.total, PrimaryText),
            Triple("Fresh", coverage.fresh, Success),
            Triple("Stale", coverage.stale, Warning),
            Triple("Not analyzed", coverage.missing, SecondaryText),
            Triple("Running", coverage.running, SelectionText),
            Triple("Failed", coverage.failed, Error),
        )
    BoxWithConstraints {
      val columns = analysisMetricColumnCount(maxWidth)
      Column(
          modifier = Modifier.padding(top = 8.dp),
          verticalArrangement = Arrangement.spacedBy(8.dp)) {
            metrics.chunked(columns).forEach { row ->
              Row(Modifier.fillMaxWidth()) {
                row.forEachIndexed { index, (label, value, tint) ->
                  if (index > 0) IdeVerticalSeparator(Modifier.height(40.dp))
                  Column(Modifier.weight(1f).padding(start = if (index == 0) 0.dp else 8.dp)) {
                    Text(
                        if (available) value.toString() else "—",
                        color = if (value > 0 || label == "Total files") tint else SecondaryText,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Medium)
                    Text(
                        label,
                        color = SecondaryText,
                        fontSize = 11.sp,
                        modifier = Modifier.padding(top = 2.dp))
                  }
                }
              }
            }
          }
    }
  }
}

@Composable
private fun AnalysisAdvancedOptions(
    maxFiles: String,
    maxRetries: String,
    model: ScopedModel,
    remoteProviderConfirmed: Boolean,
    requiresRemoteConfirmation: Boolean,
    expanded: Boolean,
    enabled: Boolean,
    onToggle: () -> Unit,
    onMaxFilesChanged: (String) -> Unit,
    onMaxRetriesChanged: (String) -> Unit,
    onRemoteProviderConfirmed: (Boolean) -> Unit,
) {
  val stateLabel =
      if (requiresRemoteConfirmation) "Remote confirmation required"
      else "$maxFiles files · $maxRetries retries"
  IdeDisclosureHeader(
      title = "Advanced options",
      expanded = expanded,
      onToggle = onToggle,
      stateLabel = stateLabel,
      stateTint = if (requiresRemoteConfirmation) Warning else SecondaryText,
  )
  if (expanded) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      ResponsiveFieldPair(
          modifier = Modifier.fillMaxWidth(),
          first = { modifier ->
            CompactSingleLineField(
                maxFiles,
                onMaxFilesChanged,
                label = "File limit (1–500)",
                modifier = modifier,
                enabled = enabled)
          },
          second = { modifier ->
            CompactSingleLineField(
                maxRetries,
                onMaxRetriesChanged,
                label = "Retry limit (0–3)",
                modifier = modifier,
                enabled = enabled)
          },
      )
      RemoteProviderConfirmation(
          ModelScope.Bug,
          model,
          remoteProviderConfirmed,
          onRemoteProviderConfirmed,
          enabled = enabled)
    }
  }
}

@Composable
private fun AnalysisErrorsHeader(count: Int, expanded: Boolean, onToggle: () -> Unit) {
  IdePaneHeader(
      title = "Analysis errors",
      icon = DesktopIcon.Problems,
      expanded = expanded,
      onToggle = onToggle,
      stateLabel =
          if (count == 0) "No errors" else "$count ${if (count == 1) "error" else "errors"}",
      stateTint = if (count == 0) SecondaryText else Error,
  )
}

@Composable
private fun AnalysisFailureRow(failure: AnalysisFailurePresentation) {
  Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
    Row(verticalAlignment = Alignment.CenterVertically) {
      DesktopLineIcon(DesktopIcon.Problems, "Analysis error", tint = Error, iconSize = 16.dp)
      Spacer(Modifier.width(8.dp))
      Text(
          failure.path,
          color = PrimaryText,
          fontFamily = FontFamily.Monospace,
          fontSize = 11.sp,
          maxLines = 2,
          overflow = TextOverflow.Ellipsis,
          modifier = Modifier.weight(1f))
      Text("Attempt ${failure.attempts}", color = SecondaryText, fontSize = 11.sp)
    }
    SelectionContainer {
      Text(
          failure.error,
          color = Error,
          fontSize = 11.sp,
          lineHeight = 16.sp,
          modifier = Modifier.padding(start = 24.dp, top = 4.dp),
      )
    }
    IdeHorizontalSeparator(Modifier.padding(top = 8.dp))
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

@Composable
internal fun RemoteProviderConfirmation(
    scope: ModelScope,
    model: ScopedModel,
    confirmed: Boolean,
    onConfirmed: (Boolean) -> Unit,
    enabled: Boolean = true,
) {
  Text(
      modelDestinationLabel(scope, model),
      color = if (model.remoteProvider) Warning else SecondaryText,
      fontSize = 11.sp,
      modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
  )
  if (model.remoteProvider)
      ChromeButton(
          onClick = { onConfirmed(!confirmed) },
          enabled = enabled,
          selected = confirmed,
          role = androidx.compose.ui.semantics.Role.Checkbox,
          accessibleName = "Confirm remote destination",
          modifier =
              Modifier.padding(top = MiniOrcaSpacing.compact).semantics {
                stateDescription = if (confirmed) "Confirmed" else "Not confirmed"
              },
      ) {
        Text(if (confirmed) "✓" else "□", fontSize = 14.sp)
        Spacer(Modifier.width(MiniOrcaSpacing.compact))
        Text("Confirm remote destination${if (confirmed) " · confirmed" else ""}", fontSize = 11.sp)
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
            SectionLabel("Filters")
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
