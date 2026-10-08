import { strict as assert } from 'node:assert';

async function wheel(page, region, delta) {
  const bounds = await region.boundingBox();
  await page.mouse.move(bounds.x + 8, bounds.y + bounds.height / 2);
  await page.mouse.wheel(0, delta);
  await page.evaluate(
    () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
  );
}

async function shellBounds(page) {
  return page.evaluate(() => ({
    header: document.querySelector('.topbar').getBoundingClientRect().top,
    footer: document.querySelector('.statusbar').getBoundingClientRect().bottom,
    viewport: innerHeight,
    scroll: scrollY,
    height: document.scrollingElement.scrollHeight,
  }));
}

export async function testScrolling({ test, pageFor, nav, idle, layout }) {
  await test('Content scrolls while the footer stays at the window edge', async () => {
    const { page, close } = await pageFor();
    try {
      await nav(page, 'Instructions');
      await page.getByRole('button', { name: 'Load scope', exact: true }).click();
      await idle(page);
      const main = page.locator('#main');
      for (const [width, height, larger] of [
        [1440, 1000, false],
        [900, 640, true],
      ]) {
        await page.setViewportSize({ width, height });
        if (larger) {
          await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          await page.getByRole('button', { name: 'Porcelain theme' }).click();
        }
        await main.evaluate((element) => element.scrollTo(0, 0));
        const initial = await shellBounds(page);
        assert.equal(initial.header, 0);
        assert.equal(initial.footer, height);
        assert.equal(initial.height, height);
        assert.equal(initial.scroll, 0);
        assert.equal(
          await main.evaluate((element) => element.scrollHeight > element.clientHeight),
          true,
          'The instructions remain accessible through the content pane',
        );
        await wheel(page, main, 200);
        await page.waitForFunction(() => document.querySelector('#main').scrollTop > 0);
        await wheel(page, main, 10000);
        await page.waitForFunction(() => {
          const main = document.querySelector('#main');
          return main.scrollTop + main.clientHeight >= main.scrollHeight - 1;
        });
        await wheel(page, main, 500);
        assert.deepEqual(await shellBounds(page), initial, 'The footer stays attached at the end');
        await layout(page, `scroll-footer-bottom-${width}`);
        await wheel(page, main, -10000);
        await page.waitForFunction(() => document.querySelector('#main').scrollTop === 0);
        await wheel(page, main, -500);
        assert.deepEqual(await shellBounds(page), initial, 'The shell stays attached at the start');
      }
    } finally {
      await close();
    }
  });

  await test('Content, sidebar and terminal boundaries do not scroll their host', async () => {
    for (const [route, selector] of [
      ['Instructions', '#main'],
      ['Summary', '.sidebar'],
      ['Terminal', '.terminal-container'],
    ]) {
      const { page, close } = await pageFor();
      try {
        await nav(page, route);
        await idle(page);
        // A scrollable ancestor makes boundary chaining observable without native rubber-banding.
        await page.evaluate(() => {
          const host = document.createElement('div');
          host.id = 'scroll-host';
          host.style.cssText = 'position:fixed;inset:0;overflow:auto';
          const spacer = document.createElement('div');
          spacer.style.height = '100vh';
          const shell = document.querySelector('.app-window');
          shell.before(host);
          host.append(shell, spacer);
        });
        const host = page.locator('#scroll-host');
        assert.equal(
          await host.evaluate((element) => element.scrollHeight > element.clientHeight),
          true,
        );
        const region = page.locator(selector);
        await region.evaluate((element) => element.scrollTo(0, element.scrollHeight));
        const initial = await shellBounds(page);
        await wheel(page, region, 500);
        assert.equal(
          await host.evaluate((element) => element.scrollTop),
          0,
          `${route}: scrolling past the pane must not move the enclosing window`,
        );
        assert.deepEqual(await shellBounds(page), initial);
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      } finally {
        await close();
      }
    }
  });
}
