import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const workflow = readFileSync(new URL("../../../.github/workflows/desktop-smoke.yml", import.meta.url), "utf8");
const lifecycle = readFileSync(new URL("./accept-windows-installer.ps1", import.meta.url), "utf8");
const server = readFileSync(new URL("./windows-collaboration-server.ps1", import.meta.url), "utf8");
const backend = readFileSync(new URL("./windows-business-server.ps1", import.meta.url), "utf8");

// Static integration contracts, not a parallel YAML parser. The workflow engine
// and the coordinator's existing YAML tooling own actual document parsing.
test("installed collaboration is a separate opt-in selecting only the Windows acceptance job", () => {
  const input = workflow.match(/ {6}windows_collaboration_acceptance:\n(?: {8}.*\n)+/)[0];
  assert.match(input, /type: boolean/); assert.match(input, /default: false/);
  assert.match(workflow, /if: inputs.windows_acceptance != true && inputs.windows_collaboration_acceptance != true/);
  assert.match(workflow, /if: inputs.windows_acceptance == true \|\| inputs.windows_collaboration_acceptance == true/);
});

test("prepares and always stops the same private collaboration services without uploading state", () => {
  assert.match(workflow, /name: Prepare isolated installed collaboration services\n {8}if: inputs.windows_collaboration_acceptance == true/);
  assert.match(workflow, /name: Stop isolated installed collaboration services\n {8}if: always\(\) && inputs.windows_collaboration_acceptance == true/);
  assert.match(workflow, /windows-collaboration-server.ps1 -StateDirectory "\$env:RUNNER_TEMP\/multica-collaboration" -RepoRoot "\$env:GITHUB_WORKSPACE" -Stop/);
  const upload = workflow.split("      - name: Upload native Windows evidence\n")[1].split("      - name:")[0];
  assert.match(upload, /multica-windows-collaboration-\*\.json/);
  assert(!upload.includes("server-private") && !upload.includes("web-private") && !upload.includes("multica-collaboration/"));
});

test("runs collaboration only after candidate installs and compiles the Host without TestMain", () => {
  const hook = lifecycle.match(/if \(\$CollaborationAcceptance[^\n]+/)[0];
  assert(hook.includes("upgrade_candidate") && hook.includes("reinstall_candidate"));
  assert(!hook.includes("install_previous"));
  assert.match(lifecycle, /\[switch\]\$CollaborationAcceptance/);
  assert.match(server, /go test -c -o \$daemon \.\/internal\/daemon/);
  assert(!server.includes("-run '^$'"));
  assert.match(server, /Explicit task database identity/);
  assert(backend.indexOf("SELECT current_database()") < backend.indexOf("go run ./cmd/migrate up"));
  assert(backend.indexOf("current_setting('data_directory')") < backend.indexOf("go run ./cmd/migrate up"));
});
