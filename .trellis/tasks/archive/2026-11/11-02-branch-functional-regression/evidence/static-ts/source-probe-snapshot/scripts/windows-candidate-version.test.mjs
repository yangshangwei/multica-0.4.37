import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

const workflow = readFileSync(new URL("../.github/workflows/desktop-smoke.yml", import.meta.url), "utf8").replace(/\r\n/g, "\n");
const desktopRoot = fileURLToPath(new URL("../apps/desktop/", import.meta.url));
const acceptance = workflow.split("\n  windows-acceptance:\n")[1];

function step(name) {
  const body = acceptance.split(`      - name: ${name}\n`)[1]?.split("      - name: ")[0];
  assert.ok(body, `Missing Windows acceptance step: ${name}`);
  return body;
}

test("Windows acceptance offers an optional candidate version without changing the unique RC default", () => {
  const inputs = workflow.split("\npermissions:")[0];
  assert.match(inputs, /      candidate_version:\n(?: {8}[^\n]*\n)* {8}type: string\n/);
  assert.match(inputs, /      candidate_version:\n(?: {8}[^\n]*\n)* {8}default: ""\n/);
  assert.match(acceptance, /MULTICA_DESKTOP_VERSION: \$\{\{ inputs\.candidate_version \|\| format\('0\.5\.3-rc\.\{0\}\.\{1\}', github\.run_number, github\.run_attempt\) \}\}/);
});

test("Windows acceptance validates and normalizes the environment value before packaging", () => {
  const validation = step("Validate candidate version");
  assert.match(validation, /working-directory: apps\/desktop/);
  assert.match(validation, /shell: pwsh/);
  assert.ok(acceptance.indexOf("- name: Validate candidate version") < acceptance.indexOf("- name: Package Windows x64 candidate"));
  assert.match(validation, /\$LASTEXITCODE -ne 0/);
  assert.match(validation, /"MULTICA_DESKTOP_VERSION=\$version" >> \$env:GITHUB_ENV/);
  assert.doesNotMatch(validation, /\$\{\{/);

  const code = validation.match(/node --input-type=module -e '([^']+)'/)?.[1];
  assert.ok(code, "Version validation must execute the packaging validator through Node");
  assert.match(code, /desktopVersionOverride/);
  for (const [input, expected] of [["0.6.0", "0.6.0"], ["v0.6.0", "0.6.0"], ["0.5.3-rc.42.2", "0.5.3-rc.42.2"]]) {
    const result = spawnSync(process.execPath, ["--input-type=module", "-e", code], {
      cwd: desktopRoot,
      env: { ...process.env, MULTICA_DESKTOP_VERSION: input },
      encoding: "utf8",
    });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, expected);
  }
  for (const input of ["", " ", "0.6.0\n", "0.6.0;echo injected", "0.6.0$(echo injected)", "0.6.0-01"]) {
    const result = spawnSync(process.execPath, ["--input-type=module", "-e", code], {
      cwd: desktopRoot,
      env: { ...process.env, MULTICA_DESKTOP_VERSION: input },
      encoding: "utf8",
    });
    assert.notEqual(result.status, 0, `Invalid candidate ${JSON.stringify(input)} must stop packaging`);
    assert.equal(result.stdout, "");
  }
});

test("packaging, installed lifecycle and updater acceptance use the same validated version", () => {
  assert.match(step("Package Windows x64 candidate"), /node scripts\/package\.mjs --win --x64 --publish never/);
  assert.doesNotMatch(step("Package Windows x64 candidate"), /env:/);
  const lifecycle = step("Accept Windows installer lifecycle");
  assert.match(lifecycle, /\$version = \$env:MULTICA_DESKTOP_VERSION/);
  assert.match(lifecycle, /-ExpectedVersion \$version/);
  assert.match(step("Accept installed Windows automatic update"), /-ExpectedVersion \$env:MULTICA_DESKTOP_VERSION/);
  assert.match(step("Validate acceptance scripts"), /node --test scripts\/windows-candidate-version\.test\.mjs/);
});
