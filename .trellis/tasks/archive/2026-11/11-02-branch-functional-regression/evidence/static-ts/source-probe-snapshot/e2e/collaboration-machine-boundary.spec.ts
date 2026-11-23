import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { API_BASE, call, callJSON, createActor, createAgent, SimulatedDaemon, sql, type Issue } from "./fixtures/core-journeys";

// R53/R56 (AC84, AC87) at the HTTP boundary. An ordinary agent's real `mat_`
// task token in the source workspace sees a collaboration issue only as a
// minimal reference, and every write it tries is refused. A human participant's
// legacy comment sub-issue preview/create is refused for a collaboration anchor
// and still works for an ordinary one. Legacy lane only: the shared fixtures log
// in by email code and SimulatedDaemon claims with a member session, neither of
// which password mode offers. (Password-mode REST auth does accept `mat_` via
// PasswordSessionForTask, password_auth.go:50; only daemon/realtime auth refuses it.)
type CollaborationIssue = { id: string; lifecycle_epoch: string; write_context: { participant_epoch: string; account_epoch: string } };
type Receipt = { code: string; status: number; comment_id: string; targets: unknown[] };
type ErrorBody = { code?: string; error?: string };

const apiUrl = process.env.COLLABORATION_E2E_API_URL;
const REFERENCE_KEYS = ["can_open", "collaboration_reference", "id", "identifier", "number", "title", "workspace_id"];

test("C13 an ordinary agent's task token sees a collaboration issue only as a reference and cannot write it", async () => {
  test.setTimeout(120_000);
  test.skip(!apiUrl || !process.env.DATABASE_URL || process.env.E2E_PASSWORD_AUTH === "1",
    "Requires the legacy-auth collaboration lane (email-code fixtures and a member-session daemon)");
  // The machine calls go to API_BASE; the lane gate names COLLABORATION_E2E_API_URL. They must be one server.
  expect(new URL(API_BASE).origin).toBe(new URL(apiUrl!).origin);
  expect(await (await fetch(`${API_BASE}/api/config`)).json()).toMatchObject({ auth_mode: "legacy" });

  // The workspace keeps its collaboration record, so it is not deleted at the
  // end: the server refuses that by design (workspace.go:1137, R37).
  const owner = await createActor("boundary");
  expect(await callJSON(owner, "/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const tag = randomUUID().slice(0, 8);
  const secret = `PRIVATE_COLLABORATION_BODY_${tag}`;
  const shared = await owner.api.requestJSON<CollaborationIssue>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: owner.workspace.id, handler_user_id: owner.userId,
      title: `Boundary shared ${tag}`, description: secret, attachment_ids: [],
    },
  });
  const sharedComment = await owner.api.requestJSON<Receipt>(`/api/collaboration/issues/${shared.id}/comments`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      content: `Shared comment ${secret}`, expected_lifecycle_epoch: shared.lifecycle_epoch,
      expected_participant_epoch: shared.write_context.participant_epoch,
      expected_account_epoch: shared.write_context.account_epoch, target_versions: [],
    },
  });
  expect(sharedComment).toMatchObject({ code: "comment_created", status: 201, targets: [] });
  const sharedRow = await callJSON<Issue & { number: number }>(owner, `/api/issues/${shared.id}`);
  // An ordinary, unassigned issue and comment: the positive control for every check.
  const ordinary = await owner.api.createIssue(`Boundary ordinary ${tag}`, { status: "todo", description: `Ordinary body ${tag}` }) as Issue;
  const ordinaryComment = await callJSON<{ id: string }>(owner, `/api/issues/${ordinary.id}/comments`, "POST", { content: `Ordinary comment ${tag}` });

  // A real task token: an ordinary agent claims ordinary work and is running it.
  const daemon = new SimulatedDaemon(owner);
  await daemon.register();
  const agent = await createAgent(owner, daemon.runtimeId);
  const work = await owner.api.createIssue(`Boundary agent work ${tag}`, { status: "todo", assignee_type: "agent", assignee_id: agent.id }) as Issue;
  await expect.poll(async () => (await sql("SELECT id FROM agent_task_queue WHERE issue_id = $1", [work.id])).length).toBe(1);
  const claimed = await daemon.claim();
  await daemon.start(claimed!.id);
  const token = String(claimed!.auth_token ?? "");
  expect(token.startsWith("mat_")).toBe(true);
  const machine = (path: string, method = "GET", body?: unknown) => fetch(`${API_BASE}${path}`, {
    method, headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const refusal = async (response: Response, status: number, code: string) => {
    const text = await response.text();
    expect(response.status, text).toBe(status);
    expect(JSON.parse(text) as ErrorBody).toMatchObject({ code });
    expect(text).not.toContain(secret);
  };
  // Reads. Search and list return the collaboration issue as identity only
  // (collaborationReferenceResponse, collaboration_boundary.go:24-38; search
  // issue.go:1052-1055, list issue.go:1684-1687) and the ordinary issue in full.
  const reference = { id: shared.id, workspace_id: owner.workspace.id, number: sharedRow.number,
    identifier: sharedRow.identifier, title: `Boundary shared ${tag}`, collaboration_reference: true, can_open: false };
  for (const path of [`/api/issues/search?q=${tag}`, `/api/issues?q=${tag}`]) {
    const response = await machine(path);
    const text = await response.text();
    expect(response.status, text).toBe(200);
    expect(text).not.toContain(secret);
    const issues = (JSON.parse(text) as { issues: Array<Record<string, unknown>> }).issues;
    const sharedHit = issues.find((item) => item.id === shared.id);
    expect(sharedHit, path).toEqual(reference);
    expect(Object.keys(sharedHit!).sort()).toEqual(REFERENCE_KEYS);
    const ordinaryHit = issues.find((item) => item.id === ordinary.id);
    expect(ordinaryHit, path).toMatchObject({ title: ordinary.title, status: "todo" });
    expect(ordinaryHit!.collaboration_reference).toBeUndefined();
  }
  // The human owner's own search sees the full issue, so the shape above is the machine's view.
  const humanSearch = await callJSON<{ issues: Array<Record<string, unknown>> }>(owner, `/api/issues/search?q=${tag}`);
  const humanHit = humanSearch.issues.find((item) => item.id === shared.id);
  expect(humanHit).toMatchObject({ id: shared.id, description: secret, status: sharedRow.status });
  expect(humanHit!.collaboration_reference).toBeUndefined();

  // Detail, comments and the account route resolve no access for a non-human
  // principal: CapabilitiesFor grants a machine Reference only (access.go:79-86),
  // and ResolveIssueAccess returns not_found for it (collaboration.go:65-70).
  for (const path of [`/api/issues/${shared.id}`, `/api/issues/${sharedRow.identifier}`, `/api/issues/${shared.id}/comments`, `/api/collaboration/issues/${shared.id}`]) {
    await refusal(await machine(path), 404, "not_found");
  }
  const ordinaryRead = await machine(`/api/issues/${ordinary.id}`);
  expect(ordinaryRead.status).toBe(200);
  expect(await ordinaryRead.json()).toMatchObject({ id: ordinary.id, description: `Ordinary body ${tag}` });
  expect(await callJSON(owner, `/api/collaboration/issues/${shared.id}`)).toMatchObject({ id: shared.id });
  expect(sharedRow).toMatchObject({ id: shared.id, description: secret });

  // Writes. Every legacy write to a collaboration issue or its comments is
  // refused before it touches the row (writeUnsupportedCollaborationAction,
  // collaboration_boundary.go:57-98, called from handler.go:1157, comment.go:3236,
  // reaction.go:80 and handleLegacyCollaborationUpdate).
  const before = await sql<{ title: string; revision: string; comments: string; reactions: string }>(
    `SELECT i.title, i.revision::text,
            (SELECT count(*)::text FROM comment c WHERE c.issue_id = i.id) AS comments,
            (SELECT count(*)::text FROM comment_reaction r JOIN comment c ON c.id = r.comment_id WHERE c.issue_id = i.id) AS reactions
       FROM issue i WHERE i.id = $1`, [shared.id]);
  await refusal(await machine(`/api/issues/${shared.id}/comments`, "POST", { content: "Machine comment" }), 409, "unsupported_collaboration_action");
  await refusal(await machine(`/api/issues/${shared.id}`, "PUT", { title: "Machine rename" }), 409, "unsupported_collaboration_action");
  await refusal(await machine(`/api/issues/${shared.id}`, "DELETE"), 409, "unsupported_collaboration_action");
  await refusal(await machine(`/api/comments/${sharedComment.comment_id}`, "PUT", { content: "Machine edit" }), 409, "unsupported_collaboration_action");
  await refusal(await machine(`/api/comments/${sharedComment.comment_id}/reactions`, "POST", { emoji: "+1" }), 409, "unsupported_collaboration_action");
  expect(await sql(
    `SELECT i.title, i.revision::text,
            (SELECT count(*)::text FROM comment c WHERE c.issue_id = i.id) AS comments,
            (SELECT count(*)::text FROM comment_reaction r JOIN comment c ON c.id = r.comment_id WHERE c.issue_id = i.id) AS reactions
       FROM issue i WHERE i.id = $1`, [shared.id])).toEqual(before);
  expect((await sql<{ content: string }>("SELECT content FROM comment WHERE id = $1", [sharedComment.comment_id]))[0]!.content).toBe(`Shared comment ${secret}`);
  // The same token may still write ordinary work, so the refusals above are collaboration-specific.
  expect((await machine(`/api/issues/${ordinary.id}/comments`, "POST", { content: `Machine ordinary comment ${tag}` })).status).toBe(201);
  // Legacy comment sub-issue routes are human-only (router.go:2394-2395)...
  const machinePreview = await machine(`/api/comments/${ordinaryComment.id}/sub-issue-preview`);
  expect(machinePreview.status).toBe(403);
  expect(await machinePreview.json()).toEqual({ error: "this endpoint is only available to human actors" });
  // ...and for a human participant they refuse a collaboration anchor:
  // BuildSourceIssueSnapshot returns ErrSourceContextCollaboration (source_context.go:275-281),
  // written as unsupported_collaboration_action (handler source_context.go:847-851).
  await refusal(await call(owner, `/api/comments/${sharedComment.comment_id}/sub-issue-preview`), 409, "unsupported_collaboration_action");
  const ordinaryPreview = await callJSON<{ capture_token: string; anchor_comment_id: string }>(owner, `/api/comments/${ordinaryComment.id}/sub-issue-preview`);
  expect(ordinaryPreview).toMatchObject({ anchor_comment_id: ordinaryComment.id, capture_token: expect.stringMatching(/^sha256:[0-9a-f]{64}:/) });
  const counts = async () => (await sql<{ issues: string; contexts: string; quick: string }>(
    `SELECT (SELECT count(*)::text FROM issue WHERE workspace_id = $1) AS issues,
            (SELECT count(*)::text FROM issue_source_context WHERE workspace_id = $1) AS contexts,
            (SELECT count(*)::text FROM agent_task_queue WHERE context->>'type' = 'quick_create' AND context->>'workspace_id' = $1::text) AS quick`,
    [owner.workspace.id]))[0];
  const beforeCreate = await counts();
  // A well-formed capture token does not unlock a collaboration anchor in either mode.
  for (const body of [
    { mode: "manual", capture_token: ordinaryPreview.capture_token, issue: { title: `Copied from shared ${tag}` } },
    { mode: "agent", capture_token: ordinaryPreview.capture_token, quick_create: { agent_id: agent.id, prompt: `Copy the shared thread ${tag}` } },
  ]) {
    await refusal(await call(owner, `/api/comments/${sharedComment.comment_id}/sub-issues`, "POST", body), 409, "unsupported_collaboration_action");
  }
  expect(await counts()).toEqual(beforeCreate);
  expect(await sql("SELECT id FROM issue WHERE parent_issue_id = $1", [shared.id])).toEqual([]);
  // The same route still creates from an ordinary anchor.
  const created = await call(owner, `/api/comments/${ordinaryComment.id}/sub-issues`, "POST",
    { mode: "manual", capture_token: ordinaryPreview.capture_token, issue: { title: `Copied from ordinary ${tag}` } });
  expect(created.status, await created.clone().text()).toBe(201);
  const child = await created.json() as { id: string; parent_issue_id: string };
  expect(child.parent_issue_id).toBe(ordinary.id);
  expect(await sql("SELECT state, source_issue_id::text, anchor_comment_id::text FROM issue_source_context WHERE issue_id = $1", [child.id]))
    .toEqual([{ state: "attached", source_issue_id: ordinary.id, anchor_comment_id: ordinaryComment.id }]);

  // Finishing the run revokes its task token (daemon.go:4012).
  await daemon.complete(claimed!.id, `Boundary run finished ${tag}`);
  expect((await machine(`/api/issues/${ordinary.id}`)).status).toBe(401);
});
