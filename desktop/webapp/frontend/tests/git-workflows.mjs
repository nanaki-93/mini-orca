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
    return page.locator('.fix-review');
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
      await page.evaluate(() => {
        const tree = window.fixture.state.gitWorktrees[1];
        window.fixture.state.gitWorktrees.push({
          ...tree,
          path: '/fixture/long-title',
          branch: `codex/${'long-worktree-title-'.repeat(8)}`,
        });
      });
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
        const rows = page.locator('.git-worktree');
        assert.equal(await rows.count(), 2);
        let columns;
        for (const row of await rows.all()) {
          const buttons = row.getByRole('button');
          assert.deepEqual(await buttons.allTextContents(), [
            'View implementation',
            'Preview diff',
            'Discard change',
            'Merge into current branch',
          ]);
          assert.equal(await row.locator('p, ul, .badge').count(), 0);
          const bounds = await Promise.all(
            (await buttons.all()).map((button) => button.boundingBox()),
          );
          for (const box of bounds) {
            assert.ok(Math.abs(box.y - bounds[0].y) < 1, 'buttons share a top edge');
            assert.ok(Math.abs(box.height - bounds[0].height) < 1, 'buttons share a height');
          }
          if (columns) bounds.forEach((box, i) => assert.ok(Math.abs(box.x - columns[i]) < 1));
          columns = bounds.map((box) => box.x);
        }
        const buttons = rows.first().getByRole('button');
        await buttons.nth(0).focus();
        for (let i = 1; i < 4; i++) {
          await page.keyboard.press('Tab');
          assert.equal(
            await buttons.nth(i).evaluate((button) => button === document.activeElement),
            true,
          );
        }
      }
    } finally {
      await close();
    }
  });
  await test('Git lists only daemon-approved merge candidates and preserves hidden implementations', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'View implementation', exact: true }).click();
      await idle(page);
      await page.evaluate(() => {
        const source = window.fixture.state.gitWorktrees[1];
        for (const state of [
          'dirty',
          'locked',
          'prunable',
          'detached',
          'already-merged',
          'unrelated',
          'unavailable',
        ])
          window.fixture.state.gitWorktrees.push({
            ...source,
            path: `/fixture/${state}`,
            branch: `codex/${state}`,
            mergeable: false,
          });
      });
      await nav(page, 'Git / Worktrees');
      assert.deepEqual(await page.locator('.git-worktree h3').allTextContents(), ['codex/fix']);
      await page.evaluate(() => {
        window.fixture.state.gitWorktrees[1].mergeable = false;
      });
      await page.getByRole('button', { name: 'Refresh Git', exact: true }).click();
      await idle(page);
      assert.equal(await page.locator('.git-worktree').count(), 0);
      await page.getByText('No worktrees ready to merge', { exact: true }).waitFor();
      await nav(page, 'Changes');
      assert.equal(
        await page.getByRole('button', { name: 'View implementation', exact: true }).count(),
        1,
      );
      assert.equal(
        (await posts(page)).some((request) =>
          /\/git\/(merge|discard-worktree)$/.test(request.path),
        ),
        false,
      );
    } finally {
      await close();
    }
  });
  await test('Git discard confirms removal and keeps the current project unchanged', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'View implementation', exact: true }).click();
      await idle(page);
      await nav(page, 'Git / Worktrees');
      const before = await posts(page);
      const project = await page.evaluate(() => window.fixture.state.project);
      await page.getByRole('button', { name: 'Discard change', exact: true }).click();
      const dialog = page.getByRole('dialog', { name: 'Discard codex/fix?', exact: true });
      await dialog.getByText(/local task history without merging/).waitFor();
      assert.deepEqual(await posts(page), before);
      await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
      assert.deepEqual(await posts(page), before);
      await page.getByRole('button', { name: 'Discard change', exact: true }).click();
      await layout(page, 'git-discard-confirmation');
      await dialog.getByRole('button', { name: 'Discard change', exact: true }).press('Enter');
      await idle(page);
      await page.getByText('Git operation: discarded', { exact: true }).waitFor();
      assert.equal(await page.locator('.git-worktree').count(), 0);
      assert.deepEqual(await page.evaluate(() => window.fixture.state.project), project);
      assert.equal(await page.evaluate(() => window.fixture.state.git.head), 'head-main');
      const mutation = (await posts(page)).filter((request) =>
        request.path.endsWith('/git/discard-worktree'),
      );
      assert.equal(mutation.length, 1);
      assert.deepEqual(mutation[0].body, {
        project_id: project.project_id,
        project_revision: project.project_revision,
        head: 'head-main',
        branch: 'main',
        path: '/fixture/harbor-fix',
        source_head: 'head-fix',
        source_branch: 'codex/fix',
        confirm: true,
        confirm_execution: true,
      });
      assert.equal(
        (await posts(page)).some((request) => request.path.endsWith('/git/merge')),
        false,
      );
      await nav(page, 'Changes');
      assert.equal(
        await page.getByRole('button', { name: 'View implementation', exact: true }).count(),
        0,
      );
    } finally {
      await close();
    }
  });
  for (const failure of ['locked', 'stale', 'transport', 'lost response'])
    await test(`Git discard handles ${failure} without claiming an unverified removal`, async () => {
      const { page, close } = await pageFor({ git: true, gitDiscardFailure: failure === 'locked' });
      try {
        await nav(page, 'Git / Worktrees');
        await page.getByRole('button', { name: 'View implementation', exact: true }).click();
        await idle(page);
        await nav(page, 'Git / Worktrees');
        await page.getByRole('button', { name: 'Discard change', exact: true }).click();
        const dialog = page.getByRole('dialog', { name: 'Discard codex/fix?', exact: true });
        await dialog.waitFor();
        await page.evaluate((failure) => {
          if (failure === 'stale') window.fixture.state.gitWorktrees[1].head = 'head-new';
          if (failure === 'transport')
            window.fixture.failures['/api/projects/current/git/discard-worktree'] = 'transport';
          if (failure === 'lost response') {
            const original = window.go.main.Desktop.Request;
            window.go.main.Desktop.Request = async (...args) => {
              const result = await original(...args);
              if (args[2] === '/api/projects/current/git/discard-worktree')
                throw new Error('Response lost');
              return result;
            };
          }
        }, failure);
        await dialog.getByRole('button', { name: 'Discard change', exact: true }).click();
        await idle(page);
        assert.equal(await page.getByText('Git operation: discarded', { exact: true }).count(), 0);
        if (failure === 'locked')
          await page.getByText('The checkout is locked.', { exact: true }).waitFor();
        if (failure === 'stale')
          await page.getByText('Worktree changed; refresh Git status.', { exact: true }).waitFor();
        if (failure === 'transport' || failure === 'lost response') {
          await page.getByText('Git outcome unknown.', { exact: true }).waitFor();
          assert.equal(
            await page.getByRole('button', { name: 'Discard change', exact: true }).isDisabled(),
            true,
          );
          await page.getByRole('button', { name: 'Refresh project', exact: true }).click();
          await idle(page);
          await page
            .getByText('Git outcome unknown.', { exact: true })
            .waitFor({ state: 'hidden' });
        }
        assert.equal(
          await page.locator('.git-worktree').count(),
          failure === 'lost response' ? 0 : 1,
        );
        await nav(page, 'Changes');
        assert.equal(
          await page.getByRole('button', { name: 'View implementation', exact: true }).count(),
          failure === 'lost response' ? 0 : 1,
        );
      } finally {
        await close();
      }
    });
  await test('Git Apply and Commit executes directly with an optional editable message', async () => {
    const { page, close } = await pageFor({ git: true, trusted: true });
    try {
      const preparation = await prepare(page);
      const before = await posts(page);
      await preparation.getByRole('button', { name: 'Cancel', exact: true }).click();
      assert.deepEqual(await posts(page), before);
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await preparation
        .getByRole('button', { name: 'Prepare in current workspace', exact: true })
        .click();
      const proposal = await review(page);
      assert.equal(await proposal.getByLabel('Commit message', { exact: true }).isVisible(), false);
      await proposal.locator('summary').filter({ hasText: 'Commit message' }).click();
      const message = proposal.getByLabel('Commit message', { exact: true });
      assert.ok((await message.inputValue()).length > 0);
      await message.fill('');
      assert.equal(
        await proposal.getByRole('button', { name: 'Apply & Commit', exact: true }).isDisabled(),
        true,
      );
      await message.fill('fix: propagate cancellation');
      await layout(page, 'git-commit-review');
      assert.equal(
        (await posts(page)).some((r) => r.path.endsWith('/apply')),
        false,
      );
      await proposal.getByRole('button', { name: 'Apply & Commit', exact: true }).press('Enter');
      await idle(page);
      assert.equal(await page.getByRole('dialog').count(), 0);
      await page.getByText('Committed commit- on main', { exact: true }).waitFor();
      const applied = (await posts(page)).filter((r) => r.path.endsWith('/apply'));
      assert.equal(applied.length, 1);
      assert.equal(applied[0].body.commit_message, 'fix: propagate cancellation');
      assert.equal(applied[0].body.head, 'head-main');
      assert.equal(applied[0].body.branch, 'main');
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
      const title = await page.evaluate(() => Object.values(window.fixture.state.changes)[0].title);
      await dialog.getByRole('button', { name: 'Apply & Commit', exact: true }).click();
      await idle(page);
      await page.getByText('Files applied. Commit failed.', { exact: true }).waitFor();
      assert.equal(await page.getByRole('dialog').count(), 0);
      const retry = page.locator('.chat-outcome');
      await retry.locator('summary').filter({ hasText: 'Commit message' }).click();
      await retry
        .getByLabel('Commit message', { exact: true })
        .fill('fix: retry after Git identity setup');
      await retry.getByRole('button', { name: 'Retry commit', exact: true }).click();
      await idle(page);
      const requests = await posts(page);
      assert.equal(requests.filter((r) => r.path.endsWith('/apply')).length, 1);
      assert.equal(requests.find((r) => r.path.endsWith('/apply')).body.commit_message, title);
      assert.equal(requests.filter((r) => r.path.endsWith('/commit')).length, 1);
      assert.equal(
        requests.find((r) => r.path.endsWith('/commit')).body.commit_message,
        'fix: retry after Git identity setup',
      );
      assert.equal(await page.getByRole('dialog').count(), 0);
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
      assert.deepEqual(requests[capture].body.finding_targets, [
        {
          project_id: 'project-1',
          kind: 'semantic',
          id: 'finding-1',
          path: 'internal/worker/process.go',
        },
      ]);
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
  for (const conflict of [false, true])
    await test(`Git merge ${conflict ? 'conflicts and abort remain visible' : 'runs directly with an optional preview'}`, async () => {
      const { page, close } = await pageFor({ git: true, gitMergeConflict: conflict });
      try {
        await nav(page, 'Git / Worktrees');
        await page.getByRole('button', { name: 'View implementation', exact: true }).click();
        await idle(page);
        await nav(page, 'Git / Worktrees');
        await page.getByRole('button', { name: 'Preview diff', exact: true }).click();
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
        await idle(page);
        assert.equal(await page.getByRole('dialog').count(), 0);
        const mutations = (await posts(page)).filter((r) => r.path.endsWith('/git/merge'));
        assert.equal(mutations.length, 1);
        assert.equal(mutations[0].body.source_head, 'head-fix');
        assert.equal(mutations[0].body.confirm_execution, true);
        if (conflict) {
          assert.equal(await page.locator('.git-worktree').count(), 0);
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
          assert.equal(await page.locator('.git-worktree').count(), 1);
        } else {
          await page.getByText('Git operation: merged', { exact: true }).waitFor();
          await page
            .getByText('Removed worktree: /fixture/harbor-fix. The branch is kept.', {
              exact: true,
            })
            .waitFor();
          assert.equal(await page.locator('.git-worktree').count(), 0);
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
  for (const conflict of [false, true])
    await test(`Git merge finding completion is refreshed and scoped (conflict=${conflict})`, async () => {
      const gitFindingCompletions = [
        {
          project_id: 'project-1',
          kind: 'semantic',
          id: 'finding-1',
          path: 'internal/worker/process.go',
          commit_oid: 'fix-commit',
        },
        {
          project_id: 'project-1',
          kind: 'performance',
          id: 'perf-1',
          path: 'internal/worker/process.go',
          commit_oid: 'fix-commit',
        },
        {
          project_id: 'project-1',
          kind: 'security',
          source: 'deterministic',
          id: 'sec-1',
          path: 'internal/worker/process.go',
          commit_oid: 'fix-commit',
        },
      ];
      const { page, close } = await pageFor({
        git: true,
        gitMergeConflict: conflict,
        gitFindingCompletions,
      });
      try {
        await page.evaluate(() => {
          const state = window.fixture.state;
          state.results.bugs = {
            semantic: [
              state.finding,
              { ...state.finding, id: 'unrelated', title: 'Separate concern' },
            ],
          };
          state.security.findings.push({
            ...state.security.findings[0],
            id: 'unrelated',
            title: 'Separate security concern',
          });
        });
        await nav(page, 'Bugs');
        assert.equal(
          await page.locator('.result-row').getByText('Done', { exact: true }).count(),
          0,
        );
        await nav(page, 'Git / Worktrees');
        await page.getByRole('button', { name: 'Merge into current branch', exact: true }).click();
        await idle(page);
        assert.equal(await page.getByRole('dialog').count(), 0);
        for (const category of ['Bugs', 'Performance', 'Security']) {
          await nav(page, category);
          assert.equal(
            await page.locator('.result-row').getByText('Done', { exact: true }).count(),
            conflict ? 0 : 1,
          );
          await page.locator('.result-row').first().click();
          if (!conflict) {
            await page
              .getByLabel('Finding state', { exact: true })
              .getByText('Done', { exact: true })
              .waitFor();
            if (category === 'Security')
              await page
                .getByLabel('Finding state', { exact: true })
                .getByText('unverified', { exact: true })
                .waitFor();
            await layout(page, `merged-finding-${category}`);
          }
        }
        const requests = await posts(page);
        assert.equal(requests.filter((request) => request.path.includes('/findings/')).length, 0);
        // A later findings read, independent of the merge receipt, retains Done.
        await nav(page, 'Overview');
        await nav(page, 'Bugs');
        assert.equal(
          await page.locator('.result-row').getByText('Done', { exact: true }).count(),
          conflict ? 0 : 1,
        );
        if (!conflict) {
          await page.setViewportSize({ width: 900, height: 640 });
          await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          await layout(page, 'merged-findings-list-large-text');
        }
      } finally {
        await close();
      }
    });
  await test('Git cleanup failure hides merged worktrees but keeps their implementation accessible', async () => {
    const { page, close } = await pageFor({ git: true, gitCleanupFailure: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'View implementation', exact: true }).click();
      await idle(page);
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'Merge into current branch', exact: true }).click();
      await idle(page);
      assert.equal(await page.getByRole('dialog').count(), 0);
      await page.getByText('Git operation: merged', { exact: true }).waitFor();
      await page
        .getByText(/Merge completed, but worktree .* was not removed: checkout is locked/)
        .waitFor();
      assert.equal(await page.locator('.git-worktree').count(), 0);
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
      await page.getByRole('button', { name: 'View implementation', exact: true }).click();
      await idle(page);
      await nav(page, 'Git / Worktrees');
      await page.getByRole('button', { name: 'Preview diff', exact: true }).click();
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
      await idle(page);
      await page.getByText('Git outcome unknown.', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Refresh project', exact: true }).click();
      await idle(page);
      assert.equal(await page.locator('.git-worktree').count(), 0);
      await nav(page, 'Changes');
      assert.equal(
        await page.getByRole('button', { name: 'View implementation', exact: true }).count(),
        0,
      );
    } finally {
      await close();
    }
  });
  await test('Git dirty state hides merge candidates while creation errors stop preparation', async () => {
    const { page, close } = await pageFor({ git: true, gitDirty: true, gitCreateFailure: true });
    try {
      await nav(page, 'Git / Worktrees');
      assert.equal(await page.locator('.git-worktree').count(), 0);
      await page.getByText('No worktrees ready to merge', { exact: true }).waitFor();
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
  await test('Git direct merge rejects stale previews and late navigation without mutation', async () => {
    for (const scenario of ['navigate', 'head', 'branch', 'failure']) {
      const { page, close } = await pageFor({ git: true });
      try {
        await nav(page, 'Git / Worktrees');
        await page.evaluate((scenario) => {
          if (scenario === 'navigate')
            window.fixture.hold = '/api/projects/current/git/merge-preview';
          else if (scenario === 'failure')
            window.fixture.failures['/api/projects/current/git/merge-preview'] = 409;
          else {
            const original = window.go.main.Desktop.Request;
            window.go.main.Desktop.Request = async (...args) => {
              const response = await original(...args);
              if (args[2] === '/api/projects/current/git/merge-preview') {
                const preview = JSON.parse(response.body);
                preview.source[scenario] = 'changed';
                return { ...response, body: JSON.stringify(preview) };
              }
              return response;
            };
          }
        }, scenario);
        await page.getByRole('button', { name: 'Merge into current branch', exact: true }).click();
        if (scenario === 'navigate') {
          await page.waitForFunction(() =>
            window.fixture.requests.some((r) => r.path.endsWith('/git/merge-preview')),
          );
          await nav(page, 'Overview');
          await page.evaluate(() => window.fixture.release());
        }
        await idle(page);
        assert.equal(await page.getByRole('dialog').count(), 0);
        const requests = await posts(page);
        assert.equal(
          requests.some(
            (r) => r.path.endsWith('/git/merge') || r.path.endsWith('/execution-trust'),
          ),
          false,
        );
      } finally {
        await close();
      }
    }
  });
  await test('Git direct merge suppresses duplicate clicks while capturing the current heads', async () => {
    const { page, close } = await pageFor({ git: true });
    try {
      await nav(page, 'Git / Worktrees');
      await page.evaluate(() => {
        window.fixture.hold = '/api/projects/current/git/merge-preview';
      });
      const merge = page.getByRole('button', { name: 'Merge into current branch', exact: true });
      await merge.evaluate((button) => {
        button.click();
        button.click();
      });
      await page.waitForFunction(() =>
        window.fixture.requests.some((r) => r.path.endsWith('/git/merge-preview')),
      );
      assert.equal(await merge.isDisabled(), true);
      assert.equal(await page.getByRole('dialog').count(), 0);
      assert.equal(
        await page
          .locator('.busy-strip')
          .getByRole('button', { name: 'Cancel', exact: true })
          .count(),
        0,
      );
      await page.evaluate(() => window.fixture.release());
      await idle(page);
      const requests = await posts(page);
      assert.equal(requests.filter((r) => r.path.endsWith('/git/merge-preview')).length, 1);
      assert.equal(requests.filter((r) => r.path.endsWith('/git/merge')).length, 1);
      const mutation = requests.find((r) => r.path.endsWith('/git/merge'));
      assert.equal(mutation.body.head, 'head-main');
      assert.equal(mutation.body.source_head, 'head-fix');
      assert.equal(mutation.body.confirm_execution, true);
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
      await page.getByRole('button', { name: 'Preview diff', exact: true }).click();
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
