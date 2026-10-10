import type { AnalysisPreview } from './models';
import { type State, type AnalysisAssignments } from './workspace';
import { Badge, Icon } from './ui';
import { AgentModel, ModelIcon } from './agent-model';

const operations = [
  {
    key: 'code',
    label: 'Bug analysis',
    inputLabel: 'Bug analysis model',
    accent: 'bugs',
    icon: 'bug',
    stages: ['semantic'],
  },
  {
    key: 'performance',
    label: 'Performance',
    inputLabel: 'Performance model',
    accent: 'performance',
    icon: 'gauge',
    stages: ['performance'],
  },
  {
    key: 'security',
    label: 'Security',
    inputLabel: 'Security model',
    accent: 'security',
    icon: 'shield',
    stages: ['security_ai'],
  },
  {
    key: 'features',
    label: 'Feature discovery',
    inputLabel: 'Feature discovery model',
    accent: 'features',
    icon: 'sparkles',
    stages: ['feature_suggestions'],
  },
] as const;

export function ModelSelectors({
  s,
  value,
  disabled,
  onChange,
}: {
  s: State;
  value: AnalysisAssignments;
  disabled: boolean;
  onChange: (models: AnalysisAssignments) => void;
}) {
  return (
    <div className="analysis-model-selectors">
      {operations.map((operation) => (
        <AgentModel
          key={operation.key}
          s={s}
          type="analysis"
          operation={operation}
          value={value[operation.key]}
          disabled={disabled}
          onChange={(id) => onChange({ ...value, [operation.key]: id })}
        />
      ))}
    </div>
  );
}

export function AnalysisModelSummary({ s, value }: { s: State; value: AnalysisAssignments }) {
  return (
    <div className="analysis-model-summary">
      {operations.map(({ key, label, icon }) => {
        const model =
          s.availableModels?.models.find((choice) => choice.id === value[key])?.model ||
          s.models?.scopes[value[key]];
        return (
          <div className="analysis-operation" key={key}>
            <Icon name={icon} />
            <div>
              <strong>{label}</strong>
              <p className="small muted">
                {
                  {
                    code: 'Find correctness and error-handling issues.',
                    performance: 'Identify opportunities to measure.',
                    security: 'Inspect security-sensitive paths.',
                    features: 'Discover ideas from your project goals.',
                  }[key]
                }
              </p>
              <span className="analysis-assigned-model">{model?.model || 'Model unavailable'}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function CapturedModels({
  plan,
  compact = false,
}: {
  plan?: AnalysisPreview;
  compact?: boolean;
}) {
  if (!plan?.models && !plan?.providers?.length)
    return <p className="small muted">Models unavailable</p>;
  return (
    <div className={`grid captured-models${compact ? ' captured-models--compact' : ''}`}>
      {operations.map(({ key, label, stages }) => {
        const spec = plan?.providers?.find((provider) =>
          provider.stages.some((stage) => (stages as readonly string[]).includes(stage)),
        )?.model;
        return (
          <div className="captured-model" key={key} title={compact ? label : undefined}>
            <strong className="block">{label}</strong>
            <div className="captured-model-identity">
              <ModelIcon model={spec} />
              <span className="captured-model-name">{spec?.model || 'Unavailable'}</span>
              {!compact && spec && <Badge value={spec.remote_provider ? 'Remote' : 'Local'} />}
            </div>
            {!compact && spec && <div className="small muted">{spec.provider_origin}</div>}
            {!compact && spec?.reasoning_effort && (
              <div className="small muted">Thinking: {spec.reasoning_effort}</div>
            )}
          </div>
        );
      })}
    </div>
  );
}
