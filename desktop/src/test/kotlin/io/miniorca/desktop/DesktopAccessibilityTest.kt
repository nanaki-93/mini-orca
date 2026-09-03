package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopAccessibilityTest {
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
    assertEquals(
        DesktopShortcut.FocusBugsFilters,
        desktopShortcut("F", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.FocusDraft, desktopShortcut("D", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.ValidateDraft, desktopShortcut("V", primaryModifier = true, shift = true))
    assertEquals(
        DesktopShortcut.RunDraftChecks, desktopShortcut("C", primaryModifier = true, shift = true))
  }

  @Test
  fun workspaceSemanticsKeepTextualSelectedStateWithoutCounters() {
    assertEquals("Editor, selected", workspaceSemanticsLabel(Workspace.Editor, true))
    assertEquals("Bugs, not selected", workspaceSemanticsLabel(Workspace.Bugs, false))
  }

  @Test
  fun editorFileHeaderExposesTheSelectedFileIdentityWithoutProgressState() {
    val header =
        editorFileHeaderUiState(
            ProjectFileInfo(
                path = "cmd/miniorca/main.go",
                contentHash = "hash",
                name = "main.go",
                language = "Go",
                sizeBytes = 0,
                lineCount = 0,
                modifiedAt = "",
                binary = false,
            ))

    assertEquals("main.go. cmd/miniorca/main.go", editorFileHeaderDescription(header))
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
