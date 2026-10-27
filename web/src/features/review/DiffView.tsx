// A GitHub-style file diff: unified or split, collapsed unchanged regions that
// expand on click, syntax highlighting, line numbers and line comments.

import { Fragment, useMemo, useState, type ReactNode } from 'react';
import { ChevronsUpDown, Plus } from 'lucide-react';
import type { DiffLine } from '@/api/types';
import { anchorKey, buildSegments, lineAnchor, splitRows, type LineAnchor, type Segment } from '@/lib/diff';
import { escapeHtml, highlightLine } from '@/lib/highlight';
import { cx } from '@/components/ui';
import type { DiffLayout } from '@/app/store';

export interface DiffComments {
  /** Lines that carry a comment or an open comment form stay visible. */
  anchored: ReadonlySet<string>;
  /** Content shown under a line: its comments and any open form. */
  render: (anchor: LineAnchor) => ReactNode;
  /** Opens a comment form on a line; absent when comments cannot be added. */
  onAdd?: (anchor: LineAnchor) => void;
}

export function DiffView({ lines, language, layout, wrap, anchorPrefix, comments }: { lines: DiffLine[]; language: string; layout: DiffLayout; wrap: boolean; anchorPrefix: string; comments?: DiffComments }) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const pinned = useMemo(() => {
    const indexes = new Set<number>();
    lines.forEach((line, index) => {
      const anchor = lineAnchor(line);
      if (anchor && comments?.anchored.has(anchorKey(anchor))) indexes.add(index);
    });
    return indexes;
  }, [lines, comments?.anchored]);
  const segments = useMemo(() => buildSegments(lines, 3, expanded, pinned), [lines, expanded, pinned]);
  const html = useMemo(() => lines.map(line => (line.kind === 'hunk' ? escapeHtml(line.text) : highlightLine(line.text, language))), [lines, language]);
  const position = useMemo(() => new Map(lines.map((line, index) => [line, index])), [lines]);
  const expand = (segment: Segment) => setExpanded(set => new Set(set).add(segment.start));
  const columns = layout === 'split' ? 4 : 3;

  if (!lines.length) return <div className="diff-empty muted small">No textual changes in this file.</div>;
  return (
    <table className={cx('diff', `diff-${layout}`, wrap && 'diff-wrap')}>
      <colgroup>
        {layout === 'split' ? (
          <>
            <col className="col-ln" />
            <col className="col-code" />
            <col className="col-ln" />
            <col className="col-code" />
          </>
        ) : (
          <>
            <col className="col-ln" />
            <col className="col-ln" />
            <col className="col-code" />
          </>
        )}
      </colgroup>
      <tbody>
        {segments.map(segment =>
          segment.type === 'gap' ? (
            <tr key={`g${segment.start}`} className="diff-gap">
              <td colSpan={columns}>
                <button type="button" className="diff-expand" onClick={() => expand(segment)}>
                  <ChevronsUpDown size={13} aria-hidden /> Show {segment.count} unchanged line{segment.count === 1 ? '' : 's'}
                </button>
              </td>
            </tr>
          ) : layout === 'split' ? (
            <Fragment key={`s${segment.start}`}>
              {splitRows(lines, segment.start, segment.end).map(row => {
                const left = row.left?.kind === 'removed' ? lineAnchor(row.left) : null;
                const right = row.right && row.right.kind !== 'removed' ? lineAnchor(row.right) : null;
                return (
                  <Fragment key={row.index}>
                    <tr id={`${anchorPrefix}-${row.index}`} className="diff-row">
                      <td className={cx('ln', row.left && side(row.left, 'left'))}>
                        <LineNumber value={row.left?.old_line} anchor={left} comments={comments} />
                      </td>
                      <td className={cx('code', row.left ? side(row.left, 'left') : 'empty')} dangerouslySetInnerHTML={{ __html: row.left ? cell(row.left, html[position.get(row.left) ?? -1]) : '' }} />
                      <td className={cx('ln', row.right && side(row.right, 'right'))}>
                        <LineNumber value={row.right?.new_line} anchor={right} comments={comments} />
                      </td>
                      <td className={cx('code', row.right ? side(row.right, 'right') : 'empty')} dangerouslySetInnerHTML={{ __html: row.right ? cell(row.right, html[position.get(row.right) ?? -1]) : '' }} />
                    </tr>
                    <Annotation anchors={[left, right]} comments={comments} columns={columns} />
                  </Fragment>
                );
              })}
            </Fragment>
          ) : (
            <Fragment key={`u${segment.start}`}>
              {lines.slice(segment.start, segment.end).map((line, offset) => {
                const index = segment.start + offset;
                const kind = line.kind === 'added' ? 'add' : line.kind === 'removed' ? 'del' : line.kind === 'hunk' ? 'hunk' : 'ctx';
                const anchor = lineAnchor(line);
                return (
                  <Fragment key={index}>
                    <tr id={`${anchorPrefix}-${index}`} className={cx('diff-row', `row-${kind}`)}>
                      <td className="ln">{line.kind === 'added' ? '' : line.old_line || ''}</td>
                      <td className="ln">
                        <LineNumber value={line.kind === 'removed' ? undefined : line.new_line} anchor={anchor} comments={comments} />
                      </td>
                      <td className="code" dangerouslySetInnerHTML={{ __html: `<span class="sign">${kind === 'add' ? '+' : kind === 'del' ? '−' : ' '}</span>${html[index]}` }} />
                    </tr>
                    <Annotation anchors={[anchor]} comments={comments} columns={columns} />
                  </Fragment>
                );
              })}
            </Fragment>
          ),
        )}
      </tbody>
    </table>
  );
}

/** A line number that also opens a comment form on its line. */
function LineNumber({ value, anchor, comments }: { value?: number; anchor: LineAnchor | null; comments?: DiffComments }) {
  if (!anchor || !comments?.onAdd) return <>{value || ''}</>;
  const add = comments.onAdd;
  return (
    <span className="ln-cell">
      <button type="button" className="ln-comment" aria-label={`Comment on ${anchor.side === 'old' ? 'removed ' : ''}line ${anchor.line}`} onClick={() => add(anchor)}>
        <Plus size={12} aria-hidden />
      </button>
      {value || ''}
    </span>
  );
}

function Annotation({ anchors, comments, columns }: { anchors: (LineAnchor | null)[]; comments?: DiffComments; columns: number }) {
  if (!comments) return null;
  const content = anchors.filter((anchor): anchor is LineAnchor => Boolean(anchor) && comments.anchored.has(anchorKey(anchor!))).map(anchor => <Fragment key={anchorKey(anchor)}>{comments.render(anchor)}</Fragment>);
  if (!content.length) return null;
  return (
    <tr className="diff-annotation">
      <td colSpan={columns}>{content}</td>
    </tr>
  );
}

function side(line: DiffLine, which: 'left' | 'right'): string {
  if (line.kind === 'removed' && which === 'left') return 'del';
  if (line.kind === 'added' && which === 'right') return 'add';
  return 'ctx';
}

function cell(line: DiffLine, html: string | undefined): string {
  const sign = line.kind === 'added' ? '+' : line.kind === 'removed' ? '−' : ' ';
  return `<span class="sign">${sign}</span>${html ?? ''}`;
}
