package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.interaction.collectIsHoveredAsState
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.DropdownMenu
import androidx.compose.material.LocalContentColor
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
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.disabled
import androidx.compose.ui.semantics.dismiss
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.PopupProperties

/** Shared visual policy for all compact controls; Material is not the control implementation. */
internal data class IdeActionColors(
    val background: Color,
    val hoveredBackground: Color,
    val pressedBackground: Color,
    val selectedBackground: Color,
    val disabledBackground: Color,
    val content: Color,
    val selectedContent: Color,
    val disabledContent: Color,
    val border: Color,
)

internal data class IdeActionInteraction(
    val hovered: Boolean = false,
    val pressed: Boolean = false
)

internal fun ideActionBackground(
    colors: IdeActionColors,
    enabled: Boolean,
    selected: Boolean,
    interaction: IdeActionInteraction,
): Color =
    when {
      !enabled -> colors.disabledBackground
      selected -> colors.selectedBackground
      interaction.pressed -> colors.pressedBackground
      interaction.hovered -> colors.hoveredBackground
      else -> colors.background
    }

/**
 * The one interactive surface used by dense buttons, tabs, and disclosure toggles.
 *
 * It intentionally keeps ordinary chrome flat and square while allowing contained workflow actions
 * to supply their small control corner radius.
 */
@Composable
@OptIn(ExperimentalFoundationApi::class)
internal fun IdeActionSurface(
    onClick: () -> Unit,
    colors: IdeActionColors,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    selected: Boolean = false,
    focusHighlight: Boolean = false,
    minimumHeight: Dp = 32.dp,
    contentPadding: PaddingValues = PaddingValues(horizontal = 8.dp, vertical = 4.dp),
    role: Role = Role.Button,
    shape: Shape = RoundedCornerShape(0.dp),
    accessibleName: String? = null,
    tooltip: String? = accessibleName,
    interactionSource: MutableInteractionSource? = null,
    interactionOverride: IdeActionInteraction? = null,
    content: @Composable RowScope.() -> Unit,
) {
  val interactions = interactionSource ?: remember { MutableInteractionSource() }
  val observedHovered by interactions.collectIsHoveredAsState()
  val observedPressed by interactions.collectIsPressedAsState()
  val focused by interactions.collectIsFocusedAsState()
  val interaction = interactionOverride ?: IdeActionInteraction(observedHovered, observedPressed)
  val background =
      ideActionBackground(
          colors = colors, enabled = enabled, selected = selected, interaction = interaction)
  val contentColor =
      when {
        !enabled -> colors.disabledContent
        selected -> colors.selectedContent
        else -> colors.content
      }
  val clickBehavior =
      if (enabled)
          Modifier.clickable(
              interactionSource = interactions,
              indication = null,
              role = role,
              onClickLabel = accessibleName,
              onClick = onClick)
      else Modifier.semantics { disabled() }
  val surface: @Composable () -> Unit = {
    Row(
        modifier =
            modifier
                .heightIn(min = minimumHeight)
                .clip(shape)
                .background(background)
                .border(
                    BorderStroke(
                        1.dp, if (focused || focusHighlight) FocusAccent else colors.border),
                    shape)
                .semantics { accessibleName?.let { contentDescription = it } }
                .then(clickBehavior)
                .padding(contentPadding),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.Center,
    ) {
      CompositionLocalProvider(LocalContentColor provides contentColor) {
        ProvideTextStyle(IdeTypography.action) { content() }
      }
    }
  }
  if (tooltip == null) surface()
  else TooltipArea(tooltip = { IdeControlTooltip(tooltip) }) { surface() }
}

@Composable
private fun IdeControlTooltip(label: String) {
  Text(
      label,
      color = PrimaryText,
      style = IdeTypography.section,
      modifier =
          Modifier.background(OverlaySurface)
              .border(BorderStroke(1.dp, PaneSeparator))
              .padding(horizontal = 6.dp, vertical = 4.dp),
  )
}

/** Quiet chrome has transparent hover/press treatment, separate from workflow actions. */
@Composable
internal fun ChromeButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    selected: Boolean = false,
    focusHighlight: Boolean = false,
    background: Color = Color.Transparent,
    contentPadding: PaddingValues = PaddingValues(horizontal = 8.dp, vertical = 4.dp),
    role: Role = Role.Button,
    accessibleName: String? = null,
    tooltip: String? = accessibleName,
    interactionSource: MutableInteractionSource? = null,
    interactionOverride: IdeActionInteraction? = null,
    content: @Composable RowScope.() -> Unit,
) {
  IdeActionSurface(
      onClick = onClick,
      colors =
          IdeActionColors(
              background = background,
              hoveredBackground = OverlaySurface,
              pressedBackground = EditorCanvas,
              selectedBackground = SelectionSurface,
              disabledBackground = background,
              content = SecondaryText,
              selectedContent = PrimaryText,
              disabledContent = FaintText,
              border = Color.Transparent),
      modifier = modifier,
      enabled = enabled,
      selected = selected,
      focusHighlight = focusHighlight,
      contentPadding = contentPadding,
      role = role,
      accessibleName = accessibleName,
      tooltip = tooltip,
      interactionSource = interactionSource,
      interactionOverride = interactionOverride,
      content = content,
  )
}

@Composable
internal fun ChromeTab(
    onClick: () -> Unit,
    selected: Boolean,
    modifier: Modifier = Modifier,
    focusHighlight: Boolean = false,
    accessibleName: String? = null,
    content: @Composable RowScope.() -> Unit,
) {
  ChromeButton(
      onClick = onClick,
      selected = selected,
      focusHighlight = focusHighlight,
      role = Role.Tab,
      accessibleName = accessibleName,
      modifier =
          modifier.heightIn(min = 32.dp).drawWithContent {
            drawContent()
            if (selected)
                drawLine(
                    SelectionAccent,
                    Offset(0f, size.height - 1.dp.toPx()),
                    Offset(size.width, size.height - 1.dp.toPx()),
                    2.dp.toPx())
          },
      content = content,
  )
}

/** One flat pane-header composition keeps disclosure targets separate from trailing actions. */
@Composable
internal fun IdePaneHeader(
    title: String,
    modifier: Modifier = Modifier,
    icon: DesktopIcon? = null,
    stateLabel: String? = null,
    stateTint: Color = SecondaryText,
    expanded: Boolean? = null,
    onToggle: (() -> Unit)? = null,
    disclosureModifier: Modifier = Modifier,
    actions: @Composable RowScope.() -> Unit = {},
    overflow: (@Composable () -> Unit)? = null,
    collapse: (@Composable () -> Unit)? = null,
) {
  require((expanded == null) == (onToggle == null)) {
    "A pane header must provide both disclosure state and toggle callback, or neither."
  }
  Row(
      modifier =
          modifier.fillMaxWidth().heightIn(min = 32.dp).padding(horizontal = 8.dp, vertical = 4.dp),
      verticalAlignment = Alignment.CenterVertically,
      horizontalArrangement = Arrangement.spacedBy(4.dp),
  ) {
    if (onToggle != null) {
      val disclosureLabel = if (expanded == true) "Collapse $title" else "Expand $title"
      ChromeButton(
          onClick = onToggle,
          modifier =
              disclosureModifier.weight(1f).semantics {
                stateDescription = if (expanded == true) "Expanded" else "Collapsed"
              },
          contentPadding = PaddingValues(0.dp),
          accessibleName = disclosureLabel,
          tooltip = null,
      ) {
        DesktopLineIcon(
            if (expanded == true) DesktopIcon.ChevronDown else DesktopIcon.ChevronRight,
            description = disclosureLabel,
            iconSize = 16.dp)
        icon?.let {
          Spacer(Modifier.width(4.dp))
          DesktopLineIcon(it, title, iconSize = 16.dp)
        }
        Spacer(Modifier.width(4.dp))
        HeaderTitle(title, stateLabel, stateTint)
      }
    } else {
      icon?.let { DesktopLineIcon(it, title, iconSize = 16.dp) }
      HeaderTitle(title, stateLabel, stateTint, Modifier.weight(1f))
    }
    Row(horizontalArrangement = Arrangement.spacedBy(4.dp), content = actions)
    overflow?.invoke()
    collapse?.invoke()
  }
}

@Composable
private fun RowScope.HeaderTitle(
    title: String,
    stateLabel: String?,
    stateTint: Color,
    modifier: Modifier = Modifier.weight(1f),
) {
  Column(modifier) {
    Text(title, color = PrimaryText, style = IdeTypography.compactBody, maxLines = 2)
    stateLabel?.let { Text(it, color = stateTint, style = IdeTypography.section, maxLines = 2) }
  }
}

/** A compact disclosure target with independently composable trailing actions. */
@Composable
internal fun IdeDisclosureHeader(
    title: String,
    expanded: Boolean,
    onToggle: () -> Unit,
    modifier: Modifier = Modifier,
    stateLabel: String? = null,
    stateTint: Color = SecondaryText,
    actions: @Composable RowScope.() -> Unit = {},
    overflow: (@Composable () -> Unit)? = null,
    collapse: (@Composable () -> Unit)? = null,
) {
  IdePaneHeader(
      title = title,
      expanded = expanded,
      onToggle = onToggle,
      disclosureModifier = modifier,
      stateLabel = stateLabel,
      stateTint = stateTint,
      actions = actions,
      overflow = overflow,
      collapse = collapse,
  )
}

@Composable
internal fun IdeHorizontalSeparator(modifier: Modifier = Modifier) {
  Box(modifier.fillMaxWidth().height(1.dp).background(PaneSeparator))
}

@Composable
internal fun IdeVerticalSeparator(modifier: Modifier = Modifier) {
  Box(modifier.fillMaxHeight().width(1.dp).background(PaneSeparator))
}

/** Retains Compose Desktop's menu placement and key handling behind shared IDE presentation. */
internal object IdePopupMenuDefaults {
  val minWidth = 196.dp
  val maxWidth = 360.dp
  val maxHeight = 360.dp
  val rowMinimumHeight = 32.dp
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
      accessibleName = label,
      tooltip = null,
      contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
  ) {
    icon?.let {
      DesktopLineIcon(it, label, tint = if (enabled) SecondaryText else FaintText, iconSize = 16.dp)
      Spacer(Modifier.width(8.dp))
    }
    Text(label, style = IdeTypography.compactBody, modifier = Modifier.weight(1f))
    status?.let {
      Spacer(Modifier.width(12.dp))
      it()
    }
  }
}
