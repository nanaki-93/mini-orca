package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
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
    assertTrue(rows.all { it.row().source.isBlank() })
    assertTrue(rows.all { securityResultIsLoaded(it, page) })
    assertTrue(rows.all { it.row().state.isBlank() })
    assertEquals(2, rows.map { it.row().key }.distinct().size)
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(resultProjectFixture(), resultIndexFixture()),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("security") to page.section)))
    assertIs<SecurityPreparationDecision.Eligible>(
        securityPreparationDecision(rows.first(), state.index))
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

  @Test
  fun summaryAndSecurityResultDoNotTreatMissingSavedDetailsAsAssurance() {
    val page = securityPageFixture()
    val run =
        page.run!!.copy(
            status = "completed",
            sections =
                page.run.sections.map {
                  if (it.category == "security")
                      it.copy(status = "completed_empty", findingCount = 0)
                  else it
                })
    val pending = page.copy(run = run, section = AnalysisSectionState())
    val metric = summaryIssueMetrics(page.project, run, emptyMap())[2]
    assertEquals(0, metric.value)
    assertEquals("Completed · details not confirmed", metric.status)
    assertEquals("0 reported · details not confirmed", metric.detailStatus)
    assertEquals(
        AnalysisResultAvailability.PendingDetails, pending.emptyPresentation(0).availability)
    assertEquals("0 reported", pending.countLabel(0))

    val unknownRun =
        run.copy(
            sections =
                run.sections.map {
                  if (it.category == "security") it.copy(findingCount = null) else it
                })
    val unknown = summaryIssueMetrics(page.project, unknownRun, emptyMap())[2]
    assertNull(unknown.value)
    assertEquals("Count unavailable", unknown.detailStatus)
    assertTrue(
        page
            .copy(run = unknownRun, section = AnalysisSectionState())
            .countLabel(0)
            .contains("count unavailable"))
  }

  @Test
  fun evidenceIdentityRequiresTheValidReportAndFindingSourcePair() {
    val page = securityPageFixture()
    val source = SecurityResult(report("deterministic", listOf(finding)), finding, false, page)
    val modelFinding = finding.copy(evidenceKind = "model_suspicion")
    val model = SecurityResult(report("ai", listOf(modelFinding)), modelFinding, false, page)
    val unexpected =
        SecurityResult(report("deterministic", listOf(modelFinding)), modelFinding, false, page)
    val unavailable = SecurityResult(report("ai", listOf(finding)), finding, false, page)

    assertEquals(SecurityEvidencePresentation.SourceRule, securityEvidencePresentation(source))
    assertEquals(
        "A source rule match identifies a pattern; it does not confirm a vulnerability.",
        securityEvidencePresentation(source).warning)
    assertEquals(SecurityEvidencePresentation.ModelHypothesis, securityEvidencePresentation(model))
    assertTrue(
        securityEvidencePresentation(model).warning.startsWith("Unverified model hypothesis."))
    assertEquals(SecurityEvidencePresentation.Unavailable, securityEvidencePresentation(unexpected))
    assertEquals(
        SecurityEvidencePresentation.Unavailable, securityEvidencePresentation(unavailable))
    assertTrue(
        securityEvidencePresentation(unavailable)
            .warning
            .contains("Do not treat this finding as verified."))
    assertTrue(unexpected.row().source.isBlank())
  }

  @Test
  fun staleAnchorsRemainMarkedForTheExistingFixEligibilityOwner() {
    val page = securityPageFixture()
    val stale =
        securityResults(page.copy(run = requireNotNull(page.run).copy(status = "stale"))).first()

    assertTrue(stale.stale)
    assertBlocked(stale, resultIndexFixture(), "Analyze again")
  }

  @Test
  fun equalFindingValuesKeepDistinctReportOwnersAndExactSummaryTargets() {
    val page = securityPageFixture()
    val source = page.results!!.security.first().copy(findings = listOf(finding))
    val ai = source.copy(source = "ai")
    val loaded =
        page.copy(
            section =
                page.section.copy(results = page.results!!.copy(security = listOf(source, ai))))
    val rows = securityResults(loaded)
    assertEquals(2, rows.size)
    assertEquals(setOf("deterministic", "ai"), rows.map { it.report.source }.toSet())
    assertEquals(2, rows.map { it.rowKey }.toSet().size)
    assertTrue(rows.all { securityResultIsLoaded(it, loaded) })
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(project = loaded.project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = loaded.run,
                    sections = mapOf(AnalysisResultKey("security") to loaded.section)))
    val targets =
        summaryFindingPreview(state).rows.filter {
          it.target.category == AnalysisResultType.Security
        }
    assertEquals(rows.map { it.rowKey }.toSet(), targets.map { it.target.rowKey }.toSet())
    assertTrue(
        targets.all { resolveSummaryTarget(it.target, state) is ExplicitResultTarget.Resolved })
    // A finding-only intent cannot pick one of two equally valued owners (including a stored scan).
    assertFalse(
        securityFindingIsCurrent(
            finding, state.copy(security = SecurityWorkspaceState(sourceReport = source))))
  }

  @Test
  fun invalidAnchorsStayBrowsableButDuplicatedIdentitiesCannotAuthorize() {
    val page = securityPageFixture()
    val source = page.results!!.security.first()
    val missing = finding.copy(id = "missing", anchor = SecuritySourceAnchor())
    val invalid = finding.copy(id = "invalid", anchor = SecuritySourceAnchor("elsewhere.go", 0, -1))
    val loaded =
        page.copy(
            section =
                page.section.copy(
                    results =
                        page.results!!.copy(
                            security = listOf(source.copy(findings = listOf(missing, invalid))))))
    val rows = securityResults(loaded)
    assertEquals(2, rows.size)
    assertEquals(setOf("missing", "invalid"), rows.map { it.finding.id }.toSet())
    assertTrue(rows.all { securityResultIsLoaded(it, loaded) })
    assertTrue(rows.all { it.row().key.isNotBlank() })
    val browserRows = rows.map { it.row() }
    assertEquals(2, filteredResultRows(browserRows, ResultBrowserFilter.All, "").size)
    assertEquals(browserRows.first().key, resultBrowserSelection(null, browserRows))
    assertEquals(
        invalid.id,
        rows
            .single {
              it.rowKey ==
                  filteredResultRows(browserRows, ResultBrowserFilter.All, "elsewhere.go")
                      .single()
                      .key
            }
            .finding
            .id)

    val duplicated = source.copy(findings = listOf(finding, finding))
    val ambiguous =
        loaded.copy(
            section =
                loaded.section.copy(results = loaded.results!!.copy(security = listOf(duplicated))))
    assertEquals(2, securityResults(ambiguous).size)
    assertTrue(securityResults(ambiguous).all { !securityResultIsLoaded(it, ambiguous) })
    val twoReports =
        ambiguous.copy(
            section =
                ambiguous.section.copy(
                    results = ambiguous.results!!.copy(security = listOf(source, source))))
    assertTrue(securityResults(twoReports).all { !securityResultIsLoaded(it, twoReports) })
    val altered = source.copy(reason = "other result with same report identity")
    val conflicting =
        ambiguous.copy(
            section =
                ambiguous.section.copy(
                    results = ambiguous.results!!.copy(security = listOf(source, altered))))
    assertTrue(securityResults(conflicting).all { !securityResultIsLoaded(it, conflicting) })
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(project = twoReports.project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = twoReports.run,
                    sections = mapOf(AnalysisResultKey("security") to twoReports.section)))
    val targets =
        summaryFindingPreview(state).rows.filter {
          it.target.category == AnalysisResultType.Security
        }
    assertEquals(2, targets.size)
    assertTrue(
        targets.all { resolveSummaryTarget(it.target, state) is ExplicitResultTarget.Unavailable })
  }

  @Test
  fun preparationExplainsOwnerFreshnessAndReportStatus() {
    val page = securityPageFixture()
    val index = resultIndexFixture()
    val source = securityResults(page).first()
    assertIs<SecurityPreparationDecision.Eligible>(securityPreparationDecision(source, index))
    val other = page.copy(section = page.section.copy(results = null))
    assertBlocked(source, index, "missing or ambiguous", other)
    val repeated =
        page.copy(
            section =
                page.section.copy(
                    results = page.results!!.copy(security = listOf(source.report, source.report))))
    assertBlocked(securityResults(repeated).first(), index, "missing or ambiguous")
    assertBlocked(source, index.copy(projectRevision = "other"), "current project index")
    assertBlocked(source, null, "current project index")
    val stale = page.copy(run = page.run!!.copy(status = "stale"))
    assertBlocked(securityResults(stale).first(), index, "Analyze again")
    val unsupported =
        page.copy(
            section =
                page.section.copy(
                    results =
                        page.results!!.copy(
                            security = listOf(source.report.copy(status = "failed")))))
    assertBlocked(securityResults(unsupported).single(), index, "completed or partial")
    val altered = source.report.copy(reason = "changed")
    val replaced =
        page.copy(
            section = page.section.copy(results = page.results!!.copy(security = listOf(altered))))
    assertBlocked(source, index, "missing or ambiguous", replaced)
  }

  @Test
  fun preparationExplainsIndexHashRangeAndDeclarationFailures() {
    val page = securityPageFixture()
    val source = securityResults(page).first()
    val index = resultIndexFixture()
    val indexed = index.files.single()
    fun withReport(report: SecurityFileReport): SecurityResult {
      val updated =
          page.copy(
              section = page.section.copy(results = page.results!!.copy(security = listOf(report))))
      return securityResults(updated).single()
    }
    fun withFinding(value: SecurityFinding) =
        withReport(source.report.copy(findings = listOf(value)))
    assertBlocked(source, index.copy(files = emptyList()), "missing from the project index")
    assertBlocked(source, index.copy(files = listOf(indexed, indexed)), "path is duplicated")
    assertBlocked(withReport(source.report.copy(contentHash = "")), index, "hash is missing")
    assertBlocked(
        source, index.copy(files = listOf(indexed.copy(contentHash = ""))), "hash is missing")
    assertBlocked(
        source,
        index.copy(files = listOf(indexed.copy(contentHash = "other"))),
        "no longer matches")
    assertBlocked(
        withFinding(source.finding.copy(anchor = source.finding.anchor.copy(path = "other.go"))),
        index,
        "report's indexed target")
    assertBlocked(
        withFinding(source.finding.copy(anchor = source.finding.anchor.copy(startLine = 0))),
        index,
        "range must be positive")
    assertBlocked(
        withFinding(source.finding.copy(anchor = source.finding.anchor.copy(endLine = 21))),
        index,
        "range must be positive")
    assertBlocked(
        withFinding(source.finding.copy(anchor = source.finding.anchor.copy(endLine = 3))),
        index,
        "range must be positive")
    assertBlocked(
        withFinding(source.finding.copy(anchor = source.finding.anchor.copy(symbol = ""))),
        index,
        "must name")
    assertBlocked(
        source,
        index.copy(files = listOf(indexed.copy(symbols = emptyList()))),
        "declaration is missing")
    assertBlocked(
        source,
        index.copy(files = listOf(indexed.copy(symbols = indexed.symbols + indexed.symbols))),
        "declaration is ambiguous")
    assertBlocked(
        source,
        index.copy(
            files =
                listOf(
                    indexed.copy(symbols = listOf(indexed.symbols.single().copy(startLine = 5))))),
        "within one valid indexed declaration")
    assertBlocked(
        source,
        index.copy(
            files =
                listOf(
                    indexed.copy(symbols = listOf(indexed.symbols.single().copy(endLine = 30))))),
        "valid indexed declaration")
    assertBlocked(source, index.copy(files = listOf(indexed.copy(binary = true))), "Binary files")
    assertBlocked(
        source, index.copy(files = listOf(indexed.copy(language = "Python"))), "Go files only")
    assertBlocked(
        source,
        index.copy(
            files =
                listOf(
                    indexed.copy(
                        symbols = listOf(indexed.symbols.single().copy(confidence = "inferred"))))),
        "exact Go declaration")
    assertBlocked(
        source,
        index.copy(
            files =
                listOf(
                    indexed.copy(
                        symbols = listOf(indexed.symbols.single().copy(atomicTarget = false))))),
        "Multi-function")
    assertBlocked(
        source,
        index.copy(
            files =
                listOf(
                    indexed.copy(
                        symbols = listOf(indexed.symbols.single().copy(kind = "package"))))),
        "Go function")
  }

  @Test
  fun loadedPreparationRequiresTheSameFileDeclarationAndAnchor() {
    val result = securityResults(securityPageFixture()).first()
    val index = resultIndexFixture().files.single()
    val target =
        assertIs<SecurityPreparationDecision.Eligible>(
            securityPreparationDecision(result, resultIndexFixture()))
    val file =
        ProjectFileInfo(
            index.path,
            index.contentHash,
            "main.go",
            language = "Go",
            sizeBytes = 12,
            lineCount = index.lineCount,
            modifiedAt = "",
            binary = false)
    val response =
        SymbolsResponse(target.projectId, target.projectRevision, target.path, index.symbols)
    assertEquals(target, loadedSecurityPreparationDecision(target, file, response))
    fun blocked(fileInfo: ProjectFileInfo, symbolsResponse: SymbolsResponse, expected: String) {
      val reason =
          assertIs<SecurityPreparationDecision.Blocked>(
                  loadedSecurityPreparationDecision(target, fileInfo, symbolsResponse))
              .reason
      assertTrue(reason.contains(expected), reason)
    }
    blocked(file.copy(contentHash = "changed"), response, "hash")
    blocked(file.copy(contentHash = ""), response, "hash")
    blocked(file.copy(path = "other.go"), response, "path")
    blocked(file, response.copy(projectId = "other"), "different project")
    blocked(file, response.copy(projectRevision = "other"), "revision")
    blocked(file, response.copy(path = "other.go"), "file")
    blocked(file.copy(lineCount = 3), response, "anchor")
    blocked(
        file, response.copy(symbols = index.symbols + index.symbols), "exact indexed declaration")
    blocked(file, response.copy(symbols = emptyList()), "exact indexed declaration")
    blocked(
        file,
        response.copy(symbols = listOf(index.symbols.single().copy(signature = "changed"))),
        "exact indexed declaration")
    blocked(file.copy(binary = true), response, "Binary files")
    blocked(file.copy(language = "Python"), response, "Go files only")
  }

  private fun assertBlocked(
      result: SecurityResult,
      index: ProjectIndex?,
      expected: String,
      page: AnalysisResultPageState = result.page,
  ) {
    val reason =
        assertIs<SecurityPreparationDecision.Blocked>(
                securityPreparationDecision(result, index, page))
            .reason
    assertTrue(reason.contains(expected), reason)
  }

  @Test
  fun semanticResultsRetainTheirOwnBrowserAndSummaryIdentity() {
    val page = securityPageFixture()
    val semantic =
        UnifiedFinding(
            id = "semantic",
            category = "security",
            source = "analysis",
            projectId = "project",
            projectRevision = "revision",
            location = FindingLocation("main.go"))
    val loaded =
        page.copy(
            section = page.section.copy(results = page.results!!.copy(semantic = listOf(semantic))))
    val rows = securityResults(loaded).map { it.row() } + loaded.semantic.map(::semanticResultRow)
    assertEquals(3, rows.size)
    assertEquals(3, rows.map { it.key }.distinct().size)
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(project = loaded.project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = loaded.run,
                    sections = mapOf(AnalysisResultKey("security") to loaded.section)))
    val target =
        summaryFindingPreview(state)
            .rows
            .single { it.target.producer is SummaryFindingProducer.Semantic }
            .target
    assertEquals(semanticResultRow(semantic).key, target.rowKey)
    assertTrue(resolveSummaryTarget(target, state) is ExplicitResultTarget.Resolved)
  }

  @Test
  fun storedScanCannotReplaceDisappearingWorkspaceDetails() {
    val page = securityPageFixture()
    val workspaceFinding = page.results!!.security.first().findings.single()
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(resultProjectFixture(), resultIndexFixture()),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("security") to page.section)),
            security =
                SecurityWorkspaceState(
                    sourceReport =
                        report("deterministic", listOf(workspaceFinding))
                            .copy(contentHash = "base")))
    val selected =
        securityResults(state.analysisResultPage("security")).single {
          it.report.source == "deterministic"
        }
    assertTrue(securityResultIsLoaded(selected, state.analysisResultPage("security")))
    assertTrue(securityFindingIsCurrent(workspaceFinding, state))

    val withoutDetails = state.copy(analysisRun = ProjectAnalysisRunState(run = page.run))
    assertFalse(securityResultIsLoaded(selected, withoutDetails.analysisResultPage("security")))
    assertFalse(securityFindingIsCurrent(workspaceFinding, withoutDetails))
    assertFalse(
        securityFindingIsCurrent(
            workspaceFinding, withoutDetails.copy(analysisRun = ProjectAnalysisRunState())))
    assertTrue(securityExplicitScanFindingIsCurrent(workspaceFinding, withoutDetails))
    assertFalse(
        securityFindingIsCurrent(
            workspaceFinding, state.copy(projectState = state.projectState.copy(index = null))))
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
