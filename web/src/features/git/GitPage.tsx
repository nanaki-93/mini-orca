// Branches and worktrees: where agent work lives, how it relates to the main
// branch, and a merge preview before integrating it.

import { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, GitBranch, GitCommitHorizontal, GitMerge, ShieldOff } from 'lucide-react';
import { useGitWorkspace, useMainProject, useTrust } from '@/api/hooks';
import type { GitWorktree, MergePreview } from '@/api/types';
import { useActions, useBusy } from '@/app/actions';
import { navigate, setParam, useRoute } from '@/lib/router';
import { parseGitDiff } from '@/lib/diff';
import { parseCommits } from '@/lib/workflow';
import { basename, languageOf, plural, shortHash } from '@/lib/util';
import { Badge, Button, Callout, EmptyState, Skeleton, Spinner, cx } from '@/components/ui';
import { TaskStatusBadge } from '@/components/domain';
import type { Tone } from '@/lib/util';
import { DiffView } from '@/features/review/DiffView';
import { useUI } from '@/app/store';

export function GitPage() {
  const route = useRoute();
  const { data: git, isLoading, error } = useGitWorkspace();
  const main = useMainProject().data;
  const trust = useTrust().data;
  const actions = useActions();
  const busy = useBusy();
  const trees = (git?.worktrees ?? []).filter(t => !t.current);
  const selectedPath = route.params.get('path') ?? trees[0]?.path ?? '';
  const selected = trees.find(t => t.path === selectedPath) ?? null;

  if (isLoading) return <div className="page-scroll"><div className="page-inner"><Skeleton lines={6} /></div></div>;
  if (error || !git?.available) return <EmptyState icon={GitBranch} title="Git is not available">This project is not a Git repository, or Git could not be read.</EmptyState>;

  return (
    <div className="split-page">
      <div className="split-main page-scroll">
        <div className="page-inner stack">
          <div className="page-head">
            <div className="grow">
              <h1 className="page-title">Branches</h1>
              <p className="page-sub">Agents can work in separate Git worktrees. Review their branch here and merge it into <span className="mono">{git.branch}</span>.</p>
            </div>
          </div>
          {git.merge_head ? (
            <Callout tone="danger" icon={AlertTriangle} title="A merge is in progress" action={main ? <Button variant="danger" size="sm" onClick={() => void actions.abortMerge(git, main)} disabled={Boolean(busy)}>Abort merge</Button> : undefined}>
              {git.conflicts.length ? `Resolve ${plural(git.conflicts.length, 'conflicted file')} in your editor and commit, or abort: ${git.conflicts.slice(0, 4).join(', ')}` : 'Finish the merge in your editor and commit, or abort it.'}
            </Callout>
          ) : null}
          <div className="card">
            <div className="card-head">
              <GitBranch size={15} aria-hidden />
              <span className="strong">{git.branch}</span>
              <Badge>main checkout</Badge>
              <span className="spacer" />
              <span className="mono xs faint">{shortHash(git.head)}</span>
              {git.dirty ? <Badge tone="warning">uncommitted changes</Badge> : null}
            </div>
          </div>
          <div className="list-group-head">Worktrees <span className="count">{trees.length}</span></div>
          {!trees.length ? (
            <EmptyState icon={GitMerge} title="No worktrees">When agents work in isolation they get their own worktree and branch, listed here to review and merge.</EmptyState>
          ) : (
            <div role="list">
              {trees.map(tree => (
                <button key={tree.path} type="button" role="listitem" className={cx('list-row', tree.path === selectedPath && 'is-active')} onClick={() => setParam('path', tree.path)}>
                  <GitBranch size={14} className={tree.mergeable ? 'tone-warning' : 'faint'} aria-hidden />
                  <span className="list-title truncate">{tree.branch || basename(tree.path)}</span>
                  <span className="spacer" />
                  <IntegrationBadge tree={tree} />
                  <span className="xs muted">{plural((tree.sessions ?? []).length + (tree.completed ?? []).length, 'task')}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      {selected && main ? <WorktreeDetail tree={selected} trusted={Boolean(trust?.trusted)} /> : null}
    </div>
  );
}

/** The worktree's relation to the main branch, as the daemon's gitops reports it. */
function integrationInfo(tree: GitWorktree): { label: string; tone: Tone } {
  if (tree.error) return { label: 'Unavailable', tone: 'danger' };
  if (tree.dirty) return { label: 'Uncommitted changes', tone: 'warning' };
  if (tree.integration === 'merged') return { label: 'Merged', tone: 'success' };
  if (tree.integration === 'unrelated') return { label: 'Unrelated history', tone: 'danger' };
  if (tree.integration === 'behind') return { label: (tree.overlap_paths ?? []).length ? plural(tree.overlap_paths!.length, 'overlapping file') : 'Main moved ahead', tone: 'warning' };
  if (tree.mergeable) return { label: 'Ready to merge', tone: 'accent' };
  if (tree.integration === 'up_to_date') return { label: 'No new commits', tone: 'neutral' };
  return { label: 'Work in progress', tone: 'neutral' };
}

function IntegrationBadge({ tree }: { tree: GitWorktree }) {
  const info = integrationInfo(tree);
  return <Badge tone={info.tone}>{info.label}</Badge>;
}

function notMergeableReason(tree: GitWorktree): string {
  if (tree.integration === 'merged') return 'This branch is already part of the main branch.';
  if (tree.dirty) return 'Commit or discard the uncommitted changes in this worktree before merging.';
  if (tree.integration === 'up_to_date') return 'This branch has no commits that the main branch lacks yet.';
  return 'Merge becomes available when the tasks on this branch are applied and committed.';
}

function WorktreeDetail({ tree, trusted }: { tree: GitWorktree; trusted: boolean }) {
  const git = useGitWorkspace().data!;
  const main = useMainProject().data!;
  const actions = useActions();
  const busy = useBusy();
  const layout = useUI(s => s.diffLayout);
  const [preview, setPreview] = useState<MergePreview | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let live = true;
    setPreview(null);
    if (!tree.mergeable) return;
    setLoading(true);
    void actions.previewMerge(git, tree, main).then(result => {
      if (live) {
        setPreview(result);
        setLoading(false);
      }
    });
    return () => {
      live = false;
    };
  }, [tree.path, tree.head]); // eslint-disable-line react-hooks/exhaustive-deps

  const files = useMemo(() => parseGitDiff(preview?.diff), [preview]);
  const commits = parseCommits(preview?.commits);
  const tasks = [...(tree.sessions ?? []), ...(tree.completed ?? [])];

  return (
    <aside className="detail-pane detail-wide" aria-label="Worktree">
      <div className="detail-head">
        <GitBranch size={15} aria-hidden />
        <span className="strong">{tree.branch || basename(tree.path)}</span>
        <IntegrationBadge tree={tree} />
        <span className="spacer" />
        {tree.mergeable ? (
          <Button variant="primary" icon={GitMerge} onClick={() => void actions.merge(git, tree, main)} disabled={!trusted || Boolean(busy) || Boolean(git.merge_head)} loading={busy === 'Merging…'}>
            Merge into {git.branch}
          </Button>
        ) : null}
      </div>
      <div className="xs faint mono truncate">{tree.path}</div>
      {!tree.mergeable ? <p className="small muted">{notMergeableReason(tree)}</p> : null}
      {!trusted && tree.mergeable ? <Callout tone="warning" icon={ShieldOff} title="Merging needs trust">Merges can run repository filters and merge drivers. Allow checks in Settings first.</Callout> : null}
      {tree.integration === 'behind' && (tree.overlap_paths ?? []).length ? (
        <Callout tone="warning" icon={AlertTriangle} title={`${plural(tree.overlap_paths!.length, 'file')} changed on both branches`}>{tree.overlap_paths!.join(', ')}</Callout>
      ) : null}
      {tasks.length ? (
        <div>
          <div className="label-row">Tasks on this branch</div>
          {tasks.map(t => (
            <button key={t.id} type="button" className="mini-row" onClick={() => navigate(`/tasks/${t.id}`, { wt: tree.path })}>
              <span className="truncate grow">{t.title}</span>
              <TaskStatusBadge entry={t} />
            </button>
          ))}
        </div>
      ) : null}
      {loading ? <div className="row small muted"><Spinner size={14} /> Preparing merge preview…</div> : null}
      {commits.length ? (
        <div>
          <div className="label-row">{plural(commits.length, 'commit')}</div>
          <ul className="commits">{commits.map(c => (<li key={c.sha}><GitCommitHorizontal size={13} aria-hidden /><span className="mono xs faint">{c.sha.slice(0, 7)}</span><span className="small truncate">{c.subject}</span></li>))}</ul>
        </div>
      ) : null}
      {files.map(file => (
        <div key={file.path} className="merge-file">
          <div className="fc-diff-head"><span className="strong small truncate">{file.path}</span></div>
          {file.binary ? <div className="small muted">Binary file</div> : <DiffView lines={file.lines} language={languageOf(file.path)} layout={layout} wrap={false} anchorPrefix={`m-${file.path}`} />}
        </div>
      ))}
    </aside>
  );
}
