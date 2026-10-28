// Navigation and the supporting workspaces: the inbox, the command palette,
// creating a task, branches and issues.

import { expect, test } from './support/daemon';

test('the inbox summarizes what needs you and opens a task', async ({ page }) => {
  await page.goto('/#/');
  await expect(page.getByRole('heading', { name: 'Inbox' })).toBeVisible();
  await expect(page.getByText('1 task to review · 1 agent run in progress · 1 failed or stale · 1 branch ready to merge')).toBeVisible();
  await page.getByRole('region', { name: 'Needs your review' }).getByText('Reject nested asset paths').click();
  await expect(page).toHaveURL(/#\/tasks\/chg-1$/);
  await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toContainText('Reject nested asset paths in the Web UI handler');
});

test('g-sequences and the palette move between workspaces', async ({ page }) => {
  await page.goto('/#/');
  await expect(page.getByRole('heading', { name: 'Inbox' })).toBeVisible();
  await page.keyboard.press('g');
  await page.keyboard.press('i');
  await expect(page).toHaveURL(/#\/issues/);

  await page.keyboard.press('Control+k');
  await page.getByPlaceholder(/search files, tasks/i).fill('webui.go');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/#\/code\?file=internal%2Fapi%2Fhandlers%2Fwebui\.go|#\/code\?file=internal\/api\/handlers\/webui\.go/);
  await expect(page.getByText('func ServeAsset() {}')).toBeVisible();
});

test('creates a draft task with a scope and criteria', async ({ page, daemon }) => {
  await page.goto('/#/tasks');
  await expect(page.getByRole('heading', { name: 'Tasks' })).toBeVisible();
  await page.keyboard.press('c');
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('What should change?').fill('Return 404 for missing assets');
  await dialog.getByPlaceholder('Search files to add…').fill('webui');
  await dialog.getByRole('option', { name: /internal\/api\/handlers\/webui\.go/ }).first().click();
  await dialog.getByPlaceholder(/e\.g\. Empty input/).fill('Missing assets return 404');
  await dialog.getByRole('button', { name: 'Add' }).click();
  await dialog.getByRole('button', { name: 'Save draft' }).click();

  await expect(page.getByText('Draft saved.')).toBeVisible();
  const [create] = daemon.mutations('/changes');
  expect(create.body).toMatchObject({ project_id: 'p1', title: 'Return 404 for missing assets', acceptance_criteria: ['Missing assets return 404'] });
  expect(create.body?.paths).toContain('internal/api/handlers/webui.go');
  await expect(page).toHaveURL(/#\/tasks\/chg-100/);
});

test('branches explain why a worktree cannot be merged yet', async ({ page }) => {
  await page.goto('/#/git');
  await page.getByRole('listitem').filter({ hasText: 'mo/chg-2' }).click();
  const detail = page.getByRole('complementary', { name: 'Worktree' });
  await expect(detail.getByText('No new commits')).toBeVisible();
  await expect(detail.getByText('This branch has no commits that the main branch lacks yet.')).toBeVisible();
  await expect(detail.getByRole('button', { name: /merge into/i })).toHaveCount(0);

  await page.getByRole('listitem').filter({ hasText: 'mo/chg-7' }).click();
  await expect(detail.getByRole('button', { name: 'Merge into main' })).toBeEnabled();
  await expect(detail.getByText('1 overlapping file')).toBeVisible();
});

test('issues label where each finding comes from', async ({ page }) => {
  await page.goto('/#/issues');
  const table = page.getByRole('table');
  await expect(table.getByRole('row', { name: /size limit/ })).toContainText('Security rule');
  await expect(table.getByRole('row', { name: /ignores cancellation/ })).toContainText('AI suggestion');
});

for (const theme of ['dark', 'light'] as const) {
  test(`every workspace renders in the ${theme} theme`, async ({ page }) => {
    await page.addInitScript(value => localStorage.setItem('mini-orca.prefs.v2', JSON.stringify({ theme: value })), theme);
    for (const [hash, heading] of [['/#/', 'Inbox'], ['/#/tasks', 'Tasks'], ['/#/issues', 'Issues'], ['/#/git', 'Branches'], ['/#/settings', 'Project']] as const) {
      await page.goto(hash);
      await expect(page.getByRole('heading', { name: heading, exact: true })).toBeVisible();
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    }
  });
}

test('usage lists what agents spent per task and the task shows its own spend', async ({ page }) => {
  await page.goto('/#/settings/usage');
  const table = page.getByRole('table', { name: 'Usage by task' });
  await expect(table.getByRole('row').filter({ hasText: 'Reject nested asset paths' })).toContainText('$0.194');
  await expect(table.getByRole('row').filter({ hasText: 'Total' })).toContainText('≥ $0.575');
  await expect(page.getByText('Some models have no configured rate.')).toBeVisible();
  await table.getByRole('row').filter({ hasText: 'Reject nested asset paths' }).click();
  await expect(page).toHaveURL(/#\/tasks\/chg-1/);
  await expect(page.getByRole('complementary', { name: 'Review gate' })).toContainText('6 calls · 65k tokens · $0.194');
  await expect(page.getByRole('region', { name: 'Agent run' })).toContainText('21k tokens · $0.061');
});

test('a finished agent run is announced with its spend', async ({ page, daemon }) => {
  daemon.push({ kind: 'workflow', session_id: 'chg-1', workflow_id: 'wf-1', title: 'Reject nested asset paths in the Web UI handler', status: 'awaiting_human_review', usage: { model_calls: 6, input_tokens: 59_200, output_tokens: 5_300 }, cost: { status: 'estimated', currency: 'USD', known_subtotal: 0.194, total: 0.194, reasons: [] } });
  await page.goto('/#/tasks');
  await expect(page.getByText('Agents finished “Reject nested asset paths in the Web UI handler”. It needs your review.')).toBeVisible();
  await expect(page.getByText('6 calls · 65k tokens · $0.194')).toBeVisible();
});

test('set-aside files are announced and explained with their remedy', async ({ page, daemon }) => {
  daemon.serve('/api/projects/current/quarantine', () => ({
    files: [{ kind: 'agent_journal', file: '.mini-orca/agents/ab12/journal.json', quarantined_as: '.mini-orca/agents/ab12/journal.quarantined-20261027T101500Z.json', reason: 'it is not valid JSON', remedy: 'Mini-Orca no longer reads the set-aside file. Inspect it if you need its history, then delete it.', at: '2026-10-27T10:15:00Z' }],
    skipped: [{ kind: 'task', file: '.mini-orca/changes/change-9f.json', reason: 'written by a newer Mini-Orca', remedy: 'Upgrade Mini-Orca to use it, or move it away.' }],
  }));
  await page.goto('/#/');
  await page.getByRole('button', { name: 'Review' }).click();
  await expect(page).toHaveURL(/#\/settings\/project/);
  const list = page.getByRole('region', { name: 'Set-aside files' });
  await expect(list).toContainText('.mini-orca/agents/ab12/journal.json');
  await expect(list).toContainText('It is not valid JSON. Its bytes are in .mini-orca/agents/ab12/journal.quarantined-20261027T101500Z.json.');
  await expect(list).toContainText('Upgrade Mini-Orca to use it, or move it away.');
});

test('agent roles can run with any listed model and a project budget', async ({ page, daemon }) => {
  await page.goto('/#/settings/models');
  const card = page.getByRole('region', { name: 'Agent roles' });
  await expect(card).toContainText('From config.yaml');
  const review = card.getByLabel('Reviews');
  await expect(review).toHaveValue('analyze');
  await expect(review.getByRole('option', { name: 'Gemini 3.8 Pro (agy:gemini-3.8-pro-high)' })).toBeAttached();
  await expect(card).toContainText('Pi models could not be loaded.');

  await review.selectOption('agy:gemini-3.8-pro-high');
  await expect(card.getByText('remote · cli://agy')).toBeVisible();
  const calls = card.getByLabel('Model calls');
  await calls.fill('50');
  await expect(card.getByText('At most 40 (config.yaml).')).toBeVisible();
  await expect(card.getByRole('button', { name: 'Save' })).toBeDisabled();
  await calls.fill('20');
  await card.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Agent settings saved. New runs use them.')).toBeVisible();
  await expect(card).toContainText('Project settings');
  const [saved] = daemon.mutations('/agent-settings/control');
  expect(saved.body).toMatchObject({ action: 'replace', confirm: true, version: 0, configuration: 'c'.repeat(64), values: { roles: { creation: 'bug', testing: 'code', review: 'agy:gemini-3.8-pro-high' }, disabled_skills: [] } });
  expect((saved.body!.values as { limits: Record<string, number> }).limits.model_calls).toBe(20);
  await expect(review).toHaveValue('agy:gemini-3.8-pro-high');

  await card.getByRole('button', { name: 'Use config.yaml' }).click();
  await expect(card).toContainText('From config.yaml');
  await expect(review).toHaveValue('analyze');
  expect(daemon.mutations('/agent-settings/control')[1].body).toMatchObject({ action: 'reset', version: 1 });
});

test('a daemon without config.yaml warns that it uses the default models', async ({ page, daemon }) => {
  daemon.serve('/api/models/current', () => ({ scopes: {}, config_file: '' }));
  await page.goto('/#/');
  await page.getByRole('button', { name: 'No config.yaml · default models' }).click();
  await expect(page).toHaveURL(/#\/settings\/models/);
  await expect(page.getByText('No config.yaml was found')).toBeVisible();
});

test('stale and failed tasks can be deleted from the list and the task page', async ({ page, daemon }) => {
  daemon.addTask({ project_id: 'p1', project_revision: 'r41', id: 'chg-8', kind: 'fix', title: 'Trim the import log', revision: 3, hash: 'h8', state: 'draft', freshness: 'stale', updated_at: '2026-10-20T10:00:00Z', paths: ['internal/project/import.go'] });
  await page.goto('/#/tasks');
  const attention = page.getByRole('region', { name: 'Failed or stale' });
  await expect(attention.getByRole('listitem')).toHaveCount(2);
  await attention.getByRole('button', { name: 'Delete all 2' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('“Cache the parsed index”, “Trim the import log”');
  await dialog.getByRole('button', { name: 'Delete 2 tasks' }).click();
  await expect(page.getByText('2 tasks deleted.')).toBeVisible();
  await expect(attention).toHaveCount(0);
  expect(daemon.mutations('/delete').map(request => request.body)).toEqual([
    { project_id: 'p1', project_revision: 'r42', revision: 2, hash: 'x' },
    { project_id: 'p1', project_revision: 'r41', revision: 3, hash: 'h8' },
  ]);

  await page.goto('/#/tasks/chg-1');
  await page.getByRole('button', { name: 'Delete task' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete task' }).click();
  await expect(page).toHaveURL(/#\/tasks$/);
  await expect(page.getByText('Reject nested asset paths')).toHaveCount(0);
});
