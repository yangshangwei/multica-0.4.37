// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useLogout } from "./use-logout";

// What client state gets erased, and the order it has to happen in, is pinned
// in core's platform/session-cleanup.test.ts — the session-expiry path runs
// the same function. What is left for this hook is the logout-only tail:
// erase, then drop auth, then move the URL.
const calls = vi.hoisted(() => [] as string[]);
const mockClearClientSessionData = vi.hoisted(() => vi.fn());
const mockAuthLogout = vi.hoisted(() => vi.fn());
const mockPush = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());
const queryClient = vi.hoisted(() => ({ id: "query-client" }));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => queryClient,
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector?: (s: unknown) => unknown) => {
      const state = { logout: mockAuthLogout };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ logout: mockAuthLogout }) },
  ),
}));

vi.mock("@multica/core/platform", () => ({
  clearClientSessionData: mockClearClientSessionData,
}));

vi.mock("@multica/core/paths", () => ({
  paths: { login: () => "/login" },
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: mockPush }),
}));

vi.mock("sonner", () => ({ toast: { error: mockToastError } }));
vi.mock("../i18n", () => ({ useT: () => ({ t: () => "Could not complete the request. Please try again later." }) }));

describe("useLogout", () => {
  beforeEach(() => {
    calls.length = 0;
    vi.clearAllMocks();
    mockClearClientSessionData.mockImplementation(() => calls.push("clear"));
    mockAuthLogout.mockImplementation((beforeLogout?: () => void) => { beforeLogout?.(); calls.push("authLogout"); });
    mockPush.mockImplementation(() => calls.push("push"));
  });

  // Erasing after the auth store publishes `unauthenticated` would race the
  // shells' login redirect, and navigating first would leave the caller on a
  // workspace URL that renders null.
  it("erases client state, then drops auth, then navigates to /login", async () => {
    const { result } = renderHook(() => useLogout());
    await act(async () => { await result.current(); });

    expect(calls).toEqual(["clear", "authLogout", "push"]);
    expect(mockClearClientSessionData).toHaveBeenCalledWith(queryClient);
    expect(mockPush).toHaveBeenCalledWith("/login");
  });
  it("does not navigate or erase drafts before the cookie session has been cleared", async () => {
    let finish!: () => void;
    mockAuthLogout.mockImplementation((beforeLogout: () => void) => new Promise<void>(resolve => { finish = () => { beforeLogout(); calls.push("authLogout"); resolve(); }; }));
    const { result } = renderHook(() => useLogout());
    const first = result.current();
    await result.current();
    expect(mockAuthLogout).toHaveBeenCalledOnce();
    expect(mockClearClientSessionData).not.toHaveBeenCalled(); expect(mockPush).not.toHaveBeenCalled();
    await act(async () => { finish(); await first; });
    expect(calls).toEqual(["clear", "authLogout", "push"]);
  });

  it("handles logout failure visibly without navigation or client cleanup", async () => {
    mockAuthLogout.mockRejectedValueOnce(new Error("private request detail"));
    const { result } = renderHook(() => useLogout());
    await act(async () => { await result.current(); });
    expect(mockPush).not.toHaveBeenCalled(); expect(mockClearClientSessionData).not.toHaveBeenCalled();
    expect(mockToastError).toHaveBeenCalledWith("Could not complete the request. Please try again later.");
  });

});
