package io.miniorca.desktop

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext
import kotlinx.coroutines.yield

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
  private var admissionDismissed = false
  private var pendingAdmissionToken: Long? = null
  private var statusReadVersion = 0L
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
    if (event is DesktopEvent.AnalysisRunUpdated &&
        (current.previewIntent != null || current.action.isNotEmpty()) &&
        (event.state.fileSelection.saving && !current.fileSelection.saving ||
            event.state.run?.identity != current.run?.identity ||
            current.previewIntent?.resumeRun != null &&
                (event.state.run?.identity != current.previewIntent?.resumeRun ||
                    event.state.run?.plan != current.previewIntent?.resumePlan))) {
      generation++
      actionJob?.cancel()
      pendingAdmissionToken = null
      if (current.action in setOf("pause", "cancel") &&
          event.state.run?.identity != current.run?.identity) {
        val project = project()
        val replacement = event.state.run
        val token = generation
        if (project != null) {
          scope.launch {
            // beforeEvent runs before the replacement is reduced into state.
            yield()
            if (isCurrent(project, token) && current.run?.identity == replacement?.identity) {
              update(current.copy(action = ""))
              if (replacement != null &&
                  replacement.identity.projectId == project.id &&
                  replacement.identity.projectRevision == project.revision)
                  observe(project, token, replacement)
            }
          }
        }
      }
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
    pendingAdmissionToken = null
    coordinator.stopAnalysisPolling()
    cancelResultReads()
    update(
        current.copy(
            admission = null,
            previewIntent = null,
            admissionRecovery = null,
            action = "",
            sections = current.sections.mapValues { it.value.copy(loading = false) }))
  }

  private fun cancelResultReads() {
    resultJobs.values.forEach(Job::cancel)
    resultJobs.clear()
    resultGenerations.clear()
  }

  fun dismissAdmission() {
    val pendingAdmission = actionJob?.isActive == true && current.action in setOf("start", "resume")
    if (pendingAdmission) admissionDismissed = true
    else {
      generation++
      actionJob?.cancel()
    }
    update(
        current.copy(
            admission = null,
            previewIntent = null,
            admissionRecovery = null,
            action = "",
            error = null,
            errorKind = null))
    // Keep an in-flight admission alive: the daemon may admit it after this first status read.
    // Its eventual response (or failure) will reconcile again without restoring permission.
    if (pendingAdmission) {
      val project = project() ?: return
      val token = generation
      val readVersion = ++statusReadVersion
      scope.launch { readCurrent(project, token, readVersion = readVersion) }
    } else project()?.let { observe(it, generation, current.run) }
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
    if (pendingAdmissionToken != null) return
    val project = project() ?: return
    val run = current.run
    val resumeRun = if (resume) run ?: return else null
    val capturedLimits = resumeRun?.plan?.limits ?: limits
    val capturedRefresh = resumeRun?.plan?.refresh ?: refresh
    val capturedRetry = resumeRun?.plan?.retryStaleFailed ?: retryStaleFailed
    val intent =
        AnalysisPreviewIntent(
            project.id,
            project.revision,
            capturedLimits,
            capturedRefresh,
            capturedRetry,
            resumeRun?.identity,
            resumeRun?.plan)
    requestPreview(intent)
  }

  fun retryPreview() {
    val intent = current.previewIntent ?: return
    if (current.action.isNotEmpty() || current.admission != null || current.error == null) return
    if (!matchesIntent(intent)) {
      update(
          current.copy(
              previewIntent = null,
              admission = null,
              error = "Analysis scope changed. Close and request a fresh preview.",
              errorKind = AnalysisRunErrorKind.Action))
      return
    }
    requestPreview(intent)
  }

  private fun matchesIntent(intent: AnalysisPreviewIntent): Boolean {
    val project = project() ?: return false
    if (project.id != intent.projectId || project.revision != intent.projectRevision) return false
    if (intent.resumeRun == null) return true
    val run = current.run ?: return false
    return run.identity == intent.resumeRun && run.plan == intent.resumePlan
  }

  private fun requestPreview(intent: AnalysisPreviewIntent) {
    val project = WorkflowProjectIdentity(intent.projectId, intent.projectRevision)
    val token = begin("preview", intent)
    actionJob =
        scope.launch {
          try {
            val preview = io { api.previewAnalysis(intent.request()) }
            if (!isCurrent(project, token)) return@launch
            require(matchesIntent(intent)) {
              "Analysis scope changed. Close and request a fresh preview."
            }
            require(
                preview.schemaVersion == "1" &&
                    preview.scope == "project" &&
                    preview.identity.projectId == project.id &&
                    preview.identity.projectRevision == project.revision &&
                    preview.limits == intent.limits &&
                    preview.refresh == intent.refresh &&
                    preview.retryStaleFailed == intent.retryStaleFailed &&
                    (intent.resumeRun == null || preview.identity == intent.resumeRun.queue())) {
                  "The analysis preview no longer matches this project. Request a fresh preview."
                }
            update(
                current.copy(action = "", admission = AnalysisAdmission(preview, intent.resumeRun)))
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
    if (admission.preview.files.isEmpty()) {
      update(
          current.copy(
              error =
                  "No eligible files to analyze. Request a new preview after changing the selection.",
              errorKind = AnalysisRunErrorKind.Action))
      return
    }
    if (!admission.isConfirmed()) {
      update(
          current.copy(
              error = "Confirm each remote destination and the Security review before starting.",
              errorKind = AnalysisRunErrorKind.Action))
      return
    }
    val preview = admission.preview
    val intent = current.previewIntent
    if (intent == null ||
        !matchesIntent(intent) ||
        preview.identity.projectId != project.id ||
        preview.identity.projectRevision != project.revision ||
        admission.resumeRun != intent.resumeRun ||
        preview.limits != intent.limits ||
        preview.refresh != intent.refresh ||
        preview.retryStaleFailed != intent.retryStaleFailed) {
      update(
          current.copy(
              admission = null,
              error = "Analysis scope changed. Request a fresh preview.",
              errorKind = AnalysisRunErrorKind.Action))
      return
    }
    val confirmations =
        AnalysisRunConfirmations(admission.providerIds.toList(), admission.securityReview)
    // Consume before dispatch, including queued cancellation and uncertain/failed HTTP responses.
    val token = begin(if (admission.resumeRun == null) "start" else "resume")
    pendingAdmissionToken = token
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
            recover(
                project,
                token,
                error,
                intent.takeUnless { admissionDismissed },
                PendingAdmissionRun(preview.identity, admission.resumeRun, intent.resumePlan))
          } finally {
            if (pendingAdmissionToken == token) pendingAdmissionToken = null
          }
        }
  }

  fun control(action: String) {
    require(action == "pause" || action == "cancel")
    if (pendingAdmissionToken != null) return
    val project = project() ?: return
    val run = current.run ?: return
    val command = if (action == "pause") AnalysisRunCommand.Pause else AnalysisRunCommand.Cancel
    if (command !in analysisRunCommands(state().project, run)) return
    val pending = current.action
    if (pending.isNotEmpty() && !(pending == "pause" && action == "cancel")) return
    val token = begin(action)
    actionJob =
        scope.launch {
          if (!isCurrentControl(project, run.identity, token)) return@launch
          if (command !in analysisRunCommands(state().project, current.run)) {
            update(current.copy(action = ""))
            observe(project, token, current.run)
            return@launch
          }
          try {
            val updated = io {
              api.controlAnalysis(AnalysisRunControlRequest(run.identity, action))
            }
            if (!isCurrentControl(project, run.identity, token)) return@launch
            require(updated.identity == run.identity) {
              "Analysis changed while updating its controls."
            }
            update(current.copy(action = ""))
            observe(project, token, updated)
          } catch (error: CancellationException) {
            throw error
          } catch (error: Exception) {
            if (isCurrentControl(project, run.identity, token)) recover(project, token, error)
          }
        }
  }

  private fun isCurrentControl(
      project: WorkflowProjectIdentity,
      identity: AnalysisRunIdentity,
      token: Long
  ) = isCurrent(project, token) && current.run?.identity == identity

  fun refresh() {
    fileSelection.refresh()
    if (actionJob?.isActive == true) return
    val project = project() ?: return
    val token = begin("refresh")
    actionJob = scope.launch { readCurrent(project, token) }
  }

  private data class PendingAdmissionRun(
      val queue: AnalysisQueueIdentity,
      val resumeRun: AnalysisRunIdentity?,
      val resumePlan: AnalysisRunPreview?
  ) {
    fun matches(run: AnalysisRun): Boolean =
        run.identity.queue() == queue &&
            (resumeRun == null || run.identity.id == resumeRun.id && run.plan == resumePlan)
  }

  private suspend fun recover(
      project: WorkflowProjectIdentity,
      token: Long,
      error: Exception,
      admissionIntent: AnalysisPreviewIntent? = null,
      pendingAdmission: PendingAdmissionRun? = null
  ) {
    if (!isCurrent(project, token)) return
    fail(project, token, error, "Analysis action failed")
    // Only a known rejection can retain the request scope for an explicit fresh preview.
    // An uncertain response may already have admitted the run; never restore its consent.
    val rejected = error is ApiException && error.status == 409
    update(
        current.copy(
            admissionRecovery =
                if (pendingAdmission == null) null
                else if (rejected) AdmissionRecovery.Rejected else AdmissionRecovery.Uncertain,
            previewIntent = admissionIntent?.takeIf { rejected && matchesIntent(it) }))
    readCurrent(
        project,
        token,
        keepError = true,
        uncertainAdmission =
            pendingAdmission.takeUnless { error is ApiException && error.status == 409 })
  }

  private suspend fun readCurrent(
      project: WorkflowProjectIdentity,
      token: Long,
      keepError: Boolean = false,
      uncertainAdmission: PendingAdmissionRun? = null,
      readVersion: Long = ++statusReadVersion
  ) {
    val priorRun = current.run
    try {
      val run = io { api.analysisRun(project.id, project.revision) }
      if (!isCurrent(project, token) || readVersion != statusReadVersion || current.run != priorRun)
          return
      update(
          current.copy(
              action = "",
              error = if (keepError) current.error else null,
              errorKind = if (keepError) current.errorKind else null))
      observe(project, token, run, uncertainAdmission)
    } catch (error: CancellationException) {
      throw error
    } catch (error: Exception) {
      if (!isCurrent(project, token) || readVersion != statusReadVersion || current.run != priorRun)
          return
      if (keepError && current.error != null)
          update(
              current.copy(
                  action = "",
                  error =
                      "${current.error} · Analysis status could not be read: ${error.message ?: "unavailable"}",
                  errorKind = AnalysisRunErrorKind.StatusRead))
      else statusReadFailed(project, token, error)
      // Overview exposes retained progress after a save fault so explicit recovery controls remain
      // reachable.
      try {
        val retained = io { api.overview(project.revision).analysisRun }
        if (isCurrent(project, token) &&
            readVersion == statusReadVersion &&
            current.run == priorRun &&
            retained != null)
            acceptRun(project, retained)
      } catch (canceled: CancellationException) {
        throw canceled
      } catch (_: Exception) {
        // Preserve the original status failure and all evidence if optional recovery also fails.
      }
    }
  }

  private fun observe(
      project: WorkflowProjectIdentity,
      token: Long,
      seed: AnalysisRun?,
      uncertainAdmission: PendingAdmissionRun? = null
  ) {
    if (!isCurrent(project, token)) return
    val observedRun = current.run
    var acceptedIdentity = seed?.identity
    coordinator.observeAnalysis(
        project,
        seed,
        onUpdate = { run ->
          if (!isCurrent(project, token)) false
          else if (observedRun?.identity != current.run?.identity &&
              run?.identity != current.run?.identity)
              false
          else if (run != null &&
              acceptedIdentity != null &&
              run.identity != acceptedIdentity &&
              (uncertainAdmission == null ||
                  seed?.identity != observedRun?.identity ||
                  acceptedIdentity != seed?.identity ||
                  !uncertainAdmission.matches(run))) {
            update(
                current.copy(
                    admission = null,
                    previewIntent = null,
                    error =
                        listOfNotNull(current.error, "Analysis was replaced. Refresh its status.")
                            .joinToString(" · "),
                    errorKind = AnalysisRunErrorKind.Action))
            false
          } else {
            if (acceptRun(project, run)) {
              acceptedIdentity = run?.identity
              true
            } else false
          }
        },
        fetch = { io { api.analysisRun(project.id, project.revision) } },
        onFailure = { error ->
          if (isCurrent(project, token) && current.error != null)
              update(
                  current.copy(
                      error =
                          "${current.error} · Analysis status could not be read: ${error.message ?: "unavailable"}",
                      errorKind = AnalysisRunErrorKind.StatusRead))
          else statusReadFailed(project, token, error)
        })
  }

  private fun acceptRun(project: WorkflowProjectIdentity, run: AnalysisRun?): Boolean {
    if (run != null &&
        (run.schemaVersion != "1" ||
            run.identity.projectId != project.id ||
            run.identity.projectRevision != project.revision && run.status != "stale")) {
      update(
          current.copy(
              error = "The returned analysis belongs to another project revision.",
              errorKind = AnalysisRunErrorKind.Action))
      return false
    }
    val statusChanged = current.run?.status != run?.status
    val resumeObsolete =
        current.previewIntent?.let {
          it.resumeRun != null && (run?.identity != it.resumeRun || run.plan != it.resumePlan)
        } == true
    if (current.run?.identity != run?.identity) {
      val prior = current.run
      val previousRun =
          if (run != null &&
              prior != null &&
              prior.status in
                  setOf(
                      "completed",
                      "completed_empty",
                      "partial",
                      "failed",
                      "unavailable",
                      "canceled"))
              prior
          else current.previousRun
      resultJobs.values.forEach(Job::cancel)
      resultJobs.clear()
      resultGenerations.clear()
      update(
          current.copy(
              run = run,
              previousRun = previousRun,
              sections = emptyMap(),
              admission = null,
              previewIntent =
                  current.previewIntent.takeUnless {
                    it?.resumeRun != null || current.run != null
                  }))
    } else
        update(
            current.copy(
                run = run,
                admission = current.admission.takeUnless { resumeObsolete },
                previewIntent = current.previewIntent.takeUnless { resumeObsolete }))
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

  private fun begin(action: String, intent: AnalysisPreviewIntent? = null): Long {
    admissionDismissed = false
    generation++
    actionJob?.cancel()
    coordinator.stopAnalysisPolling()
    update(
        current.copy(
            action = action,
            admission = null,
            previewIntent = intent,
            admissionRecovery = null,
            error = null,
            errorKind = null))
    return generation
  }

  private fun fail(
      project: WorkflowProjectIdentity,
      token: Long,
      error: Exception,
      fallback: String
  ) {
    if (isCurrent(project, token))
        update(
            current.copy(
                action = "",
                error = error.message ?: fallback,
                errorKind = AnalysisRunErrorKind.Action))
  }

  private fun statusReadFailed(project: WorkflowProjectIdentity, token: Long, error: Exception) {
    if (isCurrent(project, token))
        update(
            current.copy(
                action = "",
                error = error.message ?: "Analysis status could not be read",
                errorKind = AnalysisRunErrorKind.StatusRead))
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
