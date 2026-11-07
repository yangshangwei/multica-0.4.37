// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import type { StorageAdapter } from "../types/storage";
import { createWorkspaceNamePreferencesStore } from "./workspace-name-preferences";

function memoryStorage(initial: string | null = null) {
  let value = initial;
  return {
    getItem: vi.fn(() => value),
    setItem: vi.fn((_key: string, next: string) => { value = next; }),
    removeItem: vi.fn(() => { value = null; }),
  } satisfies StorageAdapter;
}

describe("workspace name preferences", () => {
  it("defers reading persistent state until client hydration", async () => {
    const storage = memoryStorage(JSON.stringify({ state: { seriesByUser: { alice: "space" } }, version: 0 }));
    const store = createWorkspaceNamePreferencesStore(storage);
    expect(storage.getItem).not.toHaveBeenCalled();
    expect(store.getState().seriesByUser).toEqual({});
    await store.persist.rehydrate();
    expect(store.getState().seriesByUser).toEqual({ alice: "space" });
  });

  it("restores each account independently across store instances", async () => {
    const storage = memoryStorage();
    const first = createWorkspaceNamePreferencesStore(storage);
    first.getState().setSeries("alice", "computing");
    first.getState().setSeries("bob", "nature");
    first.getState().setSeries("alice", "all");
    const restored = createWorkspaceNamePreferencesStore(storage);
    await restored.persist.rehydrate();
    expect(restored.getState().seriesByUser).toEqual({ alice: "all", bob: "nature" });
    expect(restored.getState().seriesByUser.charlie).toBeUndefined();
  });

  it("hydrates before the first write so other account preferences survive", () => {
    const storage = memoryStorage(JSON.stringify({ state: { seriesByUser: { alice: "space" } }, version: 0 }));
    const store = createWorkspaceNamePreferencesStore(storage);
    store.getState().setSeries("bob", "ai");
    expect(store.getState().seriesByUser).toEqual({ alice: "space", bob: "ai" });
  });

  it("ignores anonymous and invalid selections without writing storage", () => {
    const storage = memoryStorage();
    const store = createWorkspaceNamePreferencesStore(storage);
    for (const account of [null, undefined, "", "   "]) {
      store.getState().setSeries(account, "space");
    }
    // Persisted/external values may bypass the static type at runtime.
    // @ts-expect-error Deliberately test malformed boundary input.
    store.getState().setSeries("alice", "stars");
    expect(store.getState().seriesByUser).toEqual({});
    expect(storage.setItem).not.toHaveBeenCalled();
  });

  it("retains valid account preferences while dropping malformed entries", async () => {
    const storage = memoryStorage(JSON.stringify({ state: {
      seriesByUser: { alice: "nature", bob: "all", bad: "stars", other: 4, "": "ai" },
      setSeries: "corrupted action",
    }, version: 0 }));
    const store = createWorkspaceNamePreferencesStore(storage);
    await store.persist.rehydrate();
    expect(store.getState().seriesByUser).toEqual({ alice: "nature", bob: "all" });
    expect(store.getState().setSeries).toBeTypeOf("function");
  });

  it.each(["not-json", "null", "[]", '{"state":null}', '{"state":{"seriesByUser":[]}}'])(
    "can still save after loading malformed storage: %s", async (initial) => {
      const store = createWorkspaceNamePreferencesStore(memoryStorage(initial));
      await store.persist.rehydrate();
      expect(store.getState().seriesByUser).toEqual({});
      store.getState().setSeries("alice", "voyage");
      expect(store.getState().seriesByUser).toEqual({ alice: "voyage" });
    },
  );

  it("keeps the session selection usable when device storage throws", async () => {
    const fail = () => { throw new Error("Storage unavailable"); };
    const store = createWorkspaceNamePreferencesStore({ getItem: fail, setItem: fail, removeItem: fail });
    await store.persist.rehydrate();
    expect(() => store.getState().setSeries("alice", "computing")).not.toThrow();
    expect(store.getState().seriesByUser).toEqual({ alice: "computing" });
    await store.persist.rehydrate();
    expect(store.getState().seriesByUser).toEqual({ alice: "computing" });
  });
});
