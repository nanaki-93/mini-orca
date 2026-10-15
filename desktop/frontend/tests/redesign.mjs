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

  await test('Overview displays 4-box Project Pulse with Tests & Checks box and inline Verification Phases drawer', async () => {
    const { page, close } = await pageFor();
    try {
      assert.equal(await page.locator('.kpi-pulse-card').count(), 4);
      assert.equal(await page.locator('.topbar button:has-text("Run Tests")').count(), 0);
      await page.getByText('System Architecture & Entry Points', { exact: true }).waitFor();

      // Clicking the Tests & Checks box toggles the inline Verification Phases drawer
      await page.locator('.kpi-pulse-card').first().click();
      await page.getByText('Verification Phases', { exact: true }).waitFor();
      await page.locator('.kpi-pulse-card').first().click();

      // Clicking the Active Proposal box navigates to Code & Diff
      await page.locator('.kpi-pulse-card').nth(3).click();
      await idle(page);
      await page.locator('.unified-ide-shell').waitFor();
    } finally {
      await close();
    }
  });

  await test('Engineering Specification (Docs) supports C4 System Context, Subsystem Contracts, Runtime Scenarios, and ADRs', async () => {
    const { page, close } = await pageFor();
    try {
      await nav(page, 'Docs');
      await idle(page);
      await page.getByRole('heading', { name: 'Engineering Specification', exact: true }).waitFor();
      await page.getByText('Layered Architecture Map', { exact: true }).waitFor();

      // Switch to 2. Components & Contracts
      await page.getByRole('tab', { name: /2\. Components/ }).click();
      await page.getByText('Domain Responsibilities', { exact: true }).waitFor();
      await page.getByText('Public Contracts & Interfaces', { exact: true }).waitFor();

      // Switch to 3. Runtime Scenarios
      await page.getByRole('tab', { name: /3\. Runtime Scenarios/ }).click();
      await page.getByText('Step 1 · Admission', { exact: true }).first().waitFor();
      await page.getByText('Step 2 · Execution', { exact: true }).first().waitFor();
      await page.getByText('Step 3 · Publication', { exact: true }).first().waitFor();

      // Switch to 4. Decisions & Ops
      await page.getByRole('tab', { name: /4\. Decisions & Ops/ }).click();
      await page.getByText('Architectural Decision Records (ADRs)', { exact: true }).waitFor();
      await page.getByText('Operational Runbook', { exact: true }).waitFor();
      await page.getByText('Ubiquitous Domain Language (Glossary)', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Unified Code & Diff supports Direct Edit with line gutter, Split/Unified Diff Review, Inspector, and Multi-File Docked AI Composer', async () => {
    const { page, close } = await pageFor({ trusted: true });
    try {
      await nav(page, 'Code & Diff');
      await idle(page);
      await page.locator('.unified-ide-shell').waitFor();

      // Open a file in the explorer
      await page.locator('.explorer-file-btn').first().click();
      await idle(page);

      // Generate a diff proposal from the docked AI composer
      await page
        .getByLabel('Ask about this file', { exact: true })
        .fill('Add context cancellation check');
      await page.getByRole('button', { name: 'Generate Diff', exact: true }).click();
      await idle(page);

      // Verify Hunk-by-Hunk Diff Review controls & Split/Unified diff layout switcher
      await page.getByRole('tab', { name: /^Diff Review/ }).click();
      const diffLayoutGroup = page.getByRole('group', { name: 'Diff layout', exact: true });
      await diffLayoutGroup.waitFor();
      await diffLayoutGroup.getByRole('button', { name: 'Split', exact: true }).click();
      await diffLayoutGroup.getByRole('button', { name: 'Unified', exact: true }).click();
      const acceptHunk = page.getByRole('button', { name: 'Accept Hunk', exact: true }).first();
      await acceptHunk.waitFor();
      await acceptHunk.click();
      await page.getByRole('button', { name: 'Accepted', exact: true }).first().waitFor();

      // Switch to Direct Edit mode, verify line-number gutter, and edit inline
      await page.getByRole('tab', { name: 'Direct Edit', exact: true }).click();
      await page.locator('.code-gutter').waitFor();
      const editor = page.getByLabel('Direct code editor', { exact: true });
      await editor.waitFor();
      await editor.fill('package worker\n\n// Updated inline\n');
      assert.equal(
        await page.getByRole('button', { name: 'Save File (⌘S)', exact: true }).first().isEnabled(),
        true,
      );

      // Toggle side-by-side Symbol Inspector
      await page.getByRole('button', { name: 'Toggle symbol inspector', exact: true }).click();
      await page.getByLabel('Symbol and blast radius inspector', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Toggle symbol inspector', exact: true }).click();

      // Toggle Explorer collapse
      await page.getByRole('button', { name: 'Toggle file explorer', exact: true }).click();
      assert.equal(await page.locator('.unified-ide-explorer.collapsed').count(), 1);
      await page.getByRole('button', { name: 'Toggle file explorer', exact: true }).click();

      // Open multi-file @context picker and History drawer in docked composer
      await page.getByRole('button', { name: 'Add context file', exact: true }).click();
      await page.getByLabel('Context file picker', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Add context file', exact: true }).click();

      await page.getByRole('button', { name: /^History \(/ }).click();
      await page.getByLabel('Conversation history', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Issues & Ideas removes KPI boxes and supports sorting, select-all on page, keyboard j/k nav, and Inspector card', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    try {
      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByRole('heading', { name: 'Issues & Feature Ideas', exact: true }).waitFor();
      assert.equal(await page.locator('.kpi-pulse-card').count(), 0);
      await page.getByText('Selected Issue Inspector', { exact: true }).waitFor();

      // Sort issues and Select all on page
      const sortSelect = page.getByLabel('Sort issues', { exact: true });
      await sortSelect.waitFor();
      await sortSelect.selectOption('path');
      await sortSelect.selectOption('severity');

      const selectAllCheckbox = page.getByLabel('Select all issues on page', { exact: true });
      await selectAllCheckbox.check();
      const batchBar = page.getByRole('region', { name: 'Batch issue actions', exact: true });
      await batchBar.waitFor();
      await selectAllCheckbox.uncheck();

      // Clicking a row keeps the split view and updates Selected Issue Inspector
      const firstRow = page.locator('.result-row').first();
      await firstRow.click();
      await page.getByText('Selected Issue Inspector', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Full Details', exact: true }).waitFor();

      await page.getByRole('button', { name: /^Features \(/ }).click();
      await page.getByText('Project goals & Feature discovery', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Save idea', exact: true }).waitFor();
      await page.getByRole('button', { name: 'Generate & Review Diff', exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Command Palette supports keyboard ArrowDown and Enter navigation', async () => {
    const { page, close } = await pageFor();
    try {
      await page.getByRole('button', { name: 'Search files and commands', exact: true }).click();
      const input = page.locator('#search');
      await input.waitFor();
      await input.fill('Models');
      await input.press('Enter');
      await idle(page);
      await page.getByRole('heading', { name: 'Models', exact: true }).waitFor();
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

  await test('Workflow models share the dynamic Agy, Pi, and configured catalog with Analysis models', async () => {
    const configured = `configured:${'a'.repeat(64)}`;
    const availableModels = {
      defaults: { analyze: configured, bug: configured, function: configured },
      pi: { status: 'ready' },
      agy: { status: 'ready' },
      models: [
        {
          id: configured,
          name: 'Gemini 3.8 Flash',
          provider: 'openai',
          source: 'configured',
          location: 'remote',
          model: {
            model: 'gemini-3.8-flash',
            profile: configured,
            scope: '',
            provider_origin: 'https://provider.invalid',
            remote_provider: true,
            timeout: '2m',
            reasoning_effort: 'medium',
          },
        },
        {
          id: 'agy:gemini-4-argon-high',
          name: 'Gemini 4 Argon (High)',
          provider: 'agy',
          source: 'agy',
          location: 'unknown',
          model: {
            model: 'gemini-4-argon-high',
            profile: 'agy:gemini-4-argon-high',
            scope: '',
            provider_origin: 'cli://agy',
            remote_provider: true,
            timeout: '2m',
            reasoning_effort: '',
          },
        },
        {
          id: 'pi:lm-studio/qwen/local',
          name: 'Qwen Local',
          provider: 'lm-studio',
          source: 'pi',
          location: 'local',
          model: {
            model: 'lm-studio/qwen/local',
            profile: 'pi:lm-studio/qwen/local',
            scope: '',
            provider_origin: 'cli://pi',
            remote_provider: true,
            timeout: '2m',
            reasoning_effort: '',
          },
        },
      ],
    };
    const { page, close } = await pageFor({ availableModels, trusted: true });
    try {
      await nav(page, 'Settings & Rules');
      await idle(page);
      await page.getByRole('button', { name: 'Model Routing', exact: true }).click();
      await idle(page);

      const bugModel = page.getByLabel('Bug analysis model', { exact: true });
      await bugModel.click();
      const dialog = page.getByRole('dialog', { name: 'Choose a model', exact: true });
      await dialog.getByRole('button', { name: 'Agy', exact: true }).click();
      await dialog.getByRole('option', { name: /Gemini 4 Argon \(High\)/ }).click();
      assert.equal(await bugModel.getAttribute('value'), 'agy:gemini-4-argon-high');

      const creation = page.getByLabel('Creation model', { exact: true });
      assert.equal(await creation.getAttribute('value'), configured);
      await creation.click();
      await dialog.getByRole('button', { name: 'Pi', exact: true }).click();
      await dialog.getByRole('option', { name: /Qwen Local/ }).click();
      assert.equal(await creation.getAttribute('value'), 'pi:lm-studio/qwen/local');

      const testing = page.getByLabel('Testing model', { exact: true });
      await testing.click();
      await dialog.getByRole('button', { name: 'Agy', exact: true }).click();
      await dialog.getByRole('option', { name: /Gemini 4 Argon \(High\)/ }).click();
      assert.equal(await testing.getAttribute('value'), 'agy:gemini-4-argon-high');

      const batchInput = page.locator('#settings-batch_files');
      await batchInput.fill('42');
      await page
        .getByLabel('Refresh previously analyzed files on next run', { exact: true })
        .check();

      // Reload the app to verify model selections and batch settings persist across restarts and project restore
      await page.reload();
      await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
      await idle(page);

      await nav(page, 'Settings & Rules');
      await idle(page);
      await page.getByRole('button', { name: 'Model Routing', exact: true }).click();
      await idle(page);

      assert.equal(
        await page.getByLabel('Bug analysis model', { exact: true }).getAttribute('value'),
        'agy:gemini-4-argon-high',
      );
      assert.equal(
        await page.getByLabel('Creation model', { exact: true }).getAttribute('value'),
        'pi:lm-studio/qwen/local',
      );
      assert.equal(
        await page.getByLabel('Testing model', { exact: true }).getAttribute('value'),
        'agy:gemini-4-argon-high',
      );
      assert.equal(await page.locator('#settings-batch_files').inputValue(), '42');
      assert.equal(
        await page
          .getByLabel('Refresh previously analyzed files on next run', { exact: true })
          .isChecked(),
        true,
      );

      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByRole('button', { name: 'Fix with Agent', exact: true }).first().click();
      await idle(page);
      await page.getByRole('heading', { name: 'Agent Mission Control', exact: true }).waitFor();

      const workflowReq = await page.evaluate(() =>
        window.fixture.requests.find((r) => r.path.endsWith('/workflow')),
      );
      assert.ok(workflowReq, 'Fix with Agent dispatches a workflow request');
      assert.deepEqual(workflowReq.body.models, {
        create: 'pi:lm-studio/qwen/local',
        test: 'agy:gemini-4-argon-high',
        review: configured,
      });
      assert.deepEqual(
        workflowReq.body.confirmed_profiles.sort(),
        [configured, 'agy:gemini-4-argon-high', 'pi:lm-studio/qwen/local'].sort(),
      );
    } finally {
      await close();
    }
  });

  await test('Saved Model Presets, Batch Multi-Finding Fix Queue, and Symbol Blast Radius Explorer', async () => {
    const { page, close } = await pageFor({ trusted: true });
    try {
      // 1. Saved Model Presets in Models page and Command Palette
      await nav(page, 'Settings & Rules');
      await idle(page);
      await page.getByRole('button', { name: 'Model Routing', exact: true }).click();
      await idle(page);
      await page.getByText('Saved model presets', { exact: true }).waitFor();
      await page.getByText('Fast Local / Speed', { exact: true }).waitFor();
      await page.getByText('Deep Audit', { exact: true }).waitFor();

      const presetInput = page.getByLabel('Custom preset name', { exact: true });
      await presetInput.fill('Offline Audit');
      await page.getByRole('button', { name: 'Save preset', exact: true }).click();
      await page.getByText('Offline Audit', { exact: true }).waitFor();

      // Switch preset from Command Palette
      await page.getByRole('button', { name: 'Search files and commands', exact: true }).click();
      const searchInput = page.locator('#search');
      await searchInput.fill('Preset: Deep Audit');
      await page.getByRole('button', { name: 'Preset: Deep Audit', exact: true }).click();

      // 2. Batch Multi-Finding Selection & Action Bar in Issues & Ideas
      await nav(page, 'Issues & Ideas');
      await idle(page);
      const firstCheckbox = page.locator('.result-row input[type="checkbox"]').first();
      await firstCheckbox.check();
      const batchBar = page.getByRole('region', { name: 'Batch issue actions', exact: true });
      await batchBar.waitFor();
      await batchBar.getByRole('button', { name: 'Fix Selected (1)', exact: true }).waitFor();
      await batchBar.getByRole('button', { name: 'Clear', exact: true }).click();

      // 3. Interactive Symbols & Blast Radius Explorer in Code & Diff
      await nav(page, 'Code & Diff');
      await idle(page);
      await page.locator('.explorer-file-btn').first().click();
      await idle(page);
      await page.getByRole('tab', { name: 'Direct Edit', exact: true }).click();
      const symbolDisclosure = page.locator('summary', {
        hasText: 'Symbols, Blast Radius & File Inspection',
      });
      await symbolDisclosure.click();
      const processPill = page
        .getByLabel('File symbols', { exact: true })
        .getByRole('button', { name: /^Process/ });
      await processPill.click();
      await idle(page);
      await page.getByText(/Blast Radius · Process/).waitFor();
      await page.getByText('Calls Process', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Paused analysis can be inspected in Last Run, resumed, or restarted from zero with New Analysis', async () => {
    const { page, close } = await pageFor({
      runStatus: 'paused',
      featuresReady: true,
      resumeCompletes: true,
    });
    try {
      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByText('3 issues · 1 feature ideas', { exact: true }).waitFor();
      await page.getByText('Analysis paused', { exact: true }).waitFor();

      // Navigate to Last Run to inspect progress and coverage
      await page.getByRole('button', { name: 'Last Run', exact: true }).first().click();
      await idle(page);
      await page.getByRole('heading', { name: 'Last Run', exact: true }).waitFor();
      await page.getByText('Run details', { exact: true }).waitFor();

      // Resume the paused analysis directly from Last Run
      await page.getByRole('button', { name: 'Resume Analysis', exact: true }).click();
      await idle(page);
      const resumeControl = await page.evaluate(() =>
        window.fixture.requests.find(
          (r) => r.path.endsWith('/analysis/run/control') && r.body?.action === 'resume',
        ),
      );
      assert.ok(
        resumeControl,
        'Resume Analysis dispatches /analysis/run/control with action=resume',
      );

      // Pause the run again and start a New Analysis from Issues & Ideas, verifying issues & feature ideas clear to 0
      await page.evaluate(() => {
        window.fixture.state.run.status = 'paused';
        window.fixture.state.run.sections.forEach((sec) => {
          sec.status = 'paused';
        });
      });
      await page.getByRole('button', { name: 'Issues & Ideas', exact: true }).first().click();
      await idle(page);
      await page.getByRole('button', { name: 'Refresh', exact: true }).click();
      await idle(page);

      await page.getByRole('button', { name: 'New Analysis', exact: true }).first().click();
      await idle(page);

      const startReq = await page.evaluate(() =>
        window.fixture.requests.find(
          (r) => r.method === 'POST' && r.path.endsWith('/analysis/run'),
        ),
      );
      assert.ok(startReq, 'New Analysis dispatches POST /analysis/run');
      assert.equal(startReq.body.clear_results, true);
      assert.equal(startReq.body.refresh, true);

      await page.getByText('0 issues · 0 feature ideas', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Dedicated Architecture, Features, Last Run, and Model Routing pages render simplified single-focus views', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    try {
      await nav(page, 'Architecture');
      await idle(page);
      await page.getByRole('heading', { name: 'Architecture', exact: true }).waitFor();
      await page.getByText('Quick Entry Files', { exact: true }).waitFor();

      await nav(page, 'Features');
      await idle(page);
      await page.getByText('Project goals & Feature discovery', { exact: true }).waitFor();

      await nav(page, 'Last Run');
      await idle(page);
      await page.getByRole('heading', { name: 'Last Run', exact: true }).waitFor();
      assert.equal(await page.locator('.run-category-list').count(), 0);

      await nav(page, 'Model Routing');
      await idle(page);
      await page.getByRole('heading', { name: 'Models', exact: true }).waitFor();
      await page.getByText('Analysis models', { exact: true }).waitFor();
      await page.getByText('Workflow models', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Agent Mission Control 4-stage pipeline and interactive Terminal UI (TUI) mode', async () => {
    const { page, close } = await pageFor({ trusted: true, featuresReady: true });
    try {
      // 1. Dispatch Fix with Agent from Issues & Ideas and inspect Agent Mission Control
      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByRole('button', { name: 'Fix with Agent', exact: true }).first().click();
      await idle(page);

      await page.getByRole('heading', { name: 'Agent Mission Control', exact: true }).waitFor();
      await page.getByText('Architect & Code Agent', { exact: true }).waitFor();
      await page.getByText('Verification & Test Agent', { exact: true }).waitFor();
      await page.getByText('Clean-Code Critic Agent', { exact: true }).waitFor();
      await page.getByText('Human Precision Gate', { exact: true }).waitFor();

      // Revisit finished Step 3 (Critic Review) via the top clickable stage bar
      await page.getByRole('tab', { name: /Step 3 · Critic Review/ }).click();
      await page.getByText('Critic Agent Verdict', { exact: true }).waitFor();
      assert.equal(
        await page
          .getByLabel('Read-only diff for internal/worker/process.go', { exact: true })
          .count(),
        0,
      );

      // Advance back to Step 4 (Review & Commit) and verify single-action Accept & Commit
      await page.getByRole('button', { name: 'Next: Review & Commit', exact: true }).click();
      await page
        .getByLabel('Read-only diff for internal/worker/process.go', { exact: true })
        .waitFor();
      const applyBtn = page.getByRole('button', { name: 'Accept & Commit', exact: true });
      await applyBtn.waitFor();
      assert.equal(await applyBtn.count(), 1);
      assert.equal(await applyBtn.isEnabled(), true);
      await applyBtn.click();
      await idle(page);
      await page.getByRole('heading', { name: 'Change applied', exact: true }).waitFor();

      // 2. Switch to live Terminal UI (TUI) mode
      await page.getByRole('button', { name: 'Switch to Terminal UI (TUI) mode' }).click();
      const tui = page.getByRole('application', { name: 'Mini-Orca Terminal UI' });
      await tui.waitFor();
      await page.getByText('MINI-ORCA // TUI', { exact: true }).waitFor();
      await page.getByText('4-STAGE AUTONOMOUS CLEAN-CODE PIPELINE', { exact: true }).waitFor();
      await page.getByText('CLEAN-SOFTWARE PRECISION SCORECARD', { exact: true }).waitFor();

      // Switch TUI palette to CRT Green
      await page.getByRole('button', { name: 'CRT Green', exact: true }).click();
      assert.equal(await tui.getAttribute('data-tui-palette'), 'crt');

      // Navigate TUI views via numbered tabs ([1] Overview, [5] Issues)
      await page.getByRole('button', { name: /\[1\] Overview/ }).click();
      await page.getByText('SOURCE FILES', { exact: true }).waitFor();
      await page.getByRole('button', { name: /\[5\] Issues/ }).click();
      await page.getByText('FINDING INSPECTOR', { exact: true }).waitFor();

      // Use orca> command prompt to return to GUI mode
      const cmdPrompt = page.getByLabel('TUI command prompt', { exact: true });
      await cmdPrompt.fill(':gui');
      await cmdPrompt.press('Enter');
      await page.getByRole('heading', { name: 'Issues & Feature Ideas', exact: true }).waitFor();
    } finally {
      await close();
    }
  });
} finally {
  await browser.close();
  server.close();
}
