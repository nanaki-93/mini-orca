package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class BugsWorkspaceStateTest {
  private val verified =
      UnifiedFinding(
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
          taskSpec =
              BugTaskSpec(
                  schemaVersion = "1",
                  targetPath = "main.go",
                  targetSymbol = "Run",
                  targetSignature = "func Run() error",
                  acceptanceCriteria = listOf("Return the error."),
                  nonGoals = listOf("Do not edit other files."),
                  goTestCandidate =
                      GoTestCandidateSpec("TestRun", "package main\nfunc TestRun() {}"),
              ),
      )
  private val suggested =
      UnifiedFinding(
          id = "ai-1",
          source = "analysis",
          confidence = "suggested",
          severity = "info",
          title = "Consider timeout",
          message = "The call may block",
          status = "dismissed",
          freshness = "stale",
      )

  @Test
  fun filteringAndClassificationKeepVerifiedAndAiFindingsDistinct() {
    val findings =
        listOf(verified, suggested, suggested.copy(id = "unknown", confidence = "unknown"))

    assertEquals(FindingClassification.Verified, classifyFinding(verified))
    assertEquals(FindingClassification.Suggested, classifyFinding(suggested))
    assertEquals(FindingClassification.Unclassified, classifyFinding(findings.last()))
    assertEquals(
        listOf(verified),
        filterFindings(findings, BugsFilters(query = "ignored", severity = "warning")))
    assertEquals(
        findings.drop(1),
        filterFindings(findings, BugsFilters(lifecycle = "dismissed", freshness = "stale")))
  }

  @Test
  fun priorityGroupsNormalizeSeverityAndKeepBackendOrderWithinEachSection() {
    val low = verified.copy(id = "low", severity = " low ")
    val unknown = suggested.copy(id = "unknown", severity = "critical")
    val medium = suggested.copy(id = "medium", severity = "MEDIUM")
    val highFirst = verified.copy(id = "high-1", severity = "high")
    val highSecond = suggested.copy(id = "high-2", severity = " HIGH ")
    val blank = suggested.copy(id = "blank", severity = " ")

    val groups = groupFindingsByPriority(listOf(low, unknown, medium, highFirst, highSecond, blank))

    assertEquals(
        listOf(
            FindingPriority.High,
            FindingPriority.Medium,
            FindingPriority.Low,
            FindingPriority.Other),
        groups.map { it.priority })
    assertEquals(listOf(highFirst, highSecond), groups[0].findings)
    assertEquals(listOf(medium), groups[1].findings)
    assertEquals(listOf(low), groups[2].findings)
    assertEquals(listOf(unknown, blank), groups[3].findings)
  }

  @Test
  fun filtersApplyBeforePriorityGroupingWithoutEmptySections() {
    val highVerified = verified.copy(id = "high-verified", severity = "high")
    val highSuggested = suggested.copy(id = "high-suggested", severity = "high")
    val filtered =
        filterFindings(
            listOf(highSuggested, highVerified, suggested.copy(id = "low", severity = "low")),
            BugsFilters(source = "vet"))
    val groups = groupFindingsByPriority(filtered)

    assertEquals(listOf(highVerified), filtered)
    assertEquals(listOf(FindingPriority.High), groups.map { it.priority })
    assertEquals(listOf(highVerified), groups.single().findings)
  }

  @Test
  fun activeFiltersStayVisibleWhenAdvancedControlsAreCollapsed() {
    assertEquals(
        listOf("Search", "Source: vet", "Lifecycle: open"),
        activeBugsFilters(BugsFilters(query = "error", source = "vet", lifecycle = "open")),
    )
    assertTrue(activeBugsFilters(BugsFilters()).isEmpty())
  }

  @Test
  fun detailsSelectionUsesTheVisibleSharedFindingPresentation() {
    val presentation =
        findingsPresentation(listOf(verified, suggested), BugsFilters(source = "vet"))

    assertEquals(verified, visibleFindingByDisplayKey(presentation, findingDisplayKey(verified)))
    assertEquals(null, visibleFindingByDisplayKey(presentation, findingDisplayKey(suggested)))
    assertEquals(null, visibleFindingByDisplayKey(presentation, null))
  }

  @Test
  fun compactEmptyMessagesRetainFilterContextAndVerifiedScanBoundary() {
    assertEquals("No findings yet.", findingsPresentation(emptyList(), BugsFilters()).emptyMessage)
    assertEquals(
        "No findings match these filters.",
        findingsPresentation(emptyList(), BugsFilters(source = "vet")).emptyMessage)
    assertTrue(verifiedScanProgress(null).summary.contains("never starts one automatically"))
    assertTrue(
        verifiedScanProgress(GoScanReport(status = "running"))
            .summary
            .contains("isolated copy; source remains unchanged"))
  }

  @Test
  fun triageAndPrepareFixRemainRevisionAndLocationSafe() {
    assertEquals(
        listOf(
            FindingLifecycleAction("Mark fixed", "fixed"),
            FindingLifecycleAction("Dismiss", "dismissed")),
        findingLifecycleActions(verified))
    assertEquals(
        listOf(FindingLifecycleAction("Reopen", "open")), findingLifecycleActions(suggested))
    assertTrue(findingCanPrepareFix(verified))
    assertFalse(findingCanPrepareFix(suggested))
    assertFalse(findingCanPrepareFix(verified.copy(location = FindingLocation())))
    assertFalse(findingCanPrepareFix(verified.copy(freshness = "stale")))
    assertFalse(
        findingCanPrepareFix(
            verified.copy(taskSpec = verified.taskSpec?.copy(targetSignature = ""))))

    val index =
        ProjectIndex(
            "project", "revision", files = listOf(IndexedFile("main.go", "hash", "Go", false)))
    assertEquals(
        EditorNavigationTarget("main.go", "Run", 7), findingNavigationTarget(verified, index))
    assertEquals(
        null, findingNavigationTarget(verified.copy(location = FindingLocation("other.go")), index))
    assertEquals(EditorNavigationTarget("main.go", "Run"), findingTaskNavigationTarget(verified))
    val requirement = requireNotNull(findingTaskRequirement(verified))
    assertTrue(requirement.contains("Acceptance criteria:"))
    assertTrue(requirement.contains("Non-goals:"))
    assertTrue(requirement.contains("review only"))
  }

  @Test
  fun findingPresentationLabelsExposeProvenanceLocationLifecycleAndFreshness() {
    assertTrue(findingProvenanceLabel(verified).contains("VERIFIED / TOOL-REPORTED"))
    assertTrue(findingProvenanceLabel(verified).contains("source vet"))
    assertEquals("open · fresh", findingStatusLabel(verified))
    assertFalse(findingStatusLabel(verified).contains("revision"))
    assertEquals("main.go:7 · Run", findingLocationLabel(verified))
  }

  @Test
  fun verifiedScanProgressOnlyPollsActiveScansAndShowsWarnings() {
    assertFalse(shouldPollVerifiedScan(null))
    assertTrue(shouldPollVerifiedScan(GoScanReport(status = "running")))
    assertFalse(shouldPollVerifiedScan(GoScanReport(status = "completed")))

    val progress =
        verifiedScanProgress(
            GoScanReport(
                status = "failed",
                phases = listOf(GoScanPhase("go vet", "failed", output = "vet output"))))
    assertFalse(progress.canCancel)
    assertTrue(progress.summary.contains("results remain available"))
    assertEquals(listOf("go vet: failed: vet output"), progress.warnings)
  }
}
