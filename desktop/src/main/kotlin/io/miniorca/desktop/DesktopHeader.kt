package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun AppTopBar(
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
        modifier = Modifier.fillMaxWidth().height(58.dp).background(Panel).border(BorderStroke(1.dp, Border)).padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MiniOrcaMark()
        Spacer(Modifier.width(10.dp))
        Text("Mini-Orca", color = PrimaryText, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.width(12.dp))
        Text(projectBreadcrumbLabel(project), color = SecondaryText, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
        Spacer(Modifier.width(12.dp))
        Text(connectionPresentation.label, color = connectionPresentation.color, fontSize = 11.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
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
private fun TopBarButton(label: String, onClick: () -> Unit, enabled: Boolean = true, tone: ActionTone = ActionTone.Neutral) {
    FocusFlowButton(
        onClick = onClick,
        enabled = enabled,
        tone = tone,
        density = ButtonDensity.Toolbar,
    ) { Text(label, fontSize = 11.sp, maxLines = 1) }
}

@Composable
internal fun WorkspaceRail(selected: Workspace, onSelect: (Workspace) -> Unit, modifier: Modifier = Modifier) {
    Column(modifier.background(Panel).border(BorderStroke(1.dp, Border)).padding(horizontal = 8.dp, vertical = 12.dp)) {
        SectionLabel("WORKSPACES", Modifier.padding(horizontal = 8.dp, vertical = 4.dp))
        Workspace.entries.forEach { workspace ->
            val isSelected = workspace == selected
            FocusFlowButton(
                onClick = { onSelect(workspace) },
                modifier = Modifier.fillMaxWidth().padding(top = 6.dp).semantics {
                    contentDescription = workspaceSemanticsLabel(workspace, isSelected)
                    this.selected = isSelected
                },
                tone = ActionTone.Navigation,
                selected = isSelected,
            ) {
                WorkspaceGlyph(workspace, isSelected)
                Spacer(Modifier.width(8.dp))
                Text(workspaceRailLabel(workspace), fontSize = 11.sp, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
            }
        }
    }
}

@Composable
internal fun MiniOrcaMark() {
    Canvas(Modifier.size(26.dp).semantics { contentDescription = "Mini-Orca" }) {
        drawRoundRect(Accent, cornerRadius = androidx.compose.ui.geometry.CornerRadius(8.dp.toPx()))
        drawCircle(CyanAccent, radius = size.minDimension * 0.18f, center = Offset(size.width * 0.36f, size.height * 0.42f))
        drawLine(PrimaryText, Offset(size.width * 0.26f, size.height * 0.68f), Offset(size.width * 0.72f, size.height * 0.68f), strokeWidth = 2.dp.toPx(), cap = StrokeCap.Round)
    }
}

@Composable
private fun WorkspaceGlyph(workspace: Workspace, selected: Boolean) {
    val color = if (selected) CyanAccent else FaintText
    Canvas(Modifier.size(15.dp)) {
        val stroke = Stroke(width = 1.5.dp.toPx(), cap = StrokeCap.Round)
        when (workspace) {
            Workspace.Summary -> {
                drawRect(color, topLeft = Offset(size.width * 0.12f, size.height * 0.16f), size = Size(size.width * 0.76f, size.height * 0.68f), style = stroke)
                drawLine(color, Offset(size.width * 0.30f, size.height * 0.38f), Offset(size.width * 0.72f, size.height * 0.38f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                drawLine(color, Offset(size.width * 0.30f, size.height * 0.62f), Offset(size.width * 0.60f, size.height * 0.62f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
            }
            Workspace.Analysis -> {
                drawLine(color, Offset(size.width * 0.15f, size.height * 0.75f), Offset(size.width * 0.40f, size.height * 0.48f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                drawLine(color, Offset(size.width * 0.40f, size.height * 0.48f), Offset(size.width * 0.62f, size.height * 0.62f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                drawLine(color, Offset(size.width * 0.62f, size.height * 0.62f), Offset(size.width * 0.86f, size.height * 0.22f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                listOf(Offset(size.width * 0.15f, size.height * 0.75f), Offset(size.width * 0.40f, size.height * 0.48f), Offset(size.width * 0.62f, size.height * 0.62f), Offset(size.width * 0.86f, size.height * 0.22f)).forEach { drawCircle(color, radius = 1.5.dp.toPx(), center = it) }
            }
            Workspace.Bugs -> {
                drawCircle(color, radius = size.minDimension * 0.36f, center = center, style = stroke)
                drawLine(color, Offset(center.x, size.height * 0.34f), Offset(center.x, size.height * 0.58f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                drawCircle(color, radius = 1.dp.toPx(), center = Offset(center.x, size.height * 0.71f))
            }
            Workspace.Editor -> {
                drawLine(color, Offset(size.width * 0.18f, size.height * 0.24f), Offset(size.width * 0.38f, size.height * 0.50f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                drawLine(color, Offset(size.width * 0.38f, size.height * 0.50f), Offset(size.width * 0.18f, size.height * 0.76f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                drawLine(color, Offset(size.width * 0.82f, size.height * 0.24f), Offset(size.width * 0.62f, size.height * 0.50f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
                drawLine(color, Offset(size.width * 0.62f, size.height * 0.50f), Offset(size.width * 0.82f, size.height * 0.76f), strokeWidth = 1.5.dp.toPx(), cap = StrokeCap.Round)
            }
        }
    }
}

fun projectBreadcrumbLabel(project: ProjectAnalysis?): String = project?.name ?: "No project open"

internal data class ConnectionPresentation(val label: String, val color: androidx.compose.ui.graphics.Color, val canReconnect: Boolean)

internal fun connectionPresentation(connection: ConnectionState): ConnectionPresentation = when {
    connection.connected -> ConnectionPresentation("Connected", Success, false)
    connection.label.equals("Connecting", ignoreCase = true) -> ConnectionPresentation("Connecting", Warning, false)
    else -> ConnectionPresentation("Disconnected", Error, true)
}

fun workspaceRailLabel(workspace: Workspace): String = when (workspace) {
    Workspace.Summary -> "Summary"
    Workspace.Analysis -> "Analysis"
    Workspace.Bugs -> "Bugs"
    Workspace.Editor -> "Editor"
}

fun workspaceSemanticsLabel(workspace: Workspace, selected: Boolean): String =
    "${workspaceRailLabel(workspace)}${if (selected) ", selected" else ", not selected"}"
