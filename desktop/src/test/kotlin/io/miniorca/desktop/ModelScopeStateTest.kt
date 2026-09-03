package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ModelScopeStateTest {
    @Test fun confirmationsRemainIndependentAcrossMixedDestinations() {
        val confirmations = ScopedConfirmationState()
            .withConfirmation(ModelScope.Analyze, true)
            .withConfirmation(ModelScope.Function, true)

        assertTrue(confirmations.confirmed(ModelScope.Analyze))
        assertFalse(confirmations.confirmed(ModelScope.Bug))
        assertTrue(confirmations.confirmed(ModelScope.Function))
    }

    @Test fun catalogIdentityCoversAllLocalMixedAndAllRemoteProfiles() {
        val allLocal = catalog(analyzeRemote = false, bugRemote = false, functionRemote = false)
        val mixed = catalog(analyzeRemote = true, bugRemote = true, functionRemote = false)
        val allRemote = catalog(analyzeRemote = true, bugRemote = true, functionRemote = true)

        assertTrue(allLocal.identity().none { it.remoteProvider })
        assertEquals(listOf(true, true, false), mixed.identity().map { it.remoteProvider })
        assertTrue(allRemote.identity().all { it.remoteProvider })
        assertNotEquals(mixed.identity(), allRemote.identity())
    }

    @Test fun changedCatalogRequiresFreshScopeConfirmations() {
        val before = catalog(analyzeRemote = true, bugRemote = false, functionRemote = true)
        val changed = before.copy(scopes = before.scopes + ("function" to before.forScope(ModelScope.Function).copy(model = "new-model")))
        val confirmations = ScopedConfirmationState(analyze = true, bug = true, function = true)

        assertNotEquals(before.identity(), changed.identity())
        assertEquals(ScopedConfirmationState(), if (before.identity() != changed.identity()) ScopedConfirmationState() else confirmations)
    }

    @Test fun staleServerConfirmationExplainsWhichScopeMustBeReconfirmed() {
        val stale = ApiException(400, ApiError(message = "remote provider confirmation required"), "remote provider confirmation required")

        assertTrue(staleRemoteConfirmationMessage(stale, ModelScope.Bug).orEmpty().contains("bugs"))
        assertNull(staleRemoteConfirmationMessage(ApiException(409, message = "stale revision"), ModelScope.Function))
    }

    private fun catalog(analyzeRemote: Boolean, bugRemote: Boolean, functionRemote: Boolean) = ModelCatalog(
        scopes = mapOf(
            "analyze" to ScopedModel("analyze", "analyze", "analyze-model", remoteProvider = analyzeRemote),
            "bug" to ScopedModel("bug", "bug", "bug-model", remoteProvider = bugRemote),
            "function" to ScopedModel("function", "function", "function-model", remoteProvider = functionRemote),
        ),
    )
}
