import { createHash, randomUUID } from "node:crypto";
import { execFileSync } from "node:child_process";
import { chmodSync, existsSync, lstatSync, mkdirSync, readFileSync, realpathSync, writeFileSync } from "node:fs";
import { isAbsolute, join, normalize } from "node:path";
import { expect, type Page, type Response, type Route } from "@playwright/test";
import pg from "pg";
import type { CollaborationStageCandidateRef, CollaborationStageCodeCandidate } from "@multica/core/collaboration";
import { CollaborationCompletionPreviewSchema } from "../../packages/core/collaboration/completion-schema";
import type { ClosureActor } from "./collaboration-multi-account-closure";
import { readClosureIssue, workflowAuthority } from "./collaboration-multi-account-closure";
import { readCode, readStage, stageObservations, stagePanel, stageRequest, stageRoot, stageSuccess, stageUI,
  type StageHTTPFact, type createStageGitInput } from "./collaboration-stage-approvals";
import { StageExecutionControl, type StageControlReply, type StageControlRequest } from "./collaboration-stage-control";

export type StageEvidenceRecorder = (name: string, facts: unknown) => void;
export const stageBytesHash = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");
const canonicalUUID = (value: string) => /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(value) && value !== "00000000-0000-0000-0000-000000000000";
export function stageObject(value: unknown): asserts value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) throw new Error("Stage fixture expected an object");
}
export function stageString(value: unknown, field: string): string {
  if (typeof value !== "string" || value === "") throw new Error(`Stage fixture omitted ${field}`);
  return value;
}

export async function stageResponseFact(response: Response, actor: ClosureActor): Promise<StageHTTPFact> {
  const request = response.request(), body = request.postData(), text = await response.text();
  const parse = (value: string | null): unknown => {
    if (!value) return null;
    try { return JSON.parse(value); } catch { return value; }
  };
  return { actor_id: actor.id, method: request.method(), path: new URL(request.url()).pathname,
    request_id: request.headers()["idempotency-key"] ?? null, request_body: parse(body), request_sha256: body === null ? null : stageBytesHash(body),
    status: response.status(), response_body: parse(text), response_sha256: stageBytesHash(text), at: new Date().toISOString() };
}

export interface StageInterruptedCompletion {
  actor_id: string; path: string; request_id: string; request_body: Record<string, unknown>; request_sha256: string;
  transport: "aborted_before_forward";
}

/** The first final-write attempt reaches the browser transport but not the API. */
export async function loseStageCompletionBeforeForward(page: Page, actor: ClosureActor, path: string, action: () => Promise<unknown>): Promise<StageInterruptedCompletion> {
  let resolve!: (value: StageInterruptedCompletion) => void, reject!: (error: unknown) => void;
  const captured = new Promise<StageInterruptedCompletion>((yes, no) => { resolve = yes; reject = no; });
  const match = (url: URL) => url.pathname === path;
  const intercept = async (route: Route) => {
    if (route.request().method() !== "POST") { await route.continue(); return; }
    try {
      const request = route.request(), text = request.postData(), key = request.headers()["idempotency-key"];
      if (!text || !key || !canonicalUUID(key)) throw new Error("Completion omitted its original operation identity");
      const body: unknown = JSON.parse(text); stageObject(body); stageObject(body.completion);
      if (body.action !== "complete" || body.completion.operation_id !== key) throw new Error("Completion differs from its original operation identity");
      const original: StageInterruptedCompletion = { actor_id: actor.id, path, request_id: key, request_body: body,
        request_sha256: stageBytesHash(text), transport: "aborted_before_forward" };
      await route.abort("failed"); resolve(original);
    } catch (error) { await route.abort("failed").catch(() => undefined); reject(error); }
  };
  await page.route(match, intercept);
  const deadline = setTimeout(() => reject(new Error("Original completion request was not observed within the existing action deadline")), 20_000);
  try { const [, original] = await Promise.all([Promise.resolve().then(action), captured]); return original; }
  finally { clearTimeout(deadline); await page.unroute(match, intercept); }
}

/** Both platforms use the actual shared form and root/child preview endpoint. */
export async function stageCompletionReview(page: Page, actor: ClosureActor, expected: {
  issueId: string; childId: string; approvalId: string; artifactDigest: string; sourceRevision: string; submissionId: string;
}, retain: (fact: StageHTTPFact) => StageHTTPFact) {
  await page.getByRole("button", { name: "Accept and complete", exact: true }).click();
  const form = page.getByRole("form", { name: "Accept and complete", exact: true });
  const gate = form.getByRole("region", { name: "Completion authorization", exact: true });
  await expect(gate).toBeVisible();
  await gate.getByRole("checkbox", { name: "Use completion authorization", exact: true }).check();
  await expect(form.getByRole("button", { name: "Accept and complete", exact: true })).toBeDisabled();
  await gate.getByRole("combobox", { name: "Source sub-issue", exact: true }).selectOption(expected.childId);
  const path = `/api/collaboration/issues/${expected.issueId}/delivery-evidence/completion-preview`;
  const [response] = await Promise.all([
    page.waitForResponse((reply) => new URL(reply.url()).pathname === path && reply.request().method() === "POST", { timeout: 20_000 }),
    gate.getByRole("combobox", { name: "Artifact evidence", exact: true }).selectOption(expected.approvalId),
  ]);
  const fact = retain(await stageResponseFact(response, actor));
  expect(fact.status).toBe(200);
  const preview = CollaborationCompletionPreviewSchema.parse(fact.response_body);
  expect(preview).toMatchObject({ issue_id: expected.issueId, source_issue_id: expected.childId, policy_mode: "enforce",
    reason: "authorization_available", enforced: false, usable_for_completion: false, stage_blockers: [], stage_blockers_truncated: false,
    observation: { verdict: "pass", approval_id: expected.approvalId, artifact_digest: expected.artifactDigest, source_revision: expected.sourceRevision } });
  if (!preview.authorization_binding) throw new Error("Current preview did not provide an exact authorization binding");
  expect(preview.authorization_binding.target).toMatchObject({ kind: "collaboration_submission", issue_id: expected.issueId,
    submission_id: expected.submissionId, acceptance_note: "" });
  expect(preview.authorization_binding.consumer).toMatchObject({ kind: "human", user_id: actor.id });
  await expect(gate).toContainText(expected.artifactDigest); await expect(gate).toContainText(expected.sourceRevision);
  return { form, gate, fact, preview };
}

/** These exact roots are allocated before the actual API's packager is constructed. */
export function stageArtifactEnvironment() {
  const staging = process.env.MULTICA_ARTIFACT_STAGING_ROOT, output = process.env.MULTICA_ARTIFACT_OUTPUT_ROOT;
  const rawSlots = process.env.COLLABORATION_STAGE_ARTIFACT_SLOTS;
  if (!staging || !output || !rawSlots) throw new Error("Stage execution requires allocated artifact roots and generation slots");
  for (const [path, mode] of [[staging, 0o500], [output, 0o700]] as const) {
    if (!isAbsolute(path) || normalize(path) !== path || realpathSync(path) !== path) throw new Error("Artifact root must be an absolute canonical path");
    const st = lstatSync(path);
    if (!st.isDirectory() || st.isSymbolicLink() || (st.mode & 0o777) !== mode || st.uid !== process.getuid?.()) throw new Error("Artifact root ownership/mode differs from its allocation");
  }
  const slots: unknown = JSON.parse(rawSlots);
  if (!Array.isArray(slots) || slots.length !== 8 || slots.some((id) => typeof id !== "string" || !canonicalUUID(id)) || new Set(slots).size !== slots.length) {
    throw new Error("Stage execution requires eight distinct allocated artifact generation UUIDs");
  }
  return { staging, output, slots: slots as string[] };
}

interface ArtifactReply {
  approval: { id: string; status: string; decided_by: string | null };
  execution_id: string; spec_digest: string; status: string; receipt_digest: string;
  receipt: { output_digest?: string; output_bytes?: number };
}
function artifactReply(value: unknown): ArtifactReply {
  stageObject(value); stageObject(value.approval); stageObject(value.receipt);
  for (const id of [value.execution_id, value.approval.id]) if (typeof id !== "string" || !canonicalUUID(id)) throw new Error("Artifact API omitted its actual identity");
  if (typeof value.spec_digest !== "string" || !/^[0-9a-f]{64}$/.test(value.spec_digest) || typeof value.status !== "string" ||
    typeof value.approval.status !== "string" || typeof value.receipt_digest !== "string") throw new Error("Artifact API returned an incomplete binding");
  return value as unknown as ArtifactReply;
}

/** Only one regular USTAR entry is expected; compare every actual byte and EOF. */
export function verifyStageArtifactTar(archive: Buffer, expected: Buffer) {
  const expectedLength = 512 + Math.ceil(expected.length / 512) * 512 + 1024;
  if (archive.length !== expectedLength) throw new Error("Actual artifact has an unexpected archive size");
  const header = archive.subarray(0, 512);
  const field = (start: number, end: number) => header.subarray(start, end).toString("utf8").replace(/\0.*$/s, "");
  const octal = (start: number, end: number) => {
    const value = header.subarray(start, end).toString("latin1");
    if (!/^[0-7]+[\0 ]+$/.test(value)) throw new Error("Actual artifact has an invalid USTAR numeric field");
    return Number.parseInt(value, 8);
  };
  let checksum = 0;
  for (let i = 0; i < 512; i++) checksum += i >= 148 && i < 156 ? 32 : header[i]!;
  if (field(0, 100) !== "stage-regression.json" || field(257, 263) !== "ustar" || field(263, 265) !== "00" ||
    field(345, 500) !== "" || ![0, 48].includes(header[156]!) || octal(100, 108) !== 0o644 || octal(108, 116) !== 0 ||
    octal(116, 124) !== 0 || octal(124, 136) !== expected.length || octal(136, 148) !== 0 || octal(148, 156) !== checksum) {
    throw new Error("Actual artifact header does not match the fixed packager contract");
  }
  if (!archive.subarray(512, 512 + expected.length).equals(expected) || archive.subarray(512 + expected.length).some((byte) => byte !== 0)) {
    throw new Error("Actual artifact bytes, padding or EOF differ from the selected Git object");
  }
  return { artifact_digest: stageBytesHash(archive), size_bytes: archive.length, source_file_sha256: stageBytesHash(expected) };
}

/** No successful artifact, approval, execution fact or metric row is seeded. */
export class StageExecutionFixture {
  private readonly db: pg.Client;
  private database: string | null = null;
  constructor() {
    if (!process.env.DATABASE_URL?.trim()) throw new Error("Stage execution fixture requires an explicit task database");
    this.db = new pg.Client({ connectionString: process.env.DATABASE_URL, connectionTimeoutMillis: 5000 });
  }
  async open(expected: string) {
    await this.db.connect();
    const actual = (await this.db.query<{ name: string }>("SELECT current_database() AS name")).rows[0]?.name;
    if (actual !== expected) throw new Error("Stage execution database identity mismatch");
    this.database = expected;
  }
  private async currentDatabase() {
    if (!this.database || (await this.db.query<{ name: string }>("SELECT current_database() AS name")).rows[0]?.name !== this.database) {
      throw new Error("Stage execution fixture has no verified database");
    }
  }

  /** An ordinary producer is setup only; the local packager is the real execution. */
  async ordinaryProducer(workspaceId: string, childId: string, ownerId: string, suffix: string) {
    await this.currentDatabase();
    const runtimeId = randomUUID(), agentId = randomUUID(), taskId = randomUUID();
    await this.db.query("BEGIN");
    try {
      const scope = await this.db.query(`SELECT i.id FROM issue i JOIN member m ON m.workspace_id=i.workspace_id AND m.user_id=$3
        WHERE i.id=$1 AND i.workspace_id=$2 AND NOT EXISTS(SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id)`, [childId, workspaceId, ownerId]);
      if (scope.rowCount !== 1) throw new Error("Ordinary artifact producer scope does not match the test child");
      await this.db.query(`INSERT INTO agent_runtime(id,workspace_id,name,runtime_mode,provider,status,device_info,metadata,last_seen_at,visibility,owner_id)
        VALUES($1,$2,$3,'cloud','stage_artifact_fixture','online','','{}',clock_timestamp(),'private',$4)`, [runtimeId, workspaceId, `Synthetic artifact runtime ${suffix}`, ownerId]);
      await this.db.query(`INSERT INTO agent(id,workspace_id,name,description,runtime_mode,runtime_config,visibility,permission_mode,max_concurrent_tasks,owner_id,runtime_id,autonomy_level)
        VALUES($1,$2,$3,'','cloud','{}','private','private',1,$4,$5,'operator')`, [agentId, workspaceId, `Synthetic artifact producer ${suffix}`, ownerId, runtimeId]);
      await this.db.query(`INSERT INTO agent_task_queue(id,agent_id,status,priority,issue_id,runtime_id,originator_user_id,accountable_user_id,claim_generation,started_at)
        VALUES($1,$2,'running',0,$3,$4,$5,$5,1,clock_timestamp())`, [taskId, agentId, childId, runtimeId, ownerId]);
      await this.db.query("COMMIT");
    } catch (error) { await this.db.query("ROLLBACK"); throw error; }
    return { runtimeId, agentId, taskId, workspaceId, childId, ownerId, source: "synthetic_ordinary_producer_setup" };
  }

  async executionSources(workspaceId: string, suffix: string) {
    await this.currentDatabase();
    const deployment = randomUUID(), metric = randomUUID();
    await this.db.query("BEGIN");
    try {
      for (const [id, kind] of [[deployment, "deployment"], [metric, "metric"]] as const) {
        const remote = Number.parseInt(id.replaceAll("-", "").slice(0, 10), 16) + 1;
        await this.db.query(`INSERT INTO github_installation(id,workspace_id,installation_id,account_login,account_type)
          VALUES($1,$2,$3,$4,'User')`, [id, workspaceId, remote, `controlled-stage-${kind}-${suffix}`]);
      }
      await this.db.query("COMMIT");
    } catch (error) { await this.db.query("ROLLBACK"); throw error; }
    return { deployment_integration_id: deployment, metric_integration_id: metric };
  }

  async packageBuild(actor: ClosureActor, apiUrl: string, producer: Awaited<ReturnType<StageExecutionFixture["ordinaryProducer"]>>,
    git: ReturnType<typeof createStageGitInput>, head: string, slot: number, record: StageEvidenceRecorder, retain: (fact: StageHTTPFact) => StageHTTPFact) {
    await this.currentDatabase();
    const allocation = stageArtifactEnvironment(), stagingId = allocation.slots[slot];
    if (!stagingId) throw new Error("No allocated artifact generation for this scenario");
    const directory = join(allocation.staging, stagingId), files = join(directory, "files");
    const info = lstatSync(directory);
    if (!info.isDirectory() || info.isSymbolicLink() || (info.mode & 0o777) !== 0o700 || info.uid !== process.getuid?.() ||
      existsSync(join(directory, "manifest.json")) || existsSync(files)) throw new Error("Artifact generation is already used or differs from its allocation");
    if (head !== git.head && head !== git.initialHead) throw new Error("Build must use an actual selected fixture commit");
    const sourceBytes = execFileSync(git.config.executable, ["-c", "core.hooksPath=/dev/null", "-C", join(git.directory, "source"), "show", `${head}:stage-regression.json`], {
      env: { PATH: process.env.PATH, HOME: git.directory, GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: "/dev/null", GIT_CONFIG_SYSTEM: "/dev/null", GIT_TERMINAL_PROMPT: "0" },
      maxBuffer: 16 * 1024, timeout: 20_000,
    });
    const entries = [{ path: "stage-regression.json", size_bytes: sourceBytes.length, sha256: stageBytesHash(sourceBytes) }];
    const inputDigest = stageBytesHash(`multica-artifact-input-v1\n${JSON.stringify(entries)}`);
    const manifest = { workspace_id: producer.workspaceId, agent_id: producer.agentId, task_id: producer.taskId,
      task_generation: 1, resource_id: producer.childId, source_revision: head, input_manifest_digest: inputDigest };
    const manifestBytes = JSON.stringify(manifest);
    mkdirSync(files, { mode: 0o700 });
    writeFileSync(join(files, "stage-regression.json"), sourceBytes, { flag: "wx", mode: 0o400 });
    writeFileSync(join(directory, "manifest.json"), manifestBytes, { flag: "wx", mode: 0o400 });
    chmodSync(files, 0o500); chmodSync(directory, 0o500);
    const request = { agent_id: producer.agentId, task_id: producer.taskId, resource_kind: "issue", resource_id: producer.childId,
      action: "package_release_candidate", staging_id: stagingId, source_revision: head,
      staging_manifest_digest: stageBytesHash(`multica-artifact-staging-v1\n${manifestBytes}`), input_manifest_digest: inputDigest,
      entries, target_environment: "local-synthetic", policy_version: "artifact-tar-v1" };
    const headers = { "X-Workspace-ID": producer.workspaceId };
    const created = retain(await stageRequest(apiUrl, actor, "/api/agent-approvals/artifact-package", "POST", request, undefined, headers));
    expect(created.status).toBe(201);
    const pending = artifactReply(created.response_body);
    expect(pending).toMatchObject({ status: "unclaimed", approval: { status: "pending" } });
    const base = `/api/agent-approvals/${pending.approval.id}`;
    const approved = retain(await stageRequest(apiUrl, actor, `${base}/decision`, "POST", {
      decision: "approve", expected_spec_digest: pending.spec_digest, note: "Synthetic human reviewed the exact local packager input and source commit.",
    }, undefined, headers));
    expect(approved.status).toBe(200);
    const execution = retain(await stageRequest(apiUrl, actor, `${base}/artifact-package/execution`, "POST", { expected_spec_digest: pending.spec_digest }, undefined, headers));
    expect(execution.status).toBe(200);
    const completed = artifactReply(execution.response_body);
    expect(completed).toMatchObject({ execution_id: pending.execution_id, spec_digest: pending.spec_digest, status: "succeeded",
      approval: { id: pending.approval.id, status: "executed", decided_by: actor.id } });
    const path = join(allocation.output, completed.execution_id, "artifact.tar"), stat = lstatSync(path);
    if (!stat.isFile() || stat.isSymbolicLink() || stat.size > 32 * 1024) throw new Error("Actual artifact is not the bounded local tar output");
    const proof = verifyStageArtifactTar(readFileSync(path), sourceBytes);
    expect(completed.receipt).toMatchObject({ output_digest: proof.artifact_digest, output_bytes: proof.size_bytes });
    const persisted = await this.db.query<{ output_artifact_digest: string; source_revision: string; result_status: string }>(
      "SELECT output_artifact_digest,source_revision,result_status FROM agent_artifact_execution WHERE execution_id=$1 AND approval_id=$2 AND workspace_id=$3 AND resource_id=$4",
      [completed.execution_id, pending.approval.id, producer.workspaceId, producer.childId]);
    expect(persisted.rows).toEqual([{ output_artifact_digest: proof.artifact_digest, source_revision: head, result_status: "succeeded" }]);
    record("real local artifact API and tar output", { approval_id: pending.approval.id, execution_id: completed.execution_id,
      spec_digest: completed.spec_digest, receipt_digest: completed.receipt_digest, source_revision: head, ...proof,
      ordinary_producer: producer, signatures: "synthetic human account", external_deployment: false });
    return { approvalId: pending.approval.id, executionId: completed.execution_id, specDigest: completed.spec_digest,
      artifactDigest: proof.artifact_digest, receiptDigest: completed.receipt_digest, sourceRevision: head, childId: producer.childId };
  }

  async rootEffects(issueId: string) {
    await this.currentDatabase();
    const result = await this.db.query(`SELECT i.status,i.project_id,i.revision,c.revision AS collaboration_revision,
      (SELECT count(*)::int FROM collaboration_event e WHERE e.issue_id=i.id AND e.kind='acceptance_submitted') submissions,
      (SELECT count(*)::int FROM collaboration_stage_decision d WHERE d.issue_id=i.id) decisions
      FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id WHERE i.id=$1`, [issueId]);
    if (result.rows.length !== 1) throw new Error("Stage effect snapshot lost its original issue");
    return result.rows[0];
  }
  async completionSnapshot(issueId: string, childId: string, projectId: string) {
    await this.currentDatabase();
    const result = await this.db.query<{ issue_revision: string; child_revision: string; project_revision: string }>(`SELECT
      i.revision::text AS issue_revision,child.revision::text AS child_revision,p.revision::text AS project_revision
      FROM issue i JOIN issue child ON child.parent_issue_id=i.id AND child.workspace_id=i.workspace_id AND child.project_id=i.project_id
      JOIN project p ON p.id=i.project_id AND p.workspace_id=i.workspace_id WHERE i.id=$1 AND child.id=$2 AND p.id=$3`, [issueId, childId, projectId]);
    if (result.rows.length !== 1) throw new Error("Completion evidence no longer belongs to the exact parent/project");
    const row = result.rows[0]!;
    const numbers = { issueRevision: Number(row.issue_revision), childRevision: Number(row.child_revision), projectRevision: Number(row.project_revision) };
    if (Object.values(numbers).some((n) => !Number.isSafeInteger(n) || n < 0)) throw new Error("Completion revision cannot be represented exactly");
    return numbers;
  }
  async close() { await this.db.end(); }
}

export async function promoteStageArtifactReader(owner: ClosureActor, apiUrl: string, workspaceId: string, memberId: string, actorId: string) {
  const fact = await stageRequest(apiUrl, owner, `/api/workspaces/${workspaceId}/members/${memberId}`, "PATCH", { role: "admin" }, undefined, { "X-Workspace-ID": workspaceId });
  expect(fact.status).toBe(200);
  const members = await owner.client.requestJSON<{ id: string; user_id: string; role: string }[]>(`/api/workspaces/${workspaceId}/members`, { headers: { "X-Workspace-ID": workspaceId } });
  expect(members.find((member) => member.id === memberId)).toMatchObject({ user_id: actorId, role: "admin" });
  return fact;
}

export async function configureStageExecution(control: StageExecutionControl, workspaceId: string,
  sources: { deployment_integration_id: string; metric_integration_id: string }, previousArtifact: string, suffix: string) {
  const request: Extract<StageControlRequest, { action: "source.configure" }> = { request_id: randomUUID(), action: "source.configure", workspace_id: workspaceId,
    ...sources, environment: `synthetic-stage-${suffix}`, remote_instance: "controlled-local-stage-fixture", remote_tenant: suffix,
    remote_target: `target-${suffix}`, conditional_version: "1", current_artifact_digest: previousArtifact,
    query_id: "synthetic-payment-error-rate", query_digest: stageBytesHash("controlled-query-v1"), rule_id: "synthetic-test-rule",
    rule_digest: stageBytesHash("controlled-rule-v1"), metric_freshness_seconds: 120 };
  const reply = await control.send(request);
  if (!reply.ok) throw new Error(`Controlled target configuration refused: ${reply.code}`);
  const target = stageString(reply.result.target_digest, "target digest");
  expect(target).toMatch(/^[0-9a-f]{64}$/);
  return { request, target, callBaseline: reply.calls.count };
}

export function stageControlSucceeded(reply: StageControlReply) {
  if (!reply.ok) throw new Error(`Controlled stage request refused: ${reply.code}`);
  return reply.result;
}

export async function stageAdvanceAPI(apiUrl: string, actor: ClosureActor, issueId: string, candidate: CollaborationStageCandidateRef,
  target: string, decisions: string[] = []) {
  return stageRequest(apiUrl, actor, `${stageRoot(issueId)}/advance`, "POST", { ...await stageObservations(actor, issueId),
    candidate, target_stage: target, decision_ids: decisions });
}
export async function stageMoveUI(page: Page, actor: ClosureActor, issueId: string, target: string) {
  const label = `Move to ${target}`;
  await stagePanel(page).getByRole("button", { name: label, exact: true }).click();
  const form = page.getByRole("form", { name: label, exact: true });
  const fact = await stageUI(page, actor, `${stageRoot(issueId)}/advance`, "POST", () => form.getByRole("button", { name: "Confirm stage change", exact: true }).click());
  stageSuccess(fact, "advance_stage"); await expect(form).toHaveCount(0);
  return fact;
}

export async function stageApprovalUI(page: Page, step: "code_review" | "qa" | "release" | "rollback",
  selected: { dependency?: string; report?: string; target?: string; deployment?: string; observation?: string }, reason: string) {
  const label = { code_review: "Approve code review", qa: "Approve QA", release: "Approve release", rollback: "Approve rollback" }[step];
  await stagePanel(page).getByRole("button", { name: label, exact: true }).click();
  const form = page.getByRole("form", { name: label, exact: true });
  if (selected.report) await form.getByLabel("Test report", { exact: true }).selectOption(selected.report);
  if (selected.target) await form.getByLabel("Deployment target", { exact: true }).selectOption(selected.target);
  if (selected.deployment) await form.getByLabel("Original deployment", { exact: true }).selectOption(selected.deployment);
  if (selected.observation) await form.getByLabel("Metric observation", { exact: true }).selectOption(selected.observation);
  if (selected.dependency) await form.getByLabel("Preceding approval", { exact: true }).selectOption(selected.dependency);
  await form.getByLabel("Reason", { exact: true }).fill(reason);
  await form.getByRole("checkbox", { name: "I reviewed this exact candidate and evidence", exact: true }).check();
  await expect(form.getByRole("button", { name: "Record approval", exact: true })).toBeEnabled();
  return { form, submit: () => form.getByRole("button", { name: "Record approval", exact: true }).click() };
}

export async function stageSubmissionRequest(actor: ClosureActor, issueId: string, text: string) {
  const issue = await readClosureIssue(actor, issueId);
  const options = await actor.client.requestJSON<{ default_status: string; status_options_version: string }>(`/api/collaboration/issues/${issueId}/status-options?action=submit_acceptance`);
  return { ...workflowAuthority(issue), expected_status: options.default_status, status_options_version: options.status_options_version,
    note: text, basis: "Synthetic stage fixture evidence", attachment_ids: [] };
}

export async function setStageM1Policy(actor: ClosureActor, apiUrl: string, workspaceId: string, projectId: string, mode: "off" | "observe" | "enforce") {
  const headers = { "X-Workspace-ID": workspaceId };
  const current = await actor.client.requestJSON<{ revision: number }>(`/api/projects/${projectId}/delivery-policy`, { headers });
  const fact = await stageRequest(apiUrl, actor, `/api/projects/${projectId}/delivery-policy`, "PUT", {
    expected_policy_revision: current.revision, mode, policy_version: "delivery-artifact-observe-v1", stage: "done",
    reason: `Synthetic explicit M1 ${mode} policy; stage approval remains independent`,
  }, undefined, headers);
  expect(fact.status).toBe(200); stageObject(fact.response_body);
  expect(fact.response_body.mode).toBe(mode);
  const revision = fact.response_body.revision;
  if (typeof revision !== "number" || !Number.isSafeInteger(revision)) throw new Error("M1 policy omitted its actual revision");
  return { fact, revision };
}

export async function stageCompleteBody(actor: ClosureActor, issueId: string, options: { submissionId?: string; result?: string; note?: string } = {}) {
  const issue = await readClosureIssue(actor, issueId);
  const status = await actor.client.requestJSON<{ default_status: string; status_options_version: string }>(`/api/collaboration/issues/${issueId}/status-options?action=complete`);
  const note = options.note ?? "Synthetic explicit final acceptance";
  return { ...workflowAuthority(issue), action: "complete", status: status.default_status, status_options_version: status.status_options_version,
    note: options.result ?? note, basis: options.result ? "Actual source-child tar verified" : "", attachment_ids: [],
    ...(options.submissionId ? { submission_id: options.submissionId } : {}) };
}

export async function stageAuthorizeM1(fixture: StageExecutionFixture, actor: ClosureActor, apiUrl: string,
  input: { issueId: string; workspaceId: string; projectId: string; policyRevision: number;
    build: { childId: string; approvalId: string; artifactDigest: string; specDigest: string; sourceRevision: string };
    submissionId?: string; result?: string; note?: string }) {
  const issue = await readClosureIssue(actor, input.issueId);
  const body = await stageCompleteBody(actor, input.issueId, input);
  const versions = await fixture.completionSnapshot(input.issueId, input.build.childId, input.projectId);
  const target = { kind: input.submissionId ? "collaboration_submission" : "collaboration_self", issue_id: input.issueId,
    expected_issue_revision: versions.issueRevision, expected_status: issue.status, target_status: body.status,
    expected_collaboration_revision: body.expected_collaboration_revision, expected_lifecycle_epoch: body.expected_lifecycle_epoch,
    expected_participant_epoch: body.expected_participant_epoch, expected_account_epoch: body.expected_account_epoch,
    status_options_version: body.status_options_version,
    ...(input.submissionId ? { submission_id: input.submissionId, acceptance_note: body.note }
      : { result: body.note, basis: body.basis, draft_id: null, attachment_ids: [], source_comment_id: null }) };
  const operationId = randomUUID();
  const request = { contract_version: "delivery-completion-enforcement-v1", source_workspace_id: input.workspaceId, operation_id: operationId,
    target, project: { id: input.projectId, expected_revision: versions.projectRevision, expected_policy_revision: input.policyRevision },
    evidence: { issue_id: input.build.childId, expected_issue_revision: versions.childRevision, approval_id: input.build.approvalId,
      expected_source_revision: input.build.sourceRevision, expected_spec_digest: input.build.specDigest, expected_artifact_digest: input.build.artifactDigest },
    consumer: { kind: "human", user_id: actor.id }, expires_at: new Date(Date.now() + 60_000).toISOString(), reason: "Synthetic current source owner grants this exact completion only" };
  const fact = await stageRequest(apiUrl, actor, "/api/delivery-completion-authorizations", "POST", request);
  expect(fact.status).toBe(201); stageObject(fact.response_body);
  const authorizationId = stageString(fact.response_body.authorization_id, "M1 authorization ID");
  expect(fact.response_body).toMatchObject({ operation_id: operationId, issue_id: input.issueId, state: "issued" });
  return { fact, request, authorizationId, operationId, body: { ...body, completion: { authorization_id: authorizationId,
    operation_id: operationId, expected_issue_revision: versions.issueRevision } } };
}

export function selectedRelease(record: CollaborationStageCodeCandidate, decisionId: string) {
  const approval = record.decisions.find((item) => item.decision_id === decisionId);
  expect(approval).toMatchObject({ step: "release", decision: "approved", state: "valid" });
  const intents = record.execution_operations?.filter((item) => item.approval_decision_id === decisionId) ?? [];
  expect(intents).toHaveLength(1);
  const operation = intents[0]!;
  const fact = record.execution_facts.find((item) => item.fact_id === operation.fact_id);
  expect(fact).toMatchObject({ intent_id: operation.intent_id, approval_decision_id: decisionId,
    target_digest: operation.target_digest, artifact_digest: operation.artifact_digest });
  return { approval: approval!, operation, fact: fact! };
}

export async function stageCurrentApproval(actor: ClosureActor, issueId: string, candidate: CollaborationStageCandidateRef, step: string) {
  const view = await readCode(actor, issueId, candidate.candidate_id);
  return view.decisions.find((item) => item.step === step && item.decision === "approved" && item.state === "valid");
}
export const stageCurrent = async (actor: ClosureActor, issueId: string) => (await readStage(actor, issueId)).stage?.current;
