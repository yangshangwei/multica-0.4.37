import { useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import { collaborationScope, parseCollaborationIssue } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { renderWithI18n } from "../test/i18n";
import { CollaborationDetail } from "./collaboration-detail";

const state = vi.hoisted(() => ({ comments: [] as { id: string; created_at: string }[], pending: true }));
const client = new QueryClient();
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
vi.mock("@multica/core/collaboration", async (original) => ({ ...await original<typeof import("@multica/core/collaboration")>(), useCollaborationIssue: (_scope: unknown, id: string) => ({ data: { ...issue, id } }) }));
vi.mock("@tanstack/react-query", async (original) => ({ ...await original<typeof import("@tanstack/react-query")>(),
  useQueryClient: () => client,
  useQuery: () => ({ data: undefined }),
  useInfiniteQuery: () => ({ data: { pages: [{ items: state.comments }] }, isPending: state.pending, isSuccess: !state.pending }),
}));
vi.mock("../platform", () => ({ useRestoredScrollRef: () => () => {} }));
vi.mock("../rich-content", () => ({ RichContent: () => null }));
vi.mock("./workflow", () => ({ CollaborationWorkflow: () => null }));
vi.mock("./execution-panel", () => ({ CollaborationExecutionPanel: () => null }));
vi.mock("./activity", () => ({ CollaborationActivity: () => null }));
vi.mock("./discussion-actions", () => ({ CollaborationIssueReactions: () => null }));
vi.mock("./attachment-panel", () => ({ CollaborationAttachmentPanel: () => null }));
vi.mock("./field-summary", () => ({ CollaborationFieldSummary: () => null }));
// The detail owns these two sibling component identities; their behavior is
// tested separately in comment-list and comment-composer suites.
vi.mock("./comment-list", () => ({ CollaborationCommentList: ({ comments }: { comments: { id: string }[] }) => <ol data-testid="comment-list">{comments.map((comment) => <li id={`comment-${comment.id}`} key={comment.id} />)}</ol> }));
vi.mock("./comment-composer", () => ({ CollaborationCommentComposer: function Composer() {
  const [value, setValue] = useState("");
  return <form data-testid="comment-composer"><textarea aria-label="Composer draft" value={value} onChange={(event) => setValue(event.target.value)} /></form>;
} }));
afterEach(() => { client.clear(); state.comments = []; state.pending = true; vi.restoreAllMocks(); });

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
