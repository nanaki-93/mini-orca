import { useState } from 'react';
import { workspace as w, canAcceptChange, activeChangeWorkflow, type State } from './workspace';
import { WorkflowProgress, defaultWorkflowModels } from './change-workflow';
import {
  Badge,
  Button,
  Disclosure,
  Empty,
  Go,
  Heading,
  Notice,
  Panel,
  Prose,
  BulletContent,
  human,
} from './ui';
import { FixWorkspace } from './fix-workspace';
import { ChangeOutcome, ChangeChecks, ProposalDiff } from './change-shared';
import type { ChangeSession } from './models';

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
      <Heading
        title={reviewOnly ? 'Changes' : 'Chat'}
        detail={
          reviewOnly
            ? 'Review the captured files and check evidence before applying.'
            : 'Describe a change. Review the generated diff and accept when it is ready.'
        }
        variant="intro"
      >
        <Go page="editor" icon="code">
          Go to file
        </Go>
        <Go page="models" icon="layers">
          Manage models
        </Go>
        <Go page={reviewOnly ? 'chat' : 'changes'} icon={reviewOnly ? 'chat' : 'branch'}>
          {reviewOnly ? 'Back to conversation' : 'Review changes'}
        </Go>
        <Button
          disabled={!!s.busy || running}
          onClick={() => {
            w.newChange();
            void w.navigate('chat');
          }}
        >
          New conversation
        </Button>
      </Heading>
      <ChangeOutcome s={s} />
      <div
        className={`change-workspace ${reviewOnly ? 'change-workspace--review' : 'change-workspace--compose'}`}
      >
        {!reviewOnly && (
          <section className="workspace-page chat-conversation" aria-label="Change conversation">
            {!change ? (
              <Panel title="Task and file scope">
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
              </Panel>
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
            <Panel title="Describe the change" className="chat-composer">
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
                  : 'Generation and checks run automatically. Include a _test.go path to use separate testing and review agents.'}
              </p>
              <div className="actions section-gap">
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
        {reviewOnly && (
          <section className="workspace-page chat-review" aria-label="Proposal review">
            <WorkflowProgress s={s} />
            {change?.changes.length ? (
              <>
                <div className="review-overview row between wrap">
                  <div>
                    <h2>Proposal diff</h2>
                    <p className="small muted">
                      Revision {change.revision} · Review each file before accepting.
                    </p>
                  </div>
                  <Badge
                    value={`${change.changes.length} ${change.changes.length === 1 ? 'file' : 'files'}`}
                  />
                </div>
                <ChangeFiles change={change} />
                <ChangeChecks s={s} tests={tests} />
                <Panel title="Accept this change" className="change-acceptance">
                  <p className="small muted">
                    Accept applies revision {change.revision} to the {change.changes.length}{' '}
                    displayed {change.changes.length === 1 ? 'file' : 'files'}. Review each diff
                    above.
                  </p>
                  <Button
                    tone="primary"
                    icon="check"
                    disabled={!!s.busy || !canAcceptChange(s)}
                    onClick={() => void w.acceptChange()}
                  >
                    Accept changes
                  </Button>
                  {!canAcceptChange(s) && (
                    <p className="small muted">
                      Current passing checks and an approved agent review, when used, are required.
                    </p>
                  )}
                </Panel>
                <Panel className="chat-provider-context">
                  <Disclosure title="Provider context">
                    <p>
                      {change.context_manifest?.model} · {change.context_manifest?.provider_origin}
                    </p>
                    <BulletContent
                      title="Included files and guides"
                      items={change.context_manifest?.included?.map((file) => file.path)}
                    />
                  </Disclosure>
                </Panel>
              </>
            ) : (
              <Panel>
                <Empty title="No proposal yet" icon="code" />
              </Panel>
            )}
          </section>
        )}
      </div>
    </div>
  );
}

function ChangeFiles({ change }: { change: ChangeSession }) {
  const [selected, setSelected] = useState('');
  const files = change.changes.map((edit) => edit.path);
  const path = files.includes(selected) ? selected : files[0] || '';
  const edit = change.changes.find((file) => file.path === path);
  return (
    <Panel title="Files to change" className="change-files">
      <label className="block">
        Select a file to review
        <select
          className="field"
          aria-label="Files to change"
          value={path}
          onChange={(event) => setSelected(event.target.value)}
        >
          {!files.length && <option value="">No captured files</option>}
          {files.map((file) => (
            <option key={file} value={file}>
              {file}
            </option>
          ))}
        </select>
      </label>
      {edit && <ProposalDiff edit={edit} index={files.indexOf(path)} total={files.length} />}
    </Panel>
  );
}
