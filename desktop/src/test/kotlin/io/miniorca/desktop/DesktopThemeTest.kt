package io.miniorca.desktop

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

class DesktopThemeTest {
  @Test
  fun idePaletteUsesTheApprovedSemanticColors() {
    assertEquals(Color(0xFF1E1F22), AppBackground)
    assertEquals(Color(0xFF18191B), Chrome)
    assertEquals(Color(0xFF1E1F22), Panel)
    assertEquals(Color(0xFF26282C), Card)
    assertEquals(Color(0xFF2B2D30), StrongSurface)
    assertEquals(Color(0xFF323438), Border)
    assertEquals(Color(0xFFF2F2F2), PrimaryText)
    assertEquals(Color(0xFFC4C7C5), SecondaryText)
    assertEquals(Color(0xFF3574F0), SelectionAccent)
    assertEquals(Color(0xFF2E436E), SelectionSurface)
    assertEquals(Color(0xFFA8C7FA), SelectionText)
    assertEquals(Color(0xFF3574F0), ActionFill)
    assertEquals(Color(0xFF0B0D10), OnActionFill)
    assertEquals(Color(0xFFA8C7FA), FocusAccent)
    assertEquals(Color(0xFF65D6A3), Success)
    assertEquals(Color(0xFFF2BE66), Warning)
    assertEquals(Color(0xFFFF8F98), Error)
    assertEquals(Color(0xFF18352C), DiffAddedBackground)
    assertEquals(Color(0xFF3A232B), DiffRemovedBackground)
    assertNotEquals(ActionFill, OnActionFill)
  }

  @Test
  fun semanticSurfaceRolesKeepRailPaneCanvasOverlayAndSeparatorDistinct() {
    assertEquals(Color(0xFF18191B), ActivityRail)
    assertEquals(Color(0xFF1E1F22), ToolWindowSurface)
    assertEquals(Color(0xFF2B2D30), EditorCanvas)
    assertEquals(Color(0xFF26282C), OverlaySurface)
    assertEquals(Color(0xFF323438), PaneSeparator)
    assertEquals(ActivityRail, Chrome)
    assertEquals(ToolWindowSurface, Panel)
    assertEquals(OverlaySurface, Card)
    assertEquals(PaneSeparator, Border)
    assertEquals(
        5, setOf(ActivityRail, ToolWindowSurface, EditorCanvas, OverlaySurface, PaneSeparator).size)
  }

  @Test
  fun semanticTypographyRolesUseDenseBodyAndSectionMetrics() {
    assertEquals(13.sp, IdeTypography.body.fontSize)
    assertEquals(20.sp, IdeTypography.body.lineHeight)
    assertEquals(12.sp, IdeTypography.compactBody.fontSize)
    assertEquals(18.sp, IdeTypography.compactBody.lineHeight)
    assertEquals(11.sp, IdeTypography.section.fontSize)
    assertEquals(16.sp, IdeTypography.section.lineHeight)
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
    assertEquals(SelectionText, actionToneStyle(ActionTone.Navigation).content)
    assertEquals(SelectionSurface, actionToneStyle(ActionTone.Navigation).selectedBackground)
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
  fun containedControlsUseTheSixDpBaselineAndKeepFocusDistinctFromSelection() {
    assertEquals(RoundedCornerShape(6.dp), MiniOrcaShapes.small)
    assertEquals(MiniOrcaShapes.small, MiniOrcaButtonDefaults.shape)
    assertNotEquals(SelectionSurface, FocusAccent)
    assertNotEquals(SelectionAccent, FocusAccent)
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
  fun essentialTextActionAndFocusColorsMeetTheIdeContrastTargetsOnResolvedSurfaces() {
    assertTrue(contrastRatio(PrimaryText, AppBackground) >= 4.5)
    assertTrue(contrastRatio(SecondaryText, Panel) >= 4.5)
    assertTrue(contrastRatio(PrimaryText, EditorCanvas) >= 4.5)
    assertTrue(contrastRatio(PrimaryText, OverlaySurface) >= 4.5)
    assertTrue(contrastRatio(OnActionFill, ActionFill) >= 4.5)
    assertTrue(contrastRatio(SelectionText, SelectionSurface) >= 4.5)
    assertTrue(contrastRatio(FocusAccent, Panel) >= 3.0)
    assertTrue(contrastRatio(FocusAccent, OverlaySurface) >= 3.0)
    assertTrue(contrastRatio(FocusAccent, SelectionSurface) >= 3.0)
    assertTrue(
        contrastRatio(
            actionToneStyle(ActionTone.Primary).disabledContent,
            blendOver(actionToneStyle(ActionTone.Primary).disabledBackground, Panel)) >= 4.5)
    assertTrue(contrastRatio(Success, DiffAddedBackground) >= 4.5)
    assertTrue(contrastRatio(Error, DiffRemovedBackground) >= 4.5)
  }
}
