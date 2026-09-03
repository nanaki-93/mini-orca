package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class IdeUiContractBaselineTest {
  @Test
  fun landingIsolationAndNarrowEditorDrawersRemainExplicit() {
    assertEquals(DesktopShellMode.ProjectLanding, desktopShellMode(DesktopState()))
    assertTrue(shortcutAvailable(DesktopShellMode.ProjectLanding, DesktopShortcut.OpenProject))
    assertFalse(shortcutAvailable(DesktopShellMode.ProjectLanding, DesktopShortcut.OpenFile))
    assertFalse(useNarrowLayout(1_000f))
    assertTrue(useNarrowLayout(999f))
    assertTrue(editorDrawerActionsVisible(Workspace.Editor, 999f))
    assertFalse(editorDrawerActionsVisible(Workspace.Bugs, 999f))
  }

  @Test
  fun openingANewFileReplacesTheOnlyActiveFileSymbolAndDraftContext() {
    val first = file("cmd/first.go", "first-hash")
    val second = file("internal/second.go", "second-hash")
    val symbol = SymbolInfo("First", "function", confidence = "exact", atomicTarget = true)
    val draft = draft(first)
    val initial =
        DesktopState(
            projectState = ProjectWorkspaceState(project()),
            selection =
                FileSelectionState(
                    selectedFile = first, symbols = listOf(symbol), selectedSymbol = symbol),
            review = DraftReviewState(draft = draft, editor = editableDraft(draft)),
        )

    val next = initial.reduce(DesktopEvent.FileLoaded(second, emptyList()))

    assertEquals(second, next.selectedFile)
    assertTrue(next.symbols.isEmpty())
    assertNull(next.selectedSymbol)
    assertNull(next.review.draft)
    assertNull(next.review.editor)
  }

  @Test
  fun sourceAndDiffPresentationsPreserveTheirInputText() {
    val source = "package demo\\nfunc Run() string { return \\\"ok\\\" }\\n"
    val diff =
        UnifiedDiff(
            "main.go",
            "main.go",
            listOf(
                DiffLine("removed", oldLine = 2, text = "func Run() string { return \\\"old\\\" }"),
                DiffLine("added", newLine = 2, text = "func Run() string { return \\\"ok\\\" }"),
            ))
    val originalLines = diff.lines.toList()
    val rows = sideBySideDiffRows(diff)

    assertEquals(source, highlightedCode(source).text)
    assertTrue(sourceTapSelectsContext(dragged = false))
    assertFalse(sourceTapSelectsContext(dragged = true))
    assertEquals(originalLines, diff.lines)
    assertEquals("removed", rows.single().before?.change)
    assertEquals("added", rows.single().proposed?.change)
  }

  @Test
  fun applyRequiresCurrentValidationChecksAndMatchingIdentity() {
    val project = project()
    val file = file("main.go", "base-hash")
    val validDraft = draft(file)
    val currentChecks =
        DraftCheckReport(
            targetPath = file.path,
            applicable = true,
            checks = listOf(DraftCheck("go test", required = true, state = "passed")),
            draftId = validDraft.id,
            draftRevision = validDraft.revision,
            draftHash = validDraft.hash)

    assertTrue(
        draftReviewEligibility(editableDraft(validDraft), validDraft, currentChecks, file, project)
            .eligible)
    assertFalse(
        draftReviewEligibility(
                editableDraft(validDraft),
                validDraft,
                currentChecks.copy(draftHash = "stale"),
                file,
                project)
            .eligible)
    assertFalse(
        draftReviewEligibility(
                editDraft(
                    editableDraft(validDraft), declaration = "func Run() error { return nil }"),
                validDraft,
                currentChecks,
                file,
                project)
            .eligible)
  }

  @Test
  fun remoteProviderDestinationKeepsConfirmationRequirementVisible() {
    val destination =
        modelDestinationLabel(
            ModelScope.Function,
            ScopedModel(
                scope = "function",
                profile = "function",
                model = "remote-model",
                remoteProvider = true))

    assertTrue(destination.contains("remote provider"))
    assertTrue(destination.contains("confirmation required"))
  }

  private fun project() =
      ProjectAnalysis(
          "project",
          "revision",
          "project",
          "/tmp/project",
          "go",
          fileCount = 2,
          sourceFileCount = 2,
          totalLines = 2,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun file(path: String, hash: String) =
      ProjectFileInfo(
          path,
          hash,
          path.substringAfterLast('/'),
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)

  private fun draft(file: ProjectFileInfo) =
      DeclarationDraft(
          id = "draft",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = file.contentHash,
          targetPath = file.path,
          mode = "replace_symbol",
          targetSymbol = "Run",
          declaration = "func Run() string { return \\\"ok\\\" }",
          revision = 2,
          hash = "draft-hash",
          validation =
              DeclarationValidation(
                  applicable = true,
                  scopeMode = "replace_symbol",
                  diff = UnifiedDiff(file.path, file.path)))
}
