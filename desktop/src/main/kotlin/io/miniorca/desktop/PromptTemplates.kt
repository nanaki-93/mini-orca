package io.miniorca.desktop

data class PromptTemplate(
    val id: String,
    val label: String,
    val action: String,
    val defaultRequest: String,
    val scopeMode: String = "strict_symbol",
    val readOnly: Boolean = false,
    val testFileOnly: Boolean = false,
)

val promptTemplates = listOf(
    PromptTemplate("explain-file", "Explain file", "analyze_file", "Explain this selected file in plain language.", readOnly = true),
    PromptTemplate("explain-symbol", "Explain symbol", "explain_symbol", "Explain this selected symbol in plain language.", readOnly = true),
    PromptTemplate("fix-bug", "Fix bug", "fix", "Describe the bug and its expected behavior."),
    PromptTemplate("refactor", "Refactor", "refactor", "Refactor this symbol while preserving behavior."),
    PromptTemplate("add-validation", "Add validation", "fix", "Add focused input validation and explicit errors."),
    PromptTemplate("add-documentation", "Add documentation", "document", "Add concise documentation for this symbol."),
    PromptTemplate("generate-test", "Generate test", "generate_test", "Generate a focused test for this selected test symbol.", testFileOnly = true),
)

fun templateFor(id: String): PromptTemplate = promptTemplates.firstOrNull { it.id == id } ?: PromptTemplate("custom", "Custom", "fix", "")

fun templateAllowed(template: PromptTemplate, file: ProjectFileInfo?): Boolean = !template.testFileOnly || file?.path?.endsWith("_test.go") == true
