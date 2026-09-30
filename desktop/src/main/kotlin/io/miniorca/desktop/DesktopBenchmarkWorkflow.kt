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
    val selectionBlockedReason: String?,
    val comparisonBlockedReason: String?,
) {
  val discoveryBlockedReason: String?
    get() = (candidate as? BenchmarkCandidateDecision.Blocked)?.reason

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
  return BenchmarkEligibility(candidate, selectionReason, comparisonReason)
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
    val indexChanged =
        event is DesktopEvent.IndexRefreshed &&
            state().project?.projectRevision != event.index.projectRevision
    if (event is DesktopEvent.DraftEdited ||
        event is DesktopEvent.ChatProposalLoaded ||
        event is DesktopEvent.DraftLoaded ||
        event == DesktopEvent.DraftMarkedStale ||
        event == DesktopEvent.DraftDiscarded ||
        event is DesktopEvent.FileLoaded ||
        event is DesktopEvent.ProjectLoaded ||
        indexChanged) {
      invalidate()
    }
  }

  fun invalidate() {
    benchmarkActionGeneration++
    benchmarkCatalogJob?.cancel()
    benchmarkCatalogJob = null
    benchmarkJob?.cancel()
    benchmarkJob = null
  }

  fun stopComparison() {
    invalidate()
    dispatch(DesktopEvent.GoBenchmarkComparisonStopped)
  }

  fun cancel() {
    invalidate()
    if (state().review.benchmark.running) dispatch(DesktopEvent.GoBenchmarkComparisonStopped)
  }

  /** Lists trusted daemon-built benchmark choices. This GET never executes project code. */
  fun loadGoBenchmarks() {
    val decision = benchmarkEligibility(state())
    val candidate = decision.candidate as? BenchmarkCandidateDecision.Ready
    if (candidate == null) {
      dispatch(DesktopEvent.GoBenchmarkDiscoveryFailed(decision.discoveryBlockedReason!!))
      return
    }
    val (draft, project, identity) = candidate
    cancel()
    val generation = benchmarkActionGeneration
    benchmarkCatalogJob =
        scope.launch {
          try {
            val catalog = io {
              api.goBenchmarkCatalog(draft.id, project.projectRevision, draft.revision, draft.hash)
            }
            if (isCurrentBenchmarkCatalogAction(identity, generation) &&
                catalog.matches(identity)) {
              dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog))
              dispatch(
                  DesktopEvent.Status(
                      if (catalog.available) "Select a benchmark to compare."
                      else catalog.reason.ifBlank { "No compatible benchmark is available." }))
            }
          } catch (_: CancellationException) {
            throw CancellationException()
          } catch (error: ApiException) {
            if (!isCurrentBenchmarkCatalogAction(identity, generation)) return@launch
            val message = error.message ?: "Benchmark lookup failed"
            dispatch(DesktopEvent.GoBenchmarkDiscoveryFailed(message))
            if (error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
            dispatch(DesktopEvent.Failed(message))
          } catch (error: Exception) {
            if (isCurrentBenchmarkCatalogAction(identity, generation)) {
              val message = error.message ?: "Benchmark lookup failed"
              dispatch(DesktopEvent.GoBenchmarkDiscoveryFailed(message))
              dispatch(DesktopEvent.Failed(message))
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
    benchmarkActionGeneration++
    benchmarkJob?.cancel()
    dispatch(DesktopEvent.GoBenchmarkSelected(choice))
  }

  /**
   * The explicit action may trust local execution only after the displayed fixed argv is selected.
   */
  fun compareSelectedGoBenchmark() {
    val current = state()
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
            ++benchmarkActionGeneration,
        )
    benchmarkJob?.cancel()
    dispatch(DesktopEvent.GoBenchmarkComparisonStarted)
    dispatch(
        DesktopEvent.Status(
            if (catalog.trusted) "Comparing ${choice.name} in isolated copies…"
            else "Trusting local execution, then comparing ${choice.name} in isolated copies…"))
    benchmarkJob =
        scope.launch {
          try {
            val comparison = io {
              if (!catalog.trusted) {
                val trust = api.executionTrust(project.projectRevision)
                if (trust.projectId != project.projectId ||
                    trust.projectRevision != project.projectRevision ||
                    trust.commands != listOf(listOf("go", "test", "./...")))
                    throw IllegalStateException(
                        "Local execution trust scope changed; review the benchmark command again.")
                api.trustProjectExecution(project.projectRevision)
              }
              api.compareGoBenchmark(
                  draft.id, project.projectRevision, draft.revision, draft.hash, choice)
            }
            if (!isCurrentBenchmarkAction(request)) return@launch
            val responseMismatch = benchmarkResponseMismatch(comparison, draft, choice)
            if (responseMismatch != null) {
              dispatch(
                  DesktopEvent.GoBenchmarkComparisonLoaded(
                      comparison.copy(status = "unavailable", reason = responseMismatch)))
              dispatch(DesktopEvent.Status(responseMismatch))
              return@launch
            }
            dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
            dispatch(
                DesktopEvent.Status(
                    if (comparison.status == "completed") "Benchmark evidence is ready for review."
                    else
                        comparison.reason.ifBlank {
                          "Benchmark comparison did not produce measurements."
                        }))
          } catch (_: CancellationException) {
            if (isCurrentBenchmarkAction(request))
                dispatch(DesktopEvent.GoBenchmarkComparisonStopped)
            throw CancellationException()
          } catch (error: ApiException) {
            if (!isCurrentBenchmarkAction(request)) return@launch
            if (error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
            val message = error.message ?: "Benchmark comparison failed"
            dispatch(DesktopEvent.GoBenchmarkComparisonFailed(message))
            dispatch(DesktopEvent.Failed(message))
          } catch (error: Exception) {
            if (!isCurrentBenchmarkAction(request)) return@launch
            val message = error.message ?: "Benchmark comparison failed"
            dispatch(DesktopEvent.GoBenchmarkComparisonFailed(message))
            dispatch(DesktopEvent.Failed(message))
          }
        }
  }

  private fun isCurrentBenchmarkAction(request: BenchmarkActionRequest): Boolean =
      request.generation == benchmarkActionGeneration &&
          state().project?.let { WorkflowProjectIdentity(it.projectId, it.projectRevision) } ==
              request.project &&
          currentBenchmarkDraftIdentity() == request.draft &&
          benchmarkEligibility(state()).canCompare &&
          state().review.benchmark.catalog?.matches(request.draft) == true &&
          state().review.benchmark.selected == request.choice

  private fun isCurrentBenchmarkCatalogAction(
      identity: WorkflowDraftIdentity,
      generation: Long,
  ): Boolean =
      generation == benchmarkActionGeneration && currentBenchmarkDraftIdentity() == identity

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
