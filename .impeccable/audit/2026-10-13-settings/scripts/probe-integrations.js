// Feed this body to resume-run.mjs; do not run it as a standalone browser script.
// All exercised writes are memory-only mocks. No real token is created or copied.
const caseFilter = process.env.SETTINGS_AUDIT_CASE?.trim() || '';
const caseNames = ['token-unconfirmed-dismissal', 'github-second-toggle-during-save', 'integration-connected-badge-contrast'];
if (caseFilter && !caseNames.includes(caseFilter)) throw new Error(`Unknown SETTINGS_AUDIT_CASE: ${caseFilter}`);
const resultFile = path.join(dir, caseFilter ? 'probe-integrations-github-results.json' : 'probe-integrations-results.json');
const output = { recordedAt: new Date().toISOString(), caseFilter: caseFilter || null, cases: [] };
await fs.mkdir(path.join(dir, 'screenshots'), { recursive: true });
await fs.mkdir(path.join(dir, 'ax'), { recursive: true });

const evidence = async name => {
  const screenshot = await shot(name);
  const ax = path.join(dir, 'ax', `${name}.txt`);
  await fs.writeFile(ax, await page.locator('body').ariaSnapshot());
  return { screenshot, ax };
};

async function runProbe(name, body) {
  if (caseFilter && caseFilter !== name) return;
  const result = { name, status: 'probe-error', evidence: [] };
  output.cases.push(result);
  let viewport;
  let ownsFixture = false;
  try {
    // Unrecognized control paths return the current state without changing it.
    viewport = await control('/');
    if ((await page.getByRole('dialog').count()) + (await page.getByRole('alertdialog').count())) {
      throw new Error('An unrelated dialog is open; refusing to dismiss it');
    }
    await queryClient();
    await page.evaluate(async () => {
      if (window.__settingsIntegrationsProbe) throw new Error('Previous probe fixture was not restored');
      const url = performance.getEntriesByType('resource').map(entry => entry.name)
        .find(name => /packages\/core\/api\/index\.ts(?:\?|$)/.test(name));
      if (!url) throw new Error('Loaded core API module URL was not found');
      const api = (await import(url)).getApi();
      const qc = window.__auditQC;
      const state = {
        api, qc, methods: new Map(), queries: [],
        href: location.href, history: history.state,
        dark: document.documentElement.classList.contains('dark'),
        colorScheme: document.documentElement.style.getPropertyValue('color-scheme'),
        colorSchemePriority: document.documentElement.style.getPropertyPriority('color-scheme'),
      };
      state.mock = (key, replacement) => {
        if (!state.methods.has(key)) state.methods.set(key, Object.getOwnPropertyDescriptor(api, key));
        Object.defineProperty(api, key, { configurable: true, writable: true, value: replacement });
      };
      state.seed = async (key, data) => {
        await qc.cancelQueries({ queryKey: key, exact: true });
        if (!state.queries.some(saved => JSON.stringify(saved.key) === JSON.stringify(key))) {
          const query = qc.getQueryCache().find({ queryKey: key, exact: true });
          state.queries.push({ key, query, state: query ? { ...query.state } : null });
        }
        qc.setQueryData(key, data);
      };
      window.__settingsIntegrationsProbe = state;
    });
    ownsFixture = true;
    await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
    await body(result);
  } catch (error) {
    result.status = 'probe-error';
    result.error = error.message;
  } finally {
    result.cleanupErrors = [];
    if (ownsFixture) {
      try {
        await page.evaluate(async () => {
          const state = window.__settingsIntegrationsProbe;
          state.releaseUpdate?.();
          if (state.pendingUpdate) await state.pendingUpdate;
        });
        const tokenDialog = page.getByRole('dialog', { name: '个人访问 token 已创建', exact: true });
        if (await tokenDialog.isVisible()) {
          await tokenDialog.getByRole('checkbox').check();
          await tokenDialog.getByRole('button', { name: '完成', exact: true }).click();
        }
        // Unmount our components before restoring query snapshots or API methods.
        await navigate('profile');
      } catch (error) {
        result.cleanupErrors.push(`unmount: ${error.message}`);
      }
      try {
        const errors = await page.evaluate(async () => {
          const state = window.__settingsIntegrationsProbe;
          const errors = [];
          for (const saved of state.queries) {
            try { await state.qc.cancelQueries({ queryKey: saved.key, exact: true }); }
            catch (error) { errors.push(`cancel query: ${error.message}`); }
          }
          for (const [key, descriptor] of state.methods) {
            try {
              if (descriptor) Object.defineProperty(state.api, key, descriptor);
              else delete state.api[key];
            } catch (error) { errors.push(`restore ${key}: ${error.message}`); }
          }
          for (const saved of state.queries) {
            try {
              if (saved.query) saved.query.setState(saved.state);
              else state.qc.removeQueries({ queryKey: saved.key, exact: true });
            } catch (error) { errors.push(`restore query: ${error.message}`); }
          }
          document.documentElement.classList.toggle('dark', state.dark);
          if (state.colorScheme) document.documentElement.style.setProperty('color-scheme', state.colorScheme, state.colorSchemePriority);
          else document.documentElement.style.removeProperty('color-scheme');
          history.replaceState(state.history, '', state.href);
          dispatchEvent(new PopStateEvent('popstate'));
          delete window.__settingsIntegrationsProbe;
          return errors;
        });
        result.cleanupErrors.push(...errors);
      } catch (error) {
        result.cleanupErrors.push(`restore fixture: ${error.message}`);
      }
    }
    if (viewport) {
      try { await control('/viewport', { width: viewport.bounds.width, height: viewport.bounds.height, zoom: viewport.zoom }); }
      catch (error) { result.cleanupErrors.push(`viewport: ${error.message}`); }
    }
    await fs.writeFile(resultFile, JSON.stringify(output, null, 2));
    console.log(JSON.stringify({ case: name, status: result.status, cleanupErrors: result.cleanupErrors }));
  }
  if (result.cleanupErrors.length) throw new Error(`Cleanup failed for ${name}; stopping to avoid fixture leakage`);
}

async function githubFixture() {
  await page.evaluate(async () => {
    const state = window.__settingsIntegrationsProbe;
    const { api, qc } = state;
    const workspaces = qc.getQueryData(['workspaces', 'list']);
    // Electron uses MemoryRouter; the document URL does not carry the workspace.
    const settingsLink = document.querySelector('[data-sidebar="menu-button"][href$="/settings"]');
    const segments = settingsLink ? new URL(settingsLink.href).pathname.split('/').filter(Boolean) : [];
    const slug = segments.length >= 2 ? decodeURIComponent(segments[segments.length - 2]) : null;
    const workspace = workspaces?.find(item => item.slug === slug);
    if (!workspace) throw new Error('Current workspace is not present in the query cache');
    const memberKey = ['workspaces', workspace.id, 'members'];
    const members = qc.getQueryData(memberKey) ?? await api.listMembers(workspace.id);
    if (!Array.isArray(members) || !members.length) throw new Error('Membership fixture requires an existing workspace member');
    state.workspace = { ...workspace, settings: {
      ...workspace.settings, github_enabled: true, github_pr_sidebar_enabled: true,
      co_authored_by_enabled: true, github_auto_link_prs_enabled: true,
    } };
    const workspaceList = () => workspaces.map(item => item.id === workspace.id ? state.workspace : item);
    const memberList = members.map(member => ({ ...member, role: 'owner' }));
    const installations = { configured: true, can_manage: true, installations: [{
      id: 'audit-github-installation', installation_id: 1, account_login: 'audit-connected',
      account_type: 'Organization', connected_by: 'Audit fixture',
    }] };
    const listMembers = api.listMembers.bind(api);
    const listInstallations = api.listGitHubInstallations.bind(api);
    state.mock('listWorkspaces', async () => workspaceList());
    state.mock('listMembers', async id => id === workspace.id ? memberList : listMembers(id));
    state.mock('listGitHubInstallations', async id => id === workspace.id ? installations : listInstallations(id));
    state.mock('deleteGitHubInstallation', async () => { throw new Error('Disconnect is outside this memory fixture'); });
    state.calls = [];
    state.mock('updateWorkspace', (id, data) => {
      if (id !== workspace.id) return Promise.reject(new Error('Workspace is outside this memory fixture'));
      const updated = { ...state.workspace, settings: { ...data.settings } };
      state.calls.push({ prSidebar: data.settings.github_pr_sidebar_enabled, coAuthor: data.settings.co_authored_by_enabled });
      if (state.calls.length === 1) {
        state.pendingUpdate = new Promise(resolve => {
          state.releaseUpdate = () => { state.workspace = updated; resolve(updated); };
        });
        return state.pendingUpdate;
      }
      state.workspace = updated;
      return Promise.resolve(updated);
    });
    await state.seed(['workspaces', 'list'], workspaceList());
    await state.seed(memberKey, memberList);
    await state.seed(['github', workspace.id, 'installations'], installations);
  });
  await navigate('integrations');
}

await runProbe('token-unconfirmed-dismissal', async result => {
  await page.evaluate(() => {
    const state = window.__settingsIntegrationsProbe;
    state.tokens = [];
    state.created = 0;
    state.mock('listPersonalAccessTokens', async () => [...state.tokens]);
    state.mock('createPersonalAccessToken', async input => {
      const index = ++state.created;
      const token = { id: `audit-token-${index}`, name: input.name, token_prefix: 'audit_fake',
        created_at: '2026-10-14T00:00:00Z', expires_at: null, last_used_at: null };
      state.tokens.push(token);
      return { ...token, token: `AUDIT-ONLY-NOT-A-REAL-TOKEN-${index}` };
    });
    state.mock('revokePersonalAccessToken', async id => { state.tokens = state.tokens.filter(token => token.id !== id); });
  });
  await navigate('tokens');
  result.dismissals = [];
  for (const method of ['escape', 'close-button']) {
    const panel = page.getByRole('tabpanel', { name: 'API Token', exact: true });
    await panel.getByRole('textbox', { name: /^Token 名称/ }).fill(`审计虚拟令牌 ${method}`);
    await panel.getByRole('button', { name: '创建', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: '个人访问 token 已创建', exact: true });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('checkbox')).not.toBeChecked();
    await expect(dialog.getByRole('button', { name: '完成', exact: true })).toBeDisabled();
    if (method === 'escape') result.evidence.push(await evidence('probe-integrations-token-unconfirmed'));
    const close = dialog.locator('[data-slot="dialog-close"]');
    const available = method === 'escape' || await close.isVisible();
    if (method === 'escape') await page.keyboard.press('Escape');
    else if (available) await close.click();
    await page.waitForTimeout(200);
    const dismissed = !await dialog.isVisible();
    result.dismissals.push({ method, available, storedConfirmed: false, doneDisabled: true, dismissed });
    if (dismissed) result.evidence.push(await evidence(`probe-integrations-token-after-${method}`));
    else {
      await dialog.getByRole('checkbox').check();
      await dialog.getByRole('button', { name: '完成', exact: true }).click();
    }
  }
  result.fakeTokensCreated = await page.evaluate(() => window.__settingsIntegrationsProbe.created);
  result.status = result.dismissals.some(item => item.dismissed) ? 'finding' : 'not-reproduced';
});

await runProbe('github-second-toggle-during-save', async result => {
  await githubFixture();
  await page.getByRole('link', { name: 'GitHub', exact: true }).click();
  const first = page.getByRole('switch', { name: 'Pull Request 侧栏', exact: true });
  const second = page.getByRole('switch', { name: 'Co-authored-by trailer', exact: true });
  await expect(first).toHaveAttribute('aria-checked', 'true');
  await expect(second).toHaveAttribute('aria-checked', 'true');
  await expect(first).toBeEnabled();
  await first.click();
  await expect.poll(() => page.evaluate(() => window.__settingsIntegrationsProbe.calls.length)).toBe(1);
  result.secondWasEnabled = await second.isEnabled();
  if (result.secondWasEnabled) await second.click();
  await page.waitForTimeout(150);
  result.callsWhilePending = await page.evaluate(() => window.__settingsIntegrationsProbe.calls);
  result.secondCheckedWhilePending = await second.getAttribute('aria-checked');
  result.evidence.push(await evidence('probe-integrations-github-pending'));
  await page.evaluate(async () => {
    const state = window.__settingsIntegrationsProbe;
    state.releaseUpdate();
    await state.pendingUpdate;
  });
  await expect(first).toBeEnabled();
  await expect(first).toHaveAttribute('aria-checked', 'false');
  result.callsAfterSettlement = await page.evaluate(() => window.__settingsIntegrationsProbe.calls);
  result.secondCheckedAfterSettlement = await second.getAttribute('aria-checked');
  result.status = result.secondWasEnabled && result.callsAfterSettlement.length === 1
    && result.secondCheckedAfterSettlement === 'true' ? 'finding' : 'not-reproduced';
});

await runProbe('integration-connected-badge-contrast', async result => {
  const { scanContrast } = await import(path.resolve(dir, '../2026-10-11-desktop-core/scripts/contrast.mjs'));
  await githubFixture();
  const badge = page.getByRole('link', { name: 'GitHub', exact: true }).getByText('已连接', { exact: true });
  await expect(badge).toBeVisible();
  result.themes = [];
  for (const dark of [false, true]) {
    await page.evaluate(dark => {
      document.documentElement.classList.toggle('dark', dark);
      document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
    }, dark);
    await badge.scrollIntoViewIfNeeded();
    await page.mouse.move(0, 0);
    await page.waitForTimeout(150);
    const contrast = await scanContrast(page);
    const badgeClass = await badge.getAttribute('class');
    const measurements = contrast.measurements.filter(item => item.text === '已连接' && item.class === badgeClass);
    if (!measurements.length) throw new Error('Connected badge was not sampled by contrast.mjs');
    const theme = dark ? 'dark' : 'light';
    result.themes.push({ theme, measurements, failures: measurements.filter(item => item.ratio < item.floor) });
    result.evidence.push(await evidence(`probe-integrations-connected-${theme}`));
  }
  result.status = result.themes.some(theme => theme.failures.length) ? 'finding' : 'not-reproduced';
});

console.log(`Integration probe results: ${resultFile}`);
