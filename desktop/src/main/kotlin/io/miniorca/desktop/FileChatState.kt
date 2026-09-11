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
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    mode: ChatEditMode,
    requestedSymbol: String,
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
        symbols.any { it.name == name } ->
            ChatTargetValidation(
                message = "${name} already exists in this file; select it to replace instead.")
        else -> ChatTargetValidation(ChatTarget(mode, name))
      }
    }
  }
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
