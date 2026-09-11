package io.miniorca.desktop

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext

/** Owns only the explicit deterministic file scan. AI review belongs to the project run. */
internal class DesktopSecurityWorkflow(
    private val api: ApiClient,
    private val scope: CoroutineScope,
    private val ioDispatcher: CoroutineDispatcher,
    private val state: () -> DesktopState,
    private val dispatch: (DesktopEvent) -> Unit,
    private val clearRemoteConfirmation: () -> Unit,
) {
  private var securityJob: Job? = null
  private var actionGeneration = 0L

  fun beforeEvent(event: DesktopEvent) {
    val indexChanged =
        event is DesktopEvent.IndexRefreshed &&
            state().project?.projectRevision != event.index.projectRevision
    if (event is DesktopEvent.ProjectLoaded || event is DesktopEvent.FileLoaded || indexChanged) {
      cancel()
      clearRemoteConfirmation()
    }
  }

  fun cancel() {
    actionGeneration++
    securityJob?.cancel()
    securityJob = null
    if (state().security.action.isNotBlank()) dispatch(DesktopEvent.SecurityActionCanceled)
  }

  fun scanSecurity() {
    val state = state()
    val project = state.project ?: return
    val file =
        state.selectedFile
            ?: run {
              dispatch(
                  DesktopEvent.SecurityActionFailed(
                      "scan", "Open one indexed Go file before scanning."))
              return
            }
    if (file.language != "Go") {
      dispatch(
          DesktopEvent.SecurityActionFailed(
              "scan", "Security scanning currently supports one indexed Go file."))
      return
    }
    cancel()
    val generation = actionGeneration
    val identity = fileIdentity(file, project)
    dispatch(DesktopEvent.SecurityActionStarted("scan"))
    securityJob =
        scope.launch {
          try {
            val report = io { api.securityScan(file.path, project.projectRevision) }
            if (isCurrentAction(identity, generation)) {
              if (securityReportMatchesFile(report, identity, "deterministic"))
                  dispatch(DesktopEvent.SecurityReportLoaded(report))
              else
                  dispatch(
                      DesktopEvent.SecurityActionFailed(
                          "scan", "The returned scan no longer matches the selected file."))
            }
          } catch (_: CancellationException) {
            throw CancellationException()
          } catch (error: Exception) {
            if (isCurrentAction(identity, generation))
                dispatch(
                    DesktopEvent.SecurityActionFailed(
                        "scan", error.message ?: "Security scan failed"))
          }
        }
  }

  private fun isCurrentAction(identity: WorkflowFileIdentity, generation: Long): Boolean {
    val current = state()
    val project = current.project ?: return false
    val file = current.selectedFile ?: return false
    return generation == actionGeneration && fileIdentity(file, project) == identity
  }

  private fun fileIdentity(file: ProjectFileInfo, project: ProjectAnalysis): WorkflowFileIdentity =
      WorkflowFileIdentity(
          WorkflowProjectIdentity(project.projectId, project.projectRevision),
          file.path,
          file.contentHash)

  private suspend fun <T> io(block: () -> T): T =
      withContext(ioDispatcher) { runInterruptible { block() } }
}

private fun securityReportMatchesFile(
    report: SecurityFileReport,
    file: WorkflowFileIdentity,
    expectedSource: String,
): Boolean =
    report.projectId == file.project.id &&
        report.projectRevision == file.project.revision &&
        report.path == file.path &&
        report.contentHash == file.contentHash &&
        report.source == expectedSource
