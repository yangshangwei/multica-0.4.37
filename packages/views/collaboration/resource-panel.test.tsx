import { afterEach, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationIssueOptions, collaborationKeys, collaborationScope, executeCollaborationResource, parseCollaborationIssue, readCollaborationDrafts } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationExecutionPanel } from "./execution-panel";
import { NavigationProvider } from "../navigation";
import { CollaborationResourcePanel } from "./resource-panel";

// Core resource/Switch suites own authority, malformed DTO and recovery matrices.
const source = collaborationExecutionIssueFixture;
const id = source.id;
const target = source.execution.targets[0]!;
const runtimeId = "00000000-0000-4000-8000-000000000099";
const resource = { role: "agent", agent_id: target.resource_id, runtime_id: runtimeId, owner_user_id: id, execution_workspace_id: id,
  agent_name: "Implementation agent", runtime: { id: runtimeId, name: "Runtime", custom_name: "Development", provider: "codex" }, owner: source.creator,
  consented: false, grant_id: null, grant_epoch: null, can_consent: true, can_revoke: false };
const resources = { protocol_version: 1, issue_id: id, lifecycle_epoch: source.lifecycle_epoch, target_set_version: source.execution.target_set_version, can_organize: true,
  targets: [{ target_id: target.id, kind: target.kind, resource_id: target.resource_id, name: "Implementation agent", target_epoch: target.target_epoch, config_version: target.config_version,
    authorized: false, reason: "grant_required", resources: [resource] }] };
const identity = { issue_id: id, target_id: target.id, agent_id: resource.agent_id, proposal_id: null,
  identity: { kind: "enrolled", installation_id: id, binding_id: runtimeId, binding_epoch: "9007199254740993" }, reason: "" };
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
function mount() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const issue = parseCollaborationIssue({ ...source, write_context: { participant_epoch: "1", account_epoch: "1" } })!;
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => `https://server.example${path}` }}><CollaborationExecutionPanel scope={scope} issue={issue} /></NavigationProvider></QueryClientProvider>);
  return { scope };
}

it("confirms the real owner's resource and recovers a lost response without resuming", async () => {
  let requestId = "";
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith("/resources")) return json(resources);
    if (url.includes("/resources/identity?")) return json(identity);
    if (url.includes("/resources/operations/")) return json({ protocol_version: 2, issue_id: id, request_id: requestId, action: "consent_resource", status: 201, code: "resource_consented", scope: "target", target_id: target.id, grant_id: id });
    if (url.endsWith("/consents")) { requestId = new Headers(init.headers).get("Idempotency-Key")!; throw new TypeError("Lost response"); }
    throw new Error(`Unexpected URL ${url}`);
  });
  vi.stubGlobal("fetch", fetch);
  const { scope } = mount();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Authorize Implementation agent" }));
  await user.click(await screen.findByRole("button", { name: "Confirm authorization" }));
  expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Check original result" }));
  expect(await screen.findByText("Resource authorized. Explicit resume is still required.")).toBeInTheDocument();
  await waitFor(() => expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]));
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(1);
  expect(fetch.mock.calls.some(([url]) => url.endsWith("/control"))).toBe(false);
});

it("registers only a selected candidate from the explicit workspace", async () => {
  const newAgent = "00000000-0000-4000-8000-000000000098";
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith("/resources")) return json(resources);
    if (url.endsWith("/api/workspaces")) return json([{ id, name: "Resource workspace", slug: "resources", role: "owner" }]);
    if (url.includes("/resources/options?")) return json({ protocol_version: 1, issue_id: id, execution_workspace_id: id,
      items: [{ kind: "agent", resource_id: newAgent, name: "New agent", available: true, reason: "", resources: [{ ...resource, agent_id: newAgent }] }] });
    if (url.endsWith("/targets")) return json({ protocol_version: 2, issue_id: id, request_id: new Headers(init.headers).get("Idempotency-Key"), action: "register_target", status: 201, code: "target_registered", scope: "target", target_id: newAgent, grant_id: null }, 201);
    throw new Error(`Unexpected URL ${url}`);
  });
  vi.stubGlobal("fetch", fetch);
  mount();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Manage AI resources" }));
  await user.click(await screen.findByRole("button", { name: "Add AI target" }));
  await user.selectOptions(await screen.findByLabelText("Resource workspace"), id);
  await user.selectOptions(await screen.findByLabelText("AI target"), `agent:${newAgent}`);
  await user.click(screen.getByRole("button", { name: "Register target" }));
  expect(await screen.findByText("Target registered. Each owner must authorize their resources.")).toBeInTheDocument();
  const post = fetch.mock.calls.find(([, init]) => init.method === "POST");
  expect(JSON.parse(post?.[1]?.body as string)).toMatchObject({ resource_id: newAgent, expected_target_set_version: source.execution.target_set_version });
});

it("commits an authorized Switch proposal only after reviewing the explicit resume requirement", async () => {
  const proposalId = "00000000-0000-4000-8000-000000000097";
  const proposal = { proposal_id: proposalId, target_id: target.id, proposed_by: id, lifecycle_epoch: "1",
    base_target_epoch: target.target_epoch, base_config_version: target.config_version,
    proposed_target_epoch: (BigInt(target.target_epoch) + 1n).toString(), proposed_config_version: (BigInt(target.config_version) + 1n).toString(),
    status: "consent_ready", reason: "", resources: [{ role: "agent", agent_id: resource.agent_id, runtime_id: runtimeId,
      execution_workspace_id: id, owner_user_id: id, consented: true, revoked: false, grant_id: null, grant_epoch: null }] };
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith("/switch-proposals")) return json([proposal]);
    if (url.endsWith("/commit")) return json({ protocol_version: 2, issue_id: id, request_id: new Headers(init.headers).get("Idempotency-Key"), action: "commit_target_switch", status: 200, code: "target_switched",
      scope: "target", target_id: target.id, proposal_id: proposalId, channel_id: target.channel_id, queue_state: "paused", control_version: target.control.control_version,
      target_set_version: source.execution.target_set_version, group_id: null, group_settled: false, grant_id: null, batch_id: null, command_id: null, run_id: null, run_settled: false, channels: [], target_holds: [] });
    throw new Error(`Unexpected URL ${url}`);
  });
  vi.stubGlobal("fetch", fetch);
  mount();
  const user = userEvent.setup();
  await user.click(screen.getAllByRole("button", { name: "Switch proposals" })[0]!);
  await user.click(await screen.findByRole("button", { name: "Commit Switch" }));
  expect(screen.getByText(/The server will verify every owner consent/)).toBeInTheDocument();
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(0);
  await user.click(screen.getByRole("button", { name: "Apply configuration" }));
  expect(await screen.findByText("Configuration switched. Explicit resume is still required.")).toBeInTheDocument();
  expect(fetch.mock.calls.some(([url]) => url.endsWith("/control"))).toBe(false);
});

it("waits for coherent issue and resource observations before opening consent after another actor adopts", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const before = { ...source, write_context: { participant_epoch: "1", account_epoch: "1" }, execution: {
    ...source.execution, target_set_version: "1", channels: [], targets: [], commands: [], groups: [], runs: [],
  } };
  const currentTarget = { ...target, channel_id: null, target_epoch: "1", config_version: "1", group_generation: "0", active_group_id: null, hold_reason: "initial" };
  const after = { ...before, execution: { ...before.execution, target_set_version: "2", targets: [currentTarget] } };
  const currentResources = { ...resources, target_set_version: "2", targets: [{ ...resources.targets[0]!, target_epoch: "1", config_version: "1" }] };
  let adopted = false;
  let releaseIssue!: () => void;
  const issueReady = new Promise<void>((resolve) => { releaseIssue = resolve; });
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith(`/api/collaboration/issues/${id}`)) { if (adopted) await issueReady; return json(adopted ? after : before); }
    if (url.endsWith("/resources")) return json(adopted ? currentResources : { ...resources, target_set_version: "1", targets: [] });
    if (url.includes("/resources/offers?")) return json({ protocol_version: 1, issue_id: id, lifecycle_epoch: source.lifecycle_epoch, target_set_version: adopted ? "2" : "1", can_offer: true, items: [], next_cursor: null });
    if (url.includes("/resources/identity?")) return json(identity);
    if (url.endsWith("/consents")) return json({ protocol_version: 2, issue_id: id, request_id: new Headers(init.headers).get("Idempotency-Key"), action: "consent_resource", status: 201,
      code: "resource_consented", scope: "target", target_id: target.id, grant_id: id }, 201);
    throw new Error(`Unexpected URL ${url}`);
  });
  vi.stubGlobal("fetch", fetch);
  function LiveIssue() {
    const query = useQuery(collaborationIssueOptions(client, scope, id));
    return query.data ? <CollaborationResourcePanel scope={scope} issue={query.data} locked={false} submit={(draft) => executeCollaborationResource(client, scope, draft)} /> : null;
  }
  renderWithI18n(<QueryClientProvider client={client}><LiveIssue /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Manage AI resources" }));
  await screen.findByText("No AI targets have been registered.");
  adopted = true;
  const previousIssueReads = fetch.mock.calls.filter(([url]) => url.endsWith(`/api/collaboration/issues/${id}`)).length;
  await user.click(screen.getByRole("button", { name: "Refresh resources" }));
  const authorize = await screen.findByRole("button", { name: "Authorize Implementation agent" });
  expect(authorize).toBeDisabled();
  expect(fetch.mock.calls.filter(([url]) => url.endsWith(`/api/collaboration/issues/${id}`)).length).toBeGreaterThan(previousIssueReads);
  await user.click(authorize);
  expect(fetch.mock.calls.some(([url]) => url.includes("/resources/identity?"))).toBe(false);
  await act(async () => { releaseIssue(); });
  await waitFor(() => expect(authorize).toBeEnabled());
  await user.click(authorize);
  const confirm = await screen.findByRole("button", { name: "Confirm authorization" });
  await waitFor(() => expect(confirm).toBeEnabled());

  // A later configuration change keeps the open observation frozen. Explicit
  // refresh releases that unsubmitted selection and fetches a new matching pair.
  currentTarget.config_version = "2";
  currentResources.targets[0]!.config_version = "2";
  const reconfigured = parseCollaborationIssue(after);
  expect(reconfigured).not.toBeNull();
  await act(async () => {
    client.setQueryData(collaborationKeys.issue(scope, id), reconfigured!);
    await client.invalidateQueries({ queryKey: [...collaborationKeys.issue(scope, id), "resources"] });
  });
  await waitFor(() => expect(confirm).toBeDisabled());
  await user.click(screen.getByRole("button", { name: "Refresh resources" }));
  await waitFor(() => expect(screen.queryByRole("button", { name: "Confirm authorization" })).not.toBeInTheDocument());
  await waitFor(() => expect(authorize).toBeEnabled());
  await user.click(authorize);
  await user.click(await screen.findByRole("button", { name: "Confirm authorization" }));
  await waitFor(() => expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(1));
  const post = fetch.mock.calls.find(([, init]) => init.method === "POST")!;
  expect(post[0]).toContain(`/targets/${target.id}/consents`);
  expect(JSON.parse(post[1].body)).toMatchObject({ expected_target_epoch: "1", expected_config_version: "2", agent_id: resource.agent_id });
  expect(JSON.parse(post[1].body)).not.toHaveProperty("target_id");
  expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]);
});
