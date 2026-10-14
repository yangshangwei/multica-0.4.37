// Feed this file to resume-run.mjs on stdin. It never submits a product form.
const configurationReport = {
  recordedAt: new Date().toISOString(),
  environment: 'Isolated Electron renderer; Chinese settings; native window sizing and zoom',
  limitations: [
    'Synthetic in-memory Quick Action data proves rendering, not API parsing.',
    'Focus and center hit tests do not establish physical-device touch behavior.',
    'Shortcut bindings are inspected without recording or changing them.',
  ],
  cases: [],
};
const configurationResultsPath = path.join(dir, 'probe-configuration-results.json');
const configurationContrastPath = path.resolve(dir, '../2026-10-11-desktop-core/scripts/contrast.mjs');
await fs.mkdir(path.join(dir, 'screenshots'), { recursive: true });
await fs.mkdir(path.join(dir, 'ax'), { recursive: true });

const configurationPaint = async () => {
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
};
const configurationViewport = async values => {
  await control('/viewport', values);
  await configurationPaint();
};
const configurationTheme = async dark => {
  await page.evaluate(value => {
    document.documentElement.classList.toggle('dark', value);
    document.documentElement.style.colorScheme = value ? 'dark' : 'light';
  }, dark);
  await configurationPaint();
};
const configurationCapture = async name => {
  const screenshot = await shot(name);
  const axPath = path.join(dir, 'ax', `${name}.json`);
  const textPath = path.join(dir, 'ax', `${name}.txt`);
  const ax = await cdp.send('Accessibility.getFullAXTree');
  await fs.writeFile(axPath, JSON.stringify(ax, null, 2));
  await fs.writeFile(textPath, await page.locator('body').ariaSnapshot());
  return { screenshot, ax: axPath, ariaSnapshot: textPath };
};

const configurationGeometry = async locator => locator.evaluate(root => {
  const bounds = element => {
    const rect = element.getBoundingClientRect();
    return Object.fromEntries(['x', 'y', 'width', 'height', 'top', 'right', 'bottom', 'left'].map(key => [key, Number(rect[key].toFixed(2))]));
  };
  const controls = [...root.querySelectorAll('button,input,textarea,[role="combobox"]')].map(element => {
    const rect = bounds(element);
    const x = rect.left + rect.width / 2;
    const y = rect.top + rect.height / 2;
    const inViewport = rect.width > 0 && rect.height > 0 && rect.left >= 0 && rect.top >= 0 && rect.right <= innerWidth && rect.bottom <= innerHeight;
    const centerInViewport = x >= 0 && x < innerWidth && y >= 0 && y < innerHeight;
    const hit = centerInViewport ? document.elementFromPoint(x, y) : null;
    const clippedBy = [];
    for (let ancestor = element.parentElement; ancestor; ancestor = ancestor.parentElement) {
      const style = getComputedStyle(ancestor);
      const clip = bounds(ancestor);
      const clipsX = /(hidden|clip|auto|scroll)/.test(style.overflowX) && (rect.left < clip.left || rect.right > clip.right);
      const clipsY = /(hidden|clip|auto|scroll)/.test(style.overflowY) && (rect.top < clip.top || rect.bottom > clip.bottom);
      if (clipsX || clipsY) clippedBy.push({ tag: ancestor.tagName, className: String(ancestor.className), overflowX: style.overflowX, overflowY: style.overflowY, scrollHeight: ancestor.scrollHeight, clientHeight: ancestor.clientHeight, bounds: clip });
    }
    return {
      id: element.id,
      role: element.getAttribute('role') || element.tagName.toLowerCase(),
      name: element.getAttribute('aria-label') || [...(element.labels || [])].map(label => label.textContent.trim()).join(' ') || element.getAttribute('placeholder') || element.textContent.trim(),
      value: 'value' in element ? element.value : null,
      bounds: rect,
      disabled: element.matches(':disabled,[aria-disabled="true"]'),
      focused: element === document.activeElement,
      fullyInViewport: inViewport,
      centerHitsControl: !!hit && (hit === element || element.contains(hit)),
      clippedBy,
    };
  });
  const style = getComputedStyle(root);
  return {
    viewport: { width: innerWidth, height: innerHeight, devicePixelRatio },
    root: { bounds: bounds(root), maxHeight: style.maxHeight, overflowX: style.overflowX, overflowY: style.overflowY, clientHeight: root.clientHeight, scrollHeight: root.scrollHeight },
    controls,
  };
});

const configurationClose = async dialog => {
  if (!await dialog.isVisible().catch(() => false)) return;
  // Cleanup may need to close a dialog whose Cancel button is outside the viewport.
  // This invokes only that dialog's local cancellation handler, never Save.
  const cancel = dialog.getByRole('button', { name: '取消', exact: true });
  if (await cancel.count()) await cancel.evaluate(button => button.click());
  else await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
};

const configurationCase = async (id, inspect) => {
  const result = { id, status: 'running', startedAt: new Date().toISOString(), observations: [], artifacts: [], limitations: [], cleanup: [] };
  const scope = { dialogs: [], restoreCache: null };
  let original;
  configurationReport.cases.push(result);
  try {
    if ((await page.getByRole('dialog').count()) + (await page.getByRole('alertdialog').count()) > 0) {
      throw new Error('An existing dialog is open; refusing to disturb another inspection.');
    }
    original = {
      window: await control('/'),
      renderer: await page.evaluate(() => ({
        dark: document.documentElement.classList.contains('dark'),
        colorScheme: document.documentElement.style.getPropertyValue('color-scheme'),
        colorSchemePriority: document.documentElement.style.getPropertyPriority('color-scheme'),
        tab: new URL(location.href).searchParams.get('tab') || 'profile',
      })),
    };
    await inspect(result, scope);
    result.status = result.observations.some(item => item.error) ? 'partial' : 'captured';
  } catch (error) {
    result.status = 'error';
    result.error = error.stack || String(error);
  } finally {
    const restore = async (name, action) => {
      try {
        const evidence = await action();
        result.cleanup.push({ name, restored: true, evidence });
      } catch (error) {
        result.cleanup.push({ name, restored: false, error: error.message });
        result.status = 'cleanup-error';
      }
    };
    for (const dialog of [...scope.dialogs].reverse()) await restore('owned-dialog', () => configurationClose(dialog));
    if (scope.restoreCache) await restore('query-cache', scope.restoreCache);
    if (original) {
      await restore('viewport', async () => {
        const { width, height } = original.window.bounds;
        await configurationViewport({ width, height, zoom: original.window.zoom });
        const state = await control('/');
        if (state.bounds.width !== width || state.bounds.height !== height || state.zoom !== original.window.zoom) throw new Error('Native viewport restoration did not match the initial state.');
        return { width, height, zoom: state.zoom };
      });
      await restore('settings-tab', () => navigate(original.renderer.tab));
      await restore('theme', async () => {
        await page.evaluate(state => {
          document.documentElement.classList.toggle('dark', state.dark);
          if (state.colorScheme) document.documentElement.style.setProperty('color-scheme', state.colorScheme, state.colorSchemePriority);
          else document.documentElement.style.removeProperty('color-scheme');
        }, original.renderer);
      });
    }
    result.finishedAt = new Date().toISOString();
    await fs.writeFile(configurationResultsPath, JSON.stringify(configurationReport, null, 2));
    console.log(JSON.stringify({ id, status: result.status, observations: result.observations.length, cleanupErrors: result.cleanup.filter(item => !item.restored).length }));
  }
};

await configurationCase('property-dialog-geometry', async (result, scope) => {
  await configurationViewport({ width: 1440, height: 1000, zoom: 1 });
  await configurationTheme(false);
  await navigate('properties');
  const dialog = page.getByRole('dialog', { name: '新建属性', exact: true });
  scope.dialogs.push(dialog);
  await page.getByRole('tabpanel').getByRole('button', { name: '新建属性', exact: true }).click();
  await expect(dialog).toBeVisible();
  await dialog.locator('#property-name').fill('审计草稿，不保存');
  const options = dialog.getByPlaceholder('选项名称', { exact: true });
  result.limitations.push('The 12-option draft is prepared using local Add option clicks, including DOM clicks when that control moves outside the viewport. This setup does not claim the add gesture is reachable.');
  for (let index = 0; index < 12; index++) {
    if (index > 0) {
      await dialog.getByRole('button', { name: '添加选项', exact: true }).evaluate(button => button.click());
      await expect(options).toHaveCount(index + 1);
    }
    await options.nth(index).fill(`审计选项 ${String(index + 1).padStart(2, '0')}`);
  }
  for (const variant of [
    { name: '900x700-100pct', width: 900, height: 700, zoom: 1 },
    { name: '900x1000-200pct', width: 900, height: 1000, zoom: 2 },
  ]) {
    await configurationViewport(variant);
    const observation = { variant, optionCount: await options.count(), beforeFocus: await configurationGeometry(dialog), focusProbes: [] };
    result.observations.push(observation);
    result.artifacts.push(await configurationCapture(`configuration-properties-${variant.name}`));
    for (const [name, target] of [
      ['property-name', dialog.locator('#property-name')],
      ['last-option', options.last()],
      ['save-without-activation', dialog.getByRole('button', { name: '保存属性', exact: true })],
    ]) {
      try {
        await target.focus();
        await configurationPaint();
        const geometry = await configurationGeometry(dialog);
        observation.focusProbes.push({ name, focusedControl: geometry.controls.find(item => item.focused) || null, dialogBounds: geometry.root.bounds });
      } catch (error) {
        observation.focusProbes.push({ name, error: error.message });
      }
    }
    result.artifacts.push(await configurationCapture(`configuration-properties-${variant.name}-after-focus`));
  }
});

await configurationCase('configuration-accessible-names', async (result, scope) => {
  await configurationViewport({ width: 1440, height: 1000, zoom: 1 });
  await navigate('shortcuts');
  const recorderDOM = await page.getByRole('tabpanel').locator('button[aria-pressed][aria-label]').evaluateAll(buttons => buttons.map(button => ({
    name: button.getAttribute('aria-label'),
    descriptionId: button.getAttribute('aria-describedby'),
    pressed: button.getAttribute('aria-pressed'),
    visibleText: button.textContent.trim(),
    keyTitles: [...button.querySelectorAll('[data-slot="shortcut-keycaps"] [title]')].map(key => key.getAttribute('title')),
    keycapsHidden: button.querySelector('[data-slot="shortcut-keycaps"]')?.getAttribute('aria-hidden') || null,
  })));
  const shortcutAX = await cdp.send('Accessibility.getFullAXTree');
  const recorderNames = new Set(recorderDOM.map(item => item.name));
  result.observations.push({
    section: 'shortcuts-current-values',
    recorders: recorderDOM,
    ax: shortcutAX.nodes.filter(node => !node.ignored && node.role?.value === 'button' && recorderNames.has(node.name?.value)),
  });
  result.artifacts.push(await configurationCapture('configuration-shortcut-values'));

  for (const form of [
    { tab: 'properties', create: '新建属性', title: '新建属性', expectedPurpose: '类型', name: 'property-type' },
    { tab: 'issue-statuses', create: '添加状态', title: '新建状态', expectedPurpose: '分类', name: 'status-category' },
  ]) {
    const dialog = page.getByRole('dialog', { name: form.title, exact: true });
    scope.dialogs.push(dialog);
    try {
      await navigate(form.tab);
      await page.getByRole('tabpanel').getByRole('button', { name: form.create, exact: true }).first().click();
      await expect(dialog).toBeVisible();
      const controls = await dialog.locator('[role="combobox"]').evaluateAll(elements => elements.map(element => ({
        id: element.id,
        text: element.textContent.trim(),
        ariaLabel: element.getAttribute('aria-label'),
        ariaLabelledBy: element.getAttribute('aria-labelledby'),
        associatedLabels: [...(element.labels || [])].map(label => label.textContent.trim()),
      })));
      const tree = await cdp.send('Accessibility.getFullAXTree');
      result.observations.push({ section: form.name, expectedPurpose: form.expectedPurpose, controls, ax: tree.nodes.filter(node => !node.ignored && node.role?.value === 'combobox') });
      result.artifacts.push(await configurationCapture(`configuration-${form.name}`));
    } catch (error) {
      result.observations.push({ section: form.name, error: error.message });
    } finally {
      await configurationClose(dialog);
    }
  }
});

await configurationCase('quick-action-warning-contrast', async (result, scope) => {
  const { scanContrast } = await import(configurationContrastPath);
  await configurationViewport({ width: 1440, height: 1000, zoom: 1 });
  await navigate('quick-actions');
  await queryClient();
  await page.waitForFunction(() => window.__auditQC.getQueryCache().getAll().some(query => query.queryKey[0] === 'quick-actions' && query.queryKey[2] === 'list' && query.queryKey[3] === true && query.getObserversCount() > 0 && query.state.fetchStatus === 'idle'));
  const cacheToken = `configuration-${Date.now()}`;
  scope.restoreCache = async () => page.evaluate(token => {
    const snapshot = window.__auditConfigurationSnapshots?.[token];
    if (!snapshot) return { noSnapshotCreated: true };
    const client = window.__auditQC;
    const current = client.getQueryCache().find({ queryKey: snapshot.queryKey, exact: true });
    if (current) current.setState(snapshot.state);
    else client.getQueryCache().build(client, snapshot.options, snapshot.state);
    const restored = client.getQueryCache().find({ queryKey: snapshot.queryKey, exact: true });
    if (restored.state.data !== snapshot.state.data) throw new Error('Original query data identity was not restored.');
    delete window.__auditConfigurationSnapshots[token];
    if (Object.keys(window.__auditConfigurationSnapshots).length === 0) delete window.__auditConfigurationSnapshots;
    return { queryKey: snapshot.queryKey, originalStatus: restored.state.status, dataIdentityRestored: true };
  }, cacheToken);
  const seeded = await page.evaluate(async token => {
    const client = window.__auditQC;
    const candidates = client.getQueryCache().getAll().filter(query => query.queryKey[0] === 'quick-actions' && query.queryKey[2] === 'list' && query.queryKey[3] === true && query.getObserversCount() > 0);
    if (candidates.length !== 1) throw new Error(`Expected one active Quick Action catalog, found ${candidates.length}.`);
    const query = candidates[0];
    await client.cancelQueries({ queryKey: query.queryKey, exact: true });
    window.__auditConfigurationSnapshots ||= {};
    window.__auditConfigurationSnapshots[token] = { queryKey: query.queryKey, state: query.state, options: query.options };
    const oldDate = new Date(Date.now() - 120 * 86400000).toISOString();
    const fixture = {
      id: token, workspace_id: query.queryKey[1], name: '审计：长期未使用', description: 'Temporary renderer-only contrast fixture',
      assignee_type: 'agent', assignee_id: `${token}-agent`, prompt: 'Local audit fixture; never execute.',
      visibility: 'public', status: 'active', last_used_at: null, use_count: 0, created_by_id: '',
      created_at: oldDate, updated_at: oldDate, target_name: '审计私有目标', target_public: false, target_missing: false,
    };
    client.setQueryData(query.queryKey, { quick_actions: [fixture] });
    return { queryKey: query.queryKey, fixture };
  }, cacheToken);
  result.fixture = seeded;
  const panel = page.getByRole('tabpanel');
  await expect(panel.getByText(seeded.fixture.name, { exact: true })).toBeVisible();
  for (const dark of [false, true]) {
    await configurationTheme(dark);
    await expect(panel.getByText(seeded.fixture.name, { exact: true })).toBeVisible();
    const contrast = await scanContrast(page);
    const warnings = await panel.locator('.text-warning').evaluateAll(elements => elements.map(element => {
      const style = getComputedStyle(element);
      return { text: element.textContent.trim(), color: style.color, backgroundColor: style.backgroundColor, fontSize: style.fontSize, opacity: style.opacity, className: element.className };
    }));
    const measurements = contrast.measurements.filter(item => /(?:^|\s)text-warning(?:\s|$)/.test(item.class));
    if (measurements.length < 2) result.limitations.push(`${dark ? 'dark' : 'light'}: contrast scanner returned fewer than two warning samples; inspect screenshots and computed styles before concluding.`);
    result.observations.push({ theme: dark ? 'dark' : 'light', computed: warnings, measurements, failures: measurements.filter(item => item.ratio < item.floor), scannerPath: configurationContrastPath });
    result.artifacts.push(await configurationCapture(`configuration-quick-action-warning-${dark ? 'dark' : 'light'}`));
  }
});

configurationReport.finishedAt = new Date().toISOString();
await fs.writeFile(configurationResultsPath, JSON.stringify(configurationReport, null, 2));
console.log(configurationResultsPath);
