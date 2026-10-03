package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private val analysisFileTableMinimumHeight = 160.dp
private val analysisFileTableMaximumHeight = 400.dp

private val workspacePageHorizontalGutter = 24.dp

internal fun workspacePagePadding(vertical: Dp): PaddingValues =
    PaddingValues(horizontal = workspacePageHorizontalGutter, vertical = vertical)

/** Keeps the nested file list bounded within the viewport remaining after measured content. */
internal fun analysisFileTableHeight(availableHeight: Dp, occupiedHeight: Dp?): Dp =
    if (occupiedHeight == null) analysisFileTableMinimumHeight
    else
        (availableHeight - occupiedHeight).coerceIn(
            analysisFileTableMinimumHeight, analysisFileTableMaximumHeight)

@Composable
internal fun AnalysisWorkspacePane(
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions,
    filesView: AnalysisFilesViewState =
        remember(state.project?.projectId, state.project?.projectRevision) {
          AnalysisFilesViewState()
        },
) {
  val analysis = state.analysis
  var resultsHelpExpanded by remember(state.project?.projectId) { mutableStateOf(false) }
  BoxWithConstraints(Modifier.fillMaxSize()) {
    val density = LocalDensity.current
    var headerHeight by remember { mutableStateOf<Int?>(null) }
    var runHeight by remember { mutableStateOf<Int?>(null) }
    var categoryHeight by remember { mutableStateOf<Int?>(null) }
    var fileChromeHeight by remember { mutableStateOf<Int?>(null) }
    val occupiedHeight =
        listOfNotNull(headerHeight, runHeight, categoryHeight, fileChromeHeight)
            .takeIf { it.size == 4 }
            ?.sum()
            ?.let { measured -> with(density) { measured.toDp() + 32.dp + 48.dp } }
    val fileTableHeight = analysisFileTableHeight(maxHeight, occupiedHeight)
    LazyColumn(
        Modifier.fillMaxSize().testTag("analysis-page"),
        contentPadding = workspacePagePadding(vertical = 16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
          item { Box(Modifier.onSizeChanged { headerHeight = it.height }) { AnalysisPageHeader() } }
          item {
            Box(Modifier.onSizeChanged { runHeight = it.height }) {
              AnalysisRunPanel(state, actions)
            }
          }
          item {
            Column(
                Modifier.fillMaxWidth().onSizeChanged { categoryHeight = it.height },
                verticalArrangement = Arrangement.spacedBy(8.dp)) {
                  IdePaneHeader(
                      title = "Saved results",
                      actions = {
                        ChromeButton(
                            onClick = { resultsHelpExpanded = !resultsHelpExpanded },
                            accessibleName = "About result counts",
                            modifier =
                                Modifier.semantics {
                                  stateDescription =
                                      if (resultsHelpExpanded) "Expanded" else "Collapsed"
                                }) {
                              Text(if (resultsHelpExpanded) "Hide help" else "About counts")
                            }
                      })
                  if (resultsHelpExpanded)
                      Text(
                          "Saved findings stay visible during retries. Successful reviews replace each file’s previous findings; run coverage is shown separately.",
                          color = SecondaryText,
                          style = IdeTypography.workspaceMetadata)
                  AnalysisCategoryPanels(state, actions.openResults)
                }
          }
          item {
            AnalysisFileSelector(
                analysis,
                actions,
                fileTableHeight,
                { height -> fileChromeHeight = height },
                filesView)
          }
        }
  }
}

@Composable
private fun AnalysisPageHeader() {
  Column(
      Modifier.fillMaxWidth().padding(horizontal = 2.dp),
      verticalArrangement = Arrangement.spacedBy(2.dp),
  ) {
    Text(
        "Analysis",
        color = PrimaryText,
        style = IdeTypography.workspaceHeading,
        modifier = Modifier.semantics { heading() },
    )
  }
}

@Composable
private fun AnalysisRunPanel(
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions,
) {
  AnalysisRunStrip(
      state, actions, AnalysisRunStripScope.Analysis, Modifier.testTag("analysis-run-panel"))
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
    val refreshStatus: () -> Unit,
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
      IdeCheckbox(
          checked = confirmed,
          onCheckedChange = onConfirmed,
          accessibleName = "Confirm remote destination",
          modifier = Modifier.padding(top = MiniOrcaSpacing.compact),
          enabled = enabled,
          stateLabel = if (confirmed) "Confirmed" else "Not confirmed",
          label = "Confirm remote destination")
}

@Composable
internal fun BugsWorkspacePane(state: BugsWorkspacePaneState, actions: BugsWorkspaceActions) {
  val visible = groupFindingsByPriority(state.findings).flatMap { it.findings }
  var scanExpanded by
      remember(state.project?.projectId, state.project?.projectRevision, state.scan) {
        mutableStateOf(false)
      }
  AnalysisResultsPane(
      page = state.page,
      rows = visible.map(::semanticResultRow),
      browser = state.browser,
      openAnalysis = actions.openAnalysis,
      retryResults = actions.retryResults,
      tools = {
        val scan = verifiedScanProgress(state.project, state.scanState, state.scan)
        VerifiedChecksActionRow(state, scan, actions)
        IdeDisclosureHeader(
            "Command and output",
            scanExpanded,
            { scanExpanded = !scanExpanded },
            stateLabel = "Details")
        if (scanExpanded)
            Column(
                Modifier.fillMaxWidth()
                    .heightIn(max = 180.dp)
                    .verticalScroll(rememberScrollState())
                    .testTag("scan-diagnostics")
                    .padding(8.dp)) {
                  state.scan?.let { VerifiedScanDiagnostics(it) }
                      ?: Text("No scan report available.", style = IdeTypography.compactBody)
                }
      }) { key ->
        visible
            .firstOrNull { semanticResultRow(it).key == key }
            ?.let {
              FindingDetailsRegion(
                  it,
                  actions.findingActions,
                  findingPreparationDecision(it, state.page.project, state.findings, state.index),
                  sourceAvailable = findingNavigationTarget(it, state.index) != null)
            }
      }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun VerifiedChecksActionRow(
    state: BugsWorkspacePaneState,
    progress: VerifiedScanProgress,
    actions: BugsWorkspaceActions,
) {
  Column(
      Modifier.fillMaxWidth().testTag("verified-checks-row"),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        FlowRow(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
            itemVerticalAlignment = Alignment.CenterVertically) {
              Text("Verified Go scan", color = PrimaryText, style = IdeTypography.resultHeading)
              IdeLabelBadge(progress.statusLabel, evidenceColor(checkStatus(progress.statusLabel)))
            }
        SelectionContainer {
          Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(
                "Project: ${state.project?.projectId?.takeIf { it.isNotBlank() } ?: "unavailable"} · Revision: ${state.project?.projectRevision?.takeIf { it.isNotBlank() } ?: "unavailable"}",
                style = IdeTypography.compactBody,
                color = SecondaryText)
            Text(
                "Whole project · Go parser · go vet ./... · go test ./...",
                style = IdeTypography.compactBody,
                color = SecondaryText)
            Text(
                "Executes project code in a temporary copy · not sandboxed",
                style = IdeTypography.compactBody,
                color = Warning)
            Text(progress.summary, style = IdeTypography.compactBody, color = SecondaryText)
            // Admission, cancellation and availability can take precedence over read progress,
            // but must not hide a separate status failure beside retained evidence.
            val readFailure =
                when (val read = state.scanState.read) {
                  is VerifiedScanRead.Unavailable -> "Status unavailable" to read.message
                  is VerifiedScanRead.PollUnavailable -> "Live status unavailable" to read.message
                  else -> null
                }
            if (readFailure != null && readFailure.second != progress.summary)
                Text(
                    "${readFailure.first}: ${readFailure.second}",
                    style = IdeTypography.compactBody,
                    color = Warning)
            when (val refresh = state.scanState.findingsRefresh) {
              is VerifiedScanFindingsRefresh.Unavailable ->
                  Text(
                      "Tool findings unavailable: ${refresh.message}. Previously loaded findings may be stale.",
                      style = IdeTypography.compactBody,
                      color = Warning)
              VerifiedScanFindingsRefresh.Stale ->
                  Text("Tool findings · stale", style = IdeTypography.compactBody, color = Warning)
              VerifiedScanFindingsRefresh.Refreshing ->
                  Text(
                      "Refreshing findings…",
                      style = IdeTypography.compactBody,
                      color = SecondaryText)
              else -> Unit
            }
          }
        }
        FlowRow(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)) {
              when (progress.action) {
                VerifiedScanAction.Start ->
                    MiniOrcaButton(onClick = actions.startScan, tone = ActionTone.Neutral) {
                      Text("Trust project-code execution & run checks")
                    }
                VerifiedScanAction.Cancel ->
                    MiniOrcaButton(onClick = actions.cancelScan, tone = ActionTone.Destructive) {
                      Text("Cancel checks")
                    }
                VerifiedScanAction.Waiting ->
                    MiniOrcaButton(onClick = {}, enabled = false, tone = ActionTone.Neutral) {
                      Text("Trust project-code execution & run checks")
                    }
              }
              if (state.project?.projectId?.isNotBlank() == true &&
                  state.project.projectRevision.isNotBlank() &&
                  state.scanState.read != VerifiedScanRead.Reading &&
                  state.scanState.operation != VerifiedScanOperation.Starting &&
                  state.scanState.operation != VerifiedScanOperation.CancellationRequested)
                  MiniOrcaButton(
                      onClick = actions.refreshScanStatus, tone = ActionTone.Navigation) {
                        Text("Refresh scan status")
                      }
            }
      }
}

@Composable
internal fun VerifiedScanDiagnostics(scan: GoScanReport) {
  if (scan.phases.isEmpty()) Text("Check outcome unavailable", style = IdeTypography.compactBody)
  scan.phases.forEach { phase ->
    IdeHorizontalSeparator(Modifier.padding(vertical = 8.dp))
    SelectionContainer {
      Text(
          phase.name.ifBlank { "Unnamed scan phase" },
          color = PrimaryText,
          style = IdeTypography.resultHeading)
    }
    IdeLabelBadge(
        phase.state.takeIf { it.isNotBlank() }?.let(::analysisStatusLabel) ?: "State unavailable",
        evidenceColor(checkStatus(phase.state)))
    if (phase.command.isEmpty())
        Text("Command unavailable", style = IdeTypography.compactBody, color = SecondaryText)
    else {
      DiagnosticText("\$ ${phase.command.joinToString(" ")}", color = SecondaryText)
      // Zero is also the wire default when a command never produced an exit result.
      if (phase.exitCode != 0 &&
          phase.state.lowercase() in setOf("failed", "canceled", "cancelled"))
          Text(
              "Exit code: ${phase.exitCode}",
              style = IdeTypography.compactBody,
              color = SecondaryText)
    }
    if (phase.output.isBlank())
        Text("No recorded output", style = IdeTypography.compactBody, color = SecondaryText)
    else {
      if (phase.output.trimEnd().endsWith("\n[output truncated]"))
          Text("Recorded output truncated", style = IdeTypography.compactBody, color = Warning)
      DiagnosticText(phase.output)
    }
  }
}

@Composable
internal fun FindingDetailsRegion(
    finding: UnifiedFinding,
    actions: FindingActions,
    preparation: FindingPreparationDecision =
        FindingPreparationDecision.Blocked("Current project evidence is unavailable."),
    sourceAvailable: Boolean? = null,
) {
  var technical by remember(findingDisplayKey(finding)) { mutableStateOf(false) }
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
    ResultDetailHeader(semanticResultRow(finding))
    IdeLabelBadge(findingEvidenceIdentity(finding), findingEvidenceTint(finding))
    ModelResultContent(
        finding.message.ifBlank { "No summary supplied." },
        preview = false,
        style = IdeTypography.workspaceBody)
    FindingActionButtons(finding, actions, preparation, sourceAvailable)
    if (sourceAvailable == false)
        Text(
            "Open source requires a path in the current project index.",
            style = IdeTypography.compactBody,
            color = SecondaryText)
    if (preparation is FindingPreparationDecision.Blocked)
        Text(preparation.reason, style = IdeTypography.compactBody, color = SecondaryText)
    IdeDisclosureHeader("Evidence and fix criteria", technical, { technical = !technical })
    if (technical) {
      Text(
          findingEvidenceSummary(finding), style = IdeTypography.compactBody, color = SecondaryText)
      if (finding.evidence.isNotBlank()) ModelResultContent(finding.evidence, preview = false)
      finding.taskSpec?.let { task ->
        SelectionContainer {
          Text(
              "${task.targetSymbol} · ${task.targetSignature}",
              style = IdeTypography.resultCode,
              color = PrimaryText)
        }
        Text("Acceptance criteria", style = IdeTypography.resultLabel, color = PrimaryText)
        task.acceptanceCriteria.forEach { ModelResultContent(it, preview = false) }
        if (task.nonGoals.isNotEmpty()) {
          Text("Non-goals", style = IdeTypography.resultLabel, color = PrimaryText)
          task.nonGoals.forEach { ModelResultContent(it, preview = false) }
        }
        task.goTestCandidate?.let { candidate ->
          Text(
              "Test candidate · review only",
              style = IdeTypography.resultLabel,
              color = PrimaryText)
          SelectionContainer {
            Text(candidate.name, style = IdeTypography.resultCode, color = PrimaryText)
          }
          ModelResultContent(candidate.content, preview = false)
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
    val browser: ResultBrowserState = newResultBrowserState(page),
    val index: ProjectIndex? = null,
    val project: ProjectAnalysis? = page.project,
    val scanState: VerifiedScanState = VerifiedScanState(),
)

/** Finding navigation, task preparation, triage, and scan intents. */
internal data class BugsWorkspaceActions(
    val findingActions: FindingActions,
    val startScan: () -> Unit,
    val cancelScan: () -> Unit,
    val openAnalysis: () -> Unit = {},
    val retryResults: (() -> Unit)? = null,
    val refreshScanStatus: () -> Unit = {},
)
