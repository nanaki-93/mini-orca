package io.miniorca.desktop

import androidx.compose.material.Text
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class EditorWorkspaceTest {
  @Test
  fun creationAcceptsAGoFileWithoutDeclarationsAndExplainsUnsupportedOrBusyStates() {
    val file = testFile("empty.go").copy(content = "package demo\n")
    assertNull(declarationCreationBlockedReason(file))
    assertEquals(
        "Open a Go file to create a function or type.", declarationCreationBlockedReason(null))
    assertEquals(
        "Function and type creation requires a Go source file.",
        declarationCreationBlockedReason(file.copy(language = "Kotlin")))
    assertEquals(
        "Function and type creation requires a Go source file.",
        declarationCreationBlockedReason(file.copy(binary = true)))
    assertEquals(
        "Wait for the current generation or validation to finish.",
        declarationCreationBlockedReason(file, busy = true))
    assertNull(
        editorChromeUiState(
                file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null)
            .creationBlockedReason)
  }

  @Test
  fun newFunctionIsVisibleAndKeyboardActivationDoesNotSelectAnEditorSurface() {
    listOf(360 to 1.5f, 800 to 1f).forEach { (width, scale) ->
      var creations = 0
      var surfaceSelections = 0
      val file = testFile("internal/empty.go").copy(content = "package demo\n")
      val chrome =
          editorChromeUiState(
              file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null)
      ComposeVisualFixture(width, 650, scale) {
            EditorWorkspace(
                chrome,
                null,
                { surfaceSelections++ },
                { creations++ },
                canvas = { Text(file.content) })
          }
          .use { fixture ->
            fixture.render("editor-new-function-$width-$scale")
            fixture.assertTextFits("New function")
            assertTrue(fixture.hasDescription("New function in internal/empty.go"))
            assertFalse(fixture.isDisabled("New function"))
            assertEquals(0, creations)
            assertTrue(fixture.requestFocus("New function"))
            fixture.render()
            fixture.pressKey(Key.Enter)
            fixture.render()
            assertEquals(1, creations)
            assertEquals(0, surfaceSelections)
            assertEquals("package demo\n", file.content)
          }
    }
  }

  @Test
  fun contextOffersCreationWithNoSelectedSymbolWithoutRunningAnalysis() {
    val file = testFile("empty.go").copy(content = "package demo\n")
    val inspector =
        symbolInspectorUiState(
            file, emptyList(), null, null, false, InspectorProviderState(false, false), null)
    var creations = 0
    var analyses = 0
    ComposeVisualFixture(320, 200, 1.5f) {
          ContextCreationAction(
              ContextToolWindowState(inspector, ScopedModel(), false, null, null),
              ContextToolWindowActions(
                  {}, { analyses++ }, { analyses++ }, {}, {}, createDeclaration = { creations++ }))
        }
        .use { fixture ->
          fixture.render("context-new-function-320-1.5")
          assertTrue(fixture.hasText("New function"))
          fixture.clickText("New function")
          assertEquals(1, creations)
          assertEquals(0, analyses)
        }
  }

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
    assertEquals("VALIDATED DRAFT", chrome.stageLabel)
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
    assertEquals("REVIEW CANDIDATE", review.stageLabel)
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

    assertEquals(
        listOf(
            EditorBreadcrumbSegment("src", EditorBreadcrumbKind.Folder),
            EditorBreadcrumbSegment("…", EditorBreadcrumbKind.Collapsed),
            EditorBreadcrumbSegment("run.go", EditorBreadcrumbKind.File),
            EditorBreadcrumbSegment("Run", EditorBreadcrumbKind.Symbol),
        ),
        chrome.breadcrumbSegments,
    )
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
  fun rootAndNestedFilesKeepTheirActualSegmentsWhenNoSymbolIsSelected() {
    assertEquals(
        listOf(EditorBreadcrumbSegment("main.go", EditorBreadcrumbKind.File)),
        editorBreadcrumbSegments("main.go"),
    )
    assertEquals(
        listOf(
            EditorBreadcrumbSegment("internal", EditorBreadcrumbKind.Folder),
            EditorBreadcrumbSegment("server", EditorBreadcrumbKind.Folder),
            EditorBreadcrumbSegment("main.go", EditorBreadcrumbKind.File),
        ),
        editorBreadcrumbSegments("internal/server/main.go"),
    )
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
    assertEquals(
        listOf(EditorBreadcrumbSegment("No file selected", EditorBreadcrumbKind.Placeholder)),
        chrome.breadcrumbSegments,
    )
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
    assertEquals("VALIDATED DRAFT", ready.stage)
    assertEquals("1 changed lines", ready.changedLines)
    assertTrue(ready.reviewAvailable)
    assertEquals(
        "Validate to compose a diff.",
        candidateSummaryPresentation(draft.copy(validation = null), chrome)!!.changedLines)
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
