import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AnchorHTMLAttributes } from "react";
import type { Agent, AgentRuntime, Project, ProjectResource, Squad, SquadMemberStatus } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";

const mocks = vi.hoisted(() => ({
  agents: [] as Agent[], squads: [] as Squad[], runtimes: [] as AgentRuntime[], resources: [] as ProjectResource[],
  configure: vi.fn(), configureWorkspace: vi.fn(), open: vi.fn(), push: vi.fn(),
  resourcesLoading: false,
  roster: [] as SquadMemberStatus[],
  rosters: {} as Record<string, SquadMemberStatus[]>,
  pendingQuery: "",
  errorQuery: "",
  retainErrorData: false,
  errorStatus: 500,
  refetchResources: vi.fn(),
  refetchRoster: vi.fn(),
  refetchOther: vi.fn(),
}));
vi.mock("@tanstack/react-query", async () => {
  const queryResult = ({ queryKey }: { queryKey: unknown[] }) => ({
    data: queryKey.includes(mocks.errorQuery) && !mocks.retainErrorData ? undefined
      : queryKey.includes("members-status") ? { members: mocks.rosters[String(queryKey[3])] ?? mocks.roster }
      : queryKey.includes("resources") ? mocks.resources
      : queryKey.includes("agents") ? mocks.agents
        : queryKey.includes("squads") ? mocks.squads
          : queryKey.includes("runtimes") ? mocks.runtimes : [],
    isLoading: queryKey.includes("resources") && mocks.resourcesLoading,
    isPending: (queryKey.includes("resources") && mocks.resourcesLoading) || queryKey.includes(mocks.pendingQuery),
    isError: queryKey.includes(mocks.errorQuery),
    isFetching: false,
    error: queryKey.includes(mocks.errorQuery) ? { status: mocks.errorStatus } : null,
    fetchStatus: queryKey.includes(mocks.pendingQuery) ? "paused" : "idle",
    refetch: queryKey.includes("resources") ? mocks.refetchResources
      : queryKey.includes("members-status") ? mocks.refetchRoster : mocks.refetchOther,
  });
  return {
    ...await vi.importActual<Record<string, unknown>>("@tanstack/react-query"),
    useQuery: queryResult,
    useQueries: ({ queries }: { queries: { queryKey: unknown[] }[] }) => queries.map(queryResult),
  };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/permissions", async () => ({
  ...await vi.importActual<Record<string, unknown>>("@multica/core/permissions"),
  useCurrentMember: () => ({ userId: "user-1", role: "admin", isLoading: false }),
}));
vi.mock("@multica/core/projects/mutations", () => ({
  useConfigureProjectSquads: (wsId: string) => {
    mocks.configureWorkspace(wsId);
    return { mutateAsync: mocks.configure, isPending: false };
  },
}));
vi.mock("@multica/core/modals", () => ({ useModalStore: { getState: () => ({ open: mocks.open }) } }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({
  runtimes: () => "/ws/runtimes", squadDetail: (id: string) => `/ws/squads/${id}`,
}) }));
vi.mock("../../navigation", () => ({
  useNavigation: () => ({ push: mocks.push }),
  AppLink: (props: AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} />,
}));
vi.mock("../../agents/create/use-role-templates", async () => ({
  ...await vi.importActual<Record<string, unknown>>("../../agents/create/use-role-templates"),
  useSquadTemplates: () => ({ data: [{ key: "feature-delivery", title: "Feature delivery" }], isLoading: false }),
}));
vi.mock("./project-squad-picker", () => ({
  ProjectSquadPicker: ({ onChange, disabled }: { onChange: (value: object) => void; disabled?: boolean }) => (
    <>
      <button disabled={disabled} onClick={() => onChange({})}>Choose no squad</button>
      <button disabled={disabled} onClick={() => onChange({ squad_id: "replacement-squad" })}>Choose replacement squad</button>
      <button disabled={disabled} onClick={() => onChange({ squad_id: "squad-1" })}>Choose current squad</button>
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
const SQUAD = {
  id: "squad-1", workspace_id: "ws-1", leader_id: "leader-1", archived_at: null,
  name: "Delivery team", description: "Our customized delivery process.",
} as Squad;
const PROJECT = {
  id: "project-1", workspace_id: "ws-1", title: "Product launch",
  execution_squad: { state: "configured", template_key: "feature-delivery", squad_id: "squad-1", runtime_id: "requested-runtime" },
} as Project;

beforeEach(() => {
  vi.clearAllMocks();
  mocks.open.mockReset();
  mocks.agents = [LEADER, IMPLEMENTER]; mocks.squads = [SQUAD]; mocks.runtimes = [RUNTIME]; mocks.resources = [];
  mocks.roster = [LEADER, IMPLEMENTER].map((agent) => ({
    member_type: "agent", member_id: agent.id, status: "idle", active_issues: [], last_active_at: null,
  }));
  mocks.resourcesLoading = false;
  mocks.rosters = {};
  mocks.pendingQuery = "";
  mocks.errorQuery = "";
  mocks.retainErrorData = false;
  mocks.errorStatus = 500;
  mocks.configure.mockResolvedValue(PROJECT);
});

async function openManager(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Manage squads" }));
  return within(await screen.findByRole("dialog", { name: "Manage squads" }));
}

async function openActions(user: ReturnType<typeof userEvent.setup>, name = "Delivery team") {
  await user.click(screen.getByRole("button", { name: `Actions for ${name}` }));
  return within(await screen.findByRole("menu"));
}

async function expectDispatchDisabled(user: ReturnType<typeof userEvent.setup>, name = "Delivery team") {
  const menu = await openActions(user, name);
  expect(menu.getByRole("menuitem", { name: "New issue with this squad" })).toHaveAttribute("aria-disabled", "true");
  await user.keyboard("{Escape}");
}

describe("ProjectSquadSection", () => {
  it.each([1, 8, 20])("keeps %s squads in a compact summary until management is opened", (count) => {
    mocks.squads = Array.from({ length: count }, (_, index) => ({ ...SQUAD, id: `squad-${index + 1}`, name: `Team ${index + 1}` }));
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squads: mocks.squads.map((squad) => ({ state: "configured", squad_id: squad.id })) }} />);

    expect(screen.getByRole("button", { name: "Manage squads" })).toBeInTheDocument();
    expect(screen.getByText(`${count} ready`)).toBeInTheDocument();
    expect(screen.getByText("New issue default: Team 1")).toBeInTheDocument();
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /New issue with this squad|Hand to squad/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/Execution runtime:/)).not.toBeInTheDocument();
    expect(mocks.open).not.toHaveBeenCalled();
  });

  it("opens the manager by keyboard, shows the saved description and restores focus on dismissal", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    const trigger = screen.getByRole("button", { name: "Manage squads" });
    trigger.focus();
    await user.keyboard("{Enter}");

    const manager = within(await screen.findByRole("dialog", { name: "Manage squads" }));
    expect(manager.getByText(SQUAD.description)).toBeInTheDocument();
    expect(manager.getByRole("link", { name: "Delivery team" })).toHaveAttribute("href", "/ws/squads/squad-1");
    expect(manager.getByText("New issue default")).toBeInTheDocument();
    expect(manager.queryByText(/Execution runtime:/)).not.toBeInTheDocument();

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  it("opens ordinary issue creation with the project's squad and todo status only on request", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    expect(mocks.open).not.toHaveBeenCalled();
    mocks.open.mockImplementation(() => {
      expect(screen.queryByRole("dialog", { name: "Manage squads" })).not.toBeInTheDocument();
    });
    await openManager(user);
    const menu = await openActions(user);
    await user.click(menu.getByRole("menuitem", { name: "New issue with this squad" }));
    await waitFor(() => expect(mocks.open).toHaveBeenCalledExactlyOnceWith("create-issue", {
      project_id: "project-1", assignee_type: "squad", assignee_id: "squad-1", status: "todo",
    }));
  });

  it("checks the actual leader's runtime against the local directory", async () => {
    const user = userEvent.setup();
    mocks.resources = [{ resource_type: "local_directory", resource_ref: { daemon_id: "requested-machine", local_path: "/repo" } } as ProjectResource];
    mocks.runtimes.push({ ...RUNTIME, id: "requested-runtime", daemon_id: "requested-machine" });
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    expect(screen.getByText("1 need attention")).toBeInTheDocument();
    const manager = await openManager(user);
    expect(manager.getByText("Different machine")).toBeInTheDocument();
    await expectDispatchDisabled(user);
  });

  it("checks the full roster when a non-leader moves to a different machine", async () => {
    const user = userEvent.setup();
    mocks.resources = [{ resource_type: "local_directory", resource_ref: { daemon_id: "actual-machine", local_path: "/repo" } } as ProjectResource];
    mocks.agents = [LEADER, { ...IMPLEMENTER, runtime_id: "worker-runtime" }];
    mocks.runtimes.push({ ...RUNTIME, id: "worker-runtime", daemon_id: "other-machine" });
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    const manager = await openManager(user);
    expect(manager.getByText("Different machine")).toBeInTheDocument();
    await expectDispatchDisabled(user);
  });

  it("requires invocation access to every agent in the roster", async () => {
    const user = userEvent.setup();
    mocks.agents = [LEADER, { ...IMPLEMENTER, owner_id: "someone-else" }];
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    const manager = await openManager(user);
    expect(manager.getByText("Squad unavailable")).toBeInTheDocument();
    await expectDispatchDisabled(user);
  });

  it("does not claim readiness while resources are loading", async () => {
    const user = userEvent.setup();
    mocks.resourcesLoading = true;
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    expect(screen.getByText("Checking 1")).toBeInTheDocument();
    expect(screen.getByText("Checking the default squad’s availability.")).toBeInTheDocument();
    expect(screen.queryByText("1 ready")).not.toBeInTheDocument();
    await openManager(user);
    await expectDispatchDisabled(user);
  });

  it.each(["resources", "members-status", "members"])("keeps dispatch disabled while %s is paused before loading", async (query) => {
    const user = userEvent.setup();
    mocks.pendingQuery = query;
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    expect(screen.getByText("Checking 1")).toBeInTheDocument();
    const manager = await openManager(user);
    expect(manager.getByText("Checking availability...")).toBeInTheDocument();
    await expectDispatchDisabled(user);
    expect(screen.getByRole("button", { name: "Retry availability" })).toBeInTheDocument();
  });

  it("fails closed on resource query errors and retries the readiness queries", async () => {
    const user = userEvent.setup();
    mocks.errorQuery = "resources";
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    expect(screen.getByText("Could not check availability")).toBeInTheDocument();
    expect(screen.getByText("1 need attention")).toBeInTheDocument();
    await openManager(user);
    await expectDispatchDisabled(user);
    await user.click(screen.getByRole("button", { name: "Retry availability" }));
    expect(mocks.refetchResources).toHaveBeenCalledOnce();
    expect(mocks.refetchRoster).toHaveBeenCalledOnce();
    expect(mocks.configure).not.toHaveBeenCalled();
    const menu = await openActions(user);
    await user.click(menu.getByRole("menuitem", { name: "Change squad" }));
    expect(screen.getByRole("button", { name: "Save squad" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Retry availability" })).toBeInTheDocument();
  });

  it.each([
    ["Choose no squad", { id: "project-1", squads: [] }],
    ["Choose replacement squad", { id: "project-1", squads: [{ squad_id: "replacement-squad" }] }],
  ])("allows %s when the deleted default's roster returns 404", async (selection, request) => {
    const user = userEvent.setup();
    mocks.squads = [{ ...SQUAD, id: "replacement-squad", name: "Replacement squad" }];
    mocks.roster = [];
    mocks.errorQuery = "members-status";
    mocks.errorStatus = 404;
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);

    await openManager(user);
    await expectDispatchDisabled(user, "Feature delivery");
    const menu = await openActions(user, "Feature delivery");
    await user.click(menu.getByRole("menuitem", { name: "Change squad" }));
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
    const manager = await openManager(user);
    expect(manager.getByText("Runtime offline")).toBeInTheDocument();
    await expectDispatchDisabled(user);
    await user.click(screen.getByRole("button", { name: "Connect runtime" }));
    expect(mocks.push).toHaveBeenCalledWith("/ws/runtimes");
  });

  it("retries the retained setup choice without recreating the project", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squad: {
      state: "failed", template_key: "feature-delivery", runtime_id: "requested-runtime", error_code: "preparation_failed",
    } }} />);
    await openManager(user);
    expect(screen.getByText(/Your project is saved/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry setup" }));
    await waitFor(() => expect(mocks.configure).toHaveBeenCalledWith({
      id: "project-1", squads: [{ template_key: "feature-delivery", runtime_id: "requested-runtime", language: "en" }],
    }));
    expect(mocks.configureWorkspace).toHaveBeenCalledWith("ws-1");
  });

  it("requires a new selection instead of retrying a malformed configuration as an empty clear", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squad: {
      state: "failed", error_code: "invalid_configuration",
    } }} />);

    await openManager(user);
    expect(screen.queryByRole("button", { name: "Retry setup" })).not.toBeInTheDocument();
    const menu = await openActions(user, "Execution squad");
    await user.click(menu.getByRole("menuitem", { name: "Change squad" }));
    expect(screen.getByRole("button", { name: "Save squad" })).toBeInTheDocument();
    expect(mocks.configure).not.toHaveBeenCalled();
  });

  it("saves an explicit no-squad default without changing existing issues", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    await openManager(user);
    const menu = await openActions(user);
    await user.click(menu.getByRole("menuitem", { name: "Change squad" }));
    expect(screen.getByText("This changes defaults for future issues. Existing issues keep their assignees.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Choose no squad" }));
    await user.click(screen.getByRole("button", { name: "Save squad" }));
    expect(mocks.configure).toHaveBeenCalledWith({ id: "project-1", squads: [] });
    expect(mocks.open).not.toHaveBeenCalled();
  });
  it("shows all candidates and dispatches only the explicitly chosen squad", async () => {
    const user = userEvent.setup();
    mocks.squads.push({ ...SQUAD, id: "squad-2", name: "Review team" });
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squads: [
      PROJECT.execution_squad!, { state: "configured", squad_id: "squad-2" },
    ] }} />);

    const manager = await openManager(user);
    expect(manager.getByRole("group", { name: "Delivery team" })).toBeInTheDocument();
    const menu = await openActions(user, "Review team");
    await user.click(menu.getByRole("menuitem", { name: "New issue with this squad" }));
    await waitFor(() => expect(mocks.open).toHaveBeenCalledExactlyOnceWith("create-issue", {
      project_id: "project-1", assignee_type: "squad", assignee_id: "squad-2", status: "todo",
    }));
  });

  it("removes a candidate while preserving the other configured squads", async () => {
    const user = userEvent.setup();
    mocks.squads.push({ ...SQUAD, id: "squad-2", name: "Review team" });
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squads: [
      PROJECT.execution_squad!, { state: "configured", squad_id: "squad-2" },
    ] }} />);
    await openManager(user);
    const menu = await openActions(user, "Review team");
    expect(screen.getByText("Only the project association is removed. Existing issues keep their assignees.")).toBeInTheDocument();
    await user.click(menu.getByRole("menuitem", { name: "Remove Review team" }));
    expect(mocks.configure).toHaveBeenCalledExactlyOnceWith({
      id: "project-1", squads: [{ template_key: "feature-delivery", runtime_id: "requested-runtime" }],
    });
  });

  it("edits one candidate without replacing the others", async () => {
    const user = userEvent.setup();
    mocks.squads.push({ ...SQUAD, id: "squad-2", name: "Review team" });
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squads: [
      PROJECT.execution_squad!, { state: "configured", squad_id: "squad-2" },
    ] }} />);
    const manager = await openManager(user);
    const row = within(manager.getByRole("group", { name: "Review team" }));
    const menu = await openActions(user, "Review team");
    await user.click(menu.getByRole("menuitem", { name: "Change squad" }));
    await user.click(row.getByRole("button", { name: "Choose replacement squad" }));
    await user.click(row.getByRole("button", { name: "Save squad" }));
    expect(mocks.configure).toHaveBeenCalledExactlyOnceWith({
      id: "project-1", squads: [
        { template_key: "feature-delivery", runtime_id: "requested-runtime" },
        { squad_id: "replacement-squad" },
      ],
    });
  });

  it("adds a candidate without replacing the default squad", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    await openManager(user);
    await user.click(screen.getByRole("button", { name: "Add squad" }));
    await user.click(screen.getByRole("button", { name: "Choose replacement squad" }));
    await user.click(screen.getByRole("button", { name: "Save squad" }));
    expect(mocks.configure).toHaveBeenCalledExactlyOnceWith({
      id: "project-1", squads: [
        { template_key: "feature-delivery", runtime_id: "requested-runtime" },
        { squad_id: "replacement-squad" },
      ],
    });
  });

  it("does not submit duplicate candidates when adding an existing selection", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    await openManager(user);
    await user.click(screen.getByRole("button", { name: "Add squad" }));
    await user.click(screen.getByRole("button", { name: "Choose current squad" }));
    await user.click(screen.getByRole("button", { name: "Save squad" }));
    expect(mocks.configure).toHaveBeenCalledExactlyOnceWith({
      id: "project-1", squads: [{ template_key: "feature-delivery", runtime_id: "requested-runtime" }],
    });
  });

  it("sets a future-issue default by reordering every candidate and retaining template provenance", async () => {
    const user = userEvent.setup();
    mocks.squads.push({ ...SQUAD, id: "squad-2", name: "Review team" }, { ...SQUAD, id: "squad-3", name: "Operations team" });
    const configs = [PROJECT.execution_squad!, { state: "configured", squad_id: "squad-2" } as const,
      { state: "configured", squad_id: "squad-3", template_key: "incident-response", runtime_id: "other-runtime" } as const];
    const { rerender } = renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squads: configs }} />);
    await openManager(user);
    const menu = await openActions(user, "Review team");
    await user.click(menu.getByRole("menuitem", { name: "Set as new issue default" }));

    expect(mocks.configure).toHaveBeenCalledExactlyOnceWith({ id: "project-1", squads: [
      { squad_id: "squad-2" },
      { template_key: "feature-delivery", runtime_id: "requested-runtime" },
      { template_key: "incident-response", runtime_id: "other-runtime" },
    ] });
    expect(mocks.open).not.toHaveBeenCalled();
    rerender(<ProjectSquadSection project={{ ...PROJECT, execution_squads: [configs[1]!, configs[0]!, configs[2]!] }} />);
    expect(within(screen.getByRole("group", { name: "Review team" })).getByText("New issue default")).toBeInTheDocument();
    expect(within(screen.getByRole("group", { name: "Delivery team" })).queryByText("New issue default")).not.toBeInTheDocument();
  });

  it("keeps live roster failures visible while closed and shares that result with the manager", async () => {
    const user = userEvent.setup();
    mocks.squads.push({ ...SQUAD, id: "squad-2", name: "Review team" });
    const project = { ...PROJECT, execution_squads: [PROJECT.execution_squad!, { state: "configured", squad_id: "squad-2" } as const] };
    const { rerender } = renderWithI18n(<ProjectSquadSection project={project} />);
    expect(screen.getByText("2 ready")).toBeInTheDocument();

    mocks.errorQuery = "squad-1";
    mocks.retainErrorData = true;
    rerender(<ProjectSquadSection project={project} />);
    expect(screen.getByText("1 ready")).toBeInTheDocument();
    expect(screen.getByText("1 need attention")).toBeInTheDocument();
    expect(screen.getByText("Could not check availability")).toBeInTheDocument();
    const manager = await openManager(user);
    expect(within(manager.getByRole("group", { name: "Delivery team" })).getByText("Could not check availability")).toBeInTheDocument();
    expect(within(manager.getByRole("group", { name: "Review team" })).getByText("Ready for an issue")).toBeInTheDocument();
    await expectDispatchDisabled(user);
  });

  it("reveals actual runtime details only on request", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    await openManager(user);
    const menu = await openActions(user);
    await user.click(menu.getByRole("menuitem", { name: "Runtime details" }));
    expect(screen.getByText(/Execution runtime:.*Work Mac/)).toBeInTheDocument();
    expect(screen.queryByText(/requested-runtime/)).not.toBeInTheDocument();
  });

  it("allows direct removal when a deleted roster returns 404", async () => {
    const user = userEvent.setup();
    mocks.errorQuery = "members-status";
    mocks.errorStatus = 404;
    renderWithI18n(<ProjectSquadSection project={PROJECT} />);
    await openManager(user);
    const menu = await openActions(user);
    await user.click(menu.getByRole("menuitem", { name: "Remove Delivery team" }));
    expect(mocks.configure).toHaveBeenCalledExactlyOnceWith({ id: "project-1", squads: [] });
  });

  it("prevents configuration writes and issue creation from overlapping an in-flight default change", async () => {
    const user = userEvent.setup();
    let finish!: (project: Project) => void;
    mocks.configure.mockImplementationOnce(() => new Promise<Project>((resolve) => { finish = resolve; }));
    mocks.squads.push({ ...SQUAD, id: "squad-2", name: "Review team" });
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squads: [
      PROJECT.execution_squad!, { state: "configured", squad_id: "squad-2" },
    ] }} />);
    await openManager(user);
    let menu = await openActions(user, "Review team");
    await user.click(menu.getByRole("menuitem", { name: "Set as new issue default" }));
    menu = await openActions(user);
    await user.click(menu.getByRole("menuitem", { name: "Remove Delivery team" }));
    menu = await openActions(user);
    await user.click(menu.getByRole("menuitem", { name: "New issue with this squad" }));

    expect(mocks.configure).toHaveBeenCalledOnce();
    expect(mocks.open).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Manage squads" })).toBeInTheDocument();
    await act(async () => finish(PROJECT));
  });

  it("shows a failed default write and allows a retry without opening an issue", async () => {
    const user = userEvent.setup();
    mocks.configure.mockRejectedValueOnce(new Error("Default could not be saved"));
    mocks.squads.push({ ...SQUAD, id: "squad-2", name: "Review team" });
    renderWithI18n(<ProjectSquadSection project={{ ...PROJECT, execution_squads: [
      PROJECT.execution_squad!, { state: "configured", squad_id: "squad-2" },
    ] }} />);
    await openManager(user);
    let menu = await openActions(user, "Review team");
    await user.click(menu.getByRole("menuitem", { name: "Set as new issue default" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Default could not be saved");
    menu = await openActions(user, "Review team");
    await user.click(menu.getByRole("menuitem", { name: "Set as new issue default" }));
    expect(mocks.configure).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(mocks.open).not.toHaveBeenCalled();
  });

});
