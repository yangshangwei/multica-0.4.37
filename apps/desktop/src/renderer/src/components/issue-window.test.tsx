import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AuthRecoveryState } from "@multica/core/auth";

const state = vi.hoisted(() => ({
  user: { id: "user-1" } as {
    id: string;
    requires_account_setup?: boolean;
    requires_password_change?: boolean;
  },
  workspaces: [] as { id: string; slug: string }[],
  ready: false,
  unavailable: false,
  isFetching: false,
  error: new TypeError("offline"),
  errorUpdatedAt: 1_792_000_000_000,
  refetch: vi.fn(),
}));

vi.mock("@multica/core/auth", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/auth")>();
  const auth = { get user() { return state.user; } };
  return {
    ...actual,
    useAuthStore: Object.assign(
      (selector: (value: typeof auth) => unknown) => selector(auth),
      { getState: () => auth },
    ),
  };
});

vi.mock("@multica/core/workspace", () => ({
  useWorkspaceList: () => ({
    workspaces: state.workspaces,
    ready: state.ready,
    unavailable: state.unavailable,
    isFetching: state.isFetching,
    error: state.error,
    errorUpdatedAt: state.errorUpdatedAt,
    refetch: state.refetch,
  }),
}));

vi.mock("@multica/core/platform", () => ({
  setCurrentWorkspace: vi.fn(),
}));

vi.mock("@multica/core/paths", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/paths")>()),
  WorkspaceSlugProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("@multica/views/collaboration", () => ({
  AccountWorkbenchPage: ({ destination }: { destination: { issueId: string } }) => (
    <div data-testid="account-detail">{destination.issueId}</div>
  ),
}));

vi.mock("@multica/views/i18n", () => ({
  useT: () => ({ t: (select: (messages: { page: { title: string } }) => string) => select({ page: { title: "Collaboration" } }) }),
}));

vi.mock("@multica/ui/components/common/multica-icon", () => ({
  MulticaIcon: () => <div data-testid="workspace-loading" />,
}));

vi.mock("@multica/views/modals/registry", () => ({
  ModalRegistry: () => null,
}));

vi.mock("@multica/views/layout", () => ({
  WorkspacePresencePrefetch: () => null,
}));

vi.mock("@multica/views/platform", () => ({
  DragStrip: () => null,
}));

vi.mock("../pages/issue-detail-page", () => ({
  IssueDetailPage: () => <div data-testid="issue-detail" />,
}));

// Boundary behavior and main-window handoff live in connection-recovery-boundary.test.tsx.
vi.mock("./connection-recovery-boundary", () => ({
  DesktopConnectionRecoveryBoundary: ({
    onRetry,
    recovery,
    children,
  }: {
    onRetry: () => void;
    recovery: AuthRecoveryState | null;
    children: React.ReactNode;
  }) => recovery ? (
    <button type="button" disabled={recovery.isRetrying} onClick={onRetry}>
      Retry workspace list
    </button>
  ) : children,
}));

vi.mock("../platform/issue-window-navigation", () => ({
  IssueWindowNavigationProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

import { IssueWindow } from "./issue-window";
import { parseIssueWindowRequest } from "../../../shared/issue-window";
import { setCurrentWorkspace } from "@multica/core/platform";

const context = {
  kind: "issue" as const,
  scope: "workspace" as const,
  path: "/acme/issues/MUL-1",
  title: "MUL-1",
  workspaceSlug: "acme",
  issueId: "MUL-1",
};

beforeEach(() => {
  state.user = { id: "user-1" };
  vi.mocked(setCurrentWorkspace).mockClear();
  state.workspaces = [];
  state.ready = false;
  state.unavailable = false;
  state.isFetching = false;
  state.refetch.mockReset().mockResolvedValue({});
});

describe("IssueWindow", () => {
  it("opens account details during a workspace-list failure and clears the workspace mirror", async () => {
    state.unavailable = true;
    const account = parseIssueWindowRequest({
      path: "/account?view=current&issue=11111111-1111-4111-8111-111111111111",
      title: "Shared task",
    });
    if (!account) throw new Error("Expected a valid account issue window");

    render(<IssueWindow context={account} />);

    expect(await screen.findByTestId("account-detail")).toHaveTextContent(account.issueId);
    expect(setCurrentWorkspace).toHaveBeenLastCalledWith(null, null);
    expect(screen.queryByRole("button", { name: "Retry workspace list" })).toBeNull();
  });

  it.each(["requires_account_setup", "requires_password_change"] as const)("keeps %s authoritative for account popouts", (guard) => {
    state.user = { id: "user-1", [guard]: true };
    const account = parseIssueWindowRequest({
      path: "/account?view=current&issue=11111111-1111-4111-8111-111111111111",
      title: "Shared task",
    });
    if (!account) throw new Error("Expected a valid account issue window");

    render(<IssueWindow context={account} />);

    expect(screen.queryByTestId("account-detail")).toBeNull();
  });

  it("keeps loading after an initial workspace-list failure", async () => {
    render(<IssueWindow context={context} />);

    expect(await screen.findByTestId("workspace-loading")).toBeInTheDocument();
    expect(screen.queryByText("Issue unavailable")).toBeNull();
  });

  it("shows unavailable only after an authoritative list omits the workspace", async () => {
    state.ready = true;

    render(<IssueWindow context={context} />);

    expect(await screen.findByText("Issue unavailable")).toBeInTheDocument();
    expect(screen.queryByTestId("workspace-loading")).toBeNull();
  });

  it("offers a retry when the initial workspace-list request fails", async () => {
    state.unavailable = true;

    render(<IssueWindow context={context} />);

    const retry = await screen.findByRole("button", {
      name: "Retry workspace list",
    });
    fireEvent.click(retry);
    expect(state.refetch).toHaveBeenCalledOnce();
    expect(state.refetch).toHaveBeenCalledWith({ cancelRefetch: false });
    expect(screen.queryByTestId("workspace-loading")).toBeNull();
  });
});
