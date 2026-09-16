package io.miniorca.desktop

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
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
  val presentation = toolbarPresentation(state.widthDp)
  val separateStatusRow =
      state.analysisStatus != null &&
          state.widthDp / LocalDensity.current.fontScale < COMPACT_TOOLBAR_WIDTH
  Column(modifier.fillMaxWidth().background(ActivityRail)) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
      MiniOrcaMark()
      Spacer(Modifier.width(12.dp))
      ProjectActionsMenu(
          projectType = state.project?.type,
          projectLabel = projectBreadcrumbLabel(state.project),
          projectAvailable = state.project != null,
          reconnectAvailable = connectionPresentation.canReconnect,
          onImport = actions.onImport,
          onReanalyze = actions.onReanalyze,
          onReconnect = actions.onReconnect,
          modifier = Modifier.width(if (presentation.showBranchContext) 180.dp else 128.dp))
      if (presentation.showBranchContext) {
        Spacer(Modifier.width(8.dp))
        BranchContext(state.gitStatus)
      }
      Box(Modifier.weight(1f).padding(start = 16.dp), contentAlignment = Alignment.CenterStart) {
        ChromeButton(
            onClick = actions.onPalette,
            background = ToolWindowSurface,
            modifier =
                Modifier.widthIn(max = 420.dp)
                    .fillMaxWidth()
                    .then(paletteFocusRequester?.let { Modifier.focusRequester(it) } ?: Modifier)
                    .semantics { contentDescription = "Search files, symbols, commands" }) {
              DesktopLineIcon(DesktopIcon.Search, "Search", iconSize = 16.dp)
              Spacer(Modifier.width(8.dp))
              Text(
                  if (presentation.showSearchLabel) "Search files, symbols, commands" else "Search",
                  fontSize = 12.sp,
                  maxLines = 1,
                  overflow = TextOverflow.Ellipsis,
                  modifier = Modifier.weight(1f))
              Text(
                  "⌘P",
                  color = FaintText,
                  fontSize = 11.sp,
                  modifier = Modifier.padding(start = 8.dp))
            }
      }
      if (state.showEditorDrawerActions) {
        TopBarButton("Files", actions.onOpenExplorer)
        Spacer(Modifier.width(6.dp))
        TopBarButton("Context", actions.onOpenContext)
        Spacer(Modifier.width(6.dp))
      }
      Spacer(Modifier.width(16.dp))
      if (!separateStatusRow) ToolbarStatus(state, connectionPresentation, presentation)
    }
    if (separateStatusRow) {
      Row(
          Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, bottom = 8.dp),
          horizontalArrangement = Arrangement.End,
          verticalAlignment = Alignment.CenterVertically) {
            ToolbarStatus(state, connectionPresentation, presentation)
          }
    }
  }
}

internal data class ToolbarState(
    val widthDp: Float,
    val project: ProjectAnalysis?,
    val busy: Boolean,
    val operationStatus: String,
    val connection: ConnectionState,
    val gitStatus: GitStatus?,
    val showEditorDrawerActions: Boolean,
    val analysisStatus: ToolbarAnalysisStatus? = null,
)

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
    presentation: ToolbarPresentation,
) {
  if (state.busy) {
    IdeBusyIndicator(
        Modifier.size(14.dp).semantics { contentDescription = state.operationStatus },
        color = FocusAccent,
        strokeWidth = 2.dp)
    Spacer(Modifier.width(10.dp))
  }
  state.analysisStatus?.let { analysis ->
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
  }
  ConnectionChip(connection, compact = !presentation.showFullConnectionLabel)
}

internal data class ToolbarActions(
    val onImport: () -> Unit,
    val onReanalyze: () -> Unit,
    val onReconnect: () -> Unit,
    val onPalette: () -> Unit,
    val onOpenExplorer: () -> Unit,
    val onOpenContext: () -> Unit,
)

internal data class ToolbarPresentation(
    val showFullConnectionLabel: Boolean,
    val showSearchLabel: Boolean,
    val showBranchContext: Boolean,
)

internal fun toolbarPresentation(widthDp: Float): ToolbarPresentation =
    ToolbarPresentation(
        showFullConnectionLabel = widthDp >= COMPACT_TOOLBAR_WIDTH,
        showSearchLabel = widthDp >= EXPANDED_TOOLBAR_WIDTH,
        showBranchContext = widthDp >= COMPACT_TOOLBAR_WIDTH,
    )

@Composable
private fun ProjectActionsMenu(
    projectType: String?,
    projectLabel: String,
    projectAvailable: Boolean,
    reconnectAvailable: Boolean,
    onImport: () -> Unit,
    onReanalyze: () -> Unit,
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
              label = "Open project",
              onClick = {
                expanded = false
                restoreFocus = true
                onImport()
              },
              icon = DesktopIcon.Folder)
          IdeDropdownMenuItem(
              label = "Re-index project",
              onClick = {
                expanded = false
                restoreFocus = true
                onReanalyze()
              },
              enabled = projectAvailable,
              icon = DesktopIcon.Refresh)
          if (reconnectAvailable)
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
    drawRoundRect(
        SelectionAccent, cornerRadius = androidx.compose.ui.geometry.CornerRadius(8.dp.toPx()))
    drawCircle(
        FocusAccent,
        radius = size.minDimension * 0.18f,
        center = Offset(size.width * 0.36f, size.height * 0.42f))
    drawLine(
        PrimaryText,
        Offset(size.width * 0.26f, size.height * 0.68f),
        Offset(size.width * 0.72f, size.height * 0.68f),
        strokeWidth = 2.dp.toPx(),
        cap = StrokeCap.Round)
  }
}

fun projectBreadcrumbLabel(project: ProjectAnalysis?): String = project?.name ?: "No project open"

internal data class BranchPresentation(val label: String, val detail: String)

internal fun branchPresentation(gitStatus: GitStatus?): BranchPresentation =
    gitStatus
        ?.takeIf { it.available && it.branch.isNotBlank() }
        ?.let { BranchPresentation(it.branch, "Current Git branch: ${it.branch}") }
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
private fun ConnectionChip(presentation: ConnectionPresentation, compact: Boolean) {
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
    Text(
        if (compact) presentation.label.removePrefix("Daemon ").replaceFirstChar { it.uppercase() }
        else presentation.label,
        color = presentation.color,
        fontSize = 11.sp,
        maxLines = 1)
  }
}

private const val COMPACT_TOOLBAR_WIDTH = 1_000f
private const val EXPANDED_TOOLBAR_WIDTH = 1_220f
