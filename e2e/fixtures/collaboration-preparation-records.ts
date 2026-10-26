import { execFileSync } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { expect, type Locator, type Page } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "../fixtures";

interface RunIdentity {
  deployment_id: string; installation_id: string; issue_id: string; session_id: string;
  target_id: string; channel_id: string; batch_id: string; group_id: string; run_id: string;
  fence: string; claim_generation: string; launch_nonce: string;
}
interface DeliveredFile { id: string; version: string; name: string; content_type: string; size_bytes: number }
interface RunInput {
  command_id: string; author_user_id: string; participant_epoch: string; account_epoch: string;
  lifecycle_epoch: string; body: string; attachment_ids: string[];
}
interface PreparationObservation {
  identity: RunIdentity; event_id: string; attempt_seq: string; attempt_nonce: string; manifest_digest: string;
  stage: string; state: string; code?: string;
}
interface PublicPreparation {
  version: number; attachments: { id: string; version: string; name: string; size_bytes: number }[];
  git: { object_format: string; commit_oid: string } | null;
  stages: { seq: string; stage: string; state: string; code?: string }[];
}
interface RunRecords {
  protocol_version: number; issue_id: string;
  run: { id: string; group_id: string; progress_state: string; proof_state: string; business_outcome: string | null; settled_at: string | null };
  recording: { status: string; inputs_recorded: boolean }; preparation?: PublicPreparation;
  items: { id: string; kind: string; body: string }[]; next_cursor: string | null;
}

export function preparationRecordEnvironment() {
  const { DATABASE_URL: databaseUrl, COLLABORATION_E2E_API_URL: apiUrl,
    NEXT_PUBLIC_API_URL: browserApi, MULTICA_DEPLOYMENT_ID: deploymentId } = process.env;
  if (!databaseUrl || !apiUrl || !browserApi || !deploymentId) throw new Error("Preparation fixtures require explicit isolated API, database and deployment settings");
  for (const value of [databaseUrl, apiUrl, browserApi]) {
    if (!["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("Preparation fixtures require loopback test endpoints");
  }
  if (!["postgres:", "postgresql:"].includes(new URL(databaseUrl).protocol) ||
    ["", "multica", "postgres", "template0", "template1"].includes(new URL(databaseUrl).pathname.slice(1))) {
    throw new Error("Preparation fixtures refuse a default database");
  }
  if (new URL(apiUrl).origin !== new URL(browserApi).origin) throw new Error("Fixture and browser API origins differ");
  return { databaseUrl, apiUrl, deploymentId };
}

// This helper only seals and validates synthetic data with the production
// protocol. It does not import a service, daemon or provider executor.
const preparationSealer = String.raw`package main
import (
  "encoding/json"
  "errors"
  "os"
  "github.com/multica-ai/multica/server/pkg/protocol"
)
// Drafts carry an empty manifest_digest until sealing. The protocol type now
// validates while decoding, so decode drafts through a method-free copy.
type observationDraft protocol.CollaborationPreparationObservation
func main() {
  var request struct {
    Manifest protocol.CollaborationPreparation
    Inputs []protocol.CollaborationRunInput
    Observations []observationDraft
    Result protocol.CollaborationProviderResult
  }
  must := func(err error) { if err != nil { panic("synthetic preparation protocol validation failed") } }
  decoder := json.NewDecoder(os.Stdin)
  decoder.DisallowUnknownFields()
  must(decoder.Decode(&request))
  must(protocol.SealCollaborationPreparation(&request.Manifest))
  must(protocol.ValidateCollaborationPreparation(&request.Manifest, request.Manifest.Identity, request.Inputs))
  must(protocol.ValidateCollaborationProviderResult(request.Result))
  observations := make([]protocol.CollaborationPreparationObservation, 0, len(request.Observations))
  for _, draft := range request.Observations {
    item := protocol.CollaborationPreparationObservation(draft)
    item.ManifestDigest = request.Manifest.ManifestDigest
    must(protocol.ValidateCollaborationPreparationObservation(item))
    if item.Identity != request.Manifest.Identity || item.Stage == "git" && request.Manifest.Git == nil {
      must(errors.New("unrelated synthetic observation"))
    }
    observations = append(observations, item)
  }
  must(json.NewEncoder(os.Stdout).Encode(map[string]any{"manifest":request.Manifest, "observations":observations}))
}
`;

function sealPreparation(manifest: Record<string, unknown>, inputs: RunInput[], observations: PreparationObservation[], result: Record<string, unknown>) {
  const directory = mkdtempSync(join(tmpdir(), "multica-preparation-seal-"));
  try {
    const source = join(directory, "main.go");
    writeFileSync(source, preparationSealer, { mode: 0o600 });
    const output = execFileSync("go", ["run", "-mod=readonly", source], {
      cwd: resolve(import.meta.dirname, "../../server"), input: JSON.stringify({ manifest, inputs, observations, result }),
      env: { ...Object.fromEntries(["PATH", "HOME", "TMPDIR", "GOPATH", "GOCACHE", "GOMODCACHE"].flatMap((key) => process.env[key] ? [[key, process.env[key]]] : [])), GOTOOLCHAIN: "go1.26.6" },
      encoding: "utf8", timeout: 60_000, maxBuffer: 1 << 20,
    });
    const sealed: { manifest: Record<string, unknown> & { manifest_digest: string }; observations: PreparationObservation[] } = JSON.parse(output);
    expect(sealed.manifest.manifest_digest).toMatch(/^[a-f0-9]{64}$/);
    expect(sealed.observations).toHaveLength(observations.length);
    return sealed;
  } finally { rmSync(directory, { recursive: true, force: true }); }
}

/** Human APIs are real. Runtime history is explicitly synthetic and inserted
 * with its first immutable snapshot. No live resource, task, capability,
 * reservation, executable, provider account or daemon is created. */
export async function seedCollaborationPreparationRecords() {
  const environment = preparationRecordEnvironment();
  const suffix = randomUUID().slice(0, 10);
  const source = new TestApiClient();
  await source.login(`preparation-source-${suffix}@example.invalid`, "Preparation source owner");
  const workspace = await source.ensureWorkspace("Preparation record source", `preparation-${suffix}`);
  await source.markUserOnboarded();
  const sourceId = (await source.requestJSON<{ id: string }>("/api/me")).id;
  const viewer = new TestApiClient();
  await viewer.login(`preparation-viewer-${suffix}@example.invalid`, "Outside preparation viewer");
  await viewer.markUserOnboarded();
  const viewerId = (await viewer.requestJSON<{ id: string }>("/api/me")).id;
  expect(await viewer.getWorkspaces()).toEqual([]);

  const draftId = randomUUID();
  const fileName = `explicitly-delivered-specification-for-the-synthetic-preparation-record-${suffix}.txt`;
  const upload = new FormData();
  upload.set("draft_id", draftId);
  upload.set("source_workspace_id", workspace.id);
  upload.set("file", new Blob(["Explicitly delivered synthetic preparation input.\n"], { type: "text/plain" }), fileName);
  const uploadResponse = await fetch(`${environment.apiUrl}/api/collaboration/attachments/drafts`, {
    method: "POST", headers: { Authorization: `Bearer ${source.getToken()!}`, "Idempotency-Key": randomUUID() }, body: upload,
  });
  expect(uploadResponse.status).toBe(201);
  const file: DeliveredFile = await uploadResponse.json();
  expect(file.name).toBe(fileName);
  expect(file.version).toMatch(/^[1-9][0-9]*$/);
  const issue = await source.requestJSON<{ id: string; title: string; session: { id: string; queue_state: string } }>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: workspace.id, handler_user_id: viewerId, title: `Synthetic preparation records ${suffix}`,
      description: "Synthetic retained history for protected preparation-summary reads. No provider or process was started.",
      draft_id: draftId, attachment_ids: [file.id],
    },
  });
  expect(issue.session.queue_state).toBe("paused");
  const makeRun = () => ({ runId: randomUUID(), targetId: randomUUID(), channelId: randomUUID(), batchId: randomUUID(),
    commandId: randomUUID(), groupId: randomUUID(), agentId: randomUUID(), runtimeId: randomUUID(), grantId: randomUUID(),
    installationId: randomUUID(), bindingId: randomUUID(), launchNonce: randomUUID() });
  const prepared = makeRun(), legacy = makeRun();
  const identity: RunIdentity = { deployment_id: environment.deploymentId, installation_id: prepared.installationId,
    issue_id: issue.id, session_id: issue.session.id, target_id: prepared.targetId, channel_id: prepared.channelId,
    batch_id: prepared.batchId, group_id: prepared.groupId, run_id: prepared.runId, fence: "1", claim_generation: "1", launch_nonce: prepared.launchNonce };
  const input = "Frozen synthetic input: inspect only the explicitly delivered file and fixed Git commit.";
  const result = "Synthetic recorded business failure; termination remains unconfirmed.";
  const inputs: RunInput[] = [{ command_id: prepared.commandId, author_user_id: viewerId, participant_epoch: "1",
    account_epoch: "1", lifecycle_epoch: "1", body: input, attachment_ids: [file.id] }];
  const attemptNonce = randomUUID();
  const privateInstructions = "PRIVATE_PREPARATION_INSTRUCTIONS_DO_NOT_PROJECT /private/synthetic-owner/worktree";
  const bundleDigest = "c".repeat(64), gitBundleDigest = "d".repeat(64), commit = "b".repeat(40);
  const providerResult = { outcome: "failure", body: result, record: { version: 1, completeness: "complete",
    messages: Array.from({ length: 52 }, (_, i) => ({ kind: "message", role: "assistant", body: `Synthetic recorded preparation explanation ${i + 1}.` })),
  } };
  const observations: PreparationObservation[] = [
    { identity, event_id: randomUUID(), attempt_seq: "1", attempt_nonce: attemptNonce, manifest_digest: "", stage: "attachments", state: "prepared" },
    { identity, event_id: randomUUID(), attempt_seq: "1", attempt_nonce: attemptNonce, manifest_digest: "", stage: "git", state: "failed", code: "git_unavailable" },
    { identity, event_id: randomUUID(), attempt_seq: "1", attempt_nonce: attemptNonce, manifest_digest: "", stage: "workspace", state: "failed", code: "input_limit_exceeded" },
  ];
  const sealed = sealPreparation({ version: 1, identity, command_ids: [prepared.commandId],
    resource: { agent_id: prepared.agentId, runtime_id: prepared.runtimeId, owner_user_id: sourceId, execution_workspace_id: workspace.id,
      target_epoch: "1", config_version: "1", runtime_config_epoch: "1" },
    provider: { provider: "codex", protocol_family: "codex", adapter_id: "static_json_v1", adapter_version: 1,
      bundle_digest: bundleDigest, account_source: "runtime_configuration", resume: "fresh", fallback: false },
    invocation: { instructions: privateInstructions, model: "fixture-model", thinking_level: "" },
    attachments: [{ id: file.id, version: file.version, name: file.name, content_type: file.content_type,
      size_bytes: file.size_bytes, destination: `attachments/${file.id}/${file.version}` }],
    git: { object_format: "sha1", commit_oid: commit, bundle_digest: gitBundleDigest },
    limits: { file_count: 32, per_file_bytes: 100 << 20, total_input_bytes: 256 << 20, workspace_bytes: 512 << 20,
      git_expanded_bytes: 256 << 20, entry_count: 4096, timeout_seconds: 60 },
  }, inputs, observations, providerResult);
  const publicFile = { id: file.id, version: file.version, name: file.name, size_bytes: file.size_bytes };
  const initialSummary: PublicPreparation = { version: 1, attachments: [publicFile], git: { object_format: "sha1", commit_oid: commit },
    stages: [{ seq: "2", stage: "attachments", state: "prepared" }, { seq: "3", stage: "git", state: "failed", code: "git_unavailable" }] };
  const finalSummary: PublicPreparation = { ...initialSummary, stages: [...initialSummary.stages,
    { seq: "5", stage: "workspace", state: "failed", code: "input_limit_exceeded" }] };

  const db = new pg.Client(environment.databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN");
    for (const selected of [prepared, legacy]) {
      const isPrepared = selected === prepared;
      const resource = { role: "agent", agent_id: selected.agentId, runtime_id: selected.runtimeId,
        execution_workspace_id: workspace.id, owner_user_id: sourceId };
      await db.query("INSERT INTO collaboration_channel (id,session_id,root_agent_id,group_generation,next_batch_seq,next_command_seq,active_group_id,pause_reason) VALUES ($1,$2,$3,1,2,2,$4,'fixture_paused')",
        [selected.channelId, issue.session.id, selected.agentId, selected.groupId]);
      await db.query("INSERT INTO collaboration_target (id,session_id,kind,resource_id,roster,channel_id,group_generation,active_group_id,pause_reason) VALUES ($1,$2,'agent',$3,$4::jsonb,$5,1,$6,'fixture_paused')",
        [selected.targetId, issue.session.id, selected.agentId, JSON.stringify([resource]), selected.channelId, selected.groupId]);
      await db.query("INSERT INTO collaboration_batch (id,session_id,channel_id,effective_target_id,seq,state,root_command_id,group_id,started_at) VALUES ($1,$2,$3,$4,1,'in_flight',$5,$6,now())",
        [selected.batchId, issue.session.id, selected.channelId, selected.targetId, selected.commandId, selected.groupId]);
      await db.query("INSERT INTO collaboration_command (id,session_id,target_id,input_event_id,channel_id,batch_id,group_id,seq,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content,attachments,state) VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,1,1,1,$9,$10::jsonb,'in_flight')",
        [selected.commandId, issue.session.id, selected.targetId, randomUUID(), selected.channelId, selected.batchId, selected.groupId,
          viewerId, isPrepared ? input : "Unclaimed synthetic legacy input", JSON.stringify(isPrepared ? [file.id] : [])]);
      await db.query("INSERT INTO collaboration_execution_group (id,session_id,target_id,command_id,generation,target_epoch,config_version,roster,channel_id,batch_id,state) VALUES ($1,$2,$3,$4,1,1,1,$5::jsonb,$6,$7,'active')",
        [selected.groupId, issue.session.id, selected.targetId, selected.commandId, JSON.stringify([{ ...resource, grant_id: selected.grantId, grant_epoch: "1" }]), selected.channelId, selected.batchId]);
      await db.query(`INSERT INTO collaboration_run
        (id,session_id,target_id,group_id,command_id,step_key,step_kind,agent_id,target_epoch,config_version,
         grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch,
         channel_id,batch_id,resource_owner_user_id,deployment_id,launch_nonce,fence,claim_generation,
         progress_state,business_outcome,proof_state,authorized_attempt_seq,prompt_snapshot,preparation_snapshot,containment)
        VALUES ($1,$2,$3,$4,$5,'root','agent',$6,1,1,$7,1,1,$8,$9,$10,$11,1,$12,$13,$14,$15,$16,1,1,$17,$18,'unknown',$19,$20::jsonb,$21::jsonb,$22)`,
      [selected.runId, issue.session.id, selected.targetId, selected.groupId, selected.commandId, selected.agentId,
        selected.grantId, workspace.id, selected.runtimeId, selected.installationId, selected.bindingId,
        selected.channelId, selected.batchId, sourceId, environment.deploymentId, selected.launchNonce,
        isPrepared ? "ending" : "prepared", isPrepared ? "failure" : null, isPrepared ? 1 : 0,
        isPrepared ? JSON.stringify(inputs) : null, isPrepared ? JSON.stringify(sealed.manifest) : null,
        isPrepared ? "linux_pid_namespace_v1" : null]);
      await db.query("INSERT INTO collaboration_group_input (group_id,command_id,channel_id,batch_id,seq,consuming_run_id) VALUES ($1,$2,$3,$4,1,$5)",
        [selected.groupId, selected.commandId, selected.channelId, selected.batchId, selected.runId]);
    }
    await db.query("UPDATE collaboration_execution_group SET state='ending',expansion_sealed_at=now() WHERE id=$1", [prepared.groupId]);
    // A real claim creates the termination capability before it freezes the
    // preparation snapshot, and records reads resolve the frozen identity from
    // it. Seed the same row; proof stays unconfirmed (no proof/confirmed_at).
    await db.query(`INSERT INTO collaboration_termination
      (issue_id,run_id,fence,launch_nonce,deployment_id,installation_id,identity_kind,public_key,token_hash,runtime_identity,token_ciphertext)
      VALUES ($1,$2,1,$3,$4,$5,'managed',$6,$7,$8::jsonb,$9)`,
      [issue.id, prepared.runId, prepared.launchNonce, environment.deploymentId, prepared.installationId,
        randomBytes(32), "e".repeat(64), JSON.stringify(identity), randomBytes(48)]);
    await db.query("INSERT INTO collaboration_attempt (run_id,attempt_seq,attempt_nonce,claim_generation,containment) VALUES ($1,1,$2,1,'linux_pid_namespace_v1')", [prepared.runId, attemptNonce]);
    await db.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,'resource-snapshot',1,'resource_snapshot',$3::jsonb)",
      [issue.id, prepared.runId, JSON.stringify({ provider: "fixture-provider", model: "fixture-model", account_source: "runtime_configuration" })]);
    for (const [index, observation] of sealed.observations.slice(0, 2).entries()) {
      await db.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,$3,$4,'preparation',$5::jsonb)",
        [issue.id, prepared.runId, observation.event_id, index + 2, JSON.stringify(observation)]);
    }
    const outcomeEventId = randomUUID();
    await db.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,$3,4,'outcome',$4::jsonb)",
      [issue.id, prepared.runId, outcomeEventId, JSON.stringify({ identity, event_id: outcomeEventId, attempt_seq: "1", attempt_nonce: attemptNonce, ...providerResult })]);
    await db.query("COMMIT");
  } catch (error) { await db.query("ROLLBACK"); throw error; }
  finally { await db.end(); }

  const privateValues = [privateInstructions, "/private/synthetic-owner/worktree", sealed.manifest.manifest_digest,
    bundleDigest, gitBundleDigest, attemptNonce, prepared.launchNonce, prepared.installationId, prepared.bindingId];
  const readRecords = async (actor: TestApiClient, runId: string, cursor?: string, limit = 50) => {
    const query = new URLSearchParams({ limit: String(limit) });
    if (cursor) query.set("cursor", cursor);
    const response = await fetch(`${environment.apiUrl}/api/collaboration/issues/${issue.id}/runs/${runId}/records?${query}`, {
      headers: { Authorization: `Bearer ${actor.getToken()!}` },
    });
    expect(response.status).toBe(200);
    expect(response.headers.get("cache-control")).toContain("no-store");
    const page: RunRecords = await response.json();
    expect(page).toMatchObject({ protocol_version: 1, issue_id: issue.id, run: { id: runId, proof_state: "unknown", settled_at: null } });
    const wire = JSON.stringify(page);
    for (const value of [...privateValues, '"manifest_digest"', '"bundle_digest"', '"attempt_nonce"', '"destination"', '"invocation"']) expect(wire).not.toContain(value);
    expect(page.items.every((item) => item.kind !== "preparation")).toBe(true);
    return page;
  };
  const verifyAPIPaging = async () => {
    const before = await readRecords(viewer, prepared.runId, undefined, 1);
    expect(before.preparation).toEqual(initialSummary);
    expect(before.items).toHaveLength(1);
    expect(before.items[0]).toMatchObject({ kind: "input", body: input });
    expect(before.next_cursor).not.toBeNull();
    const connection = new pg.Client(environment.databaseUrl);
    await connection.connect();
    try {
      await connection.query("BEGIN");
      const retained = await connection.query("SELECT prompt_snapshot,preparation_snapshot,proof_state,business_outcome,authorized_attempt_seq FROM collaboration_run WHERE id=$1 AND session_id=$2 FOR UPDATE",
        [prepared.runId, issue.session.id]);
      expect(retained.rowCount).toBe(1);
      const observation = sealed.observations[2]!;
      await connection.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,$3,5,'preparation',$4::jsonb)",
        [issue.id, prepared.runId, observation.event_id, JSON.stringify(observation)]);
      const unchanged = await connection.query("SELECT prompt_snapshot,preparation_snapshot,proof_state,business_outcome,authorized_attempt_seq FROM collaboration_run WHERE id=$1 AND session_id=$2",
        [prepared.runId, issue.session.id]);
      expect(unchanged.rows).toEqual(retained.rows);
      await connection.query("COMMIT");
    } catch (error) { await connection.query("ROLLBACK"); throw error; }
    finally { await connection.end(); }
    const oldPage = await readRecords(viewer, prepared.runId, before.next_cursor!, 100);
    expect(oldPage.preparation).toEqual(initialSummary);
    expect(oldPage.run).toEqual(before.run);
    expect(oldPage.next_cursor).toBeNull();
    expect(oldPage.items.at(-1)).toMatchObject({ kind: "result", body: result });
    const fresh = await readRecords(viewer, prepared.runId);
    const otherViewer = await readRecords(source, prepared.runId);
    expect(fresh.preparation).toEqual(finalSummary);
    expect(otherViewer.preparation).toEqual(finalSummary);
    expect(fresh.run).toEqual(before.run);
    expect(otherViewer.run).toEqual(before.run);
    const old = await readRecords(viewer, legacy.runId);
    expect(old.preparation).toBeUndefined();
    expect(old.run).toMatchObject({ progress_state: "prepared", proof_state: "unknown", business_outcome: null, settled_at: null });
    expect(old.recording).toEqual({ status: "not_recorded", inputs_recorded: false });
    expect(old.items).toEqual([]);
    expect(await source.countIssueDispatches(issue.id)).toBe(0);
    return { source: "synthetic retained history validated by production protocol; real protected HTTP reads",
      issue_id: issue.id, run_id: prepared.runId, legacy_run_id: legacy.runId,
      frozen_watermark: "4", later_observation_seq: "5", preparation: finalSummary, run: fresh.run, legacy: old.run };
  };
  return { source, viewer, issueId: issue.id, title: issue.title, runId: prepared.runId, legacyRunId: legacy.runId,
    file: publicFile, commit, input, result, verifyAPIPaging };
}

export type PreparationRecordFixture = Awaited<ReturnType<typeof seedCollaborationPreparationRecords>>;
export const preparationAccountPath = (fixture: PreparationRecordFixture, legacy = false) =>
  `/account?view=participating&issue=${fixture.issueId}#run-${legacy ? fixture.legacyRunId : fixture.runId}`;

interface PreparationRecordAdapter {
  navigate(page: Page, path: string): Promise<void>;
  refresh(page: Page, path: string): Promise<void>;
  capture(stage: string, page: Page, focus: Locator): Promise<void>;
}

export async function exercisePreparationRecords(viewer: Page, owner: Page, fixture: PreparationRecordFixture, adapter: PreparationRecordAdapter) {
  const evidence = await fixture.verifyAPIPaging();
  const showSummary = async (page: Page) => {
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const records = page.locator(`[id="run-${fixture.runId}"]`).getByRole("region", { name: "Run records", exact: true });
    const summary = records.getByRole("region", { name: "Run preparation", exact: true });
    await expect(summary).toBeVisible();
    const files = summary.locator("details");
    if (!await files.evaluate((element) => (element as HTMLDetailsElement).open)) await summary.getByText("1 recorded attachment version", { exact: true }).click();
    await expect(summary.getByText(fixture.file.name, { exact: true })).toBeVisible();
    await expect(summary.getByText(`Version ${fixture.file.version} · ${fixture.file.size_bytes} bytes`, { exact: true })).toBeVisible();
    await expect(summary.getByText(fixture.file.id, { exact: true })).toBeVisible();
    await expect(summary.getByText(fixture.commit, { exact: true })).toBeVisible();
    await expect(summary.getByText("Attachments", { exact: true }).locator("..")).toContainText("Prepared");
    await expect(summary.getByText("Runtime files", { exact: true }).locator("..")).toContainText("Not recorded");
    await expect(summary.getByText("The fixed Git commit could not be prepared.", { exact: true })).toBeVisible();
    await expect(summary.getByText("Preparation exceeded its size or file limit.", { exact: true })).toBeVisible();
    await expect(records.getByText("Run outcome", { exact: true }).locator("..")).toContainText("Failed");
    await expect(records.getByText("Termination proof", { exact: true }).locator("..")).toContainText("Termination unconfirmed");
    await expect(records.getByText("Group status", { exact: true }).locator("..")).toContainText("Waiting for termination");
    expect(await summary.textContent()).not.toMatch(/not run|termination confirmed|exited_proven/);
    const more = records.getByRole("button", { name: "Load more records", exact: true });
    if (await more.count()) await more.click();
    await expect(records.getByText(fixture.result, { exact: true })).toBeVisible();
    return summary;
  };
  await adapter.navigate(viewer, preparationAccountPath(fixture));
  const summary = await showSummary(viewer);
  await adapter.capture("prepared", viewer, summary);
  await adapter.refresh(viewer, preparationAccountPath(fixture));
  await showSummary(viewer);
  await adapter.navigate(owner, preparationAccountPath(fixture));
  await showSummary(owner);
  await adapter.navigate(viewer, preparationAccountPath(fixture, true));
  const legacyRecords = viewer.locator(`[id="run-${fixture.legacyRunId}"]`).getByRole("region", { name: "Run records", exact: true });
  const legacySummary = legacyRecords.getByRole("region", { name: "Run preparation", exact: true });
  await expect(legacySummary.getByText("Not recorded", { exact: true })).toBeVisible();
  expect(await legacySummary.textContent()).not.toMatch(/prepared|failed|not run|confirmed/i);
  await expect(legacyRecords.getByText("No business result recorded", { exact: true })).toBeVisible();
  await expect(legacyRecords.getByText("Termination proof", { exact: true }).locator("..")).toContainText("Termination unconfirmed");
  await adapter.capture("legacy", viewer, legacySummary);
  return evidence;
}
