// Run through resume-run.mjs on the disposable audit renderer; never standalone.
const output = {
  recordedAt: new Date().toISOString(),
  environment: 'Isolated Electron renderer; all writes used below are memory-only API mocks.',
  cases: [],
};
await fs.mkdir(path.join(dir, 'ax'), { recursive: true });

async function mockWrites(mode) {
  await page.evaluate(async mode => {
    if (window.__settingsPersonalProbe) throw new Error('Previous personal probe mocks were not restored');
    const url = performance.getEntriesByType('resource').map(entry => entry.name)
      .find(name => /packages\/core\/api\/index\.ts(?:\?|$)/.test(name));
    if (!url) throw new Error('Loaded core API module not found');
    const { getApi, ApiError } = await import(url);
    const api = getApi();
    const methods = ['updateMe', 'uploadFile', 'createMember'];
    for (const method of methods) {
      if (typeof api[method] !== 'function') throw new Error(`Missing API method: ${method}`);
    }
    const state = {
      api, calls: [], pending: [],
      originals: methods.map(method => [method, Object.getOwnPropertyDescriptor(api, method)]),
      beforeToasts: new Set(document.querySelectorAll('[data-sonner-toast]')),
    };
    window.__settingsPersonalProbe = state;
    api.updateMe = async payload => {
      state.calls.push({ method: 'updateMe', fields: Object.keys(payload), nameEmpty: payload.name?.trim() === '' });
      throw new ApiError('AUDIT_PERSONAL_PROFILE_WRITE_FAILED', 500, 'Internal Server Error');
    };
    api.uploadFile = async file => {
      state.calls.push({ method: 'uploadFile', name: file.name, size: file.size, type: file.type });
      if (mode !== 'avatar') throw new Error('Unexpected upload blocked by personal probe');
      return {
        id: '', filename: file.name, size: file.size, content_type: file.type,
        url: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+afo0AAAAASUVORK5CYII=',
      };
    };
    api.createMember = (workspaceId, payload) => {
      state.calls.push({ method: 'createMember', workspaceId, email: payload.email, role: payload.role });
      if (mode !== 'invite') return Promise.reject(new Error('Unexpected invitation blocked by personal probe'));
      return new Promise((_, reject) => state.pending.push(reject));
    };
  }, mode);
}

async function readMock() {
  return page.evaluate(() => {
    const state = window.__settingsPersonalProbe;
    if (!state) throw new Error('Personal probe mocks are not installed');
    return {
      calls: state.calls,
      freshToasts: [...document.querySelectorAll('[data-sonner-toast]')]
        .filter(node => !state.beforeToasts.has(node))
        .map(node => ({ type: node.getAttribute('data-type'), text: node.textContent })),
    };
  });
}

async function restoreWrites() {
  await page.evaluate(async () => {
    const state = window.__settingsPersonalProbe;
    if (!state) return;
    for (const reject of state.pending.splice(0)) reject(new Error('AUDIT_PERSONAL_INVITATION_CANCELLED'));
    await new Promise(resolve => setTimeout(resolve, 0));
    for (const [method, descriptor] of state.originals) {
      if (descriptor) Object.defineProperty(state.api, method, descriptor);
      else delete state.api[method];
    }
    delete window.__settingsPersonalProbe;
  });
}

async function capture(result, suffix = '') {
  const name = `resume-personal-${result.id}${suffix}`;
  const axFile = path.join(dir, 'ax', `${name}.txt`);
  await fs.writeFile(axFile, await page.locator('body').ariaSnapshot());
  result.evidence ??= [];
  result.evidence.push({ screenshot: path.relative(dir, await shot(name)), ax: path.relative(dir, axFile) });
}

async function runCase(id, sources, inspect) {
  const result = { id, sources, status: 'probe_failed', findingReproduced: null };
  try {
    await inspect(result);
  } catch (error) {
    result.status = 'probe_failed';
    result.error = error.stack || error.message;
    result.mockAtFailure = await readMock().catch(() => null);
    await capture(result, '-probe-error').catch(captureError => { result.captureError = captureError.message; });
  } finally {
    // Unmount editors before restoring methods, so queued blur/debounce work
    // cannot escape the mocks. Pending invitation promises are then rejected.
    try {
      await page.keyboard.press('Escape');
      await navigate('preferences');
    } catch (error) {
      result.cleanupNavigationError = error.message;
    }
    try {
      await restoreWrites();
      result.mocksRestored = true;
    } catch (error) {
      result.mocksRestored = false;
      result.cleanupError = error.message;
    }
    output.cases.push(result);
    await fs.writeFile(path.join(dir, 'probe-personal-results.json'), JSON.stringify(output, null, 2));
    console.log(JSON.stringify({ id, status: result.status, findingReproduced: result.findingReproduced, mocksRestored: result.mocksRestored }));
  }
}

await runCase('avatar-failure', [
  'packages/views/settings/components/account-tab.tsx:126',
  'packages/views/common/avatar-upload-control.tsx:186',
], async result => {
  await navigate('profile');
  await mockWrites('avatar');
  const panel = page.getByRole('tabpanel', { name: '个人资料', exact: true });
  await panel.locator('input[type="file"]').setInputFiles(path.join(dir, 'screenshots/profile-light-wide.png'));
  const dialog = page.getByRole('dialog', { name: /编辑头像|Edit avatar/i });
  await expect(dialog).toBeVisible();
  const save = dialog.getByRole('button', { name: /^(保存|Save|Apply)$/i });
  await expect(save).toBeEnabled();
  await save.click();
  await expect.poll(async () => (await readMock()).calls.filter(call => call.method === 'updateMe').length).toBeGreaterThan(0);
  await page.waitForTimeout(250);
  result.observed = { ...await readMock(), dialogOpen: await dialog.isVisible() };
  result.findingReproduced = !result.observed.dialogOpen && result.observed.freshToasts.some(toast =>
    toast.type === 'success' && /头像已更新|avatar updated/i.test(toast.text));
  result.status = 'product_behavior';
  result.expected = 'Rejected profile save keeps the crop dialog open and never announces avatar success.';
  await capture(result);
});

await runCase('empty-name', [
  'packages/views/settings/components/account-tab.tsx:90',
  'packages/views/settings/components/use-auto-save.ts:114',
], async result => {
  await navigate('profile');
  const panel = page.getByRole('tabpanel', { name: '个人资料', exact: true });
  const name = panel.locator('input[name="profile-name"]');
  const description = panel.locator('textarea[name="profile-description"]');
  const originalName = await name.inputValue();
  const originalDescription = await description.inputValue();
  if (!originalName.trim()) {
    result.status = 'skipped_static_only';
    result.reason = 'The fixture has no nonempty saved name to clear safely.';
    await capture(result);
    return;
  }
  await mockWrites('empty-name');
  const draft = `AUDIT_UNSAVED_PROFILE_CONTEXT_${Date.now()}`;
  try {
    await name.fill('');
    await description.fill(draft);
    await description.blur();
    await page.waitForTimeout(900); // Exceeds the component's 650 ms debounce.
    const validation = await name.evaluate(node => ({
      value: node.value, required: node.required,
      invalid: node.getAttribute('aria-invalid'), describedBy: node.getAttribute('aria-describedby'),
      validationMessage: node.validationMessage,
    }));
    const alerts = await panel.getByRole('alert').allTextContents();
    const statuses = await panel.getByRole('status').allTextContents();
    const validationHints = (await panel.innerText()).split('\n')
      .filter(text => /必填|不能为空|不得为空|未保存|required|cannot be empty|not saved/i.test(text));
    const mock = await readMock();
    result.observed = { ...mock, validation, alerts, statuses, validationHints, draftRetainedBeforeNavigation: await description.inputValue() === draft };
    await capture(result, '-draft');
    await navigate('preferences');
    await navigate('profile');
    result.observed.draftLostAfterNavigation = await description.inputValue() !== draft;
    result.findingReproduced = validation.value === '' && validation.invalid !== 'true' &&
      !validation.validationMessage && alerts.length === 0 && statuses.length === 0 && validationHints.length === 0 &&
      mock.calls.length === 0 && result.observed.draftRetainedBeforeNavigation;
    result.status = 'product_behavior';
    result.expected = 'An empty required name explains why profile context cannot be saved; unsaved content is identifiable.';
    await capture(result, '-returned');
  } finally {
    if (await name.isVisible().catch(() => false)) {
      // Restore the description while the blank name still prevents autosave.
      await description.fill(originalDescription);
      await name.fill(originalName);
      await name.blur();
    }
  }
});

await runCase('invite-repeat-enter', [
  'packages/views/settings/components/members-tab.tsx:434',
  'packages/views/settings/components/members-tab.tsx:812',
], async result => {
  await navigate('members');
  const panel = page.getByRole('tabpanel', { name: '成员', exact: true });
  const email = panel.locator('input[name="invite-email"]');
  if (!await email.isVisible()) {
    result.status = 'skipped_static_only';
    result.reason = 'Invite controls are unavailable for the current fixture; no membership or permission was changed.';
    await capture(result);
    return;
  }
  await mockWrites('invite');
  await email.fill(`audit-double-enter-${Date.now()}@example.invalid`);
  await email.press('Enter');
  await expect.poll(async () => (await readMock()).calls.filter(call => call.method === 'createMember').length).toBeGreaterThan(0);
  const invite = panel.getByRole('button', { name: /^(邀请(?:中\.\.\.)?|Invite|Inviting\.\.\.)$/i });
  const pending = { buttonDisabled: await invite.isDisabled(), emailEnabled: await email.isEnabled() };
  if (pending.emailEnabled) await email.press('Enter');
  await page.waitForTimeout(200);
  const mock = await readMock();
  result.observed = { ...mock, pending, requestCount: mock.calls.filter(call => call.method === 'createMember').length };
  result.findingReproduced = result.observed.requestCount > 1;
  result.status = 'product_behavior';
  result.expected = 'A second Enter during a pending invitation does not dispatch another request.';
  await capture(result);
});
