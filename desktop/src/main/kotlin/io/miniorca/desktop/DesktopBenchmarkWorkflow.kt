package io.miniorca.desktop

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext

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
    val (draft, project, file) = currentBenchmarkDraft() ?: return
    val identity = draftIdentity(draft, project, file)
    cancel()
    val generation = benchmarkActionGeneration
    benchmarkCatalogJob =
        scope.launch {
          try {
            val catalog = io {
              api.goBenchmarkCatalog(draft.id, project.projectRevision, draft.revision, draft.hash)
            }
            if (isCurrentBenchmarkCatalogAction(identity, generation) && catalog.matches(draft)) {
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
            if (error.status == 409) dispatch(DesktopEvent.DraftMarkedStale)
            dispatch(DesktopEvent.Failed(error.message ?: "Benchmark lookup failed"))
          } catch (error: Exception) {
            if (isCurrentBenchmarkCatalogAction(identity, generation))
                dispatch(DesktopEvent.Failed(error.message ?: "Benchmark lookup failed"))
          }
        }
  }

  fun selectGoBenchmark(choice: GoBenchmarkChoice) {
    val state = state()
    val catalog = state.review.benchmark.catalog ?: return
    val draft = state.review.draft ?: return
    if (!catalog.matches(draft) || choice !in catalog.benchmarks) return
    if (state.review.benchmark.selected == choice) return
    benchmarkActionGeneration++
    benchmarkJob?.cancel()
    dispatch(DesktopEvent.GoBenchmarkSelected(choice))
  }

  /**
   * The explicit action may trust local execution only after the displayed fixed argv is selected.
   */
  fun compareSelectedGoBenchmark() {
    val (draft, project, file) = currentBenchmarkDraft() ?: return
    val catalog = state().review.benchmark.catalog ?: return
    val choice = state().review.benchmark.selected ?: return
    if (!catalog.available || !catalog.matches(draft) || choice !in catalog.benchmarks) {
      dispatch(
          DesktopEvent.Failed("Benchmark selection is stale; list compatible benchmarks again."))
      return
    }
    val request =
        BenchmarkActionRequest(
            WorkflowProjectIdentity(project.projectId, project.projectRevision),
            draftIdentity(draft, project, file),
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
            dispatch(DesktopEvent.GoBenchmarkComparisonStopped)
            dispatch(DesktopEvent.Failed(error.message ?: "Benchmark comparison failed"))
          } catch (error: Exception) {
            if (!isCurrentBenchmarkAction(request)) return@launch
            dispatch(DesktopEvent.GoBenchmarkComparisonStopped)
            dispatch(DesktopEvent.Failed(error.message ?: "Benchmark comparison failed"))
          }
        }
  }

  private fun isCurrentBenchmarkAction(request: BenchmarkActionRequest): Boolean =
      request.generation == benchmarkActionGeneration &&
          state().project?.let { WorkflowProjectIdentity(it.projectId, it.projectRevision) } ==
              request.project &&
          currentBenchmarkDraftIdentity() == request.draft &&
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

  private fun currentBenchmarkDraft(): Triple<DeclarationDraft, ProjectAnalysis, ProjectFileInfo>? {
    val state = state()
    val draft = state.review.draft ?: return null
    val project = state.project ?: return null
    val file = state.selectedFile ?: return null
    if (state.review.editor?.status != DraftEditorStatus.Valid ||
        draft.validation?.applicable != true ||
        !draftEditorMatchesOpenFile(state.review.editor, file, project)) {
      dispatch(DesktopEvent.Failed("Validate the current draft before comparing a benchmark."))
      return null
    }
    return Triple(draft, project, file)
  }

  private fun currentBenchmarkDraftIdentity(): WorkflowDraftIdentity? =
      currentDraftIdentity()?.takeIf {
        val draft = state().review.draft
        state().review.editor?.status == DraftEditorStatus.Valid &&
            draft?.validation?.applicable == true
      }

  private fun currentDraftIdentity(): WorkflowDraftIdentity? {
    val current = state()
    val draft = current.review.draft ?: return null
    val project = current.project ?: return null
    val file = current.selectedFile ?: return null
    return draftIdentity(draft, project, file)
  }

  private fun draftIdentity(
      draft: DeclarationDraft,
      project: ProjectAnalysis,
      file: ProjectFileInfo,
  ): WorkflowDraftIdentity =
      WorkflowDraftIdentity(
          WorkflowFileIdentity(
              WorkflowProjectIdentity(project.projectId, project.projectRevision),
              file.path,
              file.contentHash),
          draft.id,
          draft.revision,
          draft.hash)

  private suspend fun <T> io(block: () -> T): T =
      withContext(ioDispatcher) { runInterruptible { block() } }
}

private fun GoBenchmarkCatalog.matches(draft: DeclarationDraft): Boolean =
    draftId == draft.id &&
        draftRevision == draft.revision &&
        draftHash == draft.hash &&
        projectId == draft.projectId &&
        projectRevision == draft.projectRevision &&
        baseFileHash == draft.baseFileHash &&
        targetPath == draft.targetPath

private fun GoBenchmarkCatalog.matches(identity: WorkflowDraftIdentity): Boolean =
    draftId == identity.id && draftRevision == identity.revision && draftHash == identity.hash

private fun GoBenchmarkComparison.matches(draft: DeclarationDraft): Boolean =
    draftId == draft.id &&
        draftRevision == draft.revision &&
        draftHash == draft.hash &&
        projectId == draft.projectId &&
        projectRevision == draft.projectRevision &&
        baseFileHash == draft.baseFileHash &&
        targetPath == draft.targetPath
