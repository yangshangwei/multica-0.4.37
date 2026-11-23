// @vitest-environment node
import { InfiniteQueryObserver, QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "../api";
import type { Issue } from "../types";
import { onIssueCreated, onIssueDeleted, onIssueUpdated } from "../issues/ws-updaters";
import { projectIssueCandidatesOptions, projectKeys } from "./queries";

vi.mock("../api", async (original) => ({
  ...(await original<typeof import("../api")>()),
  api: { listIssues: vi.fn(), getIssue: vi.fn() },
}));
afterEach(() => vi.resetAllMocks());

function issue(number: number, overrides: Partial<Issue> = {}): Issue {
  return {
    id: `issue-${number}`, workspace_id: "w1", number, identifier: `MUL-${number}`,
    title: `Task ${number}`, description: null, status: "todo", priority: "none",
    assignee_type: null, assignee_id: null, creator_type: "member", creator_id: "u1",
    parent_issue_id: null, project_id: null, position: number, stage: null,
    start_date: null, due_date: null, metadata: {}, properties: {},
    created_at: "2026-10-11", updated_at: "2026-10-11", ...overrides,
  };
}

describe("project issue candidates", () => {
  it("browses all raw windows with server-side eligibility and no status or parent restriction", async () => {
    const rows = Array.from({ length: 65 }, (_, i) => issue(i + 1));
    rows[0] = issue(1, { status: "done", parent_issue_id: "parent" });
    rows[1] = issue(2, { status: "cancelled" });
    rows[29] = issue(30, { project_id: "already-linked" });
    rows[30] = issue(31, { workspace_id: "foreign-workspace" });
    rows[31] = issue(32, { admission_status: "pending" });
    vi.mocked(api.listIssues).mockImplementation(async (params) => ({
      issues: rows.slice(params?.offset, (params?.offset ?? 0) + 30), total: rows.length,
    }));
    const qc = new QueryClient();
    const result = await qc.fetchInfiniteQuery({ ...projectIssueCandidatesOptions("w1", ""), pages: 4 });

    expect(result.pages.flatMap((page) => page.issues)).toEqual(
      rows.filter((row) => row.workspace_id === "w1" && row.project_id === null && row.admission_status !== "pending"),
    );
    expect(vi.mocked(api.listIssues).mock.calls.map(([params]) => params?.offset)).toEqual([0, 30, 60]);
    for (const [params] of vi.mocked(api.listIssues).mock.calls) {
      expect(params).toEqual({
        workspace_id: "w1", q: "", include_no_project: true,
        limit: 30, offset: expect.any(Number), sort_by: "created_at", sort_direction: "desc",
      });
    }
    expect(result.pages.at(-1)?.nextOffset).toBeUndefined();
    qc.clear();
  });

  it("searches the same eligible population and separates normalized queries and workspaces", async () => {
    vi.mocked(api.listIssues).mockResolvedValue({ issues: [issue(1)], total: 1 });
    const qc = new QueryClient();
    const options = projectIssueCandidatesOptions("w1", " login ");
    expect(options.queryKey).toEqual(projectIssueCandidatesOptions("w1", "login").queryKey);
    expect(options.queryKey).not.toEqual(projectIssueCandidatesOptions("w2", "login").queryKey);
    expect(options.queryKey).not.toEqual(projectIssueCandidatesOptions("w1", "").queryKey);
    expect(projectIssueCandidatesOptions("", "").enabled).toBe(false);

    await qc.fetchInfiniteQuery(options);

    expect(api.listIssues).toHaveBeenCalledWith(expect.objectContaining({
      workspace_id: "w1", q: "login", include_no_project: true,
    }));
    qc.clear();
  });

  it.each(["12A-34", "a56e8429-ff4e-4eec-8743-93907c5e7469"])(
    "resolves %s exactly within the requested workspace",
    async (reference) => {
      vi.mocked(api.getIssue).mockResolvedValue(issue(34));
      const qc = new QueryClient();
      const result = await qc.fetchInfiniteQuery(projectIssueCandidatesOptions("w1", reference));

      expect(result.pages).toEqual([{ issues: [issue(34)], nextOffset: undefined }]);
      expect(api.getIssue).toHaveBeenCalledWith(reference, {
        workspaceId: "w1", signal: expect.any(AbortSignal),
      });
      expect(api.listIssues).not.toHaveBeenCalled();
      qc.clear();
    },
  );

  it.each<Partial<Issue>>([
    { project_id: "another-project" }, { workspace_id: "w2" },
    { admission_status: "pending" }, { admission_status: "rejected" },
  ])(
    "excludes ineligible exact references: %j",
    async (overrides) => {
      vi.mocked(api.getIssue).mockResolvedValue(issue(1, overrides));
      const qc = new QueryClient();
      expect((await qc.fetchInfiniteQuery(projectIssueCandidatesOptions("w1", "MUL-1"))).pages)
        .toEqual([{ issues: [], nextOffset: undefined }]);
      qc.clear();
    },
  );

  it("treats only a missing reference as empty and preserves actionable failures", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(api.getIssue).mockRejectedValueOnce(new ApiError("Not found", 404, "Not Found"));
    expect((await qc.fetchInfiniteQuery(projectIssueCandidatesOptions("w1", "MUL-1"))).pages)
      .toEqual([{ issues: [], nextOffset: undefined }]);
    for (const error of [new ApiError("Forbidden", 403, "Forbidden"), new Error("Offline")]) {
      vi.mocked(api.getIssue).mockRejectedValueOnce(error);
      await expect(qc.fetchInfiniteQuery(projectIssueCandidatesOptions("w1", "MUL-2"))).rejects.toBe(error);
    }
    qc.clear();
  });

  it("terminates an empty server page even if its total is stale", async () => {
    vi.mocked(api.listIssues).mockResolvedValue({ issues: [], total: 100 });
    const qc = new QueryClient();
    const result = await qc.fetchInfiniteQuery({ ...projectIssueCandidatesOptions("w1", ""), pages: 2 });
    expect(result.pages).toEqual([{ issues: [], nextOffset: undefined }]);
    expect(api.listIssues).toHaveBeenCalledTimes(1);
    qc.clear();
  });

  it("refreshes an open candidate list after remote project assignment and issue deletion", async () => {
    let rows = [issue(1), issue(2)];
    vi.mocked(api.listIssues).mockImplementation(async () => ({ issues: rows, total: rows.length }));
    const qc = new QueryClient();
    const observer = new InfiniteQueryObserver(qc, {
      ...projectIssueCandidatesOptions("w1", ""), staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => {});
    try {
      await observer.refetch();
      expect(observer.getCurrentResult().data?.pages.flatMap((page) => page.issues)).toEqual(rows);

      rows = [issue(1, { project_id: "project-1" }), issue(2)];
      onIssueUpdated(qc, "w1", rows[0]!, { projectChanged: true });
      await vi.waitFor(() => expect(observer.getCurrentResult().data?.pages.flatMap((page) => page.issues))
        .toEqual([issue(2)]));

      rows = [rows[0]!];
      onIssueDeleted(qc, "w1", "issue-2");
      await vi.waitFor(() => expect(observer.getCurrentResult().data?.pages.flatMap((page) => page.issues))
        .toEqual([]));
      expect(api.listIssues).toHaveBeenCalledTimes(3);
    } finally {
      unsubscribe();
      qc.clear();
    }
  });

  it("refreshes candidates for a new unassigned task without invalidating unrelated project projections", async () => {
    let rows: Issue[] = [];
    vi.mocked(api.listIssues).mockImplementation(async () => ({ issues: rows, total: rows.length }));
    const qc = new QueryClient();
    const searchKey = projectIssueCandidatesOptions("w1", "launch").queryKey;
    const otherWorkspaceKey = projectIssueCandidatesOptions("w2", "").queryKey;
    const empty = { pages: [{ issues: [], nextOffset: undefined }], pageParams: [0] };
    qc.setQueryData(projectKeys.list("w1"), []);
    qc.setQueryData(searchKey, empty);
    qc.setQueryData(otherWorkspaceKey, empty);
    const observer = new InfiniteQueryObserver(qc, {
      ...projectIssueCandidatesOptions("w1", ""), staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => {});
    try {
      await observer.refetch();
      expect(observer.getCurrentResult().data?.pages.flatMap((page) => page.issues)).toEqual([]);

      rows = [issue(3)];
      onIssueCreated(qc, "w1", rows[0]!);

      await vi.waitFor(() => expect(observer.getCurrentResult().data?.pages.flatMap((page) => page.issues))
        .toEqual(rows));
      expect(api.listIssues).toHaveBeenCalledTimes(2);
      expect(qc.getQueryState(searchKey)?.isInvalidated).toBe(true);
      expect(qc.getQueryState(projectKeys.list("w1"))?.isInvalidated).toBe(false);
      expect(qc.getQueryState(otherWorkspaceKey)?.isInvalidated).toBe(false);
    } finally {
      unsubscribe();
      qc.clear();
    }
  });
});
