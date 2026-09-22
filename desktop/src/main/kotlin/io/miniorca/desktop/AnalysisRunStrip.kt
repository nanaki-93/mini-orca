package io.miniorca.desktop

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
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
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp

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
  val currentPaths = presentation.currentFiles
  val inlineThreshold = if (scope == AnalysisRunStripScope.Analysis) 900.dp else 760.dp

  MiniOrcaPanel(
      modifier = modifier.testTag("analysis-run-strip"), contentPadding = PaddingValues(12.dp)) {
        val isAnalysisPage = scope == AnalysisRunStripScope.Analysis
        if (isAnalysisPage) {
          AnalysisRunTitle(run, presentation)
        }
        BoxWithConstraints(Modifier.fillMaxWidth()) {
          val inline = maxWidth / LocalDensity.current.fontScale >= inlineThreshold
          Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            if (inline) {
              Row(
                  Modifier.fillMaxWidth(),
                  horizontalArrangement = Arrangement.spacedBy(12.dp),
                  verticalAlignment = Alignment.CenterVertically) {
                    AnalysisRunMetadata(
                        run,
                        presentation,
                        currentPaths,
                        pathsExpanded,
                        { pathsExpanded = !pathsExpanded },
                        Modifier.weight(1f),
                        showFileCount = !isAnalysisPage)
                    run?.let {
                      AnalysisRunProgressTrack(
                          presentation,
                          analysisStatusTint(it.status),
                          Modifier.weight(1f),
                          showPercent = isAnalysisPage)
                    }
                    AnalysisRunControls(state, commands, actions)
                  }
            } else {
              Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                AnalysisRunMetadata(
                    run,
                    presentation,
                    currentPaths,
                    pathsExpanded,
                    { pathsExpanded = !pathsExpanded },
                    Modifier.fillMaxWidth(),
                    showFileCount = !isAnalysisPage)
                run?.let {
                  AnalysisRunProgressTrack(
                      presentation,
                      analysisStatusTint(it.status),
                      Modifier.fillMaxWidth(),
                      showPercent = isAnalysisPage)
                }
                AnalysisRunControls(state, commands, actions)
              }
            }
            if (pathsExpanded)
                currentPaths.drop(1).forEach { path ->
                  Text(
                      "Current: $path",
                      color = SecondaryText,
                      style = IdeTypography.workspaceMetadata)
                }
          }
        }
        if (isAnalysisPage) {
          if (presentation.headline != "Current run")
              Text(
                  presentation.headline,
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
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
private fun AnalysisRunTitle(run: AnalysisRun?, presentation: ProjectRunPresentation) {
  Row(
      Modifier.fillMaxWidth().padding(bottom = 10.dp),
      horizontalArrangement = Arrangement.spacedBy(10.dp),
      verticalAlignment = Alignment.CenterVertically) {
        val tint = analysisStatusTint(run?.status)
        Box(Modifier.size(28.dp), contentAlignment = Alignment.Center) {
          if (presentation.isActive) {
            IdeBusyIndicator(Modifier.size(22.dp), color = tint, strokeWidth = 2.dp)
          } else {
            Canvas(Modifier.size(10.dp)) { drawCircle(tint) }
          }
        }
        Column {
          Text(analysisRunTitle(run, presentation), style = IdeTypography.workspaceHeading)
          if (run != null) {
            Text(
                if (presentation.totalFiles > 0)
                    "${presentation.finishedFiles} of ${presentation.totalFiles} files finished"
                else "File progress unavailable",
                color = SecondaryText,
                style = IdeTypography.compactBody)
          }
        }
      }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisRunMetadata(
    run: AnalysisRun?,
    presentation: ProjectRunPresentation,
    currentPaths: List<String>,
    pathsExpanded: Boolean,
    onTogglePaths: () -> Unit,
    modifier: Modifier,
    showFileCount: Boolean = true,
) {
  FlowRow(
      modifier = modifier,
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(6.dp)) {
        IdeLabelBadge(
            if (run == null) "Ready" else presentation.status, analysisStatusTint(run?.status))
        if (run != null && showFileCount) {
          Text(
              if (presentation.totalFiles > 0)
                  "${presentation.finishedFiles} of ${presentation.totalFiles} files finished"
              else "File progress unavailable",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
        }
        if (currentPaths.isNotEmpty()) {
          Text(
              "Current: ${currentPaths.first()}",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata,
              modifier = Modifier.widthIn(max = 520.dp))
          if (currentPaths.size > 1)
              ChromeButton(
                  onClick = onTogglePaths,
                  accessibleName =
                      if (pathsExpanded) "Hide active files" else "Show active files") {
                    Text(
                        if (pathsExpanded) "Hide active files"
                        else "+${currentPaths.size - 1} active files",
                        style = IdeTypography.workspaceMetadata)
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
              "${(fileProgress * 100).toInt()}%",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
        }
      }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun AnalysisRunControls(
    state: AnalysisWorkspacePaneState,
    commands: List<AnalysisRunCommand>,
    actions: AnalysisWorkspaceActions?,
) {
  if (actions == null || commands.isEmpty()) return
  FlowRow(
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
