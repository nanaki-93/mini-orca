package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
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
  val draftVisible =
      state.draft != null &&
          state.editor != null &&
          chatDraftMatchesSession(state.draft, state.session)
  Column(modifier) {
    ToolWindowScopeHeader(
        "ASSISTANT",
        assistantToolWindowScope(state.selected, state.target, state.draft, state.newSymbol),
        Modifier)
    Column(
        Modifier.weight(1f)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 8.dp)
            .padding(bottom = 8.dp)) {
          Column(Modifier.fillMaxWidth()) {
            IdePaneHeader(
                title = "Conversation",
                icon = DesktopIcon.Editor,
                stateLabel =
                    if (bound) "Bound to the focused declaration" else "Select a declaration")
            Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
              Text(
                  state.selected?.path ?: "Open a file to draft.",
                  color = PrimaryText,
                  fontFamily = FontFamily.Monospace,
                  fontSize = 12.sp)
              Text(
                  state.target?.let { "${it.mode.label} · ${it.symbol}" }
                      ?: "Select a declaration or enter a new name.",
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
                    label = "New Go function or type name",
                    modifier = Modifier.fillMaxWidth().padding(top = 9.dp))
              }
              CompactMultilineField(
                  value = state.messageInput,
                  onValueChange = conversationActions.updateMessageValue,
                  label = "Message",
                  enabled = !state.sending && state.target != null,
                  placeholder = "Describe one declaration change",
                  minLines = 3,
                  modifier =
                      Modifier.fillMaxWidth().padding(top = 9.dp).focusRequester(state.chatFocus))
              RemoteProviderConfirmation(
                  ModelScope.Function,
                  state.functionModel,
                  state.remoteConfirmed,
                  conversationActions.confirmRemoteProvider)
              MiniOrcaButton(
                  onClick = conversationActions.inspectContext,
                  enabled = state.selected != null && !state.sending,
                  tone = ActionTone.Neutral,
                  modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) {
                    Text("Inspect context")
                  }
              if (state.sending)
                  MiniOrcaButton(
                      onClick = conversationActions.cancel,
                      tone = ActionTone.Destructive,
                      modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
                        Text("Cancel request")
                      }
              else
                  MiniOrcaButton(
                      onClick = conversationActions.send,
                      enabled =
                          state.target != null &&
                              state.message.isNotBlank() &&
                              (!state.functionModel.remoteProvider || state.remoteConfirmed),
                      tone = if (draftVisible) ActionTone.Neutral else ActionTone.Primary,
                      modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
                        Text("Send message")
                      }
            }
            IdeHorizontalSeparator()
          }
          if (draftVisible) {
            AssistantDraftEditorSection(
                state.editor,
                state.draftInput ?: TextFieldValue(state.editor.declaration),
                state.draftFocus,
                state.draft.engineeringInsight,
                state.draft.state.equals("stale", ignoreCase = true),
                editorActions)
          }
        }
  }
}

@Composable
private fun AssistantDraftEditorSection(
    editor: EditableDraftState,
    declaration: TextFieldValue,
    draftFocus: FocusRequester,
    insight: EngineeringInsight?,
    stale: Boolean,
    actions: DraftEditorActions
) {
  Column(Modifier.fillMaxWidth()) {
    val draft = editor.serverDraft
    val canValidate =
        editor.status in
            setOf(DraftEditorStatus.Generated, DraftEditorStatus.Dirty, DraftEditorStatus.Invalid)
    IdePaneHeader(
        title = "Editable draft",
        icon = DesktopIcon.Document,
        stateLabel = editor.status.name.lowercase(),
        stateTint = draftEditorStatusColor(editor.status))
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      Text(
          "Next: validate ${draft.targetSymbol} in ${draft.targetPath}.",
          color = SecondaryText,
          fontFamily = FontFamily.Monospace,
          fontSize = 10.sp)
      CompactMultilineField(
          value = declaration,
          onValueChange = actions.updateDeclarationValue,
          enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
          label = "Declaration only",
          minLines = 5,
          textStyle =
              androidx.compose.ui.text.TextStyle(
                  fontFamily = FontFamily.Monospace, fontSize = 11.sp),
          modifier = Modifier.fillMaxWidth().padding(top = 7.dp).focusRequester(draftFocus))
      if (requiredImportsVisible(editor)) {
        CompactSingleLineField(
            editor.imports.joinToString(", "),
            { value -> actions.updateImports(parseRequiredImports(value)) },
            enabled =
                editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
            label = "Required imports",
            modifier = Modifier.fillMaxWidth().padding(top = 7.dp))
      }
      AssistantValidationDiagnostics(editor.diagnostics, draft)
      Text(
          draftEditorStatusMessage(editor.status),
          color = draftEditorStatusColor(editor.status),
          fontSize = 10.sp,
          modifier = Modifier.padding(top = 5.dp))
      MiniOrcaButton(
          onClick = actions.validate,
          enabled = canValidate,
          tone = ActionTone.Primary,
          modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
            Text(
                if (editor.status == DraftEditorStatus.Validating) "Validating declaration…"
                else "Validate draft for ${draft.targetSymbol}")
          }
      EngineeringInsightPanel(insight, stale = stale, scopeLabel = "Current proposal")
    }
    IdeHorizontalSeparator()
  }
}

@Composable
private fun AssistantValidationDiagnostics(
    diagnostics: List<DeclarationFinding>,
    draft: DeclarationDraft,
) {
  if (diagnostics.isEmpty()) return
  var expanded by rememberSaveable(draft.id, draft.revision, draft.hash) { mutableStateOf(false) }
  Text(
      "${diagnostics.size} validation ${if (diagnostics.size == 1) "diagnostic" else "diagnostics"} require attention.",
      color = Error,
      fontSize = 10.sp,
      modifier = Modifier.padding(top = 5.dp))
  IdeDisclosureHeader(
      title = "Validation diagnostics",
      expanded = expanded,
      onToggle = { expanded = !expanded },
      stateLabel = if (expanded) "Expanded" else "Collapsed",
      stateTint = Error)
  if (expanded)
      SelectionContainer {
        Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
          diagnostics.forEach { diagnostic ->
            Text(
                "${diagnostic.code}: ${diagnostic.message}",
                color = Error,
                fontSize = 10.sp,
                modifier = Modifier.padding(top = 3.dp))
          }
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
    val messageInput: TextFieldValue = TextFieldValue(message),
    val draftInput: TextFieldValue? = null,
)

/** Conversation intents that do not mutate the editable declaration. */
internal data class AssistantConversationActions(
    val updateMessage: (String) -> Unit,
    val updateNewSymbol: (String) -> Unit,
    val confirmRemoteProvider: (Boolean) -> Unit,
    val inspectContext: () -> Unit,
    val send: () -> Unit,
    val cancel: () -> Unit,
    val updateMessageValue: (TextFieldValue) -> Unit = { value -> updateMessage(value.text) },
)

/** Editable declaration intents, separate from the chat conversation. */
internal data class DraftEditorActions(
    val updateDeclaration: (String) -> Unit,
    val updateImports: (List<String>) -> Unit,
    val validate: () -> Unit,
    val updateDeclarationValue: (TextFieldValue) -> Unit = { value ->
      updateDeclaration(value.text)
    },
)

internal fun requiredImportsVisible(editor: EditableDraftState): Boolean =
    editor.imports.isNotEmpty()

internal fun parseRequiredImports(value: String): List<String> =
    value.split(',').map { it.trim() }.filter { it.isNotBlank() }

internal fun draftEditorStatusMessage(status: DraftEditorStatus): String =
    when (status) {
      DraftEditorStatus.Generated -> "Validate before review."
      DraftEditorStatus.Dirty -> "Edits need validation and focused checks."
      DraftEditorStatus.Validating -> "Validating."
      DraftEditorStatus.Valid -> "Validated. Run focused checks before review."
      DraftEditorStatus.Invalid -> "Fix validation diagnostics before continuing."
      DraftEditorStatus.Stale -> "Draft is stale. Start a new conversation."
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
