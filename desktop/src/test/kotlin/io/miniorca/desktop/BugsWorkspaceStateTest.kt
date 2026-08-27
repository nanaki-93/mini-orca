package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class BugsWorkspaceStateTest {
    private val verified = UnifiedFinding(
        id = "vet-1",
        source = "vet",
        confidence = "tool_reported",
        severity = "warning",
        title = "Unchecked error",
        message = "Check the returned error",
        projectRevision = "revision",
        location = FindingLocation("main.go", startLine = 7, symbol = "Run"),
        evidence = "err is ignored",
        status = "open",
        freshness = "fresh",
    )
    private val suggested = UnifiedFinding(
        id = "ai-1",
        source = "analysis",
        confidence = "suggested",
        severity = "info",
        title = "Consider timeout",
        message = "The call may block",
        status = "dismissed",
        freshness = "stale",
    )

    @Test fun filteringAndClassificationKeepVerifiedAndAiFindingsDistinct() {
        val findings = listOf(verified, suggested, suggested.copy(id = "unknown", confidence = "unknown"))

        assertEquals(FindingClassification.Verified, classifyFinding(verified))
        assertEquals(FindingClassification.Suggested, classifyFinding(suggested))
        assertEquals(FindingClassification.Unclassified, classifyFinding(findings.last()))
        assertEquals(listOf(verified), filterFindings(findings, BugsFilters(query = "ignored", severity = "warning")))
        assertEquals(findings.drop(1), filterFindings(findings, BugsFilters(lifecycle = "dismissed", freshness = "stale")))
    }

    @Test fun triageAndPrepareFixRemainRevisionAndLocationSafe() {
        assertEquals(listOf(FindingLifecycleAction("Mark fixed", "fixed"), FindingLifecycleAction("Dismiss", "dismissed")), findingLifecycleActions(verified))
        assertEquals(listOf(FindingLifecycleAction("Reopen", "open")), findingLifecycleActions(suggested))
        assertTrue(findingCanPrepareFix(verified))
        assertFalse(findingCanPrepareFix(suggested))
        assertFalse(findingCanPrepareFix(verified.copy(location = FindingLocation())))

        val index = ProjectIndex("project", "revision", files = listOf(IndexedFile("main.go", "hash", "Go", false)))
        assertEquals(EditorNavigationTarget("main.go", "Run", 7), findingNavigationTarget(verified, index))
        assertEquals(null, findingNavigationTarget(verified.copy(location = FindingLocation("other.go")), index))
    }

    @Test fun findingPresentationLabelsExposeProvenanceLocationLifecycleFreshnessAndRevision() {
        assertTrue(findingProvenanceLabel(verified).contains("VERIFIED / TOOL-REPORTED"))
        assertTrue(findingProvenanceLabel(verified).contains("source vet"))
        assertTrue(findingStatusLabel(verified).contains("open · fresh · revision revision"))
        assertEquals("main.go:7 · Run", findingLocationLabel(verified))
    }

    @Test fun verifiedScanProgressOnlyPollsActiveScansAndShowsWarnings() {
        assertFalse(shouldPollVerifiedScan(null))
        assertTrue(shouldPollVerifiedScan(GoScanReport(status = "running")))
        assertFalse(shouldPollVerifiedScan(GoScanReport(status = "completed")))

        val progress = verifiedScanProgress(GoScanReport(status = "failed", phases = listOf(GoScanPhase("go vet", "failed", output = "vet output"))))
        assertFalse(progress.canCancel)
        assertTrue(progress.summary.contains("completed phase results"))
        assertEquals(listOf("go vet: failed: vet output"), progress.warnings)
    }
}
