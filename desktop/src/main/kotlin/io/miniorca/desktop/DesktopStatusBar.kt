package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal data class DesktopStatusProvider(
    val scope: ModelScope,
    val model: ScopedModel,
)

internal data class DesktopStatusProviderPresentation(
    val detail: String,
    val remoteProvider: Boolean,
    val capturedRun: Boolean = false,
)

internal data class DesktopStatusBarPresentation(
    val provider: DesktopStatusProviderPresentation?,
    val modelsLabel: String,
    val modelsDetail: String,
    val configuredScopes: List<DesktopStatusProvider> = emptyList(),
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
  val complete = configured.all { modelConfigurationAvailable(it.model) }
  val models =
      configured
          .map { it.model }
          .distinctBy { Triple(it.providerOrigin, it.model, it.remoteProvider) }
  val cloud = models.count { it.remoteProvider }
  return DesktopStatusBarPresentation(
      provider,
      if (complete) "Models: ${models.size - cloud} local · $cloud cloud"
      else "Models: unavailable",
      if (complete) "Distinct configured models by destination; shared models are counted once."
      else "Model counts are unavailable until all configured model scopes have been loaded.",
      configuredScopes = configured,
  )
}

@Composable
internal fun PersistentStatusBar(
    presentation: DesktopStatusBarPresentation,
    onOpenDetails: () -> Unit,
    modifier: Modifier = Modifier,
    detailsFocusRequester: FocusRequester? = null,
) {
  Column(
      modifier
          .fillMaxWidth()
          .background(ActivityRail)
          .drawBehind {
            val stroke = 1.dp.toPx()
            drawLine(PaneSeparator, Offset(0f, stroke / 2), Offset(size.width, stroke / 2), stroke)
          }
          .testTag("model-count-footer")) {
        Row(
            Modifier.fillMaxWidth().heightIn(min = 32.dp).padding(horizontal = 16.dp),
            horizontalArrangement = Arrangement.End,
            verticalAlignment = Alignment.CenterVertically,
        ) {
          ChromeButton(
              onClick = onOpenDetails,
              modifier =
                  (detailsFocusRequester?.let { Modifier.focusRequester(it) } ?: Modifier).testTag(
                      "model-count-action"),
              contentPadding = PaddingValues(horizontal = 8.dp, vertical = 4.dp),
              accessibleName = "Configured model details",
              tooltip = null,
          ) {
            Text(
                presentation.modelsLabel,
                color = SecondaryText,
                style = IdeTypography.resultCode.copy(lineHeight = 16.sp),
                textAlign = TextAlign.End)
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
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
          Text(
              "Current configured scopes",
              color = PrimaryText,
              style = IdeTypography.workspaceHeading)
          Text(presentation.modelsLabel)
          DiagnosticText(presentation.modelsDetail, color = SecondaryText)
          presentation.configuredScopes.forEach { provider -> ConfiguredProviderRow(provider) }
          presentation.provider?.let { provider ->
            IdeHorizontalSeparator()
            Text(
                if (provider.capturedRun) "Displayed run configuration"
                else "Current workspace configuration",
                color = PrimaryText,
                style = IdeTypography.workspaceHeading)
            DiagnosticText(
                provider.detail,
                color = if (provider.remoteProvider) Warning else SecondaryText,
                modifier = Modifier.padding(bottom = 6.dp),
            )
          }
          DiagnosticText("Provider health · unchecked", color = SecondaryText)
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
      "Run configuration · $detail",
      remoteProvider = remote > 0,
      capturedRun = true,
  )
}

private fun providerPresentation(
    provider: DesktopStatusProvider
): DesktopStatusProviderPresentation? {
  val model = provider.model
  if (model.model.isBlank() && model.profile.isBlank() && model.providerOrigin.isBlank())
      return null
  return DesktopStatusProviderPresentation(
      if (modelConfigurationAvailable(model)) modelDestinationLabel(provider.scope, model)
      else "${provider.scope.label}: Configuration unavailable",
      remoteProvider = model.remoteProvider,
  )
}

private fun modelConfigurationAvailable(model: ScopedModel): Boolean =
    model.model.isNotBlank() && model.providerOrigin.isNotBlank()

@Composable
internal fun ConfiguredProviderRow(provider: DesktopStatusProvider) {
  val model = provider.model
  SelectionContainer {
    Column(
        Modifier.fillMaxWidth().background(EditorCanvas).padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp)) {
          Text(provider.scope.label, color = PrimaryText, style = IdeTypography.workspaceHeading)
          Text(
              if (!modelConfigurationAvailable(model)) "Configuration unavailable"
              else if (model.remoteProvider) "Cloud · configured" else "Local · configured",
              color = if (model.remoteProvider) Warning else SecondaryText,
              style = IdeTypography.workspaceMetadata)
          Text(
              "Model: ${model.model.ifBlank { "Unavailable" }}",
              color = PrimaryText,
              style = IdeTypography.resultCode)
          Text(
              "Destination: ${model.providerOrigin.ifBlank { "Unavailable" }}",
              color = SecondaryText,
              style = IdeTypography.resultCode)
          if (model.profile.isNotBlank())
              Text(
                  "Profile: ${model.profile}",
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
          if (model.reasoningEffort.isNotBlank())
              Text(
                  "Reasoning: ${model.reasoningEffort}",
                  color = SecondaryText,
                  style = IdeTypography.workspaceMetadata)
        }
  }
}
