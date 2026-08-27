package io.miniorca.desktop

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.MaterialTheme
import androidx.compose.material.Shapes
import androidx.compose.material.Surface
import androidx.compose.material.Text
import androidx.compose.material.Typography
import androidx.compose.material.darkColors
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** The approved Focus Flow palette. Every desktop color is derived from these semantic roles. */
internal object MiniOrcaPalette {
    val appBackground = Color(0xFF080917)
    val surface = Color(0xFF101225)
    val raisedSurface = Color(0xFF171A31)
    val strongSurface = Color(0xFF222640)
    val border = Color(0xFF292D49)
    val primaryText = Color(0xFFF2F2FB)
    val secondaryText = Color(0xFF9297B6)
    val faintText = Color(0xFF5F6485)
    val primaryAccent = Color(0xFF9B8CFF)
    val onPrimaryAccent = Color(0xFF110D31)
    val secondaryAccent = Color(0xFF62D8EF)
    val success = Color(0xFF55DDB0)
    val warning = Color(0xFFFFC86E)
    val error = Color(0xFFFF7F9F)
    val codeKeyword = Color(0xFFC89FFF)
    val codeFunction = Color(0xFF7EDCF2)
    val codeString = Color(0xFFF0CA7D)
    val codeComment = Color(0xFF697093)
    val codeType = Color(0xFF7FE0BD)
}

internal object MiniOrcaSpacing {
    val compact = 4.dp
    val standard = 8.dp
    val roomy = 12.dp
    val section = 16.dp
}

internal val MiniOrcaShapes = Shapes(
    small = RoundedCornerShape(8.dp),
    medium = RoundedCornerShape(10.dp),
    large = RoundedCornerShape(14.dp),
)

internal val MiniOrcaTypography = Typography(
    defaultFontFamily = FontFamily.Default,
    h6 = androidx.compose.ui.text.TextStyle(fontWeight = FontWeight.SemiBold, fontSize = 18.sp),
    body1 = androidx.compose.ui.text.TextStyle(fontSize = 13.sp),
    body2 = androidx.compose.ui.text.TextStyle(fontSize = 12.sp),
    caption = androidx.compose.ui.text.TextStyle(fontSize = 11.sp),
)

@Composable
internal fun MiniOrcaTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colors = darkColors(
            primary = MiniOrcaPalette.primaryAccent,
            secondary = MiniOrcaPalette.secondaryAccent,
            background = MiniOrcaPalette.appBackground,
            surface = MiniOrcaPalette.surface,
            error = MiniOrcaPalette.error,
            onPrimary = MiniOrcaPalette.onPrimaryAccent,
            onBackground = MiniOrcaPalette.primaryText,
            onSurface = MiniOrcaPalette.primaryText,
            onError = MiniOrcaPalette.appBackground,
        ),
        typography = MiniOrcaTypography,
        shapes = MiniOrcaShapes,
        content = content,
    )
}

internal val AppBackground = MiniOrcaPalette.appBackground
internal val Panel = MiniOrcaPalette.surface
internal val Card = MiniOrcaPalette.raisedSurface
internal val StrongSurface = MiniOrcaPalette.strongSurface
internal val Border = MiniOrcaPalette.border
internal val PrimaryText = MiniOrcaPalette.primaryText
internal val SecondaryText = MiniOrcaPalette.secondaryText
internal val FaintText = MiniOrcaPalette.faintText
internal val Accent = MiniOrcaPalette.primaryAccent
internal val OnAccent = MiniOrcaPalette.onPrimaryAccent
internal val CyanAccent = MiniOrcaPalette.secondaryAccent
internal val Success = MiniOrcaPalette.success
internal val Warning = MiniOrcaPalette.warning
internal val Error = MiniOrcaPalette.error
internal val CodeKeyword = MiniOrcaPalette.codeKeyword
internal val CodeFunction = MiniOrcaPalette.codeFunction
internal val CodeString = MiniOrcaPalette.codeString
internal val CodeComment = MiniOrcaPalette.codeComment
internal val CodeType = MiniOrcaPalette.codeType

data class StatusBadgeStyle(val label: String, val color: Color)

internal fun statusBadgeStyle(status: String): StatusBadgeStyle = when (status.lowercase()) {
    "fresh" -> StatusBadgeStyle("Fresh", Success)
    "stale" -> StatusBadgeStyle("Stale", Warning)
    "running" -> StatusBadgeStyle("Running", Warning)
    "failed" -> StatusBadgeStyle("Failed", Error)
    else -> StatusBadgeStyle("Not analyzed", SecondaryText)
}

internal fun badgeColor(status: String): Color = statusBadgeStyle(status).color

@Composable
internal fun FocusFlowPanel(
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
    Text(label, color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold, modifier = modifier)
}

@Composable
internal fun StatusBadge(status: String, modifier: Modifier = Modifier) {
    val style = statusBadgeStyle(status)
    Text(
        style.label,
        color = style.color,
        fontSize = 10.sp,
        fontWeight = FontWeight.SemiBold,
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        modifier = modifier
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
    FocusFlowPanel(modifier = modifier, raised = true) {
        Text(title, color = PrimaryText, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(MiniOrcaSpacing.standard))
        Text(message, color = accent, fontSize = 13.sp)
    }
}

internal fun formatBytes(bytes: Long): String = when {
    bytes < 1024 -> "$bytes B"
    bytes < 1024 * 1024 -> "${bytes / 1024} KB"
    else -> "${bytes / (1024 * 1024)} MB"
}
