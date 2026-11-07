// @vitest-environment jsdom

import type { ReactNode } from "react";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { onlineManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { ApiError } from "../api/client";
import { useCreatePersonalAccessToken, useRevokePersonalAccessToken } from "./token-queries";

vi.mock("../api", () => ({
  api: { createPersonalAccessToken: vi.fn(), revokePersonalAccessToken: vi.fn() },
}));

const clients: QueryClient[] = [];
const request = { name: "Test CLI", expires_in_days: 90 };
const credential = "mca_hook_test_only_credential";

function setup<T>(hook: () => T) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: 2, retryDelay: 0, gcTime: Infinity } },
  });
  clients.push(client);
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return renderHook(hook, { wrapper });
}

beforeEach(() => {
  vi.resetAllMocks();
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
  onlineManager.setOnline(true);
});

describe("personal access token mutations", () => {
  // Full QueryCache/MutationCache observation with the real view is owned by tokens-tab.test.tsx.
  it("delivers the one-time token to the caller while resolving without mutation data", async () => {
    const onCreated = vi.fn();
    vi.mocked(api.createPersonalAccessToken).mockResolvedValue({
      id: "token-1", name: request.name, token_prefix: "mca_test", token: credential,
      created_at: "2026-10-01T00:00:00Z", expires_at: null, last_used_at: null,
    });
    const { result } = setup(() => useCreatePersonalAccessToken(onCreated));

    await act(async () => {
      await expect(result.current.mutateAsync(request)).resolves.toBeUndefined();
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.createPersonalAccessToken).toHaveBeenCalledExactlyOnceWith(request);
    expect(onCreated).toHaveBeenCalledExactlyOnceWith(credential);
    expect(result.current.data).toBeUndefined();
    expect(result.current.variables).toEqual(request);
  });

  it("does not retry an unusable create response even when global mutation retries are enabled", async () => {
    const onCreated = vi.fn();
    const error = new ApiError("Invalid API token creation response", 502, "Bad Gateway", {
      code: "invalid_personal_access_token",
    });
    vi.mocked(api.createPersonalAccessToken).mockRejectedValue(error);
    const { result } = setup(() => useCreatePersonalAccessToken(onCreated));

    await act(async () => {
      await expect(result.current.mutateAsync(request)).rejects.toBe(error);
    });

    expect(api.createPersonalAccessToken).toHaveBeenCalledExactlyOnceWith(request);
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("returns a failed revocation to the confirmation without automatically resubmitting", async () => {
    const error = new Error("Revocation unavailable");
    vi.mocked(api.revokePersonalAccessToken).mockRejectedValue(error);
    const { result } = setup(useRevokePersonalAccessToken);

    await act(async () => {
      await expect(result.current.mutateAsync("token-1")).rejects.toBe(error);
    });

    expect(api.revokePersonalAccessToken).toHaveBeenCalledExactlyOnceWith("token-1");
  });

  it.each(["create", "revoke"] as const)("does not queue %s until a later offline recovery", async (action) => {
    onlineManager.setOnline(false);
    vi.mocked(api.createPersonalAccessToken).mockRejectedValue(new Error("Network unavailable"));
    vi.mocked(api.revokePersonalAccessToken).mockRejectedValue(new Error("Network unavailable"));
    const { result } = setup(() => ({
      create: useCreatePersonalAccessToken(vi.fn()),
      revoke: useRevokePersonalAccessToken(),
    }));

    act(() => {
      if (action === "create") result.current.create.mutate(request);
      else result.current.revoke.mutate("token-1");
    });

    await waitFor(() => expect(result.current[action].isError).toBe(true));
    expect(result.current[action].isPaused).toBe(false);
    expect(action === "create" ? api.createPersonalAccessToken : api.revokePersonalAccessToken).toHaveBeenCalledOnce();
  });
});
