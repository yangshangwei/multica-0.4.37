import { beforeEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { ApiClient } from "@multica/core/api";
import { getCurrentSlug, getCurrentWsId, setCurrentWorkspace } from "@multica/core/platform";
import type { ReactNode } from "react";

const navigation = vi.hoisted(() => ({ replace: vi.fn(), pathname: "/admin", searchParams: new URLSearchParams(), hash: "" }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: navigation.replace }) }));
vi.mock("@multica/views/navigation", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/views/navigation")>();
  return { ...actual, useNavigation: () => navigation };
});
vi.mock("@multica/views/admin", () => ({ AdminShell: ({ children }: { children: ReactNode }) => <div>{children}</div> }));
import { AdminRoute } from "./admin-route";

beforeEach(() => { navigation.replace.mockClear(); navigation.pathname = "/admin"; navigation.searchParams = new URLSearchParams(); navigation.hash = ""; });

describe("web administration route", () => {
  it("clears the workspace mirror and preserves the admin login destination", () => {
    const store = createAuthStore({ api: new ApiClient(""), storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
    registerAuthStore(store);
    store.setState({ status: "unauthenticated", isLoading: false });
    setCurrentWorkspace("acme", "workspace-id");
    render(<AdminRoute><p>Admin content</p></AdminRoute>);
    expect(getCurrentSlug()).toBeNull();
    expect(getCurrentWsId()).toBeNull();
    expect(navigation.replace).toHaveBeenCalledWith("/login?next=%2Fadmin");
  });

  it("preserves the protected management path, filters and fragment through login", () => {
    const store = createAuthStore({ api: new ApiClient(""), storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
    registerAuthStore(store);
    store.setState({ status: "unauthenticated", isLoading: false });
    navigation.pathname = "/admin/tasks";
    navigation.searchParams = new URLSearchParams({ status: "queued", source: "issue" });
    navigation.hash = "#selected";
    render(<AdminRoute>{null}</AdminRoute>);
    const redirect = new URL(navigation.replace.mock.calls[0]?.[0], "https://multica.test");
    expect(redirect.pathname).toBe("/login");
    expect(redirect.searchParams.get("next")).toBe("/admin/tasks?status=queued&source=issue#selected");
  });

  it("waits for the existing password completion flow without redirecting to a workspace", () => {
    const store = createAuthStore({ api: new ApiClient(""), storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
    registerAuthStore(store);
    store.setState({ status: "password_change_required", isLoading: false });
    render(<AdminRoute>{null}</AdminRoute>);
    expect(navigation.replace).not.toHaveBeenCalled();
  });
});
