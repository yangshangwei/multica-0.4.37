import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Agent, AgentRuntime, ConfigureProjectSquadRequest, Squad, SquadTemplate } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";

const mocks = vi.hoisted(() => ({
  agents: [] as Agent[],
  squads: [] as Squad[],
  runtimes: [] as AgentRuntime[],
  templates: [] as SquadTemplate[],
  runtimeError: false,
  role: "admin" as "admin" | "member",
  push: vi.fn(),
}));

vi.mock("@tanstack/react-query", async () => ({
  ...await vi.importActual<Record<string, unknown>>("@tanstack/react-query"),
  useQuery: ({ queryKey }: { queryKey: unknown[] }) => ({
    data: queryKey.includes("agents") ? mocks.agents
      : queryKey.includes("squads") ? mocks.squads
        : queryKey.includes("runtimes") ? mocks.runtimes : [],
    isLoading: false,
    isError: queryKey.includes("runtimes") && mocks.runtimeError,
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/permissions", async () => ({
  ...await vi.importActual<Record<string, unknown>>("@multica/core/permissions"),
  useCurrentMember: () => ({ userId: "user-1", role: mocks.role, isLoading: false }),
}));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ runtimes: () => "/ws/runtimes" }) }));
vi.mock("../../navigation", () => ({ useNavigation: () => ({ push: mocks.push }) }));
vi.mock("../../agents/create/use-role-templates", () => ({
  useSquadTemplates: () => ({ data: mocks.templates, isLoading: false, isError: false }),
}));
// Machine grouping and keyboard behavior belong to runtime-picker.test.tsx.
// Keep this suite focused on the project-specific eligibility and choice wiring.
vi.mock("../../agents/components/inspector/runtime-picker", () => ({
  RuntimePicker: ({ value, runtimes, onChange }: { value: string; runtimes: AgentRuntime[]; onChange: (id: string) => void }) => (
    <select aria-label="Runtime" value={value} onChange={(event) => onChange(event.target.value)}>
      <option value="">Choose runtime</option>
      {runtimes.map((runtime) => <option key={runtime.id} value={runtime.id}>{runtime.id}</option>)}
    </select>
  ),
}));

import { ProjectSquadPicker } from "./project-squad-picker";
import { ProjectSquadsPicker } from "./project-squads-picker";

const RUNTIME: AgentRuntime = {
  id: "runtime-1", workspace_id: "ws-1", daemon_id: "machine-1", name: "Codex (machine-1)",
  runtime_mode: "local", provider: "codex", launch_header: "", status: "online", device_info: "",
  metadata: {}, owner_id: "user-1", visibility: "private", last_seen_at: new Date().toISOString(), created_at: "", updated_at: "",
};
const LEADER: Agent = {
  id: "leader-1", workspace_id: "ws-1", name: "Leader", runtime_id: "runtime-1", owner_id: "user-1",
  permission_mode: "private", invocation_targets: [], archived_at: null,
  description: "", instructions: "", avatar_url: null, runtime_mode: "local", runtime_config: {},
  custom_args: [], visibility: "private", status: "idle", max_concurrent_tasks: 1, model: "", skills: [],
  created_at: "", updated_at: "", archived_by: null,
};
const SQUAD: Squad = {
  id: "squad-1", workspace_id: "ws-1", name: "My squad", description: "", instructions: "", avatar_url: null,
  leader_id: "leader-1", creator_id: "user-1", created_at: "", updated_at: "", archived_at: null, archived_by: null,
};
const TEMPLATE = { key: "feature-delivery", title: "Feature delivery", description: "Plan, build, and review a feature.", leader: { title: "Coordinator" }, members: [{ title: "Developer" }] } as SquadTemplate;

function Harness({ initial = { template_key: "feature-delivery" }, localDaemonId }: { initial?: ConfigureProjectSquadRequest; localDaemonId?: string }) {
  const [value, setValue] = useState(initial);
  return <>
    <ProjectSquadPicker value={value} onChange={setValue} localDaemonId={localDaemonId} />
    <output aria-label="Saved choice">{JSON.stringify(value)}</output>
  </>;
}

function MultiHarness({ initial = [{ template_key: "feature-delivery" }], localDaemonId }: { initial?: ConfigureProjectSquadRequest[]; localDaemonId?: string }) {
  const [value, setValue] = useState(initial);
  return <>
    <ProjectSquadsPicker value={value} onChange={setValue} localDaemonId={localDaemonId} />
    <output aria-label="Saved choices">{JSON.stringify(value)}</output>
  </>;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.runtimes = [RUNTIME];
  mocks.agents = [LEADER];
  mocks.squads = [SQUAD];
  mocks.templates = [TEMPLATE, { ...TEMPLATE, key: "bug-triage", title: "Bug triage" }];
  mocks.runtimeError = false;
  mocks.role = "admin";
});

describe("ProjectSquadPicker", () => {
  it("inherits an authorized Mika runtime when several machines are available", async () => {
    mocks.runtimes = [RUNTIME, { ...RUNTIME, id: "mika-runtime", daemon_id: "mika-machine" }];
    mocks.agents.push({ ...LEADER, id: "mika", system_key: "mika", runtime_id: "mika-runtime" });
    renderWithI18n(<Harness />);

    await waitFor(() => expect(screen.getByLabelText("Saved choice")).toHaveTextContent('"runtime_id":"mika-runtime"'));
    expect(screen.getByRole("combobox", { name: "Execution squad" })).toHaveTextContent("Feature delivery");
  });

  it("does not guess between machines when there is no appropriate default", () => {
    mocks.runtimes = [RUNTIME, { ...RUNTIME, id: "runtime-2", daemon_id: "machine-2" }];
    renderWithI18n(<Harness />);

    expect(screen.getByLabelText("Saved choice")).not.toHaveTextContent("runtime_id");
    expect(screen.getByText(/Choose a runtime now, or connect one later/)).toBeInTheDocument();
  });

  it("preserves the retained runtime when the runtime query fails", () => {
    mocks.runtimes = [];
    mocks.runtimeError = true;
    renderWithI18n(<Harness initial={{ template_key: "feature-delivery", runtime_id: "runtime-1" }} />);
    expect(screen.getByLabelText("Saved choice")).toHaveTextContent('"runtime_id":"runtime-1"');
  });

  it("limits local project setup to the directory's authorized machine", async () => {
    mocks.runtimes = [
      { ...RUNTIME, id: "remote", daemon_id: "remote-machine" },
      RUNTIME,
      { ...RUNTIME, id: "private", owner_id: "someone-else" },
    ];
    renderWithI18n(<Harness localDaemonId="machine-1" />);

    await waitFor(() => expect(screen.getByLabelText("Saved choice")).toHaveTextContent('"runtime_id":"runtime-1"'));
    expect(screen.queryByRole("option", { name: "remote" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "private" })).not.toBeInTheDocument();
  });

  it("offers assignable existing squads and replaces the template/runtime choice", async () => {
    const user = userEvent.setup();
    mocks.agents.push({ ...LEADER, id: "private-leader", owner_id: "someone-else" });
    mocks.squads.push({ ...SQUAD, id: "private-squad", name: "Private squad", leader_id: "private-leader" });
    renderWithI18n(<Harness />);

    await user.click(screen.getByRole("combobox", { name: "Execution squad" }));
    const existing = await screen.findByRole("option", { name: "My squad" });
    expect(screen.queryByRole("option", { name: "Private squad" })).not.toBeInTheDocument();
    await user.click(existing);
    expect(JSON.parse(screen.getByLabelText("Saved choice").textContent!)).toEqual({ squad_id: "squad-1" });
  });

  it("lets regular members choose another owner's legitimately shared squad", async () => {
    const user = userEvent.setup();
    mocks.role = "member";
    mocks.agents = [{
      ...LEADER,
      owner_id: "another-owner",
      permission_mode: "public_to",
      invocation_targets: [{ target_type: "workspace", target_id: "ws-1" }],
    }];
    renderWithI18n(<Harness />);

    await user.click(screen.getByRole("combobox", { name: "Execution squad" }));
    await user.click(await screen.findByRole("option", { name: "My squad" }));
    expect(JSON.parse(screen.getByLabelText("Saved choice").textContent!)).toEqual({ squad_id: "squad-1" });
  });

  it("can opt out without reinstating the default", async () => {
    const user = userEvent.setup();
    renderWithI18n(<Harness />);
    await user.click(screen.getByRole("combobox", { name: "Execution squad" }));
    await user.click(await screen.findByRole("option", { name: "No execution squad" }));
    expect(screen.getByLabelText("Saved choice")).toHaveTextContent("{}");
    expect(screen.queryByLabelText("Runtime")).not.toBeInTheDocument();
  });
});

describe("ProjectSquadsPicker", () => {
  it("can defer an automatically suggested runtime without losing squads", async () => {
    const user = userEvent.setup();
    renderWithI18n(<MultiHarness />);
    await user.click(await screen.findByRole("button", { name: "Connect later" }));
    await user.click(screen.getByRole("button", { name: "Choose execution squads" }));
    await user.click(screen.getByRole("checkbox", { name: /Bug triage/ }));
    expect(JSON.parse(screen.getByLabelText("Saved choices").textContent!)).toEqual([
      { template_key: "feature-delivery" }, { template_key: "bug-triage" },
    ]);
  });

  it("does not select squads on another machine when choosing all", async () => {
    const user = userEvent.setup();
    renderWithI18n(<MultiHarness localDaemonId="other-machine" />);
    await user.click(screen.getByRole("button", { name: "Choose execution squads" }));
    expect(screen.getByRole("checkbox", { name: /My squad/ })).toHaveAttribute("aria-disabled", "true");
    await user.click(screen.getByRole("button", { name: "Select all" }));
    expect(JSON.parse(screen.getByLabelText("Saved choices").textContent!)).toEqual([
      { template_key: "feature-delivery" }, { template_key: "bug-triage" },
    ]);
  });

  it("explains squads and keeps multiple selections with their runtime", async () => {
    const user = userEvent.setup();
    renderWithI18n(<MultiHarness />);
    expect(screen.getByText(/AI agents that plan, carry out, and review/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Choose execution squads" }));
    await user.click(screen.getByRole("checkbox", { name: /Bug triage/ }));
    await waitFor(() => expect(JSON.parse(screen.getByLabelText("Saved choices").textContent!)).toEqual([
      { template_key: "feature-delivery", runtime_id: "runtime-1" },
      { template_key: "bug-triage", runtime_id: "runtime-1" },
    ]));
    expect(screen.getByRole("checkbox", { name: /Feature delivery/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Bug triage/ })).toBeChecked();
  });

  it("selects all available choices and retains an explicit empty selection", async () => {
    const user = userEvent.setup();
    renderWithI18n(<MultiHarness />);
    await user.click(screen.getByRole("button", { name: "Choose execution squads" }));
    await user.click(screen.getByRole("button", { name: "Select all" }));
    await waitFor(() => expect(JSON.parse(screen.getByLabelText("Saved choices").textContent!)).toHaveLength(3));
    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(screen.getByLabelText("Saved choices")).toHaveTextContent("[]");
    expect(screen.queryByLabelText("Runtime")).not.toBeInTheDocument();
  });

  it("preserves an unavailable saved choice until the user removes it", () => {
    mocks.templates = [];
    renderWithI18n(<MultiHarness initial={[{ template_key: "missing-template" }]} />);
    expect(screen.getByLabelText("Saved choices")).toHaveTextContent("missing-template");
    expect(screen.getByText(/This squad or template is unavailable/)).toBeInTheDocument();
  });
});
