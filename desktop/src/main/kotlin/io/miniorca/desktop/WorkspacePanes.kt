package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.material.Checkbox
import androidx.compose.material.Divider
import androidx.compose.material.LinearProgressIndicator
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private val workspaceWideGutterMinimum = 900.dp
private val analysisControlColumnMinimum = 280.dp
private val analysisControlColumnLargeTextMinimum = 320.dp
private val analysisControlColumnMaximum = 360.dp
private val analysisRunMinimumWidth = 420.dp
private val analysisRunLargeTextMinimumWidth = 440.dp
private val analysisRunControlGap = 20.dp

internal fun workspacePageHorizontalGutter(availableWidth: Dp): Dp =
    if (availableWidth >= workspaceWideGutterMinimum) 24.dp else 16.dp

internal fun workspacePagePadding(availableWidth: Dp, vertical: Dp): PaddingValues =
    PaddingValues(horizontal = workspacePageHorizontalGutter(availableWidth), vertical = vertical)

internal data class AnalysisRunControlLayout(val stacked: Boolean, val controlsWidth: Dp)

internal fun analysisRunControlLayout(
    availableWidth: Dp,
    fontScale: Float = 1f,
): AnalysisRunControlLayout {
  val enlargedText = fontScale > 1.15f
  val controlsMinimum =
      if (enlargedText) analysisControlColumnLargeTextMinimum else analysisControlColumnMinimum
  val runMinimum = if (enlargedText) analysisRunLargeTextMinimumWidth else analysisRunMinimumWidth
  val controlsWidth =
      (availableWidth * 0.3f).coerceIn(controlsMinimum, analysisControlColumnMaximum)
  return AnalysisRunControlLayout(
      stacked = availableWidth < runMinimum + analysisRunControlGap + controlsWidth,
      controlsWidth = controlsWidth,
  )
}

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
  val options =
      AnalyzeAllRunOptions(
              maxFiles = maxFiles.toIntOrNull() ?: defaultAnalyzeAllFileLimit,
              maxRetries = maxRetries.toIntOrNull() ?: defaultAnalyzeAllRetryLimit,
              confirmRemoteProvider = state.remoteProviderConfirmed,
          )
          .bounded()
  val presentation = analyzeAllPresentation(state.job, state.coverage)
  BoxWithConstraints(Modifier.fillMaxSize()) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = workspacePagePadding(maxWidth, vertical = 24.dp),
        verticalArrangement = Arrangement.spacedBy(24.dp),
    ) {
      item {
        Row(verticalAlignment = Alignment.CenterVertically) {
          DesktopLineIcon(DesktopIcon.Analysis, "Analysis", tint = SelectionText, iconSize = 26.dp)
          Spacer(Modifier.width(12.dp))
          Column(Modifier.weight(1f)) {
            Text(
                "Analysis", color = PrimaryText, fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
          }
        }
      }
      item { AnalysisCoverageMetrics(presentation.coverage, available = state.coverage != null) }
      item {
        BoxWithConstraints(Modifier.fillMaxWidth()) {
          val layout = analysisRunControlLayout(maxWidth, LocalDensity.current.fontScale)
          val runContent: @Composable (Modifier) -> Unit = { modifier ->
            AnalysisRunCard(presentation.run, state.job, modifier)
          }
          val controlsContent: @Composable (Modifier) -> Unit = { modifier ->
            MiniOrcaPanel(modifier, contentPadding = PaddingValues(20.dp)) {
              SectionLabel("Run controls")
              AnalysisRunControls(
                  run = presentation.run,
                  state = state,
                  actions = actions,
                  startState =
                      AnalyzeAllStartControlsState(
                          maxFiles,
                          maxRetries,
                          state.model,
                          state.remoteProviderConfirmed,
                          options),
                  startActions =
                      AnalyzeAllStartActions(
                          { maxFiles = it },
                          { maxRetries = it },
                          actions.confirmRemoteProvider,
                          actions.start),
              )
              if (state.job != null &&
                  presentation.run.statusLabel in setOf("Running", "Pausing", "Canceling")) {
                Text(
                    if (state.model.model.isBlank()) "Bugs model is not configured."
                    else modelDestinationLabel(ModelScope.Bug, state.model),
                    color = SecondaryText,
                    fontSize = 12.sp,
                    lineHeight = 18.sp,
                    modifier = Modifier.padding(top = 16.dp))
              }
            }
          }
          if (!layout.stacked) {
            Row(Modifier.fillMaxWidth()) {
              runContent(Modifier.weight(1f))
              Spacer(Modifier.width(analysisRunControlGap))
              controlsContent(Modifier.width(layout.controlsWidth))
            }
          } else {
            Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
              controlsContent(Modifier.fillMaxWidth())
              runContent(Modifier.fillMaxWidth())
            }
          }
        }
      }
      item {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
          SectionLabel("Failures")
          Spacer(Modifier.width(8.dp))
          Text(presentation.failures.size.toString(), color = FaintText, fontSize = 12.sp)
        }
        Divider(Modifier.padding(top = 12.dp), color = Border)
      }
      if (presentation.failures.isEmpty()) {
        item {
          Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier.size(36.dp).background(Success.copy(alpha = 0.08f), MiniOrcaShapes.large),
                contentAlignment = Alignment.Center) {
                  DesktopLineIcon(
                      DesktopIcon.Check, "No failures", tint = Success, iconSize = 20.dp)
                }
            Spacer(Modifier.width(12.dp))
            Column {
              Text("No failures", color = PrimaryText, fontSize = 13.sp)
              Text(
                  if (state.job == null) "Run failures appear here."
                  else presentation.noErrorsMessage,
                  color = SecondaryText,
                  fontSize = 12.sp,
                  modifier = Modifier.padding(top = 4.dp))
            }
          }
        }
      } else {
        items(presentation.failures, key = { it.path }) { failure -> AnalysisFailureRow(failure) }
      }
    }
  }
}

@Composable
private fun AnalysisRunControls(
    run: AnalyzeAllRunPresentation,
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions,
    startState: AnalyzeAllStartControlsState,
    startActions: AnalyzeAllStartActions,
) {
  when (run.statusLabel) {
    "Running" -> {
      ResponsiveActionGroup(
          Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard),
          minimumHorizontalWidth = 220.dp) {
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
          run.controls,
          color = Warning,
          fontSize = 12.sp,
          modifier = Modifier.padding(top = MiniOrcaSpacing.standard))
      MiniOrcaButton(
          onClick = actions.cancel,
          enabled = run.statusLabel == "Pausing",
          tone = ActionTone.Destructive,
          density = ButtonDensity.Toolbar,
          modifier = Modifier.padding(top = MiniOrcaSpacing.standard)) {
            Text("Cancel", fontSize = 11.sp)
          }
    }
    "Paused" -> {
      RemoteProviderConfirmation(
          ModelScope.Bug, state.model, state.remoteProviderConfirmed, actions.confirmRemoteProvider)
      ResponsiveActionGroup(
          Modifier.fillMaxWidth().padding(top = MiniOrcaSpacing.standard),
          minimumHorizontalWidth = 220.dp) {
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
    else -> AnalyzeAllStartControls(startState, startActions)
  }
}

@Composable
private fun AnalysisCoverageMetrics(coverage: AnalysisCoveragePresentation, available: Boolean) {
  MiniOrcaPanel(Modifier.fillMaxWidth(), contentPadding = PaddingValues(20.dp)) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
      SectionLabel("Coverage")
      Spacer(Modifier.weight(1f))
      Text(if (available) "Indexed files" else "Unavailable", color = FaintText, fontSize = 12.sp)
    }
    Spacer(Modifier.height(20.dp))
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
      Column(verticalArrangement = Arrangement.spacedBy(20.dp)) {
        metrics.chunked(columns).forEach { row ->
          Row(Modifier.fillMaxWidth()) {
            row.forEachIndexed { index, (label, value, tint) ->
              if (index > 0) Box(Modifier.height(54.dp).width(1.dp).background(Border))
              Column(Modifier.weight(1f).padding(start = if (index == 0) 0.dp else 20.dp)) {
                Text(
                    if (available) value.toString() else "—",
                    color = if (value > 0 || label == "Total files") tint else SecondaryText,
                    fontSize = 28.sp,
                    fontWeight = FontWeight.Medium)
                Text(
                    label,
                    color = SecondaryText,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = 5.dp))
              }
            }
          }
        }
      }
    }
  }
}

@Composable
private fun AnalysisRunCard(
    run: AnalyzeAllRunPresentation,
    job: AnalyzeAllJob?,
    modifier: Modifier,
) {
  val tint =
      when (run.statusLabel) {
        "Failed" -> Error
        "Completed" -> Success
        "Running" -> SelectionText
        "Paused",
        "Pausing",
        "Canceling",
        "Stale" -> Warning
        else -> SecondaryText
      }
  MiniOrcaPanel(modifier, contentPadding = PaddingValues(20.dp)) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
      SectionLabel("Current run")
      Spacer(Modifier.weight(1f))
      Text(
          run.statusLabel,
          color = tint,
          fontSize = 12.sp,
          modifier =
              Modifier.background(tint.copy(alpha = 0.10f), MiniOrcaShapes.small)
                  .padding(horizontal = 8.dp, vertical = 4.dp))
    }
    Spacer(Modifier.height(20.dp))
    Text(
        if (run.candidates > 0) "${run.completed + run.failed} of ${run.candidates} files processed"
        else "No files queued",
        color = PrimaryText,
        fontSize = 16.sp,
        fontWeight = FontWeight.Medium)
    LinearProgressIndicator(
        progress = analysisRunProgress(run),
        modifier = Modifier.fillMaxWidth().padding(top = 12.dp).height(4.dp),
        color = tint,
        backgroundColor = StrongSurface)
    Text(
        "${run.completed} complete  ·  ${run.running} running  ·  ${run.remaining} remaining  ·  ${run.failed} failed",
        color = SecondaryText,
        fontSize = 12.sp,
        modifier = Modifier.padding(top = 12.dp))
    job?.files
        ?.firstOrNull { it.status.equals("running", ignoreCase = true) }
        ?.let { file ->
          Row(Modifier.padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
            DesktopLineIcon(DesktopIcon.File, "Current file", iconSize = 16.dp)
            Spacer(Modifier.width(8.dp))
            Text(
                file.path,
                color = SecondaryText,
                fontSize = 12.sp,
                fontFamily = FontFamily.Monospace,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis)
          }
        }
    Text(
        run.statusDetail,
        color = SecondaryText,
        fontSize = 12.sp,
        lineHeight = 18.sp,
        modifier = Modifier.padding(top = 16.dp))
    if (job != null) {
      Divider(Modifier.padding(vertical = 16.dp), color = Border)
      Text(
          "${run.maxFiles} file limit  ·  ${run.maxRetries} retries per file",
          color = FaintText,
          fontSize = 12.sp)
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
            label = "File limit (1–500)",
            modifier = modifier)
      },
      second = { modifier ->
        CompactSingleLineField(
            state.maxRetries,
            actions.updateMaxRetries,
            label = "Retry limit (0–3)",
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
