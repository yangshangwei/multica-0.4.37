import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { notificationPreferenceKeys } from "@multica/core/notification-preferences/queries";
import type { NotificationPreferenceResponse } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { NotificationsTab } from "./notifications-tab";

const api = vi.hoisted(() => ({ getNotificationPreferences: vi.fn() }));
const mutate = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/api", () => ({ api }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/notification-preferences/mutations", () => ({
  useUpdateNotificationPreferences: () => ({ mutate }),
}));
vi.mock("./browser-notification-setting", () => ({ BrowserNotificationSetting: () => null }));

const key = notificationPreferenceKeys.all("ws-1");
let client: QueryClient;

function mount() {
  return renderWithI18n(<QueryClientProvider client={client}><NotificationsTab /></QueryClientProvider>);
}

beforeEach(() => {
  vi.clearAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
});
afterEach(() => { cleanup(); client.clear(); });

describe("Notification preference recovery", () => {
  it("does not show editable defaults while the first read is pending", () => {
    api.getNotificationPreferences.mockReturnValue(new Promise(() => {}));
    mount();
    expect(screen.getByRole("status")).toHaveTextContent("Loading notification settings");
    expect(screen.queryByRole("switch")).toBeNull();
  });

  it("keeps cold Retry mounted and disabled until the recovered preferences load", async () => {
    api.getNotificationPreferences.mockRejectedValueOnce(new Error("offline"));
    let resolve!: (data: NotificationPreferenceResponse) => void;
    api.getNotificationPreferences.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load notification settings");
    expect(screen.queryByRole("switch")).toBeNull();
    const retry = screen.getByRole("button", { name: "Retry" });
    fireEvent.click(retry);
    await waitFor(() => expect(retry).toBeDisabled());
    expect(screen.getByRole("button", { name: "Retrying..." })).toBe(retry);
    fireEvent.click(retry);
    expect(api.getNotificationPreferences).toHaveBeenCalledTimes(2);
    expect(mutate).not.toHaveBeenCalled();
    await act(async () => resolve({ workspace_id: "ws-1", preferences: { comments: "muted" } }));
    expect(await screen.findByRole("switch", { name: "Comments" })).not.toBeChecked();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("preserves cached switches and their values after a refresh error", async () => {
    client.setQueryData(key, { workspace_id: "ws-1", preferences: { comments: "muted" } });
    api.getNotificationPreferences.mockRejectedValue(new Error("offline"));
    mount();
    const comments = screen.getByRole("switch", { name: "Comments" });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh notification settings");
    expect(screen.getByRole("switch", { name: "Comments" })).toBe(comments);
    expect(comments).not.toBeChecked();
    fireEvent.click(comments);
    expect(mutate).toHaveBeenCalledWith({}, expect.any(Object));
  });

  it("shows default enabled values only after a successful empty preference response", async () => {
    api.getNotificationPreferences.mockResolvedValue({ workspace_id: "ws-1", preferences: {} });
    mount();
    await screen.findByRole("switch", { name: "Comments" });
    expect(screen.getAllByRole("switch")).toHaveLength(7);
    for (const control of screen.getAllByRole("switch")) expect(control).toBeChecked();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
