# Mini-Orca improvement plan

## Goal

Keep Mini-Orca small while adding a native Kotlin Compose Desktop client, project import analysis, useful project/file metadata, and atomic code generation that is constrained to one symbol in one file while still receiving project-wide context.

## Architecture

- Keep the Go daemon as the single backend and LLM integration point.
- Add a small, thread-safe active-project store shared by the API, file tree, and code-generation handler.
- On import, scan the project locally, ask the configured LLM for an architectural summary, and write `.mini-orca/analysis.md` inside the imported project.
- Build code-generation context from the analysis, a complete file inventory, build metadata, and bounded source snippets. Always prioritize the selected file.
- Add a Compose Desktop client under `desktop/` that calls the daemon over HTTP and preserves the existing charcoal, blue-accent, three-pane IDE style.

## Delivery steps

1. **Correctness and security fixes**
   - Fix the human-gate data race.
   - Replace unsafe path-prefix checks with canonical project-boundary validation.
   - Require exact safe-shell executable matches.
   - Fix Kotlin/Gradle and `pyproject.toml` project detection.
   - Fix chat-history role persistence and guard file reads against binary/oversized files.
   - Add regression tests for each fix.

2. **Project analysis backend**
   - Add `POST /api/projects/import` and `GET /api/projects/current`.
   - Collect project type, language/file/line counts, build file, and complete file inventory.
   - Generate and persist `.mini-orca/analysis.md` with an AI architectural summary.
   - Add `GET /api/projects/current/files/info?path=...` for selected-file metadata.

3. **Atomic code generation**
   - Require `file_path` and `target_symbol` in code-generation requests.
   - Supply project-wide context without allowing generated output to target another file.
   - Tell the model to change only the named function or class and return one complete target file.

4. **Compose Desktop client**
   - Add a minimal Gradle/Compose Desktop application.
   - Provide project import, project summary, file explorer/viewer, selected-file details, and a symbol-scoped generation panel.
   - Keep state and networking simple: one window, one API client, no database or extra service layer.

5. **Verification and documentation**
   - Run Go unit, race, vet, and formatting checks.
   - Build and test the desktop module.
   - Update the README/API documentation with the new workflow and launch commands.

## Acceptance criteria

- Importing a valid directory creates `.mini-orca/analysis.md` and makes it the active project.
- Project and selected-file information are visible in the desktop UI.
- Code generation is rejected unless both a file and function/class name are selected.
- The generation prompt includes global project context and explicitly permits changes to only one target file/symbol.
- Attempts to read outside the active project are rejected.
- Existing Go tests plus new regression tests pass under the race detector.
- The Compose Desktop app builds and retains the visual style of the original web IDE.

## Implementation status

Completed on 2026-08-17. The Go race suite, `go vet`, and the Compose Desktop Gradle build all pass. The implementation also fixes the executor working-directory and generated-target traversal defects found during the final audit.

The executable task breakdown is maintained in [`tasks/INDEX.md`](tasks/INDEX.md).

Reusable execution prompts are available for [one task](tasks/PROMPT_EXECUTE_TASK.md) and [all tasks in order](tasks/PROMPT_EXECUTE_ALL_TASKS.md).
