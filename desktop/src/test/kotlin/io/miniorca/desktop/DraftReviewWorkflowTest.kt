package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DraftReviewWorkflowTest {
  @Test
  fun editedCandidateRejectsLateCheckCompletionAndOldApplyEvidence() {
    val validated =
        draft(
            2,
            "validated",
            DeclarationValidation(
                true, "symbol_plus_imports", diff = UnifiedDiff("main.go", "main.go")),
            "replace_symbol")
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = validated.id,
            draftRevision = validated.revision,
            draftHash = validated.hash)
    val ready =
        DesktopState(
            projectState = ProjectWorkspaceState(project()),
            selection = FileSelectionState(selectedFile = file()),
            review =
                DraftReviewState(
                    draft = validated, editor = editableDraft(validated), checks = checks))
    assertTrue(
        draftReviewEligibility(
                ready.review.editor, ready.review.draft, ready.review.checks, file(), project())
            .eligible)
    val pending = ready.reduce(DesktopEvent.ChecksStarted(7, CheckCandidate(validated)))
    val edited = pending.reduce(DesktopEvent.DraftEdited(imports = listOf("fmt")))
    val late = edited.reduce(DesktopEvent.ChecksCompleted(7, checks))
    assertEquals("fmt", late.review.editor?.imports?.single())
    assertEquals(DraftEditorStatus.Dirty, late.review.editor?.status)
    assertNull(late.review.checks)
    assertNull(late.review.checkAttempt)
    assertFalse(
        draftReviewEligibility(late.review.editor, late.review.draft, checks, file(), project())
            .eligible)
    assertFalse(draftApplyEligibility(late.review.draft, checks, file()).eligible)
    val discarded = late.reduce(DesktopEvent.DraftDiscarded)
    assertNull(discarded.review.draft)
    assertNull(discarded.reduce(DesktopEvent.ChecksCompleted(7, checks)).review.checks)
  }

  @Test
  fun editedDraftRequiresFreshValidationAndChecksBeforeTheFakeTransportCanApplyAndUndo() {
    listOf("replace_symbol", "create_symbol").forEach { mode ->
      val requests = mutableListOf<String>()
      val client =
          ApiClient(
              transport =
                  DaemonTransport { method, path, body ->
                    requests += "$method $path ${body.orEmpty()}"
                    when (method to path) {
                      "PATCH" to "/api/projects/current/drafts/draft" -> {
                        assertContains(body.orEmpty(), "\"expected_revision\":2")
                        TransportResponse(
                            200, draftJson(3, "edited", validation = "null", mode = mode))
                      }
                      "POST" to "/api/projects/current/drafts/draft/validate" ->
                          TransportResponse(
                              200,
                              draftJson(
                                  3,
                                  "validated",
                                  validation =
                                      "{\"applicable\":true,\"scope_mode\":\"symbol_plus_imports\",\"diff\":{\"old_path\":\"main.go\",\"new_path\":\"main.go\",\"lines\":[]}}",
                                  mode = mode))
                      "POST" to "/api/projects/current/drafts/draft/checks" ->
                          TransportResponse(
                              200,
                              "{\"target_path\":\"main.go\",\"applicable\":true,\"draft_id\":\"draft\",\"draft_revision\":3,\"draft_hash\":\"validated\",\"checks\":[]}")
                      "POST" to "/api/projects/current/apply" ->
                          TransportResponse(
                              200,
                              "{\"project_revision\":\"next\",\"post_apply_hash\":\"after\",\"undo_available\":true}")
                      "POST" to "/api/projects/current/undo" ->
                          TransportResponse(
                              200,
                              "{\"project_revision\":\"restored\",\"post_apply_hash\":\"base\",\"undo_available\":false}")
                      else -> error("Unexpected request: $method $path")
                    }
                  })
      val original =
          draft(
              2,
              "base",
              validation =
                  DeclarationValidation(
                      true, "symbol_plus_imports", diff = UnifiedDiff("main.go", "main.go")),
              mode = mode)
      val editedState =
          DesktopState(
                  review = DraftReviewState(draft = original, editor = editableDraft(original)))
              .reduce(DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }"))

      assertFalse(
          draftReviewEligibility(
                  editedState.review.editor,
                  editedState.review.draft,
                  editedState.review.checks,
                  file(),
                  project())
              .eligible)
      val updated =
          client.updateDraft(
              original.id,
              "revision",
              original.revision,
              editedState.review.editor!!.declaration,
              editedState.review.editor.imports)
      val validated = client.validateDraft(updated.id, "revision", updated.revision)
      val validatedState = editedState.reduce(DesktopEvent.DraftLoaded(validated))
      assertFalse(
          draftReviewEligibility(validatedState.review.editor, validated, null, file(), project())
              .eligible)
      assertFalse(requests.any { it.startsWith("POST /api/projects/current/apply") })
      val checks = client.checkDraft(validated.id, "revision", validated.revision, validated.hash)
      val checkedState = validatedState.reduce(DesktopEvent.ChecksLoaded(checks))
      assertTrue(checks.checks.isEmpty())
      val rerunning = checkedState.reduce(DesktopEvent.ChecksStarted(1, CheckCandidate(validated)))
      assertFalse(
          draftReviewEligibility(
                  rerunning.review.editor,
                  rerunning.review.draft,
                  rerunning.review.checks,
                  file(),
                  project(),
                  rerunning.review.checkAttempt)
              .eligible)
      val canceled =
          rerunning.reduce(
              DesktopEvent.ChecksStopped(1, ValidationAttemptStatus.Canceled, "Canceled by user"))
      assertFalse(
          draftReviewEligibility(
                  canceled.review.editor,
                  canceled.review.draft,
                  canceled.review.checks,
                  file(),
                  project(),
                  canceled.review.checkAttempt)
              .eligible)
      assertTrue(
          canceled
              .reduce(DesktopEvent.ChecksStarted(2, CheckCandidate(validated)))
              .reduce(DesktopEvent.ChecksCompleted(2, checks))
              .review
              .checkAttempt == null)

      assertTrue(
          draftReviewEligibility(
                  checkedState.review.editor,
                  checkedState.review.draft,
                  checkedState.review.checks,
                  file(),
                  project())
              .eligible)
      val applied = client.applyDraft(validated)
      val undone = client.undo("project", applied.projectRevision, applied.postApplyHash)

      assertTrue(applied.undoAvailable)
      assertFalse(undone.undoAvailable)
      assertTrue(requests.any { it.startsWith("POST /api/projects/current/apply") })
      assertTrue(requests.any { it.startsWith("POST /api/projects/current/undo") })
    }
  }

  @Test
  fun revisionChangingIndexRetainsEditableBufferAndFindingsButRevokesDraftAuthority() {
    val validated =
        draft(
            2,
            "validated",
            DeclarationValidation(
                true,
                "symbol_plus_imports",
                diagnostics = listOf(DeclarationFinding("warning", "Prior diagnostic")),
                diff = UnifiedDiff("main.go", "main.go")),
            "replace_symbol")
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = validated.id,
            draftRevision = validated.revision,
            draftHash = validated.hash)
    val finding =
        UnifiedFinding(projectId = "project", projectRevision = "revision", freshness = "fresh")
    val overview = ProjectOverview(projectId = "project", projectRevision = "revision")
    val index = ProjectIndex("project", "revision")
    val attempt = ProjectIndexingAttempt(1, "project", "revision", "/tmp/project")
    val original =
        DesktopState(
            projectState =
                ProjectWorkspaceState(
                    project = project(),
                    index = index,
                    overview = overview,
                    sourceChangeObserved = true),
            selection = FileSelectionState(selectedFile = file()),
            findings = FindingsState(findings = listOf(finding)),
            review =
                DraftReviewState(
                    draft = validated,
                    editor =
                        editableDraft(validated)
                            .copy(declaration = "func Run() error { return nil }"),
                    checks = checks),
            jobs = JobState(status = "Previous work"))
    assertTrue(
        draftReviewEligibility(original.review.editor, validated, checks, file(), project())
            .eligible)

    val running = original.reduce(DesktopEvent.ProjectIndexingStarted(attempt))
    val failed =
        running.reduce(
            DesktopEvent.ProjectIndexingStopped(
                attempt, ProjectIndexingOutcome.Failed("Index unavailable")))
    assertEquals(original.review, failed.review)
    assertEquals(index, failed.index)
    assertEquals(overview, failed.overview)
    assertEquals(listOf(finding), failed.findings.findings)

    val unchanged = running.reduce(DesktopEvent.ProjectIndexingCompleted(attempt, index))
    assertEquals(original.review, unchanged.review)
    assertEquals(overview, unchanged.overview)
    assertEquals(listOf(finding), unchanged.findings.findings)
    assertEquals("Project inventory refreshed", unchanged.status)
    assertTrue(
        draftReviewEligibility(
                unchanged.review.editor,
                unchanged.review.draft,
                unchanged.review.checks,
                file(),
                unchanged.project)
            .eligible)

    val changed =
        running.reduce(
            DesktopEvent.ProjectIndexingCompleted(attempt, ProjectIndex("project", "next")))
    assertEquals("next", changed.project?.projectRevision)
    assertEquals(DraftEditorStatus.Stale, changed.review.editor?.status)
    assertEquals("func Run() error { return nil }", changed.review.editor?.declaration)
    assertEquals(validated.id, changed.review.draft?.id)
    assertNull(changed.review.draft?.validation)
    assertNull(changed.review.editor?.serverDraft?.validation)
    assertNull(changed.review.editor?.validationAttempt)
    assertTrue(changed.review.editor?.diagnosticsAreRetained == true)
    assertEquals(
        "Prior diagnostic", requireNotNull(changed.review.editor).diagnostics.single().message)
    assertNull(changed.review.checks)
    assertNull(changed.review.checkAttempt)
    assertFalse(
        draftReviewEligibility(
                changed.review.editor, changed.review.draft, checks, file(), changed.project)
            .eligible)
    assertFalse(draftApplyEligibility(changed.review.draft, changed.review.checks, file()).eligible)
    assertEquals(overview, changed.overview)
    assertEquals(listOf(finding), changed.findings.findings)
    assertFalse(changed.projectState.sourceChangeObserved)
  }

  @Test
  fun benchmarkInvalidationPreservesReviewEligibilityAndPriorEvidenceUntilSourceActuallyChanges() {
    val validated =
        draft(
            2,
            "validated",
            DeclarationValidation(
                true, "symbol_plus_imports", diff = UnifiedDiff("main.go", "main.go")),
            "replace_symbol")
    val checks =
        DraftCheckReport(
            "main.go", true, draftId = "draft", draftRevision = 2, draftHash = "validated")
    val choice = GoBenchmarkChoice("BenchmarkRun", listOf("go", "test", "."), "scope")
    val catalog =
        GoBenchmarkCatalog(
            draftId = validated.id,
            draftRevision = validated.revision,
            draftHash = validated.hash,
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "base",
            targetPath = "main.go",
            available = true,
            benchmarks = listOf(choice))
    val comparison =
        GoBenchmarkComparison(
            draftId = validated.id,
            draftRevision = validated.revision,
            draftHash = validated.hash,
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "base",
            targetPath = "main.go",
            benchmark = choice.name,
            scope = choice.scope,
            status = "completed")
    val original =
        DesktopState(
            projectState = ProjectWorkspaceState(project = project()),
            selection = FileSelectionState(selectedFile = file()),
            review =
                DraftReviewState(
                    draft = validated,
                    editor = editableDraft(validated),
                    checks = checks,
                    benchmark =
                        BenchmarkEvidenceState(
                            catalog = catalog,
                            selected = choice,
                            comparison = comparison,
                            discovery = BenchmarkDiscoveryOutcome.Loaded,
                            admission = BenchmarkAdmissionOutcome.Admitting)))
    val unchanged = original.reduce(DesktopEvent.SelectedFileRefreshed(file(), emptyList()))
    assertEquals(original.review, unchanged.review)
    val applied = original.reduce(DesktopEvent.Applied(ApplyResult("next", "after", true)))
    val stopped = original.reduce(DesktopEvent.GoBenchmarkDiscoveryInvalidated)
    for (state in listOf(applied, stopped)) {
      assertEquals(validated, state.review.draft)
      assertEquals(original.review.editor, state.review.editor)
      assertEquals(checks, state.review.checks)
      assertEquals(comparison, state.review.benchmark.comparison)
      assertNull(state.review.benchmark.catalog)
      assertNull(state.review.benchmark.selected)
      assertFalse(state.review.benchmark.running)
      assertTrue(
          draftReviewEligibility(
                  state.review.editor,
                  state.review.draft,
                  state.checks,
                  state.selectedFile,
                  state.project)
              .eligible,
          "Benchmarks must not become an unconditional Review/Apply prerequisite")
    }
    val changed =
        original.reduce(
            DesktopEvent.SelectedFileRefreshed(file().copy(contentHash = "after"), emptyList()))
    assertEquals(DraftEditorStatus.Stale, changed.review.editor?.status)
    assertEquals(comparison, changed.review.benchmark.comparison)
    assertNull(changed.review.benchmark.catalog)
    assertNull(changed.review.benchmark.selected)
    assertFalse(changed.review.benchmark.running)
    assertFalse(
        draftReviewEligibility(
                changed.review.editor,
                changed.review.draft,
                changed.checks,
                changed.selectedFile,
                changed.project)
            .eligible)
  }

  @Test
  fun optionalBenchmarkEvidenceDoesNotChangeCandidateCheckOrSourceReviewGuards() {
    val validated =
        draft(
            2,
            "validated",
            DeclarationValidation(
                true, "symbol_plus_imports", diff = UnifiedDiff("main.go", "main.go")),
            "replace_symbol")
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = validated.id,
            draftRevision = validated.revision,
            draftHash = validated.hash)
    val samples = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1000, 100.0, 10, 1) })
    val completed =
        GoBenchmarkComparison(
            draftId = validated.id,
            draftRevision = validated.revision,
            draftHash = validated.hash,
            projectId = validated.projectId,
            projectRevision = validated.projectRevision,
            targetPath = validated.targetPath,
            baseFileHash = validated.baseFileHash,
            benchmark = "BenchmarkRun",
            status = "completed",
            base = samples,
            candidate = samples)
    for ((label, evidence) in
        listOf(
            "absent" to BenchmarkEvidenceState(),
            "completed" to BenchmarkEvidenceState(comparison = completed),
            "inconclusive" to
                BenchmarkEvidenceState(
                    comparison = completed.copy(candidate = GoBenchmarkMeasurement())),
            "failed" to
                BenchmarkEvidenceState(
                    comparison = completed,
                    latestOutcome =
                        BenchmarkComparisonOutcome(
                            completed.copy(status = "failed", base = null, candidate = null))),
            "stale" to BenchmarkEvidenceState(comparison = completed.copy(draftHash = "older")))) {
      val ready =
          DesktopState(
              projectState = ProjectWorkspaceState(project()),
              selection = FileSelectionState(selectedFile = file()),
              review =
                  DraftReviewState(
                      draft = validated,
                      editor = editableDraft(validated),
                      checks = checks,
                      benchmark = evidence))
      fun eligible(state: DesktopState) =
          draftReviewEligibility(
                  state.review.editor,
                  state.review.draft,
                  state.review.checks,
                  state.selectedFile,
                  state.project,
                  state.review.checkAttempt)
              .eligible
      assertTrue(eligible(ready), "$label is optional evidence")
      val blocked =
          listOf(
              ready.reduce(
                  DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }")),
              ready.copy(review = ready.review.copy(checks = null)),
              ready.copy(review = ready.review.copy(checks = checks.copy(draftHash = "other"))),
              ready.copy(
                  review =
                      ready.review.copy(
                          draft = validated.copy(validation = null),
                          editor = editableDraft(validated.copy(validation = null)))),
              ready.copy(
                  selection =
                      ready.selection.copy(selectedFile = file().copy(contentHash = "changed"))),
              ready.copy(
                  projectState =
                      ready.projectState.copy(project = project().copy(projectRevision = "next"))),
              ready.reduce(DesktopEvent.ChecksStarted(1, CheckCandidate(validated))))
      blocked.forEachIndexed { index, state ->
        assertFalse(eligible(state), "$label cannot bypass guard $index")
      }
    }
  }

  private fun draft(
      revision: Long,
      hash: String,
      validation: DeclarationValidation?,
      mode: String
  ) =
      DeclarationDraft(
          "draft",
          "project",
          "revision",
          "base",
          "main.go",
          mode,
          "Run",
          "func Run() {}",
          revision = revision,
          hash = hash,
          validation = validation)

  private fun file() =
      ProjectFileInfo(
          "main.go",
          "base",
          "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)

  private fun project() =
      ProjectAnalysis(
          "project",
          "revision",
          "project",
          "/tmp/project",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 1,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun draftJson(revision: Long, hash: String, validation: String, mode: String) =
      "{\"id\":\"draft\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"target_path\":\"main.go\",\"mode\":\"$mode\",\"target_symbol\":\"Run\",\"declaration\":\"func Run() error { return nil }\",\"imports\":[],\"revision\":$revision,\"hash\":\"$hash\",\"state\":\"valid\",\"validation\":$validation}"
}
