package io.miniorca.desktop

import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
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
  fun fileCreationHasOneVisibleActionInDockedEditorAndContextAndRemainsAvailableInCompactContext() {
    val file = testFile("empty.go").copy(content = "package demo\n")
    val inspector =
        symbolInspectorUiState(
            file, emptyList(), null, null, false, InspectorProviderState(false, false), null)
    var creations = 0
    var analyses = 0
    val state = ContextToolWindowState(inspector, ScopedModel(), false, null, null)
    val actions =
        ContextToolWindowActions(
            {}, { analyses++ }, { analyses++ }, {}, {}, createDeclaration = { creations++ })
    ComposeVisualFixture(800, 320, 1.5f) {
          Row {
            EditorWorkspace(
                editorChromeUiState(
                    file, null, EditorSurface.Source, progress(EditorProgress.Inspect), null),
                null,
                {},
                { creations++ },
                canvas = { Text(file.content) },
                modifier = Modifier.weight(1f))
            CompositionLocalProvider(LocalContextCreationActionVisible provides false) {
              ContextToolWindow(state, actions, Modifier.weight(1f))
            }
          }
        }
        .use { fixture ->
          fixture.render("editor-context-one-new-function-800-1.5")
          assertEquals(1, fixture.textCount("New function"))
          fixture.clickText("New function")
          assertEquals(1, creations)
          assertEquals(0, analyses)
        }

    ComposeVisualFixture(320, 200, 1.5f) {
          CompositionLocalProvider(LocalContextCreationActionVisible provides true) {
            ContextToolWindow(state, actions, Modifier.fillMaxSize())
          }
        }
        .use { fixture ->
          fixture.render("context-new-function-compact-320-1.5")
          assertEquals(1, fixture.textCount("New function"))
          fixture.clickText("New function")
          assertEquals(2, creations)
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
  fun candidateTabAndEditDraftAreLocalNavigationAndReflectLiveEvidence() {
    var review by mutableStateOf(editorComparisonReviewFixture())
    var surface by mutableStateOf(EditorSurface.Source)
    var edits = 0
    var creations = 0
    ComposeVisualFixture(800, 600, 1.5f) {
          EditorWorkspace(
              editorChromeUiState(
                  review.selected,
                  review.selectedSymbol,
                  surface,
                  EditorProgressUiState(EditorProgress.Review, ""),
                  review.draft),
              review,
              { surface = it },
              { creations++ },
              canvas = {
                if (surface == EditorSurface.Review) ReviewDiffCanvas(review.draft)
                else Text("Source fixture")
              },
              onEditDraft = { edits++ })
        }
        .use { fixture ->
          fixture.render("editor-progression-ready-800-1.5")
          fixture.assertTextFits("Edit draft")
          fixture.awaitDescription("Focused checks: 1 checks (1 required) are current.", "Passed")
          fixture.clickText("Candidate diff")
          fixture.render()
          assertEquals(EditorSurface.Review, surface)
          assertEquals(0, edits)
          assertEquals(0, creations)
          assertFalse(fixture.hasEditableText())
          assertTrue(fixture.requestFocus("Edit draft"))
          fixture.render()
          fixture.pressKey(Key.Enter)
          assertEquals(1, edits)
          review = review.copy(checks = review.checks!!.copy(draftHash = "previous-draft"))
          fixture.render("editor-progression-stale-800-1.5")
          fixture.assertTextFits("Checks · Stale")
          fixture.assertTextFits("Review · Stale")
          review = review.copy(checks = null, checksRunning = true)
          fixture.render("editor-progression-running-800-1.5")
          fixture.assertTextFits("Checks · Running")
          assertEquals(1, edits)
          assertEquals(0, creations)
        }
  }

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
