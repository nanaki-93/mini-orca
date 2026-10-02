package io.miniorca.desktop

import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertSame

class DraftFieldBuffersTest {
  private fun draft(
      id: String = "draft",
      revision: Long = 1,
      hash: String = "hash",
      declaration: String = "func Run() {}",
      imports: List<String> = listOf("fmt"),
  ) =
      DeclarationDraft(
          id,
          "project",
          "project-revision",
          "base",
          "main.go",
          "replace_symbol",
          "Run",
          declaration,
          imports,
          revision = revision,
          hash = hash)

  @Test
  fun sameCandidateRetainsRawBuffersThroughNavigationAndIntermediatePatch() {
    val original = draft()
    val fields =
        reconcileDraftFields(null, editableDraft(original))!!.copy(
            declaration = TextFieldValue(original.declaration, TextRange(4, 8)),
            imports = TextFieldValue(" fmt, ", TextRange(5, 6)))
    val dirty = editDraft(editableDraft(original), imports = listOf("fmt"))
    assertSame(fields, reconcileDraftFields(fields, dirty))
    assertSame(fields, reconcileDraftFields(fields, dirty.copy(status = DraftEditorStatus.Stale)))
    val patch =
        dirty.copy(
            serverDraft = original.copy(revision = 2, hash = "patch"),
            status = DraftEditorStatus.Validating)
    assertSame(fields, reconcileDraftFields(fields, patch))
    assertEquals(" fmt, ", reconcileDraftFields(fields, patch)?.imports?.text)
  }

  @Test
  fun acceptedResponsePreservesUnchangedSelectionsAndClampsOnlyNormalizedFields() {
    val original = draft()
    val fields =
        reconcileDraftFields(null, editableDraft(original))!!.copy(
            declaration = TextFieldValue(original.declaration, TextRange(4, 8), TextRange(4, 8)),
            imports = TextFieldValue("fmt", TextRange(1, 3), TextRange(1, 3)))
    val unchanged =
        reconcileDraftFields(
            fields,
            editableDraft(original.copy(revision = 2, hash = "new"))
                .copy(acceptanceGeneration = 1))!!
    assertSame(fields.declaration, unchanged.declaration)
    assertSame(fields.imports, unchanged.imports)
    assertEquals(TextRange(4, 8), unchanged.declaration.composition)
    assertEquals(TextRange(1, 3), unchanged.imports.composition)
    val normalized =
        reconcileDraftFields(
            unchanged,
            editableDraft(
                    original.copy(
                        revision = 3, hash = "newer", declaration = "go", imports = listOf("io")))
                .copy(acceptanceGeneration = 2))!!
    assertEquals("go", normalized.declaration.text)
    assertEquals(TextRange(2, 2), normalized.declaration.selection)
    assertEquals(TextRange(2, 2), normalized.declaration.composition)
    assertEquals("io", normalized.imports.text)
    assertEquals(TextRange(1, 2), normalized.imports.selection)
    assertEquals(TextRange(1, 2), normalized.imports.composition)
    assertSame(
        normalized,
        reconcileDraftFields(
            normalized, editableDraft(original.copy(revision = 2)).copy(acceptanceGeneration = 1)))
  }

  @Test
  fun intermediateUpdateAndStaleStateDoNotReconcileUntilAcceptedLoad() {
    val original = draft()
    val initial =
        DesktopState(review = DraftReviewState(draft = original, editor = editableDraft(original)))
    val raw =
        reconcileDraftFields(null, initial.review.editor)!!.copy(
            imports = TextFieldValue(" fmt, ", TextRange(5)))
    val started = initial.reduce(DesktopEvent.DraftValidationStarted(7))
    val patched =
        started.reduce(
            DesktopEvent.DraftValidationUpdated(
                7, original.copy(revision = 2, hash = "patch", imports = listOf("io"))))
    assertSame(raw, reconcileDraftFields(raw, patched.review.editor))
    val stale = patched.reduce(DesktopEvent.DraftMarkedStale)
    assertSame(raw, reconcileDraftFields(raw, stale.review.editor))
    val loaded =
        patched.reduce(
            DesktopEvent.DraftLoaded(
                original.copy(revision = 2, hash = "validated", imports = listOf("io"))))
    assertEquals(1, loaded.review.editor?.acceptanceGeneration)
    assertEquals("io", reconcileDraftFields(raw, loaded.review.editor)?.imports?.text)
  }

  @Test
  fun replacementAndDiscardClearPreviousBuffers() {
    val fields =
        reconcileDraftFields(null, editableDraft(draft()))!!.copy(
            imports = TextFieldValue("partial, ", TextRange(9)))
    assertNull(reconcileDraftFields(fields, null))
    val replacement = reconcileDraftFields(fields, editableDraft(draft(id = "replacement")))!!
    assertEquals("fmt", replacement.imports.text)
    assertEquals(TextRange.Zero, replacement.imports.selection)
    val otherFile =
        reconcileDraftFields(fields, editableDraft(draft().copy(targetPath = "other.go")))!!
    assertEquals("fmt", otherFile.imports.text)
    assertEquals("other.go", otherFile.identity.path)
  }
}
