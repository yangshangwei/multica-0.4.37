// @vitest-environment node
import { expect, it } from "vitest";
import { AdminCollaborationChangeSchema, matchesAdminCollaborationResult, parseAdminCollaborationCandidates,
  parseAdminCollaborationDetail, parseAdminCollaborationIssueList, parseAdminCollaborationResult } from "./collaboration-schema";

const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const account = (n: number) => ({ user_id: id(n), name: `Account ${n}`, avatar_url: "", disabled: true });
const issue = { issue_id: id(1), identifier: "SRC-1", source_workspace_id: id(2), source_name: "Source",
  status: "done", status_category: "done", collaboration_revision: "7", creator: account(3), handler: account(4), queue_state: "closed", unsettled_runs: 1 };
const detail = { ...issue, scope: id(9), actions: [{ action: "reopen", status_options_version: "a".repeat(64), default_status: "todo",
  options: [{ key: "todo", name: "Todo", category: "todo" }] }], successor_required: true };
const input = { action: "reopen" as const, expectedCollaborationRevision: "7", expectedStatus: "done", targetStatus: "todo",
  statusOptionsVersion: "a".repeat(64), reason: "Restore handling responsibility", successorUserId: id(5) };
const result = { operation: { id: id(8), organization_id: id(9), target_id: id(1), actor_id: id(6), target_kind: "collaboration_issue",
  kind: "collaboration.reopen", state: "applied", result_code: "reopened", confirmation: "not_required", reconciliation_state: "complete", version: 1 },
  result: { issue_id: id(1), request_id: id(7), action: "reopen", code: "reopened", collaboration_revision: "9", creator_user_id: id(3),
    previous_handler_user_id: id(4), handler_user_id: id(5), previous_status: "done", status: "todo", queue_state: "paused", control_version: "4", unsettled_runs: 1 } };

it("reads minimal double-disabled metadata without exposing additive content fields", () => {
  const parsed = parseAdminCollaborationDetail({ ...detail, title: "Private title", description: "Private body" });
  expect(parsed).toMatchObject({ issueId: id(1), creator: { userId: id(3), disabled: true }, handler: { userId: id(4), disabled: true },
    successorRequired: true, collaborationRevision: "7", queueState: "closed", unsettledRuns: 1 });
  expect(parsed).not.toHaveProperty("title");
  expect(parsed).not.toHaveProperty("description");
});

it.each(["scope", "creator", "handler", "actions", "successor_required", "collaboration_revision", "queue_state"])("does not invent missing %s authority", (field) => {
  const raw: Record<string, unknown> = { ...detail };
  delete raw[field];
  expect(parseAdminCollaborationDetail(raw)).toBeNull();
});

it("refuses incompatible lifecycle, successor and status-option facts", () => {
  for (const raw of [{ ...detail, creator: { ...detail.creator, disabled: false } }, { ...detail, successor_required: false },
    { ...detail, handler: { ...detail.handler, disabled: false } }, { ...detail, queue_state: "paused" },
    { ...detail, actions: [detail.actions[0], detail.actions[0]] },
    { ...detail, actions: [{ ...detail.actions[0], action: "complete" }] }]) expect(parseAdminCollaborationDetail(raw)).toBeNull();
  expect(parseAdminCollaborationDetail({ ...detail, handler: { ...detail.handler, disabled: false }, successor_required: false })).not.toBeNull();
  expect(parseAdminCollaborationDetail({ ...detail, actions: [] })).not.toBeNull();
});

it("validates page boundaries before metadata or candidates enter a scoped cache", () => {
  expect(parseAdminCollaborationIssueList({ scope: id(9), items: [issue], next_cursor: issue.issue_id })?.nextCursor).toBe(id(1));
  expect(parseAdminCollaborationIssueList({ scope: id(9), items: [issue, issue], next_cursor: null })).toBeNull();
  expect(parseAdminCollaborationIssueList({ scope: id(9), items: [issue], next_cursor: id(20) })).toBeNull();
  const candidate = { user_id: id(5), name: "Successor", avatar_url: "", external: true };
  const page = { scope: id(9), issue_id: id(1), collaboration_revision: "7", items: [candidate], next_cursor: null };
  expect(parseAdminCollaborationCandidates(page)?.items[0]).toEqual({ userId: id(5), name: "Successor", avatarUrl: "", external: true });
  expect(parseAdminCollaborationCandidates({ ...page, items: [candidate, candidate] })).toBeNull();
  expect(parseAdminCollaborationCandidates({ ...page, items: [], next_cursor: id(5) })).toBeNull();
});

it("keeps versions exact and refuses implicit successor changes or extra authority", () => {
  expect(AdminCollaborationChangeSchema.parse(input)).toEqual(input);
  for (const raw of [{ ...input, action: "complete" }, { ...input, successorUserId: null }, { ...input, actorUserId: id(6) },
    { ...input, reason: " " }, { ...input, expectedCollaborationRevision: 7 }, { ...input, expectedCollaborationRevision: "9223372036854775808" }]) {
    expect(AdminCollaborationChangeSchema.safeParse(raw).success).toBe(false);
  }
  expect(AdminCollaborationChangeSchema.safeParse({ ...input, expectedCollaborationRevision: "9223372036854775807" }).success).toBe(true);
});

it("applies the server's UTF-8 byte limits and NUL rejection to administrative text", () => {
  expect(AdminCollaborationChangeSchema.safeParse({ ...input, reason: "中".repeat(333) }).success).toBe(true);
  expect(AdminCollaborationChangeSchema.safeParse({ ...input, reason: "中".repeat(334) }).success).toBe(false);
  expect(AdminCollaborationChangeSchema.safeParse({ ...input, reason: "reason\u0000tail" }).success).toBe(false);
  for (const field of ["expectedStatus", "targetStatus"] as const) {
    expect(AdminCollaborationChangeSchema.safeParse({ ...input, [field]: "中".repeat(43) }).success).toBe(false);
  }
});

it("correlates the durable operation and lifecycle result without treating unsettled runs as stopped", () => {
  const parsed = parseAdminCollaborationResult(result);
  expect(parsed?.result).toMatchObject({ creatorUserId: id(3), handlerUserId: id(5), queueState: "paused", unsettledRuns: 1 });
  expect(matchesAdminCollaborationResult(parsed!, id(1), id(7), input)).toBe(true);
  expect(matchesAdminCollaborationResult(parsed!, id(2), id(7), input)).toBe(false);
  expect(matchesAdminCollaborationResult(parsed!, id(1), id(8), input)).toBe(false);
  for (const changed of [{ ...input, successorUserId: id(4) }, { ...input, expectedStatus: "cancelled" },
    { ...input, targetStatus: "in_progress" }, { ...input, expectedCollaborationRevision: "9" }]) {
    expect(matchesAdminCollaborationResult(parsed!, id(1), id(7), changed)).toBe(false);
  }
});

it.each([{ actor_id: null }, { target_id: id(2) }, { target_kind: "issue" }, { kind: "user.restore" },
  { result_code: "completed" }, { state: "unknown" }, { confirmation: "pending" }])("rejects a mismatched administrative operation %j", (patch) => {
  expect(parseAdminCollaborationResult({ ...result, operation: { ...result.operation, ...patch } })).toBeNull();
});

it("does not convert reopen to automatic execution or permit a handler change on close", () => {
  expect(parseAdminCollaborationResult({ ...result, result: { ...result.result, queue_state: "active" } })).toBeNull();
  const closed = { operation: { ...result.operation, kind: "collaboration.complete", result_code: "completed" },
    result: { ...result.result, action: "complete", code: "completed", status: "done", queue_state: "closed" } };
  expect(parseAdminCollaborationResult(closed)).toBeNull();
  expect(parseAdminCollaborationResult({ ...closed, result: { ...closed.result, handler_user_id: closed.result.previous_handler_user_id } })).not.toBeNull();
});
