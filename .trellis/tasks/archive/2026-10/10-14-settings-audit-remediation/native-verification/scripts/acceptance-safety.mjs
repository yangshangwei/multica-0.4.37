// Durable renderer guard, complementary to the disposable main-process guard.
// Fixtures wrap this function and return only in-memory responses. Anything
// unrecognized falls through here and cannot perform a mutation or remote read.
export function installAcceptanceSafety() {
  if (window.__settingsAcceptanceSafety) return;
  const original = window.fetch;
  const state = { installedAt: new Date().toISOString(), blocked: [] };
  window.__settingsAcceptanceSafety = state;
  window.fetch = async function acceptanceFetch(input, init) {
    const url = new URL(input instanceof Request ? input.url : String(input), location.href);
    const method = String(init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase();
    const local = ['localhost', '127.0.0.1'].includes(url.hostname)
      && ['5666', '18572'].includes(url.port);
    if (!local || !['GET', 'HEAD', 'OPTIONS'].includes(method)) {
      state.blocked.push({ method, origin: url.origin, path: url.pathname });
      throw new Error('Native acceptance blocked an unmocked mutation or external request');
    }
    return original.call(this, input, init);
  };
}
