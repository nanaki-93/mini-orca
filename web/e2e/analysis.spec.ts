// The analysis dashboard: progress, why stages failed, results per section,
// every file's stages, and the run controls.

import { expect, test } from './support/daemon';
import { analysisRun } from './support/fixtures';

test('the dashboard explains a failed run and retries only what failed', async ({ page, daemon }) => {
  daemon.serveRun(analysisRun());
  await page.goto('/#/');
  await page.keyboard.press('g');
  await page.keyboard.press('a');
  await expect(page).toHaveURL(/#\/analysis/);
  await expect(page.getByRole('heading', { name: 'Analysis' })).toBeVisible();

  const progress = page.getByRole('region', { name: 'Progress' });
  await expect(progress).toContainText('16 of 16 stages finished');
  await expect(progress).toContainText('6stages failed');

  const failures = page.getByRole('region', { name: 'Why stages failed' });
  await expect(failures).toContainText('The model provider could not be reached.');
  await expect(failures).toContainText('Bugs, Performance, Security review · 2 files');
  await expect(failures.getByRole('button', { name: 'Check models' })).toBeVisible();

  await expect(page.getByRole('region', { name: 'Models used' })).toContainText('local-bug-model');
  await expect(page.getByRole('region', { name: 'Bugs results' })).toContainText('2 findings');

  await failures.getByRole('button', { name: 'Show failed files' }).click();
  const files = page.getByRole('region', { name: 'Files' });
  await expect(files.getByRole('row')).toHaveCount(3);
  await expect(files).toContainText('internal/vault/crypto.go');
  await expect(files).not.toContainText('internal/api/handlers/webui.go');

  await page.getByRole('region', { name: 'Bugs results' }).getByRole('button', { name: '2 findings' }).click();
  await expect(page).toHaveURL(/#\/issues\?category=bugs/);
  await page.goBack();

  await page.getByRole('button', { name: 'Retry failed' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Analyze' }).click();
  await expect(page.getByText('Analysis started.')).toBeVisible();
  const [preview] = daemon.mutations('/analysis/preview');
  expect(preview.body).toMatchObject({ retry_stale_failed: true, refresh: false, scope: 'project' });
  expect(daemon.mutations('/analysis/run')[0].body).toMatchObject({ preview_id: 'preview-1', retry_stale_failed: true });
  await expect(page.getByRole('button', { name: 'Pause' })).toBeVisible();
});

test('a running analysis can be paused and resumed with a fresh preview', async ({ page, daemon }) => {
  daemon.serveRun(analysisRun('running'));
  await page.goto('/#/analysis');
  await expect(page.getByRole('navigation', { name: 'Workspace' }).getByRole('link', { name: /Analysis/ })).toContainText('1');
  await expect(page.getByRole('contentinfo', { name: 'Status' }).getByRole('button', { name: /Analyzing \d+%/ })).toBeVisible();
  await page.getByRole('button', { name: 'Pause' }).click();
  await expect(page.getByRole('button', { name: 'Resume' })).toBeVisible();
  expect(daemon.mutations('/analysis/run/control')[0].body).toMatchObject({ action: 'pause', identity: { id: 'analysis-1' } });

  await page.getByRole('button', { name: 'Resume' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Resume' }).click();
  await expect(page.getByText('Analysis resumed.')).toBeVisible();
  expect(daemon.mutations('/analysis/preview')[0].body).toMatchObject({ resume_run: { id: 'analysis-1' }, limits: { budget_seconds: 1800 } });
  expect(daemon.mutations('/analysis/run/control')[1].body).toMatchObject({ action: 'resume', preview_id: 'preview-1' });
});

test('the code inspector shows how analysis went for the open file', async ({ page, daemon }) => {
  daemon.serveRun(analysisRun());
  await page.goto('/#/code?file=internal%2Fapi%2Fhandlers%2Fwebui.go');
  const stages = page.getByLabel('Analysis of this file');
  await expect(stages).toContainText('Bugs');
  await expect(stages).toContainText('2 findings');
  await stages.getByRole('button', { name: 'Dashboard' }).click();
  await expect(page).toHaveURL(/#\/analysis/);
});
