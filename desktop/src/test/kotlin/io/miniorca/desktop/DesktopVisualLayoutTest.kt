package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.InternalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asComposeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.platform.PlatformContext
import androidx.compose.ui.scene.CanvasLayersComposeScene
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsNode
import androidx.compose.ui.semantics.SemanticsOwner
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import kotlinx.coroutines.Dispatchers
import org.jetbrains.skia.Surface

/** Renders production components with explicit test data, without a daemon or provider. */
class DesktopVisualLayoutTest {
  @Test
  fun finalLifecycleMatrixUsesProductionPanesAtLargeText() {
    acceptanceRunStates.forEach { status ->
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(
                    resultProjectFixture(), ProjectAnalysisRunState(run = acceptanceRun(status))),
                AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
          }
          .use { fixture ->
            fixture.render("final-progress-$status-800-150")
            assertTrue(fixture.hasText("Analysis"))
          }
      listOf("bugs", "performance", "security").forEach { category ->
        ComposeVisualFixture(800, 650, 1.5f) { AcceptanceResultPane(category, status) }
            .use { fixture ->
              fixture.render("final-$category-$status-800-150")
              fixture.assertTextFits("View analysis")
              assertTrue(fixture.hasText(acceptanceResultPage(category, status).statusLabel))
              assertFalse(fixture.hasText("Start analysis"))
            }
      }
    }
  }

  @Test
  fun analysisProgressAndResultLinksRemainReadableAcrossSupportedViewports() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1280, 600, 1.25f),
            Triple(1280, 600, 1.5f))
        .forEach { (width, height, scale) ->
          var navigations = 0
          val run =
              analysisRunFixture()
                  .copy(
                      status = "running",
                      files =
                          listOf(
                              AnalysisRunFile(
                                  "internal/platform/transport/handlers/main.go",
                                  "base",
                                  "Go",
                                  listOf(AnalysisStageProgress("semantic", "running", 1, false)))))
          ComposeVisualFixture(width, height, scale) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(),
                        ProjectAnalysisRunState(
                            run = run, fileSelection = AnalysisSelectionState(selectionFixture()))),
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, { navigations++ }))
              }
              .use { fixture ->
                fixture.render("analysis-progress-$width-$scale")
                listOf("Pause", "Cancel", "Bugs", "Performance", "Security")
                    .forEach(fixture::assertTextFits)
                assertFalse(fixture.hasText("Prepare fix"))
                fixture.clickDescription("View Bugs results")
                assertEquals(1, navigations)
              }
        }
  }

  @Test
  fun analysisLifecycleAndOperationalErrorsStayExplicit() {
    listOf(
            "paused" to "Resume",
            "interrupted" to "Resume",
            "stale" to "Start analysis",
            "failed" to "Start analysis",
            "canceled" to "Start analysis")
        .forEach { (status, control) ->
          ComposeVisualFixture(800, 650, 1.5f) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(),
                        ProjectAnalysisRunState(
                            run = analysisRunFixture().copy(status = status),
                            error =
                                "Could not reach the daemon. Retry when the local service is available. Completed results remain available in their sections; no analysis request was retried.")),
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
              }
              .use { fixture ->
                fixture.render("analysis-$status-800-1.5")
                fixture.assertTextFits(control)
                fixture.assertTextWrapsWithoutClipping(
                    "Could not reach the daemon. Retry when the local service is available. Completed results remain available in their sections; no analysis request was retried.")
              }
        }
    var analysis by
        mutableStateOf(ProjectAnalysisRunState(run = analysisRunFixture().copy(status = "running")))
    var pauses = 0
    ComposeVisualFixture(800, 650) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(resultProjectFixture(), analysis),
              AnalysisWorkspaceActions(
                  { _, _ -> },
                  {
                    pauses++
                    analysis = analysis.copy(action = "pausing")
                  },
                  {},
                  {},
                  {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Pause")
          fixture.render()
          assertEquals(1, pauses)
          assertTrue(fixture.isDisabled("Pause"))
        }
  }

  @Test
  fun resultPagesUseReadableRowsAndLocalDrilldownWithoutModelActions() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1280, 600, 1.25f),
            Triple(1280, 600, 1.5f))
        .forEach { (width, height, scale) ->
          listOf("bugs", "performance", "security").forEach { category ->
            var externalActions = 0
            val title =
                when (category) {
                  "bugs" -> "Return the missing error"
                  "performance" -> "Avoid repeated allocation"
                  else -> "Credential-like assignment"
                }
            val original =
                when (category) {
                  "performance" -> performancePageFixture()
                  "security" -> securityPageFixture()
                  else ->
                      resultPageFixture("bugs").let { page ->
                        page.copy(
                            section =
                                page.section.copy(
                                    results =
                                        page.results!!.copy(
                                            semantic =
                                                page.semantic.map {
                                                  it.copy(
                                                      title = title,
                                                      severity = "high",
                                                      message =
                                                          "Return the error before processing the next request.",
                                                      source = "analysis",
                                                      confidence = "suggested",
                                                      freshness = "fresh",
                                                      status = "open")
                                                })))
                      }
                }
            var page by
                mutableStateOf(original.copy(section = AnalysisSectionState(loading = true)))
            val findingActions =
                FindingActions(
                    { externalActions++ }, { externalActions++ }, { _, _ -> externalActions++ })
            ComposeVisualFixture(width, height, scale) {
                  when (category) {
                    "performance" ->
                        PerformanceWorkspacePane(
                            PerformanceWorkspacePaneState(page, resultIndexFixture()),
                            PerformanceWorkspaceActions(
                                { _, _ -> externalActions++ },
                                { _, _ -> externalActions++ },
                                { externalActions++ },
                                findingActions))
                    "security" ->
                        SecurityWorkspacePane(
                            SecurityWorkspacePaneState(page, resultIndexFixture()),
                            SecurityWorkspaceActions(
                                { externalActions++ },
                                { externalActions++ },
                                { externalActions++ },
                                findingActions))
                    else ->
                        BugsWorkspacePane(
                            BugsWorkspacePaneState(page.semantic, null, false, page),
                            BugsWorkspaceActions(
                                findingActions,
                                { externalActions++ },
                                { externalActions++ },
                                { externalActions++ }))
                  }
                }
                .use { fixture ->
                  fixture.render()
                  assertTrue(fixture.hasText("Loading results…"))
                  assertFalse(fixture.hasDescription("Inspect $title"))
                  page = original
                  fixture.render("results-$category-$width-$scale")
                  fixture.assertTextFits("View analysis")
                  fixture.assertTextFits(title)
                  assertFalse(
                      fixture.hasEditableText(), "Result pages must show the unfiltered list")
                  assertFalse(fixture.hasText("Start analysis"))
                  assertFalse(fixture.hasText("Review"))
                  fixture.clickDescription("Inspect $title")
                  fixture.render("results-$category-detail-$width-$scale")
                  assertTrue(
                      fixture.hasText(if (width < 900) "Back to results" else "Clear selection"))
                  assertTrue(
                      fixture.hasText(
                          if (category == "performance") "Prepare optimization"
                          else if (category == "security") "Prepare fix" else "Open source"))
                  assertEquals(0, externalActions)
                  fixture.clickText(if (width < 900) "Back to results" else "Clear selection")
                  fixture.render()
                  assertTrue(fixture.hasDescription("Inspect $title"))
                  fixture.clickDescription("Inspect $title")
                  val nextRun =
                      original.run!!.copy(identity = original.run.identity.copy(id = "next-run"))
                  page =
                      original.copy(
                          run = nextRun,
                          section =
                              original.section.copy(
                                  results = original.results!!.copy(identity = nextRun.identity)))
                  fixture.render()
                  assertTrue(fixture.hasDescription("Inspect $title"))
                  assertFalse(fixture.hasText("Back to results"))
                  assertFalse(fixture.hasText("Clear selection"))
                  assertEquals(0, externalActions)
                }
          }
        }
  }

  @Test
  fun stalePartialEmptyAndHistoricalEvidenceRemainDistinct() {
    val base = performancePageFixture()
    val stale =
        base.copy(
            run = base.run!!.copy(status = "stale"),
            section =
                base.section.copy(error = "The daemon is unavailable; retained evidence is shown."))
    ComposeVisualFixture(800, 650, 1.25f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(stale, resultIndexFixture()),
              PerformanceWorkspaceActions(
                  { _, _ -> }, { _, _ -> }, {}, FindingActions({}, {}, { _, _ -> })))
        }
        .use { fixture ->
          fixture.render("results-stale-error-800-1.25")
          assertTrue(fixture.hasText("Reported findings: Not available"))
          fixture.clickDescription("Inspect Avoid repeated allocation")
          fixture.render()
          assertTrue(fixture.isDisabled("Prepare optimization"))
          assertTrue(fixture.hasText("Stale · Not measured"))
        }
    val empty = resultPageFixture("bugs").copy(run = null, section = AnalysisSectionState())
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(emptyList(), null, false, empty),
              BugsWorkspaceActions(FindingActions({}, {}, { _, _ -> }), {}, {}))
        }
        .use { fixture ->
          fixture.render("results-empty-800-1.5")
          assertTrue(fixture.hasText("Reported findings: Not available"))
          assertTrue(fixture.hasText("No findings yet."))
        }
    ComposeVisualFixture(800, 650) {
          Column {
            PreviousAnalysisDetails(
                listOf(
                    UnifiedFinding(
                        title = "Previous uncategorized risk",
                        message = "Historical prose retained for inspection.")))
          }
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("Previous uncategorized risk"))
          fixture.clickText("Previous analysis · unclassified")
          fixture.render("results-history-800")
          assertTrue(fixture.hasText("Previous uncategorized risk"))
        }
  }

  @Test
  fun formattedResponseListsUseOnlyTheOriginalLineBreaks() {
    val response = "Summary.\n\n- First item\n- Second item\n\nNext paragraph.\n1. Last item"
    ComposeVisualFixture(480, 600, 1.5f) { ModelResultContent(response) }
        .use { fixture ->
          fixture.render("model-list-spacing-480-1.5")
          fixture.assertTextLineCount(formatModelResult(response).text, 7)
          assertFalse(fixture.hasText("Show full response"))
        }
  }

  @Test
  fun compactFieldKeepsEditingAndItsAccessibleNameAfterInput() {
    var query by mutableStateOf("")
    ComposeVisualFixture(360, 100) {
          CompactSingleLineField(query, { query = it }, "Search findings", showLabel = false)
        }
        .use { fixture ->
          fixture.render()
          fixture.assertTextFits("Search findings")
          fixture.setText("repository")
          fixture.render()
          kotlin.test.assertEquals("repository", query)
          assertTrue(fixture.hasText("repository"))
          assertTrue(fixture.hasDescription("Search findings"))
        }
  }

  @Test
  fun selectedDeclarationShowsDescriptionAndExplicitActionsWithoutDetailTabs() {
    listOf(Triple(280, 600, 1f), Triple(320, 600, 1.5f), Triple(480, 650, 1.25f)).forEach {
        (width, height, scale) ->
      var actions = 0
      var outerSelections = 0
      val initial = contextVisualState()
      var state by mutableStateOf(initial)
      ComposeVisualFixture(width, height, scale) {
            RightToolWindowContainer(
                RightToolWindow.Context,
                { outerSelections++ },
                content = { _, modifier ->
                  ContextToolWindow(
                      state,
                      ContextToolWindowActions(
                          { actions++ },
                          { actions++ },
                          { actions++ },
                          { actions++ },
                          { actions++ },
                          explainSelected = { actions++ }),
                      modifier)
                })
          }
          .use { fixture ->
            fixture.render("context-description-$width-$scale")
            listOf("Context", "Assistant", "Review", "Explain declaration", "Refactor")
                .forEach(fixture::assertTextFits)
            assertTrue(fixture.hasText("Cached declaration explanation"))
            listOf(
                    "Actions",
                    "Explain",
                    "Details",
                    "File analysis",
                    "Project context",
                    "func Run() error")
                .forEach { assertFalse(fixture.hasText(it), it) }
            assertEquals(0, actions)
            assertEquals(0, outerSelections)
            assertTrue(fixture.requestFocus("Explain declaration"))
            fixture.clickText("Explain declaration")
            assertEquals(1, actions)
            state =
                state.copy(
                    inspector =
                        state.inspector!!.copy(
                            selectedSymbol =
                                state.inspector!!.selectedSymbol!!.let {
                                  it.copy(
                                      symbol = it.symbol.copy(name = "Stop"),
                                      explanation = "Stops the worker.")
                                }))
            fixture.render("context-new-selection-$width-$scale")
            assertTrue(fixture.hasText("Stop"))
            assertTrue(fixture.hasText("Stops the worker."))
            assertFalse(fixture.hasText("Cached declaration explanation"))
            assertTrue(fixture.hasText("Refactor"))
            assertFalse(fixture.hasText("Explanation needs refresh"))
            fixture.clickText("Refactor")
            assertEquals(2, actions)
          }
    }
  }

  @Test
  fun declarationDescriptionPreservesConsentCancellationAndLifecycleEvidence() {
    var requests = 0
    var cancellations = 0
    var state by
        mutableStateOf(
            contextVisualState()
                .copy(
                    functionModel = ScopedModel(scope = "function", remoteProvider = true),
                    declarationExplanation = DeclarationExplanationState()))
    val result =
        DeclarationExplanation(
            version = "v1",
            projectId = "visual-fixture",
            projectRevision = "fixture-revision",
            baseFileHash = "fixture-hash",
            anchor =
                DeclarationSourceAnchor("internal/api/user.go", "Run", "func Run() error", 5, 12),
            summary = "Validates the request before dispatching work.",
            contextManifest = ContextManifest())
    ComposeVisualFixture(360, 650, 1.5f) {
          ContextToolWindow(
              state,
              ContextToolWindowActions(
                  {},
                  {},
                  {},
                  {},
                  {},
                  explainSelected = { requests++ },
                  cancelExplanation = { cancellations++ }),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          fixture.render("context-explain-consent")
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertTrue(fixture.isDisabled("Explain declaration"))
          assertTrue(fixture.hasText("Cached declaration explanation"))
          state = state.copy(functionRemoteProviderConfirmed = true)
          fixture.render()
          fixture.clickText("Explain declaration")
          assertEquals(1, requests)
          state =
              state.copy(
                  functionRemoteProviderConfirmed = false,
                  declarationExplanation =
                      DeclarationExplanationState(status = DeclarationExplanationStatus.Loading))
          fixture.render("context-explain-loading")
          fixture.assertTextFits("Cancel explanation")
          assertFalse(fixture.isDisabled("Cancel explanation"))
          fixture.clickText("Cancel explanation")
          assertEquals(1, cancellations)
          state =
              state.copy(
                  functionRemoteProviderConfirmed = true,
                  declarationExplanation =
                      DeclarationExplanationState(
                          status = DeclarationExplanationStatus.Current, result = result))
          fixture.render("context-explain-current")
          assertTrue(fixture.hasText(result.summary))
          assertFalse(fixture.hasText("Current explanation"))
          state =
              state.copy(
                  declarationExplanation =
                      state.declarationExplanation.copy(
                          status = DeclarationExplanationStatus.Stale))
          fixture.render("context-explain-stale")
          assertFalse(fixture.hasText(result.summary))
          fixture.assertTextFits("Explanation needs refresh")
          state =
              state.copy(
                  declarationExplanation =
                      state.declarationExplanation.copy(
                          status = DeclarationExplanationStatus.Failed,
                          message =
                              "The provider could not complete the explanation. Retry after reconnecting to the local service."))
          fixture.render("context-explain-failed")
          fixture.assertTextWrapsWithoutClipping(state.declarationExplanation.message)
          assertFalse(fixture.hasText(result.summary))
          state =
              state.copy(
                  declarationExplanation =
                      state.declarationExplanation.copy(
                          status = DeclarationExplanationStatus.Canceled))
          fixture.render("context-explain-canceled")
          fixture.assertTextFits("Explanation canceled")
          assertEquals(1, requests)
        }
  }

  @Test
  fun editorComponentsRenderAtDockedAndDrawerWidths() {
    listOf(1440 to 900, 1000 to 760, 999 to 760, 800 to 650, 1280 to 600).forEach { (width, height)
      ->
      ComposeVisualFixture(width, height) { EditorVisualFixture(width.toFloat()) }
          .use { fixture ->
            fixture.render("editor-$width")
            assertTrue(fixture.hasDescription("Performance tool window, not selected"))
            assertFalse(fixture.hasText("Performance"))
            fixture.assertTextFits("user.go")
            if (width >= 1_000) {
              fixture.assertTextFits("Files")
              assertEquals(1, fixture.textCount("Files"))
              assertEquals(0, fixture.textCount("Tool windows"))
            } else {
              fixture.assertTextFits("Files")
              fixture.assertTextFits("Context")
            }
          }
    }

    val longPath = "src/platform/transport/http/handlers/user_handler.go"
    val longFile =
        ProjectFileInfo(
            path = longPath,
            contentHash = "long-path-fixture",
            name = "user_handler.go",
            language = "Go",
            sizeBytes = 20,
            lineCount = 1,
            modifiedAt = "",
            binary = false,
            content = "func ServeUser() {}",
        )
    val longChrome =
        editorChromeUiState(
            longFile,
            SymbolInfo(
                "ServeUser",
                "function",
                startLine = 1,
                endLine = 1,
                confidence = "exact",
                atomicTarget = true),
            EditorSurface.Source,
            EditorProgressUiState(EditorProgress.Inspect, ""),
            null,
        )
    ComposeVisualFixture(480, 240, 1.3f) {
          EditorWorkspace(
              longChrome, null, {}, {}, canvas = { DiffViewer(null, Modifier.fillMaxSize()) })
        }
        .use { fixture ->
          fixture.render("editor-breadcrumbs-deep-480-1.3")
          listOf("Source · user_handler.go", "src", "…", "user_handler.go", "ServeUser")
              .forEach(fixture::assertTextFits)
          assertTrue(fixture.hasDescription("Project-relative path: $longPath"))
          assertTrue(fixture.hasText("Composed diff unavailable"))
        }
  }

  @Test
  fun keyboardEventsNavigateAndActivateTheProductionRailAndCommandPalette() {
    var activeToolWindow by mutableStateOf(LeftToolWindow.Summary)
    val railFocus = FocusRequester()
    ComposeVisualFixture(48, 650, 1.3f) {
          ToolWindowBar(
              activeToolWindow, { activeToolWindow = it }, Modifier.focusRequester(railFocus))
        }
        .use { fixture ->
          fixture.render("rail-keyboard-initial-48-1.3")
          railFocus.requestFocus()
          fixture.render()
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render("rail-keyboard-arrow-48-1.3")
          assertTrue(fixture.hasDescription("Analysis tool window, not selected, focused"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render("rail-keyboard-activated-48-1.3")
          kotlin.test.assertEquals(LeftToolWindow.Analysis, activeToolWindow)
          assertTrue(fixture.hasDescription("Analysis tool window, selected, focused"))
        }

    var query by mutableStateOf("")
    val openedFiles = mutableListOf<String>()
    ComposeVisualFixture(800, 650, 1.3f) {
          Box(Modifier.fillMaxSize()) {
            CommandPaletteDialog(
                mode = PaletteMode.Files,
                query = query,
                onQuery = { query = it },
                files =
                    listOf(
                        IndexedFile("internal/alpha.go", "alpha", "Go", false),
                        IndexedFile("internal/zeta.go", "zeta", "Go", false)),
                symbols = emptyList(),
                hasActiveFile = false,
                onSelectFile = { openedFiles += it },
                onSelectSymbol = {},
                onSelectAction = {},
                onDismiss = {})
          }
        }
        .use { fixture ->
          fixture.render("palette-files-initial-800-1.3")
          assertTrue(fixture.isFocused("Filter files"))
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render("palette-files-arrow-800-1.3")
          assertTrue(fixture.hasDescription("File internal/zeta.go, selected"))
          assertTrue(fixture.pressKey(Key.Enter))
          kotlin.test.assertEquals(listOf("internal/zeta.go"), openedFiles)
        }
  }

  @Test
  fun candidateAndReviewSurfacesKeepEvidenceAndMutationGuardsExplicit() {
    val project =
        ProjectAnalysis(
            projectId = "fixture-project",
            projectRevision = "fixture-revision",
            name = "fixture",
            path = "/fixture",
            type = "Go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 12,
            summary = "Fixture project",
            aiStatus = "fresh",
            analyzedAt = "")
    val file =
        ProjectFileInfo(
            path = "internal/api/server.go",
            contentHash = "base-hash",
            name = "server.go",
            language = "Go",
            sizeBytes = 256,
            lineCount = 12,
            modifiedAt = "",
            binary = false,
            content = "package api\n\nfunc Serve() {}")
    val symbol =
        SymbolInfo(
            "Serve",
            "function",
            startLine = 3,
            endLine = 3,
            confidence = "exact",
            atomicTarget = true)
    val draft =
        DeclarationDraft(
            id = "fixture-draft",
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            baseFileHash = file.contentHash,
            targetPath = file.path,
            mode = "replace_symbol",
            targetSymbol = symbol.name,
            declaration = "func Serve() {\\n  handle()\\n}",
            revision = 1,
            hash = "draft-hash",
            validation =
                DeclarationValidation(
                    applicable = true,
                    scopeMode = "replace_symbol",
                    diff =
                        UnifiedDiff(
                            file.path,
                            file.path,
                            listOf(
                                DiffLine("removed", oldLine = 3, text = "func Serve() {}"),
                                DiffLine("added", newLine = 3, text = "func Serve() {"),
                                DiffLine("added", newLine = 4, text = "  handle()"),
                                DiffLine("added", newLine = 5, text = "}"))),
                ))
    val chrome =
        editorChromeUiState(
            file,
            symbol,
            EditorSurface.Source,
            EditorProgressUiState(EditorProgress.Review, ""),
            draft)
    val selectedSurfaces = mutableListOf<EditorSurface>()
    ComposeVisualFixture(800, 480, 1.3f) {
          EditorWorkspace(
              chrome,
              draft,
              { selectedSurfaces += it },
              onCreateDeclaration = {},
              canvas = {
                SourceEditorPane(project, file, listOf(symbol), symbol, 3, emptyList(), {})
              })
        }
        .use { fixture ->
          fixture.render("editor-candidate-800-1.3")
          assertTrue(fixture.hasText("Candidate"))
          assertTrue(fixture.hasText("VALIDATED DRAFT · 4 changed lines · focused checks pending"))
          fixture.clickText("Open review")
          kotlin.test.assertEquals(listOf(EditorSurface.Review), selectedSurfaces)
        }

    val failedChecks =
        DraftCheckReport(
            targetPath = file.path,
            applicable = true,
            checks =
                listOf(
                    DraftCheck(
                        name = "go test",
                        required = true,
                        state = "failed",
                        command = listOf("go", "test", "./..."),
                        output = "expected failure evidence")),
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash)
    val invalidDraft =
        draft.copy(
            validation =
                DeclarationValidation(
                    applicable = false,
                    scopeMode = "replace_symbol",
                    diagnostics = listOf(DeclarationFinding("syntax", "missing closing brace")),
                    diff = requireNotNull(draft.validation).diff))
    var reviewActions = 0
    var mutations = 0
    val invalidReviewState =
        ReviewToolWindowState(
            project,
            file,
            symbol,
            null,
            editableDraft(invalidDraft),
            invalidDraft,
            failedChecks,
            null,
            null,
            null,
            false)
    ComposeVisualFixture(800, 700, 1.3f) {
          ReviewToolWindow(
              invalidReviewState,
              ReviewToolWindowActions(
                  { reviewActions++ }, { reviewActions++ }, { reviewActions++ }),
              DraftApplicationActions({ mutations++ }, { mutations++ }))
        }
        .use { fixture ->
          fixture.render("review-invalid-draft-800-1.3")
          assertTrue(fixture.hasText("Progress"))
          assertTrue(fixture.hasText("Next action"))
          assertTrue(fixture.hasText("missing closing brace"))
          assertFalse(fixture.hasDescription("Expand Validation diagnostics"))
          fixture.clickText("Failed check details")
          fixture.render("review-invalid-draft-details-800-1.3")
          assertTrue(fixture.hasText("expected failure evidence"))
          kotlin.test.assertEquals(0, reviewActions)
          kotlin.test.assertEquals(0, mutations)
        }

    val readyChecks =
        failedChecks.copy(checks = listOf(DraftCheck("go test", required = true, state = "passed")))
    val boundSession =
        ChatSession(
            id = "fixture-session",
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            openPath = draft.targetPath,
            mode = draft.mode,
            targetSymbol = draft.targetSymbol,
            state = "active",
            latestDraftId = draft.id,
        )
    listOf(1440 to 900, 999 to 760, 800 to 700).forEach { (width, height) ->
      val scale = if (width == 800) 1.3f else 1f
      ComposeVisualFixture(width, height, scale) {
            ReviewToolWindow(
                invalidReviewState.copy(
                    session = boundSession,
                    editor = editableDraft(draft),
                    draft = draft,
                    checks = readyChecks),
                ReviewToolWindowActions({}, {}, {}),
                DraftApplicationActions({ mutations++ }, { mutations++ }))
          }
          .use { fixture ->
            fixture.render("review-ready-$width-${height}-$scale")
            assertTrue(fixture.hasText("Passed"))
            assertTrue(fixture.hasText("The request is bound to this candidate."))
            assertTrue(fixture.hasText("Next action"))
            assertTrue(fixture.hasText("Apply Serve to internal/api/server.go"))
            fixture.clickText("Focused check details")
            fixture.render("review-ready-details-$width-${height}-$scale")
            assertTrue(fixture.hasText("Candidate hash: draft-hash"))
            assertTrue(fixture.hasText("Check identity hash: draft-hash"))
            kotlin.test.assertEquals(0, mutations)
          }
    }

    ComposeVisualFixture(800, 360, 1.3f) {
          ReviewToolWindow(
              invalidReviewState.copy(
                  checks = null,
                  applied =
                      ApplyResult(
                          "next",
                          "post-hash",
                          true,
                          AuditEntry("apply", file.path, "applied", ""))),
              ReviewToolWindowActions({}, {}, {}),
              DraftApplicationActions({ mutations++ }, { mutations++ }))
        }
        .use { fixture ->
          fixture.render("review-receipt-800-1.3")
          assertTrue(fixture.hasText("Change applied"))
          assertTrue(fixture.hasText("Undo available"))
          assertTrue(fixture.hasText("Undo this change"))
          kotlin.test.assertEquals(0, mutations)
        }

    var contextActions = 0
    val inspector =
        requireNotNull(
            symbolInspectorUiState(
                selectedFile = file,
                symbols = listOf(symbol),
                selectedSymbol = null,
                analysis = FileAnalysis(file.path, "stale", purpose = "Routes incoming requests."),
                analysisInProgress = false,
                provider =
                    InspectorProviderState(remoteProvider = true, remoteProviderConfirmed = false),
                currentEditIdentity = null))
    ComposeVisualFixture(800, 900, 1.3f) {
          ContextToolWindow(
              ContextToolWindowState(
                  inspector,
                  ScopedModel(
                      scope = ModelScope.Bug.wireValue,
                      profile = "review-profile",
                      model = "provider/analyzer",
                      remoteProvider = true),
                  false,
                  null,
                  null,
                  FileAnalysis(file.path, "stale", purpose = "Routes incoming requests."),
                  project,
                  null),
              ContextToolWindowActions(
                  { contextActions++ },
                  { contextActions++ },
                  { contextActions++ },
                  { contextActions++ },
                  { contextActions++ }),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("context-no-symbol-800-1.3")
          assertTrue(fixture.hasText("File analysis"))
          assertFalse(fixture.hasText("Routes incoming requests."))
          assertTrue(fixture.hasText("Actions"))
          assertTrue(fixture.hasText("Analyze project"))
          assertFalse(fixture.hasText("Confirm remote destination"))
          assertFalse(fixture.hasText("Generate unit test"))
          assertFalse(fixture.hasText("Complexity and readability scores unavailable."))
          kotlin.test.assertEquals(0, contextActions)
        }

    var assistantActions = 0
    val request = "Keep **literal** request text and validate the input."
    val response =
        "**Plan:** validate the input before calling `Serve`.\n\n" +
            (1..12).joinToString("\n") { "- Preserve requirement $it." }
    val session =
        ChatSession(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            baseFileHash = file.contentHash,
            openPath = file.path,
            mode = draft.mode,
            targetSymbol = draft.targetSymbol,
            state = "active",
            latestDraftId = draft.id,
            messages =
                listOf(
                    ChatSessionMessage("user", request), ChatSessionMessage("assistant", response)))
    ComposeVisualFixture(800, 900, 1.3f) {
          AssistantToolWindow(
              AssistantToolWindowState(
                  project,
                  file,
                  session,
                  invalidDraft,
                  editableDraft(invalidDraft),
                  ChatTarget(ChatEditMode.ReplaceSymbol, symbol.name),
                  ChatEditMode.ReplaceSymbol,
                  "",
                  "Check this declaration.",
                  false,
                  ScopedModel(
                      scope = ModelScope.Function.wireValue,
                      profile = "edit-profile",
                      model = "provider/editor",
                      remoteProvider = true),
                  false,
                  FocusRequester(),
                  FocusRequester()),
              AssistantConversationActions(
                  { assistantActions++ },
                  { assistantActions++ },
                  { assistantActions++ },
                  { assistantActions++ },
                  { assistantActions++ },
                  { assistantActions++ }),
              DraftEditorActions(
                  { assistantActions++ }, { assistantActions++ }, { assistantActions++ }),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("assistant-invalid-800-1.3")
          assertTrue(fixture.hasText("Conversation"))
          assertTrue(fixture.hasText("Editable draft"))
          assertTrue(fixture.hasText("Your request"))
          assertTrue(fixture.hasText("Model response"))
          assertTrue(fixture.hasText(request))
          assertTrue(fixture.hasText(formatModelResult(response).text))
          assertTrue(fixture.hasText("Candidate for review"))
          assertTrue(fixture.hasText("missing closing brace"))
          assertEquals(1, fixture.scrollableContentCount())
          fixture.clickText("Show full response")
          fixture.render("assistant-response-expanded-800-1.3")
          assertEquals("Expanded", fixture.stateDescription("Show less"))
          assertTrue(fixture.hasText("Fix validation diagnostics before continuing."))
          assertTrue(fixture.hasText("Confirm remote destination"))
          kotlin.test.assertEquals(0, assistantActions)
        }

    var remoteConfirmation by mutableStateOf(false)
    ComposeVisualFixture(480, 180, 1.3f) {
          Column(Modifier.fillMaxSize().background(AppBackground).padding(8.dp)) {
            RemoteProviderConfirmation(
                scope = ModelScope.Analyze,
                model =
                    ScopedModel(
                        scope = ModelScope.Analyze.wireValue,
                        profile = "review-profile",
                        model = "provider/reviewer",
                        remoteProvider = true),
                confirmed = remoteConfirmation,
                onConfirmed = { remoteConfirmation = it },
            )
          }
        }
        .use { fixture ->
          fixture.render("remote-consent-unconfirmed-480-1.3")
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertEquals("Not confirmed", fixture.stateDescription("Confirm remote destination"))
          fixture.clickText("Confirm remote destination")
          fixture.render("remote-consent-confirmed-480-1.3")
          assertTrue(remoteConfirmation)
          assertTrue(fixture.hasText("Confirm remote destination · confirmed"))
          assertEquals(
              "Confirmed", fixture.stateDescription("Confirm remote destination · confirmed"))
        }
  }

  @Test
  fun sharedControlsRenderReadableStatesAtEnlargedTextScale() {
    ComposeVisualFixture(720, 180, 1.3f) { SharedControlsVisualFixture() }
        .use { fixture ->
          fixture.render("shared-controls-130")
          listOf("Apply", "Selected", "Disabled", "Focused", "Search files").forEach {
            fixture.assertTextFits(it)
          }
        }
  }

  @Test
  fun functionPresetPreparesAndFocusesTheBoundComposerWithoutSending() {
    val file =
        ProjectFileInfo(
            path = "internal/users.go",
            contentHash = "fixture-hash",
            name = "users.go",
            language = "Go",
            sizeBytes = 120,
            lineCount = 12,
            modifiedAt = "",
            binary = false)
    val symbol =
        SymbolInfo(
            "deduplicateUsers",
            "function",
            "func deduplicateUsers(users []User) []User",
            3,
            10,
            "exact",
            true)
    val target = ChatTarget(ChatEditMode.ReplaceSymbol, symbol.name)
    val focusRequester = FocusRequester()
    var message by mutableStateOf(TextFieldValue())
    var presetCalls = 0
    var sendCalls = 0
    var otherCalls = 0
    ComposeVisualFixture(480, 640, 1.3f) {
          AssistantToolWindow(
              state =
                  AssistantToolWindowState(
                      project = visualFixtureProject,
                      selected = file,
                      session = null,
                      draft = null,
                      editor = null,
                      target = target,
                      mode = ChatEditMode.ReplaceSymbol,
                      newSymbol = "",
                      message = message.text,
                      sending = false,
                      functionModel = ScopedModel(scope = ModelScope.Function.wireValue),
                      remoteConfirmed = false,
                      chatFocus = focusRequester,
                      draftFocus = FocusRequester(),
                      messageInput = message,
                      selectedSymbol = symbol,
                      targetValidation = ChatTargetValidation(target)),
              conversationActions =
                  AssistantConversationActions(
                      updateMessage = { message = TextFieldValue(it) },
                      updateNewSymbol = { otherCalls++ },
                      confirmRemoteProvider = { otherCalls++ },
                      inspectContext = { otherCalls++ },
                      send = { sendCalls++ },
                      cancel = { otherCalls++ },
                      updateMessageValue = { message = it },
                      preparePreset = { preset ->
                        presetCalls++
                        message = preparedFunctionChangeMessage(preset)
                        focusRequester.requestFocus()
                      }),
              editorActions = DraftEditorActions({}, {}, {}),
              modifier = Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render("assistant-function-presets-480-1.3")
          assertTrue(fixture.hasText("Ready for the selected declaration"))
          fixture.clickText("Fix bug")
          fixture.render("assistant-function-preset-prepared-480-1.3")

          assertEquals(1, presetCalls)
          assertEquals(0, sendCalls)
          assertEquals(0, otherCalls)
          assertTrue(fixture.hasText("Fix a bug: "))
          assertTrue(fixture.isDescriptionFocused("Intent"))
          assertTrue(fixture.isDisabled("Send message"))

          fixture.setText("Fix a bug: preserve order while deduplicating")
          fixture.render()
          fixture.clickText("Send message")
          assertEquals(1, sendCalls)

          fixture.clickText("Advanced constraints")
          fixture.render("assistant-function-constraints-480-1.3")
          assertEquals("Expanded", fixture.stateDescription("Advanced constraints"))
          assertTrue(fixture.hasText("Constraints"))
        }
  }

  @Test
  fun sharedChromeKeepsNamedActionsAndDisclosureActivationIndependent() {
    var actionCalls = 0
    var expanded by mutableStateOf(false)
    ComposeVisualFixture(520, 220, 1.5f) {
          Column(Modifier.fillMaxSize().background(AppBackground)) {
            IdeDisclosureHeader(
                title = "Details",
                expanded = expanded,
                onToggle = { expanded = !expanded },
                actions = {
                  ChromeButton(
                      onClick = { actionCalls++ },
                      accessibleName = "Refresh details",
                  ) {
                    Text("Refresh")
                  }
                })
            ChromeButton(onClick = { actionCalls++ }, accessibleName = "Run check") {
              Text("Run check")
            }
            ChromeButton(
                onClick = { actionCalls++ },
                enabled = false,
                accessibleName = "Unavailable check",
            ) {
              Text("Unavailable check")
            }
          }
        }
        .use { fixture ->
          fixture.render("shared-chrome-actions-150")
          assertTrue(fixture.hasDescription("Run check"))
          fixture.assertTextFits("Refresh")
          assertTrue(fixture.requestFocus("Run check"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, actionCalls)
          assertFalse(fixture.tryClick("Unavailable check"))
          assertEquals(1, actionCalls)
          fixture.clickText("Refresh")
          assertEquals(2, actionCalls)
          assertFalse(expanded)
          fixture.clickText("Details")
          fixture.render("shared-chrome-actions-expanded-150")
          assertTrue(expanded)
          assertEquals("Expanded", fixture.stateDescription("Details"))
          assertTrue(fixture.requestFocus("Details"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertFalse(expanded)
          assertEquals("Collapsed", fixture.stateDescription("Details"))
          assertTrue(fixture.isFocused("Details"))
        }
  }

  @Test
  fun sharedChromeFixtureShowsInteractionStatesAndKeepsTextLegibleAtOneHundredFiftyPercent() {
    ComposeVisualFixture(720, 320, 1.5f) { SharedChromeStatesVisualFixture() }
        .use { fixture ->
          fixture.render("shared-chrome-states-150")
          listOf(
                  "Default",
                  "Hovered",
                  "Pressed",
                  "Selected tab",
                  "Disabled",
                  "Focused",
                  "Collapsed section",
                  "Expanded section")
              .forEach(fixture::assertTextFits)
          assertEquals("Collapsed", fixture.stateDescription("Collapsed section"))
          assertEquals("Expanded", fixture.stateDescription("Expanded section"))
          assertTrue(fixture.isDisabled("Disabled"))
        }
  }

  @Test
  fun baselineCapturesSummaryAndExercisesOnlyTheLiveProjectMenu() {
    ComposeVisualFixture(1440, 900) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject)
        }
        .use { fixture ->
          fixture.render("summary-1440")
          assertTrue(fixture.hasText("Indexed files"))
          assertTrue(fixture.hasText("Analysis summary"))
          assertFalse(fixture.hasText("Project understanding"))
        }

    ComposeVisualFixture(1440, 900) { ToolbarVisualFixture(1440f) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("go-shop · fixture")
          fixture.render("project-menu-open")
          assertTrue(fixture.hasText("Open project"))
          assertTrue(fixture.hasText("Re-index project"))
        }
  }

  @Test
  fun toolbarProjectIconsFollowTheDetectedTypeAndKeepTheProjectMenuWorking() {
    listOf(1440 to 1f, 1000 to 1.25f, 800 to 1.5f).forEach { (width, scale) ->
      var project by mutableStateOf<ProjectAnalysis?>(visualFixtureProject)
      ComposeVisualFixture(width, 220, scale) {
            ToolbarVisualFixture(width.toFloat(), project = project)
          }
          .use { fixture ->
            listOf(
                    "go" to "Go project",
                    "JAVA" to "Java project",
                    " kotlin " to "Kotlin project",
                    "rust" to "Project type: rust",
                    "unknown" to "Project type: unknown")
                .forEach { (type, description) ->
                  project = visualFixtureProject.copy(type = type)
                  fixture.render("project-type-${type.trim()}-$width-$scale")
                  assertTrue(fixture.hasDescription(description))
                  listOf("Go project", "Java project", "Kotlin project")
                      .filter { it != description }
                      .forEach { assertFalse(fixture.hasDescription(it)) }
                }
            project = visualFixtureProject.copy(type = "go")
            fixture.render()
            fixture.clickDescription("Go project")
            fixture.render()
            assertTrue(fixture.hasText("Open project"))
            assertTrue(fixture.hasText("Re-index project"))
            fixture.dismissPopup()
            project = null
            fixture.render()
            assertTrue(fixture.hasDescription("Project"))
            assertFalse(fixture.hasDescription("Go project"))
          }
    }
  }

  @Test
  fun popupMenusUseProductionRowsForLiveProjectFlows() {
    var imports = 0
    var reindexes = 0
    var reconnects = 0
    val actions =
        ToolbarActions(
            onImport = { imports++ },
            onReanalyze = { reindexes++ },
            onReconnect = { reconnects++ },
            onPalette = {},
            onOpenExplorer = {},
            onOpenContext = {},
        )
    ComposeVisualFixture(800, 220, 1.3f) {
          ToolbarVisualFixture(
              width = 800f,
              project = null,
              connection = ConnectionState(label = "Disconnected"),
              actions = actions)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("No project open")
          fixture.render()
          assertTrue(fixture.hasText("Open project"))
          assertTrue(fixture.isDisabled("Re-index project"))
          assertTrue(fixture.hasText("Reconnect"))
          fixture.clickText("Open project")
          kotlin.test.assertEquals(1, imports)
          kotlin.test.assertEquals(0, reindexes)
          kotlin.test.assertEquals(0, reconnects)
        }

    ComposeVisualFixture(1440, 220) {
          ToolbarVisualFixture(
              width = 1440f,
              project = visualFixtureProject,
              connection = ConnectionState(label = "Disconnected"),
              actions = actions)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("go-shop · fixture")
          fixture.render()
          fixture.clickText("Re-index project")
          fixture.render()
          fixture.clickText("go-shop · fixture")
          fixture.render()
          fixture.clickText("Reconnect")
          kotlin.test.assertEquals(1, imports)
          kotlin.test.assertEquals(1, reindexes)
          kotlin.test.assertEquals(1, reconnects)
        }
  }

  @Test
  fun popupSurfaceWrapsLongRowsWithoutClipping() {
    val longLabel =
        "A long available menu action remains readable instead of being shortened at narrow widths"
    ComposeVisualFixture(320, 200, 1.3f) { PopupMenuVisualFixture(longLabel) }
        .use { fixture ->
          fixture.render("popup-surface-320-1.3")
          fixture.assertTextWrapsWithoutClipping(longLabel)
          fixture.assertTextFits("Unavailable action")
          assertTrue(fixture.isDisabled("Unavailable action"))
        }
  }

  @Test
  fun analysisStatusStaysImmediatelyBeforeDaemonAcrossToolbarSizesAndRunChanges() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(800, 650, 1.5f),
            Triple(1280, 600, 1.25f),
            Triple(1280, 600, 1.5f))
        .forEach { (width, height, scale) ->
          var state by
              mutableStateOf(
                  DesktopState(
                      projectState = ProjectWorkspaceState(resultProjectFixture()),
                      analysisRun =
                          ProjectAnalysisRunState(
                              run = analysisRunFixture().copy(status = "running"))))
          ComposeVisualFixture(width, height, scale) {
                ToolbarVisualFixture(
                    width.toFloat(),
                    analysisStatus = toolbarAnalysisStatus(state),
                    showEditorDrawerActions = useNarrowLayout(width.toFloat()))
              }
              .use { fixture ->
                val daemon = if (width < 1000) "Connected" else "Daemon connected"
                fixture.render("toolbar-analysis-$width-$scale")
                fixture.assertTextFits("Analysis · Running")
                fixture.assertTextFits(daemon)
                fixture.assertTextBefore("Analysis · Running", daemon)
                fixture.assertTextFits(
                    if (width < 1220) "Search" else "Search files, symbols, commands")
                listOf("paused", "completed").forEach { status ->
                  state =
                      state.copy(
                          analysisRun =
                              state.analysisRun.copy(
                                  run = state.analysisRun.run!!.copy(status = status)))
                  fixture.render()
                  val label = "Analysis · ${status.replaceFirstChar { it.uppercase() }}"
                  fixture.assertTextFits(label)
                  fixture.assertTextBefore(label, daemon)
                  assertFalse(fixture.hasText("Analysis · Running"))
                }
                state = state.copy(analysisRun = ProjectAnalysisRunState())
                fixture.render()
                assertFalse(fixture.hasText("Analysis · Completed"))
                fixture.assertTextFits(daemon)
              }
        }
  }

  @Test
  fun toolbarKeepsOnlyLiveProjectSearchBranchAndConnectionChrome() {
    ComposeVisualFixture(1440, 220) { ToolbarVisualFixture(1440f) }
        .use { fixture ->
          fixture.render("toolbar-live-destinations-1440")
          assertTrue(fixture.hasText("Search files, symbols, commands"))
          assertTrue(fixture.hasText("main"))
          assertTrue(fixture.hasText("Daemon connected"))
          listOf(
                  "Preview",
                  "New file",
                  "Branch actions",
                  "Content search",
                  "Settings & Help",
                  "Account",
                  "Terminal",
                  "Run / Debug")
              .forEach { label -> assertFalse(fixture.hasText(label)) }
        }
  }

  @Test
  fun terminalChromeKeepsOneControlAndVisibleStateAcrossDockAndOverlaySizes() {
    listOf(
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1280, 600, 1.5f),
            Triple(360, 240, 1.5f))
        .forEach { (width, height, scale) ->
          var layout by mutableStateOf(DesktopLayoutState())
          var opens = 0
          var session by mutableStateOf(TerminalSessionState())
          ComposeVisualFixture(width, height, scale) {
                TerminalDock(
                    layout,
                    TerminalWorkspaceState(
                        tabs = listOf(TerminalTabState(1, "Shell 1", session)), activeTabId = 1),
                    {
                      opens++
                      layout = layout.openTerminal()
                    },
                    { layout = layout.withBottomCollapsed(true) },
                    TerminalTabActions({}, {}, {}),
                    {},
                    {},
                    { modifier -> Text("Synthetic shell", modifier = modifier) })
              }
              .use { fixture ->
                fixture.render("terminal-collapsed-$width-$scale")
                fixture.assertTextFits("Terminal")
                assertEquals("Collapsed", fixture.stateDescription("Terminal"))
                assertEquals(1, fixture.textCount("Terminal"))
                listOf("Bugs & Problems", "Checks", "Output", "Synthetic shell").forEach {
                  assertFalse(fixture.hasText(it))
                }
                assertEquals(0, opens)
                assertTrue(fixture.requestFocus("Terminal"))
                assertTrue(fixture.pressKey(Key.Enter))
                fixture.render("terminal-expanded-$width-$scale")
                assertEquals("Expanded", fixture.stateDescription("Terminal"))
                assertTrue(fixture.hasText("Synthetic shell"))
                assertEquals(1, opens)
                fixture.clickText("Terminal")
                session =
                    TerminalSessionState(TerminalSessionPhase.Failed, error = "Shell launch failed")
                fixture.render("terminal-failed-collapsed-$width-$scale")
                fixture.clickText("Terminal")
                fixture.render("terminal-failed-expanded-$width-$scale")
                assertTrue(fixture.hasText("Shell 1 · Terminal needs attention"))
                fixture.clickText("Terminal")
                fixture.render()
                assertFalse(fixture.hasText("Synthetic shell"))
                assertEquals(2, opens)
              }
        }
  }

  @Test
  fun transientOpenersCanReceiveKeyboardFocus() {
    val paletteFocus = FocusRequester()
    ComposeVisualFixture(1000, 220) {
          ToolbarVisualFixture(1000f, paletteFocusRequester = paletteFocus)
        }
        .use { fixture ->
          fixture.render()
          paletteFocus.requestFocus()
          fixture.render()
          assertTrue(fixture.isFocused("Search"))
        }

    val bottomToolsFocus = FocusRequester()
    ComposeVisualFixture(800, 120) {
          TerminalBar(
              TerminalWorkspaceState(),
              collapsed = true,
              tabActions = TerminalTabActions({}, {}, {}),
              onToggle = {},
              controlModifier = Modifier.focusRequester(bottomToolsFocus),
          )
        }
        .use { fixture ->
          fixture.render()
          bottomToolsFocus.requestFocus()
          fixture.render()
          assertTrue(fixture.isFocused("Terminal"))
        }
  }

  @Test
  fun summaryDashboardShowsGroupedInterpretationWithDiagramDisclosures() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate before storage.",
                            whyItMattersHere = "Keep invalid input out of the repository.",
                            tradeoffOrFailureMode = "Validation rules must stay consistent.",
                            transferableLesson = "Validate at the request boundary.")))
    ComposeVisualFixture(1440, 1600) { ProjectSummaryPane(overview, visualFixtureProject) }
        .use { fixture ->
          fixture.awaitDescription("Show Architecture diagram", "Collapsed")
          fixture.awaitDescription("Show Flow 1 diagram", "Collapsed")
          fixture.render("summary-dashboard-collapsed-1440")
          fixture.clickDescription("Show Architecture diagram")
          fixture.clickDescription("Show Flow 1 diagram")
          fixture.awaitDescription(
              "Architecture diagram\n" + visualFixtureOverview.analysis.architecture)
          fixture.awaitDescription(
              "Flow 1 diagram\n" + visualFixtureOverview.analysis.flows.first())
          fixture.render("summary-dashboard-1440")
          listOf(
                  "Analysis summary",
                  "Purpose",
                  "Architecture",
                  "Packages / modules",
                  "Flows",
                  "Engineering insight",
                  "Validate before storage.")
              .forEach { assertTrue(fixture.hasText(it), it) }
          assertTrue(fixture.hasText("23"))
          assertTrue(fixture.hasText("Tool-reported issues"))
          fixture.assertTextAbove("Purpose", "Analysis summary")
          fixture.assertSummaryStatusPlacement("Outdated")
          listOf("Bugs", "Performance Issues", "Security Issues").forEach {
            assertTrue(fixture.hasDescription(it))
            assertFalse(fixture.hasText(it))
          }
          fixture.assertTextAbove("Packages / modules", "Flows")
          assertFalse(fixture.hasText("Entry points"))
          assertFalse(fixture.hasText("Next steps"))
          assertFalse(fixture.hasText("cmd/server/main.go"))
          assertFalse(fixture.hasText("Review boundary validation."))
          assertTrue(
              fixture.hasDescription(
                  projectSummaryPresentation(overview, visualFixtureProject).analysisMessage))
          fixture.assertTextBefore("Mechanism", "Why it matters here")
          fixture.assertTextBefore("Trade-off or failure mode", "Transferable lesson")
          assertFalse(fixture.hasText("Type: Go · Build: go.mod · Languages: Go · Markdown"))
          assertFalse(fixture.hasText(visualFixtureProject.name))
          assertFalse(fixture.hasText("AI interpretation"))
          assertFalse(fixture.hasText("Project purpose"))
          assertFalse(fixture.hasText("Interpretation details"))
          assertFalse(fixture.hasText("Analysis"))
          assertFalse(fixture.hasText("Project understanding"))
          assertFalse(fixture.hasText("Risks · AI suggestions"))
          assertFalse(fixture.hasText("MEDIUM · Input validation is incomplete."))
          assertFalse(fixture.hasText("Show full response"))
          assertFalse(fixture.tryClick("Architecture"))
          assertFalse(fixture.tryClick("Engineering insight"))
          assertFalse(fixture.hasEditableText())
        }
  }

  @Test
  fun summaryDashboardShowsCompleteLongProseInThePage() {
    val longPurpose = "This purpose remains readable directly in the dashboard. ".repeat(60)
    val overview =
        visualFixtureOverview.copy(
            analysis = visualFixtureOverview.analysis.copy(purpose = longPurpose))
    ComposeVisualFixture(800, 650, 1.5f) { ProjectSummaryPane(overview, visualFixtureProject) }
        .use { fixture ->
          fixture.render()
          fixture.scrollBy(500f)
          fixture.render("summary-dashboard-long-purpose-800-150")
          fixture.assertTextWrapsWithoutClipping(longPurpose)
          assertFalse(fixture.hasText("Show full response"))
          assertEquals(1, fixture.scrollableContentCount())
          repeat(12) {
            fixture.scrollBy(500f)
            fixture.render()
          }
          assertTrue(fixture.hasText("Flows"))
          assertFalse(fixture.hasText("Risks · AI suggestions"))
          assertFalse(fixture.hasText("MEDIUM · Input validation is incomplete."))
          assertFalse(fixture.hasText("Next steps"))
          assertFalse(fixture.hasText("Interpretation details"))
        }
  }

  @Test
  fun summaryDashboardCoverageHidesZerosAndExposesAnalysisStateInLightDescription() {
    listOf("fresh", "stale", "failed", "missing", "running").forEach { status ->
      val overview =
          visualFixtureOverview.copy(
              analysis =
                  visualFixtureOverview.analysis.copy(
                      status = status, failure = "Provider timed out."))
      ComposeVisualFixture(1280, 600) { ProjectSummaryPane(overview, visualFixtureProject) }
          .use { fixture ->
            fixture.render("summary-dashboard-$status-1280-600")
            listOf("Ready" to Success, "Stale" to Warning, "Missing" to SecondaryText).forEach {
                (label, tint) ->
              assertEquals(1, fixture.textCount(label))
              fixture.assertColorVisible(tint)
              fixture.assertTextContrast(label, blendOver(tint.copy(alpha = 0.08f), Panel))
            }
            assertFalse(fixture.hasText("Failed"))
            assertFalse(fixture.hasText("Running"))
            assertTrue(fixture.hasText("Outdated"))
            assertTrue(
                fixture.hasDescription(projectSummaryPresentation(overview, null).analysisMessage))
            assertFalse(fixture.hasText(projectSummaryPresentation(overview, null).analysisMessage))
            fixture.assertColorVisible(Warning)
            if (status == "failed") assertFalse(fixture.hasText(overview.analysis.purpose))
          }
    }
  }

  @Test
  fun summaryDashboardOmitsEmptyCoverageButRetainsUnavailableCounts() {
    ComposeVisualFixture(1440, 1100) {
          ProjectSummaryPane(
              visualFixtureOverview.copy(analysisCoverage = AnalysisCoverage()),
              visualFixtureProject)
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("Ready"))
          assertTrue(fixture.hasText("Tool-reported issues"))
          assertTrue(fixture.hasText("Purpose"))
          fixture.assertUniformSummaryCards(7)
          fixture.assertSummaryStatusPlacement("Updated")
        }
    ComposeVisualFixture(1440, 1100) { ProjectSummaryPane(null, visualFixtureProject) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Analysis summary"))
          assertTrue(fixture.hasText("Missing"))
          assertTrue(fixture.hasText("—"))
        }
  }

  @Test
  fun summaryPanelUsesStatusColorsAndPlainStatusLabels() {
    listOf(
            Triple("fresh", "Updated", Success),
            Triple("stale", "Outdated", Warning),
            Triple("failed", "Failed", Error))
        .forEach { (status, label, tint) ->
          val overview =
              visualFixtureOverview.copy(
                  analysis = visualFixtureOverview.analysis.copy(status = status),
                  analysisCoverage =
                      when (status) {
                        "fresh" -> AnalysisCoverage(total = 23, fresh = 23)
                        "stale" -> AnalysisCoverage(total = 23, stale = 23)
                        else -> AnalysisCoverage(total = 23, failed = 23)
                      })
          ComposeVisualFixture(1280, 600) { ProjectSummaryPane(overview, visualFixtureProject) }
              .use { fixture ->
                fixture.render("summary-panel-$status")
                fixture.assertSummaryStatusPlacement(label)
                fixture.assertTextContrast(label, blendOver(tint.copy(alpha = 0.16f), Panel))
                fixture.assertColorVisible(blendOver(tint.copy(alpha = 0.16f), Panel))
                fixture.assertColorVisible(blendOver(tint.copy(alpha = 0.08f), Panel))
              }
        }
  }

  @Test
  fun summaryStatusLightExposesFailureOnKeyboardFocus() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                StructuredProjectAnalysis(status = "failed", failure = "Provider timed out."),
            analysisCoverage = AnalysisCoverage())
    val description = "Project description: failed · Provider timed out."
    ComposeVisualFixture(800, 650) { ProjectSummaryPane(overview, visualFixtureProject) }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText(description))
          fixture.pressKey(Key.Tab)
          fixture.render("summary-status-keyboard-focus")
          assertTrue(fixture.isDescriptionFocused(description))
          assertTrue(fixture.hasText(description))
          fixture.assertColorVisible(FocusAccent)
        }
  }

  @Test
  fun summaryOutdatedBadgeClearsAfterRefreshAndIconsExplainThemselvesOnFocus() {
    var overview by mutableStateOf(visualFixtureOverview)
    ComposeVisualFixture(1280, 600) { ProjectSummaryPane(overview, visualFixtureProject) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Outdated"))
          overview = overview.copy(analysisCoverage = AnalysisCoverage(total = 23, fresh = 23))
          fixture.render()
          assertFalse(fixture.hasText("Outdated"))
          repeat(8) {
            if (!fixture.hasText("Bugs")) {
              fixture.pressKey(Key.Tab)
              fixture.render()
            }
          }
          fixture.render("summary-bug-icon-keyboard-label")
          assertTrue(fixture.hasText("Bugs"))
          assertTrue(fixture.hasDescription("Bugs"))
          assertFalse(fixture.hasText("Performance Issues"))
        }
  }

  @Test
  fun summaryShowsOutdatedBadgeForStaleRunEvenWhenProjectPurposeIsFresh() {
    val project = resultProjectFixture()
    val overview = visualFixtureOverview.copy(analysisCoverage = AnalysisCoverage())
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectSummaryPane(overview, project, analysisRunFixture().copy(status = "stale"))
        }
        .use { fixture ->
          fixture.render("summary-outdated-run-800-150")
          fixture.assertTextFits("Outdated")
          fixture.assertTextContrast("Outdated", blendOver(Warning.copy(alpha = 0.16f), Panel))
        }
  }

  @Test
  fun summaryMetricFlowKeepsEveryCoverageBoxVisibleAndRemovesPartialText() {
    val (base, section) = summaryBugFixture(listOf("high", "medium", "low"))
    val run =
        base.copy(
            sections =
                base.sections.map {
                  if (it.category == "performance") it.copy(status = "partial") else it
                })
    val overview =
        visualFixtureOverview.copy(
            analysisCoverage =
                AnalysisCoverage(
                    total = 5, fresh = 1, stale = 1, missing = 1, running = 1, failed = 1))
    listOf(1440 to 1f, 1000 to 1.5f, 999 to 1.5f, 800 to 1.5f).forEach { (width, scale) ->
      ComposeVisualFixture(width, 650, scale) {
            ProjectSummaryPane(overview, resultProjectFixture(), run, section)
          }
          .use { fixture ->
            fixture.render("summary-all-metrics-$width-$scale")
            listOf(
                    "Ready",
                    "Stale",
                    "Missing",
                    "Running",
                    "Failed",
                    "High: 1",
                    "Medium: 1",
                    "Low: 1")
                .forEach(fixture::assertTextFits)
            listOf("Bugs", "Performance Issues", "Security Issues").forEach {
              assertTrue(fixture.hasDescription(it))
              assertFalse(fixture.hasText(it))
            }
            assertFalse(fixture.hasText("Partial"))
            fixture.assertTextFits("Updating")
            fixture.assertSummaryStatusPlacement("Updating")
            fixture.assertUniformSummaryCards(12)
          }
    }
  }

  @Test
  fun summaryModulesShowNamesPathsAndDescriptionsInFlatRows() {
    ComposeVisualFixture(800, 650, 1.5f) {
          SummaryModules(
              listOf(
                  "internal/project (Project indexing): Builds the project context.",
                  "internal/app (Workflow orchestration): Coordinates guarded changes."))
        }
        .use { fixture ->
          fixture.render("summary-modules-800-150")
          listOf(
                  "Project indexing",
                  "internal/project",
                  "Builds the project context.",
                  "Workflow orchestration",
                  "internal/app")
              .forEach(fixture::assertTextFits)
          fixture.assertTextAbove("Project indexing", "internal/project")
          assertFalse(fixture.hasEditableText())
        }
  }

  @Test
  fun summaryMermaidDiagramsRenderAndExposeTheirSource() {
    val source =
        "flowchart TD\n A[Client] --> B{Valid?}\n B -->|yes| C[Store]\n B -->|no| D[Reject]"
    listOf(1440 to 1f, 800 to 1.5f).forEach { (width, scale) ->
      ComposeVisualFixture(width, 800, scale) { MermaidDiagram(source, "Architecture") }
          .use { fixture ->
            fixture.awaitDescription("Show Architecture diagram", "Collapsed")
            assertFalse(fixture.isDisabled("Show diagram"))
            assertFalse(fixture.hasDescription("Architecture diagram\n$source"))
            assertFalse(fixture.hasText("Mermaid source"))
            assertTrue(fixture.requestFocus("Show diagram"))
            fixture.pressKey(Key.Enter)
            fixture.awaitDescription("Architecture diagram\n$source")
            assertEquals("Expanded", fixture.stateDescription("Hide diagram"))
            fixture.render("summary-mermaid-$width-$scale")
            assertFalse(fixture.hasText("Rendering diagram…"))
            fixture.clickText("Mermaid source")
            fixture.render()
            assertTrue(fixture.hasText(source))
            assertFalse(fixture.hasEditableText())
            fixture.clickText("Hide diagram")
            fixture.render()
            assertFalse(fixture.hasDescription("Architecture diagram\n$source"))
            assertFalse(fixture.hasText(source))
            assertEquals("Collapsed", fixture.stateDescription("Show diagram"))
          }
    }
  }

  @Test
  fun summaryDiagramsDisableDisclosureWhenUnavailableAndResetForNewResults() {
    listOf("Architecture", "Flow 1").forEach { label ->
      var value by mutableStateOf("This result describes the project in prose.")
      ComposeVisualFixture(800, 650, 1.5f) { MermaidDiagram(value, label) }
          .use { fixture ->
            fixture.render("summary-diagram-unavailable-${label.replace(' ', '-')}")
            assertTrue(fixture.hasText(value))
            assertTrue(fixture.isDisabled("Show diagram"))
            assertEquals("Diagram unavailable", fixture.stateDescription("Show diagram"))
            assertFalse(fixture.hasText("Run Analysis again to generate this diagram."))
            assertFalse(fixture.tryClick("Show diagram"))

            value = "flowchart TD\n A[Client] --> B[Server]"
            fixture.awaitDescription("Show $label diagram", "Collapsed")
            fixture.clickText("Show diagram")
            fixture.awaitDescription("$label diagram\n$value")

            value = "flowchart TD\n A[Replacement]"
            fixture.awaitDescription("Show $label diagram", "Collapsed")
            assertFalse(fixture.hasDescription("$label diagram\n$value"))
            assertFalse(fixture.isDisabled("Show diagram"))

            value = "flowchart TD\n A[Node]\n click A \"https://example.com\""
            fixture.awaitDescription("Show $label diagram", "Diagram unavailable")
            assertTrue(fixture.isDisabled("Show diagram"))
            assertTrue(fixture.hasText(value))
            assertFalse(fixture.tryClick("Show diagram"))
          }
    }
  }

  @Test
  fun summaryIssuesShowPriorityBreakdownAndWeightedTrafficLights() {
    val (baseRun, section) = summaryBugFixture(listOf("high", "high", "high", "low"))
    val run =
        baseRun.copy(
            sections =
                baseRun.sections.map {
                  when (it.category) {
                    "performance" -> it.copy(findingCount = 5)
                    "security" -> it.copy(findingCount = 0)
                    else -> it
                  }
                })
    listOf(1440 to 900, 1000 to 650, 999 to 650, 800 to 650, 1280 to 600).forEach { (width, height)
      ->
      listOf(1f, 1.5f).forEach { scale ->
        ComposeVisualFixture(width, height, scale) {
              ProjectSummaryPane(visualFixtureOverview, resultProjectFixture(), run, section)
            }
            .use { fixture ->
              fixture.render()
              fixture.render("summary-issue-colors-$width-$height-$scale")
              listOf("High: 3", "Medium: 0", "Low: 1").forEach(fixture::assertTextFits)
              fixture.assertBugPrioritiesInsideCard("High: 3", "Medium: 0", "Low: 1")
              assertFalse(fixture.hasText("Score: 10 points"))
              assertFalse(fixture.hasText("Score unavailable"))
              assertFalse(fixture.hasText("Bug priorities"))
              listOf("Bugs" to Error, "Performance Issues" to Warning, "Security Issues" to Success)
                  .forEach { (label, tint) ->
                    fixture.assertColorVisible(tint)
                    assertTrue(fixture.hasDescription(label))
                    assertFalse(fixture.hasText(label))
                  }
            }
      }
    }
  }

  @Test
  fun summaryDashboardAdaptsToNarrowShortAndLargeTextViews() {
    listOf(1440 to 900, 1000 to 650, 999 to 650, 800 to 650, 1280 to 600).forEach { (width, height)
      ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        ComposeVisualFixture(width, height, scale) {
              ProjectSummaryPane(visualFixtureOverview, visualFixtureProject)
            }
            .use { fixture ->
              fixture.render("summary-dashboard-$width-$height-$scale")
              listOf(
                      "Analysis summary",
                      "Tool-reported issues",
                      "AI suggestions",
                      "Ready",
                      "Stale",
                      "Missing")
                  .forEach { label ->
                    fixture.assertTextFits(
                        label, maxLines = if (label == "Tool-reported issues") 2 else 1)
                  }
              fixture.assertUniformSummaryCards(10)
              fixture.assertSummaryStatusPlacement("Outdated")
              assertTrue(fixture.hasScrollableContent())
            }
      }
    }
    ComposeVisualFixture(800, 300) { ProjectSummaryPane(null, null) }
        .use { fixture ->
          fixture.render("summary-dashboard-empty-800")
          assertTrue(fixture.hasText("No project selected"))
          assertFalse(fixture.hasText("Analysis summary"))
        }
  }

  @Test
  fun insightDisclosureUsesKeyboardToggleAndPreservesTheFullStaleInterpretation() {
    val original = EngineeringInsightPreference.load()
    val insight =
        EngineeringInsight(
            mechanism = "The handler validates its identifier before the repository call.",
            whyItMattersHere = "The returned error remains distinguishable for the caller.",
            tradeoffOrFailureMode = "Malformed input otherwise reaches the persistence layer.",
            transferableLesson = "Keep boundary validation close to request handling.")
    val originalInsight = insight.copy()

    try {
      EngineeringInsightPreference.save(false)
      ComposeVisualFixture(720, 420, 1.5f) {
            EngineeringInsightPanel(insight, stale = true, scopeLabel = "Visual fixture")
          }
          .use { fixture ->
            fixture.render("engineering-insight-collapsed-720-1.5")
            assertTrue(fixture.hasText("Engineering insight"))
            assertTrue(fixture.hasText("AI interpretation · Visual fixture · stale"))
            assertTrue(fixture.stateDescription("Engineering insight") == "Collapsed")
            assertFalse(fixture.hasText("Mechanism"))
            assertTrue(fixture.requestFocus("Engineering insight"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render("engineering-insight-expanded-720-1.5")
            assertTrue(fixture.stateDescription("Engineering insight") == "Expanded")
            assertTrue(fixture.hasText("Mechanism"))
            assertTrue(fixture.hasText("Why it matters here"))
            assertTrue(fixture.hasText("Trade-off or failure mode"))
            assertTrue(fixture.hasText("Transferable lesson"))
            assertTrue(fixture.hasText(insight.mechanism))
            assertTrue(fixture.hasText(insight.whyItMattersHere))
            assertFalse(fixture.hasText("Close insight"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            fixture.render()
            assertFalse(fixture.hasText("Mechanism"))
            assertTrue(fixture.isFocused("Engineering insight"))
            assertEquals(originalInsight, insight)
          }
      ComposeVisualFixture(360, 220, 1.5f) {
            EngineeringInsightPanel(insight, stale = true, scopeLabel = "File")
          }
          .use { fixture ->
            fixture.render("engineering-insight-narrow-360-1.5")
            fixture.assertTextFits("Engineering insight")
            fixture.assertTextFits("AI interpretation · File · stale")
          }
      ComposeVisualFixture(360, 100) { EngineeringInsightPanel(null) }
          .use { fixture -> assertFalse(fixture.hasText("Engineering insight")) }
    } finally {
      EngineeringInsightPreference.save(original)
    }
  }

  @Test
  fun resultToolsAndToolWindowHeadersKeepInteractionLocalAtNarrowScale() {
    var workflowActions = 0
    ComposeVisualFixture(480, 650, 1.3f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(visualFixtureFindings, null, false),
              BugsWorkspaceActions(
                  FindingActions(
                      openFinding = { workflowActions++ },
                      prepareFinding = { workflowActions++ },
                      triageFinding = { _, _ -> workflowActions++ }),
                  {},
                  {}))
        }
        .use { fixture ->
          fixture.render("findings-tools-collapsed-480-1.3")
          assertTrue(fixture.stateDescription("Verified checks") == "Collapsed")
          assertTrue(fixture.requestFocus("Verified checks"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render("findings-tools-expanded-480-1.3")
          assertTrue(fixture.stateDescription("Verified checks") == "Expanded")
          assertTrue(fixture.hasText("Trust local execution & run scan"))
          assertFalse(fixture.hasEditableText())
          assertTrue(fixture.hasScrollableContent())
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.stateDescription("Verified checks") == "Collapsed")
          kotlin.test.assertEquals(0, workflowActions)
        }

    var closes = 0
    var opens = 0
    ComposeVisualFixture(360, 220, 1.3f) {
          Column(Modifier.fillMaxSize().background(AppBackground)) {
            DockedToolWindow(
                title = "Files",
                content = { Text("Indexed relative paths", modifier = it.padding(8.dp)) },
                modifier = Modifier.fillMaxWidth().weight(1f),
                onClose = { closes++ })
            TerminalDock(
                layout = DesktopLayoutState(bottomCollapsed = true),
                state = TerminalWorkspaceState(),
                tabActions = TerminalTabActions({}, {}, {}),
                onOpen = { opens++ },
                onCollapse = {},
                onHeightDelta = {},
                onHeightCommit = {},
                content = {})
          }
        }
        .use { fixture ->
          fixture.render("tool-window-controls-360-1.3")
          fixture.clickDescription("Close Files drawer")
          fixture.clickText("Terminal")
          kotlin.test.assertEquals(1, closes)
          kotlin.test.assertEquals(1, opens)
        }

    var overlayDismissals = 0
    ComposeVisualFixture(480, 420, 1.3f) {
          TerminalOverlay(
              state = TerminalWorkspaceState(),
              tabActions = TerminalTabActions({}, {}, {}),
              onDismiss = { overlayDismissals++ },
              content = { modifier -> Text("Synthetic shell", modifier = modifier) })
        }
        .use { fixture ->
          fixture.render("terminal-overlay-480-1.3")
          assertTrue(fixture.hasText("Terminal"))
          assertTrue(fixture.hasText("Synthetic shell"))
          fixture.clickText("Hide terminal")
          kotlin.test.assertEquals(1, overlayDismissals)
        }
  }

  @Test
  fun problemRowsRevealDetailsBeforeAnyWorkflowAction() {
    var sourceRequests = 0
    var mutations = 0
    ComposeVisualFixture(900, 500) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(visualFixtureFindings, null, false),
              BugsWorkspaceActions(
                  FindingActions({ sourceRequests++ }, { mutations++ }, { _, _ -> mutations++ }),
                  {},
                  {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Validate the user identifier")
          fixture.render()
          kotlin.test.assertEquals(0, sourceRequests)
          kotlin.test.assertEquals(0, mutations)
          fixture.clickText("Open source")
          kotlin.test.assertEquals(1, sourceRequests)
          kotlin.test.assertEquals(0, mutations)
        }
  }
}

/** This test-only adapter is tied to the Compose version pinned in build.gradle.kts. */
@OptIn(ExperimentalComposeUiApi::class, InternalComposeUiApi::class)
internal class ComposeVisualFixture(
    private val width: Int,
    private val height: Int,
    fontScale: Float = 1f,
    densityScale: Float = 1f,
    content: @Composable () -> Unit,
) : AutoCloseable {
  private val owners = mutableListOf<SemanticsOwner>()
  private val platform =
      object : PlatformContext by PlatformContext.Empty() {
        override val semanticsOwnerListener =
            object : PlatformContext.SemanticsOwnerListener {
              override fun onSemanticsOwnerAppended(semanticsOwner: SemanticsOwner) {
                owners += semanticsOwner
              }

              override fun onSemanticsOwnerRemoved(semanticsOwner: SemanticsOwner) {
                owners -= semanticsOwner
              }

              override fun onSemanticsChange(semanticsOwner: SemanticsOwner) = Unit

              override fun onLayoutChange(semanticsOwner: SemanticsOwner, semanticsNodeId: Int) =
                  Unit
            }
      }
  private val scene =
      CanvasLayersComposeScene(
          density = Density(densityScale, fontScale),
          size = IntSize(width, height),
          coroutineContext = Dispatchers.Unconfined,
          platformContext = platform)
  private val surface = Surface.makeRasterN32Premul(width, height)
  private var frameTime = 0L

  init {
    scene.setContent { MiniOrcaTheme { content() } }
  }

  fun render(name: String? = null) {
    repeat(3) {
      surface.canvas.clear(AppBackground.toArgb())
      scene.render(surface.canvas.asComposeCanvas(), frameTime)
      frameTime += 80_000_000
    }
    if (name != null)
        System.getProperty("miniOrca.visualOutput")?.let { output ->
          val directory = File(output).apply { mkdirs() }
          surface.makeImageSnapshot().use { rendered ->
            requireNotNull(rendered.encodeToData()).use { data ->
              File(directory, "$name.png").writeBytes(data.bytes)
            }
          }
        }
  }

  fun awaitDescription(label: String, state: String? = null) {
    fun matches(): Boolean =
        nodes().any { node ->
          node.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
              (state == null ||
                  generateSequence(node) { it.parent }
                      .any { it.config.getOrNull(SemanticsProperties.StateDescription) == state })
        }
    val deadline = System.nanoTime() + 20_000_000_000L
    render()
    while (!matches() && System.nanoTime() < deadline) {
      Thread.sleep(20)
      render()
    }
    assertTrue(matches(), "Timed out waiting for $label${state?.let { " ($it)" }.orEmpty()}")
  }

  fun hasText(label: String): Boolean =
      textNodes(label).isNotEmpty() ||
          nodes().any { it.config.getOrNull(SemanticsProperties.EditableText)?.text == label }

  fun textCount(label: String): Int = textNodes(label).size

  fun hasEditableText(): Boolean =
      nodes().any { it.config.getOrNull(SemanticsActions.SetText) != null }

  fun hasDescription(label: String): Boolean =
      nodes().any {
        it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
      }

  fun setFocusedText(value: String) {
    val editor =
        nodes().single {
          it.config.getOrNull(SemanticsProperties.Focused) == true &&
              it.config.getOrNull(SemanticsActions.SetText) != null
        }
    assertTrue(
        requireNotNull(editor.config.getOrNull(SemanticsActions.SetText)?.action)
            .invoke(AnnotatedString(value)))
  }

  fun setText(value: String) {
    val editor = nodes().single { it.config.getOrNull(SemanticsActions.SetText) != null }
    assertTrue(
        requireNotNull(editor.config.getOrNull(SemanticsActions.SetText)?.action)
            .invoke(AnnotatedString(value)))
  }

  fun clickText(label: String) {
    clickNode(textNodes(label).firstOrNull(), label)
  }

  fun clickDescription(label: String) {
    clickNode(
        nodes().firstOrNull {
          it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
        },
        label)
  }

  fun tryClick(label: String): Boolean =
      nodes()
          .filter {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true ||
                it.config.getOrNull(SemanticsProperties.Text)?.any { text -> text.text == label } ==
                    true
          }
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { node -> node.config.getOrNull(SemanticsActions.OnClick)?.action }
          .firstOrNull()
          ?.invoke() ?: false

  private fun clickNode(start: SemanticsNode?, label: String) {
    var node = start
    while (node != null) {
      val click = node.config.getOrNull(SemanticsActions.OnClick)?.action
      if (click != null) {
        assertTrue(click())
        return
      }
      node = node.parent
    }
    error("No clickable control for $label")
  }

  fun isDisabled(label: String): Boolean =
      textNodes(label).any { node ->
        generateSequence(node) { it.parent }
            .any { it.config.getOrNull(SemanticsProperties.Disabled) != null }
      }

  fun isFocused(label: String): Boolean =
      textNodes(label).any { node ->
        generateSequence(node) { it.parent }
            .any { it.config.getOrNull(SemanticsProperties.Focused) == true }
      }

  fun isDescriptionFocused(label: String): Boolean =
      nodes().any { node ->
        node.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
            node.config.getOrNull(SemanticsProperties.Focused) == true
      }

  fun stateDescription(label: String): String? =
      textNodes(label)
          .asSequence()
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { it.config.getOrNull(SemanticsProperties.StateDescription) }
          .firstOrNull()

  fun requestFocus(label: String): Boolean =
      textNodes(label)
          .asSequence()
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { it.config.getOrNull(SemanticsActions.RequestFocus)?.action }
          .firstOrNull()
          ?.invoke() ?: false

  fun hasScrollableContent(): Boolean =
      nodes().any { it.config.getOrNull(SemanticsActions.ScrollBy) != null }

  fun scrollBy(pixels: Float) {
    val scroll =
        nodes().firstNotNullOfOrNull { it.config.getOrNull(SemanticsActions.ScrollBy)?.action }
    assertTrue(requireNotNull(scroll).invoke(0f, pixels))
  }

  fun scrollableContentCount(): Int =
      nodes().count { it.config.getOrNull(SemanticsActions.ScrollBy) != null }

  fun assertTextLineCount(label: String, expected: Int) {
    val layouts = mutableListOf<TextLayoutResult>()
    textNodes(label)
        .single()
        .config
        .getOrNull(SemanticsActions.GetTextLayoutResult)
        ?.action
        ?.invoke(layouts)
    assertTrue(layouts.isNotEmpty())
    layouts.forEach { assertEquals(expected, it.lineCount) }
  }

  fun pressKey(key: Key): Boolean {
    val keyDown = scene.sendKeyEvent(KeyEvent(key, KeyEventType.KeyDown))
    val keyUp = scene.sendKeyEvent(KeyEvent(key, KeyEventType.KeyUp))
    return keyDown || keyUp
  }

  fun dismissPopup(): Boolean =
      nodes()
          .asSequence()
          .mapNotNull { it.config.getOrNull(SemanticsActions.Dismiss)?.action }
          .firstOrNull()
          ?.invoke() ?: false

  fun assertTextFits(label: String, maxLines: Int = 1) {
    assertTextLayout(label, lineCounts = 1..maxLines)
  }

  fun assertUniformSummaryCards(expectedCount: Int) {
    val cards =
        nodes().filter {
          it.config.getOrNull(SemanticsProperties.TestTag)?.startsWith("summary-metric-") == true
        }
    assertEquals(expectedCount, cards.size)
    val reference = cards.first().boundsInRoot
    cards.forEach { card ->
      val bounds = card.boundsInRoot
      assertEquals(reference.width, bounds.width, "Summary card widths must match")
      assertEquals(reference.height, bounds.height, "Summary card heights must match")
      assertTrue(
          bounds.left >= 0 && bounds.top >= 0 && bounds.right <= width && bounds.bottom <= height,
          "Every summary card must be visible: $bounds")
    }
  }

  fun assertBugPrioritiesInsideCard(high: String, medium: String, low: String) {
    listOf(high, medium, low).forEach { label ->
      assertTrue(
          generateSequence(textNodes(label).single()) { it.parent }
              .any { it.config.getOrNull(SemanticsProperties.TestTag) == "summary-metric-Bugs" },
          "Priority counts must stay inside the Bugs box")
    }
    listOf(high to medium, medium to low).forEach { (first, next) ->
      val firstBounds = textNodes(first).single().boundsInRoot
      val nextBounds = textNodes(next).single().boundsInRoot
      assertTrue(firstBounds.bottom <= nextBounds.top, "Priorities must form a vertical list")
      assertEquals(firstBounds.left, nextBounds.left)
    }
  }

  fun assertSummaryStatusPlacement(label: String) {
    val status =
        textNodes(label).single { node ->
          generateSequence(node) { it.parent }
              .any { it.config.getOrNull(SemanticsProperties.TestTag) == "summary-analysis-status" }
        }
    assertTrue(
        generateSequence(status) { it.parent }
            .any { it.config.getOrNull(SemanticsProperties.TestTag) == "analysis-summary" },
        "The status must belong to Analysis summary")
    assertTrue(textNodes("Purpose").single().boundsInRoot.bottom <= status.boundsInRoot.top)
  }

  fun assertTextBefore(label: String, following: String) {
    val first = textNodes(label).single().boundsInRoot
    val second = textNodes(following).single().boundsInRoot
    assertTrue(first.right < second.left, "$label must be left of $following")
    assertTrue(
        kotlin.math.abs(first.center.y - second.center.y) < 2f,
        "$label and $following must share a row")
  }

  fun assertTextAbove(label: String, following: String) {
    val first = textNodes(label).single().boundsInRoot
    val second = textNodes(following).single().boundsInRoot
    assertTrue(first.bottom < second.top, "$label must be above $following")
  }

  fun assertTextContrast(label: String, background: Color) {
    val matches = textNodes(label)
    assertTrue(matches.isNotEmpty(), "$label must be rendered")
    matches.forEach { node ->
      val layouts = mutableListOf<TextLayoutResult>()
      node.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action?.invoke(layouts)
      assertTrue(layouts.isNotEmpty())
      layouts.forEach { layout ->
        assertTrue(
            contrastRatio(layout.layoutInput.style.color, background) >= 4.5,
            "$label must retain readable text on $background")
      }
    }
  }

  fun assertColorVisible(color: Color) {
    val rendered =
        surface.makeImageSnapshot().use { snapshot ->
          requireNotNull(snapshot.encodeToData()).use { data ->
            javax.imageio.ImageIO.read(java.io.ByteArrayInputStream(data.bytes))
          }
        }
    val pixels = rendered.getRGB(0, 0, width, height, null, 0, width)
    assertTrue(pixels.count { it == color.toArgb() } >= 10, "$color must be visible in the render")
  }

  fun assertTextWrapsWithoutClipping(label: String) {
    assertTextLayout(label, lineCounts = 2..Int.MAX_VALUE)
  }

  private fun assertTextLayout(label: String, lineCounts: IntRange) {
    val matches = textNodes(label)
    assertTrue(matches.isNotEmpty(), "$label must be visible at $width")
    matches.forEach { node ->
      val layouts = mutableListOf<TextLayoutResult>()
      node.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action?.invoke(layouts)
      assertTrue(layouts.isNotEmpty())
      layouts.forEach { layout ->
        assertFalse(
            (0 until layout.lineCount).any(layout::isLineEllipsized),
            "$label is truncated at $width")
        // Glyph measurements can round down by a subpixel.
        assertTrue(
            layout.multiParagraph.height <= layout.size.height + 1f,
            "$label clips vertically at $width")
        assertTrue(
            layout.lineCount in lineCounts,
            "$label has ${layout.lineCount} lines; expected $lineCounts at $width")
      }
      assertTrue(node.boundsInRoot.right <= width && node.boundsInRoot.bottom <= height)
    }
  }

  private fun nodes() = owners.flatMap { descendants(it.unmergedRootSemanticsNode) }

  private fun textNodes(label: String) =
      nodes().filter { node ->
        node.config.getOrNull(SemanticsProperties.Text)?.any { it.text == label } == true
      }

  private fun descendants(node: SemanticsNode): List<SemanticsNode> =
      listOf(node) + node.children.flatMap(::descendants)

  override fun close() {
    scene.close()
    surface.close()
  }
}

private val visualFixtureProject =
    ProjectAnalysis(
        "visual-fixture",
        "fixture-revision",
        "go-shop · fixture",
        "",
        "Go",
        fileCount = 23,
        sourceFileCount = 23,
        totalLines = 1800,
        summary = "Test fixture",
        aiStatus = "fresh",
        analyzedAt = "")

private val visualFixtureOverview =
    ProjectOverview(
        projectId = "visual-fixture",
        projectRevision = "fixture-revision",
        metrics =
            ProjectMetrics(
                type = "Go",
                buildFile = "go.mod",
                fileCount = 23,
                sourceFileCount = 23,
                totalLines = 1800,
                languages = mapOf("Go" to 21, "Markdown" to 2)),
        analysis =
            StructuredProjectAnalysis(
                status = "fresh",
                purpose = "Go service with a small HTTP API and a repository layer.",
                architecture =
                    "flowchart LR\n A[HTTP handlers] --> B[Services]\n B --> C[Repository adapters]",
                components =
                    listOf(
                        "internal/api (API handlers): Receives requests.",
                        "internal/storage (Repository adapters): Persists records."),
                entryPoints = listOf("cmd/server/main.go"),
                flows =
                    listOf(
                        "sequenceDiagram\n participant C as Client\n participant A as API\n participant S as Storage\n C->>A: HTTP request\n A->>S: Store record\n S-->>A: Result\n A-->>C: Response"),
                risks = listOf(ProjectAnalysisRisk("medium", "Input validation is incomplete.")),
                nextSteps = listOf("Review boundary validation.")),
        analysisCoverage = AnalysisCoverage(total = 23, fresh = 16, stale = 4, missing = 3),
        findingCounts = FindingCounts(verified = 2, aiSuggestions = 4),
        analysisRun =
            analysisRunFixture().let { run ->
              run.copy(
                  identity =
                      run.identity.copy(
                          projectId = "visual-fixture", projectRevision = "fixture-revision"),
                  sections =
                      listOf(
                          AnalysisSectionProgress(
                              "bugs", "completed", AnalysisRunCoverage(succeeded = 1), 7),
                          AnalysisSectionProgress(
                              "performance", "partial", AnalysisRunCoverage(partial = 1), 3),
                          AnalysisSectionProgress(
                              "security",
                              "completed_empty",
                              AnalysisRunCoverage(succeeded = 1),
                              0)))
            })

@Composable
private fun ToolbarVisualFixture(
    width: Float,
    project: ProjectAnalysis? = visualFixtureProject,
    connection: ConnectionState = ConnectionState(connected = true),
    actions: ToolbarActions = ToolbarActions({}, {}, {}, {}, {}, {}),
    paletteFocusRequester: FocusRequester? = null,
    analysisStatus: ToolbarAnalysisStatus? = null,
    showEditorDrawerActions: Boolean = false,
) {
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            width,
            project,
            false,
            "",
            connection,
            GitStatus(available = true, branch = "main"),
            showEditorDrawerActions,
            analysisStatus),
        actions,
        paletteFocusRequester = paletteFocusRequester)
  }
}

@Composable
private fun PopupMenuVisualFixture(longLabel: String) {
  Box(Modifier.fillMaxSize().background(AppBackground).padding(12.dp)) {
    IdePopupMenuSurface(
        modifier = Modifier.width(280.dp),
        content = {
          IdeDropdownMenuItem(label = longLabel, onClick = {}, icon = DesktopIcon.Document)
          IdeDropdownMenuItem(
              label = "Unavailable action",
              onClick = {},
              enabled = false,
              icon = DesktopIcon.Refresh)
        })
  }
}

@Composable
private fun SharedControlsVisualFixture() {
  Column(Modifier.fillMaxSize().background(AppBackground).padding(12.dp)) {
    Row(horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp)) {
      MiniOrcaButton(onClick = {}, tone = ActionTone.Primary) { Text("Apply") }
      MiniOrcaButton(onClick = {}, tone = ActionTone.Navigation, selected = true) {
        Text("Selected")
      }
      MiniOrcaButton(onClick = {}, enabled = false) { Text("Disabled") }
      ChromeButton(onClick = {}, focusHighlight = true) { Text("Focused") }
    }
    Spacer(Modifier.height(8.dp))
    CompactSingleLineField(
        value = "",
        onValueChange = {},
        label = "Search files",
        showLabel = false,
        modifier = Modifier.fillMaxWidth())
  }
}

@Composable
private fun SharedChromeStatesVisualFixture() {
  Column(
      Modifier.fillMaxSize().background(AppBackground).padding(8.dp),
      verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp),
  ) {
    Row(horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp)) {
      ChromeButton(onClick = {}, accessibleName = "Default") { Text("Default") }
      ChromeButton(
          onClick = {},
          interactionOverride = IdeActionInteraction(hovered = true),
          accessibleName = "Hovered") {
            Text("Hovered")
          }
      ChromeButton(
          onClick = {},
          interactionOverride = IdeActionInteraction(pressed = true),
          accessibleName = "Pressed") {
            Text("Pressed")
          }
      ChromeTab(onClick = {}, selected = true, accessibleName = "Selected tab") {
        Text("Selected tab")
      }
    }
    Row(horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp)) {
      ChromeButton(onClick = {}, enabled = false, accessibleName = "Disabled") { Text("Disabled") }
      ChromeButton(onClick = {}, focusHighlight = true, accessibleName = "Focused") {
        Text("Focused")
      }
    }
    IdeDisclosureHeader("Collapsed section", expanded = false, onToggle = {})
    IdeDisclosureHeader("Expanded section", expanded = true, onToggle = {})
    IdeHorizontalSeparator()
    Row(Modifier.height(20.dp)) {
      Text("Vertical separator", style = IdeTypography.section)
      Spacer(Modifier.width(8.dp))
      IdeVerticalSeparator()
    }
  }
}

@Composable
internal fun EditorVisualFixture(width: Float) {
  val layout = DesktopLayoutState(bottomCollapsed = false)
  val panes = dockedPaneWidths(width, layout.explorerWidth, layout.actionWidth)
  val symbol =
      SymbolInfo(
          "GetUser", "function", "func GetUser(id string) (User, error)", 5, 12, "exact", true)
  val file =
      ProjectFileInfo(
          "internal/api/user.go",
          "fixture-hash",
          "user.go",
          language = "Go",
          sizeBytes = 480,
          lineCount = 18,
          modifiedAt = "",
          binary = false,
          content =
              """
        package api

        import "errors"

        func GetUser(id string) (User, error) {
            if id == "" {
                return User{}, errors.New("missing user id")
            }

            user, err := repository.Find(id)
            return user, err
        }

        type User struct {
            ID   string
            Name string
        }
      """
                  .trimIndent())
  val index =
      ProjectIndex(
          "visual-fixture",
          "fixture-revision",
          files =
              listOf(
                      "cmd/server/main.go",
                      "internal/api/routes.go",
                      file.path,
                      "internal/db/store.go",
                      "internal/models/user.go",
                      "go.mod",
                      "README.md")
                  .map { IndexedFile(it, "fixture-hash", "Go", false, analysisStatus = "fresh") })
  val analysis =
      FileAnalysis(
          file.path,
          "fresh",
          purpose = "Resolves user requests and delegates persistence to the repository.",
          symbolExplanations =
              mapOf(
                  "GetUser" to
                      "Validates the identifier before looking up a user. Returns the repository result and preserves its error."))
  val inspector =
      symbolInspectorUiState(
          file, listOf(symbol), symbol, analysis, false, InspectorProviderState(false, false), null)
  Column(Modifier.fillMaxSize().background(ToolWindowSurface)) {
    MainToolbar(
        ToolbarState(
            width,
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            useNarrowLayout(width)),
        ToolbarActions({}, {}, {}, {}, {}, {}))
    Row(Modifier.fillMaxWidth().weight(1f)) {
      ToolWindowBar(LeftToolWindow.Editor, {})
      IdeVerticalSeparator()
      Column(Modifier.weight(1f)) {
        Row(Modifier.fillMaxWidth().weight(1f)) {
          if (!useNarrowLayout(width)) {
            DockedToolWindow(
                title = "Files",
                content = { modifier ->
                  ExplorerPane(
                      ExplorerPaneState(index, file.path, "", emptySet(), false),
                      ExplorerPaneActions({}, {}, {}, {}, {}),
                      modifier)
                },
                modifier = Modifier.width(panes.explorer.dp),
                showHeader = false)
            ResizableDivider({}, {})
          }
          EditorArea(
              content = {
                EditorWorkspace(
                    EditorChromeUiState(
                        file.name,
                        file.path,
                        editorBreadcrumbSegments(file.path, symbol.name),
                        "Read-only source fixture",
                        EditorSurface.Source,
                        false,
                        "SOURCE",
                        null),
                    null,
                    {},
                    {},
                    canvas = {
                      SourceEditorPane(
                          visualFixtureProject, file, listOf(symbol), symbol, 7, emptyList(), {})
                    })
              },
              modifier = Modifier.weight(1f))
        }
        if (useNarrowLayout(width)) {
          TerminalBar(TerminalWorkspaceState(), true, {}, TerminalTabActions({}, {}, {}))
        } else {
          TerminalDock(
              layout,
              TerminalWorkspaceState(),
              {},
              {},
              TerminalTabActions({}, {}, {}),
              {},
              {},
              { modifier -> Text("Synthetic shell", modifier = modifier.padding(8.dp)) })
        }
      }
      if (!useNarrowLayout(width)) {
        ResizableDivider({}, {})
        DockedToolWindow(
            title = "Tool windows",
            content = { modifier ->
              RightToolWindowContainer(
                  RightToolWindow.Context,
                  {},
                  content = { _, contentModifier ->
                    ContextToolWindow(
                        ContextToolWindowState(
                            inspector,
                            ScopedModel(),
                            false,
                            null,
                            null,
                            analysis,
                            visualFixtureProject,
                            ProjectOverview(
                                analysis =
                                    StructuredProjectAnalysis(
                                        status = "fresh",
                                        purpose =
                                            "Go service with a small HTTP API and a repository layer."),
                                metrics =
                                    ProjectMetrics(
                                        type = "Go",
                                        buildFile = "go.mod",
                                        languages = mapOf("Go" to 7))),
                            functionModel =
                                ScopedModel(
                                    scope = "function",
                                    model = "local-function-model",
                                    providerOrigin = "http://127.0.0.1:8080"),
                            declarationExplanation =
                                DeclarationExplanationState(
                                    status = DeclarationExplanationStatus.Current,
                                    result =
                                        DeclarationExplanation(
                                            version = "v1",
                                            projectId = "visual-fixture",
                                            projectRevision = "fixture-revision",
                                            baseFileHash = file.contentHash,
                                            anchor =
                                                DeclarationSourceAnchor(
                                                    file.path,
                                                    symbol.name,
                                                    symbol.signature,
                                                    symbol.startLine,
                                                    symbol.endLine),
                                            summary =
                                                "Validates the user identifier and delegates the lookup to the repository.",
                                            behavior = listOf("rejects blank identifiers"),
                                            inputs = listOf("user identifier"),
                                            outputs = listOf("user or repository error"),
                                            contextManifest =
                                                ContextManifest(
                                                    scope = "function",
                                                    model = "local-function-model",
                                                    providerOrigin = "http://127.0.0.1:8080")),
                                    message =
                                        "Current explanation · lines ${symbol.startLine}–${symbol.endLine}")),
                        ContextToolWindowActions({}, {}, {}, {}, {}),
                        contentModifier)
                  },
                  modifier = modifier)
            },
            modifier = Modifier.width(panes.action.dp),
            showHeader = false)
      }
    }
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            DesktopStatusProviderPresentation(
                "Visual fixture · local-function-model · no backend", remoteProvider = false),
            "Models: 1 local · 2 cloud",
            "Visual fixture · no backend"),
        {})
  }
}

private fun contextVisualState(): ContextToolWindowState {
  val file =
      ProjectFileInfo(
          "internal/api/user.go",
          "fixture-hash",
          "user.go",
          language = "Go",
          sizeBytes = 480,
          lineCount = 18,
          modifiedAt = "",
          binary = false)
  val symbol = SymbolInfo("Run", "function", "func Run() error", 5, 12, "exact", true)
  val analysis =
      FileAnalysis(
          file.path,
          "fresh",
          purpose = "Routes incoming requests.",
          symbolExplanations = mapOf("Run" to "Cached declaration explanation"))
  return ContextToolWindowState(
      symbolInspectorUiState(
          file,
          listOf(symbol),
          symbol,
          analysis,
          false,
          InspectorProviderState(false, false),
          null),
      ScopedModel(),
      false,
      null,
      null,
      analysis,
      visualFixtureProject,
      visualFixtureOverview,
      declarationExplanation = DeclarationExplanationState())
}

private val visualFixtureFindings =
    listOf(
        UnifiedFinding(
            id = "fixture-1",
            severity = "high",
            source = "file_analysis",
            confidence = "suggested",
            title = "Validate the user identifier",
            message = "Check malformed identifiers before querying the repository.",
            location = FindingLocation("internal/api/user.go", 6),
            status = "open",
            freshness = "fresh"),
        UnifiedFinding(
            id = "fixture-2",
            severity = "medium",
            source = "file_analysis",
            confidence = "suggested",
            title = "Add context to repository errors",
            message = "Include the operation name when returning repository failures.",
            location = FindingLocation("internal/api/user.go", 11),
            status = "open",
            freshness = "fresh"),
    )
