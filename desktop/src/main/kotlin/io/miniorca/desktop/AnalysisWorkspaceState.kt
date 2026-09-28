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
  Resume("Resume → fresh preview"),
  Cancel("Cancel")
}

internal data class AnalysisStageFailure(
    val path: String,
    val stage: String,
    val attempts: Int,
    val reason: String
)

internal data class AnalysisStageDetail(
    val path: String,
    val status: String?,
    val attempts: Int?,
    val reused: Boolean?,
    val eligible: Boolean,
    val reason: String,
)

internal data class AnalysisStageSummary(
    val stage: String,
    val files: List<AnalysisStageDetail>,
) {
  val total: Int
    get() = files.size

  val running: Int
    get() = files.count { it.status == "running" }

  val pending: Int
    get() = files.count { it.status == "pending" }

  val finished: Int
    get() = files.count { it.status != null && analysisStageFinished(it.status) }

  val missing: Int
    get() = files.count { it.status == null }

  val attention: Int
    get() =
        files.count {
          it.eligible &&
              it.status != null &&
              it.status !in setOf("completed", "completed_empty", "running", "pending")
        }

  val statuses: Map<String, Int>
    get() = files.mapNotNull { it.status }.groupingBy { it }.eachCount()
}

internal enum class RunProgressAvailability {
  NotStarted,
  Available,
  EmptyScope,
  Incomplete,
  Unavailable,
}

internal data class ProjectRunPresentation(
    val status: String,
    val headline: String,
    val totalSteps: Int,
    val finishedSteps: Int,
    val totalFiles: Int,
    val finishedFiles: Int,
    val progressAvailability: RunProgressAvailability,
    val currentFiles: List<String>,
    val failures: List<AnalysisStageFailure>,
    val stages: List<AnalysisStageSummary>,
    val reportedAttempts: Int?,
    val lifecycleExplanation: String?,
    val commands: List<AnalysisRunCommand>,
    val isActive: Boolean,
) {
  val fileProgress: Float?
    get() =
        if (progressAvailability == RunProgressAvailability.Available && totalFiles > 0)
            finishedFiles.toFloat() / totalFiles
        else null
}

internal fun analysisStageFinished(status: String): Boolean =
    status in setOf("completed", "completed_empty", "partial", "failed", "skipped", "unavailable")

internal fun currentProjectRun(run: AnalysisRun?, project: ProjectAnalysis?): AnalysisRun? =
    run?.takeIf {
      project != null &&
          it.identity.projectId == project.projectId &&
          it.identity.projectRevision == project.projectRevision
    }

internal fun analysisRunRevisionOutdated(run: AnalysisRun?, project: ProjectAnalysis?): Boolean =
    run != null &&
        project != null &&
        run.identity.projectId == project.projectId &&
        run.identity.projectRevision != project.projectRevision

internal fun analysisRunOutdated(run: AnalysisRun?, project: ProjectAnalysis?): Boolean =
    run != null && currentProjectRun(run, project) == null

internal fun analysisRunStale(run: AnalysisRun?, project: ProjectAnalysis?): Boolean =
    run?.status == "stale" || analysisRunOutdated(run, project)

/** Run-bound commands require a matching project; an absent run has no run identity to guard. */
internal fun analysisRunCommands(
    project: ProjectAnalysis?,
    run: AnalysisRun?,
): List<AnalysisRunCommand> {
  if (run != null && currentProjectRun(run, project) == null) return emptyList()
  return when (run?.status) {
    null,
    "stale",
    "canceled",
    "completed",
    "completed_empty",
    "partial",
    "failed",
    "unavailable" -> listOf(AnalysisRunCommand.Start, AnalysisRunCommand.RetryStaleFailed)
    "queued",
    "running" -> listOf(AnalysisRunCommand.Pause, AnalysisRunCommand.Cancel)
    "pausing" -> listOf(AnalysisRunCommand.Cancel)
    "paused",
    "interrupted" -> listOf(AnalysisRunCommand.Resume, AnalysisRunCommand.Cancel)
    else -> emptyList()
  }
}

internal fun AnalysisRun.showsProgressOnSummary(): Boolean =
    isActive() || status in setOf("paused", "interrupted")

internal fun projectRunPresentation(
    project: ProjectAnalysis?,
    analysis: ProjectAnalysisRunState,
): ProjectRunPresentation {
  val run = analysis.run
  val outdated = analysisRunOutdated(run, project)
  val stale = analysisRunStale(run, project)
  val active = run?.isActive() == true && !stale
  val inventory = run?.let { capturedRunProgress(it, project) }
  val availability =
      when {
        run == null -> RunProgressAvailability.NotStarted
        inventory == null -> RunProgressAvailability.Unavailable
        inventory.inconsistent -> RunProgressAvailability.Incomplete
        inventory.totalFiles == 0 -> RunProgressAvailability.EmptyScope
        else -> RunProgressAvailability.Available
      }
  val stages =
      inventory
          ?.let { captured ->
            val matched = captured.files.associate { (file, _) -> file.path to file }
            val stageOrder =
                captured.planFiles.flatMap { it.stages.map { stage -> stage.stage } }.distinct()
            stageOrder.map { stageId ->
              AnalysisStageSummary(
                  stageId,
                  captured.planFiles.mapNotNull { planFile ->
                    val plannedStage =
                        planFile.stages.firstOrNull { it.stage == stageId }
                            ?: return@mapNotNull null
                    val reported =
                        matched[planFile.path]?.stages?.firstOrNull { it.stage == stageId }
                    AnalysisStageDetail(
                        path = planFile.path,
                        status = reported?.status,
                        attempts = reported?.attempts?.takeIf { it >= 0 },
                        reused = reported?.cached,
                        eligible = plannedStage.eligible,
                        reason =
                            reported?.reason?.takeIf { it.isNotBlank() }
                                ?: plannedStage.reason.takeIf {
                                  !plannedStage.eligible && it.isNotBlank()
                                }
                                ?: if (reported == null) "Stage progress has not been reported."
                                else if (reported.status in
                                    setOf("failed", "unavailable", "interrupted", "partial"))
                                    "No diagnostic was supplied for this stage."
                                else "No reason was supplied for this stage.")
                  })
            }
          }
          .orEmpty()
  val finishedSteps =
      inventory?.files.orEmpty().sumOf { (file, planned) ->
        file.stages.count { stage ->
          planned.stages.any { it.stage == stage.stage } && analysisStageFinished(stage.status)
        }
      }
  return ProjectRunPresentation(
      status = if (stale) "Stale" else analysisStatusLabel(run?.status),
      headline =
          analysisRunHeadline(run, finishedSteps, inventory?.totalSteps ?: 0, availability, stale),
      totalSteps = inventory?.totalSteps ?: 0,
      finishedSteps = finishedSteps,
      totalFiles = inventory?.totalFiles ?: 0,
      finishedFiles =
          inventory?.files.orEmpty().count { (file, planned) ->
            planned.stages.isNotEmpty() &&
                file.stages.size == planned.stages.size &&
                file.stages.all { analysisStageFinished(it.status) }
          },
      progressAvailability = availability,
      currentFiles =
          if (!active) emptyList()
          else
              inventory
                  ?.files
                  .orEmpty()
                  .filter { (file, _) -> file.stages.any { it.status == "running" } }
                  .map { it.first.path },
      stages = stages,
      reportedAttempts = inventory?.reportedAttempts(),
      lifecycleExplanation =
          if (outdated) "This run is out of date; no current lifecycle controls are available."
          else analysisRunLifecycleExplanation(run, analysis),
      failures =
          inventory?.files.orEmpty().flatMap { (file, planned) ->
            file.stages
                .filter { stage ->
                  stage.status in setOf("failed", "unavailable", "interrupted") &&
                      !(stage.status == "unavailable" &&
                          planned.stages.any { it.stage == stage.stage && !it.eligible })
                }
                .map {
                  AnalysisStageFailure(
                      file.path,
                      it.stage,
                      it.attempts,
                      it.reason.ifBlank { "No diagnostic was supplied for this stage." })
                }
          },
      commands =
          if (analysis.statusUnavailable ||
              analysis.controlRequest?.outcome in
                  setOf(AnalysisControlOutcome.Reconciling, AnalysisControlOutcome.Unconfirmed))
              emptyList()
          else analysisRunCommands(project, run),
      isActive = active)
}

internal fun analysisRunLifecycleExplanation(
    run: AnalysisRun?,
    analysis: ProjectAnalysisRunState,
): String? {
  if (analysis.statusUnavailable)
      return "Current run status unavailable; the last accepted snapshot is retained."
  if (analysis.controlRequest?.outcome == AnalysisControlOutcome.Unconfirmed)
      return "Control outcome unconfirmed; check the durable run status before another action."
  if (analysis.controlRequest?.outcome == AnalysisControlOutcome.Reconciling)
      return "Control rejected; checking the durable run status."
  val requesting =
      analysis.controlRequest?.takeIf { it.outcome == AnalysisControlOutcome.Requesting }
  if (requesting != null)
      return when (requesting.action) {
        "pause" -> "Requesting pause…"
        "cancel" -> "Requesting cancellation…"
        else -> null
      }
  return when (run?.status) {
    "queued",
    "running" -> "Work is admitted. Pause waits for the current stage boundary."
    "pausing" -> "Pause requested; waiting for the current stage boundary. No new stage will start."
    "paused" -> "Dispatch has stopped. Resume requires a fresh preview and confirmation."
    "interrupted" -> "Work was interrupted. Resume requires a fresh preview and confirmation."
    "canceling" ->
        "Cancellation accepted; active work is stopping. Completed evidence remains available."
    "canceled" ->
        "Cancellation settled. Completed evidence remains available; a new analysis requires admission."
    "partial" ->
        "Analysis completed partially; inspect the retained evidence before starting another run."
    "failed" -> "Analysis failed; inspect the retained evidence before starting another run."
    "unavailable" ->
        "Analysis unavailable; inspect the reported reason before starting another run."
    else -> null
  }
}

private data class CapturedRunProgress(
    val totalFiles: Int,
    val totalSteps: Int,
    val planFiles: List<AnalysisPlannedFile>,
    val files: List<Pair<AnalysisRunFile, AnalysisPlannedFile>>,
    val inconsistent: Boolean,
) {
  fun reportedAttempts(): Int? {
    if (inconsistent || files.size != totalFiles) return null
    var total = 0
    for ((file, planned) in files) {
      if (file.stages.size != planned.stages.size) return null
      for (stage in file.stages) {
        if (stage.attempts < 0 || stage.attempts > Int.MAX_VALUE - total) return null
        total += stage.attempts
      }
    }
    return total
  }
}

private fun capturedRunProgress(run: AnalysisRun, project: ProjectAnalysis?): CapturedRunProgress? {
  if (currentProjectRun(run, project) == null || run.plan.identity != run.identity.queue())
      return null
  val planned = run.plan.files
  // A malformed plan cannot provide a reliable denominator or stage identity.
  if (planned.any { file ->
    file.path.isBlank() ||
        file.stages.any { it.stage.isBlank() } ||
        file.stages.map { it.stage }.distinct().size != file.stages.size
  } || planned.map { it.path }.distinct().size != planned.size)
      return null
  val byPath = planned.associateBy { it.path }
  val counts = run.files.groupingBy { it.path }.eachCount()
  var inconsistent = run.files.size != planned.size
  val files =
      run.files.mapNotNull { file ->
        val plan = byPath[file.path]
        if (counts[file.path] != 1 ||
            plan == null ||
            plan.contentHash != file.contentHash ||
            plan.language != file.language ||
            file.stages.any { it.stage.isBlank() } ||
            file.stages.map { it.stage }.distinct().size != file.stages.size ||
            file.stages.any { stage -> plan.stages.none { it.stage == stage.stage } }) {
          inconsistent = true
          null
        } else {
          if (file.stages.size != plan.stages.size) inconsistent = true
          file to plan
        }
      }
  return CapturedRunProgress(
      planned.size, planned.sumOf { it.stages.size }, planned, files, inconsistent)
}

internal fun analysisRunHeadline(
    run: AnalysisRun?,
    finishedSteps: Int,
    totalSteps: Int,
    availability: RunProgressAvailability = RunProgressAvailability.Available,
    stale: Boolean = run?.status == "stale",
): String {
  if (run == null) return "Last run · None"
  val facts = mutableListOf(if (run.isActive() && !stale) "Current run" else "Last run")
  if (run.isActive() && !stale) {
    if (run.windowFilesCompleted > 0) {
      facts +=
          "${run.windowFilesCompleted} ${if (run.windowFilesCompleted == 1) "file" else "files"} processed in current window"
    }
  } else {
    if (stale) facts += "Stale"
    else if (run.status !in setOf("completed", "completed_empty"))
        facts += analysisStatusLabel(run.status)
    if (totalSteps > 0 && availability == RunProgressAvailability.Available)
        facts += "$finishedSteps of $totalSteps stages"
  }
  facts += analysisRunTimeMetadata(run)
  return facts.joinToString(" · ")
}

internal fun analysisRunTimeMetadata(run: AnalysisRun): List<String> = buildList {
  add(
      run.elapsedSeconds.takeIf { it > 0 }?.let { "Reported run time · ${it}s" }
          ?: "Reported run time · unavailable")
  run.windowElapsedSeconds.takeIf { it > 0 }?.let { add("Current window · ${it}s") }
  run.createdAt.takeIf { it.isNotBlank() }?.let { add("Created · $it") }
  run.updatedAt.takeIf { it.isNotBlank() }?.let { add("Updated · $it") }
}

/** The large heading shown above the Analysis page's run panel, distinct from the compact badge. */
internal fun analysisRunTitle(run: AnalysisRun?, presentation: ProjectRunPresentation): String =
    when {
      run == null -> "Ready to analyze"
      presentation.status == "Stale" -> "Analysis out of date"
      run.status == "queued" -> "Queued to analyze"
      run.status == "pausing" -> "Pausing analysis"
      run.status == "canceling" -> "Canceling analysis"
      run.isActive() -> "Analyzing selected files"
      run.status == "paused" -> "Analysis paused"
      run.status == "interrupted" -> "Analysis interrupted"
      run.status == "failed" -> "Analysis failed"
      run.status in setOf("completed", "completed_empty") -> "Analysis complete"
      run.status == "partial" -> "Analysis partially complete"
      run.status in setOf("canceled", "cancelled") -> "Analysis canceled"
      else -> "Analysis ${presentation.status.lowercase()}"
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

  /** A matching detail read is not the same as a completed, current result. */
  private val completedZeroDetails: Boolean
    get() =
        reportedCount == 0 &&
            !section.loading &&
            section.error == null &&
            run?.status in setOf("completed", "completed_empty") &&
            progress?.status in setOf("completed", "completed_empty") &&
            results?.let {
              it.progress.status in setOf("completed", "completed_empty") &&
                  it.progress.findingCount == 0 &&
                  it.semantic.isEmpty() &&
                  it.performance.all { report ->
                    report.status in setOf("completed", "completed_empty") &&
                        report.findings.isEmpty()
                  } &&
                  it.security.all { report ->
                    report.status in setOf("completed", "completed_empty") &&
                        report.findings.isEmpty()
                  }
            } == true

  fun countLabel(loadedCount: Int): String {
    val count = reportedCount
    if (count == loadedCount &&
        !section.loading &&
        section.error == null &&
        !stale &&
        run?.status in setOf("completed", "completed_empty") &&
        progress?.status in setOf("completed", "completed_empty") &&
        results != null &&
        (loadedCount > 0 || completedZeroDetails))
        return "$loadedCount ${if (loadedCount == 1) "finding" else "findings"}"
    val loaded = if (loadedCount > 0) "$loadedCount loaded · " else ""
    return "$loaded${count?.let { "$it reported" } ?: "— reported (count unavailable)"}"
  }

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
            AnalysisResultAvailability.Error,
            "Results could not be refreshed.",
            section.error.ifBlank { "The saved result read failed without a diagnostic." })
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
                  "$currentReportedCount findings were reported. Analysis is still in progress; view analysis for the current category status.")
          else
              AnalysisResultEmptyPresentation(
                  AnalysisResultAvailability.Running,
                  "Analysis is in progress.",
                  listOfNotNull(
                          currentReportedCount?.let {
                            "$it findings reported so far; results are not final."
                          },
                          coverageLabel)
                      .joinToString(" · "))
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
          if (completedZeroDetails)
              AnalysisResultEmptyPresentation(
                  AnalysisResultAvailability.CompletedEmpty,
                  "No findings in the analyzed scope.",
                  "")
          else
              AnalysisResultEmptyPresentation(
                  AnalysisResultAvailability.PendingDetails,
                  if (currentReportedCount != null)
                      "$currentReportedCount findings were reported; completed details are not available yet."
                  else "Completed result details and reported count are not available yet.",
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
