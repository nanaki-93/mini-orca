import { strict as assert } from 'node:assert';

export async function testGitWorkflows({ test, pageFor, nav, idle, layout }) {
  const prepare = async (page) => {
    await nav(page, 'Bugs');
    await page.locator('.result-row').first().click();
    await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
    return page.getByRole('dialog', { name: 'Prepare fix workspace', exact: true });
  };
  const review = async (page) => {
    await idle(page);
    await page.getByRole('button', { name: 'Review next file', exact: true }).click();
    await page.getByRole('button', { name: 'Apply & Commit', exact: true }).press('Enter');
    return page.getByRole('dialog', { name: 'Apply & Commit', exact: true });
  };
  const posts = (page) =>
    page.evaluate(() => window.fixture.requests.filter((request) => request.method === 'POST'));
  await test('Git sidebar branch and management navigation are passive', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await idle(page);
      const branch = page.getByRole('button', {
        name: 'Current branch: main. Open Git / Worktrees',
        exact: true,
      });
      const name = await page.locator('.project-label strong').boundingBox();
      const location = await branch.boundingBox();
      assert.ok(location.y >= name.y + name.height - 2, 'branch is below the project name');
      const before = await posts(page);
      await branch.click();
      await page.getByRole('heading', { name: 'Worktrees', exact: true }).waitFor();
      assert.deepEqual(await posts(page), before);
      assert.equal(
        await page
          .getByRole('complementary', { name: 'Application' })
          .getByText('New workflow', { exact: true })
          .count(),
        0,
      );
      for (const [width, height, large] of [
        [1440, 1000, false],
        [900, 640, true],
      ]) {
        await page.setViewportSize({ width, height });
        if (large) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await layout(page, `git-management-${width}`);
      }
    } finally {
      await close();
    }
  });
  await test('Git Apply and Commit edits the message and cancellation writes nothing', async () => {
    const { page, close } = await pageFor({ git: true, trusted: true });
    try {
      let dialog = await prepare(page);
      const before = await posts(page);
      await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
      assert.deepEqual(await posts(page), before);
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await dialog
        .getByRole('button', { name: 'Prepare in current workspace', exact: true })
        .click();
      dialog = await review(page);
      assert.ok(
        (await dialog.getByLabel('Commit message', { exact: true }).inputValue()).length > 0,
      );
      await dialog
        .getByLabel('Commit message', { exact: true })
        .fill('fix: propagate cancellation');
      await layout(page, 'git-commit-review');
      await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
      assert.equal(
        (await posts(page)).some((r) => r.path.endsWith('/apply')),
        false,
      );
      await page.getByRole('button', { name: 'Apply & Commit', exact: true }).click();
      await dialog
        .getByLabel('Commit message', { exact: true })
        .fill('fix: propagate cancellation');
      await dialog.getByRole('button', { name: 'Apply & Commit', exact: true }).click();
      await idle(page);
      await page.getByText('Committed commit- on main', { exact: true }).waitFor();
      assert.equal(
        await page
          .getByText('Source or guidance changed. Refresh the findings and start a new fix.', {
            exact: true,
          })
          .count(),
        0,
      );
      const apply = (await posts(page)).find((r) => r.path.endsWith('/apply'));
      assert.equal(apply.body.commit_message, 'fix: propagate cancellation');
      assert.equal(apply.body.head, 'head-main');
      assert.equal(apply.body.branch, 'main');
    } finally {
      await close();
    }
  });
  await test('Git failed commit retries without another apply or review', async () => {
    const { page, close } = await pageFor({ git: true, trusted: true, gitCommitFailure: true });
    try {
      const workspace = await prepare(page);
      await workspace
        .getByRole('button', { name: 'Prepare in current workspace', exact: true })
        .click();
      const dialog = await review(page);
      await dialog.getByRole('button', { name: 'Apply & Commit', exact: true }).click();
      await idle(page);
      await page.getByText('Files applied. Commit failed.', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Retry commit', exact: true }).click();
      const retry = page.getByRole('dialog', { name: 'Retry commit', exact: true });
      await retry
        .getByLabel('Commit message', { exact: true })
        .fill('fix: retry after Git identity setup');
      await retry.getByRole('button', { name: 'Retry commit', exact: true }).click();
      await idle(page);
      const requests = await posts(page);
      assert.equal(requests.filter((r) => r.path.endsWith('/apply')).length, 1);
      assert.equal(requests.filter((r) => r.path.endsWith('/commit')).length, 1);
      assert.equal(requests.filter((r) => r.path.endsWith('/review')).length, 1);
    } finally {
      await close();
    }
  });
  await test('Git Prepare fix creates a worktree and recaptures the task there', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      const dialog = await prepare(page);
      await dialog.getByRole('radio', { name: 'New Git worktree', exact: true }).check();
      assert.match(
        await dialog.getByLabel('New branch name', { exact: true }).inputValue(),
        /^codex\//,
      );
      await dialog.getByLabel('New branch name', { exact: true }).fill('codex/cancel-fix');
      await dialog
        .getByLabel('Worktree path', { exact: true })
        .fill('/fixture/harbor/.worktrees/cancel-fix');
      await layout(page, 'git-prepare-worktree');
      await dialog.getByRole('button', { name: 'Create worktree & prepare', exact: true }).click();
      await page.getByRole('button', { name: 'Review next file', exact: true }).waitFor();
      await idle(page);
      const requests = await posts(page);
      const creation = requests.findIndex((r) => r.path.endsWith('/git/worktrees'));
      const capture = requests.findIndex(
        (r, i) => i > creation && r.path === '/api/projects/current/changes',
      );
      assert.ok(creation >= 0 && capture > creation);
      assert.equal(requests[creation].body.target_branch, 'codex/cancel-fix');
      assert.equal(requests[creation].body.path, '/fixture/harbor/.worktrees/cancel-fix');
      assert.equal(requests[capture].body.project_id, 'project-worktree-codex/cancel-fix');
      assert.equal(requests[capture].query.worktree_path, '/fixture/harbor/.worktrees/cancel-fix');
      assert.equal(
        requests.slice(creation).some((r) => r.path === '/api/projects/restore'),
        false,
      );
      assert.equal(
        await page.evaluate(() => localStorage.getItem('mini-orca:last-project')),
        '/fixture/harbor',
      );
      await page
        .getByRole('button', {
          name: 'Current branch: main. Open Git / Worktrees',
          exact: true,
        })
        .waitFor();
    } finally {
      await close();
    }
  });
  await test('Git worktrees keep parallel implementations separate through cancel and Apply', async () => {
    const { page, close } = await pageFor({ git: true, workflowRunning: true });
    try {
      for (const name of ['first', 'second']) {
        const dialog = await prepare(page);
        await dialog.getByRole('radio', { name: 'New Git worktree', exact: true }).check();
        assert.match(
          await dialog.getByLabel('Worktree path', { exact: true }).inputValue(),
          /^\/fixture\/harbor\/\.worktrees\//,
        );
        await dialog.getByLabel('New branch name', { exact: true }).fill(`codex/${name}`);
        await dialog
          .getByLabel('Worktree path', { exact: true })
          .fill(`/fixture/harbor/.worktrees/${name}`);
        await dialog
          .getByRole('button', { name: 'Create worktree & prepare', exact: true })
          .click();
        await page.getByRole('button', { name: 'Cancel workflow', exact: true }).waitFor();
        await idle(page);
      }
      const running = await page.evaluate(() =>
        Object.values(window.fixture.worktreeStates).map(
          (state) => Object.values(state.changes)[0].workflow.status,
        ),
      );
      assert.deepEqual(running, ['running', 'running']);
      await page.getByRole('button', { name: 'Cancel workflow', exact: true }).click();
      await idle(page);
      assert.deepEqual(
        await page.evaluate(() =>
          Object.values(window.fixture.worktreeStates).map(
            (state) => Object.values(state.changes)[0].workflow.status,
          ),
        ),
        ['running', 'canceled'],
      );
      await page.getByRole('button', { name: 'All implementations', exact: true }).click();
      await page.getByRole('button', { name: 'View implementation', exact: true }).first().click();
      await page.evaluate(() =>
        window.fixture.worktreeStates['/fixture/harbor/.worktrees/first'].completeWorkflow(),
      );
      await page.getByRole('button', { name: 'Review next file', exact: true }).waitFor();
      await page.setViewportSize({ width: 900, height: 640 });
      await page.getByRole('button', { name: 'Larger text', exact: true }).click();
      await layout(page, 'parallel-worktree-review-large-text');
      const commit = await review(page);
      await commit.getByRole('button', { name: 'Apply & Commit', exact: true }).click();
      await page.getByText(/Committed .* on codex\/first/).waitFor();
      const mutations = (await posts(page)).filter((r) => /\/(review|apply)$/.test(r.path));
      assert.equal(mutations.length, 2);
      assert.ok(
        mutations.every((r) => r.query.worktree_path === '/fixture/harbor/.worktrees/first'),
      );
      await page
        .getByRole('button', { name: 'Current branch: main. Open Git / Worktrees', exact: true })
        .waitFor();
      assert.equal(await page.evaluate(() => window.fixture.state.changeReceipt), null);
      assert.equal(
        await page.evaluate(
          () => window.fixture.worktreeStates['/fixture/harbor/.worktrees/second'].changeReceipt,
        ),
        null,
      );
    } finally {
      await close();
    }
  });
  await test('Git feature conversations choose isolated worktrees and preserve test options', async () => {
    for (const markdown of [false, true]) {
      const { page, close } = await pageFor({ git: true });
      try {
        await nav(page, 'Chat');
        await page
          .getByLabel('Files to change', { exact: true })
          .fill(markdown ? 'README.md' : 'internal/feature.go\ninternal/feature_test.go');
        await page
          .getByLabel('Change request', { exact: true })
          .fill('Implement the requested feature in this captured scope.');
        if (markdown)
          await page
            .getByRole('checkbox', { name: 'Run project tests after generation', exact: true })
            .uncheck();
        const before = (await posts(page)).length;
        await page.getByRole('button', { name: 'Generate changes', exact: true }).click();
        const dialog = page.getByRole('dialog', { name: 'Prepare feature workspace', exact: true });
        await dialog.waitFor();
        await idle(page);
        assert.equal((await posts(page)).length, before);
        await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
        assert.equal((await posts(page)).length, before);
        await page.getByRole('button', { name: 'Generate changes', exact: true }).click();
        await dialog.getByRole('radio', { name: 'New Git worktree', exact: true }).check();
        await dialog.getByLabel('New branch name', { exact: true }).fill('codex/feature');
        await dialog
          .getByLabel('Worktree path', { exact: true })
          .fill('/fixture/harbor/.worktrees/feature');
        await dialog
          .getByRole('button', { name: 'Create worktree & prepare', exact: true })
          .click();
        await page.waitForFunction(
          (suffix) => window.fixture.requests.some((r) => r.path.endsWith(suffix)),
          markdown ? '/checks' : '/workflow',
        );
        await idle(page);
        const requests = await posts(page);
        const capture = requests.findLast((r) => r.path === '/api/projects/current/changes');
        assert.equal(capture.body.kind, 'feature');
        assert.equal(capture.query.worktree_path, '/fixture/harbor/.worktrees/feature');
        if (markdown)
          assert.equal(requests.findLast((r) => r.path.endsWith('/checks')).body.run_tests, false);
        await page
          .getByRole('button', { name: 'Current branch: main. Open Git / Worktrees', exact: true })
          .waitFor();
        assert.equal(await page.getByRole('alert').count(), 0);
      } finally {
        await close();
      }
    }
  });
  await test('Git branch switch requires confirmation and refreshes the sidebar', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.getByLabel('Switch to branch', { exact: true }).selectOption('feature/other');
      await page.getByRole('button', { name: 'Switch branch', exact: true }).click();
      const dialog = page.getByRole('dialog', { name: 'Switch to feature/other?', exact: true });
      assert.equal(
        (await posts(page)).some((r) => r.path.endsWith('/git/switch')),
        false,
      );
      await dialog.getByRole('button', { name: 'Switch branch', exact: true }).click();
      await idle(page);
      await page
        .getByRole('button', {
          name: 'Current branch: feature/other. Open Git / Worktrees',
          exact: true,
        })
        .waitFor();
    } finally {
      await close();
    }
  });
  await test('Git can reopen main in its existing checkout without switching a checked-out branch', async () => {
    const { page, close } = await pageFor({ git: true, gitBranch: 'codex/fix' });
    try {
      await page.evaluate(() => {
        window.fixture.state.gitWorktrees[1].branch = 'main';
        window.fixture.state.gitWorktrees[1].path = '/fixture/main';
      });
      await nav(page, 'Git / Worktrees');
      const tree = page
        .locator('.git-worktree')
        .filter({ has: page.getByRole('heading', { name: 'main', exact: true }) });
      await tree.getByRole('button', { name: 'Open project here', exact: true }).click();
      await idle(page);
      await page
        .getByRole('button', { name: 'Current branch: main. Open Git / Worktrees', exact: true })
        .waitFor();
      const requests = await posts(page);
      assert.equal(requests.at(-1).path, '/api/projects/restore');
      assert.equal(requests.at(-1).body.project_path, '/fixture/main');
      assert.equal(
        requests.some((r) => r.path.endsWith('/git/switch')),
        false,
      );
    } finally {
      await close();
    }
  });
  for (const conflict of [false, true])
    await test(`Git merge ${conflict ? 'conflicts and abort remain visible' : 'previews before merging'}`, async () => {
      const { page, close } = await pageFor({ git: true, gitMergeConflict: conflict });
      try {
        await nav(page, 'Git / Worktrees');
        await page.getByRole('button', { name: 'View implementations', exact: true }).click();
        await idle(page);
        await nav(page, 'Git / Worktrees');
        await page.getByRole('button', { name: 'Merge into Current Branch', exact: true }).click();
        await idle(page);
        const preview = page.getByRole('dialog', { name: 'Review worktree merge', exact: true });
        await preview.getByText('Incoming commits', { exact: true }).waitFor();
        assert.equal(
          (await posts(page)).some((r) => r.path.endsWith('/git/merge')),
          false,
        );
        await layout(page, `git-merge-preview-${conflict}`);
        await preview
          .getByRole('button', { name: 'Merge into current branch', exact: true })
          .click();
        await page
          .getByRole('dialog', { name: 'Merge codex/fix into main?', exact: true })
          .getByRole('button', { name: 'Merge into current branch', exact: true })
          .click();
        await idle(page);
        if (conflict) {
          assert.equal(await page.locator('.git-worktree').count(), 2);
          await page
            .locator('.git-conflicts')
            .getByText('internal/worker/process.go', { exact: true })
            .waitFor();
          await layout(page, 'git-merge-conflicts');
          await page.getByRole('button', { name: 'Abort merge', exact: true }).click();
          await page
            .getByRole('dialog', { name: 'Abort this merge?', exact: true })
            .getByRole('button', { name: 'Abort merge', exact: true })
            .click();
          await idle(page);
          assert.equal(await page.locator('.git-conflicts').count(), 0);
          await page.getByText('Git operation: aborted', { exact: true }).waitFor();
          assert.equal(await page.locator('.git-worktree').count(), 2);
        } else {
          await page.getByText('Git operation: merged', { exact: true }).waitFor();
          await page
            .getByText('Removed worktree: /fixture/harbor-fix. The branch is kept.', {
              exact: true,
            })
            .waitFor();
          assert.equal(await page.locator('.git-worktree').count(), 1);
          assert.equal(
            await page.getByRole('option', { name: 'codex/fix', exact: true }).isEnabled(),
            true,
          );
        }
        await nav(page, 'Changes');
        assert.equal(
          await page.getByRole('button', { name: 'View implementation', exact: true }).count(),
          conflict ? 1 : 0,
        );
      } finally {
        await close();
      }
    });
  await test('Git cleanup failure keeps the merged worktree and its implementation visible', async () => {
    const { page, close } = await pageFor({ git: true, gitCleanupFailure: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'View implementations', exact: true }).click();
      await idle(page);
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'Merge into Current Branch', exact: true }).click();
      await idle(page);
      await page
        .getByRole('dialog', { name: 'Review worktree merge', exact: true })
        .getByRole('button', { name: 'Merge into current branch', exact: true })
        .click();
      const confirmation = page.getByRole('dialog', {
        name: 'Merge codex/fix into main?',
        exact: true,
      });
      await confirmation
        .getByText(/source worktree directory and its local task history will be deleted/)
        .waitFor();
      await confirmation
        .getByRole('button', { name: 'Merge into current branch', exact: true })
        .click();
      await idle(page);
      await page.getByText('Git operation: merged', { exact: true }).waitFor();
      await page
        .getByText(/Merge completed, but worktree .* was not removed: checkout is locked/)
        .waitFor();
      assert.equal(await page.locator('.git-worktree').count(), 2);
      await nav(page, 'Changes');
      assert.equal(
        await page.getByRole('button', { name: 'View implementation', exact: true }).count(),
        1,
      );
    } finally {
      await close();
    }
  });
  await test('Git refresh closes removed implementations after a lost merge response', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'View implementations', exact: true }).click();
      await idle(page);
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'Merge into Current Branch', exact: true }).click();
      await idle(page);
      await page.evaluate(() => {
        const original = window.go.main.Desktop.Request;
        window.go.main.Desktop.Request = async (...args) => {
          const result = await original(...args);
          if (args[2] === '/api/projects/current/git/merge') throw new Error('Response lost');
          return result;
        };
      });
      await page
        .getByRole('dialog', { name: 'Review worktree merge', exact: true })
        .getByRole('button', { name: 'Merge into current branch', exact: true })
        .click();
      await page
        .getByRole('dialog', { name: 'Merge codex/fix into main?', exact: true })
        .getByRole('button', { name: 'Merge into current branch', exact: true })
        .click();
      await idle(page);
      await page.getByText('Git outcome unknown.', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Refresh project', exact: true }).click();
      await idle(page);
      assert.equal(await page.locator('.git-worktree').count(), 1);
      await nav(page, 'Changes');
      assert.equal(
        await page.getByRole('button', { name: 'View implementation', exact: true }).count(),
        0,
      );
    } finally {
      await close();
    }
  });
  await test('Git dirty state blocks switch and merge while creation errors stop preparation', async () => {
    const { page, close } = await pageFor({ git: true, gitDirty: true, gitCreateFailure: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.getByLabel('Switch to branch', { exact: true }).selectOption('feature/other');
      assert.equal(
        await page.getByRole('button', { name: 'Switch branch', exact: true }).isDisabled(),
        true,
      );
      assert.equal(
        await page
          .getByRole('button', { name: 'Merge into Current Branch', exact: true })
          .isDisabled(),
        true,
      );
      const dialog = await prepare(page);
      await dialog.getByRole('radio', { name: 'New Git worktree', exact: true }).check();
      await dialog.getByRole('button', { name: 'Create worktree & prepare', exact: true }).click();
      await idle(page);
      await page.getByText('Branch already exists.', { exact: true }).waitFor();
      assert.equal(
        (await posts(page)).some((r) => r.path === '/api/projects/current/changes'),
        false,
      );
    } finally {
      await close();
    }
  });
  await test('Git lost mutation response exposes recovery before another action', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.evaluate(() => {
        window.fixture.failures['/api/projects/current/git/switch'] = 'transport';
      });
      await page.getByLabel('Switch to branch', { exact: true }).selectOption('feature/other');
      await page.getByRole('button', { name: 'Switch branch', exact: true }).click();
      await page
        .getByRole('dialog', { name: 'Switch to feature/other?', exact: true })
        .getByRole('button', { name: 'Switch branch', exact: true })
        .click();
      await idle(page);
      await page.getByText('Git outcome unknown.', { exact: true }).waitFor();
      assert.equal(
        await page.getByRole('button', { name: 'Switch branch', exact: true }).isDisabled(),
        true,
      );
      await page.getByRole('button', { name: 'Refresh project', exact: true }).click();
      await idle(page);
      await page.getByText('Git outcome unknown.', { exact: true }).waitFor({ state: 'hidden' });
      assert.equal(
        await page.getByRole('button', { name: 'Switch branch', exact: true }).isEnabled(),
        true,
      );
    } finally {
      await close();
    }
  });
  await test('Git late merge preview cannot reopen after navigation', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/git/merge-preview';
      });
      await page.getByRole('button', { name: 'Merge into Current Branch', exact: true }).click();
      await nav(page, 'Overview');
      await page.evaluate(() => window.fixture.release());
      await idle(page);
      assert.equal(
        await page.getByRole('dialog', { name: 'Review worktree merge', exact: true }).count(),
        0,
      );
    } finally {
      await close();
    }
  });
}
