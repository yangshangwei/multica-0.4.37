import type { ReactNode } from "react";
import { act, cleanup, render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { EMPTY_USER } from "@multica/core/api/schemas";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import type { Workspace } from "@multica/core/types";
import { useTabStore } from "./stores/tab-store";
import { useWindowOverlayStore } from "./stores/window-overlay-store";

// Keep the real auth, Query, tab and overlay stores, and AppContent's effects.
// Shell rendering and native services are outside this routing regression.
vi.mock("@multica/views/admin", () => ({ AdminSessionBoundary: () => null }));
vi.mock("./components/desktop-layout", () => ({ DesktopShell: () => null }));
vi.mock("./components/window-overlay", () => ({ WindowOverlay: () => null }));
vi.mock("./components/issue-window", () => ({ IssueWindow: () => null }));
vi.mock("./components/update-notification", () => ({}));
vi.mock("./components/close-behavior-prompt", () => ({}));
vi.mock("./pages/login", () => ({ DesktopLoginPage: () => null }));
vi.mock("./pages/endpoint-setup", () => ({}));
vi.mock("./components/connection-recovery-boundary", () => ({
  DesktopConnectionRecoveryBoundary: ({ children }: { children: ReactNode }) => children,
}));

import { AppContent } from "./App";

let client: ApiClient;
let auth: ReturnType<typeof createAuthStore>;
let queryClient: QueryClient;
const newUser = { ...EMPTY_USER, id: "new-user", name: "New user" };

function mountApp() {
  return render(
    <QueryClientProvider client={queryClient}>
      <AppContent onServerEditorOpenChange={vi.fn()} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  useTabStore.getState().reset();
  useWindowOverlayStore.getState().close();
  client = new ApiClient("https://registration.example.invalid");
  setApiInstance(client);
  vi.spyOn(client, "registerPassword").mockResolvedValue({ token: "test-session", user: newUser });
  vi.spyOn(client, "listWorkspaces").mockResolvedValue([]);
  vi.spyOn(client, "listMyInvitations").mockResolvedValue([]);
  auth = createAuthStore({
    api: client,
    storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() },
  });
  registerAuthStore(auth);
  auth.setState({ isLoading: false, status: "unauthenticated" });
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  Object.defineProperty(window, "desktopAPI", {
    configurable: true,
    value: {
      runtimeConfig: { ok: true, config: { apiUrl: client.getBaseUrl() } },
      onRuntimeConfigReset: () => () => {},
      onInviteOpen: () => () => {},
      onAuthToken: () => () => {},
    },
  });
  Object.defineProperty(window, "daemonAPI", {
    configurable: true,
    value: { setTargetApiUrl: vi.fn() },
  });
});

afterEach(() => {
  cleanup();
  queryClient.clear();
  vi.restoreAllMocks();
});

describe("desktop first-run routing", () => {
  // The input matrix belongs in platform/account-bootstrap.test.ts. These
  // tests exercise the registration transition and competing React effects.
  it("opens onboarding after registration when the empty workspace list arrives", async () => {
    let resolveWorkspaces!: (workspaces: Workspace[]) => void;
    vi.mocked(client.listWorkspaces).mockImplementation(() => new Promise((resolve) => {
      resolveWorkspaces = resolve;
    }));
    mountApp();

    await act(async () => {
      await auth.getState().registerPassword("new_user", "test-password", "New user");
    });
    expect(useTabStore.getState().activeScope).toBe("none");
    expect(useWindowOverlayStore.getState().overlay).toBeNull();

    await act(async () => { resolveWorkspaces([]); });

    await waitFor(() => expect(useWindowOverlayStore.getState().overlay).toEqual({ type: "onboarding" }));
    expect(auth.getState().user?.onboarded_at).toBeNull();
    expect(useTabStore.getState().activeScope).toBe("none");
    expect(useTabStore.getState().accountGroup).toBeNull();
  });

  it("keeps the collaboration fallback for an onboarded account without workspaces", async () => {
    auth.setState({ user: { ...newUser, onboarded_at: "2026-11-01T00:00:00Z" }, status: "authenticated" });
    mountApp();

    await waitFor(() => expect(useTabStore.getState().activeScope).toBe("account"));
    expect(client.listMyInvitations).not.toHaveBeenCalled();
    expect(useWindowOverlayStore.getState().overlay?.type).not.toBe("onboarding");
  });

  it("preserves an explicit account destination for a user who has not onboarded", async () => {
    auth.setState({ user: newUser, status: "authenticated" });
    useTabStore.getState().setSessionOwner(JSON.stringify([client.getBaseUrl(), newUser.id]));
    useTabStore.getState().switchAccount("/account?view=participating");
    mountApp();

    await waitFor(() => expect(client.listWorkspaces).toHaveBeenCalled());
    await act(async () => {});
    expect(useTabStore.getState().activeScope).toBe("account");
    expect(useWindowOverlayStore.getState().overlay).toBeNull();
    expect(client.listMyInvitations).not.toHaveBeenCalled();
  });
});
