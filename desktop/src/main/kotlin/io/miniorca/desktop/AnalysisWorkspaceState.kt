package io.miniorca.desktop

internal const val defaultAnalyzeAllFileLimit = 100
private const val maximumAnalyzeAllFileLimit = 500
internal const val defaultAnalyzeAllRetryLimit = 1
private const val maximumAnalyzeAllRetryLimit = 3

/** The bounded, user-confirmed options for one explicit Analyze-all run. */
data class AnalyzeAllRunOptions(
    val maxFiles: Int = defaultAnalyzeAllFileLimit,
    val maxRetries: Int = defaultAnalyzeAllRetryLimit,
    val confirmRemoteProvider: Boolean = false,
) {
  fun bounded(): AnalyzeAllRunOptions =
      copy(
          maxFiles = maxFiles.coerceIn(1, maximumAnalyzeAllFileLimit),
          maxRetries = maxRetries.coerceIn(0, maximumAnalyzeAllRetryLimit),
      )
}

fun shouldPollAnalyzeAll(job: AnalyzeAllJob?): Boolean =
    job?.status?.lowercase() in setOf("running", "pausing", "canceling")

/**
 * Owns the tiny polling state machine so asynchronous status results cannot revive a job belonging
 * to an earlier project revision or a disposed UI.
 */
class AnalyzeAllPollingController {
  private var activeRevision: String? = null
  private var polling = false

  fun activate(revision: String) {
    activeRevision = revision
    polling = false
  }

  fun receive(revision: String, job: AnalyzeAllJob?): AnalyzeAllJob? {
    if (revision != activeRevision ||
        job?.projectRevision?.takeIf { it.isNotBlank() }?.let { it != revision } == true) {
      polling = false
      return null
    }
    polling = shouldPollAnalyzeAll(job)
    return job
  }

  fun shouldPoll(revision: String): Boolean = polling && revision == activeRevision

  fun stop() {
    polling = false
  }

  fun dispose() {
    activeRevision = null
    polling = false
  }
}

internal data class AnalysisCoveragePresentation(
    val total: Int,
    val fresh: Int,
    val stale: Int,
    val missing: Int,
    val running: Int,
    val failed: Int,
)

internal data class AnalyzeAllRunPresentation(
    val statusLabel: String,
    val statusDetail: String,
    val controls: String,
    val maxFiles: Int,
    val maxRetries: Int,
    val candidates: Int,
    val completed: Int,
    val failed: Int,
    val running: Int,
    val remaining: Int,
)

internal data class AnalysisFailurePresentation(
    val path: String,
    val attempts: Int,
    val error: String,
)

internal data class AnalyzeAllPresentation(
    val coverage: AnalysisCoveragePresentation,
    val run: AnalyzeAllRunPresentation,
    val failures: List<AnalysisFailurePresentation>,
) {
  val statusLabel: String
    get() = run.statusLabel

  val statusDetail: String
    get() = "Coverage: ${coverageSummary(coverage)}. ${run.statusDetail}"

  val controls: String
    get() = run.controls

  val noErrorsMessage: String
    get() = "No analysis errors in this run."
}

internal fun analyzeAllPresentation(
    job: AnalyzeAllJob?,
    coverage: AnalysisCoverage?
): AnalyzeAllPresentation {
  val coveragePresentation = coverage.toPresentation()
  val fileStates = job?.files.orEmpty().map(::classifyFile)
  val failures = fileStates.filter { it.isFailure }.map { it.file.toFailurePresentation() }
  val completed = fileStates.count { it.isCompleted }
  val failed = fileStates.count { it.isFailure }
  val running = fileStates.count { it.isRunning }
  val candidates = fileStates.size
  val run =
      AnalyzeAllRunPresentation(
          statusLabel = job.statusLabel(),
          statusDetail = job.statusDetail(),
          controls = job.controls(),
          maxFiles = job?.maxFiles ?: 0,
          maxRetries = job?.maxRetries ?: 0,
          candidates = candidates,
          completed = completed,
          failed = failed,
          running = running,
          remaining = (candidates - completed - failed - running).coerceAtLeast(0),
      )
  return AnalyzeAllPresentation(coveragePresentation, run, failures)
}

private data class AnalyzeAllFileState(
    val file: AnalyzeAllFileJob,
    val isFailure: Boolean,
    val isCompleted: Boolean,
    val isRunning: Boolean,
)

private fun classifyFile(file: AnalyzeAllFileJob): AnalyzeAllFileState {
  val status = file.status.normalizedAnalyzeAllStatus()
  val isFailure = status == "failed" || file.error.isNotBlank()
  return AnalyzeAllFileState(
      file = file,
      isFailure = isFailure,
      isCompleted = !isFailure && status == "completed",
      isRunning = !isFailure && status == "running",
  )
}

private fun AnalysisCoverage?.toPresentation() =
    AnalysisCoveragePresentation(
        total = this?.total ?: 0,
        fresh = this?.fresh ?: 0,
        stale = this?.stale ?: 0,
        missing = this?.missing ?: 0,
        running = this?.running ?: 0,
        failed = this?.failed ?: 0,
    )

private fun AnalyzeAllFileJob.toFailurePresentation() =
    AnalysisFailurePresentation(
        path = path,
        attempts = attempts,
        error = error.trim().ifBlank { "Analysis failed" },
    )

private fun AnalyzeAllJob?.statusLabel() =
    when (this?.status.normalizedAnalyzeAllStatus()) {
      null -> "Not started"
      "running" -> "Running"
      "pausing" -> "Pausing"
      "canceling" -> "Canceling"
      "paused" -> "Paused"
      "completed" -> "Completed"
      "canceled" -> "Canceled"
      "failed" -> "Failed"
      "stale" -> "Stale"
      else -> this?.status.orEmpty().ifBlank { "Unknown" }.replaceFirstChar { it.uppercase() }
    }

private fun AnalyzeAllJob?.statusDetail() =
    when (this?.status.normalizedAnalyzeAllStatus()) {
      null ->
          "No Analyze-all job is available (204 No Content). Import and reindex never start one automatically."
      "running" ->
          "Analyze-all is processing candidates. Run counts and analysis errors update as work completes."
      "pausing" -> "Analyze-all is pausing; the run summary and analysis errors remain visible."
      "canceling" -> "Analyze-all is canceling; the run summary and analysis errors remain visible."
      "paused" -> "Analyze-all is paused. Resume explicitly to process the remaining candidates."
      "completed" -> "Analyze-all completed. Start a new run to refresh coverage."
      "canceled" ->
          "Analyze-all was canceled. Its run summary and recorded analysis errors remain visible; start a new explicit run to continue."
      "failed" -> "Analyze-all failed. Inspect the analysis errors and retry explicitly."
      "stale" -> "Analyze-all is out of date and cannot resume. Start a new run."
      else ->
          "Analyze-all is ${this?.status.orEmpty().ifBlank { "in an unknown state" }}; start a new run for the current project."
    }

private fun AnalyzeAllJob?.controls() =
    when (this?.status.normalizedAnalyzeAllStatus()) {
      "running" -> "Pause or cancel"
      "pausing" -> "Cancel"
      "canceling" -> "Canceling"
      "paused" -> "Resume or cancel"
      "failed" -> "Retry"
      "stale",
      "completed",
      "canceled" -> "Start a new run"
      else -> "Start"
    }

private fun String?.normalizedAnalyzeAllStatus(): String? =
    this?.trim()?.lowercase()?.ifBlank { null }

private fun coverageSummary(coverage: AnalysisCoveragePresentation): String =
    "${coverage.fresh} fresh · ${coverage.stale} stale · ${coverage.missing} missing · ${coverage.running} running · ${coverage.failed} failed"
