package io.miniorca.desktop

enum class FunctionChangePreset(val label: String, private val requestLead: String) {
  Fix("Fix", "Fix a bug"),
  Refactor("Refactor", "Refactor without changing behavior"),
  Document("Document", "Document without changing behavior");

  fun preparedMessage(): String = "$requestLead: "
}

fun hasFunctionChangeIntent(rawIntent: String): Boolean {
  val intent = rawIntent.trim()
  return intent.isNotEmpty() &&
      FunctionChangePreset.entries.none { intent == it.preparedMessage().trim() }
}

fun functionChangeRequest(message: String, advancedConstraints: String): String {
  val intent = message.trim()
  val constraints = advancedConstraints.trim()
  return if (constraints.isEmpty()) intent else "$intent\n\nConstraints:\n$constraints"
}

data class DirectEditRequest(
    val target: ChatTarget,
    val selectedSymbol: SymbolInfo,
    val currentDraft: CurrentEditIdentity? = null,
) {
  val requiresDraftDiscard: Boolean =
      currentDraft?.hasDraft == true && !currentDraft.matchesTarget(target, selectedSymbol)

  val discardPrompt: String =
      "Discard draft for ${currentDraft?.targetSymbol} and edit ${target.symbol}?"
}

fun directEditRequest(
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    currentEditIdentity: CurrentEditIdentity?,
): DirectEditRequest? {
  val symbol = selectedSymbol ?: return null
  val eligibility = symbolEditEligibility(selectedFile, symbols, symbol)
  return eligibility.target?.let { target ->
    DirectEditRequest(target, symbol, currentEditIdentity)
  }
}

private fun CurrentEditIdentity.matchesTarget(target: ChatTarget, symbol: SymbolInfo): Boolean =
    mode == target.mode && targetSymbol == target.symbol && targetSymbol == symbol.name
