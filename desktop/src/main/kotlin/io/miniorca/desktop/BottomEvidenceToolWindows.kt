package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class BottomToolWindowSummary(
    val text: String,
    val attention: Boolean = false,
)

internal data class ChecksToolWindowState(
    val project: ProjectAnalysis?,
    val selected: ProjectFileInfo?,
    val editor: EditableDraftState?,
    val draft: DeclarationDraft?,
    val checks: DraftCheckReport?,
    val checksRunning: Boolean,
)

internal data class CheckEvidenceRow(
    val name: String,
    val required: Boolean,
    val status: ReviewEvidenceStatus,
    val command: String,
    val output: String,
)

internal data class ChecksToolWindowPresentation(
    val evidence: ReviewEvidenceUiState,
    val diagnostics: List<DeclarationFinding>,
    val checks: List<CheckEvidenceRow>,
) {
  val summary: BottomToolWindowSummary
    get() =
        when (evidence.checks.status) {
          ReviewEvidenceStatus.Failed -> BottomToolWindowSummary("Checks failed", attention = true)
          ReviewEvidenceStatus.Stale -> BottomToolWindowSummary("Checks stale", attention = true)
          ReviewEvidenceStatus.Running -> BottomToolWindowSummary("Checks running")
          ReviewEvidenceStatus.Passed -> BottomToolWindowSummary("Checks passed")
          ReviewEvidenceStatus.Skipped -> BottomToolWindowSummary("Checks skipped")
          ReviewEvidenceStatus.Missing -> BottomToolWindowSummary("No focused checks")
        }
}

internal fun checksToolWindowPresentation(
    state: ChecksToolWindowState
): ChecksToolWindowPresentation =
    ChecksToolWindowPresentation(
        evidence =
            reviewEvidenceUiState(
                state.project,
                state.selected,
                state.editor,
                state.draft,
                state.checks,
                state.checksRunning),
        diagnostics = state.editor?.diagnostics.orEmpty().take(MAX_DIAGNOSTICS),
        checks =
            state.checks?.checks.orEmpty().map { check ->
              CheckEvidenceRow(
                  name = check.name.ifBlank { "Unnamed focused check" },
                  required = check.required,
                  status = checkStatus(check.state),
                  command = check.command.joinToString(" "),
                  output = sanitizedOutputText(check.output),
              )
            },
    )

@Composable
internal fun ChecksToolWindow(
    presentation: ChecksToolWindowPresentation,
    modifier: Modifier = Modifier,
) {
  Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(12.dp)) {
    MiniOrcaPanel(Modifier.fillMaxWidth(), raised = true) {
      SectionLabel("Checks")
      Text(
          "Evidence only; run checks and Apply stay in Review.",
          color = SecondaryText,
          fontSize = 11.sp,
          modifier = Modifier.padding(top = 5.dp))
      EvidenceTextRow(presentation.evidence.identity)
      EvidenceTextRow(presentation.evidence.validation)
      EvidenceTextRow(presentation.evidence.checks)
    }
    if (presentation.diagnostics.isNotEmpty()) {
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(Modifier.fillMaxWidth()) {
        SectionLabel("VALIDATION DIAGNOSTICS")
        SelectionContainer {
          Column(Modifier.padding(top = 5.dp)) {
            presentation.diagnostics.forEach { diagnostic ->
              Text(
                  "${diagnostic.code}: ${sanitizedOutputText(diagnostic.message, 512)}",
                  color = Error,
                  fontFamily = FontFamily.Monospace,
                  fontSize = 11.sp,
                  modifier = Modifier.padding(top = 3.dp))
            }
          }
        }
      }
    }
    Spacer(Modifier.height(8.dp))
    MiniOrcaPanel(Modifier.fillMaxWidth()) {
      SectionLabel("FOCUSED CHECKS")
      if (presentation.checks.isEmpty())
          Text(
              presentation.evidence.checks.detail,
              color = SecondaryText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 5.dp))
      presentation.checks.forEach { check ->
        Text(
            "${check.name} · ${check.status.label} · ${if (check.required) "required" else "optional"}",
            color = evidenceColor(check.status),
            fontSize = 11.sp,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(top = 6.dp))
        if (check.command.isNotBlank()) SelectableOutputText("$ ${check.command}")
        if (check.output.isNotBlank()) SelectableOutputText(check.output)
      }
    }
  }
}

internal data class OutputToolWindowState(
    val status: String,
    val error: String?,
    val loading: Boolean,
    val fileAnalysis: FileAnalysis?,
    val analyzeAll: AnalyzeAllJob?,
    val coverage: AnalysisCoverage?,
    val scan: GoScanReport?,
    val analysisInProgress: Boolean,
    val generating: Boolean,
    val validating: Boolean,
)

internal data class OutputEntry(
    val title: String,
    val status: String,
    val detail: String,
    val output: String = "",
) {
  val failed: Boolean
    get() = status.lowercase() in setOf("failed", "error")

  val running: Boolean
    get() = status.lowercase() in setOf("running", "canceling", "validating")
}

internal data class OutputToolWindowPresentation(val entries: List<OutputEntry>) {
  val summary: BottomToolWindowSummary
    get() =
        when {
          entries.any(OutputEntry::failed) -> BottomToolWindowSummary("Output has failures", true)
          entries.any(OutputEntry::running) -> BottomToolWindowSummary("Output running")
          else -> BottomToolWindowSummary("Output ready")
        }
}

internal fun outputToolWindowPresentation(
    state: OutputToolWindowState
): OutputToolWindowPresentation {
  val entries = buildList {
    if (state.validating)
        add(OutputEntry("Draft validation", "Validating", "Draft validation is running."))
    if (state.generating)
        add(OutputEntry("Generation", "Running", "Generating a preview-only draft."))
    if (state.analysisInProgress)
        add(
            OutputEntry(
                "File analysis", "Running", "File analysis is running for the selected file."))
    state.fileAnalysis?.let { analysis ->
      add(
          OutputEntry(
              "File analysis",
              analysis.status.outputStatus(),
              analysis.failure.ifBlank {
                "Latest file analysis is ${analysis.status.outputStatus()}."
              },
              sanitizedOutputText(analysis.failure),
          ))
    }
    state.analyzeAll?.let { job ->
      val presentation = analyzeAllPresentation(job, state.coverage)
      add(OutputEntry("Analyze-all", presentation.run.statusLabel, presentation.run.statusDetail))
      presentation.failures.forEach { failure ->
        add(
            OutputEntry(
                "Analysis failure · ${failure.path}",
                "Failed",
                "Attempt ${failure.attempts}",
                sanitizedOutputText(failure.error),
            ))
      }
    }
    state.scan?.let { scan ->
      add(
          OutputEntry(
              "Verified scan", scan.status.outputStatus(), verifiedScanProgress(scan).summary))
      scan.phases.forEach { phase ->
        add(
            OutputEntry(
                "Scan phase · ${phase.name.ifBlank { "unnamed" }}",
                phase.state.outputStatus(),
                phase.command.joinToString(" ").ifBlank { "No command reported." },
                sanitizedOutputText(phase.output),
            ))
      }
    }
    state.error?.takeIf(String::isNotBlank)?.let { error ->
      add(OutputEntry("Daemon failure", "Failed", sanitizedOutputText(error, 1024)))
    }
    add(
        OutputEntry(
            "Latest operation",
            if (state.loading) "Running" else "Ready",
            sanitizedOutputText(state.status, 1024).ifBlank { "No operation status is available." },
        ))
  }
  return OutputToolWindowPresentation(entries)
}

@Composable
internal fun OutputToolWindow(
    presentation: OutputToolWindowPresentation,
    modifier: Modifier = Modifier,
) {
  Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(12.dp)) {
    SectionLabel("Output")
    Text(
        "Recorded operation details. Opening this tab does not start work.",
        color = SecondaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = 5.dp))
    presentation.entries.forEach { entry ->
      MiniOrcaPanel(Modifier.fillMaxWidth().padding(top = 8.dp)) {
        Text(entry.title, color = PrimaryText, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
        Text(
            entry.status,
            color = if (entry.failed) Error else SecondaryText,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = 3.dp))
        SelectableOutputText(entry.detail)
        if (entry.output.isNotBlank()) SelectableOutputText(entry.output)
      }
    }
  }
}

@Composable
private fun EvidenceTextRow(row: ReviewEvidenceRow) {
  Text(
      "${row.label} · ${row.status.label} · ${row.detail}",
      color = evidenceColor(row.status),
      fontSize = 11.sp,
      modifier = Modifier.padding(top = 6.dp))
}

@Composable
private fun SelectableOutputText(value: String) {
  SelectionContainer {
    Text(
        sanitizedOutputText(value),
        color = SecondaryText,
        fontFamily = FontFamily.Monospace,
        fontSize = 10.sp,
        modifier = Modifier.padding(top = 4.dp))
  }
}

internal fun sanitizedOutputText(value: String, limit: Int = MAX_OUTPUT_CHARS): String {
  val cleaned = value.replace(CONTROL_CHARACTERS, " ").trim()
  return if (cleaned.length <= limit) cleaned else "${cleaned.take(limit)}\n… output truncated"
}

private fun String.outputStatus(): String =
    trim().ifBlank { "Unknown" }.replaceFirstChar { it.uppercase() }

private const val MAX_DIAGNOSTICS = 32
private const val MAX_OUTPUT_CHARS = 4_096
private val CONTROL_CHARACTERS = Regex("[\\u0000-\\u0008\\u000B\\u000C\\u000E-\\u001F\\u007F]")
