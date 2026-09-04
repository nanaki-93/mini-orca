package io.miniorca.desktop

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

class DesktopThemeTest {
  @Test
  fun charcoalPaletteUsesTheApprovedSemanticColors() {
    assertEquals(Color(0xFF171B20), AppBackground)
    assertEquals(Color(0xFF12161B), Chrome)
    assertEquals(Color(0xFF1C2229), Panel)
    assertEquals(Color(0xFF242B33), Card)
    assertEquals(Color(0xFF2B333D), StrongSurface)
    assertEquals(Color(0xFF343D48), Border)
    assertEquals(Color(0xFFE6EDF3), PrimaryText)
    assertEquals(Color(0xFFAAB6C3), SecondaryText)
    assertEquals(Color(0xFF79B3FF), SelectionAccent)
    assertEquals(Color(0xFF285FCB), ActionFill)
    assertEquals(Color(0xFF65D2EC), FocusAccent)
    assertEquals(Color(0xFF65D6A3), Success)
    assertEquals(Color(0xFFF2BE66), Warning)
    assertEquals(Color(0xFFFF8F98), Error)
    assertEquals(Color(0xFF18352C), DiffAddedBackground)
    assertEquals(Color(0xFF3A232B), DiffRemovedBackground)
    assertNotEquals(ActionFill, OnActionFill)
  }

  @Test
  fun statusStylesKeepTheStateInTextAsWellAsColor() {
    assertEquals(StatusBadgeStyle("Fresh", Success), statusBadgeStyle("fresh"))
    assertEquals(StatusBadgeStyle("Stale", Warning), statusBadgeStyle("stale"))
    assertEquals(StatusBadgeStyle("Running", Warning), statusBadgeStyle("running"))
    assertEquals(StatusBadgeStyle("Failed", Error), statusBadgeStyle("failed"))
    assertEquals(StatusBadgeStyle("Not analyzed", SecondaryText), statusBadgeStyle("missing"))
  }

  @Test
  fun actionTonesUseSharedPaletteTokensForEverySemanticRole() {
    assertEquals(ActionFill, actionToneStyle(ActionTone.Primary).background)
    assertEquals(SelectionAccent, actionToneStyle(ActionTone.Navigation).content)
    assertEquals(Success, actionToneStyle(ActionTone.Positive).content)
    assertEquals(Warning, actionToneStyle(ActionTone.Attention).content)
    assertEquals(Error, actionToneStyle(ActionTone.Destructive).content)
    assertEquals(PrimaryText, actionToneStyle(ActionTone.Neutral).content)
    ActionTone.entries.forEach { tone ->
      val style = actionToneStyle(tone)
      assertNotEquals(style.background, style.disabledBackground)
      assertNotEquals(style.content, style.disabledContent)
      assertNotEquals(style.background, style.pressedBackground)
      assertNotEquals(style.background, style.selectedBackground)
    }
  }

  @Test
  fun compactButtonDensitiesStayWithinTheApprovedControlScale() {
    assertEquals(36.dp, buttonDensityStyle(ButtonDensity.Standard).height)
    assertEquals(32.dp, buttonDensityStyle(ButtonDensity.Toolbar).height)
    assertEquals(
        10.dp,
        buttonDensityStyle(ButtonDensity.Standard)
            .contentPadding
            .calculateLeftPadding(androidx.compose.ui.unit.LayoutDirection.Ltr))
    assertEquals(
        8.dp,
        buttonDensityStyle(ButtonDensity.Toolbar)
            .contentPadding
            .calculateLeftPadding(androidx.compose.ui.unit.LayoutDirection.Ltr))
  }

  @Test
  fun actionGroupsSwitchToVerticalBeforeTheyBecomeCrowded() {
    assertEquals(ActionGroupLayout.Vertical, actionGroupLayout(459f))
    assertEquals(ActionGroupLayout.Horizontal, actionGroupLayout(460f))
    assertEquals(
        ActionGroupLayout.Vertical, actionGroupLayout(419f, minimumHorizontalWidthDp = 420f))
    assertEquals(
        ActionGroupLayout.Horizontal, actionGroupLayout(420f, minimumHorizontalWidthDp = 420f))
  }

  @Test
  fun semanticSyntaxTokensDoNotChangeSourceText() {
    val source = "package demo\n// note\nfunc Run() string { return \"ok\" }\n"
    val highlighted = highlightedCode(source)

    assertEquals(source, highlighted.text)
    assertTrue(highlighted.spanStyles.any { it.item.color == CodeComment })
    assertTrue(highlighted.spanStyles.any { it.item.color == CodeString })
    assertTrue(highlighted.spanStyles.any { it.item.color == CodeKeyword })
  }

  @Test
  fun essentialTextActionAndFocusColorsMeetTheDarkThemeContrastTargets() {
    assertTrue(contrastRatio(PrimaryText, AppBackground) >= 4.5)
    assertTrue(contrastRatio(SecondaryText, Panel) >= 4.5)
    assertTrue(contrastRatio(OnActionFill, ActionFill) >= 4.5)
    assertTrue(contrastRatio(FocusAccent, Panel) >= 3.0)
    assertTrue(contrastRatio(Success, DiffAddedBackground) >= 4.5)
    assertTrue(contrastRatio(Error, DiffRemovedBackground) >= 4.5)
  }
}
