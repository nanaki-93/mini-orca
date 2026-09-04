package io.miniorca.desktop

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun PerformanceWorkspacePane(
    state: PerformanceWorkspacePaneState,
    actions: PerformanceWorkspaceActions
) {
  var category by remember { mutableStateOf("") }
  var impact by remember { mutableStateOf("") }
  var path by remember { mutableStateOf("") }
  var selectedID by remember { mutableStateOf("") }
  val job = state.job
  val findings =
      state.report
          ?.findings
          .orEmpty()
          .filter {
            (category.isBlank() || it.category.equals(category, true)) &&
                (impact.isBlank() || it.potentialImpact.equals(impact, true)) &&
                (path.isBlank() || state.report?.paths?.get(it.id).orEmpty().contains(path, true))
          }
          .sortedWith(
              compareByDescending<PerformanceFinding> { performanceImpactOrder(it.potentialImpact) }
                  .thenByDescending { performanceConfidenceOrder(it.confidence) }
                  .thenBy { state.report?.paths?.get(it.id).orEmpty() }
                  .thenBy { it.startLine }
                  .thenBy { it.id })
  val selected = findings.firstOrNull { it.id == selectedID }
  LazyColumn(Modifier.fillMaxSize().padding(12.dp)) {
    item {
      WorkspacePaneHeader("Performance")
      Text("Source-based review · Not measured", color = Warning, fontSize = 12.sp)
      Text(
          modelDestinationLabel(ModelScope.Analyze, state.model),
          color = SecondaryText,
          fontSize = 11.sp)
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(), contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Review controls")
            Text(
                performanceStatusLabel(job),
                color = PrimaryText,
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 6.dp))
            state.context?.let { preview ->
              CompactKeyValueRows(
                  listOf(
                      "Selected" to "${preview.files.size} files",
                      "Eligible limits" to
                          "${preview.excluded} excluded · ${preview.oversized} oversized · ${preview.outsideLimit} outside limit",
                  ),
                  modifier = Modifier.padding(top = 6.dp))
            }
            when (job?.status) {
              "running" ->
                  ResponsiveActionGroup(Modifier.padding(top = 8.dp)) {
                    MiniOrcaButton(
                        actions.pause,
                        tone = ActionTone.Attention,
                        density = ButtonDensity.Toolbar) {
                          Text("Pause", fontSize = 11.sp)
                        }
                    MiniOrcaButton(
                        actions.cancel,
                        tone = ActionTone.Destructive,
                        density = ButtonDensity.Toolbar) {
                          Text("Cancel", fontSize = 11.sp)
                        }
                  }
              "paused" -> {
                RemoteProviderConfirmation(
                    ModelScope.Analyze,
                    state.model,
                    state.remoteProviderConfirmed,
                    actions.confirmRemoteProvider)
                ResponsiveActionGroup(Modifier.padding(top = 8.dp)) {
                  MiniOrcaButton(
                      { actions.resume(state.remoteProviderConfirmed) },
                      enabled = !state.model.remoteProvider || state.remoteProviderConfirmed,
                      tone = ActionTone.Primary,
                      density = ButtonDensity.Toolbar) {
                        Text("Resume", fontSize = 11.sp)
                      }
                  MiniOrcaButton(
                      actions.cancel,
                      tone = ActionTone.Destructive,
                      density = ButtonDensity.Toolbar) {
                        Text("Cancel", fontSize = 11.sp)
                      }
                }
              }
              else -> {
                RemoteProviderConfirmation(
                    ModelScope.Analyze,
                    state.model,
                    state.remoteProviderConfirmed,
                    actions.confirmRemoteProvider)
                ResponsiveActionGroup(Modifier.padding(top = 8.dp)) {
                  MiniOrcaButton(
                      actions.preview, tone = ActionTone.Neutral, density = ButtonDensity.Toolbar) {
                        Text("Preview limits", fontSize = 11.sp)
                      }
                  MiniOrcaButton(
                      { state.context?.let { actions.start(it, state.remoteProviderConfirmed) } },
                      enabled =
                          state.context != null &&
                              (!state.model.remoteProvider || state.remoteProviderConfirmed),
                      tone = ActionTone.Primary,
                      density = ButtonDensity.Toolbar) {
                        Text("Analyze performance", fontSize = 11.sp)
                      }
                }
              }
            }
          }
      Spacer(Modifier.height(8.dp))
      MiniOrcaPanel(
          Modifier.fillMaxWidth(), contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
            SectionLabel("Filter opportunities")
            ResponsiveFieldPair(
                modifier = Modifier.padding(top = 6.dp),
                first = { modifier ->
                  CompactSingleLineField(category, { category = it }, "Category", modifier)
                },
                second = { modifier ->
                  CompactSingleLineField(impact, { impact = it }, "Potential impact", modifier)
                })
            CompactSingleLineField(
                path, { path = it }, "Path", Modifier.fillMaxWidth().padding(top = 6.dp))
          }
      Spacer(Modifier.height(8.dp))
      selected?.let { finding ->
        PerformanceFindingDetails(finding, state.report?.paths?.get(finding.id).orEmpty(), actions)
      }
      SectionLabel("Opportunities", Modifier.padding(top = 8.dp))
    }
    if (state.report == null)
        item {
          SystemStateMessage(
              "No performance review",
              "Preview a bounded queue, then explicitly start a source-based review. No project performance claim is made.",
              modifier = Modifier.fillMaxWidth().padding(top = 8.dp))
        }
    else if (findings.isEmpty())
        item {
          SystemStateMessage(
              "No opportunities identified in the reviewed files",
              "Coverage and skipped files remain shown above; this is not a measured performance verdict.",
              modifier = Modifier.fillMaxWidth().padding(top = 8.dp))
        }
    else
        items(findings, key = { it.id }) { finding ->
          MiniOrcaButton(
              onClick = { selectedID = finding.id },
              tone = ActionTone.Neutral,
              density = ButtonDensity.Toolbar,
              modifier = Modifier.fillMaxWidth().padding(top = 6.dp)) {
                Text(
                    "${finding.category.uppercase()} · ${finding.potentialImpact} · ${state.report?.paths?.get(finding.id).orEmpty()}:${finding.startLine} · ${finding.title}",
                    fontSize = 11.sp)
              }
        }
  }
}

@Composable
private fun PerformanceFindingDetails(
    finding: PerformanceFinding,
    path: String,
    actions: PerformanceWorkspaceActions
) {
  MiniOrcaPanel(
      Modifier.fillMaxWidth(),
      raised = true,
      contentPadding = PaddingValues(MiniOrcaSpacing.standard)) {
        SectionLabel("Selected opportunity")
        CompactKeyValueRows(
            listOf("Location" to "$path:${finding.startLine}", "Confidence" to finding.confidence),
            Modifier.padding(top = 6.dp))
        Text(
            "Observed pattern: ${finding.observedPattern}",
            color = PrimaryText,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = 6.dp))
        Text(
            "When it matters: ${finding.workloadConditions}\nRecommendation: ${finding.recommendation}\nTrade-off: ${finding.tradeoff}\nVerify: ${finding.verificationPlan}",
            color = SecondaryText,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = 6.dp))
        EngineeringInsightPanel(
            finding.engineeringInsight, scopeLabel = "Selected performance opportunity")
        ResponsiveActionGroup(Modifier.padding(top = 8.dp)) {
          MiniOrcaButton(
              { actions.openInEditor(path, finding) },
              tone = ActionTone.Neutral,
              density = ButtonDensity.Toolbar) {
                Text("Open in Editor", fontSize = 11.sp)
              }
          MiniOrcaButton(
              { actions.prepareOptimization(path, finding) },
              enabled = finding.symbol.isNotBlank(),
              tone = ActionTone.Primary,
              density = ButtonDensity.Toolbar) {
                Text("Prepare optimization", fontSize = 11.sp)
              }
        }
      }
}

internal data class PerformanceWorkspacePaneState(
    val job: PerformanceJob?,
    val report: PerformanceReport?,
    val context: PerformanceQueuePreview?,
    val model: ScopedModel,
    val remoteProviderConfirmed: Boolean,
)

internal data class PerformanceWorkspaceActions(
    val confirmRemoteProvider: (Boolean) -> Unit,
    val preview: () -> Unit,
    val start: (PerformanceQueuePreview, Boolean) -> Unit,
    val pause: () -> Unit,
    val resume: (Boolean) -> Unit,
    val cancel: () -> Unit,
    val openInEditor: (String, PerformanceFinding) -> Unit,
    val prepareOptimization: (String, PerformanceFinding) -> Unit,
)

private fun performanceStatusLabel(job: PerformanceJob?): String =
    when (job?.status) {
      "running" -> "Current run: running · ${job.elapsed / 1_000_000_000}s execution budget used"
      "paused" -> "Current run: paused · resume is explicit"
      "canceled" -> "Last run: canceled; completed reviews remain available"
      "stale" -> "Last run: outdated — source or policy changed"
      "completed" -> "Last run: completed source-based queue"
      else -> "No review has started"
    }

private fun performanceImpactOrder(value: String): Int =
    when (value) {
      "high" -> 4
      "medium" -> 3
      "low" -> 2
      else -> 1
    }

private fun performanceConfidenceOrder(value: String): Int =
    when (value) {
      "high" -> 3
      "medium" -> 2
      else -> 1
    }
