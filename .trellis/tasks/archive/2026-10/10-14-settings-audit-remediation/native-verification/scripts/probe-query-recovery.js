await queryClient();
await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
const results = { recordedAt: new Date().toISOString(), cases: [] };
const cases = [
  { tab: 'labels', family: 'labels', suffix: '/api/labels' },
  { tab: 'properties', family: 'properties', suffix: '/api/properties' },
  { tab: 'quick-actions', family: 'quick-actions', suffix: '/api/quick-actions' },
  { tab: 'issue-statuses', family: 'issue-statuses', suffix: '/api/issue-statuses' },
  { tab: 'members', family: 'members', suffix: '/members' },
];
for (const current of cases) {
  const result = { case: `cold-${current.tab}`, fixture: 'HTTP 500 on a read-only GET', requests: 0 };
  const matches = key => current.family === 'members' ? key[0] === 'workspaces' && key[2] === 'members' : key[0] === current.family;
  const pattern = '**/api/**';
  const handler = async route => {
    if (route.request().method() === 'GET' && new URL(route.request().url()).pathname.endsWith(current.suffix)) {
      result.requests++;
      await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'Settings audit: injected read failure' }) });
    } else await route.continue();
  };
  try {
    await navigate('profile');
    await page.evaluate(async family => {
      const matches = q => family === 'members' ? q.queryKey[0] === 'workspaces' && q.queryKey[2] === 'members' : q.queryKey[0] === family;
      await window.__auditQC.cancelQueries({ predicate: matches });
      window.__readAuditOriginal = window.__auditQC.getQueryCache().getAll().filter(matches).map(q => ({ key: q.queryKey, data: q.state.data }));
      window.__auditQC.removeQueries({ predicate: matches });
    }, current.family);
    await context.route(pattern, handler);
    await navigate(current.tab);
    await page.waitForFunction(family => window.__auditQC.getQueryCache().getAll().some(q =>
      (family === 'members' ? q.queryKey[0] === 'workspaces' && q.queryKey[2] === 'members' : q.queryKey[0] === family) && q.state.status === 'error',
    ), current.family, { timeout: 20000 });
    const panel = page.getByRole('tabpanel').first();
    result.panel = await panel.innerText();
    result.alerts = await panel.getByRole('alert').count();
    result.retryButtons = await panel.getByRole('button', { name: /重试|重新加载/ }).count();
    result.queries = await page.evaluate(family => window.__auditQC.getQueryCache().getAll().filter(q =>
      family === 'members' ? q.queryKey[0] === 'workspaces' && q.queryKey[2] === 'members' : q.queryKey[0] === family,
    ).map(q => ({ key: q.queryKey, status: q.state.status, hasData: q.state.data !== undefined })), current.family);
    result.findingConfirmed = result.requests > 0 && result.alerts === 0 && result.retryButtons === 0;
    await shot(`resume-error-${current.tab}`);
    await fs.writeFile(path.join(dir, 'ax', `resume-error-${current.tab}.txt`), await page.locator('body').ariaSnapshot());
    if (current.tab === 'members') {
      await navigate('workspace');
      await page.waitForTimeout(300);
      result.workspaceFallback = await page.getByRole('tabpanel').first().innerText();
      result.workspaceEnabledButtons = await page.getByRole('tabpanel').first().getByRole('button').evaluateAll(buttons => buttons.map(button => ({ name: button.textContent.trim(), disabled: button.disabled })));
      await shot('resume-workspace-member-read-error');
    }
  } catch (error) { result.probeError = error.message; }
  finally {
    await context.unroute(pattern, handler);
    await page.evaluate(async family => {
      const matches = q => family === 'members' ? q.queryKey[0] === 'workspaces' && q.queryKey[2] === 'members' : q.queryKey[0] === family;
      await window.__auditQC.cancelQueries({ predicate: matches });
      for (const saved of window.__readAuditOriginal ?? []) {
        if (saved.data !== undefined) window.__auditQC.setQueryData(saved.key, saved.data);
      }
      delete window.__readAuditOriginal;
    }, current.family);
    await navigate('profile');
  }
  results.cases.push(result);
  await fs.writeFile(path.join(dir, 'probe-query-recovery-results.json'), JSON.stringify(results, null, 2));
  console.log(JSON.stringify({ case: result.case, requests: result.requests, confirmed: result.findingConfirmed, probeError: result.probeError }));
}

// A cached MCP name draft must survive a transient list refresh failure.
const hot = { case: 'mcp-cached-refresh', fixture: 'HTTP 500 on MCP GET with existing cache', requests: 0 };
const pattern = '**/api/**';
const handler = async route => {
  if (route.request().method() === 'GET' && new URL(route.request().url()).pathname.endsWith('/mcp-servers')) {
    hot.requests++;
    await route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"Settings audit: MCP refresh failed"}' });
  } else await route.continue();
};
try {
  await navigate('mcp');
  const rename = page.getByRole('button', { name: /^重命名 MCP Server/ }).first();
  await rename.click();
  const input = page.getByRole('textbox').first();
  await input.fill('Audit unsaved MCP name');
  await input.evaluate(el => { window.__mcpAuditInput = el; });
  await page.evaluate(() => {
    const q = window.__auditQC.getQueryCache().getAll().find(q => q.queryKey[0] === 'workspaces' && q.queryKey[2] === 'mcp-servers');
    window.__mcpAuditOriginal = { key: q.queryKey, data: q.state.data };
  });
  await shot('resume-mcp-draft-before-refresh');
  await context.route(pattern, handler);
  await page.evaluate(() => window.__auditQC.invalidateQueries({ queryKey: window.__mcpAuditOriginal.key }));
  await page.waitForFunction(() => window.__auditQC.getQueryState(window.__mcpAuditOriginal.key)?.status === 'error');
  hot.after = await page.evaluate(() => ({
    retainedCache: window.__auditQC.getQueryData(window.__mcpAuditOriginal.key)?.length,
    inputConnected: window.__mcpAuditInput.isConnected,
    inputFocused: document.activeElement === window.__mcpAuditInput,
    draftBeforeRemoval: window.__mcpAuditInput.value,
  }));
  hot.panel = await page.getByRole('tabpanel').first().innerText();
  hot.findingConfirmed = hot.after.retainedCache > 0 && !hot.after.inputConnected;
  await shot('resume-mcp-draft-refresh-error');
  await fs.writeFile(path.join(dir, 'ax', 'resume-mcp-draft-refresh-error.txt'), await page.locator('body').ariaSnapshot());
} catch (error) { hot.probeError = error.message; }
finally {
  await context.unroute(pattern, handler);
  await page.evaluate(() => {
    if (window.__mcpAuditOriginal) window.__auditQC.setQueryData(window.__mcpAuditOriginal.key, window.__mcpAuditOriginal.data);
    delete window.__mcpAuditOriginal;
    delete window.__mcpAuditInput;
  });
  await navigate('profile');
}
results.cases.push(hot);
await fs.writeFile(path.join(dir, 'probe-query-recovery-results.json'), JSON.stringify(results, null, 2));
console.log(JSON.stringify({ case: hot.case, requests: hot.requests, confirmed: hot.findingConfirmed, probeError: hot.probeError }));
