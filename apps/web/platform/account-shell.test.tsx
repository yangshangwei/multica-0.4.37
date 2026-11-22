import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { ApiClient } from "@multica/core/api";
import { EMPTY_USER } from "@multica/core/api/schemas";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { getCurrentSlug, getCurrentWsId, setCurrentWorkspace } from "@multica/core/platform";
import { useWorkspaceSlug } from "@multica/core/paths";
import type { Workspace } from "@multica/core/types";
import AccountLayout from "../app/account/layout";

const state = vi.hoisted(() => ({
  sidebar: {
    navigationWorkspace: { id: "preview-workspace", slug: "selected" } as Workspace | null,
    onWorkspaceSelect: vi.fn(),
    workspaceListUnavailable: false, onRetryWorkspaces: vi.fn(),
  },
  navigation: { pathname: "/account", searchParams: new URLSearchParams(), hash: "", replace: vi.fn() },
}));
vi.mock("./scope-shell", () => ({ useWebSidebarNavigation: () => state.sidebar }));
vi.mock("@multica/views/navigation", async (original) => ({ ...await original<typeof import("@multica/views/navigation")>(), useNavigation: () => state.navigation }));
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (selector: (value: unknown) => string) => selector({ toggle_sidebar: "Toggle sidebar" }) }) }));
// This suite owns account layout wiring; shared tests own directory UI behavior.
vi.mock("@multica/views/layout", () => ({
  AppSidebar: (props: typeof state.sidebar) => {
    expect(useWorkspaceSlug()).toBeNull();
    expect(getCurrentSlug()).toBeNull();
    expect(getCurrentWsId()).toBeNull();
    return <aside aria-label="Application navigation">
      <output data-testid="preview">{props.navigationWorkspace?.slug ?? "none"}</output>
      {props.workspaceListUnavailable && <button onClick={props.onRetryWorkspaces}>Retry workspaces</button>}
    </aside>;
  },
}));
vi.mock("@multica/views/modals/registry", () => ({
  FeedbackModalHost: () => <div data-testid="feedback-modal-host" />,
}));

beforeEach(() => {
  vi.clearAllMocks();
  const auth = createAuthStore({ api: new ApiClient(""), storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(auth);
  auth.setState({ status: "authenticated", isLoading: false, user: { ...EMPTY_USER, id: "account-user" } });
  state.sidebar.navigationWorkspace = { id: "preview-workspace", slug: "selected" } as Workspace;
  state.sidebar.workspaceListUnavailable = false;
  Object.defineProperty(window, "matchMedia", { configurable: true, value: vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })) });
  setCurrentWorkspace("outgoing", "outgoing-id");
});

function PrivateContent({ children }: { children: ReactNode }) {
  expect(getCurrentSlug()).toBeNull();
  expect(getCurrentWsId()).toBeNull();
  return <p>{children}</p>;
}

describe("Web account sidebar layout", () => {
  it("mounts shared navigation and its compact trigger after clearing the workspace mirror", () => {
    render(<AccountLayout><PrivateContent>Account content</PrivateContent></AccountLayout>);
    expect(screen.getByRole("complementary", { name: "Application navigation" })).toBeInTheDocument();
    expect(screen.getByTestId("preview")).toHaveTextContent("selected");
    expect(screen.getByRole("button", { name: "Toggle sidebar" })).toBeInTheDocument();
    expect(screen.getByTestId("feedback-modal-host")).toBeInTheDocument();
    expect(screen.getByText("Account content")).toBeInTheDocument();
  });

  it("keeps content and recovery reachable without a selected workspace", () => {
    state.sidebar.navigationWorkspace = null;
    state.sidebar.workspaceListUnavailable = true;
    render(<AccountLayout><PrivateContent>Account content</PrivateContent></AccountLayout>);
    expect(screen.getByText("Account content")).toBeInTheDocument();
    expect(screen.getByTestId("preview")).toHaveTextContent("none");
    fireEvent.click(screen.getByRole("button", { name: "Retry workspaces" }));
    expect(state.sidebar.onRetryWorkspaces).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: "Toggle sidebar" })).toBeInTheDocument();
  });
});
