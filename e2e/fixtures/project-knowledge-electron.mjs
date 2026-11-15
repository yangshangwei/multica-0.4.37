import assert from "node:assert/strict";
import { app } from "electron";
import { isAbsolute } from "node:path";

for (const [name, value] of [["home", process.env.HOME], ["temp", process.env.TMPDIR]]) {
  assert(value && isAbsolute(value), `An explicit isolated ${name} is required`);
  app.setPath(name, value);
}
await import("./collaboration-account-electron.mjs");
