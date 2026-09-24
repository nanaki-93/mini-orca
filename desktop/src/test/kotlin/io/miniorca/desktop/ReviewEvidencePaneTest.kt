package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ReviewEvidencePaneTest {

  @Test
  fun detailedChecksStayReachableForFailedSkippedAndStaleEvidenceWithoutExecuting() {
    listOf("failed", "skipped", "stale").forEach { state ->
      val current = draft()
      val output = "compiler\u0000 diagnostic\n" + "details ".repeat(700)
      val checks =
          DraftCheckReport(
              "main.go",
              true,
              checks =
                  listOf(
                      DraftCheck(
                          "go test",
                          true,
                          if (state == "stale") "passed" else state,
                          command = listOf("go", "test", "./..."),
                          output = output)),
              draftId = current.id,
              draftRevision = current.revision,
              draftHash = if (state == "stale") "old" else current.hash)
      var calls = 0
      ComposeVisualFixture(360, 900, 1.5f) {
            ReviewToolWindow(
                ReviewToolWindowState(
                    project(),
                    file(),
                    null,
                    null,
                    editableDraft(current),
                    current,
                    checks,
                    null,
                    null,
                    null,
                    false),
                ReviewToolWindowActions({ calls++ }, { calls++ }, { calls++ }),
                DraftApplicationActions({ calls++ }, { calls++ }))
          }
          .use { fixture ->
            fixture.render()
            fixture.clickText(if (state == "failed") "Failed check details" else "Check details")
            fixture.render("bottom-review-$state-360-1.5")
            assertTrue(fixture.hasText("$ go test ./..."))
            assertTrue(fixture.hasText(output.replace('\u0000', ' ').take(4_096)))
            assertTrue(fixture.hasText("… output truncated"))
            assertFalse(fixture.hasText(sanitizedOutputText(output)))
            assertFalse(fixture.hasText(output))
            assertTrue(fixture.hasText("Show full available output"))
            fixture.clickDescription("Expand available diagnostic output")
            fixture.render()
            assertTrue(fixture.hasText(output.replace('\u0000', ' ').trim()))
            assertFalse(fixture.hasText(sanitizedOutputText(output)))
            assertEquals(0, calls)
          }
    }
  }

  @Test
  fun diagnosticSanitizationPreservesLinesAndTabsWhileBoundingRecordedOutput() {
    assertEquals("line one\n\tline two", sanitizedOutputText("line one\n\tline two\u0000"))
    assertEquals("01234\n… output truncated", sanitizedOutputText("0123456789", 5))
    assertEquals(
        4_096 + "\n… output truncated".length, sanitizedOutputText("x".repeat(5_000)).length)
    assertEquals("01234", sanitizedOutputText("01234", 5))
    assertEquals("", sanitizedOutputText("\u0000\u0001"))
    val failed =
        DraftCheckReport(
            "main.go",
            false,
            checks = listOf(DraftCheck("compile", true, "failed", output = "bad\u0000 token")))
    assertEquals("compile: bad token", checkFailurePreview(failed))
  }

  @Test
  fun validatedButUncheckedDraftStaysInReviewWithAnExplicitCheckAction() {
    val evidence =
        reviewEvidenceUiState(project(), file(), editableDraft(draft()), draft(), checks = null)
    val decision =
        applyDecisionUiState(project(), file(), editableDraft(draft()), draft(), null, null)
    val next =
        reviewNextActionUiState(evidence, decision, draft(), null, null, checksRunning = false)

    assertEquals(ReviewEvidenceStatus.Passed, evidence.validation.status)
    assertEquals(ReviewEvidenceStatus.Missing, evidence.checks.status)
    assertTrue(evidence.canRunChecks)
    assertEquals(ReviewNextActionKind.RunChecks, next.kind)
    assertEquals("Run", next.scope.substringBefore(" in "))
  }

  @Test
  fun readyApplyIsTheOnlyPrimaryProgressActionAndFailedChecksKeepAQuickErrorPreview() {
    val current = draft()
    val passed =
        DraftCheckReport(
            "main.go",
            true,
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)
    val readyEvidence =
        reviewEvidenceUiState(project(), file(), editableDraft(current), current, passed)
    val readyDecision =
        applyDecisionUiState(project(), file(), editableDraft(current), current, passed, null)

    assertEquals(
        ReviewNextActionKind.Apply,
        reviewNextActionUiState(readyEvidence, readyDecision, current, passed, null, false).kind)

    val failed =
        passed.copy(
            checks =
                listOf(
                    DraftCheck(
                        "go test",
                        required = true,
                        state = "failed",
                        command = listOf("go", "test", "./..."),
                        output = "--- FAIL: TestRun expected 200")))
    assertEquals("go test: --- FAIL: TestRun expected 200", checkFailurePreview(failed))
  }

  @Test
  fun progressionKeepsTheRequestToReviewOrderAndUsesTheGuardedDecisionForReview() {
    val current = draft()
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)
    val evidence = reviewEvidenceUiState(project(), file(), editableDraft(current), current, checks)
    val decision =
        applyDecisionUiState(project(), file(), editableDraft(current), current, checks, null)
    val session =
        ChatSession(
            "session",
            current.projectId,
            current.projectRevision,
            current.baseFileHash,
            current.targetPath,
            current.mode,
            current.targetSymbol,
            latestDraftId = current.id)

    val progression = reviewProgressionRows(session, current, evidence, decision)

    assertEquals(
        listOf("Request", "Draft", "Validation", "Focused checks", "Review"),
        progression.map { it.label })
    assertEquals(ReviewEvidenceStatus.Passed, progression.last().status)
  }

  @Test
  fun staleOrFailedCheckEvidenceCannotEnableApply() {
    val current = draft()
    val stale =
        DraftCheckReport(
            "main.go",
            true,
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = "old-hash")
    val failed =
        DraftCheckReport(
            "main.go",
            true,
            checks = listOf(DraftCheck("go test", required = true, state = "failed")),
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)

    val staleEvidence =
        reviewEvidenceUiState(project(), file(), editableDraft(current), current, stale)
    val failedEvidence =
        reviewEvidenceUiState(project(), file(), editableDraft(current), current, failed)

    assertEquals(ReviewEvidenceStatus.Stale, staleEvidence.checks.status)
    assertEquals(ReviewEvidenceStatus.Failed, failedEvidence.checks.status)
  }

  @Test
  fun verificationAndReceiptsKeepIdentityGuardsInternal() {
    val current = draft()
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)
    val evidence = reviewEvidenceUiState(project(), file(), editableDraft(current), current, checks)
    val receipt =
        applyDecisionUiState(
            project(),
            file(),
            editableDraft(current),
            current,
            checks,
            ApplyResult(
                "revision", "post-hash", true, AuditEntry("apply", "main.go", "applied", "")))

    assertFalse(evidence.identity.detail.contains("revision"))
    assertFalse(evidence.identity.detail.contains("hash"))
    assertFalse(evidence.checks.detail.contains("revision"))
    assertFalse(evidence.checks.detail.contains("hash"))
    assertEquals("main.go was updated.", receipt.receiptDetail)
  }

  @Test
  fun aManualEditInvalidatesCurrentCheckEvidenceAndReturnsTheUserToDraftRecovery() {
    val current = draft()
    val matchingChecks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)
    val edited = editDraft(editableDraft(current), declaration = "func Run() error { return nil }")
    val evidence =
        reviewEvidenceUiState(
            project(), file(), edited, current.copy(validation = null), matchingChecks)

    assertEquals(ReviewEvidenceStatus.Missing, evidence.validation.status)
    assertFalse(evidence.canRunChecks)
    assertTrue(reviewValidationSummary(edited, validationCurrent = false).contains("Manual edits"))
  }

  @Test
  fun diagnosticsAndReadOnlyImpactAndGitContextRemainExplicit() {
    val invalid =
        draft()
            .copy(
                validation =
                    DeclarationValidation(
                        false,
                        "replace_symbol",
                        diagnostics =
                            listOf(DeclarationFinding("scope", "Only one declaration may change.")),
                        diff = UnifiedDiff("main.go", "main.go")))
    val evidence =
        reviewEvidenceUiState(project(), file(), editableDraft(invalid), invalid, checks = null)
    val impact =
        ImpactPreview(
            "main.go", "Run", listOf(ImpactReference("main_test.go", "Run", "high", "calls Run")))

    assertEquals(ReviewEvidenceStatus.Failed, evidence.validation.status)
    assertEquals(
        "Only one declaration may change.", invalid.validation!!.diagnostics.single().message)
    assertTrue(advisoryImpactLabel(impact).contains("read-only"))
    assertTrue(
        gitContextLabel(GitStatus(true, "main", "modified", "changed")).contains("read-only"))
    assertTrue(gitContextLabel(null).contains("read-only"))
  }

  @Test
  fun taskBoundFailureCreatesABoundedExplicitRepairMessageUntilTheLimit() {
    val task = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Return an error."))
    val current = draft().copy(taskSpec = task)
    val session = ChatSession(id = "session", taskSpec = task, repairCount = 2)
    val checks =
        DraftCheckReport(
            "main.go",
            false,
            listOf(
                DraftCheck(
                    "task test candidate",
                    required = true,
                    state = "failed",
                    output = "<workspace>/main_test.go: assertion failed")),
            current.id,
            current.revision,
            current.hash)

    val repair = repairMessageForChecks(session, current, checks)

    assertTrue(repair?.contains("sanitized focused check evidence") == true)
    assertTrue(repair.contains("assertion failed"))
    assertTrue(repair.length < 4096)
    assertEquals(null, repairMessageForChecks(session.copy(repairCount = 3), current, checks))
    assertFalse(
        checksMatchDraft(
            DraftCheckReport(
                "main.go",
                false,
                draftId = current.id,
                draftRevision = current.revision,
                draftHash = "stale"),
            current))
  }

  @Test
  fun editorProgressionRetainsMissingFailedRunningAndStaleEvidence() {
    val ready = editorComparisonReviewFixture()
    assertTrue(editorProgressionRows(ready).all { it.status == ReviewEvidenceStatus.Passed })
    val missing = editorProgressionRows(ready.copy(checks = null))
    assertEquals(ReviewEvidenceStatus.Missing, missing[3].status)
    assertEquals(ReviewEvidenceStatus.Missing, missing[4].status)
    val failed =
        editorProgressionRows(
            ready.copy(
                checks =
                    ready.checks!!.copy(
                        checks = listOf(DraftCheck("go test", required = true, state = "failed")))))
    assertEquals(ReviewEvidenceStatus.Failed, failed[3].status)
    assertEquals(ReviewEvidenceStatus.Failed, failed[4].status)
    val running = editorProgressionRows(ready.copy(checksRunning = true, checks = null))
    assertEquals(ReviewEvidenceStatus.Running, running[3].status)
    assertEquals(ReviewEvidenceStatus.Running, running[4].status)
    val stale =
        editorProgressionRows(ready.copy(selected = ready.selected!!.copy(contentHash = "changed")))
    assertEquals(ReviewEvidenceStatus.Stale, stale[1].status)
    assertTrue(stale.last().status != ReviewEvidenceStatus.Passed)
    val edited =
        editorProgressionRows(
            ready.copy(editor = editDraft(ready.editor!!, "func GetUser() {}", emptyList())))
    assertTrue(edited[2].status != ReviewEvidenceStatus.Passed)
    assertTrue(edited.last().status != ReviewEvidenceStatus.Passed)
  }

  @Test
  fun aRerunCannotAdvertiseReadyOrApplyUsingThePreviousPassingReport() {
    val state = editorComparisonReviewFixture()
    val evidence =
        reviewEvidenceUiState(
            state.project, state.selected, state.editor, state.draft, state.checks, true)
    val decision =
        applyDecisionUiState(
            state.project, state.selected, state.editor, state.draft, state.checks, null)
    assertTrue(decision.eligible)
    assertEquals("Checks running", reviewReadinessTitle(evidence, decision))
    val next =
        reviewNextActionUiState(evidence, decision, state.draft, state.checks, state.session, true)
    assertEquals(ReviewNextActionKind.Waiting, next.kind)
    assertFalse(next.enabled)
    assertEquals(
        ReviewEvidenceStatus.Running,
        editorProgressionRows(state.copy(checksRunning = true)).last().status)
  }

  @Test
  fun requiredCheckCountsNeverPromoteMissingStaleSkippedOrOptionalEvidence() {
    val state = editorComparisonReviewFixture()
    assertEquals(null, requiredChecksSummary(null, state.draft, false))
    assertEquals(
        "1 of 1 required check passed", requiredChecksSummary(state.checks, state.draft, false))
    assertEquals(
        "Results belong to an earlier candidate.",
        requiredChecksSummary(state.checks!!.copy(draftHash = "previous"), state.draft, false))
    assertEquals(
        "0 of 1 required check passed",
        requiredChecksSummary(
            state.checks.copy(
                checks =
                    listOf(
                        DraftCheck("optional", false, "passed"),
                        DraftCheck("required", true, "skipped"))),
            state.draft,
            false))
    assertEquals(
        "No required checks reported.",
        requiredChecksSummary(state.checks.copy(checks = emptyList()), state.draft, false))
    assertEquals(
        ReviewEvidenceStatus.Missing,
        reviewEvidenceUiState(null, null, null, null, null).identity.status)
  }

  private fun draft() =
      DeclarationDraft(
          "draft",
          "project",
          "revision",
          "base",
          "main.go",
          "replace_symbol",
          "Run",
          "func Run() {}",
          revision = 2,
          hash = "draft-hash",
          validation =
              DeclarationValidation(
                  true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")),
      )

  private fun project() =
      ProjectAnalysis(
          "project",
          "revision",
          "project",
          "/tmp/project",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 1,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun file() =
      ProjectFileInfo(
          "main.go",
          "base",
          "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)
}
