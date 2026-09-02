package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.AlertDialog
import androidx.compose.material.Text
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.Composable
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun CommandPaletteDialog(
    mode: PaletteMode, query: String, onQuery: (String) -> Unit, files: List<IndexedFile>, symbols: List<SymbolInfo>, analysis: FileAnalysis?,
    onSelectFile: (String) -> Unit, onSelectSymbol: (SymbolInfo) -> Unit, onSelectAction: (String) -> Unit, onDismiss: () -> Unit,
) {
    val title = when (mode) {
        PaletteMode.Files -> "Go to file · ⌘P"
        PaletteMode.Symbols -> "Go to symbol · ⌘⇧O"
        PaletteMode.Actions -> "Focused action · ⌘K"
    }
    val filterFocusRequester = FocusRequester()
    val filteredFiles = files.filter { it.path.contains(query, ignoreCase = true) }.take(12)
    val filteredSymbols = symbols.filter { it.name.contains(query, ignoreCase = true) }.take(12)
    val filteredActions = availableCommandActions(analysis).filter { commandActionLabel(it).contains(query, ignoreCase = true) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column {
                CompactSingleLineField(value = query, onValueChange = onQuery, label = { Text("Filter") }, modifier = Modifier.fillMaxWidth().focusRequester(filterFocusRequester))
                Spacer(Modifier.height(8.dp))
                when (mode) {
                    PaletteMode.Files -> if (filteredFiles.isEmpty()) SystemStateMessage("No file matches", "Change the filter to search indexed relative paths.") else filteredFiles.forEach { file ->
                        PaletteEntry("File ${file.path}", file.path, onClick = { onSelectFile(file.path) })
                    }
                    PaletteMode.Symbols -> if (filteredSymbols.isEmpty()) SystemStateMessage("No symbol matches", "Open a file with an indexed symbol or change the filter.") else filteredSymbols.forEach { symbol ->
                        PaletteEntry("Symbol ${symbol.kind} ${symbol.name}", "${symbol.kind} · ${symbol.name}", onClick = { onSelectSymbol(symbol) })
                    }
                    PaletteMode.Actions -> if (filteredActions.isEmpty()) SystemStateMessage("No action matches", "Change the filter to view available focused actions.") else filteredActions.forEach { action ->
                        val label = commandActionLabel(action)
                        PaletteEntry("Action $label", label, onClick = { onSelectAction(action) })
                    }
                }
            }
        },
        confirmButton = { FocusFlowButton(onClick = onDismiss, tone = ActionTone.Neutral) { Text("Close") } },
    )
    LaunchedEffect(Unit) { filterFocusRequester.requestFocus() }
}

internal fun availableCommandActions(analysis: FileAnalysis?): List<String> = buildList {
    addAll(listOf("fix", "refactor", "document", "create_declaration"))
    if (analysis?.status.equals("fresh", ignoreCase = true)) add("refresh_file_analysis")
}

internal fun commandActionLabel(action: String): String = when (action) {
    "create_declaration" -> "Create declaration"
    "refresh_file_analysis" -> "Refresh file analysis"
    else -> action.replaceFirstChar { it.uppercase() }
}

@Composable
private fun PaletteEntry(description: String, label: String, onClick: () -> Unit) {
    FocusFlowButton(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth().padding(top = 3.dp).semantics { contentDescription = description },
        tone = ActionTone.Navigation,
    ) { Text(label, fontFamily = FontFamily.Monospace, fontSize = 11.sp) }
}
