import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { IterationEventsView } from "./iteration-events-view";
import { IterationIssueList } from "./iteration-issue-list";
import { IterationParticipation } from "./iteration-participation";
import { IterationCurrentComparison } from "./iteration-current-comparison";
import { alpha, beta, issuePage, source, ws } from "./test-fixtures";
import projects from "../locales/en/projects.json";
import issues from "../locales/en/issues.json";

const completeState = vi.hoisted(() => ({ label: false, custom: false }));
vi.mock("@multica/core/issue-statuses/hooks", () => ({ useIssueStatuses: () => ({ entryOf: (key: string) => ({ name: key === "qa" ? "QA" : "Review" }) }) }));
vi.mock("../i18n", () => ({ useLocale: () => "en", useT: (namespace: string) => ({ t: (fn: (value: unknown) => string) => fn(namespace === "issues" ? issues : projects) }) }));
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
  iterationGroupedIssuesOptions: (_ws: string, id: string, params: Record<string, string>) => ({ queryKey: ["iterations", _ws, "grouped", id, params], queryFn: async () => ({ ...issuePage([alpha, { ...beta, status: "done" }]), items: issuePage([alpha, { ...beta, status: "done" }]).items.map((item, index) => ({ ...item, ...(completeState.label && params.scope !== "original" ? { labels: [{ id: ws, name: "Launch" }] } : {}), ...(completeState.custom ? { status_key: index ? "review" : "qa", status_category: "in_review" } : {}) })) }) }),
}));
function mount(node: React.ReactNode) {
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{node}</QueryClientProvider>);
  return userEvent.setup();
}
beforeEach(() => {
  vi.resetAllMocks();
  completeState.label = false;
  completeState.custom = false;
  vi.mocked(api.listMembers).mockResolvedValue([]);
  vi.mocked(api.getIterationCapabilities).mockResolvedValue({ workspace_id: ws, schema_version: 1, supported: true, enabled: false, manual: true, atomic_handoff: true });
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [{ ...source, status: "completed" }], next_cursor: null });
  vi.mocked(api.getIterationIssues).mockResolvedValue(issuePage([alpha], "next"));
  vi.mocked(api.getIssue).mockResolvedValue({ ...alpha, title: "Renamed current task", status_category: "done" });
});

describe("iteration complete details", () => {
  it("keeps distinct custom status groups even when both share a category", async () => {
    completeState.custom = true;
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(screen.getByText("Filter and group tasks"));
    await user.selectOptions(screen.getByLabelText("Group tasks by"), "status");
    expect(await screen.findByRole("heading", { name: "QA (1)" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Review (1)" })).toBeInTheDocument();
  });
  it("retains a missing selected label visibly after changing scope", async () => {
    completeState.label = true;
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(screen.getByText("Filter and group tasks"));
    await screen.findByRole("option", { name: "Launch" });
    await user.selectOptions(screen.getByLabelText("Label"), ws);
    await user.selectOptions(screen.getByLabelText("Current scope"), "original");
    expect(await screen.findByRole("option", { name: "Selected value has no matches" })).toBeInTheDocument();
    expect(screen.getByLabelText("Label")).toHaveValue(ws);
  });
  it("distinguishes cancelling an iteration from cancelling a task in audit labels", async () => {
    const event = { id: ws, sequence: 1, iteration_id: source.id, issue_id: null, operation_id: ws, kind: "cancel", actor: { type: "member", id: ws }, occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z", before_facts: null, after_facts: null, reason: "Schedule changed" };
    mount(<IterationEventsView wsId={ws} events={[event, { ...event, id: alpha.id, issue_id: alpha.id }]} timezone="UTC" />);
    expect(screen.getByText("Iteration cancelled")).toBeInTheDocument();
    expect(screen.getByText("Task cancelled")).toBeInTheDocument();
  });
  it("renders actor, saved-clock time and named membership transitions, preserving unknown history", async () => {
    const event = { id: ws, sequence: 1, iteration_id: source.id, issue_id: alpha.id, operation_id: ws, kind: "join", actor: { type: "member", id: ws }, occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z", before_facts: null, after_facts: { source_iteration_id: null, target_iteration_id: source.id }, reason: "Prioritized for launch" };
    mount(<IterationEventsView wsId={ws} events={[event, { ...event, id: alpha.id, kind: "issue_changed", before_facts: {}, after_facts: {} }]} timezone="Asia/Shanghai" />);
    expect(await screen.findByRole("link", { name: source.name })).toHaveAttribute("href", `/iterations/${source.id}`);
    expect(screen.getByText("Added to iteration")).toBeInTheDocument();
    expect(screen.getAllByText("Prioritized for launch")).toHaveLength(2);
    expect(screen.getAllByText(/Actor:/)[0]).toHaveTextContent(`member · ${ws}`);
    expect(screen.getAllByText(/Actor:/)[0]).toHaveTextContent("02:05");
    expect(screen.getAllByText("Not recorded in this snapshot")).toHaveLength(2);
    expect(screen.getByText("No current iteration")).toBeInTheDocument();
  });
  it("identifies event tasks from frozen facts and safely falls back without fetching current tasks", () => {
    const event = { id: ws, sequence: 1, iteration_id: source.id, issue_id: alpha.id, operation_id: ws, kind: "issue_changed", actor: { type: "member", id: ws }, occurred_at: "2026-10-06T18:05:00Z", sampled_at: "2026-10-06T18:05:00Z", before_facts: { title: "Earlier frozen title" }, after_facts: { identifier: "OLD-1", title: "Frozen updated title" }, reason: null };
    mount(<IterationEventsView wsId={ws} events={[
      event,
      { ...event, id: alpha.id, kind: "delete", issue_id: beta.id, before_facts: { title: "Deleted frozen title" }, after_facts: null },
      { ...event, id: beta.id, before_facts: { title: 7 }, after_facts: { identifier: {}, title: false } },
    ]} timezone="UTC" />);
    expect(screen.getByText("Task: OLD-1 · Frozen updated title")).toBeInTheDocument();
    expect(screen.getByText(`Task: ${beta.id} · Deleted frozen title`)).toBeInTheDocument();
    expect(screen.getByText(`Task: ${alpha.id}`)).toBeInTheDocument();
    expect(screen.queryByText("Earlier frozen title")).not.toBeInTheDocument();
    expect(api.getIssue).not.toHaveBeenCalled();
  });
  it("groups the entire filtered result, including rows beyond the visible page", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await screen.findByRole("link", { name: `${alpha.identifier} · ${alpha.title}` });
    expect(screen.queryByRole("link", { name: `${beta.identifier} · ${beta.title}` })).not.toBeInTheDocument();
    await user.click(screen.getByText("Filter and group tasks"));
    await user.selectOptions(screen.getByLabelText("Group tasks by"), "status");
    expect(await screen.findByRole("link", { name: `${beta.identifier} · ${beta.title}` })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Done (1)" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Next page" })).not.toBeInTheDocument();
    expect(screen.getByText(/Summary metrics always describe/)).toBeInTheDocument();
  });
  it("marks uncaptured historical labels unknown without enriching from live tasks", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical />);
    await user.click(screen.getByText("Filter and group tasks"));
    await user.selectOptions(screen.getByLabelText("Group tasks by"), "label");
    expect(await screen.findByRole("heading", { name: "Not recorded in this snapshot (2)" })).toBeInTheDocument();
    expect(api.getIssue).not.toHaveBeenCalled();
  });
  it("sends filters to the server and resets pagination without filtering the summary", async () => {
    const user = mount(<IterationIssueList wsId={ws} id={source.id} historical={false} />);
    await user.click(await screen.findByRole("button", { name: "Next page" }));
    await user.click(screen.getByText("Filter and group tasks"));
    await user.selectOptions(screen.getByLabelText("Priority"), "high");
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
  it("explains deleted or inaccessible live tasks without discarding frozen history", async () => {
    vi.mocked(api.getIssue).mockRejectedValue(new ApiError("Missing", 404, "Not Found"));
    const user = mount(<IterationCurrentComparison wsId={ws} issue={issuePage([alpha]).items[0]!} />);
    await user.click(screen.getByRole("button", { name: "Compare with current task" }));
    expect(await screen.findByText(/Task deleted or no longer accessible/)).toBeInTheDocument();
  });
});
