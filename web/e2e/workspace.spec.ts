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
