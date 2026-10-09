import type { State } from './workspace';
import { BulletContent, Notice, Panel, Prose, human } from './ui';

function solutionText(text: string) {
  const source = text.trim();
  // Older saved explanations combined a labeled cause and solution in one field.
  if (
    !/^(?:#{1,6}[ \t]+)?(?:\*\*|__)?(?:root[ \t]+)?cause(?:\*\*|__)?[ \t]*(?::|\r?\n)/i.test(source)
  )
    return source;
  const solution =
    /(?:^|\n|[.!?][ \t]+)(?:#{1,6}[ \t]+)?(?:\*\*|__)?(?:proposed[ \t]+)?solution(?:\*\*|__)?[ \t]*(?::(?:\*\*|__)?|\r?\n)/i.exec(
      source,
    );
  return solution ? source.slice(solution.index + solution[0].length).trim() : source;
}

export function FixSolution({ s }: { s: State }) {
  const change = s.change;
  const workflow = change?.workflow;
  const explanations = change?.messages
    .filter((entry) => entry.role === 'assistant')
    .map((entry) => solutionText(entry.content))
    .filter(Boolean);
  const review = workflow?.review;
  const needsChanges = review && (review.verdict !== 'approve' || review.findings.length > 0);
  const checks = change?.checks.filter((check) => check.state !== 'passed');
  return (
    <Panel title="Proposed solution" className="fix-explanation fix-solution">
      {explanations?.length ? (
        explanations.map((text, index) => <Prose key={index} text={text} />)
      ) : (
        <Prose
          text={
            s.changeSeed?.finding?.solution ||
            'Review the generated explanation and file differences below. The proposed correction is subject to tests and your review.'
          }
        />
      )}
      {workflow?.reason && <Notice error={workflow.status === 'failed'}>{workflow.reason}</Notice>}
      {needsChanges && (
        <section className="fix-review-issues" aria-label="Changes needed">
          <h3>Changes needed before acceptance</h3>
          <Prose text={review.summary} />
          <BulletContent title="Requested changes" items={review.findings} />
        </section>
      )}
      <BulletContent
        title="Checks needing attention"
        items={checks?.map((check) => `${check.name}: ${human(check.state)}`)}
      />
      <BulletContent
        title="Acceptance criteria"
        items={change?.acceptance_criteria || s.changeSeed?.acceptance_criteria}
      />
    </Panel>
  );
}
