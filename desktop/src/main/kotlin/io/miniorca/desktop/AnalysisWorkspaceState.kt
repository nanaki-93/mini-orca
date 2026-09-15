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
  val finishedSteps =
      stages.count {
        it.status in
            setOf("completed", "completed_empty", "partial", "failed", "skipped", "unavailable")
      }
  return ProjectRunPresentation(
      status = analysisStatusLabel(run?.status),
      headline = analysisRunHeadline(run, finishedSteps, stages.size),
      totalSteps = stages.size,
      finishedSteps = finishedSteps,
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

internal fun analysisRunHeadline(run: AnalysisRun?, finishedSteps: Int, totalSteps: Int): String {
  if (run == null) return "Last run · None"
  val facts = mutableListOf(if (run.isActive()) "Current run" else "Last run")
  if (run.isActive()) {
    if (totalSteps > 0) {
      facts += "$finishedSteps finished"
      facts += "${(totalSteps - finishedSteps).coerceAtLeast(0)} remaining"
    }
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

internal fun analysisCoverageLabel(coverage: AnalysisRunCoverage?): String =
    coverage?.let {
      "${it.succeeded} covered · ${it.partial} partial · ${it.pending + it.running} remaining · ${it.failed} failed · ${it.skipped} skipped · ${it.unavailable} unavailable"
    } ?: "Coverage is not available yet."
