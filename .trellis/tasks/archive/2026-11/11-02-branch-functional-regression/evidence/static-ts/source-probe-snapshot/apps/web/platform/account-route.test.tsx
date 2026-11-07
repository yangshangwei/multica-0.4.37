import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { ApiClient } from "@multica/core/api";
import { EMPTY_USER } from "@multica/core/api/schemas";
import { getCurrentSlug, getCurrentWsId, setCurrentWorkspace } from "@multica/core/platform";
import { AccountRoute } from "./account-route";

const navigation = vi.hoisted(() => ({ replace: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "" }));
vi.mock("@multica/views/navigation", async (original) => ({ ...await original<typeof import("@multica/views/navigation")>(), useNavigation: () => navigation }));
vi.mock("@multica/core/workspace", () => ({ useWorkspaceList: () => { throw new Error("Account routes cannot depend on workspace discovery"); } }));

beforeEach(() => { navigation.replace.mockClear(); navigation.pathname = "/account"; navigation.searchParams = new URLSearchParams(); navigation.hash = ""; });
function store() {
  const auth = createAuthStore({ api: new ApiClient(""), storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(auth);
  return auth;
}

it("clears the workspace mirror before mounting authenticated account content", () => {
  store().setState({ status: "authenticated", isLoading: false, user: { ...EMPTY_USER, id: "account-user" } });
  setCurrentWorkspace("old-space", "old-id");
  function PrivateContent() {
    expect(getCurrentSlug()).toBeNull();
    expect(getCurrentWsId()).toBeNull();
    return <p>Account content</p>;
  }
  render(<AccountRoute><PrivateContent /></AccountRoute>);
  expect(screen.getByText("Account content")).toBeTruthy();
  expect(navigation.replace).not.toHaveBeenCalled();
});

it("preserves account selection and comment anchor through login without mounting content", () => {
  store().setState({ status: "unauthenticated", isLoading: false });
  const issue = "00000000-0000-4000-8000-000000000001";
  navigation.searchParams = new URLSearchParams({ view: "participating", issue });
  navigation.hash = "#comment-00000000-0000-4000-8000-000000000002";
  render(<AccountRoute><p>Private content</p></AccountRoute>);
  expect(screen.queryByText("Private content")).toBeNull();
  expect(navigation.replace).toHaveBeenCalledWith(`/login?next=${encodeURIComponent(`/account?view=participating&issue=${issue}${navigation.hash}`)}`);
});

it.each(["authenticating", "recovering", "password_change_required", "account_setup_required"] as const)("keeps %s accounts out of the workspace-independent private tree", (status) => {
  store().setState({ status, isLoading: status === "authenticating", user: { ...EMPTY_USER, id: "account-user" } });
  render(<AccountRoute><p>Private content</p></AccountRoute>);
  expect(screen.queryByText("Private content")).toBeNull();
  expect(navigation.replace).not.toHaveBeenCalled();
});
