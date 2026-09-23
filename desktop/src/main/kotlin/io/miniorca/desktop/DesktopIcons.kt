package io.miniorca.desktop

import androidx.compose.foundation.layout.size
import androidx.compose.material.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** Shared 24-unit line drawings keep small chrome icons legible and consistent. */
internal enum class DesktopIcon(private val path: String) {
  Project("M3 5h6l2 2h10v13H3Z"),
  Go("M2 9h3 M1 12h3 M2 15h3 M13 8H9l-2 4 2 4h4v-4h-3 M19 8h-2l-2 4 2 4h2l3-4Z"),
  Java(
      "M5 10h12v5a4 4 0 0 1-4 4H9a4 4 0 0 1-4-4Z M17 11h2a2 2 0 0 1 0 4h-2 M3 22h16 M10 7c-5-3 5-3 1-6 M14 8c-3-2 4-3 3-5"),
  Kotlin("M4 3h16l-9 9 9 9H4Z M4 12l9-9 M4 12l9 9"),
  Summary("M5 3h9l5 5v13H5Z M14 3v6h5 M8 12h8 M8 16h5"),
  Analysis("M3 21V11h4v10 M10 21V7h4v14 M17 21V3h4v18 M2 7l6-4 5 1 7-3"),
  Performance("M4 18a9 9 0 1 1 16 0 M8 18h8 M12 14l5-6 M5 11H3 M12 5V3 M19 11h2"),
  Security("M12 3l8 3v6c0 5-8 9-8 9s-8-4-8-9V6Z M8 12l3 3 5-6"),
  Problems(
      "M8 8h8v9a4 4 0 0 1-8 0Z M9 8V6a3 3 0 0 1 6 0v2 M12 9v11 M4 6l4 4 M20 6l-4 4 M3 13h5 M16 13h5 M4 21l4-4 M16 17l4 4"),
  Editor("M3 4h18v16H3Z M3 8h18 M8 11l-3 3 3 3 M16 11l3 3-3 3 M13 10l-2 8"),
  Search("M10 3a7 7 0 1 1 0 14a7 7 0 1 1 0-14 M15 15l6 6"),
  Branch(
      "M6 3a2 2 0 1 1 0 4a2 2 0 1 1 0-4 M6 17a2 2 0 1 1 0 4a2 2 0 1 1 0-4 M18 3a2 2 0 1 1 0 4a2 2 0 1 1 0-4 M6 7v10 M18 7v2c0 4-12 2-12 6"),
  Add("M12 4v16 M4 12h16"),
  Refresh("M20 9a8 8 0 0 0-14-4L3 8 M3 3v5h5 M4 15a8 8 0 0 0 14 4l3-3 M16 16h5v5"),
  More("M4 12h1 M11.5 12h1 M19 12h1"),
  ChevronRight("M9 5l7 7-7 7"),
  ChevronDown("M5 9l7 7 7-7"),
  Folder("M3 5h6l2 2h10v13H3Z"),
  File("M5 3h9l5 5v13H5Z M14 3v6h5"),
  Code("M8 6L3 12l5 6 M16 6l5 6-5 6 M14 4l-4 16"),
  Document("M5 3h9l5 5v13H5Z M14 3v6h5 M8 12h8 M8 16h6"),
  Run("M7 3l14 9-14 9Z"),
  Close("M6 6l12 12 M6 18L18 6"),
  Lock("M6 10h12v10H6Z M8 10V7a4 4 0 0 1 8 0v3"),
  Check("M4 12l5 5L20 6"),
  ;

  val image: ImageVector by lazy {
    ImageVector.Builder(name, 24.dp, 24.dp, 24f, 24f)
        .addPath(
            pathData = PathParser().parsePathString(path).toNodes(),
            stroke = SolidColor(Color.Black),
            strokeLineWidth = 1.5f,
            strokeLineCap = StrokeCap.Round,
            strokeLineJoin = StrokeJoin.Round,
        )
        .build()
  }
}

@Composable
internal fun DesktopLineIcon(
    icon: DesktopIcon,
    description: String,
    modifier: Modifier = Modifier,
    tint: Color = SecondaryText,
    iconSize: Dp = 20.dp,
) {
  Icon(icon.image, description, modifier.size(iconSize), tint)
}
