package io.miniorca.desktop

import java.io.File
import java.util.concurrent.CompletableFuture
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopProjectSwitchTest {
  private val project = resultProjectFixture()
  private val draft = DeclarationDraft(id = "draft-1", revision = 1, hash = "hash-1")
  private val remote =
      ScopedModel(scope = "analyze", profile = "remote", model = "m1", remoteProvider = true)

  private fun context(
      project: ProjectAnalysis? = this.project,
      draft: DeclarationDraft? = null,
      model: ScopedModel = remote,
      confirmed: Boolean = false,
      editor: EditableDraftState? = draft?.let(::editableDraft),
  ): ProjectSwitchContext =
      ProjectSwitchContext(
          project?.let(::SwitchProjectIdentity),
          SwitchDraftIdentity(null, draft, editor),
          SwitchAnalyzeDestination(model),
          confirmed)

  private class CleanupSwitch {
    val admission = ProjectSwitchAdmission()
    val approved =
        ProjectSwitchContext(
            SwitchProjectIdentity(resultProjectFixture()),
            SwitchDraftIdentity(null, DeclarationDraft(id = "draft"), null),
            SwitchAnalyzeDestination(ScopedModel(scope = "analyze", remoteProvider = true)),
            true)
    var context = approved
    var opening = true
    val completion = CompletableFuture<TerminalWorkspaceState>()
    var terminal = TerminalWorkspaceState()
    val effects = mutableListOf<String>()
    var error: String? = null
    var timeout: (() -> Unit)? = null
    var stopped = false
    val request = admission.choose("/next", approved)!!.requestId

    init {
      admission.approveDraft(request, approved)
    }

    fun commit() {
      commitProjectSwitch(
          request,
          admission,
          { context },
          { opening },
          SwitchTerminalCleanup(
              {
                effects += "cleanup"
                completion
              },
              { terminal }),
          { effects += "discard" },
          { effects += "import $it" },
          {},
          { error = it },
          { it() },
          { callback ->
            timeout = callback
            { stopped = true }
          })
    }

    fun finish() = completion.complete(TerminalWorkspaceState())
  }

  @Test
  fun cleanupCompletionMustRevalidateProjectDraftDestinationAndOpeningBeforeDiscard() {
    val changes =
        listOf<(CleanupSwitch) -> Unit>(
            { it.context = it.context.copy(project = null) },
            {
              it.context =
                  it.context.copy(
                      draft = it.context.draft.copy(draft = DeclarationDraft(id = "edited")))
            },
            {
              it.context =
                  it.context.copy(destination = it.context.destination.copy(model = "new-model"))
            },
            { it.context = it.context.copy(analyzeConfirmed = false) },
            { it.opening = false })
    changes.forEach { change ->
      val switch = CleanupSwitch()
      switch.commit()
      change(switch)
      switch.finish()
      assertEquals(listOf("cleanup"), switch.effects)
      assertTrue(switch.error.orEmpty().contains("has not been switched"))
      assertEquals(SwitchReviewStage.Committed, switch.admission.pending?.stage)
      assertTrue(!switch.admission.cleanupOutstanding)
      assertTrue(switch.stopped)
    }
  }

  @Test
  fun identityChangedBeforeQueuedCleanupDoesNotCloseNewProjectsShells() {
    val admission = ProjectSwitchAdmission()
    val context =
        ProjectSwitchContext(
            SwitchProjectIdentity(resultProjectFixture()),
            SwitchDraftIdentity(null, null, null),
            SwitchAnalyzeDestination(ScopedModel(scope = "analyze")),
            false)
    var current = context
    val request = admission.choose("/next", context)!!.requestId
    var posted: (() -> Unit)? = null
    var calls = 0
    var error: String? = null
    commitProjectSwitch(
        request,
        admission,
        { current },
        { true },
        SwitchTerminalCleanup(
            {
              calls++
              CompletableFuture.completedFuture(TerminalWorkspaceState())
            },
            { TerminalWorkspaceState() }),
        { calls++ },
        { calls++ },
        {},
        { error = it },
        { posted = it },
        { {} })
    current = current.copy(project = null)
    posted!!()
    assertEquals(0, calls)
    assertTrue(error.orEmpty().contains("before shell cleanup"))
  }

  @Test
  fun cleanupTimeoutIsVisibleAndLateSuccessCannotDiscardOrImport() {
    val switch = CleanupSwitch()
    switch.commit()
    switch.timeout!!()
    assertTrue(switch.error.orEmpty().contains("15 seconds"))
    assertEquals(listOf("cleanup"), switch.effects)
    switch.finish()
    assertEquals(listOf("cleanup"), switch.effects)
    assertEquals(SwitchReviewStage.Committed, switch.admission.pending?.stage)
    assertTrue(switch.error.orEmpty().contains("now finished"))
    assertTrue(!switch.admission.cleanupOutstanding)
    switch.admission.finish(switch.request)
    assertNull(switch.admission.pending)
  }

  @Test
  fun timedOutCleanupKeepsAdmissionUntilLateClosureSoCancelingNextReviewPreservesTabs() {
    val switch = CleanupSwitch()
    switch.commit()
    switch.timeout!!()
    assertTrue(switch.admission.cleanupOutstanding)
    // Attempting to close the timed-out review cannot relinquish terminal ownership.
    switch.admission.finish(switch.request)
    assertEquals(SwitchReviewStage.Committed, switch.admission.pending?.stage)
    assertNull(switch.admission.choose("/another", switch.context))
    assertEquals(listOf("cleanup"), switch.effects)

    // Even a delayed cleanup result cannot dispatch the original import.
    switch.finish()
    assertEquals(listOf("cleanup"), switch.effects)
    assertTrue(!switch.admission.cleanupOutstanding)
    switch.admission.finish(switch.request)
    val next = switch.admission.choose("/another", switch.context)!!
    switch.admission.dismiss(next.requestId)
    assertNull(switch.admission.pending)
    assertEquals(listOf("cleanup"), switch.effects)
  }

  @Test
  fun verifiedCleanupImportsOnceButIncompleteOrExceptionalCleanupDoesNot() {
    val success = CleanupSwitch()
    success.commit()
    success.commit()
    success.finish()
    assertEquals(listOf("cleanup", "discard", "import /next"), success.effects)
    assertNull(success.admission.pending)
    assertTrue(success.stopped)

    val pending = CleanupSwitch()
    pending.commit()
    pending.completion.complete(TerminalWorkspaceState(tabs = listOf(TerminalTabState(1, "Shell"))))
    assertEquals(listOf("cleanup"), pending.effects)
    assertTrue(pending.error.orEmpty().contains("incomplete"))

    val failed = CleanupSwitch()
    failed.commit()
    failed.completion.completeExceptionally(IllegalStateException("shell refused to stop"))
    assertEquals(listOf("cleanup"), failed.effects)
    assertTrue(failed.error.orEmpty().contains("shell refused to stop"))
  }

  @Test
  fun cleanupResultCannotAuthorizeImportIfAnotherTabAppearsBeforeUiCompletion() {
    val switch = CleanupSwitch()
    switch.commit()
    switch.terminal = TerminalWorkspaceState(tabs = listOf(TerminalTabState(2, "New shell")))
    switch.finish()
    assertEquals(listOf("cleanup"), switch.effects)
    assertTrue(switch.error.orEmpty().contains("incomplete"))
    assertEquals(SwitchReviewStage.Committed, switch.admission.pending?.stage)
  }

  @Test
  fun cleanupPendingAndSynchronousCleanupFailureKeepDraftAndProject() {
    val pending = CleanupSwitch()
    pending.commit()
    pending.completion.complete(
        TerminalWorkspaceState(
            tabs =
                listOf(TerminalTabState(1, "Shell", TerminalSessionState(cleanupPending = true)))))
    assertEquals(listOf("cleanup"), pending.effects)
    assertTrue(pending.error.orEmpty().contains("incomplete"))

    val failed = CleanupSwitch()
    val request = failed.request
    commitProjectSwitch(
        request,
        failed.admission,
        { failed.context },
        { true },
        SwitchTerminalCleanup(
            { throw IllegalStateException("cleanup unavailable") }, { TerminalWorkspaceState() }),
        { failed.effects += "discard" },
        { failed.effects += "import $it" },
        {},
        { failed.error = it },
        { it() },
        { {} })
    assertEquals(emptyList(), failed.effects)
    assertTrue(failed.error.orEmpty().contains("cleanup unavailable"))
    assertEquals(SwitchReviewStage.Committed, failed.admission.pending?.stage)
  }

  @Test
  fun chooserCancellationAndEveryReviewDismissalHaveNoPrivilegedEffects() {
    val admission = ProjectSwitchAdmission()
    val initial = context(draft = draft)
    val calls = mutableListOf<String>()
    chooseProjectDirectory({ null }) { admission.choose(it, initial) }
    assertNull(admission.pending)
    chooseProjectDirectory({ File("/next") }) { admission.choose(it, initial) }
    val first = admission.pending!!.requestId
    assertEquals("/next", admission.pending?.path)
    assertEquals(SwitchReviewStage.Draft, admission.pending?.stage)
    admission.dismiss(first)
    assertNull(admission.pending)

    val second = admission.choose("/next", initial)!!.requestId
    admission.approveDraft(first, initial)
    assertEquals(SwitchReviewStage.Draft, admission.pending?.stage)
    admission.approveDraft(second, initial)
    assertEquals(SwitchReviewStage.Provider, admission.pending?.stage)
    admission.dismiss(first)
    assertEquals(SwitchReviewStage.Provider, admission.pending?.stage)
    admission.dismiss(second)
    assertNull(admission.pending)

    val third = admission.choose("/next", initial)!!.requestId
    admission.approveDraft(third, initial)
    val confirmed = initial.copy(analyzeConfirmed = true)
    admission.approveProvider(third, confirmed)
    assertEquals(SwitchReviewStage.Final, admission.pending?.stage)
    admission.dismiss(third)
    assertNull(admission.pending)
    assertEquals(emptyList(), calls)
  }

  @Test
  fun currentIdentitiesMustBeReviewedAgainBeforeOneFinalCommit() {
    val admission = ProjectSwitchAdmission()
    val original = context(draft = draft)
    val calls = mutableListOf<String>()
    val request = admission.choose("/next", original)!!.requestId
    assertNull(admission.choose("/different", original))
    assertEquals("/next", admission.pending?.path)
    admission.approveDraft(request, original)
    val confirmed = original.copy(analyzeConfirmed = true)
    admission.approveProvider(request, confirmed)
    assertNull(
        admission.commit(
            request,
            confirmed.copy(project = SwitchProjectIdentity(project.copy(projectRevision = "new")))))
    assertEquals(SwitchReviewStage.Draft, admission.pending?.stage)
    assertNull(admission.commit(request, confirmed))
    assertEquals(SwitchReviewStage.Draft, admission.pending?.stage)
    assertEquals(emptyList(), calls)

    admission.approveDraft(request, confirmed)
    admission.commit(request, confirmed)?.let { calls += "commit ${it.path}" }
    admission.commit(request, confirmed)?.let { calls += "commit ${it.path}" }
    admission.dismiss(request)
    assertEquals(SwitchReviewStage.Committed, admission.pending?.stage)
    assertEquals(listOf("commit /next"), calls)
    admission.finish(request)
    val next = admission.choose("/next", confirmed)!!.requestId
    assertNull(admission.commit(request, confirmed))
    assertEquals(SwitchReviewStage.Draft, admission.pending?.stage)
    admission.dismiss(next)
  }

  @Test
  fun editorChangesAndDestinationChangesRevokeEarlierApproval() {
    val admission = ProjectSwitchAdmission()
    val original = context(draft = draft)
    val request = admission.choose("/next", original)!!.requestId
    admission.approveDraft(request, original)
    admission.approveProvider(request, original.copy(analyzeConfirmed = true))
    val changedEditor =
        original.copy(
            draft = original.draft.copy(editor = editDraft(editableDraft(draft), "changed")))
    assertNull(admission.commit(request, changedEditor))
    assertEquals(SwitchReviewStage.Draft, admission.pending?.stage)

    admission.approveDraft(request, changedEditor)
    admission.approveProvider(request, changedEditor)
    val newDestination =
        changedEditor.copy(
            destination = SwitchAnalyzeDestination(remote.copy(model = "m2")),
            analyzeConfirmed = false)
    assertNull(admission.commit(request, newDestination))
    assertEquals(SwitchReviewStage.Draft, admission.pending?.stage)
    admission.approveDraft(request, newDestination)
    assertEquals(SwitchReviewStage.Provider, admission.pending?.stage)
    assertNull(admission.commit(request, newDestination))
    admission.approveProvider(request, newDestination)
    assertEquals(SwitchReviewStage.Provider, admission.pending?.stage)
    admission.approveProvider(request, newDestination.copy(analyzeConfirmed = true))
    assertEquals(SwitchReviewStage.Final, admission.pending?.stage)
  }

  @Test
  fun returningToAnEarlierProjectOrProviderCannotReuseAnObservedObsoleteApproval() {
    val admission = ProjectSwitchAdmission()
    val original = context(model = ScopedModel(scope = "analyze"))
    val request = admission.choose("/next", original)!!.requestId
    assertEquals(SwitchReviewStage.Final, admission.pending?.stage)
    val otherProject = original.copy(project = SwitchProjectIdentity(project.copy(projectId = "B")))
    assertNull(admission.commit(request, otherProject))
    assertNull(admission.commit(request, original))
    assertEquals(SwitchReviewStage.Review, admission.pending?.stage)
    assertNull(admission.commit(request, original))
    admission.approveReview(request, original)
    assertEquals(SwitchReviewStage.Final, admission.pending?.stage)

    val otherDestination = original.copy(destination = SwitchAnalyzeDestination(remote))
    assertNull(admission.commit(request, otherDestination))
    assertEquals(SwitchReviewStage.Provider, admission.pending?.stage)
    assertNull(admission.commit(request, original))
    assertEquals(SwitchReviewStage.Review, admission.pending?.stage)
    assertNull(admission.commit(request, original))
    admission.approveReview(request, original)
    assertEquals(SwitchReviewStage.Final, admission.pending?.stage)
    assertEquals("/next", admission.commit(request, original)?.path)
  }

  @Test
  fun reviewMustMatchLatestIdentityBeforeFinalApproval() {
    val admission = ProjectSwitchAdmission()
    val original = context(model = ScopedModel(scope = "analyze"))
    val request = admission.choose("/next", original)!!.requestId
    val changed = original.copy(project = SwitchProjectIdentity(project.copy(projectId = "B")))
    assertNull(admission.commit(request, changed))
    assertEquals(SwitchReviewStage.Review, admission.pending?.stage)
    admission.approveReview(request, original)
    assertNull(admission.commit(request, changed))
    assertEquals(SwitchReviewStage.Review, admission.pending?.stage)
    admission.approveReview(request, changed)
    assertEquals("/next", admission.commit(request, changed)?.path)
    assertNull(admission.commit(request, changed))
  }

  @Test
  fun snapshotCapturesRealDraftAndAnalyzeConfirmationOwner() {
    val snapshot =
        DesktopWorkflowSnapshot(
            state =
                DesktopState(
                    projectState = ProjectWorkspaceState(project, resultIndexFixture()),
                    review =
                        DraftReviewState(
                            draft = draft, editor = editDraft(editableDraft(draft), "buffer"))),
            modelCatalog = ModelCatalog(mapOf(ModelScope.Analyze.wireValue to remote)),
            providerConfirmations = ScopedConfirmationState(analyze = true))
    val context = ProjectSwitchContext(snapshot)
    assertEquals(SwitchProjectIdentity(project), context.project)
    assertEquals("buffer", context.draft.editor?.declaration)
    assertEquals(SwitchAnalyzeDestination(remote), context.destination)
    assertEquals(true, context.analyzeConfirmed)
    assertEquals(SwitchReviewStage.Draft, ProjectSwitchAdmission().choose("/next", context)?.stage)
  }

  @Test
  fun localDestinationNeedsNoRemoteConfirmationAndDraftRemovalRevokesApproval() {
    val admission = ProjectSwitchAdmission()
    val local = context(draft = draft, model = ScopedModel(scope = "analyze"))
    val request = admission.choose("/next", local)!!.requestId
    admission.approveDraft(request, local)
    assertEquals(SwitchReviewStage.Final, admission.pending?.stage)
    val withoutDraft = local.copy(draft = SwitchDraftIdentity(null, null, null))
    assertNull(admission.commit(request, withoutDraft))
    assertEquals(SwitchReviewStage.Review, admission.pending?.stage)
    assertNull(admission.commit(request, withoutDraft))
    admission.approveReview(request, withoutDraft)
    assertEquals(SwitchReviewStage.Final, admission.pending?.stage)
    assertEquals("/next", admission.commit(request, withoutDraft)?.path)
  }
}
