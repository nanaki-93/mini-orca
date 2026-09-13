package io.miniorca.desktop

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.graalvm.polyglot.Context
import org.graalvm.polyglot.Engine
import org.graalvm.polyglot.ResourceLimits
import org.graalvm.polyglot.Source

internal data class MermaidDocument(val svg: String, val source: String)

internal object MermaidRenderer {
  private val engine by lazy {
    Engine.newBuilder("js").option("engine.WarnInterpreterOnly", "false").build()
  }
  private val renderer by lazy {
    val script =
        requireNotNull(javaClass.getResourceAsStream("/mermaid/renderer.js")) {
              "Bundled Mermaid renderer is missing"
            }
            .bufferedReader()
            .use { it.readText() }
    Source.newBuilder("js", script, "mini-orca-mermaid.js").build()
  }
  private val cache =
      object : LinkedHashMap<String, MermaidDocument>(32, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, MermaidDocument>?) =
            size > 32
      }

  suspend fun render(source: String): MermaidDocument =
      withContext(Dispatchers.Default) {
        require(source.length <= 16384 && source.lines().size <= 256) {
          "Diagram exceeds the rendering limit"
        }
        require(Regex("^(flowchart|graph|sequenceDiagram)\\b").containsMatchIn(source.trim())) {
          "Use a Mermaid flowchart or sequence diagram"
        }
        require(
            !Regex("(?im)^\\s*(click|style|classDef|linkStyle)\\b|%%\\{|<[a-zA-Z!/]")
                .containsMatchIn(source)) {
              "Diagram contains unsupported styling, links or markup"
            }
        synchronized(cache) { cache[source] }
            ?.let {
              return@withContext it
            }
        ensureActive()
        val context =
            Context.newBuilder("js")
                .engine(engine)
                .allowAllAccess(false)
                .resourceLimits(
                    ResourceLimits.newBuilder().statementLimit(20_000_000, null).build())
                .build()
        val document = suspendCancellableCoroutine { continuation ->
          continuation.invokeOnCancellation { context.close(true) }
          try {
            val rendered =
                context.use {
                  it.eval(renderer)
                  val svg =
                      it.getBindings("js")
                          .getMember("MiniOrcaMermaid")
                          .getMember("render")
                          .execute(source, palette())
                          .asString()
                  require(svg.length <= 2_000_000) { "Rendered diagram exceeds the size limit" }
                  MermaidDocument(svg, source)
                }
            continuation.resume(rendered)
          } catch (exception: Exception) {
            continuation.resumeWithException(exception)
          }
        }
        ensureActive()
        synchronized(cache) { cache[source] = document }
        document
      }

  private fun palette(): String =
      Json.encodeToString(
          mapOf(
              "bg" to EditorCanvas.hex(),
              "fg" to PrimaryText.hex(),
              "muted" to SecondaryText.hex(),
              "line" to ResultAccent.hex(),
              "accent" to ResultAccent.hex(),
              "surface" to Panel.hex(),
              "border" to ControlBorder.hex(),
          ))

  private fun Color.hex(): String = "#%06x".format(toArgb() and 0xffffff)
}
