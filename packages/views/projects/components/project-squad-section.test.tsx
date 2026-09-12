import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Agent, AgentRuntime, Project, ProjectResource, Squad, SquadMemberStatus } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";

const mocks = vi.hoisted(() => ({
  agents: [] as Agent[], squads: [] as Squad[], runtimes: [] as AgentRuntime[], resources: [] as ProjectResource[],
  configure: vi.fn(), configureWorkspace: vi.fn(), open: vi.fn(), push: vi.fn(),
  resourcesLoading: false,
  roster: [] as SquadMemberStatus[],
  pendingQuery: "",
  errorQuery: "",
  errorStatus: 500,
  refetchResources: vi.fn(),
  refetchRoster: vi.fn(),
  refetchOther: vi.fn(),
}));
vi.mock("@tanstack/react-query", async () => ({
  ...await vi.importActual<Record<string, unknown>>("@tanstack/react-query"),
  useQuery: ({ queryKey }: { queryKey: unknown[] }) => ({
    data: queryKey.includes(mocks.errorQuery) ? undefined
      : queryKey.includes("members-status") ? { members: mocks.roster }
      : queryKey.includes("resources") ? mocks.resources
      : queryKey.includes("agents") ? mocks.agents
        : queryKey.includes("squads") ? mocks.squads
          : queryKey.includes("runtimes") ? mocks.runtimes : [],
    isLoading: queryKey.includes("resources") && mocks.resourcesLoading,
    isPending: (queryKey.includes("resources") && mocks.resourcesLoading) || queryKey.includes(mocks.pendingQuery),
    isError: queryKey.includes(mocks.errorQuery),
    error: queryKey.includes(mocks.errorQuery) ? { status: mocks.errorStatus } : null,
    fetchStatus: queryKey.includes(mocks.pendingQuery) ? "paused" : "idle",
    refetch: queryKey.includes("resources") ? mocks.refetchResources
      : queryKey.includes("members-status") ? mocks.refetchRoster : mocks.refetchOther,
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/permissions", async () => ({
  ...await vi.importActual<Record<string, unknown>>("@multica/core/permissions"),
  useCurrentMember: () => ({ userId: "user-1", role: "admin", isLoading: false }),
}));
vi.mock("@multica/core/projects/mutations", () => ({
  useConfigureProjectSquad: (wsId: string) => {
    mocks.configureWorkspace(wsId);
    return { mutateAsync: mocks.configure, isPending: false };
  },
}));
vi.mock("@multica/core/modals", () => ({ useModalStore: { getState: () => ({ open: mocks.open }) } }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({
  runtimes: () => "/ws/runtimes", squadDetail: (id: string) => `/ws/squads/${id}`,
}) }));
vi.mock("../../navigation", () => ({ useNavigation: () => ({ push: mocks.push }) }));
vi.mock("../../agents/create/use-role-templates", async () => ({
  ...await vi.importActual<Record<string, unknown>>("../../agents/create/use-role-templates"),
  useSquadTemplates: () => ({ data: [{ key: "feature-delivery", title: "Feature delivery" }], isLoading: false }),
}));
vi.mock("./project-squad-picker", () => ({
  ProjectSquadPicker: ({ onChange, disabled }: { onChange: (value: object) => void; disabled?: boolean }) => (
    <>
      <button disabled={disabled} onClick={() => onChange({})}>Choose no squad</button>
      <button disabled={disabled} onClick={() => onChange({ squad_id: "replacement-squad" })}>Choose replacement squad</button>
    </>
  ),
}));

import { ProjectSquadSection } from "./project-squad-section";

const RUNTIME = {
  id: "actual-runtime", workspace_id: "ws-1", daemon_id: "actual-machine", name: "Codex (Work Mac)",
  runtime_mode: "local", provider: "codex", status: "online", owner_id: "user-1", visibility: "private",
} as AgentRuntime;
const LEADER: Agent = {
  id: "leader-1", workspace_id: "ws-1", runtime_id: "actual-runtime", owner_id: "user-1",
  permission_mode: "private", invocation_targets: [], archived_at: null, name: "Leader",
  description: "", instructions: "", avatar_url: null, runtime_mode: "local", runtime_config: {},
  custom_args: [], visibility: "private", status: "idle", max_concurrent_tasks: 1, model: "", skills: [],
  created_at: "", updated_at: "", archived_by: null,
};
const IMPLEMENTER: Agent = { ...LEADER, id: "implementer-1", name: "Implementer" };
const SQUAD = { id: "squad-1", workspace_id: "ws-1", leader_id: "leader-1", archived_at: null, name: "Delivery team" } as Squad;
const PROJECT = {
  id: "project-1", workspace_id: "ws-1", title: "Product launch",
  execution_squad: { state: "configured", template_key: "feature-delivery", squad_id: "squad-1", runtime_id: "requested-runtime" },
} as Project;

beforeEach(() => {
  vi.clearAllMocks();
  mocks.agents = [LEADER, IMPLEMENTER]; mocks.squads = [SQUAD]; mocks.runtimes = [RUNTIME]; mocks.resources = [];
  mocks.roster = [LEADER, IMPLEMENTER].map((agent) => ({
    member_type: "agent", member_id: agent.id, status: "idle", active_issues: [], last_active_at: null,
  }));
  mocks.resourcesLoading = false;
  mocks.pendingQuery = "";
  mocks.errorQuery = "";
  mocks.errorStatus = 500;
  mocks.configure.mockResolvedValue(PROJECT);
});

describe("ProjectSquadSection", () => {
  it("opens ordinary issue creation with the project's squad and todo status only on request", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    expect(mocks.open).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Hand to squad" }));
    expect(mocks.open).toHaveBeenCalledExactlyOnceWith("create-issue", {
      project_id: "project-1", assignee_type: "squad", assignee_id: "squad-1", status: "todo",
    });
  });

  it("checks the actual leader's runtime against the local directory", () => {
    mocks.resources = [{ resource_type: "local_directory", resource_ref: { daemon_id: "requested-machine", local_path: "/repo" } } as ProjectResource];
    mocks.runtimes.push({ ...RUNTIME, id: "requested-runtime", daemon_id: "requested-machine" });
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    expect(screen.getByText("Different machine")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
  });

  it("checks the full roster when a non-leader moves to a different machine", () => {
    mocks.resources = [{ resource_type: "local_directory", resource_ref: { daemon_id: "actual-machine", local_path: "/repo" } } as ProjectResource];
    mocks.agents = [LEADER, { ...IMPLEMENTER, runtime_id: "worker-runtime" }];
    mocks.runtimes.push({ ...RUNTIME, id: "worker-runtime", daemon_id: "other-machine" });
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    expect(screen.getByText("Different machine")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
  });

  it("requires invocation access to every agent in the roster", () => {
    mocks.agents = [LEADER, { ...IMPLEMENTER, owner_id: "someone-else" }];
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    expect(screen.getByText("Squad unavailable")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
  });

  it("does not claim readiness while resources are loading", () => {
    mocks.resourcesLoading = true;
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    expect(screen.getByText("Checking availability...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
  });

  it.each(["resources", "members-status", "members"])("keeps dispatch disabled while %s is paused before loading", (query) => {
    mocks.pendingQuery = query;
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    expect(screen.getByText("Checking availability...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Retry availability" })).toBeInTheDocument();
  });

  it("fails closed on resource query errors and retries the readiness queries", async () => {
    const user = userEvent.setup();
    mocks.errorQuery = "resources";
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    expect(screen.getByText("Could not check availability")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Retry availability" }));
    expect(mocks.refetchResources).toHaveBeenCalledOnce();
    expect(mocks.refetchRoster).toHaveBeenCalledOnce();
    expect(mocks.configure).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Change squad" }));
    expect(screen.getByRole("button", { name: "Save squad" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Retry availability" })).toBeInTheDocument();
  });

  it.each([
    ["Choose no squad", { id: "project-1" }],
    ["Choose replacement squad", { id: "project-1", squad_id: "replacement-squad" }],
  ])("allows %s when the deleted default's roster returns 404", async (selection, request) => {
    const user = userEvent.setup();
    mocks.squads = [{ ...SQUAD, id: "replacement-squad", name: "Replacement squad" }];
    mocks.roster = [];
    mocks.errorQuery = "members-status";
    mocks.errorStatus = 404;
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Change squad" }));
    const choice = screen.getByRole("button", { name: selection });
    expect(choice).toBeEnabled();
    await user.click(choice);
    const save = screen.getByRole("button", { name: "Save squad" });
    expect(save).toBeEnabled();
    await user.click(save);

    expect(mocks.configure).toHaveBeenCalledExactlyOnceWith(request);
    expect(mocks.open).not.toHaveBeenCalled();
  });

  it("shows connection recovery for an offline execution machine", async () => {
    const user = userEvent.setup();
    mocks.runtimes = [{ ...RUNTIME, status: "offline" }];
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    expect(screen.getByText("Runtime offline")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hand to squad" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Connect runtime" }));
    expect(mocks.push).toHaveBeenCalledWith("/ws/runtimes");
  });

  it("retries the retained setup choice without recreating the project", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squad: {
      state: "failed", template_key: "feature-delivery", runtime_id: "requested-runtime", error_code: "preparation_failed",
    } }} />);
    expect(screen.getByText(/Your project is saved/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry setup" }));
    await waitFor(() => expect(mocks.configure).toHaveBeenCalledWith({
      id: "project-1", template_key: "feature-delivery", runtime_id: "requested-runtime", language: "en",
    }));
    expect(mocks.configureWorkspace).toHaveBeenCalledWith("ws-1");
  });

  it("requires a new selection instead of retrying a malformed configuration as an empty clear", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squad: {
      state: "failed", error_code: "invalid_configuration",
    } }} />);

    expect(screen.queryByRole("button", { name: "Retry setup" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Change squad" }));
    expect(screen.getByRole("button", { name: "Save squad" })).toBeInTheDocument();
    expect(mocks.configure).not.toHaveBeenCalled();
  });

  it("saves an explicit no-squad default without changing existing issues", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    await user.click(screen.getByRole("button", { name: "Change squad" }));
    expect(screen.getByText("This changes defaults for future issues. Existing issues keep their assignees.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Choose no squad" }));
    await user.click(screen.getByRole("button", { name: "Save squad" }));
    expect(mocks.configure).toHaveBeenCalledWith({ id: "project-1" });
    expect(mocks.open).not.toHaveBeenCalled();
  });
});
