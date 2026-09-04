package io.miniorca.desktop

import androidx.compose.ui.graphics.Color
import kotlin.test.Test
import kotlin.test.assertEquals

class ChromeControlsTest {
  @Test
  fun sharedActionSurfaceUsesOnePredictableStatePriority() {
    val colors =
        IdeActionColors(
            background = Color.Black,
            hoveredBackground = Color.DarkGray,
            pressedBackground = Color.Gray,
            selectedBackground = Color.Blue,
            disabledBackground = Color.LightGray,
            content = Color.White,
            selectedContent = Color.White,
            disabledContent = Color.Gray,
            border = Color.Transparent,
        )

    assertEquals(Color.Black, ideActionBackground(colors, true, false, IdeActionInteraction()))
    assertEquals(
        Color.DarkGray,
        ideActionBackground(colors, true, false, IdeActionInteraction(hovered = true)))
    assertEquals(
        Color.Gray,
        ideActionBackground(
            colors, true, false, IdeActionInteraction(hovered = true, pressed = true)))
    assertEquals(
        Color.Blue, ideActionBackground(colors, true, true, IdeActionInteraction(pressed = true)))
    assertEquals(
        Color.LightGray,
        ideActionBackground(colors, false, true, IdeActionInteraction(pressed = true)))
  }
}
