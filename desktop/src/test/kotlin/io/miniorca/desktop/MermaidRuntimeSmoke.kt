package io.miniorca.desktop

import androidx.compose.ui.graphics.toArgb
import kotlinx.coroutines.runBlocking

/**
 * Run with the packaged runtime and application jars to check the offline rendering dependencies.
 */
fun main() = runBlocking {
  listOf(
          """flowchart TD
            subgraph Client["Desktop client"]
              Summary["Summary"] --> Api["API client"]
            end
            Api --> Daemon["Go daemon"]""",
          """sequenceDiagram
            participant C as Client
            participant A as API
            C->>A: Request
            alt accepted
              A-->>C: Result
            else rejected
              A-->>C: Error
            end""",
      )
      .forEach { source ->
        val document = MermaidRenderer.render(source)
        val image = renderMermaidImage(document.svg)
        val pixels = IntArray(image.bitmap.width * image.bitmap.height)
        image.bitmap.readPixels(pixels)
        check(pixels.count { it == PrimaryText.toArgb() } > 20) { "Diagram labels are missing" }
      }
  println("Packaged Mermaid rendering passed.")
}
