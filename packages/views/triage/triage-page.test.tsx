import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { TriageHistoryEntry } from "@multica/core/triage";
import { beforeEach, expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import { TriagePage } from "./triage-page";

const state = vi.hoisted(() => ({
  enabled: true,
  compact: false,
  role: "member" as string | null,
  admissionStatus: "pending",
  reviewerId: null as string | null,
  snoozedUntil: null as string | null,
  replace: vi.fn(),
  params: new URLSearchParams("issue=a"),
  rows: ["a", "b"],
  total: 2,
  historyEntries: [] as TriageHistoryEntry[],
  historyTotal: 0,
}));
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (
      selector: (value: typeof en) => string,
      values?: Record<string, string | number>,
    ) =>
      selector(en).replace(/{{(\w+)}}/g, (_, key: string) =>
        String(values?.[key] ?? key),
      ),
    i18n: { language: "en" },
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    triage: () => "/acme/triage",
    settings: () => "/acme/settings",
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: state.role }),
}));
vi.mock("../navigation", () => ({
  useNavigation: () => ({ searchParams: state.params, replace: state.replace }),
  AppLink: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));
vi.mock("../layout/page-header", () => ({
  PAGE_GUTTER: "px-4",
  PageHeader: ({ children }: { children: React.ReactNode }) => (
    <header>{children}</header>
  ),
}));
vi.mock("@multica/ui/hooks/use-mobile", () => ({ useIsCompact: () => state.compact }));
vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizablePanel: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizableHandle: () => null,
}));
vi.mock("../issues/components/issue-detail", () => ({
  IssueDetail: ({
    reviewContext,
    leadingAction,
  }: {
    reviewContext?: React.ReactNode;
    leadingAction?: React.ReactNode;
  }) => (
    <section aria-label="Task detail">
      {leadingAction}
      {reviewContext}
      <input aria-label="Description" />
    </section>
  ),
}));
vi.mock("./triage-filters", () => ({
  TriageFilters: ({ history }: { history: boolean }) => (
    <div role="group" aria-label={history ? "History filters" : "Queue filters"} />
  ),
}));
vi.mock("./triage-history", () => ({
  TriageItemHistory: () => null,
  TriageHistoryRecord: () => null,
  TriageHistoryContent: () => null,
  TriageExecutionStatus: () => null,
  TriageDuplicateTarget: () => null,
}));
vi.mock("./triage-create-dialog", () => ({
  TriageCreateDialog: () => <div role="dialog">Create form</div>,
}));
vi.mock("./triage-import-dialog", () => ({
  TriageImportDialog: ({
    batchId,
    onClose,
  }: {
    batchId?: string;
    onClose: () => void;
  }) => (
    <div role="dialog">
      <span>{batchId}</span>
      <button onClick={onClose}>Close import</button>
    </div>
  ),
}));
vi.mock("./triage-batch-dialog", () => ({
  TriageBatchDialog: ({ items }: { items: { issue: { title: string } }[] }) => (
    <div role="dialog">{items.map((item) => item.issue.title).join(", ")}</div>
  ),
}));
vi.mock("./triage-action-dialog", () => ({
  TriageActionDialog: ({
    action,
    onSuccess,
    onClose,
  }: {
    action: string;
    onSuccess: (value: unknown) => void;
    onClose: () => void;
  }) => (
    <div role="dialog">
      <span>{`Action ${action}`}</span>
      <button
        onClick={() => {
          onSuccess({
            item: { issue: { id: "a" } },
            action: { action, execution_status: "not_requested" },
          });
          onClose();
        }}
      >
        Confirm decision
      </button>
    </div>
  ),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: ({ queryKey }: { queryKey: string[] }) => {
    const kind = queryKey[2];
    const item = (id: string) => ({
      issue: {
        id,
        identifier: `MUL-${id}`,
        title: `Task ${id}`,
        admission_status: state.admissionStatus,
        priority: "none",
        revision: 1,
      },
      entered_at: "2026-10-04T10:00:00Z",
      first_entered_at: "2026-10-04T10:00:00Z",
      source: "manual",
      round: 1,
      reviewer_id: state.reviewerId,
      reviewer_valid: true,
      snoozed_until: state.snoozedUntil,
    });
    if (kind === "settings")
      return {
        data: {
          supported: true,
          enabled: state.enabled,
          acceptance_status: "backlog",
        },
      };
    if (kind === "counts")
      return {
        data: {
          pending: state.rows.length,
          ready: state.rows.length,
          snoozed: 0,
        },
      };
    if (kind === "list")
      return {
        data: { items: state.rows.map(item), total: state.total },
      };
    if (kind === "detail") return { data: item("a") };
    if (kind === "history")
      return {
        data: { entries: state.historyEntries, total: state.historyTotal },
      };
    if (queryKey.at(-1) === "members")
      return { data: [{ user_id: "reviewer-1", name: "Avery" }] };
    return { data: [] };
  },
}));

beforeEach(() => {
  state.enabled = true;
  state.compact = false;
  state.role = "member";
  state.admissionStatus = "pending";
  state.reviewerId = null;
  state.snoozedUntil = null;
  state.rows = ["a", "b"];
  state.total = 2;
  state.historyEntries = [];
  state.historyTotal = 0;
  state.params = new URLSearchParams("issue=a");
  state.replace.mockClear();
});
it("keeps history available but hides intake controls when disabled", () => {
  state.enabled = false;
  render(<TriagePage />);
  expect(
    screen.queryByRole("button", { name: "Create pending task" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "History" })).toBeVisible();
  expect(screen.getByText(en.disabled)).toBeVisible();
});
it("runs queue shortcuts only from queue focus and advances only after a confirmed decision", () => {
  render(<TriagePage />);
  const editor = screen.getByRole("textbox", { name: "Description" });
  editor.focus();
  fireEvent.keyDown(editor, { key: "3" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  const row = screen.getByRole("button", { name: /MUL-a Task a/ });
  row.focus();
  fireEvent.keyDown(row, { key: "3" });
  expect(screen.getByText("Action reject")).toBeVisible();
  expect(state.replace).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Confirm decision" }));
  expect(state.replace).toHaveBeenCalledWith("/acme/triage?issue=b");
});

it("keeps search, sort and expanded filters with the queue", () => {
  render(<TriagePage />);
  const queue = screen.getByRole("region", { name: en.queue_label });
  const search = within(queue).getByRole("textbox", { name: en.search });
  expect(within(queue).getByRole("combobox", { name: en.sort })).toBeVisible();
  fireEvent.click(within(queue).getByRole("button", { name: en.filters }));
  expect(within(queue).getByRole("group", { name: "Queue filters" })).toBeVisible();
  search.focus();
  fireEvent.keyDown(search, { key: "3" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("keeps compact review focused on detail and restores queue controls with Back", () => {
  state.compact = true;
  render(<TriagePage />);
  expect(screen.queryByRole("textbox", { name: en.search })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: en.filters })).not.toBeInTheDocument();
  const detail = screen.getByRole("region", { name: "Task detail" });
  fireEvent.click(within(detail).getByRole("button", { name: en.back }));
  expect(state.replace).toHaveBeenCalledExactlyOnceWith("/acme/triage");
  expect(screen.getByRole("textbox", { name: en.search })).toBeVisible();
  expect(screen.queryByRole("region", { name: "Task detail" })).not.toBeInTheDocument();
});

it("does not run queue decision shortcuts while a closed sort picker has focus", () => {
  render(<TriagePage />);
  const sort = screen.getByRole("combobox", { name: en.sort });
  sort.focus();
  fireEvent.keyDown(sort, { key: "3" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(state.replace).not.toHaveBeenCalled();
});

it("opens the explicit execute decision through the acceptance menu by keyboard", async () => {
  const user = userEvent.setup();
  render(<TriagePage />);
  expect(screen.queryByRole("button", { name: en.accept_and_execute })).not.toBeInTheDocument();
  const trigger = screen.getByRole("button", { name: en.accept_options });
  trigger.focus();
  await user.keyboard("{ArrowDown}");
  expect(screen.getByRole("menuitem", { name: en.accept_and_execute })).toHaveFocus();
  await user.keyboard("{Escape}");
  expect(trigger).toHaveFocus();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  await user.keyboard("{ArrowDown}");
  const execute = screen.getByRole("menuitem", { name: en.accept_and_execute });
  expect(execute).toHaveFocus();
  await user.keyboard("{Enter}");
  expect(screen.getByText("Action accept_and_execute")).toBeVisible();
  expect(state.replace).not.toHaveBeenCalled();
});

it("places reviewer assignment in review context and labels the queue reviewer", () => {
  state.reviewerId = "reviewer-1";
  render(<TriagePage />);
  const detail = screen.getByRole("region", { name: "Task detail" });
  expect(within(detail).getByText(en.pending)).toBeVisible();
  const reviewer = within(detail).getByRole("button", { name: en.assign_reviewer });
  expect(reviewer).toHaveTextContent("Reviewer: Avery");
  expect(screen.getByRole("button", { name: /MUL-a Task a/ })).toHaveTextContent("Reviewer: Avery");
  expect(screen.queryByText(en.priorities.none)).not.toBeInTheDocument();
  fireEvent.click(reviewer);
  expect(screen.getByText("Action assign_reviewer")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Confirm decision" }));
  expect(state.replace).not.toHaveBeenCalled();
});

it("keeps selected queue items available to batch review", () => {
  render(<TriagePage />);
  fireEvent.click(screen.getByRole("checkbox", { name: "Select Task b" }));
  fireEvent.click(screen.getByRole("button", { name: en.batch }));
  expect(screen.getByRole("dialog")).toHaveTextContent("Task b");
  expect(screen.getByRole("dialog")).not.toHaveTextContent("Task a");
});

it("returns from full-width history detail without losing history conditions", () => {
  state.params = new URLSearchParams("view=history&issue=a&q=launch&offset=50");
  render(<TriagePage />);
  expect(screen.queryByRole("region", { name: en.queue_label })).not.toBeInTheDocument();
  expect(screen.getByRole("textbox", { name: en.search })).toHaveValue("launch");
  const detail = screen.getByRole("region", { name: "Task detail" });
  fireEvent.click(within(detail).getByRole("button", { name: en.history }));
  expect(state.replace).toHaveBeenCalledExactlyOnceWith(
    "/acme/triage?view=history&q=launch&offset=50",
  );
});

it("keeps reviewer and review actions unavailable without membership", () => {
  state.role = null;
  render(<TriagePage />);
  for (const name of [en.accept, en.reject, en.duplicate, en.snooze, en.assign_reviewer]) {
    expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
  }
  const row = screen.getByRole("button", { name: /MUL-a Task a/ });
  row.focus();
  fireEvent.keyDown(row, { key: "1" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it.each([
  { admission: "pending", action: "unsnooze", label: en.unsnooze },
  { admission: "rejected", action: "reopen", label: en.reopen },
])("preserves $action recovery", ({ admission, action, label }) => {
  state.admissionStatus = admission;
  state.snoozedUntil = "2026-10-12T10:00:00Z";
  render(<TriagePage />);
  fireEvent.click(screen.getByRole("button", { name: label }));
  expect(screen.getByText(`Action ${action}`)).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Confirm decision" }));
  expect(state.replace).not.toHaveBeenCalled();
});

it("opens a notification batch deep link and clears it on close", () => {
  state.params = new URLSearchParams("view=history&batch=batch-1");
  render(<TriagePage />);
  expect(screen.getByRole("dialog")).toHaveTextContent("batch-1");
  fireEvent.click(screen.getByRole("button", { name: "Close import" }));
  expect(state.replace).toHaveBeenCalledWith("/acme/triage?view=history");
});

it("keeps a cleared search as the latest intent through delayed route acknowledgements and a review decision", () => {
  const { rerender } = render(<TriagePage />);
  const search = screen.getByRole("textbox", { name: en.search });
  fireEvent.change(search, { target: { value: "jk123" } });
  fireEvent.change(search, { target: { value: "jk1231" } });
  fireEvent.change(search, { target: { value: "" } });
  // The first platform replacement is still pending while later input accumulates.
  expect(state.replace).toHaveBeenCalledTimes(1);
  state.params = new URLSearchParams("issue=a&q=jk123");
  rerender(<TriagePage />);
  expect(search).toHaveValue("");
  expect(state.replace).toHaveBeenLastCalledWith("/acme/triage?issue=a");
  state.params = new URLSearchParams("issue=a");
  rerender(<TriagePage />);
  fireEvent.click(screen.getByRole("button", { name: "Accept" }));
  fireEvent.click(screen.getByRole("button", { name: "Confirm decision" }));
  const target = state.replace.mock.calls.at(-1)?.[0] as string;
  expect(new URL(target, "https://example.test").searchParams.has("q")).toBe(
    false,
  );
  expect(target).toBe("/acme/triage?issue=b");
});

it("adopts external route changes instead of pinning the original filters", () => {
  const { rerender } = render(<TriagePage />);
  state.params = new URLSearchParams("view=snoozed&q=external");
  rerender(<TriagePage />);
  expect(screen.getByRole("textbox", { name: en.search })).toHaveValue(
    "external",
  );
});

it("reserves the chat launcher corner while enabled pagination advances the queue", () => {
  state.total = 101;
  render(<TriagePage />);

  const next = screen.getByRole("button", { name: en.next });
  expect(next).toBeEnabled();
  expect(next.parentElement?.parentElement).toHaveClass("pe-chat-launcher");
  fireEvent.click(next);
  expect(state.replace).toHaveBeenCalledWith("/acme/triage?issue=a&offset=50");
});

// The field allowlist/date preservation matrix is owned by triage-ui.test.ts.
// This checks the summary and reset against the actual intended-route hook.
it("shows collapsed history conditions and keeps clearing as the latest URL intent", () => {
  state.params = new URLSearchParams({
    view: "history",
    q: "launch",
    source: "csv",
    result: "accept",
    processed_by: "reviewer-1",
    processed_after: "2026-10-01",
    processed_before: "2026-10-09",
    offset: "50",
    priority: "high",
    from: "saved-link",
  });
  const { rerender } = render(<TriagePage />);
  expect(screen.getByRole("button", { name: en.filters })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  const summary = screen.getByRole("list", { name: en.history_filters.label });
  expect(within(summary).getAllByRole("listitem")).toHaveLength(6);
  for (const text of [
    "Search: launch",
    "Source: CSV",
    "Result: Accept",
    "Processed by: Avery",
    "Processed after: 2026-10-01",
    "Processed before: 2026-10-09",
  ]) {
    expect(within(summary).getByText(text)).toBeVisible();
  }

  const search = screen.getByRole("textbox", { name: en.search });
  fireEvent.change(search, { target: { value: "new launch" } });
  const pendingUrl = String(state.replace.mock.calls[0]?.[0]);
  fireEvent.click(screen.getAllByRole("button", { name: en.clear_filters })[0]!);
  expect(search).toHaveValue("");
  expect(screen.queryByRole("list", { name: en.history_filters.label })).not.toBeInTheDocument();
  expect(state.replace).toHaveBeenCalledTimes(1);

  state.params = new URL(pendingUrl, "https://example.test").searchParams;
  rerender(<TriagePage />);
  expect(search).toHaveValue("");
  expect(screen.queryByRole("list", { name: en.history_filters.label })).not.toBeInTheDocument();
  expect(state.replace).toHaveBeenLastCalledWith(
    "/acme/triage?view=history&priority=high&from=saved-link",
  );
});

it("distinguishes empty history from filtered no matches and offers a direct reset", () => {
  state.params = new URLSearchParams("view=history&priority=high");
  const { rerender } = render(<TriagePage />);
  expect(screen.getByRole("heading", { name: en.history_empty })).toBeVisible();
  expect(screen.queryByRole("button", { name: en.clear_filters })).not.toBeInTheDocument();

  state.params = new URLSearchParams("view=history&result=reject&offset=50&from=saved-link");
  rerender(<TriagePage />);
  const emptyState = screen.getByRole("heading", { name: en.history_no_matches }).parentElement!;
  expect(within(emptyState).getByText(en.history_no_matches_description)).toBeVisible();
  fireEvent.click(within(emptyState).getByRole("button", { name: en.clear_filters }));
  expect(state.replace).toHaveBeenLastCalledWith(
    "/acme/triage?view=history&from=saved-link",
  );
  expect(screen.getByRole("heading", { name: en.history_empty })).toBeVisible();
});

it.each([
  { name: "unfiltered", query: "" },
  {
    name: "filtered",
    query: "q=launch&source=csv&result=reject&processed_by=reviewer-1&processed_after=2026-10-01&processed_before=2026-10-09",
  },
])("recovers an empty history page with $name results by changing only the offset", ({ query }) => {
  state.params = new URLSearchParams(
    "view=history&offset=150&from=saved-link&priority=high",
  );
  new URLSearchParams(query).forEach((value, key) => state.params.set(key, value));
  state.historyTotal = 12;
  const expectedParams = new URLSearchParams(state.params);
  expectedParams.delete("offset");
  render(<TriagePage />);

  const emptyState = screen.getByRole("heading", {
    name: en.history_page_empty,
  }).parentElement!;
  expect(screen.queryByRole("heading", { name: en.history_empty })).not.toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: en.history_no_matches })).not.toBeInTheDocument();
  expect(within(emptyState).queryByRole("button", { name: en.clear_filters })).not.toBeInTheDocument();
  fireEvent.click(within(emptyState).getByRole("button", { name: en.history_first_page }));

  expect(state.replace).toHaveBeenCalledExactlyOnceWith(
    `/acme/triage?${expectedParams.toString()}`,
  );
  expect(screen.getByRole("textbox", { name: en.search })).toHaveValue(
    expectedParams.get("q") ?? "",
  );
});

it("keeps empty history page recovery ahead of a delayed pagination acknowledgement", () => {
  state.params = new URLSearchParams(
    "view=history&offset=150&q=launch&source=csv&from=saved-link",
  );
  state.historyTotal = 12;
  const { rerender } = render(<TriagePage />);
  fireEvent.click(screen.getByRole("button", { name: en.previous }));
  const pendingUrl = String(state.replace.mock.calls[0]?.[0]);
  fireEvent.click(screen.getByRole("button", { name: en.history_first_page }));
  expect(state.replace).toHaveBeenCalledTimes(1);

  state.params = new URL(pendingUrl, "https://example.test").searchParams;
  rerender(<TriagePage />);
  expect(state.replace).toHaveBeenLastCalledWith(
    "/acme/triage?view=history&q=launch&source=csv&from=saved-link",
  );
  expect(screen.getByRole("textbox", { name: en.search })).toHaveValue("launch");
  const summary = screen.getByRole("list", { name: en.history_filters.label });
  expect(within(summary).getByText("Search: launch")).toBeVisible();
  expect(within(summary).getByText("Source: CSV")).toBeVisible();
});

it("preserves history pagination and existing import and task destinations", () => {
  const entry: TriageHistoryEntry = {
    id: "event-a",
    kind: "action",
    action: "accept",
    issue_id: "a",
    identifier: "MUL-a",
    title: "Task a",
    actor_id: "reviewer-1",
    created_at: "2026-10-09T15:00:00Z",
    reason: null,
    before: {},
    after: {},
    batch_id: null,
    filename: null,
    counts: null,
  };
  state.params = new URLSearchParams("view=history");
  state.historyEntries = [
    entry,
    {
      ...entry,
      id: "import-a",
      kind: "import",
      action: "import",
      issue_id: null,
      batch_id: "batch-a",
      filename: "launch.csv",
    },
  ];
  state.historyTotal = 101;
  const { rerender } = render(<TriagePage />);
  fireEvent.click(screen.getByRole("button", { name: en.next }));
  expect(state.replace).toHaveBeenCalledExactlyOnceWith("/acme/triage?view=history&offset=50");
  state.params = new URLSearchParams("view=history&offset=50");
  rerender(<TriagePage />);

  fireEvent.click(screen.getByRole("button", { name: "CSV import: launch.csv" }));
  expect(screen.getByRole("dialog")).toHaveTextContent("batch-a");
  fireEvent.click(screen.getByRole("button", { name: "Close import" }));
  fireEvent.click(screen.getByRole("button", { name: "MUL-a Task a" }));
  expect(state.replace).toHaveBeenLastCalledWith("/acme/triage?view=history&offset=50&issue=a");
});
