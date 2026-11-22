// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { createSidebarPreferencesStore } from "./sidebar-preferences";

function storage(initial?: string) {
  let value = initial ?? null;
  return { getItem: () => value, setItem: vi.fn((_key: string, next: string) => { value = next; }), removeItem: vi.fn() };
}

describe("sidebar group preferences", () => {
  it("starts with daily work open and secondary groups closed", () => {
    const store = createSidebarPreferencesStore(storage());
    expect(store.getState().groups).toEqual({ work: true, ai: false, resources: false, collaboration: false });
  });

  it("restores only boolean preferences and preserves explicit user choices", async () => {
    const adapter = storage(JSON.stringify({ state: { groups: { work: false, ai: true, resources: "open", injected: true } }, version: 0 }));
    const store = createSidebarPreferencesStore(adapter);
    await store.persist.rehydrate();
    expect(store.getState().groups).toEqual({ work: false, ai: true, resources: false, collaboration: false });
    store.getState().setGroupOpen("resources", true);
    const remount = createSidebarPreferencesStore(adapter);
    await remount.persist.rehydrate();
    expect(remount.getState().groups).toEqual({ work: false, ai: true, resources: true, collaboration: false });
    expect(JSON.parse(adapter.getItem()!).state).toEqual({ groups: { work: false, ai: true, resources: true, collaboration: false } });
  });

  it("defaults legacy collaboration disclosure to closed and restores its later manual choice", async () => {
    const adapter = storage(JSON.stringify({ state: { groups: { work: false, ai: true, resources: true } }, version: 0 }));
    const store = createSidebarPreferencesStore(adapter);
    await store.persist.rehydrate();
    expect(store.getState().groups.collaboration).toBe(false);
    store.getState().setGroupOpen("collaboration", true);
    const remount = createSidebarPreferencesStore(adapter);
    await remount.persist.rehydrate();
    expect(remount.getState().groups).toEqual({ work: false, ai: true, resources: true, collaboration: true });
  });

  it("keeps navigation working when browser storage is blocked", async () => {
    const store = createSidebarPreferencesStore({ getItem: () => { throw new Error("blocked"); }, setItem: () => { throw new Error("blocked"); }, removeItem: vi.fn() });
    await store.persist.rehydrate();
    expect(() => store.getState().setGroupOpen("ai", true)).not.toThrow();
    expect(store.getState().groups.ai).toBe(true);
  });
});
