package io.miniorca.desktop

import androidx.compose.ui.graphics.toArgb
import kotlinx.coroutines.runBlocking

/**
 * Run with the packaged runtime and application jars to check the offline rendering dependencies.
 */
fun main() = runBlocking {
  val document = MermaidRenderer.render("flowchart TD\n A[Client] --> B[Server]")
  val image = renderMermaidImage(document.svg)
  val pixels = IntArray(image.bitmap.width * image.bitmap.height)
  image.bitmap.readPixels(pixels)
  check(pixels.count { it == PrimaryText.toArgb() } > 20) { "Diagram labels are missing" }
  println("Packaged Mermaid rendering passed.")
}
