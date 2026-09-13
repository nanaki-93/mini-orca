package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class SecurityWorkspaceTest {
  private val finding =
      SecurityFinding(
          id = "rule-1",
          title = "Credential-like assignment",
          rule = "credential_literal",
          severity = "high",
          evidenceKind = "rule_match",
          triage = "open",
          verificationState = "unverified",
          anchor = SecuritySourceAnchor("main.go", 2, 2, "Run"),
          observedCondition = "A value matches the deterministic rule.",
          remediation = "Move it out of source.")

  @Test
  fun reportsStaySeparateAndRequireTheCurrentIndexedFileHash() {
    val source = report("deterministic", listOf(finding))
    val ai = report("ai", emptyList(), status = "completed_empty")
    val state =
        DesktopState()
            .reduce(DesktopEvent.SecurityReportLoaded(source))
            .reduce(DesktopEvent.SecurityReportLoaded(ai))

    assertEquals(source, state.security.sourceReport)
    assertEquals(ai, state.security.aiReport)
    val index =
        resultIndexFixture().copy(files = listOf(IndexedFile("main.go", "hash", "Go", false)))
    assertTrue(securityReportMatchesIndex(source, index))
    assertFalse(securityReportMatchesIndex(source.copy(contentHash = "next"), index))
    assertTrue(securityReportMatchesIndex(ai, index))
  }

  @Test
  fun navigationIsReadOnlyAndUsesTheRealAnchor() {
    val index =
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
                        lineCount = 2,
                        symbols =
                            listOf(
                                SymbolInfo(
                                    "Run",
                                    "function",
                                    startLine = 1,
                                    endLine = 2,
                                    confidence = "exact",
                                    atomicTarget = true)))))
    assertEquals(
        EditorNavigationTarget("main.go", "Run", 2),
        securityFindingNavigationTarget(finding, index))
    assertNull(
        securityFindingNavigationTarget(
            finding.copy(anchor = finding.anchor.copy(path = "other.go")), index))
  }

  @Test
  fun anchorsMustBelongToTheIndexedFileAndTheExactDeclaration() {
    val indexed =
        IndexedFile(
            "main.go",
            "hash",
            "Go",
            false,
            lineCount = 12,
            symbols =
                listOf(
                    SymbolInfo(
                        "Run",
                        "function",
                        startLine = 3,
                        endLine = 8,
                        confidence = "exact",
                        atomicTarget = true)))
    val declaration = indexed.symbols.single()

    assertTrue(
        securityAnchorWithinDeclaration(
            finding.anchor.copy(startLine = 4, endLine = 6), indexed, declaration))
    assertFalse(securityAnchorIsValid(finding.anchor.copy(endLine = 13), indexed))
    assertFalse(
        securityAnchorWithinDeclaration(finding.anchor.copy(startLine = 2), indexed, declaration))
    assertFalse(
        securityAnchorWithinDeclaration(finding.anchor.copy(endLine = 9), indexed, declaration))
    assertFalse(securityAnchorIsValid(finding.anchor.copy(symbol = "Missing"), indexed))
    assertFalse(securityAnchorIsValid(finding.anchor.copy(startLine = 2), indexed))
    assertFalse(securityAnchorIsValid(finding.anchor.copy(startLine = 4, endLine = 9), indexed))
  }

  @Test
  fun workspaceEntryAndSelectionOnlyChangeLocalState() {
    val state =
        DesktopState(
                workspace = Workspace.Bugs,
                security = SecurityWorkspaceState(sourceReport = report("deterministic")))
            .reduce(DesktopEvent.WorkspaceSelected(Workspace.Security))
            .reduce(
                DesktopEvent.SymbolSelected(
                    SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))

    assertEquals(Workspace.Security, state.workspace)
    assertEquals("", state.security.action)
    assertEquals("deterministic", state.security.sourceReport?.source)
  }

  @Test
  fun changingFilesCancelsOnlyTheInFlightSecurityActionAndKeepsPriorEvidenceForStalePresentation() {
    val state =
        DesktopState(security = SecurityWorkspaceState(sourceReport = report("deterministic")))
            .reduce(DesktopEvent.SecurityActionStarted("scan"))
            .reduce(DesktopEvent.SecurityActionCanceled)

    assertEquals("", state.security.action)
    assertEquals("deterministic", state.security.sourceReport?.source)
    assertEquals(SecuritySectionOperationStatus.Canceled, state.security.sourceOperation.status)
  }

  @Test
  fun sectionOperationStatesRemainIndependentAndVisibleWithoutReports() {
    val canceled =
        DesktopState()
            .reduce(DesktopEvent.SecurityActionStarted("scan"))
            .reduce(DesktopEvent.SecurityActionCanceled)
    val failed =
        DesktopState()
            .reduce(DesktopEvent.SecurityActionStarted("review"))
            .reduce(DesktopEvent.SecurityActionFailed("review", "provider unavailable"))

    assertEquals(SecuritySectionOperationStatus.Canceled, canceled.security.sourceOperation.status)
    assertEquals(SecuritySectionOperationStatus.Idle, canceled.security.aiOperation.status)
    assertEquals(SecuritySectionOperationStatus.Failed, failed.security.aiOperation.status)
    assertEquals("provider unavailable", failed.security.aiOperation.message)
    assertEquals(SecuritySectionOperationStatus.Idle, failed.security.sourceOperation.status)
  }

  @Test
  fun projectResultsDistinguishRulesAndAiAndDoNotDependOnTheSelectedFile() {
    val page = securityPageFixture()
    val rows = securityResults(page)
    assertEquals(2, rows.size)
    assertTrue(rows.any { it.row().source.startsWith("Source rule") })
    assertTrue(rows.any { it.row().source.startsWith("AI suspicion") })
    assertEquals(2, rows.map { it.row().key }.distinct().size)
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(resultProjectFixture(), resultIndexFixture()),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("security") to page.section)))
    assertTrue(securityFindingIsCurrent(rows.first().finding, state))
    assertTrue(securityFindingCanPrepareFix(rows.first().finding, state.index))
    assertFalse(
        securityFindingIsCurrent(
            rows.first().finding,
            state.copy(
                projectState =
                    state.projectState.copy(index = state.index!!.copy(projectRevision = "next")))))
    assertFalse(
        securityFindingIsCurrent(
            rows.first().finding,
            state.copy(
                analysisRun = state.analysisRun.copy(run = page.run!!.copy(status = "stale")))))
    assertEquals("Stale", page.copy(run = page.run.copy(status = "stale")).statusLabel)
  }

  private fun report(
      source: String,
      findings: List<SecurityFinding> = emptyList(),
      status: String = "completed"
  ) =
      SecurityFileReport(
          "1", "project", "revision", "main.go", "hash", status, source, findings = findings)
}

internal fun securityPageFixture(): AnalysisResultPageState {
  val page = resultPageFixture("security")
  val finding =
      SecurityFinding(
          id = "security",
          title = "Credential-like assignment",
          rule = "credential_literal",
          severity = "high",
          evidenceKind = "rule_match",
          triage = "open",
          verificationState = "unverified",
          anchor = SecuritySourceAnchor("main.go", 4, 4, "Run"),
          observedCondition = "A source rule matched this assignment.",
          remediation = "Move the value out of source.")
  val source =
      SecurityFileReport(
          projectId = "project",
          projectRevision = "revision",
          path = "main.go",
          contentHash = "base",
          status = "partial",
          source = "deterministic",
          findings = listOf(finding),
          reason = "Some rules were unavailable.")
  val ai =
      source.copy(
          source = "ai",
          findings =
              listOf(
                  finding.copy(
                      title = "Review input boundary",
                      evidenceKind = "model_suspicion",
                      confidence = "medium")))
  return page.copy(
      section = page.section.copy(results = page.results!!.copy(security = listOf(source, ai))))
}
