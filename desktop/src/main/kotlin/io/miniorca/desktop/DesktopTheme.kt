package io.miniorca.desktop

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.pow
import org.jetbrains.jewel.intui.standalone.theme.IntUiTheme

/** Every desktop color is derived from this single dark semantic palette. */
internal object MiniOrcaPalette {
  val activityRail = Color(0xFF171821)
  val toolWindow = Color(0xFF202130)
  val editorCanvas = Color(0xFF1A1B26)
  val overlay = Color(0xFF292B3C)
  // The mock's raised surface is too close to the canvas for the maintained header boundary.
  val header = Color(0xFF2E3044)
  val control = Color(0xFF292B3C)
  val controlHover = Color(0xFF34364B)
  // Decorative boundaries need less contrast than essential field and control outlines.
  val paneSeparator = Color(0xFF404158)
  val controlBorder = Color(0xFF9299BE)
  val primaryText = Color(0xFFC0CAF5)
  val secondaryText = Color(0xFFA9B1D6)
  val faintText = Color(0xFFA9B1D6)
  val selectionAccent = Color(0xFF7DCFFF)
  val selectionSurface = Color(0xFF242A41)
  val selectionText = Color(0xFFC0CAF5)
  val actionFill = Color(0xFF7AA2F7)
  val actionHover = Color(0xFF9AB8FF)
  val onActionFill = Color(0xFF161824)
  val focusAccent = Color(0xFFD7DEFF)
  val information = Color(0xFF7DCFFF)
  val success = Color(0xFF9ECE6A)
  // Brighter than the reference amber so warning labels survive selected translucent fills.
  val warning = Color(0xFFE6B66F)
  val error = Color(0xFFFF8FA3)
  val identityAccent = Color(0xFFBB9AF7)
  val diffAddedBackground = Color(0xFF26382F)
  val diffRemovedBackground = Color(0xFF402632)
  val codeKeyword = Color(0xFFBB9AF7)
  val codeFunction = Color(0xFFE0AF68)
  val codeString = Color(0xFF9ECE6A)
  val codeComment = Color(0xFF9299BE)
  val codeType = Color(0xFF7DCFFF)
}

internal object MiniOrcaSpacing {
  val compact = 4.dp
  val standard = 8.dp
  val roomy = 12.dp
  val section = 16.dp
}

internal object MiniOrcaShapes {
  val control = RoundedCornerShape(10.dp)
  val interactiveCard = RoundedCornerShape(14.dp)
  val workspace = RoundedCornerShape(18.dp)
  val overlay = RoundedCornerShape(18.dp)
  val indicator = RoundedCornerShape(4.dp)
  val pill = RoundedCornerShape(50)
}

internal object IdeTypography {
  val body = androidx.compose.ui.text.TextStyle(fontSize = 13.sp, lineHeight = 20.sp)
  val compactBody = androidx.compose.ui.text.TextStyle(fontSize = 12.sp, lineHeight = 18.sp)
  val workspaceBody = body.copy(fontSize = 14.sp, lineHeight = 22.sp)
  val workspaceMetadata = compactBody.copy(fontSize = 13.sp, lineHeight = 20.sp)
  val workspaceHeading = workspaceBody.copy(fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
  val toolbarIdentity =
      compactBody.copy(fontSize = 13.sp, fontWeight = FontWeight.SemiBold, lineHeight = 18.sp)
  val resultHeading = body.copy(fontWeight = FontWeight.SemiBold)
  val resultLabel = compactBody.copy(fontWeight = FontWeight.SemiBold)
  val resultCode = compactBody.copy(fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
  val section =
      androidx.compose.ui.text.TextStyle(
          fontWeight = FontWeight.SemiBold, fontSize = 12.sp, lineHeight = 18.sp)
  val action =
      androidx.compose.ui.text.TextStyle(
          fontWeight = FontWeight.Medium,
          fontSize = 12.sp,
          lineHeight = 16.sp,
          letterSpacing = 0.15.sp)
}

internal enum class ActionTone {
  Primary,
  Navigation,
  Positive,
  PositivePrimary,
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
    val selectedContent: Color = content,
)

internal fun actionToneStyle(tone: ActionTone): ActionToneStyle =
    when (tone) {
      // Bright inverse-text fills cannot support the cyan selection outline at 3:1;
      // selected primary actions use the dark selected surface with light text.
      ActionTone.Primary ->
          ActionToneStyle(
              ActionFill,
              MiniOrcaPalette.actionHover,
              SelectionSurface,
              OnActionFill,
              Panel,
              FaintText,
              ActionFill,
              SelectionText)
      ActionTone.Navigation ->
          ActionToneStyle(
              labelBadgeBackground(SelectionAccent),
              blendOver(SelectionAccent.copy(alpha = 0.24f), Panel),
              SelectionSurface,
              SelectionText,
              Panel,
              FaintText,
              SelectionAccent)
      ActionTone.Positive ->
          ActionToneStyle(
              labelBadgeBackground(Success),
              blendOver(Success.copy(alpha = 0.24f), Panel),
              blendOver(Success.copy(alpha = 0.28f), Panel),
              Success,
              Panel,
              FaintText,
              Success)
      ActionTone.PositivePrimary ->
          ActionToneStyle(
              Success,
              blendOver(Success.copy(alpha = 0.90f), EditorCanvas),
              SelectionSurface,
              OnActionFill,
              Panel,
              FaintText,
              Success,
              Success)
      ActionTone.Attention ->
          ActionToneStyle(
              labelBadgeBackground(Warning),
              blendOver(Warning.copy(alpha = 0.24f), Panel),
              blendOver(Warning.copy(alpha = 0.28f), Panel),
              Warning,
              Panel,
              FaintText,
              Warning)
      ActionTone.Destructive ->
          ActionToneStyle(
              labelBadgeBackground(Error),
              blendOver(Error.copy(alpha = 0.20f), Panel),
              blendOver(Error.copy(alpha = 0.21f), Panel),
              Error,
              Panel,
              FaintText,
              Error)
      ActionTone.Neutral ->
          ActionToneStyle(
              StrongSurface,
              ControlHover,
              SelectionSurface,
              PrimaryText,
              Panel,
              FaintText,
              ControlBorder)
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
                .clip(MiniOrcaShapes.control)
                .background(EditorCanvas)
                .border(
                    BorderStroke(
                        if (focused) 2.dp else 1.dp, if (focused) FocusAccent else ControlBorder),
                    MiniOrcaShapes.control)
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

/** A contained, selectable multiline input that shares the compact dark field treatment. */
@Composable
internal fun CompactMultilineField(
    value: TextFieldValue,
    onValueChange: (TextFieldValue) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    placeholder: String = "",
    minLines: Int = 3,
    textStyle: TextStyle = IdeTypography.compactBody,
) {
  val interactions = remember { MutableInteractionSource() }
  val focused by interactions.collectIsFocusedAsState()
  Column(modifier) {
    Text(
        label,
        color = SecondaryText,
        style = IdeTypography.section,
        modifier = Modifier.padding(bottom = 5.dp))
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        enabled = enabled,
        singleLine = false,
        interactionSource = interactions,
        textStyle = textStyle.copy(color = if (enabled) PrimaryText else FaintText),
        cursorBrush = SolidColor(FocusAccent),
        modifier =
            Modifier.fillMaxWidth()
                .heightIn(min = (minLines * 20).dp)
                .clip(MiniOrcaShapes.control)
                .background(EditorCanvas)
                .border(
                    BorderStroke(
                        if (focused) 2.dp else 1.dp, if (focused) FocusAccent else ControlBorder),
                    MiniOrcaShapes.control)
                .semantics { contentDescription = label },
        decorationBox = { input ->
          Box(Modifier.padding(horizontal = 10.dp, vertical = 8.dp)) {
            if (value.text.isEmpty() && placeholder.isNotBlank())
                Text(placeholder, color = FaintText, style = textStyle)
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
              selectedContent = style.selectedContent,
              disabledContent = style.disabledContent,
              border = if (enabled) style.border else ControlBorder),
      modifier = modifier,
      enabled = enabled,
      selected = selected,
      focusHighlight = focusHighlight,
      minimumHeight = densityStyle.height,
      contentPadding = densityStyle.contentPadding,
      shape = MiniOrcaShapes.control,
      content = content,
  )
}

@Composable
internal fun MiniOrcaTheme(content: @Composable () -> Unit) {
  IntUiTheme(isDark = true) {
    // Material's text primitive reads these locals, while all visual roles stay in the IDE palette.
    CompositionLocalProvider(LocalContentColor provides PrimaryText) {
      ProvideTextStyle(IdeTypography.body) { content() }
    }
  }
}

internal val ActivityRail = MiniOrcaPalette.activityRail
internal val ToolWindowSurface = MiniOrcaPalette.toolWindow
internal val EditorCanvas = MiniOrcaPalette.editorCanvas
internal val OverlaySurface = MiniOrcaPalette.overlay
internal val HeaderSurface = MiniOrcaPalette.header
internal val PaneSeparator = MiniOrcaPalette.paneSeparator
internal val ControlBorder = MiniOrcaPalette.controlBorder
internal val ControlHover = MiniOrcaPalette.controlHover

internal val AppBackground = ActivityRail
internal val Chrome = ActivityRail
internal val Panel = ToolWindowSurface
internal val Card = OverlaySurface
internal val StrongSurface = MiniOrcaPalette.control
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
internal val ResultAccent = MiniOrcaPalette.information
internal val Information = MiniOrcaPalette.information
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

// Resolve against the panel so a selected or highlighted parent cannot reduce label contrast.
internal fun labelBadgeBackground(tint: Color): Color = blendOver(tint.copy(alpha = 0.16f), Panel)

internal fun statusBadgeStyle(status: String): StatusBadgeStyle =
    when (status.lowercase()) {
      "fresh" -> StatusBadgeStyle("Fresh", Success)
      "stale" -> StatusBadgeStyle("Stale", Warning)
      "running" -> StatusBadgeStyle("Running", Information)
      "failed" -> StatusBadgeStyle("Failed", Error)
      "ignored",
      "excluded",
      "skipped" -> StatusBadgeStyle("Ignored", SecondaryText)
      else -> StatusBadgeStyle("Not analyzed", SecondaryText)
    }

internal fun analysisStatusTint(status: String?): Color =
    when (status) {
      "completed",
      "completed_empty" -> Success
      "queued",
      "running",
      "pausing",
      "canceling" -> Information
      "stale",
      "partial",
      "paused",
      "interrupted" -> Warning
      "failed" -> Error
      else -> SecondaryText
    }

@Composable
internal fun MiniOrcaPanel(
    modifier: Modifier = Modifier,
    raised: Boolean = false,
    contentPadding: PaddingValues = PaddingValues(MiniOrcaSpacing.roomy),
    content: @Composable () -> Unit,
) {
  Column(
      modifier
          .clip(MiniOrcaShapes.interactiveCard)
          .background(if (raised) Card else Panel)
          .border(BorderStroke(1.dp, Border), MiniOrcaShapes.interactiveCard)
          .padding(contentPadding)) {
        content()
      }
}

@Composable
internal fun IdeBusyIndicator(
    modifier: Modifier = Modifier,
    color: Color = FocusAccent,
    strokeWidth: androidx.compose.ui.unit.Dp = 2.dp,
) {
  val rotation =
      rememberInfiniteTransition(label = "ideBusyIndicator")
          .animateFloat(
              initialValue = 0f,
              targetValue = 360f,
              animationSpec =
                  infiniteRepeatable(tween(900, easing = LinearEasing), RepeatMode.Restart),
              label = "ideBusyIndicatorRotation")
  Canvas(modifier) {
    val stroke = strokeWidth.toPx()
    drawArc(
        color = color,
        startAngle = rotation.value - 90f,
        sweepAngle = 250f,
        useCenter = false,
        style =
            androidx.compose.ui.graphics.drawscope.Stroke(
                width = stroke, cap = androidx.compose.ui.graphics.StrokeCap.Round),
    )
  }
}

@Composable
internal fun IdeProgressBar(
    progress: Float,
    modifier: Modifier = Modifier,
    color: Color = FocusAccent,
    trackColor: Color = StrongSurface,
) {
  Box(modifier.clip(MiniOrcaShapes.pill).background(trackColor)) {
    Box(
        Modifier.fillMaxWidth(progress.coerceIn(0f, 1f))
            .fillMaxHeight()
            .background(color, MiniOrcaShapes.pill))
  }
}

@Composable
internal fun SectionLabel(label: String, modifier: Modifier = Modifier) {
  Text(label, color = PrimaryText, style = IdeTypography.section, modifier = modifier)
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
  IdeLabelBadge(
      label = style.label,
      tint = style.color,
      modifier = modifier,
      accessibleName = "Status: ${style.label}",
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

@Composable
internal fun WorkspaceSection(
    title: String? = null,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
  Column(
      modifier
          .fillMaxWidth()
          .clip(MiniOrcaShapes.interactiveCard)
          .background(Panel)
          .border(1.dp, PaneSeparator, MiniOrcaShapes.interactiveCard)
          .padding(16.dp),
      verticalArrangement = Arrangement.spacedBy(12.dp)) {
        title?.let {
          Text(
              it,
              color = ResultAccent,
              style = IdeTypography.workspaceHeading,
              modifier = Modifier.semantics { heading() })
        }
        content()
      }
}
