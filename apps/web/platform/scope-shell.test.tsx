import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useScopeHistoryStore } from "@multica/core/navigation";
import { WebScopeShell } from "./scope-shell";

const state = vi.hoisted(() => ({
  owner: "server:viewer" as string | null,
  navigation: { pathname: "/second/projects/p1", searchParams: new URLSearchParams("tab=overview"), hash: "#description", push: vi.fn() },
  list: { ready: true, workspaces: [{ slug: "first" }, { slug: "second" }], unavailable: false, isFetching: false, refetch: vi.fn() },
  resume: vi.fn(),
}));
vi.mock("./navigation-owner", () => ({ useWebNavigationOwner: () => state.owner }));
vi.mock("./navigation", () => ({ useWebScopeResume: () => state.resume }));
vi.mock("@multica/core/workspace", () => ({ useWorkspaceList: () => state.list }));
vi.mock("@multica/views/navigation", async (original) => ({ ...await original<typeof import("@multica/views/navigation")>(), useNavigation: () => state.navigation }));
vi.mock("@multica/views/i18n", () => ({ useT: () => ({ t: (selector: (value: unknown) => string) => selector({ sidebar: { create_workspace: "Create workspace" }, page: { refresh: "Refresh" } }) }) }));
// Shared tests own switch semantics; this suite owns its Web wiring.
vi.mock("@multica/views/layout", () => ({ GlobalScopeNavigation: ({ onWorkspace, onCollaboration, children }: { onWorkspace: () => void; onCollaboration: () => void; children: React.ReactNode }) => <nav aria-label="Application navigation"><button onClick={onWorkspace}>Workspaces</button><button onClick={onCollaboration}>Collaboration</button>{children}</nav> }));

function location(path: string) {
  const url = new URL(path, "https://test.invalid");
  state.navigation.pathname = url.pathname;
  state.navigation.searchParams = url.searchParams;
  state.navigation.hash = url.hash;
}
const page = () => <WebScopeShell><main>Content</main></WebScopeShell>;

beforeEach(() => {
  useScopeHistoryStore.getState().setOwner(null);
  state.owner = "server:viewer";
  state.list = { ready: true, workspaces: [{ slug: "first" }, { slug: "second" }], unavailable: false, isFetching: false, refetch: vi.fn() };
  location("/second/projects/p1?tab=overview#description");
  state.resume.mockReset();
  state.navigation.push.mockReset();
});

describe("WebScopeShell", () => {
  it("resumes both full locations and leaves the selected scope alone", () => {
    const workspace = "/second/projects/p1?tab=overview#description";
    const account = "/account?view=participating&issue=00000000-0000-4000-8000-000000000001#memory";
    const view = render(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Collaboration" }));
    expect(state.resume).toHaveBeenLastCalledWith("/account?view=current");
    location(account);
    view.rerender(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenLastCalledWith(workspace);
    location(workspace);
    view.rerender(page());
    fireEvent.click(screen.getByRole("button", { name: "Collaboration" }));
    expect(state.resume).toHaveBeenLastCalledWith(account);
  });

  it("keeps account content and navigation available with no workspaces", () => {
    location("/account");
    state.list.workspaces = [];
    render(page());
    expect(screen.getByText("Content")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenCalledWith("/workspaces/new");
    fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));
    expect(state.navigation.push).toHaveBeenCalledWith("/workspaces/new");
  });

  it("falls back from a removed space using the authoritative directory", () => {
    const view = render(page());
    location("/account");
    state.list.workspaces = [{ slug: "first" }];
    view.rerender(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenCalledWith("/first/issues");
  });

  it("fetches unknown membership on demand and retains account content after failure", async () => {
    location("/account");
    state.list.ready = false;
    state.list.unavailable = true;
    state.list.workspaces = [];
    state.list.refetch.mockResolvedValue({ data: undefined });
    render(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    await waitFor(() => expect(state.list.refetch).toHaveBeenCalled());
    expect(state.resume).not.toHaveBeenCalled();
    expect(screen.getByText("Content")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Refresh" })).toBeInTheDocument();
  });

  it("ignores a late workspace fetch after the user moves elsewhere", async () => {
    location("/account");
    state.list.ready = false;
    state.list.workspaces = [];
    let finish!: (value: { data: { slug: string }[] }) => void;
    state.list.refetch.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const view = render(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    location("/account?view=participating");
    view.rerender(page());
    await act(async () => finish({ data: [{ slug: "first" }] }));
    expect(state.resume).not.toHaveBeenCalled();
  });

  it("does not show application chrome during authentication or one-shot flows", () => {
    state.owner = null;
    const view = render(page());
    expect(screen.queryByRole("navigation")).toBeNull();
    state.owner = "server:viewer";
    location("/workspaces/new");
    view.rerender(page());
    expect(screen.queryByRole("navigation")).toBeNull();
  });
});
