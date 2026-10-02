package io.miniorca.desktop

import java.nio.file.Files
import java.nio.file.Path
import java.security.MessageDigest
import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

class CalibratedWorkflowAcceptanceTest {
  @Test
  fun restoredProjectReplacementFunctionAndTypeCompleteTheExplicitGuardedJourney() {
    for (kind in
        listOf<DeclarationCreationKind?>(
            null, DeclarationCreationKind.Function, DeclarationCreationKind.Type)) {
      DeclarationJourney(kind).use { journey ->
        val presenter = journey.presenter
        journey.restoreAndSelect()
        assertEquals(Workspace.Editor, presenter.snapshot.value.state.workspace)
        assertEquals(listOf("POST /api/projects/restore"), journey.writes())
        journey.requestCandidate()
        val generated = presenter.snapshot.value.state.review.draft!!
        presenter.dispatch(
            DesktopEvent.DraftEdited("// Reviewed locally\n${generated.declaration}"))
        assertEquals(DraftEditorStatus.Dirty, presenter.snapshot.value.state.review.editor?.status)
        presenter.applyEditableDraft()
        journey.drain()
        assertEquals(journey.original, Files.readString(journey.source))
        assertFalse(journey.writes().any { it.endsWith("/apply") })
        presenter.validateEditableDraft()
        journey.drain()
        assertEquals(DraftEditorStatus.Valid, presenter.snapshot.value.state.review.editor?.status)
        presenter.runDraftChecks()
        journey.drain()
        if (kind == null) {
          assertEquals(
              "failed", presenter.snapshot.value.state.review.checks?.checks?.single()?.state)
          presenter.reviseWithCheckOutput(ChatEditMode.ReplaceSymbol, "")
          journey.drain()
          assertEquals(1, presenter.snapshot.value.state.chat.session?.repairCount)
          assertNull(presenter.snapshot.value.state.review.checks)
          presenter.validateEditableDraft()
          journey.drain()
          presenter.runDraftChecks()
          journey.drain()
          assertEquals(2, journey.writes().count { it.contains("execution-trust") })
        }
        val ready = reviewToolWindowState(presenter.snapshot.value.state)
        assertTrue(
            draftReviewEligibility(
                    ready.editor, ready.draft, ready.checks, ready.selected, ready.project)
                .eligible)
        val writesBeforeReview = journey.writes()
        ComposeVisualFixture(1024, 768) { ReviewDiffCanvas(ready.draft) }
            .use { fixture ->
              fixture.render()
              fixture.clickDescription("Unified diff")
              fixture.render()
              assertFalse(fixture.hasEditableText("Read-only composed diff"))
            }
        assertEquals(writesBeforeReview, journey.writes())
        assertEquals(journey.original, Files.readString(journey.source))
        presenter.applyEditableDraft()
        presenter.applyEditableDraft()
        presenter.refreshSelectedFile()
        journey.drain()
        val applied = presenter.snapshot.value.state
        assertNotNull(applied.review.applied)
        assertEquals(DraftMutationOperation.Apply, applied.review.receiptScope?.operation)
        assertEquals(Files.readString(journey.source), applied.selectedFile?.content)
        assertTrue(Files.readString(journey.source).contains(ready.draft!!.declaration))
        assertTrue(
            undoEligibility(
                    applied.project,
                    applied.selectedFile,
                    applied.review.applied,
                    applied.review.receiptScope)
                .eligible)
        presenter.undoAppliedDraft()
        presenter.undoAppliedDraft()
        journey.drain()
        assertEquals(journey.original, Files.readString(journey.source))
        val undone = presenter.snapshot.value.state
        assertEquals(journey.original, undone.selectedFile?.content)
        assertEquals(DraftMutationOperation.Undo, undone.review.receiptScope?.operation)
        presenter.undoAppliedDraft()
        journey.drain()
        assertEquals(1, journey.writes().count { it.endsWith("/apply") })
        assertEquals(1, journey.writes().count { it.endsWith("/undo") })
      }
    }
  }

  @Test
  fun externalEditsAndFailedFreshnessReadsBlockReviewAndReceiptWithoutLosingTheDraft() {
    for (afterApply in listOf(false, true)) {
      for (readFails in listOf(false, true)) {
        DeclarationJourney(DeclarationCreationKind.Function).use { journey ->
          journey.restoreAndSelect()
          journey.requestCandidate()
          val presenter = journey.presenter
          presenter.validateEditableDraft()
          journey.drain()
          presenter.runDraftChecks()
          journey.drain()
          val declaration = presenter.snapshot.value.state.review.editor!!.declaration
          if (afterApply) {
            presenter.applyEditableDraft()
            journey.drain()
          }
          val receipt = presenter.snapshot.value.state.review.applied
          val writes = journey.writes()
          Files.writeString(journey.source, Files.readString(journey.source) + "// external edit\n")
          journey.failSourceRead = readFails
          presenter.refreshSelectedFile()
          journey.drain()
          if (!afterApply) {
            assertEquals(declaration, presenter.snapshot.value.state.review.editor?.declaration)
            assertEquals(
                DraftEditorStatus.Stale, presenter.snapshot.value.state.review.editor?.status)
            presenter.applyEditableDraft()
          } else {
            assertEquals(receipt, presenter.snapshot.value.state.review.applied)
            presenter.undoAppliedDraft()
          }
          journey.drain()
          assertEquals(writes, journey.writes())
          assertTrue(Files.readString(journey.source).endsWith("// external edit\n"))
          if (readFails) assertNull(presenter.snapshot.value.state.selectedFile)
        }
      }
    }
  }
}

/** Real presenter and wire decoding; only the daemon/provider and guarded disk writes are fakes. */
private class DeclarationJourney(val kind: DeclarationCreationKind?) : AutoCloseable {
  val directory: Path = Files.createTempDirectory("mini-orca-declaration-journey-")
  val source: Path = directory.resolve("main.go")
  val original = "package main\n\nfunc Run() int { return 1 }\n"
  private var revision = "initial"
  private val symbol = SymbolInfo("Run", "function", "func Run() int", 3, 3, "exact", true)
  private val target = if (kind == null) "Run" else "Added"
  private val mode = if (kind == null) ChatEditMode.ReplaceSymbol else ChatEditMode.CreateSymbol
  private val task =
      if (kind == null)
          BugTaskSpec(
              targetPath = "main.go",
              targetSymbol = "Run",
              goTestCandidate = GoTestCandidateSpec("TestRun", "func TestRun(t *testing.T) {}"))
      else null
  private var session = ChatSession()
  private var draft = DeclarationDraft()
  private var proposals = 0
  private var checks = 0
  private var appliedHash: String? = null
  var failSourceRead = false
  private val calls = mutableListOf<String>()
  private val dispatcher =
      object : CoroutineDispatcher() {
        val pending = ArrayDeque<Runnable>()

        override fun dispatch(context: CoroutineContext, block: Runnable) {
          pending.addLast(block)
        }
      }
  private val scope = CoroutineScope(SupervisorJob() + dispatcher)
  private val store = LastProjectStore(InMemoryPreferences()).apply { save(directory.toString()) }
  val presenter =
      DesktopWorkflowPresenter(
          ApiClient(transport = DaemonTransport(::respond)), store, scope, dispatcher)

  init {
    Files.writeString(source, original)
  }

  fun drain() {
    var iterations = 0
    while (dispatcher.pending.isNotEmpty()) {
      check(iterations++ < 1000) { "Unexpected ongoing work" }
      dispatcher.pending.removeFirst().run()
    }
  }

  fun restoreAndSelect() {
    presenter.start()
    drain()
    assertEquals(directory.toString(), presenter.snapshot.value.state.project?.path)
    assertEquals(Workspace.Summary, presenter.snapshot.value.state.workspace)
    presenter.selectFile("main.go")
    drain()
    presenter.dispatch(DesktopEvent.SymbolSelected(symbol))
  }

  fun requestCandidate() {
    if (task != null)
        presenter.dispatch(DesktopEvent.SuggestionPrepared("fix", "Return two", symbol, task))
    val beforeConsent = writes()
    presenter.sendChatMessage(mode, target, "Return two", creationKind = kind)
    drain()
    assertEquals(beforeConsent, writes(), "A remote destination requires explicit scope consent")
    presenter.setProviderConfirmation(ModelScope.Function, true)
    presenter.sendChatMessage(mode, target, "Return two", creationKind = kind)
    drain()
    assertNotNull(presenter.snapshot.value.state.review.draft, presenter.snapshot.value.state.error)
  }

  fun writes(): List<String> = calls.filter { !it.startsWith("GET ") }

  private fun hash(content: String) =
      MessageDigest.getInstance("SHA-256").digest(content.toByteArray()).joinToString("") {
        "%02x".format(it)
      }

  private fun file(): ProjectFileInfo {
    val content = Files.readString(source)
    return ProjectFileInfo(
        "main.go",
        hash(content),
        "main.go",
        language = "Go",
        sizeBytes = content.length.toLong(),
        lineCount = content.lines().size,
        modifiedAt = "",
        binary = false,
        content = content)
  }

  private fun project() =
      ProjectAnalysis(
          "journey",
          revision,
          "Journey",
          directory.toString(),
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 3,
          summary = "",
          aiStatus = "missing",
          analyzedAt = "")

  private inline fun <reified T> response(value: T) =
      TransportResponse(200, Json.encodeToString(value))

  private fun respond(method: String, route: String, body: String?): TransportResponse {
    calls += "$method $route"
    val path = route.substringBefore('?')
    val request = body?.let { Json.parseToJsonElement(it).jsonObject }
    return when {
      path == "/status" -> response(DaemonStatus("running", "test"))
      path == "/api/models/current" ->
          response(
              ModelCatalog(
                  ModelScope.entries.associate {
                    it.wireValue to
                        ScopedModel(
                            it.wireValue, "fake", "fake-model", "https://provider.example", true)
                  }))
      path == "/api/projects/restore" -> response(project())
      path.endsWith("/index") ->
          response(
              ProjectIndex(
                  "journey",
                  revision,
                  files = listOf(IndexedFile("main.go", file().contentHash, "Go", false))))
      path.endsWith("/files/info") ->
          if (failSourceRead) TransportResponse(503, """{"message":"Source read unavailable"}""")
          else response(file())
      path.endsWith("/files/symbols") ->
          response(SymbolsResponse("journey", revision, "main.go", listOf(symbol)))
      path.endsWith("/files/analysis") ->
          TransportResponse(200, """{"path":"main.go","status":"missing"}""")
      path.endsWith("/impact") -> TransportResponse(200, """{"target_path":"main.go"}""")
      path.endsWith("/git") -> TransportResponse(200, """{"available":false}""")
      path.endsWith("/analysis/run") || path.endsWith("/scan") -> TransportResponse(204, "")
      path.endsWith("/overview") ||
          path.endsWith("/findings") ||
          path.endsWith("/analysis/files") -> TransportResponse(200, "{}")
      path.endsWith("/chat/sessions") -> {
        session =
            ChatSession(
                "session",
                "journey",
                revision,
                file().contentHash,
                "main.go",
                mode.wireValue,
                target,
                "active",
                taskSpec = task)
        response(session)
      }
      path.endsWith("/messages") -> {
        assertEquals("true", request!!["confirm_remote_provider"]?.jsonPrimitive?.content)
        proposals++
        val declaration =
            when (kind) {
              DeclarationCreationKind.Type -> "type Added struct { Value int }"
              else -> "func $target() int { return 2 }"
            }
        draft =
            DeclarationDraft(
                "draft-$proposals",
                "journey",
                revision,
                file().contentHash,
                "main.go",
                mode.wireValue,
                target,
                declaration,
                revision = 1,
                hash = "candidate-$proposals-1",
                taskSpec = task,
                parentDraftId = session.latestDraftId)
        session = session.copy(latestDraftId = draft.id)
        response(
            ChatDraftProposal(
                session.id, draft, ChatSessionMessage("assistant", "Candidate ready")))
      }
      method == "PATCH" && path.contains("/drafts/") -> {
        assertEquals(
            draft.revision.toString(), request!!["expected_revision"]?.jsonPrimitive?.content)
        draft =
            draft.copy(
                declaration = request.getValue("declaration").jsonPrimitive.content,
                imports = request["imports"]?.jsonArray?.map { it.jsonPrimitive.content }.orEmpty(),
                revision = draft.revision + 1,
                hash = "candidate-$proposals-${draft.revision + 1}",
                validation = null)
        response(draft)
      }
      path.endsWith("/validate") -> {
        draft =
            draft.copy(
                validation =
                    DeclarationValidation(
                        true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")))
        response(draft)
      }
      path.endsWith("/execution-trust") ->
          response(
              ExecutionTrust(
                  "journey",
                  revision,
                  method == "POST",
                  listOf(
                      if (method == "GET") listOf("go", "test", "./...", "-run", "^TestRun$")
                      else listOf("go", "test", "./..."))))
      path.endsWith("/checks") -> {
        val passed = kind != null || checks++ > 0
        response(
            DraftCheckReport(
                "main.go",
                passed,
                listOf(
                    DraftCheck(
                        "go_test",
                        true,
                        if (passed) "passed" else "failed",
                        output = if (passed) "ok" else "TestRun expected two")),
                draft.id,
                draft.revision,
                draft.hash))
      }
      path.endsWith("/apply") -> {
        assertEquals("true", request!!["confirm"]?.jsonPrimitive?.content)
        assertEquals(draft.hash, request["draft_hash"]?.jsonPrimitive?.content)
        assertEquals(draft.baseFileHash, file().contentHash)
        val before = file().contentHash
        Files.writeString(
            source,
            (if (kind == null) "package main\n\n" else original + "\n") + draft.declaration + "\n")
        revision = "applied"
        appliedHash = file().contentHash
        response(
            ApplyResult(
                revision,
                file().contentHash,
                true,
                AuditEntry(
                    "apply",
                    "main.go",
                    "applied",
                    "2026-10-03",
                    beforeHash = before,
                    afterHash = file().contentHash)))
      }
      path.endsWith("/undo") -> {
        assertEquals(appliedHash, request!!["post_apply_hash"]?.jsonPrimitive?.content)
        assertEquals(appliedHash, file().contentHash)
        Files.writeString(source, original)
        revision = "undone"
        response(
            ApplyResult(
                revision,
                file().contentHash,
                false,
                AuditEntry("undo", "main.go", "undone", "2026-10-03")))
      }
      else -> error("Unexpected journey request: $method $route")
    }
  }

  override fun close() {
    presenter.close()
    scope.cancel()
    Files.deleteIfExists(source)
    Files.deleteIfExists(directory)
  }
}
