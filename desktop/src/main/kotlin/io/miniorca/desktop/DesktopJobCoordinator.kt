package io.miniorca.desktop

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Coordinates project-wide polling without owning the workflow snapshot or publishing state. */
internal class DesktopJobCoordinator(
    parentScope: CoroutineScope,
    private val pollingIntervalMillis: Long,
) : AutoCloseable {
  private val lifetime = SupervisorJob(parentScope.coroutineContext[Job])
  private val scope = CoroutineScope(parentScope.coroutineContext + lifetime)

  private var activeProject: WorkflowProjectIdentity? = null
  private var analysisPoll: Job? = null
  private var verifiedScanPoll: Job? = null
  private var analysisGeneration = 0L
  private var verifiedScanGeneration = 0L

  fun projectOpened(identity: WorkflowProjectIdentity) {
    activeProject = identity
    cancelPolling()
  }

  fun projectClosed() {
    activeProject = null
    cancelPolling()
  }

  fun observeAnalysis(
      identity: WorkflowProjectIdentity,
      run: AnalysisRun?,
      onUpdate: (AnalysisRun?) -> Boolean,
      fetch: suspend () -> AnalysisRun?,
      onFailure: (Exception) -> Unit,
  ) {
    if (identity != activeProject) return
    stopAnalysisPolling()
    if (!onUpdate(run) || run?.isActive() != true) return
    val generation = analysisGeneration
    analysisPoll =
        scope.launch {
          while (isCurrent(identity, generation, analysisGeneration)) {
            delay(pollingIntervalMillis)
            try {
              val updated = fetch()
              if (!isCurrent(identity, generation, analysisGeneration)) return@launch
              if (!onUpdate(updated) || updated?.isActive() != true) return@launch
            } catch (error: CancellationException) {
              throw error
            } catch (error: Exception) {
              if (isCurrent(identity, generation, analysisGeneration)) onFailure(error)
              return@launch
            }
          }
        }
  }

  fun observeVerifiedScan(
      identity: WorkflowProjectIdentity,
      scan: GoScanReport?,
      onUpdate: (GoScanReport?) -> Boolean,
      onSeedTerminal: () -> Unit,
      onPollTerminal: () -> Unit,
      fetch: suspend () -> GoScanReport?,
      onFailure: (Exception) -> Unit,
  ) {
    if (identity != activeProject) return
    stopVerifiedScanPolling()
    if (!onUpdate(scan)) return
    if (!shouldPollVerifiedScan(scan)) {
      onSeedTerminal()
      return
    }

    val generation = verifiedScanGeneration
    verifiedScanPoll =
        scope.launch {
          while (isCurrent(identity, generation, verifiedScanGeneration)) {
            try {
              val updated = fetch()
              if (!isCurrent(identity, generation, verifiedScanGeneration)) return@launch
              if (!onUpdate(updated)) return@launch
              if (!shouldPollVerifiedScan(updated)) {
                onPollTerminal()
                return@launch
              }
            } catch (_: CancellationException) {
              throw CancellationException()
            } catch (error: Exception) {
              if (isCurrent(identity, generation, verifiedScanGeneration)) onFailure(error)
              return@launch
            }
            delay(pollingIntervalMillis)
          }
        }
  }

  fun stopVerifiedScanPolling() {
    verifiedScanGeneration++
    verifiedScanPoll?.cancel()
    verifiedScanPoll = null
  }

  override fun close() {
    projectClosed()
    lifetime.cancel()
  }

  private fun cancelPolling() {
    stopAnalysisPolling()
    stopVerifiedScanPolling()
  }

  fun stopAnalysisPolling() {
    analysisGeneration++
    analysisPoll?.cancel()
    analysisPoll = null
  }

  private fun isCurrent(
      identity: WorkflowProjectIdentity,
      generation: Long,
      currentGeneration: Long,
  ): Boolean = activeProject == identity && generation == currentGeneration
}
