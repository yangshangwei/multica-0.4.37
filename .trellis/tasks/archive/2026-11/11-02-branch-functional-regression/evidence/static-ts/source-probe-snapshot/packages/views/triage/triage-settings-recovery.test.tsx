import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api/client";
import { triageKeys, type TriageSettings } from "@multica/core/triage";
import { workspaceKeys } from "@multica/core/workspace/queries";
import type { MemberWithUser } from "@multica/core/types";
import { renderWithI18n } from "../test/i18n";
import { TriageSettingsTab } from "./triage-settings-tab";

const api = vi.hoisted(() => ({ getTriageSettings: vi.fn(), listTriageItems: vi.fn(), listMembers: vi.fn() }));
const save = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", () => ({ api }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ triage: () => "/acme/triage" }) }));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return { useAuthStore: Object.assign(
    (selector: (value: typeof state) => unknown) => selector(state),
    { getState: () => state },
  ) };
});
vi.mock("./triage-fields", () => ({
  AcceptanceStatusSelect: () => null, ReviewerSelect: () => null, TriageSelect: () => null,
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  useUpdateTriageSettings: () => ({ mutateAsync: save, isPending: false }),
}));

const settings: TriageSettings = {
  supported: true, enabled: true, acceptance_status: "backlog", require_priority: false,
  responsibility_mode: "none", responsibility_member_id: null, revision: 2,
};
const key = triageKeys.settings("ws-1");
const membersKey = workspaceKeys.members("ws-1");
const members: MemberWithUser[] = [{
  id: "member-1", workspace_id: "ws-1", user_id: "user-1", role: "owner",
  name: "Alex", email: "alex@example.com", avatar_url: null, created_at: "2026-10-14T00:00:00Z",
}];
let client: QueryClient;
function mount() {
  return renderWithI18n(<QueryClientProvider client={client}><TriageSettingsTab /></QueryClientProvider>);
}

beforeEach(() => {
  vi.resetAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  client.setQueryData(membersKey, members);
  api.listMembers.mockResolvedValue(members);
  api.listTriageItems.mockResolvedValue({ counts: { pending: 0, ready: 0, snoozed: 0 } });
});
afterEach(() => { cleanup(); client.clear(); });

describe("Triage settings refresh recovery", () => {
  it("preserves the focused switch and unsaved draft through failure and retry", async () => {
    client.setQueryData(key, settings);
    api.getTriageSettings.mockRejectedValueOnce(new Error("offline"));
    let resolve!: (value: TriageSettings) => void;
    api.getTriageSettings.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    mount();
    const priority = screen.getByRole("switch", { name: "Require a priority" });
    fireEvent.click(priority);
    priority.focus();
    expect(priority).toBeChecked();
    await act(async () => { await client.invalidateQueries({ queryKey: key }); });
    expect(screen.getByRole("switch", { name: "Require a priority" })).toBe(priority);
    expect(priority).toBeChecked();
    expect(priority).toHaveFocus();
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh triage settings");
    const retry = screen.getByRole("button", { name: "Retry" });
    fireEvent.click(retry);
    await waitFor(() => expect(retry).toBeDisabled());
    expect(screen.getByRole("button", { name: "Retrying..." })).toBe(retry);
    expect(priority).toBeInTheDocument();
    await act(async () => resolve(settings));
    fireEvent.click(screen.getByRole("button", { name: "Save settings" }));
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ require_priority: true, expected_revision: 2 }));
  });

  it("keeps cold recovery distinct from unsupported settings", async () => {
    api.getTriageSettings.mockRejectedValueOnce(new Error("offline"));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load triage settings");
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled();
  });

  it("hides the draft after a definitive access denial", async () => {
    client.setQueryData(key, settings);
    api.getTriageSettings.mockRejectedValue(new ApiError("denied", 403, "Forbidden"));
    mount();
    fireEvent.click(screen.getByRole("switch", { name: "Require a priority" }));
    await act(async () => { await client.invalidateQueries({ queryKey: key }); });
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.queryByRole("button", { name: "Save settings" })).toBeNull();
  });

  it("offers permission recovery before showing controls when membership is unknown", async () => {
    client.removeQueries({ queryKey: membersKey });
    client.setQueryData(key, settings);
    api.listMembers.mockRejectedValueOnce(new Error("offline"));
    mount();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load workspace permissions.");
    expect(screen.queryByRole("switch")).toBeNull();
    fireEvent.click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("switch", { name: "Require a priority" })).toBeEnabled();
  });

  it("retains authorized input on membership refresh failure and clears it on definitive denial", async () => {
    client.setQueryData(key, settings);
    mount();
    const priority = screen.getByRole("switch", { name: "Require a priority" });
    fireEvent.click(priority);
    priority.focus();
    api.listMembers.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: membersKey }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh workspace permissions.");
    expect(screen.getByRole("switch", { name: "Require a priority" })).toBe(priority);
    expect(priority).toBeChecked();
    expect(priority).toHaveFocus();

    api.listMembers.mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"));
    await act(async () => { await client.invalidateQueries({ queryKey: membersKey }); });
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("You do not have access to these settings.");
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.queryByRole("button", { name: "Save settings" })).toBeNull();
    api.listMembers.mockRejectedValueOnce(new Error("offline"));
    fireEvent.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(within(alert).getByRole("button", { name: "Retry" })).toBeEnabled());
    expect(screen.queryByRole("switch")).toBeNull();

    fireEvent.click(within(alert).getByRole("button", { name: "Retry" }));
    const restored = await screen.findByRole("switch", { name: "Require a priority" });
    expect(restored).not.toBeChecked();
    expect(screen.getByRole("button", { name: "Save settings" })).toBeDisabled();
  });
});
