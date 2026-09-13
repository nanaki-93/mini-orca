package io.miniorca.desktop

import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.toComposeImageBitmap
import com.github.weisj.jsvg.parser.LoaderContext
import com.github.weisj.jsvg.parser.SVGLoader
import com.github.weisj.jsvg.parser.resources.ResourcePolicy
import java.awt.RenderingHints
import java.awt.image.BufferedImage
import kotlin.math.ceil
import kotlin.math.min
import kotlin.math.sqrt
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

internal data class MermaidImage(val bitmap: ImageBitmap, val width: Float, val height: Float)

internal suspend fun renderMermaidImage(svg: String): MermaidImage =
    withContext(Dispatchers.Default) {
      val document =
          svg.byteInputStream().use {
            SVGLoader()
                .load(
                    it,
                    null,
                    LoaderContext.builder().externalResourcePolicy(ResourcePolicy.DENY_ALL).build())
          } ?: error("Unable to decode the diagram image")
      val size = document.size()
      require(size.width in 1f..12000f && size.height in 1f..12000f) {
        "Diagram dimensions are unsupported"
      }
      // Preserve sharp text at larger font scales and zoom, with a bounded bitmap allocation.
      val scale = min(3.0, sqrt(16_000_000.0 / (size.width * size.height)))
      val image =
          BufferedImage(
              ceil(size.width * scale).toInt(),
              ceil(size.height * scale).toInt(),
              BufferedImage.TYPE_INT_ARGB)
      val graphics = image.createGraphics()
      try {
        graphics.setRenderingHint(
            RenderingHints.KEY_ANTIALIASING, RenderingHints.VALUE_ANTIALIAS_ON)
        graphics.setRenderingHint(
            RenderingHints.KEY_TEXT_ANTIALIASING, RenderingHints.VALUE_TEXT_ANTIALIAS_ON)
        graphics.scale(scale, scale)
        document.render(null, graphics)
      } finally {
        graphics.dispose()
      }
      MermaidImage(image.toComposeImageBitmap(), size.width, size.height)
    }
