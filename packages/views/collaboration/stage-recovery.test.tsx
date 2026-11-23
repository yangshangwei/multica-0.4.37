import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { clearCollaborationAccess, collaborationScope, executeCollaborationStageIntent, prepareCollaborationStageAction, parseCollaborationIssue,
  parseCollaborationStageView, readCollaborationStageIntents } from "@multica/core/collaboration";
import { collaborationIssueFixture, stageIds, stageIssueContextFixture, stageViewManagedFixture, stageConfirmations, stageDigests } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationStageRecovery } from "./stage-panel";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((c) => c.clear()); localStorage.clear(); vi.unstubAllGlobals(); });
it("C01 denied detail recovery is accessible without an issue DTO and only checks the original key", async () => {
  const calls: { method: string; url: string }[] = [];
  let known = false;
  vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit = {}) => {
    calls.push({ method: init.method ?? "GET", url });
    if (init.method === "POST") throw new TypeError("Lost response");
    return new Response(JSON.stringify(known ? { code: "stage_proposal_accepted", collaboration_revision: "12", stage_confirmation: stageConfirmations.decide_proposal } : { code: "not_found" }), { status: known ? 200 : 404 });
  }));
  setApiInstance(new ApiClient("https://server.example"));const client = new QueryClient();clients.push(client);
  const scope = collaborationScope(stageIds.product), issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture })!;
  const intent = prepareCollaborationStageAction(scope, issue, parseCollaborationStageView(stageViewManagedFixture)!, { action: "decide_proposal",
    proposalId: stageIds.proposal, candidate: { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 }, decision: "accept", reason: "PRIVATE approval draft" });
  await executeCollaborationStageIntent(client, scope, intent);clearCollaborationAccess(client, scope, issue.id);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationStageRecovery scope={scope} issueId={issue.id} /></QueryClientProvider>);
  const region = screen.getByRole("region", { name: "Original stage request recovery" });
  expect(within(region).queryByText("PRIVATE approval draft")).not.toBeInTheDocument();
  expect(within(region).queryByRole("button", { name: "Send the same request again" })).not.toBeInTheDocument();
  await userEvent.click(within(region).getByRole("button", { name: "Check original result" }));
  expect(await within(region).findByText("The original result is still unknown. The request is kept.")).toBeVisible();
  known = true;await userEvent.click(within(region).getByRole("button", { name: "Check original result" }));
  await waitFor(() => expect(readCollaborationStageIntents(scope, { kind: "issue", issueId: issue.id })).toEqual([]));
  expect(calls.map((c) => c.method)).toEqual(["POST", "GET", "GET"]);
  expect(calls.slice(1).every((c) => c.url.endsWith(`/operations/${intent.requestId}`))).toBe(true);
});
