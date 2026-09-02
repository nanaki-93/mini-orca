package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun DraftContextPane(
    project: ProjectAnalysis?, selected: ProjectFileInfo?, session: ChatSession?, draft: DeclarationDraft?, editor: EditableDraftState?, target: ChatTarget?, mode: ChatEditMode, newSymbol: String,
    message: String, sending: Boolean, remoteProvider: Boolean, remoteConfirmed: Boolean, chatFocus: FocusRequester, draftFocus: FocusRequester,
    onMessage: (String) -> Unit, onNewSymbol: (String) -> Unit, onRemoteConfirmed: (Boolean) -> Unit, onInspectContext: () -> Unit, onDraftDeclaration: (String) -> Unit,
    onDraftImports: (List<String>) -> Unit, onValidateDraft: () -> Unit, onSend: () -> Unit, onCancel: () -> Unit, modifier: Modifier,
) {
    val bound = target != null && chatSessionMatches(session, selected, project, target)
    Column(modifier.verticalScroll(rememberScrollState())) {
        FocusFlowPanel(Modifier.fillMaxWidth()) {
            SectionLabel("BOUND CONVERSATION")
            Text(selected?.path ?: "Open one Go file before drafting.", color = PrimaryText, fontFamily = FontFamily.Monospace, fontSize = 12.sp)
            Text(target?.let { "${it.mode.label} · ${it.symbol}" } ?: "Select an eligible declaration or enter a new declaration name.", color = if (target == null) Warning else SecondaryText, fontSize = 11.sp)
            if (bound && session != null) {
                session.messages.forEach { turn ->
                    Text(turn.role.uppercase(), color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 7.dp))
                    Text(turn.content, color = PrimaryText, fontSize = 12.sp)
                }
            }
            if (mode == ChatEditMode.CreateSymbol) {
                CompactSingleLineField(newSymbol, onNewSymbol, label = { Text("New Go function or type name") }, modifier = Modifier.fillMaxWidth().padding(top = 9.dp))
            }
            OutlinedTextField(message, onMessage, enabled = !sending && target != null, label = { Text("Message") }, placeholder = { Text("Describe one declaration change") }, minLines = 3, modifier = Modifier.fillMaxWidth().padding(top = 9.dp).focusRequester(chatFocus))
            RemoteProviderConfirmation(remoteProvider, remoteConfirmed, onRemoteConfirmed)
            FocusFlowButton(onClick = onInspectContext, enabled = selected != null && !sending, tone = ActionTone.Neutral, modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) { Text("Inspect context") }
            if (sending) FocusFlowButton(onClick = onCancel, tone = ActionTone.Destructive, modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) { Text("Cancel request") }
            else FocusFlowButton(onClick = onSend, enabled = target != null && message.isNotBlank() && (!remoteProvider || remoteConfirmed), tone = ActionTone.Primary, modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) { Text("Send message") }
        }
        if (draft != null && editor != null && chatDraftMatchesSession(draft, session)) DraftEditorCard(editor, draftFocus, onDraftDeclaration, onDraftImports, onValidateDraft)
    }
}

@Composable
private fun DraftEditorCard(editor: EditableDraftState, draftFocus: FocusRequester, onDeclaration: (String) -> Unit, onImports: (List<String>) -> Unit, onValidate: () -> Unit) {
    FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 10.dp), raised = true) {
        val draft = editor.serverDraft
        val canValidate = editor.status in setOf(DraftEditorStatus.Generated, DraftEditorStatus.Dirty, DraftEditorStatus.Invalid)
        SectionLabel("EDITABLE DECLARATION DRAFT · ${editor.status.name.lowercase()}")
        Text(draft.targetSymbol, color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 10.sp)
        OutlinedTextField(editor.declaration, onDeclaration, enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale), label = { Text("Declaration only") }, minLines = 5, textStyle = androidx.compose.ui.text.TextStyle(fontFamily = FontFamily.Monospace, fontSize = 11.sp), modifier = Modifier.fillMaxWidth().padding(top = 7.dp).focusRequester(draftFocus))
        CompactSingleLineField(editor.imports.joinToString(", "), { value -> onImports(value.split(',').map { it.trim() }.filter { it.isNotBlank() }) }, enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale), label = { Text("Required imports") }, modifier = Modifier.fillMaxWidth().padding(top = 7.dp))
        editor.diagnostics.forEach { Text("${it.code}: ${it.message}", color = Error, fontSize = 10.sp) }
        Text(draftEditorStatusMessage(editor.status), color = draftEditorStatusColor(editor.status), fontSize = 10.sp, modifier = Modifier.padding(top = 5.dp))
        FocusFlowButton(onClick = onValidate, enabled = canValidate, tone = ActionTone.Primary, modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) { Text(if (editor.status == DraftEditorStatus.Validating) "Validating declaration…" else "Validate draft") }
    }
}

internal fun draftEditorStatusMessage(status: DraftEditorStatus): String = when (status) {
    DraftEditorStatus.Generated -> "Validate this generated draft before review."
    DraftEditorStatus.Dirty -> "Manual edits cleared prior validation and checks."
    DraftEditorStatus.Validating -> "Validation is running."
    DraftEditorStatus.Valid -> "Validated declaration. Review evidence and focused checks are current context."
    DraftEditorStatus.Invalid -> "Fix validation diagnostics before continuing."
    DraftEditorStatus.Stale -> "This draft no longer matches the open file. Start a new conversation."
}

internal fun draftEditorStatusColor(status: DraftEditorStatus) = when (status) {
    DraftEditorStatus.Valid -> Success
    DraftEditorStatus.Invalid, DraftEditorStatus.Stale -> Error
    DraftEditorStatus.Dirty, DraftEditorStatus.Validating -> Warning
    DraftEditorStatus.Generated -> SecondaryText
}
