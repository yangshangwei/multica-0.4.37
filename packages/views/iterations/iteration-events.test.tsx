import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { iterationActivityOptions, iterationGroupedIssuesOptions, type IterationEvent } from "@multica/core/iterations";
import { IterationEventsPanel } from "./iteration-events";
import { alpha, beta, issuePage, source, statistics, targetA, ws } from "./test-fixtures";
import projects from "../locales/en/projects.json";
import chineseProjects from "../locales/zh-Hans/projects.json";
import issues from "../locales/en/issues.json";
import chineseIssues from "../locales/zh-Hans/issues.json";

// Classification/count matrices live in core/iterations/scope.test.ts; traversal in activity.test.ts.
vi.mock("../navigation", () => ({ AppLink: (props: React.ComponentProps<"a">) => <a {...props} /> }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ iterationDetail: (id: string) => `/iterations/${id}`, issueDetail: (id: string) => `/issues/${id}` }) }));
vi.mock("@multica/core/api", async (original) => ({
  ...await original<typeof import("@multica/core/api")>(),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => "session",
    getIterationEvents: vi.fn(),
    getIterationIssues: vi.fn(),
    getIteration: vi.fn(),
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
  vi.mocked(api.getIterationIssues).mockResolvedValue(issuePage([alpha, beta]));
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
const defaults: Props = { wsId: ws, id: source.id, iteration: source, timezone: "Asia/Shanghai", statistics, snapshot: null };
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

describe("business scope details", () => {
  const planned = { ...source, status: "planned", started_at: null };
  const start = () => event(1, { kind: "start", issue_id: null, before_facts: null, after_facts: null });
  const join = (sequence: number, title = "Late task") => event(sequence, {
    after_facts: { issue_id: alpha.id, identifier: alpha.identifier, title, status_category: "todo", source_iteration_id: null, target_iteration_id: source.id },
  });
  const leave = (sequence: number) => event(sequence, {
    kind: "leave", before_facts: { issue_id: alpha.id, identifier: alpha.identifier, title: "Late task", status_category: "todo", source_iteration_id: source.id, target_iteration_id: null }, after_facts: null,
  });
  const scopeSummary = () => screen.getByRole("region", { name: "Effective scope summary" });
  const metric = (name: string) => within(scopeSummary()).getByRole("button", { name: new RegExp(`^View ${name}:`) });
  const details = () => screen.getByRole("region", { name: "Scope metric details" });

  it("describes a two-task plan without presenting a start baseline or post-start growth", async () => {
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([event(1, { kind: "planned_activity" })]));
    mount({ iteration: planned, statistics: { ...statistics, original: 0, initial_effective: 0, net_effective_change: 2 } });
    expect(metric("Planned tasks")).toHaveTextContent("2 tasks");
    expect(scopeSummary()).toHaveTextContent("Starting this iteration records the original commitment.");
    expect(within(scopeSummary()).queryByText("Scope at start")).not.toBeInTheDocument();
    expect(within(scopeSummary()).queryByText("Net effective change")).not.toBeInTheDocument();
    expect(within(scopeSummary()).queryByText("Tasks added after start")).not.toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Planning adjustments" })).toHaveAttribute("aria-pressed", "true");
    expect(api.getIterationIssues).not.toHaveBeenCalled();
  });

  it("retains planning history after cancellation without claiming a frozen commitment", async () => {
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([event(1, { kind: "planned_activity" })]));
    mount({ iteration: { ...planned, status: "cancelled" }, statistics: { ...statistics, original: 0, current: 0, effective: 0 } });
    expect(scopeSummary()).toHaveTextContent("Cancelled before starting");
    expect(within(scopeSummary()).queryByRole("button")).not.toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Planning adjustments" })).toBeVisible();
    expect(screen.queryByText("Effective scope at closure")).not.toBeInTheDocument();
  });

  it("classifies planned cancellations and restorations without counting them as post-start changes", async () => {
    const change = (sequence: number, before: string, after: string, title: string) => event(sequence, {
      kind: "planned_activity",
      before_facts: { issue_id: alpha.id, title, status_category: before },
      after_facts: { issue_id: alpha.id, title, status_category: after },
    });
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([
      change(1, "todo", "cancelled", "Planned cancellation"),
      change(2, "cancelled", "done", "Planned restoration"),
      change(3, "done", "todo", "Planned reopening"),
      change(4, "future-category", "cancelled", "Unknown transition"),
    ]));
    const { user } = mount({ iteration: planned });
    await user.click(await screen.findByRole("button", { name: "All activity" }));
    const category = screen.getByRole("combobox", { name: "Change category" });
    await user.click(category);
    await user.click(await screen.findByRole("option", { name: "Cancelled" }));
    expect(screen.getByRole("status")).toHaveTextContent("1 matching record");
    expect(screen.getByText("Planned cancellation", { selector: "p" })).toBeVisible();
    expect(screen.queryByText("Unknown transition", { selector: "p" })).not.toBeInTheDocument();
    await user.click(category);
    await user.click(await screen.findByRole("option", { name: "Restored or reopened" }));
    expect(screen.getByRole("status")).toHaveTextContent("2 matching records");
    expect(screen.getByText("Planned restoration", { selector: "p" })).toBeVisible();
    expect(screen.getByText("Planned reopening", { selector: "p" })).toBeVisible();
    expect(within(scopeSummary()).queryByText("Tasks added after start")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Effective scope [+-]/)).not.toBeInTheDocument();
  });

  it("loads exact current members only on demand and combines task search with recorded project and assignee filters", async () => {
    const items = issuePage([alpha, beta]).items.map((issue, index) => ({ ...issue, project_id: ws, project_name: "Recorded project", assignee_type: "member", assignee_id: index ? beta.id : alpha.id, assignee_name: index ? "Bea" : "Ada" }));
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([alpha, beta]), items });
    const { user } = mount();
    expect(api.getIterationIssues).not.toHaveBeenCalled();
    const control = metric("Current effective scope");
    control.focus();
    await user.keyboard("{Enter}");
    const detail = within(details());
    expect(await detail.findByRole("link", { name: /Alpha task/ })).toHaveAttribute("href", `/issues/${alpha.id}`);
    expect(control).toHaveAttribute("aria-pressed", "true");
    expect(control).toHaveAttribute("aria-controls", details().id);
    expect(api.getIterationIssues).toHaveBeenCalledWith(ws, source.id, { scope: "current", limit: "100" }, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    await user.click(detail.getByRole("combobox", { name: "Project" }));
    await user.click(await screen.findByRole("option", { name: "Recorded project" }));
    await user.click(detail.getByRole("combobox", { name: "Assignee" }));
    await user.click(await screen.findByRole("option", { name: "Bea" }));
    await user.type(detail.getByRole("textbox", { name: "Search task records" }), "ITR-2");
    expect(detail.queryByRole("link", { name: /Alpha task/ })).not.toBeInTheDocument();
    expect(detail.getByRole("link", { name: /Beta task/ })).toBeVisible();
    expect(detail.getByRole("status")).toHaveTextContent("1 matching task");
    expect(metric("Current effective scope")).toHaveTextContent("2 tasks");
    await user.type(detail.getByRole("textbox", { name: "Search task records" }), " no match");
    expect(detail.getByText("No matching tasks.")).toBeVisible();
    await user.click(detail.getByRole("button", { name: "Clear filters" }));
    expect(detail.getByRole("status")).toHaveTextContent("2 matching tasks");
    expect(api.getIterationIssues).toHaveBeenCalledTimes(1);
    expect(api.getIssue).not.toHaveBeenCalled();
  });

  it("resets every inline task refinement when the same metric is selected again", async () => {
    const items = issuePage([alpha, beta]).items.map((issue, index) => ({
      ...issue, project_id: index ? beta.id : alpha.id, project_name: index ? "Beta project" : "Alpha project",
      assignee_type: "member", assignee_id: index ? beta.id : alpha.id, assignee_name: index ? "Bea" : "Ada",
    }));
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([alpha, beta]), items });
    const { user } = mount();
    const control = metric("Current effective scope");
    await user.click(control);
    const detail = within(details());
    await detail.findByRole("link", { name: /Beta task/ });
    await user.type(detail.getByRole("textbox", { name: "Search task records" }), "Beta");
    await user.click(detail.getByRole("combobox", { name: "Project" }));
    await user.click(await screen.findByRole("option", { name: "Beta project" }));
    await user.click(detail.getByRole("combobox", { name: "Assignee" }));
    await user.click(await screen.findByRole("option", { name: "Bea" }));
    expect(detail.getByRole("status")).toHaveTextContent("1 matching task");

    await user.click(control);
    expect(detail.getByRole("textbox", { name: "Search task records" })).toHaveValue("");
    for (const name of ["Project", "Assignee"]) expect(detail.getByRole("combobox", { name }).querySelector('[data-slot="select-value"]')).toHaveTextContent("All");
    expect(detail.getByRole("status")).toHaveTextContent("2 matching tasks");
    expect(detail.getByRole("link", { name: /Alpha task/ })).toBeVisible();
    expect(control).toHaveTextContent("2 tasks");
    expect(api.getIterationIssues).toHaveBeenCalledTimes(1);
  });

  it("does not present a mismatched live revision as an exact task set, and retries the complete read", async () => {
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([alpha, beta]), scope_revision: source.scope_revision + 1 });
    const { user } = mount();
    await user.click(metric("Current effective scope"));
    const detail = within(details());
    expect(await detail.findByRole("alert")).toHaveTextContent("The scope changed");
    expect(detail.queryByRole("link")).not.toBeInTheDocument();
    expect(metric("Current effective scope")).toHaveTextContent("2 tasks");
    vi.mocked(api.getIterationIssues).mockResolvedValue(issuePage([alpha, beta]));
    await user.click(detail.getByRole("button", { name: "Retry" }));
    expect(await detail.findByRole("link", { name: /Alpha task/ })).toBeVisible();
  });

  it("retains a selected task set and its search after a temporary refresh failure", async () => {
    const { user, client } = mount();
    await user.click(metric("Current effective scope"));
    const detail = within(details());
    await detail.findByRole("link", { name: /Alpha task/ });
    await user.type(detail.getByRole("textbox", { name: "Search task records" }), "Beta");
    vi.mocked(api.getIterationIssues).mockRejectedValue(new ApiError("Busy", 429, "Too Many Requests"));
    await act(() => client.invalidateQueries({ queryKey: iterationGroupedIssuesOptions(ws, source.id, { scope: "current" }).queryKey }));
    expect(await detail.findByRole("alert")).toHaveTextContent("Previously loaded tasks are still shown");
    expect(detail.getByRole("link", { name: /Beta task/ })).toBeVisible();
    expect(detail.queryByRole("link", { name: /Alpha task/ })).not.toBeInTheDocument();
    let finish!: (value: ReturnType<typeof issuePage>) => void;
    vi.mocked(api.getIterationIssues).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    await user.click(detail.getByRole("button", { name: "Retry" }));
    expect(detail.getByRole("button", { name: "Retry" })).toBeDisabled();
    await act(async () => { finish(issuePage([alpha, beta])); });
    await waitFor(() => expect(detail.queryByRole("alert")).not.toBeInTheDocument());
    expect(detail.getByRole("textbox", { name: "Search task records" })).toHaveValue("Beta");
    expect(metric("Current effective scope")).toHaveAttribute("aria-pressed", "true");
  });

  it("ignores stale live task errors and an in-flight retry after the selected metric becomes frozen", async () => {
    const { user, client, rerender } = mount();
    await user.click(metric("Current effective scope"));
    const detail = within(details());
    await detail.findByRole("link", { name: /Alpha task/ });
    vi.mocked(api.getIterationIssues).mockRejectedValue(new ApiError("Busy", 429, "Too Many Requests"));
    await act(() => client.invalidateQueries({ queryKey: iterationGroupedIssuesOptions(ws, source.id, { scope: "current" }).queryKey }));
    expect(await detail.findByRole("alert")).toHaveTextContent("Previously loaded tasks are still shown");
    let rejectRead!: (error: unknown) => void;
    vi.mocked(api.getIterationIssues).mockImplementation(() => new Promise((_, reject) => { rejectRead = reject; }));
    await user.click(detail.getByRole("button", { name: "Retry" }));
    expect(detail.getByRole("button", { name: "Retry" })).toBeDisabled();

    const frozenScope = issuePage([alpha, beta]).items.map((issue) => ({ ...issue, title: `Frozen ${issue.title}` }));
    rerender({ snapshot: snapshot({ scope: frozenScope }) });
    expect(detail.getByText("Frozen Alpha task")).toBeVisible();
    expect(detail.queryByRole("alert")).not.toBeInTheDocument();
    expect(detail.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    await act(async () => { rejectRead(new ApiError("Still busy", 429, "Too Many Requests")); });
    await waitFor(() => expect(client.getQueryState(iterationGroupedIssuesOptions(ws, source.id, { scope: "current" }).queryKey)?.fetchStatus).toBe("idle"));
    expect(detail.queryByRole("alert")).not.toBeInTheDocument();
    expect(detail.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    expect(detail.getByRole("status")).toHaveTextContent("2 matching tasks");
    expect(metric("Effective scope at closure")).toHaveTextContent("2 tasks");
    expect(api.getIterationIssues).toHaveBeenCalledTimes(3);
    expect(api.getIssue).not.toHaveBeenCalled();
  });

  it("shows loading for an activity metric until its complete history is available", async () => {
    let finish!: (value: ReturnType<typeof page>) => void;
    vi.mocked(api.getIterationEvents).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    const { user } = mount({ statistics: { ...statistics, added_unique: 1 } });
    await user.click(metric("Tasks added after start"));
    expect(within(details()).getByRole("status")).toHaveTextContent("Loading activity");
    expect(within(details()).queryByRole("alert")).not.toBeInTheDocument();
    await act(async () => { finish(page([start(), join(2)])); });
    await waitFor(() => expect(within(details()).getByRole("status")).toHaveTextContent("1 matching task"));
  });

  it("filters operation children without losing the start context or any original record", async () => {
    const operation = ws;
    const baseline = (sequence: number, issue: typeof alpha) => event(sequence, {
      kind: "baseline", operation_id: operation, issue_id: issue.id,
      after_facts: { issue_id: issue.id, identifier: issue.identifier, title: issue.title, status_category: "todo" },
    });
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([{ ...start(), operation_id: operation }, baseline(2, alpha), baseline(3, beta)]));
    const { user } = mount();
    await screen.findByText("View 3 records", { selector: "summary" });
    await user.type(screen.getByRole("textbox", { name: "Search task records" }), "ITR-1");
    expect(screen.getByRole("status")).toHaveTextContent("1 matching record");
    const full = screen.getByText("View all 3 operation records", { selector: "summary" }).closest("details")!;
    expect(within(full).getByText("Beta task")).not.toBeVisible();
    expect(screen.getAllByText("Iteration started").some((node) => node.closest("details") === null)).toBe(true);
    await user.click(within(full).getByText("View all 3 operation records", { selector: "summary" }));
    expect(within(full).getByText("Beta task")).toBeVisible();
    expect(within(full).getAllByText("Technical audit", { selector: "summary" })).toHaveLength(3);
    expect(within(full).getByText(event(3).id)).not.toBeVisible();
  });

  it("counts added tasks once while preserving removal and re-entry occurrences and their audits", async () => {
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([start(), join(2), leave(3), { ...join(4), kind: "reenter" }, leave(5), { ...join(6), kind: "reenter" }]));
    const { user } = mount({ statistics: { ...statistics, original: 0, initial_effective: 0, current: 1, effective: 1, added_unique: 1, removed_events: 2, reentry_events: 2, net_effective_change: 1 } });
    await screen.findAllByText("Late task", { selector: "p" });
    await user.click(metric("Tasks added after start"));
    expect(within(details()).getByRole("status")).toHaveTextContent("1 matching task");
    expect(within(details()).getByText("Late task", { selector: "p" })).toBeVisible();
    await user.click(screen.getByText("View all scope counts", { selector: "summary" }));
    await user.click(metric("Re-entries"));
    expect(within(details()).getByRole("status")).toHaveTextContent("2 matching records");
    const audits = within(details()).getAllByText("Technical audit", { selector: "summary" });
    expect(audits).toHaveLength(2);
    expect(screen.getByText(event(6).id)).not.toBeVisible();
    await user.click(audits[0]!);
    expect(screen.getByText(event(6).id)).toBeVisible();
  });

  it("explains a net-zero change with contributing records and keeps counters unchanged by refinements", async () => {
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([start(), join(2), leave(3)]));
    const { user } = mount({ statistics: { ...statistics, original: 0, initial_effective: 0, current: 0, effective: 0, added_unique: 1, removed_events: 1 } });
    await screen.findAllByText("Late task", { selector: "p" });
    await user.click(metric("Net effective change"));
    const detail = within(details());
    expect(detail.getByRole("status")).toHaveTextContent("2 matching records");
    expect(detail.getByText("Effective scope +1")).toBeVisible();
    expect(detail.getByText("Effective scope -1")).toBeVisible();
    await user.click(detail.getByRole("combobox", { name: "Change category" }));
    await user.click(await screen.findByRole("option", { name: "Removed" }));
    await user.type(detail.getByRole("textbox", { name: "Search task records" }), "ITR-1");
    expect(detail.getByRole("status")).toHaveTextContent("1 matching record");
    expect(metric("Net effective change")).toHaveTextContent("0 tasks");
    await user.type(detail.getByRole("textbox", { name: "Search task records" }), " absent");
    expect(detail.getByText("No matching activity.")).toBeVisible();
    await user.click(detail.getByRole("button", { name: "Clear filters" }));
    expect(detail.getByRole("status")).toHaveTextContent("2 matching records");
  });

  it("keeps incomplete evidence distinct from an authoritative zero", async () => {
    vi.mocked(api.getIterationEvents).mockResolvedValue(page([start(), join(2)]));
    const { user } = mount({ statistics: { ...statistics, added_unique: 2 } });
    await screen.findByText("Late task", { selector: "p" });
    await user.click(metric("Tasks added after start"));
    expect(within(details()).getByRole("alert")).toHaveTextContent("Some supporting records are unavailable");
    expect(metric("Tasks added after start")).toHaveTextContent("2 tasks");
    expect(within(details()).queryByText("No matching activity.")).not.toBeInTheDocument();
  });

  it("removes selected details and counters when the lazy task read revokes access", async () => {
    vi.mocked(api.getIterationIssues).mockRejectedValue(new ApiError("Denied", 403, "Forbidden"));
    const { user } = mount();
    await screen.findByText("No activity recorded yet.");
    await user.click(metric("Current effective scope"));
    expect(await screen.findByRole("alert")).toHaveTextContent("You no longer have access");
    expect(screen.queryByRole("region", { name: "Scope metric details" })).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Effective scope summary" })).not.toBeInTheDocument();
  });

  it("uses frozen original tasks directly and only reads the current task on comparison", async () => {
    const saved = issuePage([alpha]).items.map((issue) => ({ ...issue, title: "Frozen original" }));
    const { user } = mount({ snapshot: snapshot({ original: saved, statistics: { ...statistics, original: 1, initial_effective: 1 } }) });
    await user.click(metric("Scope at start"));
    const detail = within(details());
    expect(detail.getByText("Frozen original")).toBeVisible();
    expect(api.getIterationIssues).not.toHaveBeenCalled();
    expect(api.getIterationEvents).not.toHaveBeenCalled();
    expect(api.getIssue).not.toHaveBeenCalled();
    vi.mocked(api.getIssue).mockResolvedValue({ ...alpha, title: "Live renamed task" });
    await user.click(detail.getByRole("button", { name: "Compare with current task" }));
    expect(await detail.findByText("Live renamed task")).toBeVisible();
    expect(detail.getAllByText("Frozen original")).toHaveLength(2);
  });
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
    await user.click(screen.getAllByText("Technical audit", { selector: "summary" })[0]!);
    expect(screen.getByText(event(4).id)).toBeVisible();
    expect(api.getIssue).not.toHaveBeenCalled();
  });

  it("keeps every authoritative count available with explicit task/event units and snapshot precedence", async () => {
    const frozen = { ...statistics, original: 4, initial_effective: 4, current: 5, cancelled: 1, effective: 4, completed: 2, original_completed: 1, remaining: 2, added_unique: 1, removed_events: 0, reentry_events: 2, cancel_events: 1, reopen_events: 3, started: 6, net_effective_change: 0, net_effective_change_ratio: 0 };
    const { user, rerender } = mount({ statistics: { ...statistics, effective: 99 }, snapshot: snapshot({ statistics: frozen }) });
    const summary = screen.getByRole("region", { name: "Effective scope summary" });
    for (const [label, value] of [["Scope at start", "4 tasks"], ["Effective scope at closure", "4 tasks"], ["Net effective change", "0 tasks"], ["Tasks added after start", "1 task"], ["Cancellations", "1 event"]]) {
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
    await user.click(within(screen.getByText("Historical task 15").closest("li")!).getByText("Technical audit", { selector: "summary" }));
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
    await user.click(screen.getByText("Technical audit", { selector: "summary" }));
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
