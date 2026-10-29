// The core job: review every changed file, approve this exact version and
// apply it, with the review gate tracking each step.

import { expect, test } from './support/daemon';
import { agentSettings, rejectedTask } from './support/fixtures';

test('reviews, approves and applies a task through the gate', async ({ page, daemon }) => {
  await page.goto('/#/tasks/chg-1');
  const gate = page.getByRole('complementary', { name: 'Review gate' });
  await expect(gate.getByText('0 of 3')).toBeVisible();
  await expect(gate.getByRole('button', { name: /apply changes/i })).toBeDisabled();
  // An optional check failure is a warning, not a blocker.
  await expect(gate.getByText('1 optional failed')).toBeVisible();

  await page.getByRole('button', { name: 'Review files' }).click();
  await expect(page).toHaveURL(/#\/tasks\/chg-1\/files/);
  for (let viewed = 1; viewed <= 3; viewed++) {
    await page.keyboard.press('v');
    await expect(page.getByText(`${viewed}/3 viewed`)).toBeVisible();
  }
  expect(daemon.mutations('/review-file').map(request => request.body?.path)).toEqual([
    'internal/api/handlers/webui.go',
    'internal/api/handlers/webui_test.go',
    'internal/api/handlers/mime.go',
  ]);

  await page.getByRole('tab', { name: /conversation/i }).click();
  await gate.getByRole('button', { name: 'Approve this version' }).click();
  await expect(page.getByText('Approved this exact version')).toBeVisible();

  await gate.getByRole('button', { name: 'Apply & commit' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('commits it on main');
  // The running task in another package keeps going.
  await expect(dialog).not.toContainText('running task');
  await dialog.getByRole('button', { name: 'Apply & commit' }).click();
  await expect(page.getByText('Applied and committed.')).toBeVisible();

  const [apply] = daemon.mutations('/apply');
  expect(apply.body).toMatchObject({ project_id: 'p1', project_revision: 'r42', revision: 3, hash: 'sha256:9f2c1a7be4', confirm: true, branch: 'main' });
});

test('cancelling the Apply confirmation writes nothing', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, reviewed_files: session.changes.map(change => change.path), reviewed_hash: session.hash });
  await page.goto('/#/tasks/chg-1');
  await page.getByRole('complementary', { name: 'Review gate' }).getByRole('button', { name: /apply/i }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByRole('alertdialog')).toBeHidden();
  expect(daemon.mutations('/apply')).toEqual([]);
});

test('compares the split and unified diff layouts', async ({ page }) => {
  await page.goto('/#/tasks/chg-1/files');
  const diff = page.getByRole('region', { name: /Diff of internal\/api\/handlers\/webui.go/ });
  await expect(diff).toContainText('strings.Contains(name, "/")');
  await page.keyboard.press('s');
  await expect(page.getByRole('button', { name: /unified view/i })).toBeVisible();
});

test('overrides the reviewers with a reason, then approves with notes', async ({ page, daemon }) => {
  daemon.sessions.set('chg-1', rejectedTask());
  await page.goto('/#/tasks/chg-1');
  const gate = page.getByRole('complementary', { name: 'Review gate' });
  await expect(gate.getByText('Changes requested')).toBeVisible();
  await expect(gate.getByRole('button', { name: /^Approve/ })).toHaveCount(0);

  // Advisory notes are shown apart from blocking findings.
  await page.getByRole('button', { name: /Correctness/ }).click();
  await expect(page.getByText('Consider a table-driven test.')).toBeVisible();
  await expect(page.getByText('1 blocking finding')).toBeVisible();

  await gate.getByRole('button', { name: 'Override reviewers…' }).click();
  const dialog = page.getByRole('dialog', { name: 'Override the reviewers?' });
  await expect(dialog).toContainText('Move the MIME table into its own package.');
  await expect(dialog.getByRole('button', { name: 'Override reviewers' })).toBeDisabled();
  await dialog.getByLabel('Override reason').fill('The MIME table is private to this handler.');
  await dialog.getByRole('button', { name: 'Override reviewers' }).click();
  await expect(dialog).toBeHidden();
  await expect(gate.getByText('Overridden by you')).toBeVisible();
  await expect(page.getByText('The MIME table is private to this handler.')).toBeVisible();
  expect(daemon.mutations('/review-override')[0].body).toMatchObject({ revision: 3, hash: 'sha256:9f2c1a7be4', workflow_id: 'wf-1', reason: 'The MIME table is private to this handler.' });

  await gate.getByRole('button', { name: 'Add notes' }).click();
  await gate.getByLabel('Approval notes').fill('Revisit the package split next sprint.');
  await gate.getByRole('button', { name: 'Approve with notes' }).click();
  await expect(page.getByText('Approved this exact version')).toBeVisible();
  await expect(gate.getByText('Revisit the package split next sprint.')).toBeVisible();
  expect(daemon.mutations('/review')[0].body).toMatchObject({ hash: 'sha256:9f2c1a7be4', notes: 'Revisit the package split next sprint.' });
  await expect(gate.getByRole('button', { name: 'Apply & commit' })).toBeEnabled();
});

test('checks from an earlier project tree must run again before approval', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, reviewed_files: session.changes.map(change => change.path), checks_outdated: true });
  await page.goto('/#/tasks/chg-1');
  const gate = page.getByRole('complementary', { name: 'Review gate' });
  await expect(gate.getByText('Outdated')).toBeVisible();
  await expect(gate.getByText('The project changed since checks ran. Run them again before approval.')).toBeVisible();
  await expect(gate.getByRole('button', { name: /^Approve/ })).toHaveCount(0);
  await page.getByRole('tab', { name: /checks/i }).click();
  await expect(page.getByText('These checks ran on an earlier version of the project')).toBeVisible();
});

test('a stale task names the file that changed and is re-analyzed', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, freshness: 'stale', stale_reason: 'go.mod changed since this proposal was made' });
  await page.goto('/#/tasks/chg-1');
  await expect(page.getByText('Out of date: go.mod changed since this proposal was made. Re-analyze it against the current files, or delete it.')).toBeVisible();
  await expect(page.getByRole('button', { name: /run agents again/i })).toHaveCount(0);
  await page.locator('header.task-header').getByRole('button', { name: 'Re-analyze' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('A model checks whether the issue is still present in the current files.');
  await dialog.getByRole('button', { name: 'Re-analyze' }).click();

  await expect(page.getByText('Still valid: the task now matches the current files. Run the agents when ready.')).toBeVisible();
  expect(daemon.mutations('/reanalyze')[0].body).toMatchObject({ project_id: 'p1', revision: session.revision, hash: session.hash, confirm_remote_provider: false });
  await expect(page.locator('header.task-header').getByRole('button', { name: 'Run agents again' })).toHaveCount(0);
  await expect(page.locator('header.task-header').getByRole('button', { name: 'Start agents' })).toBeEnabled();
  await expect(page.getByText('Out of date:')).toHaveCount(0);
});

test('a stale task that is no longer valid is deleted', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, freshness: 'stale', stale_reason: 'internal/api/handlers/webui.go changed since this proposal was made' });
  daemon.reanalysisOutcome = 'not_valid';
  await page.goto('/#/tasks/chg-1');
  await page.locator('header.task-header').getByRole('button', { name: 'Re-analyze' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Re-analyze' }).click();

  await expect(page.getByText(`No longer valid: “${session.title}” was deleted.`)).toBeVisible();
  await expect(page).toHaveURL(/#\/tasks$/);
  await expect(page.getByRole('link', { name: new RegExp(session.title) })).toHaveCount(0);
});

test('Apply names the running tasks whose files it changes', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, reviewed_files: session.changes.map(change => change.path), reviewed_hash: session.hash });
  daemon.sessions.set('chg-9', { ...session, id: 'chg-9', title: 'Serve fonts with a content type', targets: [{ path: 'internal/api/handlers/fonts.go', hash: '', exists: false }], workflow: { ...session.workflow!, id: 'wf-9', status: 'running', input: undefined } });
  await page.goto('/#/tasks/chg-1');
  await page.getByRole('complementary', { name: 'Review gate' }).getByRole('button', { name: /apply/i }).click();
  await expect(page.getByRole('alertdialog')).toContainText('It stops 1 running task whose files it changes: “Serve fonts with a content type”.');
});

test('line comments become the brief of the next revision', async ({ page, daemon }) => {
  await page.goto('/#/tasks/chg-1/files');
  const diff = page.getByRole('region', { name: /Diff of internal\/api\/handlers\/webui.go/ });
  await diff.getByRole('row').filter({ hasText: 'strings.Contains(name, "/")' }).hover();
  await diff.getByRole('button', { name: 'Comment on line 42' }).click();
  await diff.getByRole('textbox', { name: 'Comment on line 42' }).fill('Also reject backslashes.');
  await diff.locator('.comment-form').getByRole('button', { name: /^Comment/ }).click();
  await expect(diff.getByText('Also reject backslashes.')).toBeVisible();
  expect(daemon.mutations('/comments')[0].body).toMatchObject({ path: 'internal/api/handlers/webui.go', side: 'new', line: 42, text: 'Also reject backslashes.', revision: 3 });
  await expect(page.getByLabel('1 open comments')).toBeVisible();

  await page.getByRole('button', { name: 'Revise with 1 comment' }).click();
  await expect(page.getByText('Proposal updated. Review the new diff.')).toBeVisible();
  expect(daemon.mutations('/messages')[0].body).toMatchObject({ include_comments: true, message: '' });
  await expect(page.getByRole('button', { name: /Revise with/ })).toHaveCount(0);
});

test('a revision that takes long shows its time and can be stopped', async ({ page }) => {
  let sent = 0;
  // The agent never answers this revision: only Stop ends the wait.
  await page.route(url => url.pathname.endsWith('/changes/chg-1/messages'), () => {
    sent++;
  });
  await page.goto('/#/tasks/chg-1');
  const revise = page.getByRole('region', { name: 'Ask for a revision' });
  await revise.getByLabel('Revision request').fill('Also reject backslashes.');
  await revise.getByRole('button', { name: /^Revise/ }).click();
  const progress = revise.getByRole('status');
  await expect(progress).toContainText(/Agent working · 0:0\d/);
  await expect(revise.getByRole('button', { name: /^Revise/ })).toBeDisabled();

  await progress.getByRole('button', { name: 'Stop' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Stop revision' }).click();
  await expect(page.getByText('Revision stopped.')).toBeVisible();
  await expect(progress).toHaveCount(0);
  await expect(revise.getByLabel('Revision request')).toHaveValue('Also reject backslashes.');
  expect(sent).toBe(1);
});

test('a task revised in the conversation runs the agents again on that version', async ({ page, daemon }) => {
  await page.goto('/#/tasks/chg-1');
  const header = page.locator('header.task-header');
  // A run is always on offer, beside the review step.
  await expect(header.getByRole('button', { name: 'Review files' })).toBeVisible();
  await expect(header.getByRole('button', { name: 'Run agents again' })).toBeEnabled();

  const revise = page.getByRole('region', { name: 'Ask for a revision' });
  await revise.getByLabel('Revision request').fill('Also reject backslashes.');
  await revise.getByRole('button', { name: /^Revise/ }).click();
  await expect(page.getByText('Proposal updated. Review the new diff.')).toBeVisible();
  const run = page.getByRole('region', { name: 'Agent run' });
  await expect(run.getByText('The proposal changed after this run')).toBeVisible();
  // A revision after the run is not a failure, and the old run no longer counts.
  await expect(header.getByText('Revised')).toBeVisible();
  const gate = page.getByRole('complementary', { name: 'Review gate' });
  await expect(gate.getByText('Not run on this version')).toBeVisible();
  await expect(gate.getByText('Run after the agents')).toBeVisible();

  await run.getByRole('button', { name: 'Run agents again' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog.getByRole('heading', { name: 'Run the agents again?' })).toBeVisible();
  await expect(dialog.getByRole('radio', { name: /Check and review this version/ })).toBeChecked();
  await dialog.getByRole('button', { name: 'Run agents' }).click();
  await expect(header.getByRole('button', { name: 'Stop agents' })).toBeVisible();
  expect(daemon.mutations('/workflow')[0].body).toMatchObject({ resume_from: 'checks', revision: 4, hash: 'sha256:9f2c1a7be4-r4', auto_repair: true });
  // The kept tests are only run: the test stage says so.
  await expect(run.getByText('Run the checks')).toBeVisible();
});

test('a changed creation model makes the agents write the change again', async ({ page, daemon }) => {
  daemon.serve('/api/projects/current/agent-settings', () => agentSettings(1, { creation: 'code', testing: 'code', review: 'analyze' }, null));
  await page.goto('/#/tasks/chg-1');
  await page.locator('header.task-header').getByRole('button', { name: 'Run agents again' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog.getByRole('radio', { name: /Check and review this version/ })).toBeDisabled();
  await expect(dialog.getByRole('radio', { name: /Write new tests/ })).toBeDisabled();
  await expect(dialog.getByText('The creation model changed since the last run, so the agents write the change again.').first()).toBeVisible();
  await expect(dialog.getByRole('radio', { name: /Write the change again/ })).toBeChecked();
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(dialog).toBeHidden();
  expect(daemon.mutations('/workflow')).toEqual([]);
});

test('line comments go to the agents only with a start that writes code', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, comments: [{ id: 'cm-1', path: 'internal/api/handlers/webui.go', side: 'new', line: 42, excerpt: 'if name == "" || strings.Contains(name, "/") {', text: 'Prefer path.Clean.', hash: session.hash, created_at: session.updated_at }] });
  await page.goto('/#/tasks/chg-1');
  await page.locator('header.task-header').getByRole('button', { name: 'Run agents again' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('Your 1 line comment stays open: this start writes no code.');
  await dialog.getByRole('radio', { name: /Write new tests/ }).check();
  await expect(dialog).toContainText('Your 1 line comment goes to the agents as a repair brief.');
  await dialog.getByRole('button', { name: 'Run agents' }).click();
  await expect(page.locator('header.task-header').getByRole('button', { name: 'Stop agents' })).toBeVisible();
  expect(daemon.mutations('/workflow')[0].body).toMatchObject({ resume_from: 'test', include_comments: true });
});

test('a comment can be deleted before it is sent', async ({ page, daemon }) => {
  const session = daemon.sessions.get('chg-1')!;
  daemon.sessions.set('chg-1', { ...session, comments: [{ id: 'cm-1', path: 'internal/api/handlers/webui.go', side: 'new', line: 42, excerpt: 'if name == "" || strings.Contains(name, "/") {', text: 'Prefer path.Clean.', hash: session.hash, created_at: session.updated_at }] });
  await page.goto('/#/tasks/chg-1/files');
  await expect(page.getByText('Prefer path.Clean.')).toBeVisible();
  await page.getByRole('button', { name: 'Delete comment' }).click();
  await expect(page.getByText('Prefer path.Clean.')).toBeHidden();
  expect(daemon.mutations('/comments/delete')[0].body).toMatchObject({ id: 'cm-1' });
});

test('a page from an earlier daemon launch is told to reload', async ({ page, daemon }) => {
  daemon.expectedConsole.push(/status of 401/);
  await page.goto('/#/tasks/chg-1/files');
  await page.evaluate(() => document.querySelector('meta[name="mini-orca-write-token"]')?.setAttribute('content', 'previous-launch'));
  await page.keyboard.press('v');
  await expect(page.getByText("Reload the Web UI to get this daemon's write token.")).toBeVisible();
  expect(daemon.sessions.get('chg-1')!.reviewed_files).toEqual([]);
});
