// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { noopLogger } from "../logger";
import { collaborationControlReceiptFixture as receipt, collaborationControlRequestId as key, collaborationControlResumeInput as input } from "../collaboration/test-fixtures";

const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
afterEach(() => { vi.unstubAllGlobals(); setSchemaLogger(noopLogger); setCurrentWorkspace(null, null); });

it("sends exact captured Resume versions/key using account transport, retaining partial decisions", async () => {
  setCurrentWorkspace("ambient-space", receipt.issue_id);
  const fetchMock = vi.fn().mockResolvedValue(json(receipt));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  client.setToken("test-token");
  const controller = new AbortController();
  await expect(client.controlCollaborationIssue(receipt.issue_id, key, input, { signal: controller.signal })).resolves.toEqual(receipt);
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe(`https://api.example.test/api/collaboration/issues/${receipt.issue_id}/control`);
  expect(JSON.parse(init.body)).toEqual(input);
  const headers = new Headers(init.headers);
  expect(headers.get("Idempotency-Key")).toBe(key);
  expect(headers.get("Authorization")).toBe("Bearer test-token");
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  expect(init.signal).toBeInstanceOf(AbortSignal);
  expect(fetchMock).toHaveBeenCalledOnce();
});

it("keeps Stop independent from gate and hold observations", async () => {
  const { expected_execution_epoch: _gate, ...base } = input;
  const stop = { ...base, action: "stop" as const, targets: input.targets.map(({ expected_hold_version: _hold, ...target }) => target) };
  const channels = receipt.channels.map((channel) => ({ ...channel, code: channel.group_id === null ? "already_ended" : "stop_requested", queue_state: "paused" }));
  const stopped = { ...receipt, action: "stop", code: "stop_requested", queue_state: "paused", channels,
    targets: receipt.targets.map((target, index) => ({ ...target, ...channels[index] })), target_holds: [] };
  const fetchMock = vi.fn().mockResolvedValue(json(stopped));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  await expect(client.controlCollaborationIssue(receipt.issue_id, key, stop)).resolves.toEqual(stopped);
  const body = fetchMock.mock.calls[0]![1].body;
  expect(JSON.parse(body)).toEqual(stop);
  expect(body).not.toContain("expected_execution_epoch");
  expect(body).not.toContain("expected_hold_version");
  await expect(client.controlCollaborationIssue(receipt.issue_id, key, { ...stop, targets: input.targets })).rejects.toThrow("Invalid collaboration control intent");
  expect(fetchMock).toHaveBeenCalledOnce();
});

it("parses the same structured refusal from POST 409 and recovery 200 without retrying the mutation", async () => {
  const refused = { ...receipt, status: 409, code: "stale_hold", targets: [], channels: [], target_holds: [] };
  const fetchMock = vi.fn().mockResolvedValueOnce(json(refused, 409)).mockResolvedValueOnce(json(refused));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  await expect(client.controlCollaborationIssue(receipt.issue_id, key, input)).resolves.toEqual(refused);
  await expect(client.getCollaborationControlOperation(receipt.issue_id, key, input)).resolves.toEqual(refused);
  expect(fetchMock).toHaveBeenCalledTimes(2);
  expect(fetchMock.mock.calls[1]![0]).toBe(`https://api.example.test/api/collaboration/issues/${receipt.issue_id}/control/operations/${key}`);
  expect(fetchMock.mock.calls[1]![1].method).toBeUndefined();
  expect(fetchMock.mock.calls[1]![1].body).toBeUndefined();
});

it("rejects control receipts for another issue, original key, action or target scope", async () => {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  for (const patch of [{ issue_id: "00000000-0000-4000-8000-000000000099" }, { request_id: "another-key" }, { action: "stop" }, { scope: "target" }]) {
    fetchMock.mockResolvedValueOnce(json({ ...receipt, ...patch }));
    await expect(client.getCollaborationControlOperation(receipt.issue_id, key, input)).resolves.toBeNull();
  }
  const wrongTarget = "00000000-0000-4000-8000-000000000099";
  const channel = receipt.channels[0]!;
  const targetInput = { action: "resume" as const, scope: "target" as const, expected_execution_epoch: input.expected_execution_epoch,
    expected_lifecycle_epoch: input.expected_lifecycle_epoch, expected_participant_epoch: input.expected_participant_epoch, expected_account_epoch: input.expected_account_epoch, targets: [input.targets[0]!] };
  fetchMock.mockResolvedValueOnce(json({ ...receipt, code: "resumed", scope: "target", target_id: wrongTarget, channel_id: channel.channel_id, group_settled: channel.group_settled,
    channels: [channel], targets: [{ target_id: wrongTarget, ...channel }], target_holds: [{ ...receipt.target_holds[0], target_id: wrongTarget }] }));
  await expect(client.controlCollaborationIssue(receipt.issue_id, key, targetInput)).resolves.toBeNull();
});

it("keeps a missing original receipt unresolved and rejects access denial without private payloads", async () => {
  const error = vi.fn(), warn = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(json({ code: "operation_not_found", error: "PRIVATE_MISSING" }, 404))
    .mockResolvedValueOnce(json({ code: "forbidden", error: "PRIVATE_DENIAL", token: "PRIVATE_TOKEN" }, 403))
    .mockResolvedValueOnce(json({ ...receipt, code: "PRIVATE_CODE" }, 409));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, error, warn } });
  await expect(client.getCollaborationControlOperation(receipt.issue_id, key, input)).resolves.toBeNull();
  await expect(client.controlCollaborationIssue(receipt.issue_id, key, input)).rejects.toMatchObject({ status: 403, message: "Collaboration control request failed", body: undefined });
  await expect(client.controlCollaborationIssue(receipt.issue_id, key, input)).resolves.toBeNull();
  expect(fetchMock).toHaveBeenCalledTimes(3);
  expect(JSON.stringify([...error.mock.calls, ...warn.mock.calls])).not.toContain("PRIVATE_");
});

it("discards control responses and refusals after a credential change during decoding", async () => {
  for (const status of [200, 409]) {
    let resolve!: (value: unknown) => void;
    let started!: () => void;
    const decoding = new Promise<void>((done) => { started = done; });
    const pending = new Promise<unknown>((done) => { resolve = done; });
    const response = new Response("", { status });
    vi.spyOn(response, "json").mockImplementation(() => { started(); return pending; });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
    const client = new ApiClient("https://api.example.test");
    client.setToken("old-session");
    const result = expect(client.controlCollaborationIssue(receipt.issue_id, key, input)).rejects.toMatchObject({ name: "AbortError" });
    await decoding;
    client.setToken("new-session");
    resolve(status === 200 ? receipt : { ...receipt, status: 409, code: "stale_hold", targets: [], channels: [], target_holds: [] });
    await result;
  }
});
