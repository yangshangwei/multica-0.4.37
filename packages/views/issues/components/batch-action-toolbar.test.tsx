vi.mock("../../iterations/iteration-assignment", () => ({
  IterationBatchAssignment: ({ issueIds }: { issueIds: string[] }) => <button aria-label="Iteration batch selection" data-issue-ids={issueIds.join(",")}>Iterations</button>,
}));
import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import type { Issue, UpdateIssueRequest } from "@multica/core/types";
import enIssues from "../../locales/en/issues.json";
import { BatchActionToolbar } from "./batch-action-toolbar";

// Mutable selection state shared with the store mock below. The real toolbar
// derives the pickers' current value from the issues it can resolve out of
// this selection, so each test sets `selection.selectedIds` before rendering.
const selection = vi.hoisted(() => ({
  selectedIds: new Set<string>(),
  clear: vi.fn(),
}));

vi.mock("@multica/core/issues/stores/selection-store", () => ({
  useIssueSelectionStore: (selector: (s: typeof selection) => unknown) =>
    selector(selection),
}));

const batchUpdate = vi.hoisted(() => vi.fn().mockResolvedValue({ updated: 1 }));
vi.mock("@multica/core/issues/mutations", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/issues/mutations")>(),
  useBatchUpdateIssues: () => ({ mutateAsync: batchUpdate, isPending: false }),
  useBatchDeleteIssues: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("../../i18n", () => ({
  useT: () => ({ t: (selector: (value: typeof import("../../locales/en/issues.json")) => unknown) => selector(enIssues) }),
}));

// Render each picker as a probe that surfaces the value the toolbar passed in,
// so the test asserts the wiring (real `commonIssueFields` runs underneath).
vi.mock("./pickers", () => ({
  StatusPicker: ({ status }: { status: string | null }) => (
    <div data-testid="status-picker" data-status={status ?? "__none__"} />
  ),
  PriorityPicker: ({ priority, onUpdate }: { priority: string | null; onUpdate: (updates: Partial<UpdateIssueRequest>) => void }) => (
    <button data-testid="priority-picker" data-priority={priority ?? "__none__"} onClick={() => onUpdate({ priority: "high" })} />
  ),
  AssigneePicker: ({
    assigneeType,
    assigneeId,
    mixed,
  }: {
    assigneeType: string | null;
    assigneeId: string | null;
    mixed?: boolean;
  }) => (
    <div
      data-testid="assignee-picker"
      data-assignee-type={assigneeType ?? "__null__"}
      data-assignee-id={assigneeId ?? "__null__"}
      data-mixed={String(Boolean(mixed))}
    />
  ),
}));

function makeIssue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "Issue 1",
    description: null,
    status: "todo",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 1,
    stage: null,
    start_date: null,
    due_date: null,
    metadata: {},
    properties: {},
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  selection.selectedIds = new Set();
  batchUpdate.mockClear();
});

describe("BatchActionToolbar picker wiring", () => {
  // Selection equality belongs to the toolbar; numeric validity belongs to Core field-revision.test.ts.
  it.each([
    { name: "one observed selected row", revisions: [7], expected: 7 },
    { name: "equal selected observations", revisions: [7, 7], expected: 7 },
    { name: "different selected observations", revisions: [7, 8], expected: undefined },
    { name: "a missing selected observation", revisions: [7, undefined], expected: undefined },
    { name: "invalid equal observations", revisions: [0, 0], expected: undefined },
  ])("uses only a common rendered revision for $name", async ({ revisions, expected }) => {
    const issues = revisions.map((revision, index) => makeIssue({ id: `selected-${index}`, revision }));
    selection.selectedIds = new Set([...issues.map((issue) => issue.id), "missing-row"]);
    render(<BatchActionToolbar issues={[...issues, makeIssue({ id: "unselected", revision: 99 })]} />);
    fireEvent.click(screen.getByTestId("priority-picker"));
    await waitFor(() => expect(batchUpdate).toHaveBeenCalledOnce());
    expect(batchUpdate).toHaveBeenCalledWith({
      ids: issues.map((issue) => issue.id),
      updates: { priority: "high", ...(expected !== undefined ? { expected_revision: expected } : {}) },
    });
  });

  it("has no action when selected IDs resolve to no rendered issues", () => {
    selection.selectedIds = new Set(["missing-row"]);
    render(<BatchActionToolbar issues={[makeIssue({ id: "a", revision: 7 })]} />);
    expect(screen.queryByTestId("priority-picker")).not.toBeInTheDocument();
    expect(batchUpdate).not.toHaveBeenCalled();
  });

  it("names and activates the clear-selection button", () => {
    selection.selectedIds = new Set(["a"]);
    render(<BatchActionToolbar issues={[makeIssue({ id: "a" })]} />);
    fireEvent.click(screen.getByRole("button", { name: "Clear selection" }));
    expect(selection.clear).toHaveBeenCalled();
  });

  it("passes only resolved selected tasks to the iteration entry", () => {
    selection.selectedIds = new Set(["a", "stale"]);
    render(<BatchActionToolbar issues={[makeIssue({ id: "a" }), makeIssue({ id: "b" })]} />);
    expect(screen.getByRole("button", { name: "Iteration batch selection" })).toHaveAttribute("data-issue-ids", "a");
  });
  it("reflects the shared status / priority / assignee of the selected issues", () => {
    const issues = [
      makeIssue({ id: "a", status: "in_progress", priority: "high", assignee_type: "member", assignee_id: "u-1" }),
      makeIssue({ id: "b", status: "in_progress", priority: "high", assignee_type: "member", assignee_id: "u-1" }),
    ];
    selection.selectedIds = new Set(["a", "b"]);

    render(<BatchActionToolbar issues={issues} />);

    expect(screen.getByTestId("status-picker")).toHaveAttribute("data-status", "in_progress");
    expect(screen.getByTestId("priority-picker")).toHaveAttribute("data-priority", "high");
    const assignee = screen.getByTestId("assignee-picker");
    expect(assignee).toHaveAttribute("data-assignee-type", "member");
    expect(assignee).toHaveAttribute("data-assignee-id", "u-1");
    expect(assignee).toHaveAttribute("data-mixed", "false");
  });

  it("falls back to an empty (no-checkmark) state when the selection is mixed", () => {
    const issues = [
      makeIssue({ id: "a", status: "todo", priority: "none", assignee_type: "member", assignee_id: "u-1" }),
      makeIssue({ id: "b", status: "done", priority: "urgent", assignee_type: "agent", assignee_id: "ag-1" }),
    ];
    selection.selectedIds = new Set(["a", "b"]);

    render(<BatchActionToolbar issues={issues} />);

    expect(screen.getByTestId("status-picker")).toHaveAttribute("data-status", "__none__");
    expect(screen.getByTestId("priority-picker")).toHaveAttribute("data-priority", "__none__");
    expect(screen.getByTestId("assignee-picker")).toHaveAttribute("data-mixed", "true");
  });

  it("treats an all-unassigned selection as unassigned, not mixed", () => {
    const issues = [
      makeIssue({ id: "a", assignee_type: null, assignee_id: null }),
      makeIssue({ id: "b", assignee_type: null, assignee_id: null }),
    ];
    selection.selectedIds = new Set(["a", "b"]);

    render(<BatchActionToolbar issues={issues} />);

    const assignee = screen.getByTestId("assignee-picker");
    expect(assignee).toHaveAttribute("data-mixed", "false");
    expect(assignee).toHaveAttribute("data-assignee-type", "__null__");
    expect(assignee).toHaveAttribute("data-assignee-id", "__null__");
  });

  it("renders nothing when nothing is selected", () => {
    render(<BatchActionToolbar issues={[makeIssue({ id: "a" })]} />);
    expect(screen.queryByTestId("status-picker")).toBeNull();
  });

  it("removes the toolbar after the final selected issue is cleared", async () => {
    const issues = [makeIssue({ id: "a" })];
    selection.selectedIds = new Set(["a"]);
    const view = render(<BatchActionToolbar issues={issues} />);

    expect(screen.getByTestId("status-picker")).toBeInTheDocument();
    selection.selectedIds = new Set();
    view.rerender(<BatchActionToolbar issues={issues} />);

    await waitFor(() => {
      expect(screen.queryByTestId("status-picker")).not.toBeInTheDocument();
    });
  });
});
