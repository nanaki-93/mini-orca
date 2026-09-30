package io.miniorca.desktop

enum class FindingClassification {
  Verified,
  Suggested,
  Unclassified,
}

enum class FindingPriority(val sectionLabel: String) {
  High("HIGH PRIORITY"),
  Medium("MEDIUM PRIORITY"),
  Low("LOW PRIORITY"),
  Other("OTHER PRIORITY"),
}

data class FindingPriorityGroup(
    val priority: FindingPriority,
    val findings: List<UnifiedFinding>,
)

data class FindingLifecycleAction(val label: String, val status: String)

data class VerifiedScanProgress(
    val statusLabel: String,
    val summary: String,
    val action: VerifiedScanAction,
)

/** The scan owner permits one explicit action for its current lifecycle state. */
enum class VerifiedScanAction {
  Start,
  Cancel,
  Waiting,
}

fun classifyFinding(finding: UnifiedFinding): FindingClassification =
    when (finding.confidence.lowercase()) {
      "tool_reported" -> FindingClassification.Verified
      "suggested" -> FindingClassification.Suggested
      else -> FindingClassification.Unclassified
    }

internal fun findingPriority(finding: UnifiedFinding): FindingPriority =
    when (finding.severity.trim().lowercase()) {
      "high" -> FindingPriority.High
      "medium" -> FindingPriority.Medium
      "low" -> FindingPriority.Low
      else -> FindingPriority.Other
    }

internal fun groupFindingsByPriority(findings: List<UnifiedFinding>): List<FindingPriorityGroup> =
    FindingPriority.entries.mapNotNull { priority ->
      val groupedFindings = findings.filter { findingPriority(it) == priority }
      groupedFindings.takeIf { it.isNotEmpty() }?.let { FindingPriorityGroup(priority, it) }
    }

internal fun findingDisplayKey(finding: UnifiedFinding): String =
    listOf(
            finding.id,
            finding.location.path,
            finding.location.startLine.toString(),
            finding.location.symbol,
            finding.title,
        )
        .joinToString(separator = ":")

fun findingLifecycleActions(finding: UnifiedFinding): List<FindingLifecycleAction> =
    when (finding.status.lowercase()) {
      "open" ->
          listOf(
              FindingLifecycleAction("Mark fixed", "fixed"),
              FindingLifecycleAction("Dismiss", "dismissed"))
      "fixed",
      "dismissed" -> listOf(FindingLifecycleAction("Reopen", "open"))
      else -> emptyList()
    }

fun shouldPollVerifiedScan(scan: GoScanReport?): Boolean =
    scan?.status?.lowercase() in setOf("running", "pausing", "canceling")

fun verifiedScanStatusLabel(scan: GoScanReport?): String =
    scan?.status?.takeIf { it.isNotBlank() }?.let(::analysisStatusLabel) ?: "Not run"

/** Lifecycle completion does not attest to the outcome of any individual phase. */
internal fun verifiedScanPhaseSummary(report: GoScanReport): String =
    if (report.phases.isEmpty()) "No phases reported; no check outcome is available."
    else
        "Reported phases: " +
            report.phases.joinToString("; ") { phase ->
              "${phase.name.ifBlank { "Unnamed scan phase" }} — ${phase.state.takeIf { it.isNotBlank() }?.let(::analysisStatusLabel) ?: "State unavailable"}"
            } +
            ". Review command and output for details."

/**
 * Eligibility uses the current project's identity and the last confirmed status, never old evidence
 * alone.
 */
fun verifiedScanProgress(state: DesktopState): VerifiedScanProgress =
    verifiedScanProgress(state.project, state.verifiedScan, state.findings.scan)

internal fun verifiedScanProgress(
    project: ProjectAnalysis?,
    scanState: VerifiedScanState,
    report: GoScanReport?,
): VerifiedScanProgress {
  val read = scanState.read
  val operation = scanState.operation
  val availability =
      when {
        project == null -> "Load a project before running verified Go checks."
        project.projectId.isBlank() || project.projectRevision.isBlank() ->
            "The current project identity or revision is missing. Reopen the project."
        !project.type.equals("go", ignoreCase = true) ->
            if (project.type.isBlank() || project.type.equals("unknown", ignoreCase = true))
                "Project type is unknown; verified checks require a root Go module."
            else "Verified checks require a root Go module; this project is ${project.type}."
        else -> null
      }
  if (availability != null)
      return VerifiedScanProgress("Unavailable", availability, VerifiedScanAction.Waiting)

  return when (operation) {
    VerifiedScanOperation.Starting ->
        VerifiedScanProgress(
            "Starting",
            "Requesting trust and starting verified checks; previous results remain available.",
            VerifiedScanAction.Waiting)
    VerifiedScanOperation.CancellationRequested ->
        VerifiedScanProgress(
            "Cancellation requested",
            "Waiting for a terminal scan status; cancellation is not confirmed yet.",
            VerifiedScanAction.Waiting)
    is VerifiedScanOperation.StartUncertain ->
        VerifiedScanProgress("Start unconfirmed", operation.message, VerifiedScanAction.Waiting)
    is VerifiedScanOperation.CancellationUnconfirmed ->
        VerifiedScanProgress(
            "Cancellation unconfirmed", operation.message, VerifiedScanAction.Waiting)
    is VerifiedScanOperation.Failed ->
        VerifiedScanProgress("Operation failed", operation.message, VerifiedScanAction.Waiting)
    VerifiedScanOperation.Idle ->
        when (read) {
          VerifiedScanRead.Unread ->
              VerifiedScanProgress(
                  "Status unread",
                  "Read scan status before starting checks.",
                  VerifiedScanAction.Waiting)
          VerifiedScanRead.Reading ->
              VerifiedScanProgress(
                  "Reading status", "Checking the current scan status.", VerifiedScanAction.Waiting)
          is VerifiedScanRead.Unavailable ->
              VerifiedScanProgress("Status unavailable", read.message, VerifiedScanAction.Waiting)
          is VerifiedScanRead.PollUnavailable ->
              VerifiedScanProgress(
                  "Live status unavailable", read.message, VerifiedScanAction.Waiting)
          VerifiedScanRead.Absent ->
              VerifiedScanProgress(
                  if (report == null) "Not run" else "No current report",
                  if (report == null)
                      "No verified checks have run. Importing or reindexing never starts them automatically."
                  else
                      "No current scan report was found. Earlier scan evidence remains available; importing or reindexing never starts checks automatically.",
                  VerifiedScanAction.Start)
          VerifiedScanRead.Loaded -> {
            if (report == null ||
                report.projectId != project?.projectId ||
                report.projectRevision != project.projectRevision)
                VerifiedScanProgress(
                    "Status unknown",
                    "The scan report does not match the current project. Refresh status.",
                    VerifiedScanAction.Waiting)
            else
                when (report.status.lowercase()) {
                  "running" ->
                      VerifiedScanProgress(
                          verifiedScanStatusLabel(report),
                          "Verified checks are running in a temporary copied workspace; source remains unchanged.",
                          VerifiedScanAction.Cancel)
                  "pausing",
                  "canceling" ->
                      VerifiedScanProgress(
                          verifiedScanStatusLabel(report),
                          "Verified checks are ${report.status.lowercase()} in a temporary copied workspace; source remains unchanged.",
                          VerifiedScanAction.Waiting)
                  "completed",
                  "failed",
                  "canceled",
                  "cancelled" ->
                      VerifiedScanProgress(
                          verifiedScanStatusLabel(report),
                          "Verified checks ${report.status.lowercase()}; ${verifiedScanPhaseSummary(report)}" +
                              if (report.status.equals("completed", ignoreCase = true))
                                  " Completion is not proof that every phase passed."
                              else "",
                          VerifiedScanAction.Start)
                  else ->
                      VerifiedScanProgress(
                          "Status unknown",
                          "Unrecognized scan lifecycle '${report.status}'; refresh status before another action.",
                          VerifiedScanAction.Waiting)
                }
          }
        }
  }
}

sealed interface FindingPreparationDecision {
  data class Eligible(val target: EditorNavigationTarget, val task: BugTaskSpec) :
      FindingPreparationDecision

  data class Blocked(val reason: String) : FindingPreparationDecision
}

/** Index evidence is a preflight only; the loaded file and symbols must be checked again. */
fun findingPreparationDecision(
    finding: UnifiedFinding,
    project: ProjectAnalysis?,
    findings: List<UnifiedFinding>,
    index: ProjectIndex?,
): FindingPreparationDecision {
  fun blocked(reason: String) = FindingPreparationDecision.Blocked(reason)
  if (project == null ||
      index == null ||
      project.projectId.isBlank() ||
      project.projectRevision.isBlank() ||
      index.projectId != project.projectId ||
      index.projectRevision != project.projectRevision)
      return blocked("Load the current project index before preparing a fix.")
  if (finding.projectId != project.projectId ||
      finding.projectRevision != project.projectRevision ||
      findings.count { it == finding } != 1)
      return blocked("This finding is no longer in the current Bugs results.")
  if (!finding.freshness.equals("fresh", ignoreCase = true))
      return blocked("Analyze again to prepare a fix from current source.")
  val task = finding.taskSpec ?: return blocked("This finding has no reviewed fix task.")
  if (task.schemaVersion != "1" ||
      task.targetPath.isBlank() ||
      task.targetPath != finding.location.path ||
      task.targetSymbol.isBlank() ||
      task.targetSymbol != finding.location.symbol ||
      task.targetSignature.isBlank() ||
      task.acceptanceCriteria.isEmpty() ||
      task.acceptanceCriteria.any { it.isBlank() })
      return blocked(
          "The fix task needs a matching path, declaration, signature and acceptance criteria.")
  val files = index.files.filter { it.path == task.targetPath }
  if (files.size != 1)
      return blocked("The target file is missing or ambiguous in the project index.")
  val file = files.single()
  if (finding.fileHash.isBlank() ||
      file.contentHash.isBlank() ||
      finding.fileHash != file.contentHash)
      return blocked("The finding's file hash no longer matches the indexed source.")
  val declarations = file.symbols.filter { it.name == task.targetSymbol }
  if (declarations.size != 1)
      return blocked("The task must identify one unambiguous indexed declaration.")
  val symbol = declarations.single()
  if (symbol.signature != task.targetSignature)
      return blocked("The task signature does not match the indexed declaration.")
  val eligibility =
      symbolEditEligibility(
          ProjectFileInfo(
              file.path,
              file.contentHash,
              file.path.substringAfterLast('/'),
              language = file.language,
              sizeBytes = file.sizeBytes,
              lineCount = file.lineCount,
              modifiedAt = file.modifiedAt,
              binary = file.binary),
          file.symbols,
          symbol)
  if (!eligibility.eligible) return blocked(eligibility.blockedReason)
  return FindingPreparationDecision.Eligible(EditorNavigationTarget(file.path, symbol.name), task)
}

/** Recheck the preflight target against source returned by both file and symbol reads. */
fun loadedFindingPreparationDecision(
    decision: FindingPreparationDecision.Eligible,
    file: ProjectFileInfo,
    symbols: List<SymbolInfo>,
    expectedHash: String,
): FindingPreparationDecision {
  val task = decision.task
  if (file.path != decision.target.path ||
      expectedHash.isBlank() ||
      file.contentHash != expectedHash)
      return FindingPreparationDecision.Blocked("The loaded source does not match the fix target.")
  val matches = symbols.filter { it.name == task.targetSymbol }
  if (matches.size != 1 || matches.single().signature != task.targetSignature)
      return FindingPreparationDecision.Blocked(
          "The loaded source must contain one exact declaration with the task signature.")
  val eligibility = symbolEditEligibility(file, symbols, matches.single())
  if (!eligibility.eligible) return FindingPreparationDecision.Blocked(eligibility.blockedReason)
  return decision
}

fun findingTaskRequirement(task: BugTaskSpec): String =
    buildString {
          append("Implement the reviewed bug task for ").append(task.targetSymbol).append(".\n")
          append("Target: ").append(task.targetPath).append("\n")
          append("Signature: ").append(task.targetSignature).append("\n\n")
          append("Acceptance criteria:\n")
          task.acceptanceCriteria.forEach { append("- ").append(it).append('\n') }
          append("Non-goals:\n")
          if (task.nonGoals.isEmpty()) append("- None supplied.\n")
          else task.nonGoals.forEach { append("- ").append(it).append('\n') }
          task.goTestCandidate?.let { candidate ->
            append("Optional Go test candidate (review only; do not write automatically): ")
                .append(candidate.name)
                .append("\n")
            append(candidate.content).append('\n')
          }
        }
        .trim()

internal fun findingEvidenceSummary(finding: UnifiedFinding): String =
    when (classifyFinding(finding)) {
      FindingClassification.Verified -> "Reported by ${finding.source.ifBlank { "a local tool" }}."
      FindingClassification.Suggested ->
          "Model proposal; validate against source before preparing a change."
      FindingClassification.Unclassified -> "Evidence origin was not classified."
    }

/** Only explicitly classified semantic bugs and tool-reported diagnostics belong on Bugs. */
internal fun DesktopState.projectBugFindings(): List<UnifiedFinding> {
  val page = analysisResultPage("bugs")
  val project = page.project ?: return page.semantic
  val verified =
      findings.findings.filter {
        classifyFinding(it) == FindingClassification.Verified &&
            it.category in setOf("", "bugs") &&
            it.projectId == project.projectId &&
            it.projectRevision == project.projectRevision
      }
  return (page.semantic + verified).distinctBy(::findingDisplayKey)
}
