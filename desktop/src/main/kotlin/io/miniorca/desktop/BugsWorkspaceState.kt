package io.miniorca.desktop

enum class FindingClassification(val provenanceLabel: String) {
  Verified("VERIFIED / TOOL-REPORTED"),
  Suggested("AI SUGGESTIONS"),
  Unclassified("UNCLASSIFIED FINDINGS"),
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
    val summary: String,
    val canCancel: Boolean,
)

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
    scan?.status?.lowercase() in setOf("running", "canceling")

fun verifiedScanProgress(scan: GoScanReport?): VerifiedScanProgress =
    when {
      scan == null ->
          VerifiedScanProgress(
              "No verified scan. Importing or reindexing never starts one automatically.", false)
      shouldPollVerifiedScan(scan) ->
          VerifiedScanProgress(
              "Trusted local execution: verified scan ${scan.status.lowercase()} in a temporary copied workspace; source remains unchanged.",
              true)
      else ->
          VerifiedScanProgress(
              "Verified scan ${scan.status.lowercase()}; results remain available.", false)
    }

fun findingCanPrepareFix(finding: UnifiedFinding): Boolean {
  val task = finding.taskSpec ?: return false
  return finding.freshness.lowercase() == "fresh" &&
      task.schemaVersion == "1" &&
      task.targetPath.isNotBlank() &&
      task.targetPath == finding.location.path &&
      task.targetSymbol.isNotBlank() &&
      task.targetSymbol == finding.location.symbol &&
      task.targetSignature.isNotBlank() &&
      task.acceptanceCriteria.isNotEmpty()
}

fun findingTaskNavigationTarget(finding: UnifiedFinding): EditorNavigationTarget? =
    finding.taskSpec
        ?.takeIf { findingCanPrepareFix(finding) }
        ?.let { EditorNavigationTarget(it.targetPath, it.targetSymbol) }

fun findingTaskRequirement(finding: UnifiedFinding): String? {
  val task = finding.taskSpec?.takeIf { findingCanPrepareFix(finding) } ?: return null
  return buildString {
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
}

internal fun findingProvenanceLabel(finding: UnifiedFinding): String =
    "${classifyFinding(finding).provenanceLabel} · source ${finding.source.ifBlank { "unknown" }} · confidence ${finding.confidence.ifBlank { "unknown" }}"

internal fun findingStatusLabel(finding: UnifiedFinding): String =
    "${finding.status.ifBlank { "unknown" }} · ${finding.freshness.ifBlank { "unknown" }}"

/** Only explicitly classified semantic bugs and tool-reported diagnostics belong on Bugs. */
internal fun DesktopState.projectBugFindings(): List<UnifiedFinding> {
  val page = analysisResultPage("bugs")
  val verified =
      findings.findings.filter {
        classifyFinding(it) == FindingClassification.Verified && it.category in setOf("", "bugs")
      }
  return (page.semantic + verified).distinctBy(::findingDisplayKey)
}
