package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import kotlinx.serialization.json.Json

class ApplyReceiptTest {
  private val base = editorComparisonReviewFixture()
  private val scope =
      AppliedDeclarationScope(
          base.project!!.projectId, base.selected!!.path, "GetUser", DraftMutationOperation.Apply)
  private val receipt = ApplyResult("next", "after", true)
  private val project = base.project!!.copy(projectRevision = "next")
  private val file = base.selected!!.copy(contentHash = "after")

  @Test
  fun onlyMatchingRefreshedSourceAndTheImmediatelyPrecedingApplyPermitUndo() {
    assertTrue(undoEligibility(project, file, receipt, scope).eligible)
    assertFalse(
        undoEligibility(project, file.copy(contentHash = "external"), receipt, scope).eligible)
    assertFalse(
        undoEligibility(project.copy(projectRevision = "external"), file, receipt, scope).eligible)
    assertFalse(undoEligibility(project, file.copy(path = "other.go"), receipt, scope).eligible)
    assertFalse(undoEligibility(project, null, receipt, scope).eligible)
    assertFalse(undoEligibility(project, file, receipt, null).eligible)
    assertFalse(undoEligibility(project, file, receipt.copy(undoAvailable = false), scope).eligible)
    assertFalse(
        undoEligibility(project, file, receipt, scope.copy(operation = DraftMutationOperation.Undo))
            .eligible)
    val conflict =
        DraftMutationAttempt(
            DraftMutationOperation.Undo, DraftMutationStatus.Conflict, "Backup expired")
    assertEquals("Backup expired", undoEligibility(project, file, receipt, scope, conflict).reason)
  }

  @Test
  fun metadataWarningKeepsAppliedReceiptAndGuardedUndoVisible() {
    val warning = "Source changed, but the audit could not be saved."
    val state =
        base.copy(
            project = project,
            selected = file,
            applied = receipt.copy(warnings = listOf(warning)),
            receiptScope = scope,
        )
    var undoes = 0
    ComposeVisualFixture(400, 600) {
          ReviewToolWindow(
              state, ReviewToolWindowActions({}, {}, {}), DraftApplicationActions({}, { undoes++ }))
        }
        .use { fixture ->
          fixture.render("receipt-metadata-warning")
          assertTrue(fixture.hasText("Change applied"))
          assertTrue(fixture.hasText(warning))
          fixture.clickText("Undo this change")
          fixture.render("receipt-metadata-warning-undo")
          assertEquals(1, undoes)
        }
  }

  @Test
  fun optionalAuditFieldsDecodeAndContradictoryReceiptsAreRejected() {
    val result =
        Json.decodeFromString<ApplyResult>(
            """{"project_revision":"next","post_apply_hash":"after","undo_available":true,"audit":{"id":"apply-1","action":"apply","target_path":"${scope.path}","outcome":"applied","timestamp":"2026-10-03T00:00:00Z","before_hash":"before","after_hash":"after","project_id":"${scope.projectId}","project_revision":"next","generation_id":"draft-1"}}""")
    validateMutationReceipt(result, scope)
    assertEquals(emptyList(), result.warnings)
    assertEquals("before", result.audit!!.beforeHash)
    assertEquals("draft-1", result.audit.generationId)
    for (invalid in
        listOf(
            result.copy(postApplyHash = ""),
            result.copy(audit = result.audit.copy(targetPath = "other.go")),
            result.copy(audit = result.audit.copy(outcome = "failed")),
            result.copy(audit = result.audit.copy(afterHash = "foreign")))) {
      assertFailsWith<IllegalArgumentException> { validateMutationReceipt(invalid, scope) }
    }
    validateMutationReceipt(receipt, scope)
  }

  @Test
  fun receiptKeepsActualAuditSelectableAndConflictRecoveryReachable() {
    val audit =
        AuditEntry(
            "apply",
            scope.path,
            "applied",
            "2026-10-03",
            id = "apply-1",
            beforeHash = "before",
            afterHash = "after")
    val state =
        base.copy(
            project = project,
            selected = file,
            applied = receipt.copy(audit = audit),
            receiptScope = scope,
            mutation =
                DraftMutationAttempt(
                    DraftMutationOperation.Undo,
                    DraftMutationStatus.Conflict,
                    "Source changed externally"),
            receiptRefreshError = "Read unavailable")
    var refreshes = 0
    var undoes = 0
    ComposeVisualFixture(360, 500, 1.5f) {
          ReviewToolWindow(
              state,
              ReviewToolWindowActions({}, {}, {}, refreshSource = { refreshes++ }),
              DraftApplicationActions({}, { undoes++ }))
        }
        .use { fixture ->
          fixture.render("f34-receipt-conflict-150")
          assertTrue(fixture.hasText("Change applied"))
          assertFalse(fixture.hasText("Change undone"))
          assertTrue(fixture.hasText("Audit ID: apply-1"))
          assertTrue(fixture.hasText("Before hash: before"))
          assertTrue(fixture.hasText("Source refresh failed: Read unavailable"))
          assertFalse(fixture.hasText("Undo this change"))
          fixture.revealText("Refresh source", "review-scroll")
          fixture.clickText("Refresh source")
          fixture.render("f34-receipt-refresh-action-150")
          assertEquals(1, refreshes)
          assertEquals(0, undoes)
        }
    ComposeVisualFixture(600, 400) { ApplyReceiptDetails(receipt, scope) }
        .use { fixture ->
          fixture.render("f34-receipt-no-audit")
          assertTrue(fixture.hasText("Resulting source hash: after"))
          assertFalse(fixture.hasText("Audit ID:"))
          assertFalse(fixture.hasText("Recorded at:"))
        }
  }
}
