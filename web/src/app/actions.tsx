// User actions against the daemon. Every mutation runs at most once at a time,
// carries the captured project/proposal identity, asks for explicit consent
// where the daemon requires `confirm`, and refreshes what it changed.

import { useSyncExternalStore } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, errorMessage, query, withWorktree } from '@/api/client';
import type { AgentPreferences, AgentSettings, AnalysisFileSelection, AnalysisRunSummary, ProjectSetup, SetupModel, SetupModelTest, SetupScope, SetupTimeouts, ChangeHistoryEntry, ChangeMutationResult, ChangeSession, ExecutionTrust, Finding, GitWorkspace, GitWorktree, LoadedSession, MergePreview, ModelsCurrent, Project } from '@/api/types';
import { useFeedback } from '@/components/overlays';
import { canOverrideReview, changeIdentity, openComments, proposalReadiness, workflowActive } from '@/lib/readiness';
import { changeOverlaps, mergeRequest, runStartOptions, workflowDestinations, workflowRequest } from '@/lib/workflow';
import { navigate } from '@/lib/router';
import { commentsBrief, isTestPath, plural } from '@/lib/util';
import { RunStartChoice } from '@/components/domain';
import { keys } from '@/api/hooks';

// ------------------------------------------------------------------ busy state

let busyLabel = '';
const listeners = new Set<() => void>();
function setBusy(label: string) {
  busyLabel = label;
  listeners.forEach(fn => fn());
}

export function useBusy(): string {
  return useSyncExternalStore(
    fn => {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    () => busyLabel,
  );
}

// ------------------------------------------------------------ revision state

/** A revision request in flight: when it started and how to stop waiting for it. */
export interface PendingRevision {
  sessionId: string;
  since: number;
  stop: () => void;
}

let pendingRevision: PendingRevision | null = null;
const revisionListeners = new Set<() => void>();
function setPendingRevision(next: PendingRevision | null) {
  pendingRevision = next;
  revisionListeners.forEach(fn => fn());
}

/** The revision a task is waiting for, so its page can show the time and Stop. */
export function usePendingRevision(): PendingRevision | null {
  return useSyncExternalStore(
    fn => {
      revisionListeners.add(fn);
      return () => revisionListeners.delete(fn);
    },
    () => pendingRevision,
  );
}

const changePath = (session: LoadedSession, suffix: string) => withWorktree('/api/projects/current/changes/' + encodeURIComponent(session.id) + suffix, session.checkout.worktreePath);

/**
 * One dispatch window's bounds: up to 64 files and 30 minutes, two attempts
 * per stage. A window that ends pauses the run; resume continues it.
 */
const ANALYSIS_LIMITS = { batch_files: 64, budget_seconds: 1800, max_attempts_per_stage: 2 };

export interface AnalysisOptions {
  /** Retry only stages that failed, went stale or were canceled. */
  retryFailed?: boolean;
  /** Analyze again even where saved results are current. */
  refresh?: boolean;
  /** Resume this paused or interrupted run. */
  resume?: AnalysisRunSummary;
}

interface AnalysisPreview {
  identity: unknown;
  preview_id: string;
  providers?: { id: string; remote_confirmation_required?: boolean; model?: { model?: string; scope?: string; provider_origin?: string } }[];
  /** Retained files are outside a retry's scope: they keep their saved results. */
  files?: { retained?: boolean }[];
  max_model_requests?: number;
  security_review_intent_required?: boolean;
}

/** What deleting a task needs: the version shown and the checkout that owns it. */
export interface DeletableTask {
  id: string;
  title: string;
  worktreePath: string;
  project_id: string;
  project_revision: string;
  revision: number;
  hash: string;
}

export interface DraftTask {
  title: string;
  kind: string;
  description: string;
  paths: string[];
  criteria: string[];
  start: 'agents' | 'quick' | 'draft';
  autoRepair: boolean;
}

export function useActions() {
  const client = useQueryClient();
  const { confirm, toast } = useFeedback();

  async function guarded<T>(label: string, fn: () => Promise<T>): Promise<T | undefined> {
    if (busyLabel) {
      toast(`Wait for “${busyLabel.replace(/…$/, '')}” to finish.`, 'warning');
      return undefined;
    }
    setBusy(label);
    try {
      return await fn();
    } finally {
      setBusy('');
    }
  }

  function fail(prefix: string, err: unknown) {
    toast(prefix, 'danger', errorMessage(err));
  }

  function setSession(next: ChangeSession | null | undefined, previous: LoadedSession) {
    if (!next) return;
    const merged: LoadedSession = { ...next, checkout: previous.checkout, undo: previous.undo };
    client.setQueryData(keys.session(previous.id), merged);
    void client.invalidateQueries({ queryKey: ['changes'] });
  }

  function refreshAll() {
    return client.invalidateQueries();
  }

  async function confirmRemote(scope: string, action: string): Promise<{ ok: boolean; remote: boolean }> {
    const models = client.getQueryData<ModelsCurrent>(keys.models);
    const model = models?.scopes?.[scope];
    if (!model?.remote_provider) return { ok: true, remote: false };
    const ok = await confirm({
      title: 'Send code to a remote model?',
      body: `${action} uses ${model.model || scope} at ${model.provider_origin || 'a remote provider'}. Bounded project context for this request leaves your machine.`,
      confirmLabel: 'Send request',
    });
    return { ok, remote: true };
  }

  /** Names the running tasks in this checkout that applying `changed` likely stops; the rest keep running. */
  function runningTasksNote(session: { id: string; checkout: { worktreePath: string } }, changed: string[]): string {
    const lists = client.getQueriesData<ChangeHistoryEntry[]>({ queryKey: ['changes'] });
    const running = new Map<string, ChangeHistoryEntry>();
    for (const [key, list] of lists) {
      const checkout = String(key[1] ?? '');
      for (const task of list ?? []) {
        const where = task.worktree_path || checkout;
        if (task.id !== session.id && where === session.checkout.worktreePath && ['running', 'canceling'].includes(task.workflow_status ?? '') && changeOverlaps(changed, task.paths ?? [])) running.set(task.id, task);
      }
    }
    if (!running.size) return '';
    const names = [...running.values()].slice(0, 3).map(task => `“${task.title || task.id}”`).join(', ');
    return ` It stops ${plural(running.size, 'running task')} whose files it changes: ${names}${running.size > 3 ? ', …' : ''}.`;
  }

  // ---------------------------------------------------------------- tasks

  async function createTask(project: Project, worktreePath: string, draft: DraftTask): Promise<string | null> {
    const created = await guarded('Creating task…', async () => {
      try {
        return await api.post<ChangeSession>(withWorktree('/api/projects/current/changes', worktreePath), {
          project_id: project.project_id,
          project_revision: project.project_revision,
          kind: draft.kind,
          title: draft.title.trim(),
          paths: draft.paths,
          acceptance_criteria: draft.criteria.length ? draft.criteria : ['Changes pass verification checks'],
          plan: draft.description.trim() ? { problem: draft.title.trim(), approach: draft.description.trim() } : undefined,
        });
      } catch (err) {
        fail('Could not create the task', err);
        return null;
      }
    });
    if (!created) return null;
    const loaded: LoadedSession = { ...created, checkout: { project_id: created.project_id || project.project_id, project_revision: created.project_revision || project.project_revision, worktreePath } };
    client.setQueryData(keys.session(created.id), loaded);
    void client.invalidateQueries({ queryKey: ['changes'] });
    navigate(`/tasks/${created.id}`, { wt: worktreePath || undefined });
    const prompt = draft.title.trim() + (draft.description.trim() ? `\n\nSpecification:\n${draft.description.trim()}` : '');
    if (draft.start === 'agents') await startAgents(loaded, { message: prompt, autoRepair: draft.autoRepair });
    else if (draft.start === 'quick') await refine(loaded, prompt);
    else toast('Draft saved.', 'success');
    return created.id;
  }

  async function startAgents(session: LoadedSession, options: { message?: string; autoRepair?: boolean } = {}): Promise<boolean> {
    const comments = openComments(session).length;
    const message = (options.message ?? session.title).trim();
    if (!message) {
      toast('Describe the change for the agents first.', 'warning');
      return false;
    }
    let settings: AgentSettings | null = null;
    try {
      settings = await api.get<AgentSettings>(withWorktree('/api/projects/current/agent-settings' + query({ project_id: session.checkout.project_id, project_revision: session.checkout.project_revision }), session.checkout.worktreePath));
    } catch {
      settings = null; // Daemon defaults apply; unknown destinations are never auto-confirmed.
    }
    const choice = workflowDestinations(settings, client.getQueryData<ModelsCurrent>(keys.models));
    const security = session.kind === 'security';
    const starts = runStartOptions(session, choice.selections.create, isTestPath);
    let start = starts.initial; // The choice below updates it while the dialog is open.
    const again = starts.options.length > 0;
    const ok = await confirm({
      title: again ? 'Run the agents again?' : 'Start the agents?',
      body: (
        <div className="stack">
          {again ? (
            <>
              <RunStartChoice options={starts.options} initial={starts.initial} comments={comments} onChange={value => (start = value)} />
              <p className="small muted">Checks run in an isolated copy of the project. Nothing reaches your files until you review and apply the result.{options.autoRepair ? ' A failed check or a rejecting review starts an automatic repair of the code (up to two).' : ''}</p>
            </>
          ) : (
            <p>Agents write the change, write and run tests in an isolated copy of the project, then specialist reviewers check it. Nothing reaches your files until you review and apply it.</p>
          )}
          {choice.destinations.length ? (
            <div className="consent-list">
              <div className="small strong">Sends code to remote models</div>
              {choice.destinations.map(d => (
                <div key={d.stage} className="small">
                  <span className="muted">{d.stage}</span> · {d.model} <span className="faint">at {d.origin}</span>
                </div>
              ))}
            </div>
          ) : (
            <p className="small muted">All selected models run locally.</p>
          )}
          {security ? <p className="small">This is a security task: a security specialist reviews it.</p> : null}
          {comments && !again ? <p className="small">{commentsBrief(comments)}</p> : null}
        </div>
      ),
      confirmLabel: again ? 'Run agents' : 'Start agents',
    });
    if (!ok) return false;
    return Boolean(
      await guarded('Starting agents…', async () => {
        try {
          const body = workflowRequest(changeIdentity(session), message, choice, { autoRepair: options.autoRepair, confirmSecurity: security, planHash: session.plan?.hash, includeComments: comments > 0 && start !== 'checks', start });
          setSession(await api.post<ChangeSession>(changePath(session, '/workflow'), body), session);
          toast('Agents started. Progress updates live.', 'info');
          return true;
        } catch (err) {
          fail('The agents could not start', err);
          return false;
        }
      }),
    );
  }

  async function cancelAgents(session: LoadedSession) {
    if (!workflowActive(session) || !session.workflow) return;
    if (!(await confirm({ title: 'Stop the agents?', body: 'Running agents stop. Nothing is applied.', confirmLabel: 'Stop agents', cancelLabel: 'Keep running', danger: true }))) return;
    await guarded('Stopping agents…', async () => {
      try {
        setSession(await api.post<ChangeSession>(changePath(session, '/workflow/cancel'), { project_id: session.checkout.project_id, project_revision: session.checkout.project_revision, workflow_id: session.workflow!.id }), session);
        toast('Agents stopped.', 'info');
      } catch (err) {
        fail('Could not stop the agents', err);
      }
    });
  }

  /** Revises the proposal from a message and, when asked, the open line comments. */
  async function refine(session: LoadedSession, message: string, includeComments = false): Promise<boolean> {
    const text = message.trim();
    if (!text && !includeComments) {
      toast('Describe what should change first.', 'warning');
      return false;
    }
    const consent = await confirmRemote('code', `Revising “${session.title || session.id}”`);
    if (!consent.ok) return false;
    return Boolean(
      await guarded('Generating…', async () => {
        // The model call runs inside this request; stopping closes it, and the
        // daemon cancels the call and saves nothing.
        const controller = new AbortController();
        setPendingRevision({ sessionId: session.id, since: Date.now(), stop: () => controller.abort() });
        try {
          setSession(await api.post<ChangeSession>(changePath(session, '/messages'), { ...changeIdentity(session), message: text, profile: 'code', confirm_remote_provider: consent.remote, include_comments: includeComments || undefined }, controller.signal), session);
          toast('Proposal updated. Review the new diff.', 'success');
          return true;
        } catch (err) {
          if (controller.signal.aborted) {
            // A revision that finished as Stop was pressed is kept: show the saved version.
            void client.invalidateQueries({ queryKey: keys.session(session.id) });
            toast('Revision stopped.', 'info');
          } else {
            fail('Generation failed', err);
          }
          return false;
        } finally {
          setPendingRevision(null);
        }
      }),
    );
  }

  /** Stops waiting for the task's revision; the daemon then cancels the model call. */
  async function stopRevision(session: LoadedSession) {
    const pending = pendingRevision;
    if (pending?.sessionId !== session.id) return;
    if (!(await confirm({ title: 'Stop the revision?', body: 'The agent stops and this version stays as it is. Nothing is applied.', confirmLabel: 'Stop revision', cancelLabel: 'Keep waiting', danger: true }))) return;
    pending.stop();
  }

  // ---------------------------------------------------------------- review

  async function addComment(session: LoadedSession, anchor: { path: string; side: 'new' | 'old'; line: number }, text: string): Promise<boolean> {
    return Boolean(
      await guarded('Saving comment…', async () => {
        try {
          setSession(await api.post<ChangeSession>(changePath(session, '/comments'), { ...changeIdentity(session), ...anchor, text: text.trim() }), session);
          return true;
        } catch (err) {
          fail('Could not save the comment', err);
          return false;
        }
      }),
    );
  }

  async function deleteComment(session: LoadedSession, id: string): Promise<void> {
    await guarded('Deleting comment…', async () => {
      try {
        setSession(await api.post<ChangeSession>(changePath(session, '/comments/delete'), { ...changeIdentity(session), id }), session);
      } catch (err) {
        fail('Could not delete the comment', err);
      }
    });
  }

  async function markViewed(session: LoadedSession, path: string): Promise<boolean> {
    return Boolean(
      await guarded('Saving review…', async () => {
        try {
          setSession(await api.post<ChangeSession>(changePath(session, '/review-file'), { ...changeIdentity(session), path }), session);
          return true;
        } catch (err) {
          fail(`Could not mark ${path} viewed`, err);
          return false;
        }
      }),
    );
  }

  async function runChecks(session: LoadedSession) {
    await guarded('Running checks…', async () => {
      try {
        const updated = await api.post<ChangeSession>(changePath(session, '/checks'), { ...changeIdentity(session), run_lint: true, run_tests: true });
        setSession(updated, session);
        const failed = (updated.checks ?? []).filter(check => check.state === 'failed').length;
        toast(failed ? `${plural(failed, 'check')} failed.` : 'All checks passed.', failed ? 'warning' : 'success');
      } catch (err) {
        fail('Checks could not run', err);
      }
    });
  }

  /** Approves this exact version; the daemon also answers a waiting workflow input. */
  async function approve(session: LoadedSession, notes = ''): Promise<boolean> {
    if (!proposalReadiness(session).canApprove) return false;
    const result = await guarded('Approving…', async () => {
      try {
        const body = notes.trim() ? { ...changeIdentity(session), notes: notes.trim() } : changeIdentity(session);
        setSession(await api.post<ChangeSession>(changePath(session, '/review'), body), session);
        toast('Approved this exact version. You can apply it now.', 'success');
        return true;
      } catch (err) {
        fail('Approval failed', err);
        return false;
      }
    });
    return Boolean(result);
  }

  /** Overrides the reviewers' rejection of this version; it approves nothing. */
  async function overrideReview(session: LoadedSession, reason: string): Promise<boolean> {
    if (!canOverrideReview(session) || !reason.trim()) return false;
    const result = await guarded('Overriding…', async () => {
      try {
        setSession(await api.post<ChangeSession>(changePath(session, '/review-override'), { ...changeIdentity(session), workflow_id: session.workflow!.id, reason: reason.trim() }), session);
        toast('Reviewers overridden. View the files and approve this version to apply it.', 'info');
        return true;
      } catch (err) {
        fail('Could not override the reviewers', err);
        return false;
      }
    });
    return Boolean(result);
  }

  async function reject(session: LoadedSession) {
    const input = session.workflow?.input;
    if (!input || input.status !== 'waiting') return;
    if (!(await confirm({ title: 'Reject this proposal?', body: 'The workflow ends and this version can no longer be approved.', confirmLabel: 'Reject', danger: true }))) return;
    await guarded('Rejecting…', async () => {
      try {
        setSession(await api.post<ChangeSession>(changePath(session, '/workflow/input'), { ...changeIdentity(session), workflow_id: session.workflow!.id, request_id: input.request_id, decision: 'reject' }), session);
        toast('Proposal rejected.', 'info');
      } catch (err) {
        fail('Could not reject', err);
      }
    });
  }

  async function apply(session: LoadedSession, project: Project | undefined, options: { commit: boolean; message: string }) {
    if (!proposalReadiness(session).canApply) return;
    const destination = session.checkout.worktreePath || project?.path || 'the current checkout';
    const commit = options.commit && Boolean(session.git?.head);
    if (commit && !options.message.trim()) {
      toast('Enter a commit message or turn off “Commit”.', 'warning');
      return;
    }
    const ok = await confirm({
      title: commit ? 'Apply and commit?' : 'Apply these changes?',
      body: `Writes “${session.title || session.id}” to ${destination}${commit ? ` and commits it on ${session.git?.branch || 'the current branch'}` : ''}. Undo stays available for the latest Apply while the files are unchanged${commit ? '; it restores the files but keeps the commit' : ''}.${runningTasksNote(session, (session.changes ?? []).map(change => change.path))}`,
      confirmLabel: commit ? 'Apply & commit' : 'Apply',
    });
    if (!ok) return;
    await guarded('Applying…', async () => {
      try {
        const body: Record<string, unknown> = { ...changeIdentity(session), confirm: true };
        if (commit) Object.assign(body, { head: session.git!.head, branch: session.git!.branch, commit_message: options.message.trim() });
        const result = await api.post<ChangeMutationResult>(changePath(session, '/apply'), body);
        if (result?.commit?.state === 'failed') toast('Applied, but the commit failed.', 'warning', result.commit.error);
        else toast(result?.commit?.state === 'committed' ? 'Applied and committed.' : 'Applied.', 'success');
        for (const warning of result?.warnings ?? []) toast(warning, 'warning');
        await refreshAll();
      } catch (err) {
        fail('Apply failed', err);
      }
    });
  }

  async function undo(session: LoadedSession) {
    if (!proposalReadiness(session).canUndo) return;
    const ok = await confirm({
      title: 'Undo these changes?',
      body: `Restores the files changed by “${session.title || session.id}” after verifying they still match the applied version.${session.commit?.state === 'committed' ? ' The commit stays in history.' : ''}${runningTasksNote(session, (session.changes ?? []).map(change => change.path))}`,
      confirmLabel: 'Undo',
      danger: true,
    });
    if (!ok) return;
    await guarded('Undoing…', async () => {
      try {
        await api.post(changePath(session, '/undo'), { ...changeIdentity(session), confirm: true });
        toast('Changes undone.', 'success');
        await refreshAll();
      } catch (err) {
        fail('Undo failed', err);
      }
    });
  }

  // ---------------------------------------------------------------- project

  async function setTrust(project: Project, worktreePath: string, trust: ExecutionTrust | undefined, next: boolean) {
    const commands = (trust?.commands ?? []).map(argv => argv.join(' '));
    const ok = await confirm({
      title: next ? 'Allow this project to run checks?' : 'Stop running project checks?',
      body: next ? `Checks and agent verification run these commands in an isolated copy of the project: ${commands.join(' · ') || 'the project check commands'}.` : 'Checks, scans and agent verification stop running project code.',
      confirmLabel: next ? 'Allow' : 'Revoke',
      danger: !next,
    });
    if (!ok) return;
    await guarded('Saving trust…', async () => {
      try {
        await api.post(withWorktree('/api/projects/current/execution-trust', worktreePath), { project_revision: project.project_revision, confirm: next });
        toast(next ? 'Checks allowed for this project.' : 'Trust revoked.', 'success');
        void client.invalidateQueries({ queryKey: ['trust'] });
      } catch (err) {
        fail('Could not update trust', err);
      }
    });
  }

  async function reindex(project: Project, worktreePath: string) {
    await guarded('Reindexing…', async () => {
      try {
        await api.post(withWorktree('/api/projects/current/reindex', worktreePath), { project_revision: project.project_revision });
        toast('Project reindexed.', 'success');
        await refreshAll();
      } catch (err) {
        fail('Reindex failed', err);
      }
    });
  }

  async function openProject(path: string) {
    await guarded('Opening project…', async () => {
      try {
        try {
          await api.post('/api/projects/restore', { project_path: path });
        } catch {
          await api.post('/api/projects/import', { project_path: path });
        }
        navigate('/', { wt: undefined });
        toast(`Opened ${path}`, 'success');
        await refreshAll();
      } catch (err) {
        fail('Could not open the project', err);
      }
    });
  }

  async function triageFinding(project: Project, worktreePath: string, finding: Finding, status: 'dismissed' | 'open') {
    await guarded('Updating issue…', async () => {
      try {
        await api.patch(withWorktree('/api/projects/current/findings/' + encodeURIComponent(finding.id), worktreePath), { project_revision: project.project_revision, status });
        toast(status === 'dismissed' ? 'Issue dismissed.' : 'Issue reopened.', 'success');
        void client.invalidateQueries({ queryKey: ['findings'] });
      } catch (err) {
        fail('Could not update the issue', err);
      }
    });
  }

  /** Previews an analysis (or the resumption of a stopped run), asks for consent, then starts it. */
  async function runAnalysis(project: Project, worktreePath: string, options: AnalysisOptions = {}) {
    const resume = options.resume?.identity;
    await guarded(resume ? 'Preparing to resume…' : 'Preparing analysis…', async () => {
      try {
        // A resumption repeats the run's own plan options.
        const plan = options.resume?.plan;
        const limits = plan?.limits ?? ANALYSIS_LIMITS;
        const refresh = resume ? Boolean(plan?.refresh) : Boolean(options.refresh);
        const retryFailed = resume ? Boolean(plan?.retry_stale_failed) : Boolean(options.retryFailed);
        const preview = await api.post<AnalysisPreview>(withWorktree('/api/projects/current/analysis/preview', worktreePath), {
          project_id: project.project_id,
          project_revision: project.project_revision,
          scope: 'project',
          refresh,
          retry_stale_failed: retryFailed || undefined,
          resume_run: resume,
          limits,
        });
        const providers = preview.providers ?? [];
        const remote = providers.filter(p => p.remote_confirmation_required).map(p => `${p.model?.model || p.model?.scope} at ${p.model?.provider_origin || 'remote provider'}`);
        const security = Boolean(preview.security_review_intent_required);
        const what = options.retryFailed ? 'Retries the failed, stale and canceled stages of' : options.refresh ? 'Analyzes again, ignoring saved results,' : resume ? 'Resumes the analysis of' : 'Analyzes';
        const files = preview.files ?? [];
        const kept = files.filter(file => file.retained).length;
        const ok = await confirm({
          title: resume ? 'Resume the analysis?' : options.retryFailed ? 'Retry failed analysis?' : 'Analyze the project?',
          body: `${what} ${plural(files.length - kept, 'file')} (up to ${preview.max_model_requests ?? 0} model requests) for bugs, performance and security, then refreshes the generated docs.${kept ? ` The other ${plural(kept, 'file')} keep their saved results.` : ''}${security ? ' Includes an AI security review.' : ''}${remote.length ? ` Sends bounded source context to ${remote.join(', ')}.` : ''}`,
          confirmLabel: resume ? 'Resume' : 'Analyze',
        });
        if (!ok) return;
        const confirmations = { provider_ids: providers.map(p => p.id).filter(Boolean), security_review: security };
        if (resume) {
          await api.post(withWorktree('/api/projects/current/analysis/run/control', worktreePath), { identity: resume, action: 'resume', preview_id: preview.preview_id, confirmations });
        } else {
          await api.post(withWorktree('/api/projects/current/analysis/run', worktreePath), { identity: preview.identity, preview_id: preview.preview_id, limits, refresh, retry_stale_failed: retryFailed || undefined, confirmations });
        }
        toast(resume ? 'Analysis resumed.' : 'Analysis started.', 'info');
        void client.invalidateQueries({ queryKey: ['overview'] });
      } catch (err) {
        fail(resume ? 'Analysis could not resume' : 'Analysis could not start', err);
      }
    });
  }

  /** Pauses or cancels the running analysis; finished stages keep their results. */
  async function controlAnalysis(run: AnalysisRunSummary, worktreePath: string, action: 'pause' | 'cancel') {
    if (!run.identity) return;
    if (action === 'cancel') {
      const ok = await confirm({ title: 'Cancel the analysis?', body: 'Running model requests stop. Finished files keep their results; a new analysis starts over for the rest.', confirmLabel: 'Cancel analysis', cancelLabel: 'Keep running', danger: true });
      if (!ok) return;
    }
    await guarded(action === 'pause' ? 'Pausing analysis…' : 'Canceling analysis…', async () => {
      try {
        await api.post(withWorktree('/api/projects/current/analysis/run/control', worktreePath), { identity: run.identity, action });
        toast(action === 'pause' ? 'Analysis pauses after the current requests.' : 'Analysis canceled.', 'info');
        void client.invalidateQueries({ queryKey: ['overview'] });
      } catch (err) {
        fail(action === 'pause' ? 'Could not pause the analysis' : 'Could not cancel the analysis', err);
      }
    });
  }

  // --------------------------------------------------------------- project setup

  /** Saves the project's configuration file; the daemon applies it at once. */
  async function saveSetup(setup: ProjectSetup, draft: Pick<ProjectSetup, 'models' | 'roles' | 'limits' | 'timeouts'>): Promise<ProjectSetup | null> {
    return (await guarded('Saving the project configuration…', async () => {
      try {
        const saved = await api.post<ProjectSetup>('/api/projects/current/setup', { project_id: setup.project_id, project_revision: setup.project_revision, models: draft.models, roles: draft.roles, limits: draft.limits, timeouts: draft.timeouts });
        client.setQueryData([...keys.setup, setup.project_revision], saved);
        void client.invalidateQueries({ queryKey: ['models'] });
        void client.invalidateQueries({ queryKey: ['settings'] });
        toast('Project configuration saved.', 'success', saved.config_file);
        if (saved.overrides_reset) toast('Agent overrides from Settings → Models & agents were replaced by these settings.', 'info');
        return saved;
      } catch (err) {
        fail('Could not save the project configuration', err);
        return null;
      }
    })) ?? null;
  }

  /** Saves the project's time limits alone and applies them at once. */
  async function saveTimeouts(setup: ProjectSetup, timeouts: SetupTimeouts): Promise<boolean> {
    return (await guarded('Saving time limits…', async () => {
      try {
        const saved = await api.post<ProjectSetup>('/api/projects/current/setup/timeouts', { project_id: setup.project_id, project_revision: setup.project_revision, timeouts });
        client.setQueryData([...keys.setup, setup.project_revision], saved);
        void client.invalidateQueries({ queryKey: ['models'] });
        toast('Time limits saved.', 'success', saved.config_file);
        return true;
      } catch (err) {
        fail('Could not save the time limits', err);
        return false;
      }
    })) ?? false;
  }

  /** Keeps the daemon's configuration for this project and stops offering setup. */
  async function skipSetup(setup: ProjectSetup): Promise<boolean> {
    return (await guarded('Skipping setup…', async () => {
      try {
        const skipped = await api.post<ProjectSetup>('/api/projects/current/setup/skip', { project_id: setup.project_id, project_revision: setup.project_revision });
        client.setQueryData([...keys.setup, setup.project_revision], skipped);
        return true;
      } catch (err) {
        fail('Could not skip setup', err);
        return false;
      }
    })) ?? false;
  }

  /** Sends one short request to a model as setup would configure it. */
  async function testSetupModel(setup: ProjectSetup, scope: SetupScope, model: SetupModel): Promise<SetupModelTest | null> {
    try {
      return await api.post<SetupModelTest>('/api/projects/current/setup/test-model', { project_id: setup.project_id, project_revision: setup.project_revision, scope, model });
    } catch (err) {
      fail('Could not test the model', err);
      return null;
    }
  }

  /** Saves which files analysis leaves out. */
  async function saveAnalysisSelection(selection: AnalysisFileSelection, excluded: string[]): Promise<boolean> {
    return (await guarded('Saving analysis exclusions…', async () => {
      try {
        const saved = await api.post<AnalysisFileSelection>('/api/projects/current/analysis/selection', { project_id: selection.project_id, project_revision: selection.project_revision, selection_id: selection.selection_id, excluded_paths: excluded });
        client.setQueryData([...keys.selection, selection.project_revision], saved);
        return true;
      } catch (err) {
        fail('Could not save analysis exclusions', err);
        return false;
      }
    })) ?? false;
  }

  // ------------------------------------------------------------------ deletion

  /**
   * Deletes tasks after one confirmation and returns how many were deleted.
   * `leave` runs first, so a page showing a deleted task navigates away
   * before the task list refreshes; its cached page expires unused.
   */
  async function deleteTasks(tasks: DeletableTask[], leave?: () => void): Promise<number> {
    if (!tasks.length) return 0;
    const one = tasks.length === 1;
    const ok = await confirm({
      title: one ? 'Delete this task?' : `Delete ${tasks.length} tasks?`,
      body: one
        ? `“${tasks[0].title || tasks[0].id}” and its conversation, proposals and comments are removed. Project files are not changed.`
        : `Their conversations, proposals and comments are removed. Project files are not changed. ${tasks.slice(0, 5).map(t => `“${t.title || t.id}”`).join(', ')}${tasks.length > 5 ? ` and ${tasks.length - 5} more` : ''}.`,
      confirmLabel: one ? 'Delete task' : `Delete ${tasks.length} tasks`,
      danger: true,
    });
    if (!ok) return 0;
    return (await guarded(one ? 'Deleting task…' : 'Deleting tasks…', async () => {
      const deleted: string[] = [];
      const failures: string[] = [];
      for (const task of tasks) {
        try {
          await api.post(withWorktree('/api/projects/current/changes/' + encodeURIComponent(task.id) + '/delete', task.worktreePath), { project_id: task.project_id, project_revision: task.project_revision, revision: task.revision, hash: task.hash });
          deleted.push(task.id);
        } catch (err) {
          failures.push(`${task.title || task.id}: ${errorMessage(err)}`);
        }
      }
      if (deleted.length) leave?.();
      void client.invalidateQueries({ queryKey: ['changes'] });
      if (deleted.length) toast(one ? 'Task deleted.' : `${plural(deleted.length, 'task')} deleted.`, 'success');
      if (failures.length) toast(`Could not delete ${plural(failures.length, 'task')}`, 'danger', failures.join('\n'));
      return deleted.length;
    })) ?? 0;
  }

  // ------------------------------------------------------------ agent settings

  /** Replaces the project's agent roles and budget, or resets them to config.yaml. */
  async function controlAgentSettings(settings: AgentSettings, worktreePath: string, values: AgentPreferences | null) {
    const reset = values === null;
    return guarded(reset ? 'Resetting agent settings…' : 'Saving agent settings…', async () => {
      try {
        const saved = await api.post<AgentSettings>(withWorktree('/api/projects/current/agent-settings/control', worktreePath), {
          project_id: settings.project_id,
          project_revision: settings.project_revision,
          configuration: settings.configuration,
          version: settings.version,
          action: reset ? 'reset' : 'replace',
          confirm: true,
          ...(reset ? {} : { values }),
        });
        client.setQueryData([...keys.settings(worktreePath), settings.project_revision], saved);
        void client.invalidateQueries({ queryKey: keys.settings(worktreePath) });
        toast(reset ? 'Agents follow the configuration again.' : 'Agent settings saved. New runs use them.', 'success');
        return saved;
      } catch (err) {
        fail(reset ? 'Could not reset agent settings' : 'Could not save agent settings', err);
        return null;
      }
    });
  }

  // ---------------------------------------------------------------- git

  async function previewMerge(workspace: GitWorkspace, worktree: GitWorktree, main: Project): Promise<MergePreview | null> {
    try {
      return await api.post<MergePreview>('/api/projects/current/git/merge-preview', mergeRequest(workspace, worktree, main));
    } catch (err) {
      fail('Merge preview failed', err);
      return null;
    }
  }

  async function merge(workspace: GitWorkspace, worktree: GitWorktree, main: Project) {
    const overlap = worktree.overlap_paths ?? [];
    const ok = await confirm({
      title: `Merge ${worktree.branch || 'worktree'} into ${workspace.branch || 'the current branch'}?`,
      body: `Merges ${worktree.head.slice(0, 7)}, marks its linked issues done, then removes the worktree and its branch.${overlap.length ? ` ${plural(overlap.length, 'path')} changed on both branches: ${overlap.slice(0, 3).join(', ')}${overlap.length > 3 ? ', …' : ''}.` : ''} Conflicts stop the merge so you can resolve or abort it. Running tasks keep going unless the merge changes their files.`,
      confirmLabel: 'Merge',
    });
    if (!ok) return;
    await guarded('Merging…', async () => {
      try {
        const result = await api.post<{ state: string; warnings?: string[] }>('/api/projects/current/git/merge', mergeRequest(workspace, worktree, main));
        if (result?.state === 'conflicts') toast('The merge stopped with conflicts. Resolve them in your editor, or abort.', 'warning');
        else toast(`Merged ${worktree.branch || worktree.path}.`, 'success');
        for (const warning of result?.warnings ?? []) toast(warning, 'warning');
        await refreshAll();
      } catch (err) {
        fail('Merge failed', err);
      }
    });
  }

  async function abortMerge(workspace: GitWorkspace, main: Project) {
    if (!workspace.merge_head) return;
    if (!(await confirm({ title: 'Abort the merge?', body: 'Discards conflict resolutions and restores the branch to its state before the merge.', confirmLabel: 'Abort merge', danger: true }))) return;
    await guarded('Aborting merge…', async () => {
      try {
        await api.post('/api/projects/current/git/abort-merge', { project_id: main.project_id, project_revision: main.project_revision, head: workspace.head, branch: workspace.branch, merge_head: workspace.merge_head, confirm: true, confirm_execution: true });
        toast('Merge aborted.', 'success');
        await refreshAll();
      } catch (err) {
        fail('Abort failed', err);
      }
    });
  }

  async function explainFile(project: Project, worktreePath: string, file: { path: string; content_hash: string }) {
    const isGo = file.path.endsWith('.go');
    const consent = await confirmRemote(isGo ? 'code' : 'bug', `Explaining ${file.path}`);
    if (!consent.ok) return null;
    return guarded('Explaining…', async () => {
      try {
        if (isGo && file.content_hash) {
          return { kind: 'explanation' as const, value: await api.post<FileExplanation>(withWorktree('/api/projects/current/files/explanation', worktreePath), { project_id: project.project_id, project_revision: project.project_revision, base_file_hash: file.content_hash, target_path: file.path, confirm_remote_provider: consent.remote }) };
        }
        return { kind: 'analysis' as const, value: await api.post<FileAnalysis>(withWorktree('/api/projects/current/files/analysis', worktreePath), { path: file.path, project_revision: project.project_revision, refresh: true, confirm_remote_provider: consent.remote }) };
      } catch (err) {
        fail('Explanation failed', err);
        return null;
      }
    });
  }

  return { guarded, createTask, startAgents, cancelAgents, refine, stopRevision, markViewed, runChecks, addComment, deleteComment, approve, overrideReview, reject, apply, undo, setTrust, reindex, openProject, triageFinding, deleteTasks, saveSetup, saveTimeouts, skipSetup, testSetupModel, saveAnalysisSelection, runAnalysis, controlAnalysis, controlAgentSettings, previewMerge, merge, abortMerge, explainFile };
}

export interface FileExplanation {
  summary?: string;
  behavior?: string[];
  inputs?: string[];
  outputs?: string[];
  error_behavior?: string[];
}

export interface FileAnalysis {
  status?: string;
  purpose?: string;
  responsibilities?: string[];
}
