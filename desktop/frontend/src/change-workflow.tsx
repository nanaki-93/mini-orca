import { useWorkspace } from './workspace-context';
import { type State } from './workspace';
import type { ChangeWorkflowModels } from './models';
import { AgentModel } from './agent-model';

export function WorkflowModels({
  s,
  value,
  disabled,
}: {
  s: State;
  value: ChangeWorkflowModels;
  disabled: boolean;
}) {
  const w = useWorkspace();
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
