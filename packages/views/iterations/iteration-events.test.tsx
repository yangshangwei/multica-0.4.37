import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { iterationActivityOptions, type IterationEvent } from "@multica/core/iterations";
import { IterationEventsPanel } from "./iteration-events";
import { alpha, issuePage, source, statistics, targetA, ws } from "./test-fixtures";
import projects from "../locales/en/projects.json";
import chineseProjects from "../locales/zh-Hans/projects.json";
import issues from "../locales/en/issues.json";
import chineseIssues from "../locales/zh-Hans/issues.json";

// Projection/classification and traversal matrices live in core/iterations/activity.test.ts.
vi.mock("../navigation", () => ({ AppLink: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ iterationDetail: (id: string) => `/iterations/${id}` }) }));
vi.mock("@multica/core/api", async (original) => ({
  ...await original<typeof import("@multica/core/api")>(),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => "session",
    getIterationEvents: vi.fn(),
    listIterations: vi.fn(),
    listMembers: vi.fn(),
    listAgents: vi.fn(),
    listSquads: vi.fn(),
    getIssue: vi.fn(),
  },
}));
const translations = createInstance();
beforeAll(async () => {
  await translations.init({
    lng: "en", fallbackLng: "en", interpolation: { escapeValue: false },
    resources: { en: { projects, issues }, "zh-Hans": { projects: chineseProjects, issues: chineseIssues } },
  });
});
const clients: QueryClient[] = [];
beforeEach(async () => {
  vi.resetAllMocks();
  await translations.changeLanguage("en");
  vi.mocked(api.getIterationEvents).mockResolvedValue(page([]));
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [source, targetA], next_cursor: null });
  vi.mocked(api.listMembers).mockResolvedValue([]);
  vi.mocked(api.listAgents).mockResolvedValue([]);
  vi.mocked(api.listSquads).mockResolvedValue([]);
});
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); });

function event(sequence: number, overrides: Partial<IterationEvent> = {}): IterationEvent {
  return {
    id: `50000000-0000-4000-8000-${String(sequence).padStart(12, "0")}`,
    sequence, iteration_id: source.id, issue_id: alpha.id,
    operation_id: `60000000-0000-4000-8000-${String(sequence).padStart(12, "0")}`,
    kind: "join", actor: { type: "system", id: null, source: "test" },
    occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z",
    before_facts: null,
    after_facts: { title: `Historical task ${sequence}`, source_iteration_id: null, target_iteration_id: source.id },
    reason: null,
    ...overrides,
  };
}
const page = (items: IterationEvent[], next_cursor: string | null = null, iterationId = source.id) => ({ workspace_id: ws, iteration_id: iterationId, items, next_cursor });
type Props = React.ComponentProps<typeof IterationEventsPanel>;
const defaults: Props = { wsId: ws, id: source.id, timezone: "Asia/Shanghai", statistics, snapshot: null };
function mount(initial: Partial<Props> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const node = (props: Partial<Props>) => <I18nextProvider i18n={translations}><QueryClientProvider client={client}><IterationEventsPanel {...defaults} {...props} /></QueryClientProvider></I18nextProvider>;
  const view = render(node(initial));
  return { client, user: userEvent.setup(), rerender: (props: Partial<Props>) => view.rerender(node(props)) };
}
const snapshot = (overrides: Partial<NonNullable<Props["snapshot"]>> = {}): NonNullable<Props["snapshot"]> => ({
  schema_version: 1, workspace_id: ws, iteration_id: source.id, operation_id: ws,
  end_type: "completed", reason: "Finished", logical_ended_at: "2026-10-07T01:00:00Z", processed_at: "2026-10-07T01:00:00Z",
  original: issuePage([alpha]).items, scope: issuePage([alpha]).items, events: [event(1)], statistics, destinations: [],
  ...overrides,
});

describe("iteration activity panel", () => {
  it("filters a complete chronology and reveals every original record in a cross-page operation", async () => {
    const operation = event(1).operation_id;
    const baseline = event(1, { kind: "baseline", after_facts: { title: "Frozen commitment" } });
    const start = event(2, { kind: "start", issue_id: null, operation_id: operation, after_facts: null });
    const edit = event(3, { kind: "issue_changed", before_facts: { title: "Draft" }, after_facts: { title: "Updated title" } });
    vi.mocked(api.getIterationEvents).mockResolvedValueOnce(page([baseline], "next")).mockResolvedValueOnce(page([start, edit, event(4)]));
    const { user } = mount();
    expect(await screen.findByText("Historical task 4")).toBeVisible();
    expect(screen.queryByText("Updated title")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Oct 7, 2026" })).toBeVisible();
    const disclosure = screen.getByText("View 2 records", { selector: "summary" });
    expect(screen.getByText("Frozen commitment")).not.toBeVisible();
    disclosure.focus();
    expect(disclosure).toHaveFocus();
    await user.click(disclosure);
    expect(screen.getByText("Frozen commitment")).toBeVisible();
    expect(screen.getByText("Original commitment recorded")).toBeVisible();
    const all = screen.getByRole("button", { name: "All activity" });
    all.focus();
    await user.keyboard("{Enter}");
    expect(all).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("Updated title", { selector: "p" })).toBeVisible();
    expect(screen.getByRole("status")).toHaveTextContent("4 records");
    await user.click(screen.getAllByText("Event details", { selector: "summary" })[0]!);
    expect(screen.getByText(event(4).id)).toBeVisible();
    expect(api.getIssue).not.toHaveBeenCalled();
  });

  it("keeps every authoritative count available with explicit task/event units and snapshot precedence", async () => {
    const frozen = { ...statistics, original: 4, initial_effective: 4, current: 5, cancelled: 1, effective: 4, completed: 2, original_completed: 1, remaining: 2, added_unique: 1, removed_events: 0, reentry_events: 2, cancel_events: 1, reopen_events: 3, started: 6, net_effective_change: 0, net_effective_change_ratio: 0 };
    const { user, rerender } = mount({ statistics: { ...statistics, effective: 99 }, snapshot: snapshot({ statistics: frozen }) });
    const summary = screen.getByRole("region", { name: "Effective scope summary" });
    for (const [label, value] of [["Initial effective scope", "4 tasks"], ["Effective scope at closure", "4 tasks"], ["Net effective change", "0 tasks"], ["Distinct tasks added", "1 task"], ["Cancellations", "1 event"]]) {
      expect(within(summary).getByText(label!).parentElement).toHaveTextContent(value!);
    }
    const disclosure = screen.getByText("View all scope counts", { selector: "summary" });
    const counts = disclosure.closest("details")!;
    expect(within(counts).getByText("Removed during iteration")).not.toBeVisible();
    await user.click(disclosure);
    for (const [label, value] of [["Scope at closure", "5 tasks"], ["Cancelled tasks", "1 task"], ["Removed during iteration", "0 events"], ["Re-entries", "2 events"], ["Reopens", "3 events"], ["Started tasks", "6 tasks"], ["Effective scope change rate", "0%"]]) {
      expect(within(counts).getByText(label!).parentElement).toHaveTextContent(value!);
    }
    rerender({ statistics: { ...statistics, effective: 98, removed_events: 97 }, snapshot: snapshot({ statistics: frozen }) });
    expect(screen.queryByText(/9[789] tasks/)).not.toBeInTheDocument();
    expect(api.getIterationEvents).not.toHaveBeenCalled();
  });

  it("appends older groups locally and retains visible history and the filter through a temporary refresh failure", async () => {
    const items = Array.from({ length: 15 }, (_, index) => event(index + 1));
    vi.mocked(api.getIterationEvents).mockResolvedValue(page(items));
    const { client, user } = mount();
    await screen.findByText("Historical task 15");
    expect(screen.queryByText("Historical task 1")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Show more activity" }));
    expect(screen.getByText("Historical task 1")).toBeVisible();
    expect(screen.getByText("Historical task 15")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "All activity" }));
    await user.click(within(screen.getByText("Historical task 15").closest("li")!).getByText("Event details", { selector: "summary" }));
    expect(screen.getByText(event(15).id)).toBeVisible();
    vi.mocked(api.getIterationEvents).mockRejectedValue(new ApiError("Busy", 429, "Too Many Requests"));
    await act(() => client.invalidateQueries({ queryKey: iterationActivityOptions(ws, source.id).queryKey }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh activity");
    expect(screen.getByText("Historical task 1")).toBeVisible();
    expect(screen.getByRole("button", { name: "All activity" })).toHaveAttribute("aria-pressed", "true");
    let finish!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.getIterationEvents).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(screen.getByRole("button", { name: "Retry" })).toBeDisabled();
    expect(screen.getByText("Historical task 1")).toBeVisible();
    await act(async () => { finish(page([...items, event(16)])); });
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(screen.getByText("Historical task 1")).toBeVisible();
    expect(screen.getByText(event(15).id)).toBeVisible();
  });

  it.each([403, 404])("removes protected rows and scope data after a definitive %s", async (status) => {
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([event(1)]));
    const { client } = mount();
    await screen.findByText("Historical task 1");
    vi.mocked(api.getIterationEvents).mockRejectedValue(new ApiError("Unavailable", status, "Unavailable"));
    await act(() => client.invalidateQueries({ queryKey: iterationActivityOptions(ws, source.id).queryKey }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.queryByText("Historical task 1")).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Effective scope summary" })).not.toBeInTheDocument();
    expect(client.getQueryData(iterationActivityOptions(ws, source.id).queryKey)).toBeUndefined();
  });

  it("distinguishes first loading, first-read failure and a successfully empty history", async () => {
    let fail!: (error: Error) => void;
    vi.mocked(api.getIterationEvents).mockImplementationOnce(() => new Promise((_, reject) => { fail = reject; }));
    const { user } = mount();
    expect(screen.getByRole("status")).toHaveTextContent("Loading activity");
    expect(screen.queryByText("No activity recorded yet.")).not.toBeInTheDocument();
    await act(async () => { fail(new Error("offline")); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load activity");
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([]));
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("No activity recorded yet.")).toBeVisible();
  });

  it("offers all activity when there are no scope matches, including unknown event kinds", async () => {
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([event(1, { kind: "future_kind", before_facts: {}, after_facts: { title: "Future event" } })]));
    const { user } = mount();
    expect(await screen.findByText("No scope changes recorded.")).toBeVisible();
    expect(screen.queryByText("No activity recorded yet.")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "All activity" }));
    expect(screen.getByText("Future event", { selector: "p" })).toBeVisible();
    await user.click(screen.getByText("Event details", { selector: "summary" }));
    expect(screen.getByText("future_kind")).toBeVisible();
  });

  it("resets filtering and local pagination when iteration identity changes", async () => {
    vi.mocked(api.getIterationEvents).mockImplementation(async (_ws, id) => page([event(1, {
      iteration_id: id, kind: "issue_changed", before_facts: {}, after_facts: { title: id === source.id ? "First iteration" : "Second iteration" },
    })], null, id));
    const { user, rerender } = mount();
    await screen.findByText("No scope changes recorded.");
    await user.click(screen.getByRole("button", { name: "All activity" }));
    expect(screen.getByText("First iteration", { selector: "p" })).toBeVisible();
    rerender({ id: targetA.id });
    await screen.findByText("No scope changes recorded.");
    expect(screen.getByRole("button", { name: "Scope changes" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByText("First iteration")).not.toBeInTheDocument();
    expect(screen.queryByText("Second iteration")).not.toBeInTheDocument();
  });

  it("shows frozen field changes and localized actor fallbacks without invented current task facts", async () => {
    await translations.changeLanguage("zh-Hans");
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([event(1, {
      kind: "issue_changed", actor: { type: "agent", id: alpha.id, user_id: ws },
      before_facts: { title: "旧标题", status_key: "todo", status_category: "todo", assignee_type: "member", assignee_id: ws, assignee_name: "陈安" },
      after_facts: { title: "保存的标题", status_key: "done", status_category: "done", assignee_type: null, assignee_id: null },
    })]));
    const { user } = mount();
    await screen.findByText("暂无范围变化。");
    await user.click(screen.getByRole("button", { name: "全部活动" }));
    expect(screen.getByText("保存的标题", { selector: "p" })).toBeVisible();
    expect(screen.getByText(chineseIssues.status.todo)).toBeVisible();
    expect(screen.getByText(chineseIssues.status.done)).toBeVisible();
    expect(screen.getByText("陈安")).toBeVisible();
    expect(screen.getByText(/未分配/)).toBeVisible();
    expect(screen.queryByText(/Unknown Agent/)).not.toBeInTheDocument();
    expect(screen.queryByText(ws)).not.toBeInTheDocument();
    expect(api.getIssue).not.toHaveBeenCalled();
  });
});
