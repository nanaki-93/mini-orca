package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class BugsWorkspaceStateTest {

  @Test
  fun verifiedScanDetailsRetainEveryPhaseCommandAndOutputWithoutStartingWork() {
    val report =
        GoScanReport(
            status = "failed",
            phases =
                listOf(
                    GoScanPhase(
                        "go vet",
                        "failed",
                        listOf("go", "vet", "./..."),
                        "vet\u0000 diagnostic",
                        1),
                    GoScanPhase(
                        "go test",
                        "skipped",
                        listOf("go", "test", "./..."),
                        "Skipped after cancellation")))
    var executions = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(emptyList(), report, false),
              BugsWorkspaceActions(
                  FindingActions({}, { _, _ -> }, {}), { executions++ }, { executions++ }))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Command and output")
          fixture.render("bottom-scan-details-800-1.5")
          assertTrue(fixture.hasText("$ go vet ./..."))
          assertTrue(fixture.hasText("vet  diagnostic"))
          assertTrue(fixture.hasText("$ go test ./..."))
          assertTrue(fixture.hasText("Skipped after cancellation"))
          assertEquals(0, executions)
        }
  }

  private val verified =
      UnifiedFinding(
          id = "vet-1",
          source = "vet",
          confidence = "tool_reported",
          severity = "warning",
          title = "Unchecked error",
          message = "Check the returned error",
          projectId = "project",
          projectRevision = "revision",
          fileHash = "hash",
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
  fun bugsPageExcludesGeneralAndOtherCategorySuggestionsButKeepsVerifiedDiagnostics() {
    val page = resultPageFixture("bugs")
    val section =
        page.section.copy(
            results =
                page.results!!.copy(
                    semantic = page.results!!.semantic + suggested.copy(category = "security"),
                    unclassified = listOf(suggested)))
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(project = page.project),
            findings = FindingsState(findings = listOf(verified, suggested)),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run, sections = mapOf(AnalysisResultKey("bugs") to section)))
    assertEquals(setOf("bug", "vet-1"), state.projectBugFindings().map { it.id }.toSet())
    assertEquals(
        listOf(suggested.copy(freshness = "stale")), state.analysisResultPage("bugs").unclassified)
    assertEquals(1, state.analysisResultPage("bugs").reportedCount)
  }

  @Test
  fun verifiedFindingsRequireTheCurrentProjectAndRevisionEvenWithoutARun() {
    val project = resultProjectFixture()
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(project = project),
            findings =
                FindingsState(
                    findings =
                        listOf(
                            verified,
                            verified.copy(id = "foreign-project", projectId = "other"),
                            verified.copy(id = "foreign-revision", projectRevision = "old"),
                            verified.copy(id = "missing-project", projectId = ""),
                            verified.copy(id = "missing-revision", projectRevision = ""))))

    assertEquals(listOf(verified), state.projectBugFindings())
    assertTrue(state.copy(projectState = ProjectWorkspaceState()).projectBugFindings().isEmpty())
    assertTrue(
        state
            .copy(
                projectState =
                    ProjectWorkspaceState(project = project.copy(projectRevision = "next")))
            .projectBugFindings()
            .isEmpty())
  }

  @Test
  fun bugsDatasetDeduplicatesMatchingSemanticAndVerifiedWithoutCollapsingDifferentPaths() {
    val page = resultPageFixture("bugs")
    val semantic = page.results!!.semantic.single()
    val sameIdOtherPath = semantic.copy(location = FindingLocation("other.go"))
    val section =
        page.section.copy(
            results =
                page.results!!.copy(
                    semantic = listOf(semantic, sameIdOtherPath), unclassified = listOf(suggested)))
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(project = page.project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run, sections = mapOf(AnalysisResultKey("bugs") to section)),
            findings =
                FindingsState(
                    findings =
                        listOf(
                            semantic.copy(confidence = "tool_reported"),
                            sameIdOtherPath.copy(confidence = "tool_reported"),
                            verified,
                            verified.copy(
                                location =
                                    FindingLocation("sub/main.go", startLine = 7, symbol = "Run")),
                            verified.copy(id = "foreign", projectId = "other"))))

    assertEquals(
        listOf(
            semantic,
            sameIdOtherPath,
            verified,
            verified.copy(
                location = FindingLocation("sub/main.go", startLine = 7, symbol = "Run"))),
        state.projectBugFindings())
    assertEquals(
        listOf(suggested.copy(freshness = "stale")), state.analysisResultPage("bugs").unclassified)
  }

  @Test
  fun classificationKeepsVerifiedAndAiFindingsDistinct() {
    val findings =
        listOf(verified, suggested, suggested.copy(id = "unknown", confidence = "unknown"))

    assertEquals(FindingClassification.Verified, classifyFinding(verified))
    assertEquals(FindingClassification.Suggested, classifyFinding(suggested))
    assertEquals(FindingClassification.Unclassified, classifyFinding(findings.last()))
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
  fun triageAndExactPreparationKeepDistinctNavigationPolicies() {
    assertEquals(
        listOf(
            FindingLifecycleAction("Mark fixed", "fixed"),
            FindingLifecycleAction("Dismiss", "dismissed")),
        findingLifecycleActions(verified))
    assertEquals(
        listOf(FindingLifecycleAction("Reopen", "open")), findingLifecycleActions(suggested))
    val project = resultProjectFixture()
    val index = eligibleIndex()
    assertEquals(
        EditorNavigationTarget("main.go", "Run", 7), findingNavigationTarget(verified, index))
    assertEquals(
        null, findingNavigationTarget(verified.copy(location = FindingLocation("other.go")), index))
    val decision = findingPreparationDecision(verified, project, listOf(verified), index)
    assertEquals(
        FindingPreparationDecision.Eligible(
            EditorNavigationTarget("main.go", "Run"), requireNotNull(verified.taskSpec)),
        decision)
    val requirement = findingTaskRequirement((decision as FindingPreparationDecision.Eligible).task)
    assertTrue(requirement.contains("Acceptance criteria:\n- Return the error."))
    assertTrue(requirement.contains("Non-goals:\n- Do not edit other files."))
    assertTrue(requirement.contains("review only; do not write automatically"))
  }

  private fun eligibleIndex() =
      ProjectIndex(
          "project",
          "revision",
          files =
              listOf(
                  IndexedFile(
                      "main.go",
                      "hash",
                      "Go",
                      false,
                      symbols =
                          listOf(
                              SymbolInfo(
                                  "Run", "function", "func Run() error", 7, 10, "exact", true)))))

  @Test
  fun preparationRejectsMissingForeignOrOutdatedEvidenceAndHashes() {
    val project = resultProjectFixture()
    val index = eligibleIndex()
    fun reason(
        finding: UnifiedFinding,
        findings: List<UnifiedFinding> = listOf(finding),
        activeProject: ProjectAnalysis? = project,
        activeIndex: ProjectIndex? = index
    ): String =
        (findingPreparationDecision(finding, activeProject, findings, activeIndex)
                as FindingPreparationDecision.Blocked)
            .reason

    assertTrue(reason(verified, emptyList()).contains("no longer"))
    assertTrue(reason(verified, listOf(verified, verified)).contains("no longer"))
    assertTrue(reason(verified.copy(projectId = "other")).contains("no longer"))
    assertTrue(reason(verified.copy(projectRevision = "other")).contains("no longer"))
    assertTrue(
        reason(verified, activeProject = project.copy(projectRevision = "other"))
            .contains("current"))
    assertTrue(
        reason(verified, activeIndex = index.copy(projectRevision = "other")).contains("index"))
    assertTrue(reason(verified, activeIndex = null).contains("index"))
    assertTrue(reason(verified.copy(freshness = "stale")).contains("Analyze again"))
    assertTrue(reason(verified.copy(fileHash = "")).contains("hash"))
    assertTrue(reason(verified.copy(fileHash = "other")).contains("hash"))
    assertTrue(
        reason(
                verified,
                activeIndex =
                    index.copy(files = listOf(index.files.single().copy(contentHash = ""))))
            .contains("hash"))
    assertTrue(reason(verified, activeIndex = index.copy(files = emptyList())).contains("file"))
    assertTrue(
        reason(verified, activeIndex = index.copy(files = index.files + index.files))
            .contains("ambiguous"))
  }

  @Test
  fun preparationRejectsMalformedTasksAndInexactOrUnsupportedDeclarations() {
    val index = eligibleIndex()
    val file = index.files.single()
    val symbol = file.symbols.single()
    fun reason(finding: UnifiedFinding = verified, candidate: IndexedFile = file): String =
        (findingPreparationDecision(
                finding,
                resultProjectFixture(),
                listOf(finding),
                index.copy(files = listOf(candidate)))
                as FindingPreparationDecision.Blocked)
            .reason
    fun task(change: (BugTaskSpec) -> BugTaskSpec) =
        verified.copy(taskSpec = change(requireNotNull(verified.taskSpec)))

    assertTrue(reason(verified.copy(taskSpec = null)).contains("no reviewed"))
    assertTrue(reason(task { it.copy(schemaVersion = "2") }).contains("task"))
    assertTrue(reason(task { it.copy(targetPath = "other.go") }).contains("task"))
    assertTrue(reason(task { it.copy(targetSymbol = "Other") }).contains("task"))
    assertTrue(reason(task { it.copy(targetSignature = "") }).contains("task"))
    assertTrue(reason(task { it.copy(acceptanceCriteria = listOf(" ")) }).contains("task"))
    assertTrue(reason(task { it.copy(targetSignature = "func Run()") }).contains("signature"))
    assertTrue(reason(candidate = file.copy(symbols = emptyList())).contains("declaration"))
    assertTrue(
        reason(candidate = file.copy(symbols = listOf(symbol, symbol))).contains("unambiguous"))
    assertTrue(
        reason(candidate = file.copy(symbols = listOf(symbol.copy(name = "Runner"))))
            .contains("declaration"))
    assertTrue(
        reason(candidate = file.copy(symbols = listOf(symbol.copy(signature = "func Run()"))))
            .contains("signature"))
    assertTrue(
        reason(candidate = file.copy(symbols = listOf(symbol.copy(confidence = "approximate"))))
            .contains("exact"))
    assertTrue(
        reason(candidate = file.copy(symbols = listOf(symbol.copy(atomicTarget = false))))
            .contains("declaration"))
    assertTrue(
        reason(candidate = file.copy(symbols = listOf(symbol.copy(kind = "package"))))
            .contains("function"))
    assertTrue(reason(candidate = file.copy(binary = true)).contains("Binary"))
    assertTrue(reason(candidate = file.copy(language = "Kotlin")).contains("Go"))
  }

  @Test
  fun findingPresentationLabelsExposeProvenanceLocationLifecycleAndFreshness() {
    assertEquals(FindingClassification.Verified, classifyFinding(verified))
    assertTrue(findingEvidenceSummary(verified).contains("vet"))
    assertEquals("Tool report · vet", findingEvidenceIdentity(verified))
    assertEquals("Model suggestion", findingEvidenceIdentity(suggested))
    assertEquals("", findingMaterialStateLabel(verified))
    assertEquals("Stale", findingMaterialStateLabel(verified.copy(freshness = "stale")))
    assertEquals("main.go:7 · Run", findingLocationLabel(verified))
  }

  @Test
  fun verifiedScanProgressOnlyOffersActionsOwnedByTheCurrentLifecycleState() {
    assertFalse(shouldPollVerifiedScan(null))
    assertTrue(verifiedScanProgress(null).summary.contains("never starts them automatically"))
    assertEquals("Not run", verifiedScanProgress(null).statusLabel)
    assertEquals(VerifiedScanAction.Start, verifiedScanProgress(null).action)
    assertTrue(
        verifiedScanProgress(GoScanReport(status = "running"))
            .summary
            .contains("temporary copied workspace; source remains unchanged"))
    assertTrue(shouldPollVerifiedScan(GoScanReport(status = "running")))
    assertEquals(
        VerifiedScanAction.Cancel, verifiedScanProgress(GoScanReport(status = "running")).action)
    assertTrue(shouldPollVerifiedScan(GoScanReport(status = "canceling")))
    assertEquals(
        VerifiedScanAction.Waiting, verifiedScanProgress(GoScanReport(status = "canceling")).action)
    assertTrue(shouldPollVerifiedScan(GoScanReport(status = "pausing")))
    assertEquals(
        VerifiedScanAction.Waiting, verifiedScanProgress(GoScanReport(status = "pausing")).action)
    assertFalse(shouldPollVerifiedScan(GoScanReport(status = "completed")))

    val progress =
        verifiedScanProgress(
            GoScanReport(
                status = "failed",
                phases = listOf(GoScanPhase("go vet", "failed", output = "vet output"))))
    assertEquals(VerifiedScanAction.Start, progress.action)
    assertTrue(progress.summary.contains("results, command and output remain available"))
  }
}
