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
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.InternalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Color
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
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.TextLayoutResult
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
    listOf(1600 to 1000, 1440 to 900, 1000 to 760, 999 to 760, 800 to 650, 1280 to 600).forEach {
        (width, height) ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        ComposeVisualFixture(width, height, scale) {
              EditorVisualFixture(width.toFloat(), comparison = true)
            }
            .use { fixture ->
              fixture.render("comparison-frame-$width-$height-$scale")
              listOf("Candidate diff", "Read-only", "New function", "Side-by-side", "Unified")
                  .forEach(fixture::assertTextFits)
              assertFalse(fixture.hasEditableText("Read-only composed diff"))
              assertTrue(fixture.hasDescription("Read-only composed diff"))
              val wide = fixture.hasText("Current")
              val column =
                  fixture.taggedBounds(
                      if (wide) "diff-Current-column" else "diff-Current → Candidate-column")
              assertTrue(
                  column.height > 180f,
                  "Comparison must keep usable canvas height at $width/$height/$scale: $column")
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
            val run = acceptanceRun(status)
            val presentation = projectRunPresentation(ProjectAnalysisRunState(run = run))
            if (presentation.headline != "Current run")
                fixture.assertTextFits(presentation.headline)
            else
                fixture.assertTextFits(
                    "${presentation.finishedFiles} of ${presentation.totalFiles} files finished")
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
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, navigations::add))
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
              {})
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
            if (width == 1440) {
              fixture.assertTextSharesRowBefore("Running", "0 of 2 files finished")
              fixture.assertTextSharesRowBefore(
                  "0 of 2 files finished", "Current: ${paths.first()}")
              fixture.assertTextSharesRowBefore("Current: ${paths.first()}", "Pause")
            } else {
              fixture.assertTextAbove("Current: ${paths.first()}", "Pause")
            }
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
            fixture.assertTextFits("Resume")
            fixture.clickText("Resume")
            assertEquals(1, resumes)
          }
    }
  }

  @Test
  fun unknownFileProgressKeepsOneTruthfulTrackAcrossActiveAndPausedRuns() {
    listOf(
            Triple("running", "Pause", "Resume"),
            Triple("paused", "Resume", "Pause"),
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
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}),
                )
              }
              .use { fixture ->
                fixture.render("analysis-progress-unknown-$status-800-1.5")
                fixture.assertTextFits(analysisStatusLabel(status))
                fixture.assertTextFits("File progress unavailable")
                fixture.assertTextFits(expectedAction)
                fixture.assertTextFits("Cancel")
                assertFalse(fixture.hasText(absentAction))
                assertFalse(fixture.hasText("0 of 0 files finished"))
                assertEquals(1, fixture.tagCount("analysis-run-progress-track"))
                assertTrue(fixture.hasDescription("Files finished: progress unavailable"))
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
                FindingActions({ externalActions++ }, { _, _ -> externalActions++ })
            ComposeVisualFixture(width, height, scale) {
                  when (category) {
                    "performance" ->
                        PerformanceWorkspacePane(
                            PerformanceWorkspacePaneState(page, resultIndexFixture()),
                            PerformanceWorkspaceActions(
                                { _, _ -> externalActions++ },
                                { navigation += Workspace.Analysis },
                                findingActions))
                    "security" ->
                        SecurityWorkspacePane(
                            SecurityWorkspacePaneState(page, resultIndexFixture()),
                            SecurityWorkspaceActions(
                                { externalActions++ },
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
                  assertEquals(
                      !resultListDetailUsesTwoPanes(width.dp, scale),
                      fixture.hasText("Back to results"))
                  assertFalse(fixture.hasText("Clear selection"))
                  assertTrue(fixture.hasText("Prepare fix"))
                  if (category == "performance") {
                    assertTrue(fixture.hasText("Model suggestion"))
                    assertTrue(
                        fixture.hasText(
                            "Unmeasured recommendation. Benchmark the affected workload before claiming an improvement."))
                  }
                  assertFalse(fixture.hasText("Open source"))
                  assertEquals(0, externalActions)
                  if (width < 900) {
                    fixture.clickText("Back to results")
                    fixture.render()
                    assertTrue(fixture.hasDescription("Inspect $title"))
                    fixture.clickDescription("Inspect $title")
                  }
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
  fun securityProductionRendersKeepEvidenceIdentityAndScopedEmptyStateDistinct() {
    val populated = securityPageFixture()
    var prepared = 0
    var openedAnalysis = 0
    ComposeVisualFixture(1440, 900) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(populated, resultIndexFixture()),
              SecurityWorkspaceActions(
                  { prepared++ }, { openedAnalysis++ }, FindingActions({}, { _, _ -> })))
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
                  { prepared++ }, { openedAnalysis++ }, FindingActions({}, { _, _ -> })))
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
                  { prepared++ }, { openedAnalysis++ }, FindingActions({}, { _, _ -> })))
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
              PerformanceWorkspaceActions({ _, _ -> }, {}, FindingActions({}, { _, _ -> })))
        }
        .use { fixture ->
          fixture.render("results-stale-error-800-1.25")
          assertFalse(fixture.hasText("Reported findings"))
          fixture.clickDescription("Inspect Avoid repeated allocation")
          fixture.render()
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertTrue(fixture.hasText("Stale"))
        }
    val empty = resultPageFixture("bugs").copy(run = null, section = AnalysisSectionState())
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(emptyList(), null, false, empty),
              BugsWorkspaceActions(FindingActions({}, { _, _ -> }), {}, {}))
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
  fun performanceBenchmarkStatusAndMeasurementDetailsRemainReadableAcrossLayouts() {
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
    listOf(Triple(1440, 900, 1f), Triple(800, 400, 1.5f)).forEach { (width, height, scale) ->
      ComposeVisualFixture(width, height, scale) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    performancePageFixture(),
                    resultIndexFixture(),
                    benchmarkComparison = comparison,
                    expectedBenchmarkIdentity = identity,
                    selectedBenchmark = choice),
                PerformanceWorkspaceActions({ _, _ -> }, {}, FindingActions({}, { _, _ -> })))
          }
          .use { fixture ->
            fixture.render("performance-benchmark-$width-$height-$scale")
            assertTrue(fixture.hasText("Measured · selected benchmark"))
            fixture.clickText("Benchmark evidence")
            fixture.render("performance-benchmark-expanded-$width-$height-$scale")
            assertTrue(
                fixture.hasText(
                    "Benchmark evidence is candidate-specific and does not measure this model suggestion."))
            fixture.clickText("Measurement details")
            fixture.render("performance-benchmark-details-$width-$height-$scale")
            fixture.assertTextFits("Benchmark evidence")
            assertTrue(fixture.hasText("Measured trade-offs"))
            assertTrue(fixture.hasText("BenchmarkRun"))
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
  fun filledWorkspacePanesKeepRoundedCornersGuttersAndFullWidthTerminal() {
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
                fixture.assertWorkspaceFrameGeometry(docked = width >= 1000)
                fixture.assertTextFits("Terminal")
                fixture.assertTextFits("Analysis · Completed")
                fixture.assertTextFits("user.go")
              }
        }
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
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render("summary-1440")
          assertTrue(fixture.hasText("Go · go.mod · 23 indexed files · 1,800 lines · Markdown"))
          assertTrue(fixture.hasText("Analysis coverage"))
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
  fun roundedAnalysisGroupsRunControlsAndShowsItsFileTableAtSupportedSizes() {
    listOf(1600 to 1000, 1440 to 900, 1000 to 760, 999 to 760, 800 to 650, 1280 to 600).forEach {
        (width, height) ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        ComposeVisualFixture(width, height, scale) { RoundedAnalysisVisualFixture(width.toFloat()) }
            .use { fixture ->
              fixture.render("analysis-frame-$width-$height-$scale")
              assertTrue(fixture.hasDescription("Analysis tool window, selected"))
              fixture.assertTextFits("Running")
              fixture.assertTextFits("8 of 12 files finished")
              fixture.assertTextFits("Current: internal/api/user.go")
              fixture.assertTextFits("Pause")
              fixture.assertTextFits("Cancel")
              if (width >= 1440 && scale == 1f) {
                fixture.assertAnalysisTableColumns()
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
                fixture.assertSummaryColumns()
                fixture.assertTextBefore("Bugs", "Performance")
                fixture.assertTextBefore("Performance", "Security")
              }
            }
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
                  "Analysis coverage",
                  visualFixtureProject.name,
                  "Architecture",
                  "Packages / modules",
                  "Flows",
                  "Engineering insight",
                  "Validate before storage.")
              .forEach { assertTrue(fixture.hasText(it), it) }
          assertTrue(fixture.hasText("Go · go.mod · 23 indexed files · 1,800 lines · Markdown"))
          assertTrue(
              fixture.hasText("Overall findings · 2 tool-reported issues · 4 AI suggestions"))
          fixture.assertTextAbove(visualFixtureProject.name, "Analysis coverage")
          fixture.assertSummaryStatusPlacement("Outdated")
          listOf("Bugs", "Performance", "Security").forEach {
            assertTrue(fixture.hasDescription("View $it results"))
            assertTrue(fixture.hasText(it))
          }
          fixture.assertSummaryColumns()
          assertFalse(fixture.hasText("Entry points"))
          assertFalse(fixture.hasText("Next steps"))
          assertFalse(fixture.hasText("cmd/server/main.go"))
          assertFalse(fixture.hasText("Review boundary validation."))
          assertTrue(
              fixture.hasDescription(
                  projectSummaryPresentation(overview, visualFixtureProject).analysisMessage))
          fixture.assertTextAbove("Mechanism", "Why it matters here")
          fixture.assertTextAbove("Trade-off or failure mode", "Transferable lesson")
          assertFalse(fixture.hasText("Type: Go · Build: go.mod · Languages: Go · Markdown"))
          assertTrue(fixture.hasText(visualFixtureProject.name))
          assertFalse(fixture.hasText("AI interpretation"))
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
              visualFixtureProject,
              {})
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("0 up to date"))
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
          ComposeVisualFixture(1280, 600) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
              .use { fixture ->
                fixture.render("summary-panel-$status")
                fixture.assertSummaryStatusPlacement(label)
                fixture.assertTextContrast(label, Panel)
                fixture.assertColorVisible(tint)
                fixture.assertColorVisible(Panel)
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
    ComposeVisualFixture(800, 650) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText(description))
          assertTrue(fixture.pressKey(Key.Tab))
          assertTrue(fixture.isFocused("View analysis"))
          assertTrue(fixture.pressKey(Key.Tab))
          fixture.render("summary-status-keyboard-focus")
          assertTrue(fixture.isDescriptionFocused(description))
          assertTrue(fixture.hasText(description))
          fixture.assertColorVisible(FocusAccent)
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
            analysis = StructuredProjectAnalysis(status = "fresh", purpose = "Current purpose"),
            analysisCoverage = AnalysisCoverage(total = 3, fresh = 3))
    ComposeVisualFixture(1440, 900) {
          ProjectSummaryPane(
              overview, resultProjectFixture(), navigations::add, run = run, sections = sections)
        }
        .use { fixture ->
          fixture.render("summary-live-running-1440")
          fixture.assertSummaryStatusPlacement("Updating")
          fixture.assertCategoryBoxesFit()
          listOf("17", "18", "19").forEach(fixture::assertTextFits)
          fixture.assertTextFits("Loading details")
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
          listOf("17", "18", "19", "Loading details").forEach { assertFalse(fixture.hasText(it)) }
          listOf("27", "28", "29").forEach(fixture::assertTextFits)
          fixture.assertSummaryStatusPlacement("Updated")
          fixture.assertCategoryBoxesFit()
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
          assertTrue(fixture.hasDescription("Bugs"))
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
            fixture.assertSummaryStatusPlacement("Paused")
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
  fun validDiagramDisclosureAcceptsTheFirstClickWhileRenderingStarts() {
    val source = "flowchart TD\n A[Client] --> B[Server]"
    ComposeVisualFixture(800, 650, 1.5f) {
          MermaidDiagram(source, "Architecture", title = "Architecture")
        }
        .use { fixture ->
          fixture.render("summary-mermaid-first-frame")
          assertTrue(
              fixture.tryClick("Show diagram"),
              "A valid diagram must not ignore the first disclosure click while rendering")
          fixture.awaitDescription("Architecture diagram\n$source")
          assertEquals("Expanded", fixture.stateDescription("Hide diagram"))
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
            // State semantics can observe the render result before enabled recomposes.
            fixture.render()
            assertTrue(fixture.isDisabled("Show diagram"))
            assertTrue(fixture.hasText(value))
            assertFalse(fixture.tryClick("Show diagram"))

            value = "sequenceDiagram\n Client->>API: Retry\n API-->>Client: Ready"
            fixture.render()
            assertTrue(fixture.tryClick("Show diagram"))
            fixture.awaitDescription("$label diagram\n$value")
            assertEquals("Expanded", fixture.stateDescription("Hide diagram"))
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
    listOf(1440 to 900, 1000 to 650, 999 to 650, 800 to 650, 1280 to 600).forEach { (width, height)
      ->
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
                      triageFinding = { _, _ -> workflowActions++ }),
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
    var mutations = 0
    ComposeVisualFixture(900, 500) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(visualFixtureFindings, null, false),
              BugsWorkspaceActions(
                  FindingActions({ mutations++ }, { _, _ -> mutations++ }), {}, {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Validate the user identifier")
          fixture.render()
          kotlin.test.assertEquals(0, mutations)
          assertFalse(fixture.hasText("Open source"))
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

/** This test-only adapter is tied to the Compose version pinned in build.gradle.kts. */
@OptIn(ExperimentalComposeUiApi::class, InternalComposeUiApi::class)
internal class ComposeVisualFixture(
    private val width: Int,
    private val height: Int,
    fontScale: Float = 1f,
    densityScale: Float = 1f,
    content: @Composable () -> Unit,
) : AutoCloseable {
  private val clipboard =
      object : Clipboard {
        override val nativeClipboard = java.awt.datatransfer.Clipboard("visual-test")

        override suspend fun getClipEntry(): ClipEntry? =
            nativeClipboard.getContents(null)?.let(::ClipEntry)

        override suspend fun setClipEntry(clipEntry: ClipEntry?) {
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
  private val surface = Surface.makeRasterN32Premul(width, height)
  private var frameTime = 0L

  init {
    scene.setContent {
      CompositionLocalProvider(LocalClipboard provides clipboard) { MiniOrcaTheme { content() } }
    }
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

  fun awaitVisibleDescription(label: String) {
    fun visible(): Boolean =
        nodes().any { node ->
          node.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true &&
              node.config.getOrNull(SemanticsActions.OnClick) != null &&
              node.boundsInRoot.let { bounds ->
                bounds.width > 0 &&
                    bounds.height > 0 &&
                    bounds.left >= 0 &&
                    bounds.top >= 0 &&
                    bounds.right <= width &&
                    bounds.bottom <= height
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

  fun assertCategoryBoxesFit() {
    val bounds =
        AnalysisResultType.entries.map { visibleActionBounds("View ${it.workspace.name} results") }
    bounds.forEach {
      assertEquals(bounds.first().width, it.width, 1f, "Category widths must match")
      assertEquals(bounds.first().height, it.height, 1f, "Category heights must match")
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

  fun requestDescriptionFocus(label: String): Boolean =
      nodes()
          .asSequence()
          .filter {
            it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
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

  fun taggedBounds(tag: String): Rect = taggedNode(tag).boundsInRoot

  fun tagCount(tag: String): Int =
      nodes().count { it.config.getOrNull(SemanticsProperties.TestTag) == tag }

  private fun taggedNode(tag: String): SemanticsNode =
      nodes().single { it.config.getOrNull(SemanticsProperties.TestTag) == tag }

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

  fun copyTextByDragging(label: String): String {
    val bounds = textNodes(label).first().boundsInRoot
    val start = Offset(bounds.left + 1f, bounds.top + 10f)
    val end = Offset(minOf(bounds.right - 2f, bounds.left + 120f), start.y)
    scene.sendPointerEvent(PointerEventType.Press, start, button = PointerButton.Primary)
    scene.sendPointerEvent(PointerEventType.Move, (start + end) / 2f)
    scene.sendPointerEvent(PointerEventType.Move, end)
    scene.sendPointerEvent(PointerEventType.Release, end, button = PointerButton.Primary)
    render()
    pressKey(Key.Copy)
    render()
    return clipboard.nativeClipboard.getData(DataFlavor.stringFlavor) as String
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
    repeat(100) {
      if (visible()) return
      scrollBy(160f, scrollTag)
      render()
    }
    error("$label must be reachable by scrolling")
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

  fun assertSummaryColumns() {
    fun bounds(tag: String) =
        nodes().single { it.config.getOrNull(SemanticsProperties.TestTag) == tag }.boundsInRoot
    val architecture = bounds("summary-architecture")
    val flows = bounds("summary-flows")
    val modules = bounds("summary-modules")
    val insight = bounds("summary-insight")
    assertEquals(architecture.width, flows.width, 1f)
    assertTrue(architecture.right < flows.left)
    assertEquals(architecture.top, flows.top, 1f)
    assertTrue(modules.top >= maxOf(architecture.bottom, flows.bottom) + 16f)
    assertEquals(modules.top, insight.top, 1f)
    assertEquals(architecture.left, modules.left, 1f)
    assertEquals(flows.left, insight.left, 1f)
  }

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

  fun assertAnalysisTableColumns() {
    val headers =
        listOf("File", "Analysis state", "Details").map { label ->
          textNodes(label).minBy { it.boundsInRoot.top }.boundsInRoot
        }
    headers.zipWithNext().forEach { (first, second) ->
      assertTrue(first.right < second.left)
      assertEquals(first.center.y, second.center.y, 2f)
    }
  }

  fun assertTextBefore(label: String, following: String) {
    val first = textNodes(label).single().boundsInRoot
    val second = textNodes(following).single().boundsInRoot
    assertTrue(first.right < second.left, "$label must be left of $following")
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
    assertEquals(56f, panes.first().left, 1f, "The rail and outer inset must stay visible")
    assertEquals(width - 8f, panes.last().right, 1f, "The trailing frame must stay visible")
    panes.forEach { pane ->
      assertEquals(editor.top, pane.top, 1f)
      assertEquals(editor.bottom, pane.bottom, 1f, "All panes must end above the terminal")
    }
    panes.zipWithNext().forEach { (left, right) ->
      assertEquals(8f, right.left - left.right, 1f, "A single gutter separates adjacent panes")
    }
    if (docked)
        assertTrue(editor.width >= MIN_EDITOR_WIDTH, "The editor must keep its minimum width")
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
      listOf(pane.left + 1, pane.right - 2).forEach { x ->
        listOf(pane.top + 1, pane.bottom - 2).forEach { y ->
          assertEquals(
              ActivityRail.toArgb(),
              rendered.getRGB(x.toInt(), y.toInt()),
              "Filled children must be clipped to the rounded pane at $x,$y")
        }
      }
      assertFalse(
          ActivityRail.toArgb() == rendered.getRGB(pane.center.x.toInt(), (pane.top + 2).toInt()),
          "The top edge must show the actual filled pane")
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
                    files = paths.map { AnalysisPlannedFile(it, "base", "Go", 20, emptyList()) }),
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
            width,
            project,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            false,
            toolbarAnalysisStatus(
                DesktopState(
                    projectState = ProjectWorkspaceState(project = project),
                    analysisRun = analysis))),
        ToolbarActions({}, {}, {}, {}, {}, {}))
    WorkspaceFrame(
        rail = { ToolWindowBar(LeftToolWindow.Analysis, {}) },
        panes = {
          EditorArea(
              {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(project, analysis),
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
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
            width,
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            false,
            toolbarAnalysisStatus(
                DesktopState(
                    projectState = ProjectWorkspaceState(project = visualFixtureProject),
                    analysisRun = ProjectAnalysisRunState(run = overview.analysisRun)))),
        ToolbarActions({}, {}, {}, {}, {}, {}))
    WorkspaceFrame(
        rail = { ToolWindowBar(LeftToolWindow.Summary, {}) },
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
    comparison: Boolean = false
) {
  val layout = DesktopLayoutState(bottomCollapsed = !terminalExpanded)
  val panes = dockedPaneWidths(width, layout.explorerWidth, layout.actionWidth)
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
            width,
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            useNarrowLayout(width),
            ToolbarAnalysisStatus(
                "Analysis · Completed", "Whole-project analysis · Completed", false, false)),
        ToolbarActions({}, {}, {}, {}, {}, {}))
    WorkspaceFrame(
        rail = { ToolWindowBar(LeftToolWindow.Editor, {}) },
        panes = {
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
          if (!useNarrowLayout(width)) {
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
                modifier = Modifier.width(panes.action.dp),
                showHeader = false)
          }
        },
        terminal = {
          if (useNarrowLayout(width)) {
            TerminalBar(
                TerminalWorkspaceState(),
                true,
                {},
                TerminalTabActions({}, {}, {}),
                modifier = Modifier.semantics { contentDescription = "Terminal fixture" })
          } else {
            TerminalDock(
                layout,
                TerminalWorkspaceState(),
                {},
                {},
                TerminalTabActions({}, {}, {}),
                {},
                {},
                { modifier -> Text("Synthetic shell", modifier = modifier.padding(8.dp)) },
                modifier = Modifier.semantics { contentDescription = "Terminal fixture" })
          }
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
