package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class SecurityFilters(
    val query: String = "",
    val severity: String = "",
    val triage: String = "",
)

internal data class SecurityWorkspacePaneState(
    val project: ProjectAnalysis?,
    val selectedFile: ProjectFileInfo?,
    val index: ProjectIndex?,
    val sourceReport: SecurityFileReport?,
    val aiReport: SecurityFileReport?,
    val action: String,
    val error: String?,
    val model: ScopedModel,
    val remoteProviderConfirmed: Boolean,
    val sourceOperation: SecuritySectionOperation = SecuritySectionOperation(),
    val aiOperation: SecuritySectionOperation = SecuritySectionOperation(),
)

internal data class SecurityWorkspaceActions(
    val confirmRemoteProvider: (Boolean) -> Unit,
    val scan: () -> Unit,
    val review: () -> Unit,
    val openSource: (SecurityFinding) -> Unit,
    val prepareFix: (SecurityFinding) -> Unit,
)

internal fun securityReportIsCurrent(
    report: SecurityFileReport?,
    project: ProjectAnalysis?,
    file: ProjectFileInfo?,
): Boolean =
    report != null &&
        project != null &&
        file != null &&
        report.projectId == project.projectId &&
        report.projectRevision == project.projectRevision &&
        report.path == file.path &&
        report.contentHash == file.contentHash

internal fun securityReportStateLabel(
    report: SecurityFileReport?,
    project: ProjectAnalysis?,
    file: ProjectFileInfo?,
    operation: SecuritySectionOperation = SecuritySectionOperation(),
): String =
    when {
      report == null -> securitySectionOperationLabel(operation)
      !securityReportIsCurrent(report, project, file) ->
          "Stale — select the matching file and run again"
      report.status == "partial" ->
          "Partial — ${report.reason.ifBlank { "coverage reached the five-finding limit" }}"
      report.status == "completed_empty" ->
          "Completed — no findings returned; this does not prove the file is secure"
      else -> report.status.replace('_', ' ').replaceFirstChar { it.uppercase() }
    }

internal fun securitySectionOperationLabel(operation: SecuritySectionOperation): String =
    when (operation.status) {
      SecuritySectionOperationStatus.Idle -> "Not run"
      SecuritySectionOperationStatus.Running -> "Running"
      SecuritySectionOperationStatus.Canceled -> "Canceled"
      SecuritySectionOperationStatus.Failed -> "Failed — ${operation.message}"
    }

internal fun filterSecurityFindings(
    findings: List<SecurityFinding>,
    filters: SecurityFilters,
): List<SecurityFinding> =
    findings.filter {
      val query = filters.query.trim()
      (query.isBlank() ||
          listOf(it.title, it.rule, it.category, it.observedCondition, it.anchor.symbol).any { text
            ->
            text.contains(query, ignoreCase = true)
          }) &&
          (filters.severity.isBlank() || it.severity.equals(filters.severity.trim(), true)) &&
          (filters.triage.isBlank() || it.triage.equals(filters.triage.trim(), true))
    }

internal fun securityFindingNavigationTarget(
    finding: SecurityFinding,
    index: ProjectIndex?,
): EditorNavigationTarget? {
  val path = finding.anchor.path
  val file = index?.files?.firstOrNull { it.path == path } ?: return null
  if (!securityAnchorIsValid(finding.anchor, file)) return null
  return EditorNavigationTarget(path, finding.anchor.symbol, finding.anchor.startLine)
}

internal fun securityAnchorIsValid(anchor: SecuritySourceAnchor, file: IndexedFile): Boolean =
    anchor.path == file.path &&
        anchor.startLine >= 1 &&
        anchor.endLine >= anchor.startLine &&
        anchor.endLine <= file.lineCount &&
        (anchor.symbol.isBlank() ||
            file.symbols.any { symbol ->
              symbol.name == anchor.symbol &&
                  anchor.startLine >= symbol.startLine &&
                  anchor.endLine <= symbol.endLine
            })

internal fun securityAnchorWithinDeclaration(
    anchor: SecuritySourceAnchor,
    file: IndexedFile,
    declaration: SymbolInfo,
): Boolean =
    securityAnchorIsValid(anchor, file) &&
        declaration.startLine >= 1 &&
        declaration.endLine >= declaration.startLine &&
        anchor.startLine >= declaration.startLine &&
        anchor.endLine <= declaration.endLine

internal fun securityFindingIsCurrent(finding: SecurityFinding, state: DesktopState): Boolean {
  val project = state.project ?: return false
  val file = state.selectedFile ?: return false
  val report =
      listOfNotNull(state.security.sourceReport, state.security.aiReport).firstOrNull { candidate ->
        securityReportIsCurrent(candidate, project, file) &&
            candidate.findings.any { it.id == finding.id }
      } ?: return false
  return report.findings.any { it == finding } &&
      securityFindingNavigationTarget(finding, state.index) != null
}

@Composable
internal fun SecurityWorkspacePane(
    state: SecurityWorkspacePaneState,
    actions: SecurityWorkspaceActions
) {
  var query by remember { mutableStateOf("") }
  var severity by remember { mutableStateOf("") }
  var triage by remember { mutableStateOf("") }
  val filters = SecurityFilters(query, severity, triage)
  LazyColumn(Modifier.fillMaxSize().padding(12.dp)) {
    item {
      IdePaneHeader(
          title = "Security",
          icon = DesktopIcon.Problems,
          stateLabel = state.selectedFile?.path ?: "No file selected",
          actions = {
            MiniOrcaButton(
                onClick = actions.scan,
                enabled = state.selectedFile?.language == "Go" && state.action.isBlank(),
                tone = ActionTone.Primary,
                density = ButtonDensity.Toolbar) {
                  Text(if (state.action == "scan") "Scanning…" else "Scan", fontSize = 11.sp)
                }
            MiniOrcaButton(
                onClick = actions.review,
                enabled =
                    state.selectedFile?.takeIf { !it.binary } != null &&
                        state.action.isBlank() &&
                        (!state.model.remoteProvider || state.remoteProviderConfirmed),
                tone = ActionTone.Navigation,
                density = ButtonDensity.Toolbar) {
                  Text(if (state.action == "review") "Reviewing…" else "Review", fontSize = 11.sp)
                }
          })
      Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
        Text(
            "Scan uses deterministic source-only rules. Review is advisory, uses Analyze scope, and never executes project code.",
            color = SecondaryText,
            fontSize = 10.sp,
            modifier = Modifier.padding(top = 4.dp))
        Text(
            "Each action covers one selected file and reports at most five findings.",
            color = SecondaryText,
            fontSize = 10.sp,
            modifier = Modifier.padding(top = 3.dp))
        if (state.model.remoteProvider)
            Text(
                "Confirmation applies only to the next Security review.",
                color = SecondaryText,
                fontSize = 10.sp,
                modifier = Modifier.padding(top = 6.dp))
        if (state.model.remoteProvider)
            RemoteProviderConfirmation(
                ModelScope.Analyze,
                state.model,
                state.remoteProviderConfirmed,
                actions.confirmRemoteProvider,
                enabled = state.action.isBlank())
        state.error?.let {
          Text(it, color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 6.dp))
        }
      }
      Spacer(Modifier.height(8.dp))
      IdeDisclosureHeader("Filters", expanded = true, onToggle = {}, stateLabel = "Local only")
      Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
        CompactSingleLineField(
            query, { query = it }, "Search security findings", modifier = Modifier.fillMaxWidth())
        CompactSingleLineField(
            severity,
            { severity = it },
            "Severity",
            modifier = Modifier.fillMaxWidth().padding(top = 6.dp))
        CompactSingleLineField(
            triage,
            { triage = it },
            "Triage",
            modifier = Modifier.fillMaxWidth().padding(top = 6.dp))
      }
    }
    securityReportSection(
        "Source rule matches", state.sourceReport, state.sourceOperation, state, filters, actions)
    securityReportSection(
        "AI suggestions", state.aiReport, state.aiOperation, state, filters, actions)
  }
}

private fun androidx.compose.foundation.lazy.LazyListScope.securityReportSection(
    title: String,
    report: SecurityFileReport?,
    operation: SecuritySectionOperation,
    state: SecurityWorkspacePaneState,
    filters: SecurityFilters,
    actions: SecurityWorkspaceActions,
) {
  item {
    val findings =
        if (securityReportIsCurrent(report, state.project, state.selectedFile))
            filterSecurityFindings(report?.findings.orEmpty(), filters)
        else emptyList()
    var selectedFindingId by
        remember(report?.generatedAt, report?.contentHash, report?.source) {
          mutableStateOf<String?>(null)
        }
    val selected = findings.firstOrNull { it.id == selectedFindingId }
    Column(Modifier.fillMaxWidth()) {
      Spacer(Modifier.height(8.dp))
      IdePaneHeader(
          title = title,
          stateLabel =
              securityReportStateLabel(report, state.project, state.selectedFile, operation))
      IdeHorizontalSeparator(Modifier.padding(top = 4.dp))
      if (report != null && operation.status != SecuritySectionOperationStatus.Idle)
          Text(
              "Latest attempt: ${securitySectionOperationLabel(operation)}",
              color =
                  if (operation.status == SecuritySectionOperationStatus.Failed) Error
                  else SecondaryText,
              fontSize = 10.sp,
              modifier = Modifier.padding(top = 4.dp))
      if (findings.isEmpty())
          Text(
              "No current findings match these filters.",
              color = SecondaryText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 6.dp))
      else {
        findings.forEach { finding ->
          ChromeButton(
              onClick = { selectedFindingId = finding.id },
              selected = finding.id == selectedFindingId,
              modifier = Modifier.fillMaxWidth().padding(top = 4.dp)) {
                Text(
                    "${finding.severity.uppercase()} · ${finding.title.ifBlank { "Untitled security finding" }} · ${finding.anchor.path}:${finding.anchor.startLine}",
                    color = PrimaryText,
                    fontSize = 11.sp,
                    modifier = Modifier.weight(1f))
              }
        }
        selected?.let { finding ->
          SecurityFindingDetails(
              finding, securityFindingCanPrepareFix(finding, state.index), actions)
        }
      }
    }
  }
}

private fun securityFindingCanPrepareFix(finding: SecurityFinding, index: ProjectIndex?): Boolean {
  val file = index?.files?.firstOrNull { it.path == finding.anchor.path } ?: return false
  val declaration =
      file.symbols.singleOrNull {
        it.name == finding.anchor.symbol && it.atomicTarget && it.confidence == "exact"
      } ?: return false
  return file.language == "Go" && securityAnchorWithinDeclaration(finding.anchor, file, declaration)
}

@Composable
private fun SecurityFindingDetails(
    finding: SecurityFinding,
    canPrepareFix: Boolean,
    actions: SecurityWorkspaceActions,
) {
  Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 6.dp)) {
    Text(
        "${finding.evidenceKind.replace('_', ' ')} · ${finding.triage.ifBlank { "untriaged" }} · ${finding.verificationState.ifBlank { "unverified" }}",
        color = SecondaryText,
        fontSize = 10.sp)
    Text(
        "${finding.anchor.path}:${finding.anchor.startLine}${finding.anchor.symbol.takeIf { it.isNotBlank() }?.let { " · $it" }.orEmpty()}",
        color = SecondaryText,
        fontSize = 10.sp)
    Text(
        finding.observedCondition,
        color = PrimaryText,
        fontSize = 11.sp,
        modifier = Modifier.padding(top = 4.dp))
    Text(
        "Remediation: ${finding.remediation}",
        color = SecondaryText,
        fontSize = 10.sp,
        modifier = Modifier.padding(top = 4.dp))
    if (finding.evidenceKind == "model_suspicion") {
      Text(
          "Confidence: ${finding.confidence.ifBlank { "not provided" }}",
          color = SecondaryText,
          fontSize = 10.sp,
          modifier = Modifier.padding(top = 4.dp))
      Text(
          "Preconditions / unknowns: ${finding.preconditions.ifBlank { "not provided" }}",
          color = SecondaryText,
          fontSize = 10.sp,
          modifier = Modifier.padding(top = 3.dp))
      Text(
          "Safe verification idea: ${finding.verificationIdea.ifBlank { "not provided" }}",
          color = SecondaryText,
          fontSize = 10.sp,
          modifier = Modifier.padding(top = 3.dp))
    }
    Row(Modifier.padding(top = 6.dp)) {
      MiniOrcaButton(
          onClick = { actions.openSource(finding) },
          tone = ActionTone.Navigation,
          density = ButtonDensity.Toolbar) {
            Text("Open source", fontSize = 11.sp)
          }
      MiniOrcaButton(
          onClick = { actions.prepareFix(finding) },
          enabled = canPrepareFix,
          tone = ActionTone.Navigation,
          density = ButtonDensity.Toolbar,
          modifier = Modifier.padding(start = 6.dp)) {
            Text("Prepare fix", fontSize = 11.sp)
          }
    }
  }
}
