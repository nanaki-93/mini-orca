package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Button
import androidx.compose.material.Checkbox
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun FocusedActionPane(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    session: ChatSession?,
    draft: DeclarationDraft?,
    editor: EditableDraftState?,
    mode: ChatEditMode,
    newSymbol: String,
    message: String,
    sending: Boolean,
    remoteProvider: Boolean,
    remoteConfirmed: Boolean,
    onSelectSymbol: (SymbolInfo) -> Unit,
    onMode: (ChatEditMode) -> Unit,
    onNewSymbol: (String) -> Unit,
    onMessage: (String) -> Unit,
    onRemoteConfirmed: (Boolean) -> Unit,
    onInspectContext: () -> Unit,
    onDraftDeclaration: (String) -> Unit,
    onDraftImports: (List<String>) -> Unit,
    onValidateDraft: () -> Unit,
    onSend: () -> Unit,
    onCancel: () -> Unit,
    modifier: Modifier,
) {
    val validation = validateChatTarget(selected, symbols, selectedSymbol, mode, newSymbol)
    val boundSession = validation.target?.let { target -> chatSessionMatches(session, selected, project, target) }
    Column(modifier.background(Panel).border(androidx.compose.foundation.BorderStroke(1.dp, Border)).padding(16.dp).verticalScroll(rememberScrollState())) {
        Text("FILE-SCOPED CHAT", fontWeight = FontWeight.SemiBold)
        Text("One project · one open file · one declaration", color = SecondaryText, fontSize = 12.sp)
        Spacer(Modifier.height(12.dp))
        Text("BOUND FILE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Text(selected?.path ?: "Open a Go file to begin", fontFamily = FontFamily.Monospace, fontSize = 12.sp)
        Text(
            when {
                boundSession == true -> "Conversation active for ${session?.targetSymbol}."
                validation.target != null -> "A conversation will be bound to ${validation.target.symbol} when you send."
                else -> validation.message
            },
            color = if (validation.valid) SecondaryText else Warning,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = 4.dp),
        )
        Spacer(Modifier.height(12.dp))
        Text("EDIT MODE", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        ChatEditMode.entries.forEach { option ->
            Text(
                option.label,
                color = if (option == mode) Accent else PrimaryText,
                fontSize = 12.sp,
                modifier = Modifier.fillMaxWidth().clickable(enabled = !sending) { onMode(option) }.padding(top = 5.dp),
            )
        }
        if (mode == ChatEditMode.ReplaceSymbol) {
            Text("SELECTED SYMBOL", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 12.dp))
            if (symbols.isEmpty()) Text("No extracted symbols are available for this file.", color = SecondaryText, fontSize = 11.sp)
            symbols.filter { it.kind.lowercase() in setOf("function", "type") }.forEach { symbol ->
                Text(
                    "${symbol.kind} · ${symbol.signature.ifBlank { symbol.name }} · lines ${symbol.startLine}–${symbol.endLine}",
                    color = if (symbol == selectedSymbol) Accent else PrimaryText,
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.sp,
                    modifier = Modifier.fillMaxWidth().clickable(enabled = !sending) { onSelectSymbol(symbol) }.padding(top = 5.dp),
                )
            }
        } else {
            OutlinedTextField(
                value = newSymbol,
                onValueChange = onNewSymbol,
                enabled = !sending,
                label = { Text("New Go function or type name") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
            )
        }
        Spacer(Modifier.height(12.dp))
        Text("CONVERSATION", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (boundSession == true && session != null) {
            if (session.messages.isEmpty()) Text("No messages yet. Send an explicit request to create the first proposal.", color = SecondaryText, fontSize = 11.sp)
            session.messages.forEach { turn ->
                Text(turn.role.uppercase(), color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 7.dp))
                Text(turn.content, color = PrimaryText, fontSize = 12.sp)
                if (turn.draftId.isNotBlank()) Text("Draft ${turn.draftId}", color = SecondaryText, fontSize = 10.sp, fontFamily = FontFamily.Monospace)
            }
            if (chatDraftMatchesSession(draft, session) && editor != null) EditableDraftPane(editor, onDraftDeclaration, onDraftImports, onValidateDraft)
        } else Text("Only messages from this bound file session appear here.", color = SecondaryText, fontSize = 11.sp)
        Spacer(Modifier.height(12.dp))
        OutlinedTextField(
            value = message,
            onValueChange = onMessage,
            enabled = !sending,
            label = { Text("Message") },
            placeholder = { Text("Describe one declaration change") },
            modifier = Modifier.fillMaxWidth(),
            minLines = 3,
        )
        if (remoteProvider) {
            Row(modifier = Modifier.padding(top = 8.dp)) {
                Checkbox(checked = remoteConfirmed, onCheckedChange = onRemoteConfirmed, enabled = !sending)
                Text("Confirm before sending bounded context to the remote provider", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 12.dp))
            }
        }
        Button(onClick = onInspectContext, enabled = selected != null && !sending, modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) { Text("Inspect context") }
        if (sending) Button(onClick = onCancel, modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) { Text("Cancel request") }
        else Button(onClick = onSend, enabled = validation.valid && message.isNotBlank() && (!remoteProvider || remoteConfirmed), modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) { Text("Send message") }
        Text("Sending creates a preview-only declaration draft. It never writes project source.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 10.dp))
    }
}

@Composable
private fun EditableDraftPane(
    editor: EditableDraftState,
    onDeclaration: (String) -> Unit,
    onImports: (List<String>) -> Unit,
    onValidate: () -> Unit,
) {
    Column(Modifier.fillMaxWidth().background(Card, RoundedCornerShape(6.dp)).padding(8.dp)) {
        val draft = editor.serverDraft
        Text("EDITABLE DRAFT · ${editor.status.name.lowercase()}", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Text("${draft.mode} · ${draft.targetSymbol}${draft.parentDraftId.takeIf { it.isNotBlank() }?.let { " · revises $it" }.orEmpty()}", color = SecondaryText, fontSize = 10.sp)
        OutlinedTextField(
            value = editor.declaration,
            onValueChange = onDeclaration,
            enabled = editor.status != DraftEditorStatus.Validating && editor.status != DraftEditorStatus.Stale,
            label = { Text("Declaration only") },
            modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
            minLines = 5,
            textStyle = androidx.compose.ui.text.TextStyle(fontFamily = FontFamily.Monospace, fontSize = 11.sp),
        )
        OutlinedTextField(
            value = editor.imports.joinToString(", "),
            onValueChange = { value -> onImports(value.split(',').map { it.trim() }.filter { it.isNotBlank() }) },
            enabled = editor.status != DraftEditorStatus.Validating && editor.status != DraftEditorStatus.Stale,
            label = { Text("Required imports (comma separated)") },
            modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
            singleLine = true,
        )
        editor.diagnostics.take(8).forEach { diagnostic ->
            Text("${diagnostic.code}: ${diagnostic.message}", color = Error, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
        }
        when (editor.status) {
            DraftEditorStatus.Dirty -> Text("Manual edits cleared prior validation and checks.", color = Warning, fontSize = 10.sp, modifier = Modifier.padding(top = 5.dp))
            DraftEditorStatus.Invalid -> Text("Fix the diagnostics and validate again. Invalid drafts cannot run checks or Apply.", color = Error, fontSize = 10.sp, modifier = Modifier.padding(top = 5.dp))
            DraftEditorStatus.Stale -> Text("This draft no longer matches the open file. Start a new conversation.", color = Error, fontSize = 10.sp, modifier = Modifier.padding(top = 5.dp))
            DraftEditorStatus.Valid -> Text("Validated declaration. Checks and Apply remain in the review step.", color = Success, fontSize = 10.sp, modifier = Modifier.padding(top = 5.dp))
            else -> Unit
        }
        Button(
            onClick = onValidate,
            enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
            modifier = Modifier.fillMaxWidth().padding(top = 7.dp),
        ) { Text(if (editor.status == DraftEditorStatus.Validating) "Validating declaration…" else "Validate draft") }
        Text("Validation formats and composes the declaration in memory; it never writes project source.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 5.dp))
    }
}
