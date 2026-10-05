// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { triageCountsOptions, triageListOptions, triageKeys, triageHistoryOptions, triageDetailOptions, triageSettingsOptions } from "./queries";
import { onTriageUpdated, invalidateTriageIssue } from "./cache";
import { issueKeys } from "../issues/queries";
import { projectKeys } from "../projects/queries";
vi.mock("../api", () => ({ api: { listTriageItems: vi.fn(), listTriageHistory: vi.fn(), getTriageItem: vi.fn(), getTriageSettings: vi.fn() } }));
afterEach(() => vi.clearAllMocks());

describe("triage workspace queries", () => {
  it("keys the whole filter/page and scopes all projection keys", () => {
    expect(triageListOptions("w1", { limit: 10, offset: 0 }).queryKey).not.toEqual(triageListOptions("w1", { limit: 20, offset: 0 }).queryKey);
    expect(triageListOptions("w1", { q: "x" }).queryKey).not.toEqual(triageListOptions("w1", { q: "y" }).queryKey);
    expect(triageHistoryOptions("w1", { processed_by: "u1" }).queryKey).not.toEqual(triageHistoryOptions("w1", { processed_by: "u2" }).queryKey);
    expect(triageDetailOptions("w1", "same-id").queryKey).not.toEqual(triageDetailOptions("w2", "same-id").queryKey);
    expect(triageSettingsOptions("").enabled).toBe(false);
    expect(triageDetailOptions("w1", "").enabled).toBe(false);
  });
  it("refreshes queue and global counts within 30 seconds for stored snoozes", () => {
    expect(triageListOptions("w1").refetchInterval).toBeLessThanOrEqual(30_000);
    expect(triageCountsOptions("w1").refetchInterval).toBeLessThanOrEqual(30_000);
  });
  it("counts uses an unfiltered scoped request independently of a queue filter", async () => {
    const response = { items: [], total: 0, counts: { pending: 12, ready: 5, snoozed: 7 }, limit: 1, offset: 0 };
    vi.mocked(api.listTriageItems).mockResolvedValue(response);
    const qc = new QueryClient();
    const data = await qc.fetchQuery(triageCountsOptions("w1"));
    expect(data).toEqual(response.counts);
    expect(api.listTriageItems).toHaveBeenCalledWith("w1", { view: "ready", limit: 1 }, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    qc.clear();
  });
  it("invalidates triage and formal projections only for the event workspace", () => {
    const qc = new QueryClient();
    const affected = [triageKeys.settings("w1"), triageKeys.counts("w1"), triageKeys.list("w1", {}), triageKeys.history("w1", {}), triageKeys.detail("w1", "i1"), triageKeys.import("w1", "b1"), issueKeys.detail("w1", "i1"), issueKeys.list("w1"), projectKeys.list("w1")];
    affected.forEach(key => qc.setQueryData(key, { ready: true }));
    qc.setQueryData(triageKeys.counts("w2"), { ready: true });
    onTriageUpdated(qc, { workspace_id: "w1", issue_id: "i1", batch_id: "b1" });
    affected.forEach(key => expect(qc.getQueryState(key)?.isInvalidated).toBe(true));
    expect(qc.getQueryState(triageKeys.counts("w2"))?.isInvalidated).toBe(false);
    qc.clear();
  });
  it("keeps triage queries fresh when an edit affects an unloaded pending input", () => {
    const qc = new QueryClient();
    const key = triageKeys.list("w1", { q: "new title" });
    qc.setQueryData(key, { items: [] });
    invalidateTriageIssue(qc, "w1", "unloaded", { admissionStatus: "pending" });
    expect(qc.getQueryState(key)?.isInvalidated).toBe(true);
    qc.clear();
  });
  it("does not refetch triage for unrelated formal issue content", () => {
    const qc = new QueryClient();
    const key = triageKeys.list("w1", {});
    qc.setQueryData(key, { items: [] });
    invalidateTriageIssue(qc, "w1", "formal", { admissionStatus: "accepted" });
    expect(qc.getQueryState(key)?.isInvalidated).toBe(false);
    qc.clear();
  });
  it("ignores malformed realtime events instead of invalidating an arbitrary workspace", () => {
    const qc = new QueryClient();
    qc.setQueryData(triageKeys.counts("w1"), {});
    onTriageUpdated(qc, { workspace_id: 4 });
    expect(qc.getQueryState(triageKeys.counts("w1"))?.isInvalidated).toBe(false);
    qc.clear();
  });
});
