package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import java.nio.file.Files

/** Native verification only: production panes, synthetic data, no API client or provider. */
fun main() {
  val directory = Files.createTempDirectory("mini-orca-acceptance-long-project-name-")
  Files.writeString(directory.resolve("main.go"), "package main\n")
  miniOrcaApplication(
      createTerminal = {
        DesktopTerminalWorkspace { path ->
          DesktopTerminalSession(
              path,
              shell = "/bin/zsh",
              environment =
                  System.getenv() +
                      mapOf("ZDOTDIR" to directory.toString(), "HISTFILE" to "/dev/null"))
        }
      }) { terminal ->
        NativeAcceptanceScreen(terminal, directory.toString())
      }
}

internal val acceptanceRunStates =
    listOf(
        "not_started",
        "running",
        "paused",
        "interrupted",
        "completed",
        "completed_empty",
        "partial",
        "canceled",
        "failed",
        "unavailable",
        "stale")

private val acceptancePages =
    listOf(
        "Progress",
        "Bugs",
        "Performance",
        "Security",
        "Editor",
        "Terminal states",
        "Shell",
        "Launch failure")
private const val acceptanceFailure =
    "Synthetic provider failure for internal/platform/transport/handlers/long_request_handler.go: the response could not be validated. Retained results remain available; a fresh analysis requires explicit admission."

@Composable
private fun NativeAcceptanceScreen(terminal: DesktopTerminalWorkspace, directory: String) {
  var page by remember { mutableStateOf("Progress") }
  var stateIndex by remember { mutableStateOf(0) }
  var scale by remember { mutableStateOf(1f) }
  val state = acceptanceRunStates[stateIndex]
  val density = LocalDensity.current
  val failedTerminal = remember {
    DesktopTerminalWorkspace { path ->
      DesktopTerminalSession(
          path,
          factory =
              TerminalProcessFactory {
                error(
                    "Synthetic shell launch failure in a temporary project. No process was created. Use + to retry; the project source remains unchanged.")
              })
    }
  }
  DisposableEffect(failedTerminal) { onDispose { failedTerminal.close() } }
  LaunchedEffect(page) {
    if (page == "Launch failure") failedTerminal.activate(directory)
    if (page == "Shell") terminal.activate(directory)
  }
  TerminalFocusReturnEffect(terminal) { page = "Progress" }
  CompositionLocalProvider(LocalDensity provides Density(density.density, scale)) {
    Column(Modifier.fillMaxSize().background(ToolWindowSurface)) {
      Text(
          "Verification fixture · synthetic data",
          modifier = Modifier.padding(8.dp),
          style = IdeTypography.resultLabel)
      FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        acceptancePages.forEach { destination ->
          ChromeButton(onClick = { page = destination }, selected = page == destination) {
            Text(destination)
          }
        }
      }
      FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        ChromeButton(onClick = { stateIndex = (stateIndex + 1) % acceptanceRunStates.size }) {
          Text("Next state")
        }
        Text("State: $state", modifier = Modifier.padding(8.dp))
        listOf(1f, 1.25f, 1.5f).forEach { value ->
          ChromeButton(onClick = { scale = value }, selected = scale == value) {
            Text("${(value * 100).toInt()}%")
          }
        }
      }
      IdeHorizontalSeparator()
      BoxWithConstraints(Modifier.weight(1f).fillMaxWidth()) {
        key(page, state) {
          when (page) {
            "Progress" ->
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(),
                        ProjectAnalysisRunState(run = acceptanceRun(state))),
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
            "Bugs",
            "Performance",
            "Security" -> AcceptanceResultPane(page.lowercase(), state)
            "Editor" -> EditorVisualFixture(maxWidth.value)
            "Terminal states" -> TerminalStateMatrix()
            "Shell" -> AcceptanceTerminal(terminal, directory)
            "Launch failure" -> AcceptanceTerminal(failedTerminal, directory)
          }
        }
      }
    }
  }
}

internal fun acceptanceRun(status: String): AnalysisRun? {
  if (status == "not_started") return null
  val hasEvidence = status in setOf("completed", "completed_empty", "partial", "stale")
  val count = if (hasEvidence) (if (status == "completed_empty") 0 else 1) else null
  val coverage =
      when (status) {
        "running" -> AnalysisRunCoverage(total = 1, running = 1)
        "failed" -> AnalysisRunCoverage(total = 1, failed = 1)
        "unavailable" -> AnalysisRunCoverage(total = 1, unavailable = 1)
        "partial" -> AnalysisRunCoverage(total = 1, partial = 1)
        "completed",
        "completed_empty",
        "stale" -> AnalysisRunCoverage(total = 1, succeeded = 1)
        else -> AnalysisRunCoverage(total = 1, pending = 1)
      }
  val stageStatus =
      when {
        status == "failed" -> "failed"
        status == "unavailable" -> "unavailable"
        status == "running" -> "running"
        hasEvidence -> "succeeded"
        else -> "queued"
      }
  return analysisRunFixture()
      .copy(
          status = status,
          reason = if (status in setOf("failed", "partial")) acceptanceFailure else "",
          sections =
              analysisRunFixture().sections.map {
                it.copy(status = status, coverage = coverage, findingCount = count)
              },
          files =
              listOf(
                  AnalysisRunFile(
                      "internal/platform/transport/handlers/long_request_handler.go",
                      "base",
                      "Go",
                      listOf("semantic", "performance", "security_rules", "security_ai").map { stage
                        ->
                        val failed =
                            status == "failed" || status == "partial" && stage == "performance"
                        AnalysisStageProgress(
                            stage,
                            if (failed) "failed" else stageStatus,
                            if (hasEvidence || failed) 1 else 0,
                            false,
                            if (failed) null else count,
                            reason = if (failed) acceptanceFailure else "")
                      })))
}

internal fun acceptanceResultPage(category: String, status: String): AnalysisResultPageState {
  val original =
      when (category) {
        "performance" -> performancePageFixture()
        "security" -> securityPageFixture()
        else -> resultPageFixture("bugs")
      }
  val run = acceptanceRun(status)
  if (run == null) return original.copy(run = null, section = AnalysisSectionState())
  val progress = run.sections.first { it.category == category }
  val hasRows = status in setOf("completed", "partial", "stale")
  val results =
      original.results!!.copy(
          identity = run.identity,
          progress = progress,
          semantic =
              if (hasRows)
                  original.semantic.map {
                    it.copy(
                        title = "Return the missing error",
                        message = "Check the returned error before processing another request.",
                        severity = "high",
                        confidence = "suggested",
                        status = "open",
                        freshness = "fresh")
                  }
              else emptyList(),
          performance = if (hasRows) original.results!!.performance else emptyList(),
          security = if (hasRows) original.results!!.security else emptyList())
  return original.copy(
      run = run,
      section =
          AnalysisSectionState(
              results,
              loading = status == "running",
              error = if (status == "failed") acceptanceFailure else null))
}

@Composable
internal fun AcceptanceResultPane(category: String, status: String) {
  var page by remember(category, status) { mutableStateOf(acceptanceResultPage(category, status)) }
  val actions = FindingActions({}, {}, { _, _ -> })
  when (category) {
    "performance" ->
        PerformanceWorkspacePane(
            PerformanceWorkspacePaneState(page, resultIndexFixture()),
            PerformanceWorkspaceActions(
                { _, _ -> }, { _, _ -> }, {}, { page = page.copy(path = it) }, actions))
    "security" ->
        SecurityWorkspacePane(
            SecurityWorkspacePaneState(page, resultIndexFixture()),
            SecurityWorkspaceActions({}, {}, {}, { page = page.copy(path = it) }, actions))
    else ->
        BugsWorkspacePane(
            BugsWorkspacePaneState(page.semantic, null, false, page),
            BugsWorkspaceActions(actions, {}, {}, {}, { page = page.copy(path = it) }))
  }
}

@Composable
internal fun TerminalStateMatrix() {
  Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
    TerminalSessionPhase.entries.forEach { phase ->
      Text(phase.name, style = IdeTypography.resultLabel, modifier = Modifier.padding(8.dp))
      val session =
          TerminalSessionState(
              phase,
              exitCode = if (phase == TerminalSessionPhase.Exited) 0 else null,
              error =
                  if (phase == TerminalSessionPhase.Failed) "Synthetic launch failure" else null)
      TerminalBar(
          TerminalWorkspaceState(
              tabs = listOf(TerminalTabState(1, "Shell 1", session)), activeTabId = 1),
          false,
          {},
          TerminalTabActions({}, {}, {}))
    }
    Text("Cleanup pending", style = IdeTypography.resultLabel, modifier = Modifier.padding(8.dp))
    TerminalBar(
        TerminalWorkspaceState(
            tabs =
                listOf(
                    TerminalTabState(
                        1,
                        "Shell 1",
                        TerminalSessionState(TerminalSessionPhase.Running, cleanupPending = true))),
            activeTabId = 1),
        false,
        {},
        TerminalTabActions({}, {}, {}))
  }
}

@Composable
private fun AcceptanceTerminal(terminal: DesktopTerminalWorkspace, directory: String) {
  val state by terminal.state.collectAsState()
  Column(Modifier.fillMaxSize()) {
    TerminalBar(
        state,
        false,
        {},
        TerminalTabActions(
            terminal::selectShell,
            { terminal.createShell(directory) },
            { terminal.closeSession(it) }))
    TerminalToolWindow(terminal, Modifier.weight(1f).fillMaxWidth())
  }
}
