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

enum class ProjectOpeningKind {
  Import,
  Restore,
}

sealed interface ProjectOpeningOutcome {
  data object Opening : ProjectOpeningOutcome

  data class Failed(val message: String) : ProjectOpeningOutcome

  data object Canceled : ProjectOpeningOutcome
}

data class ProjectOpeningAttempt(
    val requestId: Long,
    val path: String,
    val kind: ProjectOpeningKind,
    val outcome: ProjectOpeningOutcome = ProjectOpeningOutcome.Opening,
)

sealed interface ProjectIndexingOutcome {
  data object Running : ProjectIndexingOutcome

  data class Succeeded(val revision: String) : ProjectIndexingOutcome

  data class Failed(val message: String) : ProjectIndexingOutcome

  data object Canceled : ProjectIndexingOutcome
}

data class ProjectIndexingAttempt(
    val generation: Long,
    val projectId: String,
    val projectRevision: String,
    val path: String,
    val outcome: ProjectIndexingOutcome = ProjectIndexingOutcome.Running,
)

sealed interface ProjectDetailsOutcome {
  data object Refreshing : ProjectDetailsOutcome

  data object Available : ProjectDetailsOutcome

  data class Unavailable(val message: String) : ProjectDetailsOutcome
}

data class ProjectWorkspaceState(
    val project: ProjectAnalysis? = null,
    val index: ProjectIndex? = null,
    val overview: ProjectOverview? = null,
    val detailsOutcome: ProjectDetailsOutcome? = null,
    val sourceChangeObserved: Boolean = false,
    val openingAttempt: ProjectOpeningAttempt? = null,
    val indexingAttempt: ProjectIndexingAttempt? = null,
    val rememberedPath: String? = null,
    val preferenceReadWarning: String? = null,
    val preferenceSaveWarning: String? = null,
) {
  // Read-only bridge for existing shell/header consumers until they render attempts.
  val openingError: String?
    get() = (openingAttempt?.outcome as? ProjectOpeningOutcome.Failed)?.message

  // Existing rendering fixtures still construct an error-only state; migrate them with the UI.
  constructor(
      openingError: String
  ) : this(
      openingAttempt =
          ProjectOpeningAttempt(
              0, "", ProjectOpeningKind.Import, ProjectOpeningOutcome.Failed(openingError)))
}

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
    val pendingFilePath: String? = null,
    val failedFilePath: String? = null,
)

data class JobState(
    val loading: Boolean = false,
    val status: String = "Daemon ready",
    val error: String? = null,
)

sealed interface VerifiedScanRead {
  data object Unread : VerifiedScanRead

  data object Reading : VerifiedScanRead

  data object Absent : VerifiedScanRead

  data object Loaded : VerifiedScanRead

  data class Unavailable(val message: String) : VerifiedScanRead

  data class PollUnavailable(val message: String) : VerifiedScanRead
}

sealed interface VerifiedScanFindingsRefresh {
  data object Unread : VerifiedScanFindingsRefresh

  data object Refreshing : VerifiedScanFindingsRefresh

  data object Stale : VerifiedScanFindingsRefresh

  data object Current : VerifiedScanFindingsRefresh

  data class Unavailable(val message: String) : VerifiedScanFindingsRefresh
}

sealed interface VerifiedScanOperation {
  data object Idle : VerifiedScanOperation

  data object Starting : VerifiedScanOperation

  data object CancellationRequested : VerifiedScanOperation

  data class Failed(val message: String) : VerifiedScanOperation

  data class StartUncertain(val message: String) : VerifiedScanOperation

  data class CancellationUnconfirmed(val message: String) : VerifiedScanOperation
}

data class VerifiedScanState(
    val read: VerifiedScanRead = VerifiedScanRead.Unread,
    val operation: VerifiedScanOperation = VerifiedScanOperation.Idle,
    val findingsRefresh: VerifiedScanFindingsRefresh = VerifiedScanFindingsRefresh.Unread,
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

data class AnalysisPreviewIntent(
    val projectId: String,
    val projectRevision: String,
    val limits: AnalysisRunLimits,
    val refresh: Boolean,
    val retryStaleFailed: Boolean,
    val resumeRun: AnalysisRunIdentity? = null,
    val resumePlan: AnalysisRunPreview? = null,
) {
  fun request() =
      AnalysisPreviewRequest(
          projectId, projectRevision, "project", refresh, limits, resumeRun, retryStaleFailed)
}

data class AnalysisResultKey(val category: String, val path: String = "")

data class AnalysisSectionState(
    val results: AnalysisSectionResults? = null,
    val loading: Boolean = false,
    val error: String? = null,
)

enum class AdmissionRecovery {
  Rejected,
  Uncertain,
}

enum class AnalysisRunErrorKind {
  Preview,
  Admission,
  Control,
  StatusRead,
}

enum class AnalysisControlOutcome {
  Requesting,
  Reconciling,
  Unconfirmed,
}

data class AnalysisControlRequest(val action: String, val outcome: AnalysisControlOutcome)

/** Consent is transient and belongs only to this admission preview. */
data class ProjectAnalysisRunState(
    val run: AnalysisRun? = null,
    val previousRun: AnalysisRun? = null,
    val admission: AnalysisAdmission? = null,
    val previewIntent: AnalysisPreviewIntent? = null,
    val admissionRecovery: AdmissionRecovery? = null,
    val action: String = "",
    val error: String? = null,
    val errorKind: AnalysisRunErrorKind? = null,
    val controlRequest: AnalysisControlRequest? = null,
    val statusUnavailable: Boolean = false,
    val sections: Map<AnalysisResultKey, AnalysisSectionState> = emptyMap(),
    val fileSelection: AnalysisSelectionState = AnalysisSelectionState(),
)

internal fun ProjectAnalysisRunState.showsAdmissionOverlay(): Boolean =
    admission != null ||
        action == "preview" ||
        error != null &&
            errorKind in setOf(AnalysisRunErrorKind.Preview, AnalysisRunErrorKind.Admission)

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

/** Work captured for a transactional ordinary-file replacement, not inspection or evidence. */
internal data class FileNavigationIdentity(
    val project: SwitchProjectIdentity,
    val index: ProjectIndex,
    val selectedFile: ProjectFileInfo?,
    val draft: SwitchDraftIdentity,
    val chatRequestId: Long,
)

internal fun DesktopState.fileNavigationIdentity(path: String): FileNavigationIdentity? {
  val project = project ?: return null
  val index = index ?: return null
  if (index.projectId != project.projectId ||
      index.projectRevision != project.projectRevision ||
      index.files.count { it.path == path } != 1)
      return null
  return FileNavigationIdentity(
      SwitchProjectIdentity(project),
      index,
      selectedFile,
      SwitchDraftIdentity(chat.session, review.draft, review.editor),
      chat.pendingRequestId)
}

internal fun DesktopState.matchesFileNavigation(
    path: String,
    identity: FileNavigationIdentity
): Boolean = index === identity.index && fileNavigationIdentity(path) == identity

data class ChatRequestScope(
    val projectId: String,
    val projectRevision: String,
    val path: String,
    val baseFileHash: String,
    val target: ChatTarget,
    val taskSpec: BugTaskSpec? = null,
    val declaration: SymbolInfo? = null,
) {
  fun matchesDraft(draft: DeclarationDraft): Boolean =
      projectId == draft.projectId &&
          projectRevision == draft.projectRevision &&
          path == draft.targetPath &&
          baseFileHash == draft.baseFileHash &&
          target.mode.wireValue == draft.mode &&
          target.symbol == draft.targetSymbol &&
          (sameTaskSpec(taskSpec, draft.taskSpec) ||
              taskSpec != null &&
                  draft.taskSpec != null &&
                  repairTaskSpecMatches(taskSpec, draft.taskSpec))

  fun matches(session: ChatSession): Boolean =
      projectId == session.projectId &&
          projectRevision == session.projectRevision &&
          path == session.openPath &&
          baseFileHash == session.baseFileHash &&
          target.mode.wireValue == session.mode &&
          target.symbol == session.targetSymbol &&
          (sameTaskSpec(taskSpec, session.taskSpec) ||
              taskSpec != null &&
                  session.taskSpec != null &&
                  repairTaskSpecMatches(taskSpec, session.taskSpec))
}

sealed interface ChatRequestOutcome {
  data object Running : ChatRequestOutcome

  data class Succeeded(val sessionId: String, val draftId: String) : ChatRequestOutcome

  data class Failed(val message: String) : ChatRequestOutcome

  data object Canceled : ChatRequestOutcome
}

data class ChatRequestAttempt(
    val generation: Long,
    val scope: ChatRequestScope,
    val destination: ScopedModel,
    val remoteConfirmed: Boolean,
    val requestText: String,
    val admittedDraftRevision: Long,
    val outcome: ChatRequestOutcome = ChatRequestOutcome.Running,
    val invalidationReason: String? = null,
    val creationKind: String? = null,
)

data class ChatState(
    val session: ChatSession? = null,
    val pendingRequestId: Long = 0,
    val attempts: List<ChatRequestAttempt> = emptyList(),
)

private fun ChatState.finishAttempt(generation: Long, outcome: ChatRequestOutcome): ChatState =
    copy(
        pendingRequestId = if (pendingRequestId == generation) 0 else pendingRequestId,
        attempts =
            attempts.map { attempt ->
              if (attempt.generation == generation && attempt.outcome == ChatRequestOutcome.Running)
                  attempt.copy(outcome = outcome)
              else attempt
            })

private fun DesktopState.matchesChatScope(scope: ChatRequestScope): Boolean {
  val relevantTask =
      preparedTaskSpec?.takeIf {
        scope.target.mode == ChatEditMode.ReplaceSymbol &&
            it.targetPath == scope.path &&
            it.targetSymbol == scope.target.symbol
      }
  return project?.let {
    it.projectId == scope.projectId && it.projectRevision == scope.projectRevision
  } == true &&
      selectedFile?.let { it.path == scope.path && it.contentHash == scope.baseFileHash } == true &&
      (scope.target.mode != ChatEditMode.ReplaceSymbol ||
          selectedSymbol == scope.declaration &&
              selectedSymbol?.name == scope.target.symbol &&
              selectedSymbol in symbols) &&
      sameTaskSpec(relevantTask, scope.taskSpec)
}

data class CheckCandidate(
    val projectId: String,
    val projectRevision: String,
    val path: String,
    val baseFileHash: String,
    val draftId: String,
    val revision: Long,
    val hash: String,
) {
  constructor(
      draft: DeclarationDraft
  ) : this(
      draft.projectId,
      draft.projectRevision,
      draft.targetPath,
      draft.baseFileHash,
      draft.id,
      draft.revision,
      draft.hash)
}

data class CheckAttempt(
    val requestId: Long,
    val candidate: CheckCandidate,
    val status: ValidationAttemptStatus,
    val message: String = "",
)

data class DraftReviewState(
    val checks: DraftCheckReport? = null,
    val draft: DeclarationDraft? = null,
    val editor: EditableDraftState? = null,
    val applied: ApplyResult? = null,
    val benchmark: BenchmarkEvidenceState = BenchmarkEvidenceState(),
    val checkAttempt: CheckAttempt? = null,
)

sealed interface BenchmarkDiscoveryOutcome {
  data object NotRequested : BenchmarkDiscoveryOutcome

  data object Loading : BenchmarkDiscoveryOutcome

  data object Loaded : BenchmarkDiscoveryOutcome

  data class Unavailable(val reason: String) : BenchmarkDiscoveryOutcome

  data class Failed(val message: String) : BenchmarkDiscoveryOutcome

  data object Invalidated : BenchmarkDiscoveryOutcome
}

sealed interface BenchmarkAdmissionOutcome {
  data object Idle : BenchmarkAdmissionOutcome

  data object Admitting : BenchmarkAdmissionOutcome

  data object Running : BenchmarkAdmissionOutcome

  data object Stopped : BenchmarkAdmissionOutcome

  data class Failed(val message: String) : BenchmarkAdmissionOutcome
}

enum class BenchmarkComparisonStatus {
  Completed,
  Canceled,
  Failed,
  Unavailable,
  Unsupported,
}

/** The daemon response is recorded verbatim, independently of retained measurements. */
data class BenchmarkComparisonOutcome(val response: GoBenchmarkComparison) {
  val status: BenchmarkComparisonStatus
    get() =
        when (response.status) {
          "completed" -> BenchmarkComparisonStatus.Completed
          "canceled" -> BenchmarkComparisonStatus.Canceled
          "failed" -> BenchmarkComparisonStatus.Failed
          "unavailable" -> BenchmarkComparisonStatus.Unavailable
          else -> BenchmarkComparisonStatus.Unsupported
        }
}

/** Catalog authority, latest outcome and retained measurements have independent lifecycles. */
data class BenchmarkEvidenceState(
    val catalog: GoBenchmarkCatalog? = null,
    val selected: GoBenchmarkChoice? = null,
    val comparison: GoBenchmarkComparison? = null,
    val latestOutcome: BenchmarkComparisonOutcome? = null,
    val discovery: BenchmarkDiscoveryOutcome = BenchmarkDiscoveryOutcome.NotRequested,
    val admission: BenchmarkAdmissionOutcome = BenchmarkAdmissionOutcome.Idle,
) {
  /** Admission is active before execution starts as well as while comparison is running. */
  val running: Boolean
    get() =
        admission == BenchmarkAdmissionOutcome.Admitting ||
            admission == BenchmarkAdmissionOutcome.Running
}

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
    val verifiedScan: VerifiedScanState = VerifiedScanState(),
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

  data class ProjectOpeningStarted(val attempt: ProjectOpeningAttempt) : DesktopEvent

  data class ProjectOpeningCanceled(val requestId: Long) : DesktopEvent

  data class RememberedProjectRead(val path: String?, val warning: String? = null) : DesktopEvent

  data class ProjectPreferenceSaved(val path: String) : DesktopEvent

  data class ProjectPreferenceSaveFailed(val message: String) : DesktopEvent

  data class IndexRefreshed(val index: ProjectIndex) : DesktopEvent

  data class ProjectIndexingStarted(val attempt: ProjectIndexingAttempt) : DesktopEvent

  data class ProjectIndexingCompleted(
      val attempt: ProjectIndexingAttempt,
      val index: ProjectIndex
  ) : DesktopEvent

  data class ProjectIndexingStopped(
      val attempt: ProjectIndexingAttempt,
      val outcome: ProjectIndexingOutcome,
  ) : DesktopEvent

  data class OverviewLoaded(val overview: ProjectOverview) : DesktopEvent

  data class ProjectDetailsUpdated(val outcome: ProjectDetailsOutcome) : DesktopEvent

  data class FindingsLoaded(val findings: List<UnifiedFinding>) : DesktopEvent

  data class FindingStatusUpdated(val findingId: String, val status: String) : DesktopEvent

  data class AnalysisRunUpdated(val state: ProjectAnalysisRunState) : DesktopEvent

  data class AnalyzeAllLoaded(val job: AnalyzeAllJob?) : DesktopEvent

  data class PerformanceLoaded(val job: PerformanceJob?, val report: PerformanceReport?) :
      DesktopEvent

  data class PerformanceContextLoaded(val context: PerformanceQueuePreview) : DesktopEvent

  data class GoScanLoaded(val scan: GoScanReport?) : DesktopEvent

  data class VerifiedScanReadUpdated(val read: VerifiedScanRead) : DesktopEvent

  data class VerifiedScanFindingsUpdated(val outcome: VerifiedScanFindingsRefresh) : DesktopEvent

  data class VerifiedScanOperationUpdated(val operation: VerifiedScanOperation) : DesktopEvent

  data class SecurityActionStarted(val action: String) : DesktopEvent

  data object SecurityActionCanceled : DesktopEvent

  data class SecurityReportLoaded(val report: SecurityFileReport) : DesktopEvent

  data class SecurityActionFailed(val action: String, val message: String) : DesktopEvent

  data class FileLoaded(val file: ProjectFileInfo, val symbols: List<SymbolInfo>) : DesktopEvent

  sealed interface LocalReadFailure : DesktopEvent {
    val message: String
  }

  data class FileLoadFailed(override val message: String, val path: String) : LocalReadFailure

  data class ProjectLoadFailed(val requestId: Long, val message: String) : DesktopEvent

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

  data class ChecksStarted(val requestId: Long, val candidate: CheckCandidate) : DesktopEvent

  data class ChecksCompleted(val requestId: Long, val checks: DraftCheckReport) : DesktopEvent

  data class ChecksStopped(
      val requestId: Long,
      val status: ValidationAttemptStatus,
      val message: String,
  ) : DesktopEvent

  data object GoBenchmarkDiscoveryStarted : DesktopEvent

  data object GoBenchmarkDiscoveryInvalidated : DesktopEvent

  data class GoBenchmarkDiscoveryFailed(val message: String) : DesktopEvent

  data class GoBenchmarkCatalogLoaded(val catalog: GoBenchmarkCatalog) : DesktopEvent

  data class GoBenchmarkSelected(val choice: GoBenchmarkChoice) : DesktopEvent

  data object GoBenchmarkAdmissionStarted : DesktopEvent

  data object GoBenchmarkComparisonStarted : DesktopEvent

  data class GoBenchmarkComparisonFailed(val message: String) : DesktopEvent

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

  data class DraftValidationStarted(val requestId: Long) : DesktopEvent

  data class DraftValidationUpdated(val requestId: Long, val draft: DeclarationDraft) :
      DesktopEvent

  data class DraftValidationStopped(
      val requestId: Long,
      val status: ValidationAttemptStatus,
      val message: String,
  ) : DesktopEvent

  data object DraftMarkedStale : DesktopEvent

  data class DraftLoaded(val draft: DeclarationDraft) : DesktopEvent

  data object DraftDiscarded : DesktopEvent

  data class Applied(val result: ApplyResult?) : DesktopEvent

  data class Failed(val message: String) : DesktopEvent

  data class Status(val message: String) : DesktopEvent
}

fun DesktopState.reduce(event: DesktopEvent): DesktopState =
    when (event) {
      DesktopEvent.Loading -> copy(jobs = jobs.copy(loading = true, error = null))
      is DesktopEvent.WorkspaceSelected -> copy(workspace = event.workspace)
      is DesktopEvent.ConnectionUpdated -> copy(connection = event.connection)
      is DesktopEvent.RememberedProjectRead,
      is DesktopEvent.ProjectPreferenceSaved,
      is DesktopEvent.ProjectPreferenceSaveFailed,
      is DesktopEvent.ProjectOpeningStarted,
      is DesktopEvent.ProjectOpeningCanceled,
      is DesktopEvent.ProjectLoadFailed,
      is DesktopEvent.ProjectLoaded,
      is DesktopEvent.ProjectIndexingStarted,
      is DesktopEvent.ProjectIndexingCompleted,
      is DesktopEvent.ProjectIndexingStopped -> withProjectWorkspaceEvent(event)
      is DesktopEvent.LocalReadFailure -> withLocalReadFailure(event)
      is DesktopEvent.IndexRefreshed -> withRefreshedIndex(event.index)
      is DesktopEvent.OverviewLoaded ->
          copy(projectState = projectState.copy(overview = event.overview))
      is DesktopEvent.ProjectDetailsUpdated ->
          copy(projectState = projectState.copy(detailsOutcome = event.outcome))
      is DesktopEvent.FindingsLoaded -> copy(findings = findings.copy(findings = event.findings))
      is DesktopEvent.FindingStatusUpdated -> withFindingStatus(event)
      is DesktopEvent.AnalysisRunUpdated -> withAnalysisRunUpdate(event.state)
      is DesktopEvent.AnalyzeAllLoaded -> copy(findings = findings.copy(analyzeAll = event.job))
      is DesktopEvent.PerformanceLoaded ->
          copy(
              findings =
                  findings.copy(performanceJob = event.job, performanceReport = event.report))
      is DesktopEvent.PerformanceContextLoaded ->
          copy(findings = findings.copy(performanceContext = event.context))
      is DesktopEvent.GoScanLoaded,
      is DesktopEvent.VerifiedScanReadUpdated,
      is DesktopEvent.VerifiedScanFindingsUpdated,
      is DesktopEvent.VerifiedScanOperationUpdated -> withVerifiedScanEvent(event)
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
      is DesktopEvent.ChecksStarted,
      is DesktopEvent.ChecksCompleted,
      is DesktopEvent.ChecksStopped,
      is DesktopEvent.ChecksLoaded -> withCheckEvent(event)
      DesktopEvent.GoBenchmarkDiscoveryStarted,
      DesktopEvent.GoBenchmarkDiscoveryInvalidated,
      is DesktopEvent.GoBenchmarkDiscoveryFailed,
      is DesktopEvent.GoBenchmarkCatalogLoaded,
      is DesktopEvent.GoBenchmarkSelected,
      DesktopEvent.GoBenchmarkAdmissionStarted,
      DesktopEvent.GoBenchmarkComparisonStarted,
      is DesktopEvent.GoBenchmarkComparisonFailed,
      is DesktopEvent.GoBenchmarkComparisonLoaded,
      DesktopEvent.GoBenchmarkComparisonStopped -> withBenchmarkEvent(event)
      is DesktopEvent.ChatLoaded -> copy(chat = chat.copy(session = event.session))
      is DesktopEvent.ChatProposalLoaded -> {
        val messages =
            event.session.messages +
                ChatSessionMessage(role = "user", content = event.userMessage) +
                event.proposal.assistantMessage
        copy(
            chat =
                chat.copy(
                    session =
                        event.session.copy(
                            latestDraftId = event.proposal.draft.id, messages = messages)),
            review =
                review.copy(
                    draft = event.proposal.draft,
                    editor = editableDraft(event.proposal.draft),
                    checks = null,
                    checkAttempt = null,
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
                        checkAttempt = null,
                        benchmark = review.benchmark.withoutCatalog()),
                jobs = jobs.copy(loading = false))
          } ?: this
      is DesktopEvent.DraftValidationStarted,
      is DesktopEvent.DraftValidationUpdated,
      is DesktopEvent.DraftValidationStopped -> withValidationEvent(event)
      DesktopEvent.DraftMarkedStale -> withStaleDraft()
      is DesktopEvent.DraftLoaded ->
          copy(
              review =
                  review.copy(
                      draft = event.draft,
                      editor =
                          editableDraft(event.draft)
                              .copy(
                                  acceptanceGeneration =
                                      (review.editor?.acceptanceGeneration ?: 0) + 1),
                      checks = null,
                      checkAttempt = null,
                      benchmark =
                          if (review.draft?.id == event.draft.id) review.benchmark.withoutCatalog()
                          else BenchmarkEvidenceState()),
              jobs = jobs.copy(loading = false))
      DesktopEvent.DraftDiscarded ->
          copy(
              chat = ChatState(),
              review = DraftReviewState(applied = review.applied),
              jobs = jobs.copy(loading = false, error = null))
      is DesktopEvent.Applied ->
          copy(
              review =
                  review.copy(
                      applied = event.result, benchmark = review.benchmark.withoutCatalog()),
              jobs = jobs.copy(loading = false))
      is DesktopEvent.Failed -> copy(jobs = jobs.copy(loading = false, error = event.message))
      is DesktopEvent.Status -> copy(jobs = jobs.copy(status = event.message))
    }

private fun DesktopState.withVerifiedScanEvent(event: DesktopEvent): DesktopState =
    when (event) {
      is DesktopEvent.GoScanLoaded ->
          copy(
              findings = if (event.scan == null) findings else findings.copy(scan = event.scan),
              verifiedScan =
                  verifiedScan.copy(
                      read =
                          if (event.scan == null) VerifiedScanRead.Absent
                          else VerifiedScanRead.Loaded))
      is DesktopEvent.VerifiedScanReadUpdated ->
          copy(verifiedScan = verifiedScan.copy(read = event.read))
      is DesktopEvent.VerifiedScanOperationUpdated ->
          copy(verifiedScan = verifiedScan.copy(operation = event.operation))
      is DesktopEvent.VerifiedScanFindingsUpdated ->
          copy(verifiedScan = verifiedScan.copy(findingsRefresh = event.outcome))
      else -> this
    }

private fun DesktopState.withStaleDraft(): DesktopState =
    review.editor?.let { editor ->
      copy(
          review =
              review.copy(
                  draft = editor.serverDraft.copy(validation = null),
                  editor =
                      editor.copy(
                          serverDraft = editor.serverDraft.copy(validation = null),
                          retainedValidation =
                              editor.serverDraft.validation ?: editor.retainedValidation,
                          status = DraftEditorStatus.Stale,
                          validationAttempt = null),
                  checks = null,
                  checkAttempt = null,
                  benchmark = review.benchmark.withoutCatalog()),
          jobs = jobs.copy(loading = false))
    } ?: this

private fun DesktopState.withCheckEvent(event: DesktopEvent): DesktopState =
    when (event) {
      is DesktopEvent.ChecksLoaded ->
          copy(
              review = review.copy(checks = event.checks, checkAttempt = null),
              jobs = jobs.copy(loading = false, error = null))
      is DesktopEvent.ChecksStarted ->
          if (review.draft?.let(::CheckCandidate) == event.candidate)
              copy(
                  review =
                      review.copy(
                          checkAttempt =
                              CheckAttempt(
                                  event.requestId,
                                  event.candidate,
                                  ValidationAttemptStatus.Running)))
          else this
      is DesktopEvent.ChecksCompleted ->
          if (currentCheckAttempt(event.requestId))
              copy(
                  review = review.copy(checks = event.checks, checkAttempt = null),
                  jobs = jobs.copy(loading = false, error = null))
          else this
      is DesktopEvent.ChecksStopped ->
          if (currentCheckAttempt(event.requestId))
              copy(
                  review =
                      review.copy(
                          checkAttempt =
                              review.checkAttempt?.copy(
                                  status = event.status, message = event.message)),
                  jobs = jobs.copy(loading = false))
          else this
      else -> this
    }

private fun DesktopState.currentCheckAttempt(requestId: Long): Boolean =
    review.checkAttempt?.let {
      it.requestId == requestId &&
          it.status == ValidationAttemptStatus.Running &&
          review.draft?.let(::CheckCandidate) == it.candidate
    } == true

private fun DesktopState.withLoadedProject(event: DesktopEvent.ProjectLoaded): DesktopState =
    copy(
        workspace = Workspace.Summary,
        projectState =
            projectState.copy(
                project = event.project,
                index = event.index,
                overview = null,
                detailsOutcome = null,
                sourceChangeObserved = false,
                openingAttempt = null,
                indexingAttempt = null,
                preferenceSaveWarning = null),
        selection = FileSelectionState(),
        findings = FindingsState(),
        verifiedScan = VerifiedScanState(),
        analysisRun = ProjectAnalysisRunState(),
        security = SecurityWorkspaceState(),
        chat = ChatState(),
        review = DraftReviewState(),
        jobs = jobs.copy(loading = false, status = "Imported ${event.project.name}", error = null),
    )

private fun DesktopState.withProjectWorkspaceEvent(event: DesktopEvent): DesktopState =
    when (event) {
      is DesktopEvent.RememberedProjectRead ->
          copy(
              projectState =
                  projectState.copy(
                      rememberedPath = event.path, preferenceReadWarning = event.warning))
      is DesktopEvent.ProjectPreferenceSaved ->
          copy(projectState = projectState.copy(rememberedPath = event.path))
      is DesktopEvent.ProjectPreferenceSaveFailed ->
          copy(projectState = projectState.copy(preferenceSaveWarning = event.message))
      is DesktopEvent.ProjectOpeningStarted ->
          copy(
              projectState =
                  projectState.copy(
                      openingAttempt = event.attempt,
                      indexingAttempt =
                          projectState.indexingAttempt?.let { attempt ->
                            if (attempt.outcome == ProjectIndexingOutcome.Running)
                                attempt.copy(outcome = ProjectIndexingOutcome.Canceled)
                            else attempt
                          }))
      is DesktopEvent.ProjectLoaded -> withLoadedProject(event)
      is DesktopEvent.ProjectOpeningCanceled ->
          stopProjectOpening(event.requestId, ProjectOpeningOutcome.Canceled)
      is DesktopEvent.ProjectLoadFailed ->
          stopProjectOpening(event.requestId, ProjectOpeningOutcome.Failed(event.message))
      is DesktopEvent.ProjectIndexingStarted,
      is DesktopEvent.ProjectIndexingCompleted,
      is DesktopEvent.ProjectIndexingStopped -> withProjectIndexingEvent(event)
      else -> this
    }

private fun DesktopState.stopProjectOpening(
    requestId: Long,
    outcome: ProjectOpeningOutcome
): DesktopState {
  val attempt = projectState.openingAttempt ?: return this
  if (attempt.requestId != requestId || attempt.outcome != ProjectOpeningOutcome.Opening)
      return this
  return copy(
      projectState = projectState.copy(openingAttempt = attempt.copy(outcome = outcome)),
      jobs =
          jobs.copy(loading = false, error = (outcome as? ProjectOpeningOutcome.Failed)?.message))
}

private fun DesktopState.withLocalReadFailure(event: DesktopEvent.LocalReadFailure): DesktopState =
    when (event) {
      is DesktopEvent.FileLoadFailed ->
          copy(
              selection =
                  selection.copy(
                      pendingFilePath = null,
                      failedFilePath = event.path,
                      fileReadError = event.message),
              jobs = jobs.copy(loading = false, error = event.message))
    }

private fun DesktopState.withValidationEvent(event: DesktopEvent): DesktopState =
    when (event) {
      is DesktopEvent.DraftValidationStarted -> withValidationStarted(event.requestId)
      is DesktopEvent.DraftValidationUpdated -> withValidationUpdated(event)
      is DesktopEvent.DraftValidationStopped -> withValidationStopped(event)
      else -> this
    }

private fun DesktopState.withValidationStarted(requestId: Long): DesktopState =
    review.editor?.let { editor ->
      copy(
          review =
              review.copy(
                  draft = editor.serverDraft.copy(validation = null),
                  editor =
                      editor.copy(
                          serverDraft = editor.serverDraft.copy(validation = null),
                          retainedValidation =
                              editor.serverDraft.validation ?: editor.retainedValidation,
                          status = DraftEditorStatus.Validating,
                          validationAttempt =
                              ValidationAttempt(requestId, ValidationAttemptStatus.Running)),
                  checks = null,
                  checkAttempt = null,
                  benchmark = review.benchmark.withoutCatalog()),
          jobs = jobs.copy(loading = false))
    } ?: this

private fun DesktopState.withValidationUpdated(
    event: DesktopEvent.DraftValidationUpdated
): DesktopState =
    review.editor
        ?.takeIf {
          it.validationAttempt ==
              ValidationAttempt(event.requestId, ValidationAttemptStatus.Running)
        }
        ?.let { editor ->
          copy(
              review =
                  review.copy(
                      draft = event.draft.copy(validation = null),
                      editor = editor.copy(serverDraft = event.draft.copy(validation = null))))
        } ?: this

private fun DesktopState.withValidationStopped(
    event: DesktopEvent.DraftValidationStopped
): DesktopState =
    review.editor
        ?.takeIf {
          it.status == DraftEditorStatus.Validating &&
              it.validationAttempt?.requestId == event.requestId &&
              it.validationAttempt.status == ValidationAttemptStatus.Running
        }
        ?.let { editor ->
          copy(
              review =
                  review.copy(
                      editor =
                          editor.copy(
                              status =
                                  if (editor.unvalidatedLocalEdits) DraftEditorStatus.Dirty
                                  else DraftEditorStatus.Generated,
                              validationAttempt =
                                  ValidationAttempt(event.requestId, event.status, event.message))))
        } ?: this

private fun DesktopState.withBenchmarkEvent(event: DesktopEvent): DesktopState {
  val benchmark = review.benchmark
  val updated =
      when (event) {
        DesktopEvent.GoBenchmarkDiscoveryStarted ->
            benchmark
                .withoutCatalog()
                .copy(
                    discovery = BenchmarkDiscoveryOutcome.Loading,
                    admission =
                        if (benchmark.running ||
                            benchmark.admission == BenchmarkAdmissionOutcome.Stopped)
                            BenchmarkAdmissionOutcome.Stopped
                        else BenchmarkAdmissionOutcome.Idle)
        DesktopEvent.GoBenchmarkDiscoveryInvalidated -> benchmark.withoutCatalog()
        is DesktopEvent.GoBenchmarkDiscoveryFailed ->
            benchmark
                .withoutCatalog()
                .copy(discovery = BenchmarkDiscoveryOutcome.Failed(event.message))
        is DesktopEvent.GoBenchmarkCatalogLoaded ->
            benchmark.copy(
                catalog = event.catalog,
                selected = null,
                discovery =
                    if (event.catalog.available) BenchmarkDiscoveryOutcome.Loaded
                    else BenchmarkDiscoveryOutcome.Unavailable(event.catalog.reason),
                admission =
                    if (benchmark.running) BenchmarkAdmissionOutcome.Stopped
                    else BenchmarkAdmissionOutcome.Idle)
        is DesktopEvent.GoBenchmarkSelected ->
            if (benchmark.selected == event.choice) benchmark
            else
                benchmark.copy(
                    selected = event.choice,
                    admission =
                        if (benchmark.running) BenchmarkAdmissionOutcome.Stopped
                        else BenchmarkAdmissionOutcome.Idle)
        DesktopEvent.GoBenchmarkAdmissionStarted ->
            benchmark.copy(admission = BenchmarkAdmissionOutcome.Admitting, latestOutcome = null)
        DesktopEvent.GoBenchmarkComparisonStarted ->
            benchmark.copy(admission = BenchmarkAdmissionOutcome.Running, latestOutcome = null)
        is DesktopEvent.GoBenchmarkComparisonFailed ->
            benchmark.copy(admission = BenchmarkAdmissionOutcome.Failed(event.message))
        is DesktopEvent.GoBenchmarkComparisonLoaded ->
            benchmark.copy(
                comparison =
                    if (event.comparison.base?.samples?.isNotEmpty() == true ||
                        event.comparison.candidate?.samples?.isNotEmpty() == true)
                        event.comparison
                    else benchmark.comparison,
                latestOutcome = BenchmarkComparisonOutcome(event.comparison),
                admission = BenchmarkAdmissionOutcome.Idle)
        DesktopEvent.GoBenchmarkComparisonStopped ->
            benchmark.copy(admission = BenchmarkAdmissionOutcome.Stopped)
        else -> return this
      }
  val clearsError =
      event is DesktopEvent.GoBenchmarkCatalogLoaded ||
          event == DesktopEvent.GoBenchmarkAdmissionStarted ||
          event == DesktopEvent.GoBenchmarkComparisonStarted ||
          event is DesktopEvent.GoBenchmarkComparisonLoaded
  return copy(
      review = review.copy(benchmark = updated),
      jobs = jobs.copy(loading = updated.running, error = if (clearsError) null else jobs.error))
}

private fun BenchmarkEvidenceState.withoutCatalog(): BenchmarkEvidenceState =
    copy(
        catalog = null,
        selected = null,
        discovery =
            if (discovery == BenchmarkDiscoveryOutcome.NotRequested)
                BenchmarkDiscoveryOutcome.NotRequested
            else BenchmarkDiscoveryOutcome.Invalidated,
        admission = if (running) BenchmarkAdmissionOutcome.Stopped else admission)

private fun DesktopState.withProjectIndexingEvent(event: DesktopEvent): DesktopState =
    when (event) {
      is DesktopEvent.ProjectIndexingStarted ->
          copy(projectState = projectState.copy(indexingAttempt = event.attempt))
      is DesktopEvent.ProjectIndexingCompleted -> withCompletedIndexing(event)
      is DesktopEvent.ProjectIndexingStopped -> withStoppedIndexing(event)
      else -> this
    }

private fun DesktopState.matchesIndexing(attempt: ProjectIndexingAttempt): Boolean =
    projectState.indexingAttempt == attempt &&
        attempt.outcome == ProjectIndexingOutcome.Running &&
        projectState.openingAttempt?.outcome != ProjectOpeningOutcome.Opening &&
        project?.let {
          it.projectId == attempt.projectId &&
              it.projectRevision == attempt.projectRevision &&
              it.path == attempt.path
        } == true

private fun DesktopState.withCompletedIndexing(
    event: DesktopEvent.ProjectIndexingCompleted
): DesktopState {
  if (!matchesIndexing(event.attempt)) return this
  if (event.index.projectId != event.attempt.projectId || event.index.projectRevision.isBlank())
      return withStoppedIndexing(
          DesktopEvent.ProjectIndexingStopped(
              event.attempt,
              ProjectIndexingOutcome.Failed(
                  "Re-index returned a mismatched project or revision. Try re-indexing again.")))
  val refreshed = withRefreshedIndex(event.index)
  return refreshed.copy(
      projectState =
          refreshed.projectState.copy(
              indexingAttempt =
                  event.attempt.copy(
                      outcome = ProjectIndexingOutcome.Succeeded(event.index.projectRevision))))
}

private fun DesktopState.withStoppedIndexing(
    event: DesktopEvent.ProjectIndexingStopped
): DesktopState {
  if (!matchesIndexing(event.attempt) ||
      event.outcome is ProjectIndexingOutcome.Succeeded ||
      event.outcome == ProjectIndexingOutcome.Running)
      return this
  val outcome =
      if (event.outcome is ProjectIndexingOutcome.Failed && event.outcome.message.isBlank())
          ProjectIndexingOutcome.Failed("Could not re-index project. Try again.")
      else event.outcome
  return copy(
      projectState = projectState.copy(indexingAttempt = event.attempt.copy(outcome = outcome)))
}

private fun DesktopState.withRefreshedIndex(index: ProjectIndex): DesktopState {
  val loaded = project ?: return this
  if (loaded.projectId != index.projectId || index.projectRevision.isBlank()) return this
  val revisionChanged = loaded.projectRevision != index.projectRevision
  val current = if (revisionChanged) reduce(DesktopEvent.DraftMarkedStale) else this
  return current.copy(
      projectState =
          projectState.copy(
              project = loaded.copy(projectRevision = index.projectRevision),
              index = index,
              sourceChangeObserved = false),
      analysisRun = analysisRun.afterRevisionChange(revisionChanged),
      verifiedScan = if (revisionChanged) VerifiedScanState() else verifiedScan,
      chat = if (revisionChanged) ChatState() else chat,
      review =
          if (revisionChanged)
              current.review.copy(
                  draft = if (current.review.editor == null) null else current.review.draft,
                  checks = null,
                  checkAttempt = null,
                  benchmark = current.review.benchmark.withoutCatalog())
          else review,
      jobs = jobs.copy(loading = false, status = "Project inventory refreshed", error = null),
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
  // Loaded-file authority remains valid while a separate replacement read is pending.
  private var fileRequest: RequestIdentity? = null
  private var pendingFileRequest: RequestIdentity? = null
  private var analysisRequest: Long = 0
  private var chatRequest: Long = 0
  private var draftRevision: Long = 0
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
    if (event is DesktopEvent.DraftEdited ||
        event == DesktopEvent.DraftMarkedStale ||
        event is DesktopEvent.ChatProposalLoaded ||
        event is DesktopEvent.DraftLoaded ||
        event is DesktopEvent.ProjectLoaded ||
        event is DesktopEvent.FileLoaded) {
      draftRequest = 0
    }
    val previousProject = state.project
    val previousAttempt =
        state.chat.attempts.lastOrNull { it.outcome == ChatRequestOutcome.Running }
    if (event is DesktopEvent.DraftEdited ||
        event is DesktopEvent.DraftLoaded ||
        event is DesktopEvent.DraftValidationUpdated ||
        event == DesktopEvent.DraftMarkedStale ||
        event == DesktopEvent.DraftDiscarded)
        draftRevision++
    state = state.reduce(event)
    if (previousAttempt != null &&
        event !is DesktopEvent.ChatProposalLoaded &&
        event !is DesktopEvent.ChatLoaded &&
        (!state.matchesChatScope(previousAttempt.scope) ||
            previousAttempt.admittedDraftRevision != draftRevision)) {
      chatRequest = 0
      state =
          state.copy(
              chat =
                  state.chat.copy(
                      attempts =
                          state.chat.attempts.map { attempt ->
                            if (attempt.generation == state.chat.pendingRequestId &&
                                attempt.outcome == ChatRequestOutcome.Running)
                                attempt.copy(
                                    outcome = ChatRequestOutcome.Canceled,
                                    invalidationReason =
                                        if (previousAttempt.admittedDraftRevision != draftRevision)
                                            "The draft changed during the request."
                                        else "The request target changed.")
                            else attempt
                          },
                      pendingRequestId = 0))
    }
    if (event is DesktopEvent.ProjectLoaded ||
        previousProject?.projectId != state.project?.projectId ||
        previousProject?.projectRevision != state.project?.projectRevision) {
      fileRequest = null
      pendingFileRequest = null
      state = state.copy(selection = state.selection.copy(pendingFilePath = null))
    }
    return state
  }

  fun beginProjectLoad(
      path: String = "",
      kind: ProjectOpeningKind = ProjectOpeningKind.Import
  ): Long =
      nextId().also { requestId ->
        fileRequest = null
        pendingFileRequest = null
        dispatch(DesktopEvent.ProjectOpeningStarted(ProjectOpeningAttempt(requestId, path, kind)))
        dispatch(DesktopEvent.Loading)
      }

  fun beginProjectIndexing(): ProjectIndexingAttempt? {
    val project = state.project ?: return null
    if (state.projectState.openingAttempt?.outcome == ProjectOpeningOutcome.Opening ||
        state.projectState.indexingAttempt?.outcome == ProjectIndexingOutcome.Running)
        return null
    return ProjectIndexingAttempt(
            nextId(), project.projectId, project.projectRevision, project.path)
        .also { dispatch(DesktopEvent.ProjectIndexingStarted(it)) }
  }

  fun projectIndexingCompleted(attempt: ProjectIndexingAttempt, index: ProjectIndex): Boolean {
    if (!state.matchesIndexing(attempt)) return false
    dispatch(DesktopEvent.ProjectIndexingCompleted(attempt, index))
    return state.projectState.indexingAttempt?.outcome is ProjectIndexingOutcome.Succeeded
  }

  fun projectIndexingFailed(attempt: ProjectIndexingAttempt, message: String): Boolean =
      if (state.matchesIndexing(attempt))
          accept(
              DesktopEvent.ProjectIndexingStopped(attempt, ProjectIndexingOutcome.Failed(message)))
      else false

  fun cancelProjectIndexing(attempt: ProjectIndexingAttempt): Boolean =
      if (state.matchesIndexing(attempt))
          accept(DesktopEvent.ProjectIndexingStopped(attempt, ProjectIndexingOutcome.Canceled))
      else false

  fun projectLoaded(requestId: Long, project: ProjectAnalysis, index: ProjectIndex): Boolean {
    if (!isCurrentProjectRequest(requestId)) return false
    if (project.projectId != index.projectId || project.projectRevision != index.projectRevision) {
      projectFailed(
          requestId, "Project and index identity do not match. Try opening the project again.")
      return false
    }
    return accept(DesktopEvent.ProjectLoaded(project, index))
  }

  fun isCurrentProjectRequest(requestId: Long): Boolean =
      state.projectState.openingAttempt?.let {
        it.requestId == requestId && it.outcome == ProjectOpeningOutcome.Opening
      } == true

  fun projectFailed(requestId: Long, message: String): Boolean =
      if (isCurrentProjectRequest(requestId))
          accept(DesktopEvent.ProjectLoadFailed(requestId, message))
      else false

  fun cancelProjectLoad(requestId: Long): Boolean =
      if (isCurrentProjectRequest(requestId)) accept(DesktopEvent.ProjectOpeningCanceled(requestId))
      else false

  fun beginFileLoad(path: String): RequestIdentity? {
    val project = state.project ?: return null
    val request = RequestIdentity(nextId(), project.projectId, project.projectRevision, path)
    pendingFileRequest = request
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("Loading $path…"))
    state =
        state.copy(
            selection =
                state.selection.copy(
                    pendingFilePath = path, failedFilePath = null, fileReadError = null))
    return request
  }

  fun fileLoaded(
      request: RequestIdentity,
      file: ProjectFileInfo,
      symbols: List<SymbolInfo>
  ): Boolean {
    val resolved = request.copy(contentHash = file.contentHash)
    if (!isCurrentFileLoad(request) || file.path != request.path) return false
    pendingFileRequest = null
    fileRequest = resolved
    analysisRequest = 0
    chatRequest = 0
    dispatch(DesktopEvent.FileLoaded(file, symbols))
    return true
  }

  internal fun fileNavigationLoaded(
      request: RequestIdentity,
      file: ProjectFileInfo,
      symbols: List<SymbolInfo>,
      identity: FileNavigationIdentity,
  ): Boolean =
      state.matchesFileNavigation(request.path, identity) && fileLoaded(request, file, symbols)

  fun fileFailed(request: RequestIdentity, message: String): Boolean =
      if (isCurrentFileLoad(request)) {
        pendingFileRequest = null
        accept(DesktopEvent.FileLoadFailed(message, request.path))
      } else false

  fun cancelFileLoad(request: RequestIdentity): Boolean =
      if (isCurrentFileLoad(request)) {
        pendingFileRequest = null
        dispatch(DesktopEvent.Status("File load canceled"))
        state =
            state.copy(
                selection = state.selection.copy(pendingFilePath = null),
                jobs = state.jobs.copy(loading = false))
        true
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

  // Existing session reads do not create submitted request turns.
  fun beginChatLoad(): Pair<Long, RequestIdentity>? =
      fileRequest
          ?.takeUnless {
            state.chat.attempts.any { attempt -> attempt.outcome == ChatRequestOutcome.Running }
          }
          ?.let { file -> nextId().also { chatRequest = it } to file }

  fun beginChatAttempt(
      scope: ChatRequestScope,
      destination: ScopedModel,
      requestText: String,
      remoteConfirmed: Boolean = false,
      creationKind: String? = null,
  ): Pair<Long, RequestIdentity>? {
    val file = fileRequest ?: return null
    if (!matchesFile(file) ||
        !state.matchesChatScope(scope) ||
        state.chat.attempts.any { it.outcome == ChatRequestOutcome.Running })
        return null
    val generation = nextId()
    chatRequest = generation
    state =
        state.copy(
            chat =
                state.chat.copy(
                    pendingRequestId = generation,
                    attempts =
                        state.chat.attempts +
                            ChatRequestAttempt(
                                generation,
                                scope,
                                destination,
                                remoteConfirmed,
                                requestText,
                                draftRevision,
                                creationKind = creationKind)))
    return generation to file
  }

  fun isCurrentChatAttempt(requestId: Long, file: RequestIdentity): Boolean =
      currentChatAttempt(requestId, file) != null

  fun chatAttemptFailed(requestId: Long, file: RequestIdentity, message: String): Boolean {
    val attempt = currentChatAttempt(requestId, file) ?: return false
    chatRequest = 0
    state =
        state.copy(
            chat = state.chat.finishAttempt(requestId, ChatRequestOutcome.Failed(message)),
            jobs =
                state.jobs.copy(
                    loading = false,
                    error = message,
                    status = "Request failed for ${attempt.scope.target.symbol}."))
    return true
  }

  private fun currentChatAttempt(
      requestId: Long,
      file: RequestIdentity,
  ): ChatRequestAttempt? =
      state.chat.attempts.lastOrNull()?.takeIf {
        requestId == chatRequest &&
            requestId == state.chat.pendingRequestId &&
            it.generation == requestId &&
            it.outcome == ChatRequestOutcome.Running &&
            matchesFile(file) &&
            state.matchesChatScope(it.scope) &&
            it.admittedDraftRevision == draftRevision
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
      if ((state.chat.pendingRequestId == 0L ||
          currentChatAttempt(requestId, file)?.let {
            it.requestText == userMessage &&
                it.scope.matches(session) &&
                it.scope.matchesDraft(proposal.draft)
          } == true) &&
          requestId == chatRequest &&
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
              proposal.draft.baseFileHash)) {
        val published = accept(DesktopEvent.ChatProposalLoaded(session, userMessage, proposal))
        if (published && state.chat.pendingRequestId == requestId)
            state =
                state.copy(
                    chat =
                        state.chat.finishAttempt(
                            requestId, ChatRequestOutcome.Succeeded(session.id, proposal.draft.id)))
        chatRequest = 0
        published
      } else false

  fun cancelChatLoad(requestId: Long, file: RequestIdentity): Boolean =
      if (requestId == chatRequest && matchesFile(file)) {
        chatRequest = 0
        state = state.copy(chat = state.chat.finishAttempt(requestId, ChatRequestOutcome.Canceled))
        accept(DesktopEvent.Status("Chat request canceled"))
      } else false

  fun beginDraftLoad(): Pair<Long, RequestIdentity>? =
      fileRequest?.let { file -> nextId().also { draftRequest = it } to file }

  fun beginDraftValidation(): Pair<Long, RequestIdentity>? {
    if (state.review.editor?.status == DraftEditorStatus.Validating) return null
    return beginDraftLoad()?.also { (request, _) ->
      dispatch(DesktopEvent.DraftValidationStarted(request))
    }
  }

  fun draftValidationStopped(
      requestId: Long,
      file: RequestIdentity,
      status: ValidationAttemptStatus,
      message: String,
  ): Boolean =
      if (currentValidation(requestId, file)) {
        draftRequest = 0
        accept(DesktopEvent.DraftValidationStopped(requestId, status, message))
      } else false

  fun draftValidationUpdated(
      requestId: Long,
      file: RequestIdentity,
      draft: DeclarationDraft,
  ): Boolean =
      if (currentValidation(requestId, file) &&
          state.review.editor?.serverDraft?.let {
            it.id == draft.id &&
                it.projectId == draft.projectId &&
                it.projectRevision == draft.projectRevision &&
                it.targetPath == draft.targetPath &&
                it.baseFileHash == draft.baseFileHash
          } == true)
          accept(DesktopEvent.DraftValidationUpdated(requestId, draft))
      else false

  fun draftValidated(requestId: Long, file: RequestIdentity, draft: DeclarationDraft): Boolean =
      if (currentValidation(requestId, file) &&
          state.review.editor?.serverDraft?.let {
            it.id == draft.id &&
                draft.revision == it.revision &&
                it.hash == draft.hash &&
                it.declaration == draft.declaration &&
                it.imports == draft.imports &&
                it.mode == draft.mode &&
                it.targetSymbol == draft.targetSymbol
          } == true)
          draftLoaded(requestId, file, draft)
      else false

  private fun currentValidation(requestId: Long, file: RequestIdentity): Boolean =
      requestId == draftRequest &&
          matchesFile(file) &&
          state.review.editor?.validationAttempt ==
              ValidationAttempt(requestId, ValidationAttemptStatus.Running)

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
          checks.targetPath == draft.targetPath &&
          checks.draftId == draft.id &&
          checks.draftRevision == draft.revision &&
          checks.draftHash == draft.hash)
          currentCheck(requestId, file, draft) &&
              accept(DesktopEvent.ChecksCompleted(requestId, checks))
      else false

  fun beginDraftChecks(draft: DeclarationDraft): Pair<Long, RequestIdentity>? {
    if (state.review.checkAttempt?.status == ValidationAttemptStatus.Running) return null
    return beginDraftLoad()?.also { (request, _) ->
      dispatch(DesktopEvent.ChecksStarted(request, CheckCandidate(draft)))
    }
  }

  fun draftChecksStopped(
      requestId: Long,
      file: RequestIdentity,
      draft: DeclarationDraft,
      status: ValidationAttemptStatus,
      message: String,
  ): Boolean =
      if (currentCheck(requestId, file, draft)) {
        draftRequest = 0
        accept(DesktopEvent.ChecksStopped(requestId, status, message))
      } else false

  private fun currentCheck(
      requestId: Long,
      file: RequestIdentity,
      draft: DeclarationDraft
  ): Boolean =
      requestId == draftRequest &&
          matchesFile(file) &&
          state.review.draft?.let(::CheckCandidate) == CheckCandidate(draft) &&
          state.review.checkAttempt ==
              CheckAttempt(requestId, CheckCandidate(draft), ValidationAttemptStatus.Running)

  fun currentFileRequest(): RequestIdentity? = fileRequest

  fun currentPendingFileRequest(): RequestIdentity? = pendingFileRequest

  internal fun isCurrentFileLoad(request: RequestIdentity): Boolean =
      pendingFileRequest == request && matchesProject(request)

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
        checks.targetPath == draft.targetPath &&
        checks.draftId == draft.id &&
        checks.draftRevision == draft.revision &&
        checks.draftHash == draft.hash &&
        checks.checks.all { it.state.lowercase() in setOf("passed", "skipped") }

fun draftReviewEligibility(
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?,
    selectedFile: ProjectFileInfo?,
    project: ProjectAnalysis?,
    checkAttempt: CheckAttempt? = null,
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
  if (checkAttempt?.candidate == CheckCandidate(draft)) {
    val reason =
        when (checkAttempt.status) {
          ValidationAttemptStatus.Running -> "Wait for focused checks to finish."
          ValidationAttemptStatus.Failed ->
              "Focused checks failed to run. Run them again before Apply."
          ValidationAttemptStatus.Canceled ->
              "Focused checks were canceled. Run them again before Apply."
        }
    return ApplyEligibility(false, reason)
  }
  return draftApplyEligibility(draft, checks, selectedFile)
}

private fun DesktopState.withAnalysisRunUpdate(updated: ProjectAnalysisRunState): DesktopState {
  val obsolete =
      updated.previewIntent?.let { intent ->
        val run = updated.run
        intent.resumeRun != null &&
            (run?.identity != intent.resumeRun ||
                run.plan != intent.resumePlan ||
                run.status != analysisRun.run?.status ||
                AnalysisRunCommand.Resume !in analysisRunCommands(project, run)) ||
            analysisRun.run != null && run?.identity != analysisRun.run.identity
      } == true
  return copy(
      analysisRun =
          updated
              .copy(
                  previewIntent =
                      updated.previewIntent.takeUnless { updated.fileSelection.saving || obsolete },
                  admission = updated.admission.takeUnless { obsolete },
                  action = if (obsolete && updated.action == "preview") "" else updated.action)
              .afterRevisionChange(projectState.sourceChangeObserved))
}

private fun ProjectAnalysisRunState.afterRevisionChange(changed: Boolean): ProjectAnalysisRunState =
    if (changed)
        copy(
            admission = null,
            previewIntent = null,
            action = "",
            controlRequest = null,
            statusUnavailable = false,
            run = run?.copy(status = "stale"))
    else this

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
