package io.miniorca.desktop

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** The small, dependency-free line icon vocabulary shared by desktop chrome. */
internal enum class DesktopIcon {
  Project,
  Summary,
  Analysis,
  Performance,
  Problems,
  Editor,
  Search,
  Branch,
  Add,
  Refresh,
  More,
  ChevronRight,
  ChevronDown,
  Folder,
  File,
  Run,
  Debug,
  Settings,
  Help,
  Close,
  Terminal,
}

@Composable
internal fun DesktopLineIcon(
    icon: DesktopIcon,
    description: String,
    modifier: Modifier = Modifier,
    tint: Color = SecondaryText,
    iconSize: Dp = 20.dp,
) {
  Canvas(
      modifier = modifier.size(iconSize).semantics { contentDescription = description },
  ) {
    val stroke = Stroke(width = minOf(this.size.width, this.size.height) / 12f)
    val center = Offset(this.size.width / 2, this.size.height / 2)
    val quarter = this.size.width / 4
    fun line(startX: Float, startY: Float, endX: Float, endY: Float) =
        drawLine(tint, Offset(startX, startY), Offset(endX, endY), stroke.width, StrokeCap.Round)
    fun outline(inset: Float = quarter) =
        drawRoundRect(
            tint,
            Offset(inset, inset),
            Size(this.size.width - inset * 2, this.size.height - inset * 2),
            cornerRadius = androidx.compose.ui.geometry.CornerRadius(2.dp.toPx()),
            style = stroke)

    when (icon) {
      DesktopIcon.Project,
      DesktopIcon.Folder -> {
        drawRoundRect(
            tint,
            Offset(quarter * 0.55f, quarter * 1.05f),
            Size(this.size.width - quarter * 1.1f, this.size.height - quarter * 1.6f),
            cornerRadius = androidx.compose.ui.geometry.CornerRadius(2.dp.toPx()),
            style = stroke)
        line(quarter * 0.9f, quarter, this.size.width * 0.48f, quarter)
      }
      DesktopIcon.Summary -> {
        outline()
        line(quarter * 1.35f, quarter * 1.5f, this.size.width - quarter, quarter * 1.5f)
        line(quarter * 1.35f, center.y, this.size.width - quarter, center.y)
        line(
            quarter * 1.35f,
            this.size.height - quarter * 1.5f,
            this.size.width * 0.65f,
            this.size.height - quarter * 1.5f)
      }
      DesktopIcon.Analysis -> {
        drawCircle(tint, radius = quarter * 0.86f, center = center, style = stroke)
        line(
            center.x + quarter * 0.62f,
            center.y + quarter * 0.62f,
            this.size.width - quarter * 0.5f,
            this.size.height - quarter * 0.5f)
      }
      DesktopIcon.Performance -> {
        outline()
        line(quarter * 1.1f, this.size.height - quarter * 1.05f, quarter * 1.6f, center.y)
        line(quarter * 1.6f, center.y, center.x, this.size.height - quarter * 1.5f)
        line(
            center.x,
            this.size.height - quarter * 1.5f,
            this.size.width - quarter * 0.85f,
            quarter * 1.05f)
      }
      DesktopIcon.Problems,
      DesktopIcon.Debug -> {
        drawCircle(tint, radius = quarter * 0.9f, center = center, style = stroke)
        line(center.x, quarter * 1.38f, center.x, center.y + quarter * 0.35f)
        drawCircle(
            tint,
            radius = stroke.width * 0.42f,
            center = Offset(center.x, this.size.height - quarter * 1.38f))
      }
      DesktopIcon.Editor -> {
        outline()
        line(quarter * 1.2f, center.y, this.size.width - quarter * 1.2f, center.y)
        line(center.x, quarter * 1.2f, center.x, this.size.height - quarter * 1.2f)
      }
      DesktopIcon.Search -> {
        drawCircle(
            tint,
            radius = quarter * 0.72f,
            center = Offset(center.x - quarter * 0.3f, center.y - quarter * 0.3f),
            style = stroke)
        line(
            center.x + quarter * 0.25f,
            center.y + quarter * 0.25f,
            this.size.width - quarter * 0.4f,
            this.size.height - quarter * 0.4f)
      }
      DesktopIcon.Branch -> {
        drawCircle(tint, radius = stroke.width * 0.6f, center = Offset(quarter, quarter))
        drawCircle(
            tint,
            radius = stroke.width * 0.6f,
            center = Offset(quarter, this.size.height - quarter))
        drawCircle(
            tint,
            radius = stroke.width * 0.6f,
            center = Offset(this.size.width - quarter, center.y))
        line(quarter, quarter + stroke.width, quarter, this.size.height - quarter - stroke.width)
        line(quarter, center.y, this.size.width - quarter, center.y)
      }
      DesktopIcon.Add -> {
        line(center.x, quarter, center.x, this.size.height - quarter)
        line(quarter, center.y, this.size.width - quarter, center.y)
      }
      DesktopIcon.Refresh -> {
        drawArc(
            tint,
            40f,
            270f,
            false,
            Offset(quarter, quarter),
            Size(this.size.width - quarter * 2, this.size.height - quarter * 2),
            style = stroke)
        line(
            this.size.width - quarter * 0.85f,
            quarter * 1.2f,
            this.size.width - quarter * 0.2f,
            quarter * 1.2f)
      }
      DesktopIcon.More -> {
        repeat(3) { index ->
          drawCircle(
              tint,
              radius = stroke.width * 0.52f,
              center = Offset(quarter + index * quarter, center.y))
        }
      }
      DesktopIcon.ChevronRight -> {
        line(quarter * 1.35f, quarter, this.size.width - quarter * 1.1f, center.y)
        line(
            this.size.width - quarter * 1.1f, center.y, quarter * 1.35f, this.size.height - quarter)
      }
      DesktopIcon.ChevronDown -> {
        line(quarter, quarter * 1.35f, center.x, this.size.height - quarter * 1.1f)
        line(
            center.x, this.size.height - quarter * 1.1f, this.size.width - quarter, quarter * 1.35f)
      }
      DesktopIcon.File -> {
        outline()
        line(this.size.width * 0.58f, quarter, this.size.width - quarter, this.size.height * 0.42f)
      }
      DesktopIcon.Run -> {
        drawLine(
            tint,
            Offset(quarter * 1.2f, quarter),
            Offset(quarter * 1.2f, this.size.height - quarter),
            stroke.width,
            StrokeCap.Round)
        drawLine(
            tint,
            Offset(quarter * 1.2f, quarter),
            Offset(this.size.width - quarter, center.y),
            stroke.width,
            StrokeCap.Round)
        drawLine(
            tint,
            Offset(this.size.width - quarter, center.y),
            Offset(quarter * 1.2f, this.size.height - quarter),
            stroke.width,
            StrokeCap.Round)
      }
      DesktopIcon.Settings -> {
        drawCircle(tint, radius = quarter * 0.72f, center = center, style = stroke)
        repeat(4) { index ->
          val offset = if (index % 2 == 0) quarter else -quarter
          if (index < 2) line(center.x + offset, center.y, center.x + offset * 1.5f, center.y)
          else line(center.x, center.y + offset, center.x, center.y + offset * 1.5f)
        }
      }
      DesktopIcon.Help -> {
        drawCircle(tint, radius = quarter, center = center, style = stroke)
        line(
            center.x - quarter * 0.3f,
            center.y - quarter * 0.3f,
            center.x,
            center.y - quarter * 0.55f)
        line(
            center.x,
            center.y - quarter * 0.55f,
            center.x + quarter * 0.35f,
            center.y - quarter * 0.1f)
        drawCircle(
            tint,
            radius = stroke.width * 0.42f,
            center = Offset(center.x, this.size.height - quarter * 1.3f))
      }
      DesktopIcon.Close -> {
        line(quarter, quarter, this.size.width - quarter, this.size.height - quarter)
        line(this.size.width - quarter, quarter, quarter, this.size.height - quarter)
      }
      DesktopIcon.Terminal -> {
        outline()
        line(quarter * 1.25f, quarter * 1.35f, center.x, center.y)
        line(center.x, center.y, quarter * 1.25f, this.size.height - quarter * 1.35f)
        line(
            center.x + quarter * 0.35f,
            this.size.height - quarter * 1.35f,
            this.size.width - quarter * 1.15f,
            this.size.height - quarter * 1.35f)
      }
    }
  }
}
