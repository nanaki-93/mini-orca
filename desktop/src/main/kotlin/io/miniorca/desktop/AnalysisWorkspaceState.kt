package io.miniorca.desktop

internal const val defaultAnalyzeAllFileLimit = 100
private const val maximumAnalyzeAllFileLimit = 500
internal const val defaultAnalyzeAllRetryLimit = 1
private const val maximumAnalyzeAllRetryLimit = 3
internal val defaultAnalysisRunLimits = AnalysisRunLimits(100, 900, 2)

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

internal enum class AnalysisRunCommand(val label: String) {
  Start("Start analysis"),
  RetryStaleFailed("Analyze stale & failed"),
  Pause("Pause"),
  Resume("Resume"),
  Cancel("Cancel")
}

internal data class AnalysisStageFailure(
    val path: String,
    val stage: String,
    val attempts: Int,
    val reason: String
)

internal data class ProjectRunPresentation(
    val status: String,
    val headline: String,
    val totalSteps: Int,
    val finishedSteps: Int,
    val totalFiles: Int,
    val finishedFiles: Int,
    val currentFiles: List<String>,
    val failures: List<AnalysisStageFailure>,
    val commands: List<AnalysisRunCommand>,
    val isActive: Boolean,
) {
  val fileProgress: Float?
    get() = if (totalFiles == 0) null else finishedFiles.toFloat() / totalFiles
}

internal fun analysisStageFinished(status: String): Boolean =
    status in setOf("completed", "completed_empty", "partial", "failed", "skipped", "unavailable")

internal fun currentProjectRun(run: AnalysisRun?, project: ProjectAnalysis?): AnalysisRun? =
    run?.takeIf {
      project != null &&
          it.identity.projectId == project.projectId &&
          it.identity.projectRevision == project.projectRevision
    }

internal fun AnalysisRun.showsProgressOnSummary(): Boolean =
    isActive() || status in setOf("paused", "interrupted")

internal fun projectRunPresentation(analysis: ProjectAnalysisRunState): ProjectRunPresentation {
  val run = analysis.run
  val stages = run?.files.orEmpty().flatMap { it.stages }
  val plannedFiles = run?.plan?.files.orEmpty().associateBy { it.path }
  val finishedSteps = stages.count { analysisStageFinished(it.status) }
  return ProjectRunPresentation(
      status = analysisStatusLabel(run?.status),
      headline = analysisRunHeadline(run, finishedSteps, stages.size),
      totalSteps = stages.size,
      finishedSteps = finishedSteps,
      totalFiles = run?.files?.size ?: 0,
      finishedFiles =
          run?.files.orEmpty().count { file ->
            file.stages.isNotEmpty() && file.stages.all { analysisStageFinished(it.status) }
          },
      currentFiles =
          run?.files
              .orEmpty()
              .filter { file -> file.stages.any { it.status == "running" } }
              .map { it.path },
      failures =
          run?.files.orEmpty().flatMap { file ->
            val ineligibleStages =
                plannedFiles[file.path]?.stages.orEmpty().filterNot { it.eligible }.map { it.stage }
            file.stages
                .filter {
                  it.status in setOf("failed", "unavailable", "interrupted") &&
                      it.reason.isNotBlank() &&
                      !(it.status == "unavailable" && it.stage in ineligibleStages)
                }
                .map { AnalysisStageFailure(file.path, it.stage, it.attempts, it.reason) }
          },
      commands =
          when (run?.status) {
            "queued",
            "running" -> listOf(AnalysisRunCommand.Pause, AnalysisRunCommand.Cancel)
            "pausing" -> listOf(AnalysisRunCommand.Cancel)
            "canceling" -> emptyList()
            "paused",
            "interrupted" -> listOf(AnalysisRunCommand.Resume, AnalysisRunCommand.Cancel)
            "stale" ->
                listOf(
                    AnalysisRunCommand.Start,
                    AnalysisRunCommand.RetryStaleFailed,
                    AnalysisRunCommand.Cancel)
            else -> listOf(AnalysisRunCommand.Start, AnalysisRunCommand.RetryStaleFailed)
          },
      isActive = run?.isActive() == true)
}

internal fun analysisRunHeadline(run: AnalysisRun?, finishedSteps: Int, totalSteps: Int): String {
  if (run == null) return "Last run · None"
  val facts = mutableListOf(if (run.isActive()) "Current run" else "Last run")
  if (run.isActive()) {
    if (run.windowFilesCompleted > 0) {
      facts +=
          "${run.windowFilesCompleted} ${if (run.windowFilesCompleted == 1) "file" else "files"} processed"
    }
    if (run.windowElapsedSeconds > 0) facts += "${run.windowElapsedSeconds}s elapsed"
  } else {
    if (run.status !in setOf("completed", "completed_empty"))
        facts += analysisStatusLabel(run.status)
    if (totalSteps > 0) facts += "$finishedSteps of $totalSteps stages"
    run.updatedAt.takeIf { it.isNotBlank() }?.let { facts += it }
  }
  return facts.joinToString(" · ")
}

internal fun analysisStatusLabel(status: String?): String =
    when (status) {
      null,
      "" -> "Not started"
      "completed_empty" -> "Completed · no findings"
      else -> status.replace('_', ' ').replaceFirstChar { it.uppercase() }
    }

internal enum class AnalysisResultType(val category: String, val workspace: Workspace) {
  Bugs("bugs", Workspace.Bugs),
  Performance("performance", Workspace.Performance),
  Security("security", Workspace.Security);

  companion object {
    fun fromCategory(category: String): AnalysisResultType =
        entries.firstOrNull { it.category == category }
            ?: error("Unknown analysis category: $category")
  }
}

internal fun analysisCategoryWorkspace(category: String): Workspace =
    AnalysisResultType.fromCategory(category).workspace

internal fun analysisCategoryLabel(category: String): String =
    analysisCategoryWorkspace(category).name

internal enum class AnalysisResultAvailability {
  NoProject,
  NotStarted,
  Loading,
  Running,
  Paused,
  Interrupted,
  CompletedEmpty,
  PendingDetails,
  Partial,
  Failed,
  Canceled,
  Unavailable,
  Stale,
  Error,
  FilterNoMatch,
}

internal data class AnalysisResultEmptyPresentation(
    val availability: AnalysisResultAvailability,
    val message: String,
    val detail: String,
)

/** Current progress and retained evidence stay distinct across all project files. */
internal data class AnalysisResultPageState(
    val type: AnalysisResultType,
    val project: ProjectAnalysis?,
    val run: AnalysisRun?,
    val section: AnalysisSectionState = AnalysisSectionState(),
) {
  val category: String
    get() = type.category

  val results: AnalysisSectionResults?
    get() =
        section.results?.takeIf {
          it.identity == run?.identity &&
              it.identity.projectId == project?.projectId &&
              it.progress.category == category &&
              it.progress == progress &&
              it.path.isEmpty()
        }

  val stale: Boolean
    get() = run?.status == "stale" || run?.identity?.projectRevision != project?.projectRevision

  val progress: AnalysisSectionProgress?
    get() =
        run?.takeIf { it.identity.projectId == project?.projectId }
            ?.sections
            ?.firstOrNull { it.category == category }

  val statusLabel: String
    get() = if (stale && run != null) "Stale" else analysisStatusLabel(progress?.status)

  val reportedCount: Int?
    get() = if (stale) null else progress?.findingCount

  val coverageLabel: String?
    get() = if (stale) null else analysisCoverageLabel(progress?.coverage)

  val runTimeLabel: String?
    get() =
        run?.takeIf { it.identity.projectId == project?.projectId }
            ?.let { it.elapsedSeconds.takeIf { seconds -> seconds > 0 } ?: it.windowElapsedSeconds }
            ?.takeIf { it > 0 }
            ?.let { "Run time · ${it}s" }

  /**
   * Empty-result precedence is deliberate: stale evidence and refresh errors are never presented as
   * a clean result, and completed-empty requires both the current category record and its details.
   */
  fun emptyPresentation(loadedRows: Int): AnalysisResultEmptyPresentation {
    check(loadedRows == 0) { "Empty presentation requires no loaded rows." }
    if (project == null)
        return AnalysisResultEmptyPresentation(
            AnalysisResultAvailability.NoProject, "Open a project to view analysis results.", "")
    if (stale && run != null)
        return AnalysisResultEmptyPresentation(
            AnalysisResultAvailability.Stale,
            "Results are out of date.",
            "View analysis to refresh evidence for the current project revision.")
    if (section.error != null)
        return AnalysisResultEmptyPresentation(
            AnalysisResultAvailability.Error, "Results could not be refreshed.", section.error)
    if (section.loading)
        return AnalysisResultEmptyPresentation(
            AnalysisResultAvailability.Loading, "Loading results…", "")
    val currentRun = currentProjectRun(run, project)
    if (currentRun == null)
        return AnalysisResultEmptyPresentation(
            AnalysisResultAvailability.NotStarted,
            "Analysis has not started.",
            "View analysis to start a run for this project.")
    val currentProgress =
        progress
            ?: return AnalysisResultEmptyPresentation(
                AnalysisResultAvailability.PendingDetails,
                "Result details are not available yet.",
                "View analysis for the current category status.")
    val currentReportedCount = reportedCount
    return when (currentProgress.status) {
      "queued" ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.Running,
              "Analysis is queued.",
              "View analysis for the current category status.")
      "running",
      "pausing",
      "canceling" ->
          if (currentReportedCount != null && currentReportedCount > 0)
              AnalysisResultEmptyPresentation(
                  AnalysisResultAvailability.PendingDetails,
                  "No results loaded yet.",
                  "$currentReportedCount findings were reported. View analysis for the current category status.")
          else
              AnalysisResultEmptyPresentation(
                  AnalysisResultAvailability.Running,
                  "No findings yet.",
                  coverageLabel.orEmpty())
      "paused" ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.Paused,
              "Analysis is paused.",
              "View analysis to resume it.")
      "interrupted" ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.Interrupted,
              "Analysis was interrupted.",
              "View analysis to resume or start another run.")
      "completed",
      "completed_empty" ->
          if (currentReportedCount == 0 &&
              results?.progress?.status in setOf("completed", "completed_empty") &&
              results?.progress?.findingCount == 0)
              AnalysisResultEmptyPresentation(
                  AnalysisResultAvailability.CompletedEmpty,
                  "No findings in the analyzed scope.",
                  "")
          else
              AnalysisResultEmptyPresentation(
                  AnalysisResultAvailability.PendingDetails,
                  if (currentReportedCount != null && currentReportedCount > 0)
                      "$currentReportedCount findings were reported."
                  else "Completed result details are not available yet.",
                  "View analysis for the current category status.")
      "partial" ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.Partial,
              "Analysis completed partially.",
              "View analysis for incomplete coverage.")
      "failed" ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.Failed,
              "Analysis failed for this category.",
              currentRun.reason.ifBlank { "View analysis to retry or inspect the failure." })
      "canceled",
      "cancelled" ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.Canceled,
              "Analysis was canceled.",
              "View analysis to start another run when ready.")
      "unavailable" ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.Unavailable,
              "Analysis is unavailable for this category.",
              currentRun.reason.ifBlank { "View analysis for availability details." })
      else ->
          AnalysisResultEmptyPresentation(
              AnalysisResultAvailability.PendingDetails,
              "Result details are not available yet.",
              "View analysis for the current category status.")
    }
  }

  val semantic: List<UnifiedFinding>
    get() =
        results
            ?.semantic
            .orEmpty()
            .filter {
              it.category == category &&
                  it.projectId == project?.projectId &&
                  it.projectRevision == run?.identity?.projectRevision
            }
            .map { if (stale) it.copy(freshness = "stale") else it }

  val unclassified: List<UnifiedFinding>
    get() = results?.unclassified.orEmpty().map { it.copy(freshness = "stale") }
}

internal fun DesktopState.analysisResultPage(category: String) =
    AnalysisResultPageState(
        AnalysisResultType.fromCategory(category),
        project,
        analysisRun.run,
        analysisRun.sections[AnalysisResultKey(category)] ?: AnalysisSectionState())

internal fun analysisCoverageLabel(coverage: AnalysisRunCoverage?): String? =
    coverage
        ?.takeIf { it.total > 0 }
        ?.let {
          buildList {
                add("${it.succeeded}/${it.total} stages covered")
                if (it.partial > 0) add("${it.partial} partial")
                if (it.pending + it.running > 0) add("${it.pending + it.running} remaining")
                if (it.failed > 0) add("${it.failed} failed")
                if (it.skipped > 0) add("${it.skipped} skipped")
                if (it.unavailable > 0) add("${it.unavailable} unavailable")
              }
              .joinToString(" · ")
        }
