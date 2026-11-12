// @vitest-environment jsdom

import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";

const { mockOpenModal, mockHint, mockGetIssue, mockToastError, authState } = vi.hoisted(() => ({
  mockOpenModal: vi.fn(),
  mockHint: vi.fn(),
  mockGetIssue: vi.fn(),
  mockToastError: vi.fn(),
  authState: { ready: true },
}));

vi.mock("@multica/core/modals", () => ({
  useModalStore: (selector: (state: { open: typeof mockOpenModal }) => unknown) =>
    selector({ open: mockOpenModal }),
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign((selector: (state: { user: { id: string } }) => unknown) => {
    if (!authState.ready) throw new Error("Auth store not initialised");
    return selector({ user: { id: "user-1" } });
  }, { getState: () => {
    if (!authState.ready) throw new Error("Auth store not initialised");
    return { user: { id: "user-1" } };
  } }),
}));

vi.mock("@multica/core/api", () => ({
  api: { getDeliveryCompletionHint: mockHint, getIssue: mockGetIssue },
  ApiError: class ApiError extends Error {
    status = 500;
    body: unknown;
  },
  errorCode: () => undefined,
}));

vi.mock("@multica/core/issues", () => ({
  classifyCompletionHintIntent: vi.fn(() => ({
    kind: "requires_hint",
    request: {
      expected_issue_revision: 4,
      target_status: "done",
      evaluation_key: null,
    },
  })),
  isLegacyCompletionHintError: (error: unknown) =>
    typeof error === "object" && error !== null && "legacy" in error,
  issueStatusCategory: (issue: Issue) => issue.status_category ?? issue.status,
  deliveryCompletionScope: (userId: string, workspaceId: string, issueId: string) => ({ userId, workspaceId, issueId, server: "test", apiScope: "session" }),
}));

vi.mock("@multica/core/issue-statuses", () => ({
  isIssueStatusCategory: (value: string) =>
    ["backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"].includes(value),
}));

vi.mock("sonner", () => ({ toast: { error: mockToastError } }));
vi.mock("../../i18n", () => ({ useT: () => ({ t: () => "completion hint message" }) }));

import { useCompletionHintGate } from "./completion-hint-gate";

const issue = (overrides: Partial<Issue> = {}): Issue => ({
  id: "issue-1",
  workspace_id: "ws-1",
  number: 1,
  identifier: "MUL-1",
  title: "Issue",
  description: null,
  status: "todo",
  status_category: "todo",
  priority: "medium",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: "project-1",
  position: 1,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  properties: {},
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  revision: 4,
  ...overrides,
});

beforeEach(() => {
  mockOpenModal.mockReset();
  mockHint.mockReset();
  mockGetIssue.mockReset();
  mockToastError.mockReset();
  authState.ready = true;
});

function runGate(
  result: { current: ReturnType<typeof useCompletionHintGate> },
  current = issue(),
  onContinue = vi.fn(),
) {
  act(() => {
    result.current({
      issue: current,
      targetStatus: "done",
      targetCategory: "done",
      updates: { status: "done" },
      continueMutation: onContinue,
    });
  });
  return onContinue;
}

describe("useCompletionHintGate", () => {
  it("does not require the auth store for ordinary surface initialization", () => {
    authState.ready = false;
    expect(() => renderHook(() => useCompletionHintGate("issue-1"))).not.toThrow();
  });

  it("routes enforce to explicit authorization and forwards settlement without a second PUT", async () => {
    mockHint.mockResolvedValue({ policy_mode: "enforce", hint: "attention", target_category: "done", enforced: false, usable_for_completion: false });
    const { result } = renderHook(() => useCompletionHintGate("issue-1"));
    const onContinue = vi.fn(), onSuccess = vi.fn(), onSettled = vi.fn();
    act(() => result.current({ issue: issue(), targetStatus: "done", targetCategory: "done", updates: { status: "done" },
      continueMutation: onContinue, options: { onSuccess, onSettled } }));
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledWith("issue-completion-authorization", expect.anything()));
    const data = mockOpenModal.mock.calls[0]?.[1] as { onCompleted: (issue: Issue) => void };
    const completedIssue = issue({ status: "done", status_category: "done", revision: 5 });
    act(() => { data.onCompleted(completedIssue); data.onCompleted(completedIssue); });
    expect(onContinue).not.toHaveBeenCalled();
    expect(onSuccess).toHaveBeenCalledExactlyOnceWith(completedIssue);
    expect(onSettled).toHaveBeenCalledTimes(1);
    expect(mockHint.mock.calls[0]?.[2]).toMatchObject({ workspaceId: "ws-1" });
  });

  it("settles the waiting caller when the authorization flow is cancelled", async () => {
    mockHint.mockResolvedValue({ policy_mode: "enforce", hint: "attention", target_category: "done", enforced: false, usable_for_completion: false });
    const { result } = renderHook(() => useCompletionHintGate("issue-1"));
    const onContinue = vi.fn(), onSettled = vi.fn();
    act(() => result.current({ issue: issue(), targetStatus: "done", targetCategory: "done", updates: { status: "done" },
      continueMutation: onContinue, options: { onSettled } }));
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledWith("issue-completion-authorization", expect.anything()));

    const data = mockOpenModal.mock.calls[0]?.[1] as { onCancel: () => void };
    act(() => data.onCancel());
    expect(onContinue).not.toHaveBeenCalled();
    expect(onSettled).toHaveBeenCalledTimes(1);
  });

  it("aborts the authorization owner when the displayed issue changes", async () => {
    mockHint.mockResolvedValue({ policy_mode: "enforce", hint: "attention", target_category: "done", enforced: false, usable_for_completion: false });
    const { result, rerender } = renderHook(({ owner }) => useCompletionHintGate(owner), { initialProps: { owner: "issue-1" } });
    const onContinue = runGate(result);
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledTimes(1));
    const data = mockOpenModal.mock.calls[0]?.[1] as { ownerSignal: AbortSignal; onCompleted: (issue: Issue) => void };
    rerender({ owner: "issue-2" });
    expect(data.ownerSignal.aborted).toBe(true);
    act(() => data.onCompleted(issue({ status: "done" })));
    expect(onContinue).not.toHaveBeenCalled();
  });

  it("leaves collaboration source completion to its existing workflow", () => {
    const account = { user_id: "user-1", name: "Handler", avatar_url: "", external: true };
    const { result } = renderHook(() => useCompletionHintGate());
    const onContinue = runGate(result, issue({ collaboration: {
      creator: account, handler: account, has_external_collaborators: true,
    } }));
    expect(onContinue).toHaveBeenCalledTimes(1);
    expect(mockHint).not.toHaveBeenCalled();
  });

  it("refreshes the issue revision before an explicit retry", async () => {
    mockHint.mockResolvedValue({ hint: "unexpected" });
    mockGetIssue.mockResolvedValue(issue({ revision: 5 }));
    const { result } = renderHook(() => useCompletionHintGate());
    const onContinue = runGate(result);
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledTimes(1));
    const data = mockOpenModal.mock.calls[0]?.[1] as { onRetry: () => void };
    await act(async () => { data.onRetry(); });
    await waitFor(() => expect(mockHint).toHaveBeenCalledTimes(2));
    expect(mockHint.mock.calls[1]?.[1]).toMatchObject({ expected_issue_revision: 5 });
    expect(onContinue).not.toHaveBeenCalled();
  });

  it("cannot confirm an obsolete modal after its owner unmounts", async () => {
    mockHint.mockResolvedValue({ hint: "attention", target_category: "done", enforced: false, usable_for_completion: false });
    const { result, unmount } = renderHook(() => useCompletionHintGate());
    const onContinue = runGate(result);
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledTimes(1));
    const data = mockOpenModal.mock.calls[0]?.[1] as { onConfirm: () => Promise<void> };
    unmount();
    await act(async () => { await data.onConfirm(); });
    expect(onContinue).not.toHaveBeenCalled();
  });

  it("consumes a confirmation only once while the real mutation is pending", async () => {
    mockHint.mockResolvedValue({ hint: "attention", target_category: "done", enforced: false, usable_for_completion: false });
    let complete!: () => void;
    const mutation = new Promise<void>((resolve) => { complete = resolve; });
    const onContinue = vi.fn(() => mutation);
    const { result } = renderHook(() => useCompletionHintGate());
    runGate(result, issue(), onContinue);
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledTimes(1));
    const data = mockOpenModal.mock.calls[0]?.[1] as { onConfirm: () => Promise<void> };
    let first!: Promise<void>;
    act(() => { first = data.onConfirm(); void data.onConfirm(); });
    expect(onContinue).toHaveBeenCalledTimes(1);
    await act(async () => { complete(); await first; });
  });

  it("opens attention once, cancels without PUT, and confirms exactly once", async () => {
    mockHint.mockResolvedValue({
      hint: "attention",
      target_category: "done",
      enforced: false,
      usable_for_completion: false,
    });
    const { result } = renderHook(() => useCompletionHintGate());
    const firstContinue = runGate(result);
    runGate(result);
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledTimes(1));
    expect(mockHint).toHaveBeenCalledTimes(1);
    expect(firstContinue).not.toHaveBeenCalled();

    const data = mockOpenModal.mock.calls[0]?.[1] as { onCancel: () => void };
    act(() => data.onCancel());
    expect(firstContinue).not.toHaveBeenCalled();

    const secondContinue = runGate(result);
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledTimes(2));
    const confirmData = mockOpenModal.mock.calls[1]?.[1] as { onConfirm: () => Promise<void> };
    await act(async () => { await confirmData.onConfirm(); });
    expect(secondContinue).toHaveBeenCalledTimes(1);
  });

  it("does not write for an unknown response", async () => {
    mockHint.mockResolvedValue({
      hint: "unexpected",
      target_category: "done",
      enforced: false,
      usable_for_completion: false,
    });
    const { result } = renderHook(() => useCompletionHintGate());
    const onContinue = runGate(result);
    await waitFor(() => expect(mockOpenModal).toHaveBeenCalledTimes(1));
    const data = mockOpenModal.mock.calls[0]?.[1] as { state: string; onRetry: () => void };
    expect(data.state).toBe("unknown");
    expect(data.onRetry).toBeTypeOf("function");
    expect(onContinue).not.toHaveBeenCalled();
  });

  it("bypasses only an unstructured legacy 404/405", async () => {
    mockHint
      .mockRejectedValueOnce({ status: 404, body: {}, legacy: true })
      .mockResolvedValueOnce({
        hint: "attention",
        target_category: "done",
        enforced: false,
        usable_for_completion: false,
      });
    const { result } = renderHook(() => useCompletionHintGate());
    const onContinue = runGate(result);
    await waitFor(() => expect(onContinue).toHaveBeenCalledTimes(1));
    runGate(result);
    await waitFor(() => expect(mockHint).toHaveBeenCalledTimes(2));
  });

  it("fails closed when revision is missing", () => {
    const { result } = renderHook(() => useCompletionHintGate());
    const onContinue = runGate(result, issue({ revision: undefined }));
    expect(mockHint).not.toHaveBeenCalled();
    expect(onContinue).not.toHaveBeenCalled();
    expect(mockToastError).toHaveBeenCalled();
  });

  it("does not hint non-completion transitions or project-less issues", () => {
    const { result } = renderHook(() => useCompletionHintGate());
    const nonDone = vi.fn();
    act(() => result.current({
      issue: issue(),
      targetStatus: "in_review",
      targetCategory: "in_review",
      updates: { status: "in_review" },
      continueMutation: nonDone,
    }));
    const noProject = vi.fn();
    act(() => result.current({
      issue: issue({ project_id: null }),
      targetStatus: "done",
      targetCategory: "done",
      updates: { status: "done" },
      continueMutation: noProject,
    }));
    expect(nonDone).toHaveBeenCalledTimes(1);
    expect(noProject).toHaveBeenCalledTimes(1);
    expect(mockHint).not.toHaveBeenCalled();
  });
});
