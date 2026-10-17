// Verify the retry loop with an actual 500 Response at the renderer fetch
// boundary. This avoids the status-0 quirk of CDP fulfill in this Electron.
await control('/viewport', { width: 1440, height: 1000, zoom: 1 });
await navigate('profile');
await queryClient();
const result = { recordedAt: new Date().toISOString(), fixture: 'In-memory fetch Response(500), real ApiClient and Query observers' };
try {
  await page.evaluate(async () => {
    const qc = window.__auditQC, predicate = q => q.queryKey[0] === 'issue-statuses';
    await qc.cancelQueries({ predicate });
    const state = { fetch: window.fetch, reads: [], trace: [], saved: qc.getQueryCache().getAll().filter(predicate).map(q => ({ key: q.queryKey, data: q.state.data })) };
    window.__status500Audit = state;
    window.fetch = async function(input, init) {
      const url = new URL(input instanceof Request ? input.url : input, location.href);
      if (url.pathname === '/api/issue-statuses' && (init?.method ?? 'GET') === 'GET') {
        const response = new Response('{"error":"Audit: status catalog unavailable"}', { status: 500, headers: { 'Content-Type': 'application/json' } });
        state.reads.push({ time: performance.now(), status: response.status });
        return response;
      }
      return state.fetch.call(this, input, init);
    };
    qc.removeQueries({ predicate });
    state.unsubscribe = qc.getQueryCache().subscribe(event => {
      if (predicate(event.query)) state.trace.push({ time: performance.now(), type: event.type, action: event.action?.type, status: event.query.state.status, fetch: event.query.state.fetchStatus, observers: event.query.getObserversCount() });
    });
  });
  await navigate('issue-statuses');
  await page.waitForTimeout(5100);
  Object.assign(result, await page.evaluate(() => ({ reads: window.__status500Audit.reads, trace: window.__status500Audit.trace })));
  result.panel = await page.getByRole('tabpanel').first().innerText();
  result.confirmed = result.reads.length > 4 && result.reads.every(read => read.status === 500);
  result.intervalMs = result.reads.at(-1).time - result.reads[0].time;
  await shot('resume-status-500-retry-loop');
  await fs.writeFile(path.join(dir, 'ax', 'resume-status-500-retry-loop.txt'), await page.locator('body').ariaSnapshot());
} finally {
  await page.evaluate(async () => {
    const state = window.__status500Audit;
    if (!state) return;
    state.unsubscribe?.();
    window.fetch = state.fetch;
    await window.__auditQC.cancelQueries({ predicate: q => q.queryKey[0] === 'issue-statuses' });
    for (const saved of state.saved) if (saved.data !== undefined) window.__auditQC.setQueryData(saved.key, saved.data);
    delete window.__status500Audit;
  });
  await navigate('profile');
}
await fs.writeFile(path.join(dir, 'probe-status-loop-response500.json'), JSON.stringify(result, null, 2));
console.log(JSON.stringify({ confirmed: result.confirmed, requests: result.reads.length, intervalMs: result.intervalMs }));
