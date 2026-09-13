package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class TerminalTabsTest {
  @Test
  fun shellTabsShareOneBarAndHaveIndependentKeyboardAccessibleActions() {
    var selected: Long? = null
    var closed: Long? = null
    var created = 0
    var toggled = 0
    val tabs =
        (1L..2L).map {
          TerminalTabState(it, "Shell $it", TerminalSessionState(TerminalSessionPhase.Running))
        }
    ComposeVisualFixture(800, 100, 1.5f) {
          TerminalBar(
              TerminalWorkspaceState(tabs = tabs, activeTabId = 2),
              false,
              { toggled++ },
              TerminalTabActions({ selected = it }, { created++ }, { closed = it }))
        }
        .use { fixture ->
          fixture.render("terminal-tabs-keyboard")
          fixture.clickDescription("Shell 1")
          assertEquals(1L, selected)
          fixture.clickDescription("Close Shell 2")
          assertEquals(2L, closed)
          assertEquals(1L, selected)
          assertEquals(0, toggled)
          fixture.clickDescription("New shell")
          assertEquals(1, created)
          assertTrue(fixture.requestFocus("Shell 2"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(2L, selected)
          assertEquals("Shell running", fixture.stateDescription("Shell 2"))
          listOf(
                  "Open shell",
                  "Close shell",
                  "Focus terminal",
                  "Reindex project",
                  "Back to editor · Ctrl+Shift+F12")
              .forEach { assertFalse(fixture.hasText(it)) }
        }
  }

  @Test
  fun cleanupPendingKeepsTheTabVisibleAndDisablesCloseAndCreate() {
    var actions = 0
    val tab =
        TerminalTabState(
            1,
            "Shell 1",
            TerminalSessionState(
                TerminalSessionPhase.Closed, error = "Shell did not stop", cleanupPending = true))
    ComposeVisualFixture(640, 100) {
          TerminalBar(
              TerminalWorkspaceState(tabs = listOf(tab), activeTabId = 1),
              false,
              {},
              TerminalTabActions({}, { actions++ }, { actions++ }))
        }
        .use { fixture ->
          fixture.render("terminal-tabs-cleanup")
          assertTrue(fixture.hasText("Shell 1 · Stopping shell…"))
          assertFalse(fixture.tryClick("New shell"))
          assertFalse(fixture.tryClick("Close Shell 1"))
          assertEquals(0, actions)
        }
  }

  @Test
  fun tabStripSupportsNarrowShortAndScaledLayoutsAndKeepsNewShellReachable() {
    val sizes = listOf(1440 to 900, 1000 to 650, 999 to 650, 800 to 650, 1280 to 600)
    sizes.forEach { (width, height) ->
      listOf(1f, 1.25f, 1.5f).forEach { scale ->
        val tabs =
            (1L..8L).map {
              TerminalTabState(it, "Shell $it", TerminalSessionState(TerminalSessionPhase.Running))
            }
        var created = false
        ComposeVisualFixture(width, height, scale) {
              Column(Modifier.fillMaxSize().background(EditorCanvas)) {
                TerminalDock(
                    DesktopLayoutState(bottomCollapsed = false),
                    TerminalWorkspaceState(tabs = tabs, activeTabId = 8),
                    {},
                    {},
                    TerminalTabActions({}, { created = true }, {}),
                    {},
                    {},
                    { Text("Synthetic terminal content") })
              }
            }
            .use { fixture ->
              repeat(3) { fixture.render() }
              fixture.render("terminal-tabs-$width-$height-$scale")
              fixture.assertTextFits("Shell 8")
              assertTrue(fixture.hasDescription("New shell"))
              assertTrue(fixture.hasDescription("Close Shell 8"))
              fixture.clickDescription("New shell")
              assertTrue(created)
            }
      }
    }
  }
}
