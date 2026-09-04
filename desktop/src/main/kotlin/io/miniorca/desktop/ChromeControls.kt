package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.interaction.collectIsHoveredAsState
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.DropdownMenu
import androidx.compose.material.LocalContentColor
import androidx.compose.material.MaterialTheme
import androidx.compose.material.ProvideTextStyle
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.dismiss
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.PopupProperties

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
  val pressed by interactions.collectIsPressedAsState()
  val focused by interactions.collectIsFocusedAsState()
  val fill =
      when {
        !enabled -> background
        selected -> SelectionSurface
        pressed -> StrongSurface
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

/** Retains Compose Desktop's menu placement and key handling behind shared IDE presentation. */
internal object IdePopupMenuDefaults {
  val minWidth = 196.dp
  val maxWidth = 360.dp
  val maxHeight = 360.dp
  val rowMinimumHeight = 36.dp
}

@Composable
internal fun IdeDropdownMenu(
    expanded: Boolean,
    onDismissRequest: () -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
  DropdownMenu(
      expanded = expanded,
      onDismissRequest = onDismissRequest,
      modifier =
          modifier
              .widthIn(min = IdePopupMenuDefaults.minWidth, max = IdePopupMenuDefaults.maxWidth)
              .heightIn(max = IdePopupMenuDefaults.maxHeight),
      properties = PopupProperties(focusable = true),
      scrollState = rememberScrollState(),
  ) {
    IdePopupMenuSurface(content = content, onDismissRequest = onDismissRequest)
  }
}

@Composable
internal fun IdePopupMenuSurface(
    content: @Composable ColumnScope.() -> Unit,
    modifier: Modifier = Modifier,
    onDismissRequest: (() -> Unit)? = null,
) {
  Column(
      modifier
          .fillMaxWidth()
          .shadow(6.dp, MiniOrcaShapes.medium)
          .clip(MiniOrcaShapes.medium)
          .background(Card)
          .border(BorderStroke(1.dp, Border), MiniOrcaShapes.medium)
          .semantics {
            onDismissRequest?.let { onDismiss ->
              dismiss {
                onDismiss()
                true
              }
            }
          }
          .padding(vertical = 4.dp),
      content = content,
  )
}

/** A compact shared control inside the Desktop menu's established popup and dismissal boundary. */
@Composable
internal fun IdeDropdownMenuItem(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    icon: DesktopIcon? = null,
    status: @Composable (() -> Unit)? = null,
) {
  ChromeButton(
      onClick = onClick,
      modifier = modifier.fillMaxWidth().heightIn(min = IdePopupMenuDefaults.rowMinimumHeight),
      enabled = enabled,
      background = Color.Transparent,
      contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
  ) {
    icon?.let {
      DesktopLineIcon(it, label, tint = if (enabled) SecondaryText else FaintText, iconSize = 16.dp)
      androidx.compose.foundation.layout.Spacer(Modifier.width(8.dp))
    }
    Text(label, fontSize = 12.sp, lineHeight = 16.sp, modifier = Modifier.weight(1f))
    status?.let {
      androidx.compose.foundation.layout.Spacer(Modifier.width(12.dp))
      it()
    }
  }
}
