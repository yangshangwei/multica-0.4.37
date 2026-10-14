// Shared by task-local acceptance probes only. No product code imports this.
export async function beginFixture(page, navigate, queryClient) {
  await navigate('profile');
  await queryClient();
  await page.evaluate(async () => {
    if (window.__nativeAcceptanceFixture) throw new Error('A prior native fixture is still active');
    const url = performance.getEntriesByType('resource').map(entry => entry.name)
      .find(name => /packages\/core\/api\/index\.ts(?:\?|$)/.test(name));
    if (!url) throw new Error('Core API module is not loaded');
    const api = (await import(url)).getApi();
    const qc = window.__auditQC;
    const workspaces = qc.getQueryData(['workspaces', 'list']);
    const settingsLink = document.querySelector('[data-sidebar="menu-button"][href$="/settings"]');
    const parts = settingsLink ? new URL(settingsLink.href).pathname.split('/').filter(Boolean) : [];
    const slug = parts.length > 1 ? decodeURIComponent(parts.at(-2)) : null;
    const workspace = workspaces?.find(item => item.slug === slug);
    if (!workspace) throw new Error('Active workspace cannot be identified');
    const state = {
      api, qc, workspace, workspaces, fetch: window.fetch,
      methods: new Map(), queries: new Map(), routes: new Map(), reads: [],
      theme: document.documentElement.classList.contains('dark'),
      scheme: document.documentElement.style.colorScheme,
    };
    state.mock = (key, value) => {
      if (!state.methods.has(key)) state.methods.set(key, Object.getOwnPropertyDescriptor(api, key));
      Object.defineProperty(api, key, { configurable: true, writable: true, value });
    };
    state.remember = key => {
      const hash = JSON.stringify(key);
      if (state.queries.has(hash)) return;
      const query = qc.getQueryCache().find({ queryKey: key, exact: true });
      state.queries.set(hash, { key, query, options: query?.options, state: query ? { ...query.state } : null });
    };
    state.seed = async (key, data) => {
      state.remember(key);
      await qc.cancelQueries({ queryKey: key, exact: true });
      qc.setQueryData(key, data);
    };
    state.clear = async key => {
      state.remember(key);
      await qc.cancelQueries({ queryKey: key, exact: true });
      qc.removeQueries({ queryKey: key, exact: true });
    };
    window.fetch = async function nativeFixtureFetch(input, init) {
      const url = new URL(input instanceof Request ? input.url : String(input), location.href);
      const method = String(init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase();
      const route = state.routes.get(`${method} ${url.pathname}`);
      if (!route) return state.fetch.call(this, input, init);
      const response = await route(input, init);
      state.reads.push({ method, path: url.pathname, status: response.status });
      return response;
    };
    window.__nativeAcceptanceFixture = state;
  });
}

export async function endFixture(page, navigate) {
  await navigate('profile');
  return page.evaluate(async () => {
    const state = window.__nativeAcceptanceFixture;
    if (!state) return { restored: false, error: 'Missing fixture' };
    for (const saved of state.queries.values()) {
      await state.qc.cancelQueries({ queryKey: saved.key, exact: true });
    }
    for (const [key, descriptor] of state.methods) {
      if (descriptor) Object.defineProperty(state.api, key, descriptor);
      else delete state.api[key];
    }
    for (const saved of state.queries.values()) {
      const query = state.qc.getQueryCache().find({ queryKey: saved.key, exact: true });
      if (saved.state) {
        if (query) query.setState(saved.state);
        else state.qc.getQueryCache().build(state.qc, saved.options, saved.state);
      } else state.qc.removeQueries({ queryKey: saved.key, exact: true });
    }
    window.fetch = state.fetch;
    document.documentElement.classList.toggle('dark', state.theme);
    document.documentElement.style.colorScheme = state.scheme;
    delete window.__nativeAcceptanceFixture;
    return { restored: true, methods: state.methods.size, queries: state.queries.size, safetyPresent: !!window.__settingsAcceptanceSafety };
  });
}

export async function captureEvidence({ page, cdp, shot, fs, path, dir }, name) {
  const screenshot = await shot(name);
  const ax = path.join(dir, 'ax', `${name}.txt`);
  await fs.writeFile(ax, await page.locator('body').ariaSnapshot());
  await fs.writeFile(path.join(dir, 'ax', `${name}.json`), JSON.stringify(await cdp.send('Accessibility.getFullAXTree'), null, 2));
  return { screenshot, ax };
}
