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
    val contextManifest: ContextManifest? = null,
    val analysisInProgress: Boolean = false,
    val generating: Boolean = false,
    val draftValidationInProgress: Boolean = false,
    val declarationExplanation: DeclarationExplanationState = DeclarationExplanationState(),
) {
  fun model(scope: ModelScope): ScopedModel = modelCatalog.forScope(scope)

  fun providerConfirmed(scope: ModelScope): Boolean = providerConfirmations.confirmed(scope)
}

private data class AnalyzeAllActionRequest(
    val project: WorkflowProjectIdentity,
    val generation: Long,
    val expectedJob: AnalyzeAllJob? = null,
    val requiresExpectedJob: Boolean = false,
)

private data class PerformanceActionRequest(
    val project: WorkflowProjectIdentity,
    val generation: Long,
    val expectedJob: PerformanceJob? = null,
    val requiresExpectedJob: Boolean = false,
)

private data class VerifiedScanActionRequest(
    val project: WorkflowProjectIdentity,
    val generation: Long,
    val expectedScan: GoScanReport? = null,
    val requiresExpectedScan: Boolean = false,
)

private data class PerformanceJobIdentity(
    val project: WorkflowProjectIdentity,
    val id: String,
    val generation: String,
    val queueId: String,
)

private data class PerformanceReportPublication(
    val job: PerformanceJobIdentity?,
    val generation: Long,
)

/**
 * Owns daemon interaction and every workflow coroutine for the desktop app. Its state flow contains
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

  private var connectionJob: Job? = null
  private var projectJob: Job? = null
  private var fileJob: Job? = null
  private var enrichmentJobs: List<Job> = emptyList()
  private var analysisJob: Job? = null
  private var declarationExplanationJob: Job? = null
  private var chatJob: Job? = null
  private var draftValidationJob: Job? = null
  private var draftChecksJob: Job? = null
  private var analysisGeneration = 0L
  private var declarationExplanationGeneration = 0L
  private var analyzeAllActionGeneration = 0L
  private var performanceActionGeneration = 0L
  private var performanceReportPublicationGeneration = 0L
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
    if (event is DesktopEvent.ProjectLoaded) {
      invalidateJobActions()
      jobCoordinator.projectOpened(event.project.identity())
    }
    controller.dispatch(event)
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
                previous.copy(modelCatalog = catalog, providerConfirmations = confirmations)
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
    cancelProjectScopedWork()
    val request = controller.beginProjectLoad()
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
            if (!controller.projectLoaded(request, project, index)) return@launch
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
          } catch (_: CancellationException) {
            throw CancellationException()
          } catch (error: Exception) {
            if (!controller.isCurrentProjectRequest(request)) return@launch
            if (restore)
                dispatch(DesktopEvent.Failed(error.message ?: "Could not reopen the last project"))
            else modelRequestFailed(error, ModelScope.Analyze, "Import failed")
          }
        }
  }

  fun reanalyze() {
    val project = snapshot.value.state.project ?: return
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
            if (matchesProject(project.identity()))
                dispatch(DesktopEvent.Failed(error.message ?: "Re-analysis failed"))
          }
        }
  }

  fun selectFile(
      path: String,
      editorTarget: EditorNavigationTarget? = null,
      preparedFixRequest: String? = null,
      preparedTaskSpec: BugTaskSpec? = null,
  ) {
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
    val request = controller.beginFileLoad(path) ?: return
    publish()
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
            if (controller.fileFailed(request, error.message ?: "File load failed")) publish()
          }
        }
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

  fun analyzeSelected(refresh: Boolean) {
    val state = snapshot.value.state
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    if (snapshot.value.model(ModelScope.Bug).remoteProvider &&
        !snapshot.value.providerConfirmed(ModelScope.Bug)) {
      dispatch(
          DesktopEvent.Failed("Confirm the Bugs model destination before analyzing this file."))
      return
    }
    analysisJob?.cancel()
    val generation = ++analysisGeneration
    val (request, fileRequest) = controller.beginAnalysis() ?: return
    dispatch(DesktopEvent.Loading)
    dispatch(DesktopEvent.Status("${if (refresh) "Refreshing" else "Analyzing"} ${file.path}…"))
    setOperation(analysis = true)
    analysisJob =
        scope.launch {
          try {
            val result = io {
              api.analyze(
                  file.path,
                  project.projectRevision,
                  refresh,
                  snapshot.value.providerConfirmed(ModelScope.Bug))
            }
            if (generation == analysisGeneration &&
                controller.analysisCompleted(request, fileRequest, result)) {
              publish()
              dispatch(DesktopEvent.Status("Summary ${result.status}"))
            }
          } catch (_: CancellationException) {
            dispatch(DesktopEvent.Status("Analysis canceled"))
            throw CancellationException()
          } catch (error: Exception) {
            if (generation == analysisGeneration && isCurrentFile(file.identity(project)))
                modelRequestFailed(error, ModelScope.Bug, "Analysis failed")
          } finally {
            if (analysisJob === coroutineContext[Job]) setOperation(analysis = false)
          }
        }
  }

  fun cancelAnalysis() {
    analysisGeneration++
    analysisJob?.cancel()
    setOperation(analysis = false)
  }

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
                    session.copy(repairCount = session.repairCount + if (repair) 1 else 0),
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
            if (activeTask == identity && controller.cancelChatLoad(request, fileRequest)) publish()
            if (activeTask == identity)
                modelRequestFailed(error, ModelScope.Function, "Chat request failed")
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
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    if (!draftEditorMatchesOpenFile(editor, file, project)) {
      dispatch(DesktopEvent.DraftMarkedStale)
      dispatch(DesktopEvent.Failed("The draft no longer matches the open file."))
      return
    }
    val (request, fileRequest) = controller.beginDraftLoad() ?: return
    val identity = editor.serverDraft.identity(file.identity(project))
    activeDraft = identity
    draftValidationJob?.cancel()
    dispatch(DesktopEvent.DraftValidationStarted)
    dispatch(DesktopEvent.Status("Validating ${editor.serverDraft.targetSymbol}…"))
    setOperation(validating = true)
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
            val validated = io {
              api.validateDraft(updated.id, project.projectRevision, updated.revision)
            }
            if (activeDraft == identity &&
                controller.draftLoaded(request, fileRequest, validated)) {
              activeDraft = validated.identity(identity.file)
              publish()
              dispatch(
                  DesktopEvent.Status(
                      if (validated.validation?.applicable == true) "Draft validation passed."
                      else "Draft validation needs attention."))
            }
          } catch (_: CancellationException) {
            dispatch(DesktopEvent.Status("Draft validation canceled"))
            throw CancellationException()
          } catch (error: ApiException) {
            if (activeDraft != identity) return@launch
            if (error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
            dispatch(DesktopEvent.Failed(error.message ?: "Draft validation failed"))
          } catch (error: Exception) {
            if (activeDraft != identity) return@launch
            dispatch(DesktopEvent.Failed(error.message ?: "Draft validation failed"))
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
    val (request, fileRequest) = controller.beginDraftLoad() ?: return
    val identity = draft.identity(file.identity(project))
    activeDraft = identity
    draftChecksJob?.cancel()
    dispatch(DesktopEvent.Loading)
    val taskTestName = draft.taskSpec?.goTestCandidate?.name
    dispatch(
        DesktopEvent.Status(
            if (taskTestName == null)
                "Running source-only focused checks for ${draft.targetSymbol}…"
            else
                "Trusting local execution for ${draft.targetSymbol}, then running focused checks…"))
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
            }
          } catch (_: CancellationException) {
            throw CancellationException()
          } catch (error: ApiException) {
            if (activeDraft != identity) return@launch
            if (error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
            dispatch(DesktopEvent.Failed(error.message ?: "Focused checks failed"))
          } catch (error: Exception) {
            if (activeDraft != identity) return@launch
            dispatch(DesktopEvent.Failed(error.message ?: "Focused checks failed"))
          }
        }
  }

  fun applyEditableDraft() {
    val state = snapshot.value.state
    val draft = state.review.draft ?: return
    val project = state.project ?: return
    val file = state.selectedFile ?: return
    val identity = draft.identity(file.identity(project))
    val eligibility =
        draftReviewEligibility(state.review.editor, draft, state.checks, file, project)
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

  fun startAnalyzeAll(options: AnalyzeAllRunOptions) {
    val request = beginAnalyzeAllAction() ?: return
    runAnalyzeAllAction(request, "Unable to start Analyze-all") {
      api.startAnalyzeAll(
          request.project.revision,
          options.maxFiles,
          options.maxRetries,
          options.confirmRemoteProvider)
    }
  }

  fun pauseAnalyzeAll() {
    val expectedJob = snapshot.value.state.findings.analyzeAll ?: return
    val request = beginAnalyzeAllAction(expectedJob) ?: return
    runAnalyzeAllAction(request, "Unable to update Analyze-all") {
      api.pauseAnalyzeAll(request.project.revision)
    }
  }

  fun resumeAnalyzeAll(confirmRemoteProvider: Boolean) {
    val expectedJob = snapshot.value.state.findings.analyzeAll ?: return
    val request = beginAnalyzeAllAction(expectedJob) ?: return
    runAnalyzeAllAction(request, "Unable to update Analyze-all") {
      api.resumeAnalyzeAll(request.project.revision, confirmRemoteProvider)
    }
  }

  fun cancelAnalyzeAll() {
    val expectedJob = snapshot.value.state.findings.analyzeAll ?: return
    val request = beginAnalyzeAllAction(expectedJob) ?: return
    runAnalyzeAllAction(request, "Unable to update Analyze-all") {
      api.cancelAnalyzeAll(request.project.revision)
    }
  }

  fun previewPerformance(maxFiles: Int = 100, runBudgetSeconds: Int = 900) {
    val project = snapshot.value.state.project ?: return
    val identity = project.identity()
    scope.launch {
      try {
        val preview = io { api.performanceContext(identity.revision, maxFiles, runBudgetSeconds) }
        if (matchesProject(identity)) dispatch(DesktopEvent.PerformanceContextLoaded(preview))
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (matchesProject(identity))
            dispatch(DesktopEvent.Failed(error.message ?: "Performance preview failed"))
      }
    }
  }

  fun startPerformance(preview: PerformanceQueuePreview, confirmRemoteProvider: Boolean) {
    val request = beginPerformanceAction() ?: return
    runPerformanceAction(request, "Unable to start Performance review") {
      api.startPerformanceJob(
          request.project.revision,
          preview.maxFiles,
          queueId = preview.queueId,
          policyFingerprint = preview.policyFingerprint,
          confirmRemoteProvider = confirmRemoteProvider)
    }
  }

  fun pausePerformance() {
    val expectedJob = snapshot.value.state.findings.performanceJob ?: return
    val request = beginPerformanceAction(expectedJob) ?: return
    runPerformanceAction(request, "Unable to pause Performance review") {
      api.pausePerformanceJob(request.project.revision, expectedJob.id)
    }
  }

  fun resumePerformance(confirmRemoteProvider: Boolean) {
    val expectedJob = snapshot.value.state.findings.performanceJob ?: return
    val request = beginPerformanceAction(expectedJob) ?: return
    runPerformanceAction(request, "Unable to resume Performance review") {
      api.resumePerformanceJob(request.project.revision, expectedJob.id, confirmRemoteProvider)
    }
  }

  fun cancelPerformance() {
    val expectedJob = snapshot.value.state.findings.performanceJob ?: return
    val request = beginPerformanceAction(expectedJob) ?: return
    runPerformanceAction(request, "Unable to cancel Performance review") {
      api.cancelPerformanceJob(request.project.revision, expectedJob.id)
    }
  }

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
    cancelAnalysis()
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
    fileJob?.cancel()
    enrichmentJobs.forEach(Job::cancel)
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
    val workspaceAnalyzeAllGeneration = analyzeAllActionGeneration
    val workspacePerformanceGeneration = performanceActionGeneration
    val workspacePerformanceReportGeneration = performanceReportPublicationGeneration
    val workspaceScanGeneration = verifiedScanActionGeneration
    scope.launch {
      try {
        val details = io {
          WorkflowProjectWorkspaceDetails(
              api.overview(identity.revision),
              api.findings(identity.revision),
              api.analyzeAllJob(identity.revision),
              api.goScan(identity.revision),
              api.performanceJob(identity.revision),
              api.performanceReport(identity.revision))
        }
        if (!matchesProject(identity)) return@launch
        dispatch(DesktopEvent.OverviewLoaded(details.overview))
        if (isCurrentVerifiedScanAction(identity, workspaceScanGeneration))
            dispatch(DesktopEvent.FindingsLoaded(details.findings.findings))
        publishAnalyzeAll(identity, details.analyzeAll, workspaceAnalyzeAllGeneration)
        publishPerformance(
            identity,
            details.performanceJob,
            details.performanceReport,
            workspacePerformanceGeneration,
            workspacePerformanceReportGeneration)
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

  private fun refreshAnalysisFreshness(identity: WorkflowProjectIdentity) {
    scope.launch {
      try {
        val index = io { api.index() }
        if (matchesProject(identity) && index.projectRevision == identity.revision)
            dispatch(DesktopEvent.IndexRefreshed(index))
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (_: Exception) {
        // Cache freshness is optional enrichment.
      }
    }
  }

  private fun runAnalyzeAllAction(
      request: AnalyzeAllActionRequest,
      fallback: String,
      action: () -> AnalyzeAllJob,
  ) {
    scope.launch {
      try {
        val job = io { if (canInvokeAnalyzeAllAction(request)) action() else null } ?: return@launch
        if (isCurrentAnalyzeAllAction(request.project, request.generation))
            publishAnalyzeAll(request.project, job, request.generation)
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (isCurrentAnalyzeAllAction(request.project, request.generation)) {
          modelRequestFailed(error, ModelScope.Bug, fallback)
          recoverAnalyzeAllPolling(request.project, request.generation)
        }
      }
    }
  }

  private fun publishAnalyzeAll(
      identity: WorkflowProjectIdentity,
      job: AnalyzeAllJob?,
      actionGeneration: Long = analyzeAllActionGeneration,
  ) {
    if (!isCurrentAnalyzeAllAction(identity, actionGeneration)) return
    jobCoordinator.observeAnalyzeAll(
        identity,
        job,
        onUpdate = { updated ->
          if (!isCurrentAnalyzeAllAction(identity, actionGeneration)) return@observeAnalyzeAll false
          if (updated?.projectRevision?.takeIf { it.isNotBlank() } != null &&
              updated.projectRevision != identity.revision)
              return@observeAnalyzeAll false
          dispatch(DesktopEvent.AnalyzeAllLoaded(updated))
          refreshAnalysisFreshness(identity)
          true
        },
        fetch = { io { api.analyzeAllJob(identity.revision) } },
        onFailure = { error ->
          if (isCurrentAnalyzeAllAction(identity, actionGeneration))
              dispatch(DesktopEvent.Failed(error.message ?: "Analyze-all status failed"))
        })
  }

  private fun runPerformanceAction(
      request: PerformanceActionRequest,
      fallback: String,
      action: () -> PerformanceJob,
  ) {
    scope.launch {
      try {
        val job =
            io { if (canInvokePerformanceAction(request)) action() else null } ?: return@launch
        if (!isCurrentPerformanceAction(request.project, request.generation)) return@launch
        val publication =
            publishPerformance(request.project, job, null, request.generation) ?: return@launch
        try {
          val report = io { api.performanceReport(request.project.revision) }
          val currentJob = snapshot.value.state.findings.performanceJob
          if (isCurrentPerformanceAction(request.project, request.generation) &&
              currentJob?.identity() == publication.job &&
              performanceReportPublicationGeneration == publication.generation) {
            performanceReportPublicationGeneration++
            dispatch(DesktopEvent.PerformanceLoaded(currentJob, report))
          }
        } catch (_: CancellationException) {
          throw CancellationException()
        } catch (error: Exception) {
          if (isCurrentPerformanceAction(request.project, request.generation))
              dispatch(DesktopEvent.Failed(error.message ?: "Performance report failed"))
        }
      } catch (_: CancellationException) {
        throw CancellationException()
      } catch (error: Exception) {
        if (isCurrentPerformanceAction(request.project, request.generation)) {
          modelRequestFailed(error, ModelScope.Analyze, fallback)
          recoverPerformancePolling(request.project, request.generation)
        }
      }
    }
  }

  private fun publishPerformance(
      identity: WorkflowProjectIdentity,
      job: PerformanceJob?,
      report: PerformanceReport?,
      actionGeneration: Long = performanceActionGeneration,
      expectedReportPublicationGeneration: Long? = null,
  ): PerformanceReportPublication? {
    if (!isCurrentPerformanceAction(identity, actionGeneration)) return null
    if (expectedReportPublicationGeneration != null &&
        expectedReportPublicationGeneration != performanceReportPublicationGeneration)
        return null
    var publication: PerformanceReportPublication? = null
    jobCoordinator.observePerformance(
        identity,
        job,
        report,
        onUpdate = { updatedJob, updatedReport ->
          if (!isCurrentPerformanceAction(identity, actionGeneration))
              return@observePerformance false
          if (updatedJob != null && !updatedJob.belongsTo(identity)) return@observePerformance false
          performanceReportPublicationGeneration++
          publication =
              PerformanceReportPublication(
                  updatedJob?.identity(), performanceReportPublicationGeneration)
          dispatch(DesktopEvent.PerformanceLoaded(updatedJob, updatedReport))
          true
        },
        fetch = {
          io { api.performanceJob(identity.revision) to api.performanceReport(identity.revision) }
        },
        onFailure = { error ->
          if (isCurrentPerformanceAction(identity, actionGeneration))
              dispatch(DesktopEvent.Failed(error.message ?: "Performance status failed"))
        })
    return publication
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
    fileJob?.cancel()
    enrichmentJobs.forEach(Job::cancel)
    cancelAll()
  }

  private fun beginAnalyzeAllAction(
      expectedJob: AnalyzeAllJob? = null,
  ): AnalyzeAllActionRequest? {
    val identity = snapshot.value.state.project?.identity() ?: return null
    jobCoordinator.beginAnalyzeAllAction(identity)
    return AnalyzeAllActionRequest(
        identity,
        ++analyzeAllActionGeneration,
        expectedJob,
        requiresExpectedJob = expectedJob != null)
  }

  private fun beginPerformanceAction(
      expectedJob: PerformanceJob? = null,
  ): PerformanceActionRequest? {
    val identity = snapshot.value.state.project?.identity() ?: return null
    jobCoordinator.beginPerformanceAction(identity)
    return PerformanceActionRequest(
        identity,
        ++performanceActionGeneration,
        expectedJob,
        requiresExpectedJob = expectedJob != null)
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

  private fun recoverAnalyzeAllPolling(identity: WorkflowProjectIdentity, generation: Long) {
    if (!isCurrentAnalyzeAllAction(identity, generation)) return
    val job = snapshot.value.state.findings.analyzeAll ?: return
    if (!job.belongsTo(identity)) return
    publishAnalyzeAll(identity, job, generation)
  }

  private fun recoverPerformancePolling(identity: WorkflowProjectIdentity, generation: Long) {
    if (!isCurrentPerformanceAction(identity, generation)) return
    val findings = snapshot.value.state.findings
    val job = findings.performanceJob ?: return
    if (!job.belongsTo(identity)) return
    publishPerformance(identity, job, findings.performanceReport, generation)
  }

  private fun recoverVerifiedScanPolling(identity: WorkflowProjectIdentity, generation: Long) {
    if (!isCurrentVerifiedScanAction(identity, generation)) return
    val scan = snapshot.value.state.findings.scan ?: return
    if (!scan.belongsTo(identity)) return
    publishVerifiedScan(identity, scan, generation)
  }

  private fun invalidateJobActions() {
    analyzeAllActionGeneration++
    performanceActionGeneration++
    verifiedScanActionGeneration++
  }

  private fun isCurrentVerifiedScanAction(
      identity: WorkflowProjectIdentity,
      generation: Long,
  ): Boolean = generation == verifiedScanActionGeneration && matchesProject(identity)

  private fun isCurrentAnalyzeAllAction(
      identity: WorkflowProjectIdentity,
      generation: Long,
  ): Boolean = generation == analyzeAllActionGeneration && matchesProject(identity)

  private fun isCurrentPerformanceAction(
      identity: WorkflowProjectIdentity,
      generation: Long,
  ): Boolean = generation == performanceActionGeneration && matchesProject(identity)

  private fun canInvokeAnalyzeAllAction(request: AnalyzeAllActionRequest): Boolean =
      isCurrentAnalyzeAllAction(request.project, request.generation) &&
          (!request.requiresExpectedJob ||
              snapshot.value.state.findings.analyzeAll == request.expectedJob)

  private fun canInvokePerformanceAction(request: PerformanceActionRequest): Boolean =
      isCurrentPerformanceAction(request.project, request.generation) &&
          (!request.requiresExpectedJob ||
              snapshot.value.state.findings.performanceJob == request.expectedJob)

  private fun canInvokeVerifiedScanAction(request: VerifiedScanActionRequest): Boolean =
      isCurrentVerifiedScanAction(request.project, request.generation) &&
          (!request.requiresExpectedScan ||
              snapshot.value.state.findings.scan == request.expectedScan)

  private fun modelRequestFailed(error: Exception, scope: ModelScope, fallback: String) {
    val staleConfirmation = staleRemoteConfirmationMessage(error, scope)
    if (staleConfirmation != null) setProviderConfirmation(scope, false)
    dispatch(DesktopEvent.Failed(staleConfirmation ?: error.message ?: fallback))
  }

  private fun publish() {
    mutableSnapshot.value = mutableSnapshot.value.copy(state = controller.state)
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

  private fun setOperation(
      analysis: Boolean? = null,
      generating: Boolean? = null,
      validating: Boolean? = null
  ) {
    val current = mutableSnapshot.value
    mutableSnapshot.value =
        current.copy(
            analysisInProgress = analysis ?: current.analysisInProgress,
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

private fun AnalyzeAllJob.belongsTo(identity: WorkflowProjectIdentity): Boolean =
    projectId == identity.id && projectRevision == identity.revision

private fun PerformanceJob.belongsTo(identity: WorkflowProjectIdentity): Boolean =
    projectId == identity.id && projectRevision == identity.revision

private fun PerformanceJob.identity(): PerformanceJobIdentity =
    PerformanceJobIdentity(
        WorkflowProjectIdentity(projectId, projectRevision), id, generation, queueId)

private fun GoScanReport.belongsTo(identity: WorkflowProjectIdentity): Boolean =
    projectId == identity.id && projectRevision == identity.revision

private fun DeclarationDraft.identity(file: WorkflowFileIdentity): WorkflowDraftIdentity =
    WorkflowDraftIdentity(file, id, revision, hash)

private data class WorkflowProjectWorkspaceDetails(
    val overview: ProjectOverview,
    val findings: FindingsResponse,
    val analyzeAll: AnalyzeAllJob?,
    val scan: GoScanReport?,
    val performanceJob: PerformanceJob?,
    val performanceReport: PerformanceReport?,
)
