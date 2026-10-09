import { forwardRef, useImperativeHandle, useRef } from "react";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@multica/ui/components/ui/dropdown-menu";
import { DRAFT_NEW_SESSION, useChatStore } from "@multica/core/chat";
import { chatKeys } from "@multica/core/chat/queries";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { projectKeys } from "@multica/core/projects/queries";
import { renderWithI18n } from "../../test/i18n";
import { ChatWindow } from "./chat-window";
import { ChatFab } from "./chat-fab";

vi.mock("@multica/core/chat", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/chat")>();
  return {
    ...actual,
    useChatStore: actual.createChatStore({ storage: { getItem: () => null, setItem: () => {}, removeItem: () => {} } }),
  };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("@multica/core/agents", () => ({
  isAgentRuntimeBound: () => true,
  useAgentPresenceDetail: () => ({ availability: "online" }),
  useCustomizeConversationStartersHref: () => undefined,
  useWorkspaceAgentAvailability: () => "available",
}));
vi.mock("@multica/views/issues/components", () => ({ canAssignAgent: () => true }));
vi.mock("@multica/core/shortcuts", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/shortcuts")>(),
  useShortcut: () => null,
}));
vi.mock("@multica/core/logger", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/logger")>();
  return { ...actual, createLogger: () => actual.noopLogger };
});
vi.mock("./use-chat-project-context-support", () => ({ useChatProjectContextSupport: () => true }));
vi.mock("./use-chat-context-items", () => ({ useChatContextItems: () => [] }));
vi.mock("./chat-message-list", () => ({ ChatMessageList: () => null, ChatMessageSkeleton: () => null }));
vi.mock("./chat-empty-state", () => ({ EmptyState: () => <p>Start a conversation</p> }));
vi.mock("./new-chat-button", () => ({ AgentPicker: () => <button type="button">Choose an agent</button> }));
vi.mock("../../editor", async () => ({
  ...await vi.importActual<typeof import("../../editor/use-upload-gate")>("../../editor/use-upload-gate"),
  ...await vi.importActual<typeof import("../../editor/use-composer-submit")>("../../editor/use-composer-submit"),
  useFileDropZone: () => ({ isDragOver: false, dropZoneProps: {} }),
  FileDropOverlay: () => null,
  // Tiptap has its own suite. Keep the actual ChatInput, draft store, portal
  // controls and focus handoff, with a native editor whose identity is visible.
  ContentEditor: forwardRef(function Editor({ value, onUpdate, showBubbleMenu, isVisible = true }: { value: string; onUpdate: (text: string) => void; showBubbleMenu?: boolean; isVisible?: boolean }, forwardedRef) {
    const ref = useRef<HTMLTextAreaElement>(null);
    useImperativeHandle(forwardedRef, () => ({
      focus: () => ref.current?.focus(),
      blur: () => ref.current?.blur(),
      getMarkdown: () => ref.current?.value ?? "",
      hasActiveUploads: () => false,
      flushPendingUpdate: () => null,
      adoptContent: (text: string) => { if (ref.current) ref.current.value = text; },
    }));
    return <>
      <textarea ref={ref} aria-label="Chat message" defaultValue={value} onChange={(event) => onUpdate(event.target.value)} />
      {isVisible && showBubbleMenu && <DropdownMenu>
        <DropdownMenuTrigger>Heading styles</DropdownMenuTrigger>
        <DropdownMenuContent><DropdownMenuItem>Normal text</DropdownMenuItem></DropdownMenuContent>
      </DropdownMenu>}
    </>;
  }),
}));
vi.mock("./use-chat-resize", () => ({
  useChatResize: () => ({ renderWidth: 480, renderHeight: 640, isAtMax: false, boundsReady: true, isDragging: false, toggleExpand: vi.fn(), startDrag: vi.fn() }),
}));

const clients: QueryClient[] = [];
const initialStore = useChatStore.getState();

function renderChat(withProject = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  clients.push(client);
  client.setQueryData(workspaceKeys.agents("ws-1"), [{ id: "agent-1", name: "Multica", runtime_id: "runtime-1", owner_id: "user-1", archived_at: null }]);
  client.setQueryData(workspaceKeys.members("ws-1"), []);
  const projects = withProject ? [{ id: "project-1", title: "Launch Plan", icon: null, status: "in_progress" }] : [];
  client.setQueryData(projectKeys.list("ws-1"), { projects });
  if (withProject) useChatStore.setState({ selectedProjectId: "project-1" });
  client.setQueryData(chatKeys.sessions("ws-1"), []);
  client.setQueryData(chatKeys.pendingTasks("ws-1"), { tasks: [] });
  client.setQueryData(chatKeys.pendingTasksHasAny("ws-1"), { has_pending: false });
  return renderWithI18n(
    <QueryClientProvider client={client}>
      <button type="button">Page action</button>
      <ChatWindow />
      <ChatFab />
    </QueryClientProvider>,
  );
}

beforeEach(() => { useChatStore.setState(initialStore, true); });
afterEach(() => { for (const client of clients.splice(0)) client.clear(); });

describe("ChatWindow visibility and focus", () => {
  // Focus transition and readiness cases live in use-chat-input-focus.test.ts.
  it("keeps the closed composer mounted, inert, and out of the accessibility tree", () => {
    const { container } = renderChat();
    const composer = container.querySelector("textarea");
    expect(composer).not.toBeNull();
    expect(composer?.closest("[inert]")).not.toBeNull();
    expect(composer?.closest('[aria-hidden="true"]')).not.toBeNull();
    expect(screen.queryByRole("textbox", { name: "Chat message" })).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog", { name: "Chat" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ask Multica" })).toBeEnabled();
  });

  it("opens on the composer and restores the launcher without losing a draft", async () => {
    const user = userEvent.setup();
    renderChat();
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    const dialog = screen.getByRole("dialog", { name: "Chat" });
    const composer = within(dialog).getByRole("textbox", { name: "Chat message" });
    expect(composer).toHaveFocus();
    await user.type(composer, "Unsent chat draft");
    await user.click(within(dialog).getByRole("button", { name: "Minimize" }));
    expect(screen.queryByRole("dialog", { name: "Chat" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ask Multica" })).toHaveFocus();
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    expect(screen.getByRole("textbox", { name: "Chat message" })).toBe(composer);
    expect(composer).toHaveValue("Unsent chat draft");
    expect(useChatStore.getState().inputDrafts[DRAFT_NEW_SESSION]).toBe("Unsent chat draft");
    expect(composer).toHaveFocus();
  });

  it("dismisses an open history popup and returns focus when chat closes from a portal", async () => {
    const user = userEvent.setup();
    renderChat();
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    await user.click(screen.getByRole("button", { name: "New chat", expanded: false }));
    const emptyHistory = await screen.findByText("No previous chats");
    const popup = emptyHistory.closest<HTMLElement>('[data-slot="popover-content"]');
    act(() => { popup?.focus(); });
    expect(popup).toHaveFocus();
    act(() => { useChatStore.getState().setOpen(false); });
    await waitFor(() => expect(screen.queryByText("No previous chats")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Ask Multica" })).toHaveFocus();
    expect(screen.queryByRole("dialog", { name: "Chat" })).not.toBeInTheDocument();
  });

  it("closes the composer add menu without unmounting the editor or restoring a hidden trigger", async () => {
    const user = userEvent.setup();
    renderChat();
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    const composer = screen.getByRole("textbox", { name: "Chat message" });
    await user.type(composer, "Keep this draft");
    await user.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByRole("menuitem", { name: "Image or files" })).toBeInTheDocument();
    act(() => { useChatStore.getState().setOpen(false); });
    await waitFor(() => expect(screen.queryByRole("menuitem", { name: "Image or files" })).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Ask Multica" })).toHaveFocus();
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    expect(screen.getByRole("textbox", { name: "Chat message" })).toBe(composer);
    expect(composer).toHaveValue("Keep this draft");
  });

  it("dismisses the project picker while retaining its context and composer", async () => {
    const user = userEvent.setup();
    renderChat(true);
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    const composer = screen.getByRole("textbox", { name: "Chat message" });
    await user.type(composer, "Project draft");
    await user.click(screen.getByRole("button", { name: "Change project context" }));
    expect(await screen.findByPlaceholderText("Search projects...")).toBeInTheDocument();
    act(() => { useChatStore.getState().setOpen(false); });
    await waitFor(() => expect(screen.queryByPlaceholderText("Search projects...")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Ask Multica" })).toHaveFocus();
    expect(useChatStore.getState().selectedProjectId).toBe("project-1");
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    expect(screen.getByRole("textbox", { name: "Chat message" })).toBe(composer);
    expect(composer).toHaveValue("Project draft");
  });

  it("dismisses the retained editor's formatting popup without losing its draft", async () => {
    const user = userEvent.setup();
    renderChat();
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    const composer = screen.getByRole("textbox", { name: "Chat message" });
    await user.type(composer, "Formatted draft");
    await user.click(screen.getByRole("button", { name: "Heading styles" }));
    expect(await screen.findByRole("menuitem", { name: "Normal text" })).toBeInTheDocument();
    act(() => { useChatStore.getState().setOpen(false); });
    await waitFor(() => expect(screen.queryByRole("menuitem", { name: "Normal text" })).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Ask Multica" })).toHaveFocus();
    await user.click(screen.getByRole("button", { name: "Ask Multica" }));
    expect(screen.getByRole("textbox", { name: "Chat message" })).toBe(composer);
    expect(composer).toHaveValue("Formatted draft");
  });
});
