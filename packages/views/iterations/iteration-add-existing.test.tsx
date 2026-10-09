import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import type { Issue } from "@multica/core/types";
import type { IterationDraft } from "@multica/core/iterations";
import { renderWithI18n } from "../test/i18n";
import { IterationAddExisting } from "./iteration-add-existing";
import { alpha, beta, previewFor, receipt, settings, source, targetA, targetB, ws } from "./test-fixtures";

// Eligibility and pasted-ID rules are canonical in
// packages/core/iterations/candidates.test.ts; this suite covers the wiring.
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: { user: { id: string } }) => unknown) => select({ user: { id: ws } }),
    { getState: () => ({ user: { id: ws } }) },
  ),
}));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => "session",
    getIterationSettings: vi.fn(),
    listIterations: vi.fn(),
    searchIssues: vi.fn(),
    listIssues: vi.fn(),
    getIssue: vi.fn(),
    previewIteration: vi.fn(),
    applyIterationOperation: vi.fn(),
  },
}));

const done: Issue = { ...beta, id: "30000000-0000-4000-8000-000000000003", identifier: "ITR-3", title: "Done task", status: "done", current_iteration_id: null };
const member: Issue = { ...beta, id: "30000000-0000-4000-8000-000000000004", identifier: "ITR-4", title: "Member task", current_iteration_id: targetA.id };
// Server-side truth; search results are snapshots of it at search time.
let server: Map<string, Issue>;
const setServer = (...issues: Issue[]) => { server = new Map(issues.map((issue) => [issue.id, issue])); };
function preview(draft: IterationDraft) {
  const base = previewFor(draft, draft.moves.map((move) => server.get(move.issue_id)!));
  return { ...base, issues: base.issues.map((fact) => ({ ...fact, source_id: server.get(fact.issue_id)!.current_iteration_id ?? null })) };
}

beforeEach(() => {
  vi.resetAllMocks();
  window.localStorage.clear();
  setServer({ ...alpha, current_iteration_id: null }, { ...beta, current_iteration_id: null });
  vi.mocked(api.getIterationSettings).mockResolvedValue(settings);
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [source, targetA, targetB], next_cursor: null });
  vi.mocked(api.listIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.searchIssues).mockImplementation(async () => ({ issues: [...server.values()].map((issue) => ({ ...issue, match_source: "title" })), total: server.size }));
  vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => preview(draft));
  vi.mocked(api.applyIterationOperation).mockResolvedValue(receipt);
});

function mount(iteration: { id: string; status: string } = targetA, open = true) {
  const onAdded = vi.fn();
  renderWithI18n(
    <QueryClientProvider client={new QueryClient()}>
      <IterationAddExisting wsId={ws} iteration={iteration} open={open} available onAdded={onAdded} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  return { user, onAdded, search: (text: string) => user.type(screen.getByRole("textbox", { name: "Search tasks" }), text) };
}
const appliedDraft = () => vi.mocked(api.applyIterationOperation).mock.calls[0]![1].draft;

it("offers unplanned open work as soon as the dialog opens", async () => {
  vi.mocked(api.listIssues).mockResolvedValue({ issues: [...server.values()], total: 45 });
  const { user, onAdded } = mount();
  const unplanned = await screen.findByRole("region", { name: "Open tasks without an iteration" });
  expect(vi.mocked(api.listIssues).mock.calls[0]![0]).toMatchObject({ include_no_iteration: true, sort_by: "updated_at" });
  expect(await within(unplanned).findByText(/Showing the 30 most recently updated/)).toBeInTheDocument();
  await user.click(within(unplanned).getByRole("button", { name: "Select all 2 available tasks" }));
  // Picks visible in the list are not repeated in a second list.
  expect(screen.queryByRole("region", { name: /Also selected/ })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Add 2 tasks" }));
  await waitFor(() => expect(onAdded).toHaveBeenCalledOnce());
  expect(api.searchIssues).not.toHaveBeenCalled();
});

it("reads nothing while the kept-mounted dialog is closed", () => {
  mount(targetA, false);
  expect(api.listIssues).not.toHaveBeenCalled();
});

it("adds several searched tasks to a planned iteration in one click without asking for a reason", async () => {
  const { user, onAdded, search } = mount();
  await search("task");
  await user.click(await screen.findByRole("button", { name: "Select all 2 available tasks" }));
  expect(screen.queryByLabelText("Reason")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Add 2 tasks" }));
  await waitFor(() => expect(onAdded).toHaveBeenCalledOnce());
  expect(appliedDraft().reason).toBeNull();
  expect(appliedDraft().moves.map((move) => [move.issue_id, move.target_id, move.expected_source_id, move.expected_issue_revision])).toEqual([
    [alpha.id, targetA.id, null, alpha.revision],
    [beta.id, targetA.id, null, beta.revision],
  ]);
});

it("explains blocked tasks and requires a reason only to move work out of another iteration", async () => {
  setServer({ ...alpha, current_iteration_id: source.id }, { ...beta, current_iteration_id: null }, done, member);
  const { user, search } = mount();
  await search("task");
  const results = await screen.findByRole("region", { name: "Search results" });
  // Base UI renders the checkbox as a span, so disabled is aria-disabled.
  expect(await within(results).findByRole("checkbox", { name: /Member task.*Already in this iteration/ })).toHaveAttribute("aria-disabled", "true");
  expect(within(results).getByRole("checkbox", { name: /Done task.*Completed tasks can only join an active iteration/ })).toHaveAttribute("aria-disabled", "true");
  await user.click(within(results).getByRole("checkbox", { name: /Alpha task.*Moves from Source iteration/ }));
  const reason = screen.getByLabelText("Reason");
  expect(reason).toBeRequired();
  expect(screen.getByRole("button", { name: "Add 1 task" })).toBeDisabled();
  await user.type(reason, "Pulled forward for launch");
  await user.click(screen.getByRole("button", { name: "Add 1 task" }));
  await waitFor(() => expect(api.applyIterationOperation).toHaveBeenCalledOnce());
  expect(appliedDraft().reason).toBe("Pulled forward for launch");
  expect(appliedDraft().moves).toEqual([expect.objectContaining({ issue_id: alpha.id, expected_source_id: source.id, allow_completed: false })]);
});

it("asks for explicit consent before completed work joins an active iteration", async () => {
  setServer(done);
  const { user, search } = mount({ ...targetA, status: "active" });
  await search("done");
  await user.click(await screen.findByRole("checkbox", { name: /Done task.*Completed/ }));
  expect(screen.getByText("Optional")).toBeInTheDocument();
  const add = screen.getByRole("button", { name: "Add 1 task" });
  expect(add).toBeDisabled();
  await user.click(screen.getByRole("checkbox", { name: "Add 1 completed task to this iteration" }));
  await user.click(add);
  await waitFor(() => expect(api.applyIterationOperation).toHaveBeenCalledOnce());
  expect(appliedDraft().moves).toEqual([expect.objectContaining({ issue_id: done.id, allow_completed: true })]);
});

it("stops instead of applying when a selected task moved after it was shown", async () => {
  const { user, search } = mount();
  await search("Alpha");
  await user.click(await screen.findByRole("checkbox", { name: /Alpha task/ }));
  server.set(alpha.id, { ...server.get(alpha.id)!, current_iteration_id: source.id });
  await user.click(screen.getByRole("button", { name: "Add 1 task" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Some tasks changed since you selected them");
  expect(screen.getByLabelText("Reason")).toBeRequired();
  expect(api.previewIteration).toHaveBeenCalledOnce();
  expect(api.applyIterationOperation).not.toHaveBeenCalled();
});

it("shows server validation on the affected selected row and keeps the selection", async () => {
  vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => {
    const result = preview(draft);
    return draft.moves[0]!.expected_issue_revision === 1 ? result : { ...result, invalid_items: [{ issue_id: alpha.id, code: "iteration_rollover_exhausted" }] };
  });
  const { user, search } = mount();
  await search("Alpha");
  await user.click(await screen.findByRole("checkbox", { name: /Alpha task/ }));
  // Once the search is cleared the pick is no longer visible above, so it is listed separately.
  await user.clear(screen.getByRole("textbox", { name: "Search tasks" }));
  const selected = screen.getByRole("region", { name: /Also selected/ });
  await user.click(screen.getByRole("button", { name: "Add 1 task" }));
  expect(await within(selected).findByRole("alert")).toHaveTextContent("rollover limit");
  expect(within(selected).getByRole("checkbox", { name: /Alpha task/ })).toBeChecked();
  expect(api.applyIterationOperation).not.toHaveBeenCalled();
});
