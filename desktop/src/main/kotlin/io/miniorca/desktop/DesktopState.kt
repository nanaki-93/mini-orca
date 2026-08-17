package io.miniorca.desktop

data class DesktopState(
    val project: ProjectAnalysis? = null,
    val index: ProjectIndex? = null,
    val selectedFile: ProjectFileInfo? = null,
    val symbols: List<SymbolInfo> = emptyList(),
    val selectedSymbol: SymbolInfo? = null,
    val preparedAction: String = "",
    val preparedRequest: String = "",
    val analysis: FileAnalysis? = null,
    val candidate: GenerationResult? = null,
    val comparisonBase: GenerationResult? = null,
    val comparison: CandidateComparison? = null,
    val checks: CandidateCheckReport? = null,
    val impact: ImpactPreview? = null,
    val gitStatus: GitStatus? = null,
    val status: String = "Daemon ready",
    val error: String? = null,
    val loading: Boolean = false,
)

sealed interface DesktopEvent {
    data object Loading : DesktopEvent
    data class ProjectLoaded(val project: ProjectAnalysis, val index: ProjectIndex) : DesktopEvent
    data class IndexRefreshed(val index: ProjectIndex) : DesktopEvent
    data class FileLoaded(val file: ProjectFileInfo, val symbols: List<SymbolInfo>, val impact: ImpactPreview? = null, val gitStatus: GitStatus? = null) : DesktopEvent
    data class SymbolSelected(val symbol: SymbolInfo) : DesktopEvent
    data class SuggestionPrepared(val action: String, val request: String, val symbol: SymbolInfo?) : DesktopEvent
    data class AnalysisLoaded(val analysis: FileAnalysis) : DesktopEvent
    data class CandidateLoaded(val candidate: GenerationResult) : DesktopEvent
    data class AlternateCandidateLoaded(val base: GenerationResult, val candidate: GenerationResult) : DesktopEvent
    data class ComparisonLoaded(val comparison: CandidateComparison) : DesktopEvent
    data object CandidateDiscarded : DesktopEvent
    data class ChecksLoaded(val checks: CandidateCheckReport) : DesktopEvent
    data class Failed(val message: String) : DesktopEvent
    data class Status(val message: String) : DesktopEvent
}

fun DesktopState.reduce(event: DesktopEvent): DesktopState = when (event) {
    DesktopEvent.Loading -> copy(loading = true, error = null)
    is DesktopEvent.ProjectLoaded -> copy(project = event.project, index = event.index, selectedFile = null, symbols = emptyList(), selectedSymbol = null, preparedAction = "", preparedRequest = "", analysis = null, candidate = null, comparisonBase = null, comparison = null, checks = null, loading = false, status = "Imported ${event.project.name}", error = null)
    is DesktopEvent.IndexRefreshed -> copy(project = project?.copy(projectRevision = event.index.projectRevision), index = event.index, loading = false, status = "Re-analyzed project index", error = null)
    is DesktopEvent.FileLoaded -> copy(selectedFile = event.file, symbols = event.symbols, selectedSymbol = null, preparedAction = "", preparedRequest = "", analysis = null, candidate = null, comparisonBase = null, comparison = null, checks = null, impact = event.impact, gitStatus = event.gitStatus, loading = false, status = event.file.path, error = null)
    is DesktopEvent.SymbolSelected -> copy(selectedSymbol = event.symbol, error = null)
    is DesktopEvent.SuggestionPrepared -> copy(selectedSymbol = event.symbol ?: selectedSymbol, preparedAction = event.action, preparedRequest = event.request, error = null)
    is DesktopEvent.AnalysisLoaded -> copy(analysis = event.analysis, loading = false, error = null)
    is DesktopEvent.CandidateLoaded -> copy(candidate = event.candidate, comparisonBase = null, comparison = null, checks = null, loading = false, error = null)
    is DesktopEvent.AlternateCandidateLoaded -> copy(candidate = event.candidate, comparisonBase = event.base, comparison = null, checks = null, loading = false, error = null)
    is DesktopEvent.ComparisonLoaded -> copy(comparison = event.comparison, loading = false, error = null)
    DesktopEvent.CandidateDiscarded -> copy(candidate = null, comparisonBase = null, comparison = null, checks = null, loading = false, status = "Discarded preview", error = null)
    is DesktopEvent.ChecksLoaded -> copy(checks = event.checks, loading = false, error = null)
    is DesktopEvent.Failed -> copy(loading = false, error = event.message)
    is DesktopEvent.Status -> copy(status = event.message)
}
