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
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ triage: () => "/acme/triage", settings: () => "/acme/settings", issueDetail: (id: string) => `/acme/issues/${id}`, docsPage: (slug: string) => `/acme/docs/${slug}` }) }));
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
vi.mock("./triage-create-dialog", () => ({ TriageCreateDialog: () => <div role="dialog" aria-label="Task intake form" /> }));
vi.mock("./triage-import-dialog", () => ({
  TriageImportDialog: ({ onClose }: { onClose: () => void }) => (
    <div role="dialog" aria-label="CSV intake form"><button onClick={onClose}>Close CSV intake</button></div>
  ),
}));
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
const emptyList = { items: [], total: 0, counts: { pending: 0, ready: 0, snoozed: 0 } };
const historyEntry: TriageHistoryEntry = {
  id: "event-1", kind: "action", issue_id: "a", identifier: "MUL-1", title: "Review this task", action: "accept",
  actor_id: "user-1", created_at: "2026-10-04T10:00:00Z", reason: null, before: {}, after: {}, batch_id: null, filename: null, counts: null,
};
const historyResponse = { entries: [historyEntry], total: 1, offset: 0, limit: 50 };
const emptyHistory = { entries: [], total: 0, offset: 0, limit: 1 };
const welcomeTitle = "Collect first, decide what to do";
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

it("waits for unfiltered review history before welcoming a first submission", async () => {
  listItems.mockResolvedValue(emptyList);
  let finishHistory!: (value: typeof emptyHistory) => void;
  listHistory.mockImplementation(() => new Promise((resolve) => { finishHistory = resolve; }));
  mount();

  await waitFor(() => expect(listHistory).toHaveBeenCalledExactlyOnceWith("ws", { limit: 1 }, expect.anything()));
  expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument();
  await act(async () => finishHistory(emptyHistory));
  expect(await screen.findByRole("heading", { name: welcomeTitle })).toBeVisible();
  expect(screen.getByRole("link", { name: "Read docs" })).toHaveAttribute("href", "/acme/docs/triage");
  expect(screen.queryByRole("region", { name: en.queue_label })).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: en.import_csv }));
  expect(screen.getByRole("dialog", { name: "CSV intake form" })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Close CSV intake" }));
  fireEvent.click(screen.getByRole("button", { name: en.create }));
  expect(screen.getByRole("dialog", { name: "Task intake form" })).toBeVisible();
});

it("does not reuse invalidated empty history after the last task is reviewed", async () => {
  listItems.mockResolvedValue(emptyList);
  listHistory.mockResolvedValue(emptyHistory);
  mount();
  await screen.findByRole("heading", { name: welcomeTitle });

  listItems.mockResolvedValue(listResponse);
  await act(async () => { await client.invalidateQueries({ queryKey: triageKeys.all("ws") }); });
  expect(await screen.findByRole("button", { name: /Review this task/ })).toBeVisible();
  expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument();

  listItems.mockResolvedValue(emptyList);
  let finishHistory!: (value: typeof historyResponse) => void;
  listHistory.mockImplementation(() => new Promise((resolve) => { finishHistory = resolve; }));
  await act(async () => { await client.invalidateQueries({ queryKey: triageKeys.all("ws") }); });
  await waitFor(() => expect(finishHistory).toBeTypeOf("function"));
  expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument();

  await act(async () => finishHistory({ ...historyResponse, limit: 1 }));
  const historyLink = await screen.findByRole("button", { name: "View review history" });
  expect(screen.getByRole("heading", { name: "No tasks ready for review" })).toBeVisible();
  fireEvent.click(historyLink);
  expect(state.replace).toHaveBeenCalledExactlyOnceWith("/acme/triage?view=history");
  expect(screen.getByRole("tab", { name: en.history })).toHaveFocus();
});

it("directs an empty ready queue to snoozed tasks without requesting history", async () => {
  listItems.mockResolvedValue({ ...emptyList, counts: { pending: 1, ready: 0, snoozed: 1 } });
  mount();

  const button = await screen.findByRole("button", { name: "View snoozed tasks" });
  expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument();
  expect(listHistory).not.toHaveBeenCalled();
  fireEvent.click(button);
  expect(state.replace).toHaveBeenCalledExactlyOnceWith("/acme/triage?view=snoozed");
});

it("keeps the history shortcut while sorting or opening filters on an empty ready queue", async () => {
  state.params = new URLSearchParams("sort=newest");
  listItems.mockResolvedValue(emptyList);
  mount();

  const historyLink = await screen.findByRole("button", { name: "View review history" });
  fireEvent.click(screen.getByRole("button", { name: en.filters }));
  expect(historyLink).toBeVisible();
  expect(screen.getByRole("button", { name: en.filters })).toHaveAttribute("aria-expanded", "true");
  expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument();
});

it.each([
  ["a search", "q=missing", en.no_matches],
  ["a saved filter", "source=csv", en.no_matches],
  ["a later page", "offset=50", en.empty],
  ["a task link", "issue=a", en.empty],
  ["another tab", "view=all", en.empty],
])("retains the queue context for %s when there are no rows", async (_case, query, heading) => {
  state.params = new URLSearchParams(query);
  listItems.mockResolvedValue(emptyList);
  listHistory.mockResolvedValue(emptyHistory);
  mount();

  expect(await screen.findByRole("heading", { name: heading })).toBeVisible();
  expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument();
  expect(listHistory).not.toHaveBeenCalled();
});

it("keeps the queue and intake controls when the history check fails", async () => {
  listItems.mockResolvedValue(emptyList);
  listHistory.mockRejectedValue(new ApiError("history failed", 500, "Internal Server Error"));
  mount();

  await waitFor(() => expect(client.getQueryState(triageKeys.history("ws", { limit: 1 }))?.status).toBe("error"));
  expect(screen.getByRole("region", { name: en.queue_label })).toBeVisible();
  expect(screen.getByRole("button", { name: en.create })).toBeVisible();
  expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument();
});

it.each(["queue", "settings", "history"] as const)("does not show a first-use introduction after a cached-empty %s refresh fails", async (source) => {
  listItems.mockResolvedValue(emptyList);
  listHistory.mockResolvedValue(emptyHistory);
  mount();
  await screen.findByRole("heading", { name: welcomeTitle });

  const request = source === "queue" ? listItems : source === "settings" ? getSettings : listHistory;
  request.mockRejectedValue(new ApiError("offline", 500, "Internal Server Error"));
  await act(async () => { await client.invalidateQueries({ queryKey: triageKeys.all("ws") }); });
  await waitFor(() => expect(screen.queryByRole("heading", { name: welcomeTitle })).not.toBeInTheDocument());
  expect(screen.getByRole("region", { name: en.queue_label })).toBeVisible();
  expect(screen.getByRole("button", { name: en.create })).toBeVisible();
});

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
