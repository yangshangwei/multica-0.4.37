/**
 * @vitest-environment jsdom
 */
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { IssueStatusEntry } from "@multica/core/types";
import en from "../../locales/en/settings.json";
import enIssues from "../../locales/en/issues.json";
import { IssueStatusesTab } from "./issue-statuses-tab";

const reorderMutate = vi.hoisted(() => vi.fn());
let catalog: IssueStatusEntry[] = [];
let role: string = "owner";

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey: readonly unknown[] }) => ({
    data: options.queryKey[0] === "issue-statuses" ? catalog : members(),
    isLoading: false,
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (s: unknown) => unknown) => selector({ user: { id: "u-1" } }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members", "ws-1"] }),
}));
// Only the fetch is stubbed. The module's pure helpers (`issueStatusColor`)
// are what the rows render with, and a stub of those would test the stub.
vi.mock("@multica/core/issue-statuses/queries", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/issue-statuses/queries")>()),
  issueStatusListOptions: () => ({ queryKey: ["issue-statuses", "ws-1"] }),
}));
vi.mock("@multica/core/issue-statuses/mutations", () => ({
  useCreateIssueStatus: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateIssueStatus: () => ({ mutate: vi.fn(), isPending: false }),
  useArchiveIssueStatus: () => ({ mutate: vi.fn() }),
  useReorderIssueStatuses: () => ({ mutate: reorderMutate }),
}));
vi.mock("../../issues/utils/status-label", () => ({
  useStatusLabel: () => (key: string) => key,
}));
vi.mock("../../i18n", () => ({
  useT: (namespace: string) => ({
    t: (accessor: (dict: unknown) => string, params?: Record<string, unknown>) => {
      const template = accessor(namespace === "issues" ? enIssues : en);
      if (!params) return template;
      return template.replace(/\{\{(\w+)\}\}/g, (_, k: string) => String(params[k] ?? ""));
    },
  }),
}));

function members() {
  return [{ user_id: "u-1", role }];
}

function entry(overrides: Partial<IssueStatusEntry>): IssueStatusEntry {
  return {
    id: overrides.key ?? "id",
    workspace_id: "ws-1",
    key: "custom",
    name: "Custom",
    description: "",
    category: "in_review",
    color: "#ff0000",
    is_system: false,
    position: 1,
    archived_at: null,
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

const BUILT_IN_IN_REVIEW = entry({
  id: "in_review",
  key: "in_review",
  name: "In Review",
  is_system: true,
  position: 0,
});

afterEach(() => {
  cleanup();
  reorderMutate.mockClear();
  catalog = [];
  role = "owner";
});

describe("IssueStatusesTab", () => {
  // Creation answers to workspace role alone since MUL-6643 removed the
  // rollout flag; an owner gets the affordance on every deployment.
  it("offers status creation to an owner", () => {
    catalog = [BUILT_IN_IN_REVIEW];
    render(<IssueStatusesTab />);

    expect(screen.getAllByLabelText(en.issue_statuses.add).length).toBeGreaterThan(0);
  });

  it("hides creation and row actions from a non-admin member", () => {
    catalog = [BUILT_IN_IN_REVIEW, entry({ key: "qa", name: "QA" })];
    role = "member";
    render(<IssueStatusesTab />);

    expect(screen.queryByLabelText(en.issue_statuses.add)).toBeNull();
    expect(screen.getByText("QA")).toBeInTheDocument();
    expect(
      screen.queryByLabelText(
        en.issue_statuses.actions.open.replace("{{name}}", "QA"),
      ),
    ).toBeNull();
  });

  // Archiving retires a status from FUTURE assignment; the row has to stay
  // legible so an admin can tell what a lingering status on an old issue is.
  it("keeps an archived status visible but not actionable", () => {
    catalog = [
      BUILT_IN_IN_REVIEW,
      entry({ key: "qa", name: "QA", archived_at: "2026-01-01T00:00:00Z" }),
    ];
    render(<IssueStatusesTab />);

    // Hidden until the toggle is on, but the toggle itself is enabled because
    // the workspace has one.
    expect(screen.queryByText("QA")).toBeNull();
    expect(screen.getByRole("switch")).toBeEnabled();
  });

  // The category header used to repeat the sentence its built-in row already
  // carried, so every category said the same thing twice. (MUL-6422)
  it("states a category's behavior once, on its built-in row", () => {
    catalog = [BUILT_IN_IN_REVIEW];
    render(<IssueStatusesTab />);

    expect(screen.getAllByText(en.issue_statuses.categories.in_review)).toHaveLength(1);
  });

  // A toggle that can only ever reveal nothing is not worth a row of chrome.
  it("offers the archived toggle only once something is archived", () => {
    catalog = [BUILT_IN_IN_REVIEW, entry({ key: "qa", name: "QA" })];
    render(<IssueStatusesTab />);

    expect(screen.queryByRole("switch")).toBeNull();
  });

  it("does not offer reorder when a category holds a single custom status", () => {
    catalog = [BUILT_IN_IN_REVIEW, entry({ key: "qa", name: "QA" })];
    render(<IssueStatusesTab />);

    expect(
      screen.queryByLabelText(
        en.issue_statuses.actions.reorder.replace("{{name}}", "QA"),
      ),
    ).toBeNull();
  });

  it("offers reorder once a category holds two", () => {
    catalog = [
      BUILT_IN_IN_REVIEW,
      entry({ id: "qa", key: "qa", name: "QA", position: 1 }),
      entry({ id: "uat", key: "uat", name: "UAT", position: 2 }),
    ];
    render(<IssueStatusesTab />);

    expect(
      screen.getByLabelText(en.issue_statuses.actions.reorder.replace("{{name}}", "QA")),
    ).toBeInTheDocument();
  });

  it("moves by keyboard, prevents repeat submissions and keeps focus at the boundary", async () => {
    const user = userEvent.setup();
    catalog = [
      BUILT_IN_IN_REVIEW,
      entry({ key: "qa", name: "QA", position: 1 }),
      entry({ key: "uat", name: "UAT", position: 2 }),
    ];
    render(<IssueStatusesTab />);

    const move = screen.getByRole("button", { name: "Move UAT up" });
    move.focus();
    await user.keyboard("{Enter}");
    expect(reorderMutate).toHaveBeenCalledTimes(1);
    expect(reorderMutate.mock.calls[0]![0]).toMatchObject({
      category: "in_review", ordered: [{ id: "uat" }, { id: "qa" }],
    });
    expect(move).toHaveFocus();
    expect(move).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByRole("button", { name: "Move UAT down" }));
    expect(reorderMutate).toHaveBeenCalledTimes(1);

    act(() => {
      reorderMutate.mock.calls[0]![1].onSuccess();
      reorderMutate.mock.calls[0]![1].onSettled();
    });
    await waitFor(() => expect(screen.getByText(/UAT moved to position 1/)).toHaveAttribute("role", "status"));
    expect(move).toHaveFocus();
    await user.keyboard(" ");
    expect(reorderMutate).toHaveBeenCalledTimes(1);
  });

  it("keeps category boundaries and archived rows out of the order mutation", async () => {
    catalog = [
      BUILT_IN_IN_REVIEW,
      entry({ key: "qa", name: "QA", position: 1 }),
      entry({ key: "uat", name: "UAT", position: 2 }),
      entry({ key: "retired", name: "Retired", position: 3, archived_at: "2026-01-01" }),
      entry({ key: "todo_custom", name: "Queued", category: "todo" }),
    ];
    render(<IssueStatusesTab />);
    await userEvent.click(screen.getByRole("switch"));
    expect(screen.getByText("Retired")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Move Retired up" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Move Queued up" })).toBeNull();
    const boundary = screen.getByRole("button", { name: "Move QA up" });
    expect(boundary).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(boundary);
    expect(reorderMutate).not.toHaveBeenCalled();
    // Mixed active/archived views are read-only for ordering. The real Query
    // normalization regression lives in issue-statuses-ordering.test.tsx.
    await userEvent.click(screen.getByRole("switch"));
    fireEvent.click(screen.getByRole("button", { name: "Move QA down" }));
    expect(reorderMutate.mock.calls[0]![0]).toMatchObject({
      ordered: [{ id: "uat" }, { id: "qa" }],
    });
    expect(screen.queryByText("Retired")).toBeNull();
  });

  it("restores the prior order and announces a failed save without moving focus", () => {
    catalog = [
      BUILT_IN_IN_REVIEW,
      entry({ key: "qa", name: "QA", position: 1 }),
      entry({ key: "uat", name: "UAT", position: 2 }),
    ];
    render(<IssueStatusesTab />);
    const move = screen.getByRole("button", { name: "Move UAT up" });
    move.focus();
    fireEvent.click(move);
    act(() => {
      reorderMutate.mock.calls[0]![1].onError(new Error("offline"));
      reorderMutate.mock.calls[0]![1].onSettled();
    });
    expect(screen.getByText(en.issue_statuses.reorder_failed)).toHaveAttribute("role", "status");
    expect(move).toHaveFocus();
    expect(move).toHaveAttribute("aria-disabled", "false");
    expect(screen.getByRole("button", { name: "Move QA up" })).toHaveAttribute("aria-disabled", "true");
  });

  it("cannot move across a visible archived row", async () => {
    catalog = [
      BUILT_IN_IN_REVIEW,
      entry({ key: "qa", name: "QA", position: 1 }),
      entry({ key: "retired", name: "Retired", position: 2, archived_at: "2026-01-01" }),
      entry({ key: "uat", name: "UAT", position: 3 }),
    ];
    render(<IssueStatusesTab />);
    await userEvent.click(screen.getByRole("switch"));
    const before = screen.getByRole("button", { name: "Move QA down" });
    const after = screen.getByRole("button", { name: "Move UAT up" });
    expect(before).toHaveAttribute("aria-disabled", "true");
    expect(after).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(before);
    fireEvent.click(after);
    expect(reorderMutate).not.toHaveBeenCalled();
  });
});
