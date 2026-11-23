// Parsing and original-key boundary matrices live in core/collaboration/memory-*.test.ts.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "@multica/core/api";
import { collaborationMemoryKeys, collaborationScope, parseCollaborationIssue, parseCollaborationMemory, parseCollaborationMemoryVersions,
  readCollaborationDrafts, type CollaborationMemoryReceipt } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationMemoryPanel } from "./memory-panel";

const userId = "00000000-0000-4000-8000-000000000001";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
vi.mock("../navigation", () => ({ AppLink: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} /> }));
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
const author = { kind: "member", member: issue.creator, agent: null, resource_owner: null };
const memory = (version = "1", content = "Initial decision", canEdit = true) => parseCollaborationMemory({
  issue_id: issue.id, version, content, bytes: new TextEncoder().encode(content).length, max_bytes: 131072,
  author, created_at: "2026-10-26T00:00:00Z", restored_from_version: null, can_edit: canEdit, can_purge: true,
})!;
const history = () => parseCollaborationMemoryVersions({ issue_id: issue.id, current_version: "2", max_bytes: 131072, can_purge: true,
  items: ["2", "1"].map(version => ({ version, bytes: 16, author, created_at: "2026-10-26T00:00:00Z", restored_from_version: null,
    purged: false, purged_at: null, purged_by: null, purged_by_admin: false })), next_cursor: null })!;
let api: ApiClient;
let client: QueryClient;
let scope: ReturnType<typeof collaborationScope>;
beforeEach(() => {
  localStorage.clear();
  api = new ApiClient("https://memory.example"); setApiInstance(api); scope = collaborationScope(userId);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  vi.spyOn(api, "getCollaborationMemory").mockResolvedValue(memory());
  vi.spyOn(api, "getCollaborationMemoryVersions").mockResolvedValue(history());
  vi.spyOn(api, "getCollaborationMemoryOperation").mockResolvedValue(null);
});
afterEach(() => { client.clear(); localStorage.clear(); vi.restoreAllMocks(); });
function mount(currentIssue = issue) {
  return renderWithI18n(<QueryClientProvider client={client}><CollaborationMemoryPanel scope={scope} issue={currentIssue} /></QueryClientProvider>);
}
function receipt(requestId: string, version: string | null, code = "memory_updated", status = 200): CollaborationMemoryReceipt {
  return { request_id: requestId, action: "memory_update", protocol_version: 1, issue_id: issue.id, version,
    current_version: version ?? "2", code, status, max_bytes: 131072, ...(status === 200 ? { bytes: 8, event_seq: "3" } : {}) };
}

it("does not refocus a shared-context anchor on resume but focuses a fresh open", async () => {
  const view = (resume: boolean, navigationKey: number) => <QueryClientProvider client={client}><CollaborationMemoryPanel scope={scope} issue={issue}
    destination={{ view: "current", issueId: issue.id, commentId: null, anchor: "memory" }} isScopeResume={resume} navigationKey={navigationKey} /></QueryClientProvider>;
  const mounted = renderWithI18n(view(true, 1));
  await screen.findByText("Initial decision");
  expect(document.getElementById("memory")).not.toHaveFocus();
  mounted.rerender(view(false, 2));
  expect(document.getElementById("memory")).toHaveFocus();
});

it("leaves a dismissed version dialog closed on resume and reopens it for an explicit link", async () => {
  const version = vi.spyOn(api, "getCollaborationMemoryVersion").mockRejectedValue(new Error("Version unavailable in fixture"));
  const view = (resume: boolean, navigationKey: number) => <QueryClientProvider client={client}><CollaborationMemoryPanel scope={scope} issue={issue}
    destination={{ view: "current", issueId: issue.id, commentId: null, memoryVersion: "1" }} isScopeResume={resume} navigationKey={navigationKey} /></QueryClientProvider>;
  const mounted = renderWithI18n(view(true, 1));
  await screen.findByText("Initial decision");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(version).not.toHaveBeenCalled();
  mounted.rerender(view(false, 2));
  await screen.findByRole("dialog");
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  mounted.rerender(view(false, 3));
  expect(await screen.findByRole("dialog")).toBeVisible();
});

it("keeps the editing base and text after a stale save until the user explicitly rebases", async () => {
  const update = vi.spyOn(api, "updateCollaborationMemory").mockImplementationOnce(async (_id, key) => receipt(key, null, "stale_memory_version", 409))
    .mockImplementationOnce(async (_id, key) => receipt(key, "3"));
  mount();
  await userEvent.click(await screen.findByRole("button", { name: "Edit shared context" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Shared context (Markdown)" }), { target: { value: "My decision to merge" } });
  vi.mocked(api.getCollaborationMemory).mockResolvedValue(memory("2", "Another writer's decision"));
  await userEvent.click(screen.getByRole("button", { name: "Save shared context" }));
  await screen.findByText(/Someone updated the shared context after you started/);
  expect(update).toHaveBeenCalledWith(issue.id, expect.any(String), { expected_version: "1", content: "My decision to merge" });
  expect(screen.getByRole("textbox", { name: "Shared context (Markdown)" })).toHaveValue("My decision to merge");
  expect(screen.getByRole("button", { name: "Save shared context" })).toBeDisabled();
  await userEvent.click(await screen.findByRole("button", { name: "Use current version" }));
  expect(screen.getByRole("textbox", { name: "Shared context (Markdown)" })).toHaveValue("My decision to merge");
  await userEvent.click(screen.getByRole("button", { name: "Save shared context" }));
  await waitFor(() => expect(update).toHaveBeenCalledTimes(2));
  expect(update.mock.calls[1]?.[2]).toEqual({ expected_version: "2", content: "My decision to merge" });
  expect(update.mock.calls[1]?.[1]).not.toBe(update.mock.calls[0]?.[1]);
});

it("requires explicit review when realtime refreshes the current version during editing", async () => {
  const update = vi.spyOn(api, "updateCollaborationMemory").mockImplementation(async (_id, key) => receipt(key, "3"));
  mount();
  await userEvent.click(await screen.findByRole("button", { name: "Edit shared context" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Shared context (Markdown)" }), { target: { value: "Unmerged local text" } });
  act(() => client.setQueryData(collaborationMemoryKeys.root(scope, issue.id), memory("2", "New remote text")));
  expect(await screen.findByText(/Someone updated the shared context after you started/)).toBeVisible();
  expect(screen.getByRole("textbox", { name: "Shared context (Markdown)" })).toHaveValue("Unmerged local text");
  expect(screen.getByText("New remote text")).toBeVisible();
  expect(screen.getByRole("button", { name: "Save shared context" })).toBeDisabled();
  expect(update).not.toHaveBeenCalled();
});

it("restores an uncertain update after remount and retries its original key and payload", async () => {
  const update = vi.spyOn(api, "updateCollaborationMemory").mockRejectedValueOnce(new TypeError("Response lost"))
    .mockImplementationOnce(async (_id, key) => receipt(key, "2"));
  const view = mount();
  await userEvent.click(await screen.findByRole("button", { name: "Edit shared context" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Shared context (Markdown)" }), { target: { value: "Retained request" } });
  await userEvent.click(screen.getByRole("button", { name: "Save shared context" }));
  await screen.findByRole("button", { name: "Retry original request" });
  const original = readCollaborationDrafts({ ...scope, issueId: issue.id })[0]!;
  view.unmount(); mount();
  await screen.findByRole("button", { name: "Check original result" });
  expect(update).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("button", { name: "Edit shared context" })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  await waitFor(() => expect(update).toHaveBeenCalledTimes(2));
  expect(update.mock.calls[1]).toEqual(update.mock.calls[0]);
  expect(update.mock.calls[1]?.[1]).toBe(original.requestId);
  await waitFor(() => expect(readCollaborationDrafts({ ...scope, issueId: issue.id })).toEqual([]));
});

it("confirms restore and keeps closed content read-only while permitting historical disposal", async () => {
  vi.mocked(api.getCollaborationMemory).mockResolvedValue(memory("2"));
  const restore = vi.spyOn(api, "restoreCollaborationMemoryVersion").mockImplementation(async (_id, key) => ({ ...receipt(key, "3"), action: "memory_restore", code: "memory_restored" }));
  const view = mount();
  await userEvent.click(await screen.findByRole("button", { name: "Version history" }));
  await userEvent.click(await screen.findByRole("button", { name: "Restore v1" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Restore v1?" });
  expect(restore).not.toHaveBeenCalled();
  await userEvent.click(within(dialog).getByRole("button", { name: "Restore as new version" }));
  await waitFor(() => expect(restore).toHaveBeenCalledTimes(1));
  vi.mocked(api.getCollaborationMemory).mockResolvedValue(memory("2", "Closed decision", false));
  await act(async () => { await client.invalidateQueries({ queryKey: collaborationMemoryKeys.root(scope, issue.id) }); });
  view.rerender(<QueryClientProvider client={client}><CollaborationMemoryPanel scope={scope} issue={{ ...issue, status: "done", status_category: "done" }} /></QueryClientProvider>);
  expect(await screen.findByText("This issue is closed, so its shared context is read-only.")).toBeVisible();
  expect(screen.queryByRole("button", { name: "Edit shared context" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Restore v1" })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Purge v1" }));
  const purge = await screen.findByRole("alertdialog", { name: "Purge the text of v1?" });
  expect(within(purge).getByRole("button", { name: "Purge text" })).toBeDisabled();
});

it("removes private editor content after access is revoked", async () => {
  mount();
  await userEvent.click(await screen.findByRole("button", { name: "Edit shared context" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Shared context (Markdown)" }), { target: { value: "PRIVATE_EDITOR" } });
  vi.mocked(api.getCollaborationMemory).mockRejectedValue(new ApiError("Access lost", 403, "Forbidden"));
  await act(async () => { await client.invalidateQueries({ queryKey: collaborationMemoryKeys.root(scope, issue.id) }); });
  await waitFor(() => expect(screen.queryByRole("textbox")).not.toBeInTheDocument());
  expect(screen.queryByText("PRIVATE_EDITOR")).not.toBeInTheDocument();
});
