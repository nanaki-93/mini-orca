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
              .height(58.dp)
              .background(Panel)
              .border(BorderStroke(1.dp, Border))
              .padding(horizontal = 16.dp),
      verticalAlignment = Alignment.CenterVertically,
  ) {
    MiniOrcaMark()
    if (presentation.showProductName) {
      Spacer(Modifier.width(10.dp))
      Text("Mini-Orca", color = PrimaryText, fontWeight = FontWeight.SemiBold)
    }
    Spacer(Modifier.width(10.dp))
    Text(
        projectBreadcrumbLabel(state.project),
        color = SecondaryText,
        fontSize = 12.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = Modifier.weight(1f))
    if (state.busy) {
      Spacer(Modifier.width(8.dp))
      CircularProgressIndicator(Modifier.size(16.dp), color = CyanAccent, strokeWidth = 2.dp)
      Text(
          state.operationStatus,
          color = SecondaryText,
          fontSize = 11.sp,
          maxLines = 1,
          overflow = TextOverflow.Ellipsis,
          modifier = Modifier.padding(start = 6.dp))
    }
    Spacer(Modifier.width(8.dp))
    Text(
        connectionPresentation.label,
        color = connectionPresentation.color,
        fontSize = 11.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis)
    Spacer(Modifier.width(8.dp))
    if (state.showEditorDrawerActions) {
      TopBarButton("Files", actions.onOpenExplorer, tone = ActionTone.Navigation)
      Spacer(Modifier.width(6.dp))
      TopBarButton("Context", actions.onOpenContext, tone = ActionTone.Navigation)
      Spacer(Modifier.width(6.dp))
    }
    TopBarButton("Command", actions.onPalette, tone = ActionTone.Navigation)
    Spacer(Modifier.width(6.dp))
    if (presentation.showProjectActionsInline) {
      TopBarButton(
          "Re-index", actions.onReanalyze, state.project != null, tone = ActionTone.Primary)
      Spacer(Modifier.width(6.dp))
      if (connectionPresentation.canReconnect) {
        TopBarButton("Reconnect", actions.onReconnect, tone = ActionTone.Attention)
        Spacer(Modifier.width(6.dp))
      }
      TopBarButton("Open project", actions.onImport, tone = ActionTone.Primary)
    } else {
      ProjectActionsMenu(
          projectAvailable = state.project != null,
          reconnectAvailable = connectionPresentation.canReconnect,
          onImport = actions.onImport,
          onReanalyze = actions.onReanalyze,
          onReconnect = actions.onReconnect,
      )
    }
  }
}

internal data class ToolbarState(
    val widthDp: Float,
    val project: ProjectAnalysis?,
    val busy: Boolean,
    val operationStatus: String,
    val connection: ConnectionState,
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
    val showProjectActionsInline: Boolean,
)

internal fun toolbarPresentation(widthDp: Float): ToolbarPresentation =
    ToolbarPresentation(
        showProductName = widthDp >= COMPACT_TOOLBAR_WIDTH,
        showProjectActionsInline = widthDp >= EXPANDED_TOOLBAR_WIDTH,
    )

@Composable
private fun ProjectActionsMenu(
    projectAvailable: Boolean,
    reconnectAvailable: Boolean,
    onImport: () -> Unit,
    onReanalyze: () -> Unit,
    onReconnect: () -> Unit,
) {
  var expanded by remember { mutableStateOf(false) }
  Box {
    TopBarButton("Project", { expanded = true }, tone = ActionTone.Primary)
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
    tone: ActionTone = ActionTone.Neutral
) {
  FocusFlowButton(
      onClick = onClick,
      enabled = enabled,
      tone = tone,
      density = ButtonDensity.Toolbar,
  ) {
    Text(label, fontSize = 11.sp, maxLines = 1)
  }
}

@Composable
internal fun MiniOrcaMark() {
  Canvas(Modifier.size(26.dp).semantics { contentDescription = "Mini-Orca" }) {
    drawRoundRect(Accent, cornerRadius = androidx.compose.ui.geometry.CornerRadius(8.dp.toPx()))
    drawCircle(
        CyanAccent,
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

internal data class ConnectionPresentation(
    val label: String,
    val color: androidx.compose.ui.graphics.Color,
    val canReconnect: Boolean
)

internal fun connectionPresentation(connection: ConnectionState): ConnectionPresentation =
    when {
      connection.connected -> ConnectionPresentation("Connected", Success, false)
      connection.label.equals("Connecting", ignoreCase = true) ->
          ConnectionPresentation("Connecting", Warning, false)
      else -> ConnectionPresentation("Disconnected", Error, true)
    }

private const val COMPACT_TOOLBAR_WIDTH = 1_000f
private const val EXPANDED_TOOLBAR_WIDTH = 1_220f
