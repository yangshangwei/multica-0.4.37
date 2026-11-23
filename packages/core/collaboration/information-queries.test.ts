// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess } from "./access";
import { collaborationCreationOptions } from "./creation";
import { collaborationInformationHistoryOptions, collaborationInformationHandoffOptions,
  collaborationInformationAffectedOptions,
  shareCollaborationInformationPages, shareCollaborationInformationAffectedPages } from "./information-queries";
import { CollaborationInformationAffectedSchema, CollaborationInformationHistorySchema } from "./information-schema";
import { collaborationIssueFixture } from "./test-fixtures";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const id = auth.user.id;
const uuid = (value: number) => `00000000-0000-4000-8000-${value.toString(16).padStart(12, "0")}`;
const client = new QueryClient();
const response = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const item = (value: number, fraction = "000100") => ({ kind: "information_request", handoff_id: uuid(value), created_at: `2026-11-18T08:00:00.${fraction}Z`,
  provenance: { state: "legacy_unlinked", event_id: null, event_sequence: null }, actor_user_id: id, note: "Legacy",
  from_user_id: null, to_user_id: id, previous_status: "todo", resolution: "open", requester_user_id: null,
  parent_request_id: null, block: { protocol_version: 1, state: "legacy" }, superseded_by_repair_handoff_id: null });
const upper = { handoff_id: uuid(2), created_at: "2026-11-18T08:00:00.000900Z" };
const history = (value: number, next: string | null, fraction = "000100") => CollaborationInformationHistorySchema.parse({
  protocol_version: 1, issue_id: id, collaboration_revision: "2", lifecycle_epoch: "1", upper_handoff: upper,
  items: [item(value, fraction)], next_cursor: next,
});
const affected = (ids: number[], next: string | null, count = "3") => CollaborationInformationAffectedSchema.parse({
  protocol_version: 1, issue_id: id, collaboration_revision: "2", lifecycle_epoch: "1", upper_handoff: upper,
  repair_handoff_id: uuid(9), superseded_request_count: count, next_cursor: next,
  items: ids.map((value) => ({ request_id: uuid(value), request_event_id: null,
    provenance: { state: "legacy_unlinked", event_id: null, event_sequence: null }, resolution: "superseded", superseded_by_repair_handoff_id: uuid(9) })),
});
afterEach(() => { client.clear(); vi.unstubAllGlobals(); });

it("keeps one revision/lifecycle/watermark and exact microsecond ordering across history pages", () => {
  const a = history(2, "page-2", "000900"), b = history(3, null);
  const data = { pages: [a, b], pageParams: [undefined, "page-2"] };
  expect(shareCollaborationInformationPages(undefined, data)).toEqual(data);
  expect(() => shareCollaborationInformationPages(undefined, { ...data, pages: [a, { ...b, collaboration_revision: "3" }] })).toThrow();
  expect(() => shareCollaborationInformationPages(undefined, { ...data, pages: [a, { ...b, items: a.items }] })).toThrow();
  expect(() => shareCollaborationInformationPages(undefined, { ...data, pages: [a, { ...b, next_cursor: "page-2" }] })).toThrow();
});

it("does not call a partial affected relation complete or omit legacy identities", () => {
  const first = affected([2], "next"), last = affected([3, 4], null);
  const data = { pages: [first, last], pageParams: [undefined, "next"] };
  expect(shareCollaborationInformationAffectedPages(undefined, data)).toEqual(data);
  expect(() => shareCollaborationInformationAffectedPages(undefined, { pages: [affected([2, 3], null)], pageParams: [undefined] })).toThrow();
  expect(() => shareCollaborationInformationAffectedPages(undefined, { ...data, pages: [first, affected([2, 4], null)] })).toThrow();
  expect(() => shareCollaborationInformationAffectedPages(undefined, { ...data, pages: [first, affected([3, 4], null, "4")] })).toThrow();
});

it("binds the affected read to the original repair's immutable decimal denominator", async () => {
  setApiInstance(new ApiClient("https://server.example")); const scope = collaborationScope(id);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(affected([2], "next", "3"))));
  await expect(client.fetchInfiniteQuery(collaborationInformationAffectedOptions(client, scope, id, uuid(9), undefined, "2"))).rejects.toThrow();
});

it("treats a missing exact handoff as unavailable after a successful current parent read", async () => {
  setApiInstance(new ApiClient("https://server.example")); const scope = collaborationScope(id);
  client.setQueryData(collaborationKeys.issue(scope, id), collaborationIssueFixture);
  const fetch = vi.fn().mockResolvedValueOnce(response({ code: "not_found" }, 404)).mockResolvedValueOnce(response(collaborationIssueFixture));
  vi.stubGlobal("fetch", fetch);
  await expect(client.fetchQuery(collaborationInformationHandoffOptions(client, scope, id, uuid(7)))).rejects.toThrow();
  expect(collaborationIssueAccessDenied(client, scope, id)).toBe(false);
  expect(client.getQueryData(collaborationKeys.issue(scope, id))).toEqual(collaborationIssueFixture);
  expect(fetch.mock.calls[1]?.[0]).toContain(`/issues/${id}`);
});

it("clears protected history only after the current parent denial and rejects late private pages", async () => {
  setApiInstance(new ApiClient("https://server.example")); const scope = collaborationScope(id);
  let finish!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const options = collaborationInformationHistoryOptions(client, scope, id);
  client.setQueryData(options.queryKey, { pages: [history(2, null, "000900")], pageParams: [undefined] });
  if (typeof options.queryFn !== "function") throw new Error("The protected history query is required");
  const pending = expect(options.queryFn({ client, queryKey: options.queryKey, signal: new AbortController().signal,
    pageParam: undefined, direction: "forward", meta: undefined })).rejects.toMatchObject({ name: "AbortError" });
  clearCollaborationAccess(client, scope, id); finish(response(history(2, null, "000900"))); await pending;
  expect(client.getQueryData(options.queryKey)).toBeUndefined();
});

it("never reuses a previous project's policy as the selected project observation", async () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api); const scope = collaborationScope(id);
  const a = collaborationCreationOptions(client, scope, id, "", uuid(2)), b = collaborationCreationOptions(client, scope, id, "", uuid(3));
  expect(a.queryKey).not.toEqual(b.queryKey);
  const source = { id, name: "Source", slug: "source" };
  const fetch = vi.fn().mockResolvedValue(response({ creation_enabled: true, sources: [source], source,
    max_attachment_bytes: 104857600, stage_policy_observation: { protocol_version: 1, state: "known", source_workspace_id: id,
      project_id: uuid(2), policy_revision: "0", managed: false } }));
  vi.stubGlobal("fetch", fetch);
  await expect(api.getCollaborationCreationOptions(id, { projectId: uuid(3) })).resolves.toBeNull();
  expect(fetch.mock.calls[0]?.[0]).toContain(`project_id=${uuid(3)}`);
});

it("rejects oversized history responses and malformed identities without fabricating an empty page", async () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const large = { ...history(2, null, "000900"), unexpected: "x".repeat(65_536) };
  const fetch = vi.fn().mockResolvedValueOnce(response(large)).mockResolvedValueOnce(response({ ...history(2, null, "000900"), issue_id: uuid(8) }));
  vi.stubGlobal("fetch", fetch);
  await expect(api.getCollaborationInformationHistory(id)).resolves.toBeNull();
  await expect(api.getCollaborationInformationHistory(id)).resolves.toBeNull();
});
