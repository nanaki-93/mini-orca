package io.miniorca.desktop

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
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
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun MainToolbar(
    state: ToolbarState,
    actions: ToolbarActions,
    modifier: Modifier = Modifier,
    paletteFocusRequester: FocusRequester? = null,
) {
  val connectionPresentation = connectionPresentation(state.connection)
  BoxWithConstraints(modifier.fillMaxWidth().background(ActivityRail)) {
    val scaledWidth = maxWidth.value / LocalDensity.current.fontScale
    val compact = maxWidth < 1000.dp
    val wrapped = scaledWidth < 1400f
    Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp)) {
      Row(
          Modifier.fillMaxWidth().heightIn(min = 40.dp),
          verticalAlignment = Alignment.CenterVertically) {
            MiniOrcaMark()
            Spacer(Modifier.width(12.dp))
            ProjectActionsMenu(
                projectType = state.project?.type,
                projectLabel = projectBreadcrumbLabel(state.project),
                projectAvailable = state.project != null,
                availability =
                    projectActionAvailability(
                        state.project,
                        state.openingAttempt,
                        state.indexingAttempt,
                        state.switchPending),
                reconnectAvailable = connectionPresentation.canReconnect,
                onImport = actions.onImport,
                onReindex = actions.onReindex,
                onReconnect = actions.onReconnect,
                modifier = Modifier.width(180.dp))
            IdeVerticalSeparator(Modifier.height(20.dp))
            BranchContext(state.gitStatus)
            if (!wrapped) {
              HeaderSearch(
                  actions.onPalette,
                  paletteFocusRequester,
                  Modifier.weight(1f).padding(start = 16.dp))
              Spacer(Modifier.width(16.dp))
              ToolbarStatus(state, connectionPresentation)
            }
          }
      state.openingAttempt?.let { attempt ->
        Spacer(Modifier.height(8.dp))
        ProjectOpeningFeedback(state.project, attempt, actions, state.switchPending)
      }
      if (state.preferenceReadWarning != null || state.preferenceSaveWarning != null) {
        Column(
            Modifier.fillMaxWidth()
                .heightIn(max = 160.dp)
                .verticalScroll(rememberScrollState())
                .testTag("project-preference-scroll")) {
              ProjectPreferenceWarnings(state.preferenceReadWarning, state.preferenceSaveWarning)
            }
      }
      if (wrapped) {
        Spacer(Modifier.height(4.dp))
        if (compact) {
          HeaderSearch(actions.onPalette, paletteFocusRequester, Modifier.fillMaxWidth())
          Spacer(Modifier.height(4.dp))
          Row(verticalAlignment = Alignment.CenterVertically) {
            ToolbarStatus(state, connectionPresentation)
          }
        } else {
          Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            HeaderSearch(actions.onPalette, paletteFocusRequester, Modifier.weight(1f))
            Spacer(Modifier.width(16.dp))
            ToolbarStatus(state, connectionPresentation)
          }
        }
      }
    }
  }
}

@Composable
private fun HeaderSearch(onClick: () -> Unit, focusRequester: FocusRequester?, modifier: Modifier) {
  Box(modifier, contentAlignment = Alignment.CenterStart) {
    ChromeButton(
        onClick = onClick,
        background = ToolWindowSurface,
        modifier =
            Modifier.widthIn(max = 420.dp)
                .fillMaxWidth()
                .then(focusRequester?.let { Modifier.focusRequester(it) } ?: Modifier)
                .semantics { contentDescription = "Search files, symbols, commands" }) {
          DesktopLineIcon(DesktopIcon.Search, "Search", iconSize = 16.dp)
          Spacer(Modifier.width(8.dp))
          Text(
              "Search files, symbols, commands",
              fontSize = 12.sp,
              maxLines = 1,
              overflow = TextOverflow.Ellipsis,
              modifier = Modifier.weight(1f))
          Text("⌘P", color = FaintText, fontSize = 11.sp, modifier = Modifier.padding(start = 8.dp))
        }
  }
}

internal data class ToolbarState(
    val project: ProjectAnalysis?,
    val busy: Boolean,
    val operationStatus: String,
    val connection: ConnectionState,
    val gitStatus: GitStatus?,
    val analysisStatus: ToolbarAnalysisStatus? = null,
    val openingAttempt: ProjectOpeningAttempt? = null,
    val indexingAttempt: ProjectIndexingAttempt? = null,
    val switchPending: Boolean = false,
    val preferenceReadWarning: String? = null,
    val preferenceSaveWarning: String? = null,
)

@Composable
private fun ProjectOpeningFeedback(
    project: ProjectAnalysis?,
    attempt: ProjectOpeningAttempt,
    actions: ToolbarActions,
    switchPending: Boolean,
) {
  val restoring = attempt.kind == ProjectOpeningKind.Restore
  val failure = attempt.outcome as? ProjectOpeningOutcome.Failed
  SystemStateMessage(
      title =
          when {
            failure != null ->
                if (restoring) "Could not restore project" else "Could not import project"
            attempt.outcome == ProjectOpeningOutcome.Opening ->
                if (restoring) "Restoring local project…" else "Importing project…"
            else -> if (restoring) "Restore canceled" else "Import canceled"
          },
      message =
          when {
            failure != null ->
                "The requested project did not open. The current project is unchanged."
            attempt.outcome == ProjectOpeningOutcome.Opening && restoring ->
                "Reading saved local project data; no model request is made. The current project remains open."
            attempt.outcome == ProjectOpeningOutcome.Opening ->
                "Import may use the configured Analyze provider and require confirmation. The current project remains open."
            else -> "The current project remains open. Switch project… to choose another folder."
          },
      accent = if (failure != null) Error else SecondaryText,
      modifier =
          Modifier.fillMaxWidth()
              .heightIn(max = 220.dp)
              .verticalScroll(rememberScrollState())
              .testTag("project-opening-scroll"),
      action = {
        Text("Current project", color = SecondaryText, style = IdeTypography.resultLabel)
        Text(projectBreadcrumbLabel(project), color = PrimaryText, style = IdeTypography.resultCode)
        Spacer(Modifier.height(8.dp))
        Text("Requested path", color = SecondaryText, style = IdeTypography.resultLabel)
        SelectionContainer {
          Text(
              attempt.path,
              color = PrimaryText,
              fontFamily = FontFamily.Monospace,
              style = IdeTypography.resultCode)
        }
        if (failure != null) {
          Spacer(Modifier.height(8.dp))
          Text("Opening diagnostic", color = SecondaryText, style = IdeTypography.resultLabel)
          DiagnosticText(failure.message, color = Error)
          Spacer(Modifier.height(8.dp))
          if (restoring) {
            MiniOrcaButton(
                onClick = actions.onRetryRestore,
                enabled = !switchPending,
                tone = ActionTone.Neutral) {
                  Text("Retry restore")
                }
            Spacer(Modifier.height(8.dp))
          }
          MiniOrcaButton(
              onClick = actions.onImport, enabled = !switchPending, tone = ActionTone.Neutral) {
                Text(if (project == null) "Open project…" else "Switch project…")
              }
        }
      })
}

internal data class ToolbarAnalysisStatus(
    val label: String,
    val detail: String,
    val running: Boolean,
    val attention: Boolean,
)

internal fun toolbarAnalysisStatus(state: DesktopState): ToolbarAnalysisStatus? {
  val project = state.project ?: return null
  val analysis = state.analysisRun
  val run = analysis.run?.takeIf { it.identity.projectId == project.projectId }
  if (run == null &&
      analysis.action.isBlank() &&
      analysis.admission == null &&
      analysis.error == null)
      return null
  val stale =
      run != null &&
          (run.status == "stale" || run.identity.projectRevision != project.projectRevision)
  val status =
      when {
        analysis.action.isNotBlank() -> analysisStatusLabel(analysis.action)
        analysis.admission != null -> "Review scope"
        stale -> "Stale"
        analysis.error != null -> "Error"
        else -> analysisStatusLabel(run?.status)
      }
  return ToolbarAnalysisStatus(
      label = "Analysis · $status",
      detail = "Whole-project analysis · $status" + analysis.error?.let { ". $it" }.orEmpty(),
      running = analysis.action.isNotBlank() || (!stale && run?.isActive() == true),
      attention =
          stale ||
              analysis.error != null ||
              run?.status in setOf("failed", "partial", "interrupted"))
}

@Composable
private fun ToolbarStatus(
    state: ToolbarState,
    connection: ConnectionPresentation,
) {
  if (state.busy) {
    IdeBusyIndicator(
        Modifier.size(14.dp).semantics { contentDescription = state.operationStatus },
        color = FocusAccent,
        strokeWidth = 2.dp)
    Spacer(Modifier.width(10.dp))
  }
  val analysis =
      state.analysisStatus
          ?: ToolbarAnalysisStatus(
              "Analysis · Unavailable",
              "Whole-project analysis · Unavailable; no current analysis evidence",
              running = false,
              attention = false)
  val color =
      if (analysis.attention) Warning else if (analysis.running) Information else SecondaryText
  Row(
      Modifier.semantics { contentDescription = analysis.detail }.padding(vertical = 6.dp),
      verticalAlignment = Alignment.CenterVertically) {
        if (analysis.running) {
          IdeBusyIndicator(Modifier.size(12.dp), color = color, strokeWidth = 2.dp)
        } else {
          Canvas(Modifier.size(6.dp)) { drawCircle(color) }
        }
        Spacer(Modifier.width(6.dp))
        Text(analysis.label, color = color, fontSize = 11.sp)
      }
  Spacer(Modifier.width(10.dp))
  IdeVerticalSeparator(Modifier.height(20.dp))
  Spacer(Modifier.width(10.dp))
  ConnectionChip(connection)
}

internal data class ToolbarActions(
    val onImport: () -> Unit,
    val onReindex: () -> Unit,
    val onReconnect: () -> Unit,
    val onPalette: () -> Unit,
    val onRetryRestore: () -> Unit = {},
)

@Composable
private fun ProjectActionsMenu(
    projectType: String?,
    projectLabel: String,
    projectAvailable: Boolean,
    availability: ProjectActionAvailability,
    reconnectAvailable: Boolean,
    onImport: () -> Unit,
    onReindex: () -> Unit,
    onReconnect: () -> Unit,
    modifier: Modifier = Modifier,
) {
  val typePresentation = projectTypePresentation(projectType)
  val triggerFocus = remember { FocusRequester() }
  var expanded by remember { mutableStateOf(false) }
  var restoreFocus by remember { mutableStateOf(false) }
  Box(modifier) {
    TopBarButton(
        projectLabel,
        { expanded = true },
        icon = typePresentation.icon,
        iconDescription = typePresentation.description,
        iconTint = typePresentation.tint,
        labelStyle = IdeTypography.toolbarIdentity,
        modifier = Modifier.fillMaxWidth().focusRequester(triggerFocus))
    IdeDropdownMenu(
        expanded = expanded,
        onDismissRequest = {
          expanded = false
          restoreFocus = true
        }) {
          IdeDropdownMenuItem(
              label = if (projectAvailable) "Switch project…" else "Open project…",
              onClick = {
                expanded = false
                restoreFocus = true
                onImport()
              },
              enabled = availability.open,
              icon = DesktopIcon.Folder)
          IdeDropdownMenuItem(
              label = "Re-index project",
              onClick = {
                expanded = false
                restoreFocus = true
                onReindex()
              },
              enabled = availability.reindex,
              icon = DesktopIcon.Refresh)
          if (reconnectAvailable) {
            Text(
                "Daemon disconnected. Reconnect reads daemon status and model configuration; it does not contact a provider or run project code.",
                color = Error,
                style = IdeTypography.compactBody,
                modifier = Modifier.widthIn(max = 280.dp).padding(8.dp))
            IdeDropdownMenuItem(
                label = "Reconnect",
                onClick = {
                  expanded = false
                  restoreFocus = true
                  onReconnect()
                },
                icon = DesktopIcon.Refresh)
          }
        }
  }
  LaunchedEffect(restoreFocus) {
    if (restoreFocus) {
      triggerFocus.requestFocus()
      restoreFocus = false
    }
  }
}

@Composable
private fun TopBarButton(
    label: String,
    onClick: () -> Unit,
    icon: DesktopIcon? = null,
    modifier: Modifier = Modifier,
    iconDescription: String = label,
    iconTint: Color = SecondaryText,
    labelStyle: TextStyle = IdeTypography.action,
) {
  ChromeButton(
      onClick = onClick,
      modifier = modifier,
      accessibleName = label,
  ) {
    icon?.let {
      DesktopLineIcon(it, iconDescription, iconSize = 18.dp, tint = iconTint)
      Spacer(Modifier.width(8.dp))
    }
    Text(
        label,
        style = labelStyle,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = if (icon != null) Modifier.weight(1f) else Modifier)
    if (icon != null) DesktopLineIcon(DesktopIcon.ChevronDown, "Project menu", iconSize = 12.dp)
  }
}

internal data class ProjectTypePresentation(
    val icon: DesktopIcon,
    val description: String,
    val tint: Color,
)

internal fun projectTypePresentation(type: String?): ProjectTypePresentation =
    when (type?.trim()?.lowercase()) {
      "go" -> ProjectTypePresentation(DesktopIcon.Go, "Go project", Information)
      "java" -> ProjectTypePresentation(DesktopIcon.Java, "Java project", Warning)
      "kotlin" -> ProjectTypePresentation(DesktopIcon.Kotlin, "Kotlin project", SelectionAccent)
      else ->
          ProjectTypePresentation(
              DesktopIcon.Project,
              type?.takeIf { it.isNotBlank() }?.let { "Project type: $it" } ?: "Project",
              SecondaryText)
    }

@Composable
internal fun MiniOrcaMark() {
  Canvas(Modifier.size(26.dp).semantics { contentDescription = "Mini-Orca" }) {
    val stroke = 1.8.dp.toPx()
    val center = Offset(size.width * 0.5f, size.height * 0.5f)
    drawRoundRect(
        ToolWindowSurface, cornerRadius = androidx.compose.ui.geometry.CornerRadius(8.dp.toPx()))
    drawRoundRect(
        SelectionAccent,
        topLeft = Offset(size.width * 0.08f, size.height * 0.08f),
        size = androidx.compose.ui.geometry.Size(size.width * 0.84f, size.height * 0.84f),
        cornerRadius = androidx.compose.ui.geometry.CornerRadius(7.dp.toPx()),
        style = androidx.compose.ui.graphics.drawscope.Stroke(stroke))
    drawArc(
        MiniOrcaPalette.identityAccent,
        startAngle = 205f,
        sweepAngle = 235f,
        useCenter = false,
        topLeft = Offset(size.width * 0.24f, size.height * 0.24f),
        size = androidx.compose.ui.geometry.Size(size.width * 0.52f, size.height * 0.52f),
        style = androidx.compose.ui.graphics.drawscope.Stroke(stroke, cap = StrokeCap.Round))
    drawCircle(Information, size.minDimension * 0.08f, center)
  }
}

fun projectBreadcrumbLabel(project: ProjectAnalysis?): String = project?.name ?: "No project open"

internal data class BranchPresentation(val label: String, val detail: String)

internal fun branchPresentation(gitStatus: GitStatus?): BranchPresentation =
    gitStatus
        ?.takeIf { it.available && it.branch.isNotBlank() }
        ?.let { BranchPresentation(it.branch.trim(), "Current Git branch: ${it.branch.trim()}") }
        ?: BranchPresentation("Unavailable", "Git branch is unavailable for the selected file.")

@Composable
private fun BranchContext(gitStatus: GitStatus?) {
  val presentation = branchPresentation(gitStatus)
  Row(
      modifier =
          Modifier.semantics { contentDescription = presentation.detail }
              .padding(horizontal = 8.dp, vertical = 6.dp),
      verticalAlignment = Alignment.CenterVertically) {
        DesktopLineIcon(DesktopIcon.Branch, presentation.detail, tint = SecondaryText)
        Spacer(Modifier.width(4.dp))
        Text(
            presentation.label,
            color = SecondaryText,
            fontSize = 12.sp,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.widthIn(max = 112.dp))
      }
}

internal data class ConnectionPresentation(
    val label: String,
    val color: androidx.compose.ui.graphics.Color,
    val canReconnect: Boolean
)

internal fun connectionPresentation(connection: ConnectionState): ConnectionPresentation =
    when {
      connection.connected -> ConnectionPresentation("Daemon connected", Success, false)
      connection.label.equals("Connecting", ignoreCase = true) ->
          ConnectionPresentation("Daemon connecting", Warning, false)
      else -> ConnectionPresentation("Daemon disconnected", Error, true)
    }

@Composable
private fun ConnectionChip(presentation: ConnectionPresentation) {
  Row(
      modifier =
          Modifier.semantics {
                contentDescription =
                    "${presentation.label}; this is daemon status, not model connectivity"
              }
              .padding(vertical = 6.dp),
      verticalAlignment = Alignment.CenterVertically,
  ) {
    Canvas(Modifier.size(6.dp)) { drawCircle(presentation.color) }
    Spacer(Modifier.width(7.dp))
    Text(presentation.label, color = presentation.color, fontSize = 11.sp, maxLines = 1)
  }
}
