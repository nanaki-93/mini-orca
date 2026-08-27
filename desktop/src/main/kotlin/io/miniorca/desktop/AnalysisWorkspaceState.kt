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
    fun bounded(): AnalyzeAllRunOptions = copy(
        maxFiles = maxFiles.coerceIn(1, maximumAnalyzeAllFileLimit),
        maxRetries = maxRetries.coerceIn(0, maximumAnalyzeAllRetryLimit),
    )
}

fun shouldPollAnalyzeAll(job: AnalyzeAllJob?): Boolean =
    job?.status?.lowercase() in setOf("running", "pausing", "canceling")

/**
 * Owns the tiny polling state machine so asynchronous status results cannot
 * revive a job belonging to an earlier project revision or a disposed UI.
 */
class AnalyzeAllPollingController {
    private var activeRevision: String? = null
    private var polling = false

    fun activate(revision: String) {
        activeRevision = revision
        polling = false
    }

    fun receive(revision: String, job: AnalyzeAllJob?): AnalyzeAllJob? {
        if (revision != activeRevision || job?.projectRevision?.takeIf { it.isNotBlank() }?.let { it != revision } == true) {
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

internal data class AnalyzeAllPresentation(
    val statusLabel: String,
    val statusDetail: String,
    val controls: String,
)

internal fun analyzeAllPresentation(job: AnalyzeAllJob?, coverage: AnalysisCoverage?): AnalyzeAllPresentation {
    val coverageLabel = "${coverage?.fresh ?: 0} fresh · ${coverage?.stale ?: 0} stale · ${coverage?.missing ?: 0} missing · ${coverage?.running ?: 0} running · ${coverage?.failed ?: 0} failed"
    return when (job?.status?.lowercase()) {
        "running" -> AnalyzeAllPresentation("Running", "Analyze-all is processing files at project revision ${job.projectRevision.ifBlank { "current" }}. Completed results remain available below.", "Pause or cancel")
        "pausing" -> AnalyzeAllPresentation("Pausing", "Analyze-all is pausing; completed results remain available below.", "Cancel")
        "canceling" -> AnalyzeAllPresentation("Canceling", "Analyze-all is canceling; completed results remain available below.", "Canceling")
        "paused" -> AnalyzeAllPresentation("Paused", "Analyze-all is paused. Resume explicitly to process the remaining files.", "Resume or cancel")
        "completed" -> AnalyzeAllPresentation("Completed", "Analyze-all completed for this project revision. Start a new run to refresh results.", "Start a new run")
        "canceled" -> AnalyzeAllPresentation("Canceled", "Analyze-all was canceled. Completed results remain available; start a new explicit run to continue.", "Start a new run")
        "failed" -> AnalyzeAllPresentation("Failed", "Analyze-all failed for this revision. Completed results remain available; retry explicitly.", "Retry")
        null -> AnalyzeAllPresentation("Not started", "No Analyze-all job is available (204 No Content). Import and reindex never start one automatically.", "Start")
        else -> AnalyzeAllPresentation(job.status.replaceFirstChar { it.uppercase() }, "Analyze-all is ${job.status}; stale jobs cannot resume on a newer project revision.", "Start a new run")
    }.let { it.copy(statusDetail = "Coverage: $coverageLabel. ${it.statusDetail}") }
}
