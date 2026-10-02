package io.miniorca.desktop

import java.io.File
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext

data class WorkflowProjectIdentity(val id: String, val revision: String)

data class WorkflowFileIdentity(
    val project: WorkflowProjectIdentity,
    val path: String,
    val contentHash: String,
)

data class WorkflowTaskIdentity(
    val file: WorkflowFileIdentity,
    val mode: ChatEditMode,
    val symbol: String,
    val taskSpec: BugTaskSpec?,
)

data class WorkflowDraftIdentity(
    val file: WorkflowFileIdentity,
    val id: String,
    val revision: Long,
    val hash: String,
)

data class DeclarationExplanationTarget(
    val file: WorkflowFileIdentity,
    val symbol: String,
    val signature: String,
    val startLine: Int,
    val endLine: Int,
)

enum class DeclarationExplanationStatus {
  Unavailable,
  Loading,
  Current,
  Stale,
  Canceled,
  Failed,
}

data class DeclarationExplanationState(
    val status: DeclarationExplanationStatus = DeclarationExplanationStatus.Unavailable,
    val target: DeclarationExplanationTarget? = null,
    val result: DeclarationExplanation? = null,
    val message: String = "No on-demand explanation has been requested.",
)

enum class ContextInspectionStatus {
  Closed,
  Loading,
  Ready,
  Failed,
  Stale,
  Canceled,
}

data class ContextInspectionIdentity(
    val file: WorkflowFileIdentity,
    val symbol: SymbolInfo?,
    val creationName: String = "",
    val creationKind: String = "",
    val action: String,
    val intent: String = "Fix",
    val model: ScopedModel,
)

data class ContextInspectionState(
    val status: ContextInspectionStatus = ContextInspectionStatus.Closed,
    val identity: ContextInspectionIdentity? = null,
    val generation: Long = 0,
    val manifest: ContextManifest? = null,
    val message: String = "",
)

data class DesktopWorkflowSnapshot(
    val state: DesktopState = DesktopState(),
    val modelCatalog: ModelCatalog = ModelCatalog(),
    val providerConfirmations: ScopedConfirmationState = ScopedConfirmationState(),
    val securityReviewRemoteConfirmed: Boolean = false,
    val contextInspection: ContextInspectionState = ContextInspectionState(),
    val analysisInProgress: Boolean = false,
    val generating: Boolean = false,
    val draftValidationInProgress: Boolean = false,
    val declarationExplanation: DeclarationExplanationState = DeclarationExplanationState(),
) {
  fun model(scope: ModelScope): ScopedModel = modelCatalog.forScope(scope)

  fun providerConfirmed(scope: ModelScope): Boolean = providerConfirmations.confirmed(scope)
}

private data class VerifiedScanActionRequest(
    val project: WorkflowProjectIdentity,
    val generation: Long,
    val expectedScan: GoScanReport? = null,
    val requiresExpectedScan: Boolean = false,
)

/**
 * Coordinates daemon interactions and workflow owners for the desktop app. Its state flow contains
 * only immutable snapshots; visual-only Compose state remains with the composables that render it.
 */
class DesktopWorkflowPresenter(
    private val api: ApiClient,
    private val lastProjectStore: LastProjectStore,
    parentScope: CoroutineScope,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val pollingIntervalMillis: Long = 750,
) : AutoCloseable {
  private val lifetime = SupervisorJob(parentScope.coroutineContext[Job])
  private val scope = CoroutineScope(parentScope.coroutineContext + lifetime)
  private val controller = DesktopWorkflowController()
  private val jobCoordinator = DesktopJobCoordinator(scope, pollingIntervalMillis)
  private val mutableSnapshot = MutableStateFlow(DesktopWorkflowSnapshot())
  private val benchmarkWorkflow =
      DesktopBenchmarkWorkflow(api, scope, ioDispatcher, { controller.state }, ::dispatch)
  private val securityWorkflow =
      DesktopSecurityWorkflow(
          api,
          scope,
          ioDispatcher,
          { controller.state },
          ::dispatch,
          ::clearSecurityReviewRemoteConfirmation)

  private val analysisWorkflow =
      DesktopAnalysisWorkflow(
          api, scope, ioDispatcher, jobCoordinator, { controller.state }, ::dispatch)

  private var connectionJob: Job? = null
  private var startupJob: Job? = null
  private var started = false
  private var explicitOpenStarted = false
  private var projectJob: Job? = null
  private val acceptedProjects = Channel<Pair<Long, String>>(Channel.UNLIMITED)
  private var latestAcceptedProjectRequest = 0L
  private var workspaceDetailsGeneration = 0L
  private val preferenceSaveJob =
      scope.launch {
        for ((request, path) in acceptedProjects) {
          try {
            require(path.isNotBlank()) { "The opened project has no path to remember." }
            io { lastProjectStore.save(path) }
            if (request == latestAcceptedProjectRequest)
                dispatch(DesktopEvent.ProjectPreferenceSaved(path))
          } catch (canceled: CancellationException) {
            throw canceled
          } catch (error: Exception) {
            if (request == latestAcceptedProjectRequest) {
              dispatch(
                  DesktopEvent.ProjectPreferenceSaveFailed(
                      "Could not remember this project for the next launch: " +
                          (error.message?.takeIf(String::isNotBlank)
                              ?: "local storage unavailable")))
            }
          }
        }
      }
  private var fileJob: Job? = null
  private var fileFreshnessJob: Job? = null
  private var enrichmentJobs: List<Job> = emptyList()
  private var declarationExplanationJob: Job? = null
  private var contextInspectionJob: Job? = null
  private var contextInspectionGeneration = 0L
  private var chatJob: Job? = null
  private var closed = false
  private var draftValidationJob: Job? = null
  private var draftChecksJob: Job? = null
  private var declarationExplanationGeneration = 0L
  private var verifiedScanActionGeneration = 0L
  private var verifiedScanFindingsGeneration = 0L
  private var activeTask: WorkflowTaskIdentity? = null
  private var activeDraft: WorkflowDraftIdentity? = null
  private var preparationSelectionGeneration = 0L

  val snapshot: StateFlow<DesktopWorkflowSnapshot> = mutableSnapshot.asStateFlow()

  fun start() {
    if (started) return
    started = true
    refreshConnection()
    startupJob =
        scope.launch {
          val remembered =
              try {
                DesktopEvent.RememberedProjectRead(io { lastProjectStore.load() })
              } catch (canceled: CancellationException) {
                throw canceled
              } catch (error: Exception) {
                DesktopEvent.RememberedProjectRead(
                    null,
                    "Could not read the last project from local preferences: " +
                        (error.message?.takeIf(String::isNotBlank) ?: "storage unavailable"))
              }
          // A chooser-based open owns the newer intent, even if this read finishes afterward.
          if (explicitOpenStarted) return@launch
          controller.dispatch(remembered)
          publish()
          remembered.path?.let { openProject(it, restore = true) }
        }
  }

  fun dispatch(event: DesktopEvent) {
    if (event is DesktopEvent.SymbolSelected ||
        event is DesktopEvent.EditorContextSelected ||
        event is DesktopEvent.SourceLineSelected ||
        event is DesktopEvent.WorkspaceSelected)
        preparationSelectionGeneration++
    val before = selectedDeclarationTarget(controller.state)
    analysisWorkflow.beforeEvent(event)
    benchmarkWorkflow.beforeEvent(event)
    securityWorkflow.beforeEvent(event)
    if (event is DesktopEvent.ProjectLoaded) {
      invalidateJobActions()
      jobCoordinator.projectOpened(event.project.identity())
    }
    val pendingChat = controller.state.chat.pendingRequestId
    controller.dispatch(event)
    if (pendingChat != 0L && controller.state.chat.pendingRequestId != pendingChat) {
      activeTask = null
      chatJob?.cancel()
      chatJob = null
      setOperation(generating = false)
    }
    if (event is DesktopEvent.DraftEdited) {
      draftValidationJob?.cancel()
      setOperation(validating = false)
    }
    if (event is DesktopEvent.IndexRefreshed &&
        snapshot.value.state.project?.projectRevision != event.index.projectRevision) {
      controller.state.project?.identity()?.let(jobCoordinator::projectOpened)
    }
    invalidateContextInspectionIfTargetChanged()
    val after = selectedDeclarationTarget(controller.state)
    if (before != after &&
        mutableSnapshot.value.declarationExplanation.status !=
            DeclarationExplanationStatus.Unavailable) {
      invalidateDeclarationExplanation(
          "Selection changed. Request a new explanation for the current declaration.")
    }
    publish()
  }

  fun clearPreparedSuggestion() {
    dispatch(DesktopEvent.SuggestionCleared)
  }

  fun setProviderConfirmation(scope: ModelScope, confirmed: Boolean) {
    val previous = mutableSnapshot.value
    mutableSnapshot.value =
        previous.copy(
            providerConfirmations =
                previous.providerConfirmations.withConfirmation(scope, confirmed),
        )
    if (scope == ModelScope.Function && previous.providerConfirmed(scope) && !confirmed) {
      invalidateChatAuthorization()
    }
    if (scope == ModelScope.Function &&
        !confirmed &&
        previous.providerConfirmed(scope) &&
        previous.model(scope).remoteProvider &&
        previous.declarationExplanation.status == DeclarationExplanationStatus.Loading)
        invalidateDeclarationExplanation(
            "Function model consent was revoked. Request a new explanation after confirming the destination.")
  }

  /** Security review uses a per-request confirmation, independent of other Analyze operations. */
  fun setSecurityReviewRemoteConfirmation(confirmed: Boolean) {
    mutableSnapshot.value = mutableSnapshot.value.copy(securityReviewRemoteConfirmed = confirmed)
  }

  fun refreshConnection() {
    connectionJob?.cancel()
    connectionJob =
        scope.launch {
          val startedAt = System.nanoTime()
          try {
            val (status, catalog) = io { api.status() to api.modelCatalog() }
            val previous = mutableSnapshot.value
            val function = catalog.forScope(ModelScope.Function)
            val confirmations =
                if (previous.modelCatalog.identity() == catalog.identity())
                    previous.providerConfirmations
                else ScopedConfirmationState()
            mutableSnapshot.value =
                previous.copy(
                    modelCatalog = catalog,
                    providerConfirmations = confirmations,
                    securityReviewRemoteConfirmed =
                        previous.securityReviewRemoteConfirmed.takeIf {
                          previous.modelCatalog.identity() == catalog.identity()
                        } ?: false)
            if (previous.modelCatalog.identity() != catalog.identity()) {
              invalidateChatAuthorization()
              invalidateContextInspectionIfTargetChanged()
              analysisWorkflow.providerChanged()
              if (previous.declarationExplanation.status == DeclarationExplanationStatus.Loading)
                  invalidateDeclarationExplanation(
                      "Model catalog changed; Function confirmation was reset. Request a new explanation.")
            } else analysisWorkflow.refresh()
            dispatch(
                DesktopEvent.ConnectionUpdated(
                    ConnectionState(
                        "Daemon connected",
                        "${function.profile} · ${function.model}",
                        status.version,
                        true,
                        api.endpointLocality(),
                        "${(System.nanoTime() - startedAt) / 1_000_000}ms")))
          } catch (_: CancellationException) {
            throw CancellationException()
          } catch (_: Exception) {
            dispatch(
                DesktopEvent.ConnectionUpdated(
                    ConnectionState(
                        label = "Daemon unavailable", locality = api.endpointLocality())))
          }
        }
  }

  fun loadProject(path: String, restore: Boolean) {
    explicitOpenStarted = true
    openProject(path, restore)
  }

  fun retryProjectRestore() {
    val attempt = controller.state.projectState.openingAttempt ?: return
    if (attempt.kind != ProjectOpeningKind.Restore ||
        attempt.outcome !is ProjectOpeningOutcome.Failed)
        return
    explicitOpenStarted = true
    openProject(attempt.path, restore = true)
  }

  private fun openProject(path: String, restore: Boolean) {
    projectJob?.cancel()
    supersedeProjectDetailsRefresh("Project opening interrupted the workspace detail refresh.")
    clearSecurityReviewRemoteConfirmation()
    cancelProjectScopedWork()
    val request =
        controller.beginProjectLoad(
            path, if (restore) ProjectOpeningKind.Restore else ProjectOpeningKind.Import)
    publish()
    dispatch(
        DesktopEvent.Status(
            if (restore) "Reopening ${File(path).name}…" else "Importing ${File(path).name}…"))
    projectJob =
        scope.launch {
          try {
            val (project, index) =
                io {
                  val loaded =
                      if (restore) api.restoreProject(path)
                      else
                          api.importProject(
                              path, snapshot.value.providerConfirmed(ModelScope.Analyze))
                  loaded to api.index()
                }
            if (!controller.projectLoaded(request, project, index)) {
              if (controller.state.projectState.openingAttempt?.requestId == request) publish()
              return@launch
            }
            publish()
            latestAcceptedProjectRequest = request
            acceptedProjects.send(request to project.path)
            if (restore) dispatch(DesktopEvent.Status("Reopened ${project.name}"))
            jobCoordinator.projectOpened(project.identity())
            refreshProjectWorkspace(project.identity())
          } catch (canceled: CancellationException) {
            if (controller.cancelProjectLoad(request)) publish()
            throw canceled
          } catch (error: Exception) {
            if (!controller.isCurrentProjectRequest(request)) return@launch
            val message =
                if (restore)
                    error.message?.takeIf(String::isNotBlank) ?: "Could not restore project"
                else modelRequestFailureMessage(error, ModelScope.Analyze, "Import failed")
            if (controller.projectFailed(request, message)) publish()
          }
        }
  }

  fun reindexProject() {
    val attempt = controller.beginProjectIndexing() ?: return
    supersedeProjectDetailsRefresh("Re-indexing interrupted the workspace detail refresh.")
    publish()
    clearSecurityReviewRemoteConfirmation()
    cancelProjectScopedWork()
    projectJob =
        scope.launch {
          try {
            val index = io { api.reindex(attempt.projectRevision) }
            if (!controller.projectIndexingCompleted(attempt, index)) {
              publish()
              restoreProjectObservationAfterIndexing(attempt)
              return@launch
            }
            publish()
            val refreshed = controller.state.project?.identity() ?: return@launch
            jobCoordinator.projectOpened(refreshed)
            refreshProjectWorkspace(refreshed)
          } catch (canceled: CancellationException) {
            if (controller.cancelProjectIndexing(attempt)) {
              publish()
              restoreProjectObservationAfterIndexing(attempt)
            }
            throw canceled
          } catch (error: Exception) {
            if (controller.projectIndexingFailed(
                attempt,
                error.message?.takeIf(String::isNotBlank)
                    ?: "Could not re-index project. Try again.")) {
              publish()
              restoreProjectObservationAfterIndexing(attempt)
            }
          }
        }
  }

  private fun restoreProjectObservationAfterIndexing(attempt: ProjectIndexingAttempt) {
    val state = controller.state
    val outcome = state.projectState.indexingAttempt
    if (outcome?.generation != attempt.generation ||
        outcome.outcome !is ProjectIndexingOutcome.Failed &&
            outcome.outcome != ProjectIndexingOutcome.Canceled ||
        state.projectState.openingAttempt?.outcome == ProjectOpeningOutcome.Opening ||
        state.project?.let {
          it.projectId == attempt.projectId &&
              it.projectRevision == attempt.projectRevision &&
              it.path == attempt.path
        } != true)
        return
    jobCoordinator.projectOpened(
        WorkflowProjectIdentity(attempt.projectId, attempt.projectRevision))
    analysisWorkflow.refresh()
  }

  private fun invalidateFileSelectionWork() {
    clearSecurityReviewRemoteConfirmation()
    if (mutableSnapshot.value.declarationExplanation.status !=
        DeclarationExplanationStatus.Unavailable) {
      invalidateDeclarationExplanation(
          "Selection changed. Request a new explanation for the current declaration.")
    }
    chatJob?.cancel()
    draftValidationJob?.cancel()
    draftChecksJob?.cancel()
    activeTask = null
    activeDraft = null
    setOperation(generating = false, validating = false)
    fileFreshnessJob?.cancel()
    enrichmentJobs.forEach(Job::cancel)
    securityWorkflow.cancel()
    benchmarkWorkflow.invalidate()
  }

  private fun cancelFileReadJob() {
    // A queued coroutine may be canceled before its body (and cancellation catch) starts.
    controller.currentPendingFileRequest()?.let { request ->
      if (controller.cancelFileLoad(request)) publish()
    }
    fileJob?.cancel()
  }

  internal fun selectFile(
      path: String,
      editorTarget: EditorNavigationTarget? = null,
      preparedFixRequest: String? = null,
      preparedTaskSpec: BugTaskSpec? = null,
  ) = openFileInEditor(path, editorTarget, preparedFixRequest, preparedTaskSpec)

  private fun loadFile(
      path: String,
      editorTarget: EditorNavigationTarget? = null,
      preparedFixRequest: String? = null,
      preparedTaskSpec: BugTaskSpec? = null,
      preparationFinding: UnifiedFinding? = null,
      inspectionResult: PerformanceResult? = null,
      inspectionCurrent: () -> Boolean = { true },
      onInspectionLoaded: (() -> Unit)? = null,
      navigationCurrent: () -> Boolean = { true },
      navigationIdentity: FileNavigationIdentity? = null,
  ) {
    cancelFileReadJob()
    val request = controller.beginFileLoad(path) ?: return
    if (snapshot.value.contextInspection.identity?.file?.path != path)
        invalidateContextInspectionIfTargetChanged(pendingPath = path)
    val selectionGeneration = preparationSelectionGeneration
    fun obsoletePreparation(): Boolean {
      if (!navigationCurrent()) return true
      if (inspectionResult != null &&
          (selectionGeneration != preparationSelectionGeneration ||
              !inspectionCurrent() ||
              performanceSourceTarget(inspectionResult) != editorTarget))
          return true
      if (preparationFinding == null) return false
      if (selectionGeneration != preparationSelectionGeneration) return true
      val latest = preparationDecision(preparationFinding)
      return latest !is FindingPreparationDecision.Eligible ||
          latest.target != editorTarget ||
          latest.task != preparedTaskSpec
    }
    publish()
    fileJob =
        scope.launch {
          try {
            if (!controller.isCurrentFileLoad(request)) return@launch
            if (obsoletePreparation()) {
              if (controller.cancelFileLoad(request)) publish()
              return@launch
            }
            val (file, symbolResponse) = io { api.fileInfo(path) to api.symbols(path) }
            val symbols = symbolResponse.symbols
            if (!controller.isCurrentFileLoad(request)) return@launch
            if (obsoletePreparation()) {
              if (controller.cancelFileLoad(request)) publish()
              return@launch
            }
            val index = controller.state.index
            val readFailure =
                when {
                  index?.projectId != request.projectId ||
                      index.projectRevision != request.projectRevision ||
                      index.files.none { it.path == path } ->
                      "The destination is no longer indexed in the active project."
                  file.path != path -> "The file response belongs to another source."
                  symbolResponse.path != path -> "Loaded declarations belong to another source."
                  symbolResponse.projectId != request.projectId ||
                      symbolResponse.projectRevision != request.projectRevision ->
                      "Loaded declarations belong to another project or revision."
                  else -> null
                }
            if (readFailure != null) {
              if (controller.fileFailed(request, "Could not open $path: $readFailure")) publish()
              return@launch
            }
            val prepared =
                if (preparationFinding != null) {
                  val latest =
                      preparationDecision(preparationFinding) as FindingPreparationDecision.Eligible
                  when (val validated =
                      loadedFindingPreparationDecision(
                          latest, file, symbols, preparationFinding.fileHash)) {
                    is FindingPreparationDecision.Blocked -> {
                      if (controller.fileFailed(request, validated.reason)) publish()
                      return@launch
                    }
                    is FindingPreparationDecision.Eligible -> validated
                  }
                } else null
            val published =
                if (navigationIdentity != null)
                    controller.fileNavigationLoaded(request, file, symbols, navigationIdentity)
                else controller.fileLoaded(request, file, symbols)
            if (!published) return@launch
            invalidateContextInspectionIfTargetChanged()
            invalidateFileSelectionWork()
            publish()
            onInspectionLoaded?.invoke()
            if (prepared != null) {
              val symbol = symbols.single { it.name == prepared.task.targetSymbol }
              dispatch(DesktopEvent.EditorContextSelected(symbol, symbol.startLine))
              dispatch(
                  DesktopEvent.SuggestionPrepared(
                      "fix", findingTaskRequirement(prepared.task), symbol, prepared.task))
            } else {
              editorTarget?.let { target ->
                val selection =
                    if (inspectionResult != null) EditorNavigationSelection(null, target.line)
                    else resolveEditorNavigation(symbols, target)
                dispatch(DesktopEvent.EditorContextSelected(selection.symbol, selection.focusLine))
              }
              preparedFixRequest?.let { requestText ->
                dispatch(
                    DesktopEvent.SuggestionPrepared(
                        "fix",
                        requestText,
                        editorTarget?.let { resolveEditorNavigation(symbols, it).symbol },
                        preparedTaskSpec))
              }
            }
            val loaded = controller.currentFileRequest() ?: return@launch
            loadFileEnrichments(loaded)
          } catch (canceled: CancellationException) {
            if (controller.cancelFileLoad(request)) publish()
            throw canceled
          } catch (error: Exception) {
            if (obsoletePreparation()) {
              if (controller.cancelFileLoad(request)) publish()
            } else if (controller.fileFailed(
                request, error.message?.takeIf(String::isNotBlank) ?: "File load failed")) {
              publish()
            }
          }
        }
  }

  /**
   * Read-only return from a shell: retain the draft if unchanged, invalidate evidence if changed.
   */
  fun refreshSelectedFile() {
    val project = snapshot.value.state.project?.identity() ?: return
    val selected = snapshot.value.state.selectedFile ?: return
    fileFreshnessJob?.cancel()
    fileFreshnessJob =
        scope.launch {
          fun isCurrent(): Boolean =
              matchesProject(project) &&
                  snapshot.value.state.selectedFile?.let {
                    it.path == selected.path && it.contentHash == selected.contentHash
                  } == true
          try {
            val file = io { api.fileInfo(selected.path) }
            if (!isCurrent()) return@launch
            require(file.path == selected.path) { "Refreshed source belongs to another file." }
            if (file.contentHash == selected.contentHash) return@launch
            val symbols = io { api.symbols(selected.path).symbols }
            if (!isCurrent()) return@launch
            invalidateFileEvidenceWork()
            dispatch(DesktopEvent.SelectedFileRefreshed(file, symbols))
          } catch (canceled: CancellationException) {
            throw canceled
          } catch (error: Exception) {
            if (!isCurrent()) return@launch
            invalidateFileEvidenceWork()
            dispatch(
                DesktopEvent.SelectedFileUnavailable(
                    error.message ?: "Could not refresh the selected file."))
          }
        }
  }

  private fun invalidateFileEvidenceWork() {
    chatJob?.cancel()
    draftValidationJob?.cancel()
    draftChecksJob?.cancel()
    enrichmentJobs.forEach(Job::cancel)
    benchmarkWorkflow.invalidate()
    securityWorkflow.cancel()
    activeTask = null
    activeDraft = null
  }

  fun openFileInEditor(
      path: String,
      editorTarget: EditorNavigationTarget? = null,
      preparedFixRequest: String? = null,
      preparedTaskSpec: BugTaskSpec? = null,
  ) {
    val intent = fileNavigationIntent(path, editorTarget, preparedFixRequest, preparedTaskSpec)
    if (intent == null) {
      dispatch(
          DesktopEvent.Failed(
              "This file no longer points to an indexed file in the active project."))
      return
    }
    if (intent.requiresDiscard) {
      dispatch(
          DesktopEvent.Failed(
              "Review and confirm discarding the current work before opening this file."))
      return
    }
    confirmFileNavigationIntent(intent)
  }

  internal data class FileNavigationIntent(
      val path: String,
      val editorTarget: EditorNavigationTarget?,
      val preparedFixRequest: String?,
      val preparedTaskSpec: BugTaskSpec?,
      val identity: FileNavigationIdentity,
      val generating: Boolean,
      val composerHasWork: Boolean,
      val composerCurrent: () -> Boolean,
  ) {
    val hasWork: Boolean
      get() =
          identity.draft.hasWork || identity.chatRequestId != 0L || generating || composerHasWork

    val requiresDiscard: Boolean
      get() = identity.selectedFile?.path != path && hasWork
  }

  private var pendingFileNavigationIntent: FileNavigationIntent? = null

  internal fun fileNavigationIntent(
      path: String,
      editorTarget: EditorNavigationTarget? = null,
      preparedFixRequest: String? = null,
      preparedTaskSpec: BugTaskSpec? = null,
      composerHasWork: Boolean = false,
      composerCurrent: () -> Boolean = { true },
  ): FileNavigationIntent? {
    // Even an invalid new action revokes an older dialog callback.
    pendingFileNavigationIntent = null
    val identity = controller.state.fileNavigationIdentity(path) ?: return null
    if (editorTarget != null && editorTarget.path != path || !composerCurrent()) return null
    return FileNavigationIntent(
            path,
            editorTarget,
            preparedFixRequest,
            preparedTaskSpec,
            identity,
            snapshot.value.generating,
            composerHasWork,
            composerCurrent)
        .also { pendingFileNavigationIntent = it }
  }

  private fun currentFileNavigation(intent: FileNavigationIntent): Boolean =
      controller.state.matchesFileNavigation(intent.path, intent.identity) &&
          snapshot.value.generating == intent.generating &&
          intent.composerCurrent()

  internal fun confirmFileNavigationIntent(
      intent: FileNavigationIntent,
      onLoaded: (() -> Unit)? = null,
  ): Boolean {
    val pending = pendingFileNavigationIntent
    if (pending === intent) pendingFileNavigationIntent = null
    if (pending !== intent || !currentFileNavigation(intent)) {
      dispatch(DesktopEvent.Failed("The file or current work changed. Choose Open file again."))
      return false
    }
    dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
    if (intent.identity.selectedFile?.path == intent.path) {
      cancelFileReadJob()
      intent.editorTarget?.let { target ->
        val selection = resolveEditorNavigation(controller.state.symbols, target)
        dispatch(DesktopEvent.EditorContextSelected(selection.symbol, selection.focusLine))
      }
      return true
    }
    loadFile(
        intent.path,
        intent.editorTarget,
        intent.preparedFixRequest,
        intent.preparedTaskSpec,
        navigationCurrent = { currentFileNavigation(intent) },
        navigationIdentity = intent.identity,
        onInspectionLoaded = onLoaded)
    return true
  }

  /** An approval is bound to the observed project, finding, target and complete draft buffer. */
  internal data class FindingIntent(
      val finding: UnifiedFinding,
      val prepare: Boolean,
      val project: SwitchProjectIdentity,
      val draft: SwitchDraftIdentity,
      val selectedFile: ProjectFileInfo?,
      val target: EditorNavigationTarget,
  )

  internal fun findingIntent(finding: UnifiedFinding, prepare: Boolean): FindingIntent? {
    val state = snapshot.value.state
    val project = state.project ?: return null
    val target =
        if (prepare) (preparationDecision(finding) as? FindingPreparationDecision.Eligible)?.target
        else {
          val displayed = state.loadedFindingsFor(finding.category)
          if (finding.projectId != project.projectId ||
              finding.projectRevision != project.projectRevision ||
              displayed.count { it == finding } != 1)
              null
          else findingNavigationTarget(finding, state.index)
        }
    return target?.let {
      FindingIntent(
          finding,
          prepare,
          SwitchProjectIdentity(project),
          SwitchDraftIdentity(state.chat.session, state.review.draft, state.review.editor),
          state.selectedFile,
          it)
    }
  }

  private fun approvedFindingIntent(
      finding: UnifiedFinding,
      prepare: Boolean,
  ): EditorNavigationTarget? {
    val current = findingIntent(finding, prepare)
    if (current == null) {
      val reason =
          if (prepare) (preparationDecision(finding) as FindingPreparationDecision.Blocked).reason
          else "This finding no longer points to a file in the active project."
      dispatch(DesktopEvent.Failed(reason))
      return null
    }
    val state = snapshot.value.state
    val sameSource = !prepare && state.selectedFile?.path == current.target.path
    if (!sameSource && current.draft.hasWork) {
      dispatch(
          DesktopEvent.Failed(
              "Review and confirm discarding the current draft before opening this finding."))
      return null
    }
    return current.target
  }

  internal fun confirmFindingIntent(intent: FindingIntent): Boolean {
    if (findingIntent(intent.finding, intent.prepare) != intent) {
      dispatch(DesktopEvent.Failed("The finding or draft changed. Choose the action again."))
      return false
    }
    if (intent.draft.hasWork) discardDraft()
    if (intent.prepare) prepareFinding(intent.finding) else openFinding(intent.finding)
    return true
  }

  fun openFinding(finding: UnifiedFinding) {
    val target = approvedFindingIntent(finding, false) ?: return
    if (snapshot.value.state.selectedFile?.path == target.path) {
      dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
      val selection = resolveEditorNavigation(snapshot.value.state.symbols, target)
      dispatch(DesktopEvent.EditorContextSelected(selection.symbol, selection.focusLine))
    } else openFileInEditor(target.path, target)
  }

  fun prepareFinding(finding: UnifiedFinding) {
    val target = approvedFindingIntent(finding, true) ?: return
    val decision = preparationDecision(finding) as? FindingPreparationDecision.Eligible ?: return
    dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
    loadFile(target.path, target, preparationFinding = finding, preparedTaskSpec = decision.task)
  }

  private fun preparationDecision(finding: UnifiedFinding): FindingPreparationDecision {
    val state = snapshot.value.state
    return findingPreparationDecision(
        finding, state.project, state.loadedFindingsFor(finding.category), state.index)
  }

  private fun DesktopState.loadedFindingsFor(category: String): List<UnifiedFinding> =
      when (category) {
        "",
        "bugs" -> projectBugFindings()
        "performance",
        "security" -> analysisResultPage(category).semantic
        else -> emptyList()
      }

  fun viewAnalysisResults(category: String, path: String) {
    if (category !in setOf("bugs", "performance", "security")) return
    if (path.isNotBlank() && snapshot.value.state.index?.files?.none { it.path == path } != false)
        return
    dispatch(DesktopEvent.WorkspaceSelected(analysisCategoryWorkspace(category)))
  }

  internal data class PerformanceSourceIntent(
      val result: PerformanceResult,
      val runIdentity: AnalysisRunIdentity,
      val project: SwitchProjectIdentity,
      val draft: SwitchDraftIdentity,
      val selectedFile: ProjectFileInfo?,
      val target: EditorNavigationTarget,
      val index: ProjectIndex,
      val selectionCurrent: () -> Boolean,
  )

  private fun performanceSourceTarget(result: PerformanceResult): EditorNavigationTarget? {
    val state = snapshot.value.state
    val project = state.project ?: return null
    val page = state.analysisResultPage("performance")
    val runIdentity = page.run?.identity ?: return null
    val index = state.index ?: return null
    if (result.page.run?.identity != runIdentity ||
        result.report.projectId != project.projectId ||
        result.report.projectRevision != project.projectRevision ||
        runIdentity.projectId != project.projectId ||
        runIdentity.projectRevision != project.projectRevision ||
        performanceResults(page).count {
          it.report == result.report && it.finding == result.finding
        } != 1 ||
        index.projectId != project.projectId ||
        index.projectRevision != project.projectRevision ||
        index.files.count { it.path == result.report.path } != 1)
        return null
    return EditorNavigationTarget(
        result.report.path, line = result.finding.startLine.takeIf { it > 0 } ?: 0)
  }

  internal fun performanceSourceIntent(
      result: PerformanceResult,
      selectionCurrent: () -> Boolean = { true },
  ): PerformanceSourceIntent? {
    if (!selectionCurrent()) return null
    val state = snapshot.value.state
    val target = performanceSourceTarget(result) ?: return null
    return PerformanceSourceIntent(
        result,
        result.page.run!!.identity,
        SwitchProjectIdentity(state.project!!),
        SwitchDraftIdentity(state.chat.session, state.review.draft, state.review.editor),
        state.selectedFile,
        target,
        state.index!!,
        selectionCurrent)
  }

  private fun currentPerformanceSource(intent: PerformanceSourceIntent): Boolean =
      snapshot.value.state.index === intent.index &&
          performanceSourceIntent(intent.result, intent.selectionCurrent) == intent

  internal fun openPerformanceFinding(
      result: PerformanceResult,
      selectionCurrent: () -> Boolean = { true },
      onLoaded: (() -> Unit)? = null,
  ) {
    val intent = performanceSourceIntent(result, selectionCurrent)
    if (intent == null) {
      dispatch(
          DesktopEvent.Failed(
              "This opportunity no longer has one indexed source file in the active project. Refresh the results or index."))
      return
    }
    if (intent.selectedFile?.path != intent.target.path && intent.draft.hasWork) {
      dispatch(
          DesktopEvent.Failed(
              "Review and confirm discarding the current draft before opening this opportunity."))
      return
    }
    if (intent.selectedFile?.path == intent.target.path) {
      dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
      dispatch(DesktopEvent.EditorContextSelected(null, intent.target.line))
    } else {
      dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
      loadFile(
          intent.target.path,
          intent.target,
          inspectionResult = result,
          inspectionCurrent = {
            snapshot.value.state.index === intent.index && intent.selectionCurrent()
          },
          onInspectionLoaded = onLoaded)
    }
  }

  internal fun confirmPerformanceSourceIntent(
      intent: PerformanceSourceIntent,
      onLoaded: (() -> Unit)? = null,
  ): Boolean {
    if (!currentPerformanceSource(intent)) {
      dispatch(DesktopEvent.Failed("The opportunity or draft changed. Choose Open source again."))
      return false
    }
    if (intent.selectedFile?.path != intent.target.path && intent.draft.hasWork) discardDraft()
    openPerformanceFinding(intent.result, intent.selectionCurrent, onLoaded)
    return true
  }

  internal data class PerformancePreparationIntent(
      val result: PerformanceResult,
      val target: PerformancePreparationDecision.Eligible,
      val runIdentity: AnalysisRunIdentity,
      val project: SwitchProjectIdentity,
      val draft: SwitchDraftIdentity,
      val selectedFile: ProjectFileInfo?,
      val selectionCurrent: () -> Boolean,
  )

  internal fun performancePreparationIntent(
      result: PerformanceResult,
      selectionCurrent: () -> Boolean = { true },
  ): PerformancePreparationIntent? {
    if (!selectionCurrent()) return null
    val state = snapshot.value.state
    val page = state.analysisResultPage("performance")
    if (result.page.run?.identity != page.run?.identity ||
        result.page.project != page.project ||
        result.page.results != page.results ||
        result.stale ||
        result.page.stale ||
        performanceResults(page).count {
          it.report == result.report && it.finding == result.finding
        } != 1)
        return null
    val target =
        performancePreparationDecision(result.copy(page = page), state.index)
            as? PerformancePreparationDecision.Eligible ?: return null
    return PerformancePreparationIntent(
        result,
        target,
        page.run!!.identity,
        SwitchProjectIdentity(state.project!!),
        SwitchDraftIdentity(state.chat.session, state.review.draft, state.review.editor),
        state.selectedFile,
        selectionCurrent)
  }

  private fun currentPerformancePreparation(intent: PerformancePreparationIntent): Boolean =
      performancePreparationIntent(intent.result, intent.selectionCurrent) == intent

  internal fun preparePerformanceFinding(
      result: PerformanceResult,
      selectionCurrent: () -> Boolean = { true },
      inputCurrent: () -> Boolean = { true },
      onPrepared: () -> Unit = {},
  ) {
    val intent = performancePreparationIntent(result, selectionCurrent)
    if (intent == null) {
      val page = snapshot.value.state.analysisResultPage("performance")
      val reason =
          if (result.page.run?.identity != page.run?.identity ||
              result.page.project != page.project ||
              result.page.results != page.results ||
              performanceResults(page).count {
                it.report == result.report && it.finding == result.finding
              } != 1)
              "This opportunity changed or is ambiguous. Refresh Performance results."
          else
              (performancePreparationDecision(result.copy(page = page), snapshot.value.state.index)
                      as? PerformancePreparationDecision.Blocked)
                  ?.reason ?: "This opportunity changed. Refresh Performance results."
      dispatch(DesktopEvent.Failed(reason))
      return
    }
    if (intent.draft.hasWork) {
      dispatch(
          DesktopEvent.Failed(
              "Review and confirm discarding the current draft before preparing this opportunity."))
      return
    }
    loadPerformancePreparation(intent, inputCurrent, onPrepared)
  }

  internal fun confirmPerformancePreparationIntent(
      intent: PerformancePreparationIntent,
      inputCurrent: () -> Boolean = { true },
      onPrepared: () -> Unit = {},
  ): Boolean {
    if (!currentPerformancePreparation(intent)) {
      dispatch(DesktopEvent.Failed("The opportunity or draft changed. Choose Prepare fix again."))
      return false
    }
    loadPerformancePreparation(intent, inputCurrent, onPrepared)
    return true
  }

  private var performancePreparationGeneration = 0L

  private fun loadPerformancePreparation(
      intent: PerformancePreparationIntent,
      inputCurrent: () -> Boolean,
      onPrepared: () -> Unit,
  ) {
    val attempt = ++performancePreparationGeneration
    val generation = preparationSelectionGeneration
    val fileRequest = controller.currentFileRequest()
    cancelFileReadJob()
    fileJob =
        scope.launch {
          fun current(): Boolean =
              attempt == performancePreparationGeneration &&
                  generation == preparationSelectionGeneration &&
                  controller.currentFileRequest() == fileRequest &&
                  inputCurrent() &&
                  currentPerformancePreparation(intent)
          try {
            val (file, response) =
                io { api.fileInfo(intent.target.path) to api.symbols(intent.target.path) }
            if (!current()) return@launch
            if (response.projectId != intent.target.projectId ||
                response.projectRevision != intent.target.projectRevision ||
                response.path != intent.target.path) {
              dispatch(
                  DesktopEvent.Failed(
                      "Loaded declarations belong to another project, revision or file."))
              return@launch
            }
            when (val decision =
                loadedPerformancePreparationDecision(
                    intent.target, intent.result.finding, file, response.symbols)) {
              is PerformancePreparationDecision.Blocked -> {
                dispatch(DesktopEvent.Failed(decision.reason))
                return@launch
              }
              is PerformancePreparationDecision.Eligible -> Unit
            }
            // Admission commits only after both reads validate; a failed read retains the draft.
            invalidateFileSelectionWork()
            val request = controller.beginFileLoad(intent.target.path) ?: return@launch
            if (!controller.fileLoaded(request, file, response.symbols)) return@launch
            publish()
            dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
            dispatch(
                DesktopEvent.EditorContextSelected(
                    intent.target.declaration, intent.result.finding.startLine))
            dispatch(
                DesktopEvent.SuggestionPrepared(
                    "fix",
                    performancePreparationRequest(intent.result.finding),
                    intent.target.declaration))
            onPrepared()
            controller.currentFileRequest()?.let(::loadFileEnrichments)
          } catch (canceled: CancellationException) {
            throw canceled
          } catch (error: Exception) {
            if (current())
                dispatch(
                    DesktopEvent.Failed(
                        error.message?.takeIf(String::isNotBlank) ?: "File load failed"))
          }
        }
  }

  fun scanSecurity() = securityWorkflow.scanSecurity()

  fun reviewSecurity() = previewAnalysis()

  internal data class SecuritySourceIntent(
      val result: SecurityResult,
      val target: EditorNavigationTarget,
      val project: SwitchProjectIdentity,
      val index: ProjectIndex,
      val draft: SwitchDraftIdentity,
      val selectedFile: ProjectFileInfo?,
      val selectionCurrent: () -> Boolean,
  )

  internal fun securitySourceIntent(
      result: SecurityResult,
      selectionCurrent: () -> Boolean = { true },
  ): SecuritySourceIntent? {
    if (!selectionCurrent()) return null
    val state = snapshot.value.state
    val page = state.analysisResultPage("security")
    val target = securitySourceTarget(result, page, state.index) ?: return null
    return SecuritySourceIntent(
        result,
        target,
        SwitchProjectIdentity(state.project ?: return null),
        state.index ?: return null,
        SwitchDraftIdentity(state.chat.session, state.review.draft, state.review.editor),
        state.selectedFile,
        selectionCurrent)
  }

  private fun currentSecuritySource(intent: SecuritySourceIntent): Boolean =
      snapshot.value.state.index === intent.index &&
          securitySourceIntent(intent.result, intent.selectionCurrent) == intent

  internal fun openSecurityFinding(
      result: SecurityResult,
      selectionCurrent: () -> Boolean = { true },
      inputCurrent: () -> Boolean = { true },
      onLoaded: () -> Unit = {},
  ) {
    val intent = securitySourceIntent(result, selectionCurrent)
    if (intent == null) {
      dispatch(
          DesktopEvent.Failed(
              "This Security result or its source range is missing, invalid or ambiguous in the active project. Refresh the results or index."))
      return
    }
    if (intent.selectedFile?.path != intent.target.path && intent.draft.hasWork) {
      dispatch(
          DesktopEvent.Failed("Confirm discarding the current draft before opening this source."))
      return
    }
    loadSecuritySource(intent, inputCurrent, onLoaded)
  }

  internal fun confirmSecuritySourceIntent(
      intent: SecuritySourceIntent,
      inputCurrent: () -> Boolean = { true },
      onLoaded: () -> Unit = {},
  ): Boolean {
    if (!currentSecuritySource(intent) || !inputCurrent()) {
      dispatch(
          DesktopEvent.Failed(
              "The Security result, selection or input changed. Choose Open source again."))
      return false
    }
    loadSecuritySource(intent, inputCurrent, onLoaded)
    return true
  }

  private var securitySourceGeneration = 0L

  private fun loadSecuritySource(
      intent: SecuritySourceIntent,
      inputCurrent: () -> Boolean,
      onLoaded: () -> Unit,
  ) {
    val attempt = ++securitySourceGeneration
    val generation = preparationSelectionGeneration
    val fileRequest = controller.currentFileRequest()
    fun current(): Boolean =
        attempt == securitySourceGeneration &&
            generation == preparationSelectionGeneration &&
            controller.currentFileRequest() == fileRequest &&
            inputCurrent() &&
            currentSecuritySource(intent)
    if (intent.selectedFile?.path == intent.target.path) {
      if (current()) {
        dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
        dispatch(DesktopEvent.EditorContextSelected(null, intent.target.line))
      }
      return
    }
    cancelFileReadJob()
    fileJob =
        scope.launch {
          try {
            val (file, response) =
                io { api.fileInfo(intent.target.path) to api.symbols(intent.target.path) }
            if (!current()) return@launch
            if (file.path != intent.target.path ||
                file.lineCount < intent.result.finding.anchor.endLine ||
                response.path != intent.target.path ||
                response.projectId != intent.project.id ||
                response.projectRevision != intent.project.revision) {
              dispatch(
                  DesktopEvent.Failed(
                      "Loaded source does not contain the reported range or belongs to another project or file. Choose Open source again."))
              return@launch
            }
            invalidateFileSelectionWork()
            val request = controller.beginFileLoad(intent.target.path) ?: return@launch
            if (!controller.fileLoaded(request, file, response.symbols)) return@launch
            publish()
            dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
            dispatch(DesktopEvent.EditorContextSelected(null, intent.target.line))
            onLoaded()
            controller.currentFileRequest()?.let(::loadFileEnrichments)
          } catch (canceled: CancellationException) {
            throw canceled
          } catch (error: Exception) {
            if (current())
                dispatch(
                    DesktopEvent.Failed(
                        error.message?.takeIf(String::isNotBlank) ?: "Source read failed"))
          }
        }
  }

  internal data class SecurityPreparationIntent(
      val result: SecurityResult,
      val target: SecurityPreparationDecision.Eligible,
      val project: SwitchProjectIdentity,
      val index: ProjectIndex,
      val draft: SwitchDraftIdentity,
      val selectedFile: ProjectFileInfo?,
      val selectionCurrent: () -> Boolean,
  )

  internal fun securityPreparationIntent(
      result: SecurityResult,
      selectionCurrent: () -> Boolean = { true },
  ): SecurityPreparationIntent? {
    if (!selectionCurrent()) return null
    val state = snapshot.value.state
    val page = state.analysisResultPage("security")
    val target =
        securityPreparationDecision(result, state.index, page)
            as? SecurityPreparationDecision.Eligible ?: return null
    return SecurityPreparationIntent(
        result,
        target,
        SwitchProjectIdentity(state.project ?: return null),
        state.index ?: return null,
        SwitchDraftIdentity(state.chat.session, state.review.draft, state.review.editor),
        state.selectedFile,
        selectionCurrent)
  }

  private fun currentSecurityPreparation(intent: SecurityPreparationIntent): Boolean =
      snapshot.value.state.index === intent.index &&
          securityPreparationIntent(intent.result, intent.selectionCurrent) == intent

  internal fun prepareSecurityFinding(
      result: SecurityResult,
      selectionCurrent: () -> Boolean = { true },
      inputCurrent: () -> Boolean = { true },
      onPrepared: () -> Unit = {},
  ) {
    val intent = securityPreparationIntent(result, selectionCurrent)
    if (intent == null) {
      val state = snapshot.value.state
      val reason =
          (securityPreparationDecision(result, state.index, state.analysisResultPage("security"))
                  as? SecurityPreparationDecision.Blocked)
              ?.reason ?: "The Security result or selection changed. Choose Prepare fix again."
      dispatch(DesktopEvent.Failed(reason))
      return
    }
    if (intent.draft.hasWork) {
      dispatch(DesktopEvent.Failed("Confirm discarding the current draft before preparing a fix."))
      return
    }
    loadSecurityPreparation(intent, inputCurrent, onPrepared)
  }

  internal fun confirmSecurityPreparationIntent(
      intent: SecurityPreparationIntent,
      inputCurrent: () -> Boolean = { true },
      onPrepared: () -> Unit = {},
  ): Boolean {
    if (!currentSecurityPreparation(intent) || !inputCurrent()) {
      dispatch(
          DesktopEvent.Failed(
              "The Security result, selection or input changed. Choose Prepare fix again."))
      return false
    }
    loadSecurityPreparation(intent, inputCurrent, onPrepared)
    return true
  }

  private var securityPreparationGeneration = 0L

  private fun loadSecurityPreparation(
      intent: SecurityPreparationIntent,
      inputCurrent: () -> Boolean,
      onPrepared: () -> Unit,
  ) {
    val attempt = ++securityPreparationGeneration
    val generation = preparationSelectionGeneration
    val fileRequest = controller.currentFileRequest()
    cancelFileReadJob()
    fileJob =
        scope.launch {
          fun current(): Boolean =
              attempt == securityPreparationGeneration &&
                  generation == preparationSelectionGeneration &&
                  controller.currentFileRequest() == fileRequest &&
                  inputCurrent() &&
                  currentSecurityPreparation(intent)
          try {
            val (file, response) =
                io { api.fileInfo(intent.target.path) to api.symbols(intent.target.path) }
            if (!current()) return@launch
            when (val decision = loadedSecurityPreparationDecision(intent.target, file, response)) {
              is SecurityPreparationDecision.Blocked -> {
                dispatch(DesktopEvent.Failed(decision.reason))
                return@launch
              }
              is SecurityPreparationDecision.Eligible -> Unit
            }
            // No draft, editor or composer state changes until both reads and identities validate.
            invalidateFileSelectionWork()
            val request = controller.beginFileLoad(intent.target.path) ?: return@launch
            if (!controller.fileLoaded(request, file, response.symbols)) return@launch
            publish()
            dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
            dispatch(
                DesktopEvent.EditorContextSelected(
                    intent.target.declaration, intent.target.anchor.startLine))
            dispatch(
                DesktopEvent.SuggestionPrepared(
                    "fix", securityPreparationRequest(intent.result), intent.target.declaration))
            onPrepared()
            controller.currentFileRequest()?.let(::loadFileEnrichments)
          } catch (canceled: CancellationException) {
            throw canceled
          } catch (error: Exception) {
            if (current())
                dispatch(
                    DesktopEvent.Failed(
                        error.message?.takeIf(String::isNotBlank) ?: "Security source read failed"))
          }
        }
  }

  fun triageFinding(finding: UnifiedFinding, action: FindingLifecycleAction) {
    val project = snapshot.value.state.project ?: return
    if (finding.projectRevision.isNotBlank() &&
        finding.projectRevision != project.projectRevision) {
      dispatch(
          DesktopEvent.Failed("Refresh findings before changing triage for out-of-date results."))
      return
    }
    scope.launch {
      try {
        io { api.updateFindingStatus(finding.id, project.projectRevision, action.status) }
        if (!matchesProject(project.identity())) return@launch
        dispatch(DesktopEvent.FindingStatusUpdated(finding.id, action.status))
        refreshFindings(project.identity())
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (matchesProject(project.identity()))
            dispatch(DesktopEvent.Failed(error.message ?: "Unable to update finding triage"))
      }
    }
  }

  fun analyzeSelected(refresh: Boolean) = previewAnalysis(refresh = refresh)

  fun cancelAnalysis() = analysisWorkflow.control("cancel")

  fun previewAnalysis(
      limits: AnalysisRunLimits = AnalysisRunLimits(100, 900, 2),
      retryStaleFailed: Boolean = false,
      refresh: Boolean = !retryStaleFailed
  ) =
      analysisWorkflow.preview(
          refresh = refresh, limits = limits, retryStaleFailed = retryStaleFailed)

  fun resumeAnalysis() = analysisWorkflow.preview(resume = true)

  fun retryAnalysisPreview() = analysisWorkflow.retryPreview()

  fun startAnalysis() = analysisWorkflow.admit()

  fun pauseAnalysis() = analysisWorkflow.control("pause")

  fun refreshAnalysis() = analysisWorkflow.refreshStatus()

  fun refreshAnalysisSelection() = analysisWorkflow.fileSelection.refresh()

  fun saveAnalysisSelection(excludedPaths: List<String>) =
      analysisWorkflow.fileSelection.save(excludedPaths)

  fun dismissAnalysisAdmission() = analysisWorkflow.dismissAdmission()

  fun confirmAnalysisProvider(id: String, confirmed: Boolean) =
      analysisWorkflow.confirmProvider(id, confirmed)

  fun confirmAnalysisSecurity(confirmed: Boolean) = analysisWorkflow.confirmSecurity(confirmed)

  fun loadAnalysisResults(category: String, path: String = "") =
      analysisWorkflow.retryResults(category, path)

  fun explainSelectedDeclaration() {
    val admitted = snapshot.value
    val state = admitted.state
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    val target = selectedDeclarationTarget(state)
    if (target == null) {
      mutableSnapshot.value =
          mutableSnapshot.value.copy(
              declarationExplanation =
                  DeclarationExplanationState(
                      status = DeclarationExplanationStatus.Unavailable,
                      message = "Select one exact atomic Go declaration to explain."))
      return
    }
    val functionDestination = admitted.model(ModelScope.Function)
    val confirmRemoteProvider = admitted.providerConfirmed(ModelScope.Function)
    if (functionDestination.remoteProvider && !confirmRemoteProvider) {
      mutableSnapshot.value =
          mutableSnapshot.value.copy(
              declarationExplanation =
                  DeclarationExplanationState(
                      status = DeclarationExplanationStatus.Failed,
                      target = target,
                      message =
                          "Confirm the Function model destination before explaining this declaration."))
      return
    }
    declarationExplanationJob?.cancel()
    val generation = ++declarationExplanationGeneration
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            declarationExplanation =
                DeclarationExplanationState(
                    status = DeclarationExplanationStatus.Loading,
                    target = target,
                    message = "Explaining ${target.symbol}…"))
    declarationExplanationJob =
        scope.launch {
          try {
            val result = io {
              val current = snapshot.value
              if (current.model(ModelScope.Function) != functionDestination ||
                  (functionDestination.remoteProvider &&
                      !current.providerConfirmed(ModelScope.Function)) ||
                  generation != declarationExplanationGeneration)
                  throw CancellationException()
              api.explainDeclaration(
                  project.projectId,
                  project.projectRevision,
                  file.contentHash,
                  file.path,
                  target.symbol,
                  confirmRemoteProvider)
            }
            if (generation != declarationExplanationGeneration ||
                selectedDeclarationTarget(snapshot.value.state) != target)
                return@launch
            if (!explanationMatchesTarget(result, target)) {
              mutableSnapshot.value =
                  mutableSnapshot.value.copy(
                      declarationExplanation =
                          DeclarationExplanationState(
                              status = DeclarationExplanationStatus.Stale,
                              target = target,
                              message =
                                  "The returned explanation no longer matches the selected declaration."))
              return@launch
            }
            mutableSnapshot.value =
                mutableSnapshot.value.copy(
                    declarationExplanation =
                        DeclarationExplanationState(
                            status = DeclarationExplanationStatus.Current,
                            target = target,
                            result = result,
                            message =
                                "Current explanation · lines ${result.anchor.startLine}–${result.anchor.endLine}"))
          } catch (_: CancellationException) {
            throw CancellationException()
          } catch (error: Exception) {
            if (generation == declarationExplanationGeneration &&
                selectedDeclarationTarget(snapshot.value.state) == target) {
              mutableSnapshot.value =
                  mutableSnapshot.value.copy(
                      declarationExplanation = explanationFailureState(error, target))
            }
          }
        }
  }

  fun cancelDeclarationExplanation() {
    val current = mutableSnapshot.value.declarationExplanation
    declarationExplanationGeneration++
    declarationExplanationJob?.cancel()
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            declarationExplanation =
                current.copy(
                    status = DeclarationExplanationStatus.Canceled,
                    result = null,
                    message = "Explanation canceled."))
  }

  fun cancelGeneration() {
    val requestId = controller.state.chat.pendingRequestId
    if (requestId != 0L) {
      controller.currentFileRequest()?.let { file ->
        if (controller.cancelChatLoad(requestId, file)) publish()
      }
    }
    activeTask = null
    chatJob?.cancel()
    chatJob = null
    setOperation(generating = false)
  }

  fun cancelDraftValidation() {
    val attempt = controller.state.review.editor?.validationAttempt
    if (attempt?.status == ValidationAttemptStatus.Running) {
      controller.currentFileRequest()?.let { file ->
        if (controller.draftValidationStopped(
            attempt.requestId,
            file,
            ValidationAttemptStatus.Canceled,
            "Draft validation canceled. Validate the draft again to continue.")) {
          publish()
          dispatch(DesktopEvent.Status("Draft validation canceled"))
        }
      }
    }
    val checkAttempt = controller.state.review.checkAttempt
    val draft = controller.state.review.draft
    if (checkAttempt?.status == ValidationAttemptStatus.Running && draft != null) {
      controller.currentFileRequest()?.let { file ->
        if (controller.draftChecksStopped(
            checkAttempt.requestId,
            file,
            draft,
            ValidationAttemptStatus.Canceled,
            "Focused checks canceled. Run them again to continue."))
            publish()
      }
    }
    activeDraft = null
    draftValidationJob?.cancel()
    draftChecksJob?.cancel()
    setOperation(validating = false)
  }

  fun inspectContext(
      action: String,
      mode: ChatEditMode = ChatEditMode.ReplaceSymbol,
      creationName: String = "",
      creationKind: String = "",
  ) {
    val state = snapshot.value.state
    val project = state.project
    val file = state.selectedFile
    if (project == null || file == null) {
      contextInspectionGeneration++
      contextInspectionJob?.cancel()
      mutableSnapshot.value =
          mutableSnapshot.value.copy(
              contextInspection =
                  ContextInspectionState(
                      status = ContextInspectionStatus.Failed,
                      generation = contextInspectionGeneration,
                      message =
                          if (project == null) "Open a project to inspect context."
                          else "Open a file to inspect context."))
      return
    }
    val previewAction =
        if (mode != ChatEditMode.CreateSymbol && action == "analyze_file") "analyze_file" else "fix"
    val intent =
        if (mode == ChatEditMode.CreateSymbol) "Create $creationKind"
        else
            when (action) {
              "fix" -> "Fix"
              "refactor" -> "Refactor"
              "document" -> "Document"
              "analyze_file" -> "Analyze file"
              else -> "Assistant request ($action)"
            }
    val identity =
        ContextInspectionIdentity(
            file.identity(project),
            state.selectedSymbol.takeIf { mode == ChatEditMode.ReplaceSymbol },
            creationName.takeIf { mode == ChatEditMode.CreateSymbol }.orEmpty(),
            creationKind.takeIf { mode == ChatEditMode.CreateSymbol }.orEmpty(),
            previewAction,
            intent,
            snapshot.value.model(
                if (previewAction == "analyze_file") ModelScope.Bug else ModelScope.Function))
    if (snapshot.value.contextInspection.status == ContextInspectionStatus.Loading &&
        snapshot.value.contextInspection.identity == identity)
        return
    startContextInspection(identity)
  }

  /**
   * The composer owns creation input; changing it revokes the captured preview without requesting
   * another.
   */
  fun contextCreationTargetChanged(name: String, kind: String) {
    val current = mutableSnapshot.value.contextInspection.identity ?: return
    if (current.creationKind != kind || (kind.isNotEmpty() && current.creationName != name))
        invalidateContextInspectionIfTargetChanged(creationTargetChanged = true)
  }

  fun retryContextInspection() {
    val identity = snapshot.value.contextInspection.identity ?: return
    if (snapshot.value.contextInspection.status == ContextInspectionStatus.Stale ||
        !isCurrentContextInspection(identity)) {
      invalidateContextInspectionIfTargetChanged()
      return
    }
    startContextInspection(identity)
  }

  private fun startContextInspection(identity: ContextInspectionIdentity) {
    contextInspectionGeneration++
    val generation = contextInspectionGeneration
    contextInspectionJob?.cancel()
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            contextInspection =
                ContextInspectionState(ContextInspectionStatus.Loading, identity, generation))
    contextInspectionJob =
        scope.launch {
          try {
            val manifest = io { api.context(identity.file.path, identity.action) }
            if (canPublishContextInspection(generation, identity)) {
              val destinationChanged = contextDestinationMismatch(identity, manifest)
              mutableSnapshot.value =
                  mutableSnapshot.value.copy(
                      contextInspection =
                          ContextInspectionState(
                              if (destinationChanged) ContextInspectionStatus.Stale
                              else ContextInspectionStatus.Ready,
                              identity,
                              generation,
                              manifest.takeUnless { destinationChanged },
                              if (destinationChanged)
                                  "The preview destination differs from the captured model. Inspect the current target again."
                              else ""))
            }
          } catch (canceled: CancellationException) {
            throw canceled
          } catch (error: Exception) {
            if (canPublishContextInspection(generation, identity))
                mutableSnapshot.value =
                    mutableSnapshot.value.copy(
                        contextInspection =
                            ContextInspectionState(
                                ContextInspectionStatus.Failed,
                                identity,
                                generation,
                                message =
                                    error.message?.takeIf(String::isNotBlank)
                                        ?: "Context preview failed"))
          }
        }
  }

  private fun contextDestinationMismatch(
      identity: ContextInspectionIdentity,
      manifest: ContextManifest,
  ): Boolean {
    val expectedScope = if (identity.action == "analyze_file") "bug" else "function"
    val captured = identity.model
    return (manifest.scope.isNotBlank() && manifest.scope != expectedScope) ||
        (captured.model.isNotBlank() &&
            manifest.model.isNotBlank() &&
            manifest.model != captured.model) ||
        (captured.providerOrigin.isNotBlank() &&
            manifest.providerOrigin.isNotBlank() &&
            manifest.providerOrigin != captured.providerOrigin) ||
        (captured.model.isNotBlank() &&
            manifest.remoteProvider != null &&
            manifest.remoteProvider != captured.remoteProvider)
  }

  private fun isCurrentContextInspection(identity: ContextInspectionIdentity): Boolean =
      isCurrentContextInspection(identity, controller.state)

  private fun isCurrentContextInspection(
      identity: ContextInspectionIdentity,
      state: DesktopState,
  ): Boolean =
      state.project?.let { project ->
        project.identity() == identity.file.project &&
            state.selectedFile?.identity(project) == identity.file
      } == true &&
          (identity.creationKind.isNotEmpty() || state.selectedSymbol == identity.symbol) &&
          mutableSnapshot.value.model(
              if (identity.action == "analyze_file") ModelScope.Bug else ModelScope.Function) ==
              identity.model

  private fun invalidateContextInspectionIfTargetChanged(
      pendingPath: String? = null,
      creationTargetChanged: Boolean = false,
  ) {
    val current = mutableSnapshot.value.contextInspection
    val identity = current.identity ?: return
    val project = controller.state.project
    if (project == null || project.projectId != identity.file.project.id) {
      closeContextInspection()
      return
    }
    if (current.status == ContextInspectionStatus.Stale ||
        current.status == ContextInspectionStatus.Closed)
        return
    if (!creationTargetChanged && pendingPath == null && isCurrentContextInspection(identity))
        return
    if (!creationTargetChanged && pendingPath != null && pendingPath == identity.file.path) return
    contextInspectionGeneration++
    contextInspectionJob?.cancel()
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            contextInspection =
                current.copy(
                    status = ContextInspectionStatus.Stale,
                    generation = contextInspectionGeneration,
                    message = "The inspection target changed. Inspect the current target again."))
  }

  private fun canPublishContextInspection(
      generation: Long,
      identity: ContextInspectionIdentity,
  ): Boolean =
      generation == contextInspectionGeneration &&
          snapshot.value.contextInspection.identity == identity &&
          snapshot.value.contextInspection.status == ContextInspectionStatus.Loading &&
          isCurrentContextInspection(identity)

  fun cancelContextInspection() {
    val current = snapshot.value.contextInspection
    if (current.status != ContextInspectionStatus.Loading) return
    contextInspectionGeneration++
    contextInspectionJob?.cancel()
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            contextInspection =
                current.copy(
                    status = ContextInspectionStatus.Canceled,
                    generation = contextInspectionGeneration,
                    message = "Context inspection canceled."))
  }

  fun closeContextInspection() {
    contextInspectionGeneration++
    contextInspectionJob?.cancel()
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            contextInspection = ContextInspectionState(generation = contextInspectionGeneration))
  }

  internal fun sendChatMessage(
      mode: ChatEditMode,
      requestedSymbol: String,
      rawIntent: String,
      constraints: String = "",
      repair: Boolean = false,
      creationKind: DeclarationCreationKind? = null,
  ) {
    if (closed) return
    val state = snapshot.value.state
    if (state.chat.pendingRequestId != 0L) return
    val project = state.project ?: return
    val file = state.selectedFile
    val target = validateChatTarget(state.selection, mode, requestedSymbol)
    val workflow = snapshot.value
    val destination = workflow.model(ModelScope.Function)
    val remoteConfirmed = workflow.providerConfirmed(ModelScope.Function)
    val blockedReason =
        assistantComposerBlockedReason(
            mode,
            file,
            target,
            rawIntent,
            workflow.generating,
            workflow.draftValidationInProgress ||
                state.review.editor?.status == DraftEditorStatus.Validating,
            destination,
            remoteConfirmed)
    if (blockedReason != null) {
      dispatch(DesktopEvent.Failed(blockedReason))
      return
    }
    if (file == null) return
    val chatTarget = target.target ?: return
    val request = functionChangeRequest(rawIntent, constraints)
    val content =
        if (mode == ChatEditMode.CreateSymbol && creationKind != null)
            creationMessage(creationKind, chatTarget.symbol, request)
        else request
    val taskSpec =
        state.preparedTaskSpec?.takeIf {
          it.targetPath == file.path &&
              it.targetSymbol == chatTarget.symbol &&
              chatTarget.mode == ChatEditMode.ReplaceSymbol
        }
    val identity =
        WorkflowTaskIdentity(file.identity(project), chatTarget.mode, chatTarget.symbol, taskSpec)
    val (requestId, fileRequest) =
        controller.beginChatAttempt(
            ChatRequestScope(
                project.projectId,
                project.projectRevision,
                file.path,
                file.contentHash,
                chatTarget,
                taskSpec,
                if (mode == ChatEditMode.ReplaceSymbol) state.selectedSymbol else null),
            destination,
            content,
            remoteConfirmed) ?: return
    activeTask = identity
    val matchingSession =
        state.chat.session?.takeIf { chatSessionMatches(it, file, project, chatTarget, taskSpec) }
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("Sending a request for ${chatTarget.symbol} in ${file.path}…"))
    setOperation(generating = true)
    chatJob =
        scope.launch {
          try {
            if (!canDispatchChatRequest(
                requestId, fileRequest, identity, destination, remoteConfirmed))
                return@launch
            val session =
                matchingSession
                    ?: io {
                      api.openChatSession(
                          project.projectId,
                          project.projectRevision,
                          file.contentHash,
                          file.path,
                          chatTarget.mode.wireValue,
                          chatTarget.symbol,
                          taskSpec)
                    }
            if (!chatSessionMatches(session, file, project, chatTarget, taskSpec))
                throw IllegalStateException(
                    "The opened chat session does not match the request target.")
            if (!canDispatchChatRequest(
                requestId, fileRequest, identity, destination, remoteConfirmed)) {
              if (controller.cancelChatLoad(requestId, fileRequest)) publish()
              return@launch
            }
            val proposal = io {
              api.sendChatMessage(
                  session.id, content, session.latestDraftId, remoteConfirmed, repair)
            }
            if (!canDispatchChatRequest(
                requestId, fileRequest, identity, destination, remoteConfirmed)) {
              if (controller.cancelChatLoad(requestId, fileRequest)) publish()
              return@launch
            }
            if (controller.chatProposalLoaded(
                requestId,
                fileRequest,
                session.copy(
                    repairCount = session.repairCount + if (repair) 1 else 0,
                    taskSpec =
                        sessionTaskSpecAfterProposal(session.taskSpec, proposal.draft.taskSpec)),
                content,
                proposal)) {
              activeDraft = proposal.draft.identity(identity.file)
              publish()
              dispatch(DesktopEvent.Status("Draft is ready for review."))
            } else if (controller.chatAttemptFailed(
                requestId,
                fileRequest,
                "The chat response does not match the request target. Send again to retry.")) {
              publish()
            }
          } catch (canceled: CancellationException) {
            if (controller.cancelChatLoad(requestId, fileRequest)) publish()
            throw canceled
          } catch (error: Exception) {
            if (canDispatchChatRequest(
                requestId, fileRequest, identity, destination, remoteConfirmed)) {
              val staleConsent = staleRemoteConfirmationMessage(error, ModelScope.Function)
              val message =
                  staleConsent
                      ?: (error as? ApiException)?.error?.userMessage?.takeIf(String::isNotBlank)
                      ?: "Chat request failed. Send again to retry."
              if (controller.chatAttemptFailed(requestId, fileRequest, message)) publish()
              if (staleConsent != null) setProviderConfirmation(ModelScope.Function, false)
            } else if (controller.cancelChatLoad(requestId, fileRequest)) publish()
          } finally {
            if (chatJob === coroutineContext[Job] && controller.state.chat.pendingRequestId == 0L) {
              chatJob = null
              setOperation(generating = false)
            }
          }
        }
  }

  private fun canDispatchChatRequest(
      requestId: Long,
      file: RequestIdentity,
      identity: WorkflowTaskIdentity,
      destination: ScopedModel,
      remoteConfirmed: Boolean,
  ): Boolean =
      !closed &&
          activeTask == identity &&
          controller.isCurrentChatAttempt(requestId, file) &&
          snapshot.value.model(ModelScope.Function) == destination &&
          snapshot.value.providerConfirmed(ModelScope.Function) == remoteConfirmed &&
          (!destination.remoteProvider || remoteConfirmed)

  private fun invalidateChatAuthorization() {
    val requestId = controller.state.chat.pendingRequestId
    if (requestId == 0L) return
    controller.currentFileRequest()?.let { file ->
      if (controller.cancelChatLoad(requestId, file)) publish()
    }
    activeTask = null
    chatJob?.cancel()
    chatJob = null
    setOperation(generating = false)
  }

  fun reviseWithCheckOutput(mode: ChatEditMode, requestedSymbol: String) {
    val state = snapshot.value.state
    val message =
        repairMessageForChecks(state.chat.session, state.review.draft, state.checks) ?: return
    sendChatMessage(mode, requestedSymbol, message, repair = true)
  }

  fun validateEditableDraft() {
    val state = snapshot.value.state
    val editor = state.review.editor ?: return
    if (editor.status == DraftEditorStatus.Validating) return
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    if (!draftEditorMatchesOpenFile(editor, file, project)) {
      dispatch(DesktopEvent.DraftMarkedStale)
      dispatch(DesktopEvent.Failed("The draft no longer matches the open file."))
      return
    }
    benchmarkWorkflow.stopComparison()
    val (request, fileRequest) = controller.beginDraftValidation() ?: return
    val identity = editor.serverDraft.identity(file.identity(project))
    activeDraft = identity
    draftValidationJob?.cancel()
    publish()
    dispatch(DesktopEvent.Status("Validating ${editor.serverDraft.targetSymbol}…"))
    setOperation(validating = true)
    fun reportFailure(error: Exception, conflict: Boolean) {
      val message = error.message?.takeIf(String::isNotBlank) ?: "Draft validation request failed"
      if (controller.draftValidationStopped(
          request, fileRequest, ValidationAttemptStatus.Failed, message)) {
        publish()
        if (conflict) dispatch(DesktopEvent.DraftMarkedStale)
        dispatch(DesktopEvent.Failed(message))
      }
    }
    draftValidationJob =
        scope.launch {
          try {
            val updated = io {
              api.updateDraft(
                  editor.serverDraft.id,
                  project.projectRevision,
                  editor.serverDraft.revision,
                  editor.declaration,
                  editor.imports)
            }
            if (!controller.draftValidationUpdated(request, fileRequest, updated)) return@launch
            publish()
            val validated = io {
              api.validateDraft(updated.id, project.projectRevision, updated.revision)
            }
            if (activeDraft == identity &&
                controller.draftValidated(request, fileRequest, validated)) {
              activeDraft = validated.identity(identity.file)
              publish()
              dispatch(
                  DesktopEvent.Status(
                      if (validated.validation?.applicable == true) "Draft validation passed."
                      else "Draft validation needs attention."))
            }
          } catch (canceled: CancellationException) {
            if (controller.draftValidationStopped(
                request,
                fileRequest,
                ValidationAttemptStatus.Canceled,
                "Draft validation canceled. Validate the draft again to continue.")) {
              publish()
              dispatch(DesktopEvent.Status("Draft validation canceled"))
            }
            throw canceled
          } catch (error: ApiException) {
            reportFailure(error, error.status == 409)
          } catch (error: Exception) {
            reportFailure(error, false)
          } finally {
            if (draftValidationJob === coroutineContext[Job]) setOperation(validating = false)
          }
        }
  }

  fun runDraftChecks() {
    val state = snapshot.value.state
    val draft = state.review.draft ?: return
    val editor = state.review.editor
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    val eligibility = draftReviewEligibility(editor, draft, null, file, project)
    if (editor?.status != DraftEditorStatus.Valid ||
        !draftEditorMatchesOpenFile(editor, file, project)) {
      dispatch(DesktopEvent.Failed(eligibility.reason))
      return
    }
    val (request, fileRequest) = controller.beginDraftChecks(draft) ?: return
    val identity = draft.identity(file.identity(project))
    activeDraft = identity
    draftChecksJob?.cancel()
    publish()
    dispatch(DesktopEvent.Loading)
    val taskTestName = draft.taskSpec?.goTestCandidate?.name
    dispatch(
        DesktopEvent.Status(
            if (taskTestName == null)
                "Running source-only focused checks for ${draft.targetSymbol}…"
            else
                "Trusting local execution for ${draft.targetSymbol}, then running focused checks…"))
    fun reportFailure(error: Exception, conflict: Boolean) {
      val message = error.message?.takeIf(String::isNotBlank) ?: "Focused checks request failed"
      if (controller.draftChecksStopped(
          request, fileRequest, draft, ValidationAttemptStatus.Failed, message)) {
        publish()
        if (conflict) dispatch(DesktopEvent.DraftMarkedStale)
        dispatch(DesktopEvent.Failed(message))
      }
    }
    draftChecksJob =
        scope.launch {
          try {
            val checks = io {
              if (taskTestName != null) {
                val expectedCommand = listOf("go", "test", "./...", "-run", "^$taskTestName$")
                val trustScope = api.executionTrust(project.projectRevision, taskTestName)
                if (trustScope.commands != listOf(expectedCommand)) {
                  throw IllegalStateException(
                      "Local execution command scope changed; review it again before trusting execution.")
                }
                api.trustProjectExecution(project.projectRevision)
              }
              api.checkDraft(draft.id, project.projectRevision, draft.revision, draft.hash)
            }
            if (activeDraft == identity &&
                controller.draftChecksLoaded(request, fileRequest, draft, checks)) {
              publish()
              dispatch(
                  DesktopEvent.Status(
                      if (checks.applicable) "Focused checks passed."
                      else "Focused checks need attention."))
            } else if (activeDraft == identity) {
              reportFailure(
                  IllegalStateException("Focused checks returned a different draft."), false)
            }
          } catch (canceled: CancellationException) {
            if (controller.draftChecksStopped(
                request,
                fileRequest,
                draft,
                ValidationAttemptStatus.Canceled,
                "Focused checks canceled. Run them again to continue."))
                publish()
            throw canceled
          } catch (error: ApiException) {
            reportFailure(error, error.status == 409)
          } catch (error: Exception) {
            reportFailure(error, false)
          }
        }
  }

  fun loadGoBenchmarks() = benchmarkWorkflow.loadGoBenchmarks()

  fun selectGoBenchmark(choice: GoBenchmarkChoice) = benchmarkWorkflow.selectGoBenchmark(choice)

  fun compareSelectedGoBenchmark() = benchmarkWorkflow.compareSelectedGoBenchmark()

  fun applyEditableDraft() {
    val state = snapshot.value.state
    val draft = state.review.draft ?: return
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    val identity = draft.identity(file.identity(project))
    val eligibility =
        draftReviewEligibility(
            state.review.editor, draft, state.checks, file, project, state.review.checkAttempt)
    if (!eligibility.eligible) {
      dispatch(DesktopEvent.Failed(eligibility.reason))
      return
    }
    benchmarkWorkflow.invalidate()
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("Applying reviewed declaration draft…"))
    scope.launch {
      try {
        val result = io { api.applyDraft(draft) }
        if (activeDraft != null && activeDraft != identity) return@launch
        dispatch(DesktopEvent.Applied(result))
        dispatch(DesktopEvent.Status("Applied ${draft.targetPath}; Undo is available."))
        reloadAfterMutation(identity.file, result.projectRevision)
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (currentDraftIdentity() == identity)
            dispatch(DesktopEvent.Failed(error.message ?: "Apply failed"))
      }
    }
  }

  fun undoAppliedDraft() {
    val state = snapshot.value.state
    val project = state.project ?: return
    val result = state.review.applied ?: return
    val file = state.selectedFile ?: return
    val identity = file.identity(project)
    benchmarkWorkflow.invalidate()
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("Undoing the applied declaration draft…"))
    scope.launch {
      try {
        val undo = io { api.undo(project.projectId, result.projectRevision, result.postApplyHash) }
        if (!isCurrentFile(identity)) return@launch
        dispatch(DesktopEvent.Applied(undo))
        reloadAfterMutation(identity, undo.projectRevision)
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (isCurrentFile(identity)) dispatch(DesktopEvent.Failed(error.message ?: "Undo failed"))
      }
    }
  }

  // Existing page intents converge on the unified admission; old checkboxes cannot grant it
  // consent.
  fun startAnalyzeAll(options: AnalyzeAllRunOptions) =
      previewAnalysis(
          limits =
              AnalysisRunLimits(options.maxFiles, 900, (options.maxRetries + 1).coerceIn(1, 4)))

  fun pauseAnalyzeAll() = pauseAnalysis()

  fun resumeAnalyzeAll(confirmRemoteProvider: Boolean) = resumeAnalysis()

  fun cancelAnalyzeAll() = cancelAnalysis()

  fun previewPerformance(maxFiles: Int = 100, runBudgetSeconds: Int = 900) =
      previewAnalysis(limits = AnalysisRunLimits(maxFiles, runBudgetSeconds, 2))

  fun startPerformance(preview: PerformanceQueuePreview, confirmRemoteProvider: Boolean) =
      previewAnalysis(limits = AnalysisRunLimits(preview.maxFiles, 900, 2))

  fun pausePerformance() = pauseAnalysis()

  fun resumePerformance(confirmRemoteProvider: Boolean) = resumeAnalysis()

  fun cancelPerformance() = cancelAnalysis()

  fun runVerifiedScan() {
    if (verifiedScanProgress(snapshot.value.state).action != VerifiedScanAction.Start) return
    val request = beginVerifiedScanAction() ?: return
    dispatch(DesktopEvent.VerifiedScanOperationUpdated(VerifiedScanOperation.Starting))
    dispatch(DesktopEvent.VerifiedScanFindingsUpdated(VerifiedScanFindingsRefresh.Stale))
    scope.launch {
      var startRequestAttempted = false
      try {
        val report =
            io {
              if (!canInvokeVerifiedScanAction(request)) null
              else {
                val trustScope = api.executionTrust(request.project.revision)
                if (!canInvokeVerifiedScanAction(request)) null
                else {
                  validateVerifiedScanTrust(trustScope, request.project)
                  val acknowledgment = api.trustProjectExecution(request.project.revision)
                  if (!canInvokeVerifiedScanAction(request)) null
                  else {
                    validateVerifiedScanTrust(acknowledgment, request.project)
                    check(acknowledgment.trusted) {
                      "Project-code execution trust was not confirmed; review and retry."
                    }
                    startRequestAttempted = true
                    api.startGoScan(request.project.revision)
                  }
                }
              }
            } ?: return@launch
        if (!isCurrentVerifiedScanAction(request.project, request.generation)) return@launch
        if (rejectForeignVerifiedScan(report, request.project, actionResponse = true)) return@launch
        dispatch(DesktopEvent.VerifiedScanOperationUpdated(VerifiedScanOperation.Idle))
        publishVerifiedScan(request.project, report, request.generation)
      } catch (canceled: CancellationException) {
        throw canceled
      } catch (error: Exception) {
        if (isCurrentVerifiedScanAction(request.project, request.generation)) {
          val detail = error.message?.takeIf(String::isNotBlank) ?: "Request failed"
          dispatch(
              DesktopEvent.VerifiedScanOperationUpdated(
                  if (startRequestAttempted)
                      VerifiedScanOperation.StartUncertain(
                          "Start request failed ($detail). The scan may have started; refresh scan status before trying again.")
                  else VerifiedScanOperation.Failed("Unable to start verified scan: $detail")))
          if (!startRequestAttempted)
              recoverVerifiedScanPolling(request.project, request.generation)
        }
      }
    }
  }

  private fun validateVerifiedScanTrust(
      trust: ExecutionTrust,
      identity: WorkflowProjectIdentity,
  ) {
    check(
        trust.projectId.isNotBlank() &&
            trust.projectRevision.isNotBlank() &&
            trust.projectId == identity.id &&
            trust.projectRevision == identity.revision) {
          "Project-code execution trust identity changed; review and retry."
        }
    check(trust.commands == listOf(listOf("go", "test", "./..."))) {
      "Local execution command scope changed; review it again before trusting execution."
    }
  }

  /** Recovery reads status only; it never grants execution trust or retries an ambiguous start. */
  fun refreshVerifiedScanStatus() {
    val state = snapshot.value.state
    val identity = state.project?.identity() ?: return
    if (identity.id.isBlank() ||
        identity.revision.isBlank() ||
        state.verifiedScan.read == VerifiedScanRead.Reading ||
        state.verifiedScan.operation == VerifiedScanOperation.Starting ||
        state.verifiedScan.operation == VerifiedScanOperation.CancellationRequested)
        return
    supersedeProjectDetailsRefresh(
        "Scan status refresh interrupted the workspace detail refresh. Re-index to retry.")
    val generation = ++verifiedScanActionGeneration
    verifiedScanFindingsGeneration++
    if (state.verifiedScan.findingsRefresh == VerifiedScanFindingsRefresh.Refreshing)
        dispatch(DesktopEvent.VerifiedScanFindingsUpdated(VerifiedScanFindingsRefresh.Stale))
    jobCoordinator.stopVerifiedScanPolling()
    dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Reading))
    scope.launch {
      try {
        val report = io { api.goScan(identity.revision) }
        if (!isCurrentVerifiedScanAction(identity, generation)) return@launch
        if (rejectForeignVerifiedScan(report, identity)) return@launch
        if (state.verifiedScan.operation !is VerifiedScanOperation.CancellationUnconfirmed ||
            report?.isTerminalVerifiedScan() == true ||
            report == null)
            dispatch(DesktopEvent.VerifiedScanOperationUpdated(VerifiedScanOperation.Idle))
        publishVerifiedScan(identity, report, generation)
      } catch (canceled: CancellationException) {
        throw canceled
      } catch (error: Exception) {
        if (isCurrentVerifiedScanAction(identity, generation))
            dispatch(
                DesktopEvent.VerifiedScanReadUpdated(
                    VerifiedScanRead.Unavailable(
                        "Unable to refresh scan status: " +
                            (error.message?.takeIf(String::isNotBlank) ?: "read unavailable"))))
      }
    }
  }

  fun cancelVerifiedScan() {
    if (verifiedScanProgress(snapshot.value.state).action != VerifiedScanAction.Cancel) return
    val request = beginVerifiedScanAction(snapshot.value.state.findings.scan) ?: return
    dispatch(DesktopEvent.VerifiedScanOperationUpdated(VerifiedScanOperation.CancellationRequested))
    scope.launch {
      try {
        val report =
            io {
              if (!canInvokeVerifiedScanAction(request)) null
              else api.cancelGoScan(request.project.revision)
            } ?: return@launch
        if (!isCurrentVerifiedScanAction(request.project, request.generation) ||
            hasTerminalVerifiedScanObservation(request.project))
            return@launch
        if (rejectForeignVerifiedScan(report, request.project, actionResponse = true)) return@launch
        publishVerifiedScan(request.project, report, request.generation)
      } catch (canceled: CancellationException) {
        throw canceled
      } catch (error: Exception) {
        if (isCurrentVerifiedScanAction(request.project, request.generation) &&
            !hasTerminalVerifiedScanObservation(request.project)) {
          val detail = error.message?.takeIf(String::isNotBlank) ?: "Request failed"
          dispatch(
              DesktopEvent.VerifiedScanOperationUpdated(
                  VerifiedScanOperation.CancellationUnconfirmed(
                      "Cancellation could not be confirmed ($detail). Refresh scan status to check the outcome.")))
          recoverVerifiedScanPolling(request.project, request.generation)
        }
      }
    }
  }

  fun discardDraft() {
    cancelGeneration()
    draftValidationJob?.cancel()
    draftChecksJob?.cancel()
    activeTask = null
    activeDraft = null
    dispatch(DesktopEvent.DraftDiscarded)
  }

  private fun cancelAll() {
    closeContextInspection()
    invalidateJobActions()
    benchmarkWorkflow.cancel()
    analysisWorkflow.detach()
    cancelGeneration()
    cancelDraftValidation()
    if (mutableSnapshot.value.declarationExplanation.status ==
        DeclarationExplanationStatus.Loading) {
      cancelDeclarationExplanation()
    } else {
      declarationExplanationGeneration++
      declarationExplanationJob?.cancel()
    }
    jobCoordinator.projectClosed()
  }

  override fun close() {
    closed = true
    cancelAll()
    connectionJob?.cancel()
    startupJob?.cancel()
    projectJob?.cancel()
    acceptedProjects.close()
    preferenceSaveJob.cancel()
    fileFreshnessJob?.cancel()
    cancelFileReadJob()
    enrichmentJobs.forEach(Job::cancel)
    securityWorkflow.cancel()
    jobCoordinator.close()
    lifetime.cancel()
  }

  private fun loadFileEnrichments(request: RequestIdentity) {
    enrichmentJobs =
        listOf(
            scope.launch {
              optionalFileLoad(request, { api.analysis(request.path, request.projectRevision) }) {
                controller.analysisLoaded(request, it)
              }
            },
            scope.launch {
              optionalFileLoad(request, { api.impact(request.path) }) {
                controller.impactLoaded(request, it)
              }
            },
            scope.launch {
              optionalFileLoad(request, { api.gitStatus(request.path) }) {
                controller.gitStatusLoaded(request, it)
              }
            },
        )
  }

  private suspend fun <T> optionalFileLoad(
      request: RequestIdentity,
      requestValue: () -> T,
      accept: (T) -> Boolean
  ) {
    try {
      val value = io(requestValue)
      if (accept(value)) publish()
    } catch (_: CancellationException) {
      throw CancellationException()
    } catch (_: Exception) {
      controller.optionalLoadFailed(request)
    }
  }

  private fun refreshProjectWorkspace(identity: WorkflowProjectIdentity) {
    dispatch(DesktopEvent.ProjectDetailsUpdated(ProjectDetailsOutcome.Refreshing))
    dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Reading))
    analysisWorkflow.refresh()
    val workspaceScanGeneration = verifiedScanActionGeneration
    val detailsGeneration = ++workspaceDetailsGeneration
    fun currentDetails() =
        detailsGeneration == workspaceDetailsGeneration &&
            matchesProject(identity) &&
            isCurrentVerifiedScanAction(identity, workspaceScanGeneration)
    scope.launch {
      try {
        val details = io {
          WorkflowProjectWorkspaceDetails(
              api.overview(identity.revision),
              api.findings(identity.revision),
              api.goScan(identity.revision))
        }
        if (!currentDetails()) return@launch
        if (rejectForeignVerifiedScan(details.scan, identity)) {
          dispatch(
              DesktopEvent.ProjectDetailsUpdated(
                  ProjectDetailsOutcome.Unavailable(
                      "Workspace details include a scan report for another project; previous results were retained.")))
          return@launch
        }
        dispatch(DesktopEvent.OverviewLoaded(details.overview))
        dispatch(DesktopEvent.FindingsLoaded(details.findings.findings))
        publishVerifiedScan(
            identity, details.scan, workspaceScanGeneration, refreshSeedTerminal = false)
        dispatch(DesktopEvent.ProjectDetailsUpdated(ProjectDetailsOutcome.Available))
      } catch (canceled: CancellationException) {
        throw canceled
      } catch (error: Exception) {
        if (currentDetails()) {
          val detail = error.message?.takeIf(String::isNotBlank) ?: "read unavailable"
          dispatch(
              DesktopEvent.ProjectDetailsUpdated(
                  ProjectDetailsOutcome.Unavailable(
                      "Project inventory is available; workspace details could not be refreshed: $detail")))
          dispatch(
              DesktopEvent.VerifiedScanReadUpdated(
                  VerifiedScanRead.Unavailable(
                      "Workspace details could not be read; scan status is unconfirmed: $detail")))
        }
      }
    }
  }

  private fun refreshFindings(
      identity: WorkflowProjectIdentity,
      scanActionGeneration: Long? = null,
  ) {
    val findingsGeneration = ++verifiedScanFindingsGeneration
    if (scanActionGeneration != null)
        dispatch(DesktopEvent.VerifiedScanFindingsUpdated(VerifiedScanFindingsRefresh.Refreshing))
    fun current() =
        matchesProject(identity) &&
            findingsGeneration == verifiedScanFindingsGeneration &&
            (scanActionGeneration == null ||
                isCurrentVerifiedScanAction(identity, scanActionGeneration))
    scope.launch {
      try {
        val response = io { api.findings(identity.revision) }
        if (current()) {
          dispatch(DesktopEvent.FindingsLoaded(response.findings))
          if (scanActionGeneration != null)
              dispatch(
                  DesktopEvent.VerifiedScanFindingsUpdated(VerifiedScanFindingsRefresh.Current))
        }
      } catch (canceled: CancellationException) {
        throw canceled
      } catch (error: Exception) {
        if (scanActionGeneration != null && current())
            dispatch(
                DesktopEvent.VerifiedScanFindingsUpdated(
                    VerifiedScanFindingsRefresh.Unavailable(
                        "Scan findings could not be refreshed; previous rows may be stale: " +
                            (error.message?.takeIf(String::isNotBlank) ?: "read unavailable"))))
      }
    }
  }

  private fun publishVerifiedScan(
      identity: WorkflowProjectIdentity,
      scan: GoScanReport?,
      actionGeneration: Long = verifiedScanActionGeneration,
      refreshSeedTerminal: Boolean = true,
  ) {
    if (!isCurrentVerifiedScanAction(identity, actionGeneration) ||
        rejectForeignVerifiedScan(scan, identity))
        return
    fun isCurrentCancelPoll(): Boolean =
        matchesProject(identity) &&
            actionGeneration == verifiedScanActionGeneration - 1 &&
            snapshot.value.state.verifiedScan.operation ==
                VerifiedScanOperation.CancellationRequested &&
            snapshot.value.state.findings.scan?.belongsTo(identity) == true
    var terminalFromCancelPoll = false
    jobCoordinator.observeVerifiedScan(
        identity,
        scan,
        onUpdate = { updated ->
          val cancelPoll = isCurrentCancelPoll()
          if (!isCurrentVerifiedScanAction(identity, actionGeneration) && !cancelPoll)
              return@observeVerifiedScan false
          if (rejectForeignVerifiedScan(updated, identity, polling = true))
              return@observeVerifiedScan false
          dispatch(DesktopEvent.GoScanLoaded(updated))
          val terminal =
              updated?.status?.lowercase() in setOf("completed", "failed", "canceled", "cancelled")
          terminalFromCancelPoll = cancelPoll && terminal
          val operation = snapshot.value.state.verifiedScan.operation
          if (terminal &&
              (operation == VerifiedScanOperation.CancellationRequested ||
                  operation is VerifiedScanOperation.CancellationUnconfirmed))
              dispatch(DesktopEvent.VerifiedScanOperationUpdated(VerifiedScanOperation.Idle))
          else if (!shouldPollVerifiedScan(updated) &&
              operation == VerifiedScanOperation.CancellationRequested)
              dispatch(
                  DesktopEvent.VerifiedScanOperationUpdated(
                      VerifiedScanOperation.CancellationUnconfirmed(
                          "Cancellation is not confirmed by a terminal report. Refresh scan status to check the outcome.")))
          true
        },
        onSeedTerminal = {
          if (refreshSeedTerminal &&
              isCurrentVerifiedScanAction(identity, actionGeneration) &&
              scan?.isTerminalVerifiedScan() == true)
              refreshFindings(identity, actionGeneration)
        },
        onPollTerminal = {
          if ((isCurrentVerifiedScanAction(identity, actionGeneration) || terminalFromCancelPoll) &&
              snapshot.value.state.findings.scan?.isTerminalVerifiedScan() == true)
              refreshFindings(identity, verifiedScanActionGeneration)
        },
        fetch = { io { api.goScan(identity.revision) } },
        onFailure = { error ->
          if (isCurrentVerifiedScanAction(identity, actionGeneration) || isCurrentCancelPoll()) {
            dispatch(
                DesktopEvent.VerifiedScanReadUpdated(
                    VerifiedScanRead.PollUnavailable(
                        "Live scan status could not be read: " +
                            (error.message?.takeIf(String::isNotBlank) ?: "read unavailable"))))
            if (snapshot.value.state.verifiedScan.operation ==
                VerifiedScanOperation.CancellationRequested)
                dispatch(
                    DesktopEvent.VerifiedScanOperationUpdated(
                        VerifiedScanOperation.CancellationUnconfirmed(
                            "Live status is unavailable; cancellation is not confirmed. Refresh scan status to check the outcome.")))
          }
        })
  }

  private fun rejectForeignVerifiedScan(
      scan: GoScanReport?,
      identity: WorkflowProjectIdentity,
      actionResponse: Boolean = false,
      polling: Boolean = false,
  ): Boolean {
    if (scan == null || scan.belongsTo(identity)) return false
    val message =
        "Verified scan report identity does not match the current project; refresh status before relying on it."
    if (actionResponse) {
      dispatch(DesktopEvent.VerifiedScanOperationUpdated(VerifiedScanOperation.Failed(message)))
    } else {
      dispatch(
          DesktopEvent.VerifiedScanReadUpdated(
              if (polling) VerifiedScanRead.PollUnavailable(message)
              else VerifiedScanRead.Unavailable(message)))
      if (polling &&
          snapshot.value.state.verifiedScan.operation ==
              VerifiedScanOperation.CancellationRequested)
          dispatch(
              DesktopEvent.VerifiedScanOperationUpdated(
                  VerifiedScanOperation.CancellationUnconfirmed(
                      "Cancellation is not confirmed by a matching report. Refresh scan status to check the outcome.")))
    }
    return true
  }

  private fun reloadAfterMutation(file: WorkflowFileIdentity, revision: String) {
    scope.launch {
      try {
        val (index, loaded, symbols) =
            io { Triple(api.index(), api.fileInfo(file.path), api.symbols(file.path).symbols) }
        if (!matchesProject(file.project)) return@launch
        dispatch(DesktopEvent.IndexRefreshed(index))
        dispatch(DesktopEvent.FileLoaded(loaded, symbols))
        dispatch(DesktopEvent.Status("Project refreshed."))
        refreshProjectWorkspace(WorkflowProjectIdentity(file.project.id, revision))
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (matchesProject(file.project))
            dispatch(DesktopEvent.Failed(error.message ?: "Project refresh failed"))
      }
    }
  }

  private fun cancelProjectScopedWork() {
    jobCoordinator.projectClosed()
    fileFreshnessJob?.cancel()
    cancelFileReadJob()
    enrichmentJobs.forEach(Job::cancel)
    securityWorkflow.cancel()
    cancelAll()
  }

  private fun beginVerifiedScanAction(
      expectedScan: GoScanReport? = null,
  ): VerifiedScanActionRequest? {
    val identity = snapshot.value.state.project?.identity() ?: return null
    supersedeProjectDetailsRefresh(
        "Verified scan action interrupted the workspace detail refresh. Re-index to retry.")
    verifiedScanFindingsGeneration++
    return VerifiedScanActionRequest(
        identity,
        ++verifiedScanActionGeneration,
        expectedScan,
        requiresExpectedScan = expectedScan != null)
  }

  private fun GoScanReport.isTerminalVerifiedScan(): Boolean =
      status.lowercase() in setOf("completed", "failed", "canceled", "cancelled")

  // Polling can observe completion while a DELETE is still in flight. Its terminal report is
  // newer evidence than either a delayed DELETE response or a transport failure.
  private fun hasTerminalVerifiedScanObservation(identity: WorkflowProjectIdentity): Boolean {
    val scan = snapshot.value.state.findings.scan ?: return false
    return scan.belongsTo(identity) && scan.isTerminalVerifiedScan()
  }

  private fun recoverVerifiedScanPolling(identity: WorkflowProjectIdentity, generation: Long) {
    if (!isCurrentVerifiedScanAction(identity, generation)) return
    val scan = snapshot.value.state.findings.scan ?: return
    if (!scan.belongsTo(identity)) return
    publishVerifiedScan(identity, scan, generation, refreshSeedTerminal = false)
  }

  private fun supersedeProjectDetailsRefresh(message: String) {
    workspaceDetailsGeneration++
    if (controller.state.projectState.detailsOutcome == ProjectDetailsOutcome.Refreshing) {
      dispatch(DesktopEvent.ProjectDetailsUpdated(ProjectDetailsOutcome.Unavailable(message)))
      if (snapshot.value.state.verifiedScan.read == VerifiedScanRead.Reading)
          dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Unavailable(message)))
    }
  }

  private fun invalidateJobActions() {
    verifiedScanActionGeneration++
    verifiedScanFindingsGeneration++
  }

  private fun isCurrentVerifiedScanAction(
      identity: WorkflowProjectIdentity,
      generation: Long,
  ): Boolean = generation == verifiedScanActionGeneration && matchesProject(identity)

  private fun canInvokeVerifiedScanAction(request: VerifiedScanActionRequest): Boolean =
      isCurrentVerifiedScanAction(request.project, request.generation) &&
          (!request.requiresExpectedScan ||
              snapshot.value.state.findings.scan == request.expectedScan)

  private fun modelRequestFailureMessage(
      error: Exception,
      scope: ModelScope,
      fallback: String
  ): String {
    val staleConfirmation = staleRemoteConfirmationMessage(error, scope)
    if (staleConfirmation != null) setProviderConfirmation(scope, false)
    return staleConfirmation ?: error.message?.takeIf(String::isNotBlank) ?: fallback
  }

  private fun clearSecurityReviewRemoteConfirmation() {
    if (mutableSnapshot.value.securityReviewRemoteConfirmed)
        mutableSnapshot.value = mutableSnapshot.value.copy(securityReviewRemoteConfirmed = false)
  }

  private fun publish() {
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            state = controller.state,
            analysisInProgress =
                controller.state.analysisRun.run?.isActive() == true ||
                    controller.state.analysisRun.action.isNotEmpty())
  }

  private fun invalidateDeclarationExplanation(message: String) {
    declarationExplanationGeneration++
    declarationExplanationJob?.cancel()
    val current = mutableSnapshot.value.declarationExplanation
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            declarationExplanation =
                current.copy(status = DeclarationExplanationStatus.Stale, message = message))
  }

  private fun setOperation(generating: Boolean? = null, validating: Boolean? = null) {
    val current = mutableSnapshot.value
    mutableSnapshot.value =
        current.copy(
            generating = generating ?: current.generating,
            draftValidationInProgress = validating ?: current.draftValidationInProgress,
        )
  }

  private fun matchesProject(identity: WorkflowProjectIdentity): Boolean =
      snapshot.value.state.project?.identity() == identity

  private fun isCurrentFile(identity: WorkflowFileIdentity): Boolean {
    val state = snapshot.value.state
    val project = state.project ?: return false
    return state.selectedFile?.identity(project) == identity
  }

  private fun currentDraftIdentity(): WorkflowDraftIdentity? {
    val state = snapshot.value.state
    val project = state.project ?: return null
    val file = state.selectedFile ?: return null
    val draft = state.review.draft ?: return null
    return draft.identity(file.identity(project))
  }

  private suspend fun <T> io(block: () -> T): T =
      withContext(ioDispatcher) { runInterruptible { block() } }
}

private fun ProjectAnalysis.identity(): WorkflowProjectIdentity =
    WorkflowProjectIdentity(projectId, projectRevision)

private fun ProjectFileInfo.identity(project: ProjectAnalysis): WorkflowFileIdentity =
    WorkflowFileIdentity(project.identity(), path, contentHash)

private fun selectedDeclarationTarget(state: DesktopState): DeclarationExplanationTarget? {
  val project = state.project ?: return null
  val file = state.selectedFile ?: return null
  val symbol = state.selectedSymbol ?: return null
  if (!symbolEditEligibility(file, state.symbols, symbol).eligible) return null
  return DeclarationExplanationTarget(
      file.identity(project), symbol.name, symbol.signature, symbol.startLine, symbol.endLine)
}

private fun explanationMatchesTarget(
    explanation: DeclarationExplanation,
    target: DeclarationExplanationTarget,
): Boolean =
    explanation.projectId == target.file.project.id &&
        explanation.projectRevision == target.file.project.revision &&
        explanation.baseFileHash == target.file.contentHash &&
        explanation.anchor.path == target.file.path &&
        explanation.anchor.symbol == target.symbol &&
        explanation.anchor.signature == target.signature &&
        explanation.anchor.startLine == target.startLine &&
        explanation.anchor.endLine == target.endLine

private fun explanationFailureState(
    error: Exception,
    target: DeclarationExplanationTarget,
): DeclarationExplanationState =
    if (error is ApiException && error.status == 409)
        DeclarationExplanationState(
            status = DeclarationExplanationStatus.Stale,
            target = target,
            result = null,
            message = "The project or declaration changed. Request a fresh explanation.")
    else
        DeclarationExplanationState(
            status = DeclarationExplanationStatus.Failed,
            target = target,
            result = null,
            message = error.message ?: "Declaration explanation failed")

private fun GoScanReport.belongsTo(identity: WorkflowProjectIdentity): Boolean =
    projectId == identity.id && projectRevision == identity.revision

private fun DeclarationDraft.identity(file: WorkflowFileIdentity): WorkflowDraftIdentity =
    WorkflowDraftIdentity(file, id, revision, hash)

private data class WorkflowProjectWorkspaceDetails(
    val overview: ProjectOverview,
    val findings: FindingsResponse,
    val scan: GoScanReport?,
)
