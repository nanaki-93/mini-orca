package io.miniorca.desktop

import androidx.compose.ui.graphics.Color

internal enum class AnalysisFileSyncStatus(val label: String, val tint: Color) {
  Updated("Up to date", Success),
  Missing("Not analyzed", Warning),
  Stale("Outdated", Warning),
  Partial("Incomplete", Warning),
  Failed("Failed", Error),
  Running("Analyzing", Information),
  Pending("Waiting", Information),
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
) {
  val needsAttention
    get() = status !in setOf(AnalysisFileSyncStatus.Updated, AnalysisFileSyncStatus.Excluded)
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
  return AnalysisFileStatus(file, status, explanation)
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
  All("All files"),
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
