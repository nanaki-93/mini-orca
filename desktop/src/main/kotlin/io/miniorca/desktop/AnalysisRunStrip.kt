package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlin.math.roundToInt

internal enum class AnalysisRunStripScope {
  Analysis,
  Summary,
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
internal fun AnalysisRunStrip(
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions?,
    scope: AnalysisRunStripScope,
    modifier: Modifier = Modifier,
    onOpenAnalysis: (() -> Unit)? = null,
) {
  val run = state.analysis.run
  val presentation = projectRunPresentation(state.project, state.analysis)
  val commands =
      when (scope) {
        AnalysisRunStripScope.Analysis -> presentation.commands
        AnalysisRunStripScope.Summary ->
            presentation.commands.filter {
              it in
                  setOf(
                      AnalysisRunCommand.Pause,
                      AnalysisRunCommand.Resume,
                      AnalysisRunCommand.Cancel)
            }
      }
  var pathsExpanded by remember(run?.identity) { mutableStateOf(false) }

  MiniOrcaPanel(
      modifier = modifier.testTag("analysis-run-strip"),
      contentPadding =
          if (scope == AnalysisRunStripScope.Analysis) PaddingValues(16.dp)
          else PaddingValues(12.dp)) {
        when (scope) {
          AnalysisRunStripScope.Analysis ->
              AnalysisRunPanel(state, run, presentation, commands, actions, pathsExpanded) {
                pathsExpanded = !pathsExpanded
              }
          AnalysisRunStripScope.Summary ->
              SummaryRunPanel(
                  state, run, presentation, commands, actions, pathsExpanded, onOpenAnalysis) {
                    pathsExpanded = !pathsExpanded
                  }
        }
        if (analysisRunOutdated(run, state.project))
            Text(
                if (analysisRunRevisionOutdated(run, state.project)) "Outdated · previous revision"
                else "Outdated · another project",
                color = Warning,
                style = IdeTypography.workspaceMetadata)
        if (scope == AnalysisRunStripScope.Analysis) {
          if (run?.plan?.compatibilityStage?.isNotBlank() == true)
              Text(
                  "Saved limited run: ${analysisStageLabel(run.plan.compatibilityStage)}",
                  color = Warning,
                  style = IdeTypography.workspaceMetadata)
          if (run?.plan?.retryStaleFailed == true)
              Text(
                  "Scope: stale & failed files",
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
        }
        if (scope == AnalysisRunStripScope.Summary) AnalysisActionFeedback(state.analysis)
      }
}

@Composable
private fun AnalysisRunPanel(
    state: AnalysisWorkspacePaneState,
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    commands: List<AnalysisRunCommand>,
    actions: AnalysisWorkspaceActions?,
    pathsExpanded: Boolean,
    onTogglePaths: () -> Unit,
) {
  val refreshFocus = remember(state.project?.projectId) { FocusRequester() }
  var focusedCommand by
      remember(state.project?.projectId) { mutableStateOf<AnalysisRunCommand?>(null) }
  val currentCommands by rememberUpdatedState(commands)
  LaunchedEffect(commands, focusedCommand) {
    if (focusedCommand != null && focusedCommand !in commands) {
      if (actions != null &&
          state.project != null &&
          (run != null || state.analysis.error != null) &&
          state.analysis.action.isBlank())
          refreshFocus.requestFocus()
      focusedCommand = null
    }
  }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
      AnalysisLifecycleIndicator(run, presentation, state.analysis)
      AnalysisRunContent(
          run,
          presentation,
          state.analysis,
          pathsExpanded,
          onTogglePaths,
          Modifier.weight(1f).testTag("analysis-run-content"))
    }
    AnalysisActionFeedback(state.analysis)
    if (actions != null && state.project != null && (run != null || state.analysis.error != null))
        MiniOrcaButton(
            onClick = actions.refreshStatus,
            enabled = state.analysis.action.isBlank(),
            tone = ActionTone.Neutral,
            modifier =
                Modifier.focusRequester(refreshFocus)
                    .semantics { contentDescription = "Refresh analysis run status" }
                    .testTag("analysis-refresh-status")) {
              Text("Refresh status", style = IdeTypography.action)
            }
    AnalysisRunDiagnostic(run)
    AnalysisRunControls(
        state,
        commands,
        actions,
        Modifier.testTag("analysis-run-controls"),
        onFocusChanged = { command, focused ->
          if (focused) focusedCommand = command
          else if (focusedCommand == command && command in currentCommands) focusedCommand = null
        })
    if (pathsExpanded) AnalysisExpandedPaths(presentation.currentFiles)
    if (run != null || state.analysis.previousRun != null) {
      var detailsExpanded by
          remember(run?.identity ?: state.analysis.previousRun?.identity) { mutableStateOf(false) }
      IdeDisclosureHeader("Run details", detailsExpanded, { detailsExpanded = !detailsExpanded })
      if (detailsExpanded) {
        if (run != null) {
          analysisRunSupplementalMetadata(run, presentation)?.let {
            Text(it, color = SecondaryText, style = IdeTypography.workspaceMetadata)
          }
          Text(
              presentation.reportedAttempts?.let { "$it attempts" } ?: "Attempts unavailable",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
          AnalysisStageRows(run, presentation)
        }
        AnalysisRunHistory(state.project, run, state.analysis.previousRun)
      }
    }
  }
}

@Composable
private fun AnalysisLifecycleIndicator(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    analysis: ProjectAnalysisRunState
) {
  val uncertain = analysisLifecycleStatusLabel(presentation, analysis) != presentation.status
  val tint = if (uncertain) Warning else analysisRunDisplayTint(run, presentation)
  Box(Modifier.size(28.dp), contentAlignment = Alignment.Center) {
    if (presentation.isActive) {
      IdeBusyIndicator(Modifier.size(22.dp), color = tint, strokeWidth = 2.dp)
    } else {
      DesktopLineIcon(
          if (uncertain) DesktopIcon.Warning
          else analysisStatusIcon(if (presentation.status == "Stale") "stale" else run?.status),
          "",
          tint = tint)
    }
  }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisRunContent(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    analysis: ProjectAnalysisRunState,
    pathsExpanded: Boolean,
    onTogglePaths: () -> Unit,
    modifier: Modifier,
) {
  Column(modifier, verticalArrangement = Arrangement.spacedBy(8.dp)) {
    val lifecycle = analysisLifecycleStatusLabel(presentation, analysis)
    Text(
        if (lifecycle != presentation.status) lifecycle else analysisRunTitle(run, presentation),
        style = IdeTypography.workspaceHeading)
    if (run != null) {
      val outcome = analysisRunOutcomeLabel(presentation)
      FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        if (outcome == null || presentation.isActive)
            Text(
                analysisFileProgressLabel(presentation),
                color = SecondaryText,
                style = IdeTypography.compactBody)
        if (outcome != null)
            Text(
                outcome,
                color = Warning,
                style = IdeTypography.compactBody,
                modifier = Modifier.testTag("analysis-run-attention"))
      }
      if (presentation.isActive)
          AnalysisRunProgressTrack(
              presentation,
              analysisRunDisplayTint(run, presentation),
              Modifier.fillMaxWidth(),
              showPercent = true)
    }
    AnalysisCurrentFiles(presentation.currentFiles, pathsExpanded, onTogglePaths)
  }
}

internal fun analysisStageBreakdown(stage: AnalysisStageSummary): String =
    buildList {
          add("${stage.finished}/${stage.total} finished")
          if (stage.running > 0) add("${stage.running} running")
          if (stage.pending > 0) add("${stage.pending} pending")
          if (stage.missing > 0) add("${stage.missing} unreported")
          val ineligible = stage.files.count { !it.eligible }
          if (ineligible > 0) add("$ineligible not applicable")
          stage.files
              .filter {
                it.eligible &&
                    it.status !in setOf(null, "completed", "completed_empty", "running", "pending")
              }
              .groupingBy { it.status!! }
              .eachCount()
              .forEach { (status, count) ->
                add("$count ${analysisStatusLabel(status).lowercase()}")
              }
        }
        .joinToString(" · ")

internal fun analysisRunOutcomeLabel(presentation: ProjectRunPresentation): String? {
  val stages = presentation.stages.flatMap { it.files }.filter { it.eligible }
  val facts = buildList {
    val failures = stages.count { it.status == "failed" }
    if (failures > 0) add("$failures ${if (failures == 1) "failure" else "failures"}")
    val missing = stages.filter { it.status == null }.map { it.path }.distinct().size
    if (missing > 0) add("$missing missing ${if (missing == 1) "file" else "files"}")
    stages
        .filter { it.status in setOf("unavailable", "interrupted", "partial") }
        .groupingBy { it.status!! }
        .eachCount()
        .forEach { (status, count) -> add("$count ${analysisStatusLabel(status).lowercase()}") }
  }
  return facts.takeIf { it.isNotEmpty() }?.joinToString(" · ")
}

@Composable
private fun AnalysisRunDiagnostic(run: AnalysisRun?) {
  if (run?.reason?.isNotBlank() != true) return
  var expanded by remember(run.identity) { mutableStateOf(false) }
  IdeDisclosureHeader("Run diagnostic", expanded, { expanded = !expanded })
  if (expanded) DiagnosticText(run.reason, color = Warning)
}

@Composable
private fun AnalysisStageRows(run: AnalysisRun, presentation: ProjectRunPresentation) {
  if (presentation.stages.isEmpty()) return
  Column(
      Modifier.fillMaxWidth().testTag("analysis-stage-rows"),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text("Captured stages", style = IdeTypography.resultHeading, color = PrimaryText)
        presentation.stages.forEach { stage ->
          var expanded by remember(run.identity, stage.stage) { mutableStateOf(false) }
          val label = analysisStageLabel(stage.stage)
          IdeDisclosureHeader(
              "$label · ${analysisStageBreakdown(stage)}",
              expanded,
              { expanded = !expanded },
              modifier = Modifier.testTag("analysis-stage-${stage.stage}"))
          if (expanded) {
            Column(
                Modifier.fillMaxWidth()
                    .heightIn(max = 240.dp)
                    .verticalScroll(rememberScrollState())
                    .testTag("analysis-stage-details-${stage.stage}"),
                verticalArrangement = Arrangement.spacedBy(8.dp)) {
                  stage.files.forEach { detail ->
                    SelectionContainer {
                      Column {
                        Text(
                            detail.path,
                            color = PrimaryText,
                            style = IdeTypography.workspaceMetadata)
                        Text(
                            analysisStageDetailLabel(detail),
                            color = SecondaryText,
                            style = IdeTypography.workspaceMetadata)
                      }
                    }
                    DiagnosticText(detail.reason, color = SecondaryText)
                  }
                }
          }
        }
      }
}

private val savedRunStatuses =
    setOf("completed", "completed_empty", "partial", "failed", "unavailable", "canceled")

internal fun savedRunIdentityLabel(run: AnalysisRun): String =
    "Project ${run.identity.projectId} · Revision ${run.identity.projectRevision} · " +
        "Queue ${run.identity.queueId} · Run ${run.identity.id} · Generation ${run.identity.generation} · " +
        "Policy ${run.identity.policyFingerprint} · Provider ${run.identity.providerFingerprint}"

internal fun savedRunScopeLabel(run: AnalysisRun): String =
    "Captured scope · ${run.plan.scope.ifBlank { "unreported" }} · " +
        "${run.plan.files.size} planned ${if (run.plan.files.size == 1) "file" else "files"}" +
        if (run.plan.retryStaleFailed) " · stale & failed retry" else ""

@Composable
private fun AnalysisRunHistory(
    project: ProjectAnalysis?,
    current: AnalysisRun?,
    previous: AnalysisRun?,
) {
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
    if (current != null && current.status in savedRunStatuses) {
      Text("Latest saved run", color = PrimaryText, style = IdeTypography.resultHeading)
      SelectionContainer {
        Column {
          Text(
              savedRunIdentityLabel(current),
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
          Text(
              savedRunScopeLabel(current),
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
        }
      }
    }
    if (previous != null && previous.identity != current?.identity) {
      var expanded by remember(current?.identity, previous.identity) { mutableStateOf(false) }
      IdeDisclosureHeader(
          "Previous observed run · ${analysisStatusLabel(previous.status)}",
          expanded,
          { expanded = !expanded },
          modifier = Modifier.testTag("analysis-previous-run"))
      if (expanded) {
        val outdated =
            project == null ||
                previous.identity.projectId != project.projectId ||
                previous.identity.projectRevision != project.projectRevision
        Column(
            Modifier.fillMaxWidth()
                .heightIn(max = 280.dp)
                .verticalScroll(rememberScrollState())
                .testTag("analysis-previous-run-details"),
            verticalArrangement = Arrangement.spacedBy(6.dp)) {
              if (outdated)
                  Text(
                      "Outdated · previous run belongs to another project revision; not current evidence.",
                      color = Warning,
                      style = IdeTypography.compactBody)
              SelectionContainer {
                Column {
                  Text(
                      savedRunIdentityLabel(previous),
                      color = PrimaryText,
                      style = IdeTypography.workspaceMetadata)
                  Text(
                      savedRunScopeLabel(previous),
                      color = SecondaryText,
                      style = IdeTypography.workspaceMetadata)
                  Text(
                      "Lifecycle · ${analysisStatusLabel(previous.status)}",
                      color = SecondaryText,
                      style = IdeTypography.workspaceMetadata)
                  analysisRunTimeMetadata(previous).forEach { fact ->
                    Text(fact, color = SecondaryText, style = IdeTypography.workspaceMetadata)
                  }
                }
              }
              if (previous.reason.isNotBlank() ||
                  previous.status in setOf("failed", "unavailable", "interrupted", "partial")) {
                Text("Run diagnostic", color = Warning, style = IdeTypography.compactBody)
                DiagnosticText(
                    previous.reason.ifBlank { "No diagnostic was supplied for this run." },
                    color = Warning)
              }
              // Inspect only the saved snapshot against its own captured revision; never use it
              // for the current overview's counts, controls or result pages.
              val history =
                  project
                      ?.takeIf { it.projectId == previous.identity.projectId }
                      ?.copy(projectRevision = previous.identity.projectRevision)
                      ?.let { projectRunPresentation(it, ProjectAnalysisRunState(run = previous)) }
              when (history?.progressAvailability) {
                null,
                RunProgressAvailability.Unavailable ->
                    Text(
                        "Stage failure details unavailable for this captured run.",
                        color = Warning,
                        style = IdeTypography.compactBody)
                RunProgressAvailability.Incomplete ->
                    Text(
                        "Stage failure record incomplete · only validated failures shown.",
                        color = Warning,
                        style = IdeTypography.compactBody)
                else -> Unit
              }
              if (history?.failures?.isNotEmpty() == true) {
                Text("Reported stage failures", color = Warning, style = IdeTypography.compactBody)
                history.failures.forEach { failure ->
                  SelectionContainer {
                    Text(
                        "${failure.path} · ${analysisStageLabel(failure.stage)} · ${failure.attempts} attempts reported",
                        color = SecondaryText,
                        style = IdeTypography.workspaceMetadata)
                  }
                  DiagnosticText(failure.reason, color = Warning)
                }
              } else if (history?.progressAvailability in
                  setOf(RunProgressAvailability.Available, RunProgressAvailability.EmptyScope)) {
                Text(
                    "No stage failures reported in this saved run.",
                    color = SecondaryText,
                    style = IdeTypography.workspaceMetadata)
              }
            }
      }
    }
  }
}

private fun analysisStageDetailLabel(detail: AnalysisStageDetail): String {
  val status = analysisStatusLabel(detail.status ?: "unreported")
  val reuse =
      when (detail.reused) {
        true -> "reported reused"
        false -> "not reported reused"
        null -> "unreported"
      }
  return "${if (detail.eligible) status else "Not applicable · $status"} · Attempts reported: ${detail.attempts?.toString() ?: "unreported"} · Reuse: $reuse"
}

private fun analysisRunSupplementalMetadata(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
): String? {
  if (run == null) return null
  val facts = mutableListOf<String>()
  if (presentation.isActive) {
    if (run.windowFilesCompleted > 0) {
      facts +=
          "${run.windowFilesCompleted} ${if (run.windowFilesCompleted == 1) "file" else "files"} processed in current window"
    }
  } else if (presentation.progressAvailability == RunProgressAvailability.Available &&
      presentation.totalSteps > 0) {
    facts += "${presentation.finishedSteps} of ${presentation.totalSteps} stages"
  }
  facts += analysisRunTimeMetadata(run)
  return facts.joinToString(" · ")
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun SummaryRunPanel(
    state: AnalysisWorkspacePaneState,
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    commands: List<AnalysisRunCommand>,
    actions: AnalysisWorkspaceActions?,
    pathsExpanded: Boolean,
    onOpenAnalysis: (() -> Unit)?,
    onTogglePaths: () -> Unit,
) {
  val currentPaths = presentation.currentFiles
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
    AnalysisRunMetadata(
        run, presentation, state.analysis, currentPaths, pathsExpanded, onTogglePaths)
    if (run != null && presentation.isActive)
        AnalysisRunProgressTrack(
            presentation, analysisRunDisplayTint(run, presentation), Modifier.fillMaxWidth())
    AnalysisRunDiagnostic(run)
    if (onOpenAnalysis != null &&
        run != null &&
        (state.analysis.statusUnavailable ||
            state.analysis.error != null ||
            state.analysis.controlRequest?.outcome == AnalysisControlOutcome.Unconfirmed))
        MiniOrcaButton(onClick = onOpenAnalysis, tone = ActionTone.Navigation) {
          Text("View analysis", style = IdeTypography.action)
        }
    AnalysisRunControls(state, commands, actions)
    if (pathsExpanded) AnalysisExpandedPaths(currentPaths)
  }
}

private fun analysisRunDisplayTint(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
): androidx.compose.ui.graphics.Color =
    analysisStatusTint(if (presentation.status == "Stale") "stale" else run?.status)

private fun analysisFileProgressLabel(presentation: ProjectRunPresentation): String =
    when (presentation.progressAvailability) {
      RunProgressAvailability.Available ->
          "${presentation.finishedFiles}/${presentation.totalFiles} files finished"
      RunProgressAvailability.EmptyScope -> "Empty run scope"
      RunProgressAvailability.Incomplete -> "Progress incomplete"
      RunProgressAvailability.NotStarted,
      RunProgressAvailability.Unavailable -> "Progress unavailable"
    }

internal fun analysisLifecycleStatusLabel(
    presentation: ProjectRunPresentation,
    analysis: ProjectAnalysisRunState,
): String =
    when {
      analysis.statusUnavailable ||
          (analysis.error != null && analysis.errorKind == AnalysisRunErrorKind.StatusRead) ->
          "Status unavailable"
      analysis.controlRequest?.outcome == AnalysisControlOutcome.Unconfirmed ->
          "Control outcome unconfirmed"
      analysis.controlRequest?.outcome == AnalysisControlOutcome.Reconciling ->
          "Checking run status"
      analysis.controlRequest?.outcome == AnalysisControlOutcome.Requesting ->
          presentation.lifecycleExplanation ?: presentation.status
      else -> presentation.status
    }

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisRunMetadata(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    analysis: ProjectAnalysisRunState,
    currentPaths: List<String>,
    pathsExpanded: Boolean,
    onTogglePaths: () -> Unit,
    modifier: Modifier = Modifier,
) {
  FlowRow(
      modifier = modifier,
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(6.dp)) {
        val status =
            if (run == null) "Ready" else analysisLifecycleStatusLabel(presentation, analysis)
        val uncertain = status != presentation.status && run != null
        val outcome = analysisRunOutcomeLabel(presentation)
        IdeLabelBadge(
            listOfNotNull(status, outcome).joinToString(" · "),
            if (uncertain) Warning else analysisRunDisplayTint(run, presentation),
            icon =
                if (uncertain) DesktopIcon.Warning
                else
                    analysisStatusIcon(
                        if (presentation.status == "Stale") "stale" else run?.status))
        if (run != null) {
          if (uncertain)
              Text(
                  "Saved · ${presentation.status}",
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
          if (outcome == null || presentation.isActive)
              Text(
                  analysisFileProgressLabel(presentation),
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
        }
        AnalysisCurrentFiles(currentPaths, pathsExpanded, onTogglePaths)
      }
}

@Composable
private fun AnalysisCurrentFiles(
    currentPaths: List<String>,
    pathsExpanded: Boolean,
    onTogglePaths: () -> Unit,
) {
  if (currentPaths.isNotEmpty()) {
    Text(
        "Current: ${currentPaths.first()}",
        color = SecondaryText,
        style = IdeTypography.workspaceMetadata,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = Modifier.widthIn(max = 520.dp))
    ChromeButton(
        onClick = onTogglePaths,
        accessibleName = if (pathsExpanded) "Hide active files" else "Show active files",
        modifier =
            Modifier.semantics {
              stateDescription = if (pathsExpanded) "Expanded" else "Collapsed"
            }) {
          Text(
              if (pathsExpanded) "Hide paths"
              else if (currentPaths.size == 1) "Full path" else "+${currentPaths.size - 1} files",
              style = IdeTypography.workspaceMetadata)
        }
  }
}

@Composable
private fun AnalysisExpandedPaths(currentPaths: List<String>) {
  SelectionContainer {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
      currentPaths.forEach { path ->
        Text("Current: $path", color = SecondaryText, style = IdeTypography.workspaceMetadata)
      }
    }
  }
}

@Composable
private fun AnalysisRunProgressTrack(
    presentation: ProjectRunPresentation,
    color: androidx.compose.ui.graphics.Color,
    modifier: Modifier,
    showPercent: Boolean = false,
) {
  val fileProgress = presentation.fileProgress
  val description =
      if (fileProgress == null) analysisFileProgressLabel(presentation)
      else "Files finished: ${presentation.finishedFiles} of ${presentation.totalFiles}"
  Row(
      modifier,
      horizontalArrangement = Arrangement.spacedBy(10.dp),
      verticalAlignment = Alignment.CenterVertically) {
        val trackModifier =
            Modifier.weight(1f)
                .height(12.dp)
                .semantics { contentDescription = description }
                .testTag("analysis-run-progress-track")
        if (fileProgress == null) {
          Box(
              trackModifier.clip(MiniOrcaShapes.pill).background(StrongSurface),
              contentAlignment = Alignment.Center) {
                if (presentation.isActive) {
                  IdeBusyIndicator(Modifier.size(10.dp), color = color, strokeWidth = 2.dp)
                }
              }
        } else {
          IdeProgressBar(fileProgress, trackModifier, color = color, trackColor = StrongSurface)
        }
        if (showPercent && fileProgress != null) {
          Text(
              "${(fileProgress * 100).roundToInt()}%",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata,
              modifier = Modifier.testTag("analysis-run-progress-percent"))
        }
      }
}

@Composable
internal fun AnalysisActionFeedback(analysis: ProjectAnalysisRunState) {
  if (analysis.controlRequest?.outcome != AnalysisControlOutcome.Requesting)
      analysis.action
          .takeIf { it.isNotEmpty() }
          ?.let {
            Text(
                "Analysis: ${it.replaceFirstChar { character -> character.uppercase() }}…",
                style = IdeTypography.workspaceMetadata,
                color = SelectionText)
          }
  analysis.error?.let {
    DiagnosticText(it.ifBlank { "No failure details available." }, color = Error)
  }
}

internal fun analysisRunActionEnabled(state: AnalysisWorkspacePaneState): Boolean =
    state.project != null && state.analysis.action.isBlank() && !state.analysis.fileSelection.saving

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisRunControls(
    state: AnalysisWorkspacePaneState,
    commands: List<AnalysisRunCommand>,
    actions: AnalysisWorkspaceActions?,
    modifier: Modifier = Modifier,
    onFocusChanged: (AnalysisRunCommand, Boolean) -> Unit = { _, _ -> },
) {
  if (actions == null || commands.isEmpty()) return
  FlowRow(
      modifier = modifier,
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        commands.forEach { command ->
          MiniOrcaButton(
              onClick = {
                when (command) {
                  AnalysisRunCommand.Start,
                  AnalysisRunCommand.RetryStaleFailed ->
                      actions.start(
                          defaultAnalysisRunLimits, command == AnalysisRunCommand.RetryStaleFailed)
                  AnalysisRunCommand.Pause -> actions.pause()
                  AnalysisRunCommand.Resume -> actions.resume()
                  AnalysisRunCommand.Cancel -> actions.cancel()
                }
              },
              enabled = analysisRunActionEnabled(state),
              modifier =
                  Modifier.onFocusChanged { onFocusChanged(command, it.isFocused) }
                      .semantics {
                        contentDescription =
                            when (command) {
                              AnalysisRunCommand.Resume -> "Resume analysis with a fresh preview"
                              AnalysisRunCommand.Cancel -> "Cancel analysis run"
                              AnalysisRunCommand.Pause ->
                                  "Pause analysis at the next stage boundary"
                              AnalysisRunCommand.Start -> "Start new analysis"
                              AnalysisRunCommand.RetryStaleFailed ->
                                  "Retry stale and failed analysis"
                            }
                      },
              tone =
                  when (command) {
                    AnalysisRunCommand.Cancel -> ActionTone.Destructive
                    AnalysisRunCommand.RetryStaleFailed -> ActionTone.Neutral
                    else -> ActionTone.Primary
                  }) {
                Text(
                    if (command == AnalysisRunCommand.Start &&
                        state.analysis.run?.status == "canceled")
                        "Start new analysis"
                    else command.label,
                    style = IdeTypography.action)
              }
        }
      }
}
