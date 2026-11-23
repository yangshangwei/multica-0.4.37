import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import test from "node:test";
import pg from "pg";
import { buildFixture, fixtureId, FIXTURE_KEY, optionsFrom, PRIORITIES, runFixture, STATUSES } from "./seed-rich-test-data.mjs";

const context = {
  workspace: { id: "a136610a-3a67-4e93-8a5e-c12f3ff28a36", slug: "fixture-test", issue_prefix: "SEED" },
  ownerId: "695c1c89-111f-44ec-a9d5-7a006324740a",
  memberId: "9f4b905a-4b3c-4a49-8987-a3b3ef7614b9",
  now: new Date("2026-10-12T04:00:00Z"), triageEnabled: true,
};

test("CLI requires an explicit local database and cannot redirect through URL options", () => {
  assert.deepEqual(optionsFrom(["--help"], {}), { help: true });
  assert.throws(() => optionsFrom([], {}), /DATABASE_URL is required/);
  assert.throws(() => optionsFrom([], { DATABASE_URL: "postgres://db.example.test/multica" }), /loopback/);
  assert.throws(() => optionsFrom([], { DATABASE_URL: "postgres://localhost/multica?host=db.example.test" }), /override/);
  assert.throws(() => optionsFrom(["--workspace"], { DATABASE_URL: "postgres://localhost/multica" }));
  assert.throws(() => optionsFrom(["--unknown"], { DATABASE_URL: "postgres://localhost/multica" }));
  const options = optionsFrom(["--workspace", "qa-workspace", "--dry-run", "--reset"], { DATABASE_URL: "postgres://localhost/multica" });
  assert.equal(options.workspace, "qa-workspace"); assert.equal(options.dryRun, true); assert.equal(options.reset, true);
});

test("fixture identity is stable across dates and distinct across workspaces", () => {
  const first = buildFixture(context);
  const later = buildFixture({ ...context, now: new Date("2026-12-01T04:00:00Z") });
  assert.deepEqual(first.rows.map((row) => row.data.id), later.rows.map((row) => row.data.id));
  assert.notEqual(first.agentId, fixtureId(randomUUID(), "agent/reviewer"));
  assert.equal(new Set(first.rows.map((row) => row.key)).size, first.rows.length);
});

test("planned iteration facts match its nonterminal scope and never forge closed history", () => {
  const { rows, iterationId } = buildFixture(context);
  const issues = rows.filter((row) => row.table === "issue");
  assert.deepEqual(new Set(issues.map((row) => row.data.status)), new Set(STATUSES));
  assert.deepEqual(new Set(issues.map((row) => row.data.priority)), new Set(PRIORITIES));
  const scope = issues.filter((row) => row.data.current_iteration_id === iterationId);
  const events = rows.filter((row) => row.table === "iteration_event");
  assert.equal(events.length, scope.length);
  assert.equal(rows.find((row) => row.table === "iteration").data.scope_revision, scope.length + 1);
  for (const [index, event] of events.entries()) {
    const issue = scope.find((row) => row.data.id === event.data.issue_id).data;
    const facts = JSON.parse(event.data.after_facts);
    assert.equal(event.data.sequence, index + 1);
    assert.equal(event.data.kind, "planned_activity"); assert.equal(event.data.before_facts, "null");
    assert.equal(facts.status_key, issue.status); assert.equal(facts.assignee_id, issue.assignee_id);
    assert.equal(facts.target_iteration_id, iterationId);
    assert.ok(!["done", "cancelled"].includes(issue.status));
  }
  assert.ok(!rows.some((row) => row.table === "iteration_snapshot"));
  assert.ok(!buildFixture({ ...context, iterationEnabled: false }).rows.some((row) => row.table === "iteration_participation"));
});

test("execution samples cannot run or retry and use the actual conversation contract", () => {
  const fixture = buildFixture(context);
  const runtime = fixture.rows.find((row) => row.table === "agent_runtime").data;
  assert.equal(runtime.status, "offline"); assert.equal(runtime.daemon_id, null);
  const tasks = fixture.rows.filter((row) => row.table === "agent_task_queue");
  assert.deepEqual(new Set(tasks.map((row) => row.data.status)), new Set(["queued", "completed", "failed", "cancelled"]));
  for (const task of tasks) {
    assert.equal(task.data.runtime_id, runtime.id); assert.equal(task.data.attempt, task.data.max_attempts);
    assert.equal(task.data.accountable_user_id, task.data.originator_user_id);
    if (task.data.issue_id) assert.equal(fixture.rows.find((row) => row.table === "issue" && row.data.id === task.data.issue_id).data.assignee_id, fixture.agentId);
    assert.equal(Boolean(task.data.completed_at), task.data.status !== "queued");
  }
  for (const starter of JSON.parse(fixture.rows.find((row) => row.table === "agent").data.conversation_starters)) {
    assert.equal(typeof starter.label, "string"); assert.equal(typeof starter.prompt, "string");
  }
  assert.ok(fixture.rows.filter((row) => row.table === "comment").every((row) => row.data.dispatch_eligible === false));
  const inboxTypes = new Set(["new_comment", "mentioned", "issue_assigned", "status_changed", "task_failed"]);
  assert.ok(fixture.rows.filter((row) => row.table === "inbox_item").every((row) => inboxTypes.has(row.data.type)));
  const statusDetails = JSON.parse(fixture.rows.find((row) => row.table === "inbox_item" && row.data.type === "status_changed").data.details);
  assert.equal(statusDetails.from, "in_review"); assert.equal(statusDetails.to, "done");
});

// Point this only at a disposable database with the real application schema.
// Normal Node tests do not discover .env or connect to a developer's live data.
const integrationURL = process.env.SEED_TEST_DATABASE_URL;
test("PostgreSQL fixture isolation, idempotency and reset guards", { skip: !integrationURL }, async (t) => {
  const options = optionsFrom([], { DATABASE_URL: integrationURL });
  const databaseName = new URL(integrationURL).pathname.slice(1);
  assert.match(databaseName, /^multica_(rich_seed_verify_|seed_test_)/, "Integration tests require a disposable fixture database");
  const client = new pg.Client({ connectionString: options.connectionString });
  await client.connect();
  const workspaceId = randomUUID();
  const slug = `seed-regression-${workspaceId.slice(0, 8)}`;
  const sentinelId = randomUUID();
  const activeId = randomUUID();
  const run = (patch = {}) => runFixture(client, { workspace: slug, dryRun: false, reset: false, ...patch });
  const amount = async (sql, values = []) => Number((await client.query(sql, values)).rows[0].amount);
  try {
    const ownerId = (await client.query('SELECT id FROM "user" ORDER BY id LIMIT 1')).rows[0]?.id;
    assert.ok(ownerId, "The scratch database needs an existing fixture owner");
    await client.query("INSERT INTO workspace(id,name,slug,issue_prefix,issue_counter,planning_timezone) VALUES($1,'Seed safety regression',$2,'SEED',17,'Asia/Shanghai')", [workspaceId, slug]);
    await client.query("INSERT INTO member(workspace_id,user_id,role) VALUES($1,$2,'owner')", [workspaceId, ownerId]);
    await client.query("INSERT INTO workspace_iteration_settings(workspace_id,enabled) VALUES($1,true)", [workspaceId]);
    await client.query("INSERT INTO workspace_triage_settings(workspace_id,enabled) VALUES($1,true)", [workspaceId]);
    await client.query("INSERT INTO issue(id,workspace_id,title,status,creator_type,creator_id,number) VALUES($1,$2,'Unrelated sentinel','todo','member',$3,1)", [sentinelId, workspaceId, ownerId]);
    await client.query("INSERT INTO iteration(id,workspace_id,name,timezone,start_date,end_date,created_by,status,started_at,started_by) VALUES($1,$2,'Existing active','Asia/Shanghai',current_date,current_date+14,$3,'active',now(),$3)", [activeId, workspaceId, ownerId]);
    const activeBefore = (await client.query("SELECT * FROM iteration WHERE id=$1", [activeId])).rows[0];

    await t.test("dry-run is read only and leaves counters and all baseline records unchanged", async () => {
      const result = await run({ dryRun: true });
      assert.equal(result.inserted.issue, 44);
      assert.equal(await amount("SELECT issue_counter AS amount FROM workspace WHERE id=$1", [workspaceId]), 17);
      assert.equal(await amount("SELECT count(*) AS amount FROM issue WHERE workspace_id=$1", [workspaceId]), 1);
    });
    await t.test("missing triage settings keep the server's default-enabled intake scenarios", async () => {
      await client.query("DELETE FROM workspace_triage_settings WHERE workspace_id=$1", [workspaceId]);
      try {
        const result = await run({ dryRun: true });
        assert.equal(result.inserted.issue_triage, 2);
        assert.equal(await amount("SELECT count(*) AS amount FROM workspace_triage_settings WHERE workspace_id=$1", [workspaceId]), 0);
      } finally {
        await client.query("INSERT INTO workspace_triage_settings(workspace_id,enabled) VALUES($1,true)", [workspaceId]);
      }
    });
    await t.test("a failure after initial inserts rolls back every fixture row and number reservation", async () => {
      const conflictingAgentId = randomUUID();
      await client.query("INSERT INTO agent(id,workspace_id,name,runtime_mode,owner_id) VALUES($1,$2,'桌面场景 · 回归助手','local',$3)", [conflictingAgentId, workspaceId, ownerId]);
      try { await assert.rejects(run(), (error) => error.code === "23505"); }
      finally { await client.query("DELETE FROM agent WHERE id=$1", [conflictingAgentId]); }
      assert.equal(await amount("SELECT issue_counter AS amount FROM workspace WHERE id=$1", [workspaceId]), 17);
      assert.equal(await amount("SELECT count(*) AS amount FROM issue WHERE workspace_id=$1", [workspaceId]), 1);
      for (const table of ["project", "issue_label", "agent_runtime"]) {
        assert.equal(await amount(`SELECT count(*) AS amount FROM ${table} WHERE workspace_id=$1`, [workspaceId]), 0, table);
      }
    });
    const first = await run();
    const issueId = first.ids.issue_with_replies;
    const commentId = fixtureId(workspaceId, "comment/8");
    const deletedIssueId = fixtureId(workspaceId, "issue/07");

    await t.test("repeat seed preserves edits, read state, counters and intentional deletions", async () => {
      const counter = await amount("SELECT issue_counter AS amount FROM workspace WHERE id=$1", [workspaceId]);
      await client.query("UPDATE issue SET title='Human edited title',status='todo' WHERE id=$1", [issueId]);
      await client.query("UPDATE inbox_item SET read=true WHERE id=$1", [fixtureId(workspaceId, "inbox/1")]);
      await client.query("DELETE FROM comment WHERE id=$1", [commentId]);
      await client.query("DELETE FROM issue WHERE id=$1", [deletedIssueId]);
      const second = await run();
      assert.deepEqual(second.inserted, {});
      assert.equal((await client.query("SELECT title FROM issue WHERE id=$1", [issueId])).rows[0].title, "Human edited title");
      assert.equal((await client.query("SELECT read FROM inbox_item WHERE id=$1", [fixtureId(workspaceId, "inbox/1")])).rows[0].read, true);
      assert.equal(await amount("SELECT count(*) AS amount FROM comment WHERE id=$1", [commentId]), 0);
      assert.equal(await amount("SELECT count(*) AS amount FROM issue WHERE id=$1", [deletedIssueId]), 0);
      assert.equal(await amount("SELECT issue_counter AS amount FROM workspace WHERE id=$1", [workspaceId]), counter);
      assert.deepEqual((await client.query("SELECT * FROM iteration WHERE id=$1", [activeId])).rows[0], activeBefore);
    });
    await t.test("a non-fixture child causes reset to roll back before any deletion", async () => {
      const externalId = randomUUID();
      await client.query("INSERT INTO issue(id,workspace_id,title,creator_type,creator_id,number,parent_issue_id) VALUES($1,$2,'Other user child','member',$3,20000,$4)", [externalId, workspaceId, ownerId, issueId]);
      try { await assert.rejects(run({ reset: true }), /non-fixture.*reference/); }
      finally { await client.query("DELETE FROM issue WHERE id=$1", [externalId]); }
      assert.equal(await amount("SELECT count(*) AS amount FROM issue WHERE id=$1", [issueId]), 1);
    });
    await t.test("soft references nested in JSON prevent orphaning external records", async () => {
      const externalId = randomUUID();
      await client.query("INSERT INTO activity_log(id,workspace_id,issue_id,action,details) VALUES($1,$2,$3,'external note',$4::jsonb)", [externalId, workspaceId, sentinelId, JSON.stringify({ nested: { referenced_issue: issueId } })]);
      try { await assert.rejects(run({ reset: true, dryRun: true }), /activity_log/); }
      finally { await client.query("DELETE FROM activity_log WHERE id=$1", [externalId]); }
    });
    await t.test("missing ownership marker and connected runtime are refused", async () => {
      const metadata = (await client.query("SELECT metadata FROM issue WHERE id=$1", [issueId])).rows[0].metadata;
      await client.query("UPDATE issue SET metadata='{}'::jsonb WHERE id=$1", [issueId]);
      try { await assert.rejects(run(), /marker missing/); }
      finally { await client.query("UPDATE issue SET metadata=$2::jsonb WHERE id=$1", [issueId, JSON.stringify(metadata)]); }
      await client.query("UPDATE agent_runtime SET status='online' WHERE id=$1", [first.ids.offline_runtime]);
      try { await assert.rejects(run(), /potentially executable/); }
      finally { await client.query("UPDATE agent_runtime SET status='offline' WHERE id=$1", [first.ids.offline_runtime]); }
    });
    await t.test("disabled features do not strand earlier fixture rows or change user settings", async () => {
      await client.query("UPDATE workspace_iteration_settings SET enabled=false WHERE workspace_id=$1", [workspaceId]);
      await client.query("UPDATE workspace_triage_settings SET enabled=false WHERE workspace_id=$1", [workspaceId]);
      const preview = await run({ reset: true, dryRun: true });
      assert.equal(preview.deleted.issue_triage, 2); assert.equal(preview.deleted.iteration_event, 18);
      await run({ reset: true });
      assert.equal(await amount("SELECT count(*) AS amount FROM issue WHERE workspace_id=$1", [workspaceId]), 1);
      assert.equal(await amount("SELECT count(*) AS amount FROM issue WHERE metadata->>'fixture_key'=$1 AND workspace_id=$2", [FIXTURE_KEY, workspaceId]), 0);
      for (const table of ["issue_triage", "iteration_participation", "iteration_event", "agent", "agent_runtime", "project", "chat_session", "comment", "inbox_item"]) {
        assert.equal(await amount(`SELECT count(*) AS amount FROM ${table} WHERE workspace_id=$1`, [workspaceId]), 0, table);
      }
      assert.equal((await client.query("SELECT enabled FROM workspace_iteration_settings WHERE workspace_id=$1", [workspaceId])).rows[0].enabled, false);
      assert.equal((await client.query("SELECT enabled FROM workspace_triage_settings WHERE workspace_id=$1", [workspaceId])).rows[0].enabled, false);
      assert.deepEqual((await client.query("SELECT * FROM iteration WHERE id=$1", [activeId])).rows[0], activeBefore);
    });
    await client.query("DELETE FROM issue WHERE id=$1", [sentinelId]);
    await client.query("DELETE FROM iteration WHERE id=$1", [activeId]);
    await client.query("DELETE FROM workspace_triage_settings WHERE workspace_id=$1", [workspaceId]);
    await client.query("DELETE FROM workspace_iteration_settings WHERE workspace_id=$1", [workspaceId]);
    await client.query("DELETE FROM member WHERE workspace_id=$1", [workspaceId]);
    await client.query("DELETE FROM workspace WHERE id=$1", [workspaceId]);
  } finally { await client.end(); }
});
