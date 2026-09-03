package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material.CircularProgressIndicator
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
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
    project: ProjectAnalysis?,
    busy: Boolean,
    connection: ConnectionState,
    onImport: () -> Unit,
    onReanalyze: () -> Unit,
    onReconnect: () -> Unit,
    onPalette: () -> Unit,
    showEditorDrawerActions: Boolean,
    onOpenExplorer: () -> Unit,
    onOpenContext: () -> Unit,
) {
  val connectionPresentation = connectionPresentation(connection)
  Row(
      modifier =
          Modifier.fillMaxWidth()
              .height(58.dp)
              .background(Panel)
              .border(BorderStroke(1.dp, Border))
              .padding(horizontal = 16.dp),
      verticalAlignment = Alignment.CenterVertically,
  ) {
    MiniOrcaMark()
    Spacer(Modifier.width(10.dp))
    Text("Mini-Orca", color = PrimaryText, fontWeight = FontWeight.SemiBold)
    Spacer(Modifier.width(12.dp))
    Text(
        projectBreadcrumbLabel(project),
        color = SecondaryText,
        fontSize = 12.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = Modifier.weight(1f))
    Spacer(Modifier.width(12.dp))
    Text(
        connectionPresentation.label,
        color = connectionPresentation.color,
        fontSize = 11.sp,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis)
    if (busy) {
      Spacer(Modifier.width(8.dp))
      CircularProgressIndicator(Modifier.size(16.dp), color = CyanAccent, strokeWidth = 2.dp)
    }
    Spacer(Modifier.width(12.dp))
    if (showEditorDrawerActions) {
      TopBarButton("Files", onOpenExplorer, tone = ActionTone.Navigation)
      Spacer(Modifier.width(6.dp))
      TopBarButton("Context", onOpenContext, tone = ActionTone.Navigation)
      Spacer(Modifier.width(6.dp))
    }
    TopBarButton("Command", onPalette, tone = ActionTone.Navigation)
    Spacer(Modifier.width(6.dp))
    TopBarButton("Re-index", onReanalyze, project != null, tone = ActionTone.Primary)
    Spacer(Modifier.width(6.dp))
    if (connectionPresentation.canReconnect) {
      TopBarButton("Reconnect", onReconnect, tone = ActionTone.Attention)
      Spacer(Modifier.width(6.dp))
    }
    TopBarButton("Open project", onImport, tone = ActionTone.Primary)
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
