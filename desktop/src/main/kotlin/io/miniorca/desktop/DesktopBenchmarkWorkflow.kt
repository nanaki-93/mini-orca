package io.miniorca.desktop

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext

internal sealed interface BenchmarkCandidateDecision {
  data class Ready(
      val draft: DeclarationDraft,
      val project: ProjectAnalysis,
      val identity: WorkflowDraftIdentity,
  ) : BenchmarkCandidateDecision

  data class Blocked(val reason: String) : BenchmarkCandidateDecision
}

internal data class BenchmarkEligibility(
    val candidate: BenchmarkCandidateDecision,
    val discoveryBlockedReason: String?,
    val selectionBlockedReason: String?,
    val comparisonBlockedReason: String?,
) {
  val canDiscover: Boolean
    get() = discoveryBlockedReason == null

  val canCompare: Boolean
    get() = comparisonBlockedReason == null
}

/** The same read-only decision authorizes explicit actions and explains disabled controls. */
internal fun benchmarkEligibility(
    state: DesktopState,
    choice: GoBenchmarkChoice? = state.review.benchmark.selected,
): BenchmarkEligibility {
  val candidate = benchmarkCandidateDecision(state)
  val discoveryReason =
      (candidate as? BenchmarkCandidateDecision.Blocked)?.reason
          ?: if (state.review.benchmark.discovery == BenchmarkDiscoveryOutcome.Loading)
              "Compatible benchmark lookup is already in progress."
          else null
  val selectionReason =
      when (candidate) {
        is BenchmarkCandidateDecision.Blocked -> candidate.reason
        is BenchmarkCandidateDecision.Ready ->
            benchmarkSelectionBlockedReason(state.review.benchmark, candidate.identity, choice)
      }
  val comparisonReason =
      selectionReason
          ?: when {
            choice!!.name.isBlank() -> "The selected benchmark has no name; refresh the catalog."
            choice.scope.isBlank() ->
                "The selected benchmark has no scope guard; refresh the catalog."
            choice.command.isEmpty() -> "The selected benchmark has no argv; refresh the catalog."
            else -> null
          }
  return BenchmarkEligibility(candidate, discoveryReason, selectionReason, comparisonReason)
}

private fun benchmarkCandidateDecision(state: DesktopState): BenchmarkCandidateDecision {
  val project =
      state.project
          ?: return BenchmarkCandidateDecision.Blocked("Open a project before listing benchmarks.")
  val file =
      state.selectedFile
          ?: return BenchmarkCandidateDecision.Blocked(
              "Select the candidate file before listing benchmarks.")
  val draft =
      state.review.draft
          ?: return BenchmarkCandidateDecision.Blocked(
              "Prepare and validate a draft before listing benchmarks.")
  val editor = state.review.editor
  val reason =
      when {
        editor?.status != DraftEditorStatus.Valid || draft.validation?.applicable != true ->
            "Validate the exact current draft before listing or comparing benchmarks."
        goBenchmarkComparisonIdentity(draft) == null ->
            "The candidate identity is incomplete; refresh and validate the draft."
        editor.serverDraft != draft ||
            editor.declaration != draft.declaration ||
            editor.imports != draft.imports ->
            "The draft and editor candidate differ; validate the current draft again."
        !draftEditorMatchesOpenFile(editor, file, project) ->
            "The candidate does not match the current project, revision, file path or base hash; refresh and validate the draft."
        else -> null
      }
  if (reason != null) return BenchmarkCandidateDecision.Blocked(reason)
  return BenchmarkCandidateDecision.Ready(
      draft,
      project,
      WorkflowDraftIdentity(
          WorkflowFileIdentity(
              WorkflowProjectIdentity(project.projectId, project.projectRevision),
              file.path,
              file.contentHash),
          draft.id,
          draft.revision,
          draft.hash))
}

private fun benchmarkSelectionBlockedReason(
    evidence: BenchmarkEvidenceState,
    identity: WorkflowDraftIdentity,
    choice: GoBenchmarkChoice?,
): String? =
    when {
      evidence.discovery != BenchmarkDiscoveryOutcome.Loaded || evidence.catalog == null ->
          "List compatible benchmarks for the current candidate before selecting or comparing."
      !evidence.catalog.available ->
          evidence.catalog.reason.ifBlank {
            "No compatible benchmark is available; refresh the catalog."
          }
      !evidence.catalog.matches(identity) ->
          "Benchmark catalog is stale; list compatible benchmarks again."
      choice == null -> "Select one listed benchmark before comparing."
      choice !in evidence.catalog.benchmarks ->
          "Benchmark selection is stale; list compatible benchmarks and select an exact returned choice."
      else -> null
    }

private data class BenchmarkActionRequest(
    val project: WorkflowProjectIdentity,
    val draft: WorkflowDraftIdentity,
    val choice: GoBenchmarkChoice,
    val catalog: GoBenchmarkCatalog,
    val generation: Long,
)

/**
 * Owns benchmark request lifetimes while reading and publishing through the desktop state store.
 */
internal class DesktopBenchmarkWorkflow(
    private val api: ApiClient,
    private val scope: CoroutineScope,
    private val ioDispatcher: CoroutineDispatcher,
    private val state: () -> DesktopState,
    private val dispatch: (DesktopEvent) -> Unit,
) {
  private var benchmarkCatalogJob: Job? = null
  private var benchmarkJob: Job? = null
  private var benchmarkActionGeneration = 0L

  fun beforeEvent(event: DesktopEvent) {
    val current = state()
    // Use the reducer's acceptance rules: ignored refreshes and obsolete validation events
    // must not cancel work for an unchanged candidate.
    val lifecycleChanged =
        when (event) {
          is DesktopEvent.IndexRefreshed,
          is DesktopEvent.ProjectIndexingCompleted ->
              current.reduce(event).project?.projectRevision != current.project?.projectRevision
          is DesktopEvent.SelectedFileRefreshed ->
              current.reduce(event).selectedFile != current.selectedFile
          is DesktopEvent.DraftValidationStarted,
          is DesktopEvent.DraftValidationUpdated,
          is DesktopEvent.DraftValidationStopped ->
              current.reduce(event).review.editor != current.review.editor
          else -> false
        }
    if (event is DesktopEvent.GoBenchmarkCatalogLoaded) {
      benchmarkCatalogJob?.cancel()
      benchmarkCatalogJob = null
    }
    if (event is DesktopEvent.GoBenchmarkCatalogLoaded ||
        event is DesktopEvent.GoBenchmarkSelected &&
            event.choice != current.review.benchmark.selected) {
      cancelPendingComparison()
    }
    if (event is DesktopEvent.DraftEdited ||
        event is DesktopEvent.ChatProposalLoaded ||
        event is DesktopEvent.DraftLoaded ||
        event == DesktopEvent.DraftMarkedStale ||
        event == DesktopEvent.DraftDiscarded ||
        event is DesktopEvent.FileLoaded ||
        event is DesktopEvent.ProjectLoaded ||
        event is DesktopEvent.SelectedFileUnavailable ||
        event is DesktopEvent.Applied ||
        lifecycleChanged) {
      invalidate()
    }
  }

  fun invalidate() {
    benchmarkActionGeneration++
    benchmarkCatalogJob?.cancel()
    benchmarkCatalogJob = null
    benchmarkJob?.cancel()
    benchmarkJob = null
    dispatch(DesktopEvent.GoBenchmarkDiscoveryInvalidated)
  }

  private fun cancelPendingComparison() {
    benchmarkActionGeneration++
    benchmarkJob?.cancel()
    benchmarkJob = null
  }

  fun stopComparison() {
    invalidate()
    dispatch(DesktopEvent.GoBenchmarkComparisonStopped)
  }

  fun cancel() = invalidate()

  /** Lists trusted daemon-built benchmark choices. This GET never executes project code. */
  fun loadGoBenchmarks() {
    if (state().review.benchmark.discovery == BenchmarkDiscoveryOutcome.Loading) return
    val decision = benchmarkEligibility(state())
    val candidate = decision.candidate as? BenchmarkCandidateDecision.Ready
    if (candidate == null) {
      dispatch(DesktopEvent.GoBenchmarkDiscoveryFailed(decision.discoveryBlockedReason!!))
      return
    }
    val (draft, project, identity) = candidate
    invalidate()
    val generation = benchmarkActionGeneration
    dispatch(DesktopEvent.GoBenchmarkDiscoveryStarted)
    benchmarkCatalogJob =
        scope.launch {
          try {
            val catalog = io {
              api.goBenchmarkCatalog(draft.id, project.projectRevision, draft.revision, draft.hash)
            }
            if (!isCurrentBenchmarkCatalogAction(identity, generation)) return@launch
            if (catalog.available && !catalog.matches(identity)) {
              dispatch(
                  DesktopEvent.GoBenchmarkDiscoveryFailed(
                      "Benchmark catalog does not match the current candidate; refresh compatible benchmarks."))
              return@launch
            }
            // The lookup is complete; catalog replacement must only cancel other pending work.
            benchmarkCatalogJob = null
            // Sparse unavailable responses carry a reason, not reusable executable authority.
            dispatch(
                DesktopEvent.GoBenchmarkCatalogLoaded(
                    if (catalog.available) catalog else catalog.copy(benchmarks = emptyList())))
            dispatch(
                DesktopEvent.Status(
                    when {
                      !catalog.available ->
                          catalog.reason.ifBlank { "No compatible benchmark is available." }
                      catalog.benchmarks.isEmpty() -> "No compatible benchmark is available."
                      else -> "Select a benchmark to compare."
                    }))
          } catch (error: CancellationException) {
            if (isCurrentBenchmarkCatalogAction(identity, generation))
                dispatch(DesktopEvent.GoBenchmarkDiscoveryInvalidated)
            throw error
          } catch (error: ApiException) {
            if (!isCurrentBenchmarkCatalogAction(identity, generation)) return@launch
            val message = error.message ?: "Benchmark lookup failed"
            if (error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
            dispatch(DesktopEvent.GoBenchmarkDiscoveryFailed(message))
          } catch (error: Exception) {
            if (isCurrentBenchmarkCatalogAction(identity, generation)) {
              val message = error.message ?: "Benchmark lookup failed"
              dispatch(DesktopEvent.GoBenchmarkDiscoveryFailed(message))
            }
          }
        }
  }

  fun selectGoBenchmark(choice: GoBenchmarkChoice) {
    val current = state()
    val decision = benchmarkEligibility(current, choice)
    decision.selectionBlockedReason?.let {
      dispatch(DesktopEvent.GoBenchmarkComparisonFailed(it))
      return
    }
    if (current.review.benchmark.selected == choice) return
    cancelPendingComparison()
    dispatch(DesktopEvent.GoBenchmarkSelected(choice))
  }

  /**
   * The explicit action may trust local execution only after the displayed fixed argv is selected.
   */
  fun compareSelectedGoBenchmark() {
    val current = state()
    if (current.review.benchmark.running) return
    val decision = benchmarkEligibility(current)
    decision.comparisonBlockedReason?.let {
      dispatch(DesktopEvent.GoBenchmarkComparisonFailed(it))
      return
    }
    val candidate = decision.candidate as BenchmarkCandidateDecision.Ready
    val (draft, project, identity) = candidate
    val catalog = current.review.benchmark.catalog!!
    val choice = current.review.benchmark.selected!!
    val request =
        BenchmarkActionRequest(
            WorkflowProjectIdentity(project.projectId, project.projectRevision),
            identity,
            choice,
            catalog,
            ++benchmarkActionGeneration,
        )
    benchmarkJob?.cancel()
    dispatch(
        if (catalog.trusted) DesktopEvent.GoBenchmarkComparisonStarted
        else DesktopEvent.GoBenchmarkAdmissionStarted)
    dispatch(
        DesktopEvent.Status(
            if (catalog.trusted) "Comparing ${choice.name} in isolated copies…"
            else "Admitting local execution for ${choice.name}…"))
    benchmarkJob =
        scope.launch {
          try {
            if (!catalog.trusted && !admitBenchmarkExecution(request)) return@launch
            if (!isCurrentBenchmarkAction(request)) return@launch
            if (!catalog.trusted) {
              dispatch(DesktopEvent.GoBenchmarkComparisonStarted)
              dispatch(DesktopEvent.Status("Comparing ${choice.name} in isolated copies…"))
            }
            val comparison =
                currentBenchmarkRequest(request) {
                  api.compareGoBenchmark(
                      draft.id, project.projectRevision, draft.revision, draft.hash, choice)
                } ?: return@launch
            if (!isCurrentBenchmarkAction(request)) return@launch
            val responseMismatch = benchmarkResponseMismatch(comparison, draft, choice)
            // Sparse unavailability explains this current request without establishing evidence
            // identity. Responses carrying measurements still require the normal association
            // checks.
            val sparseUnavailable =
                comparison.status == "unavailable" &&
                    comparison.base == null &&
                    comparison.candidate == null
            if (responseMismatch != null && !sparseUnavailable) {
              failBenchmarkAdmission(responseMismatch, rediscover = true)
            } else {
              publishBenchmarkOutcome(comparison)
            }
          } catch (error: CancellationException) {
            if (isCurrentBenchmarkAction(request))
                dispatch(DesktopEvent.GoBenchmarkComparisonStopped)
            throw error
          } catch (error: ApiException) {
            if (!isCurrentBenchmarkAction(request)) return@launch
            if (error.status == 409) {
              dispatch(DesktopEvent.DraftMarkedStale)
              failBenchmarkAdmission(error.message ?: "Candidate changed; refresh and validate it.")
            } else {
              failBenchmarkAdmission(benchmarkTransportFailure(error))
            }
          } catch (error: Exception) {
            if (isCurrentBenchmarkAction(request))
                failBenchmarkAdmission(benchmarkTransportFailure(error))
          }
        }
  }

  private fun publishBenchmarkOutcome(comparison: GoBenchmarkComparison) {
    val outcome = BenchmarkComparisonOutcome(comparison)
    if (outcome.status == BenchmarkComparisonStatus.Unavailable)
        dispatch(DesktopEvent.GoBenchmarkDiscoveryInvalidated)
    dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
    val message =
        when (outcome.status) {
          BenchmarkComparisonStatus.Completed -> "Benchmark evidence is ready for review."
          BenchmarkComparisonStatus.Canceled ->
              comparison.reason.ifBlank { "The daemon canceled the benchmark comparison." }
          BenchmarkComparisonStatus.Failed ->
              comparison.reason.ifBlank {
                "The daemon failed to complete the benchmark comparison."
              }
          BenchmarkComparisonStatus.Unavailable ->
              "${comparison.reason.ifBlank { "Benchmark comparison is unavailable." }} Refresh compatible benchmarks and select again."
          BenchmarkComparisonStatus.Unsupported ->
              "Unsupported benchmark status ${comparison.status.ifBlank { "(not recorded)" }}; no successful comparison is confirmed." +
                  comparison.reason.takeIf { it.isNotBlank() }?.let { " $it" }.orEmpty()
        }
    dispatch(DesktopEvent.Status(message))
  }

  private suspend fun admitBenchmarkExecution(request: BenchmarkActionRequest): Boolean {
    val trust =
        currentBenchmarkRequest(request) { api.executionTrust(request.project.revision) }
            ?: return false
    if (!isCurrentBenchmarkAction(request)) return false
    benchmarkTrustMismatch(trust, request, acknowledgment = false)?.let {
      failBenchmarkAdmission(it, rediscover = true)
      return false
    }
    val acknowledgment =
        currentBenchmarkRequest(request) { api.trustProjectExecution(request.project.revision) }
            ?: return false
    if (!isCurrentBenchmarkAction(request)) return false
    benchmarkTrustMismatch(acknowledgment, request, acknowledgment = true)?.let {
      failBenchmarkAdmission(it, rediscover = true)
      return false
    }
    return true
  }

  private fun benchmarkTrustMismatch(
      trust: ExecutionTrust,
      request: BenchmarkActionRequest,
      acknowledgment: Boolean,
  ): String? =
      when {
        trust.projectId.isBlank() ||
            trust.projectRevision.isBlank() ||
            trust.projectId != request.project.id ||
            trust.projectRevision != request.project.revision ->
            "Local execution trust identity changed."
        trust.commands != listOf(listOf("go", "test", "./...")) ->
            "Local execution trust scope changed."
        acknowledgment && !trust.trusted -> "Local execution trust was not granted."
        else -> null
      }

  private fun failBenchmarkAdmission(message: String, rediscover: Boolean = false) {
    if (rediscover) dispatch(DesktopEvent.GoBenchmarkDiscoveryInvalidated)
    dispatch(
        DesktopEvent.GoBenchmarkComparisonFailed(
            if (rediscover) "$message Refresh compatible benchmarks and select again."
            else message))
  }

  private fun benchmarkTransportFailure(error: Exception): String {
    val message = error.message ?: "Request failed"
    return if (state().review.benchmark.admission == BenchmarkAdmissionOutcome.Running)
        "$message Execution may have started; no new measurements were confirmed."
    else "Local execution admission failed: $message"
  }

  // Guard both scheduling and actual transport entry: a queued I/O stage may become obsolete.
  private suspend fun <T> currentBenchmarkRequest(
      request: BenchmarkActionRequest,
      block: () -> T,
  ): T? {
    if (!isCurrentBenchmarkAction(request)) return null
    return io { if (isCurrentBenchmarkAction(request)) block() else null }
  }

  private fun isCurrentBenchmarkAction(request: BenchmarkActionRequest): Boolean {
    val current = state()
    val eligibility = benchmarkEligibility(current)
    return request.generation == benchmarkActionGeneration &&
        (eligibility.candidate as? BenchmarkCandidateDecision.Ready)?.identity == request.draft &&
        eligibility.canCompare &&
        current.review.benchmark.catalog == request.catalog &&
        current.review.benchmark.selected == request.choice
  }

  private fun isCurrentBenchmarkCatalogAction(
      identity: WorkflowDraftIdentity,
      generation: Long,
  ): Boolean =
      generation == benchmarkActionGeneration &&
          state().review.benchmark.discovery == BenchmarkDiscoveryOutcome.Loading &&
          currentBenchmarkDraftIdentity() == identity

  private fun benchmarkResponseMismatch(
      comparison: GoBenchmarkComparison,
      draft: DeclarationDraft,
      choice: GoBenchmarkChoice,
  ): String? =
      when {
        !comparison.matches(draft) ->
            "Benchmark response is stale because the reviewed candidate identity changed."
        comparison.benchmark != choice.name ->
            "Benchmark response is stale because the selected benchmark changed."
        comparison.scope != choice.scope ->
            "Benchmark response is unavailable because the displayed benchmark scope changed."
        else -> null
      }

  private fun currentBenchmarkDraftIdentity(): WorkflowDraftIdentity? =
      (benchmarkEligibility(state()).candidate as? BenchmarkCandidateDecision.Ready)?.identity

  private suspend fun <T> io(block: () -> T): T =
      withContext(ioDispatcher) { runInterruptible { block() } }
}

private fun GoBenchmarkCatalog.matches(identity: WorkflowDraftIdentity): Boolean =
    draftId == identity.id &&
        draftRevision == identity.revision &&
        draftHash == identity.hash &&
        projectId == identity.file.project.id &&
        projectRevision == identity.file.project.revision &&
        baseFileHash == identity.file.contentHash &&
        targetPath == identity.file.path

private fun GoBenchmarkComparison.matches(draft: DeclarationDraft): Boolean =
    draftId == draft.id &&
        draftRevision == draft.revision &&
        draftHash == draft.hash &&
        projectId == draft.projectId &&
        projectRevision == draft.projectRevision &&
        baseFileHash == draft.baseFileHash &&
        targetPath == draft.targetPath
