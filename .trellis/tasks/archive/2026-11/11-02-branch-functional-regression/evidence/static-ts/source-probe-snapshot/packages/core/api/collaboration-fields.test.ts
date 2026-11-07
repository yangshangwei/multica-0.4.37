// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import type { CollaborationEditInput } from "../collaboration/schema";
import { collaborationFieldOptionsFixture as fieldOptions } from "../collaboration/test-fixtures";

const issueId = fieldOptions.issue_id;
const propertyId = fieldOptions.properties[0]!.id;
const labelId = fieldOptions.labels[0]!.id;
const input = {
  requestId: "original-fields-request",
  expectedCollaborationRevision: "9007199254740993",
  expectedLifecycleEpoch: "9007199254740994",
  expectedParticipantEpoch: "9007199254740995",
  expectedAccountEpoch: "9223372036854775807",
};
const versions = {
  expected_collaboration_revision: input.expectedCollaborationRevision,
  expected_lifecycle_epoch: input.expectedLifecycleEpoch,
  expected_participant_epoch: input.expectedParticipantEpoch,
  expected_account_epoch: input.expectedAccountEpoch,
};
const receipt = { code: "issue_edited", collaboration_revision: "9007199254740996" };

function jsonResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), { status });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

afterEach(() => {
  vi.unstubAllGlobals();
  setSchemaLogger(noopLogger);
  setCurrentWorkspace(null, null);
});

it.each([
  { fields: { priority: "high", labelIds: [labelId], properties: { [propertyId]: "option-one" } }, wire: { priority: "high", label_ids: [labelId], properties: { [propertyId]: "option-one" } } },
  { fields: { priority: null, labelIds: null, properties: null }, wire: { priority: null, label_ids: null, properties: null } },
  { fields: { labelIds: [] }, wire: { label_ids: [] } },
  { fields: { properties: {} }, wire: { properties: {} } },
  { fields: { properties: { [propertyId]: null } }, wire: { properties: { [propertyId]: null } } },
  { fields: { title: "Changed", priority: undefined, labelIds: undefined, properties: undefined }, wire: { title: "Changed" } },
] satisfies { fields: Partial<CollaborationEditInput>; wire: object }[])("preserves field patches and explicit empty/null/omitted meanings", async ({ fields, wire }) => {
  setCurrentWorkspace("unrelated-space", propertyId);
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(receipt));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  client.setToken("test-only-token");
  await expect(client.editCollaborationIssue(issueId, { ...input, ...fields })).resolves.toEqual(receipt);
  expect(fetchMock).toHaveBeenCalledOnce();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe(`https://api.example.test/api/collaboration/issues/${issueId}`);
  expect(init.method).toBe("PATCH");
  expect(JSON.parse(init.body)).toEqual({ ...versions, ...wire });
  const headers = new Headers(init.headers);
  expect(headers.get("Idempotency-Key")).toBe(input.requestId);
  expect(headers.get("Authorization")).toBe("Bearer test-only-token");
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
});

it("keeps the original property patch, label order, versions and key after an uncertain write", async () => {
  const fields: CollaborationEditInput = { ...input, priority: "none", labelIds: [labelId, labelId], properties: { [propertyId]: ["option-one"] } };
  const original = structuredClone(fields);
  const fetchMock = vi.fn()
    .mockRejectedValueOnce(new TypeError("Network unavailable"))
    .mockResolvedValueOnce(jsonResponse({ code: "stale_revision", collaboration_revision: "9223372036854775807" }, 409));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  await expect(client.editCollaborationIssue(issueId, fields)).rejects.toThrow("Network unavailable");
  expect(fetchMock).toHaveBeenCalledOnce();
  await expect(client.editCollaborationIssue(issueId, fields)).rejects.toMatchObject({ status: 409, body: { code: "stale_revision" } });
  expect(fetchMock).toHaveBeenCalledTimes(2);
  for (const [, init] of fetchMock.mock.calls) {
    expect(JSON.parse(init.body)).toEqual({ ...versions, priority: "none", label_ids: [labelId, labelId], properties: { [propertyId]: ["option-one"] } });
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe(input.requestId);
  }
  expect(fields).toEqual(original);
});

it("reads only the explicit issue's authorized options without current workspace headers", async () => {
  setCurrentWorkspace("unrelated-space", propertyId);
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(fieldOptions));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  client.setToken("test-only-token");
  const controller = new AbortController();
  await expect(client.getCollaborationFieldOptions(issueId, { signal: controller.signal })).resolves.toEqual(fieldOptions);
  expect(fetchMock).toHaveBeenCalledOnce();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe(`https://api.example.test/api/collaboration/issues/${issueId}/field-options`);
  expect(init.method ?? "GET").toBe("GET");
  expect(init.body).toBeUndefined();
  const headers = new Headers(init.headers);
  expect(headers.get("Authorization")).toBe("Bearer test-only-token");
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  controller.abort();
  expect(init.signal.aborted).toBe(true);
});

it("rejects a valid options response for a different issue", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ ...fieldOptions, issue_id: propertyId })));
  await expect(new ApiClient("https://api.example.test").getCollaborationFieldOptions(issueId)).resolves.toBeNull();
});

it.each([403, 404, 503])("does not replace a %s options response with broad workspace catalogs", async (status) => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ code: "unavailable" }, status));
  vi.stubGlobal("fetch", fetchMock);
  await expect(new ApiClient("https://api.example.test").getCollaborationFieldOptions(issueId)).rejects.toMatchObject({ status });
  expect(fetchMock).toHaveBeenCalledOnce();
});

it("makes malformed private options unavailable without leaking response or parser text", async () => {
  const warn = vi.fn();
  const info = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  vi.stubGlobal("fetch", vi.fn()
    .mockResolvedValueOnce(new Response("PRIVATE_INVALID_JSON"))
    .mockResolvedValueOnce(jsonResponse({ ...fieldOptions, properties: [{ ...fieldOptions.properties[0], type: "PRIVATE_TYPE" }] })));
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
  await expect(client.getCollaborationFieldOptions(issueId)).resolves.toBeNull();
  await expect(client.getCollaborationFieldOptions(issueId)).resolves.toBeNull();
  expect(warn).toHaveBeenCalledTimes(2);
  expect(JSON.stringify([...warn.mock.calls, ...info.mock.calls])).not.toContain("PRIVATE_");
});

it.each(["account", "server", "caller"])("discards field options after %s change during body parsing", async (reason) => {
  const body = deferred<unknown>();
  const parsing = deferred<void>();
  const response = new Response();
  vi.spyOn(response, "json").mockImplementation(() => { parsing.resolve(); return body.promise; });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  const client = new ApiClient("https://api.example.test");
  client.setToken("first-account");
  const controller = new AbortController();
  const result = expect(client.getCollaborationFieldOptions(issueId, { signal: controller.signal })).rejects.toMatchObject({ name: "AbortError" });
  await parsing.promise;
  if (reason === "account") client.setToken("second-account");
  else if (reason === "server") client.invalidateSession();
  else controller.abort();
  body.resolve(fieldOptions);
  await result;
});
