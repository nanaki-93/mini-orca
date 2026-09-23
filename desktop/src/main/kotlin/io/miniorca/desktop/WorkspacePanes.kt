package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
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

private val workspaceWideGutterMinimum = 900.dp
private val analysisFileTableMinimumHeight = 160.dp
private val analysisFileTableMaximumHeight = 400.dp

internal fun workspacePageHorizontalGutter(availableWidth: Dp): Dp =
    if (availableWidth >= workspaceWideGutterMinimum) 24.dp else 16.dp

internal fun workspacePagePadding(availableWidth: Dp, vertical: Dp): PaddingValues =
    PaddingValues(horizontal = workspacePageHorizontalGutter(availableWidth), vertical = vertical)

/** Keeps the nested file list bounded within the viewport remaining after measured content. */
internal fun analysisFileTableHeight(availableHeight: Dp, occupiedHeight: Dp?): Dp =
    if (occupiedHeight == null) analysisFileTableMinimumHeight
    else
        (availableHeight - occupiedHeight).coerceIn(
            analysisFileTableMinimumHeight, analysisFileTableMaximumHeight)

@Composable
internal fun AnalysisWorkspacePane(
    state: AnalysisWorkspacePaneState,
    actions: AnalysisWorkspaceActions
) {
  val analysis = state.analysis
  val presentation = projectRunPresentation(analysis)
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
        contentPadding = workspacePagePadding(maxWidth, vertical = 16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
          item { Box(Modifier.onSizeChanged { headerHeight = it.height }) { AnalysisPageHeader() } }
          item {
            Box(Modifier.onSizeChanged { runHeight = it.height }) {
              AnalysisRunPanel(state, actions)
            }
          }
          item {
            Box(Modifier.onSizeChanged { categoryHeight = it.height }) {
              AnalysisCategoryPanels(state, actions.openResults)
            }
          }
          item {
            AnalysisFileSelector(analysis, actions, fileTableHeight) { height ->
              fileChromeHeight = height
            }
          }
          items(presentation.failures) { failure -> AnalysisFailureDetails(failure) }
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

@Composable
internal fun AnalysisFailureDetails(failure: AnalysisStageFailure) {
  MiniOrcaPanel(
      modifier = Modifier.fillMaxWidth().testTag("analysis-stage-failure-${failure.path}"),
      contentPadding = PaddingValues(16.dp)) {
        Text(
            "${analysisStageLabel(failure.stage)} · ${failure.path}",
            style = IdeTypography.resultLabel,
            color = PrimaryText)
        Text(
            "Attempts: ${failure.attempts}",
            style = IdeTypography.compactBody,
            color = SecondaryText)
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
      browser = state.browser,
      openAnalysis = actions.openAnalysis,
      tools = {
        val scan = verifiedScanProgress(state.scan)
        VerifiedChecksActionRow(state.scan, scan, actions)
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
                    .padding(8.dp)) {
                  Text(scan.summary, style = IdeTypography.compactBody, color = SecondaryText)
                  state.scan?.let { VerifiedScanDiagnostics(it) }
                }
      }) { key ->
        visible
            .firstOrNull { semanticResultRow(it).key == key }
            ?.let { FindingDetailsRegion(it, actions.findingActions) }
      }
}

@Composable
@OptIn(ExperimentalLayoutApi::class)
private fun VerifiedChecksActionRow(
    report: GoScanReport?,
    progress: VerifiedScanProgress,
    actions: BugsWorkspaceActions,
) {
  FlowRow(
      Modifier.fillMaxWidth().testTag("verified-checks-row"),
      horizontalArrangement = Arrangement.spacedBy(8.dp),
      verticalArrangement = Arrangement.spacedBy(4.dp),
      itemVerticalAlignment = Alignment.CenterVertically) {
        Text("Verified checks", color = PrimaryText, style = IdeTypography.resultHeading)
        IdeLabelBadge(progress.statusLabel, evidenceColor(checkStatus(report?.status.orEmpty())))
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
                Text("${progress.statusLabel} checks")
              }
        }
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
  Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
    ResultDetailHeader(semanticResultRow(finding))
    IdeLabelBadge(findingEvidenceIdentity(finding), findingEvidenceTint(finding))
    ModelResultContent(
        finding.message.ifBlank { "No summary supplied." }, style = IdeTypography.workspaceBody)
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
    val browser: ResultBrowserState = newResultBrowserState(page),
)

/** Finding navigation, task preparation, triage, and scan intents. */
internal data class BugsWorkspaceActions(
    val findingActions: FindingActions,
    val startScan: () -> Unit,
    val cancelScan: () -> Unit,
    val openAnalysis: () -> Unit = {},
)
