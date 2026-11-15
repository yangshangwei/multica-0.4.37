import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationIssueAccessDenied, collaborationScope, parseCollaborationIssue, readCollaborationStageIntents } from "@multica/core/collaboration";
import {
  collaborationIssueFixture, stageCandidateV1Fixture, stageCandidateV2Fixture, stageConfirmations, stageDigests, stageIds, stageIssueContextFixture, stageObservationsFixture,
  stagePolicyFixture, stagePolicyVirtualFixture, stageProposalAcceptedDetailFixture, stageProposalDetailFixture, stageProposalPageFixture, stageRolesFixture,
  stageViewManagedFixture, stageViewNeedsConfirmationFixture, stageViewUnmanagedFixture, stageChildFixture, stagePullRequestFixture,
} from "@multica/core/collaboration/test-fixtures";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationStagePanel } from "./stage-panel";

// Core stage-schema / stage-actions / stage-presentation tests own the parsing,
// confirmation-matching, recovery and capability matrices. This suite keeps
// the happy paths, the wiring to the frozen wire contract, accessibility and
// the named "never shown as passed" regressions.
const viewer = stageIds.product;
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); });

const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture, handler: { ...collaborationIssueFixture.handler, name: "Hana Handler" } })!;
const issuePath = `/api/collaboration/issues/${stageIds.issue}`;
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const participants = { protocol_version: 1, issue_id: stageIds.issue, collaboration_revision: collaborationIssueFixture.collaboration_revision,
  participants: (["business_owner", "implementer", "product_approver", "code_reviewer", "qa_approver", "release_approver"] as const)
    .map((role) => ({ ...stageRolesFixture[role], account_active: true })) };

type Route = (request: { method: string; url: URL; init: RequestInit }) => Response | Promise<Response> | undefined;
/** Endpoint-routed fake server: an unexpected request fails the test instead of being answered. */
function serve(...routes: Route[]) {
  const calls: { method: string; path: string; headers: Record<string, string>; body: unknown }[] = [];
  const fetch = vi.fn(async (input: string, init: RequestInit = {}) => {
    const url = new URL(input), method = (init.method ?? "GET").toUpperCase();
    calls.push({ method, path: `${url.pathname}${url.search}`, headers: (init.headers ?? {}) as Record<string, string>, body: typeof init.body === "string" ? JSON.parse(init.body) : undefined });
    for (const route of routes) { const response = await route({ method, url, init }); if (response) return response; }
    if (method === "GET" && url.pathname === `/api/workspaces/${stageIds.workspace}/members`) return json([{ id: stageIds.product, user_id: stageIds.product, workspace_id: stageIds.workspace, role: "owner" }]);
    throw new Error(`Unexpected stage request: ${method} ${url.pathname}`);
  });
  vi.stubGlobal("fetch", fetch);
  return { calls, writes: () => calls.filter((call) => call.method !== "GET") };
}
const get = (suffix: string, data: () => unknown, status = 200): Route => ({ method, url }) => method === "GET" && url.pathname === `${issuePath}${suffix}` ? json(data(), status) : undefined;
const stageView = (view: unknown): Route => get("/stage", () => view);
const proposalList = (page: unknown = stageProposalPageFixture): Route => get("/stage/proposals", () => page);
const proposalDetail = (detail: () => unknown): Route => get(`/stage/proposals/${stageIds.proposal}`, detail);
const noProposals = proposalList({ issue_id: stageIds.issue, items: [], next_cursor: null });

function mount(locale: "en" | "zh-Hans" = "en") {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const navigation = { pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path: string) => `https://server.example${path}` };
  const scope = collaborationScope(viewer);
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}><CollaborationStagePanel scope={scope} issue={issue} /></NavigationProvider></QueryClientProvider>, { locale });
  return { client, scope };
}
const panel = (name = "Stage gates") => screen.getByRole("region", { name });
const stageButtons = () => within(panel()).queryAllByRole("button").map((button) => button.textContent);

it.each([
  ["an older server without the route", (): Response => new Response("404 page not found", { status: 404 }), "Stage gates are not available on this server.", []],
  ["a malformed view", (): Response => json({ ...stageViewManagedFixture, managed: "yes" }), "Stage gate data is unavailable. Nothing shown here is confirmed.", ["Refresh"]],
  ["an unready schema", (): Response => json({ code: "stage_schema_unavailable", error: "x" }, 503), "Stage gates are not ready on this server yet.", ["Refresh"]],
])("renders %s as unavailable with no stage action and never as a satisfied gate", async (_name, answer, text, buttons) => {
  serve(({ method, url }) => method === "GET" && url.pathname === `${issuePath}/stage` ? answer() : undefined);
  mount();
  expect(await within(panel()).findByText(text)).toBeVisible();
  expect(stageButtons()).toEqual(buttons);
  expect(within(panel()).queryByText(/Confirmed for the current lifecycle|Approval is current|Enabled/)).not.toBeInTheDocument();
});

it("disables every action and says so when the server reports a state this app does not know", async () => {
  serve(stageView({ ...stageViewManagedFixture, stage: { current: "hotfix", stage_revision: "3" } }), proposalList());
  mount();
  expect(await within(panel()).findByRole("alert")).toHaveTextContent("The server reported a state this app does not recognize. Actions stay disabled.");
  expect(within(panel()).getByText("Unknown stage")).toBeVisible();
  for (const name of ["Change roles", "Confirm existing sub-issues", "File a proposal"]) expect(within(panel()).queryByRole("button", { name })).not.toBeInTheDocument();
});

it.each([["en", "Stage gates", "Stage gates are turned off for this project.", "Project stage policy"], ["zh-Hans", "阶段门禁", "此项目已关闭阶段门禁。", "项目阶段策略"]] as const)(
  "shows an unmanaged root as not enabled and offers no stage write in %s", async (locale, title, text, policy) => {
    const server = serve(stageView(stageViewUnmanagedFixture));
    mount(locale);
    expect(await within(panel(title)).findByText(text)).toBeVisible();
    expect(within(panel(title)).getAllByRole("button").map((button) => button.textContent)).toEqual([policy]);
    expect(server.writes()).toEqual([]);
  });

it("keeps earlier roles, sub-issues and proposals readable as history on an unmanaged root, with nothing to act on", async () => {
  const server = serve(stageView({ ...stageViewUnmanagedFixture, role_revision: "1", roles: stageRolesFixture }), proposalList(), proposalDetail(() => stageProposalDetailFixture));
  mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Earlier stage records" }));
  expect(await within(panel()).findByText("Ivan Implementer")).toBeVisible();
  await userEvent.click(await within(panel()).findByRole("button", { name: /^Open proposal/ }));
  const record = await waitFor(() => { const node = document.querySelector<HTMLElement>(`[data-proposal-id="${stageIds.proposal}"]`); expect(node).not.toBeNull(); return node!; });
  expect(within(record).getByText("Split the refund flow (approver edit)")).toBeVisible();
  expect(within(record).queryByRole("button")).not.toBeInTheDocument();
  for (const name of ["Configure roles", "Change roles", "File a proposal", "Confirm existing sub-issues"]) expect(within(panel()).queryByRole("button", { name })).not.toBeInTheDocument();
  expect(server.writes()).toEqual([]);
});

it("shows policy, stage, the business owner apart from the current handler, and what still needs confirmation", async () => {
  serve(stageView(stageViewNeedsConfirmationFixture), noProposals);
  mount();
  const region = panel();
  expect(await within(region).findByText("Enabled · revision 1")).toBeVisible();
  expect(within(region).getByText("Development")).toBeVisible();
  const facts = within(region).getAllByRole("definition").map((item) => item.textContent);
  expect(facts).toEqual(expect.arrayContaining(["Not configured", "Hana Handler"]));
  const missing = within(region).getByText("Confirmation needed").closest("div")!;
  expect(within(missing).getByText("Roles are not configured.")).toBeVisible();
  expect(within(missing).getByText("Existing sub-issues are not confirmed for the current lifecycle.")).toBeVisible();
  const child = within(region).getByRole("link", { name: "SRC-12 Existing checkout child" });
  expect(child).toHaveAttribute("href", `/source/issues/${stageIds.child1}`);
  expect(within(region).getByText("Not confirmed", { selector: "p" })).toBeVisible();
  // The hints of this view offer roles and proposals, not a decision or a scope confirmation.
  expect(within(region).getByRole("button", { name: "Configure roles" })).toBeEnabled();
  expect(within(region).queryByRole("button", { name: "Confirm existing sub-issues" })).not.toBeInTheDocument();
});

it("shows a role holder whose account no longer resolves as unavailable and makes the configurer choose again", async () => {
  serve(stageView({ ...stageViewManagedFixture, roles: { ...stageRolesFixture, business_owner: null } }), noProposals, get("/participants", () => participants));
  mount();
  const region = panel();
  await within(region).findByText("Enabled · revision 1");
  // Configured but unresolvable is not the same statement as "not configured".
  expect(within(region).getAllByText("Account unavailable", { selector: "dd" })).toHaveLength(2);
  expect(within(region).queryByText("Not configured", { selector: "dd" })).not.toBeInTheDocument();
  await userEvent.click(within(region).getByRole("button", { name: "Change roles" }));
  const dialog = await screen.findByRole("dialog", { name: "Change roles" });
  expect(within(dialog).getByRole("combobox", { name: "Business owner" })).toHaveValue("");
  expect(within(dialog).getByRole("combobox", { name: "Implementer" })).toHaveValue(stageIds.implementer);
});

it("configures the six roles through one frozen POST and blocks a separation conflict before any request", async () => {
  const server = serve(stageView(stageViewNeedsConfirmationFixture), noProposals, get("/participants", () => participants),
    ({ method, url }) => method === "POST" && url.pathname === `${issuePath}/stage/roles` ? json({ code: "stage_roles_configured", collaboration_revision: "9007199254740994", stage_confirmation: stageConfirmations.configure_roles }) : undefined);
  mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Configure roles" }));
  const dialog = await screen.findByRole("dialog", { name: "Configure roles" });
  const form = within(dialog).getByRole("form", { name: "Configure roles" });
  const select = (name: string) => within(form).getByRole("combobox", { name });
  await within(select("Business owner")).findByRole("option", { name: "Olivia Owner" });
  const choice = { "Business owner": stageIds.owner, Implementer: stageIds.implementer, "Product approver": stageIds.product, "Code reviewer": stageIds.implementer, "QA approver": stageIds.qa, "Release approver": stageIds.release };
  for (const [name, value] of Object.entries(choice)) await userEvent.selectOptions(select(name), value);
  expect(within(dialog).getByText("The implementer cannot review their own code.")).toBeVisible();
  expect(select("Code reviewer")).toHaveAttribute("aria-invalid", "true");
  await userEvent.click(within(dialog).getByRole("button", { name: "Save roles" }));
  expect(server.writes()).toEqual([]);
  await userEvent.selectOptions(select("Code reviewer"), stageIds.reviewer);
  await userEvent.click(within(dialog).getByRole("button", { name: "Save roles" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  const [write] = server.writes();
  expect(write).toMatchObject({ method: "POST", path: `${issuePath}/stage/roles` });
  expect(write!.headers["Idempotency-Key"]).toMatch(/^[0-9a-f-]{36}$/);
  expect(write!.body).toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "0", roles: {
    business_owner_user_id: stageIds.owner, implementer_user_id: stageIds.implementer, product_approver_user_id: stageIds.product,
    code_reviewer_user_id: stageIds.reviewer, qa_approver_user_id: stageIds.qa, release_approver_user_id: stageIds.release } });
  expect(within(panel()).getByText("Roles saved.").closest('[role="status"]')).not.toBeNull();
  expect(server.writes()).toHaveLength(1);
});

it("offers accept, modify-and-accept and reject as separate actions pinned to the newest version, and accepts exactly that reference", async () => {
  const server = serve(stageView(stageViewManagedFixture), proposalList(), proposalDetail(() => stageProposalDetailFixture),
    ({ method, url }) => method === "POST" && url.pathname === `${issuePath}/stage/proposals/${stageIds.proposal}/decisions`
      ? json({ code: "stage_proposal_accepted", collaboration_revision: "9007199254740994", stage_confirmation: stageConfirmations.decide_proposal }) : undefined);
  mount();
  const row = await within(panel()).findByRole("button", { name: "Open proposal: Split the refund flow (approver edit)" });
  expect(row).toHaveAttribute("aria-pressed", "false");
  await userEvent.click(row);
  expect(row).toHaveAttribute("aria-pressed", "true");
  const record = await waitFor(() => { const node = document.querySelector<HTMLElement>(`[data-proposal-id="${stageIds.proposal}"]`); expect(node).not.toBeNull(); return node!; });
  expect(within(record).getAllByRole("button").map((button) => button.textContent)).toEqual(["Accept", "Modify and accept", "Reject", "Revise"]);
  expect(within(record).getByText("From an agent comment")).toBeVisible();
  expect(within(record).getByRole("link", { name: "Open the source comment" })).toHaveAttribute("href", `/account?view=participating&issue=${stageIds.issue}#comment-${stageIds.comment}`);
  await userEvent.click(within(record).getByRole("button", { name: "Accept" }));
  const dialog = await screen.findByRole("dialog", { name: "Accept version 2" });
  expect(within(dialog).getByText("This decision applies to version 2 only. It is never moved to a later version.")).toBeVisible();
  expect(within(dialog).getByText("Accepting creates 2 sub-issues in the source workspace, unassigned. Nothing is assigned or started.")).toBeVisible();
  const accept = within(dialog).getByRole("button", { name: "Accept version 2" });
  expect(accept).toBeDisabled();
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I reviewed version 2 as shown here" }));
  await userEvent.click(accept);
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  const [write] = server.writes();
  expect(write!.body).toEqual({ ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1",
    candidate: { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 }, decision: "accept", reason: "" });
  expect(within(panel()).getByText("Version accepted. Its sub-issues were created.")).toBeVisible();
});

it("shows a committed refusal with its original status and code, keeps the typed reason and does not treat it as lost access", async () => {
  const server = serve(stageView(stageViewManagedFixture), proposalList(), proposalDetail(() => stageProposalDetailFixture),
    ({ method }) => method === "POST" ? json({ code: "stage_role_required", error: "stage_role_required", status: 403, collaboration_revision: collaborationIssueFixture.collaboration_revision }, 403) : undefined);
  const { client, scope } = mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: /^Open proposal/ }));
  await userEvent.click(await within(panel()).findByRole("button", { name: "Reject" }));
  const dialog = await screen.findByRole("dialog", { name: "Reject version 2" });
  const reason = within(dialog).getByRole("textbox", { name: "Reason" });
  await userEvent.click(within(dialog).getByRole("button", { name: "Reject version 2" }));
  expect(within(dialog).getByText("Give a reason for rejecting.")).toBeVisible();
  expect(server.writes()).toEqual([]);
  await userEvent.type(reason, "Out of scope for this release");
  await userEvent.click(within(dialog).getByRole("button", { name: "Reject version 2" }));
  const alert = await within(dialog).findByRole("alert");
  expect(alert).toHaveTextContent("The server refused this request. Only the configured holder of this role can do this. (HTTP 403 · stage_role_required)");
  expect(alert).toHaveTextContent("Your input is kept. Review the current state before trying again.");
  expect(reason).toHaveValue("Out of scope for this release");
  expect(server.writes()[0]!.body).toMatchObject({ decision: "reject", reason: "Out of scope for this release" });
  expect(collaborationIssueAccessDenied(client, scope, stageIds.issue)).toBe(false);
  // A stored refusal is final for its key: nothing stays pending and nothing is resent.
  expect(readCollaborationStageIntents({ server: scope.server, userId: scope.userId }, { kind: "issue", issueId: stageIds.issue })).toEqual([]);
  expect(server.writes()).toHaveLength(1);
});

it("runs modify-and-accept as two explicit requests and accepts exactly the version it just saved", async () => {
  const candidate3 = { ...stageCandidateV2Fixture, candidate_id: "00000000-0000-4000-8000-000000000313", version: "3", content_digest: "f".repeat(64),
    parent_candidate_id: stageIds.candidate2, content: { ...stageCandidateV2Fixture.content, summary: "Approver wording" } };
  let saved = false;
  const server = serve(stageView(stageViewManagedFixture), proposalList(),
    proposalDetail(() => saved ? { ...stageProposalDetailFixture, versions: [candidate3, { ...stageCandidateV2Fixture, state: "superseded" }, stageCandidateV1Fixture] } : stageProposalDetailFixture),
    ({ method, url }) => {
      if (method !== "POST") return undefined;
      if (url.pathname.endsWith("/versions")) {
        saved = true;
        return json({ code: "stage_proposal_revised", collaboration_revision: collaborationIssueFixture.collaboration_revision, stage_confirmation: { ...stageConfirmations.revise_proposal,
          candidate_id: candidate3.candidate_id, version: "3", content_digest: candidate3.content_digest, parent_candidate_id: stageIds.candidate2 } });
      }
      return json({ code: "stage_proposal_accepted", collaboration_revision: collaborationIssueFixture.collaboration_revision, stage_confirmation: { ...stageConfirmations.decide_proposal,
        candidate_id: candidate3.candidate_id, version: "3", content_digest: candidate3.content_digest } });
    });
  mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: /^Open proposal/ }));
  await userEvent.click(await within(panel()).findByRole("button", { name: "Modify and accept" }));
  const first = await screen.findByRole("dialog", { name: "Modify and accept · step 1 of 2" });
  expect(within(first).getByText("Save your changes as a new version first. You then accept exactly that version in a separate step.")).toBeVisible();
  const summary = within(first).getByRole("textbox", { name: "Summary" });
  // Saving identical content is not offered.
  expect(within(first).getByRole("button", { name: "Save new version" })).toBeDisabled();
  await userEvent.clear(summary);
  await userEvent.type(summary, "Approver wording");
  await userEvent.click(within(first).getByRole("button", { name: "Save new version" }));
  const second = await screen.findByRole("dialog", { name: "Modify and accept · step 2 of 2" });
  // Saving decided nothing: only the revision was written so far.
  expect(server.writes().map((call) => call.path)).toEqual([`${issuePath}/stage/proposals/${stageIds.proposal}/versions`]);
  expect(server.writes()[0]!.body).toMatchObject({ expected_candidate: { candidate_id: stageIds.candidate2, version: "2", content_digest: stageDigests.content2 },
    content: { summary: "Approver wording" }, source: { kind: "manual" } });
  expect(await within(second).findByText("Approver wording")).toBeVisible();
  await userEvent.click(within(second).getByRole("checkbox", { name: "I reviewed version 3 as shown here" }));
  await userEvent.click(within(second).getByRole("button", { name: "Accept version 3" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(server.writes()).toHaveLength(2);
  expect(server.writes()[1]).toMatchObject({ path: `${issuePath}/stage/proposals/${stageIds.proposal}/decisions`,
    body: { candidate: { candidate_id: candidate3.candidate_id, version: "3", content_digest: candidate3.content_digest }, decision: "accept" } });
  expect(server.writes()[1]!.headers["Idempotency-Key"]).not.toBe(server.writes()[0]!.headers["Idempotency-Key"]);
});

it("keeps an unconfirmed request, recovers it by GET only and never writes again on its own", async () => {
  let posts = 0, receipt: "missing" | "incomplete" | "success" = "missing";
  const server = serve(stageView(stageViewNeedsConfirmationFixture), noProposals, get("/participants", () => participants),
    ({ method, url }) => {
      if (method === "POST") { posts += 1; throw new TypeError("Failed to fetch"); }
      if (url.pathname.startsWith(`${issuePath}/operations/`)) {
        return receipt === "missing" ? json({ code: "not_found", error: "not_found" }, 404) : receipt === "incomplete" ? json({ code: "operation_incomplete", error: "x" }, 409)
          : json({ code: "stage_roles_configured", collaboration_revision: "9007199254740994", stage_confirmation: stageConfirmations.configure_roles });
      }
      return undefined;
    });
  const { client, scope } = mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Configure roles" }));
  const dialog = await screen.findByRole("dialog", { name: "Configure roles" });
  const select = (name: string) => within(dialog).getByRole("combobox", { name });
  await within(select("Business owner")).findByRole("option", { name: "Olivia Owner" });
  for (const [name, value] of Object.entries({ "Business owner": stageIds.owner, Implementer: stageIds.implementer, "Product approver": stageIds.product,
    "Code reviewer": stageIds.reviewer, "QA approver": stageIds.qa, "Release approver": stageIds.release })) await userEvent.selectOptions(select(name), value);
  await userEvent.click(within(dialog).getByRole("button", { name: "Save roles" }));
  expect(await within(dialog).findByText("The result is not confirmed. The request is kept; check its original result.")).toBeVisible();
  const key = server.writes()[0]!.headers["Idempotency-Key"]!;
  // The form is locked by the unconfirmed request; closing the dialog leaves recovery in the panel.
  expect(within(dialog).getByRole("button", { name: "Save roles" })).toBeDisabled();
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  const region = panel();
  expect(within(region).getByText("Unconfirmed stage requests")).toBeVisible();
  expect(within(region).getByRole("button", { name: "Configure roles" })).toBeDisabled();
  expect(within(region).queryByRole("button", { name: "Discard local record" })).not.toBeInTheDocument();
  const check = within(region).getByRole("button", { name: "Check original result" });
  await waitFor(() => expect(check).toHaveFocus());
  await userEvent.click(check);
  expect(await within(region).findByText("The original result is still unknown. The request is kept.")).toBeVisible();
  receipt = "incomplete";
  await userEvent.click(check);
  expect(await within(region).findByText("The server has not finished this request yet. Check again shortly.")).toBeVisible();
  const recoveries = server.calls.filter((call) => call.path.startsWith(`${issuePath}/operations/`));
  expect(recoveries.map((call) => [call.method, call.path])).toEqual([["GET", `${issuePath}/operations/${key}`], ["GET", `${issuePath}/operations/${key}`]]);
  expect(posts).toBe(1);
  expect(readCollaborationStageIntents({ server: scope.server, userId: scope.userId }, { kind: "issue", issueId: stageIds.issue })).toHaveLength(1);
  // Only after a check settled nothing may the human discard the local record, with its caveat.
  expect(within(region).getByRole("button", { name: "Discard local record" })).toHaveAccessibleDescription(/does not cancel the request/);
  receipt = "success";
  await userEvent.click(check);
  expect(await within(region).findByText("Roles saved.")).toBeVisible();
  expect(within(region).queryByText("Unconfirmed stage requests")).not.toBeInTheDocument();
  expect(posts).toBe(1);
  expect(collaborationIssueAccessDenied(client, scope, stageIds.issue)).toBe(false);
});

const scopeView = { ...stageViewNeedsConfirmationFixture, missing: ["scope"], role_revision: "1", roles: stageRolesFixture,
  capabilities: { ...stageViewNeedsConfirmationFixture.capabilities, confirm_scope: true } };

it("confirms explicit none and an observed full code head for a descendant in one A5 request", async () => {
  const descendant = { ...stageChildFixture, issue_id: stageIds.child2, identifier: "SRC-13", title: "Nested refund child", depth: 2 };
  const view = { ...scopeView, children: { items: [stageChildFixture, descendant], truncated: false } };
  const server = serve(stageView(view), noProposals, ({ method, url }) => method === "POST" && url.pathname === `${issuePath}/stage/scope-confirmations`
    ? json({ code: "stage_scope_confirmed", collaboration_revision: "9007199254740994", stage_confirmation: stageConfirmations.confirm_scope }) : undefined);
  mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Confirm existing sub-issues" }));
  const dialog = await screen.findByRole("dialog", { name: "Confirm existing sub-issues" });
  await userEvent.click(within(dialog).getByRole("checkbox", { name: /SRC-12 Existing checkout child/ }));
  await userEvent.click(within(dialog).getByRole("checkbox", { name: /SRC-13 Nested refund child/ }));
  const submit = within(dialog).getByRole("button", { name: "Confirm 2 sub-issues" });
  expect(submit).toBeDisabled();
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Code version for SRC-12" }), "none");
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Code version for SRC-13" }), JSON.stringify(stagePullRequestFixture));
  expect(within(dialog).getByText("Sub-issue depth 2")).toBeVisible();
  await userEvent.click(submit);
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(server.writes()).toHaveLength(1);
  expect(server.writes()[0]).toMatchObject({ method: "POST", path: `${issuePath}/stage/scope-confirmations`, body: {
    ...stageObservationsFixture, expected_policy_revision: "1", expected_role_revision: "1", expected_scope_revision: "0", reason: "",
    children: [{ issue_id: stageIds.child1, content_digest: stageDigests.child1, code: null },
      { issue_id: stageIds.child2, content_digest: stageDigests.child1, code: { pull_request: stagePullRequestFixture } }],
  } });
});

it("keeps the selected code head when the view refreshes and requires reopening after it changes", async () => {
  let view = scopeView;
  const server = serve(get("/stage", () => view), noProposals);
  const { client } = mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Confirm existing sub-issues" }));
  const dialog = await screen.findByRole("dialog", { name: "Confirm existing sub-issues" });
  await userEvent.click(within(dialog).getByRole("checkbox", { name: /SRC-12 Existing checkout child/ }));
  const select = within(dialog).getByRole("combobox", { name: "Code version for SRC-12" });
  await userEvent.selectOptions(select, JSON.stringify(stagePullRequestFixture));
  view = { ...scopeView, children: { items: [{ ...stageChildFixture, pull_requests: [{ ...stagePullRequestFixture, head_sha: "b".repeat(40) }] }], truncated: false } };
  await client.refetchQueries();
  expect(await within(dialog).findByText("The selected scope or code changed. Close this form and review the current versions again.")).toBeVisible();
  expect(select).toHaveValue(JSON.stringify(stagePullRequestFixture));
  expect(within(dialog).getByRole("button", { name: "Confirm 1 sub-issue" })).toBeDisabled();
  expect(server.writes()).toEqual([]);
});

it("reads the project policy for the source workspace and does not offer enabling until activation is exactly available", async () => {
  const server = serve(stageView(stageViewUnmanagedFixture), ({ method, url }) => method === "GET" && url.pathname === `/api/projects/${stageIds.project}/stage-policy` ? json(stagePolicyVirtualFixture) : undefined);
  mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Project stage policy" }));
  const policy = await within(panel()).findByRole("region", { name: "Project stage policy" });
  expect(await within(policy).findByText("Never set")).toBeVisible();
  expect(within(policy).getByRole("status")).toHaveTextContent("Enabling stage gates is not available yet on this server.");
  expect(within(policy).queryByRole("button", { name: "Enable stage gates" })).not.toBeInTheDocument();
  expect(within(policy).queryByRole("textbox")).not.toBeInTheDocument();
  const read = server.calls.find((call) => call.path === `/api/projects/${stageIds.project}/stage-policy`)!;
  expect(read.headers["X-Workspace-ID"]).toBe(stageIds.workspace);
  expect(server.writes()).toEqual([]);
});

it("renders the temporary 503 on enabling as not available yet, not as an error to retry, and keeps nothing pending", async () => {
  const off = { ...stagePolicyFixture, policy_revision: "2", enabled: false, activation_available: true };
  const server = serve(stageView(stageViewUnmanagedFixture), ({ method, url }) => {
    if (url.pathname !== `/api/projects/${stageIds.project}/stage-policy`) return undefined;
    return method === "GET" ? json(off) : json({ code: "stage_policy_unavailable", error: "stage_policy_unavailable" }, 503);
  });
  const { scope } = mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Project stage policy" }));
  const policy = await within(panel()).findByRole("region", { name: "Project stage policy" });
  const enable = await within(policy).findByRole("button", { name: "Enable stage gates" });
  await userEvent.click(enable);
  expect(within(policy).getByText("Give a reason of 1 to 1,000 characters.")).toBeVisible();
  expect(server.writes()).toEqual([]);
  await userEvent.type(within(policy).getByRole("textbox", { name: "Reason" }), "Start the pilot");
  await userEvent.click(enable);
  const status = await within(panel()).findByText("Nothing was saved. Enabling stage gates is not available yet on this server. (HTTP 503 · stage_policy_unavailable)");
  expect(status.closest('[role="status"]')).not.toBeNull();
  expect(within(panel()).queryByRole("alert")).not.toBeInTheDocument();
  const [write] = server.writes();
  expect(write).toMatchObject({ method: "PUT", path: `/api/projects/${stageIds.project}/stage-policy`, body: { expected_policy_revision: "2", enabled: true, reason: "Start the pilot" } });
  expect(write!.headers["X-Workspace-ID"]).toBe(stageIds.workspace);
  expect(readCollaborationStageIntents({ server: scope.server, userId: scope.userId }, { kind: "project", workspaceId: stageIds.workspace, projectId: stageIds.project })).toEqual([]);
  expect(within(policy).queryByText("Unconfirmed stage requests")).not.toBeInTheDocument();
  expect(server.writes()).toHaveLength(1);
});

it("C08 keeps explicit policy resend available without unlocking a replacement request", async () => {
  const policyPath = `/api/projects/${stageIds.project}/stage-policy`;
  let writes = 0, readFailed = false;
  const server = serve(stageView(stageViewUnmanagedFixture), ({ method, url }) => {
    if (method === "GET" && url.pathname.startsWith(`${policyPath}/operations/`)) return json({ code: "stage_policy_operation_not_found" }, 404);
    if (url.pathname !== policyPath) return undefined;
    if (method === "GET") return readFailed ? json({ error: "unavailable" }, 500)
      : json({ ...stagePolicyFixture, policy_revision: "2", enabled: false });
    writes += 1;
    if (writes === 1) throw new TypeError("Lost policy response");
    return json({ code: "stage_policy_revised", stage_confirmation: { action: "revise_policy", project_id: stageIds.project, policy_revision: "3", enabled: true } });
  });
  const { client, scope } = mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Project stage policy" }));
  const policy = await within(panel()).findByRole("region", { name: "Project stage policy" });
  const enable = await within(policy).findByRole("button", { name: "Enable stage gates" });
  await userEvent.type(within(policy).getByRole("textbox", { name: "Reason" }), "Enable this project");
  await userEvent.click(enable);
  const resend = await within(policy).findByRole("button", { name: "Send the same request again" });
  await waitFor(() => expect(resend).toBeEnabled());
  expect(enable).toBeDisabled();
  await userEvent.click(within(policy).getByRole("button", { name: "Check original result" }));
  await waitFor(() => expect(resend).toBeEnabled());
  expect(server.writes()).toHaveLength(1);
  readFailed = true;
  await client.refetchQueries({ predicate: (query) => query.queryKey.includes("policy") });
  await waitFor(() => expect(resend).toBeDisabled());
  expect(within(policy).getByRole("button", { name: "Check original result" })).toBeEnabled();
  readFailed = false;
  await client.refetchQueries({ predicate: (query) => query.queryKey.includes("policy") });
  await waitFor(() => expect(resend).toBeEnabled());
  await userEvent.click(resend);
  await waitFor(() => expect(readCollaborationStageIntents(scope, { kind: "project", workspaceId: stageIds.workspace, projectId: stageIds.project })).toEqual([]));
  const attempts = server.writes();
  expect(attempts).toHaveLength(2);
  expect(attempts[1]).toMatchObject({
    method: attempts[0]!.method,
    path: attempts[0]!.path,
    body: attempts[0]!.body,
    headers: { "Idempotency-Key": attempts[0]!.headers["Idempotency-Key"], "X-Workspace-ID": stageIds.workspace },
  });
});

it("lists an accepted proposal's decision and its created sub-issues from the authorized detail, with no further decision offered", async () => {
  serve(stageView(stageViewManagedFixture), proposalList({ ...stageProposalPageFixture, items: [{ ...stageProposalPageFixture.items[0], state: "accepted" }] }), proposalDetail(() => stageProposalAcceptedDetailFixture));
  mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: /^Open proposal/ }));
  const record = await waitFor(() => { const node = document.querySelector<HTMLElement>(`[data-proposal-id="${stageIds.proposal}"]`); expect(node).not.toBeNull(); return node!; });
  expect(within(record).getByText("Accepted version 2 after the approver's edit")).toBeVisible();
  expect(within(record).getAllByText("Priya Product", { selector: "dd" }).length).toBeGreaterThan(0);
  expect(within(record).getByRole("link", { name: /SRC-13/ })).toHaveAttribute("href", `/source/issues/${stageIds.child2}`);
  expect(within(record).queryByRole("button")).not.toBeInTheDocument();
});

it.each(["owner", "admin", "member", "unknown", "denied"] as const)("C05 policy write affordance requires current source authority: %s", async (role) => {
  serve(stageView(stageViewUnmanagedFixture), ({ url }) => {
    if (url.pathname === `/api/projects/${stageIds.project}/stage-policy`) return json({ ...stagePolicyFixture, enabled: false });
    if (url.pathname === `/api/workspaces/${stageIds.workspace}/members`) return role === "denied" ? json({ error: "forbidden" }, 403) : json([
      { id: stageIds.product, workspace_id: stageIds.workspace, user_id: stageIds.product, role }]);
    return undefined;
  });
  mount();await userEvent.click(await within(panel()).findByRole("button", { name: "Project stage policy" }));
  const policy = await within(panel()).findByRole("region", { name: "Project stage policy" });
  await within(policy).findByText("Disabled");
  if (role === "owner" || role === "admin") expect(await within(policy).findByRole("button", { name: "Enable stage gates" })).toBeEnabled();
  else expect(within(policy).queryByRole("button", { name: "Enable stage gates" })).not.toBeInTheDocument();
});
it("C06 failed R1 refresh disables a retained roles form until its next successful read", async () => {
  let failed = false;
  serve(({ url }) => url.pathname === `${issuePath}/stage` ? failed ? json({ error: "unavailable" }, 500) : json(stageViewManagedFixture) : undefined,
    noProposals, get("/participants", () => participants));
  const { client } = mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: "Change roles" }));
  const dialog = await screen.findByRole("dialog", { name: "Change roles" });
  const save = await within(dialog).findByRole("button", { name: "Save roles" });
  await waitFor(() => expect(save).toBeEnabled());
  failed = true;await client.refetchQueries({ predicate: (q) => q.queryKey.at(-1) === "view" });
  await waitFor(() => expect(save).toBeDisabled());expect(within(dialog).getByRole("combobox", { name: "Product approver" })).toHaveValue(stageIds.product);
  failed = false;await client.refetchQueries({ predicate: (q) => q.queryKey.at(-1) === "view" });
  await waitFor(() => expect(save).toBeEnabled());
});
it("C06 failed R3 refresh disables a retained proposal decision", async () => {
  let failed = false;
  serve(stageView(stageViewManagedFixture), proposalList(), ({ url }) => url.pathname === `${issuePath}/stage/proposals/${stageIds.proposal}`
    ? failed ? json({ error: "unavailable" }, 500) : json(stageProposalDetailFixture) : undefined);
  const { client } = mount();
  await userEvent.click(await within(panel()).findByRole("button", { name: /^Open proposal/ }));
  await userEvent.click(await within(panel()).findByRole("button", { name: "Accept" }));
  const dialog = await screen.findByRole("dialog", { name: "Accept version 2" });
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I reviewed version 2 as shown here" }));
  const save = within(dialog).getByRole("button", { name: "Accept version 2" });expect(save).toBeEnabled();
  failed = true;await client.refetchQueries({ predicate: (q) => q.queryKey.includes("proposal") });
  await waitFor(() => expect(save).toBeDisabled());expect(within(dialog).getByRole("checkbox")).toBeChecked();
  failed = false;await client.refetchQueries({ predicate: (q) => q.queryKey.includes("proposal") });
  await waitFor(() => expect(save).toBeEnabled());
});
it.each(["policy", "authority"] as const)("C06 failed %s refresh locks the retained policy form and recovers after a fresh read", async (failure) => {
  let failed = false;
  serve(stageView(stageViewUnmanagedFixture), ({ url }) => {
    if (url.pathname === `/api/projects/${stageIds.project}/stage-policy`) return failed && failure === "policy" ? json({ error: "unavailable" }, 500) : json({ ...stagePolicyFixture, enabled: false });
    if (url.pathname === `/api/workspaces/${stageIds.workspace}/members`) return failed && failure === "authority" ? json({ error: "unavailable" }, 500) : json([
      { id: stageIds.product, user_id: stageIds.product, workspace_id: stageIds.workspace, role: "owner" }]);
    return undefined;
  });
  const { client } = mount();await userEvent.click(await within(panel()).findByRole("button", { name: "Project stage policy" }));
  const policy = await within(panel()).findByRole("region", { name: "Project stage policy" });
  const save = await within(policy).findByRole("button", { name: "Enable stage gates" });
  await userEvent.type(within(policy).getByRole("textbox", { name: "Reason" }), "Retain policy draft");
  failed = true;await client.refetchQueries({ predicate: (q) => q.queryKey.includes(failure === "policy" ? "policy" : "stage-policy-authority") });
  await waitFor(() => expect(save).toBeDisabled());
  expect(within(policy).getByRole("textbox", { name: "Reason" })).toHaveValue("Retain policy draft");
  failed = false;await client.refetchQueries({ predicate: (q) => q.queryKey.includes(failure === "policy" ? "policy" : "stage-policy-authority") });
  await waitFor(() => expect(save).toBeEnabled());
});
