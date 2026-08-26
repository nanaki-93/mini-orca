package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.AlertDialog
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun CommandPaletteDialog(
    mode: PaletteMode, query: String, onQuery: (String) -> Unit, files: List<IndexedFile>, symbols: List<SymbolInfo>,
    onSelectFile: (String) -> Unit, onSelectSymbol: (SymbolInfo) -> Unit, onSelectAction: (String) -> Unit, onDismiss: () -> Unit,
) {
    val title = when (mode) {
        PaletteMode.Files -> "Go to file · ⌘P"
        PaletteMode.Symbols -> "Go to symbol · ⌘⇧O"
        PaletteMode.Actions -> "Focused action · ⌘K"
    }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column {
                OutlinedTextField(value = query, onValueChange = onQuery, singleLine = true, label = { Text("Filter") }, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(8.dp))
                when (mode) {
                    PaletteMode.Files -> files.filter { it.path.contains(query, ignoreCase = true) }.take(12).forEach { file ->
                        Button(onClick = { onSelectFile(file.path) }, modifier = Modifier.fillMaxWidth().padding(top = 3.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(file.path, fontFamily = FontFamily.Monospace, fontSize = 11.sp) }
                    }
                    PaletteMode.Symbols -> symbols.filter { it.name.contains(query, ignoreCase = true) }.take(12).forEach { symbol ->
                        Button(onClick = { onSelectSymbol(symbol) }, modifier = Modifier.fillMaxWidth().padding(top = 3.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("${symbol.kind} · ${symbol.name}", fontFamily = FontFamily.Monospace, fontSize = 11.sp) }
                    }
                    PaletteMode.Actions -> listOf("fix", "refactor", "document").filter { it.contains(query, ignoreCase = true) }.forEach { action ->
                        Button(onClick = { onSelectAction(action) }, modifier = Modifier.fillMaxWidth().padding(top = 3.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(action.replaceFirstChar { it.uppercase() }) }
                    }
                }
            }
        },
        confirmButton = { Button(onClick = onDismiss) { Text("Close") } },
    )
}
