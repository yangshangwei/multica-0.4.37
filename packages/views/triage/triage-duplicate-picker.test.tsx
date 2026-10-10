import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import type { Issue, ListIssuesResponse } from "@multica/core/types";
import type { TriageItem, TriageSettings } from "@multica/core/triage";
import en from "../locales/en/triage.json";
import { TriageActionDialog } from "./triage-action-dialog";

const { mutate, close } = vi.hoisted(() => ({ mutate: vi.fn(), close: vi.fn() }));
vi.mock("@multica/core/api", async (original) => ({
  ...(await original<typeof import("@multica/core/api")>()),
  api: { listIssues: vi.fn(), getIssue: vi.fn() },
}));
vi.mock("@multica/core/paths", () => ({ useCurrentWorkspace: () => ({ slug: "acme" }) }));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  useTriageAction: () => ({ mutateAsync: mutate, isPending: false }),
  triageDetailOptions: () => ({ queryKey: ["test-detail"], queryFn: async () => null, enabled: false }),
}));
vi.mock("../i18n", () => ({
  useT: () => ({ t: (selector: (value: typeof en) => string) => selector(en), i18n: { language: "en" } }),
}));
vi.mock("../issues/utils/status-label", () => ({ useStatusLabel: () => (status: string) => status }));
vi.mock("./triage-execution-preview", () => ({
  useTriageExecutionPrerequisites: () => ({ canExecute: false }), TriageExecutionSummary: () => null,
}));
vi.mock("./triage-fields", () => ({
  TriageField: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  ReviewerSelect: () => null, TriageFieldsEditor: () => null,
}));

function issue(number: number, overrides: Partial<Issue> = {}): Issue {
  return {
    id: `issue-${number}`, identifier: `ACM-${number}`, number, workspace_id: "ws-1",
    title: `Task ${number}`, description: null, status: "todo", priority: "none",
    admission_status: "not_required", assignee_type: null, assignee_id: null,
    creator_type: "member", creator_id: "u1", parent_issue_id: null, project_id: null,
    position: number, stage: null, start_date: null, due_date: null, revision: 1,
    metadata: {}, properties: {}, created_at: "2026-10-10", updated_at: "2026-10-10",
    ...overrides,
  };
}
const item: TriageItem = {
  issue: { ...issue(1), admission_status: "pending", revision: 1 },
  candidate_project_id: null, candidate_assignee_type: null, candidate_assignee_id: null,
  reviewer_id: null, reviewer_valid: false, round: 1, first_entered_at: "2026-10-10",
  entered_at: "2026-10-10", snoozed_until: null, duplicate_issue_id: null,
  duplicate_identifier: null, source: "manual", source_url: null, external_id: null,
  batch_id: null, filename: null, row_number: null,
};
const settings: TriageSettings = {
  supported: true, enabled: true, acceptance_status: "todo", require_priority: false,
  responsibility_mode: "none", responsibility_member_id: null, revision: 1,
};
let client: QueryClient;
beforeEach(() => {
  vi.resetAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  vi.mocked(api.listIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.getIssue).mockResolvedValue(issue(4));
  mutate.mockResolvedValue({ item, action: {} });
});
afterEach(() => client.clear());
function renderDialog() {
  return render(<QueryClientProvider client={client}>
    <TriageActionDialog wsId="ws-1" item={item} action="duplicate" settings={settings}
      onClose={close} onSuccess={() => {}} />
  </QueryClientProvider>);
}

describe("duplicate target picker", () => {
  it("opens on focus, excludes invalid candidates, and selects without submitting", async () => {
    const user = userEvent.setup();
    const rows = [issue(1), issue(2, { title: "Completed child", status: "done", parent_issue_id: "parent" }),
      issue(3, { status: "cancelled", admission_status: "accepted" }),
      issue(4, { admission_status: "pending" }), issue(5, { admission_status: "rejected" }),
      issue(6, { admission_status: "duplicate" }), issue(7, { workspace_id: "other" }),
      issue(8, { admission_status: "future" })];
    vi.mocked(api.listIssues).mockResolvedValue({ issues: rows, total: rows.length });
    renderDialog();
    await user.click(screen.getByRole("combobox", { name: en.duplicate_target }));
    const completed = await screen.findByRole("option", { name: /Completed child/ });
    expect(completed).toHaveTextContent("ACM-2");
    expect(completed).toHaveTextContent("done");
    expect(screen.getByRole("listbox", { name: en.duplicate_candidates })).toBeVisible();
    expect(screen.getAllByRole("option")).toHaveLength(2);
    await user.click(screen.getByRole("option", { name: /Completed child/ }));
    expect(screen.getByRole("combobox")).toHaveValue("ACM-2");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(screen.getByRole("combobox")).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("combobox")).not.toHaveAttribute("aria-controls");
    expect(mutate).not.toHaveBeenCalled();
    await user.type(screen.getByRole("textbox", { name: en.reason }), "Same work");
    await user.click(screen.getByRole("button", { name: en.duplicate }));
    expect(mutate).toHaveBeenCalledWith(expect.objectContaining({ input: expect.objectContaining({ duplicate_issue_id: "ACM-2", reason: "Same work" }) }));
  });

  it("keeps later pages reachable when the first page is excluded and retries page failures", async () => {
    vi.mocked(api.listIssues)
      .mockResolvedValueOnce({ issues: [issue(1), issue(2, { admission_status: "pending" })], total: 3 })
      .mockRejectedValueOnce(new Error("Offline"))
      .mockResolvedValueOnce({ issues: [issue(3)], total: 3 });
    renderDialog();
    fireEvent.focus(screen.getByRole("combobox"));
    fireEvent.click(await screen.findByRole("button", { name: en.duplicate_more }));
    expect(await screen.findByText(en.duplicate_more_failed)).toBeVisible();
    expect(screen.queryByText(en.duplicate_empty)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: en.retry }));
    expect(await screen.findByRole("option", { name: /ACM-3/ })).toBeVisible();
    expect(screen.queryByRole("button", { name: en.duplicate_more })).not.toBeInTheDocument();
    expect(vi.mocked(api.listIssues).mock.calls.map(([params]) => params?.offset)).toEqual([0, 2, 2]);
  });

  it("shows a recoverable initial error, then distinguishes an empty workspace from no matches", async () => {
    vi.mocked(api.listIssues).mockRejectedValueOnce(new Error("Offline"));
    renderDialog();
    fireEvent.focus(screen.getByRole("combobox"));
    expect(await screen.findByText(en.duplicate_failed)).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: en.retry }));
    expect(await screen.findByText(en.duplicate_empty)).toBeVisible();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "Absent" } });
    expect(screen.getByText(en.loading)).toBeVisible();
    expect(await screen.findByText(en.duplicate_no_results)).toBeVisible();
  });

  it("lets keyboard users retry pagination without selecting an existing task", async () => {
    const user = userEvent.setup();
    vi.mocked(api.listIssues)
      .mockResolvedValueOnce({ issues: [issue(2)], total: 2 })
      .mockRejectedValueOnce(new Error("Offline"))
      .mockResolvedValueOnce({ issues: [issue(3)], total: 2 });
    renderDialog();
    await user.click(screen.getByRole("combobox"));
    const more = await screen.findByRole("button", { name: en.duplicate_more });
    act(() => more.focus());
    await user.keyboard("{Enter}");
    const retry = await screen.findByRole("button", { name: en.retry });
    act(() => retry.focus());
    await user.keyboard("{Enter}");
    expect(await screen.findByRole("option", { name: /ACM-3/ })).toBeVisible();
    expect(screen.getByRole("combobox")).toHaveValue("");
    expect(mutate).not.toHaveBeenCalled();
  });

  it("closes only candidates and restores input focus when Escape is pressed on load more", async () => {
    const user = userEvent.setup();
    vi.mocked(api.listIssues).mockResolvedValue({ issues: [issue(2)], total: 2 });
    renderDialog();
    const input = screen.getByRole("combobox");
    await user.click(input);
    const more = await screen.findByRole("button", { name: en.duplicate_more });
    act(() => more.focus());
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(input).toHaveFocus();
    expect(input).toHaveAttribute("aria-expanded", "false");
    expect(close).not.toHaveBeenCalled();
  });

  it.each(["Fix UI/UX", "Handle /api/issues failure"])("searches the slash-containing title %s", async (title) => {
    vi.mocked(api.listIssues).mockResolvedValue({ issues: [issue(2, { title })], total: 1 });
    renderDialog();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: title } });
    expect(await screen.findByRole("option", { name: new RegExp(title) })).toBeVisible();
    expect(api.listIssues).toHaveBeenCalledWith(expect.objectContaining({ q: title }));
    expect(screen.queryByText(en.invalid_duplicate_link)).not.toBeInTheDocument();
  });

  it("hides previous search results immediately and ignores late responses after a query change", async () => {
    let resolveOld!: (value: ListIssuesResponse) => void;
    const oldRequest = new Promise<ListIssuesResponse>((resolve) => { resolveOld = resolve; });
    vi.mocked(api.listIssues).mockImplementation(async (params) => {
      if (params?.q === "old") return oldRequest;
      if (params?.q === "new") return { issues: [issue(3, { title: "New result" })], total: 1 };
      return { issues: [issue(2, { title: "Browse result" })], total: 1 };
    });
    renderDialog();
    fireEvent.focus(screen.getByRole("combobox"));
    await screen.findByRole("option", { name: /Browse result/ });
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "old" } });
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    await waitFor(() => expect(api.listIssues).toHaveBeenCalledWith(expect.objectContaining({ q: "old" })));
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "new" } });
    expect(await screen.findByRole("option", { name: /New result/ })).toBeVisible();
    await act(async () => resolveOld({ issues: [issue(4, { title: "Old result" })], total: 1 }));
    expect(screen.queryByRole("option", { name: /Old result/ })).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "" } });
    expect(await screen.findByRole("option", { name: /Browse result/ })).toBeVisible();
    expect(screen.queryByRole("option", { name: /New result/ })).not.toBeInTheDocument();
  });

  it("supports arrow/Enter selection, Escape without closing the dialog, and Tab to the reason", async () => {
    const user = userEvent.setup();
    vi.mocked(api.listIssues).mockResolvedValue({ issues: [issue(2), issue(3)], total: 2 });
    renderDialog();
    const input = screen.getByRole("combobox");
    await user.click(input);
    await screen.findByRole("option", { name: /ACM-3/ });
    await user.keyboard("{ArrowDown}{Enter}");
    expect(input).toHaveValue("ACM-3");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    await user.click(input);
    await screen.findByRole("listbox");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(close).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeVisible();
    await user.tab();
    expect(screen.getByRole("textbox", { name: en.reason })).toHaveFocus();
  });

  // The parsing matrix lives in triage-ui.test.ts; these assert submission wiring.
  it("rejects title text as an identity but keeps manual identifier submission", async () => {
    renderDialog();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "Task title" } });
    fireEvent.click(screen.getByRole("button", { name: en.duplicate }));
    expect(screen.getByText(en.invalid_duplicate_link)).toBeVisible();
    expect(mutate).not.toHaveBeenCalled();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "ACM-4" } });
    fireEvent.click(screen.getByRole("button", { name: en.duplicate }));
    expect(mutate).toHaveBeenCalledWith(expect.objectContaining({ input: expect.objectContaining({ duplicate_issue_id: "ACM-4" }) }));
  });

  it("searches a pasted scoped link and submits its reference without requiring selection", async () => {
    vi.mocked(api.getIssue).mockResolvedValue(issue(4));
    renderDialog();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "https://example.test/acme/issues/ACM-4" } });
    expect(await screen.findByRole("option", { name: /ACM-4/ })).toBeVisible();
    expect(api.getIssue).toHaveBeenCalledWith("ACM-4", expect.objectContaining({ workspaceId: "ws-1" }));
    fireEvent.click(screen.getByRole("button", { name: en.duplicate }));
    expect(mutate).toHaveBeenCalledWith(expect.objectContaining({ input: expect.objectContaining({ duplicate_issue_id: "ACM-4" }) }));
  });

  it("applies the same candidate scope to exact lookups and accepts legacy formal tasks", async () => {
    vi.mocked(api.getIssue)
      .mockResolvedValueOnce(issue(4, { admission_status: "pending" }))
      .mockResolvedValueOnce(issue(5, { workspace_id: "other" }))
      .mockResolvedValueOnce(issue(6, { admission_status: undefined }));
    renderDialog();
    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "/acme/issues/ACM-4" } });
    expect(await screen.findByText(en.duplicate_no_results)).toBeVisible();
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    fireEvent.change(input, { target: { value: "/acme/issues/ACM-5" } });
    await waitFor(() => expect(api.getIssue).toHaveBeenCalledWith("ACM-5", expect.anything()));
    expect(await screen.findByText(en.duplicate_no_results)).toBeVisible();
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    fireEvent.change(input, { target: { value: "/acme/issues/ACM-6" } });
    expect(await screen.findByRole("option", { name: /ACM-6/ })).toBeVisible();
  });
});
