import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { chatKeys } from "@multica/core/chat/queries";
import { squadListOptions } from "@multica/core/workspace/queries";
import { api } from "@multica/core/api";
import type { ChatPinnedAgent, ChatSession } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Agent } from "@multica/core/types";
import enChat from "../../locales/en/chat.json";
import enIssues from "../../locales/en/issues.json";
import enModals from "../../locales/en/modals.json";
import { toast } from "sonner";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/api", () => ({ api: {
  listChatPinnedAgents: vi.fn(async () => []),
  pinChatAgent: vi.fn(async () => undefined),
  unpinChatAgent: vi.fn(async () => undefined),
} }));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: ({ actorId }: { actorId: string }) => (
    <span data-testid={`avatar-${actorId}`} />
  ),
}));

import { AgentPicker, NewChatButton } from "./new-chat-button";

const TEST_RESOURCES = { en: { chat: enChat, issues: enIssues, modals: enModals } };

function makeAgent(overrides: Partial<Agent> & Pick<Agent, "id" | "name" | "owner_id">): Agent {
  return {
    workspace_id: "ws-1",
    runtime_id: "runtime-1",
    description: "",
    instructions: "",
    avatar_url: null,
    runtime_mode: "local",
    runtime_config: {},
    custom_args: [],
    visibility: "workspace",
    permission_mode: "public_to",
    invocation_targets: [{ target_type: "workspace", target_id: null }],
    status: "idle",
    max_concurrent_tasks: 1,
    model: "sonnet",
    skills: [],
    created_at: new Date(0).toISOString(),
    updated_at: new Date(0).toISOString(),
    archived_at: null,
    archived_by: null,
    ...overrides,
    id: overrides.id,
    name: overrides.name,
    owner_id: overrides.owner_id,
  };
}

const agents = [
  makeAgent({ id: "mine-alpha", name: "Alpha", owner_id: "user-1" }),
  makeAgent({ id: "mine-zhang", name: "张三", owner_id: "user-1" }),
  makeAgent({ id: "other-beta", name: "Beta", owner_id: "user-2", description: "Review database migrations" }),
  makeAgent({ id: "other-gamma", name: "Gamma", owner_id: "user-2" }),
];

// The ⊕ button carries the localized "New chat" label as its accessible name.
const NEW_CHAT_LABEL = enChat.window.new_chat_tooltip;

function renderChat(content: React.ReactNode, pins: ChatPinnedAgent[] = [], sessions: Partial<ChatSession>[] = []) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  client.setQueryData(chatKeys.pinnedAgents("ws-1"), pins);
  client.setQueryData(chatKeys.sessions("ws-1"), sessions);
  client.setQueryData(squadListOptions("ws-1").queryKey, [
    { id: "squad-1", workspace_id: "ws-1", name: "Delivery", description: "Deliver features", leader_id: "other-beta",
      instructions: "", avatar_url: null, creator_id: "user-1", created_at: "2026-09-01", updated_at: "2026-09-01",
      archived_at: null, archived_by: null },
  ]);
  render(<QueryClientProvider client={client}>
    <I18nProvider locale="en" resources={TEST_RESOURCES}>{content}</I18nProvider>
  </QueryClientProvider>);
  return client;
}

function renderPicker(onStart = vi.fn(), directory = agents) {
  renderChat(<NewChatButton agents={directory} userId="user-1" onStart={onStart} />);
  fireEvent.click(screen.getByRole("button", { name: NEW_CHAT_LABEL }));
  return { onStart };
}

beforeEach(() => vi.clearAllMocks());

describe("NewChatButton", () => {
  it("searches saved responsibilities and exposes the shared discovery categories", async () => {
    const { onStart } = renderPicker();
    const input = await screen.findByPlaceholderText("Search names or responsibilities...");
    expect(screen.getByRole("button", { name: "Planning and coordination" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "AI squads" })).not.toBeInTheDocument();
    expect(screen.queryByText(enModals.create_issue.actor_picker.creator_hint)).not.toBeInTheDocument();
    fireEvent.change(input, { target: { value: "migrations" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onStart).toHaveBeenCalledWith(agents[2]);
  });

  it("opens the agent picker below the ⊕ trigger", async () => {
    renderPicker();

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveAttribute("data-side", "bottom");
  });

  it("searches the full agent directory by name", async () => {
    renderPicker();

    const input = await screen.findByRole("textbox", { name: "Filter options" });
    fireEvent.change(input, { target: { value: "ta" } });
    const dialog = screen.getByRole("dialog");

    expect(within(dialog).queryByText("Alpha")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("张三")).not.toBeInTheDocument();
    expect(within(dialog).getByText("Beta")).toBeInTheDocument();
    expect(within(dialog).queryByText("Gamma")).not.toBeInTheDocument();
  });

  it("matches agent names by pinyin", async () => {
    renderPicker();

    const input = await screen.findByRole("textbox", { name: "Filter options" });
    fireEvent.change(input, { target: { value: "zhang" } });
    const dialog = screen.getByRole("dialog");

    expect(within(dialog).getByText("张三")).toBeInTheDocument();
    expect(within(dialog).queryByText("Alpha")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("Beta")).not.toBeInTheDocument();
  });

  it("shows the shared empty state when no agents match", async () => {
    renderPicker();

    const input = await screen.findByRole("textbox", { name: "Filter options" });
    fireEvent.change(input, { target: { value: "missing" } });

    expect(screen.getByText("No matching actors")).toBeInTheDocument();
    expect(screen.queryByText("My agents")).not.toBeInTheDocument();
    expect(screen.queryByText("Others")).not.toBeInTheDocument();
  });

  it("pre-checks no agent (a new chat has no current) and reports the chosen agent", async () => {
    const { onStart } = renderPicker();

    const dialog = screen.getByRole("dialog");
    // No agent should carry a visible check mark for a fresh new chat.
    const alphaRow = within(dialog).getByText("Alpha").closest("button");
    expect(alphaRow).not.toBeNull();
    expect(alphaRow!.querySelector("svg:not(.invisible)")).toBeNull();

    fireEvent.click(within(dialog).getByText("Beta"));

    expect(onStart).toHaveBeenCalledWith(agents[2]);
    await waitFor(() => {
      expect(screen.queryByRole("textbox", { name: "Filter options" })).not.toBeInTheDocument();
    });
  });

  it("starts immediately without a picker when only one agent exists", () => {
    const onStart = vi.fn();
    renderChat(<NewChatButton agents={[agents[0]!]} userId="user-1" onStart={onStart} />);

    fireEvent.click(screen.getByRole("button", { name: NEW_CHAT_LABEL }));

    expect(onStart).toHaveBeenCalledWith(agents[0]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps runtime-unbound agents disabled while bound offline agents remain selectable", async () => {
    const offline = makeAgent({ id: "offline", name: "Offline", owner_id: "user-1", status: "offline" });
    const unbound = makeAgent({ id: "unbound", name: "Unbound", owner_id: "user-1", runtime_id: undefined });
    const { onStart } = renderPicker(vi.fn(), [unbound, offline]);
    expect(await screen.findByRole("button", { name: /Unbound Agent/ })).toBeDisabled();
    fireEvent.click(screen.getByText("Unbound"));
    expect(onStart).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("Offline"));
    expect(onStart).toHaveBeenCalledWith(offline);
  });

  it("shows leaders in planning and selects the agent rather than its squad", async () => {
    const { onStart } = renderPicker();
    fireEvent.click(await screen.findByRole("button", { name: "Planning and coordination" }));
    expect(screen.getByText("Leads: Delivery")).toBeInTheDocument();
    expect(screen.queryByText("Alpha")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Beta"));
    expect(onStart).toHaveBeenCalledWith(agents[2]);
  });

  it("uses chat pins and activity recents without exposing creation defaults", async () => {
    renderChat(<NewChatButton agents={agents} userId="user-1" onStart={vi.fn()} />, [
      { agent_id: "mine-alpha", position: 1 },
    ], [
      { agent_id: "other-gamma", status: "active", pinned: true, updated_at: "2026-09-01" },
      { agent_id: "other-beta", status: "active", updated_at: "2026-10-01" },
      { agent_id: "other-beta", status: "active", updated_at: "2026-09-30" },
      { agent_id: "mine-zhang", status: "archived", updated_at: "2026-10-02" },
    ]);
    fireEvent.click(screen.getByRole("button", { name: NEW_CHAT_LABEL }));
    expect(await screen.findByRole("heading", { name: "Favorites" })).toBeInTheDocument();
    const recent = screen.getByRole("heading", { name: "Recently used" }).closest("section")!;
    expect(within(recent).getAllByRole("button").filter((item) => item.hasAttribute("data-picker-item"))
      .map((item) => item.textContent)).toEqual([expect.stringContaining("Beta"), expect.stringContaining("Gamma")]);
    expect(screen.queryByText("张三")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Set current as default" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Unpin Alpha from favorites" }));
    await waitFor(() => expect(api.unpinChatAgent).toHaveBeenCalledWith("mine-alpha"));
  });

  it("pins through the existing chat mutation and leaves the picker open", async () => {
    renderPicker();
    fireEvent.click(await screen.findByRole("button", { name: "Pin Alpha to favorites" }));
    await waitFor(() => expect(api.pinChatAgent).toHaveBeenCalledWith("mine-alpha"));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("keeps shortcuts and keyboard focus while a pin request is pending", async () => {
    let finishPin!: () => void;
    vi.mocked(api.pinChatAgent).mockImplementationOnce(() => new Promise<ChatPinnedAgent>((resolve) => {
      finishPin = () => resolve({ agent_id: "other-beta", position: 2 });
    }));
    renderChat(<NewChatButton agents={agents} userId="user-1" onStart={vi.fn()} />,
      [{ agent_id: "mine-alpha", position: 1 }],
      [{ agent_id: "other-beta", status: "active", updated_at: "2026-10-01" }]);
    fireEvent.click(screen.getByRole("button", { name: NEW_CHAT_LABEL }));
    const pinButton = await screen.findByRole("button", { name: "Pin Beta to favorites" });
    pinButton.focus();
    fireEvent.click(pinButton);
    await waitFor(() => expect(api.pinChatAgent).toHaveBeenCalledWith("other-beta"));
    expect(screen.getByRole("heading", { name: "Favorites" })).toBeInTheDocument();
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toContainElement(document.activeElement as HTMLElement);
    finishPin();
    await waitFor(() => expect(screen.getByRole("button", { name: "Pin Beta to favorites" })).toBeEnabled());
  });

  it("reports failed pin updates and rolls back the favorite", async () => {
    const notify = vi.spyOn(toast, "error");
    vi.mocked(api.pinChatAgent).mockRejectedValueOnce(new Error("Unavailable"));
    renderPicker();
    fireEvent.click(await screen.findByRole("button", { name: "Pin Alpha to favorites" }));
    await waitFor(() => expect(notify).toHaveBeenCalledWith(enChat.window.pin_failed));
    expect(await screen.findByRole("button", { name: "Pin Alpha to favorites" })).toBeEnabled();
    notify.mockRestore();
  });

  it("honors the server's five-pin limit including pins outside the visible directory", async () => {
    renderChat(<NewChatButton agents={agents} userId="user-1" onStart={vi.fn()} />,
      ["mine-alpha", "hidden-1", "hidden-2", "hidden-3", "hidden-4"].map((agent_id, position) => ({ agent_id, position })));
    fireEvent.click(screen.getByRole("button", { name: NEW_CHAT_LABEL }));
    fireEvent.click(await screen.findByRole("button", { name: "Browse all (4)" }));
    expect(screen.getByRole("button", { name: "Pin Beta to favorites" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Unpin Alpha from favorites" })).toBeEnabled();
    expect(screen.getByText("Beta").closest("button")).toBeEnabled();
  });

  it("highlights the current agent and preserves top placement for switching", async () => {
    const onSelect = vi.fn();
    renderChat(<AgentPicker agents={agents} userId="user-1" currentAgentId="mine-alpha" onSelect={onSelect}
      side="top" trigger="Switch agent" triggerRender={<button type="button" />} />);
    fireEvent.click(screen.getByRole("button", { name: "Switch agent" }));
    expect(await screen.findByRole("dialog")).toHaveAttribute("data-side", "top");
    expect(screen.getByRole("button", { name: /Alpha Agent.*Selected/ })).toBeInTheDocument();
    fireEvent.click(screen.getByText("Beta"));
    expect(onSelect).toHaveBeenCalledWith(agents[2]);
  });

  it("keeps the no-agent shortcut", () => {
    const onStart = vi.fn();
    renderChat(<NewChatButton agents={[]} userId="user-1" onStart={onStart} />);
    fireEvent.click(screen.getByRole("button", { name: NEW_CHAT_LABEL }));
    expect(onStart).toHaveBeenCalledWith(null);
  });

  it("blocks the unbound single-agent shortcut", () => {
    const onStart = vi.fn();
    const unbound = makeAgent({ id: "unbound", name: "Unbound", owner_id: "user-1", runtime_id: undefined });
    renderChat(<NewChatButton agents={[unbound]} userId="user-1" onStart={onStart} />);
    fireEvent.click(screen.getByRole("button", { name: NEW_CHAT_LABEL }));
    expect(onStart).not.toHaveBeenCalled();
  });
});
