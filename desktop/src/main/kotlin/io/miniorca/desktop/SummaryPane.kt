package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Button
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun SummaryPane(
    selected: ProjectFileInfo?, symbols: List<SymbolInfo>, analysis: FileAnalysis?, selectedSymbol: SymbolInfo?, analysisInProgress: Boolean,
    onAnalyze: () -> Unit, onRefresh: () -> Unit, onCancel: () -> Unit, onSelectSymbol: (SymbolInfo) -> Unit, onPrepareSuggestion: (Suggestion) -> Unit,
) {
    if (selected == null) {
        EmptyPane("Summary", "Select a file to view its deterministic facts and semantic summary.")
        return
    }
    val state = summaryState(analysis, symbols)
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        androidx.compose.foundation.layout.Row(verticalAlignment = Alignment.CenterVertically) {
            Text("SUMMARY", fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.width(10.dp))
            Text(analysisBadge(state.status), color = badgeColor(state.status), fontSize = 12.sp)
            Spacer(Modifier.weight(1f))
            if (analysisInProgress) Button(onClick = onCancel) { Text("Cancel") }
            else {
                Button(onClick = onAnalyze) { Text("Analyze") }
                Spacer(Modifier.width(8.dp))
                Button(onClick = onRefresh) { Text("Refresh") }
            }
        }
        Spacer(Modifier.height(18.dp))
        SummarySection("DETERMINISTIC FACTS") {
            Text("${selected.path} · ${selected.language} · ${selected.lineCount} lines · ${formatBytes(selected.sizeBytes)}", color = PrimaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
            if (state.approximateSymbols) Text("Some targets are approximate; confirm their scope before generation.", color = Warning, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
        }
        Spacer(Modifier.height(16.dp))
        SummarySection("SYMBOLS") {
            if (state.emptySymbols) Text("No symbols were extracted from this file.", color = SecondaryText, fontSize = 12.sp)
            symbols.forEach { symbol ->
                Column(
                    Modifier.fillMaxWidth().background(if (selectedSymbol?.name == symbol.name) Card else androidx.compose.ui.graphics.Color.Transparent, RoundedCornerShape(5.dp))
                        .clickable { onSelectSymbol(symbol) }.padding(8.dp),
                ) {
                    Text("${symbol.kind}  ${symbol.name}", fontFamily = FontFamily.Monospace, fontSize = 12.sp)
                    Text("${symbol.signature.ifBlank { "No signature" }} · lines ${symbol.startLine}–${symbol.endLine} · ${symbol.confidence} · ${if (symbol.atomicTarget) "atomic target" else "not atomic"}", color = SecondaryText, fontSize = 11.sp)
                }
            }
        }
        Spacer(Modifier.height(16.dp))
        SummarySection("MODEL-GENERATED INTERPRETATION") {
            when (state.status) {
                "fresh", "stale" -> {
                    Text(analysis?.purpose.orEmpty().ifBlank { "No purpose returned." }, color = PrimaryText, fontSize = 13.sp)
                    LabeledItems("Responsibilities", analysis?.responsibilities.orEmpty())
                    LabeledItems("Dependencies", analysis?.dependencies.orEmpty())
                    LabeledItems("Side effects", analysis?.sideEffects.orEmpty())
                    selectedSymbol?.let { symbol -> analysis?.symbolExplanations?.get(symbol.name)?.let { explanation -> LabeledItems("Explanation · ${symbol.name}", listOf(explanation)) } }
                    LabeledItems("Findings (model suggestions)", analysis?.risks.orEmpty().map { "${it.severity.uppercase()} · ${it.summary}" })
                    Text("Suggested atomic tasks (model suggestions)", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 12.dp))
                    analysis?.suggestions.orEmpty().forEach { suggestion ->
                        Button(onClick = { onPrepareSuggestion(suggestion) }, modifier = Modifier.padding(top = 5.dp)) { Text(suggestion.title) }
                        Text(suggestion.summary, color = SecondaryText, fontSize = 11.sp)
                    }
                }
                "failed" -> Text(state.failure.ifBlank { "The model could not produce a usable summary. Retry the analysis." }, color = Error, fontSize = 13.sp)
                "running" -> Text("Analysis is running for this file.", color = SecondaryText, fontSize = 13.sp)
                else -> Text("No semantic summary exists yet. Analyze sends only this selected file and bounded project facts.", color = SecondaryText, fontSize = 13.sp)
            }
        }
    }
}

@Composable
internal fun CodePane(project: ProjectAnalysis?, selected: ProjectFileInfo?, selectedSymbol: SymbolInfo?, focusedLine: Int) {
    val source = when {
        selected != null -> if (selected.binary) "Binary file: source preview is unavailable." else selected.content
        project != null -> project.summary
        else -> "Select Import to analyze a project. Mini-Orca indexes only policy-eligible project files."
    }
    SelectionContainer {
        Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
            if (focusedLine > 0) {
                Text(
                    "Editor context · ${selectedSymbol?.name ?: "line $focusedLine"} · line $focusedLine",
                    color = SecondaryText,
                    fontSize = 11.sp,
                    modifier = Modifier.padding(bottom = 10.dp),
                )
            }
            Text(text = highlightedCode(source), color = PrimaryText, fontFamily = if (selected != null) FontFamily.Monospace else FontFamily.Default, fontSize = 13.sp, lineHeight = 20.sp)
        }
    }
}

@Composable
internal fun EmptyPane(title: String, message: String) {
    Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.TopCenter) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(title, color = PrimaryText, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(8.dp))
            Text(message, color = SecondaryText, fontSize = 13.sp)
        }
    }
}

@Composable
private fun SummarySection(title: String, content: @Composable () -> Unit) {
    Text(title, color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
    Spacer(Modifier.height(6.dp))
    content()
}

@Composable
private fun LabeledItems(label: String, values: List<String>) {
    if (values.isEmpty()) return
    Text(label, color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 12.dp))
    values.forEach { Text("• $it", color = PrimaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 3.dp)) }
}
