// Feed to resume-run.mjs. Catalog records and all platform operations are fixtures.
const { beginFixture, endFixture, captureEvidence } = await import(path.join(dir, 'scripts/native-fixtures.mjs'));
const evidenceContext = { page, cdp, shot, fs, path, dir };
const result = { recordedAt: new Date().toISOString(), cases: [], fixture: 'In-memory catalog rows; disposable Electron IPC; no product writes.' };
const desktopOnly = process.env.SETTINGS_NATIVE_SECTION === 'desktop';
const resultFile = path.join(dir, desktopOnly ? 'desktop-semantics-results.json' : 'layout-desktop-results.json');
await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
await beginFixture(page, navigate, queryClient);
try {
  await page.evaluate(async () => {
    const state = window.__nativeAcceptanceFixture;
    const base = { id: '11111111-1111-4111-8111-111111111111', workspace_id: state.workspace.id, created_at: '2026-10-14T00:00:00Z', updated_at: '2026-10-14T00:00:00Z' };
    const labels = { labels: [{ ...base, name: 'Native acceptance label', resource_type: 'issue', color: '#336699' }], total: 1 };
    const properties = { properties: [{ ...base, name: 'Native acceptance property', type: 'select', config: {}, position: 1, archived: false }], total: 1 };
    const actions = { quick_actions: [{ ...base, name: 'Native acceptance action', description: 'A fixture for catalog geometry', assignee_type: 'agent', assignee_id: 'fixture-agent', prompt: 'Fixture', visibility: 'public', status: 'active', last_used_at: null, use_count: 0, created_by_id: 'fixture-user', target_name: 'Fixture reviewer', target_public: true, target_missing: false }], total: 1 };
    state.mock('listLabels', async () => labels);
    state.mock('listProperties', async () => properties);
    state.mock('listQuickActions', async () => actions);
    await state.seed(['labels', state.workspace.id, 'list', 'issue'], labels);
    await state.seed(['properties', state.workspace.id, 'list', true], properties);
    await state.seed(['quick-actions', state.workspace.id, 'list', true], actions);
  });
  const catalogs = [
    ['labels', '标签', 'Native acceptance label 的操作'],
    ['properties', '属性', 'Native acceptance property 的操作'],
    ['quick-actions', '快捷操作', '打开"Native acceptance action"的操作菜单'],
  ];
  for (const [width, height, zoom] of desktopOnly ? [] : [[1440, 1000, 1], [900, 900, 1], [900, 1000, 2]]) {
    await control('/viewport', { width, height, zoom });
    for (let sidebarPass = 0; sidebarPass < 2; sidebarPass++) {
      if (sidebarPass) {
        const trigger = page.locator('[data-sidebar="trigger"]:visible').first();
        await trigger.click();
        await page.waitForTimeout(350);
        if (width === 900) {
          await expect(page.getByRole('dialog')).toBeVisible();
          result.cases.push({ id: 'sidebar-modal', cssWidth: width / zoom, underlyingCatalogIntentionallyInert: true });
          await page.keyboard.press('Escape');
          await expect(page.getByRole('dialog')).toBeHidden();
        }
      }
      for (const [tab, title, actionName] of catalogs) {
        await navigate(tab);
        const panel = page.getByRole('tabpanel', { name: title, exact: true });
        const action = panel.getByRole('button', { name: actionName, exact: true });
        await expect(action).toBeVisible();
        // Scroll only vertically: horizontal auto-scroll would conceal F03.
        const geometry = await action.evaluate(element => {
          for (let parent = element.parentElement; parent; parent = parent.parentElement) {
            if (/(auto|scroll)/.test(getComputedStyle(parent).overflowY) && parent.scrollHeight > parent.clientHeight) {
              const item = element.getBoundingClientRect(), box = parent.getBoundingClientRect();
              if (item.top < box.top || item.bottom > box.bottom) parent.scrollTop += item.y + item.height / 2 - box.y - box.height / 2;
            }
          }
          const box = element.getBoundingClientRect(), card = element.closest('.overflow-hidden');
          const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
          return { cssWidth: innerWidth, cssHeight: innerHeight, x: box.x, right: box.right, y: box.y, bottom: box.bottom, width: box.width, height: box.height, hit: !!hit && (element === hit || element.contains(hit)), cardWidth: card?.clientWidth, cardScrollWidth: card?.scrollWidth, cardScrollLeft: card?.scrollLeft };
        });
        expect(geometry.x).toBeGreaterThanOrEqual(0);
        expect(geometry.right).toBeLessThanOrEqual(geometry.cssWidth);
        expect(geometry.y).toBeGreaterThanOrEqual(0);
        expect(geometry.bottom).toBeLessThanOrEqual(geometry.cssHeight);
        expect(geometry.hit).toBe(true);
        expect(geometry.cardScrollWidth).toBeLessThanOrEqual(geometry.cardWidth + 1);
        expect(geometry.cardScrollLeft).toBe(0);
        await page.mouse.click(geometry.x + geometry.width / 2, geometry.y + geometry.height / 2);
        await expect(page.getByRole('menu')).toBeVisible();
        await page.keyboard.press('Escape');
        await expect(page.getByRole('menu')).toBeHidden();
        await action.focus();
        await page.keyboard.press('ArrowDown');
        await expect(page.getByRole('menu')).toBeVisible();
        const evidence = await captureEvidence(evidenceContext, `layout-${tab}-${width / zoom}-${sidebarPass}`);
        await page.keyboard.press('Escape');
        await expect(page.getByRole('menu')).toBeHidden();
        result.cases.push({ id: `F03-${tab}`, sidebarPass, geometry, pointer: true, keyboard: true, evidence });
        await fs.writeFile(resultFile, JSON.stringify(result, null, 2));
      }
    }
    if (width === 1440) await page.locator('[data-sidebar="trigger"]:visible').first().click();
  }
  await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
  for (const [tab, role, minimum] of [['members', 'combobox', 3], ['daemon', 'switch', 2], ['behavior', 'combobox', 1]]) {
    await navigate(tab);
    const tree = await cdp.send('Accessibility.getFullAXTree');
    const nodes = tree.nodes.filter(node => !node.ignored && node.role?.value === role);
    expect(nodes.length).toBeGreaterThanOrEqual(minimum);
    expect(nodes.filter(node => !node.name?.value?.trim())).toHaveLength(0);
    result.cases.push({ id: `F05-${tab}`, names: nodes.map(node => node.name.value), evidence: await captureEvidence(evidenceContext, `desktop-names-${tab}`) });
  }
  await navigate('updates');
  const updatePanel = page.getByRole('tabpanel', { name: '更新', exact: true });
  const button = updatePanel.getByRole('button', { name: '立即检查', exact: true });
  const status = updatePanel.getByRole('status');
  await status.evaluate(element => { window.__nativeUpdateStatus = element; });
  for (const mode of ['normal', 'reject', 'normal']) {
    await control('/ipc', { channel: 'updater:check', mode });
    await button.focus();
    await button.click();
    await expect(status).toContainText(mode === 'reject' ? '无法检查更新' : '已是最新版本');
    await expect(button).toBeFocused();
    expect(await status.evaluate(element => element === window.__nativeUpdateStatus)).toBe(true);
    result.cases.push({ id: 'F17-update-live-result', mode, message: await status.innerText(), focusRetained: true, nodeRetained: true });
  }
  await captureEvidence(evidenceContext, 'desktop-update-live-result');
  result.status = 'pass';
} catch (error) {
  result.status = 'fail';
  result.error = error.message;
  throw error;
} finally {
  await control('/ipc', { channel: 'updater:check', mode: 'normal' });
  result.cleanup = await endFixture(page, navigate).catch(error => ({ restored: false, error: error.message }));
  result.completedAt = new Date().toISOString();
  await fs.writeFile(resultFile, JSON.stringify(result, null, 2));
  console.log(JSON.stringify({ status: result.status, cases: result.cases.length, error: result.error, cleanup: result.cleanup }));
}
