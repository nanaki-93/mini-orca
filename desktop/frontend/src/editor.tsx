import { useWorkspace } from './workspace-context';
import { useEffect, useMemo, useRef, useState } from 'react';
import {
  activeChangeWorkflow,
  changeChecksPassed,
  workflowSetupModels,
  worktreeDefaultTarget,
  type State,
} from './workspace';
import {
  ChangeHistory,
  ChangeOutcome,
  ProposalDiff,
  isAddedLine,
  isRemovedLine,
  receiptNeedsAttention,
} from './change-shared';
import { native } from './api';
import type * as M from './models';
import {
  Badge,
  Button,
  BulletContent,
  Disclosure,
  Empty,
  Go,
  Heading,
  Icon,
  InsightCard,
  KeyValues,
  Notice,
  Panel,
  Prose,
  PriorityIcon,
} from './ui';

interface DiffHunk {
  index: number;
  oldStart: number;
  oldEnd: number;
  newStart: number;
  newEnd: number;
  lines: { kind: string; old_line: number; new_line: number; text: string }[];
}

interface SplitDiffRow {
  oldLine?: number;
  oldText?: string;
  oldKind?: 'del' | 'ctx';
  newLine?: number;
  newText?: string;
  newKind?: 'add' | 'ctx';
}

function buildSplitDiffRows(
  lines: { kind: string; old_line: number; new_line: number; text: string }[],
): SplitDiffRow[] {
  const rows: SplitDiffRow[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (!isAddedLine(line.kind) && !isRemovedLine(line.kind)) {
      rows.push({
        oldLine: line.old_line || undefined,
        oldText: line.text,
        oldKind: 'ctx',
        newLine: line.new_line || undefined,
        newText: line.text,
        newKind: 'ctx',
      });
      i++;
      continue;
    }
    const dels: typeof lines = [];
    const adds: typeof lines = [];
    while (i < lines.length && isRemovedLine(lines[i].kind)) {
      dels.push(lines[i]);
      i++;
    }
    while (i < lines.length && isAddedLine(lines[i].kind)) {
      adds.push(lines[i]);
      i++;
    }
    const maxLen = Math.max(dels.length, adds.length);
    for (let j = 0; j < maxLen; j++) {
      const d = dels[j];
      const a = adds[j];
      rows.push({
        oldLine: d ? d.old_line || undefined : undefined,
        oldText: d ? d.text : undefined,
        oldKind: d ? 'del' : undefined,
        newLine: a ? a.new_line || undefined : undefined,
        newText: a ? a.text : undefined,
        newKind: a ? 'add' : undefined,
      });
    }
  }
  return rows;
}

function splitDiffIntoHunks(
  lines: { kind: string; old_line: number; new_line: number; text: string }[] = [],
): DiffHunk[] {
  const changedIndices: number[] = [];
  lines.forEach((line, idx) => {
    if (isAddedLine(line.kind) || isRemovedLine(line.kind)) changedIndices.push(idx);
  });
  if (!changedIndices.length) return [];
  const ranges: [number, number][] = [];
  for (const idx of changedIndices) {
    const start = Math.max(0, idx - 2);
    const end = Math.min(lines.length - 1, idx + 2);
    if (!ranges.length || start > ranges[ranges.length - 1][1] + 1) {
      ranges.push([start, end]);
    } else {
      ranges[ranges.length - 1][1] = Math.max(ranges[ranges.length - 1][1], end);
    }
  }
  return ranges.map(([start, end], i) => {
    const slice = lines.slice(start, end + 1);
    const oldLines = slice.filter((l) => l.old_line > 0).map((l) => l.old_line);
    const newLines = slice.filter((l) => l.new_line > 0).map((l) => l.new_line);
    return {
      index: i,
      oldStart: oldLines[0] || 1,
      oldEnd: oldLines.at(-1) || oldLines[0] || 1,
      newStart: newLines[0] || 1,
      newEnd: newLines.at(-1) || newLines[0] || 1,
      lines: slice,
    };
  });
}

function rejectHunkFromDiff(
  allLines: { kind: string; old_line: number; new_line: number; text: string }[],
  hunk: DiffHunk,
): string {
  const hunkSet = new Set(hunk.lines);
  const kept: string[] = [];
  for (const line of allLines) {
    if (hunkSet.has(line)) {
      if (!isAddedLine(line.kind)) kept.push(line.text);
    } else {
      if (!isRemovedLine(line.kind)) kept.push(line.text);
    }
  }
  return kept.join('\n');
}

function FileTree({
  paths,
  selected,
  busy,
  diffStats,
  onSelect,
  depth = 0,
}: {
  paths: string[];
  selected?: string;
  busy: boolean;
  diffStats: Map<string, { adds: number; dels: number }>;
  onSelect: (path: string) => void;
  depth?: number;
}) {
  const folders = new Map<string, string[]>();
  const leaves: string[] = [];
  for (const path of paths) {
    const parts = path.split('/');
    if (parts.length > depth + 1) {
      const name = parts[depth];
      folders.set(name, [...(folders.get(name) || []), path]);
    } else leaves.push(path);
  }
  return (
    <>
      {[...folders]
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([name, children]) => (
          <details className="explorer-folder" key={name} open={depth < 2}>
            <summary>
              <Icon name="folder" />
              {name}
            </summary>
            <FileTree
              paths={children}
              selected={selected}
              busy={busy}
              diffStats={diffStats}
              onSelect={onSelect}
              depth={depth + 1}
            />
          </details>
        ))}
      {leaves.map((path) => {
        const stat = diffStats.get(path);
        return (
          <button
            key={path}
            title={path}
            disabled={busy}
            className={`explorer-file-btn file-item ${selected === path ? 'active' : ''}`}
            onClick={() => onSelect(path)}
          >
            <Icon name="file" />
            <span>{path.split('/').at(-1)}</span>
            {stat && (
              <span className="explorer-diff-stat">
                <span style={{ color: 'var(--green)' }}>+{stat.adds}</span>
                <span style={{ color: 'var(--red)' }}>-{stat.dels}</span>
              </span>
            )}
          </button>
        );
      })}
    </>
  );
}

export function Editor({ s }: { s: State }) {
  const w = useWorkspace();
  const [filter, setFilter] = useState('');
  const [limit, setLimit] = useState(120);
  const [explorerCollapsed, setExplorerCollapsed] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [useWorktree, setUseWorktree] = useState(false);
  const [prompt, setPrompt] = useState(s.changeSeed?.message || '');
  const [acceptedHunks, setAcceptedHunks] = useState<Record<string, number[]>>({});
  const [diffLayout, setDiffLayout] = useState<'unified' | 'split'>('unified');
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const [customTargets, setCustomTargets] = useState<string[]>([]);
  const [contextPickerOpen, setContextPickerOpen] = useState(false);
  const gutterRef = useRef<HTMLDivElement | null>(null);

  const change = s.change?.state === 'draft' ? s.change : undefined;
  const changedPaths = useMemo(() => (change?.changes || []).map((c) => c.path), [change?.changes]);
  const [selectedPath, setSelectedPath] = useState<string>(
    () =>
      (s.file?.path && changedPaths.includes(s.file.path) ? s.file.path : changedPaths[0]) ||
      s.file?.path ||
      s.changeSeed?.paths[0] ||
      '',
  );
  useEffect(() => {
    if (s.project && !s.changeHistory) {
      void w.loadChangeHistory();
    }
  }, [s.project?.project_id]);
  useEffect(() => {
    if (s.file?.path) setSelectedPath(s.file.path);
  }, [s.file?.path]);
  useEffect(() => {
    if (changedPaths.length > 0 && (!selectedPath || !changedPaths.includes(selectedPath))) {
      setSelectedPath(changedPaths[0]);
      setMode('diff');
    }
  }, [change?.id, change?.revision, changedPaths]);

  const activePath =
    selectedPath || s.file?.path || changedPaths[0] || s.changeSeed?.paths[0] || '';
  const exactEdit = useMemo(
    () => (change?.changes || []).find((c) => c.path === activePath),
    [change?.changes, activePath],
  );

  const [mode, setMode] = useState<'diff' | 'edit'>(() =>
    exactEdit || changedPaths.length > 0 || s.page === 'changes' ? 'diff' : 'edit',
  );
  const activeEdit = mode === 'diff' ? exactEdit || change?.changes?.[0] : exactEdit;
  useEffect(() => {
    if (exactEdit) setMode('diff');
  }, [exactEdit?.path, change?.revision]);

  const [opened, setOpened] = useState<string[]>(() =>
    [...new Set([activePath, ...changedPaths])].filter(Boolean),
  );
  useEffect(() => {
    setOpened([...new Set([activePath, ...changedPaths])].filter(Boolean));
  }, [s.project?.project_id]);
  useEffect(() => {
    setOpened((prev) => {
      const next = [...prev];
      for (const p of [activePath, ...changedPaths]) {
        if (p && !next.includes(p)) next.push(p);
      }
      return next;
    });
  }, [activePath, changedPaths]);

  const baseContent = exactEdit ? exactEdit.content : s.file?.content || '';
  const [draftContent, setDraftContent] = useState(baseContent);
  useEffect(() => {
    setDraftContent(baseContent);
  }, [baseContent, activePath]);

  const diffStats = useMemo(() => {
    const map = new Map<string, { adds: number; dels: number }>();
    for (const edit of change?.changes || []) {
      const adds = edit.diff?.lines?.filter((l) => isAddedLine(l.kind)).length || 0;
      const dels = edit.diff?.lines?.filter((l) => isRemovedLine(l.kind)).length || 0;
      map.set(edit.path, { adds, dels });
    }
    return map;
  }, [change?.changes]);

  const allPaths = useMemo(() => {
    const indexed = (s.index?.files || []).map((f) => f.path);
    return [...new Set([...changedPaths, ...indexed])];
  }, [s.index?.files, changedPaths]);

  const filteredPaths = useMemo(
    () => allPaths.filter((p) => p.toLowerCase().includes(filter.toLowerCase())),
    [allPaths, filter],
  );

  const selectPath = (path: string) => {
    setSelectedPath(path);
    const existsInIndex = s.index?.files.some((f) => f.path === path);
    if (existsInIndex || !change?.changes.some((c) => c.path === path)) {
      void w.openFile(path);
    } else {
      w.set({ page: 'editor' });
    }
  };

  const hunks = useMemo(() => splitDiffIntoHunks(activeEdit?.diff?.lines || []), [activeEdit]);
  const editPath = activeEdit?.path || activePath;
  const fileAcceptedHunks = acceptedHunks[editPath] || [];
  const dirty = !s.file?.binary && draftContent !== (s.file?.content ?? '');
  const draftLineCount = useMemo(
    () => Math.max(1, draftContent.split('\n').length),
    [draftContent],
  );

  const composerTargets = useMemo(() => {
    const base =
      change?.targets.map((t) => t.path) ||
      (s.changeSeed?.paths.length ? s.changeSeed.paths : activePath ? [activePath] : []);
    return [...new Set([...base, ...(activePath ? [activePath] : []), ...customTargets])].slice(
      0,
      8,
    );
  }, [change?.targets, s.changeSeed?.paths, activePath, customTargets]);

  useEffect(() => {
    void native().SetUnsavedDraft(Boolean(mode === 'edit' && dirty));
  }, [mode, dirty]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
        if (mode === 'edit' && activePath && !s.busy) {
          event.preventDefault();
          void w.saveDirectFile(activePath, draftContent);
        }
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [mode, activePath, draftContent, s.busy, w]);

  const submitPrompt = async () => {
    const trimmed = prompt.trim();
    if (!trimmed || s.busy) return;
    const targetPaths = composerTargets;
    if (!targetPaths.length) return;
    const seed: M.ChangeSeed = {
      title: change?.title || s.changeSeed?.title || `Update ${targetPaths[0].split('/').at(-1)}`,
      kind: change?.kind || s.changeSeed?.kind || 'feature',
      paths: targetPaths,
      message: trimmed,
      acceptance_criteria: change?.acceptance_criteria || s.changeSeed?.acceptance_criteria || [],
      finding: s.changeSeed?.finding,
    };
    setPrompt('');
    setContextPickerOpen(false);
    if (useWorktree && s.repository?.available && !w.worktreePath) {
      const prep = await w.readFixPreparation();
      const target = worktreeDefaultTarget(
        seed.title,
        prep.repository.root || s.project?.path || '.',
      );
      await w.prepareFixInWorktree(
        seed,
        workflowSetupModels(s),
        prep,
        target.branch,
        target.path,
        true,
      );
    } else {
      await w.prepareChange(seed, true, workflowSetupModels(s).create);
      w.set({ page: 'editor' });
    }
  };

  return (
    <div className="workspace-page source-workspace">
      <div className="unified-ide-shell editor-workspace">
        <aside
          className={`unified-ide-explorer file-browser${explorerCollapsed ? ' collapsed' : ''}`}
          aria-label="Project files"
        >
          <div className="unified-ide-explorer-head">
            <div className="row between">
              <strong>Explorer</strong>
              <Go page="search" icon="search" tone="ghost small">
                Find
              </Go>
            </div>
            <div className="browser-search">
              <input
                aria-label="Filter source files"
                placeholder="Filter files…"
                value={filter}
                onChange={(e) => {
                  setFilter(e.target.value);
                  setLimit(120);
                }}
              />
            </div>
          </div>
          <div className="unified-ide-explorer-list file-list">
            {filter ? (
              filteredPaths.slice(0, limit).map((path) => {
                const stat = diffStats.get(path);
                return (
                  <button
                    key={path}
                    title={path}
                    className={`explorer-file-btn file-item ${activePath === path ? 'active' : ''}`}
                    disabled={!!s.busy}
                    onClick={() => selectPath(path)}
                  >
                    <Icon name="file" />
                    <span>{path}</span>
                    {stat && (
                      <span className="explorer-diff-stat">
                        <span style={{ color: 'var(--green)' }}>+{stat.adds}</span>
                        <span style={{ color: 'var(--red)' }}>-{stat.dels}</span>
                      </span>
                    )}
                  </button>
                );
              })
            ) : (
              <FileTree
                paths={filteredPaths.slice(0, limit)}
                selected={activePath}
                busy={!!s.busy}
                diffStats={diffStats}
                onSelect={selectPath}
              />
            )}
            {!filteredPaths.length && <p className="small muted">No matching files.</p>}
            {filteredPaths.length > limit && (
              <Button tone="ghost small" onClick={() => setLimit(limit + 120)}>
                Show more
              </Button>
            )}
          </div>
        </aside>

        <div className="unified-ide-center editor-content">
          <div className="unified-ide-toolbar">
            <div className="unified-ide-tabs" aria-label="Open files">
              <Button
                tone="ghost small"
                icon="sidebar"
                aria-label="Toggle file explorer"
                title="Toggle file explorer"
                onClick={() => setExplorerCollapsed((c) => !c)}
              />
              {opened.map((path) => {
                const stat = diffStats.get(path);
                return (
                  <button
                    key={path}
                    className={`file-tab-pill ${activePath === path ? 'active' : ''}`}
                    aria-pressed={activePath === path}
                    disabled={!!s.busy}
                    onClick={() => selectPath(path)}
                  >
                    <Icon name="file" />
                    <span>{path.split('/').at(-1)}</span>
                    {stat && (
                      <span style={{ color: 'var(--accent)', fontSize: '0.72rem' }}>
                        +{stat.adds}/-{stat.dels}
                      </span>
                    )}
                    {opened.length > 1 && (
                      <span
                        role="button"
                        aria-label={`Close ${path}`}
                        title="Close tab"
                        style={{ marginLeft: '4px', opacity: 0.6, cursor: 'pointer' }}
                        onClick={(e) => {
                          e.stopPropagation();
                          const next = opened.filter((p) => p !== path);
                          setOpened(next);
                          if (activePath === path && next[0]) selectPath(next[0]);
                        }}
                      >
                        ×
                      </span>
                    )}
                  </button>
                );
              })}
            </div>

            <div className="row" style={{ gap: '8px' }}>
              {s.changeReceipt?.undo_available && (
                <Button
                  tone="ghost small"
                  icon="undo"
                  disabled={!!s.busy}
                  onClick={() => void w.undoChange()}
                >
                  Undo Last Change
                </Button>
              )}
              {mode === 'diff' && (
                <div className="mode-switch-group" role="group" aria-label="Diff layout">
                  <button
                    type="button"
                    aria-pressed={diffLayout === 'unified'}
                    className={`mode-switch-btn ${diffLayout === 'unified' ? 'active' : ''}`}
                    onClick={() => setDiffLayout('unified')}
                  >
                    Unified
                  </button>
                  <button
                    type="button"
                    aria-pressed={diffLayout === 'split'}
                    className={`mode-switch-btn ${diffLayout === 'split' ? 'active' : ''}`}
                    onClick={() => setDiffLayout('split')}
                  >
                    Split
                  </button>
                </div>
              )}
              <div className="mode-switch-group" role="tablist" aria-label="Editor mode">
                <button
                  role="tab"
                  aria-selected={mode === 'diff'}
                  className={`mode-switch-btn ${mode === 'diff' ? 'active' : ''}`}
                  onClick={() => {
                    if (changedPaths.length > 0 && !changedPaths.includes(activePath)) {
                      setSelectedPath(changedPaths[0]);
                    }
                    setMode('diff');
                  }}
                >
                  Diff Review {hunks.length > 0 ? `(${hunks.length})` : ''}
                </button>
                <button
                  role="tab"
                  aria-selected={mode === 'edit'}
                  className={`mode-switch-btn ${mode === 'edit' ? 'active' : ''}`}
                  onClick={() => setMode('edit')}
                >
                  Direct Edit
                </button>
              </div>
              <Button
                tone={inspectorOpen ? 'primary small' : 'ghost small'}
                icon="compass"
                aria-label="Toggle symbol inspector"
                aria-pressed={inspectorOpen}
                onClick={() => setInspectorOpen((v) => !v)}
              >
                Inspector {s.symbols.length > 0 ? `(${s.symbols.length})` : ''}
              </Button>
              {mode === 'edit' && activePath && !s.file?.binary && (
                <Button
                  tone="primary small"
                  icon="check"
                  disabled={!!s.busy || !dirty}
                  onClick={() => void w.saveDirectFile(activePath, draftContent)}
                >
                  Save File (⌘S)
                </Button>
              )}
            </div>
          </div>

          <div className="unified-ide-workarea">
            <div className="unified-ide-body">
              {s.fileStale && s.file && (
                <Notice tone="warning">
                  File evidence is outdated.{' '}
                  <Button tone="ghost small" disabled={!!s.busy} onClick={() => void w.reindex()}>
                    Refresh project
                  </Button>
                </Notice>
              )}
              {(s.uncertain || receiptNeedsAttention(s.changeReceipt)) && <ChangeOutcome s={s} />}

              {mode === 'diff' ? (
                change && change.changes.length > 0 ? (
                  <div>
                    <div className="proposal-action-banner">
                      <div className="row wrap" style={{ gap: '8px' }}>
                        <strong>{change.title}</strong>
                        <Badge
                          value={
                            change.state !== 'draft'
                              ? change.state
                              : changeChecksPassed(change)
                                ? 'checks passed'
                                : 'draft'
                          }
                          tone={changeChecksPassed(change) ? 'green' : 'violet'}
                        />
                        <span className="small muted font-mono">
                          {change.changes.length} {change.changes.length === 1 ? 'file' : 'files'}
                        </span>
                      </div>
                      <div className="row wrap" style={{ gap: '8px' }}>
                        {change.workflow && (
                          <Button
                            tone="ghost small"
                            icon="sparkles"
                            onClick={() => void w.navigate('agents')}
                          >
                            Workflow Stages
                          </Button>
                        )}
                        {change.state === 'draft' && (
                          <>
                            {change.checks.some((c) => c.state === 'failed') && (
                              <Button
                                tone="ghost small"
                                icon="sparkles"
                                disabled={!!s.busy}
                                onClick={() => void w.repairChange()}
                              >
                                Repair Failed Checks
                              </Button>
                            )}
                            <Button
                              tone="ghost small"
                              icon="close"
                              disabled={!!s.busy || activeChangeWorkflow(change)}
                              onClick={() => w.newChange()}
                            >
                              Discard
                            </Button>
                            <Button
                              tone="primary small"
                              icon="check"
                              disabled={!!s.busy || activeChangeWorkflow(change)}
                              onClick={() => void w.applyProposalNow(change.title)}
                            >
                              Accept &amp; Commit
                            </Button>
                          </>
                        )}
                      </div>
                    </div>

                    {activeEdit ? (
                      hunks.length > 0 ? (
                        hunks.map((hunk) => {
                          const isAccepted = fileAcceptedHunks.includes(hunk.index);
                          return (
                            <div
                              key={hunk.index}
                              className={`hunk-card ${isAccepted ? 'accepted' : ''}`}
                            >
                              <div className="hunk-card-head">
                                <span>
                                  {activeEdit.path} · Hunk #{hunk.index + 1} (Lines {hunk.newStart}–
                                  {hunk.newEnd})
                                </span>
                                <div className="row" style={{ gap: '6px' }}>
                                  <Button
                                    tone={isAccepted ? 'primary small' : 'ghost small'}
                                    icon="check"
                                    disabled={!!s.busy || change.state !== 'draft'}
                                    onClick={() =>
                                      setAcceptedHunks((prev) => ({
                                        ...prev,
                                        [editPath]: isAccepted
                                          ? (prev[editPath] || []).filter((i) => i !== hunk.index)
                                          : [...(prev[editPath] || []), hunk.index],
                                      }))
                                    }
                                  >
                                    {isAccepted ? 'Accepted' : 'Accept Hunk'}
                                  </Button>
                                  <Button
                                    tone="ghost small"
                                    icon="close"
                                    disabled={!!s.busy || change.state !== 'draft'}
                                    onClick={() => {
                                      const nextContent = rejectHunkFromDiff(
                                        activeEdit.diff.lines,
                                        hunk,
                                      );
                                      void w.updateProposalFileContent(
                                        activeEdit.path,
                                        nextContent,
                                        `Rejected hunk #${hunk.index + 1} in ${activeEdit.path}.`,
                                      );
                                    }}
                                  >
                                    Reject Hunk
                                  </Button>
                                  <Button
                                    tone="ghost small"
                                    icon="code"
                                    onClick={() => {
                                      setDraftContent(activeEdit.content);
                                      setMode('edit');
                                    }}
                                  >
                                    Edit Inline
                                  </Button>
                                </div>
                              </div>
                              {diffLayout === 'split' ? (
                                <table className="hunk-code-table hunk-split-table">
                                  <tbody>
                                    {buildSplitDiffRows(hunk.lines).map((row, rowIdx) => (
                                      <tr key={rowIdx}>
                                        <td className={`ln ${row.oldKind === 'del' ? 'del' : ''}`}>
                                          {row.oldLine ?? ''}
                                        </td>
                                        <td
                                          className={`split-cell ${row.oldKind === 'del' ? 'del' : ''}`}
                                        >
                                          {row.oldText !== undefined
                                            ? `${row.oldKind === 'del' ? '- ' : '  '}${row.oldText}`
                                            : ''}
                                        </td>
                                        <td className={`ln ${row.newKind === 'add' ? 'add' : ''}`}>
                                          {row.newLine ?? ''}
                                        </td>
                                        <td
                                          className={`split-cell ${row.newKind === 'add' ? 'add' : ''}`}
                                        >
                                          {row.newText !== undefined
                                            ? `${row.newKind === 'add' ? '+ ' : '  '}${row.newText}`
                                            : ''}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              ) : (
                                <table className="hunk-code-table">
                                  <tbody>
                                    {hunk.lines.map((line, lineIdx) => {
                                      const isAdd = isAddedLine(line.kind);
                                      const isDel = isRemovedLine(line.kind);
                                      const cls = isAdd ? 'add' : isDel ? 'del' : '';
                                      const prefix = isAdd ? '+' : isDel ? '-' : ' ';
                                      return (
                                        <tr key={lineIdx} className={cls}>
                                          <td className="ln">{isAdd ? '' : line.old_line || ''}</td>
                                          <td className="ln">{isDel ? '' : line.new_line || ''}</td>
                                          <td>
                                            {prefix} {line.text}
                                          </td>
                                        </tr>
                                      );
                                    })}
                                  </tbody>
                                </table>
                              )}
                            </div>
                          );
                        })
                      ) : (
                        <ProposalDiff
                          edit={activeEdit}
                          index={Math.max(0, changedPaths.indexOf(activeEdit.path))}
                          total={change.changes.length}
                        />
                      )
                    ) : (
                      <Empty
                        title="Select a changed file to inspect hunks"
                        detail="Click one of the changed file tabs above to review its diff hunks."
                        icon="code"
                      />
                    )}
                  </div>
                ) : (
                  <div className="stack" style={{ gap: '12px' }}>
                    <Empty
                      title="No active diff for this file"
                      detail="Switch to Direct Edit to modify the file directly, or describe a change in the AI prompt bar below to generate a hunk-by-hunk diff."
                      icon="sparkles"
                    >
                      <div className="actions">
                        {activePath && (
                          <Button tone="primary" icon="code" onClick={() => setMode('edit')}>
                            Switch to Direct Edit
                          </Button>
                        )}
                        <Go page="bugs" tone="ghost" icon="bug">
                          Fix an Issue
                        </Go>
                      </div>
                    </Empty>
                    {(s.changeHistory || []).length > 0 && (
                      <Panel title="Saved Proposals & Fixes">
                        <div className="stack" style={{ gap: '6px' }}>
                          {(s.changeHistory || []).slice(0, 6).map((entry) => (
                            <div className="list-row" key={entry.id}>
                              <span className="list-copy">
                                <strong>{entry.title}</strong>
                                <small className="muted">
                                  {entry.workflow_status || entry.state} · revision {entry.revision}
                                </small>
                              </span>
                              <Button
                                tone="primary small"
                                disabled={!!s.busy}
                                onClick={() => void w.resumeChange(entry.id)}
                              >
                                Open Diff
                              </Button>
                            </div>
                          ))}
                        </div>
                      </Panel>
                    )}
                  </div>
                )
              ) : activePath && s.file ? (
                <div className="source-document">
                  <div className="row between wrap" style={{ marginBottom: '8px' }}>
                    <span className="row" style={{ gap: '8px' }}>
                      <strong className="font-mono small">{s.file.path}</strong>
                      {dirty && <Badge value="Unsaved edits" tone="amber" />}
                      {s.symbol && <Badge value={`Symbol: ${s.symbol}`} tone="blue" />}
                    </span>
                    <div className="row" style={{ gap: '8px' }}>
                      <span className="small muted">
                        {draftLineCount} lines · {s.file.language || 'text'}
                      </span>
                      {dirty && (
                        <Button
                          tone="ghost small"
                          onClick={() => setDraftContent(s.file?.content || '')}
                        >
                          Reset
                        </Button>
                      )}
                    </div>
                  </div>
                  {s.file.binary ? (
                    <Empty title="Binary file" />
                  ) : (
                    <div className="code-editor-with-gutter">
                      <div className="code-gutter" ref={gutterRef} aria-hidden="true">
                        {Array.from({ length: draftLineCount }, (_, idx) => (
                          <div key={idx + 1} className="code-gutter-ln">
                            {idx + 1}
                          </div>
                        ))}
                      </div>
                      <textarea
                        aria-label="Direct code editor"
                        className="direct-code-editor"
                        value={draftContent}
                        onChange={(e) => setDraftContent(e.target.value)}
                        onScroll={(e) => {
                          if (gutterRef.current) {
                            gutterRef.current.scrollTop = e.currentTarget.scrollTop;
                          }
                        }}
                        spellCheck={false}
                      />
                    </div>
                  )}
                  <FileInspectionDrawer s={s} />
                </div>
              ) : (
                <Empty title="Choose a file from the Explorer" icon="code" />
              )}
            </div>

            {inspectorOpen && (
              <aside
                className="unified-ide-inspector"
                aria-label="Symbol and blast radius inspector"
              >
                <div className="row between" style={{ marginBottom: '10px' }}>
                  <strong>Symbols & Blast Radius</strong>
                  <Button tone="ghost small" icon="close" onClick={() => setInspectorOpen(false)} />
                </div>
                <DeclarationPicker s={s} />
                <div className="actions" style={{ marginTop: '8px', marginBottom: '10px' }}>
                  <Button
                    tone="ghost small"
                    disabled={!!s.busy || !s.symbol || s.fileStale}
                    onClick={() => void w.explain()}
                  >
                    Explain
                  </Button>
                  <Button
                    tone="ghost small"
                    disabled={!!s.busy || s.fileStale}
                    onClick={() => void w.analyzeFile()}
                  >
                    Analyze File
                  </Button>
                  <Button
                    tone="ghost small"
                    disabled={!!s.busy || s.fileStale}
                    onClick={() => void w.securityReview(false)}
                  >
                    Security Scan
                  </Button>
                </div>
                {s.impact && s.impact.target_symbol === s.symbol ? (
                  <div className="stack" style={{ gap: '6px' }}>
                    <small className="uppercase muted">
                      Blast Radius · {s.impact.target_symbol} ({s.impact.references.length})
                    </small>
                    {s.impact.references.length === 0 ? (
                      <p className="small muted">No external references indexed.</p>
                    ) : (
                      s.impact.references.map((ref, idx) => (
                        <div
                          key={`${ref.path}:${ref.symbol}:${idx}`}
                          className="row between wrap"
                          style={{
                            padding: '6px 8px',
                            borderRadius: '6px',
                            border: '1px solid var(--line)',
                            background: 'var(--canvas)',
                          }}
                        >
                          <div className="stack" style={{ gap: '2px' }}>
                            <strong className="font-mono small">{ref.path}</strong>
                            <span className="small muted">
                              {ref.symbol ? `${ref.symbol} · ` : ''}
                              {ref.reason}
                            </span>
                          </div>
                          <Button
                            tone="ghost small"
                            icon="code"
                            disabled={!!s.busy}
                            onClick={() => void w.openFile(ref.path, ref.symbol)}
                          >
                            Open
                          </Button>
                        </div>
                      ))
                    )}
                  </div>
                ) : (
                  <p className="small muted">
                    Select a declaration above to inspect callers and blast radius across the
                    repository.
                  </p>
                )}
              </aside>
            )}
          </div>

          <div className="docked-composer">
            {historyOpen && (
              <div className="composer-history-drawer" aria-label="Conversation history">
                <div className="row between">
                  <strong className="small uppercase muted">Prompt & Proposal History</strong>
                  <Button tone="ghost small" icon="close" onClick={() => setHistoryOpen(false)}>
                    Close
                  </Button>
                </div>
                {(change?.messages || []).map((entry, idx) => (
                  <div key={idx} className="small">
                    <strong
                      style={{ color: entry.role === 'user' ? 'var(--accent)' : 'var(--green)' }}
                    >
                      {entry.role === 'user' ? 'You: ' : 'Mini-Orca: '}
                    </strong>
                    <span>{entry.content}</span>
                  </div>
                ))}
                <ChangeHistory s={s} />
              </div>
            )}
            {contextPickerOpen && (
              <div className="composer-history-drawer" aria-label="Context file picker">
                <div className="row between">
                  <strong className="small uppercase muted">
                    Target Files in AI Context ({composerTargets.length}/8 max)
                  </strong>
                  <Button
                    tone="ghost small"
                    icon="close"
                    onClick={() => setContextPickerOpen(false)}
                  >
                    Done
                  </Button>
                </div>
                <div className="row wrap" style={{ gap: '6px' }}>
                  {allPaths.slice(0, 24).map((path) => {
                    const included = composerTargets.includes(path);
                    const isPrimary = path === activePath;
                    return (
                      <Button
                        key={path}
                        tone={included ? 'primary small' : 'ghost small'}
                        disabled={isPrimary || (!included && composerTargets.length >= 8)}
                        onClick={() =>
                          setCustomTargets((prev) =>
                            prev.includes(path) ? prev.filter((p) => p !== path) : [...prev, path],
                          )
                        }
                      >
                        @{path.split('/').at(-1)}
                      </Button>
                    );
                  })}
                </div>
              </div>
            )}
            <form
              className="docked-composer-row"
              onSubmit={(e) => {
                e.preventDefault();
                void submitPrompt();
              }}
            >
              <Button tone="ghost small" icon="clock" onClick={() => setHistoryOpen((v) => !v)}>
                History ({(change?.messages.length || 0) + (s.changeHistory?.length || 0)})
              </Button>
              <div className="row wrap" style={{ gap: '4px' }}>
                {composerTargets.map((target) => (
                  <Badge key={target} value={`@${target.split('/').at(-1)}`} tone="violet" />
                ))}
                <Button
                  tone="ghost small"
                  aria-label="Add context file"
                  title="Attach up to 8 files to the AI proposal context"
                  onClick={() => setContextPickerOpen((v) => !v)}
                >
                  + Context ({composerTargets.length}/8)
                </Button>
              </div>
              <input
                aria-label="Ask about this file"
                className="docked-composer-input"
                placeholder="Ask AI to edit, refactor, or fix bugs in this file (generates reviewable diff hunks)…"
                value={prompt}
                disabled={!!s.busy || activeChangeWorkflow(change)}
                onChange={(e) => setPrompt(e.target.value)}
              />
              {s.repository?.available && !w.worktreePath && (
                <label
                  className="row small muted"
                  style={{ gap: '5px', cursor: 'pointer', whiteSpace: 'nowrap' }}
                  title="Run in isolated Git worktree instead of in-place"
                >
                  <input
                    type="checkbox"
                    checked={useWorktree}
                    onChange={(e) => setUseWorktree(e.target.checked)}
                  />
                  Worktree
                </label>
              )}
              <Button
                tone="primary small"
                icon="sparkles"
                type="submit"
                disabled={!prompt.trim() || !!s.busy || (!activePath && !change)}
              >
                Generate Diff
              </Button>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
}

function canSelectSymbol(s: State, symbol: M.SymbolInfo): boolean {
  if (!symbol.atomic_target) return false;
  if (s.file?.language === 'Go') return symbol.confidence === 'exact';
  return symbol.confidence === 'exact' || symbol.confidence === 'approximate';
}

function DeclarationPicker({ s }: { s: State }) {
  const w = useWorkspace();
  return (
    <div className="row wrap declaration-picker">
      <label className="sr-only" htmlFor="symbol-picker">
        Declaration
      </label>
      <select
        className="field symbol-picker"
        id="symbol-picker"
        disabled={!!s.busy || s.fileStale}
        value={s.symbol}
        onChange={(e) => void w.selectSymbol(e.target.value)}
      >
        <option value="">Choose a declaration…</option>
        {s.symbols.map((symbol, i) => (
          <option
            value={symbol.name}
            key={`${symbol.name}:${i}`}
            disabled={!canSelectSymbol(s, symbol)}
          >
            {symbol.name} · {symbol.kind} (L{symbol.start_line}–{symbol.end_line})
          </option>
        ))}
      </select>
    </div>
  );
}

function FileInspectionDrawer({ s }: { s: State }) {
  const w = useWorkspace();
  return (
    <div style={{ marginTop: '14px' }}>
      <Disclosure
        title={`Symbols, Blast Radius & File Inspection${s.symbols.length ? ` (${s.symbols.length} symbols)` : ''}`}
      >
        <DeclarationPicker s={s} />
        {s.symbols.length > 0 && (
          <div
            className="row wrap symbol-pills"
            aria-label="File symbols"
            style={{ gap: '6px', marginTop: '8px' }}
          >
            {s.symbols.map((symbol, i) => {
              const selectable = canSelectSymbol(s, symbol);
              const active = s.symbol === symbol.name;
              return (
                <Button
                  key={`${symbol.name}:${i}`}
                  tone={active ? 'primary small' : 'ghost small'}
                  disabled={!!s.busy || s.fileStale || !selectable}
                  onClick={() => void w.selectSymbol(active ? '' : symbol.name)}
                >
                  <span>{symbol.name}</span>
                  <small className="muted" style={{ marginLeft: '4px' }}>
                    L{symbol.start_line}
                  </small>
                </Button>
              );
            })}
          </div>
        )}
        <div className="actions" style={{ marginTop: '8px' }}>
          <Button disabled={!!s.busy || !s.symbol || s.fileStale} onClick={() => void w.explain()}>
            Explain declaration
          </Button>
          <Button disabled={!!s.busy || s.fileStale} onClick={() => void w.analyzeFile()}>
            Analyze file
          </Button>
          <Button disabled={!!s.busy || s.fileStale} onClick={() => void w.securityReview(false)}>
            Scan source
          </Button>
          <Button disabled={!!s.busy || s.fileStale} onClick={() => void w.securityReview(true)}>
            AI Security review
          </Button>
        </div>
        {s.impact && s.impact.target_symbol === s.symbol && (
          <Panel
            title={`Blast Radius · ${s.impact.target_symbol} (${s.impact.references.length} ${s.impact.references.length === 1 ? 'reference' : 'references'})`}
            className="symbol-impact-panel"
          >
            {s.impact.references.length === 0 ? (
              <p className="small muted">
                No external callers or references indexed for this symbol.
              </p>
            ) : (
              <div className="stack" style={{ gap: '6px' }}>
                {s.impact.references.map((ref, idx) => (
                  <div
                    key={`${ref.path}:${ref.symbol}:${idx}`}
                    className="row between wrap"
                    style={{
                      padding: '6px 8px',
                      borderRadius: '4px',
                      border: '1px solid var(--line)',
                    }}
                  >
                    <div className="row wrap" style={{ gap: '8px' }}>
                      <strong className="font-mono small">{ref.path}</strong>
                      {ref.symbol && <Badge value={ref.symbol} tone="violet" />}
                      <span className="small muted">{ref.reason}</span>
                    </div>
                    <Button
                      tone="ghost small"
                      icon="code"
                      disabled={!!s.busy}
                      onClick={() => void w.openFile(ref.path, ref.symbol)}
                    >
                      Open
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </Panel>
        )}
        {s.explanation && (
          <Panel title={`About ${s.explanation.anchor.symbol}`}>
            <Prose text={s.explanation.summary} />
            <BulletContent title="Behavior" items={s.explanation.behavior} />
            <InsightCard insight={s.explanation.engineering_insight} />
          </Panel>
        )}
        {s.fileAnalysis && (
          <Panel
            className="source-analysis"
            title="File analysis"
            actions={<Badge value={s.fileAnalysis.status} />}
          >
            <Prose text={s.fileAnalysis.purpose || s.fileAnalysis.failure} />
            <BulletContent title="Responsibilities" items={s.fileAnalysis.responsibilities} />
            {(s.fileAnalysis.risks || []).map((risk, i) => (
              <Disclosure
                key={i}
                title={
                  <span className="row wrap">
                    <PriorityIcon value={risk.severity} />
                    {risk.summary}
                  </span>
                }
              >
                <InsightCard insight={risk.engineering_insight} />
              </Disclosure>
            ))}
          </Panel>
        )}
      </Disclosure>
    </div>
  );
}

export function Context({ s }: { s: State }) {
  const w = useWorkspace();
  const context = s.context;
  if (!context)
    return (
      <Empty title={s.file ? 'Context unavailable' : 'Choose a source file'}>
        {s.file ? (
          <Button disabled={!!s.busy} onClick={() => void w.inspectContext()}>
            Refresh context
          </Button>
        ) : (
          <Go page="editor">Browse source</Go>
        )}
      </Empty>
    );
  return (
    <div className="workspace-page context-inspection">
      <Heading variant="intro" title="Provider context" detail={s.file?.path} />
      <Panel
        title="Context"
        actions={
          <Button tone="small" disabled={!!s.busy} onClick={() => void w.inspectContext()}>
            Refresh
          </Button>
        }
      >
        <div className="mini-metrics">
          <div>
            <strong>{context.included?.length ?? '—'}</strong>
            <small>Included files</small>
          </div>
          <div>
            <strong>{context.estimated_tokens?.toLocaleString() ?? '—'}</strong>
            <small>Estimated tokens</small>
          </div>
          <div>
            <strong>{context.excluded?.length ?? '—'}</strong>
            <small>Excluded</small>
          </div>
        </div>
        <KeyValues
          values={[
            ['Model', context.model],
            ['Destination', context.provider_origin],
            ['Scope', context.scope],
            ['Truncated', context.truncated === undefined ? '—' : context.truncated ? 'Yes' : 'No'],
          ]}
        />
      </Panel>
      <Panel title="Included files">
        <div className="table-wrap" tabIndex={0} aria-label="Included context files">
          <table>
            <thead>
              <tr>
                <th>Path</th>
                <th>Bytes</th>
                <th>Tokens</th>
              </tr>
            </thead>
            <tbody>
              {(context.included || []).map((file) => (
                <tr key={file.path}>
                  <td className="path">
                    {file.path}
                    {file.truncated && <Badge value="truncated" />}
                  </td>
                  <td>{file.size_bytes.toLocaleString()}</td>
                  <td>{file.estimated_tokens?.toLocaleString() ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
    </div>
  );
}
