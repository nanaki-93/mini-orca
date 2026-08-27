package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

data class DiffCell(val lineNumber: Int?, val text: String, val change: String)
data class DiffRow(val before: DiffCell?, val proposed: DiffCell?)

fun sideBySideDiffRows(diff: UnifiedDiff): List<DiffRow> {
    val rows = mutableListOf<DiffRow>(); var pendingRemoval: DiffLine? = null
    fun add(before: DiffLine?, after: DiffLine?) { rows += DiffRow(before?.let { DiffCell(it.oldLine.takeIf { n -> n > 0 }, it.text, it.kind) }, after?.let { DiffCell(it.newLine.takeIf { n -> n > 0 }, it.text, it.kind) }) }
    diff.lines.forEach { line -> when (line.kind) {
        "removed" -> { pendingRemoval?.let { add(it, null) }; pendingRemoval = line }
        "added" -> { add(pendingRemoval, line); pendingRemoval = null }
        else -> { pendingRemoval?.let { add(it, null); pendingRemoval = null }; add(line, line) }
    } }
    pendingRemoval?.let { add(it, null) }; return rows
}

@Composable
internal fun DiffViewer(diff: UnifiedDiff?, sideBySide: Boolean = true, modifier: Modifier = Modifier) {
    if (diff == null) { SystemStateMessage("Composed diff unavailable", "Validate the latest draft to view the read-only composed diff.", modifier = modifier); return }
    SelectionContainer { Column(modifier.background(Card).padding(8.dp)) {
        Text(if (sideBySide) "BEFORE (READ-ONLY)                                      PROPOSED (READ-ONLY)" else "UNIFIED COMPOSED DIFF (READ-ONLY)", color = SecondaryText, fontSize = 10.sp)
        if (sideBySide) sideBySideDiffRows(diff).forEach { row -> Row(Modifier.fillMaxWidth()) { DiffCellText(row.before, "Before", Modifier.weight(1f)); DiffCellText(row.proposed, "Proposed", Modifier.weight(1f)) } }
        else diff.lines.forEach { line -> Text("${line.kind.uppercase()} ${line.oldLine.takeIf { it > 0 } ?: line.newLine} ${line.text}", color = if (line.kind == "added") Success else if (line.kind == "removed") Error else PrimaryText, fontFamily = FontFamily.Monospace, fontSize = 10.sp) }
    } }
}

@Composable private fun DiffCellText(cell: DiffCell?, side: String, modifier: Modifier) { Text(cell?.let { "${it.change.uppercase()} ${it.lineNumber ?: ""} ${it.text}" } ?: "$side unchanged", color = when (cell?.change) { "added" -> Success; "removed" -> Error; else -> PrimaryText }, fontFamily = FontFamily.Monospace, fontSize = 10.sp, modifier = modifier) }
