package io.miniorca.desktop

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext

/** Admits project actions and reads daemon-owned progress. Selection never schedules analysis. */
internal class DesktopAnalysisWorkflow(
    private val api: ApiClient,
    private val scope: CoroutineScope,
    private val ioDispatcher: CoroutineDispatcher,
    private val coordinator: DesktopJobCoordinator,
    private val state: () -> DesktopState,
    private val dispatch: (DesktopEvent) -> Unit,
) {
  val fileSelection = DesktopAnalysisSelectionWorkflow(api, scope, ioDispatcher, state, dispatch)
  private var actionJob: Job? = null
  private var generation = 0L
  private val resultJobs = mutableMapOf<AnalysisResultKey, Job>()
  private val resultGenerations = mutableMapOf<AnalysisResultKey, Long>()
  private val current
    get() = state().analysisRun

  fun beforeEvent(event: DesktopEvent) {
    if (event is DesktopEvent.FindingStatusUpdated) {
      // A read begun before the explicit write cannot undo the newly accepted triage state.
      cancelResultReads()
      update(current.copy(sections = current.sections.mapValues { it.value.copy(loading = false) }))
    }
    if (event is DesktopEvent.ProjectLoaded ||
        event is DesktopEvent.IndexRefreshed &&
            event.index.projectRevision != state().project?.projectRevision)
        detach()
  }

  /** Disconnects this client without canceling the durable daemon run. */
  fun detach() {
    fileSelection.detach()
    generation++
    actionJob?.cancel()
    coordinator.stopAnalysisPolling()
    cancelResultReads()
    update(
        current.copy(
            admission = null,
            action = "",
            sections = current.sections.mapValues { it.value.copy(loading = false) }))
  }

  private fun cancelResultReads() {
    resultJobs.values.forEach(Job::cancel)
    resultJobs.clear()
    resultGenerations.clear()
  }

  fun dismissAdmission() {
    if (current.action == "preview") {
      generation++
      actionJob?.cancel()
    }
    update(
        current.copy(
            admission = null,
            action = if (current.action == "preview") "" else current.action,
            error = null))
    project()?.let { observe(it, generation, current.run) }
  }

  fun providerChanged() {
    // A pending preview also belongs to the old destination catalog.
    detach()
    refresh()
  }

  fun confirmProvider(id: String, confirmed: Boolean) {
    val admission = current.admission ?: return
    if (admission.preview.providers.none { it.id == id && it.remoteConfirmationRequired }) return
    update(
        current.copy(
            admission =
                admission.copy(
                    providerIds =
                        if (confirmed) admission.providerIds + id else admission.providerIds - id)))
  }

  fun confirmSecurity(confirmed: Boolean) {
    val admission = current.admission ?: return
    update(current.copy(admission = admission.copy(securityReview = confirmed)))
  }

  fun preview(
      limits: AnalysisRunLimits = AnalysisRunLimits(100, 900, 2),
      resume: Boolean = false,
      retryStaleFailed: Boolean = false,
      refresh: Boolean = !retryStaleFailed
  ) {
    val project = project() ?: return
    val run = current.run
    val resumeRun = if (resume) run ?: return else null
    val capturedLimits = resumeRun?.plan?.limits ?: limits
    val capturedRefresh = resumeRun?.plan?.refresh ?: refresh
    val capturedRetry = resumeRun?.plan?.retryStaleFailed ?: retryStaleFailed
    val token = begin("preview")
    actionJob =
        scope.launch {
          try {
            val preview = io {
              api.previewAnalysis(
                  AnalysisPreviewRequest(
                      project.id,
                      project.revision,
                      "project",
                      capturedRefresh,
                      capturedLimits,
                      resumeRun?.identity,
                      capturedRetry))
            }
            if (!isCurrent(project, token)) return@launch
            require(
                preview.schemaVersion == "1" &&
                    preview.scope == "project" &&
                    preview.identity.projectId == project.id &&
                    preview.identity.projectRevision == project.revision &&
                    preview.limits == capturedLimits &&
                    preview.refresh == capturedRefresh &&
                    preview.retryStaleFailed == capturedRetry &&
                    (resumeRun == null || preview.identity == resumeRun.identity.queue())) {
                  "The analysis preview no longer matches this project. Request a fresh preview."
                }
            update(
                current.copy(
                    action = "", admission = AnalysisAdmission(preview, resumeRun?.identity)))
            observe(project, token, current.run)
          } catch (error: CancellationException) {
            throw error
          } catch (error: Exception) {
            fail(project, token, error, "Analysis preview failed")
            observe(project, token, current.run)
          }
        }
  }

  fun admit() {
    val project = project() ?: return
    val admission = current.admission ?: return
    if (!admission.isConfirmed()) {
      update(
          current.copy(
              error = "Confirm each remote destination and the Security review before starting."))
      return
    }
    val preview = admission.preview
    if (preview.identity.projectId != project.id ||
        preview.identity.projectRevision != project.revision ||
        admission.resumeRun != null && admission.resumeRun != current.run?.identity) {
      update(
          current.copy(
              admission = null, error = "Analysis scope changed. Request a fresh preview."))
      return
    }
    val confirmations =
        AnalysisRunConfirmations(admission.providerIds.toList(), admission.securityReview)
    // Consume before dispatch, including queued cancellation and uncertain/failed HTTP responses.
    val token = begin(if (admission.resumeRun == null) "start" else "resume")
    actionJob =
        scope.launch {
          try {
            val run = io {
              if (admission.resumeRun == null)
                  api.startAnalysis(
                      AnalysisRunStartRequest(
                          preview.identity,
                          preview.previewId,
                          preview.limits,
                          preview.refresh,
                          confirmations,
                          preview.retryStaleFailed))
              else
                  api.controlAnalysis(
                      AnalysisRunControlRequest(
                          admission.resumeRun, "resume", preview.previewId, confirmations))
            }
            if (!isCurrent(project, token)) return@launch
            require(
                run.identity.queue() == preview.identity &&
                    (admission.resumeRun == null || run.identity.id == admission.resumeRun.id)) {
                  "The returned analysis does not match its admission preview."
                }
            update(current.copy(action = ""))
            observe(project, token, run)
          } catch (error: CancellationException) {
            throw error
          } catch (error: Exception) {
            recover(project, token, error)
          }
        }
  }

  fun control(action: String) {
    require(action == "pause" || action == "cancel")
    val project = project() ?: return
    val run = current.run ?: return
    val token = begin(action)
    actionJob =
        scope.launch {
          try {
            val updated = io {
              api.controlAnalysis(AnalysisRunControlRequest(run.identity, action))
            }
            if (!isCurrent(project, token)) return@launch
            require(updated.identity == run.identity) {
              "Analysis changed while updating its controls."
            }
            update(current.copy(action = ""))
            observe(project, token, updated)
          } catch (error: CancellationException) {
            throw error
          } catch (error: Exception) {
            recover(project, token, error)
          }
        }
  }

  fun refresh() {
    fileSelection.refresh()
    if (actionJob?.isActive == true) return
    val project = project() ?: return
    val token = begin("refresh")
    actionJob = scope.launch { readCurrent(project, token) }
  }

  private suspend fun recover(project: WorkflowProjectIdentity, token: Long, error: Exception) {
    if (!isCurrent(project, token)) return
    fail(project, token, error, "Analysis action failed")
    // A transport failure may follow durable admission. Recover by reading, never retrying consent.
    readCurrent(project, token, keepError = true)
  }

  private suspend fun readCurrent(
      project: WorkflowProjectIdentity,
      token: Long,
      keepError: Boolean = false
  ) {
    try {
      val run = io { api.analysisRun(project.id, project.revision) }
      if (!isCurrent(project, token)) return
      update(current.copy(action = "", error = if (keepError) current.error else null))
      observe(project, token, run)
    } catch (error: CancellationException) {
      throw error
    } catch (error: Exception) {
      fail(project, token, error, "Analysis status could not be read")
      // Overview exposes retained progress after a save fault so explicit recovery controls remain
      // reachable.
      try {
        val retained = io { api.overview(project.revision).analysisRun }
        if (isCurrent(project, token) && retained != null) acceptRun(project, retained)
      } catch (canceled: CancellationException) {
        throw canceled
      } catch (_: Exception) {
        // Preserve the original status failure and all evidence if optional recovery also fails.
      }
    }
  }

  private fun observe(project: WorkflowProjectIdentity, token: Long, seed: AnalysisRun?) {
    if (!isCurrent(project, token)) return
    coordinator.observeAnalysis(
        project,
        seed,
        onUpdate = { run ->
          if (!isCurrent(project, token)) false
          else if (run != null && seed != null && run.identity != seed.identity) {
            update(
                current.copy(
                    admission = null, error = "Analysis was replaced. Refresh its status."))
            false
          } else acceptRun(project, run)
        },
        fetch = { io { api.analysisRun(project.id, project.revision) } },
        onFailure = { error -> fail(project, token, error, "Analysis status could not be read") })
  }

  private fun acceptRun(project: WorkflowProjectIdentity, run: AnalysisRun?): Boolean {
    if (run != null &&
        (run.schemaVersion != "1" ||
            run.identity.projectId != project.id ||
            run.identity.projectRevision != project.revision && run.status != "stale")) {
      update(current.copy(error = "The returned analysis belongs to another project revision."))
      return false
    }
    val statusChanged = current.run?.status != run?.status
    if (current.run?.identity != run?.identity) {
      resultJobs.values.forEach(Job::cancel)
      resultJobs.clear()
      resultGenerations.clear()
      update(current.copy(run = run, sections = emptyMap(), admission = null))
    } else update(current.copy(run = run))
    if (statusChanged) fileSelection.refresh()
    if (run != null && run.identity.projectRevision == project.revision) {
      (listOf("bugs", "performance", "security").map { AnalysisResultKey(it) } +
              current.sections.keys)
          .distinct()
          .forEach { if (resultJobs[it]?.isActive != true) loadResults(it.category, it.path) }
    }
    return true
  }

  fun retryResults(category: String, path: String = "") {
    require(category in setOf("bugs", "performance", "security"))
    if (current.sections[AnalysisResultKey(category, path)]?.loading == true) return
    loadResults(category, path)
  }

  fun loadResults(category: String, path: String = "") {
    require(category in setOf("bugs", "performance", "security"))
    val project = project() ?: return
    val run = current.run ?: return
    if (run.identity.projectRevision != project.revision) return
    val key = AnalysisResultKey(category, path)
    if (path.isNotEmpty() && run.files.none { it.path == path }) {
      section(key, AnalysisSectionState(error = "This file is outside the captured project run."))
      return
    }
    resultJobs[key]?.cancel()
    val token = (resultGenerations[key] ?: 0) + 1
    resultGenerations[key] = token
    section(
        key, (current.sections[key] ?: AnalysisSectionState()).copy(loading = true, error = null))
    resultJobs[key] =
        scope.launch {
          try {
            val result = io { api.analysisResults(run.identity, category, path) }
            if (!isCurrentResult(project, run.identity, key, token)) return@launch
            require(
                result.identity == run.identity &&
                    result.progress.category == category &&
                    result.path == path &&
                    result.semantic.all {
                      it.category == category &&
                          it.projectId == project.id &&
                          run.files.any { file -> file.path == it.location.path } &&
                          (path.isEmpty() || it.location.path == path) &&
                          (it.freshness != "fresh" ||
                              it.projectRevision == project.revision &&
                                  run.files.any { file ->
                                    file.path == it.location.path && file.contentHash == it.fileHash
                                  })
                    } &&
                    result.unclassified.all {
                      it.category !in setOf("bugs", "performance", "security") &&
                          it.projectId == project.id &&
                          run.files.any { file -> file.path == it.location.path } &&
                          (path.isEmpty() || it.location.path == path)
                    } &&
                    (category == "performance" || result.performance.isEmpty()) &&
                    (category == "security" || result.security.isEmpty()) &&
                    result.performance.all {
                      it.projectId == project.id &&
                          it.projectRevision == project.revision &&
                          run.files.any { file ->
                            file.path == it.path && file.contentHash == it.contentHash
                          } &&
                          (path.isEmpty() || it.path == path)
                    } &&
                    result.security.all {
                      it.source in setOf("ai", "deterministic") &&
                          it.projectId == project.id &&
                          it.projectRevision == project.revision &&
                          run.files.any { file ->
                            file.path == it.path && file.contentHash == it.contentHash
                          } &&
                          (path.isEmpty() || it.path == path)
                    }) {
                  "The returned evidence does not match this analysis section."
                }
            if (current.run == run) section(key, AnalysisSectionState(results = result))
          } catch (error: CancellationException) {
            throw error
          } catch (error: Exception) {
            if (isCurrentResult(project, run.identity, key, token))
                section(
                    key,
                    (current.sections[key] ?: AnalysisSectionState()).copy(
                        loading = false,
                        error = error.message ?: "Section results could not be read"))
          } finally {
            if (isCurrentResult(project, run.identity, key, token) && current.run != run)
                loadResults(category, path)
          }
        }
  }

  private fun isCurrentResult(
      project: WorkflowProjectIdentity,
      identity: AnalysisRunIdentity,
      key: AnalysisResultKey,
      token: Long
  ) = project() == project && current.run?.identity == identity && resultGenerations[key] == token

  private fun section(key: AnalysisResultKey, value: AnalysisSectionState) =
      update(current.copy(sections = current.sections + (key to value)))

  private fun begin(action: String): Long {
    generation++
    actionJob?.cancel()
    coordinator.stopAnalysisPolling()
    update(current.copy(action = action, admission = null, error = null))
    return generation
  }

  private fun fail(
      project: WorkflowProjectIdentity,
      token: Long,
      error: Exception,
      fallback: String
  ) {
    if (isCurrent(project, token))
        update(current.copy(action = "", error = error.message ?: fallback))
  }

  private fun project(): WorkflowProjectIdentity? =
      state().project?.let { WorkflowProjectIdentity(it.projectId, it.projectRevision) }

  private fun isCurrent(project: WorkflowProjectIdentity, token: Long) =
      generation == token && project() == project

  private fun update(value: ProjectAnalysisRunState) =
      dispatch(DesktopEvent.AnalysisRunUpdated(value))

  private suspend fun <T> io(block: () -> T): T =
      withContext(ioDispatcher) { runInterruptible { block() } }
}
