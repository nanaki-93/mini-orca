package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class DesktopStateTest {
  @Test
  fun projectLoadClearsPriorSelectionAndDraft() {
    val project =
        ProjectAnalysis(
            "id",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 2,
            summary = "",
            aiStatus = "fresh",
            analyzedAt = "")
    val index = ProjectIndex("id", "revision")
    val state =
        DesktopState(
                selection =
                    FileSelectionState(
                        selectedFile =
                            ProjectFileInfo(
                                "main.go",
                                "hash",
                                "main.go",
                                language = "Go",
                                sizeBytes = 1,
                                lineCount = 1,
                                modifiedAt = "",
                                binary = false)),
                review = DraftReviewState(draft = DeclarationDraft(id = "draft")),
            )
            .reduce(DesktopEvent.ProjectLoaded(project, index))
    assertNull(state.selectedFile)
    assertNull(state.review.draft)
    assertEquals(project, state.project)
  }

  @Test
  fun apiClientUsesTypedTransportAndErrorMessages() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("GET", method)
                  assertEquals("/api/projects/current/index", path)
                  TransportResponse(
                      200, "{\"project_id\":\"p\",\"project_revision\":\"r\",\"files\":[]}")
                })
    assertEquals("p", client.index().projectId)
    val failed =
        ApiClient(
            transport =
                DaemonTransport { _, _, _ ->
                  TransportResponse(
                      409, "{\"message\":\"stale\",\"user_message\":\"Reload first\"}")
                })
    val error = runCatching { failed.index() }.exceptionOrNull()
    assertTrue(error is ApiException && error.message == "Reload first")
  }

  @Test
  fun apiClientReadsDaemonCanonicalVersion() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("GET", method)
                  assertEquals("/status", path)
                  TransportResponse(
                      200,
                      "{\"status\":\"running\",\"version\":\"4.4.0\",\"workflow\":\"single_coder_preview\"}")
                })

    assertEquals("4.4.0", client.status().version)
  }

  @Test
  fun indexAcceptsLegacyNullCollectionFields() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("GET", method)
                  assertEquals("/api/projects/current/index", path)
                  TransportResponse(
                      200,
                      """{"project_id":"p","project_revision":"r","files":[{"path":"main.go","content_hash":"hash","language":"Go","binary":false,"symbols":null}]}""")
                })

    val index = client.index()

    assertEquals(emptyList(), index.files.single().symbols)
  }

  @Test
  fun reindexKeepsSelectedFileAndClearsLoadingState() {
    val selected =
        ProjectFileInfo(
            "main.go",
            "hash",
            "main.go",
            language = "Go",
            sizeBytes = 1,
            lineCount = 1,
            modifiedAt = "",
            binary = false)
    val refreshed = ProjectIndex("id", "new-revision")
    val state =
        DesktopState(
                selection = FileSelectionState(selectedFile = selected),
                jobs = JobState(loading = true))
            .reduce(DesktopEvent.IndexRefreshed(refreshed))
    assertEquals(selected, state.selectedFile)
    assertEquals(refreshed, state.index)
    assertTrue(!state.loading)
  }

  @Test
  fun suggestionPreparesARequestWithoutChangingTheCurrentDraft() {
    val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
    val draft = DeclarationDraft(id = "draft")
    val state =
        DesktopState(review = DraftReviewState(draft = draft))
            .reduce(DesktopEvent.SuggestionPrepared("fix", "Handle empty input", symbol))
    assertEquals(symbol, state.selectedSymbol)
    assertEquals("fix", state.preparedAction)
    assertEquals("Handle empty input", state.preparedRequest)
    assertEquals(draft, state.review.draft)
  }

  @Test
  fun explainSymbolRemainsAReadOnlyPreparedAction() {
    val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)

    val state =
        DesktopState()
            .reduce(
                DesktopEvent.SuggestionPrepared(
                    "explain_symbol", "Show the cached explanation for Run.", symbol),
            )

    assertEquals(symbol, state.selectedSymbol)
    assertEquals("explain_symbol", state.preparedAction)
    assertEquals("Show the cached explanation for Run.", state.preparedRequest)
    assertNull(state.review.draft)
  }

  @Test
  fun fileSwitchRejectsLateEnrichmentAndClearsFileBoundState() {
    val controller = DesktopWorkflowController(projectState())
    val first = controller.beginFileLoad("first.go")!!
    assertTrue(controller.fileLoaded(first, file("first.go", "first"), emptyList()))
    val loadedFirst = controller.currentFileRequest()!!
    assertTrue(
        controller.chatLoaded(
            controller.beginChatLoad()!!.first, loadedFirst, session("first.go", "first")))

    val second = controller.beginFileLoad("second.go")!!
    assertNull(controller.state.selectedFile)
    assertNull(controller.state.chat.session)
    assertNull(controller.state.review.draft)
    assertTrue(!controller.analysisLoaded(loadedFirst, FileAnalysis("first.go", "fresh")))
    assertTrue(controller.fileLoaded(second, file("second.go", "second"), emptyList()))
    assertEquals("second.go", controller.state.selectedFile?.path)
  }

  @Test
  fun projectAndCanceledFileRequestsRejectStaleResponses() {
    val controller = DesktopWorkflowController()
    val firstProjectRequest = controller.beginProjectLoad()
    val secondProjectRequest = controller.beginProjectLoad()
    val project = project()
    val index = ProjectIndex("project", "revision")

    assertTrue(!controller.projectLoaded(firstProjectRequest, project, index))
    assertTrue(controller.projectLoaded(secondProjectRequest, project, index))
    val fileRequest = controller.beginFileLoad("main.go")!!
    assertTrue(controller.cancelFileLoad(fileRequest))
    assertTrue(!controller.fileLoaded(fileRequest, file("main.go", "hash"), emptyList()))
  }

  @Test
  fun draftEligibilityRequiresMatchingLatestValidationAndChecks() {
    val selected = file("main.go", "base")
    val draft =
        DeclarationDraft(
            id = "draft",
            baseFileHash = "base",
            targetPath = "main.go",
            revision = 2,
            hash = "latest",
            validation =
                DeclarationValidation(
                    true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
    val staleChecks =
        DraftCheckReport("main.go", true, draftId = "draft", draftRevision = 1, draftHash = "old")
    val currentChecks =
        DraftCheckReport(
            "main.go", true, draftId = "draft", draftRevision = 2, draftHash = "latest")

    assertTrue(!draftApplyEligibility(draft, staleChecks, selected).eligible)
    assertTrue(draftApplyEligibility(draft, currentChecks, selected).eligible)
  }

  @Test
  fun workspaceSwitchKeepsTheOpenFileAndDirtyDraft() {
    val selected = file("main.go", "base")
    val draft =
        DeclarationDraft(
            id = "draft",
            targetPath = "main.go",
            baseFileHash = "base",
            declaration = "func Run() {}",
            revision = 3)
    val initial =
        DesktopState(
            workspace = Workspace.Editor,
            selection = FileSelectionState(selectedFile = selected),
            review = DraftReviewState(draft = draft),
        )

    val switched = initial.reduce(DesktopEvent.WorkspaceSelected(Workspace.Bugs))

    assertEquals(Workspace.Bugs, switched.workspace)
    assertEquals(selected, switched.selectedFile)
    assertEquals(draft, switched.review.draft)
  }

  @Test
  fun findingNavigationOnlyUsesTheActiveProjectIndexAndKeepsItsContext() {
    val index =
        ProjectIndex(
            "project",
            "revision",
            files = listOf(IndexedFile("internal/main.go", "hash", "Go", false)))
    val finding =
        UnifiedFinding(
            location = FindingLocation("internal/main.go", startLine = 7, symbol = "Run"))

    assertEquals(
        EditorNavigationTarget("internal/main.go", "Run", 7),
        findingNavigationTarget(finding, index))
    assertNull(
        findingNavigationTarget(
            finding.copy(location = FindingLocation("../outside.go", startLine = 7)), index))
  }

  @Test
  fun findingNavigationPrefersExactSymbolThenLineRange() {
    val symbols =
        listOf(
            SymbolInfo(
                "Other",
                "function",
                startLine = 1,
                endLine = 3,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Run",
                "function",
                startLine = 10,
                endLine = 15,
                confidence = "exact",
                atomicTarget = true),
        )

    assertEquals(
        symbols[1], symbolForNavigation(symbols, EditorNavigationTarget("main.go", "Run", 2)))
    assertEquals(
        symbols[1], symbolForNavigation(symbols, EditorNavigationTarget("main.go", line = 12)))
    assertEquals(12, navigationFocusLine(EditorNavigationTarget("main.go", "Run", 12), symbols[1]))
    assertEquals(10, navigationFocusLine(EditorNavigationTarget("main.go", "Run"), symbols[1]))
    assertEquals(0, navigationFocusLine(EditorNavigationTarget("main.go"), null))
  }

  @Test
  fun findingNavigationResolvesAnUnambiguousMentionedDeclaration() {
    val command =
        SymbolInfo(
            "diffCmd", "var", startLine = 3, endLine = 9, confidence = "exact", atomicTarget = true)
    val index =
        ProjectIndex(
            "project",
            "revision",
            files =
                listOf(IndexedFile("command.go", "hash", "Go", false, symbols = listOf(command))))
    val finding =
        UnifiedFinding(
            title = "Cobra command behavior",
            message = "diffCmd should validate its input before running.",
            location = FindingLocation("command.go"),
        )

    val target = findingNavigationTarget(finding, index)

    assertEquals(EditorNavigationTarget("command.go", "diffCmd", 3), target)
    assertEquals(
        EditorNavigationSelection(command, 3),
        resolveEditorNavigation(listOf(command), requireNotNull(target)))
  }

  @Test
  fun sourceLineSelectionUsesTheMostSpecificValidDeclaration() {
    val symbols =
        listOf(
            SymbolInfo(
                "Invalid",
                "function",
                startLine = 0,
                endLine = 4,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Container",
                "type",
                startLine = 1,
                endLine = 20,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Nested",
                "function",
                startLine = 5,
                endLine = 8,
                confidence = "approximate",
                atomicTarget = false),
            SymbolInfo(
                "ApproximateTie",
                "function",
                startLine = 10,
                endLine = 12,
                confidence = "approximate",
                atomicTarget = false),
            SymbolInfo(
                "AtomicTie",
                "function",
                startLine = 10,
                endLine = 12,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "FirstAtomicTie",
                "function",
                startLine = 14,
                endLine = 16,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "SecondAtomicTie",
                "function",
                startLine = 14,
                endLine = 16,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Reversed",
                "function",
                startLine = 22,
                endLine = 21,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Empty",
                "function",
                startLine = 0,
                endLine = 0,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Disjoint",
                "function",
                startLine = 30,
                endLine = 31,
                confidence = "exact",
                atomicTarget = true),
        )

    assertEquals("Nested", symbolAtLine(symbols, 6)?.name)
    assertEquals("AtomicTie", symbolAtLine(symbols, 11)?.name)
    assertEquals("FirstAtomicTie", symbolAtLine(symbols, 15)?.name)
    assertEquals("Disjoint", symbolAtLine(symbols, 30)?.name)
    assertNull(symbolAtLine(symbols, 21))

    val draft = DeclarationDraft(id = "draft")
    val state =
        DesktopState(
                selection = FileSelectionState(selectedSymbol = symbols[1], focusedLine = 2),
                review = DraftReviewState(draft = draft),
            )
            .reduce(DesktopEvent.SourceLineSelected(SourceLineSelection(6, symbols[2])))

    assertEquals(symbols[2], state.selectedSymbol)
    assertEquals(6, state.selection.focusedLine)
    assertEquals(draft, state.review.draft)

    val outsideDeclaration =
        state.reduce(DesktopEvent.SourceLineSelected(sourceLineSelection(symbols, 21)))

    assertNull(outsideDeclaration.selectedSymbol)
    assertEquals(21, outsideDeclaration.selection.focusedLine)
    assertEquals(draft, outsideDeclaration.review.draft)
  }

  @Test
  fun discardingForANewEditClearsOnlyTheInMemoryConversationDraftAndChecks() {
    val selected =
        ProjectFileInfo(
            "main.go",
            "hash",
            "main.go",
            language = "Go",
            sizeBytes = 1,
            lineCount = 1,
            modifiedAt = "",
            binary = false)
    val draft = DeclarationDraft(id = "draft")
    val receipt = ApplyResult("revision", "post-apply", true)
    val initial =
        DesktopState(
            selection = FileSelectionState(selectedFile = selected),
            chat = ChatState(ChatSession(id = "session")),
            review =
                DraftReviewState(
                    draft = draft,
                    editor = editableDraft(draft),
                    checks = DraftCheckReport("main.go", true),
                    applied = receipt),
        )

    val discarded = initial.reduce(DesktopEvent.DraftDiscarded)

    assertEquals(selected, discarded.selectedFile)
    assertNull(discarded.chat.session)
    assertNull(discarded.review.draft)
    assertNull(discarded.review.editor)
    assertNull(discarded.review.checks)
    assertEquals(receipt, discarded.review.applied)
  }

  @Test
  fun contextInspectorClientKeepsOnlySourceFreeManifestMetadata() {
    val client =
        ApiClient(
            "https://provider.example",
            DaemonTransport { _, _, _ ->
              TransportResponse(
                  200,
                  """{"included":[{"path":"main.go","size_bytes":20,"hash":"sha256:base","estimated_tokens":5}],"excluded":[{"path":".env","include":false,"reason":"secret"}],"estimated_tokens":5,"byte_limit":1024,"token_limit":256,"scope":"function","model":"local-code","provider_origin":"http://localhost:11434","content":"private source must not reach the UI model"}""")
            })

    val manifest = client.context("main.go")

    assertEquals("Remote endpoint", client.endpointLocality())
    assertEquals("main.go", manifest.included.single().path)
    assertEquals("secret", manifest.excluded.single().reason)
    assertEquals("function", manifest.scope)
    assertEquals("local-code", manifest.model)
    assertEquals("http://localhost:11434", manifest.providerOrigin)
    assertTrue(!Json.encodeToString(manifest).contains("private source"))
  }

  private fun project() =
      ProjectAnalysis(
          "project",
          "revision",
          "fixture",
          "/tmp/fixture",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 2,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun projectState() =
      DesktopState(
          projectState = ProjectWorkspaceState(project(), ProjectIndex("project", "revision")))

  private fun file(path: String, hash: String) =
      ProjectFileInfo(
          path,
          hash,
          path,
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)

  private fun session(path: String, hash: String) =
      ChatSession(
          id = "session",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = hash,
          openPath = path)
}
