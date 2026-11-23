import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { criticalPathCases, validateCriticalPathReport } from "./check-agent-critical-path.mjs";

function report() {
  return {
    config: { workers: 1, projects: [{ id: "chromium", name: "chromium", retries: 0, repeatEach: 1 }] },
    errors: [],
    stats: { expected: 4, skipped: 0, unexpected: 0, flaky: 0 },
    suites: [{ specs: [], suites: criticalPathCases.map(({ file, title }, i) => ({
      specs: [{ file, title, id: `case-${i}`, ok: true, tests: [{
        projectId: "chromium", projectName: "chromium", expectedStatus: "passed", status: "expected",
        results: [{ status: "passed", retry: 0, errors: [] }],
      }] }],
    })) }],
  };
}

test("four canonical original cases pass with exactly one Chromium attempt", () => {
  assert.equal(criticalPathCases.length, 4);
  for (const { file, title } of criticalPathCases) {
    const source = readFileSync(new URL(`../e2e/${file}`, import.meta.url), "utf8");
    assert.ok(source.includes(`test("${title}"`), `case must remain in its original source: ${title}`);
  }
  assert.equal(validateCriticalPathReport(report()).passed, 4);
});

const mutations = {
  empty: (r) => { r.suites = []; },
  missing: (r) => { r.suites[0].suites.pop(); },
  extra: (r) => { r.suites[0].suites.push(structuredClone(r.suites[0].suites[0])); },
  global_error: (r) => { r.errors = [{ message: "worker startup failed" }]; },
  missing_errors: (r) => { delete r.errors; },
  summary_only: (r) => { delete r.suites; },
  multiple_workers: (r) => { r.config.workers = 2; },
  extra_project: (r) => { r.config.projects.push({ name: "firefox" }); },
  configured_retries: (r) => { r.config.projects[0].retries = 1; },
  repeated_suite: (r) => { r.config.projects[0].repeatEach = 2; },
  false_summary: (r) => { r.stats.skipped = 1; },
  invalid_suite: (r) => { r.suites[0].suites = [null]; },
  invalid_specs: (r) => { r.suites[0].specs = {}; },
};
for (const [name, mutate] of Object.entries(mutations)) {
  test(`reject report: ${name}`, () => {
    const r = report(); mutate(r);
    assert.throws(() => validateCriticalPathReport(r));
  });
}

const specMutations = {
  wrong_file: (s) => { s.file = `other/${s.file}`; },
  wrong_title: (s) => { s.title += " altered"; },
  not_ok: (s) => { s.ok = false; },
  no_test: (s) => { s.tests = []; },
  duplicate_project: (s) => { s.tests.push(structuredClone(s.tests[0])); },
  wrong_project: (s) => { s.tests[0].projectName = "firefox"; },
  wrong_project_id: (s) => { s.tests[0].projectId = "other"; },
  expected_failure: (s) => { s.tests[0].expectedStatus = "failed"; },
  skipped: (s) => { s.tests[0].expectedStatus = "skipped"; s.tests[0].status = "skipped"; },
  flaky: (s) => { s.tests[0].status = "flaky"; },
  unexpected: (s) => { s.tests[0].status = "unexpected"; },
  missing_result: (s) => { s.tests[0].results = []; },
  multiple_results: (s) => { s.tests[0].results.push({ status: "passed", retry: 1, errors: [] }); },
  retry: (s) => { s.tests[0].results[0].retry = 1; },
  result_error: (s) => { s.tests[0].results[0].errors = [{ message: "failed assertion" }]; },
  singular_error: (s) => { s.tests[0].results[0].error = { message: "failed assertion" }; },
  missing_errors: (s) => { delete s.tests[0].results[0].errors; },
  ...Object.fromEntries(["failed", "timedOut", "skipped", "interrupted"].map((status) => [
    `result_${status}`, (s) => { s.tests[0].results[0].status = status; },
  ])),
};
for (const [name, mutate] of Object.entries(specMutations)) {
  test(`reject case: ${name}`, () => {
    const r = report(); mutate(r.suites[0].suites[0].specs[0]);
    assert.throws(() => validateCriticalPathReport(r));
  });
}

test("malformed root values are not coverage", () => {
  for (const value of [null, [], "passed", {}, { suites: null }]) {
    assert.throws(() => validateCriticalPathReport(value));
  }
});
