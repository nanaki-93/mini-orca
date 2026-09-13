package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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

class ContextToolWindowTest {
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
  fun declarationExplanationPresentationDistinguishesLifecycleAndKeepsFactsBounded() {
    assertEquals("Explain declaration", explanationActionLabel(DeclarationExplanationState()))
    assertEquals(
        "Cancel explanation",
        explanationActionLabel(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Loading)))
    assertEquals(
        "Refresh explanation",
        explanationActionLabel(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Current)))
    val facts =
        explanationFacts(
            DeclarationExplanation(
                version = "v1",
                projectId = "project",
                projectRevision = "revision",
                baseFileHash = "hash",
                anchor =
                    DeclarationSourceAnchor(
                        "main.go", "Run", "func Run()", startLine = 2, endLine = 4),
                summary = "Runs.",
                behavior = listOf("dispatches work"),
                sideEffects = listOf("writes output"),
                contextManifest = ContextManifest()))

    assertEquals("Behavior", facts.first().first)
    assertEquals(listOf("dispatches work"), facts.first().second)
    assertEquals(listOf("writes output"), facts[3].second)
    assertEquals(
        listOf("Scope" to "Function", "Model" to "model-a", "Provider" to "http://127.0.0.1:8080"),
        explanationProvenanceFacts(
            ContextManifest(model = "model-a", providerOrigin = "http://127.0.0.1:8080")))
    assertEquals(listOf("Scope" to "Function"), explanationProvenanceFacts(ContextManifest()))
  }

  @Test
  fun explanationFactsStaySeparateAndSourceDisclosureIsLocalAndResetsWithTheResponse() {
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
    ComposeVisualFixture(360, 1100, 1.5f) {
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
              .forEach { assertTrue(fixture.hasText(it), it) }
          fixture.assertTextWrapsWithoutClipping("Validate the request before dispatching work.")
          assertFalse(fixture.hasText("Outputs"))
          assertFalse(fixture.hasText("model-a"))
          assertEquals(1, fixture.scrollableContentCount())
          assertTrue(fixture.requestFocus("Explanation source"))
          fixture.pressKey(Key.Enter)
          fixture.render("context-explanation-source-360-1.5")
          assertEquals("Expanded", fixture.stateDescription("Explanation source"))
          assertTrue(fixture.hasText("model-a"))
          assertTrue(fixture.hasText("local-provider"))
          state = state.copy(result = result.copy(summary = "Replacement explanation."))
          fixture.render()
          assertEquals("Collapsed", fixture.stateDescription("Explanation source"))
          assertFalse(fixture.hasText("model-a"))
          assertTrue(fixture.hasText("Replacement explanation."))
        }
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
