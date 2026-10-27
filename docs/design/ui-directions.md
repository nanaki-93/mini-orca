# UI design direction

The browser workspace was rebuilt from zero around the three jobs in
[UI_DESIGN_GUIDELINES.md](../UI_DESIGN_GUIDELINES.md): start a task and watch
the agents, review and apply, and understand the project. This note records the
references each part borrows from and why, so later changes keep the same
character.

| Surface | Reference | What it borrows |
| --- | --- | --- |
| Browser shell and lists | Linear | Quiet sidebar with counts, grouped lists ("Needs you", "Agents working"), `c` to create, `g` sequences, `⌘K` for everything, keyboard-first rows. |
| Browser task page | GitHub pull requests | Conversation, Files changed and Checks tabs; a file tree with viewed marks and progress; a merge-box style gate that lists what Apply still needs. |
| Browser explorer | JetBrains IDEs | File tree, editor tabs, a gutter with line numbers and issue markers, an inspector for the selected file. |
| Browser overall feel | Vercel | Neutral surfaces, one accent, restrained borders, Inter and JetBrains Mono, dense but readable type scale, light and dark parity. |

## Choices

- **Built for reading.** Full diffs, briefs and side rails; dense lists whose
  trailing columns line up so a glance compares rows.
- **The gate is the product.** The review gate gets the most prominence: it is
  what makes agent output safe to apply.
- **Live by default.** Pages subscribe to the daemon's event stream, so a
  running task updates in place without a refresh key.
- **A document on a desk.** Pages sit in one content panel inset from quieter
  window chrome, so the work stands apart from navigation.
- **No decoration without information.** Colors carry status (accent for "needs
  you", amber for running, red for failed, green for applied) and always pair with
  a glyph or a word.
