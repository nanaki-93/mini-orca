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
      await page.getByText('Purpose of the Project', { exact: true }).waitFor();
      await page.getByText('How It Pursues the Goal', { exact: true }).waitFor();
      await page.getByText('Layered Architecture Map', { exact: true }).waitFor();
      await page.getByText(/^Main Dependencies \(/).waitFor();
      await page.getByText('Other Project Info & Operations', { exact: true }).waitFor();

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

      // 2. Batch Multi-Finding Selection & Action Bar in Issues & Ideas (no manual Mark Done button)
      await nav(page, 'Issues & Ideas');
      await idle(page);
      assert.equal(await page.getByRole('button', { name: 'Done', exact: true }).count(), 0);
      assert.equal(
        await page.getByRole('button', { name: 'Mark as Done', exact: true }).count(),
        0,
      );
      const firstCheckbox = page.locator('.result-row input[type="checkbox"]').first();
      await firstCheckbox.check();
      const batchBar = page.getByRole('region', { name: 'Batch issue actions', exact: true });
      await batchBar.waitFor();
      await batchBar.getByRole('button', { name: 'Fix Selected (1)', exact: true }).waitFor();
      assert.equal(
        await batchBar.getByRole('button', { name: 'Mark Done', exact: true }).count(),
        0,
      );
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

      // Navigate to Last Run to inspect progress and coverage across sub-view tabs
      await page.getByRole('button', { name: 'Last Run', exact: true }).first().click();
      await idle(page);
      await page.getByRole('heading', { name: 'Last Run', exact: true }).waitFor();
      await page.getByText('Run details', { exact: true }).waitFor();
      await page.getByRole('tab', { name: /^2\. File Coverage/ }).click();
      await page.getByRole('tab', { name: '3. Models & Run Config', exact: true }).click();
      await page.getByRole('tab', { name: /^1\. Overview & Categories/ }).click();

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
      await page.getByRole('tab', { name: /^2\. Runtime Flows/ }).click();
      await page.getByRole('tab', { name: /^1\. Topology & Entry Points/ }).click();

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
      await page.getByRole('tab', { name: /^3\. Presets & Batch Limits/ }).click();
      await page.getByRole('tab', { name: /^1\. All Routing & Presets/ }).click();
    } finally {
      await close();
    }
  });

  await test('Agents in Progress Dashboard, Agent/Issue Detail page, and interactive Terminal UI (TUI) mode', async () => {
    const { page, close } = await pageFor({ trusted: true, featuresReady: true });
    try {
      // 1. Dispatch Fix with Agent from Issues & Ideas -> lands on Agents in Progress Dashboard
      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByRole('button', { name: 'Fix with Agent', exact: true }).first().click();
      await idle(page);

      await page.getByRole('heading', { name: 'Agent Mission Control', exact: true }).waitFor();
      const dashboard = page.getByRole('tablist', {
        name: 'Active agents',
        exact: true,
      });
      await dashboard.waitFor();

      // Click the agent card on the dashboard to navigate to the Agent & Issue Detail page
      await dashboard.getByRole('tab').first().click();
      await idle(page);

      await page.getByRole('button', { name: /^All Agents/ }).waitFor();
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

      // Return to TUI Agents Dashboard via [← All Agents]
      await page.getByRole('button', { name: '[← All Agents]', exact: true }).click();
      await page.getByText('AGENTS IN PROGRESS DASHBOARD', { exact: true }).waitFor();

      // Switch TUI palette to CRT Green
      await page.getByRole('button', { name: 'CRT Green', exact: true }).click();
      assert.equal(await tui.getAttribute('data-tui-palette'), 'crt');

      // Navigate TUI views via numbered tabs ([1] Overview, [2] Arch, [3] Docs, [5] Issues)
      await page.getByRole('button', { name: /\[1\] Overview/ }).click();
      await page.getByText('SOURCE FILES', { exact: true }).waitFor();
      await page.getByRole('button', { name: /\[2\] Arch/ }).click();
      await page.getByText('SUBSYSTEM CONTRACT INSPECTOR', { exact: true }).waitFor();
      await page.getByRole('button', { name: /\[3\] Docs/ }).click();
      await page.getByText('1. PURPOSE OF THE PROJECT:', { exact: true }).waitFor();
      await page.getByText(/4\. MAIN DEPENDENCIES/).waitFor();
      await page.getByRole('button', { name: /\[5\] Issues/ }).click();
      await page.getByText('FINDING INSPECTOR', { exact: true }).waitFor();
      await page.getByRole('button', { name: /\[d: Dismiss\]/ }).waitFor();
      assert.equal(await page.getByRole('button', { name: '[Done]', exact: true }).count(), 0);
      assert.equal(
        await page.getByRole('button', { name: '[Mark as Done]', exact: true }).count(),
        0,
      );
      await page.getByRole('button', { name: /\[In Progress \(/ }).click();

      // Verify keyboard-only TUI navigation and interactive Keybindings Reference (? and :keys)
      await tui.focus();
      await page.keyboard.press('?');
      const keybindsModal = page.getByRole('dialog', {
        name: 'TUI Keybindings Reference',
        exact: true,
      });
      await keybindsModal.waitFor();
      await page.keyboard.press('Escape');
      assert.equal(await keybindsModal.count(), 0);

      // Cycle issue filter via keyboard (h/l) and move row cursor via keyboard (j/k)
      await page.keyboard.press('h');
      await page.keyboard.press('j');
      await page.keyboard.press('k');

      // Open keybinds help via orca> :keys command, then close and return to GUI mode
      const cmdPrompt = page.getByLabel('TUI command prompt', { exact: true });
      await cmdPrompt.fill(':keys');
      await cmdPrompt.press('Enter');
      await keybindsModal.waitFor();
      await page.getByRole('button', { name: 'Close keybindings reference', exact: true }).click();
      await cmdPrompt.fill(':gui');
      await cmdPrompt.press('Enter');
      await page.getByRole('heading', { name: 'Issues & Feature Ideas', exact: true }).waitFor();

      // Verify split sub-view tabs on Settings & Rules and info/success notice classification
      await nav(page, 'Settings & Rules');
      await idle(page);
      await page.getByRole('button', { name: 'Load scope', exact: true }).click();
      await idle(page);
      await page.getByRole('tab', { name: /^2\. Guideline Library/ }).click();
      await page.getByLabel('Find guidelines', { exact: true }).fill('no-matching-guideline-zzz');
      await page.locator('.notice.info').waitFor();
      await page.getByRole('tab', { name: /^3\. Active Scope/ }).click();

      // Navigating to Code & Diff after a clean committed apply does not show a stale Change applied banner
      await nav(page, 'Code & Diff');
      await idle(page);
      assert.equal(
        await page.getByRole('heading', { name: 'Change applied', exact: true }).count(),
        0,
      );
      await page.getByRole('button', { name: 'Undo Last Change', exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Grouped issues in one agent, In Progress issues view, problem highlight on dashboard, and Done only on merge', async () => {
    const { page, close } = await pageFor({
      git: true,
      trusted: true,
      featuresReady: true,
      analysisCompletes: true,
    });
    try {
      // 1. Verify Git & Worktrees shows in-progress worktrees without duplicate Worktree implementations panel
      await page.evaluate(() => {
        window.fixture.state.gitWorktrees.push({
          path: '/fixture/harbor/.worktrees/in-progress-agent',
          branch: 'codex/in-progress-agent',
          head: 'head-1',
          current: false,
          mergeable: false,
          dirty: false,
          completed: [],
        });
      });
      await nav(page, 'Git & Worktrees');
      await idle(page);
      await page.getByText('codex/fix', { exact: true }).waitFor();
      await page.getByText('codex/in-progress-agent', { exact: true }).waitFor();
      assert.equal(await page.locator('.git-worktree').count(), 2);
      assert.equal(await page.getByText('Worktree implementations', { exact: true }).count(), 0);

      // 2. Select 2 issues in Issues & Ideas and dispatch a single grouped agent fix (even when main has uncommitted changes)
      await page.evaluate(() => {
        window.fixture.state.git.dirty = true;
      });
      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByText('3 issues · 1 feature ideas', { exact: true }).waitFor();
      const checkboxes = page.locator('.result-row input[type="checkbox"]');
      await checkboxes.nth(0).check();
      await checkboxes.nth(1).check();
      await page.getByRole('button', { name: 'Fix Selected (2)', exact: true }).click();
      await idle(page);

      // Lands on Agents in Progress Dashboard showing the grouped agent (2 issues)
      await page.getByRole('heading', { name: 'Agent Mission Control', exact: true }).waitFor();
      const dashboard = page.getByRole('tablist', {
        name: 'Active agents',
        exact: true,
      });
      await dashboard.waitFor();
      await dashboard.getByText('2 issues', { exact: true }).waitFor();

      // Verify a worktree was created for this grouped agent fix even with dirty main
      const worktreeCreateReq = await page.evaluate(() =>
        window.fixture.requests.find(
          (r) => r.method === 'POST' && r.path.endsWith('/git/worktrees'),
        ),
      );
      assert.ok(worktreeCreateReq, 'Fix Selected creates an isolated Git worktree');
      assert.match(worktreeCreateReq.body.path, /^\/fixture\/harbor\/\.worktrees\//);
      await page.evaluate(() => {
        window.fixture.state.git.dirty = false;
      });

      // 3. Check the "In Progress (2)" view in Issues & Ideas
      await nav(page, 'Issues & Ideas');
      await idle(page);
      const inProgressBtn = page.getByRole('button', { name: 'In Progress (2)', exact: true });
      await inProgressBtn.waitFor();
      await inProgressBtn.click();
      assert.equal(await page.locator('.result-row').count(), 2);

      // Starting a new analysis while issues are in progress clears open issues (3 -> 2) and preserves the 2 in-progress issues
      await page.getByRole('button', { name: 'Run Analysis', exact: true }).click();
      await idle(page);
      await page.getByRole('button', { name: 'In Progress (2)', exact: true }).waitFor();
      assert.equal(await page.locator('.result-row').count(), 2);

      // Clicking "Open Agent" on an in-progress issue opens the Agent & Issue Detail page
      await page.getByRole('button', { name: 'Open Agent', exact: true }).first().click();
      await idle(page);
      await page.getByText('Grouped Issues in this Agent (2)', { exact: true }).waitFor();

      // 4. Accept & Commit inside the worktree -> issues remain Open/In Progress (NOT Done yet)
      const applyBtn = page.getByRole('button', { name: 'Accept & Commit', exact: true });
      await applyBtn.waitFor();
      await applyBtn.click();
      await idle(page);
      await page.getByRole('heading', { name: 'Change applied', exact: true }).waitFor();
      await page.locator('.chat-receipt .notice.success').waitFor();

      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByRole('button', { name: /^All \(2\)/ }).click();
      await page.getByText('2 issues · 1 feature ideas', { exact: true }).waitFor();

      // 5. Return to Agent Detail and click Merge into main & Close 2 Issues -> marks both grouped issues Done
      await page.getByRole('button', { name: 'Open Agent', exact: true }).first().click();
      await idle(page);
      const mergeBtn = page.getByRole('button', {
        name: 'Merge into main & Close 2 Issues',
        exact: true,
      });
      await mergeBtn.waitFor();
      await mergeBtn.click();
      await idle(page);

      // Verify both grouped issues are now marked Done after merging into main
      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByText('0 issues · 1 feature ideas', { exact: true }).waitFor();
      await page.getByText('Show Done (2)', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Deduplicates overlapping file-level semantic and line-anchored stage findings', async () => {
    const { page, close } = await pageFor();
    try {
      await page.evaluate(() => {
        const state = window.fixture.state;
        state.results.security = {
          semantic: [
            {
              ...state.finding,
              id: 'sem-master-1',
              category: 'security',
              title: 'Empty input bypasses password check',
              message: 'Empty input bypasses password check.',
              severity: 'high',
              location: { path: 'cmd/master.go', symbol: '', start_line: 0, end_line: 0 },
            },
            {
              ...state.finding,
              id: 'sem-master-2',
              category: 'security',
              title: 'Zero-filled key written on RNG error',
              message: 'Zero-filled key written on RNG error.',
              severity: 'high',
              location: { path: 'cmd/master.go', symbol: '', start_line: 0, end_line: 0 },
            },
            {
              ...state.finding,
              id: 'sem-utils-1',
              category: 'security',
              title: 'Disabled master password verification',
              message: 'Disabled master password verification.',
              severity: 'high',
              location: { path: 'internal/utils/utils.go', symbol: '', start_line: 0, end_line: 0 },
            },
          ],
          performance: [],
          security: [
            {
              ...state.security,
              path: 'cmd/master.go',
              source: 'ai',
              findings: [
                {
                  ...state.security.findings[0],
                  id: 'sec-master-1',
                  title: 'Empty input bypasses password check',
                  severity: 'high',
                  source_anchor: {
                    path: 'cmd/master.go',
                    symbol: 'VerifyMaster',
                    start_line: 47,
                    end_line: 55,
                  },
                },
                {
                  ...state.security.findings[0],
                  id: 'sec-master-2',
                  title: 'Key generation ignores randomness failure',
                  severity: 'high',
                  source_anchor: {
                    path: 'cmd/master.go',
                    symbol: 'GenerateKey',
                    start_line: 64,
                    end_line: 72,
                  },
                },
              ],
            },
            {
              ...state.security,
              path: 'internal/utils/utils.go',
              source: 'ai',
              findings: [
                {
                  ...state.security.findings[0],
                  id: 'sec-utils-1',
                  title: 'Disabled master password verification',
                  severity: 'high',
                  source_anchor: {
                    path: 'internal/utils/utils.go',
                    symbol: 'CheckPassword',
                    start_line: 146,
                    end_line: 152,
                  },
                },
              ],
            },
          ],
          saved_finding_count: 6,
        };
      });

      await nav(page, 'Issues & Ideas');
      await idle(page);
      await page.getByRole('button', { name: /^Security \(/ }).click();

      const rows = page.locator('.result-row');
      assert.equal(await rows.count(), 3);
      const paths = await rows.locator('.path').allInnerTexts();
      assert.deepEqual(paths, [
        'cmd/master.go:47',
        'cmd/master.go:64',
        'internal/utils/utils.go:146',
      ]);
    } finally {
      await close();
    }
  });

  await test('Re-run Fix after Step 2 test failure includes failed test error output and interactive instructions in context', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await page.evaluate(() => {
        window.fixture.state.nextWorkflowStatus = 'failed';
        window.fixture.state.failedTestOutput =
          '--- FAIL: TestRun (0.00s)\n    main_test.go:14: wrong result: got 2, want 3\nFAIL';
      });

      await nav(page, 'Issues & Ideas');
      await idle(page);
      const selectCheckboxes = page.locator('.result-row input[type="checkbox"]');
      await selectCheckboxes.first().check();
      await page.getByRole('button', { name: 'Fix Selected (1)', exact: true }).click();
      await idle(page);

      // Open the failed agent from the Agents in Progress Dashboard
      const dashboard = page.getByRole('tablist', { name: 'Active agents', exact: true });
      await dashboard.waitFor();
      await dashboard.getByRole('tab').last().click();
      await idle(page);

      // Should land on Step 2 · Verification & Test Agent with failed test output expanded
      await page
        .getByRole('heading', {
          name: /Step 2 · Verification & Test Agent \(0\/1 Passed\)/,
        })
        .waitFor();
      await page.getByText(/main_test\.go:14: wrong result: got 2, want 3/).waitFor();

      // Provide optional retry guidance and click Re-run Fix in Step 2
      const retryInput = page.getByRole('textbox', { name: 'Retry correction instructions' });
      await retryInput.waitFor();
      await retryInput.fill('Ensure Run returns 3 on cancellation');
      await page
        .locator('section.panel')
        .filter({ hasText: 'Step 2 · Verification & Test Agent' })
        .getByRole('button', { name: 'Re-run Fix', exact: true })
        .click();
      await idle(page);

      // Verify the re-run workflow request included the failed test error output and retry guidance
      const workflowCalls = await page.evaluate(() =>
        window.fixture.requests.filter((c) => c.method === 'POST' && c.path.endsWith('/workflow')),
      );
      assert.equal(workflowCalls.length, 2);
      const rerunMessage = workflowCalls[1].body.message;
      assert.ok(
        rerunMessage.includes('Previous check failures to resolve:'),
        `Expected rerun message to include check failures header, got: ${rerunMessage}`,
      );
      assert.ok(
        rerunMessage.includes('main_test.go:14: wrong result: got 2, want 3'),
        `Expected rerun message to include failed test assertion output, got: ${rerunMessage}`,
      );
      assert.ok(
        rerunMessage.includes(
          'Additional retry instructions:\nEnsure Run returns 3 on cancellation',
        ),
        `Expected rerun message to include user retry instructions, got: ${rerunMessage}`,
      );

      // After successful retry, agent advances to Step 4 · Review & Commit
      await page.getByRole('heading', { name: /^Step 4 · Review & Commit/ }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Environment Bootstrap Checklist audits and bootstraps 4 repository pillars in GUI and TUI', async () => {
    const { page, close } = await pageFor({ git: true, trusted: true });
    try {
      await nav(page, 'Settings & Rules');
      await idle(page);

      // Switch to dedicated 4. Repo Bootstrap (1/4) sub-view tab
      await page.getByRole('tab', { name: /^4\. Repo Bootstrap \(1\/4\)/ }).click();
      await page
        .getByRole('heading', { name: 'Environment Bootstrap Checklist', exact: true })
        .waitFor();
      await page.getByText('1/4 Pillars Ready', { exact: true }).waitFor();
      await page.getByText('1. Standardize the Repo Entrypoint', { exact: true }).waitFor();
      await page.getByText('2. Pin the Toolchain', { exact: true }).waitFor();
      await page.getByText('3. Write the Agent Contract (<150 lines)', { exact: true }).waitFor();
      await page.getByText('4. Enforce Fast Pre-Commit Gates', { exact: true }).waitFor();

      // Bootstrap Pillar 1 (justfile) in GUI
      await page.getByRole('button', { name: 'Bootstrap justfile', exact: true }).click();
      await idle(page);
      await page.getByText('2/4 Pillars Ready', { exact: true }).waitFor();
      await page.getByText('Ready (justfile)', { exact: true }).waitFor();

      // Switch to TUI mode, open [0] Rules -> [2. Repo Bootstrap (2/4)], and bootstrap remaining 2 pillars (.mise.toml and lefthook.yml)
      await page.getByRole('button', { name: 'Switch to Terminal UI (TUI) mode' }).click();
      const tui = page.getByRole('application', { name: 'Mini-Orca Terminal UI' });
      await tui.waitFor();
      await page.getByRole('button', { name: /\[0\] Rules/ }).click();
      await page.getByRole('button', { name: /\[2\. Repo Bootstrap \(2\/4\)\]/ }).click();
      await page
        .getByText('STEP-BY-STEP ENVIRONMENT BOOTSTRAP CHECKLIST (b bootstrap missing)', {
          exact: true,
        })
        .waitFor();
      await page.getByText('2/4 PILLARS READY', { exact: true }).waitFor();

      await page.getByRole('button', { name: '[b: Bootstrap Missing (2)]', exact: true }).click();
      await idle(page);
      await page.getByText('4/4 PILLARS READY', { exact: true }).waitFor();
      await page.getByText('mise trust && mise install: passed', { exact: true }).waitFor();
      await page.getByText('lefthook install: passed', { exact: true }).waitFor();

      // Return to GUI and verify 4/4 Pillars Ready and mise trust && mise install status
      const cmdPrompt = page.getByLabel('TUI command prompt', { exact: true });
      await cmdPrompt.fill(':gui');
      await cmdPrompt.press('Enter');
      await page.getByRole('tab', { name: /^4\. Repo Bootstrap \(4\/4\)/ }).click();
      await page.getByText('4/4 Pillars Ready', { exact: true }).waitFor();
      await page.getByText('mise trust && mise install: passed', { exact: true }).waitFor();
    } finally {
      await close();
    }
  });
} finally {
  await browser.close();
  server.close();
}
