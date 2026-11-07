import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import { scripts } from "./changelog-test-helpers.mjs";

const workflow = readFileSync(resolve(scripts, "../.github/workflows/release.yml"), "utf8");
// The workflow uses two-space job keys and scalar/inline-list needs. Parse that
// actual graph, so a helper test cannot hide a skipped upstream-only dependency.
const jobs = new Map();
for (const match of workflow.matchAll(/^ {2}([a-z][a-z0-9-]*):\n([\s\S]*?)(?=^ {2}[a-z][a-z0-9-]*:|$(?![\s\S]))/gm)) jobs.set(match[1], match[2]);
const needs = (name) => (/^ {4}needs: (.+)$/m.exec(jobs.get(name) ?? "")?.[1] ?? "").replace(/[[\]]/g, "").split(/[,\s]+/).filter(Boolean);

test("repository-wide serialization starts before discovery and never cancels an active release", () => {
  assert.match(workflow.split("\njobs:")[0], /concurrency:\n {2}group: changelog-release-\$\{\{ github.repository \}\}\n {2}cancel-in-progress: false/);
});

test("fork publication requires verified backend/frontend manifests but no upstream-only job", () => {
  const visited = new Set();
  function walk(name) {
    assert.ok(jobs.has(name), `missing job ${name}`);
    if (visited.has(name)) return;
    visited.add(name);
    for (const dependency of needs(name)) walk(dependency);
  }
  walk("publish-changelog");
  for (const job of ["verify", "changelog", "docker-backend-build", "docker-backend-merge", "docker-web-build", "docker-web-merge"]) assert.ok(visited.has(job), `${job} must gate publication`);
  for (const job of ["release", "desktop", "helm-chart"]) assert.equal(visited.has(job), false, `fork publication must not wait for skipped ${job}`);
  assert.doesNotMatch(jobs.get("publish-changelog"), /if:.*always\(|HOMEBREW|repository_owner == 'multica-ai'/);
  assert.match(jobs.get("publish-changelog"), /--verify "\$VERIFY_RESULT"/);
  assert.match(jobs.get("publish-changelog"), /--backend "\$BACKEND_RESULT"/);
  assert.match(jobs.get("publish-changelog"), /--web "\$WEB_RESULT"/);
});

test("one generated artifact reaches every consuming build before compilation", () => {
  assert.match(jobs.get("changelog") ?? "", /release-changelog\.mjs prepare/);
  assert.equal((workflow.match(/release-changelog\.mjs prepare/g) ?? []).length, 1);
  for (const name of ["docker-backend-build", "docker-web-build"]) {
    assert.ok(needs(name).includes("changelog"), `${name} must await feed generation`);
    const body = jobs.get(name);
    assert.match(body, /actions\/download-artifact@v4/);
    assert.match(body, /name: release-changelog/);
    assert.match(body, /cp .release-changelog\/changelog.json server\/internal\/changelog\/content\/changelog.json/);
    const copied = body.indexOf("cp .release-changelog/changelog.json");
    const build = body.indexOf("Build and push by digest");
    assert.ok(copied > 0 && copied < build);
    assert.doesNotMatch(body, /generate-changelog\.mjs|release-changelog\.mjs prepare/);
  }
  assert.match(jobs.get("release"), /if: github.repository_owner == 'multica-ai'/);
  assert.deepEqual(needs("release"), ["verify"]);
  assert.doesNotMatch(jobs.get("release"), /cp .release-changelog|--skip=validate/);
  assert.deepEqual(needs("desktop"), ["release"]);
});
