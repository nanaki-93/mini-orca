package io.miniorca.desktop

data class DesktopState(
    val project: ProjectAnalysis? = null,
    val index: ProjectIndex? = null,
    val selectedFile: ProjectFileInfo? = null,
    val symbols: List<SymbolInfo> = emptyList(),
    val analysis: FileAnalysis? = null,
    val candidate: GenerationResult? = null,
    val checks: CandidateCheckReport? = null,
    val status: String = "Daemon ready",
    val error: String? = null,
    val loading: Boolean = false,
)

sealed interface DesktopEvent {
    data object Loading : DesktopEvent
    data class ProjectLoaded(val project: ProjectAnalysis, val index: ProjectIndex) : DesktopEvent
    data class FileLoaded(val file: ProjectFileInfo, val symbols: List<SymbolInfo>) : DesktopEvent
    data class AnalysisLoaded(val analysis: FileAnalysis) : DesktopEvent
    data class CandidateLoaded(val candidate: GenerationResult) : DesktopEvent
    data class ChecksLoaded(val checks: CandidateCheckReport) : DesktopEvent
    data class Failed(val message: String) : DesktopEvent
    data class Status(val message: String) : DesktopEvent
}

fun DesktopState.reduce(event: DesktopEvent): DesktopState = when (event) {
    DesktopEvent.Loading -> copy(loading = true, error = null)
    is DesktopEvent.ProjectLoaded -> copy(project = event.project, index = event.index, selectedFile = null, symbols = emptyList(), analysis = null, candidate = null, checks = null, loading = false, status = "Imported ${event.project.name}", error = null)
    is DesktopEvent.FileLoaded -> copy(selectedFile = event.file, symbols = event.symbols, analysis = null, candidate = null, checks = null, loading = false, status = event.file.path, error = null)
    is DesktopEvent.AnalysisLoaded -> copy(analysis = event.analysis, loading = false, error = null)
    is DesktopEvent.CandidateLoaded -> copy(candidate = event.candidate, checks = null, loading = false, error = null)
    is DesktopEvent.ChecksLoaded -> copy(checks = event.checks, loading = false, error = null)
    is DesktopEvent.Failed -> copy(loading = false, error = event.message)
    is DesktopEvent.Status -> copy(status = event.message)
}
