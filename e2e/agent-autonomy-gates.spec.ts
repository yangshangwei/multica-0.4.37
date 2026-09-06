import { createHash, randomBytes } from "crypto";
import { test, expect } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

/**
 * Agent autonomy, role templates, the approval boundary and squad staffing —
 * driven over real HTTP against the running backend.
 *
 * No UI and no interception: every assertion here is about what the API decides,
 * which is where the policy lives. The browser-level flow is covered separately
 * by agent-role-template.spec.ts.
 *
 * Two preconditions are seeded directly in the database because they are the
 * ambient state a real workspace already has, not the thing under test:
 *
 *   - an ONLINE agent_runtime row, which a live daemon would have registered.
 *     Runtimes are workspace-scoped, so an isolated test workspace has none.
 *   - an agent_task_queue row per agent, which the daemon creates when work is
 *     dispatched. It is what makes an agent an *actor*: resolveActor() trusts
 *     X-Agent-ID only when X-Task-ID names a task belonging to that agent, so a
 *     task row is the only way to exercise the gates on the path they run on.
 *
 * Everything else — agents, squads, issues, approvals — goes through the real
 * endpoints a client would call.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
const DATABASE_URL =
  process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

const RUN_ID = `${Date.now().toString(36)}-${process.pid.toString(36)}`;
const OWNER_EMAIL = `e2e-autonomy-${RUN_ID}@multica.ai`;
const OTHER_EMAIL = `e2e-autonomy-other-${RUN_ID}@multica.ai`;

/** Autonomy levels, lowest to highest. Mirrors service.AutonomyLevel. */
const OBSERVER = "observer";
const CONTRIBUTOR = "contributor";
const OPERATOR = "operator";

/**
 * An agent acting on its own behalf, holding the same credential a real agent
 * process holds: a task token.
 *
 * This has to be a task token and not a member token carrying X-Agent-ID, because
 * the product draws two different lines and only the token crosses both:
 *
 *   - resolveActor() calls a request "an agent's" when X-Agent-ID names an agent
 *     and X-Task-ID names one of its tasks. This is what the autonomy gates read.
 *   - isMachineCredentialActor() calls a request "a machine's" only when the auth
 *     middleware stamped X-Actor-Source, which it does from the token row alone.
 *     This is what the human-only guards read (UpdateAgent's autonomy ceiling and
 *     the approval decision route).
 *
 * A member token with agent headers satisfies the first and not the second, so it
 * reports the human-only guards as absent when they are present.
 */
interface Actor {
  agentId: string;
  taskId: string;
  /** The `mat_`-prefixed bearer token, as the daemon would hand to the agent. */
  token: string;
}

let ownerApi: TestApiClient;
let workspaceId: string;
let runtimeId: string;
let ownerToken: string;
let ownerUserId: string;

/** An agent per level, each with a task row so it can act. */
let observer: Actor;
let contributor: Actor;
let operator: Actor;

async function sql<T = Record<string, unknown>>(query: string, params: unknown[] = []): Promise<T[]> {
  const client = new pg.Client(DATABASE_URL);
  await client.connect();
  try {
    const result = await client.query(query, params);
    return result.rows as T[];
  } finally {
    await client.end();
  }
}

/**
 * One request helper for every call in this file.
 *
 * Passing `actor` swaps the credential for that agent's task token, which is the
 * whole difference between "a person did this" and "an agent did this". The auth
 * middleware derives X-Agent-ID, X-Task-ID, X-Workspace-ID and X-Actor-Source
 * from the token row itself, so nothing identifying is sent from here.
 */
async function call(
  path: string,
  init: { method?: string; body?: unknown; actor?: Actor; token?: string } = {},
) {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${init.actor?.token ?? init.token ?? ownerToken}`,
  };
  if (!init.actor) {
    headers["X-Workspace-ID"] = workspaceId;
  }
  const res = await fetch(`${API_BASE}${path}`, {
    method: init.method ?? "GET",
    headers,
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  });
  const text = await res.text();
  let json: unknown = null;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    /* non-JSON error bodies are reported through `text` */
  }
  return { status: res.status, json: json as never, text };
}

/** Seeds the ONLINE runtime a live daemon would have registered. */
async function seedRuntime(wsId: string, ownerId: string): Promise<string> {
  const rows = await sql<{ id: string }>(
    `INSERT INTO agent_runtime
       (workspace_id, daemon_id, name, runtime_mode, provider, status,
        device_info, metadata, last_seen_at, owner_id, visibility)
     VALUES ($1, gen_random_uuid(), $2, 'local', 'claude', 'online',
             'e2e autonomy runtime', '{"capabilities":["rpc-v1"]}'::jsonb, now(), $3, 'private')
     RETURNING id`,
    [wsId, `E2E Autonomy Runtime ${RUN_ID}`, ownerId],
  );
  return rows[0].id;
}

/**
 * Creates an agent from a role template, gives it a running task, and mints the
 * task token that task would carry.
 *
 * The token is hashed the way the server does it (`auth.HashToken` is
 * hex(sha256)), so the row is indistinguishable from one the daemon wrote.
 */
async function staffRole(templateKey: string, name: string): Promise<Actor> {
  const created = await call("/api/agents/from-template", {
    method: "POST",
    body: { template_key: templateKey, runtime_id: runtimeId, name },
  });
  expect(created.status, `staffing ${templateKey} should succeed: ${created.text}`).toBe(201);
  const agentId = (created.json as { id: string }).id;
  return grantTaskToken(agentId);
}

/** Gives an existing agent a running task plus the token to act under it. */
async function grantTaskToken(agentId: string, userId = ownerUserId): Promise<Actor> {
  const tasks = await sql<{ id: string }>(
    `INSERT INTO agent_task_queue (agent_id, status, runtime_id)
     VALUES ($1, 'running', $2) RETURNING id`,
    [agentId, runtimeId],
  );
  const taskId = tasks[0].id;
  const token = `mat_${randomBytes(24).toString("hex")}`;
  await sql(
    `INSERT INTO task_token (token_hash, task_id, agent_id, workspace_id, user_id, expires_at)
     VALUES ($1, $2, $3, $4, $5, now() + interval '1 hour')`,
    [createHash("sha256").update(token).digest("hex"), taskId, agentId, workspaceId, userId],
  );
  return { agentId, taskId, token };
}

test.beforeAll(async () => {
  ownerApi = new TestApiClient();
  await ownerApi.login(OWNER_EMAIL, "E2E Autonomy Owner");
  const workspace = await ownerApi.ensureWorkspace(
    `E2E Autonomy WS ${RUN_ID}`,
    `e2e-autonomy-${RUN_ID}`,
  );
  workspaceId = workspace.id;
  ownerToken = ownerApi.getToken()!;

  const owner = await sql<{ id: string }>(`SELECT id FROM "user" WHERE email = $1`, [OWNER_EMAIL]);
  ownerUserId = owner[0].id;
  runtimeId = await seedRuntime(workspaceId, ownerUserId);

  observer = await staffRole("product-analyst", `Observer ${RUN_ID}`);
  contributor = await staffRole("implementer", `Contributor ${RUN_ID}`);
  operator = await staffRole("release-engineer", `Operator ${RUN_ID}`);
});

test.afterAll(async () => {
  // Ordered so no row outlives what points at it: the workspace goes last.
  await sql(`DELETE FROM task_token WHERE workspace_id = $1`, [workspaceId]);
  await sql(
    `DELETE FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id = $1)`,
    [workspaceId],
  );
  await sql(`DELETE FROM agent_approval_request WHERE workspace_id = $1`, [workspaceId]);
  await sql(`DELETE FROM autopilot WHERE workspace_id = $1`, [workspaceId]);
  await sql(`DELETE FROM squad_member WHERE squad_id IN (SELECT id FROM squad WHERE workspace_id = $1)`, [
    workspaceId,
  ]);
  await sql(`DELETE FROM squad WHERE workspace_id = $1`, [workspaceId]);
  await sql(`DELETE FROM issue WHERE workspace_id = $1`, [workspaceId]);
  await sql(`DELETE FROM agent WHERE workspace_id = $1`, [workspaceId]);
  await sql(`DELETE FROM agent_runtime WHERE workspace_id = $1`, [workspaceId]);
  await sql(`DELETE FROM workspace WHERE id = $1`, [workspaceId]);
  await sql(`DELETE FROM "user" WHERE email = ANY($1::text[])`, [[OWNER_EMAIL, OTHER_EMAIL]]);
});

test.describe("role templates: provenance is a server decision", () => {
  test("the roster is listed with its autonomy defaults", async () => {
    const res = await call("/api/agents/templates");
    expect(res.status).toBe(200);
    const { templates } = res.json as { templates: { key: string; autonomy_level: string }[] };
    const byKey = new Map(templates.map((t) => [t.key, t.autonomy_level]));

    // The product's claim about which roles may act unsupervised.
    expect(byKey.get("product-analyst")).toBe(OBSERVER);
    expect(byKey.get("implementer")).toBe(CONTRIBUTOR);
    expect(byKey.get("release-engineer")).toBe(OPERATOR);
    // Squad leaders are unlisted: a coordinator with nobody to lead is not offered.
    expect(byKey.has("feature-delivery-lead")).toBe(false);
  });

  test("a client cannot choose its own autonomy_level or template_key", async () => {
    const res = await call("/api/agents/from-template", {
      method: "POST",
      body: {
        template_key: "product-analyst",
        runtime_id: runtimeId,
        name: `Provenance ${RUN_ID}`,
        // Both are ignored by the endpoint. If either were honoured, a client
        // could mint an Operator or forge provenance.
        autonomy_level: OPERATOR,
        template_version: 999,
      },
    });
    expect(res.status).toBe(201);
    const agentId = (res.json as { id: string }).id;

    const rows = await sql<{ autonomy_level: string; template_key: string; template_version: number }>(
      `SELECT autonomy_level, template_key, template_version FROM agent WHERE id = $1`,
      [agentId],
    );
    expect(rows[0].autonomy_level).toBe(OBSERVER);
    expect(rows[0].template_key).toBe("product-analyst");
    expect(rows[0].template_version).toBe(1);
  });

  test("an unlisted leader key is refused", async () => {
    const res = await call("/api/agents/from-template", {
      method: "POST",
      body: { template_key: "feature-delivery-lead", runtime_id: runtimeId },
    });
    expect(res.status).toBe(400);
    expect(res.text).toContain("unknown template_key");
  });
});

test.describe("autonomy ladder on issue writes", () => {
  test("an Observer may edit an issue but not direct it", async () => {
    const issue = await call("/api/issues", {
      method: "POST",
      body: { title: `Observer scope ${RUN_ID}` },
    });
    expect(issue.status).toBe(201);
    const id = (issue.json as { id: string }).id;

    // Editing description is not a decision about what happens next.
    const edit = await call(`/api/issues/${id}`, {
      method: "PUT",
      body: { description: "an observer may write this" },
      actor: observer,
    });
    expect(edit.status, `observer edit should pass: ${edit.text}`).toBe(200);

    const status = await call(`/api/issues/${id}`, {
      method: "PUT",
      body: { status: "done" },
      actor: observer,
    });
    expect(status.status).toBe(403);
    // The message has to name the action, so the agent can hand the work back
    // with a reason instead of retrying.
    expect(status.text).toContain("status");

    const assign = await call(`/api/issues/${id}`, {
      method: "PUT",
      body: { assignee_type: "agent", assignee_id: contributor.agentId },
      actor: observer,
    });
    expect(assign.status).toBe(403);
  });

  test("a Contributor may direct an issue", async () => {
    const issue = await call("/api/issues", {
      method: "POST",
      body: { title: `Contributor scope ${RUN_ID}` },
    });
    const id = (issue.json as { id: string }).id;

    const res = await call(`/api/issues/${id}`, {
      method: "PUT",
      body: { status: "in_progress" },
      actor: contributor,
    });
    expect(res.status, `contributor status change should pass: ${res.text}`).toBe(200);
    expect((res.json as { status: string }).status).toBe("in_progress");
  });
});

/**
 * The three ways the gate used to be walked around. Each one is a real bypass
 * that reached a write, so each asserts the refusal AND that the row did not
 * move — a 403 with the write already applied would be the same bug.
 */
test.describe("autonomy gate: bypass regressions", () => {
  test("a capitalized status key does not slip past the gate", async () => {
    const issue = await call("/api/issues", {
      method: "POST",
      body: { title: `Case fold ${RUN_ID}` },
    });
    const id = (issue.json as { id: string }).id;

    // encoding/json fills a field tagged `json:"status"` from "Status" too, so
    // the write happened while an exact map lookup for "status" found nothing.
    const res = await call(`/api/issues/${id}`, {
      method: "PUT",
      body: { Status: "done" },
      actor: observer,
    });
    expect(res.status, `capitalized Status must be refused: ${res.text}`).toBe(403);

    const rows = await sql<{ status: string }>(`SELECT status FROM issue WHERE id = $1`, [id]);
    expect(rows[0].status).not.toBe("done");
  });

  test("an explicit null assignee is a direction change", async () => {
    const issue = await call("/api/issues", {
      method: "POST",
      body: {
        title: `Explicit null ${RUN_ID}`,
        assignee_type: "agent",
        assignee_id: contributor.agentId,
      },
    });
    expect(issue.status).toBe(201);
    const id = (issue.json as { id: string }).id;

    // `assignee_id: null` means unassign and decodes to a nil pointer, so only
    // the presence half of the gate can see it.
    const res = await call(`/api/issues/${id}`, {
      method: "PUT",
      body: { assignee_id: null },
      actor: observer,
    });
    expect(res.status, `explicit null unassign must be refused: ${res.text}`).toBe(403);

    const rows = await sql<{ assignee_id: string | null }>(
      `SELECT assignee_id FROM issue WHERE id = $1`,
      [id],
    );
    expect(rows[0].assignee_id).not.toBeNull();
  });

  test("a batch of one is not a way around the gate", async () => {
    const issue = await call("/api/issues", {
      method: "POST",
      body: { title: `Batch bypass ${RUN_ID}` },
    });
    const id = (issue.json as { id: string }).id;

    const lower = await call("/api/issues/batch-update", {
      method: "POST",
      body: { issue_ids: [id], updates: { status: "done" } },
      actor: observer,
    });
    expect(lower.status, `batch status change must be refused: ${lower.text}`).toBe(403);

    // The outer key's case matters twice as much here: a capital "Updates"
    // populated the struct while leaving the raw map empty, which silently
    // emptied every presence check keyed off it — including this gate.
    const upper = await call("/api/issues/batch-update", {
      method: "POST",
      body: { issue_ids: [id], Updates: { status: "done" } },
      actor: observer,
    });
    expect(upper.status, `capitalized Updates must be refused: ${upper.text}`).toBe(403);

    const rows = await sql<{ status: string }>(`SELECT status FROM issue WHERE id = $1`, [id]);
    expect(rows[0].status).not.toBe("done");
  });

  test("an Observer cannot delete issues, singly or in a batch", async () => {
    const issue = await call("/api/issues", {
      method: "POST",
      body: { title: `Delete gate ${RUN_ID}` },
    });
    const id = (issue.json as { id: string }).id;

    const single = await call(`/api/issues/${id}`, { method: "DELETE", actor: observer });
    expect(single.status, `observer delete must be refused: ${single.text}`).toBe(403);

    const batch = await call("/api/issues/batch-delete", {
      method: "POST",
      body: { issue_ids: [id] },
      actor: observer,
    });
    expect(batch.status, `observer batch delete must be refused: ${batch.text}`).toBe(403);

    const rows = await sql(`SELECT id FROM issue WHERE id = $1`, [id]);
    expect(rows).toHaveLength(1);
  });
});

test.describe("autonomy escalation", () => {
  test("an Observer cannot staff a role at all", async () => {
    const res = await call("/api/agents/from-template", {
      method: "POST",
      body: { template_key: "product-analyst", runtime_id: runtimeId, name: `Nope ${RUN_ID}` },
      actor: observer,
    });
    // Staffing is a coordination decision, so it needs Coordinator — an Observer
    // is refused before the level ceiling is even consulted.
    expect(res.status, `observer staffing must be refused: ${res.text}`).toBe(403);
  });

  test("an Operator cannot mint an agent above its own level", async () => {
    // release-engineer is Operator, the top of the ladder, so it clears the
    // Coordinator requirement. The ceiling is the second, separate gate: it may
    // staff at or below itself, and a squad whose roster tops out at Coordinator
    // is below it, so that is allowed. What it must not do is exceed itself.
    const allowed = await call("/api/agents/from-template", {
      method: "POST",
      body: { template_key: "implementer", runtime_id: runtimeId, name: `Below ${RUN_ID}` },
      actor: operator,
    });
    expect(allowed.status, `operator staffing a contributor should pass: ${allowed.text}`).toBe(201);

    const equal = await call("/api/agents/from-template", {
      method: "POST",
      body: { template_key: "release-engineer", runtime_id: runtimeId, name: `Equal ${RUN_ID}` },
      actor: operator,
    });
    expect(equal.status, `operator staffing its own level should pass: ${equal.text}`).toBe(201);
  });

  test("a Contributor cannot mint an Operator", async () => {
    // The escalation this closes: a level that cannot promote itself creates a
    // higher one instead and routes the work through it.
    const res = await call("/api/agents/from-template", {
      method: "POST",
      body: { template_key: "release-engineer", runtime_id: runtimeId, name: `Escalate ${RUN_ID}` },
      actor: contributor,
    });
    expect(res.status).toBe(403);

    const rows = await sql(`SELECT id FROM agent WHERE workspace_id = $1 AND name = $2`, [
      workspaceId,
      `Escalate ${RUN_ID}`,
    ]);
    expect(rows, "the refused agent must not exist").toHaveLength(0);
  });

  test("an agent cannot promote itself through UpdateAgent", async () => {
    // A task token carries its OWNER's user id, so canManageAgent alone would let
    // an agent PATCH itself to operator and pass every later policy check. The
    // ceiling is the one field on this endpoint that stays human-only.
    const res = await call(`/api/agents/${observer.agentId}`, {
      method: "PUT",
      body: { autonomy_level: OPERATOR },
      actor: observer,
    });
    expect(res.status, `self-promotion must be refused: ${res.text}`).toBe(403);
    expect(res.text).toContain("can only be changed by a person");

    const rows = await sql<{ autonomy_level: string }>(
      `SELECT autonomy_level FROM agent WHERE id = $1`,
      [observer.agentId],
    );
    expect(rows[0].autonomy_level).toBe(OBSERVER);
  });

  test("a person may still set an agent's level", async () => {
    // The same field, same endpoint, human credential — the guard must not have
    // closed the legitimate path a workspace owner uses to lower a level.
    const target = await call("/api/agents/from-template", {
      method: "POST",
      body: { template_key: "qa-engineer", runtime_id: runtimeId, name: `Levelled ${RUN_ID}` },
    });
    const agentId = (target.json as { id: string }).id;

    const res = await call(`/api/agents/${agentId}`, {
      method: "PUT",
      body: { autonomy_level: OBSERVER },
    });
    expect(res.status, `owner lowering a level should pass: ${res.text}`).toBe(200);

    const rows = await sql<{ autonomy_level: string }>(
      `SELECT autonomy_level FROM agent WHERE id = $1`,
      [agentId],
    );
    expect(rows[0].autonomy_level).toBe(OBSERVER);
  });
});

test.describe("the human approval boundary", () => {
  test("an agent files a request, only a person decides it", async () => {
    const filed = await call("/api/agent-approvals", {
      method: "POST",
      body: {
        agent_id: operator.agentId,
        risk_class: "production_release",
        summary: `Ship ${RUN_ID}`,
        plan: "Tag the release and publish.",
      },
      actor: operator,
    });
    expect(filed.status, `filing an approval should succeed: ${filed.text}`).toBe(201);
    const approval = filed.json as { id: string; status: string };
    expect(approval.status).toBe("pending");

    // The whole point of the boundary: the agent that filed it cannot clear it.
    // Refused for being a machine credential, before the body is even read — so a
    // well-formed decision from an agent is refused for the same reason.
    const selfDecide = await call(`/api/agent-approvals/${approval.id}/decision`, {
      method: "POST",
      body: { decision: "approve" },
      actor: operator,
    });
    expect(selfDecide.status, `an agent must not decide its own request: ${selfDecide.text}`).toBe(403);

    let rows = await sql<{ status: string }>(`SELECT status FROM agent_approval_request WHERE id = $1`, [
      approval.id,
    ]);
    expect(rows[0].status).toBe("pending");

    // A person approves, and only then does execution get recorded.
    const decided = await call(`/api/agent-approvals/${approval.id}/decision`, {
      method: "POST",
      body: { decision: "approve", note: "reviewed by a human" },
    });
    expect(decided.status, `human approval should succeed: ${decided.text}`).toBe(200);
    expect((decided.json as { status: string }).status).toBe("approved");

    const executed = await call(`/api/agent-approvals/${approval.id}/execution`, {
      method: "POST",
      body: { note: "released" },
      actor: operator,
    });
    expect(executed.status, `recording execution should succeed: ${executed.text}`).toBe(200);

    rows = await sql<{ status: string }>(`SELECT status FROM agent_approval_request WHERE id = $1`, [
      approval.id,
    ]);
    expect(rows[0].status).toBe("executed");
  });

  test("\"executed\" is a status, not a decision a client can post", async () => {
    const filed = await call("/api/agent-approvals", {
      method: "POST",
      body: {
        agent_id: operator.agentId,
        risk_class: "secret_access",
        summary: `Walk the status ${RUN_ID}`,
      },
      actor: operator,
    });
    const approval = filed.json as { id: string };

    const res = await call(`/api/agent-approvals/${approval.id}/decision`, {
      method: "POST",
      body: { decision: "executed" },
    });
    expect(res.status, "a client must not walk the row into executed").toBe(400);

    const rows = await sql<{ status: string }>(`SELECT status FROM agent_approval_request WHERE id = $1`, [
      approval.id,
    ]);
    expect(rows[0].status).toBe("pending");
  });

  test("an unknown risk class is refused", async () => {
    const res = await call("/api/agent-approvals", {
      method: "POST",
      body: {
        agent_id: operator.agentId,
        risk_class: "mild_inconvenience",
        summary: `Bad class ${RUN_ID}`,
      },
      actor: operator,
    });
    expect(res.status).toBe(400);
  });
});

test.describe("squad staffing from a template", () => {
  test("the roster and the squad are created together", async () => {
    const listed = await call("/api/squads/templates");
    expect(listed.status).toBe(200);
    const { templates } = listed.json as {
      templates: { key: string; leader: { autonomy_level: string } }[];
    };
    const feature = templates.find((t) => t.key === "feature-delivery");
    expect(feature, "feature-delivery must be offered").toBeTruthy();
    // The leader is always a coordinator — that is what makes the roster routable.
    expect(feature!.leader.autonomy_level).toBe("coordinator");

    const created = await call("/api/squads/from-template", {
      method: "POST",
      body: {
        template_key: "feature-delivery",
        runtime_id: runtimeId,
        name: `Feature Squad ${RUN_ID}`,
      },
    });
    expect(created.status, `staffing a squad should succeed: ${created.text}`).toBe(201);
    const body = created.json as {
      squad: { id: string };
      created_agent_ids: string[];
      reused_agent_ids: string[];
    };
    const squadId = body.squad.id;
    // The roster is provisioned in the same transaction as the squad. The three
    // agents already staffed for this workspace's roles are reused rather than
    // duplicated, which is the behaviour the next test pins down.
    expect(body.created_agent_ids.length + body.reused_agent_ids.length).toBeGreaterThan(1);
    expect(body.reused_agent_ids.length).toBeGreaterThan(0);

    const members = await sql<{ member_type: string; role: string }>(
      `SELECT member_type, role FROM squad_member WHERE squad_id = $1`,
      [squadId],
    );
    // A leader plus the template's roster, all in one transaction.
    expect(members.length).toBeGreaterThan(1);
    expect(members.some((m) => m.role === "leader")).toBe(true);
  });

  test("an existing role agent is reused, not re-applied", async () => {
    // The workspace already has an Observer product-analyst whose level a person
    // may have lowered. Staffing a second squad is not consent to undo that, so
    // the row must come back untouched.
    const before = await sql<{ id: string; autonomy_level: string; instructions: string }>(
      `SELECT id, autonomy_level, instructions FROM agent
        WHERE workspace_id = $1 AND template_key = 'product-analyst'
        ORDER BY created_at LIMIT 1`,
      [workspaceId],
    );
    expect(before).toHaveLength(1);

    const created = await call("/api/squads/from-template", {
      method: "POST",
      body: { template_key: "bug-fix", runtime_id: runtimeId, name: `Bug Squad ${RUN_ID}` },
    });
    expect(created.status, `second squad should staff: ${created.text}`).toBe(201);

    const after = await sql<{ autonomy_level: string; instructions: string }>(
      `SELECT autonomy_level, instructions FROM agent WHERE id = $1`,
      [before[0].id],
    );
    expect(after[0].autonomy_level).toBe(before[0].autonomy_level);
    expect(after[0].instructions).toBe(before[0].instructions);
  });

  test("an Observer cannot staff a squad", async () => {
    const res = await call("/api/squads/from-template", {
      method: "POST",
      body: { template_key: "feature-delivery", runtime_id: runtimeId, name: `Denied ${RUN_ID}` },
      actor: observer,
    });
    // Without this gate, an agent refused by POST /api/squads could staff an
    // entire roster through this route instead.
    expect(res.status, `observer squad staffing must be refused: ${res.text}`).toBe(403);

    const rows = await sql(`SELECT id FROM squad WHERE workspace_id = $1 AND name = $2`, [
      workspaceId,
      `Denied ${RUN_ID}`,
    ]);
    expect(rows).toHaveLength(0);
  });

  test("staffing cannot pull in a role agent the caller may not invoke", async () => {
    // A plain member — not owner/admin, who bypass the predicate by design.
    const otherApi = new TestApiClient();
    await otherApi.login(OTHER_EMAIL, "E2E Autonomy Other");
    const others = await sql<{ id: string }>(`SELECT id FROM "user" WHERE email = $1`, [OTHER_EMAIL]);
    await sql(
      `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')
       ON CONFLICT (workspace_id, user_id) DO UPDATE SET role = 'member'`,
      [workspaceId, others[0].id],
    );

    // A runtime this member is allowed to build on, so the request gets past the
    // private-runtime check and actually reaches the wiring predicate under test.
    const shared = await sql<{ id: string }>(
      `INSERT INTO agent_runtime
         (workspace_id, daemon_id, name, runtime_mode, provider, status,
          device_info, metadata, last_seen_at, owner_id, visibility)
       VALUES ($1, gen_random_uuid(), $2, 'local', 'claude', 'online',
               'e2e shared runtime', '{"capabilities":["rpc-v1"]}'::jsonb, now(), $3, 'public')
       RETURNING id`,
      [workspaceId, `E2E Shared Runtime ${RUN_ID}`, ownerUserId],
    );

    // The lookup that finds a role's existing agent keys only on workspace +
    // template_key, so it happily finds the private ones owned by someone else.
    // Without the wiring predicate the caller would end up controlling
    // squad.instructions — appended verbatim to the leader's briefing — for an
    // agent they cannot even invoke.
    const res = await call("/api/squads/from-template", {
      method: "POST",
      body: {
        template_key: "feature-delivery",
        runtime_id: shared[0].id,
        name: `Cross Member ${RUN_ID}`,
      },
      token: otherApi.getToken()!,
    });
    expect(res.status, `cross-member staffing must be refused: ${res.text}`).toBe(409);
    expect(res.text).toContain("staff this squad");

    // The transaction rolled back, so nothing is left half-built.
    const squads = await sql(`SELECT id FROM squad WHERE workspace_id = $1 AND name = $2`, [
      workspaceId,
      `Cross Member ${RUN_ID}`,
    ]);
    expect(squads, "the refused squad must not exist").toHaveLength(0);
  });
});

test.describe("coordination gates on squads and automation", () => {
  let squadId: string;

  test.beforeAll(async () => {
    const created = await call("/api/squads/from-template", {
      method: "POST",
      body: { template_key: "bug-fix", runtime_id: runtimeId, name: `Gated Squad ${RUN_ID}` },
    });
    expect(created.status, `squad for gate tests should staff: ${created.text}`).toBe(201);
    squadId = (created.json as { squad: { id: string } }).squad.id;
  });

  test("a Contributor cannot edit a squad or its membership", async () => {
    // squad.instructions is the leader's routing policy and reaches every
    // member's next turn, so editing it is a Coordinator decision.
    const update = await call(`/api/squads/${squadId}`, {
      method: "PUT",
      body: { instructions: "route everything to me" },
      actor: contributor,
    });
    expect(update.status, `contributor squad edit must be refused: ${update.text}`).toBe(403);

    const add = await call(`/api/squads/${squadId}/members`, {
      method: "POST",
      body: { member_type: "agent", member_id: observer.agentId },
      actor: contributor,
    });
    expect(add.status, `contributor membership change must be refused: ${add.text}`).toBe(403);

    const archive = await call(`/api/squads/${squadId}`, { method: "DELETE", actor: contributor });
    expect(archive.status, `contributor squad archive must be refused: ${archive.text}`).toBe(403);

    const rows = await sql<{ instructions: string; archived_at: string | null }>(
      `SELECT instructions, archived_at FROM squad WHERE id = $1`,
      [squadId],
    );
    expect(rows[0].instructions).not.toContain("route everything to me");
    expect(rows[0].archived_at).toBeNull();
  });

  test("a Contributor cannot create or delete standing automation", async () => {
    // A fully valid body, so a 400 cannot be mistaken for the gate firing.
    const body = {
      title: `Gated Autopilot ${RUN_ID}`,
      assignee_type: "agent",
      assignee_id: contributor.agentId,
      execution_mode: "create_issue",
    };
    const created = await call("/api/autopilots", { method: "POST", body, actor: contributor });
    expect(created.status, `contributor autopilot create must be refused: ${created.text}`).toBe(403);

    // The same body from a person is accepted, proving the refusal was the
    // autonomy gate and not a malformed request.
    const byHuman = await call("/api/autopilots", { method: "POST", body });
    expect(byHuman.status, `owner autopilot create should pass: ${byHuman.text}`).toBe(201);
    const autopilotId = (byHuman.json as { id: string }).id;

    const deleted = await call(`/api/autopilots/${autopilotId}`, {
      method: "DELETE",
      actor: contributor,
    });
    expect(deleted.status, `contributor autopilot delete must be refused: ${deleted.text}`).toBe(403);
    const surviving = await sql(`SELECT id FROM autopilot WHERE id = $1`, [autopilotId]);
    expect(surviving, "the autopilot must survive a refused delete").toHaveLength(1);

    const refused = await sql(`SELECT id FROM autopilot WHERE workspace_id = $1 AND title = $2`, [
      workspaceId,
      body.title,
    ]);
    // Exactly one: the human's. The agent's attempt created nothing.
    expect(refused).toHaveLength(1);
  });
});
