package io.miniorca.desktop

import androidx.compose.foundation.Canvas
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
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
) {
  val run = state.analysis.run
  val presentation = projectRunPresentation(state.analysis)
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
  var pathsExpanded by remember(run?.identity, presentation.currentFiles) { mutableStateOf(false) }

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
              SummaryRunPanel(state, run, presentation, commands, actions, pathsExpanded) {
                pathsExpanded = !pathsExpanded
              }
        }
        if (scope == AnalysisRunStripScope.Analysis) {
          run?.reason?.takeIf { it.isNotBlank() }?.let { DiagnosticText(it, color = Warning) }
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
        state.analysis.action
            .takeIf { it.isNotEmpty() }
            ?.let {
              Text(
                  "${it.replaceFirstChar { character -> character.uppercase() }}…",
                  style = IdeTypography.workspaceMetadata,
                  color = SelectionText)
            }
        state.analysis.error?.let { DiagnosticText(it, color = Error) }
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
  Row(
      Modifier.fillMaxWidth(),
      horizontalArrangement = Arrangement.spacedBy(12.dp),
      verticalAlignment = Alignment.Top) {
        AnalysisLifecycleIndicator(run, presentation)
        AnalysisRunContent(
            run,
            presentation,
            pathsExpanded,
            onTogglePaths,
            Modifier.weight(1f).testTag("analysis-run-content"))
        AnalysisRunControls(state, commands, actions, Modifier.testTag("analysis-run-controls"))
      }
}

@Composable
private fun AnalysisLifecycleIndicator(run: AnalysisRun?, presentation: ProjectRunPresentation) {
  val tint = analysisStatusTint(run?.status)
  Box(Modifier.size(28.dp), contentAlignment = Alignment.Center) {
    if (presentation.isActive) {
      IdeBusyIndicator(Modifier.size(22.dp), color = tint, strokeWidth = 2.dp)
    } else {
      Canvas(Modifier.size(10.dp)) { drawCircle(tint) }
    }
  }
}

@Composable
private fun AnalysisRunContent(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    pathsExpanded: Boolean,
    onTogglePaths: () -> Unit,
    modifier: Modifier,
) {
  Column(modifier, verticalArrangement = Arrangement.spacedBy(8.dp)) {
    Column {
      Text(analysisRunTitle(run, presentation), style = IdeTypography.workspaceHeading)
      Text(
          analysisFileProgressLabel(presentation),
          color = SecondaryText,
          style = IdeTypography.compactBody)
    }
    if (run != null) {
      AnalysisRunProgressTrack(
          presentation, analysisStatusTint(run.status), Modifier.fillMaxWidth(), showPercent = true)
    }
    AnalysisCurrentFiles(
        presentation.currentFiles, pathsExpanded, onTogglePaths, showExpandedPaths = true)
    analysisRunSupplementalMetadata(run, presentation)?.let { metadata ->
      Text(metadata, color = SecondaryText, style = IdeTypography.workspaceMetadata)
    }
  }
}

private fun analysisRunSupplementalMetadata(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
): String? {
  if (run == null) return null
  val facts = mutableListOf<String>()
  if (run.isActive()) {
    if (run.windowFilesCompleted > 0) {
      facts +=
          "${run.windowFilesCompleted} ${if (run.windowFilesCompleted == 1) "file" else "files"} processed"
    }
    if (run.windowElapsedSeconds > 0) facts += "${run.windowElapsedSeconds}s elapsed"
  } else {
    if (presentation.totalSteps > 0)
        facts += "${presentation.finishedSteps} of ${presentation.totalSteps} stages"
    run.updatedAt.takeIf { it.isNotBlank() }?.let(facts::add)
  }
  return facts.takeIf { it.isNotEmpty() }?.joinToString(" · ")
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
    onTogglePaths: () -> Unit,
) {
  val currentPaths = presentation.currentFiles
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically) {
          AnalysisRunMetadata(
              run, presentation, currentPaths, pathsExpanded, onTogglePaths, Modifier.weight(1f))
          run?.let {
            AnalysisRunProgressTrack(
                presentation, analysisStatusTint(it.status), Modifier.weight(1f))
          }
          AnalysisRunControls(state, commands, actions)
        }
    if (pathsExpanded)
        currentPaths.drop(1).forEach { path ->
          Text("Current: $path", color = SecondaryText, style = IdeTypography.workspaceMetadata)
        }
  }
}

private fun analysisFileProgressLabel(presentation: ProjectRunPresentation): String =
    if (presentation.totalFiles > 0)
        "${presentation.finishedFiles} of ${presentation.totalFiles} files finished"
    else "File progress unavailable"

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisRunMetadata(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    currentPaths: List<String>,
    pathsExpanded: Boolean,
    onTogglePaths: () -> Unit,
    modifier: Modifier,
) {
  FlowRow(
      modifier = modifier,
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(6.dp)) {
        IdeLabelBadge(
            if (run == null) "Ready" else presentation.status, analysisStatusTint(run?.status))
        if (run != null) {
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
    showExpandedPaths: Boolean = false,
) {
  if (currentPaths.isNotEmpty()) {
    Text(
        "Current: ${currentPaths.first()}",
        color = SecondaryText,
        style = IdeTypography.workspaceMetadata,
        modifier = Modifier.widthIn(max = 520.dp))
    if (currentPaths.size > 1)
        ChromeButton(
            onClick = onTogglePaths,
            accessibleName = if (pathsExpanded) "Hide active files" else "Show active files") {
              Text(
                  if (pathsExpanded) "Hide active files"
                  else "+${currentPaths.size - 1} active files",
                  style = IdeTypography.workspaceMetadata)
            }
    if (showExpandedPaths && pathsExpanded)
        currentPaths.drop(1).forEach { path ->
          Text("Current: $path", color = SecondaryText, style = IdeTypography.workspaceMetadata)
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
      if (fileProgress == null) "Files finished: progress unavailable"
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
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisRunControls(
    state: AnalysisWorkspacePaneState,
    commands: List<AnalysisRunCommand>,
    actions: AnalysisWorkspaceActions?,
    modifier: Modifier = Modifier,
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
              enabled =
                  state.project != null &&
                      state.analysis.action.isBlank() &&
                      !state.analysis.fileSelection.saving,
              tone =
                  when (command) {
                    AnalysisRunCommand.Cancel -> ActionTone.Destructive
                    AnalysisRunCommand.RetryStaleFailed -> ActionTone.Neutral
                    else -> ActionTone.Primary
                  }) {
                Text(command.label, style = IdeTypography.workspaceMetadata)
              }
        }
      }
}
