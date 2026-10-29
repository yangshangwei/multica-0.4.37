// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import type { CollaborationRecipientSearchInput } from "../collaboration/schema";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import { setCurrentWorkspace } from "../platform/workspace-storage";

const issueId = "00000000-0000-4000-8000-000000000001";
const sourceWorkspaceId = "00000000-0000-4000-8000-000000000002";
const member = { user_id: "00000000-0000-4000-8000-000000000003", name: "Member", avatar_url: "", external: false };
const outsider = { user_id: "00000000-0000-4000-8000-000000000004", name: "Outsider", avatar_url: "", external: true };
const page = { issue_id: issueId, source_workspace_id: sourceWorkspaceId, scope: "directory", source_member: true, recent: [outsider], recipients: [member, outsider], next_cursor: "page-two" };
const issueInput: CollaborationRecipientSearchInput = { issueId };

const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const client = () => new ApiClient("https://api.example.test");

afterEach(() => {
  vi.unstubAllGlobals();
  setSchemaLogger(noopLogger);
  setCurrentWorkspace(null, null);
});

it.each([
  { input: { issueId, q: " PRIVATE_NAME " } satisfies CollaborationRecipientSearchInput, data: page, body: { issue_id: issueId, q: "PRIVATE_NAME", limit: 30, members_only: false } },
  { input: { sourceWorkspaceId, cursor: "page-two", limit: 10, membersOnly: true } satisfies CollaborationRecipientSearchInput, data: { ...page, issue_id: null, next_cursor: null },
    body: { source_workspace_id: sourceWorkspaceId, q: "", cursor: "page-two", limit: 10, members_only: true } },
])("sends only the supplied context, query and paging", async ({ input, data, body }) => {
  setCurrentWorkspace("unrelated-workspace", member.user_id);
  const info = vi.fn();
  const fetchMock = vi.fn().mockResolvedValue(json(data));
  vi.stubGlobal("fetch", fetchMock);
  const api = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
  const controller = new AbortController();
  await expect(api.searchCollaborationRecipients(input, { signal: controller.signal })).resolves.not.toBeNull();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe("https://api.example.test/api/collaboration/recipients/search");
  expect(init.method).toBe("POST");
  expect(JSON.parse(init.body)).toEqual(body);
  const headers = new Headers(init.headers);
  expect(headers.get("Content-Type")).toBe("application/json");
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  expect(headers.has("Idempotency-Key")).toBe(false);
  controller.abort();
  expect(init.signal.aborted).toBe(true);
  expect(JSON.stringify(info.mock.calls)).not.toContain("PRIVATE_");
});

it("keeps recent collaborators only on the first unfiltered page", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(json(page))));
  const api = client();
  await expect(api.searchCollaborationRecipients(issueInput)).resolves.toEqual(page);
  await expect(api.searchCollaborationRecipients({ issueId, q: "Mem" })).resolves.toMatchObject({ recent: [], recipients: [member, outsider] });
  await expect(api.searchCollaborationRecipients({ issueId, cursor: "page-one" })).resolves.toMatchObject({ recent: [], recipients: [member, outsider] });
});

it("drops one malformed account without emptying the rest of the page", async () => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  const extra = { ...member, email: "PRIVATE_EMAIL", username: "PRIVATE_USERNAME" };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ ...page, recent: [null, outsider],
    recipients: [extra, { ...outsider, user_id: "PRIVATE_USER" }, { ...outsider, external: "true" },
      { ...outsider, name: 42 }, { ...outsider, avatar_url: null }, { name: "No identity" }, outsider] })));
  const result = await client().searchCollaborationRecipients(issueInput);
  expect(result).toEqual({ ...page, recent: [outsider], recipients: [member, outsider] });
  expect(JSON.stringify(result)).not.toContain("PRIVATE_");
  expect(warn).not.toHaveBeenCalled();
});

it("drops an ambiguous membership row when the caller is a source member", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ ...page, recent: [{ ...outsider, external: null }], recipients: [{ ...outsider, external: null }, member] })));
  await expect(client().searchCollaborationRecipients(issueInput)).resolves.toMatchObject({ recent: [], recipients: [member] });
});

it.each([
  ["invalid json", new Response("PRIVATE_JSON")],
  ["a non-array list", json({ ...page, recipients: { PRIVATE_LIST: member } })],
  ["a missing membership flag", json({ ...page, source_member: undefined })],
  ["a malformed context", json({ ...page, issue_id: "PRIVATE_ISSUE" })],
  ["an empty cursor", json({ ...page, next_cursor: "" })],
])("returns unavailable for %s without logging private values", async (_, response) => {
  const warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  await expect(client().searchCollaborationRecipients(issueInput)).resolves.toBeNull();
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
});

it.each([
  { input: issueInput, data: { ...page, issue_id: sourceWorkspaceId } },
  { input: issueInput, data: { ...page, issue_id: null } },
  { input: { sourceWorkspaceId } satisfies CollaborationRecipientSearchInput, data: page },
  { input: { sourceWorkspaceId } satisfies CollaborationRecipientSearchInput, data: { ...page, issue_id: null, source_workspace_id: issueId } },
  { input: { issueId, limit: 1 } satisfies CollaborationRecipientSearchInput, data: page },
  { input: { issueId, cursor: "page-two" } satisfies CollaborationRecipientSearchInput, data: page },
])("rejects a page that belongs to a different search context", async ({ input, data }) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json(data)));
  await expect(client().searchCollaborationRecipients(input)).resolves.toBeNull();
});

it("does not reinterpret an in-flight search when its caller changes context", async () => {
  let resolve!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((done) => { resolve = done; })));
  const input = { ...issueInput };
  const result = client().searchCollaborationRecipients(input);
  input.issueId = sourceWorkspaceId;
  resolve(json({ ...page, issue_id: sourceWorkspaceId }));
  await expect(result).resolves.toBeNull();
});

it("never discloses membership to an outside caller or offers non-members in a members-only list", async () => {
  vi.stubGlobal("fetch", vi.fn()
    .mockResolvedValueOnce(json({ ...page, source_member: false }))
    .mockResolvedValueOnce(json({ ...page, recent: [member, outsider], next_cursor: "page-two" }))
    .mockResolvedValueOnce(json({ ...page, source_member: false, recent: [member], recipients: [member] })));
  const api = client();
  await expect(api.searchCollaborationRecipients(issueInput)).resolves.toMatchObject({
    recent: [{ ...outsider, external: null }], recipients: [{ ...member, external: null }, { ...outsider, external: null }] });
  await expect(api.searchCollaborationRecipients({ issueId, membersOnly: true })).resolves.toMatchObject({ recent: [member], recipients: [member], next_cursor: "page-two" });
  await expect(api.searchCollaborationRecipients({ issueId, membersOnly: true })).resolves.toMatchObject({ source_member: false, recent: [], recipients: [], next_cursor: null });
});

it("refuses missing, mixed or invalid searches before sending a request", async () => {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const api = client();
  // @ts-expect-error Exercise callers that bypass the exclusive context type.
  await expect(api.searchCollaborationRecipients({ issueId, sourceWorkspaceId })).rejects.toThrow("exactly one context");
  // @ts-expect-error Exercise an older or untyped caller without a context.
  await expect(api.searchCollaborationRecipients({ q: "PRIVATE_QUERY" })).rejects.toThrow("exactly one context");
  for (const limit of [0, 51, 1.5, Number.NaN]) await expect(api.searchCollaborationRecipients({ issueId, limit })).rejects.toThrow(TypeError);
  await expect(api.searchCollaborationRecipients({ issueId, cursor: "" })).rejects.toThrow(TypeError);
  expect(fetchMock).not.toHaveBeenCalled();
});

it("treats an older server without the route as unavailable but keeps structured refusals", async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
    .mockResolvedValueOnce(json({ error: "PRIVATE_ERROR", code: "not_found" }, 404))
    .mockResolvedValueOnce(json({ error: "PRIVATE_ERROR", code: "invalid_recipient_query" }, 400));
  vi.stubGlobal("fetch", fetchMock);
  const api = client();
  await expect(api.searchCollaborationRecipients(issueInput)).resolves.toBeNull();
  await expect(api.searchCollaborationRecipients(issueInput)).rejects.toMatchObject({ status: 404, body: { code: "not_found" } });
  await expect(api.searchCollaborationRecipients(issueInput)).rejects.toBeInstanceOf(ApiError);
  expect(fetchMock).toHaveBeenCalledTimes(3);
});
