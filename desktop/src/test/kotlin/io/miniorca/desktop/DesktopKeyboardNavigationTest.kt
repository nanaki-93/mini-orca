package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopKeyboardNavigationTest {
  @Test
  fun paletteShortcutsKeepFilesAndSymbolsDirectWhileCommandKFocusesChat() {
    assertEquals(DesktopShortcut.OpenFile, desktopShortcut("P", primaryModifier = true))
    assertEquals(
        DesktopShortcut.OpenSymbol, desktopShortcut("O", primaryModifier = true, shift = true))
    assertEquals(DesktopShortcut.FocusChat, desktopShortcut("K", primaryModifier = true))
  }

  @Test
  fun resultNavigationAndFixPreparationStaySeparateFromKeyboardInspection() {
    var destination: Workspace? = null
    var preparations = 0
    val page = performancePageFixture()
    ComposeVisualFixture(1_000, 760, 1.25f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(page, resultIndexFixture()),
              PerformanceWorkspaceActions(
                  prepareOptimization = { _, _ -> preparations++ },
                  openAnalysis = { destination = Workspace.Analysis },
                  semanticActions = FindingActions({}, { _, _ -> })))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("View analysis"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(Workspace.Analysis, destination)
          assertEquals(0, preparations)

          assertTrue(fixture.requestDescriptionFocus("Inspect Avoid repeated allocation"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasText("Prepare fix"))
          assertEquals(0, preparations)

          assertTrue(fixture.requestFocus("Prepare fix"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, preparations)
        }
  }

  @Test
  fun analysisKeyboardControlsNavigateOrUpdateOnlyLocalPresentationState() {
    val navigations = mutableListOf<Workspace>()
    var starts = 0
    var pauses = 0
    var resumes = 0
    var cancels = 0
    var refreshes = 0
    var saves = 0
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                files =
                    listOf(
                        AnalysisRunFile(
                            "helper.go",
                            "helper",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false))),
                        AnalysisRunFile(
                            "main.go",
                            "main",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false))),
                    ),
                sections = analysisRunFixture().sections.map { it.copy(status = "running") },
            )
    val actions =
        AnalysisWorkspaceActions(
            start = { _, _ -> starts++ },
            pause = { pauses++ },
            resume = { resumes++ },
            cancel = { cancels++ },
            openResults = { navigations += it },
            refreshSelection = { refreshes++ },
            saveSelection = { saves++ },
        )
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(
                      run = run, fileSelection = AnalysisSelectionState(selectionFixture()))),
              actions,
          )
        }
        .use { fixture ->
          fixture.render()
          AnalysisResultType.entries.forEach { type ->
            assertTrue(fixture.requestDescriptionFocus("View ${type.workspace.name} results"))
            assertTrue(fixture.pressKey(Key.Enter))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
          }
          assertEquals(
              AnalysisResultType.entries.flatMap { listOf(it.workspace, it.workspace) },
              navigations)

          assertTrue(fixture.requestDescriptionFocus("Show active files"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasDescription("Hide active files"))

          assertTrue(fixture.requestDescriptionFocus("Collapse Files"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("Collapsed", fixture.stateDescription("Files"))
          assertTrue(fixture.requestDescriptionFocus("Expand Files"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Expanded", fixture.stateDescription("Files"))

          assertTrue(fixture.requestDescriptionFocus("Filter files"))
          fixture.setFocusedText("helper.go")
          fixture.render()
          assertTrue(fixture.hasText("helper.go"))
          assertTrue(fixture.requestDescriptionFocus("Needs attention"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Needs attention"))

          assertEquals(0, starts)
          assertEquals(0, pauses)
          assertEquals(0, resumes)
          assertEquals(0, cancels)
          assertEquals(0, refreshes)
          assertEquals(0, saves)
        }
  }

  @Test
  fun analysisKeyboardRunSelectionAndDetailsControlsRemainIndependent() {
    var pauses = 0
    var resumes = 0
    var cancels = 0
    val runActions =
        AnalysisWorkspaceActions(
            start = { _, _ -> },
            pause = { pauses++ },
            resume = { resumes++ },
            cancel = { cancels++ },
            openResults = {},
        )
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = "running"))),
              runActions,
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Pause"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertTrue(fixture.requestFocus("Cancel"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, pauses)
          assertEquals(1, cancels)
        }

    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = "paused"))),
              runActions,
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Resume"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, resumes)
        }

    val selectionWithDetails =
        selectionFixture()
            .copy(
                files =
                    selectionFixture().files.map { file ->
                      if (file.path == "main.go")
                          file.copy(
                              stages =
                                  listOf(
                                      AnalysisFileStageStatus(
                                          "semantic", "missing", "Semantic results are missing."),
                                      AnalysisFileStageStatus(
                                          "performance",
                                          "fresh",
                                          "Performance results are current."),
                                  ))
                      else file
                    })
    var saves = 0
    ComposeVisualFixture(1_440, 900) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionWithDetails)),
              AnalysisWorkspaceActions(
                  start = { _, _ -> },
                  pause = {},
                  resume = {},
                  cancel = {},
                  openResults = {},
                  saveSelection = { saves++ },
              ),
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Analyze main.go"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, saves)
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(
              "Expanded", fixture.descriptionStateDescription("Analysis details for main.go"))
          assertEquals(1, saves, "Opening details must not update selection")
        }

    listOf(
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "running"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "paused"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "interrupted"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                fileSelection = AnalysisSelectionState(selectionWithDetails, saving = true)),
            ProjectAnalysisRunState(
                action = "pausing", fileSelection = AnalysisSelectionState(selectionWithDetails)),
        )
        .forEach { analysis ->
          ComposeVisualFixture(1_440, 900) {
                AnalysisFileSelector(
                    analysis,
                    AnalysisWorkspaceActions(
                        start = { _, _ -> },
                        pause = {},
                        resume = {},
                        cancel = {},
                        openResults = {},
                        saveSelection = { saves++ },
                    ),
                )
              }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.isDescriptionDisabled("Analyze main.go"))
              }
        }
  }

  @Test
  fun unavailableArchitecturePreviewIsNotKeyboardActivatable() {
    listOf(1_600 to 1_000, 800 to 650).forEach { (width, height) ->
      ComposeVisualFixture(width, height) {
            ProjectSummaryPane(
                visualFixtureOverview.copy(
                    analysis = visualFixtureOverview.analysis.copy(status = "missing")),
                visualFixtureProject,
                {})
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasDescription("Select Architecture summary from preview"))
            assertTrue(fixture.isDescriptionDisabled("Select Architecture summary from preview"))
            assertFalse(fixture.requestDescriptionFocus("Select Architecture summary from preview"))
          }
    }
  }

  @Test
  fun summaryKeyboardControlsKeepDisclosuresLocalAndNavigateOnlyToTheirWorkspace() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate requests before persistence.",
                            whyItMattersHere = "Invalid input stays outside the repository.",
                            tradeoffOrFailureMode = "Rules need one owner.")))
    listOf(1_600 to 1_000, 800 to 650).forEach { (width, height) ->
      val destinations = mutableListOf<Workspace>()
      var analysisCalls = 0
      val actions =
          AnalysisWorkspaceActions(
              start = { _, _ -> analysisCalls++ },
              pause = { analysisCalls++ },
              resume = { analysisCalls++ },
              cancel = { analysisCalls++ },
              openResults = { analysisCalls++ },
              refreshSelection = { analysisCalls++ },
              saveSelection = { analysisCalls++ })
      ComposeVisualFixture(width, height) {
            ProjectSummaryPane(
                overview, visualFixtureProject, destinations::add, analysisActions = actions)
          }
          .use { fixture ->
            fixture.render()
            assertEquals(emptyList(), destinations)
            assertEquals(0, analysisCalls)
            assertTrue(fixture.requestDescriptionFocus("Select Flows summary"))
            assertTrue(fixture.pressKey(Key.Enter), "Outline must support keyboard activation")
            fixture.render()
            assertTrue(fixture.isDescriptionSelected("Select Flows summary"))
            assertEquals(emptyList(), destinations, "Flows selection must remain local")
            assertEquals(0, analysisCalls)
            assertTrue(fixture.requestDescriptionFocus("Select Coverage summary"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(emptyList(), destinations, "Coverage selection must remain local")
            assertEquals(0, analysisCalls)
            fixture.revealText("View analysis")
            assertTrue(fixture.requestFocus("View analysis"))
            fixture.render()
            assertTrue(fixture.isFocusedControl("View analysis"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(listOf(Workspace.Analysis), destinations)
            assertEquals(0, analysisCalls)
            val expectedDestinations =
                listOf(
                    Workspace.Analysis, Workspace.Bugs, Workspace.Performance, Workspace.Security)
            listOf("Bugs", "Performance", "Security").forEachIndexed { index, category ->
              fixture.revealSummaryCategory(category)
              val control = "View $category results"
              assertTrue(fixture.requestDescriptionFocus(control))
              fixture.render()
              assertTrue(fixture.isFocusedControl(control))
              assertTrue(fixture.pressKey(Key.Spacebar))
              fixture.render()
              assertEquals(expectedDestinations.take(index + 2), destinations)
              assertEquals(0, analysisCalls)
            }
            assertEquals(expectedDestinations, destinations)
            assertTrue(fixture.requestDescriptionFocus("Select Engineering insight summary"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(expectedDestinations, destinations, "Insight selection must remain local")
            assertEquals(0, analysisCalls)
            val insightDisclosure = "Expand More insight"
            assertTrue(fixture.requestDescriptionFocus(insightDisclosure))
            fixture.render()
            assertTrue(fixture.isFocusedControl(insightDisclosure))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals("Expanded", fixture.descriptionStateDescription("Collapse More insight"))
            assertEquals(expectedDestinations, destinations, "Disclosures must remain local")
            assertEquals(0, analysisCalls)
            assertTrue(fixture.requestDescriptionFocus("Select Architecture summary"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(
                expectedDestinations, destinations, "Architecture selection must remain local")
            assertEquals(0, analysisCalls)
            fixture.awaitDescription("Show Architecture diagram", "Collapsed")
            assertTrue(fixture.requestDescriptionFocus("Show Architecture diagram"))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Show Architecture diagram"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.awaitDescription("Hide Architecture diagram", "Expanded")
            assertEquals(expectedDestinations, destinations, "Diagram must remain local")
            assertEquals(0, analysisCalls)
            assertTrue(fixture.requestDescriptionFocus("Select Coverage summary"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(expectedDestinations, destinations, "Section selection must not navigate")
            assertEquals(0, analysisCalls)
          }
    }
  }

  @Test
  fun terminalInputOwnsInterruptAndOrdinaryAppChordsUntilExplicitFocusReturn() {
    assertFalse(appShortcutAllowed(terminalFocused = true))
    assertTrue(appShortcutAllowed(terminalFocused = false))
    assertFalse(terminalReturnShortcut(java.awt.event.KeyEvent.VK_C, control = true, shift = false))
    assertFalse(
        terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = false, shift = true))
    assertFalse(
        terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = true, shift = false))
    assertTrue(terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = true, shift = true))
  }

  @Test
  fun iconRailKeepsAccessibleNamesAndKeyboardReachabilityAtEverySupportedSize() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1280, 600, 1.25f),
            Triple(1280, 600, 1.5f))
        .forEach { (width, height, scale) ->
          var active by mutableStateOf(LeftToolWindow.Summary)
          var selections = 0
          val focus = FocusRequester()
          ComposeVisualFixture(width, height, scale) {
                Row(Modifier.fillMaxSize()) {
                  ToolWindowBar(
                      active,
                      {
                        active = it
                        selections++
                      },
                      Modifier.focusRequester(focus))
                  Box(Modifier.weight(1f))
                }
              }
              .use { fixture ->
                fixture.render("navigation-icons-$width-$scale")
                assertFalse(fixture.hasText("Perf."))
                LeftToolWindow.entries.forEach {
                  assertFalse(fixture.hasText(leftToolWindowLabel(it)))
                  assertTrue(fixture.hasDescription(toolWindowSemanticsLabel(it, it == active)))
                }
                listOf(
                        "Project",
                        "Results",
                        "Editing",
                        "Run & progress",
                        "findings",
                        "Running",
                        "Completed")
                    .forEach { assertFalse(fixture.hasText(it)) }
                assertEquals(null, fixture.stateDescription("Performance"))
                focus.requestFocus()
                fixture.render()
                repeat(5) {
                  fixture.pressKey(Key.DirectionDown)
                  fixture.render()
                }
                fixture.render("navigation-editor-focused-$width-$scale")
                assertEquals(0, selections)
                assertEquals(LeftToolWindow.Summary, active)
                assertTrue(fixture.hasDescription("Editor tool window, not selected, focused"))
                assertFalse(fixture.hasText("Editor"))
                fixture.pressKey(Key.Enter)
                fixture.render()
                assertEquals(LeftToolWindow.Editor, active)
                assertEquals(1, selections)
                assertTrue(fixture.hasDescription("Editor tool window, selected, focused"))
              }
        }
  }

  @Test
  fun responsiveShellKeepsTheExactBreakpointDocked() {
    assertEquals(
        ResponsiveShellPresentation(
            left = ResponsiveShellRegion.Docked,
            right = ResponsiveShellRegion.Docked,
            bottom = ResponsiveShellRegion.Docked,
        ),
        responsiveShellPresentation(1_000f),
    )
    assertEquals(
        ResponsiveShellPresentation(
            left = ResponsiveShellRegion.Drawer,
            right = ResponsiveShellRegion.Drawer,
            bottom = ResponsiveShellRegion.Overlay,
        ),
        responsiveShellPresentation(999f),
    )
  }

  @Test
  fun tabGroupArrowsMoveFocusWithoutActivatingTheDestination() {
    val entries = listOf("Source", "Review", "History")

    assertEquals(
        TabGroupInteraction("History"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Previous),
    )
    assertEquals(
        TabGroupInteraction("Review"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Next),
    )
    assertEquals(
        TabGroupInteraction("Source", "Source"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Activate),
    )
  }

  @Test
  fun activityRailCyclesTheSixRetainedWorkspaces() {
    val entries = LeftToolWindow.entries.toList()

    assertEquals(
        LeftToolWindow.Analysis,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Next)?.focused)
    assertEquals(
        LeftToolWindow.Editor,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Previous)?.focused)
    assertEquals(
        TabGroupInteraction(LeftToolWindow.Editor, LeftToolWindow.Editor),
        tabGroupInteraction(entries, LeftToolWindow.Editor, TabGroupKey.Activate),
    )
  }

  @Test
  fun escapeSelectsOnlyTheTopmostTransientSurface() {
    assertEquals(
        TransientSurface.Context,
        topmostTransientSurface(
            contextVisible = true,
            paletteVisible = true,
            statusDetailsVisible = true,
            bottomToolsVisible = true,
            drawerVisible = true,
        ),
    )
    assertEquals(
        TransientSurface.BottomTools,
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = false,
            bottomToolsVisible = true,
            drawerVisible = true,
        ),
    )
    assertNull(
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = false,
            bottomToolsVisible = false,
            drawerVisible = false,
        ),
    )
  }

  @Test
  fun leavingEditorClosesOnlyAnEditorDrawer() {
    assertEquals(true, closesEditorDrawerOnWorkspaceChange(Workspace.Editor, Workspace.Bugs))
    assertEquals(false, closesEditorDrawerOnWorkspaceChange(Workspace.Editor, Workspace.Editor))
    assertEquals(false, closesEditorDrawerOnWorkspaceChange(Workspace.Analysis, Workspace.Bugs))
  }

  @Test
  fun focusedTabsKeepTextualAccessibilityStateAtCompactWidths() {
    assertEquals(
        "Editor tool window, selected, focused",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true, focused = true),
    )
    assertEquals(
        "Context tool window tab, not selected, focused",
        rightToolWindowTabDescription(
            RightToolWindow.Context,
            selected = false,
            focused = true,
        ),
    )
  }

  @Test
  fun compactToolbarAndBreadcrumbPoliciesKeepLongTextBounded() {
    assertEquals(ToolbarPresentation(false, false, false), toolbarPresentation(999f))
    assertEquals(ToolbarPresentation(true, false, true), toolbarPresentation(1_000f))
    assertEquals(
        listOf("very", "…", "main.go", "Run"),
        editorBreadcrumbSegments("very/long/project/path/main.go", "Run").map { it.label },
    )
  }
}
