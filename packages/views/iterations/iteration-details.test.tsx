import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { IterationEventsView } from "./iteration-events-view";
import { IterationIssueList } from "./iteration-issue-list";
import { IterationParticipation } from "./iteration-participation";
import { IterationCurrentComparison } from "./iteration-current-comparison";
import { alpha, beta, issuePage, source, ws } from "./test-fixtures";
import projects from "../locales/en/projects.json";
import issues from "../locales/en/issues.json";

const completeState = vi.hoisted(() => ({ label: false, custom: false, groupReads: 0 }));
vi.mock("@multica/core/issue-statuses/hooks", () => ({ useIssueStatuses: () => ({ entryOf: (key: string) => ({ name: key === "qa" ? "QA" : "Review" }) }) }));
vi.mock("../navigation", () => ({ AppLink: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ iterationDetail: (id: string) => `/iterations/${id}`, issueDetail: (id: string) => `/issues/${id}` }) }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: Object.assign((select: (state: { user: { id: string } }) => unknown) => select({ user: { id: ws } }), { getState: () => ({ user: { id: ws } }) }) }));
vi.mock("@multica/core/api", async (original) => ({
  ...(await original<typeof import("@multica/core/api")>()),
  api: { listMembers: vi.fn(), getBaseUrl: () => "test", getSessionScope: () => "session", getIterationCapabilities: vi.fn(), listIterations: vi.fn(), getIterationIssues: vi.fn(), getIssue: vi.fn() },
}));
// Complete-pagination and revision fencing have their canonical tests in core.
vi.mock("@multica/core/iterations", async (original) => ({
  ...(await original<typeof import("@multica/core/iterations")>()),
  iterationGroupedIssuesOptions: (_ws: string, id: string, params: Record<string, string>) => ({ queryKey: ["iterations", _ws, "grouped", id, params], queryFn: async () => {
    completeState.groupReads++;
    return { ...issuePage([alpha, { ...beta, status: "done" }]), filter_options: metadataFor(params), items: issuePage([alpha, { ...beta, status: "done" }]).items.map((item, index) => ({ ...item, ...(completeState.label && params.scope !== "original" ? { labels: [{ id: ws, name: "Launch" }] } : {}), ...(completeState.custom ? { status_key: index ? "review" : "qa", status_category: "in_review" } : {}) })) };
  } }),
}));
function metadataFor(params: Record<string, string> = {}) {
  return { statuses: completeState.custom ? ["qa", "review"] : ["todo", "done"], projects: [{ id: null, name: null }], assignees: [{ type: null, id: null, name: null }], labels: completeState.label && params.scope !== "original" ? [{ id: ws, name: "Launch" }] : [] };
}
async function selectOption(user: ReturnType<typeof userEvent.setup>, name: string, option: string) {
  await user.click(screen.getByRole("combobox", { name }));
  await user.click(await screen.findByRole("option", { name: option }));
}
const translations = createInstance();
beforeAll(async () => {
  await translations.init({ lng: "en", fallbackLng: "en", resources: { en: { projects, issues } }, interpolation: { escapeValue: false } });
});
function mount(node: React.ReactNode, client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
  render(<I18nextProvider i18n={translations}><QueryClientProvider client={client}>{node}</QueryClientProvider></I18nextProvider>);
  return userEvent.setup();
}
beforeEach(() => {
  vi.resetAllMocks();
  completeState.label = false;
  completeState.custom = false;
  completeState.groupReads = 0;
  vi.mocked(api.listMembers).mockResolvedValue([]);
  vi.mocked(api.getIterationCapabilities).mockResolvedValue({ workspace_id: ws, schema_version: 1, supported: true, enabled: false, manual: true, atomic_handoff: true });
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [{ ...source, status: "completed" }], next_cursor: null });
  vi.mocked(api.getIterationIssues).mockImplementation(async (_ws, _id, params = {}) => ({ ...issuePage([alpha], "next"), filter_options: metadataFor(params) }));
  vi.mocked(api.getIssue).mockResolvedValue({ ...alpha, title: "Renamed current task", status_category: "done" });
});

describe("iteration complete details", () => {
  it("labels task search visibly with a matching accessible name and placeholder", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    const search = screen.getByRole("textbox", { name: "Search tasks" });
    expect(screen.getByText("Search tasks", { selector: "label" })).toBeVisible();
    expect(search).toHaveAttribute("placeholder", "Search tasks");
    await user.type(search, "Alpha");
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { search: "Alpha" }, expect.anything()));
  });
  it("uses complete response choices without fetching all task pages when filters open or scope changes", async () => {
    completeState.label = true;
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    const initialReads = vi.mocked(api.getIterationIssues).mock.calls.length;
    await user.click(screen.getByText("Filter and group tasks"));
    expect(completeState.groupReads).toBe(0);
    expect(api.getIterationIssues).toHaveBeenCalledTimes(initialReads);
    await user.click(screen.getByRole("combobox", { name: "Label" }));
    expect(await screen.findByRole("option", { name: "Launch" })).toBeVisible();
    await user.keyboard("{Escape}");
    await selectOption(user, "Current scope", "Original commitment");
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { scope: "original" }, expect.anything()));
    expect(completeState.groupReads).toBe(0);
    expect(vi.mocked(api.getIterationIssues).mock.calls.every(([, , params]) => params?.limit !== "100")).toBe(true);
  });
  it("explains unavailable old-server choices while preserving search, priority and grouping", async () => {
    vi.mocked(api.getIterationIssues).mockResolvedValue(issuePage([alpha]));
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    await user.click(screen.getByText("Filter and group tasks"));
    expect(screen.getByText(projects.iterations.audit.advancedFiltersUnavailable)).toBeVisible();
    expect(screen.getByRole("combobox", { name: "Label" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "Group tasks by" })).not.toBeDisabled();
    await selectOption(user, "Priority", "High");
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { priority: "high" }, expect.anything()));
    expect(completeState.groupReads).toBe(0);
  });
  it("returns to the previous page and resets page history after search", async () => {
    vi.mocked(api.getIterationIssues).mockImplementation(async (_ws, _id, params = {}) => ({ ...issuePage(params.cursor ? [beta] : [alpha], params.cursor ? null : "next"), filter_options: metadataFor(params) }));
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    expect(await screen.findByRole("button", { name: "Previous page" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next page" }));
    await screen.findByRole("link", { name: `${beta.identifier} · ${beta.title}` });
    await user.click(screen.getByRole("button", { name: "Previous page" }));
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next page" }));
    await screen.findByRole("link", { name: `${beta.identifier} · ${beta.title}` });
    await user.type(screen.getByRole("textbox", { name: "Search tasks" }), "A");
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { search: "A" }, expect.anything());
  });
  it("restarts from the first page and clears history on a stale-cursor retry", async () => {
    vi.mocked(api.getIterationIssues).mockImplementation(async (_ws, _id, params = {}) => {
      if (params.cursor) throw new ApiError("Stale", 409, "Conflict", { code: "cursor_stale" });
      return { ...issuePage([alpha], "next"), filter_options: metadataFor(params) };
    });
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(await screen.findByRole("button", { name: "Next page" }));
    await user.click(await screen.findByRole("button", { name: "Retry" }));
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, {}, expect.anything());
  });
  it("clears previous-page history when scope, grouping or advanced filters change", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(await screen.findByRole("button", { name: "Next page" }));
    expect(screen.getByRole("button", { name: "Previous page" })).not.toBeDisabled();
    await selectOption(user, "Current scope", "Original commitment");
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { scope: "original" }, expect.anything()));
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next page" }));
    await user.click(screen.getByText("Filter and group tasks"));
    await selectOption(user, "Group tasks by", "Status");
    await screen.findByRole("heading", { name: "Done (1)" });
    expect(screen.queryByRole("button", { name: "Previous page" })).not.toBeInTheDocument();
    await selectOption(user, "Group tasks by", "No grouping");
    expect(await screen.findByRole("button", { name: "Previous page" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next page" }));
    await selectOption(user, "Priority", "High");
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { scope: "original", priority: "high" }, expect.anything()));
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
  });
  it("retains authorized rows and filter choices after a temporary refresh error", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />, client);
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    vi.mocked(api.getIterationIssues).mockRejectedValue(new ApiError("Unavailable", 503, "Unavailable"));
    await client.invalidateQueries({ queryKey: ["iterations", ws] });
    const retry = await screen.findByRole("button", { name: "Retry" });
    expect(screen.getByRole("link", { name: `${alpha.identifier} · ${alpha.title}` })).toBeVisible();
    await user.click(screen.getByText("Filter and group tasks"));
    expect(screen.getByRole("combobox", { name: "Status" })).not.toBeDisabled();
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([alpha], "next"), filter_options: metadataFor() });
    await user.click(retry);
    await waitFor(() => expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument());
  });
  it("removes protected rows and stored choices after access is revoked", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />, client);
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    vi.mocked(api.getIterationIssues).mockRejectedValue(new ApiError("Denied", 403, "Forbidden"));
    await client.invalidateQueries({ queryKey: ["iterations", ws] });
    await screen.findByRole("alert");
    expect(screen.queryByRole("link", { name: `${alpha.identifier} · ${alpha.title}` })).not.toBeInTheDocument();
    await user.click(screen.getByText("Filter and group tasks"));
    expect(screen.getByRole("combobox", { name: "Status" })).toBeDisabled();
  });
  it("keeps distinct custom status groups even when both share a category", async () => {
    completeState.custom = true;
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(screen.getByText("Filter and group tasks"));
    await selectOption(user, "Group tasks by", "Status");
    expect(await screen.findByRole("heading", { name: "QA (1)" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Review (1)" })).toBeInTheDocument();
  });
  it("retains a missing selected label visibly after changing scope", async () => {
    completeState.label = true;
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(screen.getByText("Filter and group tasks"));
    await selectOption(user, "Label", "Launch");
    await selectOption(user, "Current scope", "Original commitment");
    await user.click(screen.getByRole("combobox", { name: "Label" }));
    expect(await screen.findByRole("option", { name: "Selected value has no matches" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.getByRole("combobox", { name: "Label" })).toHaveTextContent("Selected value has no matches");
  });
  it("distinguishes cancelling an iteration from cancelling a task in audit labels", async () => {
    const event = { id: ws, sequence: 1, iteration_id: source.id, issue_id: null, operation_id: ws, kind: "cancel", actor: { type: "member", id: ws }, occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z", before_facts: null, after_facts: null, reason: "Schedule changed" };
    mount(<IterationEventsView wsId={ws} events={[event, { ...event, id: alpha.id, sequence: 2, operation_id: alpha.id, issue_id: alpha.id }]} timezone="UTC" />);
    expect(screen.getByText("Iteration cancelled")).toBeVisible();
    expect(screen.getByText("Task cancelled")).toBeVisible();
  });
  it("renders actor, saved-clock time and named membership transitions, preserving unknown history", async () => {
    const event = { id: ws, sequence: 1, iteration_id: source.id, issue_id: alpha.id, operation_id: ws, kind: "join", actor: { type: "member", id: ws }, occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z", before_facts: null, after_facts: { source_iteration_id: null, target_iteration_id: source.id }, reason: "Prioritized for launch" };
    mount(<IterationEventsView wsId={ws} events={[event, { ...event, id: alpha.id, sequence: 2, operation_id: alpha.id, kind: "issue_changed", before_facts: {}, after_facts: {} }]} timezone="Asia/Shanghai" />);
    expect(await screen.findByRole("link", { name: source.name })).toHaveAttribute("href", `/iterations/${source.id}`);
    expect(screen.getByText("Added to iteration")).toBeInTheDocument();
    expect(screen.getAllByText("Prioritized for launch")).toHaveLength(2);
    const moved = screen.getByText("Added to iteration").closest("li")!;
    expect(within(moved).getByText("Member")).toBeVisible();
    expect(moved.querySelector("time")).toHaveTextContent("02:05");
    expect(moved.querySelector("time")).toHaveAttribute("title", "Asia/Shanghai");
    expect(screen.getByRole("heading", { name: "Oct 7, 2026" })).toBeVisible();
    const changed = screen.getByText("Task updated").closest("li")!;
    expect(within(changed).queryByText("Not recorded in this snapshot")).not.toBeInTheDocument();
    expect(within(changed).queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("No current iteration")).toBeInTheDocument();
  });
  it("keeps missing references visible for an actual legacy membership change", () => {
    mount(<IterationEventsView wsId={ws} events={[{ id: ws, sequence: 1, iteration_id: source.id, issue_id: alpha.id, operation_id: ws, kind: "join", actor: null, occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z", before_facts: null, after_facts: { source_iteration_id: null }, reason: null }]} timezone="UTC" />);
    expect(screen.getByText("Not recorded in this snapshot")).toBeVisible();
    expect(screen.getByText("No current iteration")).toBeVisible();
  });
  it("identifies event tasks from frozen facts and safely falls back without fetching current tasks", async () => {
    const event = { id: ws, sequence: 1, iteration_id: source.id, issue_id: alpha.id, operation_id: ws, kind: "issue_changed", actor: { type: "member", id: ws }, occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z", before_facts: { title: "Earlier frozen title" }, after_facts: { identifier: "OLD-1", title: "Frozen updated title" }, reason: null };
    const user = mount(<IterationEventsView wsId={ws} events={[
      event,
      { ...event, id: alpha.id, sequence: 2, operation_id: alpha.id, kind: "delete", issue_id: beta.id, before_facts: { title: "Deleted frozen title" }, after_facts: null },
      { ...event, id: beta.id, sequence: 3, operation_id: beta.id, before_facts: { title: 7 }, after_facts: { identifier: {}, title: false } },
    ]} timezone="UTC" />);
    expect(screen.getByText("OLD-1").parentElement).toHaveTextContent("Frozen updated title");
    expect(screen.getByText("Deleted frozen title")).toBeVisible();
    expect(screen.getByText("Earlier frozen title")).toBeVisible();
    screen.getAllByText(alpha.id).forEach((id) => expect(id).not.toBeVisible());
    const unnamed = screen.getByText("Task title not recorded").closest("li")!;
    await user.click(within(unnamed).getByText("Technical audit", { selector: "summary" }));
    expect(within(unnamed).getByText(alpha.id)).toBeVisible();
    expect(api.getIssue).not.toHaveBeenCalled();
  });
  it("groups the entire filtered result, including rows beyond the visible page", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    expect(screen.queryByRole("link", { name: `${beta.identifier} · ${beta.title}` })).not.toBeInTheDocument();
    await user.click(screen.getByText("Filter and group tasks"));
    await selectOption(user, "Group tasks by", "Status");
    expect(await screen.findByRole("link", { name: `${beta.identifier} · ${beta.title}` })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Done (1)" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Next page" })).not.toBeInTheDocument();
    expect(screen.getByText(/Summary metrics always describe/)).toBeInTheDocument();
  });
  it("marks uncaptured historical labels unknown without enriching from live tasks", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical />);
    await user.click(screen.getByText("Filter and group tasks"));
    await selectOption(user, "Group tasks by", "Label");
    expect(await screen.findByRole("heading", { name: "Not recorded in this snapshot (2)" })).toBeInTheDocument();
    expect(api.getIssue).not.toHaveBeenCalled();
  });
  it("sends filters to the server and resets pagination without filtering the summary", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(await screen.findByRole("button", { name: "Next page" }));
    await user.click(screen.getByText("Filter and group tasks"));
    await selectOption(user, "Priority", "High");
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { priority: "high" }, expect.anything()));
  });
  it("keeps participation links available when planning is disabled", async () => {
    const user = mount(<IterationParticipation wsId={ws} issueId={alpha.id} />);
    const disclosure = await screen.findByText("Iteration participation history");
    expect(api.listIterations).not.toHaveBeenCalled();
    await user.click(disclosure);
    expect(await screen.findByRole("link", { name: source.name })).toHaveAttribute("href", `/iterations/${source.id}`);
    expect(api.listIterations).toHaveBeenCalledWith(ws, { issue_id: alpha.id }, expect.anything());
  });
  it("compares frozen task facts without replacing them with the current record", async () => {
    const frozen = issuePage([alpha]).items[0]!;
    const user = mount(<IterationCurrentComparison wsId={ws} issue={frozen} />);
    await user.click(screen.getByRole("button", { name: "Compare with current task" }));
    const current = await screen.findByText("Current value");
    expect(within(current.parentElement!).getByText(/Renamed current task/)).toBeInTheDocument();
    expect(screen.getByText("Historical value").parentElement).toHaveTextContent(alpha.title);
  });
  it("keeps missing stored reference names unknown while explicit unassigned values remain none", async () => {
    const frozen = { ...issuePage([alpha]).items[0]!, project_id: ws, project_name: null, assignee_type: "member", assignee_id: ws, assignee_name: null };
    const user = mount(<IterationCurrentComparison wsId={ws} issue={frozen} />);
    await user.click(screen.getByRole("button", { name: "Compare with current task" }));
    const historical = await screen.findByRole("heading", { name: "Historical value" });
    const current = screen.getByRole("heading", { name: "Current value" });
    for (const field of ["Project", "Assignee"]) {
      expect(within(historical.parentElement!).getByText(field).parentElement).toHaveTextContent("Not recorded in this snapshot");
      expect(within(current.parentElement!).getByText(field).parentElement).toHaveTextContent("None");
    }
  });
  it("explains deleted or inaccessible live tasks without discarding frozen history", async () => {
    vi.mocked(api.getIssue).mockRejectedValue(new ApiError("Missing", 404, "Not Found"));
    const user = mount(<IterationCurrentComparison wsId={ws} issue={issuePage([alpha]).items[0]!} />);
    await user.click(screen.getByRole("button", { name: "Compare with current task" }));
    expect(await screen.findByText(/Task deleted or no longer accessible/)).toBeInTheDocument();
  });
});
