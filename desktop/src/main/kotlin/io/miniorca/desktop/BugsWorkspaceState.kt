package io.miniorca.desktop

/** Client-side filters keep the loaded, revision-bound finding set inspectable without a scan. */
data class BugsFilters(
    val query: String = "",
    val source: String = "",
    val severity: String = "",
    val freshness: String = "",
    val lifecycle: String = "",
)

enum class FindingClassification(val sectionLabel: String, val description: String) {
    Verified("VERIFIED / TOOL-REPORTED", "Reported by an isolated parser, vet, or test scan."),
    Suggested("AI SUGGESTIONS", "Model interpretation; review before treating it as a defect."),
    Unclassified("UNCLASSIFIED FINDINGS", "Unexpected finding confidence; it is not presented as verified."),
}

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

fun findingCanPrepareFix(finding: UnifiedFinding): Boolean =
    finding.freshness.lowercase() == "fresh" && finding.location.path.isNotBlank()

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
