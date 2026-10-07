import { chromium } from '@playwright/test';
import { strict as assert } from 'node:assert';
import { mkdir, writeFile } from 'node:fs/promises';
import { serve } from '../preview.mjs';
import { installFixture } from './fixture.mjs';

let server;
let url;
let browser;
const output = new URL('../test-results/', import.meta.url).pathname;
await mkdir(output, { recursive: true });
let checks = 0;
const errors = [];
// Planned presentation states, not an attestation of coverage. Captures below record
// what actually ran; each owner extends its cases as its surfaces are migrated.
const surfaceInventory = {
  overview: {
    summary: ['reference', 'empty', 'unavailable', 'stale', 'failed', 'long identity'],
    welcome: ['no project', 'offline', 'opening', 'open failed'],
    project: ['no project', 'loaded', 'opening', 'open failed', 'long paths'],
    models: ['configured', 'unavailable', 'empty', 'busy', 'long destinations'],
    search: ['files and commands', 'filtered empty', 'large list', 'long paths'],
    diagrams: ['rendered', 'empty', 'prose', 'invalid', 'oversized', 'source disclosure'],
  },
  analysis: {
    analysis: ['setup', 'no run', 'saved run', 'busy', 'unavailable models', 'dirty selection'],
    'analysis-preview': [
      'new',
      'repair',
      'continuation',
      'absent',
      'consent',
      'mismatch',
      'long paths and destinations',
      'empty scope',
      'feature-only scope',
      'unavailable captured models',
      'unavailable captured provider',
    ],
    'analysis-run': [
      'absent',
      'queued',
      'running',
      'pausing',
      'paused',
      'interrupted',
      'canceling',
      'canceled',
      'failed',
      'partial',
      'completed',
      'unknown counts',
    ],
  },
  results: {
    bugs: [
      'list',
      'detail',
      'filtered empty',
      'unavailable',
      'read failed',
      'retained stale',
      'partial',
      'pagination',
      'unclassified',
    ],
    performance: [
      'list',
      'typed detail',
      'semantic detail',
      'filtered empty',
      'unavailable',
      'read failed',
      'retained stale',
      'partial',
      'pagination',
      'unclassified',
    ],
    security: [
      'list',
      'typed detail',
      'semantic detail',
      'filtered empty',
      'unavailable',
      'read failed',
      'retained stale',
      'partial',
      'pagination',
      'unclassified',
      'selected source',
    ],
  },
  features: {
    features: [
      'goals',
      'suggestions',
      'triage filters',
      'context and generation metadata',
      'long content',
      'empty',
      'unavailable',
      'requested failure with retained ideas',
      'stale',
      'busy',
    ],
  },
  'change-workspace': {
    chat: [
      'new',
      'seeded',
      'restored',
      'history unavailable',
      'busy',
      'failed',
      'canceled',
      'proposal',
      'checks failed',
      'reviewed',
      'stale',
      'receipt applied/undone/prepared/recovery required',
      'verification absent/verified/failed/unavailable',
      'long warnings and diagnostics',
      'busy receipt actions',
      'uncertain write and refreshed recovery',
    ],
  },
  instructions: {
    instructions: [
      'scope',
      'edit',
      'preview',
      'guidance',
      'excluded',
      'stale',
      'read failed',
      'loading',
      'busy',
      'long root and directory scope',
      'unchanged preview',
    ],
  },
  editor: {
    editor: ['no file', 'source', 'binary', 'file analysis', 'unavailable', 'busy', 'stale'],
    context: ['populated', 'empty', 'unavailable', 'read failed', 'truncated', 'long metadata'],
    // main.tsx dispatches both to Editor; Editor renders Context and selects its tab
    // for both; Workspace.navigate calls inspectContext for either. No alias control.
    manifest: ['Context alias: same rendering, loading and selected tab; no separate control'],
    assistant: [
      'request and presets',
      'long explanation and conversation',
      'optional constraints',
      'target changes',
      'stale',
      'busy',
      'canceled',
    ],
    'new-declaration': [
      'function/type/var/const',
      'long explanation and conversation',
      'optional constraints',
      'stale',
      'busy',
      'canceled',
    ],
    draft: ['absent', 'generated', 'edited', 'validated', 'invalid', 'stale', 'busy'],
    checks: ['absent', 'passed', 'failed', 'skipped', 'canceled', 'busy', 'stale'],
    review: ['ready', 'dirty', 'missing validation', 'failed checks', 'stale', 'busy', 'uncertain'],
  },
  tools: {
    receipt: ['absent', 'applied', 'undone', 'optional audit unavailable', 'busy', 'uncertain'],
    benchmark: [
      'catalog',
      'empty',
      'unavailable',
      'samples',
      'failed',
      'canceled',
      'busy',
      'stale',
    ],
    scan: ['absent', 'running', 'completed', 'failed', 'partial', 'canceled', 'unknown phases'],
    terminal: [
      'idle',
      'active',
      'multiple tabs',
      'busy',
      'launch failed',
      'exited',
      'cleanup failed',
    ],
  },
};
const aliases = { welcome: 'project', manifest: 'context' };
const captures = [];
async function pageFor(options = {}) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  try {
    const page = await context.newPage();
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('request', (request) => {
      if (!request.url().startsWith(url) && !request.url().startsWith('data:'))
        errors.push(`Unexpected outbound request: ${request.url()}`);
    });
    await page.addInitScript(installFixture, options);
    await page.goto(url);
    await page
      .getByRole('heading', { name: options.projectName || 'harbor', exact: true })
      .waitFor();
    await page.waitForFunction(() =>
      window.fixture.requests.some((request) => request.path?.endsWith('/analysis/selection')),
    );
    return { page, close: () => context.close() };
  } catch (error) {
    await context.close();
    throw error;
  }
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
async function startAnalysis(page, resume = false) {
  await page
    .getByRole('button', { name: resume ? 'Resume analysis' : 'Start analysis', exact: true })
    .click();
  await page.getByRole('dialog').getByRole('button', { name: 'Start', exact: true }).click();
  await idle(page);
}
async function openSummaryDiagrams(page) {
  await page.locator('.metric-card[data-accent="diagrams"]').click();
  await page.getByRole('heading', { name: 'Architecture and Flow', exact: true }).waitFor();
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
  const route = await page.locator('.page[data-accent]').getAttribute('data-accent');
  const owner = Object.keys(surfaceInventory).find((owner) => route in surfaceInventory[owner]);
  assert.ok(owner, `${name}: route ${route} belongs to the presentation inventory`);
  const screenshot = `${output}/${name}.png`;
  await page.screenshot({ path: screenshot, fullPage: true });
  captures.push({
    route,
    owner,
    stateCase: name,
    viewport: page.viewportSize(),
    theme: await page.locator('html').getAttribute('data-theme'),
    textSize:
      (await page
        .getByRole('button', { name: 'Larger text', exact: true })
        .getAttribute('aria-pressed')) === 'true'
        ? 'larger'
        : 'standard',
    screenshot,
  });
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
async function headingContainment(heading) {
  const bounds = await heading.boundingBox();
  assert.ok(bounds, 'Heading is rendered');
  const parts = [
    heading.locator('h1'),
    ...(await heading.locator('p').all()),
    ...(await heading.getByRole('button').all()),
  ];
  const boxes = [];
  for (const part of parts) {
    assert.equal(await part.isVisible(), true, 'Heading content remains visible');
    const box = await part.boundingBox();
    assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
    assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    for (const other of boxes)
      assert.ok(
        box.x + box.width <= other.x + 1 ||
          other.x + other.width <= box.x + 1 ||
          box.y + box.height <= other.y + 1 ||
          other.y + other.height <= box.y + 1,
        'Heading title, detail and actions do not collide',
      );
    assert.equal(
      await part.evaluate((element) => element.scrollWidth > element.clientWidth + 1),
      false,
      'Heading text is not clipped',
    );
    boxes.push(box);
  }
  const clippedText = await heading.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
    const clipped = [];
    while (walker.nextNode()) {
      if (!walker.currentNode.textContent.trim()) continue;
      const range = document.createRange();
      range.selectNodeContents(walker.currentNode);
      if (
        [...range.getClientRects()].some(
          (rect) =>
            rect.left < bounds.left - 1 ||
            rect.right > bounds.right + 1 ||
            rect.top < bounds.top - 1 ||
            rect.bottom > bounds.bottom + 1,
        )
      )
        clipped.push(walker.currentNode.textContent);
    }
    return clipped;
  });
  assert.deepEqual(clippedText, [], 'Complete heading text stays within the introduction');
  checks++;
}
async function introductionTreatment(surface) {
  return surface.evaluate((element) => {
    const style = getComputedStyle(element);
    const button = getComputedStyle(element.querySelector('.button'));
    return {
      background: style.backgroundColor,
      border: style.border,
      radius: style.borderRadius,
      padding: style.padding,
      shadow: style.boxShadow,
      buttonHeight: button.minHeight,
      buttonRadius: button.borderRadius,
    };
  });
}
async function panelTreatment(panel) {
  return panel.evaluate((element) => {
    const style = getComputedStyle(element);
    return {
      background: style.backgroundColor,
      border: style.border,
      radius: style.borderRadius,
      headerPadding: getComputedStyle(element.querySelector('.panel-head')).padding,
      titleSize: getComputedStyle(element.querySelector('.panel-head h2')).fontSize,
      bodyPadding: getComputedStyle(element.querySelector('.panel-body')).padding,
    };
  });
}
async function focusedChecksLayout(page) {
  const surface = page.locator('.checks-workspace');
  assert.equal(await surface.evaluate((element) => getComputedStyle(element).gap), '20px');
  const overflow = await page
    .locator(
      '#main, .page, .checks-workspace, .checks-workspace .panel, .checks-workspace .panel-head, .checks-workspace .panel-body, .checks-workspace .disclosure-body, .checks-workspace pre',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.id || element.getAttribute('aria-label')),
    );
  assert.deepEqual(overflow, [], 'Focused controls, commands and complete output stay contained');
  for (const panel of await surface.locator('.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const control of await panel.locator('button, label, summary, pre').all()) {
      if (!(await control.isVisible())) continue;
      const box = await control.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
  }
  const boxes = [];
  for (const control of await surface.locator('.button, .checkbox-line').all()) {
    assert.equal(await control.isVisible(), true);
    const box = await control.boundingBox();
    const bounds = await surface.boundingBox();
    assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
    for (const other of boxes)
      assert.ok(
        box.x + box.width <= other.x + 1 ||
          other.x + other.width <= box.x + 1 ||
          box.y + box.height <= other.y + 1 ||
          other.y + other.height <= box.y + 1,
        'Focused-check actions and optional controls do not collide',
      );
    boxes.push(box);
  }
  checks++;
}
async function assistantLayout(page) {
  const composition = page.locator('.assistant-composition');
  await headingContainment(page.locator('.source-workspace .page-heading--intro'));
  assert.equal(await composition.evaluate((element) => getComputedStyle(element).gap), '20px');
  const overflow = await page
    .locator(
      '#main, .page, .assistant-composition, .assistant-composition .panel, .assistant-composition .panel-head, .assistant-composition .panel-body, .assistant-composition .prose, .assistant-composition li, .assistant-composition .disclosure-body',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.id),
    );
  assert.deepEqual(overflow, [], 'Assistant requests and complete explanations stay contained');
  for (const panel of await composition.locator('.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const control of await panel.locator('button, input, textarea, select, summary').all()) {
      if (!(await control.isVisible())) continue;
      const box = await control.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
  }
  const boxes = [];
  for (const part of await composition
    .locator('.assistant-request-actions .button, .form-grid label')
    .all()) {
    const box = await part.boundingBox();
    for (const other of boxes)
      assert.ok(
        box.x + box.width <= other.x + 1 ||
          other.x + other.width <= box.x + 1 ||
          box.y + box.height <= other.y + 1 ||
          other.y + other.height <= box.y + 1,
        'Request actions and creation fields do not collide',
      );
    boxes.push(box);
  }
  checks++;
}
async function chatConversationLayout(page) {
  const workspace = page.locator('.chat-page');
  const conversation = page.getByRole('region', { name: 'Change conversation', exact: true });
  const review = page.getByRole('region', { name: 'Proposal review', exact: true });
  await headingContainment(workspace.locator('.page-heading--intro'));
  for (const region of [workspace, workspace.locator('.change-workspace'), conversation])
    assert.equal(await region.evaluate((element) => getComputedStyle(element).gap), '20px');
  const heading = await workspace.locator('.page-heading').boundingBox();
  const body = await workspace.locator('.change-workspace').boundingBox();
  assert.ok(Math.abs(body.y - heading.y - heading.height - 20) <= 1);
  const left = await conversation.boundingBox();
  const right = await review.boundingBox();
  if (page.viewportSize().width > 1100) {
    assert.ok(Math.abs(right.y - left.y) <= 1, 'Conversation and review columns align');
    assert.ok(Math.abs(right.x - left.x - left.width - 20) <= 1);
  } else {
    assert.ok(Math.abs(right.y - left.y - left.height - 20) <= 1);
    assert.ok(Math.abs(right.x - left.x) <= 1);
    assert.ok(Math.abs(right.width - left.width) <= 1, 'Compact Chat stacks at full width');
  }
  const overflow = await page
    .locator(
      '#main, .page, .chat-page, .chat-conversation, .chat-conversation .panel, .chat-conversation .panel-head, .chat-conversation .panel-body, .chat-conversation .prose, .chat-conversation li, .chat-conversation .notice, .chat-conversation .disclosure-body, .chat-conversation .list-row',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.id),
    );
  assert.deepEqual(overflow, [], 'Chat scope, messages and history wrap within their panels');
  for (const panel of await conversation.locator('.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const control of await panel.locator('button, textarea, input, select, summary').all()) {
      if (!(await control.isVisible())) continue;
      const box = await control.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
    const boxes = [];
    for (const part of await panel.locator('.panel-head > *').all()) {
      const box = await part.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      for (const other of boxes)
        assert.ok(
          box.x + box.width <= other.x + 1 ||
            other.x + other.width <= box.x + 1 ||
            box.y + box.height <= other.y + 1 ||
            other.y + other.height <= box.y + 1,
          'Conversation titles and status labels do not collide',
        );
      boxes.push(box);
    }
  }
  checks++;
}
async function chatReviewLayout(page) {
  const review = page.getByRole('region', { name: 'Proposal review', exact: true });
  assert.equal(await review.evaluate((element) => getComputedStyle(element).gap), '20px');
  const overflow = await page
    .locator(
      '#main, .page, .chat-review, .chat-review .panel, .chat-review .panel-head, .chat-review .panel-body, .chat-review .code-header, .chat-review .disclosure-body, .chat-review .notice',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.id),
    );
  assert.deepEqual(
    overflow,
    [],
    'Proposal evidence stays within its panels; diff panes scroll locally',
  );
  for (const panel of await review.locator('.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const control of await panel.locator('button, summary').all()) {
      if (!(await control.isVisible())) continue;
      const box = await control.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
    const parts = await panel.locator('.panel-head > *, .code-header > *').all();
    const boxes = [];
    for (const part of parts) {
      const box = await part.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      for (const other of boxes)
        assert.ok(
          box.x + box.width <= other.x + 1 ||
            other.x + other.width <= box.x + 1 ||
            box.y + box.height <= other.y + 1 ||
            other.y + other.height <= box.y + 1,
          'Proposal metadata and status do not collide',
        );
      boxes.push(box);
    }
  }
  for (const diff of await review.locator('.diff').all()) {
    assert.equal(await diff.getAttribute('tabindex'), '0');
    assert.equal(await diff.locator('textarea, input, [contenteditable="true"]').count(), 0);
    assert.equal(
      await diff.locator('pre').evaluate((element) => getComputedStyle(element).overflowX),
      'auto',
    );
    assert.notEqual(
      await diff
        .locator('code')
        .first()
        .evaluate((element) => getComputedStyle(element).userSelect),
      'none',
    );
  }
  checks++;
}
async function chatOutcomeLayout(page) {
  const outcome = page.getByRole('region', { name: 'Change outcome', exact: true });
  await headingContainment(page.locator('.chat-page .page-heading--intro'));
  for (const region of [outcome, outcome.locator('.chat-receipt .panel-body')])
    if (await region.count())
      assert.equal(await region.evaluate((element) => getComputedStyle(element).gap), '20px');
  const sections = await page.locator('.chat-page > *').all();
  for (let i = 1; i < sections.length; i++) {
    const before = await sections[i - 1].boundingBox();
    const after = await sections[i].boundingBox();
    assert.ok(Math.abs(after.y - before.y - before.height - 20) <= 1);
    assert.ok(Math.abs(after.x - before.x) <= 1);
    assert.ok(Math.abs(after.width - before.width) <= 1);
  }
  const overflow = await page
    .locator(
      '#main, .page, .chat-page, .chat-outcome, .chat-outcome .panel, .chat-outcome .panel-head, .chat-outcome .panel-body, .chat-outcome .notice, .chat-verification, .chat-outcome .disclosure-body',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.id),
    );
  assert.deepEqual(overflow, [], 'Receipt warnings and verification stay within their containers');
  const bounds = await outcome.boundingBox();
  for (const control of await outcome.locator('button, summary').all()) {
    if (!(await control.isVisible())) continue;
    const box = await control.boundingBox();
    assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
    assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
  }
  for (const group of await outcome.locator('.panel-head, .actions, summary .row').all()) {
    const container = await group.boundingBox();
    const boxes = [];
    for (const child of await group.locator(':scope > *').all()) {
      const box = await child.boundingBox();
      assert.ok(box.x >= container.x - 1 && box.x + box.width <= container.x + container.width + 1);
      assert.ok(
        box.y >= container.y - 1 && box.y + box.height <= container.y + container.height + 1,
      );
      for (const other of boxes)
        assert.ok(
          box.x + box.width <= other.x + 1 ||
            other.x + other.width <= box.x + 1 ||
            box.y + box.height <= other.y + 1 ||
            other.y + other.height <= box.y + 1,
          'Receipt actions, check names and status labels never collide',
        );
      boxes.push(box);
    }
  }
  for (const output of await outcome.locator('pre').all()) {
    assert.equal(await output.evaluate((element) => getComputedStyle(element).overflowY), 'auto');
    assert.notEqual(
      await output.evaluate((element) => getComputedStyle(element).userSelect),
      'none',
    );
  }
  assert.equal(await outcome.locator('input, textarea, [contenteditable="true"]').count(), 0);
  checks++;
}
async function instructionsLayout(page) {
  const workspace = page.locator('.instructions-page');
  await headingContainment(workspace.locator('.page-heading--intro'));
  for (const region of [
    workspace,
    workspace.locator('.instruction-grid'),
    ...(await workspace.locator('.stack').all()),
  ])
    assert.equal(await region.evaluate((element) => getComputedStyle(element).gap), '20px');
  const wizard = await page
    .getByRole('region', { name: 'Instruction wizard', exact: true })
    .boundingBox();
  const guidance = await page
    .getByRole('region', { name: 'Effective project guidance', exact: true })
    .boundingBox();
  if (page.viewportSize().width > 1100) {
    assert.ok(Math.abs(guidance.y - wizard.y) <= 1);
    assert.ok(Math.abs(guidance.x - wizard.x - wizard.width - 20) <= 1);
  } else {
    assert.ok(Math.abs(guidance.y - wizard.y - wizard.height - 20) <= 1);
    assert.ok(Math.abs(guidance.x - wizard.x) <= 1);
    assert.ok(Math.abs(guidance.width - wizard.width) <= 1);
  }
  const overflow = await page
    .locator(
      '#main, .page, .instructions-page, .instruction-grid, .instructions-page .stack, .instructions-page .panel, .instructions-page .panel-head, .instructions-page .panel-body, .instructions-page .prose, .instructions-page .disclosure-body, .instructions-page .notice, .instructions-page summary, .instruction-presets',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.id),
    );
  assert.deepEqual(overflow, [], 'Guidance, paths and exclusions wrap without clipping');
  for (const panel of await workspace.locator('.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const control of await panel.locator('button, input, textarea, summary').all()) {
      if (!(await control.isVisible())) continue;
      const box = await control.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
  }
  for (const group of await workspace.locator('.panel-head, .actions, .wizard-steps').all()) {
    const boxes = [];
    for (const part of await group.locator(':scope > *').all()) {
      const box = await part.boundingBox();
      for (const other of boxes)
        assert.ok(
          box.x + box.width <= other.x + 1 ||
            other.x + other.width <= box.x + 1 ||
            box.y + box.height <= other.y + 1 ||
            other.y + other.height <= box.y + 1,
          'Wizard titles, steps and actions do not collide',
        );
      boxes.push(box);
    }
  }
  checks++;
}
async function featuresLayout(page) {
  const workspace = page.locator('.features-page');
  await headingContainment(workspace.locator('.page-heading--intro'));
  assert.equal(await workspace.evaluate((element) => getComputedStyle(element).gap), '20px');
  const overflow = await page
    .locator(
      '#main, .page, .features-page, .features-page .panel, .features-page .panel-head, .features-page .panel-body, .features-page .toolbar, .features-page .prose, .features-page .disclosure-body, .features-page li, .features-page .notice',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.id),
    );
  assert.deepEqual(overflow, [], 'Feature content wraps within its own region');
  for (const region of await workspace.locator('.panel-head, .toolbar, .actions').all()) {
    const bounds = await region.boundingBox();
    const boxes = [];
    for (const child of await region.locator(':scope > *').all()) {
      const box = await child.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
      for (const other of boxes)
        assert.ok(
          box.x + box.width <= other.x + 1 ||
            other.x + other.width <= box.x + 1 ||
            box.y + box.height <= other.y + 1 ||
            other.y + other.height <= box.y + 1,
          'Feature headers, filters and actions do not collide',
        );
      boxes.push(box);
    }
  }
  for (const panel of await workspace.locator('.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const control of await panel.locator('button, textarea').all()) {
      assert.equal(await control.isVisible(), true);
      const box = await control.boundingBox();
      assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
  }
  checks++;
}
async function resultsListLayout(page) {
  const workspace = page.locator('.results-page');
  await headingContainment(workspace.locator('.page-heading--intro'));
  assert.equal(await workspace.evaluate((element) => getComputedStyle(element).gap), '20px');
  const overflowing = await page
    .locator(
      '#main, .page, .results-page, .results-page .panel, .results-page .panel-head, .results-page .panel-body, .results-page .toolbar, .results-page .result-row, .results-page .list-copy, .results-page .result-badges',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.id || element.className),
    );
  assert.deepEqual(overflowing, [], 'Findings content wraps rather than clipping internally');
  for (const panel of await workspace.locator('section.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const action of await panel.getByRole('button').all()) {
      if (!(await action.isVisible())) continue;
      const box = await action.boundingBox();
      assert.ok(box && box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
  }
  const collisions = await workspace.locator('.result-row, .toolbar').evaluateAll((elements) =>
    elements.flatMap((element) => {
      const boxes = [...element.children].map((child) => child.getBoundingClientRect());
      return boxes.flatMap((box, i) =>
        boxes
          .slice(i + 1)
          .filter(
            (other) =>
              box.left < other.right - 1 &&
              other.left < box.right - 1 &&
              box.top < other.bottom - 1 &&
              other.top < box.bottom - 1,
          ),
      );
    }),
  );
  assert.deepEqual(
    collisions,
    [],
    'Finding titles, metadata, badges and filter fields do not collide',
  );
  const clippedText = await workspace
    .locator('.result-row, .panel-head, summary')
    .evaluateAll((elements) =>
      elements.flatMap((element) => {
        const bounds = element.getBoundingClientRect();
        const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
        const clipped = [];
        while (walker.nextNode()) {
          if (!walker.currentNode.textContent.trim()) continue;
          const range = document.createRange();
          range.selectNodeContents(walker.currentNode);
          if (
            [...range.getClientRects()].some(
              (rect) =>
                rect.left < bounds.left - 1 ||
                rect.right > bounds.right + 1 ||
                rect.top < bounds.top - 1 ||
                rect.bottom > bounds.bottom + 1,
            )
          )
            clipped.push(walker.currentNode.textContent);
        }
        return clipped;
      }),
    );
  assert.deepEqual(
    clippedText,
    [],
    'Complete findings and report titles remain inside their boundaries',
  );
  checks++;
}
async function resultsDetailLayout(page) {
  const workspace = page.locator('.results-detail');
  await headingContainment(workspace.locator('.page-heading--intro'));
  for (const region of [
    workspace,
    workspace.locator('.results-detail-layout'),
    ...(await workspace.locator('.results-detail-layout > .stack').all()),
  ])
    assert.equal(await region.evaluate((element) => getComputedStyle(element).gap), '20px');
  const overflowing = await page
    .locator(
      '#main, .page, .results-detail, .results-detail .panel, .results-detail .panel-head, .results-detail .panel-body, .results-detail .stack, .results-detail .key-values, .results-detail .key-values dd, .results-detail .prose, .results-detail .disclosure-body, .results-detail li',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.className || element.tagName),
    );
  assert.deepEqual(
    overflowing,
    [],
    'Complete detail evidence and anchors wrap inside their panels',
  );
  const [evidence, source] = await Promise.all(
    (await workspace.locator('.results-detail-layout > .stack').all()).map((stack) =>
      stack.boundingBox(),
    ),
  );
  if (source.x > evidence.x + 1) {
    assert.ok(Math.abs(source.y - evidence.y) <= 1, 'Evidence and source columns align');
    assert.ok(Math.abs(source.x - evidence.x - evidence.width - 20) <= 1);
  } else {
    assert.ok(Math.abs(source.y - evidence.y - evidence.height - 20) <= 1);
    assert.ok(
      Math.abs(source.width - evidence.width) <= 1,
      'Compact evidence stacks at full width',
    );
  }
  if (page.viewportSize().width === 800) assert.ok(Math.abs(source.x - evidence.x) <= 1);
  if (page.viewportSize().width === 1440) assert.ok(source.x > evidence.x + 1);
  for (const panel of await workspace.locator('.panel').all()) {
    const bounds = await panel.boundingBox();
    for (const action of await panel.getByRole('button').all()) {
      const box = await action.boundingBox();
      assert.ok(box && box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
  }
  checks++;
}
async function analysisPreviewLayout(page) {
  await headingContainment(page.locator('.page-heading--intro'));
  const intro = await page.locator('.page-heading--intro').boundingBox();
  const composition = page.locator('.analysis-preview-layout');
  const body = await composition.boundingBox();
  assert.ok(Math.abs(body.y - (intro.y + intro.height) - 20) <= 1);
  for (const region of [composition, composition.locator(':scope > .stack')])
    assert.equal(await region.evaluate((element) => getComputedStyle(element).gap), '20px');
  const overflowing = await page
    .locator(
      '#main, .page, .workspace-page, .analysis-preview-layout, .analysis-preview .stack, .analysis-preview .panel, .analysis-preview .panel-head, .analysis-preview .panel-body, .analysis-preview .three-columns > div, .analysis-preview .list-copy, .analysis-preview .notice',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.id || element.className),
    );
  assert.deepEqual(overflowing, [], 'Preview content wraps without internal horizontal clipping');
  for (const panel of await page.locator('.analysis-preview section.panel').all()) {
    const bounds = await panel.boundingBox();
    assert.ok(bounds.x >= body.x - 1 && bounds.x + bounds.width <= body.x + body.width + 1);
    for (const action of await panel.getByRole('button').all()) {
      const box = await action.boundingBox();
      assert.ok(box && box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
    }
  }
  const stack = await composition.locator(':scope > .stack').boundingBox();
  const files = await composition.locator(':scope > .panel').boundingBox();
  if (page.viewportSize().width > 1000) {
    assert.ok(Math.abs(files.y - stack.y) <= 1, 'Selected files align with scope');
    assert.ok(Math.abs(files.x - (stack.x + stack.width) - 20) <= 1);
  } else {
    assert.ok(Math.abs(files.y - (stack.y + stack.height) - 20) <= 1);
    assert.ok(Math.abs(files.width - stack.width) <= 1, 'Compact preview stacks full-width panels');
  }
  checks++;
}
async function analysisRunLayout(page) {
  const workspace = page.locator('.analysis-run');
  await headingContainment(workspace.locator('.page-heading--intro'));
  const blocks = await workspace.locator(':scope > *').all();
  for (let i = 1; i < blocks.length; i++) {
    const before = await blocks[i - 1].boundingBox();
    const after = await blocks[i].boundingBox();
    assert.ok(
      Math.abs(after.y - (before.y + before.height) - 20) <= 1,
      'Run sections retain Summary’s 20px rhythm',
    );
  }
  const overflow = await page
    .locator(
      '#main, .page, .analysis-run, .analysis-run .panel, .analysis-run .panel-head, .analysis-run .panel-body, .analysis-run .three-columns > div, .analysis-run .scroll-list, .analysis-run summary, .analysis-run .list-row, .analysis-run .notice',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.id || element.className),
    );
  assert.deepEqual(overflow, [], 'Run content remains contained without horizontal clipping');
  for (const panel of await workspace.locator('section.panel').all()) {
    const bounds = await panel.boundingBox();
    const headerParts = await panel.locator('.panel-head > *').all();
    const boxes = [];
    for (const part of [...headerParts, ...(await panel.getByRole('button').all())]) {
      const box = await part.boundingBox();
      assert.ok(box && box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
      assert.ok(box.y >= bounds.y - 1 && box.y + box.height <= bounds.y + bounds.height + 1);
      for (const other of boxes)
        assert.ok(
          box.x + box.width <= other.x + 1 ||
            other.x + other.width <= box.x + 1 ||
            box.y + box.height <= other.y + 1 ||
            other.y + other.height <= box.y + 1,
          'Panel titles, statuses and actions do not collide',
        );
      boxes.push(box);
    }
  }
  checks++;
}
async function analysisSections(page, hasRun) {
  const settings = page.locator('section.panel').filter({
    has: page.getByRole('heading', { name: 'Run settings', exact: true }),
  });
  const lastRun = page.locator('section.panel').filter({
    has: page.getByRole('heading', { name: 'Last run', exact: true }),
  });
  const heading = page
    .locator('.page-heading')
    .filter({ has: page.getByRole('heading', { name: 'Analysis', exact: true }) });
  const content = await page.locator('.workspace-page').boundingBox();
  await headingContainment(heading);
  const headingBox = await heading.boundingBox();
  const settingsBox = await settings.boundingBox();
  const sectionGap = (before, after) =>
    assert.ok(
      Math.abs(after.y - (before.y + before.height) - 20) <= 1,
      'Workspace sections retain Summary’s 20px rhythm',
    );
  sectionGap(headingBox, settingsBox);
  // Allow one CSS pixel for fractional layout rounding.
  assert.ok(Math.abs(settingsBox.x - content.x) <= 1, 'Settings starts at the content edge');
  assert.ok(Math.abs(settingsBox.width - content.width) <= 1, 'Settings spans the content');
  assert.equal(await lastRun.count(), hasRun ? 1 : 0);
  const toolbar = await page.locator('.toolbar').boundingBox();
  const table = await page.locator('.table-wrap').boundingBox();
  let previous = settingsBox;
  if (hasRun) {
    const box = await lastRun.boundingBox();
    assert.ok(Math.abs(box.x - content.x) <= 1, 'Last run starts at the content edge');
    assert.ok(Math.abs(box.width - content.width) <= 1, 'Last run spans the content');
    sectionGap(settingsBox, box);
    const header = lastRun.locator('.panel-head');
    const headerBox = await header.boundingBox();
    const parts = [
      lastRun.getByRole('heading', { name: 'Last run', exact: true }),
      header.getByRole('img'),
      header.getByText(/^(running|paused|failed|completed)$/, { exact: true }),
      header.getByRole('button', { name: 'View run', exact: true }),
    ];
    const boxes = [];
    for (const part of parts) {
      assert.equal(await part.isVisible(), true, 'Last run header content remains visible');
      const bounds = await part.boundingBox();
      assert.ok(bounds.x >= headerBox.x && bounds.y >= headerBox.y);
      assert.ok(bounds.x + bounds.width <= headerBox.x + headerBox.width + 1);
      assert.ok(bounds.y + bounds.height <= headerBox.y + headerBox.height + 1);
      for (const other of boxes)
        assert.ok(
          bounds.x + bounds.width <= other.x + 1 ||
            other.x + other.width <= bounds.x + 1 ||
            bounds.y + bounds.height <= other.y + 1 ||
            other.y + other.height <= bounds.y + 1,
          'Last run header content does not collide',
        );
      boxes.push(bounds);
    }
    previous = box;
  }
  sectionGap(previous, toolbar);
  sectionGap(toolbar, table);
  for (const box of [toolbar, table]) {
    assert.ok(Math.abs(box.x - content.x) <= 1, 'File controls start at the workspace edge');
    assert.ok(Math.abs(box.width - content.width) <= 1, 'File controls span the workspace');
  }
  for (const control of await page.locator('.toolbar').locator('input, button').all()) {
    assert.equal(await control.isVisible(), true, 'File controls remain reachable');
    const box = await control.boundingBox();
    assert.ok(box.x >= toolbar.x - 1 && box.x + box.width <= toolbar.x + toolbar.width + 1);
    assert.ok(box.y >= toolbar.y - 1 && box.y + box.height <= toolbar.y + toolbar.height + 1);
  }
  checks++;
}
async function analysisSettingsFields(page, columns, expanded) {
  const settings = page.locator('section.panel').filter({
    has: page.getByRole('heading', { name: 'Run settings', exact: true }),
  });
  const panelBox = await settings.boundingBox();
  for (const selector of ['select', ...(expanded ? ['input[type="number"]'] : [])]) {
    const controls = settings.locator(selector);
    assert.equal(await controls.count(), 3);
    const boxes = [];
    for (const control of await controls.all()) {
      assert.equal(await control.isVisible(), true);
      const box = await control.boundingBox();
      const label = await control.locator('..').boundingBox();
      assert.ok(box.x >= panelBox.x && box.x + box.width <= panelBox.x + panelBox.width + 1);
      assert.ok(box.y >= label.y && box.y + box.height <= label.y + label.height + 1);
      boxes.push(box);
    }
    for (let i = 1; i < boxes.length; i++) {
      assert.ok(Math.abs(boxes[i].width - boxes[0].width) <= 1, 'Equal control widths');
      if (columns === 3) {
        assert.ok(Math.abs(boxes[i].y - boxes[0].y) <= 1, 'Aligned control tops');
        assert.ok(boxes[i].x >= boxes[i - 1].x + boxes[i - 1].width, 'Separate columns');
      } else {
        assert.ok(Math.abs(boxes[i].x - boxes[0].x) <= 1, 'Aligned stacked controls');
        assert.ok(boxes[i].y >= boxes[i - 1].y + boxes[i - 1].height, 'Single-column reflow');
      }
    }
  }
  assert.equal(
    await settings.evaluate((panel) => panel.scrollWidth > panel.clientWidth + 1),
    false,
    'Settings has no internal horizontal overflow',
  );
  checks++;
}
function savedModelsFixture(long = false) {
  const details = [
    {
      label: 'Code',
      profile: 'bug',
      scope: 'bug',
      model: 'prior-code',
      origin: 'http://127.0.0.1:11434',
      remote: false,
      stages: ['semantic'],
    },
    {
      label: 'Performance & Security',
      profile: 'analyze',
      scope: 'analyze',
      model: 'prior-review',
      origin: 'https://review.invalid',
      remote: true,
      stages: ['performance', 'security_ai'],
    },
    {
      label: 'Feature discovery',
      profile: 'function',
      scope: 'analyze',
      model: 'prior-features',
      origin: 'https://features.invalid',
      remote: true,
      stages: ['feature_suggestions'],
    },
  ].map((detail) => ({
    ...detail,
    model: detail.model + (long ? `-${'unbrokenidentifier'.repeat(16)}` : ''),
    origin: detail.origin + (long ? `/v1/${'unbrokenorigin'.repeat(20)}` : ''),
  }));
  return {
    details,
    options: {
      capturedModels: { code: 'bug', review: 'analyze', features: 'function' },
      capturedProviders: details.map((detail, index) => ({
        id: `saved-provider-${index}`,
        stages: detail.stages,
        model: {
          scope: detail.scope,
          profile: detail.profile,
          model: detail.model,
          provider_origin: detail.origin,
          remote_provider: detail.remote,
          reasoning_effort: 'medium',
          timeout: '2m',
        },
        remote_confirmation_required: detail.remote,
      })),
    },
  };
}
async function capturedDetails(panel, expected, columns) {
  if (!expected) {
    assert.equal(await panel.getByText('Models unavailable', { exact: true }).count(), 1);
    assert.equal(await panel.locator('.three-columns').count(), 0);
    assert.equal(await panel.locator('.badge').count(), 0);
    checks++;
    return;
  }
  const groups = panel.locator('.three-columns > div');
  assert.equal(await groups.count(), 3);
  const boxes = [];
  const bodyBox = await panel.locator('.panel-body').boundingBox();
  for (const [index, detail] of expected.entries()) {
    const group = groups.nth(index);
    assert.equal(await group.locator('strong').innerText(), detail.label);
    assert.equal(await group.getByText(`Profile: ${detail.profile}`, { exact: true }).count(), 1);
    if (detail.model) {
      assert.equal(await group.getByText(detail.model, { exact: true }).count(), 1);
      assert.equal(await group.getByText(detail.origin, { exact: true }).count(), 1);
      assert.equal(await group.locator('.badge').innerText(), detail.remote ? 'Remote' : 'Local');
    } else {
      assert.equal(await group.getByText('Unavailable', { exact: true }).count(), 1);
      assert.equal(await group.locator('.badge').count(), 0);
    }
    const box = await group.boundingBox();
    assert.ok(box.x >= bodyBox.x && box.x + box.width <= bodyBox.x + bodyBox.width + 1);
    assert.ok(box.y >= bodyBox.y && box.y + box.height <= bodyBox.y + bodyBox.height + 1);
    const clipped = await group.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
      const failures = [];
      while (walker.nextNode()) {
        if (!walker.currentNode.textContent.trim()) continue;
        const range = document.createRange();
        range.selectNodeContents(walker.currentNode);
        for (const rect of range.getClientRects())
          if (
            rect.left < bounds.left - 1 ||
            rect.right > bounds.right + 1 ||
            rect.top < bounds.top - 1 ||
            rect.bottom > bounds.bottom + 1
          )
            failures.push(walker.currentNode.textContent);
      }
      return failures;
    });
    assert.deepEqual(clipped, [], `${detail.label}: complete text stays inside its group`);
    boxes.push(box);
  }
  for (let i = 1; i < boxes.length; i++) {
    assert.ok(Math.abs(boxes[i].width - boxes[0].width) <= 1, 'Equal captured group widths');
    if (columns === 3) {
      assert.ok(Math.abs(boxes[i].y - boxes[0].y) <= 1, 'Aligned captured groups');
      assert.ok(boxes[i].x >= boxes[i - 1].x + boxes[i - 1].width, 'Separate captured columns');
    } else {
      assert.ok(Math.abs(boxes[i].x - boxes[0].x) <= 1, 'Aligned stacked captured groups');
      assert.ok(boxes[i].y >= boxes[i - 1].y + boxes[i - 1].height, 'Single captured column');
    }
  }
  checks++;
}
async function analysisOverflow(page) {
  const overflowing = await page
    .locator(
      '#main, .page, .workspace-page, .analysis-sections, .analysis-sections .panel, .analysis-sections .panel-head, .analysis-sections .panel-body, .workspace-page > .toolbar, .analysis-last-run .three-columns, .analysis-last-run .three-columns > div',
    )
    .evaluateAll((elements) =>
      elements
        .filter((element) => element.scrollWidth > element.clientWidth + 1)
        .map((element) => element.id || element.className),
    );
  assert.deepEqual(overflowing, [], 'No internal main/page/panel/group horizontal overflow');
  const table = page.locator('.table-wrap');
  const tableBox = await table.boundingBox();
  const pageBox = await page.locator('.workspace-page').boundingBox();
  assert.ok(
    tableBox.x >= pageBox.x && tableBox.x + tableBox.width <= pageBox.x + pageBox.width + 1,
  );
  assert.equal(await table.evaluate((element) => getComputedStyle(element).overflowX), 'auto');
  checks++;
}
async function contrast(page, name) {
  const failures = await page.evaluate(() => {
    const rgba = (color) => {
      const channels = color.match(/[\d.]+/g).map(Number);
      const rgb = channels.slice(0, 3);
      return [
        ...(color.startsWith('color(srgb ') ? rgb.map((channel) => channel * 255) : rgb),
        channels[3] ?? 1,
      ];
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
      '.analysis-last-run .panel-head span:not(.status-dot)',
      '.analysis-last-run .three-columns strong',
      '.analysis-last-run .three-columns .row > span',
      '.metric-label',
      '.metric-number',
      '.metric-action',
      '.summary-hero .page-heading p',
      '.summary-facts dt',
      '.summary-facts dd',
      '.coverage-ring strong',
      '.coverage-copy p',
      '.legend span',
      '.badge',
      '.list-copy strong',
      '.list-copy small',
      '.results-state',
      '.results-page .page-heading p',
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
  server = await serve(0);
  url = `http://127.0.0.1:${server.address().port}`;
  browser = await chromium.launch({
    headless: true,
    ...(process.env.MINI_ORCA_TEST_BROWSER || process.platform === 'darwin'
      ? { channel: process.env.MINI_ORCA_TEST_BROWSER || 'chrome' }
      : {}),
  });
  await test('Redundant suggestion labels are removed while advisory content remains', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    assert.equal(
      await page
        .locator('.badge')
        .getByText(/^(ai suggestion|suggested)$/i)
        .count(),
      0,
    );
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
    await page.getByRole('heading', { name: 'Retry failed work', exact: true }).waitFor();
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
      const analysis = page.locator('.metric-card[data-accent="analysis-run"]');
      const analysisDot = analysis.getByRole('img');
      assert.equal(await analysisDot.getAttribute('class'), `status-dot ${tone}`);
      assert.equal(
        await analysisDot.getAttribute('aria-label'),
        `Project Analysis: ${status.replaceAll('_', ' ')}`,
      );
      assert.equal(await analysis.locator('.metric-action').innerText(), 'View run');
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
  await test('Summary diagram card opens all saved charts and entry points locally', async () => {
    const { page, close } = await pageFor();
    await idle(page);
    const requests = await page.evaluate(() => window.fixture.requests.length);
    for (const name of ['Architecture diagram', 'Flow 1 diagram', 'Flow 2 diagram'])
      assert.equal(await page.getByRole('img', { name, exact: true }).count(), 0);
    const card = page.locator('.metric-card[data-accent="diagrams"]');
    assert.equal(await card.locator('.metric-label').innerText(), 'Architecture and Flow');
    assert.equal(await card.locator('.metric-action').innerText(), 'Explore');
    assert.equal(
      await card.getByRole('img').getAttribute('aria-label'),
      'Architecture and Flow: success',
    );
    assert.equal(await page.locator('.metric-card').count(), 6);
    assert.equal(await page.getByText('Show architecture and flow', { exact: true }).count(), 0);
    assert.equal(await page.getByRole('heading', { name: 'Architecture', exact: true }).count(), 0);
    assert.equal(
      await page.getByRole('heading', { name: 'Project flows', exact: true }).count(),
      0,
    );
    await card.focus();
    await page.keyboard.press('Enter');
    await page.getByRole('heading', { name: 'Architecture and Flow', exact: true }).waitFor();
    assert.equal(await page.evaluate(() => window.fixture.requests.length), requests);
    assert.equal(await page.locator('.diagram img').count(), 3);
    for (const name of ['Architecture diagram', 'Flow 1 diagram', 'Flow 2 diagram'])
      assert.equal(await page.getByRole('img', { name, exact: true }).count(), 1);
    await page.getByText('cmd/server/main.go', { exact: true }).waitFor();
    for (const [name, file] of [
      ['Architecture diagram', 'architecture'],
      ['Flow 1 diagram', 'flowchart'],
      ['Flow 2 diagram', 'sequence'],
    ])
      await page.getByRole('img', { name, exact: true }).screenshot({
        path: `${output}/diagrams-${file}.png`,
      });
    const before = await page.evaluate(() =>
      window.fixture.requests.filter((request) => request.method !== 'GET'),
    );
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
    await layout(page, 'summary-navigation-cards-900');
    await contrast(page, 'Summary navigation cards large text');
    await openSummaryDiagrams(page);
    for (const [name, file] of [
      ['Architecture diagram', 'architecture'],
      ['Flow 1 diagram', 'flowchart'],
      ['Flow 2 diagram', 'sequence'],
    ])
      await page.getByRole('img', { name, exact: true }).screenshot({
        path: `${output}/diagrams-${file}-900.png`,
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
  await test('Summary analysis card opens the saved run or setup without starting work', async () => {
    for (const options of [{}, { empty: true }]) {
      const { page, close } = await pageFor(options);
      await idle(page);
      const card = page.locator('.metric-card').filter({ hasText: 'Project Analysis' });
      assert.equal(
        await card.locator('.metric-action').innerText(),
        options.empty ? 'Prepare analysis' : 'View run',
      );
      assert.equal(
        await card.getByRole('img').getAttribute('aria-label'),
        `Project Analysis: ${options.empty ? 'not run' : 'completed'}`,
      );
      const bounds = await page.locator('.metric-card').evaluateAll((cards) =>
        cards.map((card) => {
          const { width, height } = card.getBoundingClientRect();
          return { width, height };
        }),
      );
      assert.equal(bounds.length, 6);
      for (const box of bounds) {
        assert.ok(Math.abs(box.width - bounds[0].width) < 1);
        assert.equal(box.height, bounds[0].height);
      }
      const calls = await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      );
      await page.setViewportSize({ width: 800, height: 900 });
      await page.getByRole('button', { name: 'Larger text' }).click();
      await layout(page, `summary-cards-${options.empty ? 'no-run' : 'saved-run'}`);
      await card.focus();
      await contrast(page, 'Summary analysis card keyboard focus');
      await page.keyboard.press('Enter');
      await page
        .getByRole('heading', {
          name: options.empty ? 'Analysis' : 'Project analysis',
          exact: true,
        })
        .waitFor();
      assert.equal(await page.getByRole('dialog').count(), 0);
      assert.deepEqual(
        await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        ),
        calls,
      );
      assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      await close();
    }
  });
  await test('Summary keeps cards, facts and actions readable across window and text sizes', async () => {
    for (const long of [false, true]) {
      const projectName = long
        ? 'Harbor distributed background workers and durable local results'
        : 'harbor';
      const projectPath = long
        ? `/fixture/workspaces/${'long-project-directory'.repeat(6)}/harbor`
        : '/fixture/harbor';
      const projectSummary = long
        ? 'The service accepts requests through a local HTTP API and schedules bounded background work.\n\nEach worker records durable results while preserving cancellation and queue limits. Longer operations share the same context so shutdown remains predictable.'
        : 'A bounded worker service with a local queue.';
      const { page, close } = await pageFor({
        projectName,
        projectPath,
        projectSummary,
        featuresReady: true,
      });
      await idle(page);
      assert.equal(await page.locator('.summary-hero .page-heading p').innerText(), projectPath);
      assert.deepEqual(await page.locator('.summary-facts dd').allTextContents(), [
        'go',
        '18',
        '2,450',
        '0',
      ]);
      const requests = await page.evaluate(() => window.fixture.requests.length);
      for (const [width, columns] of [
        [1440, 3],
        [1000, 2],
        [700, 1],
      ]) {
        await page.setViewportSize({ width, height: 900 });
        if (width === 1000) await page.getByRole('button', { name: 'Larger text' }).click();
        for (const theme of ['dark', 'light']) {
          if ((await page.locator('html').getAttribute('data-theme')) !== theme)
            await page.getByRole('button', { name: `Switch to ${theme} appearance` }).click();
          const boxes = await page.locator('.metric-card').evaluateAll((cards) =>
            cards.map((card) => {
              const { x, y, width, height } = card.getBoundingClientRect();
              return { x, y, width, height };
            }),
          );
          assert.equal(boxes.filter((box) => Math.abs(box.y - boxes[0].y) < 1).length, columns);
          for (const box of boxes) assert.ok(Math.abs(box.width - boxes[0].width) < 1);
          const clipped = await page
            .locator('.summary-page')
            .evaluate((summary) =>
              [
                ...summary.querySelectorAll(
                  'button, h1, .page-heading p, dt, dd, .metric-label, .metric-value, .metric-action',
                ),
              ]
                .filter((element) => element.scrollWidth > element.clientWidth + 1)
                .map((element) => element.textContent.trim()),
            );
          assert.deepEqual(clipped, [], 'Summary content must wrap without clipping');
          assert.equal(
            await page.getByRole('button', { name: 'Refresh', exact: true }).isVisible(),
            true,
          );
          assert.equal(
            await page.getByRole('button', { name: 'Analyze project', exact: true }).isVisible(),
            true,
          );
          await layout(page, `summary-refined-${long ? 'long' : 'normal'}-${width}-${theme}`);
          await contrast(page, `Summary refined ${width} ${theme}`);
          if (width === 1440 && !long) {
            for (const card of await page.locator('.metric-card').all()) {
              await card.hover();
              await contrast(page, `Summary card hover ${theme}`);
            }
            await page.locator('.summary-hero h1').hover();
          }
        }
      }
      await page.getByText('Components', { exact: true }).click();
      await page.getByText('Worker pool', { exact: true }).waitFor();
      await page.getByText('Why & tradeoffs', { exact: true }).click();
      await page
        .getByText('Make cancellation part of the work boundary.', { exact: true })
        .waitFor();
      assert.equal(await page.evaluate(() => window.fixture.requests.length), requests);
      await close();
    }
  });
  await test('Summary coverage and facts preserve empty and unavailable values', async () => {
    for (const unavailable of [false, true]) {
      const { page, close } = await pageFor({
        overviewReadFail: unavailable,
        coverage: { total: 0, fresh: 0, stale: 0, missing: 0, failed: 0 },
      });
      await idle(page);
      assert.equal(await page.locator('.coverage-ring strong').innerText(), '—');
      assert.equal(
        await page.locator('.coverage-copy p').innerText(),
        unavailable ? 'Not available yet' : '0 of 0 files current',
      );
      assert.deepEqual(
        await page.locator('.legend strong').allTextContents(),
        Array(3).fill(unavailable ? '—' : '0'),
      );
      if (unavailable) {
        await page
          .getByRole('alert')
          .filter({ hasText: 'Saved overview could not be read.' })
          .waitFor();
        assert.deepEqual(await page.locator('.summary-facts dd').allTextContents(), [
          'go',
          '—',
          '—',
          '—',
        ]);
        const details = await page.locator('.summary-details').boundingBox();
        const overview = await page.locator('.summary-details > .panel').boundingBox();
        assert.equal(overview.width, details.width);
      }
      await layout(page, `summary-coverage-${unavailable ? 'unavailable' : 'empty'}`);
      await close();
    }
  });
  await test('Fenced charts and saved prose remain readable', async () => {
    const { page, close } = await pageFor({
      architecture:
        'The API admits requests into a bounded queue.\n\n```mermaid\ngraph LR\n  API --> Queue\n```',
      flows: ['``` Mermaid \r\nsequenceDiagram\r\n  API->>Worker: Process\r\n```'],
    });
    await openSummaryDiagrams(page);
    assert.equal(await page.locator('.diagram img').count(), 2);
    await page
      .getByText('The API admits requests into a bounded queue.', { exact: true })
      .waitFor();
    await close();

    const legacy = await pageFor({
      architecture: 'The API admits requests to the worker service.',
      flows: ['Each request is validated before the worker processes it.'],
    });
    await openSummaryDiagrams(legacy.page);
    assert.equal(await legacy.page.locator('.diagram img').count(), 0);
    await legacy.page
      .getByText('The API admits requests to the worker service.', { exact: true })
      .waitFor();
    await legacy.page
      .getByText('Each request is validated before the worker processes it.', { exact: true })
      .waitFor();
    await legacy.close();
  });
  await test('Unavailable and invalid charts retain clear states and complete source', async () => {
    const missing = await pageFor({ architecture: '', flows: [] });
    await openSummaryDiagrams(missing.page);
    await missing.page.getByText('No architecture overview saved.', { exact: true }).waitFor();
    await missing.page.getByText('No project flows saved.', { exact: true }).waitFor();
    assert.equal(await missing.page.locator('.diagram img').count(), 0);
    await missing.close();

    for (const [source, error] of [
      ['flowchart NOT_A_DIRECTION\n  API --> Worker', 'Diagram preview unavailable'],
      ['flowchart TD\n' + '%% comment\n'.repeat(160), 'Diagram is too large to render.'],
      ['flowchart TD\n  A["' + 'x'.repeat(16000) + '"]', 'Diagram is too large to render.'],
    ]) {
      const { page, close } = await pageFor({ architecture: source, flows: [] });
      await openSummaryDiagrams(page);
      await page.getByText(error, { exact: true }).waitFor();
      assert.equal(await page.locator('.diagram img').count(), 0);
      const saved = page.locator('.diagram pre');
      assert.equal(await saved.isVisible(), true);
      assert.equal(await saved.textContent(), source.trim());
      await saved.focus();
      assert.equal(await saved.evaluate((element) => element === document.activeElement), true);
      await close();
    }
  });
  await test('Source uses Summary framing without clipping read-only content or workflow controls', async () => {
    const path = `internal/${'long-directory-'.repeat(18)}/${'long-file-'.repeat(16)}.go`;
    const source = [
      'package worker',
      '',
      'import "context"',
      '',
      'func Process(ctx context.Context) error {',
      `\t// ${'long-source-line-'.repeat(160)}`,
      '\treturn nil',
      '}',
      ...Array.from({ length: 180 }, (_, i) => `// Complete source line ${i}`),
    ].join('\n');
    const { page, close } = await pageFor({
      sourcePaths: [path],
      sourceContent: source,
      sourceFile: { line_count: 188 },
    });
    try {
      for (const theme of ['dark', 'light']) {
        if (theme === 'light')
          await page.getByRole('button', { name: 'Switch to light appearance' }).click();
        await nav(page, 'Summary');
        const referenceIntro = await introductionTreatment(page.locator('.summary-hero'));
        const referencePanel = await panelTreatment(
          page.locator('.summary-details > .panel').first(),
        );
        await nav(page, 'Source');
        await page.locator('.file-item').click();
        await page.getByLabel('Declaration', { exact: true }).selectOption('Process');
        const workspace = page.locator('.source-workspace');
        const code = page.getByLabel('Read-only source', { exact: true });
        assert.deepEqual(
          await introductionTreatment(workspace.locator('.page-heading--intro')),
          referenceIntro,
        );
        assert.deepEqual(
          await panelTreatment(workspace.locator('.source-analysis')),
          referencePanel,
        );
        assert.equal(
          await code
            .locator('code')
            .allTextContents()
            .then((lines) => lines.join('\n')),
          source
            .split('\n')
            .map((line) => line || ' ')
            .join('\n'),
        );
        assert.equal(await code.evaluate((element) => element.isContentEditable), false);
        assert.equal(await workspace.locator('textarea').count(), 0);
        assert.equal(await code.locator('.selected-line').count(), 3);
        await page.getByLabel('Declaration', { exact: true }).selectOption('');
        await code.evaluate((element) => {
          element.scrollTop = element.scrollHeight;
        });
        await page.getByLabel('Declaration', { exact: true }).selectOption('Process');
        await page.waitForFunction(() => {
          const code = document.querySelector('.source-code');
          const selected = code.querySelector('.selected-line');
          if (!selected) return false;
          const bounds = code.getBoundingClientRect();
          const line = selected.getBoundingClientRect();
          return line.top >= bounds.top && line.bottom <= bounds.bottom;
        });
        assert.equal(
          await code.evaluate((element) => {
            const range = document.createRange();
            range.selectNodeContents(element.querySelector('.selected-line code'));
            const selection = window.getSelection();
            selection.removeAllRanges();
            selection.addRange(range);
            return selection.toString();
          }),
          'func Process(ctx context.Context) error {',
        );
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        );
        for (const larger of [false, true]) {
          if (larger) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          for (const width of [1440, 1280, 1001, 800]) {
            await page.setViewportSize({ width, height: 1000 });
            await headingContainment(workspace.locator('.page-heading--intro'));
            const bounds = await workspace.locator('.editor-content').boundingBox();
            for (const control of await workspace
              .locator('.tabs .tab, .declaration-picker > *, .source-inspection > .actions .button')
              .all()) {
              if (!(await control.isVisible())) continue;
              const box = await control.boundingBox();
              assert.ok(
                box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1,
                'Source actions and tabs stay within the inspection column',
              );
            }
            assert.equal(
              await code.evaluate((element) => element.scrollWidth > element.clientWidth),
              true,
              'Long code scrolls locally',
            );
            await code.focus();
            assert.equal(
              await code.evaluate((element) => element === document.activeElement),
              true,
            );
            await layout(page, `source-long-${theme}-${larger ? 'larger' : 'standard'}-${width}`);
            await contrast(page, `source-${theme}-${larger ? 'larger' : 'standard'}-${width}`);
          }
          if (larger) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        }
        await page.getByText('Dependencies & side effects', { exact: true }).click();
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter((request) => request.method !== 'GET'),
          ),
          before,
          'Source selection, disclosures and appearance do not admit work',
        );
      }
    } finally {
      await close();
    }
  });
  await test('Context keeps Summary panel rhythm and complete long metadata without admitting work', async () => {
    const path = `internal/${'context-directory-'.repeat(30)}/worker.go`;
    const hash = 'abcdef0123456789'.repeat(24);
    const origin = `https://${'provider-destination-'.repeat(24)}.example/v1`;
    const reason = `Policy exclusion: ${'complete-reason-'.repeat(35)} final reason.`;
    const related = `RelatedDeclaration${'LongName'.repeat(30)}`;
    const { page, close } = await pageFor({
      context: {
        included: [{ path, size_bytes: 12345, estimated_tokens: 2345, hash, truncated: true }],
        excluded: [{ path: `${path}.excluded`, reason }],
        estimated_tokens: 2345,
        truncated: true,
        scope: 'function',
        model: `captured-model-${'identifier-'.repeat(30)}`,
        provider_origin: origin,
      },
      impact: {
        references: [
          { path, symbol: related, confidence: 'lexical', reason: 'Advisory reference' },
        ],
      },
    });
    try {
      for (const theme of ['dark', 'light']) {
        if (theme === 'light')
          await page.getByRole('button', { name: 'Switch to light appearance' }).click();
        await nav(page, 'Summary');
        const reference = await panelTreatment(page.locator('.summary-details > .panel').first());
        await openSource(page);
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        );
        await page.getByRole('tab', { name: 'Context', exact: true }).click();
        const context = page.locator('.context-inspection');
        await context.getByRole('heading', { name: 'Related declarations', exact: true }).waitFor();
        assert.deepEqual(await panelTreatment(context.locator('.panel').first()), reference);
        assert.equal(
          await page
            .getByRole('tab', { name: 'Context', exact: true })
            .getAttribute('aria-selected'),
          'true',
        );
        assert.deepEqual(await context.locator('.mini-metrics strong').allTextContents(), [
          '1',
          '2,345',
          '1',
        ]);
        assert.equal(await context.locator('.key-values dd').last().innerText(), 'Yes');
        await context.getByText(origin, { exact: true }).waitFor();
        await context.getByText(reason, { exact: true }).waitFor();
        await context.getByText('truncated', { exact: true }).waitFor();
        await context.getByText('Content hashes', { exact: true }).click();
        assert.equal(await context.locator('.mono').innerText(), hash);
        await context.getByText(`${path} · ${related}`, { exact: true }).waitFor();
        assert.equal(await context.locator('textarea, input, [contenteditable="true"]').count(), 0);
        assert.equal(
          await context.locator('.mono').evaluate((element) => {
            const range = document.createRange();
            range.selectNodeContents(element);
            const selection = window.getSelection();
            selection.removeAllRanges();
            selection.addRange(range);
            return selection.toString();
          }),
          hash,
        );
        for (const larger of [false, true]) {
          if (larger) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          for (const width of [1440, 1280, 1001, 800]) {
            await page.setViewportSize({ width, height: 1000 });
            assert.deepEqual(
              await context.evaluate((element) => ({
                gap: getComputedStyle(element).gap,
                overflow: element.scrollWidth > element.clientWidth + 1,
              })),
              { gap: '20px', overflow: false },
            );
            const bounds = await context.boundingBox();
            const refresh = await context
              .getByRole('button', { name: 'Refresh', exact: true })
              .boundingBox();
            assert.ok(
              refresh.x >= bounds.x && refresh.x + refresh.width <= bounds.x + bounds.width + 1,
            );
            const table = page.getByLabel('Included context files', { exact: true });
            await table.focus();
            assert.equal(
              await table.evaluate((element) => element === document.activeElement),
              true,
            );
            await layout(
              page,
              `context-long-truncated-${theme}-${larger ? 'larger' : 'standard'}-${width}`,
            );
            await contrast(page, `context-${theme}-${larger ? 'larger' : 'standard'}-${width}`);
          }
          if (larger) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        }
        const reads = await page.evaluate(
          () =>
            window.fixture.requests.filter((request) => request.path?.endsWith('/context')).length,
        );
        await context.getByRole('button', { name: 'Refresh', exact: true }).click();
        await page.waitForFunction(
          (count) =>
            window.fixture.requests.filter((request) => request.path?.endsWith('/context'))
              .length ===
            count + 1,
          reads,
        );
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter((request) => request.method !== 'GET'),
          ),
          before,
          'Context navigation, hashes, appearance and explicit refresh remain local reads',
        );
        assert.equal(await page.getByRole('dialog').count(), 0);
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      }
    } finally {
      await close();
    }
  });
  await test('Context empty, unavailable metadata and read failures preserve their meanings', async () => {
    for (const stateCase of [
      'populated',
      'empty',
      'metadata-unavailable',
      'unavailable',
      'read-failed',
    ]) {
      const options = {
        impact: { references: [] },
        ...(stateCase === 'empty'
          ? { context: { included: [], excluded: [], estimated_tokens: 0, truncated: false } }
          : {}),
        ...(stateCase === 'metadata-unavailable' ? { context: {} } : {}),
        ...(stateCase === 'unavailable' ? { context: null } : {}),
      };
      const { page, close } = await pageFor(options);
      try {
        await openSource(page);
        if (stateCase === 'read-failed')
          await page.evaluate(() => {
            window.fixture.failures['/api/projects/current/context'] = 503;
          });
        await page.getByRole('tab', { name: 'Context', exact: true }).click();
        if (stateCase === 'unavailable' || stateCase === 'read-failed') {
          await page.getByRole('heading', { name: 'Context unavailable', exact: true }).waitFor();
          if (stateCase === 'read-failed') {
            await page.getByText('Fixture rejection', { exact: false }).waitFor();
          }
        } else {
          const context = page.locator('.context-inspection');
          await context
            .getByRole('heading', { name: 'Related declarations', exact: true })
            .waitFor();
          await context.getByText('No references found.', { exact: true }).waitFor();
          if (stateCase === 'empty' || stateCase === 'metadata-unavailable') {
            assert.deepEqual(
              await context.locator('.mini-metrics strong').allTextContents(),
              Array(3).fill(stateCase === 'empty' ? '0' : '—'),
            );
            assert.equal(await context.locator('tbody tr').count(), 0);
            assert.deepEqual(await context.locator('.key-values dd').allTextContents(), [
              '—',
              '—',
              '—',
              stateCase === 'empty' ? 'No' : '—',
            ]);
          }
        }
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page.getByRole('button', { name: 'Switch to light appearance' }).click();
          for (const larger of [false, true]) {
            if (larger)
              await page.getByRole('button', { name: 'Larger text', exact: true }).click();
            for (const width of [1440, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await layout(
                page,
                `context-${stateCase}-${theme}-${larger ? 'larger' : 'standard'}-${width}`,
              );
            }
            if (larger)
              await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          }
        }
        if (stateCase === 'read-failed') {
          await page.getByRole('button', { name: 'Refresh context', exact: true }).click();
          await page.getByRole('heading', { name: 'Included files', exact: true }).waitFor();
          await layout(page, 'context-read-failed-recovered-light-standard-800');
        }
        assert.equal(
          await page.evaluate(
            () => window.fixture.requests.filter((request) => request.method !== 'GET').length,
          ),
          0,
        );
      } finally {
        await close();
      }
    }
  });
  await test('Assistant and new-declaration retain Summary rhythm, complete history and local inputs', async () => {
    const symbol = `Process${'LongTarget'.repeat(25)}`;
    const summary = `Complete explanation. ${'LongExplanation'.repeat(40)}\n\nFinal summary. <script>window.assistantExecuted = true</script> ![remote](https://assistant.invalid/image.png)`;
    const detail = `Complete inputs and side effects: ${'LongDetail'.repeat(50)} final detail.`;
    const response = `Complete assistant response. ${'LongResponse'.repeat(45)}\n\nFinal response.`;
    const request = `Preserve behavior: ${'LongRequest'.repeat(30)}`;
    const constraints = `Keep the interface: ${'LongConstraint'.repeat(25)}`;
    for (const route of ['assistant', 'new-declaration']) {
      const create = route === 'new-declaration';
      const { page, close } = await pageFor({
        remote: true,
        explanation: {
          anchor: { path: 'internal/worker/process.go', symbol },
          summary,
          behavior: [detail],
          inputs: [detail],
          outputs: [detail],
          side_effects: [detail],
          error_behavior: [detail],
        },
        declarationAssistantMessage: response,
      });
      try {
        const references = {};
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page.getByRole('button', { name: 'Switch to light appearance' }).click();
          references[theme] = await panelTreatment(
            page.locator('.summary-details > .panel').first(),
          );
        }
        await page.getByRole('button', { name: 'Switch to dark appearance' }).click();
        await openSource(page);
        await page.getByRole('button', { name: 'Explain declaration', exact: true }).click();
        await page.getByRole('dialog').getByRole('button', { name: 'Continue' }).click();
        await idle(page);
        await page.getByRole('heading', { name: `About ${symbol}`, exact: true }).waitFor();
        if (create)
          await page.getByRole('button', { name: 'New declaration', exact: true }).click();
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((entry) => entry.method !== 'GET'),
        );
        const composition = page.locator('.assistant-composition');
        if (create) {
          assert.deepEqual(await page.getByLabel('Kind').locator('option').allTextContents(), [
            'function',
            'type',
            'var',
            'const',
          ]);
          for (const kind of ['function', 'type', 'var', 'const'])
            await page.getByLabel('Kind').selectOption(kind);
          await page.getByLabel('Kind').selectOption('type');
          await page.getByLabel('Name', { exact: true }).fill(`New${symbol}`);
        }
        for (const preset of ['Fix', 'Refactor', 'Document']) {
          await composition.getByRole('button', { name: preset, exact: true }).click();
          assert.equal(
            await page.getByLabel('Change request', { exact: true }).inputValue(),
            `${preset}${preset === 'Fix' ? ' a bug' : ' without changing behavior'}: `,
          );
        }
        await page.getByLabel('Change request', { exact: true }).fill(request);
        await composition.getByText('Constraints', { exact: true }).click();
        await page.getByLabel('Change constraints', { exact: true }).fill(constraints);
        await composition.getByText('Constraints', { exact: true }).click();
        assert.equal(
          await page.getByLabel('Change constraints', { exact: true }).isVisible(),
          false,
        );
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter((entry) => entry.method !== 'GET'),
          ),
          before,
          'Presets, fields and disclosures never generate',
        );
        await page.getByRole('button', { name: 'Prepare draft', exact: true }).click();
        const dialog = page.getByRole('dialog');
        await dialog.getByText('Scope: function', { exact: true }).waitFor();
        await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
        await idle(page);
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter((entry) => entry.method !== 'GET'),
          ),
          before,
          'Canceling Function consent never prepares',
        );
        assert.equal(
          await page.getByLabel('Change request', { exact: true }).inputValue(),
          request,
        );
        await page.getByRole('button', { name: 'Prepare draft', exact: true }).click();
        await dialog.getByRole('button', { name: 'Continue' }).click();
        await page.getByLabel('Declaration draft', { exact: true }).waitFor();
        await idle(page);
        const sent = await page.evaluate(() =>
          window.fixture.requests.find((entry) => entry.path.endsWith('/messages')),
        );
        assert.equal(
          sent.body.message,
          `${create ? `Create one type named New${symbol}.\n\n` : ''}${request}\n\nConstraints:\n${constraints}`,
        );
        assert.equal(sent.body.confirm_remote_provider, true);
        const session = await page.evaluate(() =>
          window.fixture.requests.find((entry) => entry.path.endsWith('/chat/sessions')),
        );
        assert.equal(session.body.mode, create ? 'create_symbol' : 'replace_symbol');
        assert.equal(session.body.target_symbol, create ? `New${symbol}` : 'Process');
        await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
        if (create)
          await page.getByRole('button', { name: 'New declaration', exact: true }).click();
        await composition.getByText('Inputs, outputs & side effects', { exact: true }).click();
        await composition.getByText('Why & tradeoffs', { exact: true }).click();
        await composition.getByText('Constraints', { exact: true }).click();
        await page.getByLabel('Change request', { exact: true }).fill('Local follow-up');
        await page.getByLabel('Change constraints', { exact: true }).fill('Local boundary');
        if (create) await page.getByLabel('Name', { exact: true }).fill(`New${symbol}`);
        const passive = await page.evaluate(() =>
          window.fixture.requests.filter((entry) => entry.method !== 'GET'),
        );
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page.getByRole('button', { name: 'Switch to light appearance' }).click();
          assert.deepEqual(
            await panelTreatment(composition.locator('.panel').first()),
            references[theme],
          );
          for (const larger of [false, true]) {
            if (larger)
              await page.getByRole('button', { name: 'Larger text', exact: true }).click();
            for (const width of [1440, 1280, 1001, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await assistantLayout(page);
              assert.equal(
                await page.getByLabel('Change request', { exact: true }).inputValue(),
                'Local follow-up',
              );
              assert.equal(
                await page.getByLabel('Change constraints', { exact: true }).inputValue(),
                'Local boundary',
              );
              if (create)
                assert.equal(
                  await page.getByLabel('Name', { exact: true }).inputValue(),
                  `New${symbol}`,
                );
              await composition.getByText('Final response.', { exact: true }).waitFor();
              assert.ok((await composition.innerText()).includes('Final summary.'));
              assert.ok((await composition.innerText()).includes('final detail.'));
              assert.ok((await composition.innerText()).includes(request));
              assert.equal(
                await composition.locator('a[href^="https://assistant"], img, script').count(),
                0,
              );
              assert.equal(await page.evaluate(() => window.assistantExecuted), undefined);
              await layout(
                page,
                `${route}-long-explanation-history-${theme}-${larger ? 'larger' : 'standard'}-${width}`,
              );
              await contrast(page, `${route}-${theme}-${larger ? 'larger' : 'standard'}-${width}`);
            }
            if (larger)
              await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          }
        }
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter((entry) => entry.method !== 'GET'),
          ),
          passive,
          'Reflow and explanation/history inspection never admit work',
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      } finally {
        await close();
      }
    }
  });
  await test('Both Assistant routes preserve busy, canceled and stale requests and target input lifetimes', async () => {
    for (const route of ['assistant', 'new-declaration']) {
      const create = route === 'new-declaration';
      const { page, close } = await pageFor();
      try {
        await openSource(page);
        await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
        if (create)
          await page.getByRole('button', { name: 'New declaration', exact: true }).click();
        assert.equal(
          await page.getByRole('button', { name: 'Prepare draft', exact: true }).isDisabled(),
          true,
        );
        await page.getByLabel('Change request', { exact: true }).fill('Return the context error.');
        if (create) {
          assert.equal(
            await page.getByRole('button', { name: 'Prepare draft', exact: true }).isDisabled(),
            true,
          );
          await page.getByLabel('Name', { exact: true }).fill('NewWorker');
        }
        await page
          .locator('.assistant-composition')
          .getByText('Constraints', { exact: true })
          .click();
        await page
          .getByLabel('Change constraints', { exact: true })
          .fill('Preserve the interface.');
        await page.evaluate(() => {
          window.fixture.hold = '/api/projects/current/chat/sessions/session-1/messages';
        });
        await page.getByRole('button', { name: 'Prepare draft', exact: true }).click();
        await page.waitForFunction(() =>
          window.fixture.requests.some((entry) => entry.path.endsWith('/messages')),
        );
        for (const control of [
          page.getByLabel('Change request', { exact: true }),
          page.getByLabel('Change constraints', { exact: true }),
          ...(await page.locator('.assistant-request .actions button').all()),
          page.getByRole('button', { name: 'Prepare draft', exact: true }),
        ])
          assert.equal(await control.isDisabled(), true);
        assert.equal(
          await page.getByRole('button', { name: 'Inspect context', exact: true }).isDisabled(),
          false,
        );
        if (!create)
          assert.equal(await page.getByLabel('Declaration', { exact: true }).isDisabled(), true);
        await page.setViewportSize({ width: 800, height: 1000 });
        await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await assistantLayout(page);
        await layout(page, `${route}-busy-dark-larger-800`);
        await page
          .locator('.busy-strip')
          .getByRole('button', { name: 'Cancel', exact: true })
          .click();
        await idle(page);
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
        await page.waitForTimeout(100);
        assert.equal(await page.getByLabel('Declaration draft', { exact: true }).count(), 0);
        assert.equal(
          await page.getByLabel('Change request', { exact: true }).inputValue(),
          'Return the context error.',
        );
        assert.equal(
          await page.getByLabel('Change constraints', { exact: true }).inputValue(),
          'Preserve the interface.',
        );
        assert.equal(
          await page.getByRole('button', { name: 'Prepare draft', exact: true }).isDisabled(),
          false,
        );
        await layout(page, `${route}-canceled-dark-larger-800`);
        if (!create) {
          await page.getByLabel('Declaration', { exact: true }).selectOption('');
          assert.equal(
            await page.getByRole('button', { name: 'Prepare draft', exact: true }).isDisabled(),
            true,
          );
          assert.equal(
            await page.getByLabel('Change request', { exact: true }).inputValue(),
            'Return the context error.',
          );
          await page.getByLabel('Declaration', { exact: true }).selectOption('Process');
        }
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((entry) => entry.method !== 'GET'),
        );
        await page.evaluate(() => {
          window.fixture.state.changed = true;
        });
        // Assistant navigation uses the existing source-freshness read; new-declaration
        // shares its owner but has no separate freshness trigger.
        await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
        await page.getByText('File evidence is outdated.', { exact: false }).waitFor();
        if (create)
          await page.getByRole('button', { name: 'New declaration', exact: true }).click();
        await page.getByLabel('Change request', { exact: true }).fill('Retry only after refresh.');
        if (create) await page.getByLabel('Name', { exact: true }).fill('NewWorker');
        assert.equal(
          await page.getByRole('button', { name: 'Prepare draft', exact: true }).isDisabled(),
          true,
        );
        await assistantLayout(page);
        await layout(page, `${route}-stale-dark-larger-800`);
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter((entry) => entry.method !== 'GET'),
          ),
          before,
          'Target changes and stale navigation do not generate',
        );
        await page.locator('.file-item[title="internal/worker/config.go"]').click();
        await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
        assert.equal(
          await page.getByLabel('Change request', { exact: true }).inputValue(),
          '',
          'A different file keeps the existing Assistant key reset',
        );
      } finally {
        await page
          .evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          })
          .catch(() => {});
        await close();
      }
    }
  });
  await test('Source empty, binary, unavailable and failed evidence retain content and recovery', async () => {
    for (const stateCase of [
      'no-file',
      'binary',
      'unavailable',
      'analysis-failed',
      'analysis-read-failed',
      'file-read-failed',
    ]) {
      const { page, close } = await pageFor({
        ...(stateCase === 'binary'
          ? { sourceFile: { binary: true, symbols: [], language: 'binary' } }
          : {}),
        ...(stateCase === 'unavailable' ? { fileAnalysis: null } : {}),
        ...(stateCase === 'analysis-failed'
          ? {
              fileAnalysis: {
                status: 'failed',
                failure: `Analysis failed: ${'diagnostic_'.repeat(90)} final reason.`,
              },
            }
          : {}),
      });
      try {
        await page.setViewportSize({ width: 800, height: 1000 });
        await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await nav(page, 'Source');
        await page.getByRole('heading', { name: 'Choose a file', exact: true }).waitFor();
        if (stateCase !== 'no-file') {
          if (stateCase.endsWith('read-failed'))
            await page.evaluate((kind) => {
              window.fixture.failures[
                `/api/projects/current/files/${kind === 'file-read-failed' ? 'info' : 'analysis'}`
              ] = 503;
            }, stateCase);
          await page.locator('.file-item').first().click();
          if (stateCase === 'binary') {
            await page.getByRole('heading', { name: 'Binary file', exact: true }).waitFor();
            assert.equal(await page.getByLabel('Read-only source', { exact: true }).count(), 0);
          } else if (stateCase === 'file-read-failed') {
            await page.getByText('Fixture rejection', { exact: false }).waitFor();
            await page.getByRole('heading', { name: 'Choose a file', exact: true }).waitFor();
            await page.locator('.file-item').first().click();
            await page.getByLabel('Read-only source', { exact: true }).waitFor();
          } else {
            await page.getByLabel('Read-only source', { exact: true }).waitFor();
            if (stateCase === 'analysis-failed')
              await page.getByText(/Analysis failed:.*final reason\./).waitFor();
            if (stateCase === 'unavailable' || stateCase === 'analysis-read-failed')
              assert.equal(
                await page.getByRole('heading', { name: 'File analysis', exact: true }).count(),
                0,
              );
            if (stateCase === 'analysis-read-failed')
              await page.getByText('Fixture rejection', { exact: false }).waitFor();
          }
        }
        await headingContainment(page.locator('.source-workspace .page-heading--intro'));
        await layout(page, `source-${stateCase}-800-dark-larger`);
        assert.equal(
          await page.evaluate(
            () => window.fixture.requests.filter((request) => request.method !== 'GET').length,
          ),
          0,
        );
      } finally {
        await close();
      }
    }
  });
  await test('Source filtering, pagination, busy selection and stale restrictions remain explicit', async () => {
    const paths = [
      'internal/worker/process.go',
      ...Array.from({ length: 105 }, (_, i) => `internal/file-${i}.go`),
    ];
    const { page, close } = await pageFor({ sourcePaths: paths });
    try {
      await nav(page, 'Source');
      assert.equal(await page.locator('.file-item').count(), 100);
      await page.getByRole('button', { name: 'Show more', exact: true }).click();
      assert.equal(await page.locator('.file-item').count(), 106);
      await page.getByLabel('Filter source files').fill('process');
      assert.equal(await page.locator('.file-item').count(), 1);
      await page.getByLabel('Filter source files').fill('');
      assert.equal(await page.locator('.file-item').count(), 100);
      await page.locator('.file-item').first().click();
      await page.getByLabel('Declaration', { exact: true }).selectOption('Process');
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/files/analysis';
      });
      await page.getByRole('button', { name: 'Analyze file', exact: true }).click();
      await page.locator('.busy-strip').waitFor({ state: 'visible' });
      await page.waitForFunction(() =>
        window.fixture.requests.some(
          (request) => request.method === 'POST' && request.path.endsWith('/files/analysis'),
        ),
      );
      assert.equal(await page.getByLabel('Declaration', { exact: true }).isDisabled(), true);
      assert.equal(await page.locator('.file-item:enabled').count(), 0);
      for (const name of ['Analyze file', 'Explain declaration'])
        assert.equal(await page.getByRole('button', { name, exact: true }).isDisabled(), true);
      await page.setViewportSize({ width: 800, height: 1000 });
      await layout(page, 'source-busy-800-dark');
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await idle(page);
      await nav(page, 'Summary');
      await page.evaluate(() => {
        window.fixture.state.changed = true;
      });
      await nav(page, 'Source');
      await page.getByText('File evidence is outdated.', { exact: false }).waitFor();
      assert.equal(await page.getByLabel('Declaration', { exact: true }).isDisabled(), true);
      for (const name of ['Analyze file', 'Explain declaration'])
        assert.equal(await page.getByRole('button', { name, exact: true }).isDisabled(), true);
      await layout(page, 'source-stale-800-dark');
    } finally {
      await page
        .evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        })
        .catch(() => {});
      await close();
    }
  });
  await test('Color hierarchy, readable themes and compact details preserve local navigation', async () => {
    const { page, close } = await pageFor({ unknown: true });
    assert.equal(await page.locator('[data-accent="performance"] .metric-number').innerText(), '—');
    const categories = await page
      .locator('.metric-card .metric-number')
      .evaluateAll((elements) => elements.map((element) => getComputedStyle(element).color));
    assert.equal(new Set(categories).size, 4);
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
  await test('Draft editing retains Summary panels, exact inputs and explicit validation across states', async () => {
    const declaration = `func Process(ctx context.Context) error {\n\t// ${'LongDeclaration'.repeat(60)}\n\treturn ctx.Err()\n}\n`;
    const imports = `  context  \n\nexample.invalid/${'long-import/'.repeat(35)}package\n\tstrings\t\n`;
    const diagnostic = `Validation failed: ${'LongDiagnostic'.repeat(65)} final diagnostic.`;
    for (const invalid of [false, true]) {
      const { page, close } = await pageFor({
        draftImports: ['context', `example.invalid/${'long-import/'.repeat(35)}package`],
        ...(invalid
          ? {
              draftValidation: {
                applicable: false,
                scope_mode: 'single_declaration',
                diagnostics: [{ message: diagnostic }],
              },
            }
          : {}),
      });
      try {
        const references = {};
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page.getByRole('button', { name: 'Switch to light appearance' }).click();
          references[theme] = await panelTreatment(
            page.locator('.summary-details > .panel').first(),
          );
        }
        await page.getByRole('button', { name: 'Switch to dark appearance' }).click();
        await openSource(page);
        assert.equal(
          await page.getByRole('tab', { name: 'Draft', exact: true }).isDisabled(),
          true,
        );
        assert.equal(await page.getByLabel('Declaration draft', { exact: true }).count(), 0);
        await layout(page, 'draft-absent-dark-standard-1440');
        await prepare(page);
        const draft = page.locator('.draft-workspace');
        const editor = page.getByLabel('Declaration draft', { exact: true });
        const importEditor = page.getByLabel('One import per line', { exact: true });
        const passive = async () =>
          page.evaluate(() => window.fixture.requests.filter((entry) => entry.method !== 'GET'));
        const render = async (state, matrix = false) => {
          const before = await passive();
          for (const theme of matrix ? ['dark', 'light'] : ['dark']) {
            if (theme === 'light')
              await page.getByRole('button', { name: 'Switch to light appearance' }).click();
            assert.deepEqual(
              await panelTreatment(draft.locator('.panel').first()),
              references[theme],
            );
            for (const larger of matrix ? [false, true] : [false]) {
              if (larger)
                await page.getByRole('button', { name: 'Larger text', exact: true }).click();
              for (const width of matrix ? [1440, 1280, 1001, 800] : [800]) {
                await page.setViewportSize({ width, height: 1000 });
                const issues = await draft.evaluate((root) => {
                  const bounds = root.getBoundingClientRect();
                  const issues = [];
                  for (const node of root.querySelectorAll('.panel, .button, textarea')) {
                    if (!node.getClientRects().length) continue;
                    const box = node.getBoundingClientRect();
                    if (box.left < bounds.left - 1 || box.right > bounds.right + 1)
                      issues.push(`${node.className} escapes draft`);
                  }
                  const actions = [...root.querySelectorAll(':scope > .actions .button')].map(
                    (node) => node.getBoundingClientRect(),
                  );
                  for (let i = 0; i < actions.length; i++) {
                    for (const other of actions.slice(i + 1)) {
                      const box = actions[i];
                      if (
                        box.left < other.right - 1 &&
                        box.right > other.left + 1 &&
                        box.top < other.bottom - 1 &&
                        box.bottom > other.top + 1
                      )
                        issues.push('Draft continuation actions overlap');
                    }
                  }
                  const editor = root.querySelector('.draft-code');
                  if (
                    editor.getBoundingClientRect().height < 390 ||
                    getComputedStyle(editor).resize !== 'vertical'
                  )
                    issues.push('Draft editor lost height or vertical resize');
                  return issues;
                });
                assert.deepEqual(issues, []);
                await layout(
                  page,
                  `draft-${state}-${theme}-${larger ? 'larger' : 'standard'}-${width}`,
                );
              }
              if (larger)
                await page.getByRole('button', { name: 'Larger text', exact: true }).click();
            }
          }
          if (matrix) await page.getByRole('button', { name: 'Switch to dark appearance' }).click();
          assert.deepEqual(
            await passive(),
            before,
            'Draft disclosures and reflow never validate, execute or write',
          );
        };
        const beforeDisclosure = await passive();
        await draft.getByText('Imports', { exact: true }).click();
        assert.deepEqual(await passive(), beforeDisclosure);
        assert.equal(
          await draft.locator('textarea').count(),
          2,
          'Only declaration and imports are editable',
        );
        assert.deepEqual(await draft.locator('.key-values dd').allTextContents(), [
          'internal/worker/process.go',
          'Process',
          '1',
        ]);
        await render('generated', true);
        assert.equal(
          await draft.getByRole('button', { name: 'Continue to checks', exact: true }).count(),
          0,
        );
        await editor.fill(declaration);
        await importEditor.fill(imports);
        assert.equal(await editor.inputValue(), declaration);
        assert.equal(await importEditor.inputValue(), imports);
        assert.equal(await draft.locator('.key-values dd').last().innerText(), '1 + local edits');
        await render('edited');
        const validationPath = '/api/projects/current/drafts/draft-1/validate';
        await page.evaluate((path) => {
          window.fixture.hold = path;
        }, validationPath);
        await draft.getByRole('button', { name: 'Validate draft', exact: true }).click();
        await page.waitForFunction(
          (path) => window.fixture.requests.some((entry) => entry.path === path),
          validationPath,
        );
        assert.equal(await editor.isDisabled(), true);
        assert.equal(await importEditor.isDisabled(), true);
        assert.equal(
          await draft.getByRole('button', { name: 'Validate draft', exact: true }).isDisabled(),
          true,
        );
        await render('busy-validation');
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
        await idle(page);
        const patch = await page.evaluate(() =>
          window.fixture.requests.find(
            (entry) => entry.method === 'PATCH' && entry.path.endsWith('/drafts/draft-1'),
          ),
        );
        assert.equal(patch.body.declaration, declaration);
        assert.deepEqual(
          patch.body.imports,
          imports
            .split('\n')
            .map((value) => value.trim())
            .filter(Boolean),
        );
        assert.equal(await editor.inputValue(), declaration);
        assert.equal(await draft.locator('.key-values dd').last().innerText(), '2');
        assert.equal(await importEditor.inputValue(), patch.body.imports.join('\n'));
        assert.equal(
          await draft.getByRole('button', { name: 'Continue to checks', exact: true }).count(),
          invalid ? 0 : 1,
        );
        if (invalid)
          assert.equal(
            await draft.locator('.draft-validation [role="alert"]').innerText(),
            diagnostic,
          );
        await render(invalid ? 'invalid-long-diagnostics' : 'validated', true);
        await importEditor.fill(`${imports}\nnet/http`);
        assert.equal(
          await draft.getByRole('button', { name: 'Continue to checks', exact: true }).count(),
          0,
        );
        assert.equal(
          await draft.locator('.draft-validation').count(),
          0,
          'Retained diagnostics cannot approve local edits',
        );
        await page.getByRole('tab', { name: 'Checks', exact: true }).click();
        assert.equal(
          await page.getByRole('button', { name: 'Run checks', exact: true }).isDisabled(),
          true,
        );
        await page.getByRole('tab', { name: 'Draft', exact: true }).click();
        assert.equal(await editor.inputValue(), declaration);
        assert.equal(await importEditor.inputValue(), `${imports}\nnet/http`);
        await render('edited-after-validation');
        await page.evaluate(() => {
          window.fixture.state.changed = true;
        });
        await page.getByRole('tab', { name: 'Assistant', exact: true }).click();
        await page.getByRole('tab', { name: 'Draft', exact: true }).click();
        await page.getByText('File evidence is outdated.', { exact: false }).waitFor();
        assert.equal(await editor.isDisabled(), true);
        assert.equal(await importEditor.isDisabled(), true);
        assert.equal(
          await draft.getByRole('button', { name: 'Validate draft', exact: true }).isDisabled(),
          true,
        );
        await render('stale');
        assert.equal(
          await page.evaluate(() =>
            window.fixture.requests.some((entry) => /\/(checks|apply)$/.test(entry.path)),
          ),
          false,
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      } finally {
        await page
          .evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          })
          .catch(() => {});
        await close();
      }
    }
  });
  await test('Focused checks retain Summary panels, complete evidence and deliberate actions', async () => {
    const command = ['go', 'test', `./${'long-package/'.repeat(45)}worker`];
    const output = `Check diagnostic: ${'LongDiagnostic'.repeat(65)}\nFinal output line.`;
    const attentionChecks = [
      { name: 'parse', required: true, state: 'passed', command: [], output: '', exit_code: 0 },
      { name: 'format', required: true, state: 'failed', command, output, exit_code: 2 },
      { name: 'lint', required: false, state: 'skipped', command, output, exit_code: 0 },
      { name: 'tests', required: false, state: 'canceled', command, output, exit_code: -1 },
    ];
    for (const attention of [false, true]) {
      const { page, close } = await pageFor({
        ...(attention ? { draftChecks: { applicable: false, checks: attentionChecks } } : {}),
      });
      try {
        const references = {};
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page.getByRole('button', { name: 'Switch to light appearance' }).click();
          references[theme] = await panelTreatment(
            page.locator('.summary-details > .panel').first(),
          );
        }
        await page.getByRole('button', { name: 'Switch to dark appearance' }).click();
        await prepare(page);
        await page.getByRole('button', { name: 'Validate draft', exact: true }).click();
        await idle(page);
        await page.getByRole('button', { name: 'Continue to checks', exact: true }).click();
        const surface = page.locator('.checks-workspace');
        const run = surface.getByRole('button', { name: 'Run checks', exact: true });
        const review = surface.getByRole('button', { name: 'Review change', exact: true });
        const benchmarks = surface.getByRole('button', { name: 'Benchmarks', exact: true });
        const passive = async () =>
          page.evaluate(() => window.fixture.requests.filter((entry) => entry.method !== 'GET'));
        assert.equal(await surface.getByLabel('Lint', { exact: true }).isChecked(), false);
        assert.equal(await surface.getByLabel('Tests', { exact: true }).isChecked(), false);
        assert.equal(
          await surface.getByText('Parse & format (required)', { exact: true }).isVisible(),
          true,
        );
        assert.equal(
          await surface.getByRole('heading', { name: 'No checks yet', exact: true }).isVisible(),
          true,
        );
        assert.equal(await run.isEnabled(), true);
        assert.equal(await review.isDisabled(), true);
        assert.equal(await benchmarks.isEnabled(), true);
        for (const width of [1440, 800]) {
          await page.setViewportSize({ width, height: 1000 });
          await focusedChecksLayout(page);
          await layout(page, `checks-absent-dark-standard-${width}-${attention}`);
        }
        if (attention) {
          await surface.getByLabel('Lint', { exact: true }).check();
          await surface.getByLabel('Tests', { exact: true }).check();
        }
        await run.click();
        if (attention)
          await page
            .getByRole('dialog')
            .getByRole('button', { name: 'Trust this project' })
            .click();
        await idle(page);
        const request = await page.evaluate(() =>
          window.fixture.requests.find((entry) => entry.path.endsWith('/drafts/draft-1/checks')),
        );
        assert.equal(request.body.run_lint, attention);
        assert.equal(request.body.run_tests, attention);
        assert.equal(await review.isDisabled(), attention);
        assert.equal(await benchmarks.isEnabled(), true);
        assert.equal(
          await surface.getByRole('button', { name: 'Repair with assistant', exact: true }).count(),
          attention ? 1 : 0,
        );
        assert.equal(
          await surface.locator('.checks-controls .badge').textContent(),
          attention ? 'needs attention' : 'passed',
        );
        if (attention) {
          assert.deepEqual(await surface.locator('.check-evidence .badge').allTextContents(), [
            'passed',
            'failed',
            'skipped',
            'canceled',
          ]);
          assert.deepEqual(
            await surface.locator('.check-evidence .panel-body > .row').allTextContents(),
            ['Required', 'RequiredExit 2', 'Optional', 'OptionalExit -1'],
          );
        }
        const before = await passive();
        for (const summary of await surface.locator('summary').all()) await summary.click();
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page.getByRole('button', { name: 'Switch to light appearance' }).click();
          for (const panel of await surface.locator('.panel').all())
            assert.deepEqual(await panelTreatment(panel), references[theme]);
          for (const larger of [false, true]) {
            if (larger)
              await page.getByRole('button', { name: 'Larger text', exact: true }).click();
            for (const width of [1440, 1280, 1001, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await focusedChecksLayout(page);
              if (attention) {
                for (const name of ['format', 'lint', 'tests']) {
                  assert.equal(
                    await surface.getByLabel(`${name} command`, { exact: true }).textContent(),
                    command.join(' '),
                  );
                  assert.equal(
                    await surface.getByLabel(`${name} output`, { exact: true }).textContent(),
                    output,
                  );
                }
                await surface.getByLabel('format output', { exact: true }).focus();
                assert.equal(
                  await surface
                    .getByLabel('format output', { exact: true })
                    .evaluate(
                      (element) =>
                        element === document.activeElement &&
                        getComputedStyle(element).userSelect !== 'none',
                    ),
                  true,
                );
              }
              await layout(
                page,
                `checks-${attention ? 'failed-skipped-canceled-long-evidence' : 'passed'}-${theme}-${larger ? 'larger' : 'standard'}-${width}`,
              );
            }
            if (larger)
              await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          }
        }
        assert.deepEqual(
          await passive(),
          before,
          'Output disclosure and reflow never run, repair, review or write',
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        if (attention) {
          const path = '/api/projects/current/chat/sessions/session-1/messages';
          const count = await page.evaluate(
            (path) => window.fixture.requests.filter((entry) => entry.path === path).length,
            path,
          );
          await page.evaluate((path) => {
            window.fixture.hold = path;
          }, path);
          const repairAction = surface.getByRole('button', {
            name: 'Repair with assistant',
            exact: true,
          });
          await repairAction.click();
          await page.waitForFunction(
            ({ path, count }) =>
              window.fixture.requests.filter((entry) => entry.path === path).length > count,
            { path, count },
          );
          for (const action of [run, review, benchmarks, repairAction])
            assert.equal(await action.isDisabled(), true);
          for (const name of ['Lint', 'Tests'])
            assert.equal(await surface.getByLabel(name, { exact: true }).isDisabled(), true);
          await focusedChecksLayout(page);
          await layout(page, 'checks-busy-repair-light-standard-800');
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
          await idle(page);
          await page.getByLabel('Declaration draft', { exact: true }).waitFor();
          const repair = await page.evaluate(() =>
            window.fixture.requests.filter((entry) => entry.path.endsWith('/messages')).at(-1),
          );
          assert.equal(repair.body.repair, true);
          assert.equal(repair.body.parent_draft_id, 'draft-1');
          assert.equal(
            repair.body.message,
            'Repair this draft using the failed check diagnostics. Preserve the task scope.',
          );
        } else {
          await benchmarks.click();
          await idle(page);
          assert.equal(
            await page.evaluate(() =>
              window.fixture.requests
                .filter((entry) => entry.path.endsWith('/drafts/draft-1/benchmarks'))
                .every((entry) => entry.method === 'GET'),
            ),
            true,
          );
          await nav(page, 'Source');
          await page.getByRole('tab', { name: 'Checks', exact: true }).click();
          await review.click();
          await idle(page);
          await page.getByRole('heading', { name: 'Review change', exact: true }).waitFor();
          assert.equal(
            await page.evaluate(() =>
              window.fixture.requests.some((entry) => entry.path.endsWith('/apply')),
            ),
            false,
          );
        }
      } finally {
        await page
          .evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          })
          .catch(() => {});
        await close();
      }
    }
  });
  await test('Focused checks preserve invalid, dirty, busy, canceled and stale candidate restrictions', async () => {
    const { page, close } = await pageFor({
      draftValidation: {
        applicable: false,
        scope_mode: 'single_declaration',
        diagnostics: [{ message: 'Invalid declaration' }],
      },
    });
    try {
      await prepare(page);
      await page.getByRole('button', { name: 'Validate draft', exact: true }).click();
      await idle(page);
      await page.getByRole('tab', { name: 'Checks', exact: true }).click();
      const surface = page.locator('.checks-workspace');
      const run = surface.getByRole('button', { name: 'Run checks', exact: true });
      const review = surface.getByRole('button', { name: 'Review change', exact: true });
      const benchmarks = surface.getByRole('button', { name: 'Benchmarks', exact: true });
      const render = async (state) => {
        for (const width of [1440, 800]) {
          await page.setViewportSize({ width, height: 1000 });
          await focusedChecksLayout(page);
          await layout(page, `checks-${state}-dark-standard-${width}`);
        }
      };
      assert.equal(
        await surface
          .getByRole('heading', { name: 'Draft validation required', exact: true })
          .isVisible(),
        true,
      );
      for (const action of [run, review, benchmarks]) assert.equal(await action.isDisabled(), true);
      await render('invalid');
      await page.getByRole('tab', { name: 'Draft', exact: true }).click();
      await page.evaluate(() => {
        delete window.fixture.options.draftValidation;
      });
      await page.getByRole('button', { name: 'Validate draft', exact: true }).click();
      await idle(page);
      await page.getByRole('button', { name: 'Continue to checks', exact: true }).click();
      await surface.getByLabel('Lint', { exact: true }).check();
      await surface.getByLabel('Tests', { exact: true }).check();
      await run.click();
      await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
      await idle(page);
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some((entry) => entry.path.endsWith('/checks')),
        ),
        false,
        'Declining trust does not execute checks',
      );
      const path = '/api/projects/current/drafts/draft-1/checks';
      await page.evaluate((path) => {
        window.fixture.hold = path;
      }, path);
      await run.click();
      await page.getByRole('dialog').getByRole('button', { name: 'Trust this project' }).click();
      await page.waitForFunction(
        (path) => window.fixture.requests.some((entry) => entry.path === path),
        path,
      );
      for (const action of [run, review, benchmarks]) assert.equal(await action.isDisabled(), true);
      for (const name of ['Lint', 'Tests'])
        assert.equal(await surface.getByLabel(name, { exact: true }).isDisabled(), true);
      await render('busy');
      await page
        .locator('.busy-strip')
        .getByRole('button', { name: 'Cancel', exact: true })
        .click();
      await idle(page);
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await page.waitForTimeout(100);
      assert.equal(
        await surface.locator('.check-evidence').count(),
        0,
        'Canceled late response cannot become current evidence',
      );
      assert.equal(await run.isEnabled(), true);
      assert.equal(await review.isDisabled(), true);
      await render('canceled-request');
      await run.click();
      await idle(page);
      assert.equal(await review.isEnabled(), true);
      await page.getByRole('tab', { name: 'Draft', exact: true }).click();
      await page
        .getByLabel('Declaration draft', { exact: true })
        .fill('func Process(ctx context.Context) error { return nil }');
      await page.getByRole('tab', { name: 'Checks', exact: true }).click();
      for (const action of [run, review, benchmarks]) assert.equal(await action.isDisabled(), true);
      assert.equal(await surface.locator('.check-evidence').count(), 0);
      await render('dirty');
      await page.getByRole('tab', { name: 'Draft', exact: true }).click();
      await page.getByRole('button', { name: 'Validate draft', exact: true }).click();
      await idle(page);
      await page.getByRole('button', { name: 'Continue to checks', exact: true }).click();
      await page.evaluate(() => {
        window.fixture.state.changed = true;
      });
      await page.getByRole('tab', { name: 'Source', exact: true }).click();
      await page.getByRole('tab', { name: 'Checks', exact: true }).click();
      await page.getByText('File evidence is outdated.', { exact: false }).waitFor();
      for (const action of [run, review, benchmarks]) assert.equal(await action.isDisabled(), true);
      await render('stale');
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some((entry) => /\/(apply|benchmarks)$/.test(entry.path)),
        ),
        false,
      );
      assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
    } finally {
      await page
        .evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        })
        .catch(() => {});
      await close();
    }
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
  await test('Analysis introduction reuses Summary treatment without changing either workflow', async () => {
    const { page, close } = await pageFor({
      projectName: `Harbor-${'long-project-title'.repeat(12)}`,
      projectPath: `/fixture/${'long-project-path'.repeat(16)}`,
      hasRecovery: true,
      featuresReady: true,
    });
    try {
      await idle(page);
      const calls = await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      );
      for (const width of [1440, 1280, 1001, 800]) {
        await page.setViewportSize({ width, height: 1000 });
        for (const large of [false, true]) {
          const text = page.getByRole('button', { name: 'Larger text', exact: true });
          if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
          for (const theme of ['dark', 'light']) {
            if ((await page.locator('html').getAttribute('data-theme')) !== theme)
              await page.getByRole('button', { name: `Switch to ${theme} appearance` }).click();
            await nav(page, 'Summary');
            await idle(page);
            const hero = page.locator('.summary-hero');
            assert.equal(await hero.count(), 1);
            assert.equal(
              await hero.locator('.summary-hero, .page-heading--intro').count(),
              0,
              'Summary has no nested introduction',
            );
            await headingContainment(hero.locator('.page-heading'));
            const treatment = await introductionTreatment(hero);
            const facts = await hero.locator('.summary-facts').boundingBox();
            const heading = await hero.locator('.page-heading').boundingBox();
            assert.ok(
              facts.y >= heading.y + heading.height,
              'Facts remain below the original inner heading',
            );
            assert.deepEqual(await hero.locator('.summary-facts dd').allTextContents(), [
              'go',
              '18',
              '2,450',
              '0',
            ]);
            assert.equal(await page.locator('.coverage-ring').count(), 1);
            assert.equal(await page.locator('.metric-card').count(), 6);
            const suffix = `${width}-${theme}-${large ? 'larger' : 'standard'}`;
            await layout(page, `summary-introduction-reference-${suffix}`);
            await nav(page, 'Analysis');
            const intro = page.locator('.page-heading--intro');
            await headingContainment(intro);
            assert.deepEqual(
              await introductionTreatment(intro),
              treatment,
              'Analysis shares the maintained Summary surface and button rules',
            );
            assert.equal(
              await intro
                .locator('p')
                .evaluate(
                  (detail) =>
                    getComputedStyle(detail).fontFamily ===
                    getComputedStyle(detail.parentElement.querySelector('h1')).fontFamily,
                ),
              true,
              'Task detail retains ordinary heading typography',
            );
            assert.deepEqual(await intro.getByRole('button').allTextContents(), [
              'Prepare analysis',
              'Repair analysis',
              'Search more feature suggestions',
              'Explore features',
              'Refresh',
            ]);
            assert.equal(await intro.locator('.button.primary').innerText(), 'Prepare analysis');
            assert.equal(await intro.locator('.heading-action-group').count(), 2);
            await layout(page, `analysis-introduction-repair-${suffix}`);
          }
        }
      }
      assert.deepEqual(
        await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        ),
        calls,
        'Navigation and appearance do not admit work or mutate source',
      );
      assert.equal(
        await page.evaluate(() => window.fixture.terminals.length),
        0,
        'No shell starts',
      );
    } finally {
      await close();
    }
  });
  await test('Analysis setup shares Summary panel treatment and full-width section rhythm', async () => {
    for (const empty of [false, true]) {
      const { page, close } = await pageFor({ empty });
      try {
        const reference = await panelTreatment(page.locator('.summary-details > .panel').first());
        const rhythm = await page.locator('.summary-page').evaluate((element) => {
          return getComputedStyle(element).rowGap;
        });
        await nav(page, 'Analysis');
        await idle(page);
        for (const panel of await page.locator('.analysis-sections > .panel').all())
          assert.deepEqual(
            await panelTreatment(panel),
            reference,
            'Analysis panels reuse Summary’s detail treatment without a dashboard minimum height',
          );
        for (const composition of await page.locator('.workspace-page, .analysis-sections').all())
          assert.equal(
            await composition.evaluate((element) => getComputedStyle(element).rowGap),
            rhythm,
            'Page and stacked sections share Summary’s rhythm',
          );
        assert.equal(
          await page
            .locator('.table-wrap')
            .evaluate((element) => getComputedStyle(element).borderRadius),
          reference.radius,
          'The scrolling file table retains the shared panel boundary',
        );
        const disclosure = page.locator('.analysis-sections details');
        const summary = disclosure.locator('summary');
        assert.equal(await disclosure.getAttribute('open'), null, 'Limits start collapsed');
        for (const [label, value, min, max] of [
          ['Files per batch', '20', '1', '500'],
          ['Time budget · seconds', '600', '1', '3600'],
          ['Attempts per stage', '2', '1', '4'],
        ]) {
          const field = page.getByLabel(label, { exact: true });
          assert.equal(await field.inputValue(), value);
          assert.equal(await field.getAttribute('min'), min);
          assert.equal(await field.getAttribute('max'), max);
        }
        assert.equal(await page.getByLabel('Refresh previously analyzed files').isChecked(), false);
        for (const width of [1440, 1280, 1001, 800]) {
          await page.setViewportSize({ width, height: width === 1440 ? 1000 : 900 });
          for (const theme of ['dark', 'light']) {
            if ((await page.locator('html').getAttribute('data-theme')) !== theme)
              await page.getByRole('button', { name: `Switch to ${theme} appearance` }).click();
            for (const large of [false, true]) {
              const text = page.getByRole('button', { name: 'Larger text', exact: true });
              if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
              for (const expanded of [false, true]) {
                if ((await disclosure.getAttribute('open')) !== (expanded ? '' : null))
                  await summary.click();
                await analysisSections(page, !empty);
                await analysisSettingsFields(
                  page,
                  width === 1440 || (width === 1280 && !large) ? 3 : 1,
                  expanded,
                );
                await analysisOverflow(page);
                await layout(
                  page,
                  `analysis-sections-${empty ? 'no-run' : 'saved-run'}-${width}-${theme}-${large ? 'large' : 'default'}-${expanded ? 'expanded' : 'collapsed'}`,
                );
              }
            }
          }
        }
        assert.equal(await page.getByRole('table').count(), 1);
        assert.equal(await page.getByLabel('Filter analysis files').count(), 1);
        for (const action of [
          'Prepare analysis',
          'Search more feature suggestions',
          'Explore features',
          'Refresh',
          'Include shown',
          'Exclude shown',
        ])
          assert.equal(await page.getByRole('button', { name: action, exact: true }).count(), 1);
      } finally {
        await close();
      }
    }
  });
  await test('Analysis file filtering and bulk edits retain explicit Save selection before preparation', async () => {
    const { page, close } = await pageFor({ hasRecovery: true });
    try {
      await nav(page, 'Analysis');
      await idle(page);
      await page.setViewportSize({ width: 800, height: 900 });
      await page.getByRole('button', { name: 'Larger text', exact: true }).click();
      await page.getByRole('button', { name: 'Switch to light appearance' }).click();
      const writes = () =>
        page.evaluate(() => window.fixture.requests.filter((request) => request.method !== 'GET'));
      const before = await writes();
      const filter = page.getByLabel('Filter analysis files', { exact: true });
      await filter.fill('internal/worker/');
      await page.getByRole('button', { name: 'Exclude shown', exact: true }).click();
      for (const path of ['internal/worker/process.go', 'internal/worker/config.go'])
        assert.equal(await page.getByLabel(`Include ${path}`, { exact: true }).isChecked(), false);
      await filter.fill('cmd/');
      assert.equal(await page.getByLabel('Include cmd/server/main.go').isChecked(), true);
      await filter.fill('internal/worker/config.go');
      await page.getByRole('button', { name: 'Include shown', exact: true }).click();
      assert.equal(await page.getByLabel('Include internal/worker/config.go').isChecked(), true);
      await filter.fill('');
      assert.equal(await page.getByLabel('Include internal/worker/process.go').isChecked(), false);
      for (const action of ['Prepare analysis', 'Repair analysis']) {
        await page.getByRole('button', { name: action, exact: true }).click();
        await page
          .getByRole('alert')
          .getByText('Save your file selection before preparing a run.')
          .waitFor();
      }
      assert.deepEqual(
        await writes(),
        before,
        'Filtering and dirty preparation do not save or preview',
      );
      await analysisSections(page, true);
      await analysisOverflow(page);
      await layout(page, 'analysis-dirty-selection-800-light-large');
      await page.getByRole('button', { name: 'Save selection', exact: true }).click();
      await idle(page);
      assert.equal(
        await page.getByRole('button', { name: 'Save selection', exact: true }).count(),
        0,
      );
      const saved = await writes();
      assert.equal(saved.length, before.length + 1, 'Explicit Save is the only write');
      assert.equal(saved.at(-1).path, '/api/projects/current/analysis/selection');
      assert.deepEqual(saved.at(-1).body.excluded_paths, ['internal/worker/process.go']);
      assert.equal(await page.getByLabel('Include internal/worker/process.go').isChecked(), false);
      assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
    } finally {
      await close();
    }
  });
  await test('Analysis settings are keyboard accessible and edits stay local until preparation', async () => {
    const { page, close } = await pageFor({
      hasRecovery: true,
      modelNames: {
        analyze: `review-${'long-model-name-'.repeat(12)}`,
        bug: 'code-model',
        function: 'draft-model',
      },
    });
    try {
      await nav(page, 'Analysis');
      await idle(page);
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      );
      const code = page.getByLabel('Code analysis model', { exact: true });
      const review = page.getByLabel('Performance & Security model', { exact: true });
      const features = page.getByLabel('Feature discovery model', { exact: true });
      assert.equal(
        await code.locator('option[value="analyze"]').textContent(),
        `review-${'long-model-name-'.repeat(12)} · analyze · Local`,
        'Native options retain complete model identifiers',
      );
      await code.focus();
      await page.keyboard.press('d');
      assert.equal(
        await code.inputValue(),
        'function',
        'Native select supports keyboard type-ahead',
      );
      await contrast(page, 'Analysis code selector keyboard focus');
      await page.keyboard.press('Tab');
      assert.equal(await review.evaluate((control) => control === document.activeElement), true);
      await contrast(page, 'Analysis model selector keyboard focus');
      await page.keyboard.press('Tab');
      assert.equal(await features.evaluate((control) => control === document.activeElement), true);
      await contrast(page, 'Analysis feature selector keyboard focus');
      await page.keyboard.press('Tab');
      const disclosure = page.locator('.analysis-sections details');
      const summary = disclosure.locator('summary');
      assert.equal(await summary.evaluate((control) => control === document.activeElement), true);
      await contrast(page, 'Analysis limits disclosure keyboard focus');
      await page.keyboard.press('Enter');
      assert.notEqual(await disclosure.getAttribute('open'), null);
      await page.keyboard.press('Tab');
      const batch = page.getByLabel('Files per batch', { exact: true });
      const budget = page.getByLabel('Time budget · seconds', { exact: true });
      const attempts = page.getByLabel('Attempts per stage', { exact: true });
      const refresh = page.getByLabel('Refresh previously analyzed files', { exact: true });
      assert.equal(await batch.evaluate((control) => control === document.activeElement), true);
      await code.selectOption('function');
      await review.selectOption('bug');
      await features.selectOption('function');
      await batch.fill('37');
      await budget.fill('900');
      await attempts.fill('3');
      await refresh.check();
      await summary.focus();
      await page.keyboard.press('Space');
      assert.equal(await disclosure.getAttribute('open'), null);
      await page.keyboard.press('Enter');
      assert.notEqual(await disclosure.getAttribute('open'), null);
      assert.equal(await batch.inputValue(), '37');
      assert.equal(await budget.inputValue(), '900');
      assert.equal(await attempts.inputValue(), '3');
      assert.equal(await refresh.isChecked(), true);
      await analysisSettingsFields(page, 3, true);
      // Exercise unequal label heights without replacing production labels or controls.
      const magnifiedLabels = await page.addStyleTag({
        content:
          '.analysis-run-settings .analysis-settings-fields > label > span { font-size: 3em; }',
      });
      try {
        const heights = await page
          .locator('.analysis-settings-fields')
          .first()
          .locator('label > span')
          .evaluateAll((labels) =>
            labels.map((label) => {
              const range = document.createRange();
              range.selectNodeContents(label);
              return range.getBoundingClientRect().height;
            }),
          );
        assert.ok(heights[1] > heights[0], 'The longer label wraps to another line');
        await analysisSettingsFields(page, 3, true);
      } finally {
        await magnifiedLabels.evaluate((style) => style.remove());
      }
      await layout(page, 'analysis-settings-long-options-expanded');
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
        'Setup, limits and disclosure edits permit polling reads only',
      );
      assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/analysis/preview';
      });
      await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
      await page.locator('.busy-strip').waitFor();
      await page.waitForFunction(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/analysis/preview')),
      );
      for (const control of [code, review, features])
        assert.equal(await control.isDisabled(), true);
      for (const name of [
        'Prepare analysis',
        'Repair analysis',
        'Search more feature suggestions',
        'Refresh',
        'Include shown',
        'Exclude shown',
      ])
        assert.equal(await page.getByRole('button', { name, exact: true }).isDisabled(), true);
      for (const control of [batch, budget, attempts, refresh])
        assert.equal(await control.isDisabled(), false, 'Limits remain editable while busy');
      await batch.fill('38');
      await budget.fill('901');
      await attempts.fill('4');
      await refresh.uncheck();
      const writes = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      assert.equal(writes.length, before.length + 1, 'Only explicit preparation admits work');
      const request = writes.at(-1);
      assert.equal(request.path, '/api/projects/current/analysis/preview');
      assert.deepEqual(request.body.models, {
        code: 'function',
        review: 'bug',
        features: 'function',
      });
      assert.deepEqual(request.body.limits, {
        batch_files: 37,
        budget_seconds: 900,
        max_attempts_per_stage: 3,
      });
      assert.equal(request.body.refresh, true);
      assert.equal(request.body.include_features, true);
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await idle(page);
      await page.getByRole('heading', { name: 'Ready to analyze', exact: true }).waitFor();
      assert.equal(
        await page.getByRole('dialog').count(),
        0,
        'Preparation still requires explicit Start',
      );
    } finally {
      try {
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
      } finally {
        await close();
      }
    }
  });
  await test('Analysis empty model catalog keeps unavailable native selects disabled', async () => {
    const { page, close } = await pageFor({ emptyModelCatalog: true });
    try {
      await nav(page, 'Analysis');
      await idle(page);
      for (const [label, value] of [
        ['Code analysis model', 'bug'],
        ['Performance & Security model', 'analyze'],
        ['Feature discovery model', 'analyze'],
      ]) {
        const select = page.getByLabel(label, { exact: true });
        assert.equal(await select.isDisabled(), true);
        assert.equal(await select.inputValue(), value);
        assert.deepEqual(await select.locator('option').allTextContents(), ['Models unavailable']);
      }
      // An empty catalog is present metadata; do not add a new preparation eligibility rule.
      assert.equal(
        await page.getByRole('button', { name: 'Prepare analysis', exact: true }).isDisabled(),
        false,
      );
      await analysisSettingsFields(page, 3, false);
    } finally {
      await close();
    }
  });
  await test('Analysis captured identifiers wrap in the real shell and stay distinct from setup', async () => {
    const saved = savedModelsFixture(true);
    const { page, close } = await pageFor({
      ...saved.options,
      modelNames: { analyze: 'current-review', bug: 'current-code', function: 'current-features' },
    });
    try {
      await nav(page, 'Analysis');
      await idle(page);
      const lastRun = page.locator('.analysis-last-run');
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      );
      const code = page.getByLabel('Code analysis model', { exact: true });
      assert.equal(await code.inputValue(), 'bug');
      assert.equal(await code.locator('option:checked').innerText(), 'current-code · bug · Local');
      await code.selectOption('function');
      await page.getByLabel('Performance & Security model', { exact: true }).selectOption('bug');
      await page.getByLabel('Feature discovery model', { exact: true }).selectOption('analyze');
      const disclosure = page.locator('.analysis-run-settings details');
      for (const width of [1440, 1280, 1001, 800]) {
        await page.setViewportSize({ width, height: width === 1440 ? 1000 : 900 });
        for (const large of [false, true]) {
          const text = page.getByRole('button', { name: 'Larger text', exact: true });
          if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
          for (const expanded of [false, true]) {
            if ((await disclosure.getAttribute('open')) !== (expanded ? '' : null))
              await disclosure.locator('summary').click();
            await analysisSections(page, true);
            const columns = width === 1440 || (width === 1280 && !large) ? 3 : 1;
            await analysisSettingsFields(page, columns, expanded);
            await capturedDetails(lastRun, saved.details, columns);
            await analysisOverflow(page);
            for (const theme of ['dark', 'light']) {
              if ((await page.locator('html').getAttribute('data-theme')) !== theme)
                await page.getByRole('button', { name: `Switch to ${theme} appearance` }).click();
              // Tab from the last settings control reaches the saved-run action in DOM order.
              const previous = expanded
                ? page.getByLabel('Refresh previously analyzed files')
                : disclosure.locator('summary');
              await previous.focus();
              await page.keyboard.press('Tab');
              const view = lastRun.getByRole('button', { name: 'View run', exact: true });
              assert.equal(
                await view.evaluate((button) => button === document.activeElement),
                true,
              );
              assert.equal(await view.evaluate((button) => button.matches(':focus-visible')), true);
              await contrast(page, `Captured models ${width} ${large} ${expanded} ${theme} focus`);
              await layout(
                page,
                `analysis-captured-long-${width}-${large ? 'large' : 'default'}-${expanded ? 'expanded' : 'collapsed'}-${theme}`,
              );
            }
          }
        }
      }
      assert.equal(await code.inputValue(), 'function', 'Captured rendering does not reset setup');
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
        'Setup, disclosure, appearance and focus changes permit only existing reads',
      );
      assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
    } finally {
      await close();
    }
  });
  await test('Captured metadata and fallbacks preserve Analysis, run and preview caller semantics', async () => {
    const saved = savedModelsFixture();
    // Run retains its existing viewport reflow; preview uses the captured panel's own width.
    const origins = ['http://c.test', 'https://r.test', 'https://f.test'];
    saved.details.forEach((detail, index) => {
      detail.origin = origins[index];
      saved.options.capturedProviders[index].model.provider_origin = origins[index];
    });
    for (const scenario of ['populated', 'missing-provider', 'legacy']) {
      const options =
        scenario === 'legacy'
          ? {}
          : {
              ...saved.options,
              ...(scenario === 'missing-provider'
                ? { capturedProviders: saved.options.capturedProviders.slice(0, 2) }
                : {}),
            };
      const expected =
        scenario === 'legacy'
          ? null
          : saved.details.map((detail, index) =>
              scenario === 'missing-provider' && index === 2
                ? { label: detail.label, profile: detail.profile }
                : detail,
            );
      const { page, close } = await pageFor({ ...options, runStatus: 'paused' });
      try {
        await nav(page, 'Analysis');
        await idle(page);
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        );
        for (const [width, large] of [
          [1440, false],
          [1001, true],
          [800, true],
        ]) {
          await page.setViewportSize({ width, height: width === 1440 ? 1000 : 900 });
          const text = page.getByRole('button', { name: 'Larger text', exact: true });
          if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
          await capturedDetails(
            page.locator('.analysis-last-run'),
            expected,
            width === 1440 ? 3 : 1,
          );
          await analysisSections(page, true);
          await analysisOverflow(page);
          await layout(
            page,
            `analysis-captured-${scenario}-${width}-${large ? 'large' : 'default'}`,
          );
        }
        await page.locator('.analysis-last-run').getByRole('button', { name: 'View run' }).click();
        await page.getByRole('heading', { name: 'Project analysis', exact: true }).waitFor();
        const runPanel = page.locator('section.panel').filter({
          has: page.getByRole('heading', { name: 'Captured models', exact: true }),
        });
        for (const width of [1440, 1001, 800]) {
          await page.setViewportSize({ width, height: width === 1440 ? 1000 : 900 });
          await capturedDetails(runPanel, expected, width === 1440 ? 3 : 1);
          await layout(page, `analysis-shared-run-${scenario}-${width}`);
        }
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
          'Viewing the saved run does not prepare or start it',
        );
        await page.getByRole('button', { name: 'Prepare continuation', exact: true }).click();
        await idle(page);
        await page.getByRole('heading', { name: 'Continue analysis', exact: true }).waitFor();
        const previewPanel = page.locator('section.panel').filter({
          has: page.getByRole('heading', { name: 'Models', exact: true }),
        });
        for (const width of [1440, 1001, 800]) {
          await page.setViewportSize({ width, height: width === 1440 ? 1000 : 900 });
          await capturedDetails(previewPanel, expected, 1);
          await analysisPreviewLayout(page);
          await layout(page, `analysis-shared-preview-${scenario}-${width}`);
        }
        assert.equal(await page.getByRole('dialog').count(), 0);
        const writes = await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        );
        assert.equal(writes.length, before.length + 1);
        assert.equal(writes.at(-1).path, '/api/projects/current/analysis/preview');
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      } finally {
        await close();
      }
    }
  });
  await test('Analysis previews share Summary hierarchy and keep captured scope and consent visible', async () => {
    const saved = savedModelsFixture(true);
    const path = `internal/${'longpathsegment'.repeat(20)}/worker.go`;
    const excludedPath = `private/${'excludedsegment'.repeat(20)}/notes.md`;
    const reason = `Excluded by context policy: ${'policyreason'.repeat(25)}`;
    const features = {
      expected_hash: 'features-empty',
      goals_hash: 'goals-hash',
      workspace_hash: 'workspace-hash',
      excluded_paths: [excludedPath],
      provider_id: 'saved-provider-2',
      max_model_requests: 2,
    };
    for (const mode of ['new', 'repair', 'continuation']) {
      const { page, close } = await pageFor({
        ...saved.options,
        hasRecovery: mode === 'repair',
        runStatus: mode === 'continuation' ? 'paused' : 'completed',
        previewOverride: {
          models: saved.options.capturedModels,
          providers: saved.options.capturedProviders,
          files: [
            {
              path,
              content_hash: 'long-file-hash',
              language: 'go',
              size_bytes: 2048,
              stages: [
                { stage: 'semantic', eligible: true, cached: false, max_model_requests: 3 },
                { stage: 'performance', eligible: true, cached: true, max_model_requests: 3 },
                { stage: 'security_ai', eligible: false, cached: false, max_model_requests: 0 },
              ],
            },
          ],
          excluded: [{ path: excludedPath, reason }],
          limits: { batch_files: 7, budget_seconds: 1234, max_attempts_per_stage: 3 },
          refresh: mode === 'repair',
          expected_model_requests: 3,
          max_model_requests: 8,
          features: mode === 'repair' ? null : features,
        },
      });
      try {
        const introReference = await introductionTreatment(page.locator('.summary-hero'));
        const panelReference = await panelTreatment(
          page.locator('.summary-details > .panel').first(),
        );
        await nav(page, 'Analysis');
        await idle(page);
        if (mode === 'continuation') {
          // Today's editable setup deliberately differs from the saved feature profile.
          assert.equal(await page.getByLabel('Feature discovery model').inputValue(), 'function');
          await page.getByLabel('Feature discovery model').selectOption('analyze');
          await page.getByRole('button', { name: 'View run', exact: true }).click();
          await page.getByRole('button', { name: 'Prepare continuation', exact: true }).click();
        } else {
          await page.getByLabel('Feature discovery model').selectOption('function');
          await page
            .getByRole('button', {
              name: mode === 'repair' ? 'Repair analysis' : 'Prepare analysis',
              exact: true,
            })
            .click();
        }
        await idle(page);
        await page
          .getByRole('heading', {
            name:
              mode === 'repair'
                ? 'Repair analysis'
                : mode === 'continuation'
                  ? 'Continue analysis'
                  : 'Ready to analyze',
            exact: true,
          })
          .waitFor();
        assert.deepEqual(
          await introductionTreatment(page.locator('.page-heading--intro')),
          introReference,
        );
        for (const panel of await page.locator('.analysis-preview section.panel').all())
          assert.deepEqual(await panelTreatment(panel), panelReference);
        const scope = page.locator('section.panel').filter({
          has: page.getByRole('heading', {
            name: mode === 'repair' ? 'Repair scope' : 'Scope',
            exact: true,
          }),
        });
        assert.deepEqual(await scope.locator('.mini-metrics strong').allTextContents(), [
          '1',
          '3',
          '8',
        ]);
        assert.deepEqual(await scope.locator('.key-values dd').allTextContents(), [
          '7 files',
          '1234 seconds',
          '3',
          mode === 'repair' ? 'Refresh' : 'Reuse when current',
        ]);
        await page
          .getByText('Start confirms any remote context sharing and AI Security review.', {
            exact: true,
          })
          .waitFor();
        if (mode === 'repair')
          await page
            .getByText(
              'Repair re-attempts unfinished analysis work. It does not modify your source code.',
              { exact: true },
            )
            .waitFor();
        if (mode === 'continuation') {
          await page
            .getByText("Resuming keeps this run's model choices.", { exact: true })
            .waitFor();
          await page
            .getByText(/Your current setup choices differ from this run's captured choices/)
            .waitFor();
        }
        const selected = page.locator('section.panel').filter({
          has: page.getByRole('heading', { name: 'Selected files', exact: true }),
        });
        assert.equal(await selected.getByText(path, { exact: true }).count(), 1);
        assert.equal(
          await selected.getByText('Code / Performance · cached', { exact: true }).count(),
          1,
        );
        const calls = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        const disclosure = selected.locator('details');
        assert.equal(await disclosure.getAttribute('open'), null);
        await disclosure.locator('summary').click();
        assert.equal(await selected.getByText(excludedPath, { exact: true }).isVisible(), true);
        assert.equal(await selected.getByText(reason, { exact: true }).isVisible(), true);
        assert.equal(calls.at(-1).path, '/api/projects/current/analysis/preview');
        assert.equal(calls.at(-1).body.recover_incomplete, mode === 'repair');
        assert.equal(calls.at(-1).body.include_features, mode !== 'repair');
        if (mode === 'continuation') assert.equal(calls.at(-1).body.resume_run.id, 'run-1');
        for (const width of [1440, 1280, 1001, 800]) {
          await page.setViewportSize({ width, height: 1000 });
          for (const theme of ['dark', 'light']) {
            if ((await page.locator('html').getAttribute('data-theme')) !== theme)
              await page.getByRole('button', { name: `Switch to ${theme} appearance` }).click();
            for (const large of [false, true]) {
              const text = page.getByRole('button', { name: 'Larger text', exact: true });
              if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
              await capturedDetails(page.locator('.analysis-preview-models'), saved.details, 1);
              await analysisPreviewLayout(page);
              await layout(
                page,
                `analysis-preview-${mode}-long-${width}-${theme}-${large ? 'large' : 'standard'}`,
              );
            }
          }
        }
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          calls,
          'Preview inspection, disclosures and appearance do not admit work or write source',
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        const action = page.getByRole('button', {
          name:
            mode === 'repair'
              ? 'Start repair'
              : mode === 'continuation'
                ? 'Resume analysis'
                : 'Start analysis',
          exact: true,
        });
        assert.equal(await action.isDisabled(), false);
        await action.click();
        const dialog = page.getByRole('dialog');
        await dialog.getByText(/AI Security review/).waitFor();
        assert.equal(
          await page.locator('.page-heading--intro .button.primary').isDisabled(),
          true,
          'Admission remains disabled while confirmation is pending',
        );
        const destinations = mode === 'repair' ? [saved.details[1]] : saved.details.slice(1);
        for (const destination of destinations)
          await dialog
            .getByText(`${destination.model} · ${destination.origin}`, { exact: true })
            .waitFor();
        assert.equal(
          await dialog
            .getByText(`${saved.details[0].model} · ${saved.details[0].origin}`, { exact: true })
            .count(),
          0,
          'Local Code destination does not require remote confirmation',
        );
        if (mode === 'repair')
          assert.equal(
            await dialog
              .getByText(`${saved.details[2].model} · ${saved.details[2].origin}`, { exact: true })
              .count(),
            0,
            'Repair does not request feature discovery confirmation',
          );
        await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
        await idle(page);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          calls,
          'Canceling fresh consent leaves admission untouched',
        );
      } finally {
        await close();
      }
    }
  });
  await test('Analysis preview handles empty scope, feature-only admission and missing captured evidence', async () => {
    const saved = savedModelsFixture();
    for (const scenario of [
      'empty',
      'feature-only',
      'missing-models',
      'missing-provider',
      'absent',
    ]) {
      const { page, close } = await pageFor({
        ...saved.options,
        runStatus: 'paused',
        previewOverride:
          scenario === 'absent'
            ? null
            : scenario === 'empty' || scenario === 'feature-only'
              ? {
                  files: [],
                  features:
                    scenario === 'empty'
                      ? null
                      : {
                          expected_hash: 'features-empty',
                          goals_hash: 'goals-hash',
                          workspace_hash: 'workspace-hash',
                          excluded_paths: [],
                          provider_id: 'saved-provider-2',
                          max_model_requests: 2,
                        },
                  expected_model_requests: scenario === 'empty' ? 0 : 1,
                  max_model_requests: scenario === 'empty' ? 0 : 2,
                }
              : scenario === 'missing-models'
                ? { models: null }
                : { providers: saved.options.capturedProviders.slice(0, 2) },
      });
      try {
        await nav(page, 'Analysis');
        await idle(page);
        await page.getByRole('button', { name: 'View run', exact: true }).click();
        await page.getByRole('button', { name: 'Prepare continuation', exact: true }).click();
        await idle(page);
        const calls = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        if (scenario === 'absent') {
          await page.getByRole('heading', { name: 'Prepare a new preview', exact: true }).waitFor();
          assert.equal(
            await page.getByRole('button', { name: 'Resume analysis', exact: true }).count(),
            0,
          );
        } else {
          const start = page.getByRole('button', { name: 'Resume analysis', exact: true });
          assert.equal(await start.isDisabled(), scenario === 'empty');
          if (scenario === 'empty' || scenario === 'feature-only')
            await page.getByRole('heading', { name: 'No selected files', exact: true }).waitFor();
          if (scenario === 'missing-models' || scenario === 'missing-provider')
            await capturedDetails(
              page.locator('.analysis-preview-models'),
              scenario === 'missing-models'
                ? null
                : saved.details.map((detail, i) =>
                    i === 2 ? { label: detail.label, profile: detail.profile } : detail,
                  ),
              1,
            );
        }
        for (const width of [1440, 800]) {
          await page.setViewportSize({ width, height: 1000 });
          for (const theme of ['dark', 'light']) {
            if ((await page.locator('html').getAttribute('data-theme')) !== theme)
              await page.getByRole('button', { name: `Switch to ${theme} appearance` }).click();
            for (const large of [false, true]) {
              const text = page.getByRole('button', { name: 'Larger text', exact: true });
              if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
              if (scenario === 'absent')
                await headingContainment(page.locator('.page-heading--intro'));
              else await analysisPreviewLayout(page);
              await layout(
                page,
                `analysis-preview-${scenario}-${width}-${theme}-${large ? 'large' : 'standard'}`,
              );
            }
          }
        }
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          calls,
        );
        assert.equal(await page.getByRole('dialog').count(), 0);
        if (scenario === 'feature-only') {
          await page.getByRole('button', { name: 'Resume analysis', exact: true }).click();
          const dialog = page.getByRole('dialog');
          await dialog.getByText('0 selected files', { exact: true }).waitFor();
          await dialog
            .getByText(`${saved.details[2].model} · ${saved.details[2].origin}`, { exact: true })
            .waitFor();
          await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
          await idle(page);
          assert.deepEqual(
            await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
            calls,
          );
        }
        await page
          .getByRole('button', {
            name: scenario === 'absent' ? 'Back to analysis' : 'Back',
            exact: true,
          })
          .click();
        await page.getByRole('heading', { name: 'Analysis', exact: true }).waitFor();
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          calls,
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      } finally {
        await close();
      }
    }
  });
  await test('Analysis last-run status preserves eligibility and passive View run navigation', async () => {
    for (const status of ['running', 'paused', 'failed', 'completed']) {
      for (const hasRecovery of [false, true]) {
        const { page, close } = await pageFor({ runStatus: status, hasRecovery });
        try {
          await nav(page, 'Analysis');
          await idle(page);
          const lastRun = page.locator('section.panel').filter({
            has: page.getByRole('heading', { name: 'Last run', exact: true }),
          });
          await lastRun.getByText(status, { exact: true }).waitFor();
          assert.equal(
            await lastRun.getByRole('img').getAttribute('aria-label'),
            `Analysis: ${status}`,
          );
          assert.equal(
            await page.getByRole('button', { name: 'Prepare analysis', exact: true }).isDisabled(),
            status === 'running',
          );
          const repair = page.getByRole('button', { name: 'Repair analysis', exact: true });
          const canRepair = hasRecovery && ['failed', 'completed'].includes(status);
          assert.equal(await repair.count(), canRepair ? 1 : 0);
          if (canRepair) assert.equal(await repair.isDisabled(), false);
          assert.equal(
            await page.getByRole('button', { name: 'Include shown', exact: true }).isDisabled(),
            ['running', 'paused'].includes(status),
          );
          const before = await page.evaluate(() =>
            window.fixture.requests.filter((request) => request.method !== 'GET'),
          );
          await lastRun.getByRole('button', { name: 'View run', exact: true }).click();
          await page.getByRole('heading', { name: 'Project analysis', exact: true }).waitFor();
          assert.equal(await page.getByRole('dialog').count(), 0);
          assert.deepEqual(
            await page.evaluate(() =>
              window.fixture.requests.filter((request) => request.method !== 'GET'),
            ),
            before,
            'Viewing a saved run permits polling reads but no admission, control or writes',
          );
          assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        } finally {
          await close();
        }
      }
    }
  });
  await test('Analysis preview requires consent; pause and resume use captured identities', async () => {
    const { page, close } = await pageFor({ remote: true });
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.getByRole('button', { name: 'Start analysis', exact: true }).isDisabled(),
      false,
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
    assert.equal(await page.getByRole('checkbox').count(), 0);
    await page.getByRole('button', { name: 'Start analysis', exact: true }).click();
    await page
      .getByRole('dialog')
      .getByText(/https:\/\/provider.invalid/)
      .first()
      .waitFor();
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
    await idle(page);
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some(
          (r) => r.method === 'POST' && r.path.endsWith('/analysis/run'),
        ),
      ),
      false,
    );
    await startAnalysis(page);
    await page.getByRole('button', { name: 'Pause', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Prepare continuation', exact: true }).click();
    await idle(page);
    assert.equal(await page.getByRole('checkbox').count(), 0);
    assert.equal(
      await page.getByText("Resuming keeps this run's model choices.").isVisible(),
      true,
    );
    await startAnalysis(page, true);
    const resumed = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.body?.action === 'resume'),
    );
    assert.equal(resumed.body.identity.generation, 'generation-1');
    assert.equal(resumed.body.preview_id, 'preview-bug-analyze-analyze');
    await layout(page, 'analysis-run');
    await close();
  });
  await test('Configured model selects refresh admission and remain captured on resume', async () => {
    const { page, close } = await pageFor({
      remote: true,
      modelNames: { analyze: 'review-model', bug: 'code-model', function: 'draft-model' },
    });
    await nav(page, 'Analysis');
    const before = await page.evaluate(() => window.fixture.requests.length);
    assert.equal(await page.getByLabel('Code analysis model').inputValue(), 'bug');
    assert.equal(await page.getByLabel('Performance & Security model').inputValue(), 'analyze');
    assert.equal(await page.getByLabel('Feature discovery model').inputValue(), 'analyze');
    assert.deepEqual(
      await page.getByLabel('Code analysis model').locator('option').allTextContents(),
      [
        'review-model · analyze · Remote',
        'code-model · bug · Remote',
        'draft-model · function · Remote',
      ],
    );
    await page.getByLabel('Code analysis model').selectOption('function');
    await page.getByLabel('Performance & Security model').selectOption('bug');
    await page.getByLabel('Feature discovery model').selectOption('analyze');
    assert.equal(await page.evaluate(() => window.fixture.requests.length), before);
    await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
    await idle(page);
    const choices = { code: 'function', review: 'bug', features: 'analyze' };
    const previews = await page.evaluate(() =>
      window.fixture.requests.filter((r) => r.path.endsWith('/analysis/preview')),
    );
    assert.equal(previews.length, 1);
    assert.deepEqual(previews.at(-1).body.models, choices);
    assert.equal(await page.getByRole('checkbox').count(), 0);
    await layout(page, 'analysis-captured-preview');
    await page.setViewportSize({ width: 800, height: 900 });
    await page.getByRole('button', { name: 'Larger text' }).click();
    await layout(page, 'analysis-captured-preview-compact');
    await page.getByRole('button', { name: 'Start analysis', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByText(/include AI Security review/).waitFor();
    for (const name of ['code-model', 'draft-model', 'review-model'])
      await dialog.getByText(new RegExp(name)).waitFor();
    await dialog.getByRole('button', { name: 'Start', exact: true }).click();
    await idle(page);
    const start = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.method === 'POST' && r.path.endsWith('/analysis/run')),
    );
    assert.deepEqual(start.body.models, choices);
    assert.equal(start.body.preview_id, 'preview-function-bug-analyze');
    assert.deepEqual(start.body.confirmations.provider_ids, [
      'code-function',
      'review-bug',
      'features-analyze',
    ]);
    assert.equal(start.body.confirmations.security_review, true);
    await page.getByRole('button', { name: 'Pause', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Prepare continuation', exact: true }).click();
    await idle(page);
    for (const [label, key] of [
      ['Code', 'draft-model'],
      ['Performance & Security', 'code-model'],
      ['Feature discovery', 'review-model'],
    ]) {
      // CapturedModels doesn't have a label/input, it just renders text
      await page.getByText(key, { exact: false }).first().waitFor();
    }
    await page.getByRole('button', { name: 'Resume analysis', exact: true }).click();
    assert.equal(
      await page.evaluate(() => window.fixture.requests.some((r) => r.body?.action === 'resume')),
      false,
    );
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
    await idle(page);
    await startAnalysis(page, true);
    const resume = await page.evaluate(() =>
      window.fixture.requests.find((r) => r.body?.action === 'resume'),
    );
    assert.equal(resume.body.preview_id, start.body.preview_id);
    assert.deepEqual(resume.body.confirmations, start.body.confirmations);
    await close();
  });
  await test('Analysis exposes provider progress and keeps saved results available', async () => {
    const { page, close } = await pageFor();
    await nav(page, 'Analysis');
    await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
    await idle(page);
    await startAnalysis(page);
    await page
      .getByRole('heading', { name: 'New feature suggestions', exact: true })
      .locator('..')
      .getByRole('img', { name: 'Features: pending', exact: true })
      .waitFor();
    await page.evaluate(() => {
      window.fixture.state.run.features.status = 'running';
      window.fixture.state.run.features.attempts = 1;
      window.fixture.state.run.elapsed_seconds = 75;
      window.fixture.state.run.files[0].stages[1].status = 'running';
    });
    const activity = page.getByRole('status', { name: 'Current analysis step' });
    await activity.getByText('Generating feature suggestions…', { exact: true }).waitFor();
    await activity.getByText('Performance · internal/worker/process.go', { exact: true }).waitFor();
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
  await test('Analysis run lifecycle wording and controls stay truthful and passive', async () => {
    const saved = savedModelsFixture();
    for (const status of [
      'queued',
      'running',
      'pausing',
      'paused',
      'interrupted',
      'canceling',
      'canceled',
      'failed',
      'partial',
      'completed',
    ]) {
      const active = ['queued', 'running', 'pausing', 'canceling'].includes(status);
      const resumable = ['paused', 'interrupted'].includes(status);
      const featureStatus = status === 'queued' ? 'pending' : status;
      const reason = ['paused', 'interrupted', 'canceled', 'failed', 'partial'].includes(status)
        ? `Analysis ${status}: saved evidence remains available.`
        : '';
      const { page, close } = await pageFor({
        ...saved.options,
        runStatus: status,
        runOverride: {
          reason,
          features: {
            status: featureStatus,
            attempts: 1,
            suggestion_count: status === 'completed' ? 0 : null,
          },
        },
      });
      try {
        await nav(page, 'Analysis');
        await page.getByRole('button', { name: 'View run', exact: true }).click();
        const workspace = page.locator('.analysis-run');
        const heading = workspace.locator('.page-heading--intro');
        await heading.getByText(status, { exact: true }).waitFor();
        assert.equal(
          await heading.getByRole('img').getAttribute('aria-label'),
          `Analysis: ${status}`,
        );
        assert.equal(
          await heading.getByRole('button', { name: 'Pause', exact: true }).count(),
          ['queued', 'running'].includes(status) ? 1 : 0,
        );
        assert.equal(
          await heading.getByRole('button', { name: 'Prepare continuation', exact: true }).count(),
          resumable ? 1 : 0,
        );
        const cancel = heading.getByRole('button', { name: 'Cancel run', exact: true });
        assert.equal(await cancel.count(), active || resumable ? 1 : 0);
        if (active || resumable) assert.equal(await cancel.isDisabled(), status === 'canceling');
        assert.equal(
          await workspace.getByRole('status', { name: 'Current analysis step' }).count(),
          active ? 1 : 0,
        );
        if (['queued', 'pausing', 'canceling'].includes(status))
          await workspace
            .getByText(
              {
                queued: 'Preparing analysis…',
                pausing: 'Finishing the current step before pausing…',
                canceling: 'Canceling analysis…',
              }[status],
              { exact: true },
            )
            .waitFor();
        if (reason) {
          const notice = workspace.locator(':scope > .notice');
          assert.equal(await notice.innerText(), reason);
          assert.equal(await notice.locator('details').count(), 0);
        }
        const features = workspace.locator('section.panel').filter({
          has: page.getByRole('heading', { name: 'New feature suggestions', exact: true }),
        });
        await features.locator('.panel-head').getByText(featureStatus, { exact: true }).waitFor();
        assert.equal(
          await features.locator('.metric-number').innerText(),
          status === 'completed' ? '0' : '—',
        );
        for (const category of ['bugs', 'performance', 'security']) {
          const panel = workspace
            .locator('section.panel')
            .filter({ has: page.getByRole('heading', { name: category, exact: true }) });
          assert.equal(await panel.locator('.panel-head .analysis-run-status').innerText(), status);
        }
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        const disclosure = workspace.locator('details').first();
        await disclosure.locator('summary').press('Enter');
        assert.equal(await disclosure.getAttribute('open'), '');
        await disclosure.getByText('completed', { exact: true }).first().waitFor();
        await analysisRunLayout(page);
        await layout(page, `analysis-run-${status}-1440-dark-standard`);
        await page.setViewportSize({ width: 800, height: 900 });
        await page.getByRole('button', { name: 'Switch to light appearance', exact: true }).click();
        await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await analysisRunLayout(page);
        await layout(page, `analysis-run-${status}-800-light-larger`);
        await workspace
          .getByRole('button', { name: 'Open feature suggestions', exact: true })
          .click();
        await nav(page, 'Analysis');
        await page.getByRole('button', { name: 'View run', exact: true }).click();
        for (const index of [0, 1, 2]) {
          await workspace
            .getByRole('button', { name: 'Open results', exact: true })
            .nth(index)
            .click();
          await idle(page);
          await nav(page, 'Analysis');
          await page.getByRole('button', { name: 'View run', exact: true }).click();
        }
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
          'Run disclosures and result navigation allow local reads/polling, not admission, execution or writes',
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        assert.equal(await page.getByRole('dialog').count(), 0);
      } finally {
        await close();
      }
    }
  });
  await test('Analysis run long evidence retains independent progress and unknown versus zero counts', async () => {
    const saved = savedModelsFixture(true);
    const path = `internal/${'longpathsegment'.repeat(30)}/worker.go`;
    const reason = `Feature request failed: ${'diagnostic'.repeat(35)}. File analysis remains available.`;
    const stageReason = `Stage unavailable: ${'stage-diagnostic'.repeat(25)}`;
    const { page, close } = await pageFor({
      ...saved.options,
      runStatus: 'running',
      runOverride: {
        elapsed_seconds: 0,
        window_files_completed: 0,
        features: { status: 'failed', attempts: 2, suggestion_count: null, reason },
        files: [
          {
            path,
            content_hash: 'long-file-hash',
            language: 'go',
            stages: [
              {
                stage: 'semantic',
                status: 'completed',
                attempts: 1,
                cached: true,
                finding_count: 0,
              },
              {
                stage: 'performance',
                status: 'running',
                attempts: 2,
                cached: false,
                finding_count: null,
              },
              {
                stage: 'security_rules',
                status: 'unavailable',
                attempts: 0,
                cached: false,
                finding_count: null,
                reason: stageReason,
              },
              {
                stage: 'security_ai',
                status: 'failed',
                attempts: 2,
                cached: false,
                finding_count: null,
                reason: stageReason,
              },
            ],
          },
        ],
        sections: [
          {
            category: 'bugs',
            status: 'partial',
            finding_count: null,
            coverage: {
              total: 4,
              succeeded: 1,
              partial: 1,
              failed: 0,
              pending: 1,
              running: 1,
              skipped: 0,
              unavailable: 0,
            },
          },
          {
            category: 'performance',
            status: 'failed',
            finding_count: 0,
            coverage: {
              total: 4,
              succeeded: 0,
              partial: 0,
              failed: 1,
              pending: 2,
              running: 0,
              skipped: 1,
              unavailable: 0,
            },
          },
          {
            category: 'security',
            status: 'unavailable',
            finding_count: 2,
            coverage: {
              total: 4,
              succeeded: 1,
              partial: 0,
              failed: 0,
              pending: 2,
              running: 0,
              skipped: 0,
              unavailable: 1,
            },
          },
        ],
      },
    });
    try {
      const referenceIntro = await introductionTreatment(page.locator('.summary-hero'));
      const referencePanel = await panelTreatment(page.locator('.summary-details .panel').first());
      await nav(page, 'Analysis');
      await page.getByRole('button', { name: 'View run', exact: true }).click();
      const workspace = page.locator('.analysis-run');
      const models = workspace.locator('.analysis-run-models');
      assert.deepEqual(
        await introductionTreatment(workspace.locator('.page-heading--intro')),
        referenceIntro,
      );
      assert.deepEqual(await panelTreatment(models), referencePanel);
      await workspace
        .getByRole('status', { name: 'Current analysis step' })
        .getByText(`Performance · ${path}`, { exact: true })
        .waitFor();
      assert.equal(await workspace.getByText('0s elapsed', { exact: true }).count(), 1);
      assert.equal(
        await workspace
          .getByRole('heading', { name: '0 files completed this batch', exact: true })
          .count(),
        1,
      );
      const progress = workspace.getByRole('progressbar', { name: 'Analysis progress' });
      assert.equal(await progress.getAttribute('value'), '6');
      assert.equal(await progress.getAttribute('max'), '12');
      assert.equal(
        await workspace.getByText('6 of 12 category work units finished', { exact: true }).count(),
        1,
      );
      assert.deepEqual(
        await workspace.locator('.analysis-run-categories .metric-number').allTextContents(),
        ['—', '0', '2'],
      );
      assert.equal(
        await workspace.getByText('No successful evidence yet', { exact: true }).count(),
        1,
      );
      assert.equal(await workspace.getByText('Saved findings', { exact: true }).count(), 2);
      const featurePanel = workspace.locator('section.panel').filter({
        has: page.getByRole('heading', { name: 'New feature suggestions', exact: true }),
      });
      assert.equal(await featurePanel.locator('.metric-number').innerText(), '—');
      assert.equal(await featurePanel.getByText(reason, { exact: true }).isVisible(), true);
      assert.equal(await featurePanel.locator('details').count(), 0);
      const disclosure = workspace.locator('details').first();
      await disclosure.locator('summary').press('Enter');
      await disclosure.getByText(stageReason, { exact: true }).first().waitFor();
      await disclosure.getByText('1 attempts · cached', { exact: true }).waitFor();
      await disclosure.getByText('2 attempts', { exact: true }).waitFor();
      assert.deepEqual(await disclosure.locator('.analysis-run-status').allTextContents(), [
        'completed',
        'running',
        'unavailable',
        'failed',
      ]);
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      for (const width of [1440, 1280, 1001, 800]) {
        await page.setViewportSize({ width, height: 1000 });
        for (const theme of ['dark', 'light']) {
          if ((await page.locator('html').getAttribute('data-theme')) !== theme)
            await page
              .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
              .click();
          for (const large of [false, true]) {
            const text = page.getByRole('button', { name: 'Larger text', exact: true });
            if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
            await capturedDetails(
              models,
              saved.details,
              width === 1440 || (width === 1280 && !large) ? 3 : 1,
            );
            await analysisRunLayout(page);
            await contrast(page, `Analysis run-${width}-${theme}-${large ? 'larger' : 'standard'}`);
            await layout(
              page,
              `analysis-run-long-evidence-${width}-${theme}-${large ? 'larger' : 'standard'}`,
            );
          }
        }
      }
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
      );
    } finally {
      await close();
    }
  });
  await test('Analysis run busy controls retain captured identity and explicit cancellation', async () => {
    for (const [status, action, label] of [
      ['running', 'pause', 'Pause'],
      ['paused', 'cancel', 'Cancel run'],
    ]) {
      const { page, close } = await pageFor({ runStatus: status });
      try {
        await nav(page, 'Analysis');
        await page.getByRole('button', { name: 'View run', exact: true }).click();
        const identity = await page.evaluate(() => {
          window.fixture.hold = '/api/projects/current/analysis/run/control';
          return window.fixture.state.run.identity;
        });
        await page.getByRole('button', { name: label, exact: true }).click();
        await page.waitForFunction(() => window.fixture.requests.some((r) => r.body?.action));
        const heading = page.locator('.analysis-run .page-heading--intro');
        for (const control of await heading.getByRole('button').all())
          if ((await control.innerText()) !== 'Files')
            assert.equal(await control.isDisabled(), true);
        const request = await page.evaluate(() =>
          window.fixture.requests.find((r) => r.body?.action),
        );
        assert.deepEqual(request.body, { identity, action });
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
        await idle(page);
        await heading
          .getByText(action === 'pause' ? 'paused' : 'canceled', { exact: true })
          .waitFor();
        assert.equal(await page.getByRole('dialog').count(), 0);
        assert.equal(
          await page.evaluate(() =>
            window.fixture.requests.some(
              (r) => r.method === 'POST' && r.path.endsWith('/analysis/run'),
            ),
          ),
          false,
        );
      } finally {
        try {
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
        } finally {
          await close();
        }
      }
    }
  });
  await test('An absent Analysis run keeps recovery reachable after a local refresh', async () => {
    const { page, close } = await pageFor({ runStatus: 'running' });
    try {
      await nav(page, 'Analysis');
      await page.getByRole('button', { name: 'View run', exact: true }).click();
      await page.evaluate(() => {
        window.fixture.state.run = null;
      });
      await page.getByRole('heading', { name: 'No analysis run yet', exact: true }).waitFor();
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      assert.equal(await page.locator('.analysis-run').getByRole('progressbar').count(), 0);
      for (const width of [1440, 800]) {
        await page.setViewportSize({ width, height: 1000 });
        for (const theme of ['dark', 'light']) {
          if ((await page.locator('html').getAttribute('data-theme')) !== theme)
            await page
              .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
              .click();
          for (const large of [false, true]) {
            const text = page.getByRole('button', { name: 'Larger text', exact: true });
            if ((await text.getAttribute('aria-pressed')) !== String(large)) await text.click();
            await analysisRunLayout(page);
            assert.equal(
              await page
                .locator('.analysis-run')
                .getByRole('button', { name: 'Prepare analysis', exact: true })
                .isVisible(),
              true,
            );
            await layout(
              page,
              `analysis-run-absent-${width}-${theme}-${large ? 'larger' : 'standard'}`,
            );
          }
        }
      }
      await page
        .locator('.analysis-run')
        .getByRole('button', { name: 'Prepare analysis', exact: true })
        .click();
      await page.getByRole('heading', { name: 'Analysis', exact: true }).waitFor();
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
        'Recovery opens setup without preparing or admitting analysis',
      );
    } finally {
      await close();
    }
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
    await openSummaryDiagrams(page);
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
  await test('Findings lists share Summary hierarchy and retain filters and local selection', async () => {
    for (const name of ['Bugs', 'Performance', 'Security']) {
      const { page, close } = await pageFor();
      try {
        await idle(page);
        const referenceIntro = await introductionTreatment(page.locator('.summary-hero'));
        const referencePanel = await panelTreatment(
          page.locator('.summary-details > .panel').first(),
        );
        await page.evaluate(() => {
          const state = window.fixture.state;
          const path = `internal/${'very-long-directory-'.repeat(12)}/worker.go`;
          state.finding.title += ` ${'complete-long-finding-title-'.repeat(12)}`;
          state.finding.location.path = path;
          state.performance.path = path;
          state.performance.findings[0].title += ` ${'complete-long-performance-title-'.repeat(12)}`;
          state.security.path = path;
          state.security.findings[0].source_anchor.path = path;
          state.security.findings[0].title += ` ${'complete-long-security-title-'.repeat(12)}`;
        });
        if (name === 'Security') await openSource(page);
        await nav(page, name);
        await page.locator('.result-row').first().waitFor();
        assert.deepEqual(
          await introductionTreatment(page.locator('.page-heading--intro')),
          referenceIntro,
        );
        assert.deepEqual(await panelTreatment(page.locator('.results-findings')), referencePanel);
        assert.equal(await page.locator('.result-row').count(), 1);
        await page.getByText('1 saved findings', { exact: true }).waitFor();
        const title = await page.locator('.result-row strong').innerText();
        const pathText = await page.locator('.result-row .path').innerText();
        assert.ok(pathText.includes('very-long-directory-'.repeat(12)));
        assert.ok(
          (await page.locator('.result-row small').innerText()).includes(
            { Bugs: 'AI analysis', Performance: 'Performance hypothesis', Security: 'rules' }[name],
          ),
          'Rows retain category-specific provenance and evidence kind',
        );
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page
              .getByRole('button', { name: 'Switch to light appearance', exact: true })
              .click();
          for (const larger of [false, true]) {
            const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
            if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
              await textSize.click();
            for (const width of [1440, 1280, 1001, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await resultsListLayout(page);
              assert.equal(await page.locator('.result-row strong').innerText(), title);
              assert.equal(await page.locator('.result-row .path').innerText(), pathText);
              if (name === 'Security') {
                for (const label of ['Scan source', 'AI Security review'])
                  assert.equal(
                    await page.getByRole('button', { name: label, exact: true }).isEnabled(),
                    true,
                  );
              }
              await layout(
                page,
                `${name.toLowerCase()}-list-long-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
              );
            }
            await contrast(page, `${name} findings ${theme} ${larger ? 'larger' : 'standard'}`);
          }
        }
        const filter = page.getByRole('textbox', { name: 'Filter findings', exact: true });
        await filter.fill('no-such-finding');
        await page.getByRole('heading', { name: 'No matching findings', exact: true }).waitFor();
        await resultsListLayout(page);
        await layout(page, `${name.toLowerCase()}-list-filtered-empty-800-light-larger`);
        await filter.fill('worker.go');
        await page
          .getByLabel('Finding severity', { exact: true })
          .selectOption({ label: name === 'Bugs' ? 'high' : 'medium' });
        await filter.press('Tab');
        assert.equal(
          await page
            .getByLabel('Finding severity', { exact: true })
            .evaluate((element) => element === document.activeElement),
          true,
        );
        await page.getByLabel('Finding severity', { exact: true }).press('Tab');
        assert.equal(
          await page
            .locator('.result-row')
            .evaluate((element) => element === document.activeElement),
          true,
        );
        await page.locator('.result-row').press('Enter');
        await page.getByRole('button', { name: 'All findings', exact: true }).click();
        assert.equal(await filter.inputValue(), 'worker.go');
        assert.equal(
          await page.getByLabel('Finding severity', { exact: true }).inputValue(),
          name === 'Bugs' ? 'high' : 'medium',
        );
        if (name === 'Bugs') {
          await page.getByRole('button', { name: 'Verified scan', exact: true }).click();
          await page.getByRole('button', { name: 'Run scan', exact: true }).waitFor();
          await nav(page, name);
        }
        await page.getByRole('button', { name: 'Analyze project', exact: true }).click();
        await page.getByRole('heading', { name: 'Analysis', exact: true }).waitFor();
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
          'Filters, selection, back-to-list and tool navigation remain passive',
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        assert.equal(await page.getByRole('dialog').count(), 0);
      } finally {
        await close();
      }
    }
  });
  await test('Selected findings retain complete evidence, Summary treatment and passive detail controls', async () => {
    for (const variant of [
      'bugs-semantic',
      'performance-typed',
      'security-typed',
      'performance-semantic',
      'security-semantic',
    ]) {
      const category = variant.split('-')[0];
      const name = category[0].toUpperCase() + category.slice(1);
      const semantic = variant.endsWith('-semantic');
      const { page, close } = await pageFor({ findingDetail: variant });
      try {
        await idle(page);
        const referenceIntro = await introductionTreatment(page.locator('.summary-hero'));
        const referencePanel = await panelTreatment(
          page.locator('.summary-details > .panel').first(),
        );
        const expected = await page.evaluate(
          ({ category, semantic }) => {
            const state = window.fixture.state;
            const row = semantic ? state.finding : state[category].findings[0];
            const text = semantic
              ? [
                  ['Finding', row.message],
                  ['Evidence', row.evidence],
                ]
              : category === 'performance'
                ? [
                    ['Observed pattern', row.observed_pattern],
                    ['Workload', row.workload_conditions],
                    ['Recommendation', row.recommendation],
                    ['Tradeoff', row.tradeoff],
                    ['Verification', row.verification_plan],
                  ]
                : [
                    ['Observed condition', row.observed_condition],
                    ['Evidence', row.evidence_kind],
                    ['Preconditions & unknowns', row.preconditions_or_unknowns],
                    ['Remediation', row.remediation],
                    ['Verification', row.verification_idea],
                    ['Rule', row.rule],
                    ['CWE', row.cwe],
                    ['Reference', row.reference],
                  ];
            return {
              title: row.title,
              confidence: row.confidence,
              source: semantic
                ? 'AI analysis'
                : category === 'performance'
                  ? 'Performance hypothesis'
                  : state.security.source,
              path: state.files[0].path,
              symbol: semantic
                ? row.location.symbol
                : category === 'performance'
                  ? row.symbol
                  : row.source_anchor.symbol,
              text,
              insight: row.engineering_insight,
              task: row.task_spec,
            };
          },
          { category, semantic },
        );
        await nav(page, name);
        const filter = page.getByRole('textbox', { name: 'Filter findings', exact: true });
        await filter.fill('process.go');
        await page
          .getByLabel('Finding severity', { exact: true })
          .selectOption(semantic ? 'high' : 'medium');
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        await page.locator('.result-row').press('Enter');
        const workspace = page.locator('.results-detail');
        const panel = (title) =>
          workspace
            .locator('.panel')
            .filter({ has: page.getByRole('heading', { name: title, exact: true }) });
        assert.deepEqual(
          await introductionTreatment(workspace.locator('.page-heading--intro')),
          referenceIntro,
        );
        assert.deepEqual(await panelTreatment(panel('Source')), referencePanel);
        assert.equal(await workspace.locator('h1').innerText(), expected.title);
        if (!['suggested', 'ai_suggestion'].includes(expected.confidence))
          assert.equal(
            await workspace.locator('.page-heading .badge').last().innerText(),
            expected.confidence.replaceAll('_', ' '),
          );
        for (const [title, content] of expected.text)
          assert.equal(
            await panel(title)
              .locator('.prose')
              .evaluate((element) => element.textContent),
            content.replace(/\n\s*\n/g, ''),
          );
        const metadata = panel('Source').locator('.key-values dd');
        assert.equal(await metadata.nth(0).innerText(), expected.path);
        assert.equal(await metadata.nth(1).innerText(), expected.symbol);
        assert.equal(await metadata.nth(2).innerText(), '5');
        assert.equal(await metadata.nth(3).innerText(), expected.source);
        assert.equal(
          await panel('Engineering insight')
            .locator('.prose')
            .first()
            .evaluate((element) => element.textContent),
          expected.insight.mechanism.replace(/\n\s*\n/g, ''),
        );
        await workspace.locator('summary').filter({ hasText: 'Why & tradeoffs' }).click();
        const insightText = await panel('Engineering insight').textContent();
        for (const value of Object.values(expected.insight))
          for (const paragraph of value.split(/\n\s*\n/))
            assert.ok(insightText.includes(paragraph));
        if (semantic) {
          assert.equal(
            await panel('Acceptance criteria')
              .locator(':scope > .panel-body > ul > li')
              .textContent(),
            expected.task.acceptance_criteria[0],
          );
          await workspace.locator('summary').filter({ hasText: 'Scope' }).click();
          assert.equal(
            await panel('Acceptance criteria').locator('.disclosure-body li').textContent(),
            expected.task.non_goals[0],
          );
          const guidance = panel('Acceptance criteria').locator('pre');
          assert.equal(await guidance.textContent(), expected.task.go_test_candidate.content);
          await guidance.focus();
          assert.equal(
            await guidance.evaluate((element) => element === document.activeElement),
            true,
          );
        }
        const complete = await workspace.textContent();
        for (const theme of ['dark', 'light']) {
          if (theme === 'light')
            await page
              .getByRole('button', { name: 'Switch to light appearance', exact: true })
              .click();
          for (const larger of [false, true]) {
            const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
            if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
              await textSize.click();
            for (const width of [1440, 1280, 1001, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await resultsDetailLayout(page);
              assert.equal(
                await workspace.textContent(),
                complete,
                'Reflow retains every evidence field',
              );
              for (const label of [
                'All findings',
                'Open source',
                'Prepare fix',
                'Prepare change',
                ...(semantic ? ['Dismiss', 'Mark fixed'] : []),
              ])
                assert.equal(
                  await workspace.getByRole('button', { name: label, exact: true }).isEnabled(),
                  true,
                );
              await layout(
                page,
                `${variant}-detail-long-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
              );
            }
            await contrast(page, `${variant} detail ${theme} ${larger ? 'larger' : 'standard'}`);
          }
        }
        assert.equal(await page.evaluate(() => window.detailExecuted), undefined);
        assert.equal(
          await workspace.locator('script, a, img').count(),
          0,
          'Model HTML and remote assets remain inert',
        );
        await workspace.getByRole('button', { name: 'All findings', exact: true }).click();
        assert.equal(await filter.inputValue(), 'process.go');
        assert.equal(
          await page.getByLabel('Finding severity', { exact: true }).inputValue(),
          semantic ? 'high' : 'medium',
        );
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
          'Selection, disclosures, appearance and back-to-list are passive',
        );
        // Reload retained evidence through the existing local read; preparation stays blocked.
        await page.evaluate(() => {
          window.fixture.state.finding.freshness = 'stale';
          window.fixture.state.performance.status = 'stale';
          window.fixture.state.security.status = 'stale';
        });
        await page.getByRole('button', { name: 'Refresh', exact: true }).click();
        await idle(page);
        await page.locator('.result-row').click();
        await workspace.locator('.results-state').getByText('stale', { exact: true }).waitFor();
        for (const label of ['Prepare fix', 'Prepare change'])
          assert.equal(
            await workspace.getByRole('button', { name: label, exact: true }).isDisabled(),
            true,
          );
        assert.equal(
          await workspace.getByRole('button', { name: 'Open source', exact: true }).isEnabled(),
          true,
        );
        await resultsDetailLayout(page);
        await layout(page, `${variant}-detail-stale-800-light-larger`);
        // Preserve optional-field fallbacks, including file-level findings without declaration actions.
        await workspace.getByRole('button', { name: 'All findings', exact: true }).click();
        await page.evaluate(() => {
          const state = window.fixture.state;
          state.finding.freshness = 'fresh';
          state.finding.location.symbol = '';
          delete state.finding.task_spec;
          delete state.finding.engineering_insight;
          state.performance.status = 'success';
          state.performance.findings[0].symbol = '';
          delete state.performance.findings[0].engineering_insight;
          state.security.status = 'success';
          state.security.findings[0].source_anchor.symbol = '';
          delete state.security.findings[0].engineering_insight;
          delete state.security.findings[0].cwe;
          delete state.security.findings[0].reference;
        });
        await page.getByRole('button', { name: 'Refresh', exact: true }).click();
        await idle(page);
        await page.locator('.result-row').click();
        assert.equal(
          await workspace
            .getByRole('heading', { name: 'Engineering insight', exact: true })
            .count(),
          0,
        );
        assert.equal(
          await workspace
            .getByRole('heading', { name: 'Acceptance criteria', exact: true })
            .count(),
          0,
        );
        assert.equal(
          await workspace.getByRole('button', { name: 'Prepare change', exact: true }).count(),
          0,
        );
        await workspace.getByText('File-level finding', { exact: true }).waitFor();
        if (!semantic && category === 'security')
          for (const title of ['CWE', 'Reference']) assert.equal(await panel(title).count(), 0);
        await resultsDetailLayout(page);
        await layout(page, `${variant}-detail-optional-800-light-larger`);
        await workspace.getByRole('button', { name: 'All findings', exact: true }).click();
        await page.evaluate((symbol) => {
          const state = window.fixture.state;
          state.finding.location.symbol = symbol;
          state.performance.findings[0].symbol = symbol;
          state.security.findings[0].source_anchor.symbol = symbol;
        }, expected.symbol);
        await page.getByRole('button', { name: 'Refresh', exact: true }).click();
        await idle(page);
        const previousReads = await page.evaluate(
          () => window.fixture.requests.filter((r) => r.path.endsWith('/analysis/results')).length,
        );
        await page.evaluate(() => {
          window.fixture.hold = '/api/projects/current/analysis/results';
        });
        await page.getByRole('button', { name: 'Refresh', exact: true }).click();
        await page.waitForFunction(
          (count) =>
            window.fixture.requests.filter((r) => r.path.endsWith('/analysis/results')).length >
            count,
          previousReads,
        );
        await page.locator('.busy-strip').waitFor();
        await page.locator('.result-row').click();
        for (const label of [
          'Open source',
          'Prepare fix',
          'Prepare change',
          ...(semantic ? ['Dismiss', 'Mark fixed'] : []),
        ])
          assert.equal(
            await workspace.getByRole('button', { name: label, exact: true }).isDisabled(),
            true,
          );
        assert.equal(
          await workspace.getByRole('button', { name: 'All findings', exact: true }).isEnabled(),
          true,
        );
        await resultsDetailLayout(page);
        await layout(page, `${variant}-detail-busy-800-light-larger`);
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
          'Reloading and inspecting stale, optional or busy evidence does not generate or mutate',
        );
      } finally {
        try {
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
        } finally {
          await close();
        }
      }
    }
  });
  await test('Semantic finding triage remains explicit with retained detail and fixed-state restrictions', async () => {
    const { page, close } = await pageFor();
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').click();
      const initialWrites = await page.evaluate(
        () => window.fixture.requests.filter((r) => r.method !== 'GET').length,
      );
      for (const [label, status] of [
        ['Dismiss', 'dismissed'],
        ['Reopen', 'open'],
        ['Mark fixed', 'fixed'],
      ]) {
        await page.getByRole('button', { name: label, exact: true }).click();
        await idle(page);
        assert.equal(
          await page.locator('.results-detail-source .key-values dd').last().innerText(),
          status,
        );
      }
      assert.equal(
        await page.getByRole('button', { name: 'Mark fixed', exact: true }).isDisabled(),
        true,
      );
      const writes = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      assert.deepEqual(
        writes.slice(initialWrites).map((r) => [r.method, r.path, r.body.status]),
        [
          ['PATCH', '/api/projects/current/findings/finding-1', 'dismissed'],
          ['PATCH', '/api/projects/current/findings/finding-1', 'open'],
          ['PATCH', '/api/projects/current/findings/finding-1', 'fixed'],
        ],
      );
    } finally {
      await close();
    }
  });
  await test('Finding source inspection and declaration handoff remain distinct from generation', async () => {
    for (const action of ['Open source', 'Prepare change']) {
      const { page, close } = await pageFor();
      try {
        await nav(page, 'Bugs');
        await page.locator('.result-row').click();
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        await page.getByRole('button', { name: action, exact: true }).click();
        if (action === 'Open source')
          await page.getByLabel('Read-only source', { exact: true }).waitFor();
        else await page.getByLabel('Change request', { exact: true }).waitFor();
        await idle(page);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
        );
        assert.equal(await page.getByRole('dialog').count(), 0);
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      } finally {
        await close();
      }
    }
  });
  await test('Each findings category keeps unavailable, read-failed, retained and partial evidence truthful', async () => {
    for (const name of ['Bugs', 'Performance', 'Security']) {
      const category = name.toLowerCase();
      for (const scenario of ['unavailable', 'read-failed', 'retained-stale', 'partial']) {
        const { page, close } = await pageFor();
        try {
          await idle(page);
          const before = await page.evaluate(() =>
            window.fixture.requests.filter((r) => r.method !== 'GET'),
          );
          await page.evaluate(
            ({ category, scenario }) => {
              const state = window.fixture.state;
              if (scenario === 'read-failed') {
                window.fixture.failures['/api/projects/current/analysis/results'] = 503;
                return;
              }
              const progress = {
                ...state.run.sections.find((section) => section.category === category),
                status: scenario === 'retained-stale' ? 'partial' : scenario,
                finding_count: scenario === 'unavailable' ? null : 1,
              };
              const result = {
                progress,
                saved_finding_count: scenario === 'unavailable' ? null : 1,
                semantic:
                  category === 'bugs'
                    ? [
                        {
                          ...state.finding,
                          freshness: scenario === 'retained-stale' ? 'stale' : 'fresh',
                        },
                      ]
                    : [],
                performance:
                  category === 'performance'
                    ? [
                        {
                          ...state.performance,
                          status:
                            scenario === 'retained-stale'
                              ? 'stale'
                              : scenario === 'partial'
                                ? 'partial'
                                : 'success',
                        },
                      ]
                    : [],
                security:
                  category === 'security'
                    ? [
                        {
                          ...state.security,
                          status:
                            scenario === 'retained-stale'
                              ? 'stale'
                              : scenario === 'partial'
                                ? 'partial'
                                : 'success',
                        },
                      ]
                    : [],
                retained_files:
                  scenario === 'retained-stale'
                    ? [
                        {
                          path: state.files[0].path,
                          content_hash: state.files[0].content_hash,
                          language: 'go',
                        },
                      ]
                    : [],
                unclassified: [
                  {
                    ...state.finding,
                    id: 'historical',
                    category: '',
                    title: `Historical unclassified ${'long-title-'.repeat(20)}`,
                    message: 'Historical evidence remains separate from category counts.',
                  },
                ],
              };
              if (scenario === 'unavailable') {
                result.semantic = [];
                result.performance = [];
                result.security = [];
              }
              // Typed report failures remain visible even when a category also has saved rows.
              if (category !== 'bugs') {
                const path = `internal/${'long-report-directory-'.repeat(16)}/failed.go`;
                if (category === 'performance')
                  result.performance.push({
                    ...state.performance,
                    path,
                    status: 'failed',
                    findings: [],
                    warning: `Report unavailable: ${'complete diagnostic '.repeat(30)}`,
                  });
                else
                  result.security.push({
                    ...state.security,
                    path,
                    status: 'unavailable',
                    findings: [],
                    reason: `Report unavailable: ${'complete diagnostic '.repeat(30)}`,
                  });
              }
              state.results[category] = result;
            },
            { category, scenario },
          );
          await nav(page, name);
          await idle(page);
          const workspace = page.locator('.results-page');
          await workspace.getByRole('heading', { name, exact: true }).waitFor();
          if (scenario === 'read-failed') {
            await page
              .getByRole('alert')
              .filter({ hasText: `${category}:` })
              .getByText('Fixture rejection', { exact: false })
              .waitFor();
            assert.equal(
              await workspace.getByText(/saved findings|Count unavailable/).count(),
              0,
              'A failed read does not fabricate saved counts',
            );
            if (category !== 'bugs')
              await workspace
                .getByRole('heading', { name: 'Results not loaded', exact: true })
                .waitFor();
            else
              assert.equal(
                await workspace.locator('.result-row').count(),
                1,
                'Saved semantic fallback remains accessible',
              );
          } else {
            await workspace
              .getByText(scenario === 'unavailable' ? 'Count unavailable' : '1 saved findings', {
                exact: true,
              })
              .waitFor();
            if (scenario === 'unavailable')
              await workspace
                .getByRole('heading', { name: 'No saved findings', exact: true })
                .waitFor();
            else assert.equal(await workspace.locator('.result-row').count(), 1);
            const status = scenario === 'retained-stale' ? 'partial' : scenario;
            assert.equal(
              await workspace.locator('.page-heading--intro .results-state').innerText(),
              status,
            );
            if (scenario === 'retained-stale') {
              await workspace
                .getByRole('status')
                .getByText('Includes retained results from 1 files outside this run.', {
                  exact: true,
                })
                .waitFor();
              await workspace
                .locator('.result-row .results-state')
                .getByText('stale', { exact: true })
                .waitFor();
            }
            if (category !== 'bugs') {
              const reports = workspace.getByRole('region', {
                name: 'Report summaries',
                exact: true,
              });
              await reports.getByText(/Report unavailable: complete diagnostic/).waitFor();
              await reports
                .locator('.results-state')
                .getByText(category === 'performance' ? 'failed' : 'unavailable', { exact: true })
                .waitFor();
            }
            const beforeDisclosure = await page.evaluate(() => window.fixture.requests.length);
            await workspace.locator('summary').press('Enter');
            await workspace
              .getByText('Historical evidence remains separate from category counts.', {
                exact: true,
              })
              .waitFor();
            assert.equal(
              await page.evaluate(() => window.fixture.requests.length),
              beforeDisclosure,
              'Opening unclassified evidence is local',
            );
          }
          await resultsListLayout(page);
          await layout(page, `${category}-list-${scenario}-1440-dark-standard`);
          await page.setViewportSize({ width: 800, height: 1000 });
          await page
            .getByRole('button', { name: 'Switch to light appearance', exact: true })
            .click();
          await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          await resultsListLayout(page);
          await layout(page, `${category}-list-${scenario}-800-light-larger`);
          assert.deepEqual(
            await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
            before,
            'Reading results and disclosures does not admit scans, providers, preparation or triage',
          );
          assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        } finally {
          await close();
        }
      }
    }
  });
  await test('Selected-source Security actions retain busy and stale restrictions', async () => {
    const { page, close } = await pageFor();
    try {
      await idle(page);
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      await openSource(page);
      await nav(page, 'Security');
      const actions = ['Scan source', 'AI Security review'].map((name) =>
        page.getByRole('button', { name, exact: true }),
      );
      for (const action of actions) assert.equal(await action.isEnabled(), true);
      const previousReads = await page.evaluate(
        () => window.fixture.requests.filter((r) => r.path.endsWith('/analysis/results')).length,
      );
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/analysis/results';
      });
      await page.getByRole('button', { name: 'Refresh', exact: true }).click();
      await page.waitForFunction(
        (count) =>
          window.fixture.requests.filter((r) => r.path.endsWith('/analysis/results')).length >
          count,
        previousReads,
      );
      await page.locator('.busy-strip').waitFor();
      for (const action of actions) assert.equal(await action.isDisabled(), true);
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await idle(page);
      for (const action of actions) assert.equal(await action.isEnabled(), true);
      await page.evaluate(() => {
        window.fixture.state.changed = true;
      });
      await nav(page, 'Source');
      await page.locator('.notice').filter({ hasText: 'File evidence is outdated.' }).waitFor();
      await nav(page, 'Security');
      for (const action of actions) assert.equal(await action.isDisabled(), true);
      await page.setViewportSize({ width: 800, height: 1000 });
      await page.getByRole('button', { name: 'Larger text', exact: true }).click();
      await resultsListLayout(page);
      await layout(page, 'security-list-selected-source-stale-800-dark-larger');
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
        'Selected-source presentation does not start a scan or AI review',
      );
      assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
    } finally {
      try {
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
      } finally {
        await close();
      }
    }
  });
  await test('Findings pagination and severity filtering remain available in every category', async () => {
    for (const name of ['Bugs', 'Performance', 'Security']) {
      const { page, close } = await pageFor();
      try {
        await page.evaluate((category) => {
          const state = window.fixture.state;
          const items = Array.from({ length: 101 }, (_, i) => ({
            id: `page-${i}`,
            title: `Saved finding ${i}`,
            level: i % 2 ? 'low' : 'high',
          }));
          state.results[category] = {
            saved_finding_count: 101,
            semantic:
              category === 'bugs'
                ? items.map((item) => ({ ...state.finding, ...item, severity: item.level }))
                : [],
            performance:
              category === 'performance'
                ? [
                    {
                      ...state.performance,
                      findings: items.map((item) => ({
                        ...state.performance.findings[0],
                        ...item,
                        potential_impact: item.level,
                      })),
                    },
                  ]
                : [],
            security:
              category === 'security'
                ? [
                    {
                      ...state.security,
                      findings: items.map((item) => ({
                        ...state.security.findings[0],
                        ...item,
                        severity: item.level,
                      })),
                    },
                  ]
                : [],
          };
        }, name.toLowerCase());
        await nav(page, name);
        await page.getByText('101 saved findings', { exact: true }).waitFor();
        assert.equal(await page.locator('.result-row').count(), 100);
        const before = await page.evaluate(() => window.fixture.requests.length);
        await page.getByRole('button', { name: 'Show more', exact: true }).click();
        assert.equal(await page.locator('.result-row').count(), 101);
        await page.getByLabel('Finding severity', { exact: true }).selectOption('low');
        assert.equal(await page.locator('.result-row').count(), 50);
        await page.getByLabel('Finding severity', { exact: true }).selectOption('');
        await page
          .getByRole('textbox', { name: 'Filter findings', exact: true })
          .fill('Saved finding');
        assert.equal(
          await page.locator('.result-row').count(),
          100,
          'Query changes reset pagination',
        );
        assert.equal(await page.evaluate(() => window.fixture.requests.length), before);
        await page.setViewportSize({ width: 800, height: 1000 });
        await resultsListLayout(page);
        await layout(page, `${name.toLowerCase()}-list-pagination-800-dark-standard`);
      } finally {
        await close();
      }
    }
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
  await test('Chat scope, complete conversation and history reuse Summary without resetting inputs', async () => {
    const title = `Improve recovery ${'LongTaskTitle'.repeat(10)}`;
    const paths = Array.from(
      { length: 8 },
      (_, i) => `internal/${'long_directory_'.repeat(12)}/worker_${i}.go`,
    );
    const message = `Preserve all requested behavior. ${'CompleteRequest'.repeat(40)}\n\nKeep the public API unchanged.`;
    const assistant = `Captured scope remains deliberate. ${'CompleteAssistantResponse'.repeat(35)}\n\n\`\`\`go\n// ${'long_code_'.repeat(70)}\n\`\`\`\n\nFinal explanation stays available.`;
    const { page, close } = await pageFor({ changeAssistantMessage: assistant });
    try {
      await idle(page);
      const referenceIntro = await introductionTreatment(page.locator('.summary-hero'));
      const referencePanel = await panelTreatment(page.locator('.summary-details .panel').first());
      await nav(page, 'Chat');
      await idle(page);
      const conversation = page.getByRole('region', { name: 'Change conversation', exact: true });
      assert.deepEqual(
        await introductionTreatment(page.locator('.chat-page .page-heading')),
        referenceIntro,
      );
      assert.deepEqual(
        await panelTreatment(conversation.locator('.panel').first()),
        referencePanel,
      );
      await page.getByText('Local history', { exact: true }).click();
      await page.getByText('No saved conversations.', { exact: true }).waitFor();
      const prepare = conversation.getByRole('button', { name: 'Prepare change', exact: true });
      assert.equal(await prepare.isDisabled(), true);
      await page.getByLabel('Task title', { exact: true }).fill(title);
      await page.getByLabel('Files to change', { exact: true }).fill(paths.join('\n'));
      await page.getByLabel('Change request', { exact: true }).fill(message);
      await page.getByLabel('Run project tests after generation', { exact: true }).uncheck();
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      const render = async (state, widths) => {
        for (const theme of ['dark', 'light']) {
          if ((await page.locator('html').getAttribute('data-theme')) !== theme)
            await page
              .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
              .click();
          for (const larger of [false, true]) {
            const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
            if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
              await textSize.click();
            for (const width of widths) {
              await page.setViewportSize({ width, height: 1000 });
              await chatConversationLayout(page);
              assert.equal(
                await page.getByLabel('Change request', { exact: true }).inputValue(),
                state === 'new' ? message : 'Keep this local follow-up while resizing.',
              );
              assert.equal(
                await page
                  .getByLabel('Run project tests after generation', { exact: true })
                  .isChecked(),
                false,
              );
              if (state === 'new') {
                assert.equal(
                  await page.getByLabel('Task title', { exact: true }).inputValue(),
                  title,
                );
                assert.equal(
                  await page.getByLabel('Files to change', { exact: true }).inputValue(),
                  paths.join('\n'),
                );
              } else {
                for (const path of paths)
                  assert.ok((await conversation.textContent()).includes(path));
                const messages = conversation.locator('.chat-message');
                assert.deepEqual(
                  await messages.nth(0).locator('.prose p').allTextContents(),
                  message.split('\n\n'),
                );
                assert.deepEqual(await messages.nth(1).locator('.prose p').allTextContents(), [
                  assistant.split('\n\n')[0],
                  'Final explanation stays available.',
                ]);
                assert.equal(
                  await messages.nth(1).locator('.prose pre').textContent(),
                  `// ${'long_code_'.repeat(70)}\n`,
                );
                assert.ok(
                  (await conversation.locator('.chat-history').textContent()).includes(title),
                );
              }
              assert.equal(await prepare.isEnabled(), true);
              await layout(
                page,
                `chat-conversation-${state}-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
              );
            }
          }
        }
      };
      await render('new', [1440, 800]);
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
      );
      await prepare.click();
      await idle(page);
      const creation = await page.evaluate(() =>
        window.fixture.requests.find(
          (r) => r.path === '/api/projects/current/changes' && r.method === 'POST',
        ),
      );
      assert.equal(creation.body.title, title);
      assert.deepEqual(creation.body.paths, paths);
      assert.equal(
        await page.getByLabel('Change request', { exact: true }).inputValue(),
        '',
        'Existing identity effect clears the first submitted request',
      );
      await page.getByRole('button', { name: 'Review this diff', exact: true }).click();
      await idle(page);
      await page.getByRole('button', { name: 'New conversation', exact: true }).click();
      await page.getByRole('button', { name: 'Refresh history', exact: true }).click();
      await conversation
        .locator('.list-row')
        .getByRole('button', { name: 'Resume', exact: true })
        .waitFor();
      const generated = await page.evaluate(
        () => window.fixture.requests.filter((r) => r.path.endsWith('/messages')).length,
      );
      await conversation.getByRole('button', { name: 'Resume', exact: true }).click();
      await idle(page);
      assert.equal(
        await page.getByRole('button', { name: 'Approve and apply', exact: true }).isDisabled(),
        true,
      );
      assert.equal(
        await page.getByRole('button', { name: 'Review this diff', exact: true }).isDisabled(),
        true,
      );
      assert.equal(
        await page.getByLabel('Run project tests after generation', { exact: true }).isChecked(),
        true,
        'Restore retains the existing tests default',
      );
      await page.getByLabel('Run project tests after generation', { exact: true }).uncheck();
      await page
        .getByLabel('Change request', { exact: true })
        .fill('Keep this local follow-up while resizing.');
      const restored = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      await render('restored', [1440, 1280, 1001, 800]);
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        restored,
        'Restored conversation reflow and disclosures remain passive',
      );
      assert.equal(
        await page.evaluate(
          () => window.fixture.requests.filter((r) => r.path.endsWith('/messages')).length,
        ),
        generated,
      );
      assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
    } finally {
      await close();
    }
  });
  await test('Chat unavailable history, failed requests and stale conversations retain recovery and restrictions', async () => {
    for (const state of ['history-unavailable', 'failed', 'stale']) {
      const { page, close } = await pageFor({
        changeHistoryReadFail: state === 'history-unavailable',
      });
      try {
        await nav(page, 'Chat');
        await idle(page);
        await page.getByText('Local history', { exact: true }).click();
        if (state === 'history-unavailable') {
          await page.getByRole('heading', { name: 'History unavailable', exact: true }).waitFor();
          assert.equal(
            await page.getByRole('button', { name: 'Refresh history', exact: true }).isEnabled(),
            true,
          );
        } else {
          await page
            .getByLabel('Add an existing file', { exact: true })
            .selectOption('internal/worker/process.go');
          await page
            .getByLabel('Change request', { exact: true })
            .fill('Preserve the captured scope.');
          await page.getByLabel('Run project tests after generation', { exact: true }).uncheck();
          if (state === 'failed')
            await page.evaluate(() => {
              window.fixture.failures['/api/projects/current/changes/change-1/messages'] = 503;
            });
          await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
          await idle(page);
          if (state === 'failed') {
            await page.getByText('Fixture rejection', { exact: true }).waitFor();
            assert.equal(await page.locator('.chat-message').count(), 0);
          } else {
            await page.getByRole('button', { name: 'New conversation', exact: true }).click();
            await page.evaluate(() => {
              window.fixture.state.changed = true;
            });
            await page.getByRole('button', { name: 'Refresh history', exact: true }).click();
            await page.getByRole('button', { name: 'Resume', exact: true }).click();
            await idle(page);
            await page
              .getByText('Source or guidance changed. Start a new conversation.', { exact: true })
              .waitFor();
          }
          assert.equal(
            await page.getByLabel('Change request', { exact: true }).isDisabled(),
            state === 'stale',
          );
          assert.equal(
            await page
              .getByLabel('Run project tests after generation', { exact: true })
              .isDisabled(),
            state === 'stale',
          );
          if (state === 'failed')
            await page.getByLabel('Change request', { exact: true }).fill('Retry explicitly.');
          assert.equal(
            await page.getByRole('button', { name: 'Prepare change', exact: true }).isDisabled(),
            state === 'stale',
          );
        }
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        for (const theme of ['dark', 'light']) {
          if ((await page.locator('html').getAttribute('data-theme')) !== theme)
            await page
              .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
              .click();
          for (const larger of [false, true]) {
            const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
            if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
              await textSize.click();
            for (const width of [1440, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await chatConversationLayout(page);
              await chatReviewLayout(page);
              if (state === 'stale') {
                const review = page.getByRole('region', { name: 'Proposal review', exact: true });
                for (const name of ['Check proposal', 'Review this diff', 'Approve and apply'])
                  assert.equal(
                    await review.getByRole('button', { name, exact: true }).isDisabled(),
                    true,
                  );
              }
              await layout(
                page,
                `chat-conversation-${state}-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
              );
            }
          }
        }
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
          'Inspecting conversation failures/history never dispatches work',
        );
      } finally {
        await close();
      }
    }
  });
  await test('Chat multi-file review keeps complete diffs, checks and explicit decisions across layouts', async () => {
    const paths = [
      `internal/${'long_directory_'.repeat(12)}/process.go`,
      `docs/${'long_directory_'.repeat(12)}/recovery.md`,
    ];
    const code = `// ${'complete_code_'.repeat(100)} end of diff`;
    const diagnostic = `Failure details ${'complete_diagnostic_'.repeat(100)} end of diagnostics`;
    for (const state of ['unreviewed', 'exhausted', 'missing']) {
      const { page, close } = await pageFor({
        changeProposalKind: 'performance',
        changeDiff: { lines: [{ kind: 'add', new_line: 1, text: code }] },
        changeCheckOutput: diagnostic,
        changeChecksFail: state === 'exhausted',
        changeMissingDiffs: state === 'missing',
      });
      try {
        await idle(page);
        const reference = await panelTreatment(page.locator('.summary-details .panel').first());
        await nav(page, 'Chat');
        await page.getByLabel('Files to change', { exact: true }).fill(paths.join('\n'));
        await page
          .getByLabel('Change request', { exact: true })
          .fill('Preserve complete proposal evidence.');
        await page.getByLabel('Run project tests after generation', { exact: true }).uncheck();
        await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
        await idle(page);
        const review = page.getByRole('region', { name: 'Proposal review', exact: true });
        if (state === 'missing') {
          await review.getByRole('heading', { name: 'No proposal yet', exact: true }).waitFor();
          assert.equal(await review.getByRole('button').count(), 0);
        } else {
          assert.deepEqual(await panelTreatment(review.locator('.panel').first()), reference);
          assert.equal(await review.locator('.diff').count(), paths.length);
          for (const path of paths) {
            const diff = review.getByLabel(`Read-only diff for ${path}`, { exact: true });
            assert.equal(await diff.locator('code').textContent(), code);
            assert.equal(await diff.locator('.code-header strong').textContent(), path);
          }
          await review
            .getByText('Performance unmeasured; tests do not establish a speedup.', { exact: true })
            .waitFor();
          const beforeDisclosures = await page.evaluate(() =>
            window.fixture.requests.filter((r) => r.method !== 'GET'),
          );
          await review.getByText('Diagnostics', { exact: true }).click();
          assert.equal(await review.locator('.chat-check pre').textContent(), diagnostic);
          await review.getByText('Provider context', { exact: true }).click();
          assert.deepEqual(
            await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
            beforeDisclosures,
            'Opening diagnostics and provider context is passive',
          );
          assert.equal(
            await review.getByRole('button', { name: 'Check proposal', exact: true }).isEnabled(),
            true,
          );
          assert.equal(
            await review
              .getByRole('button', { name: 'Review this diff', exact: true })
              .isDisabled(),
            state === 'exhausted',
          );
          assert.equal(
            await review
              .getByRole('button', { name: 'Approve and apply', exact: true })
              .isDisabled(),
            true,
          );
          if (state === 'exhausted') {
            assert.equal(
              await review
                .getByRole('button', { name: 'Repair failed checks', exact: true })
                .isDisabled(),
              true,
            );
            assert.equal(
              await page.evaluate(
                () => window.fixture.requests.filter((r) => r.body?.repair === true).length,
              ),
              3,
            );
          }
        }
        const render = async (label) => {
          const before = await page.evaluate(() =>
            window.fixture.requests.filter((r) => r.method !== 'GET'),
          );
          for (const theme of ['dark', 'light']) {
            if ((await page.locator('html').getAttribute('data-theme')) !== theme)
              await page
                .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
                .click();
            for (const larger of [false, true]) {
              const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
              if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
                await textSize.click();
              for (const width of [1440, 1280, 1001, 800]) {
                await page.setViewportSize({ width, height: 1000 });
                await chatReviewLayout(page);
                await layout(
                  page,
                  `chat-proposal-${label}-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
                );
              }
            }
          }
          assert.deepEqual(
            await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
            before,
            'Reflow and evidence inspection never checks, reviews or applies',
          );
          assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        };
        await render(state);
        if (state === 'unreviewed') {
          await review.getByRole('button', { name: 'Review this diff', exact: true }).click();
          await idle(page);
          await review.getByText('Revision reviewed.', { exact: true }).waitFor();
          assert.equal(
            await review
              .getByRole('button', { name: 'Review this diff', exact: true })
              .isDisabled(),
            true,
          );
          assert.equal(
            await review
              .getByRole('button', { name: 'Approve and apply', exact: true })
              .isEnabled(),
            true,
          );
          await render('reviewed');
          await page.evaluate(() => {
            window.fixture.hold = '/api/projects/current/changes/change-1/checks';
          });
          const checksBefore = await page.evaluate(
            () =>
              window.fixture.requests.filter((r) => r.path.endsWith('/changes/change-1/checks'))
                .length,
          );
          await review.getByRole('button', { name: 'Check proposal', exact: true }).click();
          await page.waitForFunction(
            (count) =>
              window.fixture.requests.filter((r) => r.path.endsWith('/changes/change-1/checks'))
                .length > count,
            checksBefore,
          );
          await page.locator('.busy-strip').waitFor();
          for (const name of ['Check proposal', 'Review this diff', 'Approve and apply'])
            assert.equal(
              await review.getByRole('button', { name, exact: true }).isDisabled(),
              true,
            );
          await render('busy');
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
          await idle(page);
          assert.equal(
            await review
              .getByRole('button', { name: 'Approve and apply', exact: true })
              .isDisabled(),
            true,
            'Rechecking clears earlier review',
          );
        }
      } finally {
        try {
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
        } finally {
          await close();
        }
      }
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
  await test('Canceled chat responses do not publish a late proposal or reset local follow-up', async () => {
    const { page, close } = await pageFor();
    try {
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
      for (const control of [
        page.getByLabel('Change request', { exact: true }),
        page.getByLabel('Run project tests after generation'),
        page.getByRole('button', { name: 'Prepare change', exact: true }),
        page.getByRole('button', { name: 'New conversation', exact: true }),
        page.getByRole('button', { name: 'Refresh history', exact: true }),
      ]) {
        if (await control.isVisible()) assert.equal(await control.isDisabled(), true);
      }
      await page.getByText('Local history', { exact: true }).click();
      assert.equal(
        await page.getByRole('button', { name: 'Refresh history', exact: true }).isDisabled(),
        true,
      );
      for (const width of [1440, 800]) {
        await page.setViewportSize({ width, height: 1000 });
        await chatConversationLayout(page);
        await layout(page, `chat-conversation-busy-${width}`);
      }
      await page
        .locator('.busy-strip')
        .getByRole('button', { name: 'Cancel', exact: true })
        .click();
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await idle(page);
      await page.getByText('Canceled. Refresh status before retrying.', { exact: true }).waitFor();
      assert.equal(
        await page.getByLabel('Read-only diff for internal/worker/process.go').count(),
        0,
      );
      assert.equal(await page.locator('.chat-message').count(), 0);
      await page
        .getByLabel('Change request', { exact: true })
        .fill('Retry only when explicitly requested.');
      for (const width of [1440, 800]) {
        await page.setViewportSize({ width, height: 1000 });
        await chatConversationLayout(page);
        assert.equal(
          await page.getByLabel('Change request', { exact: true }).inputValue(),
          'Retry only when explicitly requested.',
        );
        assert.equal(
          await page.getByRole('button', { name: 'Prepare change', exact: true }).isEnabled(),
          true,
        );
        await layout(page, `chat-conversation-canceled-${width}`);
      }
      assert.equal(
        await page.evaluate(
          () =>
            window.fixture.requests.filter(
              (r) => r.path.endsWith('/messages') && r.path.includes('/changes/'),
            ).length,
        ),
        1,
      );
    } finally {
      try {
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
      } finally {
        await close();
      }
    }
  });
  await test('Summary links feature counts to ideas and leaves findings in their workspaces', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    await idle(page);
    const card = page.locator('.metric-card[data-accent="features"]');
    assert.equal(await card.locator('.metric-number').innerText(), '1');
    assert.equal(await page.locator('.metric-card').count(), 6);
    assert.equal(await page.getByRole('heading', { name: 'Findings', exact: true }).count(), 0);
    assert.equal(await page.locator('.list-row').count(), 0);
    assert.equal(
      await page.getByRole('heading', { name: 'New feature suggestions', exact: true }).count(),
      0,
    );
    assert.equal(await page.getByText('Retry failed work', { exact: true }).count(), 0);
    assert.equal(await page.getByText('Estimated effort: medium', { exact: true }).count(), 0);
    assert.equal(
      await page.getByRole('button', { name: 'Discuss in chat', exact: true }).count(),
      0,
    );
    const requests = await page.evaluate(() => window.fixture.requests);
    assert.equal(
      requests.some((r) => r.path.endsWith('/features') && r.method === 'GET'),
      true,
    );
    assert.equal(
      requests.some((r) => r.method !== 'GET' && r.path !== '/api/projects/restore'),
      false,
    );
    await card.focus();
    await page.keyboard.press('Enter');
    await page.getByRole('heading', { name: 'Features', exact: true }).waitFor();
    await page.getByRole('heading', { name: 'Retry failed work', exact: true }).waitFor();
    await page.getByText('Estimated effort: medium', { exact: true }).waitFor();
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
  await test('Summary feature counts preserve unknown, empty, stale and failed states', async () => {
    for (const [options, count, status] of [
      [{}, '—', 'not generated'],
      [{ featuresReadFail: true }, '—', 'Unavailable'],
      [{ featuresStale: true }, '—', 'stale'],
      [{ featuresReady: true, featuresEmpty: true }, '0', 'ready'],
      [{ featuresReady: true }, '1', 'ready'],
      [{ featuresReady: true, featuresFail: true }, '1', 'failed'],
      [{ featuresReady: true, featuresStale: true }, '1', 'stale'],
      [{ featuresReady: true, featuresFail: true, featuresStale: true }, '1', 'failed'],
      [{ featuresReady: true, featuresEmpty: true, featuresFail: true }, '—', 'failed'],
      [
        {
          empty: true,
          featuresReady: true,
          featuresEmpty: true,
          featuresFail: true,
          featuresStale: true,
        },
        '—',
        'failed',
      ],
    ]) {
      const { page, close } = await pageFor(options);
      await idle(page);
      const card = page.locator('.metric-card[data-accent="features"]');
      assert.equal(await card.locator('.metric-number').innerText(), count);
      assert.equal(await card.getByRole('img').getAttribute('aria-label'), `Features: ${status}`);
      assert.equal(await page.getByText('Retry failed work', { exact: true }).count(), 0);
      assert.equal(await page.getByText(/Feature search failed/).count(), 0);
      assert.equal(await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(), 0);
      await page.setViewportSize({ width: 800, height: 900 });
      await page.getByRole('button', { name: 'Larger text' }).click();
      await layout(page, `summary-feature-count-${JSON.stringify(options)}`);
      await card.focus();
      await contrast(page, 'Summary feature counts');
      await page.getByRole('button', { name: 'Switch to light appearance' }).click();
      await contrast(page, 'Summary feature counts light');
      await card.click();
      await page.getByRole('heading', { name: 'Features', exact: true }).waitFor();
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
      assert.equal(
        await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(),
        options.featuresStale && options.featuresReady && !options.featuresEmpty ? 1 : 0,
      );
      await layout(page, `features-saved-state-${JSON.stringify(options)}`);
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
      assert.equal(
        await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(),
        options.featuresStale && options.featuresReady && !options.featuresEmpty ? 1 : 0,
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
    }
  });
  await test('Features reuse Summary treatment with complete ideas, metadata and passive filters', async () => {
    const { page, close } = await pageFor({
      featuresReady: true,
      featuresLongContent: true,
      featuresMixedTriage: true,
      modelNames: { analyze: 'feature-model-'.repeat(24) },
    });
    try {
      await idle(page);
      const referenceIntro = await introductionTreatment(page.locator('.summary-hero'));
      const referencePanel = await panelTreatment(
        page.locator('.summary-details > .panel').first(),
      );
      const report = await page.evaluate(() => window.fixture.state.features);
      await nav(page, 'Features');
      await idle(page);
      const workspace = page.locator('.features-page');
      assert.deepEqual(
        await introductionTreatment(workspace.locator('.page-heading')),
        referenceIntro,
      );
      assert.deepEqual(await panelTreatment(workspace.locator('.panel').first()), referencePanel);
      assert.equal(
        await page.getByLabel('Project goals', { exact: true }).inputValue(),
        report.goals,
      );
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      const filter = page.getByLabel('Filter feature suggestions', { exact: true });
      for (const [value, titles] of [
        ['active', [report.suggestions[0].title, 'Saved recovery idea']],
        ['saved', ['Saved recovery idea']],
        ['dismissed', ['Dismissed recovery idea']],
        ['all', report.suggestions.map((idea) => idea.title)],
      ]) {
        await filter.selectOption(value);
        assert.deepEqual(
          await workspace.locator('.feature-grid .panel-head h2').allTextContents(),
          titles,
        );
      }
      await filter.selectOption('active');
      for (const disclosure of await workspace.locator('summary').all()) await disclosure.click();
      const ideaPanel = workspace.locator('.feature-grid .panel').first();
      for (const text of [
        report.suggestions[0].title,
        report.suggestions[0].benefit,
        report.suggestions[0].evidence,
        ...report.suggestions[0].paths,
        ...report.suggestions[0].acceptance_criteria,
        report.generations[0].model_summary.model,
        report.generations[0].model_summary.provider_origin,
      ])
        assert.ok(
          (await workspace.textContent()).includes(text),
          `Complete feature content: ${text}`,
        );
      assert.equal(
        await ideaPanel.getByRole('button', { name: 'Save idea', exact: true }).isEnabled(),
        true,
      );
      const complete = await workspace.textContent();
      for (const theme of ['dark', 'light']) {
        if ((await page.locator('html').getAttribute('data-theme')) !== theme)
          await page
            .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
            .click();
        for (const larger of [false, true]) {
          const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
          if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
            await textSize.click();
          for (const width of [1440, 1280, 1001, 800]) {
            await page.setViewportSize({ width, height: 1000 });
            await featuresLayout(page);
            assert.equal(
              await workspace.textContent(),
              complete,
              'Reflow preserves feature content',
            );
            const columns = await workspace
              .locator('.feature-grid')
              .evaluate(
                (element) => getComputedStyle(element).gridTemplateColumns.split(' ').length,
              );
            assert.equal(columns, width > 1100 ? 2 : 1, 'Keep the task-specific suggestion layout');
            await layout(page, `features-long-${width}-${theme}-${larger ? 'larger' : 'standard'}`);
          }
        }
      }
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
        'Filters, disclosures and appearance perform no writes or generation',
      );
      await ideaPanel.getByRole('button', { name: 'Discuss in chat', exact: true }).click();
      await page.getByLabel('Change request', { exact: true }).waitFor();
      for (const criterion of report.suggestions[0].acceptance_criteria)
        assert.ok(
          (await page.getByLabel('Change request', { exact: true }).inputValue()).includes(
            criterion,
          ),
        );
      const seededRequest = await page.getByLabel('Change request', { exact: true }).inputValue();
      const seededTitle = await page.getByLabel('Task title', { exact: true }).inputValue();
      const seededPaths = await page.getByLabel('Files to change', { exact: true }).inputValue();
      for (const width of [1440, 800]) {
        await page.setViewportSize({ width, height: 1000 });
        await chatConversationLayout(page);
        assert.equal(
          await page.getByLabel('Change request', { exact: true }).inputValue(),
          seededRequest,
        );
        assert.equal(
          await page.getByLabel('Task title', { exact: true }).inputValue(),
          seededTitle,
        );
        assert.equal(
          await page.getByLabel('Files to change', { exact: true }).inputValue(),
          seededPaths,
        );
        assert.equal(
          await page.getByLabel('Run project tests after generation', { exact: true }).isChecked(),
          true,
        );
        await layout(page, `chat-conversation-seeded-${width}-light-larger`);
      }
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some(
            (r) => r.path.endsWith('/messages') || r.path.endsWith('/features/generate'),
          ),
        ),
        false,
        'Discuss only seeds Chat',
      );
    } finally {
      await close();
    }
  });
  await test('Features boundary states keep unavailable, empty, stale and failure meanings', async () => {
    for (const [state, options, title] of [
      ['not-generated', {}, 'No features yet'],
      ['empty', { featuresReady: true, featuresEmpty: true }, 'No new features found'],
      ['unavailable', { featuresReadFail: true }, 'Features unavailable'],
      [
        'failed-empty',
        { featuresReady: true, featuresEmpty: true, featuresFail: true },
        'No saved features',
      ],
      ['stale', { featuresReady: true, featuresStale: true, featuresLongContent: true }, null],
    ]) {
      const { page, close } = await pageFor(options);
      try {
        await idle(page);
        await nav(page, 'Features');
        await idle(page);
        if (title) await page.getByRole('heading', { name: title, exact: true }).waitFor();
        if (state === 'unavailable') {
          assert.equal(
            await page.getByRole('button', { name: 'Suggest features', exact: true }).isDisabled(),
            true,
          );
          await page.locator('.features-page').getByText('Error details', { exact: true }).click();
          await page.getByText('Saved suggestions could not be read.', { exact: true }).waitFor();
        }
        if (state === 'stale') {
          await page.getByText(/active idea is outdated/).waitFor();
          assert.equal(
            await page.getByRole('button', { name: 'Discuss in chat', exact: true }).isDisabled(),
            true,
          );
          assert.equal(
            await page.getByRole('button', { name: 'Save idea', exact: true }).isEnabled(),
            true,
          );
        }
        assert.equal(
          await page.getByText(/Added \d+ new suggestions\.|No new suggestions found\./).count(),
          0,
        );
        for (const theme of ['dark', 'light']) {
          if ((await page.locator('html').getAttribute('data-theme')) !== theme)
            await page
              .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
              .click();
          for (const larger of [false, true]) {
            const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
            if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
              await textSize.click();
            for (const width of [1440, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await featuresLayout(page);
              await layout(
                page,
                `features-${state}-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
              );
            }
          }
        }
      } finally {
        await close();
      }
    }
  });
  await test('Requested feature failure keeps ideas and triage; busy controls stay explicit', async () => {
    const { page, close } = await pageFor({
      featuresReady: true,
      featuresLongContent: true,
      featuresFail: true,
    });
    try {
      await nav(page, 'Features');
      await idle(page);
      await page.getByRole('button', { name: 'Save idea', exact: true }).click();
      await idle(page);
      const retained = await page.evaluate(() => window.fixture.state.features.suggestions);
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/features/generate';
      });
      await page
        .getByRole('button', { name: 'Search more feature suggestions', exact: true })
        .click();
      await page.waitForFunction(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/features/generate')),
      );
      for (const button of await page.locator('.features-page button').all())
        assert.equal(await button.isDisabled(), true);
      assert.equal(await page.getByLabel('Project goals', { exact: true }).isDisabled(), true);
      await page.getByLabel('Filter feature suggestions', { exact: true }).selectOption('saved');
      for (const disclosure of await page.locator('.features-page summary').all())
        await disclosure.click();
      await page.setViewportSize({ width: 800, height: 1000 });
      await page.getByRole('button', { name: 'Larger text', exact: true }).click();
      await featuresLayout(page);
      await layout(page, 'features-busy-800-dark-larger');
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await idle(page);
      await page
        .getByText('Feature search failed. Try again. Previous ideas kept.', { exact: true })
        .waitFor();
      assert.deepEqual(
        await page.evaluate(() => window.fixture.state.features.suggestions),
        retained,
      );
      assert.equal(
        await page.getByLabel('Filter feature suggestions', { exact: true }).inputValue(),
        'saved',
      );
      assert.equal(
        await page.getByText(/Added \d+ new suggestions\.|No new suggestions found\./).count(),
        0,
      );
      assert.equal(
        await page.getByRole('button', { name: 'Discuss in chat', exact: true }).isEnabled(),
        true,
      );
      await featuresLayout(page);
      await layout(page, 'features-requested-failure-retained-800-dark-larger');
      assert.equal(
        await page.evaluate(
          () => window.fixture.requests.filter((r) => r.path.endsWith('/features/generate')).length,
        ),
        1,
      );
    } finally {
      try {
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
      } finally {
        await close();
      }
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
    await page
      .getByRole('button', { name: 'Search more feature suggestions', exact: true })
      .click();
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
    await idle(page);
    assert.equal(await page.getByText(/Feature search failed/).count(), 0);
    assert.equal(await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(), 1);
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/features/generate')),
      ),
      false,
    );
    await nav(page, 'Summary');
    assert.equal(await page.getByText(/Feature search failed/).count(), 0);
    assert.equal(await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(), 0);
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
      false,
    );
    await startAnalysis(page);
    const requests = await page.evaluate(() => window.fixture.requests);
    assert.equal(
      requests.find((r) => r.path.endsWith('/analysis/preview')).body.include_features,
      true,
    );
    const start = requests.find((r) => r.path.endsWith('/analysis/run') && r.method === 'POST');
    assert.equal(start.body.include_features, true);
    assert.deepEqual(start.body.confirmations.provider_ids, ['code-bug', 'review-analyze']);
    await page.getByRole('heading', { name: 'New feature suggestions', exact: true }).waitFor();
    await layout(page, 'analysis-with-features');
    await nav(page, 'Summary');
    assert.equal(
      await page.locator('.metric-card[data-accent="features"] .metric-number').innerText(),
      '1',
    );
    assert.equal(
      await page.evaluate(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/features/generate')),
      ),
      false,
    );
    await layout(page, 'summary-analysis-features');
    await close();
  });
  await test('Feature generation accumulates ideas and identifies duplicates', async () => {
    const { page, close } = await pageFor({ remote: true });
    await nav(page, 'Features');
    await page
      .getByLabel('Project goals', { exact: true })
      .fill('Help operators recover failed jobs.');
    await page.getByRole('button', { name: 'Save goals', exact: true }).click();
    await idle(page);
    await page.getByRole('button', { name: 'Suggest features', exact: true }).click();

    // Check consent shape
    await page
      .getByRole('heading', { name: 'Search more feature suggestions', exact: true })
      .waitFor();
    await page.getByText('Origin: Features').waitFor();
    await page.getByText('Profile: analyze').waitFor();
    await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click();
    await idle(page);

    await page.getByRole('heading', { name: 'Retry failed work', exact: true }).waitFor();
    await page.getByText('Added 1 new suggestions.').waitFor();

    // Now request again with duplicateOnly
    await page.evaluate(() => {
      window.fixture.options.duplicateOnly = true;
    });

    await page
      .getByRole('button', { name: 'Search more feature suggestions', exact: true })
      .click();
    await page.getByText('Existing idea titles: 1').waitFor();
    await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click();
    await idle(page);

    await page.getByText('No new suggestions found.').waitFor();

    // Check profile/consent request-shape cases
    const requests = await page.evaluate(() =>
      window.fixture.requests.filter((r) => r.path.endsWith('/features/generate')),
    );
    assert.equal(requests.length, 2);
    assert.equal(requests[0].body.profile, 'analyze');
    assert.equal(requests[0].body.confirm_remote_provider, true);

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
    await nav(page, 'Summary');
    assert.equal(
      await page.locator('.metric-card[data-accent="features"] .metric-number').innerText(),
      '1',
    );
    await nav(page, 'Features');
    await page.getByLabel('Filter feature suggestions').selectOption('saved');
    await layout(page, 'features-saved');
    await contrast(page, 'Feature suggestions');
    await page.getByRole('button', { name: 'Dismiss', exact: true }).click();
    await idle(page);
    await page.getByRole('heading', { name: 'No matching features' }).waitFor();
    await nav(page, 'Summary');
    assert.equal(
      await page.locator('.metric-card[data-accent="features"] .metric-number').innerText(),
      '0',
    );
    await nav(page, 'Features');
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
  await test('Every instruction step retains Summary panels, long guidance and passive reflow', async () => {
    const directory = `internal/${'LongDirectory'.repeat(24)}/AGENTS.md`;
    for (const target of ['AGENTS.md', directory]) {
      const { page, close } = await pageFor({
        instructionPath: directory,
        instructionsLongContent: true,
        projectPath: `/fixture/${'LongProjectRoot'.repeat(24)}`,
      });
      try {
        const reference = await panelTreatment(page.locator('.summary-details > .panel').first());
        await nav(page, 'Instructions');
        await page.getByText('AGENTS.md · scope .', { exact: true }).waitFor();
        await page.getByLabel('Instruction path').fill('temporary/AGENTS.md');
        await page.getByLabel('Instruction path').fill(target);
        assert.equal(
          await page.locator('.instructions-page summary').count(),
          0,
          'Path editing clears the previous scope',
        );
        assert.equal(await page.getByLabel('Instruction path').inputValue(), target);
        const initialInstructions = await page.evaluate(() => window.fixture.state.instructions);
        for (const step of [1, 2, 3]) {
          if (step === 2) {
            await page.getByRole('button', { name: 'Load scope', exact: true }).click();
            await page.getByLabel('Custom instructions').waitFor();
            for (const summary of await page.locator('.instructions-page summary').all())
              await summary.click();
            const effective = page.getByRole('region', {
              name: 'Effective project guidance',
              exact: true,
            });
            for (const inherited of target === directory
              ? ['AGENTS.md', 'internal/AGENTS.md', directory]
              : ['AGENTS.md']) {
              for (const paragraph of initialInstructions[inherited]
                .split(/\n\s*\n/)
                .filter(Boolean))
                assert.ok(
                  (await effective.textContent()).includes(paragraph),
                  'Inherited guidance remains complete',
                );
            }
          }
          if (step === 3)
            await page.getByRole('button', { name: 'Continue to preview', exact: true }).click();
          assert.equal(
            await page.locator('.wizard-steps [aria-current="step"]').innerText(),
            `${step}. ${['Choose scope', 'Edit guidance', 'Preview'][step - 1]}`,
          );
          assert.deepEqual(
            await panelTreatment(page.locator('.instructions-page .panel').first()),
            reference,
          );
          for (const theme of ['dark', 'light']) {
            if ((await page.locator('html').getAttribute('data-theme')) !== theme)
              await page
                .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
                .click();
            for (const larger of [false, true]) {
              const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
              if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
                await textSize.click();
              for (const width of [1440, 1280, 1001, 800]) {
                await page.setViewportSize({ width, height: 1000 });
                await instructionsLayout(page);
                if (step === 2)
                  assert.equal(
                    await page.getByLabel('Custom instructions').inputValue(),
                    initialInstructions[target],
                  );
                if (step === 3) {
                  assert.deepEqual(
                    await page
                      .locator('.instructions-page .prose')
                      .first()
                      .locator('p')
                      .allTextContents(),
                    initialInstructions[target].split(/\n\s*\n/).filter(Boolean),
                    'The complete proposed guidance remains read-only and visible',
                  );
                  assert.equal(
                    await page
                      .getByRole('button', { name: 'Preview instruction diff', exact: true })
                      .isDisabled(),
                    true,
                    'Unchanged guidance cannot create a proposal',
                  );
                }
                await layout(
                  page,
                  `instructions-${target === directory ? 'directory' : 'root'}-step-${step}-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
                );
              }
              await contrast(
                page,
                `Instructions step ${step} ${theme} ${larger ? 'larger' : 'standard'}`,
              );
            }
          }
          // Reset appearance before comparing the next step to the dark/standard reference.
          await page
            .getByRole('button', { name: 'Switch to dark appearance', exact: true })
            .click();
          await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        }
        await page.getByRole('button', { name: 'Edit guidance', exact: true }).click();
        await page
          .getByLabel('Custom instructions')
          .fill(`${initialInstructions[target]}\nKeep boundary diagnostics complete.\n`);
        const edited = await page.getByLabel('Custom instructions').inputValue();
        await page.getByRole('button', { name: 'Continue to preview', exact: true }).click();
        assert.equal(
          await page
            .getByRole('button', { name: 'Preview instruction diff', exact: true })
            .isEnabled(),
          true,
        );
        await page.getByRole('button', { name: 'Edit guidance', exact: true }).click();
        assert.equal(await page.getByLabel('Custom instructions').inputValue(), edited);
        await page.getByRole('button', { name: 'Back to scope', exact: true }).click();
        assert.equal(await page.getByLabel('Instruction path').inputValue(), target);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.state.instructions),
          initialInstructions,
        );
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter(
              (r) => r.method !== 'GET' && !r.path.endsWith('/restore'),
            ),
          ),
          [],
          'Wizard navigation, editing, disclosures and reflow never generate, execute or write',
        );
        assert.deepEqual(await page.evaluate(() => window.fixture.terminals), []);
      } finally {
        await close();
      }
    }
  });
  await test('Instruction exclusions and stale scopes remain visible and block editing or proposals', async () => {
    const target = `internal/${'PolicyScope'.repeat(30)}/AGENTS.md`;
    for (const state of ['excluded', 'stale']) {
      const { page, close } = await pageFor({
        instructionPath: target,
        instructionsLongContent: true,
        instructionsExcluded: state === 'excluded',
        instructionsStale: state === 'stale',
      });
      try {
        await nav(page, 'Instructions');
        await page.getByLabel('Instruction path').fill(target);
        await page.getByRole('button', { name: 'Load scope', exact: true }).click();
        if (state === 'excluded') {
          await page
            .getByText('This AGENTS.md is excluded by project context policy.', { exact: true })
            .waitFor();
          for (const control of [
            page.getByLabel('Custom instructions'),
            page.getByLabel('Keep changes focused'),
            page.getByRole('button', { name: 'Add selected guidance', exact: true }),
            page.getByRole('button', { name: 'Continue to preview', exact: true }),
          ])
            assert.equal(await control.isDisabled(), true);
          await page.getByRole('button', { name: 'Back to scope', exact: true }).click();
          await page
            .getByText('This AGENTS.md is excluded by project context policy.', { exact: true })
            .waitFor();
        } else {
          await page.getByText('Load this scope again before editing.', { exact: true }).waitFor();
          assert.equal(await page.getByLabel('Custom instructions').count(), 0);
          assert.equal(
            await page.getByRole('button', { name: 'Continue to preview', exact: true }).count(),
            0,
          );
        }
        await page.getByText('AGENTS.md · scope .', { exact: true }).click();
        for (const theme of ['dark', 'light']) {
          if ((await page.locator('html').getAttribute('data-theme')) !== theme)
            await page
              .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
              .click();
          for (const larger of [false, true]) {
            const textSize = page.getByRole('button', { name: 'Larger text', exact: true });
            if (((await textSize.getAttribute('aria-pressed')) === 'true') !== larger)
              await textSize.click();
            for (const width of [1440, 800]) {
              await page.setViewportSize({ width, height: 1000 });
              await instructionsLayout(page);
              await layout(
                page,
                `instructions-${state}-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
              );
            }
          }
        }
        assert.equal(
          await page.evaluate(() =>
            window.fixture.requests.some((r) => r.path.endsWith('/instructions/proposal')),
          ),
          false,
        );
      } finally {
        await close();
      }
    }
  });
  await test('Instruction scope loading, read failure and busy proposal retain recovery and guards', async () => {
    const { page, close } = await pageFor();
    try {
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/instructions';
      });
      await nav(page, 'Instructions');
      await page
        .getByText('Load a scope to inspect its effective project guidance.', { exact: true })
        .waitFor();
      await page.waitForFunction(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/instructions')),
      );
      await instructionsLayout(page);
      await layout(page, 'instructions-scope-loading-1440-dark-standard');
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await page.getByText('AGENTS.md · scope .', { exact: true }).waitFor();
      await page.getByLabel('Instruction path').fill('internal/AGENTS.md');
      await page.evaluate(() => {
        window.fixture.failures['/api/projects/current/instructions'] = 503;
      });
      await page.getByRole('button', { name: 'Load scope', exact: true }).click();
      await page.getByText('project instructions: Fixture rejection', { exact: true }).waitFor();
      assert.equal(await page.locator('.instructions-page summary').count(), 0);
      assert.equal(await page.getByLabel('Instruction path').inputValue(), 'internal/AGENTS.md');
      await page.setViewportSize({ width: 800, height: 1000 });
      await page.getByRole('button', { name: 'Larger text', exact: true }).click();
      await instructionsLayout(page);
      await layout(page, 'instructions-scope-read-failed-800-dark-larger');
      await page.getByRole('button', { name: 'Load scope', exact: true }).click();
      await page.getByLabel('Custom instructions').waitFor();
      assert.equal(
        await page.getByText('project instructions: Fixture rejection', { exact: true }).count(),
        0,
      );
      await page
        .getByLabel('Custom instructions')
        .fill('# Internal rules\n\nPreserve cancellation and error details.\n');
      await page.getByRole('button', { name: 'Continue to preview', exact: true }).click();
      const before = await page.evaluate(() => window.fixture.state.instructions);
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/instructions/proposal';
        window.fixture.failures['/api/projects/current/instructions/proposal'] = 503;
      });
      await page.getByRole('button', { name: 'Preview instruction diff', exact: true }).click();
      await page.waitForFunction(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/instructions/proposal')),
      );
      for (const button of await page.locator('.instructions-page button').all())
        assert.equal(await button.isDisabled(), true);
      await instructionsLayout(page);
      await layout(page, 'instructions-preview-busy-800-dark-larger');
      await page.evaluate(() => {
        window.fixture.hold = '';
        window.fixture.release();
      });
      await idle(page);
      await page.getByRole('alert').filter({ hasText: 'Fixture rejection' }).waitFor();
      assert.equal(
        await page
          .getByRole('button', { name: 'Preview instruction diff', exact: true })
          .isEnabled(),
        true,
      );
      await page.getByRole('button', { name: 'Edit guidance', exact: true }).click();
      assert.equal(
        await page.getByLabel('Custom instructions').inputValue(),
        '# Internal rules\n\nPreserve cancellation and error details.\n',
      );
      assert.deepEqual(await page.evaluate(() => window.fixture.state.instructions), before);
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some(
            (r) =>
              r.path.endsWith('/apply') ||
              r.path.endsWith('/messages') ||
              (r.path.endsWith('/execution-trust') && r.method === 'POST'),
          ),
        ),
        false,
      );
    } finally {
      try {
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
      } finally {
        await close();
      }
    }
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
    await instructionsLayout(page);
    await layout(page, 'instructions-guidance');
    await contrast(page, 'Instruction wizard');
    await page.setViewportSize({ width: 900, height: 640 });
    await page.getByRole('button', { name: 'Larger text' }).click();
    await instructionsLayout(page);
    await layout(page, 'instructions-large-text');
    await page.getByRole('button', { name: 'Continue to preview', exact: true }).click();
    await instructionsLayout(page);
    await layout(page, 'instructions-preview-900-dark-larger');
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

      const hasIdeasStart = options.featuresReady && !options.featuresEmpty;
      await page
        .getByRole('button', {
          name: hasIdeasStart ? 'Search more feature suggestions' : 'Suggest features',
        })
        .click();

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
      if (options.featuresStale && hasIdeasStart)
        assert.equal(
          await page.getByRole('button', { name: 'Discuss in chat' }).isDisabled(),
          true,
        );
      assert.equal(
        await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(),
        options.featuresStale && hasIdeasStart ? 1 : 0,
      );
      await page.setViewportSize({ width: 800, height: 900 });
      await page.getByRole('button', { name: 'Larger text' }).click();
      await layout(page, `features-${JSON.stringify(options)}`);
      await nav(page, 'Summary');
      assert.equal(await page.getByText(/Feature search failed/).count(), 0);
      assert.equal(await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(), 0);
      const card = page.locator('.metric-card[data-accent="features"]');
      assert.equal(
        await card.locator('.metric-number').innerText(),
        hasIdeas ? '1' : options.featuresFail ? '—' : '0',
      );
      assert.equal(
        await card.getByRole('img').getAttribute('aria-label'),
        `Features: ${options.featuresFail ? 'failed' : options.featuresStale ? 'stale' : 'ready'}`,
      );
      await card.click();
      if (options.featuresFail)
        await page
          .getByText(
            `Feature search failed. Try again.${hasIdeas ? ' Previous ideas kept.' : ''}`,
            { exact: true },
          )
          .waitFor();
      assert.equal(
        await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(),
        options.featuresStale && hasIdeasStart ? 1 : 0,
      );
      await nav(page, 'Project');
      await page.getByRole('button', { name: 'Open saved project', exact: true }).click();
      await idle(page);
      await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
      assert.equal(await page.getByText(/Feature search failed/).count(), 0);
      assert.equal(await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(), 0);
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
      await startAnalysis(page);
      await nav(page, 'Summary');
      assert.equal(await page.getByText(/Feature search failed/).count(), 0);
      const failure = page.getByRole('img', { name: 'Features: failed', exact: true });
      assert.equal(await failure.evaluate((dot) => dot.classList.contains('red')), true);
      assert.equal(
        await page.locator('.metric-card[data-accent="features"] .metric-number').innerText(),
        featuresReady ? '1' : '—',
      );
      assert.equal(await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(), 0);
      await nav(page, 'Features');
      await page.getByText(/^Feature search failed\. Try again\./).waitFor();
      assert.equal(
        await page.getByText(/active idea(?:s)? (?:is|are) outdated/).count(),
        featuresReady ? 1 : 0,
      );
      await close();
    }
  });
  await test('Chat outcomes keep receipt, verification and recovery evidence distinct across layouts', async () => {
    const warning = `Source changed; history warning ${'long_warning_'.repeat(65)} final warning.`;
    const reason = `Verification reason ${'long_reason_'.repeat(50)} final reason.`;
    const diagnostic = `Complete output\n${'diagnostic_'.repeat(100)}\nFinal diagnostic line.`;
    const checkName = `Post-Apply tests ${'LongCheckName'.repeat(30)}`;
    for (const state of [
      'applied',
      'verified',
      'failed',
      'unavailable',
      'undone',
      'prepared',
      'recovery_required',
      'index-unavailable',
      'response-lost',
    ]) {
      const verification = ['verified', 'failed', 'unavailable'].includes(state);
      const uncertain = ['index-unavailable', 'response-lost'].includes(state);
      const { page, close } = await pageFor({
        mutationWarning: warning,
        recoveryDropsWarnings: state !== 'response-lost',
        changeMutationState: ['prepared', 'recovery_required'].includes(state) ? state : undefined,
        changeMutationMissingIndex: state === 'index-unavailable',
        changeMutationResponseLost: state === 'response-lost',
        changeVerification: verification
          ? {
              status: state,
              reason,
              checks:
                state === 'unavailable'
                  ? []
                  : [
                      {
                        name: checkName,
                        state: state === 'failed' ? 'failed' : 'passed',
                        output: diagnostic,
                        required: true,
                      },
                    ],
            }
          : undefined,
      });
      try {
        await idle(page);
        const reference = await panelTreatment(page.locator('.summary-details .panel').first());
        await nav(page, 'Chat');
        await page.getByLabel('Add an existing file').selectOption('internal/worker/process.go');
        await page.getByLabel('Change request', { exact: true }).fill('Handle cancellation.');
        await page.getByLabel('Run project tests after generation').uncheck();
        await page.getByRole('button', { name: 'Prepare change', exact: true }).click();
        await idle(page);
        await page.getByRole('button', { name: 'Review this diff', exact: true }).click();
        await idle(page);
        await page.getByRole('button', { name: 'Approve and apply', exact: true }).click();
        await page
          .getByRole('dialog')
          .getByRole('button', { name: 'Apply proposal', exact: true })
          .click();
        await idle(page);
        const outcome = page.getByRole('region', { name: 'Change outcome', exact: true });
        if (state === 'response-lost') {
          await outcome.getByText('Write outcome unknown.', { exact: true }).waitFor();
          assert.equal(await outcome.locator('.chat-receipt').count(), 0);
          await chatOutcomeLayout(page);
          await layout(page, 'chat-outcome-response-lost-before-recovery-1440-dark-standard');
          await nav(page, 'Summary');
          await nav(page, 'Chat');
          await outcome.getByRole('heading', { name: 'Change applied', exact: true }).waitFor();
        }
        await outcome.getByText(warning, { exact: true }).waitFor();
        const render = async (label) => {
          const before = await page.evaluate(() =>
            window.fixture.requests.filter((r) => r.method !== 'GET'),
          );
          for (const theme of ['dark', 'light']) {
            if ((await page.locator('html').getAttribute('data-theme')) !== theme)
              await page
                .getByRole('button', { name: `Switch to ${theme} appearance`, exact: true })
                .click();
            for (const larger of [false, true]) {
              const size = page.getByRole('button', { name: 'Larger text', exact: true });
              if (((await size.getAttribute('aria-pressed')) === 'true') !== larger)
                await size.click();
              for (const width of ['applied', 'failed', 'response-lost'].includes(state)
                ? [1440, 1280, 1001, 800]
                : [1440, 800]) {
                await page.setViewportSize({ width, height: 1000 });
                await chatOutcomeLayout(page);
                await layout(
                  page,
                  `chat-outcome-${label}-${width}-${theme}-${larger ? 'larger' : 'standard'}`,
                );
              }
              await contrast(
                page,
                `Chat outcome ${label} ${theme} ${larger ? 'larger' : 'standard'}`,
              );
            }
          }
          assert.deepEqual(
            await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
            before,
            'Outcome inspection and reflow never verify, analyze or write',
          );
          assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
        };
        if (verification) {
          await outcome.getByRole('button', { name: 'Verify applied change', exact: true }).click();
          await page
            .getByRole('dialog')
            .getByRole('button', { name: 'Trust this project', exact: true })
            .click();
          await idle(page);
          const evidence = outcome.getByLabel('Post-Apply verification', { exact: true });
          await evidence.getByText(state, { exact: true }).first().waitFor();
          await evidence.getByText(reason, { exact: true }).waitFor();
          const before = await page.evaluate(() =>
            window.fixture.requests.filter((r) => r.method !== 'GET'),
          );
          if (state !== 'unavailable') {
            await evidence.locator('summary').click();
            assert.equal(await evidence.locator('pre').textContent(), diagnostic);
          } else assert.equal(await evidence.locator('details').count(), 0);
          assert.deepEqual(
            await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
            before,
            'Verification output disclosure is passive',
          );
        }
        if (state === 'undone') {
          await outcome.getByRole('button', { name: 'Undo proposal', exact: true }).click();
          await page
            .getByRole('dialog')
            .getByRole('button', { name: 'Undo proposal', exact: true })
            .click();
          await idle(page);
        }
        await outcome
          .getByRole('heading', {
            name: `Change ${['prepared', 'recovery_required', 'undone'].includes(state) ? state.replaceAll('_', ' ') : 'applied'}`,
            exact: true,
          })
          .waitFor();
        assert.deepEqual(await panelTreatment(outcome.locator('.panel')), reference);
        const applied = !['prepared', 'recovery_required', 'undone'].includes(state);
        assert.equal(
          await outcome
            .getByText('Acceptance criteria still need review.', { exact: true })
            .count(),
          applied ? 1 : 0,
        );
        assert.equal(
          await outcome.getByText('No post-Apply verification available.', { exact: true }).count(),
          applied && !verification ? 1 : 0,
        );
        assert.equal(
          await outcome.getByRole('button', { name: 'Undo proposal', exact: true }).count(),
          state === 'undone' ? 0 : 1,
        );
        for (const control of await outcome.getByRole('button').all())
          assert.equal(
            await control.isDisabled(),
            uncertain && (await control.textContent()).trim() !== 'Refresh project',
          );
        assert.equal(
          await page.getByRole('button', { name: 'Approve and apply', exact: true }).isDisabled(),
          true,
        );
        await render(state);
        if (['prepared', 'recovery_required'].includes(state)) {
          const undoPath = '/api/projects/current/changes/change-1/undo';
          await page.evaluate((path) => {
            window.fixture.hold = path;
          }, undoPath);
          await outcome.getByRole('button', { name: 'Undo proposal', exact: true }).click();
          await page
            .getByRole('dialog')
            .getByText('Recover the interrupted grouped change.', { exact: true })
            .waitFor();
          await page
            .getByRole('dialog')
            .getByRole('button', { name: 'Undo proposal', exact: true })
            .click();
          await page.waitForFunction(
            (path) => window.fixture.requests.some((r) => r.path === path),
            undoPath,
          );
          assert.equal(
            await outcome.getByRole('button', { name: 'Undo proposal', exact: true }).isDisabled(),
            true,
          );
          await render(`${state}-undo-busy`);
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
          await idle(page);
          await outcome.getByRole('heading', { name: 'Change undone', exact: true }).waitFor();
          assert.equal(await outcome.getByRole('button').count(), 0);
          await render(`${state}-restored`);
        }
        if (state === 'applied' || uncertain) {
          const heldPath = uncertain
            ? '/api/projects/current/reindex'
            : '/api/projects/current/changes/change-1/verify';
          await page.evaluate((path) => {
            window.fixture.hold = path;
          }, heldPath);
          await outcome
            .getByRole('button', {
              name: uncertain ? 'Refresh project' : 'Verify applied change',
              exact: true,
            })
            .click();
          if (!uncertain)
            await page
              .getByRole('dialog')
              .getByRole('button', { name: 'Trust this project', exact: true })
              .click();
          await page.waitForFunction(
            (path) => window.fixture.requests.some((r) => r.path === path),
            heldPath,
          );
          for (const control of await outcome.getByRole('button').all())
            assert.equal(await control.isDisabled(), true);
          await render(`${state}-busy`);
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
          await idle(page);
          if (uncertain) {
            assert.equal(
              await outcome.getByText('Write outcome unknown.', { exact: true }).count(),
              0,
            );
            assert.equal(
              await page
                .getByRole('button', { name: 'Approve and apply', exact: true })
                .isDisabled(),
              true,
            );
            assert.equal(
              await outcome.getByRole('button', { name: 'Undo proposal', exact: true }).isEnabled(),
              true,
            );
            await outcome.getByRole('button', { name: 'Undo proposal', exact: true }).click();
            await page
              .getByRole('dialog')
              .getByRole('button', { name: 'Undo proposal', exact: true })
              .click();
            await idle(page);
            await outcome.getByRole('heading', { name: 'Change undone', exact: true }).waitFor();
            await render(`${state}-recovered-undone`);
          }
        }
        assert.equal(
          await page.evaluate(
            () => window.fixture.requests.filter((r) => r.path.endsWith('/apply')).length,
          ),
          1,
          'No repeat Apply occurs',
        );
      } finally {
        try {
          await page.evaluate(() => {
            window.fixture.hold = '';
            window.fixture.release();
          });
        } finally {
          await close();
        }
      }
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
  await test('Analysis screen repair and feature actions', async () => {
    // 1. Without recovery
    let result = await pageFor({ runStatus: 'completed' });
    await nav(result.page, 'Analysis');
    assert.equal(await result.page.getByRole('button', { name: 'Repair analysis' }).count(), 0);
    await result.close();

    // 2. With recovery
    result = await pageFor({
      runStatus: 'completed',
      hasRecovery: true,
      featuresReady: true,
      featuresEmpty: true,
      remote: true,
    });
    await nav(result.page, 'Analysis');
    await result.page.getByRole('button', { name: 'Repair analysis' }).waitFor();
    await result.page.getByRole('button', { name: 'Search more feature suggestions' }).waitFor();

    // Check tooltip is not there when not busy
    assert.equal(
      await result.page
        .getByRole('button', { name: 'Search more feature suggestions' })
        .getAttribute('title'),
      null,
    );

    // Make it busy by preparing a run and holding the request
    await result.page.evaluate(() => {
      window.fixture.hold = '/api/projects/current/analysis/preview';
    });
    await result.page.getByRole('button', { name: 'Prepare analysis' }).click();

    // Check that Search button gets disabled and has tooltip
    assert.equal(
      await result.page
        .getByRole('button', { name: 'Search more feature suggestions' })
        .isDisabled(),
      true,
    );
    assert.equal(
      await result.page
        .getByRole('button', { name: 'Search more feature suggestions' })
        .getAttribute('title'),
      'Cannot search while another operation is running.',
    );

    await result.page.evaluate(() => {
      window.fixture.hold = undefined;
      window.fixture.release();
    });
    await idle(result.page);
    await result.page.getByRole('button', { name: 'Back' }).click();

    // Test clicking Repair analysis
    await result.page.getByRole('button', { name: 'Repair analysis' }).click();
    await idle(result.page);
    await result.page.getByRole('heading', { name: 'Repair analysis' }).waitFor();
    let requests = await result.page.evaluate(() => window.fixture.requests);
    let previewReq = requests.reverse().find((r) => r.path.endsWith('/analysis/preview'));
    assert.equal(previewReq.body.recover_incomplete, true);

    await result.page.getByRole('button', { name: 'Back' }).click();
    await idle(result.page);

    // Test clicking Search more feature suggestions
    await result.page.getByRole('button', { name: 'Search more feature suggestions' }).click();
    await result.page.getByText('Origin: Analysis').waitFor();
    await result.page
      .getByRole('dialog')
      .getByRole('button', { name: 'Continue', exact: true })
      .click();
    await idle(result.page);

    requests = await result.page.evaluate(() => window.fixture.requests);
    const generateReq = requests.reverse().find((r) => r.path.endsWith('/features/generate'));
    assert.equal(generateReq.body.profile, 'analyze');
    assert.equal(typeof generateReq.body.analysis_selection_id, 'string');
    assert.equal(generateReq.body.analysis_selection_id.startsWith('selection-'), true);
    assert.equal(generateReq.body.confirm_remote_provider, true);

    await result.close();
  });
  assert.deepEqual(errors, []);
  await writeFile(
    `${output}/presentation-evidence.json`,
    JSON.stringify({ surfaceInventory, aliases, captures }, null, 2),
  );
  console.log(`PASS ${checks} workflow and layout checks; no browser errors or external requests`);
} finally {
  try {
    await browser?.close();
  } finally {
    if (server)
      await new Promise((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
  }
}
