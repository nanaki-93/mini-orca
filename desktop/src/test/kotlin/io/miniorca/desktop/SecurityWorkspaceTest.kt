package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class SecurityWorkspaceTest {
  private val project =
      ProjectAnalysis(
          "project",
          "revision",
          "project",
          "/tmp/project",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 2,
          summary = "",
          aiStatus = "",
          analyzedAt = "")
  private val file =
      ProjectFileInfo(
          "main.go",
          "hash",
          "main.go",
          language = "Go",
          sizeBytes = 2,
          lineCount = 2,
          modifiedAt = "",
          binary = false)
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
  fun reportsStaySeparateAndBecomeStaleWhenTheSelectedFileChanges() {
    val source = report("deterministic", listOf(finding))
    val ai = report("ai", emptyList(), status = "completed_empty")
    val state =
        DesktopState()
            .reduce(DesktopEvent.SecurityReportLoaded(source))
            .reduce(DesktopEvent.SecurityReportLoaded(ai))

    assertEquals(source, state.security.sourceReport)
    assertEquals(ai, state.security.aiReport)
    assertTrue(securityReportIsCurrent(source, project, file))
    assertFalse(securityReportIsCurrent(source, project, file.copy(contentHash = "next")))
    assertTrue(
        securityReportStateLabel(source, project, file.copy(contentHash = "next"))
            .startsWith("Stale"))
    assertTrue(securityReportStateLabel(ai, project, file).contains("does not prove"))
  }

  @Test
  fun filtersAndNavigationAreReadOnlyAndUseTheRealAnchor() {
    assertEquals(
        emptyList(), filterSecurityFindings(listOf(finding), SecurityFilters(severity = "low")))
    assertEquals(
        listOf(finding),
        filterSecurityFindings(
            listOf(finding), SecurityFilters(query = "credential", triage = "open")))
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
    assertEquals(
        "Completed",
        securityReportStateLabel(
            state.security.sourceReport, project, file, state.security.sourceOperation))
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

    assertEquals(
        "Canceled",
        securityReportStateLabel(null, project, file, canceled.security.sourceOperation))
    assertEquals(
        "Not run", securityReportStateLabel(null, project, file, canceled.security.aiOperation))
    assertEquals(
        "Failed — provider unavailable",
        securityReportStateLabel(null, project, file, failed.security.aiOperation))
    assertEquals(SecuritySectionOperationStatus.Idle, failed.security.sourceOperation.status)
  }

  private fun report(
      source: String,
      findings: List<SecurityFinding> = emptyList(),
      status: String = "completed"
  ) =
      SecurityFileReport(
          "1", "project", "revision", "main.go", "hash", status, source, findings = findings)
}
