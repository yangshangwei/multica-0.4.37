// Feed to resume-run.mjs. Every write is intercepted in memory.
const { beginFixture, endFixture, captureEvidence } = await import(path.join(dir, 'scripts/native-fixtures.mjs'));
const evidenceContext = { page, cdp, shot, fs, path, dir };
const locale = JSON.parse(await fs.readFile(path.resolve('packages/views/locales/zh-Hans/settings.json'), 'utf8'));
const selected = process.env.SETTINGS_NATIVE_CASE;
const outcomes = [];

async function runCase(id, body) {
  if (selected && !selected.split(',').includes(id)) return;
  const outcome = { id, recordedAt: new Date().toISOString(), fixture: 'Memory-only API responses and writes in the disposable renderer.' };
  await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
  await beginFixture(page, navigate, queryClient);
  try {
    await page.evaluate(async () => {
      const state = window.__nativeAcceptanceFixture;
      const authUrl = performance.getEntriesByType('resource').map(entry => entry.name).find(name => /packages\/core\/auth\/index\.ts(?:\?|$)/.test(name));
      state.auth = (await import(authUrl)).useAuthStore;
      state.user = state.auth.getState().user;
      const key = ['workspaces', state.workspace.id, 'members'];
      const members = (state.qc.getQueryData(key) ?? await state.api.listMembers(state.workspace.id)).map(member => ({ ...member, role: 'owner' }));
      state.mock('listMembers', async () => members);
      await state.seed(key, members);
    });
    await body(outcome);
    outcome.status = 'pass';
  } catch (error) {
    outcome.status = 'fail';
    outcome.error = error.message;
    await captureEvidence(evidenceContext, `state-${id}-failure`).catch(() => {});
  } finally {
    await page.evaluate(() => {
      const state = window.__nativeAcceptanceFixture;
      state.releasePending?.();
      if (state.auth && state.user) state.auth.getState().setUser(state.user);
    });
    const confirmation = page.getByRole('alertdialog');
    if (await confirmation.isVisible()) {
      const cancel = confirmation.getByRole('button', { name: '取消', exact: true });
      await expect(cancel).toBeEnabled();
      await cancel.click();
      await expect(confirmation).toBeHidden();
    }
    outcome.cleanup = await endFixture(page, navigate).catch(error => ({ restored: false, error: error.message }));
    outcome.completedAt = new Date().toISOString();
    await fs.writeFile(path.join(dir, `state-${id}.json`), JSON.stringify(outcome, null, 2));
    outcomes.push(outcome);
    console.log(JSON.stringify({ id, status: outcome.status, error: outcome.error, cleanup: outcome.cleanup }));
    if (!outcome.cleanup.restored) throw new Error(`Fixture cleanup failed: ${id}`);
  }
}

await runCase('mcp', async outcome => {
  await page.evaluate(async () => {
    const state = window.__nativeAcceptanceFixture;
    state.mcpStatus = 200;
    state.mcpKey = ['workspaces', state.workspace.id, 'mcp-servers'];
    state.servers = [{ id: '22222222-2222-4222-8222-222222222222', workspace_id: state.workspace.id, name: 'Native acceptance MCP', transport: 'http', created_at: '2026-10-14T00:00:00Z', updated_at: '2026-10-14T00:00:00Z' }];
    state.routes.set(`GET /api/workspaces/${state.workspace.id}/mcp-servers`, () => new Response(JSON.stringify(state.mcpStatus === 200 ? state.servers : { error: 'Native acceptance read failure' }), { status: state.mcpStatus, headers: { 'Content-Type': 'application/json' } }));
    await state.seed(state.mcpKey, state.servers);
  });
  await navigate('mcp');
  await page.getByRole('button', { name: /重命名.*Native acceptance MCP/ }).click();
  const input = page.getByRole('textbox', { name: /重命名/ });
  await input.fill('Native retained MCP draft');
  await input.focus();
  await input.evaluate(element => { window.__nativeMcpInput = element; });
  await page.evaluate(async () => { const state = window.__nativeAcceptanceFixture; state.mcpStatus = 500; await state.qc.invalidateQueries({ queryKey: state.mcpKey }); });
  await expect(page.getByRole('alert')).toBeVisible();
  outcome.transient = await input.evaluate(element => ({ sameNode: element === window.__nativeMcpInput, focused: element === document.activeElement, value: element.value }));
  expect(outcome.transient).toEqual({ sameNode: true, focused: true, value: 'Native retained MCP draft' });
  outcome.evidence = await captureEvidence(evidenceContext, 'state-mcp-retained-500');
  for (const status of [403, 500]) {
    await page.evaluate(async status => { const state = window.__nativeAcceptanceFixture; state.mcpStatus = status; await state.qc.invalidateQueries({ queryKey: state.mcpKey }); }, status);
    await expect(input).toHaveCount(0);
    expect(await page.evaluate(() => window.__nativeAcceptanceFixture.qc.getQueryData(window.__nativeAcceptanceFixture.mcpKey))).toBeUndefined();
  }
  outcome.deniedSnapshotStayedAbsent = true;
});

await runCase('triage', async outcome => {
  await page.evaluate(async () => {
    const state = window.__nativeAcceptanceFixture;
    state.triage = { supported: true, enabled: true, acceptance_status: 'backlog', require_priority: false, responsibility_mode: 'none', responsibility_member_id: null, revision: 2 };
    state.triageKey = ['triage', state.workspace.id, 'settings'];
    state.triageFailure = false;
    state.mock('getTriageSettings', async () => { if (state.triageFailure) throw new Error('Native acceptance transient failure'); return state.triage; });
    state.mock('listTriageItems', async () => ({ items: [], total: 0, counts: { pending: 0, ready: 0, snoozed: 0 } }));
    await state.seed(state.triageKey, state.triage);
  });
  await navigate('triage');
  const priority = page.getByRole('switch', { name: /优先级/ });
  await priority.click();
  await priority.focus();
  await priority.evaluate(element => { window.__nativeTriageInput = element; });
  await page.evaluate(async () => { const state = window.__nativeAcceptanceFixture; state.triageFailure = true; await state.qc.invalidateQueries({ queryKey: state.triageKey }); });
  await expect(page.getByRole('alert')).toBeVisible();
  outcome.retained = await priority.evaluate(element => ({ sameNode: element === window.__nativeTriageInput, focused: element === document.activeElement, checked: element.getAttribute('aria-checked') }));
  expect(outcome.retained).toEqual({ sameNode: true, focused: true, checked: 'true' });
  outcome.evidence = await captureEvidence(evidenceContext, 'state-triage-retained-error');
});

await runCase('repository', async outcome => {
  await page.evaluate(async () => {
    const state = window.__nativeAcceptanceFixture;
    state.editedWorkspace = { ...state.workspace, repos: [{ url: 'https://example.invalid/native-acceptance.git', description: 'Memory fixture' }] };
    const workspaceList = () => state.workspaces.map(workspace => workspace.id === state.workspace.id ? state.editedWorkspace : workspace);
    state.mock('listWorkspaces', async () => workspaceList());
    state.repoCalls = 0;
    state.mock('updateWorkspace', async (_id, patch) => {
      state.repoCalls++;
      if (state.repoCalls === 1) return new Promise((_resolve, reject) => { state.releasePending = () => reject(new Error('Native acceptance repository save failed')); });
      state.editedWorkspace = { ...state.editedWorkspace, ...patch };
      return state.editedWorkspace;
    });
    await state.seed(['workspaces', 'list'], workspaceList());
  });
  await navigate('repositories');
  const panel = page.getByRole('tabpanel', { name: '代码仓库', exact: true, includeHidden: true });
  const row = panel.getByRole('textbox', { name: locale.repositories.url_placeholder, exact: true, includeHidden: true });
  await expect(row).toHaveValue('https://example.invalid/native-acceptance.git');
  await panel.getByRole('button', { name: locale.repositories.delete_aria, exact: true }).click();
  const dialog = page.getByRole('alertdialog');
  const confirm = dialog.getByRole('button', { name: locale.repositories.delete_confirm_action, exact: true });
  await confirm.click();
  await expect(confirm).toBeDisabled();
  await expect(row).toHaveValue('https://example.invalid/native-acceptance.git');
  await page.keyboard.press('Escape');
  await expect(dialog).toBeVisible();
  expect(await page.evaluate(() => window.__nativeAcceptanceFixture.repoCalls)).toBe(1);
  await page.evaluate(() => window.__nativeAcceptanceFixture.releasePending());
  await expect(confirm).toBeEnabled();
  await expect(row).toHaveValue('https://example.invalid/native-acceptance.git');
  outcome.failureRetained = true;
  outcome.evidence = await captureEvidence(evidenceContext, 'state-repository-failure-retry');
  await confirm.click();
  await expect(dialog).toBeHidden();
  await expect(panel.getByText(locale.repositories.empty, { exact: true })).toBeVisible();
  outcome.requests = await page.evaluate(() => window.__nativeAcceptanceFixture.repoCalls);
  expect(outcome.requests).toBe(2);
});

await runCase('timezone', async outcome => {
  await page.evaluate(() => {
    const state = window.__nativeAcceptanceFixture;
    state.auth.getState().setUser({ ...state.user, timezone: 'UTC' });
    state.timezoneCalls = [];
    state.mock('updateMe', async patch => {
      state.timezoneCalls.push(patch.timezone);
      if (state.timezoneCalls.length === 1) return new Promise((_resolve, reject) => { state.releasePending = () => reject(new Error('Native acceptance timezone failure')); });
      return { ...state.user, ...patch };
    });
  });
  await navigate('preferences');
  const select = page.getByRole('combobox', { name: locale.preferences.timezone.title, exact: true });
  await select.click();
  await page.getByRole('option', { name: 'Europe/London', exact: true }).click();
  await expect(select).toBeDisabled();
  await expect(select).toHaveAttribute('aria-busy', 'true');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('listbox')).toBeHidden();
  expect(await page.evaluate(() => window.__nativeAcceptanceFixture.timezoneCalls)).toEqual(['Europe/London']);
  outcome.evidence = await captureEvidence(evidenceContext, 'state-timezone-pending');
  await page.evaluate(() => window.__nativeAcceptanceFixture.releasePending());
  await expect(select).toBeEnabled();
  await select.click();
  await page.getByRole('option', { name: 'Asia/Tokyo', exact: true }).click();
  await expect(select).toBeEnabled();
  await expect(select).toContainText('Asia/Tokyo');
  outcome.requests = await page.evaluate(() => window.__nativeAcceptanceFixture.timezoneCalls);
  outcome.finalTimezone = await page.evaluate(() => window.__nativeAcceptanceFixture.auth.getState().user.timezone);
  expect(outcome.requests).toEqual(['Europe/London', 'Asia/Tokyo']);
  expect(outcome.finalTimezone).toBe('Asia/Tokyo');
});

await runCase('token', async outcome => {
  await page.evaluate(async () => {
    const state = window.__nativeAcceptanceFixture;
    const base = { id: 'native-revoke', name: 'Native acceptance revoke', token_prefix: 'native_fake', created_at: '2026-10-14T00:00:00Z', expires_at: null, last_used_at: null };
    state.tokens = [base];
    state.mock('listPersonalAccessTokens', async () => [...state.tokens]);
    state.mock('createPersonalAccessToken', async request => {
      const created = { ...base, id: 'native-created', name: request.name };
      state.tokens.push(created);
      return { ...created, token: 'NATIVE-ACCEPTANCE-NOT-A-REAL-CREDENTIAL' };
    });
    state.revokeCalls = 0;
    state.mock('revokePersonalAccessToken', async id => {
      state.revokeCalls++;
      if (state.revokeCalls === 1) return new Promise((_resolve, reject) => { state.releasePending = () => reject(new Error('Native acceptance revoke failure')); });
      state.tokens = state.tokens.filter(token => token.id !== id);
    });
    await state.clear(['personal-access-tokens', state.user.id]);
  });
  await navigate('tokens');
  const panel = page.getByRole('tabpanel', { name: 'API Token', exact: true, includeHidden: true });
  await panel.getByRole('textbox', { name: locale.tokens.name_placeholder, exact: true }).fill('Native acceptance created');
  await panel.getByRole('button', { name: locale.tokens.create, exact: true }).click();
  const created = page.getByRole('dialog', { name: locale.tokens.created_dialog.title, exact: true });
  await expect(created).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(created).toBeVisible();
  await page.mouse.click(10, 10);
  await expect(created).toBeVisible();
  const close = created.locator('[data-slot="dialog-close"]');
  if (await close.isVisible() && await close.isEnabled()) await close.click();
  await expect(created).toBeVisible();
  await expect(created.getByRole('button', { name: locale.tokens.created_dialog.done, exact: true })).toBeDisabled();
  outcome.unacknowledgedDismissalsBlocked = ['escape', 'backdrop', 'close', 'done'];
  await created.getByRole('checkbox').click();
  await created.getByRole('button', { name: locale.tokens.created_dialog.done, exact: true }).click();
  await expect(created).toBeHidden();
  const revoke = panel.getByRole('button', { name: locale.tokens.revoke_aria.replace('{{name}}', 'Native acceptance revoke'), exact: true, includeHidden: true });
  await revoke.click();
  const dialog = page.getByRole('alertdialog');
  const confirm = dialog.getByRole('button', { name: /吊销/ });
  await confirm.click();
  await expect(confirm).toBeDisabled();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeVisible();
  expect(await page.evaluate(() => window.__nativeAcceptanceFixture.revokeCalls)).toBe(1);
  await page.evaluate(() => window.__nativeAcceptanceFixture.releasePending());
  await expect(confirm).toBeEnabled();
  await expect(revoke).toBeVisible();
  outcome.evidence = await captureEvidence(evidenceContext, 'state-token-revoke-failure');
  await confirm.click();
  await expect(dialog).toBeHidden();
  await expect(revoke).toHaveCount(0);
  outcome.revokeRequests = await page.evaluate(() => window.__nativeAcceptanceFixture.revokeCalls);
  expect(outcome.revokeRequests).toBe(2);
});

await runCase('profile', async outcome => {
  await page.evaluate(() => {
    const state = window.__nativeAcceptanceFixture;
    state.profileCalls = [];
    state.mock('updateMe', async patch => { state.profileCalls.push(patch); return { ...state.user, ...patch }; });
  });
  const panel = page.getByRole('tabpanel', { name: '个人资料', exact: true });
  const name = panel.locator('input[name="profile-name"]');
  const description = panel.locator('textarea[name="profile-description"]');
  await name.evaluate(element => { window.__nativeProfileName = element; });
  await description.evaluate(element => { window.__nativeProfileDescription = element; });
  await name.fill('');
  await description.fill('Native acceptance retained profile context');
  await description.blur();
  await page.waitForTimeout(800);
  await expect(name).toHaveAttribute('aria-invalid', 'true');
  expect(await page.evaluate(() => window.__nativeAcceptanceFixture.profileCalls)).toHaveLength(0);
  await name.fill('Native acceptance profile');
  await name.blur();
  await expect.poll(() => page.evaluate(() => window.__nativeAcceptanceFixture.profileCalls.length)).toBe(1);
  expect(await name.evaluate(element => element === window.__nativeProfileName)).toBe(true);
  expect(await description.evaluate(element => element === window.__nativeProfileDescription)).toBe(true);
  outcome.saved = await page.evaluate(() => window.__nativeAcceptanceFixture.profileCalls[0]);
  expect(outcome.saved.name).toBe('Native acceptance profile');
  expect(outcome.saved.profile_description).toBe('Native acceptance retained profile context');
  outcome.activeDraftRetained = true;
  outcome.evidence = await captureEvidence(evidenceContext, 'state-profile-corrected-save');
});

if (outcomes.some(outcome => outcome.status !== 'pass')) throw new Error('Native state/write acceptance failed; inspect per-case results.');
