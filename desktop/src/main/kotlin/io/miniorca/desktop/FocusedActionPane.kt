package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun FocusedActionPane(
    selected: ProjectFileInfo?, symbols: List<SymbolInfo>, selectedSymbol: SymbolInfo?, action: String, request: String, manualSymbol: String,
    scopeMode: String, generating: Boolean, onSelectSymbol: (SymbolInfo) -> Unit, onAction: (String) -> Unit, onRequest: (String) -> Unit,
    onManualSymbol: (String) -> Unit, onTemplate: (String) -> Unit, onToggleScope: () -> Unit, onInspectContext: () -> Unit, onGenerate: () -> Unit, onCancel: () -> Unit,
    onExplain: () -> Unit, modifier: Modifier,
) {
    Column(modifier.background(Panel).border(BorderStroke(1.dp, Border)).padding(16.dp).verticalScroll(rememberScrollState())) {
        Text("FOCUSED ACTION", fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(4.dp))
        Text("One project · one file · one symbol", color = SecondaryText, fontSize = 12.sp)
        Spacer(Modifier.height(20.dp))
        Text("TARGET FILE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Text(selected?.path ?: "Select a source file", fontFamily = FontFamily.Monospace, fontSize = 12.sp)
        Spacer(Modifier.height(16.dp))
        Text("SYMBOLS", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (selected == null) Text("Choose a file to load atomic targets.", color = SecondaryText, fontSize = 12.sp)
        else if (symbols.isEmpty()) Text("No exact atomic targets are available for this file.", color = SecondaryText, fontSize = 12.sp)
        else symbols.take(8).forEach { symbol ->
            Text("${symbol.kind}  ${symbol.name}  L${symbol.startLine}–${symbol.endLine}", color = if (selectedSymbol?.name == symbol.name) Accent else PrimaryText, fontSize = 11.sp, fontFamily = FontFamily.Monospace, modifier = Modifier.clickable { onSelectSymbol(symbol) }.padding(top = 5.dp))
        }
        selectedSymbol?.let { symbol ->
            Spacer(Modifier.height(14.dp))
            Text("SELECTED SYMBOL", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
            Text(symbol.name, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
            Button(onClick = onExplain, modifier = Modifier.padding(top = 6.dp)) { Text("Explain symbol") }
        }
        Spacer(Modifier.height(16.dp))
        Text("MANUAL TARGET", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        OutlinedTextField(value = manualSymbol, onValueChange = onManualSymbol, enabled = !generating, placeholder = { Text("Function or class name") }, singleLine = true, modifier = Modifier.fillMaxWidth())
        if (selectedSymbol == null) Text("Manual targets are checked conservatively and may be rejected without an exact parser-backed symbol.", color = Warning, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(14.dp))
        Text("TEMPLATE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        promptTemplates.forEach { template -> Button(onClick = { onTemplate(template.id) }, enabled = selected != null && !generating, modifier = Modifier.padding(top = 4.dp), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(template.label, fontSize = 10.sp) } }
        Spacer(Modifier.height(10.dp))
        Text("ACTION", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Row(horizontalArrangement = Arrangement.spacedBy(4.dp), modifier = Modifier.fillMaxWidth()) {
            listOf("fix", "refactor", "document").forEach { option -> Button(onClick = { onAction(option) }, enabled = !generating, colors = ButtonDefaults.buttonColors(backgroundColor = if (action == option) Accent else Card, contentColor = PrimaryText)) { Text(option.replaceFirstChar { it.uppercase() }, fontSize = 10.sp) } }
        }
        Spacer(Modifier.height(10.dp))
        OutlinedTextField(value = request, onValueChange = onRequest, enabled = !generating, label = { Text("Request") }, placeholder = { Text("Describe one focused change") }, modifier = Modifier.fillMaxWidth(), minLines = 3)
        Spacer(Modifier.height(12.dp))
        Text("SCOPE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Button(onClick = onToggleScope, enabled = !generating, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(if (scopeMode == "strict_symbol") "Strict symbol" else "Symbol + required imports", fontSize = 11.sp) }
        Spacer(Modifier.height(12.dp))
        Button(onClick = onInspectContext, enabled = selected != null && !generating, modifier = Modifier.fillMaxWidth(), colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("Inspect context") }
        Spacer(Modifier.height(8.dp))
        if (generating) Button(onClick = onCancel, modifier = Modifier.fillMaxWidth()) { Text("Cancel generation") }
        else Button(onClick = onGenerate, enabled = selected != null && action != "analyze_file" && action != "explain_symbol", modifier = Modifier.fillMaxWidth(), colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = Color.White)) { Text("Generate preview") }
        if (action == "analyze_file" || action == "explain_symbol") Text("This template is read-only. Review its summary without creating a candidate.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(12.dp))
        Text("Nothing is written automatically. Review the diff and required checks before Apply.", color = SecondaryText, fontSize = 11.sp)
    }
}
