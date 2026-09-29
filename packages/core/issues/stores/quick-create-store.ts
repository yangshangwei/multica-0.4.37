"use client";

import { create } from "zustand";
import { createJSONStorage, persist, type PersistStorage } from "zustand/middleware";
import { useAuthStore } from "../../auth";
import {
  createWorkspaceAwareStorage,
  getCurrentSlug,
  getCurrentWsId,
  registerForWorkspaceRehydration,
} from "../../platform/workspace-storage";
import { defaultStorage } from "../../platform/storage";
import { registerDraftCleanup } from "../../drafts/cleanup-registry";

export type QuickCreateActorType = "agent" | "squad";
export interface QuickCreateActorRef {
  type: QuickCreateActorType;
  id: string;
}

export interface QuickCreateScope {
  workspaceSlug: string;
  workspaceId: string;
  userId: string;
  resetGeneration: number;
}

interface QuickCreatePreferences {
  lastActorType: QuickCreateActorType | null;
  lastActorId: string | null;
  keepOpen: boolean;
  favoriteActors: QuickCreateActorRef[];
  recentActors: QuickCreateActorRef[];
}

interface QuickCreateState extends QuickCreatePreferences {
  hydratedScope: QuickCreateScope | null;
  resetGeneration: number;
  setLastActor: (type: QuickCreateActorType | null, id: string | null) => void;
  setKeepOpen: (value: boolean) => void;
  toggleFavoriteActor: (actor: QuickCreateActorRef, scope?: QuickCreateScope | null) => boolean;
  recordSuccessfulActor: (actor: QuickCreateActorRef, scope: QuickCreateScope | null) => boolean;
}

const RECENT_ACTOR_LIMIT = 20;
const STORAGE_KEY = "multica_quick_create";
const defaults = (): QuickCreatePreferences => ({
  lastActorType: null,
  lastActorId: null,
  keepOpen: false,
  favoriteActors: [],
  recentActors: [],
});

function actorRef(value: unknown): QuickCreateActorRef | null {
  if (!value || typeof value !== "object") return null;
  const { type, id } = value as Record<string, unknown>;
  return (type === "agent" || type === "squad") && typeof id === "string" && id.trim()
    ? { type, id }
    : null;
}

function sameActor(a: QuickCreateActorRef, b: QuickCreateActorRef): boolean {
  return a.type === b.type && a.id === b.id;
}

function actorRefs(value: unknown, limit = Infinity): QuickCreateActorRef[] {
  if (!Array.isArray(value)) return [];
  const result: QuickCreateActorRef[] = [];
  const seen = new Set<string>();
  for (const entry of value) {
    const actor = actorRef(entry);
    if (!actor) continue;
    const key = `${actor.type}:${actor.id}`;
    if (seen.has(key)) continue;
    seen.add(key);
    result.push(actor);
    if (result.length === limit) break;
  }
  return result;
}

function preferences(value: unknown): QuickCreatePreferences {
  if (!value || typeof value !== "object" || Array.isArray(value)) return defaults();
  const data = value as Record<string, unknown>;
  const lastActor = actorRef({ type: data.lastActorType, id: data.lastActorId });
  return {
    lastActorType: lastActor?.type ?? null,
    lastActorId: lastActor?.id ?? null,
    keepOpen: data.keepOpen === true,
    favoriteActors: actorRefs(data.favoriteActors),
    // The old tuple is a real accepted submission. An explicit empty history
    // remains empty; legacy lastAgentId and project preferences stay ignored.
    recentActors: data.recentActors === undefined && lastActor
      ? [lastActor]
      : actorRefs(data.recentActors, RECENT_ACTOR_LIMIT),
  };
}

/** Capture before awaiting a create request; null scopes can never write. */
export function captureQuickCreateScope(
  expectedWorkspaceId?: string,
  expectedUserId?: string,
): QuickCreateScope | null {
  // The auth proxy is registered by the platform after module initialization.
  const auth = useAuthStore.getState?.();
  const workspaceSlug = getCurrentSlug();
  const workspaceId = getCurrentWsId();
  const userId = auth?.user?.id;
  if (
    auth?.status !== "authenticated" || !userId || !workspaceSlug || !workspaceId ||
    (expectedWorkspaceId !== undefined && expectedWorkspaceId !== workspaceId) ||
    (expectedUserId !== undefined && expectedUserId !== userId)
  ) return null;
  return { workspaceSlug, workspaceId, userId, resetGeneration: useQuickCreateStore.getState().resetGeneration };
}

function sameScope(a: QuickCreateScope | null, b: QuickCreateScope | null): boolean {
  return a === b || (!!a && !!b &&
    a.workspaceSlug === b.workspaceSlug && a.workspaceId === b.workspaceId &&
    a.userId === b.userId && a.resetGeneration === b.resetGeneration);
}

export function isQuickCreateScopeCurrent(scope: QuickCreateScope | null): boolean {
  return scope !== null && sameScope(scope, captureQuickCreateScope());
}

/** Also checks live mirrors, closing the gap before workspace rehydration starts. */
export function isQuickCreateStoreReady(
  state: Pick<QuickCreateState, "hydratedScope" | "resetGeneration">,
  expectedWorkspaceId?: string,
  expectedUserId?: string,
): boolean {
  const current = captureQuickCreateScope(expectedWorkspaceId, expectedUserId);
  return current !== null && state.resetGeneration === current.resetGeneration &&
    sameScope(state.hydratedScope, current);
}

// Read metadata exists only between this local storage boundary and merge. It
// never reaches persisted JSON or the store's public preference fields.
const hydrationRead = Symbol("quick-create-hydration");
interface StoredPreferences extends QuickCreatePreferences {
  [hydrationRead]?: { scope: QuickCreateScope | null; generation: number };
}
let readGeneration = 0;
const jsonStorage = createJSONStorage<StoredPreferences>(
  () => createWorkspaceAwareStorage(defaultStorage),
)!;
const scopedStorage: PersistStorage<StoredPreferences> = {
  getItem: async (name) => {
    const read = { scope: captureQuickCreateScope(), generation: ++readGeneration };
    let value: unknown;
    try {
      // Auth cleanup owns account isolation; no anonymous read may restore a
      // prior user's preferences while the platform is authenticating.
      value = read.scope ? (await jsonStorage.getItem(name))?.state : undefined;
    } catch {
      // Missing and corrupt records both hydrate fresh defaults, never data
      // left in memory by a different workspace. Do not overwrite on read.
      value = undefined;
    }
    return { state: { ...preferences(value), [hydrationRead]: read }, version: 0 };
  },
  setItem: (name, value) => {
    if (isQuickCreateStoreReady(useQuickCreateStore.getState())) {
      return jsonStorage.setItem(name, value);
    }
    return undefined;
  },
  removeItem: (name) => jsonStorage.removeItem(name),
};

// Picker-local, workspace-scoped preferences. Account isolation comes from
// session cleanup, not the browser profile: the storage key has no user ID.
// Prompt/project drafts stay in draft-store; project is not a preference.
export const useQuickCreateStore = create<QuickCreateState>()(
  persist<QuickCreateState, [], [], StoredPreferences>(
    (set, get) => ({
      ...defaults(),
      hydratedScope: null,
      resetGeneration: 0,
      setLastActor: (type, id) => {
        if (!isQuickCreateStoreReady(get())) return;
        const actor = actorRef({ type, id });
        set({ lastActorType: actor?.type ?? null, lastActorId: actor?.id ?? null });
      },
      setKeepOpen: (value) => {
        if (isQuickCreateStoreReady(get())) set({ keepOpen: value });
      },
      toggleFavoriteActor: (value, scope = captureQuickCreateScope()) => {
        const actor = actorRef(value);
        if (!actor || !isQuickCreateScopeCurrent(scope) || !isQuickCreateStoreReady(get())) return false;
        set(({ favoriteActors }) => ({
          favoriteActors: favoriteActors.some((entry) => sameActor(entry, actor))
            ? favoriteActors.filter((entry) => !sameActor(entry, actor))
            : [...favoriteActors, actor],
        }));
        return true;
      },
      recordSuccessfulActor: (value, scope) => {
        const actor = actorRef(value);
        if (!actor || !isQuickCreateScopeCurrent(scope) || !isQuickCreateStoreReady(get())) return false;
        set(({ recentActors }) => ({
          lastActorType: actor.type,
          lastActorId: actor.id,
          recentActors: [actor, ...recentActors.filter((entry) => !sameActor(entry, actor))].slice(0, RECENT_ACTOR_LIMIT),
        }));
        return true;
      },
    }),
    {
      name: STORAGE_KEY,
      storage: scopedStorage,
      skipHydration: true,
      partialize: ({ lastActorType, lastActorId, keepOpen, favoriteActors, recentActors }) => ({
        lastActorType, lastActorId, keepOpen, favoriteActors, recentActors,
      }),
      onRehydrateStorage: () => {
        // The storage write gate makes this in-memory invalidation safe: it
        // cannot overwrite the destination namespace before reading it.
        useQuickCreateStore.setState({ hydratedScope: null });
      },
      merge: (persisted, current) => {
        const data = persisted as StoredPreferences;
        const read = data[hydrationRead];
        if (!read || read.generation !== readGeneration || !sameScope(read.scope, captureQuickCreateScope())) {
          return current;
        }
        return { ...current, ...preferences(data), hydratedScope: read.scope };
      },
    },
  ),
);

registerForWorkspaceRehydration(() => { void useQuickCreateStore.persist.rehydrate(); });
// Workspace activation (or the mounted create panel for an unchanged scope)
// owns the first read. Importing draft cleanup must not read auth before the
// platform and its singleton dependency graph have finished initializing.

registerDraftCleanup({
  storageKey: STORAGE_KEY,
  workspaceScoped: true,
  resetInMemory: () => {
    readGeneration += 1;
    useQuickCreateStore.setState((state) => ({
      ...defaults(),
      hydratedScope: null,
      resetGeneration: state.resetGeneration + 1,
    }));
  },
});
