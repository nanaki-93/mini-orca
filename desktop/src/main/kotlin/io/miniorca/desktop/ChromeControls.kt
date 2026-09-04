package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.interaction.collectIsHoveredAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material.LocalContentColor
import androidx.compose.material.MaterialTheme
import androidx.compose.material.ProvideTextStyle
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp

/** Quiet chrome has its own interaction treatment, separate from workflow actions. */
@Composable
internal fun ChromeButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    selected: Boolean = false,
    focusHighlight: Boolean = false,
    background: Color = Color.Transparent,
    contentPadding: PaddingValues = PaddingValues(horizontal = 10.dp, vertical = 6.dp),
    role: Role = Role.Button,
    content: @Composable RowScope.() -> Unit,
) {
  val interactions = remember { MutableInteractionSource() }
  val hovered by interactions.collectIsHoveredAsState()
  val focused by interactions.collectIsFocusedAsState()
  val fill =
      when {
        selected -> SelectionAccent.copy(alpha = 0.10f)
        hovered -> StrongSurface
        else -> background
      }
  Row(
      modifier
          .heightIn(min = 32.dp)
          .clip(MiniOrcaShapes.small)
          .background(fill)
          .border(
              BorderStroke(1.dp, if (focused || focusHighlight) FocusAccent else Color.Transparent),
              MiniOrcaShapes.small)
          .clickable(
              interactions, indication = null, enabled = enabled, role = role, onClick = onClick)
          .padding(contentPadding),
      verticalAlignment = Alignment.CenterVertically,
      horizontalArrangement = Arrangement.Center,
  ) {
    CompositionLocalProvider(
        LocalContentColor provides
            if (!enabled) FaintText else if (selected) PrimaryText else SecondaryText) {
          ProvideTextStyle(MaterialTheme.typography.button) { content() }
        }
  }
}

@Composable
internal fun ChromeTab(
    onClick: () -> Unit,
    selected: Boolean,
    modifier: Modifier = Modifier,
    focusHighlight: Boolean = false,
    content: @Composable RowScope.() -> Unit,
) {
  ChromeButton(
      onClick = onClick,
      selected = selected,
      focusHighlight = focusHighlight,
      role = Role.Tab,
      modifier =
          modifier.heightIn(min = 38.dp).drawWithContent {
            drawContent()
            if (selected)
                drawLine(SelectionAccent, Offset(0f, 0f), Offset(size.width, 0f), 2.dp.toPx())
          },
      content = content,
  )
}
