import type { State } from './workspace';
import { BulletContent, Disclosure, Notice, Panel, Prose, human } from './ui';

export function SolutionExcerpt({ text }: { text: string }) {
  const paragraph = text.split(/\n\s*\n/)[0];
  const summary = paragraph.length > 240 ? `${paragraph.slice(0, 240).trimEnd()}…` : paragraph;
  return (
    <>
      <Prose text={summary} />
      {summary !== text && (
        <Disclosure title="Full explanation">
          <Prose text={text} />
        </Disclosure>
      )}
    </>
  );
}

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
        <SolutionExcerpt text={explanations[explanations.length - 1]} />
      ) : (
        <SolutionExcerpt
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
    </Panel>
  );
}
