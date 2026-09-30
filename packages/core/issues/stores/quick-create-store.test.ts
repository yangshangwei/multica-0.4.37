// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  captureQuickCreateScope,
  isQuickCreateScopeCurrent,
  isQuickCreateStoreReady,
  useQuickCreateStore,
  type QuickCreateActorRef,
} from "./quick-create-store";
import { setCurrentWorkspace } from "../../platform/workspace-storage";
import { resetAllRegisteredDrafts } from "../../drafts/cleanup-registry";

const storage = vi.hoisted(() => {
  const values = new Map<string, string>();
  return {
    values,
    getItem: vi.fn<(key: string) => string | null | Promise<string | null>>(
      (key) => values.get(key) ?? null,
    ),
    setItem: vi.fn((key: string, value: string) => { values.set(key, value); }),
    removeItem: vi.fn((key: string) => { values.delete(key); }),
  };
});
const auth = vi.hoisted(() => ({
  user: { id: "user-1" },
  status: "authenticated",
}));
const readAuth = vi.hoisted(() => vi.fn());
vi.mock("../../platform/storage", () => ({ defaultStorage: storage }));
vi.mock("../../auth", () => ({ useAuthStore: { getState: () => { readAuth(); return auth; } } }));
const authReadsAtImport = readAuth.mock.calls.length;

const DEFAULTS = {
  lastActorType: null,
  lastActorId: null,
  keepOpen: false,
  favoriteActors: [],
  recentActors: [],
  defaultActor: null,
};
const agent = (id: string): QuickCreateActorRef => ({ type: "agent", id });
const squad = (id: string): QuickCreateActorRef => ({ type: "squad", id });
const key = (slug: string) => `multica_quick_create:${slug}`;
function seed(slug: string, state: unknown) {
  storage.values.set(key(slug), JSON.stringify({ state, version: 0 }));
}
async function activate(slug: string, id = `ws-${slug}`) {
  setCurrentWorkspace(slug, id);
  await Promise.resolve();
  await useQuickCreateStore.persist.rehydrate();
}
function record(actor: QuickCreateActorRef) {
  return useQuickCreateStore.getState().recordSuccessfulActor(actor, captureQuickCreateScope());
}
function deferredRead() {
  let resolve!: (value: string | null) => void;
  const promise = new Promise<string | null>((done) => { resolve = done; });
  return { promise, resolve };
}

beforeEach(async () => {
  setCurrentWorkspace(null, null);
  await Promise.resolve();
  resetAllRegisteredDrafts();
  storage.values.clear();
  storage.getItem.mockReset().mockImplementation((name) => storage.values.get(name) ?? null);
  storage.setItem.mockClear();
  auth.user = { id: "user-1" };
  auth.status = "authenticated";
  await activate("a");
});
afterEach(async () => {
  setCurrentWorkspace(null, null);
  await Promise.resolve();
  await useQuickCreateStore.persist.rehydrate();
});

describe("quick create preferences", () => {
  it("sets and clears a default without changing recent, favorite or last-accepted choices", () => {
    record(agent("last"));
    const before = useQuickCreateStore.getState();
    expect(before.setDefaultActor(squad("default"), captureQuickCreateScope())).toBe(true);
    expect(useQuickCreateStore.getState()).toMatchObject({
      defaultActor: squad("default"), lastActorId: "last", recentActors: [agent("last")], favoriteActors: [],
    });
    expect(useQuickCreateStore.getState().setDefaultActor(null)).toBe(true);
    expect(useQuickCreateStore.getState().defaultActor).toBeNull();
  });

  it("restores defaults per workspace and clears them on logout", async () => {
    useQuickCreateStore.getState().setDefaultActor(agent("a-default"));
    await activate("b");
    expect(useQuickCreateStore.getState().defaultActor).toBeNull();
    useQuickCreateStore.getState().setDefaultActor(squad("b-default"));
    await activate("a");
    expect(useQuickCreateStore.getState().defaultActor).toEqual(agent("a-default"));
    resetAllRegisteredDrafts();
    expect(useQuickCreateStore.getState().defaultActor).toBeNull();
  });

  it("rejects a default write captured before workspace or session replacement", async () => {
    const scope = captureQuickCreateScope();
    setCurrentWorkspace("b", "ws-b");
    expect(useQuickCreateStore.getState().setDefaultActor(agent("old"), scope)).toBe(false);
    await activate("a");
    resetAllRegisteredDrafts();
    await activate("a");
    expect(useQuickCreateStore.getState().setDefaultActor(agent("old"), scope)).toBe(false);
    expect(useQuickCreateStore.getState().defaultActor).toBeNull();
  });

  it("normalizes absent or invalid defaults and strips persisted metadata", async () => {
    for (const value of [undefined, null, { type: "human", id: "x" }, { type: "squad", id: " " }]) {
      seed("b", { defaultActor: value });
      await activate("b");
      expect(useQuickCreateStore.getState().defaultActor).toBeNull();
    }
    seed("b", { defaultActor: { ...agent("valid"), name: "not persisted" } });
    await activate("b");
    expect(useQuickCreateStore.getState().defaultActor).toEqual(agent("valid"));
  });

  it("does not read platform auth while the store module is initializing", () => {
    expect(authReadsAtImport).toBe(0);
  });

  it("starts with allowlisted preferences and no project memory", () => {
    expect(useQuickCreateStore.getState()).toMatchObject(DEFAULTS);
    expect(useQuickCreateStore.getState()).not.toHaveProperty("lastProjectId");
    expect(useQuickCreateStore.getState()).not.toHaveProperty("setLastProjectId");
  });

  it("retains the last-actor setter and keep-open preference", () => {
    const { setLastActor, setKeepOpen } = useQuickCreateStore.getState();
    setLastActor("agent", "agent-1");
    expect(useQuickCreateStore.getState()).toMatchObject({ lastActorType: "agent", lastActorId: "agent-1" });
    setLastActor("squad", "squad-1");
    setKeepOpen(true);
    expect(useQuickCreateStore.getState()).toMatchObject({ lastActorType: "squad", lastActorId: "squad-1", keepOpen: true });
    setLastActor(null, null);
    expect(useQuickCreateStore.getState()).toMatchObject({ lastActorType: null, lastActorId: null });
  });

  it("toggles favorites in insertion order using both actor type and ID", () => {
    const { toggleFavoriteActor } = useQuickCreateStore.getState();
    toggleFavoriteActor(agent("same"));
    toggleFavoriteActor(squad("same"));
    toggleFavoriteActor(agent("third"));
    expect(useQuickCreateStore.getState().favoriteActors).toEqual([agent("same"), squad("same"), agent("third")]);
    toggleFavoriteActor(agent("same"));
    toggleFavoriteActor(agent("same"));
    expect(useQuickCreateStore.getState().favoriteActors).toEqual([squad("same"), agent("third"), agent("same")]);
    expect(useQuickCreateStore.getState().recentActors).toEqual([]);
    expect(useQuickCreateStore.getState().lastActorId).toBeNull();
  });

  it.each(["recordSuccessfulActor", "recordRecentActor"] as const)("%s promotes repeats and retains 20 typed actors", (action) => {
    const recordActor = (actor: QuickCreateActorRef) => useQuickCreateStore.getState()[action](actor, captureQuickCreateScope());
    for (let i = 0; i < 21; i += 1) recordActor(agent(String(i)));
    recordActor(squad("20"));
    recordActor(agent("10"));
    const state = useQuickCreateStore.getState();
    expect(state.recentActors).toHaveLength(20);
    expect(state.recentActors.slice(0, 3)).toEqual([agent("10"), squad("20"), agent("20")]);
    expect(state.recentActors).not.toContainEqual(agent("0"));
    expect(state.recentActors).not.toContainEqual(agent("1"));
    expect(state.recentActors.filter((ref) => ref.type === "agent" && ref.id === "10")).toHaveLength(1);
    expect(state).toMatchObject(action === "recordSuccessfulActor"
      ? { lastActorType: "agent", lastActorId: "10" }
      : { lastActorType: null, lastActorId: null });
    expect(state.favoriteActors).toEqual([]);
  });

  it("records a manual assignee without changing quick-create choices", async () => {
    record(squad("quick-fallback"));
    const { setDefaultActor, toggleFavoriteActor, setKeepOpen, recordRecentActor } = useQuickCreateStore.getState();
    setDefaultActor(agent("default"));
    toggleFavoriteActor(squad("favorite"));
    setKeepOpen(true);

    expect(recordRecentActor(agent("manual"), captureQuickCreateScope())).toBe(true);
    const expected = {
      defaultActor: agent("default"),
      lastActorType: "squad",
      lastActorId: "quick-fallback",
      favoriteActors: [squad("favorite")],
      keepOpen: true,
      recentActors: [agent("manual"), squad("quick-fallback")],
    };
    expect(useQuickCreateStore.getState()).toMatchObject(expected);
    await activate("b");
    await activate("a");
    expect(useQuickCreateStore.getState()).toMatchObject(expected);
  });

  it("leaves quick-create fallback and default empty after a first manual assignment", () => {
    const actor = { ...squad("manual"), name: "not persisted" };
    expect(useQuickCreateStore.getState().recordRecentActor(actor, captureQuickCreateScope())).toBe(true);
    expect(useQuickCreateStore.getState()).toMatchObject({ ...DEFAULTS, recentActors: [squad("manual")] });
  });

  it.each([null, { type: "human", id: "person" }, { type: "agent", id: "" }, { type: "squad", id: " " }])("rejects malformed recent actor %j", (actor) => {
    record(agent("previous"));
    const before = useQuickCreateStore.getState();
    storage.setItem.mockClear();
    expect(before.recordRecentActor(actor as QuickCreateActorRef, captureQuickCreateScope())).toBe(false);
    expect(useQuickCreateStore.getState()).toBe(before);
    expect(storage.setItem).not.toHaveBeenCalled();
  });

  it("updates last actor and recents together for subscribers", () => {
    const snapshots: unknown[] = [];
    const unsubscribe = useQuickCreateStore.subscribe((state) => {
      snapshots.push([state.lastActorType, state.lastActorId, state.recentActors]);
    });
    record(squad("one"));
    unsubscribe();
    expect(snapshots).toEqual([["squad", "one", [squad("one")]]]);
  });

  it("persists only durable preference fields and restores them on return", async () => {
    useQuickCreateStore.getState().toggleFavoriteActor(agent("favorite"));
    record(squad("recent"));
    useQuickCreateStore.getState().setKeepOpen(true);
    expect(JSON.parse(storage.values.get(key("a"))!)).toEqual({
      state: { lastActorType: "squad", lastActorId: "recent", keepOpen: true, favoriteActors: [agent("favorite")], recentActors: [squad("recent")], defaultActor: null },
      version: 0,
    });
    await activate("b");
    expect(useQuickCreateStore.getState()).toMatchObject(DEFAULTS);
    await activate("a");
    expect(useQuickCreateStore.getState()).toMatchObject({ favoriteActors: [agent("favorite")], recentActors: [squad("recent")], keepOpen: true });
  });
});

describe("stored preference normalization", () => {
  it("preserves an old valid last actor and seeds only that real recent", async () => {
    seed("b", { lastActorType: "squad", lastActorId: "legacy", keepOpen: true, lastProjectId: "ignored", lastAgentId: "ignored", setLastActor: "invalid" });
    await activate("b");
    expect(useQuickCreateStore.getState()).toMatchObject({ lastActorType: "squad", lastActorId: "legacy", keepOpen: true, favoriteActors: [], recentActors: [squad("legacy")] });
    expect(useQuickCreateStore.getState().setLastActor).toBeTypeOf("function");
    expect(useQuickCreateStore.getState()).not.toHaveProperty("lastProjectId");
    expect(useQuickCreateStore.getState()).not.toHaveProperty("lastAgentId");
  });

  it("preserves intentionally empty recents without reseeding last actor", async () => {
    seed("b", { lastActorType: "agent", lastActorId: "legacy", recentActors: [], favoriteActors: [] });
    await activate("b");
    expect(useQuickCreateStore.getState()).toMatchObject({ recentActors: [], favoriteActors: [] });
  });

  it.each([null, [], false, "bad", { lastActorType: "human", lastActorId: "id", keepOpen: "yes", favoriteActors: {}, recentActors: null }, { lastActorType: "agent", lastActorId: "   " }].map((value) => [value]))("defaults invalid stored data: %j", async (value) => {
    record(agent("previous-workspace"));
    seed("b", value);
    await activate("b");
    expect(useQuickCreateStore.getState()).toMatchObject(DEFAULTS);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(true);
  });

  it("validates, strips extra fields, deduplicates typed refs, and caps restored recents", async () => {
    seed("b", {
      favoriteActors: [agent("same"), { ...squad("same"), name: "not saved" }, agent("same"), { type: "agent", id: "" }, { type: "person", id: "bad" }, null],
      recentActors: [squad("same"), squad("same"), ...Array.from({ length: 25 }, (_, i) => agent(String(i)))],
    });
    await activate("b");
    expect(useQuickCreateStore.getState().favoriteActors).toEqual([agent("same"), squad("same")]);
    expect(useQuickCreateStore.getState().recentActors).toEqual([squad("same"), ...Array.from({ length: 19 }, (_, i) => agent(String(i)))]);
  });

  it.each(["{broken", "null", '{"state":null}'])("recovers malformed storage %s without retaining the previous workspace", async (value) => {
    record(agent("previous-workspace"));
    storage.values.set(key("b"), value);
    await activate("b");
    expect(useQuickCreateStore.getState()).toMatchObject(DEFAULTS);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(true);
    expect(record(agent("recovered"))).toBe(true);
  });

  it("recovers a failed storage read to fresh defaults", async () => {
    record(agent("previous-workspace"));
    storage.getItem.mockImplementation(() => { throw new Error("storage unavailable"); });
    await activate("b");
    expect(useQuickCreateStore.getState()).toMatchObject(DEFAULTS);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(true);
  });
});

describe("workspace and session scope", () => {
  it("rejects writes immediately after switching, before the rehydration microtask", async () => {
    record(agent("a-recent"));
    const oldScope = captureQuickCreateScope();
    seed("b", { favoriteActors: [squad("b-favorite")], keepOpen: true });
    const before = storage.values.get(key("b"));
    setCurrentWorkspace("b", "ws-b");
    expect(useQuickCreateStore.persist.hasHydrated()).toBe(true);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState(), "ws-b", "user-1")).toBe(false);
    expect(isQuickCreateScopeCurrent(oldScope)).toBe(false);
    expect(useQuickCreateStore.getState().toggleFavoriteActor(agent("wrong"))).toBe(false);
    expect(useQuickCreateStore.getState().recordSuccessfulActor(agent("wrong"), oldScope)).toBe(false);
    expect(useQuickCreateStore.getState().recordRecentActor(agent("wrong"), oldScope)).toBe(false);
    expect(useQuickCreateStore.getState().recordRecentActor(agent("wrong"), captureQuickCreateScope())).toBe(false);
    useQuickCreateStore.getState().setLastActor("agent", "wrong");
    useQuickCreateStore.getState().setKeepOpen(false);
    expect(storage.values.get(key("b"))).toBe(before);
    await activate("b");
    expect(useQuickCreateStore.getState()).toMatchObject({ favoriteActors: [squad("b-favorite")], recentActors: [], keepOpen: true });
  });

  it("requires route and user identities to match the current authenticated mirror", () => {
    expect(captureQuickCreateScope("ws-other", "user-1")).toBeNull();
    expect(captureQuickCreateScope("ws-a", "other-user")).toBeNull();
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState(), "ws-other", "user-1")).toBe(false);
    auth.status = "recovering";
    expect(captureQuickCreateScope()).toBeNull();
    expect(record(agent("wrong"))).toBe(false);
    expect(useQuickCreateStore.getState().recordRecentActor(agent("wrong"), captureQuickCreateScope())).toBe(false);
  });

  it("rejects a recent-only write with no captured scope", () => {
    const before = useQuickCreateStore.getState();
    storage.setItem.mockClear();
    expect(before.recordRecentActor(agent("wrong"), null)).toBe(false);
    expect(useQuickCreateStore.getState()).toBe(before);
    expect(storage.setItem).not.toHaveBeenCalled();
  });

  it("rejects recent-only writes while the current scope is rehydrating", async () => {
    const scope = captureQuickCreateScope();
    const pending = deferredRead();
    storage.getItem.mockReturnValueOnce(pending.promise);
    const reading = useQuickCreateStore.persist.rehydrate();
    const before = useQuickCreateStore.getState();
    storage.setItem.mockClear();
    expect(isQuickCreateScopeCurrent(scope)).toBe(true);
    expect(before.recordRecentActor(agent("wrong"), scope)).toBe(false);
    expect(useQuickCreateStore.getState()).toBe(before);
    expect(storage.setItem).not.toHaveBeenCalled();
    pending.resolve(null);
    await reading;
  });

  it("rejects a previous user's delayed success and preferences", () => {
    record(agent("user-one"));
    const scope = captureQuickCreateScope();
    auth.user = { id: "user-2" };
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(false);
    expect(useQuickCreateStore.getState().recordSuccessfulActor(squad("late"), scope)).toBe(false);
    expect(useQuickCreateStore.getState().recordRecentActor(squad("late"), scope)).toBe(false);
  });

  it("clears every preference and rejects same-user success after auth cleanup", async () => {
    useQuickCreateStore.getState().toggleFavoriteActor(agent("favorite"));
    record(squad("recent"));
    useQuickCreateStore.getState().setKeepOpen(true);
    const scope = captureQuickCreateScope();
    const generation = useQuickCreateStore.getState().resetGeneration;
    resetAllRegisteredDrafts();
    storage.values.clear(); // Session cleanup removes persisted keys after resetting memory.
    expect(useQuickCreateStore.getState()).toMatchObject(DEFAULTS);
    expect(useQuickCreateStore.getState().resetGeneration).toBe(generation + 1);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(false);
    await useQuickCreateStore.persist.rehydrate();
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(true);
    expect(useQuickCreateStore.getState().recordSuccessfulActor(agent("late"), scope)).toBe(false);
    expect(useQuickCreateStore.getState().recordRecentActor(agent("late"), scope)).toBe(false);
    expect(useQuickCreateStore.getState().recentActors).toEqual([]);
  });

  it("never publishes an earlier workspace read after a later workspace is ready", async () => {
    const pending = deferredRead();
    storage.getItem.mockImplementation((name) => name === key("a") ? pending.promise : storage.values.get(name) ?? null);
    const earlier = useQuickCreateStore.persist.rehydrate();
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(false);
    seed("b", { favoriteActors: [agent("b-favorite")] });
    await activate("b");
    pending.resolve(JSON.stringify({ state: { favoriteActors: [squad("stale-a")] }, version: 0 }));
    await earlier;
    expect(useQuickCreateStore.getState().favoriteActors).toEqual([agent("b-favorite")]);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState(), "ws-b", "user-1")).toBe(true);
  });

  it("rejects stale hydration when auth reset occurs during the read", async () => {
    const pending = deferredRead();
    storage.getItem.mockReturnValueOnce(pending.promise);
    const reading = useQuickCreateStore.persist.rehydrate();
    resetAllRegisteredDrafts();
    pending.resolve(JSON.stringify({ state: { favoriteActors: [squad("old-session")] }, version: 0 }));
    await reading;
    expect(useQuickCreateStore.getState()).toMatchObject(DEFAULTS);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(false);
  });

  it("rejects an in-flight read if only the current user changes", async () => {
    const pending = deferredRead();
    storage.getItem.mockReturnValueOnce(pending.promise);
    const reading = useQuickCreateStore.persist.rehydrate();
    auth.user = { id: "user-2" };
    pending.resolve(JSON.stringify({ state: { recentActors: [agent("old-user")] }, version: 0 }));
    await reading;
    expect(useQuickCreateStore.getState().recentActors).toEqual([]);
    expect(isQuickCreateStoreReady(useQuickCreateStore.getState())).toBe(false);
  });
});
