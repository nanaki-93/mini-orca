import { chromium } from '@playwright/test';
import { strict as assert } from 'node:assert';
import { serve } from '../preview.mjs';
import { installFixture } from './fixture.mjs';
import { testThemes } from './themes.mjs';

const server = await serve(0);
const url = `http://127.0.0.1:${server.address().port}`;
let browser;
try {
  browser = await chromium.launch();
} catch {
  browser = await chromium.launch({
    executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  });
}

async function idle(page) {
  await page.locator('.busy-strip').waitFor({ state: 'hidden' });
}

async function pageFor(options = {}) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  const page = await context.newPage();
  await page.addInitScript(installFixture, options);
  await page.goto(url);
  await page.getByRole('heading', { name: options.projectName || 'harbor', exact: true }).waitFor();
  await idle(page);
  return { page, close: () => context.close() };
}

async function nav(page, label) {
  const sidebar = page.getByRole('complementary', { name: 'Application' });
  await sidebar.getByRole('button', { name: label, exact: true }).click();
}

async function test(name, fn) {
  await fn();
  console.log(`PASS: ${name}`);
}

try {
  await testThemes({ test, pageFor, nav, idle });

  await test('Overview displays 3-card Project Pulse and interactive Architecture entry points', async () => {
    const { page, close } = await pageFor();
    try {
      assert.equal(await page.locator('.kpi-pulse-card').count(), 3);
      await page.getByText('System Architecture & Entry Points', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Open Code & Diff', exact: true }).click();
      await idle(page);
      await page.locator('.unified-ide-shell').waitFor();
    } finally {
      await close();
    }
  });

  await test('Unified Code & Diff supports Direct Edit, Save File, Hunk-by-Hunk Diff Review, and Docked AI Composer', async () => {
    const { page, close } = await pageFor({ proposalReady: true });
    try {
      await nav(page, 'Code & Diff');
      await idle(page);
      await page.locator('.unified-ide-shell').waitFor();

      // Open a file in the explorer
      await page.locator('.explorer-file-btn').first().click();
      await idle(page);

      // Verify Hunk-by-Hunk Diff Review controls
      await page.getByRole('tab', { name: 'Diff Review', exact: true }).click();
      const acceptHunk = page.getByRole('button', { name: 'Accept Hunk', exact: true }).first();
      if ((await acceptHunk.count()) > 0) {
        await acceptHunk.click();
      }

      // Switch to Direct Edit mode and edit inline
      await page.getByRole('tab', { name: 'Direct Edit', exact: true }).click();
      const editor = page.getByLabel('Direct code editor', { exact: true });
      await editor.waitFor();
      await editor.fill('package worker\n\n// Updated inline\n');
      assert.equal(
        await page.getByRole('button', { name: 'Save File (⌘S)', exact: true }).first().isEnabled(),
        true,
      );

      // Toggle Explorer collapse
      await page.getByRole('button', { name: 'Toggle file explorer', exact: true }).click();
      assert.equal(await page.locator('.unified-ide-explorer.collapsed').count(), 1);
      await page.getByRole('button', { name: 'Toggle file explorer', exact: true }).click();

      // Open History drawer in docked composer
      await page.getByRole('button', { name: /^History \(/ }).click();
      await page.getByLabel('Conversation history', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Issues & Ideas unifies Bugs, Security, Performance, and Features with Inspector card', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    try {
      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByRole('heading', { name: 'Issues & Feature Ideas', exact: true }).waitFor();
      await page.getByText('Selected Issue Inspector', { exact: true }).waitFor();
      await page.getByRole('button', { name: /^Features \(/ }).click();
      await page.getByRole('button', { name: 'Generate & Review Diff', exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Git & Worktrees and Settings & Rules with Consolidated Workspace Trust', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git & Worktrees');
      await idle(page);
      await page.locator('.git-split-grid').waitFor();

      await nav(page, 'Settings & Rules');
      await idle(page);
      const trustCheckbox = page.getByLabel('Consolidated Workspace Trust', { exact: true });
      await trustCheckbox.waitFor();
      await trustCheckbox.check();
      assert.equal(
        await page.evaluate(() => localStorage.getItem('mini-orca:workspace-trust')),
        'true',
      );
    } finally {
      await close();
    }
  });
} finally {
  await browser.close();
  server.close();
}
