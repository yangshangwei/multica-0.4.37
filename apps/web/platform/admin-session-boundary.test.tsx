import { act, render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { adminApiScope, adminKeys } from "@multica/core/admin";
import type { User } from "@multica/core/types";
import { AdminSessionBoundary } from "./admin-session-boundary";

const user: User = { id: "admin", name: "Admin", email: "", avatar_url: null, onboarded_at: null, onboarding_questionnaire: {}, starter_content_state: null, language: null, profile_description: "", timezone: null, created_at: "", updated_at: "" };

describe("admin cache boundary outside administration routes", () => {
  it.each(["account", "logout", "credential", "server"])("erases old data on %s change", (change) => {
    const api = new ApiClient("https://one.example");
    setApiInstance(api);
    const store = createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
    registerAuthStore(store);
    store.setState({ user, status: "authenticated", isLoading: false });
    const qc = new QueryClient();
    const key = adminKeys.resource({ apiScope: adminApiScope(), userId: user.id, organizationId: "org" }, "tasks");
    qc.setQueryData(key, ["private"]);
    qc.setQueryData(["workspace"], ["workspace"]);
    const view = render(<QueryClientProvider client={qc}><AdminSessionBoundary /></QueryClientProvider>);
    act(() => {
      if (change === "account") store.setState({ user: { ...user, id: "another" } });
      if (change === "logout") store.getState().logout();
      if (change === "credential") { api.setToken("replacement"); store.setState({ user }); }
      if (change === "server") { setApiInstance(new ApiClient("https://two.example")); view.rerender(<QueryClientProvider client={qc}><AdminSessionBoundary /></QueryClientProvider>); }
    });
    expect(qc.getQueryData(key)).toBeUndefined();
    expect(qc.getQueryData(["workspace"])).toEqual(["workspace"]);
    view.unmount();
    qc.clear();
  });
});
