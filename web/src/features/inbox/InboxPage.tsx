// The landing page: what needs you now, what agents are doing, and the state
// of the project, so a glance answers "what should I do next?".

import { Bot, CircleAlert, FolderOpen, GitMerge, Inbox, Play, Plus, ShieldOff, Sparkles } from 'lucide-react';
import { useFindings, useGitWorkspace, useMainProject, useOverview, useProject, useTasks, useTrust } from '@/api/hooks';
import { openNewTask } from '@/app/store';
import { useActions, useBusy } from '@/app/actions';
import { useCheckout } from '@/app/checkout';
import { navigate } from '@/lib/router';
import { taskStatus } from '@/lib/readiness';
import { plural, severityRank } from '@/lib/util';
import { Button, Callout, EmptyState, Kbd, Skeleton } from '@/components/ui';
import { SeverityBadge, isOpenFinding } from '@/components/domain';
import { TaskRow } from '@/features/tasks/TaskListPage';
import { OpenProject } from '@/features/settings/OpenProject';

export function InboxPage() {
  const main = useMainProject();
  const project = useProject().data;
  const { tasks, isLoading } = useTasks();
  const findings = (useFindings().data?.findings ?? []).filter(isOpenFinding);
  const overview = useOverview().data;
  const git = useGitWorkspace().data;
  const trust = useTrust().data;
  const actions = useActions();
  const busy = useBusy();
  const { worktreePath } = useCheckout();

  if (main.isError) {
    return (
      <div className="page-scroll">
        <div className="page-inner page-narrow">
          <EmptyState icon={FolderOpen} title="Open a project to start">
            Mini-Orca works on one local repository at a time. Choose its folder to index it.
          </EmptyState>
          <OpenProject />
        </div>
      </div>
    );
  }

  const needs = tasks.filter(t => ['needs_you', 'ready'].includes(taskStatus(t).status));
  const failed = tasks.filter(t => ['failed', 'stale'].includes(taskStatus(t).status));
  const running = tasks.filter(t => taskStatus(t).status === 'running');
  const recent = tasks.filter(t => taskStatus(t).status === 'applied').slice(0, 4);
  const toMerge = (git?.worktrees ?? []).filter(tree => !tree.current && tree.mergeable);
  const topIssues = [...findings].sort((a, b) => severityRank(a.severity) - severityRank(b.severity)).slice(0, 5);
  const coverage = overview?.analysis_coverage;
  const nothing = !needs.length && !failed.length && !running.length && !toMerge.length;

  return (
    <div className="page-scroll">
      <div className="page-inner">
        <div className="page-head">
          <div className="grow">
            <h1 className="page-title">Inbox</h1>
            <p className="page-sub">{isLoading ? ' ' : inboxSummary(needs.length, running.length, failed.length, toMerge.length)}</p>
          </div>
          <Button variant="primary" icon={Plus} onClick={() => openNewTask()} kbd="C">
            New task
          </Button>
        </div>
        {trust && !trust.trusted ? (
          <Callout tone="warning" icon={ShieldOff} title="Agents can't verify their work yet" action={<Button size="sm" onClick={() => navigate('/settings/trust')}>Allow checks</Button>}>
            Tests and lint run only after you allow this project to run its check commands in an isolated copy.
          </Callout>
        ) : null}
        {isLoading ? (
          <Skeleton lines={6} />
        ) : nothing && !tasks.length ? (
          <div className="welcome">
            <div className="welcome-card">
              <Sparkles size={22} className="tone-accent" aria-hidden />
              <h2>Describe a change. Agents do the rest; you stay in control.</h2>
              <p className="muted">Agents write the change and its tests in an isolated copy, run the checks and review it. You read the diff, approve the exact version and apply it — with Undo.</p>
              <div className="row">
                <Button variant="primary" icon={Plus} onClick={() => openNewTask()}>
                  New task
                </Button>
                <span className="xs muted">
                  or press <Kbd>C</Kbd> anywhere
                </span>
              </div>
            </div>
          </div>
        ) : nothing ? (
          <EmptyState icon={Inbox} title="Nothing needs you">
            No task is waiting for review and no agent is running.
          </EmptyState>
        ) : null}

        <div className="inbox-grid">
          <div className="stack-lg">
            {needs.length ? (
              <Section title="Needs your review" count={needs.length}>
                {needs.map(t => <TaskRow key={t.id} task={t} />)}
              </Section>
            ) : null}
            {running.length ? (
              <Section title="Agents working" count={running.length} icon={Bot}>
                {running.map(t => <TaskRow key={t.id} task={t} />)}
              </Section>
            ) : null}
            {failed.length ? (
              <Section title="Failed or stale" count={failed.length}>
                {failed.map(t => <TaskRow key={t.id} task={t} />)}
              </Section>
            ) : null}
            {toMerge.length ? (
              <Section title="Ready to merge" count={toMerge.length} icon={GitMerge}>
                {toMerge.map(tree => (
                  <button key={tree.path} type="button" className="list-row" onClick={() => navigate('/git', { path: tree.path })}>
                    <GitMerge size={14} className="tone-warning" aria-hidden />
                    <span className="list-title truncate">{tree.branch || tree.path}</span>
                    <span className="spacer" />
                    <span className="xs muted">{plural((tree.sessions ?? []).length + (tree.completed ?? []).length, 'task')}</span>
                    {(tree.overlap_paths ?? []).length ? <span className="badge badge-warning">{plural(tree.overlap_paths!.length, 'overlap')}</span> : null}
                  </button>
                ))}
              </Section>
            ) : null}
            {recent.length ? (
              <Section title="Recently applied" count={recent.length}>
                {recent.map(t => <TaskRow key={t.id} task={t} compact />)}
              </Section>
            ) : null}
          </div>
          <div className="stack-lg">
            <div className="card">
              <div className="card-head">
                <CircleAlert size={15} aria-hidden />
                <span className="strong">Open issues</span>
                <span className="spacer" />
                <button type="button" className="link-btn small" onClick={() => navigate('/issues')}>
                  View all
                </button>
              </div>
              {topIssues.length ? (
                <ul className="mini-list">
                  {topIssues.map(f => (
                    <li key={f.id}>
                      <button type="button" className="mini-row" onClick={() => navigate('/issues', { id: f.id })}>
                        <SeverityBadge severity={f.severity} />
                        <span className="truncate grow">{f.title}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <div className="card-body muted small">No open issues. Analysis findings appear here.</div>
              )}
            </div>
            <div className="card">
              <div className="card-head">
                <Sparkles size={15} aria-hidden />
                <span className="strong">Project analysis</span>
              </div>
              <div className="card-body stack">
                {coverage ? (
                  <div className="stat-row">
                    <Stat label="Fresh" value={coverage.fresh ?? 0} />
                    <Stat label="Stale" value={coverage.stale ?? 0} />
                    <Stat label="Missing" value={coverage.missing ?? 0} />
                  </div>
                ) : null}
                <p className="small muted">{overview?.analysis_run ? `Last run: ${overview.analysis_run.status}` : 'Analysis finds bugs, performance and security issues and writes the project docs.'}</p>
                {project ? (
                  <Button size="sm" icon={Play} onClick={() => void actions.runAnalysis(project, worktreePath)} disabled={Boolean(busy)}>
                    Analyze project
                  </Button>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function inboxSummary(needs: number, running: number, failed: number, toMerge: number): string {
  const parts = [
    needs ? `${plural(needs, 'task')} to review` : '',
    running ? `${plural(running, 'agent run')} in progress` : '',
    failed ? `${failed} failed or stale` : '',
    toMerge ? `${plural(toMerge, 'branch', 'branches')} ready to merge` : '',
  ].filter(Boolean);
  return parts.length ? parts.join(' · ') : 'Nothing is waiting for you.';
}

function Section({ title, count, icon: Icon, children }: { title: string; count: number; icon?: typeof Bot; children: React.ReactNode }) {
  return (
    <section className="list-group" aria-label={title}>
      <div className="list-group-head">
        {Icon ? <Icon size={13} aria-hidden /> : null}
        {title}
        <span className="count">{count}</span>
      </div>
      <div role="list">{children}</div>
    </section>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="stat">
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  );
}
