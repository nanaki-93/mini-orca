package io.miniorca.desktop

/** Client-side filters keep the loaded, revision-bound finding set inspectable without a scan. */
data class BugsFilters(
    val query: String = "",
    val source: String = "",
    val severity: String = "",
    val freshness: String = "",
    val lifecycle: String = "",
)

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

data class VerifiedScanProgress(val summary: String, val canCancel: Boolean, val warnings: List<String>)

fun classifyFinding(finding: UnifiedFinding): FindingClassification = when (finding.confidence.lowercase()) {
    "tool_reported" -> FindingClassification.Verified
    "suggested" -> FindingClassification.Suggested
    else -> FindingClassification.Unclassified
}

fun filterFindings(findings: List<UnifiedFinding>, filters: BugsFilters): List<UnifiedFinding> = findings.filter { finding ->
    matchesFindingQuery(finding, filters.query) &&
        matchesFindingField(finding.source, filters.source) &&
        matchesFindingField(finding.severity, filters.severity) &&
        matchesFindingField(finding.freshness, filters.freshness) &&
        matchesFindingField(finding.status, filters.lifecycle)
}

internal fun findingPriority(finding: UnifiedFinding): FindingPriority = when (finding.severity.trim().lowercase()) {
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

internal fun activeBugsFilters(filters: BugsFilters): List<String> = buildList {
    filters.query.trim().takeIf { it.isNotBlank() }?.let { add("Search") }
    filters.source.trim().takeIf { it.isNotBlank() }?.let { add("Source: $it") }
    filters.severity.trim().takeIf { it.isNotBlank() }?.let { add("Severity: $it") }
    filters.freshness.trim().takeIf { it.isNotBlank() }?.let { add("Freshness: $it") }
    filters.lifecycle.trim().takeIf { it.isNotBlank() }?.let { add("Lifecycle: $it") }
}

fun findingLifecycleActions(finding: UnifiedFinding): List<FindingLifecycleAction> = when (finding.status.lowercase()) {
    "open" -> listOf(FindingLifecycleAction("Mark fixed", "fixed"), FindingLifecycleAction("Dismiss", "dismissed"))
    "fixed", "dismissed" -> listOf(FindingLifecycleAction("Reopen", "open"))
    else -> emptyList()
}

fun shouldPollVerifiedScan(scan: GoScanReport?): Boolean = scan?.status?.lowercase() in setOf("running", "canceling")

fun verifiedScanProgress(scan: GoScanReport?): VerifiedScanProgress = when {
    scan == null -> VerifiedScanProgress("No verified scan has run. Import and reindex never start one automatically.", false, emptyList())
    shouldPollVerifiedScan(scan) -> VerifiedScanProgress("Verified scan ${scan.status.lowercase()} in an isolated copy; imported source remains unchanged.", true, scanWarnings(scan))
    else -> VerifiedScanProgress("Verified scan ${scan.status.lowercase()}; completed phase results remain available.", false, scanWarnings(scan))
}

fun findingCanPrepareFix(finding: UnifiedFinding): Boolean {
    val task = finding.taskSpec ?: return false
    return finding.freshness.lowercase() == "fresh" &&
        task.schemaVersion == "1" &&
        task.targetPath.isNotBlank() && task.targetPath == finding.location.path &&
        task.targetSymbol.isNotBlank() && task.targetSymbol == finding.location.symbol &&
        task.targetSignature.isNotBlank() && task.acceptanceCriteria.isNotEmpty()
}

fun findingTaskNavigationTarget(finding: UnifiedFinding): EditorNavigationTarget? =
    finding.taskSpec?.takeIf { findingCanPrepareFix(finding) }?.let { EditorNavigationTarget(it.targetPath, it.targetSymbol) }

fun findingTaskRequirement(finding: UnifiedFinding): String? {
    val task = finding.taskSpec?.takeIf { findingCanPrepareFix(finding) } ?: return null
    return buildString {
        append("Implement the reviewed bug task for ").append(task.targetSymbol).append(".\n")
        append("Target: ").append(task.targetPath).append("\n")
        append("Signature: ").append(task.targetSignature).append("\n\n")
        append("Acceptance criteria:\n")
        task.acceptanceCriteria.forEach { append("- ").append(it).append('\n') }
        append("Non-goals:\n")
        if (task.nonGoals.isEmpty()) append("- None supplied.\n") else task.nonGoals.forEach { append("- ").append(it).append('\n') }
        task.goTestCandidate?.let { candidate ->
            append("Optional Go test candidate (review only; do not write automatically): ").append(candidate.name).append("\n")
            append(candidate.content).append('\n')
        }
    }.trim()
}

internal fun findingProvenanceLabel(finding: UnifiedFinding): String =
    "${classifyFinding(finding).provenanceLabel} · source ${finding.source.ifBlank { "unknown" }} · confidence ${finding.confidence.ifBlank { "unknown" }}"

internal fun findingStatusLabel(finding: UnifiedFinding): String =
    "${finding.status.ifBlank { "unknown" }} · ${finding.freshness.ifBlank { "unknown" }}"

private fun matchesFindingQuery(finding: UnifiedFinding, query: String): Boolean {
    val normalized = query.trim()
    if (normalized.isBlank()) return true
    return listOf(finding.title, finding.message, finding.evidence, finding.source, finding.rule, finding.location.path, finding.location.symbol)
        .any { value -> value.contains(normalized, ignoreCase = true) }
}

private fun matchesFindingField(value: String, filter: String): Boolean =
    filter.trim().isBlank() || value.equals(filter.trim(), ignoreCase = true)

private fun scanWarnings(scan: GoScanReport): List<String> = scan.phases
    .filter { phase -> phase.state.lowercase() !in setOf("passed", "running", "") }
    .map { phase -> "${phase.name}: ${phase.state}${phase.output.takeIf { it.isNotBlank() }?.let { ": $it" }.orEmpty()}" }
