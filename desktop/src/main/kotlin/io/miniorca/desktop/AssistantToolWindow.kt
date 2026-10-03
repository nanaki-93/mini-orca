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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp

@Composable
private fun CreationNameControls(
    state: AssistantToolWindowState,
    actions: AssistantConversationActions,
    blocked: String?,
) {
  Text(
      "Declaration kind",
      color = SecondaryText,
      style = IdeTypography.resultLabel,
      modifier = Modifier.padding(top = 9.dp, bottom = 5.dp))
  Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
    DeclarationCreationKind.entries.forEach { kind ->
      val active = state.creationKind == kind
      ChromeButton(
          onClick = { actions.changeCreationKind(kind) },
          enabled = blocked == null,
          selected = active,
          role = Role.RadioButton,
          accessibleName = "New Go ${kind.noun}",
          tooltip = null,
          modifier =
              Modifier.weight(1f).semantics {
                selected = active
                stateDescription = if (active) "Selected" else "Not selected"
              }) {
            Text(
                "${if (active) "Selected" else "Select"} · ${kind.noun.replaceFirstChar { it.uppercase() }}")
          }
    }
  }
  if (blocked != null) {
    Text(
        blocked,
        color = Warning,
        style = IdeTypography.compactBody,
        modifier = Modifier.padding(top = 5.dp))
  }
  CompactSingleLineField(
      state.newSymbol,
      actions.updateNewSymbol,
      label = "New ${state.creationKind.noun} name",
      enabled = blocked == null,
      modifier =
          Modifier.fillMaxWidth()
              .padding(top = 9.dp)
              .then(state.creationNameFocus?.let { Modifier.focusRequester(it) } ?: Modifier))
  if (state.targetValidation.message.isNotBlank()) {
    Text(
        state.targetValidation.message,
        color = if (state.targetValidation.valid) SecondaryText else Warning,
        style = IdeTypography.compactBody,
        modifier = Modifier.padding(top = 5.dp).testTag("assistant-name-feedback"))
  }
}

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
              state.selected,
              state.sending ||
                  state.validating ||
                  state.editor?.status == DraftEditorStatus.Validating)
      else null
  val history = assistantHistoryEntries(state)
  val blockedReason = assistantComposerBlockedReason(state)
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
    Column(
        Modifier.weight(1.3f)
            .verticalScroll(rememberScrollState())
            .testTag("assistant-composer-scroll")) {
          ToolWindowScopeHeader(
              "ASSISTANT",
              assistantToolWindowScope(
                  state.selected,
                  state.target,
                  null,
                  state.newSymbol,
                  state.mode,
                  state.creationKind),
              Modifier)
          IdePaneHeader(
              title = if (creating) "New ${state.creationKind.noun}" else "Edit declaration",
              icon = DesktopIcon.Editor)
          Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
            if (!creating) {
              Text(
                  state.target?.let { "${it.mode.label} · ${it.symbol}" }
                      ?: state.targetValidation.message.ifBlank {
                        "Select a declaration or enter a new name."
                      },
                  color = if (state.target == null) Warning else SecondaryText,
                  style = IdeTypography.compactBody,
                  modifier = Modifier.padding(top = 5.dp))
            }
            if (creating) CreationNameControls(state, conversationActions, creationBlocked)
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
                  enabled =
                      if (creating) creationBlocked == null
                      else !state.sending && state.target != null,
                  placeholder = "Optional compatibility, allocation, or error-handling limits",
                  minLines = 2,
                  modifier = Modifier.fillMaxWidth().padding(top = 5.dp))
            }
          }
          IdeHorizontalSeparator()
          Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 8.dp)) {
            MiniOrcaButton(
                onClick = conversationActions.inspectContext,
                enabled = !state.sending,
                tone = ActionTone.Neutral,
                modifier =
                    Modifier.fillMaxWidth()
                        .padding(top = 8.dp)
                        .then(
                            state.inspectContextFocus?.let { Modifier.focusRequester(it) }
                                ?: Modifier)) {
                  Text("Inspect context")
                }
            val model = state.functionModel
            if (model.remoteProvider || functionDestinationMetadataComplete(model)) {
              RemoteProviderConfirmation(
                  ModelScope.Function,
                  model.copy(
                      profile = model.profile.ifBlank { "profile unavailable" },
                      model = model.model.ifBlank { "model unavailable" }),
                  state.remoteConfirmed,
                  conversationActions.confirmRemoteProvider)
            } else {
              Text(
                  functionDestinationLabel(model),
                  color = SecondaryText,
                  style = IdeTypography.compactBody)
            }
            Text(
                "Function provider origin: ${model.providerOrigin.takeIf { it.isNotBlank() }?.let { sanitizedOutputText(it, 256) } ?: "unavailable"}",
                color = SecondaryText,
                style = IdeTypography.compactBody)
            if (state.sending) IdeLabelBadge("Generating", Information, icon = DesktopIcon.Refresh)
            else
                blockedReason?.let {
                  Text(
                      it,
                      color = Warning,
                      style = IdeTypography.compactBody,
                      modifier = Modifier.padding(top = 6.dp))
                }
            if (state.sending)
                MiniOrcaButton(
                    onClick = conversationActions.cancel,
                    tone = ActionTone.Destructive,
                    modifier =
                        Modifier.fillMaxWidth()
                            .padding(top = 7.dp)
                            .testTag("assistant-request-action")) {
                      Text("Cancel request")
                    }
            else
                MiniOrcaButton(
                    onClick = conversationActions.send,
                    enabled = blockedReason == null,
                    tone = if (draftVisible) ActionTone.Neutral else ActionTone.Primary,
                    modifier =
                        Modifier.fillMaxWidth()
                            .padding(top = 7.dp)
                            .testTag("assistant-request-action")) {
                      Text(if (creating) "Generate ${state.creationKind.noun}" else "Send message")
                    }
          }
        }
    IdeHorizontalSeparator()
    Column(
        Modifier.weight(1f)
            .verticalScroll(rememberScrollState())
            .testTag("assistant-history-scroll")
            .padding(bottom = 8.dp)) {
          if (draftVisible) {
            AssistantDraftEditorSection(
                state.editor,
                state.draftInput ?: TextFieldValue(state.editor.declaration),
                state.importInput ?: TextFieldValue(state.editor.imports.joinToString(", ")),
                state.draftFocus,
                state.discardFocus,
                state.draft.engineeringInsight,
                state.draft.state.equals("stale", ignoreCase = true),
                draftCreationKind(state.draft, state.attempts),
                editorActions)
          }
          IdePaneHeader(
              title = "Conversation",
              stateLabel = if (history.isEmpty()) "No requests yet" else "Requests and responses")
          Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
            history.forEach { entry ->
              when (entry) {
                is AssistantHistoryEntry.Turn -> {
                  Text(
                      assistantMessageLabel(entry.message.role),
                      color =
                          if (entry.message.role.equals("assistant", ignoreCase = true))
                              ResultAccent
                          else SelectionText,
                      style = IdeTypography.resultHeading,
                      modifier = Modifier.padding(top = 12.dp, bottom = 4.dp))
                  entry.scopeLabel?.let {
                    Text(it, color = Warning, style = IdeTypography.compactBody)
                  }
                  if (entry.message.role.equals("assistant", ignoreCase = true)) {
                    ModelResultContent(entry.message.content)
                  } else {
                    SelectionContainer {
                      Text(entry.message.content, color = PrimaryText, style = IdeTypography.body)
                    }
                  }
                }
                is AssistantHistoryEntry.Attempt -> {
                  Text(
                      when (entry.attempt.outcome) {
                        ChatRequestOutcome.Running -> "Request running"
                        is ChatRequestOutcome.Failed -> "Request failed"
                        ChatRequestOutcome.Canceled -> "Request canceled"
                        is ChatRequestOutcome.Succeeded -> "Request completed"
                      },
                      color =
                          if (entry.attempt.outcome is ChatRequestOutcome.Failed) Error
                          else SecondaryText,
                      style = IdeTypography.resultHeading,
                      modifier = Modifier.padding(top = 12.dp, bottom = 4.dp))
                  entry.scopeLabel?.let {
                    Text(it, color = Warning, style = IdeTypography.compactBody)
                  }
                  SelectionContainer {
                    Text(entry.attempt.requestText, color = PrimaryText, style = IdeTypography.body)
                  }
                  when (val outcome = entry.attempt.outcome) {
                    is ChatRequestOutcome.Failed -> DiagnosticText(outcome.message, color = Error)
                    ChatRequestOutcome.Canceled ->
                        entry.attempt.invalidationReason?.let { DiagnosticText(it) }
                    else -> Unit
                  }
                }
              }
            }
          }
        }
  }
}

private fun functionDestinationMetadataComplete(model: ScopedModel): Boolean =
    model.profile.isNotBlank() && model.model.isNotBlank() && model.providerOrigin.isNotBlank()

internal fun functionDestinationLabel(model: ScopedModel): String =
    if (functionDestinationMetadataComplete(model) || model.remoteProvider)
        modelDestinationLabel(
            ModelScope.Function,
            model.copy(
                profile = model.profile.ifBlank { "profile unavailable" },
                model = model.model.ifBlank { "model unavailable" }))
    else
        "${ModelScope.Function.label}: ${model.profile.ifBlank { "profile unavailable" }} · " +
            "${model.model.ifBlank { "model unavailable" }} · provider locality unavailable"

internal fun assistantComposerBlockedReason(state: AssistantToolWindowState): String? =
    assistantComposerBlockedReason(
        state.mode,
        state.selected,
        state.targetValidation,
        state.message,
        state.sending,
        state.validating || state.editor?.status == DraftEditorStatus.Validating,
        state.functionModel,
        state.remoteConfirmed)

@Composable
private fun AssistantDraftEditorSection(
    editor: EditableDraftState,
    declaration: TextFieldValue,
    importText: TextFieldValue,
    draftFocus: FocusRequester,
    discardFocus: FocusRequester?,
    insight: EngineeringInsight?,
    stale: Boolean,
    creationKind: DeclarationCreationKind?,
    actions: DraftEditorActions
) {
  Column(Modifier.fillMaxWidth()) {
    val draft = editor.serverDraft
    IdePaneHeader(
        title = "Editable draft",
        icon = DesktopIcon.Document,
        actionsBelow = true,
        stateTint = ResultAccent,
        actions = {
          IdeLabelBadge(
              draftEditorStatusLabel(editor.status), draftEditorStatusColor(editor.status))
        })
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
      SelectionContainer {
        Text(
            draftIdentityLabel(draft, creationKind),
            color = SecondaryText,
            fontFamily = FontFamily.Monospace,
            style = IdeTypography.resultCode)
      }
      CompactMultilineField(
          value = declaration,
          onValueChange = actions.updateDeclarationValue,
          enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
          label = "Declaration only",
          minLines = 5,
          textStyle = IdeTypography.resultCode,
          modifier = Modifier.fillMaxWidth().padding(top = 7.dp).focusRequester(draftFocus))
      CompactSingleLineField(
          importText,
          actions.updateImportValue,
          enabled = editor.status !in setOf(DraftEditorStatus.Validating, DraftEditorStatus.Stale),
          label = "Required imports",
          modifier = Modifier.fillMaxWidth().padding(top = 7.dp))
      DraftValidationStage(editor, actions)
      MiniOrcaButton(
          onClick = actions.discard,
          tone = ActionTone.Destructive,
          modifier =
              Modifier.fillMaxWidth()
                  .padding(top = 5.dp)
                  .then(discardFocus?.let { Modifier.focusRequester(it) } ?: Modifier)) {
            Text("Discard draft…")
          }
      EngineeringInsightPanel(insight, stale = stale, scopeLabel = "Current proposal")
    }
    IdeHorizontalSeparator()
  }
}

@Composable
internal fun DraftValidationDiagnostics(
    diagnostics: List<DeclarationFinding>,
    retained: Boolean = false,
) {
  if (diagnostics.isEmpty()) return
  IdePaneHeader(
      title = if (retained) "Previous validation diagnostics" else "Validation diagnostics",
      stateLabel =
          if (retained) "Retained; revalidate for current approval"
          else "${diagnostics.size} require attention",
      stateTint = if (retained) Warning else Error)
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

internal sealed interface AssistantHistoryEntry {
  data class Turn(val message: ChatSessionMessage, val scopeLabel: String? = null) :
      AssistantHistoryEntry

  data class Attempt(val attempt: ChatRequestAttempt, val scopeLabel: String? = null) :
      AssistantHistoryEntry
}

/** The daemon owns successful turns; attempts only supply positions and local outcomes. */
internal fun assistantHistoryEntries(state: AssistantToolWindowState): List<AssistantHistoryEntry> {
  val project = state.project ?: return emptyList()
  val file = state.selected ?: return emptyList()
  val session =
      state.session?.takeIf { it.projectId == project.projectId && it.openPath == file.path }
  val turns = session?.messages.orEmpty()
  val entries = mutableListOf<AssistantHistoryEntry>()
  var cursor = 0
  val attempts =
      state.attempts.filter {
        it.scope.projectId == project.projectId && it.scope.path == file.path
      }
  for ((position, attempt) in attempts.withIndex()) {
    val scopeLabel =
        if (attempt.scope.projectRevision == project.projectRevision &&
            attempt.scope.baseFileHash == file.contentHash &&
            attempt.scope.target == state.target &&
            (attempt.scope.target.mode != ChatEditMode.ReplaceSymbol ||
                attempt.scope.declaration == state.selectedSymbol) &&
            sameTaskSpec(attempt.scope.taskSpec, state.taskSpec))
            null
        else
            "Earlier scope: ${attempt.scope.path} · ${attempt.scope.target.symbol} " +
                "(${attempt.scope.target.mode.label}; project ${attempt.scope.projectId}, " +
                "revision ${attempt.scope.projectRevision}, file hash ${attempt.scope.baseFileHash})"
    val outcome = attempt.outcome
    if (outcome is ChatRequestOutcome.Succeeded &&
        session != null &&
        outcome.sessionId == session.id &&
        attempt.scope.matches(session)) {
      val match = matchingSuccessfulTurn(turns, cursor, attempt)
      if (match != null) {
        val responseIndex = match.second
        for (index in cursor..responseIndex) {
          entries +=
              AssistantHistoryEntry.Turn(turns[index], scopeLabel ?: sessionScopeLabel(state))
        }
        cursor = responseIndex + 1
        continue
      }
    }
    if (outcome !is ChatRequestOutcome.Succeeded) {
      // Turns predating an unrecorded session load belong before this submitted request.
      val nextTurn =
          attempts.drop(position + 1).firstNotNullOfOrNull { later ->
            if (later.outcome is ChatRequestOutcome.Succeeded &&
                session != null &&
                later.outcome.sessionId == session.id &&
                later.scope.matches(session))
                matchingSuccessfulTurn(turns, cursor, later)?.first
            else null
          } ?: turns.size
      for (index in cursor until nextTurn) {
        entries += AssistantHistoryEntry.Turn(turns[index], sessionScopeLabel(state))
      }
      cursor = nextTurn
      entries += AssistantHistoryEntry.Attempt(attempt, scopeLabel)
    }
  }
  for (index in cursor until turns.size) {
    entries += AssistantHistoryEntry.Turn(turns[index], sessionScopeLabel(state))
  }
  return entries
}

private fun matchingSuccessfulTurn(
    turns: List<ChatSessionMessage>,
    cursor: Int,
    attempt: ChatRequestAttempt,
): Pair<Int, Int>? {
  val outcome = attempt.outcome as? ChatRequestOutcome.Succeeded ?: return null
  val responseIndex =
      (cursor until turns.size).firstOrNull {
        turns[it].role.equals("assistant", ignoreCase = true) &&
            turns[it].draftId == outcome.draftId
      } ?: return null
  val requestIndex =
      (cursor until responseIndex).lastOrNull {
        turns[it].role.equals("user", ignoreCase = true) && turns[it].content == attempt.requestText
      } ?: return null
  return requestIndex to responseIndex
}

private fun sessionScopeLabel(state: AssistantToolWindowState): String? {
  val session = state.session ?: return null
  if (state.target != null &&
      chatSessionMatches(session, state.selected, state.project, state.target, state.taskSpec))
      return null
  return "Earlier scope: ${session.openPath} · ${session.targetSymbol} " +
      "(${session.mode}; project ${session.projectId}, revision ${session.projectRevision}, " +
      "file hash ${session.baseFileHash})"
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
    val discardFocus: FocusRequester? = null,
    val messageInput: TextFieldValue = TextFieldValue(message),
    val draftInput: TextFieldValue? = null,
    val importInput: TextFieldValue? = null,
    val selectedSymbol: SymbolInfo? = null,
    val targetValidation: ChatTargetValidation = ChatTargetValidation(target),
    val advancedConstraintsInput: TextFieldValue = TextFieldValue(),
    val creationKind: DeclarationCreationKind = DeclarationCreationKind.Function,
    val creationNameFocus: FocusRequester? = null,
    val attempts: List<ChatRequestAttempt> = emptyList(),
    val taskSpec: BugTaskSpec? = null,
    val inspectContextFocus: FocusRequester? = null,
    val validating: Boolean = false,
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
    val changeCreationKind: (DeclarationCreationKind) -> Unit,
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
    val updateImportValue: (TextFieldValue) -> Unit,
    val validate: () -> Unit,
    val discard: () -> Unit,
    val cancelValidation: (() -> Unit)? = null,
    val review: (() -> Unit)? = null,
    val updateDeclarationValue: (TextFieldValue) -> Unit = { value ->
      updateDeclaration(value.text)
    },
)

internal fun parseRequiredImports(value: String): List<String> =
    value.split(',').map { it.trim() }.filter { it.isNotBlank() }

internal fun draftCreationKind(
    draft: DeclarationDraft,
    attempts: List<ChatRequestAttempt>,
): DeclarationCreationKind? =
    attempts
        .lastOrNull { attempt ->
          val outcome = attempt.outcome as? ChatRequestOutcome.Succeeded
          outcome?.draftId == draft.id &&
              attempt.scope.projectId == draft.projectId &&
              attempt.scope.projectRevision == draft.projectRevision &&
              attempt.scope.path == draft.targetPath &&
              attempt.scope.baseFileHash == draft.baseFileHash &&
              attempt.scope.target.mode.wireValue == draft.mode &&
              attempt.scope.target.symbol == draft.targetSymbol
        }
        ?.creationKind
        ?.let { noun -> DeclarationCreationKind.entries.find { it.noun == noun } }

internal fun draftIdentityLabel(
    draft: DeclarationDraft,
    creationKind: DeclarationCreationKind?,
): String {
  val scope =
      when (draft.mode) {
        ChatEditMode.ReplaceSymbol.wireValue -> "Replace declaration"
        ChatEditMode.CreateSymbol.wireValue ->
            creationKind?.let { "New ${it.noun}" } ?: "New declaration (kind unavailable)"
        else -> "Draft scope unavailable"
      }
  return "$scope · ${draft.targetSymbol} · ${draft.targetPath} · Server draft revision ${draft.revision}"
}

internal fun draftDiagnosticsAreEarlierEvidence(editor: EditableDraftState): Boolean =
    editor.diagnosticsAreRetained ||
        editor.status in
            setOf(DraftEditorStatus.Dirty, DraftEditorStatus.Validating, DraftEditorStatus.Stale)

internal fun draftEditorStatusLabel(status: DraftEditorStatus): String =
    when (status) {
      DraftEditorStatus.Generated -> "Not validated"
      DraftEditorStatus.Dirty -> "Edited · not validated"
      DraftEditorStatus.Validating -> "Validating"
      DraftEditorStatus.Valid -> "Validated"
      DraftEditorStatus.Invalid -> "Invalid"
      DraftEditorStatus.Stale -> "Stale"
    }

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
