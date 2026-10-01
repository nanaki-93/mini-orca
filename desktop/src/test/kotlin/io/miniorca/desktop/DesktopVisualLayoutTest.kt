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
import androidx.compose.foundation.layout.requiredWidth
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.InternalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asComposeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.pointer.PointerButton
import androidx.compose.ui.input.pointer.PointerEventType
import androidx.compose.ui.platform.ClipEntry
import androidx.compose.ui.platform.Clipboard
import androidx.compose.ui.platform.LocalClipboard
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.PlatformContext
import androidx.compose.ui.platform.WindowInfo
import androidx.compose.ui.platform.asAwtTransferable
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.scene.CanvasLayersComposeScene
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsNode
import androidx.compose.ui.semantics.SemanticsOwner
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.DpSize
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import java.awt.datatransfer.DataFlavor
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
  fun contextPreviewDestinationAndRequestBoundariesStayReachableInProductionDialog() {
    val identity =
        ContextInspectionIdentity(
            WorkflowFileIdentity(
                WorkflowProjectIdentity("visual-project", "revision"),
                "src/contexts/request.go",
                "hash"),
            null,
            creationName = "Build",
            creationKind = "function",
            action = "fix",
            intent = "Create function",
            model = ScopedModel(scope = "function", model = "captured"))
    val cases =
        listOf(
            ContextInspectionState(status = ContextInspectionStatus.Loading, identity = identity) to
                listOf("Loading context preview…"),
            ContextInspectionState(
                status = ContextInspectionStatus.Ready,
                identity = identity,
                manifest =
                    ContextManifest(
                        scope = "function",
                        model = "response-model",
                        providerOrigin = "https://provider.test",
                        remoteProvider = true,
                        estimatedTokens = 0,
                        tokenLimit = 512,
                        byteLimit = 2048,
                        truncated = false)) to
                listOf(
                    "Scope: Function edits",
                    "Model: response-model",
                    "Sanitized provider origin: https://provider.test",
                    "Provider classification: Remote provider · confirmation required before sending project context",
                    "0 included · 0 excluded · 0 estimated tokens · not truncated",
                    "Token limit: 512 tokens",
                    "Byte limit: 2048 bytes"),
            ContextInspectionState(
                status = ContextInspectionStatus.Ready,
                identity = identity,
                manifest = ContextManifest(tokenLimit = -1, byteLimit = 0)) to
                listOf(
                    "Scope: unavailable",
                    "Model: unavailable",
                    "Sanitized provider origin: unavailable",
                    "Provider classification: Unavailable",
                    "Token limit: unavailable",
                    "Byte limit: unavailable"),
            ContextInspectionState(
                status = ContextInspectionStatus.Failed,
                identity = identity,
                message = "Daemon unavailable") to
                listOf("Context preview failed: Daemon unavailable"),
            ContextInspectionState(
                status = ContextInspectionStatus.Stale,
                identity = identity,
                message = "Destination changed") to
                listOf("Context preview stale: Destination changed"))
    for ((width, height) in listOf(800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.5f)) {
        for ((inspection, labels) in cases) {
          val label = "context-${inspection.status}-$width-$height-$scale"
          ComposeVisualFixture(width, height, scale) {
                ContextInspectorDialog(inspection, {}, {}, {})
              }
              .use { fixture ->
                fixture.render(label)
                for (text in
                    listOf(
                        "Project-relative path: src/contexts/request.go",
                        "Assistant target: Create function: Build",
                        "File-scoped preview, not the exact declaration request. Declaration requests may include different context and prompt material.",
                        "Local inspection only; no provider request or consent. Send and Explain require separate authorization.") +
                        labels) {
                  fixture.assertEveryTextLineReachable(text, "ide-dialog-body")
                  fixture.assertTextFits(text, maxLines = 12)
                }
                fixture.assertTextFits("Close")
                fixture.assertTextFits(
                    if (inspection.status == ContextInspectionStatus.Loading) "Cancel" else "Retry")
                if (inspection.status != ContextInspectionStatus.Ready)
                    assertFalse(fixture.hasText("No files included."))
                assertFalse(fixture.hasEditableText(withinTag = "ide-dialog-body"))
              }
        }
      }
    }
  }

  @Test
  fun f20ScanScopeWarningsRecoveryAndActionsStayReachableWithDiagnosticsCollapsed() {
    val viewports = listOf(1440 to 900, 1600 to 1000, 800 to 400, 800 to 650)
    for ((width, height) in viewports) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        for ((name, state) in verifiedScanLayoutCases()) {
          var calls = 0
          val label = "f20-$name-$width-$height-$scale-${density}x"
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    BugsWorkspacePane(
                        state,
                        BugsWorkspaceActions(
                            FindingActions({ calls++ }, { _, _ -> calls++ }, { calls++ }),
                            { calls++ },
                            { calls++ },
                            { calls++ },
                            { calls++ },
                            { calls++ }))
                  }
              .use { fixture ->
                fixture.render("$label-collapsed")
                fixture.revealTextFullyWithin("Verified Go scan", "result-overview")
                fixture.assertTextFits("Verified Go scan")
                val progress = verifiedScanProgress(state.project, state.scanState, state.scan)
                for (text in
                    listOf(verifiedScanScopeCopy, verifiedScanTrustCopy, progress.summary)) {
                  fixture.revealTextFullyWithin(text, "result-overview")
                  fixture.assertTextFits(text, maxLines = 12)
                }
                val action =
                    if (progress.action == VerifiedScanAction.Cancel) "Cancel checks"
                    else "Trust project-code execution & run checks"
                fixture.revealTextFullyWithin(action, "result-overview")
                fixture.assertTextFits(action)
                assertEquals(
                    progress.action == VerifiedScanAction.Waiting, fixture.isDisabled(action))
                fixture.render("$label-action")
                if (state.scanState.operation !in
                    listOf(
                        VerifiedScanOperation.Starting,
                        VerifiedScanOperation.CancellationRequested)) {
                  fixture.revealTextFullyWithin("Refresh scan status", "result-overview")
                  fixture.assertTextFits("Refresh scan status")
                } else assertFalse(fixture.hasText("Refresh scan status"))
                fixture.revealTextFullyWithin(verifiedScanEvidenceCopy, "result-overview")
                fixture.assertTextFits(verifiedScanEvidenceCopy, maxLines = 12)
                fixture.revealTextFullyWithin("Command and output", "result-overview")
                assertEquals("Collapsed", fixture.descriptionState("Expand Command and output"))
                assertEquals(0, fixture.tagCount("scan-diagnostics"))
                fixture.clickDescription("Expand Command and output")
                fixture.render("$label-expanded")
                if (state.scan == null) assertTrue(fixture.hasText("No scan report available."))
                else
                    state.scan.phases.forEach { phase ->
                      assertTrue(fixture.hasText(phase.name))
                      assertTrue(fixture.hasText(phase.output.take(4096).trimEnd()))
                    }
                assertFalse(fixture.hasEditableText(withinTag = "scan-diagnostics"))
                assertFalse(fixture.hasText("Formatting"))
                assertFalse(fixture.hasText("gofmt"))
                assertEquals(
                    0, calls, "Passive reflow and inspection must not dispatch work: $label")
              }
        }
      }
    }
  }

  @Test
  fun f17BugsProductionStatesAndActionsRemainReachableAtResponsiveTextAndDensity() {
    val base = resultPageFixture("bugs")
    val path = "internal/" + "deeply/nested/日本語/".repeat(5) + "handler.go"
    val message = "Long model explanation <script>inert</script> ".repeat(20).trim()
    val finding =
        base.semantic
            .first()
            .copy(
                id = "f17-long",
                title = "Reject incomplete results after a failed scan",
                message = message,
                evidence = "Evidence from saved analysis ".repeat(35).trim(),
                location = FindingLocation(path, startLine = 118, symbol = "Refresh"),
                source = "file_analysis",
                confidence = "suggested",
                freshness = "stale",
                status = "partial")
    val populated =
        base.copy(
            section = base.section.copy(results = base.results!!.copy(semantic = listOf(finding))))
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        val label = "f17-bugs-$width-$height-$scale-${density}x"
        var page by mutableStateOf(populated)
        val browser = newResultBrowserState(page)
        var operations = 0
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  BugsWorkspacePane(
                      BugsWorkspacePaneState(
                          page.semantic,
                          null,
                          false,
                          page,
                          browser,
                          ProjectIndex(
                              finding.projectId,
                              finding.projectRevision,
                              files = listOf(IndexedFile(path, "hash", "Go", false)))),
                      BugsWorkspaceActions(
                          FindingActions(
                              { operations++ }, { _, _ -> operations++ }, { operations++ }),
                          { operations++ },
                          { operations++ },
                          { operations++ },
                          { operations++ }))
                }
            .use { fixture ->
              fixture.render("$label-populated")
              fixture.assertTextFits("View analysis")
              fixture.clickDescription("Inspect ${finding.title}")
              fixture.render("$label-stale-partial")
              assertTrue(fixture.hasText("Partial · Stale"))
              assertTrue(fixture.hasText("Model suggestion"))
              assertTrue(fixture.hasText("$path:118 · Refresh"))
              assertTrue(fixture.hasText(message))
              assertTrue(fixture.isDisabled("Prepare fix"))
              fixture.revealText("Prepare fix", "result-detail")
              fixture.assertTextFits("Prepare fix")
              fixture.revealText("Open source", "result-detail")
              assertTrue(fixture.requestFocus("Open source"))
              fixture.render("$label-source-focused")
              assertTrue(fixture.isFocusedControl("Open source"))
              fixture.assertColorVisible(FocusAccent)
              fixture.revealText("Evidence and fix criteria", "result-detail")
              assertFalse(fixture.hasText(finding.evidence), "Optional evidence starts collapsed")
              if (width == 800 && scale == 1.5f && density == 1f) {
                fixture.clickText("Evidence and fix criteria")
                fixture.render("$label-evidence-expanded")
                assertTrue(fixture.hasText(finding.evidence))
              }
              val list = fixture.taggedBounds("result-list")
              val detail = fixture.taggedBounds("result-detail")
              assertTrue(list.height > 0 && detail.height > 0, label)
              assertTrue(list.right <= detail.left || list.bottom <= detail.top, label)
              assertTrue(detail.bottom <= height * density, label)
              browser.query = "no matching result"
              fixture.render("$label-no-match")
              assertTrue(fixture.hasText("No matching results."))
              assertTrue(fixture.hasDescription("Clear filters"))
              browser.query = ""
              page = populated.copy(section = populated.section.copy(error = "saved read failed"))
              fixture.render("$label-retained-error")
              assertTrue(fixture.hasText("Results could not be refreshed: saved read failed"))
              assertTrue(fixture.hasDescription("Inspect ${finding.title}"))
              assertEquals(0, operations, "Passive inspection and reflow cannot dispatch work")
            }
      }
    }
    fun emptyPage(status: String, count: Int?): AnalysisResultPageState {
      val progress = requireNotNull(base.progress).copy(status = status, findingCount = count)
      val run =
          requireNotNull(base.run)
              .copy(
                  status = status,
                  sections = base.run.sections.map { if (it.category == "bugs") progress else it })
      return base.copy(
          run = run,
          section =
              base.section.copy(
                  results =
                      requireNotNull(base.results)
                          .copy(progress = progress, semantic = emptyList())))
    }
    val states =
        listOf(
            Triple(
                "completed-empty", emptyPage("completed", 0), "No findings in the analyzed scope."),
            Triple("canceled", emptyPage("canceled", null), "Analysis was canceled."),
            Triple(
                "unavailable",
                emptyPage("unavailable", null),
                "Analysis is unavailable for this category."))
    for ((name, page, message) in states) {
      ComposeVisualFixture(800, 650, 1.5f) {
            BugsWorkspacePane(
                BugsWorkspacePaneState(page.semantic, null, false, page),
                BugsWorkspaceActions(FindingActions({}, { _, _ -> }, {}), {}, {}))
          }
          .use { fixture ->
            fixture.render("f17-bugs-$name-800-650-1.5-1x")
            assertTrue(fixture.taggedBounds("result-empty").height > 0)
            fixture.assertTextFits("View analysis")
            fixture.assertTextFits(message)
            if (name != "completed-empty") assertFalse(fixture.hasText("0 findings"))
          }
    }
  }

  @Test
  fun f16AnalysisRunShowsLifecycleReasonAttemptsAndRecoveryWithDetailsCollapsed() {
    val reason =
        "Stopped at src/" + "日本語-long-path/".repeat(30) + "main.go\n" + "detail ".repeat(700)
    val base =
        analysisRunFixture()
            .copy(
                plan =
                    analysisPreviewFixture().let { preview ->
                      preview.copy(
                          files =
                              preview.files.map { file ->
                                file.copy(
                                    stages =
                                        listOf(
                                            AnalysisStagePlan(
                                                "semantic", true, false, maxModelRequests = 0)))
                              })
                    },
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "completed", 2, false)))),
                elapsedSeconds = 42)
    val cases =
        listOf(
            "requesting-pause" to
                ProjectAnalysisRunState(
                    run = base.copy(status = "running"),
                    action = "pause",
                    controlRequest =
                        AnalysisControlRequest("pause", AnalysisControlOutcome.Requesting)),
            "pausing" to ProjectAnalysisRunState(run = base.copy(status = "pausing")),
            "requesting-cancel" to
                ProjectAnalysisRunState(
                    run = base.copy(status = "pausing"),
                    action = "cancel",
                    controlRequest =
                        AnalysisControlRequest("cancel", AnalysisControlOutcome.Requesting)),
            "canceling" to ProjectAnalysisRunState(run = base.copy(status = "canceling")),
            "paused" to
                ProjectAnalysisRunState(run = base.copy(status = "paused", reason = reason)),
            "interrupted" to ProjectAnalysisRunState(run = base.copy(status = "interrupted")),
            "canceled" to ProjectAnalysisRunState(run = base.copy(status = "canceled")),
            "failed" to ProjectAnalysisRunState(run = base.copy(status = "failed")),
            "partial" to ProjectAnalysisRunState(run = base.copy(status = "partial")),
            "unavailable" to ProjectAnalysisRunState(run = base.copy(status = "unavailable")),
            "unknown" to ProjectAnalysisRunState(run = base.copy(status = "future_state")),
            "status-unavailable" to
                ProjectAnalysisRunState(
                    run = base.copy(status = "paused"),
                    statusUnavailable = true,
                    error = "Status read failed"))
    for ((name, analysis) in cases) {
      var calls = 0
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(resultProjectFixture(), analysis),
                AnalysisWorkspaceActions(
                    { _, _ -> calls++ },
                    { calls++ },
                    { calls++ },
                    { calls++ },
                    { calls++ },
                    refreshStatus = { error("Unexpected status refresh") }))
          }
          .use { fixture ->
            fixture.render("f16-analysis-$name")
            val metadata = analysisRunTimeMetadata(requireNotNull(analysis.run))
            val facts = projectRunPresentation(resultProjectFixture(), analysis)
            val metadataLine =
                if (facts.isActive || facts.totalSteps == 0) metadata.joinToString(" · ")
                else
                    "${facts.finishedSteps} of ${facts.totalSteps} stages · ${metadata.joinToString(" · ")}"
            fixture.revealText(metadataLine, "analysis-page")
            assertTrue(fixture.hasText(metadataLine))
            fixture.revealText("Cumulative attempts reported · 2", "analysis-page")
            assertTrue(fixture.hasText("Cumulative attempts reported · 2"))
            when (name) {
              "requesting-pause" -> {
                assertTrue(fixture.hasText("Requesting pause…"))
                assertFalse(
                    fixture.hasText(
                        "Pause requested; waiting for the current stage boundary. No new stage will start."))
              }
              "pausing" ->
                  assertTrue(
                      fixture.hasText(
                          "Pause requested; waiting for the current stage boundary. No new stage will start."))
              "requesting-cancel" -> assertTrue(fixture.hasText("Requesting cancellation…"))
              "canceling" ->
                  assertTrue(
                      fixture.hasText(
                          "Cancellation accepted; active work is stopping. Completed evidence remains available."))
              "paused",
              "interrupted" -> {
                fixture.revealText("Resume → fresh preview", "analysis-page")
                fixture.assertTextFits("Resume → fresh preview")
              }
              "canceled" -> {
                fixture.revealText("Start new analysis", "analysis-page")
                fixture.assertTextFits("Start new analysis")
                assertFalse(fixture.hasText("Resume → fresh preview"))
              }
              "unknown" -> assertFalse(fixture.hasText("Resume → fresh preview"))
              "status-unavailable" ->
                  assertTrue(
                      fixture.hasText(
                          "Current run status unavailable; the last accepted snapshot is retained."))
            }
            if (name in
                setOf("paused", "interrupted", "canceled", "failed", "partial", "unavailable")) {
              val reasonLabel =
                  if (name == "paused")
                      "Stop reason · ${sanitizedOutputText(reason, 180).substringBefore('\n')}"
                  else "Stop reason · No stop reason was supplied for this run."
              fixture.revealText(reasonLabel, "analysis-page")
              assertTrue(fixture.hasText(reasonLabel))
              if (name == "paused") {
                assertFalse(fixture.hasText(reason))
                fixture.revealText("Run diagnostic", "analysis-page")
                fixture.clickText("Run diagnostic")
                fixture.render("f16-analysis-$name-expanded")
                fixture.revealText("Show full available output", "analysis-page")
                fixture.clickDescription("Expand available diagnostic output")
                fixture.render()
                assertTrue(fixture.hasText(reason.trim()))
                assertTrue(fixture.taggedBounds("diagnostic-output-scroll").height <= 240f * 1.5f)
              }
            }
            assertEquals(0, calls)
          }
    }
    val project = resultProjectFixture()
    val viewports = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in viewports) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        for (name in listOf("requesting-pause", "paused", "canceling", "status-unavailable")) {
          val original = cases.first { it.first == name }.second
          val analysis =
              if (name == "requesting-pause")
                  original.copy(run = original.run?.copy(reason = "Stage save needs attention"))
              else original
          val label = "f16-matrix-$name-$width-$height-$scale-${density}x"
          val pixelsWide = (width * density).toInt()
          val pixelsHigh = (height * density).toInt()
          ComposeVisualFixture(pixelsWide, pixelsHigh, scale, density) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(project, analysis),
                    AnalysisWorkspaceActions(
                        { _, _ -> error("Passive render dispatched start") },
                        { error("Passive render dispatched pause") },
                        { error("Passive render dispatched resume") },
                        { error("Passive render dispatched cancel") },
                        { error("Passive render dispatched file refresh") },
                        refreshStatus = { error("Passive render dispatched status refresh") }))
              }
              .use { fixture ->
                fixture.render("$label-analysis")
                fixture.revealText("Refresh status", "analysis-page")
                fixture.assertTextFits("Refresh status")
                if (name == "paused") {
                  fixture.revealText("Resume → fresh preview", "analysis-page")
                  fixture.assertTextFits("Resume → fresh preview")
                }
                if (name == "status-unavailable") {
                  assertFalse(fixture.hasText("Resume → fresh preview"))
                }
              }
          val state =
              DesktopState(projectState = ProjectWorkspaceState(project), analysisRun = analysis)
          val header = requireNotNull(toolbarAnalysisStatus(state))
          ComposeVisualFixture(pixelsWide, pixelsHigh, scale, density) {
                ToolbarVisualFixture(
                    width.toFloat(),
                    connection = ConnectionState(label = "Disconnected"),
                    analysisStatus = header)
              }
              .use { fixture ->
                fixture.render("$label-header")
                fixture.assertTextFits(header.label)
                fixture.assertTextBefore(header.label, "Daemon disconnected")
              }
          ComposeVisualFixture(pixelsWide, pixelsHigh, scale, density) {
                ProjectSummaryPane(null, project, {}, run = analysis.run, analysisState = analysis)
              }
              .use { fixture ->
                fixture.render("$label-summary")
                val status =
                    analysisLifecycleStatusLabel(
                        projectRunPresentation(project, analysis), analysis)
                fixture.assertTextFits(status, maxLines = 3)
                if (name == "paused") {
                  assertTrue(
                      fixture.hasText(
                          "Stop reason · ${sanitizedOutputText(reason, 180).substringBefore('\n')}"))
                  assertFalse(fixture.hasText(reason.trim()))
                }
                if (name == "requesting-pause") {
                  assertTrue(fixture.hasText("Run diagnostic · Stage save needs attention"))
                  assertFalse(fixture.hasText("Stop reason · Stage save needs attention"))
                }
              }
        }
      }
    }
  }

  @Test
  fun projectLandingRendersRememberedOpeningAndRecoveryAcrossViewports() {
    val path = "/projects/" + "日本語-very-long-directory/".repeat(6) + "workspace"
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        var requests = 0
        var app by mutableStateOf(DesktopState())
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  ProjectLanding(
                      app,
                      DesktopShellProjectActions(
                          { requests++ }, {}, { requests++ }, { requests++ }),
                      FocusRequester())
                }
            .use { fixture ->
              val label = "f06-landing-$width-$height-$scale-${density}x"
              fixture.render("$label-empty")
              fixture.assertTextFits("Open project…")
              assertTrue(fixture.hasText("No project remembered on this device."))
              assertFalse(fixture.hasText("Retry restore"))
              app = app.copy(projectState = app.projectState.copy(rememberedPath = path))
              fixture.render("$label-remembered")
              fixture.assertTextFits("Open project…")
              fixture.revealText("Remembered path", "project-landing-scroll")
              assertTrue(fixture.hasText(path))
              app =
                  app.copy(
                      projectState =
                          app.projectState.copy(
                              openingAttempt =
                                  ProjectOpeningAttempt(1, path, ProjectOpeningKind.Restore)))
              fixture.render("$label-restoring")
              assertTrue(fixture.hasText("Restoring local project…"))
              assertFalse(fixture.hasText("Retry restore"))
              app =
                  app.copy(
                      projectState =
                          app.projectState.copy(
                              openingAttempt =
                                  ProjectOpeningAttempt(
                                      1,
                                      path,
                                      ProjectOpeningKind.Restore,
                                      ProjectOpeningOutcome.Failed("Cannot read saved metadata")),
                              preferenceReadWarning = "Preferences unavailable"),
                      connection = ConnectionState(label = "Disconnected"))
              fixture.render("$label-failed")
              for (text in
                  listOf(
                      "Retry restore",
                      "Could not read last project preference",
                      "Reconnect daemon")) {
                fixture.revealText(text, "project-landing-scroll")
                fixture.assertTextFits(text)
              }
              fixture.revealText("Cannot read saved metadata", "project-landing-scroll")
              assertTrue(fixture.hasText("Cannot read saved metadata"))
              app =
                  app.copy(
                      projectState =
                          app.projectState.copy(
                              openingAttempt = null, preferenceReadWarning = null))
              fixture.render("$label-disconnected")
              fixture.revealText("Reconnect daemon", "project-landing-scroll")
              fixture.assertTextFits("Reconnect daemon")
              app =
                  app.copy(
                      connection = ConnectionState(),
                      projectState =
                          app.projectState.copy(preferenceReadWarning = "Preferences unavailable"))
              fixture.render("$label-preference-warning")
              fixture.revealText("Could not read last project preference", "project-landing-scroll")
              fixture.assertTextFits("Could not read last project preference")
              assertEquals(0, requests)
            }
      }
    }
  }

  @Test
  fun loadedHeaderRendersRequestedTargetWithoutReplacingCurrentIdentity() {
    val project = resultProjectFixture()
    val path = "/projects/" + "日本語-long-directory/".repeat(5) + "requested"
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        var actions = 0
        var attempt by
            mutableStateOf<ProjectOpeningAttempt?>(
                ProjectOpeningAttempt(1, path, ProjectOpeningKind.Restore))
        var saveWarning by mutableStateOf<String?>(null)
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  MainToolbar(
                      ToolbarState(
                          project,
                          true,
                          "Analyzing",
                          ConnectionState(label = "Disconnected"),
                          null,
                          ToolbarAnalysisStatus(
                              "Analysis · Running",
                              "Whole-project analysis · Running",
                              true,
                              false),
                          openingAttempt = attempt,
                          preferenceSaveWarning = saveWarning),
                      ToolbarActions({ actions++ }, {}, { actions++ }, {}, { actions++ }))
                }
            .use { fixture ->
              val label = "f06-header-$width-$height-$scale-${density}x"
              fixture.render("$label-restoring")
              assertTrue(fixture.hasText(project.name))
              assertTrue(fixture.hasText("Current project"))
              assertTrue(fixture.hasText("Requested path"))
              assertTrue(fixture.hasText(path))
              assertTrue(fixture.hasText("Restoring local project…"))
              assertTrue(fixture.hasText("Analysis · Running"))
              assertTrue(fixture.hasText("Daemon disconnected"))
              assertFalse(fixture.hasText("Retry restore"))
              attempt =
                  attempt?.copy(
                      outcome = ProjectOpeningOutcome.Failed("Could not read saved metadata"))
              fixture.render("$label-failed")
              fixture.revealText("Retry restore", "project-opening-scroll")
              fixture.assertTextFits("Retry restore")
              fixture.revealText("Could not read saved metadata", "project-opening-scroll")
              assertTrue(fixture.hasText(project.name))
              assertTrue(fixture.hasText(path))
              attempt = null
              saveWarning = "Disk denied: project not saved for next launch"
              fixture.render("$label-preference-save-failed")
              fixture.revealText(
                  "Disk denied: project not saved for next launch", "project-preference-scroll")
              fixture.assertTextFits("Could not remember project")
              assertTrue(fixture.hasText(project.name))
              assertFalse(fixture.hasText("Could not restore project"))
              assertEquals(0, actions)
            }
      }
    }
  }

  @Test
  fun productionIndexingFeedbackKeepsLongEvidenceAndRecoveryReachable() {
    val path = "/projects/" + "日本語-very-long-directory/".repeat(8) + "workspace"
    val diagnostic = "Cannot read inventory: " + "日本語-long-diagnostic/".repeat(18)
    val project =
        resultProjectFixture()
            .copy(path = path, name = "A long project identity with 日本語 and many segments")
    val attempt = ProjectIndexingAttempt(4, project.projectId, project.projectRevision, path)
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        var calls = 0
        var state by
            mutableStateOf(
                ToolbarState(
                    project,
                    false,
                    "",
                    ConnectionState(label = "Disconnected"),
                    null,
                    ToolbarAnalysisStatus(
                        "Analysis · Running", "Whole-project analysis · Running", true, false),
                    indexingAttempt = attempt))
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  Column(Modifier.fillMaxSize().background(AppBackground)) {
                    MainToolbar(
                        state, ToolbarActions({ calls++ }, { calls++ }, { calls++ }, { calls++ }))
                    Box(Modifier.weight(1f)) { Text("Project workspace") }
                  }
                }
            .use { fixture ->
              val label = "f07-index-$width-$height-$scale-${density}x"
              fixture.render("$label-running")
              fixture.assertTextFits("Re-indexing project…")
              assertTrue(fixture.hasDescription("Re-index in progress"))
              assertTrue(fixture.hasText("Analysis · Running"))
              assertTrue(fixture.hasText("Daemon disconnected"))
              state =
                  state.copy(
                      indexingAttempt =
                          attempt.copy(outcome = ProjectIndexingOutcome.Failed(diagnostic)))
              fixture.render("$label-failed")
              fixture.assertTextFits("Retry re-index")
              fixture.revealText("Retry re-index", "project-indexing-scroll")
              fixture.assertTextFits("Retry re-index")
              assertTrue(fixture.requestFocus("Retry re-index"))
              fixture.render("$label-retry-focused")
              assertTrue(fixture.isFocusedControl("Retry re-index"))
              fixture.revealText("Project path", "project-indexing-scroll")
              assertTrue(fixture.hasText(project.name))
              assertTrue(fixture.hasText(path))
              fixture.revealText("Re-index diagnostic", "project-indexing-scroll")
              assertTrue(fixture.hasText(diagnostic))
              assertTrue(
                  fixture.taggedBounds("project-indexing-scroll").height <= 220f * density + 2f)
              state = state.copy(switchPending = true)
              fixture.render("$label-disabled")
              fixture.revealText("Retry re-index", "project-indexing-scroll")
              assertTrue(fixture.isDisabled("Retry re-index"))
              state =
                  state.copy(
                      switchPending = false,
                      indexingAttempt = attempt.copy(outcome = ProjectIndexingOutcome.Canceled))
              fixture.render("$label-canceled")
              fixture.revealText("Retry re-index", "project-indexing-scroll")
              fixture.assertTextFits("Retry re-index")
              state =
                  state.copy(
                      indexingAttempt =
                          attempt.copy(
                              outcome = ProjectIndexingOutcome.Succeeded("inventory-next")),
                      detailsOutcome =
                          ProjectDetailsOutcome.Unavailable("Saved details unavailable"))
              fixture.render("$label-details-unavailable")
              fixture.revealText("Saved details unavailable", "project-indexing-scroll")
              assertTrue(fixture.hasText("inventory-next"))
              assertTrue(
                  fixture.hasText(
                      "Workspace details unavailable; retained findings keep their existing freshness labels."))
              assertFalse(fixture.hasText("Retry re-index"))
              assertEquals(0, calls)
            }
      }
    }
  }

  @Test
  fun projectSwitchReviewKeepsConsentAndCleanupErrorsReachableAcrossViewports() {
    val path = "/projects/" + "日本語-very-long-directory/".repeat(8) + "next"
    val model =
        ScopedModel(
            scope = "analyze",
            profile = "remote",
            model = "example-model",
            providerOrigin = "https://provider.example/" + "long-destination/".repeat(6),
            remoteProvider = true)
    val context =
        ProjectSwitchContext(
            SwitchProjectIdentity(resultProjectFixture()),
            SwitchDraftIdentity(null, DeclarationDraft(id = "draft"), null),
            SwitchAnalyzeDestination(model),
            false)
    val terminal =
        TerminalWorkspaceState(
            tabs = listOf(TerminalTabState(1, "Hidden shell"), TerminalTabState(2, "Exited shell")))
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        var pending by
            mutableStateOf(PendingProjectSwitch(1, path, context, SwitchReviewStage.Draft))
        var feedback by mutableStateOf(SwitchCleanupFeedback())
        var calls = 0
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  Box(
                      Modifier.fillMaxSize().background(AppBackground),
                      contentAlignment = Alignment.Center) {
                        IdeDialogSurface(
                            maxHeight = (height - 64).coerceAtMost(520).dp,
                            title = { Text("Switch project?") },
                            content = {
                              ProjectSwitchReviewBody(pending, model, false, terminal, feedback) {
                                calls++
                              }
                            },
                            actions = {
                              ProjectSwitchReviewActions(
                                  pending,
                                  false,
                                  terminal,
                                  feedback,
                                  { calls++ },
                                  { calls++ },
                                  { calls++ },
                                  { calls++ },
                                  { calls++ })
                            })
                      }
                }
            .use { fixture ->
              val label = "f07-switch-$width-$height-$scale-${density}x"
              fixture.render("$label-draft")
              fixture.assertTextFits("Cancel switch")
              fixture.assertTextFits("Approve draft discard for switch")
              assertTrue(
                  fixture.hasText(
                      "If you switch, the in-memory conversation, editable draft and focused checks will be discarded. Continuing this review does not discard them yet."))
              fixture.revealText("Requested project: $path", "ide-dialog-body")
              assertTrue(fixture.hasText("Hidden shell"))
              pending = pending.copy(stage = SwitchReviewStage.Provider)
              fixture.render("$label-provider")
              fixture.assertTextFits("Cancel switch")
              assertTrue(fixture.isDisabled("Continue with provider"))
              pending = pending.copy(stage = SwitchReviewStage.Final)
              fixture.render("$label-final")
              fixture.assertTextFits("Close shells and switch")
              assertTrue(fixture.requestFocus("Cancel switch"))
              fixture.render("$label-safe-focus")
              assertTrue(fixture.isFocusedControl("Cancel switch"))
              pending = pending.copy(stage = SwitchReviewStage.Committed)
              feedback = SwitchCleanupFeedback(outstanding = true)
              fixture.render("$label-cleanup-pending")
              assertFalse(fixture.hasText("Cancel switch"))
              assertFalse(fixture.hasText("Close shells and switch"))
              feedback =
                  SwitchCleanupFeedback(error = "Cleanup failed: " + "diagnostic/".repeat(20))
              fixture.render("$label-cleanup-error")
              fixture.assertTextFits("Close review")
              fixture.revealText(feedback.error!!, "ide-dialog-body")
              assertEquals(0, calls)
            }
      }
    }
  }

  @Test
  fun projectMenuKeepsOpenSwitchAndReindexAvailabilityAcrossViewports() {
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        var project by mutableStateOf<ProjectAnalysis?>(null)
        var calls = 0
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  ToolbarVisualFixture(
                      width.toFloat(),
                      project = project,
                      actions = ToolbarActions({ calls++ }, { calls++ }, { calls++ }, { calls++ }))
                }
            .use { fixture ->
              val label = "f07-menu-$width-$height-$scale-${density}x"
              fixture.render()
              fixture.clickText("No project open")
              fixture.render("$label-no-project")
              fixture.assertTextFits("Open project…")
              assertTrue(fixture.isDisabled("Re-index project"))
              fixture.dismissPopup()
              project = visualFixtureProject
              fixture.render()
              fixture.clickText("go-shop · fixture")
              fixture.render("$label-loaded")
              fixture.assertTextFits("Switch project…")
              fixture.assertTextFits("Re-index project")
              assertEquals(0, calls)
            }
      }
    }
  }

  @Test
  fun assembledShellKeepsRailHeaderFooterAndTerminalSeparateAcrossViewportAndDensity() {
    val project =
        resultProjectFixture().copy(name = "A long project identity with 日本語 and many segments")
    val model = ScopedModel(model = "local-model", providerOrigin = "http://localhost:11434")
    val unavailable =
        desktopStatusBarPresentation(
            DesktopState(projectState = ProjectWorkspaceState(project)),
            DesktopShellStatusProviders(model, model, ScopedModel()))
    val run = analysisRunFixture()
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    // Density is independent of text scaling: these two captures have the same logical viewport.
    val captures = sizes.flatMap { (w, h) -> listOf(1f, 1.25f, 1.5f).map { Triple(w, h, it) } }
    for ((logicalWidth, logicalHeight, scale) in captures) {
      for (density in if (logicalWidth == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        val width = (logicalWidth * density).toInt()
        val height = (logicalHeight * density).toInt()
        for (expanded in listOf(false, true)) {
          val status = if (expanded) "failed" else "running"
          val app =
              DesktopState(
                  projectState = ProjectWorkspaceState(project),
                  analysisRun = ProjectAnalysisRunState(run = run.copy(status = status)))
          val label =
              "f03-shell-$logicalWidth-$logicalHeight-$scale-${density}x-$status-${if (expanded) "expanded" else "collapsed"}"
          val railFocus = FocusRequester()
          ComposeVisualFixture(width, height, scale, densityScale = density) {
                Column(Modifier.fillMaxSize().background(AppBackground)) {
                  MainToolbar(
                      ToolbarState(
                          project,
                          false,
                          "",
                          ConnectionState(label = "Disconnected"),
                          GitStatus(available = true, branch = "feature/long-identity-with-日本語"),
                          toolbarAnalysisStatus(app)),
                      ToolbarActions({}, {}, {}, {}))
                  WorkspaceFrame(
                      rail = {
                        ToolWindowBar(
                            LeftToolWindow.Analysis,
                            {},
                            Modifier.focusRequester(railFocus),
                            onOpenTerminal = {})
                      },
                      panes = { EditorArea({ Text("Project workspace") }, Modifier.weight(1f)) },
                      terminal = {
                        TerminalDock(
                            DesktopLayoutState(bottomCollapsed = !expanded),
                            TerminalWorkspaceState(),
                            {},
                            {},
                            TerminalTabActions({}, {}, {}),
                            {},
                            {},
                            { modifier -> Box(modifier.background(EditorCanvas)) },
                            modifier = Modifier.testTag("f03-terminal"))
                      },
                      modifier = Modifier.weight(1f))
                  PersistentStatusBar(unavailable, {})
                }
              }
              .use { fixture ->
                fixture.render(label)
                railFocus.requestFocus()
                fixture.render("$label-selected-focused")
                fixture.assertRailLabelFits("Performance")
                assertTrue(fixture.hasDescription("Analysis tool window, selected, focused"))
                assertTrue(fixture.hasText("Models: unavailable"))
                assertTrue(
                    fixture.hasText("Analysis · ${status.replaceFirstChar { it.uppercase() }}"))
                assertTrue(fixture.hasText("Daemon disconnected"))
                val header = fixture.firstVisibleTextBounds("Daemon disconnected")
                val footer = fixture.taggedBounds("model-count-footer")
                val terminal = fixture.taggedBounds("f03-terminal")
                val editor = fixture.descriptionBounds("Editor area")
                assertTrue(header.bottom <= editor.top, "$label: header overlaps workspace")
                assertTrue(editor.bottom <= terminal.top, "$label: terminal overlaps workspace")
                assertTrue(terminal.bottom <= footer.top, "$label: terminal overlaps footer")
                assertEquals(editor.left, terminal.left, density, "$label: terminal left alignment")
                assertEquals(
                    editor.right, terminal.right, density, "$label: terminal right alignment")
                assertEquals(width.toFloat(), footer.right, density)
                assertEquals(height.toFloat(), footer.bottom, density)
                fixture.assertTextFits("Models: unavailable")
              }
        }
      }
    }
  }

  @Test
  fun adaptiveProductionEditorAndContentMatrixKeepsRegionsBounded() {
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    val extreme = DesktopLayoutState(explorerWidth = 520f, actionWidth = 560f, bottomHeight = 520f)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        for (review in listOf(false, true)) {
          val label =
              "f04-editor-${if (review) "review" else "source"}-$width-$height-$scale-${density}x"
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    AdaptiveProductionEditorFixture(extreme, review)
                  }
              .use { fixture ->
                fixture.render(label)
                val files = fixture.taggedBounds("f04-files")
                val canvas = fixture.taggedBounds("f04-canvas")
                val tool = fixture.taggedBounds("f04-tool")
                val dock = fixture.taggedBounds("f04-dock")
                val footer = fixture.taggedBounds("model-count-footer")
                assertTrue(dock.bottom <= footer.top, label)
                assertTrue(footer.bottom <= height * density, label)
                val mode = resolveDesktopLayout(extreme, width.toFloat(), scale).mode
                if (mode == DesktopLayoutMode.Wide) {
                  assertTrue(files.width > 0 && canvas.width > 0 && tool.width > 0, label)
                  assertTrue(files.right <= canvas.left && canvas.right <= tool.left, label)
                  assertTrue(canvas.bottom <= dock.top, label)
                } else {
                  assertTrue(files.width > 0, label)
                  fixture.scrollBy(380f * density, "f04-arrangement")
                  fixture.render()
                  assertTrue(fixture.taggedBounds("f04-canvas").height > 0, label)
                  if (review && width == 800 && height == 650 && scale == 1.5f) {
                    val viewport = fixture.taggedBounds("diff-Current-vertical")
                    assertTrue(viewport.height >= 100f * density, "$label: $viewport")
                  }
                  fixture.scrollBy(100_000f, "f04-arrangement")
                  fixture.render("$label-tool-revealed")
                  val revealed = fixture.taggedBounds("f04-tool")
                  assertTrue(revealed.width > 0 && revealed.height > 0, "$label: $revealed")
                  assertTrue(revealed.top < dock.top, label)
                }
                if (review && width == 1440 && height == 900 && scale == 1.5f) {
                  val viewport = fixture.taggedBounds("diff-Current-vertical")
                  assertTrue(
                      viewport.height >= 120f * density,
                      "$label: usable diff viewport required, got $viewport")
                  assertTrue(viewport.bottom <= dock.top, label)
                }
                assertTrue(fixture.hasText(if (review) "Candidate diff" else "Read-only"), label)
                assertFalse(
                    fixture.hasEditableText(
                        withinTag = if (review) "diff-Current-column" else "source-viewport"))
                assertTrue(fixture.hasDescription("Files tool window"))
                assertTrue(fixture.hasDescription("Tool windows tool window"))
              }
        }
        // These are the real page components, not a placeholder pane inside the shell.
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  RoundedSummaryVisualFixture(width.toFloat())
                }
            .use { fixture ->
              fixture.render("f04-summary-$width-$height-$scale-${density}x")
              assertTrue(fixture.hasText("Analysis coverage"))
            }
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
                }
            .use { fixture ->
              fixture.render("f04-summary-content-$width-$height-$scale-${density}x")
              fixture.scrollBy(520f * density)
              fixture.render("f04-summary-categories-$width-$height-$scale-${density}x")
              assertTrue(fixture.hasDescription("View Bugs results"))
              assertTrue(fixture.hasDescription("View Security results"))
              if (width == 800 && scale == 1.5f && density == 1f) {
                fixture.revealText("Expand diagram")
                fixture.clickText("Expand diagram")
                fixture.render("f04-summary-help-expanded-800-650-1.5-1x")
                assertTrue(fixture.hasText("Close"))
              }
            }
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  AcceptanceResultPane("bugs", "partial")
                }
            .use { fixture ->
              fixture.render("f04-results-$width-$height-$scale-${density}x")
              val list = fixture.taggedBounds("result-list")
              val detail = fixture.taggedBounds("result-detail")
              assertTrue(list.height > 0 && detail.height > 0)
              assertTrue(list.bottom <= height * density && detail.bottom <= height * density)
              assertTrue(list.right <= detail.left || list.bottom <= detail.top)
              assertTrue(fixture.hasText("View analysis"))
            }
      }
    }
  }

  @Test
  fun f23SourceCompositionKeepsIdentityRecoveryAndCreationReachable() {
    val evidence = sourceNavigationReviewFixture()
    val path = requireNotNull(evidence.selected).path
    val failedPath = "internal/" + "replacement/日本語/".repeat(8) + "user.go"
    val preferred = DesktopLayoutState(explorerWidth = 520f, actionWidth = 560f)
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        for (failed in listOf(false, true)) {
          var actions = 0
          val read =
              if (failed) FileReadUiState.Failed(failedPath, "Local read unavailable.") else null
          val label =
              "f23-source-$width-$height-$scale-${density}x-${if (failed) "failed" else "loaded"}"
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    AdaptiveProductionEditorFixture(
                        preferred,
                        false,
                        evidence = evidence,
                        fileRead = read,
                        terminalCollapsed = true,
                        onOpenFile = { actions++ },
                        onCreate = { actions++ },
                        onRequest = { actions++ },
                        onSourceLine = { actions++ })
                  }
              .use { fixture ->
                fixture.render(label)
                assertTrue(fixture.hasText(path), label)
                assertTrue(fixture.hasText("Read-only"), label)
                assertFalse(fixture.hasEditableText(withinTag = "source-viewport"))
                if (resolveDesktopLayout(preferred, width.toFloat(), scale).mode ==
                    DesktopLayoutMode.Compact) {
                  fixture.revealTextFullyWithin("New function", "f04-arrangement")
                }
                fixture.assertTextFits("New function")
                assertTrue(fixture.requestFocus("New function"), label)
                fixture.awaitDescriptionFocus("New function in $path")
                fixture.assertDescriptionFullyVisible("New function in $path", "f04-canvas")
                fixture.render("$label-creation-focused")
                if (failed) {
                  assertTrue(
                      fixture.requestDescriptionFocus("Retry opening $failedPath", "f04-canvas"),
                      label)
                  fixture.awaitDescriptionFocus("Retry opening $failedPath")
                  fixture.assertDescriptionFullyVisible("Retry opening $failedPath", "f04-canvas")
                  fixture.render("$label-recovery-focused")
                  assertTrue(fixture.hasText("The current file remains open."), label)
                  assertTrue(fixture.hasDescription("Failed destination: $failedPath"), label)
                }
                if (resolveDesktopLayout(preferred, width.toFloat(), scale).mode ==
                    DesktopLayoutMode.Compact) {
                  fixture.revealTagFullyWithin("source-viewport", "f04-arrangement")
                }
                fixture.render("$label-source-revealed")
                val source = fixture.taggedBounds("source-viewport")
                assertTrue(
                    source.height >= 100f * density, "$label: usable source required, got $source")
                if (resolveDesktopLayout(preferred, width.toFloat(), scale).mode ==
                    DesktopLayoutMode.Compact) {
                  fixture.scrollBy(100_000f, "f04-arrangement")
                  fixture.render("$label-context-revealed")
                }
                assertTrue(fixture.hasText("Context"), label)
                assertEquals(0, actions, label)
                assertEquals(520f, preferred.explorerWidth)
                assertEquals(560f, preferred.actionWidth)
              }
        }
      }
    }
  }

  @Test
  fun f23PendingHiddenFilesAndBlockedCreationRemainReachableAcrossLayoutMatrix() {
    val evidence = sourceNavigationReviewFixture()
    val file = requireNotNull(evidence.selected)
    val destination = "internal/" + "replacement/日本語/".repeat(8) + "user.go"
    val preferred = DesktopLayoutState(explorerWidth = 520f, actionWidth = 560f)
    val inventory =
        (1..80).map { IndexedFile("internal/records/record-$it.go", "hash-$it", "Go", false) }
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        var layout by mutableStateOf(preferred.copy(leftToolWindowVisible = false))
        var read by mutableStateOf<FileReadUiState?>(FileReadUiState.Pending(destination))
        var actions = 0
        val label = "f23-states-$width-$height-$scale-${density}x"
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  AdaptiveProductionEditorFixture(
                      layout,
                      false,
                      evidence = evidence,
                      fileRead = read,
                      creationInProgress = true,
                      extraIndexedFiles = inventory,
                      onLeftTool = { layout = layout.openLeft(it) },
                      terminalCollapsed = true,
                      onOpenFile = { actions++ },
                      onCreate = { actions++ },
                      onRequest = { actions++ },
                      onSourceLine = { actions++ })
                }
            .use { fixture ->
              fixture.render("$label-pending-hidden")
              assertTrue(fixture.hasDescription("Pending destination: $destination"), label)
              assertTrue(fixture.hasDescription("Project-relative path: ${file.path}"), label)
              assertTrue(fixture.isDisabled("New function"), label)
              assertTrue(fixture.hasDescription("Editor tool window, selected"), label)
              assertEquals(0, fixture.tagCount("f04-files"))
              fixture.clickDescription("Editor tool window, selected")
              fixture.render("$label-pending-files")
              assertTrue(layout.leftToolWindowVisible)
              assertTrue(fixture.descriptionBounds("Indexed file tree").height > 0f, label)
              if (resolveDesktopLayout(layout, width.toFloat(), scale).mode ==
                  DesktopLayoutMode.Compact) {
                fixture.revealTagFullyWithin("explorer-header-scroll", "f04-arrangement")
              }
              assertTrue(fixture.hasDescription("Pending destination: $destination"), label)
              read = FileReadUiState.Failed(destination, "Local read unavailable.")
              fixture.render("$label-failed-files")
              fixture.scrollBy(100_000f, "explorer-header-scroll")
              fixture.render()
              assertTrue(
                  fixture.requestDescriptionFocus("Retry opening $destination", "f04-files"), label)
              fixture.awaitDescriptionFocus("Retry opening $destination")
              fixture.assertDescriptionFullyVisible("Retry opening $destination", "f04-files")
              fixture.render("$label-retry-focused")
              assertTrue(fixture.hasText("The current file remains open."), label)
              assertEquals(0, actions, label)
            }
      }
    }
  }

  @Test
  fun f23RetainedDraftIdentityRemainsAvailableAcrossCompactReflow() {
    val evidence = sourceNavigationReviewFixture()
    val file = requireNotNull(evidence.selected)
    val symbol = requireNotNull(evidence.selectedSymbol)
    val target = "internal/other/日本語/" + "nested/".repeat(8) + "user.go"
    val identity = CurrentEditIdentity(ChatEditMode.ReplaceSymbol, target, "Other", true)
    val binding = "Retained draft: $target · Other (not the inspected declaration)"
    val preferred = DesktopLayoutState(explorerWidth = 520f, actionWidth = 560f)
    var actions = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AdaptiveProductionEditorFixture(
              preferred,
              false,
              evidence = evidence,
              progress = EditorProgressUiState(EditorProgress.Inspect, "", identity),
              terminalCollapsed = true,
              onCreate = { actions++ },
              onRequest = { actions++ },
              onWrite = { actions++ },
              onSourceLine = { actions++ })
        }
        .use { fixture ->
          for ((width, height) in listOf(800 to 650, 1280 to 600, 1600 to 1000)) {
            fixture.resize(width, height)
            fixture.render()
            assertTrue(fixture.hasDescription("Project-relative path: ${file.path}"))
            assertTrue(fixture.hasText(binding))
            assertTrue(fixture.hasText("Inspecting declaration: ${symbol.name} · Lines 5–12"))
            assertTrue(fixture.hasText("Read-only"))
            fixture.horizontalScrollBy("editor-path", 10000f)
            fixture.horizontalScrollBy("editor-retained-draft", 10000f)
            fixture.render("f23-retained-draft-$width-$height")
            assertTrue(fixture.horizontalScrollValue("editor-path") > 0f)
            assertTrue(fixture.horizontalScrollValue("editor-retained-draft") > 0f)
            if (width == 800) {
              fixture.revealTagFullyWithin("source-viewport", "f04-arrangement")
              fixture.render("f23-retained-draft-source-$width-$height")
            }
            assertTrue(fixture.taggedBounds("source-viewport").height >= 100f)
            assertEquals(0, actions)
          }
        }
  }

  @Test
  fun f23UnchangedSourceRetainsMultilineCopyBothScrollAxesInspectionAndDraftFocus() {
    val evidence = sourceNavigationReviewFixture()
    val symbol = requireNotNull(evidence.selectedSymbol)
    val draft = requireNotNull(evidence.draft)
    var input by
        mutableStateOf(TextFieldValue(draft.declaration + " // local edit", TextRange(7, 13)))
    var scale by mutableStateOf(1f)
    var right by mutableStateOf(RightToolWindow.Context)
    var actions = 0
    val preferred = DesktopLayoutState(explorerWidth = 520f, actionWidth = 560f)
    ComposeVisualFixture(1600, 1000) {
          CompositionLocalProvider(LocalDensity provides Density(1f, scale)) {
            AdaptiveProductionEditorFixture(
                preferred,
                false,
                evidence = evidence,
                rightTool = right,
                onRightTool = { right = it },
                draftInput = input,
                onDraftInput = { input = it },
                terminalCollapsed = true,
                onSourceLine = { actions++ },
                onRequest = { actions++ },
                onWrite = { actions++ },
                onValidate = { actions++ },
                onCreate = { actions++ })
          }
        }
        .use { fixture ->
          fixture.render("f23-continuity-before")
          fixture.dragSourceText(5, 0, 7, 25)
          fixture.pressKey(Key.Copy)
          val copied = fixture.clipboardText()
          assertTrue(copied.contains('\n'))
          assertTrue(copied.startsWith("func GetUser"), copied)
          for ((width, textScale) in listOf(800 to 1f, 1600 to 1.25f, 800 to 1.5f, 1600 to 1f)) {
            fixture.resize(width, 1000)
            scale = textScale
            fixture.render("f23-continuity-selection-$width-$textScale")
            fixture.pressKey(Key.Copy)
            assertEquals(copied, fixture.clipboardText())
            assertTrue(
                fixture.hasDescription("Selected declaration ${symbol.name} marker at line 5"))
            assertEquals(0, actions)
          }
          fixture.horizontalScrollWithin("source-viewport", 230f)
          fixture.scrollTagged("source-vertical", false, 200f)
          fixture.render()
          val horizontal = fixture.horizontalScrollWithinValue("source-viewport")
          val vertical = fixture.scrollPosition("source-vertical", false)
          assertTrue(horizontal > 0f && vertical > 0f)
          fixture.clickText("Assistant")
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Declaration only"))
          fixture.render()
          val edited = input
          for ((width, textScale) in listOf(800 to 1.5f, 1600 to 1.25f, 800 to 1f, 1600 to 1f)) {
            fixture.resize(width, 1000)
            scale = textScale
            fixture.render("f23-continuity-draft-$width-$textScale")
            assertEquals(edited, input)
            assertTrue(fixture.isDescriptionFocused("Declaration only"))
            assertEquals(horizontal, fixture.horizontalScrollWithinValue("source-viewport"), 1f)
            assertEquals(vertical, fixture.scrollPosition("source-vertical", false), 1f)
            assertEquals(symbol, evidence.selectedSymbol)
            assertEquals(0, actions)
          }
        }
  }

  @Test
  fun adaptiveProductionEditorCapturesBothSidesOfItsMeasuredBoundary() {
    val preferred = DesktopLayoutState(bottomHeight = 140f)
    for (scale in listOf(1f, 1.25f, 1.5f)) {
      val boundary =
          (TOOL_WINDOW_BAR_WIDTH * scale +
                  2 * WORKSPACE_FRAME_INSET +
                  (DesktopLayoutState.MIN_EXPLORER_WIDTH +
                      MIN_EDITOR_CANVAS_WIDTH +
                      DesktopLayoutState.MIN_ACTION_WIDTH) * scale +
                  2 * RESIZE_DIVIDER_WIDTH)
              .toInt()
      for (width in listOf(boundary - 1, boundary, boundary + 1)) {
        val mode = resolveDesktopLayout(preferred, width.toFloat(), scale).mode
        ComposeVisualFixture(width, 768, scale) {
              AdaptiveProductionEditorFixture(preferred, review = false)
            }
            .use { fixture ->
              fixture.render("f04-boundary-$width-$scale-$mode")
              val files = fixture.taggedBounds("f04-files")
              val canvas = fixture.taggedBounds("f04-canvas")
              if (mode == DesktopLayoutMode.Wide) {
                assertTrue(files.right <= canvas.left)
                assertTrue(canvas.width >= MIN_EDITOR_CANVAS_WIDTH * scale - 2f)
              } else {
                assertTrue(files.bottom <= canvas.top)
                fixture.scrollBy(100_000f, "f04-arrangement")
                fixture.render()
                assertTrue(fixture.taggedBounds("f04-tool").height > 0)
              }
              assertTrue(
                  fixture.taggedBounds("f04-dock").bottom <=
                      fixture.taggedBounds("model-count-footer").top)
            }
      }
    }
  }

  @Test
  fun adaptiveEditorRetainsEditedDraftSourceDiffAndRunningTerminalAcrossBreakpoint() {
    val original = editorComparisonReviewFixture()
    val file =
        requireNotNull(original.selected).let {
          it.copy(content = it.content + "\n" + "veryLongArgument".repeat(30))
        }
    val longLine = "    return repository.Lookup(id, " + "veryLongArgument".repeat(25) + ")"
    val draft =
        requireNotNull(original.draft).let {
          it.copy(
              validation =
                  it.validation!!.copy(
                      diff =
                          UnifiedDiff(
                              file.path,
                              file.path,
                              listOf(
                                  DiffLine("context", 1, 1, longLine),
                                  DiffLine("removed", 2, 0, longLine),
                                  DiffLine("added", 0, 2, longLine + " // candidate")))))
        }
    val evidence = original.copy(selected = file, draft = draft)
    var input by mutableStateOf(TextFieldValue(draft.declaration))
    var right by mutableStateOf(RightToolWindow.Context)
    var surface by mutableStateOf(EditorSurface.Source)
    val runningTab =
        TerminalTabState(17, "Shell 17", TerminalSessionState(TerminalSessionPhase.Running))
    val terminalState = TerminalWorkspaceState(tabs = listOf(runningTab), activeTabId = 17)
    var validations = 0
    var requests = 0
    var writes = 0
    var starts = 0
    val preferred = DesktopLayoutState(explorerWidth = 520f, actionWidth = 560f)
    ComposeVisualFixture(1600, 900, 1.5f) {
          AdaptiveProductionEditorFixture(
              preferred,
              review = false,
              evidence = evidence,
              terminalState = terminalState,
              rightTool = right,
              onRightTool = { right = it },
              selectedSurface = surface,
              onSurface = { surface = it },
              draftInput = input,
              onDraftInput = { input = it },
              onValidate = { validations++ },
              onRequest = { requests++ },
              onWrite = { writes++ },
              onTerminal = { starts++ })
        }
        .use { fixture ->
          fun assertPassiveState() {
            assertEquals(RightToolWindow.Assistant, right)
            assertEquals(terminalState.activeTab, runningTab)
            assertTrue(fixture.hasText("Shell 17"))
            assertEquals(0, validations + requests + writes + starts)
          }
          fixture.render("f04-reflow-wide")
          assertTrue(
              fixture.taggedBounds("f04-files").right <= fixture.taggedBounds("f04-canvas").left)
          assertEquals(DesktopLayoutMode.Wide, resolveDesktopLayout(preferred, 1600f, 1.5f).mode)
          assertEquals(DesktopLayoutMode.Compact, resolveDesktopLayout(preferred, 800f, 1.5f).mode)
          assertFalse(fixture.hasEditableText(withinTag = "source-viewport"))
          val selectedSource = fixture.copyTextByDragging("package api")
          assertTrue(selectedSource.isNotEmpty())
          for (width in listOf(800, 1600)) {
            fixture.resize(width, 900)
            fixture.render()
            fixture.pressKey(Key.Copy)
            assertEquals(selectedSource, fixture.clipboardText())
          }
          fixture.clickText("Assistant")
          fixture.render()
          assertEquals(RightToolWindow.Assistant, right)
          fixture.horizontalScrollWithin("source-viewport", 230f)
          fixture.render()
          val sourceScroll = fixture.horizontalScrollWithinValue("source-viewport")
          assertTrue(sourceScroll > 0f)
          assertTrue(fixture.requestDescriptionFocus("Declaration only"))
          fixture.render()
          assertTrue(fixture.isDescriptionFocused("Declaration only"))
          fixture.setFocusedText(draft.declaration + " // edited")
          fixture.render()
          fixture.selectEditorText("Declaration only", 7, 13)
          fixture.render()
          val edited = input
          assertEquals(TextRange(7, 13), edited.selection)
          assertTrue(fixture.isDescriptionFocused("Declaration only"))
          for (width in listOf(800, 1600)) {
            fixture.resize(width, 900)
            fixture.render("f04-reflow-source-$width")
            assertEquals(edited, input)
            assertTrue(fixture.isDescriptionFocused("Declaration only"))
            assertEquals(sourceScroll, fixture.horizontalScrollWithinValue("source-viewport"), 5f)
            assertPassiveState()
          }
          fixture.clickText("Candidate diff")
          fixture.render()
          assertEquals(EditorSurface.Review, surface)
          assertFalse(fixture.hasEditableText("Read-only composed diff"))
          val selectedDiff = fixture.copyTextByDragging(longLine)
          assertTrue(selectedDiff.isNotEmpty())
          for (width in listOf(800, 1600)) {
            fixture.resize(width, 900)
            fixture.render()
            fixture.pressKey(Key.Copy)
            assertEquals(selectedDiff, fixture.clipboardText())
          }
          fixture.horizontalScrollBy("diff-Current-horizontal", 220f)
          fixture.render()
          val currentScroll = fixture.horizontalScrollValue("diff-Current-horizontal")
          assertTrue(currentScroll > 0f)
          assertEquals(0f, fixture.horizontalScrollValue("diff-Candidate-horizontal"))
          for (width in listOf(800, 1600)) {
            fixture.resize(width, 900)
            fixture.render("f04-reflow-diff-$width")
            assertEquals(
                currentScroll, fixture.horizontalScrollValue("diff-Current-horizontal"), 5f)
            assertEquals(0f, fixture.horizontalScrollValue("diff-Candidate-horizontal"))
            assertEquals(edited.text, input.text)
            assertEquals(EditorSurface.Review, surface)
            assertPassiveState()
          }
          fixture.clickDescription("Unified diff")
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Unified diff"))
          fixture.resize(800, 900)
          fixture.render("f04-reflow-unified-compact")
          fixture.resize(1600, 900)
          fixture.render("f04-reflow-unified-wide")
          assertTrue(fixture.isDescriptionSelected("Unified diff"))
          assertEquals(edited.text, input.text)
          assertPassiveState()
        }
  }

  @Test
  fun modelFooterSeparatesFromShellAndTrailsAtLargerTextAndReducedWidth() {
    val project = resultProjectFixture()
    val state = DesktopState(projectState = ProjectWorkspaceState(project))
    val model = ScopedModel(model = "local-model", providerOrigin = "http://localhost:11434")
    val available =
        desktopStatusBarPresentation(state, DesktopShellStatusProviders(model, model, model))
    val unavailable =
        desktopStatusBarPresentation(
            state, DesktopShellStatusProviders(model, model, ScopedModel()))
    for ((width, height, scale) in
        listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f), Triple(1280, 600, 1.25f))) {
      for (presentation in listOf(available, unavailable)) {
        var opens = 0
        ComposeVisualFixture(width, height, scale) {
              Column(Modifier.fillMaxSize().background(AppBackground)) {
                MainToolbar(
                    ToolbarState(project, false, "", ConnectionState(), null),
                    ToolbarActions({}, {}, {}, {}))
                WorkspaceFrame(
                    rail = { ToolWindowBar(LeftToolWindow.Summary, {}, onOpenTerminal = {}) },
                    panes = { EditorArea({ Text("Project workspace") }, Modifier.weight(1f)) },
                    terminal = {
                      TerminalBar(
                          TerminalWorkspaceState(), true, {}, TerminalTabActions({}, {}, {}))
                    },
                    modifier = Modifier.weight(1f))
                PersistentStatusBar(presentation, { opens++ })
              }
            }
            .use { fixture ->
              fixture.render("shell-footer-$width-$scale-${presentation === unavailable}")
              val footer = fixture.taggedBounds("model-count-footer")
              val action = fixture.taggedBounds("model-count-action")
              assertEquals(width.toFloat(), footer.right, 1f)
              assertEquals(height.toFloat(), footer.bottom, 1f)
              assertEquals(width - 16f, action.right, 1f)
              assertTrue(action.top > 0 && action.bottom <= footer.bottom)
              fixture.assertTextFits(presentation.modelsLabel)
              fixture.assertTopKeyline("model-count-footer", PaneSeparator)
              fixture.clickDescription("Configured model details")
              assertEquals(1, opens)
            }
      }
    }
  }

  @Test
  fun f02AdmissionAndReviewMatrixKeepsDecisionsAndFailuresReachable() {
    val destination = "https://provider.example/日本語/" + "long-destination/".repeat(3)
    val error = "Destination unavailable. Refresh the preview before starting."
    val preview =
        analysisPreviewFixture().let { original ->
          original.copy(
              providers =
                  original.providers.map { provider ->
                    provider.copy(model = provider.model.copy(providerOrigin = destination))
                  })
        }
    val review = editorComparisonReviewFixture()
    val failedChecks =
        review.checks!!.copy(
            checks =
                listOf(
                    DraftCheck(
                        "go test",
                        true,
                        "failed",
                        listOf("go", "test", "./..."),
                        "Focused checks failed. Fix the test before applying.")))
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        var confirmations = 0
        var starts = 0
        ComposeVisualFixture(width, height, scale) {
              Box(Modifier.fillMaxSize().background(Panel), contentAlignment = Alignment.Center) {
                IdeDialogSurface(
                    maxHeight = 520.dp,
                    title = { Text("Analyze whole project") },
                    content = {
                      DesktopAnalysisAdmissionContent(
                          ProjectAnalysisRunState(
                              admission = AnalysisAdmission(preview), error = error),
                          { _, _ -> confirmations++ },
                          { confirmations++ })
                    },
                    actions = {
                      MiniOrcaButton(onClick = {}, tone = ActionTone.Neutral) { Text("Close") }
                      MiniOrcaButton(
                          onClick = { starts++ }, enabled = false, tone = ActionTone.Primary) {
                            Text("Start analysis")
                          }
                    })
              }
            }
            .use { fixture ->
              fixture.render("f02-admission-$width-$height-$scale")
              fixture.revealText(error, "ide-dialog-body")
              fixture.assertTextFits("Close")
              fixture.assertTextFits("Start analysis")
              fixture.revealText("Remote destination: $destination", "ide-dialog-body")
              fixture.revealText("Include AI Security review", "ide-dialog-body")
              fixture.render("f02-admission-consent-$width-$height-$scale")
              assertTrue(fixture.hasDescription("Include AI Security review"))
              assertTrue(fixture.isDisabled("Start analysis"))
              assertEquals(0, confirmations)
              assertEquals(0, starts)
            }
        var operations = 0
        ComposeVisualFixture(width, height, scale) {
              ReviewToolWindow(
                  review.copy(checks = failedChecks),
                  ReviewToolWindowActions({ operations++ }, { operations++ }, { operations++ }),
                  DraftApplicationActions({ operations++ }, { operations++ }))
            }
            .use { fixture ->
              fixture.render("f02-review-failed-$width-$height-$scale")
              assertTrue(fixture.hasText("Failed"))
              assertTrue(fixture.hasText("Failed check details"))
              assertFalse(fixture.hasText("Ready to apply"))
              fixture.clickText("Failed check details")
              fixture.render("f02-review-failed-detail-$width-$height-$scale")
              fixture.revealText("Focused checks failed. Fix the test before applying.")
              fixture.render("f02-review-failed-evidence-$width-$height-$scale")
              assertEquals(0, operations)
            }
      }
    }
  }

  @Test
  fun f02DensityCapturesRetainProductionStateAndExpandedEvidence() {
    val output = "start\n" + "long diagnostic\n".repeat(330) + "last available line"
    for (density in listOf(1f, 2f)) {
      val width = (800 * density).toInt()
      val height = (650 * density).toInt()
      var changes = 0
      ComposeVisualFixture(width, height, 1.5f, densityScale = density) {
            Column(
                Modifier.fillMaxSize()
                    .background(Panel)
                    .verticalScroll(rememberScrollState())
                    .padding(12.dp)) {
                  ChromeTab(onClick = { changes++ }, selected = true, focusHighlight = true) {
                    Text("Selected analysis tab")
                  }
                  IdeCheckbox(
                      checked = false,
                      onCheckedChange = { changes++ },
                      accessibleName = "Confirm remote destination",
                      label = "Confirm remote destination")
                  CompactSingleLineField(
                      value = "invalid path",
                      onValueChange = { changes++ },
                      label = "Project path",
                      errorText = "Choose a valid project path")
                  DiagnosticText(output)
                }
          }
          .use { fixture ->
            fixture.render("f02-controls-evidence-${density}x-collapsed")
            fixture.assertTextFits("Selected analysis tab")
            fixture.assertColorVisible(FocusAccent)
            fixture.assertColorVisible(SelectionAccent)
            fixture.assertTextFits("Choose a valid project path")
            assertTrue(fixture.hasText("… output truncated"))
            fixture.revealText("Show full available output")
            fixture.render("f02-controls-evidence-${density}x-preview-action")
            fixture.clickDescription("Expand available diagnostic output")
            fixture.render("f02-controls-evidence-${density}x-expanded")
            assertTrue(fixture.hasText(output))
            assertTrue(fixture.taggedBounds("diagnostic-output-scroll").height <= 240f * density)
            val disclosure = fixture.firstVisibleTextBounds("Show preview")
            assertTrue(disclosure.bottom <= height, "Expanded disclosure must remain reachable")
            assertEquals(0, changes)
          }
    }
  }

  @Test
  fun f14ConsentAndRecoveryKeepDecisionsReachableAcrossViewportTextAndDensity() {
    val base = analysisPreviewFixture()
    val run = analysisRunFixture()
    val origin = "https://provider.example/日本語/" + "long-destination/".repeat(9) + "end"
    val providers =
        base.providers.mapIndexed { index, provider ->
          provider.copy(
              model =
                  provider.model.copy(
                      scope = "shared", model = "same-model", providerOrigin = origin),
              id = "remote-$index")
        }
    val files =
        (1..22).map { index -> base.files.single().copy(path = "src/package/file-$index.go") }
    val preview = base.copy(providers = providers, files = files)
    val intent =
        AnalysisPreviewIntent(
            "project", "revision", base.limits, true, false, run.identity, run.plan)
    val diagnostic = "Admission failed: " + "long diagnostic detail ".repeat(25) + "end"
    val states =
        listOf(
            "unchecked" to ProjectAnalysisRunState(admission = AnalysisAdmission(preview)),
            "partial" to
                ProjectAnalysisRunState(
                    admission = AnalysisAdmission(preview, providerIds = setOf("remote-0"))),
            "ready" to
                ProjectAnalysisRunState(
                    admission =
                        AnalysisAdmission(
                            preview,
                            providerIds = providers.map { it.id }.toSet(),
                            securityReview = true)),
            "resume" to
                ProjectAnalysisRunState(
                    admission =
                        AnalysisAdmission(
                            preview, run.identity, providers.map { it.id }.toSet(), true)),
            "empty" to
                ProjectAnalysisRunState(
                    admission =
                        AnalysisAdmission(
                            preview.copy(files = emptyList()),
                            providerIds = providers.map { it.id }.toSet(),
                            securityReview = true)),
            "loading" to ProjectAnalysisRunState(action = "preview", previewIntent = intent),
            "rejected" to
                ProjectAnalysisRunState(
                    run = run,
                    previewIntent = intent,
                    admissionRecovery = AdmissionRecovery.Rejected,
                    error = diagnostic),
            "obsolete" to
                ProjectAnalysisRunState(
                    run = run.copy(identity = run.identity.copy(generation = "new")),
                    previewIntent = intent,
                    admissionRecovery = AdmissionRecovery.Rejected,
                    error = diagnostic),
            "uncertain" to
                ProjectAnalysisRunState(
                    run = run, admissionRecovery = AdmissionRecovery.Uncertain, error = diagnostic))
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      // Same logical viewport at both densities, independent of font scaling.
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        for ((variant, state) in states) {
          var operations = 0
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    Box(
                        Modifier.fillMaxSize().background(Panel),
                        contentAlignment = Alignment.Center) {
                          IdeDialogSurface(
                              maxHeight = (height - 64).coerceAtMost(520).dp,
                              title = { DesktopAnalysisAdmissionTitle(state) },
                              content = {
                                DesktopAnalysisAdmissionContent(
                                    state, { _, _ -> operations++ }, { operations++ })
                              },
                              actions = {
                                DesktopAnalysisAdmissionActions(
                                    state, { operations++ }, { operations++ }, { operations++ })
                              },
                              focusSafeActionOnOpen = true)
                        }
                  }
              .use { fixture ->
                val label = "f14-$variant-$width-$height-$scale-${density}x"
                fixture.render(label)
                val body = fixture.taggedBounds("ide-dialog-body")
                val close = fixture.firstVisibleTextBounds("Close")
                assertTrue(body.height > 0 && body.bottom <= close.top, label)
                assertTrue(close.top >= 0 && close.bottom <= height * density, label)
                assertTrue(fixture.isFocusedControl("Close"), label)
                fixture.assertTextFits("Close")
                val decision =
                    when (variant) {
                      "loading",
                      "obsolete",
                      "uncertain" -> null
                      "rejected" -> "Review fresh preview"
                      "resume" -> "Resume analysis"
                      else -> "Start analysis"
                    }
                if (decision != null) {
                  val action = fixture.firstVisibleTextBounds(decision)
                  assertTrue(action.top >= body.bottom && action.bottom <= height * density, label)
                  fixture.assertTextFits(decision)
                }
                when (variant) {
                  "unchecked",
                  "partial",
                  "ready",
                  "resume",
                  "empty" -> {
                    assertEquals(
                        22.takeIf { variant != "empty" } ?: 0, state.admission!!.preview.files.size)
                    assertTrue(
                        fixture.hasDescription("Expand Included file 1 · src/package/file-1.go") ||
                            variant == "empty",
                        label)
                    for (provider in providers) {
                      val consent =
                          "Confirm shared destination · same-model (provider ${provider.id})"
                      assertEquals(1, fixture.clickableDescriptionCount(consent), label)
                      assertEquals(
                          if (provider.id in state.admission.providerIds) ToggleableState.On
                          else ToggleableState.Off,
                          fixture.descriptionToggleableState(consent),
                          label)
                      assertEquals(
                          if (provider.id in state.admission.providerIds) "Confirmed"
                          else "Not confirmed",
                          fixture.descriptionStateDescription(consent),
                          label)
                    }
                    assertTrue(fixture.hasDescription("Include AI Security review"), label)
                    assertEquals(
                        variant !in listOf("ready", "resume"),
                        fixture.isDisabled(decision!!),
                        label)
                    fixture.revealText("Remote destination: $origin", "ide-dialog-body")
                    fixture.assertTextWrapsWithoutClipping("Remote destination: $origin")
                    if (variant == "empty")
                        assertTrue(fixture.hasText("No eligible files to analyze."), label)
                  }
                  "loading" -> {
                    assertTrue(
                        fixture.hasText("Preparing continuation preview for this analysis run…"),
                        label)
                    assertFalse(
                        fixture.hasText("Expected model requests without retries: 3"), label)
                  }
                  else -> {
                    fixture.revealText(diagnostic, "ide-dialog-body")
                    fixture.assertTextWrapsWithoutClipping(diagnostic)
                    assertTrue(fixture.copyTextByDragging(diagnostic).isNotBlank(), label)
                    assertEquals(
                        variant == "rejected", fixture.hasText("Review fresh preview"), label)
                    assertFalse(fixture.hasText("Start analysis"), label)
                    assertFalse(fixture.hasText("Resume analysis"), label)
                  }
                }
                fixture.render("$label-inspected")
                assertEquals(0, operations, label)
                if (variant == "unchecked" && width == 800 && scale == 1.5f) {
                  val consent = "Confirm shared destination · same-model (provider remote-0)"
                  fixture.scrollBy(-100_000f, "ide-dialog-body")
                  fixture.render()
                  for (attempt in 0 until 80) {
                    val bounds = fixture.descriptionBounds(consent)
                    if (bounds.height > 0 && bounds.top >= body.top && bounds.bottom <= body.bottom)
                        break
                    fixture.scrollBy(80f, "ide-dialog-body")
                    fixture.render()
                  }
                  val bounds = fixture.descriptionBounds(consent)
                  assertTrue(
                      bounds.height > 0 && bounds.top >= body.top && bounds.bottom <= body.bottom,
                      label)
                  assertTrue(fixture.requestDescriptionFocus(consent), label)
                  fixture.render("$label-focused-consent")
                  assertTrue(fixture.isDescriptionFocused(consent), label)
                  assertEquals("Not confirmed", fixture.descriptionStateDescription(consent), label)
                  assertEquals(0, operations, label)
                  val disclosure = "Expand Included file 1 · src/package/file-1.go"
                  assertTrue(fixture.requestDescriptionFocus(disclosure), label)
                  assertTrue(fixture.pressKey(Key.Enter), label)
                  fixture.render("$label-disclosed")
                  assertEquals(
                      "Expanded",
                      fixture.descriptionStateDescription(
                          "Collapse Included file 1 · src/package/file-1.go"),
                      label)
                  fixture.resize((1024 * density).toInt(), (768 * density).toInt())
                  fixture.render("$label-resized")
                  assertEquals(
                      "Expanded",
                      fixture.descriptionStateDescription(
                          "Collapse Included file 1 · src/package/file-1.go"),
                      label)
                  assertEquals(0, operations, label)
                  assertTrue(fixture.isDisabled("Start analysis"), label)
                  assertEquals(
                      ToggleableState.Off,
                      fixture.descriptionToggleableState(
                          "Confirm shared destination · same-model (provider remote-0)"),
                      label)
                }
              }
        }
      }
    }
  }

  @Test
  fun f13AdmissionModesAndRecoveryRenderAcrossHostSizesAndTextScales() {
    val base = analysisPreviewFixture()
    val run = analysisRunFixture()
    val file =
        base.files
            .single()
            .copy(
                stages =
                    listOf(
                        AnalysisStagePlan(
                            "semantic", true, false, "Fresh review", "bug-provider", 1),
                        AnalysisStagePlan("security_rules", true, true, "Reused rules", "", 0)))
    val exclusion = AnalysisExcludedFile("excluded/generated.go", "Source policy exclusion")
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        val variants =
            listOf(
                "full" to
                    ProjectAnalysisRunState(
                        admission =
                            AnalysisAdmission(
                                base.copy(files = listOf(file), excluded = listOf(exclusion)))),
                "selective" to
                    ProjectAnalysisRunState(
                        admission =
                            AnalysisAdmission(
                                base.copy(
                                    files = listOf(file),
                                    excluded = listOf(exclusion),
                                    retryStaleFailed = true,
                                    refresh = false))),
                "resume" to
                    ProjectAnalysisRunState(
                        admission =
                            AnalysisAdmission(
                                base.copy(
                                    files = listOf(file),
                                    expectedModelRequests = 1,
                                    maxModelRequests = 2,
                                    compatibilityStage = "semantic"),
                                run.identity)),
                "empty" to
                    ProjectAnalysisRunState(
                        admission =
                            AnalysisAdmission(
                                base.copy(files = emptyList(), excluded = listOf(exclusion)))),
                "loading" to
                    ProjectAnalysisRunState(
                        action = "preview",
                        previewIntent =
                            AnalysisPreviewIntent("project", "revision", base.limits, false, true)),
                "failure" to
                    ProjectAnalysisRunState(
                        error = "Preview unavailable: connection refused",
                        previewIntent =
                            AnalysisPreviewIntent("project", "revision", base.limits, false, true)))
        for ((variant, initial) in variants) {
          var state by mutableStateOf(initial)
          var operations = 0
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    Box(
                        Modifier.fillMaxSize().background(Panel),
                        contentAlignment = Alignment.Center) {
                          IdeDialogSurface(
                              maxHeight = (height - 64).coerceAtMost(520).dp,
                              title = { DesktopAnalysisAdmissionTitle(state) },
                              content = {
                                DesktopAnalysisAdmissionContent(
                                    state, { _, _ -> operations++ }, { operations++ })
                              },
                              actions = {
                                DesktopAnalysisAdmissionActions(
                                    state, { operations++ }, { operations++ }, { operations++ })
                              },
                              focusSafeActionOnOpen = true)
                        }
                  }
              .use { fixture ->
                val label = "f13-$variant-$width-$height-$scale-${density}x"
                fixture.render(label)
                assertTrue(fixture.isFocusedControl("Close"), label)
                fixture.assertTextFits("Close")
                when (variant) {
                  "loading" ->
                      assertTrue(
                          fixture.hasText("Preparing stale & failed analysis preview…"), label)
                  "failure" -> {
                    fixture.revealText("Preview unavailable: connection refused", "ide-dialog-body")
                    fixture.render("$label-diagnostic")
                    assertTrue(fixture.hasText("Retry preview"), label)
                  }
                  else -> {
                    assertTrue(
                        fixture.hasText(
                            "Expected model requests without retries: ${state.admission!!.preview.expectedModelRequests}"),
                        label)
                    if (variant == "empty") assertTrue(fixture.isDisabled("Start analysis"), label)
                    if (variant == "resume")
                        assertTrue(
                            fixture.hasText("This saved run covers only Code analysis."), label)
                    if (variant != "resume")
                        fixture.revealText("Source policy exclusion", "ide-dialog-body")
                    fixture.render("$label-exclusions")
                  }
                }
                assertEquals(0, operations, label)
              }
        }
      }
    }
  }

  @Test
  fun analysisAdmissionManyRecordsRemainReachableWithoutDispatchAcrossDialogSizes() {
    val base = analysisPreviewFixture()
    val longPath = "src/日本語/" + "nested-package/".repeat(10) + "last.go"
    val files =
        (1..24).map { index ->
          AnalysisPlannedFile(
              if (index == 24) longPath else "src/package/file-$index.go",
              "hash-$index",
              "Go",
              40,
              listOf(
                  AnalysisStagePlan(
                      "semantic",
                      true,
                      false,
                      "Backend stage reason for included file $index",
                      "bug-provider",
                      1)))
        }
    val lastReason = "Selective retry exclusion: " + "no stale evidence ".repeat(5) + "end"
    val stageReason = "Backend stage reason for included file 24: " + "details ".repeat(7) + "end"
    val origin = "https://example.test/" + "long-origin/".repeat(7) + "end"
    val exclusions =
        (1..16).map { index ->
          AnalysisExcludedFile(
              "excluded/日本語/file-$index.go",
              if (index == 16) lastReason
              else "Policy or source eligibility reason for excluded file $index")
        }
    val sizes = listOf(1440 to 900, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) {
      val density = if (width == 800) 2f else 1f
      val scale = if (width == 1440) 1f else 1.5f
      val resume = width == 1280
      val preview =
          base.copy(
              files =
                  files.map { file ->
                    if (file.path == longPath)
                        file.copy(stages = file.stages.map { it.copy(reason = stageReason) })
                    else file
                  },
              excluded = exclusions,
              providers =
                  base.providers.map { provider ->
                    provider.copy(model = provider.model.copy(providerOrigin = origin))
                  })
      var state by
          mutableStateOf(
              ProjectAnalysisRunState(
                  admission =
                      AnalysisAdmission(
                          preview, if (resume) analysisRunFixture().identity else null)))
      var starts = 0
      var closes = 0
      var confirmations = 0
      val action = if (resume) "Resume analysis" else "Start analysis"
      ComposeVisualFixture((width * density).toInt(), (height * density).toInt(), scale, density) {
            Box(Modifier.fillMaxSize().background(Panel), contentAlignment = Alignment.Center) {
              IdeDialogSurface(
                  maxHeight = (height - 64).coerceAtMost(520).dp,
                  title = { DesktopAnalysisAdmissionTitle(state) },
                  content = {
                    DesktopAnalysisAdmissionContent(
                        state,
                        { id, checked ->
                          confirmations++
                          state =
                              state.copy(
                                  admission =
                                      state.admission!!.copy(
                                          providerIds =
                                              if (checked) state.admission!!.providerIds + id
                                              else state.admission!!.providerIds - id))
                        },
                        { checked ->
                          confirmations++
                          state =
                              state.copy(
                                  admission = state.admission!!.copy(securityReview = checked))
                        })
                  },
                  actions = {
                    DesktopAnalysisAdmissionActions(state, { closes++ }, { starts++ }, {})
                  },
                  focusSafeActionOnOpen = true,
                  onDismissRequest = { closes++ })
            }
          }
          .use { fixture ->
            val label = "f13-many-$width-$height-$scale-${density}x"
            fun assertActions() {
              val body = fixture.taggedBounds("ide-dialog-body")
              for (text in listOf("Close", action)) {
                val bounds = fixture.firstVisibleTextBounds(text)
                assertTrue(bounds.top >= 0 && bounds.bottom <= height * density, "$label: $text")
                assertTrue(bounds.top >= body.bottom, "$label: $text must remain below the body")
              }
            }
            fixture.render("$label-collapsed")
            assertTrue(fixture.isFocusedControl("Close"), label)
            assertTrue(fixture.isDisabled(action), label)
            assertEquals(
                24,
                files.withIndex().count { (index, file) ->
                  fixture.hasDescription("Expand Included file ${index + 1} · ${file.path}")
                },
                label)
            for (excluded in exclusions) {
              assertEquals(1, fixture.textCount(excluded.path), "$label: ${excluded.path}")
              assertEquals(1, fixture.textCount(excluded.reason), "$label: ${excluded.path}")
            }
            assertActions()
            val last = "Included file 24 · $longPath"
            fixture.revealText(longPath, "ide-dialog-body")
            fixture.assertTextWrapsWithoutClipping(longPath)
            fixture.copyTextByDragging(longPath, longPath)
            assertTrue(fixture.requestDescriptionFocus("Expand $last"), label)
            fixture.render()
            assertTrue(fixture.isDescriptionFocused("Expand $last"), label)
            assertTrue(fixture.pressKey(Key.Enter), label)
            fixture.render("$label-expanded")
            assertEquals("Expanded", fixture.descriptionStateDescription("Collapse $last"))
            fixture.revealText("Reason: $stageReason", "ide-dialog-body")
            fixture.assertTextWrapsWithoutClipping("Reason: $stageReason")
            fixture.copyTextByDragging("Reason: $stageReason", "Reason: $stageReason")
            if (width == 800) {
              fixture.resize(1024 * 2, 768 * 2)
              fixture.render("$label-resized-expanded")
              assertEquals("Expanded", fixture.descriptionStateDescription("Collapse $last"))
              fixture.resize(width * 2, height * 2)
              fixture.render()
              assertEquals("Expanded", fixture.descriptionStateDescription("Collapse $last"))
              assertActions()
            }
            assertTrue(fixture.requestDescriptionFocus("Collapse $last"), label)
            assertTrue(fixture.pressKey(Key.Spacebar), label)
            fixture.render("$label-collapsed-again")
            assertEquals("Collapsed", fixture.descriptionStateDescription("Expand $last"))
            if (width == 800) {
              for (excluded in exclusions) {
                fixture.revealText(excluded.reason, "ide-dialog-body")
                val body = fixture.taggedBounds("ide-dialog-body")
                val record = fixture.firstVisibleTextBounds(excluded.reason)
                assertTrue(record.top >= body.top && record.bottom <= body.bottom, excluded.path)
              }
            }
            fixture.revealText(lastReason, "ide-dialog-body")
            fixture.assertTextWrapsWithoutClipping(lastReason)
            val body = fixture.taggedBounds("ide-dialog-body")
            val finalRecord = fixture.firstVisibleTextBounds(lastReason)
            assertTrue(finalRecord.top >= body.top && finalRecord.bottom <= body.bottom, label)
            fixture.render("$label-final-exclusion")
            fixture.copyTextByDragging(lastReason, lastReason)
            fixture.revealText("Remote destination: $origin", "ide-dialog-body")
            fixture.assertTextWrapsWithoutClipping("Remote destination: $origin")
            fixture.copyTextByDragging("Remote destination: $origin", "Remote destination: $origin")
            assertTrue(fixture.verticalScrollValue("ide-dialog-body") > 0f, label)
            assertActions()
            assertEquals(0, starts + closes + confirmations, label)
            assertFalse(state.admission!!.isConfirmed(), label)
            assertTrue(fixture.isDisabled(action), label)
            if (width == 800) {
              for (description in
                  listOf(
                      "Confirm bug destination · bug-model (provider bug-provider)",
                      "Confirm analyze destination · review-model (provider analyze-provider)",
                      "Include AI Security review")) {
                assertTrue(fixture.requestDescriptionFocus(description), description)
                fixture.render()
                assertTrue(fixture.pressKey(Key.Spacebar), description)
                fixture.render()
              }
              assertEquals(3, confirmations)
              assertTrue(state.admission!!.isConfirmed())
              assertEquals(0, starts)
              assertTrue(fixture.requestFocus(action))
              fixture.render("$label-enabled")
              assertActions()
              assertFalse(fixture.isDisabled(action))
              assertEquals(0, starts)
              assertTrue(fixture.pressKey(Key.Enter))
              fixture.render()
              assertEquals(1, starts, "Explicit admission must activate only once")
            }
            if (resume) {
              assertTrue(fixture.requestFocus("Close"))
              fixture.render()
              assertTrue(fixture.pressKey(Key.Escape))
              assertEquals(1, closes)
              assertEquals(0, starts)
            }
          }
    }
  }

  @Test
  fun admissionKeepsFailureConsentAndDecisionsReachableInBoundedDialog() {
    val error =
        "Destination unavailable: /project/日本語/long-provider-path. Refresh the preview before starting."
    for ((width, height) in listOf(800 to 650, 1280 to 600)) {
      val preview = analysisPreviewFixture()
      val state = ProjectAnalysisRunState(admission = AnalysisAdmission(preview), error = error)
      var starts = 0
      var confirmations = 0
      ComposeVisualFixture(width, height, 1.5f) {
            Box(Modifier.fillMaxSize().background(Panel), contentAlignment = Alignment.Center) {
              IdeDialogSurface(
                  maxHeight = 520.dp,
                  title = { Text("Analyze whole project") },
                  content = {
                    DesktopAnalysisAdmissionContent(
                        state, { _, _ -> confirmations++ }, { confirmations++ })
                  },
                  actions = {
                    MiniOrcaButton(onClick = {}, tone = ActionTone.Neutral) { Text("Close") }
                    MiniOrcaButton(
                        onClick = { starts++ }, enabled = false, tone = ActionTone.Primary) {
                          Text("Start analysis")
                        }
                  })
            }
          }
          .use { fixture ->
            fixture.render("admission-dialog-$width-$height-150")
            fixture.revealText(error, "ide-dialog-body")
            fixture.assertTextWrapsWithoutClipping(error)
            fixture.revealText("Remote destination: https://bug.example", "ide-dialog-body")
            fixture.revealText("Include AI Security review", "ide-dialog-body")
            val consent = fixture.firstVisibleTextBounds("Include AI Security review")
            assertTrue(consent.bottom <= height, "Consent must be reachable by body scrolling")
            for (label in listOf("Close", "Start analysis")) {
              val bounds = fixture.firstVisibleTextBounds(label)
              assertTrue(bounds.top >= 0 && bounds.bottom <= height, "$label must remain visible")
            }
            assertTrue(fixture.verticalScrollValue("ide-dialog-body") > 0f)
            assertEquals(0, starts)
            assertEquals(0, confirmations)
          }
    }
  }

  @Test
  fun productionNavigationAndCommandsRemainSansSerifWhileEvidenceIsMonospaced() {
    ComposeVisualFixture(1_600, 1_000) { RoundedSummaryVisualFixture(1_600f) }
        .use { fixture ->
          fixture.render()
          fixture.assertTextFontFamily("View analysis", FontFamily.SansSerif)
          fixture.assertTextFontFamily("Analysis coverage", FontFamily.Monospace)
        }
    ComposeVisualFixture(1_600, 1_000) { RoundedAnalysisVisualFixture(1_600f) }
        .use { fixture ->
          fixture.render()
          listOf("Pause", "Cancel", "All", "Needs attention", "Excluded").forEach {
            fixture.assertTextFontFamily(it, FontFamily.SansSerif)
          }
          fixture.assertTextFontFamily("Current: internal/api/user.go", FontFamily.Monospace)
        }
    ComposeVisualFixture(1_440, 900) { AcceptanceResultPane("bugs", "partial") }
        .use { fixture ->
          fixture.render()
          fixture.assertTextFontFamily("View analysis", FontFamily.SansSerif)
        }
  }

  @Test
  fun longAnalysisPathUsesTechnicalTypeWithoutLosingSelectableContent() {
    val path = "internal/services/identity/handlers/über-long-request-validation-handler.go"
    val selection = selectionFixture().copy(files = listOf(AnalysisSelectableFile(path, "")))
    ComposeVisualFixture(900, 420, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection)),
              AnalysisWorkspaceActions(
                  { _, _ -> },
                  {},
                  {},
                  {},
                  {},
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render("typography-analysis-long-path-900-150")
          fixture.assertTextFontFamily(path, FontFamily.Monospace)
          fixture.assertTextWrapsWithoutClipping(path)
          assertFalse(fixture.hasEditableText(withinTag = "analysis-file-row-$path"))
          fixture.assertTextFontFamily("Status unavailable", FontFamily.SansSerif)
        }
  }

  @Test
  fun unicodeProjectAndEngineeringProseStayReadableAtLargeText() {
    val purpose =
        "Résumé: 日本語 notes explain the project’s boundary and its local-first behavior. ".repeat(5)
    val mechanism =
        "日本語: the handler validates names before repository access; résumé preserved. ".repeat(3)
    val overview =
        visualFixtureOverview.copy(
            analysis = visualFixtureOverview.analysis.copy(purpose = purpose))
    ComposeVisualFixture(800, 650, 1.5f) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.render("typography-summary-unicode-800-150")
          fixture.assertTextFontFamily(purpose, FontFamily.SansSerif)
          fixture.assertTextWrapsWithoutClipping(purpose)
          assertFalse(fixture.hasEditableText(withinTag = "summary-introduction"))
        }
    ComposeVisualFixture(520, 440, 1.5f) {
          SummaryEngineeringInsightPanel(
              listOf(EngineeringInsightPiece("Mechanism", mechanism)),
              stale = false,
              ownerIdentity = "typography-fixture")
        }
        .use { fixture ->
          fixture.render("typography-engineering-unicode-520-150")
          fixture.assertTextFontFamily("Mechanism", FontFamily.Monospace)
          fixture.assertTextFontFamily(mechanism, FontFamily.SansSerif)
          fixture.assertTextWrapsWithoutClipping(mechanism)
        }
  }

  @Test
  fun wrappedFindingSummaryKeepsTechnicalLocationAndSansSerifNarrative() {
    val path = "internal/api/über-long-request-validation-handler.go:42"
    val summary =
        "The identifier may contain 日本語 input; validate it before repository access so the " +
            "caller receives the original failure instead of a misleading success."
    val finding =
        visualFixtureFindings
            .first()
            .copy(
                message = summary,
                location =
                    FindingLocation("internal/api/über-long-request-validation-handler.go", 42),
                freshness = "stale")
    ComposeVisualFixture(520, 650, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(16.dp)) {
            FindingDetailsRegion(finding, FindingActions({}, { _, _ -> }, {}))
          }
        }
        .use { fixture ->
          fixture.render("typography-finding-wrapped-520-150")
          fixture.assertTextFontFamily(path, FontFamily.Monospace)
          fixture.assertTextFontFamily(summary, FontFamily.SansSerif)
          fixture.assertTextWrapsWithoutClipping(summary)
          assertFalse(fixture.hasEditableText())
          assertTrue(fixture.hasText("Stale"))
        }
  }

  @Test
  fun toolbarMarkAndActivityIconsRenderAtOneAndTwoTimesDensity() {
    listOf(1f, 2f).forEach { density ->
      ComposeVisualFixture(
              (220 * density).toInt(), (80 * density).toInt(), densityScale = density) {
                Row(
                    Modifier.fillMaxSize(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
                      MiniOrcaMark()
                      listOf(
                              DesktopIcon.Summary,
                              DesktopIcon.Analysis,
                              DesktopIcon.Problems,
                              DesktopIcon.Performance,
                              DesktopIcon.Security,
                              DesktopIcon.Editor)
                          .forEach { icon ->
                            Column(
                                horizontalAlignment =
                                    androidx.compose.ui.Alignment.CenterHorizontally) {
                                  DesktopLineIcon(
                                      icon, "${icon.name} destination", iconSize = 16.dp)
                                  Spacer(Modifier.height(4.dp))
                                  DesktopLineIcon(
                                      icon, "${icon.name} destination at 20dp", iconSize = 20.dp)
                                }
                          }
                    }
              }
          .use { fixture ->
            fixture.render("tokyo-midnight-icons-${density}x")
            assertTrue(fixture.hasDescription("Mini-Orca"))
            listOf("Summary", "Analysis", "Problems", "Performance", "Security", "Editor").forEach {
              assertTrue(fixture.hasDescription("$it destination"))
              assertTrue(fixture.hasDescription("$it destination at 20dp"))
            }
          }
    }
  }

  @Test
  fun productionReferenceMatrixUsesAttachmentSizeAndRepresentativeDensity() {
    listOf(Triple("1x", 1_512, 712), Triple("2x", 3_024, 1_424)).forEach {
        (densityLabel, width, height) ->
      val density = if (densityLabel == "2x") 2f else 1f
      val logicalWidth = width / density
      fun renderReferenceSurface(surface: String, content: @Composable () -> Unit) {
        ComposeVisualFixture(width, height, densityScale = density, content = content).use { fixture
          ->
          fixture.render("final-reference-$surface-$densityLabel")
          when (surface) {
            "summary" -> fixture.assertTextFits("Analysis coverage")
            "analysis" -> {
              fixture.assertTextFits("8 of 12 files finished")
              fixture.assertTextFits("Current: internal/api/user.go")
              fixture.assertTextFits("Pause")
              fixture.assertTextFits("Cancel")
            }
            "source" -> fixture.assertTextFits("Read-only")
            "review" -> {
              fixture.assertTextFits("Read-only")
              assertFalse(fixture.hasEditableText("Read-only composed diff"))
            }
            else -> fixture.assertTextFits("View analysis")
          }
        }
      }

      renderReferenceSurface("summary") { RoundedSummaryVisualFixture(logicalWidth) }
      renderReferenceSurface("analysis") { RoundedAnalysisVisualFixture(logicalWidth) }
      renderReferenceSurface("bugs") { AcceptanceResultPane("bugs", "partial") }
      renderReferenceSurface("performance") { AcceptanceResultPane("performance", "partial") }
      renderReferenceSurface("security") { AcceptanceResultPane("security", "partial") }
      renderReferenceSurface("source") { EditorVisualFixture(logicalWidth) }
      renderReferenceSurface("review") { EditorVisualFixture(logicalWidth, comparison = true) }
    }
  }

  @Test
  fun nativeTerminalContentStaysInsideTheRoundedDockWhenResized() {
    listOf(140f, 220f, 360f).forEach { dockHeight ->
      ComposeVisualFixture(1000, 600) {
            TerminalDock(
                DesktopLayoutState(bottomCollapsed = false, bottomHeight = dockHeight),
                TerminalWorkspaceState(),
                {},
                {},
                TerminalTabActions({}, {}, {}),
                {},
                {},
                { modifier ->
                  Box(modifier.testTag("native-terminal-content").background(EditorCanvas))
                },
                modifier = Modifier.testTag("native-terminal-dock"))
          }
          .use { fixture ->
            fixture.render("terminal-native-inset-$dockHeight")
            val dock = fixture.taggedBounds("native-terminal-dock")
            val content = fixture.taggedBounds("native-terminal-content")
            assertEquals(8f, content.left - dock.left, 1f)
            assertEquals(8f, dock.right - content.right, 1f)
            assertEquals(8f, dock.bottom - content.bottom, 1f)
            assertTrue(content.height > 70f)
          }
    }
  }

  @Test
  fun candidateComparisonFillsTheEditorAcrossWindowAndTextSizes() {
    listOf(1600 to 1000, 1440 to 900, 1024 to 768, 1000 to 760, 999 to 760, 800 to 650, 1280 to 600)
        .forEach { (width, height) ->
          listOf(1f, 1.25f, 1.5f).forEach { scale ->
            ComposeVisualFixture(width, height, scale) {
                  EditorVisualFixture(width.toFloat(), comparison = true)
                }
                .use { fixture ->
                  fixture.render("comparison-frame-$width-$height-$scale")
                  if (width >= 1440) {
                    listOf("Candidate diff", "Read-only", "New function", "Side-by-side", "Unified")
                        .forEach(fixture::assertTextFits)
                  }
                  assertTrue(fixture.hasText("Candidate diff"))
                  assertTrue(fixture.hasText("Read-only"))
                  assertTrue(fixture.hasText("New function"))
                  assertFalse(fixture.hasEditableText("Read-only composed diff"))
                  assertTrue(fixture.hasDescription("Read-only composed diff"))
                  val wide = fixture.hasText("Current")
                  val column = fixture.taggedBounds("diff-Current-column")
                  assertTrue(
                      column.height > if (width >= 999) 180f else -1f,
                      "Comparison must render its canvas at $width/$height/$scale: $column")
                  if (wide) {
                    val candidate = fixture.taggedBounds("diff-Candidate-column")
                    assertEquals(column.bottom, candidate.bottom, 1f)
                    assertEquals(column.height, candidate.height, 1f)
                  }
                }
          }
        }
  }

  @Test
  fun sourceGutterAndBreadcrumbStayAlignedAtLargeText() {
    ComposeVisualFixture(1000, 760, 1.5f) { EditorVisualFixture(1000f) }
        .use { fixture ->
          fixture.render("source-alignment-1000-760-1.5")
          fixture.assertTextFits("Read-only")
          (1..12).forEach { line ->
            val number = fixture.taggedBounds("source-number-$line")
            val code = fixture.taggedBounds("source-code-$line")
            assertEquals(number.top, code.top, 1f)
            assertEquals(30f, code.height, 1f)
          }
          assertFalse(fixture.hasEditableText(withinTag = "source-viewport"))
          assertTrue(fixture.copyTextByDragging("package api").isNotEmpty())
        }
  }

  @Test
  fun declarationEmphasisMatchesGutterAndKeepsFocusAndTextSelectionDistinct() {
    val file =
        ProjectFileInfo(
            path = "internal/worker/main.go",
            contentHash = "range-fixture",
            name = "main.go",
            language = "Go",
            sizeBytes = 100,
            lineCount = 999,
            modifiedAt = "",
            binary = false,
            content =
                "package worker\n\nfunc Run() {\n    work()\n}\n\n// outside declaration\n// last loaded line")
    val exact =
        SymbolInfo(
            "Run",
            "function",
            startLine = 3,
            endLine = 5,
            confidence = "exact",
            atomicTarget = true)
    for (scale in listOf(1f, 1.5f)) {
      var symbol by mutableStateOf(exact)
      var focusedLine by mutableStateOf(4)
      ComposeVisualFixture(800, 650, scale) {
            EditorWorkspace(
                editorChromeUiState(
                    file,
                    symbol,
                    EditorSurface.Source,
                    EditorProgressUiState(EditorProgress.Inspect, ""),
                    null),
                null,
                {},
                {},
                canvas = {
                  SourceEditorPane(
                      resultProjectFixture(),
                      file,
                      listOf(symbol),
                      symbol,
                      focusedLine,
                      emptyList(),
                      {})
                })
          }
          .use { fixture ->
            fixture.render("f23-source-range-focused-$scale")
            (1..8).forEach { line ->
              val gutter = fixture.taggedBounds("source-gutter-$line")
              val code = fixture.taggedBounds("source-code-$line")
              assertEquals(gutter.top, code.top, 1f)
              assertEquals(gutter.height, code.height, 1f)
              assertEquals(20f * scale, code.height, 1f)
              assertEquals(
                  fixture.renderedPixels("source-gutter-$line").first(),
                  fixture.renderedPixels("source-code-$line").first(),
                  "Line $line must share its gutter/code emphasis")
            }
            val ordinary = fixture.renderedPixels("source-code-3").first()
            val focusedDeclaration = fixture.renderedPixels("source-code-4").first()
            assertEquals(SelectionSurface.toArgb(), ordinary)
            assertTrue(
                ordinary != EditorCanvas.toArgb(),
                "Ordinary declaration rows must have visible emphasis")
            assertTrue(focusedDeclaration != ordinary)
            assertTrue(
                fixture.hasDescription(
                    "Line 4, focused location in selected declaration, selectable declaration Run"))
            val beforeSelection = fixture.renderedPixels("source-code-3")
            assertTrue(fixture.copyTextByDragging("func Run() {").isNotEmpty())
            fixture.render("f23-source-range-text-selection-$scale")
            assertFalse(
                beforeSelection.contentEquals(fixture.renderedPixels("source-code-3")),
                "Text selection must paint separately from declaration emphasis")
            focusedLine = 7
            fixture.render("f23-source-range-focused-location-$scale")
            val focusedLocation = fixture.renderedPixels("source-code-7").first()
            assertTrue(focusedLocation != ordinary && focusedLocation != focusedDeclaration)
            assertTrue(fixture.hasDescription("Line 7, focused location"))
            symbol = exact.copy(confidence = "approximate", endLine = Int.MAX_VALUE)
            fixture.render("f23-source-range-approximate-clamped-$scale")
            assertTrue(
                fixture.hasText(
                    "Inspecting declaration: Run · Lines 3–8 · Approximate · read-only"))
            assertEquals(SelectionSurface.toArgb(), fixture.renderedPixels("source-code-8").first())
            assertEquals(
                0,
                fixture.tagCount("source-code-9"),
                "Never fabricate indexed lines beyond loaded content")
            for (invalid in
                listOf(
                    exact.copy(startLine = 0),
                    exact.copy(endLine = 2),
                    exact.copy(startLine = 9, endLine = 999))) {
              symbol = invalid
              focusedLine = 0
              fixture.render("f23-source-invalid-${invalid.startLine}-${invalid.endLine}-$scale")
              assertTrue(fixture.hasText("Inspecting declaration: Run · Line range unavailable"))
              (1..8).forEach { line ->
                assertEquals(
                    EditorCanvas.toArgb(),
                    fixture.renderedPixels("source-code-$line").last(),
                    "Invalid ranges must not paint declaration emphasis outside text selection")
                assertEquals(
                    EditorCanvas.toArgb(), fixture.renderedPixels("source-gutter-$line").first())
              }
              assertFalse(
                  fixture.hasDescription(
                      "Selected declaration Run marker at line ${invalid.startLine}"))
            }
            assertFalse(fixture.hasEditableText(withinTag = "source-viewport"))
          }
    }
  }

  @Test
  fun finalLifecycleMatrixUsesProductionPanesAtLargeText() {
    acceptanceRunStates.forEach { status ->
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(
                    resultProjectFixture(), ProjectAnalysisRunState(run = acceptanceRun(status))),
                AnalysisWorkspaceActions(
                    { _, _ -> },
                    {},
                    {},
                    {},
                    {},
                    refreshStatus = { error("Unexpected status refresh") }))
          }
          .use { fixture ->
            fixture.render("final-progress-$status-800-150")
            val run = acceptanceRun(status)
            val presentation =
                projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
            fixture.assertTextFits(analysisRunTitle(run, presentation), maxLines = 3)
            assertEquals(
                1,
                fixture.taggedTextCount(
                    "analysis-run-content", analysisRunTitle(run, presentation)))
            assertEquals(0, fixture.taggedTextCount("analysis-run-content", presentation.status))
            assertFalse(fixture.hasText(presentation.headline))
            if (status == "completed") {
              assertTrue(fixture.hasText("1 of 1 files finished"))
              assertTrue(fixture.hasText("100%"))
            }
            assertFalse(fixture.hasText("Run details"))
            assertFalse(fixture.hasText("Run limits"))
          }
      listOf("bugs", "performance", "security").forEach { category ->
        ComposeVisualFixture(800, 650, 1.5f) { AcceptanceResultPane(category, status) }
            .use { fixture ->
              fixture.render("final-$category-$status-800-150")
              fixture.assertTextFits("View analysis")
              val page = acceptanceResultPage(category, status)
              if (status == "completed") assertFalse(fixture.hasText("Completed"))
              else
                  fixture.assertTextFits(
                      if (status == "completed_empty") "No results" else page.statusLabel)
              AnalysisResultType.entries.forEach { type ->
                assertFalse(fixture.hasDescription("View ${type.workspace.name} results"))
              }
              assertFalse(fixture.hasText("Start analysis"))
            }
      }
    }
  }

  @Test
  fun analysisRunPanelKeepsActiveAndTerminalMetadataWithoutLifecycleHeadlines() {
    val active =
        analysisRunFixture()
            .copy(
                status = "running",
                elapsedSeconds = 125,
                windowFilesCompleted = 3,
                windowElapsedSeconds = 42,
                createdAt = "2026-09-15T14:00:00Z",
                updatedAt = "2026-09-15T15:30:00Z")
    val stopped = active.copy(status = "paused", windowFilesCompleted = 0, windowElapsedSeconds = 0)
    val withoutTiming =
        active.copy(elapsedSeconds = 0, windowElapsedSeconds = 0, createdAt = "", updatedAt = "")
    val timestamps = "Created · 2026-09-15T14:00:00Z · Updated · 2026-09-15T15:30:00Z"
    listOf(
            active to
                "3 files processed in current window · Reported run time · 125s · Current window · 42s · $timestamps",
            stopped to "Reported run time · 125s · $timestamps",
            stopped.copy(status = "interrupted") to "Reported run time · 125s · $timestamps",
            stopped.copy(status = "completed") to "Reported run time · 125s · $timestamps",
            withoutTiming to
                "3 files processed in current window · Reported run time · unavailable")
        .forEach { (run, metadata) ->
          val presentation =
              projectRunPresentation(resultProjectFixture(), ProjectAnalysisRunState(run = run))
          ComposeVisualFixture(1440, 900) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(), ProjectAnalysisRunState(run = run)),
                    AnalysisWorkspaceActions(
                        { _, _ -> },
                        {},
                        {},
                        {},
                        {},
                        refreshStatus = { error("Unexpected status refresh") }))
              }
              .use { fixture ->
                fixture.render("analysis-run-metadata-${run.status}")
                fixture.assertTextFits(analysisRunTitle(run, presentation))
                fixture.assertTextFits(metadata, maxLines = 3)
                assertFalse(fixture.hasText(presentation.headline))
              }
        }
  }

  @Test
  fun savedRunHistoryIsBoundedReadOnlyAndKeyedByFullIdentity() {
    val project = resultProjectFixture()
    val current =
        analysisRunFixture()
            .copy(
                status = "completed",
                identity = analysisRunFixture().identity.copy(id = "latest", generation = "new"),
                elapsedSeconds = 73,
                createdAt = "2026-09-15T14:00:00Z",
                updatedAt = "2026-09-15T15:00:00Z")
    val reason = "Historical failure: " + "sensitive context/".repeat(350)
    val planned =
        AnalysisPlannedFile(
            "old.go",
            "oldhash",
            "Go",
            20,
            listOf(AnalysisStagePlan("semantic", true, false, maxModelRequests = 0)))
    val reported =
        AnalysisRunFile(
            "old.go",
            "oldhash",
            "Go",
            listOf(AnalysisStageProgress("semantic", "failed", 2, false, reason = reason)))
    val previous =
        analysisRunFixture()
            .copy(
                identity = analysisRunFixture().identity.copy(id = "older", generation = "old"),
                status = "failed",
                plan =
                    analysisPreviewFixture()
                        .copy(scope = "selected files", files = listOf(planned)),
                files = listOf(reported),
                elapsedSeconds = 125,
                createdAt = "2026-09-14T12:00:00Z",
                updatedAt = "2026-09-14T13:00:00Z")
    for ((width, height, scale) in listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f))) {
      var snapshot by mutableStateOf(ProjectAnalysisRunState(run = current))
      var calls = 0
      ComposeVisualFixture(width, height, scale) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(project, snapshot),
                AnalysisWorkspaceActions(
                    { _, _ -> calls++ },
                    { calls++ },
                    { calls++ },
                    { calls++ },
                    { calls++ },
                    { calls++ },
                    { calls++ },
                    refreshStatus = { error("Unexpected status refresh") }))
          }
          .use { fixture ->
            fixture.render("history-latest-$width")
            fixture.revealText("Latest saved run", "analysis-page")
            assertTrue(fixture.hasText(savedRunIdentityLabel(current)))
            assertTrue(
                fixture.hasText(
                    "Older run details unavailable in this session · only the latest saved run is restored after restart."))
            assertFalse(fixture.hasText("No stage failures reported in this saved run."))
            snapshot = snapshot.copy(previousRun = previous)
            fixture.render("history-collapsed-$width")
            fixture.revealText("Previous observed run · Failed", "analysis-page")
            assertTrue(fixture.hasDescription("Expand Previous observed run · Failed"))
            assertTrue(fixture.requestDescriptionFocus("Expand Previous observed run · Failed"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render("history-expanded-$width")
            assertTrue(fixture.hasDescription("Collapse Previous observed run · Failed"))
            assertTrue(fixture.hasText(savedRunIdentityLabel(previous)))
            assertTrue(fixture.hasText("Captured scope · selected files · 1 planned file"))
            assertTrue(fixture.hasText("Lifecycle · Failed"))
            assertTrue(fixture.hasText("Reported run time · 125s"))
            assertTrue(fixture.hasText("Created · 2026-09-14T12:00:00Z"))
            assertTrue(fixture.hasText("Updated · 2026-09-14T13:00:00Z"))
            assertTrue(fixture.hasText("old.go · Code analysis · 2 attempts reported"))
            assertTrue(fixture.hasText("… output truncated"))
            assertTrue(
                fixture.taggedBounds("analysis-previous-run-details").height <= 280f * scale + 2f)
            snapshot = snapshot.copy(run = current.copy(updatedAt = "2026-09-15T16:00:00Z"))
            fixture.render("history-poll-$width")
            assertTrue(fixture.hasDescription("Collapse Previous observed run · Failed"))
            assertTrue(fixture.isFocusedControl("Collapse Previous observed run · Failed"))
            snapshot =
                snapshot.copy(
                    run = current.copy(identity = current.identity.copy(generation = "next")))
            fixture.render("history-replaced-$width")
            fixture.revealText("Previous observed run · Failed", "analysis-page")
            assertTrue(fixture.hasDescription("Expand Previous observed run · Failed"))
            snapshot =
                snapshot.copy(
                    previousRun =
                        previous.copy(
                            identity = previous.identity.copy(projectRevision = "earlier")))
            fixture.render("history-outdated-$width")
            fixture.clickDescription("Expand Previous observed run · Failed")
            fixture.render("history-outdated-expanded-$width")
            assertTrue(
                fixture.hasText(
                    "Outdated · previous run belongs to another project revision; not current evidence."))
            assertEquals(0, calls)
          }
    }
  }

  @Test
  fun incompletePreviousRunShowsKnownFailuresAndMissingRunDiagnostic() {
    val project = resultProjectFixture()
    val current = analysisRunFixture().copy(status = "completed")
    val plan =
        analysisPreviewFixture()
            .copy(
                files =
                    listOf("known.go", "missing.go").map { path ->
                      AnalysisPlannedFile(
                          path,
                          "base",
                          "Go",
                          20,
                          listOf(AnalysisStagePlan("semantic", true, false, maxModelRequests = 0)))
                    })
    val failedFile =
        AnalysisRunFile(
            "known.go",
            "base",
            "Go",
            listOf(AnalysisStageProgress("semantic", "failed", 2, false, reason = "Known failure")))
    for (status in listOf("failed", "unavailable")) {
      val previous =
          analysisRunFixture()
              .copy(
                  identity = analysisRunFixture().identity.copy(id = "older", generation = status),
                  status = status,
                  reason = "",
                  plan = plan,
                  files = listOf(failedFile))
      val history = projectRunPresentation(project, ProjectAnalysisRunState(run = previous))
      assertEquals(RunProgressAvailability.Incomplete, history.progressAvailability)
      assertEquals(1, history.failures.size)
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(
                    project, ProjectAnalysisRunState(run = current, previousRun = previous)),
                AnalysisWorkspaceActions(
                    { _, _ -> },
                    {},
                    {},
                    {},
                    {},
                    refreshStatus = { error("Unexpected status refresh") }))
          }
          .use { fixture ->
            fixture.render("history-incomplete-$status")
            fixture.revealText(
                "Previous observed run · ${analysisStatusLabel(status)}", "analysis-page")
            fixture.clickDescription(
                "Expand Previous observed run · ${analysisStatusLabel(status)}")
            fixture.render("history-incomplete-expanded-$status")
            assertTrue(fixture.hasText("No diagnostic was supplied for this run."))
            assertTrue(
                fixture.hasText("Stage failure record incomplete · only validated failures shown."))
            assertTrue(fixture.hasText("known.go · Code analysis · 2 attempts reported"))
            assertTrue(fixture.hasText("Known failure"))
            assertFalse(fixture.hasText("No stage failures reported in this saved run."))
            assertFalse(fixture.hasText("missing.go · Code analysis · 2 attempts reported"))
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
          val navigations = mutableListOf<Workspace>()
          val run =
              analysisRunFixture()
                  .copy(
                      status = "running",
                      sections =
                          analysisRunFixture().sections.map {
                            it.copy(
                                status = "running",
                                coverage = AnalysisRunCoverage(total = 1, running = 1),
                                findingCount = null)
                          },
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
                    AnalysisWorkspaceActions(
                        { _, _ -> },
                        {},
                        {},
                        {},
                        navigations::add,
                        refreshStatus = { error("Unexpected status refresh") }))
              }
              .use { fixture ->
                fixture.render("analysis-progress-$width-$scale")
                listOf("Pause", "Cancel").forEach(fixture::assertTextFits)
                assertFalse(fixture.hasText("Prepare fix"))
                assertFalse(fixture.hasText("Open results"))
                assertFalse(fixture.hasText("findings"))
                fixture.assertCategoryBoxesFit()
                AnalysisResultType.entries.forEach { type ->
                  fixture.clickVisibleDescription("View ${type.workspace.name} results")
                }
                assertEquals(AnalysisResultType.entries.map { it.workspace }, navigations)
              }
        }
  }

  @Test
  fun summaryPairsOnlyAtReadableLocalWidthAndKeepsOptionalNarrativeFullWidth() {
    val project = visualFixtureProject
    val overview =
        visualFixtureOverview.copy(
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh",
                    purpose = "Long project interpretation. ".repeat(40),
                    architecture = "Architecture description. ".repeat(30),
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "The mechanism remains readable. ".repeat(12),
                            whyItMattersHere = "Local context.",
                            tradeoffOrFailureMode = "A longer trade-off.")))
    listOf(
            Triple(1160, 1f, true),
            Triple(1100, 1f, false),
            Triple(920, 1f, false),
            Triple(880, 1f, false),
            Triple(1440, 1.5f, false),
            Triple(800, 1.5f, false))
        .forEach { (width, scale, pairResults) ->
          ComposeVisualFixture(width, 2400, scale) { ProjectSummaryPane(overview, project, {}) }
              .use { fixture ->
                fixture.render()
                val coverage = fixture.taggedBounds("summary-coverage-column")
                val results = fixture.taggedBounds("summary-results")
                if (pairResults)
                    assertTrue(
                        coverage.right <= results.left,
                        "paired results at $width/$scale: $coverage / $results")
                else
                    assertTrue(
                        coverage.bottom <= results.top,
                        "stacked results at $width/$scale: $coverage / $results")
                fixture.revealText("Engineering insight")
                val architecture = fixture.taggedBounds("summary-architecture")
                val insight = fixture.taggedBounds("summary-insight")
                if (width - 48 >= 840 * scale)
                    assertTrue(
                        architecture.right <= insight.left,
                        "paired narrative at $width/$scale: $architecture / $insight")
                else
                    assertTrue(
                        architecture.bottom <= insight.top,
                        "stacked narrative at $width/$scale: $architecture / $insight")
                fixture.revealText("Open Editor")
                assertTrue(fixture.hasText("Change lifecycle"))
              }
        }
    val insightOnly = overview.copy(analysis = overview.analysis.copy(architecture = ""))
    ComposeVisualFixture(1440, 900) { ProjectSummaryPane(insightOnly, project, {}) }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Engineering insight")
          assertEquals(0, fixture.tagCount("summary-architecture"))
          val row = fixture.taggedBounds("summary-narrative-row")
          val insight = fixture.taggedBounds("summary-insight")
          assertEquals(row.width, insight.width, 1f)
        }
    val architectureOnly =
        overview.copy(analysis = overview.analysis.copy(engineeringInsight = null))
    ComposeVisualFixture(1440, 900) { ProjectSummaryPane(architectureOnly, project, {}) }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Architecture")
          assertEquals(0, fixture.tagCount("summary-insight"))
          assertEquals(
              fixture.taggedBounds("summary-narrative-row").width,
              fixture.taggedBounds("summary-architecture").width,
              1f)
        }
  }

  @Test
  fun f08ProductionSummaryMatrixKeepsIdentityActionsAndLowerContentReachable() {
    val longName = "go-shop-accounting-reconciliation-and-order-processing-service-2026"
    val project = visualFixtureProject.copy(name = longName)
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    purpose = "A local service for orders and reconciliation. ".repeat(6),
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate at the boundary.",
                            whyItMattersHere = "Orders must be consistent.",
                            tradeoffOrFailureMode = "Validation can reject legacy input.")))
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
          val label = "f08-summary-$width-$height-$scale-${density}x"
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    ProjectSummaryPane(
                        overview,
                        project,
                        {},
                        analysisActions =
                            AnalysisWorkspaceActions(
                                { _, _ -> },
                                {},
                                {},
                                {},
                                {},
                                refreshStatus = { error("Unexpected status refresh") }))
                  }
              .use { fixture ->
                fixture.render("$label-introduction")
                fixture.assertTextFits(longName, 4)
                fixture.assertTextFits("Go project")
                fixture.assertTextFits("Start analysis")
                fixture.revealText("Analysis coverage")
                fixture.render("$label-coverage")
                fixture.revealText("Engineering insight")
                fixture.render("$label-narrative")
                fixture.revealText("Open Editor")
                fixture.assertTextFits("Open Editor")
                fixture.render("$label-lifecycle")
                assertTrue(fixture.hasText("Change lifecycle"), label)
              }
        }
      }
    }
  }

  @Test
  fun f08ProductionSummaryStateCapturesKeepDiagnosticsAndDisclosuresAvailable() {
    val project = visualFixtureProject
    val base = visualFixtureOverview
    val states: List<Pair<String, ProjectOverview?>> =
        listOf(
            "missing" to base.copy(analysis = base.analysis.copy(status = "missing", purpose = "")),
            "stale" to base.copy(analysis = base.analysis.copy(status = "stale")),
            "failed" to
                base.copy(
                    analysis =
                        base.analysis.copy(
                            status = "failed", purpose = "", failure = "Provider unavailable")),
            "unknown" to null)
    for ((name, overview) in states) {
      ComposeVisualFixture(800, 650, 1.5f) { ProjectSummaryPane(overview, project, {}) }
          .use { fixture ->
            fixture.render("f08-summary-$name-800-650-1.5")
            fixture.assertTextFits("Go project")
            fixture.revealText("Analysis coverage")
            fixture.render("f08-summary-$name-coverage-800-650-1.5")
            fixture.revealText("Open Editor")
            fixture.render("f08-summary-$name-lifecycle-800-650-1.5")
            if (name == "failed") {
              fixture.revealText("Project description: failed · Provider unavailable")
              fixture.assertTextFits("Project description: failed · Provider unavailable", 2)
            }
            if (name == "unknown") {
              fixture.revealText(
                  "Overall findings · — tool-reported issues · — AI suggestions", "summary-scroll")
              assertTrue(
                  fixture.hasText("Overall findings · — tool-reported issues · — AI suggestions"))
            }
          }
    }
    val insightOverview =
        base.copy(
            analysis =
                base.analysis.copy(
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate first.",
                            whyItMattersHere = "Keep records consistent.",
                            tradeoffOrFailureMode = "Legacy input may fail.")))
    ComposeVisualFixture(1440, 900) { ProjectSummaryPane(insightOverview, project, {}) }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Engineering insight")
          fixture.render("f08-summary-disclosures-collapsed-1440")
          fixture.clickDescription("Expand Architecture diagram")
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render("f08-summary-disclosures-expanded-1440")
          assertTrue(fixture.hasText("Legacy input may fail."))
        }
  }

  @Test
  fun summaryPreviewEntryAndFeedbackWrapAtCompactTextScaleWithoutHidingCoverage() {
    val project = visualFixtureProject
    listOf(1440 to 1f, 800 to 1.5f).forEach { (width, scale) ->
      var state by mutableStateOf(ProjectAnalysisRunState())
      var previews = 0
      val actions =
          AnalysisWorkspaceActions(
              { _, _ -> previews++ },
              {},
              {},
              {},
              {},
              refreshStatus = { error("Unexpected status refresh") })
      ComposeVisualFixture(width, 650, scale) {
            ProjectSummaryPane(
                visualFixtureOverview,
                project,
                {},
                analysisState = state,
                analysisActions = actions)
          }
          .use { fixture ->
            fixture.render("summary-preview-ready-$width-$scale")
            fixture.assertTextFits("Start analysis")
            assertTrue(fixture.requestFocus("Start analysis"))
            fixture.render("summary-preview-focused-$width-$scale")
            assertTrue(fixture.isFocused("Start analysis"))
            fixture.assertTextAbove("Start analysis", "Analysis coverage")
            fixture.clickText("Start analysis")
            assertEquals(1, previews)
            state = state.copy(action = "previewing")
            fixture.render("summary-preview-pending-$width-$scale")
            fixture.assertTextFits("Analysis: Previewing…")
            fixture.assertTextAbove("Start analysis", "Analysis: Previewing…")
            assertTrue(fixture.isDisabled("Start analysis"))
            state = state.copy(action = "", error = "Preview failed: connection unavailable.")
            fixture.render("summary-preview-failed-$width-$scale")
            fixture.assertTextFits("Preview failed: connection unavailable.")
            fixture.assertTextAbove("Start analysis", "Preview failed: connection unavailable.")
            fixture.assertTextFits("Start analysis")
            assertTrue(fixture.hasText("View analysis"))
          }
    }
  }

  @Test
  fun activeSummaryReadsIntroductionBeforeRunAndTabsFromRunToCoverage() {
    val project = visualFixtureProject
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                identity =
                    analysisRunFixture()
                        .identity
                        .copy(
                            projectId = project.projectId,
                            projectRevision = project.projectRevision),
                files = emptyList())
    ComposeVisualFixture(1440, 1100) {
          ProjectSummaryPane(
              visualFixtureOverview,
              project,
              {},
              run = run,
              analysisActions =
                  AnalysisWorkspaceActions(
                      { _, _ -> },
                      {},
                      {},
                      {},
                      {},
                      refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render("f08-summary-active-introduction-first")
          val introduction = fixture.taggedBounds("summary-introduction")
          val strip = fixture.taggedBounds("summary-analysis-run-strip")
          val coverage = fixture.taggedBounds("summary-coverage-results")
          assertTrue(introduction.bottom < strip.top, "Introduction must precede active run")
          assertTrue(strip.bottom < coverage.top, "Active run must precede coverage")
          assertTrue(fixture.requestFocus("Pause"))
          fixture.render()
          assertTrue(fixture.pressKey(Key.Tab))
          fixture.render()
          assertTrue(fixture.isFocused("Cancel"), "Run controls must tab in reading order")
          assertTrue(fixture.pressKey(Key.Tab))
          fixture.render()
          assertTrue(fixture.isTaggedNodeFocused("summary-analysis-status"))
          assertTrue(fixture.pressKey(Key.Tab))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Up to date, 16 files"))
          repeat(3) {
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
          }
          assertTrue(fixture.isFocused("View analysis"), "Coverage navigation follows the legend")
        }
  }

  @Test
  fun sharedRunStripWrapsProgressAndUsesCurrentLifecycleControlsInSummary() {
    val paths = listOf("internal/transport/main.go", "internal/storage/repository.go")
    val initialRun =
        analysisRunFixture()
            .copy(
                status = "running",
                identity =
                    analysisRunFixture()
                        .identity
                        .copy(
                            projectId = visualFixtureProject.projectId,
                            projectRevision = visualFixtureProject.projectRevision),
                plan =
                    analysisPreviewFixture()
                        .copy(
                            identity =
                                analysisPreviewFixture()
                                    .identity
                                    .copy(
                                        projectId = visualFixtureProject.projectId,
                                        projectRevision = visualFixtureProject.projectRevision),
                            files =
                                paths.mapIndexed { index, path ->
                                  AnalysisPlannedFile(
                                      path,
                                      "hash-$index",
                                      "Go",
                                      20,
                                      listOf(
                                          AnalysisStagePlan(
                                              "semantic", true, false, maxModelRequests = 0)))
                                }),
                files =
                    paths.mapIndexed { index, path ->
                      AnalysisRunFile(
                          path,
                          "hash-$index",
                          "Go",
                          listOf(AnalysisStageProgress("semantic", "running", 1, false)))
                    })
    listOf(1440 to 1f, 800 to 1.5f).forEach { (width, scale) ->
      var state by mutableStateOf(ProjectAnalysisRunState(run = initialRun))
      var pauses = 0
      var resumes = 0
      var cancels = 0
      val actions =
          AnalysisWorkspaceActions(
              { _, _ -> error("Summary must not start or retry analysis") },
              { pauses++ },
              { resumes++ },
              { cancels++ },
              {},
              refreshStatus = { error("Unexpected status refresh") })
      ComposeVisualFixture(width, 650, scale) {
            ProjectSummaryPane(
                visualFixtureOverview,
                visualFixtureProject,
                {},
                run = state.run,
                analysisState = state,
                analysisActions = actions)
          }
          .use { fixture ->
            fixture.render("summary-run-strip-$width-$scale")
            fixture.assertTextFits("Running")
            fixture.assertTextFits("0 of 2 files finished")
            fixture.assertTextFits("Pause")
            fixture.assertTextFits("Cancel")
            assertFalse(fixture.hasText("Start analysis"))
            assertFalse(fixture.hasText("Analyze stale & failed"))
            assertTrue(fixture.taggedBounds("analysis-run-progress-track").width > 0f)
            assertEquals(0, fixture.tagCount("analysis-run-content"))
            assertEquals(0, fixture.tagCount("analysis-run-controls"))
            fixture.assertTextFits("Running")
            fixture.assertTextFits("0 of 2 files finished")
            fixture.assertTextFits("Current: ${paths.first()}", maxLines = 2)
            fixture.assertTextFits("Pause")
            fixture.clickText("Pause")
            fixture.clickText("Cancel")
            assertEquals(1, pauses)
            assertEquals(1, cancels)
            fixture.clickDescription("Show active files")
            fixture.render("summary-run-strip-expanded-$width-$scale")
            fixture.assertTextFits("Current: ${paths.last()}", maxLines = 2)

            state = state.copy(action = "pausing")
            fixture.render("summary-run-strip-disabled-$width-$scale")
            assertTrue(fixture.isDisabled("Pause"))

            state = state.copy(run = initialRun.copy(status = "paused"), action = "")
            fixture.render("summary-run-strip-paused-$width-$scale")
            fixture.assertTextFits("Paused")
            fixture.assertTextFits("Resume → fresh preview")
            fixture.clickText("Resume → fresh preview")
            assertEquals(1, resumes)
          }
    }
  }

  @Test
  fun unknownFileProgressKeepsOneTruthfulTrackAcrossActiveAndPausedRuns() {
    listOf(
            Triple("running", "Pause", "Resume"),
            Triple("paused", "Resume → fresh preview", "Pause"),
        )
        .forEach { (status, expectedAction, absentAction) ->
          val run =
              analysisRunFixture()
                  .copy(
                      status = status,
                      files = emptyList(),
                      sections = analysisRunFixture().sections.map { it.copy(status = status) },
                  )
          ComposeVisualFixture(800, 650, 1.5f) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(), ProjectAnalysisRunState(run = run)),
                    AnalysisWorkspaceActions(
                        { _, _ -> },
                        {},
                        {},
                        {},
                        {},
                        refreshStatus = { error("Unexpected status refresh") }),
                )
              }
              .use { fixture ->
                fixture.render("analysis-progress-unknown-$status-800-1.5")
                fixture.assertTextFits(analysisStatusLabel(status))
                fixture.assertTextFits(
                    "File progress incomplete · captured records missing or inconsistent",
                    maxLines = 2)
                fixture.assertTextFits(expectedAction)
                fixture.assertTextFits("Cancel")
                assertFalse(fixture.hasText(absentAction))
                assertFalse(fixture.hasText("0 of 0 files finished"))
                assertEquals(1, fixture.tagCount("analysis-run-progress-track"))
                assertTrue(
                    fixture.hasDescription(
                        "File progress incomplete · captured records missing or inconsistent"))
              }
        }
  }

  @Test
  fun categoryBoxesExposeNamesAndSingleKeyboardActions() {
    val navigations = mutableListOf<Workspace>()
    ComposeVisualFixture(480, 650, 1.5f) {
          Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            AnalysisResultType.entries.forEach { type ->
              AnalysisCategoryBox(
                  type = type,
                  count = null,
                  status = null,
                  tint = SecondaryText,
                  onClick = { navigations += type.workspace })
            }
            ChromeButton(onClick = {}) { Text("After categories") }
          }
        }
        .use { fixture ->
          fixture.render()
          fixture.assertCategoryBoxesFit()
          assertEquals(3, fixture.textCount("—"))
          assertEquals(3, fixture.textCount("Not analyzed"))
          AnalysisResultType.entries.forEachIndexed { index, type ->
            val name = type.workspace.name
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render("category-focus-${type.category}-480-1.5")
            assertTrue(fixture.isDescriptionFocused("View $name results"))
            fixture.assertTextFits(name)
            val icon = fixture.taggedBounds("analysis-category-icon-${type.category}")
            val content = fixture.taggedBounds("analysis-category-content-${type.category}")
            assertTrue(icon.right < content.left, "Category icon must lead its content")
            fixture.assertColorVisible(FocusAccent)
            assertEquals(index * 2, navigations.size, "Focus must only disclose the name")
            assertTrue(fixture.pressKey(Key.Enter))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertEquals(List(2) { type.workspace }, navigations.takeLast(2))
            assertEquals((index + 1) * 2, navigations.size)
          }
          assertTrue(fixture.pressKey(Key.Tab))
          fixture.render()
          assertTrue(
              fixture.isFocused("After categories"), "Icons and tooltips must not add tab stops")
          assertTrue(fixture.hasText("Security"))
          fixture.hoverVisibleDescription("View Performance results")
          fixture.render("category-hover-480-1.5")
          fixture.assertTextFits("Performance")
          fixture.assertColorVisible(analysisCategoryBoxColors().hoveredBackground)
          assertEquals(6, navigations.size, "Hover must only disclose the name")
        }
  }

  @Test
  fun analysisLifecycleAndOperationalErrorsStayExplicit() {
    listOf(
            "paused" to "Resume → fresh preview",
            "interrupted" to "Resume → fresh preview",
            "stale" to "Start analysis",
            "failed" to "Start analysis",
            "canceled" to "Start new analysis")
        .forEach { (status, control) ->
          ComposeVisualFixture(800, 650, 1.5f) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(),
                        ProjectAnalysisRunState(
                            run = analysisRunFixture().copy(status = status),
                            error =
                                "Could not reach the daemon. Retry when the local service is available. Completed results remain available in their sections; no analysis request was retried.")),
                    AnalysisWorkspaceActions(
                        { _, _ -> },
                        {},
                        {},
                        {},
                        {},
                        refreshStatus = { error("Unexpected status refresh") }))
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
                  {},
                  refreshStatus = { error("Unexpected status refresh") }))
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
  fun stageDisclosureKeepsKeyboardFocusThroughPollingAndResetsForNewRun() {
    val path = "src/" + "日本語-long-folder/".repeat(8) + "main.go"
    val original =
        analysisRunFixture()
            .copy(
                status = "interrupted",
                reason = "",
                plan =
                    analysisRunFixture()
                        .plan
                        .copy(
                            files =
                                listOf(
                                    AnalysisPlannedFile(
                                        path,
                                        "base",
                                        "Go",
                                        20,
                                        listOf(
                                            AnalysisStagePlan(
                                                "semantic", true, false, maxModelRequests = 0))))),
                files =
                    listOf(
                        AnalysisRunFile(
                            path,
                            "base",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "interrupted", 2, true)))))
    var run by mutableStateOf(original)
    var dispatches = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(), ProjectAnalysisRunState(run = run)),
              AnalysisWorkspaceActions(
                  { _, _ -> dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render("stage-collapsed")
          assertTrue(fixture.hasText("Attention · 1 interrupted"))
          assertFalse(fixture.hasText(path))
          fixture.revealText("Code analysis · 0/1 finished · 1 interrupted", "analysis-page")
          val toggle = "Expand Code analysis · 0/1 finished · 1 interrupted"
          assertTrue(fixture.requestDescriptionFocus(toggle))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render("stage-expanded")
          assertTrue(
              fixture.isDescriptionFocused("Collapse Code analysis · 0/1 finished · 1 interrupted"))
          fixture.revealText("No diagnostic was supplied for this stage.", "analysis-page")
          assertTrue(fixture.hasText(path))
          assertTrue(fixture.hasText("Interrupted · Attempts reported: 2 · Reuse: reported reused"))
          fixture.revealText("Run diagnostic", "analysis-page")
          fixture.clickText("Run diagnostic")
          fixture.render()
          fixture.revealText("No diagnostic was supplied for this run.", "analysis-page")
          assertTrue(fixture.hasText("No diagnostic was supplied for this run."))
          run = run.copy(updatedAt = "2026-09-28T10:00:00Z")
          fixture.render("stage-same-run-poll")
          assertTrue(fixture.hasText(path))
          assertTrue(fixture.hasText("No diagnostic was supplied for this run."))
          run =
              run.copy(
                  identity = run.identity.copy(id = "new-run"),
                  plan = run.plan.copy(identity = run.identity.copy(id = "new-run").queue()))
          fixture.render("stage-new-run")
          assertFalse(fixture.hasText(path))
          assertTrue(fixture.hasText("Stop reason · No stop reason was supplied for this run."))
          assertEquals(0, dispatches)
        }
  }

  @Test
  fun analysisPageKeepsAttentionNearStatusAndStageDiagnosticsDisclosed() {
    val failure =
        "The semantic scanner could not read cmd/miniorca/main.go. Retry analysis after restoring the file."
    val run =
        analysisRunFixture()
            .copy(
                status = "failed",
                plan =
                    analysisPreviewFixture()
                        .copy(
                            files =
                                listOf(
                                    AnalysisPlannedFile(
                                        "cmd/miniorca/main.go",
                                        "base",
                                        "Go",
                                        20,
                                        listOf(
                                            AnalysisStagePlan(
                                                "semantic", true, false, maxModelRequests = 0))))),
                files =
                    listOf(
                        AnalysisRunFile(
                            "cmd/miniorca/main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress(
                                    "semantic", "failed", 2, false, reason = failure)))))
    val state =
        AnalysisWorkspacePaneState(
            resultProjectFixture(),
            ProjectAnalysisRunState(
                run = run, fileSelection = AnalysisSelectionState(selectionFixture())))
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> }, {}, {}, {}, {}, refreshStatus = { error("Unexpected status refresh") })

    ComposeVisualFixture(1_600, 1_600) { AnalysisWorkspacePane(state, actions) }
        .use { fixture ->
          fixture.render("analysis-hierarchy-stage-failure")
          assertEquals(1, fixture.textCount("Analysis"))
          fixture.assertTextAbove("Analysis", "Bugs")
          fixture.assertTextAbove("Bugs", "Files")
          fixture.assertTextAbove("Attention · 1 failed", "Files")
          assertFalse(fixture.hasText(failure))
          fixture.clickDescription("Expand Code analysis · 1/1 finished · 1 failed")
          fixture.render("analysis-hierarchy-stage-expanded")
          fixture.assertTextFits(failure)
          assertTrue(fixture.taggedBounds("analysis-stage-details-semantic").height > 0f)
        }

    ComposeVisualFixture(800, 650, 1.5f) { AnalysisWorkspacePane(state, actions) }
        .use { fixture ->
          fixture.render("analysis-stage-failure-reachable-800-150")
          fixture.revealText("Code analysis · 1/1 finished · 1 failed", "analysis-page")
          assertTrue(
              fixture.requestDescriptionFocus("Expand Code analysis · 1/1 finished · 1 failed"))
          fixture.render()
          assertTrue(fixture.isDescriptionFocused("Expand Code analysis · 1/1 finished · 1 failed"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          fixture.revealText(failure, "analysis-page")
          fixture.assertTextWrapsWithoutClipping(failure)
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
            val navigation = mutableListOf<Workspace>()
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
                FindingActions({ externalActions++ }, { _, _ -> externalActions++ }, {})
            ComposeVisualFixture(width, height, scale) {
                  when (category) {
                    "performance" ->
                        PerformanceWorkspacePane(
                            PerformanceWorkspacePaneState(page, resultIndexFixture()),
                            PerformanceWorkspaceActions(
                                { externalActions++ },
                                { navigation += Workspace.Analysis },
                                findingActions,
                                openSource = { externalActions++ }))
                    "security" ->
                        SecurityWorkspacePane(
                            SecurityWorkspacePaneState(page, resultIndexFixture()),
                            SecurityWorkspaceActions(
                                { _, _ -> externalActions++ },
                                { _, _ -> externalActions++ },
                                { navigation += Workspace.Analysis },
                                findingActions))
                    else ->
                        BugsWorkspacePane(
                            BugsWorkspacePaneState(page.semantic, null, false, page),
                            BugsWorkspaceActions(
                                findingActions,
                                { externalActions++ },
                                { externalActions++ },
                                { navigation += Workspace.Analysis }))
                  }
                }
                .use { fixture ->
                  fixture.render()
                  assertTrue(fixture.hasText("Loading results…"))
                  assertEquals(1, fixture.textCount("Loading results…"))
                  assertFalse(fixture.hasDescription("Inspect $title"))
                  page = original
                  fixture.render("results-$category-$width-$scale")
                  fixture.assertTextFits("View analysis")
                  fixture.assertTextFits(title)
                  assertFalse(fixture.hasDescription("View Bugs results"))
                  assertFalse(fixture.hasDescription("View Performance results"))
                  assertFalse(fixture.hasDescription("View Security results"))
                  assertFalse(fixture.hasText("AI SUGGESTIONS"))
                  assertFalse(fixture.hasText("AI suspicion"))
                  assertFalse(fixture.hasText("Performance review"))
                  if (category == "performance")
                      assertTrue(fixture.hasText("Not measured · explicit local execution"))
                  else assertFalse(fixture.hasText("Not measured"))
                  assertTrue(fixture.hasDescription("Filter results"))
                  assertTrue(
                      fixture.hasText(if (category == "performance") "Impact" else "Severity"))
                  assertFalse(fixture.hasText("Start analysis"))
                  assertFalse(fixture.hasText("Review"))
                  fixture.clickDescription("Inspect $title")
                  fixture.render("results-$category-detail-$width-$scale")
                  assertFalse(fixture.hasText("Back to results"))
                  assertFalse(fixture.hasText("Clear selection"))
                  assertTrue(fixture.hasText("Prepare fix"))
                  if (category == "performance") {
                    assertTrue(fixture.hasText("Model suggestion"))
                    assertTrue(
                        fixture.hasText(
                            "Unmeasured recommendation. Benchmark the affected workload before claiming an improvement."))
                  }
                  if (category == "bugs") {
                    assertTrue(fixture.hasText("Open source"))
                    assertTrue(fixture.isDisabled("Open source"))
                  } else assertTrue(fixture.hasText("Open source"))
                  assertEquals(0, externalActions)
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
                  fixture.clickText("View analysis")
                  assertEquals(listOf(Workspace.Analysis), navigation)
                  assertEquals(0, externalActions)
                  fixture.clickVisibleDescription("Inspect $title")
                  fixture.render()
                  if (category != "bugs") {
                    fixture.clickText("Prepare fix")
                    assertEquals(1, externalActions)
                  }
                }
          }
        }
  }

  @Test
  fun f19SecurityProductionMatrixKeepsEvidenceActionsAndScrollRegionsReachable() {
    val original = securityPageFixture()
    val path = "internal/" + "日本語/very-long-directory/".repeat(8) + "boundary.go"
    val reports = requireNotNull(original.results)
    val rule =
        reports.security.first().let { report ->
          report.copy(
              path = path,
              findings =
                  report.findings.map { finding ->
                    finding.copy(
                        anchor = finding.anchor.copy(path = path),
                        observedCondition = "Source pattern at the trust boundary. ".repeat(18))
                  })
        }
    val ai =
        reports.security
            .last()
            .copy(
                findings =
                    reports.security.last().findings.map {
                      it.copy(observedCondition = "Advisory hypothesis, not verified. ".repeat(18))
                    })
    val unknown =
        rule.copy(
            source = "other",
            findings =
                rule.findings.map {
                  it.copy(id = "unknown-provenance", title = "Unknown evidence type")
                })
    val page =
        original.copy(
            section =
                original.section.copy(results = reports.copy(security = listOf(rule, ai, unknown))))
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        val label = "f19-security-$width-$height-$scale-${density}x"
        val browser = newResultBrowserState(page)
        var privileged = 0
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  SecurityWorkspacePane(
                      SecurityWorkspacePaneState(page, resultIndexFixture(), browser),
                      SecurityWorkspaceActions(
                          { _, _ -> privileged++ },
                          { _, _ -> privileged++ },
                          { privileged++ },
                          FindingActions(
                              { privileged++ }, { _, _ -> privileged++ }, { privileged++ }),
                          reviewSecurityIntent = { privileged++ }))
                }
            .use { fixture ->
              fixture.render("$label-loaded")
              fixture.assertTextFits("View analysis")
              fixture.revealText("Review Security intent", "result-overview")
              fixture.assertTextFits("Review Security intent")
              browser.choose(
                  securityResults(page).single { it.finding.id == "unknown-provenance" }.rowKey)
              fixture.render("$label-unknown")
              assertTrue(
                  fixture.hasText(
                      "Evidence type was unavailable. Do not treat this finding as verified."))
              browser.choose(
                  securityResults(page).single { it.report.source == "deterministic" }.rowKey)
              fixture.render("$label-rule")
              assertTrue(fixture.hasText("Source rule"))
              assertTrue(fixture.hasText(path + ":4"))
              assertTrue(
                  fixture.hasText(
                      "A source rule match identifies a pattern; it does not confirm a vulnerability."))
              assertFalse(fixture.hasText("Scope"), "Optional report metadata starts collapsed")
              val list = fixture.taggedBounds("result-list")
              val detail = fixture.taggedBounds("result-detail")
              assertTrue(
                  list.height > 0f && detail.height > 0f && detail.bottom <= height * density,
                  label)
              assertTrue(list.right <= detail.left || list.bottom <= detail.top, label)
              fixture.revealText("Open source", "result-detail")
              fixture.assertTextFits("Open source")
              fixture.revealText("Prepare fix", "result-detail")
              fixture.assertTextFits("Prepare fix")
              assertTrue(fixture.isDisabled("Prepare fix"), "Long path is absent from the index")
              assertTrue(fixture.requestFocus("Open source"))
              fixture.render("$label-focused")
              assertTrue(fixture.isFocusedControl("Open source"))
              fixture.assertColorVisible(FocusAccent)
              assertFalse(fixture.hasEditableText(withinTag = "result-detail"))
              assertTrue(fixture.verticalScrollValue("result-detail") > 0f)
              browser.choose(securityResults(page).single { it.report.source == "ai" }.rowKey)
              fixture.render("$label-model")
              assertTrue(fixture.hasText("Model hypothesis"))
              assertTrue(
                  fixture.hasText(
                      "Unverified model hypothesis. Validate the preconditions and source evidence before remediation."))
              if (width == 800 && scale == 1.5f && density == 1f) {
                browser.choose(
                    securityResults(page).single { it.report.source == "deterministic" }.rowKey)
                fixture.render()
                fixture.revealText(path + ":4", "result-detail")
                val warning =
                    "A source rule match identifies a pattern; it does not confirm a vulnerability."
                fixture.revealText(warning, "result-detail")
                assertTrue(fixture.copyTextByDragging(warning, warning).isNotEmpty())
                fixture.revealText("Report metadata", "result-detail")
                fixture.clickText("Report metadata")
                fixture.render("$label-metadata-expanded")
                assertTrue(fixture.hasText("Content hash"))
                assertTrue(fixture.taggedBounds("result-detail").bottom <= height * density)
              }
              assertEquals(0, privileged, label)
            }
      }
    }
  }

  @Test
  fun securityProductionRendersKeepEvidenceIdentityAndScopedEmptyStateDistinct() {
    val populated = securityPageFixture()
    var prepared = 0
    var openedAnalysis = 0
    ComposeVisualFixture(1440, 900) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(populated, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> prepared++ },
                  { _, _ -> },
                  { openedAnalysis++ },
                  FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render("security-populated-wide-1440-900")
          fixture.clickDescription("Inspect Credential-like assignment")
          fixture.render("security-source-rule-wide-1440-900")
          assertTrue(fixture.hasText("Source rule"))
          assertTrue(
              fixture.hasText(
                  "A source rule match identifies a pattern; it does not confirm a vulnerability."))
          fixture.clickDescription("Inspect Review input boundary")
          fixture.render("security-model-hypothesis-wide-1440-900")
          assertTrue(fixture.hasText("Model hypothesis"))
          assertTrue(
              fixture.hasText(
                  "Unverified model hypothesis. Validate the preconditions and source evidence before remediation."))
          assertEquals(0, prepared)
          assertEquals(0, openedAnalysis)
        }

    val reports = requireNotNull(populated.results)
    val sourceReport = reports.security.single { it.source == "deterministic" }
    val unavailableEvidence =
        populated.copy(
            section =
                populated.section.copy(
                    results = reports.copy(security = listOf(sourceReport.copy(source = "ai")))))
    ComposeVisualFixture(800, 650, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(unavailableEvidence, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { _, _ -> prepared++ },
                  { _, _ -> },
                  { openedAnalysis++ },
                  FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render("security-unavailable-evidence-compact-800-650-150")
          fixture.clickDescription("Inspect Credential-like assignment")
          fixture.render("security-unavailable-evidence-detail-compact-800-650-150")
          assertTrue(fixture.hasText("Evidence type unavailable"))
          assertTrue(
              fixture.hasText(
                  "Evidence type was unavailable. Do not treat this finding as verified."))
          assertEquals(0, prepared)
          assertEquals(0, openedAnalysis)
        }

    val empty = resultPageFixture("security").copy(run = null, section = AnalysisSectionState())
    ComposeVisualFixture(800, 650, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(empty, null),
              SecurityWorkspaceActions(
                  { _, _ -> prepared++ },
                  { _, _ -> },
                  { openedAnalysis++ },
                  FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render("security-empty-compact-800-650-150")
          fixture.assertTextFits("Analysis has not started.")
          assertFalse(fixture.hasText("Prepare fix"))
          fixture.clickText("View analysis")
          assertEquals(1, openedAnalysis)
          assertEquals(0, prepared)
        }
  }

  @Test
  fun f05SavedResultRecoveryCapturesProductionPagesAtSupportedSizesAndScales() {
    val viewports = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in viewports) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        AnalysisResultType.entries.forEach { type ->
          val original =
              when (type.category) {
                "performance" -> performancePageFixture()
                "security" -> securityPageFixture()
                else -> resultPageFixture("bugs")
              }
          var page by
              mutableStateOf(
                  original.copy(section = AnalysisSectionState(error = "saved read failed")))
          val browser = newResultBrowserState(original)
          val rows =
              when (type.category) {
                "performance" -> performanceResults(original).map(PerformanceResult::row)
                "security" -> securityResults(original).map(SecurityResult::row)
                else -> original.semantic.map(::semanticResultRow)
              }
          var visibleRows by mutableStateOf(emptyList<ResultRowPresentation>())
          var retries = 0
          var navigations = 0
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    AnalysisResultsPane(
                        page,
                        visibleRows,
                        browser,
                        openAnalysis = { navigations++ },
                        retryResults = { retries++ }) { key ->
                          Text("Retained detail for $key")
                        }
                  }
              .use { fixture ->
                val label = "f05-${type.category}-$width-$height-$scale-${density}x"
                fixture.render("$label-empty-read-failed")
                fixture.assertTextFits("Retry loading results")
                fixture.assertTextFits("No result details loaded yet.")
                assertFalse(fixture.hasText("No findings in the analyzed scope."))
                fixture.revealText("Retry loading results", "result-read-feedback")
                fixture.render("$label-empty-recovery-revealed")
                assertTrue(
                    fixture.firstVisibleTextBounds("Retry loading results").bottom <=
                        height * density)
                page = original.copy(section = original.section.copy(error = "saved read failed"))
                visibleRows = rows
                fixture.render("$label-retained-read-failed")
                fixture.assertTextFits("Results could not be refreshed: saved read failed")
                assertTrue(fixture.taggedBounds("result-list").height > 0f)
                assertTrue(
                    fixture.taggedBounds("result-read-feedback").bottom <=
                        fixture.taggedBounds("result-list").top)
                browser.query = "no matching saved finding"
                fixture.render("$label-filtered-read-failed")
                fixture.assertTextFits("No matching results.")
                fixture.assertTextFits("Results could not be refreshed: saved read failed")
                assertTrue(
                    fixture.taggedBounds("result-read-feedback").bottom <=
                        fixture.taggedBounds("result-empty").top)
                fixture.revealText("Retry loading results", "result-read-feedback")
                fixture.render("$label-filtered-recovery-revealed")
                assertTrue(
                    fixture.firstVisibleTextBounds("Retry loading results").bottom <=
                        height * density)
                fixture.revealText("Clear filters", "result-overview")
                fixture.clickDescription("Clear filters")
                fixture.render("$label-cleared-local-filter")
                assertTrue(fixture.taggedBounds("result-list").height > 0f)
                assertEquals(0, retries + navigations, label)
              }
        }
      }
    }
  }

  @Test
  fun f05ReviewAttemptCapturesDoNotPromoteRetainedPass() {
    val base = editorComparisonReviewFixture()
    val draft = requireNotNull(base.draft)
    val diagnostic = "Validation transport failed for the selected declaration."
    val started =
        DesktopState(
                review =
                    DraftReviewState(draft = draft, editor = base.editor, checks = base.checks))
            .reduce(DesktopEvent.DraftValidationStarted(4))
    val stopped =
        started.reduce(
            DesktopEvent.DraftValidationStopped(4, ValidationAttemptStatus.Failed, diagnostic))
    assertFalse(
        draftReviewEligibility(
                stopped.review.editor,
                stopped.review.draft,
                stopped.checks,
                base.selected,
                base.project,
                stopped.review.checkAttempt)
            .eligible)
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
          var state by
              mutableStateOf(
                  base.copy(
                      editor = stopped.review.editor,
                      draft = stopped.review.draft,
                      checks = stopped.review.checks))
          var actions = 0
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    ReviewToolWindow(
                        state,
                        ReviewToolWindowActions({ actions++ }, { actions++ }, { actions++ }),
                        DraftApplicationActions({ actions++ }, { actions++ }))
                  }
              .use { fixture ->
                val label = "f05-review-$width-$height-$scale-${density}x"
                fixture.render("$label-validation-failed")
                assertTrue(fixture.hasText("Validation failed"))
                assertFalse(fixture.hasText("Ready to apply"))
                assertFalse(fixture.hasText("Apply change"))
                assertTrue(fixture.hasText("Validate the latest draft before applying it."))
                fixture.revealText("Edit draft", "review-action-scroll")
                fixture.assertTextFits("Edit draft")
                state =
                    base.copy(
                        checkAttempt =
                            CheckAttempt(7, CheckCandidate(draft), ValidationAttemptStatus.Running))
                fixture.render("$label-checks-running-after-pass")
                assertTrue(fixture.hasText("Checks running"))
                assertTrue(
                    fixture.hasText("Previous check report (retained; not current approval)"))
                assertFalse(fixture.hasText("Ready to apply"))
                assertFalse(fixture.hasText("Apply change"))
                assertEquals(0, actions)
              }
        }
      }
    }
  }

  @Test
  fun f05SelectionConnectionOpeningAndLifecycleCapturesUseProductionOwners() {
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    val selection = selectionFixture()
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> }, {}, {}, {}, {}, refreshStatus = { error("Unexpected status refresh") })
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        val label = "f05-owners-$width-$height-$scale-${density}x"
        val pxWidth = (width * density).toInt()
        val pxHeight = (height * density).toInt()
        var refreshes = 0
        val failedSelection =
            ProjectAnalysisRunState(
                fileSelection =
                    AnalysisSelectionState(
                        selection,
                        error = "Saved selection could not be written",
                        failure = AnalysisSelectionFailure.Save))
        ComposeVisualFixture(pxWidth, pxHeight, scale, density) {
              AnalysisFileSelector(
                  failedSelection, actions.copy(refreshSelection = { refreshes++ }))
            }
            .use { fixture ->
              fixture.render("$label-selection-save-failed")
              fixture.clickDescription("Collapse Files")
              fixture.render("$label-selection-failed-collapsed")
              assertTrue(fixture.hasText("Could not save file selection"))
              assertTrue(
                  fixture.hasText(
                      "The last confirmed selection is still shown. Refresh files reads the saved selection; it does not retry a failed change or start analysis."))
              assertTrue(fixture.hasText("Refresh files"))
              fixture.assertTextFits("Refresh files")
              assertTrue(fixture.requestFocus("Refresh files"))
              fixture.render()
              assertEquals(0, refreshes)
              assertTrue(fixture.pressKey(Key.Enter))
              assertEquals(1, refreshes)
            }
        var opens = 0
        var reconnects = 0
        val landingActions =
            DesktopShellProjectActions(
                importProject = { opens++ }, reindexProject = {}, reconnect = { reconnects++ })
        ComposeVisualFixture(pxWidth, pxHeight, scale, density) {
              ProjectLanding(
                  DesktopState(
                      projectState =
                          ProjectWorkspaceState(
                              rememberedPath = "/projects/remembered",
                              openingAttempt =
                                  ProjectOpeningAttempt(
                                      1,
                                      "/projects/remembered",
                                      ProjectOpeningKind.Restore,
                                      ProjectOpeningOutcome.Failed("Local read failed"))),
                      connection = ConnectionState(label = "Disconnected")),
                  landingActions,
                  FocusRequester())
            }
            .use { fixture ->
              fixture.render("$label-landing-opening-and-offline")
              assertTrue(fixture.hasText("Could not restore project"))
              assertTrue(fixture.hasText("Daemon disconnected"))
              fixture.assertTextFits("Open project…")
              fixture.revealText("Retry restore", "project-landing-scroll")
              fixture.assertTextFits("Retry restore")
              fixture.revealText("Reconnect daemon", "project-landing-scroll")
              fixture.assertTextFits("Reconnect daemon")
              assertEquals(0, opens + reconnects)
            }
        ComposeVisualFixture(pxWidth, pxHeight, scale, density) {
              MainToolbar(
                  ToolbarState(
                      visualFixtureProject,
                      false,
                      "",
                      ConnectionState(label = "Disconnected"),
                      null,
                      openingAttempt =
                          ProjectOpeningAttempt(
                              1,
                              "/projects/other",
                              ProjectOpeningKind.Restore,
                              ProjectOpeningOutcome.Failed("Local read failed"))),
                  ToolbarActions({ opens++ }, {}, { reconnects++ }, {}))
            }
            .use { fixture ->
              fixture.render("$label-toolbar-opening-failed")
              assertTrue(fixture.hasText("Current project"))
              assertTrue(fixture.hasText(visualFixtureProject.name))
              assertTrue(fixture.hasText("Could not restore project"))
              assertTrue(fixture.hasText("/projects/other"))
              fixture.assertTextFits("Retry restore")
              assertEquals(0, opens + reconnects)
            }
        ComposeVisualFixture(pxWidth, pxHeight, scale, density) {
              ExplorerPane(
                  ExplorerPaneState(
                      null,
                      null,
                      "",
                      emptySet(),
                      false,
                      projectAvailable = false,
                      readError = "Local file read failed"),
                  ExplorerPaneActions({}, {}, {}, {}, {}, openProject = { opens++ }),
                  Modifier.fillMaxSize())
            }
            .use { fixture ->
              fixture.render("$label-explorer-read-failed")
              assertTrue(fixture.hasText("Could not open file"))
              fixture.assertTextFits("Open project")
              fixture.assertTextAbove("Could not open file", "No project open")
              assertEquals(0, opens)
            }
        ComposeVisualFixture(pxWidth, pxHeight, scale, density) {
              ContextToolWindow(
                  ContextToolWindowState(
                      null,
                      ScopedModel(),
                      false,
                      null,
                      null,
                      fileReadError = "Local file read failed"),
                  ContextToolWindowActions({}, {}, {}, {}, {}, openFile = { opens++ }))
            }
            .use { fixture ->
              fixture.render("$label-context-read-failed")
              assertTrue(fixture.hasText("Could not open file"))
              fixture.assertTextFits("Select a file")
              assertFalse(fixture.hasText("No file selected"))
              assertEquals(0, opens)
            }
        val run = analysisRunFixture().copy(status = "interrupted")
        ComposeVisualFixture(pxWidth, pxHeight, scale, density) {
              AnalysisRunStrip(
                  AnalysisWorkspacePaneState(
                      visualFixtureProject,
                      ProjectAnalysisRunState(
                          run = run,
                          error = "Daemon status read failed",
                          fileSelection = failedSelection.fileSelection)),
                  actions,
                  AnalysisRunStripScope.Analysis)
            }
            .use { fixture ->
              fixture.render("$label-interrupted-run-error")
              assertTrue(fixture.hasText("Analysis action needs attention"))
              assertTrue(fixture.hasText("Daemon status read failed"))
              assertFalse(fixture.hasText("Ready to apply"))
            }
        ComposeVisualFixture(pxWidth, pxHeight, scale, density) {
              ProjectSummaryPane(
                  visualFixtureOverview,
                  resultProjectFixture(),
                  {},
                  run =
                      run.copy(
                          sections =
                              run.sections.map {
                                if (it.category == "security")
                                    it.copy(status = "failed", findingCount = 19)
                                else it
                              }),
                  sections =
                      mapOf(
                          AnalysisResultKey("security") to
                              AnalysisSectionState(error = "Saved details could not be read")))
            }
            .use { fixture ->
              fixture.render("$label-summary-unavailable-details")
              fixture.revealText("Saved details unavailable · 19 reported")
              fixture.render("$label-summary-details-revealed")
              assertTrue(fixture.hasText("Saved details unavailable · 19 reported"))
              assertFalse(fixture.hasText("No Security findings"))
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
                  {}, {}, FindingActions({}, { _, _ -> }, {}), openSource = {}))
        }
        .use { fixture ->
          fixture.render("results-stale-error-800-1.25")
          assertFalse(fixture.hasText("Reported findings"))
          fixture.clickDescription("Inspect Avoid repeated allocation")
          fixture.render()
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertTrue(fixture.hasText("Stale"))
          assertTrue(fixture.hasText("Report warning"))
          assertTrue(fixture.hasText("Saved evidence may not match current source. Analyze again."))
          assertTrue(fixture.hasText("Analyze again to prepare a fix from current source."))
          assertFalse(fixture.hasText("Prompt version"))
        }
    val empty = resultPageFixture("bugs").copy(run = null, section = AnalysisSectionState())
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(emptyList(), null, false, empty),
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }, {}), {}, {}))
        }
        .use { fixture ->
          fixture.render("results-empty-800-1.5")
          assertFalse(fixture.hasText("Reported findings"))
          assertTrue(fixture.hasText("Analysis has not started."))
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
  fun performanceHypothesisKeepsCompleteQualitativeEvidenceAndWarningsReachable() {
    val original = performancePageFixture()
    val path =
        "internal/platform/transport/generated/configuration/validation/repeated_allocation_handler.go"
    val narrative =
        "Observe allocations in a representative workload with <literal> markup. ".repeat(3)
    val warning = "Partial analysis: some source contexts were unavailable. ".repeat(2)
    val report =
        original.results!!
            .performance
            .single()
            .copy(
                path = path,
                status = "partial",
                warning = warning,
                model = "",
                profile = "local profile",
                providerOrigin = "",
                generatedAt = "",
                findings =
                    listOf(
                        original.results!!
                            .performance
                            .single()
                            .findings
                            .single()
                            .copy(
                                observedPattern = narrative,
                                recommendation = narrative,
                                workloadConditions = narrative,
                                tradeoff = narrative,
                                verificationPlan = narrative,
                                engineeringInsight =
                                    EngineeringInsight(
                                        mechanism = narrative,
                                        whyItMattersHere = narrative,
                                        tradeoffOrFailureMode = narrative,
                                        transferableLesson = narrative))))
    val page =
        original.copy(
            section =
                original.section.copy(
                    results = original.results!!.copy(performance = listOf(report))))
    listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f)).forEach { (width, height, scale) ->
      ComposeVisualFixture(width, height, scale) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(page, resultIndexFixture()),
                PerformanceWorkspaceActions(
                    {}, {}, FindingActions({}, { _, _ -> }, {}), openSource = {}))
          }
          .use { fixture ->
            fixture.render("performance-hypothesis-$width-$scale")
            fixture.clickDescription("Inspect Avoid repeated allocation")
            fixture.render("performance-hypothesis-detail-$width-$scale")
            assertTrue(fixture.hasText("$path:4 · Run"))
            assertTrue(fixture.hasText("Partial"))
            assertTrue(fixture.hasText("Partial report; some evidence may be missing."))
            assertTrue(fixture.hasText("Model suggestion"))
            assertTrue(fixture.hasText("Potential impact · qualitative, not a measured gain"))
            assertTrue(
                fixture.hasText("Model confidence · not a measurement or speedup probability"))
            assertTrue(fixture.hasText("high"))
            assertTrue(fixture.hasText("medium"))
            fixture.revealText(warning, "result-detail")
            if (width == 800) fixture.assertTextWrapsWithoutClipping(warning)
            fixture.revealText("Prepare fix", "result-detail")
            assertTrue(fixture.isDisabled("Prepare fix"))
            assertTrue(
                fixture.hasText("The target file is missing or ambiguous in the project index."))
            fixture.revealText(narrative, "result-detail")
            fixture.assertTextWrapsWithoutClipping(narrative)
            fixture.revealText("Engineering insight", "result-detail")
            if (!fixture.hasText("Transferable lesson")) fixture.clickText("Engineering insight")
            fixture.render("performance-hypothesis-insight-$width-$scale")
            assertTrue(fixture.hasText("Transferable lesson"))
            fixture.revealText("Report metadata", "result-detail")
            fixture.clickText("Report metadata")
            fixture.render("performance-hypothesis-metadata-$width-$scale")
            assertTrue(fixture.hasText("Provider origin"))
            assertTrue(fixture.hasText("Not supplied."))
            assertTrue(fixture.hasText("local profile"))
          }
    }
    val missing =
        report.copy(
            status = "completed",
            warning = "",
            findings =
                listOf(
                    report.findings
                        .single()
                        .copy(
                            startLine = 0,
                            symbol = "",
                            potentialImpact = "",
                            confidence = "",
                            workloadConditions = "",
                            verificationPlan = "",
                            engineeringInsight = null)))
    val missingPage =
        page.copy(
            section =
                page.section.copy(results = page.results!!.copy(performance = listOf(missing))))
    ComposeVisualFixture(800, 650, 1.25f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(missingPage, resultIndexFixture()),
              PerformanceWorkspaceActions(
                  {}, {}, FindingActions({}, { _, _ -> }, {}), openSource = {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Inspect Avoid repeated allocation")
          fixture.render("performance-hypothesis-missing-800-1.25")
          assertTrue(fixture.hasText("$path · Source line not supplied · Symbol not supplied"))
          assertFalse(fixture.hasText("$path:0"))
          assertTrue(fixture.hasText("Not supplied."))
          assertTrue(fixture.hasText("Engineering insight"))
        }
  }

  @Test
  fun f18ProductionBrowserStatesRemainReachableAcrossViewportsTextAndDensity() {
    val original = performancePageFixture()
    val longPath = "internal/" + "deeply/nested/日本語/".repeat(5) + "handler.go"
    val report = original.results!!.performance.single()
    val populated =
        original.copy(
            section =
                original.section.copy(
                    results =
                        original.results!!.copy(
                            performance =
                                listOf(
                                    report.copy(
                                        path = longPath,
                                        findings =
                                            (1..12).map { n ->
                                              report.findings
                                                  .single()
                                                  .copy(
                                                      id = "perf-$n",
                                                      title = "Repeated allocation $n")
                                            })))))
    val sizes =
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600) +
            listOf(627 to 768, 629 to 768, 937 to 768, 939 to 768)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      if (width in listOf(627, 629) && scale != 1f || width in listOf(937, 939) && scale != 1.5f)
          continue
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        val label = "f18-performance-$width-$height-$scale-${density}x"
        var page by mutableStateOf(populated)
        val browser = newResultBrowserState(page)
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  PerformanceWorkspacePane(
                      PerformanceWorkspacePaneState(page, resultIndexFixture(), browser = browser),
                      PerformanceWorkspaceActions(
                          { error("Passive render prepared a fix") },
                          {},
                          FindingActions(
                              { error("Passive render prepared a semantic fix") },
                              { _, _ -> error("Passive render changed triage") },
                              { error("Passive render opened source") }),
                          openSource = { error("Passive render opened source") },
                          loadBenchmarks = { error("Passive render listed benchmarks") },
                          selectBenchmark = { error("Passive render selected benchmark") },
                          runBenchmark = { error("Passive render ran benchmark") }))
                }
            .use { fixture ->
              fixture.render("$label-populated")
              fixture.revealText("Repeated allocation 12", "result-list")
              assertTrue(fixture.hasDescription("Inspect Repeated allocation 12"), label)
              fixture.clickDescription("Inspect Repeated allocation 12")
              fixture.render("$label-selected")
              assertTrue(fixture.hasText("$longPath:4 · Run"), label)
              fixture.revealText("Prepare fix", "result-detail")
              assertTrue(fixture.isDisabled("Prepare fix"), label)
              fixture.assertTextFits("Prepare fix")
              fixture.clickText("Explore benchmark evidence")
              fixture.render("$label-evidence")
              assertTrue(
                  fixture.hasText(
                      "No benchmark evidence is available for the current candidate. Listing is read-only; running a benchmark requires explicit local execution."),
                  label)
              val list = fixture.taggedBounds("result-list")
              val detail = fixture.taggedBounds("result-detail")
              assertTrue(list.height > 0 && detail.height > 0, label)
              assertTrue(list.right <= detail.left || list.bottom <= detail.top, label)
              assertTrue(detail.bottom <= height * density, label)
              browser.query = "absent title"
              fixture.render("$label-no-match")
              assertTrue(fixture.hasText("No matching results."), label)
              browser.query = ""
              page = populated.copy(section = populated.section.copy(error = "Saved read failed"))
              fixture.render("$label-retained-error")
              assertTrue(
                  fixture.hasText("Results could not be refreshed: Saved read failed"), label)
              assertTrue(fixture.hasText("12 loaded · 1 reported"), label)
            }
      }
    }
    val emptyProgress = original.progress!!.copy(status = "completed", findingCount = 0)
    val emptyRun =
        original.run!!.copy(
            status = "completed",
            sections =
                original.run.sections.map {
                  if (it.category == "performance") emptyProgress else it
                })
    val empty =
        original.copy(
            run = emptyRun,
            section =
                original.section.copy(
                    results =
                        original.results!!.copy(
                            progress = emptyProgress, performance = emptyList())))
    val cases =
        listOf(
            "empty" to (empty to "No findings in the analyzed scope."),
            "unavailable" to
                (empty.copy(
                    run =
                        emptyRun.copy(
                            status = "unavailable",
                            sections =
                                emptyRun.sections.map {
                                  if (it.category == "performance")
                                      it.copy(status = "unavailable", findingCount = null)
                                  else it
                                }),
                    section =
                        empty.section.copy(
                            results =
                                empty.results!!.copy(
                                    progress =
                                        emptyProgress.copy(
                                            status = "unavailable", findingCount = null)))) to
                    "Analysis is unavailable for this category."),
            "stale" to
                (populated.copy(run = populated.run!!.copy(status = "stale")) to
                    "Saved evidence may not match current source. Analyze again."),
            "partial" to
                (populated.copy(
                    section =
                        populated.section.copy(
                            results =
                                populated.results!!.copy(
                                    performance = listOf(report.copy(status = "partial"))))) to
                    "Partial report; some evidence may be missing."))
    for ((name, expectation) in cases) {
      val (page, text) = expectation
      ComposeVisualFixture(800, 650, 1.5f) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(page, resultIndexFixture()),
                PerformanceWorkspaceActions(
                    {}, {}, FindingActions({}, { _, _ -> }, {}), openSource = {}))
          }
          .use { fixture ->
            fixture.render("f18-performance-$name-800-650-1.5-1x")
            if (name == "stale" || name == "partial") {
              fixture.clickDescription(
                  if (name == "partial") "Inspect Avoid repeated allocation"
                  else "Inspect Repeated allocation 1")
              fixture.render()
              fixture.revealText(text, "result-detail")
            }
            assertTrue(fixture.hasText(text), name)
          }
    }
  }

  @Test
  fun benchmarkMedianColumnsAndCollapsedLimitationsRenderAcrossEvidenceOutcomes() {
    val choice =
        GoBenchmarkChoice("BenchmarkRun", listOf("go", "test", "-bench", "^BenchmarkRun$"), "scope")
    val identity =
        GoBenchmarkComparisonIdentity(
            "draft", 1, "candidate", "project", "revision", "base", "main.go")
    val comparison =
        GoBenchmarkComparison(
            draftId = "draft",
            draftRevision = 1,
            draftHash = "candidate",
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "base",
            targetPath = "main.go",
            benchmark = choice.name,
            scope = choice.scope,
            status = "completed",
            command = choice.command,
            base = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 100.0, 10, 1) }),
            candidate = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 90.0, 12, 1) }))
    val draft =
        DeclarationDraft(
            id = identity.draftId,
            revision = identity.draftRevision,
            hash = identity.draftHash,
            projectId = identity.projectId,
            projectRevision = identity.projectRevision,
            baseFileHash = identity.baseFileHash,
            targetPath = identity.targetPath,
            validation =
                DeclarationValidation(
                    true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")))
    val catalog =
        GoBenchmarkCatalog(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            targetPath = draft.targetPath,
            available = true,
            benchmarks = listOf(choice))
    val snapshot =
        DesktopState(
            projectState = ProjectWorkspaceState(project = performancePageFixture().project),
            selection =
                FileSelectionState(
                    selectedFile =
                        ProjectFileInfo(
                            "main.go",
                            "base",
                            "main.go",
                            language = "Go",
                            sizeBytes = 1,
                            lineCount = 1,
                            modifiedAt = "",
                            binary = false)),
            review =
                DraftReviewState(
                    draft = draft,
                    editor = editableDraft(draft),
                    benchmark =
                        BenchmarkEvidenceState(
                            catalog = catalog,
                            selected = choice,
                            discovery = BenchmarkDiscoveryOutcome.Loaded,
                            comparison = comparison)))
    val current =
        PerformanceWorkspacePaneState(
            performancePageFixture(),
            resultIndexFixture(),
            benchmarkComparison =
                comparison.copy(
                    candidate =
                        GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 90.0, 10, 1) })),
            expectedBenchmarkIdentity = identity,
            selectedBenchmark = choice,
            benchmarkEligibility = benchmarkEligibility(snapshot))
    val cases =
        listOf(
            Triple("measured", current, listOf("90 ns/op", "Observed change: -10.0%")),
            Triple(
                "trade-off",
                current.copy(benchmarkComparison = comparison),
                listOf("12 B/op", "Observed change: +20.0%")),
            Triple(
                "partial",
                current.copy(
                    benchmarkComparison =
                        comparison.copy(
                            candidate =
                                GoBenchmarkMeasurement(
                                    listOf(
                                        GoBenchmarkSample(1, 80.0, 0, 0),
                                        GoBenchmarkSample(1, 90.0, 0, 0))))),
                listOf(
                    "85 ns/op",
                    "Candidate samples: 2",
                    "partial; 2/2 valid observations; 5 required")),
            Triple(
                "missing-memory",
                current.copy(
                    benchmarkComparison =
                        comparison.copy(
                            candidate =
                                GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 90.0) }))),
                listOf(
                    "90 ns/op",
                    "Unavailable (B/op)",
                    "Unavailable (allocs/op)",
                    "unavailable; 0/5 valid observations; 5 required")),
            Triple(
                "missing-side",
                current.copy(benchmarkComparison = comparison.copy(candidate = null)),
                listOf(
                    "Candidate samples: not returned",
                    "Unavailable (ns/op)",
                    "measurement not returned")),
            Triple(
                "empty-side",
                current.copy(
                    benchmarkComparison = comparison.copy(candidate = GoBenchmarkMeasurement())),
                listOf(
                    "Candidate samples: 0",
                    "Unavailable (ns/op)",
                    "empty samples; 0/0 valid observations; 5 required")),
            Triple(
                "noisy",
                current.copy(
                    benchmarkComparison =
                        comparison.copy(
                            candidate =
                                GoBenchmarkMeasurement(
                                    listOf(80.0, 80.0, 80.0, 80.0, 120.0).map {
                                      GoBenchmarkSample(1, it, 10, 1)
                                    }))),
                listOf("80 ns/op", "Observed change: -20.0%")),
            Triple(
                "stale",
                current.copy(expectedBenchmarkIdentity = identity.copy(draftHash = "changed")),
                listOf("90 ns/op", "Historical change: -10.0%")),
            Triple(
                "running-prior",
                current.copy(benchmarkAdmission = BenchmarkAdmissionOutcome.Running),
                listOf("90 ns/op", "Historical change: -10.0%"))) +
            listOf("canceled", "failed", "unavailable", "future-status").flatMap { status ->
              val terminal =
                  GoBenchmarkComparison(
                      status = status, reason = "Recorded $status reason; refresh to recover.")
              listOf(
                  Triple(
                      "$status-prior",
                      current.copy(benchmarkLatestOutcome = BenchmarkComparisonOutcome(terminal)),
                      listOf("90 ns/op", "Historical change: -10.0%")),
                  Triple(
                      "$status-no-measurements",
                      current.copy(
                          benchmarkComparison = null,
                          benchmarkLatestOutcome = BenchmarkComparisonOutcome(terminal)),
                      emptyList()))
            }
    for ((name, state, observations) in cases) {
      for ((width, height, scale) in listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f))) {
        val status =
            performanceBenchmarkStatusPresentation(
                state.benchmarkComparison,
                state.expectedBenchmarkIdentity,
                state.selectedBenchmark,
                admission = state.benchmarkAdmission,
                eligibility = state.benchmarkEligibility,
                latestOutcome = state.benchmarkLatestOutcome)
        var requests = 0
        ComposeVisualFixture(width, height, scale) {
              PerformanceWorkspacePane(
                  state,
                  PerformanceWorkspaceActions(
                      {},
                      {},
                      FindingActions({}, { _, _ -> }, {}),
                      openSource = {},
                      loadBenchmarks = { requests++ },
                      selectBenchmark = { requests++ },
                      runBenchmark = { requests++ }))
            }
            .use { fixture ->
              fixture.render()
              assertTrue(fixture.hasText(status.stateLabel), name)
              assertTrue(
                  fixture.hasText(status.summary),
                  "Essential reason survives all collapsed details: $name")
              assertFalse(fixture.hasText("Baseline median"))
              fixture.clickDescription("Expand Explore benchmark evidence")
              fixture.render("f22-medians-$name-collapsed-$width-$height-$scale")
              fixture.revealTextFullyWithin(status.summary, "result-overview")
              fixture.assertTextFits(status.summary, maxLines = 30)
              when (name) {
                "trade-off" ->
                    assertTrue(status.summary.contains("trade-off, not an unconditional win"))
                "partial",
                "missing-side",
                "empty-side" -> assertTrue(status.summary.contains("five valid samples per side"))
                "missing-memory" ->
                    assertTrue(status.summary.contains("cannot establish a performance win"))
                "noisy" -> assertTrue(status.summary.contains("too variable"))
                "stale" -> assertTrue(status.summary.contains("different draft or source revision"))
                "running-prior" -> assertTrue(status.summary.contains("is running"))
              }
              state.benchmarkLatestOutcome?.let { assertEquals(it.response.reason, status.summary) }
              assertFalse(
                  fixture.hasText("Baseline median"), "Optional measurements start collapsed")
              state.benchmarkComparison?.let { evidence ->
                val assessment =
                    performanceBenchmarkPresentation(
                        evidence,
                        state.expectedBenchmarkIdentity,
                        state.selectedBenchmark,
                        status.priorEvidence)
                if (status.priorEvidence) {
                  fixture.revealTextFullyWithin(assessment.summary, "result-overview")
                  fixture.assertTextFits(assessment.summary, maxLines = 30)
                  assertFalse(fixture.hasText("CPU is lower for the selected benchmark."))
                }
                val details =
                    if (status.priorEvidence) "Prior measurement details" else "Measurement details"
                fixture.revealTextFullyWithin(details, "result-overview")
                fixture.clickText(details)
                fixture.render()
                fixture.revealTextFullyWithin("Baseline median", "result-overview")
                for (text in
                    listOf(
                        "Baseline median",
                        "Candidate median",
                        "Change / availability",
                        "Time (ns/op)",
                        "Bytes (B/op)",
                        "Allocations (allocs/op)",
                        "Baseline samples: 5",
                        "100 ns/op",
                        "10 B/op",
                        "1 allocs/op") + observations) {
                  fixture.revealTextFullyWithin(text, "result-overview")
                  fixture.assertTextFits(text, maxLines = 20)
                }
                if (name == "trade-off" && width == 1440) {
                  assertFalse(fixture.hasText("Project ID: ${evidence.projectId}"))
                  fixture.revealTextFullyWithin("Recorded conditions & identity", "result-overview")
                  fixture.clickText("Recorded conditions & identity")
                  fixture.render()
                  performanceBenchmarkRecordedRows(evidence).forEach { (label, value) ->
                    assertTrue(fixture.hasText("$label: $value"), label)
                  }
                  fixture.revealTextFullyWithin("Returned sample details", "result-overview")
                  fixture.clickText("Returned sample details")
                  fixture.render()
                  performanceBenchmarkSampleRows(evidence).forEach { (label, value) ->
                    assertTrue(fixture.hasText("$label: $value"), label)
                  }
                  fixture.revealTextFullyWithin(
                      "Copy displayed benchmark evidence", "result-overview")
                  fixture.clickText("Copy displayed benchmark evidence")
                  fixture.render("f22-recorded-details-$width-$height-$scale")
                  assertTrue(fixture.hasText("Displayed benchmark evidence copied."))
                  assertEquals(
                      performanceBenchmarkCopyText(
                          evidence,
                          assessment,
                          status.priorEvidence || assessment.isStale,
                          true,
                          true),
                      fixture.clipboardText())
                  fixture.failClipboardWrites = true
                  fixture.clickText("Copy displayed benchmark evidence")
                  fixture.render()
                  assertTrue(
                      fixture.hasText("Could not copy benchmark evidence: Clipboard unavailable"))
                  assertEquals(0, requests, "Clipboard failure remains local")
                }
                fixture.render("f22-medians-$name-expanded-$width-$height-$scale")
                if (status.priorEvidence) {
                  assertTrue(fixture.hasText("Prior benchmark evidence"))
                  assertFalse(fixture.hasText("Measured · selected benchmark"))
                }
              }
              assertEquals(0, requests, "Inspection is passive: $name")
            }
      }
    }
  }

  @Test
  fun benchmarkInspectionWrapsLongEvidenceAcrossViewportTextDensityAndResize() {
    val path = "internal/" + "解析/日本語/évidence/".repeat(12) + "測定.go"
    val hash = "abc0123456789".repeat(12)
    val choice =
        GoBenchmarkChoice(
            "Benchmark解析_" + "日本語".repeat(12),
            listOf(
                "go",
                "test",
                "./$path",
                "-bench",
                "^Benchmark解析$",
                "-count=5",
                "-benchtime",
                "100ms",
                "-benchmem"),
            "opaque-" + hash)
    val comparison =
        GoBenchmarkComparison(
            draftId = "draft-" + hash,
            draftRevision = 2,
            draftHash = hash,
            projectId = "project",
            projectRevision = "revision-" + hash,
            baseFileHash = hash,
            targetPath = path,
            benchmark = choice.name,
            scope = choice.scope,
            status = "completed",
            command = choice.command,
            base = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1000, 100.0, 10, 1) }),
            candidate = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1000, 90.0, 10, 1) }))
    val draft =
        DeclarationDraft(
            id = comparison.draftId,
            revision = comparison.draftRevision,
            hash = hash,
            projectId = comparison.projectId,
            projectRevision = comparison.projectRevision,
            baseFileHash = hash,
            targetPath = path,
            validation =
                DeclarationValidation(true, "strict_symbol", diff = UnifiedDiff(path, path)))
    val catalog =
        GoBenchmarkCatalog(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = hash,
            targetPath = path,
            available = true,
            benchmarks = listOf(choice))
    val page = performancePageFixture()
    val snapshot =
        DesktopState(
            projectState =
                ProjectWorkspaceState(
                    project = page.project!!.copy(projectRevision = draft.projectRevision)),
            selection =
                FileSelectionState(
                    selectedFile =
                        ProjectFileInfo(
                            path,
                            hash,
                            "測定.go",
                            language = "Go",
                            sizeBytes = 1,
                            lineCount = 1,
                            modifiedAt = "",
                            binary = false)),
            review =
                DraftReviewState(
                    draft = draft,
                    editor = editableDraft(draft),
                    benchmark =
                        BenchmarkEvidenceState(
                            catalog = catalog,
                            selected = choice,
                            discovery = BenchmarkDiscoveryOutcome.Loaded,
                            comparison = comparison)))
    val eligibility = benchmarkEligibility(snapshot)
    assertTrue(eligibility.canCompare)
    val current =
        PerformanceWorkspacePaneState(
            page,
            resultIndexFixture(),
            benchmarkComparison = comparison,
            expectedBenchmarkIdentity = goBenchmarkComparisonIdentity(draft),
            selectedBenchmark = choice,
            benchmarkCatalog = catalog,
            benchmarkDiscovery = BenchmarkDiscoveryOutcome.Loaded,
            benchmarkEligibility = eligibility)
    val reason =
        "Daemon could not finish " +
            "解析対象/évidence/".repeat(14) +
            "; no new measurements were confirmed. Refresh to recover."
    val variants =
        listOf(
            "current" to current,
            "running-prior" to current.copy(benchmarkAdmission = BenchmarkAdmissionOutcome.Running),
            "failed-prior" to
                current.copy(
                    benchmarkLatestOutcome =
                        BenchmarkComparisonOutcome(
                            GoBenchmarkComparison(status = "failed", reason = reason))),
            "stale" to
                current.copy(
                    expectedBenchmarkIdentity =
                        goBenchmarkComparisonIdentity(draft.copy(hash = "changed"))))
    var caseIndex = 0
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        for (density in listOf(1f, 2f)) {
          val (name, state) = variants[caseIndex++ % variants.size]
          val status =
              performanceBenchmarkStatusPresentation(
                  comparison,
                  state.expectedBenchmarkIdentity,
                  choice,
                  state.benchmarkDiscovery,
                  state.benchmarkAdmission,
                  eligibility,
                  catalog,
                  state.benchmarkLatestOutcome)
          val assessment =
              performanceBenchmarkPresentation(
                  comparison, state.expectedBenchmarkIdentity, choice, status.priorEvidence)
          val prefix = "f22-responsive-$name-$width-$height-$scale-${density}x"
          var requests = 0
          ComposeVisualFixture(width, height, scale, density) {
                PerformanceWorkspacePane(
                    state,
                    PerformanceWorkspaceActions(
                        {},
                        {},
                        FindingActions({}, { _, _ -> }, {}),
                        openSource = {},
                        loadBenchmarks = { requests++ },
                        selectBenchmark = { requests++ },
                        runBenchmark = { requests++ }))
              }
              .use { fixture ->
                fixture.render("$prefix-collapsed")
                fixture.assertEveryTextLineReachable(status.summary, "result-overview")
                assertFalse(fixture.hasText("Baseline median"))
                val scrollOwners = fixture.scrollableContentCount()
                fixture.revealTextFullyWithin("Explore benchmark evidence", "result-overview")
                fixture.clickText("Explore benchmark evidence")
                fixture.render()
                val details =
                    if (status.priorEvidence) "Prior measurement details" else "Measurement details"
                fixture.revealTextFullyWithin(details, "result-overview")
                fixture.clickText(details)
                fixture.render()
                assertEquals(
                    scrollOwners,
                    fixture.scrollableContentCount(),
                    "Inspection reuses the overview scroll owner")
                fixture.assertEveryTextLineReachable(
                    assessment.summary, "result-overview", "benchmark-measurement-evidence")
                fun checkMetrics() {
                  val stacked = !fixture.hasText("Metric")
                  for (metric in assessment.metrics) {
                    val tag = "benchmark-metric-${metric.label}"
                    for ((label, side) in
                        listOf("Baseline" to metric.base, "Candidate" to metric.candidate)) {
                      assertTrue(
                          fixture.hasDescription(
                              "${metric.label}, $label median: ${side.display()}"))
                      if (stacked) assertEquals(1, fixture.taggedTextCount(tag, "$label median"))
                      fixture.assertEveryTextLineReachable(
                          side.medianLabel(metric.label), "result-overview", tag, label)
                      fixture.assertEveryTextLineReachable(
                          side.availabilityLabel(), "result-overview", tag, label)
                    }
                    fixture.assertEveryTextLineReachable(
                        metric.changeLabel(status.priorEvidence || assessment.isStale),
                        "result-overview",
                        tag)
                  }
                }
                checkMetrics()
                fixture.revealTextFullyWithin("Time (ns/op)", "result-overview")
                fixture.scrollBy(
                    fixture.firstVisibleTextBounds("Time (ns/op)").top -
                        fixture.taggedBounds("result-overview").top,
                    "result-overview")
                fixture.render("$prefix-medians")
                for (disclosure in
                    listOf("Recorded conditions & identity", "Returned sample details")) {
                  fixture.revealTextFullyWithin(disclosure, "result-overview")
                  fixture.clickText(disclosure)
                  fixture.render()
                }
                for ((label, value) in
                    performanceBenchmarkRecordedRows(comparison) +
                        performanceBenchmarkSampleRows(comparison)) {
                  fixture.assertEveryTextLineReachable(
                      "$label: $value", "result-overview", "benchmark-measurement-evidence")
                }
                fixture.assertEveryTextLineReachable(
                    "Opaque scope guard: ${comparison.scope}",
                    "result-overview",
                    "benchmark-measurement-evidence")
                fixture.render("$prefix-identity")
                fixture.revealTextFullyWithin(
                    "Copy displayed benchmark evidence", "result-overview")
                fixture.assertTextFits("Copy displayed benchmark evidence", maxLines = 4)
                fixture.clickText("Copy displayed benchmark evidence")
                fixture.render("$prefix-copy")
                val copied =
                    performanceBenchmarkCopyText(
                        comparison,
                        assessment,
                        status.priorEvidence || assessment.isStale,
                        true,
                        true)
                assertEquals(copied, fixture.clipboardText())
                // Resize the same composition with disclosures open and unchanged evidence.
                // The pane gutters consume 64 dp; density scales pixels, not the text budget.
                for (offset in listOf(-2, 2)) {
                  val breakpointWidth = ((640 * scale + 64 + offset) * density).toInt()
                  fixture.resize(breakpointWidth, height)
                  fixture.render()
                  assertEquals(offset < 0, !fixture.hasText("Metric"))
                  checkMetrics()
                  fixture.revealTextFullyWithin("Time (ns/op)", "result-overview")
                  fixture.scrollBy(
                      fixture.firstVisibleTextBounds("Time (ns/op)").top -
                          fixture.taggedBounds("result-overview").top,
                      "result-overview")
                  fixture.render("$prefix-breakpoint-$offset")
                  assertTrue(
                      fixture.hasText("Project ID: project"),
                      "Resize retains conditions disclosure")
                  assertTrue(
                      fixture.hasText(
                          "Baseline sample 1: iterations=1000; ns/op=100.0; B/op=10; allocs/op=1"))
                  fixture.revealTextFullyWithin(
                      "Copy displayed benchmark evidence", "result-overview")
                  fixture.clickText("Copy displayed benchmark evidence")
                  fixture.render()
                  assertEquals(
                      copied,
                      fixture.clipboardText(),
                      "Resizing cannot change evidence or its qualification")
                  assertEquals(scrollOwners, fixture.scrollableContentCount())
                }
                fixture.resize(width, height)
                fixture.render()
                checkMetrics()
                assertEquals(0, requests, "Disclosure, copy and resizing are local: $prefix")
              }
        }
      }
    }
  }

  @Test
  fun benchmarkDiscoveryOutcomesAndRecoveryRemainVisibleWithMeasurementDetailsCollapsed() {
    val draft =
        DeclarationDraft(
            id = "draft",
            revision = 1,
            hash = "candidate",
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "base",
            targetPath = "main.go",
            declaration = "func Run() {}",
            validation =
                DeclarationValidation(
                    true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")))
    val choice =
        GoBenchmarkChoice(
            "BenchmarkRun",
            listOf(
                "go",
                "test",
                ".",
                "-run",
                "^$",
                "-bench",
                "^BenchmarkRun$",
                "-count",
                "5",
                "-benchtime",
                "100ms",
                "-benchmem",
                "-timeout",
                "15s"),
            "opaque:scope-guard")
    val catalog =
        GoBenchmarkCatalog(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            targetPath = draft.targetPath,
            available = true,
            trusted = true,
            benchmarks = listOf(choice))
    val comparison =
        GoBenchmarkComparison(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            targetPath = draft.targetPath,
            benchmark = choice.name,
            scope = choice.scope,
            status = "completed",
            command = choice.command,
            base = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 100.0, 10, 1) }),
            candidate = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 90.0, 10, 1) }))
    val current =
        DesktopState(
            projectState =
                ProjectWorkspaceState(
                    project =
                        performancePageFixture()
                            .project!!
                            .copy(projectId = "project", projectRevision = "revision")),
            selection =
                FileSelectionState(
                    selectedFile =
                        ProjectFileInfo(
                            "main.go",
                            "base",
                            "main.go",
                            language = "Go",
                            sizeBytes = 1,
                            lineCount = 1,
                            modifiedAt = "",
                            binary = false)),
            review =
                DraftReviewState(
                    draft = draft,
                    editor = editableDraft(draft),
                    benchmark = BenchmarkEvidenceState(comparison = comparison)))
    fun withEvidence(evidence: BenchmarkEvidenceState) =
        current.copy(
            review = current.review.copy(benchmark = evidence.copy(comparison = comparison)))
    val cases =
        listOf(
            "missing" to current.copy(projectState = ProjectWorkspaceState()),
            "file-mismatch" to
                current.copy(
                    selection =
                        current.selection.copy(
                            selectedFile = current.selectedFile!!.copy(contentHash = "changed"))),
            "project-mismatch" to
                current.copy(
                    projectState =
                        current.projectState.copy(
                            project = current.project!!.copy(projectRevision = "changed"))),
            "selection-cleared" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog, discovery = BenchmarkDiscoveryOutcome.Loaded)),
            "argv-changed" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog =
                            catalog.copy(
                                benchmarks = listOf(choice.copy(command = choice.command + "-v"))),
                        selected = choice.copy(command = choice.command + "-v"),
                        discovery = BenchmarkDiscoveryOutcome.Loaded)),
            "dirty" to
                current.copy(
                    review =
                        current.review.copy(
                            editor =
                                current.review.editor!!.copy(status = DraftEditorStatus.Dirty))),
            "loading" to
                withEvidence(BenchmarkEvidenceState(discovery = BenchmarkDiscoveryOutcome.Loading)),
            "empty" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog.copy(benchmarks = emptyList()),
                        discovery = BenchmarkDiscoveryOutcome.Loaded)),
            "unavailable" to
                withEvidence(
                    BenchmarkEvidenceState(
                        discovery =
                            BenchmarkDiscoveryOutcome.Unavailable(
                                "The candidate has no compatible benchmark."))),
            "failed" to
                withEvidence(
                    BenchmarkEvidenceState(
                        discovery =
                            BenchmarkDiscoveryOutcome.Failed(
                                "Read-only lookup timed out. Refresh to retry."))),
            "trusted" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog,
                        selected = choice,
                        discovery = BenchmarkDiscoveryOutcome.Loaded)),
            "untrusted" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog.copy(trusted = false),
                        selected = choice,
                        discovery = BenchmarkDiscoveryOutcome.Loaded)),
            "incomplete-choice" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog =
                            catalog.copy(benchmarks = listOf(choice.copy(command = emptyList()))),
                        selected = choice.copy(command = emptyList()),
                        discovery = BenchmarkDiscoveryOutcome.Loaded)),
            "admitting" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog.copy(trusted = false),
                        selected = choice,
                        discovery = BenchmarkDiscoveryOutcome.Loaded,
                        admission = BenchmarkAdmissionOutcome.Admitting)),
            "running" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog,
                        selected = choice,
                        discovery = BenchmarkDiscoveryOutcome.Loaded,
                        admission = BenchmarkAdmissionOutcome.Running)),
            "stale-selection" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog,
                        selected = choice.copy(scope = "old-scope"),
                        discovery = BenchmarkDiscoveryOutcome.Loaded)),
            "admission-failed" to
                withEvidence(
                    BenchmarkEvidenceState(
                        catalog = catalog,
                        selected = choice,
                        discovery = BenchmarkDiscoveryOutcome.Loaded,
                        admission =
                            BenchmarkAdmissionOutcome.Failed(
                                "Comparison timed out; project execution may have begun.")))) +
            listOf("completed", "canceled", "failed", "unavailable", "future-status").map { status
              ->
              "terminal-$status" to
                  withEvidence(
                      BenchmarkEvidenceState(
                          catalog = catalog,
                          selected = choice,
                          discovery = BenchmarkDiscoveryOutcome.Loaded,
                          latestOutcome =
                              BenchmarkComparisonOutcome(
                                  GoBenchmarkComparison(
                                      status = status, reason = "Recorded $status reason"))))
            }
    for ((name, snapshot) in cases) {
      for ((width, height, scale) in listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f))) {
        val evidence = snapshot.review.benchmark
        val eligibility = benchmarkEligibility(snapshot)
        val identity =
            (eligibility.candidate as? BenchmarkCandidateDecision.Ready)
                ?.draft
                ?.let(::goBenchmarkComparisonIdentity)
        val status =
            performanceBenchmarkStatusPresentation(
                comparison,
                identity,
                evidence.selected,
                evidence.discovery,
                evidence.admission,
                eligibility,
                evidence.catalog,
                evidence.latestOutcome)
        var actions = 0
        ComposeVisualFixture(width, height, scale) {
              PerformanceWorkspacePane(
                  PerformanceWorkspacePaneState(
                      performancePageFixture(),
                      resultIndexFixture(),
                      benchmarkComparison = comparison,
                      expectedBenchmarkIdentity = identity,
                      benchmarkCatalog = evidence.catalog,
                      selectedBenchmark = evidence.selected,
                      benchmarkDiscovery = evidence.discovery,
                      benchmarkAdmission = evidence.admission,
                      benchmarkLatestOutcome = evidence.latestOutcome,
                      benchmarkEligibility = eligibility),
                  PerformanceWorkspaceActions(
                      {},
                      {},
                      FindingActions({}, { _, _ -> }, {}),
                      openSource = {},
                      loadBenchmarks = { actions++ },
                      selectBenchmark = { actions++ },
                      runBenchmark = { actions++ }))
            }
            .use { fixture ->
              fixture.render()
              assertTrue(fixture.hasText(status.stateLabel))
              fixture.clickDescription("Expand Explore benchmark evidence")
              fixture.render("f21-discovery-$name-$width-$height-$scale")
              assertTrue(fixture.hasText(status.summary))
              assertEquals(name !in listOf("trusted", "untrusted"), status.priorEvidence)
              val detailsLabel =
                  if (status.priorEvidence) "Prior measurement details" else "Measurement details"
              assertTrue(fixture.hasText(detailsLabel))
              assertFalse(
                  fixture.hasText("Prior benchmark evidence") ||
                      fixture.hasText("Benchmark evidence"),
                  "Measurement details are optional and collapsed")
              eligibility.discoveryBlockedReason?.let { assertTrue(fixture.hasText(it)) }
              val recovery =
                  if (evidence.discovery == BenchmarkDiscoveryOutcome.NotRequested)
                      "List compatible benchmarks"
                  else "Refresh compatible benchmarks"
              val active = evidence.running
              assertEquals(!eligibility.canDiscover || active, fixture.isDisabled(recovery))
              if (evidence.selected != null) {
                val required =
                    listOf(
                        "Selected benchmark: BenchmarkRun",
                        "Project ID: project",
                        "Project revision: revision",
                        "Target path: main.go",
                        "Validated draft revision: 1",
                        "Package working directory: .",
                        "Opaque scope guard (identity metadata): ${evidence.selected.scope}")
                required.forEach { text ->
                  fixture.revealTextFullyWithin(text, "result-overview")
                  fixture.assertTextFits(text, maxLines = 3)
                }
                val argv = performanceBenchmarkArgv(evidence.selected.command)
                if (argv.isNotEmpty()) {
                  argv.lines().forEach { argument ->
                    fixture.revealTextFullyWithin(argument, "result-overview")
                    fixture.assertTextFits(argument, maxLines = 3)
                  }
                } else {
                  assertTrue(fixture.hasText("No argv returned; execution is blocked."))
                }
                for (text in
                    listOf(
                        "Running benchmarks executes imported project code.",
                        "Execution may have external effects, including file and network access.",
                        "Baseline and candidate use copied workspaces; these are not a security sandbox.",
                        "Trust contract (separate from selected argv): go test ./...",
                        "This is broader than benchmark-only permission.",
                        "Trust lasts for this project revision in the daemon session.",
                        "Granting trust does not execute “go test ./...”.",
                        "The combined action requests the selected benchmark separately.")) {
                  fixture.revealTextFullyWithin(text, "result-overview")
                  fixture.assertTextFits(text, maxLines = 15)
                }
                val runLabel =
                    when (evidence.admission) {
                      BenchmarkAdmissionOutcome.Admitting -> "Checking execution trust…"
                      BenchmarkAdmissionOutcome.Running -> "Comparing benchmark…"
                      else ->
                          if (evidence.catalog!!.trusted) "Run selected benchmark"
                          else "Trust and run selected benchmark"
                    }
                fixture.revealTextFullyWithin(runLabel, "result-overview")
                assertEquals(!eligibility.canCompare || active, fixture.isDisabled(runLabel))
                fixture.render("f21-admission-$name-$width-$height-$scale")
              }
              fixture.revealTextFullyWithin(recovery, "result-overview")
              fixture.assertTextFits(recovery)
              fixture.revealTextFullyWithin(detailsLabel, "result-overview")
              fixture.clickText(detailsLabel)
              fixture.render("f21-prior-$name-$width-$height-$scale")
              assertTrue(
                  fixture.hasText(
                      if (status.priorEvidence) "Prior benchmark evidence"
                      else "Benchmark evidence"))
              assertTrue(
                  fixture.hasText("BenchmarkRun"), "Retained measurement values remain readable")
              val assessment =
                  performanceBenchmarkPresentation(
                      comparison,
                      identity,
                      evidence.selected.takeIf { eligibility.canCompare },
                      status.priorEvidence)
              assertTrue(fixture.hasText(assessment.stateLabel))
              fixture.revealTextFullyWithin(assessment.summary, "result-overview")
              fixture.assertTextFits(assessment.summary, maxLines = 15)
              if (status.priorEvidence) {
                assertFalse(fixture.hasText("Measured · selected benchmark"))
                assertFalse(fixture.hasText("CPU is lower for the selected benchmark."))
                assertFalse(fixture.hasText("Measured trade-offs"))
              }
              fixture.render("f22-outcome-details-$name-$width-$height-$scale")
              assertEquals(0, actions, "Rendering and disclosure must not discover, select or run")
            }
      }
    }
  }

  @Test
  fun measurementFreeTerminalResponsesExposeTheirOwnDetailsAlongsidePriorEvidence() {
    val prior =
        GoBenchmarkComparison(
            benchmark = "BenchmarkPrior",
            status = "completed",
            command = listOf("go", "test", "prior", "-count=5"),
            base = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1000, 100.0, 10, 1) }),
            candidate = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1000, 80.0, 0, 0) }))
    val responses =
        listOf("completed", "canceled", "failed", "unavailable", "future-status").map { status ->
          GoBenchmarkComparison(
              draftId = "response-draft",
              draftRevision = 3,
              draftHash = "response-hash",
              projectId = "response-project",
              projectRevision = "response-revision",
              targetPath = "internal/日本語/response.go",
              baseFileHash = "response-base",
              benchmark = "BenchmarkResponse",
              scope = "response-scope",
              status = status,
              reason = "Recorded $status reason",
              command = listOf("go", "test", "response", "-count", "5", "-benchmem"),
              base = GoBenchmarkMeasurement(),
              candidate = null)
        } + GoBenchmarkComparison(status = "unavailable", reason = "Sparse response reason")
    for ((index, response) in responses.withIndex()) {
      for (retainPrior in listOf(false, true)) {
        for ((width, height, scale) in listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f))) {
          val initial =
              if (retainPrior)
                  DesktopState().reduce(DesktopEvent.GoBenchmarkComparisonLoaded(prior))
              else DesktopState()
          val evidence =
              initial
                  .reduce(DesktopEvent.GoBenchmarkComparisonStarted)
                  .reduce(DesktopEvent.GoBenchmarkComparisonLoaded(response))
                  .review
                  .benchmark
          assertEquals(if (retainPrior) prior else null, evidence.comparison)
          assertEquals(response, evidence.latestOutcome!!.response)
          var requests = 0
          ComposeVisualFixture(width, height, scale) {
                PerformanceWorkspacePane(
                    PerformanceWorkspacePaneState(
                        performancePageFixture(),
                        null,
                        benchmarkComparison = evidence.comparison,
                        benchmarkLatestOutcome = evidence.latestOutcome),
                    PerformanceWorkspaceActions(
                        { requests++ },
                        { requests++ },
                        FindingActions({ requests++ }, { _, _ -> requests++ }, { requests++ }),
                        openSource = { requests++ },
                        loadBenchmarks = { requests++ },
                        selectBenchmark = { requests++ },
                        runBenchmark = { requests++ }))
              }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.hasText(response.reason))
                fixture.clickDescription("Expand Explore benchmark evidence")
                fixture.render()
                assertTrue(
                    fixture.hasText("Latest response details"),
                    "Every terminal response is inspectable")
                fixture.revealTextFullyWithin("Latest response details", "result-overview")
                fixture.clickText("Latest response details")
                fixture.render()
                assertTrue(fixture.hasText("Latest comparison response"))
                assertFalse(fixture.hasText("Baseline median"), "No measurements are invented")
                for (label in listOf("Recorded conditions & identity", "Returned sample details")) {
                  fixture.revealTextFullyWithin(label, "result-overview")
                  fixture.clickText(label)
                  fixture.render()
                }
                for ((label, value) in
                    performanceBenchmarkRecordedRows(response) +
                        performanceBenchmarkSampleRows(response)) {
                  val text = "$label: $value"
                  fixture.revealTextFullyWithin(text, "result-overview")
                  fixture.assertTextFits(text, maxLines = 20)
                }
                fixture.revealTextFullyWithin("Copy displayed response details", "result-overview")
                fixture.clickText("Copy displayed response details")
                fixture.render("f22-latest-response-$index-$retainPrior-$width-$height-$scale")
                val copied = fixture.clipboardText()
                assertEquals(performanceBenchmarkResponseCopyText(response, true, true), copied)
                assertTrue(copied.startsWith("Latest comparison response"))
                assertTrue(copied.contains("Daemon status: ${response.status}"))
                assertTrue(copied.contains("Daemon reason: ${response.reason}"))
                for ((label, value) in
                    performanceBenchmarkRecordedRows(response) +
                        performanceBenchmarkSampleRows(response)) {
                  assertTrue(copied.contains("$label: $value"))
                }
                assertFalse(
                    copied.contains("BenchmarkPrior"), "Do not substitute prior identity or argv")
                assertFalse(copied.contains("Baseline median"))
                fixture.revealTextFullyWithin("Latest response details", "result-overview")
                fixture.clickText("Latest response details")
                fixture.render()
                if (retainPrior) {
                  fixture.revealTextFullyWithin("Prior measurement details", "result-overview")
                  fixture.clickText("Prior measurement details")
                  fixture.render()
                  assertTrue(fixture.hasText("Prior benchmark evidence"))
                  fixture.revealTextFullyWithin("100 ns/op", "result-overview")
                  fixture.assertTextFits("100 ns/op")
                  assertFalse(fixture.hasText("Measured · selected benchmark"))
                } else {
                  assertFalse(fixture.hasText("Prior measurement details"))
                }
                assertEquals(0, requests, "Response and prior evidence inspection remain passive")
              }
        }
      }
    }
  }

  @Test
  fun longBenchmarkCatalogAdmissionAndDiagnosticsUseOneReachableOverview() {
    val path = "internal/" + "日本語-équipe-δοκιμή/".repeat(10) + "work.go"
    val draft =
        DeclarationDraft(
            id = "draft",
            revision = 7,
            hash = "candidate",
            projectId = "project-" + "équipe日本語".repeat(12),
            projectRevision = "revision-" + "abcdef0123456789".repeat(12),
            baseFileHash = "base",
            targetPath = path,
            declaration = "func Run() {}",
            validation =
                DeclarationValidation(true, "strict_symbol", diff = UnifiedDiff(path, path)))
    val choices =
        List(24) { number ->
          val name = "Benchmark${number}_" + "日本語Workload".repeat(10)
          GoBenchmarkChoice(
              name,
              listOf(
                  "go",
                  "test",
                  ".",
                  "-run",
                  "^$",
                  "-bench",
                  "^$name$",
                  "-count",
                  "5",
                  "-benchtime",
                  "100ms",
                  "-benchmem",
                  "-timeout",
                  "15s",
                  "long argument with spaces 日本語-équipe ".repeat(12)),
              "opaque:" + "日本語abcdef0123456789".repeat(12))
        }
    val catalog =
        GoBenchmarkCatalog(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            targetPath = path,
            available = true,
            benchmarks = choices)
    val current =
        DesktopState(
            projectState =
                ProjectWorkspaceState(
                    project =
                        performancePageFixture()
                            .project!!
                            .copy(
                                projectId = draft.projectId,
                                projectRevision = draft.projectRevision)),
            selection =
                FileSelectionState(
                    selectedFile =
                        ProjectFileInfo(
                            path,
                            "base",
                            "work.go",
                            language = "Go",
                            sizeBytes = 1,
                            lineCount = 1,
                            modifiedAt = "",
                            binary = false)),
            review =
                DraftReviewState(
                    draft = draft,
                    editor = editableDraft(draft),
                    benchmark =
                        BenchmarkEvidenceState(
                            catalog = catalog, discovery = BenchmarkDiscoveryOutcome.Loaded)))
    val diagnostic =
        "Comparison timed out; execution may have begun.\n" +
            (1..18).joinToString("\n") {
              "Diagnostic $it · 日本語-équipe · retry only after reviewing the current candidate."
            } +
            "\nRefresh compatible benchmarks to review the scope again."
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        for (density in
            if (scale == 1.5f && width in listOf(800, 1280)) listOf(1f, 2f) else listOf(1f)) {
          var snapshot by mutableStateOf(current)
          var selections = 0
          var discoveries = 0
          var executions = 0
          val label = "f21-long-$width-$height-$scale-${density}x"
          val page = performancePageFixture()
          val browser = newResultBrowserState(page)
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    val evidence = snapshot.review.benchmark
                    PerformanceWorkspacePane(
                        PerformanceWorkspacePaneState(
                            performancePageFixture(),
                            resultIndexFixture(),
                            expectedBenchmarkIdentity = goBenchmarkComparisonIdentity(draft),
                            benchmarkCatalog = evidence.catalog,
                            selectedBenchmark = evidence.selected,
                            benchmarkDiscovery = evidence.discovery,
                            benchmarkAdmission = evidence.admission,
                            benchmarkEligibility = benchmarkEligibility(snapshot),
                            browser = browser),
                        PerformanceWorkspaceActions(
                            { error("Layout prepared a fix") },
                            { error("Layout opened analysis") },
                            FindingActions(
                                { error("Layout prepared a fix") },
                                { _, _ -> error("Layout changed triage") },
                                { error("Layout opened source") }),
                            openSource = { error("Layout opened source") },
                            loadBenchmarks = { discoveries++ },
                            selectBenchmark = { choice ->
                              selections++
                              snapshot =
                                  snapshot.copy(
                                      review =
                                          snapshot.review.copy(
                                              benchmark =
                                                  snapshot.review.benchmark.copy(
                                                      selected = choice)))
                            },
                            runBenchmark = { executions++ }))
                  }
              .use { fixture ->
                fixture.render()
                fixture.clickDescription("Expand Explore benchmark evidence")
                fixture.render("$label-catalog")
                assertTrue(fixture.hasText("Select one listed benchmark before comparing."))
                assertFalse(fixture.hasText("Daemon-returned argv (read-only)"))
                fixture.assertTextOrder(choices.map { "Select · ${it.name}" })
                for (choice in choices) {
                  fixture.revealTextFullyWithin("Select · ${choice.name}", "result-overview")
                  fixture.assertTextFits("Select · ${choice.name}", maxLines = 10)
                }
                assertEquals(0, selections, "Scrolling does not select")
                fixture.clickText("Select · ${choices.last().name}")
                fixture.render("$label-selected")
                assertEquals(1, selections)
                val required =
                    performanceBenchmarkAdmissionRows(
                            choices.last(), benchmarkEligibility(snapshot).candidate)
                        .map { (key, value) -> "$key: $value" } +
                        performanceBenchmarkArgv(choices.last().command).lines() +
                        listOf(
                            "Running benchmarks executes imported project code.",
                            "Execution may have external effects, including file and network access.",
                            "Baseline and candidate use copied workspaces; these are not a security sandbox.",
                            "Trust contract (separate from selected argv): go test ./...",
                            "This is broader than benchmark-only permission.",
                            "Granting trust does not execute “go test ./...”.")
                required.forEach { fixture.assertEveryTextLineReachable(it, "result-overview") }
                fixture.revealTextFullyWithin("Trust and run selected benchmark", "result-overview")
                fixture.assertTextFits("Trust and run selected benchmark")
                assertFalse(fixture.isDisabled("Trust and run selected benchmark"))
                fixture.render("$label-untrusted-action")
                snapshot =
                    snapshot.copy(
                        review =
                            snapshot.review.copy(
                                benchmark =
                                    snapshot.review.benchmark.copy(
                                        catalog = catalog.copy(trusted = true))))
                fixture.render()
                fixture.revealTextFullyWithin("Run selected benchmark", "result-overview")
                assertFalse(fixture.isDisabled("Run selected benchmark"))
                fixture.render("$label-trusted-action")
                snapshot =
                    snapshot.copy(
                        review =
                            snapshot.review.copy(
                                benchmark =
                                    snapshot.review.benchmark.copy(
                                        admission = BenchmarkAdmissionOutcome.Failed(diagnostic))))
                fixture.render()
                fixture.assertEveryTextLineReachable(diagnostic, "result-overview")
                fixture.render("$label-diagnostic-tail")
                fixture.revealTextFullyWithin("Refresh compatible benchmarks", "result-overview")
                assertFalse(fixture.isDisabled("Refresh compatible benchmarks"))
                fixture.render("$label-recovery")
                fixture.resize((1024 * density).toInt(), (768 * density).toInt())
                fixture.render()
                fixture.revealTextFullyWithin("Run selected benchmark", "result-overview")
                fixture.render("$label-resized-action")
                assertFalse(
                    fixture.hasText("Benchmark evidence"),
                    "Optional measurement help stays collapsed")
                assertEquals(1, selections)
                assertEquals(0, discoveries, "Disclosure, scrolling and resize are read-only")
                assertEquals(0, executions, "Reachability must not activate execution")
              }
        }
      }
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
  fun compactMetadataKeepsLongUnicodeValuesReadableAndSelectableAtLargeText() {
    val destination = "internal/über/日本語/very-long-destination/".repeat(3) + "résumé.go"
    listOf(320, 480).forEach { width ->
      var actions = 0
      ComposeVisualFixture(width, 440, 1.5f) {
            Column(Modifier.fillMaxWidth().background(Panel).padding(8.dp)) {
              CompactKeyValueRows(listOf("Destination" to destination, "Status" to "Available"))
              ChromeButton(onClick = { actions++ }) { Text("Run") }
            }
          }
          .use { fixture ->
            fixture.render("compact-metadata-$width-150")
            assertTrue(fixture.hasDescription("Destination: $destination"))
            fixture.assertTextWrapsWithoutClipping(destination)
            fixture.assertTextFits("Available")
            assertFalse(fixture.hasEditableText())
            val copied = fixture.copyTextByDragging(destination)
            assertTrue(copied.isNotEmpty() && destination.contains(copied), "Copied: $copied")
            assertEquals(0, actions, "Selecting metadata must not activate an action")
          }
    }
  }

  @Test
  fun diagnosticDisclosureKeepsSanitizedAvailableOutputSelectableAndBounded() {
    val raw =
        "start \u0000 diagnostic\n" + "more evidence\n".repeat(420) + "END OF AVAILABLE OUTPUT"
    val cleaned = raw.replace('\u0000', ' ')
    var value by mutableStateOf(raw)
    var work = 0
    ComposeVisualFixture(360, 500, 1.5f) {
          Column {
            DiagnosticText(value)
            ChromeButton(onClick = { work++ }) { Text("Run checks") }
          }
        }
        .use { fixture ->
          fixture.render("diagnostic-preview-360-150")
          val preview = cleaned.take(4_096)
          assertTrue(fixture.hasText(preview))
          assertTrue(fixture.hasText("… output truncated"))
          assertFalse(fixture.hasText(sanitizedOutputText(raw)))
          assertFalse(fixture.hasText(cleaned))
          val previewCopy = fixture.copyTextByDragging(preview)
          assertTrue(
              previewCopy.isNotEmpty() && preview.startsWith(previewCopy), "Copied: $previewCopy")
          assertFalse(previewCopy.contains("… output truncated"))
          assertFalse(previewCopy.contains('\u0000'))
          assertEquals(0, work, "Copying the preview must not start checks")
          assertEquals(
              "Collapsed",
              fixture.descriptionStateDescription("Expand available diagnostic output"))
          assertTrue(fixture.requestDescriptionFocus("Expand available diagnostic output"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render("diagnostic-expanded-360-150")
          assertEquals(
              "Expanded",
              fixture.descriptionStateDescription("Collapse available diagnostic output"))
          assertTrue(fixture.hasText(cleaned))
          assertFalse(fixture.hasText(raw))
          assertFalse(fixture.hasText(sanitizedOutputText(raw)))
          assertTrue(fixture.taggedBounds("diagnostic-output-scroll").height <= 360f)
          val copied = fixture.copyTextByDragging(cleaned)
          assertTrue(copied.isNotEmpty() && cleaned.contains(copied), "Copied: $copied")
          assertFalse(copied.contains('\u0000'))
          assertFalse(copied.contains("Show full available output"))
          fixture.scrollBy(100_000f, "diagnostic-output-scroll")
          fixture.render("diagnostic-tail-360-150")
          assertTrue(fixture.verticalScrollValue("diagnostic-output-scroll") > 0f)
          assertEquals(0, work)

          assertTrue(fixture.requestDescriptionFocus("Collapse available diagnostic output"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(
              "Collapsed",
              fixture.descriptionStateDescription("Expand available diagnostic output"))
          assertTrue(fixture.hasText(preview))
          assertTrue(fixture.hasText("… output truncated"))
          assertEquals(0, work)
          fixture.clickDescription("Expand available diagnostic output")
          fixture.render()
          assertTrue(fixture.hasText(cleaned))

          value = "replacement \u0001 output" + "x".repeat(4_100)
          fixture.render()
          assertEquals(
              "Collapsed",
              fixture.descriptionStateDescription("Expand available diagnostic output"))
          assertTrue(fixture.hasText("… output truncated"))
          assertFalse(fixture.hasText(sanitizedOutputText(value)))
          assertFalse(fixture.hasText(cleaned))
          assertEquals(0, work)
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
  fun compactFieldsKeepRequiredErrorsVisibleAndAnnouncedAcrossEditingAndResizing() {
    var search by mutableStateOf("")
    var notes by mutableStateOf(TextFieldValue(""))
    var searchError by mutableStateOf<String?>(null)
    var notesError by mutableStateOf<String?>(null)
    ComposeVisualFixture(320, 370, 1.5f) {
          Column(Modifier.fillMaxWidth().background(Panel).padding(8.dp)) {
            CompactSingleLineField(
                search,
                { search = it },
                "Search findings",
                showLabel = false,
                helperText = "Optional search hint",
                errorText = searchError)
            CompactMultilineField(
                notes,
                { notes = it },
                "Notes",
                minLines = 2,
                helperText = "Optional note hint",
                errorText = notesError)
          }
        }
        .use { fixture ->
          fixture.render("fields-help-320-150")
          assertTrue(fixture.hasText("Optional search hint"))
          assertTrue(fixture.hasText("Optional note hint"))
          assertEquals(null, fixture.descriptionError("Search findings"))
          searchError = "Search must contain a project path"
          notesError = "Notes must explain why the change is safe across lines and narrow windows"
          fixture.render("fields-invalid-320-150")
          fixture.assertTextFits("Search must contain a project path", maxLines = 2)
          fixture.assertTextWrapsWithoutClipping(notesError!!)
          assertFalse(fixture.hasText("Optional search hint"))
          assertFalse(fixture.hasText("Optional note hint"))
          assertEquals(searchError, fixture.descriptionError("Search findings"))
          assertEquals(notesError, fixture.descriptionError("Notes"))
          fixture.setTextForDescription("Search findings", "résumé")
          fixture.render()
          assertEquals("résumé", search)
          assertTrue(fixture.hasDescription("Search findings"))
          assertEquals(searchError, fixture.descriptionError("Search findings"))
          fixture.resize(450, 370)
          fixture.render("fields-invalid-resized-450-150")
          fixture.assertTextFits("Search must contain a project path", maxLines = 2)
          assertEquals(notesError, fixture.descriptionError("Notes"))
          searchError = null
          notesError = null
          fixture.render()
          assertEquals(null, fixture.descriptionError("Search findings"))
          assertTrue(fixture.hasText("Optional search hint"))
        }
  }

  @Test
  fun compactSingleLineFieldRetainsLongInputAndCaretAtLargeText() {
    val longValue = "résumé/日本語/" + "very-long-search-segment/".repeat(8)
    var value by mutableStateOf(longValue)
    ComposeVisualFixture(210, 110, 1.5f) {
          CompactSingleLineField(
              value,
              { value = it },
              "Search",
              showLabel = false,
              placeholder = "Find a declaration",
              errorText = "Choose a shorter query")
        }
        .use { fixture ->
          fixture.render("field-long-single-line-210-150")
          fixture.assertTextFits("Choose a shorter query", maxLines = 2)
          assertTrue(fixture.editorTextWidth("Search") > 210f)
          fixture.focusDescribedEditor("Search")
          fixture.selectEditorText("Search", longValue.length, longValue.length)
          fixture.render("field-long-single-line-end-210-150")
          assertEquals(longValue, value)
          assertEquals(longValue.length, fixture.editorSelectionEnd("Search"))
        }
  }

  @Test
  fun compactFieldsPreserveTextEditingSelectionAndDisabledPresentation() {
    var query by mutableStateOf("Initial search")
    var notes by mutableStateOf(TextFieldValue("First line\nSecond line"))
    var enabled by mutableStateOf(true)
    ComposeVisualFixture(280, 320, 1.5f) {
          Column(Modifier.fillMaxWidth().background(Panel).padding(8.dp)) {
            CompactSingleLineField(
                query,
                { query = it },
                "Filter",
                showLabel = false,
                enabled = enabled,
                errorText = "Invalid filter")
            CompactMultilineField(
                notes,
                { notes = it },
                "Details",
                enabled = enabled,
                minLines = 2,
                errorText = "Invalid details")
          }
        }
        .use { fixture ->
          fixture.render()
          fixture.focusDescribedEditor("Filter")
          fixture.render()
          assertTrue(fixture.isDescriptionFocused("Filter"))
          fixture.pressKey(Key.Spacebar)
          fixture.render()
          assertTrue(query.contains(' '), "Space must enter text, not activate a parent")
          fixture.setTextForDescription("Details", "First line\nSecond line\nThird line")
          fixture.render("fields-multiline-edited-280-150")
          assertEquals("First line\nSecond line\nThird line", notes.text)
          fixture.focusDescribedEditor("Details")
          val lineBreaks = notes.text.count { it == '\n' }
          fixture.pressKey(Key.Enter)
          fixture.render()
          assertEquals(lineBreaks + 1, notes.text.count { it == '\n' }, "Enter inserts a line")
          assertTrue(fixture.hasDescription("Details"))
          assertEquals("Invalid details", fixture.descriptionError("Details"))
          fixture.selectEditorText("Details", 6, 10)
          fixture.render()
          assertEquals(6, notes.selection.start)
          assertEquals(10, notes.selection.end)
          assertTrue(fixture.pressKey(Key.Copy))
          assertEquals("line", fixture.clipboardText())
          fixture.resize(360, 320)
          fixture.render()
          assertEquals(6, notes.selection.start)
          assertEquals(10, notes.selection.end)
          enabled = false
          fixture.render("fields-disabled-360-150")
          assertTrue(fixture.isDescriptionDisabled("Filter"))
          assertTrue(fixture.isDescriptionDisabled("Details"))
          assertEquals("Invalid filter", fixture.descriptionError("Filter"))
          assertEquals("Invalid details", fixture.descriptionError("Details"))
        }
  }

  @Test
  fun selectedDeclarationShowsIdentityAndExplicitActionsWithoutFileTabs() {
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
            listOf("Actions", "File analysis").forEach { assertFalse(fixture.hasText(it), it) }
            assertTrue(fixture.hasText("File details"))
            assertTrue(fixture.hasText("Project context"))
            assertFalse(fixture.hasText("Routes incoming requests."))
            assertFalse(fixture.hasText("Go service with a small HTTP API and a repository layer."))
            fixture.assertTextFits(initial.inspector!!.file.path)
            assertTrue(fixture.hasText("Line range unavailable · Go"))
            assertTrue(fixture.hasText("Exact atomic target · editable"))
            assertTrue(fixture.hasText("Source/index comparison unavailable"))
            assertTrue(fixture.hasText("Line range unavailable"))
            assertFalse(fixture.hasText("func Run() error"))
            fixture.clickText("Declaration details")
            fixture.render()
            assertTrue(fixture.hasText("func Run() error"))
            fixture.clickText("File details")
            fixture.render("context-file-details-$width-$scale")
            fixture.revealText("Routes incoming requests.")
            fixture.assertTextFits("Routes incoming requests.")
            fixture.clickText("Project context")
            fixture.render("context-project-details-$width-$scale")
            fixture.revealText("Go service with a small HTTP API and a repository layer.")
            if (width < 480) {
              fixture.assertTextWrapsWithoutClipping(
                  "Go service with a small HTTP API and a repository layer.")
            } else {
              fixture.assertTextFits("Go service with a small HTTP API and a repository layer.")
            }
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
            state =
                state.copy(
                    fileAnalysis =
                        state.fileAnalysis!!.copy(
                            symbolExplanations = mapOf("Stop" to "Stops the worker.")))
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
  fun indexedReferencesStayReachableInCompactContextWithoutDispatch() {
    val path = "internal/" + "deep/".repeat(18) + "caller.go"
    val reason = "Indexed relationship via " + "an adapter and a helper ".repeat(12)
    for ((width, scale) in listOf(280 to 1f, 320 to 1.5f, 480 to 1.25f)) {
      var actions = 0
      val initial = contextVisualState()
      val state =
          initial.copy(
              impact =
                  ImpactPreview(
                      initial.inspector!!.file.path,
                      references =
                          listOf(
                              ImpactReference(path, "CallRun", "approximate", reason),
                              ImpactReference("other/consumer.go", "", "exact", "Indexed use."))))
      ComposeVisualFixture(width, 650, scale) {
            ContextToolWindow(
                state,
                ContextToolWindowActions(
                    { actions++ },
                    { actions++ },
                    { actions++ },
                    { actions++ },
                    { actions++ },
                    explainSelected = { actions++ }))
          }
          .use { fixture ->
            fixture.render("context-references-collapsed-$width-$scale")
            assertTrue(
                fixture.hasText(
                    "File-scoped reference preview · advisory indexed relationships, not runtime callers."))
            assertFalse(fixture.hasText(path))
            fixture.revealText("References", "context-content")
            fixture.clickText("References")
            fixture.render("context-references-expanded-$width-$scale")
            for (label in
                listOf(
                    path,
                    "CallRun",
                    "Confidence · approximate",
                    reason,
                    "other/consumer.go",
                    "Confidence · exact",
                    "Indexed use.")) {
              fixture.revealText(label, "context-content")
              assertTrue(fixture.hasText(label))
            }
            fixture.assertTextWrapsWithoutClipping(reason)
            assertEquals(0, actions)
          }
    }
  }

  @Test
  fun declarationDescriptionPreservesConsentCancellationAndLifecycleEvidence() {
    var requests = 0
    var cancellations = 0
    var refactors = 0
    val longModel = "provider/" + "long-destination/".repeat(8) + "editor"
    val remoteModel =
        ScopedModel(
            scope = "function", profile = "editor", model = longModel, remoteProvider = true)
    var state by
        mutableStateOf(
            contextVisualState()
                .copy(
                    functionModel = remoteModel,
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
                  { refactors++ },
                  confirmFunctionRemoteProvider = {
                    state = state.copy(functionRemoteProviderConfirmed = it)
                  },
                  explainSelected = { requests++ },
                  cancelExplanation = { cancellations++ }),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          fixture.render("context-explain-consent")
          val destination = modelDestinationLabel(ModelScope.Function, remoteModel)
          fixture.revealText(destination)
          fixture.assertTextWrapsWithoutClipping(destination)
          fixture.assertTextAboveDescription(destination, "Confirm remote destination")
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertTrue(fixture.isDisabled("Explain declaration"))
          assertTrue(fixture.hasText("Cached declaration explanation"))
          fixture.clickText("Refactor")
          assertEquals(1, refactors)
          fixture.clickVisibleDescription("Confirm remote destination")
          fixture.render()
          assertEquals(0, requests)
          assertTrue(fixture.hasText(destination))
          fixture.clickText("Explain declaration")
          assertEquals(1, requests)
          state =
              state.copy(
                  functionRemoteProviderConfirmed = false,
                  declarationExplanation =
                      DeclarationExplanationState(
                          status = DeclarationExplanationStatus.Loading,
                          target = contextExplanationTarget(state)))
          fixture.render("context-explain-loading")
          fixture.revealText(destination)
          fixture.assertTextAboveDescription(destination, "Confirm remote destination")
          assertTrue(fixture.isDescriptionDisabled("Confirm remote destination"))
          fixture.assertTextFits("Explaining…")
          assertTrue(fixture.hasText("Explanation in progress. Cancel to stop this request."))
          assertTrue(fixture.hasText("Saved file analysis · Fresh"))
          fixture.assertTextFits("Cancel explanation")
          assertFalse(fixture.isDisabled("Cancel explanation"))
          fixture.clickText("Cancel explanation")
          assertEquals(1, cancellations)
          state =
              state.copy(
                  functionRemoteProviderConfirmed = true,
                  declarationExplanation =
                      DeclarationExplanationState(
                          status = DeclarationExplanationStatus.Current,
                          target = contextExplanationTarget(state),
                          result = result))
          fixture.render("context-explain-current")
          fixture.revealText(destination)
          fixture.assertTextAboveDescription(destination, "Confirm remote destination")
          assertTrue(fixture.hasText(result.summary))
          fixture.assertTextFits("Refresh explanation")
          fixture.assertTextFits("Current explanation")
          assertTrue(
              fixture.hasText(
                  "On-demand · matches the selected loaded source and declaration; not a disk check."))
          state =
              state.copy(
                  declarationExplanation =
                      state.declarationExplanation.copy(
                          status = DeclarationExplanationStatus.Stale))
          fixture.render("context-explain-stale")
          assertFalse(fixture.hasText(result.summary))
          assertTrue(fixture.hasText("Source or selection changed. Request a new explanation."))
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
          assertTrue(fixture.hasText("Saved file analysis · Fresh"))
          assertFalse(fixture.hasText(result.summary))
          state =
              state.copy(
                  declarationExplanation =
                      state.declarationExplanation.copy(
                          status = DeclarationExplanationStatus.Canceled,
                          message = "Explanation canceled."))
          fixture.render("context-explain-canceled")
          fixture.assertTextFits("Explanation canceled")
          assertTrue(fixture.hasText("Explanation canceled."))
          assertEquals(1, requests)
          assertEquals(1, cancellations)
          assertEquals(1, refactors)
          state =
              state.copy(
                  functionModel = remoteModel.copy(remoteProvider = false),
                  functionRemoteProviderConfirmed = false)
          fixture.render("context-explain-local")
          assertTrue(
              fixture.hasText(modelDestinationLabel(ModelScope.Function, state.functionModel)))
          assertFalse(fixture.hasText("Confirm remote destination"))
          assertFalse(fixture.isDisabled("Explain declaration"))
        }
  }

  @Test
  fun compactExplanationLifecycleKeepsRecoveryReachableWithHelpCollapsed() {
    val base = contextVisualState()
    for ((width, scale) in listOf(280 to 1.5f, 480 to 1f)) {
      var state by mutableStateOf(base)
      var requests = 0
      ComposeVisualFixture(width, 400, scale) {
            ContextToolWindow(
                state,
                ContextToolWindowActions(
                    {},
                    {},
                    {},
                    {},
                    {},
                    explainSelected = { requests++ },
                    cancelExplanation = { requests++ }),
                Modifier.fillMaxSize())
          }
          .use { fixture ->
            for (status in DeclarationExplanationStatus.entries) {
              state =
                  base.copy(
                      declarationExplanation =
                          DeclarationExplanationState(
                              status = status,
                              target =
                                  if (status == DeclarationExplanationStatus.Unavailable) null
                                  else contextExplanationTarget(base),
                              result =
                                  if (status == DeclarationExplanationStatus.Current)
                                      DeclarationExplanation(
                                          version = "v1",
                                          projectId = "visual-fixture",
                                          projectRevision = "fixture-revision",
                                          baseFileHash = "fixture-hash",
                                          anchor =
                                              DeclarationSourceAnchor(
                                                  "internal/api/user.go",
                                                  "Run",
                                                  "func Run() error",
                                                  5,
                                                  12),
                                          summary = "Validates the request.",
                                          contextManifest = ContextManifest())
                                  else null,
                              message =
                                  if (status == DeclarationExplanationStatus.Failed)
                                      "Provider connection failed; retry after reconnecting."
                                  else ""))
              fixture.render("f24-lifecycle-$status-$width-$scale")
              val badge = explanationStatusStyle(status).label
              fixture.revealText(badge, "context-content")
              fixture.assertTextFits(badge, maxLines = 3)
              val action = explanationActionLabel(state.declarationExplanation)
              fixture.revealText(action, "context-content")
              fixture.assertTextFits(action)
              if (status == DeclarationExplanationStatus.Failed) {
                fixture.revealText(state.declarationExplanation.message, "context-content")
                fixture.assertTextFits(state.declarationExplanation.message, maxLines = 4)
              }
              assertTrue(fixture.hasText("Refactor"))
              assertEquals(0, requests, "Rendering $status must not retry or cancel")
            }
          }
    }
  }

  @Test
  fun contextHierarchyAndActionsRemainReachableAtCompactAndEditorSizes() {
    val base = contextVisualState()
    val path = "internal/" + "deep/".repeat(12) + "caller.go"
    val reason = "Advisory indexed relationship through " + "multiple adapters ".repeat(10)
    val signature = "func Run(" + "longArgument context.Context, ".repeat(8) + ") error"
    val selected = requireNotNull(base.inspector).selectedSymbol!!
    val inspector = base.inspector.copy(selectedSymbol = selected.copy(signature = signature))
    val remote =
        ScopedModel(
            scope = "function",
            profile = "editor",
            model = "remote/" + "model/".repeat(9),
            remoteProvider = true)
    val state =
        base.copy(
            inspector = inspector,
            functionModel = remote,
            impact =
                ImpactPreview(
                    inspector.file.path,
                    references = listOf(ImpactReference(path, "Run", "approximate", reason))),
            declarationExplanation =
                DeclarationExplanationState(
                    status = DeclarationExplanationStatus.Failed,
                    target = contextExplanationTarget(base.copy(inspector = inspector)),
                    message = "Provider unavailable. Reconnect before retrying."))
    // M10 hierarchy: identity, explanation/consent/actions, then optional local details.
    // The production dock scrolls instead of copying the mock's two-column layout.
    for ((width, scale, density) in
        (listOf(280, 320, 480).flatMap { width ->
          listOf(1f, 1.25f, 1.5f).map { scale -> Triple(width, scale, 1f) }
        } + listOf(Triple(320, 1.25f, 2f), Triple(280, 1.5f, 2f)))) {
      val height = if (width == 480) 650 else 600
      var privileged = 0
      ComposeVisualFixture((width * density).toInt(), (height * density).toInt(), scale, density) {
            ContextToolWindow(
                state,
                ContextToolWindowActions(
                    { privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { privileged++ },
                    confirmFunctionRemoteProvider = { privileged++ },
                    explainSelected = { privileged++ },
                    cancelExplanation = { privileged++ }),
                Modifier.fillMaxSize())
          }
          .use { fixture ->
            fixture.render("f24-context-$width-$height-$scale-${density}x")
            fixture.assertEveryTextLineReachable(inspector.file.path, "context-content")
            fixture.revealText("Declaration details", "context-content")
            fixture.clickText("Declaration details")
            fixture.render()
            fixture.assertEveryTextLineReachable(signature, "context-content")
            val destination = modelDestinationLabel(ModelScope.Function, remote)
            fixture.assertEveryTextLineReachable(destination, "context-content")
            assertTrue(fixture.hasText("Explanation failed"))
            assertTrue(fixture.hasText(state.declarationExplanation.message))
            assertTrue(fixture.isDisabled("Explain declaration"))
            fixture.revealText("Refactor", "context-content")
            fixture.assertTextFits("Refactor")
            fixture.revealText("References", "context-content")
            fixture.clickText("References")
            fixture.render("f24-context-expanded-$width-$scale-${density}x")
            for (text in listOf(path, reason)) {
              fixture.assertEveryTextLineReachable(text, "context-content")
            }
            fixture.revealText("File details", "context-content")
            fixture.clickText("File details")
            fixture.revealText("Project context", "context-content")
            fixture.clickText("Project context")
            fixture.render("f24-context-local-details-$width-$scale-${density}x")
            fixture.revealText("Routes incoming requests.", "context-content")
            assertEquals(0, privileged, "Disclosure and scrolling are passive")
          }
    }
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        val density = if (width == 800 && scale == 1.5f) 2f else 1f
        var privileged = 0
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  AdaptiveProductionEditorFixture(
                      DesktopLayoutState(),
                      false,
                      terminalCollapsed = true,
                      onRequest = { privileged++ },
                      onWrite = { privileged++ },
                      onSourceLine = { privileged++ })
                }
            .use { fixture ->
              fixture.render("f24-editor-context-$width-$height-$scale-${density}x")
              assertTrue(fixture.hasText("Context"))
              assertTrue(fixture.hasText("No explanation yet"))
              assertTrue(fixture.hasText("Declaration details"))
              assertTrue(
                  fixture.hasText("Refactor"),
                  "Refactor present at $width x $height / $scale / ${density}x")
              if (fixture.taggedBounds("context-content").height == 0f) {
                fixture.scrollBy(100_000f, "f04-arrangement")
                fixture.render()
              }
              fixture.revealText("Refactor", "context-content")
              fixture.assertTextFits("Refactor")
              assertEquals(0, privileged, "Composition and layout cannot start workflow work")
            }
      }
    }
  }

  @Test
  fun filledWorkspacePanesKeepFlatCornersGuttersAndFullWidthTerminal() {
    listOf(1600 to 1000, 1440 to 900, 1000 to 760, 999 to 760, 800 to 650, 1280 to 600).forEach {
        (width, height) ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        (if (width >= 1000) listOf(false, true) else listOf(false)).forEach { expanded ->
          ComposeVisualFixture(width, height, scale) {
                EditorVisualFixture(width.toFloat(), terminalExpanded = expanded)
              }
              .use { fixture ->
                fixture.render(
                    "frame-$width-$height-$scale-${if (expanded) "expanded" else "collapsed"}")
                if (width >= 999) fixture.assertWorkspaceFrameGeometry(docked = true)
                fixture.assertTextFits("Terminal")
                fixture.assertTextFits("Analysis · Completed")
                fixture.assertTextFits("user.go")
              }
        }
      }
    }
  }

  @Test
  fun expandedTerminalRemainsDockedAcrossBreakpointAndInShortWindow() {
    listOf(1000 to 760, 999 to 760, 1280 to 600).forEach { (width, height) ->
      ComposeVisualFixture(width, height) {
            EditorVisualFixture(width.toFloat(), terminalExpanded = true)
          }
          .use { fixture ->
            fixture.render("terminal-docked-$width-$height")
            val dock = fixture.taggedBounds("terminal-fixture-dock")
            assertEquals(220f, dock.height, 1f)
            assertTrue(fixture.hasText("Synthetic shell"))
            assertFalse(fixture.hasDescription("Terminal overlay"))
          }
    }
  }

  @Test
  fun gutterHandlesRetainVisibleKeyboardFocusAndCommitResizing() {
    listOf(false, true).forEach { horizontal ->
      var size by mutableStateOf(220f)
      val commits = mutableListOf<Float>()
      val label =
          if (horizontal) "Resize bottom pane. Use Up or Down Arrow."
          else "Resize adjacent panes. Use Left or Right Arrow."
      ComposeVisualFixture(160, 160) {
            Box(Modifier.fillMaxSize().background(ActivityRail)) {
              if (horizontal) HorizontalResizableDivider({ size += it }, { commits += size })
              else ResizableDivider({ size += it }, { commits += size })
            }
          }
          .use { fixture ->
            fixture.render()
            fixture.assertColorVisible(ControlBorder)
            assertTrue(fixture.requestDescriptionFocus(label))
            fixture.render()
            fixture.assertColorVisible(FocusAccent)
            assertTrue(fixture.pressKey(if (horizontal) Key.DirectionUp else Key.DirectionRight))
            fixture.render("frame-${if (horizontal) "horizontal" else "vertical"}-splitter-focus")
            assertEquals(232f, size)
            assertEquals(listOf(232f), commits)
            assertTrue(fixture.pressKey(if (horizontal) Key.DirectionDown else Key.DirectionLeft))
            fixture.render()
            assertEquals(220f, size)
            assertEquals(listOf(232f, 220f), commits)
            fixture.dragDescription(label, if (horizontal) Offset(0f, 40f) else Offset(40f, 0f))
            assertTrue(
                if (horizontal) size < 220f else size > 220f,
                "Pointer resizing must still update the pane")
            fixture.awaitResizeCommit(commits, expectedCount = 3)
            assertEquals(3, commits.size)
            assertEquals(size, commits.last(), "Pointer release must save the current size")
            repeat(20) { drag ->
              val distance = if (drag % 2 == 0) -40f else 40f
              val previousSize = size
              fixture.dragDescription(
                  label, if (horizontal) Offset(0f, distance) else Offset(distance, 0f))
              assertTrue(size != previousSize, "Each pointer drag must resize the pane")
              fixture.awaitResizeCommit(commits, expectedCount = 4 + drag)
              assertEquals(4 + drag, commits.size, "Each release must commit exactly once")
              assertEquals(size, commits.last(), "Each release must save the current size")
            }
          }
    }
  }

  @Test
  fun editorComponentsRenderWithDockedPanesAtEveryWidth() {
    listOf(1440 to 900, 1000 to 760, 999 to 760, 800 to 650, 1280 to 600).forEach { (width, height)
      ->
      val layout = DesktopLayoutState(explorerWidth = 330f, actionWidth = 410f)
      ComposeVisualFixture(width, height) { EditorVisualFixture(width.toFloat(), layout = layout) }
          .use { fixture ->
            fixture.render("editor-$width")
            assertEquals(330f, fixture.taggedBounds("editor-files-dock").width, 1f)
            assertEquals(410f, fixture.taggedBounds("editor-context-dock").width, 1f)
            assertTrue(fixture.hasDescription("Performance tool window, not selected"))
            fixture.assertRailLabelFits("Performance")
            fixture.assertTextFits("user.go")
            fixture.assertTextFits("Files")
            if (width >= 999) fixture.assertTextFits("Context")
            assertEquals(1, fixture.textCount("Files"))
            assertEquals(0, fixture.textCount("Tool windows"))
            assertTrue(fixture.hasDescription("Files tool window"))
            assertTrue(fixture.hasDescription("Tool windows tool window"))
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
          listOf("Source", "src", "…", "user_handler.go", "ServeUser")
              .forEach(fixture::assertTextFits)
          assertTrue(fixture.hasDescription("Project-relative path: $longPath"))
          assertTrue(fixture.hasText("Composed diff unavailable"))
        }
  }

  @Test
  fun keyboardEventsNavigateAndActivateTheProductionRailAndCommandPalette() {
    var activeToolWindow by mutableStateOf(LeftToolWindow.Summary)
    val railFocus = FocusRequester()
    ComposeVisualFixture(120, 650, 1.3f) {
          ToolWindowBar(
              activeToolWindow, { activeToolWindow = it }, Modifier.focusRequester(railFocus), {})
        }
        .use { fixture ->
          fixture.render("rail-keyboard-initial-labeled-1.3")
          railFocus.requestFocus()
          fixture.render()
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render("rail-keyboard-arrow-labeled-1.3")
          assertTrue(fixture.hasDescription("Analysis tool window, not selected, focused"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render("rail-keyboard-activated-labeled-1.3")
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
                onMode = {},
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
  fun paletteModeControlsRetainTheQueryAndKeepSearchFocusedWithBoundedLongResults() {
    var mode by mutableStateOf(PaletteMode.Files)
    var query by mutableStateOf("service")
    var dismissed = false
    val selectedFiles = mutableListOf<String>()
    val selectedSymbols = mutableListOf<String>()
    val files =
        (1..30).map {
          IndexedFile("internal/service/very-long-file-name-$it.go", "$it", "Go", false)
        }
    val displayedFiles =
        commandSearchResults(PaletteMode.Files, "service", files, emptyList(), hasActiveFile = true)
    ComposeVisualFixture(800, 650, 1.5f) {
          CommandPaletteDialog(
              mode = mode,
              query = query,
              onQuery = { query = it },
              onMode = { mode = it },
              files = files,
              symbols =
                  listOf(
                      SymbolInfo(
                          "ServiceHandler",
                          "function",
                          startLine = 12,
                          confidence = "exact",
                          atomicTarget = true)),
              hasActiveFile = true,
              onSelectFile = { selectedFiles += it },
              onSelectSymbol = { selectedSymbols += it.name },
              onSelectAction = {},
              onDismiss = { dismissed = true })
        }
        .use { fixture ->
          fixture.render("palette-long-files-800-1.5")
          assertTrue(fixture.hasText("Files"))
          assertTrue(fixture.hasText("Symbols"))
          assertTrue(fixture.hasText("Commands"))
          assertTrue(fixture.hasScrollableContent())
          repeat(displayedFiles.lastIndex) { assertTrue(fixture.pressKey(Key.DirectionDown)) }
          fixture.render()
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render()
          assertTrue(fixture.hasDescription("File ${displayedFiles.first().path}, selected"))
          fixture.awaitVisibleDescription("File ${displayedFiles.first().path}, selected")
          assertTrue(fixture.pressKey(Key.DirectionUp))
          fixture.render()
          assertTrue(fixture.hasDescription("File ${displayedFiles.last().path}, selected"))
          fixture.awaitVisibleDescription("File ${displayedFiles.last().path}, selected")
          fixture.clickVisibleDescription("File ${displayedFiles.last().path}, selected")
          assertEquals(listOf(displayedFiles.last().path), selectedFiles)
          fixture.clickText("Symbols")
          fixture.render("palette-symbols-800-1.5")
          assertEquals(PaletteMode.Symbols, mode)
          assertEquals("service", query)
          assertTrue(fixture.isDescriptionFocused("Filter active-file symbols"))
          fixture.clickText("Symbols")
          fixture.render()
          assertTrue(fixture.isDescriptionFocused("Filter active-file symbols"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(listOf("ServiceHandler"), selectedSymbols)
          fixture.setFocusedText("missing")
          fixture.render("palette-empty-symbols-800-1.5")
          assertTrue(fixture.hasText("No active-file symbol matches"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(listOf("ServiceHandler"), selectedSymbols)
          fixture.clickText("Close")
          assertTrue(dismissed)
        }
  }

  @Test
  fun paletteLongResultsKeepTheLastKeyboardSelectionVisibleInAShortWindow() {
    val files =
        (1..30).map {
          IndexedFile("internal/service/very-long-file-name-$it.go", "$it", "Go", false)
        }
    val displayedFiles =
        commandSearchResults(
            PaletteMode.Files, "service", files, emptyList(), hasActiveFile = false)
    val selectedFiles = mutableListOf<String>()
    var dismissed = false
    ComposeVisualFixture(1280, 600, 1.5f) {
          CommandPaletteDialog(
              mode = PaletteMode.Files,
              query = "service",
              onQuery = {},
              onMode = {},
              files = files,
              symbols = emptyList(),
              hasActiveFile = false,
              onSelectFile = { selectedFiles += it },
              onSelectSymbol = {},
              onSelectAction = {},
              onDismiss = { dismissed = true })
        }
        .use { fixture ->
          fixture.render("palette-long-files-1280-1.5")
          repeat(displayedFiles.lastIndex) { assertTrue(fixture.pressKey(Key.DirectionDown)) }
          fixture.awaitVisibleDescription("File ${displayedFiles.last().path}, selected")
          fixture.render("palette-long-files-1280-1.5-last-selection")
          fixture.clickVisibleDescription("File ${displayedFiles.last().path}, selected")
          assertEquals(listOf(displayedFiles.last().path), selectedFiles)
          fixture.clickText("Close")
          assertTrue(dismissed)
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
              ReviewToolWindowState(
                  project,
                  file,
                  symbol,
                  null,
                  editableDraft(draft),
                  draft,
                  null,
                  null,
                  null,
                  null,
                  false),
              { selectedSurfaces += it },
              onCreateDeclaration = {},
              canvas = {
                SourceEditorPane(project, file, listOf(symbol), symbol, 3, emptyList(), {})
              })
        }
        .use { fixture ->
          fixture.render("editor-candidate-800-1.3")
          assertTrue(fixture.hasText("Candidate diff"))
          assertTrue(fixture.hasText("Validate"))
          assertTrue(fixture.hasText("Checks"))
          assertFalse(fixture.hasText("focused checks pending"))
          fixture.clickText("Candidate diff")
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
          assertTrue(fixture.hasText("Validation failed"))
          assertTrue(fixture.hasText("Edit draft"))
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
            assertTrue(fixture.hasText("Source unchanged"))
            assertTrue(fixture.hasText("Ready to apply"))
            assertTrue(fixture.hasDescription("Apply Serve to internal/api/server.go"))
            fixture.clickText("Check details")
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
    ComposeVisualFixture(300, 900, 1.3f) {
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
          fixture.scrollBy(2000f)
          fixture.render()
          fixture.assertTextAboveDescription("Candidate for review", "Invalid")
          fixture.clickText("Show full response")
          fixture.render("assistant-response-expanded-800-1.3")
          assertEquals("Expanded", fixture.stateDescription("Show less"))
          assertTrue(fixture.hasText("Fix validation diagnostics before continuing."))
          assertTrue(fixture.hasText("Confirm remote destination"))
          kotlin.test.assertEquals(0, assistantActions)
        }

    var remoteConfirmation by mutableStateOf(false)
    val remoteChanges = mutableListOf<Boolean>()
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
                onConfirmed = {
                  remoteChanges += it
                  remoteConfirmation = it
                },
            )
          }
        }
        .use { fixture ->
          fixture.render("remote-consent-unconfirmed-480-1.3")
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertEquals(1, fixture.clickableDescriptionCount("Confirm remote destination"))
          assertEquals(
              ToggleableState.Off, fixture.descriptionToggleableState("Confirm remote destination"))
          assertEquals(
              "Not confirmed", fixture.descriptionStateDescription("Confirm remote destination"))
          fixture.clickVisibleDescription("Confirm remote destination")
          fixture.render("remote-consent-confirmed-480-1.3")
          assertEquals(listOf(true), remoteChanges)
          assertEquals(
              ToggleableState.On, fixture.descriptionToggleableState("Confirm remote destination"))
          assertEquals(
              "Confirmed", fixture.descriptionStateDescription("Confirm remote destination"))
          assertTrue(fixture.requestDescriptionFocus("Confirm remote destination"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(listOf(true, false), remoteChanges)
          assertEquals(
              ToggleableState.Off, fixture.descriptionToggleableState("Confirm remote destination"))
        }
  }

  @Test
  fun disabledAndLocalDestinationsDoNotGrantRemoteConfirmation() {
    val remote =
        ScopedModel(
            scope = ModelScope.Analyze.wireValue,
            profile = "review-profile",
            model = "provider/reviewer",
            remoteProvider = true)
    var changes = 0
    ComposeVisualFixture(480, 220, 1.5f) {
          Column(Modifier.fillMaxSize().background(AppBackground).padding(8.dp)) {
            RemoteProviderConfirmation(
                ModelScope.Analyze, remote, true, { changes++ }, enabled = false)
            RemoteProviderConfirmation(
                ModelScope.Analyze, remote.copy(remoteProvider = false), false, { changes++ })
          }
        }
        .use { fixture ->
          fixture.render("remote-consent-disabled-and-local-480-150")
          assertEquals(1, fixture.textCount("Confirm remote destination"))
          assertTrue(fixture.isDescriptionDisabled("Confirm remote destination"))
          assertEquals(
              ToggleableState.On, fixture.descriptionToggleableState("Confirm remote destination"))
          assertEquals(
              "Confirmed", fixture.descriptionStateDescription("Confirm remote destination"))
          assertFalse(fixture.tryClick("Confirm remote destination"))
          assertFalse(fixture.requestDescriptionFocus("Confirm remote destination"))
          fixture.pressKey(Key.Enter)
          assertEquals(0, changes)
        }
  }

  @Test
  fun flatSharedSurfacesKeepBordersAndGrowAroundLargerText() {
    val heights = mutableListOf<Pair<Float, Float>>()
    listOf(1f, 1.5f).forEach { scale ->
      ComposeVisualFixture(520, 420, scale) {
            Column(
                Modifier.fillMaxSize().background(ActivityRail).padding(MiniOrcaSpacing.section),
                verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.standard)) {
                  MiniOrcaPanel(Modifier.fillMaxWidth().testTag("flat-panel")) {
                    Text("Panel content")
                  }
                  MiniOrcaPanel(Modifier.fillMaxWidth().testTag("flat-raised"), raised = true) {
                    Text("Raised content")
                  }
                  WorkspaceSection("Workspace", Modifier.testTag("flat-workspace")) {
                    Text("Workspace content")
                  }
                  CompactSingleLineField(
                      value = "Search files",
                      onValueChange = {},
                      label = "Search",
                      showLabel = false,
                      modifier = Modifier.testTag("flat-field"))
                  Row(horizontalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.standard)) {
                    MiniOrcaButton(onClick = {}, modifier = Modifier.testTag("flat-button")) {
                      Text("Ready")
                    }
                    MiniOrcaButton(
                        onClick = {},
                        focusHighlight = true,
                        modifier = Modifier.testTag("growing-button")) {
                          Text("Apply\nNow")
                        }
                  }
                  IdeProgressBar(
                      0.5f, Modifier.width(120.dp).height(12.dp).testTag("round-progress"))
                }
          }
          .use { fixture ->
            fixture.render("flat-shared-geometry-${(scale * 100).toInt()}")
            listOf("Panel content", "Raised content", "Workspace", "Workspace content", "Ready")
                .forEach(fixture::assertTextFits)
            fixture.assertTextLineCount("Apply\nNow", 2)
            listOf(
                    "flat-panel" to Panel,
                    "flat-raised" to Card,
                    "flat-workspace" to Panel,
                    "flat-field" to EditorCanvas,
                    "flat-button" to StrongSurface)
                .forEach { (tag, fill) -> fixture.assertFlatCorner(tag, fill) }
            fixture.assertColorVisible(FocusAccent)
            val field = fixture.taggedBounds("flat-field")
            val button = fixture.taggedBounds("flat-button")
            val growing = fixture.taggedBounds("growing-button")
            val progress = fixture.taggedBounds("round-progress")
            assertTrue(field.height >= 34f, "Field minimum height at $scale: $field")
            assertTrue(button.height >= 32f, "Button minimum height at $scale: $button")
            assertTrue(growing.height > button.height, "Multiline action must grow at $scale")
            assertTrue(progress.height == 12f, "Progress capsule retains its track height")
            assertTrue(fixture.firstVisibleTextBounds("Ready").bottom <= button.bottom)
            assertTrue(fixture.firstVisibleTextBounds("Apply\nNow").bottom <= growing.bottom)
            heights += field.height to growing.height
          }
    }
    assertTrue(heights[1].first > heights[0].first, "Field must grow with font scale")
    assertTrue(heights[1].second > heights[0].second, "Button must grow with font scale")
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
  fun summaryIdentityKeepsTypeBesideNameAndFactsWithoutRepeatingType() {
    val project =
        visualFixtureProject.copy(
            name = "Sample workspace",
            type = "Java",
            buildFile = "pom.xml",
            languages = mapOf("Java" to 4, "Kotlin" to 2),
            aiStatus = "missing")
    ComposeVisualFixture(1440, 900) { ProjectSummaryPane(null, project, {}) }
        .use { fixture ->
          fixture.render("summary-identity-java-wide")
          fixture.assertTextSharesRowBefore("Sample workspace", "Java project")
          fixture.assertTextFits("Java project")
          assertEquals(1, fixture.taggedTextCount("summary-project-type", "Java project"))
          assertTrue(fixture.hasText("pom.xml · 23 indexed files · 1,800 lines · Java · Kotlin"))
          assertEquals(1, fixture.textCount("Java project"))
          assertTrue(fixture.hasText("Project description: unavailable"))
        }
  }

  @Test
  fun summaryIdentityWrapsLongNameAndRealTypeAtCompactLargeText() {
    val name =
        "Project migration workspace with multiple integrations and a lengthy identity "
            .repeat(3)
            .trim()
    val customType =
        "Legacy mixed-language application with custom build metadata ".repeat(3).trim()
    val project =
        visualFixtureProject.copy(
            name = name, type = customType, buildFile = "build.gradle.kts", aiStatus = "missing")
    ComposeVisualFixture(800, 650, 1.5f) { ProjectSummaryPane(null, project, {}) }
        .use { fixture ->
          fixture.render("summary-identity-long-800-150")
          val label = "Project type: $customType"
          fixture.assertTextWrapsWithoutClipping(name)
          fixture.assertTextWrapsWithoutClipping(label)
          val identity = fixture.taggedBounds("summary-project-identity")
          val nameBounds = fixture.firstVisibleTextBounds(name)
          val typeBounds = fixture.taggedBounds("summary-project-type")
          assertTrue(nameBounds.bottom < typeBounds.top, "Long type must flow below long name")
          assertTrue(nameBounds.right <= identity.right && typeBounds.right <= identity.right)
          assertTrue(typeBounds.bottom <= identity.bottom)
          assertTrue(fixture.hasText("Project description: unavailable"))
          assertTrue(fixture.hasText("build.gradle.kts · 23 indexed files · 1,800 lines"))
        }
    ComposeVisualFixture(800, 650, 1.5f) { ProjectSummaryPane(null, project.copy(type = " "), {}) }
        .use { fixture ->
          fixture.render("summary-identity-blank-type-800-150")
          fixture.assertTextFits("Project")
          assertEquals(1, fixture.taggedTextCount("summary-project-type", "Project"))
          assertTrue(fixture.hasText("build.gradle.kts · 23 indexed files · 1,800 lines"))
        }
  }

  @Test
  fun baselineCapturesSummaryAndExercisesOnlyTheLiveProjectMenu() {
    ComposeVisualFixture(1440, 900) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render("summary-1440")
          assertEquals(1, fixture.textCount("Summary"))
          fixture.assertTextSharesRowBefore(visualFixtureProject.name, "Go project")
          assertTrue(fixture.hasText("go.mod · 23 indexed files · 1,800 lines · Go · Markdown"))
          assertTrue(fixture.hasText("Analysis coverage"))
          fixture.assertTextAbove("Summary", visualFixtureProject.name)
          assertFalse(fixture.hasText("Project understanding"))
        }

    ComposeVisualFixture(1440, 900) { ToolbarVisualFixture(1440f) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("go-shop · fixture")
          fixture.render("project-menu-open")
          assertTrue(fixture.hasText("Switch project…"))
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
            assertTrue(fixture.hasText("Switch project…"))
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
            onReindex = { reindexes++ },
            onReconnect = { reconnects++ },
            onPalette = {},
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
          assertTrue(fixture.hasText("Open project…"))
          assertTrue(fixture.isDisabled("Re-index project"))
          assertTrue(fixture.hasText("Reconnect"))
          fixture.clickText("Open project…")
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
  fun f16HeaderAndSummaryUseTheAnalysisLifecycleWithoutConfusingDaemonConnectivity() {
    val project = resultProjectFixture()
    val run = analysisRunFixture()
    val states =
        listOf(
            "running" to ProjectAnalysisRunState(run = run.copy(status = "running")),
            "requesting-pause" to
                ProjectAnalysisRunState(
                    run = run.copy(status = "running"),
                    action = "pause",
                    controlRequest =
                        AnalysisControlRequest("pause", AnalysisControlOutcome.Requesting)),
            "pausing" to ProjectAnalysisRunState(run = run.copy(status = "pausing")),
            "requesting-cancel" to
                ProjectAnalysisRunState(
                    run = run.copy(status = "pausing"),
                    action = "cancel",
                    controlRequest =
                        AnalysisControlRequest("cancel", AnalysisControlOutcome.Requesting)),
            "paused" to ProjectAnalysisRunState(run = run.copy(status = "paused")),
            "canceling" to ProjectAnalysisRunState(run = run.copy(status = "canceling")),
            "canceled" to ProjectAnalysisRunState(run = run.copy(status = "canceled")),
            "unconfirmed" to
                ProjectAnalysisRunState(
                    run = run.copy(status = "paused"),
                    controlRequest =
                        AnalysisControlRequest("cancel", AnalysisControlOutcome.Unconfirmed)),
            "failed-refresh" to
                ProjectAnalysisRunState(
                    run = run.copy(status = "paused"),
                    statusUnavailable = true,
                    error = "Read failed",
                    errorKind = AnalysisRunErrorKind.StatusRead))
    for ((name, analysis) in states) {
      val presentation = projectRunPresentation(project, analysis)
      val label = analysisLifecycleStatusLabel(presentation, analysis)
      val state =
          DesktopState(projectState = ProjectWorkspaceState(project), analysisRun = analysis)
      val header = requireNotNull(toolbarAnalysisStatus(state))
      assertTrue(header.detail.contains(presentation.lifecycleExplanation.orEmpty()), name)
      assertEquals("Analysis · $label", header.label, name)
      if (name == "failed-refresh") {
        assertTrue(header.detail.contains("Status read failed; last accepted run retained"))
        assertTrue(header.detail.contains("Last accepted run · Paused"))
      }
      ComposeVisualFixture(800, 650) {
            AnalysisRunStrip(
                AnalysisWorkspacePaneState(project, analysis), null, AnalysisRunStripScope.Analysis)
          }
          .use { fixture ->
            fixture.render("f16-analysis-lifecycle-$name")
            fixture.assertTextFits(analysisRunTitle(analysis.run, presentation))
            presentation.lifecycleExplanation?.let { assertTrue(fixture.hasText(it), name) }
          }
      ComposeVisualFixture(1440, 900) {
            ToolbarVisualFixture(
                1440f,
                connection = ConnectionState(label = "Disconnected"),
                analysisStatus = header)
          }
          .use { fixture ->
            fixture.render("f16-header-$name")
            fixture.assertTextFits(header.label)
            fixture.assertTextBefore(header.label, "Daemon disconnected")
          }
      ComposeVisualFixture(800, 650) {
            ProjectSummaryPane(null, project, {}, run = analysis.run, analysisState = analysis)
          }
          .use { fixture ->
            fixture.render("f16-summary-$name")
            fixture.assertTextFits(label, maxLines = 3)
            fixture.assertSummaryStatusPlacement(label)
            presentation.lifecycleExplanation?.let { assertTrue(fixture.hasText(it), name) }
          }
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
                ToolbarVisualFixture(width.toFloat(), analysisStatus = toolbarAnalysisStatus(state))
              }
              .use { fixture ->
                val daemon = "Daemon connected"
                fixture.render("toolbar-analysis-$width-$scale")
                fixture.assertTextFits("Analysis · Running")
                fixture.assertTextFits(daemon)
                fixture.assertTextBefore("Analysis · Running", daemon)
                fixture.assertTextFits("Search files, symbols, commands")
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
                state =
                    state.copy(
                        analysisRun =
                            state.analysisRun.copy(
                                error = "unavailable", errorKind = AnalysisRunErrorKind.StatusRead))
                fixture.render("toolbar-analysis-read-failure-$width-$scale")
                fixture.assertTextFits("Analysis · Status unavailable")
                fixture.assertTextBefore("Analysis · Status unavailable", daemon)
                assertTrue(
                    fixture.hasDescription(
                        "Whole-project analysis · Status unavailable · Last accepted run · Completed · Status read failed; last accepted run retained · unavailable"))
                state = state.copy(analysisRun = ProjectAnalysisRunState())
                fixture.render()
                assertFalse(fixture.hasText("Analysis · Completed"))
                fixture.assertTextFits("Analysis · Unavailable")
                fixture.assertTextBefore("Analysis · Unavailable", daemon)
                fixture.assertTextFits(daemon)
              }
        }
  }

  @Test
  fun toolbarWrapsLongIdentityWithoutHidingSearchOrIndependentStatuses() {
    val name = "A very long project name with 日本語 and a full identity beyond the header"
    val branch = "feature/very-long-branch-with-the-real-revision-name"
    for ((width, height) in
        listOf(800 to 650, 999 to 650, 1000 to 650, 1280 to 600, 1399 to 650, 1400 to 650)) {
      for (scale in if (width >= 1399) listOf(1f, 1.25f, 1.5f) else listOf(1.25f, 1.5f)) {
        var git by mutableStateOf<GitStatus?>(GitStatus(available = true, branch = branch))
        var connection by mutableStateOf(ConnectionState(label = "Disconnected"))
        var analysis by mutableStateOf<ToolbarAnalysisStatus?>(null)
        ComposeVisualFixture(width, height, scale) {
              ToolbarVisualFixture(
                  width.toFloat(),
                  project = visualFixtureProject.copy(name = name),
                  gitStatus = git,
                  connection = connection,
                  analysisStatus = analysis)
            }
            .use { fixture ->
              fun verify(status: String, daemon: String) {
                fixture.render("header-wrap-$width-$height-$scale-$status")
                assertTrue(fixture.hasDescription(name))
                fixture.awaitVisibleDescription(name)
                fixture.awaitVisibleDescription("Search files, symbols, commands")
                fixture.assertTextFits("Search files, symbols, commands")
                fixture.assertTextFits(status)
                fixture.assertTextFits(daemon)
                fixture.assertTextBefore(status, daemon)
                val search = fixture.firstVisibleTextBounds("Search files, symbols, commands")
                val branchBounds = fixture.firstVisibleTextBounds(branchPresentation(git).label)
                val analysisBounds = fixture.firstVisibleTextBounds(status)
                val daemonBounds = fixture.firstVisibleTextBounds(daemon)
                assertTrue(branchBounds.bottom <= search.top || branchBounds.right <= search.left)
                assertTrue(
                    search.bottom <= analysisBounds.top || search.right <= analysisBounds.left)
                assertTrue(daemonBounds.right <= width && daemonBounds.bottom <= height)
              }
              verify("Analysis · Unavailable", "Daemon disconnected")
              assertTrue(fixture.hasDescription("Current Git branch: $branch"))
              analysis =
                  ToolbarAnalysisStatus(
                      "Analysis · Failed", "Whole-project analysis · Failed", false, true)
              verify("Analysis · Failed", "Daemon disconnected")
              git = GitStatus(available = true, branch = " \t ")
              connection = ConnectionState(connected = true)
              verify("Analysis · Failed", "Daemon connected")
              assertTrue(fixture.hasDescription("Git branch is unavailable for the selected file."))
              fixture.assertTextFits("Unavailable")
            }
      }
    }
  }

  @Test
  fun toolbarKeepsIdentityAndIndependentStatusesWhenEvidenceChanges() {
    val longName = "A project name that is much longer than the header control"
    val longBranch = "feature/a-branch-name-that-exceeds-the-visible-header-space"
    var project by mutableStateOf(visualFixtureProject.copy(name = longName))
    var gitStatus by mutableStateOf<GitStatus?>(GitStatus(available = true, branch = longBranch))
    var connection by mutableStateOf(ConnectionState(label = "Disconnected"))
    var analysisStatus by mutableStateOf<ToolbarAnalysisStatus?>(null)
    var searches = 0
    ComposeVisualFixture(1440, 220) {
          ToolbarVisualFixture(
              width = 1440f,
              project = project,
              gitStatus = gitStatus,
              connection = connection,
              analysisStatus = analysisStatus,
              actions = ToolbarActions({}, {}, {}, { searches++ }))
        }
        .use { fixture ->
          fixture.render("toolbar-unknown-disconnected")
          assertTrue(fixture.hasDescription(longName))
          assertTrue(fixture.hasDescription("Current Git branch: $longBranch"))
          assertTrue(
              fixture.hasDescription(
                  "Daemon disconnected; this is daemon status, not model connectivity"))
          fixture.assertTextBefore("Analysis · Unavailable", "Daemon disconnected")
          assertFalse(fixture.hasText("Analysis · Completed"))
          assertEquals(0, searches)
          fixture.clickText("Search files, symbols, commands")
          assertEquals(1, searches)

          gitStatus = GitStatus(available = true, branch = "  \t ")
          fixture.render()
          fixture.assertTextFits("Unavailable")
          assertTrue(fixture.hasDescription("Git branch is unavailable for the selected file."))
          assertFalse(fixture.hasText("main"))
          gitStatus = null
          fixture.render()
          assertTrue(fixture.hasDescription("Git branch is unavailable for the selected file."))

          val run = analysisRunFixture().copy(status = "failed")
          val state =
              DesktopState(
                  projectState = ProjectWorkspaceState(resultProjectFixture()),
                  analysisRun = ProjectAnalysisRunState(run = run))
          analysisStatus = toolbarAnalysisStatus(state)
          fixture.render("toolbar-failed-disconnected")
          fixture.assertTextBefore("Analysis · Failed", "Daemon disconnected")
          assertTrue(
              fixture.hasDescription(
                  "Whole-project analysis · Failed · Analysis failed; inspect the retained evidence before starting another run."))
          assertFalse(fixture.hasText("Daemon connected"))
          analysisStatus =
              toolbarAnalysisStatus(
                  state.copy(
                      analysisRun =
                          ProjectAnalysisRunState(
                              run = run.copy(identity = run.identity.copy(projectId = "other")))))
          fixture.render("toolbar-foreign-run")
          fixture.assertTextBefore("Analysis · Unavailable", "Daemon disconnected")
          assertFalse(fixture.hasText("Analysis · Failed"))
          analysisStatus =
              toolbarAnalysisStatus(
                  state.copy(
                      projectState =
                          state.projectState.copy(
                              project = state.project!!.copy(projectRevision = "next"))))
          fixture.render("toolbar-stale-run")
          fixture.assertTextBefore("Analysis · Stale", "Daemon disconnected")
          assertFalse(fixture.hasText("Analysis · Failed"))
          analysisStatus = null
          project = visualFixtureProject
          connection = ConnectionState(connected = true)
          fixture.render()
          fixture.assertTextBefore("Analysis · Unavailable", "Daemon connected")
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
          assertFalse(fixture.hasText("Mini-Orca"))
          fixture.assertTextBefore("go-shop · fixture", "main")
          fixture.assertTextBefore("main", "Search files, symbols, commands")
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
          assertTrue(fixture.isFocused("Search files, symbols, commands"))
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
  fun collapsedFilesKeepLockErrorAndKeyboardRecoveryReachableAtReducedHeight() {
    for ((width, height) in listOf(800 to 440, 1280 to 480)) {
      val view = AnalysisFilesViewState().apply { expanded = false }
      var reads = 0
      var starts = 0
      var saves = 0
      ComposeVisualFixture(width, height, 1.5f) {
            AnalysisWorkspacePane(
                AnalysisWorkspacePaneState(
                    resultProjectFixture(),
                    ProjectAnalysisRunState(
                        run = analysisRunFixture().copy(status = "running"),
                        fileSelection =
                            AnalysisSelectionState(
                                selectionFixture(),
                                error = "Selection read failed: try refresh",
                                failure = AnalysisSelectionFailure.Read))),
                AnalysisWorkspaceActions(
                    { _, _ -> starts++ },
                    {},
                    {},
                    {},
                    {},
                    { reads++ },
                    { saves++ },
                    refreshStatus = { error("Unexpected status refresh") }),
                view)
          }
          .use { fixture ->
            fixture.render()
            for (label in
                listOf(
                    "Selection locked. Finish or cancel the current run to change files.",
                    "Selection read failed: try refresh",
                    "Refresh files")) {
              fixture.revealText(label, "analysis-page")
              if (label == "Refresh files") fixture.assertTextFits(label)
              else assertTrue(fixture.hasText(label))
            }
            assertTrue(fixture.requestFocus("Refresh files"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            assertEquals(1, reads)
            assertEquals(0, starts + saves)
            assertFalse(fixture.hasDescription("Analyze main.go"))
          }
    }
  }

  @Test
  fun roundedAnalysisGroupsRunControlsAndShowsItsFileTableAtSupportedSizes() {
    listOf(1600 to 1000, 1440 to 900).forEach { (width, height) ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        ComposeVisualFixture(width, height, scale) { RoundedAnalysisVisualFixture(width.toFloat()) }
            .use { fixture ->
              fixture.render("analysis-frame-$width-$height-$scale")
              assertTrue(fixture.hasDescription("Analysis tool window, selected"))
              fixture.assertTextFits("Analyzing selected files")
              assertEquals(
                  1, fixture.taggedTextCount("analysis-run-content", "Analyzing selected files"))
              assertEquals(0, fixture.taggedTextCount("analysis-run-content", "Running"))
              fixture.assertTextFits("8 of 12 files finished")
              fixture.assertTextFits("Current: internal/api/user.go")
              fixture.assertTextFits("Pause")
              fixture.assertTextFits("Cancel")
              fixture.assertAnalysisRunGeometry()
              if (width == 1600 && scale == 1f) {
                assertTrue(fixture.hasText("67%"))
                fixture.assertTextAbove("8 of 12 files finished", "Current: internal/api/user.go")
              }
              if (width >= 1440 && scale == 1f) {
                fixture.revealText("Refresh files", "analysis-page")
                fixture.scrollBy(180f, "analysis-page")
                fixture.render()
                fixture.revealText("File", "analysis-page")
                fixture.assertAnalysisTableColumns()
              }
              if (width == 1600 && scale == 1.5f) {
                fixture.revealText("Files", "analysis-page")
                listOf("Files", "All", "Needs attention", "Up to date", "Excluded").forEach { label
                  ->
                  fixture.revealText(label, "analysis-page")
                  assertTrue(
                      fixture.firstVisibleTextBounds(label).right <=
                          fixture.taggedBounds("analysis-file-panel").right,
                      "$label must fit inside the workspace panel")
                }
              }
              if (width == 1600 && scale == 1f) {
                fixture.revealText("Saved results · Bugs / Performance / Security", "analysis-page")
                fixture.assertAnalysisCategoryGeometry()
                fixture.revealText("Refresh files", "analysis-page")
                fixture.scrollBy(180f, "analysis-page")
                fixture.render()
                val table = fixture.taggedBounds("analysis-file-table")
                val analysisPage = fixture.taggedBounds("analysis-page")
                listOf("File", "Analysis state", "Details").forEach { header ->
                  val bounds = fixture.firstVisibleTextBounds(header)
                  assertTrue(
                      bounds.top >= analysisPage.top && bounds.bottom <= analysisPage.bottom,
                      "$header must remain reachable in the Analysis viewport")
                  assertTrue(
                      bounds.bottom <= table.top,
                      "$header must remain immediately above the bounded table")
                }
                val firstRow = fixture.taggedBounds("analysis-file-row-cmd/server/main.go")
                assertTrue(firstRow.top >= table.top && firstRow.bottom <= table.bottom)
                assertTrue(
                    firstRow.top >= analysisPage.top && firstRow.bottom <= analysisPage.bottom)
              }
              fixture.revealText("Files", "analysis-page")
              assertTrue(fixture.hasDescription("Collapse Files"))
              fixture.revealText(
                  "Selection locked. Finish or cancel the current run to change files.",
                  "analysis-page")
              fixture.assertTextFits(
                  "Selection locked. Finish or cancel the current run to change files.")
              fixture.render("analysis-files-frame-$width-$height-$scale")
              fixture.revealText("Refresh files", "analysis-page")
              fixture.assertTextFits("Refresh files")
            }
      }
    }
  }

  @Test
  fun f15CompactSummaryAndHeaderMatchTheAnalysisViewportAndDensityMatrix() {
    val project = resultProjectFixture()
    val analysis = roundedAnalysisStateFixture()
    val run = requireNotNull(analysis.run)
    val overview =
        visualFixtureOverview.copy(
            projectId = project.projectId, projectRevision = project.projectRevision)
    val status =
        requireNotNull(
            toolbarAnalysisStatus(
                DesktopState(
                    projectState = ProjectWorkspaceState(project), analysisRun = analysis)))
    assertEquals("Analysis · Running", status.label)
    var dispatches = 0
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> dispatches++ },
            { dispatches++ },
            { dispatches++ },
            { dispatches++ },
            { dispatches++ },
            refreshStatus = { error("Unexpected status refresh") })
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
          val pixelWidth = (width * density).toInt()
          val pixelHeight = (height * density).toInt()
          val label = "f15-matrix-$width-$height-$scale-${density}x"
          ComposeVisualFixture(pixelWidth, pixelHeight, scale, density) {
                ProjectSummaryPane(
                    overview,
                    project,
                    {},
                    run = run,
                    analysisState = analysis,
                    analysisActions = actions)
              }
              .use { fixture ->
                fixture.render("$label-summary")
                assertEquals(1, fixture.tagCount("summary-analysis-run-strip"))
                fixture.assertTextFits("Pause")
                fixture.assertTextFits("Cancel")
                assertTrue(fixture.hasText("Running"))
                assertTrue(fixture.hasDescription("Show active files"))
              }
          ComposeVisualFixture(pixelWidth, pixelHeight, scale, density) {
                MainToolbar(
                    ToolbarState(
                        project,
                        false,
                        "",
                        ConnectionState(connected = true),
                        GitStatus(available = true, branch = "main"),
                        status),
                    ToolbarActions({}, {}, {}, {}))
              }
              .use { fixture ->
                fixture.render("$label-header")
                fixture.assertTextFits("Analysis · Running")
                assertFalse(fixture.hasText("Analysis · Stale"))
              }
        }
      }
    }
    assertEquals(0, dispatches)
  }

  @Test
  fun analysisOverviewStacksControlsAndKeepsFullPathsLocalAcrossPollsAndResizes() {
    val path = "internal/" + "日本語-very-long-directory/".repeat(10) + "worker.go"
    val base = requireNotNull(roundedAnalysisStateFixture().run)
    val run =
        base.copy(
            plan =
                base.plan.copy(
                    files =
                        base.plan.files.mapIndexed { index, file ->
                          if (index == 2) file.copy(path = path) else file
                        }),
            files =
                base.files.mapIndexed { index, file ->
                  if (index == 2) file.copy(path = path) else file
                })
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
          var state by
              mutableStateOf(
                  AnalysisWorkspacePaneState(
                      resultProjectFixture(), roundedAnalysisStateFixture().copy(run = run)))
          var dispatches = 0
          val actions =
              AnalysisWorkspaceActions(
                  { _, _ -> dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  { dispatches++ },
                  refreshStatus = { error("Unexpected status refresh") })
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    AnalysisWorkspacePane(state, actions)
                  }
              .use { fixture ->
                val label = "f15-overview-$width-$height-$scale-${density}x"
                fixture.render("$label-collapsed")
                fixture.assertTextFits("Pause")
                fixture.assertTextFits("Cancel")
                fixture.revealText(
                    "Finished includes partial and failed outcomes; it does not mean successful.",
                    "analysis-page")
                fixture.revealText("Show full path", "analysis-page")
                fixture.clickDescription("Show active files")
                fixture.render("$label-expanded")
                fixture.revealText("Current: $path", "analysis-page")
                assertTrue(fixture.hasText("Current: $path"))
                val nextPath = "internal/db/store.go"
                val polled =
                    run.copy(
                        windowFilesCompleted = 2,
                        files =
                            run.files.map { file ->
                              when (file.path) {
                                path ->
                                    file.copy(
                                        stages = file.stages.map { it.copy(status = "completed") })
                                nextPath ->
                                    file.copy(
                                        stages = file.stages.map { it.copy(status = "running") })
                                else -> file
                              }
                            })
                state = state.copy(analysis = state.analysis.copy(run = polled))
                fixture.render("$label-polled")
                assertTrue(fixture.hasDescription("Hide active files"))
                fixture.revealText("Current: $nextPath", "analysis-page")
                fixture.resize(
                    ((width - 80).coerceAtLeast(720) * density).toInt(), (height * density).toInt())
                fixture.render("$label-resized")
                assertTrue(fixture.hasDescription("Hide active files"))
                fixture.revealText("Refresh files", "analysis-page")
                fixture.assertTextFits("Refresh files")
                assertEquals(0, dispatches)
                state =
                    state.copy(
                        analysis =
                            state.analysis.copy(
                                run =
                                    polled.copy(
                                        identity =
                                            run.identity.copy(generation = "new-generation"))))
                fixture.render("$label-replaced")
                fixture.revealText("Show full path", "analysis-page")
                assertTrue(fixture.hasDescription("Show active files"))
                assertEquals(0, dispatches)
              }
        }
      }
    }
  }

  @Test
  fun analysisResultsStayNavigableWithRetainedRowsAndReadErrorsDuringAndAfterRun() {
    val page = resultPageFixture("bugs")
    val run = page.run!!.copy(status = "running")
    val sections =
        AnalysisResultType.entries.associate { type ->
          AnalysisResultKey(type.category) to
              AnalysisSectionState(
                  results = analysisResultsFixture(run, type.category),
                  error = "Saved details read failed")
        }
    var status by mutableStateOf("running")
    val destinations = mutableListOf<Workspace>()
    var otherCalls = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  page.project,
                  ProjectAnalysisRunState(run = run.copy(status = status), sections = sections)),
              AnalysisWorkspaceActions(
                  { _, _ -> otherCalls++ },
                  { otherCalls++ },
                  { otherCalls++ },
                  { otherCalls++ },
                  destinations::add,
                  { otherCalls++ },
                  { otherCalls++ },
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          for (phase in listOf("running", "failed")) {
            status = phase
            fixture.render("f15-categories-$phase")
            fixture.revealText("Saved results · Bugs / Performance / Security", "analysis-page")
            fixture.assertTextFits("Saved results · Bugs / Performance / Security")
            assertTrue(fixture.hasText("Saved details unavailable · 1 reported"))
            for (type in AnalysisResultType.entries) {
              fixture.revealText(type.workspace.name, "analysis-page")
              assertTrue(
                  fixture.hasText(
                      "Loaded · ${if (type == AnalysisResultType.Bugs) 1 else 0} matching ${if (type == AnalysisResultType.Bugs) "finding" else "findings"}"))
              fixture.clickVisibleDescription("View ${type.workspace.name} results")
            }
          }
          assertEquals(
              List(2) { AnalysisResultType.entries.map { it.workspace } }.flatten(), destinations)
          assertEquals(0, otherCalls)
        }
  }

  @Test
  fun categoryCardsDoNotTurnMissingOrForeignDetailsIntoCompletedEmpty() {
    val original = resultPageFixture("bugs")
    val run =
        original.run!!.copy(
            status = "completed_empty",
            sections =
                original.run.sections.map {
                  if (it.category == "bugs") it.copy(status = "completed_empty", findingCount = 0)
                  else it.copy(findingCount = null)
                })
    val foreign =
        analysisResultsFixture(run, "bugs")
            .copy(identity = run.identity.copy(generation = "other"), semantic = emptyList())
    var sections by
        mutableStateOf(mapOf(AnalysisResultKey("bugs") to AnalysisSectionState(results = foreign)))
    var current by mutableStateOf(run)
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  original.project, ProjectAnalysisRunState(run = current, sections = sections)),
              AnalysisWorkspaceActions(
                  { _, _ -> },
                  {},
                  {},
                  {},
                  {},
                  refreshStatus = { error("Unexpected status refresh") }))
        }
        .use { fixture ->
          fixture.render("f15-categories-unconfirmed-zero")
          fixture.revealText("0 reported · details not confirmed", "analysis-page")
          assertTrue(fixture.hasText("Loaded details · unavailable"))
          assertTrue(fixture.hasText("Completed · details not confirmed"))
          current = run.copy(sections = run.sections.map { it.copy(findingCount = null) })
          sections = emptyMap()
          fixture.render("f15-categories-unknown-reported")
          fixture.revealText("Saved results · Bugs / Performance / Security", "analysis-page")
          assertTrue(fixture.hasText("Count unavailable"))
          assertEquals(3, fixture.textCount("—"))
          assertTrue(fixture.hasText("Loaded details · unavailable"))
        }
  }

  @Test
  fun analysisOverviewDistinguishesUnavailableProgressAndKeepsApplicableActions() {
    val base = requireNotNull(roundedAnalysisStateFixture().run)
    val mismatched =
        base.copy(plan = base.plan.copy(identity = base.plan.identity.copy(queueId = "other")))
    var state by
        mutableStateOf(
            AnalysisWorkspacePaneState(
                resultProjectFixture(), ProjectAnalysisRunState(run = mismatched)))
    var starts = 0
    var resumes = 0
    var otherActions = 0
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> starts++ },
            { otherActions++ },
            { resumes++ },
            { otherActions++ },
            { otherActions++ },
            refreshStatus = { error("Unexpected status refresh") })
    ComposeVisualFixture(800, 650, 1.5f) { AnalysisWorkspacePane(state, actions) }
        .use { fixture ->
          fixture.render("f15-unavailable-progress")
          fixture.assertTextFits("File progress unavailable")
          assertFalse(fixture.hasText("0 of 0 files finished"))
          assertFalse(fixture.hasText("Current: internal/api/user.go"))
          fixture.assertTextFits("Pause")
          fixture.assertTextFits("Cancel")
          assertEquals(0, starts + resumes + otherActions)
          state = state.copy(analysis = state.analysis.copy(run = base.copy(status = "paused")))
          fixture.render("f15-paused-progress")
          fixture.revealText("Resume → fresh preview", "analysis-page")
          fixture.assertTextFits("Resume → fresh preview")
          assertEquals(0, starts + resumes + otherActions)
          state = state.copy(analysis = state.analysis.copy(run = null))
          fixture.render("f15-not-started")
          fixture.revealText("Start analysis", "analysis-page")
          fixture.assertTextFits("Start analysis")
          assertFalse(fixture.hasText("0 of 0 files finished"))
          assertEquals(0, starts + resumes + otherActions)
          fixture.clickText("Start analysis")
          assertEquals(1, starts)
        }
  }

  @Test
  fun f12ProductionAnalysisKeepsFilesReachableAtCompactViewportsAndTextScales() {
    for ((width, height) in listOf(1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        ComposeVisualFixture(width, height, scale) { RoundedAnalysisVisualFixture(width.toFloat()) }
            .use { fixture ->
              val label = "f12-analysis-$width-$height-$scale"
              fixture.render("$label-top")
              fixture.revealText("Refresh files", "analysis-page")
              fixture.assertTextFits("Refresh files")
              fixture.revealText("15 of 15 files match", "analysis-page")
              fixture.revealText("cmd/server/main.go", "analysis-page")
              assertTrue(fixture.hasDescription("Analyze cmd/server/main.go"), label)
              val table = fixture.taggedBounds("analysis-file-table")
              assertTrue(table.height > 0f, "$label: the file list must remain bounded")
              fixture.render("$label-files")
              fixture.scrollBy(100_000f, "analysis-file-table")
              fixture.render()
              assertTrue(fixture.hasText(".env"), "$label: the final file must be reachable")
            }
      }
    }
  }

  @Test
  fun analysisFileStatusMarkersPrecedeLabelsInAlignedRows() {
    listOf(1_600 to 900).forEach { (width, height) ->
      ComposeVisualFixture(width, height, 1f) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(
                    fileSelection =
                        AnalysisSelectionState(
                            selectionFixture()
                                .copy(
                                    files =
                                        selectionFixture().files.map { file ->
                                          if (file.path == "main.go")
                                              file.copy(
                                                  stages =
                                                      listOf(
                                                          AnalysisFileStageStatus(
                                                              "semantic", "pending", "Queued")))
                                          else file
                                        }))),
                AnalysisWorkspaceActions(
                    { _, _ -> },
                    {},
                    {},
                    {},
                    {},
                    refreshStatus = { error("Unexpected status refresh") }))
          }
          .use { fixture ->
            fixture.render("analysis-status-markers-$width")
            listOf(".env" to "Excluded", "helper.go" to "Up to date", "main.go" to "Pending")
                .forEach { (path, label) ->
                  assertTrue(fixture.hasDescription("Analyze $path"))
                  assertEquals(
                      if (path == ".env") 0 else 1,
                      fixture.clickableDescriptionCount("Analyze $path"))
                  assertEquals(
                      if (path == ".env") ToggleableState.Off else ToggleableState.On,
                      fixture.descriptionToggleableState("Analyze $path"))
                  assertEquals(
                      if (path == ".env") "Excluded from analysis" else "Selected for analysis",
                      fixture.descriptionStateDescription("Analyze $path"))
                  val marker = fixture.taggedBounds("analysis-file-status-marker-$path")
                  val status = fixture.taggedTextBounds("analysis-file-row-$path", label)
                  val row = fixture.taggedBounds("analysis-file-row-$path")
                  assertTrue(marker.right < status.left, "$path marker must precede its status")
                  assertTrue(
                      marker.top < status.bottom && marker.bottom > status.top,
                      "$path marker must align with its status")
                  assertTrue(
                      marker.left >= row.left && marker.right <= row.right,
                      "$path marker must remain inside its row")
                  if (path == "main.go") {
                    assertTrue(fixture.hasText(label), "Pending must remain an explicit label")
                    fixture.assertColorVisible(Warning)
                  }
                }
          }
    }
  }

  @Test
  fun fileRowsReflowWithNativeWidthAndTextScaleWithoutLosingEvidence() {
    val path =
        "internal/services/identity/日本語/" + "long-request-validation/".repeat(3) + "handler.go"
    val excludedPath = "vendor/generated/identity/configuration-with-long-name.go"
    val reason = "Policy excludes this file: " + "generated/identity/configuration/".repeat(4)
    val savedReason = "Saved evidence is outdated: " + "source/revision/".repeat(6)
    val runReason = "Analysis is scanning: " + "internal/services/identity/".repeat(5)
    val selection =
        selectionFixture()
            .copy(
                files =
                    listOf(
                        AnalysisSelectableFile(
                            path, "", selectionStageFixture("stale", savedReason)),
                        AnalysisSelectableFile(excludedPath, reason)))
    val run =
        analysisRunFixture().let { original ->
          original.copy(
              status = "running",
              plan =
                  original.plan.copy(
                      files = listOf(AnalysisPlannedFile(path, "base", "Go", 20, emptyList()))),
              files =
                  listOf(
                      AnalysisRunFile(
                          path,
                          "base",
                          "Go",
                          listOf(
                              AnalysisStageProgress(
                                  "semantic", "running", 1, false, reason = runReason)))))
        }
    for ((width, scale, density) in
        listOf(
            Triple(1131, 1f, 1f),
            Triple(1133, 1f, 1f),
            Triple(1600, 1f, 1f),
            Triple(1600, 1.25f, 1f),
            Triple(1600, 1.5f, 1f),
            Triple(800, 1f, 1f),
            Triple(800, 1.5f, 1f),
            Triple(800, 1.5f, 2f))) {
      var writes = 0
      var reads = 0
      var admissions = 0
      ComposeVisualFixture((width * density).toInt(), (900 * density).toInt(), scale, density) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(
                    run = run, fileSelection = AnalysisSelectionState(selection)),
                AnalysisWorkspaceActions(
                    { _, _ -> admissions++ },
                    { admissions++ },
                    { admissions++ },
                    { admissions++ },
                    { admissions++ },
                    { reads++ },
                    { writes++ },
                    refreshStatus = { error("Unexpected status refresh") }),
                400.dp)
          }
          .use { fixture ->
            val label = "f12-rows-$width-$scale-${density}x"
            fixture.render("$label-initial")
            val tag = "analysis-file-row-$path"
            val row = fixture.taggedBounds(tag)
            val identity = fixture.taggedTextBounds(tag, path)
            val state = fixture.taggedTextBounds(tag, "Running")
            val summary = fixture.taggedTextBounds(tag, "Code analysis")
            assertTrue(identity.right <= row.right && summary.right <= row.right, label)
            if ((width == 1600 && scale <= 1.25f) || (width == 1133 && scale == 1f)) {
              fixture.assertAnalysisTableColumnsForRow(tag, path, "Running", "Code analysis")
              assertTrue(identity.top < state.bottom && state.top < identity.bottom, label)
            } else {
              assertTrue(identity.bottom <= state.top && state.bottom <= summary.top, label)
              assertTrue(fixture.hasText("Analysis state"))
            }
            fixture.assertTextFontFamily(path, FontFamily.Monospace)
            if (width == 800) fixture.assertTextWrapsWithoutClipping(path)
            assertTrue(fixture.hasDescription("Analyze $path"))
            assertTrue(fixture.isDescriptionDisabled("Analyze $path"))
            assertTrue(fixture.hasText("Saved analysis: outdated"))
            fixture.clickDescription("Analysis details for $path")
            fixture.render("$label-expanded")
            val detail =
                "Code analysis · Running: $runReason\nSaved analysis: Outdated\n" +
                    listOf(
                            "Code analysis",
                            "Performance review",
                            "Security rules",
                            "AI Security review")
                        .joinToString("\n") { "$it: $savedReason" }
            assertTrue(fixture.hasText(detail), "$label: complete saved evidence must be available")
            fixture.scrollBy(100_000f, "analysis-file-table")
            fixture.render("$label-excluded")
            assertTrue(fixture.hasText(reason), "$label: policy reason must be visible")
            fixture.assertTextWrapsWithoutClipping(reason)
            assertTrue(fixture.isDescriptionDisabled("Analyze $excludedPath"))
            assertEquals(0, writes + reads + admissions)
          }
    }
  }

  @Test
  fun savedFreshnessAndAdmittedProgressRemainSeparateInProductionRows() {
    val stages = listOf("fresh", "stale", "failed", "partial", "unavailable", "future_status")
    val files =
        stages.map { stage ->
          AnalysisSelectableFile(
              "$stage.go",
              "",
              listOf(
                  AnalysisFileStageStatus("semantic", "fresh", "Current"),
                  AnalysisFileStageStatus("performance", stage, "Saved $stage"),
                  AnalysisFileStageStatus("security_rules", "skipped", "Not applicable")))
        } +
            listOf(
                AnalysisSelectableFile("policy.go", "Excluded by policy"),
                AnalysisSelectableFile("user.go", "", selectionStageFixture("stale", "Old source")))
    var selection by
        mutableStateOf(selectionFixture().copy(files = files, excludedPaths = listOf("user.go")))
    val original = analysisRunFixture()
    val planned =
        files.map {
          AnalysisPlannedFile(
              it.path,
              "base",
              "Go",
              20,
              listOf(AnalysisStagePlan("semantic", true, false, maxModelRequests = 0)))
        }
    var run by mutableStateOf<AnalysisRun?>(null)
    var reads = 0
    var writes = 0
    var starts = 0
    for ((width, scale) in listOf(1600 to 1f, 800 to 1.5f)) {
      selection = selection.copy(excludedPaths = listOf("user.go"))
      run = null
      ComposeVisualFixture(width, 900, scale) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(
                    run = run, fileSelection = AnalysisSelectionState(selection)),
                AnalysisWorkspaceActions(
                    { _, _ -> starts++ },
                    { starts++ },
                    { starts++ },
                    { starts++ },
                    { starts++ },
                    { reads++ },
                    { writes++ },
                    refreshStatus = { error("Unexpected status refresh") }),
                600.dp)
          }
          .use { fixture ->
            val label = "f12-freshness-$width-$scale"
            fixture.render("$label-saved")
            for ((stage, status) in
                stages.zip(
                    listOf(
                        "Up to date",
                        "Outdated",
                        "Failed",
                        "Incomplete",
                        "Unavailable",
                        "Status unavailable"))) {
              fixture.revealText("$stage.go", "analysis-file-table")
              assertEquals(1, fixture.taggedTextCount("analysis-file-row-$stage.go", status), label)
            }
            fixture.revealText("policy.go", "analysis-file-table")
            assertEquals(1, fixture.taggedTextCount("analysis-file-row-policy.go", "Excluded"))
            assertTrue(fixture.isDescriptionDisabled("Analyze policy.go"))
            fixture.revealText("user.go", "analysis-file-table")
            assertEquals(1, fixture.taggedTextCount("analysis-file-row-user.go", "Excluded"))
            run =
                original.copy(
                    status = "running",
                    plan = original.plan.copy(files = planned),
                    files =
                        files.map {
                          AnalysisRunFile(
                              it.path,
                              "base",
                              "Go",
                              listOf(AnalysisStageProgress("semantic", "pending", 0, false)))
                        })
            fixture.render("$label-pending")
            fixture.revealText("stale.go", "analysis-file-table")
            assertEquals(1, fixture.taggedTextCount("analysis-file-row-stale.go", "Pending"))
            assertEquals(
                1,
                fixture.taggedTextCount("analysis-file-row-stale.go", "Saved analysis: outdated"))
            fixture.clickDescription("Analysis details for stale.go")
            fixture.render("$label-pending-details")
            val detail =
                "Code analysis · Pending\nSaved analysis: Outdated\n" +
                    "Performance review: Saved stale"
            fixture.revealText(detail, "analysis-file-table")
            assertTrue(fixture.hasText(detail))
            run =
                run!!.copy(
                    files =
                        run!!.files.map { file ->
                          file.copy(stages = file.stages.map { it.copy(status = "completed") })
                        })
            fixture.render("$label-finished")
            fixture.revealText("stale.go", "analysis-file-table")
            assertEquals(1, fixture.taggedTextCount("analysis-file-row-stale.go", "Finished"))
            assertEquals(
                1,
                fixture.taggedTextCount("analysis-file-row-stale.go", "Saved analysis: outdated"))
            run = null
            selection = selection.copy(excludedPaths = emptyList())
            fixture.render("$label-reselected")
            fixture.revealText("user.go", "analysis-file-table")
            assertEquals(1, fixture.taggedTextCount("analysis-file-row-user.go", "Outdated"))
            assertEquals(0, reads + writes + starts)
          }
    }
  }

  @Test
  fun measuredAnalysisFileAllocationKeepsVariableContentAndBothScrollTargetsReachable() {
    val selectionError =
        "Selection refresh could not confirm the project inventory because the daemon returned a " +
            "temporary eligibility response. Keep the existing confirmed selection and try again."
    val stageFailure =
        "The semantic scanner could not read internal/services/worker6.go after the configured " +
            "retry budget. Restore the file, then retry analysis to collect complete evidence."
    val base = roundedAnalysisStateFixture()
    val extraPaths = (7..20).map { "internal/services/additional$it.go" }
    val run =
        requireNotNull(base.run)
            .copy(
                reason =
                    "The active run is using a compatibility plan while the project revision changes; " +
                        "the displayed files remain guarded by their planned content hashes.",
                files =
                    requireNotNull(base.run).files.mapIndexed { index, file ->
                      when (index) {
                        0,
                        1,
                        2 ->
                            file.copy(
                                stages =
                                    file.stages.map { stage -> stage.copy(status = "running") })
                        else -> file
                      }.let { updated ->
                        if (updated.path == "internal/services/worker6.go")
                            updated.copy(
                                stages =
                                    listOf(
                                        AnalysisStageProgress(
                                            "semantic", "failed", 2, false, reason = stageFailure)))
                        else updated
                      }
                    })
            .let {
              it.copy(
                  plan =
                      it.plan.copy(
                          files =
                              it.plan.files +
                                  extraPaths.map { path ->
                                    AnalysisPlannedFile(
                                        path,
                                        "base",
                                        "Go",
                                        20,
                                        listOf(
                                            AnalysisStagePlan(
                                                "semantic", true, false, maxModelRequests = 0)))
                                  }),
                  files =
                      it.files +
                          extraPaths.map { path ->
                            AnalysisRunFile(
                                path,
                                "base",
                                "Go",
                                listOf(AnalysisStageProgress("semantic", "pending", 1, false)))
                          })
            }
    val state =
        AnalysisWorkspacePaneState(
            resultProjectFixture(),
            base.copy(
                run = run,
                fileSelection =
                    base.fileSelection.copy(
                        selection =
                            requireNotNull(base.fileSelection.selection)
                                .copy(
                                    files =
                                        requireNotNull(base.fileSelection.selection).files +
                                            extraPaths.map { path ->
                                              AnalysisSelectableFile(path, "")
                                            }),
                        error = selectionError)))
    var actions = 0
    val callbacks =
        AnalysisWorkspaceActions(
            { _, _ -> actions++ },
            { actions++ },
            { actions++ },
            { actions++ },
            { actions++ },
            { actions++ },
            { actions++ },
            refreshStatus = { error("Unexpected status refresh") })

    ComposeVisualFixture(800, 650, 1.5f) { AnalysisWorkspacePane(state, callbacks) }
        .use { fixture ->
          fixture.render("analysis-measured-allocation-variable-content")
          fixture.clickDescription("Show active files")
          fixture.render("analysis-measured-allocation-active-paths")
          assertTrue(fixture.hasText("Current: internal/api/routes.go"))
          fixture.revealText("Filter files", "analysis-page")
          fixture.revealText(selectionError, "analysis-page")
          fixture.assertTextWrapsWithoutClipping(selectionError)
          fixture.revealText(
              "Selection locked. Finish or cancel the current run to change files.",
              "analysis-page")
          fixture.revealText("Refresh files", "analysis-page")
          fixture.revealText("cmd/server/main.go", "analysis-page")
          fixture.render()
          val measuredTableHeight = fixture.taggedBounds("analysis-file-table").height
          assertTrue(
              measuredTableHeight > 0f, "The nested file list must receive a measured height")
          assertTrue(
              fixture.scrollableContentCount() >= 2,
              "The outer page and bounded file list must remain independently scrollable")
          fixture.render()
          assertEquals(
              measuredTableHeight,
              fixture.taggedBounds("analysis-file-table").height,
              1f,
              "Measured content must converge without table-height instability")
          val stage = projectRunPresentation(state.project, state.analysis).stages.single()
          fixture.revealText("Code analysis · ${analysisStageBreakdown(stage)}", "analysis-page")
          fixture.clickText("Code analysis · ${analysisStageBreakdown(stage)}")
          fixture.render()
          fixture.revealText("Code analysis · ${analysisStageBreakdown(stage)}", "analysis-page")
          fixture.scrollBy(300f, "analysis-page")
          fixture.render()
          assertTrue(
              fixture.hasText(stageFailure), "Expanded stage retains its full failure detail")
          assertTrue(fixture.taggedBounds("analysis-stage-details-semantic").height > 0f)
          assertEquals(0, actions, "Measurement and local scrolling must not dispatch actions")
        }

    val scrollableAnalysis =
        roundedAnalysisStateFixture()
            .copy(
                fileSelection =
                    roundedAnalysisStateFixture().fileSelection.let {
                      it.copy(selection = requireNotNull(it.selection).copy(editable = true))
                    })
    ComposeVisualFixture(1_440, 900) { AnalysisFileSelector(scrollableAnalysis, callbacks) }
        .use { fixture ->
          fixture.render()
          fixture.scrollBy(100_000f, "analysis-file-table")
          fixture.render()
          assertTrue(fixture.hasText(".env"))
          val lastFile = fixture.taggedBounds("analysis-file-row-.env")
          val fileTable = fixture.taggedBounds("analysis-file-table")
          assertTrue(
              lastFile.top >= fileTable.top && lastFile.bottom <= fileTable.bottom,
              "The final file row must be reachable inside the bounded file list")
          assertEquals(0, actions, "Inner-list scrolling must not dispatch actions")
        }
  }

  @Test
  fun fileListKeepsPathAnchorAcrossDisposalAndRefreshWithoutEagerlyComposingRows() {
    val files =
        (0 until 500).map { index ->
          AnalysisSelectableFile(
              "src/file-${index.toString().padStart(3, '0')}.go",
              "",
              selectionStageFixture("missing", "No saved evidence."))
        }
    val selection = mutableStateOf(selectionFixture().copy(files = files))
    val visible = mutableStateOf(true)
    val view = AnalysisFilesViewState()
    var requests = 0
    ComposeVisualFixture(1024, 768) {
          if (visible.value)
              AnalysisFileSelector(
                  ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection.value)),
                  AnalysisWorkspaceActions(
                      { _, _ -> requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ },
                      refreshStatus = { error("Unexpected status refresh") }),
                  320.dp,
                  view = view)
        }
        .use { fixture ->
          fixture.render()
          assertEquals(500, view.listState.layoutInfo.totalItemsCount)
          assertTrue(view.listState.layoutInfo.visibleItemsInfo.size < 500)
          fixture.scrollBy(100_000f, "analysis-file-table")
          fixture.render()
          assertTrue(fixture.hasText(files.last().path))
          fixture.scrollBy(-250f, "analysis-file-table")
          fixture.render()
          val anchor = view.listState.layoutInfo.visibleItemsInfo.first().key as String
          assertTrue(anchor != files.first().path)
          visible.value = false
          fixture.render()
          visible.value = true
          fixture.render()
          assertEquals(anchor, view.listState.layoutInfo.visibleItemsInfo.first().key)
          selection.value = selection.value.copy(files = files.drop(10))
          fixture.render()
          assertEquals(anchor, view.listState.layoutInfo.visibleItemsInfo.first().key)
          assertEquals(490, view.listState.layoutInfo.totalItemsCount)
          fixture.clickDescription("Collapse Files")
          fixture.render()
          selection.value = selection.value.copy(files = files.drop(20))
          fixture.render()
          fixture.clickDescription("Expand Files")
          fixture.render()
          assertEquals(anchor, view.listState.layoutInfo.visibleItemsInfo.first().key)
          view.filter = AnalysisFileFilter.Attention
          fixture.render()
          assertEquals(anchor, view.listState.layoutInfo.visibleItemsInfo.first().key)
          view.query = "file-000"
          fixture.render()
          assertEquals(0, view.listState.layoutInfo.totalItemsCount)
          view.query = ""
          view.filter = AnalysisFileFilter.All
          fixture.render()
          assertEquals(files[20].path, view.listState.layoutInfo.visibleItemsInfo.first().key)
          fixture.scrollBy(100_000f, "analysis-file-table")
          fixture.render()
          assertTrue(fixture.hasText(files.last().path))
          val removedAnchor = view.listState.layoutInfo.visibleItemsInfo.first().key as String
          selection.value =
              selection.value.copy(
                  files = selection.value.files.filterNot { it.path == removedAnchor })
          fixture.render()
          fixture.render()
          assertEquals(files[20].path, view.listState.layoutInfo.visibleItemsInfo.first().key)
          assertEquals(479, view.listState.layoutInfo.totalItemsCount)
          fixture.render()
          assertEquals(320f, fixture.taggedBounds("analysis-file-table").height, 1f)
          assertEquals(0, requests)
        }
  }

  @Test
  fun roundedSummaryUsesTheProductionFrameAndSelectedSummaryDestination() {
    listOf(1600 to 1000, 1440 to 900, 1000 to 760, 999 to 760, 800 to 650, 1280 to 600).forEach {
        (width, height) ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        ComposeVisualFixture(width, height, scale) { RoundedSummaryVisualFixture(width.toFloat()) }
            .use { fixture ->
              fixture.render("summary-frame-$width-$height-$scale")
              assertTrue(fixture.hasDescription("Summary tool window, selected"))
              assertTrue(fixture.hasDescription("Analysis tool window, not selected"))
              fixture.assertTextFits(visualFixtureProject.name)
              fixture.assertTextFits("Analysis coverage")
              assertFalse(fixture.hasEditableText())
              if (width == 1600 && scale == 1f) {
                fixture.assertReferenceSummaryGeometry()
                fixture.assertRailLabelsOrdered()
              }
            }
      }
    }
  }

  @Test
  fun summaryCoverageAndResultsShareOneRegionAtWideAndCompactWidths() {
    for ((width, scale) in listOf(1440 to 1f, 800 to 1.5f)) {
      ComposeVisualFixture(width, 1600, scale) {
            ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
          }
          .use { fixture ->
            fixture.render("summary-coverage-results-$width-$scale")
            val region = fixture.taggedBounds("summary-coverage-results")
            val coverage = fixture.taggedBounds("analysis-summary")
            val results = fixture.taggedBounds("summary-results")
            val provenance = fixture.taggedBounds("summary-findings-provenance")
            assertEquals(1, fixture.tagCount("summary-coverage-results"))
            assertEquals(region.left, coverage.left, 1f)
            assertTrue(coverage.top >= region.top)
            if (width == 1440) {
              assertTrue(coverage.right <= results.left)
              assertTrue(results.right <= region.right)
            } else {
              assertEquals(region.left, results.left, 1f)
              assertEquals(region.width, coverage.width, 1f)
              assertEquals(region.width, results.width, 1f)
              assertTrue(coverage.bottom < results.top)
            }
            assertTrue(results.bottom <= region.bottom)
            assertTrue(provenance.top >= results.top && provenance.bottom <= results.bottom)
            listOf("Bugs", "Performance", "Security").forEach {
              assertTrue(fixture.hasDescription("View $it results"))
            }
            assertTrue(
                fixture.hasText("Overall findings · 2 tool-reported issues · 4 AI suggestions"))
          }
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
    ComposeVisualFixture(1440, 1600) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
          fixture.awaitDescription(
              "Architecture diagram preview\n" + visualFixtureOverview.analysis.architecture)
          fixture.awaitDescription(
              "Flow 1 diagram preview\n" + visualFixtureOverview.analysis.flows.first())
          fixture.render("summary-dashboard-preview-1440")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription(
              "Architecture diagram\n" + visualFixtureOverview.analysis.architecture)
          fixture.clickDescription("Close Architecture diagram")
          fixture.clickDescription("Expand Flow 1 diagram")
          fixture.awaitDescription(
              "Flow 1 diagram\n" + visualFixtureOverview.analysis.flows.first())
          fixture.clickDescription("Close Flow 1 diagram")
          fixture.render("summary-dashboard-1440")
          listOf(
                  "Analysis coverage",
                  visualFixtureProject.name,
                  "Architecture",
                  "Packages / modules",
                  "Flows",
                  "Engineering insight",
                  "Validate before storage.")
              .forEach { assertTrue(fixture.hasText(it), it) }
          fixture.assertTextSharesRowBefore(visualFixtureProject.name, "Go project")
          assertTrue(fixture.hasText("go.mod · 23 indexed files · 1,800 lines · Go · Markdown"))
          assertTrue(
              fixture.hasText("Overall findings · 2 tool-reported issues · 4 AI suggestions"))
          fixture.assertTextAbove("Summary", visualFixtureProject.name)
          fixture.assertTextAbove(visualFixtureProject.name, "Analysis coverage")
          fixture.assertSummaryStatusPlacement("Outdated")
          fixture.assertWideSummaryCoverageLayout()
          fixture.assertReferenceSummaryGeometry()
          fixture.assertNarrativeSectionOrder(withInsight = true)
          listOf("Bugs", "Performance", "Security").forEach {
            assertTrue(fixture.hasDescription("View $it results"))
            assertTrue(fixture.hasText(it))
          }
          assertFalse(fixture.hasText("Entry points"))
          assertFalse(fixture.hasText("Next steps"))
          assertFalse(fixture.hasText("cmd/server/main.go"))
          assertFalse(fixture.hasText("Review boundary validation."))
          assertTrue(
              fixture.hasDescription(
                  "Outdated · ${projectSummaryPresentation(overview, visualFixtureProject).analysisMessage}"))
          fixture.assertTextAbove("Mechanism", "Why it matters here")
          assertFalse(fixture.hasText("Trade-off or failure mode"))
          assertFalse(fixture.hasText("Transferable lesson"))
          assertTrue(fixture.hasText("AI interpretation"))
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertTrue(fixture.hasText("Trade-off or failure mode"))
          assertTrue(fixture.hasText("Transferable lesson"))
          assertFalse(fixture.hasText("Type: Go · Build: go.mod · Languages: Go · Markdown"))
          assertTrue(fixture.hasText(visualFixtureProject.name))
          assertFalse(fixture.hasText("Project purpose"))
          assertFalse(fixture.hasText("Interpretation details"))
          assertFalse(fixture.hasText("Analysis"))
          assertFalse(fixture.hasText("Project understanding"))
          assertFalse(fixture.hasText("Risks · AI suggestions"))
          assertFalse(fixture.hasText("MEDIUM · Input validation is incomplete."))
          assertFalse(fixture.hasText("Show full response"))
          assertFalse(fixture.hasEditableText())
          assertFalse(fixture.tryClick("Engineering insight"))
          assertFalse(fixture.hasEditableText())
        }
  }

  @Test
  fun summaryLowerCompositionUsesTheFullWidthWhenOnlyArchitectureContentExists() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(flows = emptyList(), engineeringInsight = null))
    ComposeVisualFixture(1440, 900) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.render("summary-lower-one-sided-1440")
          val introduction = fixture.taggedBounds("summary-introduction")
          val architecture = fixture.taggedBounds("summary-architecture")
          val modules = fixture.taggedBounds("summary-modules")
          assertEquals(introduction.width, architecture.width, 1f)
          val findings = fixture.taggedBounds("summary-selected-findings")
          assertTrue(modules.right <= findings.left)
          assertEquals(modules.top, findings.top, 1f)
          assertEquals(0, fixture.tagCount("summary-insight"))
          assertEquals(0, fixture.tagCount("summary-flows"))
          assertTrue(architecture.bottom <= modules.top)
        }
  }

  @Test
  fun summaryLowerCompositionMeasuresRightOnlyContentWithoutAnOrphanGap() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    architecture = "",
                    components = emptyList(),
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate requests before persistence.",
                            whyItMattersHere = "Invalid input stays outside the repository.")))
    for ((width, height, scale) in listOf(Triple(1440, 900, 1f), Triple(800, 650, 1.5f))) {
      ComposeVisualFixture(width, height, scale) {
            ProjectSummaryPane(overview, visualFixtureProject, {})
          }
          .use { fixture ->
            fixture.render("summary-right-only-$width-$scale")
            fixture.revealText("Engineering insight")
            fixture.render()
            assertEquals(0, fixture.tagCount("summary-architecture"))
            assertEquals(0, fixture.tagCount("summary-modules"))
            val item = fixture.taggedBounds("summary-lower-composition")
            val insight = fixture.taggedBounds("summary-insight")
            assertEquals(item.top, insight.top, 1f, "Insight starts at the item top")
            assertEquals(item.width, insight.width, 1f)
            fixture.revealText("Selected findings")
            fixture.render()
            assertEquals(item.width, fixture.taggedBounds("summary-selected-findings").width, 1f)
            fixture.revealText("Flows")
            fixture.scrollBy(100_000f)
            fixture.render()
            val flows = fixture.taggedBounds("summary-flows")
            assertEquals(item.width, flows.width, 1f)
            assertTrue(flows.bottom <= height, "Flows must be reachable at $width x $height")
            assertEquals(1, fixture.scrollableContentCount())
          }
    }
  }

  @Test
  fun summaryLowerCompositionStacksFullWidthAtCompactLargeText() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate requests before persistence.",
                            whyItMattersHere = "Invalid input stays outside the repository.")))
    ComposeVisualFixture(800, 3_000, 1.5f) {
          ProjectSummaryPane(overview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render("summary-lower-compact-800-150")
          fixture.assertNarrativeSectionOrder(withInsight = true)
          assertEquals(
              1, fixture.scrollableContentCount(), "Summary must keep one page scroll owner")
        }
  }

  @Test
  fun summaryNarrativeDiagramControlsSurviveWideStackedWideResize() {
    val source = visualFixtureOverview.analysis.architecture
    val flow = visualFixtureOverview.analysis.flows.first()
    var navigations = 0
    ComposeVisualFixture(1440, 2600) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, { navigations++ })
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          fixture.clickDescription("Zoom in Architecture")
          fixture.clickDescription("Mermaid source for Architecture")
          fixture.render("summary-diagram-state-wide")
          assertTrue(fixture.hasText("125%"))
          assertTrue(fixture.hasText("Hide Mermaid"))

          for (width in listOf(800, 1440)) {
            fixture.resize(width, 2600)
            fixture.render("summary-diagram-state-$width")
            fixture.awaitDescription("Close Architecture diagram")
            assertTrue(fixture.hasDescription("Architecture diagram\n$source"))
            assertTrue(fixture.hasText("125%"), "Zoom must survive reflow at $width")
            assertTrue(fixture.hasText("Hide Mermaid"), "Source disclosure must survive at $width")
          }
          fixture.clickDescription("Close Architecture diagram")
          fixture.assertNarrativeSectionOrder()
          fixture.clickDescription("Expand Flow 1 diagram")
          fixture.awaitDescription("Flow 1 diagram\n$flow")
          fixture.clickDescription("Close Flow 1 diagram")
          assertEquals(0, navigations, "Resizing and disclosures must not navigate")
        }
  }

  @Test
  fun categoryAndNarrativeGridsFollowLocalWidthWithoutDispatchingOnReflow() {
    val overview = visualFixtureOverview
    val summaryRun = analysisRunFixture().copy(status = "failed")
    for (scale in listOf(1f, 1.25f, 1.5f)) {
      val summaryBoundary = (200.dp * 3 * scale + 24.dp).value.toInt()
      val analysisBoundary = (200.dp * 3 * scale + 16.dp).value.toInt()
      for (delta in listOf(-1, 0, 1)) {
        // Summary's page has 24dp gutters on both sides; Analysis cards are tested locally.
        val width = summaryBoundary + 48 + delta
        val navigations = mutableListOf<Workspace>()
        ComposeVisualFixture(width, 2400, scale) {
              ProjectSummaryPane(overview, visualFixtureProject, navigations::add, run = summaryRun)
            }
            .use { fixture ->
              fixture.render("summary-category-local-$width-$scale")
              val cards =
                  AnalysisResultType.entries.map {
                    fixture.taggedBounds("summary-metric-${it.workspace.name}")
                  }
              if (delta < 0) {
                assertTrue(cards[0].bottom <= cards[1].top)
                assertTrue(cards[1].bottom <= cards[2].top)
              } else {
                assertEquals(cards[0].top, cards[1].top, 1f)
                assertEquals(cards[1].top, cards[2].top, 1f)
              }
              cards.forEach {
                assertTrue(it.width >= 200 * scale - 2, "Readable category width: $it")
              }
              assertEquals(0, navigations.size)
              AnalysisResultType.entries.forEach { type ->
                fixture.clickVisibleDescription("View ${type.workspace.name} results")
              }
              assertEquals(AnalysisResultType.entries.map { it.workspace }, navigations)
            }
        val local = analysisBoundary + delta
        val analysisNavigation = mutableListOf<Workspace>()
        ComposeVisualFixture(local, 1000, scale) {
              AnalysisCategoryPanels(
                  AnalysisWorkspacePaneState(visualFixtureProject, ProjectAnalysisRunState()),
                  analysisNavigation::add)
            }
            .use { fixture ->
              fixture.render("analysis-category-local-$local-$scale")
              val cards =
                  AnalysisResultType.entries.map {
                    fixture.taggedBounds("analysis-category-${it.category}")
                  }
              if (delta < 0) {
                assertTrue(cards[0].bottom <= cards[1].top)
                assertTrue(cards[1].bottom <= cards[2].top)
              } else {
                assertEquals(cards[0].top, cards[1].top, 1f)
                assertEquals(cards[1].top, cards[2].top, 1f)
              }
              assertEquals(3, fixture.textCount("—"), "Unknown counts must not become zero")
              assertEquals(0, analysisNavigation.size)
              AnalysisResultType.entries.forEach { type ->
                fixture.clickVisibleDescription("View ${type.workspace.name} results")
              }
              assertEquals(AnalysisResultType.entries.map { it.workspace }, analysisNavigation)
            }
      }
      for (width in listOf(800, 1440)) {
        ComposeVisualFixture(width, 2400, scale) {
              ProjectSummaryPane(overview, visualFixtureProject, {})
            }
            .use { fixture ->
              fixture.render("summary-narratives-single-column-$width-$scale")
              fixture.assertNarrativeSectionOrder()
            }
      }
    }
  }

  @Test
  fun f09SummaryCoverageProductionRenderMatrix() {
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    val selected =
        selectionFixture()
            .copy(
                projectId = visualFixtureProject.projectId,
                projectRevision = visualFixtureProject.projectRevision,
                files =
                    (0 until 40).map { index ->
                      AnalysisSelectableFile(
                          "src/deeply/nested/long/package/file-%02d.go".format(index),
                          "",
                          selectionStageFixture(
                              if (index == 0) "fresh" else "stale", "Saved evidence."))
                    })
    val states =
        listOf(
            "mixed" to (visualFixtureOverview to AnalysisSelectionState(selection = selected)),
            "current" to
                (visualFixtureOverview.copy(
                    analysisCoverage = AnalysisCoverage(total = 23, fresh = 23)) to
                    AnalysisSelectionState()),
            "unavailable" to
                (visualFixtureOverview.copy(analysisCoverage = AnalysisCoverage()) to
                    AnalysisSelectionState()),
            "empty" to
                (visualFixtureOverview to
                    AnalysisSelectionState(selection = selected.copy(files = emptyList()))),
            "failed" to
                (visualFixtureOverview to
                    AnalysisSelectionState(
                        selection = selected,
                        error = "Selection refresh timed out.",
                        failure = AnalysisSelectionFailure.Read)))
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        // Every state is captured at every size; density and text scale vary independently.
        for ((state, data) in states) {
          val label = "f09-summary-$state-$width-$height-$scale-${density}x"
          val overview = data.first
          val selectionState = data.second
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    ProjectSummaryPane(
                        overview,
                        visualFixtureProject,
                        {},
                        analysisState = ProjectAnalysisRunState(fileSelection = selectionState))
                  }
              .use { fixture ->
                fixture.render("$label-closed")
                assertTrue(fixture.hasText("Analysis coverage"), label)
                assertTrue(fixture.hasText("Saved coverage · Coverage, not a health score."), label)
                when (state) {
                  "mixed",
                  "failed" -> {
                    fixture.revealText("Analysis coverage")
                    fixture.clickVisibleDescription("Outdated, 39 files")
                    fixture.render("$label-inspected")
                    assertTrue(fixture.hasText("Outdated · 39 of 40 selected files"), label)
                    assertTrue(fixture.hasText("src/deeply/nested/long/package/file-01.go"), label)
                    fixture.scrollBy(240f * density, "summary-inspection-paths")
                    fixture.render()
                    assertTrue(fixture.verticalScrollValue("summary-inspection-paths") > 0f, label)
                    fixture.assertTextContrast("Outdated · 39 of 40 selected files", StrongSurface)
                    fixture.revealText("View analysis")
                    fixture.assertTextFits("View analysis")
                    fixture.revealText("Analysis coverage")
                    assertTrue(fixture.requestDescriptionFocus("Outdated, 39 files"), label)
                    fixture.render("$label-focused")
                    assertTrue(fixture.isFocusedControl("Outdated, 39 files"), label)
                  }
                  "current" -> assertTrue(fixture.hasText("100%"), label)
                  "unavailable" -> assertTrue(fixture.hasText("File counts unavailable"), label)
                  "empty" -> assertTrue(fixture.hasText("0 selected files"), label)
                }
                if (state == "failed")
                    assertTrue(fixture.hasText("File selection needs attention"), label)
                fixture.revealText("Open Editor")
                fixture.render("$label-lower")
                fixture.assertTextFits("Open Editor")
                assertTrue(fixture.hasText("Change lifecycle"), label)
              }
        }
      }
    }
  }

  @Test
  fun f09CoverageReflowsOnBothSidesOfLocalBreakpoints() {
    // Summary page gutter is 48 dp; coverage panel padding is another 32 dp.
    for (width in listOf(379, 381, 399, 401, 1127, 1129)) {
      ComposeVisualFixture(width, 900) {
            ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
          }
          .use { fixture ->
            fixture.render("f09-boundary-$width")
            val heading = fixture.firstVisibleTextBounds("Analysis coverage")
            val status = fixture.taggedBounds("summary-analysis-status")
            val dial = fixture.taggedBounds("summary-coverage-dial")
            val readout = fixture.firstVisibleTextBounds("16 of 23 selected files are up to date")
            val coverage = fixture.taggedBounds("analysis-summary")
            val results = fixture.taggedBounds("summary-results")
            if (width < 380) assertTrue(heading.bottom <= status.top)
            else assertTrue(heading.right <= status.left)
            if (width < 400) assertTrue(dial.bottom <= readout.top)
            else assertTrue(dial.right <= readout.left)
            if (width < 1128) assertTrue(coverage.bottom <= results.top)
            else assertTrue(coverage.right <= results.left)
            fixture.revealText("View analysis")
            fixture.assertTextFits("View analysis")
          }
    }
  }

  @Test
  fun summaryDashboardShowsCompleteLongProseInThePage() {
    val longPurpose = "This purpose remains readable directly in the dashboard. ".repeat(60)
    val overview =
        visualFixtureOverview.copy(
            analysis = visualFixtureOverview.analysis.copy(purpose = longPurpose))
    ComposeVisualFixture(800, 650, 1.5f) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
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
      ComposeVisualFixture(1280, 600) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
          .use { fixture ->
            fixture.render("summary-dashboard-$status-1280-600")
            listOf(
                    "16 up to date" to Success,
                    "4 outdated" to Warning,
                    "3 not analyzed" to SecondaryText)
                .forEach { (label, tint) ->
                  assertEquals(1, fixture.textCount(label))
                  fixture.assertColorVisible(tint)
                  fixture.assertTextContrast(label, Panel)
                }
            // Coverage remains stale regardless of the independent project-description status.
            assertFalse(fixture.hasText("File counts unavailable"))
            assertTrue(
                fixture.hasDescription(
                    "Analysis coverage: 16 of 23 selected files are up to date · 70% · 16 up to date · 4 outdated · 3 not analyzed"))
            if (status != "fresh")
                fixture.assertTextFits(
                    projectSummaryPresentation(overview, null).interpretationMessage, maxLines = 3)
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
              visualFixtureProject,
              {})
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("0 up to date"))
          assertTrue(fixture.hasText("File counts unavailable"))
          assertTrue(fixture.hasDescription("Analysis coverage: File counts unavailable"))
          assertFalse(fixture.hasText("100%"))
          fixture.assertSummaryStatusPlacement("Coverage unavailable")
          assertTrue(
              fixture.hasText("Overall findings · 2 tool-reported issues · 4 AI suggestions"))
          assertTrue(fixture.hasText(visualFixtureProject.name))
          fixture.assertUniformSummaryCards(3)
          fixture.assertSummaryStatusPlacement("Coverage unavailable")
        }
    ComposeVisualFixture(1440, 1100) { ProjectSummaryPane(null, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Analysis coverage"))
          assertTrue(fixture.hasText("File counts unavailable"))
          assertTrue(fixture.hasText("—"))
        }
  }

  @Test
  fun summaryFileEvidenceRendersSavedRowsAndRouteAtReadableWidths() {
    val path = "src/" + "日本語-long-directory/".repeat(9) + "handler.go"
    val selection =
        selectionFixture()
            .copy(
                files =
                    listOf(
                        AnalysisSelectableFile(
                            path, "", selectionStageFixture("stale", "Source changed.")),
                        AnalysisSelectableFile(
                            "a.go", "", selectionStageFixture("fresh", "Current"))))
    for ((width, height, scale) in
        listOf(
            Triple(1600, 1000, 1f),
            Triple(1440, 900, 1.25f),
            Triple(1024, 768, 1f),
            Triple(800, 650, 1.5f),
            Triple(1280, 600, 1.25f))) {
      ComposeVisualFixture(width, height, scale) {
            ProjectSummaryPane(null, analysisProjectFixture(), {}, fileSelection = selection)
          }
          .use { fixture ->
            fixture.render("summary-file-evidence-$width-$height-$scale")
            fixture.revealText("File evidence")
            fixture.render("summary-file-evidence-revealed-$width-$height-$scale")
            assertTrue(fixture.hasText(path))
            assertTrue(fixture.hasText("Showing 2 of 2 selected files · saved status"))
            assertTrue(
                fixture.hasText(
                    "Outdated · ${analysisFileStatus(selection.files.first()).explanation}"))
            fixture.revealText("All files")
            fixture.assertTextFits("All files")
            val panel = fixture.taggedBounds("summary-file-evidence")
            val action = fixture.taggedBounds("summary-all-files")
            assertTrue(action.left >= panel.left && action.right <= panel.right)
            assertTrue(action.bottom <= panel.bottom)
          }
    }
  }

  @Test
  fun f10SummaryEvidenceStatesReflowWithLongTextAndReachableLowerActions() {
    val project = resultProjectFixture()
    val path = "src/" + "日本語-long-directory/".repeat(8) + "handler.go"
    val title = "A retained finding about a long project path and its source identity ".repeat(3)
    val page = resultPageFixture("bugs")
    val finding =
        page.semantic
            .single()
            .copy(title = title, location = FindingLocation(path, startLine = 118))
    val section = page.section.copy(results = page.results!!.copy(semantic = listOf(finding)))
    val run = requireNotNull(page.run)
    val selection =
        selectionFixture()
            .copy(
                files =
                    listOf(
                        AnalysisSelectableFile(
                            path, "", selectionStageFixture("stale", "Source changed.")),
                        AnalysisSelectableFile(
                            "a.go", "", selectionStageFixture("fresh", "Current"))))
    val app =
        DesktopState(
            projectState = ProjectWorkspaceState(project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = run, sections = mapOf(AnalysisResultKey("bugs") to section)))
    val preview = summaryFindingPreview(app)
    val row = preview.rows.single()
    assertEquals(title, row.title)
    val states =
        listOf(
            "populated" to AnalysisSelectionState(selection = selection),
            "aggregate-only" to AnalysisSelectionState(),
            "confirmed-empty" to
                AnalysisSelectionState(selection = selection.copy(files = emptyList())),
            "failed-retained" to
                AnalysisSelectionState(
                    selection = selection,
                    error = "Saved selection read failed",
                    failure = AnalysisSelectionFailure.Read))
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        val label = "f10-summary-$width-$height-$scale-${density}x"
        var destination: Workspace? = null
        var selected: SummaryFindingTarget? = null
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  ProjectSummaryPane(
                      ProjectOverview(
                          project.projectId,
                          project.projectRevision,
                          analysisCoverage = AnalysisCoverage(total = 2, fresh = 1, stale = 1)),
                      project,
                      { destination = it },
                      run = run,
                      sections = app.analysisRun.sections,
                      analysisState =
                          ProjectAnalysisRunState(fileSelection = states.first().second),
                      findingState = app,
                      onFindingSelected = { selected = it })
                }
            .use { fixture ->
              fixture.render("$label-populated")
              fixture.revealText("File evidence", "summary-scroll")
              fixture.render("$label-ledger")
              assertTrue(fixture.hasText(path), label)
              assertTrue(fixture.hasText("Showing 2 of 2 selected files · saved status"), label)
              if (width == 800 && scale == 1.5f) {
                fixture.revealText(path, "summary-scroll")
                assertTrue(
                    fixture.copyTextByDragging(path).isNotBlank(),
                    "Ledger path must remain selectable")
              }
              fixture.revealText("All files", "summary-scroll")
              fixture.assertTextFits("All files")
              fixture.revealText("Selected findings", "summary-scroll")
              fixture.revealText(title, "summary-scroll")
              fixture.assertTextFits(title, 12)
              fixture.revealText(row.location, "summary-scroll")
              fixture.render("$label-findings")
              fixture.assertTextFits(row.location, 12)
              fixture.revealText("All Security results", "summary-scroll")
              fixture.assertTextFits("All Security results")
              fixture.revealText("Open Editor", "summary-scroll")
              fixture.assertTextFits("Open Editor")
              assertTrue(fixture.requestFocus("Open Editor"))
              fixture.render("$label-lower-focused")
              fixture.assertColorVisible(FocusAccent)
              assertEquals(null, destination)
              assertEquals(null, selected)
            }
      }
    }
    // Use the same production pane for state captures; each state has its own honest ledger.
    for ((name, state) in states) {
      val findingsState =
          if (name == "failed-retained")
              app.copy(
                  analysisRun =
                      app.analysisRun.copy(
                          sections =
                              mapOf(
                                  AnalysisResultKey("bugs") to
                                      section.copy(error = "Saved detail read failed"))))
          else app
      ComposeVisualFixture(800, 650, 1.5f) {
            ProjectSummaryPane(
                ProjectOverview(
                    project.projectId,
                    project.projectRevision,
                    analysisCoverage = AnalysisCoverage(total = 2, fresh = 1, stale = 1)),
                project,
                {},
                analysisState = ProjectAnalysisRunState(fileSelection = state),
                findingState = findingsState,
                onFindingSelected = {})
          }
          .use { fixture ->
            fixture.render("f10-summary-$name-800-650-150")
            fixture.revealText("File evidence", "summary-scroll")
            fixture.render("f10-summary-$name-ledger-800-650-150")
            when (name) {
              "aggregate-only" ->
                  assertTrue(
                      fixture.hasText(
                          "File paths unavailable · 2 files in saved aggregate coverage. Load a confirmed selection to inspect file evidence."))
              "confirmed-empty" ->
                  assertTrue(fixture.hasText("No files selected in the confirmed selection."))
              "failed-retained" -> {
                assertTrue(fixture.hasText(path))
                assertTrue(
                    fixture.hasText(
                        "File selection load failed · Showing last confirmed selection. Saved selection read failed"))
              }
              else -> assertTrue(fixture.hasText(path))
            }
            fixture.revealText("All files", "summary-scroll")
            fixture.render("f10-summary-$name-ledger-action-800-650-150")
            fixture.assertTextFits("All files")
            if (name == "failed-retained") {
              fixture.revealText(
                  "Bugs · Partial · 1 loaded · Saved details unavailable · Saved detail read failed",
                  "summary-scroll")
              fixture.revealText(title, "summary-scroll")
              fixture.render("f10-summary-failed-retained-finding-800-650-150")
              assertTrue(fixture.hasText(title))
              assertTrue(
                  fixture.hasText(
                      "Evidence origin unavailable · Saved details unavailable · Partial"))
            }
          }
    }
  }

  @Test
  fun f10ModulesAndLoadedFindingsPairOnlyWhenBothColumnsAreReadable() {
    val page = resultPageFixture("bugs")
    val title = "Loaded finding in the selected project"
    val section =
        page.section.copy(
            results =
                page.results!!.copy(semantic = listOf(page.semantic.single().copy(title = title))))
    val project = requireNotNull(page.project)
    val overview =
        visualFixtureOverview.copy(
            projectId = project.projectId, projectRevision = project.projectRevision)
    val app =
        DesktopState(
            projectState = ProjectWorkspaceState(project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run, sections = mapOf(AnalysisResultKey("bugs") to section)))
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (density in if (width == 800 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)) {
        ComposeVisualFixture(
                (width * density).toInt(), (height * density).toInt(), scale, density) {
                  ProjectSummaryPane(overview, project, {}, findingState = app)
                }
            .use { fixture ->
              fixture.render()
              fixture.revealText(title, "summary-scroll")
              val label = "f10-modules-findings-$width-$height-$scale-${density}x"
              fixture.render(label)
              assertTrue(fixture.hasText(title), label)
              val row = fixture.taggedBounds("summary-evidence-row")
              val modules = fixture.taggedBounds("summary-modules")
              val findings = fixture.taggedBounds("summary-selected-findings")
              if (summaryEvidencePanelsStacked(row.width.dp / density, scale)) {
                assertEquals(row.width, modules.width, 2f, label)
                assertEquals(row.width, findings.width, 2f, label)
                assertTrue(modules.bottom <= findings.top, label)
              } else {
                assertEquals(modules.top, findings.top, 2f, label)
                assertTrue(modules.right <= findings.left, label)
                assertTrue(modules.width > 0 && findings.width > 0, label)
              }
              fixture.revealText("All Security results", "summary-scroll")
              fixture.assertTextFits("All Security results")
            }
      }
    }
    for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (delta in listOf(-1, 0, 1)) {
        val width = (960 * scale).toInt() + 48 + delta
        ComposeVisualFixture(width, 1800, scale) {
              ProjectSummaryPane(overview, project, {}, findingState = app)
            }
            .use { fixture ->
              fixture.render()
              fixture.revealText("Selected findings", "summary-scroll")
              fixture.render("f10-modules-findings-boundary-$width-$scale")
              val row = fixture.taggedBounds("summary-evidence-row")
              val modules = fixture.taggedBounds("summary-modules")
              val findings = fixture.taggedBounds("summary-selected-findings")
              if (summaryEvidencePanelsStacked(row.width.dp, scale)) {
                assertTrue(modules.bottom <= findings.top)
              } else {
                assertTrue(modules.right <= findings.left)
              }
            }
      }
    }
  }

  @Test
  fun f10LedgerAndCategoriesReflowAtLocalBreakpoints() {
    val project = resultProjectFixture()
    val selection = selectionFixture()
    for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (width in
          listOf(
              (1080 * scale).toInt() - 60,
              (1080 * scale).toInt() + 60,
              (624 * scale).toInt() - 60,
              (624 * scale).toInt() + 60)) {
        ComposeVisualFixture(width, 1800, scale) {
              ProjectSummaryPane(null, project, {}, fileSelection = selection)
            }
            .use { fixture ->
              fixture.render("f10-boundary-$width-$scale")
              val coverage = fixture.taggedBounds("summary-coverage-column")
              val results = fixture.taggedBounds("summary-results")
              val ledger = fixture.taggedBounds("summary-file-evidence")
              assertTrue(ledger.left >= results.left && ledger.right <= results.right + 1f)
              if (coverageResultsStacked(
                  fixture.taggedBounds("summary-coverage-results").width.dp, scale)) {
                assertTrue(coverage.bottom <= results.top)
              } else {
                assertTrue(coverage.right <= results.left)
              }
              fixture.revealText("All files", "summary-scroll")
              fixture.assertTextFits("All files")
              fixture.revealText("All Security results", "summary-scroll")
              fixture.assertTextFits("All Security results")
            }
      }
    }
  }

  @Test
  fun summaryCoverageWithNoSelectedFilesIsNotUnknownOrUpToDate() {
    val overview = visualFixtureOverview.copy(analysisCoverage = AnalysisCoverage(total = 0))
    val selection =
        AnalysisFileSelection(
            projectId = overview.projectId,
            projectRevision = overview.projectRevision,
            selectionId = "none",
            excludedPaths = emptyList(),
            files = emptyList(),
            editable = true)
    ComposeVisualFixture(1440, 1100) {
          ProjectSummaryPane(overview, visualFixtureProject, {}, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render("summary-no-selection-1440")
          fixture.assertSummaryStatusPlacement("No files selected")
          assertTrue(fixture.hasText("0 selected files"))
          assertTrue(fixture.hasDescription("Analysis coverage: 0 selected files"))
          assertFalse(fixture.hasText("100%"))
          assertFalse(fixture.hasText("0 up to date"))
          assertFalse(fixture.hasText("File counts unavailable"))
        }
  }

  @Test
  fun dialDistinguishesAllCurrentZeroCurrentAndInvalidAggregate() {
    val project = visualFixtureProject
    listOf(
            AnalysisCoverage(total = 23, fresh = 23) to
                "Analysis coverage: 23 of 23 selected files are up to date · 100% · 23 up to date",
            AnalysisCoverage(total = 23, missing = 23) to
                "Analysis coverage: 0 of 23 selected files are up to date · 0% · 23 not analyzed",
            AnalysisCoverage(total = 23, unavailable = 23) to
                "Analysis coverage: 0 of 23 selected files are up to date · 0% · 23 unavailable")
        .forEach { (counts, description) ->
          ComposeVisualFixture(1024, 768) {
                ProjectSummaryPane(
                    visualFixtureOverview.copy(analysisCoverage = counts), project, {})
              }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.hasDescription(description))
                assertTrue(fixture.hasText(if (counts.fresh == 23) "100%" else "0%"))
                assertTrue(fixture.hasText("Saved coverage · Coverage, not a health score."))
              }
        }
    ComposeVisualFixture(1024, 768) {
          ProjectSummaryPane(
              visualFixtureOverview.copy(analysisCoverage = AnalysisCoverage(total = 1, fresh = 2)),
              project,
              {})
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Analysis coverage: File counts unavailable"))
          assertFalse(fixture.hasText("100%"))
        }
  }

  @Test
  fun summaryFailureQualificationFitsBesideRetainedSavedCoverageAndRunStatus() {
    val project = analysisProjectFixture()
    val selection = selectionFixture().copy(excludedPaths = listOf("main.go"))
    val run = analysisRunFixture().copy(status = "running")
    for ((width, scale) in listOf(1440 to 1f, 800 to 1.25f, 430 to 1.5f)) {
      ComposeVisualFixture(width, 900, scale) {
            ProjectSummaryPane(
                null,
                project,
                {},
                run = run,
                analysisState =
                    ProjectAnalysisRunState(
                        run = run,
                        fileSelection =
                            AnalysisSelectionState(
                                selection,
                                error = "Selection refresh timed out.",
                                failure = AnalysisSelectionFailure.Read)))
          }
          .use { fixture ->
            fixture.render("summary-retained-failure-$width-$scale")
            fixture.assertTextFits("1 of 1 selected files are up to date")
            fixture.assertTextFits("100%")
            fixture.assertTextFits("File selection needs attention", maxLines = 2)
            fixture.assertTextFits(
                "Current analysis run: Running · separate from saved coverage.", maxLines = 4)
            assertTrue(
                fixture.hasText(
                    "File selection load failed · Showing last confirmed selection. Selection refresh timed out."))
            assertEquals(1, fixture.tagCount("summary-analysis-run-strip"))
            fixture.revealText("View analysis")
            fixture.assertTextFits("View analysis")
          }
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
          ComposeVisualFixture(1280, 1100) {
                ProjectSummaryPane(overview, visualFixtureProject, {})
              }
              .use { fixture ->
                fixture.render("summary-panel-$status")
                fixture.assertSummaryStatusPlacement(label)
                assertTrue(
                    fixture.hasDescription(
                        "$label · ${projectSummaryPresentation(overview, visualFixtureProject).analysisMessage}"))
                fixture.assertTextContrast(label, Panel)
                fixture.assertColorVisible(tint)
                fixture.assertColorVisible(Panel)
              }
        }
  }

  @Test
  fun summaryIntroductionKeepsInterpretationFailureVisibleWithoutFocus() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                StructuredProjectAnalysis(status = "failed", failure = "Provider timed out."),
            analysisCoverage = AnalysisCoverage())
    val description = "Project description: failed · Provider timed out."
    ComposeVisualFixture(800, 650) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.render("summary-interpretation-failure")
          assertTrue(fixture.hasText(description))
          fixture.assertTextAbove(description, "Analysis coverage")
          assertFalse(fixture.hasEditableText())
        }
  }

  @Test
  fun summaryCategoriesStackAtCompactTextScale() {
    ComposeVisualFixture(800, 1_600, 1.5f) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render("summary-categories-compact-800-150")
          val cards =
              AnalysisResultType.entries.map { type ->
                fixture.taggedBounds("summary-metric-${type.workspace.name}")
              }
          cards.forEach { card ->
            assertTrue(card.height >= 132f, "Category cards must retain their minimum height")
            assertTrue(card.width >= 200f * 1.5f, "Stacked categories must remain readable")
          }
          cards.zipWithNext().forEach { (first, next) ->
            assertTrue(first.bottom <= next.top, "Category cards must stack without overlap")
          }
          val provenance = fixture.taggedBounds("summary-findings-provenance")
          assertTrue(
              provenance.top >= cards.maxOf { it.bottom },
              "Provenance must follow all category cards",
          )
        }
  }

  @Test
  fun summaryCategoriesGrowAtCompactLargeTextForLifecyclePriorityAndFailureDetails() {
    val (baseRun, bugs) = summaryBugFixture(listOf("high", "medium", "low"))
    val run =
        baseRun.copy(
            sections =
                baseRun.sections.map { progress ->
                  when (progress.category) {
                    "performance" -> progress.copy(status = "interrupted", findingCount = 18)
                    "security" -> progress.copy(status = "failed", findingCount = 19)
                    else -> progress
                  }
                })
    val sections =
        mapOf(
            AnalysisResultKey("bugs") to bugs,
            AnalysisResultKey("security") to AnalysisSectionState(error = "Result read failed"))

    ComposeVisualFixture(800, 1_100, 1.5f) {
          ProjectSummaryPane(
              visualFixtureOverview, resultProjectFixture(), {}, run = run, sections = sections)
        }
        .use { fixture ->
          fixture.render("summary-categories-details-compact-800-150")
          fixture.revealText("Saved details unavailable · 19 reported", "summary-scroll")
          fixture.assertSummaryCategoryContentContained()
          fixture.assertTextFits("Interrupted")
          fixture.assertTextFits("High: 1 · Medium: 1 · Low: 1", maxLines = 2)
          fixture.assertTextFits("Saved details unavailable · 19 reported")
          val bugsCard = fixture.taggedBounds("summary-metric-Bugs")
          assertTrue(
              bugsCard.height > 132f,
              "Priority details must grow the Bugs card beyond its minimum height")
        }
  }

  @Test
  fun summaryCategoryBoxesNavigateAndRefreshFromTheCurrentRun() {
    val navigations = mutableListOf<Workspace>()
    var run by
        mutableStateOf(
            analysisRunFixture()
                .copy(
                    status = "running",
                    sections =
                        AnalysisResultType.entries.mapIndexed { index, type ->
                          AnalysisSectionProgress(
                              type.category,
                              "running",
                              AnalysisRunCoverage(running = 1),
                              17 + index)
                        }))
    var sections by
        mutableStateOf(
            mapOf(AnalysisResultKey("performance") to AnalysisSectionState(loading = true)))
    val overview =
        visualFixtureOverview.copy(
            projectId = "project",
            projectRevision = "revision",
            analysis = StructuredProjectAnalysis(status = "fresh", purpose = "Current purpose"),
            analysisCoverage = AnalysisCoverage(total = 3, fresh = 3))
    ComposeVisualFixture(1440, 900) {
          ProjectSummaryPane(
              overview, resultProjectFixture(), navigations::add, run = run, sections = sections)
        }
        .use { fixture ->
          fixture.render("summary-live-running-1440")
          fixture.assertSummaryStatusPlacement("Updating")
          fixture.assertSummaryCategoryBoxesFit()
          listOf("17", "18", "19").forEach(fixture::assertTextFits)
          fixture.assertTextFits("Loading saved details · 18 reported")
          AnalysisResultType.entries.forEach { type ->
            fixture.clickVisibleDescription("View ${type.workspace.name} results")
          }
          assertEquals(AnalysisResultType.entries.map { it.workspace }, navigations)
          assertFalse(fixture.hasText("Complete"))
          assertFalse(fixture.hasText("Completed"))

          run =
              run.copy(
                  status = "completed",
                  sections =
                      AnalysisResultType.entries.mapIndexed { index, type ->
                        AnalysisSectionProgress(
                            type.category,
                            "completed",
                            AnalysisRunCoverage(succeeded = 1),
                            27 + index)
                      })
          sections = emptyMap()
          fixture.render("summary-live-completed-1440")
          listOf("17", "18", "19", "Loading saved details").forEach {
            assertFalse(fixture.hasText(it))
          }
          listOf("27", "28", "29").forEach(fixture::assertTextFits)
          fixture.assertSummaryStatusPlacement("Updated")
          fixture.assertSummaryCategoryBoxesFit()
          AnalysisResultType.entries.forEach { type ->
            val box = fixture.taggedBounds("summary-metric-${type.workspace.name}")
            val icon = fixture.taggedBounds("summary-category-icon-${type.category}")
            val name = fixture.taggedBounds("summary-category-name-${type.category}")
            val count = fixture.taggedBounds("summary-category-count-${type.category}")
            val status = fixture.taggedBounds("summary-category-status-${type.category}")
            assertTrue(box.height >= 132f, "Summary category cards must retain a minimum height")
            assertEquals(icon.top, name.top, 1f, "Summary icon and name must remain inline")
            assertTrue(count.top >= icon.bottom, "Summary count must remain below its icon row")
            assertTrue(status.top >= count.bottom, "Summary status must follow the count")
          }
          // Three result cards plus the current-run lifecycle badge.
          assertEquals(4, fixture.textCount("Completed"))
        }
  }

  @Test
  fun summaryOutdatedBadgeClearsAfterRefreshAndCategoryNamesStayVisible() {
    var overview by mutableStateOf(visualFixtureOverview)
    ComposeVisualFixture(1280, 600) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
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
          assertTrue(fixture.hasDescription("View Bugs results"))
          AnalysisResultType.entries.forEach { type ->
            assertFalse(fixture.hasDescription(type.workspace.name))
          }
          assertTrue(fixture.hasText("Performance"))
        }
  }

  @Test
  fun summaryShowsOutdatedBadgeForStaleRunEvenWhenProjectPurposeIsFresh() {
    val project = resultProjectFixture()
    val overview = visualFixtureOverview.copy(analysisCoverage = AnalysisCoverage())
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectSummaryPane(
              overview, project, {}, run = analysisRunFixture().copy(status = "stale"))
        }
        .use { fixture ->
          fixture.render("summary-outdated-run-800-150")
          fixture.assertTextFits("Outdated")
          fixture.assertTextContrast("Outdated", blendOver(Warning.copy(alpha = 0.16f), Panel))
        }
  }

  @Test
  fun revisionMismatchedRunAgreesAcrossAnalysisSummaryAndHeaderWithoutControls() {
    val project = resultProjectFixture().copy(projectRevision = "next")
    val original = analysisRunFixture()
    val run =
        original.copy(
            status = "running",
            files =
                listOf(
                    AnalysisRunFile(
                        "main.go",
                        "base",
                        "Go",
                        listOf(AnalysisStageProgress("semantic", "running", 1, false)))))
    val analysis = ProjectAnalysisRunState(run = run)
    val app = DesktopState(projectState = ProjectWorkspaceState(project), analysisRun = analysis)
    val header = toolbarAnalysisStatus(app)!!
    assertEquals("Analysis · Stale", header.label)
    assertFalse(header.running)
    assertTrue(header.attention)
    assertTrue(header.detail.contains("Run belongs to an older project revision"))
    var actions = 0
    val callbacks =
        AnalysisWorkspaceActions(
            { _, _ -> actions++ },
            { actions++ },
            { actions++ },
            { actions++ },
            { actions++ },
            refreshStatus = { error("Unexpected status refresh") })
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisWorkspacePane(AnalysisWorkspacePaneState(project, analysis), callbacks)
        }
        .use { fixture ->
          fixture.render("f15-outdated-analysis")
          assertTrue(fixture.hasText("Analysis out of date"))
          assertTrue(
              fixture.hasText(
                  "Outdated · run belongs to an older project revision; not current evidence."))
          assertFalse(fixture.hasText("Current: main.go"))
          assertEquals(0, fixture.tagCount("analysis-run-controls"))
          assertFalse(fixture.hasText("Pause"))
          assertFalse(fixture.hasText("Cancel"))
        }
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectSummaryPane(
              visualFixtureOverview.copy(
                  projectId = project.projectId, projectRevision = project.projectRevision),
              project,
              {},
              run = run,
              analysisState = analysis,
              analysisActions = callbacks)
        }
        .use { fixture ->
          fixture.render("f15-outdated-summary")
          assertEquals(1, fixture.tagCount("summary-analysis-run-strip"))
          assertTrue(fixture.hasText("Stale"))
          assertTrue(fixture.hasText("File progress unavailable"))
          assertTrue(
              fixture.hasText(
                  "Outdated · run belongs to an older project revision; not current evidence."))
          assertFalse(fixture.hasText("Current: main.go"))
          assertEquals(0, fixture.tagCount("summary-start-analysis"))
          assertFalse(fixture.hasText("Pause"))
          assertFalse(fixture.hasText("Cancel"))
        }
    ComposeVisualFixture(800, 650, 1.5f) {
          MainToolbar(
              ToolbarState(
                  project,
                  false,
                  "",
                  ConnectionState(connected = true),
                  GitStatus(available = true, branch = "main"),
                  header),
              ToolbarActions({}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render("f15-outdated-header")
          assertTrue(fixture.hasText("Analysis · Stale"))
          assertFalse(fixture.hasText("Analysis · Running"))
        }
    assertEquals(0, actions)
  }

  @Test
  fun summaryMetricFlowKeepsEveryCoverageBoxAndPartialStateVisible() {
    val (base, section) = summaryBugFixture(listOf("high", "medium", "low"))
    val run =
        base.copy(
            sections =
                base.sections.map {
                  if (it.category == "performance") it.copy(status = "partial") else it
                })
    val overview =
        visualFixtureOverview.copy(
            projectId = "project",
            projectRevision = "revision",
            analysisCoverage =
                AnalysisCoverage(
                    total = 5, fresh = 1, stale = 1, missing = 1, running = 1, failed = 1))
    listOf(1440 to 1f, 1000 to 1.5f, 999 to 1.5f, 800 to 1.5f).forEach { (width, scale) ->
      ComposeVisualFixture(width, 650, scale) {
            ProjectSummaryPane(
                overview, resultProjectFixture(), {}, run = run, sections = bugSections(section))
          }
          .use { fixture ->
            fixture.render("summary-all-metrics-$width-$scale")
            fixture.assertSummaryStatusPlacement("Paused")
            listOf(
                    "1 up to date",
                    "1 outdated",
                    "1 not analyzed",
                    "1 running",
                    "1 failed",
                    "High: 1 · Medium: 1 · Low: 1")
                .forEach {
                  fixture.revealText(it)
                  fixture.assertTextFits(it, maxLines = 3)
                }
            listOf("Bugs", "Performance", "Security").forEach {
              assertTrue(fixture.hasDescription("View $it results"))
              fixture.revealText(it)
              fixture.assertTextFits(it)
            }
            fixture.revealText("Partial")
            fixture.assertTextFits("Partial")
            fixture.revealSummaryStatus("Paused")
            fixture.assertTextFits("Paused")
          }
    }
  }

  @Test
  fun summaryModulesShowNamesPathsAndDescriptionsInFlatRows() {
    // Below the former 500dp / font-scale threshold, descriptions still stay beside identity.
    ComposeVisualFixture(720, 650, 1.5f) {
          SummaryModules(
              listOf(
                  "internal/project (Project indexing): Builds the project context.",
                  "internal/app (Workflow orchestration): Coordinates guarded changes."))
        }
        .use { fixture ->
          fixture.render("summary-modules-fixed-arrangement-720-150")
          listOf("Project indexing", "internal/project", "Workflow orchestration", "internal/app")
              .forEach(fixture::assertTextFits)
          listOf(
                  Triple("Project indexing", "internal/project", "Builds the project context."),
                  Triple("Workflow orchestration", "internal/app", "Coordinates guarded changes."))
              .forEach { (name, path, description) ->
                assertTrue(fixture.hasText(description), "$description must remain in the row")
                fixture.assertTextAbove(name, path)
                val identity = fixture.firstVisibleTextBounds(name)
                val modulePath = fixture.firstVisibleTextBounds(path)
                val detail = fixture.firstVisibleTextBounds(description)
                assertTrue(
                    detail.left >= maxOf(identity.right, modulePath.right),
                    "$description must trail the module identity")
                assertTrue(
                    detail.top < modulePath.bottom && detail.bottom > identity.top,
                    "$description must share the module row")
              }
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
            fixture.awaitDescription("Expand Architecture diagram", "Preview")
            assertFalse(fixture.isDisabled("Expand diagram"))
            fixture.awaitDescription("Architecture diagram preview\n$source")
            assertTrue(fixture.taggedBounds("diagram-preview").height <= 180f * scale)
            fixture.render("summary-mermaid-preview-$width-$scale")
            assertFalse(fixture.hasText("Mermaid source"))
            assertTrue(fixture.requestFocus("Expand diagram"))
            fixture.pressKey(Key.Enter)
            fixture.awaitDescription("Architecture diagram\n$source")
            fixture.awaitDescription("Close Architecture diagram")
            fixture.render("summary-mermaid-$width-$scale")
            assertFalse(fixture.hasText("Rendering diagram…"))
            fixture.clickText("Mermaid source")
            fixture.render()
            assertTrue(fixture.hasText(source))
            assertFalse(fixture.hasEditableText())
            fixture.clickDescription("Close Architecture diagram")
            fixture.render()
            assertTrue(fixture.hasDescription("Architecture diagram preview\n$source"))
            assertFalse(fixture.hasDescription("Architecture diagram\n$source"))
            assertFalse(fixture.hasText(source))
            assertEquals("Preview", fixture.stateDescription("Expand diagram"))
          }
    }
  }

  @Test
  fun diagramPresentationCapturesPreviewAndViewerAcrossViewportTextAndDensity() {
    val source = "flowchart LR\n Desktop --> API\n API --> App\n App --> Project\n App --> Storage"
    val sizes = listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)
    for ((width, height) in sizes) for (scale in listOf(1f, 1.25f, 1.5f)) {
      // The density variant keeps the same logical 800 × 650 viewport.
      val densities =
          if (width == 800 && height == 650 && scale == 1.5f) listOf(1f, 2f) else listOf(1f)
      for (density in densities) {
        val pixelWidth = (width * density).toInt()
        val pixelHeight = (height * density).toInt()
        val label = "f11-$width-$height-$scale-${density}x"
        ComposeVisualFixture(pixelWidth, pixelHeight, scale, density) {
              MermaidDiagram(source, "Architecture", title = "Architecture")
            }
            .use { fixture ->
              fixture.awaitDescription("Architecture diagram preview\n$source")
              fixture.render("$label-preview")
              assertTrue(fixture.taggedBounds("diagram-preview").height <= 180f * density)
              assertFalse(fixture.hasText("Mermaid source"))
              fixture.clickDescription("Expand Architecture diagram")
              fixture.awaitDescription("Architecture diagram\n$source")
              repeat(8) { fixture.render() }
              fixture.render("$label-expanded")
              if (width == 800 && height == 650 && scale == 1.5f && density == 1f) {
                assertTrue(fixture.requestDescriptionFocus("Zoom in Architecture"))
                fixture.render("$label-focused")
                assertTrue(fixture.isDescriptionFocused("Zoom in Architecture"))
                fixture.awaitVisibleDescription("Close Architecture diagram")
              }
              val canvas = fixture.taggedBounds("diagram-canvas")
              assertTrue(canvas.width <= pixelWidth.toFloat())
              assertTrue(canvas.height <= 300f * density)
              fixture.awaitVisibleDescription("Close Architecture diagram")
              fixture.clickText("Mermaid source")
              fixture.render("$label-source")
              assertEquals(1, fixture.taggedTextCount("diagram-source-scroll", source))
              fixture.awaitVisibleDescription("Close Architecture diagram")
              fixture.revealText("Copy source")
              fixture.awaitVisibleDescription("Copy saved content for Architecture")
            }
      }
    }
  }

  @Test
  fun diagramPresentationCapturesLoadingUnavailableFailureLegacyAndZoomBounds() {
    val source = "flowchart LR\n" + (1..18).joinToString("\n") { " N$it --> N${it + 1}" }
    val pending = kotlinx.coroutines.CompletableDeferred<MermaidImage>()
    ComposeVisualFixture(800, 650, 1.5f) {
          MermaidDiagram(
              source, "Architecture", title = "Architecture", render = { pending.await() })
        }
        .use { fixture ->
          fixture.render("f11-loading-preview-800-650-150")
          assertTrue(fixture.hasText("Rendering diagram…"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitVisibleDescription("Close Architecture diagram")
          repeat(8) { fixture.render() }
          assertTrue(fixture.hasText("Rendering diagram…"))
          fixture.awaitVisibleDescription("Zoom out Architecture")
          fixture.awaitVisibleDescription("Mermaid source for Architecture")
          fixture.awaitVisibleDescription("Close Architecture diagram")
          fixture.render("f11-loading-expanded-800-650-150")
          pending.complete(kotlinx.coroutines.runBlocking { renderSummaryDiagram(source) })
          fixture.awaitDescription("Architecture diagram\n$source")
          fixture.render("f11-wide-expanded-800-650-150")
          assertTrue(fixture.scrollMaximum("diagram-horizontal-scroll", horizontal = true) > 0f)
          fixture.scrollTagged("diagram-horizontal-scroll", horizontal = true, pixels = 100_000f)
          fixture.render("f11-wide-end-800-650-150")
          assertEquals(
              fixture.scrollMaximum("diagram-horizontal-scroll", horizontal = true),
              fixture.scrollPosition("diagram-horizontal-scroll", horizontal = true))
          repeat(4) { fixture.clickDescription("Zoom out Architecture") }
          fixture.render("f11-zoom-minimum-800-650-150")
          assertTrue(fixture.isDescriptionDisabled("Zoom out Architecture"))
          assertEquals(
              "Minimum zoom 75%", fixture.descriptionStateDescription("Zoom out Architecture"))
          repeat(5) { fixture.clickDescription("Zoom in Architecture") }
          fixture.render("f11-zoom-maximum-800-650-150")
          assertTrue(fixture.isDescriptionDisabled("Zoom in Architecture"))
          assertEquals(
              "Maximum zoom 200%", fixture.descriptionStateDescription("Zoom in Architecture"))
        }
    val tallSource = "flowchart TD\n" + (1..18).joinToString("\n") { " N$it --> N${it + 1}" }
    ComposeVisualFixture(1600, 1300, 1.5f, 2f) {
          MermaidDiagram(tallSource, "Flow 1", title = "Flow 1")
        }
        .use { fixture ->
          fixture.awaitDescription("Flow 1 diagram preview\n$tallSource")
          fixture.render("f11-tall-preview-800-650-150-2x")
          fixture.clickDescription("Expand Flow 1 diagram")
          fixture.awaitDescription("Flow 1 diagram\n$tallSource")
          fixture.render("f11-tall-expanded-800-650-150-2x")
          assertTrue(fixture.scrollMaximum("diagram-vertical-scroll", horizontal = false) > 0f)
          fixture.scrollTagged("diagram-vertical-scroll", horizontal = false, pixels = 100_000f)
          fixture.render("f11-tall-end-800-650-150-2x")
          assertEquals(
              fixture.scrollMaximum("diagram-vertical-scroll", horizontal = false),
              fixture.scrollPosition("diagram-vertical-scroll", horizontal = false))
        }
    ComposeVisualFixture(800, 650, 1.5f) {
          MermaidDiagram("API → Service", "Flow 1", title = "Flow 1")
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
          fixture.render("f11-legacy-preview-800-650-150")
          fixture.clickDescription("Expand Flow 1 diagram")
          fixture.awaitDescription("Close Flow 1 diagram")
          fixture.clickText("Mermaid source")
          fixture.render("f11-legacy-source-800-650-150")
          assertTrue(fixture.hasText("Original saved arrow chain"))
          assertTrue(fixture.hasText("Generated Mermaid for rendering (not saved)"))
        }
    ComposeVisualFixture(800, 650, 1.5f) {
          MermaidDiagram("A prose-only saved report.", "Flow 2", title = "Flow 2")
        }
        .use { fixture ->
          fixture.render("f11-prose-only-800-650-150")
          assertTrue(fixture.hasText("No diagram in saved content"))
          assertTrue(fixture.isDisabled("Expand diagram"))
        }
    val invalid = "flowchart TD\n A[Node]\n click A \"https://example.com\""
    ComposeVisualFixture(800, 650, 1.5f) {
          MermaidDiagram(invalid, "Architecture", title = "Architecture")
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Architecture diagram", "Diagram failed")
          fixture.render("f11-failed-preview-800-650-150")
          assertTrue(fixture.hasText("Original saved content"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Close Architecture diagram")
          fixture.render("f11-failed-expanded-800-650-150")
          assertTrue(
              fixture.hasText(
                  "Diagram unavailable: Diagram contains unsupported styling, links or markup"))
          fixture.clickText("Mermaid source")
          fixture.render("f11-failed-source-800-650-150")
          assertEquals(2, fixture.taggedTextCount("diagram-source-scroll", invalid))
          fixture.awaitVisibleDescription("Close Architecture diagram")
        }
  }

  @Test
  @OptIn(ExperimentalComposeUiApi::class)
  fun diagramSourceIsLiteralScrollableAndCopiesTheEntireSavedReport() {
    val saved =
        "Before **literal** <img src='remote'> [link](https://example.org)\r\n" +
            "```mermaid\nflowchart LR\n A --> B\n```\n" +
            (1..300).joinToString("\n") { "line $it -> `not a command`" } +
            "\n```mermaid\nsequenceDiagram\n A->>B: Extra\n```\nAfter\n" +
            "very-long-".repeat(180)
    var renders = 0
    val clipboard =
        object : Clipboard {
          override val nativeClipboard = java.awt.datatransfer.Clipboard("diagram-source-test")
          var fail = false

          override suspend fun getClipEntry(): ClipEntry? =
              nativeClipboard.getContents(null)?.let(::ClipEntry)

          override suspend fun setClipEntry(clipEntry: ClipEntry?) {
            if (fail) throw IllegalStateException("Clipboard unavailable")
            nativeClipboard.setContents(clipEntry?.asAwtTransferable, null)
          }
        }
    ComposeVisualFixture(800, 650, 1.5f) {
          CompositionLocalProvider(LocalClipboard provides clipboard) {
            MermaidDiagram(
                saved,
                "Architecture",
                render = {
                  renders++
                  MermaidImage(ImageBitmap(16, 16), 16f, 16f)
                })
          }
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Close Architecture diagram")
          fixture.clickText("Mermaid source")
          fixture.render("f11-long-source-800-650-150")
          assertEquals(1, fixture.taggedTextCount("diagram-source-scroll", saved))
          assertFalse(fixture.hasEditableText(withinTag = "diagram-source-scroll"))
          assertTrue(fixture.scrollMaximum("diagram-source-scroll", horizontal = false) > 0f)
          assertTrue(
              fixture.scrollMaximum("diagram-source-horizontal-scroll", horizontal = true) > 0f)
          assertFalse(fixture.hasText("Saved content copied"))
          clipboard.fail = true
          fixture.clickDescription("Copy saved content for Architecture")
          fixture.render()
          assertTrue(fixture.hasText("Could not copy saved content: Clipboard unavailable"))
          assertEquals(1, fixture.taggedTextCount("diagram-source-scroll", saved))
          clipboard.fail = false
          fixture.clickDescription("Copy saved content for Architecture")
          fixture.render()
          assertTrue(fixture.hasText("Saved content copied"))
          assertFalse(fixture.hasText("Could not copy saved content: Clipboard unavailable"))
          assertEquals(saved, clipboard.nativeClipboard.getData(DataFlavor.stringFlavor))
          assertEquals(1, renders)
        }
  }

  @Test
  fun largeSavedDiagramsRemainBoundedBeforeExpansion() {
    for ((width, height) in listOf(1200 to 80, 80 to 1200)) {
      val source = "flowchart LR\n A --> B"
      var renders = 0
      val image = MermaidImage(ImageBitmap(width, height), width.toFloat(), height.toFloat())
      ComposeVisualFixture(800, 650, 1.5f) {
            MermaidDiagram(
                source,
                "Architecture",
                render = {
                  renders++
                  image
                })
          }
          .use { fixture ->
            fixture.awaitDescription("Architecture diagram preview\n$source")
            fixture.render("summary-preview-$width-$height")
            val bounds = fixture.taggedBounds("diagram-preview")
            assertTrue(bounds.height <= 270f)
            assertTrue(bounds.width <= 800f)
            assertEquals(1, renders)
            fixture.clickDescription("Expand Architecture diagram")
            fixture.awaitDescription("Architecture diagram\n$source")
            assertEquals(1, renders)
          }
    }
  }

  @Test
  fun expandedDiagramNavigatesBothAxesWithinABoundedCanvas() {
    val source = "flowchart LR\n A --> B"
    for ((width, height) in listOf(1200 to 900, 900 to 1200)) {
      for (density in listOf(1f, 2f)) {
        var renders = 0
        val image = MermaidImage(ImageBitmap(width, height), width.toFloat(), height.toFloat())
        ComposeVisualFixture((800 * density).toInt(), (650 * density).toInt(), 1.5f, density) {
              MermaidDiagram(
                  source,
                  "Architecture",
                  render = {
                    renders++
                    image
                  })
            }
            .use { fixture ->
              fixture.awaitDescription("Architecture diagram preview\n$source")
              fixture.clickDescription("Expand Architecture diagram")
              fixture.awaitDescription("Architecture diagram\n$source")
              val canvas = fixture.taggedBounds("diagram-canvas")
              assertTrue(canvas.width <= 640f * density, "Canvas width at ${density}x")
              assertTrue(canvas.height <= 300f * density, "Canvas height at ${density}x")
              assertTrue(fixture.scrollMaximum("diagram-horizontal-scroll", horizontal = true) > 0f)
              assertTrue(fixture.scrollMaximum("diagram-vertical-scroll", horizontal = false) > 0f)
              fixture.scrollTagged(
                  "diagram-horizontal-scroll", horizontal = true, pixels = 100_000f)
              fixture.scrollTagged("diagram-vertical-scroll", horizontal = false, pixels = 100_000f)
              assertEquals(
                  fixture.scrollMaximum("diagram-horizontal-scroll", horizontal = true),
                  fixture.scrollPosition("diagram-horizontal-scroll", horizontal = true))
              assertEquals(
                  fixture.scrollMaximum("diagram-vertical-scroll", horizontal = false),
                  fixture.scrollPosition("diagram-vertical-scroll", horizontal = false))
              assertEquals(1, renders)
            }
      }
    }
  }

  @Test
  fun validDiagramDisclosureAcceptsTheFirstClickWhileRenderingStarts() {
    val source = "flowchart TD\n A[Client] --> B[Server]"
    ComposeVisualFixture(800, 650, 1.5f) {
          MermaidDiagram(source, "Architecture", title = "Architecture")
        }
        .use { fixture ->
          fixture.render("summary-mermaid-first-frame")
          assertTrue(
              fixture.tryClick("Expand diagram"),
              "A valid diagram must not ignore the first disclosure click while rendering")
          fixture.awaitDescription("Architecture diagram\n$source")
          fixture.awaitDescription("Close Architecture diagram")
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
            assertEquals(1, fixture.tagCount("diagram-preview"))
            assertTrue(fixture.hasText("No diagram in saved content"))
            assertTrue(fixture.isDisabled("Expand diagram"))
            assertEquals("Diagram unavailable", fixture.stateDescription("Expand diagram"))
            assertFalse(fixture.hasText("Run Analysis again to generate this diagram."))
            assertFalse(fixture.tryClick("Expand diagram"))

            value = "flowchart TD\n A[Client] --> B[Server]"
            fixture.awaitDescription("Expand $label diagram", "Preview")
            fixture.clickText("Expand diagram")
            fixture.awaitDescription("$label diagram\n$value")

            value = "flowchart TD\n A[Replacement]"
            fixture.awaitDescription("Expand $label diagram", "Preview")
            assertFalse(fixture.hasDescription("$label diagram\n$value"))
            assertFalse(fixture.isDisabled("Expand diagram"))

            value = "flowchart TD\n A[Node]\n click A \"https://example.com\""
            fixture.awaitDescription("Expand $label diagram", "Diagram failed")
            fixture.render()
            assertFalse(fixture.isDisabled("Expand diagram"))
            assertTrue(fixture.hasText(value))
            assertTrue(
                fixture.hasText(
                    "Diagram unavailable: Diagram contains unsupported styling, links or markup"))
            assertTrue(fixture.tryClick("Expand diagram"))

            value = "sequenceDiagram\n Client->>API: Retry\n API-->>Client: Ready"
            fixture.render()
            assertTrue(fixture.tryClick("Expand diagram"))
            fixture.awaitDescription("$label diagram\n$value")
            fixture.awaitDescription("Close $label diagram")
          }
    }
  }

  @Test
  fun summaryIssuesKeepPriorityBreakdownAndNeutralZeroCounts() {
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
              ProjectSummaryPane(
                  visualFixtureOverview,
                  resultProjectFixture(),
                  {},
                  run = run,
                  sections = bugSections(section))
            }
            .use { fixture ->
              fixture.render()
              fixture.render("summary-issue-colors-$width-$height-$scale")
              fixture.revealText("High: 3 · Medium: 0 · Low: 1")
              fixture.assertTextFits("High: 3 · Medium: 0 · Low: 1", maxLines = 3)
              fixture.assertBugPrioritiesInsideCard("High: 3 · Medium: 0 · Low: 1")
              assertFalse(fixture.hasText("Score: 10 points"))
              assertFalse(fixture.hasText("Score unavailable"))
              assertFalse(fixture.hasText("Bug priorities"))
              listOf("Bugs" to Error, "Performance" to Information, "Security" to FaintText)
                  .forEach { (label, tint) ->
                    fixture.revealText(label)
                    fixture.assertColorVisible(tint)
                    assertTrue(fixture.hasDescription("View $label results"))
                    fixture.assertTextFits(label)
                  }
            }
      }
    }
  }

  @Test
  fun summaryDashboardAdaptsToNarrowShortAndLargeTextViews() {
    listOf(1440 to 900, 1024 to 768, 1000 to 650, 999 to 650, 800 to 650, 1280 to 600).forEach {
        (width, height) ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        ComposeVisualFixture(width, height, scale) {
              ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
            }
            .use { fixture ->
              fixture.render("summary-dashboard-$width-$height-$scale")
              listOf("Analysis coverage", "16 up to date", "4 outdated", "3 not analyzed")
                  .forEach { label ->
                    fixture.revealText(label)
                    fixture.assertTextFits(label)
                  }
              fixture.assertSummaryStatusPlacement("Outdated")
              if (width < 1440) fixture.assertFixedSummaryCoverageLayout()
              listOf("Bugs", "Performance", "Security").forEach { label ->
                fixture.revealText(label)
                fixture.assertTextFits(label)
              }
              assertTrue(fixture.hasScrollableContent())
            }
      }
    }
    ComposeVisualFixture(800, 300) { ProjectSummaryPane(null, null, {}) }
        .use { fixture ->
          fixture.render("summary-dashboard-empty-800")
          assertTrue(fixture.hasText("No project selected"))
          assertFalse(fixture.hasText("Analysis coverage"))
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
                      prepareFinding = { workflowActions++ },
                      triageFinding = { _, _ -> workflowActions++ },
                      openSource = { workflowActions++ }),
                  {},
                  {}))
        }
        .use { fixture ->
          fixture.render("findings-tools-collapsed-480-1.3")
          assertTrue(fixture.hasText("Trust project-code execution & run checks"))
          assertTrue(fixture.stateDescription("Command and output") == "Collapsed")
          assertTrue(fixture.requestFocus("Command and output"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render("findings-tools-expanded-480-1.3")
          assertTrue(fixture.stateDescription("Command and output") == "Expanded")
          assertTrue(fixture.hasDescription("Filter results"))
          assertTrue(fixture.hasEditableText(withinDescription = "Filter results"))
          assertTrue(fixture.hasScrollableContent())
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.stateDescription("Command and output") == "Collapsed")
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
          fixture.assertTextAboveDescription("Files", "Close Files drawer")
          fixture.clickDescription("Close Files drawer")
          fixture.clickText("Terminal")
          kotlin.test.assertEquals(1, closes)
          kotlin.test.assertEquals(1, opens)
        }
  }

  @Test
  fun problemRowsRevealDetailsBeforeAnyWorkflowAction() {
    var mutations = 0
    ComposeVisualFixture(900, 500) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(visualFixtureFindings, null, false),
              BugsWorkspaceActions(
                  FindingActions({ mutations++ }, { _, _ -> mutations++ }, {}), {}, {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Validate the user identifier")
          fixture.render()
          kotlin.test.assertEquals(0, mutations)
          assertTrue(fixture.isDisabled("Open source"))
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertTrue(fixture.hasText("Model suggestion"))
          fixture.clickText("Evidence and fix criteria")
          fixture.render()
          assertTrue(
              fixture.hasText("Model proposal; validate against source before preparing a change."))
          kotlin.test.assertEquals(0, mutations)
        }
  }
}

internal const val verifiedScanScopeCopy =
    "Whole-project checks, independent of the Analysis file selection: parser inspection of indexed Go source; go vet ./...; go test ./..."
internal const val verifiedScanTrustCopy =
    "Checks run in a temporary copied workspace; the scan does not edit original source. Tests and package initialization can execute project code. A copy is not a security sandbox."
internal const val verifiedScanEvidenceCopy =
    "Local tool evidence is scoped to these checks, not a general safety assurance. Model suggestions are separate results below."

internal fun verifiedScanLayoutCases(): List<Pair<String, BugsWorkspacePaneState>> {
  val original = resultPageFixture("bugs")
  val finding =
      original.semantic
          .first()
          .copy(
              title = "Review unchecked input",
              confidence = "suggested",
              source = "file_analysis",
              severity = "high",
              message = "Model suggestion; inspect source before preparing a fix.")
  val page =
      original.copy(
          section =
              original.section.copy(results = original.results!!.copy(semantic = listOf(finding))))
  val project = requireNotNull(page.project).copy(type = "go")
  val phase =
      GoScanPhase("go vet", "failed", listOf("go", "vet", "./..."), "main.go:7: vet diagnostic", 1)
  val report =
      GoScanReport(project.projectId, project.projectRevision, "completed", phases = listOf(phase))
  val base =
      BugsWorkspacePaneState(
          page.semantic,
          null,
          false,
          page.copy(project = project),
          project = project,
          scanState = VerifiedScanState(read = VerifiedScanRead.Absent))
  val loaded =
      base.copy(scan = report, scanState = VerifiedScanState(read = VerifiedScanRead.Loaded))
  return listOf(
      "supported" to base,
      "unsupported" to loaded.copy(project = project.copy(type = "python")),
      "unknown-type" to loaded.copy(project = project.copy(type = "unknown")),
      "pending" to
          loaded.copy(
              scanState = loaded.scanState.copy(operation = VerifiedScanOperation.Starting)),
      "running" to loaded.copy(scan = report.copy(status = "running")),
      "cancel-pending" to
          loaded.copy(
              scan = report.copy(status = "running"),
              scanState =
                  loaded.scanState.copy(operation = VerifiedScanOperation.CancellationRequested)),
      "canceled" to
          loaded.copy(
              scan =
                  report.copy(
                      status = "canceled",
                      phases = listOf(phase.copy(state = "canceled", exitCode = 143)))),
      "failed-phase" to loaded,
      "command-failure" to loaded.copy(scan = report.copy(status = "failed")),
      "unavailable" to
          loaded.copy(
              scanState =
                  loaded.scanState.copy(
                      read =
                          VerifiedScanRead.PollUnavailable(
                              "Live status read failed; retained evidence is not current."))),
      "initial-unavailable" to
          base.copy(
              scanState =
                  VerifiedScanState(
                      read =
                          VerifiedScanRead.Unavailable(
                              "Initial status read failed; no report is known."))),
      "start-unconfirmed" to
          loaded.copy(
              scanState =
                  loaded.scanState.copy(
                      operation =
                          VerifiedScanOperation.StartUncertain(
                              "Start timed out; acceptance is unknown. Refresh scan status."))),
      "cancel-unconfirmed" to
          loaded.copy(
              scanState =
                  loaded.scanState.copy(
                      operation =
                          VerifiedScanOperation.CancellationUnconfirmed(
                              "Cancel timed out; cancellation is not confirmed. Refresh scan status."))),
      "long-output" to
          loaded.copy(
              scan =
                  report.copy(
                      phases =
                          listOf(
                              phase.copy(
                                  output =
                                      "internal/" +
                                          "deeply/nested/日本語/".repeat(12) +
                                          "handler.go:83: diagnostic\n" +
                                          "recorded evidence\n".repeat(400) +
                                          "FINAL AVAILABLE LINE\n[output truncated]")))))
}

/** This test-only adapter is tied to the Compose version pinned in build.gradle.kts. */
@OptIn(ExperimentalComposeUiApi::class, InternalComposeUiApi::class)
internal class ComposeVisualFixture(
    private var width: Int,
    private var height: Int,
    private val fontScale: Float = 1f,
    densityScale: Float = 1f,
    private val frameDurationNanos: Long = 80_000_000,
    content: @Composable () -> Unit,
) : AutoCloseable {
  var failClipboardWrites = false
  private val clipboard =
      object : Clipboard {
        override val nativeClipboard = java.awt.datatransfer.Clipboard("visual-test")

        override suspend fun getClipEntry(): ClipEntry? =
            nativeClipboard.getContents(null)?.let(::ClipEntry)

        override suspend fun setClipEntry(clipEntry: ClipEntry?) {
          if (failClipboardWrites) throw IllegalStateException("Clipboard unavailable")
          nativeClipboard.setContents(clipEntry?.asAwtTransferable, null)
        }
      }
  private val owners = mutableListOf<SemanticsOwner>()
  private val platform =
      object : PlatformContext by PlatformContext.Empty() {
        override val windowInfo =
            object : WindowInfo {
              override val isWindowFocused = true
              override val containerSize = IntSize(width, height)
              override val containerDpSize =
                  DpSize((width / densityScale).dp, (height / densityScale).dp)
            }
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
  private var surface = Surface.makeRasterN32Premul(width, height)
  private var frameTime = 0L

  init {
    scene.setContent {
      CompositionLocalProvider(LocalClipboard provides clipboard) { MiniOrcaTheme { content() } }
    }
  }

  fun resize(newWidth: Int, newHeight: Int) {
    width = newWidth
    height = newHeight
    scene.size = IntSize(width, height)
    surface.close()
    surface = Surface.makeRasterN32Premul(width, height)
  }

  fun render(name: String? = null) {
    repeat(3) {
      surface.canvas.clear(AppBackground.toArgb())
      scene.render(surface.canvas.asComposeCanvas(), frameTime)
      frameTime += frameDurationNanos
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

  fun awaitVisibleDescription(label: String) {
    fun visible(): Boolean =
        nodes().any { node ->
          node.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
              node.config.getOrNull(SemanticsActions.OnClick) != null &&
              // boundsInRoot is clipped: a sliver of the button can be visible while its label
              // is still outside the viewport. Wait for the complete layout to be revealed.
              node.positionInRoot.let { position ->
                node.size.width > 0 &&
                    node.size.height > 0 &&
                    position.x >= 0 &&
                    position.y >= 0 &&
                    position.x + node.size.width <= width &&
                    position.y + node.size.height <= height
              }
        }
    val deadline = System.nanoTime() + 20_000_000_000L
    render()
    while (!visible() && System.nanoTime() < deadline) {
      Thread.sleep(20)
      render()
    }
    assertTrue(visible(), "Timed out waiting for $label to become fully visible")
  }

  fun hasText(label: String): Boolean =
      textNodes(label).isNotEmpty() ||
          nodes().any { it.config.getOrNull(SemanticsProperties.EditableText)?.text == label }

  fun textCount(label: String): Int = textNodes(label).size

  fun firstVisibleTextBounds(label: String): Rect =
      textNodes(label)
          .filter { it.boundsInRoot.width > 0f && it.boundsInRoot.height > 0f }
          .minBy { it.boundsInRoot.top }
          .boundsInRoot

  fun taggedTextCount(tag: String, label: String): Int =
      textNodes(label).count { node ->
        generateSequence(node) { it.parent }
            .any { it.config.getOrNull(SemanticsProperties.TestTag) == tag }
      }

  fun hasEditableText(withinDescription: String? = null, withinTag: String? = null): Boolean =
      nodes().any { node ->
        node.config.getOrNull(SemanticsActions.SetText) != null &&
            (withinDescription == null ||
                generateSequence(node) { it.parent }
                    .any {
                      it.config
                          .getOrNull(SemanticsProperties.ContentDescription)
                          ?.contains(withinDescription) == true
                    }) &&
            (withinTag == null ||
                generateSequence(node) { it.parent }
                    .any { it.config.getOrNull(SemanticsProperties.TestTag) == withinTag })
      }

  fun hasDescription(label: String): Boolean =
      nodes().any {
        it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
      }

  fun semanticHeadingTexts(): List<String> =
      nodes()
          .filter { it.config.getOrNull(SemanticsProperties.Heading) != null }
          .flatMap { node ->
            node.config.getOrNull(SemanticsProperties.Text)?.map { it.text }.orEmpty()
          }

  fun isDescriptionSelected(label: String): Boolean =
      nodes()
          .asSequence()
          .filter {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
          }
          .flatMap { node -> generateSequence(node) { it.parent } }
          .any { it.config.getOrNull(SemanticsProperties.Selected) == true }

  fun isDescriptionDisabled(label: String): Boolean =
      nodes()
          .asSequence()
          .filter {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
          }
          .flatMap { node -> generateSequence(node) { it.parent } }
          .any { it.config.getOrNull(SemanticsProperties.Disabled) != null }

  fun descriptionStateDescription(label: String): String? =
      nodes()
          .asSequence()
          .filter {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
          }
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { it.config.getOrNull(SemanticsProperties.StateDescription) }
          .firstOrNull()

  fun descriptionToggleableState(label: String): ToggleableState? =
      nodes()
          .asSequence()
          .filter {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
          }
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { it.config.getOrNull(SemanticsProperties.ToggleableState) }
          .firstOrNull()

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

  private fun describedEditor(label: String): SemanticsNode =
      nodes().single { node ->
        node.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
            node.config.getOrNull(SemanticsActions.SetText) != null
      }

  fun descriptionError(label: String): String? =
      nodes()
          .firstOrNull {
            it.config.getOrNull(SemanticsProperties.ContentDescription) == listOf(label)
          }
          ?.config
          ?.getOrNull(SemanticsProperties.Error)

  fun focusDescribedEditor(label: String) {
    val position = describedEditor(label).boundsInRoot.center
    scene.sendPointerEvent(PointerEventType.Press, position, button = PointerButton.Primary)
    scene.sendPointerEvent(PointerEventType.Release, position, button = PointerButton.Primary)
    render()
  }

  fun setTextForDescription(label: String, value: String) {
    assertTrue(
        requireNotNull(describedEditor(label).config.getOrNull(SemanticsActions.SetText)?.action)
            .invoke(AnnotatedString(value)))
  }

  fun selectEditorText(label: String, start: Int, end: Int) {
    assertTrue(
        requireNotNull(
                describedEditor(label).config.getOrNull(SemanticsActions.SetSelection)?.action)
            .invoke(start, end, false))
  }

  fun clipboardText(): String = clipboard.nativeClipboard.getData(DataFlavor.stringFlavor) as String

  fun setClipboardText(value: String) {
    clipboard.nativeClipboard.setContents(java.awt.datatransfer.StringSelection(value), null)
  }

  fun hasTextMutationSemantics(withinTag: String): Boolean =
      nodes().any { node ->
        generateSequence(node) { it.parent }
            .any { it.config.getOrNull(SemanticsProperties.TestTag) == withinTag } &&
            (node.config.getOrNull(SemanticsProperties.EditableText) != null ||
                node.config.getOrNull(SemanticsActions.SetText) != null ||
                node.config.getOrNull(SemanticsActions.InsertTextAtCursor) != null)
      }

  fun editorTextWidth(label: String): Int {
    val layouts = mutableListOf<TextLayoutResult>()
    assertTrue(
        requireNotNull(
                describedEditor(label)
                    .config
                    .getOrNull(SemanticsActions.GetTextLayoutResult)
                    ?.action)
            .invoke(layouts))
    return layouts.single().size.width
  }

  fun editorSelectionEnd(label: String): Int =
      requireNotNull(
              describedEditor(label).config.getOrNull(SemanticsProperties.TextSelectionRange))
          .end

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

  fun assertCategoryBoxesFit() {
    val bounds =
        AnalysisResultType.entries.map { visibleActionBounds("View ${it.workspace.name} results") }
    bounds.forEach {
      assertEquals(bounds.first().width, it.width, 1f, "Category widths must match")
      if (bounds[0].top == bounds[1].top)
          assertEquals(
              bounds.first().height, it.height, 1f, "Side-by-side category heights must match")
    }
    bounds.zipWithNext().forEach { (first, next) ->
      assertTrue(
          first.right <= next.left || first.bottom <= next.top, "Categories must not overlap")
    }
  }

  private fun visibleActionBounds(label: String): Rect {
    val node =
        nodes().single {
          it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
              it.config.getOrNull(SemanticsActions.OnClick) != null
        }
    val bounds = node.boundsInRoot
    assertTrue(
        bounds.width > 0 &&
            bounds.height > 0 &&
            bounds.left >= 0 &&
            bounds.top >= 0 &&
            bounds.right <= width &&
            bounds.bottom <= height,
        "$label must be fully visible: $bounds in ${width}x$height")
    return bounds
  }

  fun clickableDescriptionCount(label: String): Int =
      nodes().count {
        it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
            it.config.getOrNull(SemanticsActions.OnClick) != null
      }

  fun clickVisibleDescription(label: String) {
    val bounds = visibleActionBounds(label)
    // Click empty space near the trailing edge, away from the icon/count text.
    val position = Offset(bounds.right - 12f, bounds.center.y)
    scene.sendPointerEvent(PointerEventType.Press, position, button = PointerButton.Primary)
    scene.sendPointerEvent(PointerEventType.Release, position, button = PointerButton.Primary)
    render()
  }

  fun hoverVisibleDescription(label: String) {
    scene.sendPointerEvent(PointerEventType.Move, visibleActionBounds(label).center)
    render()
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

  fun awaitDescriptionFocus(label: String) {
    fun focused(): Boolean =
        nodes().any { node ->
          node.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
              (sequenceOf(node) + descendants(node).asSequence()).any {
                it.config.getOrNull(SemanticsProperties.Focused) == true
              }
        }
    val deadline = System.nanoTime() + 2_000_000_000L
    while (!focused() && System.nanoTime() < deadline) {
      Thread.sleep(10)
      render()
    }
    assertTrue(focused(), "$label must retain keyboard focus")
  }

  fun isDescriptionFocused(label: String): Boolean {
    val described =
        nodes().filter {
          it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
        }
    return described.any { node ->
      (generateSequence(node) { it.parent } + descendants(node).asSequence()).any {
        it.config.getOrNull(SemanticsProperties.Focused) == true
      } ||
          nodes()
              .filter { it.config.getOrNull(SemanticsProperties.Focused) == true }
              .any { it.boundsInRoot.overlaps(node.boundsInRoot) }
    }
  }

  fun isFocusedControl(label: String): Boolean = isDescriptionFocused(label) || isFocused(label)

  fun isTaggedNodeFocused(tag: String): Boolean =
      nodes()
          .single { it.config.getOrNull(SemanticsProperties.TestTag) == tag }
          .config
          .getOrNull(SemanticsProperties.Focused) == true

  fun stateDescription(label: String): String? =
      textNodes(label)
          .asSequence()
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { it.config.getOrNull(SemanticsProperties.StateDescription) }
          .firstOrNull()

  fun descriptionState(label: String): String? =
      nodes()
          .asSequence()
          .filter {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
          }
          .mapNotNull { it.config.getOrNull(SemanticsProperties.StateDescription) }
          .firstOrNull()

  fun requestFocus(label: String): Boolean =
      textNodes(label)
          .asSequence()
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { it.config.getOrNull(SemanticsActions.RequestFocus)?.action }
          .firstOrNull()
          ?.invoke() ?: false

  fun dragDescription(label: String, delta: Offset) {
    val start =
        nodes()
            .single {
              it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
            }
            .boundsInRoot
            .center
    scene.sendPointerEvent(PointerEventType.Press, start, button = PointerButton.Primary)
    render()
    scene.sendPointerEvent(PointerEventType.Move, start + delta / 2f)
    render()
    scene.sendPointerEvent(PointerEventType.Move, start + delta)
    render()
    scene.sendPointerEvent(PointerEventType.Release, start + delta, button = PointerButton.Primary)
    render()
  }

  fun awaitResizeCommit(commits: List<Float>, expectedCount: Int) {
    // Saving is a LaunchedEffect; a fixed number of frames can precede snapshot delivery.
    val deadline = System.nanoTime() + 2_000_000_000L
    while (commits.size < expectedCount && System.nanoTime() < deadline) {
      render()
      Thread.yield()
    }
    assertEquals(expectedCount, commits.size, "Timed out waiting for pointer resize commit")
  }

  fun revealTagFullyWithin(tag: String, scrollTag: String) {
    repeat(20) {
      val viewport = taggedBounds(scrollTag)
      val node = taggedNode(tag)
      val top = node.positionInRoot.y
      val bottom = top + node.size.height
      if (top >= viewport.top && bottom <= viewport.bottom) return
      scrollBy(
          if (bottom > viewport.bottom) bottom - viewport.bottom else top - viewport.top, scrollTag)
      render()
    }
    error("$tag must fit within $scrollTag after scrolling")
  }

  fun assertDescriptionFullyVisible(label: String, withinTag: String) {
    fun node() =
        nodes().single { candidate ->
          candidate.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) ==
              true &&
              generateSequence(candidate) { it.parent }
                  .any { it.config.getOrNull(SemanticsProperties.TestTag) == withinTag }
        }
    fun fullyVisible(): Boolean {
      val node = node()
      val bounds = node.boundsInRoot
      return bounds.width > 0f &&
          bounds.height > 0f &&
          bounds.width >= node.size.width - 1f &&
          bounds.height >= node.size.height - 1f
    }
    val deadline = System.nanoTime() + 2_000_000_000L
    while (!fullyVisible() && System.nanoTime() < deadline) {
      Thread.sleep(10)
      render()
    }
    assertTrue(
        fullyVisible(), "$label must not be clipped: ${node().boundsInRoot} vs ${node().size}")
  }

  fun requestDescriptionFocus(label: String, withinTag: String? = null): Boolean =
      nodes()
          .asSequence()
          .filter { node ->
            node.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) ==
                true &&
                (withinTag == null ||
                    generateSequence(node) { it.parent }
                        .any { it.config.getOrNull(SemanticsProperties.TestTag) == withinTag })
          }
          .flatMap { node -> generateSequence(node) { it.parent } }
          .mapNotNull { it.config.getOrNull(SemanticsActions.RequestFocus)?.action }
          .firstOrNull()
          ?.invoke() ?: false

  fun hasScrollableContent(): Boolean =
      nodes().any { it.config.getOrNull(SemanticsActions.ScrollBy) != null }

  fun scrollBy(pixels: Float, tag: String? = null) {
    val scroll =
        nodes()
            .filter {
              it.config.getOrNull(SemanticsProperties.VerticalScrollAxisRange) != null &&
                  (tag == null || it.config.getOrNull(SemanticsProperties.TestTag) == tag)
            }
            .firstNotNullOfOrNull { it.config.getOrNull(SemanticsActions.ScrollBy)?.action }
    assertTrue(requireNotNull(scroll).invoke(0f, pixels))
  }

  fun scrollMaximum(tag: String, horizontal: Boolean): Float =
      scrollAxis(tag, horizontal).maxValue()

  fun scrollPosition(tag: String, horizontal: Boolean): Float = scrollAxis(tag, horizontal).value()

  private fun scrollAxis(tag: String, horizontal: Boolean) =
      requireNotNull(
          taggedNode(tag)
              .config
              .getOrNull(
                  if (horizontal) SemanticsProperties.HorizontalScrollAxisRange
                  else SemanticsProperties.VerticalScrollAxisRange))

  fun scrollTagged(tag: String, horizontal: Boolean, pixels: Float) {
    assertTrue(scrollMaximum(tag, horizontal) > 0f)
    val scroll = taggedNode(tag).config.getOrNull(SemanticsActions.ScrollBy)?.action
    assertTrue(
        requireNotNull(scroll)
            .invoke(if (horizontal) pixels else 0f, if (horizontal) 0f else pixels))
    render()
  }

  fun taggedBounds(tag: String): Rect = taggedNode(tag).boundsInRoot

  fun descriptionBounds(label: String): Rect =
      nodes()
          .single {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
          }
          .boundsInRoot

  fun tagCount(tag: String): Int =
      nodes().count { it.config.getOrNull(SemanticsProperties.TestTag) == tag }

  fun tagIsDecorative(tag: String): Boolean {
    val config = taggedNode(tag).config
    return config.getOrNull(SemanticsProperties.ContentDescription) == null &&
        config.getOrNull(SemanticsProperties.Text) == null &&
        config.getOrNull(SemanticsActions.OnClick) == null
  }

  private fun taggedNode(tag: String): SemanticsNode =
      nodes().single { it.config.getOrNull(SemanticsProperties.TestTag) == tag }

  private fun horizontalScrollerWithin(tag: String): SemanticsNode =
      nodes().single { node ->
        node.config.getOrNull(SemanticsProperties.HorizontalScrollAxisRange) != null &&
            generateSequence(node.parent) { it.parent }
                .any { it.config.getOrNull(SemanticsProperties.TestTag) == tag }
      }

  fun horizontalScrollWithin(tag: String, pixels: Float) {
    assertTrue(
        requireNotNull(
                horizontalScrollerWithin(tag).config.getOrNull(SemanticsActions.ScrollBy)?.action)
            .invoke(pixels, 0f))
  }

  fun awaitHorizontalScrollWithinValue(tag: String, expected: Float) {
    val deadline = System.nanoTime() + 2_000_000_000L
    while (kotlin.math.abs(horizontalScrollWithinValue(tag) - expected) > 0.5f &&
        System.nanoTime() < deadline) {
      Thread.sleep(10)
      render()
    }
    assertEquals(expected, horizontalScrollWithinValue(tag), 0.5f)
  }

  fun horizontalScrollWithinValue(tag: String): Float =
      requireNotNull(
              horizontalScrollerWithin(tag)
                  .config
                  .getOrNull(SemanticsProperties.HorizontalScrollAxisRange))
          .value()

  fun horizontalScrollBy(tag: String, pixels: Float) {
    assertTrue(
        requireNotNull(taggedNode(tag).config.getOrNull(SemanticsActions.ScrollBy)?.action)
            .invoke(pixels, 0f))
  }

  fun horizontalScrollValue(tag: String): Float =
      requireNotNull(
              taggedNode(tag).config.getOrNull(SemanticsProperties.HorizontalScrollAxisRange))
          .value()

  fun verticalScrollValue(tag: String): Float =
      requireNotNull(taggedNode(tag).config.getOrNull(SemanticsProperties.VerticalScrollAxisRange))
          .value()

  private fun sourceTextPosition(line: Int, offset: Int): Offset {
    val text =
        nodes().single { node ->
          node.config.getOrNull(SemanticsProperties.Text) != null &&
              generateSequence(node.parent) { it.parent }
                  .any { it.config.getOrNull(SemanticsProperties.TestTag) == "source-code-$line" }
        }
    val layouts = mutableListOf<TextLayoutResult>()
    assertTrue(
        requireNotNull(text.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action)
            .invoke(layouts))
    val cursor = layouts.single().getCursorRect(offset)
    val position = text.positionInRoot + cursor.center + Offset(1f, 0f)
    val viewport = taggedBounds("source-code-$line")
    assertTrue(
        viewport.contains(position), "Source position must be visible: $position in $viewport")
    return position
  }

  fun tapSourceText(line: Int, offset: Int) = tapPosition(sourceTextPosition(line, offset))

  fun tapTag(tag: String) = tapPosition(taggedBounds(tag).center)

  private fun tapPosition(position: Offset) {
    scene.sendPointerEvent(
        PointerEventType.Press,
        position,
        timeMillis = frameTime / 1_000_000L,
        button = PointerButton.Primary)
    render()
    scene.sendPointerEvent(
        PointerEventType.Release,
        position,
        timeMillis = frameTime / 1_000_000L,
        button = PointerButton.Primary)
    render()
  }

  fun dragSourceText(startLine: Int, startOffset: Int, endLine: Int, endOffset: Int) {
    val start = sourceTextPosition(startLine, startOffset)
    val end = sourceTextPosition(endLine, endOffset)
    // Use the render clock so fast offscreen gestures cannot become OS-timed double clicks.
    scene.sendPointerEvent(
        PointerEventType.Press,
        start,
        timeMillis = frameTime / 1_000_000L,
        button = PointerButton.Primary)
    render()
    repeat(12) { step ->
      scene.sendPointerEvent(
          PointerEventType.Move,
          start + (end - start) * ((step + 1) / 12f),
          timeMillis = frameTime / 1_000_000L)
      render()
    }
    scene.sendPointerEvent(
        PointerEventType.Release,
        end,
        timeMillis = frameTime / 1_000_000L,
        button = PointerButton.Primary)
    render()
  }

  fun typeCharacter(key: Key, character: Char): Boolean {
    val typed =
        java.awt.event.KeyEvent(
            java.awt.Canvas(),
            java.awt.event.KeyEvent.KEY_TYPED,
            0L,
            0,
            java.awt.event.KeyEvent.VK_UNDEFINED,
            character)
    val down =
        scene.sendKeyEvent(
            KeyEvent(key, KeyEventType.KeyDown, codePoint = character.code, nativeEvent = typed))
    val up = scene.sendKeyEvent(KeyEvent(key, KeyEventType.KeyUp))
    render()
    return down || up
  }

  fun copyTextByDragging(label: String, expectedText: String? = null): String {
    var copied = ""
    // A drag can scroll the enclosing viewport; locate the text again before each attempt.
    for (inset in listOf(1f, 4f, 8f, 12f)) {
      val bounds = textNodes(label).first().boundsInRoot
      clipboard.nativeClipboard.setContents(java.awt.datatransfer.StringSelection(""), null)
      val start = Offset(bounds.left + inset, bounds.top + minOf(10f, bounds.height / 2f))
      val end = Offset(minOf(bounds.right - 2f, bounds.left + 120f), start.y)
      scene.sendPointerEvent(PointerEventType.Press, start, button = PointerButton.Primary)
      render()
      scene.sendPointerEvent(PointerEventType.Move, (start + end) / 2f)
      render()
      scene.sendPointerEvent(PointerEventType.Move, end)
      render()
      scene.sendPointerEvent(PointerEventType.Release, end, button = PointerButton.Primary)
      render()
      pressKey(Key.Copy)
      render()
      copied = clipboard.nativeClipboard.getData(DataFlavor.stringFlavor) as String
      if (copied.isNotEmpty() &&
          (expectedText == null || (copied.length >= 3 && expectedText.contains(copied)))) {
        return copied
      }
    }
    error("Dragging $label copied '$copied' instead of text from '${expectedText ?: label}'")
  }

  fun scrollableContentCount(): Int =
      nodes().count { it.config.getOrNull(SemanticsActions.ScrollBy) != null }

  fun assertTextFontFamily(label: String, expected: FontFamily) {
    val matches = textNodes(label)
    assertTrue(matches.isNotEmpty(), "$label must be rendered")
    matches.forEach { node ->
      val layouts = mutableListOf<TextLayoutResult>()
      assertTrue(
          node.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action?.invoke(layouts) ==
              true,
          "$label must expose its text layout")
      assertTrue(layouts.isNotEmpty(), "$label must have a text layout")
      layouts.forEach { assertEquals(expected, it.layoutInput.style.fontFamily, label) }
    }
  }

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

  fun pressKey(key: Key, shift: Boolean = false): Boolean {
    val keyDown = scene.sendKeyEvent(KeyEvent(key, KeyEventType.KeyDown, isShiftPressed = shift))
    val keyUp = scene.sendKeyEvent(KeyEvent(key, KeyEventType.KeyUp, isShiftPressed = shift))
    return keyDown || keyUp
  }

  fun dismissPopup(): Boolean =
      nodes()
          .asSequence()
          .mapNotNull { it.config.getOrNull(SemanticsActions.Dismiss)?.action }
          .firstOrNull()
          ?.invoke() ?: false

  fun revealText(label: String, scrollTag: String? = null) {
    fun visible(): Boolean =
        textNodes(label).any { node ->
          val layouts = mutableListOf<TextLayoutResult>()
          node.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action?.invoke(layouts)
          val bounds = node.boundsInRoot
          bounds.height > 0 &&
              bounds.top >= 0 &&
              bounds.bottom <= height &&
              layouts.any { it.size.height <= bounds.height + 1f }
        }
    if (visible()) return
    scrollBy(-100_000f, scrollTag)
    render()
    val step = scrollTag?.let { minOf(160f, taggedBounds(it).height / 3f) } ?: 160f
    repeat(300) {
      if (visible()) return
      scrollBy(step, scrollTag)
      render()
    }
    error("$label must be reachable by scrolling")
  }

  fun revealTextFullyWithin(label: String, scrollTag: String) {
    revealText(label, scrollTag)
    repeat(100) {
      val viewport = taggedBounds(scrollTag)
      val text = firstVisibleTextBounds(label)
      if (text.top >= viewport.top + 4f && text.bottom <= viewport.bottom - 4f) return
      val delta =
          if (text.top < viewport.top + 4f) text.top - viewport.top - 4f
          else text.bottom - viewport.bottom + 4f
      scrollBy(delta, scrollTag)
      render()
    }
    error("$label must fit within $scrollTag after scrolling")
  }

  fun assertTextOrder(labels: List<String>) {
    val positions = labels.map { textNodes(it).single().positionInRoot.y }
    positions.zipWithNext().forEach { (first, second) ->
      assertTrue(first < second, "All choices must retain their returned order")
    }
  }

  fun assertEveryTextLineReachable(
      label: String,
      scrollTag: String,
      withinTag: String? = null,
      medianSide: String? = null,
  ) {
    fun node() =
        textNodes(label)
            .filter { node ->
              (withinTag == null ||
                  generateSequence(node) { it.parent }
                      .any { it.config.getOrNull(SemanticsProperties.TestTag) == withinTag }) &&
                  (medianSide == null ||
                      generateSequence(node) { it.parent }
                          .any {
                            it.config.getOrNull(SemanticsProperties.ContentDescription)?.any {
                                description ->
                              description.contains("$medianSide median:")
                            } == true
                          })
            }
            .also { assertEquals(1, it.size, "Full text must be present: $label") }
            .single()
    val layouts = mutableListOf<TextLayoutResult>()
    requireNotNull(node().config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action)
        .invoke(layouts)
    val layout = layouts.single()
    assertEquals(
        label.length,
        layout.getLineEnd(layout.lineCount - 1),
        "The complete value must be laid out")
    assertFalse(layout.didOverflowWidth, "$label must wrap within the available width")
    for (line in 0 until layout.lineCount) {
      assertFalse(layout.isLineEllipsized(line), "$label line $line must not be truncated")
      val viewport = taggedBounds(scrollTag)
      repeat(10) {
        val top = node().positionInRoot.y + layout.getLineTop(line)
        val bottom = node().positionInRoot.y + layout.getLineBottom(line)
        if (top < viewport.top || bottom > viewport.bottom) {
          scrollBy((top + bottom) / 2f - viewport.center.y, scrollTag)
          render()
        }
      }
      val visibleTop = node().positionInRoot.y + layout.getLineTop(line)
      val visibleBottom = node().positionInRoot.y + layout.getLineBottom(line)
      assertTrue(
          visibleTop >= viewport.top - 1f && visibleBottom <= viewport.bottom + 1f,
          "$label line $line must be reachable inside $scrollTag: $visibleTop..$visibleBottom in $viewport")
    }
  }

  fun assertTextWrapsAndTailIsReachable(label: String, scrollTag: String) {
    val node = textNodes(label).single()
    val layouts = mutableListOf<TextLayoutResult>()
    requireNotNull(node.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action)
        .invoke(layouts)
    assertTrue(layouts.any { it.lineCount > 1 }, "$label must wrap instead of being clipped")
    scrollBy(100_000f, scrollTag)
    render()
    val tail = textNodes(label).single().boundsInRoot
    assertTrue(
        tail.bottom > 0 && tail.bottom <= height,
        "$label tail must be reachable by scrolling: $tail in ${width}x$height")
  }

  fun revealSummaryStatus(label: String) {
    fun visible(): Boolean =
        textNodes(label).any { node ->
          val belongsToCoverage =
              generateSequence(node) { it.parent }
                  .any {
                    it.config.getOrNull(SemanticsProperties.TestTag) == "summary-analysis-status"
                  }
          val bounds = node.boundsInRoot
          belongsToCoverage && bounds.height > 0 && bounds.top >= 0 && bounds.bottom <= height
        }
    if (visible()) return
    scrollBy(-100_000f)
    render()
    repeat(100) {
      if (visible()) return
      scrollBy(160f)
      render()
    }
    error("$label in Analysis coverage must be reachable by scrolling")
  }

  fun assertReferenceSummaryGeometry() {
    fun bounds(tag: String) =
        nodes().single { it.config.getOrNull(SemanticsProperties.TestTag) == tag }.boundsInRoot
    val heading = bounds("summary-page-heading")
    val introduction = bounds("summary-introduction")
    val coverage = bounds("analysis-summary")
    val dial = bounds("summary-coverage-dial")
    assertTrue(heading.bottom <= introduction.top, "Summary heading must lead the page")
    assertTrue(introduction.bottom <= coverage.top, "Coverage must follow the introduction")
    val region = bounds("summary-coverage-results")
    val results = bounds("summary-results")
    assertEquals(region.left, coverage.left, 1f)
    if (coverageResultsStacked(region.width.dp, 1f)) {
      assertEquals(region.width, coverage.width, 1f)
      assertTrue(coverage.bottom <= results.top)
    } else {
      assertTrue(coverage.right <= results.left)
      assertTrue(results.right <= region.right)
    }
    assertTrue(dial.width > 0f && dial.height > 0f, "Coverage dial must remain visible")
  }

  fun assertNarrativeSectionOrder(withInsight: Boolean = false) {
    val narrative = taggedBounds("summary-lower-composition")
    val tags = listOfNotNull("summary-architecture", "summary-insight".takeIf { withInsight })
    val sections = tags.map(::taggedBounds)
    assertEquals(narrative.top, sections.first().top, 1f)
    assertEquals(narrative.bottom, sections.maxOf { it.bottom }, 1f)
    val architecture = taggedBounds("summary-architecture")
    val evidence = taggedBounds("summary-evidence-row")
    val modules = taggedBounds("summary-modules")
    val findings = taggedBounds("summary-selected-findings")
    val flows = taggedBounds("summary-flows")
    assertEquals(narrative.width, evidence.width, 1f)
    assertEquals(narrative.width, flows.width, 1f)
    assertEquals(evidence.bottom + MiniOrcaSpacing.section.value, flows.top, 1f)
    if (withInsight) {
      val insight = taggedBounds("summary-insight")
      assertTrue(architecture.right <= insight.left || architecture.bottom <= insight.top)
    }
    assertTrue(narrative.bottom <= evidence.top)
    if (modules.width >= evidence.width - 1f) {
      assertEquals(evidence.width, modules.width, 1f)
      assertEquals(evidence.width, findings.width, 1f)
      assertTrue(modules.bottom <= findings.top)
    } else {
      assertTrue(modules.right <= findings.left)
      assertEquals(modules.top, findings.top, 1f)
    }
  }

  fun assertTextFits(label: String, maxLines: Int = 1) {
    assertTextLayout(label, lineCounts = 1..maxLines)
  }

  fun assertFixedSummaryCoverageLayout() {
    val coverage = taggedBounds("analysis-summary")
    val heading = firstVisibleTextBounds("Analysis coverage")
    val status = taggedBounds("summary-analysis-status")
    assertTrue(status.left >= heading.right, "Coverage status must trail the heading")
    assertTrue(
        status.top < heading.bottom && status.bottom > heading.top,
        "Coverage status must share the heading row")
    val dial = taggedBounds("summary-coverage-dial")
    val legend = taggedBounds("summary-coverage-legend")
    val action = taggedBounds("summary-view-analysis")
    assertTrue(dial.left >= coverage.left && dial.right <= coverage.right)
    assertTrue(legend.left >= coverage.left && legend.right <= coverage.right)
    assertTrue(action.left >= coverage.left && action.right <= coverage.right)
    assertTrue(dial.bottom <= legend.top, "Coverage legend must follow the dial")
    assertTrue(legend.bottom <= action.top, "View analysis must follow the legend")
  }

  fun assertSummaryCategoryBoxesFit() {
    val cards =
        AnalysisResultType.entries.map { type ->
          visibleActionBounds("View ${type.workspace.name} results")
        }
    cards.forEach { bounds ->
      assertTrue(bounds.height >= 132f, "Summary category cards must retain a minimum height")
      assertTrue(
          bounds.left >= 0 && bounds.top >= 0 && bounds.right <= width && bounds.bottom <= height,
          "Every summary card must be visible: $bounds")
    }
    cards.zipWithNext().forEach { (first, next) ->
      assertTrue(first.right <= next.left, "Wide summary categories must share one row")
      assertEquals(first.top, next.top, 1f, "Wide summary categories must align at the top")
      assertEquals(first.width, next.width, 1f, "Wide summary category widths must match")
      assertEquals(first.height, next.height, 1f, "Wide summary category heights must match")
    }
  }

  fun assertSummaryCategoryContentContained() {
    AnalysisResultType.entries.forEach { type ->
      val card = taggedBounds("summary-metric-${type.workspace.name}")
      val taggedChildren =
          nodes().filter { node ->
            val tag = node.config.getOrNull(SemanticsProperties.TestTag) ?: return@filter false
            tag.startsWith("summary-category-") &&
                generateSequence(node) { it.parent }
                    .any {
                      it.config.getOrNull(SemanticsProperties.TestTag) ==
                          "summary-metric-${type.workspace.name}"
                    }
          }
      assertTrue(taggedChildren.isNotEmpty(), "${type.workspace.name} must expose tagged content")
      taggedChildren.forEach { child ->
        val bounds = child.boundsInRoot
        assertTrue(
            bounds.left >= card.left &&
                bounds.top >= card.top &&
                bounds.right <= card.right &&
                bounds.bottom <= card.bottom,
            "${child.config.getOrNull(SemanticsProperties.TestTag)} must remain inside ${type.workspace.name}: $bounds in $card")
      }
    }
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
      assertEquals(reference.width, bounds.width, 1f, "Summary card widths must match")
      assertEquals(reference.height, bounds.height, "Summary card heights must match")
      assertTrue(
          bounds.left >= 0 && bounds.top >= 0 && bounds.right <= width && bounds.bottom <= height,
          "Every summary card must be visible: $bounds")
    }
  }

  fun assertBugPrioritiesInsideCard(label: String) {
    assertTrue(
        generateSequence(textNodes(label).single()) { it.parent }
            .any { it.config.getOrNull(SemanticsProperties.TestTag) == "summary-metric-Bugs" },
        "Priority counts must stay inside the Bugs box")
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
        "The status must belong to Analysis coverage")
    val introduction =
        nodes().single {
          it.config.getOrNull(SemanticsProperties.TestTag) == "summary-introduction"
        }
    assertTrue(introduction.boundsInRoot.bottom <= status.boundsInRoot.top)
    val panel =
        nodes().single { it.config.getOrNull(SemanticsProperties.TestTag) == "analysis-summary" }
    assertTrue(
        panel.boundsInRoot.right - status.boundsInRoot.right <= 28f,
        "$label must align to the right edge of Analysis coverage")
  }

  fun assertWideSummaryCoverageLayout() {
    val section = taggedBounds("analysis-summary")
    val dial = taggedBounds("summary-coverage-dial")
    val legend = taggedBounds("summary-coverage-legend")
    val action = taggedBounds("summary-view-analysis")
    assertTrue(dial.width > 0f && dial.height > 0f)
    assertTrue(dial.top < legend.top, "Coverage legend must follow the dial")
    assertTrue(legend.bottom <= action.top, "View analysis must follow the legend")
    assertTrue(action.right <= section.right, "View analysis must stay inside coverage")
  }

  fun assertAnalysisRunGeometry() {
    val content = taggedBounds("analysis-run-content")
    val progress = taggedBounds("analysis-run-progress-track")
    val controls = taggedBounds("analysis-run-controls")
    val title = textNodes("Analyzing selected files").single().boundsInRoot
    val finished = textNodes("8 of 12 files finished").single().boundsInRoot
    val current = textNodes("Current: internal/api/user.go").single().boundsInRoot
    assertTrue(title.bottom <= finished.top, "Run title must precede the finished-file count")
    assertTrue(finished.bottom <= progress.top, "Progress must follow the title and file count")
    assertTrue(progress.bottom <= current.top, "Current file must follow progress")
    assertTrue(controls.top >= content.bottom, "Run controls must follow run content")
    assertTrue(
        controls.right <= taggedBounds("analysis-run-panel").right,
        "Run controls must fit inside the panel")
  }

  fun assertAnalysisCategoryGeometry() {
    val boxes =
        AnalysisResultType.entries.map { type ->
          taggedBounds("analysis-category-${type.category}")
        }
    boxes.forEach { box ->
      assertEquals(boxes.first().width, box.width, 1f, "Analysis category widths must match")
      assertEquals(boxes.first().height, box.height, 1f, "Analysis category heights must match")
    }
    boxes.zipWithNext().forEach { (first, second) ->
      assertTrue(first.right <= second.left, "Wide categories must share one row")
      assertEquals(first.top, second.top, 1f, "Wide categories must share a top edge")
    }
    AnalysisResultType.entries.forEach { type ->
      val icon = taggedBounds("analysis-category-icon-${type.category}")
      val content = taggedBounds("analysis-category-content-${type.category}")
      assertTrue(icon.right < content.left, "${type.category} icon must lead its content")
    }
  }

  fun assertAnalysisTableColumns() {
    val headers =
        listOf("File", "Analysis state", "Details").map { label ->
          textNodes(label)
              .filter { it.boundsInRoot.width > 0f && it.boundsInRoot.height > 0f }
              .minBy { it.boundsInRoot.top }
              .boundsInRoot
        }
    headers.zipWithNext().forEach { (first, second) ->
      assertTrue(first.right < second.left, "Headers must remain separate: $first and $second")
      assertEquals(
          first.center.y, second.center.y, 2f, "Headers must share a row: $first and $second")
    }
    assertAnalysisTableColumnsForRow(
        "analysis-file-row-cmd/server/main.go", "cmd/server/main.go", "Up to date", "Complete")
  }

  fun assertAnalysisTableColumnsForRow(row: String, path: String, status: String, summary: String) {
    val headers = listOf("File", "Analysis state", "Details").map(::firstVisibleTextBounds)
    assertAnalysisTableCellInColumn(row, path, headers[0], headers[1], "File")
    assertAnalysisTableCellInColumn(row, status, headers[1], headers[2], "Analysis state")
    assertAnalysisTableCellInColumn(row, summary, headers[2], null, "Details")
  }

  private fun assertAnalysisTableCellInColumn(
      rowTag: String,
      label: String,
      header: Rect,
      nextHeader: Rect?,
      column: String,
  ) {
    val cell = taggedTextBounds(rowTag, label)
    val row = taggedBounds(rowTag)
    assertEquals(
        header.left,
        cell.left,
        1f,
        "$label must share the $column column leading edge with its header")
    assertTrue(
        nextHeader == null || cell.right <= nextHeader.left,
        "$label must remain within the $column column")
    assertTrue(cell.right <= row.right, "$label must remain inside its row")
  }

  fun taggedTextBounds(tag: String, label: String): Rect {
    val row = taggedBounds(tag)
    return textNodes(label)
        .single { node ->
          val bounds = node.boundsInRoot
          bounds.width > 0f &&
              bounds.height > 0f &&
              bounds.left >= row.left &&
              bounds.right <= row.right &&
              bounds.top >= row.top &&
              bounds.bottom <= row.bottom
        }
        .boundsInRoot
  }

  fun assertTextBefore(label: String, following: String) {
    val first = textNodes(label).single().boundsInRoot
    val second = textNodes(following).single().boundsInRoot
    assertTrue(
        first.right < second.left,
        "$label must be left of $following at $width/$height/$fontScale: $first $second")
    assertTrue(
        kotlin.math.abs(first.center.y - second.center.y) < 2f,
        "$label and $following must share a row")
  }

  fun assertTextSharesRowBefore(label: String, following: String) {
    val first = textNodes(label).single().boundsInRoot
    val second = textNodes(following).single().boundsInRoot
    assertTrue(first.right < second.left, "$label must be left of $following")
    assertTrue(
        maxOf(first.top, second.top) < minOf(first.bottom, second.bottom),
        "$label and $following must share a row")
  }

  fun assertTextAbove(label: String, following: String) {
    val first = textNodes(label).single().boundsInRoot
    val second = textNodes(following).single().boundsInRoot
    assertTrue(first.bottom < second.top, "$label must be above $following")
  }

  fun assertTextAboveDescription(label: String, description: String) {
    val text = textNodes(label).single().boundsInRoot
    val actions =
        nodes().filter {
          it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(description) == true
        }
    assertTrue(actions.isNotEmpty(), "$description must be accessible")
    assertTrue(
        text.bottom <= actions.minOf { it.boundsInRoot.top },
        "$label $text must be above $description ${actions.map { it.boundsInRoot }}")
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

  fun assertRailLabelFits(label: String) {
    val entry =
        nodes()
            .single {
              it.config.getOrNull(SemanticsProperties.ContentDescription)?.any { description ->
                description.startsWith("$label tool window,")
              } == true
            }
            .boundsInRoot
    val text =
        textNodes(label).single { node ->
          generateSequence(node) { it.parent }.any { it.boundsInRoot == entry }
        }
    assertTextFits(label)
    val layouts = mutableListOf<TextLayoutResult>()
    text.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action?.invoke(layouts)
    val layout = layouts.single()
    val glyphs = label.indices.map(layout::getBoundingBox)
    val left = text.boundsInRoot.left + glyphs.minOf { it.left }
    val right = text.boundsInRoot.left + glyphs.maxOf { it.right }
    assertTrue(
        left >= entry.left - 1f &&
            right <= entry.right + 1f &&
            glyphs.maxOf { it.right } <= layout.size.width + 1f &&
            layout.multiParagraph.intrinsics.maxIntrinsicWidth <= layout.size.width + 1f,
        "$label glyphs ($left..$right) must fit in rail entry $entry (text width ${layout.size.width})")
  }

  fun assertRailLabelsOrdered() {
    val labels = listOf("Summary", "Analysis", "Bugs", "Performance", "Security", "Editor")
    val positions =
        labels.map { label -> textNodes(label).minBy { it.boundsInRoot.left }.boundsInRoot }
    positions.zipWithNext().forEach { (above, below) ->
      assertTrue(above.bottom < below.top, "Rail destinations must follow their display order")
    }
  }

  fun assertWorkspaceFrameGeometry(docked: Boolean) {
    fun bounds(label: String): Rect {
      val bounds =
          nodes()
              .single {
                it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
              }
              .boundsInRoot
      assertTrue(
          bounds.width > 0 &&
              bounds.height > 0 &&
              bounds.left >= 0 &&
              bounds.top >= 0 &&
              bounds.right <= width &&
              bounds.bottom <= height,
          "$label must fit: $bounds")
      return bounds
    }
    val editor = bounds("Editor area")
    val terminal = bounds("Terminal fixture")
    val panes =
        if (docked) listOf(bounds("Files tool window"), editor, bounds("Tool windows tool window"))
        else listOf(editor)
    assertEquals(
        TOOL_WINDOW_BAR_WIDTH * maxOf(1f, fontScale) + WORKSPACE_FRAME_INSET,
        panes.first().left,
        1f,
        "The rail and outer inset must stay visible")
    assertEquals(width - 8f, panes.last().right, 1f, "The trailing frame must stay visible")
    panes.forEach { pane ->
      assertEquals(editor.top, pane.top, 1f)
      assertEquals(editor.bottom, pane.bottom, 1f, "All panes must end above the terminal")
    }
    panes.zipWithNext().forEach { (left, right) ->
      assertEquals(8f, right.left - left.right, 1f, "A single gutter separates adjacent panes")
    }
    assertEquals(panes.first().left, terminal.left, 1f)
    assertEquals(panes.last().right, terminal.right, 1f)
    assertEquals(8f, terminal.top - editor.bottom, 1f)
    val rendered =
        surface.makeImageSnapshot().use { snapshot ->
          requireNotNull(snapshot.encodeToData()).use { data ->
            javax.imageio.ImageIO.read(java.io.ByteArrayInputStream(data.bytes))
          }
        }
    (panes + terminal).forEach { pane ->
      val paneFill = if (pane == editor) EditorCanvas else ToolWindowSurface
      assertEquals(
          EditorCanvas.toArgb(),
          rendered.getRGB(pane.center.x.toInt(), (pane.top - 2).toInt()),
          "Filled children must not escape the pane into the outer inset")
      // Focus indicators and header controls may cross the pane midpoint after rail resizing.
      val topFill =
          (1..9).count { step ->
            rendered.getRGB((pane.left + pane.width * step / 10).toInt(), (pane.top + 2).toInt()) ==
                paneFill.toArgb()
          }
      assertTrue(
          topFill >= 2,
          "The top edge must show the actual filled pane at $width x $height / $fontScale: $pane ($topFill of 9 samples)")
    }
  }

  fun assertFlatCorner(tag: String, fill: Color) {
    val bounds = taggedBounds(tag)
    val rendered =
        surface.makeImageSnapshot().use { snapshot ->
          requireNotNull(snapshot.encodeToData()).use { data ->
            javax.imageio.ImageIO.read(java.io.ByteArrayInputStream(data.bytes))
          }
        }
    val left = bounds.left.toInt()
    val top = bounds.top.toInt()
    assertTrue(rendered.getRGB(left, top) != fill.toArgb(), "$tag corner must remain clipped")
    assertEquals(
        fill.toArgb(),
        rendered.getRGB(left + 5, top + 5),
        "$tag must fill just inside its flat corner")
    assertEquals(
        (if (tag == "flat-field" || tag == "flat-button") ControlBorder else PaneSeparator)
            .toArgb(),
        rendered.getRGB(left + bounds.width.toInt() / 2, top),
        "$tag top keyline must remain visible")
  }

  fun assertTopKeyline(tag: String, color: Color) {
    val bounds = taggedBounds(tag)
    val rendered =
        surface.makeImageSnapshot().use { snapshot ->
          requireNotNull(snapshot.encodeToData()).use { data ->
            javax.imageio.ImageIO.read(java.io.ByteArrayInputStream(data.bytes))
          }
        }
    assertEquals(
        color.toArgb(),
        rendered.getRGB(bounds.center.x.toInt(), bounds.top.toInt()),
        "$tag must separate from the content above")
  }

  fun renderedPixels(tag: String): IntArray {
    val bounds = taggedBounds(tag)
    return surface.makeImageSnapshot().use { snapshot ->
      requireNotNull(snapshot.encodeToData()).use { data ->
        val rendered = javax.imageio.ImageIO.read(java.io.ByteArrayInputStream(data.bytes))
        rendered.getRGB(
            bounds.left.toInt(),
            bounds.top.toInt(),
            bounds.width.toInt(),
            bounds.height.toInt(),
            null,
            0,
            bounds.width.toInt())
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

internal val visualFixtureProject =
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

internal val visualFixtureOverview =
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
                  status = "completed",
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
    gitStatus: GitStatus? = GitStatus(available = true, branch = "main"),
    actions: ToolbarActions = ToolbarActions({}, {}, {}, {}),
    paletteFocusRequester: FocusRequester? = null,
    analysisStatus: ToolbarAnalysisStatus? = null,
) {
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(project, false, "", connection, gitStatus, analysisStatus),
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

internal fun roundedAnalysisStateFixture(): ProjectAnalysisRunState {
  val paths =
      listOf(
          "cmd/server/main.go",
          "internal/api/routes.go",
          "internal/api/user.go",
          "internal/db/store.go",
          "internal/models/user.go",
          "internal/service/service.go") + (1..6).map { "internal/services/worker$it.go" }
  val finished = paths.take(2) + paths.takeLast(6)
  val run =
      analysisRunFixture().let { original ->
        original.copy(
            status = "running",
            plan =
                original.plan.copy(
                    files =
                        paths.map {
                          AnalysisPlannedFile(
                              it,
                              "base",
                              "Go",
                              20,
                              listOf(
                                  AnalysisStagePlan("semantic", true, false, maxModelRequests = 0)))
                        }),
            files =
                paths.map { path ->
                  AnalysisRunFile(
                      path,
                      "base",
                      "Go",
                      listOf(
                          AnalysisStageProgress(
                              "semantic",
                              when (path) {
                                in finished -> "completed"
                                "internal/api/user.go" -> "running"
                                else -> "pending"
                              },
                              1,
                              false)))
                },
            sections =
                original.sections.map {
                  it.copy(
                      status = "running",
                      findingCount = null,
                      coverage =
                          AnalysisRunCoverage(total = 12, succeeded = 8, running = 1, pending = 3))
                })
      }
  return ProjectAnalysisRunState(
      run = run,
      fileSelection =
          AnalysisSelectionState(
              selectionFixture()
                  .copy(
                      editable = false,
                      files =
                          paths.map {
                            AnalysisSelectableFile(
                                it,
                                "",
                                selectionStageFixture(
                                    if (it in finished) "fresh" else "missing",
                                    if (it in finished) "Current" else "No saved analysis."))
                          } +
                              listOf("vendor/example.go", "generated/client.go", ".env").map {
                                AnalysisSelectableFile(it, "Project configuration")
                              })))
}

@Composable
internal fun RoundedAnalysisVisualFixture(width: Float) {
  val analysis = roundedAnalysisStateFixture()
  val project = resultProjectFixture().copy(name = "go-shop · fixture")
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            project,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            toolbarAnalysisStatus(
                DesktopState(
                    projectState = ProjectWorkspaceState(project = project),
                    analysisRun = analysis))),
        ToolbarActions({}, {}, {}, {}))
    WorkspaceFrame(
        rail = { ToolWindowBar(LeftToolWindow.Analysis, {}, onOpenTerminal = {}) },
        panes = {
          EditorArea(
              {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(project, analysis),
                    AnalysisWorkspaceActions(
                        { _, _ -> },
                        {},
                        {},
                        {},
                        {},
                        refreshStatus = { error("Unexpected status refresh") }))
              },
              Modifier.weight(1f))
        },
        terminal = {
          TerminalBar(TerminalWorkspaceState(), true, {}, TerminalTabActions({}, {}, {}))
        },
        modifier = Modifier.weight(1f))
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            DesktopStatusProviderPresentation("Visual fixture · no backend", false),
            "Models: 1 local · 2 cloud",
            "Visual fixture · no backend"),
        {})
  }
}

@Composable
internal fun RoundedSummaryVisualFixture(width: Float) {
  val overview =
      visualFixtureOverview.copy(
          analysis =
              visualFixtureOverview.analysis.copy(
                  architecture =
                      "HTTP handlers validate requests and delegate persistence to repository adapters.\n```mermaid\n${visualFixtureOverview.analysis.architecture}\n```",
                  engineeringInsight =
                      EngineeringInsight(
                          mechanism = "Validate at the request boundary.",
                          whyItMattersHere = "Keep invalid input out of the repository.")),
          analysisRun =
              visualFixtureOverview.analysisRun?.let { run ->
                run.copy(
                    sections =
                        run.sections
                            .filter { it.category != "security" }
                            .map {
                              it.copy(
                                  status = "completed",
                                  findingCount = if (it.category == "bugs") 4 else 2)
                            })
              })
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            toolbarAnalysisStatus(
                DesktopState(
                    projectState = ProjectWorkspaceState(project = visualFixtureProject),
                    analysisRun = ProjectAnalysisRunState(run = overview.analysisRun)))),
        ToolbarActions({}, {}, {}, {}))
    WorkspaceFrame(
        rail = { ToolWindowBar(LeftToolWindow.Summary, {}, onOpenTerminal = {}) },
        panes = {
          EditorArea(
              { ProjectSummaryPane(overview, visualFixtureProject, {}) }, Modifier.weight(1f))
        },
        terminal = {
          TerminalBar(TerminalWorkspaceState(), true, {}, TerminalTabActions({}, {}, {}))
        },
        modifier = Modifier.weight(1f))
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            DesktopStatusProviderPresentation("Visual fixture · no backend", false),
            "Models: 1 local · 2 cloud",
            "Visual fixture · no backend"),
        {})
  }
}

internal fun sourceNavigationReviewFixture(): ReviewToolWindowState {
  val original = editorComparisonReviewFixture()
  val path = "internal/" + "deeply/nested/日本語/".repeat(6) + "user.go"
  val file =
      requireNotNull(original.selected)
          .copy(
              path = path,
              content =
                  requireNotNull(original.selected).content +
                      "\n" +
                      "// long selectable source " +
                      "argument".repeat(100) +
                      "\n" +
                      (1..100).joinToString("\n") { "// retained source row $it" })
  val draft = requireNotNull(original.draft).copy(targetPath = path)
  return original.copy(
      selected = file,
      draft = draft,
      session = requireNotNull(original.session).copy(openPath = path))
}

internal fun editorComparisonReviewFixture(): ReviewToolWindowState {
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
  val declaration =
      """
      func GetUser(id string) (User, error) {
          if id == "" {
              return User{}, errors.New("missing user id")
          }

          return repository.Find(id)
      }
  """
          .trimIndent()
  val source = file.content.lines()
  val diff =
      UnifiedDiff(
          file.path,
          file.path,
          buildList {
            (5..9).forEach { add(DiffLine("context", it, it, source[it - 1])) }
            add(DiffLine("removed", 10, 0, source[9]))
            add(DiffLine("removed", 11, 0, source[10]))
            add(DiffLine("added", 0, 10, "    return repository.Find(id)"))
            add(DiffLine("context", 12, 11, "}"))
          })
  val draft =
      DeclarationDraft(
          "fixture-draft",
          visualFixtureProject.projectId,
          visualFixtureProject.projectRevision,
          file.contentHash,
          file.path,
          "replace_symbol",
          symbol.name,
          declaration,
          revision = 1,
          hash = "fixture-draft-hash",
          validation = DeclarationValidation(true, "replace_symbol", diff = diff))
  val session =
      ChatSession(
          "fixture-session",
          draft.projectId,
          draft.projectRevision,
          draft.baseFileHash,
          draft.targetPath,
          draft.mode,
          draft.targetSymbol,
          latestDraftId = draft.id)
  val checks =
      DraftCheckReport(
          file.path,
          true,
          checks = listOf(DraftCheck("go test", required = true, state = "passed")),
          draftId = draft.id,
          draftRevision = draft.revision,
          draftHash = draft.hash)
  return ReviewToolWindowState(
      visualFixtureProject,
      file,
      symbol,
      session,
      editableDraft(draft),
      draft,
      checks,
      null,
      null,
      null,
      false)
}

@Composable
internal fun EditorVisualFixture(
    width: Float,
    terminalExpanded: Boolean = false,
    comparison: Boolean = false,
    layout: DesktopLayoutState = DesktopLayoutState()
) {
  val layout = layout.copy(bottomCollapsed = !terminalExpanded)
  val explorerWidth = layout.explorerWidth
  val actionWidth = layout.actionWidth
  val review = editorComparisonReviewFixture()
  val file = requireNotNull(review.selected)
  val symbol = requireNotNull(review.selectedSymbol)
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
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            ToolbarAnalysisStatus(
                "Analysis · Completed", "Whole-project analysis · Completed", false, false)),
        ToolbarActions({}, {}, {}, {}))
    WorkspaceFrame(
        rail = { ToolWindowBar(LeftToolWindow.Editor, {}, onOpenTerminal = {}) },
        panes = {
          DockedToolWindow(
              title = "Files",
              content = { modifier ->
                ExplorerPane(
                    ExplorerPaneState(index, file.path, "", emptySet(), false),
                    ExplorerPaneActions({}, {}, {}, {}, {}),
                    modifier)
              },
              modifier = Modifier.requiredWidth(explorerWidth.dp).testTag("editor-files-dock"),
              showHeader = false)
          ResizableDivider({}, {})
          EditorArea(
              content = {
                EditorWorkspace(
                    editorChromeUiState(
                        file,
                        symbol,
                        if (comparison) EditorSurface.Review else EditorSurface.Source,
                        EditorProgressUiState(
                            if (comparison) EditorProgress.Review else EditorProgress.Inspect, ""),
                        if (comparison) review.draft else null),
                    if (comparison) review else null,
                    {},
                    {},
                    onEditDraft = {},
                    canvas = {
                      if (comparison) ReviewDiffCanvas(review.draft)
                      else
                          SourceEditorPane(
                              visualFixtureProject,
                              file,
                              listOf(symbol),
                              symbol,
                              7,
                              emptyList(),
                              {})
                    })
              },
              modifier = Modifier.weight(1f))
          ResizableDivider({}, {})
          DockedToolWindow(
              title = "Tool windows",
              content = { modifier ->
                RightToolWindowContainer(
                    if (comparison) RightToolWindow.Review else RightToolWindow.Context,
                    {},
                    content = { _, contentModifier ->
                      if (comparison)
                          ReviewToolWindow(
                              review,
                              ReviewToolWindowActions({}, {}, {}),
                              DraftApplicationActions({}, {}),
                              contentModifier)
                      else
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
                                          target =
                                              DeclarationExplanationTarget(
                                                  WorkflowFileIdentity(
                                                      WorkflowProjectIdentity(
                                                          "visual-fixture", "fixture-revision"),
                                                      file.path,
                                                      file.contentHash),
                                                  symbol.name,
                                                  symbol.signature,
                                                  symbol.startLine,
                                                  symbol.endLine),
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
                                                          providerOrigin =
                                                              "http://127.0.0.1:8080")),
                                          message =
                                              "Current explanation · lines ${symbol.startLine}–${symbol.endLine}")),
                              ContextToolWindowActions({}, {}, {}, {}, {}),
                              contentModifier)
                    },
                    modifier = modifier)
              },
              modifier = Modifier.requiredWidth(actionWidth.dp).testTag("editor-context-dock"),
              showHeader = false)
        },
        terminal = {
          TerminalDock(
              layout,
              TerminalWorkspaceState(),
              {},
              {},
              TerminalTabActions({}, {}, {}),
              {},
              {},
              { modifier -> Text("Synthetic shell", modifier = modifier.padding(8.dp)) },
              modifier =
                  Modifier.testTag("terminal-fixture-dock").semantics {
                    contentDescription = "Terminal fixture"
                  })
        },
        modifier = Modifier.weight(1f),
    )
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            DesktopStatusProviderPresentation(
                "Visual fixture · local-function-model · no backend", remoteProvider = false),
            "Models: 1 local · 2 cloud",
            "Visual fixture · no backend"),
        {})
  }
}

// Uses WorkspaceFrame's measured allocation and the same keyed arrangement as DesktopShell.
// Callbacks deliberately fail the test if passive composition dispatches a workflow action.
@Composable
internal fun AdaptiveProductionEditorFixture(
    layout: DesktopLayoutState,
    review: Boolean,
    evidence: ReviewToolWindowState = editorComparisonReviewFixture(),
    progress: EditorProgressUiState = EditorProgressUiState(EditorProgress.Review, ""),
    terminalState: TerminalWorkspaceState = TerminalWorkspaceState(),
    rightTool: RightToolWindow = if (review) RightToolWindow.Review else RightToolWindow.Context,
    onRightTool: (RightToolWindow) -> Unit = {},
    selectedSurface: EditorSurface = if (review) EditorSurface.Review else EditorSurface.Source,
    onSurface: (EditorSurface) -> Unit = {},
    draftInput: TextFieldValue? = null,
    onDraftInput: (TextFieldValue) -> Unit = {},
    onValidate: () -> Unit = {},
    onRequest: () -> Unit = {},
    onWrite: () -> Unit = {},
    onTerminal: () -> Unit = {},
    fileRead: FileReadUiState? = null,
    creationInProgress: Boolean = false,
    extraIndexedFiles: List<IndexedFile> = emptyList(),
    onLeftTool: (LeftToolWindow) -> Unit = {},
    onOpenFile: (String) -> Unit = {},
    onCreate: () -> Unit = {},
    onSourceLine: (SourceLineSelection) -> Unit = {},
    terminalCollapsed: Boolean = false,
) {
  val file = requireNotNull(evidence.selected)
  val symbol = requireNotNull(evidence.selectedSymbol)
  val draft = requireNotNull(evidence.draft)
  val index =
      ProjectIndex(
          "visual-fixture",
          "fixture-revision",
          files =
              listOf(
                  IndexedFile(file.path, file.contentHash, "Go", false, analysisStatus = "fresh")) +
                  extraIndexedFiles)
  val session = requireNotNull(evidence.session)
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            visualFixtureProject.copy(name = "A long project identity with 日本語 and many segments"),
            false,
            "",
            ConnectionState(connected = true),
            null),
        ToolbarActions({}, {}, {}, {}))
    WorkspaceFrame(
        rail = { ToolWindowBar(LeftToolWindow.Editor, onLeftTool, onOpenTerminal = onTerminal) },
        panes = {},
        editorPanes = { width, height ->
          val resolved = resolveDesktopLayout(layout, width, LocalDensity.current.fontScale)
          EditorPaneArrangement(
              resolved,
              layout,
              height,
              left = { modifier ->
                DockedToolWindow(
                    "Files",
                    { pane ->
                      ExplorerPane(
                          ExplorerPaneState(
                              index,
                              file.path,
                              "",
                              emptySet(),
                              false,
                              readError = (fileRead as? FileReadUiState.Failed)?.message,
                              pendingFilePath = (fileRead as? FileReadUiState.Pending)?.path,
                              failedFilePath = (fileRead as? FileReadUiState.Failed)?.path),
                          ExplorerPaneActions({}, {}, {}, {}, onOpenFile),
                          pane)
                    },
                    modifier.testTag("f04-files"),
                    showHeader = false)
              },
              canvas = { modifier ->
                EditorArea(
                    {
                      EditorWorkspace(
                          editorChromeUiState(
                              file, symbol, selectedSurface, progress, draft, creationInProgress),
                          evidence,
                          onSurface,
                          onCreate,
                          fileRead = fileRead,
                          onOpenFile = onOpenFile,
                          canvas = {
                            if (selectedSurface == EditorSurface.Review) ReviewDiffCanvas(draft)
                            else
                                SourceEditorPane(
                                    visualFixtureProject,
                                    file,
                                    listOf(symbol),
                                    symbol,
                                    7,
                                    emptyList(),
                                    onSourceLine)
                          })
                    },
                    modifier.testTag("f04-canvas"))
              },
              right = { modifier ->
                DockedToolWindow(
                    "Tool windows",
                    { pane ->
                      RightToolWindowContainer(
                          rightTool,
                          onRightTool,
                          content = { tab, contentModifier ->
                            when (tab) {
                              RightToolWindow.Review ->
                                  ReviewToolWindow(
                                      evidence,
                                      ReviewToolWindowActions(onWrite, onWrite, onWrite),
                                      DraftApplicationActions(onWrite, onWrite),
                                      contentModifier)
                              RightToolWindow.Context ->
                                  ContextToolWindow(
                                      contextVisualState()
                                          .copy(
                                              inspector =
                                                  symbolInspectorUiState(
                                                      file,
                                                      listOf(symbol),
                                                      symbol,
                                                      null,
                                                      false,
                                                      InspectorProviderState(false, false),
                                                      null)),
                                      ContextToolWindowActions({}, onRequest, onRequest, {}, {}),
                                      contentModifier)
                              RightToolWindow.Assistant ->
                                  AssistantToolWindow(
                                      AssistantToolWindowState(
                                          visualFixtureProject,
                                          file,
                                          session,
                                          draft,
                                          editableDraft(draft),
                                          ChatTarget(ChatEditMode.ReplaceSymbol, symbol.name),
                                          ChatEditMode.ReplaceSymbol,
                                          "",
                                          "Change the declaration",
                                          false,
                                          ScopedModel(),
                                          false,
                                          FocusRequester(),
                                          FocusRequester(),
                                          draftInput = draftInput),
                                      AssistantConversationActions(
                                          {}, {}, {}, {}, onRequest, onRequest),
                                      DraftEditorActions(
                                          {},
                                          {},
                                          onValidate,
                                          updateDeclarationValue = onDraftInput),
                                      contentModifier)
                            }
                          },
                          modifier = pane)
                    },
                    modifier.testTag("f04-tool"),
                    showHeader = false)
              },
              leftDivider = { ResizableDivider({}, {}) },
              rightDivider = { ResizableDivider({}, {}) },
              modifier = Modifier.testTag("f04-arrangement"))
        },
        terminal = { workspaceHeight ->
          TerminalDock(
              layout.copy(bottomCollapsed = terminalCollapsed),
              terminalState,
              {},
              {},
              TerminalTabActions({}, {}, {}),
              {},
              {},
              { modifier -> Box(modifier.background(EditorCanvas)) },
              effectiveHeight =
                  resolveTerminalDockHeight(
                      layout.copy(bottomCollapsed = terminalCollapsed),
                      workspaceHeight,
                      LocalDensity.current.fontScale),
              modifier = Modifier.testTag("f04-dock"))
        },
        modifier = Modifier.weight(1f))
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            DesktopStatusProviderPresentation("Visual fixture · no backend", false),
            "Models: unavailable",
            "Visual fixture · no backend"),
        {})
  }
}

private fun contextExplanationTarget(state: ContextToolWindowState): DeclarationExplanationTarget {
  val project = requireNotNull(state.project)
  val file = requireNotNull(state.inspector).file
  val symbol = requireNotNull(state.inspector.selectedSymbol).symbol
  return DeclarationExplanationTarget(
      WorkflowFileIdentity(
          WorkflowProjectIdentity(project.projectId, project.projectRevision),
          file.path,
          file.contentHash),
      symbol.name,
      symbol.signature,
      symbol.startLine,
      symbol.endLine)
}

internal fun contextVisualState(): ContextToolWindowState {
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
