package io.miniorca.desktop

import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DraftEditorStateTest {
  @Test
  fun manualDeclarationOrImportEditMakesTheDraftDirtyAndClearsEvidence() {
    val original =
        draft(
            validation =
                DeclarationValidation(
                    true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
    val state =
        DesktopState(
                review =
                    DraftReviewState(
                        draft = original,
                        editor = editableDraft(original),
                        checks =
                            DraftCheckReport(
                                "main.go",
                                true,
                                draftId = original.id,
                                draftRevision = original.revision,
                                draftHash = original.hash),
                        benchmark =
                            BenchmarkEvidenceState(
                                catalog = GoBenchmarkCatalog(available = true),
                                discovery = BenchmarkDiscoveryOutcome.Loaded),
                    ),
            )
            .reduce(
                DesktopEvent.DraftEdited(
                    declaration = "func Run() error { return nil }", imports = listOf("fmt")))

    assertEquals(DraftEditorStatus.Dirty, state.review.editor?.status)
    assertEquals("func Run() error { return nil }", state.review.editor?.declaration)
    assertEquals(listOf("fmt"), state.review.editor?.imports)
    assertNull(state.review.draft?.validation)
    assertNull(state.review.checks)
    assertNull(state.review.benchmark.catalog)
    assertEquals(BenchmarkDiscoveryOutcome.Invalidated, state.review.benchmark.discovery)
    assertFalse(draftApplyEligibility(state.review.draft, state.review.checks, file()).eligible)
  }

  @Test
  fun selectionAndRepeatedCallbacksDoNotEmitEdits() {
    val input = TextFieldValue("func Run() {}")
    assertNull(declarationTextEdit(input, input))
    assertNull(declarationTextEdit(input, input.copy(selection = TextRange(5))))
    val edited = TextFieldValue("func Run() { changed() }")
    assertEquals(edited.text, declarationTextEdit(input, edited)?.declaration)
    assertNull(declarationTextEdit(edited, edited))
    assertNull(importTextEdit("fmt, io", "fmt, io"))
  }

  @Test
  fun rawImportEditsAndRestoredTextRemainDirtyEvenWhenImportsNormalizeIdentically() {
    val original = draft(imports = listOf("fmt"))
    val initial =
        DesktopState(
            review =
                DraftReviewState(
                    draft = original,
                    editor = editableDraft(original),
                    checks = DraftCheckReport("main.go", true)))
    val changed = importTextEdit("fmt", " fmt, ")!!
    assertEquals(listOf("fmt"), changed.imports)
    val edited = initial.reduce(changed)
    assertEquals(DraftEditorStatus.Dirty, edited.review.editor?.status)
    assertNull(edited.review.checks)
    assertNull(edited.review.draft?.validation)

    val restored = edited.reduce(importTextEdit(" fmt, ", "fmt")!!)
    assertEquals(original.declaration, restored.review.editor?.declaration)
    assertEquals(original.imports, restored.review.editor?.imports)
    assertEquals(DraftEditorStatus.Dirty, restored.review.editor?.status)
    assertNull(restored.review.draft?.validation)
    assertNull(restored.review.checks)
  }

  @Test
  fun selectionDoesNotInvalidatePendingValidationButATextEditDoes() {
    val controller = DesktopWorkflowController()
    val projectRequest = controller.beginProjectLoad()
    assertTrue(
        controller.projectLoaded(projectRequest, project(), ProjectIndex("project", "revision")))
    val fileRequest = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(fileRequest, file(), emptyList()))
    val original = draft()
    controller.dispatch(DesktopEvent.DraftLoaded(original))

    val (request, fileIdentity) = controller.beginDraftValidation()!!
    val before = controller.state
    val input = TextFieldValue(original.declaration)
    declarationTextEdit(input, input.copy(selection = TextRange(2)))?.let(controller::dispatch)
    assertEquals(before, controller.state)
    assertTrue(controller.draftValidated(request, fileIdentity, original))

    val (lateRequest, lateFile) = controller.beginDraftValidation()!!
    controller.dispatch(declarationTextEdit(input, TextFieldValue("func Run() { changed() }"))!!)
    assertFalse(controller.draftValidated(lateRequest, lateFile, original))
    assertEquals(DraftEditorStatus.Dirty, controller.state.review.editor?.status)
    controller.dispatch(declarationTextEdit(TextFieldValue("func Run() { changed() }"), input)!!)
    assertEquals(original.declaration, controller.state.review.editor?.declaration)
    assertEquals(DraftEditorStatus.Dirty, controller.state.review.editor?.status)
    assertNull(controller.state.review.draft?.validation)
  }

  @Test
  fun requiredImportsAreVisibleOnlyWhenTheDraftAlreadyHasImports() {
    val emptyImports = editableDraft(draft())
    val populatedImports = editableDraft(draft(imports = listOf("fmt", "io")))

    assertFalse(requiredImportsVisible(emptyImports))
    assertTrue(requiredImportsVisible(populatedImports))
    assertEquals(listOf("fmt", "io"), populatedImports.imports)
    assertEquals(listOf("fmt", "io"), parseRequiredImports(" fmt, io, "))
  }

  @Test
  fun validationResponseReplacesLocalTextWithDaemonNormalizedDraft() {
    val generated = draft()
    val normalized =
        generated.copy(
            declaration = "func Run() error {\n\treturn nil\n}",
            imports = listOf("fmt"),
            revision = 3,
            hash = "normalized",
            validation =
                DeclarationValidation(
                    true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")),
        )
    val state =
        DesktopState(
                review =
                    DraftReviewState(
                        draft = generated,
                        editor =
                            editDraft(
                                editableDraft(generated),
                                declaration = "func Run() error{return nil}")))
            .reduce(DesktopEvent.DraftLoaded(normalized))

    assertEquals(DraftEditorStatus.Valid, state.review.editor?.status)
    assertEquals(normalized.declaration, state.review.editor?.declaration)
    assertEquals(normalized.imports, state.review.editor?.imports)
    assertEquals(3, state.review.draft?.revision)
  }

  @Test
  fun invalidValidationResultStaysEditableAndCannotAuthorizeChecksOrApply() {
    val invalid =
        draft(
            validation =
                DeclarationValidation(
                    false,
                    "replace_symbol",
                    diagnostics = listOf(DeclarationFinding("syntax", "missing brace")),
                    diff = UnifiedDiff("main.go", "main.go")))
    val state = DesktopState().reduce(DesktopEvent.DraftLoaded(invalid))

    assertEquals(DraftEditorStatus.Invalid, state.review.editor?.status)
    assertEquals("missing brace", state.review.editor?.diagnostics?.single()?.message)
    assertFalse(draftApplyEligibility(state.review.draft, null, file()).eligible)
    assertEquals(
        DraftEditorStatus.Dirty,
        state
            .reduce(DesktopEvent.DraftEdited(declaration = "func Run() {} "))
            .review
            .editor
            ?.status)
  }

  @Test
  fun optimisticConflictMarksTheDraftStaleAndPreservesNoPriorApproval() {
    val valid =
        draft(
            validation =
                DeclarationValidation(
                    true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
    val state =
        DesktopState(
                review =
                    DraftReviewState(
                        draft = valid,
                        editor = editableDraft(valid),
                        checks =
                            DraftCheckReport(
                                "main.go",
                                true,
                                draftId = valid.id,
                                draftRevision = valid.revision,
                                draftHash = valid.hash)))
            .reduce(DesktopEvent.DraftMarkedStale)

    assertEquals(DraftEditorStatus.Stale, state.review.editor?.status)
    assertNull(state.review.draft?.validation)
    assertNull(state.review.checks)
    assertFalse(draftApplyEligibility(state.review.draft, state.review.checks, file()).eligible)
  }

  @Test
  fun staleBaseIdentityCannotBeValidated() {
    val editor = editableDraft(draft())

    assertTrue(draftEditorMatchesOpenFile(editor, file(), project()))
    assertFalse(draftEditorMatchesOpenFile(editor, file(hash = "changed"), project()))
    assertFalse(draftEditorMatchesOpenFile(editor, file(), project(revision = "next")))
  }

  private fun draft(
      validation: DeclarationValidation? = null,
      imports: List<String> = emptyList()
  ) =
      DeclarationDraft(
          "draft",
          "project",
          "revision",
          "base",
          "main.go",
          "replace_symbol",
          "Run",
          "func Run() {}",
          imports = imports,
          revision = 2,
          hash = "hash",
          validation = validation)

  private fun file(hash: String = "base") =
      ProjectFileInfo(
          "main.go",
          hash,
          "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)

  private fun project(revision: String = "revision") =
      ProjectAnalysis(
          "project",
          revision,
          "project",
          "/tmp/project",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 1,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")
}
