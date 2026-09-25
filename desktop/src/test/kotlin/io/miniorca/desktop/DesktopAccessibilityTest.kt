package io.miniorca.desktop

import androidx.compose.ui.input.key.Key
import androidx.compose.ui.state.ToggleableState
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopAccessibilityTest {
  @Test
  fun savedReadFailureNamesRecoveryAndKeepsFilteredFailureAccessible() {
    AnalysisResultType.entries.forEach { type ->
      val page =
          resultPageFixture(type.category).let {
            it.copy(section = it.section.copy(error = "saved read failed"))
          }
      val browser = newResultBrowserState(page)
      browser.query = "no matching result"
      val rows =
          listOf(
              ResultRowPresentation(
                  "retained", "Retained result", "main.go:1", "Evidence", "high", "", ""))
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisResultsPane(page, rows, browser, openAnalysis = {}, retryResults = {}) {
              androidx.compose.material.Text("Retained detail")
            }
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("No matching results."))
            assertTrue(fixture.hasText("Results could not be refreshed: saved read failed"))
            assertTrue(
                fixture.hasText(
                    "Reloads saved results for ${type.workspace.name}; does not start analysis."))
            assertTrue(fixture.hasText("Retry loading results"))
            assertTrue(fixture.hasDescription("Clear filters"))
          }
    }
  }

  @Test
  fun reviewAttemptRetainsFailureAndDisabledReasonWithHelpCollapsed() {
    val base = editorComparisonReviewFixture()
    val draft = requireNotNull(base.draft)
    val running =
        base.copy(
            checkAttempt = CheckAttempt(6, CheckCandidate(draft), ValidationAttemptStatus.Running))
    ComposeVisualFixture(800, 650, 1.5f) {
          ReviewToolWindow(
              running, ReviewToolWindowActions({}, {}, {}), DraftApplicationActions({}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Checks running"))
          assertTrue(fixture.hasText("Previous check report (retained; not current approval)"))
          assertFalse(fixture.hasText("Ready to apply"))
          assertFalse(fixture.hasText("Apply change"))
        }
  }

  @Test
  fun resultPagesStartKeyboardNavigationAtViewAnalysis() {
    AnalysisResultType.entries.forEach { current ->
      ComposeVisualFixture(1_000, 760, 1.25f) { AcceptanceResultPane(current.category, "partial") }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocused("View analysis"))
          }
    }
  }

  @Test
  fun summaryCategoriesExposeOneNamedActionAndDecorativeIcons() {
    ComposeVisualFixture(1_600, 1_000) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render()
          AnalysisResultType.entries.forEach { type ->
            val action = "View ${type.workspace.name} results"
            assertEquals(
                1,
                fixture.clickableDescriptionCount(action),
                "${type.workspace.name} must expose exactly one navigation action")
            assertTrue(
                fixture.tagIsDecorative("summary-category-icon-${type.category}"),
                "${type.workspace.name} icon must not add a second announcement")
          }
        }
  }

  @Test
  fun summaryExposesNamedLocalDisclosuresAndVisibleNonColorInterpretationState() {
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    status = "stale",
                    failure = "The saved project description is from an older revision.",
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate requests before persistence.",
                            whyItMattersHere = "Invalid input stays outside the repository.",
                            tradeoffOrFailureMode = "Rules need one owner.")))
    ComposeVisualFixture(1_600, 1_000) { ProjectSummaryPane(overview, visualFixtureProject, {}) }
        .use { fixture ->
          fixture.render()
          assertEquals(1, fixture.textCount("Summary"))
          assertEquals(
              listOf(
                  "Summary",
                  "go-shop · fixture",
                  "Analysis coverage",
                  "Architecture",
                  "Engineering insight",
                  "More insight",
                  "Flows"),
              fixture.semanticHeadingTexts(),
              "Summary headings must expose the page and its sections in reading order")
          assertTrue(fixture.hasText("Project description: stale · source may have changed"))
          assertTrue(fixture.hasText("Outdated"))
          assertTrue(fixture.hasText("View analysis"))
          fixture.awaitDescription("Show Architecture diagram", "Collapsed")
          fixture.awaitDescription("Show Flow 1 diagram", "Collapsed")
          assertEquals("Collapsed", fixture.descriptionStateDescription("Expand More insight"))
        }
  }

  @Test
  fun soleTerminalControlAnnouncesItsStateAndActivatesFromTheKeyboard() {
    var opens = 0
    ComposeVisualFixture(800, 100, 1.5f) {
          TerminalBar(
              TerminalWorkspaceState(),
              collapsed = true,
              onToggle = { opens++ },
              tabActions = TerminalTabActions({}, {}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Open terminal · Ctrl+Shift+T"))
          assertEquals("Collapsed", fixture.stateDescription("Terminal"))
          assertEquals(0, opens)
          assertTrue(fixture.requestFocus("Terminal"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, opens)
          listOf("Bugs & Problems", "Checks", "Output").forEach { assertFalse(fixture.hasText(it)) }
        }
  }

  @Test
  fun packageOnlyCreationHasAnExplicitNameAndKeepsSourceReadOnly() {
    val file =
        ProjectFileInfo(
            "empty.go",
            "base",
            "empty.go",
            language = "Go",
            sizeBytes = 13,
            lineCount = 1,
            modifiedAt = "",
            binary = false,
            content = "package main\n")
    var requests = 0
    ComposeVisualFixture(800, 100, 1.5f) {
          NewFunctionButton(file.path, declarationCreationBlockedReason(file), { requests++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("New function in empty.go"))
          assertTrue(fixture.requestFocus("New function"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, requests)
          assertEquals("package main\n", file.content)
        }
  }

  @Test
  fun analysisFilesControlsExposeDisclosureFilterAndLockedSelectionStates() {
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
                            listOf(AnalysisStageProgress("semantic", "running", 1, false)))),
                sections = analysisRunFixture().sections.map { it.copy(status = "running") },
            )
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(
                      run = run, fileSelection = AnalysisSelectionState(selectionFixture()))),
              AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}),
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Files"))
          assertEquals("Expanded", fixture.stateDescription("Files"))
          assertTrue(fixture.hasDescription("Filter files"))
          assertTrue(fixture.isDescriptionSelected("All"))
          assertTrue(fixture.hasDescription("Analyze helper.go"))
          assertTrue(fixture.isDescriptionDisabled("Analyze helper.go"))
          assertTrue(fixture.hasText("Up to date"))
          assertEquals(
              "Selected for analysis", fixture.descriptionStateDescription("Analyze helper.go"))
          assertEquals(ToggleableState.On, fixture.descriptionToggleableState("Analyze helper.go"))
          assertTrue(fixture.hasDescription("Files finished: 0 of 1"))
          assertTrue(
              fixture.hasText(
                  "Selection locked. Finish or cancel the current run to change files."))
        }
  }

  @Test
  fun analysisFileStatusMarkersAreDecorativeAtFullSize() {
    ComposeVisualFixture(1_600, 1_000, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionFixture())),
              AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render()
          listOf(".env", "helper.go", "main.go").forEach { path ->
            assertTrue(
                fixture.tagIsDecorative("analysis-file-status-marker-$path"),
                "$path status marker must not add accessible meaning")
          }
        }
  }

  @Test
  fun analysisStageFailureRetainsItsRecoveryActionAndReadableDiagnostic() {
    val reason =
        "Semantic analysis failed because the local scanner is unavailable. Start analysis to retry."
    val run =
        analysisRunFixture()
            .copy(
                status = "failed",
                files =
                    listOf(
                        AnalysisRunFile(
                            "cmd/miniorca/main.go",
                            "base",
                            "Go",
                            listOf(
                                AnalysisStageProgress(
                                    "semantic", "failed", 1, false, reason = reason)))))
    var starts = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(), ProjectAnalysisRunState(run = run)),
              AnalysisWorkspaceActions({ _, _ -> starts++ }, {}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Start analysis"))
          fixture.clickText("Start analysis")
          assertEquals(1, starts)
          fixture.revealText(reason, "analysis-page")
          fixture.assertTextWrapsWithoutClipping(reason)
        }
  }

  @Test
  fun keyboardShortcutsCoverFocusedWorkflowWithoutMouse() {
    assertEquals(DesktopShortcut.OpenFile, desktopShortcut("P", primaryModifier = true))
    assertEquals(DesktopShortcut.OpenProject, desktopShortcut("O", primaryModifier = true))
    assertEquals(
        DesktopShortcut.OpenSymbol, desktopShortcut("O", primaryModifier = true, shift = true))
    assertEquals(DesktopShortcut.FocusChat, desktopShortcut("K", primaryModifier = true))
    assertEquals(DesktopShortcut.Generate, desktopShortcut("Enter", primaryModifier = true))
    assertEquals(DesktopShortcut.Cancel, desktopShortcut("Escape", primaryModifier = false))
    assertEquals(DesktopShortcut.NextTab, desktopShortcut("Tab", primaryModifier = true))
    assertNull(desktopShortcut("O", primaryModifier = false))
  }

  @Test
  fun keyboardRoutesCoverWorkspacesBugsDraftValidationAndChecks() {
    assertEquals(DesktopShortcut.SummaryWorkspace, desktopShortcut("1", primaryModifier = true))
    assertEquals(DesktopShortcut.AnalysisWorkspace, desktopShortcut("2", primaryModifier = true))
    assertEquals(DesktopShortcut.BugsWorkspace, desktopShortcut("3", primaryModifier = true))
    assertEquals(DesktopShortcut.EditorWorkspace, desktopShortcut("4", primaryModifier = true))
    assertNull(desktopShortcut("F", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.FocusDraft, desktopShortcut("D", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.ValidateDraft, desktopShortcut("V", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.RunDraftChecks, desktopShortcut("C", primaryModifier = true, shift = true))
  }

  @Test
  fun toolWindowSemanticsKeepTextualSelectedStateWithoutCounters() {
    assertEquals(
        "Editor tool window, selected", toolWindowSemanticsLabel(LeftToolWindow.Editor, true))
    assertEquals(
        "Bugs tool window, not selected", toolWindowSemanticsLabel(LeftToolWindow.Problems, false))
  }

  @Test
  fun retainedNavigationStatesDescribeSelectionAndFocusSeparately() {
    assertEquals(
        "Editor tool window, selected, focused",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true, focused = true),
    )
    assertEquals(
        "Editor tool window, selected",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true),
    )
  }

  @Test
  fun editorChromeExposesTheFullSelectedFileIdentityAndReadOnlySurface() {
    val chrome =
        editorChromeUiState(
            file =
                ProjectFileInfo(
                    path = "cmd/miniorca/main.go",
                    contentHash = "hash",
                    name = "main.go",
                    language = "Go",
                    sizeBytes = 0,
                    lineCount = 0,
                    modifiedAt = "",
                    binary = false,
                ),
            selectedSymbol =
                SymbolInfo("Main", "function", confidence = "exact", atomicTarget = true),
            requestedSurface = EditorSurface.Source,
            progress = EditorProgressUiState(EditorProgress.Inspect, ""),
            draft = null,
        )

    assertTrue(chrome.accessibleDescription.contains("cmd/miniorca/main.go"))
    assertTrue(chrome.accessibleDescription.contains("Selected declaration Main"))
    assertTrue(chrome.accessibleDescription.contains("Read-only source surface"))
  }

  @Test
  fun technicalReviewIdentityHashesAreLabeledForTheDisclosure() {
    assertEquals(
        listOf(
            ReviewIdentityHashDetail("Candidate hash", "draft-hash"),
            ReviewIdentityHashDetail("Check identity hash", "check-hash"),
        ),
        reviewIdentityHashDetails("draft-hash", "check-hash"),
    )
    assertEquals(emptyList(), reviewIdentityHashDetails("", null))
  }

  @Test
  fun landingModeAcceptsOnlyTheOpenProjectShortcut() {
    DesktopShortcut.entries.forEach { shortcut ->
      assertEquals(
          shortcut == DesktopShortcut.OpenProject,
          shortcutAvailable(DesktopShellMode.ProjectLanding, shortcut))
      assertTrue(shortcutAvailable(DesktopShellMode.ProjectWorkspace, shortcut))
    }
    assertTrue(!shortcutAvailable(DesktopShellMode.ProjectLanding, null))
  }

  @Test
  fun restoreProgressAndFailureNeverUnlockProjectOnlyShortcuts() {
    val attempt = ProjectOpeningAttempt(1, "/remembered", ProjectOpeningKind.Restore)
    listOf(ProjectOpeningOutcome.Opening, ProjectOpeningOutcome.Failed("Missing")).forEach { outcome
      ->
      val state =
          DesktopState(
              projectState =
                  ProjectWorkspaceState(openingAttempt = attempt.copy(outcome = outcome)))
      val mode = desktopShellMode(state)
      assertEquals(DesktopShellMode.ProjectLanding, mode)
      assertTrue(shortcutAvailable(mode, DesktopShortcut.OpenProject))
      listOf(
              DesktopShortcut.OpenFile,
              DesktopShortcut.OpenSymbol,
              DesktopShortcut.SummaryWorkspace,
              DesktopShortcut.Generate,
              DesktopShortcut.FocusDraft)
          .forEach { assertFalse(shortcutAvailable(mode, it)) }
    }
  }

  @Test
  fun highlightingLeavesSourceIntactAndStylesRecognizedTokens() {
    val source = "package demo\n// note\nfun run() = \"ok\"\n"
    val highlighted = highlightedCode(source)

    assertEquals(source, highlighted.text)
    assertTrue(highlighted.spanStyles.isNotEmpty())
  }
}
