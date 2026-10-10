import { activeChangeWorkflow, workspace as w, type State } from './workspace';
import type { ChangeWorkflowModels } from './models';
import { AgentModel } from './agent-model';
import { Badge, BulletContent, Button, Notice, Panel, Prose, human } from './ui';

export const defaultWorkflowModels: ChangeWorkflowModels = {
  create: 'function',
  test: 'bug',
  review: 'analyze',
};

export function WorkflowModels({
  s,
  value,
  disabled,
}: {
  s: State;
  value: ChangeWorkflowModels;
  disabled: boolean;
}) {
  const roles = ['create', 'test', 'review'] as const;
  return (
    <fieldset className="workflow-models" disabled={disabled}>
      <legend className="sr-only">Models for this workflow</legend>
      <div className="analysis-model-selectors workflow-model-selectors">
        {roles.map((type) => (
          <AgentModel
            key={type}
            s={s}
            type={type}
            value={value[type]}
            disabled={disabled}
            onChange={(model) => w.setWorkflowModels({ ...value, [type]: model })}
          />
        ))}
      </div>
    </fieldset>
  );
}

const stageNames: Record<string, string> = {
  create: 'Create',
  test: 'Write and run tests',
  review: 'Model review',
  human_review: 'Human diff review',
};

export function WorkflowProgress({
  s,
  showOutcome = true,
  actionLabel = 'Accept changes',
  showCancel = true,
}: {
  s: State;
  showOutcome?: boolean;
  actionLabel?: string;
  showCancel?: boolean;
}) {
  const workflow = s.change?.workflow;
  if (!workflow) return null;
  return (
    <Panel
      title="Agent workflow"
      className="workflow-progress"
      actions={
        <Badge
          value={s.change?.state === 'draft' ? workflow.status : s.change?.state || workflow.status}
        />
      }
    >
      <ol className="workflow-stages" aria-label="Workflow stages" aria-live="polite">
        {workflow.stages.map((stage) => (
          <li key={stage.name} aria-current={stage.status === 'running' ? 'step' : undefined}>
            <div className="row between wrap">
              <strong>{stageNames[stage.name] || human(stage.name)}</strong>
              <span className="row small">
                <Badge value={stage.status} />
                {stage.status === 'completed' && <span>Completed</span>}
              </span>
            </div>
            {stage.model && (
              <p className="small muted">
                {stage.model.model} · {stage.model.profile} · {stage.model.provider_origin}
              </p>
            )}
          </li>
        ))}
      </ol>
      {showOutcome && workflow.reason && (
        <Notice error={workflow.status === 'failed'}>{workflow.reason}</Notice>
      )}
      {showOutcome && workflow.review && (
        <section aria-label="Model review report">
          <div className="row wrap">
            <strong>Model verdict</strong>
            <Badge value={workflow.review.verdict} />
          </div>
          <Prose text={workflow.review.summary} />
          <BulletContent title="Requested changes" items={workflow.review.findings} />
          <p className="small muted">
            Model advice. Review the file differences and test evidence before approving.
          </p>
        </section>
      )}
      {workflow.status === 'awaiting_human_review' && s.change?.state === 'draft' && (
        <Notice>Review the file differences, then select {actionLabel}.</Notice>
      )}
      {showCancel && activeChangeWorkflow(s.change) && (
        <div className="actions">
          <Button
            disabled={!!s.busy || workflow.status === 'canceling'}
            onClick={() => void w.cancelChangeWorkflow()}
          >
            Cancel workflow
          </Button>
        </div>
      )}
    </Panel>
  );
}
