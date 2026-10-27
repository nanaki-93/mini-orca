# UI design direction

The interfaces were rebuilt from zero around the three jobs in
[UI_DESIGN_GUIDELINES.md](../UI_DESIGN_GUIDELINES.md): start a task and watch
the agents, review and apply, and understand the project. This note records the
references each surface borrows from and why, so later changes keep the same
character.

| Surface | Reference | What it borrows |
| --- | --- | --- |
| Browser shell and lists | Linear | Quiet sidebar with counts, grouped lists ("Needs you", "Agents working"), `c` to create, `g` sequences, `⌘K` for everything, keyboard-first rows. |
| Browser task page | GitHub pull requests | Conversation, Files changed and Checks tabs; a file tree with viewed marks and progress; a merge-box style gate that lists what Apply still needs. |
| Browser explorer | JetBrains IDEs | File tree, editor tabs, a gutter with line numbers and issue markers, an inspector for the selected file. |
| Browser overall feel | Vercel | Neutral surfaces, one accent, restrained borders, Inter and JetBrains Mono, dense but readable type scale, light and dark parity. |
| Terminal UI | lazygit and k9s | Fixed header and key-hint rows, numbered workspaces, bordered panes with titles and view tabs in the border, list beside detail, floating command palette and dialogs, the primary key filled in the hint bar. |

## Choices

- **Same model, two densities.** Both surfaces show the same workspaces, task
  statuses and review gate. The browser favors reading (full diffs, briefs,
  side rails); the terminal favors scanning (one row per item, compact
  stage timelines, tinted diffs) and works at 52 × 16.
- **The gate is the product.** The review gate is the one element both surfaces
  give the most prominence: it is what makes agent output safe to apply.
- **Live by default.** The browser subscribes to the daemon's event stream and
  the terminal follows running work through its progress streams, so a running task updates in place without a
  refresh key.
- **No decoration without information.** Colors carry status (accent for "needs
  you", amber for running, red for failed, green for applied) and always pair with
  a glyph or a word.
