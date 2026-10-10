import { workspace as w, canAcceptChange, activeChangeWorkflow, type State } from './workspace';
import { WorkflowProgress, defaultWorkflowModels } from './change-workflow';
import {
  Badge,
  Button,
  Disclosure,
  Empty,
  Go,
  Notice,
  Panel,
  Prose,
  BulletContent,
  human,
} from './ui';
import { FixWorkspace, ProposalReview } from './fix-workspace';
import { ProjectGuidance } from './project-guidance';
import { ChangeOutcome } from './change-shared';

export function ChangeWorkspace({ s }: { s: State }) {
  const kind = s.change?.kind || s.changeSeed?.kind;
  return s.page === 'changes' && ['fix', 'performance', 'security'].includes(kind || '') ? (
    <FixWorkspace s={s} />
  ) : (
    <ChatWorkspace s={s} />
  );
}

function ChatWorkspace({ s }: { s: State }) {
  const change = s.change;
  const reviewOnly = s.page === 'changes';
  const draft =
    s.chatDraft && s.chatDraft.sessionID === change?.id && s.chatDraft.seed === s.changeSeed
      ? s.chatDraft
      : {
          sessionID: change?.id,
          seed: s.changeSeed,
          title: change?.title || s.changeSeed?.title || 'New feature',
          paths: (change?.targets.map((target) => target.path) || s.changeSeed?.paths || []).join(
            '\n',
          ),
          message: change ? '' : s.changeSeed?.message || '',
          tests: change?.kind !== 'instructions',
        };
  const { title, paths, message, tests } = draft;
  const update = (fields: Partial<typeof draft>) => w.setChatDraft({ ...draft, ...fields });
  const taskKind = change?.kind || s.changeSeed?.kind || 'feature';
  const seed = {
    title,
    paths: paths
      .split('\n')
      .map((path) => path.trim())
      .filter(Boolean),
    message,
    kind: taskKind,
    acceptance_criteria: change?.acceptance_criteria || s.changeSeed?.acceptance_criteria || [],
  };
  const running = activeChangeWorkflow(change);
  const models = s.workflowModels || change?.workflow?.models || defaultWorkflowModels;
  const useAgents =
    taskKind !== 'instructions' &&
    (!!change?.workflow || seed.paths.some((path) => path.endsWith('_test.go')));
  const blocked =
    !!s.busy ||
    running ||
    (!!change && (change.state !== 'draft' || change.freshness !== 'current'));
  return (
    <div className="chat-page">
      <header className="chat-meta">
        <div>
          <span className="studio-eyebrow">
            {reviewOnly ? 'Proposal review' : 'Task conversation'}
          </span>
          <h1>{change?.title || s.changeSeed?.title || (reviewOnly ? 'Changes' : 'Chat')}</h1>
        </div>
        <div className="actions">
          <Go page="models" tone="ghost">
            Manage models
          </Go>
          <Go page={reviewOnly ? 'chat' : 'changes'} tone="ghost">
            {reviewOnly ? 'Back to conversation' : 'Review changes'}
          </Go>
          <Button
            tone="ghost"
            icon="plus"
            disabled={!!s.busy || running}
            onClick={() => {
              w.newChange();
              void w.navigate('chat');
            }}
          >
            New conversation
          </Button>
        </div>
      </header>
      {!reviewOnly && <ProjectGuidance s={s} />}
      <ChangeOutcome s={s} />
      <div
        className={`change-workspace ${reviewOnly ? 'change-workspace--review' : 'change-workspace--compose'}`}
      >
        {!reviewOnly && (
          <section className="workspace-page chat-conversation" aria-label="Change conversation">
            <div className="chat-messages">
              {!change ? (
                <Disclosure title={`Task scope · ${seed.paths.length} files`} open>
                  <div className="chat-scope-fields">
                    <label className="block">
                      Task title
                      <input
                        value={title}
                        onChange={(e) => update({ title: e.target.value })}
                        disabled={!!s.busy}
                        maxLength={200}
                      />
                    </label>
                  </div>
                  <div className="chat-scope-files">
                    <label className="block">
                      Files to change (one path per line)
                      <textarea
                        aria-label="Files to change"
                        className="chat-path-input"
                        value={paths}
                        onChange={(e) => update({ paths: e.target.value })}
                        disabled={!!s.busy}
                        placeholder="internal/worker/process.go&#10;internal/worker/process_test.go"
                      />
                    </label>
                    <label className="block">
                      Add an existing file
                      <select
                        className="field"
                        aria-label="Add an existing file"
                        value=""
                        disabled={!!s.busy}
                        onChange={(e) => {
                          if (e.target.value && !paths.split('\n').includes(e.target.value))
                            update({ paths: [paths, e.target.value].filter(Boolean).join('\n') });
                        }}
                      >
                        <option value="">Choose a file…</option>
                        {s.index?.files
                          .filter((file) => !file.binary && /\.(go|md)$/.test(file.path))
                          .map((file) => (
                            <option key={file.path}>{file.path}</option>
                          ))}
                      </select>
                    </label>
                  </div>
                  <p className="small muted">Up to 8 Go/Markdown files, including new files.</p>
                </Disclosure>
              ) : (
                <Panel
                  className="chat-task"
                  title={change.title}
                  actions={<Badge value={change.freshness === 'stale' ? 'stale' : change.state} />}
                >
                  <p className="small muted">
                    Revision {change.revision} · {change.targets.length} captured paths
                  </p>
                  <div className="chat-file-scope">
                    <BulletContent
                      title="File scope"
                      items={change.targets.map(
                        (target) => `${target.path}${target.exists ? '' : ' · new file'}`,
                      )}
                    />
                  </div>
                  <BulletContent title="Acceptance criteria" items={change.acceptance_criteria} />
                  {change.freshness === 'stale' && (
                    <Notice>Source or guidance changed. Start a new conversation.</Notice>
                  )}
                </Panel>
              )}
              {change?.messages.map((entry, i) => (
                <Panel
                  key={i}
                  title={entry.role === 'user' ? 'You' : 'Assistant'}
                  className="chat-message"
                >
                  <Prose text={entry.content} />
                </Panel>
              ))}
              {change && (
                <WorkflowProgress
                  s={s}
                  showOutcome={change.workflow?.status !== 'awaiting_human_review'}
                />
              )}
              {change?.changes.length ? (
                <Panel
                  className="chat-proposal"
                  title={
                    change.state !== 'draft'
                      ? `Change ${human(change.state)}`
                      : change.freshness === 'stale'
                        ? 'Proposal outdated'
                        : canAcceptChange(s)
                          ? 'Ready for review'
                          : 'Proposal needs attention'
                  }
                >
                  <div className="row between wrap">
                    <div>
                      <strong>
                        {change.changes.length} {change.changes.length === 1 ? 'file' : 'files'}{' '}
                        changed
                      </strong>
                      <p className="small muted">Review the diff and check evidence in Changes.</p>
                    </div>
                    <Go page="changes" icon="arrow" tone="primary">
                      Open proposal
                    </Go>
                  </div>
                </Panel>
              ) : null}
            </div>
            <Panel className="chat-composer">
              <h2 className="sr-only">Describe the change</h2>
              <label className="sr-only" htmlFor="chat-request">
                Change request
              </label>
              <textarea
                id="chat-request"
                className="composer"
                value={message}
                onChange={(e) => update({ message: e.target.value })}
                disabled={blocked}
                placeholder="Describe a new feature, fix, or improvement…"
              />
              {!useAgents && (
                <label className="checkbox-line">
                  <input
                    type="checkbox"
                    checked={tests}
                    disabled={blocked || change?.kind === 'instructions'}
                    onChange={(e) => update({ tests: e.target.checked })}
                  />
                  Run project tests after generation
                </label>
              )}
              <p className="small muted">
                {useAgents
                  ? 'Creation, tests and agent review run automatically.'
                  : 'Generation and checks use the models configured for this workspace.'}
              </p>
              <div className="actions section-gap composer-footer">
                <span className="small muted">{seed.paths.length} files · review before Apply</span>
                <Button
                  tone="primary"
                  icon="sparkles"
                  disabled={
                    blocked ||
                    !message.trim() ||
                    !paths.trim() ||
                    !title.trim() ||
                    (useAgents && !s.models)
                  }
                  onClick={() =>
                    void (useAgents
                      ? w.startChangeWorkflow(seed, models)
                      : w.prepareChange(seed, tests, models.create))
                  }
                >
                  Generate changes
                </Button>
              </div>
            </Panel>
          </section>
        )}
        {reviewOnly &&
          (change?.changes.length ? (
            <ProposalReview
              key={`${change.project_id}:${change.project_revision}:${change.id}:${change.revision}:${change.hash}`}
              s={s}
              change={change}
              tests={tests}
              details={
                <>
                  <ProjectGuidance s={s} />
                  <BulletContent
                    title="Captured files"
                    items={change.targets.map((target) => target.path)}
                  />
                  <BulletContent title="Acceptance criteria" items={change.acceptance_criteria} />
                  <Panel className="chat-provider-context">
                    <Disclosure title="Provider context">
                      <p>
                        {change.context_manifest?.model} ·{' '}
                        {change.context_manifest?.provider_origin}
                      </p>
                      <BulletContent
                        title="Included files and guides"
                        items={change.context_manifest?.included?.map((file) => file.path)}
                      />
                    </Disclosure>
                  </Panel>
                </>
              }
            />
          ) : (
            <Empty title="No proposal yet" icon="code">
              <Go page="chat">Start in Chat</Go>
            </Empty>
          ))}
      </div>
    </div>
  );
}
