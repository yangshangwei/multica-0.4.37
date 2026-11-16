import assert from "node:assert/strict";
import { app, ipcMain } from "electron";
import { isAbsolute } from "node:path";

for (const [name, value] of [["home", process.env.HOME], ["temp", process.env.TMPDIR]]) {
  assert(value && isAbsolute(value), `An explicit isolated ${name} is required`);
  app.setPath(name, value);
}
await import("./collaboration-account-electron.mjs");

export function configuredKnowledgeRuntimeHandler(original) {
  assert.equal(typeof original, "function");
  return (event) => {
    let payload;
    let writes = 0;
    const capture = Object.defineProperty({}, "returnValue", {
      get: () => payload,
      set: (value) => { payload = value; writes += 1; },
    });
    // Capture the original payload before touching the real IPC event:
    // Electron can send a synchronous reply on its first returnValue setter.
    original(capture);
    assert.equal(writes, 1, "The known native fixture must produce one configuration payload");
    assert(payload && typeof payload === "object" && !Array.isArray(payload));
    assert.deepEqual(Object.keys(payload).sort(), ["config", "ok", "source"]);
    assert.equal(payload.ok, true);
    assert.equal(payload.source, "dev");
    const config = payload.config;
    assert(config && typeof config === "object" && !Array.isArray(config));
    assert.deepEqual(Object.keys(config).sort(), ["apiUrl", "appUrl", "schemaVersion", "wsUrl"]);
    assert.equal(config.schemaVersion, 1);
    assert.equal(typeof config.apiUrl, "string");
    assert.equal(typeof config.wsUrl, "string");
    assert.equal(config.appUrl, config.apiUrl, "P6 uses the API-only native configuration");
    event.returnValue = { ...payload, source: "configured" };
  };
}

const channel = "runtime-config:get";
const listeners = ipcMain.listeners(channel);
assert.equal(listeners.length, 1, "P6 must wrap exactly the known native configuration listener");
ipcMain.removeListener(channel, listeners[0]);
ipcMain.on(channel, configuredKnowledgeRuntimeHandler(listeners[0]));
