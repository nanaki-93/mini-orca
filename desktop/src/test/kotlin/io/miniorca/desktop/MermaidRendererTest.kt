package io.miniorca.desktop

import androidx.compose.ui.graphics.toArgb
import kotlin.test.Test
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue
import kotlinx.coroutines.runBlocking

class MermaidRendererTest {
  @Test
  fun rendersMermaidBranchingGraphAndSequenceOffline() = runBlocking {
    listOf(
            "flowchart LR\n A[Client] --> B{Valid?}\n B -->|yes| C[Store]\n B -->|no| D[Reject]",
            "sequenceDiagram\n participant C as Client\n participant A as API\n C->>A: Request\n A-->>C: Response")
        .forEach { source ->
          val document = MermaidRenderer.render(source)
          assertTrue(document.svg.contains("<svg"))
          assertTrue(document.svg.contains("Client"))
          assertTrue(!document.svg.contains("var(--"))
          assertTrue(!document.svg.contains("<style>"))
          assertTrue(!document.svg.contains("fonts.googleapis.com"))
        }
  }

  @Test
  fun renderedImagesIncludeVisibleNodeLabels() = runBlocking {
    val document = MermaidRenderer.render("flowchart TD\n A[Visible label]")
    val image = renderMermaidImage(document.svg)
    val pixels = IntArray(image.bitmap.width * image.bitmap.height)
    image.bitmap.readPixels(pixels)
    assertTrue(
        pixels.count { it == PrimaryText.toArgb() } > 20,
        "Node label must be visible in the rendered image")
  }

  @Test
  fun invalidActiveAndOversizedInputsRetainFailureMeaning() = runBlocking {
    listOf(
            "Not a Mermaid diagram",
            "flowchart TD\n A[Node]\n click A \"https://example.com\"",
            "flowchart TD\n A[<img src='https://example.com/image'>]",
            "flowchart TD\n" + "A".repeat(16384),
        )
        .forEach { source ->
          assertFailsWith<IllegalArgumentException> { MermaidRenderer.render(source) }
        }
  }
}
