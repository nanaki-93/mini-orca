package io.miniorca.desktop

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun ExplorerPane(state: ExplorerPaneState, actions: ExplorerPaneActions, modifier: Modifier) {
    val rows = visibleExplorerRows(state.index?.files.orEmpty(), state.filter, state.collapsedDirectories)
    FocusFlowPanel(modifier = modifier, contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp)) {
        SectionLabel("PROJECT EXPLORER")
        Spacer(Modifier.height(8.dp))
        CompactSingleLineField(
            value = state.filter,
            onValueChange = actions.updateFilter,
            label = { Text("Filter indexed files") },
            placeholder = { Text("Type a relative path", color = SecondaryText, fontSize = 12.sp) },
            modifier = Modifier.fillMaxWidth().semantics { contentDescription = "Filter indexed relative file paths" },
        )
        Spacer(Modifier.height(8.dp))
        when {
            state.index == null && state.loading -> LoadingRows("Loading indexed files")
            state.index == null -> SystemStateMessage("No project open", "Open a project to browse safe, indexed relative paths.")
            rows.isEmpty() -> SystemStateMessage("No matching files", "Change the filter to view indexed relative paths.")
            else -> LazyColumn {
                items(rows, key = { it.path }) { row ->
                    ExplorerItem(
                        row = row,
                        selected = !row.directory && row.path == state.selectedPath,
                        expanded = row.path !in state.collapsedDirectories,
                        onActivate = { if (row.directory) actions.toggleDirectory(row.path) else actions.selectFile(row.path) },
                    )
                }
            }
        }
        Spacer(Modifier.height(8.dp))
    }
}

/** Immutable explorer inputs; only indexed project-relative paths are rendered. */
internal data class ExplorerPaneState(
    val index: ProjectIndex?,
    val selectedPath: String?,
    val filter: String,
    val collapsedDirectories: Set<String>,
    val loading: Boolean,
)

/** Explorer-only intents, kept separate from project and editor workflow actions. */
internal data class ExplorerPaneActions(
    val updateFilter: (String) -> Unit,
    val toggleDirectory: (String) -> Unit,
    val selectFile: (String) -> Unit,
)

@Composable
private fun ExplorerItem(row: ExplorerRow, selected: Boolean, expanded: Boolean, onActivate: () -> Unit) {
    val description = explorerRowDescription(row, selected, expanded)
    val nodeColor = explorerNodeColor(row, selected)
    Row(
        modifier = Modifier.fillMaxWidth()
            .semantics {
                contentDescription = description
                this.selected = selected
                role = Role.Button
            }
            .background(if (selected) StrongSurface else Color.Transparent, RoundedCornerShape(8.dp))
            .clickable(onClick = onActivate)
            .padding(start = (8 + row.depth * 14).dp, end = 8.dp, top = 7.dp, bottom = 7.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        ExplorerNodeIcon(row, expanded, nodeColor)
        Spacer(Modifier.width(6.dp))
        Text(explorerRoleLabel(row, expanded), color = nodeColor, fontSize = 9.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.width(7.dp))
        Text(
            row.name,
            color = if (selected || row.directory) PrimaryText else SecondaryText,
            fontFamily = FontFamily.Monospace,
            fontSize = 12.sp,
            fontWeight = if (row.directory) FontWeight.SemiBold else FontWeight.Normal,
            modifier = Modifier.weight(1f),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (!row.directory) {
            Text(row.language.ifBlank { "Text" }.uppercase(), color = FaintText, fontSize = 9.sp, maxLines = 1)
            Spacer(Modifier.width(6.dp))
            if (row.analysisStatus.equals("fresh", ignoreCase = true)) FreshnessDot() else StatusBadge(row.analysisStatus)
        }
    }
}

@Composable
private fun FreshnessDot() {
    Box(
        Modifier
            .size(7.dp)
            .background(Success, CircleShape)
            .semantics { contentDescription = "Fresh analysis" },
    )
}

private fun explorerNodeColor(row: ExplorerRow, selected: Boolean): Color = when {
    row.directory -> Warning
    selected -> CyanAccent
    else -> SecondaryText
}

@Composable
private fun ExplorerNodeIcon(row: ExplorerRow, expanded: Boolean, color: Color) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        if (row.directory) {
            ExplorerDisclosureIcon(expanded)
        } else {
            Spacer(Modifier.width(12.dp))
        }
        Spacer(Modifier.width(4.dp))
        if (row.directory) ExplorerFolderIcon(color) else ExplorerFileIcon(color)
    }
}

@Composable
private fun ExplorerDisclosureIcon(expanded: Boolean) {
    Canvas(Modifier.size(12.dp)) {
        val strokeWidth = 1.5.dp.toPx()
        val path = Path().apply {
            if (expanded) {
                moveTo(size.width * 0.18f, size.height * 0.36f)
                lineTo(size.width * 0.5f, size.height * 0.68f)
                lineTo(size.width * 0.82f, size.height * 0.36f)
            } else {
                moveTo(size.width * 0.36f, size.height * 0.18f)
                lineTo(size.width * 0.68f, size.height * 0.5f)
                lineTo(size.width * 0.36f, size.height * 0.82f)
            }
        }
        drawPath(path, FaintText, style = Stroke(strokeWidth, cap = StrokeCap.Round, join = StrokeJoin.Round))
    }
}

@Composable
private fun ExplorerFolderIcon(color: Color) {
    Canvas(Modifier.size(17.dp)) {
        val strokeWidth = 1.5.dp.toPx()
        val path = Path().apply {
            moveTo(size.width * 0.08f, size.height * 0.3f)
            lineTo(size.width * 0.36f, size.height * 0.3f)
            lineTo(size.width * 0.48f, size.height * 0.14f)
            lineTo(size.width * 0.86f, size.height * 0.14f)
            lineTo(size.width * 0.94f, size.height * 0.3f)
            lineTo(size.width * 0.88f, size.height * 0.86f)
            lineTo(size.width * 0.12f, size.height * 0.86f)
            close()
        }
        drawPath(path, color.copy(alpha = 0.18f))
        drawPath(path, color, style = Stroke(strokeWidth, cap = StrokeCap.Round, join = StrokeJoin.Round))
    }
}

@Composable
private fun ExplorerFileIcon(color: Color) {
    Canvas(Modifier.size(17.dp)) {
        val strokeWidth = 1.5.dp.toPx()
        val left = size.width * 0.18f
        val top = size.height * 0.08f
        val right = size.width * 0.82f
        val bottom = size.height * 0.92f
        val fold = size.width * 0.57f
        val foldBottom = size.height * 0.34f
        val path = Path().apply {
            moveTo(left, top)
            lineTo(fold, top)
            lineTo(right, foldBottom)
            lineTo(right, bottom)
            lineTo(left, bottom)
            close()
        }
        drawPath(path, color.copy(alpha = 0.1f))
        drawPath(path, color, style = Stroke(strokeWidth, cap = StrokeCap.Round, join = StrokeJoin.Round))
        drawLine(color, Offset(fold, top), Offset(fold, foldBottom), strokeWidth, cap = StrokeCap.Round)
        drawLine(color, Offset(fold, foldBottom), Offset(right, foldBottom), strokeWidth, cap = StrokeCap.Round)
    }
}

internal fun explorerRoleLabel(row: ExplorerRow, expanded: Boolean): String = when {
    row.directory && expanded -> "DIR −"
    row.directory -> "DIR +"
    else -> "FILE"
}

internal fun explorerRowDescription(row: ExplorerRow, selected: Boolean, expanded: Boolean): String = when {
    row.directory -> "Folder ${row.name}, ${if (expanded) "expanded" else "collapsed"}${if (selected) ", selected" else ""}"
    else -> "${row.language.ifBlank { "text" }} file ${row.name}, ${statusBadgeStyle(row.analysisStatus).label}${if (selected) ", selected" else ", not selected"}"
}

@Composable
private fun LoadingRows(label: String) {
    Column {
        Text("$label…", color = SecondaryText, fontSize = 12.sp)
        repeat(3) { Box(Modifier.fillMaxWidth().height(20.dp).padding(top = 6.dp).background(Card, RoundedCornerShape(4.dp))) }
    }
}
