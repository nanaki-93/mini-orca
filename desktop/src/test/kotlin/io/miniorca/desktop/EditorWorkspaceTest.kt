package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class EditorWorkspaceTest {
  @Test
  fun sourceStaysActiveWhenAValidatedDraftMakesReviewAvailable() {
    val chrome =
        editorChromeUiState(
            file = testFile("internal/runner/run.go"),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Review),
            draft = validatedDraft(),
        )

    assertEquals(EditorSurface.Source, chrome.activeSurface)
    assertTrue(chrome.reviewAvailable)
    assertEquals("REVIEW READY", chrome.stageLabel)
    assertTrue(chrome.accessibleDescription.contains("Read-only source surface"))
  }

  @Test
  fun reviewIsOnlyShownAfterAnExplicitSelectionAndCurrentValidation() {
    val review =
        editorChromeUiState(
            file = testFile("internal/runner/run.go"),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Review,
            progress = progress(EditorProgress.Review),
            draft = validatedDraft(),
        )
    val stale =
        editorChromeUiState(
            file = testFile("internal/runner/run.go"),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Review,
            progress = progress(EditorProgress.Edit),
            draft = validatedDraft(),
        )

    assertEquals(EditorSurface.Review, review.activeSurface)
    assertEquals("CURRENT REVIEW", review.stageLabel)
    assertEquals(EditorSurface.Source, stale.activeSurface)
    assertTrue(!stale.reviewAvailable)
  }

  @Test
  fun breadcrumbsCollapseLongPathsButAccessibilityRetainsTheFullPath() {
    val path = "src/module/feature/runner/run.go"
    val chrome =
        editorChromeUiState(
            file = testFile(path),
            selectedSymbol = symbol(),
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )

    assertEquals("src / … / run.go / Run", chrome.breadcrumbs)
    assertTrue(chrome.accessibleDescription.contains(path))
    assertTrue(chrome.accessibleDescription.contains("Selected declaration Run"))
  }

  @Test
  fun duplicateBasenamesKeepTheirOwnProjectRelativeIdentity() {
    val first =
        editorChromeUiState(
            file = testFile("cmd/worker/main.go"),
            selectedSymbol = null,
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )
    val second =
        editorChromeUiState(
            file = testFile("cmd/server/main.go"),
            selectedSymbol = null,
            requestedSurface = EditorSurface.Source,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )

    assertEquals("main.go", first.title)
    assertEquals("main.go", second.title)
    assertEquals("cmd/worker/main.go", first.path)
    assertEquals("cmd/server/main.go", second.path)
    assertTrue(first.accessibleDescription.contains(first.path))
    assertTrue(second.accessibleDescription.contains(second.path))
  }

  @Test
  fun emptyFileStateIsNeutralAndReadOnly() {
    val chrome =
        editorChromeUiState(
            file = null,
            selectedSymbol = null,
            requestedSurface = EditorSurface.Review,
            progress = progress(EditorProgress.Inspect),
            draft = null,
        )

    assertEquals("No file open", chrome.title)
    assertEquals("No file selected", chrome.path)
    assertEquals(EditorSurface.Source, chrome.activeSurface)
    assertEquals("No file selected", chrome.breadcrumbs)
    assertTrue(chrome.accessibleDescription.contains("Read-only source surface"))
  }

  @Test
  fun candidateSummaryUsesOnlyTheCurrentDraftAndValidationStage() {
    val draft =
        validatedDraft()
            .copy(
                validation =
                    DeclarationValidation(
                        applicable = true,
                        scopeMode = "replace_symbol",
                        diff =
                            UnifiedDiff(
                                "internal/runner/run.go",
                                "internal/runner/run.go",
                                listOf(DiffLine("added", newLine = 1, text = "func Run() {}"))),
                    ))
    val chrome =
        editorChromeUiState(
            testFile("internal/runner/run.go"),
            symbol(),
            EditorSurface.Source,
            progress(EditorProgress.Review),
            draft)
    val ready = candidateSummaryPresentation(draft, chrome)!!

    assertEquals("internal/runner/run.go", ready.target)
    assertEquals("REVIEW READY", ready.stage)
    assertEquals("1 changed lines in the current draft", ready.changedLines)
    assertTrue(ready.reviewAvailable)
    assertEquals(null, candidateSummaryPresentation(null, readyChrome()))
  }

  private fun readyChrome() =
      editorChromeUiState(
          testFile("main.go"), null, EditorSurface.Source, progress(EditorProgress.Inspect), null)

  private fun progress(value: EditorProgress) = EditorProgressUiState(value, "")

  private fun symbol() = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)

  private fun validatedDraft() =
      DeclarationDraft(
          id = "draft",
          targetPath = "internal/runner/run.go",
          revision = 3,
          validation =
              DeclarationValidation(
                  applicable = true,
                  scopeMode = "replace_symbol",
                  diff = UnifiedDiff("internal/runner/run.go", "internal/runner/run.go")),
      )

  private fun testFile(path: String) =
      ProjectFileInfo(
          path = path,
          contentHash = "hash",
          name = path.substringAfterLast('/'),
          language = "Go",
          sizeBytes = 0,
          lineCount = 0,
          modifiedAt = "",
          binary = false,
      )
}
