import { useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { accountPath, parseAccountPath } from "@multica/core/paths";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationDetail } from "./collaboration-detail";
import { CollaborationNotifications } from "./notifications";

const fixture = collaborationExecutionIssueFixture;
const execution = fixture.execution;
const settledAt = "2026-10-19T00:00:00Z";
const issue = parseCollaborationIssue({ ...fixture, execution: { ...execution,
  channels: [{ ...execution.channels[0], active_group_id: null }],
  targets: [{ ...execution.targets[0], active_group_id: null }],
  commands: [{ ...execution.commands[0], state: "failed" }],
  groups: [{ ...execution.groups[0], state: "failed", forced_disposition: null, settled_at: settledAt }],
  runs: [{ ...execution.runs[0], progress_state: "settled", proof_state: "exited_proven", business_outcome: "failure", forced_disposition: null, settled_at: settledAt }],
} })!;
const run = issue.execution!.runs[0]!;
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
vi.mock("@multica/core/collaboration", async (original) => ({ ...await original<typeof import("@multica/core/collaboration")>(), useCollaborationIssue: () => ({ data: issue }) }));
vi.mock("../platform", () => ({ useRestoredScrollRef: () => () => {} }));
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
vi.mock("./workflow", () => ({ CollaborationWorkflow: () => null }));
vi.mock("./activity", () => ({ CollaborationActivity: () => null }));
vi.mock("./discussion-actions", () => ({ CollaborationIssueReactions: () => null }));
vi.mock("./attachment-panel", () => ({ CollaborationAttachmentPanel: () => null, CollaborationCommentAttachments: () => null }));
vi.mock("./field-summary", () => ({ CollaborationFieldSummary: () => null }));
vi.mock("./comment-list", () => ({ CollaborationCommentList: () => null }));
vi.mock("./comment-composer", () => ({ CollaborationCommentComposer: () => null }));
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });

it("reopens and focuses a run from the same result notification after hiding its anchored records", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(issue.id);
  const destination = accountPath({ view: "participating", issueId: issue.id, runId: run.id });
  const replace = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(url.includes("/collaboration-notifications") ? {
    items: [{ id: issue.id, issue_id: issue.id, event_seq: "10", kind: "ai_failed", is_mention: false, source_kind: "issue", source_id: null,
      run_id: run.id, issue_title: issue.title, can_open: true, created_at: settledAt, read_at: null, archived_at: null }], unread_count: 1, next_cursor: null,
  } : url.includes("/records") ? {
    protocol_version: 1, issue_id: issue.id, run,
    resource: { agent_id: run.agent_id, runtime_id: issue.id, resource_owner_user_id: issue.id, execution_workspace_id: issue.id, grant_id: issue.id,
      provider: null, model: null, account_source: null },
    recording: { status: "not_recorded", inputs_recorded: false }, usage: null,
    items: [{ id: "result:1", kind: "result", outcome: "failure", body: "Result opened again" }], next_cursor: null,
  } : { issue_id: issue.id, items: [], next_cursor: null }))));
  function RoutedDetail() {
    const [path, setPath] = useState(destination);
    const url = new URL(path, "https://server.example");
    return <NavigationProvider value={{ pathname: url.pathname, searchParams: url.searchParams, hash: url.hash, push: setPath,
      replace: (next) => { replace(next); setPath(next); }, back: vi.fn(), getShareableUrl: (next) => `https://server.example${next}` }}>
      <CollaborationNotifications scope={scope} view="participating" />
      <section data-tab-scroll-root="account-detail"><CollaborationDetail scope={scope} destination={parseAccountPath(path)!} /></section>
    </NavigationProvider>;
  }
  renderWithI18n(<QueryClientProvider client={client}><RoutedDetail /></QueryClientProvider>);
  expect(await screen.findByText("Result opened again")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: `Hide records for ${run.id}` }));
  expect(screen.queryByText("Result opened again")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /Notifications/ }));
  await userEvent.click(await screen.findByRole("link", { name: issue.title }));
  expect(await screen.findByText("Result opened again")).toBeInTheDocument();
  expect(document.getElementById(`run-${run.id}`)).toHaveFocus();
  expect(replace).toHaveBeenCalledWith(accountPath({ view: "participating", issueId: issue.id }));
  const trigger = screen.getByRole("button", { name: /Notifications/ });
  await userEvent.click(trigger);
  await userEvent.keyboard("{Escape}");
  expect(trigger).toHaveFocus();
});
