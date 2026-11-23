// Reuse the original audit exercises without overwriting their evidence.
// Assertions below explicitly describe corrected behavior; old "finding"
// booleans alone are not treated as proof of success.
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

const scripts = import.meta.dirname;
const audit = path.resolve('.impeccable/audit/2026-10-13-settings');
const output = path.resolve(scripts, '..');
for (const name of ['screenshots', 'ax', 'fixtures']) fs.mkdirSync(path.join(output, name), { recursive: true });
fs.copyFileSync(path.join(audit, 'screenshots/profile-light-wide.png'), path.join(output, 'fixtures/avatar-source.png'));
const allExercises = [
  'probe-status-loop.js', 'probe-query-recovery.js', 'probe-configuration.js',
  'probe-personal.js', 'probe-integrations.js', 'probe-final-evidence.js', 'probe-token-schema.js',
];
const requested = process.argv.slice(2);
const exercises = requested.length ? allExercises.filter(file => requested.includes(file)) : allExercises;
if (!exercises.length) throw new Error('No known remediation exercise selected');
const run = { recordedAt: new Date().toISOString(), exercises: [], checks: [] };
for (const name of exercises) {
  const body = fs.readFileSync(path.join(scripts, name), 'utf8')
    .replaceAll("path.resolve(dir, '../2026-10-11-desktop-core/scripts/contrast.mjs')", "path.resolve(auditDir, '../2026-10-11-desktop-core/scripts/contrast.mjs')")
    .replaceAll("'screenshots/profile-light-wide.png'", "'fixtures/avatar-source.png'")
    .replaceAll("panel.locator('.text-warning')", "panel.locator('.text-warning, .text-warning-foreground')")
    .replaceAll(String.raw`text-warning(?:\s|$)`, String.raw`text-warning(?:-foreground)?(?:\s|$)`);
  const child = spawnSync(process.execPath, [path.join(scripts, 'resume-run.mjs')], {
    input: body, encoding: 'utf8', timeout: 180000,
    env: { ...process.env, SETTINGS_AUDIT_OUTPUT_DIR: output, SETTINGS_AUDIT_CASE: '' },
    maxBuffer: 8 * 1024 * 1024,
  });
  const entry = { name, exitCode: child.status, error: child.error?.message };
  run.exercises.push(entry);
  fs.writeFileSync(path.join(output, `${name}.log`), `${child.stdout ?? ''}${child.stderr ?? ''}`);
  console.log(JSON.stringify(entry));
  fs.writeFileSync(path.join(output, 'run.json'), JSON.stringify(run, null, 2));
  if (child.error || child.status !== 0) break;
}

const read = name => JSON.parse(fs.readFileSync(path.join(output, name), 'utf8'));
const check = (id, ok, evidence) => run.checks.push({ id, status: ok ? 'pass' : 'fail', evidence });
const completed = name => run.exercises.some(item => item.name === name && item.exitCode === 0);

if (completed('probe-status-loop.js')) {
  const value = read('probe-status-loop-response500.json');
  check('F01-bounded-status-recovery', value.reads.length > 0 && value.reads.length <= 2 && value.reads.every(item => item.status === 500) && /重试|Retry/.test(value.panel), 'probe-status-loop-response500.json');
}
if (completed('probe-query-recovery.js')) {
  const value = read('probe-query-recovery-results.json');
  for (const item of value.cases.filter(item => item.case.startsWith('cold-'))) {
    check(`F02-${item.case}`, !item.probeError && item.requests > 0 && item.alerts > 0 && item.retryButtons > 0, 'probe-query-recovery-results.json');
    if (item.case === 'cold-members') check('F02-unknown-membership-leave', (item.workspaceEnabledButtons ?? []).filter(button => /离开|Leave/.test(button.name)).every(button => button.disabled), 'probe-query-recovery-results.json');
  }
  const mcp = value.cases.find(item => item.case === 'mcp-cached-refresh');
  check('F08-retained-MCP-input', !mcp?.probeError && mcp?.after?.retainedCache > 0 && mcp.after.inputConnected && mcp.after.inputFocused, 'probe-query-recovery-results.json');
}
if (completed('probe-configuration.js')) {
  const value = read('probe-configuration-results.json');
  const geometry = value.cases.find(item => item.id === 'property-dialog-geometry');
  check('F04-bounded-property-dialog', geometry?.status === 'captured' && geometry.observations.length === 2 && geometry.observations.every(item => {
    const viewport = item.beforeFocus.viewport, box = item.beforeFocus.root.bounds;
    return box.top >= -1 && box.bottom <= viewport.height + 1 && item.focusProbes.every(probe => {
      const control = probe.focusedControl?.bounds;
      return control && control.top >= -1 && control.bottom <= viewport.height + 1 && control.left >= -1 && control.right <= viewport.width + 1;
    });
  }), 'probe-configuration-results.json');
  const accessibility = value.cases.find(item => item.id === 'configuration-accessible-names');
  const recorders = accessibility?.observations.find(item => item.section === 'shortcuts-current-values');
  check('F06-shortcut-current-values', recorders?.recorders.length > 0 && recorders.recorders.filter(item => item.keycapsHidden === 'true').every(item => {
    const node = recorders.ax.find(node => node.name?.value === item.name);
    return !!node?.description?.value?.trim() || (item.keyTitles.length > 0 && item.keyTitles.every(key => item.name.includes(key)));
  }), 'probe-configuration-results.json');
  for (const section of ['property-type', 'status-category']) {
    const item = accessibility?.observations.find(item => item.section === section);
    check(`F05-${section}`, !item?.error && item?.ax.length > 0 && item.ax.every(node => node.name?.value?.trim()), 'probe-configuration-results.json');
  }
  const contrast = value.cases.find(item => item.id === 'quick-action-warning-contrast');
  check('F07-warning-contrast', contrast?.status === 'captured' && contrast.observations.length === 2 && contrast.observations.every(item => item.measurements.length > 0 && item.measurements.every(sample => sample.ratio >= sample.floor)), 'probe-configuration-results.json');
}
if (completed('probe-personal.js')) {
  const value = read('probe-personal-results.json');
  const avatar = value.cases.find(item => item.id === 'avatar-failure');
  check('F09-avatar-failure-retained', avatar?.status === 'product_behavior' && avatar.mocksRestored && avatar.observed?.dialogOpen && !avatar.observed.freshToasts.some(item => item.type === 'success'), 'probe-personal-results.json');
  const name = value.cases.find(item => item.id === 'empty-name');
  check('F10-name-validation', name?.status === 'product_behavior' && name.mocksRestored && name.observed?.validation.invalid === 'true' && name.observed.draftRetainedBeforeNavigation && (name.observed.statuses.length > 0 || name.observed.validationHints.length > 0), 'probe-personal-results.json');
  const invite = value.cases.find(item => item.id === 'invite-repeat-enter');
  check('F11-single-pending-invite', invite?.status === 'product_behavior' && invite.mocksRestored && invite.observed?.requestCount === 1, 'probe-personal-results.json');
}
if (completed('probe-integrations.js')) {
  const value = read('probe-integrations-results.json');
  const token = value.cases.find(item => item.name === 'token-unconfirmed-dismissal');
  check('F12-token-dismissal-acknowledgement', token?.status === 'not-reproduced' && token.dismissals.length === 2 && token.dismissals.every(item => !item.dismissed) && token.cleanupErrors.length === 0, 'probe-integrations-results.json');
  const github = value.cases.find(item => item.name === 'github-second-toggle-during-save');
  check('F13-github-pending-controls', github?.status === 'not-reproduced' && github.secondWasEnabled === false && github.callsAfterSettlement.length === 1 && github.cleanupErrors.length === 0, 'probe-integrations-results.json');
  const contrast = value.cases.find(item => item.name === 'integration-connected-badge-contrast');
  check('F07-connected-contrast', contrast?.status === 'not-reproduced' && contrast.themes.length === 2 && contrast.themes.every(item => item.measurements.length > 0 && item.failures.length === 0), 'probe-integrations-results.json');
}
if (completed('probe-final-evidence.js')) {
  const value = read('probe-final-evidence-results.json');
  for (const name of ['members','daemon','behavior']) {
    const item = value.cases.find(item => item.case === `${name}-accessible-names`);
    check(`F05-${name}-named-controls`, !!item && item.unnamed.length === 0, 'probe-final-evidence-results.json');
  }
  const update = value.cases.find(item => item.case === 'update-result-announcement');
  check('F17-update-announcement', !!update?.announcementAncestor, 'probe-final-evidence-results.json');
  const menu = value.cases.find(item => item.case === 'label-menu-clipping');
  check('F03-visible-label-menu', menu?.before.x >= 0 && menu.before.right <= menu.before.viewport && menu.before.cardScrollWidth <= menu.before.cardWidth + 1, 'probe-final-evidence-results.json');
  const mcp = value.cases.find(item => item.case === 'mcp-200pct-completed');
  check('MCP-200pct-layout', mcp?.width === 450 && mcp.documentWidth <= 450 && mcp.contrastFailures.length === 0, 'probe-final-evidence-results.json');
}
if (completed('probe-token-schema.js')) {
  const value = read('probe-token-schema-results.json');
  check('F16-malformed-token-response', !value.probeError && value.requests > 0 && value.remainingTokenPanel === 1 && value.restored && !value.errors.some(error => /TypeError|Cannot read properties of null/.test(error)), 'probe-token-schema-results.json');
}
run.completedAt = new Date().toISOString();
run.status = run.exercises.length === exercises.length && run.exercises.every(item => item.exitCode === 0) && run.checks.every(item => item.status === 'pass') ? 'pass' : 'fail';
fs.writeFileSync(path.join(output, requested.length ? 'acceptance-focused.json' : 'acceptance.json'), JSON.stringify(run, null, 2));
console.log(JSON.stringify({ status: run.status, checks: run.checks.length, failed: run.checks.filter(item => item.status !== 'pass').map(item => item.id) }));
process.exitCode = run.status === 'pass' ? 0 : 1;
