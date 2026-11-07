import { useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import { collaborationScope, parseCollaborationIssue, type CollaborationIssue } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { renderWithI18n } from "../test/i18n";
import { CollaborationDetail } from "./collaboration-detail";

const state = vi.hoisted(() => ({ comments: [] as { id: string; created_at: string }[], total: undefined as number | undefined, pending: true, available: true, children: undefined as CollaborationIssue["ordinary_children"] }));
const navigation = vi.hoisted(() => ({ push: vi.fn(), prefetch: vi.fn(), isScopeResume: false, navigationKey: 0, getShareableUrl: (path: string) => `https://detail.example${path}` }));
const restoreScroll = vi.hoisted(() => vi.fn());
const client = new QueryClient();
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
vi.mock("@multica/core/collaboration", async (original) => ({ ...await original<typeof import("@multica/core/collaboration")>(), useCollaborationIssue: (_scope: unknown, id: string) => ({ data: state.available ? { ...issue, id, ordinary_children: state.children } : undefined }) }));
vi.mock("../navigation/context", () => ({ useNavigation: () => navigation }));
vi.mock("@tanstack/react-query", async (original) => ({ ...await original<typeof import("@tanstack/react-query")>(),
  useQueryClient: () => client,
  useQuery: () => ({ data: undefined }),
  useInfiniteQuery: () => ({ data: { pages: [{ items: state.comments, total: state.total }] }, isPending: state.pending, isSuccess: !state.pending }),
}));
vi.mock("../platform", () => ({ useRestoredScrollRef: () => restoreScroll }));
vi.mock("../rich-content", () => ({ RichContent: () => null }));
vi.mock("./workflow", () => ({ CollaborationWorkflow: () => null }));
vi.mock("./execution-panel", () => ({ CollaborationExecutionPanel: ({ selectedRunId }: { selectedRunId?: string }) => selectedRunId ? <div id={`run-${selectedRunId}`} data-testid="linked-run" tabIndex={-1} /> : null }));
// The detail composition suite isolates its sibling identities; shared-context
// query/mutation behavior is covered by memory-panel.test.tsx.
vi.mock("./memory-panel", () => ({ CollaborationMemoryPanel: () => <section id="memory" data-testid="memory-mount" tabIndex={-1} /> }));
vi.mock("./knowledge-panel", () => ({ CollaborationKnowledgePanel: ({ issue: value }: { issue: CollaborationIssue }) => <div data-testid="knowledge-mount">{value.id}</div> }));
vi.mock("./activity", () => ({ CollaborationActivity: () => null }));
vi.mock("./discussion-actions", () => ({ CollaborationIssueReactions: () => null }));
vi.mock("./attachment-panel", () => ({ CollaborationAttachmentPanel: () => <section data-testid="files-mount" /> }));
vi.mock("./field-summary", () => ({ CollaborationFieldSummary: () => <section data-testid="fields-mount" /> }));
// The detail owns these two sibling component identities; their behavior is
// tested separately in comment-list and comment-composer suites.
vi.mock("./comment-list", () => ({ CollaborationCommentList: ({ comments }: { comments: { id: string }[] }) => <ol data-testid="comment-list">{comments.map((comment) => <li id={`comment-${comment.id}`} key={comment.id} />)}</ol> }));
vi.mock("./comment-composer", () => ({ CollaborationCommentComposer: function Composer() {
  const [value, setValue] = useState("");
  return <form data-testid="comment-composer"><textarea aria-label="Composer draft" value={value} onChange={(event) => setValue(event.target.value)} /></form>;
} }));
afterEach(() => { client.clear(); state.comments = []; state.total = undefined; state.pending = true; state.available = true; state.children = undefined; navigation.push.mockClear(); navigation.prefetch.mockClear(); navigation.isScopeResume = false; navigation.navigationKey = 0; restoreScroll.mockReset(); vi.restoreAllMocks(); });

it.each(["en", "zh-Hans"] as const)("keeps the main journey visible and task metadata in its resource drawer in %s", async (locale) => {
  setApiInstance(new ApiClient("https://detail.example"));
  const title = locale === "en" ? "Issue details" : "任务信息";
  renderWithI18n(<CollaborationDetail scope={collaborationScope(issue.creator.user_id)} destination={{ view: "participating", issueId: issue.id, commentId: null }} />, { locale });
  const requirements = document.getElementById("description")!;
  const delivery = document.getElementById("delivery")!;
  const discussion = document.getElementById("discussion")!;
  const resources = screen.getByRole("group", { name: locale === "en" ? "Issue resources" : "任务资料" });
  for (const [before, after] of [[resources, requirements], [requirements, delivery], [delivery, discussion]]) {
    expect(before!.compareDocumentPosition(after!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  }
  expect(screen.getByTestId("comment-composer")).toBeVisible();
  expect(screen.getByTestId("fields-mount")).not.toBeVisible();
  expect(screen.getByTestId("knowledge-mount")).not.toBeVisible();
  await userEvent.click(within(resources).getByRole("button", { name: title }));
  const context = screen.getByRole("dialog", { name: title });
  expect(screen.getByTestId("fields-mount")).toBeVisible();
  expect(within(context).getByText(issue.source_workspace.name)).toBeVisible();
});

it("opens reference files directly from requirements without exposing the other auxiliaries", async () => {
  setApiInstance(new ApiClient("https://detail.example"));
  renderWithI18n(<CollaborationDetail scope={collaborationScope(issue.creator.user_id)} destination={{ view: "participating", issueId: issue.id, commentId: null }} />);
  expect(screen.getByTestId("files-mount")).not.toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Files and reference material" }));
  expect(screen.getByTestId("files-mount")).toBeVisible();
  expect(screen.getByRole("dialog", { name: "Attachments" })).toBeVisible();
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(screen.getByRole("button", { name: "Files and reference material" })).toHaveFocus());
  expect(screen.getByTestId("fields-mount")).not.toBeVisible();
  expect(screen.getByTestId("knowledge-mount")).not.toBeVisible();
});

it("reopens and focuses shared context on each explicit anchor navigation", async () => {
  setApiInstance(new ApiClient("https://detail.example"));
  const view = () => <section data-tab-scroll-root="account-detail"><CollaborationDetail scope={collaborationScope(issue.creator.user_id)} destination={{ view: "participating", issueId: issue.id, commentId: null, anchor: "memory" }} /></section>;
  const mounted = renderWithI18n(view());
  const memory = screen.getByTestId("memory-mount");
  expect(memory).toBeVisible();
  await waitFor(() => expect(memory).toHaveFocus());
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(memory).not.toBeVisible());
  screen.getByLabelText("Composer draft").focus();
  navigation.navigationKey += 1;
  mounted.rerender(view());
  expect(memory).toBeVisible();
  await waitFor(() => expect(memory).toHaveFocus());
});

it("shows the loaded range and keeps an older server's unknown total explicit", () => {
  setApiInstance(new ApiClient("https://detail.example"));
  state.pending = false;
  state.total = 5;
  state.comments = [{ id: "00000000-0000-4000-8000-000000000009", created_at: "2026-10-31T00:00:00Z" }];
  const view = () => <CollaborationDetail scope={collaborationScope(issue.creator.user_id)} destination={{ view: "participating", issueId: issue.id, commentId: null }} />;
  const mounted = renderWithI18n(view());
  expect(screen.getByTestId("knowledge-mount")).toHaveTextContent(issue.id);
  expect(screen.getByText("Loaded 1 of 5 comments")).toBeInTheDocument();
  state.total = undefined;
  mounted.rerender(view());
  expect(screen.getByText("Comment count is unavailable on this server.")).toBeInTheDocument();
});

it("restores an anchored detail on scope resume and refocuses repeated explicit opens", () => {
  setApiInstance(new ApiClient("https://detail.example"));
  const runId = "00000000-0000-4000-8000-000000000009";
  navigation.isScopeResume = true;
  const view = () => <section data-tab-scroll-root="account-detail"><CollaborationDetail scope={collaborationScope(issue.creator.user_id)} destination={{ view: "current", issueId: issue.id, commentId: null, runId }} /></section>;
  const mounted = renderWithI18n(view());
  expect(restoreScroll).toHaveBeenCalledWith(document.querySelector('[data-tab-scroll-root="account-detail"]'));
  expect(screen.getByTestId("linked-run")).not.toHaveFocus();
  navigation.isScopeResume = false;
  navigation.navigationKey += 1;
  mounted.rerender(view());
  expect(screen.getByTestId("linked-run")).toHaveFocus();
  screen.getByLabelText("Composer draft").focus();
  mounted.rerender(view());
  expect(screen.getByLabelText("Composer draft")).toHaveFocus();
  navigation.navigationKey += 1;
  mounted.rerender(view());
  expect(screen.getByTestId("linked-run")).toHaveFocus();
});

// Wire/privacy matrices belong to core's child schema tests. This verifies the
// common detail composition used by account, source, panels and native windows.
it("opens ordinary children in their returned source workspace and downgrades to references after source access changes", async () => {
  setApiInstance(new ApiClient("https://detail.example"));
  const scope = collaborationScope(issue.creator.user_id);
  const child = { id: "00000000-0000-4000-8000-000000000009", identifier: "SRC-9", title: "Independent child", can_open: true };
  state.children = { source_workspace_slug: "actual-source", items: [child] };
  const view = () => <CollaborationDetail scope={scope} destination={{ view: "participating", issueId: issue.id, commentId: null }} />;
  const mounted = renderWithI18n(view());
  await userEvent.click(within(screen.getByRole("group", { name: "Issue resources" })).getByRole("button", { name: /^Sub-issues/ }));
  const section = screen.getByRole("region", { name: "Sub-issues" });
  const link = within(section).getByRole("link", { name: /SRC-9 Independent child/ });
  expect(link).toHaveAttribute("href", `/actual-source/issues/${child.id}`);
  await userEvent.click(link);
  expect(navigation.push).toHaveBeenCalledWith(`/actual-source/issues/${child.id}`);

  state.children = { source_workspace_slug: null, items: [{ ...child, can_open: false }] };
  mounted.rerender(view());
  navigation.prefetch.mockClear();
  await userEvent.hover(within(section).getByText("Independent child"));
  expect(within(section).queryByRole("link")).not.toBeInTheDocument();
  expect(navigation.prefetch).not.toHaveBeenCalled();
});

it("distinguishes missing child support from an explicitly empty projected list", async () => {
  setApiInstance(new ApiClient("https://detail.example"));
  const scope = collaborationScope(issue.creator.user_id);
  const view = () => <CollaborationDetail scope={scope} destination={{ view: "participating", issueId: issue.id, commentId: null }} />;
  const mounted = renderWithI18n(view());
  await userEvent.click(within(screen.getByRole("group", { name: "Issue resources" })).getByRole("button", { name: /^Sub-issues/ }));
  expect(screen.getByText("Sub-issue references are unavailable on this server.")).toBeInTheDocument();
  expect(screen.queryByText("No sub-issues.")).not.toBeInTheDocument();
  state.children = { source_workspace_slug: null, items: [] };
  mounted.rerender(view());
  expect(screen.getByText("No sub-issues.")).toBeInTheDocument();
});

it("focuses the selected run and copies its account anchor from the shared detail", async () => {
  const user = userEvent.setup();
  const clipboard = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
  setApiInstance(new ApiClient("https://detail.example"));
  const runId = "00000000-0000-4000-8000-000000000009";
  renderWithI18n(<section data-tab-scroll-root="account-detail"><CollaborationDetail scope={collaborationScope(issue.creator.user_id)} destination={{ view: "current", issueId: issue.id, commentId: null, runId }} /></section>);
  expect(screen.getByTestId("linked-run")).toHaveFocus();
  await user.click(screen.getByRole("button", { name: "Copy link" }));
  expect(clipboard).toHaveBeenCalledWith(`https://detail.example/account?view=participating&issue=${issue.id}#run-${runId}`);
});

it("keeps one comment list and composer across loading, polling and first-comment updates", async () => {
  const errors = vi.spyOn(console, "error").mockImplementation(() => {});
  setApiInstance(new ApiClient("https://detail.example"));
  const scope = collaborationScope(issue.creator.user_id);
  const view = (id = issue.id) => <CollaborationDetail scope={scope} destination={{ view: "participating", issueId: id, commentId: null }} />;
  const mounted = renderWithI18n(view());
  await userEvent.type(screen.getByLabelText("Composer draft"), "Draft retained across refetch");
  state.pending = false;
  mounted.rerender(view());
  mounted.rerender(view());
  const commentId = "00000000-0000-4000-8000-000000000002";
  state.comments = [{ id: commentId, created_at: "2026-10-19T00:00:00Z" }];
  mounted.rerender(view());
  mounted.rerender(view());
  expect(screen.getAllByTestId("comment-list")).toHaveLength(1);
  expect(document.querySelectorAll(`[id="comment-${commentId}"]`)).toHaveLength(1);
  expect(screen.getAllByTestId("comment-composer")).toHaveLength(1);
  expect(screen.getByLabelText("Composer draft")).toHaveValue("Draft retained across refetch");
  expect(errors.mock.calls.some((call) => String(call[0]).includes("same key"))).toBe(false);

  state.comments = [];
  mounted.rerender(view("00000000-0000-4000-8000-000000000003"));
  expect(screen.getByLabelText("Composer draft")).toHaveValue("");
  expect(screen.getAllByTestId("comment-list")).toHaveLength(1);
});

it("removes an open resource drawer when its protected issue is no longer available", async () => {
  setApiInstance(new ApiClient("https://detail.example"));
  const view = () => <CollaborationDetail scope={collaborationScope(issue.creator.user_id)} destination={{ view: "participating", issueId: issue.id, commentId: null }} />;
  const mounted = renderWithI18n(view());
  await userEvent.click(within(screen.getByRole("group", { name: "Issue resources" })).getByRole("button", { name: "Issue details" }));
  expect(screen.getByRole("dialog")).toBeVisible();
  expect(within(screen.getByRole("dialog")).getByText(issue.source_workspace.name)).toBeVisible();
  state.available = false;
  mounted.rerender(view());
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByText(issue.source_workspace.name)).toBeNull();
  expect(screen.queryByTestId("knowledge-mount")).toBeNull();
});
