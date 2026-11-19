/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider, QueryObserver, useQuery, isCancelledError } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { ApiError, setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { createQueryClient } from "../query-client";
import { useCanonicalIssue } from "./canonical-id";
import {
  useBatchUpdateIssues,
  useCreateComment,
  useCreateCommentSubIssue,
  useDeleteComment,
  useResolveComment,
  useUpdateComment,
  useUpdateIssue,
} from "./mutations";
import {
  issueKeys,
  issueDetailOptions,
  type IssueSortParam,
} from "./queries";
import { onIssueUpdated, onIssueAuxiliaryRevision } from "./ws-updaters";
import { inboxKeys } from "../inbox/queries";
import type {
  InboxItem,
  Issue,
  ListIssuesCache,
  TimelineEntry,
} from "../types";

vi.mock("../hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

const WS_ID = "ws-1";

function makeIssue(idx: number, overrides: Partial<Issue> = {}): Issue {
  return {
    id: `issue-${idx}`,
    workspace_id: WS_ID,
    number: idx,
    identifier: `MUL-${idx}`,
    title: `Issue ${idx}`,
    description: null,
    status: "todo",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: idx,
    stage: null,
    start_date: null,
    due_date: null,
    labels: [],
    metadata: {},
  properties: {},
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

function makeInboxItem(
  id: string,
  issueId: string,
  overrides: Partial<InboxItem> = {},
): InboxItem {
  return {
    id,
    workspace_id: WS_ID,
    recipient_type: "member",
    recipient_id: "user-1",
    actor_type: "member",
    actor_id: "user-2",
    type: "status_changed",
    severity: "info",
    issue_id: issueId,
    title: `Inbox ${id}`,
    body: null,
    issue_status: "todo",
    read: false,
    archived: false,
    created_at: "2025-01-01T00:00:00Z",
    details: null,
    ...overrides,
  };
}

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

describe("useUpdateIssue — canonical detail reads during an optimistic status change", () => {
  const issueId = "cb240efb-154c-42a8-ae92-42b02676feca";
  const identifier = "MUL-134";
  const detailKey = issueKeys.detail(WS_ID, issueId);
  let qc: QueryClient;

  function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (error: Error) => void;
    const promise = new Promise<T>((done, fail) => {
      resolve = done;
      reject = fail;
    });
    return { promise, resolve, reject };
  }

  beforeEach(() => {
    qc = createQueryClient();
    qc.setDefaultOptions({
      ...qc.getDefaultOptions(),
      queries: { ...qc.getDefaultOptions().queries, retry: false },
    });
  });

  afterEach(() => {
    qc.clear();
    vi.restoreAllMocks();
  });

  it.each([
    { entry: "UUID", routeId: issueId, overlap: true },
    { entry: "identifier", routeId: identifier, overlap: true },
    { entry: "UUID without a background read", routeId: issueId, overlap: false },
    { entry: "identifier without a background read", routeId: identifier, overlap: false },
  ])("preserves the pending status, rollback and explicit retry from $entry", async ({ routeId, overlap }) => {
    const todo = makeIssue(134, { id: issueId, identifier, revision: 1, status_category: "todo" });
    let serverIssue = todo;
    let holdDetailRead = false;
    const oldRead = deferred<Issue>();
    const firstWrite = deferred<Issue>();
    const failure = new ApiError("Status update unavailable", 503, "Service Unavailable");
    const getIssue = vi.fn((id: string) => {
      if (id !== issueId && id !== identifier) throw new Error(`Unexpected issue read: ${id}`);
      return holdDetailRead && id === issueId ? oldRead.promise : Promise.resolve(serverIssue);
    });
    const updateIssue = vi.fn()
      .mockImplementationOnce(() => firstWrite.promise)
      .mockImplementationOnce(() => {
        serverIssue = { ...todo, status: "in_progress", status_category: "in_progress", revision: 2 };
        return Promise.resolve(serverIssue);
      });
    setApiInstance({ getIssue, updateIssue } as unknown as ApiClient);
    const { result } = renderHook(() => {
      const canonical = useCanonicalIssue(WS_ID, routeId);
      const reader = useQuery({
        ...issueDetailOptions(WS_ID, canonical.canonicalId ?? ""),
        enabled: canonical.canonicalId !== null,
      });
      return { canonical, reader, mutation: useUpdateIssue() };
    }, { wrapper: createWrapper(qc) });
    const events: unknown[] = [];
    function record(stage: string) {
      events.push({
        stage,
        canonicalId: result.current.canonical.canonicalId,
        readerKey: issueKeys.detail(WS_ID, result.current.canonical.canonicalId ?? ""),
        writerKey: detailKey,
        cacheStatus: qc.getQueryData<Issue>(detailKey)?.status,
        canonicalStatus: result.current.canonical.issue?.status,
        readerStatus: result.current.reader.data?.status,
        mutationStatus: result.current.mutation.status,
      });
    }
    let backgroundRead: Promise<void> | undefined;
    try {
      await waitFor(() => expect(result.current.reader.data?.status).toBe("todo"));
      expect(result.current.canonical.canonicalId).toBe(issueId);
      expect(getIssue).toHaveBeenCalledTimes(1);
      record("initial Todo loaded");

      if (overlap) {
        holdDetailRead = true;
        act(() => {
          backgroundRead = qc.refetchQueries({ queryKey: detailKey, exact: true });
        });
        await waitFor(() => expect(getIssue).toHaveBeenCalledTimes(2));
        expect(qc.getQueryState(detailKey)?.fetchStatus).toBe("fetching");
        record("background UUID read held");
      }

      act(() => { result.current.mutation.mutate({ id: issueId, status: "in_progress" }); });
      expect(qc.getQueryData<Issue>(detailKey)?.status).toBe("in_progress");
      await waitFor(() => expect(updateIssue).toHaveBeenCalledTimes(1));
      expect(updateIssue).toHaveBeenCalledWith(issueId, { status: "in_progress" });
      await waitFor(() => expect(result.current.reader.data?.status).toBe("in_progress"));
      expect(serverIssue.status).toBe("todo");
      record("first PUT held with optimistic status");

      if (overlap) {
        holdDetailRead = false;
        await act(async () => {
          oldRead.resolve(todo);
          await backgroundRead;
        });
        await waitFor(() => expect(result.current.reader.fetchStatus).toBe("idle"));
        record("old Todo read completed while PUT pending");
        expect(qc.getQueryData<Issue>(detailKey)?.status, JSON.stringify(events)).toBe("in_progress");
        expect(result.current.canonical.issue?.status).toBe("in_progress");
        expect(result.current.reader.data?.status).toBe("in_progress");
        expect(result.current.mutation.isPending).toBe(true);
        expect(serverIssue.status).toBe("todo");
      }

      await act(async () => { firstWrite.reject(failure); });
      await waitFor(() => expect(result.current.mutation.error).toBe(failure));
      await waitFor(() => expect(result.current.reader.data?.status).toBe("todo"));
      expect(result.current.canonical.issue?.status).toBe("todo");
      expect(qc.getQueryData<Issue>(detailKey)?.status).toBe("todo");
      expect(serverIssue.status).toBe("todo");
      record("503 rolled back to Todo");

      act(() => { result.current.mutation.mutate({ id: issueId, status: "in_progress" }); });
      await waitFor(() => expect(result.current.mutation.isSuccess).toBe(true));
      expect(updateIssue).toHaveBeenCalledTimes(2);
      expect(serverIssue.status).toBe("in_progress");
      expect(result.current.reader.data?.status).toBe("in_progress");
      await act(async () => { await qc.refetchQueries({ queryKey: detailKey, exact: true }); });
      expect(result.current.canonical.issue?.status).toBe("in_progress");
      expect(result.current.reader.data?.status).toBe("in_progress");
    } finally {
      holdDetailRead = false;
      await act(async () => {
        oldRead.resolve(todo);
        firstWrite.reject(failure);
        await backgroundRead;
      });
    }
  });

  it("preserves a visible identifier seed when the UUID fetch began without data", async () => {
    const todo = makeIssue(134, { id: issueId, identifier, revision: 1, status_category: "todo" });
    let serverIssue = todo;
    let holdUuidRead = true;
    const oldRead = deferred<Issue>();
    const firstWrite = deferred<Issue>();
    const failure = new ApiError("Status update unavailable", 503, "Service Unavailable");
    const getIssue = vi.fn((id: string) => {
      if (id === issueId && holdUuidRead) return oldRead.promise;
      if (id === identifier || id === issueId) return Promise.resolve(serverIssue);
      throw new Error(`Unexpected issue read: ${id}`);
    });
    const updateIssue = vi.fn()
      .mockImplementationOnce(() => firstWrite.promise)
      .mockImplementationOnce(() => {
        serverIssue = { ...todo, status: "in_progress", status_category: "in_progress", revision: 2 };
        return Promise.resolve(serverIssue);
      });
    setApiInstance({ getIssue, updateIssue } as unknown as ApiClient);
    const events: unknown[] = [];
    const unsubscribe = qc.getQueryCache().subscribe((event) => {
      if (event.type !== "updated" || event.query.queryHash !== JSON.stringify(detailKey)) return;
      events.push({
        action: event.action.type,
        cacheStatus: qc.getQueryData<Issue>(detailKey)?.status ?? null,
        fetchStatus: event.query.state.fetchStatus,
      });
    });
    // A UUID observer can start before identifier resolution supplies its
    // initialData. The visible Todo then differs from Query's fetch rollback
    // state, which still predates that seed.
    const pendingRead = qc.fetchQuery(issueDetailOptions(WS_ID, issueId)).catch((error) => {
      if (!isCancelledError(error)) throw error;
    });
    expect(qc.getQueryData(detailKey)).toBeUndefined();
    expect(qc.getQueryState(detailKey)?.fetchStatus).toBe("fetching");
    const { result } = renderHook(() => {
      const canonical = useCanonicalIssue(WS_ID, identifier);
      const reader = useQuery({
        ...issueDetailOptions(WS_ID, canonical.canonicalId ?? ""),
        enabled: canonical.canonicalId !== null,
      });
      return { canonical, reader, mutation: useUpdateIssue() };
    }, { wrapper: createWrapper(qc) });
    try {
      await waitFor(() => expect(result.current.reader.data?.status).toBe("todo"));
      expect(result.current.canonical.canonicalId).toBe(issueId);
      expect(result.current.canonical.issue?.status).toBe("todo");
      expect(qc.getQueryData<Issue>(detailKey)?.status).toBe("todo");
      expect(qc.getQueryState(detailKey)?.fetchStatus).toBe("fetching");

      act(() => { result.current.mutation.mutate({ id: issueId, status: "in_progress" }); });
      await waitFor(() => expect(updateIssue).toHaveBeenCalledTimes(1));
      expect(qc.getQueryData<Issue>(detailKey)?.status, JSON.stringify(events)).toBe("in_progress");
      expect(result.current.canonical.issue?.status).toBe("in_progress");
      expect(result.current.reader.data?.status).toBe("in_progress");
      expect(result.current.mutation.isPending).toBe(true);
      expect(serverIssue.status).toBe("todo");

      holdUuidRead = false;
      await act(async () => { oldRead.resolve(todo); await pendingRead; });
      expect(result.current.reader.data?.status).toBe("in_progress");
      await act(async () => { firstWrite.reject(failure); });
      await waitFor(() => expect(result.current.mutation.error).toBe(failure));
      await waitFor(() => expect(result.current.reader.data?.status).toBe("todo"));
      expect(serverIssue.status).toBe("todo");

      act(() => { result.current.mutation.mutate({ id: issueId, status: "in_progress" }); });
      await waitFor(() => expect(result.current.mutation.isSuccess).toBe(true));
      expect(updateIssue).toHaveBeenCalledTimes(2);
      expect(serverIssue.status).toBe("in_progress");
      await act(async () => { await qc.refetchQueries({ queryKey: detailKey, exact: true }); });
      expect(result.current.canonical.issue?.status).toBe("in_progress");
      expect(result.current.reader.data?.status).toBe("in_progress");
    } finally {
      holdUuidRead = false;
      await act(async () => {
        oldRead.resolve(todo);
        firstWrite.reject(failure);
        await pendingRead.catch(() => undefined);
      });
      unsubscribe();
    }
  });

  it("allows other issue and workspace detail reads to finish during the write", async () => {
    const todo = makeIssue(134, { id: issueId, identifier, revision: 1 });
    const other = makeIssue(135, { id: "536b1a70-54bf-4b47-9546-0565683a6993", revision: 1 });
    const otherWorkspaceKey = issueKeys.detail("ws-2", issueId);
    const otherIssueKey = issueKeys.detail(WS_ID, other.id);
    const otherWorkspaceRead = deferred<Issue>();
    const otherIssueRead = deferred<Issue>();
    const write = deferred<Issue>();
    qc.setQueryData(detailKey, todo);
    qc.setQueryData(otherWorkspaceKey, { ...todo, workspace_id: "ws-2" });
    qc.setQueryData(otherIssueKey, other);
    const updateIssue = vi.fn(() => write.promise);
    const getIssue = vi.fn((id: string) => id === other.id ? otherIssueRead.promise : otherWorkspaceRead.promise);
    setApiInstance({ getIssue, updateIssue } as unknown as ApiClient);
    const workspaceRead = qc.fetchQuery({ ...issueDetailOptions("ws-2", issueId), staleTime: 0 });
    const issueRead = qc.fetchQuery({ ...issueDetailOptions(WS_ID, other.id), staleTime: 0 });
    const { result } = renderHook(() => useUpdateIssue(), { wrapper: createWrapper(qc) });
    try {
      act(() => { result.current.mutate({ id: issueId, status: "in_progress" }); });
      await waitFor(() => expect(updateIssue).toHaveBeenCalledTimes(1));
      await act(async () => {
        otherWorkspaceRead.resolve({ ...todo, workspace_id: "ws-2", title: "Other workspace", revision: 2 });
        otherIssueRead.resolve({ ...other, title: "Other issue", revision: 2 });
        await Promise.all([workspaceRead, issueRead]);
      });
      expect(qc.getQueryData<Issue>(otherWorkspaceKey)?.title).toBe("Other workspace");
      expect(qc.getQueryData<Issue>(otherIssueKey)?.title).toBe("Other issue");
      expect(qc.getQueryData<Issue>(detailKey)?.status).toBe("in_progress");
      expect(result.current.isPending).toBe(true);
    } finally {
      await act(async () => { write.resolve({ ...todo, status: "in_progress", revision: 2 }); });
    }
  });

  it("waits for bound completion and preserves Todo after authorization refusal", async () => {
    const todo = makeIssue(134, { id: issueId, identifier, revision: 1 });
    const submission = deferred<{ issue: Issue }>();
    const completion = {
      authorization_id: "179a6b7d-ce71-46ca-9788-d58c039d7767",
      operation_id: "281a6b7d-ce71-46ca-9788-d58c039d7767",
      expected_issue_revision: 1,
    };
    const failure = new ApiError("Completion authorization refused", 403, "Forbidden");
    const submitDeliveryCompletion = vi.fn(() => submission.promise);
    const updateIssue = vi.fn();
    const getIssue = vi.fn(async () => todo);
    setApiInstance({ submitDeliveryCompletion, updateIssue, getIssue } as unknown as ApiClient);
    qc.setQueryData(detailKey, todo);
    const { result } = renderHook(() => useUpdateIssue(), { wrapper: createWrapper(qc) });
    act(() => { result.current.mutate({ id: issueId, status: "done", completion }); });
    await waitFor(() => expect(submitDeliveryCompletion).toHaveBeenCalledWith(
      issueId, { status: "done", completion }, { workspaceId: WS_ID },
    ));
    expect(qc.getQueryData<Issue>(detailKey)?.status).toBe("todo");
    expect(result.current.isPending).toBe(true);
    expect(updateIssue).not.toHaveBeenCalled();
    await act(async () => { submission.reject(failure); });
    await waitFor(() => expect(result.current.error).toBe(failure));
    expect(qc.getQueryData<Issue>(detailKey)?.status).toBe("todo");
    expect(updateIssue).not.toHaveBeenCalled();
  });
});

describe("useCreateCommentSubIssue", () => {
  it("applies the normal issue-create cache coordination", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const listKey = issueKeys.list(WS_ID);
    qc.setQueryData<ListIssuesCache>(listKey, {
      byStatus: { todo: { issues: [], total: 0 } },
    });
    const child = makeIssue(2, { parent_issue_id: "issue-1" });
    const createCommentSubIssue = vi.fn().mockResolvedValue(child);
    setApiInstance({ createCommentSubIssue } as unknown as ApiClient);
    const { result } = renderHook(() => useCreateCommentSubIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({
        anchorCommentId: "comment-1",
        data: {
          mode: "manual",
          capture_token: "sha256:capture",
          issue: { title: "Child" },
        },
      });
    });

    expect(createCommentSubIssue).toHaveBeenCalledWith("comment-1", {
      mode: "manual",
      capture_token: "sha256:capture",
      issue: { title: "Child" },
    });
    expect(
      qc.getQueryData<ListIssuesCache>(listKey)?.byStatus.todo?.issues,
    ).toContainEqual(child);
    expect(qc.getQueryState(listKey)?.isInvalidated).toBe(true);
    qc.clear();
  });
});

describe("useUpdateIssue — optimistic move keeps every bucketed board in sync", () => {
  const sort: IssueSortParam = { sort_by: "position", sort_direction: undefined };
  const myScope = "assigned";
  const myFilter = { assignee_id: "user-1" };
  const projectScope = "project:p1";
  const projectFilter = { project_id: "p1" };
  const wsKey = issueKeys.listSorted(WS_ID, sort);
  const inboxKey = inboxKeys.list(WS_ID);
  // My-Issues AND the Project board both ride this myList cache; a move that
  // only patched the workspace cache snaps back on those boards.
  const myKey = issueKeys.myListSorted(WS_ID, myScope, myFilter, sort);
  const projectKey = issueKeys.myListSorted(WS_ID, projectScope, projectFilter, sort);

  let qc: QueryClient;
  let updateIssue: ReturnType<typeof vi.fn<(id: string, data: unknown) => Promise<Issue>>>;
  let moveIssue: ReturnType<typeof vi.fn<(id: string, data: unknown) => Promise<Issue>>>;

  function makeBucketed(): ListIssuesCache {
    return {
      byStatus: {
        todo: { issues: [makeIssue(1)], total: 1 },
        in_progress: { issues: [], total: 0 },
      },
    };
  }

  function bucketIds(
    key: readonly unknown[],
    status: "todo" | "in_progress",
  ): string[] {
    const c = qc.getQueryData<ListIssuesCache>(key);
    return (c?.byStatus[status]?.issues ?? []).map((i) => i.id);
  }

  function inboxStatus(issueId: string) {
    return qc
      .getQueryData<InboxItem[]>(inboxKey)
      ?.find((item) => item.issue_id === issueId)?.issue_status;
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    updateIssue = vi.fn();
    moveIssue = vi.fn();
    setApiInstance({ updateIssue, moveIssue } as unknown as ApiClient);
    qc.setQueryData<ListIssuesCache>(wsKey, makeBucketed());
    qc.setQueryData<ListIssuesCache>(myKey, makeBucketed());
    qc.setQueryData<ListIssuesCache>(projectKey, makeBucketed());
    qc.setQueryData<InboxItem[]>(inboxKey, [
      makeInboxItem("inbox-1", "issue-1"),
      makeInboxItem("inbox-2", "issue-2"),
    ]);
  });

  afterEach(() => {
    qc.clear();
    vi.restoreAllMocks();
  });

  it("optimistically moves the card in both the workspace and myList caches", async () => {
    let resolve!: (issue: Issue) => void;
    updateIssue.mockReturnValue(
      new Promise<Issue>((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ id: "issue-1", status: "in_progress", position: 5 });
    });

    // Optimistic state — the regression: myList must move too, not just ws.
    for (const key of [wsKey, myKey, projectKey]) {
      expect(bucketIds(key, "todo")).toEqual([]);
      expect(bucketIds(key, "in_progress")).toEqual(["issue-1"]);
    }

    await act(async () => {
      resolve(makeIssue(1, { status: "in_progress", position: 5 }));
    });

    // Authoritative settle keeps the card in place in both caches.
    for (const key of [wsKey, myKey, projectKey]) {
      expect(bucketIds(key, "in_progress")).toEqual(["issue-1"]);
    }
  });

  it("does not couple a field update to the cached aggregate revision", async () => {
    qc.setQueryData(
      issueKeys.detail(WS_ID, "issue-1"),
      makeIssue(1, { revision: 6 }),
    );
    updateIssue.mockResolvedValue(makeIssue(1, { title: "Renamed", revision: 7 }));
    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ id: "issue-1", title: "Renamed" });
    });

    expect(updateIssue).toHaveBeenCalledWith("issue-1", {
      title: "Renamed",
    });
  });

  it("forwards an observed field token unchanged and rolls back a refused optimistic field", async () => {
    const detailKey = issueKeys.detail(WS_ID, "issue-1");
    qc.setQueryData<Issue>(detailKey, makeIssue(1, { revision: 6 }));
    let reject!: (error: ApiError) => void;
    updateIssue.mockReturnValue(new Promise<Issue>((_resolve, fail) => { reject = fail; }));
    const { result } = renderHook(() => useUpdateIssue(), { wrapper: createWrapper(qc) });

    act(() => result.current.mutate({ id: "issue-1", priority: "high", expected_revision: 5 }));
    await waitFor(() => expect(updateIssue).toHaveBeenCalledWith("issue-1", {
      priority: "high",
      expected_revision: 5,
    }));
    expect(qc.getQueryData<Issue>(detailKey)).toMatchObject({ priority: "high", revision: 6 });
    expect(qc.getQueryData(detailKey)).not.toHaveProperty("expected_revision");

    await act(async () => { reject(new ApiError("Revision conflict", 409, "Conflict")); });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(qc.getQueryData<Issue>(detailKey)).toMatchObject({ priority: "none", revision: 6 });
    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(updateIssue).toHaveBeenCalledOnce();

    updateIssue.mockResolvedValue(makeIssue(1, { priority: "high", revision: 7 }));
    await act(async () => {
      await result.current.mutateAsync({ id: "issue-1", priority: "high", expected_revision: 6 });
    });
    expect(updateIssue).toHaveBeenLastCalledWith("issue-1", { priority: "high", expected_revision: 6 });
    expect(qc.getQueryData<Issue>(detailKey)).toMatchObject({ priority: "high", revision: 7 });
    expect(qc.getQueryData(detailKey)).not.toHaveProperty("expected_revision");
  });

  it("does not let an older successful response overwrite a newer WS revision", async () => {
    let resolve!: (issue: Issue) => void;
    updateIssue.mockReturnValue(
      new Promise<Issue>((done) => {
        resolve = done;
      }),
    );
    const detailKey = issueKeys.detail(WS_ID, "issue-1");
    qc.setQueryData<Issue>(detailKey, makeIssue(1, { revision: 1 }));
    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ id: "issue-1", title: "local" });
    });
    await waitFor(() => expect(updateIssue).toHaveBeenCalled());
    onIssueUpdated(
      qc,
      WS_ID,
      makeIssue(1, { title: "newer remote", revision: 3 }),
    );

    await act(async () => {
      resolve(makeIssue(1, { title: "older success", revision: 2 }));
    });

    expect(qc.getQueryData<Issue>(detailKey)).toMatchObject({
      title: "newer remote",
      revision: 3,
    });
  });

  it("keeps a full response admissible after a newer revision-only response", async () => {
    let resolve!: (issue: Issue) => void;
    updateIssue.mockReturnValue(
      new Promise<Issue>((done) => {
        resolve = done;
      }),
    );
    const detailKey = issueKeys.detail(WS_ID, "issue-1");
    qc.setQueryData<Issue>(detailKey, makeIssue(1, { title: "A", revision: 1 }));
    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ id: "issue-1", title: "local" });
    });
    await waitFor(() => expect(updateIssue).toHaveBeenCalled());
    onIssueAuxiliaryRevision(qc, WS_ID, "issue-1", 3);

    await act(async () => {
      resolve(makeIssue(1, { title: "B", revision: 2 }));
    });

    expect(qc.getQueryData<Issue>(detailKey)).toMatchObject({
      title: "B",
      revision: 2,
    });
    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
  });

  it("keeps the authoritative description base while a description update is pending", async () => {
    let resolve!: (issue: Issue) => void;
    updateIssue.mockReturnValue(
      new Promise<Issue>((r) => {
        resolve = r;
      }),
    );
    const detailKey = issueKeys.detail(WS_ID, "issue-1");
    qc.setQueryData<Issue>(detailKey, makeIssue(1, { description: "base" }));
    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({
        id: "issue-1",
        description: "local edit",
        description_base: "base",
      });
    });

    await waitFor(() => {
      expect(updateIssue).toHaveBeenCalledWith("issue-1", {
        description: "local edit",
        description_base: "base",
      });
    });
    const optimistic = qc.getQueryData<Issue & { description_base?: string }>(detailKey);
    expect(optimistic?.description).toBe("base");
    expect(optimistic).not.toHaveProperty("description_base");

    await act(async () => {
      resolve(makeIssue(1, { description: "local edit" }));
    });
    expect(qc.getQueryData<Issue>(detailKey)?.description).toBe("local edit");
  });

  it("uses server move intent while keeping provisional position optimistic-only", async () => {
    moveIssue.mockResolvedValue(
      makeIssue(1, { status: "in_progress", position: 15 }),
    );
    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({
        id: "issue-1",
        status: "in_progress",
        position: 12,
        move_intent: {
          before_id: "issue-0",
          after_id: "issue-2",
        },
      });
    });

    expect(moveIssue).toHaveBeenCalledWith("issue-1", {
      status: "in_progress",
      before_id: "issue-0",
      after_id: "issue-2",
    });
    expect(updateIssue).not.toHaveBeenCalled();
    expect(
      qc
        .getQueryData<ListIssuesCache>(wsKey)
        ?.byStatus.in_progress?.issues[0]?.position,
    ).toBe(15);
  });

  it("optimistically patches the linked inbox row status and reconciles with the server response", async () => {
    let resolve!: (issue: Issue) => void;
    updateIssue.mockReturnValue(
      new Promise<Issue>((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ id: "issue-1", status: "in_progress" });
    });

    expect(inboxStatus("issue-1")).toBe("in_progress");
    expect(inboxStatus("issue-2")).toBe("todo");

    await act(async () => {
      resolve(makeIssue(1, { status: "done" }));
    });

    expect(inboxStatus("issue-1")).toBe("done");
    expect(inboxStatus("issue-2")).toBe("todo");
  });

  it("rolls both caches back when the request fails", async () => {
    updateIssue.mockRejectedValue(new Error("boom"));
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current
        .mutateAsync({ id: "issue-1", status: "in_progress", position: 5 })
        .catch(() => {});
    });

    for (const key of [wsKey, myKey, projectKey]) {
      expect(bucketIds(key, "todo")).toEqual(["issue-1"]);
      expect(bucketIds(key, "in_progress")).toEqual([]);
    }
    const invalidatedKeys = invalidateSpy.mock.calls.map((c) => c[0]?.queryKey);
    expect(invalidatedKeys).toContainEqual(issueKeys.detail(WS_ID, "issue-1"));
    expect(invalidatedKeys).toContainEqual(issueKeys.list(WS_ID));
    expect(invalidatedKeys).toContainEqual(issueKeys.myAll(WS_ID));
    expect(invalidatedKeys).toContainEqual(issueKeys.flatAll(WS_ID));
    expect(invalidatedKeys).toContainEqual(issueKeys.tableAll(WS_ID));
  });

  it("rolls the linked inbox row status back when the request fails", async () => {
    updateIssue.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current
        .mutateAsync({ id: "issue-1", status: "in_progress" })
        .catch(() => {});
    });

    expect(inboxStatus("issue-1")).toBe("todo");
    expect(inboxStatus("issue-2")).toBe("todo");
  });

  it("does not invalidate the board list on settle (no refetch flicker)", async () => {
    updateIssue.mockResolvedValue(makeIssue(1, { status: "in_progress", position: 5 }));
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ id: "issue-1", status: "in_progress", position: 5 });
    });

    const invalidatedKeys = invalidateSpy.mock.calls.map((c) => c[0]?.queryKey);
    // The board list + myList are reconciled surgically, never refetched.
    expect(invalidatedKeys).not.toContainEqual(issueKeys.list(WS_ID));
    expect(invalidatedKeys).not.toContainEqual(issueKeys.myAll(WS_ID));
  });

  it("surgically removes the issue from the old project's list on a project move (no blanket myAll refetch)", async () => {
    // A project move makes the issue leave the old project's filtered list.
    // The membership-aware coordinator removes the card from that loaded list
    // in onMutate — deterministic, no WS echo or refetch needed — replacing
    // the old blanket "invalidate myAll on settle" safety net (MUL-3669 /
    // #4548). Lists whose filter the move cannot affect stay untouched.
    let resolve!: (issue: Issue) => void;
    updateIssue.mockReturnValue(
      new Promise<Issue>((r) => {
        resolve = r;
      }),
    );
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ id: "issue-1", project_id: "project-9" });
    });

    // Optimistic: gone from the old project's list immediately; the
    // workspace board and the assignee-filtered list keep the card.
    expect(bucketIds(projectKey, "todo")).toEqual([]);
    expect(bucketIds(wsKey, "todo")).toEqual(["issue-1"]);
    expect(bucketIds(myKey, "todo")).toEqual(["issue-1"]);

    await act(async () => {
      resolve(makeIssue(1, { project_id: "project-9" }));
    });

    expect(bucketIds(projectKey, "todo")).toEqual([]);
    const invalidatedKeys = invalidateSpy.mock.calls.map((c) => c[0]?.queryKey);
    expect(invalidatedKeys).not.toContainEqual(issueKeys.myAll(WS_ID));
  });

  it("rolls the membership removal back when a project move fails", async () => {
    updateIssue.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current
        .mutateAsync({ id: "issue-1", project_id: "project-9" })
        .catch(() => {});
    });

    expect(bucketIds(projectKey, "todo")).toEqual(["issue-1"]);
  });
});

describe("useUpdateIssue — detaching a sub-issue prunes the old parent's children cache", () => {
  const PARENT_ID = "parent-1";
  const childKey = issueKeys.children(WS_ID, PARENT_ID);

  let qc: QueryClient;
  let updateIssue: ReturnType<typeof vi.fn<(id: string, data: unknown) => Promise<Issue>>>;

  function childIds(): string[] {
    return (qc.getQueryData<Issue[]>(childKey) ?? []).map((c) => c.id);
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    updateIssue = vi.fn();
    setApiInstance({ updateIssue } as unknown as ApiClient);
    // Seed the detail cache so onMutate resolves the old parent from the
    // freshest source, plus the parent's children list rendered by the
    // sub-issues section.
    const child = makeIssue(1, { parent_issue_id: PARENT_ID, stage: 2 });
    qc.setQueryData<Issue>(issueKeys.detail(WS_ID, child.id), child);
    qc.setQueryData<Issue[]>(childKey, [
      child,
      makeIssue(2, { parent_issue_id: PARENT_ID }),
    ]);
  });

  afterEach(() => {
    qc.clear();
    vi.restoreAllMocks();
  });

  it("optimistically removes the issue from the old parent's children array", async () => {
    let resolve!: (issue: Issue) => void;
    updateIssue.mockReturnValue(
      new Promise<Issue>((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ id: "issue-1", parent_issue_id: null, stage: null });
    });

    // Pruned immediately so the parent's sub-issues list drops it now, not
    // after the settle refetch; the sibling is untouched.
    expect(childIds()).toEqual(["issue-2"]);

    await act(async () => {
      resolve(makeIssue(1, { parent_issue_id: null, stage: null }));
    });
    expect(childIds()).not.toContain("issue-1");
  });

  it("restores the old parent's children when the request fails", async () => {
    updateIssue.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current
        .mutateAsync({ id: "issue-1", parent_issue_id: null, stage: null })
        .catch(() => {});
    });

    expect(childIds()).toEqual(["issue-1", "issue-2"]);
  });

  it("keeps the issue under its parent for a non-reparenting update", async () => {
    updateIssue.mockResolvedValue(
      makeIssue(1, { parent_issue_id: PARENT_ID, status: "done" }),
    );

    const { result } = renderHook(() => useUpdateIssue(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ id: "issue-1", status: "done" });
    });

    // A status-only change patches in place — never prunes the relationship.
    expect(childIds()).toEqual(["issue-1", "issue-2"]);
  });
});

describe("useBatchUpdateIssues — optimistic patch covers filtered boards too", () => {
  const sort: IssueSortParam = { sort_by: "position", sort_direction: undefined };
  const myScope = "assigned";
  const myFilter = { assignee_id: "user-1" };
  const wsKey = issueKeys.listSorted(WS_ID, sort);
  const myKey = issueKeys.myListSorted(WS_ID, myScope, myFilter, sort);

  let qc: QueryClient;
  let batchUpdateIssues: ReturnType<
    typeof vi.fn<(ids: string[], updates: unknown) => Promise<{ updated: number }>>
  >;

  function makeBucketed(): ListIssuesCache {
    return {
      byStatus: {
        todo: { issues: [makeIssue(1)], total: 1 },
        in_progress: { issues: [], total: 0 },
      },
    };
  }

  function bucketIds(key: readonly unknown[], status: "todo" | "in_progress"): string[] {
    const c = qc.getQueryData<ListIssuesCache>(key);
    return (c?.byStatus[status]?.issues ?? []).map((i) => i.id);
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    batchUpdateIssues = vi.fn();
    setApiInstance({ batchUpdateIssues } as unknown as ApiClient);
    qc.setQueryData<ListIssuesCache>(wsKey, makeBucketed());
    qc.setQueryData<ListIssuesCache>(myKey, makeBucketed());
  });

  afterEach(() => {
    qc.clear();
    vi.restoreAllMocks();
  });

  it("optimistically patches BOTH the workspace and myList caches (not just ws)", async () => {
    let resolve!: (r: { updated: number }) => void;
    batchUpdateIssues.mockReturnValue(
      new Promise<{ updated: number }>((r) => {
        resolve = r;
      }),
    );

    const { result } = renderHook(() => useBatchUpdateIssues(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({ ids: ["issue-1"], updates: { status: "in_progress" } });
    });

    // The regression Howard flagged: batch must move the card on the myList
    // board too, not only the workspace board. onMutate awaits cancelQueries,
    // so the optimistic patch lands a microtask later — wait for it.
    await waitFor(() => {
      for (const key of [wsKey, myKey]) {
        expect(bucketIds(key, "todo")).toEqual([]);
        expect(bucketIds(key, "in_progress")).toEqual(["issue-1"]);
      }
    });

    await act(async () => {
      resolve({ updated: 1 });
    });

    for (const key of [wsKey, myKey]) {
      expect(bucketIds(key, "in_progress")).toEqual(["issue-1"]);
    }
  });

  it("does not optimistically replace description merge bases", async () => {
    let resolve!: (r: { updated: number }) => void;
    batchUpdateIssues.mockReturnValue(
      new Promise<{ updated: number }>((r) => {
        resolve = r;
      }),
    );
    const detailKey = issueKeys.detail(WS_ID, "issue-1");
    qc.setQueryData<Issue>(
      detailKey,
      makeIssue(1, { description: "base with marker" }),
    );

    const { result } = renderHook(() => useBatchUpdateIssues(), {
      wrapper: createWrapper(qc),
    });

    act(() => {
      result.current.mutate({
        ids: ["issue-1"],
        updates: {
          description: "local edit",
          description_base: "base with marker",
        },
      });
    });

    await waitFor(() => {
      expect(batchUpdateIssues).toHaveBeenCalledWith(["issue-1"], {
        description: "local edit",
        description_base: "base with marker",
      });
    });
    expect(qc.getQueryData<Issue>(detailKey)?.description).toBe(
      "base with marker",
    );

    await act(async () => {
      resolve({ updated: 1 });
    });
  });

  it("rolls both caches back when the request fails", async () => {
    batchUpdateIssues.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useBatchUpdateIssues(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current
        .mutateAsync({ ids: ["issue-1"], updates: { status: "in_progress" } })
        .catch(() => {});
    });

    for (const key of [wsKey, myKey]) {
      expect(bucketIds(key, "todo")).toEqual(["issue-1"]);
      expect(bucketIds(key, "in_progress")).toEqual([]);
    }
  });

  it("forwards per-item completion controls without adding them to optimistic issue data", async () => {
    let resolve!: (value: { updated: number }) => void;
    batchUpdateIssues.mockReturnValue(new Promise((done) => { resolve = done; }));
    const detailKey = issueKeys.detail(WS_ID, "issue-1");
    qc.setQueryData<Issue>(detailKey, makeIssue(1, { revision: 6 }));
    const { result } = renderHook(() => useBatchUpdateIssues(), { wrapper: createWrapper(qc) });
    const binding = { issue_id: "issue-1", authorization_id: "authorization", operation_id: "operation", expected_issue_revision: 6 };
    const updates = { status: "in_progress" as const, project_change_reason: "Scope correction", expected_revision: 6 };
    let pending!: Promise<unknown>;
    act(() => { pending = result.current.mutateAsync({ ids: ["issue-1"], updates, completion_bindings: [binding] }); });
    await waitFor(() => expect(batchUpdateIssues).toHaveBeenCalledWith(["issue-1"], updates, { completion_bindings: [binding] }));
    const cached = qc.getQueryData<Issue>(detailKey);
    expect(cached?.status).toBe("in_progress");
    for (const control of ["completion", "completion_bindings", "project_change_reason", "expected_revision"]) expect(cached).not.toHaveProperty(control);
    await act(async () => { resolve({ updated: 1 }); await pending; });
  });

  it("refreshes actual committed state after a partial failure instead of retaining a full rollback", async () => {
    for (const key of [wsKey, myKey]) {
      qc.setQueryData(key, { byStatus: { todo: { issues: [makeIssue(1), makeIssue(2)], total: 2 }, in_progress: { issues: [], total: 0 } } });
    }
    for (const index of [1, 2]) qc.setQueryData(issueKeys.detail(WS_ID, `issue-${index}`), makeIssue(index));
    const actual = { byStatus: { todo: { issues: [makeIssue(2)], total: 1 }, in_progress: { issues: [makeIssue(1, { status: "in_progress", revision: 2 })], total: 1 } } };
    const readWorkspace = vi.fn().mockResolvedValue(actual);
    const readMyList = vi.fn().mockResolvedValue(actual);
    const subscriptions = [
      new QueryObserver(qc, { queryKey: wsKey, queryFn: readWorkspace, staleTime: Infinity }).subscribe(() => {}),
      new QueryObserver(qc, { queryKey: myKey, queryFn: readMyList, staleTime: Infinity }).subscribe(() => {}),
    ];
    batchUpdateIssues.mockRejectedValue(new Error("One item committed; another was rejected"));
    const { result } = renderHook(() => useBatchUpdateIssues(), { wrapper: createWrapper(qc) });
    try {
      await act(async () => {
        await result.current.mutateAsync({ ids: ["issue-1", "issue-2"], updates: { status: "in_progress" } }).catch(() => {});
      });
      await waitFor(() => {
        expect(readWorkspace).toHaveBeenCalledOnce();
        expect(readMyList).toHaveBeenCalledOnce();
        expect(bucketIds(wsKey, "in_progress")).toEqual(["issue-1"]);
        expect(bucketIds(myKey, "in_progress")).toEqual(["issue-1"]);
      });
      expect(batchUpdateIssues).toHaveBeenCalledOnce();
    } finally { subscriptions.forEach((unsubscribe) => unsubscribe()); }
  });

  it("does not overwrite a newer server snapshot when a batch fails", async () => {
    let reject!: (error: Error) => void;
    batchUpdateIssues.mockReturnValue(new Promise((_resolve, rejectPromise) => { reject = rejectPromise; }));
    const detailKey = issueKeys.detail(WS_ID, "issue-1");
    qc.setQueryData<Issue>(detailKey, makeIssue(1, { revision: 1 }));
    const { result } = renderHook(() => useBatchUpdateIssues(), { wrapper: createWrapper(qc) });
    let pending!: Promise<unknown>;
    act(() => { pending = result.current.mutateAsync({ ids: ["issue-1"], updates: { status: "in_progress" } }).catch(() => {}); });
    await waitFor(() => expect(batchUpdateIssues).toHaveBeenCalledOnce());
    const newer = makeIssue(1, { status: "done", revision: 8, title: "Another current edit" });
    qc.setQueryData<Issue>(detailKey, newer);
    await act(async () => { reject(new Error("Unknown commit")); await pending; });
    expect(qc.getQueryData<Issue>(detailKey)).toEqual(newer);
    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
  });

  it("does not invalidate the board list on settle (no refetch flicker)", async () => {
    batchUpdateIssues.mockResolvedValue({ updated: 1 });
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");

    const { result } = renderHook(() => useBatchUpdateIssues(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ ids: ["issue-1"], updates: { status: "in_progress" } });
    });

    const invalidatedKeys = invalidateSpy.mock.calls.map((c) => c[0]?.queryKey);
    expect(invalidatedKeys).not.toContainEqual(issueKeys.list(WS_ID));
  });

  it("surgically removes moved issues from the old project's list (no blanket myAll refetch)", async () => {
    // Mirrors useUpdateIssue: a batch project move drops the cards from the
    // old project's loaded list via the membership-aware coordinator instead
    // of refetching every filtered list (MUL-3669 / #4548).
    const projectScope = "project:p1";
    const projectFilter = { project_id: "p1" };
    const projectKey = issueKeys.myListSorted(WS_ID, projectScope, projectFilter, sort);
    qc.setQueryData<ListIssuesCache>(projectKey, makeBucketed());
    batchUpdateIssues.mockResolvedValue({ updated: 1 });
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");

    const { result } = renderHook(() => useBatchUpdateIssues(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({
        ids: ["issue-1"],
        updates: { project_id: "project-9" },
      });
    });

    expect(bucketIds(projectKey, "todo")).toEqual([]);
    // The assignee-filtered list is untouched by a project move.
    expect(bucketIds(myKey, "todo")).toEqual(["issue-1"]);
    const invalidatedKeys = invalidateSpy.mock.calls.map((c) => c[0]?.queryKey);
    expect(invalidatedKeys).not.toContainEqual(issueKeys.myAll(WS_ID));
  });
});

describe("comment mutations — owner revision and last activity", () => {
  const issueId = "issue-1";
  const detailKey = issueKeys.detail(WS_ID, issueId);
  const lastActivityKey = issueKeys.listSorted(WS_ID, {
    sort_by: "last_activity",
    sort_direction: "desc",
  });
  const positionKey = issueKeys.listSorted(WS_ID, { sort_by: "position" });

  function seed(qc: QueryClient) {
    const issue = makeIssue(1, { revision: 1 });
    const board: ListIssuesCache = {
      byStatus: { todo: { issues: [issue], total: 1 } },
    };
    qc.setQueryData<Issue>(detailKey, issue);
    qc.setQueryData<ListIssuesCache>(lastActivityKey, board);
    qc.setQueryData<ListIssuesCache>(positionKey, board);
    qc.setQueryData<TimelineEntry[]>(issueKeys.timeline(issueId), [
      {
        type: "comment",
        id: "comment-1",
        actor_type: "member",
        actor_id: "user-1",
        content: "before",
        parent_id: null,
        comment_type: "comment",
        reactions: [],
        attachments: [],
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
    ]);
  }

  it("consumes an update response's issue revision and re-sorts activity", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    seed(qc);
    setApiInstance({
      updateComment: vi.fn().mockResolvedValue({
        id: "comment-1",
        issue_id: issueId,
        content: "after",
        issue_revision: 2,
      }),
    } as unknown as ApiClient);
    const { result } = renderHook(() => useUpdateComment(issueId), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({
        commentId: "comment-1",
        content: "after",
        attachmentIds: [],
      });
    });

    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(lastActivityKey)?.isInvalidated).toBe(true);
    // The owner revision also invalidates any loaded projection containing
    // this issue, independent of sort.
    expect(qc.getQueryState(positionKey)?.isInvalidated).toBe(true);
    qc.clear();
  });

  it("invalidates the owner projection after a successful 204 delete", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    seed(qc);
    setApiInstance({
      deleteComment: vi.fn().mockResolvedValue(undefined),
    } as unknown as ApiClient);
    const { result } = renderHook(() => useDeleteComment(issueId), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync("comment-1");
    });

    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(lastActivityKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(positionKey)?.isInvalidated).toBe(true);
    qc.clear();
  });
});

describe("useResolveComment", () => {
  const ISSUE_ID = "issue-1";

  function makeComment(
    id: string,
    parentId: string | null,
    resolvedAt: string | null,
  ): TimelineEntry {
    return {
      type: "comment",
      id,
      actor_type: "member",
      actor_id: "user-1",
      content: id,
      parent_id: parentId,
      comment_type: "comment",
      reactions: [],
      attachments: [],
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      resolved_at: resolvedAt,
      resolved_by_type: resolvedAt ? "member" : null,
      resolved_by_id: resolvedAt ? "user-1" : null,
    };
  }

  // Two independent threads on one issue:
  //   root1 ─ a1 (resolved), b1
  //   root2 ─ a2 (resolved)
  function seedTimeline(qc: QueryClient) {
    const entries: TimelineEntry[] = [
      makeComment("root1", null, null),
      makeComment("a1", "root1", "2026-01-01T00:01:00Z"),
      makeComment("b1", "root1", null),
      makeComment("root2", null, null),
      makeComment("a2", "root2", "2026-01-01T00:05:00Z"),
    ];
    qc.setQueryData<TimelineEntry[]>(issueKeys.timeline(ISSUE_ID), entries);
  }

  function resolvedIds(qc: QueryClient): string[] {
    const cache = qc.getQueryData<TimelineEntry[]>(issueKeys.timeline(ISSUE_ID)) ?? [];
    return cache.filter((e) => e.resolved_at).map((e) => e.id).sort();
  }

  let qc: QueryClient;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    setApiInstance({
      resolveComment: vi.fn().mockResolvedValue({ id: "b1" }),
      unresolveComment: vi.fn().mockResolvedValue({ id: "b1" }),
    } as unknown as ApiClient);
  });

  afterEach(() => {
    qc.clear();
    vi.restoreAllMocks();
  });

  it("clears the prior resolution in the same thread when resolving another comment", async () => {
    seedTimeline(qc);

    const { result } = renderHook(() => useResolveComment(ISSUE_ID), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ commentId: "b1", resolved: true });
    });

    // b1 replaces a1 inside thread 1; a2 (thread 2) is untouched.
    expect(resolvedIds(qc)).toEqual(["a2", "b1"]);
  });

  it("does not clear resolutions in other threads", async () => {
    seedTimeline(qc);

    const { result } = renderHook(() => useResolveComment(ISSUE_ID), {
      wrapper: createWrapper(qc),
    });

    // Resolving root1 (thread 1) must leave a2 (thread 2) resolved.
    await act(async () => {
      await result.current.mutateAsync({ commentId: "root1", resolved: true });
    });

    expect(resolvedIds(qc)).toEqual(["a2", "root1"]);
  });

  it("unresolve only clears its own row, never siblings", async () => {
    // Legacy state: two resolved comments coexist in one thread.
    qc.setQueryData<TimelineEntry[]>(issueKeys.timeline(ISSUE_ID), [
      makeComment("root1", null, null),
      makeComment("a1", "root1", "2026-01-01T00:01:00Z"),
      makeComment("b1", "root1", "2026-01-01T00:02:00Z"),
    ]);

    const { result } = renderHook(() => useResolveComment(ISSUE_ID), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ commentId: "b1", resolved: false });
    });

    // Only b1 is cleared; a1 stays resolved (unresolve never mirrors the clear).
    expect(resolvedIds(qc)).toEqual(["a1"]);
  });
});

// MUL-6394: posting a comment while the Table view's grouped/facet caches are
// loaded rejected `mutateAsync` with "Cannot read properties of undefined
// (reading 'some')" — the comment WAS created server-side (the agent task
// started), but the composer showed an error toast and never appended the
// entry, so it only appeared after a reload.
describe("useCreateComment — sibling caches under a shared key prefix", () => {
  const ISSUE_ID = "issue-1";
  const tableQuery = {
    scope: { kind: "workspace" },
    filters: {},
    sort: { field: "position", direction: "asc" },
  } as const;

  let qc: QueryClient;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    setApiInstance({
      createComment: vi.fn().mockResolvedValue({
        id: "comment-1",
        issue_id: ISSUE_ID,
        author_type: "member",
        author_id: "user-1",
        content: "hello",
        type: "comment",
        parent_id: null,
        reactions: [],
        attachments: [],
        created_at: "2026-08-19T00:00:00Z",
        updated_at: "2026-08-19T00:00:00Z",
        resolved_at: null,
        resolved_by_type: null,
        resolved_by_id: null,
        issue_revision: 7,
      }),
    } as unknown as ApiClient);
  });

  afterEach(() => {
    qc.clear();
    vi.restoreAllMocks();
  });

  it("appends the created comment even when non-row table caches are loaded", async () => {
    qc.setQueryData<TimelineEntry[]>(issueKeys.timeline(ISSUE_ID), []);
    // Grouped rows are an infinite cache and facets a plain object — both live
    // under the `table-query` prefix next to the row pages, and neither has a
    // `rows` array.
    qc.setQueryData(issueKeys.tableGroups(WS_ID, tableQuery, { kind: "status" }), {
      pages: [
        { query_fingerprint: "sha256:groups", total: 0, groups: [], next_cursor: null },
      ],
      pageParams: [null],
    });
    qc.setQueryData(
      issueKeys.tableFacets(WS_ID, { query: tableQuery, facets: [{ kind: "status" }] }),
      { query_fingerprint: "sha256:facets", total: 0, facets: [] },
    );

    const { result } = renderHook(() => useCreateComment(ISSUE_ID), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync({ content: "hello" });
    });

    expect(
      qc.getQueryData<TimelineEntry[]>(issueKeys.timeline(ISSUE_ID))?.map((e) => e.id),
    ).toEqual(["comment-1"]);
  });
});
