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
