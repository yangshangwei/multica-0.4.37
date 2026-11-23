import { afterEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, newCollaborationDeliveryForm, saveCollaborationDeliveryRecord } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationCreationPanel } from "./creation-form";

// Core information-queries/schema tests own policy correlation and omission.
// This test owns the retained draft and pending-project form wiring.
const actor = collaborationIssueFixture.id;
const source = "00000000-0000-4000-8000-000000000002";
const managedProject = "00000000-0000-4000-8000-000000000003";
const unmanagedProject = "00000000-0000-4000-8000-000000000004";
const sourceChoice = { id: source, name: "Source", slug: "source" };
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const json = (data: unknown) => new Response(JSON.stringify(data));
function options(projectId?: string) {
  const managed = projectId === managedProject;
  return { creation_enabled: true, sources: [sourceChoice], source: sourceChoice, max_attachment_bytes: 104857600,
    stage_policy_observation: { protocol_version: 1, state: projectId ? "known" : "not_selected", source_workspace_id: source,
      project_id: projectId ?? null, policy_revision: projectId ? managed ? "2" : "0" : null, managed: projectId ? managed : null },
    fields: { labels: [], properties: [], default_status: "todo", statuses: [{ key: "todo", name: "To do", category: "todo" },
      ...(!managed ? [{ key: "blocked", name: "Blocked", category: "blocked" }] : [])],
    source_options: { projects: [{ id: managedProject, name: "Managed project" }, { id: unmanagedProject, name: "Unmanaged project" }], parents: [], parents_truncated: false, iterations: [], iterations_enabled: false } } };
}

it("reads the persisted project and preserves the form while the next project's policy is pending", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  saveCollaborationDeliveryRecord({ ...newCollaborationDeliveryForm({ server: scope.server, userId: actor, context: { kind: "creation", sourceWorkspaceId: source } }, "creation"),
    title: "Keep this project draft", content: "Keep this description", initialFields: { project_id: managedProject, status: "blocked" } });
  let finish!: (value: Response) => void;
  const fetch = vi.fn(async (input: string, init?: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) {
      if (!url.searchParams.has("source_workspace_id")) return json({ ...options(), source: null, fields: undefined,
        stage_policy_observation: { ...options().stage_policy_observation, source_workspace_id: null } });
      if (url.searchParams.get("project_id") === unmanagedProject) return new Promise<Response>((resolve) => { finish = resolve; });
      return json(options(url.searchParams.get("project_id") ?? undefined));
    }
    if (url.pathname === "/api/collaboration/issues" && init?.method === "POST") return json({ ...collaborationIssueFixture, source_workspace: { ...collaborationIssueFixture.source_workspace, id: source } });
    throw new Error(`Unexpected project test path ${url.pathname}`);
  }); vi.stubGlobal("fetch", fetch);
  const onCreated = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCreationPanel scope={scope} sourceWorkspaceId={source} onCreated={onCreated} /></QueryClientProvider>);
  const title = await screen.findByLabelText("Title");
  expect(title).toHaveValue("Keep this project draft");
  expect(fetch.mock.calls.some(([url]) => new URL(url).searchParams.get("project_id") === managedProject)).toBe(true);
  const submit = screen.getByRole("button", { name: "Create collaboration issue" });
  expect(submit).toBeDisabled();
  const user = userEvent.setup();
  const fields = screen.getByRole("button", { name: /Initial fields/ });
  if (fields.getAttribute("aria-expanded") !== "true") await user.click(fields);
  const project = screen.getByLabelText("Project");
  await user.selectOptions(project, unmanagedProject);
  await waitFor(() => expect(finish).toBeTypeOf("function"));
  expect(screen.getByLabelText("Title")).toBe(title);
  expect(title).toHaveValue("Keep this project draft");
  expect(screen.getByText("Checking the selected project's information policy...")).toBeInTheDocument();
  expect(screen.queryByText("This project requires complete information requests. Initial blocked status is unavailable.")).toBeNull();
  expect(submit).toBeDisabled();
  fireEvent.submit(title.closest("form")!);
  expect(fetch.mock.calls.some(([url, init]) => new URL(url).pathname === "/api/collaboration/issues" && init?.method === "POST")).toBe(false);
  await act(async () => finish(json(options(unmanagedProject))));
  await waitFor(() => expect(submit).toBeEnabled());
  expect(screen.getByLabelText("Title")).toBe(title);
  await user.click(submit);
  await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  const body = JSON.parse(String(fetch.mock.calls.find(([url, init]) => new URL(url).pathname === "/api/collaboration/issues" && init?.method === "POST")?.[1]?.body));
  expect(body).toMatchObject({ project_id: unmanagedProject, status: "blocked", title: "Keep this project draft", description: "Keep this description" });
  expect(Object.hasOwn(body, "stage_policy_observation")).toBe(false);
});
