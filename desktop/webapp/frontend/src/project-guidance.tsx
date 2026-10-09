import { workspace as w, type State } from './workspace';
import { Button, Icon } from './ui';

export function ProjectGuidance({ s }: { s: State }) {
  if (
    !s.project ||
    s.page !== 'chat' ||
    !['fix', 'performance', 'security', 'feature'].includes(
      s.change?.kind || s.changeSeed?.kind || 'feature',
    )
  )
    return null;
  const guide = s.projectInstructions;
  const error = s.resourceErrors['default instructions'];
  const excluded = guide?.effective.excluded.find((file) => file.path === 'AGENTS.md');
  const active = guide?.effective.files.some((file) => file.path === 'AGENTS.md');
  return (
    <aside className="project-guidance" aria-label="Default agent instructions">
      <Icon name="file" />
      <div>
        <strong>
          {error
            ? 'Instructions unavailable'
            : !guide
              ? 'Checking project instructions…'
              : active
                ? 'AGENTS.md · used by default'
                : excluded
                  ? 'AGENTS.md excluded'
                  : 'Add project instructions'}
        </strong>
        <p>
          {error
            ? 'Refresh to check the project guidance.'
            : active
              ? 'Agents use the root guide and applicable directory guides. Context exclusions still apply.'
              : excluded
                ? excluded.reason
                : guide
                  ? 'No root AGENTS.md. Create shared guidance for analysis, fixes and agent reviews.'
                  : 'Reading local project guidance.'}
        </p>
      </div>
      {error ? (
        <Button disabled={!!s.busy} onClick={() => void w.loadProjectInstructions()}>
          Retry instructions
        </Button>
      ) : (
        guide && (
          <Button disabled={!!s.busy} onClick={() => void w.openProjectInstructions()}>
            {active || excluded ? 'View instructions' : 'Create AGENTS.md'}
          </Button>
        )
      )}
    </aside>
  );
}
