// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient } from "../api/client";
import { setApiInstance } from "../api";
import { completionCollaborationRequestFixture, completionPreviewFixture } from "../issues/completion-test-fixtures";
import { CollaborationCompletionPreviewSchema, CollaborationCompletionCandidatesSchema } from "./completion-schema";
import { parseCollaborationActivity } from "./activity-schema";
import { collaborationKeys, collaborationScope } from "./access";
import { deliveryCompletionScope } from "../issues/completion";
import { collaborationCompletionCandidatesOptions, collaborationCompletionKeys } from "./completion-queries";
import { eraseCollaborationStageSource } from "./stage-source-cache";
import { collaborationStageKeys } from "./stage-queries";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
afterEach(() => vi.unstubAllGlobals());
function preview() {
  const original = completionPreviewFixture(), request = completionCollaborationRequestFixture("collaboration_submission");
  const { operation_id: _operation, expires_at: _expires, reason: _reason, ...binding } = request;
  return { ...original, source_issue_id: request.evidence.issue_id, source_issue_revision: request.evidence.expected_issue_revision,
    observation: { ...original.observation, issue_id: request.evidence.issue_id, issue_revision: request.evidence.expected_issue_revision },
    authorization_binding: binding, stage_blockers: [], stage_blockers_truncated: false };
}
it("keeps the root numeric revision separate from the source and exact collaboration version", () => {
  const value = CollaborationCompletionPreviewSchema.parse(preview());
  expect(value.issue_revision).toBe(7); expect(value.source_issue_revision).toBe(5);
  expect(value.authorization_binding?.target).toMatchObject({ expected_collaboration_revision: "9007199254740993" });
  for (const change of [{ source_issue_id: value.issue_id }, { issue_revision: "9007199254740993" }, { source_issue_revision: 7 },
    { observation: { ...value.observation, issue_id: value.issue_id } }, { stage_blockers: [{ code: "stage_qa_missing", stage: "qa" }] }]) {
    expect(CollaborationCompletionPreviewSchema.safeParse({ ...preview(), ...change }).success).toBe(false);
  }
});
it.each(["off", "observe", "enforce"])("keeps stage-incomplete %s previews non-authorizing", (policy_mode) => {
  const value = { ...preview(), policy_mode, reason: "stage_incomplete", authorization_binding: null, max_expires_at: null,
    stage_blockers: [{ code: "stage_evidence_unavailable", stage: "verification" }] };
  expect(CollaborationCompletionPreviewSchema.safeParse(value).success).toBe(true);
  expect(CollaborationCompletionPreviewSchema.safeParse({ ...value, authorization_binding: preview().authorization_binding }).success).toBe(false);
});
it("requires actual source candidates and correlates normal/admin preview routes without ordinary endpoint fallback", async () => {
  const value = preview();
  const input = { source_issue_id: value.source_issue_id, approval_id: value.observation.approval_id, target: value.authorization_binding.target };
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(value))); vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://api.example.test");
  const result = await api.previewCollaborationCompletion(value.issue_id, input);
  expect(result.state).toBe("available");
  expect(fetch.mock.calls[0]?.[0]).toBe(`https://api.example.test/api/collaboration/issues/${value.issue_id}/delivery-evidence/completion-preview`);
  expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toEqual(input);
  await expect(api.previewCollaborationCompletion(value.issue_id, input, { admin: true })).rejects.toThrow("target scope mismatch");
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(CollaborationCompletionCandidatesSchema.safeParse({ protocol_version: 1, issue_id: value.issue_id, issue_revision: 7, source_issue_revision: 5,
    project_id: value.project_id, checked_at: value.checked_at, items: [], next_cursor: null, has_more: false }).success).toBe(false);
});
it("treats structured source denial and an absent old route as unavailable rather than empty successful evidence", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ code: "not_found" }), { status: 404 })).mockResolvedValueOnce(new Response("not found", { status: 404 }));
  vi.stubGlobal("fetch", fetch); const api = new ApiClient("https://api.example.test"), value = preview();
  expect((await api.listCollaborationCompletionCandidates(value.issue_id, value.source_issue_id)).state).toBe("denied");
  expect((await api.listCollaborationCompletionCandidates(value.issue_id, value.source_issue_id)).state).toBe("unavailable");
});
it("does not log an administrative completion source cursor or private error body", async () => {
  const log = vi.fn(), sourceId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  const api = new ApiClient("https://api.example.test", { logger: { debug: log, info: log, warn: log, error: log } });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: "delivery_completion_denied", error: "PRIVATE_ARTIFACT_ERROR" }), { status: 403 })));
  await api.listCollaborationCompletionCandidates(preview().issue_id, sourceId, { cursor: "PRIVATE_CURSOR" }, { admin: true });
  const output = JSON.stringify(log.mock.calls);
  expect(output).not.toContain(sourceId); expect(output).not.toContain("PRIVATE_CURSOR"); expect(output).not.toContain("PRIVATE_ARTIFACT_ERROR");
});

const id = "00000000-0000-4000-8000-000000000001";
const entry = { id, issue_id: id, event_seq: "1", kind: "stage_decision_recorded", actor: null, created_at: "2026-11-14T00:00:00Z",
  details_available: true, target: null, handoff: null, comment_id: null, changes: [], stage: { candidate_id: id, decision_id: id, step: "qa" } };
it("projects only closed stage activity metadata and discards malformed/private details", () => {
  const parse = (value: unknown) => parseCollaborationActivity({ issue_id: id, items: [value], next_cursor: null })?.items[0];
  expect(parse(entry)?.stage).toEqual(entry.stage);
  for (const stage of [{ ...entry.stage, report_id: id }, { ...entry.stage, content: "PRIVATE_EVIDENCE" }, { ...entry.stage, step: "future" }, null, undefined]) {
    const value = parse({ ...entry, stage });
    expect(value).toMatchObject({ stage: null, details_available: false, changes: [] });
    expect(JSON.stringify(value)).not.toContain("PRIVATE_EVIDENCE");
  }
  for (const kind of ["stage_execution_admitted", "stage_execution_observed", "stage_rollback_observed"]) {
    expect(parse({ ...entry, kind })).toMatchObject({ stage: null, details_available: false });
  }
});

it("source denial also erases completion observations while keeping original authorization references and other workspaces", () => {
  setApiInstance(new ApiClient("https://api.example.test"));
  const client = new QueryClient(), scope = collaborationScope(id), completion = { ...deliveryCompletionScope(id, id, id), adminOrganizationId: null };
  const key = collaborationCompletionKeys.candidates(completion, "00000000-0000-4000-8000-000000000002");
  const other = collaborationCompletionKeys.candidates({ ...completion, workspaceId: "00000000-0000-4000-8000-000000000003" }, id);
  const reference = ["delivery-completion-reference", scope.server, id];
  try {
    client.setQueryData(key, { private: "owned completion observation" }); client.setQueryData(other, { private: "other workspace" });
    client.setQueryData(reference, { authorization_id: id }); client.setQueryData(collaborationKeys.issue(scope, id), { id });
    eraseCollaborationStageSource(client, scope, id);
    expect(client.getQueryData(key)).toBeUndefined(); expect(client.getQueryData(other)).toEqual({ private: "other workspace" });
    expect(client.getQueryData(reference)).toEqual({ authorization_id: id }); expect(client.getQueryData(collaborationKeys.issue(scope, id))).toEqual({ id });
  } finally { client.clear(); }
});

it.each([404, 405])("keeps an absent completion route (%s) unavailable without declaring source denial", async (status) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("route unavailable", { status })));
  const api = new ApiClient("https://api.example.test"), value = preview();
  await expect(api.listCollaborationCompletionCandidates(value.issue_id, value.source_issue_id)).resolves.toEqual({ state: "unavailable", code: null });
});

it.each(["repeated cursor", "oversized requested page"])("rejects malformed completion pagination: %s", async (kind) => {
  const value = preview();
  const item = { approval_id: id, execution_id: id, source_revision: "a".repeat(40), spec_digest: "b".repeat(64), artifact_digest: "c".repeat(64),
    receipt_digest: "d".repeat(64), created_at: value.checked_at, expires_at: value.max_expires_at, summary: "Candidate", status: "succeeded", result_code: null, selectable: true };
  const response = { protocol_version: 1, issue_id: value.issue_id, issue_revision: value.issue_revision, source_issue_id: value.source_issue_id,
    source_issue_revision: value.source_issue_revision, project_id: value.project_id, checked_at: value.checked_at,
    items: kind === "repeated cursor" ? [] : [item, { ...item, approval_id: "00000000-0000-4000-8000-000000000002" }],
    next_cursor: kind === "repeated cursor" ? "original-cursor" : null, has_more: kind === "repeated cursor" };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(response))));
  const api = new ApiClient("https://api.example.test");
  await expect(api.listCollaborationCompletionCandidates(value.issue_id, value.source_issue_id, { limit: 1, cursor: "original-cursor" }))
    .resolves.toEqual({ state: "unavailable", code: null });
});

it.each([false, true])("source cache changes only after an explicit completion denial: %s", async (denied) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(denied ? JSON.stringify({ code: "not_found" }) : "route unavailable", { status: 404 })));
  setApiInstance(new ApiClient("https://api.example.test"));
  const client = new QueryClient(), scope = collaborationScope(id);
  const completion = { ...deliveryCompletionScope(id, id, id), adminOrganizationId: "00000000-0000-4000-8000-000000000009" };
  const key = collaborationStageKeys.codeCandidate(scope, id, "00000000-0000-4000-8000-000000000004", id);
  const prior = { retained: "previously readable source" };
  try {
    client.setQueryData(key, prior);
    const result = await client.fetchQuery(collaborationCompletionCandidatesOptions(client, completion, "00000000-0000-4000-8000-000000000003"));
    expect(client.getQueryData(key)).toEqual(denied ? undefined : prior);
    expect(result.state).toBe(denied ? "denied" : "unavailable");
  } finally { client.clear(); }
});
