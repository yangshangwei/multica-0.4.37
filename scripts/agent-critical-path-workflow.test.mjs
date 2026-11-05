import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { createRequire } from "node:module";

// yaml is already a direct dependency of the shared core package; use that
// package's resolution instead of adding a duplicate root dependency.
const require = createRequire(new URL("../packages/core/package.json", import.meta.url));
const { parse } = require("yaml");
const text = readFileSync(new URL("../.github/workflows/agent-critical-path.yml", import.meta.url), "utf8");
const workflow = parse(text);

test("PR smoke covers integration/main and every shared source/config boundary", () => {
  assert.deepEqual(workflow.on.pull_request.branches, ["main", "codex/cross-workspace-collaboration"]);
  assert.equal(workflow.on.pull_request_target, undefined);
  for (const path of ["server/**", "apps/web/**", "packages/**", "e2e/**", "scripts/agent-critical-path*", "scripts/check-agent-critical-path*",
    "scripts/dev-env.sh", "scripts/local-env.sh", "playwright*.config.*", "pnpm-lock.yaml", ".github/workflows/agent-critical-path.yml"]) {
    assert.ok(workflow.on.pull_request.paths.includes(path), path);
  }
  assert.deepEqual(workflow.permissions, { contents: "read" });
  assert.ok(!text.includes("secrets."));
});

test("one isolated service plus locked browser dependencies precede the actual launcher", () => {
  const job = workflow.jobs.smoke;
  assert.equal(job.name, "agent-critical-path");
  assert.equal(job["timeout-minutes"], 30);
  assert.deepEqual(Object.keys(job.services), ["postgres"]);
  assert.equal(job.services.postgres.image, "pgvector/pgvector:pg17");
  assert.equal(job.services.postgres.env.POSTGRES_DB, "agent_smoke");
  const steps = job.steps;
  const install = steps.findIndex((s) => s.name === "Install locked dependencies and Chromium");
  const contracts = steps.findIndex((s) => s.name === "Verify smoke contracts");
  const allocation = steps.findIndex((s) => s.name === "Record runner-owned allocation");
  const run = steps.findIndex((s) => s.name === "Run four production-Web paths without a model");
  assert.ok(install < contracts && contracts < allocation && allocation < run);
  assert.match(steps[install].run, /pnpm install --frozen-lockfile/);
  assert.match(steps[install].run, /playwright install --with-deps chromium/);
  assert.match(steps[allocation].env.SMOKE_PG_PORT, /job\.services\.postgres\.ports\[5432\]/);
  assert.equal(steps[run].run, 'node scripts/agent-critical-path.mjs --config "$SMOKE_CONFIG" --output "$SMOKE_RUN"');
  assert.equal(steps[run]["continue-on-error"], undefined);
});

test("evidence upload always runs and excludes private configuration/registry", () => {
  const upload = workflow.jobs.smoke.steps.at(-1);
  assert.equal(upload.if, "always()");
  assert.equal(upload.with.path, "${{ runner.temp }}/agent-smoke-run/evidence/");
  assert.equal(upload.with["if-no-files-found"], "error");
  assert.ok(!upload.with.path.includes("registry"));
  assert.ok(!upload.with.path.includes("allocation"));
});
