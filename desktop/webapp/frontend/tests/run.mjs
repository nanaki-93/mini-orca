import { chromium } from '@playwright/test';
import { strict as assert } from 'node:assert';
import { mkdir } from 'node:fs/promises';
import { serve } from '../preview.mjs';
import { installFixture } from './fixture.mjs';

const server = await serve(0);
const url = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({
  headless: true,
  ...(process.env.MINI_ORCA_TEST_BROWSER || process.platform === 'darwin'
    ? { channel: process.env.MINI_ORCA_TEST_BROWSER || 'chrome' }
    : {}),
});
const output = new URL('../test-results/', import.meta.url).pathname;
await mkdir(output, { recursive: true });
let checks = 0;
const errors = [];
async function pageFor(options = {}) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('request', (request) => {
    if (!request.url().startsWith(url) && !request.url().startsWith('data:'))
      errors.push(`Unexpected outbound request: ${request.url()}`);
  });
  await page.addInitScript(installFixture, options);
  await page.goto(url);
  await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
  await page.waitForFunction(() =>
    window.fixture.requests.some((request) => request.path?.endsWith('/analysis/selection')),
  );
  return { page, close: () => context.close() };
}
async function idle(page) {
  await page.locator('.busy-strip').waitFor({ state: 'hidden' });
}
async function nav(page, name) {
  await page
    .getByRole('navigation', { name: /Workspaces|Tools/ })
    .getByRole('button', { name, exact: true })
    .click();
}
async function openSource(page) {
  await nav(page, 'Editor');
  await page.locator('.file-item[title="internal/worker/process.go"]').click();
  await page.getByLabel('Declaration', { exact: true }).selectOption('Process');
}
async function prepare(page, remote = false) {
  await openSource(page);
  await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
  await page
    .getByLabel('Change request')
    .fill('Return the context error when the request is canceled.');
  await page.getByRole('button', { name: 'Prepare draft', exact: true }).click();
  if (remote)
    await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click();
  await page.getByLabel('Declaration draft', { exact: true }).waitFor();
  await idle(page);
}
async function validateAndCheck(page, tests = false) {
  await page.getByRole('button', { name: 'Validate draft', exact: true }).click();
  await idle(page);
  await page.getByRole('button', { name: 'Continue to checks', exact: true }).click();
  if (tests) await page.getByLabel('Tests', { exact: true }).check();
  await page.getByRole('button', { name: 'Run checks', exact: true }).click();
  if (tests)
    await page.getByRole('dialog').getByRole('button', { name: 'Trust this project' }).click();
  await idle(page);
  await page.getByRole('button', { name: 'Review change', exact: true }).click();
  await idle(page);
}
async function layout(page, name) {
  await page.screenshot({ path: `${output}/${name}.png`, fullPage: true });
  assert.equal(
    await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1),
    false,
    `${name}: horizontal page overflow`,
  );
  assert.equal(
    await page.getByRole('heading', { name: 'Unable to display this screen' }).count(),
    0,
    `${name}: rendering failed`,
  );
  assert.equal(
    await page
      .locator('button:not([aria-label])')
      .evaluateAll(
        (buttons) =>
          buttons.filter((button) => button.offsetParent && !button.textContent.trim()).length,
      ),
    0,
    `${name}: unnamed button`,
  );
  checks++;
}
async function test(name, body) {
  await body();
  checks++;
  console.log(`PASS ${name}`);
}

try {
  await test('Navigation and disclosure do not dispatch providers, code, source writes or shells', async () => {
    const { page, close } = await pageFor({ unknown: true });
    await layout(page, 'summary');
    for (const name of [
      'Analysis',
      'Bugs',
      'Performance',
      'Security',
      'Editor',
      'Models',
      'Project',
      'Terminal',
    ]) {
      await nav(page, name);
      await layout(page, name.toLowerCase());
    }
    const calls = await page.evaluate(() =>
      window.fixture.requests.filter((request) => request.method !== 'GET'),
    );
    assert.deepEqual(
      calls.map((request) => request.path),
      ['/api/projects/restore'],
    );
    assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
    await close();
  });
  await test('Draft, checks, review, explicit Apply and guarded Undo', async () => {
    const { page, close } = await pageFor();
    await prepare(page);
    await page
      .getByLabel('Declaration draft', { exact: true })
      .fill('func Process(ctx context.Context) error {\n\treturn ctx.Err()\n}');
    await layout(page, 'draft');
    await validateAndCheck(page, true);
    await layout(page, 'review');
    assert.equal(
      await page.getByRole('button', { name: 'Apply change', exact: true }).isEnabled(),
      true,
    );
    await page.getByRole('button', { name: 'Apply change', exact: true }).click();
    await idle(page);
    await page.getByRole('heading', { name: 'Change applied' }).waitFor();
    const apply = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.path?.endsWith('/apply')),
    );
    assert.deepEqual(
      Object.keys(apply.body).sort(),
      [
        'draft_id',
        'draft_revision',
        'draft_hash',
        'project_id',
        'project_revision',
        'base_file_hash',
        'confirm',
      ].sort(),
    );
    assert.equal(apply.body.draft_revision, 2);
    assert.equal(apply.body.confirm, true);
    await layout(page, 'receipt');
    await page.getByRole('button', { name: 'Undo change', exact: true }).click();
    await page
      .getByRole('dialog')
      .getByRole('button', { name: 'Undo change', exact: true })
      .click();
    await idle(page);
    await page.getByRole('heading', { name: 'Change undone' }).waitFor();
    const undo = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.path?.endsWith('/undo')),
    );
    assert.equal(undo.body.post_apply_hash, 'applied-hash');
    assert.equal(undo.body.project_revision, 'revision-2');
    await close();
  });
  await test('Editing invalidates checks; changed source blocks Apply', async () => {
    const { page, close } = await pageFor();
    await prepare(page);
    await validateAndCheck(page);
    await page.getByRole('button', { name: 'Edit draft', exact: true }).click();
    await page
      .getByLabel('Declaration draft', { exact: true })
      .fill('func Process(ctx context.Context) error { return nil }');
    await page.getByRole('tab', { name: 'Checks', exact: true }).click();
    assert.equal(
      await page.getByRole('button', { name: 'Run checks', exact: true }).isDisabled(),
      true,
    );
    await page.getByRole('tab', { name: /^Draft/ }).click();
    await validateAndCheck(page);
    await page.evaluate(() => {
      window.fixture.state.changed = true;
    });
    await page.getByRole('button', { name: 'Apply change', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.evaluate(() => window.fixture.requests.some((r) => r.path?.endsWith('/apply'))),
      false,
    );
    assert.equal(
      await page.getByRole('button', { name: 'Apply change', exact: true }).isDisabled(),
      true,
    );
    await close();
  });
  await test('Remote consent and model content stay scoped and inert', async () => {
    const { page, close } = await pageFor({ remote: true, hostile: true });
    await prepare(page, true);
    const sent = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.path?.endsWith('/messages')),
    );
    assert.equal(sent.body.confirm_remote_provider, true);
    await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
    assert.equal(
      await page.locator('a[href^="https://evil"], img[src^="https://evil"]').count(),
      0,
    );
    assert.ok(
      await page.getByText('<img src="https://evil.invalid/tracker"', { exact: false }).count(),
    );
    await close();
  });
  await test('New declaration, discard approval and Security destination', async () => {
    const { page, close } = await pageFor({ remote: true });
    await openSource(page);
    await page.getByRole('button', { name: 'New declaration', exact: true }).click();
    await page.getByLabel('Name', { exact: true }).fill('NewWorker');
    await page.getByLabel('Change request').fill('Create a worker that returns its context error.');
    await layout(page, 'new-declaration');
    await page.getByRole('button', { name: 'Prepare draft', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Continue' }).click();
    await idle(page);
    assert.match(
      await page.getByLabel('Declaration draft', { exact: true }).inputValue(),
      /NewWorker/,
    );
    const session = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.path?.endsWith('/chat/sessions')),
    );
    assert.equal(session.body.mode, 'create_symbol');
    assert.equal(session.body.target_symbol, 'NewWorker');
    await page.locator('.file-item[title="internal/worker/config.go"]').click();
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
    assert.equal(await page.getByRole('heading', { name: 'process.go', exact: true }).count(), 1);
    await nav(page, 'Security');
    await page.getByRole('button', { name: 'AI Security review', exact: true }).click();
    assert.ok(await page.getByRole('dialog').getByText('Scope: analyze', { exact: true }).count());
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some((r) => r.path?.endsWith('/security-review')),
      ),
      false,
    );
    await close();
  });
  await test('Canceled model request cannot publish a late draft', async () => {
    const { page, close } = await pageFor();
    await openSource(page);
    await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
    await page.getByLabel('Change request').fill('Return the context error.');
    await page.evaluate(() => {
      window.fixture.hold = '/api/projects/current/chat/sessions/session-1/messages';
    });
    await page.getByRole('button', { name: 'Prepare draft', exact: true }).click();
    await page.waitForFunction(() =>
      window.fixture.requests.some((r) => r.path?.endsWith('/messages')),
    );
    await page.locator('.busy-strip').getByRole('button', { name: 'Cancel' }).click();
    await idle(page);
    await page.evaluate(() => window.fixture.release());
    await page.waitForTimeout(100);
    assert.equal(await page.getByLabel('Declaration draft', { exact: true }).count(), 0);
    assert.equal(
      await page.evaluate(() => window.fixture.requests.some((r) => r.method === 'CANCEL')),
      true,
    );
    await close();
  });
  await test('Analysis preview requires consent; pause and resume use captured identities', async () => {
    const { page, close } = await pageFor({ remote: true });
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.getByRole('button', { name: 'Start analysis', exact: true }).isDisabled(),
      true,
    );
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some(
          (r) => r.method === 'POST' && r.path.endsWith('/analysis/run'),
        ),
      ),
      false,
    );
    await layout(page, 'analysis-preview');
    await page.getByLabel('Allow selected context to this provider').check();
    await page.getByLabel('Include AI Security review').check();
    await page.getByRole('button', { name: 'Start analysis', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Pause', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Prepare continuation', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.getByLabel('Allow selected context to this provider').isChecked(),
      false,
    );
    await page.getByLabel('Allow selected context to this provider').check();
    await page.getByLabel('Include AI Security review').check();
    await page.getByRole('button', { name: 'Resume analysis', exact: true }).click();
    await idle(page);
    const resumed = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.body?.action === 'resume'),
    );
    assert.equal(resumed.body.identity.generation, 'generation-1');
    assert.equal(resumed.body.preview_id, 'preview-1');
    await layout(page, 'analysis-run');
    await close();
  });
  await test('Late file responses cannot replace the current file', async () => {
    const { page, close } = await pageFor();
    await nav(page, 'Editor');
    await page.evaluate(() => {
      window.fixture.hold = '/api/projects/current/files/info';
    });
    await page.locator('.file-item[title="internal/worker/process.go"]').click();
    await page.waitForFunction(() =>
      window.fixture.requests.some((r) => r.path?.endsWith('/files/info')),
    );
    await page.evaluate(() => {
      window.fixture.hold = '';
    });
    await page.locator('.file-item[title="internal/worker/config.go"]').click();
    await page.getByRole('heading', { name: 'config.go', exact: true }).waitFor();
    await page.evaluate(() => window.fixture.release());
    await page.waitForTimeout(100);
    assert.equal(await page.getByRole('heading', { name: 'config.go', exact: true }).count(), 1);
    await close();
  });
  await test('Ambiguous Apply failure disables repeat writes and preserves the draft', async () => {
    const { page, close } = await pageFor();
    await prepare(page);
    await validateAndCheck(page);
    await page.evaluate(() => {
      window.fixture.failures['/api/projects/current/apply'] = 'transport';
    });
    await page.getByRole('button', { name: 'Apply change', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.getByRole('button', { name: 'Apply change', exact: true }).isDisabled(),
      true,
    );
    assert.ok(
      await page
        .getByText('The source operation could not be confirmed.', { exact: false })
        .count(),
    );
    await page.getByRole('button', { name: 'Edit draft', exact: true }).click();
    assert.match(
      await page.getByLabel('Declaration draft', { exact: true }).inputValue(),
      /ctx.Err/,
    );
    await close();
  });
  await test('Benchmarks, context, explanations, scan, findings, terminal and compact layouts', async () => {
    const { page, close } = await pageFor();
    await prepare(page);
    await validateAndCheck(page);
    await page.getByRole('button', { name: 'Checks', exact: true }).click();
    await page.getByRole('button', { name: 'Benchmarks', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Compare', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Trust this project' }).click();
    await idle(page);
    await layout(page, 'benchmark');
    await nav(page, 'Editor');
    await page.getByRole('tab', { name: 'Context' }).click();
    await page.getByRole('heading', { name: 'Included files' }).waitFor();
    await layout(page, 'context');
    await page.getByRole('tab', { name: 'Source' }).click();
    await page.getByRole('button', { name: 'Explain declaration' }).click();
    await idle(page);
    await layout(page, 'assistant');
    for (const name of ['Bugs', 'Performance', 'Security']) {
      await nav(page, name);
      await page.locator('.result-row').first().click();
      await layout(page, `${name.toLowerCase()}-detail`);
    }
    await nav(page, 'Project');
    await page.getByRole('button', { name: 'Verified scan', exact: true }).click();
    await page.getByRole('button', { name: 'Run scan', exact: true }).click();
    await idle(page);
    await layout(page, 'scan');
    await nav(page, 'Terminal');
    await page.evaluate(() => {
      window.fixture.hold = 'OpenTerminal';
    });
    await page.getByRole('button', { name: 'Start terminal', exact: true }).click();
    await page.waitForFunction(() => window.fixture.terminals.some((t) => t.action === 'open'));
    assert.equal(
      await page.locator('.busy-strip').getByRole('button', { name: 'Cancel' }).count(),
      0,
    );
    assert.equal(await page.getByRole('button', { name: 'New terminal' }).isDisabled(), true);
    await page.evaluate(() => {
      window.fixture.hold = '';
      window.fixture.release();
    });
    await idle(page);
    await page.locator('.xterm-screen').waitFor();
    await page.keyboard.type('echo hello');
    await page.waitForFunction(() => window.fixture.terminals.some((t) => t.action === 'input'));
    await layout(page, 'terminal-running');
    await nav(page, 'Summary');
    await page.getByRole('button', { name: 'Explore', exact: true }).click();
    await layout(page, 'diagrams');
    await page.keyboard.press('Meta+k');
    await page.getByLabel('Search files and commands', { exact: true }).last().fill('process');
    await layout(page, 'search');
    for (const [width, height] of [
      [1024, 768],
      [900, 640],
    ]) {
      await page.setViewportSize({ width, height });
      if (width === 900) await page.getByRole('button', { name: 'Larger text' }).click();
      for (const name of ['Summary', 'Analysis', 'Bugs', 'Editor', 'Models', 'Project']) {
        await nav(page, name);
        await layout(page, `${name.toLowerCase()}-${width}`);
      }
      await page.getByRole('tab', { name: 'Review', exact: true }).count();
    }
    await page.getByRole('button', { name: 'Switch to light appearance' }).click();
    await nav(page, 'Summary');
    await layout(page, 'summary-light');
    await close();
  });
  await test('Empty, partial, canceled and failed results keep their state', async () => {
    for (const options of [
      { empty: true },
      { runStatus: 'partial' },
      { runStatus: 'canceled' },
      { runStatus: 'failed' },
    ]) {
      const { page, close } = await pageFor(options);
      await nav(page, 'Bugs');
      await layout(page, `results-${options.runStatus || 'empty'}`);
      await close();
    }
  });
  assert.deepEqual(errors, []);
  console.log(`PASS ${checks} workflow and layout checks; no browser errors or external requests`);
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
