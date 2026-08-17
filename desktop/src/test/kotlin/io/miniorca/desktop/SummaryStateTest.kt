package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class SummaryStateTest {
    @Test fun freshSummaryWithExactSymbolsIsReadyForSelection() {
        val state = summaryState(FileAnalysis("main.go", "fresh", purpose = "Runs work"), listOf(SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
        assertEquals("fresh", state.status)
        assertFalse(state.emptySymbols)
        assertFalse(state.approximateSymbols)
    }

    @Test fun staleAndFailedSummariesRemainDistinct() {
        assertEquals("stale", summaryState(FileAnalysis("main.go", "stale"), emptyList()).status)
        val failed = summaryState(FileAnalysis("main.go", "failed", failure = "retry"), emptyList())
        assertEquals("failed", failed.status)
        assertEquals("retry", failed.failure)
    }

    @Test fun approximateAndEmptySymbolsAreExplicit() {
        val approximate = summaryState(null, listOf(SymbolInfo("probablyRun", "function", confidence = "approximate", atomicTarget = false)))
        assertTrue(approximate.approximateSymbols)
        assertFalse(approximate.emptySymbols)
        assertTrue(summaryState(null, emptyList()).emptySymbols)
    }
}
