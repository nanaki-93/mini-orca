package io.miniorca.desktop

import androidx.compose.ui.graphics.Color

internal enum class AnalysisFileSyncStatus(val label: String, val tint: Color) {
  Updated("Up to date", Success),
  Missing("Not analyzed", Warning),
  Stale("Outdated", Warning),
  Partial("Incomplete", Warning),
  Failed("Failed", Error),
  Running("Running", Information),
  Pending("Pending", SecondaryText),
  Finished("Finished", Information),
  Paused("Paused", Warning),
  Interrupted("Interrupted", Warning),
  Canceled("Canceled", Warning),
  Unavailable("Unavailable", Error),
  Excluded("Excluded", SecondaryText),
  Unknown("Status unavailable", SecondaryText),
}

internal data class AnalysisFileStatus(
    val file: AnalysisSelectableFile,
    val status: AnalysisFileSyncStatus,
    val explanation: String,
    val savedStatus: AnalysisFileSyncStatus? = null,
    val summary: String = explanation,
) {
  val needsAttention
    get() = status !in setOf(AnalysisFileSyncStatus.Updated, AnalysisFileSyncStatus.Excluded)
}

/** Run progress describes admitted work, independently of saved result freshness. */
internal fun analysisFileStatuses(
    selection: AnalysisFileSelection,
    run: AnalysisRun?
): List<AnalysisFileStatus> {
  val currentRun =
      run?.takeIf {
        it.identity.projectId == selection.projectId &&
            it.identity.projectRevision == selection.projectRevision &&
            it.plan.identity == it.identity.queue() &&
            (it.isActive() || it.status in setOf("paused", "interrupted"))
      }
  val planned = currentRun?.plan?.files.orEmpty().associateBy { it.path }
  val progress =
      currentRun
          ?.files
          .orEmpty()
          .filter { planned[it.path]?.contentHash == it.contentHash }
          .associateBy { it.path }
  val excluded = selection.excludedPaths.toSet()
  return selection.files.map { file ->
    val saved = analysisFileStatus(file, file.path in excluded)
    val runningFile = progress[file.path]
    if (saved.status == AnalysisFileSyncStatus.Excluded ||
        runningFile == null ||
        runningFile.stages.isEmpty())
        saved
    else {
      val stages =
          runningFile.stages.filter { stage ->
            planned[file.path]?.stages.orEmpty().none { it.stage == stage.stage && !it.eligible }
          }
      val status =
          when {
            stages.isEmpty() -> saved.status
            stages.any {
              !analysisStageFinished(it.status) &&
                  it.status !in setOf("running", "pending", "paused", "interrupted", "canceled")
            } -> AnalysisFileSyncStatus.Unknown
            stages.any { it.status == "running" } -> AnalysisFileSyncStatus.Running
            stages.any { it.status == "failed" } -> AnalysisFileSyncStatus.Failed
            stages.any { it.status == "unavailable" } -> AnalysisFileSyncStatus.Unavailable
            stages.any { it.status == "partial" } -> AnalysisFileSyncStatus.Partial
            stages.all { analysisStageFinished(it.status) } ->
                if (saved.status == AnalysisFileSyncStatus.Updated) AnalysisFileSyncStatus.Updated
                else AnalysisFileSyncStatus.Finished
            currentRun?.status == "paused" -> AnalysisFileSyncStatus.Paused
            currentRun?.status == "interrupted" -> AnalysisFileSyncStatus.Interrupted
            stages.any { it.status == "canceled" } -> AnalysisFileSyncStatus.Canceled
            stages.any { it.status == "paused" } -> AnalysisFileSyncStatus.Paused
            stages.any { it.status == "interrupted" } -> AnalysisFileSyncStatus.Interrupted
            stages.any { it.status == "pending" } -> AnalysisFileSyncStatus.Pending
            else -> AnalysisFileSyncStatus.Unknown
          }
      val details =
          stages
              .filter {
                !analysisStageFinished(it.status) ||
                    it.status in setOf("failed", "partial", "unavailable")
              }
              .joinToString("\n") { stage ->
                "${analysisStageLabel(stage.stage)} · ${analysisStatusLabel(stage.status)}${stage.reason.takeIf { it.isNotBlank() }?.let { ": $it" }.orEmpty()}"
              }
              .ifBlank { "All run stages finished." }
      if (stages.isEmpty() || status == AnalysisFileSyncStatus.Updated) saved
      else
          saved.copy(
              status = status,
              explanation = details,
              savedStatus = saved.status,
              summary =
                  when (status) {
                    AnalysisFileSyncStatus.Running ->
                        stages
                            .filter { it.status == "running" }
                            .joinToString(" · ") { analysisStageLabel(it.stage) }
                    AnalysisFileSyncStatus.Pending -> "Queued"
                    AnalysisFileSyncStatus.Finished -> "All run stages finished."
                    else ->
                        stages
                            .firstOrNull { it.status == status.stageStatus }
                            ?.let {
                              "${analysisStageLabel(it.stage)}: ${it.reason.ifBlank { analysisStatusLabel(it.status) }}"
                            } ?: details.lineSequence().first()
                  })
    }
  }
}

internal fun analysisFileStatus(
    file: AnalysisSelectableFile,
    excluded: Boolean = false
): AnalysisFileStatus {
  if (file.reason.isNotBlank())
      return AnalysisFileStatus(file, AnalysisFileSyncStatus.Excluded, file.reason)
  if (excluded) return AnalysisFileStatus(file, AnalysisFileSyncStatus.Excluded, "Excluded by you.")
  val stages = file.stages.filter { it.status != "skipped" }
  val status =
      when {
        file.stages.isEmpty() -> AnalysisFileSyncStatus.Unknown
        stages.isEmpty() -> AnalysisFileSyncStatus.Excluded
        stages.all { it.status == "fresh" } -> AnalysisFileSyncStatus.Updated
        else -> analysisIncompleteFileStatus(stages)
      }
  val outstanding = file.stages.filter { it.status != "fresh" && it.status != "skipped" }
  val explanations =
      outstanding.map {
        "${analysisStageLabel(it.stage)}: ${it.reason.ifBlank { "No explanation was supplied for this stage." }}"
      }
  val explanation =
      when (status) {
        AnalysisFileSyncStatus.Updated -> "All applicable stages have current analysis."
        AnalysisFileSyncStatus.Unknown ->
            "No analysis status is available. Refresh files to load the saved results."
        AnalysisFileSyncStatus.Excluded ->
            file.stages.joinToString(" ") { "${analysisStageLabel(it.stage)}: ${it.reason}" }
        else -> explanations.joinToString("\n")
      }
  val summary =
      when (status) {
        AnalysisFileSyncStatus.Updated -> "Complete"
        AnalysisFileSyncStatus.Excluded,
        AnalysisFileSyncStatus.Unknown -> explanation
        else ->
            (outstanding.firstOrNull { it.status == status.stageStatus }
                    ?: outstanding.firstOrNull())
                ?.reason
                ?.takeIf { it.isNotBlank() } ?: explanation
      }
  return AnalysisFileStatus(file, status, explanation, summary = summary)
}

private val AnalysisFileSyncStatus.stageStatus: String
  get() =
      when (this) {
        AnalysisFileSyncStatus.Updated -> "fresh"
        AnalysisFileSyncStatus.Finished -> "completed"
        else -> name.lowercase()
      }

private fun analysisIncompleteFileStatus(
    stages: List<AnalysisFileStageStatus>
): AnalysisFileSyncStatus {
  val statuses = stages.map { it.status }.toSet()
  return when {
    statuses.any {
      it !in
          setOf(
              "fresh",
              "missing",
              "failed",
              "stale",
              "unavailable",
              "interrupted",
              "canceled",
              "paused",
              "pending",
              "partial",
              "running")
    } -> AnalysisFileSyncStatus.Unknown
    "running" in statuses -> AnalysisFileSyncStatus.Running
    "failed" in statuses -> AnalysisFileSyncStatus.Failed
    "stale" in statuses -> AnalysisFileSyncStatus.Stale
    "unavailable" in statuses -> AnalysisFileSyncStatus.Unavailable
    "interrupted" in statuses -> AnalysisFileSyncStatus.Interrupted
    "canceled" in statuses -> AnalysisFileSyncStatus.Canceled
    "paused" in statuses -> AnalysisFileSyncStatus.Paused
    "pending" in statuses -> AnalysisFileSyncStatus.Pending
    "partial" in statuses || "fresh" in statuses -> AnalysisFileSyncStatus.Partial
    statuses == setOf("missing") -> AnalysisFileSyncStatus.Missing
    else -> AnalysisFileSyncStatus.Unknown
  }
}

internal enum class AnalysisFileFilter(val label: String) {
  All("All"),
  Attention("Needs attention"),
  Updated("Up to date"),
  Excluded("Excluded")
}

internal fun filteredAnalysisFiles(
    files: List<AnalysisFileStatus>,
    query: String,
    filter: AnalysisFileFilter
): List<AnalysisFileStatus> =
    files.filter { row ->
      row.file.path.contains(query, ignoreCase = true) &&
          when (filter) {
            AnalysisFileFilter.All -> true
            AnalysisFileFilter.Attention -> row.needsAttention
            AnalysisFileFilter.Updated -> row.status == AnalysisFileSyncStatus.Updated
            AnalysisFileFilter.Excluded -> row.status == AnalysisFileSyncStatus.Excluded
          }
    }

/** Counts respect the active search query so a tab's number always matches what it would show. */
internal fun analysisFileFilterCounts(
    files: List<AnalysisFileStatus>,
    query: String,
): Map<AnalysisFileFilter, Int> =
    AnalysisFileFilter.entries.associateWith { filteredAnalysisFiles(files, query, it).size }

internal fun analysisSelectionCoverage(selection: AnalysisFileSelection): AnalysisCoverage {
  val excluded = selection.excludedPaths.toSet()
  val statuses = selection.files.map { analysisFileStatus(it, it.path in excluded).status }
  return AnalysisCoverage(
      total = statuses.count { it != AnalysisFileSyncStatus.Excluded },
      fresh = statuses.count { it == AnalysisFileSyncStatus.Updated },
      stale = statuses.count { it == AnalysisFileSyncStatus.Stale },
      missing = statuses.count { it == AnalysisFileSyncStatus.Missing },
      failed = statuses.count { it == AnalysisFileSyncStatus.Failed },
      running =
          statuses.count {
            it in setOf(AnalysisFileSyncStatus.Running, AnalysisFileSyncStatus.Pending)
          },
      partial =
          statuses.count {
            it in
                setOf(
                    AnalysisFileSyncStatus.Partial,
                    AnalysisFileSyncStatus.Paused,
                    AnalysisFileSyncStatus.Interrupted,
                    AnalysisFileSyncStatus.Canceled)
          },
      unavailable =
          statuses.count {
            it in setOf(AnalysisFileSyncStatus.Unavailable, AnalysisFileSyncStatus.Unknown)
          })
}

internal fun analysisCoverageStatus(coverage: AnalysisCoverage): String =
    when {
      coverage.running > 0 -> "running"
      coverage.failed > 0 -> "failed"
      coverage.stale > 0 -> "stale"
      coverage.unavailable > 0 -> "unavailable"
      coverage.partial > 0 || coverage.missing > 0 && coverage.fresh > 0 -> "partial"
      coverage.missing > 0 -> "missing"
      coverage.total > 0 && coverage.total == coverage.fresh -> "fresh"
      coverage.total == 0 -> "excluded"
      else -> "unavailable"
    }
