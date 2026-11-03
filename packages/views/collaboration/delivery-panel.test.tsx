import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, type CollaborationIssue, type CollaborationSubmission } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationDeliveryPanel } from "./delivery-panel";

// Core delivery-history/result-operation tests own page, provenance and
// original-intent matrices. This suite exercises the real result disclosures.
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const issueId = collaborationIssueFixture.id;
const author = { ...collaborationIssueFixture.handler, user_id: "00000000-0000-4000-8000-000000000002", name: "Result author" };
const current: CollaborationSubmission = {
  id: "00000000-0000-4000-8000-000000000009", issue_id: issueId, event_seq: "8", lifecycle_epoch: "1",
  author_user_id: author.user_id, author, created_at: "2026-11-03T01:00:00Z", result: "Current frozen result",
  basis: "Current verified basis", attachment_ids: [], source_comment_id: "00000000-0000-4000-8000-000000000003",
  evidence_state: "available", decision: null,
};
const prior: CollaborationSubmission = {
  ...current, id: "00000000-0000-4000-8000-000000000008", event_seq: "3", created_at: "2026-11-03T00:00:00Z",
  result: "Earlier frozen result", basis: "Earlier recorded basis", source_comment_id: null,
  decision: {
    kind: "returned", submission_id: "00000000-0000-4000-8000-000000000008", event_seq: "4",
    actor_user_id: issueId, actor: collaborationIssueFixture.creator, created_at: "2026-11-03T00:30:00Z",
    reason: "Cover the missing case", target_user_id: author.user_id,
  },
};
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); });

function detail(submission: CollaborationSubmission, completed = false) {
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, status: completed ? "done" : "in_review", status_category: completed ? "done" : "in_review",
    delivery: { current: completed ? null : submission, latest: submission, has_history: true } });
  if (!issue) throw new Error("Invalid delivery detail fixture");
  return issue;
}
function mount(issue: CollaborationIssue, locale: "en" | "zh-Hans" = "en") {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const navigation = { pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(),
    getShareableUrl: (path: string) => `https://server.example${path}` };
  return renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}>
    <CollaborationDeliveryPanel scope={collaborationScope(issueId)} issue={issue} />
  </NavigationProvider></QueryClientProvider>, { locale });
}

it("keeps exact current and earlier results distinct while loading prior decisions and only selected evidence", async () => {
  const file = { id: "00000000-0000-4000-8000-000000000010", draft_id: current.id, issue_id: issueId, intended_issue_id: issueId,
    source_workspace_id: issueId, uploader_user_id: author.user_id, name: "retained.txt", content_type: "text/plain", size_bytes: 5,
    version: "9007199254740993", created_at: current.created_at, delivered_at: current.created_at };
  const selected = { ...current, attachment_ids: [file.id] };
  const historyReads = vi.fn(async (url: string) => new Response(JSON.stringify(new URL(url).searchParams.has("cursor")
    ? { issue_id: issueId, items: [prior], next_cursor: null }
    : { issue_id: issueId, items: [selected], next_cursor: "older-results" })));
  const fetch = vi.fn(async (url: string, _init?: RequestInit) => {
    const path = new URL(url).pathname;
    if (path.endsWith("/acceptance-submissions")) return historyReads(url);
    if (path.endsWith("/attachments")) return new Response(JSON.stringify({ issue_id: issueId, attachments: [file,
      { ...file, id: "00000000-0000-4000-8000-000000000011", name: "other.txt" }] }));
    throw new Error(`Unexpected result request: ${path}`);
  });
  vi.stubGlobal("fetch", fetch);
  mount(detail(selected));
  const panel = screen.getByRole("region", { name: "Delivered result" });
  expect(within(panel).getByText(selected.result!)).toBeInTheDocument();
  expect(within(panel).getByText(selected.basis!)).toBeInTheDocument();
  expect(within(panel).getByText(author.name)).toBeInTheDocument();
  expect(panel.querySelector("time")).toHaveAttribute("datetime", selected.created_at);
  expect(await within(panel).findByRole("button", { name: "Download retained.txt" })).toBeInTheDocument();
  expect(within(panel).queryByText("other.txt")).not.toBeInTheDocument();
  expect(within(panel).getByRole("link", { name: "Open the source comment" })).toHaveAttribute("href", `/account?view=participating&issue=${issueId}#comment-${current.source_comment_id}`);
  expect(historyReads).not.toHaveBeenCalled();
  await userEvent.click(within(panel).getByRole("button", { name: "Earlier results and decisions" }));
  await waitFor(() => expect(historyReads).toHaveBeenCalledTimes(1));
  expect(within(panel).queryByText("No earlier results.")).not.toBeInTheDocument();
  expect(within(panel).queryByText(prior.result!)).not.toBeInTheDocument();
  await userEvent.click(within(panel).getByRole("button", { name: "Load more" }));
  expect(await within(panel).findByText(prior.result!)).toBeInTheDocument();
  const earlier = panel.querySelector<HTMLElement>(`[data-submission-id="${prior.id}"]`)!;
  expect(within(earlier).getByText(prior.basis!)).toBeInTheDocument();
  expect(within(earlier).getByText("Returned for revision")).toBeInTheDocument();
  expect(within(earlier).getByText(prior.decision!.reason!)).toBeInTheDocument();
  expect(panel.querySelectorAll(`[data-submission-id="${current.id}"]`)).toHaveLength(1);
  expect(historyReads.mock.calls[1]?.[0]).toContain("cursor=older-results");
  expect(fetch.mock.calls.every(([, init]) => (init?.method ?? "GET") === "GET")).toBe(true);
});

it("keeps the current result readable when earlier history is unavailable and retries the read", async () => {
  let reads = 0;
  const fetch = vi.fn(async (url: string, _init?: RequestInit) => {
    const path = new URL(url).pathname;
    if (!path.endsWith("/acceptance-submissions")) throw new Error(`Unexpected result request: ${path}`);
    return ++reads === 1 ? new Response(JSON.stringify({ error: "Unavailable" }), { status: 503 })
      : new Response(JSON.stringify({ issue_id: issueId, items: [current, prior], next_cursor: null }));
  });
  vi.stubGlobal("fetch", fetch);
  mount(detail(current));
  const panel = screen.getByRole("region", { name: "Delivered result" });
  await userEvent.click(within(panel).getByRole("button", { name: "Earlier results and decisions" }));
  expect(await within(panel).findByRole("alert")).toBeInTheDocument();
  expect(within(panel).getByText(current.result!)).toBeInTheDocument();
  expect(within(panel).queryByText("No earlier results.")).not.toBeInTheDocument();
  await userEvent.click(within(panel).getByRole("button", { name: "Refresh" }));
  expect(await within(panel).findByText(prior.result!)).toBeInTheDocument();
  expect(reads).toBe(2);
  expect(fetch.mock.calls.every(([, init]) => (init?.method ?? "GET") === "GET")).toBe(true);
});

it("shows unavailable legacy basis and author without inventing an acceptance", () => {
  const legacy: CollaborationSubmission = { ...current, lifecycle_epoch: null, author: null, basis: null, source_comment_id: null, evidence_state: "legacy" };
  mount(detail(legacy));
  const panel = screen.getByRole("region", { name: "Delivered result" });
  expect(within(panel).getByText(legacy.result!)).toBeInTheDocument();
  expect(within(panel).getByText("The basis is unavailable.")).toBeInTheDocument();
  expect(within(panel).getByText("Delivery evidence is unavailable. Revise or resubmit before acceptance.")).toBeInTheDocument();
  expect(within(panel).queryByText("Accepted", { exact: true })).not.toBeInTheDocument();
  expect(within(panel).queryByText("Decision by")).not.toBeInTheDocument();
});

it.each([
  ["en", "No initiator acceptance is recorded for this result.", "Accepted"],
  ["zh-Hans", "此结果未记录发起人验收。", "已验收"],
] as const)("shows factual completion without a linked initiator decision in %s", (locale, message, accepted) => {
  mount(detail(current, true), locale);
  const panel = screen.getByRole("region", { name: locale === "en" ? "Delivered result" : "交付结果" });
  expect(within(panel).getByText(current.result!)).toBeInTheDocument();
  expect(within(panel).getByText(message)).toBeInTheDocument();
  expect(within(panel).queryByText(accepted, { exact: true })).not.toBeInTheDocument();
});
