package io.miniorca.desktop

import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ReviewEvidencePaneTest {

  @Test
  fun shortReviewKeepsLongApplyScopeAndEvidenceIndependentlyReachable() {
    val path = "internal/" + "long-package/".repeat(16) + "main.go"
    val current = draft().copy(targetPath = path)
    val state =
        ReviewToolWindowState(
            project(),
            file().copy(path = path),
            null,
            null,
            editableDraft(current),
            current,
            DraftCheckReport(
                path,
                true,
                draftId = current.id,
                draftRevision = current.revision,
                draftHash = current.hash),
            null,
            null,
            null,
            false)
    var applies = 0
    var checks = 0
    ComposeVisualFixture(360, 320, 1.5f) {
          ReviewToolWindow(
              state,
              ReviewToolWindowActions({ checks++ }, { checks++ }, { checks++ }),
              DraftApplicationActions({ applies++ }, { applies++ }))
        }
        .use { fixture ->
          fixture.render()
          assertEquals(0, applies + checks)
          val action = fixture.taggedBounds("review-action-scroll")
          val evidence = fixture.taggedBounds("review-scroll")
          assertTrue(action.height > 0 && action.bottom <= 320f, "$action")
          assertTrue(evidence.height > 0 && evidence.bottom <= action.top, "$evidence vs $action")
          fixture.scrollBy(100_000f, "review-action-scroll")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("review-action-scroll") > 0f)
          fixture.assertTextWrapsWithoutClipping("Updates Run in $path.")
          assertTrue(
              fixture.firstVisibleTextBounds("Updates Run in $path.").bottom <= action.bottom)
          assertTrue(fixture.hasDescription("Apply Run to $path"))
          fixture.revealText("Apply change", "review-action-scroll")
          assertTrue(fixture.requestDescriptionFocus("Apply Run to $path"))
          fixture.render()
          assertTrue(fixture.isDescriptionFocused("Apply Run to $path"))
          assertEquals(0, applies + checks)
          fixture.pressKey(Key.Enter)
          fixture.render()
          assertEquals(1, applies)
          assertEquals(0, checks)
        }
  }

  @Test
  fun shortFailedReviewScrollsToDiagnosticsAndRecoversOnlyOnExplicitAction() {
    val current = draft()
    val diagnostic = "failure /very/long/" + "segment/".repeat(24) + "tail"
    val report =
        DraftCheckReport(
            "main.go",
            true,
            checks = listOf(DraftCheck("go test", true, "failed", output = diagnostic)),
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)
    var edits = 0
    var applies = 0
    ComposeVisualFixture(360, 320, 1.5f) {
          ReviewToolWindow(
              ReviewToolWindowState(
                  project(),
                  file(),
                  null,
                  null,
                  editableDraft(current),
                  current,
                  report,
                  null,
                  null,
                  null,
                  false),
              ReviewToolWindowActions({}, {}, { edits++ }),
              DraftApplicationActions({ applies++ }, { applies++ }))
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("Apply change"))
          fixture.clickText("Failed check details")
          fixture.render()
          val evidence = fixture.taggedBounds("review-scroll")
          fixture.assertTextWrapsWithoutClipping(diagnostic)
          fixture.scrollBy(100_000f, "review-scroll")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("review-scroll") > 0f)
          val diagnosticBounds = fixture.firstVisibleTextBounds(diagnostic)
          assertTrue(
              diagnosticBounds.top < evidence.bottom &&
                  diagnosticBounds.bottom > evidence.top &&
                  diagnosticBounds.bottom <= evidence.bottom,
              "Diagnostic tail must be visible inside review-scroll: $diagnosticBounds vs $evidence")
          assertTrue(evidence.bottom <= fixture.taggedBounds("review-action-scroll").top)
          fixture.revealText("Edit draft", "review-action-scroll")
          assertTrue(fixture.requestFocus("Edit draft"))
          fixture.render()
          assertTrue(fixture.isFocused("Edit draft"))
          assertEquals(0, edits + applies)
          fixture.pressKey(Key.Enter)
          fixture.render()
          assertEquals(1, edits)
          assertEquals(0, applies)
        }
  }

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
  fun evidenceRowsKeepFailureAndRecoveryVisibleWithCheckHelpCollapsed() {
    val current = draft()
    val cases =
        listOf(
            Triple("failed", "At least one focused check failed.", ReviewEvidenceStatus.Failed),
            Triple("canceled", "At least one focused check failed.", ReviewEvidenceStatus.Failed),
            Triple(
                "unknown",
                "Focused check state is unavailable for the latest draft.",
                ReviewEvidenceStatus.Missing),
            Triple(
                "partial",
                "Focused check state is unavailable for the latest draft.",
                ReviewEvidenceStatus.Missing),
            Triple(
                "unavailable",
                "Focused checks could not produce applicable evidence for this draft.",
                ReviewEvidenceStatus.Failed))
    cases.forEach { (checkState, detail, status) ->
      val checks =
          DraftCheckReport(
              "main.go",
              checkState != "unavailable",
              checks = listOf(DraftCheck("go test", true, checkState)),
              draftId = current.id,
              draftRevision = current.revision,
              draftHash = current.hash)
      val evidence =
          reviewEvidenceUiState(project(), file(), editableDraft(current), current, checks)
      val decision =
          applyDecisionUiState(project(), file(), editableDraft(current), current, checks, null)
      assertEquals(status, evidence.checks.status)
      assertFalse(decision.eligible)
      var calls = 0
      ComposeVisualFixture(600, 900, 1.5f) {
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
            assertTrue(fixture.hasText(detail), "$checkState detail must not require hover")
            assertTrue(fixture.hasText(decision.reason), "$checkState blocker must be visible")
            assertEquals(
                status.label, fixture.descriptionStateDescription("Focused checks: $detail"))
            assertFalse(fixture.hasText("Ready to apply"))
            assertFalse(fixture.hasText("Apply change"))
            assertFalse(fixture.hasText("$ go test")) // Optional check details remain collapsed.
            assertTrue(!fixture.tryClick(detail))
            assertEquals(0, calls)
          }
    }
    val stale =
        DraftCheckReport(
            "main.go",
            true,
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = "previous")
    val evidence = reviewEvidenceUiState(project(), file(), editableDraft(current), current, stale)
    assertEquals(ReviewEvidenceStatus.Stale, evidence.checks.status)
    var calls = 0
    ComposeVisualFixture(600, 900, 1.5f) {
          ReviewToolWindow(
              ReviewToolWindowState(
                  project(),
                  file(),
                  null,
                  null,
                  editableDraft(current),
                  current,
                  stale,
                  null,
                  null,
                  null,
                  false),
              ReviewToolWindowActions({ calls++ }, { calls++ }, { calls++ }),
              DraftApplicationActions({ calls++ }, { calls++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Check results no longer match the latest draft."))
          assertTrue(fixture.hasText("Results belong to an earlier candidate."))
          assertEquals(
              "Stale",
              fixture.descriptionStateDescription(
                  "Focused checks: Check results no longer match the latest draft."))
          assertFalse(fixture.hasText("Ready to apply"))
          assertEquals(0, calls)
        }
    ComposeVisualFixture(600, 900, 1.5f) {
          ReviewToolWindow(
              ReviewToolWindowState(
                  project(),
                  file().copy(contentHash = "changed"),
                  null,
                  null,
                  editableDraft(current),
                  current,
                  null,
                  null,
                  null,
                  null,
                  false),
              ReviewToolWindowActions({ calls++ }, { calls++ }, { calls++ }),
              DraftApplicationActions({ calls++ }, { calls++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Source identity"))
          assertTrue(fixture.hasText("The draft no longer matches the open file."))
          assertFalse(fixture.hasText("Source unchanged"))
          assertFalse(fixture.hasText("Ready to apply"))
          assertEquals(0, calls)
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
