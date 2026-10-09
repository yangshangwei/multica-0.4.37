import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { renderWithI18n } from "../test/i18n";
import { IterationSettingsTab } from "./iteration-settings-tab";
import { previewFor, receipt, settings, source, targetA, ws } from "./test-fixtures";

// Request persistence/access matrices live in core/iterations/command.test.tsx;
// complete preview and invalid-item matrices live in iteration-operation.test.tsx.
const identity = vi.hoisted(() => ({ role: "admin", actor: "owner" }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => ws }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ iterations: () => "/acme/iterations", settings: () => "/acme/settings" }) }));
vi.mock("@multica/core/permissions", () => ({ useCurrentMember: () => ({ role: identity.role, userId: identity.actor }) }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: Object.assign(
  (select: (state: { user: { id: string } }) => unknown) => select({ user: { id: identity.actor } }),
  { getState: () => ({ user: { id: identity.actor } }) },
) }));
vi.mock("../navigation", () => ({ AppLink: ({ href, children, ...props }: React.ComponentProps<"a">) => <a href={href} {...props}>{children}</a> }));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    getBaseUrl: () => "settings-test", getSessionScope: () => "session",
    getIterationCapabilities: vi.fn(), getIterationSettings: vi.fn(), listIterations: vi.fn(),
    getProjectPlanningTimezone: vi.fn(),
    enableIterations: vi.fn(), previewIteration: vi.fn(), applyIterationOperation: vi.fn(), getIterationOperation: vi.fn(),
  },
}));
let client: QueryClient;
let current: Awaited<ReturnType<typeof api.getIterationSettings>> = { ...settings, enabled: false, effective_timezone: "Asia/Shanghai", planning_timezone: "Asia/Shanghai" };
beforeEach(() => {
  vi.resetAllMocks();
  identity.role = "admin";
  identity.actor = "owner";
  localStorage.clear();
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  current = { ...settings, enabled: false, effective_timezone: "Asia/Shanghai", planning_timezone: "Asia/Shanghai" };
  vi.mocked(api.getIterationCapabilities).mockImplementation(async () => ({ workspace_id: ws, schema_version: 1, supported: true, manual: true, atomic_handoff: true, enabled: current.enabled }));
  vi.mocked(api.getIterationSettings).mockImplementation(async () => ({ ...current }));
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [source, targetA], next_cursor: null });
  vi.mocked(api.getProjectPlanningTimezone).mockImplementation(async () => ({ workspace_id: ws, planning_timezone: current.planning_timezone, effective_timezone: current.effective_timezone, configured: true }));
  vi.mocked(api.enableIterations).mockImplementation(async () => { current.enabled = true; current.revision++; return receipt; });
  vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => previewFor(draft));
  vi.mocked(api.applyIterationOperation).mockImplementation(async () => { current.enabled = false; current.revision++; return receipt; });
});
afterEach(() => client.clear());
function mount(locale: "en" | "zh-Hans" = "en") {
  const tree = <QueryClientProvider client={client}><IterationSettingsTab /></QueryClientProvider>;
  const view = renderWithI18n(tree, { locale });
  return { ...view, user: userEvent.setup(), refresh: () => view.rerender(tree) };
}
async function ready() {
  const toggle = await screen.findByRole("switch", { name: "Enable iterations" });
  await waitFor(() => expect(toggle).not.toHaveAttribute("aria-disabled", "true"));
  return toggle;
}
it("shows the shared timezone with a link to its only editor", async () => {
  mount();
  await ready();
  expect(screen.getByText("New iterations use the workspace timezone: Asia/Shanghai.")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Change in workspace settings" })).toHaveAttribute("href", "/acme/settings?tab=workspace");
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Save timezone" })).not.toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "Planning settings" })).not.toBeInTheDocument();
  expect(api.getProjectPlanningTimezone).not.toHaveBeenCalled();
});
it.each([
  { configured: false, timezone: "Asia/Shanghai" },
  { configured: true, timezone: "Europe/Paris" },
])("enables with the displayed timezone $timezone when configured=$configured", async ({ configured, timezone }) => {
  current = { ...current, planning_timezone: configured ? timezone : null, effective_timezone: timezone, timezone_configured: configured };
  const { user } = mount();
  const toggle = await ready();
  expect(screen.getByText(`New iterations use the workspace timezone: ${timezone}.`)).toBeInTheDocument();
  await user.click(toggle);
  await waitFor(() => expect(api.enableIterations).toHaveBeenCalledWith(ws, expect.objectContaining({ confirmed_timezone: timezone })));
});
it("enables with the displayed saved timezone and waits for the server", async () => {
  let resolve!: (value: typeof receipt) => void;
  vi.mocked(api.enableIterations).mockImplementation(() => new Promise((done) => { resolve = done; }));
  const { user } = mount();
  const toggle = await ready();
  const status = screen.getByText("Iterations are disabled. Saved history remains available.");
  expect(status).toHaveAttribute("role", "status");
  expect(toggle).toHaveAccessibleDescription(status.textContent!);
  expect(toggle).not.toBeChecked();
  expect(screen.getByRole("link", { name: "View history" })).toHaveAttribute("href", "/acme/iterations");
  await user.click(toggle);
  await waitFor(() => expect(api.enableIterations).toHaveBeenCalledWith(ws, expect.objectContaining({ expected_revision: 1, confirmed_timezone: "Asia/Shanghai" })));
  expect(toggle).not.toBeChecked();
  expect(toggle).toHaveAttribute("aria-disabled", "true");
  expect(status).toHaveTextContent("Iterations are disabled. Saved history remains available.");
  current.enabled = true;
  await act(async () => resolve(receipt));
  await waitFor(() => expect(toggle).toBeChecked());
  expect(status).toHaveTextContent("Iterations are enabled.");
  expect(toggle).toHaveAccessibleDescription("Iterations are enabled.");
  expect(screen.getByRole("link", { name: "Manage iterations" })).toBeInTheDocument();
});
it("updates feedback after enable, normal disable and re-enable without remounting", async () => {
  const { user } = mount();
  const toggle = await ready();
  const status = screen.getByText("Iterations are disabled. Saved history remains available.");
  await user.click(toggle);
  await waitFor(() => {
    expect(toggle).toBeChecked();
    expect(toggle).not.toHaveAttribute("aria-disabled", "true");
  });
  expect(status).toHaveTextContent("Iterations are enabled.");

  await user.click(toggle);
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Reason"), "Close this workspace");
  await user.click(within(dialog).getByRole("button", { name: "Preview changes" }));
  await user.click(await within(dialog).findByRole("button", { name: "Confirm changes" }));
  await waitFor(() => {
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(toggle).not.toBeChecked();
    expect(toggle).not.toHaveAttribute("aria-disabled", "true");
  });
  expect(current.enabled).toBe(false);
  expect(screen.getByRole("link", { name: "View history" })).toBeInTheDocument();
  expect(screen.queryByText("Iterations enabled.")).not.toBeInTheDocument();
  expect(screen.queryByText("Iterations are enabled.")).not.toBeInTheDocument();
  expect(status).toHaveTextContent("Iterations are disabled. Saved history remains available.");
  expect(toggle).toHaveAccessibleDescription(status.textContent!);

  await user.click(toggle);
  await waitFor(() => {
    expect(toggle).toBeChecked();
    expect(toggle).not.toHaveAttribute("aria-disabled", "true");
  });
  expect(current.enabled).toBe(true);
  expect(status).toHaveTextContent("Iterations are enabled.");
  expect(screen.getByRole("link", { name: "Manage iterations" })).toBeInTheDocument();
  expect(api.enableIterations).toHaveBeenCalledTimes(2);
  expect(api.enableIterations).toHaveBeenLastCalledWith(ws, expect.objectContaining({ expected_revision: 3 }));
  expect(api.applyIterationOperation).toHaveBeenCalledTimes(1);
});
it("adopts the saved timezone from iteration settings before enabling", async () => {
  const { user } = mount();
  const toggle = await ready();
  current = { ...current, effective_timezone: "UTC", planning_timezone: "UTC", revision: 2 };
  await act(async () => { await client.invalidateQueries({ queryKey: ["iterations", ws, "settings"] }); });
  await waitFor(() => expect(toggle).not.toHaveAttribute("aria-disabled", "true"));
  expect(screen.getByText("New iterations use the workspace timezone: UTC.")).toBeInTheDocument();
  await user.click(toggle);
  await waitFor(() => expect(api.enableIterations).toHaveBeenCalledWith(ws, expect.objectContaining({ expected_revision: 2, confirmed_timezone: "UTC" })));
});
it("opens the complete disable operation and cancellation leaves the switch on", async () => {
  current.enabled = true;
  const { user } = mount();
  const toggle = await ready();
  const status = screen.getByText("Iterations are enabled.");
  await user.click(toggle);
  expect(toggle).toBeChecked();
  const dialog = await screen.findByRole("dialog");
  expect(dialog).toHaveTextContent("Enabling again will not restore these cycles.");
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(toggle).toBeChecked();
  expect(status).toHaveTextContent("Iterations are enabled.");
  expect(screen.queryByText("Iterations are disabled. Saved history remains available.")).not.toBeInTheDocument();
  expect(api.applyIterationOperation).not.toHaveBeenCalled();
  await user.click(toggle);
  await user.type(within(screen.getByRole("dialog")).getByRole("textbox"), "Stop planning");
  await user.click(screen.getByRole("button", { name: "Preview changes" }));
  expect(await screen.findByText(/Alpha task/)).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Confirm changes" }));
  await waitFor(() => expect(toggle).not.toBeChecked());
  expect(screen.getByRole("link", { name: "View history" })).toBeInTheDocument();
  expect(api.applyIterationOperation).toHaveBeenCalledWith(ws, expect.objectContaining({ draft: expect.objectContaining({ operation: "disable", reason: "Stop planning" }), preview_hash: "hash" }));
});
it("keeps enabled feedback and the reason after a rejected disable", async () => {
  vi.mocked(api.applyIterationOperation).mockRejectedValueOnce(new ApiError("Changed", 409, "Conflict"));
  const { user } = mount();
  const toggle = await ready();
  await user.click(toggle);
  await waitFor(() => {
    expect(toggle).toBeChecked();
    expect(toggle).not.toHaveAttribute("aria-disabled", "true");
  });
  const status = screen.getByText("Iterations are enabled.");

  await user.click(toggle);
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Reason"), "Close this workspace");
  await user.click(within(dialog).getByRole("button", { name: "Preview changes" }));
  await user.click(await within(dialog).findByRole("button", { name: "Confirm changes" }));
  expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
  expect(within(dialog).getByLabelText("Reason")).toHaveValue("Close this workspace");
  expect(current.enabled).toBe(true);
  expect(toggle).toBeChecked();
  expect(status).toHaveTextContent("Iterations are enabled.");
  expect(screen.queryByText("Iterations are disabled. Saved history remains available.")).not.toBeInTheDocument();
  expect(within(dialog).getByRole("button", { name: "Preview changes" })).toBeEnabled();
  expect(within(dialog).queryByRole("button", { name: "Confirm changes" })).not.toBeInTheDocument();

  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(toggle).not.toHaveAttribute("aria-disabled", "true");
  expect(toggle).toHaveAccessibleDescription("Iterations are enabled.");
  expect(api.applyIterationOperation).toHaveBeenCalledTimes(1);
});
it("recovers the original enable request after remount even if capabilities are unavailable", async () => {
  vi.mocked(api.enableIterations).mockRejectedValue(new TypeError("Response lost"));
  const first = mount();
  await first.user.click(await ready());
  await screen.findByRole("button", { name: "Check original request" });
  const original = vi.mocked(api.enableIterations).mock.calls[0]![1];
  first.unmount();
  vi.mocked(api.getIterationCapabilities).mockResolvedValue({ workspace_id: ws, schema_version: 1, supported: false, manual: false, atomic_handoff: false, enabled: false });
  vi.mocked(api.getIterationOperation).mockResolvedValue(receipt);
  client.clear();
  const second = mount();
  expect(await screen.findByText(/This server does not support iteration settings/)).toBeInTheDocument();
  await second.user.click(screen.getByRole("button", { name: "Check original request" }));
  await waitFor(() => expect(api.getIterationOperation).toHaveBeenCalledWith(ws, original.request_id));
  expect(api.enableIterations).toHaveBeenCalledTimes(1);
});
it("shows a failed count read without presenting it as zero", async () => {
  current.enabled = true;
  vi.mocked(api.listIterations).mockRejectedValue(new Error("Offline"));
  mount();
  await ready();
  expect(await screen.findByText("Could not load the current iteration counts.")).toBeInTheDocument();
  expect(screen.queryByText("0 active · 0 planned")).not.toBeInTheDocument();
});
it("renders member settings read-only with Chinese labels", async () => {
  identity.role = "member";
  mount("zh-Hans");
  expect(await screen.findByRole("switch", { name: "启用迭代" })).toHaveAttribute("aria-disabled", "true");
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  expect(screen.getByText("新建迭代使用工作空间时区：Asia/Shanghai")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "前往修改" })).toHaveAttribute("href", "/acme/settings?tab=workspace");
  expect(screen.getByRole("link", { name: "查看历史" })).toBeInTheDocument();
});
it("removes protected settings after workspace access is revoked", async () => {
  mount();
  await ready();
  vi.mocked(api.getIterationSettings).mockRejectedValue(new ApiError("Revoked", 404, "Not Found", { code: "workspace_access_denied" }));
  await act(async () => { await client.invalidateQueries({ queryKey: ["iterations", ws, "settings"] }); });
  await waitFor(() => expect(screen.queryByRole("switch")).not.toBeInTheDocument());
  expect(screen.queryByText(/New iterations use the workspace timezone/)).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "Change in workspace settings" })).not.toBeInTheDocument();
  expect(screen.queryByText(/This server does not support iteration settings/)).not.toBeInTheDocument();
});

it("locks editing if the administrator role is lost while the tab is open", async () => {
  mount();
  const toggle = await ready();
  identity.role = "member";
  await act(async () => { await client.invalidateQueries({ queryKey: ["iterations", ws] }); });
  await waitFor(() => expect(toggle).toHaveAttribute("aria-disabled", "true"));
  expect(screen.getByText("New iterations use the workspace timezone: Asia/Shanghai.")).toBeInTheDocument();
  expect(api.enableIterations).not.toHaveBeenCalled();
});

it("retains the last read timezone but blocks changes until failed settings refresh is retried", async () => {
  const { user } = mount();
  const toggle = await ready();
  vi.mocked(api.getIterationSettings).mockRejectedValue(new Error("Offline"));
  await act(async () => { await client.invalidateQueries({ queryKey: ["iterations", ws, "settings"] }); });
  await screen.findByRole("button", { name: "Retry" });
  expect(screen.getByText("New iterations use the workspace timezone: Asia/Shanghai.")).toBeInTheDocument();
  expect(toggle).toHaveAttribute("aria-disabled", "true");
  current = { ...current, effective_timezone: "Europe/Paris", planning_timezone: "Europe/Paris" };
  vi.mocked(api.getIterationSettings).mockImplementation(async () => ({ ...current }));
  await user.click(screen.getByRole("button", { name: "Retry" }));
  await waitFor(() => expect(toggle).not.toHaveAttribute("aria-disabled", "true"));
  expect(screen.getByText("New iterations use the workspace timezone: Europe/Paris.")).toBeInTheDocument();
  expect(api.enableIterations).not.toHaveBeenCalled();
});


it("retains the original disable request when realtime closes settings before a lost response", async () => {
  current.enabled = true;
  let fail!: () => void;
  vi.mocked(api.applyIterationOperation).mockImplementation(() => new Promise((_resolve, reject) => { fail = () => reject(new TypeError("Response lost")); }));
  vi.mocked(api.getIterationOperation).mockImplementation(async (_ws, requestId) => ({ ...receipt, request_id: requestId, operation: "disable" }));
  const { user } = mount();
  await user.click(await ready());
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Reason"), "Disable original request");
  await user.click(within(dialog).getByRole("button", { name: "Preview changes" }));
  await user.click(await within(dialog).findByRole("button", { name: "Confirm changes" }));
  await waitFor(() => expect(api.applyIterationOperation).toHaveBeenCalledTimes(1));
  const original = vi.mocked(api.applyIterationOperation).mock.calls[0]![1];
  current.enabled = false;
  act(() => client.setQueryData(["iterations", ws, "settings"], { ...current }));
  await act(async () => fail());
  await user.click(await screen.findByRole("button", { name: "Check original request" }));
  await waitFor(() => expect(api.getIterationOperation).toHaveBeenCalledWith(ws, original.request_id));
  expect(api.applyIterationOperation).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("switch", { name: "Enable iterations" })).not.toBeChecked();
});

it("keeps an external disable closable without allowing a stale confirmation", async () => {
  const { user } = mount();
  const toggle = await ready();
  await user.click(toggle);
  await waitFor(() => {
    expect(toggle).toBeChecked();
    expect(toggle).not.toHaveAttribute("aria-disabled", "true");
  });
  const status = screen.getByText("Iterations are enabled.");
  await user.click(toggle);
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Reason"), "Close this workspace");
  await user.click(within(dialog).getByRole("button", { name: "Preview changes" }));
  await within(dialog).findByRole("button", { name: "Confirm changes" });

  current = { ...current, enabled: false, revision: current.revision + 1 };
  await act(async () => { await client.invalidateQueries({ queryKey: ["iterations", ws] }); });
  await waitFor(() => expect(toggle).not.toBeChecked());
  expect(status).toHaveTextContent("Iterations are disabled. Saved history remains available.");
  expect(screen.queryByText("Iterations enabled.")).not.toBeInTheDocument();
  expect(screen.queryByText("Iterations are enabled.")).not.toBeInTheDocument();
  expect(screen.getByRole("dialog")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Confirm changes" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Preview changes" })).toBeDisabled();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(toggle).not.toBeChecked();
  await waitFor(() => expect(toggle).not.toHaveAttribute("aria-disabled", "true"));
  expect(screen.getByRole("link", { name: "Change in workspace settings" })).toBeInTheDocument();
  expect(api.applyIterationOperation).not.toHaveBeenCalled();
});

it("requires current settings after a stale-timezone rejection before a new enable request", async () => {
  vi.mocked(api.enableIterations).mockImplementationOnce(async () => {
    current = { ...current, effective_timezone: "UTC", planning_timezone: "UTC" };
    vi.mocked(api.getIterationSettings).mockRejectedValue(new Error("Refresh unavailable"));
    throw new ApiError("Planning timezone changed", 409, "Conflict", { code: "iteration_preview_stale" });
  });
  const { user } = mount();
  const toggle = await ready();
  await user.click(toggle);
  await screen.findByRole("button", { name: "Retry" });
  expect(toggle).toHaveAttribute("aria-disabled", "true");
  expect(screen.getByText("New iterations use the workspace timezone: Asia/Shanghai.")).toBeInTheDocument();
  await user.click(toggle);
  expect(api.enableIterations).toHaveBeenCalledTimes(1);
  const original = vi.mocked(api.enableIterations).mock.calls[0]![1];
  expect(original.confirmed_timezone).toBe("Asia/Shanghai");

  vi.mocked(api.getIterationSettings).mockImplementation(async () => ({ ...current }));
  await user.click(screen.getByRole("button", { name: "Retry" }));
  await waitFor(() => expect(toggle).not.toHaveAttribute("aria-disabled", "true"));
  expect(screen.getByText("New iterations use the workspace timezone: UTC.")).toBeInTheDocument();
  await user.click(toggle);
  await waitFor(() => expect(api.enableIterations).toHaveBeenCalledTimes(2));
  expect(api.enableIterations).toHaveBeenLastCalledWith(ws, expect.objectContaining({ confirmed_timezone: "UTC" }));
  expect(vi.mocked(api.enableIterations).mock.calls[1]![1].request_id).not.toBe(original.request_id);
});
