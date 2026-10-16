import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MemberWithUser, Workspace } from "@multica/core/types";
import { ApiError } from "@multica/core/api";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { renderWithI18n } from "../../test/i18n";

const requests = vi.hoisted(() => ({
  listMembers: vi.fn(), listWorkspaces: vi.fn(), updateWorkspace: vi.fn(),
  leaveWorkspace: vi.fn(), deleteWorkspace: vi.fn(),
  getBaseUrl: () => "https://api.example.com",
}));
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
const navigation = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(), api: requests,
}));
vi.mock("sonner", () => ({ toast: notices }));
vi.mock("../../navigation", () => ({ useNavigation: () => navigation }));
vi.mock("@multica/core/paths", async () => {
  const { useQuery } = await import("@tanstack/react-query");
  const { workspaceListOptions } = await import("@multica/core/workspace/queries");
  return {
    useCurrentWorkspace: () => useQuery(workspaceListOptions()).data?.[0] ?? null,
    useHasOnboarded: () => true,
    resolvePostAuthDestination: () => "/",
  };
});
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return { useAuthStore: Object.assign(
    (selector: (value: typeof state) => unknown) => selector(state),
    { getState: () => state },
  ) };
});
vi.mock("@multica/core/hooks/use-file-upload", () => ({
  useFileUpload: () => ({ upload: async () => ({ link: "/workspace-avatar.png" }) }),
}));
// The real upload control owns crop retention; canvas processing has its own
// canonical coverage in common/avatar-crop.test.ts.
vi.mock("../../common/avatar-crop-dialog", () => ({
  AvatarCropDialog: ({ open, busy, onCropped }: {
    open: boolean; busy: boolean; onCropped: (file: File) => void;
  }) => open ? <button disabled={busy} onClick={() => onCropped(new File(["image"], "logo.png", { type: "image/png" }))}>Save cropped logo</button> : null,
}));
// WorkspacePlanningTimezone owns a separate query and its dedicated test suite.
vi.mock("./workspace-planning-timezone", () => ({ WorkspacePlanningTimezone: () => null }));

import { WorkspaceTab } from "./workspace-tab";

const workspace: Workspace = {
  id: "workspace-1", name: "Acme", slug: "acme", description: "Original description", context: "Original context",
  settings: {}, issue_prefix: "ACM", avatar_url: null, repos: [],
  created_at: "2026-10-14T00:00:00Z", updated_at: "2026-10-14T00:00:00Z",
};
const owner: MemberWithUser = {
  id: "member-1", user_id: "user-1", workspace_id: workspace.id, name: "Alex",
  email: "alex@example.com", role: "owner", avatar_url: null, created_at: "2026-10-14T00:00:00Z",
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

let client: QueryClient;
beforeEach(() => {
  vi.resetAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } } });
  client.setQueryData(workspaceKeys.list(), [workspace]);
  requests.listMembers.mockResolvedValue([owner]);
  requests.listWorkspaces.mockResolvedValue([workspace]);
  requests.updateWorkspace.mockImplementation(async (_id: string, patch: Partial<Workspace>) => ({ ...workspace, ...patch }));
});
afterEach(() => { cleanup(); client.clear(); });
function renderTab() {
  return renderWithI18n(<QueryClientProvider client={client}><WorkspaceTab /></QueryClientProvider>);
}

describe("WorkspaceTab permissions and save recovery", () => {
  it("keeps unknown membership neutral and blocks leave until a successful read", async () => {
    requests.listMembers.mockRejectedValueOnce(new Error("offline"));
    const user = userEvent.setup();
    renderTab();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load workspace permissions.");
    expect(screen.queryByRole("button", { name: "Leave workspace" })).not.toBeInTheDocument();
    expect(screen.queryByText("Only admins and owners can update workspace settings.")).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Name" })).toBeDisabled();
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByRole("textbox", { name: "Name" })).toBeEnabled());
    expect(screen.getByRole("button", { name: "Leave workspace" })).toBeDisabled();
    expect(requests.leaveWorkspace).not.toHaveBeenCalled();
  });

  it("does not authorize leave or management for an unrecognized membership role", async () => {
    requests.listMembers.mockResolvedValueOnce([{ ...owner, role: "unknown" }]);
    renderTab();
    await waitFor(() => expect(client.getQueryState(workspaceKeys.members(workspace.id))?.status).toBe("success"));
    expect(screen.getByRole("textbox", { name: "Name" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Leave workspace" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete workspace" })).not.toBeInTheDocument();
  });

  it("retains an authorized form on transient refresh but removes management after access denial", async () => {
    renderTab();
    const name = screen.getByRole("textbox", { name: "Name" });
    await waitFor(() => expect(name).toBeEnabled());
    fireEvent.change(name, { target: { value: "" } });
    const context = screen.getByRole("textbox", { name: "Context" });
    fireEvent.change(context, { target: { value: "Retained workspace context" } });
    requests.listMembers.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.members(workspace.id) }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh workspace permissions.");
    expect(screen.getByRole("textbox", { name: "Context" })).toBe(context);
    expect(context).toHaveValue("Retained workspace context");
    expect(name).toBeEnabled();

    requests.listMembers.mockRejectedValueOnce(new ApiError("Forbidden", 403, "Forbidden"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.members(workspace.id) }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("You do not have access to these settings.");
    expect(screen.queryByRole("button", { name: "Delete workspace" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Leave workspace" })).not.toBeInTheDocument();
  });

  it("keeps description and context drafts while a required name is corrected", async () => {
    renderTab();
    const name = screen.getByRole("textbox", { name: "Name" });
    await waitFor(() => expect(name).toBeEnabled());
    const description = screen.getByRole("textbox", { name: "Description" });
    const context = screen.getByRole("textbox", { name: "Context" });
    fireEvent.change(name, { target: { value: " " } });
    fireEvent.change(description, { target: { value: "Retained description" } });
    fireEvent.change(context, { target: { value: "Retained context" } });
    fireEvent.blur(context);
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAccessibleDescription("Enter a workspace name to save these changes.");
    expect(screen.getByRole("status")).toHaveTextContent("Unsaved changes");
    expect(requests.updateWorkspace).not.toHaveBeenCalled();

    fireEvent.change(name, { target: { value: "Acme Updated" } });
    fireEvent.blur(name);
    await waitFor(() => expect(requests.updateWorkspace).toHaveBeenCalledWith(workspace.id, {
      name: "Acme Updated", description: "Retained description", context: "Retained context",
    }));
    expect(screen.getByRole("textbox", { name: "Description" })).toBe(description);
    expect(screen.getByRole("textbox", { name: "Context" })).toBe(context);
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Saved"));
  });

  it("keeps a failed prefix confirmation open and rejects duplicate pending confirmations", async () => {
    const pending = deferred<Workspace>();
    requests.updateWorkspace.mockReturnValueOnce(pending.promise);
    const user = userEvent.setup();
    renderTab();
    const prefix = screen.getByPlaceholderText("ACM");
    await waitFor(() => expect(prefix).toBeEnabled());
    fireEvent.change(prefix, { target: { value: "NEW" } });
    fireEvent.blur(prefix);
    const dialog = await screen.findByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", { name: "Confirm" });
    await user.click(confirm);
    expect(confirm).toBeDisabled();
    await user.click(confirm);
    await user.keyboard("{Escape}");
    expect(dialog).toBeInTheDocument();
    expect(requests.updateWorkspace).toHaveBeenCalledTimes(1);

    await act(async () => pending.reject(new Error("Prefix save failed")));
    await waitFor(() => expect(notices.error).toHaveBeenCalledWith("Prefix save failed"));
    expect(dialog).toBeInTheDocument();
    expect(prefix).toHaveValue("NEW");
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(requests.updateWorkspace).toHaveBeenCalledTimes(2);
    expect(client.getQueryData<Workspace[]>(workspaceKeys.list())?.[0]?.issue_prefix).toBe("NEW");
  });

  it("keeps the crop workflow open when the workspace logo write fails", async () => {
    requests.updateWorkspace.mockRejectedValueOnce(new Error("Logo save failed"));
    const { container } = renderTab();
    await waitFor(() => expect(screen.getByRole("textbox", { name: "Name" })).toBeEnabled());
    const input = container.querySelector<HTMLInputElement>('input[type="file"]');
    if (!input) throw new Error("Logo input missing");
    fireEvent.change(input, { target: { files: [new File(["image"], "logo.png", { type: "image/png" })] } });
    fireEvent.click(screen.getByRole("button", { name: "Save cropped logo" }));
    await waitFor(() => expect(notices.error).toHaveBeenCalledWith("Logo save failed"));
    expect(notices.error).toHaveBeenCalledTimes(1);
    expect(notices.success).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Save cropped logo" })).toBeEnabled();
    expect(client.getQueryData<Workspace[]>(workspaceKeys.list())?.[0]?.avatar_url).toBeNull();
  });
});
