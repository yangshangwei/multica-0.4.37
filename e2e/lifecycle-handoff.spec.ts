import { randomUUID } from "node:crypto";
import { test as base, expect } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;

interface LifecycleFixture {
  api: TestApiClient;
  workspace: { id: string; slug: string };
  users: string[];
}

interface Handoff {
  decision: string;
  follow_up_issue_id: string;
  follow_up_created?: boolean;
  follow_up_reused?: boolean;
  queued_task_id?: string;
  audit_comment_id: string;
}

interface Issue {
  id: string;
  parent_issue_id: string | null;
  assignee_id: string | null;
  metadata: Record<string, unknown>;
}

interface TimelineEntry {
  type: string;
  id: string;
  content?: string;
}

async function withDatabase<T>(run: (client: pg.Client) => Promise<T>): Promise<T> {
  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) throw new Error("Lifecycle E2E fixtures require an explicit DATABASE_URL");
  const client = new pg.Client(databaseUrl);
  await client.connect();
  try {
    return await run(client);
  } finally {
    await client.end();
  }
}

const test = base.extend<{ lifecycle: LifecycleFixture }>({
  lifecycle: async ({ page }, use) => {
    const slug = `e2e-lifecycle-${randomUUID().slice(0, 12)}`;
    const api = new TestApiClient();
    const users = [`${slug}@multica.ai`];
    await api.login(users[0], "Lifecycle owner");
    const workspace = await api.ensureWorkspace("Lifecycle handoff E2E", slug);
    expect(workspace.slug, "cleanup must only own this test's workspace").toBe(slug);
    try {
      await api.markUserOnboarded();
      const token = api.getToken();
      if (!token) throw new Error("Lifecycle fixture login did not return a token");
      await page.addInitScript((value) => {
        localStorage.setItem("multica_token", value);
        localStorage.setItem("multica:chat:isOpen", "false");
      }, token);
      await use({ api, workspace, users });
    } finally {
      // Workspace deletion owns all dependent tasks, comments, agents and runtime
      // rows, including the follow-up created by the lifecycle endpoint itself.
      await api.deleteFeatureWorkspace(workspace.id);
      expect((await api.getWorkspaces()).some((item) => item.id === workspace.id)).toBe(false);
      await withDatabase((client) => client.query('DELETE FROM "user" WHERE email = ANY($1::text[])', [users]));
    }
  },
});

test.use({ screenshot: "only-on-failure", trace: "retain-on-failure" });

function repairEvidence() {
  return {
    kind: "rca", route: "bug-fix", cause_state: "known",
    reason: "The parser regression reproduces the failure", conclusion: "confirmed",
    evidence: ["Lifecycle E2E regression evidence"],
    diagnosis_ref: "diagnosis#parser", regression_test: "TestParserRegression",
  };
}

async function createPrivateAgent(api: TestApiClient) {
  // This registers only a runtime row: no daemon or installed agent CLI runs.
  const runtime = await api.seedProjectRuntime();
  return api.requestJSON<{ id: string }>("/api/agents", {
    method: "POST",
    body: { name: "Lifecycle private repair agent", runtime_id: runtime.id, permission_mode: "private" },
  });
}

async function workspaceState(workspaceId: string) {
  return withDatabase(async (client) => {
    const result = await client.query(
      `SELECT
         (SELECT COALESCE(jsonb_agg(to_jsonb(i) ORDER BY i.id), '[]'::jsonb)
            FROM issue i WHERE i.workspace_id = $1) AS issues,
         (SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.id), '[]'::jsonb)
            FROM comment c WHERE c.workspace_id = $1) AS comments,
         (SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id), '[]'::jsonb)
            FROM agent_task_queue t JOIN agent a ON a.id = t.agent_id
            WHERE a.workspace_id = $1) AS tasks`,
      [workspaceId],
    );
    return result.rows[0];
  });
}

test("creates and reuses an RCA repair with a visible issue audit trail", async ({ lifecycle, page }, testInfo) => {
  const { api, workspace } = lifecycle;
  const agent = await createPrivateAgent(api);
  const source: Issue = await api.createIssue("Lifecycle RCA source");
  const path = `/api/issues/${source.id}/lifecycle-handoffs`;
  const body = {
    ...repairEvidence(), follow_up_title: "Repair the lifecycle parser regression",
    assignee_type: "agent", assignee_id: agent.id,
  };
  const created = await api.requestJSON<Handoff>(path, { method: "POST", body });
  expect(created).toMatchObject({ decision: "direct-repair", follow_up_created: true });
  expect(created.queued_task_id).toBeTruthy();

  const byTitle = await api.requestJSON<Handoff>(path, { method: "POST", body });
  const byId = await api.requestJSON<Handoff>(path, {
    method: "POST", body: { ...repairEvidence(), follow_up_issue_id: created.follow_up_issue_id },
  });
  for (const result of [byTitle, byId]) {
    expect(result).toMatchObject({ follow_up_issue_id: created.follow_up_issue_id, follow_up_reused: true });
    expect(result.follow_up_created).not.toBe(true);
    expect(result.audit_comment_id).toBeTruthy();
  }
  expect(await api.countIssueDispatches(created.follow_up_issue_id)).toBe(1);
  const repair = await api.requestJSON<Issue>(`/api/issues/${created.follow_up_issue_id}`);
  expect(repair).toMatchObject({
    parent_issue_id: source.id, assignee_id: agent.id,
    metadata: {
      lifecycle_rca_conclusion: "confirmed", lifecycle_diagnosis_ref: "diagnosis#parser",
      lifecycle_regression_test: "TestParserRegression",
    },
  });
  const refreshedSource = await api.requestJSON<Issue>(`/api/issues/${source.id}`);
  for (const issue of [refreshedSource, repair]) {
    for (const value of Object.values(issue.metadata)) {
      expect(["string", "number", "boolean"]).toContain(typeof value);
    }
    const metadata = await api.requestJSON<{ metadata: Record<string, unknown> }>(`/api/issues/${issue.id}/metadata`);
    expect(metadata.metadata).toEqual(issue.metadata);
  }

  const timeline = await api.requestJSON<TimelineEntry[]>(`/api/issues/${source.id}/timeline`);
  const auditEntries = timeline.filter((entry) => entry.type === "comment");
  expect(auditEntries).toEqual(expect.arrayContaining([
    expect.objectContaining({
      id: byId.audit_comment_id,
      content: expect.stringContaining("lifecycle-handoff kind=rca decision=direct-repair"),
    }),
  ]));

  await page.goto(`/${workspace.slug}/issues/${source.id}`, { waitUntil: "domcontentloaded" });
  const audit = page.locator(`#comment-${byId.audit_comment_id}`);
  await expect(audit).toBeVisible({ timeout: 15_000 });
  await expect(audit).toContainText("lifecycle-handoff kind=rca decision=direct-repair");
  await expect(audit).toContainText(created.follow_up_issue_id);
  await expect(audit).toContainText("Lifecycle E2E regression evidence");
  await testInfo.attach("lifecycle-audit-trail", { body: await page.screenshot({ fullPage: true }), contentType: "image/png" });
});

for (const lookup of ["explicit child", "duplicate title"] as const) {
  test(`refuses unauthorized ${lookup} reuse without metadata, audit, or queue writes`, async ({ lifecycle, request }) => {
    const { api, workspace, users } = lifecycle;
    const agent = await createPrivateAgent(api);
    const source: Issue = await api.createIssue("Lifecycle permission source");
    const childTitle = "Lifecycle private repair child";
    const child: Issue = await api.createIssue(childTitle, {
      parent_issue_id: source.id, assignee_type: "agent", assignee_id: agent.id, status: "backlog",
    });
    expect(child.id).toBeTruthy();
    expect(await api.countIssueDispatches(child.id), "backlog setup must not enqueue any task").toBe(0);

    const member = new TestApiClient();
    const memberEmail = `${workspace.slug}-member@multica.ai`;
    users.push(memberEmail);
    await member.login(memberEmail, "Lifecycle member without invoke permission");
    const user = await member.requestJSON<{ id: string }>("/api/me");
    // Workspace membership is ambient setup; the forbidden handoff uses the
    // member's real login credential and the production HTTP auth middleware.
    await withDatabase((client) => client.query(
      "INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')", [workspace.id, user.id],
    ));
    const before = await workspaceState(workspace.id);
    const response = await request.post(`${API_BASE}/api/issues/${source.id}/lifecycle-handoffs`, {
      headers: { Authorization: `Bearer ${member.getToken()}`, "X-Workspace-ID": workspace.id },
      data: {
        ...repairEvidence(),
        ...(lookup === "explicit child" ? { follow_up_issue_id: child.id } : {
          follow_up_title: childTitle,
          // A permitted requested assignee cannot authorize the private agent
          // already assigned to the duplicate returned by the issue service.
          assignee_type: "member", assignee_id: user.id,
        }),
      },
    });
    expect(response.status(), await response.text()).toBe(403);
    expect(await response.json()).toEqual({ error: "you do not have permission to assign work to this agent" });
    expect(await workspaceState(workspace.id)).toEqual(before);
    expect(await api.countIssueDispatches(child.id)).toBe(0);
  });
}
