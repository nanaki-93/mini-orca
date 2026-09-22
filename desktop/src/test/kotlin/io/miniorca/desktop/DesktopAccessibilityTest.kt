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
  fun analysisFileStatusMarkersAreDecorativeAtWideAndCompactWidths() {
    listOf(1_440 to 900, 800 to 650).forEach { (width, height) ->
      ComposeVisualFixture(width, height, 1.5f) {
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
  fun highlightingLeavesSourceIntactAndStylesRecognizedTokens() {
    val source = "package demo\n// note\nfun run() = \"ok\"\n"
    val highlighted = highlightedCode(source)

    assertEquals(source, highlighted.text)
    assertTrue(highlighted.spanStyles.isNotEmpty())
  }
}
