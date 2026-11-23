import assert from "node:assert/strict";
import { fileURLToPath, pathToFileURL } from "node:url";
import { nativeInstalledLayout } from "../../../../../../e2e/fixtures/collaboration-installed-layout.mjs";

const layout = nativeInstalledLayout("/task/Multica%20Candidate.app/Contents/MacOS/Multica", "darwin");
const path = `${layout.asar}/out/renderer/index.html`;
assert.notEqual(layout.rendererUrl, pathToFileURL(path).href);
assert.notEqual(fileURLToPath(layout.rendererUrl), path);
console.log(JSON.stringify({ actual: layout.rendererUrl, expected: pathToFileURL(path).href,
  decoded_actual: fileURLToPath(layout.rendererUrl), result: "literal percent escape is interpreted as decoded path content" }));
