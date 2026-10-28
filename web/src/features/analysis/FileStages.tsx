// A file's stages in the last or running analysis, for the code inspector.

import { Activity } from 'lucide-react';
import { useOverview } from '@/api/hooks';
import { STAGES, stageOf, statusLabel, statusTone } from '@/lib/analysis';
import { navigate } from '@/lib/router';
import { plural } from '@/lib/util';
import { Badge } from '@/components/ui';

export function FileStages({ path }: { path: string }) {
  const run = useOverview().data?.analysis_run;
  const file = run?.files?.find(entry => entry.path === path);
  if (!file) return null;
  return (
    <div className="inspector-section stack" aria-label="Analysis of this file">
      <div className="row">
        <Activity size={14} aria-hidden />
        <span className="strong small">Analysis</span>
        <span className="spacer" />
        <button type="button" className="link-btn xs" onClick={() => navigate('/analysis')}>Dashboard</button>
      </div>
      <ul className="mini-list">
        {STAGES.map(({ stage, label }) => {
          const progress = stageOf(file, stage);
          if (!progress) return null;
          return (
            <li key={stage} className="stack-tight">
              <div className="row small">
                <span className="grow">{label}</span>
                {progress.finding_count ? <span className="xs muted">{plural(progress.finding_count, 'finding')}</span> : null}
                <Badge tone={statusTone(progress.status)}>{statusLabel(progress.status)}</Badge>
              </div>
              {progress.reason && progress.status !== 'skipped' ? <span className="xs muted">{progress.reason}</span> : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
