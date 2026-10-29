import { afterEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationKeys, collaborationScope, newCollaborationDeliveryForm, parseCollaborationIssue, saveCollaborationDeliveryRecord } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationWorkflow } from "./workflow";
import { CollaborationCreationPanel } from "./creation-form";
import { CollaborationPropertyInputs } from "./field-inputs";

// Field type/authority/recovery matrices are owned by core field-actions,
// field-queries and the Go collaboration_source_fields suites.
const id = "00000000-0000-4000-8000-000000000001";
const labelId = "00000000-0000-4000-8000-000000000002";
const propertyId = "00000000-0000-4000-8000-000000000003";
const projectId = "00000000-0000-4000-8000-000000000004";
const parentId = "00000000-0000-4000-8000-000000000005";
const iterationId = "00000000-0000-4000-8000-000000000006";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const sourceOptions = { projects: [{ id: projectId, name: "Shared project" }], parents: [{ id: parentId, name: "Source parent", identifier: "SRC-2" }], parents_truncated: false,
  iterations: [{ id: iterationId, name: "Delivery period", revision: "12", status: "planned" }], iterations_enabled: true };
const fields = { labels: [{ id: labelId, name: "Needs review", color: "#123456" }], properties: [{ id: propertyId, name: "Reviewed", type: "checkbox", icon: "", archived: false, config: {} }] };
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, start_date: null, due_date: null, labels: [], properties: {}, property_definitions: [],
  write_context: { participant_epoch: "1", account_epoch: "1" }, capabilities: { ...collaborationIssueFixture.capabilities, edit_source_structure: true },
  source_structure: { issue_revision: "19", project: null, parent_issue: null, iteration: null, stage: null } })!;
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const options = { ...fields, issue_id: id, source_workspace_id: id, collaboration_revision: issue.collaboration_revision, source_options: sourceOptions };
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
function context() {
  setApiInstance(new ApiClient("https://example.invalid"));
  const scope = collaborationScope(id);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  return { scope, client };
}

it("restores selected labels and false property input after a lost response and recovered refusal", async () => {
  const { scope, client } = context();
  const fetch = vi.fn(async (input: string, init?: RequestInit) => input.includes("field-options") ? json(options)
    : init?.method === "PATCH" ? Promise.reject(new TypeError("Response lost"))
      : input.includes("/operations/") ? json({ status: 409, code: "stale_revision", collaboration_revision: "9007199254740994" }) : json({ ...issue, execution: undefined }));
  vi.stubGlobal("fetch", fetch);
  const ui = (value: typeof issue) => <QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={value} /></QueryClientProvider>;
  const { rerender, unmount } = renderWithI18n(ui(issue));
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Edit issue" }));
  await user.click(await screen.findByRole("button", { name: "Labels" }));
  await user.click(screen.getByRole("button", { name: "Needs review" }));
  await user.keyboard("{Escape}");
  await user.click(screen.getByRole("button", { name: "Reviewed Empty" }));
  await user.click(screen.getByRole("button", { name: "No" }));
  rerender(ui({ ...issue, collaboration_revision: "9007199254740994" }));
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  await screen.findByRole("button", { name: "Check original result" });
  unmount();
  renderWithI18n(ui({ ...issue, collaboration_revision: "9007199254740994" }));
  await user.click(await screen.findByRole("button", { name: "Check original result" }));
  await screen.findByText(/The server did not apply this action/);
  const request = fetch.mock.calls.find(([, init]) => init?.method === "PATCH")?.[1];
  expect(JSON.parse(String(request?.body))).toMatchObject({ label_ids: [labelId], properties: { [propertyId]: false }, expected_collaboration_revision: issue.collaboration_revision });
  expect(screen.getByRole("button", { name: "Labels" })).toHaveTextContent("Needs review");
  expect(screen.getByRole("button", { name: "Reviewed No" })).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Keep input and use current version" }));
  expect(fetch.mock.calls.filter(([, init]) => init?.method === "PATCH")).toHaveLength(1);
});

it("submits source structure and iteration through their separate versioned actions", async () => {
  const { scope, client } = context();
  const fetch = vi.fn(async (input: string, init?: RequestInit) => input.includes("field-options") ? json(options)
    : init?.method === "PATCH" || init?.method === "PUT" ? json({ code: input.endsWith("/iteration") ? "iteration_changed" : "issue_edited", collaboration_revision: "9007199254740994" }) : json(issue));
  vi.stubGlobal("fetch", fetch);
  const view = renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Edit source fields" }));
  await user.selectOptions(await screen.findByLabelText("Project", { exact: true }), projectId);
  await user.selectOptions(screen.getByLabelText("Parent issue", { exact: true }), parentId);
  fireEvent.change(screen.getByLabelText("Stage under parent"), { target: { value: "2" } });
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  await screen.findByText("Action confirmed.");
  expect(JSON.parse(String(fetch.mock.calls.find(([, init]) => init?.method === "PATCH")?.[1]?.body))).toMatchObject({ project_id: projectId, parent_issue_id: parentId, stage: 2 });
  view.unmount();
  const assigned = { ...issue, source_structure: { ...issue.source_structure!, iteration: { id: parentId, name: "Current period", revision: "3", status: "active" as const } } };
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={assigned} /></QueryClientProvider>);
  await user.click(screen.getByRole("button", { name: "Change iteration" }));
  await user.selectOptions(await screen.findByLabelText("Iteration", { exact: true }), iterationId);
  expect(screen.getByRole("button", { name: "Confirm" })).toBeDisabled();
  await user.type(screen.getByLabelText("Reason for switching or leaving the iteration"), "Ready for the next delivery");
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  await waitFor(() => expect(fetch.mock.calls.some(([, init]) => init?.method === "PUT")).toBe(true));
  expect(JSON.parse(String(fetch.mock.calls.find(([, init]) => init?.method === "PUT")?.[1]?.body))).toMatchObject({ current_iteration_id: iterationId, expected_revision: "19", expected_iteration_revision: "12", iteration_reason: "Ready for the next delivery" });
});

it("does not offer source editors or request source catalogs for an external collaborator", () => {
  const { scope, client } = context();
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={{ ...issue, capabilities: { ...issue.capabilities, edit_source_structure: false } }} /></QueryClientProvider>);
  expect(screen.queryByRole("button", { name: "Edit source fields" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Change iteration" })).toBeNull();
  expect(fetch).not.toHaveBeenCalled();
});

it("creates initial fields once and keeps the source choices after remounting the draft", async () => {
  const { scope, client } = context();
  const source = { id, name: "Source", slug: "source" };
  const fetch = vi.fn(async (input: string, init?: RequestInit) => input.includes("creation-options") ? json({ creation_enabled: true, sources: [source], source: input.includes("source_workspace_id") ? source : null, max_attachment_bytes: 104857600,
    ...(input.includes("source_workspace_id") ? { fields: { ...fields, statuses: [{ key: "todo", name: "Todo", category: "todo" }, { key: "scheduled", name: "Scheduled", category: "backlog" }], default_status: "todo", source_options: sourceOptions } } : {}) }) : init?.method === "POST" ? json({ ...issue, execution: undefined }) : json({ draft_id: input.split("/").pop(), attachments: [] }));
  vi.stubGlobal("fetch", fetch);
  const onCreated = vi.fn();
  const ui = () => <QueryClientProvider client={client}><CollaborationCreationPanel scope={scope} onCreated={onCreated} /></QueryClientProvider>;
  const view = renderWithI18n(ui());
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("Title", { exact: true }), "Initial collaboration");
  await user.click(screen.getByText("Initial fields", { exact: true }));
  await user.selectOptions(screen.getByLabelText("Status", { exact: true }), "scheduled");
  await user.selectOptions(screen.getByLabelText("Priority", { exact: true }), "urgent");
  await user.selectOptions(screen.getByLabelText("Project", { exact: true }), projectId);
  await user.selectOptions(screen.getByLabelText("Iteration", { exact: true }), iterationId);
  view.unmount();
  renderWithI18n(ui());
  await user.click(await screen.findByText("Initial fields", { exact: true }));
  expect(screen.getByLabelText("Project", { exact: true })).toHaveValue(projectId);
  expect(screen.getByLabelText("Iteration", { exact: true })).toHaveValue(iterationId);
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  const writes = fetch.mock.calls.filter(([, init]) => init?.method === "POST");
  expect(writes).toHaveLength(1);
  expect(JSON.parse(String(writes[0]?.[1]?.body))).toMatchObject({ title: "Initial collaboration", status: "scheduled", priority: "urgent", project_id: projectId, current_iteration_id: iterationId, expected_iteration_revision: "12" });
});

it("allows clearing obsolete initial choices when the source changes", async () => {
  const { scope, client } = context();
  const source = { id, name: "Source", slug: "source" };
  const draft = newCollaborationDeliveryForm({ ...scope, context: { kind: "creation", sourceWorkspaceId: id } }, "creation");
  saveCollaborationDeliveryRecord({ ...draft, title: "Keep my draft", initialFields: { properties: { [propertyId]: true }, current_iteration_id: iterationId, expected_iteration_revision: "12" } });
  vi.stubGlobal("fetch", vi.fn(async (input: string) => input.includes("creation-options") ? json({ creation_enabled: true, sources: [source], source: input.includes("source_workspace_id") ? source : null, max_attachment_bytes: 104857600,
    ...(input.includes("source_workspace_id") ? { fields: { labels: [], properties: [], statuses: [{ key: "todo", name: "Todo", category: "todo" }], default_status: "todo", source_options: { ...sourceOptions, iterations: [], iterations_enabled: false } } } : {}) }) : json({ draft_id: input.split("/").pop(), attachments: [] })));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCreationPanel scope={scope} onCreated={vi.fn()} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(await screen.findByText("Initial fields"));
  expect(screen.getByRole("button", { name: "Create collaboration issue" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Clear" }));
  await user.selectOptions(screen.getByLabelText("Iteration"), "");
  expect(screen.getByLabelText("Title")).toHaveValue("Keep my draft");
  expect(screen.getByRole("button", { name: "Create collaboration issue" })).toBeEnabled();
});

it("uses an exact confirmed source account for actor fields without fetching a member directory", async () => {
  const { scope, client } = context();
  const fetch = vi.fn<(input: string, init?: RequestInit) => Promise<Response>>(async () => json({ issue_id: id, source_workspace_id: id, recipient: { user_id: parentId, name: "Source reviewer", avatar_url: "", external: false } }));
  vi.stubGlobal("fetch", fetch);
  const onChange = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationPropertyInputs scope={scope} issueId={id} sourceId={id} disabled={false} values={{}}
    definitions={[{ id: propertyId, name: "Reviewer", type: "actor", archived: false, config: {}, icon: "" }]} onChange={onChange} /></QueryClientProvider>);
  expect(fetch).not.toHaveBeenCalled();
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Exact source member account"), "reviewer@example.test");
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await screen.findByText("Source reviewer");
  expect(onChange).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Use this account" }));
  expect(onChange).toHaveBeenCalledWith(propertyId, `member:${parentId}`);
  expect(fetch).toHaveBeenCalledOnce();
  expect(fetch.mock.calls[0]?.[0]).toContain("/collaboration/recipients/resolve");
});

it("keeps a property draft during option refresh and does not submit the outer form on property Enter", async () => {
  const { scope, client } = context();
  const textOptions = { ...options, properties: [{ ...fields.properties[0]!, type: "text", name: "Notes" }] };
  let finish!: (value: Response) => void;
  const fetch = vi.fn().mockResolvedValueOnce(json(textOptions)).mockImplementationOnce(() => new Promise<Response>((resolve) => { finish = resolve; }));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={issue} initialAction="edit" /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Notes Empty" }));
  await user.type(screen.getByPlaceholderText(/Enter value/), "Keep this property draft");
  let refreshing!: Promise<void>;
  await act(async () => { refreshing = client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, id) }); });
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(screen.getByPlaceholderText(/Enter value/)).toHaveValue("Keep this property draft");
  await act(async () => { finish(json(textOptions)); await refreshing; });
  await user.type(screen.getByPlaceholderText(/Enter value/), "{Enter}");
  expect(screen.getByRole("button", { name: "Notes Keep this property draft" })).toBeInTheDocument();
  expect(fetch.mock.calls.every(([, init]) => init?.method !== "PATCH")).toBe(true);
});
