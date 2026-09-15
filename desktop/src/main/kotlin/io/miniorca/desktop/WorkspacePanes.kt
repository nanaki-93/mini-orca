package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private val workspaceWideGutterMinimum = 900.dp

internal fun workspacePageHorizontalGutter(availableWidth: Dp): Dp =
    if (availableWidth >= workspaceWideGutterMinimum) 24.dp else 16.dp

internal fun workspacePagePadding(availableWidth: Dp, vertical: Dp): PaddingValues =
    PaddingValues(horizontal = workspacePageHorizontalGutter(availableWidth), vertical = vertical)

@Composable
internal fun AnalysisWorkspacePane(
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions
) {
  val analysis = state.analysis
  val presentation = projectRunPresentation(analysis)
  val busy = analysis.action.isNotEmpty() || analysis.fileSelection.saving
  LazyColumn(
      Modifier.fillMaxSize(),
      contentPadding = PaddingValues(8.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        item {
          IdePaneHeader(
              title = presentation.headline,
              icon = DesktopIcon.Analysis,
              stateLabel = if (analysis.run?.isActive() == true) presentation.status else null,
              stateTint = analysisStatusTint(analysis.run?.status),
              actions = {
                presentation.commands.forEach { command ->
                  MiniOrcaButton(
                      onClick = {
                        when (command) {
                          AnalysisRunCommand.Start,
                          AnalysisRunCommand.RetryStaleFailed ->
                              actions.start(
                                  defaultAnalysisRunLimits,
                                  command == AnalysisRunCommand.RetryStaleFailed)
                          AnalysisRunCommand.Pause -> actions.pause()
                          AnalysisRunCommand.Resume -> actions.resume()
                          AnalysisRunCommand.Cancel -> actions.cancel()
                        }
                      },
                      enabled = state.project != null && !busy,
                      tone =
                          if (command == AnalysisRunCommand.Cancel) ActionTone.Destructive
                          else if (command == AnalysisRunCommand.RetryStaleFailed)
                              ActionTone.Neutral
                          else ActionTone.Primary,
                      density = ButtonDensity.Toolbar) {
                        Text(command.label)
                      }
                }
              })
          analysis.run
              ?.reason
              ?.takeIf { it.isNotBlank() }
              ?.let { DiagnosticText(it, color = Warning) }
          if (analysis.action.isNotEmpty())
              Text(
                  "${analysis.action.replaceFirstChar { it.uppercase() }}…",
                  style = IdeTypography.compactBody,
                  color = SelectionText)
          analysis.error?.let { DiagnosticText(it, color = Error) }
        }
        item {
          analysis.run?.let { run ->
            if (run.isActive())
                IdeProgressBar(
                    presentation.progress,
                    Modifier.fillMaxWidth().padding(vertical = 8.dp).height(4.dp),
                    color = SelectionText,
                    trackColor = StrongSurface)
            presentation.currentFiles.forEach {
              Text("Current: $it", style = IdeTypography.resultCode, color = SelectionText)
            }
            if (run.plan.compatibilityStage.isNotBlank())
                Text(
                    "Saved limited run: ${analysisStageLabel(run.plan.compatibilityStage)}",
                    color = Warning,
                    style = IdeTypography.compactBody)
            if (run.plan.retryStaleFailed)
                Text(
                    "Scope: stale & failed files",
                    color = SecondaryText,
                    style = IdeTypography.compactBody)
          }
        }
        item { AnalysisCategoryPanels(state, actions.openResults) }
        item { AnalysisFileSelector(analysis, actions) }
        items(presentation.failures) { failure -> AnalysisFailureDetails(failure) }
      }
}

@Composable
internal fun AnalysisFailureDetails(failure: AnalysisStageFailure) {
  Column(Modifier.fillMaxWidth().padding(8.dp)) {
    Text(
        "${analysisStageLabel(failure.stage)} · ${failure.path}",
        style = IdeTypography.resultLabel,
        color = PrimaryText)
    Text("Attempts: ${failure.attempts}", style = IdeTypography.compactBody, color = SecondaryText)
    DiagnosticText(failure.reason, color = Error)
  }
}

internal data class AnalysisWorkspacePaneState(
    val project: ProjectAnalysis?,
    val analysis: ProjectAnalysisRunState
)

internal data class AnalysisWorkspaceActions(
    val start: (AnalysisRunLimits, Boolean) -> Unit,
    val pause: () -> Unit,
    val resume: () -> Unit,
    val cancel: () -> Unit,
    val openResults: (Workspace) -> Unit,
    val refreshSelection: () -> Unit = {},
    val saveSelection: (List<String>) -> Unit = {},
)

@Composable
internal fun RemoteProviderConfirmation(
    scope: ModelScope,
    model: ScopedModel,
    confirmed: Boolean,
    onConfirmed: (Boolean) -> Unit,
    enabled: Boolean = true,
) {
  Text(
      modelDestinationLabel(scope, model),
      color = if (model.remoteProvider) Warning else SecondaryText,
      fontSize = 11.sp,
      modifier = Modifier.padding(top = MiniOrcaSpacing.standard),
  )
  if (model.remoteProvider)
      ChromeButton(
          onClick = { onConfirmed(!confirmed) },
          enabled = enabled,
          selected = confirmed,
          role = androidx.compose.ui.semantics.Role.Checkbox,
          accessibleName = "Confirm remote destination",
          modifier =
              Modifier.padding(top = MiniOrcaSpacing.compact).semantics {
                stateDescription = if (confirmed) "Confirmed" else "Not confirmed"
              },
      ) {
        Text(if (confirmed) "✓" else "□", fontSize = 14.sp)
        Spacer(Modifier.width(MiniOrcaSpacing.compact))
        Text("Confirm remote destination${if (confirmed) " · confirmed" else ""}", fontSize = 11.sp)
      }
}

@Composable
internal fun BugsWorkspacePane(state: BugsWorkspacePaneState, actions: BugsWorkspaceActions) {
  val visible = groupFindingsByPriority(state.findings).flatMap { it.findings }
  var scanExpanded by remember { mutableStateOf(false) }
  AnalysisResultsPane(
      page = state.page,
      rows = visible.map(::semanticResultRow),
      openAnalysis = actions.openAnalysis,
      openResults = actions.openResults,
      emptyMessage = if (state.loading) "Loading findings…" else "No findings yet.",
      tools = {
        val scan = verifiedScanProgress(state.scan)
        IdeDisclosureHeader(
            "Verified checks",
            scanExpanded,
            { scanExpanded = !scanExpanded },
            stateLabel = state.scan?.status ?: "Not run")
        if (scanExpanded)
            Column(
                Modifier.fillMaxWidth()
                    .heightIn(max = 180.dp)
                    .verticalScroll(rememberScrollState())
                    .padding(8.dp)) {
                  Text(scan.summary, style = IdeTypography.compactBody, color = SecondaryText)
                  Text(
                      "Explicit local execution: go test ./... runs in a copied workspace; go vet ./... reads source.",
                      style = IdeTypography.compactBody,
                      color = SecondaryText)
                  if (scan.canCancel)
                      MiniOrcaButton(
                          onClick = actions.cancelScan,
                          enabled = state.scan?.status == "running",
                          tone = ActionTone.Destructive) {
                            Text("Cancel scan")
                          }
                  else
                      MiniOrcaButton(onClick = actions.startScan, tone = ActionTone.Neutral) {
                        Text("Trust local execution & run scan")
                      }
                  state.scan?.let { VerifiedScanDiagnostics(it) }
                }
      }) { key ->
        visible
            .firstOrNull { semanticResultRow(it).key == key }
            ?.let { FindingDetailsRegion(it, actions.findingActions) }
      }
}

@Composable
internal fun VerifiedScanDiagnostics(scan: GoScanReport) {
  scan.phases.forEach { phase ->
    IdeHorizontalSeparator(Modifier.padding(vertical = 8.dp))
    Text(
        sanitizedOutputText(phase.name.ifBlank { "Unnamed scan phase" }, 256),
        color = PrimaryText,
        style = IdeTypography.resultHeading)
    IdeLabelBadge(analysisStatusLabel(phase.state), evidenceColor(checkStatus(phase.state)))
    if (phase.command.isNotEmpty())
        DiagnosticText("\$ ${phase.command.joinToString(" ")}", color = SecondaryText)
    DiagnosticText(phase.output.ifBlank { "No output reported for this phase." })
  }
}

@Composable
internal fun FindingDetailsRegion(finding: UnifiedFinding, actions: FindingActions) {
  var technical by remember(findingDisplayKey(finding)) { mutableStateOf(false) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
    Text(
        finding.title.ifBlank { "Untitled finding" },
        style = IdeTypography.resultHeading,
        color = PrimaryText)
    IdeLabelBadge(
        finding.severity.ifBlank { "Unknown severity" }, resultSeverityTint(finding.severity))
    Text(findingLocationLabel(finding), style = IdeTypography.resultCode, color = SelectionText)
    findingMaterialStateLabel(finding).takeIf(String::isNotBlank)?.let {
      Text(it, style = IdeTypography.compactBody, color = Warning)
    }
    ModelResultContent(finding.message.ifBlank { "No summary supplied." })
    FindingActionButtons(finding, actions)
    if (!findingCanPrepareFix(finding))
        Text(
            if (finding.freshness == "stale") "Analyze again to prepare a fix from current source."
            else "Fix preparation requires a current declaration task with acceptance criteria.",
            style = IdeTypography.compactBody,
            color = SecondaryText)
    IdeDisclosureHeader("Evidence and fix criteria", technical, { technical = !technical })
    if (technical) {
      Text(
          findingEvidenceSummary(finding), style = IdeTypography.compactBody, color = SecondaryText)
      if (finding.evidence.isNotBlank()) ModelResultContent(finding.evidence)
      finding.taskSpec?.let { task ->
        Text(
            "${task.targetSymbol} · ${task.targetSignature}",
            style = IdeTypography.resultCode,
            color = PrimaryText)
        Text("Acceptance criteria", style = IdeTypography.resultLabel, color = PrimaryText)
        task.acceptanceCriteria.forEach { ModelResultContent(it) }
        if (task.nonGoals.isNotEmpty()) {
          Text("Non-goals", style = IdeTypography.resultLabel, color = PrimaryText)
          task.nonGoals.forEach { ModelResultContent(it) }
        }
      }
      EngineeringInsightPanel(
          finding.engineeringInsight,
          stale = finding.freshness == "stale",
          scopeLabel = "Selected finding")
    }
  }
}

/** Read-only Bugs workspace inputs from the current project snapshot. */
internal data class BugsWorkspacePaneState(
    val findings: List<UnifiedFinding>,
    val scan: GoScanReport?,
    val loading: Boolean,
    val page: AnalysisResultPageState =
        AnalysisResultPageState(AnalysisResultType.Bugs, null, null),
)

/** Finding navigation, task preparation, triage, and scan intents. */
internal data class BugsWorkspaceActions(
    val findingActions: FindingActions,
    val startScan: () -> Unit,
    val cancelScan: () -> Unit,
    val openAnalysis: () -> Unit = {},
    val openResults: (Workspace) -> Unit = {},
)
