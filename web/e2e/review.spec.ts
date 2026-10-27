// The core job: review every changed file, approve this exact version and
// apply it, with the review gate tracking each step.

import { expect, test } from './support/daemon';

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
