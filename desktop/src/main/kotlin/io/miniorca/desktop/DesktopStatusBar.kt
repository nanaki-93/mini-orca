package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class DesktopStatusProvider(
    val scope: ModelScope,
    val model: ScopedModel,
)

internal data class DesktopStatusProviderPresentation(
    val detail: String,
    val remoteProvider: Boolean,
)

internal data class DesktopStatusBarPresentation(
    val provider: DesktopStatusProviderPresentation?,
    val modelsLabel: String,
    val modelsDetail: String,
)

internal fun desktopStatusBarVisible(project: ProjectAnalysis?): Boolean = project != null

internal fun desktopStatusBarPresentation(
    state: DesktopState,
    providers: DesktopShellStatusProviders,
): DesktopStatusBarPresentation {
  val provider =
      if (state.workspace in
          setOf(Workspace.Analysis, Workspace.Bugs, Workspace.Performance, Workspace.Security))
          capturedRunProviderPresentation(state)
      else providerPresentation(statusProviderForWorkspace(state.workspace, providers))
  val configured =
      listOf(
          DesktopStatusProvider(ModelScope.Analyze, providers.analyze),
          DesktopStatusProvider(ModelScope.Bug, providers.bugs),
          DesktopStatusProvider(ModelScope.Function, providers.functionEdits))
  val complete = configured.all { it.model.model.isNotBlank() }
  val models =
      configured
          .map { it.model }
          .distinctBy { Triple(it.providerOrigin, it.model, it.remoteProvider) }
  val cloud = models.count { it.remoteProvider }
  return DesktopStatusBarPresentation(
      provider,
      if (complete) "Models: ${models.size - cloud} local · $cloud cloud"
      else "Models: unavailable",
      if (complete)
          "Distinct configured models by destination; shared models are counted once.\n" +
              configured.joinToString("\n") { modelDestinationLabel(it.scope, it.model) }
      else "Model counts are unavailable until all configured model scopes have been loaded.",
  )
}

@Composable
internal fun PersistentStatusBar(
    presentation: DesktopStatusBarPresentation,
    onOpenDetails: () -> Unit,
    modifier: Modifier = Modifier,
) {
  Column(modifier.fillMaxWidth().background(ActivityRail)) {
    IdeHorizontalSeparator()
    Row(
        Modifier.fillMaxWidth().heightIn(min = 29.dp).padding(horizontal = 10.dp),
        horizontalArrangement = Arrangement.End,
        verticalAlignment = Alignment.CenterVertically,
    ) {
      ChromeButton(
          onClick = onOpenDetails,
          contentPadding = PaddingValues(horizontal = 4.dp, vertical = 2.dp),
          accessibleName = "Configured model details",
          tooltip = null,
      ) {
        Text(presentation.modelsLabel, color = SecondaryText, fontSize = 11.sp)
      }
    }
  }
}

@Composable
internal fun DesktopStatusDetailsDialog(
    presentation: DesktopStatusBarPresentation,
    onDismiss: () -> Unit,
) {
  IdeDialog(
      onDismissRequest = onDismiss,
      title = { Text("Provider details") },
      content = {
        Column(Modifier.heightIn(max = 360.dp).verticalScroll(rememberScrollState())) {
          presentation.provider?.let { provider ->
            DiagnosticText(
                provider.detail,
                color = if (provider.remoteProvider) Warning else SecondaryText,
                modifier = Modifier.padding(bottom = 6.dp),
            )
          }
          Text(presentation.modelsLabel, modifier = Modifier.padding(bottom = 6.dp))
          DiagnosticText(presentation.modelsDetail, color = SecondaryText)
        }
      },
      actions = {
        MiniOrcaButton(onClick = onDismiss, tone = ActionTone.Primary) { Text("Close") }
      },
  )
}

private fun capturedRunProviderPresentation(
    state: DesktopState
): DesktopStatusProviderPresentation? {
  val run =
      state.analysisRun.run?.takeIf { it.identity.projectId == state.project?.projectId }
          ?: return null
  val providers = run.plan.providers
  if (providers.isEmpty()) return null
  val remote = providers.count { it.model.remoteProvider }
  val detail =
      providers.joinToString("; ") { provider ->
        "${provider.model.scope}: ${provider.model.profile} · ${provider.model.model} · ${provider.model.providerOrigin} · ${if (provider.model.remoteProvider) "remote" else "local"}"
      }
  return DesktopStatusProviderPresentation(
      "Captured providers for the displayed run: $detail. This describes the run configuration, not a live connection.",
      remoteProvider = remote > 0,
  )
}

private fun providerPresentation(
    provider: DesktopStatusProvider
): DesktopStatusProviderPresentation? {
  val model = provider.model
  if (model.model.isBlank() && model.profile.isBlank() && model.providerOrigin.isBlank())
      return null
  return DesktopStatusProviderPresentation(
      modelDestinationLabel(provider.scope, model),
      remoteProvider = model.remoteProvider,
  )
}
