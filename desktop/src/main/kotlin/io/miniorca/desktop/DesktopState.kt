package io.miniorca.desktop

/** Stable product workspaces; numbered shortcuts are mapped explicitly elsewhere. */
enum class Workspace {
  Summary,
  Analysis,
  Performance,
  Bugs,
  Security,
  Editor
}

data class ProjectWorkspaceState(
    val project: ProjectAnalysis? = null,
    val index: ProjectIndex? = null,
    val overview: ProjectOverview? = null,
    val sourceChangeObserved: Boolean = false,
    val openingError: String? = null,
)

data class FileSelectionState(
    val selectedFile: ProjectFileInfo? = null,
    val symbols: List<SymbolInfo> = emptyList(),
    val selectedSymbol: SymbolInfo? = null,
    val focusedLine: Int = 0,
    val preparedAction: String = "",
    val preparedRequest: String = "",
    val preparedTaskSpec: BugTaskSpec? = null,
    val analysis: FileAnalysis? = null,
    val impact: ImpactPreview? = null,
    val gitStatus: GitStatus? = null,
    val fileReadError: String? = null,
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
    val performanceJob: PerformanceJob? = null,
    val performanceReport: PerformanceReport? = null,
    val performanceContext: PerformanceQueuePreview? = null,
)

data class AnalysisAdmission(
    val preview: AnalysisRunPreview,
    val resumeRun: AnalysisRunIdentity? = null,
    val providerIds: Set<String> = emptySet(),
    val securityReview: Boolean = false,
) {
  fun isConfirmed(): Boolean =
      preview.providers.filter { it.remoteConfirmationRequired }.all { it.id in providerIds } &&
          (!preview.securityReviewIntentRequired || securityReview)
}

data class AnalysisResultKey(val category: String, val path: String = "")

data class AnalysisSectionState(
    val results: AnalysisSectionResults? = null,
    val loading: Boolean = false,
    val error: String? = null,
)

/** Consent is transient and belongs only to this admission preview. */
data class ProjectAnalysisRunState(
    val run: AnalysisRun? = null,
    val admission: AnalysisAdmission? = null,
    val action: String = "",
    val error: String? = null,
    val sections: Map<AnalysisResultKey, AnalysisSectionState> = emptyMap(),
    val fileSelection: AnalysisSelectionState = AnalysisSelectionState(),
)

enum class SecuritySectionOperationStatus {
  Idle,
  Running,
  Canceled,
  Failed,
}

/** Lifecycle is held separately from retained evidence so a later attempt cannot erase a report. */
data class SecuritySectionOperation(
    val status: SecuritySectionOperationStatus = SecuritySectionOperationStatus.Idle,
    val message: String = "",
)

/** Security reports are file-bound evidence and deliberately never join Bugs findings state. */
data class SecurityWorkspaceState(
    val sourceReport: SecurityFileReport? = null,
    val aiReport: SecurityFileReport? = null,
    val action: String = "",
    val sourceOperation: SecuritySectionOperation = SecuritySectionOperation(),
    val aiOperation: SecuritySectionOperation = SecuritySectionOperation(),
    val error: String? = null,
)

data class EditorNavigationTarget(
    val path: String,
    val symbol: String = "",
    val line: Int = 0,
)

data class EditorNavigationSelection(
    val symbol: SymbolInfo?,
    val focusLine: Int,
)

fun nextWorkspace(workspace: Workspace): Workspace =
    when (workspace) {
      Workspace.Summary -> Workspace.Analysis
      Workspace.Analysis -> Workspace.Performance
      Workspace.Performance -> Workspace.Bugs
      Workspace.Bugs -> Workspace.Security
      Workspace.Security -> Workspace.Editor
      Workspace.Editor -> Workspace.Summary
    }

/** Rejects stale or external paths before a finding can open the Editor. */
fun findingNavigationTarget(
    finding: UnifiedFinding,
    index: ProjectIndex?
): EditorNavigationTarget? {
  val path = finding.location.path.trim()
  val file = index?.files?.firstOrNull { it.path == path } ?: return null
  val symbol = findingNavigationSymbol(finding, file)
  val line = finding.location.startLine.takeIf { it > 0 } ?: symbol?.startLine ?: 0
  return EditorNavigationTarget(path, symbol?.name ?: finding.location.symbol, line)
}

fun symbolForNavigation(symbols: List<SymbolInfo>, target: EditorNavigationTarget): SymbolInfo? =
    namedNavigationSymbol(symbols, target.symbol) ?: symbolAtLine(symbols, target.line)

fun resolveEditorNavigation(
    symbols: List<SymbolInfo>,
    target: EditorNavigationTarget
): EditorNavigationSelection {
  val symbol = symbolForNavigation(symbols, target)
  return EditorNavigationSelection(symbol, navigationFocusLine(target, symbol))
}

/**
 * Keeps a finding's precise line when known, otherwise brings its selected declaration into view.
 */
fun navigationFocusLine(target: EditorNavigationTarget, symbol: SymbolInfo?): Int =
    target.line.takeIf { it > 0 } ?: symbol?.startLine ?: 0

private fun findingNavigationSymbol(finding: UnifiedFinding, file: IndexedFile): SymbolInfo? =
    namedNavigationSymbol(file.symbols, finding.location.symbol)
        ?: symbolAtLine(file.symbols, finding.location.startLine)
        ?: mentionedNavigationSymbol(file.symbols, finding)

private fun namedNavigationSymbol(symbols: List<SymbolInfo>, requestedSymbol: String): SymbolInfo? {
  val requested = requestedSymbol.trim().removeSuffix("()")
  if (requested.isBlank()) return null
  return symbols.firstOrNull { it.name == requested }
      ?: symbols.singleOrNull { it.name.substringAfterLast('.') == requested }
}

private fun mentionedNavigationSymbol(
    symbols: List<SymbolInfo>,
    finding: UnifiedFinding
): SymbolInfo? {
  val description = listOf(finding.title, finding.message, finding.evidence).joinToString("\n")
  return symbols
      .filter { symbol ->
        Regex("(?<![A-Za-z0-9_])${Regex.escape(symbol.name)}(?![A-Za-z0-9_])")
            .containsMatchIn(description)
      }
      .singleOrNull()
}

data class ChatState(
    val session: ChatSession? = null,
    val pendingRequestId: Long = 0,
    val failure: ChatRequestFailure? = null,
)

data class ChatRequestFailure(val target: ChatTarget, val message: String)

data class DraftReviewState(
    val checks: DraftCheckReport? = null,
    val draft: DeclarationDraft? = null,
    val editor: EditableDraftState? = null,
    val applied: ApplyResult? = null,
    val benchmark: BenchmarkEvidenceState = BenchmarkEvidenceState(),
)

/** Catalog lookups and a completed comparison stay tied to their exact reviewed draft. */
data class BenchmarkEvidenceState(
    val catalog: GoBenchmarkCatalog? = null,
    val selected: GoBenchmarkChoice? = null,
    val comparison: GoBenchmarkComparison? = null,
    val running: Boolean = false,
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
 * State is separated by ownership so a file-bound result cannot accidentally overwrite project,
 * job, chat, or review state from another workflow.
 */
data class DesktopState(
    val workspace: Workspace = Workspace.Summary,
    val projectState: ProjectWorkspaceState = ProjectWorkspaceState(),
    val selection: FileSelectionState = FileSelectionState(),
    val jobs: JobState = JobState(),
    val findings: FindingsState = FindingsState(),
    val analysisRun: ProjectAnalysisRunState = ProjectAnalysisRunState(),
    val security: SecurityWorkspaceState = SecurityWorkspaceState(),
    val chat: ChatState = ChatState(),
    val review: DraftReviewState = DraftReviewState(),
    val connection: ConnectionState = ConnectionState(),
    val preparedRequestGeneration: Long = 0,
) {
  val project
    get() = projectState.project

  val index
    get() = projectState.index

  val overview
    get() = projectState.overview

  val selectedFile
    get() = selection.selectedFile

  val symbols
    get() = selection.symbols

  val selectedSymbol
    get() = selection.selectedSymbol

  val preparedAction
    get() = selection.preparedAction

  val preparedRequest
    get() = selection.preparedRequest

  val preparedTaskSpec
    get() = selection.preparedTaskSpec

  val analysis
    get() = selection.analysis

  val impact
    get() = selection.impact

  val gitStatus
    get() = selection.gitStatus

  val checks
    get() = review.checks

  val loading
    get() = jobs.loading

  val status
    get() = jobs.status

  val error
    get() = jobs.error
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

  data class AnalysisRunUpdated(val state: ProjectAnalysisRunState) : DesktopEvent

  data class AnalyzeAllLoaded(val job: AnalyzeAllJob?) : DesktopEvent

  data class PerformanceLoaded(val job: PerformanceJob?, val report: PerformanceReport?) :
      DesktopEvent

  data class PerformanceContextLoaded(val context: PerformanceQueuePreview) : DesktopEvent

  data class GoScanLoaded(val scan: GoScanReport?) : DesktopEvent

  data class SecurityActionStarted(val action: String) : DesktopEvent

  data object SecurityActionCanceled : DesktopEvent

  data class SecurityReportLoaded(val report: SecurityFileReport) : DesktopEvent

  data class SecurityActionFailed(val action: String, val message: String) : DesktopEvent

  data class FileLoaded(val file: ProjectFileInfo, val symbols: List<SymbolInfo>) : DesktopEvent

  sealed interface LocalReadFailure : DesktopEvent {
    val message: String
  }

  data class FileLoadFailed(override val message: String) : LocalReadFailure

  data class ProjectLoadFailed(override val message: String) : LocalReadFailure

  data class SelectedFileRefreshed(val file: ProjectFileInfo, val symbols: List<SymbolInfo>) :
      DesktopEvent

  data class SelectedFileUnavailable(val message: String) : DesktopEvent

  data class SymbolSelected(val symbol: SymbolInfo) : DesktopEvent

  data class EditorContextSelected(val symbol: SymbolInfo?, val line: Int) : DesktopEvent

  data class SourceLineSelected(val selection: SourceLineSelection) : DesktopEvent

  data class SuggestionPrepared(
      val action: String,
      val request: String,
      val symbol: SymbolInfo?,
      val taskSpec: BugTaskSpec? = null
  ) : DesktopEvent

  data object SuggestionCleared : DesktopEvent

  data class AnalysisLoaded(val analysis: FileAnalysis) : DesktopEvent

  data class ImpactLoaded(val impact: ImpactPreview) : DesktopEvent

  data class GitStatusLoaded(val gitStatus: GitStatus) : DesktopEvent

  data class ChecksLoaded(val checks: DraftCheckReport) : DesktopEvent

  data class GoBenchmarkCatalogLoaded(val catalog: GoBenchmarkCatalog) : DesktopEvent

  data class GoBenchmarkSelected(val choice: GoBenchmarkChoice) : DesktopEvent

  data object GoBenchmarkComparisonStarted : DesktopEvent

  data class GoBenchmarkComparisonLoaded(val comparison: GoBenchmarkComparison) : DesktopEvent

  data object GoBenchmarkComparisonStopped : DesktopEvent

  data class ChatLoaded(val session: ChatSession) : DesktopEvent

  data class ChatProposalLoaded(
      val session: ChatSession,
      val userMessage: String,
      val proposal: ChatDraftProposal
  ) : DesktopEvent

  data class DraftEdited(val declaration: String? = null, val imports: List<String>? = null) :
      DesktopEvent

  data object DraftValidationStarted : DesktopEvent

  data object DraftMarkedStale : DesktopEvent

  data class DraftLoaded(val draft: DeclarationDraft) : DesktopEvent

  data object DraftDiscarded : DesktopEvent

  data class Applied(val result: ApplyResult?) : DesktopEvent

  data class Failed(val message: String) : DesktopEvent

  data class ChatRequestFailed(val failure: ChatRequestFailure) : DesktopEvent

  data class Status(val message: String) : DesktopEvent
}

fun DesktopState.reduce(event: DesktopEvent): DesktopState =
    when (event) {
      DesktopEvent.Loading -> copy(jobs = jobs.copy(loading = true, error = null))
      is DesktopEvent.WorkspaceSelected -> copy(workspace = event.workspace)
      is DesktopEvent.ConnectionUpdated -> copy(connection = event.connection)
      is DesktopEvent.ProjectLoaded ->
          copy(
              workspace = Workspace.Summary,
              projectState = ProjectWorkspaceState(event.project, event.index),
              selection = FileSelectionState(),
              findings = FindingsState(),
              analysisRun = ProjectAnalysisRunState(),
              security = SecurityWorkspaceState(),
              chat = ChatState(),
              review = DraftReviewState(),
              jobs =
                  jobs.copy(
                      loading = false, status = "Imported ${event.project.name}", error = null),
          )
      is DesktopEvent.LocalReadFailure -> withLocalReadFailure(event)
      is DesktopEvent.IndexRefreshed -> withRefreshedIndex(event.index)
      is DesktopEvent.OverviewLoaded ->
          copy(projectState = projectState.copy(overview = event.overview))
      is DesktopEvent.FindingsLoaded -> copy(findings = findings.copy(findings = event.findings))
      is DesktopEvent.FindingStatusUpdated -> withFindingStatus(event)
      is DesktopEvent.AnalysisRunUpdated ->
          copy(analysisRun = event.state.afterRevisionChange(projectState.sourceChangeObserved))
      is DesktopEvent.AnalyzeAllLoaded -> copy(findings = findings.copy(analyzeAll = event.job))
      is DesktopEvent.PerformanceLoaded ->
          copy(
              findings =
                  findings.copy(performanceJob = event.job, performanceReport = event.report))
      is DesktopEvent.PerformanceContextLoaded ->
          copy(findings = findings.copy(performanceContext = event.context))
      is DesktopEvent.GoScanLoaded -> copy(findings = findings.copy(scan = event.scan))
      is DesktopEvent.SecurityActionStarted ->
          copy(
              security =
                  when (event.action) {
                    "scan" ->
                        security.copy(
                            action = event.action,
                            sourceOperation =
                                SecuritySectionOperation(SecuritySectionOperationStatus.Running),
                            error = null)
                    "review" ->
                        security.copy(
                            action = event.action,
                            aiOperation =
                                SecuritySectionOperation(SecuritySectionOperationStatus.Running),
                            error = null)
                    else -> security.copy(action = event.action, error = null)
                  })
      DesktopEvent.SecurityActionCanceled ->
          copy(
              security =
                  when (security.action) {
                    "scan" ->
                        security.copy(
                            action = "",
                            sourceOperation =
                                SecuritySectionOperation(SecuritySectionOperationStatus.Canceled))
                    "review" ->
                        security.copy(
                            action = "",
                            aiOperation =
                                SecuritySectionOperation(SecuritySectionOperationStatus.Canceled))
                    else -> security
                  })
      is DesktopEvent.SecurityReportLoaded ->
          copy(
              security =
                  if (event.report.source.equals("deterministic", ignoreCase = true))
                      security.copy(
                          sourceReport = event.report,
                          action = "",
                          sourceOperation = SecuritySectionOperation(),
                          error = null)
                  else
                      security.copy(
                          aiReport = event.report,
                          action = "",
                          aiOperation = SecuritySectionOperation(),
                          error = null))
      is DesktopEvent.SecurityActionFailed ->
          copy(
              security =
                  when (event.action) {
                    "scan" ->
                        security.copy(
                            action = "",
                            sourceOperation =
                                SecuritySectionOperation(
                                    SecuritySectionOperationStatus.Failed, event.message),
                            error = event.message)
                    "review" ->
                        security.copy(
                            action = "",
                            aiOperation =
                                SecuritySectionOperation(
                                    SecuritySectionOperationStatus.Failed, event.message),
                            error = event.message)
                    else -> security.copy(action = "", error = event.message)
                  })
      is DesktopEvent.FileLoaded ->
          copy(
              selection = FileSelectionState(selectedFile = event.file, symbols = event.symbols),
              chat = ChatState(),
              review = DraftReviewState(applied = review.applied),
              jobs = jobs.copy(loading = false, status = event.file.path, error = null),
          )
      is DesktopEvent.SelectedFileRefreshed -> withRefreshedFile(event)
      is DesktopEvent.SelectedFileUnavailable ->
          withObservedSourceChange()
              .copy(
                  selection = FileSelectionState(fileReadError = event.message),
                  jobs =
                      jobs.copy(
                          loading = false,
                          error = event.message,
                          status =
                              "Source could not be refreshed. Reopen the file or reindex the project."),
              )
      is DesktopEvent.SymbolSelected -> selectEditorTarget(event.symbol, event.symbol.startLine)
      is DesktopEvent.EditorContextSelected -> selectEditorTarget(event.symbol, event.line)
      is DesktopEvent.SourceLineSelected ->
          selectEditorTarget(event.selection.symbol, event.selection.line)
      is DesktopEvent.SuggestionPrepared ->
          copy(
              selection =
                  selection.copy(
                      selectedSymbol = event.symbol ?: selectedSymbol,
                      preparedAction = event.action,
                      preparedRequest = event.request,
                      preparedTaskSpec = event.taskSpec),
              preparedRequestGeneration = preparedRequestGeneration + 1,
              jobs = jobs.copy(error = null))
      DesktopEvent.SuggestionCleared -> clearPreparedSuggestion()
      is DesktopEvent.AnalysisLoaded ->
          copy(
              selection = selection.copy(analysis = event.analysis),
              jobs = jobs.copy(loading = false, error = null))
      is DesktopEvent.ImpactLoaded -> copy(selection = selection.copy(impact = event.impact))
      is DesktopEvent.GitStatusLoaded ->
          copy(selection = selection.copy(gitStatus = event.gitStatus))
      is DesktopEvent.ChecksLoaded ->
          copy(
              review = review.copy(checks = event.checks),
              jobs = jobs.copy(loading = false, error = null))
      is DesktopEvent.GoBenchmarkCatalogLoaded ->
          copy(
              review =
                  review.copy(
                      benchmark =
                          review.benchmark.copy(
                              catalog = event.catalog, selected = null, running = false)),
              jobs = jobs.copy(loading = false, error = null))
      is DesktopEvent.GoBenchmarkSelected ->
          copy(
              review =
                  review.copy(
                      benchmark = review.benchmark.copy(selected = event.choice, running = false)),
              jobs = jobs.copy(loading = false))
      DesktopEvent.GoBenchmarkComparisonStarted ->
          copy(
              review = review.copy(benchmark = review.benchmark.copy(running = true)),
              jobs = jobs.copy(loading = true, error = null))
      is DesktopEvent.GoBenchmarkComparisonLoaded ->
          copy(
              review =
                  review.copy(
                      benchmark =
                          review.benchmark.copy(comparison = event.comparison, running = false)),
              jobs = jobs.copy(loading = false, error = null))
      DesktopEvent.GoBenchmarkComparisonStopped ->
          copy(
              review = review.copy(benchmark = review.benchmark.copy(running = false)),
              jobs = jobs.copy(loading = false))
      is DesktopEvent.ChatRequestFailed ->
          copy(
              chat = chat.copy(failure = event.failure),
              jobs =
                  jobs.copy(
                      loading = false,
                      error = event.failure.message,
                      status = "Request failed for ${event.failure.target.symbol}."))
      is DesktopEvent.ChatLoaded -> copy(chat = chat.copy(session = event.session, failure = null))
      is DesktopEvent.ChatProposalLoaded -> {
        val messages =
            event.session.messages +
                ChatSessionMessage(role = "user", content = event.userMessage) +
                event.proposal.assistantMessage
        copy(
            chat =
                chat.copy(
                    failure = null,
                    session =
                        event.session.copy(
                            latestDraftId = event.proposal.draft.id, messages = messages)),
            review =
                review.copy(
                    draft = event.proposal.draft,
                    editor = editableDraft(event.proposal.draft),
                    checks = null,
                    benchmark = BenchmarkEvidenceState()),
            jobs = jobs.copy(loading = false, error = null),
        )
      }
      is DesktopEvent.DraftEdited ->
          review.editor?.let { editor ->
            val changed =
                editDraft(
                    editor,
                    event.declaration ?: editor.declaration,
                    event.imports ?: editor.imports)
            copy(
                review =
                    review.copy(
                        draft = changed.serverDraft.copy(validation = null),
                        editor = changed,
                        checks = null,
                        benchmark = review.benchmark.withoutCatalog()),
                jobs = jobs.copy(loading = false))
          } ?: this
      DesktopEvent.DraftValidationStarted -> withValidationStarted()
      DesktopEvent.DraftMarkedStale ->
          review.editor?.let { editor ->
            copy(
                review =
                    review.copy(
                        draft = editor.serverDraft.copy(validation = null),
                        editor = editor.copy(status = DraftEditorStatus.Stale),
                        checks = null,
                        benchmark = review.benchmark.withoutCatalog()),
                jobs = jobs.copy(loading = false))
          } ?: this
      is DesktopEvent.DraftLoaded ->
          copy(
              review =
                  review.copy(
                      draft = event.draft,
                      editor = editableDraft(event.draft),
                      checks = null,
                      benchmark = review.benchmark.withoutCatalog()),
              jobs = jobs.copy(loading = false))
      DesktopEvent.DraftDiscarded ->
          copy(
              chat = ChatState(),
              review =
                  DraftReviewState(
                      applied = review.applied, benchmark = review.benchmark.withoutCatalog()),
              jobs = jobs.copy(loading = false, error = null))
      is DesktopEvent.Applied -> copy(review = review.copy(applied = event.result))
      is DesktopEvent.Failed -> copy(jobs = jobs.copy(loading = false, error = event.message))
      is DesktopEvent.Status -> copy(jobs = jobs.copy(status = event.message))
    }

private fun DesktopState.withLocalReadFailure(event: DesktopEvent.LocalReadFailure): DesktopState =
    when (event) {
      is DesktopEvent.ProjectLoadFailed ->
          copy(
              projectState = projectState.copy(openingError = event.message),
              jobs = jobs.copy(loading = false, error = event.message))
      is DesktopEvent.FileLoadFailed ->
          copy(
              selection = selection.copy(fileReadError = event.message),
              jobs = jobs.copy(loading = false, error = event.message))
    }

private fun DesktopState.withValidationStarted(): DesktopState =
    review.editor?.let { editor ->
      copy(
          review =
              review.copy(
                  editor = editor.copy(status = DraftEditorStatus.Validating),
                  checks = null,
                  benchmark = review.benchmark.withoutCatalog()),
          jobs = jobs.copy(loading = false))
    } ?: this

private fun BenchmarkEvidenceState.withoutCatalog(): BenchmarkEvidenceState =
    copy(catalog = null, selected = null, running = false)

private fun DesktopState.withRefreshedIndex(index: ProjectIndex): DesktopState {
  val revisionChanged = project?.projectRevision != index.projectRevision
  return copy(
      projectState =
          projectState.copy(
              project = project?.copy(projectRevision = index.projectRevision),
              index = index,
              sourceChangeObserved = false),
      analysisRun = analysisRun.afterRevisionChange(revisionChanged),
      chat = if (revisionChanged) ChatState() else chat,
      review = if (revisionChanged) DraftReviewState(applied = review.applied) else review,
      jobs = jobs.copy(loading = false, status = "Re-analyzed project index", error = null),
  )
}

private fun DesktopState.withRefreshedFile(
    event: DesktopEvent.SelectedFileRefreshed
): DesktopState =
    if (selectedFile?.path != event.file.path ||
        selectedFile?.contentHash == event.file.contentHash)
        this
    else
        withObservedSourceChange()
            .copy(
                selection =
                    FileSelectionState(
                        selectedFile = event.file,
                        symbols = event.symbols,
                        selectedSymbol =
                            event.symbols.firstOrNull { it.name == selectedSymbol?.name },
                    ),
                jobs =
                    jobs.copy(
                        loading = false,
                        error = null,
                        status =
                            "File changed outside Mini-Orca. Review evidence is stale; reindex the project before analysis."),
            )

private fun DesktopState.withObservedSourceChange(): DesktopState =
    reduce(DesktopEvent.DraftMarkedStale)
        .copy(
            projectState = projectState.copy(sourceChangeObserved = true),
            analysisRun = analysisRun.afterRevisionChange(true),
            chat = ChatState(),
            security = SecurityWorkspaceState(),
        )

data class RequestIdentity(
    val id: Long,
    val projectId: String,
    val projectRevision: String,
    val path: String = "",
    val contentHash: String = "",
)

/**
 * Owns the small amount of request identity needed to reject late asynchronous work. The Compose
 * layer owns coroutine jobs and calls this controller before publishing each response.
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
    if (event is DesktopEvent.SelectedFileRefreshed &&
        state.selectedFile?.path == event.file.path &&
        state.selectedFile?.contentHash != event.file.contentHash) {
      fileRequest = fileRequest?.copy(id = nextId(), contentHash = event.file.contentHash)
      chatRequest = 0
      draftRequest = 0
    }
    if (event is DesktopEvent.SelectedFileUnavailable) fileRequest = null
    if (event == DesktopEvent.DraftDiscarded) {
      chatRequest = 0
      draftRequest = 0
    }
    return state.reduce(event).also { state = it }
  }

  fun beginProjectLoad(): Long =
      nextId().also {
        projectRequest = it
        fileRequest = null
        state = state.copy(projectState = state.projectState.copy(openingError = null))
        dispatch(DesktopEvent.Loading)
      }

  fun projectLoaded(requestId: Long, project: ProjectAnalysis, index: ProjectIndex): Boolean =
      requestId == projectRequest &&
          project.projectId == index.projectId &&
          project.projectRevision == index.projectRevision &&
          accept(DesktopEvent.ProjectLoaded(project, index))

  fun isCurrentProjectRequest(requestId: Long): Boolean = requestId == projectRequest

  fun projectFailed(requestId: Long, message: String): Boolean =
      if (isCurrentProjectRequest(requestId)) accept(DesktopEvent.ProjectLoadFailed(message))
      else false

  fun beginFileLoad(path: String): RequestIdentity? {
    val project = state.project ?: return null
    val request = RequestIdentity(nextId(), project.projectId, project.projectRevision, path)
    fileRequest = request
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("Loading $path…"))
    // Clear session and draft immediately, even if the previous request is slow.
    state =
        state.copy(
            selection = FileSelectionState(),
            chat = ChatState(),
            review = DraftReviewState(applied = state.review.applied))
    return request
  }

  fun fileLoaded(
      request: RequestIdentity,
      file: ProjectFileInfo,
      symbols: List<SymbolInfo>
  ): Boolean {
    val resolved = request.copy(contentHash = file.contentHash)
    if (!matchesProject(request) || fileRequest?.id != request.id || file.path != request.path)
        return false
    fileRequest = resolved
    dispatch(DesktopEvent.FileLoaded(file, symbols))
    return true
  }

  fun fileFailed(request: RequestIdentity, message: String): Boolean =
      if (fileRequest?.id == request.id && matchesProject(request))
          accept(DesktopEvent.FileLoadFailed(message))
      else false

  fun cancelFileLoad(request: RequestIdentity): Boolean =
      if (fileRequest?.id == request.id && matchesProject(request)) {
        fileRequest = null
        accept(DesktopEvent.Status("File load canceled"))
      } else false

  fun analysisLoaded(request: RequestIdentity, analysis: FileAnalysis): Boolean =
      if (matchesFile(request) && analysis.path == request.path)
          accept(DesktopEvent.AnalysisLoaded(analysis))
      else false

  fun impactLoaded(request: RequestIdentity, impact: ImpactPreview): Boolean =
      if (matchesFile(request) && impact.targetPath == request.path)
          accept(DesktopEvent.ImpactLoaded(impact))
      else false

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

  fun beginChatLoad(): Pair<Long, RequestIdentity>? =
      fileRequest?.let { file ->
        state = state.copy(chat = state.chat.copy(failure = null))
        nextId().also { chatRequest = it } to file
      }

  fun chatLoaded(requestId: Long, file: RequestIdentity, session: ChatSession): Boolean =
      if (requestId == chatRequest &&
          matchesFile(file) &&
          sessionMatches(
              file,
              session.projectId,
              session.projectRevision,
              session.openPath,
              session.baseFileHash))
          accept(DesktopEvent.ChatLoaded(session))
      else false

  fun chatProposalLoaded(
      requestId: Long,
      file: RequestIdentity,
      session: ChatSession,
      userMessage: String,
      proposal: ChatDraftProposal
  ): Boolean =
      if (requestId == chatRequest &&
          matchesFile(file) &&
          sessionMatches(
              file,
              session.projectId,
              session.projectRevision,
              session.openPath,
              session.baseFileHash) &&
          proposal.sessionId == session.id &&
          sessionMatches(
              file,
              proposal.draft.projectId,
              proposal.draft.projectRevision,
              proposal.draft.targetPath,
              proposal.draft.baseFileHash))
          accept(DesktopEvent.ChatProposalLoaded(session, userMessage, proposal))
      else false

  fun cancelChatLoad(requestId: Long, file: RequestIdentity): Boolean =
      if (requestId == chatRequest && matchesFile(file)) {
        chatRequest = 0
        accept(DesktopEvent.Status("Chat request canceled"))
      } else false

  fun beginDraftLoad(): Pair<Long, RequestIdentity>? =
      fileRequest?.let { file -> nextId().also { draftRequest = it } to file }

  fun draftLoaded(requestId: Long, file: RequestIdentity, draft: DeclarationDraft): Boolean =
      if (requestId == draftRequest &&
          matchesFile(file) &&
          sessionMatches(
              file, draft.projectId, draft.projectRevision, draft.targetPath, draft.baseFileHash))
          accept(DesktopEvent.DraftLoaded(draft))
      else false

  fun draftChecksLoaded(
      requestId: Long,
      file: RequestIdentity,
      draft: DeclarationDraft,
      checks: DraftCheckReport
  ): Boolean =
      if (requestId == draftRequest &&
          matchesFile(file) &&
          sessionMatches(
              file, draft.projectId, draft.projectRevision, draft.targetPath, draft.baseFileHash) &&
          checks.draftId == draft.id &&
          checks.draftRevision == draft.revision &&
          checks.draftHash == draft.hash)
          accept(DesktopEvent.ChecksLoaded(checks))
      else false

  fun currentFileRequest(): RequestIdentity? = fileRequest

  private fun matchesProject(request: RequestIdentity): Boolean =
      state.project?.let {
        it.projectId == request.projectId && it.projectRevision == request.projectRevision
      } == true

  private fun matchesFile(request: RequestIdentity): Boolean =
      matchesProject(request) &&
          fileRequest == request &&
          state.selectedFile?.let {
            it.path == request.path && it.contentHash == request.contentHash
          } == true

  private fun sessionMatches(
      request: RequestIdentity,
      projectId: String,
      revision: String,
      path: String,
      hash: String
  ): Boolean =
      request.projectId == projectId &&
          request.projectRevision == revision &&
          request.path == path &&
          request.contentHash == hash

  private fun accept(event: DesktopEvent): Boolean {
    dispatch(event)
    return true
  }

  private fun nextId(): Long = ++nextRequestId
}

private fun DesktopState.selectEditorTarget(symbol: SymbolInfo?, line: Int): DesktopState {
  val targetChanged = selectedSymbol?.name != symbol?.name
  return copy(
      selection =
          selection.copy(selectedSymbol = symbol, focusedLine = line).let {
            if (targetChanged) it.withoutPreparedSuggestion() else it
          },
      preparedRequestGeneration = preparedRequestGeneration + if (targetChanged) 1 else 0,
      jobs = jobs.copy(error = null),
  )
}

private fun DesktopState.clearPreparedSuggestion(): DesktopState =
    copy(
        selection = selection.withoutPreparedSuggestion(),
        preparedRequestGeneration = preparedRequestGeneration + 1,
    )

private fun FileSelectionState.withoutPreparedSuggestion(): FileSelectionState =
    copy(preparedAction = "", preparedRequest = "", preparedTaskSpec = null)

internal fun unconsumedPreparedRequest(
    state: DesktopState,
    consumedGeneration: Long,
): String? =
    state.preparedRequest.takeIf {
      state.preparedRequestGeneration > consumedGeneration && it.isNotBlank()
    }

data class ApplyEligibility(val eligible: Boolean, val reason: String)

fun draftApplyEligibility(
    draft: DeclarationDraft?,
    checks: DraftCheckReport?,
    selectedFile: ProjectFileInfo?
): ApplyEligibility {
  if (draft == null || selectedFile == null)
      return ApplyEligibility(false, "Select a file and draft first.")
  if (draft.targetPath != selectedFile.path || draft.baseFileHash != selectedFile.contentHash)
      return ApplyEligibility(false, "The draft no longer matches the selected file.")
  if (draft.validation?.applicable != true)
      return ApplyEligibility(false, "Validate the latest draft before applying it.")
  if (!checksPassForDraft(checks, draft))
      return ApplyEligibility(false, "Run checks for the latest draft before applying it.")
  return ApplyEligibility(true, "Ready to apply.")
}

private fun checksPassForDraft(checks: DraftCheckReport?, draft: DeclarationDraft): Boolean =
    checks?.applicable == true &&
        checks.draftId == draft.id &&
        checks.draftRevision == draft.revision &&
        checks.draftHash == draft.hash &&
        checks.checks.all { it.state.lowercase() in setOf("passed", "skipped") }

fun draftReviewEligibility(
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?,
    selectedFile: ProjectFileInfo?,
    project: ProjectAnalysis?
): ApplyEligibility {
  if (editor == null || draft == null) return ApplyEligibility(false, "Select a draft first.")
  if (editor.status == DraftEditorStatus.Stale)
      return ApplyEligibility(false, "The draft is stale; start a new file-scoped conversation.")
  if (editor.status == DraftEditorStatus.Dirty)
      return ApplyEligibility(false, "Manual edits require validation and fresh checks.")
  if (editor.status == DraftEditorStatus.Invalid)
      return ApplyEligibility(false, "Fix validation diagnostics before checks or Apply.")
  if (editor.status == DraftEditorStatus.Validating)
      return ApplyEligibility(false, "Wait for validation to finish.")
  if (!draftEditorMatchesOpenFile(editor, selectedFile, project))
      return ApplyEligibility(false, "The draft no longer matches the open file.")
  return draftApplyEligibility(draft, checks, selectedFile)
}

private fun ProjectAnalysisRunState.afterRevisionChange(changed: Boolean): ProjectAnalysisRunState =
    if (changed) copy(admission = null, action = "", run = run?.copy(status = "stale")) else this

private fun DesktopState.withFindingStatus(event: DesktopEvent.FindingStatusUpdated): DesktopState {
  fun List<UnifiedFinding>.updated(): List<UnifiedFinding> = map { finding ->
    if (finding.id == event.findingId) finding.copy(status = event.status) else finding
  }
  return copy(
      findings = findings.copy(findings = findings.findings.updated()),
      analysisRun =
          analysisRun.copy(
              sections =
                  analysisRun.sections.mapValues { (_, section) ->
                    section.copy(
                        results =
                            section.results?.let {
                              it.copy(
                                  semantic = it.semantic.updated(),
                                  unclassified = it.unclassified.updated())
                            })
                  }))
}
