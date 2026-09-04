package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
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
internal fun AssistantToolWindow(
    state: AssistantToolWindowState,
    conversationActions: AssistantConversationActions,
    editorActions: DraftEditorActions,
    modifier: Modifier,
) {
  val bound =
      state.target != null &&
          chatSessionMatches(state.session, state.selected, state.project, state.target)
  Column(modifier) {
    ToolWindowScopeHeader(
        "ASSISTANT",
        assistantToolWindowScope(state.selected, state.target, state.draft, state.newSymbol),
        Modifier.padding(12.dp))
    Column(
        Modifier.weight(1f)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 12.dp)
            .padding(bottom = 12.dp)) {
          FocusFlowPanel(Modifier.fillMaxWidth()) {
            SectionLabel("BOUND CONVERSATION")
            Text(
                state.selected?.path ?: "Open one Go file before drafting.",
                color = PrimaryText,
                fontFamily = FontFamily.Monospace,
                fontSize = 12.sp)
            Text(
                state.target?.let { "${it.mode.label} · ${it.symbol}" }
                    ?: "Select an eligible declaration or enter a new declaration name.",
                color = if (state.target == null) Warning else SecondaryText,
                fontSize = 11.sp)
            if (bound && state.session != null) {
              state.session.messages.forEach { turn ->
                Text(
                    turn.role.uppercase(),
                    color = SecondaryText,
                    fontSize = 10.sp,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(top = 7.dp))
                Text(turn.content, color = PrimaryText, fontSize = 12.sp)
              }
            }
            if (state.mode == ChatEditMode.CreateSymbol) {
              CompactSingleLineField(
                  state.newSymbol,
                  conversationActions.updateNewSymbol,
                  label = { Text("New Go function or type name") },
                  modifier = Modifier.fillMaxWidth().padding(top = 9.dp))
            }
            OutlinedTextField(
                state.message,
                conversationActions.updateMessage,
                enabled = !state.sending && state.target != null,
                label = { Text("Message") },
                placeholder = { Text("Describe one declaration change") },
                minLines = 3,
                modifier =
                    Modifier.fillMaxWidth().padding(top = 9.dp).focusRequester(state.chatFocus))
            RemoteProviderConfirmation(
                ModelScope.Function,
                state.functionModel,
                state.remoteConfirmed,
                conversationActions.confirmRemoteProvider)
            FocusFlowButton(
                onClick = conversationActions.inspectContext,
                enabled = state.selected != null && !state.sending,
                tone = ActionTone.Neutral,
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) {
                  Text("Inspect context")
                }
            if (state.sending)
                FocusFlowButton(
                    onClick = conversationActions.cancel,
                    tone = ActionTone.Destructive,
                    modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
                      Text("Cancel request")
                    }
            else
                FocusFlowButton(
                    onClick = conversationActions.send,
                    enabled =
                        state.target != null &&
                            state.message.isNotBlank() &&
                            (!state.functionModel.remoteProvider || state.remoteConfirmed),
                    tone = ActionTone.Primary,
                    modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
                      Text("Send message")
                    }
          }
          if (state.draft != null &&
              state.editor != null &&
              chatDraftMatchesSession(state.draft, state.session)) {
            EngineeringInsightPanel(
                state.draft.engineeringInsight,
                stale = state.draft.state.equals("stale", ignoreCase = true),
                scopeLabel = "Current proposal")
            AssistantDraftEditorCard(state.editor, state.draftFocus, editorActions)
          }
        }
  }
}

@Composable
private fun AssistantDraftEditorCard(
    editor: EditableDraftState,
    draftFocus: FocusRequester,
    actions: DraftEditorActions
) {
  FocusFlowPanel(Modifier.fillMaxWidth().padding(top = 10.dp), raised = true) {
    val draft = editor.serverDraft
    val canValidate =
        editor.status in
            setOf(DraftEditorStatus.Generated, DraftEditorStatus.Dirty, DraftEditorStatus.Invalid)
    SectionLabel("EDITABLE DECLARATION DRAFT · ${editor.status.name.lowercase()}")
    Text(
        draft.targetSymbol,
        color = SecondaryText,
        fontFamily = FontFamily.Monospace,
        fontSize = 10.sp)
    OutlinedTextField(
        editor.declaration,
        actions.updateDeclaration,
        enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
        label = { Text("Declaration only") },
        minLines = 5,
        textStyle =
            androidx.compose.ui.text.TextStyle(fontFamily = FontFamily.Monospace, fontSize = 11.sp),
        modifier = Modifier.fillMaxWidth().padding(top = 7.dp).focusRequester(draftFocus))
    if (requiredImportsVisible(editor)) {
      CompactSingleLineField(
          editor.imports.joinToString(", "),
          { value -> actions.updateImports(parseRequiredImports(value)) },
          enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
          label = { Text("Required imports") },
          modifier = Modifier.fillMaxWidth().padding(top = 7.dp))
    }
    editor.diagnostics.forEach {
      Text("${it.code}: ${it.message}", color = Error, fontSize = 10.sp)
    }
    Text(
        draftEditorStatusMessage(editor.status),
        color = draftEditorStatusColor(editor.status),
        fontSize = 10.sp,
        modifier = Modifier.padding(top = 5.dp))
    FocusFlowButton(
        onClick = actions.validate,
        enabled = canValidate,
        tone = ActionTone.Primary,
        modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
          Text(
              if (editor.status == DraftEditorStatus.Validating) "Validating declaration…"
              else "Validate draft")
        }
  }
}

/** File-scoped drafting data rendered by the Assistant tool window. */
internal data class AssistantToolWindowState(
    val project: ProjectAnalysis?,
    val selected: ProjectFileInfo?,
    val session: ChatSession?,
    val draft: DeclarationDraft?,
    val editor: EditableDraftState?,
    val target: ChatTarget?,
    val mode: ChatEditMode,
    val newSymbol: String,
    val message: String,
    val sending: Boolean,
    val functionModel: ScopedModel,
    val remoteConfirmed: Boolean,
    val chatFocus: FocusRequester,
    val draftFocus: FocusRequester,
)

/** Conversation intents that do not mutate the editable declaration. */
internal data class AssistantConversationActions(
    val updateMessage: (String) -> Unit,
    val updateNewSymbol: (String) -> Unit,
    val confirmRemoteProvider: (Boolean) -> Unit,
    val inspectContext: () -> Unit,
    val send: () -> Unit,
    val cancel: () -> Unit,
)

/** Editable declaration intents, separate from the chat conversation. */
internal data class DraftEditorActions(
    val updateDeclaration: (String) -> Unit,
    val updateImports: (List<String>) -> Unit,
    val validate: () -> Unit,
)

internal fun requiredImportsVisible(editor: EditableDraftState): Boolean =
    editor.imports.isNotEmpty()

internal fun parseRequiredImports(value: String): List<String> =
    value.split(',').map { it.trim() }.filter { it.isNotBlank() }

internal fun draftEditorStatusMessage(status: DraftEditorStatus): String =
    when (status) {
      DraftEditorStatus.Generated -> "Validate this generated draft before review."
      DraftEditorStatus.Dirty -> "Manual edits cleared prior validation and checks."
      DraftEditorStatus.Validating -> "Validation is running."
      DraftEditorStatus.Valid ->
          "Validated declaration. Review evidence and focused checks are current context."
      DraftEditorStatus.Invalid -> "Fix validation diagnostics before continuing."
      DraftEditorStatus.Stale ->
          "This draft no longer matches the open file. Start a new conversation."
    }

internal fun draftEditorStatusColor(status: DraftEditorStatus) =
    when (status) {
      DraftEditorStatus.Valid -> Success
      DraftEditorStatus.Invalid,
      DraftEditorStatus.Stale -> Error
      DraftEditorStatus.Dirty,
      DraftEditorStatus.Validating -> Warning
      DraftEditorStatus.Generated -> SecondaryText
    }
