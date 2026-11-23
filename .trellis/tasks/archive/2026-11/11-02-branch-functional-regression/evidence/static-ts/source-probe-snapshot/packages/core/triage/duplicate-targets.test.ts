// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { api, ApiError } from "../api";
import type { Issue } from "../types";
import { triageDuplicateTargetsOptions } from "./queries";

vi.mock("../api", async (original) => ({
  ...(await original<typeof import("../api")>()),
  api: { listIssues: vi.fn(), getIssue: vi.fn() },
}));
afterEach(() => vi.resetAllMocks());

function issue(number: number): Issue {
  return {
    id: `issue-${number}`, identifier: `ACM-${number}`, number, workspace_id: "w1",
    title: `Task ${number}`, description: null, status: "done", priority: "none",
    admission_status: "accepted", assignee_type: null, assignee_id: null,
    creator_type: "member", creator_id: "u1", parent_issue_id: null, project_id: null,
    position: number, stage: null, start_date: null, due_date: null,
    metadata: {}, properties: {}, created_at: "2026-10-10", updated_at: "2026-10-10",
  };
}

describe("duplicate target queries", () => {
  it("browses successive raw windows without excluding closed tasks or subtasks", async () => {
    const rows = Array.from({ length: 65 }, (_, i) => issue(i + 1));
    vi.mocked(api.listIssues).mockImplementation(async (params) => ({
      issues: rows.slice(params?.offset, (params?.offset ?? 0) + 30), total: rows.length,
    }));
    const qc = new QueryClient();
    const result = await qc.fetchInfiniteQuery({ ...triageDuplicateTargetsOptions("w1", ""), pages: 4 });
    expect(result.pages.flatMap(page => page.issues)).toEqual(rows);
    expect(api.listIssues).toHaveBeenCalledTimes(3);
    expect(vi.mocked(api.listIssues).mock.calls.map(([params]) => params?.offset)).toEqual([0, 30, 60]);
    for (const [params] of vi.mocked(api.listIssues).mock.calls) {
      expect(params).toMatchObject({ workspace_id: "w1", limit: 30, sort_by: "created_at", sort_direction: "desc" });
      expect(params).not.toHaveProperty("open_only");
      expect(params).not.toHaveProperty("top_level_only");
    }
    qc.clear();
  });

  it("isolates workspaces and resolves full identifiers and UUIDs without numeric title matches", async () => {
    const qc = new QueryClient();
    const target = { ...issue(34), identifier: "12A-34" };
    vi.mocked(api.getIssue).mockResolvedValue(target);
    const options = triageDuplicateTargetsOptions("w1", "12A-34");
    expect(options.queryKey).not.toEqual(triageDuplicateTargetsOptions("w2", "12A-34").queryKey);
    expect(options.queryKey).not.toEqual(triageDuplicateTargetsOptions("w1", "other").queryKey);
    expect(triageDuplicateTargetsOptions("", "").enabled).toBe(false);
    expect((await qc.fetchInfiniteQuery(options)).pages).toEqual([{ issues: [target], total: 1 }]);
    expect(api.getIssue).toHaveBeenLastCalledWith("12A-34", expect.objectContaining({ workspaceId: "w1" }));
    const uuid = "a56e8429-ff4e-4eec-8743-93907c5e7469";
    await qc.fetchInfiniteQuery(triageDuplicateTargetsOptions("w1", uuid));
    expect(api.getIssue).toHaveBeenLastCalledWith(uuid, expect.objectContaining({ workspaceId: "w1" }));
    expect(api.listIssues).not.toHaveBeenCalled();
    qc.clear();
  });

  it("treats only missing exact references as no matches and preserves request errors", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(api.getIssue).mockRejectedValueOnce(new ApiError("Not found", 404, "Not Found"));
    expect((await qc.fetchInfiniteQuery(triageDuplicateTargetsOptions("w1", "ACM-4"))).pages)
      .toEqual([{ issues: [], total: 0 }]);
    for (const error of [new ApiError("Forbidden", 403, "Forbidden"), new Error("Offline")]) {
      vi.mocked(api.getIssue).mockRejectedValueOnce(error);
      await expect(qc.fetchInfiniteQuery(triageDuplicateTargetsOptions("w1", "ACM-5"))).rejects.toBe(error);
    }
    qc.clear();
  });
});
