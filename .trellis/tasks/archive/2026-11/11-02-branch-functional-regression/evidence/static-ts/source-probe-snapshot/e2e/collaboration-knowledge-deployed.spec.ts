import { createHash, randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import pg from "pg";
import { expect, test } from "@playwright/test";
import type { CollaborationKnowledgeDetail, CollaborationKnowledgeProposal, CollaborationKnowledgeReceipt, CollaborationMemory } from "@multica/core/collaboration";
import { apiApprove } from "./fixtures/collaboration-knowledge";
import { assertHostVisit, authorizeHostIssue, hostAccountPath, hostControl, issueRoot, openHostConsent,
  readHostIssue, seedHostRuntime, sendHostComment, waitHostSettlement } from "./fixtures/collaboration-host-runtime";

const hash = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");
const eventKinds = ["knowledge_claim", "knowledge_preparation", "knowledge_authorization"];

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "off" });

test("deployed knowledge startup flag controls one actual Host input and its authorization events", async ({ page }, info) => {
  test.setTimeout(180_000);
  const expected = process.env.COLLABORATION_KNOWLEDGE_EXPECTED;
  test.skip(expected === undefined, "Requires an explicit true/false expectation and isolated task-owned deployment");
  if (expected !== "true" && expected !== "false") throw new Error("COLLABORATION_KNOWLEDGE_EXPECTED must be literal true or false");
  // seedHostRuntime hard-fails partially configured binaries, auth and task DB.
  // This probe never overwrites the API process's knowledge configuration.
  const fixture = await seedHostRuntime();
  const evidence: Record<string, unknown> = { scope: "deployed environment -> public API -> actual test-owned Host input",
    expected_knowledge_enabled: expected === "true", real_provider_calls: 0,
    excludes: ["real model understanding", "human approval UI", "correction/withdrawal composite", "installer and different-account journey"] };
  let settledRunId: string | undefined;
  try {
    const health = await fetch(`${fixture.apiUrl}/health`);
    expect(health.status).toBe(200);
    const apiHealth: { commit: string; pid: number } = await health.json();
    expect(apiHealth.commit).toMatch(/^[0-9a-f]{7,40}$/);
    expect(apiHealth.pid).toBeGreaterThan(0);
    const root = issueRoot(fixture.main.id);
    const memoryContent = `Ordinary source context ${randomUUID()}`;
    const memory = await fixture.webActor.client.requestJSON<CollaborationMemory>(`${root}/memory`, {
      method: "PUT", headers: { "Idempotency-Key": randomUUID() }, body: { expected_version: "0", content: memoryContent },
    });
    expect(memory.version).toBe("1");
    const content = `DEPLOYED_KNOWLEDGE_ONLY_${randomUUID()}`;
    expect([fixture.main.description, memoryContent].some((value) => value.includes(content))).toBe(false);
    const proposal: CollaborationKnowledgeProposal = { protocol_version: 1, candidate_id: randomUUID(), request_id: randomUUID(),
      summary: "Explicit deployed knowledge gate probe", content,
      target: { kind: "workspace", workspace_id: fixture.sourceWorkspace.id },
      sources: [{ kind: "memory", issue_id: fixture.main.id, version: memory.version, sha256: hash(memoryContent) }],
      evidence: [], applicability: { surfaces: ["collaboration"] }, expires_at: null, supersedes_candidate_id: null, derived_from_candidate_id: null };
    const proposed = await fixture.webActor.client.requestJSON<CollaborationKnowledgeReceipt>(`${root}/knowledge-candidates`, { method: "POST", body: proposal });
    expect(proposed).toMatchObject({ code: "knowledge_proposed", candidate_id: proposal.candidate_id, request_id: proposal.request_id, state: "pending", version: "1" });
    const approved = await apiApprove(fixture.webActor.client, proposal.candidate_id);
    expect(approved).toMatchObject({ code: "knowledge_approved", candidate_id: proposal.candidate_id, state: "approved", version: "1" });
    const reviewed = (await fixture.webActor.client.requestJSON<CollaborationKnowledgeDetail>(`/api/collaboration/knowledge-candidates/${proposal.candidate_id}`)).candidate;
    expect(reviewed).toMatchObject({ availability: "available", candidate_id: proposal.candidate_id, state: "approved", version: "1", content_sha256: hash(content) });
    if (reviewed.availability !== "available") throw new Error("Approved probe candidate is not readable");
    await fixture.webActor.authenticateWeb(page);
    await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await page.goto(hostAccountPath(fixture.main.id));
    await openHostConsent(page, fixture.main);
    await authorizeHostIssue(page, fixture, fixture.main);
    await hostControl(page, fixture.main, "resume", "issue");
    const accepted = await sendHostComment(page, fixture.main, "Perform the accepted deployment probe assignment.", fixture.webActor.id);
    expect(accepted.content).not.toContain(content);
    const visit = await fixture.daemon.waitForVisit(1);
    assertHostVisit(visit, fixture, fixture.main, [accepted]);
    expect(visit.resumed).toBe(false);
    const observed = visit.knowledge_observation;
    if (!observed) throw new Error("The explicitly built Host fake did not observe its actual input");
    expect(observed.prompt_sha256).toMatch(/^[0-9a-f]{64}$/);
    expect(observed.section_present).toBe(expected === "true");
    expect(observed.no_session_persistence).toBe(expected === "true");
    expect(observed.contributions).toEqual(expected === "true" ? [{ candidate_id: proposal.candidate_id, version: "1",
      source_issue_id: fixture.main.id, content_sha256: hash(content) }] : []);
    fixture.daemon.release(visit);
    const run = await waitHostSettlement(fixture, fixture.main, visit);
    settledRunId = run.id;
    expect((await readHostIssue(fixture)).execution!.runs).toHaveLength(1);
    expect(fixture.daemon.visits).toHaveLength(1);
    expect(fixture.daemon.errors).toEqual([]);
    const database = new pg.Client(process.env.DATABASE_URL);
    await database.connect();
    let counts: Record<string, number>;
    let databaseName: string;
    try {
      const identity = await database.query<{ name: string }>("SELECT current_database() AS name");
      databaseName = identity.rows[0]!.name;
      expect(databaseName).toBe(new URL(process.env.DATABASE_URL!).pathname.slice(1));
      const events = await database.query<{ kind: string; count: string }>(
        "SELECT kind,count(*)::text AS count FROM collaboration_run_event WHERE run_id=$1 AND kind=ANY($2::text[]) GROUP BY kind", [visit.identity.run_id, eventKinds]);
      counts = Object.fromEntries(eventKinds.map((kind) => [kind, Number(events.rows.find((row) => row.kind === kind)?.count ?? "0")]));
    } finally { await database.end(); }
    expect(counts).toEqual(Object.fromEntries(eventKinds.map((kind) => [kind, expected === "true" ? 1 : 0])));
    evidence.environment = { api: fixture.apiUrl, deployment_id: fixture.deploymentId, auth_mode: "password", api_health: apiHealth,
      database: databaseName, native_os: process.platform, native_arch: process.arch };
    evidence.approved = { candidate_id: proposal.candidate_id, version: reviewed.version, content_sha256: reviewed.content_sha256,
      source_issue_id: fixture.main.id, target_workspace_id: fixture.sourceWorkspace.id, proposed_by_user_id: fixture.webActor.id };
    evidence.accepted_input = { command_id: accepted.commandId, batch_id: accepted.batchId, author_user_id: accepted.actorId, content_sha256: hash(accepted.content) };
    evidence.actual_input = observed;
    evidence.run = { id: run.id, progress_state: run.progress_state, proof_state: run.proof_state,
      business_outcome: run.business_outcome, launch_count: run.launch_count };
    evidence.knowledge_events = counts;
    evidence.binary_sha256 = Object.fromEntries(["COLLABORATION_HOST_DAEMON", "COLLABORATION_HOST_PROVIDER", "COLLABORATION_HOST_CLI"]
      .map((key) => [key, hash(readFileSync(process.env[key]!))]));
    evidence.source_sha256 = Object.fromEntries(["e2e/collaboration-knowledge-deployed.spec.ts", "e2e/fixtures/collaboration-host-provider.go", "e2e/fixtures/collaboration-host-runtime.ts"]
      .map((path) => [path, hash(readFileSync(path))]));
  } finally {
    const summary = await fixture.daemon.close();
    evidence.daemon = summary;
    const encoded = JSON.stringify(evidence, null, 2);
    expect(encoded).not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+/);
    await info.attach("deployed-knowledge-gate-evidence", { contentType: "application/json", body: encoded });
    expect(summary.exit_code).toBe(0);
    expect(summary.errors).toEqual([]);
    expect(summary.stderr).toBe("");
    if (settledRunId) {
      expect(summary.ledger).toHaveLength(1);
      expect(summary.ledger[0]).toMatchObject({ run_id: settledRunId, phase: "proof_acknowledged", namespace_present: false,
        sealed: true, proof_state: "host_exit_observed", launches: [{ attempt_seq: "1", kind: "provider", phase: "attempt_reaped",
          process_observed: true, terminal_accepted: true, authorized: true, terminal_kind: "managed_launch_ended" }] });
    }
  }
});
