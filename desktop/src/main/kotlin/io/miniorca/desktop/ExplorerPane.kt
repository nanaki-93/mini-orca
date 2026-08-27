package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
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
internal fun ExplorerPane(
    index: ProjectIndex?, selectedPath: String?, filter: String, collapsedDirectories: Set<String>, onFilter: (String) -> Unit,
    onToggleDirectory: (String) -> Unit, onSelect: (String) -> Unit, loading: Boolean, modifier: Modifier,
) {
    val rows = visibleExplorerRows(index?.files.orEmpty(), filter, collapsedDirectories)
    FocusFlowPanel(modifier = modifier, contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp)) {
        SectionLabel("PROJECT EXPLORER")
        Text(explorerProjectLabel(index), color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 3.dp))
        Spacer(Modifier.height(10.dp))
        OutlinedTextField(
            value = filter,
            onValueChange = onFilter,
            label = { Text("Filter indexed files") },
            placeholder = { Text("Type a relative path", color = SecondaryText, fontSize = 12.sp) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth().semantics { contentDescription = "Filter indexed relative file paths" },
        )
        Spacer(Modifier.height(8.dp))
        when {
            index == null && loading -> LoadingRows("Loading indexed files")
            index == null -> SystemStateMessage("No project open", "Open a project to browse safe, indexed relative paths.")
            rows.isEmpty() -> SystemStateMessage("No matching files", "Change the filter to view indexed relative paths.")
            else -> LazyColumn {
                items(rows, key = { it.path }) { row ->
                    ExplorerItem(
                        row = row,
                        selected = !row.directory && row.path == selectedPath,
                        expanded = row.path !in collapsedDirectories,
                        onActivate = { if (row.directory) onToggleDirectory(row.path) else onSelect(row.path) },
                    )
                }
            }
        }
        Spacer(Modifier.height(8.dp))
        Text("Freshness labels: Fresh, Stale, Failed, Running, or Not analyzed.", color = SecondaryText, fontSize = 10.sp)
    }
}

@Composable
private fun ExplorerItem(row: ExplorerRow, selected: Boolean, expanded: Boolean, onActivate: () -> Unit) {
    val description = explorerRowDescription(row, selected, expanded)
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
        Text(explorerRoleLabel(row, expanded), color = if (selected) CyanAccent else FaintText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.width(7.dp))
        Text(row.name, color = if (selected) PrimaryText else SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp, modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (!row.directory) {
            Text(row.language.ifBlank { "Text" }.uppercase(), color = FaintText, fontSize = 9.sp, maxLines = 1)
            Spacer(Modifier.width(6.dp))
            StatusBadge(row.analysisStatus)
        }
    }
}

internal fun explorerProjectLabel(index: ProjectIndex?): String = index?.let { "${it.files.size} indexed files · project revision ${it.projectRevision.take(12)}" } ?: "Indexed relative paths only"

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
