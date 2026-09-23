package io.miniorca.desktop

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ReviewToolWindowTest {
  @Test
  fun taskTestTrustScopeShowsTheExactGeneratedTestCommand() {
    val current =
        draft().copy(taskSpec = BugTaskSpec(goTestCandidate = GoTestCandidateSpec("TestRun")))

    assertEquals("go test ./... -run ^TestRun$", draftProjectCodeCommand(current))
    assertEquals("go test ./...", draftProjectCodeCommand(draft()))
  }

  @Test
  fun repairAcceptsAParentProofOnlyOnceAndRejectsReplacement() {
    val sessionTask =
        BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Return the expected value."))
    val proof = GoTestCandidateSpec("TestRun", "package main\nfunc TestRun() {}")
    val parentTask = sessionTask.copy(goTestCandidate = proof)
    val current = draft().copy(taskSpec = parentTask)
    val session = ChatSession(taskSpec = sessionTask)
    val checks =
        DraftCheckReport(
            "main.go",
            false,
            checks =
                listOf(DraftCheck("task test verification", true, "failed", output = "failed")),
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)

    assertTrue(repairMessageForChecks(session, current, checks)?.contains("failed") == true)
    assertTrue(repairTaskSpecMatches(sessionTask, parentTask))
    assertFalse(
        repairTaskSpecMatches(
            parentTask,
            parentTask.copy(
                goTestCandidate =
                    GoTestCandidateSpec(
                        "TestReplacement", "package main\nfunc TestReplacement() {}"))))
  }

  @Test
  fun eligibleDecisionNamesExactlyOneSymbolAndFile() {
    val current = draft()
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)

    val decision =
        applyDecisionUiState(
            project(), file(), editableDraft(current), current, checks, applied = null)

    assertTrue(decision.eligible)
    assertEquals("Apply Run to main.go", decision.actionLabel)
    assertTrue(decision.reason.contains("Ready"))
  }

  @Test
  fun dirtyInvalidOrUncheckedDraftRemainsBlockedWithARecoveryReason() {
    val current = draft()
    val dirty = editDraft(editableDraft(current), declaration = "func Run() error { return nil }")
    val noChecks =
        applyDecisionUiState(
            project(),
            file(),
            dirty,
            current.copy(validation = null),
            checks = null,
            applied = null)
    val invalid =
        current.copy(
            validation =
                DeclarationValidation(
                    false, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
    val failedChecks =
        DraftCheckReport(
            "main.go",
            true,
            checks = listOf(DraftCheck("go test", required = true, state = "failed")),
            draftId = current.id,
            draftRevision = current.revision,
            draftHash = current.hash)
    val failed =
        applyDecisionUiState(
            project(), file(), editableDraft(invalid), invalid, failedChecks, applied = null)

    assertFalse(noChecks.eligible)
    assertTrue(noChecks.reason.contains("Manual edits"))
    assertFalse(failed.eligible)
    assertTrue(failed.reason.contains("diagnostics"))
  }

  @Test
  fun receiptUsesHumanReadableTargetAndOnlyOffersReturnedUndo() {
    val applied =
        applyDecisionUiState(
            project(), file(), null, null, null, ApplyResult("next", "after", true))
    val undone =
        applyDecisionUiState(
            project(),
            file(),
            null,
            null,
            null,
            ApplyResult(
                "restored", "base", false, audit = AuditEntry("undo", "main.go", "applied", "now")))

    assertEquals("Change applied", applied.receiptTitle)
    assertEquals("Selected file was updated.", applied.receiptDetail)
    assertEquals("Undo this change", applied.undoLabel)
    assertEquals("Change undone", undone.receiptTitle)
    assertEquals("Undo is no longer available", undone.undoLabel)
  }

  @Test
  fun readyActionStaysAnchoredAndOnlyExplicitApplyAndUndoMutate() {
    var state by mutableStateOf(editorComparisonReviewFixture())
    var applies = 0
    var undoes = 0
    var otherActions = 0
    ComposeVisualFixture(360, 850) {
          ReviewToolWindow(
              state,
              ReviewToolWindowActions({ otherActions++ }, { otherActions++ }, { otherActions++ }),
              DraftApplicationActions(
                  {
                    applies++
                    state =
                        state.copy(
                            applied =
                                ApplyResult(
                                    "next",
                                    "after",
                                    true,
                                    AuditEntry(
                                        "apply", state.draft!!.targetPath, "applied", "now")))
                  },
                  {
                    undoes++
                    state =
                        state.copy(
                            applied =
                                ApplyResult(
                                    "restored",
                                    "base",
                                    false,
                                    AuditEntry(
                                        "undo", state.draft!!.targetPath, "restored", "now")))
                  }))
        }
        .use { fixture ->
          fixture.render("review-compact-ready-360")
          fixture.assertTextFits("Ready to apply")
          fixture.assertTextFits("Apply change")
          assertEquals(1, fixture.textCount("Apply change"))
          assertFalse(fixture.hasText("Undo this change"))
          val bottom = fixture.taggedBounds("review-action-region")
          fixture.clickText("Check details")
          fixture.render()
          fixture.clickText("Project context")
          fixture.render()
          fixture.scrollBy(1000f, "review-scroll")
          fixture.render("review-details-anchored-360")
          assertEquals(bottom, fixture.taggedBounds("review-action-region"))
          fixture.assertTextFits("Apply change")
          assertEquals(0, applies + undoes + otherActions)
          assertTrue(fixture.requestFocus("Apply change"))
          fixture.render("review-apply-focus-360")
          fixture.assertColorVisible(FocusAccent)
          fixture.assertTextContrast(
              "Apply change", actionToneStyle(ActionTone.PositivePrimary).background)
          fixture.pressKey(Key.Enter)
          fixture.render("review-applied-360")
          assertEquals(1, applies)
          assertEquals(0, undoes)
          assertTrue(fixture.hasText("Change applied"))
          assertFalse(fixture.hasText("Apply change"))
          fixture.clickText("Undo this change")
          fixture.render("review-undone-360")
          assertEquals(1, applies)
          assertEquals(1, undoes)
          assertTrue(fixture.hasText("Change undone"))
          assertTrue(fixture.isDisabled("Undo is no longer available"))
          assertFalse(fixture.tryClick("Undo is no longer available"))
          assertEquals(1, undoes)
          assertEquals(0, otherActions)
        }
  }

  @Test
  fun requiredTestExecutionKeepsItsTrustScopeNextToTheExplicitAction() {
    val base = editorComparisonReviewFixture()
    val draft =
        base.draft!!.copy(
            taskSpec = BugTaskSpec(goTestCandidate = GoTestCandidateSpec("TestGetUser")))
    var state by
        mutableStateOf(base.copy(draft = draft, editor = editableDraft(draft), checks = null))
    var checks = 0
    var mutations = 0
    ComposeVisualFixture(300, 400, 1.5f) {
          ReviewToolWindow(
              state,
              ReviewToolWindowActions(
                  {
                    checks++
                    state = state.copy(checksRunning = true)
                  },
                  {},
                  {}),
              DraftApplicationActions({ mutations++ }, { mutations++ }))
        }
        .use { fixture ->
          fixture.render("review-trust-short-initial")
          val label = "Trust local execution & run checks"
          fixture.revealText(label, "review-action-scroll")
          fixture.render("review-trust-short-action")
          fixture.assertTextFits(label, maxLines = 3)
          assertTrue(fixture.hasText("go test ./... -run ^TestGetUser$"))
          fixture.assertTextAbove("go test ./... -run ^TestGetUser$", label)
          assertEquals(0, checks + mutations)
          fixture.clickText(label)
          fixture.render("review-checks-running")
          assertEquals(1, checks)
          assertEquals(0, mutations)
          assertFalse(fixture.hasText("Apply change"))
          assertTrue(fixture.hasText("Checks running"))
        }
  }

  @Test
  fun shortLargeTextReviewKeepsEveryRecoveryAndApplyScopeReachable() {
    val ready = editorComparisonReviewFixture()
    val invalid =
        ready.draft!!.copy(
            validation =
                ready.draft.validation!!.copy(
                    applicable = false,
                    diagnostics = listOf(DeclarationFinding("syntax", "missing closing brace"))))
    val states =
        listOf(
            "ready" to ready,
            "invalid" to ready.copy(draft = invalid, editor = editableDraft(invalid)),
            "dirty" to ready.copy(editor = editDraft(ready.editor!!, "func GetUser() {}")),
            "stale" to ready.copy(selected = ready.selected!!.copy(contentHash = "changed")),
            "failed" to
                ready.copy(
                    checks =
                        ready.checks!!.copy(
                            checks =
                                listOf(
                                    DraftCheck(
                                        "go test", true, "failed", output = "expected failure")))),
            "running" to ready.copy(checksRunning = true),
            "reported-running" to
                ready.copy(
                    checks =
                        ready.checks.copy(checks = listOf(DraftCheck("go test", true, "running")))),
            "skipped" to
                ready.copy(
                    checks =
                        ready.checks.copy(checks = listOf(DraftCheck("go test", true, "skipped")))))
    states.forEach { (name, state) ->
      var mutations = 0
      ComposeVisualFixture(300, 400, 1.5f) {
            ReviewToolWindow(
                state,
                ReviewToolWindowActions({}, {}, {}),
                DraftApplicationActions({ mutations++ }, { mutations++ }))
          }
          .use { fixture ->
            fixture.render("review-$name-300-400-1.5")
            if (name == "ready" || name == "skipped") {
              if (name == "skipped") {
                assertTrue(fixture.hasText("Skipped"))
                assertTrue(fixture.hasText("0 of 1 required check passed"))
              }
              fixture.revealText("Updates GetUser in internal/api/user.go.", "review-action-scroll")
              fixture.render("review-$name-short-action")
              fixture.assertTextFits("Apply change")
              fixture.assertTextFits("Updates GetUser in internal/api/user.go.", maxLines = 3)
              assertTrue(fixture.hasDescription("Apply GetUser to internal/api/user.go"))
            } else {
              assertFalse(fixture.hasText("Apply change"))
              if (name == "reported-running") {
                fixture.clickText("Check details")
                fixture.render()
                assertFalse(fixture.hasText("Rerun focused checks"))
              }
              val recovery =
                  if (name == "running" || name == "reported-running")
                      "Wait for current focused check evidence before reviewing Apply."
                  else "Edit draft"
              fixture.revealText(recovery, "review-action-scroll")
              fixture.render("review-$name-short-recovery")
              fixture.assertTextFits(recovery, maxLines = 5)
              assertFalse(fixture.hasText("Ready to apply"))
            }
            assertEquals(0, mutations)
          }
    }
  }

  @Test
  fun longTargetCannotPushTheActionOutsideATallReviewPane() {
    val base = editorComparisonReviewFixture()
    val path = "internal/" + "long-package/".repeat(18) + "user.go"
    val draft = base.draft!!.copy(targetPath = path)
    val state =
        base.copy(
            draft = draft,
            selected = base.selected!!.copy(path = path),
            editor = editableDraft(draft),
            checks = base.checks!!.copy(targetPath = path),
            session = base.session!!.copy(openPath = path))
    ComposeVisualFixture(300, 850, 1.5f) {
          ReviewToolWindow(
              state, ReviewToolWindowActions({}, {}, {}), DraftApplicationActions({}, {}))
        }
        .use { fixture ->
          fixture.render("review-long-target-300-850-1.5")
          val action = fixture.taggedBounds("review-action-region")
          assertTrue(action.height <= 425f)
          assertTrue(action.bottom <= 850f)
          fixture.revealText("Apply change", "review-action-scroll")
          fixture.assertTextFits("Apply change")
          assertTrue(fixture.hasDescription("Apply GetUser to $path"))
          fixture.revealText("Updates GetUser in $path.", "review-action-scroll")
          fixture.render("review-long-target-scope-300-850-1.5")
          assertTrue(fixture.verticalScrollValue("review-action-scroll") > 0f)
          fixture.assertTextFits("Updates GetUser in $path.", maxLines = 12)
        }
  }

  @Test
  fun receiptPathComesFromTheReturnedAuditRatherThanALaterSelection() {
    val receipt =
        ApplyResult("next", "after", true, AuditEntry("apply", "original.go", "applied", "now"))
    assertEquals(
        "original.go",
        reviewToolWindowScope(file().copy(path = "other.go"), null, draft(), receipt).path)
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
