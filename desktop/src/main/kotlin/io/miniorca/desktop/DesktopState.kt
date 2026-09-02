package io.miniorca.desktop

/** The four stable product workspaces. Rendering their navigation is Task 45. */
enum class Workspace { Summary, Analysis, Bugs, Editor }

data class ProjectWorkspaceState(
    val project: ProjectAnalysis? = null,
    val index: ProjectIndex? = null,
    val overview: ProjectOverview? = null,
)

data class FileSelectionState(
    val selectedFile: ProjectFileInfo? = null,
    val symbols: List<SymbolInfo> = emptyList(),
    val selectedSymbol: SymbolInfo? = null,
    val focusedLine: Int = 0,
    val preparedAction: String = "",
    val preparedRequest: String = "",
    val analysis: FileAnalysis? = null,
    val impact: ImpactPreview? = null,
    val gitStatus: GitStatus? = null,
)

data class JobState(
    val loading: Boolean = false,
    val status: String = "Daemon ready",
    val error: String? = null,
)

data class FindingsState(
    val findings: List<UnifiedFinding> = emptyList(),
    val scan: GoScanReport? = null,
    val analyzeAll: AnalyzeAllJob? = null,
)

data class EditorNavigationTarget(
    val path: String,
    val symbol: String = "",
    val line: Int = 0,
)

fun nextWorkspace(workspace: Workspace): Workspace = Workspace.entries[(workspace.ordinal + 1) % Workspace.entries.size]

/** Rejects stale or external paths before a finding can open the Editor. */
fun findingNavigationTarget(finding: UnifiedFinding, index: ProjectIndex?): EditorNavigationTarget? {
    val path = finding.location.path.trim()
    if (path.isBlank() || index?.files?.none { it.path == path } != false) return null
    return EditorNavigationTarget(path, finding.location.symbol, finding.location.startLine)
}

fun symbolForNavigation(symbols: List<SymbolInfo>, target: EditorNavigationTarget): SymbolInfo? =
    symbols.firstOrNull { target.symbol.isNotBlank() && it.name == target.symbol }
        ?: symbols.firstOrNull { target.line > 0 && target.line in it.startLine..it.endLine }

data class ChatState(
    val session: ChatSession? = null,
    val pendingRequestId: Long = 0,
)

data class DraftReviewState(
    val checks: CandidateCheckReport? = null,
    val draft: DeclarationDraft? = null,
    val editor: EditableDraftState? = null,
    val applied: ApplyResult? = null,
)

data class ConnectionState(
    val label: String = "Connecting",
    val model: String = "",
    val version: String = "",
    val connected: Boolean = false,
    val locality: String = "",
    val latency: String = "",
)

/**
 * State is separated by ownership so a file-bound result cannot accidentally
 * overwrite project, job, chat, or review state from another workflow.
 */
data class DesktopState(
    val workspace: Workspace = Workspace.Summary,
    val projectState: ProjectWorkspaceState = ProjectWorkspaceState(),
    val selection: FileSelectionState = FileSelectionState(),
    val jobs: JobState = JobState(),
    val findings: FindingsState = FindingsState(),
    val chat: ChatState = ChatState(),
    val review: DraftReviewState = DraftReviewState(),
    val connection: ConnectionState = ConnectionState(),
) {
    val project get() = projectState.project
    val index get() = projectState.index
    val overview get() = projectState.overview
    val selectedFile get() = selection.selectedFile
    val symbols get() = selection.symbols
    val selectedSymbol get() = selection.selectedSymbol
    val preparedAction get() = selection.preparedAction
    val preparedRequest get() = selection.preparedRequest
    val analysis get() = selection.analysis
    val impact get() = selection.impact
    val gitStatus get() = selection.gitStatus
    val checks get() = review.checks
    val loading get() = jobs.loading
    val status get() = jobs.status
    val error get() = jobs.error
}

sealed interface DesktopEvent {
    data object Loading : DesktopEvent
    data class WorkspaceSelected(val workspace: Workspace) : DesktopEvent
    data class ConnectionUpdated(val connection: ConnectionState) : DesktopEvent
    data class ProjectLoaded(val project: ProjectAnalysis, val index: ProjectIndex) : DesktopEvent
    data class IndexRefreshed(val index: ProjectIndex) : DesktopEvent
    data class OverviewLoaded(val overview: ProjectOverview) : DesktopEvent
    data class FindingsLoaded(val findings: List<UnifiedFinding>) : DesktopEvent
    data class FindingStatusUpdated(val findingId: String, val status: String) : DesktopEvent
    data class AnalyzeAllLoaded(val job: AnalyzeAllJob?) : DesktopEvent
    data class GoScanLoaded(val scan: GoScanReport?) : DesktopEvent
    data class FileLoaded(val file: ProjectFileInfo, val symbols: List<SymbolInfo>) : DesktopEvent
    data class SymbolSelected(val symbol: SymbolInfo) : DesktopEvent
    data class EditorContextSelected(val symbol: SymbolInfo?, val line: Int) : DesktopEvent
    data class SourceLineSelected(val selection: SourceLineSelection) : DesktopEvent
    data class SuggestionPrepared(val action: String, val request: String, val symbol: SymbolInfo?) : DesktopEvent
    data class AnalysisLoaded(val analysis: FileAnalysis) : DesktopEvent
    data class ImpactLoaded(val impact: ImpactPreview) : DesktopEvent
    data class GitStatusLoaded(val gitStatus: GitStatus) : DesktopEvent
    data class ChecksLoaded(val checks: CandidateCheckReport) : DesktopEvent
    data class ChatLoaded(val session: ChatSession) : DesktopEvent
    data class ChatProposalLoaded(val session: ChatSession, val userMessage: String, val proposal: ChatDraftProposal) : DesktopEvent
    data class DraftEdited(val declaration: String? = null, val imports: List<String>? = null) : DesktopEvent
    data object DraftValidationStarted : DesktopEvent
    data object DraftMarkedStale : DesktopEvent
    data class DraftLoaded(val draft: DeclarationDraft) : DesktopEvent
    data object DraftDiscarded : DesktopEvent
    data class Applied(val result: ApplyResult?) : DesktopEvent
    data class Failed(val message: String) : DesktopEvent
    data class Status(val message: String) : DesktopEvent
}

fun DesktopState.reduce(event: DesktopEvent): DesktopState = when (event) {
    DesktopEvent.Loading -> copy(jobs = jobs.copy(loading = true, error = null))
    is DesktopEvent.WorkspaceSelected -> copy(workspace = event.workspace)
    is DesktopEvent.ConnectionUpdated -> copy(connection = event.connection)
    is DesktopEvent.ProjectLoaded -> copy(
        workspace = Workspace.Summary,
        projectState = ProjectWorkspaceState(event.project, event.index),
        selection = FileSelectionState(), findings = FindingsState(), chat = ChatState(), review = DraftReviewState(),
        jobs = jobs.copy(loading = false, status = "Imported ${event.project.name}", error = null),
    )
    is DesktopEvent.IndexRefreshed -> {
        val revisionChanged = project?.projectRevision != event.index.projectRevision
        copy(
            projectState = projectState.copy(project = project?.copy(projectRevision = event.index.projectRevision), index = event.index),
            chat = if (revisionChanged) ChatState() else chat,
            review = if (revisionChanged) DraftReviewState(applied = review.applied) else review,
            jobs = jobs.copy(loading = false, status = "Re-analyzed project index", error = null),
        )
    }
    is DesktopEvent.OverviewLoaded -> copy(projectState = projectState.copy(overview = event.overview))
    is DesktopEvent.FindingsLoaded -> copy(findings = findings.copy(findings = event.findings))
    is DesktopEvent.FindingStatusUpdated -> copy(findings = findings.copy(findings = findings.findings.map { finding ->
        if (finding.id == event.findingId) finding.copy(status = event.status) else finding
    }))
    is DesktopEvent.AnalyzeAllLoaded -> copy(findings = findings.copy(analyzeAll = event.job))
    is DesktopEvent.GoScanLoaded -> copy(findings = findings.copy(scan = event.scan))
    is DesktopEvent.FileLoaded -> copy(
        selection = FileSelectionState(selectedFile = event.file, symbols = event.symbols),
        chat = ChatState(), review = DraftReviewState(applied = review.applied),
        jobs = jobs.copy(loading = false, status = event.file.path, error = null),
    )
    is DesktopEvent.SymbolSelected -> copy(selection = selection.copy(selectedSymbol = event.symbol, focusedLine = event.symbol.startLine), jobs = jobs.copy(error = null))
    is DesktopEvent.EditorContextSelected -> copy(selection = selection.copy(selectedSymbol = event.symbol, focusedLine = event.line), jobs = jobs.copy(error = null))
    is DesktopEvent.SourceLineSelected -> copy(
        selection = selection.copy(selectedSymbol = event.selection.symbol, focusedLine = event.selection.line),
        jobs = jobs.copy(error = null),
    )
    is DesktopEvent.SuggestionPrepared -> copy(selection = selection.copy(selectedSymbol = event.symbol ?: selectedSymbol, preparedAction = event.action, preparedRequest = event.request), jobs = jobs.copy(error = null))
    is DesktopEvent.AnalysisLoaded -> copy(selection = selection.copy(analysis = event.analysis), jobs = jobs.copy(loading = false, error = null))
    is DesktopEvent.ImpactLoaded -> copy(selection = selection.copy(impact = event.impact))
    is DesktopEvent.GitStatusLoaded -> copy(selection = selection.copy(gitStatus = event.gitStatus))
    is DesktopEvent.ChecksLoaded -> copy(review = review.copy(checks = event.checks), jobs = jobs.copy(loading = false, error = null))
    is DesktopEvent.ChatLoaded -> copy(chat = chat.copy(session = event.session))
    is DesktopEvent.ChatProposalLoaded -> {
        val messages = event.session.messages + ChatSessionMessage(role = "user", content = event.userMessage) + event.proposal.assistantMessage
        copy(
            chat = chat.copy(session = event.session.copy(latestDraftId = event.proposal.draft.id, messages = messages)),
            review = review.copy(draft = event.proposal.draft, editor = editableDraft(event.proposal.draft), checks = null),
            jobs = jobs.copy(loading = false, error = null),
        )
    }
    is DesktopEvent.DraftEdited -> review.editor?.let { editor ->
        val changed = editDraft(editor, event.declaration ?: editor.declaration, event.imports ?: editor.imports)
        copy(review = review.copy(draft = changed.serverDraft.copy(validation = null), editor = changed, checks = null))
    } ?: this
    DesktopEvent.DraftValidationStarted -> review.editor?.let { editor ->
        copy(review = review.copy(editor = editor.copy(status = DraftEditorStatus.Validating), checks = null))
    } ?: this
    DesktopEvent.DraftMarkedStale -> review.editor?.let { editor ->
        copy(review = review.copy(draft = editor.serverDraft.copy(validation = null), editor = editor.copy(status = DraftEditorStatus.Stale), checks = null))
    } ?: this
    is DesktopEvent.DraftLoaded -> copy(review = review.copy(draft = event.draft, editor = editableDraft(event.draft), checks = null))
    DesktopEvent.DraftDiscarded -> copy(chat = ChatState(), review = DraftReviewState(applied = review.applied), jobs = jobs.copy(error = null))
    is DesktopEvent.Applied -> copy(review = review.copy(applied = event.result))
    is DesktopEvent.Failed -> copy(jobs = jobs.copy(loading = false, error = event.message))
    is DesktopEvent.Status -> copy(jobs = jobs.copy(status = event.message))
}

data class RequestIdentity(
    val id: Long,
    val projectId: String,
    val projectRevision: String,
    val path: String = "",
    val contentHash: String = "",
)

/**
 * Owns the small amount of request identity needed to reject late asynchronous
 * work. The Compose layer owns coroutine jobs and calls this controller before
 * publishing each response.
 */
class DesktopWorkflowController(initial: DesktopState = DesktopState()) {
    var state: DesktopState = initial
        private set

    private var nextRequestId = 0L
    private var projectRequest: Long = 0
    private var fileRequest: RequestIdentity? = null
    private var analysisRequest: Long = 0
    private var chatRequest: Long = 0
    private var draftRequest: Long = 0

    fun dispatch(event: DesktopEvent): DesktopState {
        if (event == DesktopEvent.DraftDiscarded) {
            chatRequest = 0
            draftRequest = 0
        }
        return state.reduce(event).also { state = it }
    }

    fun synchronize(updated: DesktopState) {
        state = updated
    }

    fun beginProjectLoad(): Long = nextId().also {
        projectRequest = it
        fileRequest = null
        dispatch(DesktopEvent.Loading)
    }

    fun projectLoaded(requestId: Long, project: ProjectAnalysis, index: ProjectIndex): Boolean =
        requestId == projectRequest && project.projectId == index.projectId && project.projectRevision == index.projectRevision && accept(DesktopEvent.ProjectLoaded(project, index))

    fun isCurrentProjectRequest(requestId: Long): Boolean = requestId == projectRequest

    fun beginFileLoad(path: String): RequestIdentity? {
        val project = state.project ?: return null
        val request = RequestIdentity(nextId(), project.projectId, project.projectRevision, path)
        fileRequest = request
        dispatch(DesktopEvent.Loading)
        dispatch(DesktopEvent.Status("Loading $path…"))
        // Clear session and draft immediately, even if the previous request is slow.
        state = state.copy(selection = FileSelectionState(), chat = ChatState(), review = DraftReviewState(applied = state.review.applied))
        return request
    }

    fun fileLoaded(request: RequestIdentity, file: ProjectFileInfo, symbols: List<SymbolInfo>): Boolean {
        val resolved = request.copy(contentHash = file.contentHash)
        if (!matchesProject(request) || fileRequest?.id != request.id || file.path != request.path) return false
        fileRequest = resolved
        dispatch(DesktopEvent.FileLoaded(file, symbols))
        return true
    }

    fun fileFailed(request: RequestIdentity, message: String): Boolean =
        if (fileRequest?.id == request.id && matchesProject(request)) accept(DesktopEvent.Failed(message)) else false

    fun cancelFileLoad(request: RequestIdentity): Boolean =
        if (fileRequest?.id == request.id && matchesProject(request)) {
            fileRequest = null
            accept(DesktopEvent.Status("File load canceled"))
        } else false

    fun analysisLoaded(request: RequestIdentity, analysis: FileAnalysis): Boolean =
        if (matchesFile(request) && analysis.path == request.path) accept(DesktopEvent.AnalysisLoaded(analysis)) else false

    fun impactLoaded(request: RequestIdentity, impact: ImpactPreview): Boolean =
        if (matchesFile(request) && impact.targetPath == request.path) accept(DesktopEvent.ImpactLoaded(impact)) else false

    fun gitStatusLoaded(request: RequestIdentity, gitStatus: GitStatus): Boolean =
        if (matchesFile(request)) accept(DesktopEvent.GitStatusLoaded(gitStatus)) else false

    /** Optional enrichments never replace the source-first file selection with an error state. */
    fun optionalLoadFailed(request: RequestIdentity): Boolean = matchesFile(request)

    fun beginAnalysis(): Pair<Long, RequestIdentity>? {
        val request = fileRequest ?: return null
        analysisRequest = nextId()
        return analysisRequest to request
    }

    fun analysisCompleted(requestId: Long, file: RequestIdentity, analysis: FileAnalysis): Boolean =
        if (requestId == analysisRequest) analysisLoaded(file, analysis) else false

    fun beginChatLoad(): Pair<Long, RequestIdentity>? = fileRequest?.let { file -> nextId().also { chatRequest = it } to file }
    fun chatLoaded(requestId: Long, file: RequestIdentity, session: ChatSession): Boolean =
        if (requestId == chatRequest && matchesFile(file) && sessionMatches(file, session.projectId, session.projectRevision, session.openPath, session.baseFileHash)) accept(DesktopEvent.ChatLoaded(session)) else false

    fun chatProposalLoaded(requestId: Long, file: RequestIdentity, session: ChatSession, userMessage: String, proposal: ChatDraftProposal): Boolean =
        if (
            requestId == chatRequest && matchesFile(file) &&
            sessionMatches(file, session.projectId, session.projectRevision, session.openPath, session.baseFileHash) &&
            proposal.sessionId == session.id &&
            sessionMatches(file, proposal.draft.projectId, proposal.draft.projectRevision, proposal.draft.targetPath, proposal.draft.baseFileHash)
        ) accept(DesktopEvent.ChatProposalLoaded(session, userMessage, proposal)) else false

    fun cancelChatLoad(requestId: Long, file: RequestIdentity): Boolean =
        if (requestId == chatRequest && matchesFile(file)) {
            chatRequest = 0
            accept(DesktopEvent.Status("Chat request canceled"))
        } else false

    fun beginDraftLoad(): Pair<Long, RequestIdentity>? = fileRequest?.let { file -> nextId().also { draftRequest = it } to file }
    fun draftLoaded(requestId: Long, file: RequestIdentity, draft: DeclarationDraft): Boolean =
        if (requestId == draftRequest && matchesFile(file) && sessionMatches(file, draft.projectId, draft.projectRevision, draft.targetPath, draft.baseFileHash)) accept(DesktopEvent.DraftLoaded(draft)) else false

    fun draftChecksLoaded(requestId: Long, file: RequestIdentity, draft: DeclarationDraft, checks: CandidateCheckReport): Boolean =
        if (
            requestId == draftRequest && matchesFile(file) &&
            sessionMatches(file, draft.projectId, draft.projectRevision, draft.targetPath, draft.baseFileHash) &&
            checks.draftId == draft.id && checks.draftRevision == draft.revision && checks.draftHash == draft.hash
        ) accept(DesktopEvent.ChecksLoaded(checks)) else false

    fun currentFileRequest(): RequestIdentity? = fileRequest

    private fun matchesProject(request: RequestIdentity): Boolean = state.project?.let {
        it.projectId == request.projectId && it.projectRevision == request.projectRevision
    } == true

    private fun matchesFile(request: RequestIdentity): Boolean =
        matchesProject(request) && fileRequest == request && state.selectedFile?.let { it.path == request.path && it.contentHash == request.contentHash } == true

    private fun sessionMatches(request: RequestIdentity, projectId: String, revision: String, path: String, hash: String): Boolean =
        request.projectId == projectId && request.projectRevision == revision && request.path == path && request.contentHash == hash

    private fun accept(event: DesktopEvent): Boolean {
        dispatch(event)
        return true
    }

    private fun nextId(): Long = ++nextRequestId
}

data class ApplyEligibility(val eligible: Boolean, val reason: String)

fun draftApplyEligibility(draft: DeclarationDraft?, checks: CandidateCheckReport?, selectedFile: ProjectFileInfo?): ApplyEligibility {
    if (draft == null || selectedFile == null) return ApplyEligibility(false, "Select a file and draft first.")
    if (draft.targetPath != selectedFile.path || draft.baseFileHash != selectedFile.contentHash) return ApplyEligibility(false, "The draft no longer matches the selected file.")
    if (draft.validation?.applicable != true) return ApplyEligibility(false, "Validate the latest draft before applying it.")
    if (!checksPassForDraft(checks, draft)) return ApplyEligibility(false, "Run checks for the latest draft before applying it.")
    return ApplyEligibility(true, "Ready to apply.")
}

private fun checksPassForDraft(checks: CandidateCheckReport?, draft: DeclarationDraft): Boolean =
    checks?.applicable == true && checks.draftId == draft.id && checks.draftRevision == draft.revision && checks.draftHash == draft.hash &&
        checks.checks.all { it.state.lowercase() in setOf("passed", "skipped") }

fun draftReviewEligibility(editor: EditableDraftState?, draft: DeclarationDraft?, checks: CandidateCheckReport?, selectedFile: ProjectFileInfo?, project: ProjectAnalysis?): ApplyEligibility {
    if (editor == null || draft == null) return ApplyEligibility(false, "Select a draft first.")
    if (editor.status == DraftEditorStatus.Stale) return ApplyEligibility(false, "The draft is stale; start a new file-scoped conversation.")
    if (editor.status == DraftEditorStatus.Dirty) return ApplyEligibility(false, "Manual edits require validation and fresh checks.")
    if (editor.status == DraftEditorStatus.Invalid) return ApplyEligibility(false, "Fix validation diagnostics before checks or Apply.")
    if (editor.status == DraftEditorStatus.Validating) return ApplyEligibility(false, "Wait for validation to finish.")
    if (!draftEditorMatchesOpenFile(editor, selectedFile, project)) return ApplyEligibility(false, "The draft no longer matches the open file.")
    return draftApplyEligibility(draft, checks, selectedFile)
}
