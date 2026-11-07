import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import type { Invitation, MemberWithUser, ShareLink, Workspace } from "@multica/core/types";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { renderWithI18n } from "../../test/i18n";

const requests = vi.hoisted(() => ({
  listMembers: vi.fn(), listWorkspaces: vi.fn(), listWorkspaceInvitations: vi.fn(),
  listShareLinks: vi.fn(), createMember: vi.fn(), deleteMember: vi.fn(), updateMember: vi.fn(),
  revokeInvitation: vi.fn(), createShareLink: vi.fn(), revokeShareLink: vi.fn(),
}));
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(), api: requests,
}));
vi.mock("sonner", () => ({ toast: notices }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("../../navigation", () => ({ useOptionalNavigation: () => null }));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("@multica/core/paths", async () => {
  const { useQuery } = await import("@tanstack/react-query");
  const { workspaceListOptions } = await import("@multica/core/workspace/queries");
  return { useCurrentWorkspace: () => useQuery(workspaceListOptions()).data?.[0] ?? null };
});
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return { useAuthStore: Object.assign(
    (selector: (value: typeof state) => unknown) => selector(state),
    { getState: () => state },
  ) };
});

import { MembersTab } from "./members-tab";

const workspace: Workspace = {
  id: "workspace-1", name: "Acme", slug: "acme", description: "", context: "",
  settings: {}, issue_prefix: "ACM", avatar_url: null, repos: [],
  created_at: "2026-10-14T00:00:00Z", updated_at: "2026-10-14T00:00:00Z",
};
const owner: MemberWithUser = {
  id: "member-1", user_id: "user-1", workspace_id: workspace.id,
  name: "Alex", email: "alex@example.com", role: "owner", avatar_url: null,
  created_at: "2026-10-14T00:00:00Z",
};
const member: MemberWithUser = { ...owner, id: "member-2", user_id: "user-2", name: "Blair", email: "blair@example.com", role: "member" };
const invitation: Invitation = {
  id: "invitation-1", workspace_id: workspace.id, inviter_id: owner.user_id,
  invitee_email: "invited@example.com", invitee_user_id: null, role: "member", status: "pending",
  created_at: "2026-10-14T00:00:00Z", updated_at: "2026-10-14T00:00:00Z", expires_at: "2026-10-21T00:00:00Z",
};
const shareLink: ShareLink = {
  id: "link-1", workspace_id: workspace.id, code: "example-link", created_by: owner.user_id,
  role: "member", expires_at: null, max_uses: null, use_count: 0, is_active: true,
  created_at: "2026-10-14T00:00:00Z",
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
  requests.listMembers.mockResolvedValue([owner, member]);
  requests.listWorkspaceInvitations.mockResolvedValue([invitation]);
  requests.listShareLinks.mockResolvedValue([shareLink]);
  requests.createMember.mockResolvedValue(invitation);
  requests.deleteMember.mockResolvedValue(undefined);
  requests.revokeInvitation.mockResolvedValue(undefined);
});
afterEach(() => { cleanup(); client.clear(); });
function renderTab() {
  return renderWithI18n(<QueryClientProvider client={client}><MembersTab /></QueryClientProvider>);
}

describe("MembersTab read recovery", () => {
  it("keeps cold Retry mounted while fetching and never reports a failed read as an empty list", async () => {
    const retry = deferred<MemberWithUser[]>();
    requests.listMembers.mockRejectedValueOnce(new Error("offline")).mockReturnValueOnce(retry.promise);
    const user = userEvent.setup();
    renderTab();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load workspace members.");
    expect(screen.queryByText("No members found.")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Invite" })).not.toBeInTheDocument();
    const button = within(alert).getByRole("button", { name: "Retry" });
    await user.click(button);
    expect(button).toBeDisabled();
    expect(button).toHaveTextContent("Retrying...");
    await user.click(button);
    expect(requests.listMembers).toHaveBeenCalledTimes(2);
    await act(async () => retry.resolve([owner, member]));
    expect(await screen.findByRole("button", { name: "Invite" })).toBeInTheDocument();
  });

  it("preserves the invitation draft and member rows on a cached refresh failure", async () => {
    renderTab();
    const email = await screen.findByRole("textbox", { name: "user@company.com" });
    fireEvent.change(email, { target: { value: "draft@example.com" } });
    requests.listMembers.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.members(workspace.id) }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh workspace members.");
    expect(screen.getByRole("textbox", { name: "user@company.com" })).toBe(email);
    expect(email).toHaveValue("draft@example.com");
    expect(screen.getByText("Blair")).toBeInTheDocument();
  });

  it("exposes separate invitation and share-link retries when those collections fail", async () => {
    requests.listWorkspaceInvitations.mockRejectedValueOnce(new Error("invitation read failed"));
    requests.listShareLinks.mockRejectedValueOnce(new Error("link read failed"));
    const user = userEvent.setup();
    renderTab();
    await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(2));
    const invitationAlert = screen.getByText("Could not load invitations.").closest('[role="alert"]');
    const linkAlert = screen.getByText("Could not load share links.").closest('[role="alert"]');
    if (!(invitationAlert instanceof HTMLElement) || !(linkAlert instanceof HTMLElement)) throw new Error("Missing collection recovery");
    await user.click(within(invitationAlert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(invitation.invitee_email)).toBeInTheDocument();
    await user.click(within(linkAlert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(/join\?code=example-link/)).toBeInTheDocument();
  });

  it("hides cached management and invitation data when membership is definitively denied", async () => {
    renderTab();
    await screen.findByRole("button", { name: "Invite" });
    await screen.findByText(invitation.invitee_email);
    requests.listMembers.mockRejectedValueOnce(new ApiError("Forbidden", 403, "Forbidden"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.members(workspace.id) }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("You do not have access to these settings.");
    expect(screen.queryByRole("button", { name: "Invite" })).not.toBeInTheDocument();
    expect(screen.queryByText(invitation.invitee_email)).not.toBeInTheDocument();
    expect(screen.queryByText("Blair")).not.toBeInTheDocument();
  });

  it.each(["invitations", "share links"] as const)("does not revive denied %s after a transient retry or remount", async (kind) => {
    const request = kind === "invitations" ? requests.listWorkspaceInvitations : requests.listShareLinks;
    const queryKey = kind === "invitations" ? workspaceKeys.invitations(workspace.id) : workspaceKeys.shareLinks(workspace.id);
    const content = kind === "invitations" ? invitation.invitee_email : /join\?code=example-link/;
    const action = kind === "invitations" ? "Invite" : "Generate";
    const user = userEvent.setup();
    const view = renderTab();
    await screen.findByText(content);
    request.mockRejectedValueOnce(new ApiError("Forbidden", 403, "Forbidden"));
    await act(async () => { await client.invalidateQueries({ queryKey }); });
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("You do not have access to these settings.");
    expect(screen.queryByText(content)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: action })).not.toBeInTheDocument();

    request.mockRejectedValueOnce(new Error("offline"));
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(within(alert).getByRole("button", { name: "Retry" })).toBeEnabled());
    expect(screen.queryByText(content)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: action })).not.toBeInTheDocument();

    view.unmount();
    request.mockReturnValueOnce(new Promise(() => {}));
    renderTab();
    expect(screen.queryByText(content)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: action })).not.toBeInTheDocument();
  });

  it("dismisses an invitation confirmation when that collection loses access", async () => {
    const user = userEvent.setup();
    renderTab();
    await user.click(await screen.findByRole("button", { name: "Revoke invitation" }));
    expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
    requests.listWorkspaceInvitations.mockRejectedValueOnce(new ApiError("Forbidden", 403, "Forbidden"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.invitations(workspace.id) }); });
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(requests.revokeInvitation).not.toHaveBeenCalled();
  });
});

describe("MembersTab action safety", () => {
  it("names all three role and expiration selectors by purpose", async () => {
    renderTab();
    expect(await screen.findByRole("combobox", { name: "Invitation role" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Share link role" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Share link expiration" })).toBeInTheDocument();
  });

  it("rejects repeated Enter while inviting and retains the email on failure for retry", async () => {
    const pending = deferred<void>();
    requests.createMember.mockReturnValueOnce(pending.promise);
    const user = userEvent.setup();
    renderTab();
    const email = await screen.findByRole("textbox", { name: "user@company.com" });
    fireEvent.change(email, { target: { value: "new@example.com" } });
    fireEvent.keyDown(email, { key: "Enter" });
    fireEvent.keyDown(email, { key: "Enter" });
    expect(requests.createMember).toHaveBeenCalledTimes(1);
    expect(email).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "Invitation role" })).toBeDisabled();
    await act(async () => pending.reject(new Error("Invite failed")));
    await waitFor(() => expect(notices.error).toHaveBeenCalledWith("Invite failed"));
    expect(email).toBeEnabled();
    expect(email).toHaveValue("new@example.com");
    expect(notices.success).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Invite" }));
    await waitFor(() => expect(email).toHaveValue(""));
    expect(requests.createMember).toHaveBeenCalledTimes(2);
  });

  it("does not send an invitation while the user is confirming an IME composition", async () => {
    renderTab();
    const email = await screen.findByRole("textbox", { name: "user@company.com" });
    fireEvent.change(email, { target: { value: "new@example.com" } });
    fireEvent.keyDown(email, { key: "Enter", isComposing: true, keyCode: 229 });
    expect(requests.createMember).not.toHaveBeenCalled();
  });

  it.each(["member", "invitation"] as const)("retains a failed %s confirmation and blocks repeated submissions", async (kind) => {
    const pending = deferred<void>();
    const request = kind === "member" ? requests.deleteMember : requests.revokeInvitation;
    request.mockReturnValueOnce(pending.promise);
    const user = userEvent.setup();
    renderTab();
    if (kind === "member") {
      await user.click(await screen.findByRole("button", { name: "Actions for Blair" }));
      await user.click(await screen.findByRole("menuitem", { name: "Remove from workspace" }));
    } else {
      await user.click(await screen.findByRole("button", { name: "Revoke invitation" }));
    }
    const dialog = await screen.findByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", { name: "Confirm" });
    await user.click(confirm);
    expect(confirm).toBeDisabled();
    await user.click(confirm);
    await user.keyboard("{Escape}");
    expect(request).toHaveBeenCalledTimes(1);
    expect(dialog).toBeInTheDocument();
    await act(async () => pending.reject(new Error("Action failed")));
    await waitFor(() => expect(notices.error).toHaveBeenCalledWith("Action failed"));
    expect(dialog).toBeInTheDocument();
    expect(confirm).toBeEnabled();

    if (kind === "member") requests.listMembers.mockResolvedValue([owner]);
    else requests.listWorkspaceInvitations.mockResolvedValue([]);
    await user.click(confirm);
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(request).toHaveBeenCalledTimes(2);
    await waitFor(() => expect(screen.queryByText(kind === "member" ? "Blair" : invitation.invitee_email)).not.toBeInTheDocument());
  });
});
