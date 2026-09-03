package io.miniorca.desktop

enum class ChatEditMode(val wireValue: String, val label: String) {
    ReplaceSymbol("replace_symbol", "Replace selected declaration"),
    CreateSymbol("create_symbol", "Create new function/type"),
}

data class ChatTarget(val mode: ChatEditMode, val symbol: String)

data class ChatTargetValidation(val target: ChatTarget? = null, val message: String = "") {
    val valid: Boolean get() = target != null
}

private val goIdentifier = Regex("[A-Za-z_][A-Za-z0-9_]*")

fun validateChatTarget(
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    mode: ChatEditMode,
    requestedSymbol: String,
): ChatTargetValidation {
    if (selectedFile == null) return ChatTargetValidation(message = "Open one Go file before starting a conversation.")
    if (!selectedFile.language.equals("Go", ignoreCase = true)) return ChatTargetValidation(message = "File-scoped declaration chat currently supports Go files only.")
    return when (mode) {
        ChatEditMode.ReplaceSymbol -> {
            val symbol = selectedSymbol
            if (symbol == null || symbol !in symbols || symbol.confidence.lowercase() != "exact" || !symbol.atomicTarget || symbol.kind.lowercase() !in setOf("function", "method", "type", "struct", "interface", "var")) {
                ChatTargetValidation(message = "Replace requires an exact selected Go function, method, type, or single top-level variable.")
            } else ChatTargetValidation(ChatTarget(mode, symbol.name))
        }
        ChatEditMode.CreateSymbol -> {
            val name = requestedSymbol.trim()
            when {
                !goIdentifier.matches(name) -> ChatTargetValidation(message = "Enter a valid new Go function or type name.")
                symbols.any { it.name == name } -> ChatTargetValidation(message = "${name} already exists in this file; select it to replace instead.")
                else -> ChatTargetValidation(ChatTarget(mode, name))
            }
        }
    }
}

fun chatSessionMatches(session: ChatSession?, file: ProjectFileInfo?, project: ProjectAnalysis?, target: ChatTarget, taskSpec: BugTaskSpec? = null): Boolean =
    session != null && file != null && project != null &&
        session.projectId == project.projectId && session.projectRevision == project.projectRevision &&
        session.baseFileHash == file.contentHash && session.openPath == file.path &&
        session.mode == target.mode.wireValue && session.targetSymbol == target.symbol &&
        sameTaskSpec(session.taskSpec, taskSpec) &&
        session.state.lowercase() == "active"

fun chatDraftMatchesSession(draft: DeclarationDraft?, session: ChatSession?): Boolean =
    draft != null && session != null &&
        draft.projectId == session.projectId && draft.projectRevision == session.projectRevision &&
        draft.baseFileHash == session.baseFileHash && draft.targetPath == session.openPath &&
        draft.mode == session.mode && draft.targetSymbol == session.targetSymbol && sameTaskSpec(draft.taskSpec, session.taskSpec)

internal fun sameTaskSpec(left: BugTaskSpec?, right: BugTaskSpec?): Boolean =
    left == right
