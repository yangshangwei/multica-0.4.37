import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import en from "@multica/views/locales/en/auth.json";
import LoginPage from "./page";

const navigation = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn(), nativeAssign: vi.fn(), nativeReplace: vi.fn(), params: new URLSearchParams() }));
vi.mock("next/navigation", () => ({ useRouter: () => navigation, useSearchParams: () => navigation.params }));
vi.mock("@multica/views/auth", () => ({
  LoginPage: ({ onSuccess, skipWorkspaceBootstrap }: { onSuccess: () => void; skipWorkspaceBootstrap?: boolean }) =>
    <button data-skip-workspaces={String(skipWorkspaceBootstrap)} onClick={onSuccess}>Complete login</button>,
  validateCliCallback: () => false,
}));
vi.mock("@/features/auth/auth-cookie", () => ({ setLoggedInCookie: vi.fn() }));

const originalLocation = window.location;
beforeEach(() => {
  vi.clearAllMocks();
  Object.defineProperty(window, "location", { configurable: true, value: { ...originalLocation, assign: navigation.nativeAssign, replace: navigation.nativeReplace } });
});
afterEach(() => Object.defineProperty(window, "location", { configurable: true, value: originalLocation }));

it.each(["/admin", "/admin?status=queued", "/admin/tasks?source=issue#selected"])("wires %s login to the no-workspace path and preserves its next destination", async (next) => {
  navigation.params = new URLSearchParams({ next });
  const api = new ApiClient("");
  setApiInstance(api);
  registerAuthStore(createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } }));
  const list = vi.spyOn(api, "listWorkspaces");
  render(<I18nProvider locale="en" resources={{ en: { auth: en } }}><QueryClientProvider client={new QueryClient()}><LoginPage /></QueryClientProvider></I18nProvider>);
  const button = screen.getByRole("button", { name: "Complete login" });
  expect(button).toHaveAttribute("data-skip-workspaces", "true");
  fireEvent.click(button);
  await waitFor(() => expect(navigation.nativeAssign).toHaveBeenCalledWith(next));
  expect(list).not.toHaveBeenCalled();
  expect(navigation.push).not.toHaveBeenCalled();
});

it("uses document replacement for an authenticated arrival to avoid replaying a cached admin fragment", async () => {
  const next = "/admin?status=queued#scope";
  navigation.params = new URLSearchParams({ next });
  const api = new ApiClient("");
  setApiInstance(api);
  const store = createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(store);
  store.setState({ status: "authenticated", isLoading: false, user: { id: "admin", name: "Admin", email: "", avatar_url: null, onboarded_at: null, onboarding_questionnaire: {}, starter_content_state: null, language: null, profile_description: "", timezone: null, created_at: "", updated_at: "" } });
  render(<I18nProvider locale="en" resources={{ en: { auth: en } }}><QueryClientProvider client={new QueryClient()}><LoginPage /></QueryClientProvider></I18nProvider>);
  await waitFor(() => expect(navigation.nativeReplace).toHaveBeenCalledWith(next));
  expect(navigation.replace).not.toHaveBeenCalled();
});
