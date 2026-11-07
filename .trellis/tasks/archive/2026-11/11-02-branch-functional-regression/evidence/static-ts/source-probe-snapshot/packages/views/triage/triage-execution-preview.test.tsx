import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import type {
  Agent,
  RuntimeDevice,
  ProjectResource,
  Squad,
  TriageItem,
  TriageSettings,
} from "@multica/core/types";
import en from "../locales/en/triage.json";
import { TriageActionDialog } from "./triage-action-dialog";

const state = vi.hoisted(() => ({
  agents: [] as Agent[],
  squads: [] as Squad[],
  runtimes: [] as RuntimeDevice[],
  resources: [] as ProjectResource[],
  resourceLoading: false,
  resourceError: false,
  mutate: vi.fn(),
}));
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (
      selector: (value: typeof en) => string,
      values?: Record<string, string>,
    ) =>
      selector(en).replace(
        /{{(\w+)}}/g,
        (_, key: string) => values?.[key] ?? key,
      ),
    i18n: { language: "en" },
  }),
}));
vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ slug: "acme" }),
}));
vi.mock("@multica/core/permissions", async (original) => ({
  ...(await original<typeof import("@multica/core/permissions")>()),
  useCurrentMember: () => ({
    userId: "user-1",
    role: "member",
    isLoading: false,
  }),
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  useTriageAction: () => ({ mutateAsync: state.mutate, isPending: false }),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: ({
    queryKey,
    enabled,
  }: {
    queryKey: readonly unknown[];
    enabled?: boolean;
  }) => {
    if (enabled === false)
      return {
        data: undefined,
        isPending: true,
        isFetching: false,
        refetch: vi.fn(),
      };
    const key = queryKey.join("/");
    if (key.endsWith("resources"))
      return {
        data: state.resourceLoading ? undefined : state.resources,
        isPending: state.resourceLoading,
        isError: state.resourceError,
        refetch: vi.fn(),
      };
    if (key.startsWith("projects/"))
      return {
        data: { id: "project-1", workspace_id: "ws", title: "Release project" },
        isSuccess: true,
        refetch: vi.fn(),
      };
    if (key.startsWith("runtimes/"))
      return { data: state.runtimes, isSuccess: true, refetch: vi.fn() };
    if (key.endsWith("agents"))
      return { data: state.agents, isSuccess: true, refetch: vi.fn() };
    if (key.endsWith("squads"))
      return { data: state.squads, isSuccess: true, refetch: vi.fn() };
    return { refetch: vi.fn() };
  },
}));
vi.mock("../modals/issue-picker-modal", () => ({
  IssuePickerModal: () => null,
}));
vi.mock("./triage-fields", () => ({
  TriageField: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ReviewerSelect: () => null,
  TriageFieldsEditor: () => null,
}));
const item = {
  issue: {
    id: "issue-1",
    identifier: "MUL-1",
    title: "Pending task",
    revision: 7,
    admission_status: "pending",
    priority: "high",
  },
  candidate_project_id: "project-1",
  candidate_assignee_id: "agent-1",
  candidate_assignee_type: "agent",
  reviewer_id: null,
} as TriageItem;
const settings: TriageSettings = {
  supported: true,
  enabled: true,
  acceptance_status: "todo",
  require_priority: true,
  responsibility_mode: "none",
  responsibility_member_id: null,
  revision: 1,
};
function renderDialog(override: Partial<TriageItem> = {}) {
  return render(
    <TriageActionDialog
      wsId="ws"
      item={{ ...item, ...override }}
      action="accept_and_execute"
      settings={settings}
      onClose={vi.fn()}
      onSuccess={vi.fn()}
    />,
  );
}
beforeEach(() => {
  state.mutate.mockReset();
  state.resourceLoading = false;
  state.resourceError = false;
  state.squads = [];
  state.agents = [
    {
      id: "agent-1",
      workspace_id: "ws",
      name: "Release agent",
      runtime_id: "runtime-1",
      archived_at: null,
      owner_id: "user-2",
      permission_mode: "public_to",
      invocation_targets: [{ target_type: "workspace", target_id: null }],
      runtime_availability: "online",
    } as Agent,
  ];
  state.runtimes = [
    {
      id: "runtime-1",
      workspace_id: "ws",
      name: "Build machine",
      custom_name: null,
      provider: "codex",
      status: "online",
      owner_id: "user-2",
      visibility: "private",
      daemon_id: "daemon-1",
      metadata: {},
    } as RuntimeDevice,
  ];
  state.resources = [
    {
      id: "resource-1",
      workspace_id: "ws",
      project_id: "project-1",
      resource_type: "github_repo",
      label: "Main repository",
      resource_ref: { url: "https://github.com/acme/release", ref: "main" },
    } as ProjectResource,
  ];
});
it("shows the named executor, runtime, project and resources without executing or changing pending admission", () => {
  renderDialog();
  expect(screen.queryByText(en.accept_hint)).not.toBeInTheDocument();
  expect(screen.getByText(en.execute_hint)).toBeVisible();
  expect(screen.getByText("Release agent")).toBeVisible();
  expect(screen.getByText("Release project")).toBeVisible();
  expect(screen.getByText("Main repository")).toBeVisible();
  expect(screen.getByText(/https:\/\/github.com\/acme\/release/)).toBeVisible();
  expect(
    screen.getByRole("button", { name: "Accept and execute" }),
  ).toBeEnabled();
  expect(state.mutate).not.toHaveBeenCalled();
});
it("blocks confirmation when invocation is denied even if the runtime is online", () => {
  state.agents[0]!.permission_mode = "private";
  renderDialog();
  const confirm = screen.getByRole("button", { name: "Accept and execute" });
  expect(confirm).toBeDisabled();
  fireEvent.click(confirm);
  expect(state.mutate).not.toHaveBeenCalled();
});
it.each(["offline", "missing", "loading", "error"])(
  "blocks confirmation for unavailable or unresolved prerequisites: %s",
  (failure) => {
    if (failure === "offline") state.runtimes[0]!.status = "offline";
    if (failure === "missing") state.agents = [];
    if (failure === "loading") state.resourceLoading = true;
    if (failure === "error") state.resourceError = true;
    renderDialog();
    expect(
      screen.getByRole("button", { name: "Accept and execute" }),
    ).toBeDisabled();
    expect(state.mutate).not.toHaveBeenCalled();
  },
);

it("uses the selected squad's leader for invocation and execution confirmation", () => {
  state.squads = [
    {
      id: "squad-1",
      workspace_id: "ws",
      name: "Release squad",
      leader_id: "agent-1",
      archived_at: null,
    } as Squad,
  ];
  renderDialog({
    candidate_assignee_type: "squad",
    candidate_assignee_id: "squad-1",
  });
  expect(screen.queryByText(en.accept_hint)).not.toBeInTheDocument();
  expect(screen.getByText(en.execute_hint)).toBeVisible();
  expect(screen.getByText("Release agent")).toBeVisible();
  expect(screen.getByText("Leader of Release squad")).toBeVisible();
  expect(
    screen.getByRole("button", { name: "Accept and execute" }),
  ).toBeEnabled();
});
it("uses coarse availability for a hidden runtime without requiring runtime binding permission", () => {
  state.runtimes = [];
  renderDialog();
  expect(screen.getByText("Runtime managed by the agent owner")).toBeVisible();
  expect(
    screen.getByRole("button", { name: "Accept and execute" }),
  ).toBeEnabled();
  expect(state.mutate).not.toHaveBeenCalled();
});
