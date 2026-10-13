import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { IssueStatusEntry, ListIssueStatusesResponse } from "@multica/core/types";
import { issueStatusKeys } from "@multica/core/issue-statuses/queries";
import { ALL_STATUSES } from "@multica/core/issues/config";
import { renderWithI18n } from "../../test/i18n";
import { IssueStatusesTab } from "./issue-statuses-tab";

const api = vi.hoisted(() => ({
  listIssueStatuses: vi.fn(),
  reorderIssueStatuses: vi.fn(),
}));
vi.mock("@multica/core/api", () => ({ api }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return {
    useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }),
  };
});
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({
    queryKey: ["members", "ws-1"],
    queryFn: async () => [{ user_id: "user-1", role: "owner" }],
    initialData: [{ user_id: "user-1", role: "owner" }],
    staleTime: Infinity,
  }),
}));

function status(id: string, position: number, overrides: Partial<IssueStatusEntry> = {}): IssueStatusEntry {
  return {
    id, key: id, name: id, position, workspace_id: "ws-1", description: "",
    category: "in_review", color: "#6366f1", is_system: false, archived_at: null,
    created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

const retired = status("retired", 2, { archived_at: "2026-02-01T00:00:00Z" });
const catalog: ListIssueStatusesResponse = {
  statuses: [status("in_review", 0, { is_system: true }), status("A", 1), retired, status("B", 3), status("QA", 4)],
  categories: [...ALL_STATUSES],
  total: 5,
};
const key = issueStatusKeys.list("ws-1");
let client: QueryClient;

function mount() {
  return renderWithI18n(<QueryClientProvider client={client}><IssueStatusesTab /></QueryClientProvider>);
}

beforeEach(() => {
  vi.clearAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  client.setQueryData(key, catalog);
  api.listIssueStatuses.mockResolvedValue(catalog);
});
afterEach(() => { cleanup(); client.clear(); });

describe("Status ordering with the real catalog mutation", () => {
  it("prevents normalization from moving active rows across displayed archived positions", async () => {
    api.reorderIssueStatuses.mockReturnValue(new Promise(() => {}));
    mount();
    fireEvent.click(screen.getByRole("switch"));
    const move = screen.getByRole("button", { name: "Move QA up" });

    // Before the guard, QA could swap with B, but the real mutation normalized
    // all active positions to 1..3 and put QA before the archived position 2.
    await act(async () => fireEvent.click(move));
    expect(client.getQueryData(key)).toEqual(catalog);
    expect(api.reorderIssueStatuses).not.toHaveBeenCalled();
    expect(move).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByRole("button", { name: "Reorder QA" })).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText("Hide archived statuses to reorder active statuses.")).toBeInTheDocument();
  });

  it("re-enables normal ordering after hiding archived rows and retains the frozen archived entry", async () => {
    let resolve!: (data: ListIssueStatusesResponse) => void;
    api.reorderIssueStatuses.mockImplementation(() => new Promise((done) => { resolve = done; }));
    mount();
    const toggle = screen.getByRole("switch");
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    const move = screen.getByRole("button", { name: "Move QA up" });
    move.focus();
    fireEvent.click(move);
    await waitFor(() => expect(api.reorderIssueStatuses).toHaveBeenCalledWith("in_review", ["A", "QA", "B"]));
    const cached = client.getQueryData<ListIssueStatusesResponse>(key)!;
    expect(cached.statuses.filter((entry) => !entry.archived_at && !entry.is_system).map(({ id, position }) => ({ id, position })))
      .toEqual([{ id: "A", position: 1 }, { id: "QA", position: 2 }, { id: "B", position: 3 }]);
    expect(cached.statuses.find((entry) => entry.id === retired.id)).toEqual(retired);
    expect(screen.queryByText("retired")).toBeNull();
    expect(move).toHaveFocus();
    expect(screen.getByRole("button", { name: "Move QA down" })).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByRole("button", { name: "Move QA down" }));
    expect(api.reorderIssueStatuses).toHaveBeenCalledTimes(1);

    await act(async () => resolve(cached));
    expect(await screen.findByText("QA moved to position 2 in its category.")).toHaveAttribute("role", "status");
    expect(move).toHaveFocus();
  });
});
