// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider, onlineManager } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { api } from "../api";
import { useTriageAction, useCommitTriageBatch, usePreviewTriageImport, useUpdateTriageSettings } from "./mutations";
import { triageKeys } from "./queries";
import { issueKeys } from "../issues/queries";
import type { TriageActionInput, TriageActionResult } from "../types/triage";
vi.mock("../api", () => ({ api: { performTriageAction: vi.fn(), commitTriageBatch: vi.fn(), previewTriageImport: vi.fn(), updateTriageSettings: vi.fn() } }));
afterEach(() => { onlineManager.setOnline(true); vi.clearAllMocks(); });
const result: TriageActionResult = {
  item: { issue: { id: "i1", workspace_id: "w1", number: 1, identifier: "T-1", title: "Input", description: null, status: "todo", priority: "none", admission_status: "accepted", revision: 2, assignee_type: null, assignee_id: null, creator_type: "member", creator_id: "u1", parent_issue_id: null, project_id: null, position: 1, stage: null, start_date: null, due_date: null, metadata: {}, properties: {}, created_at: "2026-10-04T00:00:00Z", updated_at: "2026-10-04T00:00:00Z" }, candidate_project_id: null, candidate_assignee_type: null, candidate_assignee_id: null, reviewer_id: null, reviewer_valid: false, round: 1, first_entered_at: "2026-10-04T00:00:00Z", entered_at: "2026-10-04T00:00:00Z", snoozed_until: null, duplicate_issue_id: null, duplicate_identifier: null, source: "manual", source_url: null, external_id: null, batch_id: null, filename: null, row_number: null },
  action: { id: "a1", issue_id: "i1", actor_id: "u1", action: "accept", round: 1, reason: null, before: {}, after: {}, created_at: "2026-10-04T00:00:00Z", execution_status: "not_requested", task_id: null, execution_error: null },
};
const input: TriageActionInput = { request_id: "request-1", action: "accept", expected_revision: 1 };
function setup() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return { qc, wrapper: ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider> };
}

describe("triage mutation confirmation", () => {
  it("waits for admission receipt then refreshes projections without optimistic list insertion", async () => {
    const { qc, wrapper } = setup();
    let finish!: (value: TriageActionResult) => void;
    vi.mocked(api.performTriageAction).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    const pending = { items: ["pending"] };
    qc.setQueryData(triageKeys.list("w1", {}), pending);
    qc.setQueryData(issueKeys.list("w1"), { issues: [] });
    const { result: hook } = renderHook(() => useTriageAction("w1"), { wrapper });
    let promise!: Promise<TriageActionResult>;
    await act(async () => { promise = hook.current.mutateAsync({ id: "i1", input }); });
    expect(qc.getQueryData(triageKeys.list("w1", {}))).toEqual(pending);
    expect(qc.getQueryData(issueKeys.detail("w1", "i1"))).toBeUndefined();
    await act(async () => { finish(result); await promise; });
    expect(qc.getQueryData(issueKeys.detail("w1", "i1"))).toEqual(result.item.issue);
    expect(qc.getQueryData(issueKeys.list("w1"))).toEqual({ issues: [] });
    expect(qc.getQueryState(triageKeys.list("w1", {}))?.isInvalidated).toBe(true);
    qc.clear();
  });
  it("keeps original input and request ID after a conflict and refetches authoritative detail", async () => {
    const { qc, wrapper } = setup();
    vi.mocked(api.performTriageAction).mockRejectedValueOnce(new Error("conflict")).mockResolvedValueOnce(result);
    qc.setQueryData(triageKeys.detail("w1", "i1"), { ...result.item, issue: { ...result.item.issue, admission_status: "pending", revision: 1 } });
    const { result: hook } = renderHook(() => useTriageAction("w1"), { wrapper });
    const command = { id: "i1", input };
    await act(async () => { await expect(hook.current.mutateAsync(command)).rejects.toThrow("conflict"); });
    expect(qc.getQueryState(triageKeys.detail("w1", "i1"))?.isInvalidated).toBe(true);
    expect(command.input).toEqual(input);
    await act(async () => { await hook.current.mutateAsync(command); });
    expect(api.performTriageAction).toHaveBeenNthCalledWith(1, "w1", "i1", input);
    expect(api.performTriageAction).toHaveBeenNthCalledWith(2, "w1", "i1", input);
    qc.clear();
  });
  it("a pending command settles into its original workspace after navigation", async () => {
    const { qc, wrapper } = setup();
    let finish!: (value: TriageActionResult) => void;
    vi.mocked(api.performTriageAction).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    qc.setQueryData(triageKeys.counts("w1"), { pending: 1 });
    qc.setQueryData(triageKeys.counts("w2"), { pending: 0 });
    const { result: hook, rerender } = renderHook(({ wsId }) => useTriageAction(wsId), { initialProps: { wsId: "w1" }, wrapper });
    let promise!: Promise<TriageActionResult>;
    await act(async () => { promise = hook.current.mutateAsync({ id: "i1", input }); });
    rerender({ wsId: "w2" });
    await act(async () => { finish(result); await promise; });
    expect(api.performTriageAction).toHaveBeenCalledWith("w1", "i1", input);
    expect(qc.getQueryState(triageKeys.counts("w1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(triageKeys.counts("w2"))?.isInvalidated).toBe(false);
    expect(qc.getQueryData(issueKeys.detail("w2", "i1"))).toBeUndefined();
    qc.clear();
  });
  it("an offline paused command retains its invocation workspace after options change", async () => {
    const { qc, wrapper } = setup();
    onlineManager.setOnline(false);
    const settings = { supported: true, enabled: true, acceptance_status: "todo", require_priority: false, responsibility_mode: "none" as const, responsibility_member_id: null, revision: 2 };
    vi.mocked(api.updateTriageSettings).mockResolvedValue(settings);
    qc.setQueryData(triageKeys.settings("w2"), { ...settings, revision: 8 });
    const { result: hook, rerender } = renderHook(({ wsId }) => useUpdateTriageSettings(wsId), { initialProps: { wsId: "w1" }, wrapper });
    const command = { ...settings, expected_revision: 1 };
    let promise!: Promise<unknown>;
    await act(async () => { promise = hook.current.mutateAsync(command); });
    await waitFor(() => expect(hook.current.isPaused).toBe(true));
    expect(api.updateTriageSettings).not.toHaveBeenCalled();
    rerender({ wsId: "w2" });
    await act(async () => { onlineManager.setOnline(true); await promise; });
    expect(api.updateTriageSettings).toHaveBeenCalledWith("w1", command);
    expect(qc.getQueryData(triageKeys.settings("w1"))).toEqual(settings);
    expect(qc.getQueryData(triageKeys.settings("w2"))).toMatchObject({ revision: 8 });
    qc.clear();
  });
  it("retains a newer realtime detail if an older action receipt arrives later", async () => {
    const { qc, wrapper } = setup();
    vi.mocked(api.performTriageAction).mockResolvedValue(result);
    qc.setQueryData(issueKeys.detail("w1", "i1"), { ...result.item.issue, revision: 3, title: "Newer" });
    const { result: hook } = renderHook(() => useTriageAction("w1"), { wrapper });
    await act(async () => { await hook.current.mutateAsync({ id: "i1", input }); });
    expect(qc.getQueryData(issueKeys.detail("w1", "i1"))).toMatchObject({ title: "Newer", revision: 3 });
    qc.clear();
  });
  it("only applies successful batch receipts; failures retain their original request identity", async () => {
    const { qc, wrapper } = setup();
    vi.mocked(api.commitTriageBatch).mockResolvedValue({ results: [{ issue_id: "i1", status: "success", result }, { issue_id: "i2", status: "conflict", error: "Changed" }], success_count: 1 });
    const { result: hook } = renderHook(() => useCommitTriageBatch("w1"), { wrapper });
    const command = { items: [{ ...input, issue_id: "i1", action: "accept" as const }, { ...input, request_id: "request-2", issue_id: "i2", action: "accept" as const }] };
    await act(async () => { await hook.current.mutateAsync(command); });
    expect(qc.getQueryData(issueKeys.detail("w1", "i1"))).toEqual(result.item.issue);
    expect(qc.getQueryData(issueKeys.detail("w1", "i2"))).toBeUndefined();
    expect(command.items[1]?.request_id).toBe("request-2");
    qc.clear();
  });
  it("preview does not invalidate queue or imply an import was committed", async () => {
    const { qc, wrapper } = setup();
    const preview = { batch_id: "b1", filename: "x.csv", headers: [], mapping: {}, rows: [], counts: { valid: 0, warning: 0, error: 0, duplicate: 0 }, limits: { max_rows: 1000, max_bytes: 5242880 } };
    vi.mocked(api.previewTriageImport).mockResolvedValue(preview);
    qc.setQueryData(triageKeys.counts("w1"), {});
    const { result: hook } = renderHook(() => usePreviewTriageImport("w1"), { wrapper });
    await act(async () => { await hook.current.mutateAsync({ request_id: "r1", filename: "x.csv", csv: "title" }); });
    expect(qc.getQueryState(triageKeys.counts("w1"))?.isInvalidated).toBe(false);
    expect(qc.getQueryData(triageKeys.import("w1", "b1"))).toEqual(preview);
    qc.clear();
  });
});
