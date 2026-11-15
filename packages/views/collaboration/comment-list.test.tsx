import { afterEach, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationComment, parseCollaborationIssue, prepareCollaborationCommentChange, prepareCollaborationDiscussion, saveCollaborationDraft } from "@multica/core/collaboration";
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
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); delete (Element.prototype as { getAnimations?: unknown }).getAnimations; });
const row = (id: string) => document.getElementById(`comment-${id}`)!;
// Low-frequency comment actions live in each row's portaled Comment actions menu.
async function openMenu(id = commentId, name = "Comment actions") {
  await userEvent.click(within(row(id)).getByRole("button", { name }));
  return screen.findByRole("menu");
}
async function chooseMenuItem(name: string, id = commentId) {
  await openMenu(id);
  await userEvent.click(await screen.findByRole("menuitem", { name }));
}

// Core content-schema.test.ts owns malformed-summary and authority matrices.
it.each(["en", "zh-Hans"] as const)("distinguishes a named Agent from a same-named person in %s without fetching Agent details", async locale => {
  setApiInstance(new ApiClient("https://comments.example"));
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  const name = "Payment reviewer / 支付回归审核";
  const person = { ...comment, author: { ...issue.creator, name, external: true } };
  const agentComment = parseCollaborationComment({ ...comment, id: eventId, author_type: "agent", author: null,
    author_agent: { agent_id: eventId, name }, can_edit: false })!;
  const onReply = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue}
    comments={[person, agentComment]} onReply={onReply} /></QueryClientProvider>, { locale });
  const personRow = within(document.getElementById(`comment-${commentId}`)!);
  const agentRow = within(document.getElementById(`comment-${eventId}`)!);
  const agentLabel = locale === "en" ? "Agent" : "智能体";
  expect(personRow.getByText(name)).toBeInTheDocument();
  expect(personRow.queryByText(agentLabel, { exact: true })).not.toBeInTheDocument();
  expect(personRow.getByText(locale === "en" ? "External collaborator" : "外部协作者")).toBeInTheDocument();
  expect(agentRow.getByText(name)).toBeInTheDocument();
  expect(agentRow.getByText(agentLabel, { exact: true })).toBeInTheDocument();
  await openMenu(eventId, locale === "en" ? "Comment actions" : "评论操作");
  expect(screen.getByRole("menuitem", { name: locale === "en" ? "Comment history" : "评论历史" })).toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: locale === "en" ? "Edit comment" : "编辑评论" })).not.toBeInTheDocument();
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
  await userEvent.click(agentRow.getByRole("button", { name: locale === "en" ? "Reply" : "回复" }));
  expect(onReply).toHaveBeenCalledWith(agentComment);
  expect(fetch).not.toHaveBeenCalled();
});

it.each((["en", "zh-Hans"] as const).flatMap(locale => [undefined, "", "   "].map(name => ({ locale, name }))))("shows one generic Agent label in $locale when the Agent name is $name", ({ locale, name }) => {
  setApiInstance(new ApiClient("https://comments.example"));
  const legacy = parseCollaborationComment({ ...comment, author_type: "agent", author: null, can_edit: false,
    ...(name === undefined ? {} : { author_agent: { agent_id: eventId, name } }) })!;
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue}
    comments={[legacy]} onReply={vi.fn()} /></QueryClientProvider>, { locale });
  expect(screen.getAllByText(locale === "en" ? "Agent" : "智能体", { exact: true })).toHaveLength(1);
});

it.each(["en", "zh-Hans"] as const)("uses the %s app locale for comment and history timestamps while preserving ISO values", async locale => {
  setApiInstance(new ApiClient("https://comments.example"));
  const editedAt = "2026-10-20T13:24:00Z";
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ issue_id: actor, comment_id: commentId, items: [{
    event_id: eventId, event_seq: "1", kind: "comment_edited", actor: issue.creator, created_at: editedAt,
    comment_revision: comment.comment_revision, before: "Earlier comment", after: comment.content,
  }] }))));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue}
    comments={[comment]} onReply={vi.fn()} /></QueryClientProvider>, { locale });
  // Older than a week: a localized date, with the full time on hover.
  const time = row(commentId).querySelector("time")!;
  expect(time).toHaveAttribute("datetime", timestamp);
  expect(time).toHaveAttribute("title", new Date(timestamp).toLocaleString(locale));
  expect(time).toHaveTextContent(new Date(timestamp).toLocaleDateString(locale));
  await openMenu(commentId, locale === "en" ? "Comment actions" : "评论操作");
  await userEvent.click(await screen.findByRole("menuitem", { name: locale === "en" ? "Comment history" : "评论历史" }));
  expect(await screen.findByText(new Date(editedAt).toLocaleString(locale))).toHaveAttribute("datetime", editedAt);
});

it("loads a placeholder through the human detail route before exposing editing", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const fetch = vi.fn(async (url: string) => new Response(JSON.stringify(url.includes("attachments") ? { issue_id: actor, attachments: [] } : comment)));
  vi.stubGlobal("fetch", fetch);
  const placeholder = { ...comment, content: "", content_state: "omitted" as const, content_bytes: 10000, can_edit: false };
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue} comments={[placeholder]} onReply={vi.fn()} /></QueryClientProvider>);
  await openMenu();
  expect(screen.queryByRole("menuitem", { name: "Edit comment" })).toBeNull();
  await userEvent.keyboard("{Escape}");
  expect(fetch).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Load full text" }));
  expect(await screen.findByText("Original comment")).toBeInTheDocument();
  await openMenu();
  expect(await screen.findByRole("menuitem", { name: "Edit comment" })).toBeInTheDocument();
  expect(fetch.mock.calls[0]?.[0]).toBe(`https://comments.example/api/collaboration/issues/${actor}/comments/${commentId}`);
  expect(fetch.mock.calls.every(([url]) => !url.includes("/runtime/"))).toBe(true);
});

it("does not restore a different revision into a placeholder editor", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ ...comment, comment_revision: "9007199254740994", content: "Changed elsewhere" }))));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue}
    comments={[{ ...comment, content: "", content_state: "omitted", content_bytes: 10000, can_edit: false }]} onReply={vi.fn()} /></QueryClientProvider>);
  await userEvent.click(screen.getByRole("button", { name: "Load full text" }));
  expect(await screen.findByText("This comment changed. Refresh the list before loading its latest text.")).toBeInTheDocument();
  expect(screen.queryByText("Changed elsewhere")).toBeNull();
  await openMenu();
  expect(screen.queryByRole("menuitem", { name: "Edit comment" })).toBeNull();
});

// Permission and original-key/version matrices live in core/comment-actions.test.ts.
it("moves focus into the delete confirmation and back to the Comment actions trigger when it is dismissed", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ issue_id: actor, attachments: [] }))));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue} comments={[comment]} onReply={vi.fn()} /></QueryClientProvider>);
  const user = userEvent.setup();
  // Edit and Delete are reachable only through the menu.
  expect(screen.queryByRole("button", { name: "Delete comment" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Edit comment" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "Comment actions" }));
  await user.click(await screen.findByRole("menuitem", { name: "Delete comment" }));
  await waitFor(() => expect(screen.getByRole("form", { name: "Delete comment" })).toHaveFocus());
  await user.click(screen.getByRole("button", { name: "Close" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Comment actions" })).toHaveFocus());
});

// jsdom removes a closing menu at once and reports no focus loss when a
// focused node is removed. Chromium does both differently: the popup stays for
// its exit animation, and removing the chosen item makes the menu take focus
// back. These cases reproduce that order; the plain jsdom order is above.
function closeMenusLikeChromium() {
  Object.defineProperty(Element.prototype, "getAnimations", { configurable: true, value(this: Element) {
    return this.matches('[data-slot="dropdown-menu-content"][data-closed]') ? [{ finished: new Promise<void>((resolve) => setTimeout(resolve, 100)) }] : [];
  } });
  const removeChild = Node.prototype.removeChild;
  vi.spyOn(Node.prototype, "removeChild").mockImplementation(function (this: Node, child: Node) {
    const active = document.activeElement;
    if (active && active !== document.body && child.contains(active)) active.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
    return removeChild.call(this, child);
  } as typeof removeChild);
}

it("focuses the delete confirmation after a menu that keeps focus while it closes", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ issue_id: actor, attachments: [] }))));
  closeMenusLikeChromium();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue} comments={[comment]} onReply={vi.fn()} /></QueryClientProvider>);
  await chooseMenuItem("Delete comment");
  // The closing menu, not the page, holds focus when the form first asks for it.
  expect(screen.getByRole("menu")).toContainElement(document.activeElement as HTMLElement);
  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  await waitFor(() => expect(screen.getByRole("form", { name: "Delete comment" })).toHaveFocus());
});

it("prepares a result only after a slowly closing menu has returned focus to its trigger", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  closeMenusLikeChromium();
  const focusedAtPreparation: (Element | null)[] = [];
  const onPrepareResult = vi.fn(() => { focusedAtPreparation.push(document.activeElement); });
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue} comments={[comment]} onReply={vi.fn()} onPrepareResult={onPrepareResult} /></QueryClientProvider>);
  await chooseMenuItem("Use in result");
  // Still closing: the chosen item is focused and would vanish with the menu.
  expect(onPrepareResult).not.toHaveBeenCalled();
  await waitFor(() => expect(onPrepareResult).toHaveBeenCalledWith(comment));
  expect(focusedAtPreparation).toEqual([within(row(commentId)).getByRole("button", { name: "Comment actions" })]);
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
  await chooseMenuItem("Edit comment");
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

it("keeps a tombstone on one line and threads its surviving reply beneath it", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const reply = { ...comment, id: eventId, parent_id: commentId, content: "A surviving reply", author: { ...issue.creator, user_id: eventId, name: "Other author" }, can_edit: false };
  const tombstone = { ...comment, deleted_at: timestamp, content: "", can_edit: false, can_resolve: false };
  const onReply = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={scope} issue={issue} comments={[tombstone, reply]} onReply={onReply} /></QueryClientProvider>);
  expect(within(row(commentId)).getByText("Comment deleted")).toBeInTheDocument();
  expect(row(commentId)).toHaveAttribute("data-depth", "0");
  expect(row(eventId)).toHaveAttribute("data-depth", "1");
  // The loaded parent sits directly above, so the reply needs no link to it.
  expect(screen.queryByRole("link", { name: "View parent comment" })).toBeNull();
  expect(screen.getByText("A surviving reply")).toBeInTheDocument();
  // A tombstone keeps existing reactions but no inline Reply; that moves into its menu.
  expect(within(row(commentId)).queryByRole("button", { name: "Reply" })).toBeNull();
  expect(within(row(commentId)).getByRole("button", { name: "Remove your 👍 reaction" })).toBeEnabled();
  expect(within(row(commentId)).queryByRole("button", { name: "Choose a reaction" })).toBeNull();
  await openMenu();
  expect(screen.getByRole("menuitem", { name: "Comment history" })).toBeInTheDocument();
  expect(screen.queryByRole("menuitem", { name: "Edit comment" })).toBeNull();
  await userEvent.click(screen.getByRole("menuitem", { name: "Reply" }));
  expect(onReply).toHaveBeenCalledWith(tombstone);
  await userEvent.click(within(row(eventId)).getByRole("button", { name: "Reply" }));
  expect(onReply).toHaveBeenLastCalledWith(reply);
  // The deleted root cannot be resolved, so the thread's one control sits on the reply.
  expect(screen.getAllByRole("button", { name: "Resolve thread" })).toHaveLength(1);
  expect(within(row(eventId)).getByRole("button", { name: "Resolve thread" })).toBeEnabled();
});

it("links a reply to its parent only while that parent is outside the loaded comments", () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const reply = { ...comment, parent_id: eventId };
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue} comments={[reply]} onReply={vi.fn()} /></QueryClientProvider>);
  expect(screen.getByRole("link", { name: "View parent comment" })).toHaveAttribute("href", expect.stringContaining(`#comment-${eventId}`));
  expect(row(commentId)).toHaveAttribute("data-depth", "1");
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
  await chooseMenuItem("Edit comment");
  // The closing menu must not take focus back from the form it opened.
  await waitFor(() => expect(screen.getByLabelText("Edit comment text")).toHaveFocus());
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

// Thread grouping, order and the unresolved definition are covered by
// core/collaboration/comment-threads.test.ts; these cases own the row wiring.
const replyId = "00000000-0000-4000-8000-000000000004";
const resolver = { ...issue.creator, name: "Resolver" };
const resolvedRoot = { ...comment, resolved_at: timestamp, resolved_by: resolver };
const threadReply = { ...comment, id: replyId, parent_id: commentId, content: "Reply in thread", created_at: "2026-10-19T00:05:00Z", updated_at: "2026-10-19T00:05:00Z", reactions: [] };
const list = (comments: (typeof comment)[], props: Partial<React.ComponentProps<typeof CollaborationCommentList>> = {}) =>
  <QueryClientProvider client={client}><CollaborationCommentList scope={collaborationScope(actor)} issue={issue} comments={comments} onReply={vi.fn()} {...props} /></QueryClientProvider>;
const foldButton = () => within(document.querySelector<HTMLElement>(`[data-thread-fold="${commentId}"]`)!).getByRole("button");

it.each([["en", "2h ago"], ["zh-Hans", "2 小时前"]] as const)("shows a recent comment's relative time in %s and keeps the full time on hover", (locale, text) => {
  setApiInstance(new ApiClient("https://comments.example"));
  const recent = new Date(Date.now() - 2 * 60 * 60 * 1000 - 5000).toISOString();
  renderWithI18n(list([{ ...comment, created_at: recent, updated_at: recent }]), { locale });
  const time = row(commentId).querySelector("time")!;
  expect(time).toHaveTextContent(text);
  expect(time).toHaveAttribute("datetime", recent);
  expect(time).toHaveAttribute("title", new Date(recent).toLocaleString(locale));
});

it.each(["en", "zh-Hans"] as const)("moves the author's permission context into the menu in %s", async locale => {
  setApiInstance(new ApiClient("https://comments.example"));
  const en = locale === "en";
  renderWithI18n(list([{ ...comment, author_authority: { role: "creator", can_submit_command: true, evaluated_at: timestamp } }]), { locale });
  expect(within(row(commentId)).queryByText(en ? "Current permissions" : "当前权限")).toBeNull();
  expect(row(commentId).querySelector("details")).toBeNull();
  const menu = await openMenu(commentId, en ? "Comment actions" : "评论操作");
  expect(within(menu).getByText(en ? "Current permissions" : "当前权限")).toBeInTheDocument();
  expect(within(menu).getByText(en ? "Creator" : "发起人")).toBeInTheDocument();
  expect(within(menu).getByText(en ? "Author can submit AI instructions now" : "作者当前可提交 AI 指令")).toBeInTheDocument();
});

it("keeps one action row and prepares a result only from the explicit menu item", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  // The result dialog returns focus to the element focused when it opens, so
  // the unmounting menu item must hand over the row's persistent trigger.
  const focusedAtPreparation: (Element | null)[] = [];
  const onPrepareResult = vi.fn(() => { focusedAtPreparation.push(document.activeElement); });
  renderWithI18n(list([comment], { onPrepareResult }));
  const actions = within(row(commentId)).getAllByRole("group", { name: "Thread and reactions" });
  expect(actions).toHaveLength(1);
  expect(within(actions[0]!).getAllByRole("button").map((button) => button.getAttribute("aria-label") ?? button.textContent)).toEqual(["Reply", "Remove your 👍 reaction", "Choose a reaction", "Resolve thread"]);
  expect(within(row(commentId)).queryByRole("button", { name: "Use in result" })).toBeNull();
  expect(within(row(commentId)).queryByRole("button", { name: "Comment history" })).toBeNull();
  await openMenu();
  expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual(["Edit comment", "Delete comment", "Comment history", "Use in result"]);
  expect(onPrepareResult).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("menuitem", { name: "Use in result" }));
  // Preparation waits for the closed menu to return focus to the trigger.
  await waitFor(() => expect(onPrepareResult).toHaveBeenCalledWith(comment));
  expect(focusedAtPreparation).toEqual([within(row(commentId)).getByRole("button", { name: "Comment actions" })]);
});

it("marks only an edited comment and opens its history from that marker", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const fetch = vi.fn(async (_url: string) => new Response(JSON.stringify({ issue_id: actor, comment_id: commentId, items: [{
    event_id: eventId, event_seq: "1", kind: "comment_edited", actor: issue.creator, created_at: "2026-10-20T13:24:00Z",
    comment_revision: comment.comment_revision, before: "Earlier comment", after: comment.content }] })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(list([{ ...comment, updated_at: "2026-10-20T13:24:00Z" }, threadReply]));
  expect(within(row(replyId)).queryByRole("button", { name: "Edited" })).toBeNull();
  expect(fetch).not.toHaveBeenCalled();
  await userEvent.click(within(row(commentId)).getByRole("button", { name: "Edited" }));
  const history = await within(row(commentId)).findByRole("region", { name: "Comment history" });
  expect(await within(history).findByText("Earlier comment")).toBeInTheDocument();
  expect(fetch.mock.calls.map(([url]) => url)).toEqual([expect.stringContaining(`/comments/${commentId}/history`)]);
  await userEvent.click(within(history).getByRole("button", { name: "Hide history" }));
  expect(within(row(commentId)).queryByRole("region", { name: "Comment history" })).toBeNull();
  await waitFor(() => expect(within(row(commentId)).getByRole("button", { name: "Comment actions" })).toHaveFocus());
});

it("shows one Resolve control per unresolved thread and keeps the others in their menus", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const fetch = vi.fn().mockImplementation(async (_url: string, init: RequestInit) => new Response(JSON.stringify({ protocol_version: 2, issue_id: actor, comment_id: replyId,
    request_id: (init.headers as Record<string, string>)["Idempotency-Key"], status: 200, action: "resolve", emoji: null, code: "comment_resolved" })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(list([comment, threadReply]));
  expect(screen.getAllByRole("button", { name: "Resolve thread" })).toHaveLength(1);
  expect(within(row(commentId)).getByRole("button", { name: "Resolve thread" })).toBeEnabled();
  expect(row(replyId)).toHaveAttribute("data-depth", "1");
  await chooseMenuItem("Resolve thread", replyId);
  expect(await within(row(replyId)).findByText("Discussion updated.")).toBeInTheDocument();
  expect(fetch.mock.calls.map(([url, init]) => [init.method, url])).toEqual([["POST", expect.stringContaining(`/comments/${replyId}/resolve`)]]);
});

it.each(["en", "zh-Hans"] as const)("folds an already resolved thread into one bar and reopens it after expanding in %s", async locale => {
  setApiInstance(new ApiClient("https://comments.example"));
  const en = locale === "en";
  const fetch = vi.fn().mockImplementation(async (_url: string, init: RequestInit) => new Response(JSON.stringify({ protocol_version: 2, issue_id: actor, comment_id: commentId,
    request_id: (init.headers as Record<string, string>)["Idempotency-Key"], status: 200, action: "unresolve", emoji: null, code: "comment_unresolved" })));
  vi.stubGlobal("fetch", fetch);
  const mounted = renderWithI18n(list([resolvedRoot, threadReply]), { locale });
  expect(row(commentId)).not.toBeVisible();
  expect(row(replyId)).not.toBeVisible();
  expect(foldButton()).toHaveAttribute("aria-expanded", "false");
  expect(foldButton()).toHaveTextContent(en ? "Resolved by Resolver" : "由 Resolver 解决");
  expect(foldButton()).toHaveTextContent(en ? "Show thread" : "展开讨论");
  // A page length is not a thread size while older comments may be missing.
  expect(foldButton()).not.toHaveTextContent(en ? "2 comments" : "2 条评论");
  mounted.rerender(list([resolvedRoot, threadReply], { allLoaded: true }));
  expect(foldButton()).toHaveTextContent(en ? "2 comments" : "2 条评论");
  await userEvent.click(foldButton());
  expect(foldButton()).toHaveAttribute("aria-expanded", "true");
  expect(foldButton()).toHaveTextContent(en ? "Hide thread" : "收起讨论");
  expect(row(commentId)).toBeVisible();
  expect(row(replyId)).toBeVisible();
  expect(screen.queryByRole("button", { name: en ? "Resolve thread" : "解决讨论" })).toBeNull();
  await userEvent.click(within(row(commentId)).getByRole("button", { name: en ? "Reopen thread" : "取消解决" }));
  await waitFor(() => expect(fetch.mock.calls.map(([url, init]) => [init.method, url])).toEqual([["POST", expect.stringContaining(`/comments/${commentId}/unresolve`)]]));
});

it("keeps a thread open when it becomes resolved while the page is open", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (_url: string, init: RequestInit) => new Response(JSON.stringify({ protocol_version: 2, issue_id: actor, comment_id: commentId,
    request_id: (init.headers as Record<string, string>)["Idempotency-Key"], status: 200, action: "resolve", emoji: null, code: "comment_resolved" }))));
  const mounted = renderWithI18n(list([comment, threadReply]));
  await userEvent.click(screen.getByRole("button", { name: "Resolve thread" }));
  expect(await screen.findByText("Discussion updated.")).toBeInTheDocument();
  mounted.rerender(list([resolvedRoot, threadReply]));
  expect(foldButton()).toHaveAttribute("aria-expanded", "true");
  expect(row(commentId)).toBeVisible();
  expect(row(replyId)).toBeVisible();
  expect(within(row(commentId)).getByText("Discussion updated.")).toBeVisible();
  expect(within(row(commentId)).getByRole("button", { name: "Reopen thread" })).toBeVisible();
  await userEvent.click(foldButton());
  expect(row(commentId)).not.toBeVisible();
});

it("keeps a resolved thread open while one of its requests is unconfirmed", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  saveCollaborationDraft(prepareCollaborationDiscussion(scope, issue, eventId, { action: "add_reaction", commentId: replyId, emoji: "🎉" }));
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ protocol_version: 2, issue_id: actor, comment_id: replyId,
    request_id: eventId, status: 200, action: "add_reaction", emoji: "🎉", code: "reaction_added" }))));
  renderWithI18n(list([resolvedRoot, threadReply]));
  expect(row(replyId)).toBeVisible();
  expect(foldButton()).toHaveAttribute("aria-expanded", "true");
  expect(foldButton()).toBeDisabled();
  await userEvent.click(within(row(replyId)).getByRole("button", { name: "Check original result" }));
  // Settling the request releases the bar without hiding the result.
  await waitFor(() => expect(foldButton()).toBeEnabled());
  expect(row(replyId)).toBeVisible();
  expect(within(row(replyId)).getByText("Discussion updated.")).toBeVisible();
});

it("hides a folded thread without discarding an unsent edit inside it", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ issue_id: actor, attachments: [] }))));
  renderWithI18n(list([resolvedRoot, threadReply]));
  await userEvent.click(foldButton());
  await chooseMenuItem("Edit comment", replyId);
  fireEvent.change(screen.getByLabelText("Edit comment text"), { target: { value: "Unsent thread edit" } });
  await userEvent.click(foldButton());
  expect(row(replyId)).not.toBeVisible();
  expect(screen.queryByRole("form", { name: "Edit comment" })).toBeNull();
  await userEvent.click(foldButton());
  expect(screen.getByLabelText("Edit comment text")).toHaveValue("Unsent thread edit");
});

it("opens a folded thread for each explicit navigation to one of its comments", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const comments = [resolvedRoot, threadReply];
  const mounted = renderWithI18n(list(comments, { revealCommentId: null, revealKey: 0 }));
  expect(row(replyId)).not.toBeVisible();
  mounted.rerender(list(comments, { revealCommentId: replyId, revealKey: 1 }));
  expect(row(replyId)).toBeVisible();
  expect(row(commentId)).toBeVisible();
  await userEvent.click(foldButton());
  mounted.rerender(list(comments, { revealCommentId: replyId, revealKey: 1 }));
  expect(row(replyId)).not.toBeVisible();
  mounted.rerender(list(comments, { revealCommentId: replyId, revealKey: 2 }));
  expect(row(replyId)).toBeVisible();
});

it("opens the thread of a navigation target that loads after the intent", () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const mounted = renderWithI18n(list([resolvedRoot], { revealCommentId: replyId, revealKey: 1 }));
  expect(row(commentId)).not.toBeVisible();
  mounted.rerender(list([resolvedRoot, threadReply], { revealCommentId: replyId, revealKey: 1 }));
  expect(row(replyId)).toBeVisible();
});

it("keeps visible replies open when an older page loads their resolved root", () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const mounted = renderWithI18n(list([threadReply]));
  expect(row(replyId)).toBeVisible();
  expect(within(row(replyId)).getByRole("link", { name: "View parent comment" })).toBeInTheDocument();
  mounted.rerender(list([resolvedRoot, threadReply]));
  expect(row(replyId)).toBeVisible();
  expect(foldButton()).toHaveAttribute("aria-expanded", "true");
  expect(within(row(replyId)).queryByRole("link", { name: "View parent comment" })).toBeNull();
});
