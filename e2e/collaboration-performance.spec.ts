/**
 * PAY-241 S5 canonical sampling suite: 100 independent collaborators observing
 * status propagation, then controlled Agent result write-back on the same issue.
 *
 * Every test skips unless COLLABORATION_PERF_ACCEPTANCE=1, so `make check` and a
 * whole-directory Playwright run report skips only. With the opt-in, anything
 * other than the coordinator's isolated, pre-registered run is refused before a
 * request or a database connection is made. No browser is used.
 *
 * Statistics and their counter-examples: e2e/fixtures/collaboration-performance-stats.ts
 * and its .node-test.mjs. A result from this suite describes the source version
 * named in the run manifest only; it is not carried over to later source.
 */
import { dirname, join } from "node:path";
import { expect, test } from "@playwright/test";
import {
  guardEnvironment, loadPreparedRun, openObserverSessions, performanceOptIn, prepareRun, readGate, runPropagation, runWriteback, withPhaseEvidence,
} from "./fixtures/collaboration-performance";
import { PERFORMANCE_PROTOCOL } from "./fixtures/collaboration-performance-stats";

const optIn = performanceOptIn();
const harnessFiles = (spec: string) => [spec, ...["collaboration-performance.ts", "collaboration-performance-stats.ts",
  "collaboration-performance-stats.node-test.mjs", "collaboration-host-runtime.ts", "collaboration-host-provider.go"].map((name) => join(dirname(spec), "fixtures", name)),
join(dirname(spec), "fixtures.ts")];

test.describe("PAY-241 S5 collaboration sampling", () => {
  test("prepare: 100 password accounts join one collaboration issue through public routes", async ({ browserName: _browserName }, info) => {
    test.skip(!optIn.enabled, optIn.reason);
    // Registration is limited to 30 per address per 5 minutes; four windows are waited out, never cleared.
    test.setTimeout(60 * 60_000);
    const env = guardEnvironment("prepare");
    const prepared = await withPhaseEvidence(env, harnessFiles(info.file), () => prepareRun(env));
    expect(new Set(prepared.accounts.map((account) => account.user_id)).size).toBe(PERFORMANCE_PROTOCOL.observers);
    await info.attach("s5-prepare", { contentType: "application/json", body: Buffer.from(JSON.stringify({
      run_id: env.manifest.run_id, issue_id: prepared.issueId, distinct_accounts: prepared.accounts.length,
      note: "Setup only. Not timed and not acceptance evidence." }, null, 2)) });
  });

  test("propagation: 100 observers each see 100 status changes on their own socket and token", async ({ browserName: _browserName }, info) => {
    test.skip(!optIn.enabled, optIn.reason);
    test.setTimeout(90 * 60_000);
    const env = guardEnvironment("propagation");
    const outcome = await withPhaseEvidence(env, harnessFiles(info.file), async () => {
      // This phase never changes the gate; it only requires that creation was enabled at allocation.
      expect((await readGate(env)).creation_enabled).toBe(true);
      const sessions = await openObserverSessions(env, loadPreparedRun(env));
      try { return await runPropagation(env, sessions); } finally { await sessions.close(); }
    });
    await info.attach("s5-propagation-summary", { contentType: "application/json", body: Buffer.from(JSON.stringify(outcome.summary, null, 2)) });
    // Raw samples, summary and failures are already on disk. The gate is zero missing AND pooled p95 AND slowest-observer p95.
    expect(outcome.aborted, "the registered 100 events must all run").toBeNull();
    expect(outcome.pass, "sync gate: 10,000 converged samples, pooled p95 <= 2000 ms, per-event slowest-observer p95 <= 2000 ms").toBe(true);
  });

  test("writeback: 100 controlled runs each return one business result and one finalization", async ({ browserName: _browserName }, info) => {
    test.skip(!optIn.enabled, optIn.reason);
    test.setTimeout(120 * 60_000);
    const env = guardEnvironment("writeback");
    const outcome = await withPhaseEvidence(env, harnessFiles(info.file), async () => {
      const sessions = await openObserverSessions(env, loadPreparedRun(env));
      // runWriteback closes sockets, settles the daemon and its owner-scoped heartbeat, then restores the gate.
      // This close is the idempotent socket backstop.
      try { return await runWriteback(env, sessions); } finally { await sessions.close(); }
    });
    await info.attach("s5-writeback-summary", { contentType: "application/json", body: Buffer.from(JSON.stringify(outcome.summary, null, 2)) });
    expect(outcome.aborted, "the registered 100 runs must all run").toBeNull();
    expect(outcome.pass, "write-back gates: business result p95 <= 5000 ms and finalization p95 <= 5000 ms, each with no missing sample, and clean teardown").toBe(true);
  });
});
