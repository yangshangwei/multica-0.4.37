// @vitest-environment jsdom

import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, screen, fireEvent, waitFor } from "@testing-library/react";
import { NavigationProvider } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import type { Agent, MemberWithUser, Squad } from "@multica/core/types";
const ME = "user-me";
const OTHER = "user-other";

// Hoisted so each test can override mock return values per case.
const mocks = vi.hoisted(() => ({
  agents: [] as Agent[],
  members: [] as MemberWithUser[],
  createSquad: vi.fn(),
  addSquadMember: vi.fn(),
  navigationPush: vi.fn(),
  navigationReplace: vi.fn(),
  navigationBack: vi.fn(),
  canGoBack: false,
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  toastWarning: vi.fn(),
  invalidate: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (opts: { queryKey?: unknown[] }) => {
    const key = opts.queryKey ?? [];
    if (Array.isArray(key) && key.includes("agents")) {
      return { data: mocks.agents };
    }
    if (Array.isArray(key) && key.includes("members")) {
      return { data: mocks.members };
    }
    return { data: [] };
  },
  useQueryClient: () => ({ invalidateQueries: mocks.invalidate }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({ queryKey: ["agents"] }),
  memberListOptions: () => ({ queryKey: ["members"] }),
  workspaceKeys: { squads: (id: string) => ["squads", id] },
}));

vi.mock("@multica/core/api", () => ({
  api: {
    createSquad: (...args: unknown[]) => mocks.createSquad(...args),
    addSquadMember: (...args: unknown[]) => mocks.addSquadMember(...args),
  },
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (s: { user: { id: string } | null }) => unknown) =>
    selector({ user: { id: ME } }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    newSquad: () => "/test-ws/squads/new",
    squadDetail: (id: string) => `/test-ws/squads/${id}`,
  }),
}));

vi.mock("@multica/core/utils", () => ({
  isImeComposing: () => false,
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: ({ actorId }: { actorId: string }) => (
    <span data-testid={`avatar-${actorId}`} />
  ),
}));

vi.mock("../../common/avatar-upload-control", () => ({
  AvatarUploadControl: ({
    value,
    onUploaded,
  }: {
    value: string | null;
    onUploaded: (v: string) => void;
  }) => (
    <button
      type="button"
      data-testid="avatar-picker"
      data-value={value ?? ""}
      onClick={() => onUploaded("https://example.com/avatar.png")}
    >
      avatar
    </button>
  ),
}));

vi.mock("../../agents/components/char-counter", () => ({
  CharCounter: ({ length, max }: { length: number; max: number }) => (
    <span data-testid="char-counter">
      {length}/{max}
    </span>
  ),
}));

// Render Popover/PopoverContent inline so the picker rows are queryable
// without simulating a Base UI portal — we still drive the open prop via
// PopoverTrigger clicks but the content is always in the DOM.
vi.mock("@multica/ui/components/ui/popover", () => ({
  Popover: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  PopoverTrigger: ({
    children,
    className,
    onClick,
    render,
  }: {
    children?: ReactNode;
    className?: string;
    onClick?: () => void;
    render?: ReactNode;
  }) => {
    // Base UI's `render` prop replaces the trigger with the provided element.
    // The mock just renders it as-is so its children stay queryable in tests.
    if (render !== undefined) return <>{render}</>;
    return (
      <button type="button" className={className} onClick={onClick}>
        {children}
      </button>
    );
  },
  PopoverContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@multica/ui/components/ui/button", () => ({
  Button: ({
    children,
    disabled,
    onClick,
    type = "button",
    "aria-label": ariaLabel,
  }: {
    children: ReactNode;
    disabled?: boolean;
    onClick?: () => void;
    type?: "button" | "submit" | "reset";
    "aria-label"?: string;
  }) => (
    <button type={type} disabled={disabled} onClick={onClick} aria-label={ariaLabel}>
      {children}
    </button>
  ),
}));

vi.mock("@multica/ui/components/ui/input", () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) => (
    <input {...props} />
  ),
}));

vi.mock("@multica/ui/components/ui/label", () => ({
  Label: (props: React.LabelHTMLAttributes<HTMLLabelElement>) => (
    <label {...props} />
  ),
}));

vi.mock("../../editor/extensions/pinyin-match", () => ({
  matchesPinyin: () => false,
}));

vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => mocks.toastSuccess(...args),
    error: (...args: unknown[]) => mocks.toastError(...args),
    warning: (...args: unknown[]) => mocks.toastWarning(...args),
  },
}));

import { ManualCreateSquadPage } from "./manual-create-squad-page";

function makeAgent(overrides: Partial<Agent> & { id: string; name: string; owner_id: string | null }): Agent {
  return {
    workspace_id: "ws-1",
    runtime_id: "rt-1",
    description: "",
    instructions: "",
    avatar_url: null,
    runtime_mode: "local",
    runtime_config: {},
    custom_args: [],
    visibility: "private",
    permission_mode: "private",
    invocation_targets: [],
    status: "idle",
    max_concurrent_tasks: 1,
    model: "",
    skills: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

function makeMember(user_id: string, name: string): MemberWithUser {
  return {
    id: `m-${user_id}`,
    user_id,
    workspace_id: "ws-1",
    role: "member",
    name,
    email: `${user_id}@example.com`,
    avatar_url: null,
    created_at: "2026-01-01T00:00:00Z",
  };
}

function makeSquad(overrides: Partial<Squad> = {}): Squad {
  return {
    id: "sq-new",
    workspace_id: "ws-1",
    name: "New Squad",
    description: "",
    instructions: "",
    avatar_url: null,
    leader_id: "agent-mine-1",
    creator_id: ME,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

// "Create Squad" is both the page title and the submit button label. Always
// pick by role so tests don't depend on DOM order between the two.
function getSubmitButton(): HTMLButtonElement {
  const btn = screen
    .getAllByRole("button")
    .find((b) => b.textContent === "Create Squad");
  if (!btn) throw new Error("Create Squad submit button not found");
  return btn as HTMLButtonElement;
}

// getAllByText is typed to never return undefined slots, but the indexed
// access goes through `noUncheckedIndexedAccess` so we narrow explicitly.
function firstMatch(label: string): HTMLElement {
  const matches = screen.getAllByText(label);
  if (matches.length === 0) throw new Error(`no match for "${label}"`);
  return matches[0]!;
}

function lastMatch(label: string): HTMLElement {
  const matches = screen.getAllByText(label);
  if (matches.length === 0) throw new Error(`no match for "${label}"`);
  return matches[matches.length - 1]!;
}

function renderPage() {
  return renderWithI18n(
    <NavigationProvider value={{
      push: mocks.navigationPush,
      replace: mocks.navigationReplace,
      back: mocks.navigationBack,
      canGoBack: () => mocks.canGoBack,
      pathname: "/test-ws/squads/new/manual",
      searchParams: new URLSearchParams(),
      hash: "",
      getShareableUrl: (path) => path,
    }}>
      <ManualCreateSquadPage />
    </NavigationProvider>,
  );
}

const myAgent = makeAgent({ id: "agent-mine-1", name: "MineAgentOne", owner_id: ME });
const myAgent2 = makeAgent({ id: "agent-mine-2", name: "MineAgentTwo", owner_id: ME });
const otherAgent = makeAgent({ id: "agent-other-1", name: "OtherAgentOne", owner_id: OTHER });
const wsMember = makeMember(OTHER, "Workspace Pal");

describe("ManualCreateSquadPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.canGoBack = false;
    mocks.agents = [otherAgent, myAgent, myAgent2];
    mocks.members = [wsMember];
  });

  it("binds the name and description inputs in the identity row", () => {
    renderPage();
    const name = screen.getByPlaceholderText(/e\.g\. Frontend Team/i) as HTMLInputElement;
    fireEvent.change(name, { target: { value: "Platform Team" } });
    expect(name.value).toBe("Platform Team");

    const desc = screen.getByPlaceholderText(/Describe what this squad/i) as HTMLInputElement;
    fireEvent.change(desc, { target: { value: "We own infra" } });
    expect(desc.value).toBe("We own infra");
    // Char counter reflects the typed length.
    expect(screen.getByTestId("char-counter").textContent).toMatch(/^12\//);
  });

  it("renders the leader picker with 'My Agents' before 'Workspace Agents'", () => {
    renderPage();
    // The leader group headers come from the LeaderPicker — agent name "MineAgentOne"
    // appears under the My Agents section, "OtherAgentOne" under Workspace Agents.
    const myGroupLabels = screen.getAllByText("My Agents");
    const wsGroupLabels = screen.getAllByText("Workspace Agents");
    expect(myGroupLabels.length).toBeGreaterThanOrEqual(1);
    expect(wsGroupLabels.length).toBeGreaterThanOrEqual(1);

    // Verify ordering: the first occurrence of My Agents appears before the
    // first occurrence of Workspace Agents in the DOM (leader picker shows
    // both sections; the additional-members picker also has them, but both
    // pickers follow the same order so the assertion holds either way).
    const all = Array.from(document.querySelectorAll("*"));
    const myIdx = all.findIndex((n) => n.textContent === "My Agents");
    const wsIdx = all.findIndex((n) => n.textContent === "Workspace Agents");
    expect(myIdx).toBeLessThan(wsIdx);
  });

  it("auto-clears an additional-members entry when the same agent is picked as leader", async () => {
    renderPage();
    // Find the "MineAgentTwo" row inside the additional-members picker (the
    // leader picker also has it; clicking either toggles its respective
    // state). We pick it as an additional member via the second occurrence,
    // then promote it to leader via the first.
    // The additional-members picker's MineAgentTwo row is the LAST occurrence —
    // leader picker renders earlier in the tree.
    fireEvent.click(lastMatch("MineAgentTwo"));

    // Chip should appear (the trigger now lists MineAgentTwo).
    await waitFor(() => {
      expect(screen.getAllByText("MineAgentTwo").length).toBeGreaterThanOrEqual(2);
    });

    // Promote MineAgentTwo to leader by clicking the first occurrence.
    fireEvent.click(firstMatch("MineAgentTwo"));

    // Wire up the rest of the submit path so we can verify the sanitized
    // payload sent to addSquadMember (none — leader was the only pick).
    mocks.createSquad.mockResolvedValue(makeSquad({ leader_id: "agent-mine-2" }));
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Frontend Team/i), {
      target: { value: "Platform" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(mocks.createSquad).toHaveBeenCalledTimes(1);
    });
    // addSquadMember must NOT be called for the agent we promoted to leader.
    expect(mocks.addSquadMember).not.toHaveBeenCalled();
  });

  it("removes a member promoted to leader from selectedMembers so switching leader away does not resurrect it", async () => {
    renderPage();
    // 1. Add MineAgentTwo as an additional member.
    fireEvent.click(lastMatch("MineAgentTwo"));
    await waitFor(() => {
      expect(screen.getAllByText("MineAgentTwo").length).toBeGreaterThanOrEqual(2);
    });

    // 2. Promote MineAgentTwo to leader (first occurrence is the leader picker row).
    fireEvent.click(firstMatch("MineAgentTwo"));

    // 3. Switch leader back to MineAgentOne. With MineAgentTwo now the leader,
    //    the additional-members picker filters it out, so MineAgentOne only
    //    appears twice (leader picker + members picker) and firstMatch hits
    //    the leader picker row.
    fireEvent.click(firstMatch("MineAgentOne"));

    // 4. Submit and assert MineAgentTwo is NOT submitted as a member — the
    //    promotion must have permanently dropped it from selectedMembers.
    mocks.createSquad.mockResolvedValue(makeSquad({ id: "sq-3", leader_id: "agent-mine-1" }));
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Frontend Team/i), {
      target: { value: "Swap Squad" },
    });
    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(mocks.createSquad).toHaveBeenCalledWith({
        name: "Swap Squad",
        description: undefined,
        leader_id: "agent-mine-1",
        avatar_url: undefined,
      });
    });
    expect(mocks.addSquadMember).not.toHaveBeenCalled();
  });

  it("on success with no additional members fires exactly one success toast and navigates", async () => {
    renderPage();
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Frontend Team/i), {
      target: { value: "Solo Squad" },
    });
    // Click MineAgentOne in the leader picker (first occurrence is leader picker row).
    fireEvent.click(firstMatch("MineAgentOne"));

    mocks.createSquad.mockResolvedValue(makeSquad({ id: "sq-1", leader_id: "agent-mine-1" }));

    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(mocks.createSquad).toHaveBeenCalledWith({
        name: "Solo Squad",
        description: undefined,
        leader_id: "agent-mine-1",
        avatar_url: undefined,
      });
    });
    await waitFor(() => {
      expect(mocks.toastSuccess).toHaveBeenCalledTimes(1);
    });
    expect(mocks.addSquadMember).not.toHaveBeenCalled();
    expect(mocks.toastWarning).not.toHaveBeenCalled();
    expect(mocks.navigationPush).toHaveBeenCalledExactlyOnceWith("/test-ws/squads/sq-1");
    expect(mocks.navigationReplace).not.toHaveBeenCalled();
    expect(mocks.navigationBack).not.toHaveBeenCalled();
    expect(mocks.invalidate).toHaveBeenCalledWith({ queryKey: ["squads", "ws-1"] });
  });

  it("on success with partial member failure shows success + warning toasts and still navigates", async () => {
    renderPage();
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Frontend Team/i), {
      target: { value: "Mixed Squad" },
    });
    fireEvent.click(firstMatch("MineAgentOne"));

    // Add two additional members: the workspace pal (member) + OtherAgentOne (agent).
    // Locate them in the additional-members picker (last occurrence of each).
    fireEvent.click(lastMatch("OtherAgentOne"));
    fireEvent.click(lastMatch("Workspace Pal"));

    mocks.createSquad.mockResolvedValue(makeSquad({ id: "sq-2", leader_id: "agent-mine-1" }));
    mocks.addSquadMember
      .mockResolvedValueOnce({}) // first call succeeds
      .mockRejectedValueOnce(new Error("boom")); // second fails

    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(mocks.createSquad).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(mocks.addSquadMember).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(mocks.toastSuccess).toHaveBeenCalledTimes(1);
    });
    expect(mocks.toastWarning).toHaveBeenCalledTimes(1);
    expect(mocks.navigationPush).toHaveBeenCalledExactlyOnceWith("/test-ws/squads/sq-2");
  });

  it("on createSquad failure shows an error toast, does not navigate, and re-enables submit", async () => {
    renderPage();
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Frontend Team/i), {
      target: { value: "Boom Squad" },
    });
    fireEvent.click(firstMatch("MineAgentOne"));

    mocks.createSquad.mockRejectedValueOnce(new Error("server down"));

    fireEvent.click(getSubmitButton());

    await waitFor(() => {
      expect(mocks.toastError).toHaveBeenCalledTimes(1);
    });
    expect(mocks.navigationPush).not.toHaveBeenCalled();
    // Submit button is re-enabled (textContent reads "Create Squad" again,
    // not "Creating...").
    const button = getSubmitButton();
    expect(button.disabled).toBe(false);
  });

  it("renders a page with labelled identity fields", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: "Create Squad", level: 1 })).toBeVisible();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Name" })).toBeVisible();
    expect(screen.getByRole("textbox", { name: "Description" })).toBeVisible();
  });

  it.each(["Cancel", "Go back"])("%s returns a direct-link visit to the chooser", (name) => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name }));
    expect(mocks.navigationReplace).toHaveBeenCalledExactlyOnceWith("/test-ws/squads/new");
    expect(mocks.navigationPush).not.toHaveBeenCalled();
    expect(mocks.navigationBack).not.toHaveBeenCalled();
    expect(mocks.createSquad).not.toHaveBeenCalled();
  });

  it("uses the existing history for the return action", () => {
    mocks.canGoBack = true;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Go back" }));
    expect(mocks.navigationBack).toHaveBeenCalledTimes(1);
    expect(mocks.navigationReplace).not.toHaveBeenCalled();
  });

  it("awaits creation and disables both return actions before navigating once", async () => {
    let complete!: (squad: Squad) => void;
    mocks.createSquad.mockReturnValueOnce(new Promise<Squad>((resolve) => { complete = resolve; }));
    renderPage();
    fireEvent.change(screen.getByRole("textbox", { name: "Name" }), {
      target: { value: "Pending Squad" },
    });
    fireEvent.click(firstMatch("MineAgentOne"));
    fireEvent.click(getSubmitButton());
    expect(screen.getByRole("button", { name: "Creating..." })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Go back" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(mocks.navigationPush).not.toHaveBeenCalled();
    await act(async () => { complete(makeSquad({ id: "pending-squad" })); });
    expect(mocks.navigationPush).toHaveBeenCalledExactlyOnceWith("/test-ws/squads/pending-squad");
    expect(mocks.navigationReplace).not.toHaveBeenCalled();
    expect(mocks.navigationBack).not.toHaveBeenCalled();
  });

});
