import { chromium, expect } from '@playwright/test';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Run against an isolated production Web build. API and WS observations are
// synthetic; this never creates an account or executes an installer/Agent CLI.
const baseURL = process.env.COMPUTER_QA_BASE_URL;
if (!baseURL) throw new Error('Set COMPUTER_QA_BASE_URL to the isolated production Web origin');
const output = resolve(process.env.COMPUTER_QA_EVIDENCE || '.omx/reports/add-computer-platform');
mkdirSync(output, { recursive: true });
const userId = '11111111-1111-4111-8111-111111111111';
const workspaceId = '22222222-2222-4222-8222-222222222222';
const now = new Date().toISOString();
const workspace = {
  id: workspaceId, slug: 'computer-qa', name: 'Computer QA', issue_prefix: 'CQA',
  description: null, context: null, settings: {}, repos: [], avatar_url: null,
  created_at: now, updated_at: now,
};
const makeRuntime = (id, host, daemon, provider = 'codex', os = 'linux-amd64') => ({
  id, name: `${provider} (${host})`, custom_name: null, owner_id: userId,
  owner_name: 'Computer fixture', workspace_id: workspaceId, provider,
  daemon_id: daemon, runtime_mode: 'local', status: 'online',
  device_info: `${host} · ${os}`, agent_version: null, runtime_version: null,
  last_seen_at: new Date().toISOString(), created_at: now, updated_at: now,
  capabilities: {}, metadata: {}, models: [],
});
const original = makeRuntime('33333333-3333-4333-8333-333333333333', 'existing-computer', 'existing-daemon');
const added = makeRuntime('44444444-4444-4444-8444-444444444444', 'new-macbook', 'new-daemon', 'claude', 'darwin-arm64');
const baseConfig = {
  auth_mode: 'legacy', allow_signup: true, messaging_integrations_enabled: false,
  daemon_server_url: 'https://api.qa.internal', daemon_app_url: 'https://multica.qa.internal',
};
const browser = await chromium.launch({ headless: true });
const report = [];
try {
  for (const locale of ['en', 'zh-Hans']) {
    const translated = JSON.parse(readFileSync(resolve(`packages/views/locales/${locale}/runtimes.json`)));
    const copy = translated.connect;
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await context.addCookies([{ name: 'multica-locale', value: locale, url: baseURL }]);
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const state = { config: { ...baseConfig }, runtimes: [original], failInventory: false, inventoryRequests: 0 };
    const sockets = new Set();
    await page.routeWebSocket(/\/ws(?:\?|$)/, socket => {
      sockets.add(socket);
      socket.onMessage(message => {
        if (typeof message === 'string' && JSON.parse(message).type === 'auth') {
          socket.send(JSON.stringify({ type: 'auth_ack', payload: {} }));
        }
      });
      socket.onClose(() => sockets.delete(socket));
    });
    const notify = payload => {
      for (const socket of sockets) socket.send(JSON.stringify({ type: 'daemon:register', payload }));
    };
    const user = {
      id: userId, email: 'computer-fixture@localhost', name: 'Computer fixture',
      language: locale, avatar_url: null, onboarded_at: now,
      onboarding_questionnaire: {}, starter_content_state: 'imported',
      profile_description: '', timezone: null, created_at: now, updated_at: now,
    };
    await page.route('**/api/**', route => {
      const pathname = new URL(route.request().url()).pathname;
      let json = [];
      if (pathname === '/api/config') json = state.config;
      else if (pathname === '/api/me') json = user;
      else if (pathname === '/api/workspaces') json = [workspace];
      else if (pathname === `/api/workspaces/${workspaceId}`) json = workspace;
      else if (pathname.endsWith('/members')) json = [{
        id: '55555555-5555-4555-8555-555555555555', workspace_id: workspaceId,
        user_id: userId, role: 'owner', name: user.name, email: user.email,
        avatar_url: null, created_at: now,
      }];
      else if (pathname === '/api/runtimes') {
        state.inventoryRequests += 1;
        if (state.failInventory) return route.fulfill({ status: 503, json: { error: 'Fixture inventory unavailable' } });
        json = state.runtimes;
      } else if (pathname.endsWith('/unread-count')) json = { count: 0 };
      else if (pathname.endsWith('/preferences')) json = {};
      return route.fulfill({ json });
    });
    await page.addInitScript(() => {
      localStorage.setItem('multica_token', 'isolated-computer-fixture');
      localStorage.setItem('multica:chat:isOpen', 'false');
      localStorage.setItem('theme', 'light');
    });
    const dialog = page.getByRole('dialog');
    const enter = async () => {
      await page.goto(`${baseURL}/computer-qa/runtimes`, { waitUntil: 'domcontentloaded' });
      await page.getByRole('button', { name: translated.page.connect_remote, exact: true }).first().click();
      await expect(dialog).toBeVisible();
      await expect(dialog.locator('code')).toHaveCount(0);
    };
    const choose = platform => dialog.getByRole('tab', { name: platform, exact: true }).click();
    const capture = async name => {
      await dialog.screenshot({ path: `${output}/${locale}-${name}.png`, animations: 'disabled' });
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
    };
    await enter();
    await capture('initial');
    await dialog.getByRole('tab', { name: 'Windows', exact: true }).focus();
    await page.keyboard.press('Space');
    await expect(dialog.getByRole('tab', { name: 'Windows', exact: true })).toHaveAttribute('aria-selected', 'true');
    await expect(dialog.locator('code').first()).toContainText('install.ps1');
    await capture('windows');
    const installCopy = dialog.getByRole('button', { name: /Windows/ }).filter({ hasText: /Copy|复制/ });
    await installCopy.click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toContain('install.ps1');
    const setupText = await dialog.locator('code').filter({ hasText: /^multica setup/ }).textContent();
    await dialog.getByRole('button', { name: copy.copy_setup_aria, exact: true }).click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(setupText);
    await choose('macOS');
    await expect(dialog.locator('code').first()).toContainText('install.sh');
    await expect(dialog).not.toContainText(copy.copied);
    await capture('macos');
    await choose('Linux');
    await capture('linux');
    await dialog.locator('summary').click();
    await expect(dialog.locator('code').filter({ hasText: /^multica login --token$/ })).toHaveCount(1);
    await expect(dialog.locator('code').filter({ hasText: /^multica daemon start$/ })).toHaveCount(1);
    expect(await dialog.locator('code').allTextContents()).not.toContainEqual(expect.stringContaining('<YOUR_TOKEN>'));
    await capture('help');
    await page.setViewportSize({ width: 390, height: 844 });
    await capture('narrow-help');
    const bounds = await dialog.boundingBox();
    expect(bounds.x).toBeGreaterThanOrEqual(0);
    expect(bounds.x + bounds.width).toBeLessThanOrEqual(390);
    expect(bounds.y + bounds.height).toBeLessThanOrEqual(844);
    for (const [label, expected] of [
      [copy.copy_config_aria, "multica config set server_url 'https://api.qa.internal'\nmultica config set app_url 'https://multica.qa.internal'"],
      [copy.copy_login_aria, 'multica login --token'],
      [copy.copy_start_aria, 'multica daemon start'],
    ]) {
      await dialog.getByRole('button', { name: label, exact: true }).click();
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expected);
    }
    await expect(dialog.getByRole('button', { name: copy.close, exact: true }).last()).toBeVisible();
    await capture('narrow-help-end');
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('button', { name: translated.page.connect_remote, exact: true }).first()).toBeFocused();
    await page.setViewportSize({ width: 1440, height: 1000 });

    state.config = { ...baseConfig, daemon_server_url: 'http://127.0.0.1:8080' };
    await enter();
    await choose('Windows');
    await expect(dialog.locator('code').filter({ hasText: /multica setup|multica config set/ })).toHaveCount(0);
    await dialog.locator('summary').click();
    await expect(dialog.locator('code').filter({ hasText: /multica login|multica config set/ })).toHaveCount(0);
    await capture('address-needed');

    state.config = { ...baseConfig, cli_install_command: 'curl https://mirror.qa.internal/install.sh | bash' };
    await enter();
    await choose('Windows');
    await expect(dialog.locator('code').filter({ hasText: /install\.ps1|raw\.githubusercontent/ })).toHaveCount(0);
    await expect(dialog.getByRole('link').first()).toHaveAttribute('href', /\/computer-qa\/docs\//);
    await capture('missing-windows-installer');

    state.config = { ...baseConfig, cli_install_commands: { windows: "irm 'https://mirror.qa.internal/windows.ps1' | iex" } };
    await enter();
    await choose('Windows');
    await expect(dialog.locator('code').first()).toContainText('mirror.qa.internal/windows.ps1');
    await expect(dialog).not.toContainText('raw.githubusercontent.com');

    state.config = { ...baseConfig, daemon_server_url: '\n', daemon_app_url: '' };
    await enter();
    await choose('Windows');
    await expect(dialog).toContainText(copy.address_title);
    await expect(dialog.locator('code').filter({ hasText: /multica setup|multica config set/ })).toHaveCount(0);

    state.config = { ...baseConfig };
    state.runtimes = [{ ...original, workspace_id: undefined }];
    await enter();
    await choose('Windows');
    await expect(dialog.getByRole('button', { name: copy.retry, exact: true })).toBeVisible();
    state.runtimes = [original];
    await dialog.getByRole('button', { name: copy.retry, exact: true }).click();
    await expect(dialog).toContainText(copy.waiting_hint);
    const beforeScopeRecovery = state.inventoryRequests;
    notify({ runtime_id: original.id });
    await expect.poll(() => state.inventoryRequests).toBeGreaterThan(beforeScopeRecovery);
    await expect(dialog.getByRole('heading', { name: copy.title, exact: true })).toBeVisible();

    state.config = { ...baseConfig };
    state.runtimes = [original];
    await enter();
    await choose('Windows');
    const beforeNotify = state.inventoryRequests;
    notify(null);
    await expect.poll(() => state.inventoryRequests).toBeGreaterThan(beforeNotify);
    await expect(dialog.getByRole('heading', { name: copy.title, exact: true })).toBeVisible();
    state.runtimes = [original, makeRuntime('66666666-6666-4666-8666-666666666666', 'existing-computer', 'existing-daemon', 'claude')];
    const beforeExisting = state.inventoryRequests;
    notify({ runtime_id: '66666666-6666-4666-8666-666666666666' });
    await expect.poll(() => state.inventoryRequests).toBeGreaterThan(beforeExisting);
    await expect(dialog.getByRole('heading', { name: copy.title, exact: true })).toBeVisible();
    state.failInventory = true;
    notify({});
    await expect(dialog.getByRole('button', { name: copy.retry, exact: true })).toBeVisible({ timeout: 15000 });
    await capture('interrupted');
    state.failInventory = false;
    await dialog.getByRole('button', { name: copy.retry, exact: true }).click();
    await expect(dialog.getByRole('button', { name: copy.retry, exact: true })).toBeHidden();
    state.runtimes = [original, added];
    notify({ runtime_id: added.id });
    await expect(dialog.getByRole('heading', { name: copy.success_title, exact: true })).toBeVisible();
    await expect(dialog).toContainText('new-macbook');
    await expect(dialog).toContainText('macOS');
    await expect(dialog).toContainText(/claude/i);
    await capture('connected');
    expect(errors).toEqual([]);
    report.push({ locale, passed: true, inventoryRequests: state.inventoryRequests, viewportWidths: [1440, 390], separateHelpClipboardVerified: true, controlOnlyAddressRejected: true, incompleteBaselineRecovered: true, explicitInternalWindowsInstaller: true, pageErrors: errors });
    await context.close();
  }
} finally {
  await browser.close();
  writeFileSync(`${output}/browser-report.json`, `${JSON.stringify(report, null, 2)}\n`);
}
console.log(JSON.stringify(report, null, 2));
