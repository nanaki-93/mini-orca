package io.miniorca.desktop

import androidx.compose.ui.graphics.Color
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

class DesktopThemeTest {
    @Test fun focusFlowPaletteUsesTheApprovedSemanticColors() {
        assertEquals(Color(0xFF080917), AppBackground)
        assertEquals(Color(0xFF101225), Panel)
        assertEquals(Color(0xFF171A31), Card)
        assertEquals(Color(0xFF222640), StrongSurface)
        assertEquals(Color(0xFF292D49), Border)
        assertEquals(Color(0xFFF2F2FB), PrimaryText)
        assertEquals(Color(0xFF9297B6), SecondaryText)
        assertEquals(Color(0xFF9B8CFF), Accent)
        assertEquals(Color(0xFF62D8EF), CyanAccent)
        assertEquals(Color(0xFF55DDB0), Success)
        assertEquals(Color(0xFFFFC86E), Warning)
        assertEquals(Color(0xFFFF7F9F), Error)
        assertNotEquals(Accent, OnAccent)
    }

    @Test fun statusStylesKeepTheStateInTextAsWellAsColor() {
        assertEquals(StatusBadgeStyle("Fresh", Success), statusBadgeStyle("fresh"))
        assertEquals(StatusBadgeStyle("Stale", Warning), statusBadgeStyle("stale"))
        assertEquals(StatusBadgeStyle("Running", Warning), statusBadgeStyle("running"))
        assertEquals(StatusBadgeStyle("Failed", Error), statusBadgeStyle("failed"))
        assertEquals(StatusBadgeStyle("Not analyzed", SecondaryText), statusBadgeStyle("missing"))
    }

    @Test fun semanticSyntaxTokensDoNotChangeSourceText() {
        val source = "package demo\n// note\nfunc Run() string { return \"ok\" }\n"
        val highlighted = highlightedCode(source)

        assertEquals(source, highlighted.text)
        assertTrue(highlighted.spanStyles.any { it.item.color == CodeComment })
        assertTrue(highlighted.spanStyles.any { it.item.color == CodeString })
        assertTrue(highlighted.spanStyles.any { it.item.color == CodeKeyword })
    }
}
