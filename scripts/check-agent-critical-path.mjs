import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

// Keep the business assertions in their original specs. This inventory is the
// acceptance contract shared by selection and verification, not a duplicate test.
export const criticalPathCases = [
  { file: "core-business-journeys.spec.ts", title: "G1-01 assigning an agent queues one task that the daemon runs to a live completed reply" },
  { file: "core-business-journeys.spec.ts", title: "G1-02 a failed run notifies the creator, retries exactly once and can be stopped before it runs" },
  { file: "core-business-agent-loops.spec.ts", title: "G6 a plain comment re-engages the assignee, an @mention engages another agent and /note engages none" },
  { file: "core-business-agent-loops.spec.ts", title: "G8 a running task streams its transcript, leaves the issue status alone and stays stopped after a late completion" },
];

function requireValue(condition, message) {
  if (!condition) throw new Error(message);
}

export function validateCriticalPathReport(report) {
  requireValue(report && !Array.isArray(report) && typeof report === "object", "Invalid Playwright report");
  requireValue(Array.isArray(report.errors) && report.errors.length === 0, "Playwright reported global errors or omitted its error inventory");
  requireValue(report.config?.workers === 1, "Smoke requires one worker");
  const projects = report.config?.projects;
  requireValue(Array.isArray(projects) && projects.length === 1 && projects[0]?.id === "chromium" &&
    projects[0]?.name === "chromium" && projects[0]?.retries === 0 && projects[0]?.repeatEach === 1,
  "Smoke requires one Chromium project, one attempt and zero retries");
  const required = new Set(criticalPathCases.map(({ file, title }) => `${file}\n${title}`));
  const seen = new Set();
  const visit = (suites) => {
    requireValue(Array.isArray(suites), "Invalid suite inventory");
    for (const suite of suites) {
      requireValue(suite && Array.isArray(suite.specs), "Invalid spec inventory");
      for (const spec of suite.specs) {
        const identity = `${spec?.file}\n${spec?.title}`;
        requireValue(required.has(identity) && !seen.has(identity), "Unexpected or duplicate critical-path case");
        seen.add(identity);
        requireValue(spec.ok === true && Array.isArray(spec.tests) && spec.tests.length === 1, "Case did not have exactly one successful test");
        const test = spec.tests[0];
        requireValue(test?.projectName === "chromium" && test.projectId === "chromium" &&
          test.expectedStatus === "passed" && test.status === "expected", "Case skipped, failed, changed project or changed its expected result");
        requireValue(Array.isArray(test.results) && test.results.length === 1, "Case missing or retried");
        const result = test.results[0];
        requireValue(result?.status === "passed" && result.retry === 0 && !result.error &&
          Array.isArray(result.errors) && result.errors.length === 0, "Case did not pass its first attempt");
      }
      if (suite.suites !== undefined) visit(suite.suites);
    }
  };
  visit(report.suites);
  requireValue(seen.size === required.size, "Missing required critical-path cases");
  requireValue(report.stats?.expected === required.size && report.stats.skipped === 0 &&
    report.stats.unexpected === 0 && report.stats.flaky === 0, "Report summary disagrees with required coverage");
  return { passed: seen.size, cases: criticalPathCases };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    if (process.argv.length !== 3) throw new Error("Usage: check-agent-critical-path.mjs <playwright.json>");
    const result = validateCriticalPathReport(JSON.parse(readFileSync(process.argv[2], "utf8")));
    console.log(JSON.stringify(result));
  } catch (error) {
    console.error(`Critical-path inventory rejected: ${error instanceof SyntaxError ? "invalid JSON" : error.message}`);
    process.exitCode = 1;
  }
}
