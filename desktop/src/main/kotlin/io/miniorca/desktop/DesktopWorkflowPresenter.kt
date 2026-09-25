package io.miniorca.desktop

import java.io.File
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
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

data class DesktopWorkflowSnapshot(
    val state: DesktopState = DesktopState(),
    val modelCatalog: ModelCatalog = ModelCatalog(),
    val providerConfirmations: ScopedConfirmationState = ScopedConfirmationState(),
    val securityReviewRemoteConfirmed: Boolean = false,
    val contextManifest: ContextManifest? = null,
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
  private var projectJob: Job? = null
  private var fileJob: Job? = null
  private var fileFreshnessJob: Job? = null
  private var enrichmentJobs: List<Job> = emptyList()
  private var declarationExplanationJob: Job? = null
  private var chatJob: Job? = null
  private var draftValidationJob: Job? = null
  private var draftChecksJob: Job? = null
  private var declarationExplanationGeneration = 0L
  private var verifiedScanActionGeneration = 0L
  private var activeTask: WorkflowTaskIdentity? = null
  private var activeDraft: WorkflowDraftIdentity? = null

  val snapshot: StateFlow<DesktopWorkflowSnapshot> = mutableSnapshot.asStateFlow()

  fun start() {
    refreshConnection()
    lastProjectStore.load()?.let { loadProject(it, restore = true) }
  }

  fun dispatch(event: DesktopEvent) {
    val before = selectedDeclarationTarget(controller.state)
    analysisWorkflow.beforeEvent(event)
    benchmarkWorkflow.beforeEvent(event)
    securityWorkflow.beforeEvent(event)
    if (event is DesktopEvent.ProjectLoaded) {
      invalidateJobActions()
      jobCoordinator.projectOpened(event.project.identity())
    }
    controller.dispatch(event)
    if (event is DesktopEvent.DraftEdited) {
      draftValidationJob?.cancel()
      setOperation(validating = false)
    }
    if (event is DesktopEvent.IndexRefreshed &&
        snapshot.value.state.project?.projectRevision != event.index.projectRevision) {
      controller.state.project?.identity()?.let(jobCoordinator::projectOpened)
    }
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
    mutableSnapshot.value =
        mutableSnapshot.value.copy(
            providerConfirmations =
                mutableSnapshot.value.providerConfirmations.withConfirmation(scope, confirmed),
        )
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
            if (previous.modelCatalog.identity() != catalog.identity())
                analysisWorkflow.providerChanged()
            else analysisWorkflow.refresh()
            val function = catalog.forScope(ModelScope.Function)
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
    projectJob?.cancel()
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
            try {
              lastProjectStore.save(project.path)
            } catch (_: Exception) {
              // Project restore is optional; a persistence failure must not discard the loaded
              // project.
            }
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
                    error.message?.takeIf(String::isNotBlank) ?: "Could not reopen the last project"
                else modelRequestFailureMessage(error, ModelScope.Analyze, "Import failed")
            if (controller.projectFailed(request, message)) publish()
          }
        }
  }

  fun reanalyze() {
    val project = snapshot.value.state.project ?: return
    clearSecurityReviewRemoteConfirmation()
    cancelProjectScopedWork()
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("Refreshing deterministic project facts…"))
    projectJob =
        scope.launch {
          try {
            val index = io { api.reindex(project.projectRevision) }
            if (!matchesProject(project.identity())) return@launch
            dispatch(DesktopEvent.IndexRefreshed(index))
            val refreshed = snapshot.value.state.project?.identity() ?: return@launch
            jobCoordinator.projectOpened(refreshed)
            refreshProjectWorkspace(refreshed)
          } catch (_: CancellationException) {
            throw CancellationException()
          } catch (error: Exception) {
            if (matchesProject(project.identity())) {
              jobCoordinator.projectOpened(project.identity())
              analysisWorkflow.refresh()
              dispatch(DesktopEvent.Failed(error.message ?: "Re-analysis failed"))
            }
          }
        }
  }

  fun selectFile(
      path: String,
      editorTarget: EditorNavigationTarget? = null,
      preparedFixRequest: String? = null,
      preparedTaskSpec: BugTaskSpec? = null,
  ) {
    clearSecurityReviewRemoteConfirmation()
    benchmarkWorkflow.invalidate()
    if (mutableSnapshot.value.declarationExplanation.status !=
        DeclarationExplanationStatus.Unavailable) {
      invalidateDeclarationExplanation(
          "Selection changed. Request a new explanation for the current declaration.")
    }
    chatJob?.cancel()
    draftValidationJob?.cancel()
    draftChecksJob?.cancel()
    securityWorkflow.cancel()
    activeTask = null
    activeDraft = null
    val request = controller.beginFileLoad(path) ?: return
    publish()
    fileFreshnessJob?.cancel()
    fileJob?.cancel()
    enrichmentJobs.forEach(Job::cancel)
    fileJob =
        scope.launch {
          try {
            val (file, symbols) = io { api.fileInfo(path) to api.symbols(path).symbols }
            if (!controller.fileLoaded(request, file, symbols)) return@launch
            publish()
            editorTarget?.let { target ->
              val selection = resolveEditorNavigation(symbols, target)
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
            val loaded = controller.currentFileRequest() ?: return@launch
            loadFileEnrichments(loaded)
          } catch (_: CancellationException) {
            controller.cancelFileLoad(request)
            publish()
          } catch (error: Exception) {
            if (controller.fileFailed(
                request, error.message?.takeIf(String::isNotBlank) ?: "File load failed"))
                publish()
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
    if (snapshot.value.state.index?.files?.any { it.path == path } != true) {
      dispatch(
          DesktopEvent.Failed(
              "This file no longer points to an indexed file in the active project."))
      return
    }
    dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
    selectFile(path, editorTarget, preparedFixRequest, preparedTaskSpec)
  }

  fun openFinding(finding: UnifiedFinding) {
    val target = findingNavigationTarget(finding, snapshot.value.state.index)
    if (target == null)
        dispatch(
            DesktopEvent.Failed("This finding no longer points to a file in the active project."))
    else openFileInEditor(target.path, target)
  }

  fun prepareFinding(finding: UnifiedFinding) {
    if (!findingCanPrepareFix(finding)) {
      dispatch(DesktopEvent.Failed("Refresh this finding before preparing a fix."))
      return
    }
    val target = findingTaskNavigationTarget(finding)
    val requirement = findingTaskRequirement(finding)
    if (target == null ||
        requirement == null ||
        snapshot.value.state.index?.files?.any { it.path == target.path } != true) {
      dispatch(
          DesktopEvent.Failed("This finding no longer points to a file in the active project."))
      return
    }
    openFileInEditor(target.path, target, requirement, finding.taskSpec)
  }

  fun viewAnalysisResults(category: String, path: String) {
    if (category !in setOf("bugs", "performance", "security")) return
    if (path.isNotBlank() && snapshot.value.state.index?.files?.none { it.path == path } != false)
        return
    dispatch(DesktopEvent.WorkspaceSelected(analysisCategoryWorkspace(category)))
  }

  fun preparePerformanceFinding(path: String, finding: PerformanceFinding) {
    val state = snapshot.value.state
    val result =
        performanceResults(state.analysisResultPage("performance")).firstOrNull {
          it.report.path == path && it.finding == finding
        }
    if (result == null || !performanceCanPrepare(result, state.index)) {
      dispatch(
          DesktopEvent.Failed(
              "Refresh this opportunity; preparation requires one exact eligible Go declaration."))
      return
    }
    openFileInEditor(
        path,
        EditorNavigationTarget(path, finding.symbol, finding.startLine),
        "Optimize ${finding.symbol} without changing behavior. Observed pattern: ${finding.observedPattern} Trade-off: ${finding.tradeoff}")
  }

  fun scanSecurity() = securityWorkflow.scanSecurity()

  fun reviewSecurity() = previewAnalysis()

  fun openSecurityFinding(finding: SecurityFinding) {
    val state = snapshot.value.state
    val target =
        finding
            .takeIf {
              securityResults(state.analysisResultPage("security")).any { result ->
                result.finding == it
              } || securityFindingIsCurrent(it, state)
            }
            ?.let { securityFindingNavigationTarget(it, state.index) }
    if (target == null)
        dispatch(DesktopEvent.Failed("This security finding no longer points to an indexed file."))
    else openFileInEditor(target.path, target)
  }

  fun prepareSecurityFinding(finding: SecurityFinding) {
    val state = snapshot.value.state
    if (!securityFindingIsCurrent(finding, state)) {
      dispatch(DesktopEvent.Failed("Refresh Security results before preparing a fix."))
      return
    }
    val indexed = state.index?.files?.firstOrNull { it.path == finding.anchor.path }
    val exact =
        indexed?.symbols?.singleOrNull {
          it.name == finding.anchor.symbol && it.atomicTarget && it.confidence == "exact"
        }
    if (indexed?.language != "Go" ||
        exact == null ||
        !securityAnchorWithinDeclaration(finding.anchor, indexed, exact)) {
      dispatch(
          DesktopEvent.Failed(
              "Prepare fix is available only for one exact supported Go declaration."))
      return
    }
    openFileInEditor(
        indexed.path,
        EditorNavigationTarget(indexed.path, exact.name, finding.anchor.startLine),
        "Address the reviewed security finding in ${exact.name}.\nObserved condition: ${finding.observedCondition}\nRemediation: ${finding.remediation}\nKeep the change limited to this declaration.")
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

  fun startAnalysis() = analysisWorkflow.admit()

  fun pauseAnalysis() = analysisWorkflow.control("pause")

  fun refreshAnalysis() = analysisWorkflow.refresh()

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
    val state = snapshot.value.state
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    val symbol = state.selectedSymbol
    val target = selectedDeclarationTarget(state)
    if (symbol == null ||
        target == null ||
        file.language != "Go" ||
        !symbol.atomicTarget ||
        symbol.confidence != "exact") {
      mutableSnapshot.value =
          mutableSnapshot.value.copy(
              declarationExplanation =
                  DeclarationExplanationState(
                      status = DeclarationExplanationStatus.Unavailable,
                      message = "Select one exact atomic Go declaration to explain."))
      return
    }
    if (snapshot.value.model(ModelScope.Function).remoteProvider &&
        !snapshot.value.providerConfirmed(ModelScope.Function)) {
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
                    message = "Explaining ${symbol.name}…"))
    declarationExplanationJob =
        scope.launch {
          try {
            val result = io {
              api.explainDeclaration(
                  project.projectId,
                  project.projectRevision,
                  file.contentHash,
                  file.path,
                  symbol.name,
                  snapshot.value.providerConfirmed(ModelScope.Function))
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
    activeTask = null
    chatJob?.cancel()
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

  fun inspectContext(action: String) {
    val file = snapshot.value.state.selectedFile ?: return
    val identity = file.identity(snapshot.value.state.project ?: return)
    scope.launch {
      try {
        val manifest = io { api.context(file.path, action) }
        if (isCurrentFile(identity))
            mutableSnapshot.value = mutableSnapshot.value.copy(contextManifest = manifest)
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (isCurrentFile(identity))
            dispatch(DesktopEvent.Failed(error.message ?: "Context preview failed"))
      }
    }
  }

  fun clearContextManifest() {
    mutableSnapshot.value = mutableSnapshot.value.copy(contextManifest = null)
  }

  fun sendChatMessage(
      mode: ChatEditMode,
      requestedSymbol: String,
      message: String,
      repair: Boolean = false
  ) {
    val state = snapshot.value.state
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    val target =
        validateChatTarget(file, state.symbols, state.selectedSymbol, mode, requestedSymbol)
    if (!target.valid) {
      dispatch(DesktopEvent.Failed(target.message))
      return
    }
    val content = message.trim()
    if (!hasFunctionChangeIntent(content)) {
      val guidance =
          if (content.isBlank()) "Write a concise intent before sending."
          else "Add a concise intent after the selected preset before sending."
      dispatch(DesktopEvent.Failed(guidance))
      return
    }
    if (snapshot.value.model(ModelScope.Function).remoteProvider &&
        !snapshot.value.providerConfirmed(ModelScope.Function)) {
      dispatch(
          DesktopEvent.Failed(
              "Confirm the Function edits model destination before sending context."))
      return
    }
    val (request, fileRequest) = controller.beginChatLoad() ?: return
    val chatTarget = target.target ?: return
    val taskSpec =
        state.preparedTaskSpec?.takeIf {
          it.targetPath == file.path &&
              it.targetSymbol == chatTarget.symbol &&
              chatTarget.mode == ChatEditMode.ReplaceSymbol
        }
    val identity =
        WorkflowTaskIdentity(file.identity(project), chatTarget.mode, chatTarget.symbol, taskSpec)
    activeTask = identity
    val matchingSession =
        state.chat.session?.takeIf { chatSessionMatches(it, file, project, chatTarget, taskSpec) }
    chatJob?.cancel()
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("Sending a request for ${chatTarget.symbol} in ${file.path}…"))
    setOperation(generating = true)
    chatJob =
        scope.launch {
          try {
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
            val proposal = io {
              api.sendChatMessage(
                  session.id,
                  content,
                  session.latestDraftId,
                  snapshot.value.providerConfirmed(ModelScope.Function),
                  repair)
            }
            if (activeTask == identity &&
                controller.chatProposalLoaded(
                    request,
                    fileRequest,
                    session.copy(
                        repairCount = session.repairCount + if (repair) 1 else 0,
                        taskSpec =
                            sessionTaskSpecAfterProposal(
                                session.taskSpec, proposal.draft.taskSpec)),
                    content,
                    proposal)) {
              activeDraft = proposal.draft.identity(identity.file)
              publish()
              dispatch(DesktopEvent.Status("Draft is ready for review."))
            }
          } catch (_: CancellationException) {
            if (controller.cancelChatLoad(request, fileRequest)) publish()
            throw CancellationException()
          } catch (error: Exception) {
            if (activeTask == identity && controller.cancelChatLoad(request, fileRequest)) {
              val message =
                  modelRequestFailureMessage(error, ModelScope.Function, "Chat request failed")
              dispatch(DesktopEvent.ChatRequestFailed(ChatRequestFailure(chatTarget, message)))
            }
          } finally {
            if (chatJob === coroutineContext[Job]) setOperation(generating = false)
          }
        }
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
    val request = beginVerifiedScanAction() ?: return
    scope.launch {
      try {
        val report =
            io {
              if (!canInvokeVerifiedScanAction(request)) null
              else {
                val trustScope = api.executionTrust(request.project.revision)
                if (trustScope.commands != listOf(listOf("go", "test", "./..."))) {
                  throw IllegalStateException(
                      "Local execution command scope changed; review it again before trusting execution.")
                }
                api.trustProjectExecution(request.project.revision)
                api.startGoScan(request.project.revision)
              }
            } ?: return@launch
        if (!isCurrentVerifiedScanAction(request.project, request.generation)) return@launch
        publishVerifiedScan(request.project, report, request.generation)
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (isCurrentVerifiedScanAction(request.project, request.generation)) {
          dispatch(DesktopEvent.Failed(error.message ?: "Unable to start verified scan"))
          recoverVerifiedScanPolling(request.project, request.generation)
        }
      }
    }
  }

  fun cancelVerifiedScan() {
    val request = beginVerifiedScanAction(snapshot.value.state.findings.scan) ?: return
    scope.launch {
      try {
        val report =
            io {
              if (!canInvokeVerifiedScanAction(request)) null
              else api.cancelGoScan(request.project.revision)
            } ?: return@launch
        if (!isCurrentVerifiedScanAction(request.project, request.generation)) return@launch
        publishVerifiedScan(request.project, report, request.generation)
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (isCurrentVerifiedScanAction(request.project, request.generation)) {
          dispatch(DesktopEvent.Failed(error.message ?: "Unable to cancel verified scan"))
          recoverVerifiedScanPolling(request.project, request.generation)
        }
      }
    }
  }

  fun discardDraft() {
    chatJob?.cancel()
    draftValidationJob?.cancel()
    draftChecksJob?.cancel()
    activeTask = null
    activeDraft = null
    dispatch(DesktopEvent.DraftDiscarded)
  }

  private fun cancelAll() {
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
    cancelAll()
    connectionJob?.cancel()
    projectJob?.cancel()
    fileFreshnessJob?.cancel()
    fileJob?.cancel()
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
    analysisWorkflow.refresh()
    val workspaceScanGeneration = verifiedScanActionGeneration
    scope.launch {
      try {
        val details = io {
          WorkflowProjectWorkspaceDetails(
              api.overview(identity.revision),
              api.findings(identity.revision),
              api.goScan(identity.revision))
        }
        if (!matchesProject(identity)) return@launch
        dispatch(DesktopEvent.OverviewLoaded(details.overview))
        if (isCurrentVerifiedScanAction(identity, workspaceScanGeneration))
            dispatch(DesktopEvent.FindingsLoaded(details.findings.findings))
        publishVerifiedScan(
            identity, details.scan, workspaceScanGeneration, refreshSeedTerminal = false)
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (_: Exception) {
        if (matchesProject(identity))
            dispatch(
                DesktopEvent.Status(
                    "Project facts are available; workspace details could not be refreshed."))
      }
    }
  }

  private fun refreshFindings(
      identity: WorkflowProjectIdentity,
      scanActionGeneration: Long? = null,
  ) {
    scope.launch {
      try {
        val response = io { api.findings(identity.revision) }
        if (matchesProject(identity) &&
            (scanActionGeneration == null ||
                isCurrentVerifiedScanAction(identity, scanActionGeneration)))
            dispatch(DesktopEvent.FindingsLoaded(response.findings))
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (_: Exception) {
        // Findings refresh is enrichment; preserve visible deterministic state.
      }
    }
  }

  private fun publishVerifiedScan(
      identity: WorkflowProjectIdentity,
      scan: GoScanReport?,
      actionGeneration: Long = verifiedScanActionGeneration,
      refreshSeedTerminal: Boolean = true,
  ) {
    if (!isCurrentVerifiedScanAction(identity, actionGeneration)) return
    jobCoordinator.observeVerifiedScan(
        identity,
        scan,
        onUpdate = { updated ->
          if (!isCurrentVerifiedScanAction(identity, actionGeneration))
              return@observeVerifiedScan false
          dispatch(DesktopEvent.GoScanLoaded(updated))
          true
        },
        onSeedTerminal = {
          if (refreshSeedTerminal && isCurrentVerifiedScanAction(identity, actionGeneration))
              refreshFindings(identity, actionGeneration)
        },
        onPollTerminal = {
          if (isCurrentVerifiedScanAction(identity, actionGeneration))
              refreshFindings(identity, actionGeneration)
        },
        fetch = { io { api.goScan(identity.revision) } },
        onFailure = { error ->
          if (isCurrentVerifiedScanAction(identity, actionGeneration))
              dispatch(DesktopEvent.Failed(error.message ?: "Verified scan status failed"))
        })
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
    fileJob?.cancel()
    enrichmentJobs.forEach(Job::cancel)
    securityWorkflow.cancel()
    cancelAll()
  }

  private fun beginVerifiedScanAction(
      expectedScan: GoScanReport? = null,
  ): VerifiedScanActionRequest? {
    val identity = snapshot.value.state.project?.identity() ?: return null
    return VerifiedScanActionRequest(
        identity,
        ++verifiedScanActionGeneration,
        expectedScan,
        requiresExpectedScan = expectedScan != null)
  }

  private fun recoverVerifiedScanPolling(identity: WorkflowProjectIdentity, generation: Long) {
    if (!isCurrentVerifiedScanAction(identity, generation)) return
    val scan = snapshot.value.state.findings.scan ?: return
    if (!scan.belongsTo(identity)) return
    publishVerifiedScan(identity, scan, generation)
  }

  private fun invalidateJobActions() {
    verifiedScanActionGeneration++
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
    return staleConfirmation ?: error.message ?: fallback
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
  if (file.language != "Go" || !symbol.atomicTarget || symbol.confidence != "exact") return null
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
