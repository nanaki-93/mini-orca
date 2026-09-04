package io.miniorca.desktop

import androidx.compose.foundation.background
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
  fun analysisChromeFitsWideNarrowAndEnlargedTextViewports() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1920, 1080, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1000, 800, 1.3f))
        .forEach { (width, height, scale) ->
          ComposeVisualFixture(width, height, scale) { AnalysisVisualFixture(width.toFloat()) }
              .use { fixture ->
                fixture.render("analysis-$width-$scale")
                listOf(if (scale > 1.15f) "Perf." else "Performance", "Preview", "Pause", "Cancel")
                    .forEach { label -> fixture.assertTextFits(label) }
              }
        }
  }

  @Test
  fun analysisLifecycleControlsRenderAtNarrowEnlargedTextScale() {
    val cases =
        listOf(
            Triple("empty", null, "Start Analyze-all"),
            Triple("paused", visualFixtureJob.copy(status = "paused"), "Resume"),
            Triple("failed", visualFixtureJob.copy(status = "failed"), "Failed"),
        )

    cases.forEach { (name, job, expectedControl) ->
      ComposeVisualFixture(800, 650, 1.3f) { AnalysisPaneVisualFixture(job) }
          .use { fixture ->
            fixture.render("analysis-$name-800-1.3")
            fixture.assertTextFits(expectedControl)
          }
    }
  }

  @Test
  fun workspaceStatesKeepCompactCopyAndEvidenceReachableAcrossWideAndNarrowViews() {
    var performanceActions = 0
    val model =
        ScopedModel(
            scope = ModelScope.Analyze.wireValue,
            profile = "review-profile",
            model = "provider/reviewer",
            remoteProvider = true,
        )
    val destination = modelDestinationLabel(ModelScope.Analyze, model)
    val performanceActionsFixture =
        PerformanceWorkspaceActions(
            confirmRemoteProvider = { performanceActions++ },
            preview = { performanceActions++ },
            start = { _, _ -> performanceActions++ },
            pause = { performanceActions++ },
            resume = { performanceActions++ },
            cancel = { performanceActions++ },
            openInEditor = { _, _ -> performanceActions++ },
            prepareOptimization = { _, _ -> performanceActions++ },
        )
    val performanceFinding =
        PerformanceFinding(
            id = "performance-1",
            category = "allocation",
            potentialImpact = "high",
            confidence = "medium",
            title = "Avoid repeated buffer allocation",
            observedPattern = "A buffer is allocated for every request.",
            workloadConditions = "High request volume.",
            recommendation = "Reuse a bounded buffer.",
            tradeoff = "Retained buffers increase memory pressure.",
            verificationPlan = "Benchmark representative traffic.",
            startLine = 24,
            symbol = "Serve",
        )
    ComposeVisualFixture(1440, 900) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(
                  job = PerformanceJob(status = "running", elapsed = 8_000_000_000),
                  report =
                      PerformanceReport(
                          status = "completed",
                          paths = mapOf(performanceFinding.id to "internal/api/server.go"),
                          findings = listOf(performanceFinding)),
                  context =
                      PerformanceQueuePreview(
                          files = listOf(PerformanceJobFile(path = "internal/api/server.go")),
                          excluded = 2,
                          oversized = 1,
                          outsideLimit = 3),
                  model = model,
                  remoteProviderConfirmed = false),
              performanceActionsFixture)
        }
        .use { fixture ->
          fixture.render("performance-populated-1440")
          assertTrue(fixture.hasText("Source-based review · Not measured"))
          assertTrue(fixture.hasText(destination))
          assertTrue(fixture.hasText("Running · 8s budget used"))
          kotlin.test.assertEquals(0, performanceActions)
          fixture.clickText(
              "ALLOCATION · high · internal/api/server.go:24 · Avoid repeated buffer allocation")
          fixture.render()
          assertTrue(fixture.hasText("Observed pattern: A buffer is allocated for every request."))
          assertTrue(
              fixture.hasText(
                  "When it matters: High request volume.\nRecommendation: Reuse a bounded buffer.\nTrade-off: Retained buffers increase memory pressure.\nVerify: Benchmark representative traffic."))
          kotlin.test.assertEquals(0, performanceActions)
        }

    ComposeVisualFixture(800, 900, 1.3f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(
                  job = null,
                  report = null,
                  context =
                      PerformanceQueuePreview(
                          files = listOf(PerformanceJobFile(path = "internal/api/server.go"))),
                  model = model,
                  remoteProviderConfirmed = false),
              performanceActionsFixture)
        }
        .use { fixture ->
          fixture.render("performance-empty-800-1.3")
          assertTrue(fixture.hasText("No performance review"))
          assertTrue(fixture.hasText("Confirm remote destination"))
          fixture.assertTextFits("Preview limits")
          fixture.assertTextFits("Analyze performance")
          kotlin.test.assertEquals(0, performanceActions)
          fixture.clickText("Preview limits")
          kotlin.test.assertEquals(1, performanceActions)
        }

    ComposeVisualFixture(800, 650, 1.3f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(
                  findings = emptyList(),
                  scan =
                      GoScanReport(
                          status = "failed",
                          phases = listOf(GoScanPhase("go vet", "failed", output = "vet failed"))),
                  loading = false),
              BugsWorkspaceActions(FindingActions({}, {}, { _, _ -> }), {}, {}))
        }
        .use { fixture ->
          fixture.render("bugs-failed-800-1.3")
          assertTrue(fixture.hasText("Verified scan failed; results remain available."))
          assertTrue(fixture.hasText("Warning: go vet: failed: vet failed"))
          assertTrue(fixture.hasText("No findings yet."))
        }

    ComposeVisualFixture(800, 280, 1.3f) {
          ProblemsToolWindow(
              ProblemsToolWindowState(emptyList(), false), FindingActions({}, {}, { _, _ -> }))
        }
        .use { fixture ->
          fixture.render("problems-empty-800-1.3")
          assertTrue(fixture.hasText("No findings yet."))
        }
  }

  @Test
  fun analysisWrapsLongRemoteDestinationWithoutHidingActiveControls() {
    val model =
        ScopedModel(
            scope = ModelScope.Bug.wireValue,
            profile = "local-workstation-with-a-descriptive-profile-name",
            model = "provider/model-with-a-long-qualified-destination-name",
            remoteProvider = true,
            reasoningEffort = "high",
        )
    val destination = modelDestinationLabel(ModelScope.Bug, model)

    ComposeVisualFixture(1000, 800, 1.3f) { AnalysisVisualFixture(1000f, model = model) }
        .use { fixture ->
          fixture.render("analysis-long-destination-1000-1.3")
          fixture.assertTextFits("Pause")
          fixture.assertTextFits("Cancel")
          fixture.assertTextWrapsWithoutClipping(destination)
        }
  }

  @Test
  fun previewMenuAndDialogDoNotInvokeAnalysisActions() {
    var requests = 0
    val actions =
        AnalysisWorkspaceActions(
            { requests++ }, { requests++ }, { requests++ }, { requests++ }, { requests++ })
    ComposeVisualFixture(1440, 900) { AnalysisVisualFixture(1440f, actions) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Pause")
          kotlin.test.assertEquals(1, requests)
          fixture.clickText("Preview")
          fixture.render()
          fixture.clickText("New file")
          fixture.render()
          assertTrue(fixture.hasText("New file · Preview"))
          fixture.clickText("Close")
          fixture.render()
          assertFalse(fixture.hasText("New file · Preview"))
          kotlin.test.assertEquals(1, requests)
        }
  }

  @Test
  fun analysisHeaderPreventsRepeatedActionsAndKeepsErrorsReachable() {
    var pauses = 0
    val failedJob =
        visualFixtureJob.copy(
            files =
                visualFixtureJob.files +
                    AnalyzeAllFileJob(
                        path = "internal/api/failed.go",
                        status = "failed",
                        attempts = 2,
                        error = "sanitized analysis failure"))
    val actions = AnalysisWorkspaceActions({}, {}, { pauses++ }, {}, {})

    ComposeVisualFixture(800, 650, 1.3f) { AnalysisPaneVisualFixture(failedJob, actions) }
        .use { fixture ->
          fixture.render("analysis-errors-800-1.3")
          fixture.clickText("Pause")
          fixture.render()
          assertEquals(1, pauses)
          assertFalse(fixture.tryClick("Pause"))
          fixture.clickText("Analysis errors")
          fixture.render("analysis-errors-expanded-800-1.3")
          assertTrue(fixture.hasText("sanitized analysis failure"))
        }
  }

  @Test
  fun editorComponentsRenderAtDockedAndDrawerWidths() {
    listOf(1440 to 900, 1000 to 760, 999 to 760).forEach { (width, height) ->
      ComposeVisualFixture(width, height) { EditorVisualFixture(width.toFloat()) }
          .use { fixture ->
            fixture.render("editor-$width")
            fixture.assertTextFits("Preview")
            fixture.assertTextFits("Performance")
            fixture.assertTextFits("user.go")
            if (width >= 1_000) {
              fixture.assertTextFits("Files")
            } else {
              fixture.assertTextFits("Files")
              fixture.assertTextFits("Context")
            }
          }
    }
  }

  @Test
  fun keyboardEventsNavigateAndActivateTheProductionRailAndCommandPalette() {
    var activeToolWindow by mutableStateOf(LeftToolWindow.Summary)
    val railFocus = FocusRequester()
    ComposeVisualFixture(120, 650, 1.3f) {
          ToolWindowBar(
              activeToolWindow, { activeToolWindow = it }, Modifier.focusRequester(railFocus))
        }
        .use { fixture ->
          fixture.render("rail-keyboard-initial-120-1.3")
          railFocus.requestFocus()
          fixture.render()
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render("rail-keyboard-arrow-120-1.3")
          assertTrue(fixture.hasDescription("Analysis tool window, not selected, focused"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render("rail-keyboard-activated-120-1.3")
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
                analysis = null,
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
              canvas = {
                SourceEditorPane(project, file, listOf(symbol), symbol, 3, emptyList(), {})
              })
        }
        .use { fixture ->
          fixture.render("editor-candidate-800-1.3")
          assertTrue(fixture.hasText("Candidate"))
          assertTrue(fixture.hasText("REVIEW READY · 4 changed lines"))
          fixture.clickText("Review candidate")
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
    var reviewActions = 0
    var mutations = 0
    val reviewState =
        ReviewToolWindowState(
            project,
            file,
            symbol,
            null,
            editableDraft(draft),
            draft,
            failedChecks,
            null,
            null,
            null,
            false)
    ComposeVisualFixture(800, 700, 1.3f) {
          ReviewToolWindow(
              reviewState,
              ReviewToolWindowActions(
                  { reviewActions++ }, { reviewActions++ }, { reviewActions++ }),
              DraftApplicationActions({ mutations++ }, { mutations++ }))
        }
        .use { fixture ->
          fixture.render("review-failed-800-1.3")
          assertTrue(fixture.hasText("Validation"))
          assertTrue(fixture.hasText("Apply unavailable"))
          fixture.clickText("Show command output (1)")
          fixture.render()
          assertTrue(fixture.hasText("expected failure evidence"))
          kotlin.test.assertEquals(0, reviewActions)
          kotlin.test.assertEquals(0, mutations)
        }

    ComposeVisualFixture(800, 360, 1.3f) {
          ReviewToolWindow(
              reviewState.copy(
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
          assertTrue(fixture.hasText("Undo available."))
          assertTrue(fixture.hasText("Undo this change"))
          kotlin.test.assertEquals(0, mutations)
        }

    var contextActions = 0
    val inspector =
        requireNotNull(
            symbolInspectorUiState(
                selectedFile = file,
                symbols = listOf(symbol),
                selectedSymbol = symbol,
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
          fixture.render("context-consent-800-1.3")
          assertTrue(fixture.hasText("Actions"))
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertTrue(
              fixture.hasText("Preview only · Complexity and readability scores unavailable."))
          kotlin.test.assertEquals(0, contextActions)
        }

    var assistantActions = 0
    val session =
        ChatSession(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            baseFileHash = file.contentHash,
            openPath = file.path,
            mode = draft.mode,
            targetSymbol = draft.targetSymbol,
            state = "active",
            latestDraftId = draft.id)
    ComposeVisualFixture(800, 900, 1.3f) {
          AssistantToolWindow(
              AssistantToolWindowState(
                  project,
                  file,
                  session,
                  draft,
                  EditableDraftState(draft, status = DraftEditorStatus.Stale),
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
          fixture.render("assistant-stale-800-1.3")
          assertTrue(fixture.hasText("Conversation"))
          assertTrue(fixture.hasText("Editable draft · stale"))
          assertTrue(fixture.hasText("Draft is stale. Start a new conversation."))
          assertTrue(fixture.hasText("Confirm remote destination"))
          kotlin.test.assertEquals(0, assistantActions)
        }

    val checksPresentation =
        checksToolWindowPresentation(
            ChecksToolWindowState(project, file, editableDraft(draft), draft, failedChecks, false))
    ComposeVisualFixture(800, 500, 1.3f) { ChecksToolWindow(checksPresentation) }
        .use { fixture ->
          fixture.render("checks-failed-800-1.3")
          assertTrue(fixture.hasText("Evidence only; run checks and Apply stay in Review."))
          assertTrue(fixture.hasText("go test · Failed · required"))
          assertTrue(fixture.hasText("expected failure evidence"))
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
  fun baselineCapturesSummaryAndExercisesOpenProjectAndPreviewMenus() {
    ComposeVisualFixture(1440, 900) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render("summary-1440")
          assertTrue(fixture.hasText("Project facts"))
          assertTrue(fixture.hasText("Analysis coverage"))
          assertTrue(fixture.hasText("AI interpretation"))
        }

    ComposeVisualFixture(1440, 900) { ToolbarVisualFixture(1440f) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("go-shop · fixture")
          fixture.render("project-menu-open")
          assertTrue(fixture.hasText("Open project"))
          assertTrue(fixture.hasText("Re-index project"))
        }

    ComposeVisualFixture(1440, 900) { ToolbarVisualFixture(1440f) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Preview")
          fixture.render("preview-menu-open")
          assertTrue(fixture.hasText("New file"))
          assertTrue(fixture.hasText("Branch actions"))
        }
  }

  @Test
  fun popupMenusUseProductionRowsForDisabledLiveAndPreviewFlows() {
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
  fun popupSurfaceWrapsLongRowsAndPreviewDismissalRestoresTheTriggerFocus() {
    val longLabel =
        "A long preview menu action remains readable instead of being shortened at narrow widths"
    ComposeVisualFixture(320, 200, 1.3f) { PopupMenuVisualFixture(longLabel) }
        .use { fixture ->
          fixture.render("popup-surface-320-1.3")
          fixture.assertTextWrapsWithoutClipping(longLabel)
          fixture.assertTextFits("Unavailable action")
          assertTrue(fixture.isDisabled("Unavailable action"))
        }

    ComposeVisualFixture(800, 220, 1.3f) { ToolbarVisualFixture(800f) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Preview"))
          assertTrue(fixture.isFocused("Preview"))
          fixture.clickText("Preview")
          fixture.render()
          assertTrue(fixture.hasText("New file"))
          assertTrue(fixture.dismissPopup())
          fixture.render()
          fixture.render()
          assertFalse(fixture.hasText("New file"))
          assertTrue(fixture.isFocused("Preview"))
        }
  }

  @Test
  fun previewPopupKeepsLongContentInTheProductionScrollableMenu() {
    val features =
        (1..16).map { index ->
          PreviewFeature(
              "Preview action $index with a long but local-only description",
              "This action has no implementation.")
        }
    ComposeVisualFixture(360, 520, 1.3f) {
          Box(Modifier.fillMaxSize().background(AppBackground).padding(12.dp)) {
            PreviewFeatureMenu(features)
          }
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Preview")
          fixture.render()
          assertTrue(fixture.hasText(features.last().label))
          assertTrue(fixture.hasScrollableContent())
        }
  }

  @Test
  fun summaryDashboardKeepsLongInterpretationExpandableAndNavigationLocal() {
    val longPurpose =
        "This returned purpose stays intact when the compact dashboard only previews it. ".repeat(8)
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    purpose = longPurpose,
                    architecture = "Handlers delegate to services and repository adapters.",
                    components = listOf("HTTP handlers", "Repository adapters"),
                    risks = listOf(ProjectAnalysisRisk("medium", "Validate boundary input."))))
    val destinations = mutableListOf<Workspace>()
    ComposeVisualFixture(1440, 900) {
          ProjectSummaryPane(overview, visualFixtureProject) { destinations += it }
        }
        .use { fixture ->
          fixture.render("summary-dashboard-1440")
          assertTrue(fixture.hasText("23"))
          assertTrue(fixture.hasText("Verified findings"))
          assertFalse(fixture.hasText("Handlers delegate to services and repository adapters."))
          fixture.clickText("Analysis")
          fixture.clickText("Bugs")
          kotlin.test.assertEquals(listOf(Workspace.Analysis, Workspace.Bugs), destinations)
          fixture.clickText("Show full purpose")
          fixture.render("summary-purpose-expanded-1440")
          assertTrue(fixture.hasText(longPurpose))
          fixture.clickText("Interpretation details")
          fixture.render("summary-details-expanded-1440")
          assertTrue(fixture.hasText("Architecture"))
          assertTrue(fixture.hasText("MEDIUM · Validate boundary input."))
        }

    ComposeVisualFixture(800, 650, 1.3f) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render("summary-dashboard-800-1.3")
          fixture.assertTextFits("Project facts")
          fixture.assertTextFits("Analysis coverage")
        }

    ComposeVisualFixture(800, 300) { ProjectSummaryPane(null, null, {}) }
        .use { fixture ->
          fixture.render("summary-dashboard-empty-800")
          assertTrue(fixture.hasText("No project selected"))
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
    val prose =
        listOf(
                insight.mechanism,
                insight.whyItMattersHere,
                insight.tradeoffOrFailureMode,
                insight.transferableLesson)
            .joinToString(" ")

    try {
      EngineeringInsightPreference.save(false)
      ComposeVisualFixture(720, 420, 1.3f) {
            EngineeringInsightPanel(insight, stale = true, scopeLabel = "Visual fixture")
          }
          .use { fixture ->
            fixture.render("engineering-insight-collapsed-720-1.3")
            assertTrue(fixture.hasText("Engineering insight"))
            assertTrue(fixture.hasText("AI interpretation · Visual fixture · stale"))
            assertTrue(fixture.stateDescription("Engineering insight") == "Collapsed")
            assertFalse(fixture.hasText(prose))
            assertTrue(fixture.requestFocus("Engineering insight"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render("engineering-insight-expanded-720-1.3")
            assertTrue(fixture.stateDescription("Engineering insight") == "Expanded")
            assertTrue(fixture.hasText(prose))
            assertFalse(fixture.hasText("Close insight"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            fixture.render()
            assertFalse(fixture.hasText(prose))
            assertTrue(fixture.isFocused("Engineering insight"))
          }
      ComposeVisualFixture(360, 220, 1.3f) {
            EngineeringInsightPanel(insight, stale = true, scopeLabel = "File")
          }
          .use { fixture ->
            fixture.render("engineering-insight-narrow-360-1.3")
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
  fun filtersAndToolWindowHeadersKeepInteractionLocalAtNarrowScale() {
    var workflowActions = 0
    ComposeVisualFixture(480, 650, 1.3f) {
          ProblemsToolWindow(
              ProblemsToolWindowState(visualFixtureFindings, false),
              FindingActions(
                  openFinding = { workflowActions++ },
                  prepareFinding = { workflowActions++ },
                  triageFinding = { _, _ -> workflowActions++ }))
        }
        .use { fixture ->
          fixture.render("findings-filters-collapsed-480-1.3")
          assertTrue(fixture.stateDescription("Filters") == "Collapsed")
          assertTrue(fixture.requestFocus("Filters"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render("findings-filters-expanded-480-1.3")
          assertTrue(fixture.stateDescription("Filters") == "Expanded")
          assertTrue(fixture.hasText("Source"))
          assertTrue(fixture.hasText("Lifecycle"))
          assertTrue(fixture.hasScrollableContent())
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.stateDescription("Filters") == "Collapsed")
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
            BottomToolWindowRegion(
                layout = DesktopLayoutState(bottomCollapsed = true),
                availableToolWindows = listOf(BottomToolWindow.Output),
                summaries =
                    mapOf(BottomToolWindow.Output to BottomToolWindowSummary("Output ready")),
                onSelect = { opens++ },
                onCollapse = {},
                onHeightDelta = {},
                onHeightCommit = {},
                content = { _, _ -> })
          }
        }
        .use { fixture ->
          fixture.render("tool-window-controls-360-1.3")
          fixture.clickDescription("Close Files drawer")
          fixture.clickText("Open tools")
          kotlin.test.assertEquals(1, closes)
          kotlin.test.assertEquals(1, opens)
        }

    var overlayDismissals = 0
    ComposeVisualFixture(480, 420, 1.3f) {
          BottomToolWindowOverlay(
              layout = DesktopLayoutState(activeBottomToolWindow = BottomToolWindow.Output),
              availableToolWindows = listOf(BottomToolWindow.Output),
              summaries = mapOf(BottomToolWindow.Output to BottomToolWindowSummary("Output ready")),
              onSelect = {},
              onDismiss = { overlayDismissals++ },
              content = { _, modifier -> Text("Read-only output", modifier = modifier) })
        }
        .use { fixture ->
          fixture.render("bottom-tools-overlay-480-1.3")
          assertTrue(fixture.hasText("Bottom tools"))
          assertTrue(fixture.hasText("Output"))
          fixture.clickText("Close")
          kotlin.test.assertEquals(1, overlayDismissals)
        }
  }

  @Test
  fun problemRowsRevealDetailsBeforeAnyWorkflowAction() {
    var sourceRequests = 0
    var mutations = 0
    ComposeVisualFixture(900, 500) {
          ProblemsToolWindow(
              ProblemsToolWindowState(visualFixtureFindings, false),
              FindingActions({ sourceRequests++ }, { mutations++ }, { _, _ -> mutations++ }))
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
private class ComposeVisualFixture(
    private val width: Int,
    private val height: Int,
    fontScale: Float = 1f,
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
          density = Density(1f, fontScale),
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

  fun hasText(label: String): Boolean =
      textNodes(label).isNotEmpty() ||
          nodes().any { it.config.getOrNull(SemanticsProperties.EditableText)?.text == label }

  fun hasDescription(label: String): Boolean =
      nodes().any {
        it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
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

  fun assertTextFits(label: String) {
    assertTextLayout(label, mustWrap = false)
  }

  fun assertTextWrapsWithoutClipping(label: String) {
    assertTextLayout(label, mustWrap = true)
  }

  private fun assertTextLayout(label: String, mustWrap: Boolean) {
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
        if (mustWrap) assertTrue(layout.lineCount > 1, "$label must wrap at $width")
        else assertTrue(layout.lineCount == 1, "$label must fit on one line at $width")
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

@Composable
private fun AnalysisVisualFixture(
    width: Float,
    actions: AnalysisWorkspaceActions = AnalysisWorkspaceActions({}, {}, {}, {}, {}),
    job: AnalyzeAllJob? = visualFixtureJob,
    model: ScopedModel = ScopedModel(),
) {
  val summaries =
      mapOf(BottomToolWindow.Problems to BottomToolWindowSummary("No actionable problems"))
  val layout = DesktopLayoutState(bottomCollapsed = true)
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            width,
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            false),
        ToolbarActions({}, {}, {}, {}, {}, {}))
    Row(Modifier.fillMaxWidth().weight(1f)) {
      ToolWindowBar(LeftToolWindow.Analysis, {})
      Column(Modifier.weight(1f)) {
        Box(Modifier.weight(1f)) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  job, AnalysisCoverage(total = 23, stale = 23), model, false),
              actions)
        }
        if (useNarrowLayout(width)) {
          NarrowBottomToolWindowSummary(layout, BottomToolWindow.entries, summaries, {})
        } else {
          BottomToolWindowRegion(
              layout, BottomToolWindow.entries, summaries, {}, {}, {}, {}, { _, _ -> })
        }
      }
    }
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            listOf(
                DesktopStatusSegment(
                    DesktopStatusSegmentType.Operation,
                    "Visual fixture · no backend",
                    "Rendered Compose layout fixture; all data is test data",
                    0),
                DesktopStatusSegment(
                    DesktopStatusSegmentType.Index, "23 indexed files", "Fixture files", 1),
            )),
        width,
        {})
  }
}

@Composable
private fun AnalysisPaneVisualFixture(
    job: AnalyzeAllJob?,
    actions: AnalysisWorkspaceActions = AnalysisWorkspaceActions({}, {}, {}, {}, {}),
) {
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    AnalysisWorkspacePane(
        AnalysisWorkspacePaneState(
            job, AnalysisCoverage(total = 23, stale = 23), ScopedModel(), false),
        actions)
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
                architecture = "HTTP handlers delegate through services to repository adapters.",
                components = listOf("API handlers", "Repository adapters"),
                entryPoints = listOf("cmd/server/main.go"),
                flows = listOf("HTTP request to handler to service to repository"),
                risks = listOf(ProjectAnalysisRisk("medium", "Input validation is incomplete.")),
                nextSteps = listOf("Review boundary validation.")),
        analysisCoverage = AnalysisCoverage(total = 23, fresh = 16, stale = 4, missing = 3),
        findingCounts = FindingCounts(verified = 2, aiSuggestions = 4))

@Composable
private fun ToolbarVisualFixture(
    width: Float,
    project: ProjectAnalysis? = visualFixtureProject,
    connection: ConnectionState = ConnectionState(connected = true),
    actions: ToolbarActions = ToolbarActions({}, {}, {}, {}, {}, {}),
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
            false),
        actions)
  }
}

@Composable
private fun PopupMenuVisualFixture(longLabel: String) {
  Box(Modifier.fillMaxSize().background(AppBackground).padding(12.dp)) {
    IdePopupMenuSurface(
        modifier = Modifier.width(280.dp),
        content = {
          IdeDropdownMenuItem(
              label = longLabel,
              onClick = {},
              icon = DesktopIcon.Settings,
              status = { PreviewBadge() })
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

private val visualFixtureJob =
    AnalyzeAllJob(
        status = "running",
        maxFiles = 100,
        maxRetries = 1,
        files =
            List(19) { index ->
              AnalyzeAllFileJob(
                  path = "internal/api/handler_$index.go",
                  status =
                      when {
                        index < 8 -> "completed"
                        index == 8 -> "running"
                        else -> "pending"
                      })
            })

@Composable
private fun EditorVisualFixture(width: Float) {
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
                modifier = Modifier.width(panes.explorer.dp))
            ResizableDivider({}, {})
          }
          EditorArea(
              content = {
                EditorWorkspace(
                    EditorChromeUiState(
                        file.name,
                        file.path,
                        editorBreadcrumbLabel(file.path, symbol.name),
                        "Read-only source fixture",
                        EditorSurface.Source,
                        false,
                        "SOURCE"),
                    null,
                    {},
                    canvas = {
                      SourceEditorPane(
                          visualFixtureProject, file, listOf(symbol), symbol, 7, emptyList(), {})
                    })
              },
              modifier = Modifier.weight(1f))
        }
        if (useNarrowLayout(width)) {
          NarrowBottomToolWindowSummary(layout, BottomToolWindow.entries, emptyMap(), {})
        } else {
          BottomToolWindowRegion(
              layout,
              BottomToolWindow.entries,
              emptyMap(),
              {},
              {},
              {},
              {},
              { _, modifier ->
                ProblemsToolWindow(
                    ProblemsToolWindowState(visualFixtureFindings, false),
                    FindingActions({}, {}, { _, _ -> }),
                    modifier)
              })
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
                                        languages = mapOf("Go" to 7)))),
                        ContextToolWindowActions({}, {}, {}, {}, {}),
                        contentModifier)
                  },
                  modifier = modifier)
            },
            modifier = Modifier.width(panes.action.dp))
      }
    }
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            listOf(
                DesktopStatusSegment(
                    DesktopStatusSegmentType.Operation,
                    "Visual fixture · no backend",
                    "Rendered Compose layout fixture; all data is test data",
                    0))),
        width,
        {})
  }
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
