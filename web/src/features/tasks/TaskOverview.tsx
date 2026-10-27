// The task's conversation: what was asked, what the agents did stage by
// stage, what reviewers said, and a box to ask for a revision.

import { useState } from 'react';
import { Bot, CornerDownLeft, FileText, Gavel, ListChecks, MessageSquare, ShieldAlert, Square, User } from 'lucide-react';
import type { LoadedSession } from '@/api/types';
import { useActions, useBusy } from '@/app/actions';
import { openComments, proposalReadiness, workflowActive } from '@/lib/readiness';
import { activeOverride, stageStatusLabel, workflowSpecialists, workflowStages, type SpecialistView, type StageView } from '@/lib/workflow';
import { formatDuration, plural, timeAgo } from '@/lib/util';
import { costCaveat, costLabel, formatTokens, usageLabel } from '@/lib/usage';
import { Badge, Button, Callout, PhaseIcon, Textarea, cx } from '@/components/ui';
import { FileIcon } from '@/components/domain';
import { navigate } from '@/lib/router';

export function TaskOverview({ session }: { session: LoadedSession }) {
  return (
    <div className="page-inner stack-lg">
      <Brief session={session} />
      {session.workflow ? <AgentRun session={session} /> : null}
      <Conversation session={session} />
      <Revise session={session} />
    </div>
  );
}

function Brief({ session }: { session: LoadedSession }) {
  const plan = session.plan;
  return (
    <section className="card" aria-label="Task brief">
      <div className="card-head">
        <FileText size={15} aria-hidden />
        <span className="strong">Brief</span>
        <span className="spacer" />
        <span className="xs faint">updated {timeAgo(session.updated_at)}</span>
      </div>
      <div className="card-body stack">
        {plan?.approach ? <p className="prose">{plan.approach}</p> : <p className="muted small">No description. The agents work from the title, scope and criteria.</p>}
        {plan?.assumptions?.length ? (
          <div className="small">
            <span className="muted">Assumptions: </span>
            {plan.assumptions.join(' · ')}
          </div>
        ) : null}
        {session.acceptance_criteria?.length ? (
          <div>
            <div className="label-row">
              <ListChecks size={13} aria-hidden /> Done when
            </div>
            <ol className="criteria-read">
              {session.acceptance_criteria.map((c, i) => (
                <li key={i}>{c}</li>
              ))}
            </ol>
          </div>
        ) : null}
        <div>
          <div className="label-row">Scope · files agents may change</div>
          <div className="chips">
            {(session.targets ?? []).map(t => (
              <button key={t.path} type="button" className="chip chip-file" onClick={() => navigate('/code', { file: t.path, wt: session.checkout.worktreePath || undefined })} title={`Open ${t.path}`}>
                <FileIcon path={t.path} size={12} />
                {t.path}
                {!t.exists ? <span className="faint">new</span> : null}
              </button>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function AgentRun({ session }: { session: LoadedSession }) {
  const workflow = session.workflow!;
  const stages = workflowStages(workflow);
  const specialists = workflowSpecialists(workflow);
  const active = workflowActive(session);
  const actions = useActions();
  const busy = useBusy();
  const ended = ['failed', 'canceled', 'stale', 'outdated', 'interrupted'].includes(workflow.status);
  const override = activeOverride(workflow, session.hash);
  return (
    <section className="card" aria-label="Agent run">
      <div className="card-head">
        <Bot size={15} aria-hidden />
        <span className="strong">Agent run</span>
        {workflow.attempt && workflow.attempt > 1 ? <Badge>attempt {workflow.attempt}</Badge> : null}
        {workflow.auto_repair ? <Badge tone="neutral">auto-repair {workflow.auto_repair_attempts ?? 0}/2</Badge> : null}
        <span className="spacer" />
        {active ? (
          <Button size="sm" variant="ghost" icon={Square} onClick={() => void actions.cancelAgents(session)} disabled={workflow.status === 'canceling' || Boolean(busy)}>
            {workflow.status === 'canceling' ? 'Stopping…' : 'Stop'}
          </Button>
        ) : null}
      </div>
      {ended && workflow.reason ? (
        <div className="card-body">
          <Callout tone={workflow.status === 'canceled' ? 'neutral' : 'danger'} icon={ShieldAlert} title={workflow.status === 'canceled' ? 'Stopped' : 'The run did not finish'}>
            {workflow.reason}
          </Callout>
        </div>
      ) : null}
      {override ? (
        <div className="card-body">
          <Callout tone="info" icon={Gavel} title={`You overrode the reviewers ${timeAgo(override.at)}`}>
            {override.reason}
          </Callout>
        </div>
      ) : null}
      <ol className="timeline">
        {stages.map(stage => (
          <Stage key={stage.name} stage={stage} specialists={stage.name === 'review' ? specialists : []} />
        ))}
      </ol>
    </section>
  );
}

function Stage({ stage, specialists }: { stage: StageView; specialists: SpecialistView[] }) {
  return (
    <li className={cx('tl-item', `tl-${stage.tone}`)}>
      <div className="tl-marker">
        <PhaseIcon tone={stage.tone} size={16} label={stage.status} />
      </div>
      <div className="tl-content">
        <div className="row">
          <span className="strong">{stage.label}</span>
          <span className="xs muted">{stage.actor}</span>
          <span className="spacer" />
          {stage.model ? <span className="xs faint mono">{stage.model}</span> : null}
          {stage.usage?.model_calls ? (
            <span className="xs faint nowrap" title={`${usageLabel(stage.usage)}${costCaveat(stage.cost) ? ` — ${costCaveat(stage.cost)}` : ''}`}>
              {[`${formatTokens(stage.usage.input_tokens + stage.usage.output_tokens)} tokens`, costLabel(stage.cost)].filter(Boolean).join(' · ')}
            </span>
          ) : null}
          <span className="xs faint nowrap">{stage.durationMs ? formatDuration(stage.durationMs) : stageStatusLabel(stage)}</span>
        </div>
        {stage.reason ? <p className="small muted">{stage.reason}</p> : null}
        {specialists.length ? (
          <div className="reviewers">
            {specialists.map(sp => (
              <Reviewer key={sp.id} reviewer={sp} />
            ))}
          </div>
        ) : null}
      </div>
    </li>
  );
}

function Reviewer({ reviewer }: { reviewer: SpecialistView }) {
  const [open, setOpen] = useState(reviewer.verdict === 'changes_requested');
  const verdict = reviewer.verdict === 'approve' ? { label: 'Approved', tone: 'success' as const } : reviewer.verdict === 'changes_requested' ? { label: 'Changes requested', tone: 'warning' as const } : { label: reviewer.status, tone: 'neutral' as const };
  return (
    <div className={cx('reviewer', open && 'is-open')}>
      <button type="button" className="reviewer-head" onClick={() => setOpen(!open)} aria-expanded={open}>
        <PhaseIcon tone={reviewer.tone} size={14} />
        <span className="strong small">{reviewer.label}</span>
        <Badge tone={verdict.tone}>{verdict.label}</Badge>
        {reviewer.findings.length ? <span className="xs muted">{plural(reviewer.findings.length, 'blocking finding')}</span> : null}
        {reviewer.notes.length ? <span className="xs faint">{plural(reviewer.notes.length, 'note')}</span> : null}
        <span className="spacer" />
        <span className="xs faint">{reviewer.evidence === 'unmeasured' ? 'unmeasured advice' : 'model advice'}</span>
      </button>
      {open ? (
        <div className="reviewer-body small">
          {reviewer.summary ? <p>{reviewer.summary}</p> : null}
          {reviewer.reason ? <p className="muted">{reviewer.reason}</p> : null}
          {reviewer.findings.length ? (
            <ul className="findings">
              {reviewer.findings.map((f, i) => (
                <li key={i}>{f}</li>
              ))}
            </ul>
          ) : null}
          {reviewer.notes.length ? (
            <>
              <div className="label-row">Advisory</div>
              <ul className="findings findings-advisory">
                {reviewer.notes.map((note, i) => (
                  <li key={i}>{note}</li>
                ))}
              </ul>
            </>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function Conversation({ session }: { session: LoadedSession }) {
  const messages = (session.messages ?? []).filter(m => m.content?.trim());
  if (!messages.length) return null;
  return (
    <section className="card" aria-label="Conversation">
      <div className="card-head">
        <MessageSquare size={15} aria-hidden />
        <span className="strong">Conversation</span>
        <span className="xs faint">{messages.length}</span>
      </div>
      <ol className="messages">
        {messages.map((m, i) => (
          <li key={i} className={cx('message', m.role === 'user' ? 'from-user' : 'from-agent')}>
            <span className="message-avatar" aria-hidden>
              {m.role === 'user' ? <User size={13} /> : <Bot size={13} />}
            </span>
            <div className="message-body">
              <div className="xs muted">{m.role === 'user' ? 'You' : 'Agent'}</div>
              <div className="prose small">{m.content}</div>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Revise({ session }: { session: LoadedSession }) {
  const [text, setText] = useState('');
  const [withComments, setWithComments] = useState(true);
  const actions = useActions();
  const busy = useBusy();
  const ready = proposalReadiness(session);
  const comments = openComments(session).length;
  if (!ready.editable || workflowActive(session)) return null;
  const sendComments = comments > 0 && withComments;
  const submit = async () => {
    if (await actions.refine(session, text, sendComments)) setText('');
  };
  return (
    <section className="revise" aria-label="Ask for a revision">
      <Textarea
        rows={3}
        placeholder={(session.changes ?? []).length ? 'Ask the agent to change something in this proposal…' : 'Describe the change to generate it without the full agent run…'}
        value={text}
        onChange={e => setText(e.target.value)}
        onKeyDown={e => {
          if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
            e.preventDefault();
            void submit();
          }
        }}
        aria-label="Revision request"
      />
      {comments ? (
        <label className="check-label small">
          <input type="checkbox" checked={withComments} onChange={e => setWithComments(e.target.checked)} />
          Include my {plural(comments, 'line comment')}
        </label>
      ) : null}
      <div className="row">
        <span className="xs faint">A revision creates a new version; earlier review marks and checks no longer apply.</span>
        <span className="spacer" />
        <Button variant="secondary" icon={CornerDownLeft} onClick={() => void submit()} disabled={(!text.trim() && !sendComments) || Boolean(busy)} loading={busy === 'Generating…'} kbd="mod+Enter">
          {(session.changes ?? []).length ? 'Revise' : 'Generate'}
        </Button>
      </div>
    </section>
  );
}
