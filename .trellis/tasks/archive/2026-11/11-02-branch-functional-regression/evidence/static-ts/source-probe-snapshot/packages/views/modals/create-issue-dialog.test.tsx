import type { ReactNode } from "react";
import { beforeEach, describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

const mockSetLastMode = vi.hoisted(() => vi.fn());
const mockBeginIsolatedDraft = vi.hoisted(() => vi.fn());
const mockEndIsolatedDraft = vi.hoisted(() => vi.fn());
const mockRefetchSourceContext = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({
    data: { capture_token: "sha256:preview-token" },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: mockRefetchSourceContext,
  }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-test",
}));

vi.mock("@multica/core/issues/queries", () => ({
  sourceContextPreviewOptions: (wsId: string, anchorCommentId: string) => ({
    queryKey: ["source-context", "preview", wsId, anchorCommentId],
  }),
}));

vi.mock("@multica/core/issues/stores/draft-store", () => ({
  useIssueDraftStore: {
    getState: () => ({
      beginIsolatedDraft: mockBeginIsolatedDraft,
      endIsolatedDraft: mockEndIsolatedDraft,
    }),
  },
}));

const mockCreateModeStore = {
  lastMode: "agent" as "agent" | "manual",
  setLastMode: mockSetLastMode,
};

vi.mock("@multica/core/issues/stores/create-mode-store", () => ({
  useCreateModeStore: Object.assign(
    (selector: (s: typeof mockCreateModeStore) => unknown) =>
      selector(mockCreateModeStore),
    { getState: () => mockCreateModeStore },
  ),
}));

vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogContent: ({
    className,
    children,
  }: {
    className?: string;
    children: ReactNode;
  }) => (
    <div data-testid="dialog-content" className={className}>
      {children}
    </div>
  ),
}));

vi.mock("./quick-create-issue", () => ({
  AgentCreatePanel: ({
    data,
    onSwitchMode,
    onNeedsSpace,
    setIsExpanded,
  }: {
    data?: Record<string, unknown> | null;
    onNeedsSpace?: () => void;
    setIsExpanded?: (value: boolean) => void;
    onSwitchMode?: (carry?: Record<string, unknown> | null) => void;
  }) => (
    <div>
      <button onClick={onNeedsSpace}>request assist space</button>
      <button onClick={() => setIsExpanded?.(true)}>expand dialog</button>
      agent panel · {String(data?.anchor_comment_id ?? "ordinary")} · {data?.source_context_expanded ? "expanded" : "collapsed"}
      <button type="button" onClick={() => onSwitchMode?.({ parent_issue_id: data?.parent_issue_id })}>
        switch manual
      </button>
      <button
        type="button"
        onClick={() => (data?.source_context_on_expanded_change as ((expanded: boolean) => void) | undefined)?.(!data?.source_context_expanded)}
      >
        toggle source context
      </button>
    </div>
  ),
}));

vi.mock("./create-issue", () => ({
  ManualCreatePanel: ({
    data,
    onSwitchMode,
    onNeedsSpace,
    setIsExpanded,
  }: {
    data?: Record<string, unknown> | null;
    onNeedsSpace?: () => void;
    setIsExpanded?: (value: boolean) => void;
    onSwitchMode?: (carry?: Record<string, unknown> | null) => void;
  }) => (
    <div>
      <button onClick={onNeedsSpace}>request assist space</button>
      <button onClick={() => setIsExpanded?.(true)}>expand dialog</button>
      manual panel · {String(data?.anchor_comment_id ?? "ordinary")} · {data?.source_context_expanded ? "expanded" : "collapsed"}
      <output aria-label="iteration context">{String(data?.current_iteration_id ?? "none")}:{String(data?.expected_iteration_revision ?? "none")}</output>
      <button type="button" onClick={() => onSwitchMode?.({ parent_issue_id: data?.parent_issue_id })}>
        switch agent
      </button>
      <button
        type="button"
        onClick={() => (data?.source_context_on_expanded_change as ((expanded: boolean) => void) | undefined)?.(!data?.source_context_expanded)}
      >
        toggle source context
      </button>
    </div>
  ),
  manualDialogContentClass: (expanded: boolean, assist: boolean) => `manual-dialog-class size-${expanded ? "expanded" : assist ? "assist" : "compact"}`,
}));

// `cn` is deliberately NOT mocked here: the whole point of these assertions is
// that tailwind-merge keeps the phone cap and the `sm:` width as two separate
// groups instead of collapsing them into one max-width.
import { CreateIssueDialog } from "./create-issue-dialog";

function contentClass() {
  return screen.getByTestId("dialog-content").className;
}

describe("CreateIssueDialog sizing", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  // Concrete dimensions are covered by create-issue.test and browser QA.
  // This suite owns shell-to-panel layout state wiring.
  it.each(["agent", "manual"] as const)("expands %s once for assistance without undoing user expansion", (mode) => {
    render(<CreateIssueDialog onClose={vi.fn()} initialMode={mode} />);
    expect(contentClass()).toContain("size-compact");
    fireEvent.click(screen.getByRole("button", { name: "request assist space" }));
    expect(contentClass()).toContain("size-assist");
    fireEvent.click(screen.getByRole("button", { name: "expand dialog" }));
    fireEvent.click(screen.getByRole("button", { name: "request assist space" }));
    expect(contentClass()).toContain("size-expanded");
  });

  it("leaves ordinary create outside the isolated source-context path", () => {
    render(
      <CreateIssueDialog
        onClose={vi.fn()}
        initialMode="agent"
        data={{ parent_issue_id: "ordinary-parent" }}
      />,
    );

    expect(screen.getByText(/agent panel · ordinary/)).toBeInTheDocument();
    expect(mockBeginIsolatedDraft).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "switch manual" }));
    expect(screen.getByText(/manual panel · ordinary/)).toBeInTheDocument();
  });

  it("keeps a requested iteration in manual mode instead of dropping context on an agent switch", () => {
    render(<CreateIssueDialog onClose={vi.fn()} initialMode="agent" data={{
      workspace_id: "ws-test", current_iteration_id: "iteration-1", expected_iteration_revision: 7,
    }} />);
    expect(screen.getByText(/manual panel · ordinary/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "switch agent" }));
    expect(screen.queryByText(/agent panel · ordinary/)).not.toBeInTheDocument();
    expect(mockSetLastMode).not.toHaveBeenCalled();
  });

  it("keeps the reviewed revision for the same period and resets context for a different period", () => {
    const seed = { workspace_id: "ws-test", current_iteration_id: "iteration-1", expected_iteration_revision: 7 };
    const view = render(<CreateIssueDialog onClose={vi.fn()} initialMode="manual" data={seed} />);
    view.rerender(<CreateIssueDialog onClose={vi.fn()} initialMode="manual" data={{ ...seed, expected_iteration_revision: 8 }} />);
    expect(screen.getByLabelText("iteration context")).toHaveTextContent("iteration-1:7");
    view.rerender(<CreateIssueDialog onClose={vi.fn()} initialMode="manual" data={{ ...seed, current_iteration_id: "iteration-2", expected_iteration_revision: 2 }} />);
    expect(screen.getByLabelText("iteration context")).toHaveTextContent("iteration-2:2");
  });

  it("isolates source-context drafts and preserves source identity across mode switches", () => {
    const view = render(
      <CreateIssueDialog
        onClose={vi.fn()}
        initialMode="agent"
        data={{ anchor_comment_id: "comment-source", parent_issue_id: "parent-source" }}
      />,
    );

    expect(mockBeginIsolatedDraft).toHaveBeenCalledTimes(1);
    expect(screen.getByText(/agent panel · comment-source/)).toBeInTheDocument();
    expect(contentClass()).toContain("size-compact");
    fireEvent.click(screen.getByRole("button", { name: "toggle source context" }));
    expect(screen.getByText(/agent panel · comment-source · expanded/)).toBeInTheDocument();
    expect(contentClass()).toContain("!h-5/6");
    fireEvent.click(screen.getByRole("button", { name: "switch manual" }));
    expect(screen.getByText(/manual panel · comment-source · expanded/)).toBeInTheDocument();
    expect(contentClass()).toContain("manual-dialog-class");
    expect(contentClass()).toContain("!h-5/6");
    fireEvent.click(screen.getByRole("button", { name: "switch agent" }));
    expect(screen.getByText(/agent panel · comment-source/)).toBeInTheDocument();

    view.unmount();
    expect(mockEndIsolatedDraft).toHaveBeenCalledTimes(1);
  });
});
