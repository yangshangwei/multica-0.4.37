import { afterEach, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationComment, parseCollaborationIssue, prepareCollaborationCommentChange, saveCollaborationDraft } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationCommentList } from "./comment-list";

const actor = "00000000-0000-4000-8000-000000000001";
const commentId = "00000000-0000-4000-8000-000000000002";
const eventId = "00000000-0000-4000-8000-000000000003";
const timestamp = "2026-10-19T00:00:00Z";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
vi.mock("../navigation", () => ({ AppLink: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a> }));
const client = new QueryClient();
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
const comment = parseCollaborationComment({ id: commentId, issue_id: actor, author_type: "member", author: issue.creator, content: "Original comment", type: "comment", parent_id: null,
  comment_revision: "9007199254740993", attachment_ids: [], created_at: timestamp, updated_at: timestamp, deleted_at: null, resolved_at: null, resolved_by: null, can_edit: true, can_resolve: true, reactions: [{ emoji: "👍", count: 2, viewer_reacted: true }] })!;
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });

// Permission and original-key/version matrices live in core/comment-actions.test.ts.
it("moves focus into the delete confirmation and back to Edit when it is dismissed", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ issue_id: actor, attachments: [] }))));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue} comments={[comment]} onReply={vi.fn()} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Delete comment" }));
  await waitFor(() => expect(screen.getByRole("form", { name: "Delete comment" })).toHaveFocus());
  await user.click(screen.getByRole("button", { name: "Close" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Edit comment" })).toHaveFocus());
});

it("keeps the author's edit after a conflict and requires an explicit new baseline", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.includes("attachments")) return new Response(JSON.stringify({ issue_id: actor, attachments: [] }));
    return new Response(JSON.stringify({ status: 409, code: "stale_comment_revision", request_id: (init.headers as Record<string, string>)["Idempotency-Key"], action: "edit_comment",
      protocol_version: 2, issue_id: actor, comment_id: commentId, comment_revision: "9007199254740994", event_id: null, targets: [] }), { status: 409 });
  });
  vi.stubGlobal("fetch", fetch);
  const view = (current = comment) => <QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[current]} onReply={vi.fn()} /></QueryClientProvider>;
  const mounted = renderWithI18n(view());
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Edit comment" }));
  await user.clear(screen.getByLabelText("Edit comment text"));
  await user.type(screen.getByLabelText("Edit comment text"), "My preserved draft");
  await user.click(screen.getByRole("button", { name: "Save comment" }));
  expect(await screen.findByText("This comment changed. Your draft is preserved. Review the latest comment before saving again.")).toBeInTheDocument();
  mounted.rerender(view({ ...comment, content: "Changed elsewhere", comment_revision: "9007199254740994" }));
  expect(screen.getByLabelText("Edit comment text")).toHaveValue("My preserved draft");
  expect(screen.getByRole("button", { name: "Save comment" })).toBeDisabled();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "PATCH")).toHaveLength(1);
  await user.click(screen.getByRole("button", { name: "Use latest version" }));
  expect(screen.getByRole("button", { name: "Save comment" })).toBeEnabled();
  expect(screen.getByLabelText("Edit comment text")).toHaveValue("My preserved draft");
});

it("keeps tombstones and nested reply links while exposing only the current author's controls", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const reply = { ...comment, id: eventId, parent_id: commentId, content: "A surviving reply", author: { ...issue.creator, user_id: eventId, name: "Other author" }, can_edit: false };
  const onReply = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[{ ...comment, deleted_at: timestamp, content: "", can_edit: false, can_resolve: false }, reply]} onReply={onReply} /></QueryClientProvider>);
  expect(document.getElementById(`comment-${commentId}`)).toHaveTextContent("Comment deleted");
  expect(screen.getByRole("link", { name: "View parent comment" })).toHaveAttribute("href", expect.stringContaining(`#comment-${commentId}`));
  expect(screen.getByText("A surviving reply")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Edit comment" })).toBeNull();
  await userEvent.click(within(document.getElementById(`comment-${eventId}`)!).getByRole("button", { name: "Reply" }));
  expect(onReply).toHaveBeenCalledWith(reply);
});

it("resolves a thread and removes only the viewer's reaction through dedicated account routes", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => new Response(JSON.stringify({ protocol_version: 2, issue_id: actor, comment_id: commentId,
    request_id: (init.headers as Record<string, string>)["Idempotency-Key"], status: 200,
    action: url.endsWith("/resolve") ? "resolve" : "remove_reaction", emoji: url.endsWith("/resolve") ? null : "👍", code: url.endsWith("/resolve") ? "comment_resolved" : "reaction_removed" })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[comment]} onReply={vi.fn()} /></QueryClientProvider>);
  await userEvent.click(screen.getByRole("button", { name: "Resolve thread" }));
  expect(await screen.findByText("Discussion updated.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Remove your 👍 reaction" })).toHaveAccessibleDescription("2");
  await userEvent.click(screen.getByRole("button", { name: "Remove your 👍 reaction" }));
  expect(fetch.mock.calls.map(([url]) => url)).toEqual([expect.stringContaining(`/comments/${commentId}/resolve`), expect.stringContaining(`/comments/${commentId}/reactions/${encodeURIComponent("👍")}`)]);
  expect(fetch.mock.calls.map(([, init]) => init.method)).toEqual(["POST", "DELETE"]);
});

it("keeps a recovered conflict editor visible when its comment is outside the loaded page", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  saveCollaborationDraft(prepareCollaborationCommentChange(scope, issue, comment, eventId, { action: "edit", content: "Keep this older draft", attachmentIds: [] }));
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => {
    if (url.includes("attachments")) return new Response(JSON.stringify({ issue_id: actor, attachments: [] }));
    if (url.endsWith(`/comments/${commentId}`)) return new Response(JSON.stringify({ ...comment, content: "Newer content", comment_revision: "9007199254740994" }));
    return new Response(JSON.stringify({ status: 409, code: "stale_comment_revision", request_id: eventId, action: "edit_comment", protocol_version: 2,
      issue_id: actor, comment_id: commentId, comment_revision: "9007199254740994", event_id: null, targets: [] }));
  }));
  const mounted = renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[]} onReply={vi.fn()} /></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: "Check original result" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Use latest version" })).toBeEnabled());
  expect(screen.getByLabelText("Edit comment text")).toHaveValue("Keep this older draft");
  expect(document.getElementById(`comment-${commentId}`)).toBeInTheDocument();
  mounted.rerender(<QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[{ ...comment, content: "Newer content", comment_revision: "9007199254740994" }]} onReply={vi.fn()} /></QueryClientProvider>);
  expect(screen.getByLabelText("Edit comment text")).toHaveValue("Keep this older draft");
  expect(screen.getByRole("button", { name: "Use latest version" })).toBeEnabled();
});

it("shows byte-limit feedback without losing pasted text or submitting an oversized edit", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const fetch = vi.fn().mockImplementation(async (_url: string, _init: RequestInit) => new Response(JSON.stringify({ issue_id: actor, attachments: [] })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[comment]} onReply={vi.fn()} /></QueryClientProvider>);
  await userEvent.click(screen.getByRole("button", { name: "Edit comment" }));
  fireEvent.change(screen.getByLabelText("Edit comment text"), { target: { value: "界".repeat(30000) } });
  expect(screen.getByLabelText("Edit comment text")).toHaveValue("界".repeat(30000));
  expect(screen.getByLabelText("Edit comment text")).toHaveAttribute("aria-invalid", "true");
  expect(screen.getByText("This comment is too long. Shorten it before saving; your draft is preserved.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Save comment" })).toBeDisabled();
  expect(fetch.mock.calls.every(([, init]) => init.method !== "PATCH")).toBe(true);
  fireEvent.change(screen.getByLabelText("Edit comment text"), { target: { value: "Shortened draft" } });
  expect(screen.getByRole("button", { name: "Save comment" })).toBeEnabled();
});

it("confirms an older oversized refusal before unlocking its preserved input", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const initial = prepareCollaborationCommentChange(scope, issue, comment, eventId, { action: "edit", content: "Initial", attachmentIds: [] });
  saveCollaborationDraft({ ...initial, payload: { domain: "comment_change", commentId, action: "edit", input: { content: "界".repeat(30000), expected_comment_revision: comment.comment_revision, attachment_ids: [] } } });
  const refusal = { status: 400, code: "invalid_comment_request", request_id: eventId, action: "edit_comment", protocol_version: 2,
    issue_id: actor, comment_id: commentId, comment_revision: comment.comment_revision, event_id: null, targets: [] };
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.includes("attachments")) return new Response(JSON.stringify({ issue_id: actor, attachments: [] }));
    return new Response(JSON.stringify(refusal), { status: init.method === "PATCH" ? 400 : 200 });
  });
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[comment]} onReply={vi.fn()} /></QueryClientProvider>);
  expect(screen.getByLabelText("Edit comment text")).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  await waitFor(() => expect(screen.getByLabelText("Edit comment text")).toBeEnabled());
  expect(screen.getByLabelText("Edit comment text")).toHaveValue("界".repeat(30000));
  expect(fetch.mock.calls.filter(([url]) => !url.includes("attachments")).map(([, init]) => init.method ?? "GET")).toEqual(["PATCH", "GET"]);
  fireEvent.change(screen.getByLabelText("Edit comment text"), { target: { value: "Shortened draft" } });
  expect(screen.getByRole("button", { name: "Save comment" })).toBeEnabled();
});
