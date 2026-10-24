import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationKeys, collaborationScope, parseCollaborationIssue, readCollaborationDrafts, type CollaborationIssue } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationExecutionPanel } from "./execution-panel";

// Core resource/offer schemas and helpers own the authority, roster and recovery matrices.
// These tests exercise source-hidden owner -> organizer -> independent owner UI wiring.
const uuid = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const source = collaborationExecutionIssueFixture;
const id = source.id;
const organizer = id, supplier = uuid(2), coOwner = uuid(3), workspaceId = uuid(4), squadId = uuid(5), offerId = uuid(6), targetId = uuid(7);
const account = (userId: string) => ({ user_id: userId, name: userId === supplier ? "Private owner" : userId === coOwner ? "Second owner" : "Organizer", avatar_url: "", external: userId !== organizer });
const roster = [
  { role: "leader", agent_id: uuid(10), runtime_id: uuid(11), execution_workspace_id: workspaceId, owner_user_id: supplier, agent_name: "Private leader", runtime: { id: uuid(11), name: "Runtime", custom_name: "Private machine", provider: "codex" }, owner: account(supplier) },
  { role: "member", agent_id: uuid(12), runtime_id: uuid(13), execution_workspace_id: workspaceId, owner_user_id: coOwner, agent_name: "Co-owned member", runtime: { id: uuid(13), name: "Runtime", custom_name: "Second machine", provider: "codex" }, owner: account(coOwner) },
];
const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => auth } }));
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); auth.user.id = organizer; });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });

function harness(initiallyOffered = false) {
  const state = { offered: initiallyOffered, adopted: false, withdrawn: false, closed: false, participantEpoch: "1", hiddenInventory: false, staleAdopt: false, lostWithdraw: false, withdrawRequest: "", grants: new Set<string>() };
  const canOrganize = () => [organizer, coOwner].includes(auth.user.id);
  const resourceView = () => roster.map((resource) => ({ ...resource, consented: state.grants.has(resource.agent_id),
    grant_id: state.grants.has(resource.agent_id) && auth.user.id === resource.owner_user_id ? uuid(20) : null,
    grant_epoch: state.grants.has(resource.agent_id) && auth.user.id === resource.owner_user_id ? "1" : null,
    can_consent: state.adopted && auth.user.id === resource.owner_user_id && !state.grants.has(resource.agent_id), can_revoke: state.grants.has(resource.agent_id) && auth.user.id === resource.owner_user_id }));
  const offer = () => ({ protocol_version: 1, issue_id: id, offer_id: offerId, version: state.adopted || state.withdrawn ? "2" : "1",
    state: state.adopted ? "adopted" : state.withdrawn ? "withdrawn" : "offered", offered_by: account(supplier), kind: "squad", resource_id: squadId,
    execution_workspace_id: workspaceId, name: "Private squad", resources: roster, created_at: "2026-10-21T01:00:00Z",
    withdrawn_at: state.withdrawn ? "2026-10-21T02:00:00Z" : null, adopted_at: state.adopted ? "2026-10-21T02:00:00Z" : null,
    adopted_by: state.adopted ? account(organizer) : null, target_id: state.adopted ? targetId : null,
    available: !state.closed && !state.adopted && !state.withdrawn, reason: state.adopted ? "offer_adopted" : state.withdrawn ? "offer_withdrawn" : state.closed ? "issue_closed" : "",
    can_adopt: canOrganize() && !state.closed && !state.adopted && !state.withdrawn, can_withdraw: auth.user.id === supplier && !state.adopted && !state.withdrawn });
  const issue = () => parseCollaborationIssue({ ...source, creator: account(organizer), handler: account(coOwner), write_context: { participant_epoch: state.participantEpoch, account_epoch: "1" },
    status: state.closed ? "done" : source.status, status_category: state.closed ? "done" : source.status_category, session: { ...source.session, queue_state: state.closed ? "closed" : "paused" },
    capabilities: { ...source.capabilities, invite: canOrganize() }, execution: { ...source.execution, control: { ...source.execution.control, queue_state: state.closed ? "closed" : "paused" }, channels: [], commands: [], groups: [], runs: [],
      targets: state.adopted ? [{ id: targetId, channel_id: null, kind: "squad", resource_id: squadId, target_epoch: "1", config_version: "1", hold_reason: "initial", hold_version: "1",
        control: { scope: "target", queue_state: "paused", control_version: "1" }, group_generation: "0", active_group_id: null, pause_reason: "initial", revoked_at: null }] : [] } })!;
  const receipt = (requestId: string, action: string, code: string, status = 201) => ({ protocol_version: 2, issue_id: id, request_id: requestId, action, status, code,
    scope: action === "adopt_resource_offer" && status < 400 || action === "consent_resource" ? "target" : "issue",
    target_id: action === "adopt_resource_offer" && status < 400 || action === "consent_resource" ? targetId : null,
    grant_id: action === "consent_resource" ? uuid(20) : null,
    ...(action === "consent_resource" ? {} : { offer_id: offerId, offer_version: action === "offer_resource" ? "1" : "2" }) });
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith("/resources")) return json({ protocol_version: 1, issue_id: id, lifecycle_epoch: "1", target_set_version: source.execution.target_set_version,
      can_organize: canOrganize() && !state.closed, targets: state.adopted ? [{ target_id: targetId, kind: "squad", resource_id: squadId, name: "Private squad", target_epoch: "1", config_version: "1",
        authorized: state.grants.size === 2, reason: state.grants.size === 2 ? "" : "grant_required", resources: resourceView() }] : [] });
    if (url.includes("/resources/offers?")) return json({ protocol_version: 1, issue_id: id, lifecycle_epoch: "1", target_set_version: source.execution.target_set_version, can_offer: !state.closed, items: state.offered ? [offer()] : [], next_cursor: null });
    if (url.endsWith(`/resources/offers/${offerId}`)) return json(offer());
    if (url.endsWith("/api/workspaces")) return json([{ id: auth.user.id === organizer ? id : workspaceId, name: auth.user.id === organizer ? "Source workspace" : "Private workspace", slug: "resources", role: "owner" }]);
    if (url.includes("/resources/offers/options?")) return json({ protocol_version: 1, issue_id: id, execution_workspace_id: workspaceId,
      items: state.hiddenInventory ? [] : [{ kind: "squad", resource_id: squadId, name: "Private squad", available: true, reason: "", resources: resourceView() }] });
    if (url.includes("/resources/identity?")) {
      const agentId = new URL(url).searchParams.get("agent_id");
      return json({ issue_id: id, target_id: targetId, agent_id: agentId, proposal_id: null, identity: { kind: "enrolled", installation_id: uuid(30), binding_id: uuid(31), binding_epoch: "1" }, reason: "" });
    }
    if (url.includes("/resources/operations/")) return json(receipt(state.withdrawRequest, "withdraw_resource_offer", "resource_offer_withdrawn", 200));
    const requestId = new Headers(init.headers).get("Idempotency-Key")!;
    if (url.endsWith("/resources/offers") && init.method === "POST") {
      if (JSON.parse(String(init.body)).expected_participant_epoch !== state.participantEpoch) return json({ ...receipt(requestId, "offer_resource", "stale_authority", 409), offer_id: undefined, offer_version: undefined }, 409);
      state.offered = true; return json(receipt(requestId, "offer_resource", "resource_offered"), 201);
    }
    if (url.endsWith(`/offers/${offerId}/adopt`)) {
      if (state.staleAdopt) { state.withdrawn = true; return json(receipt(requestId, "adopt_resource_offer", "stale_offer", 409), 409); }
      state.adopted = true;
      return json(receipt(requestId, "adopt_resource_offer", "resource_offer_adopted"), 201);
    }
    if (url.endsWith(`/offers/${offerId}/withdraw`)) {
      state.withdrawn = true; state.withdrawRequest = requestId;
      if (state.lostWithdraw) throw new TypeError("Lost response");
      return json(receipt(requestId, "withdraw_resource_offer", "resource_offer_withdrawn", 200));
    }
    if (url.endsWith("/consents")) { state.grants.add(JSON.parse(String(init.body)).agent_id); return json(receipt(requestId, "consent_resource", "resource_consented"), 201); }
    throw new Error(`Unexpected URL ${url}`);
  });
  vi.stubGlobal("fetch", fetch);
  const mount = (viewerId: string) => {
    auth.user.id = viewerId;
    setApiInstance(new ApiClient("https://server.example"));
    const scope = collaborationScope(viewerId);
    const renderView = (currentIssue: CollaborationIssue) => <QueryClientProvider client={client}><CollaborationExecutionPanel scope={scope} issue={currentIssue} /></QueryClientProvider>;
    const rendered = renderWithI18n(renderView(issue()));
    return { ...rendered, scope, issue, rerenderIssue: (currentIssue: CollaborationIssue) => rendered.rerender(renderView(currentIssue)) };
  };
  return { state, fetch, mount };
}

it("lets an external owner supply a squad, an organizer adopt it, and each owner separately authorize", async () => {
  const { state, fetch, mount } = harness();
  const user = userEvent.setup();
  let view = mount(supplier);
  expect(view.issue().capabilities.submit_command).toBe(false);
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  expect(screen.queryByRole("button", { name: "Add AI target" })).not.toBeInTheDocument();
  await user.click(await screen.findByRole("button", { name: "Offer resource" }));
  const supplyForm = await screen.findByRole("form", { name: "Offer resource" });
  await user.selectOptions(within(supplyForm).getByLabelText("Resource workspace"), workspaceId);
  await user.selectOptions(await within(supplyForm).findByLabelText("AI target"), `squad:${squadId}`);
  await user.click(within(supplyForm).getByRole("button", { name: "Offer resource" }));
  expect(await screen.findByText("Resource offered.")).toBeInTheDocument();
  expect(state.adopted).toBe(false);
  expect(state.grants.size).toBe(0);
  view.unmount(); client.clear();

  view = mount(organizer);
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Adopt resource" }));
  const adoptForm = await screen.findByRole("form", { name: "Adopt resource" });
  expect(within(adoptForm).getByText(/each resource owner must authorize/i)).toBeInTheDocument();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(1);
  await waitFor(() => expect(within(adoptForm).getByRole("button", { name: "Adopt resource" })).toBeEnabled());
  await user.click(within(adoptForm).getByRole("button", { name: "Adopt resource" }));
  expect(await screen.findByText("Resource adopted. Each owner can now authorize their resource.")).toBeInTheDocument();
  expect(state.grants.size).toBe(0);
  expect(fetch.mock.calls.some(([url]) => url.includes("/resources/options?"))).toBe(false);
  view.unmount(); client.clear();

  for (const [owner, ownName, otherName] of [[supplier, "Private leader", "Co-owned member"], [coOwner, "Co-owned member", "Private leader"]]) {
    view = mount(owner!);
    await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
    await user.click(await screen.findByRole("button", { name: `Authorize ${ownName}` }));
    expect(screen.queryByRole("button", { name: `Authorize ${otherName}` })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Withdraw offer" })).not.toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: "Confirm authorization" }));
    expect(await screen.findByText("Resource authorized. Explicit resume is still required.")).toBeInTheDocument();
    expect(view.issue().execution?.targets[0]?.hold_reason).toBe("initial");
    view.unmount(); client.clear();
  }
  expect(state.grants.size).toBe(2);
  const writes = fetch.mock.calls.filter(([, init]) => init.method === "POST");
  expect(writes).toHaveLength(4);
  expect(writes.some(([url]) => /\/control|\/participants|\/targets$/.test(url))).toBe(false);
  expect(JSON.parse(writes[0]![1].body)).not.toHaveProperty("expected_target_set_version");
  expect(JSON.parse(writes[1]![1].body)).toMatchObject({ expected_offer_version: "1", expected_target_set_version: source.execution.target_set_version });
});

it("keeps the selected adoption after a stale refusal and requires a fresh selection", async () => {
  const { state, fetch, mount } = harness(true);
  state.staleAdopt = true;
  mount(organizer);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Adopt resource" }));
  const form = await screen.findByRole("form", { name: "Adopt resource" });
  const confirm = within(form).getByRole("button", { name: "Adopt resource" });
  await waitFor(() => expect(confirm).toBeEnabled());
  await user.click(confirm);
  expect(await screen.findByText("This resource offer has changed. Refresh it before confirming again.")).toBeInTheDocument();
  expect(confirm).toBeDisabled();
  expect(state.adopted).toBe(false);
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(1);
  expect(fetch.mock.calls.some(([url]) => url.includes("/operations/"))).toBe(false);
  await user.click(within(form).getByRole("button", { name: "Close" }));
  expect(screen.queryByRole("button", { name: "Adopt resource" })).not.toBeInTheDocument();
});

it("recovers a lost withdrawal using the original request without revoking a grant or stopping", async () => {
  const { state, fetch, mount } = harness(true);
  state.lostWithdraw = true;
  const { scope } = mount(supplier);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Withdraw offer" }));
  const form = await screen.findByRole("form", { name: "Withdraw offer" });
  const confirm = within(form).getByRole("button", { name: "Withdraw offer" });
  await waitFor(() => expect(confirm).toBeEnabled());
  await user.click(confirm);
  expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
  expect(readCollaborationDrafts({ ...scope, issueId: id })[0]?.requestId).toBe(state.withdrawRequest);
  await user.click(screen.getByRole("button", { name: "Check original result" }));
  expect(await screen.findByText("Resource offer withdrawn.")).toBeInTheDocument();
  await waitFor(() => expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]));
  const writes = fetch.mock.calls.filter(([, init]) => init.method === "POST");
  expect(writes).toHaveLength(1);
  expect(writes[0]?.[0]).toContain(`/offers/${offerId}/withdraw`);
  expect(fetch.mock.calls.some(([url]) => url.endsWith(`/resources/operations/${state.withdrawRequest}`))).toBe(true);
});

it("hides a selected inventory resource once a current refresh no longer discloses it", async () => {
  const { state, fetch, mount } = harness();
  const { scope } = mount(supplier);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Offer resource" }));
  const form = await screen.findByRole("form", { name: "Offer resource" });
  await user.selectOptions(within(form).getByLabelText("Resource workspace"), workspaceId);
  await user.selectOptions(await within(form).findByLabelText("AI target"), `squad:${squadId}`);
  expect(within(form).getByText(/Private leader/)).toBeInTheDocument();
  state.hiddenInventory = true;
  await client.invalidateQueries({ queryKey: [...collaborationKeys.issue(scope, id), "resource-offer-options"] });
  await waitFor(() => expect(within(form).queryByText(/Private leader/)).not.toBeInTheDocument());
  expect(within(form).getByRole("button", { name: "Offer resource" })).toBeDisabled();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(0);
});

it("drops another owner's warm offer details when the viewer loses organization rights", async () => {
  const { fetch, mount } = harness(true);
  const view = mount(organizer);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Adopt resource" }));
  await screen.findByRole("form", { name: "Adopt resource" });
  const issue = view.issue();
  view.rerenderIssue({ ...issue, creator: account(coOwner), capabilities: { ...issue.capabilities, invite: false } });
  await waitFor(() => expect(screen.queryByText("Private squad")).not.toBeInTheDocument());
  expect(screen.queryByRole("form", { name: "Adopt resource" })).not.toBeInTheDocument();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(0);
});

it("retains closed-issue offer history for its organizer and withdrawal for the supplying owner", async () => {
  const { state, mount } = harness(true);
  state.closed = true;
  const user = userEvent.setup();
  const organizerView = mount(organizer);
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  expect(await screen.findByText("Private squad")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Adopt resource" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Offer resource" })).not.toBeInTheDocument();
  organizerView.unmount(); client.clear();
  mount(supplier);
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Withdraw offer" }));
  const form = await screen.findByRole("form", { name: "Withdraw offer" });
  await waitFor(() => expect(within(form).getByRole("button", { name: "Withdraw offer" })).toBeEnabled());
  await user.click(within(form).getByRole("button", { name: "Withdraw offer" }));
  expect(await screen.findByText("Resource offer withdrawn.")).toBeInTheDocument();
});

it("adopts current authority only after explicitly refreshing and reselecting a refused supply", async () => {
  const { state, fetch, mount } = harness();
  const view = mount(supplier);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Offer resource" }));
  const form = await screen.findByRole("form", { name: "Offer resource" });
  await user.selectOptions(within(form).getByLabelText("Resource workspace"), workspaceId);
  await user.selectOptions(await within(form).findByLabelText("AI target"), `squad:${squadId}`);
  state.participantEpoch = "2";
  view.rerenderIssue(view.issue());
  await user.click(within(form).getByRole("button", { name: "Offer resource" }));
  expect(await within(form).findByRole("button", { name: "Refresh" })).toBeInTheDocument();
  expect(within(form).getByRole("button", { name: "Offer resource" })).toBeDisabled();
  expect(state.offered).toBe(false);
  await user.click(within(form).getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(within(form).getByLabelText("AI target")).toBeEnabled());
  await user.selectOptions(within(form).getByLabelText("AI target"), `squad:${squadId}`);
  await user.click(within(form).getByRole("button", { name: "Offer resource" }));
  expect(await screen.findByText("Resource offered.")).toBeInTheDocument();
  const writes = fetch.mock.calls.filter(([, init]) => init.method === "POST");
  expect(writes.map(([, init]) => JSON.parse(init.body).expected_participant_epoch)).toEqual(["1", "2"]);
  expect(new Headers(writes[0]![1].headers).get("Idempotency-Key")).not.toBe(new Headers(writes[1]![1].headers).get("Idempotency-Key"));
});

it("releases an unsubmitted adoption when the handler loses organizer authority without restoring it later", async () => {
  const { fetch, mount } = harness(true);
  const view = mount(coOwner);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Adopt resource" }));
  const form = await screen.findByRole("form", { name: "Adopt resource" });
  await waitFor(() => expect(within(form).getByRole("button", { name: "Adopt resource" })).toBeEnabled());
  const current = view.issue();
  view.rerenderIssue({ ...current, handler: account(organizer), capabilities: { ...current.capabilities, invite: false } });
  await waitFor(() => expect(screen.queryByRole("form", { name: "Adopt resource" })).not.toBeInTheDocument());
  expect(screen.queryByText("Private squad")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  await waitFor(() => expect(screen.getByRole("button", { name: "Offer resource" })).toBeEnabled());
  view.rerenderIssue(current);
  expect(screen.queryByRole("form", { name: "Adopt resource" })).not.toBeInTheDocument();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(0);
});

it("releases a hidden supply form on close so the owner can still withdraw their offered resource", async () => {
  const { state, fetch, mount } = harness(true);
  const view = mount(supplier);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Offer resource" }));
  await screen.findByRole("form", { name: "Offer resource" });
  state.closed = true;
  view.rerenderIssue(view.issue());
  await waitFor(() => expect(screen.queryByRole("form", { name: "Offer resource" })).not.toBeInTheDocument());
  await waitFor(() => expect(screen.getByRole("button", { name: "Withdraw offer" })).toBeEnabled());
  state.closed = false;
  view.rerenderIssue(view.issue());
  expect(screen.queryByRole("form", { name: "Offer resource" })).not.toBeInTheDocument();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(0);
});

it("preserves the original pending operation when organizer authority disappears during submission", async () => {
  const { fetch, mount } = harness(true);
  const originalFetch = fetch.getMockImplementation()!;
  let loseResponse!: () => void;
  const delayed = new Promise<void>((resolve) => { loseResponse = resolve; });
  fetch.mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith(`/offers/${offerId}/adopt`)) { await delayed; throw new TypeError("Lost response"); }
    return originalFetch(url, init);
  });
  const view = mount(coOwner);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Adopt resource" }));
  const form = await screen.findByRole("form", { name: "Adopt resource" });
  await waitFor(() => expect(within(form).getByRole("button", { name: "Adopt resource" })).toBeEnabled());
  await user.click(within(form).getByRole("button", { name: "Adopt resource" }));
  const original = readCollaborationDrafts({ ...view.scope, issueId: id });
  expect(original).toHaveLength(1);
  const current = view.issue();
  view.rerenderIssue({ ...current, handler: account(organizer), capabilities: { ...current.capabilities, invite: false } });
  loseResponse();
  expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
  expect(readCollaborationDrafts({ ...view.scope, issueId: id })).toEqual(original);
  expect(screen.getByRole("button", { name: "Check original result" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "Offer resource" })).toBeDisabled();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(1);
});
