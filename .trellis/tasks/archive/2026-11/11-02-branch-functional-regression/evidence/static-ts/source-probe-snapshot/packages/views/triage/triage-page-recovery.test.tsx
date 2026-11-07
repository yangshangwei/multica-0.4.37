import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError, setApiInstance, type ApiClient } from "@multica/core/api";
import { triageKeys, type TriageHistoryEntry } from "@multica/core/triage";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import { TriagePage } from "./triage-page";

const state = vi.hoisted(() => ({ params: new URLSearchParams(), replace: vi.fn() }));
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (selector: (value: typeof en) => string, values?: Record<string, string | number>) =>
      selector(en).replace(/{{(\w+)}}/g, (_, key: string) => String(values?.[key] ?? key)),
    i18n: { language: "en" },
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ triage: () => "/acme/triage", settings: () => "/acme/settings", issueDetail: (id: string) => `/acme/issues/${id}` }) }));
vi.mock("@multica/core/permissions", () => ({ useCurrentMember: () => ({ role: "member" }) }));
vi.mock("../navigation", () => ({
  useNavigation: () => ({ searchParams: state.params, replace: state.replace }),
  AppLink: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} />,
}));
vi.mock("@multica/ui/hooks/use-mobile", () => ({ useIsCompact: () => false }));
vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  ResizablePanel: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  ResizableHandle: () => null,
}));
vi.mock("../issues/components/issue-detail", () => ({ IssueDetail: () => <input aria-label="Issue draft" /> }));
vi.mock("./triage-filters", () => ({ TriageFilters: () => null }));
vi.mock("./triage-create-dialog", () => ({ TriageCreateDialog: () => null }));
vi.mock("./triage-import-dialog", () => ({ TriageImportDialog: () => null }));
vi.mock("./triage-action-dialog", () => ({ TriageActionDialog: () => null }));
vi.mock("./triage-batch-dialog", () => ({ TriageBatchDialog: () => null }));
vi.mock("./triage-history", () => ({
  TriageItemHistory: () => null,
  TriageExecutionStatus: () => null,
  TriageDuplicateTarget: () => null,
  TriageHistoryContent: () => <details><summary>Decision details</summary><span>Retained decision</span></details>,
}));

const row = {
  issue: { id: "a", identifier: "MUL-1", title: "Review this task", admission_status: "pending", priority: "none", revision: 1 },
  entered_at: "2026-10-04T10:00:00Z", first_entered_at: "2026-10-04T10:00:00Z", source: "manual", round: 1,
};
const listResponse = { items: [row], total: 1, counts: { pending: 1, ready: 1, snoozed: 0 } };
const historyEntry: TriageHistoryEntry = {
  id: "event-1", kind: "action", issue_id: "a", identifier: "MUL-1", title: "Review this task", action: "accept",
  actor_id: "user-1", created_at: "2026-10-04T10:00:00Z", reason: null, before: {}, after: {}, batch_id: null, filename: null, counts: null,
};
const historyResponse = { entries: [historyEntry], total: 1, offset: 0, limit: 50 };
let client: QueryClient;
let listItems: ReturnType<typeof vi.fn>;
let listHistory: ReturnType<typeof vi.fn>;
let getSettings: ReturnType<typeof vi.fn>;

beforeEach(() => {
  state.params = new URLSearchParams();
  state.replace.mockReset();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  listItems = vi.fn().mockResolvedValue(listResponse);
  listHistory = vi.fn().mockResolvedValue(historyResponse);
  getSettings = vi.fn().mockResolvedValue({ supported: true, enabled: true });
  setApiInstance({
    getTriageSettings: getSettings,
    listTriageItems: listItems,
    listTriageHistory: listHistory,
    getTriageItem: vi.fn().mockResolvedValue(row),
    listMembers: vi.fn().mockResolvedValue([]),
  } as unknown as ApiClient);
});
afterEach(() => client.clear());
function mount() {
  return render(<QueryClientProvider client={client}><TriagePage /></QueryClientProvider>);
}
function invalidate(view: "queue" | "history") {
  return client.invalidateQueries({ queryKey: view === "queue" ? triageKeys.lists("ws") : ["triage", "ws", "history"] });
}

it("keeps the queue and selection when a settings refresh fails, then hides them on access loss", async () => {
  mount();
  const row = await screen.findByRole("button", { name: /Review this task/ });
  const selection = screen.getByRole("checkbox", { name: "Select Review this task" });
  fireEvent.click(selection);
  getSettings.mockRejectedValue(new ApiError("settings offline", 500, "Internal Server Error"));
  await act(async () => { await client.invalidateQueries({ queryKey: triageKeys.settings("ws") }); });
  expect(await screen.findByRole("alert")).toHaveTextContent(en.settings_refresh_failed);
  expect(screen.getByRole("button", { name: /Review this task/ })).toBe(row);
  expect(selection).toBeChecked();

  getSettings.mockRejectedValue(new ApiError("access revoked", 403, "Forbidden"));
  fireEvent.click(screen.getByRole("button", { name: en.retry }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(en.access_denied));
  expect(screen.queryByRole("button", { name: /Review this task/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: en.create })).not.toBeInTheDocument();
});

describe.each(["queue", "history"] as const)("Triage %s recovery", (view) => {
  beforeEach(() => { if (view === "history") state.params = new URLSearchParams("view=history"); });

  it("shows a localized cold failure and disables Retry until recovery", async () => {
    const request = view === "queue" ? listItems : listHistory;
    request.mockRejectedValue(new ApiError("internal database failure", 500, "Internal Server Error"));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(view === "queue" ? "Could not load the triage queue" : "Could not load review history");
    expect(screen.queryByText("internal database failure")).not.toBeInTheDocument();
    let finish!: (value: unknown) => void;
    request.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    fireEvent.click(screen.getByRole("button", { name: en.retry }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Retrying..." })).toBeDisabled());
    await act(async () => { finish(view === "queue" ? listResponse : historyResponse); });
    expect(await screen.findByRole("button", { name: /Review this task/ })).toBeVisible();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("retains row identity and selection or expansion through refresh failure and retry", async () => {
    const request = view === "queue" ? listItems : listHistory;
    mount();
    const button = await screen.findByRole("button", { name: /Review this task/ });
    const retained = view === "queue"
      ? screen.getByRole("checkbox", { name: "Select Review this task" })
      : screen.getByText("Decision details").closest("details")!;
    if (view === "queue") fireEvent.click(retained);
    else fireEvent.click(screen.getByText("Decision details"));
    button.focus();
    request.mockRejectedValue(new TypeError("Failed to fetch"));
    await act(async () => { await invalidate(view); });
    expect(await screen.findByRole("alert")).toHaveTextContent(view === "queue" ? "Could not refresh the triage queue" : "Could not refresh review history");
    expect(screen.getByRole("button", { name: /Review this task/ })).toBe(button);
    expect(button).toHaveFocus();
    if (view === "queue") expect(retained).toBeChecked();
    else expect(retained).toHaveAttribute("open");
    request.mockResolvedValue(view === "queue" ? listResponse : historyResponse);
    fireEvent.click(screen.getByRole("button", { name: en.retry }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: /Review this task/ })).toBe(button);
  });

  it.each([403, 404])("removes authorized cached rows after HTTP %s", async (status) => {
    const request = view === "queue" ? listItems : listHistory;
    mount();
    await screen.findByRole("button", { name: /Review this task/ });
    if (view === "queue") fireEvent.click(screen.getByRole("checkbox", { name: "Select Review this task" }));
    request.mockRejectedValue(new ApiError("unavailable", status, "Unavailable"));
    await act(async () => { await invalidate(view); });
    await waitFor(() => expect(screen.queryByRole("button", { name: /Review this task/ })).not.toBeInTheDocument());
    expect(screen.queryByRole("button", { name: en.batch })).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("You no longer have access to this triage data");
  });
});
