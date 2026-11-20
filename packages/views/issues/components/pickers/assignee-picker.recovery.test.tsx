import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../../test/i18n";
import { AssigneePicker } from "./assignee-picker";

const queries = vi.hoisted(() => ({ members: vi.fn(), agents: vi.fn(), squads: vi.fn(), frequency: vi.fn() }));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: () => ({ id: "user-1" }) }));
vi.mock("@multica/core/agents", () => ({ isAgentRuntimeBound: () => true }));
vi.mock("@multica/core/permissions", () => ({ canAssignAgentToIssue: () => ({ allowed: true }) }));
vi.mock("@multica/core/workspace/hooks", () => ({ useActorName: () => ({ getActorName: () => "Ada" }) }));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: queries.members }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: queries.agents }),
  squadListOptions: () => ({ queryKey: ["squads"], queryFn: queries.squads }),
  assigneeFrequencyOptions: () => ({ queryKey: ["frequency"], queryFn: queries.frequency }),
}));
vi.mock("../../../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));

function renderPicker() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  const onUpdate = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}>
    <AssigneePicker assigneeType="member" assigneeId="user-1" open onOpenChange={() => {}} onUpdate={onUpdate} />
  </QueryClientProvider>);
  return { client, onUpdate };
}

beforeEach(() => {
  vi.resetAllMocks();
  queries.members.mockResolvedValue([{ user_id: "user-1", name: "Ada", role: "member" }]);
  queries.agents.mockResolvedValue([]);
  queries.squads.mockResolvedValue([]);
  queries.frequency.mockResolvedValue([]);
});

describe("AssigneePicker query recovery", () => {
  it("keeps failed groups distinct from empty search and retries without losing the search", async () => {
    const user = userEvent.setup();
    queries.agents.mockRejectedValueOnce(new Error("offline"));
    let finishRetry: (value: unknown[]) => void = () => {};
    queries.agents.mockImplementationOnce(() => new Promise<unknown[]>((resolve) => { finishRetry = resolve; }));
    renderPicker();
    expect(await screen.findByRole("option", { name: "Ada", selected: true })).toBeInTheDocument();
    const retry = await screen.findByRole("button", { name: "Retry Agents" });
    const search = screen.getByRole("textbox", { name: "Filter options" });
    await user.type(search, "unavailable");
    expect(screen.queryByText("No results")).not.toBeInTheDocument();
    await user.click(retry);
    expect(retry).toBeDisabled();
    expect(search).toHaveValue("unavailable");
    expect(search).toHaveFocus();
    await act(async () => finishRetry([]));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Retry Agents" })).not.toBeInTheDocument());
    expect(await screen.findByText("No results")).toBeInTheDocument();
  });

  it("retains successful options during a failed refresh and does not block on frequency failure", async () => {
    const user = userEvent.setup();
    queries.frequency.mockRejectedValue(new Error("frequency unavailable"));
    const { client, onUpdate } = renderPicker();
    const ada = await screen.findByRole("option", { name: "Ada", selected: true });
    queries.members.mockRejectedValueOnce(new Error("refresh failed"));
    await act(async () => { await client.invalidateQueries({ queryKey: ["members"] }); });
    expect(await screen.findByText("Could not refresh Members. Showing the last loaded options.")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Ada", selected: true })).toBe(ada);
    await user.click(ada);
    expect(onUpdate).toHaveBeenCalledWith({ assignee_type: "member", assignee_id: "user-1" });
  });

  it("announces a cold group's loading state instead of claiming an empty collection", async () => {
    queries.members.mockReturnValue(new Promise(() => {}));
    renderPicker();
    expect(await screen.findByText("Loading Members…")).toBeInTheDocument();
    expect(screen.queryByText("No results")).not.toBeInTheDocument();
  });
});
