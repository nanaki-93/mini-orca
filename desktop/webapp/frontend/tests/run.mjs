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
async function expandSummaryDiagrams(page) {
  await page.getByText('Show architecture', { exact: true }).click();
  const flows = page.getByText('Show project flows', { exact: true });
  if (await flows.count()) await flows.click();
}
async function openSource(page) {
  await nav(page, 'Source');
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
async function contrast(page, name) {
  const failures = await page.evaluate(() => {
    const rgba = (color) => {
      const channels = color.match(/[\d.]+/g).map(Number);
      return [...channels.slice(0, 3), channels[3] ?? 1];
    };
    const background = (element) => {
      const parents = [];
      for (let current = element; current; current = current.parentElement) parents.push(current);
      return parents.reverse().reduce(
        (result, parent) => {
          const color = rgba(getComputedStyle(parent).backgroundColor);
          return result.map((channel, i) => color[i] * color[3] + channel * (1 - color[3]));
        },
        [255, 255, 255],
      );
    };
    const luminance = (color) =>
      color.slice(0, 3).reduce((sum, channel, i) => {
        const value = channel / 255;
        return (
          sum +
          (value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4) *
            [0.2126, 0.7152, 0.0722][i]
        );
      }, 0);
    const ratio = (foreground, surface) => {
      const values = [luminance(foreground), luminance(surface)].sort((a, b) => b - a);
      return (values[0] + 0.05) / (values[1] + 0.05);
    };
    const failures = [];
    const selectors = [
      '.muted',
      '.prose p',
      '.panel-head h2',
      '.metric-label',
      '.metric-number',
      '.coverage-ring strong',
      '.legend span',
      '.badge',
      '.list-copy strong',
      '.list-copy small',
      '.connection',
      '.nav-link:not(:disabled)',
      '.button:not(:disabled)',
      '.text-link:not(:disabled)',
      '.file-item:not(:disabled)',
      '.tab:not(:disabled)',
      'summary',
    ];
    for (const element of document.querySelectorAll(selectors.join(','))) {
      if (!element.getClientRects().length || element.closest('[hidden]')) continue;
      const value = ratio(rgba(getComputedStyle(element).color), background(element));
      if (value < 4.5)
        failures.push(`${element.textContent.trim().slice(0, 60)}: ${value.toFixed(2)}:1 text`);
    }
    for (const element of document.querySelectorAll('.status-dot')) {
      if (!element.getClientRects().length) continue;
      const value = ratio(
        rgba(getComputedStyle(element).backgroundColor),
        background(element.parentElement),
      );
      if (value < 3) failures.push(`${element.title}: ${value.toFixed(2)}:1 indicator`);
    }
    for (const element of document.querySelectorAll(
      'input:not([type="checkbox"]):not(:disabled), textarea:not(:disabled), select.field:not(:disabled), .search-trigger',
    )) {
      if (!element.getClientRects().length || element.closest('[hidden]')) continue;
      const value = ratio(rgba(getComputedStyle(element).borderTopColor), background(element));
      if (value < 3) failures.push(`${element.tagName}: ${value.toFixed(2)}:1 control border`);
    }
    const focused = document.activeElement;
    if (
      focused?.matches(
        'button:focus-visible, input:focus-visible, select:focus-visible, textarea:focus-visible, summary:focus-visible, pre:focus-visible, .diff:focus-visible',
      )
    ) {
      const style = getComputedStyle(focused);
      const value = ratio(rgba(style.outlineColor), background(focused.parentElement));
      if (style.outlineStyle === 'none' || parseFloat(style.outlineWidth) < 2 || value < 3)
        failures.push(`Keyboard focus: ${style.outlineWidth}, ${value.toFixed(2)}:1`);
    }
    return failures;
  });
  assert.deepEqual(failures, [], `${name}: insufficient contrast`);
  checks++;
}
async function test(name, body) {
  await body();
  checks++;
  console.log(`PASS ${name}`);
}

try {
  await test('Redundant suggestion labels are removed while advisory content remains', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    assert.equal(
      await page
        .locator('.badge')
        .getByText(/^(ai suggestion|suggested)$/i)
        .count(),
      0,
    );
    await page.getByText('Retry failed work', { exact: true }).waitFor();
    await page.getByText('internal/worker/process.go · AI analysis', { exact: true }).waitFor();
    await nav(page, 'Bugs');
    assert.equal(
      await page
        .locator('.badge')
        .getByText(/^(ai suggestion|suggested)$/i)
        .count(),
      0,
    );
    await page.getByText(/AI analysis/).waitFor();
    await nav(page, 'Features');
    assert.equal(
      await page
        .locator('.badge')
        .getByText(/^(ai suggestion|suggested)$/i)
        .count(),
      0,
    );
    await page.getByRole('button', { name: 'Discuss in chat', exact: true }).waitFor();
    await close();
  });
  await test('Summary status dots distinguish empty success, incomplete and failed analysis', async () => {
    for (const [status, tone] of [
      ['completed', 'green'],
      ['completed_empty', 'green'],
      ['running', 'yellow'],
      ['partial', 'yellow'],
      ['stale', 'yellow'],
      ['unavailable', 'yellow'],
      ['failed', 'red'],
    ]) {
      const { page, close } = await pageFor({ runStatus: status });
      const dot = page.locator('[data-accent="bugs"] .status-dot');
      assert.equal(await dot.getAttribute('class'), `status-dot ${tone}`);
      assert.equal(await dot.getAttribute('aria-label'), `Bugs: ${status.replaceAll('_', ' ')}`);
      assert.equal(await page.locator('.metric-card .badge').count(), 0);
      if (status === 'completed_empty')
        assert.equal(await page.locator('[data-accent="bugs"] .metric-number').innerText(), '0');
      const bounds = await page.locator('.metric-card[data-accent="bugs"]').boundingBox();
      const position = await dot.boundingBox();
      assert.ok(position.x > bounds.x + bounds.width * 0.8);
      assert.ok(position.y < bounds.y + 40);
      await contrast(page, `Summary ${status}`);
      await page.getByRole('button', { name: 'Switch to light appearance' }).click();
      await contrast(page, `Summary ${status} light`);
      if (status === 'completed_empty') await layout(page, 'summary-empty-success');
      await close();
    }
  });
  await test('Saved architecture and flow charts render in Summary and the diagrams view', async () => {
    const { page, close } = await pageFor();
    const requests = await page.evaluate(() => window.fixture.requests.length);
    for (const name of ['Architecture diagram', 'Flow 1 diagram', 'Flow 2 diagram'])
      assert.equal(await page.getByRole('img', { name, exact: true }).count(), 0);
    await page.getByText('Show architecture', { exact: true }).focus();
    await page.keyboard.press('Enter');
    await page.getByText('Show project flows', { exact: true }).click();
    assert.equal(await page.evaluate(() => window.fixture.requests.length), requests);
    assert.equal(await page.locator('.diagram img').count(), 3);
    for (const name of ['Architecture diagram', 'Flow 1 diagram', 'Flow 2 diagram'])
      assert.equal(await page.getByRole('img', { name, exact: true }).count(), 1);
    await page.getByRole('heading', { name: 'Project flows', exact: true }).waitFor();
    for (const [name, file] of [
      ['Architecture diagram', 'architecture'],
      ['Flow 1 diagram', 'flowchart'],
      ['Flow 2 diagram', 'sequence'],
    ])
      await page.getByRole('img', { name, exact: true }).screenshot({
        path: `${output}/summary-${file}.png`,
      });
    const before = await page.evaluate(() =>
      window.fixture.requests.filter((request) => request.method !== 'GET'),
    );
    await page.getByRole('button', { name: 'Explore', exact: true }).click();
    await page.getByRole('heading', { name: 'Architecture & flows', exact: true }).waitFor();
    assert.equal(await page.locator('.diagram img').count(), 3);
    await page.waitForFunction(() =>
      [...document.querySelectorAll('.diagram img')].every(
        (image) => image.complete && image.naturalWidth > 0,
      ),
    );
    for (const source of await page.getByText('Diagram source', { exact: true }).all())
      await source.click();
    assert.match(await page.locator('.diagram pre').first().innerText(), /^flowchart TD/);
    assert.match(await page.locator('.diagram pre').last().innerText(), /^sequenceDiagram/);
    await layout(page, 'diagrams');
    await page.setViewportSize({ width: 900, height: 640 });
    await page.getByRole('button', { name: 'Larger text' }).click();
    await layout(page, 'diagrams-900');
    await page.getByRole('button', { name: 'Back to summary', exact: true }).click();
    assert.equal(await page.getByRole('img', { name: 'Architecture diagram' }).count(), 0);
    await layout(page, 'summary-charts-900');
    await expandSummaryDiagrams(page);
    for (const [name, file] of [
      ['Architecture diagram', 'architecture'],
      ['Flow 1 diagram', 'flowchart'],
      ['Flow 2 diagram', 'sequence'],
    ])
      await page.getByRole('img', { name, exact: true }).screenshot({
        path: `${output}/summary-${file}-900.png`,
      });
    assert.deepEqual(
      await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      ),
      before,
    );
    assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
    await close();
  });
  await test('Fenced charts and saved prose remain readable', async () => {
    const { page, close } = await pageFor({
      architecture:
        'The API admits requests into a bounded queue.\n\n```mermaid\ngraph LR\n  API --> Queue\n```',
      flows: ['``` Mermaid \r\nsequenceDiagram\r\n  API->>Worker: Process\r\n```'],
    });
    await expandSummaryDiagrams(page);
    assert.equal(await page.locator('.diagram img').count(), 2);
    await page
      .getByText('The API admits requests into a bounded queue.', { exact: true })
      .waitFor();
    await page.getByRole('button', { name: 'Explore', exact: true }).click();
    assert.equal(await page.locator('.diagram img').count(), 2);
    await page
      .getByText('The API admits requests into a bounded queue.', { exact: true })
      .waitFor();
    await close();

    const legacy = await pageFor({
      architecture: 'The API admits requests to the worker service.',
      flows: ['Each request is validated before the worker processes it.'],
    });
    await expandSummaryDiagrams(legacy.page);
    for (const explore of [false, true]) {
      if (explore) await legacy.page.getByRole('button', { name: 'Explore', exact: true }).click();
      assert.equal(await legacy.page.locator('.diagram img').count(), 0);
      await legacy.page
        .getByText('The API admits requests to the worker service.', { exact: true })
        .waitFor();
      await legacy.page
        .getByText('Each request is validated before the worker processes it.', { exact: true })
        .waitFor();
    }
    await legacy.close();
  });
  await test('Unavailable and invalid charts retain clear states and complete source', async () => {
    const missing = await pageFor({ architecture: '', flows: [] });
    await expandSummaryDiagrams(missing.page);
    await missing.page.getByText('No architecture overview saved.', { exact: true }).waitFor();
    await missing.page.getByRole('button', { name: 'Explore', exact: true }).click();
    await missing.page.getByText('No project flows saved.', { exact: true }).waitFor();
    assert.equal(await missing.page.locator('.diagram img').count(), 0);
    await missing.close();

    for (const [source, error] of [
      ['flowchart NOT_A_DIRECTION\n  API --> Worker', 'Diagram preview unavailable'],
      ['flowchart TD\n' + '%% comment\n'.repeat(160), 'Diagram is too large to render.'],
      ['flowchart TD\n  A["' + 'x'.repeat(16000) + '"]', 'Diagram is too large to render.'],
    ]) {
      const { page, close } = await pageFor({ architecture: source, flows: [] });
      await expandSummaryDiagrams(page);
      for (const explore of [false, true]) {
        if (explore) await page.getByRole('button', { name: 'Explore', exact: true }).click();
        await page.getByText(error, { exact: true }).waitFor();
        assert.equal(await page.locator('.diagram img').count(), 0);
        const saved = page.locator('.diagram pre');
        assert.equal(await saved.isVisible(), true);
        assert.equal(await saved.textContent(), source.trim());
        await saved.focus();
        assert.equal(await saved.evaluate((element) => element === document.activeElement), true);
      }
      await close();
    }
  });
  await test('Color hierarchy, readable themes and compact details preserve local navigation', async () => {
    const { page, close } = await pageFor({ unknown: true });
    assert.equal(await page.locator('[data-accent="performance"] .metric-number').innerText(), '—');
    const categories = await page
      .locator('.metric-card .metric-number')
      .evaluateAll((elements) => elements.map((element) => getComputedStyle(element).color));
    assert.equal(new Set(categories).size, 3);
    const before = await page.evaluate(() =>
      window.fixture.requests.filter((request) => request.method !== 'GET'),
    );
    assert.equal(await page.getByText('HTTP API', { exact: true }).isVisible(), false);
    await page.getByText('Components', { exact: true }).click();
    assert.equal(await page.getByText('HTTP API', { exact: true }).isVisible(), true);
    const why = page.getByText(
      'Workers otherwise keep consuming resources after their caller has left.',
      {
        exact: true,
      },
    );
    assert.equal(await why.isVisible(), false);
    await page.getByText('Why & tradeoffs', { exact: true }).click();
    assert.equal(await why.isVisible(), true);
    assert.deepEqual(
      await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      ),
      before,
    );
    await page.getByText('Components', { exact: true }).click();
    await page.getByText('Why & tradeoffs', { exact: true }).click();
    for (const theme of ['dark', 'light']) {
      if (theme === 'light')
        await page.getByRole('button', { name: 'Switch to light appearance' }).click();
      for (const name of [
        'Summary',
        'Analysis',
        'Bugs',
        'Performance',
        'Security',
        'Models',
        'Project',
      ]) {
        await nav(page, name);
        await contrast(page, `${name}-${theme}`);
      }
      await nav(page, 'Summary');
      await page.getByRole('button', { name: 'Analyze project', exact: true }).hover();
      await contrast(page, `Primary action hover-${theme}`);
      await openSource(page);
      await page.getByRole('tab', { name: 'Source', exact: true }).focus();
      await page.keyboard.press('Tab');
      assert.equal(
        await page
          .getByRole('tab', { name: 'Context' })
          .evaluate((element) => element === document.activeElement),
        true,
      );
      await contrast(page, `Editor keyboard focus-${theme}`);
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: 'Included files' }).waitFor();
      await contrast(page, `Context-${theme}`);
      await nav(page, 'Summary');
      await page.setViewportSize({ width: 900, height: 640 });
      await page.getByRole('button', { name: 'Larger text' }).click();
      await layout(page, `matrix-summary-${theme}-900`);
      await contrast(page, `Summary-${theme}-large-text`);
      assert.deepEqual(
        await page.locator('.sidebar .nav-link').evaluateAll((buttons) =>
          buttons
            .filter((button) => {
              const rect = button.getBoundingClientRect();
              return rect.top < 0 || rect.bottom > window.innerHeight;
            })
            .map((button) => button.getAttribute('aria-label')),
        ),
        [],
        'Compact navigation must remain visible',
      );
      await page.getByRole('button', { name: 'Larger text' }).click();
      await page.setViewportSize({ width: 1440, height: 1000 });
    }
    assert.deepEqual(
      await page.evaluate(() =>
        window.fixture.requests
          .filter((request) => request.method !== 'GET')
          .map((request) => request.path),
      ),
      ['/api/projects/restore'],
    );
    await close();
  });
  await test('Navigation and disclosure do not dispatch providers, code, source writes or shells', async () => {
    const { page, close } = await pageFor({ unknown: true });
    await layout(page, 'summary');
    for (const name of [
      'Analysis',
      'Bugs',
      'Performance',
      'Security',
      'Features',
      'Instructions',
      'Chat',
      'Source',
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
    await contrast(page, 'Review and Apply');
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
  await test('Analysis exposes provider progress and keeps saved results available', async () => {
    const { page, close } = await pageFor();
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
    await idle(page);
    await page.getByLabel('Include AI Security review').check();
    await page.getByRole('button', { name: 'Start analysis', exact: true }).click();
    await idle(page);
    await page
      .getByRole('heading', { name: 'New feature suggestions', exact: true })
      .locator('..')
      .getByRole('img', { name: 'Features: pending', exact: true })
      .waitFor();
    await page.evaluate(() => {
      window.fixture.state.run.features.status = 'running';
      window.fixture.state.run.features.attempts = 1;
      window.fixture.state.run.elapsed_seconds = 75;
    });
    const activity = page.getByRole('status', { name: 'Current analysis step' });
    await activity.getByText('Generating feature suggestions…', { exact: true }).waitFor();
    await page.getByText('75s elapsed', { exact: true }).waitFor();
    await page.getByText('Advisory ideas · 1 of 2 attempts used', { exact: true }).waitFor();
    await layout(page, 'analysis-feature-progress');
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.evaluate(() => document.documentElement.classList.add('large-text'));
    await layout(page, 'analysis-feature-progress-large-text');
    await page.getByRole('button', { name: 'Open results', exact: true }).first().click();
    await page.waitForFunction(() =>
      window.fixture.requests.some((r) => r.path.endsWith('/analysis/results')),
    );
    assert.equal(await page.evaluate(() => window.fixture.state.run.features.status), 'running');
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'View run', exact: true }).click();
    await page.evaluate(() => {
      window.fixture.state.run.status = 'partial';
      window.fixture.state.run.features.status = 'failed';
      window.fixture.state.run.features.reason =
        'The model request or response failed. Other analysis results remain available.';
    });
    await page
      .getByText('The model request or response failed. Other analysis results remain available.', {
        exact: true,
      })
      .waitFor();
    assert.equal(await activity.count(), 0, 'Settled runs must stop showing active progress');
    assert.equal(await page.getByRole('button', { name: 'Open results' }).count(), 3);
    assert.equal(
      await page.evaluate(
        () =>
          window.fixture.requests.filter(
            (r) => r.method === 'POST' && r.path.endsWith('/analysis/run'),
          ).length,
      ),
      1,
      'Polling and opening saved results must not start another analysis',
    );
    await layout(page, 'analysis-feature-failure');
    await close();
  });
  await test('Analysis progress identifies the active file and stage', async () => {
    const { page, close } = await pageFor({ runStatus: 'running' });
    await page.evaluate(() => {
      window.fixture.state.run.files[0].stages[1].status = 'running';
    });
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'View run', exact: true }).click();
    await page
      .getByRole('status', { name: 'Current analysis step' })
      .getByText('Performance · internal/worker/process.go', { exact: true })
      .waitFor();
    await layout(page, 'analysis-file-progress');
    await close();
  });
  await test('Late file responses cannot replace the current file', async () => {
    const { page, close } = await pageFor();
    await nav(page, 'Source');
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
    await nav(page, 'Source');
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
      for (const name of ['Summary', 'Analysis', 'Bugs', 'Source', 'Models', 'Project']) {
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
      await contrast(page, `Results-${options.runStatus || 'empty'}`);
      await close();
    }
  });
  await test('Chat revisions require fresh review before grouped Apply and Undo', async () => {
    const { page, close } = await pageFor();
    await nav(page, 'Chat');
    await page.getByLabel('Add an existing file').selectOption('internal/worker/process.go');
    await page.getByLabel('Change request', { exact: true }).fill('Handle cancellation.');
    await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Trust this project' }).click();
    await idle(page);
    await page.getByLabel('Read-only diff for internal/worker/process.go').waitFor();
    assert.equal(await page.getByRole('button', { name: 'Approve and apply' }).isEnabled(), false);
    await page.getByRole('button', { name: 'Review this diff' }).click();
    await idle(page);
    assert.equal(await page.getByRole('button', { name: 'Approve and apply' }).isEnabled(), true);
    await page.getByLabel('Change request', { exact: true }).fill('Preserve the existing API too.');
    await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
    await idle(page);
    assert.equal(await page.getByRole('button', { name: 'Approve and apply' }).isEnabled(), false);
    await page.getByRole('button', { name: 'Review this diff' }).click();
    await idle(page);
    await layout(page, 'chat-review');
    await page.getByRole('button', { name: 'Approve and apply' }).click();
    await page
      .getByRole('dialog')
      .getByRole('button', { name: 'Apply proposal', exact: true })
      .click();
    await idle(page);
    await page.getByRole('heading', { name: 'Change applied', exact: true }).waitFor();
    assert.equal(
      await page.evaluate(
        () =>
          window.fixture.requests.filter((r) => r.path.endsWith('/changes/change-1/apply')).length,
      ),
      1,
    );
    await page.getByRole('button', { name: 'Undo proposal', exact: true }).click();
    await page
      .getByRole('dialog')
      .getByRole('button', { name: 'Undo proposal', exact: true })
      .click();
    await idle(page);
    await page.getByRole('heading', { name: 'Change undone', exact: true }).waitFor();
    await close();
  });
  await test('Remote chat consent and passive history restore do not restore approval', async () => {
    const { page, close } = await pageFor({ remote: true });
    await nav(page, 'Chat');
    await page.getByLabel('Add an existing file').selectOption('internal/worker/process.go');
    await page.getByLabel('Change request', { exact: true }).fill('Add cancellation.');
    await page.getByLabel('Run project tests after generation').uncheck();
    await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Review this diff' }).click();
    await idle(page);
    await page.getByRole('button', { name: 'New conversation' }).click();
    await page.getByText('Local history', { exact: true }).click();
    await page.getByRole('button', { name: 'Refresh history' }).click();
    await page.getByRole('button', { name: 'Resume', exact: true }).click();
    await idle(page);
    assert.equal(await page.getByRole('button', { name: 'Approve and apply' }).isEnabled(), false);
    assert.equal(
      await page.evaluate(
        () =>
          window.fixture.requests.filter(
            (r) => r.path.endsWith('/messages') && r.path.includes('/changes/'),
          ).length,
      ),
      1,
    );
    await page.setViewportSize({ width: 800, height: 900 });
    await layout(page, 'chat-compact');
    await page.getByRole('button', { name: 'Larger text' }).click();
    await layout(page, 'chat-large-text');
    await close();
  });
  await test('Prepare fix stops after bounded repair and never applies automatically', async () => {
    const { page, close } = await pageFor({ changeChecksFail: true });
    await nav(page, 'Bugs');
    await page.locator('.result-row').first().click();
    await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Trust this project' }).click();
    await idle(page);
    const requests = await page.evaluate(() => window.fixture.requests);
    assert.equal(requests.filter((r) => r.body?.repair === true).length, 3);
    assert.equal(
      requests.some((r) => r.path.endsWith('/apply')),
      false,
    );
    assert.equal(await page.getByRole('button', { name: 'Review this diff' }).isDisabled(), true);
    await close();
  });
  await test('Canceled chat responses do not publish a late proposal', async () => {
    const { page, close } = await pageFor();
    await nav(page, 'Chat');
    await page.getByLabel('Add an existing file').selectOption('internal/worker/process.go');
    await page.getByLabel('Change request', { exact: true }).fill('Handle cancellation.');
    await page.getByLabel('Run project tests after generation').uncheck();
    await page.evaluate(() => {
      window.fixture.hold = '/api/projects/current/changes/change-1/messages';
    });
    await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
    await page.waitForFunction(() =>
      window.fixture.requests.some(
        (r) => r.path.endsWith('/messages') && r.path.includes('/changes/'),
      ),
    );
    await page.locator('.busy-strip').getByRole('button', { name: 'Cancel', exact: true }).click();
    await page.evaluate(() => {
      window.fixture.hold = '';
      window.fixture.release();
    });
    await idle(page);
    assert.equal(await page.getByLabel('Read-only diff for internal/worker/process.go').count(), 0);
    await close();
  });
  await test('Summary reads advisory ideas and Discuss only seeds Chat', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    await page.getByRole('heading', { name: 'New feature suggestions', exact: true }).waitFor();
    await page.getByRole('heading', { name: 'Retry failed work', exact: true }).waitFor();
    await page.getByText('Estimated effort: medium', { exact: true }).waitFor();
    const requests = await page.evaluate(() => window.fixture.requests);
    assert.equal(
      requests.some((r) => r.path.endsWith('/features') && r.method === 'GET'),
      true,
    );
    assert.equal(
      requests.some((r) => r.method !== 'GET' && r.path !== '/api/projects/restore'),
      false,
    );
    await page.getByRole('button', { name: 'View all ideas', exact: true }).click();
    await page.getByRole('heading', { name: 'Features', exact: true }).waitFor();
    await nav(page, 'Summary');
    await page.getByRole('button', { name: 'Discuss in chat', exact: true }).click();
    await page.getByLabel('Change request', { exact: true }).waitFor();
    assert.match(
      await page.getByLabel('Change request', { exact: true }).inputValue(),
      /Retry only eligible failed jobs/,
    );
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some(
          (r) => r.method !== 'GET' && r.path !== '/api/projects/restore',
        ),
      ),
      false,
    );
    await close();
  });
  await test('Saved suggestion states stay available without unsolicited notices', async () => {
    for (const options of [
      {},
      { featuresReadFail: true },
      { featuresStale: true },
      { featuresReady: true, featuresEmpty: true },
      { featuresReady: true, featuresFail: true },
      { featuresReady: true, featuresStale: true },
      { featuresReady: true, featuresFail: true, featuresStale: true },
      {
        empty: true,
        featuresReady: true,
        featuresEmpty: true,
        featuresFail: true,
        featuresStale: true,
      },
    ]) {
      const { page, close } = await pageFor(options);
      await idle(page);
      await page.getByRole('heading', { name: 'New feature suggestions', exact: true }).waitFor();
      if (options.featuresReadFail)
        await page.getByRole('heading', { name: 'Features unavailable', exact: true }).waitFor();
      else if (!options.featuresReady)
        await page.getByRole('heading', { name: 'No features yet', exact: true }).waitFor();
      else if (options.featuresEmpty)
        await page
          .getByRole('heading', {
            name: options.featuresFail ? 'No saved features' : 'No new features found',
            exact: true,
          })
          .waitFor();
      else {
        await page.getByRole('heading', { name: 'Retry failed work', exact: true }).waitFor();
        if (options.featuresStale)
          assert.equal(
            await page.getByRole('button', { name: 'Discuss in chat', exact: true }).isDisabled(),
            true,
          );
      }
      assert.equal(await page.getByText(/Feature search failed/).count(), 0);
      assert.equal(await page.getByText(/Ideas are outdated/).count(), 0);
      await page.setViewportSize({ width: 800, height: 900 });
      await page.getByRole('button', { name: 'Larger text' }).click();
      await layout(page, `summary-ideas-${JSON.stringify(options)}`);
      await contrast(page, 'Summary feature ideas');
      await nav(page, 'Features');
      await page.getByRole('heading', { name: 'Features', exact: true }).waitFor();
      const reads = await page.evaluate(
        () => window.fixture.requests.filter((r) => r.path.endsWith('/features')).length,
      );
      await page.getByRole('button', { name: 'Refresh suggestions', exact: true }).click();
      await page.waitForFunction(
        (reads) =>
          window.fixture.requests.filter((r) => r.path.endsWith('/features')).length > reads,
        reads,
      );
      assert.equal(await page.getByText(/Feature search failed/).count(), 0);
      assert.equal(await page.getByText(/Ideas are outdated/).count(), 0);
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some(
            (r) => r.method !== 'GET' && r.path !== '/api/projects/restore',
          ),
        ),
        false,
      );
      await close();
    }
  });
  await test('Saving goals and declining suggestion consent keep saved failures quiet', async () => {
    const { page, close } = await pageFor({
      remote: true,
      featuresReady: true,
      featuresFail: true,
      featuresStale: true,
    });
    await nav(page, 'Features');
    await page.getByLabel('Project goals', { exact: true }).fill('Help recover failed jobs.');
    await page.getByRole('button', { name: 'Save goals', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Suggest features', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
    await idle(page);
    assert.equal(await page.getByText(/Feature search failed/).count(), 0);
    assert.equal(await page.getByText(/Ideas are outdated/).count(), 0);
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/features/generate')),
      ),
      false,
    );
    await nav(page, 'Summary');
    assert.equal(await page.getByText(/Feature search failed/).count(), 0);
    assert.equal(await page.getByText(/Ideas are outdated/).count(), 0);
    await close();
  });
  await test('Project analysis includes feature suggestions in preview, consent and Summary', async () => {
    const { page, close } = await pageFor({ remote: true, analysisCompletes: true });
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
    await idle(page);
    await page.getByRole('heading', { name: 'New feature suggestions', exact: true }).waitFor();
    assert.equal(
      await page.getByRole('button', { name: 'Start analysis', exact: true }).isDisabled(),
      true,
    );
    await page.getByLabel('Allow selected context to this provider').check();
    await page.getByLabel('Include AI Security review').check();
    await page.getByRole('button', { name: 'Start analysis', exact: true }).click();
    await idle(page);
    const requests = await page.evaluate(() => window.fixture.requests);
    assert.equal(
      requests.find((r) => r.path.endsWith('/analysis/preview')).body.include_features,
      true,
    );
    const start = requests.find((r) => r.path.endsWith('/analysis/run') && r.method === 'POST');
    assert.equal(start.body.include_features, true);
    assert.deepEqual(start.body.confirmations.provider_ids, ['provider-1']);
    await page.getByRole('heading', { name: 'New feature suggestions', exact: true }).waitFor();
    await layout(page, 'analysis-with-features');
    await nav(page, 'Summary');
    await page.getByRole('heading', { name: 'Retry failed work', exact: true }).waitFor();
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/features/generate')),
      ),
      false,
    );
    await layout(page, 'summary-analysis-features');
    await close();
  });
  await test('Feature goals and idea triage are local; Discuss only seeds Chat', async () => {
    const { page, close } = await pageFor({ remote: true });
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'Explore features' }).click();
    await page.getByRole('heading', { name: 'No features yet' }).waitFor();
    await page
      .getByLabel('Project goals', { exact: true })
      .fill('Help operators recover failed jobs.');
    await page.getByRole('button', { name: 'Save goals', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/features/generate')),
      ),
      false,
    );
    await page.getByRole('button', { name: 'Suggest features', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click();
    await idle(page);
    await page.getByRole('heading', { name: 'Retry failed work', exact: true }).waitFor();
    await page.getByRole('button', { name: 'Save idea', exact: true }).click();
    await idle(page);
    await page.getByLabel('Filter feature suggestions').selectOption('saved');
    await layout(page, 'features-saved');
    await contrast(page, 'Feature suggestions');
    await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
    await idle(page);
    await page.getByRole('heading', { name: 'No matching features' }).waitFor();
    await page.getByLabel('Filter feature suggestions').selectOption('dismissed');
    await page.getByRole('button', { name: 'Reopen', exact: true }).click();
    await idle(page);
    await page.getByLabel('Filter feature suggestions').selectOption('active');
    await page.getByRole('button', { name: 'Discuss in chat', exact: true }).click();
    await page.getByLabel('Change request', { exact: true }).waitFor();
    assert.match(
      await page.getByLabel('Change request', { exact: true }).inputValue(),
      /Retry only eligible failed jobs/,
    );
    assert.equal(
      await page.evaluate(() => window.fixture.requests.some((r) => r.path.endsWith('/messages'))),
      false,
    );
    await close();
  });
  await test('Instruction wizard preserves inherited rules and requires diff review and Apply', async () => {
    const { page, close } = await pageFor();
    await nav(page, 'Instructions');
    await page.getByLabel('Instruction path').fill('internal/AGENTS.md');
    await page.getByRole('button', { name: 'Load scope', exact: true }).click();
    await page.getByLabel('Custom instructions').waitFor();
    assert.match(
      await page.getByLabel('Custom instructions').inputValue(),
      /Propagate cancellation/,
    );
    await page.getByText('AGENTS.md · scope .', { exact: true }).click();
    await page.getByText('Preserve public APIs.', { exact: true }).waitFor();
    await page.getByLabel('Keep changes focused', { exact: true }).check();
    await page.getByRole('button', { name: 'Add selected guidance', exact: true }).click();
    const content = await page.getByLabel('Custom instructions').inputValue();
    await page
      .getByLabel('Custom instructions')
      .fill(`${content}\nUse table-driven tests for boundary cases.\n`);
    await layout(page, 'instructions-guidance');
    await contrast(page, 'Instruction wizard');
    await page.setViewportSize({ width: 900, height: 640 });
    await page.getByRole('button', { name: 'Larger text' }).click();
    await layout(page, 'instructions-large-text');
    await page.getByRole('button', { name: 'Continue to preview', exact: true }).click();
    await page.getByRole('button', { name: 'Preview instruction diff', exact: true }).click();
    await idle(page);
    await page.getByLabel('Read-only diff for internal/AGENTS.md').waitFor();
    const before = await page.evaluate(() => window.fixture.requests);
    assert.equal(
      before.some(
        (r) =>
          r.path.endsWith('/messages') ||
          r.path.endsWith('/apply') ||
          (r.path.endsWith('/execution-trust') && r.method === 'POST'),
      ),
      false,
    );
    assert.equal(await page.getByRole('button', { name: 'Approve and apply' }).isDisabled(), true);
    await page.getByRole('button', { name: 'Review this diff' }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Approve and apply' }).click();
    await page
      .getByRole('dialog')
      .getByRole('button', { name: 'Apply proposal', exact: true })
      .click();
    await idle(page);
    const instructions = await page.evaluate(() => window.fixture.state.instructions);
    assert.equal(instructions['AGENTS.md'], '# Project rules\n\nPreserve public APIs.\n');
    assert.match(
      instructions['internal/AGENTS.md'],
      /Propagate cancellation[\s\S]*Preserve unrelated work[\s\S]*table-driven tests/,
    );
    await close();
  });
  await test('Requested feature results show failures and warn only about existing stale ideas', async () => {
    for (const options of [
      { featuresEmpty: true },
      { featuresFail: true, featuresStale: true },
      { featuresStale: true },
      { featuresReady: true, featuresFail: true, featuresStale: true },
    ]) {
      const { page, close } = await pageFor(options);
      await nav(page, 'Features');
      await page.getByRole('button', { name: 'Suggest features' }).click();
      await idle(page);
      if (options.featuresEmpty)
        await page
          .getByRole('heading', {
            name: options.featuresFail ? 'No saved features' : 'No new features found',
          })
          .waitFor();
      if (options.featuresFail)
        await page.getByText(/^Feature search failed\. Try again\./).waitFor();
      const hasIdeas = !options.featuresEmpty && (!options.featuresFail || options.featuresReady);
      if (options.featuresStale && hasIdeas)
        assert.equal(
          await page.getByRole('button', { name: 'Discuss in chat' }).isDisabled(),
          true,
        );
      assert.equal(
        await page.getByText(/Ideas are outdated/).count(),
        options.featuresStale && hasIdeas ? 1 : 0,
      );
      await page.setViewportSize({ width: 800, height: 900 });
      await page.getByRole('button', { name: 'Larger text' }).click();
      await layout(page, `features-${JSON.stringify(options)}`);
      await nav(page, 'Summary');
      if (options.featuresFail)
        await page
          .getByText(
            `Feature search failed. Try again.${hasIdeas ? ' Previous ideas kept.' : ''}`,
            { exact: true },
          )
          .waitFor();
      assert.equal(
        await page.getByText(/Ideas are outdated/).count(),
        options.featuresStale && hasIdeas ? 1 : 0,
      );
      await nav(page, 'Project');
      await page.getByRole('button', { name: 'Open saved project', exact: true }).click();
      await idle(page);
      await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
      assert.equal(await page.getByText(/Feature search failed/).count(), 0);
      assert.equal(await page.getByText(/Ideas are outdated/).count(), 0);
      await close();
    }
  });
  await test('Feature generation failures from a requested analysis remain visible', async () => {
    for (const featuresReady of [false, true]) {
      const { page, close } = await pageFor({
        analysisCompletes: true,
        featuresReady,
        featuresFail: true,
        featuresStale: true,
      });
      await nav(page, 'Analysis');
      await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
      await idle(page);
      await page.getByLabel('Include AI Security review').check();
      await page.getByRole('button', { name: 'Start analysis', exact: true }).click();
      await idle(page);
      await nav(page, 'Summary');
      await page
        .getByText(
          `Feature search failed. Try again.${featuresReady ? ' Previous ideas kept.' : ''}`,
          { exact: true },
        )
        .waitFor();
      assert.equal(await page.getByText(/Ideas are outdated/).count(), featuresReady ? 1 : 0);
      await nav(page, 'Features');
      await page.getByText(/^Feature search failed\. Try again\./).waitFor();
      assert.equal(await page.getByText(/Ideas are outdated/).count(), featuresReady ? 1 : 0);
      await close();
    }
  });
  await test('Applied changes require explicit verification and separate reanalysis consent', async () => {
    for (const verificationFail of [false, true]) {
      const mutationWarning = 'Source changed; history could not be updated.';
      const { page, close } = await pageFor({
        remote: true,
        verificationFail,
        mutationWarning,
        recoveryDropsWarnings: true,
      });
      await nav(page, 'Chat');
      await page.getByLabel('Add an existing file').selectOption('internal/worker/process.go');
      await page.getByLabel('Change request', { exact: true }).fill('Handle cancellation.');
      await page.getByLabel('Run project tests after generation').uncheck();
      await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
      await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click();
      await idle(page);
      await page.getByRole('button', { name: 'Review this diff' }).click();
      await idle(page);
      await page.getByRole('button', { name: 'Approve and apply' }).click();
      await page
        .getByRole('dialog')
        .getByRole('button', { name: 'Apply proposal', exact: true })
        .click();
      await idle(page);
      await page.getByText(mutationWarning, { exact: true }).waitFor();
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some(
            (r) =>
              r.path.endsWith('/verify') ||
              (r.path.endsWith('/files/analysis') && r.method === 'POST'),
          ),
        ),
        false,
      );
      await page.getByRole('button', { name: 'Verify applied change' }).click();
      await page.getByRole('dialog').getByRole('button', { name: 'Trust this project' }).click();
      await idle(page);
      await page
        .getByLabel('Post-Apply verification')
        .getByText(verificationFail ? 'failed' : 'verified', { exact: true })
        .first()
        .waitFor();
      await layout(page, verificationFail ? 'chat-verification-failed' : 'chat-verified');
      await page.getByRole('button', { name: 'Reanalyze changed files' }).click();
      await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click();
      await idle(page);
      const requests = await page.evaluate(() => window.fixture.requests);
      assert.equal(
        requests.filter((r) => r.path.endsWith('/files/analysis') && r.method === 'POST').length,
        1,
      );
      assert.equal(
        requests.some((r) => r.method === 'PATCH' && r.path.includes('/findings/')),
        false,
      );
      await page.getByText('Reanalyzed 1 changed files.', { exact: false }).waitFor();
      await close();
    }
  });
  assert.deepEqual(errors, []);
  console.log(`PASS ${checks} workflow and layout checks; no browser errors or external requests`);
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
