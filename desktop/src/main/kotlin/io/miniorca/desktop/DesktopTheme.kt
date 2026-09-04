package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.MaterialTheme
import androidx.compose.material.Shapes
import androidx.compose.material.Surface
import androidx.compose.material.Text
import androidx.compose.material.Typography
import androidx.compose.material.darkColors
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.pow
import org.jetbrains.jewel.intui.standalone.theme.IntUiTheme

/** Every desktop color is derived from this single dark semantic palette. */
internal object MiniOrcaPalette {
  val activityRail = Color(0xFF18191B)
  val toolWindow = Color(0xFF1E1F22)
  val editorCanvas = Color(0xFF2B2D30)
  val overlay = Color(0xFF26282C)
  val paneSeparator = Color(0xFF323438)
  val primaryText = Color(0xFFF2F2F2)
  val secondaryText = Color(0xFFC4C7C5)
  val faintText = Color(0xFF9FA2A6)
  val selectionAccent = Color(0xFF3574F0)
  val selectionSurface = Color(0xFF2E436E)
  val selectionText = Color(0xFFA8C7FA)
  val actionFill = Color(0xFF3574F0)
  val onActionFill = Color(0xFF0B0D10)
  val focusAccent = Color(0xFFA8C7FA)
  val success = Color(0xFF65D6A3)
  val warning = Color(0xFFF2BE66)
  val error = Color(0xFFFF8F98)
  val diffAddedBackground = Color(0xFF18352C)
  val diffRemovedBackground = Color(0xFF3A232B)
  val codeKeyword = Color(0xFFD7A4D8)
  val codeFunction = Color(0xFFE8C987)
  val codeString = Color(0xFFA8D59D)
  val codeComment = Color(0xFF93A38F)
  val codeType = Color(0xFF71D7CA)
}

internal object MiniOrcaSpacing {
  val compact = 4.dp
  val standard = 8.dp
  val roomy = 12.dp
  val section = 16.dp
}

internal enum class ActionGroupLayout {
  Horizontal,
  Vertical
}

internal fun actionGroupLayout(
    widthDp: Float,
    minimumHorizontalWidthDp: Float = 460f
): ActionGroupLayout =
    if (widthDp >= minimumHorizontalWidthDp) ActionGroupLayout.Horizontal
    else ActionGroupLayout.Vertical

@Composable
internal fun ResponsiveActionGroup(
    modifier: Modifier = Modifier,
    minimumHorizontalWidth: androidx.compose.ui.unit.Dp = 460.dp,
    content: @Composable () -> Unit,
) {
  BoxWithConstraints(modifier) {
    if (actionGroupLayout(maxWidth.value, minimumHorizontalWidth.value) ==
        ActionGroupLayout.Horizontal) {
      androidx.compose.foundation.layout.Row(
          horizontalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.compact),
          content = { content() })
    } else {
      Column(
          verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.compact),
          content = { content() })
    }
  }
}

@Composable
internal fun ResponsiveFieldPair(
    modifier: Modifier = Modifier,
    minimumHorizontalWidth: androidx.compose.ui.unit.Dp = 420.dp,
    first: @Composable (Modifier) -> Unit,
    second: @Composable (Modifier) -> Unit,
) {
  BoxWithConstraints(modifier) {
    if (actionGroupLayout(maxWidth.value, minimumHorizontalWidth.value) ==
        ActionGroupLayout.Horizontal) {
      androidx.compose.foundation.layout.Row {
        first(Modifier.weight(1f))
        Spacer(Modifier.width(MiniOrcaSpacing.compact))
        second(Modifier.weight(1f))
      }
    } else {
      Column(verticalArrangement = Arrangement.spacedBy(MiniOrcaSpacing.compact)) {
        first(Modifier.fillMaxWidth())
        second(Modifier.fillMaxWidth())
      }
    }
  }
}

internal val MiniOrcaShapes =
    Shapes(
        small = RoundedCornerShape(6.dp),
        medium = RoundedCornerShape(6.dp),
        large = RoundedCornerShape(8.dp),
    )

internal object IdeTypography {
  val body = androidx.compose.ui.text.TextStyle(fontSize = 13.sp, lineHeight = 20.sp)
  val compactBody = androidx.compose.ui.text.TextStyle(fontSize = 12.sp, lineHeight = 18.sp)
  val section =
      androidx.compose.ui.text.TextStyle(
          fontWeight = FontWeight.SemiBold, fontSize = 11.sp, lineHeight = 16.sp)
  val action =
      androidx.compose.ui.text.TextStyle(
          fontWeight = FontWeight.Normal,
          fontSize = 12.sp,
          lineHeight = 16.sp,
          letterSpacing = 0.15.sp)
}

internal val MiniOrcaTypography =
    Typography(
        defaultFontFamily = FontFamily.Default,
        h6 = IdeTypography.body.copy(fontWeight = FontWeight.SemiBold),
        body1 = IdeTypography.body,
        body2 = IdeTypography.compactBody,
        button = IdeTypography.action,
        caption = IdeTypography.section,
    )

internal enum class ActionTone {
  Primary,
  Navigation,
  Positive,
  Attention,
  Destructive,
  Neutral,
}

internal enum class ButtonDensity {
  Standard,
  Toolbar,
}

internal data class ActionToneStyle(
    val background: Color,
    val pressedBackground: Color,
    val selectedBackground: Color,
    val content: Color,
    val disabledBackground: Color,
    val disabledContent: Color,
    val border: Color,
)

internal fun actionToneStyle(tone: ActionTone): ActionToneStyle =
    when (tone) {
      ActionTone.Primary ->
          ActionToneStyle(
              ActionFill,
              ActionFill.copy(alpha = 0.84f),
              ActionFill.copy(alpha = 0.92f),
              OnActionFill,
              ActionFill.copy(alpha = 0.18f),
              FaintText,
              ActionFill.copy(alpha = 0.72f))
      ActionTone.Navigation ->
          ActionToneStyle(
              Card,
              SelectionAccent.copy(alpha = 0.25f),
              SelectionSurface,
              SelectionText,
              Panel,
              FaintText,
              Border)
      ActionTone.Positive ->
          ActionToneStyle(
              Card,
              Success.copy(alpha = 0.25f),
              Success.copy(alpha = 0.28f),
              Success,
              Panel,
              FaintText,
              Border)
      ActionTone.Attention ->
          ActionToneStyle(
              Card,
              Warning.copy(alpha = 0.25f),
              Warning.copy(alpha = 0.28f),
              Warning,
              Panel,
              FaintText,
              Border)
      ActionTone.Destructive ->
          ActionToneStyle(
              Card,
              Error.copy(alpha = 0.25f),
              Error.copy(alpha = 0.28f),
              Error,
              Panel,
              FaintText,
              Border)
      ActionTone.Neutral ->
          ActionToneStyle(
              Card,
              StrongSurface,
              StrongSurface,
              PrimaryText,
              Panel,
              FaintText,
              Border.copy(alpha = 0.9f))
    }

internal data class ButtonDensityStyle(
    val height: androidx.compose.ui.unit.Dp,
    val contentPadding: PaddingValues
)

internal fun buttonDensityStyle(density: ButtonDensity): ButtonDensityStyle =
    when (density) {
      ButtonDensity.Standard ->
          ButtonDensityStyle(32.dp, PaddingValues(horizontal = 10.dp, vertical = 4.dp))
      ButtonDensity.Toolbar ->
          ButtonDensityStyle(32.dp, PaddingValues(horizontal = 8.dp, vertical = 4.dp))
    }

@Composable
internal fun CompactSingleLineField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    showLabel: Boolean = true,
    placeholder: String? = null,
    textStyle: TextStyle = IdeTypography.compactBody,
) {
  val interactions = remember { MutableInteractionSource() }
  val focused by interactions.collectIsFocusedAsState()
  Column(modifier) {
    if (showLabel)
        Text(
            label,
            color = SecondaryText,
            style = IdeTypography.section,
            modifier = Modifier.padding(bottom = 5.dp))
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        enabled = enabled,
        singleLine = true,
        interactionSource = interactions,
        textStyle = textStyle.copy(color = if (enabled) PrimaryText else FaintText),
        cursorBrush = SolidColor(FocusAccent),
        modifier =
            Modifier.fillMaxWidth()
                .heightIn(min = 34.dp)
                .background(Panel, MiniOrcaShapes.small)
                .border(
                    BorderStroke(1.dp, if (focused) FocusAccent else Border), MiniOrcaShapes.small)
                .semantics { contentDescription = label },
        decorationBox = { input ->
          Box(
              Modifier.padding(horizontal = 10.dp, vertical = 8.dp),
              contentAlignment = Alignment.CenterStart) {
                if (value.isEmpty())
                    Text(
                        placeholder ?: if (showLabel) "" else label,
                        color = FaintText,
                        style = IdeTypography.compactBody,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis)
                input()
              }
        },
    )
  }
}

@Composable
internal fun MiniOrcaButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    tone: ActionTone = ActionTone.Neutral,
    density: ButtonDensity = ButtonDensity.Standard,
    selected: Boolean = false,
    focusHighlight: Boolean = false,
    content: @Composable RowScope.() -> Unit,
) {
  val densityStyle = buttonDensityStyle(density)
  val style = actionToneStyle(tone)
  IdeActionSurface(
      onClick = onClick,
      colors =
          IdeActionColors(
              background = style.background,
              hoveredBackground = style.pressedBackground,
              pressedBackground = style.pressedBackground,
              selectedBackground = style.selectedBackground,
              disabledBackground = style.disabledBackground,
              content = style.content,
              selectedContent = style.content,
              disabledContent = style.disabledContent,
              border = style.border),
      modifier = modifier,
      enabled = enabled,
      selected = selected,
      focusHighlight = focusHighlight,
      minimumHeight = densityStyle.height,
      contentPadding = densityStyle.contentPadding,
      shape = MiniOrcaShapes.small,
      content = content,
  )
}

@Composable
internal fun MiniOrcaTheme(content: @Composable () -> Unit) {
  IntUiTheme(isDark = true) {
    MaterialTheme(
        colors =
            darkColors(
                primary = MiniOrcaPalette.actionFill,
                secondary = MiniOrcaPalette.selectionAccent,
                background = MiniOrcaPalette.toolWindow,
                surface = MiniOrcaPalette.toolWindow,
                error = MiniOrcaPalette.error,
                onPrimary = MiniOrcaPalette.onActionFill,
                onBackground = MiniOrcaPalette.primaryText,
                onSurface = MiniOrcaPalette.primaryText,
                onError = MiniOrcaPalette.toolWindow,
            ),
        typography = MiniOrcaTypography,
        shapes = MiniOrcaShapes,
        content = content,
    )
  }
}

internal val ActivityRail = MiniOrcaPalette.activityRail
internal val ToolWindowSurface = MiniOrcaPalette.toolWindow
internal val EditorCanvas = MiniOrcaPalette.editorCanvas
internal val OverlaySurface = MiniOrcaPalette.overlay
internal val PaneSeparator = MiniOrcaPalette.paneSeparator

// These aliases keep unmigrated Material call sites on the single semantic palette.
internal val AppBackground = ToolWindowSurface
internal val Chrome = ActivityRail
internal val Panel = ToolWindowSurface
internal val Card = OverlaySurface
internal val StrongSurface = EditorCanvas
internal val Border = PaneSeparator
internal val PrimaryText = MiniOrcaPalette.primaryText
internal val SecondaryText = MiniOrcaPalette.secondaryText
internal val FaintText = MiniOrcaPalette.faintText
internal val SelectionAccent = MiniOrcaPalette.selectionAccent
internal val SelectionSurface = MiniOrcaPalette.selectionSurface
internal val SelectionText = MiniOrcaPalette.selectionText
internal val ActionFill = MiniOrcaPalette.actionFill
internal val OnActionFill = MiniOrcaPalette.onActionFill
internal val FocusAccent = MiniOrcaPalette.focusAccent
internal val Success = MiniOrcaPalette.success
internal val Warning = MiniOrcaPalette.warning
internal val Error = MiniOrcaPalette.error
internal val DiffAddedBackground = MiniOrcaPalette.diffAddedBackground
internal val DiffRemovedBackground = MiniOrcaPalette.diffRemovedBackground
internal val CodeKeyword = MiniOrcaPalette.codeKeyword
internal val CodeFunction = MiniOrcaPalette.codeFunction
internal val CodeString = MiniOrcaPalette.codeString
internal val CodeComment = MiniOrcaPalette.codeComment
internal val CodeType = MiniOrcaPalette.codeType

data class StatusBadgeStyle(val label: String, val color: Color)

internal fun statusBadgeStyle(status: String): StatusBadgeStyle =
    when (status.lowercase()) {
      "fresh" -> StatusBadgeStyle("Fresh", Success)
      "stale" -> StatusBadgeStyle("Stale", Warning)
      "running" -> StatusBadgeStyle("Running", Warning)
      "failed" -> StatusBadgeStyle("Failed", Error)
      else -> StatusBadgeStyle("Not analyzed", SecondaryText)
    }

@Composable
internal fun MiniOrcaPanel(
    modifier: Modifier = Modifier,
    raised: Boolean = false,
    contentPadding: PaddingValues = PaddingValues(MiniOrcaSpacing.roomy),
    content: @Composable () -> Unit,
) {
  Surface(
      modifier = modifier.border(BorderStroke(1.dp, Border), MiniOrcaShapes.small),
      color = if (raised) Card else Panel,
      shape = MiniOrcaShapes.small,
  ) {
    Column(Modifier.padding(contentPadding)) { content() }
  }
}

@Composable
internal fun SectionLabel(label: String, modifier: Modifier = Modifier) {
  Text(label, color = SecondaryText, style = IdeTypography.section, modifier = modifier)
}

@Composable
internal fun WorkspacePaneHeader(title: String, modifier: Modifier = Modifier) {
  IdePaneHeader(title = title, modifier = modifier)
}

@Composable
internal fun CompactKeyValueRow(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
) {
  Row(
      modifier.fillMaxWidth().semantics { contentDescription = "$label: $value" },
      verticalAlignment = androidx.compose.ui.Alignment.Top,
  ) {
    Text(
        label,
        color = SecondaryText,
        style = IdeTypography.compactBody,
        modifier = Modifier.width(96.dp),
    )
    Text(
        value,
        color = PrimaryText,
        style = IdeTypography.compactBody,
        maxLines = 2,
        overflow = TextOverflow.Ellipsis,
        modifier = Modifier.weight(1f),
    )
  }
}

@Composable
internal fun CompactKeyValueRows(
    values: List<Pair<String, String>>,
    modifier: Modifier = Modifier,
) {
  Column(modifier) {
    values.forEachIndexed { index, (label, value) ->
      if (index > 0) Spacer(Modifier.height(MiniOrcaSpacing.compact))
      CompactKeyValueRow(label, value)
    }
  }
}

@Composable
internal fun StatusBadge(status: String, modifier: Modifier = Modifier) {
  val style = statusBadgeStyle(status)
  Text(
      style.label,
      color = style.color,
      style = IdeTypography.compactBody.copy(fontWeight = FontWeight.SemiBold),
      maxLines = 1,
      overflow = TextOverflow.Ellipsis,
      modifier =
          modifier
              .background(style.color.copy(alpha = 0.12f), MiniOrcaShapes.small)
              .border(BorderStroke(1.dp, style.color.copy(alpha = 0.42f)), MiniOrcaShapes.small)
              .semantics { contentDescription = "Status: ${style.label}" }
              .padding(horizontal = MiniOrcaSpacing.standard, vertical = MiniOrcaSpacing.compact),
  )
}

@Composable
internal fun SystemStateMessage(
    title: String,
    message: String,
    accent: Color = SecondaryText,
    modifier: Modifier = Modifier,
) {
  MiniOrcaPanel(modifier = modifier, raised = true) {
    Text(title, color = PrimaryText, fontWeight = FontWeight.SemiBold)
    Spacer(Modifier.height(MiniOrcaSpacing.standard))
    Text(message, color = accent, style = IdeTypography.body)
  }
}

internal fun formatBytes(bytes: Long): String =
    when {
      bytes < 1024 -> "$bytes B"
      bytes < 1024 * 1024 -> "${bytes / 1024} KB"
      else -> "${bytes / (1024 * 1024)} MB"
    }

internal fun blendOver(foreground: Color, background: Color): Color {
  val alpha = foreground.alpha + background.alpha * (1f - foreground.alpha)
  if (alpha == 0f) return Color.Transparent
  fun channel(source: Float, backdrop: Float): Float =
      (source * foreground.alpha + backdrop * background.alpha * (1f - foreground.alpha)) / alpha
  return Color(
      red = channel(foreground.red, background.red),
      green = channel(foreground.green, background.green),
      blue = channel(foreground.blue, background.blue),
      alpha = alpha)
}

internal fun contrastRatio(foreground: Color, background: Color): Double {
  fun linear(component: Float): Double =
      component.toDouble().let {
        if (it <= 0.03928) it / 12.92 else ((it + 0.055) / 1.055).pow(2.4)
      }
  fun luminance(color: Color): Double =
      0.2126 * linear(color.red) + 0.7152 * linear(color.green) + 0.0722 * linear(color.blue)
  val resolvedForeground = blendOver(foreground, background)
  val lighter = maxOf(luminance(resolvedForeground), luminance(background))
  val darker = minOf(luminance(resolvedForeground), luminance(background))
  return (lighter + 0.05) / (darker + 0.05)
}
