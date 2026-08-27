package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Button
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun TargetContextPane(
    selected: ProjectFileInfo?, analysis: FileAnalysis?, symbols: List<SymbolInfo>, selectedSymbol: SymbolInfo?, mode: ChatEditMode, newSymbol: String,
    remoteProvider: Boolean, remoteConfirmed: Boolean, onRemoteConfirmed: (Boolean) -> Unit, onSelectSymbol: (SymbolInfo) -> Unit,
    onMode: (ChatEditMode) -> Unit, onNewSymbol: (String) -> Unit, onAnalyze: () -> Unit, onRefresh: () -> Unit, modifier: Modifier,
) {
    val target = validateChatTarget(selected, symbols, selectedSymbol, mode, newSymbol)
    Column(modifier.verticalScroll(rememberScrollState())) {
        FocusFlowPanel(Modifier.fillMaxWidth()) {
            SectionLabel("TARGET")
            Text(selected?.path ?: "Open one Go file to choose a declaration.", color = PrimaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp, modifier = Modifier.padding(top = 5.dp))
            selected?.let { file ->
                Text("${file.language} · ${formatBytes(file.sizeBytes)} · ${file.lineCount} lines · hash ${file.contentHash.take(12)}", color = SecondaryText, fontSize = 11.sp)
                Text("Analysis freshness: ${analysis?.status?.ifBlank { "not analyzed" } ?: "not analyzed"}", color = SecondaryText, fontSize = 11.sp)
                Row(Modifier.padding(top = 8.dp)) {
                    Button(onClick = onAnalyze, enabled = !remoteProvider || remoteConfirmed) { Text("Analyze") }
                    Spacer(Modifier.width(6.dp))
                    Button(onClick = onRefresh, enabled = !remoteProvider || remoteConfirmed) { Text("Refresh") }
                }
            }
            if (remoteProvider) RemoteProviderConfirmation(remoteConfirmed, onRemoteConfirmed)
        }
        FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 10.dp), raised = true) {
            SectionLabel("DECLARATION SCOPE")
            ChatEditMode.entries.forEach { option ->
                Button(onClick = { onMode(option) }, modifier = Modifier.padding(top = 6.dp), enabled = selected != null) { Text(option.label) }
            }
            if (mode == ChatEditMode.CreateSymbol) {
                OutlinedTextField(newSymbol, onNewSymbol, label = { Text("New Go function or type name") }, singleLine = true, modifier = Modifier.fillMaxWidth().padding(top = 8.dp))
            } else {
                Text("Select an exact, atomic Go function or type.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 8.dp))
                symbols.filter { it.kind.lowercase() in setOf("function", "type") }.forEach { symbol ->
                    Button(onClick = { onSelectSymbol(symbol) }, modifier = Modifier.fillMaxWidth().padding(top = 4.dp)) {
                        Text("${symbol.kind} · ${symbol.name} · ${symbol.confidence} · ${if (symbol.atomicTarget) "atomic" else "not atomic"}", fontFamily = FontFamily.Monospace, fontSize = 10.sp)
                    }
                }
            }
            Text(target.target?.let { "Ready: ${it.mode.label} ${it.symbol}. Draft is now available." } ?: target.message, color = if (target.valid) Success else Warning, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 9.dp))
        }
    }
}
