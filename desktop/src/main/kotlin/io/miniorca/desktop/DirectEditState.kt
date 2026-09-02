package io.miniorca.desktop

data class DirectEditRequest(
    val target: ChatTarget,
    val selectedSymbol: SymbolInfo,
    val currentDraft: CurrentEditIdentity? = null,
) {
    val requiresDraftDiscard: Boolean = currentDraft?.hasDraft == true &&
        !currentDraft.matchesTarget(target, selectedSymbol)

    val discardPrompt: String = "Discard draft for ${currentDraft?.targetSymbol} and edit ${target.symbol}?"
}

fun directEditRequest(
    selectedFile: ProjectFileInfo?,
    symbols: List<SymbolInfo>,
    selectedSymbol: SymbolInfo?,
    currentEditIdentity: CurrentEditIdentity?,
): DirectEditRequest? {
    val symbol = selectedSymbol ?: return null
    val eligibility = symbolEditEligibility(selectedFile, symbols, symbol)
    return eligibility.target?.let { target -> DirectEditRequest(target, symbol, currentEditIdentity) }
}

private fun CurrentEditIdentity.matchesTarget(target: ChatTarget, symbol: SymbolInfo): Boolean =
    mode == target.mode && targetSymbol == target.symbol && targetSymbol == symbol.name
