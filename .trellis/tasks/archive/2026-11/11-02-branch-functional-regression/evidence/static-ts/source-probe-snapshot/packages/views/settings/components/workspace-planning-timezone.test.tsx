import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { useProjectAccessStore } from "@multica/core/projects";
import { renderWithI18n } from "../../test/i18n";
import { WorkspacePlanningTimezone } from "./workspace-planning-timezone";

vi.mock("../../common/timezone-select", () => ({ timezoneOptions: (zone: string) => [...new Set([zone, "Asia/Shanghai", "UTC", "Europe/Paris"])] }));
const defaultTimezone = { workspace_id: "ws", planning_timezone: null, effective_timezone: "Asia/Shanghai", configured: false };
const getTimezone = vi.fn();
const updateTimezone = vi.fn();
let client: QueryClient;
beforeEach(() => {
  vi.resetAllMocks();
  useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  getTimezone.mockResolvedValue(defaultTimezone);
  updateTimezone.mockImplementation(async (wsId, zone) => {
    const saved = { workspace_id: wsId, planning_timezone: zone, effective_timezone: zone ?? "Asia/Shanghai", configured: zone !== null };
    getTimezone.mockResolvedValue(saved);
    return saved;
  });
  setApiInstance({ getProjectPlanningTimezone: getTimezone, updateProjectPlanningTimezone: updateTimezone } as unknown as ApiClient);
});
afterEach(() => client.clear());
function mount(canManage = true, wsId = "ws") {
  const ui = (id: string) => <QueryClientProvider client={client}><WorkspacePlanningTimezone key={id} wsId={id} canManage={canManage} /></QueryClientProvider>;
  const view = renderWithI18n(ui(wsId));
  return { user: userEvent.setup(), switchWorkspace: (id: string) => view.rerender(ui(id)) };
}

it("lets an administrator save a timezone and restore the server default", async () => {
  const { user } = mount();
  const select = await screen.findByRole("combobox", { name: "Planning timezone" });
  expect(select).toHaveTextContent("Asia/Shanghai");
  await user.click(select);
  await user.click(await screen.findByRole("option", { name: "UTC" }));
  expect(updateTimezone).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Save timezone" }));
  await waitFor(() => expect(updateTimezone).toHaveBeenCalledWith("ws", "UTC"));
  await user.click(await screen.findByRole("button", { name: "Restore default" }));
  await waitFor(() => expect(updateTimezone).toHaveBeenLastCalledWith("ws", null));
  await waitFor(() => expect(screen.getByRole("combobox")).toHaveTextContent("Asia/Shanghai"));
});

it("keeps a failed save editable and allows the draft to be cancelled", async () => {
  updateTimezone.mockRejectedValue(new Error("Network unavailable"));
  const { user } = mount();
  await user.click(await screen.findByRole("combobox"));
  await user.click(await screen.findByRole("option", { name: "Europe/Paris" }));
  await user.click(screen.getByRole("button", { name: "Save timezone" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Network unavailable");
  expect(screen.getByRole("combobox")).toHaveTextContent("Europe/Paris");
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(screen.getByRole("combobox")).toHaveTextContent("Asia/Shanghai");
});

it("preserves the timezone draft when another administrator changes the saved value", async () => {
  const { user } = mount();
  await user.click(await screen.findByRole("combobox"));
  await user.click(await screen.findByRole("option", { name: "Europe/Paris" }));
  getTimezone.mockResolvedValue({ ...defaultTimezone, planning_timezone: "UTC", effective_timezone: "UTC", configured: true });
  await act(async () => { await client.invalidateQueries({ queryKey: ["projects", "ws", "planning-timezone"] }); });
  expect(screen.getByRole("combobox")).toHaveTextContent("Europe/Paris");
  await user.click(screen.getByRole("button", { name: "Save timezone" }));
  expect(await screen.findByText("Planning timezone saved")).toBeInTheDocument();
  expect(updateTimezone).toHaveBeenCalledWith("ws", "Europe/Paris");
});

it("shows members the saved value without editing actions", async () => {
  getTimezone.mockResolvedValue({ ...defaultTimezone, planning_timezone: "Europe/Paris", effective_timezone: "Europe/Paris", configured: true });
  mount(false);
  expect(await screen.findByRole("combobox")).toBeDisabled();
  expect(screen.getByRole("combobox")).toHaveTextContent("Europe/Paris");
  expect(screen.queryByRole("button", { name: "Restore default" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Save timezone" })).not.toBeInTheDocument();
});

it("offers a retry without inventing a default when the setting cannot be loaded", async () => {
  getTimezone.mockRejectedValue(new Error("Offline"));
  const { user } = mount();
  expect(await screen.findByRole("alert", {}, { timeout: 4000 })).toHaveTextContent("Could not load the planning timezone");
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  getTimezone.mockResolvedValue(defaultTimezone);
  await user.click(screen.getByRole("button", { name: "Retry" }));
  expect(await screen.findByRole("combobox")).toHaveTextContent("Asia/Shanghai");
});

it("drops the old draft when switching workspaces", async () => {
  const { user, switchWorkspace } = mount();
  await user.click(await screen.findByRole("combobox"));
  await user.click(await screen.findByRole("option", { name: "UTC" }));
  getTimezone.mockResolvedValue({ ...defaultTimezone, workspace_id: "other", planning_timezone: "Europe/Paris", effective_timezone: "Europe/Paris", configured: true });
  switchWorkspace("other");
  expect(await screen.findByRole("combobox")).toHaveTextContent("Europe/Paris");
  expect(screen.queryByRole("button", { name: "Save timezone" })).not.toBeInTheDocument();
  expect(updateTimezone).not.toHaveBeenCalled();
});

it("confirms an unknown save by reading the shared setting without resubmitting", async () => {
  updateTimezone.mockImplementation(async () => {
    getTimezone.mockResolvedValue({ ...defaultTimezone, configured: true, planning_timezone: "UTC", effective_timezone: "UTC" });
    throw new TypeError("Response lost");
  });
  const { user } = mount();
  await user.click(await screen.findByRole("combobox"));
  await user.click(await screen.findByRole("option", { name: "UTC" }));
  await user.click(screen.getByRole("button", { name: "Save timezone" }));
  expect(await screen.findByText("Planning timezone saved")).toBeInTheDocument();
  expect(updateTimezone).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("button", { name: "Save timezone" })).not.toBeInTheDocument();
});

it("locks uncertain saves until a successful read and retains the draft on failed refresh", async () => {
  updateTimezone.mockImplementation(async () => {
    getTimezone.mockRejectedValue(new Error("Still offline"));
    throw new TypeError("Response lost");
  });
  const { user } = mount();
  await user.click(await screen.findByRole("combobox"));
  await user.click(await screen.findByRole("option", { name: "UTC" }));
  await user.click(screen.getByRole("button", { name: "Save timezone" }));
  await screen.findByText(/Could not load the planning timezone/, {}, { timeout: 4000 });
  const retry = screen.getByRole("button", { name: "Retry" });
  expect(screen.getByRole("combobox")).toHaveTextContent("UTC");
  expect(screen.getByRole("combobox")).toBeDisabled();
  expect(screen.getByRole("button", { name: "Save timezone" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
  getTimezone.mockResolvedValue(defaultTimezone);
  await user.click(retry);
  await waitFor(() => expect(screen.getByRole("combobox")).toBeEnabled());
  expect(screen.getByRole("combobox")).toHaveTextContent("UTC");
  expect(screen.getByRole("button", { name: "Save timezone" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
  expect(updateTimezone).toHaveBeenCalledTimes(1);
});

it("keeps confirmation retry available after an incidental successful refresh", async () => {
  updateTimezone.mockImplementation(async () => {
    getTimezone.mockRejectedValue(new Error("Still offline"));
    throw new TypeError("Response lost");
  });
  const { user } = mount();
  await user.click(await screen.findByRole("combobox"));
  await user.click(await screen.findByRole("option", { name: "UTC" }));
  await user.click(screen.getByRole("button", { name: "Save timezone" }));
  await screen.findByText(/Could not load the planning timezone/, {}, { timeout: 4000 });

  getTimezone.mockResolvedValue({ ...defaultTimezone, planning_timezone: "UTC", effective_timezone: "UTC", configured: true });
  await act(async () => { await client.invalidateQueries({ queryKey: ["projects", "ws", "planning-timezone"] }); });
  await waitFor(() => expect(screen.queryByText(/Could not load the planning timezone/)).not.toBeInTheDocument());
  expect(screen.getByRole("combobox")).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Retry" }));
  await waitFor(() => expect(screen.getByRole("combobox")).toBeEnabled());
  expect(await screen.findByText("Planning timezone saved")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  expect(updateTimezone).toHaveBeenCalledTimes(1);
});
