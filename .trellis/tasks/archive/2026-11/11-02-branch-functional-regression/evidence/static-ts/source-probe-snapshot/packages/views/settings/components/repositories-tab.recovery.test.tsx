import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import type { MemberWithUser, Workspace } from "@multica/core/types";
import { githubKeys } from "@multica/core/github";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { renderWithI18n } from "../../test/i18n";

const requests = vi.hoisted(() => ({
  listMembers: vi.fn(),
  listWorkspaces: vi.fn(),
  listGitHubInstallations: vi.fn(),
  listGitHubInstallationRepositories: vi.fn(),
  getGitHubConnectURL: vi.fn(),
  updateWorkspace: vi.fn(),
}));
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
const navigation = vi.hoisted(() => ({
  pathname: "/acme/settings",
  searchParams: new URLSearchParams("tab=repositories"),
  replace: vi.fn(),
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(),
  api: requests,
}));
vi.mock("sonner", () => ({ toast: notices }));
vi.mock("../../navigation", () => ({ useNavigation: () => navigation }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", async () => {
  const { useQuery } = await import("@tanstack/react-query");
  const { workspaceListOptions } = await import("@multica/core/workspace/queries");
  return { useCurrentWorkspace: () => useQuery(workspaceListOptions()).data?.[0] ?? null };
});
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return {
    useAuthStore: Object.assign(
      (selector: (value: typeof state) => unknown) => selector(state),
      { getState: () => state },
    ),
  };
});
vi.mock("@multica/core/config", () => {
  const state = { vcsIntegrationAvailable: false };
  return {
    useConfigStore: Object.assign(
      (selector: (value: typeof state) => unknown) => selector(state),
      { getState: () => state },
    ),
  };
});
vi.mock("@multica/core/vcs", () => ({ useRepoProvider: () => ({ kind: "git", name: "Git" }) }));

import { RepositoriesTab } from "./repositories-tab";

const workspace: Workspace = {
  id: "workspace-1", name: "Acme", slug: "acme", description: "", context: "",
  settings: {}, issue_prefix: "ACM", avatar_url: null,
  repos: [{ url: "https://github.com/acme/app" }],
  created_at: "2026-10-14T00:00:00Z", updated_at: "2026-10-14T00:00:00Z",
};
const owner: MemberWithUser = {
  id: "member-1", user_id: "user-1", workspace_id: workspace.id,
  name: "Alex", email: "alex@example.com", role: "owner", avatar_url: null,
  created_at: "2026-10-14T00:00:00Z",
};
const installations = {
  configured: true, repository_browse_configured: true, can_manage: true,
  installations: [{ id: "installation-1", account_login: "acme" }],
};
const repositoryPage = {
  repositories: [{
    id: 2, full_name: "acme/console", clone_url: "https://github.com/acme/console.git",
    html_url: "https://github.com/acme/console", description: "Console", private: false,
    archived: false, default_branch: "main",
  }],
  next_page: null,
  total_count: 1,
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
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } },
  });
  client.setQueryData(workspaceKeys.list(), [workspace]);
  requests.listMembers.mockResolvedValue([owner]);
  requests.listGitHubInstallations.mockResolvedValue(installations);
  requests.listGitHubInstallationRepositories.mockResolvedValue(repositoryPage);
  requests.updateWorkspace.mockImplementation(async (_id: string, patch: Partial<Workspace>) => ({ ...workspace, ...patch }));
});
afterEach(() => { cleanup(); client.clear(); });

function renderTab() {
  return renderWithI18n(<QueryClientProvider client={client}><RepositoriesTab /></QueryClientProvider>);
}

describe("RepositoriesTab persistence and recovery", () => {
  it("retains the final row and confirmation through a delayed failure, then retries once", async () => {
    const pending = deferred<Workspace>();
    requests.updateWorkspace.mockReturnValueOnce(pending.promise);
    const user = userEvent.setup();
    renderTab();
    await screen.findByRole("button", { name: "Add repository" });
    const rowInput = screen.getByDisplayValue(workspace.repos[0]!.url);
    await user.click(screen.getByRole("button", { name: "Delete repository" }));
    const dialog = screen.getByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", { name: "Delete repository" });
    await user.click(confirm);

    await waitFor(() => expect(requests.updateWorkspace).toHaveBeenCalledTimes(1));
    expect(rowInput).toBeInTheDocument();
    expect(screen.queryByText("No repositories yet.")).not.toBeInTheDocument();
    expect(confirm).toBeDisabled();
    await user.click(confirm);
    await user.keyboard("{Escape}");
    expect(dialog).toBeInTheDocument();
    expect(requests.updateWorkspace).toHaveBeenCalledTimes(1);

    await act(async () => pending.reject(new Error("Repository save failed")));
    await waitFor(() => expect(notices.error).toHaveBeenCalledWith("Repository save failed"));
    expect(rowInput).toBeInTheDocument();
    expect(confirm).toBeEnabled();
    expect(client.getQueryData<Workspace[]>(workspaceKeys.list())?.[0]?.repos).toEqual(workspace.repos);

    await user.click(confirm);
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(requests.updateWorkspace).toHaveBeenCalledTimes(2);
    expect(requests.updateWorkspace).toHaveBeenLastCalledWith(workspace.id, { repos: [] });
    expect(screen.getByText("No repositories yet.")).toBeInTheDocument();
    expect(client.getQueryData<Workspace[]>(workspaceKeys.list())?.[0]?.repos).toEqual([]);
  });

  it("serializes a confirmed removal behind an in-flight edit without removing the row early", async () => {
    const edit = deferred<Workspace>();
    const removal = deferred<Workspace>();
    requests.updateWorkspace.mockReturnValueOnce(edit.promise).mockReturnValueOnce(removal.promise);
    const user = userEvent.setup();
    renderTab();
    await screen.findByRole("button", { name: "Add repository" });
    const rowInput = screen.getByDisplayValue(workspace.repos[0]!.url);
    const description = screen.getByRole("textbox", { name: /^Description/ });
    fireEvent.change(description, { target: { value: "Edited description" } });
    fireEvent.blur(description);
    await waitFor(() => expect(requests.updateWorkspace).toHaveBeenCalledTimes(1));
    await user.click(screen.getByRole("button", { name: "Delete repository" }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete repository" }));
    expect(rowInput).toBeInTheDocument();
    expect(requests.updateWorkspace).toHaveBeenCalledTimes(1);

    await act(async () => edit.resolve({ ...workspace, repos: [{ ...workspace.repos[0]!, description: "Edited description" }] }));
    await waitFor(() => expect(requests.updateWorkspace).toHaveBeenCalledTimes(2));
    expect(rowInput).toBeInTheDocument();
    expect(requests.updateWorkspace).toHaveBeenLastCalledWith(workspace.id, { repos: [] });

    await act(async () => removal.resolve({ ...workspace, repos: [] }));
    await waitFor(() => expect(screen.getByText("No repositories yet.")).toBeInTheDocument());
    expect(client.getQueryData<Workspace[]>(workspaceKeys.list())?.[0]?.repos).toEqual([]);
    expect(requests.updateWorkspace).toHaveBeenCalledTimes(2);
  });

  it("can discard an unsaved empty row without blocking confirmation on the invalid draft", async () => {
    const user = userEvent.setup();
    renderTab();
    await user.click(await screen.findByRole("button", { name: "Add repository" }));
    expect(screen.getAllByRole("textbox")).toHaveLength(4);
    await user.click(screen.getAllByRole("button", { name: "Delete repository" })[1]!);
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete repository" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(screen.getAllByRole("textbox")).toHaveLength(2);
    expect(requests.updateWorkspace).not.toHaveBeenCalled();
    expect(client.getQueryData<Workspace[]>(workspaceKeys.list())?.[0]?.repos).toEqual(workspace.repos);
  });

  it("shows unavailable permissions without claiming the viewer is a regular member", async () => {
    requests.listMembers.mockRejectedValueOnce(new Error("offline"));
    const user = userEvent.setup();
    renderTab();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load workspace permissions.");
    expect(screen.queryByText("Only admins and owners can manage repositories.")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add repository" })).not.toBeInTheDocument();
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: "Add repository" })).toBeEnabled();
  });

  it("retries a GitHub connection read without misreporting missing configuration", async () => {
    requests.listGitHubInstallations.mockRejectedValueOnce(new Error("offline"));
    const user = userEvent.setup();
    renderTab();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load GitHub connections.");
    expect(screen.queryByRole("button", { name: "Connect GitHub" })).not.toBeInTheDocument();
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: "Choose from GitHub" })).toBeEnabled();
  });

  it("retains a selected repository and search input when its picker refresh fails", async () => {
    const user = userEvent.setup();
    renderTab();
    await user.click(await screen.findByRole("button", { name: "Choose from GitHub" }));
    const checkbox = await screen.findByRole("checkbox", { name: /acme\/console/ });
    await user.click(checkbox);
    const search = screen.getByRole("textbox", { name: "Search loaded repositories" });
    await user.type(search, "console");
    requests.listGitHubInstallationRepositories.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: githubKeys.repositories(workspace.id, "installation-1") }); });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not refresh repositories from GitHub.");
    expect(screen.getByRole("checkbox", { name: /acme\/console/ })).toBe(checkbox);
    expect(checkbox).toBeChecked();
    expect(screen.getByRole("textbox", { name: "Search loaded repositories" })).toBe(search);
    expect(search).toHaveValue("console");
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(checkbox).toBeChecked();
  });

  it("keeps denied GitHub repositories unavailable through a later failed retry", async () => {
    const user = userEvent.setup();
    renderTab();
    await user.click(await screen.findByRole("button", { name: "Choose from GitHub" }));
    await user.click(await screen.findByRole("checkbox", { name: /acme\/console/ }));
    requests.listGitHubInstallationRepositories.mockRejectedValueOnce(new ApiError("Forbidden", 403, "Forbidden"));
    await act(async () => { await client.invalidateQueries({ queryKey: githubKeys.repositories(workspace.id, "installation-1") }); });
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("You do not have access to these settings.");
    expect(screen.queryByRole("checkbox", { name: /acme\/console/ })).not.toBeInTheDocument();
    const importButton = within(screen.getByRole("dialog")).getByRole("button", { name: "Add repositories" });
    expect(importButton).toBeDisabled();

    requests.listGitHubInstallationRepositories.mockRejectedValueOnce(new Error("offline"));
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(within(alert).getByRole("button", { name: "Retry" })).toBeEnabled());
    expect(screen.queryByRole("checkbox", { name: /acme\/console/ })).not.toBeInTheDocument();
    expect(importButton).toBeDisabled();
  });
});
