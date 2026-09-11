package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
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
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontFamily
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
  val creating = state.mode == ChatEditMode.CreateSymbol
  val creationBlocked =
      if (creating)
          declarationCreationBlockedReason(
              state.selected, state.sending || state.editor?.status == DraftEditorStatus.Validating)
      else null
  val bound =
      state.target != null &&
          chatSessionMatches(state.session, state.selected, state.project, state.target)
  val draftVisible =
      state.draft != null &&
          state.editor != null &&
          chatDraftMatchesSession(state.draft, state.session)
  val presetBoundary =
      functionChangePresetBoundary(state.mode, state.selectedSymbol, state.targetValidation)
  val presetsAvailable = state.mode == ChatEditMode.ReplaceSymbol && presetBoundary == null
  var constraintsExpanded by
      rememberSaveable(
          state.selected?.path,
          state.selected?.contentHash,
          state.mode,
          state.target?.symbol,
      ) {
        mutableStateOf(false)
      }
  Column(modifier) {
    ToolWindowScopeHeader(
        "ASSISTANT",
        assistantToolWindowScope(
            state.selected,
            state.target,
            state.draft,
            state.newSymbol,
            state.mode,
            state.creationKind),
        Modifier)
    Column(
        Modifier.weight(1f)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 8.dp)
            .padding(bottom = 8.dp)) {
          Column(Modifier.fillMaxWidth()) {
            IdePaneHeader(
                title = if (creating) "New ${state.creationKind.noun}" else "Conversation",
                icon = DesktopIcon.Editor,
                stateLabel =
                    when {
                      creating -> "Generate a candidate, then review before applying"
                      bound -> "Bound to the focused declaration"
                      state.target != null -> "Ready for the selected declaration"
                      else -> "Select a declaration"
                    })
            Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
              Text(
                  state.selected?.path ?: "Open a file to draft.",
                  color = PrimaryText,
                  fontFamily = FontFamily.Monospace,
                  fontSize = 12.sp)
              Text(
                  creationBlocked
                      ?: state.target?.let {
                        if (creating) "New ${state.creationKind.noun} · ${it.symbol}"
                        else "${it.mode.label} · ${it.symbol}"
                      }
                      ?: state.targetValidation.message.ifBlank {
                        "Select a declaration or enter a new name."
                      },
                  color = if (state.target == null) Warning else SecondaryText,
                  fontSize = 11.sp)
              if (bound && state.session != null) {
                state.session.messages.forEach { turn ->
                  Text(
                      assistantMessageLabel(turn.role),
                      color =
                          if (turn.role.equals("assistant", ignoreCase = true)) ResultAccent
                          else SelectionText,
                      style = IdeTypography.resultHeading,
                      modifier = Modifier.padding(top = 12.dp, bottom = 4.dp))
                  if (turn.role.equals("assistant", ignoreCase = true)) {
                    ModelResultContent(turn.content)
                  } else {
                    SelectionContainer {
                      Text(turn.content, color = PrimaryText, style = IdeTypography.body)
                    }
                  }
                }
              }
              if (creating) {
                CompactSingleLineField(
                    state.newSymbol,
                    conversationActions.updateNewSymbol,
                    label = "${state.creationKind.noun.replaceFirstChar { it.uppercase() }} name",
                    enabled = creationBlocked == null,
                    modifier =
                        Modifier.fillMaxWidth()
                            .padding(top = 9.dp)
                            .then(
                                state.creationNameFocus?.let { Modifier.focusRequester(it) }
                                    ?: Modifier))
              }
              if (presetsAvailable) {
                Text(
                    "Quick change",
                    color = SecondaryText,
                    style = IdeTypography.resultLabel,
                    modifier = Modifier.padding(top = 9.dp, bottom = 5.dp))
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                  FunctionChangePreset.entries.forEach { preset ->
                    MiniOrcaButton(
                        onClick = { conversationActions.preparePreset(preset) },
                        enabled = !state.sending,
                        density = ButtonDensity.Toolbar,
                        modifier = Modifier.weight(1f)) {
                          Text(preset.label)
                        }
                  }
                }
              } else if (presetBoundary != null && state.targetValidation.valid) {
                Text(
                    presetBoundary,
                    color = Warning,
                    style = IdeTypography.compactBody,
                    modifier = Modifier.padding(top = 9.dp))
              }
              CompactMultilineField(
                  value = state.messageInput,
                  onValueChange = conversationActions.updateMessageValue,
                  label = if (creating) "Behavior" else "Intent",
                  enabled =
                      if (creating) creationBlocked == null
                      else !state.sending && state.target != null,
                  placeholder =
                      if (!creating) "For example: preserve order while deduplicating"
                      else if (state.creationKind == DeclarationCreationKind.Type)
                          "Describe the type's fields and purpose"
                      else "Describe inputs, return values and expected behavior",
                  minLines = 3,
                  modifier =
                      Modifier.fillMaxWidth().padding(top = 9.dp).focusRequester(state.chatFocus))
              state.requestFailure
                  ?.takeIf { it.target == state.target }
                  ?.let { failure ->
                    Text("Request failed", color = Error, style = IdeTypography.resultHeading)
                    DiagnosticText(failure.message, color = Error)
                  }
              IdeDisclosureHeader(
                  title = "Advanced constraints",
                  expanded = constraintsExpanded,
                  onToggle = { constraintsExpanded = !constraintsExpanded },
                  stateLabel = if (constraintsExpanded) "Expanded" else "Collapsed",
                  modifier = Modifier.padding(top = 5.dp))
              if (constraintsExpanded) {
                CompactMultilineField(
                    value = state.advancedConstraintsInput,
                    onValueChange = conversationActions.updateAdvancedConstraintsValue,
                    label = "Constraints",
                    enabled = !state.sending && state.target != null,
                    placeholder = "Optional compatibility, allocation, or error-handling limits",
                    minLines = 2,
                    modifier = Modifier.fillMaxWidth().padding(top = 5.dp))
              }
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
                              (!creating || creationBlocked == null) &&
                              hasFunctionChangeIntent(state.message) &&
                              (!state.functionModel.remoteProvider || state.remoteConfirmed),
                      tone = if (draftVisible) ActionTone.Neutral else ActionTone.Primary,
                      modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
                        Text(
                            if (creating) "Generate ${state.creationKind.noun}" else "Send message")
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
        stateLabel = "Candidate for review",
        stateTint = ResultAccent,
        actions = { IdeLabelBadge(editor.status.name, draftEditorStatusColor(editor.status)) })
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      Text(
          "Target: ${draft.targetSymbol} in ${draft.targetPath}.",
          color = SecondaryText,
          fontFamily = FontFamily.Monospace,
          style = IdeTypography.resultCode)
      CompactMultilineField(
          value = declaration,
          onValueChange = actions.updateDeclarationValue,
          enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
          label = "Declaration only",
          minLines = 5,
          textStyle = IdeTypography.resultCode,
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
      DraftValidationDiagnostics(editor.diagnostics)
      Text(
          draftEditorStatusMessage(editor.status),
          color = draftEditorStatusColor(editor.status),
          style = IdeTypography.body,
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
internal fun DraftValidationDiagnostics(diagnostics: List<DeclarationFinding>) {
  if (diagnostics.isEmpty()) return
  IdePaneHeader(
      title = "Validation diagnostics",
      stateLabel = "${diagnostics.size} require attention",
      stateTint = Error)
  SelectionContainer {
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      diagnostics.forEach { diagnostic ->
        IdeLabelBadge(
            sanitizedOutputText(diagnostic.code, 128), Error, Modifier.padding(top = 4.dp))
        DiagnosticText(diagnostic.message, Modifier.padding(top = 4.dp, bottom = 8.dp))
      }
    }
  }
}

internal fun assistantMessageLabel(role: String): String =
    when (role.lowercase()) {
      "user" -> "Your request"
      "assistant" -> "Model response"
      "system" -> "System context"
      else -> role.ifBlank { "Message" }.replaceFirstChar { it.uppercase() }
    }

internal fun creationMessage(
    kind: DeclarationCreationKind,
    name: String,
    behavior: String
): String = "Create a Go ${kind.noun} named ${name.trim()}.\n\n$behavior"

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
    val selectedSymbol: SymbolInfo? = null,
    val targetValidation: ChatTargetValidation = ChatTargetValidation(target),
    val advancedConstraintsInput: TextFieldValue = TextFieldValue(),
    val creationKind: DeclarationCreationKind = DeclarationCreationKind.Function,
    val creationNameFocus: FocusRequester? = null,
    val requestFailure: ChatRequestFailure? = null,
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
    val preparePreset: (FunctionChangePreset) -> Unit = {},
    val updateAdvancedConstraintsValue: (TextFieldValue) -> Unit = {},
)

internal fun preparedFunctionChangeMessage(preset: FunctionChangePreset): TextFieldValue {
  val message = preset.preparedMessage()
  return TextFieldValue(message, TextRange(message.length))
}

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
