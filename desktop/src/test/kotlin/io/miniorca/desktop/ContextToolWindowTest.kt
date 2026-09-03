package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ContextToolWindowTest {
  @Test
  fun contextSynchronizesBetweenFileAndExactDeclarationWithoutChangingSelectionState() {
    val file = file()
    val symbol = symbol()
    val fileContext = inspector(file, selectedSymbol = null)
    val declarationContext = inspector(file, selectedSymbol = symbol)

    assertEquals(SymbolInspectorMode.FileFallback, fileContext.mode)
    assertEquals("File context", contextHeaderLabel(fileContext))
    assertNull(contextStateBadge(fileContext))
    assertEquals(SymbolInspectorMode.SelectedSymbol, declarationContext.mode)
    assertEquals("Declaration · Run", contextHeaderLabel(declarationContext))
    assertTrue(contextToolWindowDescription(declarationContext).contains(file.path))
    assertTrue(contextToolWindowDescription(declarationContext).contains("Fresh"))
  }

  @Test
  fun contextKeepsRemoteConfirmationAdjacentToAnExplicitAnalysisAction() {
    val pending =
        symbolInspectorUiState(
            selectedFile = file(),
            symbols = listOf(symbol()),
            selectedSymbol = symbol(),
            analysis = FileAnalysis("internal/main.go", "stale"),
            analysisInProgress = false,
            provider =
                InspectorProviderState(remoteProvider = true, remoteProviderConfirmed = false),
            currentEditIdentity = null,
        )!!

    assertEquals(InspectorAnalysisAction.RefreshAnalysis, pending.analysisAction)
    assertTrue(pending.remoteProviderConfirmationRequired)
    assertTrue(contextToolWindowDescription(pending).contains("Stale"))
  }

  @Test
  fun rightToolWindowTabsAreTextualAndOnlyExplicitSelectionChangesTheirLayoutState() {
    val contextLayout = DesktopLayoutState(rightToolWindowVisible = false)

    assertEquals("Context", rightToolWindowLabel(RightToolWindow.Context))
    assertEquals(
        "Assistant tool window tab, not selected",
        rightToolWindowTabDescription(RightToolWindow.Assistant, selected = false))
    assertEquals(RightToolWindow.Context, contextLayout.activeRightToolWindow)
    assertFalse(contextLayout.rightToolWindowVisible)

    val reviewLayout =
        contextLayout
            .openRight(RightToolWindow.Review)
            .withFocus(DesktopFocusRegion.RightToolWindow)

    assertEquals(RightToolWindow.Review, reviewLayout.activeRightToolWindow)
    assertTrue(reviewLayout.rightToolWindowVisible)
    assertEquals(DesktopFocusRegion.RightToolWindow, reviewLayout.lastFocusedRegion)
  }

  @Test
  fun contextBadgesExposeBoundDraftStateWithoutAuthorizingAnyAction() {
    val draftIdentity =
        CurrentEditIdentity(ChatEditMode.ReplaceSymbol, "internal/main.go", "Run", true)
    val sessionIdentity = draftIdentity.copy(hasDraft = false)

    assertEquals(
        "CURRENT DRAFT · Run", contextStateBadge(inspector(file(), current = draftIdentity)))
    assertEquals(
        "BOUND CONVERSATION · Run", contextStateBadge(inspector(file(), current = sessionIdentity)))
    assertFalse(
        contextStateBadge(inspector(file(), current = draftIdentity)).orEmpty().contains("Apply"))
  }

  private fun inspector(
      file: ProjectFileInfo,
      selectedSymbol: SymbolInfo? = null,
      current: CurrentEditIdentity? = null,
  ) =
      symbolInspectorUiState(
          selectedFile = file,
          symbols = listOf(symbol()),
          selectedSymbol = selectedSymbol,
          analysis = FileAnalysis(file.path, "fresh", purpose = "Coordinates requests."),
          analysisInProgress = false,
          provider =
              InspectorProviderState(remoteProvider = false, remoteProviderConfirmed = false),
          currentEditIdentity = current,
      )!!

  private fun file() =
      ProjectFileInfo(
          path = "internal/main.go",
          contentHash = "hash",
          name = "main.go",
          language = "Go",
          sizeBytes = 256,
          lineCount = 18,
          modifiedAt = "",
          binary = false,
      )

  private fun symbol() =
      SymbolInfo(
          name = "Run",
          kind = "function",
          signature = "func Run() error",
          startLine = 4,
          endLine = 9,
          confidence = "exact",
          atomicTarget = true,
      )
}
