import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import type { Issue } from "@multica/core/types";
import type { IterationPreview } from "@multica/core/iterations";
import { IterationRecovery } from "./iteration-recovery";
import { IterationAssignment } from "./iteration-assignment";
import {
  alpha,
  beta,
  previewFor,
  receipt,
  settings,
  targetA,
  targetB,
  ws,
} from "./test-fixtures";
import projects from "../locales/en/projects.json";
vi.mock("../i18n", () => ({
  useT: () => ({ t: (fn: (value: typeof projects) => string, variables?: Record<string, string>) => fn(projects).replace(/\{\{(\w+)\}\}/g, (_match, key: string) => variables?.[key] ?? "") }),
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: { user: { id: string } }) => unknown) =>
      select({ user: { id: ws } }),
    { getState: () => ({ user: { id: ws } }) },
  ),
}));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => "session",
    getIterationCapabilities: vi.fn(),
    getIterationSettings: vi.fn(),
    listIterations: vi.fn(),
    getIssue: vi.fn(),
    previewIteration: vi.fn(),
    applyIterationOperation: vi.fn(),
  },
}));
const picker = vi.hoisted(() => ({ filter: undefined as ((issue: Issue) => boolean) | undefined }));
vi.mock("../modals/issue-picker-modal", () => ({ IssuePickerModal: ({ filterIssue }: { filterIssue: (issue: Issue) => boolean }) => { picker.filter = filterIssue; return null; } }));
beforeEach(() => {
  vi.resetAllMocks();
  window.localStorage.clear();
  vi.mocked(api.getIterationCapabilities).mockResolvedValue({
    workspace_id: ws,
    supported: true,
    enabled: true,
    manual: true,
    schema_version: 1,
    atomic_handoff: true,
  });
  vi.mocked(api.getIterationSettings).mockResolvedValue(settings);
  vi.mocked(api.listIterations).mockResolvedValue({
    workspace_id: ws,
    items: [targetA, targetB],
    next_cursor: null,
  });
  vi.mocked(api.getIssue).mockImplementation(async (id) =>
    id === alpha.id ? alpha : beta,
  );
  vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
    previewFor(draft, [alpha, beta].filter((issue) => draft.moves.some((move) => move.issue_id === issue.id))),
  );
  vi.mocked(api.applyIterationOperation).mockResolvedValue(receipt);
});
function mount() {
  const client = new QueryClient();
  const node = (id: string) => (
    <QueryClientProvider client={client}>
      <IterationAssignment wsId={ws} issueId={id} />
    </QueryClientProvider>
  );
  const view = render(node(alpha.id));
  return {
    user: userEvent.setup(),
    rerender: () => view.rerender(node(beta.id)),
  };
}
describe("iteration assignment scope and preview", () => {
  it("hides terminal candidates unless completed work is explicitly added to an active iteration", async () => {
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [{ ...targetA, status: "active" }, targetB], next_cursor: null });
    render(<QueryClientProvider client={new QueryClient()}><IterationAssignment wsId={ws} /></QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(await screen.findByText("Assign to iteration"));
    expect(picker.filter!({ ...alpha, status: "done" })).toBe(false);
    expect(picker.filter!({ ...alpha, status: "cancelled" })).toBe(false);
    await user.click(screen.getByRole("combobox", { name: "Iterations" }));
    await user.click(await screen.findByRole("option", { name: targetA.name }));
    await user.click(screen.getByRole("checkbox", { name: "Confirm adding completed work" }));
    expect(picker.filter!({ ...alpha, status: "done" })).toBe(true);
    expect(picker.filter!({ ...alpha, status: "cancelled" })).toBe(false);
    await user.click(screen.getByRole("combobox", { name: "Iterations" }));
    await user.click(await screen.findByRole("option", { name: targetB.name }));
    expect(picker.filter!({ ...alpha, status: "done" })).toBe(false);
  });
  it("previews 1000 UUID-selected tasks in two bulk requests without per-task reads", async () => {
    const tasks = Array.from({ length: 1000 }, (_, index) => ({ ...alpha, id: `30000000-0000-4000-8000-${String(index + 1).padStart(12, "0")}` }));
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => previewFor(draft, tasks));
    render(<QueryClientProvider client={new QueryClient()}><IterationAssignment wsId={ws} issueIds={tasks.map((issue) => issue.id)} /></QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(await screen.findByText("Assign to iteration"));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Assign to iteration" });
    expect(api.previewIteration).toHaveBeenCalledTimes(2);
    expect(api.getIssue).not.toHaveBeenCalled();
    expect(vi.mocked(api.previewIteration).mock.calls[1]![1].moves).toHaveLength(1000);
    expect(vi.mocked(api.previewIteration).mock.calls[1]![1].moves[0]?.expected_issue_revision).toBe(alpha.revision);
  });
  it("resolves readable aliases before complete bulk preview", async () => {
    vi.mocked(api.getIssue).mockResolvedValue(alpha);
    render(<QueryClientProvider client={new QueryClient()}><IterationAssignment wsId={ws} issueIds={[alpha.identifier]} /></QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(await screen.findByText("Assign to iteration"));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Assign to iteration" });
    expect(api.getIssue).toHaveBeenCalledExactlyOnceWith(alpha.identifier);
    expect(api.previewIteration).toHaveBeenCalledTimes(2);
  });
  it("rejects an incomplete preliminary set without creating a confirmable preview", async () => {
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => previewFor(draft, []));
    const { user } = mount();
    await user.click(await screen.findByText("Assign to iteration"));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("alert");
    expect(screen.queryByRole("button", { name: "Assign to iteration" })).not.toBeInTheDocument();
    expect(api.applyIterationOperation).not.toHaveBeenCalled();
  });
  it("shows processing and locks global recovery while the original request is in flight", async () => {
    let finish!: (value: typeof receipt) => void;
    vi.mocked(api.applyIterationOperation).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    render(<QueryClientProvider client={new QueryClient()}><IterationRecovery wsId={ws} /><IterationAssignment wsId={ws} issueId={alpha.id} /></QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(await screen.findByText("Assign to iteration"));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await user.click(await screen.findByRole("button", { name: "Assign to iteration" }));
    const recovery = await screen.findByRole("region", { name: "Unconfirmed iteration requests" });
    expect(within(recovery).getByRole("status")).toHaveTextContent("Processing changes");
    expect(within(recovery).getByRole("button", { name: /Check request: Assign to iteration/ })).toBeDisabled();
    await act(async () => finish(receipt));
    await waitFor(() => expect(screen.queryByRole("region", { name: "Unconfirmed iteration requests" })).not.toBeInTheDocument());
  });
  it("shows actual current membership and retains it as the initial selection", async () => {
    render(<QueryClientProvider client={new QueryClient()}><IterationAssignment wsId={ws} issueId={alpha.id} currentIterationId={targetA.id} rolloverCount={3} /></QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(await screen.findByText("Assign to iteration"));
    const currentRow = (await screen.findByText("Current iteration")).parentElement!;
    expect(within(currentRow).getByText(targetA.name)).toHaveAttribute("title", targetA.name);
    expect(screen.getByLabelText("Iterations")).toHaveTextContent(targetA.name);
    expect(screen.getByText(/Rolled over at least three times/)).toBeInTheDocument();
  });
  it("previews the exact batch selection without asking users to paste task IDs", async () => {
    render(<QueryClientProvider client={new QueryClient()}><IterationAssignment wsId={ws} issueIds={[alpha.id, beta.id]} /></QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(await screen.findByText("Assign to iteration"));
    expect(screen.queryByLabelText("Task IDs")).not.toBeInTheDocument();
    await user.click(screen.getByRole("combobox", { name: "Iterations" }));
    await user.click(await screen.findByRole("option", { name: targetA.name }));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Assign to iteration" });
    expect(vi.mocked(api.previewIteration).mock.calls.at(-1)![1].moves.map((move) => move.issue_id)).toEqual([alpha.id, beta.id]);
  });
  it("rebinds hidden issue identity and local choices when IssueDetail changes task", async () => {
    const { user, rerender } = mount();
    await user.click(await screen.findByText("Assign to iteration"));
    await user.type(screen.getByLabelText("Reason"), "Alpha assignment");
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Assign to iteration" });
    rerender();
    expect(
      screen.queryByRole("button", { name: "Assign to iteration" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByText("Assign to iteration"));
    expect(screen.getByLabelText("Reason")).toHaveValue("");
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Assign to iteration" });
    expect(api.getIssue).not.toHaveBeenCalled();
    expect(
      vi.mocked(api.previewIteration).mock.calls.at(-1)![1].moves[0]?.issue_id,
    ).toBe(beta.id);
  });
  it("locks choices until a deferred preview returns and confirms the displayed destination", async () => {
    let finish!: () => void;
    vi.mocked(api.previewIteration).mockImplementationOnce(
      (_ws, draft) =>
        new Promise<IterationPreview>((resolve) => {
          finish = () => resolve(previewFor(draft, [alpha]));
        }),
    );
    const { user } = mount();
    await user.click(await screen.findByText("Assign to iteration"));
    await user.click(screen.getByRole("combobox", { name: "Iterations" }));
    await user.click(await screen.findByRole("option", { name: targetA.name }));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await waitFor(() => expect(api.previewIteration).toHaveBeenCalled());
    expect(screen.getByLabelText("Iterations")).toBeDisabled();
    expect(screen.getByLabelText("Reason")).toBeDisabled();
    await user.click(screen.getByRole("combobox", { name: "Iterations" }));
    expect(screen.queryByRole("option", { name: targetB.name })).not.toBeInTheDocument();
    await act(async () => finish());
    expect(screen.getByLabelText("Iterations")).toHaveTextContent(targetA.name);
    await user.click(
      await screen.findByRole("button", { name: "Assign to iteration" }),
    );
    expect(
      vi.mocked(api.applyIterationOperation).mock.calls[0]![1].draft.moves[0]
        ?.target_id,
    ).toBe(targetA.id);
  });
});
