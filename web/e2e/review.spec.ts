// The core job: review every changed file, approve this exact version and
// apply it, with the review gate tracking each step.

import { expect, test } from './support/daemon';
import { rejectedTask } from './support/fixtures';

test('reviews, approves and applies a task through the gate', async ({ page, daemon }) => {
  await page.goto('/#/tasks/chg-1');
  const gate = page.getByRole('complementary', { name: 'Review gate' });
  await expect(gate.getByText('0 of 3')).toBeVisible();
  await expect(gate.getByRole('button', { name: /apply changes/i })).toBeDisabled();
  // An optional check failure is a warning, not a blocker.
  await expect(gate.getByText('1 optional failed')).toBeVisible();

  await page.getByRole('button', { name: 'Review files' }).click();
  await expect(page).toHaveURL(/#\/tasks\/chg-1\/files/);
  for (let viewed = 1; viewed <= 3; viewed++) {
    await page.keyboard.press('v');
    await expect(page.getByText(`${viewed}/3 viewed`)).toBeVisible();
  }
  expect(daemon.mutations('/review-file').map(request => request.body?.path)).toEqual([
    'internal/api/handlers/webui.go',
    'internal/api/handlers/webui_test.go',
    'internal/api/handlers/mime.go',
  ]);

  await page.getByRole('tab', { name: /conversation/i }).click();
  await gate.getByRole('button', { name: 'Approve this version' }).click();
  await expect(page.getByText('Approved this exact version')).toBeVisible();

  await gate.getByRole('button', { name: 'Apply & commit' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('commits it on main');
  await dialog.getByRole('button', { name: 'Apply & commit' }).click();
  await expect(page.getByText('Applied and committed.')).toBeVisible();

  const [apply] = daemon.mutations('/apply');
  expect(apply.body).toMatchObject({ project_id: 'p1', project_revision: 'r42', revision: 3, hash: 'sha256:9f2c1a7be4', confirm: true, branch: 'main' });
});

test('cancelling the Apply confirmation writes nothing', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, reviewed_files: session.changes.map(change => change.path), reviewed_hash: session.hash });
  await page.goto('/#/tasks/chg-1');
  await page.getByRole('complementary', { name: 'Review gate' }).getByRole('button', { name: /apply/i }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByRole('alertdialog')).toBeHidden();
  expect(daemon.mutations('/apply')).toEqual([]);
});

test('compares the split and unified diff layouts', async ({ page }) => {
  await page.goto('/#/tasks/chg-1/files');
  const diff = page.getByRole('region', { name: /Diff of internal\/api\/handlers\/webui.go/ });
  await expect(diff).toContainText('strings.Contains(name, "/")');
  await page.keyboard.press('s');
  await expect(page.getByRole('button', { name: /unified view/i })).toBeVisible();
});

test('overrides the reviewers with a reason, then approves with notes', async ({ page, daemon }) => {
  daemon.sessions.set('chg-1', rejectedTask());
  await page.goto('/#/tasks/chg-1');
  const gate = page.getByRole('complementary', { name: 'Review gate' });
  await expect(gate.getByText('Changes requested')).toBeVisible();
  await expect(gate.getByRole('button', { name: /^Approve/ })).toHaveCount(0);

  // Advisory notes are shown apart from blocking findings.
  await page.getByRole('button', { name: /Correctness/ }).click();
  await expect(page.getByText('Consider a table-driven test.')).toBeVisible();
  await expect(page.getByText('1 blocking finding')).toBeVisible();

  await gate.getByRole('button', { name: 'Override reviewers…' }).click();
  const dialog = page.getByRole('dialog', { name: 'Override the reviewers?' });
  await expect(dialog).toContainText('Move the MIME table into its own package.');
  await expect(dialog.getByRole('button', { name: 'Override reviewers' })).toBeDisabled();
  await dialog.getByLabel('Override reason').fill('The MIME table is private to this handler.');
  await dialog.getByRole('button', { name: 'Override reviewers' }).click();
  await expect(dialog).toBeHidden();
  await expect(gate.getByText('Overridden by you')).toBeVisible();
  await expect(page.getByText('The MIME table is private to this handler.')).toBeVisible();
  expect(daemon.mutations('/review-override')[0].body).toMatchObject({ revision: 3, hash: 'sha256:9f2c1a7be4', workflow_id: 'wf-1', reason: 'The MIME table is private to this handler.' });

  await gate.getByRole('button', { name: 'Add notes' }).click();
  await gate.getByLabel('Approval notes').fill('Revisit the package split next sprint.');
  await gate.getByRole('button', { name: 'Approve with notes' }).click();
  await expect(page.getByText('Approved this exact version')).toBeVisible();
  await expect(gate.getByText('Revisit the package split next sprint.')).toBeVisible();
  expect(daemon.mutations('/review')[0].body).toMatchObject({ hash: 'sha256:9f2c1a7be4', notes: 'Revisit the package split next sprint.' });
  await expect(gate.getByRole('button', { name: 'Apply & commit' })).toBeEnabled();
});

test('checks from an earlier project tree must run again before approval', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, reviewed_files: session.changes.map(change => change.path), checks_outdated: true });
  await page.goto('/#/tasks/chg-1');
  const gate = page.getByRole('complementary', { name: 'Review gate' });
  await expect(gate.getByText('Outdated')).toBeVisible();
  await expect(gate.getByText('The project changed since checks ran. Run them again before approval.')).toBeVisible();
  await expect(gate.getByRole('button', { name: /^Approve/ })).toHaveCount(0);
  await page.getByRole('tab', { name: /checks/i }).click();
  await expect(page.getByText('These checks ran on an earlier version of the project')).toBeVisible();
});

test('a stale task names the file that changed', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, freshness: 'stale', stale_reason: 'go.mod changed since this proposal was made' });
  await page.goto('/#/tasks/chg-1');
  await expect(page.getByText('Out of date: go.mod changed since this proposal was made. Start it again to continue.')).toBeVisible();
});
