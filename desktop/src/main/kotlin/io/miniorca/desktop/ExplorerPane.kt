package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun ExplorerPane(
    index: ProjectIndex?, selectedPath: String?, filter: String, collapsedDirectories: Set<String>, onFilter: (String) -> Unit,
    onToggleDirectory: (String) -> Unit, onSelect: (String) -> Unit, loading: Boolean, modifier: Modifier,
) {
    val rows = visibleExplorerRows(index?.files.orEmpty(), filter, collapsedDirectories)
    Column(modifier.background(Panel).border(BorderStroke(1.dp, Border)).padding(12.dp)) {
        Text("EXPLORER", color = SecondaryText, fontSize = 11.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(10.dp))
        OutlinedTextField(value = filter, onValueChange = onFilter, placeholder = { Text("Filter files", color = SecondaryText, fontSize = 12.sp) }, singleLine = true, modifier = Modifier.fillMaxWidth())
        Spacer(Modifier.height(8.dp))
        when {
            index == null && loading -> LoadingRows("Loading indexed files")
            index == null -> Text("Import a project to browse its safe, indexed files.", color = SecondaryText, fontSize = 13.sp)
            rows.isEmpty() -> Text("No indexed files match this filter.", color = SecondaryText, fontSize = 13.sp)
            else -> LazyColumn {
                items(rows, key = { it.path }) { row ->
                    val active = !row.directory && row.path == selectedPath
                    Row(
                        modifier = Modifier.fillMaxWidth().semantics { contentDescription = if (row.directory) "Folder ${row.name}" else "${row.name}, ${analysisBadge(row.analysisStatus)}" }.background(if (active) Card else Color.Transparent, RoundedCornerShape(5.dp))
                            .clickable { if (row.directory) onToggleDirectory(row.path) else onSelect(row.path) }.padding(start = (8 + row.depth * 14).dp, end = 6.dp, top = 6.dp, bottom = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(if (row.directory) if (row.path in collapsedDirectories) "▸" else "▾" else "▱", color = SecondaryText, fontSize = 12.sp)
                        Spacer(Modifier.width(6.dp))
                        Text(row.name, color = if (active) PrimaryText else SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp, modifier = Modifier.weight(1f), maxLines = 1)
                        if (!row.directory) Text(analysisBadge(row.analysisStatus), color = badgeColor(row.analysisStatus), fontSize = 10.sp, maxLines = 1)
                    }
                }
            }
        }
        Spacer(Modifier.height(8.dp))
        Text("✓ fresh  ● stale  ○ not analyzed", color = SecondaryText, fontSize = 10.sp)
    }
}

@Composable
private fun LoadingRows(label: String) {
    Column {
        Text("$label…", color = SecondaryText, fontSize = 12.sp)
        repeat(3) { Box(Modifier.fillMaxWidth().height(20.dp).padding(top = 6.dp).background(Card, RoundedCornerShape(4.dp))) }
    }
}
