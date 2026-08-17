package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopAccessibilityTest {
    @Test fun keyboardShortcutsCoverFocusedWorkflowWithoutMouse() {
        assertEquals(DesktopShortcut.OpenFile, desktopShortcut("P", primaryModifier = true))
        assertEquals(DesktopShortcut.OpenSymbol, desktopShortcut("O", primaryModifier = true, shift = true))
        assertEquals(DesktopShortcut.OpenAction, desktopShortcut("K", primaryModifier = true))
        assertEquals(DesktopShortcut.Generate, desktopShortcut("Enter", primaryModifier = true))
        assertEquals(DesktopShortcut.Cancel, desktopShortcut("Escape", primaryModifier = false))
        assertEquals(DesktopShortcut.NextTab, desktopShortcut("Tab", primaryModifier = true))
        assertNull(desktopShortcut("O", primaryModifier = true))
    }

    @Test fun highlightingLeavesSourceIntactAndStylesRecognizedTokens() {
        val source = "package demo\n// note\nfun run() = \"ok\"\n"
        val highlighted = highlightedCode(source)

        assertEquals(source, highlighted.text)
        assertTrue(highlighted.spanStyles.isNotEmpty())
    }
}
