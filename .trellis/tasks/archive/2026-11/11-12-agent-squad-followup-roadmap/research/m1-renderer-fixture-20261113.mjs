import { createHash, randomUUID } from "node:crypto";
import { chmod, mkdir, readFile, writeFile } from "node:fs/promises";
import pg from "pg";

const apiBase = process.env.M1_API_BASE ?? "http://localhost:18573";
const databaseURL = process.env.M1_DATABASE_URL ?? process.env.DATABASE_URL;
const stagingRoot = process.env.M1_STAGING_ROOT ?? "/private/tmp/m1-s1-20261113-safe/staging";
const outputRoot = process.env.M1_OUTPUT_ROOT ?? "/private/tmp/m1-s1-20261113-safe/output";
const fixtureFile = process.env.M1_FIXTURE_FILE ?? "/private/tmp/m1-s1-20261113-safe/fixture.json";
if (!databaseURL) throw new Error("M1_DATABASE_URL or DATABASE_URL is required");

const sha256 = (value) => createHash("sha256").update(value).digest("hex");
const digest = (domain, value) => sha256(`${domain}\n${JSON.stringify(value)}`);

async function request(path, options = {}) {
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...(options.headers ?? {}) },
  });
  const text = await response.text();
  let body;
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  if (!response.ok) throw new Error(`${options.method ?? "GET"} ${path} -> ${response.status}: ${text}`);
  return body;
}

async function stageCandidate({ workspaceID, issueID, agentID, taskID }) {
  const sourceRevision = "a".repeat(40);
  const entry = { path: "candidate.txt", size_bytes: 9, sha256: sha256("candidate") };
  const inputManifestDigest = digest("multica-artifact-input-v1", [entry]);
  const stagingID = randomUUID();
  const stagingManifest = {
    workspace_id: workspaceID, agent_id: agentID, task_id: taskID, task_generation: 1,
    resource_id: issueID, source_revision: sourceRevision, input_manifest_digest: inputManifestDigest,
  };
  const stagingManifestDigest = digest("multica-artifact-staging-v1", stagingManifest);
  const requestBody = {
    agent_id: agentID, task_id: taskID, resource_kind: "issue", resource_id: issueID,
    action: "package_release_candidate", staging_id: stagingID, source_revision: sourceRevision,
    staging_manifest_digest: stagingManifestDigest, input_manifest_digest: inputManifestDigest,
    entries: [entry], target_environment: "local-synthetic", policy_version: "artifact-tar-v1",
  };
  const stagingDir = `${stagingRoot}/${stagingID}`;
  await mkdir(`${stagingDir}/files`, { recursive: true, mode: 0o700 });
  await writeFile(`${stagingDir}/manifest.json`, JSON.stringify(stagingManifest), { mode: 0o400 });
  await writeFile(`${stagingDir}/files/candidate.txt`, "candidate", { mode: 0o400 });
  await chmod(`${stagingDir}/files`, 0o500);
  await chmod(stagingDir, 0o500);
  return { sourceRevision, requestBody, stagingDir };
}

async function main() {
  if (process.argv[2] === "package") return packageStagedFixture();
  const suffix = randomUUID().slice(0, 12);
  const email = `m1-renderer-${suffix}@example.invalid`;
  const slug = `m1-renderer-${suffix}`;
  const db = new pg.Client(databaseURL);
  await db.connect();
  let token;
  let workspace;
  try {
    await request("/auth/send-code", { method: "POST", body: JSON.stringify({ email }) });
    const code = process.env.MULTICA_DEV_VERIFICATION_CODE ?? "888888";
    const verified = await request("/auth/verify-code", { method: "POST", body: JSON.stringify({ email, code }) });
    token = verified.token;
    if (typeof token !== "string" || token.length === 0) throw new Error("verification did not return a token");
    const authHeaders = { Authorization: `Bearer ${token}` };
    await db.query(`UPDATE "user" SET onboarded_at=now(), onboarding_questionnaire='{"source":["friends_colleagues"],"source_skipped":false}'::jsonb WHERE email=$1`, [email]);
    workspace = await request("/api/workspaces", {
      method: "POST", headers: authHeaders, body: JSON.stringify({ name: `M1 renderer ${suffix}`, slug }),
    });
    const workspaceID = workspace.id;
    const workspaceHeaders = { ...authHeaders, "X-Workspace-ID": workspaceID };
    const project = await request("/api/projects", {
      method: "POST", headers: workspaceHeaders,
      body: JSON.stringify({ title: `M1 renderer project ${suffix}`, description: "Task-owned enforce fixture", status: "planned", priority: "none" }),
    });
    const issues = await Promise.all(["web", "desktop"].map((surface) => request("/api/issues", {
      method: "POST", headers: workspaceHeaders,
      body: JSON.stringify({ title: `M1 renderer ${surface} ${suffix}`, description: "Fixed local synthetic evidence", project_id: project.id, status: "in_review" }),
    })));
    const policy = await request(`/api/projects/${project.id}/delivery-policy?workspace_id=${workspaceID}`, {
      method: "PUT", headers: workspaceHeaders,
      body: JSON.stringify({ expected_policy_revision: 0, mode: "enforce", policy_version: "delivery-artifact-observe-v1", stage: "done", reason: "Task-owned renderer acceptance fixture" }),
    });

    const runtimeResult = await db.query(`
      INSERT INTO agent_runtime
        (workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at, owner_id, visibility)
      VALUES ($1, gen_random_uuid(), $2, 'local', 'codex', 'online', 'Task-owned synthetic fixture', '{"capabilities":["rpc-v1"]}'::jsonb, now(), $3, 'private')
      RETURNING id`, [workspaceID, `M1 renderer runtime ${suffix}`, verified.user.id]);
    const runtimeID = runtimeResult.rows[0].id;
    const agentResult = await db.query(`
      INSERT INTO agent
        (workspace_id, name, description, runtime_mode, runtime_config, visibility, permission_mode, max_concurrent_tasks, owner_id, runtime_id, autonomy_level)
      VALUES ($1, $2, '', 'cloud', '{}'::jsonb, 'private', 'private', 1, $3, $4, 'operator')
      RETURNING id`, [workspaceID, `M1 renderer operator ${suffix}`, verified.user.id, runtimeID]);
    const agentID = agentResult.rows[0].id;
    await mkdir(stagingRoot, { recursive: true, mode: 0o700 });
    await chmod(stagingRoot, 0o700);
    const staged = [];
    for (const issue of issues) {
      const taskResult = await db.query(`
        INSERT INTO agent_task_queue
          (agent_id, status, priority, runtime_id, issue_id, originator_user_id, accountable_user_id, claim_generation)
        VALUES ($1, 'running', 0, $2, $3, $4, $4, 1)
        RETURNING id`, [agentID, runtimeID, issue.id, verified.user.id]);
      const taskID = taskResult.rows[0].id;
      const evidence = await stageCandidate({ workspaceID, issueID: issue.id, agentID, taskID });
      staged.push({ issue, taskID, ...evidence });
    }
    await mkdir(outputRoot, { recursive: true, mode: 0o700 });
    await chmod(stagingRoot, 0o500);
    await chmod(outputRoot, 0o700);
    const fixtures = [];
    for (const [index, item] of staged.entries()) {
      const prResult = await db.query(`
        INSERT INTO github_pull_request
          (workspace_id, installation_id, repo_owner, repo_name, pr_number, title, state, html_url, pr_created_at, pr_updated_at, head_sha)
        VALUES ($1, 1, 'm1-renderer', $2, 1, 'M1 deterministic evidence', 'open', 'https://example.invalid/m1-renderer', now(), now(), $3)
        RETURNING id`, [workspaceID, `repo-${suffix}-${index}`, item.sourceRevision]);
      await db.query("INSERT INTO issue_pull_request(issue_id, pull_request_id) VALUES ($1, $2)", [item.issue.id, prResult.rows[0].id]);
      fixtures.push({
        surface: index === 0 ? "web" : "desktop", issueID: item.issue.id,
        issueIdentifier: item.issue.identifier, issueTitle: item.issue.title,
        issueRevision: item.issue.revision, sourceRevision: item.sourceRevision, requestBody: item.requestBody,
      });
    }
    const fixture = {
      apiBase, email, token, workspaceID, workspaceSlug: slug, projectID: project.id,
      policyRevision: policy.revision, fixtures, createdAt: new Date().toISOString(),
      syntheticOnly: true, providerCalls: 0, artifactPackaging: "staged_only",
    };
    await writeFile(fixtureFile, JSON.stringify(fixture, null, 2), { mode: 0o600 });
    console.log(JSON.stringify({ ...fixture, token: "<task-owned-token>" }, null, 2));
  } finally {
    await db.end();
  }
}

async function packageStagedFixture() {
  const fixture = JSON.parse(await readFile(fixtureFile, "utf8"));
  const workspaceHeaders = {
    Authorization: `Bearer ${fixture.token}`,
    "X-Workspace-ID": fixture.workspaceID,
  };
  for (const item of fixture.fixtures) {
    const idempotencyKey = randomUUID();
    const artifact = await request("/api/agent-approvals/artifact-package", {
      method: "POST", headers: { ...workspaceHeaders, "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(item.requestBody),
    });
    const approval = artifact.approval;
    const routeHeaders = { ...workspaceHeaders, "Idempotency-Key": idempotencyKey };
    await request(`/api/agent-approvals/${approval.id}/decision`, {
      method: "POST", headers: routeHeaders,
      body: JSON.stringify({ decision: "approve", note: "Task-owned deterministic renderer evidence", expected_spec_digest: artifact.spec_digest }),
    });
    const executed = await request(`/api/agent-approvals/${approval.id}/artifact-package/execution`, {
      method: "POST", headers: routeHeaders,
      body: JSON.stringify({ expected_spec_digest: artifact.spec_digest }),
    });
    Object.assign(item, {
      approvalID: approval.id, executionID: executed.execution_id,
      specDigest: artifact.spec_digest, artifactDigest: executed.output_digest,
      receiptDigest: executed.receipt_digest,
    });
    delete item.requestBody;
  }
  fixture.artifactPackaging = "local_synthetic_success";
  await writeFile(fixtureFile, JSON.stringify(fixture, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ ...fixture, token: "<task-owned-token>" }, null, 2));
}

await main();
