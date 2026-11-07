// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "../api";
import { defaultStorage } from "../platform/storage";
import { protectIterationRead } from "./access";
import { useIterationCommand, type IterationCommand } from "./command";
const identity = vi.hoisted(() => ({ actor: "actor", session: "session-a" }));
vi.mock("../auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: { user: { id: string } }) => unknown) =>
      select({ user: { id: identity.actor } }),
    { getState: () => ({ user: { id: identity.actor } }) },
  ),
}));
vi.mock("../api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api")>()),
  api: {
    getBaseUrl: () => "server",
    getSessionScope: () => identity.session,
    createIteration: vi.fn(),
    getIterationOperation: vi.fn(),
    updateIteration: vi.fn(),
  },
}));
const command: IterationCommand = {
  kind: "create",
  body: {
    request_id: "10000000-0000-4000-8000-000000000001",
    name: "Name",
    description: null,
    coordinator_user_id: null,
    start_date: "2026-10-01",
    end_date: "2026-10-14",
    confirmed_timezone: "UTC",
  },
};
const receipt = {
  workspace_id: "w",
  request_id: command.body.request_id,
  operation_id: "o",
  operation: "create",
  replayed: false,
  iteration_ids: ["i"],
  result: {
    snapshot_id: null,
    deleted: false,
    settings_revision: 1,
    issue_count: 0,
  },
  committed_at: "2026-10-01T00:00:00Z",
};
function wrapper({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={new QueryClient()}>
      {children}
    </QueryClientProvider>
  );
}
beforeEach(() => {
  vi.resetAllMocks();
  identity.actor = "actor";
  identity.session = "session-a";
  for (const key of defaultStorage.keys?.() ?? [])
    defaultStorage.removeItem(key);
});
describe("iteration command recovery", () => {
  it.each([undefined, { code: "operation_not_found" }])(
    "recovers the exact payload after a missing operation lookup: %j",
    async (body) => {
      vi.mocked(api.createIteration).mockRejectedValueOnce(
        new TypeError("Network lost"),
      );
      const first = renderHook(() => useIterationCommand("w", "create"), {
        wrapper,
      });
      await act(async () => {
        await expect(
          first.result.current.mutateAsync({ command }),
        ).rejects.toThrow();
      });
      expect(first.result.current.pending).toEqual(command);
      first.unmount();
      const second = renderHook(() => useIterationCommand("w", "create"), {
        wrapper,
      });
      expect(second.result.current.pending).toEqual(command);
      vi.mocked(api.getIterationOperation).mockRejectedValueOnce(
        new ApiError("Missing", 404, "Not Found", body),
      );
      vi.mocked(api.createIteration).mockResolvedValueOnce(receipt);
      await act(async () => {
        await second.result.current.mutateAsync({
          command: { ...command, body: { ...command.body, name: "Changed" } },
          recover: true,
        });
      });
      expect(api.createIteration).toHaveBeenLastCalledWith("w", command.body);
      expect(second.result.current.pending).toBeNull();
    },
  );
  it("releases confirmed conflicts so preserved input can be revised", async () => {
    vi.mocked(api.createIteration).mockRejectedValue(
      new ApiError("Conflict", 409, "Conflict"),
    );
    const hook = renderHook(() => useIterationCommand("w", "create"), {
      wrapper,
    });
    await act(async () => {
      await expect(
        hook.result.current.mutateAsync({ command }),
      ).rejects.toThrow();
    });
    expect(hook.result.current.pending).toBeNull();
    expect(defaultStorage.keys?.()).toEqual([]);
  });
  it("isolates pending requests by workspace", async () => {
    vi.mocked(api.createIteration).mockRejectedValue(new TypeError("Lost"));
    const hook = renderHook(() => useIterationCommand("w", "create"), {
      wrapper,
    });
    await act(async () => {
      await expect(
        hook.result.current.mutateAsync({ command }),
      ).rejects.toThrow();
    });
    const other = renderHook(() => useIterationCommand("other", "create"), {
      wrapper,
    });
    expect(other.result.current.pending).toBeNull();
  });
  it("erases pending commands and protected cache on qualified 404 lookup without retrying POST", async () => {
    const client = new QueryClient();
    const hook = renderHook(() => useIterationCommand("w", "create"), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    });
    vi.mocked(api.createIteration).mockRejectedValueOnce(new TypeError("Lost"));
    await act(async () => {
      await expect(
        hook.result.current.mutateAsync({ command }),
      ).rejects.toThrow();
    });
    client.setQueryData(["iterations", "w", "detail", "i"], {
      title: "Protected",
    });
    vi.mocked(api.getIterationOperation).mockRejectedValueOnce(
      new ApiError("Denied", 404, "Not Found", {
        code: "workspace_access_denied",
      }),
    );
    await act(async () => {
      await expect(
        hook.result.current.mutateAsync({ command, recover: true }),
      ).rejects.toMatchObject({ status: 404 });
    });
    expect(api.createIteration).toHaveBeenCalledTimes(1);
    expect(hook.result.current.pending).toBeNull();
    expect(defaultStorage.keys?.()).toEqual([]);
    expect(
      client.getQueryData(["iterations", "w", "detail", "i"]),
    ).toBeUndefined();
  });

  it("treats a documented missing iteration write as rejected without revoking workspace access", async () => {
    const client = new QueryClient();
    client.setQueryData(["iterations", "w", "detail", "other"], {
      name: "Allowed",
    });
    const hook = renderHook(() => useIterationCommand("w", "edit"), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    });
    vi.mocked(api.updateIteration).mockRejectedValueOnce(
      new ApiError("Deleted", 404, "Not Found", {
        code: "iteration_not_found",
      }),
    );
    const edit: IterationCommand = {
      kind: "edit",
      id: "i",
      body: {
        request_id: command.body.request_id,
        expected_revision: 1,
        fields: { name: "Kept input" },
        reason: "Correction",
      },
    };
    await act(async () => {
      await expect(
        hook.result.current.mutateAsync({ command: edit }),
      ).rejects.toMatchObject({ status: 404 });
    });
    expect(hook.result.current.pending).toBeNull();
    expect(defaultStorage.keys?.()).toEqual([]);
    expect(client.getQueryData(["iterations", "w", "detail", "other"])).toEqual(
      { name: "Allowed" },
    );
  });

  it("clears an already-pending hook immediately when a separate read proves qualified404 revocation", async () => {
    const client = new QueryClient();
    const hook = renderHook(() => useIterationCommand("w", "create"), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    });
    vi.mocked(api.createIteration).mockRejectedValueOnce(new TypeError("Lost"));
    await act(async () => {
      await expect(
        hook.result.current.mutateAsync({ command }),
      ).rejects.toThrow();
    });
    expect(hook.result.current.pending).not.toBeNull();
    await act(async () => {
      await expect(
        protectIterationRead(client, "w", async () => {
          throw new ApiError("Denied", 404, "Not Found", {
            code: "workspace_access_denied",
          });
        }),
      ).rejects.toMatchObject({ status: 404 });
    });
    expect(hook.result.current.pending).toBeNull();
    expect(defaultStorage.keys?.()).toEqual([]);
  });
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
}

describe("iteration recovery identity fences", () => {
  it.each(["actor", "session"] as const)(
    "does not resend an old command after the %s changes during lookup",
    async (field) => {
      const lookup = deferred<typeof receipt>();
      vi.mocked(api.getIterationOperation).mockReturnValueOnce(lookup.promise);
      vi.mocked(api.createIteration).mockResolvedValueOnce(receipt);
      const hook = renderHook(() => useIterationCommand("w", "create"), { wrapper });
      const outcome = hook.result.current
        .mutateAsync({ command, recover: true })
        .catch((error: unknown) => error);
      await waitFor(() => expect(api.getIterationOperation).toHaveBeenCalledOnce());
      identity[field] = "replacement";
      await act(async () => {
        lookup.reject(new ApiError("Missing", 404, "Not Found", { code: "operation_not_found" }));
        await outcome;
      });
      expect(api.createIteration).not.toHaveBeenCalled();
      expect(await outcome).toBeInstanceOf(Error);
    },
  );

  it("does not resend or revoke fresh access after revocation and recovery during lookup", async () => {
    const client = new QueryClient();
    const lookup = deferred<typeof receipt>();
    vi.mocked(api.getIterationOperation).mockReturnValueOnce(lookup.promise);
    vi.mocked(api.createIteration).mockResolvedValueOnce(receipt);
    const hook = renderHook(() => useIterationCommand("w", "create"), {
      wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
    });
    const outcome = hook.result.current.mutateAsync({ command, recover: true }).catch((error: unknown) => error);
    await waitFor(() => expect(api.getIterationOperation).toHaveBeenCalledOnce());
    await act(async () => {
      await expect(protectIterationRead(client, "w", async () => {
        throw new ApiError("Revoked", 404, "Not Found", { code: "workspace_access_denied" });
      })).rejects.toThrow();
      await protectIterationRead(client, "w", async () => "regranted", true);
      client.setQueryData(["iterations", "w", "detail", "new"], { title: "Fresh access" });
      lookup.reject(new ApiError("Missing", 404, "Not Found", { code: "operation_not_found" }));
      await outcome;
    });
    expect(api.createIteration).not.toHaveBeenCalled();
    expect(client.getQueryData(["iterations", "w", "detail", "new"])).toEqual({ title: "Fresh access" });
    await expect(protectIterationRead(client, "w", async () => "allowed")).resolves.toBe("allowed");
  });

  it.each(["success", "forbidden"] as const)(
    "ignores late %s from a replaced session without clearing its cache or pending request",
    async (response) => {
      const client = new QueryClient();
      const lookup = deferred<typeof receipt>();
      vi.mocked(api.getIterationOperation).mockReturnValueOnce(lookup.promise);
      const hook = renderHook(() => useIterationCommand("w", "create"), {
        wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
      });
      const outcome = hook.result.current.mutateAsync({ command, recover: true }).catch((error: unknown) => error);
      await waitFor(() => expect(api.getIterationOperation).toHaveBeenCalledOnce());
      identity.actor = "next-actor";
      identity.session = "next-session";
      const newKey = `multica_iteration_command:server:next-actor:w:create:${command.body.request_id}`;
      const newCommand = JSON.stringify({ ...command, body: { ...command.body, name: "New actor draft" } });
      defaultStorage.setItem(newKey, newCommand);
      client.setQueryData(["iterations", "w", "detail", "new"], { title: "New actor data" });
      await act(async () => {
        if (response === "success") lookup.resolve(receipt);
        else lookup.reject(new ApiError("Denied", 403, "Forbidden"));
        await outcome;
      });
      expect(await outcome).toBeInstanceOf(Error);
      expect(client.getQueryData(["iterations", "w", "detail", "new"])).toEqual({ title: "New actor data" });
      expect(defaultStorage.getItem(newKey)).toBe(newCommand);
      expect(api.createIteration).not.toHaveBeenCalled();
    },
  );
});

it("refreshes settings dependencies after a confirmed local command", async () => {
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, "invalidateQueries");
  vi.mocked(api.createIteration).mockResolvedValue(receipt);
  const hook = renderHook(() => useIterationCommand("w", "create"), {
    wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
  });
  await act(async () => { await hook.result.current.mutateAsync({ command }); });
  expect(invalidate).toHaveBeenCalledWith({ queryKey: ["triage", "w", "settings"] });
  expect(invalidate).toHaveBeenCalledWith({ queryKey: ["projects", "w", "planning-timezone"] });
  client.clear();
});

it("refreshes the shared timezone after a rejected command before a new intent", async () => {
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, "invalidateQueries");
  vi.mocked(api.createIteration).mockRejectedValue(new ApiError("Timezone changed", 409, "Conflict"));
  const hook = renderHook(() => useIterationCommand("w", "create"), {
    wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
  });
  await act(async () => { await expect(hook.result.current.mutateAsync({ command })).rejects.toThrow(); });
  expect(invalidate).toHaveBeenCalledWith({ queryKey: ["projects", "w", "planning-timezone"] });
  client.clear();
});
