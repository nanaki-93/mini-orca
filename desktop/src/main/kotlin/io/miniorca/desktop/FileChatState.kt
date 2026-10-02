package io.miniorca.desktop

enum class ChatEditMode(val wireValue: String, val label: String) {
  ReplaceSymbol("replace_symbol", "Replace selected declaration"),
  CreateSymbol("create_symbol", "Create new function/type"),
}

data class ChatTarget(val mode: ChatEditMode, val symbol: String)

data class ChatTargetValidation(val target: ChatTarget? = null, val message: String = "") {
  val valid: Boolean
    get() = target != null
}

// Match Go's letter/digit categories rather than Java identifier rules (which allow '$').
private val goIdentifier = Regex("[\\p{L}_][\\p{L}\\p{Nd}_]*")
private val goKeywords =
    setOf(
        "break",
        "case",
        "chan",
        "const",
        "continue",
        "default",
        "defer",
        "else",
        "fallthrough",
        "for",
        "func",
        "go",
        "goto",
        "if",
        "import",
        "interface",
        "map",
        "package",
        "range",
        "return",
        "select",
        "struct",
        "switch",
        "type",
        "var")

fun validateChatTarget(
    selection: FileSelectionState,
    mode: ChatEditMode,
    requestedSymbol: String,
): ChatTargetValidation =
    validateChatTarget(
        selection.selectedFile,
        selection.symbols,
        selection.selectedSymbol,
        mode,
        requestedSymbol,
        snapshotLoaded =
            selection.pendingFilePath == null &&
                selection.failedFilePath != selection.selectedFile?.path &&
                (selection.fileReadError == null || selection.failedFilePath != null))

fun validateChatTarget(
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    mode: ChatEditMode,
    requestedSymbol: String,
): ChatTargetValidation =
    validateChatTarget(selectedFile, symbols, selectedSymbol, mode, requestedSymbol, true)

private fun validateChatTarget(
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    mode: ChatEditMode,
    requestedSymbol: String,
    snapshotLoaded: Boolean,
): ChatTargetValidation {
  if (selectedFile == null)
      return ChatTargetValidation(message = "Open one Go file before starting a conversation.")
  if (selectedFile.binary)
      return ChatTargetValidation(message = "Binary files cannot be used for declaration chat.")
  if (!selectedFile.language.equals("Go", ignoreCase = true))
      return ChatTargetValidation(
          message = "File-scoped declaration chat currently supports Go files only.")
  return when (mode) {
    ChatEditMode.ReplaceSymbol -> {
      val symbol = selectedSymbol
      when {
        symbol == null ->
            ChatTargetValidation(
                message = "Select one Go declaration in the editor before replacing it.")
        symbol !in symbols ->
            ChatTargetValidation(
                message = "The selected declaration is stale. Select it again in the editor.")
        symbol.confidence.lowercase() != "exact" ->
            ChatTargetValidation(message = "Select an exact Go declaration before replacing it.")
        !symbol.atomicTarget ->
            ChatTargetValidation(
                message =
                    "Select one declaration. Multi-function or grouped declaration changes are not supported.")
        symbol.kind.lowercase() !in
            setOf("function", "method", "type", "struct", "interface", "var") ->
            ChatTargetValidation(
                message =
                    "Select a Go function, method, type, or single top-level variable to replace.")
        else -> ChatTargetValidation(ChatTarget(mode, symbol.name))
      }
    }
    ChatEditMode.CreateSymbol -> {
      val name = requestedSymbol.trim()
      when {
        name.isEmpty() ->
            ChatTargetValidation(message = "Enter a name for the new Go function or type.")
        name in goKeywords ->
            ChatTargetValidation(message = "$name is a Go keyword. Choose a different name.")
        !goIdentifier.matches(name) ->
            ChatTargetValidation(message = "Enter a valid new Go function or type name.")
        name in setOf("_", "init", "main") ->
            ChatTargetValidation(
                message = "$name is reserved for this creation workflow. Choose another name.")
        !snapshotLoaded ->
            ChatTargetValidation(
                message =
                    "Wait for the current file and its symbols to load before creating a declaration. Reopen the file if loading failed.")
        symbols.any { it.name == name } ->
            ChatTargetValidation(
                message = "${name} already exists in this file; select it to replace instead.")
        else ->
            ChatTargetValidation(
                ChatTarget(mode, name),
                "Name absent in the current file snapshot; the daemon rechecks before generation.")
      }
    }
  }
}

internal fun assistantComposerBlockedReason(
    mode: ChatEditMode,
    selected: ProjectFileInfo?,
    validation: ChatTargetValidation,
    rawIntent: String,
    sending: Boolean,
    validating: Boolean,
    functionModel: ScopedModel,
    remoteConfirmed: Boolean,
): String? =
    when {
      sending -> "Request running. Cancel before sending another request."
      mode == ChatEditMode.CreateSymbol ->
          declarationCreationBlockedReason(selected, validating)
              ?: validation.message
                  .ifBlank { "Select a declaration or enter a new name." }
                  .takeIf { !validation.valid }
              ?: if (!hasFunctionChangeIntent(rawIntent))
                  "Enter a specific behavior before sending."
              else if (functionModel.remoteProvider && !remoteConfirmed)
                  "Confirm the Function remote destination before sending."
              else null
      !validation.valid ->
          validation.message.ifBlank { "Select one Go declaration before sending." }
      validating -> "Wait for the current draft validation to finish."
      !hasFunctionChangeIntent(rawIntent) -> "Enter a specific intent before sending."
      functionModel.remoteProvider && !remoteConfirmed ->
          "Confirm the Function remote destination before sending."
      else -> null
    }

fun functionChangePresetBoundary(
    mode: ChatEditMode,
    selectedSymbol: SymbolInfo?,
    targetValidation: ChatTargetValidation,
): String? {
  if (mode != ChatEditMode.ReplaceSymbol) return null
  if (!targetValidation.valid) return targetValidation.message
  if (selectedSymbol?.kind?.lowercase() !in setOf("function", "method"))
      return "Quick changes require one Go function or method. Select one to use a preset."
  return null
}

fun chatSessionMatches(
    session: ChatSession?,
    file: ProjectFileInfo?,
    project: ProjectAnalysis?,
    target: ChatTarget,
    taskSpec: BugTaskSpec? = null
): Boolean =
    session != null &&
        file != null &&
        project != null &&
        session.projectId == project.projectId &&
        session.projectRevision == project.projectRevision &&
        session.baseFileHash == file.contentHash &&
        session.openPath == file.path &&
        session.mode == target.mode.wireValue &&
        session.targetSymbol == target.symbol &&
        sameTaskSpec(session.taskSpec, taskSpec) &&
        session.state.lowercase() == "active"

fun chatDraftMatchesSession(draft: DeclarationDraft?, session: ChatSession?): Boolean =
    draft != null &&
        session != null &&
        draft.projectId == session.projectId &&
        draft.projectRevision == session.projectRevision &&
        draft.baseFileHash == session.baseFileHash &&
        draft.targetPath == session.openPath &&
        draft.mode == session.mode &&
        draft.targetSymbol == session.targetSymbol &&
        sameTaskSpec(draft.taskSpec, session.taskSpec)

internal fun sameTaskSpec(left: BugTaskSpec?, right: BugTaskSpec?): Boolean = left == right

internal fun repairTaskSpecMatches(session: BugTaskSpec, parent: BugTaskSpec): Boolean =
    session.copy(goTestCandidate = parent.goTestCandidate) == parent &&
        (session.goTestCandidate == null || session.goTestCandidate == parent.goTestCandidate)

internal fun sessionTaskSpecAfterProposal(
    current: BugTaskSpec?,
    proposed: BugTaskSpec?
): BugTaskSpec? =
    if (current != null && proposed != null && repairTaskSpecMatches(current, proposed)) proposed
    else current

internal const val MAX_DRAFT_REPAIRS = 3

internal fun repairUnavailableReason(
    session: ChatSession?,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?
): String? =
    when {
      session == null ||
          draft == null ||
          !session.state.equals("active", ignoreCase = true) ||
          !chatDraftMatchesSession(draft, session.copy(taskSpec = draft.taskSpec)) ||
          session.latestDraftId != draft.id ->
          "Repair needs the conversation bound to this candidate."
      session.taskSpec == null ||
          draft.taskSpec == null ||
          !repairTaskSpecMatches(session.taskSpec, draft.taskSpec) ->
          "This candidate has no pinned repair task. Edit the draft manually."
      session.repairCount >= MAX_DRAFT_REPAIRS ->
          "The repair limit is reached ($MAX_DRAFT_REPAIRS of $MAX_DRAFT_REPAIRS). Edit the draft manually."
      !checksMatchDraft(checks, draft) ->
          "Run focused checks for this candidate before requesting repair."
      checks!!.checks.any { it.state.lowercase() in setOf("canceled", "cancelled") } ->
          "Checks were canceled. Run them again before requesting repair."
      checks.checks.none { it.state.lowercase() in setOf("failed", "error") } ->
          "No failed check output is available for repair. Edit the draft or rerun checks."
      else -> null
    }
