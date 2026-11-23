// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "../api";
import { protectIterationRead } from "./access";
import {
  groupIterationActivity,
  iterationActivityDays,
  iterationActivityOptions,
  projectIterationEvent,
  type IterationEvent,
} from "./activity";

vi.mock("../api", async (original) => ({
  ...await original<typeof import("../api")>(),
  api: {
    getIterationEvents: vi.fn(),
    getSessionScope: vi.fn(() => "session"),
    getBaseUrl: () => "test",
  },
}));

const workspace = "10000000-0000-4000-8000-000000000001";
const iteration = "20000000-0000-4000-8000-000000000001";
const otherIteration = "20000000-0000-4000-8000-000000000002";
const issue = "30000000-0000-4000-8000-000000000001";
const agent = "40000000-0000-4000-8000-000000000001";
function event(sequence: number, overrides: Partial<IterationEvent> = {}): IterationEvent {
  return {
    id: `50000000-0000-4000-8000-${String(sequence).padStart(12, "0")}`,
    iteration_id: iteration,
    issue_id: issue,
    operation_id: `60000000-0000-4000-8000-${String(sequence).padStart(12, "0")}`,
    sequence,
    kind: "issue_changed",
    actor: { type: "member", id: workspace, user_id: workspace },
    before_facts: { title: "Before", status_key: "todo", status_category: "todo" },
    after_facts: { title: "After", status_key: "todo", status_category: "todo" },
    occurred_at: "2026-10-06T18:05:00Z",
    sampled_at: "2026-10-06T18:05:00Z",
    reason: null,
    ...overrides,
  };
}
const page = (items: IterationEvent[], next_cursor: string | null = null) => ({
  workspace_id: workspace, iteration_id: iteration, items, next_cursor,
});
const stale = () => new ApiError("Changed", 409, "Conflict", { code: "cursor_stale" });
const clients: QueryClient[] = [];
function client() {
  const value = new QueryClient();
  clients.push(value);
  return value;
}
beforeEach(() => {
  vi.mocked(api.getIterationEvents).mockReset();
  vi.mocked(api.getSessionScope).mockReset().mockReturnValue("session");
});
afterEach(() => { clients.splice(0).forEach((value) => value.clear()); });

describe("complete iteration activity reads", () => {
  it("publishes only the complete history and keeps operation groups spanning pages", async () => {
    const start = event(1, { kind: "start", issue_id: null });
    const baseline = event(2, { kind: "baseline", operation_id: start.operation_id });
    let finish!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([start], "next"))
      .mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const query = iterationActivityOptions(workspace, iteration);
    const queryClient = client();
    const request = queryClient.fetchQuery(query);
    await vi.waitFor(() => expect(api.getIterationEvents).toHaveBeenCalledTimes(2));
    expect(query.queryKey).toEqual(["iterations", workspace, "activity", iteration]);
    expect(query.retry).toBe(false);
    expect(queryClient.getQueryData(query.queryKey)).toBeUndefined();
    finish(page([baseline, event(3, { kind: "cancel" })]));
    const items = await request;
    expect(items.map((item) => item.sequence)).toEqual([1, 2, 3]);
    const groups = groupIterationActivity(items, "scope");
    expect(groups.map((group) => group.entries.map((entry) => entry.event.sequence))).toEqual([[3], [2, 1]]);
    expect(groups[1]?.kind).toBe("start");
    expect(api.getIterationEvents).toHaveBeenNthCalledWith(1, workspace, iteration, { limit: "100" }, { signal: expect.any(AbortSignal) });
    expect(api.getIterationEvents).toHaveBeenNthCalledWith(2, workspace, iteration, { limit: "100", cursor: "next" }, { signal: expect.any(AbortSignal) });
    expect(vi.mocked(api.getIterationEvents).mock.calls[0]?.[3]?.signal).toBe(vi.mocked(api.getIterationEvents).mock.calls[1]?.[3]?.signal);
  });

  it("throws away the obsolete traversal before its one stale-cursor restart", async () => {
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([event(1)], "old"))
      .mockRejectedValueOnce(stale())
      .mockResolvedValueOnce(page([event(2)], "new"))
      .mockResolvedValueOnce(page([event(3)]));
    expect((await client().fetchQuery(iterationActivityOptions(workspace, iteration))).map((item) => item.sequence)).toEqual([2, 3]);
    expect(vi.mocked(api.getIterationEvents).mock.calls.map(([, , params]) => params?.cursor)).toEqual([undefined, "old", undefined, "new"]);
  });

  it("does not let Query retries multiply stale-cursor restarts", async () => {
    vi.mocked(api.getIterationEvents).mockRejectedValue(stale());
    await expect(client().fetchQuery(iterationActivityOptions(workspace, iteration))).rejects.toMatchObject({ status: 409 });
    expect(api.getIterationEvents).toHaveBeenCalledTimes(2);
  });

  it.each(["cursor", "identity", "sequence"])("rejects repeated %s without publishing a partial history", async (duplicate) => {
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([event(1)], "next"))
      .mockResolvedValueOnce(page([
        duplicate === "identity" ? event(1, { sequence: 2 }) : duplicate === "sequence" ? event(2, { sequence: 1 }) : event(2),
      ], duplicate === "cursor" ? "next" : null));
    const queryClient = client();
    const query = iterationActivityOptions(workspace, iteration);
    await expect(queryClient.fetchQuery(query)).rejects.toThrow(/Repeated iteration activity/);
    expect(queryClient.getQueryData(query.queryKey)).toBeUndefined();
    expect(api.getIterationEvents).toHaveBeenCalledTimes(2);
  });

  it.each(["workspace", "iteration", "event"])("checks the %s identity of every page", async (mismatch) => {
    const bad = page([event(2)]);
    if (mismatch === "workspace") bad.workspace_id = "other";
    else if (mismatch === "iteration") bad.iteration_id = otherIteration;
    else bad.items[0]!.iteration_id = otherIteration;
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([event(1)], "next")).mockResolvedValueOnce(bad);
    await expect(client().fetchQuery(iterationActivityOptions(workspace, iteration))).rejects.toThrow(/identity mismatch/);
  });

  it.each([new Error("offline"), new ApiError("Busy", 429, "Too Many Requests"), stale()])("keeps the previous complete result when a refresh fails: %s", async (error) => {
    const queryClient = client();
    const query = iterationActivityOptions(workspace, iteration);
    const previous = [event(1)];
    queryClient.setQueryData(query.queryKey, previous);
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([event(2)], "next")).mockRejectedValue(error);
    await expect(queryClient.fetchQuery(query)).rejects.toThrow();
    expect(queryClient.getQueryData(query.queryKey)).toEqual(previous);
  });

  it("removes a deleted iteration's cached history without revoking another iteration", async () => {
    const queryClient = client();
    const query = iterationActivityOptions(workspace, iteration);
    queryClient.setQueryData(query.queryKey, [event(1)]);
    queryClient.setQueryData(iterationActivityOptions(workspace, otherIteration).queryKey, [event(2)]);
    vi.mocked(api.getIterationEvents).mockRejectedValue(new ApiError("Missing", 404, "Not Found", { code: "iteration_not_found" }));
    await expect(queryClient.fetchQuery(query)).rejects.toThrow("Missing");
    expect(queryClient.getQueryData(query.queryKey)).toBeUndefined();
    expect(queryClient.getQueryData(iterationActivityOptions(workspace, otherIteration).queryKey)).toEqual([event(2)]);
  });

  it("honors cancellation even if the current page ignores its AbortSignal", async () => {
    let finish!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.getIterationEvents).mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const queryClient = client();
    const query = iterationActivityOptions(workspace, iteration);
    const request = queryClient.fetchQuery(query);
    const rejected = expect(request).rejects.toBeDefined();
    await vi.waitFor(() => expect(api.getIterationEvents).toHaveBeenCalledTimes(1));
    await queryClient.cancelQueries({ queryKey: query.queryKey });
    finish(page([event(1)], "next"));
    await rejected;
    expect(vi.mocked(api.getIterationEvents).mock.calls[0]?.[3]?.signal?.aborted).toBe(true);
    expect(api.getIterationEvents).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryData(query.queryKey)).toBeUndefined();
  });

  it.each(["success", "denied"])("fences a late %s across the whole traversal after the session changes", async (outcome) => {
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([event(1)], "next"))
      .mockImplementationOnce(async () => {
        vi.mocked(api.getSessionScope).mockReturnValue("new-session");
        if (outcome === "denied") throw new ApiError("Old session denied", 403, "Forbidden");
        return page([event(2)]);
      });
    const queryClient = client();
    queryClient.setQueryData(["iterations", workspace, "detail", otherIteration], "new-account-data");
    const query = iterationActivityOptions(workspace, iteration);
    await expect(queryClient.fetchQuery(query)).rejects.toThrow("Iteration session changed");
    expect(queryClient.getQueryData(query.queryKey)).toBeUndefined();
    expect(queryClient.getQueryData(["iterations", workspace, "detail", otherIteration])).toBe("new-account-data");
    expect(queryClient.getQueryData(["iterations", workspace, "access"])).toBeUndefined();
  });

  it("does not restore activity after another protected read revokes access", async () => {
    const queryClient = client();
    let finish!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([event(1)], "next"))
      .mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const query = iterationActivityOptions(workspace, iteration);
    queryClient.setQueryData(query.queryKey, [event(4)]);
    const request = queryClient.fetchQuery(query);
    const rejected = expect(request).rejects.toBeDefined();
    await vi.waitFor(() => expect(api.getIterationEvents).toHaveBeenCalledTimes(2));
    await expect(protectIterationRead(queryClient, workspace, async () => {
      throw new ApiError("Access removed", 403, "Forbidden");
    })).rejects.toThrow();
    finish(page([event(2)]));
    await rejected;
    expect(queryClient.getQueryData(query.queryKey)).toBeUndefined();
  });
});

describe("stored iteration activity semantics", () => {
  it.each([
    ["cancel", "todo", "cancelled", true],
    ["reopen", "cancelled", "todo", true],
    ["reopen", "done", "todo", false],
    ["reopen", undefined, "todo", false],
    ["status", "cancelled", "in_progress", true],
    ["issue_changed", "todo", "cancelled", true],
    ["planned_activity", "cancelled", "todo", true],
    ["status", "todo", "done", false],
    ["issue_changed", "todo", "todo", false],
  ])("classifies %s from saved %s → %s facts as scope-related=%s", (kind, before, after, expected) => {
    const projection = projectIterationEvent(event(1, {
      kind,
      before_facts: { status_category: before },
      after_facts: { status_category: after },
    }));
    expect(projection.scopeRelated).toBe(expected);
  });

  it("distinguishes planned membership changes from ordinary planned edits", () => {
    const joined = projectIterationEvent(event(1, { kind: "planned_activity", before_facts: null, after_facts: { source_iteration_id: null, target_iteration_id: iteration } }));
    expect(joined).toMatchObject({ kind: "join", scopeRelated: true, movement: { source: null, target: iteration } });
    expect(joined.changes).toEqual([]);
    const left = projectIterationEvent(event(2, { kind: "planned_activity", before_facts: { source_iteration_id: iteration, target_iteration_id: null }, after_facts: null }));
    expect(left).toMatchObject({ kind: "leave", scopeRelated: true, movement: { source: iteration, target: null } });
    expect(projectIterationEvent(event(3, { kind: "planned_activity" }))).toMatchObject({ kind: "plan", scopeRelated: false });
  });

  it("omits inapplicable movement but preserves unknown and explicit null on actual moves", () => {
    expect(projectIterationEvent(event(1)).movement).toBeUndefined();
    expect(projectIterationEvent(event(1, { kind: "rollover", issue_id: null, before_facts: null, after_facts: null })).movement).toBeUndefined();
    const legacy = projectIterationEvent(event(2, { kind: "join", before_facts: null, after_facts: { target_iteration_id: null } }));
    expect(legacy.movement).toEqual({ source: undefined, target: null });
    const invalid = projectIterationEvent(event(3, { kind: "leave", before_facts: { source_iteration_id: "invalid" }, after_facts: null }));
    expect(invalid.movement).toEqual({ source: undefined, target: undefined });
  });

  it("projects readable frozen identity, status, assignee and date differences without current records", () => {
    const raw = event(1, {
      actor: { type: "agent", id: agent, user_id: workspace },
      before_facts: { title: "Original title", status_key: "review_old", status_category: "in_review", assignee_type: "member", assignee_id: workspace, assignee_name: "Avery", start_date: "2026-10-01", end_date: "2026-10-14" },
      after_facts: { title: "Frozen title", identifier: "OLD-42", status_key: "done", status_category: "done", assignee_type: "agent", assignee_id: agent, assignee_name: "Builder", start_date: "2026-10-02", end_date: "2026-10-14" },
    });
    const projection = projectIterationEvent(raw);
    expect(projection.issue).toEqual({ id: issue, title: "Frozen title", identifier: "OLD-42" });
    expect(projection.actor.id).toBe(agent);
    expect(projection.changes).toEqual([
      { field: "title", before: "Original title", after: "Frozen title" },
      { field: "status", before: { key: "review_old", category: "in_review" }, after: { key: "done", category: "done" } },
      { field: "assignee", before: { type: "member", id: workspace, name: "Avery" }, after: { type: "agent", id: agent, name: "Builder" } },
      { field: "startDate", before: "2026-10-01", after: "2026-10-02" },
    ]);
    expect(projection.event).toBe(raw);
  });

  it("keeps missing values different from explicit clearing and malformed facts", () => {
    const projection = projectIterationEvent(event(1, {
      before_facts: { title: 1, assignee_type: "member", assignee_id: workspace },
      after_facts: { title: false, assignee_type: null, assignee_id: null, description: null },
    }));
    expect(projection.issue).toEqual({ id: issue, title: undefined, identifier: undefined });
    expect(projection.changes).toContainEqual({ field: "description", before: undefined, after: null });
    expect(projection.changes).toContainEqual({ field: "assignee", before: { type: "member", id: workspace, name: undefined }, after: { type: null, id: null, name: undefined } });
  });

  it("keeps every original event within its operation and orders by sequence, not timestamps", () => {
    const start = event(1, { kind: "start", issue_id: null, occurred_at: "2026-10-07T00:00:00Z" });
    const baseline = event(2, { kind: "baseline", operation_id: start.operation_id });
    const edit = event(3, { occurred_at: "2026-10-05T00:00:00Z" });
    const cancellation = event(4, { kind: "cancel", operation_id: edit.operation_id });
    const unknown = event(5, { kind: "future_kind" });
    const all = groupIterationActivity([baseline, unknown, edit, start, cancellation], "all");
    expect(all.map((group) => group.entries.map((entry) => entry.event.sequence))).toEqual([[5], [4, 3], [2, 1]]);
    expect(all[0]?.entries[0]).toMatchObject({ kind: "unknown", event: unknown });
    expect(all[1]?.issueCount).toBe(1);
    expect(all[2]?.kind).toBe("start");
    expect(groupIterationActivity([baseline, unknown, edit, start, cancellation], "scope").flatMap((group) => group.entries).map((entry) => entry.event.sequence)).toEqual([4, 3, 2, 1]);
  });

  it("groups by the saved timezone's days without overriding authoritative sequence order", () => {
    const groups = groupIterationActivity([
      event(1, { occurred_at: "2026-10-06T15:00:00Z" }),
      event(2, { occurred_at: "2026-10-06T18:00:00Z" }),
      event(3, { occurred_at: "2026-10-06T14:00:00Z" }),
    ]);
    expect(iterationActivityDays(groups, "Asia/Shanghai").map((day) => ({ date: day.date, sequences: day.groups.map((group) => group.sequence) }))).toEqual([
      { date: "2026-10-06", sequences: [3] },
      { date: "2026-10-07", sequences: [2] },
      { date: "2026-10-06", sequences: [1] },
    ]);
    expect(iterationActivityDays(groups, "UTC")).toHaveLength(1);
  });

  it("retains a day's identity when a refresh prepends newer operations on the same date", () => {
    const before = iterationActivityDays(groupIterationActivity([event(1)]), "Asia/Shanghai");
    const after = iterationActivityDays(groupIterationActivity([event(1), event(2)]), "Asia/Shanghai");
    expect(after[0]?.id).toBe(before[0]?.id);
  });
});
