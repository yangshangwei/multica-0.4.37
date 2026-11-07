// @vitest-environment jsdom

import { act, cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useAuthStore, type AuthStatus } from "../auth";
import type { StorageAdapter } from "../types/storage";
import { ClientUsageReporter } from "./reporter";

const upsertClientUsage = vi.hoisted(() => vi.fn());
vi.mock("../api", () => ({ getApi: () => ({ upsertClientUsage }) }));
vi.mock("../auth", async () => {
  const { create } = await import("zustand");
  return {
    useAuthStore: create<{ user: { id: string }; status: AuthStatus }>(() => ({
      user: { id: "same-user" },
      status: "account_setup_required",
    })),
  };
});

let storage: StorageAdapter;
beforeEach(() => {
  upsertClientUsage.mockReset().mockResolvedValue(undefined);
  const values = new Map<string, string>();
  storage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => { values.set(key, value); },
    removeItem: (key) => { values.delete(key); },
  };
});
afterEach(cleanup);

it.each(["account_setup_required", "password_change_required"] as const)(
  "defers reporting during %s and reports when the same user becomes authenticated",
  async (status) => {
    useAuthStore.setState({ status });
    render(<ClientUsageReporter storage={storage} identity={{ platform: "web" }} />);
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(upsertClientUsage).not.toHaveBeenCalled();

    act(() => useAuthStore.setState({ status: "authenticated" }));
    await waitFor(() => expect(upsertClientUsage).toHaveBeenCalledExactlyOnceWith({
      install_id: expect.any(String),
    }));
    await act(async () => { window.dispatchEvent(new Event("focus")); });
    expect(upsertClientUsage).toHaveBeenCalledOnce();
  },
);
