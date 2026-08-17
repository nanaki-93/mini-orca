package io.miniorca.desktop

data class SummaryState(
    val status: String,
    val emptySymbols: Boolean,
    val approximateSymbols: Boolean,
    val failure: String = "",
)

fun summaryState(analysis: FileAnalysis?, symbols: List<SymbolInfo>): SummaryState = SummaryState(
    status = analysis?.status ?: "missing",
    emptySymbols = symbols.isEmpty(),
    approximateSymbols = symbols.any { it.confidence != "exact" },
    failure = analysis?.failure.orEmpty(),
)
