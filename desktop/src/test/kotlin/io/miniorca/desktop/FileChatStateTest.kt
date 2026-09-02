package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class FileChatStateTest {
    @Test fun replaceRequiresAnExactSelectedFunctionOrType() {
        val exact = symbol("Run")
        val approximate = exact.copy(confidence = "approximate")

        assertTrue(validateChatTarget(file(), listOf(exact), exact, ChatEditMode.ReplaceSymbol, "").valid)
        val variable = exact.copy(name = "diffCmd", kind = "var")
        assertFalse(validateChatTarget(file(), listOf(variable), variable, ChatEditMode.ReplaceSymbol, "").valid)
        assertFalse(validateChatTarget(file(), listOf(exact.copy(atomicTarget = false)), exact.copy(atomicTarget = false), ChatEditMode.ReplaceSymbol, "").valid)
        assertFalse(validateChatTarget(file(), listOf(approximate), approximate, ChatEditMode.ReplaceSymbol, "").valid)
        assertFalse(validateChatTarget(file(), listOf(exact), null, ChatEditMode.ReplaceSymbol, "").valid)
    }

    @Test fun createRequiresAValidAbsentGoName() {
        val existing = symbol("Run")

        assertEquals("NewRun", validateChatTarget(file(), listOf(existing), null, ChatEditMode.CreateSymbol, "NewRun").target?.symbol)
        assertFalse(validateChatTarget(file(), listOf(existing), null, ChatEditMode.CreateSymbol, "Run").valid)
        assertFalse(validateChatTarget(file(), listOf(existing), null, ChatEditMode.CreateSymbol, "not valid").valid)
    }

    @Test fun directEditUsesReplaceAndMakesAnotherDraftAnExplicitDecision() {
        val run = symbol("Run")
        val other = symbol("Other")
        val currentDraft = CurrentEditIdentity(ChatEditMode.ReplaceSymbol, "main.go", "Run", hasDraft = true)

        val different = directEditRequest(file(), listOf(run, other), other, currentDraft)!!
        val same = directEditRequest(file(), listOf(run, other), run, currentDraft)!!

        assertEquals(ChatEditMode.ReplaceSymbol, different.target.mode)
        assertEquals("Other", different.target.symbol)
        assertTrue(different.requiresDraftDiscard)
        assertEquals("Discard draft for Run and edit Other?", different.discardPrompt)
        assertFalse(same.requiresDraftDiscard)
    }

    @Test fun activeSessionMustMatchTheOpenFileRevisionHashAndTarget() {
        val target = ChatTarget(ChatEditMode.ReplaceSymbol, "Run")
        val session = session()

        assertTrue(chatSessionMatches(session, file(), project(), target))
        assertFalse(chatSessionMatches(session, file("other.go", "base"), project(), target))
        assertFalse(chatSessionMatches(session, file(), project("next"), target))
        assertFalse(chatSessionMatches(session.copy(state = "stale"), file(), project(), target))
    }

    @Test fun proposalAddsOnlyBoundTurnsAndPreservesDraftLineage() {
        val controller = loadedController()
        val request = controller.beginFileLoad("main.go")!!
        assertTrue(controller.fileLoaded(request, file(), listOf(symbol("Run"))))
        val fileRequest = controller.currentFileRequest()!!
        val (chatRequest, chatFile) = controller.beginChatLoad()!!
        val session = session(messages = listOf(ChatSessionMessage("user", "First request")), latestDraftId = "draft-1")
        assertTrue(controller.chatLoaded(chatRequest, chatFile, session))
        val (proposalRequest, proposalFile) = controller.beginChatLoad()!!
        val draft = draft(parentDraftId = "draft-1")
        val proposal = ChatDraftProposal("session", draft, ChatSessionMessage("assistant", "Second proposal", draft.id))

        assertTrue(controller.chatProposalLoaded(proposalRequest, proposalFile, session, "Revise it", proposal))
        assertEquals("draft-2", controller.state.chat.session?.latestDraftId)
        assertEquals(listOf("First request", "Revise it", "Second proposal"), controller.state.chat.session?.messages?.map { it.content })
        assertTrue(chatDraftMatchesSession(controller.state.review.draft, controller.state.chat.session))
        assertEquals("draft-1", controller.state.review.draft?.parentDraftId)
        assertEquals(fileRequest, controller.currentFileRequest())
    }

    @Test fun cancellationAndRevisionChangeRejectLateChatResultsAndClearDrafts() {
        val controller = loadedController()
        val fileLoad = controller.beginFileLoad("main.go")!!
        assertTrue(controller.fileLoaded(fileLoad, file(), listOf(symbol("Run"))))
        val (chatRequest, chatFile) = controller.beginChatLoad()!!
        assertTrue(controller.cancelChatLoad(chatRequest, chatFile))
        assertFalse(controller.chatProposalLoaded(chatRequest, chatFile, session(), "Late", ChatDraftProposal("session", draft(), ChatSessionMessage("assistant", "Late"))))

        val state = DesktopState(
            projectState = ProjectWorkspaceState(project(), ProjectIndex("project", "revision")),
            chat = ChatState(session()),
            review = DraftReviewState(draft = draft()),
        ).reduce(DesktopEvent.IndexRefreshed(ProjectIndex("project", "next")))
        assertNull(state.chat.session)
        assertNull(state.review.draft)
    }

    @Test fun discardingForAnotherTargetRejectsLateChatResults() {
        val controller = loadedController()
        val request = controller.beginFileLoad("main.go")!!
        assertTrue(controller.fileLoaded(request, file(), listOf(symbol("Run"))))
        val (chatRequest, chatFile) = controller.beginChatLoad()!!

        controller.dispatch(DesktopEvent.DraftDiscarded)

        assertFalse(controller.chatProposalLoaded(chatRequest, chatFile, session(), "Late", ChatDraftProposal("session", draft(), ChatSessionMessage("assistant", "Late"))))
    }

    private fun loadedController() = DesktopWorkflowController().also { controller ->
        val request = controller.beginProjectLoad()
        assertTrue(controller.projectLoaded(request, project(), ProjectIndex("project", "revision")))
    }

    private fun project(revision: String = "revision") = ProjectAnalysis("project", revision, "project", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun file(path: String = "main.go", hash: String = "base") = ProjectFileInfo(path, hash, path, language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
    private fun symbol(name: String) = SymbolInfo(name, "function", "func $name()", 1, 3, "exact", true)
    private fun session(messages: List<ChatSessionMessage> = emptyList(), latestDraftId: String = "") = ChatSession("session", "project", "revision", "base", "main.go", "replace_symbol", "Run", "active", latestDraftId, messages)
    private fun draft(parentDraftId: String = "") = DeclarationDraft("draft-2", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}", revision = 2, hash = "draft-hash", parentDraftId = parentDraftId)
}
