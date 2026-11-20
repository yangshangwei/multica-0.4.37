// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess } from "./access";
import { CollaborationActionSchema, sendCollaborationAction } from "./action";
import { readCollaborationDrafts } from "./draft";
import { executeCollaborationAction, prepareCollaborationAction } from "./operation";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const key = "00000000-0000-4000-8000-000000000002";
const handoff = "00000000-0000-4000-8000-000000000003";
const values = new Map<string, string>();
const storage = { keys: () => [...values.keys()], getItem: (id: string) => values.get(id) ?? null,
  setItem: (id: string, value: string) => { values.set(id, value); }, removeItem: (id: string) => { values.delete(id); } };
let client = new QueryClient();
const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const information = { protocol_version: 1, managed: true, context_version: "a".repeat(64), active_request_id: null, record_state: "none",
  actions: { request: { available: false, code: "information_repair_required" }, return: { available: false, code: "information_context_unavailable" }, repair: { available: true, code: null } } };
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(auth.user.id);
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, status: "blocked", status_category: "blocked",
    write_context: { participant_epoch: "2", account_epoch: "3" }, information_workflow: information });
  if (!issue) throw new Error("The canonical information fixture is invalid");
  const draft = prepareCollaborationAction(scope, issue, key, { action: "repair_information", targetUserId: auth.user.id,
    note: "Explicit human repair", expectedStatus: "todo", statusOptionsVersion: "b".repeat(64),
    informationRepair: { context_version: "a".repeat(64), active_request_id: null, basis: { kind: "human_selected" } } });
  return { scope, issue, draft };
}
const receipt = { code: "information_repaired", collaboration_revision: "9007199254740994",
  information_repair_confirmation: { repair_handoff_id: handoff, prior_active_request_id: null,
    status: "todo", handler_user_id: auth.user.id, superseded_count: "0" } };
afterEach(() => { client.clear(); client = new QueryClient(); values.clear(); vi.unstubAllGlobals(); });

it("sends one explicit atomic repair with its exact context and no block or completion fields", async () => {
  const { scope, issue, draft } = setup();
  const fetch = vi.fn().mockResolvedValue(response(receipt)); vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).resolves.toEqual(receipt);
  expect(fetch).toHaveBeenCalledOnce();
  expect(JSON.parse(String(fetch.mock.calls[0]?.[1]?.body))).toEqual({ expected_collaboration_revision: issue.collaboration_revision,
    expected_lifecycle_epoch: "1", expected_participant_epoch: "2", expected_account_epoch: "3",
    target_user_id: auth.user.id, purpose: "information_repair", note: "Explicit human repair", expected_status: "todo",
    status_options_version: "b".repeat(64), information_repair: { context_version: "a".repeat(64), active_request_id: null, basis: { kind: "human_selected" } } });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it.each([null, { code: "operation_not_found" }, { code: "future_success", collaboration_revision: "2" }])("keeps a missing or malformed repair outcome pending without POST replay", async (body) => {
  const { scope, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response")).mockResolvedValueOnce(response(body, body && "code" in body && body.code === "operation_not_found" ? 404 : 200));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).rejects.toThrow();
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch.mock.calls[1]?.[0]).toContain(`/operations/${key}`);
  expect(fetch.mock.calls[1]?.[1]?.method ?? "GET").toBe("GET");
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
});

it("retires a confirmed original GET only when the repair confirmation matches the captured status, target and pointer", async () => {
  const { scope, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response"))
    .mockResolvedValueOnce(response({ ...receipt, information_repair_confirmation: { ...receipt.information_repair_confirmation, status: "in_progress" } }))
    .mockResolvedValueOnce(response(receipt));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
});

it("settles an exact stored refusal without sending a deferred repair or B2", async () => {
  const { scope, draft } = setup();
  const refusal = { status: 409, code: "information_context_changed", collaboration_revision: "9007199254740994" };
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response")).mockResolvedValueOnce(response(refusal));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toEqual(refusal);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch).toHaveBeenCalledTimes(2);
});

it.each([
  { status: 503, code: "information_context_unavailable" },
  { status: 409, code: "information_context_unavailable" },
  { status: 403, code: "forbidden" },
])("settles the original GET's exact $code status $status without another POST", async ({ status, code }) => {
  const { scope, issue, draft } = setup();
  const refusal = { status, code, collaboration_revision: "9007199254740994" };
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response")).mockResolvedValueOnce(response(refusal));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toEqual(refusal);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(false);
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch.mock.calls[1]?.[0]).toBe(`https://server.example/api/collaboration/issues/${issue.id}/operations/${key}`);
  expect(fetch.mock.calls[1]?.[1]?.method ?? "GET").toBe("GET");
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
});

it("requires original GET recovery before a declared semantic 503 can retire the repair", async () => {
  const { scope, draft } = setup();
  const refusal = { status: 503, code: "information_context_unavailable", collaboration_revision: "9007199254740994" };
  const fetch = vi.fn().mockResolvedValueOnce(response(refusal, 503)).mockResolvedValueOnce(response(refusal));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toEqual(refusal);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
});

it.each([
  { label: "unknown code", body: { status: 503, code: "unknown_refusal", collaboration_revision: "2" } },
  { label: "mismatched authority status", body: { status: 503, code: "forbidden", collaboration_revision: "2" } },
  { label: "different unavailable status", body: { status: 502, code: "information_context_unavailable", collaboration_revision: "2" } },
  { label: "missing revision", body: { status: 503, code: "information_context_unavailable" } },
  { label: "private delivery confirmation", body: { status: 503, code: "information_context_unavailable", collaboration_revision: "2",
    delivery_confirmation: { submission_id: handoff, attachment_ids: [] } } },
  { label: "repair success confirmation", body: { status: 503, code: "information_context_unavailable", collaboration_revision: "2",
    information_repair_confirmation: receipt.information_repair_confirmation } },
  { label: "private status options", body: { status: 503, code: "information_context_unavailable", collaboration_revision: "2",
    status_options: { action: "change_status", status_options_version: "b".repeat(64), default_status: "todo",
      options: [{ key: "todo", name: "To do", category: "todo" }] } } },
])("keeps $label pending during semantic 503 recovery", async ({ body }) => {
  const { scope, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response")).mockResolvedValueOnce(response(body));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch.mock.calls[1]?.[1]?.method ?? "GET").toBe("GET");
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
});

it("keeps an unsealed dependency GET 503 pending without exposing its error body", async () => {
  const { scope, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response"))
    .mockResolvedValueOnce(response({ code: "information_context_unavailable", error: "Private dependency error" }, 503));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).rejects.toMatchObject({ status: 503, body: undefined });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
});

it("keeps semantic 503 recovery unavailable for a different operation kind", async () => {
  const { scope, issue } = setup();
  const draft = prepareCollaborationAction(scope, issue, key, { action: "transfer", targetUserId: auth.user.id, note: "Original transfer" });
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response"))
    .mockResolvedValueOnce(response({ status: 503, code: "information_context_unavailable", collaboration_revision: "2" }));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(fetch).toHaveBeenCalledTimes(2);
});

it("retains actual protected-read denial after an unsealed GET 403", async () => {
  const { scope, issue, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Lost response"))
    .mockResolvedValueOnce(response({ code: "forbidden" }, 403));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).rejects.toMatchObject({ status: 403 });
  expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(true);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch).toHaveBeenCalledTimes(2);
});

it("fences a late repair result after actual read access loss", async () => {
  const { scope, issue, draft } = setup();
  let finish!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toMatchObject({ name: "AbortError" });
  clearCollaborationAccess(client, scope, issue.id); finish(response(receipt)); await pending;
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toBeUndefined();
});

it("requires a full new managed request while retaining old omitted unmanaged request bytes", async () => {
  const { scope, issue } = setup();
  const legacy = { action: "request_information" as const, targetUserId: auth.user.id, note: "Legacy note", expectedStatus: "blocked", statusOptionsVersion: "b".repeat(64) };
  expect(() => prepareCollaborationAction(scope, issue, key, legacy)).toThrow();
  const unmanaged = { ...issue, information_workflow: { ...issue.information_workflow!, managed: false } };
  const old = prepareCollaborationAction(scope, unmanaged, key, legacy);
  const fetch = vi.fn().mockResolvedValue(response({ code: "information_requested", collaboration_revision: "9007199254740994" }));
  vi.stubGlobal("fetch", fetch);
  await sendCollaborationAction(scope, issue.id, CollaborationActionSchema.parse(old.payload));
  const body = JSON.parse(String(fetch.mock.calls[0]?.[1]?.body));
  expect(Object.hasOwn(body, "unblock_condition")).toBe(false);
  expect(Object.hasOwn(body, "estimated_resolution_at")).toBe(false);
});
