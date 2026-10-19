import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Issue } from "@multica/core/types";
import { issueKeys } from "@multica/core/issues/queries";
import { RESOURCES } from "../../test/i18n";
import { useIssueSurfaceActions } from "./use-issue-surface-actions";

const calls = vi.hoisted(() => ({ update: vi.fn(), batchUpdate: vi.fn(), batchDelete: vi.fn() }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "source" }));
vi.mock("@multica/core/issue-statuses/hooks", () => ({ useIssueStatuses: () => ({ statuses: [{ key: "review_custom", category: "in_review" }, { key: "done_custom", category: "done" }] }) }));
vi.mock("@multica/core/issues/mutations", () => ({
  useUpdateIssue: () => ({ mutate: calls.update, isPending: false }),
  useBatchUpdateIssues: () => ({ mutateAsync: calls.batchUpdate, isPending: false }),
  useBatchDeleteIssues: () => ({ mutateAsync: calls.batchDelete, isPending: false }),
}));

const account = { user_id: "handler", name: "External handler", avatar_url: "", external: true };
const shared: Issue = { id: "shared", workspace_id: "source", number: 1, identifier: "SRC-1", title: "Shared source task", description: "",
  status: "todo", priority: "none", assignee_type: "member", assignee_id: account.user_id, creator_type: "member", creator_id: "creator",
  parent_issue_id: null, project_id: null, position: 1, stage: null, start_date: null, due_date: null, metadata: {}, properties: {},
  created_at: "2026-10-19T00:00:00Z", updated_at: "2026-10-19T00:00:00Z", revision: 7,
  collaboration: { creator: { ...account, user_id: "creator" }, handler: account, has_external_collaborators: true } };

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  // A table-only loaded record must receive the same protection as a detail.
  client.setQueryData(["issues", "source", "table-query", "rows", "fixture"], { rows: [{ issue: shared, direct_child_count: 0 }] });
  const wrapper = ({ children }: { children: ReactNode }) => <I18nProvider locale="en" resources={RESOURCES}><QueryClientProvider client={client}>{children}</QueryClientProvider></I18nProvider>;
  return { client, ...renderHook(() => useIssueSurfaceActions({ createDefaults: {} }), { wrapper }) };
}

describe("collaboration source actions", () => {
  beforeEach(() => vi.clearAllMocks());
  it("opens the authoritative workflow before an assignee or terminal-status move", () => {
    const { result } = setup();
    act(() => result.current.moveIssue(shared.id, { assignee_type: "member", assignee_id: "next", position: 20, before_id: null, after_id: "other" }));
    expect(calls.update).not.toHaveBeenCalled();
    expect(result.current.collaborationAction).toEqual({ issueId: shared.id, action: "transfer" });
    act(() => result.current.moveIssue(shared.id, { status: "done_custom", position: 20, before_id: null, after_id: null }));
    expect(result.current.collaborationAction?.action).toBe("lifecycle");
    expect(calls.update).not.toHaveBeenCalled();
  });
  it("keeps the R58 request to position and original ordinary revision only", () => {
    const { result } = setup();
    act(() => result.current.moveIssue(shared.id, { status: "todo", assignee_type: "member", assignee_id: account.user_id, position: -2, before_id: "a", after_id: "b" }));
    expect(calls.update.mock.calls[0]?.[0]).toEqual({ id: shared.id, position: -2, expected_revision: 7 });
    expect(result.current.collaborationAction).toBeNull();
  });
  it("prevents a mixed batch from partially changing ordinary rows", async () => {
    const { result } = setup();
    await expect(result.current.actions.batchUpdate(["ordinary", shared.id], { status: "done" })).rejects.toThrow(/individually/);
    await expect(result.current.actions.batchDelete(["ordinary", shared.id])).rejects.toThrow(/individually/);
    expect(calls.batchUpdate).not.toHaveBeenCalled();
    expect(calls.batchDelete).not.toHaveBeenCalled();
  });
  it("retains ordinary relative drag writes", () => {
    const { client, result } = setup();
    client.setQueryData(issueKeys.detail("source", "ordinary"), { ...shared, id: "ordinary", collaboration: undefined });
    act(() => result.current.moveIssue("ordinary", { status: "done", position: 10, before_id: "a", after_id: "b" }));
    expect(calls.update.mock.calls[0]?.[0]).toEqual({ id: "ordinary", status: "done", position: 10, move_intent: { before_id: "a", after_id: "b" } });
    expect(result.current.collaborationAction).toBeNull();
  });
});
