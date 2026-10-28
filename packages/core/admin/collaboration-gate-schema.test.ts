// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { parseAdminCollaborationGate, parseAdminCollaborationGateResult } from "./collaboration-gate-schema";

const scope = "11111111-1111-4111-8111-111111111111";
const key = "22222222-2222-4222-8222-222222222222";
const gate = { scope, as_of: "2026-10-27T00:00:00Z", creation_enabled: false, execution_enabled: false, host_execution_enabled: true, execution_epoch: "9007199254740993", allowed_actions: ["open_creation", "open_execution"] };
afterEach(() => vi.unstubAllGlobals());

it("keeps the epoch exact and drops gate actions this client does not know", () => {
  const parsed = parseAdminCollaborationGate({ ...gate, allowed_actions: ["open_creation", "close_execution", "freeze"] });
  expect(parsed).toMatchObject({ scope, creationEnabled: false, hostExecutionEnabled: true, executionEpoch: "9007199254740993", allowedActions: ["open_creation"] });
});

it("refuses to guess a malformed switch", () => {
  expect(parseAdminCollaborationGate({ ...gate, creation_enabled: "true" })).toBeNull();
  expect(parseAdminCollaborationGate({ ...gate, execution_enabled: undefined })).toBeNull();
  expect(parseAdminCollaborationGate({ ...gate, allowed_actions: "open_creation" })?.allowedActions).toEqual([]);
});

it("sends the gate action with the original idempotency key and addresses the result by scope", async () => {
  const operation = { id: key, organization_id: scope, target_id: scope, kind: "collaboration.gate", state: "succeeded", result_code: "collaboration_creation_opened", confirmation: "not_required", reconciliation_state: "complete", version: 2 };
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify({ operation, target: { ...gate, creation_enabled: true, allowed_actions: ["close_creation", "open_execution"] } }), { status: 200 }));
  vi.stubGlobal("fetch", fetcher);
  const result = await new ApiClient("https://example.test").changeAdminCollaborationGate({ action: "open_creation", reason: "Pilot" }, key);
  expect(result?.target).toMatchObject({ id: scope, creationEnabled: true });
  const [url, request] = fetcher.mock.calls[0]!;
  expect(url).toBe("https://example.test/api/admin/collaboration/gate");
  expect(request?.method).toBe("POST");
  expect(new Headers(request?.headers).get("Idempotency-Key")).toBe(key);
  expect(JSON.parse(String(request?.body))).toEqual({ action: "open_creation", reason: "Pilot" });
});

it("sends the observed epoch only when opening execution", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response("{}", { status: 200 }));
  vi.stubGlobal("fetch", fetcher);
  await new ApiClient("https://example.test").changeAdminCollaborationGate({ action: "open_execution", expectedExecutionEpoch: "9007199254740993", reason: "Pilot agents" }, key);
  expect(JSON.parse(String(fetcher.mock.calls[0]![1]?.body))).toEqual({ action: "open_execution", expected_execution_epoch: "9007199254740993", reason: "Pilot agents" });
});

it("returns null for a malformed change response so the caller reconciles by key", async () => {
  const operation = { id: key, organization_id: scope, target_id: scope, kind: "collaboration.gate", state: "succeeded", result_code: "collaboration_creation_opened", confirmation: "not_required", reconciliation_state: "complete", version: 2 };
  expect(parseAdminCollaborationGateResult({ operation, target: { ...gate, creation_enabled: "yes" } })).toBeNull();
  expect(parseAdminCollaborationGateResult({ operation: { ...operation, target_id: "not-a-uuid" }, target: gate })).toBeNull();
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify({ operation }), { status: 200 })));
  await expect(new ApiClient("https://example.test").changeAdminCollaborationGate({ action: "open_creation", reason: "Pilot" }, key)).resolves.toBeNull();
});
