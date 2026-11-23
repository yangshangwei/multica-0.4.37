import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api";
import { issueStatusKeys } from "@multica/core/issue-statuses/queries";
import { workspaceKeys } from "@multica/core/workspace/queries";
import type { IssueStatusEntry } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { IssueStatusesTab } from "./issue-statuses-tab";

const api = vi.hoisted(() => ({ listIssueStatuses: vi.fn(), listMembers: vi.fn() }));
const auth = vi.hoisted(() => ({ user: { id: "u-1" } }));
vi.mock("@multica/core/api", async (original) => ({
  ...(await original<typeof import("@multica/core/api")>()),
  api,
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof auth) => unknown) => selector(auth),
    { getState: () => auth },
  ),
}));
vi.mock("@multica/core/issue-statuses/mutations", () => ({
  useCreateIssueStatus: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateIssueStatus: () => ({ mutate: vi.fn(), isPending: false }),
  useArchiveIssueStatus: () => ({ mutate: vi.fn(), isPending: false }),
  useReorderIssueStatuses: () => ({ mutate: vi.fn(), isPending: false }),
}));

const status: IssueStatusEntry = {
  id: "qa", workspace_id: "ws-1", key: "qa", name: "Quality review",
  description: "", category: "in_review", color: "#336699", is_system: false,
  position: 1, archived_at: null, created_at: "2026-10-01", updated_at: "2026-10-01",
};
const key = issueStatusKeys.list("ws-1");
let client: QueryClient;

function mount() {
  return renderWithI18n(
    <QueryClientProvider client={client}><IssueStatusesTab /></QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.resetAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  client.setQueryData(workspaceKeys.members("ws-1"), [{ user_id: "u-1", role: "owner" }]);
  api.listMembers.mockResolvedValue([{ user_id: "u-1", role: "owner" }]);
});
afterEach(() => { cleanup(); client.clear(); });

describe("Status catalog recovery with real catalog observers", () => {
  it("settles a persistent initial failure without category-mount request loops", async () => {
    let reads = 0;
    api.listIssueStatuses.mockImplementation(() => ++reads <= 10
      ? Promise.reject(new ApiError("offline", 500, "Server error"))
      : new Promise(() => {}));
    mount();
    // Allow several notification/render cycles. The old category observers
    // remounted after each error and started another request despite retry:false.
    for (let cycle = 0; cycle < 8; cycle++) {
      await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
    }
    expect(api.listIssueStatuses).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load statuses");
    expect(screen.queryByRole("button", { name: "Add status" })).toBeNull();
  });

  it("keeps the same cold Retry button disabled until the catalog recovers", async () => {
    let resolve!: (value: { statuses: IssueStatusEntry[] }) => void;
    api.listIssueStatuses
      .mockRejectedValueOnce(new Error("offline"))
      .mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    mount();
    const retry = await screen.findByRole("button", { name: "Retry" });
    fireEvent.click(retry);
    await waitFor(() => expect(retry).toBeDisabled());
    expect(screen.getByRole("button", { name: "Retrying..." })).toBe(retry);
    fireEvent.click(retry);
    expect(api.listIssueStatuses).toHaveBeenCalledTimes(2);
    await act(async () => resolve({ statuses: [status] }));
    expect(await screen.findByText("Quality review")).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("retains an open editor and draft after a cached catalog refresh fails", async () => {
    client.setQueryData(key, { statuses: [status] });
    api.listIssueStatuses.mockRejectedValue(new Error("offline"));
    mount();
    fireEvent.click(screen.getAllByRole("button", { name: "Add status" })[0]!);
    const name = await screen.findByRole("textbox", { name: "Name" });
    expect(screen.getByRole("combobox", { name: "Category" })).toBeVisible();
    fireEvent.change(name, { target: { value: "Pending draft" } });
    name.focus();
    await act(async () => { await client.invalidateQueries({ queryKey: key }); });
    expect(await screen.findByRole("alert", { hidden: true })).toHaveTextContent("Could not refresh statuses");
    expect(screen.getByRole("textbox", { name: "Name" })).toBe(name);
    expect(name).toHaveValue("Pending draft");
    expect(name).toHaveFocus();
  });

  it("does not restore denied catalog controls after Retry fails or the tab remounts", async () => {
    client.setQueryData(key, { statuses: [status] });
    api.listIssueStatuses
      .mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"))
      .mockRejectedValueOnce(new ApiError("offline", 500, "Unavailable"))
      .mockImplementationOnce(() => new Promise(() => {}));
    const view = mount();
    expect(screen.getByText("Quality review")).toBeVisible();
    await act(async () => { await client.invalidateQueries({ queryKey: key }); });
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.queryByText("Quality review")).toBeNull();
    expect(screen.queryByRole("button", { name: "Add status" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(api.listIssueStatuses).toHaveBeenCalledTimes(2));
    expect(screen.queryByText("Quality review")).toBeNull();
    expect(screen.queryByRole("button", { name: "Add status" })).toBeNull();
    view.unmount();
    mount();
    await waitFor(() => expect(api.listIssueStatuses).toHaveBeenCalledTimes(3));
    expect(screen.queryByText("Quality review")).toBeNull();
    expect(screen.queryByRole("button", { name: "Add status" })).toBeNull();
  });
});
