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
  private var analyzeAllPoll: Job? = null
  private var performancePoll: Job? = null
  private var verifiedScanPoll: Job? = null
  private var analyzeAllGeneration = 0L
  private var performanceGeneration = 0L
  private var verifiedScanGeneration = 0L

  fun projectOpened(identity: WorkflowProjectIdentity) {
    activeProject = identity
    cancelPolling()
  }

  fun projectClosed() {
    activeProject = null
    cancelPolling()
  }

  fun beginAnalyzeAllAction(identity: WorkflowProjectIdentity) {
    if (identity == activeProject) stopAnalyzeAllPolling()
  }

  fun beginPerformanceAction(identity: WorkflowProjectIdentity) {
    if (identity == activeProject) stopPerformancePolling()
  }

  fun observeAnalyzeAll(
      identity: WorkflowProjectIdentity,
      job: AnalyzeAllJob?,
      onUpdate: (AnalyzeAllJob?) -> Boolean,
      fetch: suspend () -> AnalyzeAllJob?,
      onFailure: (Exception) -> Unit,
  ) {
    if (identity != activeProject) return
    stopAnalyzeAllPolling()
    if (!onUpdate(job)) return
    if (!shouldPollAnalyzeAll(job)) return

    val generation = analyzeAllGeneration
    analyzeAllPoll =
        scope.launch {
          while (isCurrent(identity, generation, analyzeAllGeneration)) {
            try {
              val updated = fetch()
              if (!isCurrent(identity, generation, analyzeAllGeneration)) return@launch
              if (!onUpdate(updated)) return@launch
              if (!shouldPollAnalyzeAll(updated)) return@launch
            } catch (_: CancellationException) {
              throw CancellationException()
            } catch (error: Exception) {
              if (isCurrent(identity, generation, analyzeAllGeneration)) onFailure(error)
              return@launch
            }
            delay(pollingIntervalMillis)
          }
        }
  }

  fun observePerformance(
      identity: WorkflowProjectIdentity,
      job: PerformanceJob?,
      report: PerformanceReport?,
      onUpdate: (PerformanceJob?, PerformanceReport?) -> Boolean,
      fetch: suspend () -> Pair<PerformanceJob?, PerformanceReport?>,
      onFailure: (Exception) -> Unit,
  ) {
    if (identity != activeProject) return
    stopPerformancePolling()
    if (!onUpdate(job, report)) return
    if (job?.status != "running") return

    val generation = performanceGeneration
    performancePoll =
        scope.launch {
          while (isCurrent(identity, generation, performanceGeneration)) {
            try {
              val (updatedJob, updatedReport) = fetch()
              if (!isCurrent(identity, generation, performanceGeneration)) return@launch
              if (!onUpdate(updatedJob, updatedReport)) return@launch
              if (updatedJob?.status != "running") return@launch
            } catch (_: CancellationException) {
              throw CancellationException()
            } catch (error: Exception) {
              if (isCurrent(identity, generation, performanceGeneration)) onFailure(error)
              return@launch
            }
            delay(pollingIntervalMillis)
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
    stopAnalyzeAllPolling()
    stopPerformancePolling()
    stopVerifiedScanPolling()
  }

  private fun stopAnalyzeAllPolling() {
    analyzeAllGeneration++
    analyzeAllPoll?.cancel()
    analyzeAllPoll = null
  }

  private fun stopPerformancePolling() {
    performanceGeneration++
    performancePoll?.cancel()
    performancePoll = null
  }

  private fun isCurrent(
      identity: WorkflowProjectIdentity,
      generation: Long,
      currentGeneration: Long,
  ): Boolean = activeProject == identity && generation == currentGeneration
}
