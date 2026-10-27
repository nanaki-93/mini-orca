// User actions against the daemon. Every mutation runs at most once at a time,
// carries the captured project/proposal identity, asks for explicit consent
// where the daemon requires `confirm`, and refreshes what it changed.

import { useSyncExternalStore } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, errorMessage, query, withWorktree } from '@/api/client';
import type { AgentSettings, ChangeHistoryEntry, ChangeMutationResult, ChangeSession, ExecutionTrust, Finding, GitWorkspace, GitWorktree, LoadedSession, MergePreview, ModelsCurrent, Project } from '@/api/types';
import { useFeedback } from '@/components/overlays';
import { canOverrideReview, changeIdentity, proposalReadiness, workflowActive } from '@/lib/readiness';
import { mergeRequest, workflowDestinations, workflowRequest } from '@/lib/workflow';
import { navigate } from '@/lib/router';
import { plural } from '@/lib/util';
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

const changePath = (session: LoadedSession, suffix: string) => withWorktree('/api/projects/current/changes/' + encodeURIComponent(session.id) + suffix, session.checkout.worktreePath);

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

  /** Running tasks in the same checkout that an Apply, Undo or reindex stops. */
  function runningTasksNote(session: { id: string; checkout: { worktreePath: string } }): string {
    const lists = client.getQueriesData<ChangeHistoryEntry[]>({ queryKey: ['changes'] });
    const running = new Map<string, ChangeHistoryEntry>();
    for (const [key, list] of lists) {
      const checkout = String(key[1] ?? '');
      for (const task of list ?? []) {
        const where = task.worktree_path || checkout;
        if (task.id !== session.id && where === session.checkout.worktreePath && ['running', 'canceling'].includes(task.workflow_status ?? '')) running.set(task.id, task);
      }
    }
    if (!running.size) return '';
    const names = [...running.values()].slice(0, 3).map(task => `“${task.title || task.id}”`).join(', ');
    return ` This stops ${plural(running.size, 'running task')} in this checkout: ${names}${running.size > 3 ? ', …' : ''}.`;
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
    const ok = await confirm({
      title: 'Start the agents?',
      body: (
        <div className="stack">
          <p>Agents write the change, write and run tests in an isolated copy of the project, then specialist reviewers check it. Nothing reaches your files until you review and apply it.</p>
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
        </div>
      ),
      confirmLabel: 'Start agents',
    });
    if (!ok) return false;
    return Boolean(
      await guarded('Starting agents…', async () => {
        try {
          const body = workflowRequest(changeIdentity(session), message, choice, { autoRepair: options.autoRepair, confirmSecurity: security, planHash: session.plan?.hash });
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

  async function refine(session: LoadedSession, message: string): Promise<boolean> {
    const text = message.trim();
    if (!text) {
      toast('Describe what should change first.', 'warning');
      return false;
    }
    const consent = await confirmRemote('code', `Revising “${session.title || session.id}”`);
    if (!consent.ok) return false;
    return Boolean(
      await guarded('Generating…', async () => {
        try {
          setSession(await api.post<ChangeSession>(changePath(session, '/messages'), { ...changeIdentity(session), message: text, profile: 'code', confirm_remote_provider: consent.remote }), session);
          toast('Proposal updated. Review the new diff.', 'success');
          return true;
        } catch (err) {
          fail('Generation failed', err);
          return false;
        }
      }),
    );
  }

  // ---------------------------------------------------------------- review

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
      body: `Writes “${session.title || session.id}” to ${destination}${commit ? ` and commits it on ${session.git?.branch || 'the current branch'}` : ''}. Undo stays available for the latest Apply while the files are unchanged${commit ? '; it restores the files but keeps the commit' : ''}.${runningTasksNote(session)}`,
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
      body: `Restores the files changed by “${session.title || session.id}” after verifying they still match the applied version.${session.commit?.state === 'committed' ? ' The commit stays in history.' : ''}${runningTasksNote(session)}`,
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

  async function runAnalysis(project: Project, worktreePath: string) {
    await guarded('Preparing analysis…', async () => {
      try {
        const limits = { batch_files: 64, budget_seconds: 300, max_attempts_per_stage: 2 };
        const preview = await api.post<{ identity: unknown; preview_id: string; providers?: { id: string; remote_confirmation_required?: boolean; model?: { model?: string; scope?: string; provider_origin?: string } }[]; files?: unknown[]; max_model_requests?: number; security_review_intent_required?: boolean }>(
          withWorktree('/api/projects/current/analysis/preview', worktreePath),
          { project_id: project.project_id, project_revision: project.project_revision, scope: 'project', refresh: false, limits },
        );
        const providers = preview.providers ?? [];
        const remote = providers.filter(p => p.remote_confirmation_required).map(p => `${p.model?.model || p.model?.scope} at ${p.model?.provider_origin || 'remote provider'}`);
        const security = Boolean(preview.security_review_intent_required);
        const ok = await confirm({
          title: 'Analyze the project?',
          body: `Analyzes ${plural((preview.files ?? []).length, 'file')} (up to ${preview.max_model_requests ?? 0} model requests) for bugs, performance and security, then refreshes the generated docs.${security ? ' Includes an AI security review.' : ''}${remote.length ? ` Sends bounded source context to ${remote.join(', ')}.` : ''}`,
          confirmLabel: 'Analyze',
        });
        if (!ok) return;
        await api.post(withWorktree('/api/projects/current/analysis/run', worktreePath), { identity: preview.identity, preview_id: preview.preview_id, limits, refresh: false, confirmations: { provider_ids: providers.map(p => p.id).filter(Boolean), security_review: security } });
        toast('Analysis started.', 'info');
        void client.invalidateQueries({ queryKey: ['overview'] });
      } catch (err) {
        fail('Analysis could not start', err);
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
      body: `Merges ${worktree.head.slice(0, 7)}, marks its linked issues done, then removes the worktree and its branch.${overlap.length ? ` ${plural(overlap.length, 'path')} changed on both branches: ${overlap.slice(0, 3).join(', ')}${overlap.length > 3 ? ', …' : ''}.` : ''} Conflicts stop the merge so you can resolve or abort it.${runningTasksNote({ id: '', checkout: { worktreePath: '' } })}`,
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

  return { guarded, createTask, startAgents, cancelAgents, refine, markViewed, runChecks, approve, overrideReview, reject, apply, undo, setTrust, reindex, openProject, triageFinding, runAnalysis, previewMerge, merge, abortMerge, explainFile };
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
