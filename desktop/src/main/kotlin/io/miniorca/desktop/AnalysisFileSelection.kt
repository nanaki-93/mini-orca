package io.miniorca.desktop

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class AnalysisSelectableFile(
    val path: String,
    val reason: String,
    val stages: List<AnalysisFileStageStatus> = emptyList(),
)

@Serializable
data class AnalysisFileStageStatus(val stage: String, val status: String, val reason: String)

@Serializable
data class AnalysisFileSelection(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("selection_id") val selectionId: String,
    @SerialName("excluded_paths") val excludedPaths: List<String>,
    val files: List<AnalysisSelectableFile>,
    val editable: Boolean,
)

@Serializable
data class AnalysisSelectionRequest(
    @SerialName("project_id") val projectId: String,
    @SerialName("project_revision") val projectRevision: String,
    @SerialName("selection_id") val selectionId: String,
    @SerialName("excluded_paths") val excludedPaths: List<String>,
)

data class AnalysisSelectionState(
    val selection: AnalysisFileSelection? = null,
    val loading: Boolean = false,
    val saving: Boolean = false,
    val error: String? = null,
)

internal class DesktopAnalysisSelectionWorkflow(
    private val api: ApiClient,
    private val scope: CoroutineScope,
    private val ioDispatcher: CoroutineDispatcher,
    private val state: () -> DesktopState,
    private val dispatch: (DesktopEvent) -> Unit,
) {
  private var job: Job? = null
  private var generation = 0L
  private val current
    get() = state().analysisRun.fileSelection

  fun detach() {
    generation++
    job?.cancel()
    update(AnalysisSelectionState())
  }

  fun refresh() {
    if (current.saving) return
    request(null)
  }

  fun save(excludedPaths: List<String>) {
    val selection = current.selection ?: return
    if (current.loading || current.saving || !selection.editable) return
    request(
        AnalysisSelectionRequest(
            selection.projectId, selection.projectRevision, selection.selectionId, excludedPaths))
  }

  private fun request(request: AnalysisSelectionRequest?) {
    val project = state().project ?: return
    val identity = WorkflowProjectIdentity(project.projectId, project.projectRevision)
    val token = ++generation
    job?.cancel()
    update(current.copy(loading = request == null, saving = request != null, error = null))
    if (request != null)
        dispatch(DesktopEvent.AnalysisRunUpdated(state().analysisRun.copy(admission = null)))
    job =
        scope.launch {
          try {
            val selection =
                withContext(ioDispatcher) {
                  runInterruptible {
                    if (request == null) api.analysisSelection(identity.id, identity.revision)
                    else api.saveAnalysisSelection(request)
                  }
                }
            require(
                selection.projectId == identity.id &&
                    selection.projectRevision == identity.revision) {
                  "Analysis selection belongs to a different project revision"
                }
            if (isCurrent(identity, token)) update(AnalysisSelectionState(selection))
          } catch (error: CancellationException) {
            throw error
          } catch (error: Exception) {
            if (isCurrent(identity, token))
                update(
                    current.copy(
                        loading = false,
                        saving = false,
                        error = error.message ?: "Analysis selection could not be saved or loaded"))
          }
        }
  }

  private fun isCurrent(identity: WorkflowProjectIdentity, token: Long) =
      token == generation &&
          state().project?.let { WorkflowProjectIdentity(it.projectId, it.projectRevision) } ==
              identity

  private fun update(value: AnalysisSelectionState) =
      dispatch(DesktopEvent.AnalysisRunUpdated(state().analysisRun.copy(fileSelection = value)))
}
