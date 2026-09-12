import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import type { Agent, AgentRuntime, Project, ProjectResource, SquadTemplate } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { UseSquadForProjectDialog } from "./use-squad-for-project-dialog";

// Permission and machine-selection matrices belong to core/projects/execution-squad.test.ts.
// This suite verifies the actual helper wiring and explicit project mutation boundary.
const mocks = vi.hoisted(() => ({
  workspaceId: "ws-1", projects: [] as Project[], runtimes: [] as AgentRuntime[],
  agents: [] as Agent[], resources: [] as ProjectResource[],
  projectsError: false, resourcesError: false, runtimesError: false,
  resourcesLoading: false, configure: vi.fn(), configureWorkspace: vi.fn(),
  retryProjects: vi.fn(), retryResources: vi.fn(), retryRuntimes: vi.fn(), openModal: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  queryOptions: (options: unknown) => options,
  useQuery: ({ queryKey }: { queryKey: readonly unknown[] }) => {
    if (queryKey.includes("resources")) return { data: mocks.resources, isLoading: mocks.resourcesLoading, isPending: mocks.resourcesLoading, isError: mocks.resourcesError, refetch: mocks.retryResources };
    if (queryKey[0] === "projects") return { data: mocks.projects, isLoading: false, isPending: false, isError: mocks.projectsError, refetch: mocks.retryProjects };
    if (queryKey[0] === "runtimes") return { data: mocks.runtimes, isLoading: false, isPending: false, isError: mocks.runtimesError, refetch: mocks.retryRuntimes };
    return { data: mocks.agents, isLoading: false, isPending: false, isError: false, refetch: vi.fn() };
  },
  useMutation: vi.fn(), useQueryClient: vi.fn(),
}));
vi.mock("@multica/core/projects", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/projects")>()),
  useConfigureProjectSquad: (wsId: string) => {
    mocks.configureWorkspace(wsId);
    return { mutateAsync: mocks.configure, isPending: false };
  },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => mocks.workspaceId }));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("@multica/core/modals", () => {
  const state = { open: mocks.openModal };
  return { useModalStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("./project-picker", () => ({
  ProjectPicker: ({ projectId, onUpdate }: { projectId: string | null; onUpdate: (value: { project_id: string }) => void }) => (
    <select aria-label="Project" value={projectId ?? ""} onChange={(event) => onUpdate({ project_id: event.target.value })}>
      <option value="">Choose</option>
      {mocks.projects.map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
    </select>
  ),
}));

const TEMPLATE: SquadTemplate = {
  key: "feature-delivery", version: 1, name: "Feature delivery", title: "Feature delivery",
  description: "Design, build, review, and verify a feature.", avatar_emoji: "", instructions: "Coordinate the work.",
  leader: { template_key: "tech-lead", title: "Tech lead", name: "Lead", role: "Coordinate", autonomy_level: "coordinator", avatar_emoji: "" }, members: [],
};
const PROJECT: Project = {
  id: "project-1", workspace_id: "ws-1", title: "Payments", description: null, icon: null,
  status: "planned", priority: "none", lead_type: null, lead_id: null, start_date: null, due_date: null,
  created_at: "", updated_at: "", issue_count: 0, done_count: 0, resource_count: 0,
};
function runtime(id: string, overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
    id, workspace_id: "ws-1", daemon_id: "machine-1", name: `Codex (${id})`, custom_name: id,
    runtime_mode: "local", provider: "codex", launch_header: "", status: "online", device_info: "", metadata: {},
    owner_id: "user-1", visibility: "private", last_seen_at: null, created_at: "", updated_at: "", ...overrides,
  };
}
function localDirectory(daemonId: string): ProjectResource {
  return { id: "resource-1", project_id: PROJECT.id, workspace_id: "ws-1", resource_type: "local_directory", resource_ref: { daemon_id: daemonId, local_path: "/project" }, label: null, position: 0, created_at: "", created_by: "user-1" };
}
function renderDialog() {
  const navigation: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/squads", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  const onClose = vi.fn();
  const element = () => <NavigationProvider value={navigation}><UseSquadForProjectDialog template={TEMPLATE} onClose={onClose} /></NavigationProvider>;
  const view = renderWithI18n(element());
  return { ...view, navigation, onClose, changeWorkspace: (wsId: string) => { mocks.workspaceId = wsId; view.rerender(element()); } };
}
function chooseProject() {
  fireEvent.change(screen.getByRole("combobox", { name: "Project" }), { target: { value: PROJECT.id } });
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.workspaceId = "ws-1"; mocks.projects = [PROJECT]; mocks.runtimes = [];
  mocks.agents = []; mocks.resources = []; mocks.projectsError = false; mocks.resourcesError = false;
  mocks.runtimesError = false; mocks.resourcesLoading = false;
  mocks.configure.mockResolvedValue({ ...PROJECT, execution_squad: { state: "needs_runtime", template_key: TEMPLATE.key } });
});

describe("UseSquadForProjectDialog", () => {
  it("writes only on confirmation and retains a template choice without a runtime", async () => {
    const { navigation, onClose } = renderDialog();
    expect(mocks.configure).not.toHaveBeenCalled();
    chooseProject();
    expect(mocks.configure).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Use for project" }));
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(mocks.configure).toHaveBeenCalledWith({ id: PROJECT.id, template_key: TEMPLATE.key, language: "en" });
    expect(mocks.configureWorkspace).toHaveBeenCalledWith("ws-1");
    expect(navigation.push).toHaveBeenCalledWith("/acme/projects/project-1");
  });

  it("opens project creation with the selected template when no projects exist", () => {
    mocks.projects = [];
    const { onClose } = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Create a project" }));
    expect(mocks.openModal).toHaveBeenCalledWith("create-project", { squad_template_key: TEMPLATE.key });
    expect(onClose).toHaveBeenCalledOnce();
    expect(mocks.configure).not.toHaveBeenCalled();
  });

  it("waits for project resources and binds only a runtime for the project's machine", async () => {
    mocks.runtimes = [runtime("wrong-machine", { daemon_id: "machine-2" }), runtime("project-machine")];
    mocks.resources = [localDirectory("machine-1")];
    const { rerender, navigation } = renderDialog();
    mocks.resourcesLoading = true;
    chooseProject();
    expect(screen.getByRole("button", { name: "Use for project" })).toBeDisabled();
    mocks.resourcesLoading = false;
    rerender(<NavigationProvider value={navigation}><UseSquadForProjectDialog template={TEMPLATE} onClose={vi.fn()} /></NavigationProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Use for project" }));
    await waitFor(() => expect(mocks.configure).toHaveBeenCalledOnce());
    expect(mocks.configure).toHaveBeenCalledWith({ id: PROJECT.id, template_key: TEMPLATE.key, runtime_id: "project-machine", language: "en" });
  });

  it("does not silently choose one of several eligible machines", async () => {
    mocks.runtimes = [runtime("one"), runtime("two", { daemon_id: "machine-2" })];
    renderDialog();
    chooseProject();
    expect(screen.getByRole("combobox", { name: "Execution runtime" })).toHaveTextContent("Connect later");
    fireEvent.click(screen.getByRole("button", { name: "Use for project" }));
    await waitFor(() => expect(mocks.configure).toHaveBeenCalledWith({ id: PROJECT.id, template_key: TEMPLATE.key, language: "en" }));
  });

  it("retains the selection after a rejected save and retries explicitly", async () => {
    mocks.configure.mockRejectedValueOnce(new Error("request failed"));
    renderDialog();
    chooseProject();
    fireEvent.click(screen.getByRole("button", { name: "Use for project" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not update the project's squad. Try again.");
    expect(screen.getByRole("combobox", { name: "Project" })).toHaveValue(PROJECT.id);
    fireEvent.click(screen.getByRole("button", { name: "Use for project" }));
    await waitFor(() => expect(mocks.configure).toHaveBeenCalledTimes(2));
  });

  it("provides query recovery while keeping the new-project entry available", () => {
    mocks.projectsError = true;
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.retryProjects).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: "Create a project" })).toBeEnabled();
  });

  it("does not navigate or close a later workspace after an old request finishes", async () => {
    let resolve!: (project: Project) => void;
    mocks.configure.mockImplementation(() => new Promise<Project>((done) => { resolve = done; }));
    const { changeWorkspace, navigation, onClose } = renderDialog();
    chooseProject();
    fireEvent.click(screen.getByRole("button", { name: "Use for project" }));
    changeWorkspace("ws-2");
    await act(async () => { resolve(PROJECT); });
    expect(navigation.push).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
