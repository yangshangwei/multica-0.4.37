// This final probe injects a malformed successful read, never a token write.
await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
await navigate('profile');
const result = { recordedAt: new Date().toISOString(), fixture: 'Renderer fetch returns a real Response(200, JSON null) for GET /api/tokens; the real ApiClient parser is exercised', requests: 0, errors: [] };
// Electron's CDP fulfill reports status 0 in this harness. A narrowly scoped
// in-memory Response exercises the API parser with the intended success code.
await page.evaluate(() => {
  window.__auditTokenFetch = window.fetch;
  window.__auditTokenReads = 0;
  window.fetch = async function(input, init) {
    const url = new URL(input instanceof Request ? input.url : input, location.href);
    if (url.pathname === '/api/tokens' && (init?.method ?? 'GET') === 'GET') {
      window.__auditTokenReads++;
      return new Response('null', { status: 200, headers: { 'Content-Type': 'application/json' } });
    }
    return window.__auditTokenFetch.call(this, input, init);
  };
});
const onError = error => result.errors.push(error.message);
const onConsole = message => { if (message.type() === 'error') result.errors.push(message.text().slice(0, 600)); };
page.on('pageerror', onError);
page.on('console', onConsole);
try {
  await page.getByRole('tab', { name: 'API Token', exact: true }).click();
  await page.waitForTimeout(700);
  result.requests = await page.evaluate(() => window.__auditTokenReads);
  result.body = (await page.locator('body').innerText()).slice(-1600);
  result.remainingTokenPanel = await page.getByRole('tabpanel', { name: 'API Token', exact: true }).count();
  result.confirmed = result.requests > 0 && result.errors.some(error => /Cannot read properties of null.*length|tokens.*map/.test(error));
  await shot('resume-token-malformed-response');
  await fs.writeFile(path.join(dir, 'ax', 'resume-token-malformed-response.txt'), await page.locator('body').ariaSnapshot());
} catch (error) { result.probeError = error.message; }
finally {
  await page.evaluate(() => { window.fetch = window.__auditTokenFetch; delete window.__auditTokenFetch; delete window.__auditTokenReads; });
  page.off('pageerror', onError);
  page.off('console', onConsole);
  await page.reload();
  await expect(page.getByRole('tab', { name: '个人资料', exact: true })).toBeVisible({ timeout: 15000 });
  await navigate('profile');
  result.restored = await page.getByRole('textbox', { name: '姓名', exact: true }).isVisible();
}
await fs.writeFile(path.join(dir, 'probe-token-schema-results.json'), JSON.stringify(result, null, 2));
console.log(JSON.stringify({ ...result, body: result.body?.slice(-600) }));
