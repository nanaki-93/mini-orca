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
    val totalSteps: Int,
    val finishedSteps: Int,
    val currentFiles: List<String>,
    val failures: List<AnalysisStageFailure>,
    val commands: List<AnalysisRunCommand>,
) {
  val progress: Float
    get() = if (totalSteps == 0) 0f else finishedSteps.toFloat() / totalSteps
}

internal fun projectRunPresentation(analysis: ProjectAnalysisRunState): ProjectRunPresentation {
  val run = analysis.run
  val stages = run?.files.orEmpty().flatMap { it.stages }
  val plannedFiles = run?.plan?.files.orEmpty().associateBy { it.path }
  return ProjectRunPresentation(
      status = analysisStatusLabel(run?.status),
      totalSteps = stages.size,
      finishedSteps =
          stages.count {
            it.status in
                setOf("completed", "completed_empty", "partial", "failed", "skipped", "unavailable")
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
          })
}

internal fun analysisStatusLabel(status: String?): String =
    when (status) {
      null,
      "" -> "Not started"
      "completed_empty" -> "Completed · no findings"
      else -> status.replace('_', ' ').replaceFirstChar { it.uppercase() }
    }

internal fun analysisCategoryWorkspace(category: String): Workspace =
    when (category) {
      "bugs" -> Workspace.Bugs
      "performance" -> Workspace.Performance
      "security" -> Workspace.Security
      else -> error("Unknown analysis category: $category")
    }

internal fun analysisCategoryLabel(category: String): String =
    analysisCategoryWorkspace(category).name

/**
 * Current progress and retained evidence stay distinct; local filters cannot change project
 * coverage.
 */
internal data class AnalysisResultPageState(
    val category: String,
    val project: ProjectAnalysis?,
    val run: AnalysisRun?,
    val section: AnalysisSectionState = AnalysisSectionState(),
    val path: String = "",
) {
  val results: AnalysisSectionResults?
    get() =
        section.results?.takeIf {
          it.identity == run?.identity &&
              it.identity.projectId == project?.projectId &&
              it.progress.category == category &&
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

  val semantic: List<UnifiedFinding>
    get() =
        results
            ?.semantic
            .orEmpty()
            .filter {
              it.category == category &&
                  it.projectId == project?.projectId &&
                  it.projectRevision == run?.identity?.projectRevision &&
                  matchesPath(it.location.path)
            }
            .map { if (stale) it.copy(freshness = "stale") else it }

  val unclassified: List<UnifiedFinding>
    get() =
        results
            ?.unclassified
            .orEmpty()
            .filter { matchesPath(it.location.path) }
            .map { it.copy(freshness = "stale") }

  fun matchesPath(candidate: String): Boolean =
      path.isBlank() || candidate.contains(path, ignoreCase = true)
}

internal fun DesktopState.analysisResultPage(category: String) =
    AnalysisResultPageState(
        category,
        project,
        analysisRun.run,
        analysisRun.sections[AnalysisResultKey(category)] ?: AnalysisSectionState(),
        analysisRun.resultPaths[category].orEmpty())

internal fun analysisCoverageLabel(coverage: AnalysisRunCoverage?): String =
    coverage?.let {
      "${it.succeeded} covered · ${it.partial} partial · ${it.pending + it.running} remaining · ${it.failed} failed · ${it.skipped} skipped · ${it.unavailable} unavailable"
    } ?: "Coverage is not available yet."
