import { useEffect } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useScopeHistoryStore } from "@multica/core/navigation";
import type { Workspace } from "@multica/core/types";
import { WebScopeShell, useWebSidebarNavigation } from "./scope-shell";

const state = vi.hoisted(() => ({
  owner: "server:viewer" as string | null,
  navigation: { pathname: "/second/projects/p1", searchParams: new URLSearchParams("tab=overview"), hash: "#description", push: vi.fn() },
  list: { ready: true, workspaces: [{ slug: "first" }, { slug: "second" }] as Workspace[], unavailable: false, isFetching: false, refetch: vi.fn() },
  resume: vi.fn(),
}));
vi.mock("./navigation-owner", () => ({ useWebNavigationOwner: () => state.owner }));
vi.mock("./navigation", () => ({ useWebScopeResume: () => state.resume }));
vi.mock("@multica/core/workspace", () => ({ useWorkspaceList: () => state.list }));
vi.mock("@multica/views/navigation", async (original) => ({ ...await original<typeof import("@multica/views/navigation")>(), useNavigation: () => state.navigation }));

function location(path: string) {
  const url = new URL(path, "https://test.invalid");
  state.navigation.pathname = url.pathname;
  state.navigation.searchParams = url.searchParams;
  state.navigation.hash = url.hash;
}

// Shared sidebar tests own its links and controls. This probe owns only the
// platform's selected workspace and restoration callbacks.
function SidebarProbe() {
  const { navigationWorkspace, onWorkspaceSelect, onAccountResume, workspaceListUnavailable, onRetryWorkspaces } = useWebSidebarNavigation();
  return <>
    <output data-testid="workspace">{navigationWorkspace?.slug ?? "none"}</output>
    <button onClick={() => onWorkspaceSelect(navigationWorkspace?.slug)}>Workspaces</button>
    <button onClick={() => onWorkspaceSelect("first")}>Select first</button>
    <button onClick={onAccountResume}>Return to previous collaboration</button>
    {workspaceListUnavailable && <button onClick={onRetryWorkspaces}>Refresh</button>}
  </>;
}
const page = () => <WebScopeShell><SidebarProbe /><main>Content</main></WebScopeShell>;

beforeEach(() => {
  useScopeHistoryStore.getState().setOwner(null);
  state.owner = "server:viewer";
  state.list = { ready: true, workspaces: [{ slug: "first" }, { slug: "second" }] as Workspace[], unavailable: false, isFetching: false, refetch: vi.fn() };
  location("/second/projects/p1?tab=overview#description");
  state.resume.mockReset();
  state.navigation.push.mockReset();
});

describe("WebScopeShell", () => {
  it("resumes the full workspace location and leaves the selected workspace alone", () => {
    const workspace = "/second/projects/p1?tab=overview#description";
    const account = "/account?view=participating&issue=00000000-0000-4000-8000-000000000001#memory";
    const view = render(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).not.toHaveBeenCalled();
    location(account);
    view.rerender(page());
    expect(screen.getByTestId("workspace")).toHaveTextContent("second");
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenLastCalledWith(workspace);
    location(workspace);
    view.rerender(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenCalledOnce();
  });

  it("uses the active route before a remembered workspace and opens a different selection explicitly", () => {
    useScopeHistoryStore.getState().setOwner(state.owner);
    useScopeHistoryStore.getState().record(state.owner!, "/first/projects/old");
    render(page());
    expect(screen.getByTestId("workspace")).toHaveTextContent("second");
    fireEvent.click(screen.getByRole("button", { name: "Select first" }));
    expect(state.navigation.push).toHaveBeenCalledWith("/first/issues");
    expect(state.resume).not.toHaveBeenCalled();
  });

  it("keeps account content available with no workspaces and can resume the creation flow", () => {
    location("/account");
    state.list.workspaces = [];
    render(page());
    expect(screen.getByTestId("workspace")).toHaveTextContent("none");
    expect(screen.getByText("Content")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenCalledWith("/workspaces/new");
  });

  it("falls back from a removed space using the authoritative directory", () => {
    const view = render(page());
    location("/account");
    state.list.workspaces = [{ slug: "first" }] as Workspace[];
    view.rerender(page());
    expect(screen.getByTestId("workspace")).toHaveTextContent("first");
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenCalledWith("/first/issues");
  });

  it("does not fabricate a preview or forget history while the directory is loading", () => {
    const view = render(page());
    location("/account");
    state.list.ready = false;
    state.list.workspaces = [];
    view.rerender(page());
    expect(screen.getByTestId("workspace")).toHaveTextContent("none");
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    expect(state.resume).toHaveBeenCalledWith("/second/projects/p1?tab=overview#description");
    expect(state.list.refetch).not.toHaveBeenCalled();
  });

  it("fetches unknown membership on demand and retains account content after failure", async () => {
    location("/account");
    state.list.ready = false;
    state.list.unavailable = true;
    state.list.workspaces = [];
    state.list.refetch.mockResolvedValue({ data: undefined });
    render(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    await waitFor(() => expect(state.list.refetch).toHaveBeenCalledOnce());
    expect(state.resume).not.toHaveBeenCalled();
    expect(screen.getByText("Content")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    expect(state.list.refetch).toHaveBeenCalledTimes(2);
    expect(state.navigation.push).not.toHaveBeenCalled();
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

  it("rejects a late fetch after the viewer changes", async () => {
    location("/account");
    state.list.ready = false;
    state.list.workspaces = [];
    let finish!: (value: { data: { slug: string }[] }) => void;
    state.list.refetch.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const view = render(page());
    fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
    state.owner = "another-viewer";
    view.rerender(page());
    await act(async () => finish({ data: [{ slug: "first" }] }));
    expect(state.resume).not.toHaveBeenCalled();
  });

  it("prevents obsolete sidebar callbacks from navigating or retrying for a new viewer", () => {
    let previous!: ReturnType<typeof useWebSidebarNavigation>;
    function Capture() {
      previous = useWebSidebarNavigation();
      return <SidebarProbe />;
    }
    const shell = () => <WebScopeShell><Capture /></WebScopeShell>;
    const view = render(shell());
    const old = previous;
    state.owner = "another-viewer";
    location("/account");
    view.rerender(shell());
    old.onWorkspaceSelect("first");
    old.onRetryWorkspaces();
    expect(state.resume).not.toHaveBeenCalled();
    expect(state.navigation.push).not.toHaveBeenCalled();
    expect(state.list.refetch).not.toHaveBeenCalled();
  });

  // Returning bare children until viewer discovery remounted the page and
  // detached the active editor. Keep one tree even as the shell state changes.
  it("keeps the page mounted while the viewer and scope resolve without the old top switch", () => {
    const mounts = vi.fn();
    const Page = () => {
      useEffect(() => { mounts(); }, []);
      return <main>Content</main>;
    };
    const shell = () => <WebScopeShell><Page /></WebScopeShell>;
    state.owner = null;
    location("/second/issues/00000000-0000-4000-8000-000000000001");
    const view = render(shell());
    const content = screen.getByText("Content");
    state.owner = "server:viewer";
    view.rerender(shell());
    expect(screen.queryByRole("banner")).toBeNull();
    expect(screen.queryByRole("navigation")).toBeNull();
    location("/workspaces/new");
    view.rerender(shell());
    expect(mounts).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Content")).toBe(content);
  });
});

it("resumes the remembered full account location without disturbing current account input", () => {
  const account = "/account?view=participating&issue=00000000-0000-4000-8000-000000000001#memory";
  location(account);
  const view = render(page());
  fireEvent.click(screen.getByRole("button", { name: "Return to previous collaboration" }));
  expect(state.resume).not.toHaveBeenCalled();
  location("/second/issues"); view.rerender(page());
  fireEvent.click(screen.getByRole("button", { name: "Return to previous collaboration" }));
  expect(state.resume).toHaveBeenCalledExactlyOnceWith(account);
  expect(state.navigation.push).not.toHaveBeenCalled();
  state.owner = "server:another-viewer"; view.rerender(page());
  fireEvent.click(screen.getByRole("button", { name: "Return to previous collaboration" }));
  expect(state.resume).toHaveBeenLastCalledWith("/account?view=current");
});

it("cancels a pending workspace lookup immediately when account resume is requested", async () => {
  location("/account?view=participating&issue=00000000-0000-4000-8000-000000000001#memory");
  state.list.ready = false;
  state.list.workspaces = [];
  let finish!: (value: { data: Workspace[] }) => void;
  state.list.refetch.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  render(page());
  fireEvent.click(screen.getByRole("button", { name: "Workspaces" }));
  expect(state.list.refetch).toHaveBeenCalledOnce();
  fireEvent.click(screen.getByRole("button", { name: "Return to previous collaboration" }));
  await act(async () => { finish({ data: [{ slug: "first" } as Workspace] }); });
  expect(state.resume).not.toHaveBeenCalled();
  expect(state.navigation.push).not.toHaveBeenCalled();
});
