import { useEffect, useId, useRef, useState } from 'react';
import type { Model, ModelChoice } from './models';
import { workspace as w, type State } from './workspace';
import { Button, Icon, Notice } from './ui';

export type AgentType = 'create' | 'analysis' | 'review' | 'test';

interface AgentOperation {
  label: string;
  inputLabel: string;
  accent: string;
  icon: string;
}

const agentOperations: Record<AgentType, AgentOperation> = {
  create: { label: 'Create', inputLabel: 'Creation model', accent: 'features', icon: 'sparkles' },
  analysis: { label: 'Analysis', inputLabel: 'Analysis model', accent: 'bugs', icon: 'bug' },
  review: { label: 'Review', inputLabel: 'Review model', accent: 'security', icon: 'shield' },
  test: { label: 'Test', inputLabel: 'Testing model', accent: 'performance', icon: 'circleCheck' },
};

function modelChoices(s: State, configuredOnly: boolean): ModelChoice[] {
  // Change workflows accept configured profiles; analysis also supports catalog IDs.
  return configuredOnly
    ? Object.entries(s.models?.scopes || {}).map(([id, model]) => ({
        id,
        name: model.model,
        provider: model.provider_origin,
        source: 'configured',
        location: model.remote_provider ? 'remote' : 'local',
        model,
      }))
    : s.availableModels?.models || [];
}

export function ModelIcon({ model }: { model?: Model }) {
  // Model family and destination are independent: proxies can serve any family.
  const name = model?.model.toLowerCase() || '';
  const family = /(^|[/\s:._-])(gpt|chatgpt|codex|o[134])([/\s:._-]|\d|$)/.test(name)
    ? 'openai'
    : /(^|[/\s:._-])claude([/\s:._-]|\d|$)/.test(name)
      ? 'claude'
      : /(^|[/\s:._-])gemini([/\s:._-]|\d|$)/.test(name)
        ? 'gemini'
        : 'generic';
  return (
    <span className="analysis-model-icon" data-family={family} aria-hidden="true">
      {family === 'generic' ? (
        <Icon name={model ? 'layers' : 'minus'} />
      ) : (
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6">
          {family === 'openai' &&
            Array.from({ length: 6 }, (_, index) => (
              <path
                key={index}
                transform={`rotate(${index * 60} 12 12)`}
                d="M12 8V3.5a5 5 0 0 1 7.8 4.2v5.1L12 17.3"
                strokeLinejoin="round"
              />
            ))}
          {family === 'claude' &&
            Array.from({ length: 12 }, (_, index) => (
              <path
                key={index}
                transform={`rotate(${index * 30} 12 12)`}
                d={index % 2 ? 'M12 5v7' : 'M12 2v10'}
                strokeLinecap="round"
              />
            ))}
          {family === 'gemini' && (
            <path
              d="M12 2C12 7.5 7.5 12 2 12c5.5 0 10 4.5 10 10 0-5.5 4.5-10 10-10-5.5 0-10-4.5-10-10Z"
              fill="currentColor"
              stroke="none"
            />
          )}
        </svg>
      )}
    </span>
  );
}

function choiceDescription(choice: ModelChoice) {
  const source =
    choice.source === 'pi'
      ? `Pi · ${choice.provider}`
      : choice.provider === 'openai'
        ? 'OpenAI compatible'
        : choice.provider;
  const location =
    choice.location === 'local' ? 'Local' : choice.location === 'remote' ? 'Remote' : 'CLI managed';
  return `${source} · ${location}`;
}

function ModelPicker({
  s,
  label,
  selected,
  onSelect,
  onClose,
  configuredOnly,
}: {
  s: State;
  label: string;
  selected?: string;
  onSelect: (id: string) => void;
  onClose: () => void;
  configuredOnly: boolean;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const search = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('all');
  const [active, setActive] = useState(selected || '');
  const listID = useId();
  const choices = modelChoices(s, configuredOnly).filter(
    (choice) =>
      (filter === 'all' ||
        (filter === 'local' ? choice.location === 'local' : choice.source === filter)) &&
      `${choice.name} ${choice.model.model} ${choice.provider}`
        .toLowerCase()
        .includes(query.toLowerCase().trim()),
  );
  const activeIndex = Math.max(
    0,
    choices.findIndex((choice) => choice.id === active),
  );
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    dialog.current?.showModal();
    search.current?.focus();
    return () => previous?.focus();
  }, []);
  useEffect(() => {
    document.getElementById(`${listID}-${activeIndex}`)?.scrollIntoView({ block: 'nearest' });
  }, [activeIndex, listID, query, filter]);
  const error = configuredOnly
    ? s.resourceErrors.models
    : s.resourceErrors.availableModels || s.availableModels?.pi.message;
  return (
    <dialog
      ref={dialog}
      className="model-picker"
      aria-labelledby={`${listID}-title`}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <div className="model-picker-heading">
        <div>
          <h2 id={`${listID}-title`}>Choose a model</h2>
          <p>Assign a model to {label.toLowerCase()}.</p>
        </div>
        <Button icon="close" aria-label="Close model picker" tone="ghost" onClick={onClose} />
      </div>
      <div className="input-wrap model-picker-search">
        <Icon name="search" />
        <input
          ref={search}
          role="combobox"
          aria-label="Search models"
          aria-expanded="true"
          aria-autocomplete="list"
          aria-controls={listID}
          aria-activedescendant={choices.length ? `${listID}-${activeIndex}` : undefined}
          placeholder="Search by model or provider…"
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setActive('');
          }}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
              event.preventDefault();
              const next =
                (activeIndex + (event.key === 'ArrowDown' ? 1 : -1) + choices.length) %
                choices.length;
              if (choices[next]) setActive(choices[next].id);
            } else if (event.key === 'Enter' && choices[activeIndex]) {
              event.preventDefault();
              onSelect(choices[activeIndex].id);
            }
          }}
        />
      </div>
      <div className="model-picker-filters" role="group" aria-label="Filter model catalog">
        {(
          [
            ['all', 'All models'],
            ...(!configuredOnly ? [['pi', 'Pi']] : []),
            ['local', 'Local'],
            ...(!configuredOnly ? [['configured', 'Configured']] : []),
          ] as const
        ).map(([key, name]) => (
          <Button
            key={key}
            tone="small"
            aria-pressed={filter === key}
            onClick={() => {
              setFilter(key);
              setActive('');
            }}
          >
            {name}
          </Button>
        ))}
      </div>
      {error && (
        <Notice error={configuredOnly || !!s.resourceErrors.availableModels}>{error}</Notice>
      )}
      <div
        className="model-picker-list"
        role="listbox"
        id={listID}
        aria-label="Available models"
        aria-busy={!!s.modelsLoading}
      >
        {choices.map((choice, index) => (
          <button
            key={choice.id}
            type="button"
            role="option"
            value={choice.id}
            tabIndex={-1}
            id={`${listID}-${index}`}
            aria-selected={choice.id === selected}
            className="model-picker-option"
            data-active={activeIndex === index}
            onClick={() => onSelect(choice.id)}
          >
            <ModelIcon model={choice.model} />
            <span className="model-picker-option-copy">
              <strong>{choice.name}</strong>
              <span>
                {configuredOnly ? `${choice.model.profile} · ` : ''}
                {choiceDescription(choice)}
                {choice.model.reasoning_effort
                  ? ` · Thinking: ${choice.model.reasoning_effort}`
                  : ''}
              </span>
              {choice.name !== choice.model.model && <small>{choice.model.model}</small>}
            </span>
            {choice.id === selected && <Icon name="check" />}
          </button>
        ))}
      </div>
      {!choices.length && (
        <p className="model-picker-empty" role="status">
          {s.modelsLoading
            ? 'Loading models…'
            : query || filter !== 'all'
              ? 'No models match these filters.'
              : 'No models available. Refresh the catalog to try again.'}
        </p>
      )}
      <div className="model-picker-footer">
        <span role="status">
          {s.modelsLoading ? 'Refreshing models…' : `${choices.length} models`}
        </span>
        <Button
          icon="refresh"
          tone="small"
          disabled={s.modelsLoading}
          onClick={() => void w.loadAvailableModels()}
        >
          Refresh models
        </Button>
      </div>
    </dialog>
  );
}

export function AgentModel({
  s,
  type,
  value,
  disabled,
  onChange,
  operation = agentOperations[type],
}: {
  s: State;
  type: AgentType;
  value: string;
  disabled: boolean;
  onChange: (model: string) => void;
  operation?: AgentOperation;
}) {
  const [editing, setEditing] = useState(false);
  const { label, inputLabel, accent, icon } = operation;
  const configuredOnly = type !== 'analysis';
  const selected = modelChoices(s, configuredOnly).find((choice) => choice.id === value);
  const model = selected?.model || s.models?.scopes[value];
  return (
    <div className="analysis-model-card" data-accent={accent} data-agent-type={type}>
      <div className="analysis-operation">
        <span className="analysis-operation-icons">
          <Icon name={icon} />
        </span>
        <strong>{label}</strong>
      </div>
      <button
        type="button"
        className="analysis-model-trigger"
        value={value}
        aria-label={inputLabel}
        title={`${label}: ${selected?.name || model?.model || 'Model unavailable'}`}
        aria-description={
          selected
            ? `${selected.name} · ${choiceDescription(selected)}`
            : model?.model || 'Model unavailable'
        }
        aria-haspopup="dialog"
        disabled={disabled}
        onClick={() => setEditing(true)}
      >
        <ModelIcon model={model} />
        <span className="analysis-model-choice">
          <strong>
            {selected?.name ||
              model?.model ||
              (s.availableModels || configuredOnly ? 'Model unavailable' : 'Choose a model')}
          </strong>
          <small>
            {selected
              ? choiceDescription(selected)
              : s.modelsLoading
                ? 'Loading model catalog…'
                : 'Browse available models'}
          </small>
        </span>
      </button>
      {editing && !disabled && (
        <ModelPicker
          s={s}
          label={label}
          selected={value}
          configuredOnly={configuredOnly}
          onClose={() => setEditing(false)}
          onSelect={(id) => {
            onChange(id);
            setEditing(false);
          }}
        />
      )}
    </div>
  );
}
