package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun MainToolbar(
    state: ToolbarState,
    actions: ToolbarActions,
    modifier: Modifier = Modifier,
) {
  val connectionPresentation = connectionPresentation(state.connection)
  val presentation = toolbarPresentation(state.widthDp)
  Column(modifier.fillMaxWidth().background(ActivityRail)) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 52.dp).padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
      MiniOrcaMark()
      if (presentation.showProductName) {
        Spacer(Modifier.width(10.dp))
        Text("Mini-Orca", color = PrimaryText, fontWeight = FontWeight.SemiBold)
      }
      Spacer(Modifier.width(20.dp))
      IdeVerticalSeparator(Modifier.height(22.dp))
      Spacer(Modifier.width(16.dp))
      ProjectActionsMenu(
          projectLabel = projectBreadcrumbLabel(state.project),
          projectAvailable = state.project != null,
          reconnectAvailable = connectionPresentation.canReconnect,
          onImport = actions.onImport,
          onReanalyze = actions.onReanalyze,
          onReconnect = actions.onReconnect,
          modifier = Modifier.width(if (presentation.showProductName) 164.dp else 128.dp))
      if (presentation.showBranchContext) {
        Spacer(Modifier.width(8.dp))
        BranchContext(state.gitStatus)
      }
      Box(Modifier.weight(1f).padding(horizontal = 20.dp), contentAlignment = Alignment.Center) {
        ChromeButton(
            onClick = actions.onPalette,
            background = ToolWindowSurface,
            modifier =
                Modifier.widthIn(max = 340.dp).fillMaxWidth().semantics {
                  contentDescription = "Search files, symbols, commands"
                }) {
              DesktopLineIcon(DesktopIcon.Search, "Search", iconSize = 16.dp)
              Spacer(Modifier.width(8.dp))
              Text(
                  if (presentation.showSearchLabel) "Search files, symbols, commands" else "Search",
                  fontSize = 12.sp,
                  maxLines = 1,
                  overflow = TextOverflow.Ellipsis,
                  modifier = Modifier.weight(1f))
            }
      }
      if (state.showEditorDrawerActions) {
        TopBarButton("Files", actions.onOpenExplorer)
        Spacer(Modifier.width(6.dp))
        TopBarButton("Context", actions.onOpenContext)
        Spacer(Modifier.width(6.dp))
      }
      if (state.busy) {
        IdeBusyIndicator(
            Modifier.size(14.dp).semantics { contentDescription = state.operationStatus },
            color = FocusAccent,
            strokeWidth = 2.dp)
        Spacer(Modifier.width(10.dp))
      }
      ConnectionChip(connectionPresentation, compact = !presentation.showProductName)
      Spacer(Modifier.width(12.dp))
      PreviewFeatureMenu(toolbarPreviewFeatures)
    }
    IdeHorizontalSeparator()
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
)

internal data class ToolbarActions(
    val onImport: () -> Unit,
    val onReanalyze: () -> Unit,
    val onReconnect: () -> Unit,
    val onPalette: () -> Unit,
    val onOpenExplorer: () -> Unit,
    val onOpenContext: () -> Unit,
)

internal data class ToolbarPresentation(
    val showProductName: Boolean,
    val showSearchLabel: Boolean,
    val showBranchContext: Boolean,
)

internal fun toolbarPresentation(widthDp: Float): ToolbarPresentation =
    ToolbarPresentation(
        showProductName = widthDp >= COMPACT_TOOLBAR_WIDTH,
        showSearchLabel = widthDp >= EXPANDED_TOOLBAR_WIDTH,
        showBranchContext = widthDp >= COMPACT_TOOLBAR_WIDTH,
    )

@Composable
private fun ProjectActionsMenu(
    projectLabel: String,
    projectAvailable: Boolean,
    reconnectAvailable: Boolean,
    onImport: () -> Unit,
    onReanalyze: () -> Unit,
    onReconnect: () -> Unit,
    modifier: Modifier = Modifier,
) {
  val triggerFocus = remember { FocusRequester() }
  var expanded by remember { mutableStateOf(false) }
  var restoreFocus by remember { mutableStateOf(false) }
  Box(modifier) {
    TopBarButton(
        projectLabel,
        { expanded = true },
        icon = DesktopIcon.Project,
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
) {
  ChromeButton(
      onClick = onClick,
      modifier = modifier,
  ) {
    icon?.let {
      DesktopLineIcon(it, label, iconSize = 18.dp)
      Spacer(Modifier.width(8.dp))
    }
    Text(
        label,
        fontSize = 12.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = if (icon != null) Modifier.weight(1f) else Modifier)
    if (icon != null) DesktopLineIcon(DesktopIcon.ChevronDown, "Project menu", iconSize = 12.dp)
  }
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
          Modifier.background(presentation.color.copy(alpha = 0.07f), RoundedCornerShape(20.dp))
              .border(
                  BorderStroke(1.dp, presentation.color.copy(alpha = 0.20f)),
                  RoundedCornerShape(20.dp))
              .semantics {
                contentDescription =
                    "${presentation.label}; this is daemon status, not model connectivity"
              }
              .padding(horizontal = 10.dp, vertical = 6.dp),
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

private val toolbarPreviewFeatures =
    listOf(
        PreviewFeature("New file", "Filesystem mutation is not available from this control."),
        PreviewFeature("Branch actions", "Branch switching and sync are not available."),
        PreviewFeature("Content search", "Global file-content search is not available."),
        PreviewFeature(
            "Settings & Help", "Account, notifications, settings, and help are not available."),
    )

private const val COMPACT_TOOLBAR_WIDTH = 1_000f
private const val EXPANDED_TOOLBAR_WIDTH = 1_220f
