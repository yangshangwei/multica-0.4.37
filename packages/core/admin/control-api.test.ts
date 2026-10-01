// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";

const id = "11111111-1111-4111-8111-111111111111";
const key = "22222222-2222-4222-8222-222222222222";
const operation = { id, organization_id: id, target_id: id, kind: "installation.admission", state: "succeeded", result_code: "admission_stopped", confirmation: "not_required", reconciliation_state: "complete", version: 1 };
afterEach(() => vi.unstubAllGlobals());

it("preserves the admission bigint version and original idempotency key", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify({ operation, target: { id, admission: "stopped", admission_version: "9007199254740994" } }), { status: 202 }));
  vi.stubGlobal("fetch", fetcher);
  const result = await new ApiClient("https://example.test").changeAdminAdmission(id, { admission: "stopped", expectedAdmissionVersion: "9007199254740993", reason: "Maintenance" }, key);
  expect(result?.target.admissionVersion).toBe("9007199254740994");
  const [url, request] = fetcher.mock.calls[0]!;
  expect(url).toBe(`https://example.test/api/admin/installations/${id}/admission`);
  expect(new Headers(request?.headers).get("Idempotency-Key")).toBe(key);
  expect(JSON.parse(String(request?.body))).toEqual({ admission: "stopped", expected_admission_version: "9007199254740993", reason: "Maintenance" });
});

it("preserves the exact execution timestamp and uses the existing operation lookup", async () => {
  const stamp = "2026-10-02T01:02:03.123456789Z";
  const fence = { runtime_id: id, dispatched_at: stamp, target_version: "7" };
  const fetcher = vi.fn<typeof fetch>()
    .mockResolvedValueOnce(new Response(JSON.stringify({ operation: { ...operation, kind: "task.cancel" }, target: { id, status: "cancelled", state_version: "8", execution_fence: fence } }), { status: 202 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(operation), { status: 200 }));
  vi.stubGlobal("fetch", fetcher);
  const client = new ApiClient("https://example.test");
  await client.cancelAdminExecution(id, { expectedExecutionFence: { runtimeId: id, dispatchedAt: stamp, targetVersion: "7" }, reason: "Stop this execution" }, key);
  const [url, request] = fetcher.mock.calls[0]!;
  expect(url).toBe(`https://example.test/api/admin/tasks/${id}/cancel`);
  expect(new Headers(request?.headers).get("Idempotency-Key")).toBe(key);
  expect(JSON.parse(String(request?.body))).toEqual({ reason: "Stop this execution", expected_execution_fence: fence });
  expect((await client.getAdminOperation(id))?.id).toBe(id);
  expect(fetcher.mock.calls[1]?.[0]).toBe(`https://example.test/api/admin/operations/${id}`);
});
