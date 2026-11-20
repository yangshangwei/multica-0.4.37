import type { TextareaHTMLAttributes } from "react";
import userEvent from "@testing-library/user-event";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import type { MemberWithUser, Squad } from "@multica/core/types";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { NavigationProvider } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { SquadDetailPage } from "./squad-detail-page";

const mocks = vi.hoisted(() => ({
  getSquad: vi.fn(), members: vi.fn(), update: vi.fn(),
  workspaceMembers: vi.fn(), userId: "user-1", success: vi.fn(), error: vi.fn(),
}));

vi.mock("@multica/core/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/api")>();
  return { ...actual, api: { ...actual.api,
    getSquad: mocks.getSquad, listSquadMembers: mocks.members,
    updateSquad: mocks.update, listMembers: mocks.workspaceMembers,
    getBaseUrl: () => "http://localhost:8080",
    listAgents: vi.fn().mockResolvedValue([]),
    getSquadMemberStatus: vi.fn().mockResolvedValue({ members: [] }),
  } };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useCurrentWorkspace: () => ({ id: "ws-1", slug: "acme" }), useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign((selector: (value: { user: { id: string } }) => unknown) => selector({ user: { id: mocks.userId } }),
    { getState: () => ({ user: { id: mocks.userId } }) }),
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("../../common/avatar-upload-control", () => ({ AvatarUploadControl: () => null }));
// This suite owns the host's post-debounce draft contract. ContentEditor's
// canonical suite owns pending emission and document reconciliation timing.
vi.mock("../../editor/content-editor", () => ({
  ContentEditor: ({ value, onUpdate, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement> & { value: string; onUpdate: (value: string) => void }) => (
    <textarea aria-label={props["aria-label"]} value={value} onChange={(event) => onUpdate(event.target.value)} />
  ),
}));
vi.mock("../../editor/readonly-content", () => ({ ReadonlyContent: ({ content }: { content: string }) => <div>{content}</div> }));
vi.mock("sonner", () => ({ toast: { success: mocks.success, error: mocks.error } }));

const SQUAD: Squad = {
  id: "squad-1", workspace_id: "ws-1", name: "Delivery squad", description: "Our workflow",
  instructions: "Original instructions", avatar_url: null, leader_id: "lead-1", creator_id: "user-1",
  created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z", archived_at: null, archived_by: null,
};
const MEMBER: MemberWithUser = {
  id: "m-1", user_id: "user-1", workspace_id: "ws-1", name: "Ada", email: "ada@example.com",
  role: "member", avatar_url: null, created_at: "2026-01-01T00:00:00Z",
};
const detailKey = [...workspaceKeys.squads("ws-1"), SQUAD.id];

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/squads/squad-1", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
  }}><SquadDetailPage /></NavigationProvider></QueryClientProvider>);
  return client;
}

async function openInstructions() {
  await userEvent.click(await screen.findByText("Instructions", { selector: "button" }));
  return screen.getByRole("textbox");
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.userId = "user-1";
  mocks.getSquad.mockResolvedValue(SQUAD);
  mocks.members.mockResolvedValue([]);
  mocks.workspaceMembers.mockResolvedValue([MEMBER]);
  mocks.update.mockResolvedValue(SQUAD);
});

describe("SquadDetailPage recovery and instructions", () => {
  it("keeps an emitted but unsaved draft when another member updates instructions", async () => {
    const client = renderPage();
    const editor = await openInstructions();
    fireEvent.change(editor, { target: { value: "My unsaved instructions" } });
    await act(async () => { client.setQueryData(detailKey, { ...SQUAD, instructions: "Remote instructions" }); });
    await waitFor(() => expect(screen.getByText(/changed elsewhere/)).toBeVisible());
    expect(editor).toHaveValue("My unsaved instructions");
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("adopts a remote update while clean and keeps edits typed during an awaited save", async () => {
    const client = renderPage();
    const editor = await openInstructions();
    await act(async () => { client.setQueryData(detailKey, { ...SQUAD, instructions: "Remote instructions" }); });
    await waitFor(() => expect(editor).toHaveValue("Remote instructions"));
    let complete!: (value: Squad) => void;
    mocks.update.mockReturnValueOnce(new Promise<Squad>((resolve) => { complete = resolve; }));
    fireEvent.change(editor, { target: { value: "Submitted instructions" } });
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    fireEvent.change(editor, { target: { value: "Newer local draft" } });
    mocks.getSquad.mockResolvedValue({ ...SQUAD, instructions: "Submitted instructions" });
    await act(async () => { complete({ ...SQUAD, instructions: "Submitted instructions" }); });
    await waitFor(() => expect(screen.getByRole("button", { name: "Save" })).toBeEnabled());
    expect(editor).toHaveValue("Newer local draft");
  });

  it("keeps a failed-save draft and displays the failure before allowing retry", async () => {
    mocks.update.mockRejectedValueOnce(new Error("Save unavailable"));
    renderPage();
    const editor = await openInstructions();
    fireEvent.change(editor, { target: { value: "Retry this draft" } });
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(mocks.error).toHaveBeenCalledWith("Save unavailable"));
    expect(editor).toHaveValue("Retry this draft");
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("uses the accepted save response while its follow-up refetch is still pending", async () => {
    renderPage();
    const editor = await openInstructions();
    fireEvent.change(editor, { target: { value: "Confirmed instructions" } });
    mocks.update.mockResolvedValueOnce({ ...SQUAD, instructions: "Confirmed instructions" });
    mocks.getSquad.mockReturnValue(new Promise(() => {}));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(mocks.success).toHaveBeenCalledWith("Instructions saved"));
    expect(editor).toHaveValue("Confirmed instructions");
    expect(screen.queryByText("Unsaved changes")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("removes management controls when the independent membership read loses access", async () => {
    const client = renderPage();
    await openInstructions();
    mocks.workspaceMembers.mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"));
    await act(async () => { await client.refetchQueries({ queryKey: workspaceKeys.members("ws-1"), exact: true }); });
    await waitFor(() => expect(screen.queryByRole("textbox", { name: "Instructions" })).not.toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
  });

  it("dismisses a pending archive confirmation after access loss without reopening it on recovery", async () => {
    const client = renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Archive" }));
    expect(screen.getByRole("alertdialog")).toBeVisible();

    mocks.workspaceMembers.mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"));
    await act(async () => { await client.refetchQueries({ queryKey: workspaceKeys.members("ws-1"), exact: true }); });
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());

    await act(async () => { await client.refetchQueries({ queryKey: workspaceKeys.members("ws-1"), exact: true }); });
    expect(await screen.findByRole("button", { name: "Archive" })).toBeVisible();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("retains the instructions editor while the cached roster refresh fails", async () => {
    const client = renderPage();
    const editor = await openInstructions();
    fireEvent.change(editor, { target: { value: "Roster-independent draft" } });
    mocks.members.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.refetchQueries({ queryKey: workspaceKeys.squadMembers("ws-1", "squad-1"), exact: true }); });
    expect(screen.getByRole("textbox", { name: "Instructions" })).toBe(editor);
    expect(editor).toHaveValue("Roster-independent draft");
  });

  it("renders instructions read-only for a viewer who cannot manage the squad", async () => {
    mocks.userId = "viewer";
    renderPage();
    await userEvent.click(await screen.findByRole("tab", { name: "Instructions" }));
    expect(screen.getByRole("tabpanel", { name: "Instructions" })).toHaveTextContent(SQUAD.instructions);
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
  });

  it("uses manually activated tabs and preserves the dirty-tab discard confirmation", async () => {
    renderPage();
    const membersTab = await screen.findByRole("tab", { name: "Members" });
    act(() => membersTab.focus());
    await userEvent.keyboard("{ArrowRight}");
    const instructionsTab = screen.getByRole("tab", { name: "Instructions" });
    expect(instructionsTab).toHaveFocus();
    expect(membersTab).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{Enter}");
    expect(instructionsTab).toHaveAttribute("aria-selected", "true");
    fireEvent.change(screen.getByRole("textbox", { name: "Instructions" }), { target: { value: "Unsaved" } });
    await userEvent.click(membersTab);
    expect(screen.getByRole("alertdialog")).toBeVisible();
    expect(instructionsTab).toHaveAttribute("aria-selected", "true");
    await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
    expect(screen.getByRole("textbox", { name: "Instructions" })).toHaveValue("Unsaved");
    await userEvent.click(membersTab);
    await userEvent.click(screen.getByRole("button", { name: /^Discard$/ }));
    expect(membersTab).toHaveAttribute("aria-selected", "true");
    await userEvent.click(instructionsTab);
    expect(screen.getByRole("textbox", { name: "Instructions" })).toHaveValue(SQUAD.instructions);
  });

  it("shows a recoverable initial failure and keeps Retry mounted during a cold retry", async () => {
    mocks.getSquad.mockRejectedValueOnce(new Error("offline"));
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load this squad.");
    let complete!: (value: Squad) => void;
    mocks.getSquad.mockReturnValueOnce(new Promise<Squad>((resolve) => { complete = resolve; }));
    const retry = screen.getByRole("button", { name: "Retry" });
    await userEvent.click(retry);
    expect(retry).toBeDisabled();
    await act(async () => { complete(SQUAD); });
    expect(await screen.findByRole("tab", { name: "Members" })).toBeVisible();
  });

  it("retains the editor and draft through a cached refresh failure", async () => {
    const client = renderPage();
    const editor = await openInstructions();
    fireEvent.change(editor, { target: { value: "Retained draft" } });
    mocks.getSquad.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.refetchQueries({ queryKey: detailKey, exact: true }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh this squad.");
    expect(screen.getByRole("textbox", { name: "Instructions" })).toBe(editor);
    expect(editor).toHaveValue("Retained draft");
  });

  it.each([403, 404])("does not restore a denied squad after %i then a failed retry", async (status) => {
    const client = renderPage();
    await openInstructions();
    mocks.getSquad.mockRejectedValueOnce(new ApiError("unavailable", status, "Unavailable"));
    await act(async () => { await client.refetchQueries({ queryKey: detailKey, exact: true }); });
    await waitFor(() => expect(screen.queryByRole("textbox", { name: "Instructions" })).not.toBeInTheDocument());
    expect(screen.getByRole("alert")).toHaveTextContent(status === 404 ? "Squad not found." : "You no longer have access to this squad.");
    mocks.getSquad.mockRejectedValueOnce(new Error("offline"));
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled());
    expect(screen.queryByRole("tab", { name: "Instructions" })).not.toBeInTheDocument();
    expect(client.getQueryData(detailKey)).toBeUndefined();
  });

  it("does not claim a zero-member roster after a failed first read", async () => {
    mocks.members.mockRejectedValueOnce(new Error("offline"));
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load squad members.");
    expect(screen.queryByText("0 members in this squad")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add Member" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("0 members in this squad")).toBeVisible();
  });
});
