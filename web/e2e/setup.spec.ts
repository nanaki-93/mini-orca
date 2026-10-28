// First-run project setup: the wizard opens for a project without its own
// configuration, saves models, agents and budget, then checks and exclusions.

import { expect, test } from './support/daemon';

test('a project without configuration opens setup and saves it step by step', async ({ page, daemon }) => {
  daemon.serveSetup('needed');
  await page.goto('/#/');
  await expect(page).toHaveURL(/#\/setup/);
  await expect(page.getByRole('heading', { name: 'Set up mini-orca' })).toBeVisible();

  const bug = page.getByRole('region', { name: 'Bug model' });
  await bug.getByLabel('Provider').selectOption('agy');
  await expect(bug.getByLabel('API address')).toHaveCount(0);
  await bug.getByLabel('Model').fill('gemini-3.8-pro-high');
  await bug.getByLabel('Reasoning effort').selectOption('high');
  await bug.getByRole('button', { name: 'Test connection' }).click();
  await expect(bug).toContainText('Connected to gemini-3.8-pro-high');

  const analyze = page.getByRole('region', { name: 'Analyze model' });
  await expect(analyze.getByLabel('API key (optional)')).toHaveAttribute('placeholder', '•••••••• saved');
  await page.getByRole('button', { name: 'Next' }).click();

  await page.getByLabel('Reviews').selectOption('code');
  await page.getByLabel('Model calls').fill('30');
  await page.getByRole('button', { name: 'Save and continue' }).click();
  await expect(page.getByText('Project configuration saved.')).toBeVisible();
  const [saved] = daemon.mutations('/setup');
  expect(saved.body).toMatchObject({
    models: { bug: { provider: 'agy', model: 'gemini-3.8-pro-high', reasoning_effort: 'high' }, analyze: { model: 'gemini-2.5-pro', api_key_set: true } },
    roles: { review: 'code' },
    limits: { model_calls: 30 },
  });
  expect((saved.body!.models as Record<string, { api_base_url?: string }>).bug.api_base_url).toBeUndefined();

  await expect(page.getByRole('region', { name: 'Checks' })).toContainText('go test ./...');
  await page.getByRole('button', { name: 'Next' }).click();

  const reads = page.getByRole('region', { name: 'What analysis reads' });
  await expect(reads).toContainText('usually skipped');
  await reads.getByRole('checkbox', { name: /vendor\// }).uncheck();
  await expect(reads).toContainText('6 files included');
  await page.getByRole('button', { name: 'Finish', exact: true }).click();
  await expect(page).toHaveURL(/#\/$/);
  expect(daemon.mutations('/analysis/selection')[0].body).toMatchObject({ selection_id: 'sel-0', excluded_paths: ['vendor/lib/a.go', 'vendor/lib/b.go'] });
});

test('setup can be skipped and is not offered again', async ({ page, daemon }) => {
  daemon.serveSetup('needed');
  await page.goto('/#/tasks');
  await expect(page).toHaveURL(/#\/setup/);
  await page.getByRole('button', { name: 'Skip and use the daemon’s settings' }).click();
  await expect(page).toHaveURL(/#\/$/);
  expect(daemon.mutations('/setup/skip')).toHaveLength(1);
  await expect(page.getByText('Set up this project')).toHaveCount(0);
  await page.goto('/#/settings/project');
  const card = page.getByRole('region', { name: 'Project configuration' });
  await expect(card).toContainText('Daemon configuration');
  await card.getByRole('button', { name: 'Run setup' }).click();
  await expect(page).toHaveURL(/#\/setup/);
});
