// A GitHub-style file diff: unified or split, collapsed unchanged regions that
// expand on click, syntax highlighting and line numbers.

import { Fragment, useMemo, useState } from 'react';
import { ChevronsUpDown } from 'lucide-react';
import type { DiffLine } from '@/api/types';
import { buildSegments, splitRows, type Segment } from '@/lib/diff';
import { escapeHtml, highlightLine } from '@/lib/highlight';
import { cx } from '@/components/ui';
import type { DiffLayout } from '@/app/store';

export function DiffView({ lines, language, layout, wrap, anchorPrefix }: { lines: DiffLine[]; language: string; layout: DiffLayout; wrap: boolean; anchorPrefix: string }) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const segments = useMemo(() => buildSegments(lines, 3, expanded), [lines, expanded]);
  const html = useMemo(() => lines.map(line => (line.kind === 'hunk' ? escapeHtml(line.text) : highlightLine(line.text, language))), [lines, language]);
  const position = useMemo(() => new Map(lines.map((line, index) => [line, index])), [lines]);
  const expand = (segment: Segment) => setExpanded(set => new Set(set).add(segment.start));

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
              <td colSpan={layout === 'split' ? 4 : 3}>
                <button type="button" className="diff-expand" onClick={() => expand(segment)}>
                  <ChevronsUpDown size={13} aria-hidden /> Show {segment.count} unchanged line{segment.count === 1 ? '' : 's'}
                </button>
              </td>
            </tr>
          ) : layout === 'split' ? (
            <Fragment key={`s${segment.start}`}>
              {splitRows(lines, segment.start, segment.end).map(row => (
                <tr key={row.index} id={`${anchorPrefix}-${row.index}`} className="diff-row">
                  <td className={cx('ln', row.left && side(row.left, 'left'))}>{row.left?.old_line || ''}</td>
                  <td className={cx('code', row.left ? side(row.left, 'left') : 'empty')} dangerouslySetInnerHTML={{ __html: row.left ? cell(row.left, html[position.get(row.left) ?? -1]) : '' }} />
                  <td className={cx('ln', row.right && side(row.right, 'right'))}>{row.right?.new_line || ''}</td>
                  <td className={cx('code', row.right ? side(row.right, 'right') : 'empty')} dangerouslySetInnerHTML={{ __html: row.right ? cell(row.right, html[position.get(row.right) ?? -1]) : '' }} />
                </tr>
              ))}
            </Fragment>
          ) : (
            <Fragment key={`u${segment.start}`}>
              {lines.slice(segment.start, segment.end).map((line, offset) => {
                const index = segment.start + offset;
                const kind = line.kind === 'added' ? 'add' : line.kind === 'removed' ? 'del' : line.kind === 'hunk' ? 'hunk' : 'ctx';
                return (
                  <tr key={index} id={`${anchorPrefix}-${index}`} className={cx('diff-row', `row-${kind}`)}>
                    <td className="ln">{line.kind === 'added' ? '' : line.old_line || ''}</td>
                    <td className="ln">{line.kind === 'removed' ? '' : line.new_line || ''}</td>
                    <td className="code" dangerouslySetInnerHTML={{ __html: `<span class="sign">${kind === 'add' ? '+' : kind === 'del' ? '−' : ' '}</span>${html[index]}` }} />
                  </tr>
                );
              })}
            </Fragment>
          ),
        )}
      </tbody>
    </table>
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
