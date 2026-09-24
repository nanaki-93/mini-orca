package io.miniorca.desktop

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.Density
import java.util.UUID
import java.util.prefs.Preferences

/**
 * Native-only entry point for the disposable loopback daemon acceptance journey.
 *
 * The shell launcher owns the temporary project, daemon, responder and process cleanup. This
 * harness keeps the user's recent-project and layout preferences outside the normal app node, then
 * starts the production app and presenter against that local daemon.
 */
fun main() {
  val projectPath = requireEnvironment("MINI_ORCA_ACCEPTANCE_PROJECT")
  val daemonUrl = requireEnvironment("MINI_ORCA_URL")
  val textScale =
      System.getenv("MINI_ORCA_ACCEPTANCE_TEXT_SCALE")?.let { value ->
        require(value in setOf("1", "1.25", "1.5")) { "Unsupported acceptance text scale: $value" }
        value.toFloat()
      } ?: 1f
  val preferences =
      Preferences.userRoot().node("io/miniorca/desktop/native-acceptance/${UUID.randomUUID()}")
  val api = ApiClient(daemonUrl)
  api.importProject(projectPath, confirmRemoteProvider = false)
  api.restoreProject(projectPath)
  val lastProjectStore = LastProjectStore(preferences.node("recent-project"))
  val layoutStore = DesktopLayoutStore(preferences.node("layout"))
  lastProjectStore.save(projectPath)
  Runtime.getRuntime()
      .addShutdownHook(
          Thread {
            runCatching {
              preferences.removeNode()
              preferences.flush()
            }
          })

  miniOrcaApplication { terminal ->
    val density = LocalDensity.current
    CompositionLocalProvider(LocalDensity provides Density(density.density, textScale)) {
      NativeAcceptanceApp(
          terminal = terminal,
          api = api,
          lastProjectStore = lastProjectStore,
          layoutStore = layoutStore)
    }
  }
}

@Composable
private fun NativeAcceptanceApp(
    terminal: DesktopTerminalWorkspace,
    api: ApiClient,
    lastProjectStore: LastProjectStore,
    layoutStore: DesktopLayoutStore,
) {
  MiniOrcaApp(
      terminal = terminal,
      api = api,
      lastProjectStore = lastProjectStore,
      layoutStore = layoutStore)
}

private fun requireEnvironment(name: String): String =
    System.getenv(name)?.takeIf { it.isNotBlank() }
        ?: error("$name must name the disposable native acceptance environment")
