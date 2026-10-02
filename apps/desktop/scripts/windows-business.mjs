import assert from 'node:assert/strict';
import { randomUUID, randomBytes, createHash } from 'node:crypto';
import { readFile, writeFile, mkdir, readdir, lstat, rm } from 'node:fs/promises';
import { join, resolve, relative } from 'node:path';
import { pathToFileURL } from 'node:url';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createRequire } from 'node:module';
import ts from 'typescript';
import { startFaultProxy } from './windows-business-proxy.mjs';

/* global window */
const requireRoot = createRequire(new URL('../../../package.json', import.meta.url));
const { _electron: electron } = requireRoot('@playwright/test');

const execute = promisify(execFile);
const options = Object.fromEntries(process.argv.slice(2).reduce((pairs, value, index, all) => index % 2 ? pairs : [...pairs, [value.replace(/^--/, ''), all[index + 1]]], []));
assert.equal(process.platform, 'win32', 'Native Windows required');
assert.equal(process.env.GITHUB_ACTIONS, 'true');
assert.equal(process.env.RUNNER_ENVIRONMENT, 'github-hosted');
assert(['baseline', 'verify'].includes(options.phase));
for (const key of ['state-dir', 'installed-directory', 'report']) assert(options[key], `Missing ${key}`);
const root = resolve(options['state-dir']);
assert(!relative(resolve(process.env.RUNNER_TEMP), root).startsWith('..'));
const setup = JSON.parse(await readFile(join(root, 'server-private.json'), 'utf8'));
assert.equal(new URL(setup.api).hostname, '127.0.0.1');
const installed = resolve(options['installed-directory']);
const cli = join(installed, 'resources/app.asar.unpacked/resources/bin/multica.exe');
const executable = join(installed, 'Multica.exe');
const digest = value => createHash('sha256').update(value).digest('hex');
const stateFile = join(root, 'business-private.json');
let state = options.phase === 'verify' ? JSON.parse(await readFile(stateFile, 'utf8')) : { runId: randomUUID() };
const secrets = new Set();
const redact = value => {
  let text = String(value);
  for (const secret of secrets) if (secret) text = text.split(secret).join('[redacted]');
  return text.replace(/eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g, '[redacted]').replace(/\b(?:mdt|mul|mip|mat)_[A-Za-z0-9._-]+\b/g, '[redacted]');
};
const report = { version: 1, phase: options.phase, startedAt: new Date().toISOString(), sourceCommit: process.env.GITHUB_SHA, cases: [], limitations: ['Deterministic synthetic provider; no real model credentials or calls', 'No signed/offline trust claim'] };
const emit = (name, details = {}) => { report.cases.push({ name, ...details }); process.stdout.write(name + '\n'); };
const delay = ms => new Promise(done => setTimeout(done, ms));
async function waitFor(label, fn, timeout = 90000) {
  const end = Date.now() + timeout; let error;
  while (Date.now() < end) { try { const result = await fn(); if (result) return result; } catch (caught) { error = caught; } await delay(300); }
  throw new Error(`${label} timed out${error ? ': ' + redact(error.message) : ''}`);
}
async function json(path) { return JSON.parse(await readFile(path, 'utf8')); }
async function api(path, method = 'GET', body, account, workspace, extra = {}) {
  const response = await fetch(setup.api + path, { method, signal: AbortSignal.timeout(20000), headers: { 'Content-Type': 'application/json', ...(account ? { Authorization: 'Bearer ' + account.token } : {}), ...(workspace ? { 'X-Workspace-ID': workspace.id } : {}), ...extra }, body: body === undefined ? undefined : JSON.stringify(body) });
  assert(response.ok, `${method} ${path}: HTTP ${response.status}`);
  const text = await response.text(); return text ? JSON.parse(text) : null;
}
async function createAccount(label) {
  const account = { username: 'win_' + state.runId.slice(0, 8) + '_' + label, password: 'W1!' + randomBytes(20).toString('hex') };
  secrets.add(account.password);
  const response = await api('/auth/register', 'POST', { ...account, name: 'Windows acceptance ' + label });
  account.id = response.user.id; account.token = response.token; secrets.add(account.token);
  return account;
}
async function logInAccount(account) {
  const response = await api('/auth/login', 'POST', { username: account.username, password: account.password });
  account.token = response.token; secrets.add(account.token); return account;
}
const env = { ...process.env, MULTICA_BUSINESS_STATE_DIR: root, MULTICA_WORKSPACES_ROOT: join(root, 'workspaces') };
// Discover every supported provider override from current production descriptors.
const probe = await readFile(join(setup.repo, 'server/internal/daemon/agents_probe.go'), 'utf8');
const descriptors = await readFile(join(setup.repo, 'server/pkg/agent/builtin_runtimes.go'), 'utf8');
const providers = new Set([...probe.matchAll(/probe\("(MULTICA_[A-Z0-9_]+_PATH)"/g)].map(match => match[1]));
for (const match of descriptors.matchAll(/EnvPrefix:\s*"(MULTICA_[A-Z0-9_]+)"/g)) providers.add(match[1] + '_PATH');
assert(providers.size >= 25 && providers.has('MULTICA_CLAUDE_PATH'));
for (const key of providers) env[key] = key === 'MULTICA_CLAUDE_PATH' ? join(root, 'fixture-claude.exe') : join(root, 'missing', key + '.exe');
// Helpers are production code, not replacements for product IPC or daemon behavior.
for (const name of ['daemon-profile', 'managed-control']) {
  const source = await readFile(join(setup.repo, 'apps/desktop/src/main', name + '.ts'), 'utf8');
  await writeFile(join(root, name + '.mjs'), ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } }).outputText);
}
const { deriveProfileName, healthPortForProfile } = await import(pathToFileURL(join(root, 'daemon-profile.mjs')));
const { ManagedControlConnection } = await import(pathToFileURL(join(root, 'managed-control.mjs')));
const proxy = await startFaultProxy(setup.api, setup.proxyPort);
const profile = deriveProfileName(proxy.origin);
let app, page, home;
const ownedHome = join(process.env.USERPROFILE, '.multica');
const homeMarker = join(ownedHome, 'windows-business-owner.json');
const daemonPids = new Set();
async function processAlive(pid) { try { process.kill(pid, 0); return true; } catch (error) { if (error.code === 'ESRCH') return false; throw error; } }
async function scope() {
  const control = await json(join(home, '.multica/profiles', profile, 'management-control.json'));
  secrets.add(control.token);
  const connection = new ManagedControlConnection(healthPortForProfile(profile), control.token, new AbortController().signal);
  const value = await connection.request('GET', '/management/session');
  assert.equal(value.profile, profile); assert.equal(value.user_id, state.account.id);
  if (!value.ready) return null;
  assert(value.workspace_ids.includes(state.workspace.id));
  daemonPids.add(control.pid);
  return { value, pid: control.pid };
}
async function identity(session) {
  const value = await json(join(home, '.multica/management', session.value.deployment_id, 'installation.json'));
  secrets.add(value.private_key_seed);
  return { installation_id: value.installation_id, daemon_namespace_id: value.daemon_namespace_id, public_key_sha256: digest(value.public_key) };
}
async function taskFor(issue) {
  const response = await api('/api/agents/' + state.agent.id + '/tasks', 'GET', undefined, state.account, state.workspace);
  return (Array.isArray(response) ? response : response.tasks).find(task => task.issue_id === issue.id);
}
async function newTask(title) {
  const issue = await api('/api/issues', 'POST', { title, status: 'todo', assignee_type: 'agent', assignee_id: state.agent.id }, state.account, state.workspace);
  const task = await waitFor('task running', async () => { const task = await taskFor(issue); return task?.status === 'running' ? task : null; });
  await waitFor('fixture provider started', () => json(join(root, 'provider-' + task.id + '.json')));
  return { issue, task };
}
try {
  const health = await api('/health');
  assert.equal(health.pid, setup.pid); assert.equal(health.commit, process.env.GITHUB_SHA);
  report.backend = { health, executableSha256: digest(await readFile(setup.serverExe)) };
  if (options.phase === 'baseline') {
    await assert.rejects(lstat(ownedHome), { code: 'ENOENT' });
    await mkdir(ownedHome);
    await writeFile(homeMarker, JSON.stringify({ runId: state.runId, root }));
    state.account = await createAccount('user');
    state.workspace = await api('/api/workspaces', 'POST', { name: 'Windows retained workspace', slug: 'win-' + state.runId.slice(0, 8) }, state.account);
    await api('/api/me/onboarding', 'PATCH', { questionnaire: { source: ['friends_colleagues'] } }, state.account);
    await api('/api/me/onboarding/complete', 'POST', {}, state.account);
    await api('/api/me', 'PATCH', { language: 'en' }, state.account);
    state.operator = await createAccount('admin');
    await execute(setup.serverExe, ['platform-admin', 'bootstrap', '--user', state.operator.id, '--reason', 'Disposable Windows business acceptance'], { cwd: join(setup.repo, 'server'), env: { ...env, ...setup.environment }, timeout: 60000 });
    await logInAccount(state.operator);
  } else {
    assert.deepEqual(await json(homeMarker), { runId: state.runId, root });
    for (const account of [state.account, state.operator]) { secrets.add(account.password); secrets.add(account.token); }
  }
  // Installed app runs unchanged. The runner itself is disposable; userData is task-owned.
  const userData = join(root, 'electron-user-data'); await mkdir(userData, { recursive: true });
  await writeFile(join(userData, 'updater-preferences.json'), JSON.stringify({ automaticUpdates: false }));
  app = await electron.launch({ executablePath: executable, args: ['--user-data-dir=' + userData], env, timeout: 60000 });
  page = await app.firstWindow(); await page.waitForLoadState('domcontentloaded');
  const paths = await app.evaluate(({ app }) => ({ home: app.getPath('home'), userData: app.getPath('userData'), packaged: app.isPackaged, version: app.getVersion() }));
  assert(paths.packaged, 'Must execute installed packaged Electron');
  assert.equal(resolve(paths.userData), resolve(userData));
  home = paths.home; assert.equal(resolve(home), resolve(process.env.USERPROFILE));
  report.installedVersion = paths.version; report.installedExecutableSha256 = digest(await readFile(executable));
  report.profileBoundary = { home, userData: paths.userData, disposableRunner: true, ownerMarker: homeMarker };
  emit('installed_electron_main_preload_renderer', { version: paths.version });
  if (options.phase === 'baseline') {
    await page.locator('#runtime-address').waitFor({ state: 'visible', timeout: 45000 });
    await page.locator('#runtime-address').fill(proxy.origin);
    await page.getByRole('button', { name: /^(Save and continue|保存并继续)$/ }).click();
    await page.locator('#password-username').waitFor({ state: 'visible', timeout: 45000 });
    await page.locator('#password-username').fill(state.account.username);
    await page.locator('#password-value').fill(state.account.password);
    await page.locator('button[type="submit"]').click();
    await page.locator('#password-username').waitFor({ state: 'hidden', timeout: 45000 });
    emit('password_login_via_installed_ui');
  } else {
    await waitFor('retained authenticated session', async () => {
      assert(!(await page.locator('#password-username').isVisible()), 'Upgrade returned to login');
      const metadata = await page.evaluate(() => window.daemonAPI.getInstallationMetadata());
      return metadata?.userId === state.account.id;
    });
    emit('upgrade_retains_authenticated_session_without_login');
  }
  const session = await waitFor('installation enrollment', scope);
  const currentIdentity = await identity(session);
  const config = await json(join(home, '.multica/desktop.json'));
  if (options.phase === 'baseline') { state.identity = currentIdentity; state.config = config; }
  else { assert.deepEqual(currentIdentity, state.identity); assert.deepEqual(config, state.config); }
  emit('installation_identity_and_workspace_binding', currentIdentity);
  const metadata = await page.evaluate(() => window.daemonAPI.getInstallationMetadata());
  assert.equal(metadata.userId, state.account.id); assert.equal(metadata.serverUrl, proxy.origin);
  const installations = await api('/api/admin/installations', 'GET', undefined, state.operator);
  assert(JSON.stringify(installations).includes(currentIdentity.installation_id), 'Admin installation list must contain enrolled identity');
  emit('administrator_observes_installation');
  if (options.phase === 'baseline') {
    const runtime = await waitFor('synthetic runtime advertised', async () => {
      const response = await api('/api/runtimes/', 'GET', undefined, state.account, state.workspace);
      return (Array.isArray(response) ? response : response.runtimes).find(runtime => runtime.provider === 'claude' && runtime.daemon_id === session.value.managed_daemon_id);
    });
    state.agent = await api('/api/agents', 'POST', { name: 'Windows deterministic provider', runtime_id: runtime.id, permission_mode: 'private', instructions: 'Synthetic fixture only.', mcp_config: {} }, state.account, state.workspace);
    const work = await newTask('Retain completed task through upgrade');
    await writeFile(join(root, 'release-' + work.task.id), 'release');
    const completed = await waitFor('baseline task completion', async () => { const task = await taskFor(work.issue); return task?.status === 'completed' ? task : null; });
    state.retainedIssue = work.issue; state.retainedTaskId = completed.id;
    emit('baseline_task_completed', { task_id: completed.id });
  } else {
    const retained = await taskFor(state.retainedIssue); assert.equal(retained.id, state.retainedTaskId); assert.equal(retained.status, 'completed');
    emit('upgrade_retains_configuration_identity_workspace_and_completed_task');
    const pending = await waitFor('retained pending task starts', async () => { const task = await taskFor(state.pendingIssue); assert.equal(task.id, state.pendingTaskId); return task.status === 'running' ? task : null; });
    await waitFor('retained task provider starts', () => json(join(root, 'provider-' + pending.id + '.json')));
    await writeFile(join(root, 'release-' + pending.id), 'release');
    await waitFor('retained pending task completes', async () => (await taskFor(state.pendingIssue))?.status === 'completed');
    const starts = (await readFile(join(root, 'provider-starts.jsonl'), 'utf8')).trim().split('\n').map(JSON.parse).filter(event => event.task_id === pending.id);
    assert.equal(starts.length, 1, 'Retained pending task must launch exactly once');
    emit('upgrade_retains_pending_task_and_executes_same_task_once', { task_id: pending.id });
    const work = await newTask('Windows candidate successful task');
    await writeFile(join(root, 'release-' + work.task.id), 'release');
    await waitFor('candidate task completion', async () => (await taskFor(work.issue))?.status === 'completed');
    emit('candidate_task_result_returned', { task_id: work.task.id });
    const cancelled = await newTask('Windows candidate cancellation');
    const detail = await api('/api/admin/tasks/' + cancelled.task.id, 'GET', undefined, state.operator);
    assert(detail.execution_fence?.dispatched_at);
    const cancellation = await api('/api/admin/tasks/' + cancelled.task.id + '/cancel', 'POST', { reason: 'Windows acceptance cancellation', expected_execution_fence: detail.execution_fence }, state.operator, undefined, { 'Idempotency-Key': randomUUID() });
    await waitFor('confirmed cancellation', async () => { const operation = await api('/api/admin/operations/' + cancellation.operation.id, 'GET', undefined, state.operator); return operation.confirmation === 'confirmed' && operation.state === 'succeeded'; });
    const provider = await json(join(root, 'provider-' + cancelled.task.id + '.json'));
    await waitFor('cancelled provider process exit', async () => !(await processAlive(provider.pid)));
    emit('administrator_cancel_confirmed_and_provider_exited', { task_id: cancelled.task.id, operation_id: cancellation.operation.id });
    await waitFor('daemon websocket present', () => proxy.evidence.daemon_websocket_connections > 0);
    const before = proxy.evidence.daemon_websocket_connections;
    assert(proxy.disconnectWebSockets(true) > 0);
    await waitFor('real daemon transport reconnect', () => proxy.evidence.daemon_websocket_connections > before);
    const restored = await waitFor('reconnected management session', scope);
    assert.equal(restored.pid, session.pid); assert.deepEqual(await identity(restored), state.identity);
    emit('websocket_reconnect_preserves_process_and_identity');
  }
  await writeFile(stateFile, JSON.stringify(state), { mode: 0o600 });
  report.status = 'passed';
} catch (error) {
  report.status = 'failed'; report.error = redact(error.stack || error);
} finally {
  // Release only providers created by this private fixture; never ambient agent processes.
  for (const name of await readdir(root)) if (/^provider-[a-f0-9-]+\.json$/.test(name)) await writeFile(join(root, name.replace('provider-', 'release-').replace('.json', '')), 'release');
  try {
    if (page && !page.isClosed()) { const result = await page.evaluate(() => window.daemonAPI.stop()); assert.equal(result.success, true); }
    if (app) await app.close();
    for (const pid of daemonPids) await waitFor('owned daemon exit', async () => !(await processAlive(pid)), 45000);
    report.cleanup = 'owned application and observed daemons exited';
    if (options.phase === 'baseline' && report.status === 'passed') {
      state.pendingIssue = await api('/api/issues', 'POST', { title: 'Pending task retained through installed upgrade', status: 'todo', assignee_type: 'agent', assignee_id: state.agent.id }, state.account, state.workspace);
      const pending = await waitFor('offline pending task queued', async () => { const task = await taskFor(state.pendingIssue); return task && ['pending', 'queued'].includes(task.status) ? task : null; });
      await assert.rejects(lstat(join(root, 'provider-' + pending.id + '.json')), { code: 'ENOENT' });
      state.pendingTaskId = pending.id;
      await writeFile(stateFile, JSON.stringify(state), { mode: 0o600 });
      emit('baseline_pending_task_queued_while_daemon_stopped', { task_id: pending.id });
    }
    if (options.phase === 'verify') {
      assert.deepEqual(await json(homeMarker), { runId: state.runId, root });
      await rm(ownedHome, { recursive: true });
      report.ownedProfileRemovedAfterVerification = true;
    }
  } catch (error) { report.cleanup = redact(error.message); report.status = 'failed'; }
  await proxy.close();
  report.transport = proxy.evidence;
  report.finishedAt = new Date().toISOString();
  await mkdir(resolve(options.report, '..'), { recursive: true });
  await writeFile(options.report, JSON.stringify(report, null, 2));
}
if (report.status !== 'passed') { process.stderr.write(redact(report.error || report.cleanup) + '\n'); process.exitCode = 1; }
