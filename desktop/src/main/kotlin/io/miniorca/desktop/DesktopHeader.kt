package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material.CircularProgressIndicator
import androidx.compose.material.DropdownMenu
import androidx.compose.material.DropdownMenuItem
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
  Row(
      modifier =
          modifier
              .fillMaxWidth()
              .height(52.dp)
              .background(Chrome)
              .border(BorderStroke(1.dp, Border))
              .padding(horizontal = 12.dp),
      verticalAlignment = Alignment.CenterVertically,
  ) {
    MiniOrcaMark()
    if (presentation.showProductName) {
      Spacer(Modifier.width(10.dp))
      Text("Mini-Orca", color = PrimaryText, fontWeight = FontWeight.SemiBold)
    }
    Spacer(Modifier.width(8.dp))
    ProjectActionsMenu(
        projectLabel = projectBreadcrumbLabel(state.project),
        projectAvailable = state.project != null,
        reconnectAvailable = connectionPresentation.canReconnect,
        onImport = actions.onImport,
        onReanalyze = actions.onReanalyze,
        onReconnect = actions.onReconnect,
        modifier = Modifier.weight(1f))
    if (presentation.showBranchContext) {
      Spacer(Modifier.width(8.dp))
      BranchContext(state.gitStatus)
    }
    if (state.busy) {
      Spacer(Modifier.width(8.dp))
      CircularProgressIndicator(Modifier.size(16.dp), color = FocusAccent, strokeWidth = 2.dp)
      Text(
          state.operationStatus,
          color = SecondaryText,
          fontSize = 11.sp,
          maxLines = 1,
          overflow = TextOverflow.Ellipsis,
          modifier = Modifier.padding(start = 6.dp))
    }
    Spacer(Modifier.width(8.dp))
    ConnectionChip(connectionPresentation)
    Spacer(Modifier.width(8.dp))
    if (state.showEditorDrawerActions) {
      TopBarButton("Files", actions.onOpenExplorer, tone = ActionTone.Navigation)
      Spacer(Modifier.width(6.dp))
      TopBarButton("Context", actions.onOpenContext, tone = ActionTone.Navigation)
      Spacer(Modifier.width(6.dp))
    }
    TopBarButton(
        if (presentation.showSearchLabel) "Search files, symbols, commands" else "Search",
        actions.onPalette,
        tone = ActionTone.Navigation,
        icon = DesktopIcon.Search)
    if (presentation.showSearchLabel) {
      Spacer(Modifier.width(6.dp))
      PreviewFeatureButton(
          PreviewFeature("New file", "Filesystem mutation is not available from this control."))
      Spacer(Modifier.width(6.dp))
      PreviewFeatureButton(
          PreviewFeature("Branch actions", "Branch switching and sync are not available."))
      Spacer(Modifier.width(6.dp))
      PreviewFeatureButton(
          PreviewFeature("Content search", "Global file-content search is not available."))
      Spacer(Modifier.width(6.dp))
      PreviewFeatureButton(
          PreviewFeature(
              "Settings & Help", "Account, notifications, settings, and help are not available."))
    }
    Spacer(Modifier.width(6.dp))
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
  var expanded by remember { mutableStateOf(false) }
  Box(modifier) {
    TopBarButton(
        projectLabel,
        { expanded = true },
        tone = ActionTone.Neutral,
        icon = DesktopIcon.Project,
        modifier = Modifier.fillMaxWidth())
    DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
      DropdownMenuItem(
          onClick = {
            expanded = false
            onImport()
          }) {
            Text("Open project")
          }
      DropdownMenuItem(
          onClick = {
            expanded = false
            onReanalyze()
          },
          enabled = projectAvailable) {
            Text("Re-index project")
          }
      if (reconnectAvailable)
          DropdownMenuItem(
              onClick = {
                expanded = false
                onReconnect()
              }) {
                Text("Reconnect")
              }
    }
  }
}

@Composable
private fun TopBarButton(
    label: String,
    onClick: () -> Unit,
    enabled: Boolean = true,
    tone: ActionTone = ActionTone.Neutral,
    icon: DesktopIcon? = null,
    modifier: Modifier = Modifier,
) {
  MiniOrcaButton(
      onClick = onClick,
      enabled = enabled,
      tone = tone,
      density = ButtonDensity.Toolbar,
      modifier = modifier,
  ) {
    icon?.let {
      DesktopLineIcon(
          it, label, tint = if (tone == ActionTone.Primary) OnActionFill else SelectionAccent)
      Spacer(Modifier.width(4.dp))
    }
    Text(label, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
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
              .border(BorderStroke(1.dp, Border), MiniOrcaShapes.small)
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
            modifier = Modifier.width(112.dp))
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
  Text(
      presentation.label,
      color = presentation.color,
      fontSize = 12.sp,
      maxLines = 1,
      overflow = TextOverflow.Ellipsis,
      modifier =
          Modifier.border(
                  BorderStroke(1.dp, presentation.color.copy(alpha = 0.55f)), MiniOrcaShapes.small)
              .padding(horizontal = 8.dp, vertical = 6.dp)
              .semantics {
                contentDescription =
                    "${presentation.label}; this is daemon status, not model connectivity"
              },
  )
}

private const val COMPACT_TOOLBAR_WIDTH = 1_000f
private const val EXPANDED_TOOLBAR_WIDTH = 1_220f
