package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ContextToolWindowTest {
  @Test
  fun noSelectionAndFailedLocalFileReadKeepFileNavigationAdjacent() {
    var selections = 0
    var analyses = 0
    val actions =
        ContextToolWindowActions({}, { analyses++ }, {}, {}, {}, openFile = { selections++ })
    for (error in listOf(null, "Permission denied", "")) {
      ComposeVisualFixture(320, 300, 1.5f) {
            ContextToolWindow(
                ContextToolWindowState(
                    null, ScopedModel(), false, null, null, fileReadError = error),
                actions)
          }
          .use { fixture ->
            fixture.render()
            assertTrue(
                fixture.hasText(if (error == null) "No file selected" else "Could not open file"))
            if (error != null) {
              assertTrue(
                  fixture.hasText(
                      "Reading local file data failed. ${error.ifBlank { "No details available." }} Select a file in Files to try again."))
            }
            fixture.clickText("Select a file")
          }
    }
    assertEquals(3, selections)
    assertEquals(0, analyses)
  }

  @Test
  fun projectAnalysisActionAndFileResultsHaveSeparateExplicitIntents() {
    var admissions = 0
    var navigations = 0
    val actions =
        ContextToolWindowActions({}, { admissions++ }, {}, {}, {}, viewResults = { navigations++ })
    ComposeVisualFixture(320, 300, 1.5f) {
          ContextProjectAnalysisActions(
              ContextToolWindowState(null, ScopedModel(), false, null, null), actions)
        }
        .use { fixture ->
          fixture.render("context-project-actions-320-1.5")
          fixture.assertTextFits("Analyze project")
          fixture.clickText("View this file’s results")
          assertEquals(0, admissions)
          assertEquals(1, navigations)
          fixture.clickText("Analyze project")
          assertEquals(1, admissions)
        }
  }

  @Test
  fun declarationExplanationActionsDistinguishLifecycle() {
    assertEquals("Explain declaration", explanationActionLabel(DeclarationExplanationState()))
    assertEquals(
        "Cancel explanation",
        explanationActionLabel(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Loading)))
    assertEquals(
        "Refresh explanation",
        explanationActionLabel(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Current)))
  }

  @Test
  fun declarationActionsKeepExplanationPrimaryUntilTheCurrentExplanationCanSupportRefactor() {
    assertEquals(
        DeclarationActionPresentation(ActionTone.Primary, ActionTone.Neutral),
        declarationActionPresentation(DeclarationExplanationState()))
    assertEquals(
        DeclarationActionPresentation(ActionTone.Destructive, ActionTone.Neutral),
        declarationActionPresentation(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Loading)))
    assertEquals(
        DeclarationActionPresentation(ActionTone.Neutral, ActionTone.Primary),
        declarationActionPresentation(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Current)))
    assertEquals(
        DeclarationActionPresentation(ActionTone.Primary, ActionTone.Neutral),
        declarationActionPresentation(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Stale)))
  }

  @Test
  fun declarationExplanationShowsOnlyItsSummaryAndUpdatesWithTheResponse() {
    val result =
        DeclarationExplanation(
            version = "v1",
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "hash",
            anchor = DeclarationSourceAnchor("internal/main.go", "Run", "func Run() error", 4, 9),
            summary = "**Validate** the request before dispatching work.",
            behavior = listOf("Read `request.ID`.", "Dispatch valid requests."),
            inputs = listOf("The incoming request."),
            errorBehavior = listOf("Missing input returns an error."),
            contextManifest = ContextManifest(model = "model-a", providerOrigin = "local-provider"))
    var state by
        mutableStateOf(
            DeclarationExplanationState(
                status = DeclarationExplanationStatus.Current, result = result))
    ComposeVisualFixture(360, 400, 1.5f) {
          Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
            DeclarationExplanationDetails(state)
          }
        }
        .use { fixture ->
          fixture.render("context-structured-explanation-360-1.5")
          listOf(
                  "Summary",
                  "Behavior",
                  "Inputs",
                  "Error behavior",
                  "Read request.ID.",
                  "Dispatch valid requests.",
                  "Missing input returns an error.")
              .forEach { assertFalse(fixture.hasText(it), it) }
          fixture.assertTextWrapsWithoutClipping("Validate the request before dispatching work.")
          assertFalse(fixture.hasText("Outputs"))
          assertFalse(fixture.hasText("model-a"))
          assertEquals(1, fixture.scrollableContentCount())
          assertFalse(fixture.hasText("Explanation source"))
          assertFalse(fixture.hasText("Current explanation"))
          state = state.copy(result = result.copy(summary = "Replacement explanation."))
          fixture.render()
          assertFalse(fixture.hasText("model-a"))
          assertTrue(fixture.hasText("Replacement explanation."))
        }
  }

  @Test
  fun declarationIdentityIsBoundedSelectableAndBlockedTargetsRemainInspectable() {
    val longPath = "internal/" + "nested/".repeat(18) + "handler.go"
    val signature = "func Run(" + "requestID string, ".repeat(12) + ") error"
    val loaded = file().copy(path = longPath, content = (1..6).joinToString("\n") { "line $it" })
    val exact = symbol().copy(signature = signature, startLine = 4, endLine = 900)
    val approximate = exact.copy(confidence = "approximate", atomicTarget = false)
    val grouped = exact.copy(atomicTarget = false)
    val cases =
        listOf(
            Triple(loaded, exact, "Exact atomic target · editable"),
            Triple(loaded, approximate, "Approximate indexed declaration · read-only"),
            Triple(loaded, grouped, "Exact indexed declaration · read-only"),
            Triple(
                loaded.copy(language = "Python"), exact, "Exact indexed declaration · read-only"))
    var calls = 0
    val actions =
        ContextToolWindowActions(
            {}, { calls++ }, {}, {}, { calls++ }, explainSelected = { calls++ })
    cases.forEach { (source, target, confidence) ->
      val inspector =
          symbolInspectorUiState(
              source,
              listOf(target),
              target,
              null,
              false,
              InspectorProviderState(false, false),
              null)!!
      ComposeVisualFixture(280, 400, 1.5f) {
            ContextToolWindow(
                ContextToolWindowState(inspector, ScopedModel(), false, null, null), actions)
          }
          .use { fixture ->
            fixture.render()
            fixture.assertTextWrapsWithoutClipping(longPath)
            fixture.assertTextFits("Lines 4–6 · ${source.language}")
            assertTrue(fixture.hasText(confidence))
            assertTrue(fixture.hasText("Source/index comparison unavailable"))
            if (!inspector.selectedSymbol!!.editEligibility.eligible) {
              assertTrue(fixture.hasText(inspector.selectedSymbol.editEligibility.blockedReason))
              assertFalse(fixture.hasText("Refactor"))
            }
            fixture.clickText("Declaration details")
            fixture.render()
            assertTrue(fixture.hasText(signature))
          }
    }
    val missing =
        symbolInspectorUiState(
            loaded, emptyList(), exact, null, false, InspectorProviderState(false, false), null)!!
    ComposeVisualFixture(320, 400, 1f) {
          ContextToolWindow(
              ContextToolWindowState(missing, ScopedModel(), false, null, null), actions)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(exact.name))
          assertTrue(fixture.hasText(missing.selectedSymbol!!.editEligibility.blockedReason))
          assertFalse(fixture.hasText("Refactor"))
          fixture.clickText("Declaration details")
          fixture.render()
          assertTrue(fixture.hasText(signature))
        }
    assertEquals(0, calls)
  }

  @Test
  fun declarationDetailsResetOnProjectChangeEvenWithTheSameFileAndSymbol() {
    val source = file()
    val target = symbol()
    val inspector = inspector(source, selectedSymbol = target)
    val project =
        ProjectAnalysis(
            "first",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 18,
            summary = "",
            aiStatus = "missing",
            analyzedAt = "")
    var state by
        mutableStateOf(
            ContextToolWindowState(inspector, ScopedModel(), false, null, null, project = project))
    var calls = 0
    val actions =
        ContextToolWindowActions(
            {}, { calls++ }, {}, {}, { calls++ }, explainSelected = { calls++ })
    ComposeVisualFixture(320, 400, 1f) { ContextToolWindow(state, actions) }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
          fixture.clickText("Declaration details")
          fixture.render()
          assertTrue(fixture.hasText(target.signature))
          fixture.render() // Recomposition with the same identity keeps the disclosure open.
          assertTrue(fixture.hasText(target.signature))
          state = state.copy(project = project.copy(projectId = "second"))
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
          fixture.clickText("Declaration details")
          fixture.render()
          assertTrue(fixture.hasText(target.signature))
          state = state.copy(project = project.copy(projectRevision = "new-revision"))
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
          fixture.clickText("Declaration details")
          fixture.render()
          state =
              state.copy(inspector = inspector.copy(file = source.copy(contentHash = "changed")))
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
        }
    assertEquals(0, calls)
  }

  @Test
  fun fileFallbackKeepsActionsAndDetailsWithoutMisreportingUnselectedSymbols() {
    var calls = 0
    val actions =
        ContextToolWindowActions(
            {}, { calls++ }, {}, {}, { calls++ }, explainSelected = { calls++ })
    val noSymbols = "No indexed declarations in this file. File details remain available."
    val unselected = "Select a declaration in the editor to inspect it."
    listOf(emptyList(), listOf(symbol())).forEach { symbols ->
      val inspector =
          symbolInspectorUiState(
              file(), symbols, null, null, false, InspectorProviderState(false, false), null)!!
      ComposeVisualFixture(320, 400, 1f) {
            ContextToolWindow(
                ContextToolWindowState(inspector, ScopedModel(), false, null, null), actions)
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("Actions"))
            assertTrue(fixture.hasText("Analyze project"))
            assertTrue(fixture.hasText(if (symbols.isEmpty()) noSymbols else unselected))
            assertFalse(fixture.hasText(if (symbols.isEmpty()) unselected else noSymbols))
            fixture.clickText("Details")
            fixture.render()
            assertTrue(fixture.hasText(file().path))
            assertTrue(fixture.hasText("Source/index comparison unavailable"))
            assertFalse(fixture.hasText("Explain declaration"))
          }
    }
    assertEquals(0, calls)
  }

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

  @Test
  fun contextProjectSummaryUsesTheSharedPurposeAndFreshnessPresentation() {
    val overview =
        ProjectOverview(
            analysis =
                StructuredProjectAnalysis(
                    status = "stale", purpose = "Keep request boundaries explicit."))

    val summary = projectSummaryPresentation(overview, null)

    assertEquals("Keep request boundaries explicit.", summary.purpose)
    assertEquals("stale", summary.analysisStatus)
    assertTrue(summary.analysisMessage.contains("source may have changed"))
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
